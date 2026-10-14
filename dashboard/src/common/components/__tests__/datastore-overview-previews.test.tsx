import type { ReactElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { render, within } from "@testing-library/react";
import {
  DatabaseOverviewSkeleton,
  KeyValueOverviewSkeleton,
} from "../route-skeletons";
import { DatabaseMetadataCard } from "@/features/databases/components/database-metadata-card";
import { ConnectionInfoPanel as DatabaseConnectionInfoPanel } from "@/features/databases/components/connection-info-panel";
import { KeyValueMetadataCard } from "@/features/keyvalue/components/key-value-metadata-card";
import { ConnectionInfoPanel as KeyValueConnectionInfoPanel } from "@/features/keyvalue/components/connection-info-panel";
import { KeyValueMaxmemoryPolicySection } from "@/features/keyvalue/components/key-value-maxmemory-policy-section";
import { KeyValuePersistenceModeSection } from "@/features/keyvalue/components/key-value-persistence-mode-section";
import type { DatabaseDetailView } from "@/features/databases/types";
import type { KeyValueView } from "@/features/keyvalue/types";

// The ready cards render for real (EditableFieldRow, MetadataList, the masked
// Connections panel); only their data and mutation hooks are stubbed.
vi.mock("@/features/databases/hooks/use-database-instance-types", () => ({
  useDatabaseInstanceTypes: () => ({ instanceTypes: [], loading: false }),
}));
vi.mock("@/features/keyvalue/hooks/use-key-value-instance-types", () => ({
  useKeyValueInstanceTypes: () => ({ instanceTypes: [], loading: false }),
}));
vi.mock("@/features/databases/hooks/use-rename-database", () => ({
  useRenameDatabase: () => ({ rename: vi.fn(), busy: false }),
}));
vi.mock("@/features/keyvalue/hooks/use-rename-key-value", () => ({
  useRenameKeyValue: () => ({ rename: vi.fn(), busy: false }),
}));
// The exercised PostgreSQL 18 version has no upgrade target: plain text.
vi.mock("@/features/databases/components/database-version-control", () => ({
  DatabaseVersionControl: () => <span>PostgreSQL 18</span>,
}));
const policyState = { policy: "allkeys-lru", loading: false };
vi.mock("@/features/keyvalue/hooks/use-set-key-value-maxmemory-policy", () => ({
  useSetKeyValueMaxmemoryPolicy: () => ({
    ...policyState,
    saving: false,
    save: vi.fn(),
  }),
}));
const modeState = { mode: "journal-snapshot", loading: false };
vi.mock("@/features/keyvalue/hooks/use-set-key-value-persistence-mode", () => ({
  useSetKeyValuePersistenceMode: () => ({
    ...modeState,
    saving: false,
    save: vi.fn(),
  }),
}));
const masked = { info: null, loading: false, error: undefined };
vi.mock("@/features/databases/hooks/use-connection-info", () => ({
  useConnectionInfo: () => ({ ...masked, reveal: vi.fn(), hide: vi.fn() }),
}));
vi.mock("@/features/keyvalue/hooks/use-connection-info", () => ({
  useConnectionInfo: () => ({ ...masked, reveal: vi.fn(), hide: vi.fn() }),
}));

const database: DatabaseDetailView = {
  id: "dpg-layout",
  name: "qa-layout",
  status: "available",
  plan: "free",
  version: "18",
  diskSizeGB: 1,
  createdAt: null,
  public: false,
  suspended: "not_suspended",
  databaseName: "qa_layout",
  databaseUser: "qa_layout_user",
  highAvailabilityEnabled: false,
  diskAutoscalingEnabled: false,
  poolerEnabled: false,
  readReplicas: [],
  externalHost: null,
  region: "oregon",
  backupsEnabled: false,
};

const keyValue: KeyValueView = {
  id: "red-layout",
  name: "qa-layout",
  status: "available",
  plan: "free",
  version: "8",
  createdAt: null,
  externalHost: null,
  public: false,
  suspended: false,
  region: "oregon",
};

/**
 * The card's structural signature: every element's tag and classes plus its
 * own static text, in document order. A `dd` collapses to its tag (its value
 * is resource data), and an input drops its value, so a preview matches the
 * ready card exactly when it has the same anatomy, copy and spacing.
 */
function shape(element: Element): string {
  const tag = element.tagName.toLowerCase();
  if (tag === "dd") return "dd";
  if (tag === "svg") return `svg.${element.getAttribute("class")}`;
  const own = [...element.childNodes]
    .filter((node) => node.nodeType === Node.TEXT_NODE)
    .map((node) => node.textContent?.trim())
    .filter(Boolean)
    .join(" ");
  const children = [...element.children].map(shape).join(",");
  return `${tag}.${element.getAttribute("class") ?? ""}${own ? `"${own}"` : ""}[${children}]`;
}

function card(ui: ReactElement, region?: string): Element {
  const { container } = render(ui);
  const root = region
    ? container.querySelector(`[data-skeleton-region="${region}"]`)
    : container;
  const found = root?.querySelector('[data-slot="card"]');
  expect(found).not.toBeNull();
  return found as Element;
}

describe("datastore Details and Connections previews (w4/298)", () => {
  for (const [name, Skeleton, readyDetails, readyConnections] of [
    [
      "Postgres",
      DatabaseOverviewSkeleton,
      <DatabaseMetadataCard
        key="details"
        database={database}
        onVersionChanged={vi.fn()}
        onRenamed={vi.fn()}
      />,
      <DatabaseConnectionInfoPanel key="connections" id={database.id} />,
    ],
    [
      "Key Value",
      KeyValueOverviewSkeleton,
      <KeyValueMetadataCard
        key="details"
        keyValue={keyValue}
        onRenamed={vi.fn()}
      />,
      <KeyValueConnectionInfoPanel key="connections" id={keyValue.id} />,
    ],
  ] as const) {
    it(`${name} Details preview has the ready name lead and facts grid`, () => {
      const pending = card(<Skeleton />, "metadata");
      const ready = card(readyDetails);
      expect(shape(pending)).toBe(shape(ready));

      // Inert, with no entity value: the name input is empty and disabled.
      expect(pending.hasAttribute("inert")).toBe(true);
      const input = within(pending as HTMLElement).getByRole("textbox", {
        hidden: true,
      }) as HTMLInputElement;
      expect(input.value).toBe("");
      expect(input.disabled).toBe(true);
      expect(pending.textContent).not.toContain("qa-layout");

      // Each value slot keeps the ready 20px text-sm line.
      for (const dd of pending.querySelectorAll("dd")) {
        expect(dd).toHaveClass("h-5", "min-w-0", "text-sm", "font-medium");
      }
    });

    it(`${name} Connections preview is the masked ready card`, () => {
      const pending = card(<Skeleton />, "connection");
      const ready = card(readyConnections);
      expect(shape(pending)).toBe(shape(ready));
      expect(pending.hasAttribute("inert")).toBe(true);
      const reveal = within(pending as HTMLElement).getByRole("button", {
        hidden: true,
      }) as HTMLButtonElement;
      expect(reveal.disabled).toBe(true);
    });
  }
});

function part(card: Element, slot: string): Element {
  const found = card.querySelector(`[data-slot="${slot}"]`);
  expect(found).not.toBeNull();
  return found as Element;
}

/** The ready select trigger's box: these classes set its bounds. */
const TRIGGER_BOX = [
  "flex",
  "min-h-8",
  "min-w-0",
  "flex-1",
  "items-center",
  "justify-between",
  "gap-2",
  "rounded-md",
  "border",
  "px-3",
  "py-2",
  "text-sm",
];

/**
 * A select-field preview against the ready EditableFieldRow: the same label
 * and hint block, control-row wrapping and pencil slot; a trigger box with the
 * ready sizing; `narrowLines` value lines below `sm` and one from `sm`.
 */
function expectSelectFieldPreview(
  preview: Element,
  ready: Element,
  narrowLines: number,
) {
  expect(preview.className).toBe(ready.className);
  const [previewLabel, previewControls] = [...preview.children];
  const [readyLabel, readyControls] = [...ready.children];
  expect(shape(previewLabel)).toBe(shape(readyLabel));
  expect(previewControls.className).toBe(readyControls.className);
  const [previewTrigger, previewPencil] = [...previewControls.children];
  const [readyTrigger, readyPencil] = [...readyControls.children];
  expect(readyTrigger).toHaveClass(...TRIGGER_BOX);
  expect(previewTrigger).toHaveClass(...TRIGGER_BOX);
  const lines = previewTrigger.querySelectorAll("span.h-5");
  expect(lines).toHaveLength(narrowLines);
  if (narrowLines === 2) expect(lines[1]).toHaveClass("sm:hidden");
  expect(shape(previewPencil)).toBe(shape(readyPencil));
  expect(previewControls.children).toHaveLength(2);
}

describe("Key Value policy previews (w4/299)", () => {
  afterEach(() => {
    policyState.policy = "allkeys-lru";
    policyState.loading = false;
    modeState.mode = "journal-snapshot";
    modeState.loading = false;
  });

  for (const [name, region, ready, unresolve, narrowLines] of [
    [
      "Maxmemory Policy",
      "maxmemory-policy",
      () => <KeyValueMaxmemoryPolicySection id={keyValue.id} />,
      () => Object.assign(policyState, { policy: "", loading: true }),
      2,
    ],
    [
      "Persistence Mode",
      "persistence-mode",
      () => <KeyValuePersistenceModeSection id={keyValue.id} />,
      () => Object.assign(modeState, { mode: "", loading: true }),
      1,
    ],
  ] as const) {
    it(`${name} route preview has the ready help and select field`, () => {
      const pending = card(<KeyValueOverviewSkeleton />, region);
      const readyCard = card(ready());
      expect(pending.hasAttribute("inert")).toBe(true);
      expect(shape(part(pending, "card-header"))).toBe(
        shape(part(readyCard, "card-header")),
      );
      expect(part(pending, "card-content").className).toBe(
        part(readyCard, "card-content").className,
      );
      expectSelectFieldPreview(
        part(pending, "card-content").firstElementChild!,
        part(readyCard, "card-content").firstElementChild!,
        narrowLines,
      );
      // No value and no live control before the read resolves.
      expect(
        within(pending as HTMLElement).queryByRole("combobox", {
          hidden: true,
        }),
      ).toBeNull();
      const field = part(pending, "card-content");
      expect(field.textContent).not.toMatch(/allkeys|Journal \+ Snapshot/);
    });

    it(`${name} section keeps the field anatomy while its own query loads`, () => {
      const pending = card(<KeyValueOverviewSkeleton />, region);
      unresolve();
      const loadingCard = card(ready());
      expect(shape(part(loadingCard, "card-content").firstElementChild!)).toBe(
        shape(part(pending, "card-content").firstElementChild!),
      );
      expect(
        within(loadingCard as HTMLElement).queryByRole("combobox"),
      ).toBeNull();
    });
  }
});
