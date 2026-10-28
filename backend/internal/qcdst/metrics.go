package qcdst

import (
	"fmt"
	"math"

	"github.com/example/supplier-hub/backend/internal/domain"
)

// 每个指标都写明：分子是什么、分母是什么、样本从哪来、多少条才算数。
// 这些文字会原样出现在界面上，采购员能自己复核，不用信"系统算的"。

func qualityMetrics(b Bundle) []metric {
	out := make([]metric, 0, 5)

	// ---- 批次合格率 ----
	pass, total, conditional := 0, 0, 0
	evidence := []string{}
	for _, d := range b.Deliveries {
		if d.IQCResult == "" {
			continue
		}
		total++
		// 让步接收的批次物料已入库使用，按行业口径计入合格，
		// 但必须单独披露批次数——否则会把"勉强能用"算成"质量很好"。
		if d.IQCResult == "pass" || d.IQCResult == "conditional" {
			pass++
			if d.IQCResult == "conditional" {
				conditional++
			}
		} else {
			evidence = append(evidence, d.BatchNo)
		}
	}
	m := metric{
		code: "q.batch_pass", name: "来料批次合格率", dim: domain.DimQuality, unit: "%",
		weight: 0.30, minSample: 10, auto: true, sample: total,
		numerator: float64(pass), denominator: float64(total),
		formula: "IQC 判定合格（含让步接收）的批次数 ÷ 收货总批次数", benchmark: "行业参考 ≥ 98%",
		evidence: evidenceOf(sortedDesc(evidence), 5),
	}
	if total > 0 {
		m.score = float64(pass) / float64(total) * 100
		m.raw = fmt.Sprintf("%d/%d 批 · %.1f%%", pass, total, m.score)
		if conditional > 0 {
			m.raw += fmt.Sprintf("（含让步接收 %d 批）", conditional)
		}
	} else {
		m.raw = "无 IQC 记录"
	}
	out = append(out, m)

	// ---- 来料不良率 PPM ----
	rejected, received := 0, 0
	for _, d := range b.Deliveries {
		rejected += d.Rejected
		received += d.Qty
	}
	m = metric{
		code: "q.ppm", name: "来料不良率", dim: domain.DimQuality, unit: "PPM",
		weight: 0.25, minSample: 5000, auto: true, sample: received,
		numerator: float64(rejected), denominator: float64(received),
		formula: "不良件数 ÷ 收货总件数 × 1,000,000", benchmark: "优秀 < 500 PPM",
	}
	if received > 0 {
		ppm := float64(rejected) / float64(received) * 1e6
		m.score = clamp(100*math.Exp(-ppm/1500), 0, 100)
		m.raw = fmt.Sprintf("%s PPM（不良 %d / 收货 %d 件）", thousands(int64(ppm)), rejected, received)
	} else {
		m.raw = "无收货记录"
	}
	out = append(out, m)

	// ---- 首件一次通过率 ----
	faiTotal, faiPass := 0, 0
	for _, d := range b.Deliveries {
		if !d.FirstArticle {
			continue
		}
		faiTotal++
		if d.FAIPass {
			faiPass++
		}
	}
	m = metric{
		code: "q.fai", name: "首件一次通过率", dim: domain.DimQuality, unit: "%",
		weight: 0.15, minSample: 3, auto: true, sample: faiTotal,
		numerator: float64(faiPass), denominator: float64(faiTotal),
		formula: "首件检验一次通过次数 ÷ 首件检验总次数", benchmark: "参考 ≥ 95%",
	}
	if faiTotal > 0 {
		m.score = float64(faiPass) / float64(faiTotal) * 100
		m.raw = fmt.Sprintf("%d/%d 次 · %.0f%%", faiPass, faiTotal, m.score)
	} else {
		m.raw = "无首件记录"
	}
	out = append(out, m)

	// ---- 8D 按期闭环率 ----
	closed, due := 0, 0
	openEvidence := []string{}
	for _, e := range b.Events {
		if e.DueAt == "" {
			continue
		}
		due++
		if e.ClosedAt != "" && e.ClosedAt <= e.DueAt {
			closed++
		} else {
			openEvidence = append(openEvidence, e.ID)
		}
	}
	m = metric{
		code: "q.8d", name: "整改按期闭环率", dim: domain.DimQuality, unit: "%",
		weight: 0.20, minSample: 3, auto: true, sample: due,
		numerator: float64(closed), denominator: float64(due),
		formula:   "在承诺期限（DueAt）内关闭的质量事件数 ÷ 有承诺期限的事件数",
		benchmark: "参考 ≥ 90%", evidence: evidenceOf(sortedDesc(openEvidence), 5),
	}
	if due > 0 {
		m.score = float64(closed) / float64(due) * 100
		m.raw = fmt.Sprintf("%d/%d 起按期关闭", closed, due)
	} else {
		m.raw = "无带期限的整改单"
	}
	out = append(out, m)

	// ---- 重大质量事故（按次扣分）----
	critical, major, minor := 0, 0, 0
	sevEvidence := []string{}
	for _, e := range b.Events {
		switch e.Severity {
		case "critical":
			critical++
			sevEvidence = append(sevEvidence, e.ID)
		case "major":
			major++
			sevEvidence = append(sevEvidence, e.ID)
		default:
			minor++
		}
	}
	m = metric{
		code: "q.incident", name: "重大质量事故", dim: domain.DimQuality, unit: "次",
		// minSample 提到 3：只记录到 1-2 起事件时样本太薄，
		// 按次扣分的指标在薄样本上置信度会被高估。
		weight: 0.10, minSample: 3, auto: true, sample: len(b.Events),
		numerator: float64(critical), denominator: float64(len(b.Events)),
		formula:   "100 − 停线/召回级事件 × 25 − 重大事件 × 8（下限 0）",
		benchmark: "0 次停线", evidence: evidenceOf(sortedDesc(sevEvidence), 5),
	}
	if len(b.Events) > 0 {
		m.score = clamp(100-float64(critical)*25-float64(major)*8, 0, 100)
		m.raw = fmt.Sprintf("停线 %d 起 · 重大 %d 起 · 一般 %d 起", critical, major, minor)
	} else {
		m.raw = "无质量事件"
	}
	out = append(out, m)
	return out
}

