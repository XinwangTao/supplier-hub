package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/example/supplier-hub/backend/internal/domain"
)

type AuditService struct{ suppliers *SupplierService }

func NewAuditService(suppliers *SupplierService) *AuditService {
	return &AuditService{suppliers: suppliers}
}

func item(title, description, priority string) domain.ChecklistItem {
	return domain.ChecklistItem{Title: title, Description: description, Priority: priority}
}

// Generate 依据企业档案动态生成现场审核清单：
// 法定代表人、股东结构、证书有效期、风险标签都会变成具体的核查动作，
// 而不是一份通用模板。
func (s *AuditService) Generate(ctx context.Context, id string) (domain.AuditChecklist, error) {
	supplier, err := s.suppliers.Get(ctx, id)
	if err != nil {
		return domain.AuditChecklist{}, err
	}

	shareholderNames := make([]string, 0, len(supplier.Shareholders))
	for _, h := range supplier.Shareholders {
		shareholderNames = append(shareholderNames, fmt.Sprintf("%s（%.0f%%）", h.Name, h.Ratio))
	}
	certNames := make([]string, 0, len(supplier.Certifications))
	for _, c := range supplier.Certifications {
		certNames = append(certNames, fmt.Sprintf("%s 至 %s", c.Name, c.ValidUntil))
	}

	sections := []domain.ChecklistSection{
		{Category: "主体资格与法人核验", Items: []domain.ChecklistItem{
			item("核对营业执照原件", fmt.Sprintf("统一社会信用代码 %s，经营状态 %s，确认与合同主体一致", supplier.CreditCode, supplier.BusinessStatus), "必查"),
			item("法定代表人身份核验", fmt.Sprintf("法定代表人 %s（%s），需现场出示身份证原件并核对签字样本；若由其授权代表签约，须提供加盖公章的授权委托书", supplier.LegalRepresentative, supplier.LegalRepTitle), "必查"),
			item("股东与实际控制人穿透", "股权结构："+strings.Join(shareholderNames, "、")+"。核查是否存在代持、对赌或股权质押，确认最终决策人", "必查"),
			item("注册地址与经营地址一致性", fmt.Sprintf("注册地址：%s；实际办公/生产地址：%s。不一致时索取情况说明及场地租赁合同", supplier.RegisteredAddress, supplier.OfficeAddress), "建议"),
		}},
		{Category: "资质证照", Items: []domain.ChecklistItem{
			item("核验体系证书有效性", "现场查看证书原件与认证范围："+strings.Join(certNames, "；"), "必查"),
			item("证书主体与产线对应", "确认认证覆盖的产线与本次采购品类一致，避免用总部证书覆盖外协产线", "必查"),
		}},
		{Category: "生产现场", Items: []domain.ChecklistItem{
			item("查看产线与工序布局", fmt.Sprintf("核对实际产线数量与在制品状态，与档案记录（%s）是否一致", supplier.CapacityDescription), "必查"),
			item("检查现场安全与 5S", "记录消防、危化品、通道和物料标识情况", "建议"),
			item("关键工序人员访谈", "随机访谈 2–3 名关键工序操作员，确认工艺参数受控、培训到位", "建议"),
		}},
		{Category: "设备与产能", Items: []domain.ChecklistItem{
			item("抽查关键设备台账", "核对设备型号、数量、保养记录与稼动率，确认与报称产能匹配", "必查"),
			item("验证交付节奏", fmt.Sprintf("供应商承诺交期 %d 天、最小起订量 %d，通过近期订单与排产表交叉验证", supplier.LeadTimeDays, supplier.MOQ), "必查"),
		}},
		{Category: "质量与研发", Items: []domain.ChecklistItem{
			item("查看来料与出货检验记录", "抽查近三个月 IQC/OQC 记录，核对不良率与纠正措施闭环", "必查"),
			item("现场见证关键尺寸检验", "按采购图纸抽取样品，确认量具校准状态与检验方法", "必查"),
			item("研发与专利真实性", fmt.Sprintf("发明专利 %d 项、实用新型 %d 项，研发人员 %d 人；抽查专利证书与研发项目台账", supplier.InventionPatents, supplier.UtilityPatents, supplier.RndStaff), "建议"),
		}},
		{Category: "商务与风险", Items: []domain.ChecklistItem{
			item("财务与纳税核验", fmt.Sprintf("纳税信用等级 %s，年营收约 %d 万元；索取近两年审计报告与银行流水摘要", supplier.TaxRating, supplier.AnnualRevenue), "必查"),
			item("核对风险标签", "风险标签："+strings.Join(supplier.RiskTags, "、")+"。逐项索取原始材料与处理进展，重点确认是否影响履约", "必查"),
			item("进出口与合规", fmt.Sprintf("进出口资质：%s；确认报关主体、原产地与关税承担方式", boolWord(supplier.ImportExport, "具备", "不具备")), "建议"),
			item("备选产能与应急预案", "了解单点设备、关键原料与交付中断的替代方案，约定违约与赔偿条款", "建议"),
		}},
	}

	return domain.AuditChecklist{
		SupplierID:   supplier.ID,
		SupplierName: supplier.Name,
		Summary:      fmt.Sprintf("%s（法定代表人 %s）现场审核重点：先核主体与法人授权，再看产线与产能是否匹配，最后落到证书有效期与风险标签闭环。", supplier.Name, supplier.LegalRepresentative),
		Sections:     sections,
	}, nil
}

func boolWord(v bool, yes, no string) string {
	if v {
		return yes
	}
	return no
}
