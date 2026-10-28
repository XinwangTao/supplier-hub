"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { Graph, GraphNode } from "../lib/api";
import { computeLayout, edgePath, radiusOf, type Vec } from "../lib/layout";

type Props = {
  graph: Graph;
  selectedId: string | null;
  onSelect: (node: GraphNode | null) => void;
};

const EDGE_STYLE: Record<string, { stroke: string; width: number; dash?: string; opacity: number }> = {
  bom: { stroke: "#8fb3a3", width: 1.5, opacity: 0.95 },
  process: { stroke: "#c3d6cc", width: 1, opacity: 0.85 },
  supply: { stroke: "#84a99b", width: 1, opacity: 0.8 },
  alternative: { stroke: "#d99b2b", width: 0.9, dash: "3 4", opacity: 0.34 },
  same_city: { stroke: "#7f9ec7", width: 0.8, dash: "2 4", opacity: 0.28 },
  uses: { stroke: "#b6cdc2", width: 0.9, opacity: 0.7 },
  cooccur: { stroke: "#9cbdb0", width: 0.9, dash: "5 4", opacity: 0.55 },
};

const RISK_STROKE: Record<string, string> = {
  low: "#2f7d63",
  medium: "#d99b2b",
  high: "#c9603c",
  critical: "#b03a2e",
};

