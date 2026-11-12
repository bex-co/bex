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
        // Keep `component` + `pendingComponent` in one split chunk — the
        // detail routes reuse the component as their own pending state. Must
        // match vite.config.ts's tanstackStart() grouping.
        defaultBehavior: [
          ["component", "pendingComponent"],
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
