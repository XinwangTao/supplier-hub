package main

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/example/supplier-hub/backend/internal/config"
	"github.com/example/supplier-hub/backend/internal/domain"
	"github.com/example/supplier-hub/backend/internal/fourm1e"
	"github.com/example/supplier-hub/backend/internal/handler"
	"github.com/example/supplier-hub/backend/internal/policy"
	"github.com/example/supplier-hub/backend/internal/repository"
	"github.com/example/supplier-hub/backend/internal/service"
	"github.com/example/supplier-hub/backend/internal/srm"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	dataset, meta, err := repository.BuildDataset(context.Background(), cfg.Dataset())
	if err != nil {
		// 拿不到真实数据时也要能启动：把原因如实带给前端，而不是退回编造数据。
		log.Printf("[dataset] 真实数据源装配失败: %v", err)
		dataset = []domain.Supplier{}
		meta = repository.DatasetMeta{
			Mode: "error", IsReal: false, Count: 0, Source: "",
			Message: "真实数据源装配失败：" + err.Error(),
		}
	}
	log.Printf("[dataset] mode=%s real=%v count=%d source=%s", meta.Mode, meta.IsReal, meta.Count, meta.Source)

	repo := repository.NewMemorySupplierRepository(dataset)
	suppliers := service.NewSupplierService(repo)
	audits := service.NewAuditService(suppliers)
	graphs := service.NewGraphService(repo)
	srmStore := srm.NewStore()
	ops := service.NewSupplierOpsService(repo, srmStore)
	// 巡厂五维档案落在挂载卷里(data/ 已映射到宿主机 backend/data/),重启不丢
	m1eStore := fourm1e.NewStore("data/fourm1e.json")
	scoringSvc := service.NewScoringService(repo, ops, m1eStore)
	h := handler.New(suppliers, audits, graphs, ops, scoringSvc, m1eStore, meta)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), cors(cfg.CORSOrigins))
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok", "service": "supplier-hub",
			"dataset": gin.H{"mode": meta.Mode, "is_real": meta.IsReal, "count": meta.Count},
		})
	})
	api := r.Group("/api/v1")
	// 每个响应带 trace_id（泄露可溯源），并按来源限流（拦住脚本拖库）。
	// 这两件事必须挂在总入口上，挂到单个 handler 里必然漏。
	api.Use(policy.Watermark(), policy.NewLimiter(policy.RequestsPerMin, time.Minute).Middleware())
	api.GET("/meta/policy", h.Policy)
	api.GET("/meta/dataset", h.Dataset)
	api.POST("/suppliers/search", h.Search)
	// 美团式综合分榜单:总榜(capability 省略)/ 32 类品类榜,与搜索同一评分管线
	api.GET("/scoring/ranking", h.ScoringRanking)
	api.GET("/suppliers/:id", h.Get)
	api.POST("/suppliers/:id/audit-checklist", h.Checklist)
	api.POST("/graph/build", h.BuildGraph)
	api.GET("/graph/mesh", h.Mesh)
	api.GET("/graph/supplier/:id", h.SupplierGraph)
	api.GET("/catalog/products", h.Products)
	api.GET("/catalog/capabilities", h.Capabilities)

	// QCDST 供应商能力评估
	api.GET("/qcdst/ranking", h.QCDSTRanking)
	api.GET("/qcdst/supplier/:id", h.QCDSTSupplier)
	// 闭环索引：寻源页要知道"这家在平台上评多少分、我有没有引入过"
	api.GET("/qcdst/briefs", h.QCDSTBriefs)
	api.GET("/qcdst/loop", h.QCDSTLoop)

	// 轻量 SRM：企业侧工作台与数据录入
	api.GET("/srm/workspace", h.SRMWorkspace)
	// 供应商五维档案(人机料法环):巡厂人工录入的事实数据,快照留痕可回查
	api.GET("/srm/4m1e/:supplierId", h.FourM1EGet)
	api.POST("/srm/4m1e/:supplierId", h.FourM1ESave)
	api.GET("/srm/4m1e/:supplierId/history", h.FourM1EHistory)
	api.GET("/srm/tenants", h.SRMTenants)
	api.POST("/srm/tenants", h.SRMEnsureTenant)
	api.POST("/srm/engagements", h.SRMCreateEngagement)
	api.POST("/srm/engagements/status", h.SRMEngagementStatus)
	api.GET("/srm/orders", h.SRMOrderScorecards)
	api.POST("/srm/orders", h.SRMCreateOrder)
	api.POST("/srm/deliveries", h.SRMCreateDelivery)
	api.POST("/srm/quality-events", h.SRMCreateQualityEvent)
	api.GET("/srm/audit", h.SRMAudit)
	api.POST("/srm/demo/seed", h.SRMSeedDemo)
	api.POST("/srm/demo/clear", h.SRMClearDemo)

	// 产品 BOM 知识库（产品 → 大类 → 小类 → 部件）
	api.GET("/bom/products", h.BomProducts)
	api.GET("/bom/products/:code", h.BomProductDetail)
	api.GET("/bom/products/:code/supply", h.BomProductSupply)
	api.GET("/bom/facets", h.BomFacets)
	api.GET("/bom/match", h.BomMatch)
	api.GET("/bom/supply", h.BomPartSupply)
	api.GET("/supplier/:id/parts", h.SupplierPartCoverage)
	if err := r.Run(cfg.HTTPAddr); err != nil {
		panic(err)
	}
}

func cors(origins string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, origin := range strings.Split(origins, ",") {
		allowed[strings.TrimSpace(origin)] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Header("Vary", "Origin")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
