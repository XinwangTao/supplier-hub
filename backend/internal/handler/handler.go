package handler

import (
	"errors"
	"net/http"

	"github.com/example/supplier-hub/backend/internal/domain"
	"github.com/example/supplier-hub/backend/internal/policy"
	"github.com/example/supplier-hub/backend/internal/repository"
	"github.com/example/supplier-hub/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	suppliers *service.SupplierService
	audits    *service.AuditService
	graphs    *service.GraphService
	ops       *service.SupplierOpsService
	dataset   repository.DatasetMeta
}

func New(suppliers *service.SupplierService, audits *service.AuditService, graphs *service.GraphService,
	ops *service.SupplierOpsService, dataset repository.DatasetMeta) *Handler {
	return &Handler{suppliers: suppliers, audits: audits, graphs: graphs, ops: ops, dataset: dataset}
}

// Dataset 返回当前供应商数据集的来源与真实性标记。
// 前端据此决定要不要打"未接入真实数据源"的警示条——数据是不是真的，界面上必须说得清楚。
func (h *Handler) Dataset(c *gin.Context) {
	meta := h.dataset
	// 载入演示 SRM 数据后，库里确实多了几家供应商，元信息要跟着变，
	// 否则首页会显示"0 家"却又能点开节点，自相矛盾。
	if h.ops != nil && h.ops.Store().Demo() {
		meta.Mode = "demo"
		meta.IsReal = false
		meta.Count = len(h.ops.Store().SupplierIDs())
		meta.Source = "演示 SRM 数据集"
		meta.Message = "当前供应商来自演示数据集，不是真实企业（信用代码带 DEMO 前缀）。真实使用时请导入工商信息表。"
	}
	c.JSON(http.StatusOK, gin.H{"data": meta})
}

// Policy 对外声明数据开放策略（分层、字段边界、配额、可导出范围）。
//
// 规则必须可被外部读到：否则"我们不提供批量导出"只是内部共识，
// 客户与竞品都无从确认；而写进接口，它就成了一份可被引用的契约。
func (h *Handler) Policy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": policy.Describe()})
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

// BuildGraph 接收一句自然语言（例如「我想做智能手表」），返回以该产品为中心的发散网络。
func (h *Handler) BuildGraph(c *gin.Context) {
	var req domain.GraphRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_REQUEST", "message": "请求格式无效"}})
		return
	}
	graph, err := h.graphs.BuildProductGraph(c.Request.Context(), req)
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": graph})
}

// Mesh 返回全域网：所有工艺能力与产品入口构成的基础网络。
func (h *Handler) Mesh(c *gin.Context) {
	graph, err := h.graphs.BuildMesh(c.Request.Context())
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": graph})
}

// SupplierGraph 返回某家供应商的能力与备选供应商关系。
func (h *Handler) SupplierGraph(c *gin.Context) {
	graph, err := h.graphs.BuildSupplierGraph(c.Request.Context(), c.Param("id"))
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "供应商不存在"}})
		return
	}
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": graph})
}

// Products 返回可展开的产品库清单。
func (h *Handler) Products(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": h.graphs.Products(c.Request.Context())})
}

// Capabilities 返回工艺能力库与各自的产品引用关系。
func (h *Handler) Capabilities(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": h.graphs.Capabilities(c.Request.Context())})
}

func serverError(c *gin.Context, err error) {
	c.Error(err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "服务暂时不可用"}})
}
