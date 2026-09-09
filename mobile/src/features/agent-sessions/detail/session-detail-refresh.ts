import {
  recoveryAvailable,
  type RecoveryEnvironment,
} from "../../../common/hooks/recovery-coordinator";
import { isTerminalPhase } from "../lifecycle";

export const SESSION_DETAIL_POLL_MS = 30_000;

/**
 * Direct-entry session details must refresh on their own when the list is
 * unmounted. Poll only while the session is still live and the device is
 * foreground + online — terminal sessions and background/offline work stop.
 */
export function sessionDetailPollIntervalMs(
  phase: string | null | undefined,
  environment: RecoveryEnvironment,
): number {
  if (!recoveryAvailable(environment)) return 0;
  if (isTerminalPhase(phase)) return 0;
  return SESSION_DETAIL_POLL_MS;
}
