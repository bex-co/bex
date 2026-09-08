import { useEffect, useState } from "react";
import { dataBoundary } from "@/common/apollo/data-boundary";
import { useCapabilities } from "@/features/capabilities/capabilities-provider";
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
  const capabilities = useCapabilities();
  const allowed = capabilities.allows("can_view_logs");
  const { reconnectStream, cancel } = useRecovery({
    attempt: async () => {
      if (!capabilities.allows("can_view_logs")) return;
      await session.start(filters);
    },
    maxAttempts: 1,
  });
  useEffect(() => session.subscribe(setSnapshot), [session]);
  useEffect(
    () => dataBoundary.registerResetHandler(() => session.stop(true)),
    [session],
  );
  useEffect(() => {
    if (allowed) void reconnectStream();
    return () => {
      cancel();
      session.stop(true);
    };
  }, [allowed, cancel, filters, reconnectStream, session]);
  useEffect(() => {
    if (!recoveryAvailable(environment)) session.stop();
  }, [environment, session]);
  return snapshot;
}
