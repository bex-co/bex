/**
 * AI SDK 6's provider-utils bundle contains one Node-only fallback that calls
 * `import(id)` for `node:module` / `node:dns`. The mobile chat UI never reaches
 * that server-side safe-fetch path, but Metro rejects a non-literal dynamic
 * import during static dependency collection before tree-shaking can remove it.
 *
 * Replace only that package's variable import with an explicit rejection. This
 * keeps accidental use fail-closed on native while leaving all UI-message,
 * transport, schema, and stream code byte-for-byte under the pinned SDK.
 */
module.exports = function disableAiSdkNodeLoader({ types: t }) {
  return {
    name: "disable-ai-sdk-node-loader-on-native",
    visitor: {
      CallExpression(path, state) {
        const filename = state.filename || "";
        if (
          !filename.includes(
            "/node_modules/@ai-sdk/provider-utils/dist/index.mjs",
          ) ||
          path.node.callee.type !== "Import" ||
          path.node.arguments.length !== 1 ||
          t.isStringLiteral(path.node.arguments[0])
        ) {
          return;
        }
        path.replaceWith(
          t.callExpression(
            t.memberExpression(t.identifier("Promise"), t.identifier("reject")),
            [
              t.newExpression(t.identifier("Error"), [
                t.stringLiteral(
                  "Node module loading is unavailable in the native client",
                ),
              ]),
            ],
          ),
        );
      },
    },
  };
};
