package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/example/supplier-hub/backend/internal/catalog"
	"github.com/example/supplier-hub/backend/internal/domain"
)

// 这个文件回答一个寻源里最常被问、也最容易答错的问题：
//   "做这个产品，这一类料谁能供？"
//
// BOM 给的是"要什么"，供应商池给的是"谁能做"。中间差一层匹配，
// 而匹配的口径决定结果能不能用。这里定死三条：
//
//  1. 适配分只衡量"能不能做、离得近不近"，不含评级分。
//     评级是历史表现，适配是能力覆盖。混在一起，就会出现"分数很高但
//     工艺根本不对口"的供应商排在前面——这是选错供应商最常见的原因。
//  2. 没有评级的供应商不按 0 分处理，而是单独标注"暂无"。
//     新供应商没有历史数据，不等于能力差，不能让它在排序里被系统性埋掉。
//  3. 覆盖度比最优供应商更重要。
//     一个小类只有 1 家能供，那家的分再高也是单点风险；界面必须先报覆盖，
//     再列候选。

// 适配分权重。能力覆盖是硬门槛，区域只做微调。
const (
	weightCapability = 0.7
	weightRegion     = 0.3
)

// normPlace 统一"深圳市 / 深圳 / 广东省 / 广东"这几种写法，避免同省被误判成跨区。
func normPlace(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "市")
	s = strings.TrimSuffix(s, "省")
	return s
}

func samePlace(a, b string) bool {
	na, nb := normPlace(a), normPlace(b)
	return na != "" && na == nb
}

// provinceOfName 由城市名或省名解析出所属省份，解析不出返回空。
func provinceOfName(name string) string {
	n := normPlace(name)
	if n == "" {
		return ""
	}
	for _, c := range catalog.Cities {
		if normPlace(c.Name) == n || normPlace(c.Province) == n {
			return c.Province
		}
	}
	return ""
}

// regionScore 区域分。同城协同效率最高，跨区不等于不行，只是要多留运输与差旅时间。
// 传省名或市名都可以：传"广东省"时，省内所有城市都算同省。
func regionScore(supplier domain.Supplier, region string) (float64, string) {
	if strings.TrimSpace(region) == "" {
		return 70, "未指定"
	}
	if samePlace(supplier.City, region) {
		return 100, "同城"
	}
	supProv := supplier.Province
	if supProv == "" {
		supProv = provinceOfName(supplier.City)
	}
	regionProv := provinceOfName(region)
	if supProv != "" && regionProv != "" && normPlace(supProv) == normPlace(regionProv) {
		return 75, "同省"
	}
	return 55, "跨区"
}

func capabilityName(code string) string {
	for _, c := range catalog.Capabilities {
		if c.Code == code {
			if c.Short != "" {
				return c.Short
			}
			return c.Name
		}
	}
	return code
}

// matchCategory 判断一家供应商对某小类的适配程度。hitCount 为 0 表示不对口。
func matchCategory(sup domain.Supplier, cat domain.PartCategory, primary string, region string) (domain.PartSupplierMatch, bool) {
	own := map[string]bool{}
	for _, c := range sup.Capabilities {
		own[c] = true
	}
	need := append([]string{}, cat.KeyCapabilities...)
	if primary != "" {
		found := false
		for _, c := range need {
			if c == primary {
				found = true
				break
			}
		}
		if !found {
			need = append(need, primary)
		}
	}
	if len(need) == 0 {
		return domain.PartSupplierMatch{}, false
	}

	hit, miss := []string{}, []string{}
	for _, c := range need {
		if own[c] {
			hit = append(hit, c)
		} else {
			miss = append(miss, c)
		}
	}
	if len(hit) == 0 {
		// 工艺完全不搭边就不算候选。宁可承认"这一类没人能供"，
		// 也不要凑一堆勉强沾边的企业假装供应源充足。
		return domain.PartSupplierMatch{}, false
	}

	capScore := float64(len(hit)) / float64(len(need)) * 100
	reasons := []string{}
	for _, c := range hit {
		if c == primary {
			reasons = append(reasons, fmt.Sprintf("具备主工艺：%s", capabilityName(c)))
		} else {
			reasons = append(reasons, fmt.Sprintf("覆盖工艺：%s", capabilityName(c)))
		}
	}
	// 主工艺命中额外加权：能做出这个部件的关键工序，比"顺带能做"更有说服力。
	if primary != "" && own[primary] {
		capScore += 15
		if capScore > 100 {
			capScore = 100
		}
	}
	if len(miss) > 0 {
		names := make([]string, 0, len(miss))
		for _, c := range miss {
			names = append(names, capabilityName(c))
		}
		reasons = append(reasons, fmt.Sprintf("缺工艺：%s（需外协或工序拆分）", strings.Join(names, "、")))
	}

	rScore, regionTag := regionScore(sup, region)
	fit := capScore*weightCapability + rScore*weightRegion
	if regionTag != "未指定" {
		reasons = append(reasons, fmt.Sprintf("%s（%s）", regionTag, joinCity(sup)))
	}
	if sup.IsManufacturer {
		reasons = append(reasons, "生产型厂家，可直采")
	} else {
		reasons = append(reasons, "贸易/服务商，需确认实际产线归属")
	}

	return domain.PartSupplierMatch{
		SupplierID:       sup.ID,
		Name:             sup.Name,
		ShortName:        sup.ShortName,
		City:             sup.City,
		Province:         sup.Province,
		IsMaker:          sup.IsManufacturer,
		LeadTimeDays:     sup.LeadTimeDays,
		MOQ:              sup.MOQ,
		FitScore:         round1(fit),
		FitReasons:       reasons,
		HitCapabilities:  hit,
		MissCapabilities: miss,
		CapabilityRatio:  round2(float64(len(hit)) / float64(len(need))),
		RegionTag:        regionTag,
		MainProducts:     sup.MainProducts,
	}, true
}

