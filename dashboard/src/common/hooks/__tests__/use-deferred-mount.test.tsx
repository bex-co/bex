import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { act, render, screen } from "@testing-library/react";
import { DeferredMount } from "../use-deferred-mount";

describe("DeferredMount", () => {
  let observe: ReturnType<typeof vi.fn>;
  let disconnect: ReturnType<typeof vi.fn>;
  let trigger: (intersecting: boolean) => void;

  beforeEach(() => {
    observe = vi.fn();
    disconnect = vi.fn();
    trigger = () => undefined;
    vi.stubGlobal(
      "IntersectionObserver",
      class {
        constructor(
          callback: IntersectionObserverCallback,
          _options?: IntersectionObserverInit,
        ) {
          trigger = (intersecting: boolean) => {
            callback(
              [
                {
                  isIntersecting: intersecting,
                  target: document.createElement("div"),
                } as unknown as IntersectionObserverEntry,
              ],
              this as unknown as IntersectionObserver,
            );
          };
        }
        observe = observe;
        disconnect = disconnect;
        unobserve = vi.fn();
        takeRecords = () => [];
        root = null;
        rootMargin = "";
        thresholds = [];
      },
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("keeps children unmounted until the sentinel intersects", async () => {
    render(
      <DeferredMount>
        <span>heavy panel</span>
      </DeferredMount>,
    );
    expect(screen.queryByText("heavy panel")).not.toBeInTheDocument();
    expect(observe).toHaveBeenCalled();
    await act(() => {
      trigger(true);
    });
    expect(await screen.findByText("heavy panel")).toBeInTheDocument();
    expect(disconnect).toHaveBeenCalled();
  });

  it("mounts eagerly when eager is set", () => {
    render(
      <DeferredMount eager>
        <span>heavy panel</span>
      </DeferredMount>,
    );
    expect(screen.getByText("heavy panel")).toBeInTheDocument();
    expect(observe).not.toHaveBeenCalled();
  });

  it("mounts when the location hash matches hashId", async () => {
    window.location.hash = "#insights";
    render(
      <DeferredMount hashId="insights">
        <span>insights</span>
      </DeferredMount>,
    );
    expect(await screen.findByText("insights")).toBeInTheDocument();
    window.location.hash = "";
  });
});
