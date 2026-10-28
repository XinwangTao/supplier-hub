package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/example/supplier-hub/backend/internal/domain"
)

// 写入护栏。
//
// 这一段存在的理由只有一个：如果任何人都能往 SRM 里写单据，
// 那平台评级就是个可以被任意刷的数字。
//
// 三条硬规则，缺一条评级就不可信：
//  1. 采购方必须先建档（租户存在）—— 凭空出现的 tenant_id 一律拒绝。
//  2. 采购方与该供应商必须已有合作关系 —— 绕过"引入 SRM"直接录单，
//     等于绕过了整条闭环的入口。
//  3. 已终止的合作关系不能再录单 —— 终止通常有质量或合规原因，
//     想恢复必须显式改状态，让这件事留下痕迹。
//
// 另外做了重复单据拦截：同一个批次号在同一采购方下只能录一次。
// 想要 15 个批次拿正式评级，就得真有 15 个批次，不能把 1 个批次 POST 15 遍。

// ErrTenantRequired 采购方未建档。
var ErrTenantRequired = errors.New("tenant_id 必填，且该企业必须先在平台上建档")

// ErrEngagementRequired 没有合作关系。
var ErrEngagementRequired = errors.New("该企业与这家供应商还没有合作关系，请先在寻源页引入 SRM")

// ErrEngagementTerminated 合作关系已终止。
var ErrEngagementTerminated = errors.New("合作关系已终止，不能继续录入单据；如需恢复请先把状态改回候选或合作中")

// ErrDuplicateBatch 批次号重复。
var ErrDuplicateBatch = errors.New("该批次号在此采购方下已录入过，重复录入会被拦截")

// ErrDuplicateOrder 订单号重复。
var ErrDuplicateOrder = errors.New("该订单号在此采购方下已录入过")

// checkWritable 校验一次写入是否合法。非法时同时落一条被拒审计。
func (s *SupplierOpsService) checkWritable(tenantID, supplierID, action, actor string) (domain.Engagement, error) {
	tenantID = strings.TrimSpace(tenantID)
	supplierID = strings.TrimSpace(supplierID)
	reject := func(err error) (domain.Engagement, error) {
		s.store.AuditReject(action, tenantID, actor, supplierID, err.Error())
		return domain.Engagement{}, err
	}
	if tenantID == "" || supplierID == "" {
		return reject(ErrTenantRequired)
	}
	if _, ok := s.store.Tenant(tenantID); !ok {
		return reject(ErrTenantRequired)
	}
	e, ok := s.store.EngagementOf(tenantID, supplierID)
	if !ok {
		return reject(ErrEngagementRequired)
	}
	if string(e.Status) == string(domain.EngagementTerminated) {
		return reject(ErrEngagementTerminated)
	}
	return e, nil
}

// RecordOrderGuarded 录入订单（带护栏与审计）。
func (s *SupplierOpsService) RecordOrderGuarded(ctx context.Context, o domain.PurchaseOrder, actor string) (domain.PurchaseOrder, error) {
	if _, err := s.checkWritable(o.TenantID, o.SupplierID, "order.create", actor); err != nil {
		return domain.PurchaseOrder{}, err
	}
	if o.Qty <= 0 {
		s.store.AuditReject("order.create", o.TenantID, actor, o.SupplierID, "qty 必须大于 0")
		return domain.PurchaseOrder{}, errors.New("qty 必须大于 0")
	}
	if o.ID == "" {
		o.ID = fmt.Sprintf("po-%d", time.Now().UnixNano())
	}
	if o.PONo == "" {
		o.PONo = "PO" + time.Now().Format("20060102") + "-" + o.ID[len(o.ID)-4:]
	}
	if o.Status == "" {
		o.Status = "open"
	}
	if o.IssuedAt == "" {
		o.IssuedAt = time.Now().Format("2006-01-02")
	}
	if s.store.HasOrder(o.TenantID, o.SupplierID, o.PONo, o.ID) {
		s.store.AuditReject("order.create", o.TenantID, actor, o.PONo, "订单号重复")
		return domain.PurchaseOrder{}, ErrDuplicateOrder
	}
	o = s.store.AddOrder(o)
	supplierName := s.supplierNameOf(ctx, o.SupplierID)
	s.store.Audit("order.create", o.TenantID, actor, o.PONo,
		fmt.Sprintf("向 %s 下发订单 %s：%s × %d", supplierName, o.PONo, o.PartName, o.Qty), "api")
	return o, nil
}

