package config

import (
	"os"
	"strconv"
	"time"

	"github.com/example/supplier-hub/backend/internal/repository"
)

type Config struct {
	HTTPAddr    string
	CORSOrigins string

	// ---- 真实数据源 ----
	// SupplierSource: csv（导入工商信息表）/ qcc（企查查开放平台）/ mock（仅开发）/ auto
	SupplierSource string
	SupplierCSV    string
	SupplierNames  string
	SupplierCache  string
	CacheTTL       time.Duration
	QCCKey         string
	QCCSecret      string
}

func Load() Config {
	addr := env("HTTP_ADDR", ":8080")
	origins := env("CORS_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000")

	hours := 24
	if raw := os.Getenv("SUPPLIER_CACHE_TTL_HOURS"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value >= 0 {
			hours = value
		}
	}

	return Config{
		HTTPAddr:       addr,
		CORSOrigins:    origins,
		SupplierSource: env("SUPPLIER_SOURCE", "auto"),
		SupplierCSV:    os.Getenv("SUPPLIER_CSV"),
		SupplierNames:  os.Getenv("SUPPLIER_NAMES"),
		SupplierCache:  env("SUPPLIER_CACHE", "data/cache/qcc.json"),
		CacheTTL:       time.Duration(hours) * time.Hour,
		QCCKey:         os.Getenv("QCC_KEY"),
		QCCSecret:      os.Getenv("QCC_SECRET_KEY"),
	}
}

// Dataset 把配置转成数据装配参数。
func (c Config) Dataset() repository.DatasetConfig {
	return repository.DatasetConfig{
		Mode:      c.SupplierSource,
		CSVPath:   c.SupplierCSV,
		NamesPath: c.SupplierNames,
		CachePath: c.SupplierCache,
		CacheTTL:  c.CacheTTL,
		QCCKey:    c.QCCKey,
		QCCSecret: c.QCCSecret,
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
