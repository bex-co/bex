import {
  BookOpen,
  Container,
  GitBranch,
  KeyRound,
  ScrollText,
  SquareTerminal,
  Terminal,
  type LucideIcon,
} from "lucide-react";
import type { en } from "@/i18n";

/**
 * Render's dashboard search shortcuts that bex has a destination for, in
 * Render's order (w4/711): Render's search jumps "directly to your API keys,
 * webhook settings, compliance docs, and more". The list, its keywords and
 * its matching rule are read from Render's dashboard bundle, and the
 * shortcuts bex leaves out are recorded with it
 * (docs/render-artifacts/search-shortcuts.md).
 *
 * A shortcut whose destination is already a palette page (Overview, Environment
 * Groups, Webhooks, Billing and the two Settings pages) is not repeated here:
 * its keywords join that page's in PAGE_KEYWORDS instead.
 */
export type SearchShortcut = {
  id: string;
  labelKey: keyof typeof en;
  descriptionKey: keyof typeof en;
  /** Render's keywords, kept in English like the product terms they are. */
  keywords: readonly string[];
  icon: LucideIcon;
  /** Only an Admin's Workspace Settings has the section. */
  adminOnly?: boolean;
} & (
  | {
      /** A section of a Settings page: the route and the section's anchor. */
      to: "/settings" | "/workspace/settings";
      hash: string;
    }
  | {
      /** A bex docs page, opened in a new tab as Render opens its docs. */
      href: string;
    }
);

export const SEARCH_SHORTCUTS: readonly SearchShortcut[] = [
  {
    id: "environment-variables-docs",
    labelKey: "common.searchShortcutEnvVarsDocs",
    descriptionKey: "common.searchShortcutEnvVarsDocsDescription",
    keywords: ["env", "env vars", "environment variables", "secrets", ".env"],
    icon: BookOpen,
    href: "https://bex.co/docs/platform/secrets",
  },
  {
    id: "custom-domains-docs",
    labelKey: "common.searchShortcutCustomDomainsDocs",
    descriptionKey: "common.searchShortcutCustomDomainsDocsDescription",
    keywords: ["domain", "custom domain", "dns", "ssl", "certificate"],
    icon: BookOpen,
    href: "https://bex.co/docs/platform/custom-domains",
  },
  {
    id: "git-credentials",
    labelKey: "common.searchShortcutGitCredentials",
    descriptionKey: "common.searchShortcutGitCredentialsDescription",
    keywords: ["git", "github", "gitlab", "bitbucket", "repo", "repository"],
    icon: GitBranch,
    to: "/settings",
    hash: "integrations",
  },
  {
    id: "audit-logs",
    labelKey: "common.searchShortcutAuditLogs",
    descriptionKey: "common.searchShortcutAuditLogsDescription",
    keywords: ["audit", "audit logs"],
    icon: ScrollText,
    adminOnly: true,
    to: "/workspace/settings",
    hash: "audit-logs",
  },
  {
    id: "api-keys",
    labelKey: "common.searchShortcutApiKeys",
    descriptionKey: "common.searchShortcutApiKeysDescription",
    keywords: ["token", "credential", "api key"],
    icon: KeyRound,
    to: "/settings",
    hash: "api-keys",
  },
  {
    id: "ssh-keys",
    labelKey: "common.searchShortcutSshKeys",
    descriptionKey: "common.searchShortcutSshKeysDescription",
    keywords: ["token", "credential", "ssh key"],
    icon: SquareTerminal,
    to: "/settings",
    hash: "ssh-public-keys",
  },
  {
    id: "cli-tokens",
    labelKey: "common.searchShortcutCliTokens",
    descriptionKey: "common.searchShortcutCliTokensDescription",
    keywords: ["token", "credential", "cli"],
    icon: Terminal,
    to: "/settings",
    hash: "cli-tokens",
  },
  {
    id: "registry-credentials",
    labelKey: "common.searchShortcutRegistryCredentials",
    descriptionKey: "common.searchShortcutRegistryCredentialsDescription",
    keywords: ["image", "registry", "docker", "container", "credential"],
    icon: Container,
    to: "/workspace/settings",
    hash: "registry-credentials",
  },
  {
    id: "mcp-server",
    labelKey: "common.searchShortcutMcpServer",
    descriptionKey: "common.searchShortcutMcpServerDescription",
    keywords: ["mcp", "model context protocol", "cursor", "claude", "chatgpt"],
    icon: BookOpen,
    href: "https://bex.co/docs/mcp-server",
  },
];

/**
 * The keywords of Render's shortcuts whose destination is a palette page,
 * by the page's route: Render's Overview is bex's Projects page.
 */
export const PAGE_KEYWORDS: Readonly<Record<string, readonly string[]>> = {
  "/": ["dashboard", "home", "overview"],
  "/env-groups": ["env", "env vars", "environment variables", "secrets"],
  "/webhooks": ["webhook", "webhooks", "integration", "integrations"],
  "/billing": ["billing", "payment", "subscription", "plan", "invoice"],
  "/settings": ["settings", "account", "profile", "user"],
  "/workspace/settings": [
    "settings",
    "workspace",
    "team",
    "members",
    "users",
    "permissions",
  ],
};

/** Render answers no shortcut to a query shorter than this. */
const SHORTCUT_MIN_QUERY = 3;

/**
 * Render's shortcut match: a query of three or more characters that the
 * name, or any keyword, contains, case-insensitively. `query` is already
 * trimmed and lower-cased.
 */
export function matchesShortcut(
  query: string,
  name: string,
  keywords: readonly string[],
): boolean {
  if (query.length < SHORTCUT_MIN_QUERY) return false;
  return (
    name.toLowerCase().includes(query) ||
    keywords.some((keyword) => keyword.toLowerCase().includes(query))
  );
}
