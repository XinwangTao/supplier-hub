package domain

// 产品 BOM 知识库：产品 → 大类（模块/总成）→ 小类（部件品类）→ 具体部件。
//
// 层级对应三个不同的决策动作：
//   大类   → 判断"这个产品值不值得做"（哪几块占成本、哪几块卡脖子）
//   小类   → 判断"这一类料谁能供"（跨产品通用的寻源口径）
//   部件   → 判断"具体要什么工艺"（落到工艺能力，再落到供应商）
// 所以小类必须是跨产品复用的字典，不能每个产品自己造词。

// PartCategory 部件小类字典。
type PartCategory struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// KeyCapabilities 该小类的主要工艺能力，用于从部件直接找到可选供应商。
	KeyCapabilities []string `json:"key_capabilities"`
}

// BomPart 具体部件。
type BomPart struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	Category     string `json:"category"` // PartCategory.Code
	CategoryName string `json:"category_name"`
	Qty          string `json:"qty"`        // 单机用量，如 "1"、"220"
	Critical     bool   `json:"critical"`   // 关键件：缺供会直接停线
	Capability   string `json:"capability"` // 主工艺能力代码
	Note         string `json:"note"`
}

// BomGroup 大类。一个模块或总成。
type BomGroup struct {
	Code  string    `json:"code"`
	Name  string    `json:"name"`
	Role  string    `json:"role"` // 这一块在整机里干什么
	Parts []BomPart `json:"parts"`
	// Critical 整个模块是关键路径（关键件多、替代源少）。
	Critical bool `json:"critical"`
}

// BomProduct 一个可拆解的产品。
type BomProduct struct {
	Code    string     `json:"code"`
	Name    string     `json:"name"`
	Sector  string     `json:"sector"` // 消费电子 / 家电 / 出行 / 工业装备 / 智能家居
	Aliases []string   `json:"aliases"`
	Summary string     `json:"summary"`
	Groups  []BomGroup `json:"groups"`

	// Parts 部件总数、TopParts 公开拆解资料里统计到的零件总数（仅作规模参考）。
	PartCount int `json:"part_count"`
	TopParts  int `json:"top_parts"`

	// Capabilities 该产品涉及的全部工艺能力（去重）。
	Capabilities []string `json:"capabilities"`
}

// BomFacet 小类的跨产品聚合视角：这个品类被哪些产品用到。
type BomFacet struct {
	Category PartCategory `json:"category"`
	Parts    int          `json:"parts"`
	Products []string     `json:"products"`
	Critical int          `json:"critical"`
}

// ---------------- 部件 → 供应源 ----------------
//
// 这一层把 BOM 知识库和公开寻源池接起来：回答"这一类料谁能供、供得怎么样"。
// 关键口径：适配分只衡量"能不能做、离得近不近"，不掺入评级分。
// 评级是历史表现，适配是能力覆盖，混在一起会让"分数高但工艺不对口"的
// 供应商排到前面——那正是选错供应商最常见的原因。

// CoverageRisk 一个小类的供应覆盖风险等级。
type CoverageRisk string

const (
	CoverageNone     CoverageRisk = "none"     // 0 家：公开池里找不到对口供应商
	CoverageSingle   CoverageRisk = "single"   // 1 家：单点供应，断供即停线
	CoverageNarrow   CoverageRisk = "narrow"   // 2-3 家：可选但缺乏议价与切换空间
	CoverageAdequate CoverageRisk = "adequate" // 4 家及以上
)

// PartSupplierMatch 某家供应商对某一小类的适配情况。
type PartSupplierMatch struct {
	SupplierID   string `json:"supplier_id"`
	Name         string `json:"name"`
	ShortName    string `json:"short_name"`
	City         string `json:"city"`
	Province     string `json:"province"`
	IsMaker      bool   `json:"is_manufacturer"`
	LeadTimeDays int    `json:"lead_time_days"`
	MOQ          int    `json:"moq"`
	// FitScore 适配分 0-100，只由能力覆盖（70%）与区域（30%）构成。
	FitScore float64 `json:"fit_score"`
	// FitReasons 为什么判定它对口，必须有据可查。
	FitReasons []string `json:"fit_reasons"`
	// HitCapabilities 命中的工艺；MissCapabilities 该小类要求但未覆盖的工艺。
	HitCapabilities  []string `json:"hit_capabilities"`
	MissCapabilities []string `json:"miss_capabilities"`
	CapabilityRatio  float64  `json:"capability_ratio"` // 命中 / 要求
	RegionTag        string   `json:"region_tag"`       // 同城 / 同省 / 跨区 / 未指定
	MainProducts     []string `json:"main_products"`
	// Rating 平台综合评级；没有任何企业合作过时为 nil，界面显示"暂无"。
	Rating *PlatformRating `json:"rating"`
	// Engagement 本企业与该供应商的合作关系；未引入时为 nil。
	Engagement *Engagement `json:"engagement"`
}

