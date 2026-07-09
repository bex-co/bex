import { describe, it, expect, vi, beforeEach, beforeAll } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { CustomDomainView } from "@/features/services/types";

// Drive the section by mocking the feature hooks — the same pattern as the
// env-vars panel test. The query hook is a vi.fn so each test picks the data;
// the mutation hook returns spies we assert on.
const mockUseCustomDomains = vi.fn();
const mockAddDomain = vi.fn();
const mockDeleteDomain = vi.fn();

vi.mock(
  "@/features/services/hooks/use-custom-domains",
  async (importOriginal) => {
    const actual =
      await importOriginal<
        typeof import("@/features/services/hooks/use-custom-domains")
      >();
    return {
      ...actual,
      useCustomDomains: (...a: unknown[]) => mockUseCustomDomains(...a),
      useCustomDomainMutations: () => ({
        addDomain: mockAddDomain,
        deleteDomain: mockDeleteDomain,
        busy: false,
      }),
    };
  },
);

import { CustomDomainsSection } from "@/features/services/components/custom-domains-section";

function domainsResult(
  domains: CustomDomainView[],
  over: Partial<{ loading: boolean; error: Error | undefined }> = {},
) {
  return {
    domains,
    loading: false,
    error: undefined,
    refetch: vi.fn().mockResolvedValue(domains),
    ...over,
  };
}

const verifiedDomain: CustomDomainView = {
  name: "www.example.com",
  verified: true,
  active: true,
};

// Radix's DropdownMenu relies on pointer-capture APIs jsdom doesn't implement.
beforeAll(() => {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
  if (!Element.prototype.releasePointerCapture) {
    Element.prototype.releasePointerCapture = () => {};
  }
});

beforeEach(() => {
  mockUseCustomDomains.mockReset();
  mockAddDomain.mockReset().mockResolvedValue(true);
  mockDeleteDomain.mockReset().mockResolvedValue(true);
});

describe("CustomDomainsSection", () => {
  it("renders the empty state when there are no domains", () => {
    mockUseCustomDomains.mockReturnValue(domainsResult([]));
    render(<CustomDomainsSection serviceId="web" />);
    expect(screen.getByText("No custom domains")).toBeInTheDocument();
  });

  it("lists domains with an external link and both status badges", () => {
    mockUseCustomDomains.mockReturnValue(domainsResult([verifiedDomain]));
    render(<CustomDomainsSection serviceId="web" />);

    const link = screen.getByRole("link", { name: /www\.example\.com/ });
    expect(link).toHaveAttribute("href", "https://www.example.com");
    // verified => "Verified"; active => "Active"
    expect(screen.getByText("Verified")).toBeInTheDocument();
    expect(screen.getByText("Active")).toBeInTheDocument();
  });

  it("shows Pending in both columns for an unverified domain", () => {
    mockUseCustomDomains.mockReturnValue(
      domainsResult([
        {
          ...verifiedDomain,
          name: "api.example.com",
          verified: false,
          active: false,
        },
      ]),
    );
    render(<CustomDomainsSection serviceId="web" />);
    expect(screen.getAllByText("Pending")).toHaveLength(2);
  });

  it("renders the error state when the query fails", () => {
    mockUseCustomDomains.mockReturnValue(
      domainsResult([], { error: new Error("boom") }),
    );
    render(<CustomDomainsSection serviceId="web" />);
    expect(
      screen.getByText("Couldn't load custom domains"),
    ).toBeInTheDocument();
  });

  it("opens the add dialog and submits a valid FQDN", async () => {
    mockUseCustomDomains.mockReturnValue(domainsResult([]));
    const user = userEvent.setup();
    render(<CustomDomainsSection serviceId="web" />);

    await user.click(screen.getByRole("button", { name: "Add Custom Domain" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Name"), "shop.example.com");
    await user.click(
      within(dialog).getByRole("button", { name: "Add Domain" }),
    );

    await waitFor(() =>
      expect(mockAddDomain).toHaveBeenCalledWith("shop.example.com"),
    );
  });

  it("rejects a malformed hostname without calling the mutation", async () => {
    mockUseCustomDomains.mockReturnValue(domainsResult([]));
    const user = userEvent.setup();
    render(<CustomDomainsSection serviceId="web" />);

    await user.click(screen.getByRole("button", { name: "Add Custom Domain" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Name"), "not a domain");
    await user.click(
      within(dialog).getByRole("button", { name: "Add Domain" }),
    );

    expect(screen.getByText(/Enter a valid domain/)).toBeInTheDocument();
    expect(mockAddDomain).not.toHaveBeenCalled();
  });

  it("deletes a domain after confirming from the row menu", async () => {
    mockUseCustomDomains.mockReturnValue(domainsResult([verifiedDomain]));
    const user = userEvent.setup();
    render(<CustomDomainsSection serviceId="web" />);

    await user.click(
      screen.getByRole("button", { name: "Open domain actions menu" }),
    );
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(
      within(dialog).getByText("Delete www.example.com?"),
    ).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Delete" }));

    await waitFor(() =>
      expect(mockDeleteDomain).toHaveBeenCalledWith("www.example.com"),
    );
  });
});
