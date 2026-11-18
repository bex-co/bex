import { Loader2, LogIn } from "lucide-react";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import type { LoginRefusalOrganization } from "@/common/apollo/auth-error-link";
import { useTranslations } from "@/common/hooks/use-translations";
import { useGoogleSignIn } from "@/features/auth/hooks/use-google-sign-in";
import {
  WorkspaceBlockedExits,
  WorkspaceBlockedFrame,
} from "@/features/workspaces/components/workspace-blocked-frame";
import { WorkspaceGoogleLoginBlockedSkeleton } from "@/features/workspaces/components/workspace-google-login-blocked-skeleton";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { loginBlockedCopy } from "@/features/workspaces/lib/login-requirements";

/**
 * Google login's way back in, on every page it blocks (a workspace's, an
 * org's). Render checks how the member signed in, not whether a Google
 * account is linked, so the way back in is signing this session in with
 * Google, with the Google account of their bex email. "Sign in with Google"
 * does that through Kratos and comes back to this very URL with a fresh page,
 * whose queries then decide again (`startGoogleSignIn`); there is nothing to
 * re-check by hand.
 */
export function GoogleSignInSection() {
  const { t } = useTranslations();
  const { start, busy, failed } = useGoogleSignIn();
  return (
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
  );
}

/**
 * The one page a member sees when bex-api refuses their workspace with
 * `WORKSPACE_GOOGLE_LOGIN_REQUIRED` (w7/m465, docs/ADR012-auth.md 9b), the 2FA
 * blocked page's sibling: which workspace, why, and the three ways out: sign
 * in with Google (`GoogleSignInSection`), or the shared
 * `WorkspaceBlockedExits` (switch workspace, leave this one). When the
 * workspace's org is what requires it (`organization`, w7/m466, 9c), the page
 * names the org instead.
 */
export function WorkspaceGoogleLoginBlockedPage({
  workspaceId,
  organization = null,
}: {
  workspaceId: string;
  organization?: LoginRefusalOrganization | null;
}) {
  const { t } = useTranslations();
  const { currentWorkspace } = useWorkspace();

  // The name arrives with the workspace list; a block can be recorded first.
  if (!currentWorkspace) {
    return <WorkspaceGoogleLoginBlockedSkeleton organization={organization} />;
  }
  const name = currentWorkspace.name;

  return (
    <WorkspaceBlockedFrame
      testId="workspace-google-login-blocked"
      {...loginBlockedCopy(t, "googleLogin", organization, name)}
    >
      <GoogleSignInSection />
      <WorkspaceBlockedExits workspaceId={workspaceId} name={name} />
    </WorkspaceBlockedFrame>
  );
}
