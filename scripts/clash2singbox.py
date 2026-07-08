#!/usr/bin/env python3
"""
clash2singbox.py — 把 Clash Verge 的运行配置转为 juzheng 默认订阅（sing-box JSON）。

策略：以 juzheng 默认模板的 MITM 架构为基底（保留 anthropic→go-mitmproxy 解密链路、
tun-in/mixed-back inbound、to-mitmproxy/direct/block/dns-out 等基础设施 outbound），
把 Clash 的内容注入：
  - Clash proxies（vmess/trojan/ss...） → sing-box outbounds（普通节点）
  - Clash proxy-groups（select/url-test） → sing-box outbounds（selector/urltest 组）
  - Clash rules → sing-box route.rules（域名/IP/关键字映射，指向对应 outbound tag）

用法:
  python3 scripts/clash2singbox.py [<clash.yaml>] [输出.json]
默认:
  clash.yaml = ~/Library/Application Support/io.github.clash-verge-rev.clash-verge-rev/clash-verge.yaml
  输出 = ~/.juzheng/schemes/默认订阅.json
"""
import json
import os
import sys
from typing import Optional

import yaml

# ---------- 路径 ----------
VERGE_CFG = os.path.expanduser(
    "~/Library/Application Support/io.github.clash-verge-rev.clash-verge-rev/clash-verge.yaml"
)
TEMPLATE = os.path.join(os.path.dirname(__file__), "..", "configs", "singbox-template.json")
DEFAULT_OUT = os.path.expanduser("~/.juzheng/schemes/默认订阅.json")


# ---------- Clash → sing-box 节点转换 ----------
def convert_proxy(p: dict) -> Optional[dict]:
    """把单个 Clash proxy 转为 sing-box outbound。仅支持常见类型，未知类型返回 None。"""
    t = p.get("type")
    name = p.get("name")
    server = p.get("server")
    port = p.get("port")
    base = {"tag": name}
    if t == "vmess":
        ob = {
            "type": "vmess",
            "server": server,
            "server_port": port,
            "uuid": p.get("uuid"),
            "security": p.get("cipher") or "auto",
            "alter_id": p.get("alterId", 0),
        }
        # 传输：ws / grpc / http2（简化处理 ws）
        net = p.get("network", "tcp")
        ob["network"] = net
        if net == "ws":
            ws = p.get("ws-opts") or {}
            path = ws.get("path", "/")
            headers = ws.get("headers", {})
            ob["transport"] = {"type": "ws", "path": path}
            if headers:
                ob["transport"]["headers"] = headers
        if p.get("tls"):
            ob["tls"] = {"enabled": True, "server_name": p.get("sni") or server}
        return {**ob, **base}
    if t == "vless":
        ob = {
            "type": "vless", "server": server, "server_port": port,
            "uuid": p.get("uuid"), "flow": p.get("flow", ""),
            "network": p.get("network", "tcp"),
        }
        if p.get("tls"):
            ob["tls"] = {"enabled": True, "server_name": p.get("sni") or server}
        return {**ob, **base}
    if t == "trojan":
        ob = {
            "type": "trojan", "server": server, "server_port": port,
            "password": p.get("password"),
        }
        if p.get("sni"):
            ob["tls"] = {"enabled": True, "server_name": p.get("sni")}
        return {**ob, **base}
    if t in ("ss", "shadowsocks"):
        ob = {
            "type": "shadowsocks", "server": server, "server_port": port,
            "method": p.get("cipher"), "password": p.get("password"),
        }
        return {**ob, **base}
    if t == "hysteria2":
        ob = {
            "type": "hysteria2", "server": server, "server_port": port,
            "password": p.get("password") or p.get("auth"),
        }
        if p.get("up"):
            ob["up_mbps"] = p.get("up")
        if p.get("down"):
            ob["down_mbps"] = p.get("down")
        return {**ob, **base}
    # 未知类型跳过
    return None


def convert_group(g: dict) -> dict:
    """把 Clash proxy-group 转为 sing-box selector/urltest。
    关键：把成员里的 DIRECT/REJECT/NONE 映射成 sing-box 的 direct/block。
    Clash 组还可能引用其他组（如 '🔰 节点选择' 引用 '♻️ 自动选择'），这些保留。"""
    name = g.get("name")
    members = g.get("proxies", [])
    t = g.get("type")
    # Clash 特殊目标 → sing-box 基础设施 outbound tag
    outbound_map = {"DIRECT": "direct", "REJECT": "block", "REJECT-DROP": "block", "NONE": "direct", "PASS": "direct"}
    mapped = [outbound_map.get(m, m) for m in members]
    if t == "url-test":
        ob = {
            "type": "urltest", "tag": name, "outbounds": mapped,
            "url": g.get("url") or "https://www.gstatic.com/generate_204",
            "interval": g.get("interval", "3m") if isinstance(g.get("interval"), int)
            else str(g.get("interval", "3m")) + "s" if isinstance(g.get("interval"), int)
            else (g.get("interval") or "3m"),
        }
        return ob
    # select（含 fallback 等）统一为 selector
    default = mapped[0] if mapped else ""
    ob = {"type": "selector", "tag": name, "outbounds": mapped, "default": default}
    return ob


