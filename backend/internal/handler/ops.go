package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/example/supplier-hub/backend/internal/catalog"
	"github.com/example/supplier-hub/backend/internal/domain"
	"github.com/example/supplier-hub/backend/internal/repository"
	"github.com/example/supplier-hub/backend/internal/service"
	"github.com/gin-gonic/gin"
	"time"
)

// ---------------- QCDST ----------------

// QCDSTRanking 平台级评级榜：跨企业聚合后的综合评级，供新用户选供应商参考。
func (h *Handler) QCDSTRanking(c *gin.Context) {
	ratings := h.ops.PlatformRatings(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"items": ratings,
			"meta":  buildRatingMeta(ratings),
		},
	})
}

func buildRatingMeta(list []domain.PlatformRating) gin.H {
	counts := map[string]int{}
	verified, demo, buyers := 0, 0, map[string]bool{}
	for _, r := range list {
		counts[string(r.Grade)]++
		if r.Verified {
			verified++
		}
		if r.Demo {
			demo++
		}
		for i := 0; i < r.BuyerCount; i++ {
			buyers[r.SupplierID] = true
		}
	}
	return gin.H{
		"total": len(list), "verified": verified, "demo": demo,
		"grades": counts,
		"note":   "评级只由真实 SRM 单据驱动；证据不足的供应商显示 NA，不参与排名。",
	}
}

// QCDSTSupplier 单家供应商的 QCDST 明细（含每项指标的分子分母与证据）。
func (h *Handler) QCDSTSupplier(c *gin.Context) {
	tenant := c.Query("tenant")
	assess, err := h.ops.Assess(c.Request.Context(), tenant, c.Param("id"))
	if errors.Is(err, repository.ErrNotFound) {
		notFound(c, "供应商不存在")
		return
	}
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": assess})
}

// ---------------- SRM ----------------

func (h *Handler) SRMTenants(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": h.ops.Store().Tenants()})
}

// SRMWorkspace 采购方工作台：合作关系、评估列表、汇总指标。
func (h *Handler) SRMWorkspace(c *gin.Context) {
	ws, err := h.ops.Workspace(c.Request.Context(), c.Query("tenant"))
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": ws})
}

// SRMSeedDemo 载入演示数据。演示供应商的信用代码带 DEMO 前缀，
// 页面上必须同时打"演示"标记，避免被当成真实企业。
func (h *Handler) SRMSeedDemo(c *gin.Context) {
	n, err := h.ops.SeedDemo(c.Request.Context())
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"suppliers": n, "demo": true}})
}

func (h *Handler) SRMClearDemo(c *gin.Context) {
	h.ops.ClearDemo()
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"cleared": true}})
}

// SRMCreateOrder 录入订单。这是真实数据进入系统的入口：
// 没有订单，就没有交付批次；没有批次，QCDST 的 D 维永远是"证据不足"。
//
// 走 RecordOrderGuarded：必须先有合作关系才能录单。
// 少了这道校验，任何人都能给任意供应商凭空造出交付记录，
// 平台评级就成了一个可以刷的数字。
func (h *Handler) SRMCreateOrder(c *gin.Context) {
	var req domain.PurchaseOrder
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "订单字段格式无效")
		return
	}
	if req.SupplierID == "" {
		badRequest(c, "supplier_id 必填")
		return
	}
	o, err := h.ops.RecordOrderGuarded(c.Request.Context(), req, actorOf(c))
	if errors.Is(err, service.ErrDuplicateOrder) {
		conflict(c, err.Error())
		return
	}
	if err != nil {
		forbidden(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": o})
}

func (h *Handler) SRMCreateDelivery(c *gin.Context) {
	var req domain.Delivery
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "交付记录格式无效")
		return
	}
	if req.SupplierID == "" {
		badRequest(c, "supplier_id 必填")
		return
	}
	d, err := h.ops.RecordDeliveryGuarded(c.Request.Context(), req, actorOf(c))
	if errors.Is(err, service.ErrDuplicateBatch) {
		conflict(c, err.Error())
		return
	}
	if err != nil {
		forbidden(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": d})
}

// SRMAudit 审计流水。谁在什么时候录了什么、哪些写入被拦下了。
func (h *Handler) SRMAudit(c *gin.Context) {
	limit := 100
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	entries := h.ops.AuditEntries(c.Query("tenant"), limit)
	if entries == nil {
		entries = []domain.AuditEntry{}
	}
	total, rejected, _ := h.ops.Store().AuditStats(c.Query("tenant"))
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"entries": entries, "total": total, "rejected": rejected,
	}})
}