func costMetrics(b Bundle) []metric {
	out := make([]metric, 0, 4)

	// ---- 价格竞争力 ----
	sumRatio, count := 0.0, 0
	for _, q := range b.Quotes {
		if q.BenchmarkPrice <= 0 || q.UnitPrice <= 0 {
			continue
		}
		sumRatio += q.UnitPrice / q.BenchmarkPrice
		count++
	}
	m := metric{
		code: "c.price", name: "价格竞争力", dim: domain.DimCost, unit: "分",
		weight: 0.35, minSample: 3, auto: true, sample: count,
		numerator: sumRatio, denominator: float64(maxInt(count, 1)),
		formula:   "报价 ÷ 同品类中位价，偏离 1% 扣 2 分（100 分 = 等于或低于中位价）",
		benchmark: "对标同品类中位价",
	}
	if count > 0 {
		ratio := sumRatio / float64(count)
		m.score = clamp(100-(ratio-1)*200, 0, 100)
		m.raw = fmt.Sprintf("均价为中位价的 %.1f%%", ratio*100)
	} else {
		m.raw = "无报价记录"
	}
	out = append(out, m)

	// ---- 年度降本 ----
	sum, n := 0.0, 0
	for _, q := range b.Quotes {
		if q.PreviousPrice <= 0 || q.UnitPrice <= 0 {
			continue
		}
		sum += (q.PreviousPrice - q.UnitPrice) / q.PreviousPrice
		n++
	}
	m = metric{
		code: "c.yoy", name: "年度降本幅度", dim: domain.DimCost, unit: "%",
		weight: 0.25, minSample: 3, auto: true, sample: n,
		formula:   "（上轮价 − 本轮价）÷ 上轮价，5% 降幅得满分，零降本得 50 分",
		benchmark: "参考年降 3–5%",
	}
	if n > 0 {
		yoy := sum / float64(n)
		m.score = clamp(50+yoy/0.05*50, 0, 100)
		m.raw = fmt.Sprintf("平均降本 %.1f%%", yoy*100)
		m.numerator = yoy * 100
		m.denominator = 100
	} else {
		m.raw = "无历史报价可比"
	}
	out = append(out, m)

	// ---- 成本透明度 ----
	withBreakdown := 0
	for _, q := range b.Quotes {
		if q.CostBreakdown {
			withBreakdown++
		}
	}
	m = metric{
		code: "c.breakdown", name: "成本拆分意愿", dim: domain.DimCost, unit: "%",
		weight: 0.20, minSample: 3, auto: true, sample: len(b.Quotes),
		numerator: float64(withBreakdown), denominator: float64(len(b.Quotes)),
		formula:   "提供材料/工艺/人工拆分的报价数 ÷ 报价总数",
		benchmark: "愿拆成本者议价空间更透明",
	}
	if len(b.Quotes) > 0 {
		m.score = float64(withBreakdown) / float64(len(b.Quotes)) * 100
		m.raw = fmt.Sprintf("%d/%d 份报价含拆分", withBreakdown, len(b.Quotes))
	} else {
		m.raw = "无报价记录"
	}
	out = append(out, m)

	// ---- 付款条件 ----
	daysSum, dn := 0, 0
	for _, q := range b.Quotes {
		if q.PaymentDays > 0 {
			daysSum += q.PaymentDays
			dn++
		}
	}
	m = metric{
		code: "c.payment", name: "账期水平", dim: domain.DimCost, unit: "天",
		weight: 0.20, minSample: 2, auto: true, sample: dn,
		numerator: float64(daysSum), denominator: float64(maxInt(dn, 1)),
		formula: "平均账期 ÷ 90 天 × 100（90 天记满分）", benchmark: "行业常见 60–90 天",
	}
	if dn > 0 {
		avg := float64(daysSum) / float64(dn)
		m.score = clamp(avg/90*100, 0, 100)
		m.raw = fmt.Sprintf("平均 %.0f 天", avg)
	} else {
		m.raw = "无账期数据"
	}
	out = append(out, m)
	return out
}

