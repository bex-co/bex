import { useCallback, useState } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { WorkspaceCreatePageSkeleton } from "@/common/components/route-skeletons";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { isValidEmail } from "@/common/lib/utils/email";
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
import { useWorkspaceCreationBilling } from "@/features/workspaces/hooks/use-workspace-creation-billing";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { PlanPicker } from "@/features/workspaces/components/plan-picker";
import { CreateWorkspacePaymentPanel } from "@/features/workspaces/components/create-workspace-payment-panel";
import {
  WORKSPACE_NAME_RE,
  type WorkspacePlanId,
} from "@/features/workspaces/types";

export const Route = createFileRoute("/new/workspace")({
  staticData: { chrome: true },
  component: NewWorkspacePage,
  pendingComponent: WorkspaceCreatePageSkeleton,
  beforeLoad: requireAuth(),
  validateSearch: (search: Record<string, unknown>) => ({
    attempt: typeof search.attempt === "string" ? search.attempt : undefined,
  }),
  head: ({ match }) => translatedTitleHead("workspaces.newTitle", match),
});

export function NewWorkspacePage() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { setCurrentWorkspaceId } = useWorkspace();
  const { session } = Route.useRouteContext();
  const { attempt: resumeAttemptId } = Route.useSearch();

  const traits = session?.identity?.traits as { email?: unknown } | undefined;
  const accountEmail =
    typeof traits?.email === "string" ? traits.email.trim().toLowerCase() : "";

  const [draftName, setDraftName] = useState("");
  const [selectedPlan, setSelectedPlan] = useState<WorkspacePlanId>("hobby");
  const [draftBillingEmail, setDraftBillingEmail] = useState(accountEmail);
  const [confirmedAttemptId, setConfirmedAttemptId] = useState<string | null>(
    null,
  );
  const [paymentError, setPaymentError] = useState<string | null>(null);
  const creation = useWorkspaceCreationBilling(selectedPlan, resumeAttemptId);
  const name = creation.attempt?.name ?? draftName;
  const plan = creation.attempt?.plan ?? selectedPlan;
  const billingEmail =
    creation.attempt?.billingEmail ??
    (plan === "hobby" ? accountEmail : draftBillingEmail);

  const paymentConfirmed =
    creation.attempt?.state === "setup_succeeded" ||
    creation.attempt?.id === confirmedAttemptId;
  const paymentRequired =
    creation.attempt?.paymentRequired ?? creation.policy.paymentRequired;
  const handlePaymentConfirmed = useCallback(() => {
    if (creation.attempt) setConfirmedAttemptId(creation.attempt.id);
    setPaymentError(null);
  }, [creation.attempt]);

  const nameValid = WORKSPACE_NAME_RE.test(name);
  const showNameError = name.length > 0 && !nameValid;
  const emailValid = isValidEmail(billingEmail);
  const showEmailError = billingEmail.length > 0 && !emailValid;
  const paymentBlocked = paymentRequired && !paymentConfirmed;
  const busy = creation.busy || creation.policyLoading;
  const canSubmit = nameValid && emailValid && !busy && !paymentBlocked;

  async function handleAddPaymentMethod() {
    if (!nameValid || !emailValid) return;
    setPaymentError(null);
    await creation.prepare(name, billingEmail, true);
  }

  async function handleSubmit() {
    if (!canSubmit) return;
    let pending = creation.attempt;
    if (!pending) {
      pending = await creation.prepare(name, billingEmail, false);
    }
    if (!pending) return;
    const workspace = await creation.finalize(pending);
    if (!workspace) return;
    setCurrentWorkspaceId(workspace.id);
    await navigate({ to: "/", replace: true });
  }

  async function handleCancel() {
    await creation.cancel();
    await navigate({ to: "/" });
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

          <section
            className="space-y-4"
            aria-labelledby="workspace-details-heading"
          >
            <h2
              id="workspace-details-heading"
              className="text-lg font-semibold"
            >
              {t("workspaces.detailsTitle")}
            </h2>
            <div className="space-y-2">
              <Label htmlFor="workspace-name">
                {t("workspaces.fieldSlug")}
              </Label>
              <Input
                id="workspace-name"
                value={name}
                onChange={(e) => setDraftName(e.target.value.toLowerCase())}
                placeholder={t("workspaces.fieldNamePlaceholder")}
                autoComplete="off"
                autoFocus
                disabled={creation.attempt != null}
                aria-invalid={showNameError}
                aria-describedby="workspace-slug-help"
              />
              <p
                id="workspace-slug-help"
                className="text-muted-foreground text-sm"
              >
                {t("workspaces.fieldSlugHelp")}
              </p>
              {showNameError ? (
                <p className="text-sm text-destructive" role="alert">
                  {t("workspaces.fieldNameError")}
                </p>
              ) : null}
            </div>

            <div className="space-y-2">
              <Label htmlFor="workspace-billing-email">
                {t("workspaces.billingEmail")}
              </Label>
              <Input
                id="workspace-billing-email"
                type="email"
                required
                value={billingEmail}
                onChange={(event) =>
                  setDraftBillingEmail(event.target.value.toLowerCase())
                }
                readOnly={plan === "hobby"}
                disabled={creation.attempt != null}
                autoComplete="email"
                aria-invalid={showEmailError}
                aria-describedby="workspace-billing-email-help"
              />
              <p
                id="workspace-billing-email-help"
                className="text-muted-foreground text-sm"
              >
                {plan === "hobby"
                  ? t("workspaces.billingEmailHobbyHelp")
                  : t("workspaces.billingEmailHelp")}
              </p>
              {showEmailError ? (
                <p className="text-sm text-destructive" role="alert">
                  {t("workspaces.billingEmailError")}
                </p>
              ) : null}
            </div>
          </section>

          <div className="space-y-2">
            <Label>{t("workspaces.fieldPlan")}</Label>
            <PlanPicker
              selected={plan}
              disabled={creation.attempt != null}
              onSelect={(nextPlan) => {
                setSelectedPlan(nextPlan);
                setConfirmedAttemptId(null);
                setPaymentError(null);
              }}
            />
          </div>

          <CreateWorkspacePaymentPanel
            attempt={creation.attempt}
            required={paymentRequired}
            providerAvailable={creation.policy.providerAvailable}
            disabled={!nameValid || !emailValid || busy}
            confirmed={paymentConfirmed}
            onAdd={() => void handleAddPaymentMethod()}
            onConfirmed={handlePaymentConfirmed}
            onError={setPaymentError}
          />

          {creation.error || paymentError ? (
            <Alert variant="destructive">
              <AlertTitle>{t("workspaces.createErrorTitle")}</AlertTitle>
              <AlertDescription>
                {creation.error ?? paymentError}
              </AlertDescription>
            </Alert>
          ) : null}

          <div className="flex justify-end gap-2 border-t pt-4">
            <Button
              variant="outline"
              onClick={() => void handleCancel()}
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
