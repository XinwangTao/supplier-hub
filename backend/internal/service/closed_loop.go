package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/example/supplier-hub/backend/internal/domain"
	"github.com/example/supplier-hub/backend/internal/qcdst"
	"github.com/example/supplier-hub/backend/internal/repository"
)

// 这个文件是"寻源 ⇄ SRM"闭环的接缝处。
//
// 链路：
//   公开寻源池（工商 + 工艺 + 平台评级）
//     → 企业点「引入 SRM」，建立合作关系（Engagement）
//     → 在 SRM 里下发订单、收报价、录交付与 IQC
//     → 每张订单独立出 QCDST 评分卡
//     → 供应商级 QCDST 由原始单据聚合而来
//     → 跨采购方加权后回流公开池，更新平台评级
//
// 反哺不是把私有数据公开：进入公开池的只有聚合后的分数、样本量与买方数，
// 订单金额、单价、质量事件细节一律不出现在公开接口里。

// EngageRequest 把一家供应商引入某企业的 SRM。
type EngageRequest struct {
	TenantID   string
	TenantName string
	SupplierID string
	Status     string // 见 domain.EngagementStatus；空默认为 candidate
	Category   string
	BuyerName  string
	Source     string // sourcing=从寻源页引入，manual=手工建档
	Note       string
}

type EngageResult struct {
	Engagement   domain.Engagement `json:"engagement"`
	Created      bool              `json:"created"`
	SupplierName string            `json:"supplier_name"`
	Message      string            `json:"message"`
}

// Engage 建立或更新合作关系。
//
// 两个刻意的设计：
//  1. 从寻源引入默认落在 candidate（候选），不是 active。
//     "看中了"和"真的在合作"必须分开，否则候选池会污染在管供应商口径。
//  2. 重复引入不会建第二条关系，而是返回已有的那条——同一个采购方对同一家
//     供应商只能有一份合作档案，否则评分会被同一份关系重复计入。
func (s *SupplierOpsService) Engage(ctx context.Context, req EngageRequest) (EngageResult, error) {
	supplier, err := s.repo.GetByID(ctx, req.SupplierID)
	if err != nil {
		return EngageResult{}, err
	}
	status := req.Status
	if status == "" {
		status = string(domain.EngagementCandidate)
	}
	if !domain.IsValidEngagementStatus(status) {
		return EngageResult{}, fmt.Errorf("无效的合作关系状态：%s", status)
	}
	tenant := s.store.EnsureTenant(domain.Tenant{ID: req.TenantID, Name: req.TenantName, Industry: "未填写", Plan: "team"})

	if req.Source == "" {
		req.Source = "manual"
	}
	if req.Category == "" && len(supplier.CapabilityNames) > 0 {
		req.Category = supplier.CapabilityNames[0]
	}
	now := time.Now().Format("2006-01")
	existing, found := s.store.EngagementOf(tenant.ID, supplier.ID)
	if !found && req.Note == "" {
		req.Note = fmt.Sprintf("%s 从公开寻源池引入", now)
	}

	e := domain.Engagement{
		ID:            fmt.Sprintf("eng-%s-%s", tenant.ID, supplier.ID),
		TenantID:      tenant.ID,
		SupplierID:    supplier.ID,
		Status:        status,
		Category:      req.Category,
		Since:         now,
		BuyerName:     req.BuyerName,
		Source:        req.Source,
		Note:          req.Note,
		SourcingTypes: []string{"待定"},
	}
	if found {
		e.Since = existing.Since
	}
	saved, created := s.store.UpsertEngagement(e)

	msg := ""
	if created {
		msg = fmt.Sprintf("已加入%s池：%s。录入第一张订单后 QCDST 才会开始积累。",
			domain.EngagementStatusLabel[domain.EngagementStatus(status)], supplier.Name)
	} else {
		msg = fmt.Sprintf("%s 已在本企业名单中，状态更新为 %s。",
			supplier.Name, domain.EngagementStatusLabel[domain.EngagementStatus(status)])
	}
	return EngageResult{
		Engagement: saved, Created: created, SupplierName: supplier.Name, Message: msg,
	}, nil
}

// SetEngagementStatus 推进合作关系状态。已终止的关系不允许直接复活成 active，
// 必须先退回候选重新评估——终止往往有质量或合规原因，不该一键翻篇。
func (s *SupplierOpsService) SetEngagementStatus(tenantID, supplierID, status string) (domain.Engagement, error) {
	if !domain.IsValidEngagementStatus(status) {
		return domain.Engagement{}, fmt.Errorf("无效的合作关系状态：%s", status)
	}
	current, ok := s.store.EngagementOf(tenantID, supplierID)
	if !ok {
		return domain.Engagement{}, repository.ErrNotFound
	}
	if current.Status == string(domain.EngagementTerminated) && status == string(domain.EngagementActive) {
		return domain.Engagement{}, errors.New("已终止的合作不能直接恢复为合作中，请先退回候选并重新评估")
	}
	e, ok := s.store.SetEngagementStatus(tenantID, supplierID, status)
	if !ok {
		return domain.Engagement{}, repository.ErrNotFound
	}
	return e, nil
}

