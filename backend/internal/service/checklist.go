package service

import (
	"context"
	"github.com/example/supplier-hub/backend/internal/domain"
)

type AuditService struct{ suppliers *SupplierService }

func NewAuditService(suppliers *SupplierService) *AuditService {
	return &AuditService{suppliers: suppliers}
}

func (s *AuditService) Generate(ctx context.Context, id string) (domain.AuditChecklist, error) {
	supplier, err := s.suppliers.Get(ctx, id)
	if err != nil {
		return domain.AuditChecklist{}, err
	}
	return domain.AuditChecklist{SupplierID: supplier.ID, SupplierName: supplier.Name, Summary: supplier.Name + "现场审核重点：核实生产能力、质量体系与风险标签，形成可追溯结论。", Sections: []domain.ChecklistSection{
		{Category: "资质证照", Items: []domain.ChecklistItem{{"核验营业执照与统一社会信用代码", "确认证照主体、经营期限和经营范围覆盖本次采购品类", "必查"}, {"核验体系证书有效性", "现场查看 ISO 9001、ISO 14001 等证书原件及认证范围", "必查"}}},
		{Category: "生产现场", Items: []domain.ChecklistItem{{"查看产线与工序布局", "核对实际产线、在制品和订单状态，确认与介绍资料一致", "必查"}, {"检查现场安全与 5S", "记录消防、危化品、通道和物料标识情况", "建议"}}},
		{Category: "设备产能", Items: []domain.ChecklistItem{{"抽查关键设备台账", "核对设备型号、数量、保养记录和稼动率", "必查"}, {"验证产能与交付计划", "通过近期订单、排产表和出货记录交叉验证产能", "必查"}}},
		{Category: "人员管理", Items: []domain.ChecklistItem{{"核实核心岗位配置", "确认质量、工程、生产负责人及关键岗位稳定性", "建议"}, {"抽查培训与上岗记录", "查看特种作业和关键工序培训记录", "建议"}}},
		{Category: "质量体系", Items: []domain.ChecklistItem{{"查看来料与出货检验记录", "抽查近三个月记录，核对不良率和纠正措施闭环", "必查"}, {"现场见证关键尺寸检验", "按采购图纸抽取样品，确认量具校准和检验方法", "必查"}}},
		{Category: "潜在风险", Items: []domain.ChecklistItem{{"核对风险标签及涉诉情况", "对已有风险标签逐项获取原始材料与处理进展", "必查"}, {"确认备选产能与应急预案", "了解单点设备、关键原料和交付中断的替代方案", "建议"}}},
	}}, nil
}
