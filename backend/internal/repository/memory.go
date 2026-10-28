package repository

import (
	"context"
	"fmt"
	"github.com/example/supplier-hub/backend/internal/domain"
	"strings"
)

type MemorySupplierRepository struct{ suppliers []domain.Supplier }

func NewMemorySupplierRepository(suppliers []domain.Supplier) *MemorySupplierRepository {
	return &MemorySupplierRepository{suppliers: suppliers}
}

func (r *MemorySupplierRepository) Search(_ context.Context, f domain.SearchFilters) ([]domain.Supplier, error) {
	result := make([]domain.Supplier, 0, len(r.suppliers))
	for _, s := range r.suppliers {
		if f.BusinessKeyword != "" && !strings.Contains(strings.ToLower(s.BusinessScope+" "+strings.Join(s.MainProducts, " ")), strings.ToLower(f.BusinessKeyword)) {
			continue
		}
		if f.Province != "" && s.Province != f.Province {
			continue
		}
		if f.City != "" && s.City != f.City {
			continue
		}
		if f.MinCapital > 0 && s.RegisteredCapital < f.MinCapital {
			continue
		}
		if f.MaxCapital > 0 && s.RegisteredCapital > f.MaxCapital {
			continue
		}
		if f.MinEmployees > 0 && s.EmployeeCount < f.MinEmployees {
			continue
		}
		if f.MaxEmployees > 0 && s.EmployeeCount > f.MaxEmployees {
			continue
		}
		if f.ManufacturerOnly != nil && s.IsManufacturer != *f.ManufacturerOnly {
			continue
		}
		if f.MaxRisk != "" && riskRank(s.RiskLevel) > riskRank(f.MaxRisk) {
			continue
		}
		if f.Capability != "" && !hasCapability(s, f.Capability) {
			continue
		}
		result = append(result, s)
	}
	return result, nil
}

func hasCapability(s domain.Supplier, code string) bool {
	for _, c := range s.Capabilities {
		if c == code {
			return true
		}
	}
	return false
}

func (r *MemorySupplierRepository) GetByID(_ context.Context, id string) (domain.Supplier, error) {
	for _, s := range r.suppliers {
		if s.ID == id {
			return s, nil
		}
	}
	return domain.Supplier{}, fmt.Errorf("%w: %s", ErrNotFound, id)
}

func (r *MemorySupplierRepository) ListByCapability(_ context.Context, code string) ([]domain.Supplier, error) {
	result := make([]domain.Supplier, 0, 8)
	for _, s := range r.suppliers {
		for _, c := range s.Capabilities {
			if c == code {
				result = append(result, s)
				break
			}
		}
	}
	return result, nil
}

func (r *MemorySupplierRepository) ListAll(_ context.Context) ([]domain.Supplier, error) {
	return r.suppliers, nil
}

func riskRank(level domain.RiskLevel) int {
	switch level {
	case domain.RiskLow:
		return 1
	case domain.RiskMedium:
		return 2
	case domain.RiskHigh:
		return 3
	case domain.RiskCritical:
		return 4
	}
	return 99
}

// Upsert 按 ID 覆盖写入，不存在则追加。
func (r *MemorySupplierRepository) Upsert(_ context.Context, supplier domain.Supplier) error {
	if supplier.ID == "" {
		return fmt.Errorf("supplier id is required")
	}
	for i := range r.suppliers {
		if r.suppliers[i].ID == supplier.ID {
			r.suppliers[i] = supplier
			return nil
		}
	}
	r.suppliers = append(r.suppliers, supplier)
	return nil
}
