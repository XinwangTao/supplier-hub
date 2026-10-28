"use client";

import { useEffect, useState } from "react";
import type { Checklist, Graph, GraphNode, Supplier, SupplierBrief } from "../lib/api";
import { fetchChecklist, money } from "../lib/api";
import { EngageControl, RatingChip } from "./SourcingActions";

type Props = {
  graph: Graph;
  selected: GraphNode | null;
  supplier: Supplier | null;
  supplierLoading: boolean;
  onSelectNode: (node: GraphNode | null) => void;
  onExpandProduct: (name: string) => void;
  onClose: () => void;
  tenant?: string;
  briefs?: Record<string, SupplierBrief>;
  onBriefChanged?: () => void;
};

const RISK_LABEL: Record<string, string> = { low: "低风险", medium: "中风险", high: "高风险", critical: "重点风险" };
const LEVEL_LABEL: Record<string, string> = { info: "提示", warn: "注意", critical: "关键" };

export default function DetailPanel({
  graph,
  selected,
  supplier,
  supplierLoading,
  onSelectNode,
  onExpandProduct,
  onClose,
  tenant,
  briefs,
  onBriefChanged,
}: Props) {
  const brief = supplier ? briefs?.[supplier.id] : undefined;
  if (!selected) {
    // 深链模式：?supplier=sup-001 直接打开某家供应商的工商档案
    if (supplier) {
      return (
        <aside className="panel">
          <div className="panelHead">
            <span className="panelType type-supplier">供应商档案</span>
            <button className="panelClose" onClick={onClose} aria-label="关闭">
              ×
            </button>
          </div>
          <h2 className="panelTitle">{supplier.name}</h2>
          {supplier.short_name && (
            <div className="panelSub">
              简称 {supplier.short_name} · {supplier.city}
            </div>
          )}
          <SupplierDossier
            supplier={supplier}
            graph={graph}
            onSelectNode={onSelectNode}
            tenant={tenant}
            brief={briefs?.[supplier.id]}
            onBriefChanged={onBriefChanged}
          />
        </aside>
      );
    }
    return <InsightPanel graph={graph} onSelectNode={onSelectNode} />;
  }

  return (
    <aside className="panel">
      <div className="panelHead">
        <span className={`panelType type-${selected.type}`}>{typeLabel(selected)}</span>
        <button className="panelClose" onClick={onClose} aria-label="关闭">
          ×
        </button>
      </div>
      <h2 className="panelTitle">{selected.label}</h2>
      {selected.subtitle && <div className="panelSub">{selected.subtitle}</div>}

      {selected.type === "product" && <ProductBody node={selected} graph={graph} onExpandProduct={onExpandProduct} onSelectNode={onSelectNode} />}
      {selected.type === "module" && <ModuleBody node={selected} graph={graph} onSelectNode={onSelectNode} />}
      {selected.type === "process" && <ProcessBody node={selected} graph={graph} onSelectNode={onSelectNode} />}
      {selected.type === "capability" && <CapabilityBody node={selected} graph={graph} onSelectNode={onSelectNode} />}
      {selected.type === "supplier" &&
        (supplierLoading ? (
          <div className="panelLoading">正在调取工商档案…</div>
        ) : supplier ? (
          <SupplierDossier
            supplier={supplier}
            graph={graph}
            onSelectNode={onSelectNode}
            tenant={tenant}
            brief={briefs?.[supplier.id]}
            onBriefChanged={onBriefChanged}
          />
        ) : (
          <div className="panelLoading">未能加载该供应商档案</div>
        ))}
    </aside>
  );
}

function typeLabel(node: GraphNode) {
  return { product: "产品", module: "模块", process: "工序", supplier: "供应商", capability: "工艺能力" }[node.type] || "节点";
}

