package qcdst

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/example/supplier-hub/backend/internal/domain"
)

// 数据体检。
//
// 评分引擎算得再细，喂进去的是脏数据也白搭，更糟的是：脏数据往往是
// 有意造出来的。这一段不修数据，只把问题摆到台面上，让评级带上"这个分数
// 有几成可信"的注脚。
//
// 查六件事：
//  1. 收货数与判定数对不上（accepted + rejected ≠ qty）
//  2. 准时标记与日期矛盾（说准时，实际到货日晚于承诺日）
//  3. 未来日期（单据日期晚于今天）
//  4. 缺到货日期
//  5. 小样批占比（大量极小批量单把合格率垫高）
//  6. 数据跨度（15 个批次全在一周内 ≠ 15 个批次的水平）

// smallBatchFactor 低于中位批量的这个比例，就算小样批。
const smallBatchFactor = 0.2

// CheckIntegrity 体检一批交付记录。
func CheckIntegrity(deliveries []domain.Delivery) domain.DataQuality {
	q := domain.DataQuality{BatchCount: len(deliveries), Level: "ok"}
	if len(deliveries) == 0 {
		q.Summary = "没有交付记录，无从体检"
		q.Score = 0
		return q
	}
	today := time.Now().Format("2006-01-02")

	qtys := make([]int, 0, len(deliveries))
	for _, d := range deliveries {
		if d.Qty > 0 {
			qtys = append(qtys, d.Qty)
		} else {
			q.ZeroQtyBatch++
		}
		if d.Accepted+d.Rejected > 0 && d.Accepted+d.Rejected != d.Qty {
			q.BadBalance++
		}
		// 未填 IQC 的批次不算"判定量与收货量不符"——那只是还没判。
		if d.ArrivedAt == "" {
			q.MissingDate++
		} else {
			if d.ArrivedAt > today {
				q.FutureDated++
			}
			if d.PromisedAt != "" && d.ArrivedAt > d.PromisedAt && d.OnTime {
				q.BadOnTime++
			}
		}
	}

	// 小样批：低于典型批量 20% 的批次。这些批几乎必然合格，
	// 攒一批小样单就能把"批次合格率"刷上去，而量产时完全不重现。
	//
	// 典型批量取"件数加权中位批量"（一半的货是以多大批量交付的），
	// 而不是批次数的中位数：后者在有极端值时会偏向小的那一侧，
	// 结果 12 个 10 件小样批 + 3 个 10000 件主力批反而算不出小样批。
	median := weightedMedianQty(qtys)
	small := 0
	if median > 0 {
		threshold := float64(median) * smallBatchFactor
		for _, v := range qtys {
			if float64(v) < threshold {
				small++
			}
		}
	}
	if len(qtys) > 0 {
		q.SmallBatchRate = round1(float64(small) / float64(len(qtys)) * 100)
	}
	for _, v := range qtys {
		q.TotalQty += v
	}

	add := func(code, level, title, detail string, affected int) {
		q.Issues = append(q.Issues, domain.DataIssue{
			Code: code, Level: level, Title: title, Detail: detail, Affected: affected,
		})
		if level == "bad" {
			q.Level = "bad"
		} else if level == "warn" && q.Level == "ok" {
			q.Level = "warn"
		}
	}

	if q.ZeroQtyBatch > 0 {
		add("zero_qty", "bad", fmt.Sprintf("%d 个批次收货数为 0", q.ZeroQtyBatch),
			"收货量为 0 的批次不参与合格率计算，但会虚增批次数——批次数是评级门槛，等于拿空单凑样本量。", q.ZeroQtyBatch)
	}
	if q.BadBalance > 0 {
		add("bad_balance", "warn", fmt.Sprintf("%d 个批次合格数与不良数之和不等于收货数", q.BadBalance),
			"要么判定未录完，要么单据录错。这类批次的不良率不可信。", q.BadBalance)
	}
	if q.BadOnTime > 0 {
		add("bad_on_time", "bad", fmt.Sprintf("%d 个批次标了准时但实际晚于承诺日", q.BadOnTime),
			"准时标记与到货日期自相矛盾。OTD 是交付维度的主指标，这类数据会直接把交付分抬高。", q.BadOnTime)
	}
	if q.FutureDated > 0 {
		add("future_dated", "bad", fmt.Sprintf("%d 个批次的日期在未来", q.FutureDated),
			"单据日期晚于今天，通常是录错或人为回填。", q.FutureDated)
	}
	if q.MissingDate > 0 {
		add("missing_date", "warn", fmt.Sprintf("%d 个批次没有到货日期", q.MissingDate),
			"缺日期的批次无法参与时间衰减与趋势判断，会被按最旧档处理。", q.MissingDate)
	}
	if q.SmallBatchRate >= 30 {
		add("small_batch", "warn", fmt.Sprintf("%.0f%% 的批次是小样批（不足中位批量 20%%）", q.SmallBatchRate),
			fmt.Sprintf("中位批量约 %d 件。小样批几乎必然合格，占比过高说明批次合格率是垫出来的，量产爬坡时大概率回落。", median), small)
	}

	// 完整度打分：从 100 起扣，单项封顶，避免一条问题把分数打到 0。
	score := 100.0
	n := float64(len(deliveries))
	score -= float64(q.ZeroQtyBatch) / n * 60
	score -= float64(q.BadBalance) / n * 20
	score -= float64(q.BadOnTime) / n * 40
	score -= float64(q.FutureDated) / n * 40
	score -= float64(q.MissingDate) / n * 15
	if q.SmallBatchRate >= 30 {
		score -= math.Min(q.SmallBatchRate, 100) / 100 * 20
	}
	q.Score = round1(clamp(score, 0, 100))

	switch q.Level {
	case "bad":
		q.Summary = fmt.Sprintf("发现 %d 类严重数据问题，当前分数的可信度存疑", countLevel(q.Issues, "bad"))
	case "warn":
		q.Summary = fmt.Sprintf("有 %d 处数据瑕疵，分数可用但需留意", len(q.Issues))
	default:
		q.Summary = fmt.Sprintf("%d 个批次数据自洽（累计 %s 件）", q.BatchCount, thousands(int64(q.TotalQty)))
	}
	return q
}

