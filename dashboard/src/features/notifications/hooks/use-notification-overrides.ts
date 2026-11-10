import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { PRIMED_FETCH_POLICY } from "@/common/lib/fetch-policy";
import { NotificationOverridesDocument } from "@/graphql/definitions";

export interface NotificationOverride {
  id: string;
  name: string;
  type: string;
  /** "all" | "failure" | "none" — never "default", which is not an override. */
  notificationsToSend: string;
}

export interface UseNotificationOverridesResult {
  overrides: NotificationOverride[];
  /** Loading with no data yet — the skeleton gate (dashboard/AGENTS.md). */
  loading: boolean;
  error: Error | undefined;
}

/**
 * The workspace's services whose notification setting overrides the
 * workspace default, by name (w3/m305): Render's Notification Overrides list.
 */
export function useNotificationOverrides(
  ownerId: string,
): UseNotificationOverridesResult {
  const { data, loading, error } = useQuery(NotificationOverridesDocument, {
    variables: { ownerId },
    // SetNotificationsToSend returns the service's id and value, so the cache
    // stays current after an edit without a refetch on every visit.
    fetchPolicy: PRIMED_FETCH_POLICY,
  });
  const overrides = useMemo(
    () =>
      (data?.services ?? [])
        .flatMap((service) =>
          service?.id &&
          service.notificationsToSend &&
          service.notificationsToSend !== "default"
            ? [
                {
                  id: service.id,
                  name: service.displayName || service.name || service.id,
                  type: service.type ?? "",
                  notificationsToSend: service.notificationsToSend,
                },
              ]
            : [],
        )
        .sort((a, b) => a.name.localeCompare(b.name)),
    [data],
  );
  return { overrides, loading: loading && !data, error };
}
