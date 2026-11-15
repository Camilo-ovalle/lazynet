# lazynet brand assets generator

Builds the logo, icon and README images in `assets/`.

| Script | Output |
|---|---|
| `build_logo.py` | Palette, glyph-to-path helpers, the logo concepts and the TUI mockups (`mockup_terminal`, `mockup_vlan`) |
| `build_b.py` | Variations of the "lazy terminal" mark (B1 is the chosen one) |
| `build_final.py` | The final kit as SVG in `final/`: marks, app icon, lockups and mockups |
| `social.html` | GitHub social preview (1280×640) |
| `shot.js` | Renders an SVG or HTML file to PNG with headless Chromium |

The mockups only show commands, keys and screens that exist in lazynet. Update
them when the TUI or the CLI changes.

## Requirements

- Python 3 with `fonttools`
- Node.js with `playwright` and its Chromium (`npx playwright install chromium`)
- [JetBrains Mono](https://github.com/JetBrains/JetBrainsMono/releases) TTFs
  (`Regular`, `Medium`, `Bold`, `ExtraBold`) in `fonts/`, or in the folder set
  by `LAZYNET_FONT_DIR`

`fonts/`, `node_modules/` and the build outputs are git-ignored.

## Build

```sh
cd assets/assets-generator
python3 -m venv .venv && .venv/bin/pip install fonttools
npm install playwright && npx playwright install chromium

.venv/bin/python build_final.py
node shot.js final/lazynet-mockup-terminal.svg ../lazynet-mockup-terminal.png 720 430 2
node shot.js final/lazynet-mockup-vlan.svg     ../lazynet-mockup-vlan.png     720 430 2
node shot.js social.html                       ../lazynet-github-social-1280x640.png 1280 640 1
```

If Chromium fails to start with missing `libnss3`, install it with
`sudo apt install libnss3`.
