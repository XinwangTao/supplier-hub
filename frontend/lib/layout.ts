import type { Graph, GraphNode } from "./api";

export type Vec = { x: number; y: number };
export type Bounds = { minX: number; minY: number; maxX: number; maxY: number };

export const NODE_RADIUS: Record<string, number> = {
  product: 48,
  module: 27,
  process: 15,
  supplier: 9,
  capability: 22,
};

export function radiusOf(node: GraphNode, compact = false): number {
  if (node.type === "supplier" && node.risk === "high") return NODE_RADIUS.supplier + 1.5;
  if (node.type === "capability") {
    const count = Number(node.meta?.suppliers || 0);
    return 17 + Math.min(13, count * 2.1);
  }
  if (node.type === "product" && compact) return 32;
  return NODE_RADIUS[node.type] ?? 12;
}

// 稳定 hash：同一个节点在任何一次渲染里抖动方向都一致，避免每次刷新图形乱跳。
function hash(id: string): number {
  let h = 2166136261;
  for (let i = 0; i < id.length; i++) {
    h ^= id.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  return ((h >>> 0) % 10000) / 10000;
}

const TAU = Math.PI * 2;

/**
 * 以产品为中心的放射布局：
 * 产品（圆心）→ 模块（第一环）→ 工序（第二环）→ 供应商（第三环，围绕所属工序成簇）。
 * 之后做若干轮碰撞分离，保证节点不重叠，同时保留放射结构。
 */
export function computeLayout(graph: Graph): { positions: Record<string, Vec>; bounds: Bounds } {
  const positions: Record<string, Vec> = {};
  const byType = (type: string) => graph.nodes.filter((n) => n.type === type);

  if (graph.mode === "mesh") {
    const capabilities = byType("capability");
    const products = byType("product");

    capabilities.forEach((node, i) => {
      const angle = -Math.PI / 2 + (i / Math.max(1, capabilities.length)) * TAU;
      // 奇偶交错半径，避免标签在同一圈上互相压叠
      const ring = 400 + (i % 2 === 0 ? 0 : 52) + (hash(node.id + "r") - 0.5) * 22;
      positions[node.id] = { x: Math.cos(angle) * ring, y: Math.sin(angle) * ring };
    });

    products.forEach((node, i) => {
      const angle = -Math.PI / 2 + (i / Math.max(1, products.length)) * TAU + 0.06;
      positions[node.id] = { x: Math.cos(angle) * 172, y: Math.sin(angle) * 172 };
    });
  } else if (graph.mode === "product") {
    positions["product"] = { x: 0, y: 0 };

    const modules = byType("module");
    const processes = byType("process");
    const suppliers = byType("supplier");

    const childrenOf = new Map<string, GraphNode[]>();
    processes.forEach((node) => {
      const list = childrenOf.get(node.parent_id || "") || [];
      list.push(node);
      childrenOf.set(node.parent_id || "", list);
    });

    const suppliersOf = new Map<string, GraphNode[]>();
    suppliers.forEach((node) => {
      const list = suppliersOf.get(node.parent_id || "") || [];
      list.push(node);
      suppliersOf.set(node.parent_id || "", list);
    });

    // 按模块体量（工序数 + 供应商数）分配扇区角：模块越重，占的角度越大，节点就不会挤成一坨
    const weightOf = (moduleID: string) => {
      const kids = childrenOf.get(moduleID) || [];
      const supCount = kids.reduce((sum, kid) => sum + (suppliersOf.get(kid.id)?.length || 0), 0);
      return Math.max(1, kids.length * 1.4 + supCount * 0.5);
    };
    const totalWeight = modules.reduce((sum, node) => sum + weightOf(node.id), 0) || 1;

    const moduleRadius = 206;
    const processRadius = 352;
    const sector = new Map<string, { start: number; span: number }>();
    // 从正上方开始排，第一个模块落在顶部
    let cursor = -Math.PI / 2;
    modules.forEach((node) => {
      const span = (weightOf(node.id) / totalWeight) * TAU;
      sector.set(node.id, { start: cursor, span });
      const mid = cursor + span / 2;
      positions[node.id] = { x: Math.cos(mid) * moduleRadius, y: Math.sin(mid) * moduleRadius };
      cursor += span;
    });

    const processAngle = new Map<string, number>();
    modules.forEach((module) => {
      const { start, span } = sector.get(module.id) || { start: 0, span: 0 };
      const kids = childrenOf.get(module.id) || [];
      kids.forEach((node, i) => {
        const ratio = (i + 0.5) / Math.max(1, kids.length);
        const angle = start + span * (0.12 + ratio * 0.76);
        processAngle.set(node.id, angle);
        const r = processRadius + (hash(node.id) - 0.5) * 24;
        positions[node.id] = { x: Math.cos(angle) * r, y: Math.sin(angle) * r };
      });
    });

    suppliersOf.forEach((list, parentID) => {
      const base = processAngle.get(parentID) ?? 0;
      const r = (positions[parentID] ? Math.hypot(positions[parentID].x, positions[parentID].y) : processRadius) + 84;
      list.sort((a, b) => a.id.localeCompare(b.id));
      list.forEach((node, i) => {
        // 同一工序下的多家供应商：角度微错开 + 半径拉开一档，标签才不会叠在一起
        const spread = list.length === 1 ? 0 : i / (list.length - 1) - 0.5;
        const offset = spread * 0.12;
        const angle = base + offset + (hash(node.id) - 0.5) * 0.02;
        const rr = r + spread * 52 + (hash(node.id + "s") - 0.5) * 14;
        positions[node.id] = { x: Math.cos(angle) * rr, y: Math.sin(angle) * rr };
      });
    });
  } else {
    // supplier 自我中心网
    const center = graph.nodes.find((n) => n.id === graph.center) || byType("supplier")[0];
    const capabilities = byType("capability");
    const suppliers = byType("supplier").filter((n) => n.id !== center?.id);
    if (center) positions[center.id] = { x: 0, y: 0 };
    capabilities.forEach((node, i) => {
      const angle = -Math.PI / 2 + (i / Math.max(1, capabilities.length)) * TAU;
      positions[node.id] = { x: Math.cos(angle) * 190, y: Math.sin(angle) * 190 };
    });
    suppliers.forEach((node, i) => {
      const capIndex = Math.max(0, capabilities.findIndex((c) => c.id === "cap-" + node.capability));
      const base = -Math.PI / 2 + (capIndex / Math.max(1, capabilities.length)) * TAU;
      const angle = base + (hash(node.id) - 0.5) * 0.5;
      positions[node.id] = { x: Math.cos(angle) * 360, y: Math.sin(angle) * 360 };
      void i;
    });
  }

  const compact = graph.mode === "mesh";
  separate(graph.nodes, positions, compact);

  return { positions, bounds: computeBounds(graph.nodes, positions, compact, 52) };
}

/**
 * 碰撞分离：把重叠的节点推开，但每个节点带一个回位力，避免整体结构散掉。
 * 比纯力导向稳定得多，也不会出现节点飞出去的情况。
 */
function separate(nodes: GraphNode[], positions: Record<string, Vec>, compact = false) {
  const items = nodes.filter((n) => positions[n.id]);
  const home = items.map((n) => ({ ...positions[n.id] }));
  const pass = 220;
  for (let p = 0; p < pass; p++) {
    const decay = 1 - p / pass;
    for (let i = 0; i < items.length; i++) {
      for (let j = i + 1; j < items.length; j++) {
        const a = positions[items[i].id];
        const b = positions[items[j].id];
        let dx = b.x - a.x;
        let dy = b.y - a.y;
        let dist = Math.hypot(dx, dy);
        if (dist === 0) {
          dx = hash(items[i].id + items[j].id) - 0.5;
          dy = hash(items[j].id + items[i].id) - 0.5;
          dist = Math.hypot(dx, dy) || 1;
        }
        const minDist = radiusOf(items[i], compact) + radiusOf(items[j], compact) + 16;
        if (dist >= minDist) continue;
        const push = ((minDist - dist) / dist) * 0.5 * decay;
        const ox = dx * push;
        const oy = dy * push;
        a.x -= ox;
        a.y -= oy;
        b.x += ox;
        b.y += oy;
      }
    }
    // 回位：产品/中心节点不动，其余节点轻微拉回初始环位
    for (let i = 0; i < items.length; i++) {
      const node = items[i];
      if (node.type === "product" && node.level === 0) continue;
      const pull = node.type === "supplier" ? 0.06 : 0.11;
      const cur = positions[node.id];
      cur.x += (home[i].x - cur.x) * pull * decay;
      cur.y += (home[i].y - cur.y) * pull * decay;
    }
  }
}

function computeBounds(nodes: GraphNode[], positions: Record<string, Vec>, compact: boolean, padding: number): Bounds {
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  nodes.forEach((node) => {
    const p = positions[node.id];
    if (!p) return;
    const r = radiusOf(node, compact);
    minX = Math.min(minX, p.x - r);
    minY = Math.min(minY, p.y - r);
    maxX = Math.max(maxX, p.x + r);
    maxY = Math.max(maxY, p.y + r);
  });
  if (!Number.isFinite(minX)) return { minX: -400, minY: -400, maxX: 400, maxY: 400 };
  return { minX: minX - padding, minY: minY - padding, maxX: maxX + padding, maxY: maxY + padding };
}

/** 二次贝塞尔曲线，让连线看起来像有机的"纤维"而不是直尺线。 */
export function edgePath(from: Vec, to: Vec, curve = 0.12): string {
  const dx = to.x - from.x;
  const dy = to.y - from.y;
  const mx = (from.x + to.x) / 2;
  const my = (from.y + to.y) / 2;
  const cx = mx - dy * curve;
  const cy = my + dx * curve;
  return `M ${from.x} ${from.y} Q ${cx} ${cy} ${to.x} ${to.y}`;
}