// actorOf 操作人标识。真实系统里取自登录态，这里退化为请求头，
// 但字段必须在——没有操作人的审计等于没有审计。
func actorOf(c *gin.Context) string {
	if v := c.GetHeader("X-Actor"); v != "" {
		return v
	}
	return "buyer"
}

// SRMCreateEngagement 建立采购方与供应商的合作关系。没有这层关系，
// 供应商看不到订单，采购方也无权评估——SRM 的边界就在这里。
//
// 走的是 Engage：会校验供应商真实存在、状态合法、同一对关系不重复建档。
// 直接往 store 里 append 会让同一份关系出现两次，评分被重复计入。
func (h *Handler) SRMCreateEngagement(c *gin.Context) {
	var req struct {
		TenantID    string  `json:"tenant_id"`
		TenantName  string  `json:"tenant_name"`
		SupplierID  string  `json:"supplier_id"`
		Status      string  `json:"status"`
		Category    string  `json:"category"`
		BuyerName   string  `json:"buyer_name"`
		Source      string  `json:"source"`
		Note        string  `json:"note"`
		AnnualSpend float64 `json:"annual_spend"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "合作关系字段格式无效")
		return
	}
	result, err := h.ops.Engage(c.Request.Context(), service.EngageRequest{
		TenantID: req.TenantID, TenantName: req.TenantName, SupplierID: req.SupplierID,
		Status: req.Status, Category: req.Category, BuyerName: req.BuyerName,
		Source: req.Source, Note: req.Note,
	})
	if errors.Is(err, repository.ErrNotFound) {
		notFound(c, "供应商不存在，无法引入 SRM")
		return
	}
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	if req.AnnualSpend > 0 {
		result.Engagement.AnnualSpend = req.AnnualSpend
		h.ops.Store().UpsertEngagement(result.Engagement)
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// SRMEngagementStatus 推进合作关系状态（候选 → 邀请 → 合作中 → 暂缓 / 终止）。
func (h *Handler) SRMEngagementStatus(c *gin.Context) {
	var req struct {
		TenantID   string `json:"tenant_id"`
		SupplierID string `json:"supplier_id"`
		Status     string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求格式无效")
		return
	}
	e, err := h.ops.SetEngagementStatus(req.TenantID, req.SupplierID, req.Status)
	if errors.Is(err, repository.ErrNotFound) {
		notFound(c, "合作关系不存在")
		return
	}
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": e})
}

// SRMOrderScorecards 逐单 QCDST 评分卡：这张单执行得怎么样，哪一维判不了、为什么。
func (h *Handler) SRMOrderScorecards(c *gin.Context) {
	supplierID := c.Query("supplier")
	if supplierID == "" {
		badRequest(c, "supplier 参数必填")
		return
	}
	result, err := h.ops.OrderScorecards(c.Request.Context(), c.Query("tenant"), supplierID)
	if errors.Is(err, repository.ErrNotFound) {
		notFound(c, "供应商不存在")
		return
	}
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// SRMEnsureTenant 登记/确认一家采购方企业。真实系统里这一步是注册流程，
// 这里只保证"第一次用"不会因为没租户而卡住。
//
// 未带 id 时按时间生成一个，避免第二家企业建档时撞上第一家。
func (h *Handler) SRMEnsureTenant(c *gin.Context) {
	var req domain.Tenant
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "企业信息格式无效")
		return
	}
	if req.Name == "" {
		badRequest(c, "name 必填")
		return
	}
	if req.ID == "" {
		req.ID = fmt.Sprintf("t-%d", time.Now().UnixMilli())
	}
	t := h.ops.Store().EnsureTenant(req)
	c.JSON(http.StatusOK, gin.H{"data": t})
}

// ---------------- 闭环：公开池 ⇄ 私有域 ----------------

// QCDSTBriefs 寻源页用的索引：每家供应商的平台评级 + 当前企业的合作状态。
func (h *Handler) QCDSTBriefs(c *gin.Context) {
	index := h.ops.BriefIndex(c.Request.Context(), c.Query("tenant"))
	if index.Items == nil {
		index.Items = map[string]domain.SupplierBrief{}
	}
	c.JSON(http.StatusOK, gin.H{"data": index})
}

// QCDSTLoop 闭环体检报告：平台池有多大、能不能信、本企业贡献了多少。
func (h *Handler) QCDSTLoop(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": h.ops.FeedbackLoop(c.Request.Context(), c.Query("tenant"))})
}

func (h *Handler) SRMCreateQualityEvent(c *gin.Context) {
	var req domain.QualityEvent
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "质量事件格式无效")
		return
	}
	if req.SupplierID == "" {
		badRequest(c, "supplier_id 必填")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": h.ops.RecordQualityEvent(req)})
}

// ---------------- BOM ----------------

// BomProducts 产品 BOM 目录：产品 → 大类 → 小类 → 部件。
func (h *Handler) BomProducts(c *gin.Context) {
	sector := c.Query("sector")
	items := make([]gin.H, 0, len(catalog.BomProducts))
	for _, p := range catalog.BomProducts {
		if sector != "" && p.Sector != sector {
			continue
		}
		items = append(items, gin.H{
			"code": p.Code, "name": p.Name, "sector": p.Sector, "aliases": p.Aliases,
			"summary": p.Summary, "part_count": p.PartCount, "top_parts": p.TopParts,
			"groups": len(p.Groups), "capabilities": p.Capabilities,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"items":      items,
			"sectors":    catalog.BomSectors(),
			"facets":     catalog.BomFacets(),
			"categories": catalog.PartCategories,
		},
	})
}

func (h *Handler) BomProductDetail(c *gin.Context) {
	p, ok := catalog.BomProductByCode(c.Param("code"))
	if !ok {
		notFound(c, "未收录该产品")
		return
	}
	// 把每个部件小类的说明一起带上，前端不用再查一次字典。
	cats := map[string]domain.PartCategory{}
	for _, cc := range catalog.PartCategories {
		cats[cc.Code] = cc
	}
	groups := make([]gin.H, 0, len(p.Groups))
	for _, g := range p.Groups {
		parts := make([]gin.H, 0, len(g.Parts))
		for _, pt := range g.Parts {
			parts = append(parts, gin.H{
				"code": pt.Code, "name": pt.Name,
				"category": pt.Category, "category_name": pt.CategoryName,
				"qty": pt.Qty, "critical": pt.Critical,
				"capability": pt.Capability, "note": pt.Note,
			})
		}
		groups = append(groups, gin.H{
			"code": g.Code, "name": g.Name, "role": g.Role,
			"critical": g.Critical, "parts": parts,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"product": p, "groups": groups, "total_parts": p.PartCount,
		},
	})
}

// BomFacets 小类视角：每个部件小类被多少产品用到、有多少关键件。
func (h *Handler) BomFacets(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": catalog.BomFacets()})
}

// BomPartSupply 小类供应源：这一类料谁能供、覆盖度够不够、各自评级多少。
// 这是把 BOM 和公开寻源池接起来的那一层——从"要什么料"直接走到"找谁买"。
func (h *Handler) BomPartSupply(c *gin.Context) {
	category := c.Query("category")
	if category == "" {
		badRequest(c, "category 必填")
		return
	}
	res, err := h.ops.PartSupply(c.Request.Context(), category,
		c.Query("tenant"), c.Query("region"), c.Query("capability"), c.Query("product"))
	if err != nil {
		notFound(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": res, "meta": gin.H{"demo": h.ops.Store().Demo()}})
}

// BomProductSupply 产品供应覆盖矩阵：做整机之前先看哪些模块会卡脖子。
func (h *Handler) BomProductSupply(c *gin.Context) {
	res, err := h.ops.ProductSupplyMatrix(c.Request.Context(), c.Param("code"),
		c.Query("tenant"), c.Query("region"))
	if err != nil {
		notFound(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": res})
}

// SupplierPartCoverage 供应商反查：这家能覆盖哪些部件小类与具体部件。
func (h *Handler) SupplierPartCoverage(c *gin.Context) {
	res, err := h.ops.SupplierPartCoverage(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c, "未找到该供应商")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": res})
}

// BomMatch 用一句自然语言产品描述匹配已收录的 BOM。
func (h *Handler) BomMatch(c *gin.Context) {
	q := c.Query("q")
	p, ok := catalog.MatchBomProduct(q)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"matched": false}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"matched": true, "code": p.Code, "name": p.Name, "sector": p.Sector,
		"part_count": p.PartCount, "groups": len(p.Groups),
	}})
}

func badRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_REQUEST", "message": msg}})
}

func notFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": msg}})
}

// forbidden 越权写入：没有合作关系、企业未建档、关系已终止。
// 与 badRequest 分开是为了让调用方能区分"我没说清楚"和"你不该这么做"。
func forbidden(c *gin.Context, msg string) {
	c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "NOT_ALLOWED", "message": msg}})
}

func conflict(c *gin.Context, msg string) {
	c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "DUPLICATE", "message": msg}})
}
