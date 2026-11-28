import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { CreateActionBar } from "@/common/components/create-form";
import { SudoCommandField } from "@/common/components/sudo-command-field";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  usePlanName,
  useUpdatePlanPage,
} from "@/features/workspaces/hooks/use-update-plan-page";
import { PlanChangeBlockers } from "@/features/workspaces/components/plan-change-blockers";
import { PlanComparisonCard } from "@/features/workspaces/components/plan-comparison-card";
import {
  UpdatePlanConfirmBodySkeleton,
  UpdatePlanConfirmTitle,
} from "@/features/workspaces/components/update-plan-skeletons";
import {
  UpdatePlanFrame,
  UpdatePlanLoadingFrame,
  UpdatePlanNoPermission,
  UpdatePlanSection,
  WorkspaceBillingRow,
} from "@/features/workspaces/components/update-plan-frame";
import {
  downgradeConfirmationMatches,
  downgradeConfirmationPhrase,
  isDowngrade,
  removesOrganization,
  type UpdatePlanSearch,
} from "@/features/workspaces/lib/update-plan";
import type { WorkspacePlanId } from "@/features/workspaces/types";

/**
 * Render's plan confirmation, `/billing/update-plan-confirm` (w3/m708; its
 * BillingUpdatePlanConfirmation, docs/render-artifacts/workspace-plan-change.md):
 * New plan summary sets the current plan beside the new one over the card on
 * file, and a downgrade adds Confirm downgrade, which asks for
 * `sudo downgrade workspace <name>` before Confirm. A plan reached here
 * without a downgrade (a typed URL) confirms without the phrase, as Render's
 * does. Back returns to the plan page; success goes to `next`.
 */
export function UpdatePlanConfirmPage({
  plan,
  search,
}: {
  plan: WorkspacePlanId;
  search: UpdatePlanSearch;
}) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { currentWorkspace, loading } = useWorkspace();
  const capabilities = useCapabilities();
  const { billing, change, apply } = useUpdatePlanPage(
    search.next,
    t("workspaces.updatePlanError"),
  );
  const [typed, setTyped] = useState("");
  const planName = usePlanName(plan);

  const back = () =>
    void navigate({
      to: "/billing/update-plan",
      search: { next: search.next },
      state: (prev) => prev,
    });

  if (!currentWorkspace) {
    return (
      <UpdatePlanLoadingFrame
        title={<UpdatePlanConfirmTitle name={null} />}
        loading={loading}
        submitLabel={t("workspaces.updatePlanConfirmSubmit")}
        cancelLabel={t("workspaces.updatePlanBack")}
      >
        <UpdatePlanConfirmBodySkeleton downgrade plan={plan} />
      </UpdatePlanLoadingFrame>
    );
  }

  const downgrade = isDowngrade(currentWorkspace.plan, plan);
  const phrase = downgradeConfirmationPhrase(currentWorkspace.name);
  const endsOrganization = removesOrganization(currentWorkspace, plan);
  const canConfirm =
    !change.busy &&
    (capabilities.loading || capabilities.canManage) &&
    (!downgrade || downgradeConfirmationMatches(typed, phrase));

  async function handleConfirm() {
    if (!currentWorkspace || !canConfirm) return;
    await apply(currentWorkspace.id, plan);
  }

  return (
    <UpdatePlanFrame
      title={<UpdatePlanConfirmTitle name={currentWorkspace.name} />}
      footer={
        <CreateActionBar
          submitLabel={t("workspaces.updatePlanConfirmSubmit")}
          onSubmit={() => void handleConfirm()}
          canSubmit={canConfirm}
          busy={change.busy}
          cancelLabel={t("workspaces.updatePlanBack")}
          onCancel={back}
        />
      }
    >
      <div className="space-y-10">
        <UpdatePlanNoPermission />
        <UpdatePlanSection
          id="update-plan-summary"
          title={t("workspaces.updatePlanSummaryTitle")}
        >
          <PlanComparisonCard current={currentWorkspace.plan} next={plan} />
          <WorkspaceBillingRow billing={billing} />
        </UpdatePlanSection>
        {downgrade ? (
          <UpdatePlanSection
            id="update-plan-confirm-downgrade"
            title={t("workspaces.updatePlanConfirmDowngradeTitle")}
          >
            {endsOrganization ? (
              <div className="space-y-2 text-sm">
                <p>{t("workspaces.updatePlanConfirmChangesIncluding")}</p>
                <ul className="list-disc space-y-1 pl-5">
                  <li>
                    <span className="font-semibold">
                      {t("workspaces.updatePlanRemovesOrganization")}
                    </span>{" "}
                    {t("workspaces.updatePlanRemovesOrganizationDetail")}
                  </li>
                </ul>
              </div>
            ) : null}
            <SudoCommandField
              id="sudo-command"
              promptKey={
                endsOrganization
                  ? "workspaces.updatePlanConfirmTypeAfterList"
                  : "workspaces.updatePlanConfirmType"
              }
              phrase={phrase}
              value={typed}
              onValueChange={setTyped}
              inputClassName="max-w-md"
              autoFocus
            />
          </UpdatePlanSection>
        ) : null}
        {change.error ? (
          <div className="space-y-3">
            <Alert variant="destructive">
              <AlertTitle>{t("workspaces.updatePlanErrorTitle")}</AlertTitle>
              <AlertDescription>{change.error}</AlertDescription>
            </Alert>
            <PlanChangeBlockers
              blockers={change.preconditions}
              planName={planName}
              organizationId={currentWorkspace.organization?.id ?? null}
            />
          </div>
        ) : null}
      </div>
    </UpdatePlanFrame>
  );
}
