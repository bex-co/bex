import { Link, useRouter } from "@tanstack/react-router";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  WorkspaceBlockedExits,
  WorkspaceBlockedFrame,
} from "@/features/workspaces/components/workspace-blocked-frame";
import { WorkspaceTwoFactorBlockedSkeleton } from "@/features/workspaces/components/workspace-two-factor-blocked-skeleton";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { clearWorkspaceLoginBlocks } from "@/features/workspaces/lib/workspace-login-block";

/**
 * The one page a member sees when bex-api refuses their workspace with
 * `WORKSPACE_2FA_REQUIRED` (w22/m402, docs/ADR012-auth.md 9a): which workspace,
 * why, and the three ways out. Enrollment is Kratos's own settings flow, so
 * the page links to account security rather than re-implementing it; the
 * other two exits (switch workspace, leave this one) are the shared
 * `WorkspaceBlockedExits`.
 *
 * Access returns within the server's 60 s cache of the enrollment fact after
 * the member enables 2FA, so "check again" drops the recorded block and lets
 * the page's own queries decide; if the server still refuses, this page comes
 * straight back.
 */
export function WorkspaceTwoFactorBlockedPage({
  workspaceId,
}: {
  workspaceId: string;
}) {
  const { t } = useTranslations();
  const router = useRouter();
  const { currentWorkspace } = useWorkspace();

  // The name arrives with the workspace list; a block can be recorded first.
  if (!currentWorkspace) return <WorkspaceTwoFactorBlockedSkeleton />;
  const name = currentWorkspace.name;

  function recheck() {
    clearWorkspaceLoginBlocks();
    void router.invalidate();
  }

  return (
    <WorkspaceBlockedFrame
      testId="workspace-two-factor-blocked"
      title={t("workspaces.twoFactorBlockedTitle", { workspace: name })}
      description={t("workspaces.twoFactorBlockedBody")}
    >
      <section data-region="enroll" className="space-y-2">
        <div className="flex flex-wrap items-center gap-2">
          <Button asChild>
            <Link to="/settings" hash="security">
              {t("workspaces.twoFactorEnrollLink")}
            </Link>
          </Button>
          <Button variant="outline" onClick={recheck}>
            {t("workspaces.twoFactorBlockedRecheck")}
          </Button>
        </div>
        <p className="text-xs text-muted-foreground">
          {t("workspaces.twoFactorBlockedRecheckHint")}
        </p>
      </section>
      <WorkspaceBlockedExits workspaceId={workspaceId} name={name} />
    </WorkspaceBlockedFrame>
  );
}
