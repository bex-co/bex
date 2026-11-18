import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useOrganizations } from "@/features/organizations/hooks/use-organizations";
import { DashboardLayout } from "..";

// The sidebar (and the workspace switcher in it) stands for the closed
// mobile Sheet: it mounts nothing.
vi.mock("../dashboard-sidebar", () => ({ DashboardSidebar: () => null }));
vi.mock("../dashboard-header", () => ({ DashboardHeader: () => null }));

beforeEach(() => vi.mocked(useOrganizations).mockClear());

// w4/706: the org pages' pending frames and loader read the caller's role from
// the `organizations` list, so the shell fetches it at every width, not only
// through the switcher a phone's closed sidebar never mounts.
describe("DashboardLayout's organizations fetch", () => {
  it("reads the caller's organizations with the sidebar unmounted", () => {
    render(
      <DashboardLayout>
        <h1>Page</h1>
      </DashboardLayout>,
    );
    expect(screen.getByRole("heading", { name: "Page" })).toBeInTheDocument();
    expect(useOrganizations).toHaveBeenCalled();
  });
});
