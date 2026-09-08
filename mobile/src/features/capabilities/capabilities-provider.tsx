import {
  createContext,
  useContext,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
  type ReactNode,
} from "react";
import { AccessibilityInfo, AppState } from "react-native";
import { installAccessCheck } from "@/common/apollo/access-link";
import { recoveryAvailable } from "@/common/hooks/recovery-coordinator";
import { router } from "expo-router";
import { useApolloClient } from "@apollo/client/react";
import { MobileViewerCapabilitiesDocument } from "@/generated-graphql";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  dataBoundary,
  resetWorkspaceBoundary,
} from "@/common/apollo/data-boundary";
import {
  useRecovery,
  useRecoveryEnvironment,
} from "@/common/hooks/use-recovery";
import { useWorkspace } from "@/features/workspaces/workspace-provider";
import {
  allowsAction,
  CAPABILITY_ACTIONS,
  CAPABILITY_FRESHNESS_MS,
  checkingCapabilities,
  confirmedDenied,
  downgradeDetected,
  snapshotIsFresh,
  toSnapshot,
  unavailableCapabilities,
  type CapabilityAction,
  type CapabilityState,
} from "./capability-policy";

type CapabilitiesContextValue = {
  state: CapabilityState;
  /** Protected work requires a fresh affirmative grant. */
  allows: (action: CapabilityAction) => boolean;
  /** Shell visibility only. Never authorizes reads or operations. */
  shows: (action: CapabilityAction) => boolean;
  generation: number;
  offline: boolean;
  retry: () => Promise<unknown>;
  denied: (action: CapabilityAction) => boolean;
};

const CapabilitiesContext = createContext<CapabilitiesContextValue>({
  state: checkingCapabilities,
  allows: () => false,
  shows: () => false,
  generation: 0,
  offline: false,
  retry: async () => undefined,
  denied: () => false,
});

export function useCapabilities(): CapabilitiesContextValue {
  return useContext(CapabilitiesContext);
}

