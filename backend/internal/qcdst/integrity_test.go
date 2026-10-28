package qcdst

import (
	"testing"
	"time"

	"github.com/example/supplier-hub/backend/internal/domain"
)

func spanBundle(dates []string) Bundle {
	b := Bundle{Supplier: domain.Supplier{ID: "s1"}, Tenant: domain.Tenant{ID: "t1"}}
	for _, d := range dates {
		b.Deliveries = append(b.Deliveries, domain.Delivery{
			Qty: 1000, Accepted: 1000, OnTime: true, IQCResult: "pass",
			ArrivedAt: d, PromisedAt: d, LeadDays: 20, PromisedLeadDays: 20,
		})
	}
	return b
}

// 批次数够但全挤在一周内，仍不能出正式评级。
// 15 个批次集中在一周，那叫"一周的表现"，不叫"这家供应商的水平"。
func TestShortSpanStaysProvisional(t *testing.T) {
	dates := []string{}
	base := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 20; i++ {
		dates = append(dates, base.AddDate(0, 0, i%7).Format("2006-01-02"))
	}
	a := Compute(spanBundle(dates))
	if a.BatchCount != 20 {
		t.Fatalf("批次数应为 20，got %d", a.BatchCount)
	}
	if !a.Provisional {
		t.Fatal("20 个批次但跨度仅 7 天，应仍为观察期")
	}
	if a.Grade != domain.GradeNA {
		t.Fatalf("跨度不足不应出评级，got %s", a.Grade)
	}
	found := false
	for _, g := range a.Guardrails {
		if len(g) > 0 && (contains(g, "跨度")) {
			found = true
		}
	}
	if !found {
		t.Fatalf("护栏里应说明跨度问题，got %v", a.Guardrails)
	}
}

// 同样 20 批，铺开到 120 天，就应该拿正式评级。
func TestAdequateSpanGetsGrade(t *testing.T) {
	dates := []string{}
	base := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 20; i++ {
		dates = append(dates, base.AddDate(0, 0, i*6).Format("2006-01-02"))
	}
	a := Compute(spanBundle(dates))
	if a.DataSpanDays < 90 {
		t.Fatalf("跨度应 ≥90 天，got %d", a.DataSpanDays)
	}
	if a.Provisional {
		t.Fatalf("20 批且跨度 %d 天，不该再是观察期", a.DataSpanDays)
	}
}

// 时间衰减：一年半前的单据权重必须明显低于一个月前的。
// 三年前的好成绩不该和昨天的差成绩等权。
func TestRecencyWeightDecays(t *testing.T) {
	now := time.Now()
	recent := RecencyWeight(now.AddDate(0, 0, -30).Format("2006-01-02"), 365)
	old := RecencyWeight(now.AddDate(0, 0, -365).Format("2006-01-02"), 365)
	veryOld := RecencyWeight(now.AddDate(0, 0, -730).Format("2006-01-02"), 365)
	if recent < 0.9 {
		t.Fatalf("30 天前权重应接近 1，got %.2f", recent)
	}
	if old > 0.55 || old < 0.45 {
		t.Fatalf("365 天前权重应约 0.5，got %.2f", old)
	}
	if veryOld > 0.3 {
		t.Fatalf("730 天前权重应约 0.25，got %.2f", veryOld)
	}
	if !(recent > old && old > veryOld) {
		t.Fatalf("权重应随时间单调下降：%.2f / %.2f / %.2f", recent, old, veryOld)
	}
	// 未来日期与不可解析日期一律压到最低，不该因为录错反而拿到高权重
	if w := RecencyWeight(now.AddDate(0, 0, 30).Format("2006-01-02"), 365); w > 0.2 {
		t.Fatalf("未来日期应按最低权重处理，got %.2f", w)
	}
	if w := RecencyWeight("", 365); w > 0.2 {
		t.Fatalf("空日期应按最低权重处理，got %.2f", w)
	}
}

// 趋势判定：3 分以内算噪声，不算趋势。
func TestTrendOfThreshold(t *testing.T) {
	if TrendOf(95, 90) != "up" {
		t.Error("差 +5 应判定为在变好")
	}
	if TrendOf(88, 90) != "flat" {
		t.Error("差 -2 属于噪声，应判定持平")
	}
	if TrendOf(82, 90) != "down" {
		t.Error("差 -8 应判定为在变差")
	}
	if TrendOf(-1, 90) != "unknown" {
		t.Error("近期无数据时不应编造趋势")
	}
}

// 数据体检对干净数据不应报假警。
func TestIntegrityCleanData(t *testing.T) {
	dates := []string{}
	base := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 20; i++ {
		dates = append(dates, base.AddDate(0, 0, i*6).Format("2006-01-02"))
	}
	q := CheckIntegrity(spanBundle(dates).Deliveries)
	if q.Level != "ok" {
		t.Fatalf("干净数据不应告警，got level=%s issues=%+v", q.Level, q.Issues)
	}
	if q.Score != 100 {
		t.Fatalf("干净数据完整度应为 100，got %.1f", q.Score)
	}
}

// 收货数与判定数对不上要报出来——要么判定没录完，要么单据录错。
func TestIntegrityBadBalance(t *testing.T) {
	q := CheckIntegrity([]domain.Delivery{
		{Qty: 1000, Accepted: 500, Rejected: 100, ArrivedAt: "2026-05-01", PromisedAt: "2026-05-01", OnTime: true, IQCResult: "pass"},
	})
	if q.BadBalance != 1 {
		t.Fatalf("应识别出 1 条对不上的批次，got %d", q.BadBalance)
	}
	if q.Level != "warn" {
		t.Fatalf("应为 warn，got %s", q.Level)
	}
}

// 收货数为 0 的批次：不参与合格率，却虚增批次数，而批次数是评级门槛。
func TestIntegrityZeroQtyIsBad(t *testing.T) {
	q := CheckIntegrity([]domain.Delivery{
		{Qty: 0, ArrivedAt: "2026-05-01", PromisedAt: "2026-05-01", OnTime: true, IQCResult: "pass"},
		{Qty: 1000, Accepted: 1000, ArrivedAt: "2026-05-02", PromisedAt: "2026-05-02", OnTime: true, IQCResult: "pass"},
	})
	if q.ZeroQtyBatch != 1 || q.Level != "bad" {
		t.Fatalf("空批量应判为严重问题，got zero=%d level=%s", q.ZeroQtyBatch, q.Level)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