func deliveryMetrics(b Bundle) []metric {
	out := make([]metric, 0, 4)

	// ---- 准时交付率 ----
	onTime, total := 0, 0
	late := []string{}
	for _, d := range b.Deliveries {
		total++
		if d.OnTime {
			onTime++
		} else {
			late = append(late, fmt.Sprintf("%s（承诺 %s，实际 %s）", d.BatchNo, d.PromisedAt, d.ArrivedAt))
		}
	}
	m := metric{
		code: "d.otd", name: "准时交付率 OTD", dim: domain.DimDelivery, unit: "%",
		weight: 0.35, minSample: 10, auto: true, sample: total,
		numerator: float64(onTime), denominator: float64(total),
		formula:   "按承诺到货日准时交付的批次数 ÷ 交付总批次",
		benchmark: "参考 ≥ 95%", evidence: evidenceOf(late, 4),
	}
	if total > 0 {
		m.score = float64(onTime) / float64(total) * 100
		m.raw = fmt.Sprintf("%d/%d 批 · %.1f%%", onTime, total, m.score)
	} else {
		m.raw = "无交付记录"
	}
	out = append(out, m)

	// ---- 交期承诺达成 ----
	hit, promised := 0, 0
	for _, d := range b.Deliveries {
		if d.PromisedLeadDays <= 0 {
			continue
		}
		promised++
		if d.LeadDays <= d.PromisedLeadDays {
			hit++
		}
	}
	m = metric{
		code: "d.lead", name: "交期承诺达成率", dim: domain.DimDelivery, unit: "%",
		weight: 0.25, minSample: 8, auto: true, sample: promised,
		numerator: float64(hit), denominator: float64(promised),
		formula:   "实际交付周期 ≤ 承诺周期的批次数 ÷ 有承诺周期的批次",
		benchmark: "参考 ≥ 90%",
	}
	if promised > 0 {
		m.score = float64(hit) / float64(promised) * 100
		m.raw = fmt.Sprintf("%d/%d 批未超期", hit, promised)
	} else {
		m.raw = "无承诺周期数据"
	}
	out = append(out, m)

	// ---- 订单满足率 ----
	accepted, ordered := 0, 0
	for _, d := range b.Deliveries {
		accepted += d.Accepted
		ordered += d.Qty
	}
	m = metric{
		code: "d.fill", name: "订单满足率", dim: domain.DimDelivery, unit: "%",
		weight: 0.25, minSample: 5000, auto: true, sample: ordered,
		numerator: float64(accepted), denominator: float64(ordered),
		formula:   "合格收货件数 ÷ 订单需求件数（含短装与补货）",
		benchmark: "参考 ≥ 99%",
	}
	if ordered > 0 {
		m.score = float64(accepted) / float64(ordered) * 100
		m.raw = fmt.Sprintf("%d/%d 件", accepted, ordered)
	} else {
		m.raw = "无订单数据"
	}
	out = append(out, m)

	// ---- 插单弹性 ----
	expeditedIDs := map[string]bool{}
	for _, o := range b.Orders {
		if o.Expedited {
			expeditedIDs[o.ID] = true
		}
	}
	expTotal, expOnTime := 0, 0
	for _, d := range b.Deliveries {
		if !expeditedIDs[d.POID] {
			continue
		}
		expTotal++
		if d.OnTime {
			expOnTime++
		}
	}
	m = metric{
		code: "d.flex", name: "紧急插单弹性", dim: domain.DimDelivery, unit: "%",
		weight: 0.15, minSample: 2, auto: true, sample: expTotal,
		numerator: float64(expOnTime), denominator: float64(expTotal),
		formula: "加急订单批次中准时交付的比例", benchmark: "衡量产能弹性与配合度",
	}
	if expTotal > 0 {
		m.score = float64(expOnTime) / float64(expTotal) * 100
		m.raw = fmt.Sprintf("%d/%d 批加急单准时", expOnTime, expTotal)
	} else {
		m.raw = "无加急订单"
	}
	out = append(out, m)
	return out
}

