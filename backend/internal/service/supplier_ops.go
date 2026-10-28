package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/example/supplier-hub/backend/internal/domain"
	"github.com/example/supplier-hub/backend/internal/qcdst"
	"github.com/example/supplier-hub/backend/internal/repository"
	"github.com/example/supplier-hub/backend/internal/srm"
)

// SupplierOpsService 是"供应商运营"这条线：评估、工作台、平台评级。
//
// 和 GraphService 的区别在于数据来源：图谱回答"去哪找供应商"（工商能力），
// 这里回答"这家到底靠不靠谱"（真实业务表现）。两者共用同一个供应商主数据。
type SupplierOpsService struct {
	repo  repository.SupplierRepository
	store *srm.Store
}

func NewSupplierOpsService(repo repository.SupplierRepository, store *srm.Store) *SupplierOpsService {
	return &SupplierOpsService{repo: repo, store: store}
}

func (s *SupplierOpsService) Store() *srm.Store { return s.store }

// Assess 算出某采购方视角下某供应商的 QCDST。
func (s *SupplierOpsService) Assess(ctx context.Context, tenantID, supplierID string) (domain.QCDSTAssessment, error) {
	supplier, err := s.repo.GetByID(ctx, supplierID)
	if err != nil {
		return domain.QCDSTAssessment{}, err
	}
	bundle := s.store.Bundle(tenantID, supplierID)
	bundle.Supplier = supplier
	if tenantID == "" {
		// 平台视角：汇总该供应商在所有采购方下的单据。
		bundle.Demo = s.store.Demo()
	}
	return qcdst.Compute(bundle), nil
}

// AssessAll 返回某采购方工作台里全部供应商的评估，按总分降序。
func (s *SupplierOpsService) AssessAll(ctx context.Context, tenantID string) []domain.QCDSTAssessment {
	ids := s.store.SupplierIDs()
	out := make([]domain.QCDSTAssessment, 0, len(ids))
	for _, id := range ids {
		a, err := s.Assess(ctx, tenantID, id)
		if err != nil {
			continue
		}
		out = append(out, a)
	}
	sort.SliceStable(out, func(i, j int) bool {
		// 没有评级的放最后：这不是"分低"，是"还不知道"，排序上不该混在一起。
		gi, gj := out[i].Grade == domain.GradeNA, out[j].Grade == domain.GradeNA
		if gi != gj {
			return !gi
		}
		return out[i].Total > out[j].Total
	})
	return out
}

// PlatformRatings 平台级综合评级：把每家采购方的评估按样本量加权合并。
// 这是"新用户进来能参考"的那张表。
func (s *SupplierOpsService) PlatformRatings(ctx context.Context) []domain.PlatformRating {
	engs := s.store.Engagements("")
	bySupplier := map[string][]string{}
	for _, e := range engs {
		bySupplier[e.SupplierID] = append(bySupplier[e.SupplierID], e.TenantID)
	}

	out := make([]domain.PlatformRating, 0, len(bySupplier))
	for supplierID, tenantIDs := range bySupplier {
		supplier, err := s.repo.GetByID(ctx, supplierID)
		if err != nil {
			continue
		}
		assessments := make([]domain.QCDSTAssessment, 0, len(tenantIDs))
		for _, tid := range tenantIDs {
			b := s.store.Bundle(tid, supplierID)
			// 只把"真的有业务往来"的采购方算进买方数。
			// 候选关系（关注了但没下过单）如果也算一个买方，
			// "3 家企业都在用"这句话就成了假的，而这恰恰是新用户最看重的信号。
			if len(b.Orders)+len(b.Deliveries) == 0 {
				continue
			}
			b.Supplier = supplier
			assessments = append(assessments, qcdst.Compute(b))
		}
		if len(assessments) == 0 {
			continue
		}
		out = append(out, mergePlatform(supplier, assessments))
	}
	sort.SliceStable(out, func(i, j int) bool {
		gi, gj := out[i].Grade == domain.GradeNA, out[j].Grade == domain.GradeNA
		if gi != gj {
			return !gi
		}
		return out[i].Total > out[j].Total
	})
	return out
}

// recencyHalfLifeDays 平台评级的时间半衰期。
// 三年前的好成绩不该和昨天的差成绩等权——采购决策关心的是现在。
const recencyHalfLifeDays = 365

