package repository

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/example/supplier-hub/backend/internal/domain"
	"github.com/example/supplier-hub/backend/internal/ingest"
	"github.com/example/supplier-hub/backend/internal/provider/qcc"
)

// DatasetMeta 描述当前这份供应商数据是哪里来的、是不是真实数据。
// 前端拿它决定是否展示"未接入真实数据源"的警示条。
type DatasetMeta struct {
	Mode     string   `json:"mode"` // csv / qcc / mock / none
	IsReal   bool     `json:"is_real"`
	Count    int      `json:"count"`
	Source   string   `json:"source"`
	Message  string   `json:"message"`
	LoadedAt string   `json:"loaded_at"`
	Skipped  []string `json:"skipped,omitempty"`
}

// DatasetConfig 是装配数据集的全部入参，全部来自环境变量。
type DatasetConfig struct {
	Mode      string // SUPPLIER_SOURCE: csv / qcc / mock / auto
	CSVPath   string // SUPPLIER_CSV
	NamesPath string // SUPPLIER_NAMES：一行一个企业全称，qcc 模式下按名查询
	CachePath string // SUPPLIER_CACHE：qcc 结果缓存文件
	CacheTTL  time.Duration
	QCCKey    string
	QCCSecret string
}

// BuildDataset 按配置装配真实供应商数据。
// 除显式 SUPPLIER_SOURCE=mock 外，任何情况下都不会生成虚构数据：
// 拿不到数据就返回空集 + 明确的原因，让界面如实告诉用户"还没接上真实数据源"。
func BuildDataset(ctx context.Context, cfg DatasetConfig) ([]domain.Supplier, DatasetMeta, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" || mode == "auto" {
		switch {
		case strings.TrimSpace(cfg.CSVPath) != "":
			mode = "csv"
		case strings.TrimSpace(cfg.QCCKey) != "" && strings.TrimSpace(cfg.QCCSecret) != "":
			mode = "qcc"
		default:
			mode = "none"
		}
	}

	switch mode {
	case "mock":
		return MockSuppliers(), DatasetMeta{
			Mode: "mock", IsReal: false, Count: len(MockSuppliers()),
			Source:  "内置生成器",
			Message: "当前是开发用演示数据，法人、注册资本等均为程序生成，不可用于对外展示或商务决策。",
		}, nil
	case "csv":
		return buildFromCSV(cfg)
	case "qcc":
		return buildFromQCC(ctx, cfg)
	case "none":
		return []domain.Supplier{}, DatasetMeta{
			Mode: "none", IsReal: false, Source: "",
			Message: "尚未接入真实数据源。请导入企查查/天眼查导出的企业清单（SUPPLIER_CSV），或配置企查查开放平台密钥（QCC_KEY / QCC_SECRET_KEY）。",
		}, nil
	}
	return nil, DatasetMeta{}, fmt.Errorf("不支持的 SUPPLIER_SOURCE: %s", mode)
}

func buildFromCSV(cfg DatasetConfig) ([]domain.Supplier, DatasetMeta, error) {
	records, err := ingest.LoadCSV(cfg.CSVPath)
	if err != nil {
		return nil, DatasetMeta{}, err
	}
	suppliers, skipped := materialize(records)
	meta := DatasetMeta{
		Mode: "csv", IsReal: true, Count: len(suppliers),
		Source: filepath.Base(cfg.CSVPath), LoadedAt: time.Now().Format(time.RFC3339),
		Message: fmt.Sprintf("已从 %s 导入 %d 家企业，字段原样保留。", filepath.Base(cfg.CSVPath), len(suppliers)),
		Skipped: skipped,
	}
	if meta.Source == "" {
		meta.Source = "企业工商信息导入表"
	}
	return suppliers, meta, nil
}

