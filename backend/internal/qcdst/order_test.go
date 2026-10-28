package qcdst

import (
	"testing"

	"github.com/example/supplier-hub/backend/internal/domain"
)

func testOrder(status string) domain.PurchaseOrder {
	return domain.PurchaseOrder{
		ID: "po-1", TenantID: "t1", SupplierID: "s1", PONo: "PO26001",
		PartName: "结构件", Qty: 1000, UnitPrice: 100, Amount: 100000,
		IssuedAt: "2026-03-01", PromisedAt: "2026-03-20", Status: status,
	}
}

func dim(card domain.OrderScorecard, d domain.Dimension) domain.OrderDimension {
	for _, x := range card.Dimensions {
		if x.Dimension == d {
			return x
		}
	}
	return domain.OrderDimension{}
}

// 没有交付批次时，质量维必须判不了，而且不能因为"没判"就把总分拉低到 0。
func TestScoreOrder_NoDelivery_QualityNotJudged(t *testing.T) {
	card := ScoreOrder(OrderScoreInput{Order: testOrder("open"), Profile: ProfileDefault, Benchmark: 100})
	q := dim(card, domain.DimQuality)
	if q.Judged || q.Score != -1 {
		t.Fatalf("无交付时质量维不该判定，got judged=%v score=%v", q.Judged, q.Score)
	}
	d := dim(card, domain.DimDelivery)
	if d.Judged {
		t.Fatalf("在单未到货不该判交付")
	}
	if d.Reason == "" {
		t.Fatal("判不了必须给理由，否则用户会当成 0 分")
	}
}

// 只判得出成本与交付时，总分按可判维度归一加权，而不是把其余三维当 0 分。
func TestScoreOrder_ReweightOnlyJudged(t *testing.T) {
	o := testOrder("closed")
	card := ScoreOrder(OrderScoreInput{
		Order: o, Profile: ProfileDefault, Benchmark: 100,
		Deliveries: []domain.Delivery{
			{ID: "d1", POID: "po-1", Qty: 1000, Accepted: 1000, OnTime: true,
				PromisedAt: "2026-03-20", ArrivedAt: "2026-03-20", LeadDays: 19, PromisedLeadDays: 19},
		},
	})
	c, d := dim(card, domain.DimCost), dim(card, domain.DimDelivery)
	if !c.Judged || !d.Judged {
		t.Fatal("有基准价与交付时，成本与交付都应判定")
	}
	// 成本 100（等于基准）+ 交付 100，总分应为 100，未被其余三维拉低
	if card.Total < 99 {
		t.Fatalf("可判维度都是满分时总分应接近 100，got %.1f", card.Total)
	}
	if card.JudgedWeight < 0.44 || card.JudgedWeight > 0.46 {
		t.Fatalf("可判权重应为成本 20%% 加交付 25%% 即 45%%，got %.2f", card.JudgedWeight)
	}
	// 权重不到一半不给评级：只凭两个维度下结论会误导
	if card.Grade != domain.GradeNA {
		t.Fatalf("可判权重 %.2f 低于 0.5，不应出评级，got %s", card.JudgedWeight, card.Grade)
	}
}

// 判退是硬伤：即使不良率很低，质量分也要封顶。
func TestScoreOrder_FailBatchCapsQuality(t *testing.T) {
	card := ScoreOrder(OrderScoreInput{
		Order: testOrder("closed"), Profile: ProfileDefault,
		Deliveries: []domain.Delivery{
			{ID: "d1", POID: "po-1", Qty: 1000, Accepted: 999, Rejected: 1, OnTime: true,
				PromisedAt: "2026-03-20", ArrivedAt: "2026-03-20", IQCResult: "fail"},
			{ID: "d2", POID: "po-1", Qty: 1000, Accepted: 1000, OnTime: true,
				PromisedAt: "2026-03-20", ArrivedAt: "2026-03-20", IQCResult: "pass"},
		},
	})
	q := dim(card, domain.DimQuality)
	if q.Score > 45 {
		t.Fatalf("出现判退批次时质量分应封顶 45，got %.1f", q.Score)
	}
}