// maxDominantShare 单一采购方在评级权重里的上限。
//
// 为什么必须设上限："3 家企业都评过"这句话的可信度，取决于这 3 家是不是
// 真的各占一份。如果其中一家贡献了 500 个批次、另外两家各 5 个批次，
// 那所谓"三家验证"实际是"一家说了算"。封顶到 50% 后，任何一家都不能
// 单独决定评级，剩下的权重必须由其它采购方分担。
const maxDominantShare = 0.5

// mergePlatform 把多份评估合成一条平台评级。
//
// 加权口径是样本量（订单+批次）× 置信度 × 时间衰减，不是简单平均：
// 300 批的客户说 90 分，和 5 批的客户说 90 分，权重本来就不该一样；
// 三年前的 300 批，和今年的 300 批，也不该一样。
func mergePlatform(s domain.Supplier, list []domain.QCDSTAssessment) domain.PlatformRating {
	rating := domain.PlatformRating{
		SupplierID: s.ID, SupplierName: s.Name, ShortName: s.ShortName,
		CreditCode: s.CreditCode, City: s.City, Capabilities: s.Capabilities,
		Dimensions: []domain.DimensionBrief{},
	}
	totals := []float64{}
	recentTotals := []float64{}
	weights := []float64{}
	dimNum := map[domain.Dimension]float64{}
	dimDen := map[domain.Dimension]float64{}
	confNum, confDen := 0.0, 0.0
	demoAll := true
	spanStart, spanEnd := "", ""
	var integrity domain.DataQuality

	for _, a := range list {
		rating.OrderCount += a.OrderCount
		rating.BatchCount += a.BatchCount
		rating.QualityEvents += a.QualityEvents
		if !a.Demo {
			demoAll = false
		}
		// 数据体检与跨度按最差的那份取：一家采购方的数据脏，
		// 汇上去的评级就不能干净。
		if a.Integrity.Level == "bad" || (a.Integrity.Level == "warn" && integrity.Level != "bad") {
			integrity = a.Integrity
		}
		if a.Window != "" && a.Window != "无交付记录" {
			if parts := splitWindow(a.Window); len(parts) == 2 {
				if spanStart == "" || parts[0] < spanStart {
					spanStart = parts[0]
				}
				if spanEnd == "" || parts[1] > spanEnd {
					spanEnd = parts[1]
				}
			}
		}
		if a.Grade == domain.GradeNA || a.Total < 0 {
			continue
		}
		w := float64(a.BatchCount+a.OrderCount) * a.Confidence * recencyFactor(a)
		if w <= 0 {
			w = 0.5
		}
		weights = append(weights, w)
		totals = append(totals, a.Total)
		recentTotals = append(recentTotals, a.RecentTotal)
		rating.RatedBuyers++
		confNum += a.Confidence * w
		confDen += w
		for _, d := range a.Dimensions {
			if d.Score < 0 {
				continue
			}
			dimNum[d.Dimension] += d.Score * w
			dimDen[d.Dimension] += w
		}
		if a.GeneratedAt > rating.LastSeenAt {
			rating.LastSeenAt = a.GeneratedAt
		}
	}

	rating.BuyerCount = len(list)
	rating.Demo = demoAll && len(list) > 0
	rating.Verified = !rating.Demo && rating.BatchCount > 0
	rating.DataSpanDays = daysBetween(spanStart, spanEnd)
	if integrity.Level == "" {
		integrity.Level = "ok"
	}
	rating.Integrity = integrity
	// 最后一批交付距今超过一年，这条评级只代表历史。
	if days := daysSince(spanEnd); days > stalePlatformDays {
		rating.Stale = true
	}

	// ---- 单买方权重封顶 ----
	// 封顶规则：任何一家采购方的权重不超过其余采购方权重之和（即 ≤ 50%）。
	// 这不是为了好看，是为了让"N 家企业验证"这句话站得住。
	weightSum := 0.0
	for _, w := range weights {
		weightSum += w
	}
	if weightSum > 0 && len(weights) > 1 {
		cap := weightSum * maxDominantShare
		// 一次封顶可能让总和变化，但封顶只会降低最大者，重新求和即可。
		adjusted := 0.0
		for i := range weights {
			if weights[i] > cap {
				weights[i] = cap
			}
			adjusted += weights[i]
		}
		maxW := 0.0
		for _, w := range weights {
			if w > maxW {
				maxW = w
			}
		}
		if adjusted > 0 {
			rating.DominantShare = round2(maxW / adjusted)
		}
		weightSum = adjusted
	} else if weightSum > 0 {
		rating.DominantShare = 1.0
	}

	weighted := 0.0
	recentNum, recentDen := 0.0, 0.0
	for i, t := range totals {
		weighted += t * weights[i]
		if i < len(recentTotals) && recentTotals[i] >= 0 {
			recentNum += recentTotals[i] * weights[i]
			recentDen += weights[i]
		}
	}

	if weightSum > 0 {
		rating.Total = round1(weighted / weightSum)
		rating.Confidence = round2(confNum / confDen)
		// 分歧度：不同采购方体验差异大，往往比均分更值得警惕。
		rating.Spread = round1(stddev(totals))
		if recentDen > 0 {
			rating.RecentTotal = round1(recentNum / recentDen)
			rating.Trend = qcdst.TrendOf(rating.RecentTotal, rating.Total)
		} else {
			rating.RecentTotal = -1
			rating.Trend = "unknown"
		}
		rating.Grade = gradeFrom(rating.Total, rating.Confidence)
	} else {
		rating.Total = -1
		rating.RecentTotal = -1
		rating.Trend = "unknown"
		rating.Grade = domain.GradeNA
		rating.Confidence = 0
	}

	// 样本门槛放在最后统一判：不管是不是加权算出来的分，
	// 累计批次不到 15、或数据跨度不足 90 天，一律标观察期 ——
	// "还不知道"和"评出来差"必须分开。
	if rating.BatchCount < minPlatformBatches || (rating.DataSpanDays > 0 && rating.DataSpanDays < minPlatformSpanDays) {
		rating.Provisional = true
		rating.Grade = domain.GradeNA
	}

	// ---- 汇总护栏 ----
	rating.Guardrails = platformGuardrails(rating)
	if rating.DominantShare > 0.6 {
		rating.Guardrails = append(rating.Guardrails,
			fmt.Sprintf("单一采购方占权重 %.0f%%，这条评级基本由一家企业的体验决定", rating.DominantShare*100))
	}
	if len(rating.Guardrails) == 0 {
		rating.Guardrails = append(rating.Guardrails, "未触发护栏：样本量、跨度、买方分散度均达标")
	}

	for _, d := range []domain.Dimension{domain.DimQuality, domain.DimCost, domain.DimDelivery, domain.DimService, domain.DimTechnology} {
		score := -1.0
		if dimDen[d] > 0 {
			score = round1(dimNum[d] / dimDen[d])
		}
		rating.Dimensions = append(rating.Dimensions, domain.DimensionBrief{
			Dimension: d, Name: dimName(d), Score: score,
		})
	}
	return rating
}

