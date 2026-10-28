package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/example/supplier-hub/backend/internal/catalog"
	"github.com/example/supplier-hub/backend/internal/domain"
	"github.com/example/supplier-hub/backend/internal/repository"
)

// GraphService 负责把"产品需求"翻译成"供应链网络"：
// 产品 → 模块 → 工序 → 供应商，再把供应商之间用替代关系、同城协同关系连起来，
// 得到一张以产品为中心向外发散的关联网。
type GraphService struct {
	repo repository.SupplierRepository
}

func NewGraphService(repo repository.SupplierRepository) *GraphService {
	return &GraphService{repo: repo}
}

// BuildProductGraph 生成产品发散网。query 可以是自然语言，例如「我想做一款智能手表」。
func (g *GraphService) BuildProductGraph(ctx context.Context, req domain.GraphRequest) (domain.Graph, error) {
	product := strings.TrimSpace(req.Product)
	if product == "" {
		product = strings.TrimSpace(req.Query)
	}
	query := strings.TrimSpace(req.Query)

	recipe, matched := catalog.MatchRecipe(product)
	if !matched {
		recipe = catalog.GenericRecipe(cleanProductName(product))
	}
	limit := req.MaxSuppliersPerProcess
	if limit <= 0 {
		limit = 2
	}
	if limit > 5 {
		limit = 5
	}

	nodes := make([]domain.GraphNode, 0, 96)
	edges := make([]domain.GraphEdge, 0, 160)
	edgeSeq := 0
	addEdge := func(source, target, relation, label string, weight float64) {
		edgeSeq++
		edges = append(edges, domain.GraphEdge{
			ID: fmt.Sprintf("e-%03d", edgeSeq), Source: source, Target: target,
			Relation: relation, Label: label, Weight: weight,
		})
	}

	nodes = append(nodes, domain.GraphNode{
		ID: "product", Type: domain.NodeProduct, Label: recipe.Product, Level: 0,
		Subtitle: recipe.Category + " · " + fmt.Sprintf("%d 个模块", len(recipe.Modules)),
		Meta:     map[string]string{"category": recipe.Category, "summary": recipe.Summary},
	})

	supplierNodes := map[string]*domain.GraphNode{}
	var supplierOrder []string
	usedCapabilities := map[string]bool{}
	totalProcesses, gapCount := 0, 0
	var gaps, singleSources []string
	criticalSingle := []string{}
	processSuppliers := map[string]int{}

	for mi, module := range recipe.Modules {
		moduleID := fmt.Sprintf("module-%d", mi+1)
		nodes = append(nodes, domain.GraphNode{
			ID: moduleID, Type: domain.NodeModule, Label: module.Name, Level: 1, ParentID: "product",
			Subtitle: fmt.Sprintf("%d 道工序", len(module.Processes)),
			Meta:     map[string]string{"desc": module.Desc},
		})
		addEdge("product", moduleID, "bom", "包含模块", 1)

		for pi, proc := range module.Processes {
			if req.CriticalOnly && !proc.Critical {
				continue
			}
			processID := fmt.Sprintf("process-%d-%d", mi+1, pi+1)
			capInfo, _ := catalog.CapabilityByCode(proc.Capability)
			usedCapabilities[proc.Capability] = true

			candidates, err := g.repo.ListByCapability(ctx, proc.Capability)
			if err != nil {
				return domain.Graph{}, err
			}
			candidates = filterByRegion(candidates, req.Region)
			rankSuppliers(candidates, proc.Critical)
			available := len(candidates)
			if len(candidates) > limit {
				candidates = candidates[:limit]
			}

			nodes = append(nodes, domain.GraphNode{
				ID: processID, Type: domain.NodeProcess, Label: proc.Name, Level: 2, ParentID: moduleID,
				Capability: proc.Capability,
				Subtitle:   capInfo.Name,
				Meta: map[string]string{
					"spec": proc.Spec, "qty": proc.Qty, "capability_name": capInfo.Name,
					"capability_desc": capInfo.Description, "critical": boolLabel(proc.Critical),
					"available": fmt.Sprintf("%d", available), "shown": fmt.Sprintf("%d", len(candidates)),
				},
			})
			addEdge(moduleID, processID, "process", "工序", 0.9)

			totalProcesses++
			processSuppliers[processID] = len(candidates)
			if available == 0 {
				gapCount++
				gaps = append(gaps, proc.Name)
				continue
			}
			if available == 1 {
				singleSources = append(singleSources, proc.Name)
				if proc.Critical {
					criticalSingle = append(criticalSingle, proc.Name)
				}
			}
			for _, s := range candidates {
				score := fitScore(s, proc.Critical)
				if existing, ok := supplierNodes[s.ID]; ok {
					if score > existing.Score {
						existing.Score = score
					}
				} else {
					node := &domain.GraphNode{
						ID: s.ID, Type: domain.NodeSupplier, Label: supplierLabel(s), Level: 3, ParentID: processID,
						SupplierID: s.ID, Risk: s.RiskLevel, Score: score, Capability: proc.Capability,
						Subtitle: s.Name,
						Meta: map[string]string{
							"legal_representative": s.LegalRepresentative,
							"city":                 s.City,
							"capital":              fmt.Sprintf("%d", s.RegisteredCapital),
							"employees":            fmt.Sprintf("%d", s.EmployeeCount),
							"moq":                  fmt.Sprintf("%d", s.MOQ),
							"lead_time":            fmt.Sprintf("%d", s.LeadTimeDays),
						},
					}
					supplierNodes[s.ID] = node
					supplierOrder = append(supplierOrder, s.ID)
				}
				addEdge(processID, s.ID, "supply", "可承接", float64(score)/100)
			}
		}
	}

	supplierMap := map[string]domain.Supplier{}
	all, err := g.repo.ListAll(ctx)
	if err != nil {
		return domain.Graph{}, err
	}
	for _, s := range all {
		supplierMap[s.ID] = s
	}

	for _, id := range supplierOrder {
		nodes = append(nodes, *supplierNodes[id])
	}

	// 替代关系：同一工艺能力下的供应商互为备选，这是网络"横向连接"的主要来源。
	groups := map[string][]string{}
	for _, id := range supplierOrder {
		node := supplierNodes[id]
		groups[node.Capability] = append(groups[node.Capability], id)
	}
	for _, ids := range groups {
		if len(ids) < 2 {
			continue
		}
		sort.Strings(ids)
		for i := 0; i < len(ids); i++ {
			addEdge(ids[i], ids[(i+1)%len(ids)], "alternative", "同工艺可替代", 0.45)
		}
	}

	// 同城协同：同城不同工序的供应商，往往共享模具、表面处理等本地配套资源。
	cityGroups := map[string][]string{}
	for _, id := range supplierOrder {
		s := supplierMap[id]
		cityGroups[s.City] = append(cityGroups[s.City], id)
	}
	cityEdgeBudget := 8
	cities := make([]string, 0, len(cityGroups))
	for city := range cityGroups {
		cities = append(cities, city)
	}
	sort.Strings(cities)
	for _, city := range cities {
		if cityEdgeBudget <= 0 {
			break
		}
		ids := cityGroups[city]
		if len(ids) < 2 {
			continue
		}
		for i := 0; i+1 < len(ids) && cityEdgeBudget > 0; i++ {
			a, b := supplierNodes[ids[i]], supplierNodes[ids[i+1]]
			if a.Capability == b.Capability {
				continue
			}
			addEdge(ids[i], ids[i+1], "same_city", "同城协同", 0.25)
			cityEdgeBudget--
		}
	}

	stats := domain.GraphStats{
		Modules: len(recipe.Modules), Processes: totalProcesses, Suppliers: len(supplierOrder),
		HighRisk: 0, Capabilities: len(usedCapabilities), Edges: len(edges),
	}
	citySet := map[string]bool{}
	for _, id := range supplierOrder {
		s := supplierMap[id]
		citySet[s.City] = true
		if s.RiskLevel == domain.RiskHigh || s.RiskLevel == domain.RiskCritical {
			stats.HighRisk++
		}
	}
	stats.Cities = len(citySet)

	insights := buildProductInsights(recipe, gaps, singleSources, criticalSingle, supplierOrder, supplierMap, stats, req.Region)

	return domain.Graph{
		Mode: "product", Query: query, Product: recipe.Product, Summary: recipe.Summary, Center: "product",
		Nodes: nodes, Edges: edges, Stats: stats, Insights: insights, GeneratedAt: time.Now(),
	}, nil
}

