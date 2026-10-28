package service

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/example/supplier-hub/backend/internal/domain"
	"github.com/example/supplier-hub/backend/internal/policy"
	"github.com/example/supplier-hub/backend/internal/repository"
)

type SupplierService struct{ repo repository.SupplierRepository }

func NewSupplierService(repo repository.SupplierRepository) *SupplierService {
	return &SupplierService{repo: repo}
}

func (s *SupplierService) Search(ctx context.Context, req domain.SearchRequest) (domain.SearchResult, error) {
	filters := req.Filters
	if req.Query != "" {
		parsed := ParseQuery(req.Query)
		filters = mergeFilters(parsed, filters)
	}
	if req.Page < 1 {
		req.Page = 1
	}
	// 单页条数由服务端强制：前端传多大都不作数。
	// 否则"批量导出"只需要把 page_size 调成 10000，一个请求就拖走全库。
	if req.PageSize < 1 || req.PageSize > policy.MaxPageSize {
		req.PageSize = 20
	}
	items, err := s.repo.Search(ctx, filters)
	if err != nil {
		return domain.SearchResult{}, err
	}
	scored := scoreSuppliers(items, filters)
	interpretation := explainFilters(filters)
	start := (req.Page - 1) * req.PageSize
	// 深翻页限制：未认证调用最多触及前 N 条，防止用 offset 遍历全库。
	// 这里不退化成空结果——返回可访问范围内的最后一页并说明原因，
	// 让用户知道"不是没有更多，是需要认证"，而不是以为数据就这么多。
	if start > policy.DeepPageFreeMax {
		start = policy.DeepPageFreeMax
		interpretation += " 已限制在公开层可访问的最后一页；继续翻页需完成企业认证。"
	}
	if start > len(scored) {
		start = len(scored)
	}
	end := start + req.PageSize
	if end > len(scored) {
		end = len(scored)
	}
	return domain.SearchResult{Items: scored[start:end], Total: len(scored), Page: req.Page, PageSize: req.PageSize, Applied: filters, Interpretation: interpretation}, nil
}

func (s *SupplierService) Get(ctx context.Context, id string) (domain.Supplier, error) {
	return s.repo.GetByID(ctx, id)
}

func ParseQuery(query string) domain.SearchFilters {
	f := domain.SearchFilters{}
	for _, province := range []string{"广东省", "江苏省", "浙江省", "山东省", "福建省"} {
		if strings.Contains(query, province) {
			f.Province = province
		}
	}
	cityAliases := map[string][]string{
		"东莞市": {"东莞市", "东莞"}, "深圳市": {"深圳市", "深圳"}, "苏州市": {"苏州市", "苏州"}, "宁波市": {"宁波市", "宁波"},
		"杭州市": {"杭州市", "杭州"}, "无锡市": {"无锡市", "无锡"}, "青岛市": {"青岛市", "青岛"}, "厦门市": {"厦门市", "厦门"},
	}
	for city, aliases := range cityAliases {
		for _, alias := range aliases {
			if strings.Contains(query, alias) {
				f.City = city
				break
			}
		}
	}
	keywords := []string{"注塑", "连接器", "铝合金", "自动化", "线束", "冲压", "新能源", "医疗器械"}
	for _, keyword := range keywords {
		if strings.Contains(query, keyword) {
			f.BusinessKeyword = keyword
			break
		}
	}
	if value := numberNear(query, `注册资本\s*(\d+)\s*万`); value > 0 {
		f.MinCapital = value
	}
	if value := numberNear(query, `员工(?:数|人数)?\s*(\d+)\s*人`); value > 0 {
		f.MinEmployees = int(value)
	}
	if strings.Contains(query, "生产厂家") || strings.Contains(query, "制造商") || strings.Contains(query, "生产型") {
		yes := true
		f.ManufacturerOnly = &yes
	}
	if strings.Contains(query, "低风险") || strings.Contains(query, "风险低") {
		f.MaxRisk = domain.RiskLow
	}
	if strings.Contains(query, "中风险") || strings.Contains(query, "风险可控") {
		f.MaxRisk = domain.RiskMedium
	}
	return f
}