// PartCoverage 一个小类的供应覆盖体检。
type PartCoverage struct {
	Category PartCategory `json:"category"`
	// Parts 该小类在指定产品里有多少个部件（不限产品时为全库统计）。
	Parts       int     `json:"parts"`
	Critical    bool    `json:"critical"`     // 是否含关键件
	Suppliers   int     `json:"suppliers"`    // 对口的供应商数
	Rated       int     `json:"rated"`        // 其中有平台评级（非观察期）的
	Engaged     int     `json:"engaged"`      // 本企业已引入的
	BestScore   float64 `json:"best_score"`   // 最高平台综合分，-1 表示无人有分
	BestFit     float64 `json:"best_fit"`     // 最高适配分
	Risk        string  `json:"risk"`         // CoverageRisk
	RiskLabel   string  `json:"risk_label"`   // 缺口 / 单点 / 偏窄 / 充足
	Advice      string  `json:"advice"`       // 该做什么
	TopSupplier string  `json:"top_supplier"` // 当前最优（先评级后适配）
}

// PartSupply 一个小类的供应源清单 + 覆盖体检。
type PartSupply struct {
	Coverage PartCoverage        `json:"coverage"`
	Items    []PartSupplierMatch `json:"items"`
	// DemandParts 该小类下的具体部件（限定了产品时才有）。
	DemandParts []BomPart `json:"demand_parts"`
	// Capability 可选的主工艺过滤条件。
	Capability string   `json:"capability"`
	Region     string   `json:"region"`
	Products   []string `json:"products"`
	Note       string   `json:"note"`
}

// GroupSupply 一个大类（模块）的供应覆盖概览。
type GroupSupply struct {
	Code      string         `json:"code"`
	Name      string         `json:"name"`
	Role      string         `json:"role"`
	Critical  bool           `json:"critical"`
	Coverages []PartCoverage `json:"coverages"`
	Risk      string         `json:"risk"` // 该模块最差的覆盖等级
}

// ProductSupplyMatrix 一款产品的供应覆盖矩阵：做整机之前先看哪几块会卡脖子。
type ProductSupplyMatrix struct {
	Product  BomProduct    `json:"product"`
	Groups   []GroupSupply `json:"groups"`
	Summary  SupplySummary `json:"summary"`
	Region   string        `json:"region"`
	TenantID string        `json:"tenant_id"`
	Demo     bool          `json:"demo"`
}

// SupplySummary 产品级供应体检汇总。
type SupplySummary struct {
	Categories   int `json:"categories"`    // 涉及小类数
	Gaps         int `json:"gaps"`          // 无供应源的小类数
	SinglePoints int `json:"single_points"` // 单点小类数
	// CriticalGaps 卡脖子项：关键件所在小类里，缺供或单点的数量。
	// 这是做产品前最该先看的一项——它决定要不要提前备第二源。
	CriticalGaps     int     `json:"critical_gaps"`
	CoverageRate     float64 `json:"coverage_rate"` // 有 ≥2 家可选的小类占比 %
	SecondSourceRate float64 `json:"second_source_rate"`
	Headline         string  `json:"headline"` // 一句话结论
}

// SupplierPartCoverage 供应商反查：这家能做哪些小类。
type SupplierPartCoverage struct {
	Supplier   Supplier             `json:"supplier"`
	Categories []PartCoverage       `json:"categories"`
	Parts      []SupplierPartHit    `json:"parts"`
	Summary    SupplierCoverSummary `json:"summary"`
}

// SupplierPartHit 供应商能做的具体部件及其出处。
type SupplierPartHit struct {
	Part        BomPart `json:"part"`
	ProductCode string  `json:"product_code"`
	ProductName string  `json:"product_name"`
	GroupName   string  `json:"group_name"`
	// Direct 是否直接命中部件的主工艺。
	// false 表示只命中了小类的关键工艺（例如 PCBA 厂"能覆盖"电机类，
	// 实际只是驱动器带 PCBA），这类必须标出来，否则会夸大供应商的能力边界。
	Direct bool   `json:"direct"`
	Basis  string `json:"basis"` // 命中依据，如"主工艺：PCBA"
}

// SupplierCoverSummary 供应商侧覆盖汇总。
type SupplierCoverSummary struct {
	Categories int `json:"categories"`
	Parts      int `json:"parts"`
	// DirectParts 主工艺直接命中的部件数；IndirectParts 只是沾上小类关键工艺。
	// 分开是因为后者多半要外协，不能算进"这家自己能做什么"。
	DirectParts   int     `json:"direct_parts"`
	IndirectParts int     `json:"indirect_parts"`
	Critical      int     `json:"critical"`
	AvgRivalry    float64 `json:"avg_rivalry"` // 这些小类平均有多少家同行可替代
	Headline      string  `json:"headline"`
}
