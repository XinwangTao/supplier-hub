// Package ingest 负责把外部真实数据源（企查查/天眼查导出表、开放平台 API）
// 标准化成 domain.Supplier，并从经营范围里推断工艺能力，让真实企业能挂到供应链网上。
//
// 这个包不生成任何数据：所有字段都来自入参，缺什么就空着，绝不编造。
package ingest

import (
	"sort"
	"strings"

	"github.com/example/supplier-hub/backend/internal/catalog"
	"github.com/example/supplier-hub/backend/internal/domain"
)

// Record 是外部数据源的一行原始记录，字段名贴近各平台的导出列。
// 缺失字段留空，由上层决定是否可用；这里不做任何默认值填充。
type Record struct {
	Name                string
	CreditCode          string
	LegalRepresentative string
	LegalRepTitle       string
	BusinessStatus      string
	EnterpriseType      string
	RegisteredAddress   string
	OfficeAddress       string
	EstablishDate       string
	RegisteredCapital   int64 // 万元
	PaidCapital         int64 // 万元
	TaxRating           string
	InsuredCount        int
	EmployeeCount       int
	BusinessScope       string
	Industry            string
	Province            string
	City                string
	District            string
	ContactPhone        string
	ContactEmail        string
	Website             string
	Source              string // 数据来源名称，如「企查查开放平台」
	SourceURL           string // 可回查的原始记录地址
	VerifiedAt          string // 最近一次核验时间（RFC3339）
}

// capabilityKeywords 是「经营范围 → 工艺能力」的判定词表。
// 真实企业的经营范围是长文本，靠关键词命中来归类，命中条数越多排名越前。
var capabilityKeywords = map[string][]string{
	"cnc":        {"数控", "CNC", "cnc", "精密加工", "机械加工", "车铣", "精雕", "机加工", "加工中心"},
	"mold":       {"模具", "模胚", "模架", "治具"},
	"injection":  {"注塑", "塑料制品", "塑料零件", "双色注塑", "包胶"},
	"diecasting": {"压铸", "锌合金", "镁合金", "铝合金压铸", "半固态"},
	"stamping":   {"冲压", "弹片", "屏蔽罩", "深拉伸", "级进模"},
	"mim":        {"粉末冶金", "金属注射", "MIM", "烧结", "精整"},
	"anodizing":  {"阳极氧化", "氧化处理", "电泳", "硬质氧化"},
	"coating":    {"电镀", "镀膜", "真空镀", "PVD", "喷涂", "表面处理", "喷油"},
	"glass":      {"玻璃", "盖板", "视窗", "钢化"},
	"silicone":   {"硅胶", "橡胶", "液态硅胶", "密封圈"},
	"band":       {"表带", "织带", "皮革制品", "手带", "编织带"},
	"fastener":   {"螺丝", "紧固件", "螺钉", "五金件", "铆钉"},
	"laser":      {"激光", "打标", "镭雕", "激光切割"},
	"welding":    {"焊接", "钣金", "铆接", "冲压焊接"},
	"pcb":        {"电路板", "PCB", "印制电路", "线路板", "覆铜板"},
	"pcba":       {"SMT", "贴片", "PCBA", "电子组装", "插件", "波峰焊"},
	"fpc":        {"柔性电路", "FPC", "软板", "柔性线路"},
	"display":    {"显示", "液晶", "OLED", "LCD", "触控屏", "显示屏"},
	"camera":     {"摄像头", "摄像模组", "影像", "镜头模组", "相机"},
	"sensor":     {"传感器", "感应器", "敏感元件"},
	"battery":    {"电池", "电芯", "锂电", "聚合物", "PACK"},
	"charger":    {"充电", "电源", "适配器", "无线充", "变压器"},
	"motor":      {"电机", "马达", "驱动", "微特电机"},
	"acoustics":  {"扬声器", "喇叭", "麦克风", "声学", "耳机", "受话器"},
	"connector":  {"连接器", "线束", "端子", "排线", "接插件"},
	"magnet":     {"磁铁", "磁性材料", "磁材", "钕铁硼"},
	"lens":       {"镜头", "光学", "镜片"},
	"antenna":    {"天线", "射频", "RF", "微波"},
	"heatsink":   {"散热", "导热", "均热板", "热管", "散热器"},
	"testing":    {"检测", "认证", "试验", "校准", "检验", "测试"},
	"packaging":  {"包装", "印刷", "纸品", "彩盒", "说明书"},
	"assembly":   {"组装", "整机", "代工", "装配", "OEM", "ODM", "贴牌"},
}

// lowSignalCapabilities 里的能力词太泛（"制造""安装"满街都是），
// 只有在没有更具体的命中时才作为兜底，否则所有企业都会被归到整机组装。
var lowSignalCapabilities = map[string]bool{"assembly": true, "testing": true, "packaging": true, "coating": true, "laser": true}