func joinCity(sup domain.Supplier) string {
	if sup.City != "" {
		return sup.City
	}
	return sup.Province
}

// ratingPool 一次算好全平台的评级，避免矩阵里对每个小类重复聚合。
func (s *SupplierOpsService) ratingPool(ctx context.Context) map[string]domain.PlatformRating {
	out := map[string]domain.PlatformRating{}
	for _, r := range s.PlatformRatings(ctx) {
		out[r.SupplierID] = r
	}
	return out
}

// usableRating 只有过了观察期（有正式等级）的分数才拿来做横向比较。
func usableRating(r domain.PlatformRating) *domain.PlatformRating {
	if r.Total < 0 || r.Provisional || r.Grade == domain.GradeNA {
		return nil
	}
	cp := r
	return &cp
}

func riskOf(suppliers int, critical bool) (domain.CoverageRisk, string, string) {
	switch {
	case suppliers == 0:
		return domain.CoverageNone, "缺口", "公开池里没有对口供应商，需要自行开发或调整设计选型"
	case suppliers == 1:
		if critical {
			return domain.CoverageSingle, "单点（关键件）", "关键件单点供应：断供即停线，必须在量产前锁定第二源或替代料"
		}
		return domain.CoverageSingle, "单点", "只有一家可供应，议价与切换空间都为零，建议同步开发备份源"
	case suppliers <= 3:
		return domain.CoverageNarrow, "偏窄", "可选供应商偏少，建议再储备 1-2 家并做小批量验证"
	default:
		return domain.CoverageAdequate, "充足", "可选供应商充足，按评级与适配分择优即可"
	}
}

// worstRisk 取一组风险里最差的那个，用于给模块定级。
func worstRisk(list []domain.CoverageRisk) domain.CoverageRisk {
	rank := map[domain.CoverageRisk]int{
		domain.CoverageNone:     0,
		domain.CoverageSingle:   1,
		domain.CoverageNarrow:   2,
		domain.CoverageAdequate: 3,
	}
	worst := domain.CoverageAdequate
	for _, r := range list {
		if rank[r] < rank[worst] {
			worst = r
		}
	}
	return worst
}

