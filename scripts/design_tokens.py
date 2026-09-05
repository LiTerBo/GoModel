#!/usr/bin/env python3
"""从设计规范文件生成 tokens.css（双规范：DARK/LIGHT + GoModel）。

用法:
  python scripts/design_tokens.py                     # 所有规范都生成
  python scripts/design_tokens.py dark                # 仅暗色 → prototype/dark/tokens.css
  python scripts/design_tokens.py light               # 仅亮色 → prototype/light/tokens.css
  python scripts/design_tokens.py gomodel             # 仅 GoModel → docs/design/tokens.css
  python scripts/design_tokens.py gomodel-blue        # 仅 GoModel 蓝 → docs/design/tokens-blue.css

规范 → 输出映射（用户 2026-08-04 定义）:
  design/DESIGN-DARK.md   → prototype/dark/tokens.css
  design/DESIGN-LIGHT.md  → prototype/light/tokens.css
  docs/design/DESIGN.md   → docs/design/tokens.css
  docs/design/DESIGN-BLUE.md → docs/design/tokens-blue.css

注意: 生成器只输出规范 YAML frontmatter 中**存在**的键；各规范的
colors/typography/rounded/spacing/layout/elevation/components 结构
不同时自动适配（无 elevation 则跳过阴影，无 dark-* 则跳过暗色覆盖）。
"""
import re, sys, yaml, os

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SPECS = {
    "dark":  {"src": os.path.join(ROOT, "design", "DESIGN-DARK.md"),
              "out": os.path.join(ROOT, "prototype", "dark", "tokens.css")},
    "light": {"src": os.path.join(ROOT, "design", "DESIGN-LIGHT.md"),
              "out": os.path.join(ROOT, "prototype", "light", "tokens.css")},
    "gomodel": {"src": os.path.join(ROOT, "docs/design/DESIGN.md"),
                "out": os.path.join(ROOT, "docs/design/tokens.css")},
    "gomodel-blue": {"src": os.path.join(ROOT, "docs/design/DESIGN-BLUE.md"),
                     "out": os.path.join(ROOT, "docs/design/tokens-blue.css")},
}


def parse_frontmatter(path):
    content = open(path, encoding="utf-8").read()
    # 兼容 ```markdown 包裹与裸 frontmatter
    m = (re.search(r'```markdown\s*---\n(.*?)\n---', content, re.DOTALL)
         or re.search(r'---\n(.*?)\n---', content, re.DOTALL))
    if not m:
        raise ValueError(f"frontmatter not found in {path}")
    return yaml.safe_load(m.group(1))


def main():
    targets = [a for a in sys.argv[1:]] or list(SPECS.keys())
    for t in targets:
        if t not in SPECS:
            print(f"⚠️ 未知规范: {t}（可选: dark/light/gomodel）")
            continue
        spec = SPECS[t]
        data = parse_frontmatter(spec["src"])
        os.makedirs(os.path.dirname(spec["out"]), exist_ok=True)
        build_css(data, spec["src"], spec["out"], t)


def build_css(data, src, out, tag):
    colors = data.get("colors", {})
    typo = data.get("typography", {})
    rounded = data.get("rounded", {})
    spacing = data.get("spacing", {})
    layout = data.get("layout", {})
    elev = data.get("elevation", {})
    comps = data.get("components", {})

    def resolve(val, depth=0):
        if not isinstance(val, str) or depth > 4:
            return val
        def rep(mo):
            sec, key = mo.group(1), mo.group(2)
            if sec == "colors":   return f"var(--{key})"
            if sec == "layout":   return f"var(--{key})"
            if sec == "spacing":  return f"var(--sp-{key})"
            if sec == "rounded":  return f"var(--radius-{key})"
            if sec == "elevation": return resolve(elev.get(key, ""), depth + 1)
            if sec == "typography":
                t = typo.get(key, {})
                return t.get("fontSize", val) if isinstance(t, dict) else val
            return val
        return re.sub(r'\{([a-z]+)\.([a-z0-9-]+)\}', rep, val)

    PROP_MAP = {
        "backgroundColor": ("background", lambda v: v),
        "textColor": ("color", lambda v: v),
        "borderColor": ("border", lambda v: f"1px solid {v}"),
        "rounded": ("border-radius", lambda v: v),
        "padding": ("padding", lambda v: v),
        "typography": ("font-size", lambda v: v),
        "height": ("height", lambda v: v),
        "width": ("width", lambda v: v),
        "maxHeight": ("max-height", lambda v: v),
        "flex": ("flex", lambda v: v),
        "transition": ("transition", lambda v: v),
        "direction": ("flex-direction", lambda v: v),
    }
    # 语义属性（描述子部分/状态，不直接输出为组件类 CSS）
    SEMANTIC_PROPS = {
        "activeIndicator", "collapsedWidth", "navItemHeight", "navItemRadius",
        "focusBorder", "fillColor", "sideWidth", "sideHeight", "headHeight",
        "panelFlex", "panelWidth", "direction", "labelContrastAlgorithm",
        "labelLight", "labelDark", "description",
    }

    css = []
    css.append(f"/* AUTO-GENERATED from {os.path.basename(src)} — {tag} 设计 token。")
    css.append("   修改规范后运行 scripts/design_tokens.py 重新生成。 */")
    css.append("")
    css.append(":root {")
    for k, v in colors.items():
        if isinstance(v, str):
            css.append(f"  --{k}: {v};")
    for k, v in layout.items():
        if isinstance(v, str): css.append(f"  --{k}: {v};")
    for k, v in spacing.items():
        if isinstance(v, str): css.append(f"  --sp-{k}: {v};")
    for k, v in rounded.items():
        if isinstance(v, str): css.append(f"  --radius-{k}: {v};")
    for k, v in elev.items():
        if k.startswith("level-") and isinstance(v, str):
            css.append(f"  --shadow-{k[6:]}: {resolve(v)};")
    for k, v in typo.items():
        if isinstance(v, dict) and "fontSize" in v:
            css.append(f"  --font-{k}: {v['fontSize']};")
    css.append("}")

    # 暗色主题覆盖（规范有 dark-* 键时）
    dark_keys = [k for k in colors if k.startswith("dark-")]
    if dark_keys:
        css.append('html[data-theme="dark"] {')
        for k in dark_keys:
            css.append(f"  --{k.split('-', 1)[1]}: {colors[k]};")
        css.append("}")

    css.append("")
    css.append("/* ── 组件类（components 自动生成，仅基础 chrome）── */")
    for name, props in comps.items():
        if not isinstance(props, dict):
            continue
        rules = []
        for prop, val in props.items():
            if prop in SEMANTIC_PROPS or prop not in PROP_MAP:
                continue
            cprop, conv = PROP_MAP[prop]
            rules.append(f"  {cprop}: {conv(resolve(val))};")
        if rules:
            css.append(f".{name} {{")
            css.extend(rules)
            css.append("}")
    css.append("")

    with open(out, "w") as f:
        f.write("\n".join(css))
    print(f"✅ {tag} tokens.css 已生成: {out} ({len(css)} 行)")


if __name__ == "__main__":
    main()