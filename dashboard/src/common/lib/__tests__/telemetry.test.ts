import { afterEach, describe, expect, it, vi } from "vitest";

import { trackEvent } from "../telemetry";

describe("trackEvent", () => {
  afterEach(() => {
    delete window.gtag;
  });

  it("forwards to gtag when GA is loaded", () => {
    const gtag = vi.fn();
    window.gtag = gtag;
    trackEvent("ssh_gate_shown", { surface: "shell" });
    expect(gtag).toHaveBeenCalledWith("event", "ssh_gate_shown", {
      surface: "shell",
    });
  });

  it("is a no-op without GA", () => {
    expect(() => trackEvent("ssh_gate_cta_clicked")).not.toThrow();
  });
});
