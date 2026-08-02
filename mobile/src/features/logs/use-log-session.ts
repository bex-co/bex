import { useEffect, useState } from "react";
import { recoveryAvailable } from "@/common/hooks/recovery-coordinator";
import {
  useRecovery,
  useRecoveryEnvironment,
} from "@/common/hooks/use-recovery";
import type { LogFilters } from "./types";
import { LogSession, type LogSessionSnapshot } from "./log-session";

export function useLogSession(
  session: LogSession,
  filters: LogFilters,
): LogSessionSnapshot {
  const [snapshot, setSnapshot] = useState(session.snapshot());
  const environment = useRecoveryEnvironment();
  const { reconnectStream, cancel } = useRecovery({
    attempt: async () => {
      await session.start(filters);
    },
    maxAttempts: 1,
  });
  useEffect(() => session.subscribe(setSnapshot), [session]);
  useEffect(() => {
    void reconnectStream();
    return () => {
      cancel();
      session.stop();
    };
  }, [cancel, filters, reconnectStream, session]);
  useEffect(() => {
    if (!recoveryAvailable(environment)) session.stop();
  }, [environment, session]);
  return snapshot;
}
