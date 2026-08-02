import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type MutableRefObject,
} from "react";
import { AppState, type AppStateStatus } from "react-native";
import { useNetworkState } from "@/common/apollo/network-state";
import {
  RecoveryCoordinator,
  type RecoveryCoordinatorOptions,
  type RecoveryEnvironment,
  type RecoveryReason,
  type RecoveryResult,
  type RecoverySnapshot,
} from "./recovery-coordinator";

export function useAppStateStatus(): AppStateStatus {
  const [status, setStatus] = useState<AppStateStatus>(AppState.currentState);
  useEffect(() => {
    const subscription = AppState.addEventListener("change", setStatus);
    return () => subscription.remove();
  }, []);
  return status;
}

export function useRecoveryEnvironment(): RecoveryEnvironment {
  return {
    connectivity: useNetworkState(),
    appState: useAppStateStatus(),
  };
}

export type UseRecoveryOptions = Omit<
  RecoveryCoordinatorOptions,
  "initialEnvironment"
>;

export interface UseRecoveryResult extends RecoverySnapshot {
  recover: (reason: RecoveryReason) => Promise<RecoveryResult>;
  /** Convenience for pull/polling consumers. */
  poll: () => Promise<RecoveryResult>;
  /** Convenience for EventSource/WebSocket reconnect consumers. */
  reconnectStream: () => Promise<RecoveryResult>;
  manualRetry: () => Promise<RecoveryResult>;
  cancel: () => void;
}

function latest<T>(ref: MutableRefObject<T>, value: T): MutableRefObject<T> {
  ref.current = value;
  return ref;
}

/** React binding around RecoveryCoordinator; transport behavior stays testable. */
export function useRecovery(options: UseRecoveryOptions): UseRecoveryResult {
  const environment = useRecoveryEnvironment();
  const optionsRef = latest(useRef(options), options);
  const coordinatorRef = useRef<RecoveryCoordinator | null>(null);
  if (coordinatorRef.current == null) {
    coordinatorRef.current = new RecoveryCoordinator({
      ...options,
      initialEnvironment: environment,
      attempt: (context) => optionsRef.current.attempt(context),
      refreshAuth: options.refreshAuth
        ? (signal) =>
            optionsRef.current.refreshAuth?.(signal) ?? Promise.resolve()
        : undefined,
      isAuthError: (error) => optionsRef.current.isAuthError?.(error) ?? false,
      isRetryable: (error) => optionsRef.current.isRetryable?.(error) ?? true,
    });
  }
  const coordinator = coordinatorRef.current;
  const [snapshot, setSnapshot] = useState(coordinator.getSnapshot());

  useEffect(() => coordinator.subscribe(setSnapshot), [coordinator]);
  useEffect(
    () => coordinator.setEnvironment(environment),
    [coordinator, environment],
  );
  useEffect(() => () => coordinator.dispose(), [coordinator]);

  const recover = useCallback(
    (reason: RecoveryReason) => coordinator.request(reason),
    [coordinator],
  );
  const poll = useCallback(() => coordinator.request("poll"), [coordinator]);
  const reconnectStream = useCallback(
    () => coordinator.request("stream"),
    [coordinator],
  );
  const manualRetry = useCallback(
    () => coordinator.manualRetry(),
    [coordinator],
  );
  const cancel = useCallback(() => coordinator.cancel(), [coordinator]);

  return {
    ...snapshot,
    recover,
    poll,
    reconnectStream,
    manualRetry,
    cancel,
  };
}
