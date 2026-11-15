"""lazynet - 3 variations of concept B (lazy terminal). SVG, width/height = 4x viewBox."""
from pathlib import Path
from build_logo import (text_path, svg, lockup, DARK, LIGHT, MINT, AMBER, MINT_D, AMBER_D)

OUT = Path(__file__).parent / "out_b"
OUT.mkdir(exist_ok=True)


def zz(c, x=45, y=17, big=15, small=10, dx=9, dy=8, op=0.7):
    d1, _ = text_path("z", x, y, big)
    d2, _ = text_path("z", x + dx, y - dy, small)
    return f'<path d="{d1}" fill="{c}"/><path d="{d2}" fill="{c}" opacity="{op}"/>'


def prompt(c1, c2, x=13, y=38, s=1.0):
    return (f'<g transform="translate({x} {y}) scale({s})">'
            f'<path d="M0 -7 L9 0 L0 7" fill="none" stroke="{c1}" stroke-width="5.5" '
            f'stroke-linecap="round" stroke-linejoin="round"/>'
            f'<rect x="13" y="5" width="12" height="5" rx="1.5" fill="{c2}"/></g>')


# B1 - Ventana: window chrome (title bar with dots), zz rising out of it
def b1(c1, c2, frame, inner):
    return f'''
<rect x="4" y="21" width="44" height="38" rx="8" fill="{inner}" stroke="{frame}" stroke-width="4"/>
<path d="M4 29 A8 8 0 0 1 12 21 H40 A8 8 0 0 1 48 29 V31 H4 Z" fill="{frame}"/>
<circle cx="11" cy="26" r="2" fill="{inner}"/><circle cx="17.5" cy="26" r="2" fill="{inner}"/>
{prompt(c1, c2, 12, 43)}
{zz(c1, 46, 19, 15, 10, 9, 7)}'''


# B2 - Cursor dormido: solid tile, the cursor itself is the one snoring
def b2(c1, c2, tile, ink):
    d1, _ = text_path("z", 33, 33, 16)
    d2, _ = text_path("z", 44, 23, 11)
    return f'''
<rect x="5" y="5" width="54" height="54" rx="13" fill="{tile}"/>
<path d="M15 31 L25 39 L15 47" fill="none" stroke="{ink}" stroke-width="6" stroke-linecap="round" stroke-linejoin="round"/>
<rect x="29" y="43" width="16" height="6" rx="2" fill="{c2}"/>
<path d="{d1}" fill="{ink}"/><path d="{d2}" fill="{ink}" opacity="0.6"/>'''


# B3 - Terminal de red: lazy terminal plugged into the network (cable -> device)
def b3(c1, c2, frame):
    return f'''
<rect x="4" y="18" width="40" height="30" rx="8" fill="none" stroke="{frame}" stroke-width="4.5"/>
{prompt(c1, c2, 12, 33, 0.9)}
<path d="M24 48 V56 H44" fill="none" stroke="{c1}" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"/>
<rect x="44" y="50" width="14" height="12" rx="3" fill="{c1}"/>
<rect x="48.5" y="55" width="5" height="4" rx="0.8" fill="{frame if frame != LIGHT else DARK}"/>
{zz(c1, 44, 15, 15, 10, 9, 7)}'''


VARIANTS = {
    "b1-ventana": {
        "dark": lambda: b1(MINT, AMBER, LIGHT, DARK),
        "light": lambda: b1(MINT_D, AMBER_D, DARK, LIGHT),
    },
    "b2-cursor-dormido": {
        "dark": lambda: b2(MINT, DARK, MINT, DARK),
        "light": lambda: b2(MINT_D, AMBER, DARK, LIGHT),
    },
    "b3-terminal-de-red": {
        "dark": lambda: b3(MINT, AMBER, LIGHT),
        "light": lambda: b3(MINT_D, AMBER_D, DARK),
    },
}


def build():
    for name, v in VARIANTS.items():
        (OUT / f"{name}-mark-dark.svg").write_text(svg(64, 64, v["dark"](), DARK))
        (OUT / f"{name}-mark-light.svg").write_text(svg(64, 64, v["light"](), LIGHT))
        (OUT / f"{name}-mark-transparent.svg").write_text(svg(64, 64, v["light"]()))
        icon = f'<g transform="translate(9 9) scale(0.97)">{v["dark"]()}</g>'
        (OUT / f"{name}-app-icon.svg").write_text(svg(80, 80, icon, DARK, rx=18))
        w, h, body = lockup(v["dark"](), "lazy", "net", LIGHT, MINT)
        (OUT / f"{name}-lockup-dark.svg").write_text(svg(w, h, body, DARK))
        w, h, body = lockup(v["light"](), "lazy", "net", DARK, MINT_D)
        (OUT / f"{name}-lockup-light.svg").write_text(svg(w, h, body, LIGHT))
    print("built", len(list(OUT.glob("*.svg"))), "svgs")


if __name__ == "__main__":
    build()
