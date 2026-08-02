import { useRef } from "react";
import { NetworkStatus } from "@apollo/client";
import { useMutation, useQuery } from "@apollo/client/react";
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
import {
  PostgresLifecycleController,
  postgresLifecycleCapabilities,
  type PostgresLifecycleAction,
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
  const options: MobileActionOption[] = resource
    ? postgresLifecycleCapabilities(resource).map((capability) => {
        const definition =
          capability.action === "restart"
            ? restartDatabase
            : capability.action === "suspend"
              ? suspendDatabase
              : resumeDatabase;
        return {
          key: capability.action,
          definition,
          target: {
            kind: "database" as const,
            id: resource.id,
            label: resource.name,
          },
          label: t(`safeActions.actions.${capability.action}Database`),
          run: (serverConfirmation?: string) =>
            runPostgresAction(
              controllerRef.current!,
              capability.action,
              resource,
              serverConfirmation,
            ),
        };
      })
    : [];

  const status = resource
    ? resource.suspended === "suspended"
      ? "suspended"
      : resource.status
    : t("resources.unknownStatus");
  return (
    <DatastoreDetailLayout
      title={resource?.name ?? t("datastores.postgres")}
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
    name: resource.name ?? resource.id ?? "Postgres",
    status: resource.status ?? "unknown",
    suspended: resource.suspended ?? null,
    updatedAt: resource.updatedAt,
  };
}

async function runPostgresAction(
  controller: PostgresLifecycleController,
  action: PostgresLifecycleAction,
  resource: PostgresLifecycleResource,
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
