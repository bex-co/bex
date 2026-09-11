import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
  type ReactNode,
} from "react";
import { useApolloClient } from "@apollo/client/react";
import { ViewerCapabilitiesDocument } from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import {
  getAccessGeneration,
  bumpAccessGeneration,
  subscribeAccessGeneration,
} from "@/features/capabilities/lib/access-generation";
import {
  CAPABILITY_ACTIONS,
  CAPABILITY_FRESHNESS_MS,
  allowsAction,
  checkingCapabilities,
  confirmedDenied,
  downgradeDetected,
  grantReasonKey,
  snapshotIsFresh,
  toSnapshot,
  unavailableCapabilities,
  type CapabilityAction,
  type CapabilityState,
  type CapabilityBoolean,
  actionForBoolean,
} from "@/features/capabilities/lib/capability-policy";

export interface Capabilities {
  /** The caller's UPPERCASE workspace role, or null when unresolved. */
  role: string | null;
  /** Fail-closed: true only on affirmative allowed + fresh for this workspace. */
  canView: boolean;
  canViewLogs: boolean;
  canOperate: boolean;
  canCreate: boolean;
  canViewSensitive: boolean;
  canManageKeys: boolean;
  canManage: boolean;
  canManageBilling: boolean;
  /** A fresh check is in flight (or no workspace yet). */
  loading: boolean;
  /**
   * A definitive ready+fresh answer for the current workspace. Use with
   * `!canX` for confirmed role-denial copy; never treat unavailable/stale as
   * loaded.
   */
  loaded: boolean;
  /** Receipt older than the freshness bound — actions must not stay enabled. */
  stale: boolean;
  /** Transport / authz-service failure — not a confirmed role downgrade. */
  unavailable: boolean;
  /** Identity/workspace/access generation; obsolete responses must ignore. */
  generation: number;
  allows: (action: CapabilityAction) => boolean;
  denied: (action: CapabilityAction) => boolean;
  /** Locale key for disable-with-reason / recovery copy, or undefined. */
  reasonKey: (action: CapabilityAction) => string | undefined;
  /** Re-run a fresh evaluation (focus/reconnect/manual recovery). */
  refresh: () => Promise<void>;
}

type RefreshReason = "poll" | "focus" | "reconnect" | "manual" | "workspace";

const CapabilitiesContext = createContext<Capabilities | null>(null);

function booleansFrom(
  state: CapabilityState,
  workspaceId: string | null,
  now: number,
): Pick<
  Capabilities,
  | "canView"
  | "canViewLogs"
  | "canOperate"
  | "canCreate"
  | "canViewSensitive"
  | "canManageKeys"
  | "canManage"
  | "canManageBilling"
> {
  const flag = (key: CapabilityBoolean) =>
    allowsAction(state, workspaceId, actionForBoolean(key), now);
  return {
    canView: flag("canView"),
    canViewLogs: flag("canViewLogs"),
    canOperate: flag("canOperate"),
    canCreate: flag("canCreate"),
    canViewSensitive: flag("canViewSensitive"),
    canManageKeys: flag("canManageKeys"),
    canManage: flag("canManage"),
    canManageBilling: flag("canManageBilling"),
  };
}

/**
 * One coordinated capability refresh lifecycle for the dashboard (w6/m144).
 * Polls while visible, refreshes on focus/reconnect with fresh=true, and
 * expires action/sensitive eligibility after 30s even if the next read fails.
 */
