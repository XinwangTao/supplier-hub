// Package srm 是轻量 SRM 的数据层：采购方与供应商的合作关系、订单、交付批次、
// 质量事件、报价、服务工单与现场评审。
//
// 这里的所有单据都是"业务发生时才产生"的真实记录，QCDST 的每一个分数都能回溯到
// 这里的某一行。演示数据单独标记为 Demo，并在信用代码上带 DEMO 前缀，避免被误认成真实企业。
package srm

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/example/supplier-hub/backend/internal/domain"
	"github.com/example/supplier-hub/backend/internal/qcdst"
)

type Store struct {
	mu          sync.RWMutex
	tenants     map[string]domain.Tenant
	engagements []domain.Engagement
	orders      []domain.PurchaseOrder
	deliveries  []domain.Delivery
	events      []domain.QualityEvent
	quotes      []domain.Quote
	tickets     []domain.ServiceTicket
	reviews     []domain.TechReview
	demo        bool

	// 审计留痕独立加锁：单据写入频繁，不该被审计写放大成整库锁。
	auditMu      sync.RWMutex
	auditEntries []domain.AuditEntry
	auditSeq     int
}

func NewStore() *Store {
	return &Store{tenants: map[string]domain.Tenant{}}
}

func (s *Store) Demo() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.demo
}

func (s *Store) Tenants() []domain.Tenant {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Tenant, 0, len(s.tenants))
	keys := make([]string, 0, len(s.tenants))
	for k := range s.tenants {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, s.tenants[k])
	}
	return out
}

func (s *Store) Tenant(id string) (domain.Tenant, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tenants[id]
	return t, ok
}

// Engagements 返回某采购方的合作关系；tenantID 为空时返回全部（平台视角）。
func (s *Store) Engagements(tenantID string) []domain.Engagement {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Engagement, 0, len(s.engagements))
	for _, e := range s.engagements {
		if tenantID == "" || e.TenantID == tenantID {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SupplierID < out[j].SupplierID })
	return out
}

// SupplierIDs 返回库里有 SRM 关系的供应商 ID（去重）。
func (s *Store) SupplierIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seen := map[string]bool{}
	for _, e := range s.engagements {
		seen[e.SupplierID] = true
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Bundle 组装一家供应商在某个采购方下的全部单据。
func (s *Store) Bundle(tenantID, supplierID string) qcdst.Bundle {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b := qcdst.Bundle{Tenant: s.tenants[tenantID], Demo: s.demo}
	for _, o := range s.orders {
		if o.SupplierID == supplierID && (tenantID == "" || o.TenantID == tenantID) {
			b.Orders = append(b.Orders, o)
		}
	}
	for _, d := range s.deliveries {
		if d.SupplierID == supplierID && (tenantID == "" || d.TenantID == tenantID) {
			b.Deliveries = append(b.Deliveries, d)
		}
	}
	for _, e := range s.events {
		if e.SupplierID == supplierID && (tenantID == "" || e.TenantID == tenantID) {
			b.Events = append(b.Events, e)
		}
	}
	for _, q := range s.quotes {
		if q.SupplierID == supplierID && (tenantID == "" || q.TenantID == tenantID) {
			b.Quotes = append(b.Quotes, q)
		}
	}
	for _, t := range s.tickets {
		if t.SupplierID == supplierID && (tenantID == "" || t.TenantID == tenantID) {
			b.Tickets = append(b.Tickets, t)
		}
	}
	for _, r := range s.reviews {
		if r.SupplierID == supplierID && (tenantID == "" || r.TenantID == tenantID) {
			b.Reviews = append(b.Reviews, r)
		}
	}
	return b
}

// ---------------- 写入 ----------------

func (s *Store) AddEngagement(e domain.Engagement) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.engagements = append(s.engagements, e)
}

