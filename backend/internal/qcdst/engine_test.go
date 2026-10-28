package qcdst

import (
	"testing"
	"time"

	"github.com/example/supplier-hub/backend/internal/domain"
)

func baseBundle() Bundle {
	return Bundle{
		Supplier: domain.Supplier{ID: "s1", Name: "测试精密制造有限公司", Capabilities: []string{"cnc"}},
		Tenant:   domain.Tenant{ID: "t1", Name: "测试采购方"},
	}
}

// 没有任何单据时，不能给出分数，必须明确"数据不足"。
func TestEmptyBundleIsNotScored(t *testing.T) {
	a := Compute(baseBundle())
	if a.Grade != domain.GradeNA {
		t.Fatalf("空数据应判 NA，实际 %s", a.Grade)
	}
	if !a.Provisional {
		t.Error("空数据应标记为观察期")
	}
	for _, d := range a.Dimensions {
		if d.Score != -1 {
			t.Errorf("维度 %s 无数据时应为 -1，实际 %.1f", d.Name, d.Score)
		}
	}
}

// 缺某一类单据时，该维度必须是 -1，而不是 0 分。
// 这是最容易犯的错：把"没录"当成"做得差"。
func TestMissingQuotesDoesNotZeroCost(t *testing.T) {
	b := baseBundle()
	for i := 0; i < 20; i++ {
		b.Deliveries = append(b.Deliveries, domain.Delivery{
			ID: "d", BatchNo: "B1", Qty: 1000, Accepted: 1000,
			OnTime: true, IQCResult: "pass", ArrivedAt: "2026-05-10",
		})
	}
	a := Compute(b)
	var cost domain.DimensionScore
	for _, d := range a.Dimensions {
		if d.Dimension == domain.DimCost {
			cost = d
		}
	}
	if cost.Score != -1 {
		t.Fatalf("无报价时成本维度应为 -1，实际 %.1f", cost.Score)
	}
	// 交付维度的数据是够的，必须算得出来
	for _, d := range a.Dimensions {
		if d.Dimension == domain.DimDelivery && d.Score < 90 {
			t.Errorf("全部准时交付应得高分，实际 %.1f", d.Score)
		}
	}
}

// 样本量不足时不给正式评级：10 个批次的高分不能当结论。
func TestSmallSampleIsProvisional(t *testing.T) {
	b := baseBundle()
	for i := 0; i < 8; i++ {
		b.Deliveries = append(b.Deliveries, domain.Delivery{
			ID: "d", BatchNo: "B1", Qty: 1000, Accepted: 1000,
			OnTime: true, IQCResult: "pass", ArrivedAt: "2026-05-10",
		})
	}
	a := Compute(b)
	if !a.Provisional {
		t.Error("8 个批次应标记观察期")
	}
	if a.Grade != domain.GradeNA {
		t.Errorf("样本不足不应出评级，实际 %s", a.Grade)
	}
	if a.Total < 90 {
		t.Errorf("观察期仍应保留参考分，实际 %.1f", a.Total)
	}
}

// 五项数据齐全且表现优秀时，应该拿到 A 以上。
//
// 交付日期刻意铺开在 4 个月里：40 个批次如果全挤在同一天，
// 哪怕分数再高也只是"一天的表现"，按新门槛仍是观察期。
func TestExcellentSupplierScoresHigh(t *testing.T) {
	b := baseBundle()
	start := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 40; i++ {
		arrived := start.AddDate(0, 0, i*3)
		promised := arrived.AddDate(0, 0, 5)
		b.Deliveries = append(b.Deliveries, domain.Delivery{
			ID: "d", BatchNo: "B", Qty: 2000, Accepted: 2000,
			OnTime: true, IQCResult: "pass", ArrivedAt: arrived.Format("2006-01-02"),
			LeadDays: 20, PromisedLeadDays: 25, PromisedAt: promised.Format("2006-01-02"),
		})
	}
	for i := 0; i < 4; i++ {
		b.Quotes = append(b.Quotes, domain.Quote{
			ID: "q", UnitPrice: 90, PreviousPrice: 100, BenchmarkPrice: 100,
			PaymentDays: 90, CostBreakdown: true,
		})
	}
	for i := 0; i < 4; i++ {
		b.Tickets = append(b.Tickets, domain.ServiceTicket{
			ID: "s", ResponseHours: 2, ResolveDays: 3,
		})
	}
	b.Reviews = append(b.Reviews, domain.TechReview{
		ID: "r", CapabilityCoverage: 100, AutomationLevel: 5, RndRatio: 18,
		NPIProjects: 10, NPISuccess: 10, CooperationScore: 95,
	})
	a := Compute(b)
	if a.Grade != domain.GradeA && a.Grade != domain.GradeAPlus {
		t.Fatalf("全优供应商应得 A 及以上，实际 %s（%.1f）", a.Grade, a.Total)
	}
	if a.Confidence < 0.9 {
		t.Errorf("数据齐全时置信度应接近 1，实际 %.2f", a.Confidence)
	}
}

// 每个指标都必须能说清分子分母，否则界面上"可追溯"就是空话。
func TestEveryMetricExposesEvidence(t *testing.T) {
	b := baseBundle()
	b.Deliveries = append(b.Deliveries, domain.Delivery{
		ID: "d1", BatchNo: "B1", Qty: 1000, Accepted: 900, Rejected: 100,
		OnTime: false, IQCResult: "fail", ArrivedAt: "2026-05-20", PromisedAt: "2026-05-10",
	})
	a := Compute(b)
	for _, d := range a.Dimensions {
		for _, m := range d.Metrics {
			if m.Formula == "" {
				t.Errorf("指标 %s 缺少计算公式", m.Code)
			}
			if m.Score >= 0 && m.Raw == "" {
				t.Errorf("指标 %s 有分数但没有原始值说明", m.Code)
			}
		}
	}
}
