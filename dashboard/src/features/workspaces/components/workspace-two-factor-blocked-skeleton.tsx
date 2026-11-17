import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { WorkspaceBlockedFrameSkeleton } from "@/features/workspaces/components/workspace-blocked-frame-skeleton";

/**
 * WorkspaceTwoFactorBlockedPage's pending shape: the shared blocked frame with
 * the enroll region's two buttons as placeholders and its static hint as
 * itself.
 */
export function WorkspaceTwoFactorBlockedSkeleton() {
  const { t } = useTranslations();
  return (
    <WorkspaceBlockedFrameSkeleton
      testId="workspace-two-factor-blocked-skeleton"
      description={t("workspaces.twoFactorBlockedBody")}
    >
      <section data-skeleton-region="enroll" className="space-y-2">
        <div className="flex flex-wrap items-center gap-2">
          <Skeleton className="h-9 w-60 max-w-full" />
          <Skeleton className="h-9 w-48 max-w-full" />
        </div>
        <p className="text-xs text-muted-foreground">
          {t("workspaces.twoFactorBlockedRecheckHint")}
        </p>
      </section>
    </WorkspaceBlockedFrameSkeleton>
  );
}
