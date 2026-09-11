import type { TranslationEntry } from "@/i18n";

const zhCapabilities: Record<string, TranslationEntry> = {
  "capabilities.reasonCanCreate": {
    message: "你的角色无法进行此更改。请联系工作区管理员。",
    description:
      "Tooltip/hint on a create-gated service, environment, or datastore control for a member without can_create",
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
    description:
      "Tooltip on a billing control for a member without can_manage_billing",
  },
  "capabilities.reasonCanManage": {
    message: "你的角色无法管理成员。请联系工作区管理员。",
    description:
      "Tooltip on a member-management control for a member without can_manage",
  },
  "capabilities.actionDenied": {
    message: "你的角色无法执行此操作。请联系工作区管理员。",
    description:
      "Tooltip on a service/deploy action refused by the server action decision",
  },
  "capabilities.actionUnavailable": {
    message: "服务器暂时无法确认此操作。请稍后重试。",
    description:
      "Tooltip when an action projection is unavailable or returned an unknown outcome",
  },
  "capabilities.actionChecking": {
    message: "正在检查你是否可以执行此操作…",
    description:
      "Tooltip while a service/deploy action decision is still loading",
  },
  "capabilities.blockedProtectedConfirmation": {
    message: "此受保护资源需要额外确认后才能执行此操作。",
    description: "Blocked reason for protected_confirmation_required",
  },
  "capabilities.blockedSuspended": {
    message: "服务已暂停，无法执行此操作。请先恢复服务。",
    description: "Blocked reason for suspended precondition",
  },
  "capabilities.blockedNoActiveDeploy": {
    message: "当前没有可取消的部署。",
    description: "Blocked reason for no_active_deploy",
  },
  "capabilities.blockedNoActiveRun": {
    message: "当前没有可取消的运行。",
    description: "Blocked reason for no_active_run",
  },
  "capabilities.blockedNoEligibleRollbackTarget": {
    message: "没有符合条件的早期部署可作为回滚目标。",
    description: "Blocked reason for no_eligible_rollback_target",
  },
  "capabilities.blockedBillingBlocked": {
    message: "账单需要处理后方可执行此操作。请前往账单页面解决。",
    description: "Blocked reason for billing_blocked",
  },
  "capabilities.blockedUnavailable": {
    message: "服务器暂时无法确认此操作。请稍后重试。",
    description: "Blocked reason for unavailable precondition",
  },
  "capabilities.blockedGeneric": {
    message: "服务器当前不允许此操作。请稍后重试。",
    description: "Fallback blocked reason",
  },
  "capabilities.grantUnavailable": {
    message: "无法刷新权限。请重试——这不是角色变更。",
    description:
      "Recovery copy when a capability grant check fails or is stale (m144)",
  },
  "capabilities.grantStale": {
    message: "执行此操作前需要刷新权限。",
    description:
      "Copy when a capability answer is older than the freshness bound",
  },
};

export default zhCapabilities;
