"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import NavBar from "@/components/NavBar";
import { fetchDataPolicy, type DataPolicy, type PolicyFieldGroup } from "@/lib/api";

const TIER_LABEL: Record<string, string> = {
  public: "公开层",
  verified: "认证层",
  partner: "合作层",
  owner: "自有层",
};

// 层级的排布顺序就是"关系由浅到深"，不按字母排：
// 用户要能一眼看出自己现在在哪一层、再往前一步能看到什么。
const TIER_ORDER = ["public", "verified", "partner", "owner"];

export default function PolicyPage() {
  const [doc, setDoc] = useState<DataPolicy | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    fetchDataPolicy()
      .then(setDoc)
      .catch((err) => setError(err instanceof Error ? err.message : "加载失败"));
  }, []);

  return (
    <div className="console">
      <NavBar />
      <main className="consoleBody">
        <div className="pageHead">
          <div>
            <h1>数据开放规则</h1>
            <p>
              这份页面回答一个必然会被问到的问题：<b>为什么有些信息看不到、为什么不能导出。</b>
              规则写在接口里（<code>/api/v1/meta/policy</code>），不是写在公告里 —— 公告可以改口，接口不能。
            </p>
          </div>
        </div>

        {error && <div className="errBanner">{error}</div>}
        {!doc && !error && <div className="emptyBlock">正在读取策略…</div>}

        {doc && (
          <>
            {/* 定位：先说清这个平台是什么，规则才有立足点 */}
            <section className="policyBanner">
              <b>平台定位</b>
              <p>{doc.positioning}</p>
            </section>

            <section className="card">
              <div className="cardHead">
                <h2>三条不可让渡的原则</h2>
                <span className="cardHint">违反其中任何一条，评级的决策价值就归零</span>
              </div>
              <ol className="principleList">
                {doc.principle.map((p, i) => (
                  <li key={i}>
                    <span className="principleNo">{String(i + 1).padStart(2, "0")}</span>
                    <span>{p}</span>
                  </li>
                ))}
              </ol>
            </section>

            {/* 四层阶梯 */}
            <section className="card">
              <div className="cardHead">
                <h2>四层可见范围</h2>
                <span className="cardHint">层级按“关系深度”划分，不按用户类型划分</span>
              </div>
              <div className="tierLadder">
                {TIER_ORDER.map((tier, i) => {
                  const t = doc.tiers.find((x) => x.tier === tier);
                  if (!t) return null;
                  return (
                    <div className={`tierCard tk-${tier}`} key={tier}>
                      <div className="tierTop">
                        <span className="tierStep">{i + 1}</span>
                        <b>{t.label}</b>
                      </div>
                      <dl>
                        <dt>谁</dt>
                        <dd>{t.who}</dd>
                        <dt>怎么获得</dt>
                        <dd>{t.how_to_get}</dd>
                        <dt>能看到</dt>
                        <dd>{t.can_see}</dd>
                        <dt>数据权利</dt>
                        <dd>{t.data_right}</dd>
                      </dl>
                    </div>
                  );
                })}
              </div>
            </section>

            {/* 字段矩阵：这是整页最实用的部分 */}
            <section className="card">
              <div className="cardHead">
                <h2>
                  字段边界 <em className="countTag">{doc.fields.length} 组</em>
                </h2>
                <span className="cardHint">越接近“能直接联系并成交”的字段，放得越靠后</span>
              </div>
              <table className="grid policyTable">
                <thead>
                  <tr>
                    <th style={{ width: 170 }}>字段组</th>
                    <th style={{ width: 90 }}>开放层级</th>
                    <th style={{ width: 220 }}>包含字段</th>
                    <th>为什么这么定</th>
                  </tr>
                </thead>
                <tbody>
                  {doc.fields.map((g) => (
                    <FieldRow key={g.key} group={g} />
                  ))}
                </tbody>
              </table>
            </section>

            {/* 接口层配额 */}
            <section className="card">
              <div className="cardHead">
                <h2>接口层硬约束</h2>
                <span className="cardHint">这些值由服务端强制，前端传什么参数都不作数</span>
              </div>
              <div className="loopStatRow">
                <QuotaStat label="单页条数上限" value={doc.quotas.max_page_size} unit="条" sub="超出直接回落默认值" />
                <QuotaStat label="免认证可翻到" value={doc.quotas.deep_page_free_max} unit="条" sub="防 offset 遍历全库" />
                <QuotaStat label="单来源请求上限" value={doc.quotas.requests_per_min} unit="次/分" sub="超出返回 429" />
                <QuotaStat label="免费看完整档案" value={doc.quotas.detail_free_views} unit="次/家" sub="用完需企业认证" />
              </div>
              <p className="loopHint">
                每个响应都带 <code>X-Trace-Id</code> 水印。技术手段只能约束“能不能拿”，水印约束的是“敢不敢传”——
                泄露的截图与文件可以反查到具体账号，这比封 IP 更有用。
              </p>
            </section>

            {/* 导出：允许什么、不提供什么 */}
            <section className="card">
              <div className="cardHead">
                <h2>导出范围</h2>
                <span className="cardHint">锁别人的数据，放自己的数据 —— 这条不对称是刻意的</span>
              </div>
              <div className="exportGrid">
                <div className="exportCol ok">
                  <b>可以导出</b>
                  <ul>
                    {doc.export.allowed.map((x, i) => (
                      <li key={i}>{x}</li>
                    ))}
                  </ul>
                </div>
                <div className="exportCol no">
                  <b>不提供导出</b>
                  <ul>
                    {doc.export.denied.map((x, i) => (
                      <li key={i}>{x}</li>
                    ))}
                  </ul>
                </div>
              </div>
              <div className="exportWhy">
                <p>
                  <b>为什么不给批量导出</b>
                  {doc.export.why}
                </p>
                <p>
                  <b>那我要做比价表怎么办</b>
                  {doc.export.alternative}
                </p>
              </div>
            </section>

            <section className="card">
              <div className="cardHead">
                <h2>明确禁止的做法</h2>
                <span className="cardHint">写在这里是为了让合作方在签约前就能确认边界</span>
              </div>
              <ul className="prohibitList">
                {doc.prohibited.map((x, i) => (
                  <li key={i}>{x}</li>
                ))}
              </ul>
            </section>

            <p className="footNote">
              策略版本 {doc.version} · 需要完整字段的机构客户可通过{" "}
              <Link href="/srm">供应商管理</Link> 建立合作关系，或走 API 授权与配额。
            </p>
          </>
        )}
      </main>
    </div>
  );
}

function FieldRow({ group }: { group: PolicyFieldGroup }) {
  return (
    <tr>
      <td>
        <b>{group.label}</b>
      </td>
      <td>
        <span className={`tierBadge tb-${group.tier}`}>{TIER_LABEL[group.tier] || group.tier}</span>
      </td>
      <td>
        <div className="fieldTags">
          {group.fields.map((f) => (
            <code key={f}>{f}</code>
          ))}
        </div>
      </td>
      <td className="reasonText">{group.reason}</td>
    </tr>
  );
}

function QuotaStat({ label, value, unit, sub }: { label: string; value: number; unit: string; sub: string }) {
  return (
    <div className="loopStat">
      <span className="statLabel">{label}</span>
      <div className="statValue">
        <b>{value}</b>
        <em>{unit}</em>
      </div>
      <small>{sub}</small>
    </div>
  );
}
