package ingest

import "testing"

func TestLoadCSV(t *testing.T) {
	records, err := LoadCSV("testdata/companies.csv")
	if err != nil {
		t.Fatalf("LoadCSV 失败: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("期望 2 条记录，实际 %d", len(records))
	}
	first := records[0]
	if first.Name != "测试示例精密制造有限公司" {
		t.Errorf("企业名称解析错误: %s", first.Name)
	}
	if first.RegisteredCapital != 1200 {
		t.Errorf("注册资本应解析为 1200 万，实际 %d", first.RegisteredCapital)
	}
	if first.InsuredCount != 320 {
		t.Errorf("参保人数解析错误: %d", first.InsuredCount)
	}
	if first.EstablishDate != "2015-03-12" {
		t.Errorf("日期归一化错误: %s", first.EstablishDate)
	}
	if records[1].RegisteredCapital != 15000 {
		t.Errorf("1.5 亿元应解析为 15000 万，实际 %d", records[1].RegisteredCapital)
	}
	if records[1].EstablishDate != "2018-01-05" {
		t.Errorf("紧凑日期归一化错误: %s", records[1].EstablishDate)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name  string
		scope string
		want  string
	}{
		{"某电子有限公司", "SMT贴片、PCBA组装", "pcba"},
		{"某精密有限公司", "塑胶模具设计与制造", "mold"},
		{"某电池有限公司", "锂离子电芯及电池PACK的研发生产", "battery"},
		{"某贸易有限公司", "日用百货销售", ""},
	}
	for _, item := range cases {
		got := Classify(item.name, item.scope, 1)
		if item.want == "" {
			if len(got) != 0 {
				t.Errorf("%s 不应归类，实际 %v", item.scope, got)
			}
			continue
		}
		if len(got) == 0 || got[0] != item.want {
			t.Errorf("%s 期望 %s，实际 %v", item.scope, item.want, got)
		}
	}
}

func TestShortName(t *testing.T) {
	cases := map[string]string{
		"东莞市精锐精密制造有限公司": "精锐精密制造",
		"深圳市华芯电子科技有限公司": "华芯电子科技",
		"北京某某科技有限公司":    "某某科技",
	}
	for input, want := range cases {
		if got := ShortName(input); got != want {
			t.Errorf("ShortName(%s) = %s，期望 %s", input, got, want)
		}
	}
}

func TestToSupplierKeepsBlanks(t *testing.T) {
	supplier := ToSupplier(Record{Name: "测试示例贸易有限公司", BusinessScope: "日用百货销售"}, "id-1")
	if supplier.LegalRepresentative != "" {
		t.Errorf("缺失字段应留空，实际 %q", supplier.LegalRepresentative)
	}
	if supplier.DataSource != "" {
		t.Errorf("未提供来源时不应编造来源，实际 %q", supplier.DataSource)
	}
	if supplier.RiskLevel != "medium" {
		t.Errorf("经营状态未知应标为中风险，实际 %s", supplier.RiskLevel)
	}
}
