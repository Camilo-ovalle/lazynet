# lazynet website

The landing page at <https://camilo-ovalle.github.io/lazynet>, built with
[Astro](https://astro.build).

```sh
cd site
nvm use            # Node 24 (see .nvmrc)
npm install
npm run dev        # http://localhost:4321/lazynet/
npm run build      # static site in dist/
```

| Path | Content |
|---|---|
| `src/data/content.ts` | All copy, real command outputs and install snippets |
| `src/components/` | One component per section (Hero, Commands, Install…) |
| `src/styles/global.css` | Design tokens (brand palette) and shared styles |
| `tapes/` | [VHS](https://github.com/charmbracelet/vhs) scripts for the terminal recordings |
| `scripts/sync-assets.mjs` | Copies the brand images from `../assets` into `public/brand/` |

The recordings (`public/media/`) are made in CI by `.github/workflows/pages.yml`.
To make them locally, install VHS (it needs `ttyd` and `ffmpeg`) and run, from
the repository root:

```sh
go build -o ~/.local/bin/lazynet ./cmd/lazynet
vhs site/tapes/tui.tape && vhs site/tapes/vlan.tape
```

Without recordings, the site shows the static mockups instead.

Deployment is described in [docs/releasing.md](../docs/releasing.md).
