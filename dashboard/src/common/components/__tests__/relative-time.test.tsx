import type { ReactElement } from "react";
import { hydrateRoot } from "react-dom/client";
import { renderToString } from "react-dom/server";
import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { RelativeAge, RelativeUntil } from "@/common/components/relative-time";
import { useNow } from "@/common/hooks/use-now";
import { hydrateAcrossBoundary } from "@/test/hydration";
import {
  formatRelativeAge,
  formatRelativeUntil,
} from "@/features/services/lib/format";

// w6/m102: formatRelativeAge/formatRelativeUntil measure elapsed time against a
// fresh Date.now(), evaluated once on the SSR pass and again on hydration. A
// bucket boundary crossed in between (here "now" → "1m", the exact shape of the
// live /webhook/<id> repro) makes the server and client render different text
// for the same node — React error #418. RelativeAge/RelativeUntil carry
// suppressHydrationWarning so that divergence is expected, not an error.

const INSTANT = "2026-08-26T12:00:00.000Z";
const SERVER_NOW = Date.parse(INSTANT) + 55_000; // renders "now"
const CLIENT_NOW = Date.parse(INSTANT) + 61_000; // renders "1m"

afterEach(() => {
  vi.restoreAllMocks();
});

/** Render at SERVER_NOW ("now"), hydrate at CLIENT_NOW ("1m"). */
function recoverableErrors(node: ReactElement): unknown[] {
  return hydrateAcrossBoundary(node, {
    serverNow: SERVER_NOW,
    clientNow: CLIENT_NOW,
  }).recovered;
}

describe("RelativeAge / RelativeUntil", () => {
  it("renders the same visible text as the bare formatter", () => {
    vi.spyOn(Date, "now").mockReturnValue(SERVER_NOW + 3 * 3600_000);
    render(<RelativeAge value={INSTANT} />);
    expect(screen.getByText(formatRelativeAge(INSTANT))).toBeInTheDocument();
  });

  it("renders the machine-readable instant in dateTime", () => {
    vi.spyOn(Date, "now").mockReturnValue(SERVER_NOW);
    const { container } = render(<RelativeAge value={INSTANT} />);
    const time = container.querySelector("time");
    expect(time).toHaveAttribute("dateTime", INSTANT);
    expect(time).toHaveTextContent("now");
  });

  it("renders future instants through formatRelativeUntil", () => {
    const future = new Date(CLIENT_NOW + 5 * 60_000).toISOString();
    vi.spyOn(Date, "now").mockReturnValue(CLIENT_NOW);
    render(<RelativeUntil value={future} />);
    expect(screen.getByText(formatRelativeUntil(future))).toBeInTheDocument();
  });

  it("renders the fallback, with no <time>, for a missing or unparseable value", () => {
    const { container: missing } = render(<RelativeAge value={null} />);
    expect(missing.querySelector("time")).toBeNull();
    expect(missing).toHaveTextContent("—");

    const { container: custom } = render(
      <RelativeAge value={undefined} fallback="Never used" />,
    );
    expect(custom).toHaveTextContent("Never used");

    const { container: garbage } = render(<RelativeAge value="not-a-date" />);
    expect(garbage.querySelector("time")).toBeNull();
    expect(garbage).toHaveTextContent("—");
  });

  it('renders a guarded <span> instead of a nested <time> when as="span"', () => {
    vi.spyOn(Date, "now").mockReturnValue(SERVER_NOW);
    const { container } = render(<RelativeAge value={INSTANT} as="span" />);
    expect(container.querySelector("time")).toBeNull();
    expect(container.querySelector("span")).toHaveTextContent("now");
  });

  it("survives an SSR/hydration bucket boundary without a React #418", () => {
    expect(recoverableErrors(<RelativeAge value={INSTANT} />)).toEqual([]);
    expect(
      recoverableErrors(<RelativeAge value={INSTANT} as="span" />),
    ).toEqual([]);
    expect(
      recoverableErrors(
        <RelativeUntil value={new Date(CLIENT_NOW + 61_000).toISOString()} />,
      ),
    ).toEqual([]);
  });

  it("caches the hydration snapshot even when the clock advances between reads", () => {
    let now = SERVER_NOW;
    vi.spyOn(Date, "now").mockImplementation(() => now++);
    const errors = vi.spyOn(console, "error").mockImplementation(() => {});
    const node = <RelativeAge value={INSTANT} />;
    const container = document.createElement("div");
    container.innerHTML = renderToString(node);
    document.body.appendChild(container);
    const recovered: unknown[] = [];
    let root: ReturnType<typeof hydrateRoot> | undefined;
    now = CLIENT_NOW;
    try {
      act(() => {
        root = hydrateRoot(container, node, {
          onRecoverableError: (error) => recovered.push(error),
        });
      });
      expect(container.querySelector("time")).toHaveAttribute(
        "dateTime",
        INSTANT,
      );
      expect(container.querySelector("time")).not.toBeEmptyDOMElement();
      expect(recovered).toEqual([]);
      expect(errors.mock.calls).toEqual([]);
    } finally {
      act(() => root?.unmount());
      container.remove();
    }
  });

  it("is the guard: the same boundary crossed by a bare formatter does mismatch", () => {
    // Negative control — proves the test above would actually catch a
    // regression that dropped suppressHydrationWarning from the wrapper. The
    // formatter has to run inside a component so each pass re-reads Date.now().
    const BareAge = () => (
      <time dateTime={INSTANT}>{formatRelativeAge(INSTANT)}</time>
    );
    expect(recoverableErrors(<BareAge />).length).toBeGreaterThan(0);
  });
});

