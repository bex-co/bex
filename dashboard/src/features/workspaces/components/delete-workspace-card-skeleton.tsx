import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Label } from "@/common/components/ui/label";
import { Skeleton } from "@/common/components/ui/skeleton";
import { SudoPrompt } from "@/common/components/sudo-command-field";
import {
  ReservedButton,
  ReservedText,
} from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { workspaceDeleteConfirmation } from "@/features/workspaces/types";

/**
 * DeleteWorkspaceCard's pending shape (w3/m305): its static header, the sudo
 * prompt naming the workspace (reserved invisibly until the name is known, so
 * it wraps as the ready prompt does), the Sudo Command field and the button.
 */
export function DeleteWorkspaceCardSkeleton({
  workspaceName,
}: {
  workspaceName?: string;
}) {
  const { t } = useTranslations();
  const prompt = (
    <SudoPrompt
      promptKey="workspaces.deleteConfirmLabel"
      phrase={workspaceDeleteConfirmation(workspaceName ?? "My workspace")}
    />
  );
  return (
    <Card
      className="border-destructive/50"
      aria-hidden
      data-testid="delete-workspace-skeleton"
    >
      <CardHeader>
        <CardTitle className="text-destructive">
          {t("workspaces.dangerZoneTitle")}
        </CardTitle>
        <CardDescription>
          {t("workspaces.dangerZoneDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          <p className="relative text-sm text-muted-foreground">
            {workspaceName ? prompt : <ReservedText>{prompt}</ReservedText>}
          </p>
          <Label>{t("common.sudoCommandLabel")}</Label>
          <Skeleton className="h-9 w-full max-w-sm" />
        </div>
        <ReservedButton variant="destructive">
          {t("workspaces.deleteSubmit")}
        </ReservedButton>
      </CardContent>
    </Card>
  );
}
