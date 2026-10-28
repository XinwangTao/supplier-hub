"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import {
  ENGAGEMENT_STATUS,
  fetchDatasetMeta,
  fetchPartSupply,
  fetchProductSupplyMatrix,
  fetchSupplierParts,
  type CoverageRisk,
  type DatasetMeta,
  type PartCoverage,
  type PartSupply,
  type ProductSupplyMatrix,
  type SupplierPartCoverage,
} from "@/lib/api";
import { readTenant, subscribeTenant } from "@/lib/tenant";
import { EngageControl } from "./SourcingActions";
import { GradeBadge } from "./Qcdst";

// BOM 供应源视图：从"要什么料"直接走到"找谁买"。
//
// 展示顺序刻意是先覆盖、后候选：
//   一个小类只有 1 家能供，那家分再高也是单点风险。先让用户看见风险分布，
//   再点开看具体供应商，否则很容易被一个高分供应商掩盖掉断供敞口。

export const RISK_LABEL: Record<CoverageRisk, string> = {
  none: "缺口",
  single: "单点",
  narrow: "偏窄",
  adequate: "充足",
};

const RISK_ORDER: Record<CoverageRisk, number> = { none: 0, single: 1, narrow: 2, adequate: 3 };

export function RiskTag({ risk, label }: { risk: CoverageRisk; label?: string }) {
  return <span className={`riskTag rk-${risk}`}>{label || RISK_LABEL[risk] || risk}</span>;
}

export function ProductSupplyPanel({
  productCode,
  productName,
}: {
  productCode: string;
  productName: string;
}) {
  const [tenant, setTenant] = useState("");
  const [region, setRegion] = useState("");
  const [regionInput, setRegionInput] = useState("");
  const [matrix, setMatrix] = useState<ProductSupplyMatrix | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [openCat, setOpenCat] = useState<string | null>(null);
  const [flash, setFlash] = useState("");
  const [dataset, setDataset] = useState<DatasetMeta | null>(null);

  useEffect(() => {
    setTenant(readTenant());
    return subscribeTenant(setTenant);
  }, []);

  useEffect(() => {
    fetchDatasetMeta()
      .then(setDataset)
      .catch(() => undefined);
  }, []);

  useEffect(() => {
    let alive = true;
    setLoading(true);
    fetchProductSupplyMatrix(productCode, tenant, region)
      .then((data) => {
        if (!alive) return;
        setMatrix(data);
        setError("");
      })
      .catch((err) => alive && setError(err instanceof Error ? err.message : "加载失败"))
      .finally(() => alive && setLoading(false));
    return () => {
      alive = false;
    };
  }, [productCode, tenant, region]);

  if (loading && !matrix) return <div className="emptyBlock">正在核算供应覆盖…</div>;
  if (error) return <div className="errBanner">{error}</div>;
  if (!matrix) return <div className="emptyBlock">暂无数据</div>;

  const s = matrix.summary;

  return (
    <div className="supplyPanel">
      <div className="supplyHead">
        <div className="supplyHeadline">
          <b>{productName} 的供应覆盖体检</b>
          <p>{s.headline}</p>
        </div>
        <div className="supplyActions">
          <input
            className="regionInput"
            placeholder="区域（如 深圳市 / 广东省）"
            value={regionInput}
            onChange={(e) => setRegionInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") setRegion(regionInput.trim());
            }}
          />
          <button className="btn ghost sm" onClick={() => setRegion(regionInput.trim())}>
            按区域重算
          </button>
          {region && (
            <button
              className="btn ghost sm"
              onClick={() => {
                setRegionInput("");
                setRegion("");
              }}
            >
              清除区域
            </button>
          )}
        </div>
      </div>

      <div className="supplyStats">
        <Stat label="涉及小类" value={s.categories} unit="类" />
        <Stat label="有第二源的小类" value={`${s.coverage_rate.toFixed(0)}%`} />
        <Stat label="缺供小类" value={s.gaps} unit="类" tone={s.gaps > 0 ? "bad" : "ok"} />
        <Stat label="单点小类" value={s.single_points} unit="类" tone={s.single_points > 0 ? "warn" : "ok"} />
        <Stat label="关键件卡脖子项" value={s.critical_gaps} unit="项" tone={s.critical_gaps > 0 ? "bad" : "ok"} />
      </div>

      {flash && <p className="loopFlash">{flash}</p>}

      {/* 空池必须说清楚原因：全是"缺口"不是算错了，是公开池里确实还没接入真实供应商 */}
      {dataset && dataset.count === 0 && (
        <div className="warnBanner">
          <b>公开池暂无供应商</b>
          <span>
            当前没有接入任何真实供应商档案，所以每个小类都会判为"缺口"。
            导入真实数据（CSV 或企业信息接口）后，这张表才会反映真实的供应覆盖情况。
          </span>
        </div>
      )}

      <div className="supplyGroups">
        {matrix.groups.map((g) => (
          <div key={g.code} className={`supplyGroup ${g.risk === "none" || g.risk === "single" ? "risky" : ""}`}>
            <div className="supplyGroupHead">
              <div>
                <b>{g.name}</b>
                <small>{g.role || "—"}</small>
              </div>
              <RiskTag risk={g.risk} />
            </div>
            <table className="supplyTable">
              <thead>
                <tr>
                  <th>部件小类</th>
                  <th style={{ width: 80 }}>覆盖</th>
                  <th style={{ width: 70 }}>可选</th>
                  <th style={{ width: 130 }}>当前最优</th>
                  <th style={{ width: 90 }} />
                </tr>
              </thead>
              <tbody>
                {g.coverages.map((c) => (
                  <CoverageRow
                    key={`${g.code}-${c.category.code}-${c.parts}`}
                    coverage={c}
                    open={openCat === c.category.code}
                    onToggle={() => setOpenCat(openCat === c.category.code ? null : c.category.code)}
                  >
                    {openCat === c.category.code && (
                      <CandidateList
                        productCode={productCode}
                        category={c.category.code}
                        tenant={tenant}
                        region={region}
                        onChanged={setFlash}
                      />
                    )}
                  </CoverageRow>
                ))}
              </tbody>
            </table>
          </div>
        ))}
      </div>

      <p className="footNote">
        覆盖等级按公开池中对口供应商家数判定：0 家为缺口、1 家为单点、2-3 家为偏窄、4 家及以上为充足。
        关键件单点会单独计入"卡脖子项"——它决定量产前要不要先备第二源。
      </p>
    </div>
  );
}

