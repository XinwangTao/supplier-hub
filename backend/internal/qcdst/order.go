package qcdst

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/example/supplier-hub/backend/internal/domain"
)

// 订单级评分的判定门槛。
//
// 一张订单通常只涉及两三个维度：还没发货的单判不了质量，没有询价记录的单判不了成本。
// 如果硬性要求五维齐全，绝大多数订单都会"无分"；如果缺的维度按 0 分算，
// 一张刚下单的单子就会被打成 0 分——这两种做法都会让订单评分失去意义。
// 所以这里规定：可判维度的权重合计超过下面这个值，才认为这笔订单评得出来。
const minJudgedWeightForGrade = 0.5

// OrderScoreInput 算一张订单评分卡所需的输入。
type OrderScoreInput struct {
	Order      domain.PurchaseOrder
	Deliveries []domain.Delivery      // 属于这张订单的交付批次
	Quotes     []domain.Quote         // 该供应商的报价记录，用于取基准价
	Tickets    []domain.ServiceTicket // 该供应商的工单，按时间窗关联到订单
	Events     []domain.QualityEvent  // 该供应商的质量事件，按时间窗关联
	Benchmark  float64                // 该品类的基准单价；<=0 表示没有基准，成本维不判
	Profile    Profile
	Demo       bool
}

// ScoreOrder 给一张订单打 QCDST 分。
//
// 与供应商级评估（Compute）的分工：
//   - Compute 看的是一段时期的**稳定表现**，用于评级与选择；
//   - ScoreOrder 看的是**这一张单**执行得怎么样，用于对账与追责。
//
// 两者不互相平均。订单级分数汇总不回写供应商总分，供应商总分始终从原始单据重算——
// 否则"5 张订单的平均分"会抹掉批量权重，大单小单等权，评分就被稀释了。
func ScoreOrder(in OrderScoreInput) domain.OrderScorecard {
	card := domain.OrderScorecard{
		OrderID:   in.Order.ID,
		PONo:      in.Order.PONo,
		TenantID:  in.Order.TenantID,
		PartName:  in.Order.PartName,
		Category:  in.Order.Category,
		Qty:       in.Order.Qty,
		Amount:    in.Order.Amount,
		IssuedAt:  in.Order.IssuedAt,
		ClosedAt:  in.Order.ClosedAt,
		Status:    in.Order.Status,
		Expedited: in.Order.Expedited,
		Demo:      in.Demo,
		Total:     -1,
		Grade:     domain.GradeNA,
	}

	card.Dimensions = []domain.OrderDimension{
		orderQuality(in),
		orderCost(in),
		orderDelivery(in),
		orderService(in),
		orderTechnology(in),
	}

	// 只对判得出来的维度重新归一加权。没判的维度既不算 0 分也不算满分，直接退出分母。
	judgedWeight := 0.0
	weighted := 0.0
	for _, d := range card.Dimensions {
		if d.Judged {
			judgedWeight += d.Weight
		}
	}
	if judgedWeight <= 0 {
		card.Note = "这张订单还没有任何可判定的业务记录，不给分（不等于 0 分）。"
		return card
	}
	for i := range card.Dimensions {
		d := &card.Dimensions[i]
		if d.Judged {
			d.EffectiveWeight = round2(d.Weight / judgedWeight)
			weighted += d.Score * d.EffectiveWeight
		}
	}
	card.JudgedWeight = round2(judgedWeight)
	card.Total = round1(weighted)

	// 置信度就是"这笔订单到底覆盖了多少权重"。只判了交付（25%）却打出 A+，
	// 会让人误以为这家供应商整体很好，所以低于门槛一律不出评级。
	if judgedWeight >= minJudgedWeightForGrade {
		card.Grade = gradeOf(card.Total, judgedWeight)
	}
	if judgedWeight < minJudgedWeightForGrade {
		names := []string{}
		for _, d := range card.Dimensions {
			if d.Judged {
				names = append(names, d.Name)
			}
		}
		card.Note = fmt.Sprintf("本次仅 %s 可判定（权重合计 %.0f%%），不足以代表这家供应商的整体水平，不出评级。",
			strings.Join(names, "、"), judgedWeight*100)
	}
	return card
}

// ---------------- 各维度判定 ----------------

