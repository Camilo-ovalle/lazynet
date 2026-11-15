"""lazynet logo builder - writes every asset as SVG (width/height = 4x viewBox)."""
import os
from pathlib import Path
from fontTools.ttLib import TTFont
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen

OUT = Path(__file__).parent / "out"
OUT.mkdir(exist_ok=True)

# Palette
DARK = "#121821"    # graphite navy (terminal background)
LIGHT = "#F3F5F2"   # warm off-white
MINT = "#3FE0A0"    # prompt green
AMBER = "#FFB547"   # cursor
MUTED = "#8A97A8"   # secondary text
MINT_D = "#0F9F6A"  # mint for light backgrounds (contrast)
AMBER_D = "#D98A12"

FONT_DIR = Path(os.environ.get("LAZYNET_FONT_DIR", Path(__file__).parent / "fonts"))
FONT = TTFont(FONT_DIR / "JetBrainsMono-ExtraBold.ttf")
GS = FONT.getGlyphSet()
CMAP = FONT.getBestCmap()
UPM = FONT["head"].unitsPerEm


def text_path(txt, x, baseline, size):
    """Return (svg path d, advance width) for txt drawn as outlines."""
    s = size / UPM
    ds, cx = [], x
    for ch in txt:
        g = CMAP[ord(ch)]
        pen = SVGPathPen(GS)
        GS[g].draw(TransformPen(pen, (s, 0, 0, -s, cx, baseline)))
        ds.append(pen.getCommands())
        cx += GS[g].width * s
    return " ".join(ds), cx - x


def svg(vb_w, vb_h, body, bg=None, rx=0):
    bgr = f'<rect width="{vb_w}" height="{vb_h}" rx="{rx}" fill="{bg}"/>' if bg else ""
    return (f'<svg xmlns="http://www.w3.org/2000/svg" width="{vb_w*4}" height="{vb_h*4}" '
            f'viewBox="0 0 {vb_w} {vb_h}">{bgr}{body}</svg>\n')


# ---------- Marks (64x64 grid) ----------
def mark_fanout(c1, c2, line=None):
    """A: prompt chevron fanning out to three devices (one command -> many)."""
    line = line or c1
    return f'''
<g fill="none" stroke-linecap="round" stroke-linejoin="round">
  <path d="M6 19 L19 32 L6 45" stroke="{c1}" stroke-width="7"/>
  <path d="M27 32 L35 32 M35 14 L35 50 M35 14 L46 14 M35 50 L46 50 M35 32 L46 32" stroke="{line}" stroke-width="4.5"/>
</g>
<circle cx="50" cy="14" r="6.5" fill="{c1}"/>
<circle cx="50" cy="50" r="6.5" fill="{c1}"/>
<rect x="43.5" y="25.5" width="14" height="13" rx="3" fill="{c2}"/>'''


def mark_sleepy(c1, c2, frame):
    """B: terminal window with prompt + cursor and 'z' sleeping marks (lazy)."""
    z_big, _ = text_path("z", 45, 17, 15)
    z_small, _ = text_path("z", 54, 9, 10)
    return f'''
<rect x="4" y="18" width="42" height="40" rx="9" fill="none" stroke="{frame}" stroke-width="4.5"/>
<path d="M13 31 L22 38 L13 45" fill="none" stroke="{c1}" stroke-width="5.5" stroke-linecap="round" stroke-linejoin="round"/>
<rect x="26" y="43" width="12" height="5" rx="1.5" fill="{c2}"/>
<path d="{z_big}" fill="{c1}"/>
<path d="{z_small}" fill="{c1}" opacity="0.7"/>'''


def mark_ztopo(c1, c2, line=None):
    """C: a 'z' drawn as a network path - four nodes linked like a topology."""
    line = line or c1
    return f'''
<path d="M14 16 L50 16 L14 48 L50 48" fill="none" stroke="{line}" stroke-width="6"
      stroke-linecap="round" stroke-linejoin="round"/>
<circle cx="14" cy="16" r="7" fill="{c1}"/>
<circle cx="50" cy="16" r="7" fill="{c1}"/>
<circle cx="14" cy="48" r="7" fill="{c1}"/>
<rect x="43" y="41" width="14" height="14" rx="3" fill="{c2}"/>'''