# ---------- Clash 规则 → sing-box rule ----------
def convert_rules(clash_rules: list, group_tags: set) -> list:
    """把 Clash rules 转 sing-box route.rules。只映射域名/IP/关键字。
    Clash rule 形如 DOMAIN-SUFFIX,xxx.com,策略组名 → sing-box {domain_suffix, outbound}。
    未命中已知策略组的 outbound 的规则，fallback 到 final。"""
    sb_rules = []
    # 聚合同类同 outbound 的规则，减少条目数（sing-box 支持 domain_suffix 数组）
    bucket = {}  # (field, outbound) -> set(values)
    order = []   # 保持 outbound 顺序
    for r in clash_rules:
        parts = [x.strip() for x in r.split(",")]
        if len(parts) < 2:
            continue
        kind = parts[0]
        val = parts[1]
        outbound = parts[2] if len(parts) >= 3 else "direct"
        # 仅处理已知策略组/直连/拦截，避免悬空引用
        if outbound not in group_tags and outbound not in ("DIRECT", "REJECT"):
            outbound = "proxy-node"
        # 映射 Clash 特殊 outbound
        outbound_map = {"DIRECT": "direct", "REJECT": "block"}
        outbound = outbound_map.get(outbound, outbound)
        field = None
        if kind == "DOMAIN-SUFFIX":
            field = "domain_suffix"
        elif kind == "DOMAIN":
            field = "domain"
        elif kind == "DOMAIN-KEYWORD":
            field = "domain_keyword"
        elif kind == "IP-CIDR":
            field = "ip_cidr"
        elif kind == "IP-CIDR6":
            field = "ip_cidr"
        elif kind == "GEOIP":
            field = "geoip"
        elif kind == "GEOSITE":
            field = "geosite"
        elif kind == "MATCH":
            # MATCH 是兜底，sing-box 用 route.final 处理
            continue
        else:
            # PROCESS-NAME 等不常见，跳过
            continue
        key = (field, outbound)
        if key not in bucket:
            bucket[key] = []
            order.append(key)
        if val not in bucket[key]:
            bucket[key].append(val)

    for key in order:
        field, outbound = key
        rule = {"outbound": outbound}
        rule[field] = bucket[key]
        sb_rules.append(rule)
    return sb_rules


# ---------- 主流程 ----------
def main():
    clash_path = sys.argv[1] if len(sys.argv) > 1 else VERGE_CFG
    out_path = sys.argv[2] if len(sys.argv) > 2 else DEFAULT_OUT

    print(f"读取 Clash 配置: {clash_path}")
    with open(clash_path) as f:
        clash = yaml.safe_load(f)

    print(f"读取 juzheng 模板: {TEMPLATE}")
    with open(TEMPLATE) as f:
        sb = json.load(f)

    # 1. 节点：转 Clash proxies，替换模板的占位 proxy-node
    nodes = []
    for p in clash.get("proxies", []):
        ob = convert_proxy(p)
        if ob:
            nodes.append(ob)
        else:
            print(f"  跳过未知类型节点: {p.get('name')} ({p.get('type')})")
    print(f"转换节点: {len(nodes)}")

    # 2. 代理组
    groups = []
    group_tags = set()
    for g in clash.get("proxy-groups", []):
        gob = convert_group(g)
        groups.append(gob)
        group_tags.add(g.get("name"))
    print(f"转换策略组: {len(groups)}")

    # 3. 规则
    rules = convert_rules(clash.get("rules", []), group_tags)
    print(f"转换规则: {len(rules)} 条（聚合后）")

    # 4. 组装 outbounds：先删除模板的占位 proxy-node，再放节点、组、保留基础设施
    infra_tags = {"direct", "block", "dns", "to-mitmproxy", "dns-out", "proxy-node"}
    infra = [o for o in sb["outbounds"] if o.get("tag") in infra_tags]
    # 去掉占位 proxy-node（被实际节点替代）
    infra = [o for o in infra if o.get("tag") != "proxy-node"]

    # 节点 + 组 + 基础设施（to-mitmproxy 等保留）
    sb["outbounds"] = nodes + groups + infra

    # 5. 路由规则：保留模板的 anthropic→mitm 解密规则（最优先），再加 Clash 转换的规则
    template_rules = sb.get("route", {}).get("rules", [])
    # 模板规则在前（含 anthropic 分流），Clash 规则在后
    # 但模板的 "国内直连/其余 final" 要去掉，由 Clash 规则 + final 接管
    preserved = []
    for r in template_rules:
        # 保留 anthropic→mitm 和 dns 相关
        if r.get("outbound") == "to-mitmproxy" or r.get("outbound") == "dns-out":
            preserved.append(r)
        elif r.get("inbound") == "mixed-back":  # 回环打破规则
            preserved.append(r)
    sb["route"]["rules"] = preserved + rules
    # final：默认走第一个策略组（节点选择）或第一个节点
    final_target = "🔰 节点选择" if "🔰 节点选择" in group_tags else (nodes[0]["tag"] if nodes else "direct")
    sb["route"]["final"] = final_target
    print(f"final 出口: {final_target}")

    # 6. DNS：保留模板的 DNS（避免泄露），不直接用 Clash 的（格式差异大）
    # 7. mixed-port 同步到 sing-box 的 mixed-back（让系统代理能指向）
    # 模板 mixed-back 端口 1081 保留；Clash 的 mixed-port 不影响（sing-box 用自己的）

    # 输出
    os.makedirs(os.path.dirname(out_path), exist_ok=True)
    with open(out_path, "w") as f:
        json.dump(sb, f, ensure_ascii=False, indent=2)
    print(f"\n✓ 已生成: {out_path}")
    print(f"  节点 {len(nodes)} / 组 {len(groups)} / 规则 {len(rules)}")


if __name__ == "__main__":
    main()