// EngagementOf 查某采购方与某供应商的合作关系（去重引入用）。
func (s *Store) EngagementOf(tenantID, supplierID string) (domain.Engagement, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, e := range s.engagements {
		if e.TenantID == tenantID && e.SupplierID == supplierID {
			return e, true
		}
	}
	return domain.Engagement{}, false
}

// UpsertEngagement 已存在则更新状态与字段，不存在则新建。
// 返回是否是新建——调用方据此区分"引入成功"与"状态变更"。
func (s *Store) UpsertEngagement(e domain.Engagement) (domain.Engagement, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, old := range s.engagements {
		if old.TenantID == e.TenantID && old.SupplierID == e.SupplierID {
			if e.Status != "" {
				s.engagements[i].Status = e.Status
			}
			if e.Category != "" {
				s.engagements[i].Category = e.Category
			}
			if e.BuyerName != "" {
				s.engagements[i].BuyerName = e.BuyerName
			}
			if e.Note != "" {
				s.engagements[i].Note = e.Note
			}
			if e.Source != "" {
				s.engagements[i].Source = e.Source
			}
			if e.AnnualSpend > 0 {
				s.engagements[i].AnnualSpend = e.AnnualSpend
			}
			return s.engagements[i], false
		}
	}
	s.engagements = append(s.engagements, e)
	return e, true
}

// SetEngagementStatus 只改状态。状态机由 service 层校验合法性。
func (s *Store) SetEngagementStatus(tenantID, supplierID, status string) (domain.Engagement, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.engagements {
		if e.TenantID == tenantID && e.SupplierID == supplierID {
			s.engagements[i].Status = status
			return s.engagements[i], true
		}
	}
	return domain.Engagement{}, false
}

// EnsureTenant 没有该企业时登记一个，返回实际使用的 tenant。
// 真实场景里企业是注册进来的；这里是为了让"第一次点引入"不至于因为
// 没有租户而失败——但不会凭空造数据，只登记身份。
func (s *Store) EnsureTenant(t domain.Tenant) domain.Tenant {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.ID == "" {
		t.ID = "t-self"
	}
	if old, ok := s.tenants[t.ID]; ok {
		return old
	}
	if t.Name == "" {
		t.Name = "本企业"
	}
	if t.Since == "" {
		t.Since = time.Now().Format("2006-01")
	}
	s.tenants[t.ID] = t
	return t
}

// Orders 返回某采购方（或全部）下某供应商的订单。
func (s *Store) Orders(tenantID, supplierID string) []domain.PurchaseOrder {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []domain.PurchaseOrder{}
	for _, o := range s.orders {
		if o.SupplierID == supplierID && (tenantID == "" || o.TenantID == tenantID) {
			out = append(out, o)
		}
	}
	return out
}

// HasOrder 同采购方下同供应商是否已存在这个订单号（或同 ID）。
// 重复单据必须拦：否则把一个批次 POST 15 遍就能凑够评级样本量。
func (s *Store) HasOrder(tenantID, supplierID, poNo, id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, o := range s.orders {
		if o.TenantID != tenantID || o.SupplierID != supplierID {
			continue
		}
		if (poNo != "" && o.PONo == poNo) || (id != "" && o.ID == id) {
			return true
		}
	}
	return false
}

// HasDelivery 同采购方下同供应商是否已存在这个批次号（或同 ID）。
func (s *Store) HasDelivery(tenantID, supplierID, batchNo, id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, d := range s.deliveries {
		if d.TenantID != tenantID || d.SupplierID != supplierID {
			continue
		}
		if (batchNo != "" && d.BatchNo == batchNo) || (id != "" && d.ID == id) {
			return true
		}
	}
	return false
}

func (s *Store) AddOrder(o domain.PurchaseOrder) domain.PurchaseOrder {
	s.mu.Lock()
	defer s.mu.Unlock()
	if o.ID == "" {
		o.ID = fmt.Sprintf("po-%s-%d", o.TenantID, len(s.orders)+1)
	}
	if o.IssuedAt == "" {
		o.IssuedAt = time.Now().Format("2006-01-02")
	}
	s.orders = append(s.orders, o)
	return o
}

