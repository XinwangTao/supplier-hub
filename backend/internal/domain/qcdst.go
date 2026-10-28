package domain

// QCDST 供应商能力模型：Quality / Cost / Delivery / Service / Technology。
//
// 设计原则有两条，是这套东西能不能用的关键：
//  1. 每一项分数都必须能追到原始单据（订单、批次、质量事件、报价、工单），
//     算不出来就返回"数据不足"，绝不给一个看起来合理的默认分。
//  2. 样本量决定置信度。只有 1 张订单的 100 分，不能和 300 批交付的 92 分
//     平起平坐，评级时要把这件事说清楚。

// Dimension 五维标识。
type Dimension string

const (
	DimQuality    Dimension = "Q"
	DimCost       Dimension = "C"
	DimDelivery   Dimension = "D"
	DimService    Dimension = "S"
	DimTechnology Dimension = "T"
)

// Grade 评级档位。NA 表示证据不足，不予评级——这是对外披露时的诚实做法。
type Grade string

const (
	GradeAPlus Grade = "A+"
	GradeA     Grade = "A"
	GradeB     Grade = "B"
	GradeC     Grade = "C"
	GradeD     Grade = "D"
	GradeNA    Grade = "NA"
)

// MetricScore 单个指标的得分与证据。
type MetricScore struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Unit        string   `json:"unit"`
	Weight      float64  `json:"weight"` // 维度内权重，已归一
	Score       float64  `json:"score"`  // 0-100；-1 表示数据不足
	Raw         string   `json:"raw"`    // 展示用原始值，如 "97.2%"、"1,240 PPM"
	Numerator   float64  `json:"numerator"`
	Denominator float64  `json:"denominator"`
	Sample      int      `json:"sample"`     // 参与计算的单据数
	MinSample   int      `json:"min_sample"` // 达到多少条才算数
	Confidence  float64  `json:"confidence"`
	Formula     string   `json:"formula"`
	Benchmark   string   `json:"benchmark"`
	Evidence    []string `json:"evidence"` // 最近几条可回查的单据号
	Auto        bool     `json:"auto"`     // true=SRM 自动采集，false=人工评审
}

// DimensionScore 一维的汇总。
type DimensionScore struct {
	Dimension  Dimension     `json:"dimension"`
	Name       string        `json:"name"`
	Weight     float64       `json:"weight"` // 总分权重
	Score      float64       `json:"score"`  // -1 表示该维无有效数据
	Confidence float64       `json:"confidence"`
	Coverage   float64       `json:"coverage"` // 有效指标权重占比
	Summary    string        `json:"summary"`
	Metrics    []MetricScore `json:"metrics"`
}

// ScorePoint 评分历史，用于看趋势（这家是在变好还是变差）。
type ScorePoint struct {
	Period  string  `json:"period"` // 2026-07 这样的月份
	Total   float64 `json:"total"`
	Samples int     `json:"samples"`
}

// QCDSTAssessment 一次完整评估。
type QCDSTAssessment struct {
	SupplierID   string `json:"supplier_id"`
	SupplierName string `json:"supplier_name"`
	ShortName    string `json:"short_name"`
	CreditCode   string `json:"credit_code"`
	TenantID     string `json:"tenant_id"` // 由哪家采购方评估
	TenantName   string `json:"tenant_name"`

	Total      float64          `json:"total"`
	Grade      Grade            `json:"grade"`
	Confidence float64          `json:"confidence"`
	Coverage   float64          `json:"coverage"`
	Dimensions []DimensionScore `json:"dimensions"`

	Window        string `json:"window"`
	OrderCount    int    `json:"order_count"`
	BatchCount    int    `json:"batch_count"`
	QualityEvents int    `json:"quality_events"`

	Demo bool `json:"demo"` // 演示数据：不是真实业务单据
	// Provisional 观察期：样本量不足以支撑正式评级，分数仅供参考。
	// 新供应商拿高分是最常见的误判来源，必须在字段层面就标出来。
	Provisional bool         `json:"provisional"`
	GeneratedAt string       `json:"generated_at"`
	Insights    []Insight    `json:"insights"`
	History     []ScorePoint `json:"history"`

	// ---- 防操纵护栏：与平台评级共用同一套字段 ----
	DataSpanDays int         `json:"data_span_days"` // 首末交付相隔天数
	RecentTotal  float64     `json:"recent_total"`   // 近 180 天总分；-1 表示窗口内无交付
	Trend        string      `json:"trend"`          // up / flat / down / unknown
	Stale        bool        `json:"stale"`          // 超过 365 天无新交付
	Integrity    DataQuality `json:"integrity"`
	Guardrails   []string    `json:"guardrails"`
}