func buildFromQCC(ctx context.Context, cfg DatasetConfig) ([]domain.Supplier, DatasetMeta, error) {
	if cached, ok := loadCache(cfg.CachePath, cfg.CacheTTL); ok {
		suppliers, skipped := materialize(cached.Records)
		return suppliers, DatasetMeta{
			Mode: "qcc", IsReal: true, Count: len(suppliers), Source: "企查查开放平台（缓存）",
			LoadedAt: cached.FetchedAt, Skipped: skipped,
			Message: fmt.Sprintf("使用 %s 拉取的企查查数据缓存，共 %d 家。", cached.FetchedAt, len(suppliers)),
		}, nil
	}

	names, err := readNames(cfg.NamesPath)
	if err != nil {
		return nil, DatasetMeta{}, err
	}
	if len(names) == 0 {
		return nil, DatasetMeta{}, fmt.Errorf("QCC 模式需要在 SUPPLIER_NAMES 指定的文件里提供企业全称，每行一个")
	}

	client := qcc.New(cfg.QCCKey, cfg.QCCSecret)
	records := make([]ingest.Record, 0, len(names))
	skipped := make([]string, 0)
	for _, name := range names {
		record, err := client.GetByName(ctx, name)
		if err != nil || strings.TrimSpace(record.Name) == "" {
			skipped = append(skipped, name+": "+errString(err))
			continue
		}
		records = append(records, record)
		time.Sleep(120 * time.Millisecond) // 控制调用频率，避免触发平台限流
	}
	if len(records) == 0 {
		return nil, DatasetMeta{}, fmt.Errorf("企查查没有返回任何可用记录，请检查密钥与调用配额")
	}
	saveCache(cfg.CachePath, records)

	suppliers, skippedRows := materialize(records)
	return suppliers, DatasetMeta{
		Mode: "qcc", IsReal: true, Count: len(suppliers), Source: "企查查开放平台",
		LoadedAt: time.Now().Format(time.RFC3339),
		Message:  fmt.Sprintf("已从企查查拉取 %d 家企业工商信息。", len(suppliers)),
		Skipped:  append(skipped, skippedRows...),
	}, nil
}

// materialize 去重 + 生成稳定 ID。ID 优先用统一社会信用代码——
// 它是企业的法定唯一标识，比自增编号更适合做深链与跨表关联。
func materialize(records []ingest.Record) ([]domain.Supplier, []string) {
	seen := map[string]bool{}
	suppliers := make([]domain.Supplier, 0, len(records))
	skipped := make([]string, 0)
	for _, record := range records {
		name := strings.TrimSpace(record.Name)
		if name == "" {
			continue
		}
		key := strings.TrimSpace(record.CreditCode)
		if key == "" {
			key = "name:" + name
		}
		if seen[key] {
			skipped = append(skipped, name+": 重复记录已跳过")
			continue
		}
		seen[key] = true

		id := strings.TrimSpace(record.CreditCode)
		if id == "" {
			id = "src-" + shortHash(name)
			skipped = append(skipped, name+": 缺少统一社会信用代码，使用名称哈希作为 ID")
		}
		suppliers = append(suppliers, ingest.ToSupplier(record, id))
	}
	return suppliers, skipped
}

func shortHash(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])[:12]
}

func readNames(path string) ([]string, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取企业名单失败: %w", err)
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	names := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		names = append(names, trimmed)
	}
	return names, nil
}

type cacheFile struct {
	FetchedAt string          `json:"fetched_at"`
	Records   []ingest.Record `json:"records"`
}

func loadCache(path string, ttl time.Duration) (cacheFile, bool) {
	if strings.TrimSpace(path) == "" {
		return cacheFile{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cacheFile{}, false
	}
	var cache cacheFile
	if err := json.Unmarshal(data, &cache); err != nil || len(cache.Records) == 0 {
		return cacheFile{}, false
	}
	if ttl > 0 {
		fetchedAt, err := time.Parse(time.RFC3339, cache.FetchedAt)
		if err != nil || time.Since(fetchedAt) > ttl {
			return cacheFile{}, false
		}
	}
	return cache, true
}

func saveCache(path string, records []ingest.Record) {
	if strings.TrimSpace(path) == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	payload, err := json.MarshalIndent(cacheFile{FetchedAt: time.Now().Format(time.RFC3339), Records: records}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, payload, 0o644)
}

func errString(err error) string {
	if err == nil {
		return "接口未返回数据"
	}
	return err.Error()
}
