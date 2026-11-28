import { useEffect, useRef, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { ExternalLink, Info } from "lucide-react";
import { CreateActionBar } from "@/common/components/create-form";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  PLACEHOLDER_SENTINEL,
  aroundPlaceholder,
} from "@/common/lib/i18n-placeholder";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { usePlanChangeBlockers } from "@/features/workspaces/hooks/use-plan-change-blockers";
import {
  usePlanName,
  useUpdatePlanPage,
} from "@/features/workspaces/hooks/use-update-plan-page";
import {
  UpdatePlanFrame,
  UpdatePlanLoadingFrame,
  UpdatePlanNoPermission,
  UpdatePlanSection,
  WorkspaceBillingRow,
} from "@/features/workspaces/components/update-plan-frame";
import { PlanPicker } from "@/features/workspaces/components/plan-picker";
import { PlanChangeBlockers } from "@/features/workspaces/components/plan-change-blockers";
import { OrganizationPlanNote } from "@/features/workspaces/components/organization-plan-note";
import {
  UpdatePlanBodySkeleton,
  UpdatePlanTitle,
} from "@/features/workspaces/components/update-plan-skeletons";
import {
  cardMissing,
  isDowngrade,
  planFromSearch,
  planSearchValue,
  type UpdatePlanSearch,
} from "@/features/workspaces/lib/update-plan";
import { isPaidWorkspacePlan as paidPlan } from "@/features/workspaces/lib/create-workspace-payment";
import {
  organizationPlanOwner,
  type WorkspacePlanId,
} from "@/features/workspaces/types";

/** bex's pricing page, Render's "pricing page" link. */
const PRICING_URL = "https://bex.co/pricing";
/** bex's billing docs, Render's "How billing works" (its /faq#billing). */
const BILLING_DOCS_URL = "https://bex.co/docs/platform/billing-setup";

/**
 * Render's Change plan page, `/billing/update-plan` (w3/m708; its
 * BillingUpdatePlan, docs/render-artifacts/workspace-plan-change.md). The
 * workspace's plan cards, with `?plan=` choosing one (Render's aliases) and
 * Enterprise as Get in touch; Payment Method with the card on file; each plan
 * rule the choice breaks, from bex-api's own checks, before Select plan; and
 * Cancel back to `next`. Select plan applies an upgrade here, as Render's
 * does, and takes a downgrade to the confirmation and its typed phrase.
 */
