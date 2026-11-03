#!/usr/bin/env python3
"""企查查 MCP 批量采集工具（supplier-hub）

用法:
    python scripts/qcc-collect.py status                      # 查看采集进度
    python scripts/qcc-collect.py collect --file 名单.txt      # 按名单采集（每行一家）
    python scripts/qcc-collect.py collect --pending            # 续跑 cache/qcc_pending.json
    python scripts/qcc-collect.py ingest                       # 缓存 → suppliers.csv + supplier_profiles.json

约定:
    - Key 池从 ~/.qcc-keys.json 读取（或用 --keys 指定），不写入仓库。
    - 每家企业 8 次调用：1 次查名 + 7 个增强维度，原始响应落盘到
      backend/data/cache/qcc-raw/<企业名>/<tool>.json。
    - 缓存命中即跳过，绝不重复消耗积分。
    - 接口会在 HTTP 200 的响应体里返回积分不足/地域限制，必须检查响应体关键字，
      不能只看状态码。
"""
import argparse
import csv
import json
import os
import sys
import time
import urllib.error
import urllib.request

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8")

PROJECT_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DEFAULT_DATA = os.path.join(PROJECT_ROOT, "backend", "data")
DEFAULT_KEYS = os.path.expanduser("~/.qcc-keys.json")
ENDPOINT = "https://agent.qcc.com/mcp/company/stream"

TOOLS_BASIC = [
    "get_company_registration_info",
    "get_contact_info",
]
TOOLS_ENHANCED = [
    "get_company_profile",
    "get_shareholder_info",
    "get_key_personnel",
    "get_external_investments",
    "get_branches",
]
TOOLS = TOOLS_BASIC + TOOLS_ENHANCED
QUOTA_MARKS = ("credit_balance_insufficient", "300008", "积分余额不足")
BAD_MARKS = QUOTA_MARKS + ("地域限制",)

CSV_COLUMNS = [
    "企业名称", "统一社会信用代码", "法定代表人", "法定代表人职务", "经营状态", "企业类型",
    "注册地址", "经营地址", "成立日期", "注册资本", "实缴资本", "参保人数", "员工人数",
    "经营范围", "所属行业", "省份", "城市", "区县", "联系电话", "邮箱", "网址",
    "数据来源", "详情链接",
]
PROFILE_TOOLS = {
    "简介": "get_company_profile",
    "股东信息": "get_shareholder_info",
    "主要人员": "get_key_personnel",
    "对外投资": "get_external_investments",
    "分支机构": "get_branches",
}


# ---------- 缓存读写 ----------

def cache_dir(data_dir):
    return os.path.join(data_dir, "cache", "qcc-raw")


def _payload(msg):
    """从 JSON-RPC 响应中取出业务数据。"""
    try:
        return json.loads(msg["result"]["content"][0]["text"])
    except Exception:
        return msg.get("result")


def _parse_body(raw):
    """兼容两种落盘格式：纯 JSON-RPC 对象，或 SSE（data: 行）。"""
    text = raw.strip()
    if text.startswith("{"):
        try:
            return _payload(json.loads(text))
        except Exception:
            pass
    for line in raw.splitlines():
        if not line.startswith("data: "):
            continue
        body = line[6:].strip()
        if not body.startswith("{"):
            continue
        try:
            return _payload(json.loads(body))
        except Exception:
            continue
    return None


def snapshot(path):
    """返回 (是否可用, 解析后的业务数据)。"""
    if not os.path.exists(path):
        return False, None
    raw = open(path, encoding="utf-8", errors="replace").read()
    if any(m in raw for m in BAD_MARKS):
        return False, None
    val = _parse_body(raw)
    return (val is not None), val


def is_quota(raw):
    return any(m in raw for m in QUOTA_MARKS)


def company_complete(root, name, level="basic"):
    """level=basic 只要求查名 + 工商 + 联系方式；enhanced 还要求 5 个画像维度。"""
    d = os.path.join(root, name)
    if not os.path.isdir(d):
        return False
    needed = ["get_company_by_query"] + TOOLS_BASIC
    if level == "enhanced":
        needed += TOOLS_ENHANCED
    return all(os.path.exists(os.path.join(d, t + ".json")) for t in needed)


