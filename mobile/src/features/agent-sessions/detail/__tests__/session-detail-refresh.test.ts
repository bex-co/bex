import {
  sessionDetailPollIntervalMs,
  SESSION_DETAIL_POLL_MS,
} from "../session-detail-refresh";
import type { RecoveryEnvironment } from "../../../../common/hooks/recovery-coordinator";

const onlineActive: RecoveryEnvironment = {
  connectivity: "online",
  appState: "active",
};

describe("sessionDetailPollIntervalMs", () => {
  it("polls active sessions when foreground and online", () => {
    expect(sessionDetailPollIntervalMs("running", onlineActive)).toBe(
      SESSION_DETAIL_POLL_MS,
    );
    expect(sessionDetailPollIntervalMs("creating", onlineActive)).toBe(
      SESSION_DETAIL_POLL_MS,
    );
    // Unknown phase (direct deep link before first response) stays live.
    expect(sessionDetailPollIntervalMs(undefined, onlineActive)).toBe(
      SESSION_DETAIL_POLL_MS,
    );
  });

  it("stops polling terminal sessions", () => {
    expect(sessionDetailPollIntervalMs("completed", onlineActive)).toBe(0);
    expect(sessionDetailPollIntervalMs("failed", onlineActive)).toBe(0);
    expect(sessionDetailPollIntervalMs("canceled", onlineActive)).toBe(0);
    expect(sessionDetailPollIntervalMs("hibernated", onlineActive)).toBe(0);
  });

  it("stops background and offline work", () => {
    expect(
      sessionDetailPollIntervalMs("running", {
        connectivity: "offline",
        appState: "active",
      }),
    ).toBe(0);
    expect(
      sessionDetailPollIntervalMs("running", {
        connectivity: "online",
        appState: "background",
      }),
    ).toBe(0);
  });
});