export function UpdatePlanPage({ search }: { search: UpdatePlanSearch }) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { currentWorkspace, workspaces, loading } = useWorkspace();
  const capabilities = useCapabilities();
  const { returnPath, billing, change, leave, apply } = useUpdatePlanPage(
    search.next,
    t("workspaces.updatePlanServerError"),
  );

  const current = currentWorkspace?.plan ?? "";
  const [chosen, setChosen] = useState<WorkspacePlanId | undefined>(
    planFromSearch(search.plan),
  );
  // Render's form follows a later ?plan= (a second upgrade link).
  const [seenPlan, setSeenPlan] = useState(search.plan);
  if (seenPlan !== search.plan) {
    setSeenPlan(search.plan);
    setChosen(planFromSearch(search.plan));
  }
  const selected = chosen ?? (current as WorkspacePlanId);
  const planOwner = organizationPlanOwner(currentWorkspace, workspaces);
  const canChange =
    !planOwner && (capabilities.canManage || capabilities.loading);
  const changing = selected !== current && canChange;
  const { blockers, loading: blockersLoading } = usePlanChangeBlockers(
    currentWorkspace?.id,
    changing ? selected : null,
  );
  const planName = usePlanName(selected);
  const needsCard = changing && cardMissing(selected, billing.readiness);
  const held = blockers.length > 0 || needsCard;
  const canSubmit =
    changing &&
    capabilities.canManage &&
    !held &&
    !blockersLoading &&
    !change.busy;

  // Render scrolls to its errors when Select plan is held by them.
  const errorsRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (held) errorsRef.current?.scrollIntoView?.({ block: "nearest" });
  }, [held]);

  if (!currentWorkspace) {
    return (
      <UpdatePlanLoadingFrame
        title={<UpdatePlanTitle name={null} />}
        back={returnPath}
        loading={loading}
        submitLabel={t("workspaces.updatePlanSubmit")}
        cancelLabel={t("workspaces.updatePlanCancel")}
      >
        {/* The plan is unknown until the workspace is: reserve the notice
            unless the URL chose Hobby. */}
        <UpdatePlanBodySkeleton cardNotice={chosen ? paidPlan(chosen) : true} />
      </UpdatePlanLoadingFrame>
    );
  }

  async function handleSubmit() {
    if (!currentWorkspace || !canSubmit) return;
    if (isDowngrade(current, selected)) {
      void navigate({
        to: "/billing/update-plan-confirm",
        search: { plan: planSearchValue(selected), next: search.next },
        state: (prev) => ({ ...prev, afterPlanUpdate: returnPath }),
      });
      return;
    }
    await apply(currentWorkspace.id, selected);
  }

  const [pricingBefore, pricingAfter] = aroundPlaceholder(
    t("workspaces.updatePlanCompare", { link: PLACEHOLDER_SENTINEL }),
  );
  const summary = change.error
    ? t("workspaces.updatePlanErrorPrefix", { message: change.error })
    : held
      ? t("workspaces.updatePlanErrorsSummary")
      : null;
  const shownBlockers = blockers.length > 0 ? blockers : change.preconditions;

  return (
    <UpdatePlanFrame
      title={<UpdatePlanTitle name={currentWorkspace.name} />}
      back={returnPath}
      footer={
        <CreateActionBar
          submitLabel={t("workspaces.updatePlanSubmit")}
          onSubmit={() => void handleSubmit()}
          canSubmit={canSubmit}
          busy={change.busy}
          cancelLabel={t("workspaces.updatePlanCancel")}
          onCancel={leave}
          blocked={
            summary ? { reason: summary, fieldId: "update-plan-errors" } : null
          }
        />
      }
    >
      <div className="space-y-10">
        <UpdatePlanNoPermission />
        <UpdatePlanSection
          id="update-plan-select"
          title={t("workspaces.updatePlanSelectTitle")}
          description={
            <>
              {pricingBefore}
              <a
                href={PRICING_URL}
                target="_blank"
                rel="noreferrer"
                className="text-primary font-medium underline-offset-4 hover:underline"
              >
                {t("workspaces.updatePlanPricingLink")}
              </a>
              {pricingAfter}
            </>
          }
        >
          {planOwner ? (
            <OrganizationPlanNote owner={planOwner} target="billing" />
          ) : null}
          <PlanPicker
            current={current}
            selected={selected}
            onSelect={setChosen}
            disabled={!canChange || change.busy}
          />
        </UpdatePlanSection>
        <UpdatePlanSection
          id="update-plan-payment"
          title={t("workspaces.updatePlanPaymentTitle")}
        >
          {paidPlan(selected) ? (
            <div className="flex items-start gap-3 rounded-lg border bg-muted/40 px-4 py-3 text-sm">
              <Info
                aria-hidden="true"
                className="text-muted-foreground mt-0.5 size-4 shrink-0"
              />
              <div className="flex min-w-0 flex-1 flex-col gap-2 @2xl:flex-row @2xl:items-center @2xl:justify-between @2xl:gap-4">
                <p>{t("workspaces.updatePlanCardRequired")}</p>
                <Button
                  asChild
                  size="sm"
                  variant="outline"
                  className="shrink-0 self-start @2xl:self-auto"
                >
                  <a href={BILLING_DOCS_URL} target="_blank" rel="noreferrer">
                    {t("workspaces.updatePlanHowBillingWorks")}
                    <ExternalLink aria-hidden="true" />
                  </a>
                </Button>
              </div>
            </div>
          ) : null}
          <WorkspaceBillingRow billing={billing} />
        </UpdatePlanSection>
        <div
          id="update-plan-errors"
          ref={errorsRef}
          tabIndex={-1}
          className="space-y-3 scroll-mt-6 outline-none empty:hidden"
        >
          <PlanChangeBlockers
            blockers={shownBlockers}
            planName={planName}
            organizationId={currentWorkspace.organization?.id ?? null}
          />
          {summary ? (
            <p role="alert" className="text-destructive text-sm">
              {summary}
            </p>
          ) : null}
        </div>
      </div>
    </UpdatePlanFrame>
  );
}
