/**
 * True when running inside the Expo Go store client, where config-plugin
 * assets (embedded fonts, native modules added via prebuild) are unavailable.
 * Guarded require so Node-based unit tests can import consumers of this
 * module without the expo runtime.
 */
export const isExpoGo = (() => {
  try {
    // eslint-disable-next-line @typescript-eslint/no-var-requires
    const Constants = require("expo-constants").default;
    return Constants?.executionEnvironment === "storeClient";
  } catch {
    return false;
  }
})();