// orderQuality 质量：这张单的来料检验结果与不良率。
// 没有交付批次就判不了——这是最常见的情况，必须如实说明而不是给默认分。
func orderQuality(in OrderScoreInput) domain.OrderDimension {
	d := domain.OrderDimension{
		Dimension: domain.DimQuality, Name: dimNames[domain.DimQuality],
		Weight: in.Profile.Weight[domain.DimQuality], Score: -1,
	}
	if len(in.Deliveries) == 0 {
		d.Reason = "还没有交付批次，无法判定来料质量"
		return d
	}
	total, accepted, rejected := 0, 0, 0
	passBatch, conditionalBatch, failBatch, unknownBatch := 0, 0, 0, 0
	for _, dl := range in.Deliveries {
		total += dl.Qty
		accepted += dl.Accepted
		rejected += dl.Rejected
		switch dl.IQCResult {
		case "fail":
			failBatch++
		case "conditional":
			conditionalBatch++
		case "pass":
			passBatch++
		default:
			// 没填 IQC 结果不能当成"通过"，否则没做检验的批次会冒充合格。
			unknownBatch++
		}
	}
	if passBatch+conditionalBatch+failBatch == 0 {
		d.Reason = "交付批次未记录 IQC 结果，无法判定来料质量"
		return d
	}
	// 分母只算有检验结论的批次，未填结果的不当合格也不当不合格。
	batch := passBatch + conditionalBatch + failBatch
	// 让步接收算通过但打折：它确实没退货，可"勉强能用"不该等同于"一次做对"。
	batchScore := float64(passBatch+conditionalBatch)/float64(batch)*100 - float64(conditionalBatch)/float64(batch)*15
	ppm := 0.0
	if total > 0 {
		ppm = float64(rejected) / float64(total) * 1e6
	}
	ppmScore := 100.0
	switch {
	case ppm <= 100:
		ppmScore = 100
	case ppm <= 500:
		ppmScore = 88
	case ppm <= 1000:
		ppmScore = 74
	case ppm <= 3000:
		ppmScore = 55
	case ppm <= 8000:
		ppmScore = 35
	default:
		ppmScore = 15
	}
	if failBatch > 0 {
		// 判退是硬伤，不管不良率多低都要封顶
		if ppmScore > 45 {
			ppmScore = 45
		}
		if batchScore > 45 {
			batchScore = 45
		}
	}
	d.Score = round1(clamp(batchScore*0.6+ppmScore*0.4, 0, 100))
	d.Judged = true
	d.Raw = fmt.Sprintf("%d 批（通过 %d / 让步 %d / 判退 %d）· 不良 %.0f PPM",
		batch, passBatch, conditionalBatch, failBatch, ppm)
	parts := []string{fmt.Sprintf("%d 个有检验结论的批次按 IQC 结果加权", batch)}
	if unknownBatch > 0 {
		parts = append(parts, fmt.Sprintf("另有 %d 批未记录检验结果，未计入", unknownBatch))
	}
	if conditionalBatch > 0 {
		parts = append(parts, fmt.Sprintf("%d 批为让步接收，每批扣 15 分", conditionalBatch))
	}
	if failBatch > 0 {
		parts = append(parts, fmt.Sprintf("%d 批判退，分数封顶 45", failBatch))
	}
	d.Reason = strings.Join(parts, "；")
	return d
}

// orderCost 成本：这张单的成交单价相对基准价的位置。
// 没有基准价（既没历史报价也没同类订单）就不判，避免拿一个拍脑袋的基准去扣分。
func orderCost(in OrderScoreInput) domain.OrderDimension {
	d := domain.OrderDimension{
		Dimension: domain.DimCost, Name: dimNames[domain.DimCost],
		Weight: in.Profile.Weight[domain.DimCost], Score: -1,
	}
	if in.Benchmark <= 0 {
		d.Reason = "没有可比基准价（历史报价或同类订单单价），无法判定价格水平"
		return d
	}
	if in.Order.UnitPrice <= 0 {
		d.Reason = "订单未记录单价，无法判定价格水平"
		return d
	}
	ratio := in.Order.UnitPrice / in.Benchmark
	// 比基准贵 1% 扣 4 分；便宜不加分上限 100，避免越便宜越好的误导
	// （明显低于基准往往是降配或后续涨价的前兆，采购上要警惕而不是奖励）。
	score := 100 - (ratio-1)*400
	if ratio < 0.9 {
		score = 100 - (0.9-ratio)*150
	}
	d.Score = round1(clamp(score, 0, 100))
	d.Judged = true
	d.Raw = fmt.Sprintf("单价 %.2f · 基准 %.2f（%.1f%%）", in.Order.UnitPrice, in.Benchmark, (ratio-1)*100)
	if ratio > 1 {
		d.Reason = fmt.Sprintf("高于基准价 %.1f%%，每高 1%% 扣 4 分", (ratio-1)*100)
	} else if ratio < 0.9 {
		d.Reason = fmt.Sprintf("低于基准价 %.1f%%，明显低价需核实配置与后续涨价风险", (1-ratio)*100)
	} else {
		d.Reason = "价格在基准附近，属正常区间"
	}
	return d
}

