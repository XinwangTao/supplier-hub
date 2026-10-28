"use client";

import type { DimensionScore, Grade, MetricScore } from "@/lib/api";

const DIM_COLOR: Record<string, string> = {
  Q: "#0e6253",
  C: "#3f7f6a",
  D: "#7f9ec7",
  S: "#d99b2b",
  T: "#8a6bb0",
};

export function GradeBadge({ grade, provisional, size = "md" }: { grade: Grade; provisional?: boolean; size?: "sm" | "md" | "lg" }) {
  const cls = `gradeBadge grade-${grade.replace("+", "plus").toLowerCase()} size-${size}`;
  return (
    <span className={cls} title={grade === "NA" ? "证据不足，不予评级" : `综合评级 ${grade}`}>
      {grade === "NA" ? (provisional ? "观察" : "N/A") : grade}
    </span>
  );
}

/** 五维雷达。无数据的维度不画点——把"没录"画成 0 分会让人误读成"做得差"。 */
export function QcdstRadar({ dimensions, total }: { dimensions: DimensionScore[]; total: number }) {
  const size = 300;
  const c = size / 2;
  const r = 104;
  const axes = dimensions.slice(0, 5);
  const point = (i: number, value: number) => {
    const angle = -Math.PI / 2 + (i / axes.length) * Math.PI * 2;
    const dist = (Math.max(0, Math.min(100, value)) / 100) * r;
    return { x: c + Math.cos(angle) * dist, y: c + Math.sin(angle) * dist };
  };

  const rings = [25, 50, 75, 100];
  const valid = axes.map((d, i) => ({ d, i })).filter((x) => x.d.score >= 0);
  const polygon = valid.map(({ d, i }) => point(i, d.score)).map((p) => `${p.x},${p.y}`).join(" ");

  return (
    <svg className="radar" viewBox={`0 0 ${size} ${size}`} role="img" aria-label="QCDST 五维雷达图">
      {rings.map((ring) => {
        const pts = axes.map((_, i) => point(i, ring)).map((p) => `${p.x},${p.y}`).join(" ");
        return <polygon key={ring} points={pts} fill="none" stroke="#e2e9e5" strokeWidth={ring === 100 ? 1.4 : 0.8} />;
      })}
      {axes.map((d, i) => {
        const p = point(i, 100);
        return <line key={d.dimension} x1={c} y1={c} x2={p.x} y2={p.y} stroke="#e2e9e5" strokeWidth={0.8} />;
      })}
      {polygon && <polygon points={polygon} fill="#0e625322" stroke="#0e6253" strokeWidth={1.8} strokeLinejoin="round" />}
      {valid.map(({ d, i }) => {
        const p = point(i, d.score);
        return <circle key={d.dimension} cx={p.x} cy={p.y} r={3.6} fill={DIM_COLOR[d.dimension]} stroke="#fff" strokeWidth={1.4} />;
      })}
      {axes.map((d, i) => {
        const p = point(i, 124);
        const missing = d.score < 0;
        return (
          <text
            key={d.dimension}
            x={p.x}
            y={p.y}
            textAnchor="middle"
            dominantBaseline="middle"
            className={`radarAxis ${missing ? "missing" : ""}`}
          >
            <tspan x={p.x} dy="-5">{d.dimension}</tspan>
            <tspan x={p.x} dy="13" className="radarValue">
              {missing ? "无数据" : d.score.toFixed(0)}
            </tspan>
          </text>
        );
      })}
      <text x={c} y={c + 4} textAnchor="middle" className="radarTotal">
        {total >= 0 ? total.toFixed(1) : "—"}
      </text>
    </svg>
  );
}

export function DimensionBar({ dimension }: { dimension: DimensionScore }) {
  const missing = dimension.score < 0;
  return (
    <div className="dimBar">
      <div className="dimBarHead">
        <span className="dimCode" style={{ background: DIM_COLOR[dimension.dimension] }}>
          {dimension.dimension}
        </span>
        <b>{dimension.name}</b>
        <span className="dimWeight">权重 {(dimension.weight * 100).toFixed(0)}%</span>
        <span className={`dimScore ${missing ? "missing" : ""}`}>{missing ? "证据不足" : dimension.score.toFixed(1)}</span>
      </div>
      <div className="dimTrack">
        <div
          className={`dimFill ${missing ? "missing" : ""}`}
          style={{
            width: `${missing ? 0 : Math.max(2, dimension.score)}%`,
            background: DIM_COLOR[dimension.dimension],
          }}
        />
        {/* 覆盖率不足时用斜纹提示：分数是算出来了，但只覆盖了一部分指标 */}
        {!missing && dimension.coverage < 0.99 && (
          <div className="dimGap" style={{ left: `${Math.max(2, dimension.score)}%` }} title={`仅 ${(dimension.coverage * 100).toFixed(0)}% 的指标有足够样本`} />
        )}
      </div>
      <p className="dimSummary">{dimension.summary}</p>
    </div>
  );
}

export function MetricTable({ metrics }: { metrics: MetricScore[] }) {
  return (
    <div className="metricTable">
      <div className="metricHead">
        <span>指标</span>
        <span>得分</span>
        <span>实际值 / 样本</span>
        <span>口径</span>
      </div>
      {metrics.map((m) => (
        <details className="metricRow" key={m.code}>
          <summary>
            <span className="metricName">
              {m.name}
              {!m.auto && <em className="manualTag">人工评审</em>}
            </span>
            <span className={`metricScore ${m.score < 0 ? "missing" : m.score < 70 ? "low" : ""}`}>
              {m.score < 0 ? "—" : m.score.toFixed(0)}
            </span>
            <span className="metricRaw">
              {m.raw}
              <small>
                n={m.sample}
                {m.min_sample > 0 ? ` / 门槛 ${m.min_sample}` : ""}
              </small>
            </span>
            <span className="metricConf">
              {m.score < 0 ? "样本不足" : `置信 ${(m.confidence * 100).toFixed(0)}%`}
            </span>
          </summary>
          <div className="metricDetail">
            <div>
              <b>计算口径</b>
              <p>{m.formula}</p>
            </div>
            <div>
              <b>对标</b>
              <p>{m.benchmark || "—"}</p>
            </div>
            <div>
              <b>分子 / 分母</b>
              <p className="mono">
                {m.numerator} / {m.denominator}
              </p>
            </div>
            {m.evidence.length > 0 && (
              <div className="metricEvidence">
                <b>可回查单据</b>
                <p className="mono">{m.evidence.join(" · ")}</p>
              </div>
            )}
          </div>
        </details>
      ))}
    </div>
  );
}
