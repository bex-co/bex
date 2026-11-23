import type { TranslationEntry } from "@/i18n";

const ptCapabilities: Record<string, TranslationEntry> = {
  "capabilities.reasonCanCreate": {
    message:
      "Sua função não pode fazer esta alteração. Peça a um administrador do workspace.",
    description:
      "Tooltip/hint on a create-gated service, environment, or datastore control for a member without can_create",
  },
  "capabilities.reasonCanOperate": {
    message:
      "Sua função só pode visualizar este serviço. Peça a um administrador do workspace para fazer alterações.",
    description:
      "Tooltip/hint on an operate-gated control (e.g. a command-less cron schedule) for a viewer without can_operate",
  },
  "capabilities.reasonCanViewSensitive": {
    message:
      "Sua função não pode revelar valores secretos. Peça a um administrador do workspace.",
    description:
      "Tooltip on a reveal control (connection string, env value) for a member without can_view_sensitive",
  },
  "capabilities.reasonCanManageBilling": {
    message:
      "Sua função não pode gerenciar o faturamento. Peça a um administrador do workspace ou a um membro de faturamento.",
    description:
      "Tooltip on a billing control for a member without can_manage_billing",
  },
  "capabilities.reasonCanManage": {
    message:
      "Sua função não pode gerenciar membros. Peça a um administrador do workspace.",
    description:
      "Tooltip on a member-management control for a member without can_manage",
  },
  "capabilities.actionDenied": {
    message:
      "Sua função não pode executar esta ação. Peça a um administrador do workspace.",
    description:
      "Tooltip on a service/deploy action refused by the server action decision",
  },
  "capabilities.actionAdminOnly": {
    message: "Esta ação é restrita a administradores.",
    description:
      "Tooltip on an action refused to a non-admin in a protected environment: Suspend or Resume, per resource and in bulk (Render's copy, w4/m578); a datastore's connection-info reveal and IP rules, and an environment group's variables, secret files, links and Delete (w4/m594)",
  },
  "capabilities.actionUnavailable": {
    message:
      "O servidor não conseguiu confirmar esta ação no momento. Tente novamente.",
    description:
      "Tooltip when an action projection is unavailable or returned an unknown outcome",
  },
  "capabilities.actionChecking": {
    message: "Verificando se você pode executar esta ação…",
    description:
      "Tooltip while a service/deploy action decision is still loading",
  },
  "capabilities.blockedProtectedConfirmation": {
    message:
      "Este recurso protegido precisa de uma confirmação extra antes de esta ação poder ser executada.",
    description: "Blocked reason for protected_confirmation_required",
  },
  "capabilities.blockedSuspended": {
    message:
      "Indisponível enquanto o serviço está suspenso. Retome-o primeiro.",
    description: "Blocked reason for suspended precondition",
  },
  "capabilities.blockedNotSuspended": {
    message:
      "Este recurso já está em execução, então não há nada para retomar.",
    description: "Blocked reason for not_suspended precondition",
  },
  "capabilities.blockedNoActiveDeploy": {
    message: "Não há nenhum deploy ativo para cancelar no momento.",
    description: "Blocked reason for no_active_deploy",
  },
  "capabilities.blockedNoActiveRun": {
    message: "Não há nenhuma execução ativa para cancelar no momento.",
    description: "Blocked reason for no_active_run",
  },
  "capabilities.blockedNoEligibleRollbackTarget": {
    message: "Nenhum deploy anterior é elegível como destino de rollback.",
    description: "Blocked reason for no_eligible_rollback_target",
  },
  "capabilities.blockedBillingBlocked": {
    message:
      "O faturamento precisa de atenção antes de esta ação poder ser executada. Resolva em Faturamento.",
    description: "Blocked reason for billing_blocked",
  },
  "capabilities.blockedUnavailable": {
    message:
      "O servidor não conseguiu confirmar esta ação no momento. Tente novamente.",
    description: "Blocked reason for unavailable precondition",
  },
  "capabilities.blockedGeneric": {
    message:
      "O servidor não está permitindo esta ação no momento. Tente novamente.",
    description: "Fallback blocked reason",
  },
  "capabilities.grantUnavailable": {
    message:
      "Não foi possível atualizar as permissões. Tente novamente — isto não é uma mudança de função.",
    description:
      "Recovery copy when a capability grant check fails or is stale (m144)",
  },
  "capabilities.grantStale": {
    message:
      "As permissões precisam ser atualizadas antes de esta ação poder ser executada.",
    description:
      "Copy when a capability answer is older than the freshness bound",
  },
};

export default ptCapabilities;
