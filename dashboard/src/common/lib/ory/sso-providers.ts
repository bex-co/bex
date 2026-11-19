import { isUiNodeInputAttributes, type UiNode } from "@ory/client-fetch";
import { createFrontendApi } from "@/common/lib/ory/frontend";

/**
 * Render's sign-in providers, in the order its login and sign-up pages list
 * them (dashboard.render.com/login and /register, checked 2026-11-17): GitHub
 * and Google side by side, then GitLab and Bitbucket on a second row that
 * "More options" reveals. Kratos lists its oidc nodes by provider id instead
 * (bitbucket, github, gitlab, google).
 */
const SSO_PROVIDER_ORDER: readonly string[] = [
  "github",
  "google",
  "gitlab",
  "bitbucket",
];

/** The providers Render keeps behind "More options". */
const MORE_OPTIONS_PROVIDERS: ReadonlySet<string> = new Set([
  "gitlab",
  "bitbucket",
]);

/**
 * The installation's providers as Render arranges them: the first row, and
 * the row "More options" reveals. Any provider bex does not know joins the
 * first row after GitHub and Google, in the order it came. With nothing for
 * the first row, the others take it and there is no "More options".
 */
export function arrangeSsoProviders(items: readonly string[]): {
  primary: string[];
  more: string[];
};
export function arrangeSsoProviders<T>(
  items: readonly T[],
  providerOf: (item: T) => string,
): { primary: T[]; more: T[] };
export function arrangeSsoProviders<T>(
  items: readonly T[],
  providerOf: (item: T) => string = String,
): { primary: T[]; more: T[] } {
  const rank = (item: T) => {
    const index = SSO_PROVIDER_ORDER.indexOf(providerOf(item));
    return index === -1 ? SSO_PROVIDER_ORDER.length : index;
  };
  const sorted = items
    .map((item, index) => ({ item, index }))
    .sort((a, b) => rank(a.item) - rank(b.item) || a.index - b.index)
    .map(({ item }) => item);
  const primary = sorted.filter(
    (item) => !MORE_OPTIONS_PROVIDERS.has(providerOf(item)),
  );
  const more = sorted.filter((item) =>
    MORE_OPTIONS_PROVIDERS.has(providerOf(item)),
  );
  return primary.length === 0 ? { primary: more, more: [] } : { primary, more };
}

/** A row of provider buttons: one column per button, three at most, as on
 *  Render (GitHub | Google, and GitHub | Google | SSO on its login). */
export function ssoRowColumns(count: number): string {
  if (count >= 3) return "grid-cols-3";
  return count === 2 ? "grid-cols-2" : "grid-cols-1";
}

/** A social sign-in node: Kratos's `oidc` and `saml` groups. */
export function isSsoNode(node: UiNode): boolean {
  return node.group === "oidc" || node.group === "saml";
}

/**
 * The provider id of a social sign-in button node, or undefined for any other
 * node: `provider` on login and sign-up, `link` and `unlink` in settings. The
 * skeletons count with it and the ready card lays out with it.
 */
export function ssoProviderOf(node: UiNode | undefined): string | undefined {
  if (!node || !isSsoNode(node)) return undefined;
  if (!isUiNodeInputAttributes(node.attributes)) return undefined;
  const { name, value } = node.attributes;
  if (name !== "provider" && name !== "link" && name !== "unlink") {
    return undefined;
  }
  return typeof value === "string" && value !== "" ? value : undefined;
}

/** The provider ids a Kratos flow's nodes offer, in the flow's order. */
export function ssoProvidersOfNodes(nodes: readonly UiNode[]): string[] {
  const providers: string[] = [];
  for (const node of nodes) {
    const provider = ssoProviderOf(node);
    if (provider && !providers.includes(provider)) providers.push(provider);
  }
  return providers;
}

