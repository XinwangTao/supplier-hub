// Package policy 定义平台的数据开放策略，并以机器可读的方式对外声明。
//
// 为什么要有这个包：数据壁垒如果只写在文档里，就只是口号。
// 它必须变成一组"代码里能被引用的常量 + 一个对外可查的策略接口"，
// 否则下一次做功能时，第一个被牺牲的就是字段边界。
//
// 设计原则见 docs/positioning.md：越接近"能直接联系并成交"的字段，越往后放。
package policy

// Tier 访问层级。层级不是按"用户类型"分，而是按"关系深度"分——
// 关系越深，能看到的字段越接近成交所需。
type Tier string

const (
	// TierPublic 公开层：任何人都能看，也是搜索引擎能索引的部分。
	TierPublic Tier = "public"
	// TierVerified 认证层：完成企业认证的采购方。认证是数据对等交换的前提。
	TierVerified Tier = "verified"
	// TierPartner 合作层：已与该供应商建立 SRM 合作关系。
	TierPartner Tier = "partner"
	// TierOwner 自有层：数据产生方本人，看自己的数据不受限。
	TierOwner Tier = "owner"
)

// TierDef 一个层级的说明。
type TierDef struct {
	Tier      Tier   `json:"tier"`
	Label     string `json:"label"`
	Who       string `json:"who"`
	HowToGet  string `json:"how_to_get"`
	CanSee    string `json:"can_see"`
	DataRight string `json:"data_right"`
}

// FieldGroup 一组字段的开放规则。颗粒度按"业务语义"分，不按表结构分：
// 采购关心的是"联系方式能不能拿到"，不是"contact_phone 这个列在第几行"。
type FieldGroup struct {
	Key    string   `json:"key"`
	Label  string   `json:"label"`
	Fields []string `json:"fields"`
	Tier   Tier     `json:"tier"`
	// Reason 必须写给用户看。用户能接受"有理由的限制"，不能接受"莫名其妙的不给"。
	Reason string `json:"reason"`
}

// Quotas 接口层的硬约束。这些值由服务端强制，不信任前端传参。
type Quotas struct {
	MaxPageSize     int `json:"max_page_size"`      // 单页条数上限
	DeepPageFreeMax int `json:"deep_page_free_max"` // 免认证最多可翻到的记录数
	RequestsPerMin  int `json:"requests_per_min"`   // 单 IP 每分钟请求上限
	DetailFreeViews int `json:"detail_free_views"`  // 每家供应商可免费查看完整档案的次数
}

// ExportRule 导出规则。这里只声明"能导什么、不能导什么"，不实现导出端点。
type ExportRule struct {
	// Allowed 允许导出的数据：只有企业自己录入的。
	Allowed []string `json:"allowed"`
	// Denied 明确不提供的导出。
	Denied []string `json:"denied"`
	// Why 说明为什么不做导出。
	Why string `json:"why"`
	// Alternative 不做导出时，用什么替代方案满足真实需求。
	Alternative string `json:"alternative"`
}

// Doc 对外声明的完整策略。
type Doc struct {
	Version     string       `json:"version"`
	Positioning string       `json:"positioning"`
	Principle   []string     `json:"principle"`
	Tiers       []TierDef    `json:"tiers"`
	Fields      []FieldGroup `json:"fields"`
	Quotas      Quotas       `json:"quotas"`
	Export      ExportRule   `json:"export"`
	Prohibited  []string     `json:"prohibited"`
}

// 配额常量。改这里就等于改全站约束，所以集中在一处。
const (
	MaxPageSize     = 30
	DeepPageFreeMax = 300
	// RequestsPerMin 定在 600 而不是更小，是因为限流按 IP 计，而企业出口常常是
	// 一个 NAT 后面几十号人共用。定得太紧，第一个被误伤的是正常客户，不是爬虫。
	// 也要认清它的定位：限流只能拖慢爬取速度，真正的墙是字段分层。
	RequestsPerMin  = 600
	DetailFreeViews = 1
)

