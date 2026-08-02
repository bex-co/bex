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
  MobileKeyValueLifecycleDocument,
  MobileResumeKeyValueDocument,
  MobileSuspendKeyValueDocument,
} from "@/generated-graphql";
import { DatastoreDetailLayout } from "@/features/resources/datastore-detail-layout";
import {
  KeyValueLifecycleController,
  keyValueLifecycleCapabilities,
  type KeyValueLifecycleAction,
  type KeyValueLifecycleResource,
} from "./lifecycle";

const suspendKeyValue = defineSafeAction("suspend-key-value", "key-value");
const resumeKeyValue = defineSafeAction("resume-key-value", "key-value");

export function KeyValueDetailScreen({ keyValueId }: { keyValueId: string }) {
  const { t } = useTranslations();
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
  const options: MobileActionOption[] = resource
    ? keyValueLifecycleCapabilities(resource).map((capability) => {
        const definition =
          capability.action === "suspend" ? suspendKeyValue : resumeKeyValue;
        return {
          key: capability.action,
          definition,
          target: {
            kind: "key-value" as const,
            id: resource.id,
            label: resource.name,
          },
          label: t(`safeActions.actions.${capability.action}KeyValue`),
          run: (serverConfirmation?: string) =>
            runKeyValueAction(
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
      title={resource?.name ?? t("datastores.keyValue")}
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
      onRefresh={() => void query.refetch()}
      options={options}
    />
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
    name: resource.name ?? resource.id ?? "Key Value",
    status: resource.status ?? "unknown",
    suspended: resource.suspended ?? null,
    updatedAt: resource.updatedAt,
  };
}

async function runKeyValueAction(
  controller: KeyValueLifecycleController,
  action: KeyValueLifecycleAction,
  resource: KeyValueLifecycleResource,
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