// PlatformRating 平台级综合评级：把多家采购方对同一家供应商的评估合起来。
// 新用户看到的"别人家用下来怎么样"，就是这张表。
type PlatformRating struct {
	SupplierID   string   `json:"supplier_id"`
	SupplierName string   `json:"supplier_name"`
	ShortName    string   `json:"short_name"`
	CreditCode   string   `json:"credit_code"`
	City         string   `json:"city"`
	Capabilities []string `json:"capabilities"`

	Total      float64 `json:"total"` // 样本加权均分
	Grade      Grade   `json:"grade"`
	Confidence float64 `json:"confidence"`

	BuyerCount int `json:"buyer_count"` // 多少家采购方有合作关系
	// RatedBuyers 有多少家采购方的样本量够出正式评级。
	// 合作关系数和"评过"是两件事，分母不够时不能假装有分歧度。
	RatedBuyers   int `json:"rated_buyers"`
	OrderCount    int `json:"order_count"` // 累计订单
	BatchCount    int `json:"batch_count"` // 累计交付批次
	QualityEvents int `json:"quality_events"`

	// Spread 采购方之间的分歧度（各买方总分的标准差）。
	// 分歧大意味着"不同客户体验差别很大"，比均分本身更值得看。
	Spread float64 `json:"spread"`

	Dimensions  []DimensionBrief `json:"dimensions"`
	Verified    bool             `json:"verified"` // 是否基于真实业务单据
	Demo        bool             `json:"demo"`
	Provisional bool             `json:"provisional"`
	LastSeenAt  string           `json:"last_seen_at"`

	// ---- 以下四项是防"评级被操纵"的护栏，缺一项评级就不可信 ----

	// DataSpanDays 首末交付相隔天数。15 个批次如果全发生在同一周，
	// 那是"一周的表现"，不是"一家供应商的水平"。
	DataSpanDays int `json:"data_span_days"`
	// RecentTotal 近 180 天按时间衰减加权后的总分。采购决策关心的是现在，
	// 不是三年来的平均分。与 Total 差距大 = 这家在变好或变差。
	RecentTotal float64 `json:"recent_total"`
	// Trend 近半年相对历史的变化方向：up / flat / down。
	Trend string `json:"trend"`
	// Stale 最后一批交付距今超过 365 天。超过一年没有新单据，
	// 再高的分也只是历史档案，不能当作当前能力。
	Stale bool `json:"stale"`
	// DominantShare 单一采购方在评级权重里的占比。
	// "N 家企业验证"这句话的可信度全看这个数：0.9 意味着基本是一家说了算。
	DominantShare float64 `json:"dominant_share"`
	// Integrity 原始单据的完整性体检。有重大异常时评级必须打问号。
	Integrity DataQuality `json:"integrity"`

	// Guardrails 触发了哪些护栏，逐条写明，界面照原样显示。
	Guardrails []string `json:"guardrails"`
}

// DataQuality 原始单据的完整性体检结果。
//
// 评分引擎算得再细，喂进去的是脏数据也白搭。这里不修数据，只把问题摆出来：
// 哪些批次的数字自相矛盾、有多少小样批在垫高合格率、数据跨度够不够。
type DataQuality struct {
	// Level: ok / warn / bad。bad 意味着分数已经不可信，界面必须打红标。
	Level   string      `json:"level"`
	Score   float64     `json:"score"` // 0-100，完整度打分
	Issues  []DataIssue `json:"issues"`
	Summary string      `json:"summary"`
	// SmallBatchRate 小样批（数量不足中位批 20%）占比。
	// 这个数高说明合格率是被大量小批量单撑起来的，量产时未必复现。
	SmallBatchRate float64 `json:"small_batch_rate"`
	// 关键统计，供复核
	BatchCount   int `json:"batch_count"`
	TotalQty     int `json:"total_qty"`
	ZeroQtyBatch int `json:"zero_qty_batch"`
	BadBalance   int `json:"bad_balance"` // accepted+rejected ≠ qty 的批次数
	BadOnTime    int `json:"bad_on_time"` // on_time 与日期矛盾的批次数
	FutureDated  int `json:"future_dated"`
	MissingDate  int `json:"missing_date"`
}

