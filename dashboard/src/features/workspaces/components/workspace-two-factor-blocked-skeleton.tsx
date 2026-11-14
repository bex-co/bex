import { ShieldAlert } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Label } from "@/common/components/ui/label";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * WorkspaceTwoFactorBlockedPage's pending shape, shown while the workspace's
 * name and the caller's other workspaces are still loading: the same frame,
 * card, heading row, explanation, and three action regions (enroll, switch,
 * leave). Static copy renders as itself; anything that names the workspace or
 * acts on it is a placeholder, so no action is offered before its data exists.
 */
export function WorkspaceTwoFactorBlockedSkeleton() {
  const { t } = useTranslations();
  return (
    <div
      className="flex-1 overflow-auto p-4 sm:p-6"
      data-testid="workspace-two-factor-blocked-skeleton"
      aria-hidden
    >
      <div className="mx-auto w-full max-w-xl">
        <Card>
          <CardHeader>
            <CardTitle as="h1" className="flex items-center gap-2">
              <ShieldAlert className="size-5 shrink-0 text-destructive" />
              <Skeleton className="h-4 w-3/4" />
            </CardTitle>
            <CardDescription>
              {t("workspaces.twoFactorBlockedBody")}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            <section data-skeleton-region="enroll" className="space-y-2">
              <div className="flex flex-wrap items-center gap-2">
                <Skeleton className="h-9 w-60 max-w-full" />
                <Skeleton className="h-9 w-48 max-w-full" />
              </div>
              <p className="text-xs text-muted-foreground">
                {t("workspaces.twoFactorBlockedRecheckHint")}
              </p>
            </section>
            <section data-skeleton-region="switch" className="space-y-2">
              <Label>{t("workspaces.twoFactorBlockedSwitchLabel")}</Label>
              <Skeleton className="h-9 w-full sm:w-72" />
            </section>
            <section data-skeleton-region="leave" className="space-y-3">
              <div className="space-y-1">
                <Skeleton className="h-4 w-full" />
                <Skeleton className="h-4 w-full" />
                <Skeleton className="h-4 w-2/3" />
              </div>
              <Skeleton className="h-9 w-44" />
            </section>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
