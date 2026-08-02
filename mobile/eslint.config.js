// https://docs.expo.dev/guides/using-eslint/
const { defineConfig } = require("eslint/config");
const expoConfig = require("eslint-config-expo/flat");
const eslintPluginPrettierRecommended = require("eslint-plugin-prettier/recommended");

module.exports = defineConfig([
  expoConfig,
  eslintPluginPrettierRecommended,
  {
    ignores: [
      "dist/*",
      "__generated__",
      "node_modules/*",
      "src/generated-graphql/*",
    ],
  },
  {
    rules: {
      "@typescript-eslint/no-unused-vars": "off",
      // React Compiler rules introduced in react-hooks v7 (eslint-config-expo 57).
      // They flag long-standing Animated.Value/ref patterns across the app.
      // Re-enable once those screens are refactored (e.g. to reanimated).
      "react-hooks/refs": "off",
      "react-hooks/immutability": "off",
      "react-hooks/set-state-in-effect": "off",
    },
  },
]);
