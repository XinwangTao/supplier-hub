package service

import (
	"github.com/example/supplier-hub/backend/internal/domain"
	"testing"
)

func TestParseQuery(t *testing.T) {
	f := ParseQuery("找东莞注册资本500万以上、员工100人以上的精密注塑生产厂家")
	if f.City != "东莞市" || f.MinCapital != 500 || f.MinEmployees != 100 || f.BusinessKeyword != "注塑" || f.ManufacturerOnly == nil {
		t.Fatalf("unexpected filters: %+v", f)
	}
}

func TestParseQueryRecognizesCityAlias(t *testing.T) {
	if got := ParseQuery("找苏州自动化设备制造商").City; got != "苏州市" {
		t.Fatalf("expected 苏州市, got %q", got)
	}
}

func TestScoreSuppliersSortsMatch(t *testing.T) {
	yes := true
	items := scoreSuppliers([]domain.Supplier{{ID: "a", City: "东莞市", IsManufacturer: true, RiskLevel: domain.RiskLow, EmployeeCount: 500}, {ID: "b", City: "深圳市", RiskLevel: domain.RiskHigh}}, domain.SearchFilters{City: "东莞市", ManufacturerOnly: &yes})
	if items[0].ID != "a" || items[0].MatchScore <= items[1].MatchScore {
		t.Fatalf("unexpected order: %+v", items)
	}
}
