import { afterEach, describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import i18n from "@/i18n/init";
import type { KeyValueView } from "@/features/keyvalue/types";

// The card reaches Apollo only through the instance-type catalog; the name row
// owns its own rename hook and its own test, so it is stubbed at the module
// boundary (the pattern the route test uses).
vi.mock("@/features/keyvalue/hooks/use-key-value-instance-types", () => ({
  useKeyValueInstanceTypes: () => ({
    instanceTypes: [
      {
        id: "starter",
        name: "Starter",
        cpu: "0.5",
        memory: "512 MB",
        storageGB: 1,
      },
    ],
    loading: false,
    error: undefined,
  }),
}));
vi.mock("@/features/keyvalue/components/key-value-name-row", () => ({
  KeyValueNameRow: ({ keyValue }: { keyValue: { name: string } }) => (
    <div>{keyValue.name}</div>
  ),
}));

import { KeyValueMetadataCard } from "@/features/keyvalue/components/key-value-metadata-card";

function kv(overrides: Partial<KeyValueView> = {}): KeyValueView {
  return {
    id: "red-sessions",
    name: "sessions-cache",
    status: "available",
    plan: "starter",
    version: "8",
    createdAt: null,
    externalHost: null,
    public: false,
    suspended: false,
    region: null,
    ...overrides,
  };
}

function renderCard(keyValue: KeyValueView) {
  render(<KeyValueMetadataCard keyValue={keyValue} onRenamed={vi.fn()} />);
}

describe("KeyValueMetadataCard", () => {
  // w5/m129: an unavailable store says why, translated by the reason's code;
  // bex-api's sentence is the fallback for a code this dashboard does not know.
  describe("unavailable reason", () => {
    afterEach(async () => {
      await i18n.changeLanguage("en");
    });

    // The list query refreshes a cached store's status without its reason, so
    // a store that recovered can still hold the old one.
    it("shows no reason beside a healthy store's status, even a stale one", () => {
      renderCard(
        kv({
          status: "available",
          statusReason: "The daily backup schedule could not be updated.",
          statusReasonCode: "BackupCronJobFailed",
        }),
      );
      expect(screen.getByText("Available")).toBeInTheDocument();
      expect(screen.queryByText(/backup schedule/)).not.toBeInTheDocument();
    });

    it("translates a known code", async () => {
      await i18n.changeLanguage("zh");
      renderCard(
        kv({
          status: "unavailable",
          statusReason: "The daily backup schedule could not be updated.",
          statusReasonCode: "BackupCronJobFailed",
        }),
      );
      expect(screen.getByText("无法更新每日备份计划。")).toBeInTheDocument();
      expect(screen.queryByText(/daily backup/)).not.toBeInTheDocument();
    });

    it("falls back to bex-api's sentence for an unknown code", async () => {
      await i18n.changeLanguage("zh");
      renderCard(
        kv({
          status: "unavailable",
          statusReason: "The Key Value failed to reconcile (SomethingNew).",
          statusReasonCode: "SomethingNew",
        }),
      );
      expect(
        screen.getByText("The Key Value failed to reconcile (SomethingNew)."),
      ).toBeInTheDocument();
    });
  });

  it("reads the suspended status, not a stale ready label (w1/m159)", () => {
    // deriveStatus prefers the suspended flag; with w5/061 the wire status is
    // also "suspended", so either input must keep this row aligned with the badge.
    renderCard(kv({ suspended: true }));

    expect(screen.getByText("Suspended")).toBeInTheDocument();
    expect(screen.queryByText("available")).not.toBeInTheDocument();
  });

  it("reads the plan's display name, not its id (w1/m159)", () => {
    renderCard(kv());

    expect(screen.getByText("Starter")).toBeInTheDocument();
    expect(screen.queryByText("starter")).not.toBeInTheDocument();
  });

  it("keeps a running store's status readable", () => {
    renderCard(kv());

    expect(screen.getByText("Available")).toBeInTheDocument();
  });
});
