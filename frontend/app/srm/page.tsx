"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import NavBar from "@/components/NavBar";
import { GradeBadge } from "@/components/Qcdst";
import {
  fetchTenants,
  fetchWorkspace,
  seedDemo,
  setEngagementStatus,
  ENGAGEMENT_STATUS,
  type SRMWorkspace,
} from "@/lib/api";
import { readTenant, subscribeTenant, writeTenant } from "@/lib/tenant";

const DIM_LABEL: Record<string, string> = { Q: "质", C: "本", D: "交", S: "服", T: "技" };

export default function SrmPage() {
  const [workspace, setWorkspace] = useState<SRMWorkspace | null>(null);
  const [tenants, setTenants] = useState<{ id: string; name: string }[]>([]);
  const [tenant, setTenant] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const load = useCallback(async (tenantId: string) => {
    setLoading(true);
    try {
      const data = await fetchWorkspace(tenantId);
      setWorkspace(data);
      if (!tenantId && data.tenant) setTenant(data.tenant.id);
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const applyTenant = (id: string) => {
      setTenant(id);
      void load(id);
    };
    void fetchTenants()
      .then((list) => {
        setTenants(list);
        const fromUrl = new URLSearchParams(window.location.search).get("tenant") || "";
        const stored = readTenant();
        const initial =
          (fromUrl && list.some((t) => t.id === fromUrl) && fromUrl) ||
          (stored && list.some((t) => t.id === stored) && stored) ||
          list[0]?.id ||
          "";
        if (initial !== stored) writeTenant(initial);
        applyTenant(initial);
      })
      .catch(() => void load(""));
    const unsubscribe = subscribeTenant(applyTenant);
    return unsubscribe;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const onSeed = async () => {
    setBusy(true);
    try {
      await seedDemo();
      await fetchTenants().then(setTenants);
      await load("t-demo");
      setTenant("t-demo");
    } catch (err) {
      setError(err instanceof Error ? err.message : "载入失败");
    } finally {
      setBusy(false);
    }
  };

  const stats = workspace?.stats;
  const rows = workspace?.assessments || [];
  const engagements = workspace?.engagements || [];
  const engBySupplier: Record<string, (typeof engagements)[number]> = {};
  engagements.forEach((e) => {
    engBySupplier[e.supplier_id] = e;
  });
  const candidates = engagements.filter((e) => e.status === "candidate").length;
  const activeCount = engagements.filter((e) => e.status === "active").length;

  const changeStatus = async (supplierId: string, status: string) => {
    setBusy(true);
    try {
      await setEngagementStatus(tenant, supplierId, status);
      await load(tenant);
    } catch (err) {
      setError(err instanceof Error ? err.message : "状态更新失败");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="console">
      <NavBar />
      <main className="consoleBody">
        <div className="pageHead">
          <div>
            <h1>供应商管理 · SRM</h1>
            <p>
              QCDST 评分只由这张工作台里的真实单据驱动：订单、交付批次、IQC 结果、质量事件、报价、服务工单与现场评审。
              没有单据就没有分数，证据不足不评级。
            </p>
          </div>
          <div className="pageActions">
            <select
              className="select"
              value={tenant}
              onChange={(e) => {
                writeTenant(e.target.value);
                setTenant(e.target.value);
                void load(e.target.value);
              }}
            >
              {tenants.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.name}
                </option>
              ))}
            </select>
            <Link className="btn primary" href="/search">
              从寻源池引入 →
            </Link>
            <button className="btn ghost" onClick={onSeed} disabled={busy}>
              {busy ? "载入中…" : "载入演示数据"}
            </button>
          </div>
        </div>

        {workspace?.demo && (
          <div className="demoBanner">
            <b>演示数据</b>
            <span>
              当前工作台的供应商与单据为演示用途，信用代码带 DEMO 前缀，不是真实企业。真实使用时请通过 SRM 录入自己的订单与交付记录。
            </span>
          </div>
        )}

        {error && <div className="errBanner">{error}</div>}

        {stats && (
          <section className="statRow">
            <Stat
              label="在管供应商"
              value={stats.suppliers}
              unit="家"
              sub={`${activeCount} 家合作中 · ${candidates} 家候选`}
            />
            <Stat label="累计订单" value={stats.orders} unit="张" sub={`${stats.open_orders} 张在单`} />
            <Stat label="交付批次" value={stats.deliveries} unit="批" sub="QCDST 的样本基数" />
            <Stat
              label="准时交付率"
              value={stats.on_time_rate.toFixed(1)}
              unit="%"
              tone={stats.on_time_rate < 92 ? "warn" : "ok"}
              sub="低于 92% 需排查共性原因"
            />
            <Stat label="来料合格率" value={stats.pass_rate.toFixed(1)} unit="%" sub="含让步接收" />
            <Stat label="年采购额" value={(stats.spend / 10000).toFixed(2)} unit="亿" sub="合作关系汇总" />
          </section>
        )}

        {workspace && workspace.insights.length > 0 && (
          <section className="insightRow">
            {workspace.insights.map((insight, i) => (
              <div key={`${insight.kind}-${i}`} className={`insightCard lv-${insight.level}`}>
                <b>{insight.title}</b>
                <p>{insight.detail}</p>
              </div>
            ))}
          </section>
        )}

        <section className="card">
          <div className="cardHead">
            <h2>供应商 QCDST 评估</h2>
            <span className="cardHint">
              按总分降序；观察期与证据不足的排在最后 —— 那不是"分低"，是"还不知道"。
            </span>
          </div>

          {loading && <div className="emptyBlock">正在计算…</div>}

          {!loading && rows.length === 0 && (
            <div className="emptyBlock">
              <h3>还没有供应商进入 SRM</h3>
              <p>先建立合作关系并录入第一张订单，QCDST 才有数据可算。也可以先载入演示数据看看评分模型长什么样。</p>
              <button className="btn primary" onClick={onSeed} disabled={busy}>
                载入演示数据
              </button>
            </div>
          )}

          {!loading && rows.length > 0 && (
            <table className="grid">
              <thead>
                <tr>
                  <th style={{ width: 220 }}>供应商</th>
                  <th style={{ width: 110 }}>合作状态</th>
                  <th style={{ width: 90 }}>评级</th>
                  <th style={{ width: 96 }}>总分</th>
                  <th>五维</th>
                  <th style={{ width: 110 }}>样本</th>
                  <th style={{ width: 120 }}>数据置信度</th>
                  <th style={{ width: 90 }} />
                </tr>
              </thead>
              <tbody>
                {rows.map((row) => (
                  <tr key={row.supplier_id}>
                    <td>
                      <Link href={`/vendor/${encodeURIComponent(row.supplier_id)}?tenant=${row.tenant_id}`} className="vendorCell">
                        <b>{row.short_name || row.supplier_name}</b>
                        <small>{row.supplier_name}</small>
                      </Link>
                    </td>
                    <td>
                      {(() => {
                        const eng = engBySupplier[row.supplier_id];
                        const status = eng?.status || "active";
                        return (
                          <span className="statusCell">
                            <span className={`statusChip st-${status}`}>
                              {ENGAGEMENT_STATUS[status] || status}
                            </span>
                            {status === "candidate" && (
                              <button className="linkBtn" disabled={busy} onClick={() => void changeStatus(row.supplier_id, "active")}>
                                转合作中
                              </button>
                            )}
                            {eng?.source === "sourcing" && <small className="dimText">寻源引入</small>}
                          </span>
                        );
                      })()}
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
                      <div className="miniDims">
                        {row.dimensions.map((d) => (
                          <span key={d.dimension} className="miniDim" title={`${d.name} ${d.score < 0 ? "无数据" : d.score.toFixed(1)}`}>
                            <em>{DIM_LABEL[d.dimension]}</em>
                            <i>
                              <u style={{ width: `${d.score < 0 ? 0 : Math.max(3, d.score)}%` }} />
                            </i>
                            <b className={d.score < 0 ? "missing" : ""}>{d.score < 0 ? "—" : d.score.toFixed(0)}</b>
                          </span>
                        ))}
                      </div>
                    </td>
                    <td className="mono small">
                      {row.order_count} 单 · {row.batch_count} 批
                      <br />
                      {row.quality_events} 起质量事件
                    </td>
                    <td>
                      <div className="confBar">
                        <i style={{ width: `${row.confidence * 100}%` }} />
                      </div>
                      <span className="mono small">{(row.confidence * 100).toFixed(0)}% · 覆盖 {(row.coverage * 100).toFixed(0)}%</span>
                    </td>
                    <td>
                      <Link href={`/vendor/${encodeURIComponent(row.supplier_id)}?tenant=${row.tenant_id}`} className="linkBtn">
                        明细 →
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>
      </main>
    </div>
  );
}

function Stat({ label, value, unit, sub, tone }: { label: string; value: string | number; unit: string; sub?: string; tone?: "ok" | "warn" }) {
  return (
    <div className={`statCard ${tone ? `tone-${tone}` : ""}`}>
      <span className="statLabel">{label}</span>
      <div className="statValue">
        <b>{value}</b>
        <em>{unit}</em>
      </div>
      {sub && <small>{sub}</small>}
    </div>
  );
}