export default function NetworkGraph({ graph, selectedId, onSelect }: Props) {
  const wrapRef = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState({ w: 900, h: 640 });
  const [positions, setPositions] = useState<Record<string, Vec>>({});
  const [view, setView] = useState({ x: 0, y: 0, k: 1 });
  const [hovered, setHovered] = useState<string | null>(null);
  const dragRef = useRef<{ mode: "pan" | "node"; id?: string; startX: number; startY: number; viewX: number; viewY: number; nodeX: number; nodeY: number } | null>(null);

  const layout = useMemo(() => computeLayout(graph), [graph]);

  useEffect(() => {
    setPositions(layout.positions);
  }, [layout]);

  useEffect(() => {
    const el = wrapRef.current;
    if (!el) return;
    const observer = new ResizeObserver((entries) => {
      const rect = entries[0].contentRect;
      if (rect.width > 0 && rect.height > 0) setSize({ w: rect.width, h: rect.height });
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  const fit = useCallback(() => {
    const { bounds } = layout;
    const bw = Math.max(bounds.maxX - bounds.minX, 200);
    const bh = Math.max(bounds.maxY - bounds.minY, 200);
    const k = Math.min(size.w / bw, size.h / bh) * 0.97;
    const cx = (bounds.minX + bounds.maxX) / 2;
    const cy = (bounds.minY + bounds.maxY) / 2;
    setView({ x: size.w / 2 - cx * k, y: size.h / 2 - cy * k, k });
  }, [layout, size.w, size.h]);

  useEffect(() => {
    fit();
  }, [fit]);

  useEffect(() => {
    const el = wrapRef.current;
    if (!el) return;
    const onWheel = (event: WheelEvent) => {
      event.preventDefault();
      const rect = el.getBoundingClientRect();
      const px = event.clientX - rect.left;
      const py = event.clientY - rect.top;
      setView((prev) => {
        const factor = Math.exp(-event.deltaY * 0.0016);
        const k = Math.min(3.2, Math.max(0.24, prev.k * factor));
        return { k, x: px - ((px - prev.x) * k) / prev.k, y: py - ((py - prev.y) * k) / prev.k };
      });
    };
    el.addEventListener("wheel", onWheel, { passive: false });
    return () => el.removeEventListener("wheel", onWheel);
  }, []);

  const neighbors = useMemo(() => {
    const map: Record<string, Set<string>> = {};
    graph.edges.forEach((edge) => {
      (map[edge.source] ||= new Set()).add(edge.target);
      (map[edge.target] ||= new Set()).add(edge.source);
    });
    return map;
  }, [graph.edges]);

  const focusId = hovered || selectedId;
  const focusSet = useMemo(() => {
    if (!focusId) return null;
    const set = new Set<string>([focusId]);
    neighbors[focusId]?.forEach((id) => set.add(id));
    return set;
  }, [focusId, neighbors]);

  function toGraph(clientX: number, clientY: number): Vec {
    const rect = wrapRef.current?.getBoundingClientRect();
    if (!rect) return { x: 0, y: 0 };
    return { x: (clientX - rect.left - view.x) / view.k, y: (clientY - rect.top - view.y) / view.k };
  }

  function handleBackgroundDown(event: React.PointerEvent<SVGSVGElement>) {
    if (event.button !== 0) return;
    (event.target as Element).setPointerCapture?.(event.pointerId);
    dragRef.current = { mode: "pan", startX: event.clientX, startY: event.clientY, viewX: view.x, viewY: view.y, nodeX: 0, nodeY: 0 };
  }

  function handlePointerMove(event: React.PointerEvent<SVGSVGElement>) {
    const drag = dragRef.current;
    if (!drag) return;
    if (drag.mode === "pan") {
      setView((prev) => ({ ...prev, x: drag.viewX + (event.clientX - drag.startX), y: drag.viewY + (event.clientY - drag.startY) }));
      return;
    }
    if (drag.mode === "node" && drag.id) {
      const point = toGraph(event.clientX, event.clientY);
      setPositions((prev) => ({ ...prev, [drag.id as string]: { x: point.x + drag.nodeX, y: point.y + drag.nodeY } }));
    }
  }

  function handlePointerUp() {
    dragRef.current = null;
  }

  function handleNodeDown(event: React.PointerEvent<SVGGElement>, node: GraphNode) {
    if (event.button !== 0) return;
    event.stopPropagation();
    const point = toGraph(event.clientX, event.clientY);
    const current = positions[node.id];
    dragRef.current = {
      mode: "node",
      id: node.id,
      startX: event.clientX,
      startY: event.clientY,
      viewX: view.x,
      viewY: view.y,
      nodeX: current ? current.x - point.x : 0,
      nodeY: current ? current.y - point.y : 0,
    };
  }

  const nodeById = useMemo(() => {
    const map: Record<string, GraphNode> = {};
    graph.nodes.forEach((node) => (map[node.id] = node));
    return map;
  }, [graph.nodes]);

  // 同一工序下的供应商按序号交错摆放标签（上/下），避免同名簇的标签互相压叠
  const siblingSide = useMemo(() => {
    const groups = new Map<string, GraphNode[]>();
    graph.nodes.forEach((node) => {
      if (node.type !== "supplier") return;
      const key = node.parent_id || "root";
      const list = groups.get(key) || [];
      list.push(node);
      groups.set(key, list);
    });
    const side: Record<string, boolean> = {};
    groups.forEach((list) => {
      list
        .slice()
        .sort((a, b) => a.id.localeCompare(b.id))
        .forEach((node, index) => {
          side[node.id] = list.length > 1 && index % 2 === 1;
        });
    });
    return side;
  }, [graph.nodes]);

  return (
    <div className="graphWrap" ref={wrapRef}>
      <svg
        className="graphSvg"
        width={size.w}
        height={size.h}
        viewBox={`0 0 ${size.w} ${size.h}`}
        onPointerDown={handleBackgroundDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
        onPointerLeave={handlePointerUp}
        onClick={(event) => {
          if (event.target === event.currentTarget) onSelect(null);
        }}
      >
        <defs>
          <radialGradient id="stageGlow" cx="50%" cy="50%" r="60%">
            <stop offset="0%" stopColor="#ffffff" />
            <stop offset="70%" stopColor="#fbfcfa" />
            <stop offset="100%" stopColor="#f2f6f3" />
          </radialGradient>
          <filter id="nodeShadow" x="-60%" y="-60%" width="220%" height="220%">
            <feDropShadow dx="0" dy="2" stdDeviation="3" floodColor="#1b3b32" floodOpacity="0.14" />
          </filter>
          <filter id="selectedGlow" x="-80%" y="-80%" width="260%" height="260%">
            <feDropShadow dx="0" dy="0" stdDeviation="7" floodColor="#0e6253" floodOpacity="0.42" />
          </filter>
        </defs>
        <rect width={size.w} height={size.h} fill="url(#stageGlow)" />
        <g transform={`translate(${view.x},${view.y}) scale(${view.k})`}>
          <g className="edges">
            {graph.edges.map((edge) => {
              const from = positions[edge.source];
              const to = positions[edge.target];
              if (!from || !to) return null;
              const style = EDGE_STYLE[edge.relation] || EDGE_STYLE.supply;
              const active = focusSet ? focusSet.has(edge.source) && focusSet.has(edge.target) : false;
              const dim = focusSet && !active;
              return (
                <path
                  key={edge.id}
                  d={edgePath(from, to, edge.relation === "alternative" || edge.relation === "cooccur" ? 0.16 : 0.08)}
                  fill="none"
                  stroke={active ? "#0e6253" : style.stroke}
                  strokeWidth={active ? style.width + 0.8 : style.width}
                  strokeDasharray={style.dash}
                  strokeOpacity={dim ? 0.12 : style.opacity}
                  strokeLinecap="round"
                />
              );
            })}
          </g>
          <g className="nodes">
            {graph.nodes.map((node) => {
              const point = positions[node.id];
              if (!point) return null;
              const radius = radiusOf(node, graph.mode === "mesh");
              const isSelected = selectedId === node.id;
              const isHovered = hovered === node.id;
              const dimmed = focusSet ? !focusSet.has(node.id) : false;
              const showLabel = shouldShowLabel(node, view.k, isSelected || isHovered, focusSet?.has(node.id) === true && focusId !== node.id);

              return (
                <g
                  key={node.id}
                  transform={`translate(${point.x},${point.y})`}
                  opacity={dimmed ? 0.28 : 1}
                  style={{ cursor: "pointer" }}
                  onPointerDown={(event) => handleNodeDown(event, node)}
                  onClick={(event) => {
                    event.stopPropagation();
                    onSelect(node);
                  }}
                  onPointerEnter={() => setHovered(node.id)}
                  onPointerLeave={() => setHovered(null)}
                >
                  {renderNode(node, radius, isSelected, isHovered)}
                  {showLabel && renderLabel(node, radius, point, view.k, siblingSide[node.id] === true)}
                </g>
              );
            })}
          </g>
        </g>
      </svg>
      <div className="graphControls">
        <button onClick={() => setView((v) => ({ ...v, k: Math.min(3.2, v.k * 1.25) }))} aria-label="放大">＋</button>
        <button onClick={() => setView((v) => ({ ...v, k: Math.max(0.24, v.k / 1.25) }))} aria-label="缩小">－</button>
        <button onClick={fit} aria-label="适应画布">⤢</button>
      </div>
      <div className="graphHint">拖动空白处平移 · 滚轮缩放 · 拖动节点微调 · 点击查看档案</div>
      {hovered && nodeById[hovered] && <HoverCard node={nodeById[hovered]} />}
    </div>
  );
}

function shouldShowLabel(node: GraphNode, zoom: number, active: boolean, isNeighbor: boolean): boolean {
  if (node.type === "supplier") return active || isNeighbor || zoom > 0.58;
  if (node.type === "capability") return true;
  return true;
}

function labelOffset(node: GraphNode, radius: number, below = false): { x: number; y: number; anchor: "middle" | "start" | "end" } {
  if (node.type === "product") return { x: 0, y: radius + 26, anchor: "middle" };
  // 工序在环上密集，按 id 稳定地上下交错摆放，避免标签压成一片
  if (node.type === "process") {
    const above = stableHash(node.id) > 0.5;
    return { x: 0, y: above ? -(radius + 9) : radius + 15, anchor: "middle" };
  }
  if (node.type === "supplier") {
    return { x: 0, y: below ? radius + 13 : -(radius + 6), anchor: "middle" };
  }
  return { x: 0, y: radius + 16, anchor: "middle" };
}

function stableHash(id: string): number {
  let h = 2166136261;
  for (let i = 0; i < id.length; i++) {
    h ^= id.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  return ((h >>> 0) % 1000) / 1000;
}

function renderLabel(node: GraphNode, radius: number, point: Vec, zoom = 1, below = false) {
  const offset = labelOffset(node, radius, below);
  // 标签朝圆心外侧摆放，减少环上节点标签互相压叠
  const dist = Math.hypot(point.x, point.y) || 1;
  const outward = node.type === "product" ? 0 : 1;
  const shift = node.type === "supplier" ? 3 : 10;
  const ox = (point.x / dist) * shift * outward;
  const oy = (point.y / dist) * shift * outward;
  const cls = node.type === "product" ? "nodeLabel productLabel" : node.type === "module" ? "nodeLabel moduleLabel" : node.type === "supplier" ? "nodeLabel supplierLabel" : node.type === "capability" ? "nodeLabel capabilityLabel" : "nodeLabel processLabel";
  // 供应商标签反向补偿缩放，保证缩到全图时也能看清名字，而不是糊成小点
  const fontSize = node.type === "supplier" ? Math.min(24, Math.max(12, 9.6 / Math.max(0.3, zoom))) : undefined;
  return (
    <text
      className={cls}
      x={offset.x + ox}
      y={offset.y + oy}
      textAnchor={offset.anchor}
      dy={node.type === "supplier" ? 2 : 0}
      style={fontSize ? { fontSize } : undefined}
    >
      {node.label}
    </text>
  );
}

function renderNode(node: GraphNode, radius: number, isSelected: boolean, isHovered: boolean) {
  const filter = isSelected ? "url(#selectedGlow)" : "url(#nodeShadow)";
  if (node.type === "product") {
    return (
      <g filter={filter}>
        <circle r={radius + 10} fill="#0e6253" opacity={0.08} />
        <circle r={radius} fill="#0e6253" stroke={isSelected ? "#082f28" : "#0e6253"} strokeWidth={isSelected ? 2.5 : 1} />
        <circle r={radius - 9} fill="none" stroke="#ffffff" strokeOpacity={0.22} strokeWidth={1} />
        <text className="productMonogram" textAnchor="middle" dy="7">{node.label.slice(0, 1)}</text>
      </g>
    );
  }
  if (node.type === "module") {
    return (
      <g filter={filter}>
        <circle r={radius} fill={isSelected ? "#0e6253" : "#e4f0ea"} stroke={isSelected ? "#0e6253" : "#a9c9ba"} strokeWidth={1.2} />
        <circle r={radius - 6} fill="none" stroke={isSelected ? "#ffffff" : "#0e6253"} strokeOpacity={isSelected ? 0.35 : 0.16} strokeWidth={1} />
        {isHovered && <circle r={radius + 4} fill="none" stroke="#0e6253" strokeOpacity={0.28} strokeWidth={1} />}
      </g>
    );
  }
  if (node.type === "process") {
    return (
      <g filter={filter}>
        <circle r={radius} fill="#ffffff" stroke={node.meta?.critical === "true" ? "#0e6253" : "#c3d6cc"} strokeWidth={node.meta?.critical === "true" ? 1.6 : 1.1} />
        {node.meta?.critical === "true" && <circle r={3} fill="#0e6253" />}
        {isSelected && <circle r={radius + 5} fill="none" stroke="#0e6253" strokeWidth={1.4} strokeOpacity={0.6} />}
      </g>
    );
  }
  if (node.type === "capability") {
    return (
      <g filter={filter}>
        <circle r={radius} fill="#f2f7f4" stroke="#7fa79a" strokeWidth={1.2} />
        <circle r={radius - 5} fill="none" stroke="#0e6253" strokeOpacity={0.18} strokeDasharray="2 3" />
        <text className="capabilityCount" textAnchor="middle" dy="4">{node.meta?.suppliers || ""}</text>
        {isSelected && <circle r={radius + 5} fill="none" stroke="#0e6253" strokeWidth={1.4} strokeOpacity={0.6} />}
      </g>
    );
  }
  const stroke = RISK_STROKE[node.risk || "low"] || "#2f7d63";
  return (
    <g filter={filter}>
      <circle r={radius} fill="#ffffff" stroke={stroke} strokeWidth={node.risk === "high" ? 2 : 1.4} />
      {isSelected && <circle r={radius + 6} fill="none" stroke={stroke} strokeWidth={1.4} strokeOpacity={0.75} />}
      {isHovered && !isSelected && <circle r={radius + 4} fill="none" stroke={stroke} strokeOpacity={0.4} strokeWidth={1} />}
    </g>
  );
}

function HoverCard({ node }: { node: GraphNode }) {
  const rows: [string, string][] = [];
  const meta = node.meta || {};
  if (node.type === "supplier") {
    if (meta.legal_representative) rows.push(["法定代表人", meta.legal_representative]);
    if (meta.city) rows.push(["所在城市", meta.city]);
    if (meta.employees) rows.push(["员工人数", `${meta.employees} 人`]);
    if (meta.capital) rows.push(["注册资本", `${Number(meta.capital).toLocaleString("zh-CN")} 万`]);
  } else if (node.type === "process") {
    if (meta.capability_name) rows.push(["工艺能力", meta.capability_name]);
    if (meta.spec) rows.push(["技术要求", meta.spec]);
    if (meta.available) rows.push(["在库供应商", `${meta.available} 家`]);
  } else if (node.type === "module") {
    if (meta.desc) rows.push(["模块说明", meta.desc]);
  } else if (node.type === "capability") {
    if (meta.category) rows.push(["能力类别", meta.category]);
    if (meta.suppliers) rows.push(["供应商数", `${meta.suppliers} 家`]);
    if (meta.desc) rows.push(["说明", meta.desc]);
  } else if (meta.summary) {
    rows.push(["产品定位", meta.summary]);
  }
  return (
    <div className="hoverCard">
      <div className="hoverTitle">{node.label}</div>
      {node.subtitle && <div className="hoverSub">{node.subtitle}</div>}
      {rows.slice(0, 4).map(([key, value]) => (
        <div className="hoverRow" key={key}>
          <span>{key}</span>
          <b>{value}</b>
        </div>
      ))}
      {node.score ? <div className="hoverScore">匹配度 {node.score}</div> : null}
    </div>
  );
}
