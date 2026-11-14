import { definePlugin } from "nitro";

import { SECURITY_HEADER_NAMES } from "../security-headers";

// The headers a response must carry whatever its status: the route rules set
// them on `event.res`, which h3 merges into a rendered page but not into a
// `Response` a handler returns itself, so a redirect, a 401, a 403 or a 404
// answered by a server function left without them (w21/m410 t003). Only the
// SECURITY_HEADER_NAMES are copied: the rule's cache policy is for rendered
// pages and is set where it belongs (`html-never-public.ts`, `assets-miss.ts`).

// What Nitro keeps for a matched `headers` route rule at runtime (nitro-nightly
// 3.0.1-20260310): the rule's record, with the header map under `options`. The
// hook's declared event type is `HTTPEvent`, which has no `context`; the event
// is an h3 event at runtime, so the field is read through this shape.
interface EventWithRouteRules {
  context?: {
    routeRules?: { headers?: { options?: Record<string, string> } };
  };
}

export default definePlugin((nitroApp) => {
  nitroApp.hooks?.hook("response", (response, event) => {
    const rules = (event as EventWithRouteRules).context?.routeRules?.headers
      ?.options;
    if (!rules) return;
    for (const [name, value] of Object.entries(rules)) {
      const lower = name.toLowerCase();
      if (SECURITY_HEADER_NAMES.has(lower) && !response.headers.has(lower)) {
        response.headers.set(lower, value);
      }
    }
  });
});
