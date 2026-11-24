import type { TranslationEntry } from "@/i18n";

const koCapabilities: Record<string, TranslationEntry> = {
  "capabilities.reasonCanCreate": {
    message:
      "현재 역할로는 이 변경을 할 수 없습니다. 워크스페이스 관리자에게 문의하세요.",
    description:
      "Tooltip/hint on a create-gated service, environment, or datastore control for a member without can_create",
  },
  "capabilities.reasonCanOperate": {
    message:
      "현재 역할로는 이 서비스를 볼 수만 있습니다. 변경하려면 워크스페이스 관리자에게 문의하세요.",
    description:
      "Tooltip/hint on an operate-gated lifecycle control (e.g. a service restart) for a viewer without can_operate",
  },
  "capabilities.reasonCanViewSensitive": {
    message:
      "현재 역할로는 시크릿 값을 표시할 수 없습니다. 워크스페이스 관리자에게 문의하세요.",
    description:
      "Tooltip on a reveal control (connection string, env value) for a member without can_view_sensitive",
  },
  "capabilities.reasonCanManageBilling": {
    message:
      "현재 역할로는 결제를 관리할 수 없습니다. 워크스페이스 관리자 또는 결제 멤버에게 문의하세요.",
    description:
      "Tooltip on a billing control for a member without can_manage_billing",
  },
  "capabilities.reasonCanManage": {
    message:
      "현재 역할로는 멤버를 관리할 수 없습니다. 워크스페이스 관리자에게 문의하세요.",
    description:
      "Tooltip on a member-management control for a member without can_manage",
  },
  "capabilities.actionDenied": {
    message:
      "현재 역할로는 이 작업을 수행할 수 없습니다. 워크스페이스 관리자에게 문의하세요.",
    description:
      "Tooltip on a service/deploy action refused by the server action decision",
  },
  "capabilities.actionAdminOnly": {
    message: "이 작업은 관리자만 할 수 있습니다.",
    description:
      "Tooltip on an action refused to a non-admin in a protected environment: Suspend or Resume, per resource and in bulk (Render's copy, w4/m578); a datastore's connection-info reveal and IP rules, and an environment group's variables, secret files, links and Delete (w4/m594)",
  },
  "capabilities.actionUnavailable": {
    message: "서버가 지금 이 작업을 확인하지 못했습니다. 다시 시도하세요.",
    description:
      "Tooltip when an action projection is unavailable or returned an unknown outcome",
  },
  "capabilities.actionChecking": {
    message: "이 작업을 수행할 수 있는지 확인하는 중…",
    description:
      "Tooltip while a service/deploy action decision is still loading",
  },
  "capabilities.blockedProtectedConfirmation": {
    message: "보호된 리소스이므로 이 작업을 실행하려면 추가 확인이 필요합니다.",
    description: "Blocked reason for protected_confirmation_required",
  },
  "capabilities.blockedSuspended": {
    message:
      "서비스가 일시 중지된 동안에는 사용할 수 없습니다. 먼저 재개하세요.",
    description: "Blocked reason for suspended precondition",
  },
  "capabilities.blockedNotSuspended": {
    message: "이 리소스는 이미 실행 중이므로 재개할 항목이 없습니다.",
    description: "Blocked reason for not_suspended precondition",
  },
  "capabilities.blockedNoActiveDeploy": {
    message: "지금 취소할 진행 중인 배포가 없습니다.",
    description: "Blocked reason for no_active_deploy",
  },
  "capabilities.blockedNoActiveRun": {
    message: "지금 취소할 진행 중인 실행이 없습니다.",
    description: "Blocked reason for no_active_run",
  },
  "capabilities.blockedNoEligibleRollbackTarget": {
    message: "롤백 대상으로 사용할 수 있는 이전 배포가 없습니다.",
    description: "Blocked reason for no_eligible_rollback_target",
  },
  "capabilities.blockedBillingBlocked": {
    message:
      "이 작업을 실행하려면 먼저 결제 문제를 확인해야 합니다. 결제에서 해결하세요.",
    description: "Blocked reason for billing_blocked",
  },
  "capabilities.blockedExpired": {
    message:
      "무료 플랜이 만료되어 이 리소스는 일시 중단된 상태로 유지됩니다. 다시 시작하려면 유료 플랜으로 업그레이드하세요.",
    description: "Blocked reason for expired precondition",
  },
  "capabilities.blockedUnavailable": {
    message: "서버가 지금 이 작업을 확인하지 못했습니다. 다시 시도하세요.",
    description: "Blocked reason for unavailable precondition",
  },
  "capabilities.blockedGeneric": {
    message: "서버가 지금 이 작업을 허용하지 않습니다. 다시 시도하세요.",
    description: "Fallback blocked reason",
  },
  "capabilities.grantUnavailable": {
    message:
      "권한을 새로 고치지 못했습니다. 다시 시도하세요. 역할이 변경된 것은 아닙니다.",
    description:
      "Recovery copy when a capability grant check fails or is stale (m144)",
  },
  "capabilities.grantStale": {
    message: "이 작업을 실행하려면 권한을 새로 고쳐야 합니다.",
    description:
      "Copy when a capability answer is older than the freshness bound",
  },
};

export default koCapabilities;
