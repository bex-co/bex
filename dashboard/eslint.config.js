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