# ---------- 采集 ----------

class Runner:
    def __init__(self, keys_path, account=None):
        self.ks = json.load(open(keys_path, encoding="utf-8"))
        self.ids = [a["id"] for a in self.ks["accounts"]]
        self.idx = self.ids.index(account or self.ks["active"])
        self.calls = 0
        self.quota_dead = False

    def key(self):
        return [a for a in self.ks["accounts"] if a["id"] == self.ids[self.idx]][0]["key"]

    def rotate(self):
        self.idx = (self.idx + 1) % len(self.ids)

    def call(self, tool, search_key):
        body = json.dumps({
            "jsonrpc": "2.0", "id": 1, "method": "tools/call",
            "params": {"name": tool, "arguments": {"searchKey": search_key}},
        }).encode()
        headers = {
            "Content-Type": "application/json",
            "Accept": "application/json, text/event-stream",
            "Authorization": "Bearer " + self.key(),
        }
        self.calls += 1
        try:
            r = urllib.request.urlopen(
                urllib.request.Request(ENDPOINT, data=body, headers=headers, method="POST"),
                timeout=60,
            )
            return r.read().decode("utf-8", "replace")
        except urllib.error.HTTPError as e:
            return e.read().decode("utf-8", "replace")
        except Exception as e:
            return json.dumps({"error": str(e)}, ensure_ascii=False)


def cached_credit_code(root, name):
    """基础已完成的企业，复用缓存里的统一社会信用代码，避免重复查名消耗积分。"""
    ok, info = snapshot(os.path.join(root, name, "get_company_by_query.json"))
    if not ok or not isinstance(info, dict):
        return ""
    return (info.get("企业信息") or {}).get("统一社会信用代码", "")


def collect_one(runner, root, name, level="enhanced"):
    """返回 (状态, 说明)。状态 ∈ ok / multi / quota / fail"""
    d = os.path.join(root, name)
    if company_complete(root, name, "basic"):
        code = cached_credit_code(root, name)
        full = name
        if not code:
            return "fail", "缓存缺少统一社会信用代码"
    else:
        raw = runner.call("get_company_by_query", name)
        ok, info = snapshot_raw(raw)
        if not ok:
            return ("quota" if is_quota(raw) else "fail"), raw[:160].replace("\n", " ")
        info = info or {}
        if info.get("匹配结果") != "唯一精确匹配":
            cands = [c.get("企业名称") for c in (info.get("企业信息") or [])][:4]
            return "multi", str(cands)
        entity = info.get("企业信息") or {}
        code = entity.get("统一社会信用代码", "")
        full = entity.get("企业名称", name)
        os.makedirs(d, exist_ok=True)
        open(os.path.join(d, "get_company_by_query.json"), "w", encoding="utf-8").write(raw)
    tools = TOOLS_BASIC + (TOOLS_ENHANCED if level == "enhanced" else [])
    for tool in tools:
        p = os.path.join(d, tool + ".json")
        if snapshot(p)[0]:
            continue
        raw = runner.call(tool, code or full)
        if snapshot_raw(raw)[0]:
            open(p, "w", encoding="utf-8").write(raw)
        elif is_quota(raw):
            return "quota", tool
        else:
            return "fail", tool
        time.sleep(0.8)
    return "ok", full


def snapshot_raw(raw):
    if any(m in raw for m in BAD_MARKS):
        return False, None
    val = _parse_body(raw)
    return (val is not None), val


