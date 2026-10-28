"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import NavBar from "@/components/NavBar";
import { DimensionBar, GradeBadge, MetricTable, QcdstRadar } from "@/components/Qcdst";
import { EngageControl, RatingChip } from "@/components/SourcingActions";
import { SupplierPartsPanel } from "@/components/BomSupply";
import {
  fetchBriefs,
  fetchOrderScorecards,
  fetchQcdst,
  type OrderScorecard,
  type OrderScorecardsResult,
  type QCDSTAssessment,
  type SupplierBrief,
} from "@/lib/api";
import { readTenant, subscribeTenant } from "@/lib/tenant";

const DIM_LABEL: Record<string, string> = { Q: "质", C: "本", D: "交", S: "服", T: "技" };

export default function VendorPage() {
  const params = useParams<{ id: string }>();
  const id = decodeURIComponent(String(params?.id || ""));
  const [data, setData] = useState<QCDSTAssessment | null>(null);
  const [orders, setOrders] = useState<OrderScorecardsResult | null>(null);
  const [brief, setBrief] = useState<SupplierBrief | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [tenant, setTenant] = useState("");

  const load = useCallback(
    (tenantId: string) => {
      if (!id) return;
      setLoading(true);
      Promise.all([
        fetchQcdst(id, tenantId),
        fetchOrderScorecards(id, tenantId).catch(() => null),
        fetchBriefs(tenantId)
          .then((index) => index.items?.[id] || null)
          .catch(() => null),
      ])
        .then(([assessment, cards, brief]) => {
          setData(assessment);
          setOrders(cards);
          setBrief(brief);
          setError("");
        })
        .catch((err) => setError(err instanceof Error ? err.message : "加载失败"))
        .finally(() => setLoading(false));
    },
    [id],
  );

  useEffect(() => {
    const fromUrl = new URLSearchParams(window.location.search).get("tenant") || readTenant() || "";
    setTenant(fromUrl);
    load(fromUrl);
    return subscribeTenant((next) => {
      setTenant(next);
      load(next);
    });
  }, [load]);

  return (
    <div className="console">
      <NavBar />
      <main className="consoleBody">
        {loading && <div className="emptyBlock">正在计算 QCDST…</div>}
        {error && <div className="errBanner">{error}</div>}

        {data && (
          <>
            <div className="vendorHead">
              <div className="vendorTitle">
                <Link href="/srm" className="backLink">
                  ← 返回供应商管理
                </Link>
                <h1>{data.supplier_name}</h1>
                <div className="vendorMeta">
                  <span className="mono">{data.credit_code || "无信用代码"}</span>
                  <span>评估方：{data.tenant_name || "平台汇总"}</span>
                  <span>统计窗口：{data.window}</span>
                  <span>生成于 {data.generated_at}</span>
                </div>
              </div>
              <div className="scoreBlock">
                <GradeBadge grade={data.grade} provisional={data.provisional} size="lg" />
                <div className="scoreNumber">
                  <b>{data.total >= 0 ? data.total.toFixed(1) : "—"}</b>
                  <span>/ 100</span>
                </div>
                <div className="scoreSub">
                  <span>置信度 {(data.confidence * 100).toFixed(0)}%</span>
                  <span>指标覆盖 {(data.coverage * 100).toFixed(0)}%</span>
                </div>
              </div>
            </div>

            {/* 闭环状态：左边是平台反哺回来的公开评级，右边是"我"的私有合作状态 */}
            <section className="card loopCard">
              <div className="cardHead">
                <h2>寻源 ⇄ SRM 闭环状态</h2>
                <span className="cardHint">左侧是平台公开评级（多家企业数据加权），右侧是本企业的私有合作状态</span>
              </div>
              <div className="loopGrid">
                <div className="loopCell">
                  <span className="cellLabel">平台评级（公开）</span>
                  <RatingChip brief={brief || undefined} />
                </div>
                <div className="loopCell">
                  <span className="cellLabel">本企业合作状态</span>
                  <EngageControl
                    supplierId={id}
                    supplierName={data.short_name || data.supplier_name}
                    tenant={tenant}
                    brief={brief || undefined}
                    onChanged={() => load(tenant)}
                  />
                  {brief?.in_srm && (
                    <p className="cellNote">
                      {brief.since ? `自 ${brief.since} 起 · ` : ""}本企业已录 {brief.my_order_count} 张订单、
                      {brief.my_batch_count} 个交付批次
                    </p>
                  )}
                </div>
              </div>
            </section>

            {data.provisional && (
              <div className="warnBanner">
                <b>观察期</b>
                <span>
                  该供应商只有 {data.batch_count} 个交付批次，未达到出正式评级所需的 15 个。当前分数仅供参考，
                  不参与平台排名 —— 小样本的高分和低分都不该被当成结论。
                </span>
              </div>
            )}
            {data.demo && (
              <div className="demoBanner">
                <b>演示数据</b>
                <span>本页单据为演示用途，不是真实业务记录。</span>
              </div>
            )}

            <div className="vendorGrid">
              <section className="card radarCard">
                <div className="cardHead">
                  <h2>能力画像</h2>
                  <span className="cardHint">虚线环为 25/50/75/100 分位</span>
                </div>
                <QcdstRadar dimensions={data.dimensions} total={data.total} />
                <div className="radarLegend">
                  <span>订单 {data.order_count} 张</span>
                  <span>批次 {data.batch_count} 个</span>
                  <span>质量事件 {data.quality_events} 起</span>
                </div>
              </section>

              <section className="card">
                <div className="cardHead">
                  <h2>五维得分</h2>
                  <span className="cardHint">条末斜纹表示该维指标覆盖不全</span>
                </div>
                <div className="dimList">
                  {data.dimensions.map((d) => (
                    <DimensionBar key={d.dimension} dimension={d} />
                  ))}
                </div>
              </section>
            </div>

            <SupplierPartsPanel supplierId={id} />

            {orders && <OrderScorecards result={orders} />}

            {data.insights.length > 0 && (
              <section className="insightRow">
                {data.insights.map((insight, i) => (
                  <div key={`${insight.kind}-${i}`} className={`insightCard lv-${insight.level}`}>
                    <b>{insight.title}</b>
                    <p>{insight.detail}</p>
                  </div>
                ))}
              </section>
            )}

            {data.history.length > 1 && (
              <section className="card">
                <div className="cardHead">
                  <h2>月度表现</h2>
                  <span className="cardHint">按月的质量 + 交付合成分，用来判断是在变好还是变差</span>
                </div>
                <TrendChart points={data.history} />
              </section>
            )}

            <section className="card">
              <div className="cardHead">
                <h2>指标明细与证据链</h2>
                <span className="cardHint">展开任一指标，可以看到分子、分母、样本量与可回查的单据号</span>
              </div>
              <div className="metricGroups">
                {data.dimensions.map((d) => (
                  <div key={d.dimension} className="metricGroup">
                    <div className="metricGroupHead">
                      <span>{d.name}</span>
                      <em>权重 {(d.weight * 100).toFixed(0)}%</em>
                      {d.score < 0 ? <i className="missing">证据不足</i> : <i>{d.score.toFixed(1)}</i>}
                    </div>
                    <MetricTable metrics={d.metrics} />
                  </div>
                ))}
              </div>
            </section>

            <p className="footNote">
              评分口径：Q 30% / C 20% / D 25% / S 15% / T 10%（工业装备类会自动调整权重，技术维度提到 20%）。
              所有分数由 SRM 单据实时计算，不含人工调节；人工评审项（协同配合、工序覆盖、自动化水平）会标注来源评审人。
            </p>
          </>
        )}
      </main>
    </div>
  );
}

