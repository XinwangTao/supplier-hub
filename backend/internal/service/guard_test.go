package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/example/supplier-hub/backend/internal/domain"
	"github.com/example/supplier-hub/backend/internal/qcdst"
)

// 这一组测试锁的是"评级不能被刷"这件事。
// 每一条都对应一个真实的作弊路径，不是为测试而测试。

func ctx() context.Context { return context.Background() }

// goodBundle 一份"表现良好"的基准单据，供质量封顶等用例做对照。
func goodBundle(events []domain.QualityEvent) qcdst.Bundle {
	b := qcdst.Bundle{
		Supplier: domain.Supplier{ID: "s1", Name: "测试", Capabilities: []string{"pcba"}},
		Tenant:   domain.Tenant{ID: "t1", Name: "甲"},
	}
	for i := 0; i < 20; i++ {
		b.Orders = append(b.Orders, domain.PurchaseOrder{
			ID: "po", TenantID: "t1", SupplierID: "s1", Qty: 100, UnitPrice: 10, IssuedAt: "2026-04-01",
		})
		b.Deliveries = append(b.Deliveries, domain.Delivery{
			TenantID: "t1", SupplierID: "s1", POID: "po", Qty: 100, Accepted: 100,
			OnTime: true, IQCResult: "pass", ArrivedAt: "2026-04-20", PromisedAt: "2026-04-20",
			LeadDays: 19, PromisedLeadDays: 19,
		})
	}
	b.Events = events
	return b
}

func dimOf(a domain.QCDSTAssessment, d domain.Dimension) domain.DimensionScore {
	for _, ds := range a.Dimensions {
		if ds.Dimension == d {
			return ds
		}
	}
	return domain.DimensionScore{Score: -1}
}

// 路径一：绕过"引入 SRM"，直接给任意供应商录交付记录。
// 少了这道校验，平台评级就是一个谁都能刷的数字。
func TestGuard_NoEngagementCannotRecord(t *testing.T) {
	ops, store := newOps("s1")
	store.EnsureTenant(domain.Tenant{ID: "t1", Name: "甲"})

	_, err := ops.RecordDeliveryGuarded(ctx(), domain.Delivery{
		TenantID: "t1", SupplierID: "s1", Qty: 100, Accepted: 100,
		IQCResult: "pass", ArrivedAt: "2026-05-01", BatchNo: "B1",
	}, "tester")
	if !errors.Is(err, ErrEngagementRequired) {
		t.Fatalf("无合作关系应拒绝录入，got %v", err)
	}

	// 建立合作关系后可以录。
	store.AddEngagement(domain.Engagement{TenantID: "t1", SupplierID: "s1", Status: "active"})
	if _, err := ops.RecordDeliveryGuarded(ctx(), domain.Delivery{
		TenantID: "t1", SupplierID: "s1", Qty: 100, Accepted: 100,
		IQCResult: "pass", ArrivedAt: "2026-05-01", BatchNo: "B1",
	}, "tester"); err != nil {
		t.Fatalf("有合作关系应允许录入，got %v", err)
	}
}

// 路径二：凭空造一个 tenant_id。建档这一步不能省，
// 否则"哪家企业评的"这句话没有主语。
func TestGuard_UnknownTenantRejected(t *testing.T) {
	ops, _ := newOps("s1")
	_, err := ops.RecordOrderGuarded(ctx(), domain.PurchaseOrder{
		TenantID: "t-not-exist", SupplierID: "s1", Qty: 10, PONo: "PO1",
	}, "tester")
	if !errors.Is(err, ErrTenantRequired) {
		t.Fatalf("未建档企业应拒绝，got %v", err)
	}
	// 空 tenant 同样拒绝
	if _, err := ops.RecordOrderGuarded(ctx(), domain.PurchaseOrder{
		SupplierID: "s1", Qty: 10, PONo: "PO2",
	}, "tester"); !errors.Is(err, ErrTenantRequired) {
		t.Fatalf("空 tenant_id 应拒绝，got %v", err)
	}
}

