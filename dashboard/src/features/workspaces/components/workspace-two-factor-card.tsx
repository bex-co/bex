import { Link } from "@tanstack/react-router";
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
import { Label } from "@/common/components/ui/label";
import { Switch } from "@/common/components/ui/switch";
import { useTranslations } from "@/common/hooks/use-translations";
import { useSetWorkspaceRequireTwoFactor } from "@/features/workspaces/hooks/use-set-workspace-require-two-factor";
import type { WorkspaceView } from "@/features/workspaces/types";

/**
 * Workspace Settings -> Two-factor authentication (w22/m402), Render's
 * "Enforce two-factor authentication (2FA)" login setting: while it is on, a
 * member with no second factor cannot use the workspace's resources or
 * settings, and their API keys and SSH keys are blocked, until they enroll
 * (docs/ADR012-auth.md 9a). Admins get the switch; every other member sees the
 * state without a control. The card is the same size either way, so a role
 * that loads a moment after the page never shifts it
 * (`WorkspaceTwoFactorCardSkeleton` previews this anatomy).
 *
 * The enforcement itself is server-side: this switch only writes the setting,
 * and the server's coded 409 refusals (the admin has no second factor, or the
 * deployment cannot look factors up) are shown beside it.
 */
export function WorkspaceTwoFactorCard({
  workspace,
}: {
  workspace: WorkspaceView;
}) {
  const { t } = useTranslations();
  const { setRequired, busy, error } =
    useSetWorkspaceRequireTwoFactor(workspace);
  const isAdmin = workspace.role === "admin";

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("workspaces.twoFactorTitle")}</CardTitle>
        <CardDescription>
          {`${t("workspaces.twoFactorDescription")} ${t("workspaces.twoFactorAdminOnly")}`}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex min-h-6 flex-wrap items-center justify-between gap-3">
          <Label htmlFor={isAdmin ? "workspace-two-factor" : undefined}>
            {t("workspaces.twoFactorSwitchLabel")}
          </Label>
          {isAdmin ? (
            <Switch
              id="workspace-two-factor"
              checked={workspace.requireTwoFactor}
              disabled={busy}
              onCheckedChange={(next) => void setRequired(next)}
            />
          ) : (
            <Badge
              variant={workspace.requireTwoFactor ? "secondary" : "outline"}
            >
              {t(
                workspace.requireTwoFactor
                  ? "workspaces.twoFactorStatusRequired"
                  : "workspaces.twoFactorStatusNotRequired",
              )}
            </Badge>
          )}
        </div>
        {error ? (
          <Alert variant="destructive">
            <AlertTitle>{t("workspaces.twoFactorErrorTitle")}</AlertTitle>
            <AlertDescription>
              <p>{error.text}</p>
              {error.needsEnrollment ? (
                <Link to="/settings" hash="security" className="underline">
                  {t("workspaces.twoFactorEnrollLink")}
                </Link>
              ) : null}
            </AlertDescription>
          </Alert>
        ) : null}
      </CardContent>
    </Card>
  );
}
