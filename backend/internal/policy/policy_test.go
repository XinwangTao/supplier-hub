package policy

import (
	"testing"
	"time"
)

// 这一组测试锁的是"壁垒不能被后来的人顺手拆掉"。
// 它们不测业务逻辑，只测边界：某个字段被误放到公开层，必须让 CI 挂掉，
// 而不是等上线后被人拖走一个库才发现。

func TestEveryFieldGroupHasValidTier(t *testing.T) {
	valid := map[Tier]bool{TierPublic: true, TierVerified: true, TierPartner: true, TierOwner: true}
	for _, g := range Describe().Fields {
		if !valid[g.Tier] {
			t.Fatalf("字段组 %s 的层级非法：%q", g.Key, g.Tier)
		}
		if g.Reason == "" {
			t.Fatalf("字段组 %s 缺少对用户解释的理由——没有理由的限制会变成流失", g.Key)
		}
	}
}

// 联系方式是最容易被拿去绕开平台的字段，必须锁在合作层。
func TestContactFieldsAreNotPublic(t *testing.T) {
	sensitive := map[string]bool{
		"contact_phone": true, "contact_email": true, "contact_name": true,
		"customers": true, "quote_price": true, "office_address": true,
	}
	for _, g := range Describe().Fields {
		for _, f := range g.Fields {
			if sensitive[f] && g.Tier == TierPublic {
				t.Fatalf("敏感字段 %s 出现在公开层（组 %s）——这等于把数据白送", f, g.Key)
			}
		}
	}
}

// 公开层必须保留"够用但不值钱"的字段，否则平台连被搜到都做不到。
func TestPublicTierKeepsDiscoverability(t *testing.T) {
	has := map[string]bool{}
	for _, g := range Describe().Fields {
		if g.Tier != TierPublic {
			continue
		}
		for _, f := range g.Fields {
			has[f] = true
		}
	}
	for _, must := range []string{"name", "city", "capabilities", "grade"} {
		if !has[must] {
			t.Fatalf("公开层缺少 %s：没有它，供应商连被搜到都做不到", must)
		}
	}
	for _, mustNot := range []string{"total", "dimensions"} {
		if has[mustNot] {
			t.Fatalf("%s 不应在公开层：分数可被批量采集拼榜", mustNot)
		}
	}
}

// 明确声明"不提供导出"，并且必须给出替代方案——
// 只拒绝不给出路，用户会直接走人。
func TestExportPolicyHasAlternative(t *testing.T) {
	d := Describe().Export
	if len(d.Denied) == 0 || len(d.Allowed) == 0 {
		t.Fatal("导出策略必须同时声明允许与禁止的范围")
	}
	if d.Why == "" || d.Alternative == "" {
		t.Fatal("禁止导出必须说明原因并给出替代方案")
	}
	// 企业自己的数据必须可导出：锁别人的数据、放自己的数据，这条不对称是刻意的。
	found := false
	for _, a := range d.Allowed {
		if len(a) > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("自有数据应可导出")
	}
}

func TestLimiterBlocksAfterLimit(t *testing.T) {
	l := NewLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		if !l.allow("1.1.1.1") {
			t.Fatalf("第 %d 次请求不应被拦", i+1)
		}
	}
	if l.allow("1.1.1.1") {
		t.Fatal("超出上限后必须被拦")
	}
	// 不同来源互不影响，否则一个人能把全公司拖下水。
	if !l.allow("2.2.2.2") {
		t.Fatal("其他来源不应受限")
	}
}

func TestLimiterWindowResets(t *testing.T) {
	l := NewLimiter(1, 30*time.Millisecond)
	if !l.allow("k") {
		t.Fatal("首次应放行")
	}
	if l.allow("k") {
		t.Fatal("窗口内第二次应拦")
	}
	time.Sleep(40 * time.Millisecond)
	if !l.allow("k") {
		t.Fatal("窗口过期后应恢复")
	}
}

// 配额必须是服务端常量，不能由请求参数决定。
func TestQuotasAreBounded(t *testing.T) {
	d := Describe().Quotas
	if d.MaxPageSize <= 0 || d.MaxPageSize > 100 {
		t.Fatalf("单页上限不合理：%d", d.MaxPageSize)
	}
	if d.DeepPageFreeMax < d.MaxPageSize {
		t.Fatal("深翻页上限不应小于单页上限")
	}
	if d.RequestsPerMin <= 0 {
		t.Fatal("限流阈值必须为正")
	}
}