func serviceMetrics(b Bundle) []metric {
	out := make([]metric, 0, 3)

	// ---- 响应时效 ----
	sumH, n := 0.0, 0
	for _, t := range b.Tickets {
		if t.ResponseHours > 0 {
			sumH += t.ResponseHours
			n++
		}
	}
	m := metric{
		code: "s.response", name: "平均响应时长", dim: domain.DimService, unit: "小时",
		weight: 0.35, minSample: 3, auto: true, sample: n,
		numerator: sumH, denominator: float64(maxInt(n, 1)),
		formula:   "100 − 平均响应小时数 × 2（4 小时内响应约 92 分，超过 48 小时归零）",
		benchmark: "参考 ≤ 4 小时",
	}
	if n > 0 {
		avg := sumH / float64(n)
		m.score = clamp(100-avg*2, 0, 100)
		m.raw = fmt.Sprintf("平均 %.1f 小时", avg)
		m.numerator = avg
		m.denominator = 1
	} else {
		m.raw = "无工单记录"
	}
	out = append(out, m)

	// ---- 问题解决周期 ----
	sumD, dn := 0.0, 0
	for _, t := range b.Tickets {
		if t.ResolveDays > 0 {
			sumD += t.ResolveDays
			dn++
		}
	}
	m = metric{
		code: "s.resolve", name: "问题解决周期", dim: domain.DimService, unit: "天",
		weight: 0.30, minSample: 3, auto: true, sample: dn,
		numerator: sumD, denominator: float64(maxInt(dn, 1)),
		formula:   "100 − 平均解决天数 × 5（3 天约 85 分，20 天归零）",
		benchmark: "参考 ≤ 7 天",
	}
	if dn > 0 {
		avg := sumD / float64(dn)
		m.score = clamp(100-avg*5, 0, 100)
		m.raw = fmt.Sprintf("平均 %.1f 天", avg)
		m.numerator = avg
		m.denominator = 1
	} else {
		m.raw = "无结案记录"
	}
	out = append(out, m)

	// ---- 协同配合度（人工评审）----
	sumC, cn := 0, 0
	reviewers := []string{}
	for _, r := range b.Reviews {
		if r.CooperationScore > 0 {
			sumC += r.CooperationScore
			cn++
			reviewers = append(reviewers, fmt.Sprintf("%s 评 %.0f 分（%s）", r.Reviewer, float64(r.CooperationScore), r.ReviewedAt))
		}
	}
	m = metric{
		code: "s.coop", name: "协同配合度", dim: domain.DimService, unit: "分",
		weight: 0.35, minSample: 1, auto: false, sample: cn,
		numerator: float64(sumC), denominator: float64(maxInt(cn, 1)),
		formula:   "采购/质量/工程三方评审打分（工程变更、打样、临时需求的配合程度）",
		benchmark: "人工评审，非自动采集", evidence: evidenceOf(reviewers, 3),
	}
	if cn > 0 {
		avg := float64(sumC) / float64(cn)
		m.score = clamp(avg, 0, 100)
		m.raw = fmt.Sprintf("三方评审均分 %.0f", avg)
		m.numerator = avg
		m.denominator = 1
	} else {
		m.raw = "尚未评审"
	}
	out = append(out, m)
	return out
}