function InsightPanel({ graph, onSelectNode }: { graph: Graph; onSelectNode: (node: GraphNode | null) => void }) {
  const isMesh = graph.mode === "mesh";
  return (
    <aside className="panel">
      <div className="panelHead">
        <span className="panelType type-insight">网络洞察</span>
      </div>
      <h2 className="panelTitle">{isMesh ? "全域供应链网络" : graph.product}</h2>
      <p className="panelSummary">{graph.summary}</p>

      <div className="statGrid">
        {isMesh ? (
          <>
            <Stat label="工艺能力" value={graph.stats.capabilities} />
            <Stat label="供应商" value={graph.stats.suppliers} />
            <Stat label="覆盖城市" value={graph.stats.cities} />
            <Stat label="高风险" value={graph.stats.high_risk} tone={graph.stats.high_risk > 0 ? "warn" : undefined} />
          </>
        ) : (
          <>
            <Stat label="模块" value={graph.stats.modules} />
            <Stat label="工序" value={graph.stats.processes} />
            <Stat label="供应商" value={graph.stats.suppliers} />
            <Stat label="覆盖城市" value={graph.stats.cities} />
          </>
        )}
      </div>

      <div className="insightList">
        {graph.insights.map((insight, index) => (
          <article key={`${insight.kind}-${index}`} className={`insightCard level-${insight.level}`}>
            <div className="insightHead">
              <span className="insightLevel">{LEVEL_LABEL[insight.level] || insight.level}</span>
              <h3>{insight.title}</h3>
            </div>
            <p>{insight.detail}</p>
            {insight.node_ids && insight.node_ids.length > 0 && (
              <div className="insightLinks">
                {insight.node_ids.slice(0, 4).map((id) => {
                  const node = graph.nodes.find((n) => n.id === id);
                  if (!node) return null;
                  return (
                    <button key={id} onClick={() => onSelectNode(node)}>
                      {node.label}
                    </button>
                  );
                })}
              </div>
            )}
          </article>
        ))}
      </div>

      <div className="panelFoot">
        <div className="legendTitle">图例</div>
        <div className="legendGrid">
          <Legend color="#0e6253" label="产品" />
          <Legend color="#a9c9ba" label="模块" />
          <Legend color="#ffffff" label="工序" border="#c3d6cc" />
          <Legend color="#ffffff" label="供应商" border="#2f7d63" />
          <Legend color="#d99b2b" label="同工艺可替代" dashed />
          <Legend color="#7f9ec7" label="同城协同" dashed />
        </div>
      </div>
    </aside>
  );
}

function Stat({ label, value, tone }: { label: string; value: number; tone?: "warn" }) {
  return (
    <div className={`stat ${tone ? "statWarn" : ""}`}>
      <b>{value}</b>
      <span>{label}</span>
    </div>
  );
}

function Legend({ color, label, border, dashed }: { color: string; label: string; border?: string; dashed?: boolean }) {
  return (
    <div className="legendItem">
      {dashed ? (
        <span className="legendDash" style={{ borderColor: color }} />
      ) : (
        <span className="legendDot" style={{ background: color, borderColor: border || color }} />
      )}
      {label}
    </div>
  );
}

function ProductBody({ node, graph, onExpandProduct, onSelectNode }: { node: GraphNode; graph: Graph; onExpandProduct: (name: string) => void; onSelectNode: (node: GraphNode | null) => void }) {
  const modules = graph.nodes.filter((n) => n.type === "module" && n.parent_id === node.id);
  const isMesh = graph.mode === "mesh";
  return (
    <>
      <p className="panelSummary">{node.meta?.summary}</p>
      {isMesh && (
        <button className="primaryButton block" onClick={() => onExpandProduct(node.label)}>
          展开 {node.label} 的供应链网络 →
        </button>
      )}
      {!isMesh && (
        <>
          <div className="panelSectionTitle">模块构成</div>
          <div className="chipList">
            {modules.map((module) => (
              <button key={module.id} className="chip" onClick={() => onSelectNode(module)}>
                {module.label}
              </button>
            ))}
          </div>
        </>
      )}
    </>
  );
}

function ModuleBody({ node, graph, onSelectNode }: { node: GraphNode; graph: Graph; onSelectNode: (node: GraphNode | null) => void }) {
  const processes = graph.nodes.filter((n) => n.type === "process" && n.parent_id === node.id);
  return (
    <>
      <p className="panelSummary">{node.meta?.desc}</p>
      <div className="panelSectionTitle">包含工序</div>
      <div className="rowList">
        {processes.map((process) => (
          <button key={process.id} className="rowItem" onClick={() => onSelectNode(process)}>
            <div>
              <b>{process.label}</b>
              <small>{process.subtitle}</small>
            </div>
            <span className="rowTag">{process.meta?.critical === "true" ? "关键" : "常规"}</span>
          </button>
        ))}
      </div>
    </>
  );
}

