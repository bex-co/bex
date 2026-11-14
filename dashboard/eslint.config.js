import js from "@eslint/js";
import globals from "globals";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import jsxA11yX from "eslint-plugin-jsx-a11y-x";
import tseslint from "typescript-eslint";
import { globalIgnores } from "eslint/config";

const DATE_LANGUAGE_MESSAGE =
  "Format dates in the dashboard language through src/common/lib/format, never the browser's or a fixed locale.";

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
  // Static accessibility rules for the app's JSX (w21/m395 t001). The tests'
  // fixtures are not product markup, so they are left out; the rendered output
  // of every page is checked by the axe pass instead (t002).
  {
    files: ["src/**/*.tsx"],
    ignores: ["**/*.test.tsx", "**/__tests__/**"],
    extends: [jsxA11yX.configs.recommended],
    rules: {
      // Moving focus into a dialog or an inline editor the user just opened is
      // focus management, not a page that steals focus on load; only a raw DOM
      // element's autoFocus is checked, as components decide for themselves.
      "jsx-a11y-x/no-autofocus": ["error", { ignoreNonDOM: true }],
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
  // Absolute times read in the dashboard language, never the browser's or a
  // hard-coded one (w4/600): format through src/common/lib/format, or hand
  // the i18n language to Intl explicitly.
  {
    files: ["src/**/*.{ts,tsx}"],
    ignores: [
      "src/common/lib/format/**",
      "**/*.test.{ts,tsx}",
      "**/__tests__/**",
    ],
    rules: {
      "no-restricted-imports": [
        "error",
        {
          paths: [
            {
              name: "date-fns",
              importNames: [
                "format",
                "formatDistance",
                "formatDistanceStrict",
                "formatDistanceToNow",
                "formatRelative",
                "intlFormat",
                "lightFormat",
              ],
              message: DATE_LANGUAGE_MESSAGE,
            },
          ],
          patterns: [
            {
              group: ["date-fns/locale", "date-fns/locale/*"],
              message: DATE_LANGUAGE_MESSAGE,
            },
          ],
        },
      ],
      "no-restricted-syntax": [
        "error",
        // A date with no locale, or the browser's (undefined, [] or "default").
        {
          selector:
            "CallExpression[callee.property.name=/^toLocale(Date|Time)String$/][arguments.length=0]",
          message: DATE_LANGUAGE_MESSAGE,
        },
        {
          selector:
            "CallExpression[callee.property.name=/^toLocale(Date|Time)?String$/][arguments.0.type='Identifier'][arguments.0.name='undefined']",
          message: DATE_LANGUAGE_MESSAGE,
        },
        {
          selector:
            "CallExpression[callee.property.name=/^toLocale(Date|Time)?String$/][arguments.0.type='ArrayExpression']",
          message: DATE_LANGUAGE_MESSAGE,
        },
        {
          selector:
            "CallExpression[callee.property.name=/^toLocale(Date|Time)?String$/][arguments.0.type='Literal'][arguments.0.value='default']",
          message: DATE_LANGUAGE_MESSAGE,
        },
        {
          selector:
            "CallExpression[callee.property.name='toLocaleString'][arguments.length=0][callee.object.callee.name='Date']",
          message: DATE_LANGUAGE_MESSAGE,
        },
        {
          selector:
            "CallExpression[callee.property.name='toLocaleString'][arguments.length=0][callee.object.name=/(date|At)$/i]",
          message: DATE_LANGUAGE_MESSAGE,
        },
        // Intl.DateTimeFormat for display: a fixed locale, the browser's, or none.
        // A bare `Intl.DateTimeFormat().resolvedOptions()` (the viewer's zone)
        // is allowed.
        {
          selector:
            ":matches(NewExpression, CallExpression)[callee.object.name='Intl'][callee.property.name='DateTimeFormat'][arguments.0.type=/^(Literal|ArrayExpression)$/]",
          message: DATE_LANGUAGE_MESSAGE,
        },
        {
          selector:
            ":matches(NewExpression, CallExpression)[callee.object.name='Intl'][callee.property.name='DateTimeFormat'][arguments.0.type='Identifier'][arguments.0.name='undefined']",
          message: DATE_LANGUAGE_MESSAGE,
        },
        {
          selector:
            "NewExpression[callee.object.name='Intl'][callee.property.name='DateTimeFormat'][arguments.length=0]",
          message: DATE_LANGUAGE_MESSAGE,
        },
      ],
    },
  },
  {
    ignores: ["src/graphql/definitions.ts"],
  },
]);
