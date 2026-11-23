import type { TranslationEntry } from "@/i18n";

const ruCapabilities: Record<string, TranslationEntry> = {
  "capabilities.reasonCanCreate": {
    message:
      "Ваша роль не позволяет внести это изменение. Обратитесь к администратору рабочего пространства.",
    description:
      "Tooltip/hint on a create-gated service, environment, or datastore control for a member without can_create",
  },
  "capabilities.reasonCanOperate": {
    message:
      "Ваша роль позволяет только просматривать этот сервис. Попросите администратора рабочего пространства внести изменения.",
    description:
      "Tooltip/hint on an operate-gated control (e.g. a command-less cron schedule) for a viewer without can_operate",
  },
  "capabilities.reasonCanViewSensitive": {
    message:
      "Ваша роль не позволяет раскрывать значения секретов. Обратитесь к администратору рабочего пространства.",
    description:
      "Tooltip on a reveal control (connection string, env value) for a member without can_view_sensitive",
  },
  "capabilities.reasonCanManageBilling": {
    message:
      "Ваша роль не позволяет управлять оплатой. Обратитесь к администратору рабочего пространства или участнику, отвечающему за оплату.",
    description:
      "Tooltip on a billing control for a member without can_manage_billing",
  },
  "capabilities.reasonCanManage": {
    message:
      "Ваша роль не позволяет управлять участниками. Обратитесь к администратору рабочего пространства.",
    description:
      "Tooltip on a member-management control for a member without can_manage",
  },
  "capabilities.actionDenied": {
    message:
      "Ваша роль не позволяет выполнить это действие. Обратитесь к администратору рабочего пространства.",
    description:
      "Tooltip on a service/deploy action refused by the server action decision",
  },
  "capabilities.actionAdminOnly": {
    message: "Это действие доступно только администраторам.",
    description:
      "Tooltip on an action refused to a non-admin in a protected environment: Suspend or Resume, per resource and in bulk (Render's copy, w4/m578); a datastore's connection-info reveal and IP rules, and an environment group's variables, secret files, links and Delete (w4/m594)",
  },
  "capabilities.actionUnavailable": {
    message:
      "Сервер сейчас не смог подтвердить это действие. Повторите попытку.",
    description:
      "Tooltip when an action projection is unavailable or returned an unknown outcome",
  },
  "capabilities.actionChecking": {
    message: "Проверяем, можете ли вы выполнить это действие…",
    description:
      "Tooltip while a service/deploy action decision is still loading",
  },
  "capabilities.blockedProtectedConfirmation": {
    message:
      "Для этого защищённого ресурса перед выполнением действия нужно дополнительное подтверждение.",
    description: "Blocked reason for protected_confirmation_required",
  },
  "capabilities.blockedSuspended": {
    message: "Недоступно, пока сервис приостановлен. Сначала возобновите его.",
    description: "Blocked reason for suspended precondition",
  },
  "capabilities.blockedNotSuspended": {
    message: "Этот ресурс уже работает, возобновлять нечего.",
    description: "Blocked reason for not_suspended precondition",
  },
  "capabilities.blockedNoActiveDeploy": {
    message: "Сейчас нет активного деплоя, который можно отменить.",
    description: "Blocked reason for no_active_deploy",
  },
  "capabilities.blockedNoActiveRun": {
    message: "Сейчас нет активного запуска, который можно отменить.",
    description: "Blocked reason for no_active_run",
  },
  "capabilities.blockedNoEligibleRollbackTarget": {
    message: "Нет более раннего деплоя, на который можно выполнить откат.",
    description: "Blocked reason for no_eligible_rollback_target",
  },
  "capabilities.blockedBillingBlocked": {
    message:
      "Прежде чем выполнить это действие, нужно разобраться с оплатой. Решите вопрос в разделе «Оплата».",
    description: "Blocked reason for billing_blocked",
  },
  "capabilities.blockedUnavailable": {
    message:
      "Сервер сейчас не смог подтвердить это действие. Повторите попытку.",
    description: "Blocked reason for unavailable precondition",
  },
  "capabilities.blockedGeneric": {
    message: "Сервер сейчас не разрешает это действие. Повторите попытку.",
    description: "Fallback blocked reason",
  },
  "capabilities.grantUnavailable": {
    message:
      "Не удалось обновить разрешения. Повторите попытку — это не изменение роли.",
    description:
      "Recovery copy when a capability grant check fails or is stale (m144)",
  },
  "capabilities.grantStale": {
    message: "Перед выполнением этого действия нужно обновить разрешения.",
    description:
      "Copy when a capability answer is older than the freshness bound",
  },
};

export default ruCapabilities;
