"""lazynet final kit - concept B1 (Ventana)."""
from pathlib import Path
import build_logo as L
from build_b import b1
from build_logo import svg, lockup, DARK, LIGHT, MINT, AMBER, MINT_D, AMBER_D

OUT = Path(__file__).parent / "final"
(OUT / "svg").mkdir(parents=True, exist_ok=True)

dark = lambda: b1(MINT, AMBER, LIGHT, DARK)
light = lambda: b1(MINT_D, AMBER_D, DARK, LIGHT)

files = {
    "lazynet-mark-dark.svg": svg(64, 64, dark(), DARK),
    "lazynet-mark-light.svg": svg(64, 64, light(), LIGHT),
    "lazynet-mark-transparent.svg": svg(64, 64, light()),
    "lazynet-mark-mono-dark.svg": svg(64, 64, b1(DARK, DARK, DARK, LIGHT)),
    "lazynet-app-icon.svg": svg(80, 80, f'<g transform="translate(9 9) scale(0.97)">{dark()}</g>', DARK, rx=18),
}
w, h, body = lockup(dark(), "lazy", "net", LIGHT, MINT)
files["lazynet-lockup-dark.svg"] = svg(w, h, body, DARK)
files["lazynet-lockup-dark-transparent.svg"] = svg(w, h, body)
w, h, body = lockup(light(), "lazy", "net", DARK, MINT_D)
files["lazynet-lockup-light.svg"] = svg(w, h, body, LIGHT)
files["lazynet-lockup-light-transparent.svg"] = svg(w, h, body)

for name, content in files.items():
    (OUT / "svg" / name).write_text(content)

# Lockup body for HTML pages (no background)
w, h, body = lockup(dark(), "lazy", "net", LIGHT, MINT)
(OUT / "_lockup_inline.svg").write_text(
    f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {w} {h}" width="{w*6}" height="{h*6}">{body}</svg>')

# Terminal mockup with the B1 lockup in the header
L.mark_ztopo = lambda c1, c2, line=None: dark()
(OUT / "lazynet-mockup-terminal.svg").write_text(L.mockup_terminal())
(OUT / "lazynet-mockup-vlan.svg").write_text(L.mockup_vlan())
print("ok", len(files))
