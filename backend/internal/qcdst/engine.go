// Package qcdst 把 SRM 里的原始单据算成 QCDST 五维分数。
//
// 这里刻意不做"经验加权黑箱"：每个指标都写明分子、分母、样本量和公式，
// 算不出来就标 -1（数据不足），让界面如实显示"证据不够"，而不是给个 70 分蒙过去。
package qcdst

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/example/supplier-hub/backend/internal/domain"
)

// Bundle 一次评估所需的全部原始数据。
type Bundle struct {
	Supplier   domain.Supplier
	Tenant     domain.Tenant
	Orders     []domain.PurchaseOrder
	Deliveries []domain.Delivery
	Events     []domain.QualityEvent
	Quotes     []domain.Quote
	Tickets    []domain.ServiceTicket
	Reviews    []domain.TechReview
	Demo       bool
}

// Profile 维度权重方案。不同品类的短板不一样：
// 工业装备里技术能力差一点就根本进不来，所以 T 的权重必须抬高；
// 消费电子拼的是交付与成本，Q/D 权重要保住。
type Profile struct {
	Name   string
	Weight map[domain.Dimension]float64
}

var ProfileDefault = Profile{
	Name: "通用",
	Weight: map[domain.Dimension]float64{
		domain.DimQuality: 0.30, domain.DimCost: 0.20, domain.DimDelivery: 0.25,
		domain.DimService: 0.15, domain.DimTechnology: 0.10,
	},
}

var ProfileIndustrial = Profile{
	Name: "工业装备",
	Weight: map[domain.Dimension]float64{
		domain.DimQuality: 0.30, domain.DimCost: 0.15, domain.DimDelivery: 0.22,
		domain.DimService: 0.13, domain.DimTechnology: 0.20,
	},
}

var ProfileConsumer = Profile{
	Name: "消费电子",
	Weight: map[domain.Dimension]float64{
		domain.DimQuality: 0.28, domain.DimCost: 0.24, domain.DimDelivery: 0.26,
		domain.DimService: 0.12, domain.DimTechnology: 0.10,
	},
}

// industrialCaps 判定"工业装备"权重方案的关键工艺。
var industrialCaps = map[string]bool{
	"cnc": true, "diecasting": true, "stamping": true,
	"welding": true, "motor": true, "mold": true,
}

// ProfileFor 按供应商的能力/品类挑选权重方案。
//
// 门槛刻意抬高到"多数工艺都是工业类，或至少命中两类"：
// 权重方案直接决定总分（工业方案把 T 从 10% 抬到 20%、C 从 20% 降到 15%）。
// 如果一家供应商只要在自己档案里加个 cnc 标签就能切到对自己有利的方案，
// 那等于把权重交给了被评估方自己——这是最容易被钻的空子。
func ProfileFor(s domain.Supplier) Profile {
	if len(s.Capabilities) == 0 {
		return ProfileDefault
	}
	hits := 0
	for _, c := range s.Capabilities {
		if industrialCaps[c] {
			hits++
		}
	}
	// 至少命中两类，且工业工艺占比过半，才切工业方案。
	if hits >= 2 && float64(hits)/float64(len(s.Capabilities)) >= 0.5 {
		return ProfileIndustrial
	}
	return ProfileDefault
}

var dimNames = map[domain.Dimension]string{
	domain.DimQuality: "质量 Q", domain.DimCost: "成本 C", domain.DimDelivery: "交付 D",
	domain.DimService: "服务 S", domain.DimTechnology: "技术 T",
}

// minBatchesForGrade 出正式评级所需的最少交付批次。
const minBatchesForGrade = 15

// qualityCapOnOpenCritical 存在未闭环停线/召回级事件时，质量维度的封顶分。
const qualityCapOnOpenCritical = 70

const (
	gradeAPlusMin = 90
	gradeAMin     = 80
	gradeBMin     = 70
	gradeCMin     = 60
)

func gradeOf(total float64, confidence float64) domain.Grade {
	if confidence < 0.35 {
		return domain.GradeNA
	}
	switch {
	case total >= gradeAPlusMin:
		return domain.GradeAPlus
	case total >= gradeAMin:
		return domain.GradeA
	case total >= gradeBMin:
		return domain.GradeB
	case total >= gradeCMin:
		return domain.GradeC
	default:
		return domain.GradeD
	}
}

func clamp(v, lo, hi float64) float64 {
	if math.IsNaN(v) {
		return lo
	}
	return math.Max(lo, math.Min(hi, v))
}