// Classify 从企业名称与经营范围推断工艺能力代码，按相关度排序，最多返回 max 个。
// 找不到任何具体命中时返回空，绝不强行归类——宁可在图上缺一个点，也不要挂错链。
func Classify(name, scope string, max int) []string {
	if max <= 0 {
		max = 3
	}
	text := strings.ToLower(name + " " + scope)
	if strings.TrimSpace(text) == "" {
		return nil
	}

	type hit struct {
		code  string
		score int
	}
	hits := make([]hit, 0, len(capabilityKeywords))
	for code, words := range capabilityKeywords {
		score := 0
		for _, word := range words {
			if strings.Contains(text, strings.ToLower(word)) {
				score++
			}
		}
		if score > 0 {
			hits = append(hits, hit{code: code, score: score})
		}
	}

	// 具体能力优先；泛化能力只在没有具体命中时补位
	specific := make([]hit, 0, len(hits))
	generic := make([]hit, 0, len(hits))
	for _, item := range hits {
		if lowSignalCapabilities[item.code] {
			generic = append(generic, item)
		} else {
			specific = append(specific, item)
		}
	}
	if len(specific) == 0 {
		specific = generic
	}

	sort.SliceStable(specific, func(i, j int) bool {
		if specific[i].score != specific[j].score {
			return specific[i].score > specific[j].score
		}
		return specific[i].code < specific[j].code
	})

	if len(specific) > max {
		specific = specific[:max]
	}
	out := make([]string, 0, len(specific))
	for _, item := range specific {
		out = append(out, item.code)
	}
	return out
}

// CapabilityNames 把能力代码翻译成中文名，供前端展示。
func CapabilityNames(codes []string) []string {
	names := make([]string, 0, len(codes))
	for _, code := range codes {
		if info, ok := catalog.CapabilityByCode(code); ok {
			names = append(names, info.Name)
		}
	}
	return names
}

// ToSupplier 把一条原始记录转成领域对象。所有字段原样透传，不做补全。
func ToSupplier(rec Record, id string) domain.Supplier {
	caps := Classify(rec.Name, rec.BusinessScope, 3)
	province, city, district := rec.Province, rec.City, rec.District
	if province == "" || city == "" {
		p, c, d := catalog.SplitAddress(rec.RegisteredAddress)
		if province == "" {
			province = p
		}
		if city == "" {
			city = c
		}
		if district == "" {
			district = d
		}
	}

	status := strings.TrimSpace(rec.BusinessStatus)
	risk := domain.RiskLow
	var riskTags []string
	switch {
	case status == "":
		risk = domain.RiskMedium
		riskTags = []string{"经营状态未知"}
	case strings.Contains(status, "注销"), strings.Contains(status, "吊销"), strings.Contains(status, "清算"):
		risk = domain.RiskHigh
		riskTags = []string{status}
	case strings.Contains(status, "异常"), strings.Contains(status, "迁出"):
		risk = domain.RiskMedium
		riskTags = []string{status}
	default:
		riskTags = []string{"经营存续"}
	}

	return domain.Supplier{
		ID:                  id,
		Name:                rec.Name,
		ShortName:           ShortName(rec.Name),
		CreditCode:          rec.CreditCode,
		LegalRepresentative: rec.LegalRepresentative,
		LegalRepTitle:       rec.LegalRepTitle,
		BusinessStatus:      status,
		EnterpriseType:      rec.EnterpriseType,
		RegisteredAddress:   rec.RegisteredAddress,
		OfficeAddress:       rec.OfficeAddress,
		EstablishDate:       rec.EstablishDate,
		RegisteredCapital:   rec.RegisteredCapital,
		PaidCapital:         rec.PaidCapital,
		TaxRating:           rec.TaxRating,
		InsuredCount:        rec.InsuredCount,
		EmployeeCount:       rec.EmployeeCount,
		BusinessScope:       rec.BusinessScope,
		Capabilities:        caps,
		CapabilityNames:     CapabilityNames(caps),
		Province:            province,
		City:                city,
		District:            district,
		IsManufacturer:      isManufacturer(rec),
		RiskLevel:           risk,
		RiskTags:            riskTags,
		ContactPhone:        rec.ContactPhone,
		ContactEmail:        rec.ContactEmail,
		Website:             rec.Website,
		DataSource:          rec.Source,
		SourceURL:           rec.SourceURL,
		LastVerifiedAt:      rec.VerifiedAt,
	}
}

func isManufacturer(rec Record) bool {
	scope := rec.BusinessScope
	for _, word := range []string{"生产", "制造", "加工", "研发", "组装"} {
		if strings.Contains(scope, word) {
			return true
		}
	}
	return false
}

// ShortName 从企业全称里取一个便于上图展示的简称：
// 去掉行政区划前缀与组织形式后缀，保留字号与行业词。
func ShortName(name string) string {
	s := strings.TrimSpace(name)
	if s == "" {
		return ""
	}
	for _, suffix := range []string{"股份有限公司", "有限责任公司", "有限公司", "集团", "公司"} {
		if strings.HasSuffix(s, suffix) {
			s = strings.TrimSuffix(s, suffix)
			break
		}
	}
	prefixes := []string{
		"北京市", "上海市", "天津市", "重庆市",
		"北京", "上海", "天津", "重庆",
		"广东省", "江苏省", "浙江省", "福建省", "山东省", "河南省", "河北省", "湖北省",
		"湖南省", "安徽省", "四川省", "江西省", "陕西省", "辽宁省", "吉林省", "黑龙江省",
		"云南省", "贵州省", "广西壮族自治区", "内蒙古自治区", "山西省", "甘肃省", "海南省",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimPrefix(s, prefix)
			break
		}
	}
	// 再剥掉一层「深圳市」「东莞市」这类地级市前缀（1~4 字 + 市）
	runes := []rune(s)
	for i, ch := range runes {
		if ch == '市' && i > 0 && i <= 4 {
			s = string(runes[i+1:])
			break
		}
	}
	if len([]rune(s)) > 10 {
		s = string([]rune(s)[:10])
	}
	return s
}
