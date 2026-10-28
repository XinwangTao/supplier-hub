// Package scoring 实现美团式四维综合评分:QCDST 履约 40% + 需求匹配 30% +
// 工商实力 15% + 4M1E 巡厂档案 15%。
//
// 两条铁律承自项目数据哲学:
//  1. 每个分项都能展开看构成(维度 → 子项 → 口径),分数不是黑盒;
//  2. 缺数据就置 0 并打标记(证据不足/无数据),绝不填充看似合理的值——
//     但供应商不隐藏,由置信标记告诉使用者"这个分有几成可信"。
package scoring

import (
	"fmt"
	"math"
	"time"

	"github.com/example/supplier-hub/backend/internal/domain"
)

// Weights 四维权重。加和应为 1;调整后榜单与搜索徽章同时生效。
type Weights struct {
	QCDST float64
	Match float64
	Reg   float64
	Field float64
}

var Current = Weights{QCDST: 0.40, Match: 0.30, Reg: 0.15, Field: 0.15}

// 维度标记:显式告诉使用者"这个分有几成可信"。
const (
	FlagOK         = ""
	FlagInsufficient = "insufficient_evidence" // 有单据但样本不足 15 批,QCDST 不出正式评级
	FlagNoData     = "no_data"               // 该维度完全没数据
)

type Dimension struct {
	Key    string  `json:"key"`    // qcdst | match | registry | field
	Label  string  `json:"label"`  // 展示名
	Score  float64 `json:"score"`  // 0–100;无数据为 0
	Weight float64 `json:"weight"` // 0–1
	Flag   string  `json:"flag"`   // 置信标记
	Note   string  `json:"note"`   // 一句话口径说明
}

type Result struct {
	Total      float64     `json:"total"`      // 0–100
	Dimensions []Dimension `json:"dimensions"`
	Flags      []string    `json:"flags"` // 汇总的非空标记
}

// Input 一家供应商的四维输入。QCDSTTotal<0 表示证据不足;M1E 为 nil 表示未巡厂;
// MatchScore 传 -1 表示无需求上下文(榜单场景),由 capabilityCoverage 兜底。
type Input struct {
	QCDSTTotal float64
	Supplier   domain.Supplier
	MatchScore float64
	M1E        *domain.FourM1E
}

// Compute 合成综合分。纯函数:输入现有各服务的事实输出,不反过头改它们。
func Compute(in Input) Result {
	dims := []Dimension{
		qcdstDimension(in.QCDSTTotal),
		matchDimension(in.MatchScore, in.Supplier),
		registryDimension(in.Supplier),
		fieldDimension(in.M1E),
	}
	total := 0.0
	flags := []string{}
	for _, d := range dims {
		total += d.Score * d.Weight
		if d.Flag != "" {
			flags = append(flags, d.Flag)
		}
	}
	return Result{Total: math.Round(total*10) / 10, Dimensions: dims, Flags: flags}
}

// qcdstDimension 履约维度。total<0 = 合作了但样本不足,不给分但标"证据不足";
// 完全没有单据 = 无数据。两种情况都不隐藏供应商。
func qcdstDimension(total float64) Dimension {
	if total < 0 {
		return Dimension{Key: "qcdst", Label: "QCDST 履约", Score: 0, Weight: Current.QCDST,
			Flag: FlagInsufficient, Note: "有合作单据但样本不足,暂不出正式评级"}
	}
	if total == 0 {
		return Dimension{Key: "qcdst", Label: "QCDST 履约", Score: 0, Weight: Current.QCDST,
			Flag: FlagNoData, Note: "无合作单据,履约维度无数据"}
	}
	return Dimension{Key: "qcdst", Label: "QCDST 履约", Score: total, Weight: Current.QCDST,
		Note: "订单-交付-IQC-质量事件按样本加权"}
}

// matchDimension 匹配维度。score>=0 直接用(寻源场景的可解释匹配分);
// -1 = 榜单场景没有需求上下文,退化为能力覆盖度(挂的工艺能力越多越高)。
func matchDimension(score float64, s domain.Supplier) Dimension {
	if score >= 0 {
		return Dimension{Key: "match", Label: "需求匹配", Score: score, Weight: Current.Match,
			Note: "能力覆盖/同城协同/资本门槛的可解释匹配分"}
	}
	coverage := math.Min(float64(len(s.Capabilities))/8*100, 100)
	return Dimension{Key: "match", Label: "能力覆盖", Score: math.Round(coverage*10) / 10, Weight: Current.Match,
		Note: fmt.Sprintf("榜单场景:挂载工艺能力 %d 类(上限 8 类)", len(s.Capabilities))}
}

