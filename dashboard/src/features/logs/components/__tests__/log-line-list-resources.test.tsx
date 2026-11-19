import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { LogLineList } from "../log-line-list";
import {
  resourceLabels,
  RESOURCE_DOT_CLASSES,
} from "../../lib/resource-labels";
import type { LogLine } from "../../types";
import { setupVirtualGeometry } from "@/test/virtual-geometry";

// A project view's lines name their resource (w3/m505): Render's colored dot
// and `[name]`, linked to the resource's own page.
vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  Link: ({
    to,
    params,
    children,
    className,
  }: {
    to: string;
    params: Record<string, string>;
    children: ReactNode;
    className?: string;
  }) => (
    <a
      href={to.replace(/\$(\w+)/g, (_, key: string) => params[key])}
      className={className}
    >
      {children}
    </a>
  ),
}));

function line(over: Partial<LogLine>): LogLine {
  return {
    key: over.key ?? "k",
    timestamp: "2026-11-19T00:40:00Z",
    instance: "",
    resource: "",
    type: "app",
    level: "",
    method: "",
    statusCode: "",
    host: "",
    path: "",
    message: "hello",
    spans: null,
    ...over,
  };
}

const RESOURCES = [
  {
    id: "srv-web",
    name: "storefront",
    type: "web_service",
    environmentId: null,
  },
  { id: "srv-cron", name: "nightly", type: "cron_job", environmentId: null },
  { id: "dpg-db", name: "orders-db", type: "postgres", environmentId: null },
  { id: "red-kv", name: "cache", type: "key_value", environmentId: null },
];

describe("LogLineList resource labels (w3/m505)", () => {
  setupVirtualGeometry();

  it("names each line's resource with its own color and page", () => {
    render(
      <LogLineList
        resourceLabels={resourceLabels(RESOURCES)}
        lines={[
          line({ key: "1", resource: "srv-web", message: "web line" }),
          line({ key: "2", resource: "srv-cron", message: "cron line" }),
          line({ key: "3", resource: "dpg-db", message: "pg line" }),
          line({ key: "4", resource: "red-kv", message: "kv line" }),
        ]}
      />,
    );
    const links = screen.getAllByRole("link");
    expect(links.map((a) => [a.textContent, a.getAttribute("href")])).toEqual([
      ["[storefront]", "/services/srv-web"],
      ["[nightly]", "/services/srv-cron"],
      ["[orders-db]", "/databases/dpg-db"],
      ["[cache]", "/keyvalue/red-kv"],
    ]);
    const dots = links.map(
      (a) => a.querySelector("[aria-hidden]")?.className ?? "",
    );
    RESOURCE_DOT_CLASSES.slice(0, 4).forEach((color, i) =>
      expect(dots[i]).toContain(color),
    );
    expect(new Set(dots).size).toBe(4);
  });

  it("shows an unlisted resource's id, unlinked, until a read names it", () => {
    render(
      <LogLineList
        resourceLabels={resourceLabels(RESOURCES)}
        lines={[line({ resource: "srv-new", message: "joined later" })]}
      />,
    );
    expect(screen.getByText("[srv-new]")).toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });

  it("leaves a single resource's log as it was: no resource label", () => {
    render(
      <LogLineList
        lines={[line({ resource: "srv-web", message: "service line" })]}
      />,
    );
    expect(screen.getByText("service line")).toBeInTheDocument();
    expect(screen.queryByText("[storefront]")).not.toBeInTheDocument();
    expect(screen.queryByText("[srv-web]")).not.toBeInTheDocument();
  });

  it("keeps a resource's color by the server's order and cycles past the palette", () => {
    const many = Array.from(
      { length: RESOURCE_DOT_CLASSES.length + 1 },
      (_, i) => ({
        id: `srv-${i}`,
        name: "",
        type: "web_service",
        environmentId: null,
      }),
    );
    const labels = resourceLabels(many);
    expect(labels.get("srv-0")?.dotClass).toBe(RESOURCE_DOT_CLASSES[0]);
    expect(labels.get(`srv-${RESOURCE_DOT_CLASSES.length}`)?.dotClass).toBe(
      RESOURCE_DOT_CLASSES[0],
    );
    // A resource without a name is named by its id.
    expect(labels.get("srv-1")?.name).toBe("srv-1");
  });
});
