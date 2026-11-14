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
 * WorkspaceTwoFactorCard's pending shape: its title, description and switch
 * label are static copy, so they render as themselves; only the control, which
 * depends on the caller's role and the workspace's setting, is a placeholder.
 * The placeholder is the switch's own box (h-5 w-9), and the row keeps the
 * card's min-h-6, so the swap shifts nothing. No control is enabled before its
 * deciding data exists.
 */
export function WorkspaceTwoFactorCardSkeleton() {
  const { t } = useTranslations();
  return (
    <Card data-testid="workspace-two-factor-skeleton" aria-hidden>
      <CardHeader>
        <CardTitle>{t("workspaces.twoFactorTitle")}</CardTitle>
        <CardDescription>
          {`${t("workspaces.twoFactorDescription")} ${t("workspaces.twoFactorAdminOnly")}`}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex min-h-6 flex-wrap items-center justify-between gap-3">
          <Label>{t("workspaces.twoFactorSwitchLabel")}</Label>
          <Skeleton
            data-skeleton-region="two-factor-control"
            className="h-5 w-9 shrink-0 rounded-full"
          />
        </div>
      </CardContent>
    </Card>
  );
}
