import type { ReactElement } from "react";
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render } from "@testing-library/react";
import type {
  LoginFlow,
  RecoveryFlow,
  RegistrationFlow,
  VerificationFlow,
} from "@ory/client-fetch";
import {
  Login,
  Recovery,
  Registration,
  Verification,
} from "@ory/elements-react/theme";
import i18n, { switchLanguage } from "@/i18n/init";
import { SUPPORTED_LANGUAGES } from "@/i18n/config";
import { KRATOS_PUBLIC_URL, useOryConfig } from "@/common/lib/ory/config";
import { oryAuthFormOverrides } from "@/common/lib/ory/auth-form-overrides";
import {
  AuthWidgetSkeleton,
  LoginWidgetSkeleton,
  RegistrationWidgetSkeleton,
} from "@/common/components/route-skeletons";

/**
 * w7/092: the login, sign-up, forgot-password and verification skeletons
 * reserve the words Ory's card draws, its header and its footer link, in
 * Ory's words in the page's language, so the card does not move when it
 * lands. At 390 px Ory's subtitle runs from one line (ko login) to three (de
 * login), so a header sized for English shifted the card by a line in the
 * other languages; at 320 px the footer wraps too. The skeleton lays out the
 * same text in the same type as the ready card, so it wraps to the same lines
 * at every width: these tests pin that the text is the same.
 */

/** A node as Kratos v26.2.0 sends it (production's flows, read 2026-11-18). */
function node(
  group: string,
  name: string,
  type: string,
  label?: { id: number; text: string; context?: Record<string, string> },
  value = "",
) {
  return {
    type: "input",
    group,
    attributes: { name, type, value, node_type: "input", disabled: false },
    messages: [],
    meta: label ? { label: { ...label, type: "info" } } : {},
  };
}

const CSRF = node("default", "csrf_token", "hidden", undefined, "csrf-1");
const GITHUB = (messageId: number, verb: string) =>
  node(
    "oidc",
    "provider",
    "submit",
    {
      id: messageId,
      text: `${verb} with GitHub`,
      context: { provider: "GitHub", provider_id: "github" },
    },
    "github",
  );
const EMAIL_TRAIT = {
  id: 1070002,
  text: "E-Mail",
  context: { name: "traits.email", title: "E-Mail" },
};

/** A flow's first step as Kratos mints it. Recovery and verification are
 *  `active: "code"`, the method production's Kratos enables. */
function flow(kind: string, nodes: unknown[], active?: string) {
  return {
    id: `${kind}-1`,
    type: "browser",
    state: "choose_method",
    ...(active ? { active } : {}),
    refresh: false,
    requested_aal: "aal1",
    expires_at: new Date("2030-01-01T00:00:00Z"),
    issued_at: new Date("2026-11-18T00:00:00Z"),
    request_url: `${KRATOS_PUBLIC_URL}/self-service/${kind}/browser`,
    ui: {
      action: `${KRATOS_PUBLIC_URL}/self-service/${kind}?flow=${kind}-1`,
      method: "POST",
      nodes,
    },
  };
}

function loginFlow(sso: boolean): LoginFlow {
  return flow("login", [
    ...(sso ? [GITHUB(1010002, "Sign in")] : []),
    CSRF,
    node("default", "identifier", "text", EMAIL_TRAIT),
    node("password", "password", "password", { id: 1070001, text: "Password" }),
    node(
      "password",
      "method",
      "submit",
      { id: 1010022, text: "Sign in with password" },
      "password",
    ),
  ]) as unknown as LoginFlow;
}

function registrationFlow(sso: boolean): RegistrationFlow {
  return flow("registration", [
    ...(sso ? [GITHUB(1040002, "Sign up")] : []),
    CSRF,
    node("default", "traits.email", "email", EMAIL_TRAIT),
    node("default", "traits.name", "text", {
      id: 1070002,
      text: "Name",
      context: { name: "traits.name", title: "Name" },
    }),
    node(
      "profile",
      "method",
      "submit",
      { id: 1040001, text: "Sign up" },
      "profile",
    ),
  ]) as unknown as RegistrationFlow;
}

/** Forgot-password's and verification's first step: the e-mail and Continue. */
function codeFlowNodes() {
  return [
    CSRF,
    node("code", "email", "email", { id: 1070007, text: "Email" }),
    node("code", "method", "submit", { id: 1070009, text: "Continue" }, "code"),
  ];
}

/** Each card as its page draws it: Ory's flow component with the page's
 *  config (`useOryConfig`, so its language and its field labels) and the
 *  auth overrides. */
