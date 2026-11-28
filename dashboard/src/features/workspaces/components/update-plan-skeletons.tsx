import { ArrowLeft, ExternalLink } from "lucide-react";
import { useSearch } from "@tanstack/react-router";
import { PageFrame, PageHeader } from "@/common/components/page-frame";
import { Region } from "@/common/components/route-skeletons";
import { CreateActionBarSkeleton } from "@/common/components/create-form";
import {
  ReservedButton,
  ReservedText,
} from "@/common/components/reserved-skeleton";
import { Button } from "@/common/components/ui/button";
import { Skeleton } from "@/common/components/ui/skeleton";
import { Label } from "@/common/components/ui/label";
import { SudoPrompt } from "@/common/components/sudo-command-field";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  PLACEHOLDER_SENTINEL,
  aroundPlaceholder,
} from "@/common/lib/i18n-placeholder";
import { PaymentMethodCardSkeleton } from "@/features/usage/components/billing-cards-skeleton";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  downgradeConfirmationPhrase,
  isDowngrade,
  planFromSearch,
  removesOrganization,
} from "@/features/workspaces/lib/update-plan";
import { PlanComparisonCard } from "@/features/workspaces/components/plan-comparison-card";
import type { WorkspacePlanId } from "@/features/workspaces/types";

/**
 * The plan pages' pending frames (w3/m708): the ready pages' header, sections
 * and action bar at their own sizes, so the swap only fills them in. While
 * the workspace list loads, the ready pages draw these same bodies.
 */

/** A title with the workspace's name, its name a bar until it is known. */
function NamedTitle({
  messageKey,
  name,
}: {
  messageKey:
    "workspaces.updatePlanTitle" | "workspaces.updatePlanConfirmTitle";
  name: string | null;
}) {
  const { t } = useTranslations();
  if (name !== null) return <>{t(messageKey, { name })}</>;
  const [before, after] = aroundPlaceholder(
    t(messageKey, { name: PLACEHOLDER_SENTINEL }),
  );
  return (
    <>
      {before}
      <Skeleton className="inline-block h-5 w-32 align-middle" />
      {after}
    </>
  );
}

/** Render's "Change plan for <workspace>". */
export function UpdatePlanTitle({ name }: { name: string | null }) {
  return <NamedTitle messageKey="workspaces.updatePlanTitle" name={name} />;
}

/** Render's confirmation title, "Change Plan for <workspace>". */
export function UpdatePlanConfirmTitle({ name }: { name: string | null }) {
  return (
    <NamedTitle messageKey="workspaces.updatePlanConfirmTitle" name={name} />
  );
}

/** A section heading (and its line) as the ready section draws it, the
 *  words reserved so they wrap alike at every width and in every language. */
function SectionHeadingSkeleton({
  title,
  description,
}: {
  title: string;
  description?: React.ReactNode;
}) {
  return (
    <div className="space-y-1">
      <p className="relative w-fit text-lg font-semibold">
        <ReservedText>{title}</ReservedText>
      </p>
      {description ? (
        <p className="text-muted-foreground relative text-sm">
          <ReservedText>{description}</ReservedText>
        </p>
      ) : null}
    </div>
  );
}

/** The workspace billing row: its heading and line beside the card. */
function WorkspaceBillingRowSkeleton() {
  const { t } = useTranslations();
  return (
    <div className="grid grid-cols-1 gap-4 @3xl:grid-cols-3">
      <div className="space-y-1">
        <p className="relative font-medium">
          <ReservedText>
            {t("workspaces.updatePlanWorkspaceBillingTitle")}
          </ReservedText>
        </p>
        <p className="text-muted-foreground relative text-sm">
          <ReservedText>
            {t("workspaces.updatePlanWorkspaceBillingBody")}
          </ReservedText>
        </p>
      </div>
      <div className="@3xl:col-span-2">
        <PaymentMethodCardSkeleton />
      </div>
    </div>
  );
}

/** The plan page's body: the four plan cards, then Payment Method. */
export function UpdatePlanBodySkeleton({
  cardNotice = true,
}: {
  /** Whether the paid-plan card notice is drawn (any plan but Hobby). */
  cardNotice?: boolean;
}) {
  const { t } = useTranslations();
  return (
    <div className="space-y-10">
      <Region name="select-plan" className="space-y-4">
        <SectionHeadingSkeleton
          title={t("workspaces.updatePlanSelectTitle")}
          description={t("workspaces.updatePlanCompare", {
            link: t("workspaces.updatePlanPricingLink"),
          })}
        />
        <div className="space-y-3">
          {/* Four columns draw the cards taller than their 14rem floor: the
              ready cards' content is 17.25rem there. */}
          <div className="grid grid-cols-1 gap-4 @xl:grid-cols-2 @4xl:grid-cols-4">
            {Array.from({ length: 4 }, (_, index) => (
              <Skeleton
                key={index}
                className="h-56 w-full rounded-xl @4xl:h-[17.25rem]"
              />
            ))}
          </div>
          <p className="text-muted-foreground relative text-xs">
            <ReservedText>{t("workspaces.planUsageBillingNote")}</ReservedText>
          </p>
        </div>
      </Region>
      <Region name="payment-method" className="space-y-4">
        <SectionHeadingSkeleton
          title={t("workspaces.updatePlanPaymentTitle")}
        />
        {cardNotice ? (
          <div className="flex items-start gap-3 rounded-lg border px-4 py-3 text-sm">
            <Skeleton className="mt-0.5 size-4 shrink-0" />
            <div className="flex min-w-0 flex-1 flex-col gap-2 @2xl:flex-row @2xl:items-center @2xl:justify-between @2xl:gap-4">
              <p className="relative">
                <ReservedText>
                  {t("workspaces.updatePlanCardRequired")}
                </ReservedText>
              </p>
              <ReservedButton variant="outline" size="sm">
                {t("workspaces.updatePlanHowBillingWorks")}
                <ExternalLink aria-hidden="true" />
              </ReservedButton>
            </div>
          </div>
        ) : null}
        <WorkspaceBillingRowSkeleton />
      </Region>
    </div>
  );
}

