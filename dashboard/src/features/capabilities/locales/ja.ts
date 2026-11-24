import type { TranslationEntry } from "@/i18n";

const jaCapabilities: Record<string, TranslationEntry> = {
  "capabilities.reasonCanCreate": {
    message:
      "あなたのロールではこの変更はできません。ワークスペースの管理者に依頼してください。",
    description:
      "Tooltip/hint on a create-gated service, environment, or datastore control for a member without can_create",
  },
  "capabilities.reasonCanOperate": {
    message:
      "あなたのロールでは、このサービスを閲覧のみできます。変更するには、ワークスペースの管理者に依頼してください。",
    description:
      "Tooltip/hint on an operate-gated lifecycle control (e.g. a service restart) for a viewer without can_operate",
  },
  "capabilities.reasonCanViewSensitive": {
    message:
      "あなたのロールではシークレットの値を表示できません。ワークスペースの管理者に依頼してください。",
    description:
      "Tooltip on a reveal control (connection string, env value) for a member without can_view_sensitive",
  },
  "capabilities.reasonCanManageBilling": {
    message:
      "あなたのロールでは請求を管理できません。ワークスペースの管理者または請求担当のメンバーに依頼してください。",
    description:
      "Tooltip on a billing control for a member without can_manage_billing",
  },
  "capabilities.reasonCanViewBilling": {
    message:
      "あなたのロールでは請求の詳細を表示できません。ワークスペースの管理者に依頼してください。",
    description:
      "Why a member without can_view_billing (a Contributor or a Viewer, as on Render) sees no billing page, nav entry or figures",
  },
  "capabilities.reasonCanViewActivity": {
    message:
      "あなたのロールではサービスのイベント、デプロイ、メトリクスを表示できません。ワークスペースの管理者に依頼してください。",
    description:
      "Why a member without can_view_activity (a Billing member, as on Render) sees no service events, deploys or metrics: the reason on those pages and panels, whose nav entries are hidden",
  },
  "capabilities.activityDeniedTitle": {
    message: "あなたのロールでは利用できません",
    description:
      "Title above capabilities.reasonCanViewActivity where a page or panel of service events, deploys or metrics would be",
  },
  "capabilities.reasonCanManage": {
    message:
      "あなたのロールではメンバーを管理できません。ワークスペースの管理者に依頼してください。",
    description:
      "Tooltip on a member-management control for a member without can_manage",
  },
  "capabilities.actionDenied": {
    message:
      "あなたのロールではこの操作はできません。ワークスペースの管理者に依頼してください。",
    description:
      "Tooltip on a service/deploy action refused by the server action decision",
  },
  "capabilities.actionAdminOnly": {
    message: "この操作は管理者のみ実行できます。",
    description:
      "Tooltip on an action refused to a non-admin in a protected environment: Suspend or Resume, per resource and in bulk (Render's copy, w4/m578); a datastore's connection-info reveal and IP rules, and an environment group's variables, secret files, links and Delete (w4/m594)",
  },
  "capabilities.actionUnavailable": {
    message:
      "サーバーが現在、この操作を確認できませんでした。もう一度お試しください。",
    description:
      "Tooltip when an action projection is unavailable or returned an unknown outcome",
  },
  "capabilities.actionChecking": {
    message: "この操作を実行できるかどうかを確認しています…",
    description:
      "Tooltip while a service/deploy action decision is still loading",
  },
  "capabilities.blockedProtectedConfirmation": {
    message:
      "この保護されたリソースでは、この操作を実行する前に追加の確認が必要です。",
    description: "Blocked reason for protected_confirmation_required",
  },
  "capabilities.blockedSuspended": {
    message: "サービスが一時停止中のため利用できません。先に再開してください。",
    description: "Blocked reason for suspended precondition",
  },
  "capabilities.blockedNotSuspended": {
    message: "このリソースはすでに実行中のため、再開するものはありません。",
    description: "Blocked reason for not_suspended precondition",
  },
  "capabilities.blockedNoActiveDeploy": {
    message: "現在、キャンセルできるアクティブなデプロイはありません。",
    description: "Blocked reason for no_active_deploy",
  },
  "capabilities.blockedNoActiveRun": {
    message: "現在、キャンセルできるアクティブな実行はありません。",
    description: "Blocked reason for no_active_run",
  },
  "capabilities.blockedNoEligibleRollbackTarget": {
    message: "ロールバック先として利用できる以前のデプロイはありません。",
    description: "Blocked reason for no_eligible_rollback_target",
  },
  "capabilities.blockedBillingBlocked": {
    message:
      "この操作を実行する前に、請求の確認が必要です。請求ページで解決してください。",
    description: "Blocked reason for billing_blocked",
  },
  "capabilities.blockedExpired": {
    message:
      "無料プランの有効期限が切れたため、このリソースは一時停止のままです。再開するには有料プランにアップグレードしてください。",
    description: "Blocked reason for expired precondition",
  },
  "capabilities.blockedUnavailable": {
    message:
      "サーバーが現在、この操作を確認できませんでした。もう一度お試しください。",
    description: "Blocked reason for unavailable precondition",
  },
  "capabilities.blockedGeneric": {
    message:
      "現在、サーバーがこの操作を許可していません。もう一度お試しください。",
    description: "Fallback blocked reason",
  },
  "capabilities.grantUnavailable": {
    message:
      "権限を更新できませんでした。もう一度お試しください。ロールが変更されたわけではありません。",
    description:
      "Recovery copy when a capability grant check fails or is stale (m144)",
  },
  "capabilities.grantStale": {
    message: "この操作を実行する前に、権限の更新が必要です。",
    description:
      "Copy when a capability answer is older than the freshness bound",
  },
};

export default jaCapabilities;
