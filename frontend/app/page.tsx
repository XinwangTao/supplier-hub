"use client";

import { useEffect, useState } from "react";
import { Checklist, Filters, ScoredSupplier, Supplier, getChecklist, getSupplier, searchSuppliers } from "../lib/api";

const initialFilters: Filters = { business_keyword: "", province: "", city: "", min_capital: 0, min_employees: 0, max_risk: "" };
const riskLabel: Record<string, string> = { low: "低风险", medium: "中风险", high: "高风险", critical: "重点风险" };
const money = (value: number) => `${value.toLocaleString("zh-CN")} 万`;

export default function Home() {
  const [query, setQuery] = useState("找东莞注册资本500万以上、员工100人以上的精密注塑生产厂家");
  const [filters, setFilters] = useState<Filters>(initialFilters);
  const [result, setResult] = useState<{ items: ScoredSupplier[]; total: number; interpretation: string }>({ items: [], total: 0, interpretation: "" });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [advanced, setAdvanced] = useState(false);
  const [selected, setSelected] = useState<Supplier | null>(null);
  const [checklist, setChecklist] = useState<Checklist | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [copied, setCopied] = useState(false);

  async function runSearch() {
    setLoading(true); setError("");
    try { const data = await searchSuppliers(query, filters); setResult(data); setFilters(data.applied_filters); }
    catch (e) { setError(e instanceof Error ? e.message : "请求失败"); }
    finally { setLoading(false); }
  }
  useEffect(() => { void runSearch(); }, []);
  async function openSupplier(id: string) {
    setDetailLoading(true); setChecklist(null);
    try { setSelected(await getSupplier(id)); }
    catch (e) { setError(e instanceof Error ? e.message : "加载失败"); }
    finally { setDetailLoading(false); }
  }
  async function makeChecklist() {
    if (!selected) return;
    setDetailLoading(true);
    try { setChecklist(await getChecklist(selected.id)); }
    catch (e) { setError(e instanceof Error ? e.message : "生成失败"); }
    finally { setDetailLoading(false); }
  }
  const checklistText = checklist?.sections.map(section => `【${section.category}】\n${section.items.map(item => `□ ${item.title}｜${item.description}`).join("\n")}`).join("\n\n") || "";
  async function copyChecklist() { await navigator.clipboard.writeText(checklistText); setCopied(true); setTimeout(() => setCopied(false), 1600); }
  function runExample(value: string) { setQuery(value); void searchSuppliers(value, filters).then(data => { setResult(data); setFilters(data.applied_filters); }).catch(() => setError("搜索服务暂时不可用")); }

  return <main className="shell">
    <header className="header"><div className="brand"><span className="brandMark">S</span><span>供应商寻源台</span></div><div className="headerMeta"><span className="liveDot" />Mock 数据源 · 已连接</div></header>
    <section className="hero"><div className="eyebrow">SMART SOURCING / 供应商情报</div><h1>把采购需求，变成<br /><em>可验证的供应商名单。</em></h1><p className="heroCopy">用一句话描述你的需求。系统会从经营范围、企业规模、所在地区和风险信息中，筛出值得现场核验的候选对象。</p>
      <div className="searchBox"><div className="searchIcon">⌕</div><input aria-label="自然语言寻源" value={query} onChange={e => setQuery(e.target.value)} onKeyDown={e => { if (e.key === "Enter") void runSearch(); }} /><button className="primaryButton" onClick={() => void runSearch()} disabled={loading}>{loading ? "分析中…" : "开始寻源  →"}</button></div>
      <div className="examples"><span>试试：</span><button onClick={() => runExample("找苏州自动化设备制造商，风险可控")}>苏州自动化设备制造商</button><button onClick={() => runExample("找宁波注册资本2000万以上的新能源零部件厂家")}>宁波新能源零部件厂家</button></div>
    </section>
    <section className="workspace"><div className="toolbar"><div><div className="resultTitle">候选供应商 <strong>{result.total}</strong></div><div className="interpretation">{result.interpretation || "等待搜索条件"}</div></div><button className="advancedButton" onClick={() => setAdvanced(!advanced)}>{advanced ? "收起筛选" : "高级筛选"} <span>{advanced ? "↑" : "↓"}</span></button></div>
      {advanced && <div className="filters">{[["品类关键词", "business_keyword", "输入产品或工艺"], ["省份", "province", "如：广东省"], ["城市", "city", "如：东莞市"]].map(([label, key, placeholder]) => <label key={key}>{label}<input placeholder={placeholder} value={String(filters[key as keyof Filters] || "")} onChange={e => setFilters({ ...filters, [key]: e.target.value })} /></label>)}<label>最低注册资本（万）<input type="number" value={filters.min_capital || ""} onChange={e => setFilters({ ...filters, min_capital: Number(e.target.value) })} /></label><label>最低员工数<input type="number" value={filters.min_employees || ""} onChange={e => setFilters({ ...filters, min_employees: Number(e.target.value) })} /></label><label>最高风险<select value={filters.max_risk} onChange={e => setFilters({ ...filters, max_risk: e.target.value as Filters["max_risk"] })}><option value="">不限</option><option value="low">低风险</option><option value="medium">中风险</option><option value="high">高风险</option></select></label><button className="filterApply" onClick={() => void runSearch()}>应用筛选</button></div>}
      {error && <div className="error">{error} <button onClick={() => void runSearch()}>重试</button></div>}
      <div className="contentGrid"><div className="list">{loading && !result.items.length ? <div className="empty">正在解析需求并匹配企业…</div> : !result.items.length ? <div className="empty">没有找到符合条件的供应商，请放宽筛选条件。</div> : result.items.map((supplier, index) => <article className="supplierRow" key={supplier.id} onClick={() => void openSupplier(supplier.id)}><div className="rank">{String(index + 1).padStart(2, "0")}</div><div className="supplierMain"><div className="supplierName">{supplier.name}<span className="manufacturer">{supplier.is_manufacturer ? "生产型" : "贸易/服务"}</span></div><div className="scope">{supplier.business_scope}</div><div className="tags">{supplier.risk_tags.map(tag => <span key={tag} className={supplier.risk_level === "low" ? "tagGreen" : "tag"}>{tag}</span>)}<span className="location">⌖ {supplier.province} · {supplier.city}</span></div></div><div className="metrics"><div><b>{money(supplier.registered_capital)}</b><small>注册资本</small></div><div><b>{supplier.employee_count.toLocaleString()}</b><small>员工人数</small></div></div><div className="score"><strong>{supplier.match_score}</strong><span>匹配度</span></div><div className="chevron">›</div></article>)}</div><aside className="insight"><div className="insightLabel">RANKING LOGIC</div><h3>为什么是这些供应商？</h3><p>综合匹配度以经营范围为核心，结合地区、企业规模与风险水平计算。</p><div className="weight"><div><span>经营范围匹配</span><b>40%</b></div><div className="bar"><i style={{ width: "40%" }} /></div></div><div className="weight"><div><span>规模与交付能力</span><b>35%</b></div><div className="bar"><i style={{ width: "35%" }} /></div></div><div className="weight"><div><span>风险与稳定性</span><b>25%</b></div><div className="bar"><i style={{ width: "25%" }} /></div></div><div className="tip">结果仅作为寻源参考，建议在合作前完成现场审核与资质核验。</div></aside></div>
    </section>
    {selected && <div className="modalBackdrop" onClick={() => setSelected(null)}><section className="drawer" onClick={e => e.stopPropagation()}><button className="close" onClick={() => setSelected(null)}>×</button>{detailLoading && !checklist ? <div className="empty">加载中…</div> : <><div className="drawerEyebrow">SUPPLIER PROFILE</div><h2>{selected.name}</h2><div className="drawerLocation">{selected.province} · {selected.city} · {selected.is_manufacturer ? "生产型企业" : "贸易/服务企业"}</div><div className="profileGrid"><div><small>注册资本</small><b>{money(selected.registered_capital)}</b></div><div><small>实缴资本</small><b>{money(selected.paid_capital)}</b></div><div><small>员工人数</small><b>{selected.employee_count.toLocaleString()} 人</b></div><div><small>成立日期</small><b>{selected.establish_date}</b></div></div><h4>主营与产能</h4><p className="detailText">{selected.business_scope}</p><p className="capacity">{selected.capacity_description}</p><h4>资质认证</h4><div className="certList">{selected.certifications.map(c => <span key={c}>✓ {c}</span>)}</div><div className="actionRow"><button className="primaryButton" onClick={() => void makeChecklist()} disabled={detailLoading}>生成现场审核清单</button><span className="riskStatus">{riskLabel[selected.risk_level]} · {selected.risk_tags.join("、")}</span></div>{checklist && <div className="checklist"><div className="checklistHead"><h3>现场审核清单</h3><button onClick={() => void copyChecklist()}>{copied ? "已复制" : "复制清单"}</button></div><p>{checklist.summary}</p>{checklist.sections.map(section => <div className="section" key={section.category}><h4>{section.category}</h4>{section.items.map(item => <div className="checkItem" key={item.title}><span>□</span><div><b>{item.title}</b><p>{item.description}</p></div><small>{item.priority}</small></div>)}</div>)}</div>}</>}</section></div>}
  </main>;
}