// BuildMesh 生成全域网：以工艺能力为锚点，产品作为入口挂在其上。
// 用户「突然想做一个智能手表」，就是在这张网上点亮一个产品节点，再展开它的上下游。
func (g *GraphService) BuildMesh(ctx context.Context) (domain.Graph, error) {
	all, err := g.repo.ListAll(ctx)
	if err != nil {
		return domain.Graph{}, err
	}
	supplierCount := map[string]int{}
	for _, s := range all {
		for _, c := range s.Capabilities {
			supplierCount[c]++
		}
	}

	nodes := make([]domain.GraphNode, 0, 48)
	edges := make([]domain.GraphEdge, 0, 120)
	edgeSeq := 0
	addEdge := func(source, target, relation string, weight float64) {
		edgeSeq++
		edges = append(edges, domain.GraphEdge{ID: fmt.Sprintf("e-%03d", edgeSeq), Source: source, Target: target, Relation: relation, Weight: weight})
	}

	// 工艺能力节点
	for _, c := range catalog.Capabilities {
		nodes = append(nodes, domain.GraphNode{
			ID: "cap-" + c.Code, Type: domain.NodeCapability, Label: c.Name, Level: 1,
			Capability: c.Code,
			Subtitle:   fmt.Sprintf("%s · %d 家供应商", c.Category, supplierCount[c.Code]),
			Meta: map[string]string{
				"category": c.Category, "desc": c.Description, "regions": strings.Join(c.Regions, "、"),
				"suppliers": fmt.Sprintf("%d", supplierCount[c.Code]),
			},
		})
	}

	// 产品节点 + 产品对能力的引用
	pairCount := map[string]int{}
	for _, r := range catalog.Recipes {
		productID := "product-" + r.Product
		caps := map[string]bool{}
		critical := map[string]bool{}
		for _, m := range r.Modules {
			for _, p := range m.Processes {
				caps[p.Capability] = true
				if p.Critical {
					critical[p.Capability] = true
				}
			}
		}
		nodes = append(nodes, domain.GraphNode{
			ID: productID, Type: domain.NodeProduct, Label: r.Product, Level: 0,
			Subtitle: fmt.Sprintf("%s · %d 类工艺", r.Category, len(caps)),
			Meta:     map[string]string{"category": r.Category, "summary": r.Summary},
		})
		for code := range caps {
			weight := 0.6
			if critical[code] {
				weight = 1
			}
			addEdge(productID, "cap-"+code, "uses", weight)
		}
		codeList := make([]string, 0, len(caps))
		for code := range caps {
			codeList = append(codeList, code)
		}
		for i := 0; i < len(codeList); i++ {
			for j := i + 1; j < len(codeList); j++ {
				pair := codeList[i] + "|" + codeList[j]
				pairCount[pair]++
			}
		}
	}

	// 能力之间的共现关系：经常出现在同一个产品里的工艺，往往共享客户与产能。
	type partner struct {
		code  string
		count int
	}
	partners := map[string][]partner{}
	for pair, count := range pairCount {
		if count < 3 {
			continue
		}
		parts := strings.Split(pair, "|")
		partners[parts[0]] = append(partners[parts[0]], partner{parts[1], count})
		partners[parts[1]] = append(partners[parts[1]], partner{parts[0], count})
	}
	seenPair := map[string]bool{}
	for code, list := range partners {
		sort.Slice(list, func(i, j int) bool {
			if list[i].count != list[j].count {
				return list[i].count > list[j].count
			}
			return list[i].code < list[j].code
		})
		if len(list) > 2 {
			list = list[:2]
		}
		for _, p := range list {
			a, b := code, p.code
			if a > b {
				a, b = b, a
			}
			key := a + "|" + b
			if seenPair[key] {
				continue
			}
			seenPair[key] = true
			addEdge("cap-"+a, "cap-"+b, "cooccur", float64(p.count)/4)
		}
	}

	stats := domain.GraphStats{
		Modules: 0, Processes: 0, Suppliers: len(all), Capabilities: len(catalog.Capabilities),
		Edges: len(edges), Cities: countCities(all), HighRisk: countHighRisk(all),
	}
	insights := buildMeshInsights(stats, supplierCount)

	return domain.Graph{
		Mode: "mesh", Query: "", Product: "", Center: "",
		Summary: fmt.Sprintf("全域网：%d 类工艺能力、%d 个产品入口、%d 家供应商。点击产品节点即可展开它的供应链。", len(catalog.Capabilities), len(catalog.Recipes), len(all)),
		Nodes:   nodes, Edges: edges, Stats: stats, Insights: insights, GeneratedAt: time.Now(),
	}, nil
}