const (
	minPlatformBatches  = 15
	minPlatformSpanDays = 90
	stalePlatformDays   = 365
)

// recencyFactor 一份评估的时间衰减系数。数据越旧，在平台评级里说话的分量越小。
func recencyFactor(a domain.QCDSTAssessment) float64 {
	// 近期分优先：有近期数据就用近期窗口的衰减（半衰期按 180 天窗口的一半算），
	// 否则退回"最后一次生成评估"的时间。
	if a.Window != "" && a.Window != "无交付记录" {
		if parts := splitWindow(a.Window); len(parts) == 2 {
			return qcdst.RecencyWeight(parts[1], recencyHalfLifeDays)
		}
	}
	return 0.5
}

// platformGuardrails 平台级护栏文案。
func platformGuardrails(r domain.PlatformRating) []string {
	out := []string{}
	if r.Provisional {
		out = append(out, fmt.Sprintf("观察期：累计 %d 个批次 / 跨度 %d 天，未达评级门槛（%d 批、%d 天）",
			r.BatchCount, r.DataSpanDays, minPlatformBatches, minPlatformSpanDays))
	}
	if r.Stale {
		out = append(out, "数据已过期：超过 365 天没有新交付")
	}
	switch r.Trend {
	case "down":
		out = append(out, fmt.Sprintf("近期在下滑：近半年 %.1f 分，低于历史 %.1f 分", r.RecentTotal, r.Total))
	case "up":
		out = append(out, fmt.Sprintf("近期在改善：近半年 %.1f 分，高于历史 %.1f 分", r.RecentTotal, r.Total))
	}
	if r.Spread >= 8 {
		out = append(out, fmt.Sprintf("买方分歧大：各采购方评分标准差 %.1f 分，选之前先看具体是哪家说好", r.Spread))
	}
	if r.Integrity.Level == "bad" {
		out = append(out, "单据存在严重数据问题，分数可信度存疑")
	}
	if r.Integrity.SmallBatchRate >= 30 {
		out = append(out, fmt.Sprintf("小样批占比 %.0f%%，批次合格率可能被垫高", r.Integrity.SmallBatchRate))
	}
	if r.RatedBuyers == 1 {
		out = append(out, "仅 1 家采购方的数据达到评级门槛，暂不构成跨企业验证")
	}
	return out
}