export function CapabilitiesProvider({ children }: { children: ReactNode }) {
  const { selected, refreshMembership } = useWorkspace();
  const workspaceId = selected?.id ?? null;
  const { t } = useTranslations();
  const client = useApolloClient();
  const environment = useRecoveryEnvironment();
  const available = recoveryAvailable(environment);
  const generation = useSyncExternalStore(
    dataBoundary.subscribe,
    dataBoundary.getGeneration,
    dataBoundary.getGeneration,
  );
  const [state, setState] = useState<CapabilityState>(checkingCapabilities);
  const [clock, setClock] = useState(Date.now);
  const lastResolved = useRef<CapabilityState>(checkingCapabilities);
  // Dispatch can occur between a recovery event and React's next commit.
  const eligibility = useRef({ state: checkingCapabilities, generation: -1 });
  const currentWorkspace = useRef(workspaceId);
  currentWorkspace.current = workspaceId;

  const recovery = useRecovery({
    maxAttempts: 1,
    attempt: async ({ signal, reason }) => {
      if (!workspaceId) {
        if (reason !== "poll") await refreshMembership();
        return;
      }
      const previousEligibility = eligibility.current;
      const fresh =
        previousEligibility.generation === dataBoundary.getGeneration() &&
        previousEligibility.state.status === "ready" &&
        snapshotIsFresh(previousEligibility.state.snapshot);
      if (reason !== "poll" || !fresh) {
        eligibility.current = { state: checkingCapabilities, generation: -1 };
        setState(checkingCapabilities);
      }
      const lease = dataBoundary.begin();
      const current = () =>
        !signal.aborted &&
        lease.isCurrent() &&
        currentWorkspace.current === workspaceId;
      try {
        if (reason !== "poll") {
          const verified = await refreshMembership();
          if (!current() || verified?.id !== workspaceId) return;
        }
        const result = await client.query({
          query: MobileViewerCapabilitiesDocument,
          variables: {
            ownerId: workspaceId,
            fresh: reason !== "poll" || lastResolved.current.status !== "ready",
          },
          // A successful response renews receipt time even for identical grants.
          // Apollo's structural sharing must not stand in for response freshness.
          fetchPolicy: "no-cache",
          context: { fetchOptions: { signal } },
        });
        if (!current()) return;
        if (!result.data?.viewerCapabilities)
          throw new Error("capability check unavailable");
        const next: CapabilityState = {
          status: "ready",
          snapshot: toSnapshot(
            workspaceId,
            result.data.viewerCapabilities.grants,
          ),
        };
        const previous = lastResolved.current;
        const changed =
          previous.status === "ready" &&
          previous.snapshot.workspaceId === workspaceId &&
          CAPABILITY_ACTIONS.some((action) => {
            const before = previous.snapshot.grants[action];
            const after = next.snapshot.grants[action];
            return (
              (before === "allowed" || before === "denied") &&
              (after === "allowed" || after === "denied") &&
              before !== after
            );
          });
        if (changed) {
          if (downgradeDetected(previous, next)) {
            AccessibilityInfo.announceForAccessibility(t("access.changed"));
          }
          const reset = resetWorkspaceBoundary(workspaceId);
          const resetGeneration = dataBoundary.getGeneration();
          await reset;
          if (
            signal.aborted ||
            currentWorkspace.current !== workspaceId ||
            dataBoundary.getGeneration() !== resetGeneration
          )
            return;
          router.replace("/");
        }
        const navigationGrants =
          previous.status === "ready" &&
          previous.snapshot.workspaceId === workspaceId
            ? { ...previous.snapshot.grants }
            : {};
        for (const action of CAPABILITY_ACTIONS) {
          const outcome = next.snapshot.grants[action];
          if (outcome === "allowed" || outcome === "denied")
            navigationGrants[action] = outcome;
        }
        lastResolved.current = {
          status: "ready",
          snapshot: { ...next.snapshot, grants: navigationGrants },
        };
        eligibility.current = {
          state: next,
          generation: dataBoundary.getGeneration(),
        };
        setState(next);
      } catch (error) {
        if (current()) {
          eligibility.current = {
            state: unavailableCapabilities,
            generation: -1,
          };
          setState(unavailableCapabilities);
        }
        throw error;
      } finally {
        lease.finish();
      }
    },
  });

  useEffect(() => {
    lastResolved.current = checkingCapabilities;
    eligibility.current = { state: checkingCapabilities, generation: -1 };
    setState(checkingCapabilities);
    recovery.cancel();
    if (workspaceId) void recovery.poll();
    return recovery.cancel;
  }, [workspaceId, recovery.cancel, recovery.poll]);

  useEffect(() => {
    if (!available) {
      eligibility.current = { state: checkingCapabilities, generation: -1 };
      setState(checkingCapabilities);
      return;
    }
    const interval = setInterval(
      () => void recovery.poll(),
      CAPABILITY_FRESHNESS_MS - 5_000,
    );
    return () => clearInterval(interval);
  }, [available, recovery.poll]);

  useEffect(() => {
    if (state.status !== "ready") return;
    const timeout = setTimeout(
      () => setClock(Date.now()),
      Math.max(
        0,
        state.snapshot.receivedAt + CAPABILITY_FRESHNESS_MS - Date.now(),
      ),
    );
    return () => clearTimeout(timeout);
  }, [state]);

  const resolved = !available
    ? unavailableCapabilities
    : state.status === "ready" &&
        (state.snapshot.workspaceId !== workspaceId ||
          clock - state.snapshot.receivedAt >= CAPABILITY_FRESHNESS_MS)
      ? checkingCapabilities
      : state;
  const navigation = lastResolved.current;
  const value = useMemo<CapabilitiesContextValue>(
    () => ({
      state: resolved,
      generation,
      offline: environment.connectivity !== "online",
      retry: recovery.manualRetry,
      shows: (action) =>
        navigation.status === "ready" &&
        navigation.snapshot.workspaceId === workspaceId &&
        navigation.snapshot.grants[action] === "allowed",
      allows: (action) =>
        available &&
        AppState.currentState === "active" &&
        eligibility.current.generation === dataBoundary.getGeneration() &&
        allowsAction(eligibility.current.state, workspaceId, action),
      denied: (action) => confirmedDenied(resolved, workspaceId, action),
    }),
    [
      resolved,
      navigation,
      workspaceId,
      available,
      generation,
      environment.connectivity,
      recovery.manualRetry,
    ],
  );

  useLayoutEffect(() => installAccessCheck(value.allows), [value]);

  return (
    <CapabilitiesContext.Provider value={value}>
      {children}
    </CapabilitiesContext.Provider>
  );
}
