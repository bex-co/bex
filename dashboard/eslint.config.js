import js from "@eslint/js";
import globals from "globals";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import tseslint from "typescript-eslint";
import { globalIgnores } from "eslint/config";

export default tseslint.config([
  globalIgnores([
    "dist",
    ".output",
    "coverage",
    "*.config.ts",
    "*.config.js",
    "codegen.ts",
  ]),
  {
    files: ["**/*.{ts,tsx}"],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat["recommended-latest"],
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
        allowDefaultProject: [
          "*.ts",
          "*.tsx",
          "**/*.test.ts",
          "**/*.test.tsx",
          "**/__tests__/**/*.ts",
          "**/__tests__/**/*.tsx",
        ],
      },
    },
    rules: {
      // A route module's Route export wraps its components, and TanStack
      // Router's Vite plugin hot-reloads it, so the factories count as HOCs
      // (react-refresh 0.5 knows only React's own). A route module's other
      // non-component exports still count.
      "react-refresh/only-export-components": [
        "error",
        {
          allowConstantExport: true,
          allowCompoundComponents: true,
          extraHOCs: ["createFileRoute", "createRootRouteWithContext"],
        },
      ],
      "react-hooks/incompatible-library": "off",
      "@typescript-eslint/no-floating-promises": "error",
      "@typescript-eslint/no-unused-vars": [
        "error",
        {
          argsIgnorePattern: "^_",
        },
      ],
    },
  },
  {
    files: ["**/*.test.{ts,tsx}", "**/__tests__/**/*.{ts,tsx}"],
    languageOptions: {
      parserOptions: {
        projectService: false,
      },
    },
    rules: {
      "@typescript-eslint/no-explicit-any": "off",
      // Allow test utilities that modify external state
      "react-hooks/globals": "off",
      "react-hooks/immutability": "off",
      // Disable type-aware rules for test files
      "@typescript-eslint/no-floating-promises": "off",
    },
  },
  {
    ignores: ["src/graphql/definitions.ts"],
  },
]);
