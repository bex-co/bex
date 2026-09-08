import { useEffect, useRef } from "react";
import { useMutation } from "@apollo/client/react";
import { DashboardCard } from "@/components/dashboard-card";
import {
  SafeActionPanel,
  defineSafeAction,
  mobileLifecycleResult,
  type MobileActionOption,
  type MobileActionRunResult,
} from "@/components/safe-action";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilities } from "@/features/capabilities/capabilities-provider";
import {
  MobileCancelDeployDocument,
  MobileRestartServiceDocument,
  MobileResumeServiceDocument,
  MobileRollbackServiceDocument,
  MobileSuspendServiceDocument,
  MobileTriggerDeployDocument,
} from "@/generated-graphql";
import {
  DeployActionController,
  isCancelableDeployStatus,
  isRollbackableDeployStatus,
  type DeployAction,
  type DeployActionError,
  type DeployServerGate,
  type DeployTarget,
} from "@/features/deploys/deploy-actions";
import {
  useDeployActions,
  useServerActions,
} from "@/features/capabilities/api/use-resource-actions";
import {
  blockedReasonKey,
  presentAction,
  resourceDecision,
  type ResourceActionId,
} from "@/features/capabilities/resource-actions";
import { useWorkspace } from "@/features/workspaces/workspace-provider";
import {
  ServiceLifecycleController,
  type ServiceLifecycleAction,
  type ServiceLifecycleDecision,
  type ServiceLifecycleResource,
} from "./lifecycle";

const triggerDeploy = defineSafeAction("trigger-deploy", "service");
const cancelDeploy = defineSafeAction("cancel-deploy", "deploy");
const rollbackService = defineSafeAction("rollback-service", "deploy");
const restartService = defineSafeAction("restart-service", "service");
const suspendService = defineSafeAction("suspend-service", "service");
const resumeService = defineSafeAction("resume-service", "service");

export type MobileDeployActionTarget = {
  id: string;
  status: string;
  rollbackOf?: string | null;
};

