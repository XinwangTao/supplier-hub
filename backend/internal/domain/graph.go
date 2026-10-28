package domain

import "time"

// NodeType 定义网状图里的节点层级：
// product（产品）→ module（模块）→ process（工序）→ supplier（供应商）。
// 在全域网模式下还会有 capability（工艺能力）节点，它是产品与供应商共同的锚点。
type NodeType string

const (
	NodeProduct    NodeType = "product"
	NodeModule     NodeType = "module"
	NodeProcess    NodeType = "process"
	NodeSupplier   NodeType = "supplier"
	NodeCapability NodeType = "capability"
)

// GraphNode 是网络图的一个节点。前端据此决定大小、颜色与交互。
type GraphNode struct {
	ID         string            `json:"id"`
	Type       NodeType          `json:"type"`
	Label      string            `json:"label"`
	Subtitle   string            `json:"subtitle"`
	Level      int               `json:"level"` // 0 产品 1 模块 2 工序 3 供应商
	ParentID   string            `json:"parent_id,omitempty"`
	SupplierID string            `json:"supplier_id,omitempty"`
	Capability string            `json:"capability,omitempty"`
	Risk       RiskLevel         `json:"risk,omitempty"`
	Score      int               `json:"score,omitempty"`
	Meta       map[string]string `json:"meta,omitempty"`
}

type GraphEdge struct {
	ID       string  `json:"id"`
	Source   string  `json:"source"`
	Target   string  `json:"target"`
	Relation string  `json:"relation"` // bom / process / supply / alternative / cooccur / same_city
	Label    string  `json:"label,omitempty"`
	Weight   float64 `json:"weight"`
}

// Insight 是自动推导出的供应链判断，而不是罗列数据。
type Insight struct {
	Kind    string   `json:"kind"`  // single_source / gap / concentration / risk / strength
	Level   string   `json:"level"` // info / warn / critical
	Title   string   `json:"title"`
	Detail  string   `json:"detail"`
	NodeIDs []string `json:"node_ids,omitempty"`
}

type GraphStats struct {
	Modules      int `json:"modules"`
	Processes    int `json:"processes"`
	Suppliers    int `json:"suppliers"`
	Cities       int `json:"cities"`
	HighRisk     int `json:"high_risk"`
	Capabilities int `json:"capabilities"`
	Edges        int `json:"edges"`
}

type Graph struct {
	Mode        string      `json:"mode"` // product / mesh / supplier
	Query       string      `json:"query"`
	Product     string      `json:"product"`
	Summary     string      `json:"summary"`
	Center      string      `json:"center"`
	Nodes       []GraphNode `json:"nodes"`
	Edges       []GraphEdge `json:"edges"`
	Stats       GraphStats  `json:"stats"`
	Insights    []Insight   `json:"insights"`
	GeneratedAt time.Time   `json:"generated_at"`
}

type GraphRequest struct {
	Query                  string `json:"query"`
	Product                string `json:"product"`
	MaxSuppliersPerProcess int    `json:"max_suppliers_per_process"`
	Region                 string `json:"region"`
	CriticalOnly           bool   `json:"critical_only"`
}

// ProductSummary 用于前端示例与产品库列表。
type ProductSummary struct {
	Product      string   `json:"product"`
	Category     string   `json:"category"`
	Summary      string   `json:"summary"`
	Aliases      []string `json:"aliases"`
	Modules      int      `json:"modules"`
	Processes    int      `json:"processes"`
	Capabilities int      `json:"capabilities"`
	Suppliers    int      `json:"suppliers"`
}

// CapabilitySummary 工艺能力库的一行：多少家在供、被哪些产品用到。
// 全域网侧栏与能力检索用它做"能力的索引页"。
type CapabilitySummary struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Processes   []string `json:"processes"`
	Regions     []string `json:"regions"`
	Suppliers   int      `json:"suppliers"`
	Products    []string `json:"products"`
}
