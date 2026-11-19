import type { ReactNode } from "react";

/**
 * A stand-in for the router's `Link` in a test that renders a component on
 * its own, with no router around it: a plain anchor at the path the Link
 * would build from `to` and `params`, so the test can still assert where it
 * leads. Use it as the `Link` of a `vi.mock("@tanstack/react-router")` that
 * spreads the real module.
 */
export function PlainLink({
  to,
  params,
  className,
  children,
}: {
  to: string;
  params?: Record<string, string>;
  className?: string;
  children?: ReactNode;
}) {
  const href = Object.entries(params ?? {}).reduce(
    (path, [key, value]) => path.replace(`$${key}`, value),
    to,
  );
  return (
    <a href={href} className={className}>
      {children}
    </a>
  );
}