// orderDelivery 交付：这张单的准时情况与提前期达成。
// 关键边界：订单还在执行且没到承诺日期时，不能因为"还没交"就判不及格。
func orderDelivery(in OrderScoreInput) domain.OrderDimension {
	d := domain.OrderDimension{
		Dimension: domain.DimDelivery, Name: dimNames[domain.DimDelivery],
		Weight: in.Profile.Weight[domain.DimDelivery], Score: -1,
	}
	if len(in.Deliveries) == 0 {
		if in.Order.Status == "open" {
			d.Reason = "订单仍在执行中，尚未到货，不判交付（不等于延期）"
		} else {
			d.Reason = "没有交付记录，无法判定交付表现"
		}
		return d
	}
	totalQty, onTimeQty := 0, 0
	onTimeBatches := 0
	leadNum, leadDen := 0.0, 0.0
	lateDays := 0
	for _, dl := range in.Deliveries {
		totalQty += dl.Qty
		if dl.OnTime {
			onTimeQty += dl.Qty
			onTimeBatches++
		}
		if dl.PromisedLeadDays > 0 && dl.LeadDays > 0 {
			leadNum += float64(dl.PromisedLeadDays) / float64(dl.LeadDays)
			leadDen++
		}
		if dl.ArrivedAt > dl.PromisedAt {
			lateDays = maxInt(lateDays, daysLate(dl.PromisedAt, dl.ArrivedAt))
		}
	}
	onTimeRate := 0.0
	if totalQty > 0 {
		onTimeRate = float64(onTimeQty) / float64(totalQty)
	}
	// 按批量加权而不是按批次数：一批 5000 件延误的影响远大于一批 200 件。
	onTimeScore := onTimeRate * 100
	leadScore := 100.0
	if leadDen > 0 {
		leadScore = clamp(leadNum/leadDen*100, 0, 100)
	}
	score := onTimeScore*0.7 + leadScore*0.3
	if in.Order.Expedited && onTimeRate < 1 {
		// 插单本身有额外难度，但不能完全免责——延误已经发生了
		score += 8
	}
	d.Score = round1(clamp(score, 0, 100))
	d.Judged = true
	if lateDays > 0 {
		d.Raw = fmt.Sprintf("按批量准时率 %.1f%% · 最长延误 %d 天", onTimeRate*100, lateDays)
	} else {
		d.Raw = fmt.Sprintf("按批量准时率 %.1f%%", onTimeRate*100)
	}
	d.Reason = fmt.Sprintf("%d 批中 %d 批准时，提前期达成率 %.0f%%", len(in.Deliveries), onTimeBatches, leadScore)
	if in.Order.Expedited {
		d.Reason += "；本单为紧急插单，已给 8 分宽限"
	}
	return d
}

// orderService 服务：这张单前后关联的服务工单响应与解决速度。
// 多数订单没有关联工单，这是正常状态，判不了就判不了，不影响总分。
func orderService(in OrderScoreInput) domain.OrderDimension {
	d := domain.OrderDimension{
		Dimension: domain.DimService, Name: dimNames[domain.DimService],
		Weight: in.Profile.Weight[domain.DimService], Score: -1,
	}
	related := []domain.ServiceTicket{}
	for _, t := range in.Tickets {
		if t.OpenedAt == "" || in.Order.IssuedAt == "" {
			continue
		}
		if t.OpenedAt >= in.Order.IssuedAt && daysBetween(in.Order.IssuedAt, t.OpenedAt) <= 45 {
			related = append(related, t)
		}
	}
	if len(related) == 0 {
		d.Reason = "这张订单前后 45 天内没有关联的服务工单，无法判定服务响应"
		return d
	}
	respScore, resolveScore, n := 0.0, 0.0, 0
	for _, t := range related {
		if t.ResponseHours > 0 {
			respScore += clamp(100-(t.ResponseHours-2)*12, 0, 100)
			n++
		}
		if t.ResolveDays > 0 {
			resolveScore += clamp(100-(t.ResolveDays-1)*14, 0, 100)
		}
	}
	if n == 0 {
		d.Reason = "关联工单未记录响应时长，无法判定服务响应"
		return d
	}
	d.Score = round1(clamp(respScore/float64(n)*0.6+resolveScore/float64(n)*0.4, 0, 100))
	d.Judged = true
	avgResp := 0.0
	for _, t := range related {
		avgResp += t.ResponseHours
	}
	avgResp /= float64(len(related))
	d.Raw = fmt.Sprintf("%d 张关联工单 · 平均首次响应 %.1fh", len(related), avgResp)
	d.Reason = fmt.Sprintf("按工单响应（60%%）与解决时长（40%%）加权，基准为 2 小时响应 / 1 天解决")
	return d
}

