import { describe, expect, it, vi } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { createElement } from "react";
import { render, screen } from "@testing-library/react";
import { DatabaseRowActionsWithCapabilities } from "@/features/databases/components/database-row-actions";
import { mockCapabilities } from "@/test/mocks/capabilities";

// The datastore lifecycle gate reads its projection through Apollo (w4/m578).
vi.mock("@/features/capabilities/hooks/use-lifecycle-gate", async () => {
  const { mockAllowedLifecycleGate } =
    await import("@/test/mocks/lifecycle-gate");
  return mockAllowedLifecycleGate();
});

vi.mock("@/features/databases/hooks/use-delete-database", () => ({
  useDeleteDatabase: () => ({ remove: vi.fn(), deleting: null }),
}));

// w4/244: at 390px a valid 30-character Key Value or Postgres name pushed the
// status badge past the viewport (document 390/416). The name/status row kept
// its content width, so the h1's `truncate` never engaged. Each detail
// header's title row — ready and pending alike, so the skeleton keeps the same
// structure — is now bounded to the header; geometry is verified in a browser.
const ROUTES = ["keyvalue.$keyValueId.tsx", "databases.$databaseId.tsx"];

function titleRows(source: string): string[] {
  const ready = source.match(
    /<div className="([^"]*)">\s*<h1 className="truncate text-xl font-semibold">/,
  )?.[1];
  const pending = source.match(
    /data-skeleton-region="resource-title"\s*className="([^"]*)"/,
  )?.[1];
  return [ready ?? "", pending ?? ""];
}

describe("datastore detail header title rows", () => {
  it.each(ROUTES)(
    "%s bounds its ready and pending name/status rows",
    (route) => {
      const source = readFileSync(
        join(import.meta.dirname, "..", route),
        "utf8",
      );
      const [ready, pending] = titleRows(source);
      for (const classes of [ready, pending]) {
        expect(classes.split(" ")).toEqual(
          expect.arrayContaining(["flex", "min-w-0", "max-w-full"]),
        );
      }
      expect(pending).toBe(ready);
    },
  );

  // w4/m209: the pending name bar stood 24px for a 28px text-xl h1, so the
  // tabs and the whole tab panel moved 4px down when the resource arrived.
  it.each(ROUTES)("%s reserves the h1's 28px line box", (route) => {
    const source = readFileSync(join(import.meta.dirname, "..", route), "utf8");
    const bar = source.match(
      /data-skeleton-region="resource-title"[\s\S]*?<Skeleton className="([^"]*)"/,
    )?.[1];
    expect(bar?.split(" ")).toContain("h-7");
  });

  // w3/082: the Postgres header's pending actions square was `size-9` (36px)
  // under a 32px `icon-sm` trigger, so the header stood 4px taller while the
  // database loaded and the tabs and the Logs toolbar moved up when it landed.
  // The skeleton is read against the trigger the ready header renders, so a
  // resized trigger fails here until its skeleton follows.
  it("draws the Postgres actions slot at the ready trigger's size", () => {
    render(
      createElement(DatabaseRowActionsWithCapabilities, {
        database: {
          id: "dpg-main",
          name: "primary-db",
          status: "available",
          plan: "free",
          version: "18",
          diskSizeGB: 1,
          createdAt: null,
          public: false,
          suspended: "not_suspended",
        },
        onDeleted: vi.fn(),
        lifecycle: { pending: null, run: vi.fn() },
        capabilities: mockCapabilities(),
      }),
    );
    const trigger = screen.getByRole("button", {
      name: "Actions for primary-db",
    });
    const source = readFileSync(
      join(import.meta.dirname, "..", "databases.$databaseId.tsx"),
      "utf8",
    );
    const slot = source.match(
      /data-skeleton-region="resource-actions">[\s\S]*?<Skeleton className="([^"]*)"/,
    )?.[1];
    const sizeOf = (classes: string | undefined) =>
      classes?.split(" ").filter((token) => /^(size|h|w)-\d+$/.test(token));

    expect(sizeOf(trigger.className)).toEqual(["size-8"]);
    expect(sizeOf(slot)).toEqual(sizeOf(trigger.className));
  });
});
