// Package qcc 是企查查开放平台的适配器：按企业名称或统一社会信用代码拉取真实工商信息。
//
// 只读、只透传：接口返回什么就存什么，字段缺失时留空并记账，不做任何补全或推测。
// 配置方式见 README：QCC_KEY / QCC_SECRET_KEY。
package qcc

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/example/supplier-hub/backend/internal/ingest"
)

const defaultBaseURL = "https://api.qichacha.com"

// Client 是企查查开放平台客户端。
type Client struct {
	Key       string
	SecretKey string
	BaseURL   string
	HTTP      *http.Client
}

func New(key, secret string) *Client {
	return &Client{
		Key:       key,
		SecretKey: secret,
		BaseURL:   defaultBaseURL,
		HTTP:      &http.Client{Timeout: 15 * time.Second},
	}
}

// Search 按关键词搜索企业，返回一批基础记录。
func (c *Client) Search(ctx context.Context, keyword string, size int) ([]ingest.Record, error) {
	if size <= 0 || size > 20 {
		size = 10
	}
	payload, err := c.do(ctx, "/ECIV4/Search", url.Values{
		"keyword":   {keyword},
		"pageIndex": {"1"},
		"pageSize":  {strconv.Itoa(size)},
	})
	if err != nil {
		return nil, err
	}
	items := asList(payload)
	records := make([]ingest.Record, 0, len(items))
	for _, item := range items {
		records = append(records, toRecord(item))
	}
	return records, nil
}

// GetByName 按企业全称取一条完整工商记录（含法人、注册资本、经营范围）。
func (c *Client) GetByName(ctx context.Context, name string) (ingest.Record, error) {
	payload, err := c.do(ctx, "/ECIV4/GetBasicDetailsByName", url.Values{"keyword": {name}})
	if err != nil {
		return ingest.Record{}, err
	}
	obj := asObject(payload)
	if len(obj) == 0 {
		return ingest.Record{}, fmt.Errorf("企查查未返回「%s」的工商信息", name)
	}
	return toRecord(obj), nil
}

// GetByCreditCode 按统一社会信用代码取记录，用于核验与去重。
func (c *Client) GetByCreditCode(ctx context.Context, code string) (ingest.Record, error) {
	payload, err := c.do(ctx, "/ECIV4/GetBasicDetailsByCreditCode", url.Values{"creditCode": {code}})
	if err != nil {
		return ingest.Record{}, err
	}
	obj := asObject(payload)
	if len(obj) == 0 {
		return ingest.Record{}, fmt.Errorf("企查查未返回信用代码 %s 的工商信息", code)
	}
	return toRecord(obj), nil
}

func (c *Client) do(ctx context.Context, path string, params url.Values) (interface{}, error) {
	if c.Key == "" || c.SecretKey == "" {
		return nil, fmt.Errorf("未配置企查查密钥（QCC_KEY / QCC_SECRET_KEY）")
	}
	timespan := strconv.FormatInt(time.Now().Unix(), 10)
	sum := md5.Sum([]byte(c.Key + timespan + c.SecretKey))
	params.Set("key", c.Key)
	params.Set("token", hex.EncodeToString(sum[:]))

	endpoint := strings.TrimSuffix(c.BaseURL, "/") + path + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求企查查失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("企查查返回 HTTP %d", resp.StatusCode)
	}

	var envelope struct {
		Status  string          `json:"Status"`
		Message string          `json:"Message"`
		Result  json.RawMessage `json:"Result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("解析企查查响应失败: %w", err)
	}
	if envelope.Status != "" && envelope.Status != "200" {
		return nil, fmt.Errorf("企查查接口报错: %s", strings.TrimSpace(envelope.Message))
	}
	if len(envelope.Result) == 0 {
		return nil, fmt.Errorf("企查查返回空结果")
	}
	var out interface{}
	if err := json.Unmarshal(envelope.Result, &out); err != nil {
		return nil, fmt.Errorf("解析 Result 失败: %w", err)
	}
	return out, nil
}

func asObject(payload interface{}) map[string]interface{} {
	switch value := payload.(type) {
	case map[string]interface{}:
		return value
	case []interface{}:
		if len(value) > 0 {
			if obj, ok := value[0].(map[string]interface{}); ok {
				return obj
			}
		}
	}
	return nil
}

func asList(payload interface{}) []map[string]interface{} {
	switch value := payload.(type) {
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(value))
		for _, item := range value {
			if obj, ok := item.(map[string]interface{}); ok {
				out = append(out, obj)
			}
		}
		return out
	case map[string]interface{}:
		for _, key := range []string{"Result", "Data", "List", "Items"} {
			if nested, ok := value[key]; ok {
				if list := asList(nested); len(list) > 0 {
					return list
				}
			}
		}
		return []map[string]interface{}{value}
	}
	return nil
}

// toRecord 把企查查的字段翻译成内部记录。
// 各版本接口的字段名略有差异，这里按常见别名依次尝试，取不到就留空。
func toRecord(obj map[string]interface{}) ingest.Record {
	rec := ingest.Record{
		Name:                str(obj, "Name", "name", "CompanyName"),
		CreditCode:          str(obj, "CreditCode", "creditCode", "TaxNo"),
		LegalRepresentative: str(obj, "OperName", "operName", "LegalPersonName", "LegalPerson"),
		LegalRepTitle:       str(obj, "OperTitle", "legalPersonTitle"),
		BusinessStatus:      str(obj, "Status", "status", "RegistStatus"),
		EnterpriseType:      str(obj, "EconKind", "econKind", "CompanyType"),
		RegisteredAddress:   str(obj, "Address", "address", "RegistAddress"),
		EstablishDate:       normalizeDate(str(obj, "StartDate", "startDate", "EsDate", "FoundDate")),
		RegisteredCapital:   ingest.ParseCapital(str(obj, "RegistCapi", "registCapi", "RegistCapital")),
		TaxRating:           str(obj, "TaxCreditLevel", "taxCreditLevel"),
		InsuredCount:        ingest.ParseCount(str(obj, "SocialStaffNum", "socialStaffNum", "InsuredCount")),
		EmployeeCount:       ingest.ParseCount(str(obj, "StaffNum", "staffNum", "EmployeeCount")),
		BusinessScope:       str(obj, "Scope", "scope", "BusinessScope"),
		Industry:            str(obj, "Industry", "industry", "IndustryName"),
		Province:            str(obj, "Province", "province"),
		City:                str(obj, "City", "city"),
		District:            str(obj, "District", "district", "AreaCode"),
		ContactPhone:        str(obj, "Tel", "tel", "PhoneNumber"),
		ContactEmail:        str(obj, "Email", "email"),
		Website:             str(obj, "WebSite", "website", "WebSiteList"),
		Source:              "企查查开放平台",
		VerifiedAt:          time.Now().Format(time.RFC3339),
	}
	if keyNo := str(obj, "KeyNo", "keyNo"); keyNo != "" {
		rec.SourceURL = "https://www.qcc.com/firm/" + keyNo + ".html"
	}
	return rec
}

func str(obj map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, ok := obj[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		case float64:
			return strconv.FormatFloat(typed, 'f', -1, 64)
		case bool:
			if typed {
				return "是"
			}
		}
	}
	return ""
}

func normalizeDate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	for _, layout := range []string{"2006-01-02", "2006/01/02", "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return raw
}