CONCEPTS = {
    "a-fanout": {
        "dark": lambda: mark_fanout(MINT, AMBER),
        "light": lambda: mark_fanout(MINT_D, AMBER_D),
        "icon": lambda: mark_fanout(MINT, AMBER),
    },
    "b-sleepy": {
        "dark": lambda: mark_sleepy(MINT, AMBER, LIGHT),
        "light": lambda: mark_sleepy(MINT_D, AMBER_D, DARK),
        "icon": lambda: mark_sleepy(MINT, AMBER, LIGHT),
    },
    "c-ztopo": {
        "dark": lambda: mark_ztopo(MINT, AMBER),
        "light": lambda: mark_ztopo(MINT_D, AMBER_D),
        "icon": lambda: mark_ztopo(MINT, AMBER),
    },
}


def lockup(mark_body, word_a, word_b, ca, cb):
    """Horizontal lockup: mark + 'lazy' + 'net' wordmark as outlines."""
    size = 40
    d1, w1 = text_path("lazy", 82, 54, size)
    d2, w2 = text_path("net", 82 + w1, 54, size)
    width = int(82 + w1 + w2 + 14)
    body = (f'<g transform="translate(8 8)">{mark_body}</g>'
            f'<path d="{d1}" fill="{ca}"/><path d="{d2}" fill="{cb}"/>')
    return width, 80, body


# Badge colours used by the real TUI (internal/theme, dark variant).
BADGE = {"CAT": "#5FAFFF", "NXS": "#D787FF", "ASA": "#FF8787"}
BORDER = "#2A3542"
PANEL = "#1E2834"


def _t(x, y, s, c, txt, w=500, anchor="start", op=None):
    o = f' opacity="{op}"' if op is not None else ""
    return (f'<text xml:space="preserve" x="{x}" y="{y}" font-family="JetBrains Mono, monospace" '
            f'font-weight="{w}" font-size="{s}" fill="{c}" text-anchor="{anchor}"{o}>{txt}</text>')


def _tui_base(status, status_color):
    """The real lazynet TUI layout: device pane, tabbed detail pane, status line.

    Every label, key and value here exists in the app (demo inventory, Info tab),
    so the image never promises a feature lazynet does not have.
    """
    W, H = 720, 430
    out = [f'<rect x="0" y="0" width="{W}" height="{H}" rx="14" fill="{DARK}"/>',
           f'<rect x="0" y="0" width="{W}" height="34" rx="14" fill="#1A222D"/>'
           f'<rect x="0" y="20" width="{W}" height="14" fill="#1A222D"/>']
    for cx in (22, 42, 62):
        out.append(f'<circle cx="{cx}" cy="17" r="6" fill="#3A4554"/>')
    out.append(f'<g transform="translate(318 5) scale(0.36)">{mark_ztopo(MINT, AMBER)}</g>')
    out.append(_t(344, 22, 12, MUTED, "lazynet", 700))

    # Devices pane
    out.append(f'<rect x="16" y="50" width="222" height="342" rx="8" fill="none" stroke="{BORDER}" stroke-width="2"/>')
    out.append(f'<rect x="28" y="42" width="72" height="16" fill="{DARK}"/>' + _t(32, 54, 12, MINT, "Devices", 800))
    rows = [("ACCESS", None), ("access-sw2", "CAT"), ("access-sw3", "CAT"),
            ("CORE", None), ("core-sw1", "CAT"),
            ("DATACENTER", None), ("dc-n9k-1", "NXS"), ("dc-n9k-2", "NXS"),
            ("FIREWALLS", None), ("fw-edge", "ASA")]
    y = 80
    for name, badge in rows:
        if badge is None:
            out.append(_t(30, y, 11, MUTED, name, 800))
        else:
            sel = name == "core-sw1"
            if sel:
                out.append(f'<rect x="24" y="{y-15}" width="206" height="21" rx="4" fill="{PANEL}"/>'
                           f'<rect x="24" y="{y-15}" width="3" height="21" rx="1.5" fill="{AMBER}"/>')
                out.append(_t(34, y, 12.5, AMBER, "›", 800))
            out.append(_t(48, y, 12.5, LIGHT if sel else "#C9D1DA", name, 700 if sel else 500))
            out.append(_t(222, y, 11, BADGE[badge], badge, 800, "end"))
        y += 24

    # Detail pane with the tabs drawn into its top border
    out.append(f'<rect x="248" y="50" width="456" height="342" rx="8" fill="none" stroke="{BORDER}" stroke-width="2"/>')
    out.append(f'<rect x="260" y="42" width="318" height="16" fill="{DARK}"/>')
    out.append(_t(264, 54, 12, MINT, "1 Info", 800)
               + _t(318, 54, 12, MUTED, "│ 2 Interface │ 3 Logs │ 4 Backups", 500))
    out.append(_t(264, 82, 12, MUTED, "Catalyst 9300 core switch, rack A1", 400))
    out.append(_t(264, 110, 13.5, LIGHT, "core-sw1", 800)
               + _t(340, 110, 12, BADGE["CAT"], "CAT", 800)
               + _t(372, 110, 12.5, "#C9D1DA", "IOS-XE 17.09.04a", 500))
    kv = [("Hostname", "core-sw1"), ("Model", "C9300-48P"), ("Serial", "FCW2318L0AB"),
          ("MAC", "706d.15a2.3b80"), ("IP", "10.10.0.11"), ("OS version", "IOS-XE 17.09.04a"),
          ("Uptime", "12 weeks, 3 days, 4 hours"), ("Clock", "2026-11-15 22:29:13")]
    for i, (k, v) in enumerate(kv):
        yy = 138 + i * 23
        out.append(_t(280, yy, 12.5, MUTED, k, 500) + _t(376, yy, 12.5, LIGHT, v, 500))
    out.append(_t(264, 346, 11.5, MUTED, "r refresh · p interface · v VLAN change · b backup", 500))

    # Status line
    out.append(_t(22, 416, 12, status_color, status, 700))
    out.append(_t(700, 416, 12, MUTED, "? help · q quit", 500, "end"))
    return W, H, "\n".join(out)


