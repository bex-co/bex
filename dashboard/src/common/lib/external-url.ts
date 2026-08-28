/**
 * Classifier for an untrusted rich-content (Markdown) link destination.
 *
 * The prior `href.startsWith("http")` test in the Markdown renderer classified a
 * protocol-relative destination (`//host`, or a backslash-obfuscated `/\host`) as
 * *internal*, so it navigated the trusted dashboard tab off-origin with no
 * external treatment (no `target="_blank"`, no `rel="noopener"`, no external
 * icon). Resolve the destination and classify by scheme + authority instead:
 *
 *   - a relative or root-relative path (no scheme, not `//`) stays internal;
 *   - an http(s) absolute OR protocol-relative authority is external — opened in
 *     a new tab with noopener, never navigating the current tab silently;
 *   - `mailto:` / `tel:` stay in-place links;
 *   - anything else (`javascript:`, `data:`, …) is refused (no href returned, so
 *     the caller renders plain text).
 *
 * A fixed resolution base keeps the result identical on the server and client (no
 * `window` dependency, hence no hydration mismatch). This mirrors safe-next.ts's
 * leading-slash-then-parse guard, applied to the Markdown-link output shape.
 */
export function classifyHref(href: string | undefined): {
  safeHref: string | undefined;
  isExternal: boolean;
} {
  if (!href) return { safeHref: href, isExternal: false };
  // WHATWG URL parsing folds `\` to `/` for special schemes, so normalize first
  // to catch `/\host`, `\/host`, and similar authority-introducing obfuscations.
  const normalized = href.replace(/\\/g, "/");
  const hasSchemeOrAuthority =
    /^[a-z][a-z0-9+.-]*:/i.test(normalized) || normalized.startsWith("//");
  if (!hasSchemeOrAuthority) {
    // Relative path, root-relative path, or same-document (#/?) reference.
    return { safeHref: href, isExternal: false };
  }
  let url: URL;
  try {
    url = new URL(normalized, "https://dashboard.invalid");
  } catch {
    return { safeHref: undefined, isExternal: false };
  }
  if (url.protocol === "http:" || url.protocol === "https:") {
    return { safeHref: url.href, isExternal: true };
  }
  if (url.protocol === "mailto:" || url.protocol === "tel:") {
    return { safeHref: href, isExternal: false };
  }
  return { safeHref: undefined, isExternal: false };
}

/**
 * Guard for rendering a backend-supplied service URL as an anchor `href`.
 *
 * A service's `url` is a platform-constructed `https://…` origin, but the
 * dashboard must not trust a stored URL enough to place a `javascript:` or
 * `data:` scheme into an `href` (codex-security target #4): a hostile scheme
 * there executes on click in the dashboard origin. Returns the URL only when it
 * parses as an absolute http(s) URL (also folding `\` obfuscation and rejecting
 * protocol-relative `//host` authorities); otherwise `undefined`, so the caller
 * renders the value as inert text instead of a live link.
 */
export function safeHttpHref(
  url: string | null | undefined,
): string | undefined {
  if (!url) return undefined;
  const normalized = url.replace(/\\/g, "/");
  if (normalized.startsWith("//")) return undefined; // protocol-relative authority
  let parsed: URL;
  try {
    parsed = new URL(normalized);
  } catch {
    return undefined;
  }
  return parsed.protocol === "http:" || parsed.protocol === "https:"
    ? url
    : undefined;
}