// metric 是内部计算中间态。
type metric struct {
	code        string
	name        string
	dim         domain.Dimension
	unit        string
	weight      float64
	score       float64
	raw         string
	numerator   float64
	denominator float64
	sample      int
	minSample   int
	formula     string
	benchmark   string
	evidence    []string
	auto        bool
}

func (m metric) toScore() domain.MetricScore {
	conf := 0.0
	if m.minSample > 0 {
		conf = clamp(float64(m.sample)/float64(m.minSample), 0, 1)
	}
	// score 已在 Compute 里统一判定过（无样本 = -1），这里不再二次判断，
	// 否则聚合口径和展示口径会打架。
	score := m.score
	ev := m.evidence
	if ev == nil {
		ev = []string{}
	}
	return domain.MetricScore{
		Code: m.code, Name: m.name, Unit: m.unit, Weight: m.weight,
		Score: round1(score), Raw: m.raw, Numerator: round1(m.numerator),
		Denominator: round1(m.denominator), Sample: m.sample, MinSample: m.minSample,
		Confidence: round2(conf), Formula: m.formula, Benchmark: m.benchmark,
		Evidence: ev, Auto: m.auto,
	}
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
func round2(v float64) float64 { return math.Round(v*100) / 100 }

// evidenceOf 取最近几条单据号，供界面"点开看原始记录"。
func evidenceOf(list []string, n int) []string {
	if len(list) == 0 {
		return nil
	}
	if len(list) > n {
		list = list[:n]
	}
	out := make([]string, len(list))
	copy(out, list)
	return out
}

func sortedDesc(values []string) []string {
	sort.Sort(sort.Reverse(sort.StringSlice(values)))
	return values
}

// recentWindowDays 近期窗口。采购决策看的是"最近半年行不行"，
// 半年的窗口足够抹掉单月波动，又不至于慢到看不出恶化。
const recentWindowDays = 180

// staleDays 超过这么久没有新交付，评级只当作历史档案。
const staleDays = 365

// minSpanDays 出正式评级所需的最短数据跨度。
// 15 个批次如果全发生在同一周，那是"一周的表现"，不是"一家供应商的水平"。
const minSpanDays = 90

// Compute 计算一份评估。
func Compute(b Bundle) domain.QCDSTAssessment {
	return computeAssess(b, true)
}

// computeAssess 内部实现。includeRecent 控制是否再算一次近期分，
// 传 false 用于递归调用，避免无限展开。
func computeAssess(b Bundle, includeRecent bool) domain.QCDSTAssessment {
	profile := ProfileFor(b.Supplier)
	metrics := []metric{}
	metrics = append(metrics, qualityMetrics(b)...)
	metrics = append(metrics, costMetrics(b)...)
	metrics = append(metrics, deliveryMetrics(b)...)
	metrics = append(metrics, serviceMetrics(b)...)
	metrics = append(metrics, techMetrics(b)...)

	// 先把"没有样本"的指标统一判为无效（-1），后续聚合与展示都读同一个值。
	for i := range metrics {
		if metrics[i].sample == 0 {
			metrics[i].score = -1
		}
	}

	// 未闭环的停线/召回级事件：质量维度封顶。
	//
	// 为什么用"封顶"而不是"多扣几分"：按次扣分的写法会制造逆向激励——
	// 记录得越认真的采购方，自家供应商分越低；而瞒报的反而得高分。
	// 封顶是绝对值约束，与记录多少无关：只要有一笔没闭环的停线事故，
	// 质量再多的漂亮数据也不该把这一维抬过 70 分。
	criticalOpen := 0
	for _, e := range b.Events {
		if e.Severity == "critical" && e.ClosedAt == "" {
			criticalOpen++
		}
	}

	dims := []domain.Dimension{domain.DimQuality, domain.DimCost, domain.DimDelivery, domain.DimService, domain.DimTechnology}
	dimScores := make([]domain.DimensionScore, 0, len(dims))
	var totalNum, totalDen float64

	for _, d := range dims {
		sub := filterDim(metrics, d)
		normalizeWeights(sub)
		totalW := 0.0
		validW := 0.0
		weighted := 0.0
		confNum := 0.0
		for _, m := range sub {
			totalW += m.weight
			if m.score >= 0 {
				validW += m.weight
				weighted += m.score * m.weight
				confNum += m.weight * clamp(float64(m.sample)/float64(maxInt(m.minSample, 1)), 0, 1)
			}
		}
		score := -1.0
		if validW > 0 {
			score = weighted / validW
		}
		conf := 0.0
		if validW > 0 {
			conf = confNum / validW
		}
		coverage := 0.0
		if totalW > 0 {
			coverage = validW / totalW
		}
		scores := make([]domain.MetricScore, 0, len(sub))
		for _, m := range sub {
			scores = append(scores, m.toScore())
		}
		if d == domain.DimQuality && criticalOpen > 0 && score > qualityCapOnOpenCritical {
			score = qualityCapOnOpenCritical
		}
		ds := domain.DimensionScore{
			Dimension: d, Name: dimNames[d], Weight: profile.Weight[d],
			Score: round1(score), Confidence: round2(conf), Coverage: round2(coverage),
			Metrics: scores,
		}
		ds.Summary = dimSummary(d, score, coverage, sub)
		if d == domain.DimQuality && criticalOpen > 0 {
			ds.Summary += fmt.Sprintf("；有 %d 起未闭环的停线/召回级事件，本维封顶 %d 分", criticalOpen, qualityCapOnOpenCritical)
		}
		dimScores = append(dimScores, ds)

		if score >= 0 {
			// 维度权重 × 该维覆盖率再归一：证据不全的维度不该拿满权重。
			w := profile.Weight[d] * coverage
			totalNum += score * w
			totalDen += w
		}
	}

	total := -1.0
	if totalDen > 0 {
		total = totalNum / totalDen
	}
	confidence := 0.0
	confWeight := 0.0
	for _, ds := range dimScores {
		confidence += ds.Confidence * profile.Weight[ds.Dimension] * ds.Coverage
		confWeight += profile.Weight[ds.Dimension] * ds.Coverage
	}
	if confWeight > 0 {
		confidence /= confWeight
	}

	assess := domain.QCDSTAssessment{
		SupplierID: b.Supplier.ID, SupplierName: b.Supplier.Name, ShortName: b.Supplier.ShortName,
		CreditCode: b.Supplier.CreditCode, TenantID: b.Tenant.ID, TenantName: b.Tenant.Name,
		Total: round1(total), Grade: gradeOf(total, confidence),
		Confidence: round2(confidence), Coverage: round2(avgCoverage(dimScores)),
		Dimensions: dimScores, Demo: b.Demo,
		Window:     windowOf(b),
		OrderCount: len(b.Orders), BatchCount: len(b.Deliveries), QualityEvents: len(b.Events),
		GeneratedAt: time.Now().Format("2006-01-02 15:04"),
	}
	assess.History = historyOf(b)

	// ---- 数据体检与趋势 ----
	assess.Integrity = CheckIntegrity(b.Deliveries)
	assess.DataSpanDays = DataSpanDays(b.Deliveries)
	assess.RecentTotal = -1 // 窗口内无交付时保持 -1，与"0 分"区分开
	if includeRecent {
		recent := recentBundle(b, recentWindowDays)
		if len(recent.Deliveries) > 0 {
			assess.RecentTotal = computeAssess(recent, false).Total
		}
	}
	assess.Trend = TrendOf(assess.RecentTotal, assess.Total)
	last := LastDeliveryDate(b.Deliveries)
	assess.Stale = DaysSince(last) > staleDays

	// ---- 观察期：批次数与数据跨度两项都要够 ----
	// 只看批次数的话，一周内到 15 个批次就能拿正式评级，
	// 而那只是"一周的表现"，不是这家供应商的稳定水平。
	spanShort := len(b.Deliveries) > 0 && assess.DataSpanDays > 0 && assess.DataSpanDays < minSpanDays
	if len(b.Deliveries) < minBatchesForGrade || spanShort {
		assess.Provisional = true
		assess.Grade = domain.GradeNA
		if spanShort && len(b.Deliveries) >= minBatchesForGrade {
			assess.Insights = append(assess.Insights, domain.Insight{
				Kind: "short_span", Level: "info", Title: "数据跨度不足，暂不评级",
				Detail: fmt.Sprintf("已有 %d 个批次，但全部集中在 %d 天内（要求 ≥ %d 天）。短期密集交付通常是爬坡或赶工，不代表稳定水平，继续观察至 90 天后再评级。",
					len(b.Deliveries), assess.DataSpanDays, minSpanDays),
			})
		} else {
			assess.Insights = append(assess.Insights, domain.Insight{
				Kind: "provisional", Level: "info", Title: "观察期，暂不评级",
				Detail: fmt.Sprintf("目前仅有 %d 个交付批次，低于评级所需的 %d 个。当前分数（%.1f）仅供参考，继续累积 2-3 个月单据后再评级。",
					len(b.Deliveries), minBatchesForGrade, assess.Total),
			})
		}
	}
	assess.Guardrails = guardrailsOf(assess)
	if criticalOpen > 0 {
		assess.Insights = append(assess.Insights, domain.Insight{
			Kind: "critical_open", Level: "critical",
			Title:  fmt.Sprintf("%d 起停线/召回级事件尚未闭环", criticalOpen),
			Detail: fmt.Sprintf("质量维度已封顶 %d 分。在整改关闭前，不建议扩大份额或导入新项目。", qualityCapOnOpenCritical),
		})
	}
	assess.Insights = append(buildInsights(assess, b), assess.Insights...)
	return assess
}

// guardrailsOf 把触发的护栏逐条写成人话。界面照原样显示，
// 让看分数的人知道这个分数带什么附加条件。
func guardrailsOf(a domain.QCDSTAssessment) []string {
	out := []string{}
	if a.Provisional {
		out = append(out, fmt.Sprintf("观察期：%d 个批次 / 跨度 %d 天，未达评级门槛（%d 批、%d 天）",
			a.BatchCount, a.DataSpanDays, minBatchesForGrade, minSpanDays))
	}
	if a.Stale {
		out = append(out, "数据已过期：超过 365 天没有新交付记录，分数只代表历史")
	}
	if a.Trend == "down" {
		out = append(out, fmt.Sprintf("近期在下滑：近半年 %.1f 分，低于历史 %.1f 分", a.RecentTotal, a.Total))
	}
	if a.Trend == "up" {
		out = append(out, fmt.Sprintf("近期在改善：近半年 %.1f 分，高于历史 %.1f 分", a.RecentTotal, a.Total))
	}
	for _, i := range a.Integrity.Issues {
		if i.Level == "bad" {
			out = append(out, "数据问题："+i.Title)
		}
	}
	if a.Integrity.SmallBatchRate >= 30 {
		out = append(out, fmt.Sprintf("小样批占比 %.0f%%，批次合格率可能被垫高", a.Integrity.SmallBatchRate))
	}
	if len(out) == 0 {
		out = append(out, "未触发护栏：样本量、数据跨度、单据自洽性均达标")
	}
	return out
}

// recentBundle 截取近期窗口内的单据。窗口外的一律剔除，
// 不留"部分计入"这种说不清的口径。
func recentBundle(b Bundle, days int) Bundle {
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	out := Bundle{Supplier: b.Supplier, Tenant: b.Tenant, Demo: b.Demo}
	for _, d := range b.Deliveries {
		if d.ArrivedAt >= cutoff {
			out.Deliveries = append(out.Deliveries, d)
		}
	}
	for _, o := range b.Orders {
		if o.IssuedAt >= cutoff {
			out.Orders = append(out.Orders, o)
		}
	}
	for _, e := range b.Events {
		if e.OccurredAt >= cutoff {
			out.Events = append(out.Events, e)
		}
	}
	for _, q := range b.Quotes {
		if q.QuotedAt >= cutoff {
			out.Quotes = append(out.Quotes, q)
		}
	}
	for _, t := range b.Tickets {
		if t.OpenedAt >= cutoff {
			out.Tickets = append(out.Tickets, t)
		}
	}
	for _, r := range b.Reviews {
		if r.ReviewedAt >= cutoff {
			out.Reviews = append(out.Reviews, r)
		}
	}
	return out
}

func filterDim(metrics []metric, d domain.Dimension) []metric {
	out := make([]metric, 0, 4)
	for _, m := range metrics {
		if m.dim == d {
			out = append(out, m)
		}
	}
	return out
}

// normalizeWeights 把维度内权重归一，保证权重调整后仍然总和为 1。
func normalizeWeights(list []metric) {
	sum := 0.0
	for i := range list {
		sum += list[i].weight
	}
	if sum <= 0 {
		return
	}
	for i := range list {
		list[i].weight = round2(list[i].weight / sum)
	}
}

func avgCoverage(dims []domain.DimensionScore) float64 {
	if len(dims) == 0 {
		return 0
	}
	sum := 0.0
	for _, d := range dims {
		sum += d.Coverage
	}
	return sum / float64(len(dims))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func dimSummary(d domain.Dimension, score, coverage float64, metrics []metric) string {
	if score < 0 {
		return "证据不足，暂不评分"
	}
	label := "良好"
	switch {
	case score >= 90:
		label = "优秀"
	case score >= 80:
		label = "良好"
	case score >= 70:
		label = "达标"
	case score >= 60:
		label = "偏弱"
	default:
		label = "不达标"
	}
	// 找出拖后腿的那一项，比"整体良好"有用得多。
	worst := metric{}
	worstScore := 101.0
	for _, m := range metrics {
		if m.score >= 0 && m.score < worstScore {
			worst, worstScore = m, m.score
		}
	}
	tail := ""
	if worstScore <= 75 {
		tail = fmt.Sprintf("，主要拖累项是%s（%.0f 分）", worst.name, worstScore)
	}
	cov := ""
	if coverage < 0.75 {
		cov = fmt.Sprintf("；仅 %.0f%% 的指标有足够样本", coverage*100)
	}
	return fmt.Sprintf("%s%s%s", label, tail, cov)
}

func windowOf(b Bundle) string {
	dates := []string{}
	for _, d := range b.Deliveries {
		if d.ArrivedAt != "" {
			dates = append(dates, d.ArrivedAt)
		}
	}
	if len(dates) == 0 {
		return "无交付记录"
	}
	sort.Strings(dates)
	return dates[0] + " ~ " + dates[len(dates)-1]
}

// historyOf 按月重算总分，看这家供应商是在变好还是变差。
func historyOf(b Bundle) []domain.ScorePoint {
	months := map[string]Bundle{}
	for _, d := range b.Deliveries {
		if len(d.ArrivedAt) < 7 {
			continue
		}
		m := d.ArrivedAt[:7]
		sub := months[m]
		sub.Deliveries = append(sub.Deliveries, d)
		months[m] = sub
	}
	for _, e := range b.Events {
		if len(e.OccurredAt) < 7 {
			continue
		}
		m := e.OccurredAt[:7]
		sub := months[m]
		sub.Events = append(sub.Events, e)
		months[m] = sub
	}
	for _, o := range b.Orders {
		if len(o.IssuedAt) < 7 {
			continue
		}
		m := o.IssuedAt[:7]
		sub := months[m]
		sub.Orders = append(sub.Orders, o)
		months[m] = sub
	}
	keys := make([]string, 0, len(months))
	for k := range months {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > 6 {
		keys = keys[len(keys)-6:]
	}
	out := make([]domain.ScorePoint, 0, len(keys))
	for _, k := range keys {
		sub := months[k]
		sub.Supplier = b.Supplier
		sub.Tenant = b.Tenant
		sub.Demo = b.Demo
		// 只算 Q 与 D 两维，避免月份薄样本把总分带偏。
		ms := append(qualityMetrics(sub), deliveryMetrics(sub)...)
		weighted, weight := 0.0, 0.0
		for _, m := range ms {
			if m.score >= 0 && m.sample >= m.minSample {
				weighted += m.score * m.weight
				weight += m.weight
			}
		}
		if weight == 0 {
			continue
		}
		out = append(out, domain.ScorePoint{
			Period: k, Total: round1(weighted / weight),
			Samples: len(sub.Deliveries) + len(sub.Events),
		})
	}
	return out
}

func buildInsights(a domain.QCDSTAssessment, b Bundle) []domain.Insight {
	insights := []domain.Insight{}
	if a.Grade == domain.GradeNA {
		insights = append(insights, domain.Insight{
			Kind: "insufficient", Level: "warn", Title: "证据不足，暂不出评级",
			Detail: fmt.Sprintf("目前只有 %d 张订单、%d 个交付批次（跨度 %d 天）、%d 起质量事件，覆盖度 %.0f%%。评级门槛是 %d 个批次且数据跨度 ≥ %d 天，否则等于用噪声下结论。",
				a.OrderCount, a.BatchCount, a.DataSpanDays, a.QualityEvents, a.Coverage*100,
				minBatchesForGrade, minSpanDays),
		})
	}
	for _, d := range a.Dimensions {
		if d.Score >= 0 && d.Score < 65 && d.Coverage >= 0.6 {
			insights = append(insights, domain.Insight{
				Kind: strings.ToLower(string(d.Dimension)) + "_weak", Level: "warn",
				Title:  fmt.Sprintf("%s 是短板（%.0f 分）", d.Name, d.Score),
				Detail: d.Summary,
			})
		}
	}
	if a.Demo {
		insights = append(insights, domain.Insight{
			Kind: "demo", Level: "info", Title: "演示数据",
			Detail: "当前评估基于演示用 SRM 单据，不是真实业务数据，不能作为采购决策依据。",
		})
	}
	return insights
}
