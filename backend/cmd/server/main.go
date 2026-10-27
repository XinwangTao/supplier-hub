package main

import (
	"github.com/example/supplier-hub/backend/internal/config"
	"github.com/example/supplier-hub/backend/internal/handler"
	"github.com/example/supplier-hub/backend/internal/repository"
	"github.com/example/supplier-hub/backend/internal/service"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

func main() {
	cfg := config.Load()
	repo := repository.NewMemorySupplierRepository(repository.MockSuppliers())
	suppliers := service.NewSupplierService(repo)
	audits := service.NewAuditService(suppliers)
	h := handler.New(suppliers, audits)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), cors(cfg.CORSOrigins))
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "supplier-hub"}) })
	api := r.Group("/api/v1")
	api.POST("/suppliers/search", h.Search)
	api.GET("/suppliers/:id", h.Get)
	api.POST("/suppliers/:id/audit-checklist", h.Checklist)
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
