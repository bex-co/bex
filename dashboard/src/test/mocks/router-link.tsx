import type { AnchorHTMLAttributes, ReactNode } from "react";

/**
 * A stand-in for the router's `Link` in a test that renders a component on
 * its own, with no router around it: a plain anchor at the path the Link
 * would build from `to`, `params` and `hash`, so the test can still assert
 * where it leads. Every other prop reaches the anchor, as the real Link's do
 * (a menu item rendering it `asChild` gives it its role and handlers). Use it
 * as the `Link` of a `vi.mock("@tanstack/react-router")` that spreads the real
 * module.
 */
export function PlainLink({
  to,
  params,
  hash,
  children,
  // The router's own options, which an anchor has no attribute for.
  search: _search,
  preload: _preload,
  replace: _replace,
  ...anchor
}: {
  to: string;
  params?: Record<string, string>;
  hash?: string;
  search?: unknown;
  preload?: unknown;
  replace?: boolean;
  children?: ReactNode;
} & Omit<AnchorHTMLAttributes<HTMLAnchorElement>, "href">) {
  const path = Object.entries(params ?? {}).reduce(
    (built, [key, value]) => built.replace(`$${key}`, value),
    to,
  );
  return (
    <a {...anchor} href={hash ? `${path}#${hash}` : path}>
      {children}
    </a>
  );
}