// PartSupply 某小类的供应源清单。product 为空时按全库统计需求面。
func (s *SupplierOpsService) PartSupply(ctx context.Context, category, tenantID, region, primary, productCode string) (domain.PartSupply, error) {
	cat, ok := catalog.CategoryByCode(category)
	if !ok {
		return domain.PartSupply{}, fmt.Errorf("未收录的部件小类：%s", category)
	}
	suppliers, err := s.repo.ListAll(ctx)
	if err != nil {
		return domain.PartSupply{}, err
	}
	ratings := s.ratingPool(ctx)

	items := make([]domain.PartSupplierMatch, 0, len(suppliers))
	demand := []domain.BomPart{}
	critical := false
	products := map[string]bool{}
	if productCode != "" {
		p, ok := catalog.BomProductByCode(productCode)
		if !ok {
			return domain.PartSupply{}, fmt.Errorf("未收录的产品：%s", productCode)
		}
		for _, g := range p.Groups {
			for _, pt := range g.Parts {
				if pt.Category != category {
					continue
				}
				demand = append(demand, pt)
				if pt.Critical {
					critical = true
				}
				if primary == "" && pt.Capability != "" {
					primary = pt.Capability
				}
			}
		}
		products[p.Name] = true
	} else {
		// 不限产品时，用全库统计该小类的需求面（哪些产品会用到、多少关键件）。
		for _, p := range catalog.BomProducts {
			for _, g := range p.Groups {
				for _, pt := range g.Parts {
					if pt.Category == category {
						products[p.Name] = true
						if pt.Critical {
							critical = true
						}
					}
				}
			}
		}
	}

	for _, sup := range suppliers {
		m, ok := matchCategory(sup, cat, primary, region)
		if !ok {
			continue
		}
		if r, ok := ratings[sup.ID]; ok {
			if u := usableRating(r); u != nil {
				cp := *u
				m.Rating = &cp
			}
		}
		if tenantID != "" {
			if e, ok := s.store.EngagementOf(tenantID, sup.ID); ok {
				cp := e
				m.Engagement = &cp
			}
		}
		items = append(items, m)
	}
	// 默认按适配分排序：先确认"能不能做"，再看历史表现。
	sort.Slice(items, func(i, j int) bool {
		if items[i].FitScore != items[j].FitScore {
			return items[i].FitScore > items[j].FitScore
		}
		return items[i].SupplierID < items[j].SupplierID
	})

	bestScore, bestFit, top := -1.0, 0.0, ""
	rated, engaged := 0, 0
	for _, it := range items {
		if it.Rating != nil {
			rated++
			if it.Rating.Total > bestScore {
				bestScore = it.Rating.Total
			}
		}
		if it.Engagement != nil && it.Engagement.Status != string(domain.EngagementTerminated) {
			engaged++
		}
		if it.FitScore > bestFit {
			bestFit = it.FitScore
		}
	}
	// 最优供应商先看评级；没人有评级时退化为适配分最高的那家，并说明原因。
	for _, it := range items {
		if it.Rating != nil && it.Rating.Total == bestScore {
			top = it.ShortName
			if top == "" {
				top = it.Name
			}
			break
		}
	}
	if top == "" {
		for _, it := range items {
			if it.FitScore == bestFit {
				top = it.ShortName
				if top == "" {
					top = it.Name
				}
				break
			}
		}
	}

	risk, label, advice := riskOf(len(items), critical)
	productNames := make([]string, 0, len(products))
	for k := range products {
		productNames = append(productNames, k)
	}
	sort.Strings(productNames)

	note := "适配分 = 工艺覆盖 70% + 区域 30%，不含评级分；评级单独展示，避免用历史分数掩盖工艺不对口。"
	if len(items) > 0 && rated == 0 {
		note += "本小类暂无供应商有正式评级（多为未合作或样本不足），不能据此判断优劣。"
	}

	return domain.PartSupply{
		Coverage: domain.PartCoverage{
			Category:    cat,
			Parts:       len(demand),
			Critical:    critical,
			Suppliers:   len(items),
			Rated:       rated,
			Engaged:     engaged,
			BestScore:   round1(bestScore),
			BestFit:     round1(bestFit),
			Risk:        string(risk),
			RiskLabel:   label,
			Advice:      advice,
			TopSupplier: top,
		},
		Items:       items,
		DemandParts: demand,
		Capability:  primary,
		Region:      region,
		Products:    productNames,
		Note:        note,
	}, nil
}

// partAgg 同一小类在一组部件里的聚合结果。
type partAgg struct {
	sample   domain.BomPart // 代表部件，用于取小类与主工艺名称
	primary  string         // 该小类下出现最多的主工艺；关键件的工艺优先
	count    int
	critical bool
}