function Stat({ label, value, unit, tone }: { label: string; value: number | string; unit?: string; tone?: "ok" | "warn" | "bad" }) {
  return (
    <div className={`supplyStat ${tone ? `t-${tone}` : ""}`}>
      <span>
        <b>{value}</b>
        {unit && <em>{unit}</em>}
      </span>
      <small>{label}</small>
    </div>
  );
}

function CoverageRow({
  coverage,
  open,
  onToggle,
  children,
}: {
  coverage: PartCoverage;
  open: boolean;
  onToggle: () => void;
  children?: React.ReactNode;
}) {
  const c = coverage;
  return (
    <>
      <tr className={open ? "rowOpen" : undefined} onClick={onToggle}>
        <td>
          <b>{c.category.name}</b>
          {c.critical && <em className="critDot" title="该小类含关键件">关键</em>}
          <small>{c.category.description}</small>
        </td>
        <td>
          <RiskTag risk={c.risk} label={c.risk_label} />
        </td>
        <td className="mono">
          {c.suppliers}
          {c.rated > 0 && <small className="dim"> / 评 {c.rated}</small>}
        </td>
        <td>
          {c.suppliers === 0 ? (
            <span className="dimText">—</span>
          ) : (
            <span className="bestCell">
              <b>{c.top_supplier || "—"}</b>
              <small>
                {c.best_score >= 0 ? `平台 ${c.best_score.toFixed(1)}` : `适配 ${c.best_fit.toFixed(0)}`}
              </small>
            </span>
          )}
        </td>
        <td className="rowToggle">{c.suppliers > 0 ? (open ? "收起 ▲" : "看候选 ▼") : "—"}</td>
      </tr>
      {open && c.suppliers > 0 && (
        <tr className="detailRow">
          <td colSpan={5}>{children}</td>
        </tr>
      )}
    </>
  );
}

