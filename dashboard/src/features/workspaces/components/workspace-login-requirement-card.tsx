import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import type { WorkspaceLoginRequirement } from "@/common/apollo/auth-error-link";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  LoginRequirementRow,
  OrganizationRequirementNote,
} from "@/features/workspaces/components/login-requirement-row";
import { LoginRequirementRefusalAlert } from "@/features/workspaces/components/login-requirement-refusal-alert";
import { useSetWorkspaceLoginRequirement } from "@/features/workspaces/hooks/use-set-workspace-login-requirement";
import {
  LOGIN_REQUIREMENT_SETTINGS,
  requiringOrganization,
} from "@/features/workspaces/lib/login-requirements";
import {
  proFeaturesPlan,
  type WorkspaceView,
} from "@/features/workspaces/types";

/**
 * Workspace Settings -> Two-factor authentication (w22/m402) and Google login
 * (w7/m465): Render's "Enforce 2FA & Google login" login settings. While a
 * requirement is on, a member who does not meet it (no second factor; a
 * sign-in that was not Google with the Google account of their email) cannot
 * use the workspace's resources or settings, and their API keys and SSH keys
 * are blocked (docs/ADR012-auth.md 9a, 9b). Admins get the switch; every
 * other member sees the state without a control. The card is the same size
 * either way, so a role that loads a moment after the page never shifts it
 * (`WorkspaceLoginRequirementCardSkeleton` previews this anatomy).
 *
 * The enforcement itself is server-side: this switch only writes the setting,
 * and the server's coded 409 refusals (the lock-out guard: the admin does not
 * meet the requirement themselves; or the deployment cannot check it) are
 * shown beside it, the lock-out with the way to fix it.
 *
 * Who is an admin comes from the capability check (`can_manage`), as on the
 * sibling settings cards. `workspace.role` cannot decide it: the workspace
 * list reports "admin" for every membership, so a developer was shown a live
 * switch that could only answer 403 (found by the w22/m402 live check). While
 * the capability check is in flight the control is the skeleton's placeholder.
 *
 * Enforcing either needs the Pro plan or higher, as in Render's plan table
 * (w7/070). Below Pro every member reads why, and an admin's switch cannot
 * turn it on; a workspace that already enforces it keeps it, and its admin
 * can turn it off.
 *
 * An org that requires it in all of its workspaces (w7/m466) holds the switch
 * on and locked, whatever the workspace's own setting, and the card says
 * which org, with the way to its Settings page for the org's Owners. Turning
 * the org's setting off brings the workspace's own back as it was.
 */
export function WorkspaceLoginRequirementCard({
  requirement,
  workspace,
  onChangePlan,
}: {
  requirement: WorkspaceLoginRequirement;
  workspace: WorkspaceView;
  /** Opens the change-plan dialog on this page (`?plan=change`). */
  onChangePlan: () => void;
}) {
  const { t } = useTranslations();
  const setting = LOGIN_REQUIREMENT_SETTINGS[requirement];
  const { setRequired, busy, error } = useSetWorkspaceLoginRequirement(
    requirement,
    workspace,
  );
  const { canManage: isAdmin, loading: roleLoading } = useCapabilities();
  const lockedBy = requiringOrganization(requirement, workspace.organization);
  const belowPro = !proFeaturesPlan(workspace.plan);
  const required = lockedBy !== null || workspace[setting.field];

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t(setting.titleKey)}</CardTitle>
        <CardDescription>
          {`${t(setting.descriptionKey)} ${t("workspaces.loginRequirementAdminOnly")}`}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <LoginRequirementRow
          id={`workspace-${setting.section}`}
          region={setting.section}
          label={t(setting.switchLabelKey)}
          control={isAdmin ? "switch" : roleLoading ? "pending" : "status"}
          required={required}
          disabled={busy || lockedBy !== null || (belowPro && !required)}
          onCheckedChange={(next) => void setRequired(next)}
        />
        {lockedBy ? (
          <OrganizationRequirementNote organization={lockedBy} />
        ) : null}
        {belowPro ? (
          <p className="text-muted-foreground text-xs">
            {t(setting.proPlanOnlyKey)}
            {isAdmin ? (
              <>
                {" "}
                <Button
                  type="button"
                  variant="link"
                  className="h-auto p-0 text-xs"
                  onClick={onChangePlan}
                >
                  {t("workspaces.changePlanTrigger")}
                </Button>
              </>
            ) : null}
          </p>
        ) : null}
        {error ? (
          <LoginRequirementRefusalAlert
            requirement={requirement}
            title={t(setting.errorTitleKey)}
            error={error}
          />
        ) : null}
      </CardContent>
    </Card>
  );
}
