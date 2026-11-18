import { Link, useRouter } from "@tanstack/react-router";
import { Button } from "@/common/components/ui/button";
import type { LoginRefusalOrganization } from "@/common/apollo/auth-error-link";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  WorkspaceBlockedExits,
  WorkspaceBlockedFrame,
} from "@/features/workspaces/components/workspace-blocked-frame";
import { WorkspaceTwoFactorBlockedSkeleton } from "@/features/workspaces/components/workspace-two-factor-blocked-skeleton";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { clearLoginBlocks } from "@/features/workspaces/lib/workspace-login-block";
import { loginBlockedCopy } from "@/features/workspaces/lib/login-requirements";

/**
 * Two-factor authentication's way back in, on every page it blocks (a
 * workspace's, an org's): enroll in account security, then check again.
 * Enrollment is Kratos's own settings flow, so this links to account security
 * rather than re-implementing it.
 *
 * Access returns within the server's 60 s cache of the enrollment fact, so
 * `onRecheck` drops the recorded block and lets the page's own queries
 * decide; if the server still refuses, the block comes straight back.
 */
export function TwoFactorEnrollSection({
  onRecheck,
}: {
  onRecheck: () => void;
}) {
  const { t } = useTranslations();
  return (
    <section data-region="enroll" className="space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        <Button asChild>
          <Link to="/settings" hash="security">
            {t("workspaces.twoFactorEnrollLink")}
          </Link>
        </Button>
        <Button variant="outline" onClick={onRecheck}>
          {t("workspaces.twoFactorBlockedRecheck")}
        </Button>
      </div>
      <p className="text-xs text-muted-foreground">
        {t("workspaces.twoFactorBlockedRecheckHint")}
      </p>
    </section>
  );
}

/**
 * The one page a member sees when bex-api refuses their workspace with
 * `WORKSPACE_2FA_REQUIRED` (w22/m402, docs/ADR012-auth.md 9a): which workspace,
 * why, and the three ways out: enroll (`TwoFactorEnrollSection`), or the
 * shared `WorkspaceBlockedExits` (switch workspace, leave this one). When the
 * workspace's org is what requires it (`organization`, w7/m466, 9c), the page
 * names the org instead: every one of its workspaces requires it.
 */
export function WorkspaceTwoFactorBlockedPage({
  workspaceId,
  organization = null,
}: {
  workspaceId: string;
  organization?: LoginRefusalOrganization | null;
}) {
  const { t } = useTranslations();
  const router = useRouter();
  const { currentWorkspace } = useWorkspace();

  // The name arrives with the workspace list; a block can be recorded first.
  if (!currentWorkspace) {
    return <WorkspaceTwoFactorBlockedSkeleton organization={organization} />;
  }
  const name = currentWorkspace.name;

  function recheck() {
    clearLoginBlocks();
    void router.invalidate();
  }

  return (
    <WorkspaceBlockedFrame
      testId="workspace-two-factor-blocked"
      {...loginBlockedCopy(t, "twoFactor", organization, name)}
    >
      <TwoFactorEnrollSection onRecheck={recheck} />
      <WorkspaceBlockedExits workspaceId={workspaceId} name={name} />
    </WorkspaceBlockedFrame>
  );
}
