package service

import (
	"context"
	"sort"

	"github.com/example/supplier-hub/backend/internal/domain"
	"github.com/example/supplier-hub/backend/internal/fourm1e"
	"github.com/example/supplier-hub/backend/internal/repository"
	"github.com/example/supplier-hub/backend/internal/scoring"
)

// ScoringService 美团式四维综合评分:把 QCDST(履约)、匹配度、工商实力、
// 4M1E 巡厂档案四路既有事实合成一个分数。它只做合成,不改任何一路的口径。
type ScoringService struct {
	repo repository.SupplierRepository
	ops  *SupplierOpsService
	m1e  *fourm1e.Store
}

func NewScoringService(repo repository.SupplierRepository, ops *SupplierOpsService, m1e *fourm1e.Store) *ScoringService {
	return &ScoringService{repo: repo, ops: ops, m1e: m1e}
}

// qcdstInputFor 把平台评级翻译成 scoring 的三维输入口径:
// 没出现在评级表 = 无合作单据(FlagNoData);在表里但 Grade=NA = 样本不足。
func (s *ScoringService) qcdstInputFor(ratings map[string]domain.PlatformRating, supplierID string) float64 {
	r, ok := ratings[supplierID]
	if !ok {
		return 0 // 无数据
	}
	if r.Grade == domain.GradeNA {
		return -1 // 证据不足
	}
	return r.Total
}

// PlatformRatingsMap 评级表一次算好、多处复用:搜索 30 条候选若每条都全库重算
// 平台评级,是纯粹的计算浪费。
func (s *ScoringService) PlatformRatingsMap(ctx context.Context) map[string]domain.PlatformRating {
	ratings := s.ops.PlatformRatings(ctx)
	out := make(map[string]domain.PlatformRating, len(ratings))
	for _, r := range ratings {
		out[r.SupplierID] = r
	}
	return out
}

// ScoreSupplierWith 在已有评级表上给一家供应商算综合分(批量场景复用同一张表)。
func (s *ScoringService) ScoreSupplierWith(ratings map[string]domain.PlatformRating, supplier domain.Supplier, matchScore float64) domain.OverallScore {
	var m1e *domain.FourM1E
	if rec, ok := s.m1e.Current(supplier.ID); ok {
		m := rec
		m1e = &m
	}
	result := scoring.Compute(scoring.Input{
		QCDSTTotal: s.qcdstInputFor(ratings, supplier.ID),
		Supplier:   supplier,
		MatchScore: matchScore,
		M1E:        m1e,
	})
	return toOverall(result)
}

// ScoreSupplier 便捷版:单家供应商(内部自建评级表)。
func (s *ScoringService) ScoreSupplier(ctx context.Context, supplier domain.Supplier, matchScore float64) domain.OverallScore {
	return s.ScoreSupplierWith(s.PlatformRatingsMap(ctx), supplier, matchScore)
}

// Ranking 综合总榜/品类榜。capability 为空 = 总榜。
// 榜单没有需求上下文,匹配维度退化为能力覆盖度(scoring 包内兜底)。
func (s *ScoringService) Ranking(ctx context.Context, capability string) []domain.OverallScore {
	ratings := s.PlatformRatingsMap(ctx)
	suppliers, err := s.repo.ListAll(ctx)
	if err != nil {
		return []domain.OverallScore{}
	}
	out := make([]domain.OverallScore, 0, len(suppliers))
	for _, supplier := range suppliers {
		if capability != "" && !hasCapability(supplier, capability) {
			continue
		}
		var m1e *domain.FourM1E
		if rec, ok := s.m1e.Current(supplier.ID); ok {
			m := rec
			m1e = &m
		}
		result := scoring.Compute(scoring.Input{
			QCDSTTotal: s.qcdstInputFor(ratings, supplier.ID),
			Supplier:   supplier,
			MatchScore: -1,
			M1E:        m1e,
		})
		overall := toOverall(result)
		overall.SupplierID = supplier.ID
		overall.SupplierName = supplier.Name
		overall.ShortName = supplier.ShortName
		overall.CreditCode = supplier.CreditCode
		overall.City = supplier.City
		overall.Capabilities = supplier.Capabilities
		out = append(out, overall)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Total > out[j].Total })
	return out
}

// hasCapability 与 repository 层的能力过滤同口径:精确匹配能力代码。
func hasCapability(s domain.Supplier, code string) bool {
	for _, c := range s.Capabilities {
		if c == code {
			return true
		}
	}
	return false
}

func toOverall(r scoring.Result) domain.OverallScore {
	dims := make([]domain.OverallDimension, 0, len(r.Dimensions))
	for _, d := range r.Dimensions {
		dims = append(dims, domain.OverallDimension{
			Key: d.Key, Label: d.Label, Score: d.Score, Weight: d.Weight, Flag: d.Flag, Note: d.Note,
		})
	}
	return domain.OverallScore{
		Total: r.Total, Dimensions: dims, Flags: r.Flags,
		Confidence: confidenceOf(r),
	}
}

// confidenceOf 有几个维度"有真数据"——决定分数旁的置信标识:
// high=三维以上有据,medium=两维,low=最多一维(此时分数基本只是建档信息)。
func confidenceOf(r scoring.Result) string {
	real := 0
	for _, d := range r.Dimensions {
		if d.Flag == "" && d.Score > 0 {
			real++
		}
	}
	switch {
	case real >= 3:
		return "high"
	case real >= 2:
		return "medium"
	default:
		return "low"
	}
}
