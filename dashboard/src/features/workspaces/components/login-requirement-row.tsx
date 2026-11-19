import { Link } from "@tanstack/react-router";
import { Badge } from "@/common/components/ui/badge";
import { Label } from "@/common/components/ui/label";
import { Skeleton } from "@/common/components/ui/skeleton";
import { Switch } from "@/common/components/ui/switch";
import {
  ReservedBadge,
  ReservedText,
} from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { useOrganizationRole } from "@/features/organizations/hooks/use-organization-role";
import type { WorkspaceOrganizationRef } from "@/features/workspaces/types";

/**
 * How a login-requirement row shows its state: the switch, for whoever may
 * change it; the state as a badge, for everyone else. While it loads, the
 * row holds the box of the one it will draw: the badge's (`pendingStatus`)
 * once the caller is known not to change it, else the switch's (`pending`:
 * who may is still unknown, or is known to be someone who may).
 */
export type LoginRequirementControl =
  | "switch"
  | "status"
  | "pending"
  | "pendingStatus";

/**
 * The row every login-requirement setting draws: Workspace Settings' 2FA and
 * Google login cards (w22/m402, w7/m465) and each setting of Org Settings ->
 * Security (w7/m466). Its label, then the control. The row keeps `min-h-6`
 * whichever control it holds, and wraps the control under the label when
 * both do not fit. A badge is wider than the switch, so it wraps at widths
 * where the switch does not (w7/089): a skeleton that knows the caller reads
 * the state holds the badge's own box (`pendingStatus`, with `required`), and
 * one that does not holds the switch's (h-5 w-9). No control is enabled
 * before its deciding data exists.
 */
export function LoginRequirementRow({
  id,
  region,
  label,
  control,
  required = false,
  disabled = false,
  onCheckedChange,
}: {
  /** The switch's id, which the label names. */
  id: string;
  /** The placeholder's `data-skeleton-region` stem. */
  region: string;
  label: string;
  control: LoginRequirementControl;
  required?: boolean;
  disabled?: boolean;
  onCheckedChange?: (next: boolean) => void;
}) {
  const { t } = useTranslations();
  const badgeVariant = required ? "secondary" : "outline";
  const status = t(
    required
      ? "workspaces.loginRequirementStatusRequired"
      : "workspaces.loginRequirementStatusNotRequired",
  );
  return (
    <div className="flex min-h-6 flex-wrap items-center justify-between gap-3">
      <Label htmlFor={control === "switch" ? id : undefined}>{label}</Label>
      {control === "switch" ? (
        <Switch
          id={id}
          checked={required}
          disabled={disabled}
          onCheckedChange={onCheckedChange}
        />
      ) : control === "pending" ? (
        <Skeleton
          data-skeleton-region={`${region}-control`}
          className="h-5 w-9 shrink-0 rounded-full"
        />
      ) : control === "pendingStatus" ? (
        <ReservedBadge variant={badgeVariant} region={`${region}-control`}>
          {status}
        </ReservedBadge>
      ) : (
        <Badge variant={badgeVariant}>{status}</Badge>
      )}
    </div>
  );
}

/** `relative` in both states, so the skeleton's bar has the ready box. */
const NOTE_CLASS = "relative text-muted-foreground text-xs";

/**
 * Why a workspace's own login-requirement switch is on and locked: its org
 * requires it in all of its workspaces (w7/m466, docs/ADR012-auth.md 9c). An
 * Owner of the org (as the switcher's org list reports the caller's role)
 * gets the way to the setting that holds it, the org's Settings page; nobody
 * else can change it there. `pending` reserves the note's footprint in a
 * skeleton without showing the org's name.
 */
export function OrganizationRequirementNote({
  organization,
  pending = false,
}: {
  organization: Pick<WorkspaceOrganizationRef, "id" | "name">;
  pending?: boolean;
}) {
  const { t } = useTranslations();
  const owner = useOrganizationRole(organization.id) === "OWNER";
  const settings = t("workspaces.loginRequirementOrganizationSettingsLink");
  const content = (
    <>
      {t("workspaces.loginRequirementRequiredByOrganization", {
        organization: organization.name,
      })}
      {owner ? (
        <>
          {" "}
          {pending ? (
            <span className="underline">{settings}</span>
          ) : (
            <Link
              to="/org/$orgId/settings"
              params={{ orgId: organization.id }}
              className="underline"
            >
              {settings}
            </Link>
          )}
        </>
      ) : null}
    </>
  );
  return (
    <p
      className={NOTE_CLASS}
      data-skeleton-region={
        pending ? "organization-requirement-note" : undefined
      }
      data-testid={pending ? undefined : "organization-requirement-note"}
    >
      {pending ? <ReservedText>{content}</ReservedText> : content}
    </p>
  );
}
