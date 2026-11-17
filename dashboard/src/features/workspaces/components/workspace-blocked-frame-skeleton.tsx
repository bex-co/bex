import type { ReactNode } from "react";
import { ShieldAlert } from "lucide-react";
import { PageScroll } from "@/common/components/page-frame";
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
 * `WorkspaceBlockedFrame`'s pending shape, shown while the workspace's name
 * and the caller's other workspaces are still loading: the same frame, card,
 * heading row and explanation, the requirement's own first region (passed in),
 * then the switch and leave regions. Static copy renders as itself; anything
 * that names the workspace or acts on it is a placeholder, so no action is
 * offered before its data exists.
 */
export function WorkspaceBlockedFrameSkeleton({
  testId,
  description,
  children,
}: {
  testId: string;
  description: string;
  children: ReactNode;
}) {
  const { t } = useTranslations();
  return (
    <PageScroll data-testid={testId} aria-hidden>
      <div className="mx-auto w-full max-w-xl">
        <Card>
          <CardHeader>
            <CardTitle as="h1" className="flex items-center gap-2">
              <ShieldAlert className="size-5 shrink-0 text-destructive" />
              <Skeleton className="h-4 w-3/4" />
            </CardTitle>
            <CardDescription>{description}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            {children}
            <section data-skeleton-region="switch" className="space-y-2">
              <Label>{t("workspaces.loginBlockedSwitchLabel")}</Label>
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
    </PageScroll>
  );
}