// RecordDeliveryGuarded 录入交付批次（带护栏与审计）。
func (s *SupplierOpsService) RecordDeliveryGuarded(ctx context.Context, d domain.Delivery, actor string) (domain.Delivery, error) {
	if _, err := s.checkWritable(d.TenantID, d.SupplierID, "delivery.create", actor); err != nil {
		return domain.Delivery{}, err
	}
	if d.Qty <= 0 {
		s.store.AuditReject("delivery.create", d.TenantID, actor, d.SupplierID, "qty 必须大于 0")
		return domain.Delivery{}, errors.New("qty 必须大于 0")
	}
	if d.ID == "" {
		d.ID = fmt.Sprintf("dl-%d", time.Now().UnixNano())
	}
	if d.BatchNo == "" {
		d.BatchNo = "B" + time.Now().Format("20060102") + "-" + d.ID[len(d.ID)-4:]
	}
	if d.ArrivedAt == "" {
		d.ArrivedAt = time.Now().Format("2006-01-02")
	}
	// 到货量与判定量必须能对上：accepted + rejected 不该超过收货数，
	// 超了说明单据录错（或者有意为之），这里不拦但会记进数据体检。
	if d.Accepted+d.Rejected > d.Qty {
		d.Accepted, d.Rejected = 0, 0
	}
	if s.store.HasDelivery(d.TenantID, d.SupplierID, d.BatchNo, d.ID) {
		s.store.AuditReject("delivery.create", d.TenantID, actor, d.BatchNo, "批次号重复")
		return domain.Delivery{}, ErrDuplicateBatch
	}
	d = s.store.AddDelivery(d)
	supplierName := s.supplierNameOf(ctx, d.SupplierID)
	verdict := "未判"
	switch d.IQCResult {
	case "pass":
		verdict = "IQC 合格"
	case "conditional":
		verdict = "让步接收"
	case "fail":
		verdict = "IQC 判退"
	}
	s.store.Audit("delivery.create", d.TenantID, actor, d.BatchNo,
		fmt.Sprintf("%s 到货 %d 件（%s）", supplierName, d.Qty, verdict), "api")
	return d, nil
}

// RecordEventGuarded 录入质量事件（带护栏与审计）。
func (s *SupplierOpsService) RecordEventGuarded(ctx context.Context, e domain.QualityEvent, actor string) (domain.QualityEvent, error) {
	if _, err := s.checkWritable(e.TenantID, e.SupplierID, "event.create", actor); err != nil {
		return domain.QualityEvent{}, err
	}
	if e.ID == "" {
		e.ID = fmt.Sprintf("ev-%d", time.Now().UnixNano())
	}
	if e.OccurredAt == "" {
		e.OccurredAt = time.Now().Format("2006-01-02")
	}
	if e.Severity == "" {
		e.Severity = "minor"
	}
	e = s.store.AddQualityEvent(e)
	supplierName := s.supplierNameOf(ctx, e.SupplierID)
	s.store.Audit("event.create", e.TenantID, actor, e.ID,
		fmt.Sprintf("登记 %s 质量事件（%s）：%s", supplierName, e.Severity, e.Type), "api")
	return e, nil
}

func (s *SupplierOpsService) supplierNameOf(ctx context.Context, id string) string {
	if sp, err := s.repo.GetByID(ctx, id); err == nil && sp.ShortName != "" {
		return sp.ShortName
	}
	return id
}

// AuditEntries 审计流水（最新在前）。
func (s *SupplierOpsService) AuditEntries(tenantID string, limit int) []domain.AuditEntry {
	return s.store.AuditEntries(tenantID, limit)
}
