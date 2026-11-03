#!/usr/bin/env python3
"""政府采购中标供应商发现器

从中国政府采购网检索"中标/成交"类公告，提取中标供应商名称、金额、采购人，
输出候选池供企查查体检使用。

用法:
    python scripts/discover_bidders.py --keywords 连接器,注塑件,线束 --months 12
    python scripts/discover_bidders.py --keywords-file 关键词.txt --max-per-kw 20

说明:
    - 政府采购网对普通 HTTP 请求返回"频繁访问"反爬页，必须用真实浏览器（Playwright）。
    - 公告列表默认只查近一周，必须显式传时间范围（timeType=6 + start/end）。
    - 只保留成交/中标/结果类公告，招标公告和终止公告不含供应商名称。
"""
import argparse
import json
import os
import re
import sys
import time
from datetime import date, timedelta

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8")

PROJECT_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
OUT_DIR = os.path.join(PROJECT_ROOT, "backend", "data", "cache", "candidates")
BASE = "http://search.ccgp.gov.cn/bxsearch"

POSITIVE = ("成交公告", "中标公告", "结果公告", "成交结果", "中标结果")
NEGATIVE = ("终止公告", "废标", "更正公告", "招标公告", "询价公告", "竞争性谈判公告",
            "单一来源", "资格预审", "邀请公告")


def find_chromium():
    """找本机已安装的 Playwright Chromium；找不到就让 Playwright 用自己的。"""
    local = os.path.expanduser("~/AppData/Local/ms-playwright")
    if os.path.isdir(local):
        versions = sorted(
            (d for d in os.listdir(local) if d.startswith("chromium-")),
            key=lambda x: int(re.sub(r"\D", "", x) or 0),
            reverse=True,
        )
        for v in versions:
            exe = os.path.join(local, v, "chrome-win64", "chrome.exe")
            if os.path.exists(exe):
                return exe
    return None


def classify(title):
    if any(n in title for n in NEGATIVE):
        return "skip"
    if any(p in title for p in POSITIVE):
        return "result"
    return "skip"


def search_url(keyword, start, end, page=1):
    import urllib.parse
    return (f"{BASE}?searchtype=1&page_index={page}&bidSort=0&pinMu=0&bidType=0"
            f"&kw={urllib.parse.quote(keyword)}"
            f"&start_time={start.replace('-', '%3A')}&end_time={end.replace('-', '%3A')}"
            f"&timeType=6&displayZone=&zoneId=&pppStatus=0&agentName="
            f"&dbselect=bidx&kwdata=全文检索")


def normalize_amount(raw):
    """统一成「万元」。公告里有的写万元，有的直接写元且带多位小数。"""
    if not raw:
        return ""
    try:
        val = float(raw.replace(",", ""))
    except ValueError:
        return raw
    # 带小数点的通常是「万元」口径；纯大整数多半是「元」
    if "." in raw:
        return f"{val:.2f}"
    if val > 10000:
        return f"{val / 10000:.2f}"
    return f"{val:.2f}"


def parse_detail(text):
    """从详情页正文提取关键字段。"""
    out = {}
    for key, field in (("供应商名称", "supplier"), ("成交供应商", "supplier"),
                       ("中标供应商", "supplier"), ("中标人", "supplier")):
        m = re.search(key + r"[：:]\s*([^\s，,。；;]{2,40})", text)
        if m:
            out[field] = m.group(1).strip()
            break
    m = re.search(r"中标（成交）金额[：:]\s*([\d\.,]+)", text) or \
        re.search(r"成交金额[：:]\s*([\d\.,]+)", text) or \
        re.search(r"中标金额[：:]\s*([\d\.,]+)", text)
    if m:
        out["amount"] = m.group(1)
    m = re.search(r"采购人(?:信息)?[：:\s]*\n?\s*名?\s*称?[：:]\s*([^\n]{2,40})", text)
    if m:
        out["buyer"] = m.group(1).strip()
    return out


