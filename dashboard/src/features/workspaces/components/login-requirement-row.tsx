import { Link } from "@tanstack/react-router";
import { Badge } from "@/common/components/ui/badge";
import { Label } from "@/common/components/ui/label";
import { Skeleton } from "@/common/components/ui/skeleton";
import { Switch } from "@/common/components/ui/switch";
import { ReservedText } from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { useOrganizationRole } from "@/features/organizations/hooks/use-organization-role";
import type { WorkspaceOrganizationRef } from "@/features/workspaces/types";

/**
 * How a login-requirement row shows its state: the switch, for whoever may
 * change it; the state as a badge, for everyone else; the switch's own box
 * while who may is still unknown (a skeleton, or a role still loading).
 */
export type LoginRequirementControl = "switch" | "status" | "pending";

/**
 * The row every login-requirement setting draws: Workspace Settings' 2FA and
 * Google login cards (w22/m402, w7/m465) and each setting of Org Settings ->
 * Security (w7/m466). Its label, then the control. The row keeps `min-h-6`
 * whichever control it holds, and the placeholder is the switch's own box
 * (h-5 w-9), so a role that loads after the page shifts nothing at any width.
 * No control is enabled before its deciding data exists.
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
      ) : (
        <Badge variant={required ? "secondary" : "outline"}>
          {t(
            required
              ? "workspaces.loginRequirementStatusRequired"
              : "workspaces.loginRequirementStatusNotRequired",
          )}
        </Badge>
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
