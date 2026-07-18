import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, waitFor } from "@testing-library/react";
import { useLoaderErrorRetry } from "../use-loader-error-retry";

const navigate = vi.fn(() => Promise.resolve());
vi.mock("@tanstack/react-router", () => ({
  useRouter: () => ({ navigate }),
}));

const SAME_LOCATION_REPLACE = {
  to: ".",
  search: true,
  hash: true,
  replace: true,
};

function Probe({ state, id }: { state: string; id: string }) {
  useLoaderErrorRetry({ state }, id);
  return null;
}

function BareProbe() {
  // A shell rendered outside its route (tests) has no loader result at all.
  useLoaderErrorRetry(undefined, "prj-1");
  return null;
}

beforeEach(() => {
  navigate.mockClear();
});

describe("useLoaderErrorRetry (w1/m52 t003)", () => {
  it("re-runs the loaders exactly once for a dehydrated error", async () => {
    const view = render(<Probe state="error" id="prj-1" />);
    await waitFor(() => expect(navigate).toHaveBeenCalledTimes(1));
    // A same-location REPLACE navigation (not invalidate): a navigation
    // commits the head too; search/hash: true keep the URL byte-identical.
    expect(navigate).toHaveBeenCalledWith(SAME_LOCATION_REPLACE);
    // Still errored after the retry (or re-rendered for any reason): no loop.
    view.rerender(<Probe state="error" id="prj-1" />);
    await new Promise((r) => setTimeout(r, 10));
    expect(navigate).toHaveBeenCalledTimes(1);
  });

  it("after the retry heals, re-navigates once more to recompute the head", async () => {
    const view = render(<Probe state="error" id="prj-1" />);
    await waitFor(() => expect(navigate).toHaveBeenCalledTimes(1));
    // The retry navigation began while errored, so its head captured the
    // error <title>; the error→ready transition triggers ONE more replace
    // navigation to recompute the head from the recovered loaderData.
    view.rerender(<Probe state="ready" id="prj-1" />);
    await waitFor(() => expect(navigate).toHaveBeenCalledTimes(2));
    expect(navigate).toHaveBeenLastCalledWith(SAME_LOCATION_REPLACE);
    // Settled: further re-renders in ready state do nothing.
    view.rerender(<Probe state="ready" id="prj-1" />);
    await new Promise((r) => setTimeout(r, 10));
    expect(navigate).toHaveBeenCalledTimes(2);
  });

  it("does nothing for ready or not-found loader states without a prior error", async () => {
    const view = render(<Probe state="ready" id="prj-1" />);
    view.rerender(<Probe state="not-found" id="prj-1" />);
    await new Promise((r) => setTimeout(r, 10));
    expect(navigate).not.toHaveBeenCalled();
  });

  it("is a no-op without a loader result (shell rendered outside its route)", async () => {
    render(<BareProbe />);
    await new Promise((r) => setTimeout(r, 10));
    expect(navigate).not.toHaveBeenCalled();
  });

  it("arms again when the resource key changes", async () => {
    const view = render(<Probe state="error" id="prj-1" />);
    await waitFor(() => expect(navigate).toHaveBeenCalledTimes(1));
    view.rerender(<Probe state="error" id="prj-2" />);
    await waitFor(() => expect(navigate).toHaveBeenCalledTimes(2));
  });

  it("retries an error that appears after a clean mount (mid-session blip)", async () => {
    const view = render(<Probe state="ready" id="prj-1" />);
    view.rerender(<Probe state="error" id="prj-1" />);
    await waitFor(() => expect(navigate).toHaveBeenCalledTimes(1));
  });
});
