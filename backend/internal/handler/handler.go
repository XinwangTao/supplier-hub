package handler

import (
	"errors"
	"github.com/example/supplier-hub/backend/internal/domain"
	"github.com/example/supplier-hub/backend/internal/repository"
	"github.com/example/supplier-hub/backend/internal/service"
	"github.com/gin-gonic/gin"
	"net/http"
)

type Handler struct {
	suppliers *service.SupplierService
	audits    *service.AuditService
}

func New(suppliers *service.SupplierService, audits *service.AuditService) *Handler {
	return &Handler{suppliers: suppliers, audits: audits}
}
func (h *Handler) Search(c *gin.Context) {
	var req domain.SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_REQUEST", "message": "请求格式无效"}})
		return
	}
	result, err := h.suppliers.Search(c.Request.Context(), req)
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}
func (h *Handler) Get(c *gin.Context) {
	result, err := h.suppliers.Get(c.Request.Context(), c.Param("id"))
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "供应商不存在"}})
		return
	}
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}
func (h *Handler) Checklist(c *gin.Context) {
	result, err := h.audits.Generate(c.Request.Context(), c.Param("id"))
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "供应商不存在"}})
		return
	}
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}
func serverError(c *gin.Context, err error) {
	c.Error(err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "服务暂时不可用"}})
}