// 路径三：关系已终止还继续录单。终止通常有质量或合规原因，
// 想恢复必须显式改状态，让这件事留下痕迹。
func TestGuard_TerminatedCannotRecord(t *testing.T) {
	ops, store := newOps("s1")
	store.EnsureTenant(domain.Tenant{ID: "t1", Name: "甲"})
	store.AddEngagement(domain.Engagement{TenantID: "t1", SupplierID: "s1", Status: "terminated"})
	_, err := ops.RecordDeliveryGuarded(ctx(), domain.Delivery{
		TenantID: "t1", SupplierID: "s1", Qty: 10, BatchNo: "B9", ArrivedAt: "2026-05-01",
	}, "tester")
	if !errors.Is(err, ErrEngagementTerminated) {
		t.Fatalf("已终止关系应拒绝录入，got %v", err)
	}
}

// 路径四：把同一个批次 POST 15 遍凑够评级样本量。
// 想拿 15 个批次的资格，就得真有 15 个批次。
func TestGuard_DuplicateBatchRejected(t *testing.T) {
	ops, store := newOps("s1")
	store.EnsureTenant(domain.Tenant{ID: "t1", Name: "甲"})
	store.AddEngagement(domain.Engagement{TenantID: "t1", SupplierID: "s1", Status: "active"})

	base := domain.Delivery{
		TenantID: "t1", SupplierID: "s1", Qty: 100, Accepted: 100,
		IQCResult: "pass", ArrivedAt: "2026-05-01", BatchNo: "BATCH-001",
	}
	if _, err := ops.RecordDeliveryGuarded(ctx(), base, "tester"); err != nil {
		t.Fatalf("首次录入应成功，got %v", err)
	}
	if _, err := ops.RecordDeliveryGuarded(ctx(), base, "tester"); !errors.Is(err, ErrDuplicateBatch) {
		t.Fatalf("重复批次号应拒绝，got %v", err)
	}

	// 订单号同理
	po := domain.PurchaseOrder{TenantID: "t1", SupplierID: "s1", Qty: 50, PONo: "PO-001"}
	if _, err := ops.RecordOrderGuarded(ctx(), po, "tester"); err != nil {
		t.Fatal(err)
	}
	if _, err := ops.RecordOrderGuarded(ctx(), po, "tester"); !errors.Is(err, ErrDuplicateOrder) {
		t.Fatalf("重复订单号应拒绝，got %v", err)
	}
}

// 每一次写入都要留痕，包括被拒的那次。
// 没有操作人的审计等于没有审计；被拒记录本身就是风险信号。
func TestGuard_AuditTrail(t *testing.T) {
	ops, store := newOps("s1")
	store.EnsureTenant(domain.Tenant{ID: "t1", Name: "甲"})
	store.AddEngagement(domain.Engagement{TenantID: "t1", SupplierID: "s1", Status: "active"})

	if _, err := ops.RecordOrderGuarded(ctx(), domain.PurchaseOrder{
		TenantID: "t1", SupplierID: "s1", Qty: 100, PONo: "PO-A", PartName: "外壳",
	}, "张三"); err != nil {
		t.Fatal(err)
	}
	// 一次被拒的写入
	_, _ = ops.RecordDeliveryGuarded(ctx(), domain.Delivery{
		TenantID: "t2", SupplierID: "s1", Qty: 10, BatchNo: "X",
	}, "李四")

	entries := ops.AuditEntries("t1", 50)
	if len(entries) == 0 {
		t.Fatal("应有审计记录")
	}
	if entries[0].Actor != "张三" || entries[0].Action != "order.create" {
		t.Fatalf("最近一条应是张三的订单创建，got %+v", entries[0])
	}

	all := ops.AuditEntries("", 50)
	rejected := 0
	for _, e := range all {
		if e.Rejected {
			rejected++
		}
	}
	if rejected == 0 {
		t.Fatal("被拒写入也应留痕，用于发现反复试探边界的行为")
	}
}

