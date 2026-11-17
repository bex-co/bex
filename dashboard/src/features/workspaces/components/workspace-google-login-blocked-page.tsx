import { Loader2, LogIn } from "lucide-react";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { useGoogleSignIn } from "@/features/auth/hooks/use-google-sign-in";
import {
  WorkspaceBlockedExits,
  WorkspaceBlockedFrame,
} from "@/features/workspaces/components/workspace-blocked-frame";
import { WorkspaceGoogleLoginBlockedSkeleton } from "@/features/workspaces/components/workspace-google-login-blocked-skeleton";
import { useWorkspace } from "@/features/workspaces/context/hooks";

/**
 * The one page a member sees when bex-api refuses their workspace with
 * `WORKSPACE_GOOGLE_LOGIN_REQUIRED` (w7/m465, docs/ADR012-auth.md 9b), the 2FA
 * blocked page's sibling: which workspace, why, and the three ways out.
 *
 * Render checks how the member signed in, not whether a Google account is
 * linked, so the way back in is signing this session in with Google, with the
 * Google account of their bex email. "Sign in with Google" does that through
 * Kratos and comes back to this very URL with a fresh page, whose queries then
 * decide again (`startGoogleSignIn`); there is nothing to re-check by hand.
 * The other two exits (switch workspace, leave this one) are the shared
 * `WorkspaceBlockedExits`.
 */
export function WorkspaceGoogleLoginBlockedPage({
  workspaceId,
}: {
  workspaceId: string;
}) {
  const { t } = useTranslations();
  const { currentWorkspace } = useWorkspace();
  const { start, busy, failed } = useGoogleSignIn();

  // The name arrives with the workspace list; a block can be recorded first.
  if (!currentWorkspace) return <WorkspaceGoogleLoginBlockedSkeleton />;
  const name = currentWorkspace.name;

  return (
    <WorkspaceBlockedFrame
      testId="workspace-google-login-blocked"
      title={t("workspaces.googleLoginBlockedTitle", { workspace: name })}
      description={t("workspaces.googleLoginBlockedBody")}
    >
      <section data-region="sign-in" className="space-y-2">
        {failed ? (
          <Alert variant="destructive">
            <AlertDescription>
              {t("workspaces.googleLoginSignInError")}
            </AlertDescription>
          </Alert>
        ) : null}
        <div className="flex flex-wrap items-center gap-2">
          <Button disabled={busy} onClick={() => void start()}>
            {busy ? <Loader2 className="animate-spin" /> : <LogIn />}
            {t("workspaces.googleLoginSignIn")}
          </Button>
        </div>
        <p className="text-xs text-muted-foreground">
          {t("workspaces.googleLoginBlockedHint")}
        </p>
      </section>
      <WorkspaceBlockedExits workspaceId={workspaceId} name={name} />
    </WorkspaceBlockedFrame>
  );
}
