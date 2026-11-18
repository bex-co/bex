import { Skeleton } from "@/common/components/ui/skeleton";
import type { LoginRefusalOrganization } from "@/common/apollo/auth-error-link";
import { useTranslations } from "@/common/hooks/use-translations";
import { loginBlockedCopy } from "@/features/workspaces/lib/login-requirements";
import { WorkspaceBlockedFrameSkeleton } from "@/features/workspaces/components/workspace-blocked-frame-skeleton";

/**
 * WorkspaceGoogleLoginBlockedPage's pending shape: the shared blocked frame
 * with the sign-in region's button as a placeholder (h-9, the default button
 * height) and its static hint as itself. The explanation is the page's own:
 * the org's, when its setting is the one that refused (w7/m466).
 */
export function WorkspaceGoogleLoginBlockedSkeleton({
  organization = null,
}: {
  organization?: LoginRefusalOrganization | null;
}) {
  const { t } = useTranslations();
  return (
    <WorkspaceBlockedFrameSkeleton
      testId="workspace-google-login-blocked-skeleton"
      description={loginBlockedCopy(t, "googleLogin", organization).description}
    >
      <section data-skeleton-region="sign-in" className="space-y-2">
        <div className="flex flex-wrap items-center gap-2">
          <Skeleton className="h-9 w-48 max-w-full" />
        </div>
        <p className="text-xs text-muted-foreground">
          {t("workspaces.googleLoginBlockedHint")}
        </p>
      </section>
    </WorkspaceBlockedFrameSkeleton>
  );
}