def mockup_terminal():
    """The Info tab of the TUI connected to a Catalyst."""
    W, H, body = _tui_base("connected to core-sw1", MINT)
    return svg(W, H, body)


def mockup_vlan():
    """The VLAN change confirmation: exact commands, explicit [y]."""
    W, H, body = _tui_base("review the change", AMBER)
    m = [f'<rect x="16" y="50" width="688" height="342" fill="{DARK}" opacity="0.72"/>',
         f'<rect x="150" y="80" width="420" height="300" rx="10" fill="#151C26" stroke="{AMBER}" stroke-width="2"/>',
         _t(172, 112, 14, LIGHT, "Confirm configuration change", 800)]
    kv = [("Device", "core-sw1 (10.10.0.11)"), ("Port", "GigabitEthernet1/0/7"),
          ("Current", "VLAN 110 PRINTERS"), ("Target", "VLAN 20 VOICE")]
    for i, (k, v) in enumerate(kv):
        yy = 142 + i * 22
        m.append(_t(186, yy, 12.5, MUTED, k, 500) + _t(258, yy, 12.5, AMBER if k == "Target" else LIGHT, v, 700 if k == "Target" else 500))
    m.append(_t(172, 248, 12.5, LIGHT, "These commands will be sent:", 700))
    for i, c in enumerate(["configure terminal", "interface GigabitEthernet1/0/7",
                           "switchport access vlan 20", "end"]):
        m.append(_t(186, 272 + i * 20, 12.5, MINT, c, 700))
    m.append(_t(172, 360, 12.5, AMBER, "[y]", 800) + _t(200, 360, 12.5, LIGHT, "yes, apply", 500)
             + _t(296, 360, 12.5, MUTED, "[n]", 800) + _t(324, 360, 12.5, MUTED, "no", 500))
    return svg(W, H, body + "\n" + "\n".join(m))


def build():
    (OUT / "mockup-terminal.svg").write_text(mockup_terminal())
    (OUT / "mockup-vlan.svg").write_text(mockup_vlan())
    for name, c in CONCEPTS.items():
        (OUT / f"{name}-mark-dark.svg").write_text(svg(64, 64, c["dark"](), DARK))
        (OUT / f"{name}-mark-light.svg").write_text(svg(64, 64, c["light"](), LIGHT))
        (OUT / f"{name}-mark-transparent.svg").write_text(svg(64, 64, c["light"]()))
        # App icon: mark on rounded square, 80x80 with padding
        icon = f'<g transform="translate(10 10) scale(0.94)">{c["icon"]()}</g>'
        (OUT / f"{name}-app-icon.svg").write_text(svg(80, 80, icon, DARK, rx=18))
        w, h, body = lockup(c["dark"](), "lazy", "net", LIGHT, MINT)
        (OUT / f"{name}-lockup-dark.svg").write_text(svg(w, h, body, DARK))
        w, h, body = lockup(c["light"](), "lazy", "net", DARK, MINT_D)
        (OUT / f"{name}-lockup-light.svg").write_text(svg(w, h, body, LIGHT))
    print("built", len(list(OUT.glob("*.svg"))), "svgs")


if __name__ == "__main__":
    build()
