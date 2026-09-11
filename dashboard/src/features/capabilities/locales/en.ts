import type { TranslationEntry } from "@/i18n";

// Role-reason copy shown on controls the caller's workspace role can't use
// (w9/m84). Disable-with-reason, never hide: naming the missing capability
// teaches the role model instead of leaving a control that 403s on save.
const enCapabilities: Record<string, TranslationEntry> = {
  "capabilities.reasonCanCreate": {
    message: "Your role can’t make this change. Ask a workspace admin.",
    description:
      "Tooltip/hint on a create-gated service, environment, or datastore control for a member without can_create",
  },
  "capabilities.reasonCanOperate": {
    message:
      "Your role can only view this service. Ask a workspace admin to make changes.",
    description:
      "Tooltip/hint on an operate-gated control (e.g. a command-less cron schedule) for a viewer without can_operate",
  },
  "capabilities.reasonCanViewSensitive": {
    message: "Your role can’t reveal secret values. Ask a workspace admin.",
    description:
      "Tooltip on a reveal control (connection string, env value) for a member without can_view_sensitive",
  },
  "capabilities.reasonCanManageBilling": {
    message:
      "Your role can’t manage billing. Ask a workspace admin or billing member.",
    description:
      "Tooltip on a billing control for a member without can_manage_billing",
  },
  "capabilities.reasonCanManage": {
    message: "Your role can’t manage members. Ask a workspace admin.",
    description:
      "Tooltip on a member-management control for a member without can_manage",
  },
  "capabilities.actionDenied": {
    message: "Your role can’t perform this action. Ask a workspace admin.",
    description:
      "Tooltip on a service/deploy action refused by the server action decision",
  },
  "capabilities.actionUnavailable": {
    message:
      "The server could not confirm this action right now. Try again.",
    description:
      "Tooltip when an action projection is unavailable or returned an unknown outcome",
  },
  "capabilities.actionChecking": {
    message: "Checking whether you can perform this action…",
    description:
      "Tooltip while a service/deploy action decision is still loading",
  },
  "capabilities.blockedProtectedConfirmation": {
    message:
      "This protected resource needs an extra confirmation before this action can run.",
    description: "Blocked reason for protected_confirmation_required",
  },
  "capabilities.blockedSuspended": {
    message: "Unavailable while the service is suspended. Resume it first.",
    description: "Blocked reason for suspended precondition",
  },
  "capabilities.blockedNoActiveDeploy": {
    message: "There is no active deploy to cancel right now.",
    description: "Blocked reason for no_active_deploy",
  },
  "capabilities.blockedNoActiveRun": {
    message: "There is no active run to cancel right now.",
    description: "Blocked reason for no_active_run",
  },
  "capabilities.blockedNoEligibleRollbackTarget": {
    message: "No earlier deploy is eligible as a rollback target.",
    description: "Blocked reason for no_eligible_rollback_target",
  },
  "capabilities.blockedBillingBlocked": {
    message:
      "Billing needs attention before this action can run. Resolve it in Billing.",
    description: "Blocked reason for billing_blocked",
  },
  "capabilities.blockedUnavailable": {
    message:
      "The server could not confirm this action right now. Try again.",
    description: "Blocked reason for unavailable precondition",
  },
  "capabilities.blockedGeneric": {
    message: "The server is not allowing this action right now. Try again.",
    description: "Fallback blocked reason",
  },
  "capabilities.grantUnavailable": {
    message:
      "Permissions could not be refreshed. Try again — this is not a role change.",
    description:
      "Recovery copy when a capability grant check fails or is stale (m144)",
  },
  "capabilities.grantStale": {
    message: "Permissions need a refresh before this action can run.",
    description:
      "Copy when a capability answer is older than the freshness bound",
  },
};

export default enCapabilities;
