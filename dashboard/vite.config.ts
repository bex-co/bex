import { defineConfig } from "vite";

import { devtools } from "@tanstack/devtools-vite";
import tsconfigPaths from "vite-tsconfig-paths";

import { tanstackStart } from "@tanstack/react-start/plugin/vite";

import viteReact from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { nitro } from "nitro/vite";

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    devtools(),
    tsconfigPaths({ projects: ["./tsconfig.json"] }),
    tailwindcss(),
    tanstackStart(),
    viteReact(),
    nitro(),
  ],
  ssr: {
    // @ory/elements-react ships extensionless relative imports
    // (e.g. "./session-provider") that only resolve under bundler
    // resolution, not Node's strict ESM loader — bundle it for SSR too.
    noExternal: ["@ory/elements-react"],
  },
  build: {
    assetsDir: "assets",
    cssCodeSplit: false, // Bundle all CSS into one file for SSR
    sourcemap: true, // Enable source maps for better error debugging in production
    manifest: true, // Generate .vite/manifest.json for deterministic asset resolution
  },
});
