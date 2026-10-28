package domain

// 五维档案(人·机·料·法·环)是巡厂时人工掌握的供应商现场事实,
// 与工商信息(企查查)、工艺能力标签(经营范围推断)、QCDST(单据评分)互不替代。
// 这里的每一条都来自录入,缺失就空着——宁可在档案上显示「—」,也不要填出看似合理的值。
//
// 它不是评分:QCDST 回答"这家靠不靠谱",五维档案回答"这家长什么样"。

// DeptHeadcount 部门人数分布。部门名单由录入方现场掌握,预设品质/开发/业务/生产/高层管理,允许自定义。
type DeptHeadcount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// MachineItem 关键设备台账(巡厂时抄录的型号与役龄)。
type MachineItem struct {
	Name    string `json:"name"`
	Model   string `json:"model"`
	Qty     int    `json:"qty"`
	AgeYear int    `json:"age_year"` // 役龄(年)
}

type FourM1E struct {
	SupplierID string `json:"supplier_id"`

	// 人 Man
	EmployeeTotal int             `json:"employee_total"` // 员工总数(现场口径,可与工商参保数不同)
	RnDCount      int             `json:"rnd_count"`      // 研发/工程人员数
	Departments   []DeptHeadcount `json:"departments"`    // 部门分布
	KeyPersonnel  string          `json:"key_personnel"`  // 高管与质量负责人(姓名/职务)

	// 机 Machine
	FactoryArea         int           `json:"factory_area"`      // 厂房面积 ㎡
	ProductionLines     int           `json:"production_lines"`  // 产线数
	Machines            []MachineItem `json:"machines"`          // 关键设备清单
	AutomationRate      int           `json:"automation_rate"`   // 自动化率 %
	CapacityUtilization int           `json:"capacity_utilization"` // 产能利用率 %

	// 料 Material
	MainMaterials      string `json:"main_materials"`      // 主要原材料及来源地
	DualSourcing       string `json:"dual_sourcing"`       // 关键物料双源情况
	IQCMethod          string `json:"iqc_method"`          // 来料检验方式
	MaterialCompliance string `json:"material_compliance"` // 材料合规(RoHS/REACH 等)

	// 法 Method
	Certifications       []string `json:"certifications"`       // 体系认证(ISO9001/IATF16949/ISO13485...)
	DesignCapability     string   `json:"design_capability"`    // 设计能力
	InspectionCapability string   `json:"inspection_capability"` // 检测能力
	ProcessDocs          string   `json:"process_docs"`         // 工艺文件完备度
	ChangeManagement     string   `json:"change_management"`    // 变更管理

	// 环 Environment
	EnvCertification string `json:"env_certification"` // 环境体系(ISO14001 等)
	EnvPenalty       string `json:"env_penalty"`       // 环保处罚记录
	WasteTreatment   string `json:"waste_treatment"`   // 能耗与三废处理
	Location         string `json:"location"`          // 所在地(厂区)

	// 留痕:谁在什么时候录入的。快照追加式,历史不可覆盖。
	RecordedBy string `json:"recorded_by"`
	RecordedAt string `json:"recorded_at"` // RFC3339,服务端落库时盖戳
}