func numberNear(query, pattern string) int64 {
	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(query)
	if len(match) < 2 {
		return 0
	}
	n, _ := strconv.ParseInt(match[1], 10, 64)
	return n
}

func mergeFilters(parsed, explicit domain.SearchFilters) domain.SearchFilters {
	if explicit.BusinessKeyword != "" {
		parsed.BusinessKeyword = explicit.BusinessKeyword
	}
	if explicit.Province != "" {
		parsed.Province = explicit.Province
	}
	if explicit.City != "" {
		parsed.City = explicit.City
	}
	if explicit.MinCapital > 0 {
		parsed.MinCapital = explicit.MinCapital
	}
	if explicit.MaxCapital > 0 {
		parsed.MaxCapital = explicit.MaxCapital
	}
	if explicit.MinEmployees > 0 {
		parsed.MinEmployees = explicit.MinEmployees
	}
	if explicit.MaxEmployees > 0 {
		parsed.MaxEmployees = explicit.MaxEmployees
	}
	if explicit.ManufacturerOnly != nil {
		parsed.ManufacturerOnly = explicit.ManufacturerOnly
	}
	if explicit.MaxRisk != "" {
		parsed.MaxRisk = explicit.MaxRisk
	}
	return parsed
}

func scoreSuppliers(items []domain.Supplier, f domain.SearchFilters) []domain.ScoredSupplier {
	out := make([]domain.ScoredSupplier, 0, len(items))
	for _, s := range items {
		score, reasons := 60, []string{}
		if f.BusinessKeyword != "" {
			score += 20
			reasons = append(reasons, "经营范围匹配")
		} else {
			score += 8
		}
		if f.City != "" && s.City == f.City {
			score += 10
			reasons = append(reasons, "地区精准匹配")
		} else if f.Province != "" && s.Province == f.Province {
			score += 6
			reasons = append(reasons, "省份匹配")
		}
		if s.IsManufacturer {
			score += 5
			reasons = append(reasons, "生产型企业")
		}
		if s.RiskLevel == domain.RiskLow {
			score += 5
			reasons = append(reasons, "风险较低")
		} else if s.RiskLevel == domain.RiskMedium {
			score += 1
		}
		if s.EmployeeCount >= 300 {
			score += 3
			reasons = append(reasons, "交付规模充足")
		}
		if score > 99 {
			score = 99
		}
		out = append(out, domain.ScoredSupplier{Supplier: s, MatchScore: score, Reasons: reasons})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].MatchScore != out[j].MatchScore {
			return out[i].MatchScore > out[j].MatchScore
		}
		return out[i].EmployeeCount > out[j].EmployeeCount
	})
	return out
}

func explainFilters(f domain.SearchFilters) string {
	parts := []string{}
	if f.BusinessKeyword != "" {
		parts = append(parts, "品类："+f.BusinessKeyword)
	}
	if f.Province != "" {
		parts = append(parts, "省份："+f.Province)
	}
	if f.City != "" {
		parts = append(parts, "城市："+f.City)
	}
	if f.MinCapital > 0 {
		parts = append(parts, fmt.Sprintf("注册资本≥%d万", f.MinCapital))
	}
	if f.MinEmployees > 0 {
		parts = append(parts, fmt.Sprintf("员工数≥%d人", f.MinEmployees))
	}
	if f.ManufacturerOnly != nil {
		parts = append(parts, "生产型企业")
	}
	if f.MaxRisk != "" {
		parts = append(parts, "风险≤"+string(f.MaxRisk))
	}
	if len(parts) == 0 {
		return "未设置筛选条件，按综合匹配度排序"
	}
	return strings.Join(parts, " · ")
}
