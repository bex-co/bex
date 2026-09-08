import { useRef } from "react";
import { NetworkStatus } from "@apollo/client";
import { useMutation, useQuery } from "@apollo/client/react";
import { hasAuthoritativeCurrentEvidence } from "@/common/apollo/authoritative-evidence";
import {
  defineSafeAction,
  mobileLifecycleResult,
  type MobileActionOption,
  type MobileActionRunResult,
} from "@/components/safe-action";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  MobilePostgresLifecycleDocument,
  MobileRestartPostgresDocument,
  MobileResumePostgresDocument,
  MobileSuspendPostgresDocument,
} from "@/generated-graphql";
import { DatastoreDetailLayout } from "@/features/resources/datastore-detail-layout";
import { useDatabaseActions } from "@/features/capabilities/api/use-resource-actions";
import {
  blockedReasonKey,
  presentAction,
  resourceDecision,
} from "@/features/capabilities/resource-actions";
import { useWorkspace } from "@/features/workspaces/workspace-provider";
import {
  PostgresLifecycleController,
  type PostgresLifecycleAction,
  type PostgresLifecycleDecision,
  type PostgresLifecycleResource,
} from "./lifecycle";
import {
  PostgresInsightsCard,
  type PostgresInsightsCardHandle,
} from "./postgres-insights-card";

const restartDatabase = defineSafeAction("restart-database", "database");
const suspendDatabase = defineSafeAction("suspend-database", "database");
const resumeDatabase = defineSafeAction("resume-database", "database");

export function PostgresDetailScreen({ databaseId }: { databaseId: string }) {
  const { t } = useTranslations();
  const { selected } = useWorkspace();
  const workspaceId = selected?.id ?? null;
  // Authoritative lifecycle eligibility (databaseActions): the server's
  // protected-environment and billing preconditions replace the local
  // status/suspended presentation predicates.
  const actionsState = useDatabaseActions(databaseId);
  const actionsSnapshot =
    actionsState.status === "ready" ? actionsState.snapshot : null;
  const query = useQuery(MobilePostgresLifecycleDocument, {
    variables: { id: databaseId },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    notifyOnNetworkStatusChange: true,
  });
  const [suspend] = useMutation(MobileSuspendPostgresDocument);
  const [resume] = useMutation(MobileResumePostgresDocument);
  const [restart] = useMutation(MobileRestartPostgresDocument);
  const mutationsRef = useRef({ suspend, resume, restart });
  mutationsRef.current = { suspend, resume, restart };
  const queryRef = useRef(query);
  queryRef.current = query;
  const insightsRef = useRef<PostgresInsightsCardHandle>(null);
  const controllerRef = useRef<PostgresLifecycleController | null>(null);
  if (!controllerRef.current) {
    controllerRef.current = new PostgresLifecycleController({
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
          await mutationsRef.current.restart({ variables: { id } });
        },
      },
      refresh: async (id) => {
        const result = await queryRef.current.refetch();
        const resource = result.data?.database;
        return resource?.id === id ? postgresResource(resource) : null;
      },
    });
  }

  const database = query.data?.database;
  const resource = database?.id ? postgresResource(database) : null;
  // Confirm-time reads: the run closures below execute when the user confirms,
  // not when the option rendered, so they re-read the CURRENT projection
  // snapshot. A flipped outcome or precondition fails in the controller
  // instead of silently reusing the earlier confirmation.
  const resourceRef = useRef(resource);
  resourceRef.current = resource;
  const actionsSnapshotRef = useRef(actionsSnapshot);
  actionsSnapshotRef.current = actionsSnapshot;
  const workspaceIdRef = useRef(workspaceId);
  workspaceIdRef.current = workspaceId;
  function gateFor(
    action: PostgresLifecycleAction,
  ): PostgresLifecycleDecision | null {
    const snapshot = actionsSnapshotRef.current;
    const current = resourceRef.current;
    if (!snapshot || !current) return null;
    const decision = resourceDecision(
      snapshot,
      workspaceIdRef.current,
      current.id,
      action,
    );
    return decision
      ? { outcome: decision.outcome, precondition: decision.precondition }
      : null;
  }
  const hasCurrentEvidence = hasAuthoritativeCurrentEvidence({
    networkStatus: query.networkStatus,
    error: query.error,
    hasData: Boolean(resource),
  });
  // Fail closed: without the projection for this exact database, no option
  // can claim server eligibility. Denied actions are absent; permitted-but-
  // blocked actions stay visible with their reason. The protected-environment
  // precondition stays an enabled option — the safe-action dialog presents
  // the server phrase as the explicit second confirmation step.
  const postgresDefinitions = {
    restart: restartDatabase,
    suspend: suspendDatabase,
    resume: resumeDatabase,
  } as const;
  const options: MobileActionOption[] =
    resource && hasCurrentEvidence && actionsSnapshot
      ? (Object.keys(postgresDefinitions) as PostgresLifecycleAction[]).flatMap(
          (action) => {
            const decision = resourceDecision(
              actionsSnapshot,
              workspaceId,
              resource.id,
              action,
            );
            const presentation = presentAction(decision);
            if (presentation.kind === "hidden") return [];
            const blocked =
              presentation.kind === "blocked"
                ? t(blockedReasonKey(presentation.precondition))
                : undefined;
            return [
              {
                key: action,
                definition: postgresDefinitions[action],
                target: {
                  kind: "database" as const,
                  id: resource.id,
                  label: resource.name,
                },
                label: t(`safeActions.actions.${action}Database`),
                disabledReason: blocked,
                run: async (serverConfirmation?: string) => {
                  const current = resourceRef.current ?? resource;
                  const outcome = await runPostgresAction(
                    controllerRef.current!,
                    action,
                    current,
                    serverConfirmation,
                    gateFor(action),
                  );
                  void actionsState.refresh().catch(() => undefined);
                  return outcome;
                },
              },
            ];
          },
        )
      : [];

  const status = resource
    ? resource.suspended === "suspended"
      ? "suspended"
      : resource.status
    : t("resources.unknownStatus");
  return (
    <DatastoreDetailLayout
      title={resource?.name || t("datastores.postgres")}
      subtitle={
        database?.version
          ? `PostgreSQL ${database.version}`
          : t("datastores.postgres")
      }
      status={status}
      details={[
        { label: t("datastores.plan"), value: database?.plan },
        { label: t("datastores.region"), value: database?.region },
        { label: t("datastores.id"), value: database?.id, mono: true },
      ]}
      loading={query.loading && !database}
      error={Boolean(query.error)}
      refreshing={query.networkStatus === NetworkStatus.refetch}
      onRefresh={() =>
        void Promise.all([
          query.refetch(),
          insightsRef.current?.refresh() ?? Promise.resolve(),
          actionsState.refresh(),
        ])
      }
      options={options}
    >
      {database?.id ? (
        <PostgresInsightsCard ref={insightsRef} databaseId={database.id} />
      ) : null}
    </DatastoreDetailLayout>
  );
}

function postgresResource(resource: {
  id?: string | null;
  name?: string | null;
  status?: string | null;
  suspended?: string | null;
  updatedAt?: string | null;
}): PostgresLifecycleResource {
  return {
    id: resource.id ?? "",
    name: resource.name || resource.id || "Postgres",
    status: resource.status || "unknown",
    suspended: resource.suspended ?? null,
    updatedAt: resource.updatedAt,
  };
}

async function runPostgresAction(
  controller: PostgresLifecycleController,
  action: PostgresLifecycleAction,
  resource: PostgresLifecycleResource,
  serverConfirmation: string | undefined,
  decision: PostgresLifecycleDecision | null,
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