// ---------------- 订单级评分卡 ----------------

type OrderScorecardsResult struct {
	Supplier   domain.Supplier         `json:"supplier"`
	Tenant     domain.Tenant           `json:"tenant"`
	Benchmark  OrderBenchmark          `json:"benchmark"`
	Cards      []domain.OrderScorecard `json:"cards"`
	Summary    OrderSummary            `json:"summary"`
	Assessment domain.QCDSTAssessment  `json:"assessment"`
	Demo       bool                    `json:"demo"`
}

type OrderBenchmark struct {
	Price float64 `json:"price"`
	Basis string  `json:"basis"`
}

type OrderSummary struct {
	Total        int     `json:"total"`         // 订单总数
	Judged       int     `json:"judged"`        // 评得出分数的订单数
	AverageScore float64 `json:"average_score"` // 订单级均分（仅可判订单）
	WorstScore   float64 `json:"worst_score"`
	BestScore    float64 `json:"best_score"`
	// 口径说明：订单级均分与供应商总分不是一回事，界面必须写清楚，
	// 否则用户会拿两个数字对不上来质疑数据有问题。
	Caveat string `json:"caveat"`
}

// OrderScorecards 返回某采购方下某供应商的逐单评分卡，并给出与供应商总分的口径对照。
func (s *SupplierOpsService) OrderScorecards(ctx context.Context, tenantID, supplierID string) (OrderScorecardsResult, error) {
	supplier, err := s.repo.GetByID(ctx, supplierID)
	if err != nil {
		return OrderScorecardsResult{}, err
	}
	if tenantID == "" {
		if tenants := s.store.Tenants(); len(tenants) > 0 {
			tenantID = tenants[0].ID
		}
	}
	tenant, _ := s.store.Tenant(tenantID)
	bundle := s.store.Bundle(tenantID, supplierID)
	bundle.Supplier = supplier
	bundle.Tenant = tenant

	price, basis := qcdst.BenchmarkPrice(bundle, supplierID)
	cards := qcdst.ScoreOrders(bundle, price)
	assessment := qcdst.Compute(bundle)

	summary := OrderSummary{Total: len(cards)}
	sum, worst, best := 0.0, 101.0, -1.0
	for _, c := range cards {
		if c.Total < 0 {
			continue
		}
		summary.Judged++
		sum += c.Total
		if c.Total < worst {
			worst = c.Total
		}
		if c.Total > best {
			best = c.Total
		}
	}
	if summary.Judged > 0 {
		summary.AverageScore = round1(sum / float64(summary.Judged))
		summary.WorstScore = round1(worst)
		summary.BestScore = round1(best)
	} else {
		summary.AverageScore = -1
		summary.WorstScore = -1
		summary.BestScore = -1
	}
	summary.Caveat = fmt.Sprintf(
		"订单级均分 %.1f 是 %d 张可判订单的简单平均（大单小单等权），用于对账与追责；"+
			"供应商综合分 %.1f 由全部 %d 个交付批次按指标聚合而来，用于评级与选择。"+
			"两者不等价，也不会互相覆盖。",
		summary.AverageScore, summary.Judged, assessment.Total, assessment.BatchCount)

	return OrderScorecardsResult{
		Supplier: supplier, Tenant: tenant,
		Benchmark:  OrderBenchmark{Price: price, Basis: basis},
		Cards:      cards,
		Summary:    summary,
		Assessment: assessment,
		Demo:       s.store.Demo(),
	}, nil
}

// ---------------- 寻源页索引 ----------------

