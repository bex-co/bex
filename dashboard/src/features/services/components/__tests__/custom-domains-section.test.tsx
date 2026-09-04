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
const mockClearAddError = vi.fn();
let mockAddError: string | null = null;

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
        addError: mockAddError,
        clearAddError: mockClearAddError,
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
  ownershipVerified: true,
  verified: true,
  active: true,
  redirectForName: null,
  dnsRecord: { type: "CNAME", name: "www", value: "web.onbex.co" },
  ownershipDnsRecord: null,
};

const pendingDomain: CustomDomainView = {
  name: "api.example.com",
  domainType: "subdomain",
  ownershipVerified: false,
  verified: false,
  active: false,
  redirectForName: null,
  dnsRecord: { type: "CNAME", name: "api", value: "web.onbex.co" },
  ownershipDnsRecord: {
    type: "TXT",
    name: "_bex-challenge.example.com",
    value: "bex-domain-verification=test-api",
  },
};

// w6/m23: an auto-paired www<->apex pair, as ListDomains returns after AddDomain
// auto-adds the sibling.
const apexDomain: CustomDomainView = {
  name: "foo.com",
  domainType: "apex",
  ownershipVerified: false,
  verified: false,
  active: false,
  redirectForName: null,
  dnsRecord: { type: "ALIAS", name: "@", value: "web.onbex.co" },
  ownershipDnsRecord: {
    type: "TXT",
    name: "_bex-challenge.foo.com",
    value: "bex-domain-verification=test-apex",
  },
};