// aggregateParts 把一组部件按小类合并。主工艺取该小类下出现次数最多的那个，
// 关键件的主工艺优先——关键件要用什么工艺，决定了这一类料真正该找谁。
func aggregateParts(parts []domain.BomPart) []partAgg {
	order := []string{}
	agg := map[string]*partAgg{}
	capCount := map[string]map[string]int{}
	for _, pt := range parts {
		if _, ok := agg[pt.Category]; !ok {
			agg[pt.Category] = &partAgg{sample: pt}
			capCount[pt.Category] = map[string]int{}
			order = append(order, pt.Category)
		}
		a := agg[pt.Category]
		a.count++
		if pt.Critical {
			a.critical = true
			a.sample = pt // 关键件更能代表这一类料的工艺要求
		}
		if pt.Capability != "" {
			capCount[pt.Category][pt.Capability]++
		}
	}
	out := make([]partAgg, 0, len(order))
	for _, code := range order {
		a := agg[code]
		best, bestN := "", 0
		for cap, n := range capCount[code] {
			// 关键件的主工艺直接采用，不必比次数。
			if a.sample.Capability == cap {
				best = cap
				break
			}
			if n > bestN {
				best, bestN = cap, n
			}
		}
		a.primary = best
		out = append(out, *a)
	}
	return out
}

// ProductSupplyMatrix 一款产品的供应覆盖矩阵：按大类 → 小类给出覆盖体检。
//
// 做产品之前先看这张表。它的价值不在于"找供应商"，而在于提前暴露
// 哪些模块是卡脖子项——等到量产前才发现某颗料单点供应，代价是改板或停线。
func (s *SupplierOpsService) ProductSupplyMatrix(ctx context.Context, productCode, tenantID, region string) (domain.ProductSupplyMatrix, error) {
	p, ok := catalog.BomProductByCode(productCode)
	if !ok {
		return domain.ProductSupplyMatrix{}, fmt.Errorf("未收录的产品：%s", productCode)
	}
	suppliers, err := s.repo.ListAll(ctx)
	if err != nil {
		return domain.ProductSupplyMatrix{}, err
	}
	ratings := s.ratingPool(ctx)

	// 先按小类建一次候选索引，产品里重复出现的小类不用重复匹配。
	type candItem struct {
		match  domain.PartSupplierMatch
		rating *domain.PlatformRating
	}
	cand := map[string][]candItem{}
	ensure := func(code, primary string, critical bool) []candItem {
		key := code + "|" + primary
		if list, ok := cand[key]; ok {
			return list
		}
		cat, ok := catalog.CategoryByCode(code)
		if !ok {
			cand[key] = nil
			return nil
		}
		list := []candItem{}
		for _, sup := range suppliers {
			m, ok := matchCategory(sup, cat, primary, region)
			if !ok {
				continue
			}
			var rt *domain.PlatformRating
			if r, ok := ratings[sup.ID]; ok {
				rt = usableRating(r)
			}
			list = append(list, candItem{match: m, rating: rt})
		}
		cand[key] = list
		return list
	}

	groups := make([]domain.GroupSupply, 0, len(p.Groups))
	cats := 0
	gaps, singles, criticalGaps, secondSource := 0, 0, 0, 0
	for _, g := range p.Groups {
		covs := []domain.PartCoverage{}
		risks := []domain.CoverageRisk{}
		// 按小类聚合：同一个模块里"PCB 与基板"只能出现一行，
		// 否则同一小类会因为不同部件的主工艺不同被拆成多行，
		// 既让小类总数虚高，也让用户误以为有两个独立的供应缺口。
		for _, agg := range aggregateParts(g.Parts) {
			pt := agg.sample
			list := ensure(pt.Category, agg.primary, agg.critical)
			critical := agg.critical
			rated, engaged := 0, 0
			bestScore, bestFit, top := -1.0, 0.0, ""
			for _, c := range list {
				if c.rating != nil {
					rated++
					if c.rating.Total > bestScore {
						bestScore = c.rating.Total
					}
				}
				if c.match.FitScore > bestFit {
					bestFit = c.match.FitScore
				}
			}
			for _, c := range list {
				if c.rating != nil && c.rating.Total == bestScore {
					top = c.match.ShortName
					if top == "" {
						top = c.match.Name
					}
					break
				}
			}
			if top == "" {
				for _, c := range list {
					if c.match.FitScore == bestFit {
						top = c.match.ShortName
						if top == "" {
							top = c.match.Name
						}
						break
					}
				}
			}
			if tenantID != "" {
				for _, c := range list {
					if e, ok := s.store.EngagementOf(tenantID, c.match.SupplierID); ok &&
						e.Status != string(domain.EngagementTerminated) {
						engaged++
					}
				}
			}
			risk, label, advice := riskOf(len(list), critical)
			risks = append(risks, risk)
			cat, _ := catalog.CategoryByCode(pt.Category)
			covs = append(covs, domain.PartCoverage{
				Category:    cat,
				Parts:       agg.count,
				Critical:    critical,
				Suppliers:   len(list),
				Rated:       rated,
				Engaged:     engaged,
				BestScore:   round1(bestScore),
				BestFit:     round1(bestFit),
				Risk:        string(risk),
				RiskLabel:   label,
				Advice:      advice,
				TopSupplier: top,
			})
			cats++
			switch risk {
			case domain.CoverageNone:
				gaps++
				if critical {
					criticalGaps++
				}
			case domain.CoverageSingle:
				singles++
				if critical {
					criticalGaps++
				}
			default:
				secondSource++
			}
		}
		sort.Slice(covs, func(i, j int) bool {
			ri, rj := rankRisk(covs[i].Risk), rankRisk(covs[j].Risk)
			if ri != rj {
				return ri < rj
			}
			return covs[i].Suppliers < covs[j].Suppliers
		})
		groups = append(groups, domain.GroupSupply{
			Code: g.Code, Name: g.Name, Role: g.Role, Critical: g.Critical,
			Coverages: covs, Risk: string(worstRisk(risks)),
		})
	}

	rate := 0.0
	if cats > 0 {
		rate = round1(float64(secondSource) / float64(cats) * 100)
	}
	headline := ""
	switch {
	case criticalGaps > 0:
		headline = fmt.Sprintf("%d 个关键件小类存在缺供或单点，量产前必须先解决第二源", criticalGaps)
	case gaps > 0:
		headline = fmt.Sprintf("%d 个小类在公开池里找不到对口供应商，需要自行开发", gaps)
	case singles > 0:
		headline = fmt.Sprintf("%d 个小类仅有单一供应源，建议同步储备备份供应商", singles)
	default:
		headline = "全部小类均有 2 家以上可选供应商，可按评级与适配分择优"
	}

	return domain.ProductSupplyMatrix{
		Product: p,
		Groups:  groups,
		Summary: domain.SupplySummary{
			Categories:       cats,
			Gaps:             gaps,
			SinglePoints:     singles,
			CriticalGaps:     criticalGaps,
			CoverageRate:     rate,
			SecondSourceRate: rate,
			Headline:         headline,
		},
		Region:   region,
		TenantID: tenantID,
		Demo:     s.store.Demo(),
	}, nil
}

