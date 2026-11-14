import { useState } from "react";
import { Card, CardContent } from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCardRadioGroup } from "@/common/hooks/use-card-radio-group";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  OVERLAPPING_DEPLOY_POLICIES,
  useOverlappingDeployPolicy,
  type OverlappingDeployPolicy,
} from "@/features/workspaces/hooks/use-overlapping-deploy-policy";
import {
  OverlappingDeployPolicyCardHeader,
  OverlappingDeployPolicyCardSkeleton,
  OverlappingDeployPolicyOption,
} from "@/features/workspaces/components/overlapping-deploy-policy-card-skeleton";

/**
 * Workspace Settings → Overlapping Deploy Policy (w4/653), as on Render
 * (render.com/docs/deploys): Wait, the default for every workspace created on
 * or after 2025-07-14, or Override. Every member reads it; Admins change it.
 */
export function OverlappingDeployPolicyCard({
  workspaceId,
}: {
  workspaceId: string;
}) {
  const { t } = useTranslations();
  const { canManage } = useCapabilities();
  const { policy, loading, saving, save } =
    useOverlappingDeployPolicy(workspaceId);
  // null follows the saved policy, so a save (or a refetch) re-syncs it.
  const [draft, setDraft] = useState<OverlappingDeployPolicy | null>(null);
  const saved = policy ?? "wait";
  const selected = draft ?? saved;
  const locked = !canManage || saving || policy === null;
  const radio = useCardRadioGroup(
    OVERLAPPING_DEPLOY_POLICIES,
    selected,
    setDraft,
    locked,
  );

  if (loading) {
    return <OverlappingDeployPolicyCardSkeleton canManage={canManage} />;
  }
  return (
    <Card>
      <OverlappingDeployPolicyCardHeader />
      <CardContent className="space-y-4">
        <div
          role="radiogroup"
          aria-label={t("workspaces.overlappingDeployPolicyTitle")}
          aria-disabled={locked || undefined}
          className="grid grid-cols-1 gap-2"
        >
          {OVERLAPPING_DEPLOY_POLICIES.map((option) => {
            const { ref, ...keyboard } = radio(option);
            return (
              <OverlappingDeployPolicyOption
                key={option}
                {...keyboard}
                buttonRef={ref}
                policy={option}
                checked={option === selected}
                disabled={locked}
                onClick={() => setDraft(option)}
              />
            );
          })}
        </div>
        {canManage ? (
          <Button
            size="sm"
            disabled={locked || selected === saved}
            onClick={async () => {
              if (await save(selected)) setDraft(null);
            }}
          >
            {saving
              ? t("workspaces.overlappingDeployPolicySaving")
              : t("workspaces.overlappingDeployPolicySave")}
          </Button>
        ) : (
          <p className="text-muted-foreground text-sm">
            {t("workspaces.overlappingDeployPolicyAdminOnly")}
          </p>
        )}
      </CardContent>
    </Card>
  );
}