// orderTechnology 技术：这张单的首件检验（FAI）是否一次通过。
// 有首件批次才判；常规翻单没有首件，属正常情况。
func orderTechnology(in OrderScoreInput) domain.OrderDimension {
	d := domain.OrderDimension{
		Dimension: domain.DimTechnology, Name: dimNames[domain.DimTechnology],
		Weight: in.Profile.Weight[domain.DimTechnology], Score: -1,
	}
	fai := []domain.Delivery{}
	for _, dl := range in.Deliveries {
		if dl.FirstArticle {
			fai = append(fai, dl)
		}
	}
	if len(fai) == 0 {
		d.Reason = "本单无首件检验批次（常规翻单通常没有），无法判定技术能力"
		return d
	}
	passed := 0
	for _, dl := range fai {
		if dl.FAIPass {
			passed++
		}
	}
	d.Score = round1(float64(passed) / float64(len(fai)) * 100)
	if passed == len(fai) {
		d.Score = 100
	} else {
		d.Score = 40
	}
	d.Judged = true
	d.Raw = fmt.Sprintf("首件 %d 件，通过 %d 件", len(fai), passed)
	if passed == len(fai) {
		d.Reason = "首件一次通过"
	} else {
		d.Reason = "首件未一次通过，需重新送样，按 40 分计"
	}
	return d
}

// ---------------- 工具 ----------------

// ScoreOrders 批量算一个 bundle 里所有订单的评分卡，按下单时间倒序。
// 交付批次按 POID 归单；工单与质量事件按时间窗关联。
func ScoreOrders(b Bundle, benchmark float64) []domain.OrderScorecard {
	profile := ProfileFor(b.Supplier)
	byOrder := map[string][]domain.Delivery{}
	for _, dl := range b.Deliveries {
		byOrder[dl.POID] = append(byOrder[dl.POID], dl)
	}
	out := make([]domain.OrderScorecard, 0, len(b.Orders))
	for _, o := range b.Orders {
		card := ScoreOrder(OrderScoreInput{
			Order: o, Deliveries: byOrder[o.ID], Quotes: b.Quotes,
			Tickets: b.Tickets, Events: b.Events,
			Benchmark: benchmark, Profile: profile, Demo: b.Demo,
		})
		for _, dl := range byOrder[o.ID] {
			card.BatchCount++
			if dl.OnTime {
				card.OnTimeBatches++
			}
			card.Accepted += dl.Accepted
			card.Rejected += dl.Rejected
			card.IQCResults = append(card.IQCResults, dl.IQCResult)
		}
		out = append(out, card)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].IssuedAt > out[j].IssuedAt })
	return out
}

// BenchmarkPrice 推算某供应商某品类的基准单价。
//
// 口径优先级：报价里的市场基准价 > 自家历史订单单价中位数。
// 用历史订单中位数兜底，是因为多数企业的询价记录不全，
// 但不能因此就不判成本——自家历史价本身就是采购最常拿来对比的锚。
func BenchmarkPrice(b Bundle, category string) (float64, string) {
	prices := []float64{}
	for _, q := range b.Quotes {
		if q.BenchmarkPrice > 0 && (category == "" || q.PartName != "") {
			prices = append(prices, q.BenchmarkPrice)
		}
	}
	if len(prices) > 0 {
		sort.Float64s(prices)
		return prices[len(prices)/2], "报价基准价中位数"
	}
	op := []float64{}
	for _, o := range b.Orders {
		if o.UnitPrice > 0 {
			op = append(op, o.UnitPrice)
		}
	}
	if len(op) == 0 {
		return 0, "无基准"
	}
	sort.Float64s(op)
	return op[len(op)/2], "历史订单单价中位数"
}

func daysLate(promised, arrived string) int {
	return daysBetween(promised, arrived)
}

func daysBetween(a, b string) int {
	t1, err1 := time.Parse("2006-01-02", a)
	t2, err2 := time.Parse("2006-01-02", b)
	if err1 != nil || err2 != nil {
		return 0
	}
	return int(t2.Sub(t1).Hours() / 24)
}
