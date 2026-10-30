import { useCallback, useMemo, useState } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  CancelWorkspaceCreationDocument,
  FinalizeWorkspaceCreationDocument,
  PrepareWorkspaceCreationDocument,
  RenameWorkspaceDocument,
  WorkspaceCreationAttemptDocument,
  WorkspaceCreationPolicyDocument,
  WorkspacesDocument,
  type WorkspaceCreationAttemptQuery,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import type {
  WorkspacePlanId,
  WorkspaceView,
} from "@/features/workspaces/types";

type WorkspaceCreationState =
  | "prepared"
  | "setup_pending"
  | "setup_succeeded"
  | "finalized"
  | "cleanup_pending"
  | "expired";

export interface WorkspaceCreationAttempt {
  id: string;
  name: string;
  plan: WorkspacePlanId;
  billingEmail: string;
  paymentRequired: boolean;
  state: WorkspaceCreationState;
  clientSecret: string;
  publishableKey: string;
}

function toAttempt(
  value: WorkspaceCreationAttemptQuery["workspaceCreationAttempt"] | undefined,
): WorkspaceCreationAttempt | null {
  if (
    !value?.id ||
    !value.plan ||
    !["hobby", "pro", "scale", "enterprise"].includes(value.plan) ||
    !value.state ||
    ![
      "prepared",
      "setup_pending",
      "setup_succeeded",
      "finalized",
      "cleanup_pending",
      "expired",
    ].includes(value.state)
  ) {
    return null;
  }
  return {
    id: value.id,
    name: value.name ?? "",
    plan: value.plan as WorkspacePlanId,
    billingEmail: value.billingEmail ?? "",
    paymentRequired: value.paymentRequired ?? false,
    state: value.state as WorkspaceCreationState,
    clientSecret: value.clientSecret ?? "",
    publishableKey: value.publishableKey ?? "",
  };
}

export function useWorkspaceCreationBilling(
  plan: WorkspacePlanId,
  resumeAttemptId?: string,
) {
  const { t } = useTranslations();
  const [attempt, setAttempt] = useState<WorkspaceCreationAttempt | null>(null);
  const [localError, setLocalError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const policyQuery = useQuery(WorkspaceCreationPolicyDocument, {
    variables: { plan },
  });
  const resumeQuery = useQuery(WorkspaceCreationAttemptDocument, {
    variables: { id: resumeAttemptId ?? "" },
    skip: !resumeAttemptId,
    fetchPolicy: "network-only",
  });
  const [prepareMutation] = useMutation(PrepareWorkspaceCreationDocument);
  const [finalizeMutation] = useMutation(FinalizeWorkspaceCreationDocument);
  const [cancelMutation] = useMutation(CancelWorkspaceCreationDocument);
  const [renameMutation] = useMutation(RenameWorkspaceDocument);

  // Each new resume read replaces the attempt, and another plan starts without
  // the last one's error — both settled while rendering, so neither paints a
  // frame behind its input.
  const [resumeData, setResumeData] = useState<typeof resumeQuery.data>();
  if (resumeQuery.data !== resumeData) {
    setResumeData(resumeQuery.data);
    const resumed = toAttempt(resumeQuery.data?.workspaceCreationAttempt);
    if (resumed) setAttempt(resumed);
  }
  const [errorPlan, setErrorPlan] = useState(plan);
  if (plan !== errorPlan) {
    setErrorPlan(plan);
    setLocalError(null);
  }

  const policy = useMemo(
    () => ({
      mode: policyQuery.data?.workspaceCreationPolicy?.mode ?? "off",
      paymentRequired:
        policyQuery.data?.workspaceCreationPolicy?.paymentRequired ?? false,
      providerAvailable:
        policyQuery.data?.workspaceCreationPolicy?.providerAvailable ?? false,
    }),
    [policyQuery.data],
  );

  const attemptId = attempt?.id;
  const attemptPlan = attempt?.plan;
  const prepare = useCallback(
    async (
      name: string,
      billingEmail: string,
      collectPaymentMethod: boolean,
    ) => {
      setBusy(true);
      setLocalError(null);
      try {
        const response = await prepareMutation({
          variables: {
            name,
            plan: attemptPlan ?? plan,
            billingEmail,
            attemptId: attemptId || undefined,
            collectPaymentMethod,
          },
        });
        const prepared = toAttempt(response.data?.prepareWorkspaceCreation);
        if (!prepared) throw new Error("workspace setup returned no attempt");
        setAttempt(prepared);
        return prepared;
      } catch (error) {
        setLocalError(mutationErrorMessage(error, t("workspaces.createError")));
        return null;
      } finally {
        setBusy(false);
      }
    },
    [attemptId, attemptPlan, plan, prepareMutation, t],
  );

  const finalize = useCallback(
    async (value: WorkspaceCreationAttempt, displayName = "") => {
      setBusy(true);
      setLocalError(null);
      try {
        const response = await finalizeMutation({
          variables: { attemptId: value.id },
          update(cache, { data }) {
            const created = data?.finalizeWorkspaceCreation;
            if (!created?.id) return;
            cache.updateQuery({ query: WorkspacesDocument }, (existing) => {
              const workspaces = existing?.workspaces ?? [];
              if (workspaces.some((workspace) => workspace?.id === created.id))
                return existing;
              return { workspaces: [...workspaces, created] };
            });
          },
        });
        const workspace = response.data?.finalizeWorkspaceCreation;
        if (!workspace?.id)
          throw new Error("workspace finalization returned no workspace");
        // Render's freeform name (w3/m246). The workspace exists by now, so
        // naming it is best effort: a failure leaves the slug as its name,
        // renameable in settings.
        let name = workspace.name ?? value.name;
        if (displayName && displayName !== name) {
          const renamed = await renameMutation({
            variables: { id: workspace.id, name: displayName },
          }).catch(() => null);
          if (renamed?.data?.renameWorkspace) name = displayName;
        }
        toast.success(t("workspaces.createSuccess", { name }));
        return {
          id: workspace.id,
          name,
          slug: workspace.slug ?? value.name,
          plan: workspace.plan ?? value.plan,
          role: workspace.role ?? "admin",
          createdAt: workspace.createdAt,
        } satisfies WorkspaceView;
      } catch (error) {
        setLocalError(mutationErrorMessage(error, t("workspaces.createError")));
        return null;
      } finally {
        setBusy(false);
      }
    },
    [finalizeMutation, renameMutation, t],
  );

  const cancel = useCallback(async () => {
    if (!attempt) return;
    // A rejected cancel keeps the attempt so the caller can say so and retry.
    await cancelMutation({ variables: { attemptId: attempt.id } });
    setAttempt(null);
  }, [attempt, cancelMutation]);

  const loadError = policyQuery.error ?? resumeQuery.error;

  return {
    policy,
    policyLoading: policyQuery.loading,
    attempt,
    prepare,
    finalize,
    cancel,
    busy,
    error:
      localError ??
      (loadError
        ? mutationErrorMessage(loadError, t("workspaces.creationLoadError"))
        : null),
  };
}