function ProcessBody({ node, graph, onSelectNode }: { node: GraphNode; graph: Graph; onSelectNode: (node: GraphNode | null) => void }) {
  const suppliers = graph.edges
    .filter((edge) => edge.source === node.id && edge.relation === "supply")
    .map((edge) => graph.nodes.find((n) => n.id === edge.target))
    .filter((n): n is GraphNode => Boolean(n))
    .sort((a, b) => (b.score || 0) - (a.score || 0));

  return (
    <>
      <div className="specBox">
        <div className="specRow">
          <span>工艺能力</span>
          <b>{node.meta?.capability_name}</b>
        </div>
        <div className="specRow">
          <span>单机用量</span>
          <b>{node.meta?.qty || "按图确定"}</b>
        </div>
        <div className="specRow">
          <span>在库供应商</span>
          <b>{node.meta?.available} 家</b>
        </div>
        {node.meta?.critical === "true" && <div className="specFlag">关键工序 · 直接影响量产</div>}
      </div>
      <div className="panelSectionTitle">技术要求</div>
      <p className="panelText">{node.meta?.spec}</p>
      <div className="panelSectionTitle">可承接的供应商</div>
      <div className="rowList">
        {suppliers.map((supplier) => (
          <button key={supplier.id} className="rowItem" onClick={() => onSelectNode(supplier)}>
            <div>
              <b>{supplier.label}</b>
              <small>
                {supplier.meta?.city} · 法人 {supplier.meta?.legal_representative} · {supplier.meta?.employees} 人
              </small>
            </div>
            <span className={`rowTag risk-${supplier.risk}`}>{supplier.score}</span>
          </button>
        ))}
      </div>
    </>
  );
}

function CapabilityBody({ node, graph, onSelectNode }: { node: GraphNode; graph: Graph; onSelectNode: (node: GraphNode | null) => void }) {
  const products = graph.edges
    .filter((edge) => edge.target === node.id && edge.relation === "uses")
    .map((edge) => graph.nodes.find((n) => n.id === edge.source))
    .filter((n): n is GraphNode => Boolean(n));

  return (
    <>
      <p className="panelSummary">{node.meta?.desc}</p>
      <div className="specBox">
        <div className="specRow">
          <span>能力类别</span>
          <b>{node.meta?.category}</b>
        </div>
        <div className="specRow">
          <span>全网供应商</span>
          <b>{node.meta?.suppliers} 家</b>
        </div>
        <div className="specRow">
          <span>主产业带</span>
          <b>{node.meta?.regions}</b>
        </div>
      </div>
      <div className="panelSectionTitle">使用该工艺的产品</div>
      <div className="chipList">
        {products.map((product) => (
          <button key={product.id} className="chip" onClick={() => onSelectNode(product)}>
            {product.label}
          </button>
        ))}
      </div>
    </>
  );
}