func countLevel(list []domain.DataIssue, level string) int {
	n := 0
	for _, i := range list {
		if i.Level == level {
			n++
		}
	}
	return n
}

// weightedMedianQty 件数加权中位批量：把所有批次按批量从小到大累加，
// 累计件数首次达到总量一半时的那个批量。
//
// 它回答的是"一半的货是以多大批量交付的"，这才是典型批量。
// 批次数的中位数会被大量小单带偏，用在这里是错的。
func weightedMedianQty(list []int) int {
	if len(list) == 0 {
		return 0
	}
	sorted := make([]int, len(list))
	copy(sorted, list)
	sort.Ints(sorted)
	total := 0
	for _, v := range sorted {
		total += v
	}
	if total <= 0 {
		return 0
	}
	half := float64(total) / 2
	acc := 0
	for _, v := range sorted {
		acc += v
		if float64(acc) >= half {
			return v
		}
	}
	return sorted[len(sorted)-1]
}

// DataSpanDays 首末交付相隔天数。
func DataSpanDays(deliveries []domain.Delivery) int {
	first, last := "", ""
	for _, d := range deliveries {
		if d.ArrivedAt == "" {
			continue
		}
		if first == "" || d.ArrivedAt < first {
			first = d.ArrivedAt
		}
		if last == "" || d.ArrivedAt > last {
			last = d.ArrivedAt
		}
	}
	if first == "" {
		return 0
	}
	a, err1 := time.Parse("2006-01-02", first)
	b, err2 := time.Parse("2006-01-02", last)
	if err1 != nil || err2 != nil {
		return 0
	}
	return int(b.Sub(a).Hours()/24) + 1
}

// RecencyWeight 时间衰减权重。
//
// 半衰期 halfLifeDays：距今 halfLife 天的单据权重降到 0.5。
// 采购决策关心的是"现在行不行"，三年前的好成绩不该和昨天的差成绩等权。
// 供应链上通用 12 个月半衰期——比这短会被短期波动带着跑，
// 比这长就失去了区分"以前好"和"现在好"的意义。
func RecencyWeight(dateStr string, halfLifeDays float64) float64 {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		// 日期不可解析的一律按最旧档处理：不该因为录错格式反而拿到高权重。
		return 0.1
	}
	days := time.Since(t).Hours() / 24
	if days < 0 {
		// 未来日期已经计入数据体检，这里一并压到最低权重。
		return 0.1
	}
	return math.Pow(0.5, days/halfLifeDays)
}

// LastDeliveryDate 最后一次交付日期（YYYY-MM-DD）。
func LastDeliveryDate(deliveries []domain.Delivery) string {
	last := ""
	for _, d := range deliveries {
		if d.ArrivedAt != "" && d.ArrivedAt > last {
			last = d.ArrivedAt
		}
	}
	return last
}

// DaysSince 距今天数；日期缺失返回 -1。
func DaysSince(dateStr string) int {
	if dateStr == "" {
		return -1
	}
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return -1
	}
	return int(time.Since(t).Hours() / 24)
}

// TrendOf 比较近期加权分与历史总分，给出变化方向。
// 差距不足 3 分视为持平——3 分以内的波动在供应链上叫噪声，不叫趋势。
func TrendOf(recent, total float64) string {
	if total < 0 || recent < 0 {
		return "unknown"
	}
	diff := recent - total
	switch {
	case diff >= 3:
		return "up"
	case diff <= -3:
		return "down"
	default:
		return "flat"
	}
}

var trendLabel = map[string]string{
	"up":      "在变好",
	"flat":    "基本持平",
	"down":    "在变差",
	"unknown": "无法判断",
}

func TrendLabel(t string) string {
	if v, ok := trendLabel[t]; ok {
		return v
	}
	return "无法判断"
}