func (s *Store) AddDelivery(d domain.Delivery) domain.Delivery {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d.ID == "" {
		d.ID = fmt.Sprintf("dl-%s-%d", d.TenantID, len(s.deliveries)+1)
	}
	s.deliveries = append(s.deliveries, d)
	return d
}

func (s *Store) AddQualityEvent(e domain.QualityEvent) domain.QualityEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.ID == "" {
		e.ID = fmt.Sprintf("qe-%s-%d", e.TenantID, len(s.events)+1)
	}
	s.events = append(s.events, e)
	return e
}

func (s *Store) AddQuote(q domain.Quote) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if q.ID == "" {
		q.ID = fmt.Sprintf("qt-%s-%d", q.TenantID, len(s.quotes)+1)
	}
	s.quotes = append(s.quotes, q)
}

func (s *Store) AddTicket(t domain.ServiceTicket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.ID == "" {
		t.ID = fmt.Sprintf("st-%s-%d", t.TenantID, len(s.tickets)+1)
	}
	s.tickets = append(s.tickets, t)
}

func (s *Store) AddReview(r domain.TechReview) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.ID == "" {
		r.ID = fmt.Sprintf("tr-%s-%d", r.TenantID, len(s.reviews)+1)
	}
	s.reviews = append(s.reviews, r)
}

// ---------------- 演示数据 ----------------

// demoVendor 是演示用供应商的画像参数。每家刻意给不同的短板，
// 这样评级分布才覆盖 A+ / A / B / C / 数据不足 五档，能验证"评分真的会分化"。
type demoVendor struct {
	name, short, code, city string
	caps                    []string
	otd, passRate, ppm      float64
	responseH, resolveD     float64
	priceIdx, yoy           float64
	coop, coverage          int
	automation, rnd         int
	npiProj, npiSucc        int
	orders                  int
	criticalEvents          int
}