export function ServiceActionsCard({
  service,
  deploys,
  refreshService,
  refreshDeploys,
}: {
  service: ServiceLifecycleResource;
  deploys: MobileDeployActionTarget[];
  refreshService: () => Promise<ServiceLifecycleResource | null>;
  refreshDeploys: () => Promise<MobileDeployActionTarget[]>;
}) {
  const { t } = useTranslations();
  const capabilities = useCapabilities();
  const { selected } = useWorkspace();
  const workspaceId = selected?.id ?? null;
  const canOperate = capabilities.allows("can_operate");
  // Authoritative per-resource decisions (ADR087 projections). Both must be
  // ready for this exact service before any option renders — denied actions
  // are absent, permitted-but-blocked actions stay visible with their reason,
  // and anything else fails closed to no options.
  const serverState = useServerActions(service.id);
  const deployState = useDeployActions(service.id);
  const serverSnapshot =
    serverState.status === "ready" ? serverState.snapshot : null;
  const deploySnapshot =
    deployState.status === "ready" ? deployState.snapshot : null;
  const decide = (action: ResourceActionId) =>
    serverSnapshot
      ? resourceDecision(serverSnapshot, workspaceId, service.id, action)
      : null;
  const decideDeploy = (action: ResourceActionId) =>
    deploySnapshot
      ? resourceDecision(deploySnapshot, workspaceId, service.id, action)
      : null;
  const blockedCopy = (action: ResourceActionId) => {
    const decision =
      action === "deploy" || action === "cancel_deploy" || action === "rollback"
        ? decideDeploy(action)
        : decide(action);
    const presentation = presentAction(decision);
    return presentation.kind === "blocked"
      ? t(blockedReasonKey(presentation.precondition))
      : undefined;
  };
  const serviceRef = useRef(service);
  const deploysRef = useRef(deploys);
  const refreshServiceRef = useRef(refreshService);
  const refreshDeploysRef = useRef(refreshDeploys);
  const serverSnapshotRef = useRef(serverSnapshot);
  const deploySnapshotRef = useRef(deploySnapshot);
  const workspaceIdRef = useRef(workspaceId);
  serviceRef.current = service;
  deploysRef.current = deploys;
  refreshServiceRef.current = refreshService;
  refreshDeploysRef.current = refreshDeploys;
  serverSnapshotRef.current = serverSnapshot;
  deploySnapshotRef.current = deploySnapshot;
  workspaceIdRef.current = workspaceId;

  const [suspend] = useMutation(MobileSuspendServiceDocument);
  const [resume] = useMutation(MobileResumeServiceDocument);
  const [restart] = useMutation(MobileRestartServiceDocument);
  const [trigger] = useMutation(MobileTriggerDeployDocument);
  const [cancel] = useMutation(MobileCancelDeployDocument);
  const [rollback] = useMutation(MobileRollbackServiceDocument);
  const mutationsRef = useRef({
    suspend,
    resume,
    restart,
    trigger,
    cancel,
    rollback,
  });
  mutationsRef.current = {
    suspend,
    resume,
    restart,
    trigger,
    cancel,
    rollback,
  };

  const lifecycleRef = useRef<ServiceLifecycleController | null>(null);
  if (!lifecycleRef.current) {
    lifecycleRef.current = new ServiceLifecycleController({
      mutate: {
        suspend: async (id, confirmation) => {
          await mutationsRef.current.suspend({
            variables: { id, confirm: confirmation },
          });
        },
        resume: async (id) => {
          await mutationsRef.current.resume({ variables: { id } });
        },
        restart: async (id) => {
          const result = await mutationsRef.current.restart({
            variables: { serviceId: id },
          });
          return { operationId: result.data?.restartServer?.id };
        },
      },
      refresh: (id) =>
        refreshServiceRef
          .current()
          .then((next) => (next?.id === id ? next : null)),
    });
  }

  const deployControllerRef = useRef<DeployActionController | null>(null);
  if (!deployControllerRef.current) {
    deployControllerRef.current = new DeployActionController({
      trigger: async (serviceId) => {
        const result = await mutationsRef.current.trigger({
          variables: { serviceId },
        });
        return { deploy: result.data?.triggerDeploy ?? null };
      },
      cancel: async (serviceId, deployId) => {
        const result = await mutationsRef.current.cancel({
          variables: { serviceId, deployId },
        });
        return { deploy: result.data?.cancelDeploy ?? null };
      },
      rollback: async (serviceId, deployId) => {
        const result = await mutationsRef.current.rollback({
          variables: { serviceId, deployId },
        });
        return { deploy: result.data?.rollbackService ?? null };
      },
    });
  }

  useEffect(() => {
    const deployController = deployControllerRef.current;
    return () => deployController?.clear();
  }, [service.id]);

  // ADR087 detail matrix: lifecycle/deploy/cancel require confirmed
  // can_operate; rollback's create-like class is probed server-side per
  // action, not from the workspace grant. On confirmed absence the whole
  // Actions card is absent — never an empty card, never a control that 403s
  // on tap. Presentation eligibility comes from the server projections;
  // deploy history only SELECTS the concrete cancel/rollback target row.
  if (!canOperate) {
    return null;
  }
  // Fail closed: without both projections for this exact service, no option
  // can claim server eligibility.
  if (!serverSnapshot || !deploySnapshot) {
    return null;
  }

  const options: MobileActionOption[] = [];
  const lifecycleActions = [
    { action: "restart", definition: restartService },
    { action: "suspend", definition: suspendService },
    { action: "resume", definition: resumeService },
  ] as const;
  for (const { action, definition } of lifecycleActions) {
    // Denied, unavailable, or missing rows are absent, never disabled.
    if (presentAction(decide(action)).kind === "hidden") continue;
    const blocked = blockedCopy(action);
    options.push({
      key: `service:${action}`,
      definition,
      target: { kind: "service", id: service.id, label: service.name },
      label: t(`safeActions.actions.${action}Service`),
      disabledReason: blocked,
      run: async (serverConfirmation) => {
        const outcome = await runServiceLifecycle(
          lifecycleRef.current!,
          action,
          serviceRef.current,
          serverConfirmation,
          lifecycleGateFor(action),
        );
        // Eligibility may have changed (suspend/resume flips every sibling
        // precondition); recheck both projections alongside the data refresh
        // the controller already performs.
        refreshDecisions();
        return outcome;
      },
    });
  }

  const triggerDecision = decideDeploy("deploy");
  if (presentAction(triggerDecision).kind !== "hidden") {
    options.unshift({
      key: "deploy:trigger",
      definition: triggerDeploy,
      target: { kind: "service", id: service.id, label: service.name },
      label: t("safeActions.actions.triggerDeploy"),
      disabledReason: blockedCopy("deploy"),
      run: (_serverConfirmation, retryIdentity) =>
        runDeployAction(
          deployControllerRef.current!,
          "trigger",
          serviceRef.current,
          undefined,
          retryIdentity!,
          deploysRef.current,
          refreshDeploysRef.current,
          refreshDecisions,
          deployGateFor("trigger"),
        ),
    });
  }

  const cancelDecision = decideDeploy("cancel_deploy");
  const cancelTarget = deploys.find(
    (deploy) => deploy.id && isCancelableDeployStatus(deploy.status),
  );
  // Cancel needs a concrete open row to name; the server answers only whether
  // one exists. Blocked-without-target (no_active_deploy) stays absent — the
  // terminal history itself is the explanation.
  if (
    cancelDecision !== null &&
    presentAction(cancelDecision).kind === "ready" &&
    cancelTarget
  ) {
    options.push(
      deployOption(
        cancelDeploy,
        "cancel",
        cancelTarget,
        t("safeActions.actions.cancelDeploy"),
        refreshDecisions,
      ),
    );
  }
  const rollbackDecision = decideDeploy("rollback");
  const rollbackTarget = deploys.find(
    (deploy) => deploy.id && isRollbackableDeployStatus(deploy.status),
  );
  if (presentAction(rollbackDecision).kind !== "hidden") {
    if (rollbackTarget) {
      options.push({
        ...deployOption(
          rollbackService,
          "rollback",
          rollbackTarget,
          t("safeActions.actions.rollbackDeploy"),
          refreshDecisions,
        ),
        disabledReason: blockedCopy("rollback"),
      });
    }
  }

  function refreshDecisions() {
    void Promise.all([serverState.refresh(), deployState.refresh()]).catch(
      () => undefined,
    );
  }

  // Confirm-time reads: the run closures below execute when the user confirms,
  // not when the option rendered, so they re-read the CURRENT projection
  // snapshot. A flipped outcome or precondition fails in the controller
  // instead of silently reusing the earlier confirmation.
  function lifecycleGateFor(
    action: ServiceLifecycleAction,
  ): ServiceLifecycleDecision | null {
    const snapshot = serverSnapshotRef.current;
    if (!snapshot) return null;
    const decision = resourceDecision(
      snapshot,
      workspaceIdRef.current,
      serviceRef.current.id,
      action,
    );
    return decision
      ? { outcome: decision.outcome, precondition: decision.precondition }
      : null;
  }

  function deployGateFor(action: "trigger" | "cancel" | "rollback") {
    const snapshot = deploySnapshotRef.current;
    if (!snapshot) return null;
    const decision = resourceDecision(
      snapshot,
      workspaceIdRef.current,
      serviceRef.current.id,
      action === "trigger"
        ? "deploy"
        : action === "cancel"
          ? "cancel_deploy"
          : "rollback",
    );
    return decision
      ? { outcome: decision.outcome, precondition: decision.precondition }
      : null;
  }

  function deployOption(
    definition: typeof cancelDeploy | typeof rollbackService,
    action: "cancel" | "rollback",
    target: MobileDeployActionTarget,
    label: string,
    refresh: () => void,
  ): MobileActionOption {
    return {
      key: `deploy:${action}:${target.id}`,
      definition,
      target: { kind: "deploy", id: target.id, label: target.id },
      label,
      run: (_serverConfirmation, retryIdentity) =>
        runDeployAction(
          deployControllerRef.current!,
          action,
          serviceRef.current,
          target,
          retryIdentity!,
          deploysRef.current,
          refreshDeploysRef.current,
          refresh,
          deployGateFor(action),
        ),
    };
  }

  return (
    <DashboardCard title={t("safeActions.cardTitle")}>
      <SafeActionPanel options={options} />
    </DashboardCard>
  );
}