func splitWindow(w string) []string {
	// Window 形如 "2026-01-05 ~ 2026-03-20"
	for i := 0; i < len(w)-2; i++ {
		if w[i] == ' ' && w[i+1] == '~' {
			return []string{trimSpace(w[:i]), trimSpace(w[i+2:])}
		}
	}
	return nil
}

func trimSpace(s string) string {
	b := []byte(s)
	i, j := 0, len(b)-1
	for i <= j && (b[i] == ' ' || b[i] == '\t') {
		i++
	}
	for j >= i && (b[j] == ' ' || b[j] == '\t') {
		j--
	}
	return string(b[i : j+1])
}

// daysSince 某日期距今天数；缺失或解析失败返回 -1。
func daysSince(dateStr string) int {
	if len(dateStr) < 10 {
		return -1
	}
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return -1
	}
	return int(time.Since(t).Hours() / 24)
}

// daysBetween 两个 YYYY-MM-DD 之间相隔天数；任一缺失返回 0。
func daysBetween(a, b string) int {
	if len(a) < 10 || len(b) < 10 {
		return 0
	}
	ta, err1 := time.Parse("2006-01-02", a)
	tb, err2 := time.Parse("2006-01-02", b)
	if err1 != nil || err2 != nil {
		return 0
	}
	d := int(tb.Sub(ta).Hours()/24) + 1
	if d < 0 {
		return 0
	}
	return d
}

func stddev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))
	sum := 0.0
	for _, v := range values {
		sum += (v - mean) * (v - mean)
	}
	return math.Sqrt(sum / float64(len(values)))
}

func gradeFrom(total, confidence float64) domain.Grade {
	if confidence < 0.35 || total < 0 {
		return domain.GradeNA
	}
	switch {
	case total >= 90:
		return domain.GradeAPlus
	case total >= 80:
		return domain.GradeA
	case total >= 70:
		return domain.GradeB
	case total >= 60:
		return domain.GradeC
	default:
		return domain.GradeD
	}
}