// BuildSupplierGraph 生成某家供应商的自我中心网：它能做什么、还有谁能替代它。
func (g *GraphService) BuildSupplierGraph(ctx context.Context, id string) (domain.Graph, error) {
	target, err := g.repo.GetByID(ctx, id)
	if err != nil {
		return domain.Graph{}, err
	}
	nodes := []domain.GraphNode{{
		ID: target.ID, Type: domain.NodeSupplier, Label: supplierLabel(target), Level: 0, SupplierID: target.ID,
		Risk: target.RiskLevel, Subtitle: target.City + " · 法定代表人 " + target.LegalRepresentative,
	}}
	edges := make([]domain.GraphEdge, 0, 32)
	edgeSeq := 0
	addEdge := func(source, targetID, relation string, weight float64) {
		edgeSeq++
		edges = append(edges, domain.GraphEdge{ID: fmt.Sprintf("e-%03d", edgeSeq), Source: source, Target: targetID, Relation: relation, Weight: weight})
	}

	for _, code := range target.Capabilities {
		capInfo, ok := catalog.CapabilityByCode(code)
		if !ok {
			continue
		}
		capID := "cap-" + code
		nodes = append(nodes, domain.GraphNode{
			ID: capID, Type: domain.NodeCapability, Label: capInfo.Name, Level: 1, Capability: code,
			Subtitle: capInfo.Category, Meta: map[string]string{"desc": capInfo.Description},
		})
		addEdge(target.ID, capID, "supply", 1)

		siblings, err := g.repo.ListByCapability(ctx, code)
		if err != nil {
			return domain.Graph{}, err
		}
		rankSuppliers(siblings, false)
		count := 0
		for _, s := range siblings {
			if s.ID == target.ID {
				continue
			}
			if count >= 4 {
				break
			}
			nodes = append(nodes, domain.GraphNode{
				ID: s.ID, Type: domain.NodeSupplier, Label: supplierLabel(s), Level: 2, SupplierID: s.ID,
				Risk: s.RiskLevel, Score: fitScore(s, false), Capability: code,
				Subtitle: s.City + " · 法定代表人 " + s.LegalRepresentative,
			})
			addEdge(capID, s.ID, "alternative", 0.5)
			count++
		}
	}

	stats := domain.GraphStats{Suppliers: len(nodes) - 1 - len(target.Capabilities), Capabilities: len(target.Capabilities), Edges: len(edges)}
	insights := []domain.Insight{{
		Kind: "info", Level: "info", Title: "可替代性",
		Detail: fmt.Sprintf("%s 共挂接 %d 类工艺能力，图中展示了同类工艺的备选供应商，可用于比价与产能备份。", target.Name, len(target.Capabilities)),
	}}
	return domain.Graph{
		Mode: "supplier", Query: target.Name, Product: "", Center: target.ID,
		Summary: target.Name + " 的能力与备选供应商关系。",
		Nodes:   nodes, Edges: edges, Stats: stats, Insights: insights, GeneratedAt: time.Now(),
	}, nil
}