async function runServiceLifecycle(
  controller: ServiceLifecycleController,
  action: ServiceLifecycleAction,
  resource: ServiceLifecycleResource,
  serverConfirmation: string | undefined,
  decision: ServiceLifecycleDecision | null,
): Promise<MobileActionRunResult> {
  const result = await controller.run({
    action,
    resource,
    confirmed: true,
    serverConfirmation,
    decision,
  });
  return mobileLifecycleResult(result);
}

async function runDeployAction(
  controller: DeployActionController,
  action: DeployAction,
  service: ServiceLifecycleResource,
  target: DeployTarget | undefined,
  requestId: string,
  before: MobileDeployActionTarget[],
  refresh: () => Promise<MobileDeployActionTarget[]>,
  refreshEligibility: () => void,
  server: DeployServerGate,
): Promise<MobileActionRunResult> {
  const request =
    action === "trigger"
      ? {
          action,
          requestId,
          serviceId: service.id,
          server,
        }
      : {
          action,
          requestId,
          serviceId: service.id,
          server,
          target: target!,
        };
  const result = await controller.execute(request);
  const after = await refresh().catch(() => []);
  // A successful or ambiguous action re-resolves every precondition (a new
  // deploy flips cancel/rollback; a suspension flips trigger/rollback).
  refreshEligibility();
  if (result.outcome === "accepted") {
    const observed = after.find((deploy) => deploy.id === result.deployId);
    if (observed) controller.markConverged(requestId, observed.status);
    return { status: "success" };
  }
  if (
    result.outcome === "unknown" &&
    deployOutcomeObserved(action, target, before, after)
  ) {
    return { status: "success" };
  }
  if (result.outcome === "unknown") return { status: "timeout" };
  return deployErrorResult(result.error);
}

function deployOutcomeObserved(
  action: DeployAction,
  target: DeployTarget | undefined,
  before: MobileDeployActionTarget[],
  after: MobileDeployActionTarget[],
): boolean {
  if (action === "cancel") {
    return after.some(
      (deploy) => deploy.id === target?.id && deploy.status === "canceled",
    );
  }
  if (action === "rollback") {
    return after.some((deploy) => deploy.rollbackOf === target?.id);
  }
  const beforeIds = new Set(before.map((deploy) => deploy.id));
  return after.some((deploy) => !beforeIds.has(deploy.id));
}

function deployErrorResult(error: DeployActionError): MobileActionRunResult {
  if (error.code === "conflict" || error.code === "not_found") {
    return { status: "not_allowed" };
  }
  if (error.code === "forbidden") {
    return {
      status: "error",
      error: Object.assign(new Error(error.message), { statusCode: 403 }),
    };
  }
  return {
    status: "error",
    error: Object.assign(new Error(error.message), { code: error.code }),
  };
}