// 平台评级不能被单一采购方绑架。
// 一家企业刷 500 个批次，另外两家各 5 个批次——
// 不封顶的话"3 家企业验证"实际就是"一家说了算"。
func TestPlatformRating_SingleBuyerCapped(t *testing.T) {
	ops, store := newOps("s1")
	ctx := context.Background()

	add := func(tenant string, batches int, day int, score string) {
		store.EnsureTenant(domain.Tenant{ID: tenant, Name: tenant})
		store.AddEngagement(domain.Engagement{TenantID: tenant, SupplierID: "s1", Status: "active"})
		for i := 0; i < batches; i++ {
			store.AddOrder(domain.PurchaseOrder{
				ID: "po", TenantID: tenant, SupplierID: "s1", Qty: 100, UnitPrice: 10,
				IssuedAt: "2026-05-01",
			})
			store.AddDelivery(domain.Delivery{
				TenantID: tenant, SupplierID: "s1", POID: "po", Qty: 100, Accepted: 100,
				OnTime: true, IQCResult: score, ArrivedAt: "2026-05-01", PromisedAt: "2026-05-01",
			})
		}
	}
	// 甲：500 批
	add("t-big", 500, 1, "pass")
	// 乙、丙：各 5 批，且表现差
	for _, tn := range []string{"t-a", "t-b"} {
		store.EnsureTenant(domain.Tenant{ID: tn, Name: tn})
		store.AddEngagement(domain.Engagement{TenantID: tn, SupplierID: "s1", Status: "active"})
		for i := 0; i < 5; i++ {
			store.AddOrder(domain.PurchaseOrder{ID: "p", TenantID: tn, SupplierID: "s1", Qty: 10, UnitPrice: 10})
			store.AddDelivery(domain.Delivery{
				TenantID: tn, SupplierID: "s1", POID: "p", Qty: 10, Accepted: 5, Rejected: 5,
				OnTime: false, IQCResult: "fail", ArrivedAt: "2026-05-20", PromisedAt: "2026-05-01",
			})
		}
	}

	ratings := ops.PlatformRatings(ctx)
	if len(ratings) != 1 {
		t.Fatalf("应有一条评级，got %d", len(ratings))
	}
	r := ratings[0]
	if r.DominantShare > 0.55 {
		t.Fatalf("单一买方权重占比应被封顶在 50%% 附近，got %.2f", r.DominantShare)
	}
	if r.BuyerCount != 3 {
		t.Fatalf("买方数应为 3，got %d", r.BuyerCount)
	}
	// 封顶后，表现差的两家必须能把分数拉下来——
	// 否则封顶只是个摆设，评级还是由那 500 批决定。
	if r.Total > 85 {
		t.Fatalf("两家差评买方应显著拉低总分，got %.1f（说明封顶没起作用）", r.Total)
	}
}

// 数据体检要能抓出"用一堆小样批垫高合格率"。
func TestIntegrity_DetectsSmallBatchPadding(t *testing.T) {
	deliveries := []domain.Delivery{}
	// 12 个 10 件的小样批，全部合格
	for i := 0; i < 12; i++ {
		deliveries = append(deliveries, domain.Delivery{
			Qty: 10, Accepted: 10, IQCResult: "pass", ArrivedAt: "2026-05-01", PromisedAt: "2026-05-01", OnTime: true,
		})
	}
	// 3 个 10000 件的主力批，全部判退
	for i := 0; i < 3; i++ {
		deliveries = append(deliveries, domain.Delivery{
			Qty: 10000, Accepted: 6000, Rejected: 4000, IQCResult: "fail",
			ArrivedAt: "2026-05-10", PromisedAt: "2026-05-01", OnTime: false,
		})
	}
	q := qcdst.CheckIntegrity(deliveries)
	if q.SmallBatchRate < 30 {
		t.Fatalf("小样批占比 %.0f%% 应被识别（≥30%%），说明合格率被垫高的风险没报出来", q.SmallBatchRate)
	}
	found := false
	for _, i := range q.Issues {
		if i.Code == "small_batch" {
			found = true
		}
	}
	if !found {
		t.Fatalf("应给出 small_batch 提示，got %+v", q.Issues)
	}
}

