package domain

import "time"

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type Supplier struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	CreditCode          string    `json:"credit_code"`
	BusinessScope       string    `json:"business_scope"`
	RegisteredCapital   int64     `json:"registered_capital"`
	PaidCapital         int64     `json:"paid_capital"`
	EmployeeCount       int       `json:"employee_count"`
	EstablishDate       string    `json:"establish_date"`
	Province            string    `json:"province"`
	City                string    `json:"city"`
	IsManufacturer      bool      `json:"is_manufacturer"`
	RiskLevel           RiskLevel `json:"risk_level"`
	RiskTags            []string  `json:"risk_tags"`
	Certifications      []string  `json:"certifications"`
	MainProducts        []string  `json:"main_products"`
	CapacityDescription string    `json:"capacity_description"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
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
