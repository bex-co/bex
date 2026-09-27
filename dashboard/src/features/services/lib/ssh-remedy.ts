import type { en } from "@/i18n";
import type { ServiceView } from "@/features/services/types";

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