function ReadyCard({ card, sso }: { card: Card; sso: boolean }) {
  const config = useOryConfig();
  const props = { config, components: oryAuthFormOverrides };
  switch (card) {
    case "login":
      return <Login flow={loginFlow(sso)} {...props} />;
    case "registration":
      return <Registration flow={registrationFlow(sso)} {...props} />;
    case "recovery":
      return (
        <Recovery
          flow={
            flow("recovery", codeFlowNodes(), "code") as unknown as RecoveryFlow
          }
          {...props}
        />
      );
    case "verification":
      return (
        <Verification
          flow={
            flow(
              "verification",
              codeFlowNodes(),
              "code",
            ) as unknown as VerificationFlow
          }
          {...props}
        />
      );
  }
}

type Card = "login" | "registration" | "recovery" | "verification";

function skeleton(card: Card, sso: boolean): ReactElement {
  const providers = sso ? ["github"] : [];
  switch (card) {
    case "login":
      return <LoginWidgetSkeleton ssoProviders={providers} />;
    case "registration":
      return <RegistrationWidgetSkeleton ssoProviders={providers} />;
    default:
      return <AuthWidgetSkeleton flow={card} />;
  }
}

/** The words the ready card draws: its header, and its footer's link to the
 *  other auth page (login and sign-up only). */
function readyCopy(card: Card, sso: boolean) {
  const { container, unmount } = render(<ReadyCard card={card} sso={sso} />);
  const header = container.querySelector('[data-testid="ory/card"] header');
  const footerLink = container.querySelector(
    '[data-testid="ory/screen/login/action/register"], [data-testid="ory/screen/registration/action/login"]',
  );
  const result = {
    title: header?.querySelector("h2")?.textContent ?? null,
    subtitle: header?.querySelector("p")?.textContent ?? null,
    footer: footerLink?.parentElement?.textContent ?? null,
  };
  unmount();
  return result;
}

/** The words the skeleton reserves in the same places. */
function skeletonCopy(card: Card, sso: boolean) {
  const { container, unmount } = render(skeleton(card, sso));
  const header = container.querySelector("[data-skeleton-card-header]");
  const [title, subtitle] = Array.from(header?.children ?? []);
  const result = {
    title: title?.textContent ?? null,
    subtitle: subtitle?.textContent ?? null,
    footer:
      container.querySelector("[data-skeleton-card-footer]")?.textContent ??
      null,
  };
  unmount();
  return result;
}

const CASES: [Card, boolean][] = [
  ["login", true],
  ["login", false],
  ["registration", true],
  ["registration", false],
  ["recovery", false],
  ["verification", false],
];

afterEach(async () => {
  cleanup();
  await i18n.changeLanguage("en");
});

describe("the auth card skeletons reserve Ory's words in the page's language (w7/092)", () => {
  it.each(SUPPORTED_LANGUAGES)(
    "%s: each card's title, subtitle and footer are the ready card's",
    async (lang) => {
      await switchLanguage(lang);
      for (const [card, sso] of CASES) {
        const ready = readyCopy(card, sso);
        expect(ready.subtitle, `${card} sso=${sso}`).toBeTruthy();
        expect(ready.footer !== null, `${card} footer`).toBe(
          card === "login" || card === "registration",
        );
        expect(skeletonCopy(card, sso), `${lang} ${card} sso=${sso}`).toEqual(
          ready,
        );
      }
    },
  );

  it("draws each language's own words, not English sized for every language", async () => {
    await switchLanguage("ko");
    expect(skeletonCopy("login", true)).toEqual({
      title: "로그인",
      subtitle: "email과 비밀번호 또는 소셜 제공자로 로그인",
      footer: "계정이 없으신가요? 가입",
    });
    await switchLanguage("de");
    expect(skeletonCopy("login", true).subtitle).toBe(
      "Melden Sie sich mit Ihrer email und Ihrem Passwort oder einem Social-Login an.",
    );
  });

  it("lays the words out in Ory's type: an 18 px title over 16 px text, all 1.5 lines", () => {
    const { container } = render(skeleton("login", true));
    expect(
      container.querySelector('[data-skeleton-region="auth-widget"]'),
    ).toHaveClass("[font-family:var(--font-sans-default)]");
    const header = container.querySelector("[data-skeleton-card-header]");
    expect(header).toHaveClass("flex", "flex-col", "gap-2");
    const [title, subtitle] = Array.from(header!.children);
    const footer = container.querySelector("[data-skeleton-card-footer]");
    expect(title).toHaveClass("text-lg", "leading-normal", "font-semibold");
    for (const text of [subtitle, footer]) {
      expect(text).toHaveClass("text-base", "leading-normal");
    }
    // Each reserves its words under one bar, nothing drawn at a fixed height.
    for (const line of [title, subtitle, footer]) {
      expect(line!.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(1);
      expect(line!.querySelector(".invisible")).not.toBeNull();
    }
  });
});