// Products 返回产品库清单，用于前端示例与产品选择。
func (g *GraphService) Products(ctx context.Context) []domain.ProductSummary {
	all, _ := g.repo.ListAll(ctx)
	capSuppliers := map[string]int{}
	for _, s := range all {
		for _, c := range s.Capabilities {
			capSuppliers[c]++
		}
	}
	out := make([]domain.ProductSummary, 0, len(catalog.Recipes))
	for _, r := range catalog.Recipes {
		caps := map[string]bool{}
		processes := 0
		for _, m := range r.Modules {
			for _, p := range m.Processes {
				caps[p.Capability] = true
				processes++
			}
		}
		reachable := map[string]bool{}
		for code := range caps {
			if capSuppliers[code] > 0 {
				reachable[code] = true
			}
		}
		out = append(out, domain.ProductSummary{
			Product: r.Product, Category: r.Category, Summary: r.Summary, Aliases: r.Aliases,
			Modules: len(r.Modules), Processes: processes, Capabilities: len(reachable),
			Suppliers: countReachableSuppliers(all, caps),
		})
	}
	return out
}

func countReachableSuppliers(all []domain.Supplier, caps map[string]bool) int {
	seen := map[string]bool{}
	for _, s := range all {
		for _, c := range s.Capabilities {
			if caps[c] {
				seen[s.ID] = true
				break
			}
		}
	}
	return len(seen)
}

