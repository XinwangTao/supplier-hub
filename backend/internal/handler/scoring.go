package handler

import (
	"net/http"
	"strconv"

	"github.com/example/supplier-hub/backend/internal/policy"
	"github.com/gin-gonic/gin"
)

// ScoringRanking 综合总榜/品类榜(?capability= 省略即总榜)。
// 与搜索同一评分管线;榜单场景匹配维度=能力覆盖度(scoring 包兜底)。
func (h *Handler) ScoringRanking(c *gin.Context) {
	capability := c.Query("capability")
	page := clampInt(c.Query("page"), 1, 1, 1<<30)
	pageSize := clampInt(c.Query("page_size"), 20, 1, policy.MaxPageSize)

	list := h.scoring.Ranking(c.Request.Context(), capability)
	start := (page - 1) * pageSize
	if start > len(list) {
		start = len(list)
	}
	end := start + pageSize
	if end > len(list) {
		end = len(list)
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"items":       list[start:end],
		"total":       len(list),
		"page":        page,
		"page_size":   pageSize,
		"capability":  capability,
		"weights":     gin.H{"qcdst": 0.40, "match": 0.30, "registry": 0.15, "field": 0.15},
	}})
}

func clampInt(raw string, def, min, max int) int {
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
