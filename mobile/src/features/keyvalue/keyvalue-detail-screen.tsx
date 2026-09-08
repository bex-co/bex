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
  MobileKeyValueLifecycleDocument,
  MobileResumeKeyValueDocument,
  MobileSuspendKeyValueDocument,
} from "@/generated-graphql";
import { DatastoreDetailLayout } from "@/features/resources/datastore-detail-layout";
import { useKeyValueActions } from "@/features/capabilities/api/use-resource-actions";
import {
  blockedReasonKey,
  presentAction,
  resourceDecision,
} from "@/features/capabilities/resource-actions";
import { useWorkspace } from "@/features/workspaces/workspace-provider";
import {
  KeyValueLifecycleController,
  type KeyValueLifecycleAction,
  type KeyValueLifecycleDecision,
  type KeyValueLifecycleResource,
} from "./lifecycle";
import {
  KeyValueInsightsCard,
  type KeyValueInsightsCardHandle,
} from "./keyvalue-insights-card";

const suspendKeyValue = defineSafeAction("suspend-key-value", "key-value");
const resumeKeyValue = defineSafeAction("resume-key-value", "key-value");

export function KeyValueDetailScreen({ keyValueId }: { keyValueId: string }) {
  const { t } = useTranslations();
  const { selected } = useWorkspace();
  const workspaceId = selected?.id ?? null;
  // Authoritative lifecycle eligibility (keyValueActions) — same shared
  // semantics as Postgres: server preconditions replace the local
  // status/suspended presentation predicates.
  const actionsState = useKeyValueActions(keyValueId);
  const actionsSnapshot =
    actionsState.status === "ready" ? actionsState.snapshot : null;
  const query = useQuery(MobileKeyValueLifecycleDocument, {
    variables: { id: keyValueId },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    notifyOnNetworkStatusChange: true,
  });
  const [suspend] = useMutation(MobileSuspendKeyValueDocument);
  const [resume] = useMutation(MobileResumeKeyValueDocument);
  const mutationsRef = useRef({ suspend, resume });
  mutationsRef.current = { suspend, resume };
  const queryRef = useRef(query);
  queryRef.current = query;
  const insightsRef = useRef<KeyValueInsightsCardHandle>(null);
  const controllerRef = useRef<KeyValueLifecycleController | null>(null);
  if (!controllerRef.current) {
    controllerRef.current = new KeyValueLifecycleController({
      mutate: {
        suspend: async (id, confirmation) => {
          await mutationsRef.current.suspend({
            variables: { id, confirm: confirmation },
          });
        },
        resume: async (id) => {
          await mutationsRef.current.resume({ variables: { id } });
        },
      },
      refresh: async (id) => {
        const result = await queryRef.current.refetch();
        const resource = result.data?.keyValue;
        return resource?.id === id ? keyValueResource(resource) : null;
      },
    });
  }

  const keyValue = query.data?.keyValue;
  const resource = keyValue?.id ? keyValueResource(keyValue) : null;
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
    action: KeyValueLifecycleAction,
  ): KeyValueLifecycleDecision | null {
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
  // Fail closed: without the projection for this exact store, no option can
  // claim server eligibility. Key Value has no restart verb — the projection
  // never invents one, and neither does this screen.
  const keyValueDefinitions = {
    suspend: suspendKeyValue,
    resume: resumeKeyValue,
  } as const;
  const options: MobileActionOption[] =
    resource && hasCurrentEvidence && actionsSnapshot
      ? (Object.keys(keyValueDefinitions) as KeyValueLifecycleAction[]).flatMap(
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
                definition: keyValueDefinitions[action],
                target: {
                  kind: "key-value" as const,
                  id: resource.id,
                  label: resource.name,
                },
                label: t(`safeActions.actions.${action}KeyValue`),
                disabledReason: blocked,
                run: async (serverConfirmation?: string) => {
                  const current = resourceRef.current ?? resource;
                  const outcome = await runKeyValueAction(
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
      title={resource?.name || t("datastores.keyValue")}
      subtitle={
        keyValue?.version
          ? `Valkey ${keyValue.version}`
          : t("datastores.keyValue")
      }
      status={status}
      details={[
        { label: t("datastores.plan"), value: keyValue?.plan },
        { label: t("datastores.region"), value: keyValue?.region },
        { label: t("datastores.id"), value: keyValue?.id, mono: true },
      ]}
      loading={query.loading && !keyValue}
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
      {keyValue?.id ? (
        <KeyValueInsightsCard
          ref={insightsRef}
          resourceId={keyValue.id}
          status={keyValue.status}
          suspended={keyValue.suspended}
        />
      ) : null}
    </DatastoreDetailLayout>
  );
}

function keyValueResource(resource: {
  id?: string | null;
  name?: string | null;
  status?: string | null;
  suspended?: string | null;
  updatedAt?: string | null;
}): KeyValueLifecycleResource {
  return {
    id: resource.id ?? "",
    name: resource.name || resource.id || "Key Value",
    status: resource.status || "unknown",
    suspended: resource.suspended ?? null,
    updatedAt: resource.updatedAt,
  };
}

async function runKeyValueAction(
  controller: KeyValueLifecycleController,
  action: KeyValueLifecycleAction,
  resource: KeyValueLifecycleResource,
  serverConfirmation: string | undefined,
  decision: KeyValueLifecycleDecision | null,
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