def cmd_collect(args):
    data_dir = args.data_dir
    root = cache_dir(data_dir)
    os.makedirs(root, exist_ok=True)
    names = []
    if args.pending:
        p = os.path.join(data_dir, "cache", "qcc_pending.json")
        names = json.load(open(p, encoding="utf-8"))["companies"]
    elif args.file:
        names = [ln.strip() for ln in open(args.file, encoding="utf-8") if ln.strip()]
    else:
        sys.exit("需要 --file 或 --pending")
    if args.limit:
        all_names = names
        names = names[:args.limit]
    else:
        all_names = names

    runner = Runner(args.keys, args.account)
    done, pending = [], []
    for i, name in enumerate(names, 1):
        if company_complete(root, name, args.level):
            print(f"[{i}/{len(names)}] {name}: 已完成，跳过")
            done.append(name)
            continue
        print(f"[{i}/{len(names)}] {name}", flush=True)
        status, detail = collect_one(runner, root, name, args.level)
        if status == "ok":
            print(f"   ok: {detail}")
            done.append(name)
        elif status == "multi":
            print(f"   多候选，需人工选定: {detail}")
            pending.append((name, "multi"))
        elif status == "quota":
            print(f"   积分耗尽（{detail}），尝试轮换账号")
            pending.append((name, "quota"))
            tried = 0
            while tried < len(runner.ids) - 1:
                runner.rotate()
                tried += 1
                print(f"   切换到 {runner.ids[runner.idx]} 重试")
                status, detail = collect_one(runner, root, name, args.level)
                if status == "ok":
                    print(f"   ok: {detail}")
                    done.append(name)
                    pending.pop()
                    break
                if status != "quota":
                    print(f"   {status}: {detail}")
                    break
            else:
                print("   所有账号均无余额，停止")
                break
        else:
            print(f"   失败: {detail}")
            pending.append((name, status))

    print(f"\n调用 {runner.calls} 次 | 完成 {len(done)} | 待处理 {len(pending)}")
    for n, r in pending:
        print(f"  PENDING {n}: {r}")
    # 保留本次未处理的剩余名单，避免 --limit 时丢单
    carry = [n for n, _ in pending] + [n for n in all_names if n not in done and n not in [x for x, _ in pending]]
    if carry:
        out = os.path.join(data_dir, "cache", "qcc_pending.json")
        json.dump({"created": str(time.strftime("%Y-%m-%d")),
                   "note": "collect --pending 续跑；含未处理与未完成项",
                   "companies": carry},
                  open(out, "w", encoding="utf-8"), ensure_ascii=False, indent=1)
        print(f"待查名单已更新: {out}（{len(carry)} 家）")


# ---------- 入库 ----------