func cleanProductName(query string) string {
	name := strings.TrimSpace(query)
	for _, prefix := range []string{"我想做", "我想做一个", "我想做一款", "我要做", "帮我做", "想做", "做一款", "开发一款", "一款", "一个"} {
		name = strings.TrimPrefix(name, prefix)
	}
	name = strings.TrimSuffix(name, "。")
	return strings.TrimSpace(name)
}

func filterByRegion(items []domain.Supplier, region string) []domain.Supplier {
	region = strings.TrimSpace(region)
	if region == "" {
		return items
	}
	out := make([]domain.Supplier, 0, len(items))
	for _, s := range items {
		if strings.Contains(s.Province, region) || strings.Contains(s.City, region) {
			out = append(out, s)
		}
	}
	return out
}

func rankSuppliers(items []domain.Supplier, critical bool) {
	sort.SliceStable(items, func(i, j int) bool {
		si, sj := fitScore(items[i], critical), fitScore(items[j], critical)
		if si != sj {
			return si > sj
		}
		return items[i].EmployeeCount > items[j].EmployeeCount
	})
}

// fitScore 是"这家企业能不能接这个工序"的粗排分数，用于网络图里挑选代表供应商。
func fitScore(s domain.Supplier, critical bool) int {
	score := 62
	switch s.RiskLevel {
	case domain.RiskLow:
		score += 14
	case domain.RiskMedium:
		score += 4
	default:
		score -= 10
	}
	if s.IsManufacturer {
		score += 6
	}
	switch {
	case s.EmployeeCount >= 500:
		score += 9
	case s.EmployeeCount >= 200:
		score += 5
	}
	if s.RegisteredCapital >= 3000 {
		score += 4
	}
	if s.InventionPatents >= 8 {
		score += 3
	}
	if critical && s.LeadTimeDays > 35 {
		score -= 4
	}
	if score > 98 {
		score = 98
	}
	if score < 48 {
		score = 48
	}
	return score
}

