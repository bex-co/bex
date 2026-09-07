import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useRef,
  type ReactNode,
} from "react";
import { AccessibilityInfo } from "react-native";
import { router } from "expo-router";
import { useApolloClient, useQuery } from "@apollo/client/react";
import { MobileViewerCapabilitiesDocument } from "@/generated-graphql";
import { useTranslations } from "@/common/hooks/use-translations";
import { resetWorkspaceBoundary } from "@/common/apollo/data-boundary";
import { useWorkspace } from "@/features/workspaces/workspace-provider";
import {
  allowsAction,
  checkingCapabilities,
  confirmedDenied,
  downgradeDetected,
  toSnapshot,
  unavailableCapabilities,
  type CapabilityAction,
  type CapabilityState,
} from "./capability-policy";

// ADR087 (w6/m138): the one fail-closed source of "what may I see and do in
// the selected workspace". Snapshots are in-memory only, keyed by workspace,
// re-evaluated on a 30s foreground cadence (the app's standard poll interval)
// and refreshed fresh (cache-bypassing) after a detected change. A confirmed
// same-workspace downgrade resets the data boundary — cached protected data,
// in-flight requests, and navigation history all go with it — and announces
// the change; a transport failure keeps the last resolved state and never
// reads as a role change.

const CAPABILITY_POLL_MS = 30_000;

type CapabilitiesContextValue = {
  state: CapabilityState;
  /** True only for a confirmed "allowed" grant in the selected workspace. */
  allows: (action: CapabilityAction) => boolean;
  /** True only for a confirmed "denied" grant (vs an unanswerable check). */
  denied: (action: CapabilityAction) => boolean;
};

const CapabilitiesContext = createContext<CapabilitiesContextValue>({
  state: checkingCapabilities,
  allows: () => false,
  denied: () => false,
});

export function useCapabilities(): CapabilitiesContextValue {
  return useContext(CapabilitiesContext);
}

export function CapabilitiesProvider({ children }: { children: ReactNode }) {
  const { selected } = useWorkspace();
  const { t } = useTranslations();
  const workspaceId = selected?.id ?? null;

  const client = useApolloClient();
  const { data, error } = useQuery(MobileViewerCapabilitiesDocument, {
    variables: { ownerId: workspaceId, fresh: false },
    skip: workspaceId === null,
    fetchPolicy: "network-only",
    pollInterval: CAPABILITY_POLL_MS,
    notifyOnNetworkStatusChange: false,
  });

  const state: CapabilityState = useMemo(() => {
    if (workspaceId === null) return checkingCapabilities;
    if (data?.viewerCapabilities) {
      return {
        status: "ready",
        snapshot: toSnapshot(
          workspaceId,
          data.viewerCapabilities.grants.map((grant) => ({
            action: grant.action,
            outcome: grant.outcome,
            reason: grant.reason ?? null,
          })),
        ),
      };
    }
    return error ? unavailableCapabilities : checkingCapabilities;
  }, [workspaceId, data, error]);

  // lastResolved keeps the newest READY state for the CURRENT workspace (one
  // slot — a switch starts the next workspace from checking, never from
  // another workspace's snapshot) so a transient failure mid-session neither
  // drops the tab set nor reads as a change.
  const lastResolved = useRef<
    { workspaceId: string; state: CapabilityState } | undefined
  >(undefined);
  const resolved: CapabilityState =
    state.status === "ready"
      ? state
      : lastResolved.current?.workspaceId === workspaceId &&
          state.status !== "checking"
        ? lastResolved.current.state
        : state;

  // Detect a CONFIRMED same-workspace downgrade: clear protected data through
  // the existing boundary (aborts in-flight work, clears the Apollo store,
  // drops navigation history via remount), return to Status, announce the
  // change, and re-evaluate fresh so the new shell renders from an
  // authoritative answer — not the replica cache the downgrade may still be
  // riding (ADR087 §Access changes).
  useEffect(() => {
    if (state.status !== "ready" || workspaceId === null) return;
    const previous =
      lastResolved.current?.workspaceId === workspaceId
        ? lastResolved.current.state
        : undefined;
    lastResolved.current = { workspaceId, state };
    if (previous && downgradeDetected(previous, state)) {
      AccessibilityInfo.announceForAccessibility(t("access.changed"));
      router.replace("/");
      // Reset first (aborts in-flight work, clears the store), THEN one-shot
      // a fresh evaluation via client.query — never through the hook's
      // refetch, whose variable merge would silently flip every later 30s
      // poll to fresh=true.
      void resetWorkspaceBoundary(workspaceId)
        .then(() =>
          client.query({
            query: MobileViewerCapabilitiesDocument,
            variables: { ownerId: workspaceId, fresh: true },
            fetchPolicy: "network-only",
          }),
        )
        .catch(() => undefined);
    }
  }, [state, workspaceId, client, t]);

  const value = useMemo<CapabilitiesContextValue>(
    () => ({
      state: resolved,
      allows: (action) => allowsAction(resolved, workspaceId, action),
      denied: (action) => confirmedDenied(resolved, workspaceId, action),
    }),
    [resolved, workspaceId],
  );

  return (
    <CapabilitiesContext.Provider value={value}>
      {children}
    </CapabilitiesContext.Provider>
  );
}
