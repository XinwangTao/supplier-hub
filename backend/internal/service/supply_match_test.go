package service

import (
	"context"
	"testing"

	"github.com/example/supplier-hub/backend/internal/domain"
	"github.com/example/supplier-hub/backend/internal/repository"
	"github.com/example/supplier-hub/backend/internal/srm"
)

// 造一批能力各异的供应商，覆盖"完全对口 / 部分对口 / 完全不搭边"三种情况。
func newMatchOps() *SupplierOpsService {
	repo := repository.NewMemorySupplierRepository([]domain.Supplier{
		{ID: "s-full", Name: "全工艺电子", ShortName: "全工艺", City: "深圳市", Province: "广东省",
			IsManufacturer: true, Capabilities: []string{"pcba", "testing", "battery"}},
		{ID: "s-part", Name: "部分工艺电子", ShortName: "半工艺", City: "东莞市", Province: "广东省",
			IsManufacturer: true, Capabilities: []string{"pcba"}},
		{ID: "s-none", Name: "精密五金", ShortName: "五金", City: "苏州市", Province: "江苏省",
			IsManufacturer: true, Capabilities: []string{"cnc", "stamping"}},
	})
	return NewSupplierOpsService(repo, srm.NewStore())
}

// 工艺完全不搭边的企业不能出现在候选里。
// 宁可承认"这一类没人能供"，也不要凑一堆勉强沾边的企业假装供应源充足。
func TestPartSupply_ExcludesCapabilityMismatch(t *testing.T) {
	ops := newMatchOps()
	res, err := ops.PartSupply(context.Background(), "soc", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 2 {
		t.Fatalf("soc 需要 pcba+testing，只有 2 家对口，got %d", len(res.Items))
	}
	for _, it := range res.Items {
		if it.SupplierID == "s-none" {
			t.Fatal("工艺不搭边的供应商不应成为候选")
		}
	}
}

// 适配分必须反映工艺覆盖度：全工艺覆盖 > 部分覆盖，且不掺评级分。
func TestPartSupply_FitScoreReflectsCapabilityCoverage(t *testing.T) {
	ops := newMatchOps()
	res, err := ops.PartSupply(context.Background(), "soc", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	full, part := 0.0, 0.0
	for _, it := range res.Items {
		switch it.SupplierID {
		case "s-full":
			full = it.FitScore
		case "s-part":
			part = it.FitScore
		}
	}
	if full <= part {
		t.Fatalf("工艺覆盖更全的适配分应更高：full=%.1f part=%.1f", full, part)
	}
	if full > 100 {
		t.Fatalf("适配分不应超过 100，got %.1f", full)
	}
	// 没有任何企业合作过时，所有候选都不得带评级，且 rated 计数为 0。
	if res.Coverage.Rated != 0 {
		t.Fatalf("无业务单据时不应有评级，got %d", res.Coverage.Rated)
	}
}

// 0 家对口 = 缺口，1 家 = 单点，4 家以上 = 充足。
// 这个分级决定界面是"去找供应商"还是"去开发备份源"，不能含糊。
func TestPartSupply_CoverageRiskGrading(t *testing.T) {
	// 全部不对口的小类：造一个只有 cnc 供应商的池子去问 rf 小类。
	repo := repository.NewMemorySupplierRepository([]domain.Supplier{
		{ID: "s1", Name: "五金厂", Capabilities: []string{"cnc"}},
	})
	ops := NewSupplierOpsService(repo, srm.NewStore())
	res, err := ops.PartSupply(context.Background(), "rf", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Coverage.Risk != string(domain.CoverageNone) || res.Coverage.Suppliers != 0 {
		t.Fatalf("无对口供应商应判缺口，got risk=%s suppliers=%d", res.Coverage.Risk, res.Coverage.Suppliers)
	}
	if res.Coverage.TopSupplier != "" {
		t.Fatalf("缺口状态不应推荐供应商，got %s", res.Coverage.TopSupplier)
	}

	// 单点：只有一家对口
	repo2 := repository.NewMemorySupplierRepository([]domain.Supplier{
		{ID: "s1", Name: "天线厂", ShortName: "天线", Capabilities: []string{"antenna", "fpc", "pcba"}},
	})
	ops2 := NewSupplierOpsService(repo2, srm.NewStore())
	res2, err := ops2.PartSupply(context.Background(), "rf", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if res2.Coverage.Risk != string(domain.CoverageSingle) {
		t.Fatalf("单一供应源应判单点，got %s", res2.Coverage.Risk)
	}
	if res2.Coverage.TopSupplier == "" {
		t.Fatal("单点状态也应给出当前唯一可选供应商")
	}
}

// 关键件单点（产品矩阵里）必须计入 critical_gaps：这是量产前最该先看的一项。
func TestProductSupplyMatrix_CountsCriticalSinglePoints(t *testing.T) {
	// 只留一家能做 pcba 的供应商：主控小类里的关键件就成了单点。
	repo := repository.NewMemorySupplierRepository([]domain.Supplier{
		{ID: "s1", Name: "唯一 PCBA 厂", ShortName: "唯一", City: "深圳市", Capabilities: []string{"pcba", "testing"}},
	})
	ops := NewSupplierOpsService(repo, srm.NewStore())
	m, err := ops.ProductSupplyMatrix(context.Background(), "smartwatch", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if m.Summary.Categories == 0 {
		t.Fatal("矩阵应统计到小类")
	}
	if m.Summary.SinglePoints == 0 {
		t.Fatalf("只有一家对口供应商时，应统计出单点小类，got %+v", m.Summary)
	}
	if m.Summary.Headline == "" {
		t.Fatal("应给出一句话结论")
	}
	// 每个大类都要带自己的最差风险等级，供界面按模块排序。
	for _, g := range m.Groups {
		if g.Risk == "" {
			t.Fatalf("大类 %s 缺少风险等级", g.Name)
		}
	}
}

// 供应商反查：能覆盖的小类与部件必须来自它真实的工艺能力。
func TestSupplierPartCoverage_DerivesFromCapabilities(t *testing.T) {
	ops := newMatchOps()
	res, err := ops.SupplierPartCoverage(context.Background(), "s-none")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Categories) == 0 {
		t.Fatal("具备 cnc/stamping 的供应商应至少覆盖结构件小类")
	}
	for _, c := range res.Categories {
		// 结构件小类的关键工艺必须被它覆盖，不能是无根据的关联。
		if c.Category.Code == "soc" {
			t.Fatal("做五金的不应被判为能供主控与存储")
		}
	}
	if res.Summary.Categories != len(res.Categories) {
		t.Fatalf("汇总与明细数量应一致：%d vs %d", res.Summary.Categories, len(res.Categories))
	}
	if _, err := ops.SupplierPartCoverage(context.Background(), "不存在"); err == nil {
		t.Fatal("不存在的供应商应报错")
	}
}

// 区域只在指定时才拉开差距；不指定区域不能影响排序公平性。
func TestPartSupply_RegionScoring(t *testing.T) {
	ops := newMatchOps()
	noRegion, _ := ops.PartSupply(context.Background(), "soc", "", "", "", "")
	shenzhen, _ := ops.PartSupply(context.Background(), "soc", "", "深圳市", "", "")
	if len(noRegion.Items) != len(shenzhen.Items) {
		t.Fatal("区域过滤不应增减候选数量（这里只加权不硬过滤）")
	}
	var szFit, dgFit float64
	for _, it := range shenzhen.Items {
		switch it.SupplierID {
		case "s-full":
			szFit = it.FitScore
			if it.RegionTag != "同城" {
				t.Fatalf("深圳的供应商在指定深圳时应标同城，got %s", it.RegionTag)
			}
		case "s-part":
			dgFit = it.FitScore
			if it.RegionTag != "同省" {
				t.Fatalf("东莞的供应商在指定深圳时应标同省，got %s", it.RegionTag)
			}
		}
	}
	if szFit <= dgFit {
		t.Fatalf("同城适配分应高于同省：%.1f vs %.1f", szFit, dgFit)
	}
}
