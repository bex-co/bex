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
  type DeployTarget,
} from "@/features/deploys/deploy-actions";
import {
  ServiceLifecycleController,
  isLifecycleSuspended,
  serviceLifecycleCapabilities,
  type ServiceLifecycleAction,
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
  const serviceRef = useRef(service);
  const deploysRef = useRef(deploys);
  const refreshServiceRef = useRef(refreshService);
  const refreshDeploysRef = useRef(refreshDeploys);
  serviceRef.current = service;
  deploysRef.current = deploys;
  refreshServiceRef.current = refreshService;
  refreshDeploysRef.current = refreshDeploys;

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

  const options: MobileActionOption[] = [];
  for (const capability of serviceLifecycleCapabilities(service)) {
    const definition =
      capability.action === "restart"
        ? restartService
        : capability.action === "suspend"
          ? suspendService
          : resumeService;
    options.push({
      key: `service:${capability.action}`,
      definition,
      target: { kind: "service", id: service.id, label: service.name },
      label: t(`safeActions.actions.${capability.action}Service`),
      run: (serverConfirmation) =>
        runServiceLifecycle(
          lifecycleRef.current!,
          capability.action,
          serviceRef.current,
          serverConfirmation,
        ),
    });
  }

  if (!isLifecycleSuspended(service.suspended)) {
    options.unshift({
      key: "deploy:trigger",
      definition: triggerDeploy,
      target: { kind: "service", id: service.id, label: service.name },
      label: t("safeActions.actions.triggerDeploy"),
      run: (_serverConfirmation, retryIdentity) =>
        runDeployAction(
          deployControllerRef.current!,
          "trigger",
          serviceRef.current,
          undefined,
          retryIdentity!,
          deploysRef.current,
          refreshDeploysRef.current,
        ),
    });
  }

  const cancelTarget = deploys.find(
    (deploy) => deploy.id && isCancelableDeployStatus(deploy.status),
  );
  if (cancelTarget) {
    options.push(
      deployOption(
        cancelDeploy,
        "cancel",
        cancelTarget,
        t("safeActions.actions.cancelDeploy"),
      ),
    );
  }
  const rollbackTargets = deploys
    .filter((deploy) => deploy.id && isRollbackableDeployStatus(deploy.status))
    .slice(0, 1);
  for (const target of rollbackTargets) {
    options.push(
      deployOption(
        rollbackService,
        "rollback",
        target,
        t("safeActions.actions.rollbackDeploy"),
      ),
    );
  }

  function deployOption(
    definition: typeof cancelDeploy | typeof rollbackService,
    action: "cancel" | "rollback",
    target: MobileDeployActionTarget,
    label: string,
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
  serverConfirmation?: string,
): Promise<MobileActionRunResult> {
  const result = await controller.run({
    action,
    resource,
    confirmed: true,
    serverConfirmation,
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
): Promise<MobileActionRunResult> {
  const request =
    action === "trigger"
      ? {
          action,
          requestId,
          serviceId: service.id,
          serviceSuspended: isLifecycleSuspended(service.suspended),
        }
      : {
          action,
          requestId,
          serviceId: service.id,
          serviceSuspended: isLifecycleSuspended(service.suspended),
          target: target!,
        };
  const result = await controller.execute(request);
  const after = await refresh().catch(() => []);
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