describe("RelativeTime mounted clock", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(Date.parse(INSTANT));
  });
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it("advances unchanged ages and countdowns while preserving exact metadata and span rendering", () => {
    const future = new Date(Date.parse(INSTANT) + 5 * 60_000).toISOString();
    const { container } = render(
      <>
        <RelativeAge value={INSTANT} title="exact source instant" />
        <RelativeUntil value={future} as="span" title="exact next instant" />
        <RelativeAge value={null} fallback="Never" />
      </>,
    );
    const age = container.querySelector("time")!;
    const until = container.querySelector("span")!;
    expect(age).toHaveTextContent("now");
    expect(until).toHaveTextContent("in 5m");
    for (let minute = 1; minute <= 3; minute++) {
      act(() => {
        vi.advanceTimersByTime(60_000);
      });
      expect(age).toHaveTextContent(`${minute}m`);
      expect(until).toHaveTextContent(`in ${5 - minute}m`);
      expect(age).toHaveAttribute("dateTime", INSTANT);
      expect(age).toHaveAttribute("title", "exact source instant");
      expect(until).toHaveAttribute("title", "exact next instant");
      expect(container.querySelectorAll("time")).toHaveLength(1);
      expect(screen.getByText("Never")).toBeInTheDocument();
    }
  });

  it("shares a bounded timer across many labels and the deploy elapsed-time hook", () => {
    function DeployClock() {
      const now = useNow();
      return <output data-testid="deploy-clock">{now}</output>;
    }
    const { unmount } = render(
      <>
        <DeployClock />
        {Array.from({ length: 40 }, (_, i) => (
          <RelativeAge key={i} value={INSTANT} />
        ))}
      </>,
    );
    expect(vi.getTimerCount()).toBe(1);
    act(() => {
      vi.advanceTimersByTime(120_000);
    });
    expect(screen.getAllByText("2m")).toHaveLength(40);
    expect(screen.getByTestId("deploy-clock").textContent).toBe(
      String(Date.parse(INSTANT) + 120_000),
    );
    unmount();
    expect(vi.getTimerCount()).toBe(0);
  });
});
