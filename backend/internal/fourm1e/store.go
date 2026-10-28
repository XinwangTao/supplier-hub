// Package fourm1e 存放供应商五维档案(人机料法环)的录入数据。
// 数据全部来自巡厂人工录入:这里不做任何推断、不从企查查回填、不参与评分。
// 快照追加式保存——修正等于新增一条,旧记录永远可回查(呼应平台的审计留痕原则)。
package fourm1e

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/example/supplier-hub/backend/internal/domain"
)

type Store struct {
	mu      sync.RWMutex
	path    string                      // 持久化文件路径(空 = 纯内存,测试用);巡厂数据是宝贵资产,重启不能丢
	current map[string]domain.FourM1E   // supplierID → 最新一条
	history map[string][]domain.FourM1E // supplierID → 全部快照(旧→新)
}

// NewStore 打开五维档案存储。path 非空时从文件加载历史,并在每次写入后落盘。
// 落盘用临时文件+改名,保证进程中途被杀也不会留下半个坏文件。
func NewStore(path string) *Store {
	s := &Store{path: path, current: map[string]domain.FourM1E{}, history: map[string][]domain.FourM1E{}}
	if path == "" {
		return s
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return s // 首次启动没有文件,从空库开始
	}
	var dump struct {
		Current map[string]domain.FourM1E   `json:"current"`
		History map[string][]domain.FourM1E `json:"history"`
	}
	if json.Unmarshal(raw, &dump) == nil {
		if dump.Current != nil {
			s.current = dump.Current
		}
		if dump.History != nil {
			s.history = dump.History
		}
	}
	return s
}

// persist 调用方必须已持有写锁。
func (s *Store) persist() {
	if s.path == "" {
		return
	}
	dump := struct {
		Current map[string]domain.FourM1E   `json:"current"`
		History map[string][]domain.FourM1E `json:"history"`
	}{s.current, s.history}
	raw, err := json.MarshalIndent(dump, "", " ")
	if err != nil {
		return
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, s.path)
}

// Current 返回某供应商当前生效的五维档案;没有人工录入过时 ok=false,调用方显示「—」。
func (s *Store) Current(supplierID string) (domain.FourM1E, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.current[supplierID]
	return rec, ok
}

// History 按录入时间旧→新返回全部快照。
func (s *Store) History(supplierID string) []domain.FourM1E {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.FourM1E, len(s.history[supplierID]))
	copy(out, s.history[supplierID])
	return out
}

// Append 保存一条新快照:盖写 current、追加 history。调用前必须先通过 Validate。
func (s *Store) Append(rec domain.FourM1E) domain.FourM1E {
	if rec.RecordedAt == "" {
		rec.RecordedAt = time.Now().Format(time.RFC3339)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current[rec.SupplierID] = rec
	s.history[rec.SupplierID] = append(s.history[rec.SupplierID], rec)
	s.persist()
	return rec
}

// Validate 录入校验(工作流节点④):计数非负、比例 0–100。
// 只挡非法数据,不挡"没填"——缺失字段留空是常态,不是错误。
func Validate(rec *domain.FourM1E) error {
	if rec.SupplierID == "" {
		return fmt.Errorf("缺少供应商 ID")
	}
	negatives := []struct {
		label string
		value int
	}{
		{"员工总数", rec.EmployeeTotal},
		{"研发人员数", rec.RnDCount},
		{"厂房面积", rec.FactoryArea},
		{"产线数", rec.ProductionLines},
		{"自动化率", rec.AutomationRate},
		{"产能利用率", rec.CapacityUtilization},
	}
	for _, item := range negatives {
		if item.value < 0 {
			return fmt.Errorf("%s 不能为负数", item.label)
		}
	}
	for _, item := range negatives[4:6] {
		if item.value > 100 {
			return fmt.Errorf("%s 超过 100%%", item.label)
		}
	}
	for i, dept := range rec.Departments {
		if dept.Name == "" {
			return fmt.Errorf("第 %d 个部门未填名称", i+1)
		}
		if dept.Count < 0 {
			return fmt.Errorf("部门「%s」人数不能为负数", dept.Name)
		}
	}
	for i, m := range rec.Machines {
		if m.Name == "" {
			return fmt.Errorf("第 %d 台设备未填名称", i+1)
		}
		if m.Qty < 0 {
			return fmt.Errorf("设备「%s」数量不能为负数", m.Name)
		}
		if m.AgeYear < 0 {
			return fmt.Errorf("设备「%s」役龄不能为负数", m.Name)
		}
	}
	return nil
}

// SummarizeHistory 给档案页的一句话概览:录入过几次、最近一次谁录的。
func (s *Store) SummarizeHistory(supplierID string) (count int, lastBy string, lastAt string) {
	h := s.History(supplierID)
	if len(h) == 0 {
		return 0, "", ""
	}
	// 按时间兜底排序,避免时钟偏移导致"最新"错位
	sort.SliceStable(h, func(i, j int) bool { return h[i].RecordedAt < h[j].RecordedAt })
	last := h[len(h)-1]
	return len(h), last.RecordedBy, last.RecordedAt
}