// DataIssue 一条具体的数据问题。
type DataIssue struct {
	Code     string   `json:"code"` // small_batch / bad_balance / bad_on_time / future_dated / ...
	Level    string   `json:"level"`
	Title    string   `json:"title"`
	Detail   string   `json:"detail"`
	Affected int      `json:"affected"` // 涉及多少条单据
	Evidence []string `json:"evidence"`
}

// AuditEntry 一条审计留痕。
//
// SRM 里的每一次写入都必须留下"谁、什么时候、改了什么"。
// 没有这个，IQC 结果被改了没人知道，评级就成了可以事后粉饰的东西。
type AuditEntry struct {
	ID        string `json:"id"`
	At        string `json:"at"`
	TenantID  string `json:"tenant_id"`
	Actor     string `json:"actor"`    // 操作人（真实系统里是登录账号）
	Action    string `json:"action"`   // order.create / delivery.iqc / engagement.status ...
	Object    string `json:"object"`   // 单据号或供应商 ID
	Summary   string `json:"summary"`  // 人能读懂的一句话
	Source    string `json:"source"`   // api / import / demo
	Rejected  bool   `json:"rejected"` // 被校验拦下的也算一条，用于发现误用
	RejectWhy string `json:"reject_why,omitempty"`
}

type DimensionBrief struct {
	Dimension Dimension `json:"dimension"`
	Name      string    `json:"name"`
	Score     float64   `json:"score"`
}

// ---------------- SRM ----------------

// Tenant 采购方企业。平台是多租户的：每家企业的 SRM 数据彼此隔离，
// 只有经过聚合脱敏的评级才会进入公共池。
type Tenant struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Industry string `json:"industry"`
	Plan     string `json:"plan"` // free / team / enterprise
	Since    string `json:"since"`
}

// Engagement 采购方与供应商的合作关系。没有这层关系，供应商看不到订单，
// 采购方也拿不到交付数据——SRM 的边界就是它。
//
// Status 见 EngagementStatus 常量：candidate / invited / active / paused / terminated。
type Engagement struct {
	ID              string   `json:"id"`
	TenantID        string   `json:"tenant_id"`
	SupplierID      string   `json:"supplier_id"`
	Status          string   `json:"status"`
	Category        string   `json:"category"`
	Since           string   `json:"since"`
	AnnualSpend     float64  `json:"annual_spend"`   // 万元
	ShareOfSpend    float64  `json:"share_of_spend"` // 占该品类采购额比例 %
	SourcingTypes   []string `json:"sourcing_types"`
	BuyerName       string   `json:"buyer_name"`
	SupplierContact string   `json:"supplier_contact"`
	// Source 这条关系是怎么进来的：sourcing=从公开寻源页引入，manual=手工建档。
	// 记下来是为了复盘"引入渠道质量"——从寻源引入的供应商，实际表现是否更好。
	Source string `json:"source"`
	Note   string `json:"note"`
}

// PurchaseOrder 采购订单。交付类指标的分母基本来自这里。
type PurchaseOrder struct {
	ID         string  `json:"id"`
	TenantID   string  `json:"tenant_id"`
	SupplierID string  `json:"supplier_id"`
	PONo       string  `json:"po_no"`
	Category   string  `json:"category"`
	PartName   string  `json:"part_name"`
	Qty        int     `json:"qty"`
	UnitPrice  float64 `json:"unit_price"`
	Amount     float64 `json:"amount"` // 元
	IssuedAt   string  `json:"issued_at"`
	PromisedAt string  `json:"promised_at"`
	ClosedAt   string  `json:"closed_at"`
	Status     string  `json:"status"`    // open / delivered / closed / cancelled
	Expedited  bool    `json:"expedited"` // 是否紧急插单
}

