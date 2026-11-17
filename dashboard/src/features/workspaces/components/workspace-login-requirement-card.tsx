import { Link } from "@tanstack/react-router";
import { Loader2 } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Label } from "@/common/components/ui/label";
import { Skeleton } from "@/common/components/ui/skeleton";
import { Switch } from "@/common/components/ui/switch";
import type { WorkspaceLoginRequirement } from "@/common/apollo/auth-error-link";
import { useTranslations } from "@/common/hooks/use-translations";
import { useGoogleSignIn } from "@/features/auth/hooks/use-google-sign-in";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useSetWorkspaceLoginRequirement } from "@/features/workspaces/hooks/use-set-workspace-login-requirement";
import { LOGIN_REQUIREMENT_SETTINGS } from "@/features/workspaces/lib/login-requirements";
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
  const belowPro = !proFeaturesPlan(workspace.plan);
  const required = setting.required(workspace);
  const controlId = `workspace-${setting.section}`;

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t(setting.titleKey)}</CardTitle>
        <CardDescription>
          {`${t(setting.descriptionKey)} ${t("workspaces.loginRequirementAdminOnly")}`}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex min-h-6 flex-wrap items-center justify-between gap-3">
          <Label htmlFor={isAdmin ? controlId : undefined}>
            {t(setting.switchLabelKey)}
          </Label>
          {isAdmin ? (
            <Switch
              id={controlId}
              checked={required}
              disabled={busy || (belowPro && !required)}
              onCheckedChange={(next) => void setRequired(next)}
            />
          ) : roleLoading ? (
            <Skeleton
              data-skeleton-region={`${setting.section}-control`}
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
          <Alert variant="destructive">
            <AlertTitle>{t(setting.errorTitleKey)}</AlertTitle>
            <AlertDescription>
              <p>{error.text}</p>
              {error.accountFix ? (
                <AccountFix requirement={requirement} />
              ) : null}
            </AlertDescription>
          </Alert>
        ) : null}
      </CardContent>
    </Card>
  );
}

/**
 * The lock-out guard's way out, on the admin's own account: enroll a second
 * factor in account security, or sign this session in with Google and come
 * back to this page.
 */
function AccountFix({
  requirement,
}: {
  requirement: WorkspaceLoginRequirement;
}) {
  const { t } = useTranslations();
  if (requirement === "googleLogin") return <SignInWithGoogleLink />;
  return (
    <Link to="/settings" hash="security" className="underline">
      {t("workspaces.twoFactorEnrollLink")}
    </Link>
  );
}

function SignInWithGoogleLink() {
  const { t } = useTranslations();
  const { start, busy, failed } = useGoogleSignIn();
  return (
    <>
      <Button
        type="button"
        variant="link"
        className="h-auto p-0 text-destructive underline"
        disabled={busy}
        onClick={() => void start()}
      >
        {busy ? <Loader2 className="animate-spin" /> : null}
        {t("workspaces.googleLoginSignIn")}
      </Button>
      {failed ? <p>{t("workspaces.googleLoginSignInError")}</p> : null}
    </>
  );
}