func techMetrics(b Bundle) []metric {
	out := make([]metric, 0, 4)

	// ---- 关键工序能力覆盖 ----
	sumCov, n := 0.0, 0
	for _, r := range b.Reviews {
		if r.CapabilityCoverage > 0 {
			sumCov += r.CapabilityCoverage
			n++
		}
	}
	m := metric{
		code: "t.capability", name: "关键工序覆盖度", dim: domain.DimTechnology, unit: "%",
		weight: 0.35, minSample: 1, auto: false, sample: n,
		numerator: sumCov, denominator: float64(maxInt(n, 1)),
		formula:   "已具备的关键工序数 ÷ 目标产品要求的关键工序数（现场审核确认）",
		benchmark: "覆盖不足则无法独立承接整机",
	}
	if n > 0 {
		avg := sumCov / float64(n)
		m.score = clamp(avg, 0, 100)
		m.raw = fmt.Sprintf("覆盖 %.0f%%", avg)
		m.numerator = avg
		m.denominator = 100
	} else {
		m.raw = "未做现场审核"
	}
	out = append(out, m)

	// ---- 研发投入强度 ----
	sumR, rn := 0.0, 0
	for _, r := range b.Reviews {
		if r.RndRatio > 0 {
			sumR += r.RndRatio
			rn++
		}
	}
	m = metric{
		code: "t.rnd", name: "研发投入强度", dim: domain.DimTechnology, unit: "%",
		weight: 0.25, minSample: 1, auto: false, sample: rn,
		numerator: sumR, denominator: float64(maxInt(rn, 1)),
		formula:   "研发人员占比 ÷ 15% × 100（15% 及以上记满分）",
		benchmark: "制造企业参考 5–15%",
	}
	if rn > 0 {
		avg := sumR / float64(rn)
		m.score = clamp(avg/15*100, 0, 100)
		m.raw = fmt.Sprintf("研发人员占比 %.1f%%", avg)
	} else {
		m.raw = "未采集"
	}
	out = append(out, m)

	// ---- 新品导入成功率 ----
	proj, succ := 0, 0
	for _, r := range b.Reviews {
		proj += r.NPIProjects
		succ += r.NPISuccess
	}
	m = metric{
		code: "t.npi", name: "新品导入成功率", dim: domain.DimTechnology, unit: "%",
		weight: 0.25, minSample: 2, auto: true, sample: proj,
		numerator: float64(succ), denominator: float64(proj),
		formula:   "按期量产的新项目数 ÷ 立项的新项目数",
		benchmark: "参考 ≥ 80%",
	}
	if proj > 0 {
		m.score = float64(succ) / float64(proj) * 100
		m.raw = fmt.Sprintf("%d/%d 个项目按期量产", succ, proj)
	} else {
		m.raw = "无 NPI 记录"
	}
	out = append(out, m)

	// ---- 设备与自动化水平 ----
	sumA, an := 0, 0
	for _, r := range b.Reviews {
		if r.AutomationLevel > 0 {
			sumA += r.AutomationLevel
			an++
		}
	}
	m = metric{
		code: "t.automation", name: "设备自动化水平", dim: domain.DimTechnology, unit: "级",
		weight: 0.15, minSample: 1, auto: false, sample: an,
		numerator: float64(sumA), denominator: float64(maxInt(an, 1)),
		formula:   "现场评级 1–5 级，换算为 20/40/60/80/100 分",
		benchmark: "1 手工为主，5 全自动线",
	}
	if an > 0 {
		avg := float64(sumA) / float64(an)
		m.score = clamp(avg/5*100, 0, 100)
		m.raw = fmt.Sprintf("%.1f 级", avg)
		m.numerator = avg
		m.denominator = 5
	} else {
		m.raw = "未评级"
	}
	out = append(out, m)
	return out
}

func thousands(v int64) string {
	s := fmt.Sprintf("%d", v)
	if len(s) <= 3 {
		return s
	}
	out := make([]byte, 0, len(s)+len(s)/3)
	pre := len(s) % 3
	if pre > 0 {
		out = append(out, s[:pre]...)
		if len(s) > pre {
			out = append(out, ',')
		}
	}
	for i := pre; i < len(s); i += 3 {
		out = append(out, s[i:i+3]...)
		if i+3 < len(s) {
			out = append(out, ',')
		}
	}
	return string(out)
}