func rankRisk(r string) int {
	switch domain.CoverageRisk(r) {
	case domain.CoverageNone:
		return 0
	case domain.CoverageSingle:
		return 1
	case domain.CoverageNarrow:
		return 2
	default:
		return 3
	}
}

// SupplierPartCoverage 供应商反查：这家能做哪些小类、哪些具体部件。
// 用途是判断"这家在我这条产品线上的通用性有多高"——能覆盖的小类越多，
// 整合它带来的管理成本越低。
func (s *SupplierOpsService) SupplierPartCoverage(ctx context.Context, supplierID string) (domain.SupplierPartCoverage, error) {
	sup, err := s.repo.GetByID(ctx, supplierID)
	if err != nil {
		return domain.SupplierPartCoverage{}, err
	}
	suppliers, err := s.repo.ListAll(ctx)
	if err != nil {
		return domain.SupplierPartCoverage{}, err
	}
	ratings := s.ratingPool(ctx)

	own := map[string]bool{}
	for _, c := range sup.Capabilities {
		own[c] = true
	}

	type catAgg struct {
		cat      domain.PartCategory
		products map[string]bool
		parts    int
		critical bool
		rivals   int
	}
	agg := map[string]*catAgg{}
	parts := []domain.SupplierPartHit{}
	directParts, indirectParts := 0, 0

	for _, p := range catalog.BomProducts {
		for _, g := range p.Groups {
			for _, pt := range g.Parts {
				// 直接命中：部件主工艺被这家覆盖——这是它真能自己做的。
				// 间接命中：只沾上小类的关键工艺（例如 PCBA 厂"能覆盖"电机类，
				// 实际只是驱动器带 PCBA），必须分开记，否则会夸大能力边界。
				direct := pt.Capability != "" && own[pt.Capability]
				basis := ""
				if direct {
					basis = fmt.Sprintf("主工艺：%s", capabilityName(pt.Capability))
				} else {
					cat, ok := catalog.CategoryByCode(pt.Category)
					if !ok {
						continue
					}
					for _, c := range cat.KeyCapabilities {
						if own[c] {
							basis = fmt.Sprintf("小类工艺：%s（非该件主工艺）", capabilityName(c))
							break
						}
					}
				}
				if basis == "" {
					continue
				}
				cat, _ := catalog.CategoryByCode(pt.Category)
				a, ok := agg[pt.Category]
				if !ok {
					a = &catAgg{cat: cat, products: map[string]bool{}}
					agg[pt.Category] = a
				}
				a.parts++
				a.products[p.Name] = true
				if pt.Critical {
					a.critical = true
				}
				// 全量统计：headline 里的"可自制 / 需外协"必须反映这家真实的能力边界，
				// 不能因为列表只展示 40 条就把需外协的部分说成 0。
				if direct {
					directParts++
				} else {
					indirectParts++
				}
				parts = append(parts, domain.SupplierPartHit{
					Part: pt, ProductCode: p.Code, ProductName: p.Name, GroupName: g.Name,
					Direct: direct, Basis: basis,
				})
			}
		}
	}

	cats := make([]domain.PartCoverage, 0, len(agg))
	totalRival, criticalCount := 0, 0
	for code, a := range agg {
		// 同行家数：该小类下有多少家供应商同样对口（含自己）。
		rivals := 0
		bestScore := -1.0
		for _, other := range suppliers {
			if _, ok := matchCategory(other, a.cat, "", ""); !ok {
				continue
			}
			rivals++
			if r, ok := ratings[other.ID]; ok {
				if u := usableRating(r); u != nil && u.Total > bestScore {
					bestScore = u.Total
				}
			}
		}
		a.rivals = rivals
		totalRival += rivals
		if a.critical {
			criticalCount++
		}
		risk, label, advice := riskOf(rivals, a.critical)
		prods := make([]string, 0, len(a.products))
		for k := range a.products {
			prods = append(prods, k)
		}
		sort.Strings(prods)
		_ = code
		cats = append(cats, domain.PartCoverage{
			Category:  a.cat,
			Parts:     a.parts,
			Critical:  a.critical,
			Suppliers: rivals,
			Rated:     0,
			BestScore: round1(bestScore),
			Risk:      string(risk),
			RiskLabel: label,
			Advice:    advice,
		})
	}
	sort.Slice(cats, func(i, j int) bool {
		if cats[i].Parts != cats[j].Parts {
			return cats[i].Parts > cats[j].Parts
		}
		return cats[i].Category.Code < cats[j].Category.Code
	})
	totalParts := len(parts)
	// 关键件优先，其次主工艺直接命中的（能自制的才是重点）；一次最多给 40 条，避免刷屏。
	sort.SliceStable(parts, func(i, j int) bool {
		if parts[i].Part.Critical != parts[j].Part.Critical {
			return parts[i].Part.Critical
		}
		if parts[i].Direct != parts[j].Direct {
			return parts[i].Direct
		}
		return parts[i].ProductCode < parts[j].ProductCode
	})
	shown := len(parts)
	if len(parts) > 40 {
		parts = parts[:40]
		shown = 40
	}

	avgRival := 0.0
	if len(cats) > 0 {
		avgRival = round1(float64(totalRival) / float64(len(cats)))
	}
	headline := fmt.Sprintf("可覆盖 %d 个部件小类、%d 个具体部件：%d 个由其主工艺直接命中（可自制），"+
		"%d 个仅因小类工艺相关（多半需外协）；其中 %d 类涉及关键件，"+
		"这些小类平均有 %.1f 家同行同样具备能力（数字越小越难被替代）。",
		len(cats), totalParts, directParts, indirectParts, criticalCount, avgRival)
	if totalParts > shown {
		headline += fmt.Sprintf("下表按关键件与可自制优先列出 %d 条。", shown)
	}

	return domain.SupplierPartCoverage{
		Supplier:   sup,
		Categories: cats,
		Parts:      parts,
		Summary: domain.SupplierCoverSummary{
			Categories:    len(cats),
			Parts:         totalParts,
			DirectParts:   directParts,
			IndirectParts: indirectParts,
			Critical:      criticalCount,
			AvgRivalry:    avgRival,
			Headline:      headline,
		},
	}, nil
}
