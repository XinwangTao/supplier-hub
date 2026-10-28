"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import NavBar from "@/components/NavBar";
import { GradeBadge } from "@/components/Qcdst";
import { fetchLoop, fetchRanking, type LoopReport, type PlatformRating } from "@/lib/api";
import { readTenant, subscribeTenant } from "@/lib/tenant";

export default function RankingPage() {
  const [items, setItems] = useState<PlatformRating[]>([]);
  const [meta, setMeta] = useState<Record<string, unknown> | null>(null);
  const [loop, setLoop] = useState<LoopReport | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [sort, setSort] = useState<"total" | "buyers" | "batches">("total");

  const loadLoop = useCallback((tenantId: string) => {
    fetchLoop(tenantId)
      .then(setLoop)
      .catch(() => setLoop(null));
  }, []);

  useEffect(() => {
    fetchRanking()
      .then((res) => {
        setItems(res.items || []);
        setMeta(res.meta || null);
      })
      .catch((err) => setError(err instanceof Error ? err.message : "加载失败"))
      .finally(() => setLoading(false));

    loadLoop(readTenant());
    return subscribeTenant(loadLoop);
  }, [loadLoop]);

  const sorted = [...items].sort((a, b) => {
    if (sort === "buyers") return b.buyer_count - a.buyer_count || b.total - a.total;
    if (sort === "batches") return b.batch_count - a.batch_count;
    // 观察期（无评级）永远排在最后
    const pa = a.grade === "NA" ? 1 : 0;
    const pb = b.grade === "NA" ? 1 : 0;
    if (pa !== pb) return pa - pb;
    return b.total - a.total;
  });

  const graded = items.filter((i) => i.grade !== "NA").length;

  return (
    <div className="console">
      <NavBar />
      <main className="consoleBody">
        <div className="pageHead">
          <div>
            <h1>平台综合评级</h1>
            <p>
              把每家采购方对同一家供应商的 QCDST 按样本量加权合并，形成跨企业的公共评级。
              新用户选供应商时先看这张榜，再点进去看逐项证据。
            </p>
          </div>
        </div>

        <section className="explainRow">
          <div className="explainCard">
            <b>怎么算的</b>
            <p>
              不是简单平均。每家采购方的权重 =（订单数 + 批次数）× 该次评估的置信度，
              所以交付了 300 批的客户比只试了 5 批的客户说话更重。
            </p>
          </div>
          <div className="explainCard">
            <b>分歧度是什么</b>
            <p>
              各采购方总分的标准差。均分 90 但分歧 18，说明"有人很满意、有人踩过坑"，
              比均分 85 分歧 2 更值得警惕。
            </p>
          </div>
          <div className="explainCard">
            <b>为什么有 N/A</b>
            <p>
              交付批次少于 15 个不出正式评级。新供应商、样本不足的一律标"观察"，不给排名 ——
              否则新供应商很容易靠几个批次冲到榜首。
            </p>
          </div>
        </section>

        {/* 闭环体检：先告诉用户"这个池子现在能不能信"，再看排名 */}
        {loop && (
          <section className="card loopCard">
            <div className="cardHead">
              <h2>数据闭环体检</h2>
              <span className="cardHint">评级来自各企业在 SRM 里积累的真实单据，池子越大越可信</span>
            </div>
            <div className={`loopStage ${loop.rated_suppliers === 0 ? "empty" : ""}`}>
              <b>当前阶段</b>
              <span>{loop.stage}</span>
            </div>
            <div className="loopStatRow">
              <LoopStat label="进入 SRM 的供应商" value={loop.suppliers_in_pool} unit="家" />
              <LoopStat label="已出评级" value={loop.rated_suppliers} unit="家" sub={`${loop.provisional} 家观察期`} />
              <LoopStat label="贡献数据的采购方" value={loop.tenants} unit="家" />
              <LoopStat label="累计交付批次" value={loop.total_batches} unit="批" sub={`平均分歧度 ±${loop.average_spread.toFixed(1)}`} />
              <LoopStat label="本企业在管" value={loop.my_suppliers} unit="家" sub={`${loop.my_candidates} 家候选 · ${loop.my_batches} 批`} />
            </div>
            {loop.demo > 0 && (
              <p className="loopHint">
                其中 {loop.demo} 家为演示数据，仅用于验证模型，不构成任何企业的真实评价。
              </p>
            )}
          </section>
        )}

        {error && <div className="errBanner">{error}</div>}

        <section className="card">
          <div className="cardHead">
            <h2>
              评级榜 <em className="countTag">{graded} 家已评级 / {items.length} 家在库</em>
            </h2>
            <div className="sortTabs">
              {(
                [
                  ["total", "按评级"],
                  ["buyers", "按买方数"],
                  ["batches", "按样本量"],
                ] as const
              ).map(([key, label]) => (
                <button key={key} className={`sortTab ${sort === key ? "on" : ""}`} onClick={() => setSort(key)}>
                  {label}
                </button>
              ))}
            </div>
          </div>

          {loading && <div className="emptyBlock">正在汇总…</div>}

          {!loading && items.length === 0 && (
            <div className="emptyBlock">
              <h3>还没有可汇总的评级</h3>
              <p>
                平台评级来自各企业的 SRM 单据。先去
                <Link href="/srm"> 供应商管理 </Link>
                载入演示数据或录入真实订单。
              </p>
            </div>
          )}

          {!loading && items.length > 0 && (
            <table className="grid">
              <thead>
                <tr>
                  <th style={{ width: 40 }}>#</th>
                  <th style={{ width: 210 }}>供应商</th>
                  <th style={{ width: 80 }}>评级</th>
                  <th style={{ width: 90 }}>平台分</th>
                  <th>五维</th>
                  <th style={{ width: 96 }}>采购方</th>
                  <th style={{ width: 130 }}>累计样本</th>
                  <th style={{ width: 100 }}>分歧度</th>
                </tr>
              </thead>
              <tbody>
                {sorted.map((row, i) => (
                  <tr key={row.supplier_id}>
                    <td className="mono muted">{row.grade === "NA" ? "—" : i + 1}</td>
                    <td>
                      <Link href={`/vendor/${encodeURIComponent(row.supplier_id)}`} className="vendorCell">
                        <b>{row.short_name || row.supplier_name}</b>
                        <small>
                          {row.city} · {row.capabilities?.slice(0, 2).join(" / ")}
                        </small>
                      </Link>
                    </td>
                    <td>
                      <GradeBadge grade={row.grade} provisional={row.provisional} size="sm" />
                    </td>
                    <td>
                      <span className={`bigScore ${row.total < 0 ? "missing" : ""}`}>
                        {row.total >= 0 ? row.total.toFixed(1) : "—"}
                      </span>
                    </td>
                    <td>
                      <div className="dimDots">
                        {row.dimensions.map((d) => (
                          <span key={d.dimension} className="dimDot" title={`${d.name} ${d.score < 0 ? "无数据" : d.score.toFixed(1)}`}>
                            <em>{d.name.slice(0, 1)}</em>
                            <b className={d.score < 0 ? "missing" : ""}>{d.score < 0 ? "—" : d.score.toFixed(0)}</b>
                          </span>
                        ))}
                      </div>
                    </td>
                    <td className="mono">
                      {row.rated_buyers} 家
                      <br />
                      <span className="muted small">{row.demo ? "演示" : "真实"}</span>
                    </td>
                    <td className="mono small">
                      {row.order_count} 单 · {row.batch_count} 批
                    </td>
                    <td>
                      <span className={`spreadTag ${row.spread > 8 ? "wide" : ""}`}>
                        {row.rated_buyers < 2 ? "单一买方" : row.spread < 0.2 ? "口径一致" : `± ${row.spread.toFixed(1)}`}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>

        {meta && typeof meta.note === "string" && <p className="footNote">{meta.note}</p>}
      </main>
    </div>
  );
}

function LoopStat({ label, value, unit, sub }: { label: string; value: number; unit: string; sub?: string }) {
  return (
    <div className="loopStat">
      <span className="statLabel">{label}</span>
      <div className="statValue">
        <b>{value}</b>
        <em>{unit}</em>
      </div>
      {sub && <small>{sub}</small>}
    </div>
  );
}
