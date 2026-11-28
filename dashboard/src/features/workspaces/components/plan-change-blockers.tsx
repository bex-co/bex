import type { ReactNode } from "react";
import { ExternalLink, TriangleAlert } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  useTranslations,
  type Translate,
} from "@/common/hooks/use-translations";
import { conjunctionList } from "@/common/lib/format/language";
import { ROLES } from "@/features/team/types";
import type { PlanChangeBlocker } from "@/features/workspaces/hooks/use-plan-change-blockers";

/** Render's "Workspace limits" docs link, as bex's members page. */
const WORKSPACE_LIMITS_DOCS_URL =
  "https://bex.co/docs/platform/members#roles-and-plan-limits";

/** The drawn kinds whose items are people, named "email:role". */
const PEOPLE_KINDS = new Set(["member_limit", "roles"]);

/** The order Render draws its plan rules in: the workspace caps, then the
 *  organization's, the seats, the roles, and bex's inbound IP rules last. */
const KIND_ORDER = [
  "workspace_limit",
  "service_limit",
  "secondary_workspaces",
  "member_limit",
  "roles",
  "inbound_ip_rules",
];

/** A role id as its Render label ("viewer" → "Viewer"). */
function roleLabel(role: string, t: Translate): string {
  const known = ROLES.find((r) => r === role.toUpperCase());
  return known ? t(`team.role.${known}`) : role;
}

/** An "email:role" pair (a member's sign-in id when no email resolves) as
 *  "who (Role)". */
function personWithRole(item: string, t: Translate): string {
  const at = item.lastIndexOf(":");
  if (at <= 0) return item;
  return t("workspaces.orgDowngradePersonRole", {
    person: item.slice(0, at),
    role: roleLabel(item.slice(at + 1), t),
  });
}

/** "workspace", "environment:<name>" or "service:<name>" in words. */
function ipRuleHolder(item: string, t: Translate): string {
  if (item === "workspace") return t("workspaces.orgDowngradeIpRuleWorkspace");
  const at = item.indexOf(":");
  const kind = item.slice(0, at);
  const name = item.slice(at + 1);
  if (kind === "environment") {
    return t("workspaces.orgDowngradeIpRuleEnvironment", { name });
  }
  if (kind === "service") {
    return t("workspaces.orgDowngradeIpRuleService", { name });
  }
  return item;
}

/** Merges bex-api's member_roles and invite_roles into Render's one roles
 *  blocker and puts every blocker in Render's order. */
function drawnBlockers(
  blockers: readonly PlanChangeBlocker[],
): PlanChangeBlocker[] {
  const drawn: PlanChangeBlocker[] = [];
  for (const blocker of blockers) {
    const kind =
      blocker.kind === "member_roles" || blocker.kind === "invite_roles"
        ? "roles"
        : blocker.kind;
    const same = drawn.find((d) => d.kind === kind);
    if (same) same.items = [...same.items, ...blocker.items];
    else drawn.push({ kind, items: [...blocker.items], limit: blocker.limit });
  }
  const rank = (kind: string) => {
    const at = KIND_ORDER.indexOf(kind);
    return at < 0 ? KIND_ORDER.length : at;
  };
  return drawn.sort((a, b) => rank(a.kind) - rank(b.kind));
}

export interface PlanChangeBlockersProps {
  blockers: readonly PlanChangeBlocker[];
  /** The selected plan's name ("Hobby"), as Render's copy names it. */
  planName: string;
  /** The org whose workspaces "Update workspaces" opens. */
  organizationId: string | null;
}

/**
 * Render's plan rules on its plan page (w3/m708, BillingUpdatePlan): one
 * warning per unmet rule with its title, what the plan allows, and a link that
 * opens where to fix it in a new tab, as Render's does, so the choice stays
 * here. bex names who and what is in the way, from bex-api's own refusal; the
 * inbound IP rules blocker is bex's (Render warns of it only on its
 * confirmation), worded with Render's words for it.
 */
export function PlanChangeBlockers({
  blockers,
  planName,
  organizationId,
}: PlanChangeBlockersProps) {
  const { t, i18n } = useTranslations();
  if (blockers.length === 0) return null;
  const list = conjunctionList(i18n.language);
  return (
    <div className="space-y-3" data-testid="plan-change-blockers">
      {drawnBlockers(blockers).map((blocker) => (
        <BlockerNotice
          key={blocker.kind}
          {...blockerCopy(blocker, planName, organizationId, t)}
          items={
            blocker.items.length === 0
              ? null
              : list.format(
                  blocker.items.map((item) =>
                    PEOPLE_KINDS.has(blocker.kind)
                      ? personWithRole(item, t)
                      : blocker.kind === "inbound_ip_rules"
                        ? ipRuleHolder(item, t)
                        : item,
                  ),
                )
          }
        />
      ))}
    </div>
  );
}