def main():
    ap = argparse.ArgumentParser(description="政府采购中标供应商发现")
    ap.add_argument("--keywords", help="逗号分隔的关键词")
    ap.add_argument("--keywords-file", help="关键词文件，每行一个")
    ap.add_argument("--months", type=int, default=12, help="回溯月数，默认 12")
    ap.add_argument("--max-per-kw", type=int, default=15, help="每个关键词最多抓几条成交公告")
    ap.add_argument("--max-pages", type=int, default=3, help="每个关键词翻几页")
    ap.add_argument("--delay", type=float, default=2.5, help="每次翻页/进详情的间隔秒数")
    ap.add_argument("--out", help="输出文件，默认 backend/data/cache/candidates/bidders_<日期>.json")
    args = ap.parse_args()

    keywords = []
    if args.keywords:
        keywords += [k.strip() for k in args.keywords.split(",") if k.strip()]
    if args.keywords_file:
        keywords += [ln.strip() for ln in open(args.keywords_file, encoding="utf-8") if ln.strip()]
    if not keywords:
        sys.exit("需要 --keywords 或 --keywords-file")

    try:
        from playwright.sync_api import sync_playwright
    except ImportError:
        sys.exit("需要 Playwright：pip install playwright")

    end = date.today()
    start = end - timedelta(days=args.months * 30)
    s, e = start.isoformat(), end.isoformat()
    exe = find_chromium()
    results, seen = [], set()

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True, **({"executable_path": exe} if exe else {}))
        page = browser.new_page(
            user_agent="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "
                       "(KHTML, like Gecko) Chrome/120.0 Safari/537.36")
        for kw in keywords:
            print(f"\n=== 关键词: {kw} ({s} ~ {e}) ===", flush=True)
            for page_no in range(1, args.max_pages + 1):
                try:
                    page.goto(search_url(kw, s, e, page_no), timeout=60000, wait_until="domcontentloaded")
                except Exception as ex:
                    print(f"  第 {page_no} 页打开失败: {ex}")
                    break
                page.wait_for_timeout(2000)
                body = page.inner_text("body")
                m = re.search(r"共找到\s*([\d,]+)\s*条", body)
                total = int(m.group(1).replace(",", "")) if m else 0
                links = page.eval_on_selector_all(
                    "ul.vT-srch-result-list-bid li a",
                    "els => els.map(e => ({t: e.innerText.trim(), h: e.href}))")
                print(f"  第 {page_no} 页: 共 {total} 条，本页 {len(links)} 条")
                if not links:
                    break
                got = 0
                for item in links:
                    if classify(item["t"]) != "result":
                        continue
                    if item["h"] in seen:
                        continue
                    if got >= args.max_per_kw:
                        break
                    seen.add(item["h"])
                    try:
                        page.goto(item["h"], timeout=60000, wait_until="domcontentloaded")
                        page.wait_for_timeout(1200)
                        text = page.inner_text("body")
                        info = parse_detail(text)
                    except Exception as ex:
                        print(f"    详情失败: {str(ex)[:60]}")
                        continue
                    if not info.get("supplier"):
                        print(f"    跳过（无供应商字段）: {item['t'][:40]}")
                        continue
                    rec = {
                        "供应商名称": info["supplier"],
                        "中标金额": normalize_amount(info.get("amount", "")),
                        "采购人": info.get("buyer", ""),
                        "项目名称": item["t"],
                        "关键词": kw,
                        "公告日期": "",
                        "来源链接": item["h"],
                        "发现源": "中国政府采购网",
                    }
                    dm = re.search(r"(\d{4})[年\-\.](\d{1,2})[月\-\.](\d{1,2})", text)
                    if dm:
                        rec["公告日期"] = f"{dm.group(1)}-{int(dm.group(2)):02d}-{int(dm.group(3)):02d}"
                    results.append(rec)
                    got += 1
                    print(f"    + {rec['供应商名称']} | {rec['中标金额'] or '-'}万 | {rec['采购人'][:20]}")
                    time.sleep(args.delay)
                if total <= page_no * 10 or got == 0:
                    break
                time.sleep(args.delay)
        browser.close()

    out = args.out or os.path.join(OUT_DIR, f"bidders_{date.today().isoformat()}.json")
    os.makedirs(os.path.dirname(out), exist_ok=True)
    existing = []
    if os.path.exists(out):
        try:
            existing = json.load(open(out, encoding="utf-8")).get("候选", [])
        except Exception:
            pass
    have = {(x.get("来源链接"), x.get("供应商名称")) for x in existing}
    merged = existing + [r for r in results if (r["来源链接"], r["供应商名称"]) not in have]
    json.dump({"生成时间": date.today().isoformat(), "关键词": keywords,
               "时间范围": [s, e], "候选数": len(merged), "候选": merged},
              open(out, "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    print(f"\n新增 {len(results)} 条，累计 {len(merged)} 条 → {out}")


if __name__ == "__main__":
    main()