// Delivery 交付批次。IQC 结果在这里落地，是质量维度的原始事实。
type Delivery struct {
	ID               string   `json:"id"`
	TenantID         string   `json:"tenant_id"`
	SupplierID       string   `json:"supplier_id"`
	POID             string   `json:"po_id"`
	PONo             string   `json:"po_no"`
	BatchNo          string   `json:"batch_no"`
	PartName         string   `json:"part_name"`
	Qty              int      `json:"qty"`
	Accepted         int      `json:"accepted"`
	Rejected         int      `json:"rejected"`
	ArrivedAt        string   `json:"arrived_at"`
	PromisedAt       string   `json:"promised_at"`
	OnTime           bool     `json:"on_time"`
	LeadDays         int      `json:"lead_days"`
	PromisedLeadDays int      `json:"promised_lead_days"`
	IQCResult        string   `json:"iqc_result"` // pass / fail / conditional
	DefectCodes      []string `json:"defect_codes"`
	FirstArticle     bool     `json:"first_article"`
	FAIPass          bool     `json:"fai_pass"`
}

// QualityEvent 质量事件。带 8D 与关闭时间，用来判"问题有没有真解决"。
type QualityEvent struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenant_id"`
	SupplierID   string `json:"supplier_id"`
	Type         string `json:"type"`     // 来料不良 / 制程异常 / 客诉 / 停线
	Severity     string `json:"severity"` // minor / major / critical
	OccurredAt   string `json:"occurred_at"`
	DueAt        string `json:"due_at"`
	ClosedAt     string `json:"closed_at"`
	EightD       bool   `json:"eightd"`
	RootCause    string `json:"root_cause"`
	DeductPoints int    `json:"deduct_points"` // 停线/召回这类按次扣分
}

// Quote 报价记录。成本维度靠它算价格竞争力与年降。
type Quote struct {
	ID             string  `json:"id"`
	TenantID       string  `json:"tenant_id"`
	SupplierID     string  `json:"supplier_id"`
	PartName       string  `json:"part_name"`
	UnitPrice      float64 `json:"unit_price"`
	PreviousPrice  float64 `json:"previous_price"`  // 上一轮价格，用于算年降
	BenchmarkPrice float64 `json:"benchmark_price"` // 同品类中位价，用于算竞争力
	Currency       string  `json:"currency"`
	PaymentDays    int     `json:"payment_days"`
	CostBreakdown  bool    `json:"cost_breakdown"` // 是否愿意拆成本（工艺/材料/人工）
	QuotedAt       string  `json:"quoted_at"`
	ValidUntil     string  `json:"valid_until"`
}

// ServiceTicket 服务工单：问题响应与解决速度。
type ServiceTicket struct {
	ID              string  `json:"id"`
	TenantID        string  `json:"tenant_id"`
	SupplierID      string  `json:"supplier_id"`
	Type            string  `json:"type"` // 技术咨询 / 质量整改 / 交期催办 / 售后
	OpenedAt        string  `json:"opened_at"`
	FirstResponseAt string  `json:"first_response_at"`
	ResolvedAt      string  `json:"resolved_at"`
	Status          string  `json:"status"` // open / resolved / overdue
	ResponseHours   float64 `json:"response_hours"`
	ResolveDays     float64 `json:"resolve_days"`
}

// TechReview 技术与协同评审，人工录入的那部分（也是唯一带主观判断的部分，
// 所以必须记评审人，方便追溯）。
type TechReview struct {
	ID                 string  `json:"id"`
	TenantID           string  `json:"tenant_id"`
	SupplierID         string  `json:"supplier_id"`
	ReviewedAt         string  `json:"reviewed_at"`
	Reviewer           string  `json:"reviewer"`
	CapabilityCoverage float64 `json:"capability_coverage"` // 关键工序覆盖 %
	AutomationLevel    int     `json:"automation_level"`    // 1-5
	RndRatio           float64 `json:"rnd_ratio"`           // 研发人员占比 %
	NPIProjects        int     `json:"npi_projects"`
	NPISuccess         int     `json:"npi_success"`
	CooperationScore   int     `json:"cooperation_score"` // 0-100 协同配合
	Notes              string  `json:"notes"`
}