const wwwSiblingDomain: CustomDomainView = {
  name: "www.foo.com",
  domainType: "subdomain",
  ownershipVerified: false,
  verified: false,
  active: false,
  redirectForName: "foo.com",
  dnsRecord: { type: "CNAME", name: "www", value: "web.onbex.co" },
  ownershipDnsRecord: {
    type: "TXT",
    name: "_bex-challenge.foo.com",
    value: "bex-domain-verification=test-www",
  },
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
  mockClearAddError.mockReset();
  mockAddError = null;
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
          ownershipVerified: false,
          verified: false,
          active: false,
          ownershipDnsRecord: pendingDomain.ownershipDnsRecord,
        },
      ]),
    );
    render(<CustomDomainsSection serviceId="web" />);
    expect(screen.getAllByText("Pending")).toHaveLength(2);
  });

  it("shows ownership verified while certificate activation is pending", () => {
    mockUseCustomDomains.mockReturnValue(
      domainsResult([
        {
          ...verifiedDomain,
          verified: false,
          active: false,
        },
      ]),
    );
    render(<CustomDomainsSection serviceId="web" />);
    expect(screen.getByText("Verified")).toBeInTheDocument();
    expect(screen.getByText("Pending")).toBeInTheDocument();
  });

  it("keeps certificate verification distinct from inactive serving", () => {
    mockUseCustomDomains.mockReturnValue(
      domainsResult([{ ...verifiedDomain, active: false }]),
    );
    render(<CustomDomainsSection serviceId="web" />);
    expect(screen.getAllByText("Verified")).toHaveLength(2);
    expect(screen.queryByText("Active")).not.toBeInTheDocument();
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

  it("shows the server's rejection reason inline in the add dialog", async () => {
    // The server refused the add (e.g. a wildcard). The dialog stays open and
    // shows *why* persistently — not just a transient toast that vanishes.
    mockAddError = "Wildcard hostnames are not allowed";
    mockUseCustomDomains.mockReturnValue(domainsResult([]));
    const user = userEvent.setup();
    render(<CustomDomainsSection serviceId="web" />);

    await user.click(screen.getByRole("button", { name: "Add Custom Domain" }));
    const dialog = await screen.findByRole("dialog");
    const alert = within(dialog).getByRole("alert");
    expect(alert).toHaveTextContent("Wildcard hostnames are not allowed");
    // The dialog stays on the input step (not swapped to the DNS-record step).
    expect(within(dialog).getByLabelText("Name")).toBeInTheDocument();
  });

  it("clears the server error when the domain input changes", async () => {
    mockAddError = "Wildcard hostnames are not allowed";
    mockUseCustomDomains.mockReturnValue(domainsResult([]));
    const user = userEvent.setup();
    render(<CustomDomainsSection serviceId="web" />);

    await user.click(screen.getByRole("button", { name: "Add Custom Domain" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Name"), "a");
    expect(mockClearAddError).toHaveBeenCalled();
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

  it("distinguishes ownership pending and shows TXT plus traffic records", () => {
    mockUseCustomDomains.mockReturnValue(domainsResult([pendingDomain]));
    render(<CustomDomainsSection serviceId="web" />);

    // Pending rows open by default, revealing the DNS-setup panel + record.
    expect(screen.getByText("DNS setup")).toBeInTheDocument();
    expect(screen.getByText("TXT")).toBeInTheDocument();
    expect(screen.getByText("CNAME")).toBeInTheDocument();
    expect(screen.getByText("_bex-challenge.example.com")).toBeInTheDocument();
    expect(
      screen.getByText("bex-domain-verification=test-api"),
    ).toBeInTheDocument();
    expect(screen.getByText("api")).toBeInTheDocument();
    expect(screen.getByText("web.onbex.co")).toBeInTheDocument();
    // Copy affordances for the host + target (labels double as aria-labels).
    expect(screen.getAllByRole("button", { name: "Target" })).toHaveLength(2);
  });

  it("shows apex guidance for an apex domain", () => {
    mockUseCustomDomains.mockReturnValue(
      domainsResult([
        {
          name: "example.com",
          domainType: "apex",
          ownershipVerified: false,
          verified: false,
          active: false,
          redirectForName: null,
          dnsRecord: { type: "ALIAS", name: "@", value: "web.onbex.co" },
          ownershipDnsRecord: {
            type: "TXT",
            name: "_bex-challenge.example.com",
            value: "bex-domain-verification=apex",
          },
        },
      ]),
    );
    render(<CustomDomainsSection serviceId="web" />);
    expect(screen.getByText("ALIAS")).toBeInTheDocument();
    expect(
      screen.getByText(/Apex domains can't use a plain CNAME/),
    ).toBeInTheDocument();
  });

  it("lists sibling TXT values sharing the ownership host so none get overwritten (w6/055)", () => {
    mockUseCustomDomains.mockReturnValue(
      domainsResult([apexDomain, wwwSiblingDomain]),
    );
    render(<CustomDomainsSection serviceId="web" />);

    // Both pending rows are open; each shows its own TXT value once as the
    // record to add, plus once more inside the sibling row's keep-these list.
    expect(
      screen.getAllByText("bex-domain-verification=test-apex"),
    ).toHaveLength(2);
    expect(
      screen.getAllByText("bex-domain-verification=test-www"),
    ).toHaveLength(2);
    expect(
      screen.getAllByText(
        "Other domains on this service share this TXT host — keep their records in place:",
      ),
    ).toHaveLength(2);
  });

  it("shows no sibling TXT note for a lone pending claim (w6/055)", () => {
    mockUseCustomDomains.mockReturnValue(domainsResult([pendingDomain]));
    render(<CustomDomainsSection serviceId="web" />);
    expect(screen.queryByText(/share this TXT host/)).not.toBeInTheDocument();
    expect(
      screen.getByText("bex-domain-verification=test-api"),
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
      await within(dialog).findByText("Domain claim created — set up DNS"),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByText("_bex-challenge.example.com"),
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
    expect(dialog).toHaveTextContent("The service stops serving this domain.");
    await user.click(within(dialog).getByRole("button", { name: "Delete" }));

    await waitFor(() =>
      expect(mockDeleteDomain).toHaveBeenCalledWith("www.example.com"),
    );
  });

  it("names both removals when deleting an auto-pair canonical domain", async () => {
    mockUseCustomDomains.mockReturnValue(
      domainsResult([apexDomain, wwwSiblingDomain]),
    );
    const user = userEvent.setup();
    render(<CustomDomainsSection serviceId="web" />);

    await user.click(
      screen.getAllByRole("button", { name: "Open domain actions menu" })[0],
    );
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent(
      "This removes foo.com and its automatically added redirect www.foo.com.",
    );
    await user.click(within(dialog).getByRole("button", { name: "Delete" }));

    await waitFor(() =>
      expect(mockDeleteDomain).toHaveBeenCalledWith("foo.com"),
    );
  });

  it("discloses the unchanged canonical when deleting only the generated redirect", async () => {
    mockUseCustomDomains.mockReturnValue(
      domainsResult([apexDomain, wwwSiblingDomain]),
    );
    const user = userEvent.setup();
    render(<CustomDomainsSection serviceId="web" />);

    await user.click(
      screen.getAllByRole("button", { name: "Open domain actions menu" })[1],
    );
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent(
      "This removes only www.foo.com. foo.com remains the canonical domain and does not redirect.",
    );
  });

  it("treats explicitly claimed www and apex domains as independent", async () => {
    mockUseCustomDomains.mockReturnValue(
      domainsResult([
        apexDomain,
        { ...wwwSiblingDomain, redirectForName: null },
      ]),
    );
    const user = userEvent.setup();
    render(<CustomDomainsSection serviceId="web" />);

    await user.click(
      screen.getAllByRole("button", { name: "Open domain actions menu" })[0],
    );
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent("The service stops serving this domain.");
    expect(dialog).not.toHaveTextContent("automatically added redirect");
  });
});
