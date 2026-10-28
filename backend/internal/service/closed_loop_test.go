package service

import (
	"context"
	"testing"

	"github.com/example/supplier-hub/backend/internal/domain"
	"github.com/example/supplier-hub/backend/internal/repository"
	"github.com/example/supplier-hub/backend/internal/srm"
)

// newOps 造一个只有一家供应商的最小环境。
func newOps(supplierID string) (*SupplierOpsService, *srm.Store) {
	repo := repository.NewMemorySupplierRepository([]domain.Supplier{{
		ID: supplierID, Name: "测试供应商", ShortName: "测试", CreditCode: "TEST-001",
		Capabilities: []string{"cnc"},
	}})
	store := srm.NewStore()
	return NewSupplierOpsService(repo, store), store
}

// 累计批次不到 15 的供应商必须标"观察期"，而不是"0 家企业评过分"。
// 这两句话对新用户的含义完全不同：前者是"再等等"，后者是"没人用过"。
func TestPlatformRatings_ProvisionalWhenBelowThreshold(t *testing.T) {
	ops, store := newOps("s1")
	store.EnsureTenant(domain.Tenant{ID: "t1", Name: "甲"})
	store.AddEngagement(domain.Engagement{TenantID: "t1", SupplierID: "s1", Status: "active"})
	store.AddOrder(domain.PurchaseOrder{ID: "po1", TenantID: "t1", SupplierID: "s1", Qty: 100, UnitPrice: 10, Status: "closed"})
	for i := 0; i < 11; i++ {
		store.AddDelivery(domain.Delivery{
			ID: "d", TenantID: "t1", SupplierID: "s1", POID: "po1", Qty: 100, Accepted: 100,
			OnTime: true, IQCResult: "pass", ArrivedAt: "2026-05-01", PromisedAt: "2026-05-01",
		})
	}
	ratings := ops.PlatformRatings(context.Background())
	if len(ratings) != 1 {
		t.Fatalf("应有一条评级，got %d", len(ratings))
	}
	r := ratings[0]
	if !r.Provisional {
		t.Fatalf("11 批（<15）应标观察期，got provisional=%v", r.Provisional)
	}
	if r.Grade != domain.GradeNA {
		t.Fatalf("观察期不应出评级，got %s", r.Grade)
	}
	if r.BuyerCount != 1 {
		t.Fatalf("买方数应为 1，got %d", r.BuyerCount)
	}
}

// 只有候选关系、没有业务单据的采购方，不能算进买方数。
// 否则"3 家企业都在用"会被"我关注了它"伪造出来。
func TestPlatformRatings_CandidateWithoutDocsNotABuyer(t *testing.T) {
	ops, store := newOps("s1")
	store.EnsureTenant(domain.Tenant{ID: "t1", Name: "甲"})
	store.EnsureTenant(domain.Tenant{ID: "t2", Name: "乙"})
	// 甲：真有订单和交付
	store.AddEngagement(domain.Engagement{TenantID: "t1", SupplierID: "s1", Status: "active"})
	store.AddOrder(domain.PurchaseOrder{ID: "po1", TenantID: "t1", SupplierID: "s1", Qty: 100, UnitPrice: 10})
	for i := 0; i < 20; i++ {
		store.AddDelivery(domain.Delivery{
			TenantID: "t1", SupplierID: "s1", POID: "po1", Qty: 100, Accepted: 100,
			OnTime: true, IQCResult: "pass", ArrivedAt: "2026-05-01", PromisedAt: "2026-05-01",
		})
	}
	// 乙：只是把它放进了候选池，什么单都没下
	store.AddEngagement(domain.Engagement{TenantID: "t2", SupplierID: "s1", Status: "candidate"})

	ratings := ops.PlatformRatings(context.Background())
	if len(ratings) != 1 {
		t.Fatalf("应有一条评级，got %d", len(ratings))
	}
	if ratings[0].BuyerCount != 1 {
		t.Fatalf("纯候选不该计入买方数，got %d", ratings[0].BuyerCount)
	}
}

// 引入要能去重：同一个采购方对同一家供应商只能有一份合作档案，
// 否则同一份数据会被重复计入评分。
func TestEngage_DeduplicatesAndValidates(t *testing.T) {
	ops, _ := newOps("s1")
	first, err := ops.Engage(context.Background(), EngageRequest{TenantID: "t1", TenantName: "甲", SupplierID: "s1", Source: "sourcing"})
	if err != nil || !first.Created {
		t.Fatalf("首次引入应创建成功：created=%v err=%v", first.Created, err)
	}
	if first.Engagement.Status != string(domain.EngagementCandidate) {
		t.Fatalf("从寻源引入默认应为候选，got %s", first.Engagement.Status)
	}
	second, err := ops.Engage(context.Background(), EngageRequest{TenantID: "t1", SupplierID: "s1", Source: "sourcing"})
	if err != nil || second.Created {
		t.Fatalf("重复引入不应新建：created=%v err=%v", second.Created, err)
	}
	if _, err := ops.Engage(context.Background(), EngageRequest{TenantID: "t1", SupplierID: "不存在"}); err == nil {
		t.Fatal("引入不存在的供应商应报错")
	}
	if _, err := ops.Engage(context.Background(), EngageRequest{TenantID: "t1", SupplierID: "s1", Status: "乱写"}); err == nil {
		t.Fatal("非法状态应被拒绝")
	}
}

// 已终止的合作不能直接复活成"合作中"——终止通常有质量或合规原因，不该一键翻篇。
func TestSetEngagementStatus_TerminatedCannotReopen(t *testing.T) {
	ops, _ := newOps("s1")
	if _, err := ops.Engage(context.Background(), EngageRequest{TenantID: "t1", SupplierID: "s1", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ops.SetEngagementStatus("t1", "s1", "terminated"); err != nil {
		t.Fatal(err)
	}
	if _, err := ops.SetEngagementStatus("t1", "s1", "active"); err == nil {
		t.Fatal("已终止不应能直接恢复为合作中")
	}
	e, err := ops.SetEngagementStatus("t1", "s1", "candidate")
	if err != nil || e.Status != "candidate" {
		t.Fatalf("应先退回候选：status=%s err=%v", e.Status, err)
	}
}
