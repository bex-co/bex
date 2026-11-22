import { useCallback, useMemo } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import {
  DeployPostgresAdminAppDocument,
  PostgresAdminAppsDocument,
} from "@/graphql/definitions";
import { skipPollWhenHidden, useConvergingPoll } from "@/common/lib/polling";
import { usePaymentRequiredGate } from "@/features/usage/context/payment-required-context";
import {
  isConvergingPhase,
  toServiceView,
} from "@/features/services/lib/status";
import type { ServiceView } from "@/features/services/types";
import {
  isAdminAppId,
  type AdminAppId,
  type AdminAppCredentials,
} from "@/features/databases/lib/admin-apps";

/** One card of the Apps section: a catalog app and its deployed service. */
export interface AdminAppView {
  id: AdminAppId;
  name: string;
  plan: string;
  diskSizeGB: number | null;
  /** The billing lines' monthly prices, from bex-api's price sheet. */
  monthlyUsd: string;
  diskMonthlyUsd: string | null;
  /** The app's service for this database, null until one is deployed. */
  service: ServiceView | null;
}

export interface UsePostgresAdminAppsResult {
  apps: AdminAppView[];
  /** The database's project and environment (null when ungrouped). */
  projectId: string | null;
  environmentId: string | null;
  loading: boolean;
  error: Error | undefined;
  /**
   * Deploys app with the dialog's login. Opens billing onboarding on a 402
   * and retries; resolves the new service, or rejects with the refusal.
   */
  deploy: (
    app: AdminAppId,
    credentials: AdminAppCredentials,
  ) => Promise<{ id: string; name: string }>;
}

/**
 * Reads a database's Apps section (`postgresAdminApps`) and deploys an app
 * (`deployPostgresAdminApp`). bex-api builds the service from the database's
 * connection, so this never handles a connection string. A deployed app's
 * service is polled fast while it builds and rolls out, so its badge and Open
 * app follow the deploy as Render's card does.
 */
export function usePostgresAdminApps(
  databaseId: string,
): UsePostgresAdminAppsResult {
  const { data, loading, error, refetch, startPolling, stopPolling } = useQuery(
    PostgresAdminAppsDocument,
    {
      variables: { databaseId },
      fetchPolicy: "cache-and-network",
      errorPolicy: "all",
      skipPollAttempt: skipPollWhenHidden,
    },
  );
  const [mutate] = useMutation(DeployPostgresAdminAppDocument);
  const paymentGate = usePaymentRequiredGate();

  const apps = useMemo<AdminAppView[]>(
    () =>
      (data?.postgresAdminApps ?? []).flatMap((app) =>
        app && isAdminAppId(app.id)
          ? [
              {
                id: app.id,
                name: app.name ?? app.id,
                plan: app.plan ?? "",
                diskSizeGB: app.diskSizeGB ?? null,
                monthlyUsd: app.monthlyUsd ?? "",
                diskMonthlyUsd: app.diskMonthlyUsd ?? null,
                service: app.service ? toServiceView(app.service) : null,
              },
            ]
          : [],
      ),
    [data],
  );
  const converging = apps.some(
    (app) => app.service != null && isConvergingPhase(app.service),
  );
  useConvergingPoll(startPolling, stopPolling, converging);

  const deploy = useCallback(
    async (app: AdminAppId, credentials: AdminAppCredentials) => {
      const res = await paymentGate.run(() =>
        mutate({
          variables: {
            databaseId,
            app,
            username: credentials.username,
            password: credentials.password,
          },
        }),
      );
      const created = res.data?.deployPostgresAdminApp;
      if (!created?.id)
        throw new Error("deployPostgresAdminApp returned no id");
      await refetch();
      return { id: created.id, name: created.name ?? created.id };
    },
    [databaseId, mutate, paymentGate, refetch],
  );

  return {
    apps,
    projectId: data?.database?.projectId ?? null,
    environmentId: data?.database?.environmentId ?? null,
    loading: loading && data === undefined,
    error: data === undefined ? error : undefined,
    deploy,
  };
}
