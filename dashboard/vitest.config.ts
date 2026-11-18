import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react-swc";
import path from "path";
import { tanstackRouter } from "@tanstack/router-plugin/vite";

export default defineConfig({
  plugins: [
    tanstackRouter({
      target: "react",
      autoCodeSplitting: true,
      quoteStyle: "double",
      semicolons: true,
      routeFileIgnorePattern: "__tests__",
      codeSplittingOptions: {
        // `pendingComponent` stays in the route's reference file so its
        // skeleton paints while the route's chunk loads (w3/066). Must match
        // vite.config.ts's tanstackStart() grouping.
        defaultBehavior: [
          ["component"],
          ["errorComponent"],
          ["notFoundComponent"],
        ],
      },
    }),
    react(),
  ],
  test: {
    globals: true,
    environment: "jsdom",
    // One zone for every test, whatever the machine's: CI runs in UTC, and a
    // test that only passes in the author's zone (w21/m337) fails there. The
    // hydration probe switches zones itself (src/test/hydration.ts).
    env: { TZ: "UTC" },
    testTimeout: 10000,
    // The sweep hydration-probes every page-level test's first render
    // (src/test/route-hydration-sweep.ts, w21/m337).
    setupFiles: ["./src/test/setup.ts", "./src/test/route-hydration-sweep.ts"],
    // Ory Elements' entries ship extensionless relative imports that only a
    // bundler resolves, as vite.config.ts's `ssr.noExternal` says; inlined, a
    // test can render its real flow components (w7/085).
    server: { deps: { inline: ["@ory/elements-react"] } },
    coverage: {
      provider: "v8",
      reporter: ["text-summary"],
      exclude: [
        "node_modules/",
        "src/test/",
        "**/*.d.ts",
        "**/*.config.*",
        "**/mockData",
        "dist/",
      ],
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
});
