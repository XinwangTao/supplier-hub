package repository

import (
	"context"
	"errors"

	"github.com/example/supplier-hub/backend/internal/domain"
)

var ErrNotFound = errors.New("supplier not found")

// SupplierRepository 是数据访问边界。当前实现为内存演示数据，
// 生产环境可替换为 PostgreSQL/sqlc，或包装企查查、天眼查等企业征信 API 的适配器。
type SupplierRepository interface {
	Search(ctx context.Context, filters domain.SearchFilters) ([]domain.Supplier, error)
	GetByID(ctx context.Context, id string) (domain.Supplier, error)
	ListByCapability(ctx context.Context, code string) ([]domain.Supplier, error)
	ListAll(ctx context.Context) ([]domain.Supplier, error)
	// Upsert 写入或更新一家供应商。SRM 建档与企查查回填都走这里，
	// 保证"工商档案"与"业务关系"共用同一个主数据源。
	Upsert(ctx context.Context, supplier domain.Supplier) error
}