// The installation's providers change only with Kratos's config, so the SSR
// server keeps its answer and refreshes it in the background once it is this
// old.
const SERVER_TTL_MS = 5 * 60_000;
// With no answer yet, a failed ask is not repeated for this long, so a Kratos
// that is down does not hold every page for the timeout.
const SERVER_RETRY_MS = 15_000;
const KRATOS_TIMEOUT_MS = 2_000;

const server: {
  providers: string[] | null;
  checkedAt: number;
  refresh: Promise<string[] | null> | null;
} = { providers: null, checkedAt: Number.NEGATIVE_INFINITY, refresh: null };

async function fetchSsoProviders(): Promise<string[] | null> {
  try {
    // A native (API) login flow carries the same oidc nodes a browser flow
    // does, with no cookie and no CSRF token to set up: the installation's
    // providers, from Kratos itself.
    const flow = await createFrontendApi().createNativeLoginFlow(undefined, {
      credentials: "omit",
      signal: AbortSignal.timeout(KRATOS_TIMEOUT_MS),
    });
    return ssoProvidersOfNodes(flow.ui.nodes);
  } catch {
    return null;
  }
}

/** One ask at a time; a failed ask keeps the last good answer. */
function refreshServerProviders(): Promise<string[] | null> {
  server.refresh ??= fetchSsoProviders()
    .then((providers) => {
      server.checkedAt = Date.now();
      if (providers) server.providers = providers;
      return server.providers;
    })
    .finally(() => {
      server.refresh = null;
    });
  return server.refresh;
}

/**
 * The sign-in providers this installation offers (the Kratos oidc provider
 * ids), or null when it is not known. The login and sign-up routes load it so
 * their skeletons draw the provider buttons the ready card will have.
 *
 * On the SSR server it is Kratos's answer, kept and refreshed in the
 * background; only a pod that has never had one waits for Kratos. The browser
 * never asks Kratos for it, so a client-side navigation never waits on a
 * second Kratos request before the page creates its own flow: it answers with
 * what the browser last read from a login or sign-up flow
 * (`useRememberSsoProviders`).
 */
export async function loadSsoProviders(): Promise<string[] | null> {
  if (!import.meta.env.SSR) return rememberedSsoProviders();
  const age = Date.now() - server.checkedAt;
  if (server.providers) {
    if (age >= SERVER_TTL_MS) void refreshServerProviders();
    return server.providers;
  }
  if (age < SERVER_RETRY_MS) return null;
  return refreshServerProviders();
}

// The browser keeps the provider ids it last read from a flow, in memory and
// in localStorage, so a later sign-out in the same browser draws them too.
// They are the installation's provider names, not a credential.
export const SSO_PROVIDERS_KEY = "bex.auth.sso-providers";
const PROVIDER_ID = /^[A-Za-z0-9_.:-]{1,64}$/;

let remembered: string[] | null | undefined;

function readStoredProviders(): string[] | null {
  try {
    const parsed: unknown = JSON.parse(
      window.localStorage.getItem(SSO_PROVIDERS_KEY) ?? "null",
    );
    return Array.isArray(parsed) &&
      parsed.every((id) => typeof id === "string" && PROVIDER_ID.test(id))
      ? parsed
      : null;
  } catch {
    return null;
  }
}

/** Keep the providers a login or sign-up flow offered. */
export function rememberSsoProviders(providers: string[]): void {
  remembered = providers;
  try {
    window.localStorage.setItem(SSO_PROVIDERS_KEY, JSON.stringify(providers));
  } catch {
    // Without storage the list lasts as long as the page.
  }
}

/** The providers this browser last read from a flow, or null before it has. */
export function rememberedSsoProviders(): string[] | null {
  if (typeof window === "undefined") return null;
  if (remembered === undefined) remembered = readStoredProviders();
  return remembered;
}

/** Tests only: forget every cached answer. */
export function resetSsoProvidersCache(): void {
  server.providers = null;
  server.checkedAt = Number.NEGATIVE_INFINITY;
  server.refresh = null;
  remembered = undefined;
}