func buildProductInsights(recipe catalog.Recipe, gaps, singleSources, criticalSingle []string, supplierOrder []string,
	supplierMap map[string]domain.Supplier, stats domain.GraphStats, region string) []domain.Insight {
	insights := []domain.Insight{}
	regionLabel := ""
	if strings.TrimSpace(region) != "" {
		regionLabel = "（限定区域：" + region + "）"
	}

	if len(criticalSingle) > 0 {
		insights = append(insights, domain.Insight{
			Kind: "single_source", Level: "critical",
			Title:  fmt.Sprintf("%d 道关键工序只有单一供应源", len(criticalSingle)),
			Detail: strings.Join(criticalSingle, "、") + " 目前在图中仅有 1 家候选供应商，任一环节停产都会直接卡住量产，建议至少再补 1 家备选。" + regionLabel,
		})
	}
	if nonCritical := diffNames(singleSources, criticalSingle); len(nonCritical) > 0 {
		insights = append(insights, domain.Insight{
			Kind: "single_source", Level: "info",
			Title:  fmt.Sprintf("%d 道非关键工序同样只有 1 家备选", len(nonCritical)),
			Detail: strings.Join(nonCritical, "、") + " 的可替代性较低，建议在打样阶段同步接触第二供应商，避免量产后被动。" + regionLabel,
		})
	}
	if len(gaps) > 0 {
		insights = append(insights, domain.Insight{
			Kind: "gap", Level: "warn",
			Title:  fmt.Sprintf("%d 道工序在库内没有匹配供应商", len(gaps)),
			Detail: strings.Join(gaps, "、") + " 需要补充供应商库或放宽区域限制。" + regionLabel,
		})
	}

	// 区域集中度：单一城市占比过高，物流与招工风险集中。
	cityCount := map[string]int{}
	for _, id := range supplierOrder {
		cityCount[supplierMap[id].City]++
	}
	if len(supplierOrder) >= 6 {
		topCity, topCount := "", 0
		for city, count := range cityCount {
			if count > topCount {
				topCity, topCount = city, count
			}
		}
		share := topCount * 100 / len(supplierOrder)
		if share >= 35 {
			insights = append(insights, domain.Insight{
				Kind: "concentration", Level: "warn",
				Title:  fmt.Sprintf("供应链集中在%s", topCity),
				Detail: fmt.Sprintf("%d 家候选供应商中有 %d 家位于%s（%d%%），一旦遇到限电、招工或物流中断，整条链会同步受影响。", len(supplierOrder), topCount, topCity, share),
			})
		}
	}

	if stats.HighRisk > 0 {
		insights = append(insights, domain.Insight{
			Kind: "risk", Level: "warn", Title: fmt.Sprintf("%d 家高风险供应商在链上", stats.HighRisk),
			Detail: "图中红色节点存在行政处罚或涉诉标签，若它们出现在关键工序上，建议优先替换或补充第二供应源。",
		})
	}

	// 跨产品复用：同一工艺被多个产品共用，旺季容易排队。
	reuse := map[string]int{}
	capName := map[string]string{}
	for _, r := range catalog.Recipes {
		if r.Product == recipe.Product {
			continue
		}
		seen := map[string]bool{}
		for _, m := range r.Modules {
			for _, p := range m.Processes {
				if seen[p.Capability] {
					continue
				}
				seen[p.Capability] = true
				reuse[p.Capability]++
				if c, ok := catalog.CapabilityByCode(p.Capability); ok {
					capName[p.Capability] = c.Name
				}
			}
		}
	}
	type reuseItem struct {
		code  string
		count int
	}
	items := make([]reuseItem, 0, len(reuse))
	for code, count := range reuse {
		if count >= 3 {
			items = append(items, reuseItem{code, count})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].count > items[j].count })
	if len(items) > 0 {
		top := items[0]
		insights = append(insights, domain.Insight{
			Kind: "reuse", Level: "info",
			Title:  fmt.Sprintf("%s 是跨产品共用工序", capName[top.code]),
			Detail: fmt.Sprintf("该工艺同时出现在另外 %d 个产品的供应链里，属于产能紧俏环节，谈价与排产建议提前锁量。", top.count),
		})
	}

	if len(insights) == 0 {
		insights = append(insights, domain.Insight{
			Kind: "strength", Level: "info", Title: "供应链分布均衡",
			Detail: "关键工序均有 2 家以上候选供应商，区域分布也较分散，属于可推进打样的状态。",
		})
	}
	return insights
}