// 逐单评分卡：这张单执行得怎么样。用于对账与追责，不是评级依据。
function OrderScorecards({ result }: { result: OrderScorecardsResult }) {
  const [open, setOpen] = useState<string | null>(null);
  if (!result.cards || result.cards.length === 0) {
    return (
      <section className="card">
        <div className="cardHead">
          <h2>逐单 QCDST 评分卡</h2>
        </div>
        <div className="emptyBlock">
          <p>还没有订单。订单是 QCDST 的起点：下了单、录了交付与 IQC，这张评分卡才会出现。</p>
        </div>
      </section>
    );
  }
  return (
    <section className="card">
      <div className="cardHead">
        <h2>逐单 QCDST 评分卡</h2>
        <span className="cardHint">
          共 {result.summary.total} 张订单，其中 {result.summary.judged} 张评得出分数 · 成本基准：
          {result.benchmark.price > 0 ? `${result.benchmark.price.toFixed(2)}（${result.benchmark.basis}）` : "无基准，成本维不判"}
        </span>
      </div>

      <div className="orderSummaryRow">
        <div className="orderStat">
          <span>订单级均分</span>
          <b>{result.summary.average_score >= 0 ? result.summary.average_score.toFixed(1) : "—"}</b>
        </div>
        <div className="orderStat">
          <span>最差 / 最好</span>
          <b>
            {result.summary.worst_score >= 0 ? result.summary.worst_score.toFixed(0) : "—"} /{" "}
            {result.summary.best_score >= 0 ? result.summary.best_score.toFixed(0) : "—"}
          </b>
        </div>
        <div className="orderStat">
          <span>供应商综合分</span>
          <b className="accent">{result.assessment.total >= 0 ? result.assessment.total.toFixed(1) : "—"}</b>
        </div>
      </div>
      <p className="loopHint">{result.summary.caveat}</p>

      <table className="grid orderGrid">
        <thead>
          <tr>
            <th style={{ width: 150 }}>订单</th>
            <th style={{ width: 80 }}>数量</th>
            <th style={{ width: 130 }}>交付情况</th>
            <th>五维（灰 = 该单判不了）</th>
            <th style={{ width: 90 }}>单分</th>
            <th style={{ width: 70 }} />
          </tr>
        </thead>
        <tbody>
          {result.cards.map((card) => (
            <OrderRow
              key={card.order_id}
              card={card}
              open={open === card.order_id}
              onToggle={() => setOpen(open === card.order_id ? null : card.order_id)}
            />
          ))}
        </tbody>
      </table>
    </section>
  );
}

