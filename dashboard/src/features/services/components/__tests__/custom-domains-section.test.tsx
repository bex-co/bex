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
const mockVerifyDomain = vi.fn();

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
        verifyDomain: mockVerifyDomain,
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
  domainType: "subdomain",
  verified: true,
  active: true,
  redirectForName: null,
  dnsRecord: { type: "CNAME", name: "www", value: "web.onbex.co" },
};

const pendingDomain: CustomDomainView = {
  name: "api.example.com",
  domainType: "subdomain",
  verified: false,
  active: false,
  redirectForName: null,
  dnsRecord: { type: "CNAME", name: "api", value: "web.onbex.co" },
};

// w6/m23: an auto-paired www<->apex pair, as ListDomains returns after AddDomain
// auto-adds the sibling.
const apexDomain: CustomDomainView = {
  name: "foo.com",
  domainType: "apex",
  verified: false,
  active: false,
  redirectForName: null,
  dnsRecord: { type: "ALIAS", name: "@", value: "web.onbex.co" },
};

const wwwSiblingDomain: CustomDomainView = {
  name: "www.foo.com",
  domainType: "subdomain",
  verified: false,
  active: false,
  redirectForName: "foo.com",
  dnsRecord: { type: "CNAME", name: "www", value: "web.onbex.co" },
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
  mockAddDomain
    .mockReset()
    .mockResolvedValue({ primary: pendingDomain, sibling: null });
  mockDeleteDomain.mockReset().mockResolvedValue(true);
  mockVerifyDomain.mockReset().mockResolvedValue(pendingDomain);
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

  it("auto-expands a pending domain and shows the DNS record to create", () => {
    mockUseCustomDomains.mockReturnValue(domainsResult([pendingDomain]));
    render(<CustomDomainsSection serviceId="web" />);

    // Pending rows open by default, revealing the DNS-setup panel + record.
    expect(screen.getByText("DNS setup")).toBeInTheDocument();
    expect(screen.getByText("CNAME")).toBeInTheDocument();
    expect(screen.getByText("api")).toBeInTheDocument();
    expect(screen.getByText("web.onbex.co")).toBeInTheDocument();
    // Copy affordances for the host + target (labels double as aria-labels).
    expect(screen.getByRole("button", { name: "Target" })).toBeInTheDocument();
  });

  it("shows apex guidance for an apex domain", () => {
    mockUseCustomDomains.mockReturnValue(
      domainsResult([
        {
          name: "example.com",
          domainType: "apex",
          verified: false,
          active: false,
          dnsRecord: { type: "ALIAS", name: "@", value: "web.onbex.co" },
        },
      ]),
    );
    render(<CustomDomainsSection serviceId="web" />);
    expect(screen.getByText("ALIAS")).toBeInTheDocument();
    expect(
      screen.getByText(/Apex domains can't use a plain CNAME/),
    ).toBeInTheDocument();
  });

  it("re-checks a pending domain from the DNS panel", async () => {
    mockUseCustomDomains.mockReturnValue(domainsResult([pendingDomain]));
    const user = userEvent.setup();
    render(<CustomDomainsSection serviceId="web" />);

    await user.click(screen.getByRole("button", { name: "Re-check" }));
    await waitFor(() =>
      expect(mockVerifyDomain).toHaveBeenCalledWith("api.example.com"),
    );
  });

  it("shows the DNS record after adding a domain", async () => {
    mockUseCustomDomains.mockReturnValue(domainsResult([]));
    const user = userEvent.setup();
    render(<CustomDomainsSection serviceId="web" />);

    await user.click(screen.getByRole("button", { name: "Add Custom Domain" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Name"), "api.example.com");
    await user.click(
      within(dialog).getByRole("button", { name: "Add Domain" }),
    );

    // The dialog swaps to the DNS-record step (mockAddDomain resolves a domain view).
    expect(
      await within(dialog).findByText("Domain added — set up DNS"),
    ).toBeInTheDocument();
    expect(within(dialog).getByText("web.onbex.co")).toBeInTheDocument();
  });

  it("shows DNS instructions for both halves of an auto-paired add (w6/m23)", async () => {
    mockUseCustomDomains.mockReturnValue(domainsResult([]));
    mockAddDomain.mockResolvedValue({
      primary: apexDomain,
      sibling: wwwSiblingDomain,
    });
    const user = userEvent.setup();
    render(<CustomDomainsSection serviceId="web" />);

    await user.click(screen.getByRole("button", { name: "Add Custom Domain" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Name"), "foo.com");
    await user.click(
      within(dialog).getByRole("button", { name: "Add Domain" }),
    );

    expect(
      await within(dialog).findByText(
        "www.foo.com was added automatically and redirects to foo.com — set up its DNS too",
      ),
    ).toBeInTheDocument();
    // Both records' host fields are shown: "@" for the apex, "www" for the sibling.
    expect(within(dialog).getByText("@")).toBeInTheDocument();
    expect(within(dialog).getByText("www")).toBeInTheDocument();
  });

  it("marks the redirecting sibling and canonical rows (w6/m30)", () => {
    mockUseCustomDomains.mockReturnValue(
      domainsResult([apexDomain, wwwSiblingDomain]),
    );
    render(<CustomDomainsSection serviceId="web" />);

    expect(screen.getByText("www.foo.com redirects here")).toBeInTheDocument();
    expect(screen.getByText("Redirects to foo.com")).toBeInTheDocument();
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
