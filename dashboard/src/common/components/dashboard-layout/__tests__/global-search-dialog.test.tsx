import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { GlobalSearchDialog } from "../global-search-dialog";

// The dialog resolves resources through five feature hooks + the router; mock
// them so the test drives only the service-result rendering. Only services are
// populated (with distinct types); the other kinds stay empty.
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => vi.fn(),
}));
vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => ({
    services: [
      { id: "srv-web", name: "web-app", type: "web_service" },
      { id: "srv-cron", name: "nightly", type: "cron_job" },
      { id: "srv-priv", name: "internal-api", type: "private_service" },
    ],
    loading: false,
  }),
}));
vi.mock("@/features/databases/hooks/use-databases", () => ({
  useDatabases: () => ({ databases: [], loading: false }),
}));
vi.mock("@/features/keyvalue/hooks/use-key-values", () => ({
  useKeyValues: () => ({ keyValues: [], loading: false }),
}));
vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({ projects: [], loading: false }),
}));
vi.mock("@/features/env-groups/hooks/use-env-groups", () => ({
  useEnvGroups: () => ({ groups: [], loading: false }),
}));

describe("GlobalSearchDialog", () => {
  // w4/049: the palette must name each service by its specific type + icon, not
  // collapse every service to a generic "Service" + Globe (sibling of w4/047).
  it("labels each service result by its specific type, not a generic Service", () => {
    render(<GlobalSearchDialog open onOpenChange={vi.fn()} />);

    expect(screen.getByText("Web Service")).toBeInTheDocument();
    expect(screen.getByText("Cron Job")).toBeInTheDocument();
    expect(screen.getByText("Private Service")).toBeInTheDocument();
    // The generic collapse label must be gone.
    expect(screen.queryByText("Service")).not.toBeInTheDocument();

    // Per-type icons replace the old hardcoded Globe2: the cron row gets a
    // clock, and no result carries the old generic glyph. cmdk portals the
    // dialog to document.body, so query the whole document, not the root.
    expect(document.querySelector(".lucide-clock")).not.toBeNull();
    expect(document.querySelector(".lucide-globe-2")).toBeNull();
  });
});
