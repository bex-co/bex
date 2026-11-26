import { describe, expect, it } from "vitest";
import {
  erroredShare,
  imageFailureKey,
  imageSourceStatus,
  rangeStart,
  sandboxDurationMs,
  shortDigest,
  snapshotFailureKey,
  snapshotNeverExpires,
  statusFromSearch,
} from "@/features/sandbox-group-pages/lib/sandbox-view";
import {
  isSandboxGroupNavItemActive,
  sandboxGroupPageLabelKey,
} from "@/features/sandbox-group-pages/lib/nav";

const label = (key: string) =>
  ({ "sandboxGroupPages.statusRunning": "In Betrieb" })[key] ?? key;

describe("a sandbox group's page reads (w1/m658)", () => {
  it("turns a status word, bex's or its label, into a status filter", () => {
    expect(statusFromSearch(" Running ", label)).toBe("running");
    expect(statusFromSearch("in betrieb", label)).toBe("running");
    expect(statusFromSearch("alpha", label)).toBeNull();
    expect(statusFromSearch("", label)).toBeNull();
  });

  it("starts a range at the minute, and All time nowhere", () => {
    const now = Date.parse("2026-11-26T12:34:56.789Z");
    expect(rangeStart("1h", now)).toBe("2026-11-26T11:34:00.000Z");
    expect(rangeStart("7d", now)).toBe("2026-11-19T12:34:00.000Z");
    expect(rangeStart("all", now)).toBeUndefined();
  });

  it("words the Errored share as Render does", () => {
    expect(erroredShare(0, 0, "en")).toBeNull();
    expect(erroredShare(4, 1, "en")).toBe("25%");
    expect(erroredShare(5000, 1, "en")).toBe("<0.1%");
    expect(erroredShare(3, 0, "en")).toBe("0%");
  });

  it("measures a run from its start to its end, or to now", () => {
    const now = Date.parse("2026-11-26T10:10:00Z");
    expect(
      sandboxDurationMs(
        {
          createdAt: "2026-11-26T10:00:00Z",
          startedAt: "2026-11-26T10:00:05Z",
          endedAt: "2026-11-26T10:05:05Z",
        },
        now,
      ),
    ).toBe(300_000);
    expect(
      sandboxDurationMs(
        { createdAt: "2026-11-26T10:00:00Z", startedAt: null, endedAt: null },
        now,
      ),
    ).toBe(600_000);
    expect(
      sandboxDurationMs(
        { createdAt: null, startedAt: null, endedAt: null },
        now,
      ),
    ).toBeNull();
  });

  it("maps snapshot failure codes onto Render's texts, and ttl 0 onto Never", () => {
    expect(snapshotFailureKey("DEADLINE_EXCEEDED")).toBe(
      "sandboxGroupPages.snapshotFailureTimeout",
    );
    expect(snapshotFailureKey("INTERNAL_ERROR")).toBe(
      "sandboxGroupPages.snapshotFailureInternal",
    );
    expect(snapshotFailureKey(null)).toBe(
      "sandboxGroupPages.snapshotFailureInternal",
    );
    expect(
      snapshotNeverExpires({
        ttlSeconds: 0,
        expiresAt: "9999-12-31T23:59:59Z",
      }),
    ).toBe(true);
    expect(
      snapshotNeverExpires({
        ttlSeconds: 86400,
        expiresAt: "2026-12-01T00:00:00Z",
      }),
    ).toBe(false);
  });

  it("reads an image name's status from its newest and newest available builds", () => {
    expect(
      imageSourceStatus({
        latestBuild: { status: "running" },
        latestSucceededBuild: {},
      }),
    ).toBe("sandboxGroupPages.imageImporting");
    expect(
      imageSourceStatus({
        latestBuild: { status: "failed" },
        latestSucceededBuild: {},
      }),
    ).toBe("sandboxGroupPages.imageAvailable");
    expect(
      imageSourceStatus({
        latestBuild: { status: "failed" },
        latestSucceededBuild: null,
      }),
    ).toBe("sandboxGroupPages.imageUnavailable");
  });

  it("words an import failure as Render does, and keeps a reason it has no words for", () => {
    expect(
      imageFailureKey(
        "fetch source image: UNAUTHORIZED: authentication required",
      )?.key,
    ).toBe("sandboxGroupPages.imageFailureCredentials");
    expect(
      imageFailureKey("fetch source image: MANIFEST_UNKNOWN: manifest unknown")
        ?.key,
    ).toBe("sandboxGroupPages.imageFailureNotFound");
    expect(
      imageFailureKey(
        "source image is linux/arm64; only linux/amd64 is supported",
      ),
    ).toEqual({
      key: "sandboxGroupPages.imageFailurePlatform",
      params: { platform: "linux/arm64" },
    });
    expect(imageFailureKey("DeadlineExceeded: job ran too long")?.key).toBe(
      "sandboxGroupPages.imageFailureTimeout",
    );
    expect(imageFailureKey("something new")).toBeNull();
    expect(shortDigest(`sha256:${"ab".repeat(32)}`)).toBe("abababa");
  });

  it("opens Sandboxes on the group's bare URL and names each page", () => {
    const id = "sbg-dc44tha9086ovmjtv6c0";
    expect(sandboxGroupPageLabelKey(`/sandbox-groups/${id}`, id)).toBe(
      "sandboxGroupPages.navSandboxes",
    );
    expect(sandboxGroupPageLabelKey(`/sandbox-groups/${id}/images`, id)).toBe(
      "sandboxGroupPages.navImages",
    );
    expect(sandboxGroupPageLabelKey("/sandbox-groups", id)).toBeNull();
    expect(
      isSandboxGroupNavItemActive(
        `/sandbox-groups/${id}/snapshots`,
        id,
        "/sandbox-groups/$sandboxGroupId/snapshots",
      ),
    ).toBe(true);
    expect(
      isSandboxGroupNavItemActive(
        `/sandbox-groups/${id}/snapshots`,
        id,
        "/sandbox-groups/$sandboxGroupId",
      ),
    ).toBe(false);
  });
});
