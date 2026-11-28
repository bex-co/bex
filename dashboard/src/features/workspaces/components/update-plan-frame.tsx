import type { ReactNode } from "react";
import { useNavigate } from "@tanstack/react-router";
import { ArrowLeft, Info } from "lucide-react";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { PageFrame, PageHeader } from "@/common/components/page-frame";
import { CreateActionBarSkeleton } from "@/common/components/create-form";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { PaymentMethodCardFor } from "@/features/usage/components/payment-method-card";
import type { UseBillingOnboardingResult } from "@/features/usage/hooks/use-billing-onboarding";

/**
 * What Render's two plan pages share (w3/m708): the frame, the sections, the
 * card on file, the notice for a member who cannot change the plan, and the
 * frame drawn while the workspace loads.
 */

/** Render's Back above the plan page's title. */
function UpdatePlanBack({ to }: { to: string }) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  return (
    <Button
      variant="ghost"
      size="icon"
      aria-label={t("workspaces.updatePlanBack")}
      onClick={() => void navigate({ href: to })}
    >
      <ArrowLeft />
    </Button>
  );
}

/** The plan pages' frame: the title, the body and the action bar, and on
 *  the plan page Render's Back above the title (`back`; the confirmation has
 *  its Back in the action bar only, as Render's does). */
export function UpdatePlanFrame({
  title,
  back,
  footer,
  children,
}: {
  title: ReactNode;
  back?: string;
  footer?: ReactNode;
  children: ReactNode;
}) {
  return (
    <DashboardLayout>
      <PageFrame
        header={
          <PageHeader
            leading={back ? <UpdatePlanBack to={back} /> : undefined}
            title={title}
          />
        }
        footer={footer}
      >
        {children}
      </PageFrame>
    </DashboardLayout>
  );
}

/** The frame while the workspace list loads (the page's own skeleton body),
 *  or the empty state when there is no workspace. */
export function UpdatePlanLoadingFrame({
  title,
  back,
  loading,
  submitLabel,
  cancelLabel,
  children,
}: {
  title: ReactNode;
  back?: string;
  loading: boolean;
  submitLabel: string;
  cancelLabel: string;
  children: ReactNode;
}) {
  const { t } = useTranslations();
  return (
    <UpdatePlanFrame
      title={title}
      back={back}
      footer={
        loading ? (
          <CreateActionBarSkeleton
            submitLabel={submitLabel}
            cancelLabel={cancelLabel}
            summary={false}
          />
        ) : undefined
      }
    >
      {loading ? (
        children
      ) : (
        <p className="text-muted-foreground text-sm">
          {t("workspaces.settingsEmpty")}
        </p>
      )}
    </UpdatePlanFrame>
  );
}

/** A section of the plan pages: Render's block title, its line, its body. */
export function UpdatePlanSection({
  id,
  title,
  description,
  children,
}: {
  id: string;
  title: string;
  description?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section aria-labelledby={id} className="space-y-4">
      <div className="space-y-1">
        <h2 id={id} className="text-lg font-semibold">
          {title}
        </h2>
        {description ? (
          <p className="text-muted-foreground text-sm">{description}</p>
        ) : null}
      </div>
      {children}
    </section>
  );
}

/** The card on file and its hosted Add / Update actions, under Render's
 *  "Workspace billing" heading. */
export function WorkspaceBillingRow({
  billing,
}: {
  billing: UseBillingOnboardingResult;
}) {
  const { t } = useTranslations();
  return (
    <div className="grid grid-cols-1 gap-4 @3xl:grid-cols-3">
      <div className="space-y-1">
        <h3 className="font-medium">
          {t("workspaces.updatePlanWorkspaceBillingTitle")}
        </h3>
        <p className="text-muted-foreground text-sm">
          {t("workspaces.updatePlanWorkspaceBillingBody")}
        </p>
      </div>
      <div className="@3xl:col-span-2">
        <PaymentMethodCardFor billing={billing} />
      </div>
    </div>
  );
}

/** Why a member who cannot change the plan (can_manage) can choose nothing;
 *  nothing while the grant is unknown or allowed. */
export function UpdatePlanNoPermission() {
  const { t } = useTranslations();
  const { canManage, loading } = useCapabilities();
  if (loading || canManage) return null;
  return (
    <Alert>
      <Info />
      <AlertDescription>
        {t("workspaces.updatePlanNoPermission")}
      </AlertDescription>
    </Alert>
  );
}
