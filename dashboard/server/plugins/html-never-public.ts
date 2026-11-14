import { definePlugin } from "nitro";

import { isAssetsPath } from "../assets-paths";

// Runs after route-rule headers. HTML embeds the visitor's Kratos session, so
// it is never `public`; a missing `/assets/**` file must not keep the route
// rule's immutable header (w21/m394 t004, R25-08).
export default definePlugin((nitroApp) => {
  nitroApp.hooks?.hook("response", (response, event) => {
    const isHTML = /\btext\/html\b/i.test(
      response.headers.get("content-type") ?? "",
    );
    if (
      isHTML &&
      /\bpublic\b/i.test(response.headers.get("cache-control") ?? "")
    ) {
      response.headers.set("cache-control", "private, no-store");
    } else if (
      response.status === 404 &&
      isAssetsPath(new URL(event.req.url).pathname)
    ) {
      response.headers.set("cache-control", "no-store");
    }
  });
});
