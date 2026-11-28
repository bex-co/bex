import type { en } from "@/i18n";
import type { ServiceView } from "@/features/services/types";
import { isSuspended } from "@/features/services/lib/status";

/**
 * Why a service has no SSH address, and what would change that. The gate is
 * server-side (a running, paid web/private/background service with the SSH
 * gateway configured). Of its inputs, a suspension and a Free instance type
 * are the two a user can act on themselves, so those get their own reason and
 * a link to the remedy. Anything else keeps the general explanation (w4/143:
 * the Connect menu said only "SSH isn't available", with the reason in a
 * hover-only title, and the Shell tab offered SSH key management, which cannot
 * help a Free service).
 */
export interface SshRemedy {
  reason: keyof typeof en;
  action: { label: keyof typeof en; path: "settings" | "plan" } | null;
}

// Only these types can ever carry an SSH address. A static site or cron job
// keeps the general explanation, since no plan change or resume would add SSH.
// All three live under /services, so the remedy links need no /static base.
const SSH_TYPES = new Set([
  "web_service",
  "private_service",
  "background_worker",
]);

export function sshRemedy(
  service: Pick<ServiceView, "suspended" | "plan" | "type">,
): SshRemedy {
  if (!SSH_TYPES.has(service.type)) {
    return { reason: "services.sshUnavailableHint", action: null };
  }
  // A suspended Free service has both blockers. Resuming restores traffic
  // only, so the copy must not promise SSH after a resume (w4/m183).
  if (service.suspended && service.plan === "free") {
    return {
      reason: "services.sshUnavailableSuspendedFree",
      action: { label: "services.sshRemedyResume", path: "settings" },
    };
  }
  if (service.suspended) {
    return {
      reason: "services.sshUnavailableSuspended",
      action: { label: "services.sshRemedyResume", path: "settings" },
    };
  }
  if (service.plan === "free") {
    return {
      reason: "services.sshUnavailableFree",
      action: { label: "services.sshRemedyChangePlan", path: "plan" },
    };
  }
  return { reason: "services.sshUnavailableHint", action: null };
}

/**
 * What a service's Shell page draws (w3/124): the Web Shell and the SSH
 * command when the service has an SSH address, else the "Shell access isn't
 * available" card with its remedy. Its loading frame draws the same, from the
 * service's fields as a cache or the layout's loader holds them.
 */
export type ShellAccess =
  { eligible: true } | { eligible: false; remedy: SshRemedy };

/**
 * The Shell page's shape from a service's raw fields: its `sshAddress` when
 * read (`server`), else bex-api's rule for one (`sshEligible`: a running, paid
 * web, private or background service), as the services list does not select
 * the address. Null until the type is known.
 */
export function shellAccess(
  service:
    | {
        type?: string | null;
        plan?: string | null;
        suspended?: string | null;
        sshAddress?: string | null;
      }
    | null
    | undefined,
): ShellAccess | null {
  if (!service?.type) return null;
  const suspended = isSuspended(service.suspended ?? null);
  const plan = service.plan ?? null;
  const eligible =
    service.sshAddress !== undefined
      ? Boolean(service.sshAddress)
      : SSH_TYPES.has(service.type) &&
        !suspended &&
        plan != null &&
        plan !== "free";
  if (eligible) return { eligible: true };
  return {
    eligible: false,
    remedy: sshRemedy({ type: service.type, plan, suspended }),
  };
}
