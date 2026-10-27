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
		result = append(result, s)
	}
	return result, nil
}

func (r *MemorySupplierRepository) GetByID(_ context.Context, id string) (domain.Supplier, error) {
	for _, s := range r.suppliers {
		if s.ID == id {
			return s, nil
		}
	}
	return domain.Supplier{}, fmt.Errorf("%w: %s", ErrNotFound, id)
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
