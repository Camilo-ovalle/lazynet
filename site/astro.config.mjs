// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

import { defineConfig } from "astro/config";

// The site is served from GitHub Pages as a project page, next to the APT
// repository (dists/, pool/, lazynet.gpg), which the pages workflow merges in.
export default defineConfig({
  site: "https://camilo-ovalle.github.io",
  base: "/lazynet",
  trailingSlash: "ignore",
  build: { format: "directory" },
});
