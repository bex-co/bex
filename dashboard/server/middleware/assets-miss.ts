import { isAssetsPath } from "../assets-paths";

// Nitro's static handler runs first, so an asset path (`/_assets/**`, or the
// retired `/assets/**`) that reaches this middleware names no file in the
// build. Left alone it would fall through to the SSR renderer, which answers
// the visitor's own page (the Kratos session is embedded in it) under the
// asset route rule's `public, max-age=31536000, immutable` header (w21/m394
// t004, R25-08; w4/m348).
export default function assetsMiss(event: { url: URL }): Response | undefined {
  if (!isAssetsPath(event.url.pathname)) return undefined;
  return new Response("Not found", {
    status: 404,
    headers: {
      "Content-Type": "text/plain; charset=utf-8",
      "Cache-Control": "no-store",
      "X-Content-Type-Options": "nosniff",
    },
  });
}
