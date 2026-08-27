import { describe, it, expect, vi } from "vitest";
import { act } from "react";
import { hydrateRoot } from "react-dom/client";
import { renderToString } from "react-dom/server";

import { LocalDateTime, LocalDate } from "@/common/components/local-time";
import { formatDateTime, formatDateLong } from "@/common/lib/format";

// A mid-day UTC instant: 20:28 UTC is a different clock reading (and, for the
// date-only case below, a different calendar day) in every timezone west of the
// prime meridian — the exact shape of the w6/m107 live repro.
const INSTANT = "2026-08-26T20:28:01.124Z";

/**
 * SSR-render `node`, then hydrate that exact markup and report both the server
 * HTML and the DOM text once hydration + its post-commit re-render have settled.
 * `recovered` collects React #418s (a server/client divergence) — the assertion
 * that the deferral is genuinely mismatch-free, not just suppressed.
 */
function ssrThenHydrate(node: React.ReactElement) {
  const html = renderToString(node);

  const container = document.createElement("div");
  container.innerHTML = html;
  document.body.appendChild(container);
  const recovered: unknown[] = [];
  const errSpy = vi.spyOn(console, "error").mockImplementation(() => {});
  let root: ReturnType<typeof hydrateRoot> | undefined;
  act(() => {
    root = hydrateRoot(container, node, {
      onRecoverableError: (e) => recovered.push(e),
    });
  });
  const afterHydrate = container.textContent ?? "";
  act(() => root?.unmount());
  container.remove();
  errSpy.mockRestore();

  return { html, afterHydrate, recovered };
}

describe("LocalDateTime", () => {
  it("never bakes a clock reading into the SSR markup", () => {
    // formatDateTime always renders an AM/PM clock; its absence proves the SSR
    // pass emitted the reserved-width placeholder, not the container's UTC time.
    const { html } = ssrThenHydrate(<LocalDateTime value={INSTANT} />);
    expect(html).not.toMatch(/\b[AP]M\b/);
    expect(html).toContain('data-slot="skeleton"');
    // The machine-readable instant is still present and exact.
    expect(html.toLowerCase()).toContain(`datetime="${INSTANT.toLowerCase()}"`);
  });

  it("shows the viewer-local time after hydration, with no #418", () => {
    const { afterHydrate, recovered } = ssrThenHydrate(
      <LocalDateTime value={INSTANT} />,
    );
    // Whatever timezone the test runner is in, the settled text is the local
    // formatter's output — the client-local value wins, deterministically.
    expect(afterHydrate).toBe(formatDateTime(INSTANT));
    expect(afterHydrate).toMatch(/\b[AP]M\b/);
    expect(recovered).toHaveLength(0);
  });

  it("renders the fallback (no placeholder) for a missing instant", () => {
    const { html, afterHydrate } = ssrThenHydrate(
      <LocalDateTime value={null} fallback="—" />,
    );
    expect(html).not.toContain('data-slot="skeleton"');
    expect(afterHydrate).toBe("—");
  });

  it("renders a span (not a nested time) when as='span'", () => {
    const html = renderToString(
      <time dateTime={INSTANT}>
        <LocalDateTime value={INSTANT} as="span" />
      </time>,
    );
    expect(html).not.toMatch(/<time[^>]*>\s*<time/);
  });
});

describe("LocalDate", () => {
  it("defers the date and settles on the viewer-local day, with no #418", () => {
    const { html, afterHydrate, recovered } = ssrThenHydrate(
      <LocalDate value={INSTANT} />,
    );
    expect(html).toContain('data-slot="skeleton"');
    expect(afterHydrate).toBe(formatDateLong(INSTANT));
    expect(recovered).toHaveLength(0);
  });
});
