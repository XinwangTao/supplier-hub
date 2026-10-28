package domain

// OverallScore 四维综合评分输出(美团式):每个分数都能展开到维度 → 子项口径。
// 类型放在 domain 是为了让它与具体评分实现解耦——scoring 包负责算,这里负责契约。
type OverallDimension struct {
	Key    string  `json:"key"`
	Label  string  `json:"label"`
	Score  float64 `json:"score"`  // 0–100,无数据为 0
	Weight float64 `json:"weight"` // 0–1
	Flag   string  `json:"flag"`   // "" | insufficient_evidence | no_data
	Note   string  `json:"note"`   // 一句话口径
}

type OverallScore struct {
	// 供应商身份(榜单条目直接可渲染)
	SupplierID   string   `json:"supplier_id"`
	SupplierName string   `json:"supplier_name"`
	ShortName    string   `json:"short_name"`
	CreditCode   string   `json:"credit_code"`
	City         string   `json:"city"`
	Capabilities []string `json:"capabilities"`

	Total      float64            `json:"total"` // 0–100
	Dimensions []OverallDimension `json:"dimensions"`
	Flags      []string           `json:"flags"`
	Confidence string             `json:"confidence"` // high | medium | low:有几个维度有真数据
}
