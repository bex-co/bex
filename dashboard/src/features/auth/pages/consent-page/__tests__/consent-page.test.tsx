import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type {
  ConsentErrorCode,
  ConsentView,
} from "@/common/server-fn/hydra-consent";

// The page is a pure render of the loader data the route's GET handler produced
// (docs/ADR012-auth.md §7, w4/m16) — mock the route accessor so the test drives
// that data directly, with no router mount and no Hydra.
const routeData = vi.hoisted(() => ({
  consent: null as ConsentView | null,
  consentErrorCode: null as ConsentErrorCode | null,
  consentRequiredScopes: null as string[] | null,
  challenge: undefined as string | undefined,
}));
vi.mock("@tanstack/react-router", async (orig) => ({
  ...(await orig<typeof import("@tanstack/react-router")>()),
  getRouteApi: () => ({
    useLoaderData: () => ({
      consent: routeData.consent,
      consentErrorCode: routeData.consentErrorCode,
      consentRequiredScopes: routeData.consentRequiredScopes,
    }),
    useSearch: () => ({ consent_challenge: routeData.challenge }),
  }),
}));

import ConsentPage from "@/features/auth/pages/consent-page";

const view = (overrides: Partial<ConsentView> = {}): ConsentView => ({
  challenge: "challenge-1",
  clientId: "51fb9b2c-agent",
  clientName: "Claude Code",
  redirectOrigin: "https://agent.example",
  scopes: ["openid", "offline_access"],
  audiences: ["https://api.bex.co/mcp"],
  csrfToken: "csrf-token-1",
  retryAfterFailure: false,
  ...overrides,
});

const replace = vi.fn();

beforeEach(() => {
  routeData.consent = null;
  routeData.consentErrorCode = null;
  routeData.consentRequiredScopes = null;
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
    expect(screen.getByText("Unverified third-party app")).toBeInTheDocument();
    expect(screen.getByText("51fb9b2c-agent")).toBeInTheDocument();
    expect(screen.getByText("https://agent.example")).toBeInTheDocument();
    expect(screen.getByText("openid")).toBeInTheDocument();
    expect(screen.getByText("offline_access")).toBeInTheDocument();
    expect(screen.getByText("https://api.bex.co/mcp")).toBeInTheDocument();
  });

  it("describes granular capabilities and highlights write/sensitive risk", () => {
    routeData.consent = view({
      scopes: ["openid", "bex.read", "bex.write", "bex.sensitive"],
    });

    render(<ConsentPage />);

    expect(
      screen.getByText(
        "Read ordinary workspace resources (services, logs, metrics)",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Change workspace resources (deploy, restart, create, delete, billing)",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Read secrets and connection strings (env vars, database URLs, files)",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("bex.write")).toBeInTheDocument();
    expect(screen.getByText("bex.sensitive")).toBeInTheDocument();
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

  it("still carries the pressed button's decision once the form is submitting", () => {
    // The pressed button IS the decision — it carries `name="decision"`, and the
    // browser reads the entry list only *after* the submit handler returns,
    // excluding any control disabled by then. So disabling the buttons on submit
    // (to guard a double-click) drops the submitter's own field: the POST lands
    // with a challenge, a CSRF token, and no decision at all — a 400 for every
    // real browser, while a scripted client that posts the three fields itself
    // sails through untouched. Found in a real browser (w4/m17 t001).
    routeData.consent = view();

    const { container } = render(<ConsentPage />);
    const form = container.querySelector("form")!;
    const approve = container.querySelector<HTMLButtonElement>(
      'button[value="approve"]',
    )!;
    fireEvent.submit(form, { submitter: approve });

    // What the browser would actually send, with the form in its submitting state.
    const sent = new FormData(form, approve);
    expect(sent.get("decision")).toBe("approve");
    expect(sent.get("consent_challenge")).toBe("challenge-1");
    expect(sent.get("csrf_token")).toBe("csrf-token-1");
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

  // w10/m8 t002: a recoverable GET/POST failure renders inside AuthPageShell
  // with error-specific copy instead of the bare text body the checks used to
  // return directly.
  it.each([
    ["pkce_required", "Invalid authorization request"],
    ["not_configured", "Authorization unavailable"],
    ["headless_accept_failed", "Authorization unavailable"],
    ["wrong_user", "Signed in as the wrong account"],
  ] satisfies [ConsentErrorCode, string][])(
    "renders the %s error state with its own copy",
    (code, expectedTitle) => {
      routeData.consent = null;
      routeData.consentErrorCode = code;
      routeData.challenge = "challenge-1";

      render(<ConsentPage />);

      expect(screen.getByText(expectedTitle)).toBeInTheDocument();
    },
  );

  it("names the missing capability scopes on a scope_required refusal (round-14 #1)", () => {
    routeData.consent = null;
    routeData.consentErrorCode = "scope_required";
    routeData.consentRequiredScopes = ["bex.read", "bex.write", "bex.sensitive"];
    routeData.challenge = "challenge-1";

    render(<ConsentPage />);

    expect(screen.getByText("Invalid authorization request")).toBeInTheDocument();
    expect(screen.getByText(/bex\.read, bex\.write, bex\.sensitive/)).toBeInTheDocument();
  });

  it("never reloads a recovered error code — the handler already answered", () => {
    routeData.consent = null;
    routeData.consentErrorCode = "wrong_user";
    routeData.challenge = "challenge-1";

    render(<ConsentPage />);

    expect(replace).not.toHaveBeenCalled();
    expect(screen.getByRole("link")).toHaveAttribute("href", "/");
  });
});
