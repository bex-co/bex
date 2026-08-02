import type { MobileWorkspace } from "./workspace-selection";

/**
 * Localized "role · plan" status line for a workspace row. Roles and plans are
 * closed backend enums (docs/ADR024-members.md roles; `store/plans.go` plans),
 * so known values map to translated labels and anything unmapped falls back to
 * its title-cased raw value rather than an i18n "missing" string. Pure so the
 * ordering and fallback are unit-testable.
 */

// Lowercase FGA role forms (workspaces render.go / members.Roles).
const KNOWN_ROLES = [
  "admin",
  "developer",
  "contributor",
  "viewer",
  "billing",
] as const;

// Workspace plan catalog (store/plans.go), plus legacy aliases for old rows.
const KNOWN_PLANS = ["hobby", "pro", "scale", "enterprise", "free"] as const;

const roleSet = new Set<string>(KNOWN_ROLES);
const planSet = new Set<string>(KNOWN_PLANS);

function titleCase(value: string): string {
  return value.replace(/\S+/g, (word) =>
    word ? word[0].toUpperCase() + word.slice(1).toLowerCase() : word,
  );
}

/** Resolve a raw value to a translated label, or its title-cased self. */
function label(
  raw: string,
  known: Set<string>,
  translate: (slug: string) => string,
): string {
  const slug = raw.trim().toLowerCase();
  return known.has(slug) ? translate(slug) : titleCase(raw.trim());
}

/**
 * Build the row's status line from a workspace. Role precedes plan; a missing
 * field is omitted rather than shown blank, and an all-missing workspace yields
 * an empty string (the row then shows its name alone).
 */
export function workspaceStatusLabel(
  workspace: Pick<MobileWorkspace, "role" | "plan">,
  translate: { role: (slug: string) => string; plan: (slug: string) => string },
): string {
  const parts: string[] = [];
  if (workspace.role?.trim()) {
    parts.push(label(workspace.role, roleSet, translate.role));
  }
  if (workspace.plan?.trim()) {
    parts.push(label(workspace.plan, planSet, translate.plan));
  }
  return parts.join(" · ");
}
