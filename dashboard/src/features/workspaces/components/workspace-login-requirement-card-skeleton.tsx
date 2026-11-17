import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Label } from "@/common/components/ui/label";
import { Skeleton } from "@/common/components/ui/skeleton";
import type { WorkspaceLoginRequirement } from "@/common/apollo/auth-error-link";
import { useTranslations } from "@/common/hooks/use-translations";
import { LOGIN_REQUIREMENT_SETTINGS } from "@/features/workspaces/lib/login-requirements";

/**
 * WorkspaceLoginRequirementCard's pending shape, for either requirement: its
 * title, description and switch label are static copy, so they render as
 * themselves; only the control, which depends on the caller's role and the
 * workspace's setting, is a placeholder. The placeholder is the switch's own
 * box (h-5 w-9), and the row keeps the card's min-h-6, so the swap shifts
 * nothing at any width (the row wraps the same way in both). No control is
 * enabled before its deciding data exists.
 *
 * A workspace already known to be below Pro (`belowPro`) settles to the card
 * with its plan notice, so the preview carries the notice too: static copy,
 * without the admin's "Change plan" link, which waits for the role.
 */
export function WorkspaceLoginRequirementCardSkeleton({
  requirement,
  belowPro = false,
}: {
  requirement: WorkspaceLoginRequirement;
  belowPro?: boolean;
}) {
  const { t } = useTranslations();
  const setting = LOGIN_REQUIREMENT_SETTINGS[requirement];
  return (
    <Card data-testid={`workspace-${setting.section}-skeleton`} aria-hidden>
      <CardHeader>
        <CardTitle>{t(setting.titleKey)}</CardTitle>
        <CardDescription>
          {`${t(setting.descriptionKey)} ${t("workspaces.loginRequirementAdminOnly")}`}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex min-h-6 flex-wrap items-center justify-between gap-3">
          <Label>{t(setting.switchLabelKey)}</Label>
          <Skeleton
            data-skeleton-region={`${setting.section}-control`}
            className="h-5 w-9 shrink-0 rounded-full"
          />
        </div>
        {belowPro ? (
          <p className="text-muted-foreground text-xs">
            {t(setting.proPlanOnlyKey)}
          </p>
        ) : null}
      </CardContent>
    </Card>
  );
}
