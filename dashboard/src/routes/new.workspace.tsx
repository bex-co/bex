import { useState } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { FormPageSkeleton } from "@/common/components/detail-skeletons";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Button } from "@/common/components/ui/button";
import {
  Alert,
  AlertTitle,
  AlertDescription,
} from "@/common/components/ui/alert";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCreateWorkspace } from "@/features/workspaces/hooks/use-create-workspace";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { PlanPicker } from "@/features/workspaces/components/plan-picker";
import { CreateWorkspacePaymentPanel } from "@/features/workspaces/components/create-workspace-payment-panel";
import { useBillingOnboarding } from "@/features/usage/hooks/use-billing-onboarding";
import {
  createBlockedByPayment,
  isPaidWorkspacePlan,
} from "@/features/workspaces/lib/create-workspace-payment";
import {
  WORKSPACE_NAME_RE,
  type WorkspacePlanId,
} from "@/features/workspaces/types";

export const Route = createFileRoute("/new/workspace")({
  staticData: { chrome: true },
  component: NewWorkspacePage,
  pendingComponent: FormPageSkeleton,
  beforeLoad: requireAuth(),
  head: ({ match }) => translatedTitleHead("workspaces.newTitle", match),
});

/**
 * `/new/workspace`: page heading, DNS-label slug, large plan cards (fees from
 * pricing.yaml at 30% off Render), payment panel for paid plans, then create
 * -> switch -> land in the new (empty) workspace. A plan-limit refusal
 * surfaces inline via the create hook's `error`.
 */
export function NewWorkspacePage() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { create, busy, error } = useCreateWorkspace();
  const { setCurrentWorkspaceId, currentWorkspaceId } = useWorkspace();

  const [name, setName] = useState("");
  const [plan, setPlan] = useState<WorkspacePlanId>("hobby");

  const paid = isPaidWorkspacePlan(plan);
  const billing = useBillingOnboarding({ active: paid });
  const paymentBlocked = createBlockedByPayment({
    plan,
    requirePaymentMethod: billing.readiness?.paymentMethodRequired ?? false,
    paymentMethodReady: billing.readiness?.paymentMethodReady ?? false,
    billingLoading: billing.loading,
    hasCurrentWorkspace: currentWorkspaceId != null,
  });

  const nameValid = WORKSPACE_NAME_RE.test(name);
  const showNameError = name.length > 0 && !nameValid;
  const canSubmit = nameValid && !busy && !paymentBlocked;

  async function handleSubmit() {
    if (!canSubmit) return;
    const workspace = await create(name, plan);
    if (!workspace) return;
    // create() adds the returned workspace to the shared list cache before it
    // resolves, so WorkspaceProvider recognizes this id and cannot fall back
    // to the first old workspace while navigation is in flight.
    setCurrentWorkspaceId(workspace.id);
    await navigate({ to: "/", replace: true });
  }

  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-4xl space-y-8">
          <header className="space-y-1">
            <h1 className="text-2xl font-semibold tracking-tight">
              {t("workspaces.createTitle")}
            </h1>
            <p className="text-muted-foreground">
              {t("workspaces.createDescription")}
            </p>
          </header>

          <div className="space-y-2">
            <Label htmlFor="workspace-name">
              {t("workspaces.fieldSlug")}
            </Label>
            <Input
              id="workspace-name"
              value={name}
              onChange={(e) => setName(e.target.value.toLowerCase())}
              placeholder={t("workspaces.fieldNamePlaceholder")}
              autoComplete="off"
              autoFocus
              aria-invalid={showNameError}
              aria-describedby="workspace-slug-help"
              onKeyDown={(e) => {
                if (e.key === "Enter") void handleSubmit();
              }}
            />
            <p id="workspace-slug-help" className="text-muted-foreground text-sm">
              {t("workspaces.fieldSlugHelp")}
            </p>
            {showNameError ? (
              <p className="text-sm text-destructive">
                {t("workspaces.fieldNameError")}
              </p>
            ) : null}
          </div>

          <div className="space-y-2">
            <Label>{t("workspaces.fieldPlan")}</Label>
            <PlanPicker selected={plan} onSelect={setPlan} />
          </div>

          {paid ? <CreateWorkspacePaymentPanel billing={billing} /> : null}

          {error ? (
            <Alert variant="destructive">
              <AlertTitle>{t("workspaces.createErrorTitle")}</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}

          <div className="flex justify-end gap-2 border-t pt-4">
            <Button
              variant="outline"
              onClick={() => void navigate({ to: "/" })}
              disabled={busy}
            >
              {t("workspaces.createCancel")}
            </Button>
            <Button
              onClick={() => void handleSubmit()}
              disabled={!canSubmit}
              loading={busy}
            >
              {t("workspaces.createSubmit")}
            </Button>
          </div>
        </div>
      </div>
    </DashboardLayout>
  );
}
