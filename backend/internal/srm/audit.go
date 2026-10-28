package srm

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/example/supplier-hub/backend/internal/domain"
)

// 审计留痕。
//
// 为什么必须做：QCDST 每个分数都追到某张单据，但"单据本身可以被改"。
// IQC 判退改成让步接收、到货日期回填成承诺日期前——改了没人知道，
// 评级就成了可以事后粉饰的数字。审计不阻止修改，但让修改有痕迹、可追溯。
//
// 这里刻意把"被拒绝的写入"也记一条：谁在反复尝试给没有合作关系的供应商
// 录单，本身就是一条值得看的风险信号。

type auditLog struct {
	mu      sync.RWMutex
	entries []domain.AuditEntry
	seq     int
}

func (s *Store) Audit(action, tenantID, actor, object, summary, source string) {
	s.auditMu.Lock()
	defer s.auditMu.Unlock()
	s.auditSeq++
	s.auditEntries = append(s.auditEntries, domain.AuditEntry{
		ID:       fmt.Sprintf("a-%d-%d", time.Now().UnixMilli(), s.auditSeq),
		At:       time.Now().Format("2006-01-02 15:04:05"),
		TenantID: tenantID,
		Actor:    orDefault(actor, "system"),
		Action:   action,
		Object:   object,
		Summary:  summary,
		Source:   orDefault(source, "api"),
	})
}

// auditReject 记录一次被校验拦下的写入。
func (s *Store) AuditReject(action, tenantID, actor, object, why string) {
	s.auditMu.Lock()
	defer s.auditMu.Unlock()
	s.auditSeq++
	s.auditEntries = append(s.auditEntries, domain.AuditEntry{
		ID:         fmt.Sprintf("a-%d-%d", time.Now().UnixMilli(), s.auditSeq),
		At:         time.Now().Format("2006-01-02 15:04:05"),
		TenantID:   tenantID,
		Actor:      orDefault(actor, "system"),
		Action:     action,
		Object:     object,
		Summary:    "写入被拒绝",
		Source:     "api",
		Rejected:   true,
		RejectWhy:  why,
	})
}

// AuditEntries 返回审计流水，最新的在前。
func (s *Store) AuditEntries(tenantID string, limit int) []domain.AuditEntry {
	s.auditMu.RLock()
	defer s.auditMu.RUnlock()
	out := make([]domain.AuditEntry, 0, len(s.auditEntries))
	for i := len(s.auditEntries) - 1; i >= 0; i-- {
		e := s.auditEntries[i]
		if tenantID != "" && e.TenantID != tenantID {
			continue
		}
		out = append(out, e)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// AuditStats 审计汇总：写入量、被拒量、最近写入时间。
// 被拒量突然升高通常意味着有人在批量试探边界。
func (s *Store) AuditStats(tenantID string) (total, rejected int, lastAt string) {
	s.auditMu.RLock()
	defer s.auditMu.RUnlock()
	for _, e := range s.auditEntries {
		if tenantID != "" && e.TenantID != tenantID {
			continue
		}
		total++
		if e.Rejected {
			rejected++
		}
		if e.At > lastAt {
			lastAt = e.At
		}
	}
	// 按时间倒序取第一条即可；这里 entries 是追加序，等价于时间序。
	if len(s.auditEntries) > 0 && lastAt == "" {
		lastAt = s.auditEntries[len(s.auditEntries)-1].At
	}
	return
}

// ClearAudit 清空审计流水（仅在清演示数据时调用）。
func (s *Store) ClearAudit() {
	s.auditMu.Lock()
	defer s.auditMu.Unlock()
	s.auditEntries = nil
	s.auditSeq = 0
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

// sortedActors 统计出现过的操作人，供界面做筛选。
func sortedActors(entries []domain.AuditEntry) []string {
	seen := map[string]bool{}
	for _, e := range entries {
		seen[e.Actor] = true
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
