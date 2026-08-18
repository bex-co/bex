import type { TranslationEntry } from "@/i18n";

const zhCapabilities: Record<string, TranslationEntry> = {
  "capabilities.reasonCanCreate": {
    message: "你的角色无法更改此服务运行的内容。请联系工作区管理员。",
    description:
      "Tooltip/hint on a create-gated control (pre-deploy command, cron command, statement-logging params) for a member without can_create",
  },
  "capabilities.reasonCanOperate": {
    message: "你的角色只能查看此服务。请联系工作区管理员进行更改。",
    description:
      "Tooltip/hint on an operate-gated control (e.g. a command-less cron schedule) for a viewer without can_operate",
  },
  "capabilities.reasonCanViewSensitive": {
    message: "你的角色无法查看机密值。请联系工作区管理员。",
    description:
      "Tooltip on a reveal control (connection string, env value) for a member without can_view_sensitive",
  },
  "capabilities.reasonCanManageBilling": {
    message: "你的角色无法管理账单。请联系工作区管理员或账单成员。",
    description: "Tooltip on a billing control for a member without can_manage_billing",
  },
  "capabilities.reasonCanManage": {
    message: "你的角色无法管理成员。请联系工作区管理员。",
    description: "Tooltip on a member-management control for a member without can_manage",
  },
};

export default zhCapabilities;