function CandidateList({
  productCode,
  category,
  tenant,
  region,
  onChanged,
}: {
  productCode: string;
  category: string;
  tenant: string;
  region: string;
  onChanged: (msg: string) => void;
}) {
  const [data, setData] = useState<PartSupply | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    let alive = true;
    setLoading(true);
    fetchPartSupply({ category, tenant, region, product: productCode })
      .then((res) => alive && setData(res))
      .catch((err) => alive && setError(err instanceof Error ? err.message : "加载失败"))
      .finally(() => alive && setLoading(false));
    return () => {
      alive = false;
    };
  }, [category, tenant, region, productCode]);

  if (loading) return <div className="panelLoading">正在匹配候选供应商…</div>;
  if (error) return <div className="errBanner">{error}</div>;
  if (!data) return null;

  return (
    <div className="candidateBox">
      <div className="candidateHead">
        <b>{data.coverage.category.name}：{data.coverage.suppliers} 家对口供应商</b>
        <span className="dimText">{data.coverage.advice}</span>
      </div>

      {data.demand_parts.length > 0 && (
        <div className="demandStrip">
          <em>本产品该小类下的部件：</em>
          {data.demand_parts.map((p) => (
            <span key={p.code} className="demandChip">
              {p.name}
              {p.critical && <i title="关键件">关键</i>}
              <small>×{p.qty}</small>
            </span>
          ))}
        </div>
      )}

      <div className="candidateList">
        {data.items.map((m) => (
          <div key={m.supplier_id} className="candidateCard">
            <div className="candMain">
              <div className="candTitle">
                <Link href={`/vendor/${encodeURIComponent(m.supplier_id)}${tenant ? `?tenant=${tenant}` : ""}`}>
                  {m.short_name || m.name}
                </Link>
                <span className="regionTag">{m.region_tag === "未指定" ? m.city || "—" : `${m.region_tag} · ${m.city}`}</span>
                {m.is_manufacturer ? <span className="tagMini">生产型</span> : <span className="tagMini warnTag">贸易/服务</span>}
                {m.lead_time_days > 0 && <span className="tagMini">交期 {m.lead_time_days} 天</span>}
              </div>
              <div className="candReasons">
                {m.fit_reasons.slice(0, 4).map((r, i) => (
                  <span key={i}>{r}</span>
                ))}
              </div>
            </div>
            <div className="candScores">
              <div className="fitScore">
                <b>{m.fit_score.toFixed(0)}</b>
                <small>适配分</small>
              </div>
              <div className="candRating">
                {m.rating ? (
                  <>
                    <GradeBadge grade={m.rating.grade} provisional={m.rating.provisional} size="sm" />
                    <b>{m.rating.total.toFixed(1)}</b>
                    <small>
                      {m.rating.rated_buyers} 家评过 · {m.rating.batch_count} 批
                    </small>
                  </>
                ) : (
                  <span className="dimText">暂无评级</span>
                )}
              </div>
            </div>
            <div className="candAction">
              {m.engagement ? (
                <span className={`statusChip st-${m.engagement.status}`}>
                  {ENGAGEMENT_STATUS[m.engagement.status] || m.engagement.status}
                </span>
              ) : null}
              <EngageControl
                supplierId={m.supplier_id}
                supplierName={m.short_name || m.name}
                tenant={tenant}
                category={data.coverage.category.name}
                onChanged={(msg) => onChanged(msg)}
              />
            </div>
          </div>
        ))}
      </div>

      <p className="footNote">{data.note}</p>
    </div>
  );
}

// 供应商反查：这家能覆盖哪些小类。用于判断它在某条产品线上的通用性。
export function SupplierPartsPanel({ supplierId }: { supplierId: string }) {
  const [data, setData] = useState<SupplierPartCoverage | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let alive = true;
    fetchSupplierParts(supplierId)
      .then((res) => alive && setData(res))
      .catch((err) => alive && setError(err instanceof Error ? err.message : "加载失败"));
    return () => {
      alive = false;
    };
  }, [supplierId]);

  if (error) return null;
  if (!data) return null;
  if (data.categories.length === 0) return null;

  return (
    <section className="card">
      <div className="cardHead">
        <h3>可供应部件范围</h3>
        <span className="dimText">由其登记工艺能力推导，不是自报</span>
      </div>
      <p className="cellNote">{data.summary.headline}</p>
      {data.summary.indirect_parts > 0 && (
        <p className="cellNote dim">
          「需外协」的部件只说明它的工艺与这类料相关，不等于能自己生产 —— 打样前要确认实际产线归属。
        </p>
      )}
      <div className="coverGrid">
        {data.categories.map((c) => (
          <div key={c.category.code} className={`coverCell rk-${c.risk}`}>
            <b>{c.category.name}</b>
            <small>{c.parts} 个部件可覆盖</small>
            <RiskTag risk={c.risk} />
          </div>
        ))}
      </div>
      {data.parts.length > 0 && (
        <details className="partDetails">
          <summary>看具体部件（{data.summary.parts} 个，关键件优先）</summary>
          <table className="grid">
            <thead>
              <tr>
                <th>部件</th>
                <th>所属产品</th>
                <th>模块</th>
                <th style={{ width: 190 }}>命中依据</th>
                <th style={{ width: 56 }}>用量</th>
              </tr>
            </thead>
            <tbody>
              {data.parts.map((p, i) => (
                <tr key={`${p.part.code}-${i}`}>
                  <td>
                    {p.part.name}
                    {p.part.critical && <em className="critDot">关键</em>}
                  </td>
                  <td>{p.product_name}</td>
                  <td>{p.group_name}</td>
                  <td>
                    <span className={p.direct ? "basisTag direct" : "basisTag indirect"} title={p.basis}>
                      {p.direct ? "可自制" : "需外协"}
                    </span>
                    <small className="dim">{p.basis}</small>
                  </td>
                  <td className="mono">{p.part.qty}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </details>
      )}
    </section>
  );
}

export { RISK_ORDER };
