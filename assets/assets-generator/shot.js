// Renders an HTML page or an SVG file to PNG with headless Chromium.
//
//   node shot.js <input.html|input.svg> <output.png> [width height scale]
//
// JetBrains Mono is loaded from ./fonts (or $LAZYNET_FONT_DIR) and embedded
// into the page, and SVGs are inlined, so text renders with the real font
// instead of a system fallback.
const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');

const [input, output, w = '1280', h = '640', scale = '1'] = process.argv.slice(2);
const fontDir = process.env.LAZYNET_FONT_DIR || path.join(__dirname, 'fonts');
const faces = [['Regular', 400], ['Medium', 500], ['Bold', 700], ['ExtraBold', 800]]
  .map(([n, wt]) => `@font-face{font-family:'JetBrains Mono';font-weight:${wt};` +
    `src:url(data:font/ttf;base64,${fs.readFileSync(path.join(fontDir, `JetBrainsMono-${n}.ttf`)).toString('base64')})}`).join('');

(async () => {
  const b = await chromium.launch();
  const p = await b.newPage({ viewport: { width: +w, height: +h }, deviceScaleFactor: +scale });
  if (input.endsWith('.svg')) {
    const svg = fs.readFileSync(input, 'utf8')
      .replace(/width="\d+" height="\d+"/, `width="${w}" height="${h}"`);
    await p.setContent(`<!doctype html><style>${faces}*{margin:0}body{background:transparent}` +
      `svg{display:block}</style>${svg}`);
  } else {
    await p.goto('file://' + path.resolve(input));
    await p.addStyleTag({ content: faces });
  }
  await p.evaluate(() => document.fonts.ready);
  await p.screenshot({ path: output, omitBackground: true });
  await b.close();
})();
