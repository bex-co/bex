import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import type { ConsentView } from "@/common/server-fn/hydra-consent";

// The page is a pure render of the loader data the route's GET handler produced
// (docs/ADR012-auth.md §7, w4/m16) — mock the route accessor so the test drives
// that data directly, with no router mount and no Hydra.
const routeData = vi.hoisted(() => ({
  consent: null as ConsentView | null,
  challenge: undefined as string | undefined,
}));
vi.mock("@tanstack/react-router", async (orig) => ({
  ...(await orig<typeof import("@tanstack/react-router")>()),
  getRouteApi: () => ({
    useLoaderData: () => ({ consent: routeData.consent }),
    useSearch: () => ({ consent_challenge: routeData.challenge }),
  }),
}));

import ConsentPage from "@/features/auth/pages/consent-page";

const view = (overrides: Partial<ConsentView> = {}): ConsentView => ({
  challenge: "challenge-1",
  clientId: "51fb9b2c-agent",
  clientName: "Claude Code",
  scopes: ["openid", "offline_access"],
  audiences: ["https://api.bex.co/mcp"],
  csrfToken: "csrf-token-1",
  retryAfterFailure: false,
  ...overrides,
});

const replace = vi.fn();

beforeEach(() => {
  routeData.consent = null;
  routeData.challenge = undefined;
  replace.mockClear();
  vi.stubGlobal("location", {
    href: "https://dashboard.bex.co/auth/consent?consent_challenge=challenge-1",
    replace,
  });
});

describe("ConsentPage", () => {
  it("names the client and lists every requested scope and audience", () => {
    routeData.consent = view();

    render(<ConsentPage />);

    expect(screen.getAllByText("Claude Code").length).toBeGreaterThan(0);
    expect(screen.getByText("openid")).toBeInTheDocument();
    expect(screen.getByText("offline_access")).toBeInTheDocument();
    expect(screen.getByText("https://api.bex.co/mcp")).toBeInTheDocument();
  });

  it("posts the decision back with the challenge and its CSRF token", () => {
    routeData.consent = view();

    const { container } = render(<ConsentPage />);

    const form = container.querySelector("form")!;
    expect(form.getAttribute("method")).toBe("post");
    expect(form.getAttribute("action")).toBe("/auth/consent");
    expect(
      container.querySelector('input[name="consent_challenge"]'),
    ).toHaveValue("challenge-1");
    expect(container.querySelector('input[name="csrf_token"]')).toHaveValue(
      "csrf-token-1",
    );
    const decisions = [
      ...container.querySelectorAll('button[name="decision"]'),
    ];
    expect(decisions.map((b) => b.getAttribute("value")).sort()).toEqual([
      "approve",
      "deny",
    ]);
  });

  it("surfaces a failed decision instead of silently re-asking", () => {
    routeData.consent = view({ retryAfterFailure: true });

    render(<ConsentPage />);

    expect(screen.getByRole("alert")).toBeInTheDocument();
  });

  it("reloads as a document when it arrives by client-side navigation with a challenge", () => {
    // The login-first bounce returns here via the router, which never runs the
    // GET server handler — so there is no consent view to render. Reloading is
    // what makes the handler answer (and the reload is what the user came for).
    routeData.consent = null;
    routeData.challenge = "challenge-1";

    render(<ConsentPage />);

    expect(replace).toHaveBeenCalledWith(
      "https://dashboard.bex.co/auth/consent?consent_challenge=challenge-1",
    );
  });

  it("shows the expired state — not a reload loop — when there is no challenge at all", () => {
    routeData.consent = null;
    routeData.challenge = undefined;

    render(<ConsentPage />);

    expect(replace).not.toHaveBeenCalled();
    expect(screen.getByRole("link")).toHaveAttribute("href", "/");
  });
});
