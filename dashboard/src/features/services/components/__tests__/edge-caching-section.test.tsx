import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EdgeCachingSection } from "@/features/services/components/edge-caching-section";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { CREATE_REASON, mockCapabilities } from "@/test/mocks/capabilities";

// Stub the mutation hook so tests don't need an Apollo context.
const mockSetCacheProfile = vi.fn().mockResolvedValue(true);
const mockPurgeCache = vi.fn().mockResolvedValue(true);
vi.mock("@/features/services/hooks/use-edge-cache", () => ({
  useEdgeCache: () => ({
    setCacheProfile: mockSetCacheProfile,
    purgeCache: mockPurgeCache,
    saving: false,
    purging: false,
  }),
}));

function radio(name: RegExp) {
  return screen.getByRole("radio", { name });
}

describe("EdgeCachingSection", () => {
  beforeEach(() => {
    mockSetCacheProfile.mockClear();
    mockPurgeCache.mockClear();
  });

  it("renders the three Render options with the saved profile selected", () => {
    render(
      <EdgeCachingSection
        serviceId="web"
        plan="starter"
        cacheProfile="origin-controlled-all"
      />,
    );
    expect(screen.getByText("Edge Caching")).toBeInTheDocument();
    expect(screen.getByText("Cacheable file types")).toBeInTheDocument();
    expect(screen.getAllByRole("radio")).toHaveLength(3);
    expect(radio(/^None/)).toHaveAttribute("aria-checked", "false");
    expect(radio(/Common static files/)).toHaveAttribute(
      "aria-checked",
      "false",
    );
    expect(radio(/All files/)).toHaveAttribute("aria-checked", "true");
    // The All-files option warns that dynamic content should opt out.
    expect(radio(/All files/)).toHaveTextContent(/dynamic content/);
    expect(
      screen.getByText("The cache is purged on every successful deploy."),
    ).toBeInTheDocument();
  });

  it("reads a missing profile as None", () => {
    render(
      <EdgeCachingSection serviceId="web" plan="starter" cacheProfile={null} />,
    );
    expect(radio(/^None/)).toHaveAttribute("aria-checked", "true");
  });

  it("saves the chosen profile through setCacheProfile", async () => {
    const user = userEvent.setup();
    render(
      <EdgeCachingSection
        serviceId="web"
        plan="starter"
        cacheProfile="no-cache"
      />,
    );
    const save = screen.getByRole("button", { name: "Save edge caching" });
    // Nothing to save until the selection differs from the saved profile.
    expect(save).toBeDisabled();

    await user.click(radio(/Common static files/));
    expect(radio(/Common static files/)).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(save).toBeEnabled();
    await user.click(save);

    expect(mockSetCacheProfile).toHaveBeenCalledWith(
      "web",
      "origin-controlled",
    );
  });

  it("disables Purge cache while caching is off", () => {
    render(
      <EdgeCachingSection
        serviceId="web"
        plan="starter"
        cacheProfile="no-cache"
      />,
    );
    expect(screen.getByRole("button", { name: "Purge cache" })).toBeDisabled();
  });

  it("keeps Purge cache tied to the saved profile, not the unsaved selection", async () => {
    const user = userEvent.setup();
    render(
      <EdgeCachingSection
        serviceId="web"
        plan="starter"
        cacheProfile="no-cache"
      />,
    );
    await user.click(radio(/All files/));
    expect(screen.getByRole("button", { name: "Purge cache" })).toBeDisabled();
  });

  it("confirms, then purges through purgeServiceCache when caching is on", async () => {
    const user = userEvent.setup();
    render(
      <EdgeCachingSection
        serviceId="web"
        plan="starter"
        cacheProfile="origin-controlled"
      />,
    );
    const purge = screen.getByRole("button", { name: "Purge cache" });
    expect(purge).toBeEnabled();

    await user.click(purge);
    // The confirmation appears first; nothing is purged yet.
    expect(mockPurgeCache).not.toHaveBeenCalled();
    expect(
      screen.getByRole("heading", { name: "Purge the edge cache?" }),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Purge cache" }));
    expect(mockPurgeCache).toHaveBeenCalledWith("web");
  });

  it("shows the section disabled on the free plan", () => {
    render(
      <EdgeCachingSection
        serviceId="web"
        plan="free"
        cacheProfile="no-cache"
      />,
    );
    expect(
      screen.getByText("Edge caching is not available for free web services."),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("The cache is purged on every successful deploy."),
    ).toBeNull();
    for (const option of screen.getAllByRole("radio")) {
      expect(option).toBeDisabled();
    }
    expect(
      screen.getByRole("button", { name: "Save edge caching" }),
    ).toBeDisabled();
    expect(screen.getByRole("button", { name: "Purge cache" })).toBeDisabled();
  });
});

// The cache profile is a service setting (can_create) while a purge is an
// operation a Contributor keeps (can_operate), as Render's role table has it
// (w9/m619).
describe("EdgeCachingSection role gating (w9/m619)", () => {
  beforeEach(() => {
    mockSetCacheProfile.mockClear();
    mockPurgeCache.mockClear();
  });

  afterEach(() => {
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  });

  it("holds the profile for a contributor but keeps the purge", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    const user = userEvent.setup();
    render(
      <EdgeCachingSection
        serviceId="web"
        plan="starter"
        cacheProfile="origin-controlled"
      />,
    );
    for (const option of screen.getAllByRole("radio")) {
      expect(option).toBeDisabled();
    }
    expect(
      screen.getByRole("button", { name: "Save edge caching" }),
    ).toBeDisabled();
    expect(screen.getByText(CREATE_REASON)).toBeInTheDocument();
    await user.click(radio(/All files/));
    expect(mockSetCacheProfile).not.toHaveBeenCalled();

    const purge = screen.getByRole("button", { name: "Purge cache" });
    expect(purge).toBeEnabled();
    await user.click(purge);
    await user.click(screen.getByRole("button", { name: "Purge cache" }));
    expect(mockPurgeCache).toHaveBeenCalledWith("web");
  });

  it("leaves the profile and the purge enabled for a developer", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManage: false }),
    );
    render(
      <EdgeCachingSection
        serviceId="web"
        plan="starter"
        cacheProfile="origin-controlled"
      />,
    );
    for (const option of screen.getAllByRole("radio")) {
      expect(option).toBeEnabled();
    }
    expect(screen.getByRole("button", { name: "Purge cache" })).toBeEnabled();
    expect(screen.queryByText(CREATE_REASON)).toBeNull();
  });

  it("holds the purge for a viewer without can_operate", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "VIEWER", canOperate: false, canCreate: false }),
    );
    render(
      <EdgeCachingSection
        serviceId="web"
        plan="starter"
        cacheProfile="origin-controlled"
      />,
    );
    const purge = screen.getByRole("button", { name: "Purge cache" });
    expect(purge).toBeDisabled();
    expect(purge.closest('[data-slot="tooltip-trigger"]')).not.toBeNull();
  });
});
