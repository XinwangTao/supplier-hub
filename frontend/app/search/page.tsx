"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { SupplierDossier } from "../../components/DetailPanel";
import { EngageControl, RatingChip } from "../../components/SourcingActions";
import TenantPicker from "../../components/TenantPicker";
import {
  fetchBriefs,
  fetchSupplier,
  money,
  searchSuppliers,
  type BriefIndex,
  type Filters,
  type ScoredSupplier,
  type Supplier,
} from "../../lib/api";
import { readTenant, subscribeTenant } from "../../lib/tenant";

const initialFilters: Filters = { business_keyword: "", province: "", city: "", min_capital: 0, min_employees: 0, max_risk: "" };
const riskLabel: Record<string, string> = { low: "低风险", medium: "中风险", high: "高风险", critical: "重点风险" };

export default function SearchPage() {
  const [query, setQuery] = useState("找东莞注册资本 500 万以上、员工 100 人以上的精密注塑生产厂家");
  const [filters, setFilters] = useState<Filters>(initialFilters);
  const [items, setItems] = useState<ScoredSupplier[]>([]);
  const [interpretation, setInterpretation] = useState("");
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [advanced, setAdvanced] = useState(false);
  const [selected, setSelected] = useState<Supplier | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [tenant, setTenant] = useState("");
  const [briefs, setBriefs] = useState<BriefIndex["items"]>({});

  const loadBriefs = useCallback((tenantId: string) => {
    fetchBriefs(tenantId)
      .then((index) => setBriefs(index.items || {}))
      .catch(() => setBriefs({}));
  }, []);

  useEffect(() => {
    const initial = readTenant();
    setTenant(initial);
    loadBriefs(initial);
    return subscribeTenant((id) => {
      setTenant(id);
      loadBriefs(id);
    });
  }, [loadBriefs]);

  async function runSearch(nextQuery = query, nextFilters = filters) {
    setLoading(true);
    setError("");
    try {
      const data = await searchSuppliers(nextQuery, nextFilters);
      setItems(data.items);
      setTotal(data.total);
      setInterpretation(data.interpretation);
      setFilters(data.applied_filters);
    } catch (err) {
      setError(err instanceof Error ? err.message : "搜索服务暂时不可用");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void runSearch();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function openSupplier(id: string) {
    setDetailLoading(true);
    try {
      setSelected(await fetchSupplier(id));
    } catch {
      setError("档案加载失败");
    } finally {
      setDetailLoading(false);
    }
  }

  return (
    <main className="app">
      <header className="topbar">
        <div className="brand">
          <span className="brandMark">S</span>
          <div>
            <b>供应链网络</b>
            <small>SUPPLY MESH</small>
          </div>
        </div>
        <nav className="navTabs">
          <Link href="/">产品发散网</Link>
          <Link href="/">供应链全域网</Link>
          <span className="active">寻源搜索</span>
          <Link href="/srm">供应商管理</Link>
          <Link href="/ranking">平台评级</Link>
        </nav>
        <div className="topMeta">
          <span className="liveDot" />
          {total} 家匹配企业
          <TenantPicker compact />
        </div>
      </header>

      <section className="searchPage">
        <div className="searchIntro">
          <div className="canvasEyebrow">SMART SOURCING / 供应商情报</div>
          <h1>把采购需求，变成可验证的名单</h1>
          <p>用一句话描述需求，系统会从经营范围、企业规模、所在地区与风险信息中筛出候选对象，点开即可看到法定代表人与股权结构。</p>
        </div>

        <form
          className="netSearch wide"
          onSubmit={(event) => {
            event.preventDefault();
            void runSearch();
          }}
        >
          <span className="netSearchIcon">⌕</span>
          <input value={query} onChange={(event) => setQuery(event.target.value)} aria-label="自然语言寻源" />
          <button className="primaryButton" type="submit" disabled={loading}>
            {loading ? "分析中…" : "开始寻源 →"}
          </button>
        </form>

        <div className="searchToolbar">
          <div>
            <div className="resultTitle">
              候选供应商 <strong>{total}</strong>
            </div>
            <div className="interpretation">{interpretation || "等待搜索条件"}</div>
          </div>
          <button className="advancedButton" onClick={() => setAdvanced(!advanced)}>
            {advanced ? "收起筛选" : "高级筛选"} <span>{advanced ? "↑" : "↓"}</span>
          </button>
        </div>

        {advanced && (
          <div className="filters">
            <label>
              品类关键词
              <input placeholder="如：注塑" value={filters.business_keyword} onChange={(event) => setFilters({ ...filters, business_keyword: event.target.value })} />
            </label>
            <label>
              省份
              <input placeholder="如：广东省" value={filters.province} onChange={(event) => setFilters({ ...filters, province: event.target.value })} />
            </label>
            <label>
              城市
              <input placeholder="如：东莞市" value={filters.city} onChange={(event) => setFilters({ ...filters, city: event.target.value })} />
            </label>
            <label>
              最低注册资本（万）
              <input type="number" value={filters.min_capital || ""} onChange={(event) => setFilters({ ...filters, min_capital: Number(event.target.value) })} />
            </label>
            <label>
              最低员工数
              <input type="number" value={filters.min_employees || ""} onChange={(event) => setFilters({ ...filters, min_employees: Number(event.target.value) })} />
            </label>
            <label>
              最高风险
              <select value={filters.max_risk} onChange={(event) => setFilters({ ...filters, max_risk: event.target.value as Filters["max_risk"] })}>
                <option value="">不限</option>
                <option value="low">低风险</option>
                <option value="medium">中风险</option>
                <option value="high">高风险</option>
              </select>
            </label>
            <button className="filterApply" onClick={() => void runSearch()}>
              应用筛选
            </button>
          </div>
        )}

        {error && <div className="errorBanner">{error}</div>}

        <div className="supplierList">
          {loading && !items.length && <div className="empty">正在解析需求并匹配企业…</div>}
          {!loading && !items.length && <div className="empty">没有找到符合条件的供应商，请放宽筛选条件。</div>}
          {items.map((item, index) => (
            <article className="supplierRow" key={item.id} onClick={() => void openSupplier(item.id)}>
              <div className="rank">{String(index + 1).padStart(2, "0")}</div>
              <div className="supplierMain">
                <div className="supplierName">
                  {item.name}
                  <span className="manufacturer">{item.is_manufacturer ? "生产型" : "贸易/服务"}</span>
                </div>
                <div className="scope">{item.business_scope}</div>
                <div className="tags">
                  <span className={`tagRisk risk-${item.risk_level}`}>{riskLabel[item.risk_level]}</span>
                  {(item.risk_tags || []).map((tag) => (
                    <span key={tag} className="tag">
                      {tag}
                    </span>
                  ))}
                  <span className="location">
                    ⌖ {item.province} · {item.city}
                  </span>
                  <span className="location">法人 {item.legal_representative}</span>
                </div>
              </div>
              {/* 平台评级 + 引入动作：寻源不只看工商信息，还要看别人用下来怎么样 */}
              <div className="rowLoop" onClick={(event) => event.stopPropagation()}>
                <RatingChip brief={briefs[item.id]} compact />
                <EngageControl
                  supplierId={item.id}
                  supplierName={item.short_name || item.name}
                  tenant={tenant}
                  brief={briefs[item.id]}
                  onChanged={() => loadBriefs(tenant)}
                />
              </div>
              <div className="metrics">
                <div>
                  <b>{money(item.registered_capital)}</b>
                  <small>注册资本</small>
                </div>
                <div>
                  <b>{item.employee_count.toLocaleString()}</b>
                  <small>员工人数</small>
                </div>
              </div>
              <div className="score">
                <strong>{item.match_score}</strong>
                <span>匹配度</span>
              </div>
              <div className="chevron">›</div>
            </article>
          ))}
        </div>
      </section>

      {selected && (
        <div className="drawerBackdrop" onClick={() => setSelected(null)}>
          <aside className="drawer" onClick={(event) => event.stopPropagation()}>
            <div className="panelHead">
              <span className="panelType type-supplier">供应商档案</span>
              <button className="panelClose" onClick={() => setSelected(null)} aria-label="关闭">
                ×
              </button>
            </div>
            <h2 className="panelTitle">{selected.name}</h2>
            <div className="panelSub">
              {selected.city} · {selected.district} · {selected.is_manufacturer ? "生产型企业" : "贸易/服务企业"}
            </div>
            {detailLoading ? (
              <div className="panelLoading">加载中…</div>
            ) : (
              <SupplierDossier
                supplier={selected}
                tenant={tenant}
                brief={briefs[selected.id]}
                onBriefChanged={() => loadBriefs(tenant)}
              />
            )}
          </aside>
        </div>
      )}
    </main>
  );
}
