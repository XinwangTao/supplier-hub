package repository

import (
	"context"
	"errors"
	"github.com/example/supplier-hub/backend/internal/domain"
)

var ErrNotFound = errors.New("supplier not found")

type SupplierRepository interface {
	Search(ctx context.Context, filters domain.SearchFilters) ([]domain.Supplier, error)
	GetByID(ctx context.Context, id string) (domain.Supplier, error)
}