export function SupplierDossier({
  supplier,
  graph,
  onSelectNode,
  tenant,
  brief,
  onBriefChanged,
}: {
  supplier: Supplier;
  graph?: Graph;
  onSelectNode?: (node: GraphNode | null) => void;
  tenant?: string;
  brief?: SupplierBrief;
  onBriefChanged?: () => void;
}) {
  const [checklist, setChecklist] = useState<Checklist | null>(null);
  const [busy, setBusy] = useState(false);
  const [copied, setCopied] = useState(false);
  const [flash, setFlash] = useState("");

  useEffect(() => {
    setChecklist(null);
  }, [supplier.id]);

  const alternatives = (graph?.edges || [])
    .filter((edge) => edge.relation === "alternative" && (edge.source === supplier.id || edge.target === supplier.id))
    .map((edge) => graph?.nodes.find((n) => n.id === (edge.source === supplier.id ? edge.target : edge.source)))
    .filter((n): n is GraphNode => Boolean(n) && n?.type === "supplier");

  async function makeChecklist() {
    setBusy(true);
    try {
      setChecklist(await fetchChecklist(supplier.id));
    } finally {
      setBusy(false);
    }
  }

  const checklistText =
    checklist?.sections.map((section) => `【${section.category}】\n${section.items.map((item) => `□ ${item.title}｜${item.description}`).join("\n")}`).join("\n\n") || "";

  async function copyChecklist() {
    await navigator.clipboard.writeText(checklistText);
    setCopied(true);
    setTimeout(() => setCopied(false), 1600);
  }

  // 真实数据经常整块缺失（没有股权结构、没有证书、没有客户），
  // 这里统一归一成数组/空串，缺的部分直接不渲染，而不是显示 0 或空白。
  const shareholders = supplier.shareholders ?? [];
  const personnel = supplier.key_personnel ?? [];
  const certifications = supplier.certifications ?? [];
  const customers = supplier.customers ?? [];
  const capabilityNames = supplier.capability_names ?? [];
  const riskTags = supplier.risk_tags ?? [];
  const businessStatus = supplier.business_status || "";

  return (
    <>
      <div className="dossierTags">
        <span className="tagSolid">{supplier.is_manufacturer ? "生产型" : "贸易/服务"}</span>
        <span className={`tagRisk risk-${supplier.risk_level}`}>{RISK_LABEL[supplier.risk_level]}</span>
        {supplier.tax_rating === "A" && <span className="tagSoft">纳税 A 级</span>}
        {supplier.import_export && <span className="tagSoft">进出口权</span>}
      </div>

      {/* 闭环入口：先看平台上别人用下来的评分，再决定要不要拉进自己的 SRM */}
      <div className="loopStrip">
        <div className="loopRating">
          <RatingChip brief={brief} />
        </div>
        <div className="loopAction">
          <EngageControl
            supplierId={supplier.id}
            supplierName={supplier.short_name || supplier.name}
            tenant={tenant || ""}
            brief={brief}
            onChanged={(message) => {
              setFlash(message);
              setTimeout(() => setFlash(""), 2600);
              onBriefChanged?.();
            }}
          />
        </div>
        {flash && <p className="loopFlash">{flash}</p>}
        <p className="loopHint">
          引入后在本企业 SRM 里录订单与交付，这些单据会算出该供应商的 QCDST，
          并与其他企业的评估一起加权回流到这里的平台评分。
        </p>
      </div>

      {/* 数据来源必须跟着档案一起展示：这条记录是从哪来的、什么时候核的、能不能回查 */}
      <div className="sourceStrip">
        <span>
          数据来源 <b>{supplier.data_source || "未标注"}</b>
        </span>
        {supplier.last_verified_at && <span>核验于 {supplier.last_verified_at.slice(0, 10)}</span>}
        {supplier.source_url && (
          <a href={supplier.source_url} target="_blank" rel="noreferrer">
            查看原始记录 ↗
          </a>
        )}
        {!supplier.data_source && <span className="stale">该记录未标注来源，请勿直接采信</span>}
      </div>

      <div className="dossierHero">
        <div>
          <small>法定代表人</small>
          <b>{supplier.legal_representative || "—"}</b>
          <em>{supplier.legal_rep_title}</em>
        </div>
        <div>
          <small>注册资本</small>
          <b>{money(supplier.registered_capital)}</b>
          <em>{supplier.paid_capital ? `实缴 ${money(supplier.paid_capital)}` : "实缴未披露"}</em>
        </div>
        <div>
          <small>成立时间</small>
          <b>{supplier.establish_date || "—"}</b>
          <em>{businessStatus.replace(/（.*?）/, "") || "—"}</em>
        </div>
      </div>

      <div className="panelSectionTitle">工商登记信息</div>
      <dl className="kvGrid">
        <KV k="统一社会信用代码" v={supplier.credit_code} mono />
        <KV k="企业类型" v={supplier.enterprise_type} />
        <KV k="经营状态" v={supplier.business_status} />
        <KV k="纳税信用等级" v={supplier.tax_rating ? `${supplier.tax_rating} 级` : ""} />
        <KV k="参保人数" v={supplier.insured_count ? `${supplier.insured_count} 人` : ""} />
        <KV k="员工总数" v={supplier.employee_count ? `${supplier.employee_count} 人` : ""} />
        <KV k="注册地址" v={supplier.registered_address} wide />
        <KV k="实际经营地址" v={supplier.office_address} wide />
        <KV k="年营业收入" v={supplier.annual_revenue ? `约 ${money(supplier.annual_revenue)}` : ""} />
        <KV k="厂房面积" v={supplier.factory_area ? `${supplier.factory_area.toLocaleString("zh-CN")} ㎡` : ""} />
        <KV k="专利" v={supplier.invention_patents || supplier.utility_patents ? `发明 ${supplier.invention_patents} / 实用新型 ${supplier.utility_patents}` : ""} />
        <KV k="分支机构" v={supplier.has_branches ? "有" : ""} />
      </dl>

      {shareholders.length > 0 && (
        <>
          <div className="panelSectionTitle">股权结构</div>
          <div className="shareholders">
            {shareholders.map((holder) => (
          <div className="holder" key={holder.name + holder.ratio}>
            <div className="holderTop">
              <span>
                {holder.name}
                <em>{holder.type}</em>
              </span>
              <b>{holder.ratio}%</b>
            </div>
            <div className="holderBar">
              <i style={{ width: `${Math.min(100, holder.ratio)}%` }} />
            </div>
                <small>认缴 {money(holder.subscribed)}</small>
              </div>
            ))}
          </div>
        </>
      )}

      {personnel.length > 0 && (
        <>
          <div className="panelSectionTitle">主要人员</div>
          <div className="people">
            {personnel.map((person) => (
          <div className="person" key={person.name + person.title}>
            <span>{person.name.slice(0, 1)}</span>
                <div>
                  <b>{person.name}</b>
                  <small>{person.title}</small>
                </div>
              </div>
            ))}
          </div>
        </>
      )}

      <div className="panelSectionTitle">工艺能力</div>
      <div className="chipList">
        {capabilityNames.map((name) => (
          <span className="chip static" key={name}>
            {name}
          </span>
        ))}
      </div>
      {supplier.capacity_description && <p className="panelText">{supplier.capacity_description}</p>}

      <div className="miniGrid">
        <div>
          <small>最小起订量</small>
          <b>{supplier.moq ? `${supplier.moq.toLocaleString("zh-CN")} 件` : "—"}</b>
        </div>
        <div>
          <small>打样交期</small>
          <b>{supplier.lead_time_days ? `${supplier.lead_time_days} 天` : "—"}</b>
        </div>
        <div>
          <small>研发人员</small>
          <b>{supplier.rnd_staff ? `${supplier.rnd_staff} 人` : "—"}</b>
        </div>
      </div>

      {certifications.length > 0 && (
        <>
          <div className="panelSectionTitle">资质证书</div>
          <div className="certList">
            {certifications.map((cert) => (
          <div className="cert" key={cert.name}>
            <b>{cert.name}</b>
            <small>
              {cert.issuer} · 有效期至 {cert.valid_until}
            </small>
                <em>{cert.scope}</em>
              </div>
            ))}
          </div>
        </>
      )}

      {customers.length > 0 && (
        <>
          <div className="panelSectionTitle">品牌客户</div>
          <div className="chipList">
            {customers.map((customer) => (
              <span className="chip static" key={customer}>
                {customer}
              </span>
            ))}
          </div>
        </>
      )}

      {riskTags.length > 0 && (
        <>
          <div className="panelSectionTitle">风险标签</div>
          <div className="chipList">
            {riskTags.map((tag) => (
              <span className={`chip static risk-${supplier.risk_level}`} key={tag}>
                {tag}
              </span>
            ))}
          </div>
        </>
      )}

      {alternatives.length > 0 && (
        <>
          <div className="panelSectionTitle">同工艺可替代</div>
          <div className="rowList compact">
            {alternatives.map((node) => (
              <button key={node.id} className="rowItem" onClick={() => onSelectNode?.(node)}>
                <div>
                  <b>{node.label}</b>
                  <small>
                    {node.meta?.city} · 法人 {node.meta?.legal_representative}
                  </small>
                </div>
                <span className="rowTag">{node.score}</span>
              </button>
            ))}
          </div>
        </>
      )}

      <div className="contactBox">
        <div>
          <small>联系人</small>
          <b>{supplier.contact_name}</b>
        </div>
        <div>
          <small>电话</small>
          <b>{supplier.contact_phone}</b>
        </div>
        <div>
          <small>邮箱</small>
          <b>{supplier.contact_email}</b>
        </div>
      </div>

      <div className="actionRow">
        <button className="primaryButton" onClick={() => void makeChecklist()} disabled={busy}>
          {busy ? "生成中…" : "生成现场审核清单"}
        </button>
        <span className="dataNote">{supplier.data_source} · 核验于 {supplier.last_verified_at}</span>
      </div>

      {checklist && (
        <div className="checklist">
          <div className="checklistHead">
            <h3>现场审核清单</h3>
            <button onClick={() => void copyChecklist()}>{copied ? "已复制" : "复制清单"}</button>
          </div>
          <p>{checklist.summary}</p>
          {checklist.sections.map((section) => (
            <div className="section" key={section.category}>
              <h4>{section.category}</h4>
              {section.items.map((item) => (
                <div className="checkItem" key={item.title}>
                  <span>□</span>
                  <div>
                    <b>{item.title}</b>
                    <p>{item.description}</p>
                  </div>
                  <small>{item.priority}</small>
                </div>
              ))}
            </div>
          ))}
        </div>
      )}
    </>
  );
}

function KV({ k, v, mono, wide }: { k: string; v: string; mono?: boolean; wide?: boolean }) {
  return (
    <div className={wide ? "kv wide" : "kv"}>
      <dt>{k}</dt>
      <dd className={mono ? "mono" : ""}>{v || "—"}</dd>
    </div>
  );
}
