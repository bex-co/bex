import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Label } from "@/common/components/ui/label";
import { Skeleton } from "@/common/components/ui/skeleton";
import { ReservedButton } from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { OrganizationPlanNote } from "@/features/workspaces/components/organization-plan-note";
import { AvatarEditorSkeleton } from "@/features/avatars/components/avatar-editor-skeleton";
import type { OrganizationPlanOwner } from "@/features/workspaces/types";

/**
 * WorkspaceDetailsCard's pending shape (w3/m305): the title, the Name field
 * with its Save button, the Avatar row's 64 px circle over its Edit button
 * (w1/m648), and the plan/id/slug/created grid, each value a bar at
 * its ready line height (the id beside its 32 px copy button). The grid keeps
 * the ready container-query columns, so it stacks at phone width as the card
 * does.
 *
 * A secondary workspace of an organization (w7/m464) has the org-plan note
 * under the grid: `planOwner` draws it, reserved, when the frame already
 * knows the workspace from the switcher's list.
 */
export function WorkspaceDetailsCardSkeleton({
  planOwner = null,
}: {
  planOwner?: OrganizationPlanOwner | null;
}) {
  const { t } = useTranslations();
  return (
    <Card aria-hidden data-testid="workspace-details-skeleton">
      <CardHeader>
        <CardTitle>{t("workspaces.generalTitle")}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="space-y-2">
          <Label>{t("workspaces.fieldName")}</Label>
          <div className="flex gap-2">
            <Skeleton className="h-9 w-full max-w-sm" />
            <ReservedButton variant="default">
              {t("workspaces.renameSubmit")}
            </ReservedButton>
          </div>
        </div>
        <div className="space-y-2">
          <p className="text-sm leading-none font-medium">
            {t("avatars.rowLabel")}
          </p>
          <AvatarEditorSkeleton />
        </div>
        <dl className="grid grid-cols-1 gap-4 border-t pt-4 text-sm @md:grid-cols-2 @3xl:grid-cols-4">
          <div className="space-y-1">
            <dt className="text-muted-foreground">
              {t("workspaces.fieldPlan")}
            </dt>
            <dd>
              <Skeleton className="h-5 w-32" />
            </dd>
          </div>
          <div className="space-y-1">
            <dt className="text-muted-foreground">{t("workspaces.fieldId")}</dt>
            <dd className="flex min-w-0 items-center gap-1">
              <Skeleton className="h-4 w-44 min-w-0" />
              <Skeleton className="size-8 shrink-0" />
            </dd>
          </div>
          <div className="space-y-1">
            <dt className="text-muted-foreground">
              {t("workspaces.fieldSlug")}
            </dt>
            <dd>
              <Skeleton className="h-4 w-24" />
            </dd>
          </div>
          <div className="space-y-1">
            <dt className="text-muted-foreground">
              {t("workspaces.fieldCreatedAt")}
            </dt>
            <dd>
              <Skeleton className="h-5 w-36" />
            </dd>
          </div>
        </dl>
        {planOwner ? <OrganizationPlanNote owner={planOwner} pending /> : null}
      </CardContent>
    </Card>
  );
}
