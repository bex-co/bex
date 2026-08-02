import { definePlugin } from "nitro";

const ASSOCIATION_PATHS = new Set([
  "/.well-known/apple-app-site-association",
  "/.well-known/assetlinks.json",
]);

// Nitro's static-file handler assigns text/plain to Apple's intentionally
// extensionless AASA filename after route-rule headers run. Override it at the
// response hook—the final response boundary—so both OS verifiers receive JSON.
export default definePlugin((nitroApp) => {
  nitroApp.hooks?.hook("response", (response, event) => {
    if (ASSOCIATION_PATHS.has(new URL(event.req.url).pathname)) {
      response.headers.set("content-type", "application/json; charset=utf-8");
    }
  });
});