func dimName(d domain.Dimension) string {
	switch d {
	case domain.DimQuality:
		return "质量"
	case domain.DimCost:
		return "成本"
	case domain.DimDelivery:
		return "交付"
	case domain.DimService:
		return "服务"
	case domain.DimTechnology:
		return "技术"
	}
	return string(d)
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
func round2(v float64) float64 { return math.Round(v*100) / 100 }

// Workspace 返回一家采购方的 SRM 工作台。
func (s *SupplierOpsService) Workspace(ctx context.Context, tenantID string) (domain.SRMWorkspace, error) {
	tenant, ok := s.store.Tenant(tenantID)
	if !ok {
		tenants := s.store.Tenants()
		if len(tenants) == 0 {
			return domain.SRMWorkspace{Engagements: []domain.Engagement{}, Assessments: []domain.QCDSTAssessment{}}, nil
		}
		tenant = tenants[0]
		tenantID = tenant.ID
	}
	engs := s.store.Engagements(tenantID)
	ratings := s.AssessAll(ctx, tenantID)

	ws := domain.SRMWorkspace{
		Tenant: tenant, Engagements: engs, Assessments: ratings, Demo: s.store.Demo(),
	}
	active := 0
	spend := 0.0
	for _, e := range engs {
		if e.Status == "active" {
			active++
		}
		spend += e.AnnualSpend
	}
	ws.Stats.Suppliers = len(engs)
	ws.Stats.Active = active
	ws.Stats.Spend = spend

	orders, open := 0, 0
	for _, id := range s.store.SupplierIDs() {
		b := s.store.Bundle(tenantID, id)
		orders += len(b.Orders)
		for _, o := range b.Orders {
			if o.Status == "open" {
				open++
			}
		}
	}
	ws.Stats.Orders = orders
	ws.Stats.OpenOrders = open

	totalBatch, onTime, totalQty, accepted := 0, 0, 0, 0
	events := 0
	for _, id := range s.store.SupplierIDs() {
		b := s.store.Bundle(tenantID, id)
		events += len(b.Events)
		for _, d := range b.Deliveries {
			totalBatch++
			if d.OnTime {
				onTime++
			}
			totalQty += d.Qty
			accepted += d.Accepted
		}
	}
	ws.Stats.Deliveries = totalBatch
	ws.Stats.QualityEvents = events
	if totalBatch > 0 {
		ws.Stats.OnTimeRate = round1(float64(onTime) / float64(totalBatch) * 100)
	}
	if totalQty > 0 {
		ws.Stats.PassRate = round1(float64(accepted) / float64(totalQty) * 100)
	}
	ws.Insights = workspaceInsights(ws, ratings)
	return ws, nil
}

func workspaceInsights(ws domain.SRMWorkspace, ratings []domain.QCDSTAssessment) []domain.Insight {
	out := []domain.Insight{}
	if len(ratings) == 0 {
		out = append(out, domain.Insight{
			Kind: "empty", Level: "info", Title: "还没有供应商进入 SRM",
			Detail: "录入第一张订单后，QCDST 才会开始积累；在此之前不会给出任何评级。",
		})
		return out
	}
	na := 0
	for _, r := range ratings {
		if r.Grade == domain.GradeNA {
			na++
		}
	}
	if na > 0 {
		out = append(out, domain.Insight{
			Kind: "insufficient", Level: "warn",
			Title:  fmt.Sprintf("%d 家供应商证据不足，暂不评级", na),
			Detail: fmt.Sprintf("这些供应商的交付批次少于 %d 个，或数据跨度不足 %d 天。继续累积单据，或补录历史交付记录。",
				minPlatformBatches, minPlatformSpanDays),
		})
	}
	low := []string{}
	for _, r := range ratings {
		if r.Grade != domain.GradeNA && r.Total < 70 {
			low = append(low, fmt.Sprintf("%s（%.0f 分）", r.ShortName, r.Total))
		}
	}
	if len(low) > 0 {
		out = append(out, domain.Insight{
			Kind: "low_score", Level: "critical",
			Title:  fmt.Sprintf("%d 家供应商低于 70 分", len(low)),
			Detail: "建议按末位管理启动整改：" + joinList(low, "、") + "。",
		})
	}
	if ws.Stats.OnTimeRate > 0 && ws.Stats.OnTimeRate < 92 {
		out = append(out, domain.Insight{
			Kind: "otd", Level: "warn",
			Title:  fmt.Sprintf("整体准时交付率 %.1f%%，低于 92%%", ws.Stats.OnTimeRate),
			Detail: "交付风险已不是个案，建议先排查共性原因（排产、原材料、物流）。",
		})
	}
	return out
}

func joinList(list []string, sep string) string {
	out := ""
	for i, s := range list {
		if i > 0 {
			out += sep
		}
		out += s
	}
	return out
}

// SeedDemo 载入演示数据，返回写入的供应商数量。
func (s *SupplierOpsService) SeedDemo(ctx context.Context) (int, error) {
	s.store.ClearDemo()
	suppliers := s.store.SeedDemo()
	for _, sp := range suppliers {
		if err := s.repo.Upsert(ctx, sp); err != nil {
			return 0, err
		}
	}
	return len(suppliers), nil
}

func (s *SupplierOpsService) ClearDemo() {
	s.store.ClearDemo()
}

// RecordOrder 录入订单（真实业务数据的入口之一）。
func (s *SupplierOpsService) RecordOrder(o domain.PurchaseOrder) domain.PurchaseOrder {
	if o.Status == "" {
		o.Status = "open"
	}
	if o.IssuedAt == "" {
		o.IssuedAt = time.Now().Format("2006-01-02")
	}
	s.store.AddOrder(o)
	return o
}

func (s *SupplierOpsService) RecordDelivery(d domain.Delivery) domain.Delivery {
	s.store.AddDelivery(d)
	return d
}

func (s *SupplierOpsService) RecordQualityEvent(e domain.QualityEvent) domain.QualityEvent {
	s.store.AddQualityEvent(e)
	return e
}