func buildMeshInsights(stats domain.GraphStats, supplierCount map[string]int) []domain.Insight {
	insights := []domain.Insight{}
	type capItem struct {
		code, name string
		count      int
	}
	items := make([]capItem, 0, len(catalog.Capabilities))
	for _, c := range catalog.Capabilities {
		item := capItem{c.Code, c.Name, supplierCount[c.Code]}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].count < items[j].count })
	if len(items) > 0 {
		thin := items[0]
		insights = append(insights, domain.Insight{
			Kind: "bottleneck", Level: "warn", Title: "最薄的工艺环节：" + thin.name,
			Detail: fmt.Sprintf("全网只有 %d 家供应商能承接，扩品时最容易成为瓶颈。", thin.count),
		})
	}
	insights = append(insights, domain.Insight{
		Kind: "info", Level: "info", Title: fmt.Sprintf("全网 %d 家供应商、覆盖 %d 座城市", stats.Suppliers, stats.Cities),
		Detail: "点击任一产品节点，可以展开它从模块到供应商的完整链路；点击工艺能力节点，可以看到跨产品复用的供应商池。",
	})
	return insights
}

func countCities(items []domain.Supplier) int {
	set := map[string]bool{}
	for _, s := range items {
		set[s.City] = true
	}
	return len(set)
}

func countHighRisk(items []domain.Supplier) int {
	n := 0
	for _, s := range items {
		if s.RiskLevel == domain.RiskHigh || s.RiskLevel == domain.RiskCritical {
			n++
		}
	}
	return n
}

// supplierLabel 图上只展示简称，全称放到 subtitle，避免网络图标签过长糊成一片。
func supplierLabel(s domain.Supplier) string {
	if strings.TrimSpace(s.ShortName) != "" {
		return s.ShortName
	}
	return strings.TrimSuffix(s.Name, "有限公司")
}

func boolLabel(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func diffNames(all, exclude []string) []string {
	skip := map[string]bool{}
	for _, v := range exclude {
		skip[v] = true
	}
	out := make([]string, 0, len(all))
	for _, v := range all {
		if !skip[v] {
			out = append(out, v)
		}
	}
	return out
}

// Capabilities 返回工艺能力库：每条能力带供应商规模与"被哪些产品用到"，
// 这样能力节点就成了产品与供应商之间的公共枢纽。
func (g *GraphService) Capabilities(ctx context.Context) []domain.CapabilitySummary {
	all, _ := g.repo.ListAll(ctx)
	counts := map[string]int{}
	for _, s := range all {
		for _, code := range s.Capabilities {
			counts[code]++
		}
	}

	usedBy := map[string][]string{}
	for _, recipe := range catalog.Recipes {
		seen := map[string]bool{}
		for _, module := range recipe.Modules {
			for _, proc := range module.Processes {
				if seen[proc.Capability] {
					continue
				}
				seen[proc.Capability] = true
				usedBy[proc.Capability] = append(usedBy[proc.Capability], recipe.Product)
			}
		}
	}

	out := make([]domain.CapabilitySummary, 0, len(catalog.Capabilities))
	for _, capInfo := range catalog.Capabilities {
		products := usedBy[capInfo.Code]
		if products == nil {
			products = []string{}
		}
		processes := capInfo.Processes
		if processes == nil {
			processes = []string{}
		}
		regions := capInfo.Regions
		if regions == nil {
			regions = []string{}
		}
		out = append(out, domain.CapabilitySummary{
			Code: capInfo.Code, Name: capInfo.Name, Category: capInfo.Category,
			Description: capInfo.Description, Processes: processes, Regions: regions,
			Suppliers: counts[capInfo.Code], Products: products,
		})
	}
	return out
}
