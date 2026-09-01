import { useCallback, useEffect, useMemo, useState } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  CancelWorkspaceCreationDocument,
  FinalizeWorkspaceCreationDocument,
  PrepareWorkspaceCreationDocument,
  WorkspaceCreationAttemptDocument,
  WorkspaceCreationPolicyDocument,
  WorkspacesDocument,
  type WorkspaceCreationAttemptQuery,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { graphQLErrorMessage } from "@/common/lib/graphql-error";
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

  useEffect(() => {
    const resumed = toAttempt(resumeQuery.data?.workspaceCreationAttempt);
    if (resumed) setAttempt(resumed);
  }, [resumeQuery.data]);

  useEffect(() => {
    setLocalError(null);
  }, [plan]);

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
            plan: attempt?.plan ?? plan,
            billingEmail,
            attemptId: attempt?.id || undefined,
            collectPaymentMethod,
          },
        });
        const prepared = toAttempt(response.data?.prepareWorkspaceCreation);
        if (!prepared) throw new Error("workspace setup returned no attempt");
        setAttempt(prepared);
        return prepared;
      } catch (error) {
        setLocalError(
          graphQLErrorMessage(error) ?? t("workspaces.createError"),
        );
        return null;
      } finally {
        setBusy(false);
      }
    },
    [attempt?.id, attempt?.plan, plan, prepareMutation, t],
  );

  const finalize = useCallback(
    async (value: WorkspaceCreationAttempt) => {
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
        toast.success(
          t("workspaces.createSuccess", { name: workspace.name ?? value.name }),
        );
        return {
          id: workspace.id,
          name: workspace.name ?? value.name,
          plan: workspace.plan ?? value.plan,
          role: workspace.role ?? "admin",
          createdAt: workspace.createdAt,
        } satisfies WorkspaceView;
      } catch (error) {
        setLocalError(
          graphQLErrorMessage(error) ?? t("workspaces.createError"),
        );
        return null;
      } finally {
        setBusy(false);
      }
    },
    [finalizeMutation, t],
  );

  const cancel = useCallback(async () => {
    if (!attempt) return;
    try {
      await cancelMutation({ variables: { attemptId: attempt.id } });
    } finally {
      setAttempt(null);
    }
  }, [attempt, cancelMutation]);

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
      graphQLErrorMessage(policyQuery.error) ??
      graphQLErrorMessage(resumeQuery.error),
  };
}