// 准时标记与日期自相矛盾：这类数据会直接把交付分抬高，必须标 bad。
func TestIntegrity_DetectsBadOnTime(t *testing.T) {
	q := qcdst.CheckIntegrity([]domain.Delivery{
		{Qty: 100, Accepted: 100, ArrivedAt: "2026-05-20", PromisedAt: "2026-05-01", OnTime: true, IQCResult: "pass"},
	})
	if q.Level != "bad" {
		t.Fatalf("说准时但晚到 19 天应为严重数据问题，got level=%s", q.Level)
	}
	if q.BadOnTime != 1 {
		t.Fatalf("应统计到 1 条矛盾数据，got %d", q.BadOnTime)
	}
}

// 未来日期：通常是录错或人为回填，同样按 bad 处理。
func TestIntegrity_DetectsFutureDate(t *testing.T) {
	future := time.Now().AddDate(0, 0, 30).Format("2006-01-02")
	q := qcdst.CheckIntegrity([]domain.Delivery{
		{Qty: 100, Accepted: 100, ArrivedAt: future, PromisedAt: future, OnTime: true, IQCResult: "pass"},
	})
	if q.FutureDated != 1 || q.Level != "bad" {
		t.Fatalf("未来日期应被识别为严重问题，got future=%d level=%s", q.FutureDated, q.Level)
	}
}

// 权重方案不能靠供应商给自己加一个标签来切换。
// 工业方案把 T 从 10% 抬到 20%，如果能靠一个 cnc 标签切过去，
// 等于把权重交给了被评估方自己。
func TestProfileFor_RequiresMajorityIndustrial(t *testing.T) {
	one := domain.Supplier{Capabilities: []string{"cnc", "pcba", "assembly"}}
	if qcdst.ProfileFor(one).Name == "工业装备" {
		t.Fatal("仅 1/3 工艺是工业类，不应切到工业权重方案")
	}
	two := domain.Supplier{Capabilities: []string{"cnc", "diecasting", "pcba"}}
	if qcdst.ProfileFor(two).Name != "工业装备" {
		t.Fatal("工业工艺占多数且命中两类，应切到工业权重方案")
	}
	majority := domain.Supplier{Capabilities: []string{"cnc", "welding"}}
	if qcdst.ProfileFor(majority).Name != "工业装备" {
		t.Fatal("全部是工业工艺应切到工业方案")
	}
}

// 未闭环的停线事故必须触发质量封顶，且与"记录了多少起事件"无关。
// 按次扣分的写法会制造逆向激励：记录得越认真分越低。
func TestQuality_CappedOnOpenCritical(t *testing.T) {
	base := func(events []domain.QualityEvent) domain.QCDSTAssessment {
		return qcdst.Compute(goodBundle(events))
	}
	withOpen := base([]domain.QualityEvent{
		{Severity: "critical", ClosedAt: "", OccurredAt: "2026-05-01"},
	})
	q := dimOf(withOpen, domain.DimQuality)
	if q.Score > 70 {
		t.Fatalf("有未闭环停线事故时质量应封顶 70，got %.1f", q.Score)
	}
	closed := base([]domain.QualityEvent{
		{Severity: "critical", ClosedAt: "2026-05-10", DueAt: "2026-05-15", OccurredAt: "2026-05-01"},
	})
	qc := dimOf(closed, domain.DimQuality)
	if qc.Score <= q.Score {
		t.Fatalf("闭环后质量分应高于未闭环（%.1f vs %.1f），否则整改没有回报", qc.Score, q.Score)
	}
}