/** The confirmation's body: New plan summary, and Confirm downgrade for a
 *  downgrade. Known plans draw the comparison itself, its words reserved;
 *  a known name reserves the typed phrase's prompt. */
export function UpdatePlanConfirmBodySkeleton({
  downgrade,
  current = null,
  plan = null,
  name = null,
  removesOrganization = false,
}: {
  downgrade: boolean;
  current?: string | null;
  plan?: WorkspacePlanId | null;
  name?: string | null;
  removesOrganization?: boolean;
}) {
  const { t } = useTranslations();
  const phrase = downgradeConfirmationPhrase(name ?? "");
  return (
    <div className="space-y-10">
      <Region name="plan-summary" className="space-y-4">
        <SectionHeadingSkeleton
          title={t("workspaces.updatePlanSummaryTitle")}
        />
        {current && plan ? (
          <PlanComparisonCard current={current} next={plan} pending />
        ) : (
          <div className="grid grid-cols-1 gap-3 @xl:grid-cols-2">
            <Skeleton className="h-[25rem] w-full rounded-xl @xl:h-[22.625rem]" />
            <Skeleton className="h-[25rem] w-full rounded-xl @xl:h-[22.625rem]" />
          </div>
        )}
        <WorkspaceBillingRowSkeleton />
      </Region>
      {downgrade ? (
        <Region name="confirm-downgrade" className="space-y-4">
          <SectionHeadingSkeleton
            title={t("workspaces.updatePlanConfirmDowngradeTitle")}
          />
          {removesOrganization ? (
            <div className="space-y-2 text-sm">
              <p className="relative">
                <ReservedText>
                  {t("workspaces.updatePlanConfirmChangesIncluding")}
                </ReservedText>
              </p>
              <ul className="list-disc space-y-1 pl-5">
                <li className="relative">
                  <ReservedText>
                    {t("workspaces.updatePlanRemovesOrganization")}{" "}
                    {t("workspaces.updatePlanRemovesOrganizationDetail")}
                  </ReservedText>
                </li>
              </ul>
            </div>
          ) : null}
          <div className="space-y-2">
            <p className="text-muted-foreground relative text-sm">
              <ReservedText>
                <SudoPrompt
                  promptKey={
                    removesOrganization
                      ? "workspaces.updatePlanConfirmTypeAfterList"
                      : "workspaces.updatePlanConfirmType"
                  }
                  phrase={phrase}
                />
              </ReservedText>
            </p>
            <Label>{t("common.sudoCommandLabel")}</Label>
            <Skeleton className="h-9 w-full max-w-md" />
          </div>
        </Region>
      ) : null}
    </div>
  );
}

function PendingFrame({
  shape,
  title,
  back,
  submitLabel,
  cancelLabel,
  children,
}: {
  shape: string;
  title: React.ReactNode;
  /** Whether the ready page has Back above its title (the plan page). */
  back: boolean;
  submitLabel: string;
  cancelLabel: string;
  children: React.ReactNode;
}) {
  return (
    <PageFrame
      pending={shape}
      header={
        <PageHeader
          leading={
            back ? (
              <Button variant="ghost" size="icon" tabIndex={-1}>
                <ArrowLeft />
              </Button>
            ) : undefined
          }
          title={title}
        />
      }
      footer={
        <CreateActionBarSkeleton
          submitLabel={submitLabel}
          cancelLabel={cancelLabel}
          summary={false}
        />
      }
    >
      {children}
    </PageFrame>
  );
}

/** `/billing/update-plan`'s pending frame. */
export function UpdatePlanPageSkeleton() {
  const { t } = useTranslations();
  const { currentWorkspace } = useWorkspace();
  const search = useSearch({ strict: false }) as { plan?: unknown };
  const selected =
    planFromSearch(search.plan) ?? currentWorkspace?.plan ?? "pro";
  return (
    <PendingFrame
      shape="update-plan"
      back
      title={<UpdatePlanTitle name={currentWorkspace?.name ?? null} />}
      submitLabel={t("workspaces.updatePlanSubmit")}
      cancelLabel={t("workspaces.updatePlanCancel")}
    >
      <UpdatePlanBodySkeleton cardNotice={selected !== "hobby"} />
    </PendingFrame>
  );
}

/** `/billing/update-plan-confirm`'s pending frame: a downgrade unless the
 *  workspace is known to be below the plan. */
export function UpdatePlanConfirmPageSkeleton() {
  const { t } = useTranslations();
  const { currentWorkspace } = useWorkspace();
  const search = useSearch({ strict: false }) as { plan?: unknown };
  const plan = planFromSearch(search.plan) ?? null;
  const downgrade =
    !currentWorkspace || !plan || isDowngrade(currentWorkspace.plan, plan);
  return (
    <PendingFrame
      shape="update-plan-confirm"
      back={false}
      title={<UpdatePlanConfirmTitle name={currentWorkspace?.name ?? null} />}
      submitLabel={t("workspaces.updatePlanConfirmSubmit")}
      cancelLabel={t("workspaces.updatePlanBack")}
    >
      <UpdatePlanConfirmBodySkeleton
        downgrade={downgrade}
        current={currentWorkspace?.plan ?? null}
        plan={plan}
        name={currentWorkspace?.name ?? null}
        removesOrganization={
          !!currentWorkspace &&
          !!plan &&
          removesOrganization(currentWorkspace, plan)
        }
      />
    </PendingFrame>
  );
}
