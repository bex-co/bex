import type { TranslationEntry } from "@/i18n";

// Role-reason copy shown on controls the caller's workspace role can't use
// (w9/m84). Disable-with-reason, never hide: naming the missing capability
// teaches the role model instead of leaving a control that 403s on save.
const enCapabilities: Record<string, TranslationEntry> = {
  "capabilities.reasonCanCreate": {
    message: "Your role can’t change what this service runs. Ask a workspace admin.",
    description:
      "Tooltip/hint on a create-gated control (pre-deploy command, cron command, statement-logging params) for a member without can_create",
  },
  "capabilities.reasonCanOperate": {
    message: "Your role can only view this service. Ask a workspace admin to make changes.",
    description:
      "Tooltip/hint on an operate-gated control (e.g. a command-less cron schedule) for a viewer without can_operate",
  },
  "capabilities.reasonCanViewSensitive": {
    message: "Your role can’t reveal secret values. Ask a workspace admin.",
    description:
      "Tooltip on a reveal control (connection string, env value) for a member without can_view_sensitive",
  },
  "capabilities.reasonCanManageBilling": {
    message: "Your role can’t manage billing. Ask a workspace admin or billing member.",
    description: "Tooltip on a billing control for a member without can_manage_billing",
  },
  "capabilities.reasonCanManage": {
    message: "Your role can’t manage members. Ask a workspace admin.",
    description: "Tooltip on a member-management control for a member without can_manage",
  },
};

export default enCapabilities;
