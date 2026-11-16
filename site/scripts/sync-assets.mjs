// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Copies the brand images from the repository's assets/ folder into
// public/brand/, so the site and the README share one set of files.
// public/brand/ is generated and git-ignored.
import { cpSync, mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const src = join(root, "..", "assets");
const dst = join(root, "public", "brand");
mkdirSync(dst, { recursive: true });
for (const f of [
  "lazynet-lockup-dark-transparent.svg",
  "lazynet-lockup-light-transparent.svg",
  "lazynet-mockup-terminal.png",
  "lazynet-mockup-vlan.png",
  "lazynet-github-social-1280x640.png",
]) {
  cpSync(join(src, f), join(dst, f));
}
console.log(`synced brand assets into ${dst}`);
