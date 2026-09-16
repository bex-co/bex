import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
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