// registryDimension 工商实力:注册资本 40 + 参保人数 30 + 成立年限 20 + 进出口权 10。
// 分档而非线性——10 亿资本不该比 1 亿资本高 100 倍分,档位差异才有判别力。
func registryDimension(s domain.Supplier) Dimension {
	capital := tier(float64(s.RegisteredCapital), []float64{1000, 5000, 2e4, 1e5, 5e5})   // 万
	people := tier(float64(s.InsuredCount), []float64{20, 100, 300, 1000, 3000})          // 参保
	years := tier(yearsSince(s.EstablishDate), []float64{3, 8, 15, 25, 35})
	// 各子项先归一化到 0–1 再乘权重,保证维度分不超出百分制
	score := (capital/5)*40 + (people/5)*30 + (years/5)*20
	if s.ImportExport {
		score += 10
	}
	if score == 0 {
		return Dimension{Key: "registry", Label: "工商实力", Score: 0, Weight: Current.Reg,
			Flag: FlagNoData, Note: "工商字段缺失"}
	}
	return Dimension{Key: "registry", Label: "工商实力", Score: math.Round(score*10) / 10, Weight: Current.Reg,
		Note: fmt.Sprintf("注册资本档 %.0f/5 · 参保档 %.0f/5 · 年限档 %.0f/5%s", capital, people, years, importExportNote(s.ImportExport))}
}

// fieldDimension 4M1E 巡厂档案:字段填写率 60 + 关键事实 40。
// 填写率衡量"掌握多少现场事实",关键事实衡量"掌握的是不是硬事实"。
func fieldDimension(m *domain.FourM1E) Dimension {
	if m == nil {
		return Dimension{Key: "field", Label: "4M1E 巡厂档案", Score: 0, Weight: Current.Field,
			Flag: FlagNoData, Note: "未巡厂或未录入,现场维度无数据"}
	}
	filled, total := countFilled(m)
	ratio := float64(filled) / float64(total) * 60
	bonus := 0.0
	if len(m.Certifications) > 0 {
		bonus += 20
	}
	if len(m.Machines) > 0 {
		bonus += 10
	}
	if m.DualSourcing != "" {
		bonus += 10
	}
	score := math.Round((ratio+bonus)*10) / 10
	flag := ""
	note := fmt.Sprintf("已录入 %d/%d 项现场事实", filled, total)
	if filled == 0 {
		flag = FlagNoData
		note = "已建档但未填写任何字段"
	}
	return Dimension{Key: "field", Label: "4M1E 巡厂档案", Score: score, Weight: Current.Field, Flag: flag, Note: note}
}

// countFilled 统计文本与数值字段的填写数(0 视为未填,与档案页「—」口径一致)。
func countFilled(m *domain.FourM1E) (filled, total int) {
	texts := []string{m.KeyPersonnel, m.MainMaterials, m.DualSourcing, m.IQCMethod, m.MaterialCompliance,
		m.DesignCapability, m.InspectionCapability, m.ProcessDocs, m.ChangeManagement,
		m.EnvCertification, m.EnvPenalty, m.WasteTreatment, m.Location}
	nums := []int{m.EmployeeTotal, m.RnDCount, m.FactoryArea, m.ProductionLines, m.AutomationRate, m.CapacityUtilization}
	for _, t := range texts {
		total++
		if t != "" {
			filled++
		}
	}
	for _, n := range nums {
		total++
		if n > 0 {
			filled++
		}
	}
	total += 3 // departments / machines / certifications 三个结构化字段
	if len(m.Departments) > 0 {
		filled++
	}
	if len(m.Machines) > 0 {
		filled++
	}
	if len(m.Certifications) > 0 {
		filled++
	}
	return filled, total
}

// tier 分档:返回 0–5(落在第几档)。
func tier(v float64, steps []float64) float64 {
	t := 0.0
	for _, step := range steps {
		if v >= step {
			t++
		}
	}
	return t
}

func yearsSince(date string) float64 {
	if len(date) < 4 {
		return 0
	}
	var year int
	if _, err := fmt.Sscanf(date[:4], "%d", &year); err != nil || year <= 0 {
		return 0
	}
	years := float64(time.Now().Year()) - float64(year)
	if years < 0 {
		return 0
	}
	return years
}

func importExportNote(yes bool) string {
	if yes {
		return " · 有进出口权"
	}
	return ""
}