// ---------------- 闭环：寻源 → SRM → 反哺 ----------------

// EngagementStatus 合作关系的状态机。
//
// 状态机存在的理由：寻源页上"看中了"和"真的开始合作"是两件事。
// candidate 只是把供应商放进候选池，不产生任何业务单据，因此也不进入评分；
// 只有 active 之后录了单，才开始积累 QCDST 证据。把这两件事混在一起，
// 会让"我关注了 50 家"变成"我有 50 家供应商数据"，评级池立刻被稀释。
type EngagementStatus string

const (
	EngagementCandidate  EngagementStatus = "candidate"
	EngagementInvited    EngagementStatus = "invited"
	EngagementActive     EngagementStatus = "active"
	EngagementPaused     EngagementStatus = "paused"
	EngagementTerminated EngagementStatus = "terminated"
)

// EngagementStatusLabel 状态中文名（界面与提示复用，避免各处翻译不一致）。
var EngagementStatusLabel = map[EngagementStatus]string{
	EngagementCandidate:  "候选",
	EngagementInvited:    "已邀请",
	EngagementActive:     "合作中",
	EngagementPaused:     "暂缓",
	EngagementTerminated: "已终止",
}

// IsValidEngagementStatus 校验状态值，非法状态直接拒绝写入。
func IsValidEngagementStatus(s string) bool {
	_, ok := EngagementStatusLabel[EngagementStatus(s)]
	return ok
}

// OrderDimension 订单级评分卡里的一维。
// 订单级与供应商级最大的差别是：一张订单往往只涉及两三个维度
// （比如一笔还没有交付的订单，压根判不了质量）。所以每一维都要带
// judged 标记和"为什么这么判"的说明，不能默默按 0 分或满分处理。
type OrderDimension struct {
	Dimension       Dimension `json:"dimension"`
	Name            string    `json:"name"`
	Weight          float64   `json:"weight"`           // 该维在总分里的原始权重
	EffectiveWeight float64   `json:"effective_weight"` // 可判维度归一后的权重
	Score           float64   `json:"score"`            // -1 = 这笔订单判不了
	Judged          bool      `json:"judged"`
	Raw             string    `json:"raw"`    // 展示用原始值
	Reason          string    `json:"reason"` // 判定依据，或"判不了"的原因
}

// OrderScorecard 一张订单的 QCDST 评分卡。
//
// 用途与供应商级评估不同，两者不能互相替代：
//   - 订单级用于**对账与追责**：这张单到底哪里没做好，扣多少分，翻得出单据号；
//   - 供应商级用于**评级与选择**：看的是一段时期的稳定表现。
//
// 所以订单级总分只按"这笔订单判得出来的维度"重新归一加权，
// 汇总到供应商时也不会拿订单总分去平均——那会把样本量信息抹平。
type OrderScorecard struct {
	OrderID   string  `json:"order_id"`
	PONo      string  `json:"po_no"`
	TenantID  string  `json:"tenant_id"`
	PartName  string  `json:"part_name"`
	Category  string  `json:"category"`
	Qty       int     `json:"qty"`
	Amount    float64 `json:"amount"`
	IssuedAt  string  `json:"issued_at"`
	ClosedAt  string  `json:"closed_at"`
	Status    string  `json:"status"`
	Expedited bool    `json:"expedited"`

	Total        float64          `json:"total"` // -1 = 无可判维度
	Grade        Grade            `json:"grade"`
	JudgedWeight float64          `json:"judged_weight"` // 可判维度权重合计
	Dimensions   []OrderDimension `json:"dimensions"`

	BatchCount    int      `json:"batch_count"`
	OnTimeBatches int      `json:"on_time_batches"`
	Accepted      int      `json:"accepted"`
	Rejected      int      `json:"rejected"`
	IQCResults    []string `json:"iqc_results"`
	Note          string   `json:"note"`
	Demo          bool     `json:"demo"`
}