// BriefIndex 给公开寻源页用的一份索引：
// 每家供应商在平台上的评级（反哺结果）+ 当前企业自己的合作状态（私有）。
//
// 为什么不让寻源接口直接带评级：评级是全量聚合计算，跟着每次搜索重算太重，
// 而它变化很慢。前端拉一次索引缓存在本地，搜索结果按 supplier_id 合并即可。
func (s *SupplierOpsService) BriefIndex(ctx context.Context, tenantID string) domain.BriefIndex {
	if tenantID == "" {
		if tenants := s.store.Tenants(); len(tenants) > 0 {
			tenantID = tenants[0].ID
		}
	}
	tenant, _ := s.store.Tenant(tenantID)
	index := domain.BriefIndex{
		TenantID:   tenantID,
		TenantName: tenant.Name,
		Items:      map[string]domain.SupplierBrief{},
	}

	ratings := s.PlatformRatings(ctx)
	for _, r := range ratings {
		brief := domain.SupplierBrief{
			SupplierID:    r.SupplierID,
			Rated:         r.Grade != domain.GradeNA && r.Total >= 0,
			Grade:         r.Grade,
			Total:         r.Total,
			BuyerCount:    r.BuyerCount,
			RatedBuyers:   r.RatedBuyers,
			BatchCount:    r.BatchCount,
			OrderCount:    r.OrderCount,
			Spread:        r.Spread,
			Provisional:   r.Provisional,
			Verified:      r.Verified,
			Demo:          r.Demo,
			RatingSummary: ratingSummary(r),
		}
		if brief.Rated {
			index.Meta.Rated++
		}
		index.Items[r.SupplierID] = brief
	}

	// 覆盖本企业的私有状态（候选也算"在名单里"）
	for _, e := range s.store.Engagements(tenantID) {
		brief := index.Items[e.SupplierID]
		brief.SupplierID = e.SupplierID
		brief.InSRM = true
		brief.Status = e.Status
		brief.StatusLabel = domain.EngagementStatusLabel[domain.EngagementStatus(e.Status)]
		brief.Since = e.Since
		brief.MyOrderCount = len(s.store.Orders(tenantID, e.SupplierID))
		b := s.store.Bundle(tenantID, e.SupplierID)
		brief.MyBatchCount = len(b.Deliveries)
		index.Items[e.SupplierID] = brief
		if e.Status == string(domain.EngagementCandidate) {
			index.Meta.Candidates++
		} else {
			index.Meta.InSRM++
		}
	}
	index.Meta.Buyers = len(s.store.Tenants())
	index.Meta.Demo = s.store.Demo()
	index.Meta.Note = "评级来自各企业在 SRM 中积累的真实单据，按样本量加权；公开接口不暴露订单金额与单价。"
	return index
}

// ratingSummary 把评级翻译成一句人能直接读的话。
// 采购看的是"能不能用"，不是"这个数怎么来的"，但支撑证据必须同时给出。
func ratingSummary(r domain.PlatformRating) string {
	switch {
	case r.Grade == domain.GradeNA && r.BatchCount == 0:
		return "尚无企业在 SRM 中合作，暂无平台评分"
	case r.Provisional:
		return fmt.Sprintf("%d 家企业合作中，累计 %d 批交付，样本不足暂不评级", r.BuyerCount, r.BatchCount)
	case r.RatedBuyers < 2:
		return fmt.Sprintf("%.1f 分（%s），仅 %d 家企业评过分，参考性有限", r.Total, r.Grade, r.RatedBuyers)
	default:
		tail := ""
		if r.Spread >= 6 {
			tail = fmt.Sprintf("，%d 家企业体验差异较大（分歧度 %.1f）", r.RatedBuyers, r.Spread)
		}
		return fmt.Sprintf("%.1f 分（%s），%d 家企业评过分、累计 %d 批交付%s", r.Total, r.Grade, r.RatedBuyers, r.BatchCount, tail)
	}
}

// FeedbackLoop 闭环的体检报告：平台池现在有多大、反哺有没有真的发生。
// 放在这里是因为它回答的是"这套机制跑起来了没有"，不是某个供应商好不好。
func (s *SupplierOpsService) FeedbackLoop(ctx context.Context, tenantID string) domain.LoopReport {
	ratings := s.PlatformRatings(ctx)
	report := domain.LoopReport{
		SuppliersInPool: len(ratings),
		Tenants:         len(s.store.Tenants()),
	}
	batchSum, spreadSum, ratedSum := 0, 0.0, 0
	for _, r := range ratings {
		batchSum += r.BatchCount
		if r.Grade != domain.GradeNA && r.Total >= 0 {
			ratedSum++
			spreadSum += r.Spread
		}
		if r.Verified {
			report.Verified++
		}
		if r.Provisional {
			report.Provisional++
		}
		if r.Demo {
			report.Demo++
		}
	}
	report.TotalBatches = batchSum
	report.RatedSuppliers = ratedSum
	if ratedSum > 0 {
		report.AverageSpread = math.Round(spreadSum/float64(ratedSum)*10) / 10
	}
	if tenantID != "" {
		for _, e := range s.store.Engagements(tenantID) {
			report.MySuppliers++
			if e.Status == string(domain.EngagementCandidate) {
				report.MyCandidates++
			}
			report.MyBatches += len(s.store.Bundle(tenantID, e.SupplierID).Deliveries)
		}
	}
	report.Stage = loopStage(report)
	return report
}

// loopStage 判断这套闭环目前走到哪一步。新用户进来最需要知道的不是排名，
// 而是"现在这个池子能不能信"。
func loopStage(r domain.LoopReport) string {
	switch {
	case r.SuppliersInPool == 0:
		return "空池：还没有任何供应商进入 SRM，平台评级无从谈起"
	case r.RatedSuppliers == 0:
		return "蓄水期：已有供应商进入合作，但样本都还没到评级门槛（15 批）"
	case r.RatedSuppliers < 3 || r.Tenants < 2:
		return "初期：评级样本偏少，建议只作为辅助参考，不要当作唯一依据"
	default:
		return "可用：评级已由多家企业的真实单据支撑，可用于初筛"
	}
}