export function CapabilitiesProvider({ children }: { children: ReactNode }) {
  const { currentWorkspaceId } = useWorkspace();
  const client = useApolloClient();
  const generation = useSyncExternalStore(
    subscribeAccessGeneration,
    getAccessGeneration,
    getAccessGeneration,
  );

  const [state, setState] = useState<CapabilityState>(checkingCapabilities);
  const [clock, setClock] = useState(() => Date.now());
  const lastResolved = useRef<CapabilityState>(checkingCapabilities);
  const eligibility = useRef<{ state: CapabilityState; generation: number }>({
    state: checkingCapabilities,
    generation: -1,
  });
  const workspaceRef = useRef(currentWorkspaceId);
  workspaceRef.current = currentWorkspaceId;
  const inFlight = useRef<AbortController | null>(null);
  const refreshRef = useRef<(reason: RefreshReason) => Promise<void>>(
    async () => undefined,
  );

  const refresh = useCallback(
    async (reason: RefreshReason) => {
      const workspaceId = workspaceRef.current;
      if (!workspaceId) {
        lastResolved.current = checkingCapabilities;
        eligibility.current = { state: checkingCapabilities, generation: -1 };
        setState(checkingCapabilities);
        return;
      }

      const previousEligibility = eligibility.current;
      const stillFresh =
        previousEligibility.generation === getAccessGeneration() &&
        previousEligibility.state.status === "ready" &&
        snapshotIsFresh(previousEligibility.state.snapshot);

      if (reason !== "poll" || !stillFresh) {
        eligibility.current = { state: checkingCapabilities, generation: -1 };
        if (reason !== "poll") setState(checkingCapabilities);
      }

      inFlight.current?.abort();
      const ac = new AbortController();
      inFlight.current = ac;
      const leaseGeneration = getAccessGeneration();
      const current = () =>
        !ac.signal.aborted &&
        leaseGeneration === getAccessGeneration() &&
        workspaceRef.current === workspaceId;

      const wantFresh =
        reason !== "poll" || lastResolved.current.status !== "ready";

      try {
        const result = await client.query({
          query: ViewerCapabilitiesDocument,
          variables: { ownerId: workspaceId, fresh: wantFresh },
          fetchPolicy: "no-cache",
          errorPolicy: "none",
          context: { fetchOptions: { signal: ac.signal } },
        });
        if (!current()) return;
        const payload = result.data?.viewerCapabilities;
        if (!payload) throw new Error("capability check unavailable");

        const next: CapabilityState = {
          status: "ready",
          snapshot: toSnapshot(
            workspaceId,
            payload.grants ?? [],
            payload.role ?? null,
          ),
        };

        const previous = lastResolved.current;
        if (downgradeDetected(previous, next)) {
          bumpAccessGeneration();
          if (
            ac.signal.aborted ||
            workspaceRef.current !== workspaceId ||
            getAccessGeneration() !== leaseGeneration + 1
          ) {
            // Generation bumped; the workspace effect will re-fetch.
            return;
          }
        }

        const navigationGrants =
          previous.status === "ready" &&
          previous.snapshot.workspaceId === workspaceId
            ? { ...previous.snapshot.grants }
            : {};
        for (const action of CAPABILITY_ACTIONS) {
          const outcome = next.snapshot.grants[action];
          if (outcome === "allowed" || outcome === "denied") {
            navigationGrants[action] = outcome;
          }
        }
        lastResolved.current = {
          status: "ready",
          snapshot: {
            ...next.snapshot,
            grants: navigationGrants,
          },
        };
        eligibility.current = {
          state: next,
          generation: getAccessGeneration(),
        };
        setState(next);
        setClock(Date.now());
      } catch {
        if (ac.signal.aborted || !current()) return;
        eligibility.current = {
          state: unavailableCapabilities,
          generation: -1,
        };
        setState(unavailableCapabilities);
      } finally {
        if (inFlight.current === ac) inFlight.current = null;
      }
    },
    [client],
  );
  refreshRef.current = refresh;

  useEffect(() => {
    lastResolved.current = checkingCapabilities;
    eligibility.current = { state: checkingCapabilities, generation: -1 };
    setState(checkingCapabilities);
    void refreshRef.current("workspace");
    return () => inFlight.current?.abort();
  }, [currentWorkspaceId, generation]);

  useEffect(() => {
    if (!currentWorkspaceId) return;
    const id = window.setInterval(() => {
      if (skipPollWhenHidden()) return;
      void refreshRef.current("poll");
    }, RESOURCE_POLL_INTERVAL_MS);
    return () => window.clearInterval(id);
  }, [currentWorkspaceId]);

  useEffect(() => {
    if (!currentWorkspaceId) return;
    const onFocus = () => {
      if (document.visibilityState === "visible") {
        void refreshRef.current("focus");
      }
    };
    const onOnline = () => void refreshRef.current("reconnect");
    window.addEventListener("focus", onFocus);
    document.addEventListener("visibilitychange", onFocus);
    window.addEventListener("online", onOnline);
    return () => {
      window.removeEventListener("focus", onFocus);
      document.removeEventListener("visibilitychange", onFocus);
      window.removeEventListener("online", onOnline);
    };
  }, [currentWorkspaceId]);

  useEffect(() => {
    if (state.status !== "ready") return;
    const timeout = window.setTimeout(
      () => setClock(Date.now()),
      Math.max(
        0,
        state.snapshot.receivedAt + CAPABILITY_FRESHNESS_MS - Date.now(),
      ),
    );
    return () => window.clearTimeout(timeout);
  }, [state]);

  const workspaceId = currentWorkspaceId;
  // Presentation follows `state` (including unavailable). Action eligibility is
  // gated separately via eligibility.current so a failed refresh expires
  // allows() without pretending the role changed.
  const resolved: CapabilityState = !workspaceId
    ? checkingCapabilities
    : state.status === "ready" &&
        (state.snapshot.workspaceId !== workspaceId ||
          clock - state.snapshot.receivedAt >= CAPABILITY_FRESHNESS_MS)
      ? checkingCapabilities
      : state;

  const flags = booleansFrom(
    eligibility.current.generation === generation
      ? eligibility.current.state.status === "ready" &&
        snapshotIsFresh(eligibility.current.state.snapshot, clock) &&
        eligibility.current.state.snapshot.workspaceId === workspaceId
        ? eligibility.current.state
        : checkingCapabilities
      : checkingCapabilities,
    workspaceId,
    clock,
  );
  const loaded =
    resolved.status === "ready" &&
    workspaceId !== null &&
    resolved.snapshot.workspaceId === workspaceId &&
    snapshotIsFresh(resolved.snapshot, clock) &&
    eligibility.current.generation === generation;
  const stale =
    state.status === "ready" &&
    workspaceId !== null &&
    state.snapshot.workspaceId === workspaceId &&
    !snapshotIsFresh(state.snapshot, clock);
  const unavailable = resolved.status === "unavailable";
  const loading = resolved.status === "checking" || !workspaceId;

  const value = useMemo<Capabilities>(() => {
    return {
      role: resolved.status === "ready" ? resolved.snapshot.role : null,
      ...flags,
      loading,
      loaded,
      stale,
      unavailable,
      generation,
      allows: (action) =>
        eligibility.current.generation === getAccessGeneration() &&
        allowsAction(eligibility.current.state, workspaceId, action),
      denied: (action) => confirmedDenied(resolved, workspaceId, action),
      reasonKey: (action) => {
        if (stale) return "capabilities.grantStale";
        return grantReasonKey(resolved, workspaceId, action, clock);
      },
      refresh: () => refreshRef.current("manual"),
    };
  }, [
    resolved,
    flags,
    loading,
    loaded,
    stale,
    unavailable,
    generation,
    workspaceId,
    clock,
  ]);

  return (
    <CapabilitiesContext.Provider value={value}>
      {children}
    </CapabilitiesContext.Provider>
  );
}

const FALLBACK: Capabilities = {
  role: null,
  canView: false,
  canViewLogs: false,
  canOperate: false,
  canCreate: false,
  canViewSensitive: false,
  canManageKeys: false,
  canManage: false,
  canManageBilling: false,
  loading: true,
  loaded: false,
  stale: false,
  unavailable: false,
  generation: 0,
  allows: () => false,
  denied: () => false,
  reasonKey: () => undefined,
  refresh: async () => undefined,
};

/* eslint-disable-next-line react-refresh/only-export-components */
export function useCapabilitiesContext(): Capabilities {
  return useContext(CapabilitiesContext) ?? FALLBACK;
}