def cmd_ingest(args):
    data_dir = args.data_dir
    root = cache_dir(data_dir)
    csv_path = os.path.join(data_dir, "suppliers.csv")
    rows = list(csv.DictReader(open(csv_path, encoding="utf-8-sig")))
    have = {r["企业名称"] for r in rows}

    def row_of(name):
        ok, reg = snapshot(os.path.join(root, name, "get_company_registration_info.json"))
        if not ok:
            return None
        ok, con = snapshot(os.path.join(root, name, "get_contact_info.json"))
        con = con or {}
        ind = reg.get("国标行业") or {}
        region = reg.get("地区信息") or {}
        contacts = con.get("联系方式信息") or {}

        def first(key, field):
            arr = contacts.get(key) or [{}]
            return arr[0].get(field, "") if isinstance(arr[0], dict) else ""

        code = reg.get("统一社会信用代码", "")
        return [
            reg.get("企业名称", name), code, reg.get("法定代表人", ""), "", reg.get("登记状态", ""),
            reg.get("企业类型", ""), reg.get("注册地址") or "", "", reg.get("成立日期", ""),
            reg.get("注册资本", ""), reg.get("实缴资本", ""), reg.get("参保人数", ""),
            reg.get("人员规模", ""), reg.get("经营范围", ""),
            "/".join([x for x in [ind.get("门类"), ind.get("大类")] if x]),
            region.get("省份", ""), region.get("城市", ""), region.get("区域", ""),
            first("电话", "电话号码"), first("邮箱", "邮箱"), first("网址", "网址"),
            "企查查MCP(agent.qcc.com)", f"https://www.qcc.com/web/search?key={code}",
        ]

    added, profiles = [], {}
    for name in sorted(os.listdir(root)):
        if not company_complete(root, name):
            continue
        row = row_of(name)
        if row and row[0] not in have:
            added.append(row)
            have.add(row[0])
        prof = {"企业名称": name}
        for label, tool in PROFILE_TOOLS.items():
            ok, val = snapshot(os.path.join(root, name, tool + ".json"))
            if ok and val:
                prof[label] = val
        profiles[name] = prof

    if added:
        with open(csv_path, "a", newline="", encoding="utf-8-sig") as f:
            w = csv.writer(f)
            for r in added:
                w.writerow(r)
    out = os.path.join(data_dir, "supplier_profiles.json")
    json.dump({"来源": "企查查MCP(agent.qcc.com)", "生成日期": time.strftime("%Y-%m-%d"),
               "企业数": len(profiles), "企业": profiles},
              open(out, "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    print(f"CSV 新增 {len(added)} 家 → 总计 {len(rows) + len(added)} 家")
    print(f"增强档案 {len(profiles)} 家 → {out}")
    for r in added:
        print("  +", r[0], r[1])


# ---------- 状态 ----------

def cmd_status(args):
    data_dir = args.data_dir
    root = cache_dir(data_dir)
    names = sorted(n for n in os.listdir(root) if os.path.isdir(os.path.join(root, n))) if os.path.isdir(root) else []
    basic = [n for n in names if company_complete(root, n, "basic")]
    enhanced = [n for n in names if company_complete(root, n, "enhanced")]
    csv_path = os.path.join(data_dir, "suppliers.csv")
    in_csv = []
    if os.path.exists(csv_path):
        in_csv = list(csv.DictReader(open(csv_path, encoding="utf-8-sig")))
    partial = []
    for n in names:
        d = os.path.join(root, n)
        if not company_complete(root, n, "basic"):
            partial.append((n, len([f for f in os.listdir(d) if f.endswith(".json")])))
    print(f"缓存目录      : {len(names)} 家")
    print(f"  基础完整    : {len(basic)} 家（工商+联系方式）")
    print(f"  增强完整    : {len(enhanced)} 家（+简介/股东/人员/投资/分支）")
    print(f"suppliers.csv : {len(in_csv)} 家")
    if enhanced:
        need = len(basic) - len(enhanced)
        if need > 0:
            print(f"待补增强维度  : {need} 家（≈{need * len(TOOLS_ENHANCED)} 次调用）")
    if partial:
        print(f"基础未完成    : {len(partial)} 家")
        for n, k in partial:
            print(f"  - {n}: {k}/8")
    pend = os.path.join(data_dir, "cache", "qcc_pending.json")
    if os.path.exists(pend):
        p = json.load(open(pend, encoding="utf-8"))
        print(f"待查名单      : {len(p.get('companies', []))} 家（{pend}）")


def main():
    ap = argparse.ArgumentParser(description="企查查 MCP 批量采集")
    ap.add_argument("--data-dir", default=DEFAULT_DATA)
    ap.add_argument("--keys", default=DEFAULT_KEYS)
    sub = ap.add_subparsers(dest="cmd", required=True)

    p = sub.add_parser("status", help="查看采集进度")
    p.set_defaults(func=cmd_status)

    p = sub.add_parser("collect", help="按名单采集")
    p.add_argument("--file", help="企业名单，每行一家")
    p.add_argument("--pending", action="store_true", help="续跑 cache/qcc_pending.json")
    p.add_argument("--limit", type=int, help="本批最多处理几家")
    p.add_argument("--level", choices=["basic", "enhanced"], default="enhanced",
                   help="basic=工商+联系方式；enhanced=再加 5 个画像维度（默认）")
    p.add_argument("--account", help="指定账号 ID，默认用 Key 池的 active")
    p.set_defaults(func=cmd_collect)

    p = sub.add_parser("ingest", help="缓存 → CSV / profiles")
    p.set_defaults(func=cmd_ingest)

    args = ap.parse_args()
    args.func(args)


if __name__ == "__main__":
    main()