interface BlockerCopy {
  title: string;
  message: ReactNode;
  action: { label: string; href: string } | null;
}

/** Render's title, message and fix link for one rule. */
function blockerCopy(
  blocker: PlanChangeBlocker,
  plan: string,
  organizationId: string | null,
  t: Translate,
): BlockerCopy {
  const teamMembers = {
    label: t("workspaces.planBlockerUpdateTeamMembers"),
    href: "/workspace/settings#team",
  };
  const limit = blocker.limit ?? 0;
  switch (blocker.kind) {
    case "member_limit":
      return {
        title: t("workspaces.planBlockerMemberLimitTitle"),
        message:
          limit === 1
            ? t("workspaces.planBlockerMemberLimitOne", { plan })
            : t("workspaces.planBlockerMemberLimitMany", { plan, limit }),
        action: teamMembers,
      };
    case "service_limit":
      return {
        title: t("workspaces.planBlockerServiceLimitTitle"),
        message: t("workspaces.planBlockerServiceLimit", { plan, limit }),
        action: {
          label: t("workspaces.planBlockerUpdateServices"),
          href: "/",
        },
      };
    case "workspace_limit":
      return {
        title: t("workspaces.planBlockerWorkspaceLimitTitle"),
        message: t("workspaces.planBlockerWorkspaceLimit", { limit }),
        action: {
          label: t("workspaces.planBlockerWorkspaceLimitsDocs"),
          href: WORKSPACE_LIMITS_DOCS_URL,
        },
      };
    case "secondary_workspaces":
      return {
        title: t("workspaces.planBlockerWorkspacesTitle"),
        message: t("workspaces.planBlockerWorkspaces", { plan }),
        action: organizationId
          ? {
              label: t("workspaces.planBlockerUpdateWorkspaces"),
              href: `/org/${organizationId}/workspaces`,
            }
          : null,
      };
    case "roles": {
      const roles = [
        ...new Set(
          blocker.items.map((item) =>
            roleLabel(item.slice(item.lastIndexOf(":") + 1), t),
          ),
        ),
      ].sort();
      return {
        title: t("workspaces.planBlockerRolesTitle"),
        message: t(
          roles.length > 1
            ? "workspaces.planBlockerRolesMany"
            : "workspaces.planBlockerRolesOne",
          { plan, roles: roles.join(", ") },
        ),
        action: teamMembers,
      };
    }
    case "inbound_ip_rules":
      return {
        title: t("workspaces.planBlockerIpRulesTitle"),
        message: t("workspaces.planBlockerIpRules"),
        action: {
          label: t("workspaces.planBlockerUpdateIpRules"),
          href: "/workspace/settings#inbound-ip-rules",
        },
      };
    default:
      return {
        title: t("workspaces.planBlockerOtherTitle"),
        message: t("workspaces.planBlockerOther", { plan }),
        action: null,
      };
  }
}

function BlockerNotice({
  title,
  message,
  items,
  action,
}: BlockerCopy & { items: string | null }) {
  return (
    <div
      role="alert"
      className="flex items-start gap-3 rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-sm"
    >
      <TriangleAlert
        aria-hidden="true"
        className="mt-0.5 size-4 shrink-0 text-amber-600 dark:text-amber-400"
      />
      {/* The fix link sits under the words on a phone and beside them on a
          wide page, as Render's inline notice puts its action. */}
      <div className="flex min-w-0 flex-1 flex-col gap-2 @2xl:flex-row @2xl:items-start @2xl:justify-between @2xl:gap-4">
        <div className="min-w-0 space-y-1">
          <p className="font-medium">{title}</p>
          <p>{message}</p>
          {items ? (
            <p className="text-muted-foreground break-words">{items}</p>
          ) : null}
        </div>
        {action ? (
          <Button
            asChild
            size="sm"
            variant="outline"
            className="shrink-0 self-start"
          >
            <a href={action.href} target="_blank" rel="noreferrer">
              {action.label}
              <ExternalLink aria-hidden="true" />
            </a>
          </Button>
        ) : null}
      </div>
    </div>
  );
}
