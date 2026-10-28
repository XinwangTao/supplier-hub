package domain

import "time"

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// Shareholder 股东信息，用于判断企业的实际控制关系与决策链。
type Shareholder struct {
	Name       string  `json:"name"`
	Ratio      float64 `json:"ratio"`      // 持股比例，百分比
	Type       string  `json:"type"`       // 自然人 / 企业法人 / 员工持股平台
	Subscribed int64   `json:"subscribed"` // 认缴出资额（万元）
}

// Personnel 主要人员，法定代表人与高管往往决定商务推进效率。
type Personnel struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

// Certification 资质证书，带发证机构与有效期，便于现场核验。
type Certification struct {
	Name       string `json:"name"`
	Issuer     string `json:"issuer"`
	ValidUntil string `json:"valid_until"`
	Scope      string `json:"scope"`
}

type Supplier struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"short_name"` // 图上展示用的简称，如"恒信电子"

	// ---- 工商登记 ----
	CreditCode          string        `json:"credit_code"`
	LegalRepresentative string        `json:"legal_representative"`
	LegalRepTitle       string        `json:"legal_rep_title"`
	BusinessStatus      string        `json:"business_status"`
	EnterpriseType      string        `json:"enterprise_type"`
	RegisteredAddress   string        `json:"registered_address"`
	OfficeAddress       string        `json:"office_address"`
	EstablishDate       string        `json:"establish_date"`
	RegisteredCapital   int64         `json:"registered_capital"`
	PaidCapital         int64         `json:"paid_capital"`
	TaxRating           string        `json:"tax_rating"`
	ImportExport        bool          `json:"import_export"`
	HasBranches         bool          `json:"has_branches"`
	AnnualRevenue       int64         `json:"annual_revenue"` // 万元
	InventionPatents    int           `json:"invention_patents"`
	UtilityPatents      int           `json:"utility_patents"`
	Shareholders        []Shareholder `json:"shareholders"`
	KeyPersonnel        []Personnel   `json:"key_personnel"`
	LastVerifiedAt      string        `json:"last_verified_at"`

	// ---- 经营与产能 ----
	BusinessScope       string          `json:"business_scope"`
	MainProducts        []string        `json:"main_products"`
	Capabilities        []string        `json:"capabilities"` // 能力代码，网络图的挂接点
	CapabilityNames     []string        `json:"capability_names"`
	CapacityDescription string          `json:"capacity_description"`
	FactoryArea         int             `json:"factory_area"` // ㎡
	EmployeeCount       int             `json:"employee_count"`
	InsuredCount        int             `json:"insured_count"`
	RndStaff            int             `json:"rnd_staff"`
	MOQ                 int             `json:"moq"`
	LeadTimeDays        int             `json:"lead_time_days"`
	Certifications      []Certification `json:"certifications"`
	Customers           []string        `json:"customers"`

	// ---- 地区与风险 ----
	Province       string    `json:"province"`
	City           string    `json:"city"`
	District       string    `json:"district"`
	IsManufacturer bool      `json:"is_manufacturer"`
	RiskLevel      RiskLevel `json:"risk_level"`
	RiskTags       []string  `json:"risk_tags"`

	// ---- 联系与数据来源 ----
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
	ContactEmail string `json:"contact_email"`
	Website      string `json:"website"`
	DataSource   string `json:"data_source"` // 数据来自哪家平台
	SourceURL    string `json:"source_url"`  // 可回查的原始记录地址

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SearchFilters struct {
	BusinessKeyword  string    `json:"business_keyword"`
	Province         string    `json:"province"`
	City             string    `json:"city"`
	MinCapital       int64     `json:"min_capital"`
	MaxCapital       int64     `json:"max_capital"`
	MinEmployees     int       `json:"min_employees"`
	MaxEmployees     int       `json:"max_employees"`
	ManufacturerOnly *bool     `json:"manufacturer_only"`
	MaxRisk          RiskLevel `json:"max_risk"`
	Capability       string    `json:"capability"`
}

type SearchRequest struct {
	Query    string        `json:"query"`
	Filters  SearchFilters `json:"filters"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

type ScoredSupplier struct {
	Supplier
	MatchScore int      `json:"match_score"`
	Reasons    []string `json:"reasons"`
}

type SearchResult struct {
	Items          []ScoredSupplier `json:"items"`
	Total          int              `json:"total"`
	Page           int              `json:"page"`
	PageSize       int              `json:"page_size"`
	Applied        SearchFilters    `json:"applied_filters"`
	Interpretation string           `json:"interpretation"`
}

type ChecklistItem struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
}

type ChecklistSection struct {
	Category string          `json:"category"`
	Items    []ChecklistItem `json:"items"`
}

type AuditChecklist struct {
	SupplierID   string             `json:"supplier_id"`
	SupplierName string             `json:"supplier_name"`
	Summary      string             `json:"summary"`
	Sections     []ChecklistSection `json:"sections"`
}