// SeedDemo 生成一套完整的演示 SRM 数据，并把供应商主数据一起返回，
// 由调用方写入供应商库（保证 QCDST 能查到工商档案）。
func (s *Store) SeedDemo() []domain.Supplier {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.demo = true

	tenant := domain.Tenant{
		ID: "t-demo", Name: "启元智能（演示企业）", Industry: "消费电子整机",
		Plan: "enterprise", Since: "2024-06",
	}
	s.tenants[tenant.ID] = tenant
	// 第二家采购方：用来演示"平台级评级"的跨企业汇总。
	tenant2 := domain.Tenant{
		ID: "t-demo-2", Name: "澜图工业（演示企业）", Industry: "工业自动化",
		Plan: "team", Since: "2025-01",
	}
	s.tenants[tenant2.ID] = tenant2

	vendors := []demoVendor{
		{"精睿精密制造（东莞）有限公司", "精睿精密", "DEMO-441900001", "东莞市",
			[]string{"cnc", "diecasting", "anodizing"}, 0.99, 0.998, 120, 2.2, 2.6, 0.93, 0.065, 95, 96, 5, 18, 12, 10, 42, 0},
		{"恒芯电子科技（深圳）有限公司", "恒芯电子", "DEMO-440300002", "深圳市",
			[]string{"pcba", "pcb", "testing"}, 0.96, 0.988, 420, 3.8, 5.5, 0.99, 0.048, 85, 90, 4, 12, 9, 8, 36, 1},
		{"联创显示模组（惠州）有限公司", "联创显示", "DEMO-441300003", "惠州市",
			[]string{"display", "glass", "coating"}, 0.89, 0.972, 890, 8.0, 9.0, 1.04, 0.031, 72, 82, 4, 8, 6, 5, 30, 1},
		{"宏达注塑制品（东莞）有限公司", "宏达注塑", "DEMO-441900004", "东莞市",
			[]string{"injection", "mold"}, 0.91, 0.954, 1500, 13.0, 11.5, 1.11, 0.012, 62, 72, 3, 4, 5, 2, 26, 2},
		{"臻声声学科技（苏州）有限公司", "臻声声学", "DEMO-320500005", "苏州市",
			[]string{"acoustics", "silicone"}, 0.94, 0.981, 620, 5.2, 6.8, 1.02, 0.042, 78, 86, 4, 9, 7, 6, 24, 0},
		{"新锐新能源电池（惠州）有限公司", "新锐电池", "DEMO-441300006", "惠州市",
			[]string{"battery"}, 0.93, 0.99, 300, 4.0, 4.5, 0.97, 0.055, 80, 88, 4, 15, 3, 3, 6, 0},
	}

	suppliers := make([]domain.Supplier, 0, len(vendors))
	rng := rand.New(rand.NewSource(20261027))

	for vi, v := range vendors {
		id := v.code
		caps := v.caps
		names := make([]string, 0, len(caps))
		for _, c := range caps {
			names = append(names, capName(c))
		}
		suppliers = append(suppliers, domain.Supplier{
			ID: id, Name: v.name, ShortName: v.short,
			CreditCode: v.code, City: v.city, Province: "广东省",
			Capabilities: caps, CapabilityNames: names, MainProducts: names,
			BusinessStatus: "存续（演示）", EnterpriseType: "有限责任公司",
			LegalRepresentative: "演示数据", RegisteredCapital: int64(1000 + vi*400),
			EmployeeCount: 120 + vi*40, IsManufacturer: true,
			DataSource: "演示数据（非真实企业）", RiskLevel: domain.RiskLow,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		})

		// ---- 合作与报价 ----
		active := "active"
		if v.orders < 10 {
			active = "active"
		}
		s.engagements = append(s.engagements, domain.Engagement{
			ID: "eng-" + id, TenantID: tenant.ID, SupplierID: id, Status: active,
			Category: names[0], Since: "2025-03", AnnualSpend: float64(1200 + vi*800),
			ShareOfSpend: float64(12 + vi*3), SourcingTypes: []string{"直采", "框架协议"},
			BuyerName: "李工", SupplierContact: "王经理",
		})
		// 第二家采购方只覆盖前四家，制造"买方数量不同"的差异
		if vi < 4 {
			s.engagements = append(s.engagements, domain.Engagement{
				ID: "eng2-" + id, TenantID: tenant2.ID, SupplierID: id, Status: "active",
				Category: names[0], Since: "2025-07", AnnualSpend: float64(600 + vi*300),
				ShareOfSpend: float64(8 + vi*2), SourcingTypes: []string{"项目制"},
				BuyerName: "陈工", SupplierContact: "张经理",
			})
		}

		for qi := 0; qi < 6; qi++ {
			previous := 100 * (1 + float64(6-qi)*0.01)
			price := previous * (1 - v.yoy)
			s.quotes = append(s.quotes, domain.Quote{
				ID:       fmt.Sprintf("qt-%s-%d", id, qi+1),
				TenantID: tenant.ID, SupplierID: id,
				PartName:  fmt.Sprintf("%s件-%02d", names[0], qi+1),
				UnitPrice: round2(price), PreviousPrice: round2(previous),
				BenchmarkPrice: round2(100 * v.priceIdx),
				Currency:       "CNY", PaymentDays: 60 + vi*5,
				CostBreakdown: v.coop > 80,
				QuotedAt:      monthDate(2026, 1+qi, 10), ValidUntil: monthDate(2026, 1+qi, 28),
			})
		}

		// ---- 订单 / 交付 / 质量事件 ----
		for i := 0; i < v.orders; i++ {
			month := 1 + i%10 // 2025-11 ~ 2026-10 之间铺开
			year := 2025
			if month <= 10 {
				year = 2026
			}
			issued := monthDate(year, month, 3+rng.Intn(10))
			promised := monthDate(year, month, 18+rng.Intn(8))
			poID := fmt.Sprintf("po-%s-%d", id, i+1)
			qty := 800 + rng.Intn(4200)
			expedited := rng.Float64() < 0.18
			status := "closed"
			if i >= v.orders-2 {
				status = "open"
			}
			s.orders = append(s.orders, domain.PurchaseOrder{
				ID: poID, TenantID: tenant.ID, SupplierID: id,
				PONo: fmt.Sprintf("PO26%04d%02d", i+1, vi+1), Category: names[0],
				PartName: fmt.Sprintf("%s件-%02d", names[0], i+1),
				Qty:      qty, UnitPrice: 100 * v.priceIdx, Amount: float64(qty) * 100 * v.priceIdx,
				IssuedAt: issued, PromisedAt: promised, ClosedAt: promised,
				Status: status, Expedited: expedited,
			})
			batches := 1 + rng.Intn(3)
			for b := 0; b < batches; b++ {
				onTime := rng.Float64() < v.otd
				if expedited && rng.Float64() < 0.28 {
					// 插单挤产能，准时率本来就会掉
					onTime = false
				}
				arrived := promised
				if !onTime {
					arrived = addDays(promised, 1+rng.Intn(9))
				}
				bq := qty / batches
				rejected := 0
				// 按目标 PPM 反推"这一批至少出现 1 件不良"的概率（泊松近似）。
				// 直接拿 ppm 乘批量会把概率算爆，导致每批都不合格。
				expected := v.ppm / 1e6 * float64(bq)
				if rng.Float64() < 1-math.Exp(-expected) {
					rejected = 1 + rng.Intn(3)
				}
				result := "pass"
				if rejected > 0 {
					// 不良率越高的供应商，判退的比例越高
					if rng.Float64() < (1-v.passRate)*20 {
						result = "fail"
					} else {
						result = "conditional"
					}
				} else if rng.Float64() < (1-v.passRate)*0.5 {
					result = "fail"
					rejected = 1 + rng.Intn(2)
				}
				leadDays := daysBetween(issued, arrived)
				s.deliveries = append(s.deliveries, domain.Delivery{
					ID: fmt.Sprintf("dl-%s-%d-%d", id, i+1, b+1), TenantID: tenant.ID, SupplierID: id,
					POID: poID, PONo: fmt.Sprintf("PO26%04d%02d", i+1, vi+1),
					BatchNo:  fmt.Sprintf("B%02d%02d%d", i+1, b+1, vi),
					PartName: fmt.Sprintf("%s件-%02d", names[0], i+1),
					Qty:      bq, Accepted: bq - rejected, Rejected: rejected,
					ArrivedAt: arrived, PromisedAt: promised, OnTime: onTime,
					LeadDays: leadDays, PromisedLeadDays: daysBetween(issued, promised),
					IQCResult: result, FirstArticle: b == 0, FAIPass: result != "fail",
				})
			}
		}

		for e := 0; e < 6+v.criticalEvents; e++ {
			occ := monthDate(2026, 1+e*2, 5+rng.Intn(20))
			due := addDays(occ, 14+rng.Intn(14))
			severity := "minor"
			if e < v.criticalEvents {
				severity = "critical"
			} else if e%3 == 0 {
				severity = "major"
			}
			closed := ""
			if rng.Float64() < float64(v.coop)/100 {
				closed = addDays(occ, 5+rng.Intn(20))
			}
			s.events = append(s.events, domain.QualityEvent{
				ID: fmt.Sprintf("qe-%s-%d", id, e+1), TenantID: tenant.ID, SupplierID: id,
				Type: "来料不良", Severity: severity, OccurredAt: occ, DueAt: due,
				ClosedAt: closed, EightD: severity != "minor",
				RootCause: "尺寸超差 / 外观缺陷（演示）",
			})
		}

		for t := 0; t < 9; t++ {
			opened := monthDate(2026, 2+t%8, 8+rng.Intn(15))
			hours := v.responseH * (0.6 + rng.Float64()*0.9)
			days := v.resolveD * (0.6 + rng.Float64()*0.9)
			s.tickets = append(s.tickets, domain.ServiceTicket{
				ID: fmt.Sprintf("st-%s-%d", id, t+1), TenantID: tenant.ID, SupplierID: id,
				Type:     []string{"技术咨询", "质量整改", "交期催办", "售后"}[t%4],
				OpenedAt: opened, Status: "resolved",
				FirstResponseAt: fmt.Sprintf("%s +%.1fh", opened, hours),
				ResolvedAt:      fmt.Sprintf("%s +%.1fd", opened, days),
				ResponseHours:   round2(hours), ResolveDays: round2(days),
			})
		}

		if vi < 4 {
			// 第二家采购方：量更少、口径更严，平台评级才有多买方分歧可看
			s.seedSecondBuyer(rng, tenant2.ID, id, names[0], v)
		}

		s.reviews = append(s.reviews, domain.TechReview{
			ID: "tr-" + id, TenantID: tenant.ID, SupplierID: id,
			ReviewedAt: "2026-09-18", Reviewer: "质量部 周工",
			CapabilityCoverage: float64(v.coverage), AutomationLevel: v.automation,
			RndRatio: float64(v.rnd), NPIProjects: v.npiProj, NPISuccess: v.npiSucc,
			CooperationScore: v.coop, Notes: "现场审核记录（演示数据）",
		})
	}

	return suppliers
}

