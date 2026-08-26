import type { ReactElement } from "react";
import { act } from "react";
import { hydrateRoot } from "react-dom/client";
import { renderToString } from "react-dom/server";
import { vi } from "vitest";

export interface HydrationProbe {
  /** The markup the SSR pass produced, at `serverNow`. */
  html: string;
  /**
   * Recoverable errors React raised while hydrating the SSR markup at
   * `clientNow`. A server/client text or attribute divergence — React error
   * #418 — lands here unless the diverging element carries
   * `suppressHydrationWarning`.
   */
  recovered: unknown[];
}

/**
 * Server-render `node` with the clock at `serverNow`, then hydrate that exact
 * markup with the clock at `clientNow`, and report what React complained about.
 *
 * The two clocks are what makes this a *boundary* probe: an elapsed-time
 * formatter (`formatRelativeAge`/`formatRelativeUntil`, w6/m102) reads Date.now()
 * once per render pass, so a bucket boundary crossed between the SSR render and
 * hydration makes the same node render different text on each side — the live
 * `/webhook/<id>` repro this guards against.
 */
export function hydrateAcrossBoundary(
  node: ReactElement,
  { serverNow, clientNow }: { serverNow: number; clientNow: number },
): HydrationProbe {
  const nowSpy = vi.spyOn(Date, "now").mockReturnValue(serverNow);
  const html = renderToString(node);

  nowSpy.mockReturnValue(clientNow);
  const container = document.createElement("div");
  container.innerHTML = html;
  document.body.appendChild(container);

  const recovered: unknown[] = [];
  // React logs the mismatch as well; onRecoverableError is the assertable
  // channel, so keep the console quiet.
  const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
  let root: ReturnType<typeof hydrateRoot> | undefined;
  act(() => {
    root = hydrateRoot(container, node, {
      onRecoverableError: (error) => recovered.push(error),
    });
  });
  act(() => root?.unmount());
  container.remove();
  errorSpy.mockRestore();
  nowSpy.mockRestore();

  return { html, recovered };
}