// 让步接收要扣分但不能算不合格——这是之前踩过的坑。
func TestScoreOrder_ConditionalCountsAsPassWithPenalty(t *testing.T) {
	card := ScoreOrder(OrderScoreInput{
		Order: testOrder("closed"), Profile: ProfileDefault,
		Deliveries: []domain.Delivery{
			{ID: "d1", POID: "po-1", Qty: 1000, Accepted: 1000, OnTime: true,
				PromisedAt: "2026-03-20", ArrivedAt: "2026-03-20", IQCResult: "conditional"},
		},
	})
	q := dim(card, domain.DimQuality)
	// 让步接收仍算通过（不该被当成判退），但要明显低于一次做对的 100 分
	if q.Score < 85 || q.Score >= 100 {
		t.Fatalf("单批让步接收应判为合格但扣分（期望 85-99），got %.1f", q.Score)
	}
}

// 没有基准价就不判成本，绝不拿拍脑袋的基准去扣分。
func TestScoreOrder_NoBenchmark_CostNotJudged(t *testing.T) {
	card := ScoreOrder(OrderScoreInput{
		Order: testOrder("closed"), Profile: ProfileDefault, Benchmark: 0,
		Deliveries: []domain.Delivery{{ID: "d1", POID: "po-1", Qty: 10, Accepted: 10,
			OnTime: true, PromisedAt: "2026-03-20", ArrivedAt: "2026-03-20", IQCResult: "pass"}},
	})
	if dim(card, domain.DimCost).Judged {
		t.Fatal("无基准价时成本维不该判定")
	}
}

// 未填 IQC 结果的批次不能当合格，也不能拖低分数——直接判不了。
func TestScoreOrder_UnknownIQC_NotJudged(t *testing.T) {
	card := ScoreOrder(OrderScoreInput{
		Order: testOrder("closed"), Profile: ProfileDefault,
		Deliveries: []domain.Delivery{{ID: "d1", POID: "po-1", Qty: 100, Accepted: 100,
			OnTime: true, PromisedAt: "2026-03-20", ArrivedAt: "2026-03-20"}},
	})
	if dim(card, domain.DimQuality).Judged {
		t.Fatal("未记录 IQC 结果时不该判定质量")
	}
}

// 订单没有任何可判记录时总分为 -1，且必须给出说明。
func TestScoreOrder_NothingJudged_TotalMissing(t *testing.T) {
	card := ScoreOrder(OrderScoreInput{Order: testOrder("open"), Profile: ProfileDefault})
	if card.Total != -1 {
		t.Fatalf("无可判维度时总分应为 -1，got %.1f", card.Total)
	}
	if card.Note == "" {
		t.Fatal("必须说明为什么没分")
	}
}

// 批量加权：一批大单延误的影响要大于一批小单。
func TestScoreOrder_OnTimeWeightedByQty(t *testing.T) {
	card := ScoreOrder(OrderScoreInput{
		Order: testOrder("closed"), Profile: ProfileDefault,
		Deliveries: []domain.Delivery{
			{ID: "big", POID: "po-1", Qty: 900, Accepted: 900, OnTime: false,
				PromisedAt: "2026-03-20", ArrivedAt: "2026-03-28", LeadDays: 27, PromisedLeadDays: 19, IQCResult: "pass"},
			{ID: "small", POID: "po-1", Qty: 100, Accepted: 100, OnTime: true,
				PromisedAt: "2026-03-20", ArrivedAt: "2026-03-20", LeadDays: 19, PromisedLeadDays: 19, IQCResult: "pass"},
		},
	})
	d := dim(card, domain.DimDelivery)
	if d.Score > 55 {
		t.Fatalf("900/1000 的量延误，交付分应按批量加权后明显偏低，got %.1f", d.Score)
	}
}