// 字段分组。顺序即"从严到宽"，方便前端按序渲染。
var fieldGroups = []FieldGroup{
	{
		Key: "contact", Label: "联系方式与联系人",
		Fields: []string{"contact_name", "contact_phone", "contact_email"},
		Tier:   TierPartner,
		Reason: "联系方式由供应商授权后开放，且仅在建立合作后可见。这是供应商愿意把真实产能与交期放上来的前提——如果谁都能拿到电话，供应商就没有理由留在平台内。需要联系可先发询价，由平台代转。",
	},
	{
		Key: "customers", Label: "客户名单",
		Fields: []string{"customers"},
		Tier:   TierPartner,
		Reason: "客户名单会暴露供应商正在服务哪些品牌，属于其最敏感的商业信息，泄露等于把它的客户结构交给竞争对手。",
	},
	{
		Key: "quoting", Label: "报价与历史价格",
		Fields: []string{"quote_price", "price_history"},
		Tier:   TierPartner,
		Reason: "价格是交易核心机密，且不同客户、不同批量价格本就不同，公开会造成误比价。",
	},
	{
		Key: "delivery_evidence", Label: "履约证据链",
		Fields: []string{"delivery_detail", "order_scorecard", "quality_events", "eight_d_report"},
		Tier:   TierPartner,
		Reason: "含具体订单号、批次数与不良明细，只对该供应商的合作方开放。其他企业只能看到聚合后的评级与已验证企业数。",
	},
	{
		Key: "finance", Label: "股东、人员与经营敏感项",
		Fields: []string{"shareholders", "key_personnel", "paid_capital", "tax_rating", "office_address"},
		Tier:   TierVerified,
		Reason: "股权结构与主要人员含个人信息，受《个人信息保护法》约束；精确办公地址与实缴资本属于尽调信息，需认证企业方可查看。",
	},
	{
		Key: "capability_detail", Label: "设备清单与产能明细",
		Fields: []string{"equipment_list", "capacity_description", "factory_area"},
		Tier:   TierVerified,
		Reason: "设备清单是同行做尽调最想抄的部分，认证后开放可以兼顾透明度与保护。",
	},
	{
		Key: "rating_score", Label: "评级分数与五维明细",
		Fields: []string{"total", "dimensions", "confidence", "spread"},
		Tier:   TierVerified,
		Reason: "评级由多家采购方的真实履约数据聚合而成。按数据对等原则，未贡献履约数据的企业只能看到等级与已验证企业数，看不到具体分数。",
	},
	{
		Key: "rating_grade", Label: "评级等级与已验证企业数",
		Fields: []string{"grade", "rated_buyers", "batch_count"},
		Tier:   TierPublic,
		Reason: "等级足够支撑“要不要进一步了解”的判断，是平台对外的核心信息，必须公开才能形成网络效应。",
	},
	{
		Key: "basic", Label: "工商基础信息",
		Fields: []string{"name", "credit_code_masked", "legal_representative", "registered_capital", "establish_date", "business_status", "province", "city", "capabilities"},
		Tier:   TierPublic,
		Reason: "工商公示信息本来就公开，且是搜索与筛选的基础；信用代码在公开层做掩码，可核验不可批量复制。",
	},
}

// Describe 返回当前生效的策略。接口与前端共用同一份定义，避免文档与代码打架。
func Describe() Doc {
	return Doc{
		Version:     "2026-10-28",
		Positioning: "供应商的“美团”：供应商是商家，采购企业是用户，平台用真实履约数据替代不可验证的主观推荐。",
		Principle: []string{
			"不卖排名：付费可买曝光位与增值服务，不能买评级与默认排序。",
			"不造假数据：没有真实单据就显示“暂无”，空页面优于假数据。",
			"不导出他人数据：企业自己的数据可导出，别人贡献的履约数据不行。",
			"给结论不给参数：公开层给等级，不给可被批量采集拼榜的分数。",
		},
		Tiers: []TierDef{
			{Tier: TierPublic, Label: "公开层", Who: "任何人（含未登录与搜索引擎）",
				HowToGet: "直接访问", CanSee: "工商基础信息、工艺能力标签、评级等级、已验证企业数",
				DataRight: "只读摘要，不可批量翻取"},
			{Tier: TierVerified, Label: "认证层", Who: "完成企业认证的采购方",
				HowToGet: "营业执照 + 对公打款验证", CanSee: "评级分数与五维明细、股东与人员、资质编号、设备与产能明细",
				DataRight: "需已贡献履约数据，方可查看他企贡献的分数"},
			{Tier: TierPartner, Label: "合作层", Who: "已与该供应商建立 SRM 关系的采购方",
				HowToGet: "引入 SRM 并转入“合作中”", CanSee: "联系方式、精确地址、订单评分卡、8D 报告、报价记录",
				DataRight: "仅限该合作关系内，不得对外披露"},
			{Tier: TierOwner, Label: "自有层", Who: "数据产生方本人",
				HowToGet: "天然拥有", CanSee: "自己录入的全部数据",
				DataRight: "可导出（这是企业自己的资产）"},
		},
		Fields: fieldGroups,
		Quotas: Quotas{
			MaxPageSize:     MaxPageSize,
			DeepPageFreeMax: DeepPageFreeMax,
			RequestsPerMin:  RequestsPerMin,
			DetailFreeViews: DetailFreeViews,
		},
		Export: ExportRule{
			Allowed: []string{
				"本企业录入的订单、交付批次、质量事件与供应商档案",
				"本企业的供应商评估结论与内部审核记录",
			},
			Denied: []string{
				"供应商名录的批量导出（CSV / Excel / 全量 JSON）",
				"股东、主要人员、客户名单、联系方式的导出",
				"其他企业贡献的履约原始数据",
				"平台评级榜单的批量导出",
			},
			Why:         "批量导出会把平台从“决策工具”降级为“数据源”：一次导出即可被转售、被建库、被脱离上下文传播，评级的样本口径与分歧度提示也随之丢失，反而误导使用者。",
			Alternative: "提供受控对比视图：选 3-5 家生成带水印的对比页，可打印 PDF，不提供原始数据文件；需要规模化分析的客户走 API 配额与授权协议。",
		},
		Prohibited: []string{
			"付费提升评级或默认排序",
			"人工调整 QCDST 分数（所有分数必须可追溯到单据）",
			"爬取第三方商业数据库后二次分发",
			"公开层直接返回自然人联系方式",
		},
	}
}