// SupplierBrief 挂在寻源结果上的"这一家在平台上的处境"。
//
// 寻源页要用它回答两个问题：
//  1. 别人家用下来怎么样（Rated/Grade/Total/BuyerCount）——这是平台反哺的部分；
//  2. 我自己有没有引入过（InSRM/Status）——这是私有状态，只对当前企业可见。
type SupplierBrief struct {
	SupplierID string `json:"supplier_id"`

	// 平台反哺：有没有企业在 SRM 里真实合作过并留下足够证据。
	Rated         bool    `json:"rated"`
	Grade         Grade   `json:"grade"`
	Total         float64 `json:"total"`
	BuyerCount    int     `json:"buyer_count"`
	RatedBuyers   int     `json:"rated_buyers"`
	BatchCount    int     `json:"batch_count"`
	OrderCount    int     `json:"order_count"`
	Spread        float64 `json:"spread"`
	Provisional   bool    `json:"provisional"`
	Verified      bool    `json:"verified"`
	Demo          bool    `json:"demo"`
	RatingSummary string  `json:"rating_summary"`

	// 本企业私有状态
	InSRM        bool   `json:"in_srm"`
	Status       string `json:"status"`
	StatusLabel  string `json:"status_label"`
	Since        string `json:"since"`
	MyOrderCount int    `json:"my_order_count"`
	MyBatchCount int    `json:"my_batch_count"`
}

// LoopReport 闭环体检：平台池现在有多大、反哺有没有真的发生、能信到什么程度。
// 新用户最该先看到的不是排名，而是"这个池子现在能不能信"。
type LoopReport struct {
	SuppliersInPool int     `json:"suppliers_in_pool"` // 进入过 SRM 的供应商数
	RatedSuppliers  int     `json:"rated_suppliers"`   // 已出正式评级的
	Provisional     int     `json:"provisional"`       // 观察期（样本不足）
	Verified        int     `json:"verified"`          // 基于真实单据的
	Demo            int     `json:"demo"`              // 演示数据
	Tenants         int     `json:"tenants"`           // 贡献数据的采购方数
	TotalBatches    int     `json:"total_batches"`     // 累计交付批次
	AverageSpread   float64 `json:"average_spread"`    // 平均买方分歧度

	MySuppliers  int `json:"my_suppliers"` // 当前企业已引入
	MyCandidates int `json:"my_candidates"`
	MyBatches    int `json:"my_batches"`

	Stage string `json:"stage"`
}

// BriefIndex 寻源页一次拉走的索引：supplier_id → brief。
type BriefIndex struct {
	TenantID   string                   `json:"tenant_id"`
	TenantName string                   `json:"tenant_name"`
	Items      map[string]SupplierBrief `json:"items"`
	Meta       BriefMeta                `json:"meta"`
}

type BriefMeta struct {
	Rated      int    `json:"rated"`      // 平台上有评级的供应商数
	InSRM      int    `json:"in_srm"`     // 当前企业已引入的供应商数
	Candidates int    `json:"candidates"` // 当前企业候选池数量
	Buyers     int    `json:"buyers"`     // 平台上有多少家采购方贡献了数据
	Demo       bool   `json:"demo"`
	Note       string `json:"note"`
}

// SRMWorkspace 一家采购方的工作台快照。
type SRMWorkspace struct {
	Tenant      Tenant            `json:"tenant"`
	Engagements []Engagement      `json:"engagements"`
	Assessments []QCDSTAssessment `json:"assessments"`
	Stats       SRMStats          `json:"stats"`
	Demo        bool              `json:"demo"`
	Insights    []Insight         `json:"insights"`
}

type SRMStats struct {
	Suppliers     int     `json:"suppliers"`
	Active        int     `json:"active"`
	Orders        int     `json:"orders"`
	OpenOrders    int     `json:"open_orders"`
	Deliveries    int     `json:"deliveries"`
	OnTimeRate    float64 `json:"on_time_rate"`
	PassRate      float64 `json:"pass_rate"`
	Spend         float64 `json:"spend"`
	QualityEvents int     `json:"quality_events"`
}
