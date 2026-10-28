package handler

import (
	"net/http"

	"github.com/example/supplier-hub/backend/internal/domain"
	"github.com/example/supplier-hub/backend/internal/fourm1e"
	"github.com/gin-gonic/gin"
)

// FourM1EGet 返回某供应商当前生效的五维档案。从未录入时 data 为 null,
// 前端据此显示「—」而不是渲染一个看似完整的空档案。
func (h *Handler) FourM1EGet(c *gin.Context) {
	rec, ok := h.fourm1e.Current(c.Param("supplierId"))
	count, lastBy, lastAt := h.fourm1e.SummarizeHistory(c.Param("supplierId"))
	var payload any
	if ok {
		payload = rec
	}
	c.JSON(http.StatusOK, gin.H{"data": payload, "history_count": count, "last_recorded_by": lastBy, "last_recorded_at": lastAt})
}

// FourM1ESave 保存一条巡厂录入快照:先校验(节点④),通过则追加留痕(节点⑤)。
// RecordedAt 由服务端盖戳,客户端给的录入了也不信任时间。
func (h *Handler) FourM1ESave(c *gin.Context) {
	var rec domain.FourM1E
	if err := c.ShouldBindJSON(&rec); err != nil {
		badRequest(c, err.Error())
		return
	}
	rec.SupplierID = c.Param("supplierId")
	rec.RecordedAt = ""
	if err := fourm1e.Validate(&rec); err != nil {
		badRequest(c, err.Error())
		return
	}
	saved := h.fourm1e.Append(rec)
	c.JSON(http.StatusOK, gin.H{"data": saved})
}

// FourM1EHistory 返回全部历史快照(旧→新),供"录入记录可回查"。
func (h *Handler) FourM1EHistory(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": h.fourm1e.History(c.Param("supplierId"))})
}
