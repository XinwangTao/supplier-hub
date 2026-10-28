package ingest

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// headerAliases 把各家平台导出表的列名映射到内部字段。
// 企查查、天眼查、启信宝、国家企业信用信息公示系统的导出列名都不一样，
// 这里一次性覆盖常见写法，避免用户为了导数据去改表头。
var headerAliases = map[string][]string{
	"name":                 {"企业名称", "公司名称", "企业名", "供应商名称", "客户名称", "name", "company_name", "company"},
	"credit_code":          {"统一社会信用代码", "社会信用代码", "信用代码", "纳税人识别号", "税号", "credit_code", "uscc", "unified_code"},
	"legal_representative": {"法定代表人", "法人代表", "法人", "法定代表人姓名", "legal_representative", "legal_person", "legalperson"},
	"legal_rep_title":      {"法定代表人职务", "法人职务", "职务", "legal_rep_title"},
	"business_status":      {"经营状态", "登记状态", "企业状态", "状态", "business_status", "status"},
	"enterprise_type":      {"企业类型", "公司类型", "主体类型", "enterprise_type", "company_type"},
	"registered_address":   {"注册地址", "企业地址", "住所", "地址", "registered_address", "address"},
	"office_address":       {"经营地址", "办公地址", "实际经营地址", "office_address"},
	"establish_date":       {"成立日期", "成立时间", "注册日期", "注册时间", "establish_date", "founded_at"},
	"registered_capital":   {"注册资本", "注册资金", "registered_capital", "capital"},
	"paid_capital":         {"实缴资本", "实收资本", "paid_capital"},
	"tax_rating":           {"纳税信用等级", "纳税等级", "纳税评级", "tax_rating"},
	"insured_count":        {"参保人数", "社保人数", "insure_count", "insured_count"},
	"employee_count":       {"员工人数", "人员规模", "员工总数", "employee_count", "employees"},
	"business_scope":       {"经营范围", "经营范围（一般经营项目）", "许可经营项目", "business_scope", "scope"},
	"industry":             {"所属行业", "行业", "国标行业", "industry"},
	"province":             {"省份", "省", "province"},
	"city":                 {"城市", "市", "所在城市", "city"},
	"district":             {"区县", "区", "县", "district"},
	"contact_phone":        {"联系电话", "电话", "手机", "phone", "tel"},
	"contact_email":        {"邮箱", "电子邮箱", "email"},
	"website":              {"网址", "官网", "网站", "website", "url"},
	"source":               {"数据来源", "来源", "source"},
	"source_url":           {"详情链接", "来源链接", "链接", "source_url"},
}

// LoadCSV 读取一份真实企业清单（CSV/TSV），返回标准化后的记录。
// 只做列名对齐与类型转换，绝不补造缺失字段。
func LoadCSV(path string) ([]Record, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开数据文件失败: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("读取数据文件失败: %w", err)
	}

	text := strings.TrimPrefix(string(data), "\ufeff")
	delimiter := ','
	if strings.Count(firstLine(text), "\t") > strings.Count(firstLine(text), ",") {
		delimiter = '\t'
	}

	reader := csv.NewReader(strings.NewReader(text))
	reader.Comma = delimiter
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1

	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("解析 CSV 失败: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("数据文件只有表头，没有企业记录")
	}

	index := map[string]int{}
	for i, header := range rows[0] {
		key := strings.ToLower(strings.TrimSpace(header))
		key = strings.TrimPrefix(key, "\ufeff")
		for field, aliases := range headerAliases {
			if _, filled := index[field]; filled {
				continue
			}
			for _, alias := range aliases {
				if key == strings.ToLower(alias) {
					index[field] = i
					break
				}
			}
		}
	}
	if _, ok := index["name"]; !ok {
		return nil, fmt.Errorf("表头里找不到「企业名称」列，请确认导出的是工商信息表")
	}

	records := make([]Record, 0, len(rows)-1)
	for lineNo, row := range rows[1:] {
		pick := func(field string) string {
			idx, ok := index[field]
			if !ok || idx >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[idx])
		}
		name := pick("name")
		if name == "" {
			continue
		}

		source := pick("source")
		if source == "" {
			source = "企业工商信息导入表"
		}

		records = append(records, Record{
			Name:                name,
			CreditCode:          pick("credit_code"),
			LegalRepresentative: pick("legal_representative"),
			LegalRepTitle:       pick("legal_rep_title"),
			BusinessStatus:      pick("business_status"),
			EnterpriseType:      pick("enterprise_type"),
			RegisteredAddress:   pick("registered_address"),
			OfficeAddress:       pick("office_address"),
			EstablishDate:       normalizeDate(pick("establish_date")),
			RegisteredCapital:   ParseCapital(pick("registered_capital")),
			PaidCapital:         ParseCapital(pick("paid_capital")),
			TaxRating:           pick("tax_rating"),
			InsuredCount:        ParseCount(pick("insured_count")),
			EmployeeCount:       ParseCount(pick("employee_count")),
			BusinessScope:       pick("business_scope"),
			Industry:            pick("industry"),
			Province:            pick("province"),
			City:                pick("city"),
			District:            pick("district"),
			ContactPhone:        pick("contact_phone"),
			ContactEmail:        pick("contact_email"),
			Website:             pick("website"),
			Source:              source,
			SourceURL:           pick("source_url"),
		})
		_ = lineNo
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("数据文件里没有可用的企业记录")
	}
	return records, nil
}

func firstLine(s string) string {
	if idx := strings.IndexAny(s, "\r\n"); idx >= 0 {
		return s[:idx]
	}
	return s
}

// parseCapital 把「1,200万人民币」「1.5亿元」解析成万元整数。
// ParseCapital 把「1,200万人民币」「1.5亿元」这类写法解析成万元整数。
func ParseCapital(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	cleaned := strings.NewReplacer(",", "", "，", "", " ", "", "人民币", "", "元", "").Replace(raw)
	multiplier := 1.0
	if strings.Contains(cleaned, "亿") {
		multiplier = 10000
		cleaned = strings.ReplaceAll(cleaned, "亿", "")
	}
	cleaned = strings.ReplaceAll(cleaned, "万", "")
	cleaned = strings.TrimSpace(cleaned)
	value, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0
	}
	return int64(value * multiplier)
}

// ParseCount 从「1,234人」「500-999人」这类写法里取出人数。
func ParseCount(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, raw)
	if digits == "" {
		return 0
	}
	value, err := strconv.Atoi(digits)
	if err != nil {
		return 0
	}
	return value
}

// normalizeDate 把 20230105 / 2023-1-5 / 2023/01/05 统一成 2023-01-05。
func normalizeDate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '-' || r == '/' || r == '.' || r == '年' || r == '月' || r == '日'
	})
	if len(parts) == 3 {
		year, month, day := parts[0], parts[1], parts[2]
		if len(year) == 4 {
			return fmt.Sprintf("%s-%02s-%02s", year, pad2(month), pad2(day))
		}
	}
	if len(raw) == 8 {
		if _, err := strconv.Atoi(raw); err == nil {
			return fmt.Sprintf("%s-%s-%s", raw[:4], raw[4:6], raw[6:])
		}
	}
	return raw
}

func pad2(s string) string {
	if len(s) == 1 {
		return "0" + s
	}
	return s
}