function OrderRow({ card, open, onToggle }: { card: OrderScorecard; open: boolean; onToggle: () => void }) {
  return (
    <>
      <tr className={open ? "rowOpen" : ""}>
        <td>
          <b className="mono">{card.po_no || card.order_id}</b>
          <br />
          <small>{card.part_name || card.category}</small>
          <br />
          <small className="dimText">{card.issued_at}</small>
          {card.expedited && <span className="tagSoft warnTag">加急</span>}
        </td>
        <td className="mono small">
          {card.qty.toLocaleString("zh-CN")} 件
          <br />
          <small className="dimText">¥{(card.amount / 10000).toFixed(1)} 万</small>
        </td>
        <td className="mono small">
          {card.batch_count > 0 ? `${card.batch_count} 批 · 准时 ${card.on_time_batches}` : <span className="dimText">未到货</span>}
          {card.rejected > 0 && (
            <>
              <br />
              <small className="warnText">不良 {card.rejected} 件</small>
            </>
          )}
        </td>
        <td>
          <div className="miniDims">
            {card.dimensions.map((d) => (
              <span
                key={d.dimension}
                className={`miniDim ${d.judged ? "" : "unjudged"}`}
                title={d.reason}
              >
                <em>{DIM_LABEL[d.dimension]}</em>
                <i>
                  <u style={{ width: `${!d.judged ? 0 : Math.max(3, d.score)}%` }} />
                </i>
                <b className={d.judged ? "" : "missing"}>{d.judged ? d.score.toFixed(0) : "—"}</b>
              </span>
            ))}
          </div>
          <small className="dimText">可判权重 {(card.judged_weight * 100).toFixed(0)}%</small>
        </td>
        <td>
          <span className={`bigScore ${card.total < 0 ? "missing" : ""}`}>{card.total >= 0 ? card.total.toFixed(1) : "—"}</span>
          <br />
          <GradeBadge grade={card.grade} size="sm" />
        </td>
        <td>
          <button className="linkBtn" onClick={onToggle}>
            {open ? "收起" : "依据"} {open ? "↑" : "↓"}
          </button>
        </td>
      </tr>
      {open && (
        <tr className="detailRow">
          <td colSpan={6}>
            {card.note && <p className="loopHint">{card.note}</p>}
            <div className="reasonGrid">
              {card.dimensions.map((d) => (
                <div key={d.dimension} className={`reasonCell ${d.judged ? "" : "unjudged"}`}>
                  <b>
                    {d.name} · 权重 {(d.weight * 100).toFixed(0)}%
                    {d.judged && <em>（本次归一后 {(d.effective_weight * 100).toFixed(0)}%）</em>}
                  </b>
                  <span className="mono small">{d.raw || "—"}</span>
                  <p>{d.reason}</p>
                </div>
              ))}
            </div>
          </td>
        </tr>
      )}
    </>
  );
}

function TrendChart({ points }: { points: { period: string; total: number; samples: number }[] }) {
  const w = 720;
  const h = 160;
  const pad = 28;
  const values = points.map((p) => p.total);
  const min = Math.min(...values, 60) - 4;
  const max = Math.max(...values, 100) + 2;
  const x = (i: number) => pad + (i / Math.max(1, points.length - 1)) * (w - pad * 2);
  const y = (v: number) => h - pad - ((v - min) / Math.max(1, max - min)) * (h - pad * 2);

  const path = points.map((p, i) => `${i === 0 ? "M" : "L"}${x(i)},${y(p.total)}`).join(" ");
  const area = `${path} L${x(points.length - 1)},${h - pad} L${x(0)},${h - pad} Z`;

  return (
    <svg className="trend" viewBox={`0 0 ${w} ${h}`} role="img" aria-label="月度评分趋势">
      <defs>
        <linearGradient id="trendFill" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#0e6253" stopOpacity="0.18" />
          <stop offset="100%" stopColor="#0e6253" stopOpacity="0" />
        </linearGradient>
      </defs>
      {[70, 80, 90].map((line) => (
        <g key={line}>
          <line x1={pad} y1={y(line)} x2={w - pad} y2={y(line)} stroke="#e2e9e5" strokeDasharray="3 4" />
          <text x={4} y={y(line) + 3} className="trendTick">
            {line}
          </text>
        </g>
      ))}
      <path d={area} fill="url(#trendFill)" />
      <path d={path} fill="none" stroke="#0e6253" strokeWidth={2} strokeLinejoin="round" />
      {points.map((p, i) => (
        <g key={p.period}>
          <circle cx={x(i)} cy={y(p.total)} r={3.4} fill="#fff" stroke="#0e6253" strokeWidth={1.8} />
          <text x={x(i)} y={h - 8} textAnchor="middle" className="trendTick">
            {p.period.slice(2)}
          </text>
          <text x={x(i)} y={y(p.total) - 8} textAnchor="middle" className="trendValue">
            {p.total.toFixed(0)}
          </text>
        </g>
      ))}
    </svg>
  );
}