// ClearDemo 清空全部 SRM 数据（含演示数据），回到干净状态。
func (s *Store) ClearDemo() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tenants = map[string]domain.Tenant{}
	s.engagements = nil
	s.orders = nil
	s.deliveries = nil
	s.events = nil
	s.quotes = nil
	s.tickets = nil
	s.reviews = nil
	s.demo = false
}

// ---------------- 小工具 ----------------

func monthDate(year, month, day int) string {
	if month < 1 {
		month = 1
	}
	if month > 12 {
		month = 12
	}
	if day < 1 {
		day = 1
	}
	if day > 28 {
		day = 28
	}
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
}

func addDays(date string, n int) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return t.AddDate(0, 0, n).Format("2006-01-02")
}

func daysBetween(a, b string) int {
	t1, err1 := time.Parse("2006-01-02", a)
	t2, err2 := time.Parse("2006-01-02", b)
	if err1 != nil || err2 != nil {
		return 0
	}
	return int(t2.Sub(t1).Hours() / 24)
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func capName(code string) string {
	m := map[string]string{
		"cnc": "CNC 精密加工", "diecasting": "压铸", "anodizing": "阳极氧化",
		"pcba": "SMT 与 PCBA", "pcb": "PCB 制造", "testing": "可靠性与认证",
		"display": "显示模组", "glass": "玻璃盖板", "coating": "表面处理与镀膜",
		"injection": "精密注塑", "mold": "模具设计制造",
		"acoustics": "声学器件", "silicone": "硅胶与橡胶成型",
		"battery": "锂电芯与 PACK",
	}
	if v, ok := m[code]; ok {
		return v
	}
	return strings.ToUpper(code)
}

// seedSecondBuyer 为第二家采购方生成单据。调用方必须已持有写锁。
//
// 为什么非要造第二份：平台级评级如果只有一个买方，本质上就是那家企业的内部打分，
// 谈不上"平台数据"。只有两家以上买方在同一套口径下评估，分歧度（Spread）才有意义。
func (s *Store) seedSecondBuyer(rng *rand.Rand, tenantID, supplierID, partName string, v demoVendor) {
	count := v.orders / 2
	if count < 8 {
		count = 8
	}
	for i := 0; i < count; i++ {
		month := 1 + i%8
		issued := monthDate(2026, month, 5+rng.Intn(10))
		promised := monthDate(2026, month, 19+rng.Intn(6))
		poID := fmt.Sprintf("po2-%s-%d", supplierID, i+1)
		qty := 400 + rng.Intn(1800)
		s.orders = append(s.orders, domain.PurchaseOrder{
			ID: poID, TenantID: tenantID, SupplierID: supplierID,
			PONo: fmt.Sprintf("LT26%04d", i+1), Category: partName,
			PartName: partName, Qty: qty, UnitPrice: 100 * v.priceIdx,
			Amount:   float64(qty) * 100 * v.priceIdx,
			IssuedAt: issued, PromisedAt: promised, ClosedAt: promised,
			Status: "closed", Expedited: rng.Float64() < 0.3,
		})
		batches := 1 + rng.Intn(2)
		for b := 0; b < batches; b++ {
			// 这家客户的订单批量更小、交期更紧，准时率系统性低 4 个百分点
			onTime := rng.Float64() < (v.otd - 0.04)
			arrived := promised
			if !onTime {
				arrived = addDays(promised, 1+rng.Intn(7))
			}
			bq := qty / batches
			expected := v.ppm / 1e6 * float64(bq)
			rejected := 0
			if rng.Float64() < 1-math.Exp(-expected) {
				rejected = 1 + rng.Intn(2)
			}
			result := "pass"
			if rejected > 0 {
				if rng.Float64() < (1-v.passRate)*20 {
					result = "fail"
				} else {
					result = "conditional"
				}
			}
			s.deliveries = append(s.deliveries, domain.Delivery{
				ID:       fmt.Sprintf("dl2-%s-%d-%d", supplierID, i+1, b+1),
				TenantID: tenantID, SupplierID: supplierID, POID: poID,
				PONo: fmt.Sprintf("LT26%04d", i+1), BatchNo: fmt.Sprintf("L%02d%02d", i+1, b+1),
				PartName: partName, Qty: bq, Accepted: bq - rejected, Rejected: rejected,
				ArrivedAt: arrived, PromisedAt: promised, OnTime: onTime,
				LeadDays: daysBetween(issued, arrived), PromisedLeadDays: daysBetween(issued, promised),
				IQCResult: result, FirstArticle: b == 0, FAIPass: result != "fail",
			})
		}
	}
	// 第二家客户的报价：同一家供应商对不同客户的价格本来就不一样，
	// 平台评级要能容纳这种差异，而不是假装全市场一个价。
	for qi := 0; qi < 4; qi++ {
		previous := 100 * (1 + float64(4-qi)*0.008)
		price := previous * (1 - v.yoy*0.7)
		s.quotes = append(s.quotes, domain.Quote{
			ID: fmt.Sprintf("qt2-%s-%d", supplierID, qi+1), TenantID: tenantID, SupplierID: supplierID,
			PartName:  fmt.Sprintf("%s件-%02d", partName, qi+1),
			UnitPrice: round2(price), PreviousPrice: round2(previous),
			BenchmarkPrice: round2(100 * v.priceIdx),
			Currency:       "CNY", PaymentDays: 45,
			CostBreakdown: false, QuotedAt: monthDate(2026, 2+qi, 12), ValidUntil: monthDate(2026, 2+qi, 28),
		})
	}

	// 第二家客户的质量事件与工单
	for e := 0; e < 3; e++ {
		occ := monthDate(2026, 2+e*3, 6+rng.Intn(18))
		due := addDays(occ, 14+rng.Intn(10))
		closed := ""
		if rng.Float64() < float64(v.coop)/110 {
			closed = addDays(occ, 7+rng.Intn(18))
		}
		s.events = append(s.events, domain.QualityEvent{
			ID: fmt.Sprintf("qe2-%s-%d", supplierID, e+1), TenantID: tenantID, SupplierID: supplierID,
			Type: "来料不良", Severity: "major", OccurredAt: occ, DueAt: due,
			ClosedAt: closed, EightD: true, RootCause: "装配尺寸偏差（演示）",
		})
	}
}
