import type { TranslationEntry } from "@/i18n";

const ptNotifications: Record<string, TranslationEntry> = {
  "notifications.title": {
    message: "Notificações",
    description: "Settings Notifications section card title",
  },
  "notifications.description": {
    message:
      "Receba e-mails quando um deploy de um dos seus serviços começar, for concluído com sucesso ou falhar, e quando algo falhar em tempo de execução. Estas são suas preferências pessoais — cada membro do workspace define as suas de forma independente.",
    description: "Settings Notifications section card description",
  },
  "notifications.deployStarted": {
    message: "Deploy iniciado",
    description: "Toggle label",
  },
  "notifications.deployStartedHint": {
    message:
      "Envie-me um e-mail quando um deploy começar. Enviado apenas para serviços que enviam todas as notificações.",
    description: "Toggle hint",
  },
  "notifications.deploySucceeded": {
    message: "Deploy concluído com sucesso",
    description: "Toggle label",
  },
  "notifications.deploySucceededHint": {
    message:
      "Envie-me um e-mail quando um deploy entrar no ar. Enviado apenas para serviços que enviam todas as notificações.",
    description: "Toggle hint",
  },
  "notifications.deployFailed": {
    message: "Falhas",
    description: "Toggle label",
  },
  "notifications.deployFailedHint": {
    message:
      "Envie-me um e-mail quando um deploy falhar, um serviço ficar instável, uma execução de cron falhar, o pull de uma imagem falhar ou uma sincronização de Blueprint falhar.",
    description: "Toggle hint",
  },
  "notifications.errorTitle": {
    message: "Não foi possível carregar as configurações de notificação",
    description: "Generic error title",
  },
  "notifications.errorBody": {
    message: "Algo deu errado. Tente novamente.",
    description: "Generic error body",
  },
  "notifications.updateError": {
    message: "Não foi possível salvar sua preferência",
    description: "Toast on a failed update",
  },
  "notifications.pushTitle": {
    message: "Push no celular",
    description: "Push settings card title",
  },
  "notifications.bexExtension": {
    message: "extensão do bex",
    description: "Badge distinguishing native push from Render parity",
  },
  "notifications.pushDescription": {
    message:
      "Controle os alertas do app do bex, a urgência, os horários de entrega e as exceções por serviço. As preferências de e-mail acima não são alteradas.",
    description: "Push settings description",
  },
  "notifications.pushEnabled": {
    message: "Ativar push no celular",
    description: "Push enabled toggle",
  },
  "notifications.pushEnabledHint": {
    message: "Envie alertas para seus dispositivos móveis do bex registrados.",
    description: "Push enabled hint",
  },
  "notifications.pushUnavailable": {
    message: "O envio de push não está configurado neste servidor bex",
    description: "Disabled server status",
  },
  "notifications.pushUnavailableHint": {
    message:
      "Você já pode preparar e salvar a política, mas os dispositivos registrados não receberão notificações até que um operador configure o provedor de push.",
    description: "Disabled server status hint",
  },
  "notifications.webPushTitle": {
    message: "Notificações do navegador",
    description: "Web push settings card title",
  },
  "notifications.webPushDescription": {
    message:
      "Receba os mesmos alertas neste navegador. Os filtros de eventos e os horários de silêncio são os que você definiu em Push no celular — não existe uma segunda política.",
    description: "Web push settings description",
  },
  "notifications.webPushUnsupported": {
    message: "Este navegador não oferece suporte a web push.",
    description: "Unsupported browser",
  },
  "notifications.webPushUnconfigured": {
    message: "Este servidor bex não ativou o push no navegador.",
    description: "Server VAPID unset",
  },
  "notifications.webPushDenied": {
    message:
      "As notificações estão bloqueadas para este site. Permita-as nas configurações do navegador e tente novamente.",
    description: "Permission denied",
  },
  "notifications.webPushError": {
    message:
      "Não foi possível atualizar as notificações do navegador. Tente novamente.",
    description: "Subscribe error",
  },
  "notifications.webPushOn": {
    message: "Este navegador receberá os alertas do bex.",
    description: "Subscribed state",
  },
  "notifications.webPushOff": {
    message: "Este navegador não está inscrito.",
    description: "Unsubscribed state",
  },
  "notifications.webPushEnable": {
    message: "Ativar neste navegador",
    description: "Subscribe button",
  },
  "notifications.webPushDisable": {
    message: "Desativar neste navegador",
    description: "Unsubscribe button",
  },
  "notifications.pushEvents": {
    message: "Filtros de eventos",
    description: "Push event section",
  },
  "notifications.pushEventsHint": {
    message:
      "Somente os tipos de evento selecionados com urgência igual ou superior à Urgência mínima podem gerar uma notificação push.",
    description: "Push events hint",
  },
  "notifications.pushEventNotSentYet": {
    message: "Ainda não enviado: nada envia este evento hoje.",
    description:
      "Under a push event that no part of bex emits yet (it is reserved for an upcoming feature); selecting it is kept but has no effect",
  },
  "notifications.pushEventNotSentYetShort": {
    message: "Ainda não enviado",
    description:
      "Compact form of the not-sent-yet note, under a push event in a per-service override's event grid",
  },
  "notifications.pushEventFiltered": {
    message: "Não enviado por push enquanto a Urgência mínima for {minimum}.",
    description:
      "Under a selected push event whose urgency is below the minimum ({minimum} = urgency name, e.g. 'Important')",
  },
  "notifications.pushFilteredTitle": {
    message: "Alguns eventos selecionados não serão enviados por push",
    description:
      "Heading of the save-area summary listing selected push events the minimum urgency drops",
  },
  "notifications.pushFilteredEvent": {
    message:
      "{event} é {urgency} e não será enviado por push enquanto a Urgência mínima for {minimum}.",
    description:
      "One summary line ({event} = event name, {urgency} = its urgency, {minimum} = the minimum urgency)",
  },
  "notifications.pushFilteredServiceEvent": {
    message:
      "{service}: {event} é {urgency} e não será enviado por push enquanto a Urgência mínima deste serviço for {minimum}.",
    description:
      "One summary line for a service override ({service} = service name; others as pushFilteredEvent)",
  },
  "notifications.pushOverrideFiltered_one": {
    message:
      "{count} evento selecionado não será enviado por push para este serviço enquanto a Urgência mínima dele for {minimum}.",
    description:
      "Under a service override whose own policy drops selected events ({minimum} = urgency name)",
  },
  "notifications.pushOverrideFiltered_many": {
    message:
      "{count} de eventos selecionados não serão enviados por push para este serviço enquanto a Urgência mínima dele for {minimum}.",
    description:
      "Under a service override whose own policy drops selected events ({minimum} = urgency name)",
  },
  "notifications.pushOverrideFiltered_other": {
    message:
      "{count} eventos selecionados não serão enviados por push para este serviço enquanto a Urgência mínima dele for {minimum}.",
    description:
      "Under a service override whose own policy drops selected events ({minimum} = urgency name)",
  },
  "notifications.pushEventDeployFailed": {
    message: "Deploy com falha",
    description: "Push event",
  },
  "notifications.pushEventServerFailed": {
    message: "Serviço travou",
    description: "Push event",
  },
  "notifications.pushEventCronFailed": {
    message: "Execução de cron com falha",
    description: "Push event",
  },
  "notifications.pushEventDeployStarted": {
    message: "Deploy iniciado",
    description: "Push event",
  },
  "notifications.pushEventDeploySucceeded": {
    message: "Deploy concluído com sucesso",
    description: "Push event",
  },
  "notifications.pushEventServerAvailable": {
    message: "Serviço recuperado",
    description: "Push event",
  },
  "notifications.pushEventServiceSuspended": {
    message: "Serviço suspenso",
    description: "Push event",
  },
  "notifications.pushEventServiceResumed": {
    message: "Serviço retomado",
    description: "Push event",
  },
  "notifications.pushEventAgentNeedsDecision": {
    message: "O agente precisa de uma decisão",
    description: "Push event",
  },
  "notifications.pushEventAgentPrReady": {
    message: "PR do agente pronto",
    description: "Push event",
  },
  "notifications.pushEventAgentFailed": {
    message: "Sessão do agente com falha",
    description: "Push event",
  },
  "notifications.pushEventAgentAutomationPaused": {
    message: "Automação pausada",
    description: "Push event: the platform paused an agent automation",
  },
  "notifications.pushEventPostgresUnavailable": {
    message: "Banco de dados indisponível",
    description: "Push event",
  },
  "notifications.pushEventPostgresAvailable": {
    message: "Banco de dados recuperado",
    description: "Push event",
  },
  "notifications.pushEventKeyValueUnhealthy": {
    message: "Key Value instável",
    description: "Push event",
  },
  "notifications.pushEventKeyValueAvailable": {
    message: "Key Value recuperado",
    description: "Push event",
  },
  "notifications.pushEventPostgresBackupFailed": {
    message: "Backup do banco de dados com falha",
    description: "Push event",
  },
  "notifications.pushEventPostgresRestoreFailed": {
    message: "Restauração do banco de dados com falha",
    description: "Push event",
  },
  "notifications.pushEventPostgresUpgradeFailed": {
    message: "Atualização do banco de dados com falha",
    description: "Push event",
  },
  "notifications.pushMinimumUrgency": {
    message: "Urgência mínima",
    description: "Urgency field",
  },
  "notifications.pushUrgencyRoutine": {
    message: "Rotina",
    description: "Urgency option",
  },
  "notifications.pushUrgencyImportant": {
    message: "Importante",
    description: "Urgency option",
  },
  "notifications.pushUrgencyCritical": {
    message: "Crítica",
    description: "Urgency option",
  },
  "notifications.pushTimezone": {
    message: "Fuso horário IANA",
    description: "Timezone field",
  },
  "notifications.pushTimezonePlaceholder": {
    message: "America/Los_Angeles",
    description: "Timezone placeholder",
  },
  "notifications.pushMaxDeferral": {
    message: "Adiamento máximo (horas)",
    description: "Deferral field",
  },
  "notifications.pushWorkingHours": {
    message: "Horário de trabalho",
    description: "Working hours heading",
  },
  "notifications.pushWorkingHoursHint": {
    message:
      "Quando definido, alertas não críticos aguardam até um destes intervalos.",
    description: "Working hours hint",
  },
  "notifications.pushQuietHours": {
    message: "Horário de silêncio",
    description: "Quiet hours heading",
  },
  "notifications.pushQuietHoursHint": {
    message:
      "Alertas não críticos aguardam enquanto um intervalo de silêncio estiver ativo.",
    description: "Quiet hours hint",
  },
  "notifications.weekdayMonday": {
    message: "Seg",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayTuesday": {
    message: "Ter",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayWednesday": {
    message: "Qua",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayThursday": {
    message: "Qui",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayFriday": {
    message: "Sex",
    description: "Weekday abbreviation",
  },
  "notifications.weekdaySaturday": {
    message: "Sáb",
    description: "Weekday abbreviation",
  },
  "notifications.weekdaySunday": {
    message: "Dom",
    description: "Weekday abbreviation",
  },
  "notifications.pushRangeStart": {
    message: "Início do intervalo",
    description: "Accessible time input label",
  },
  "notifications.pushRangeEnd": {
    message: "Fim do intervalo",
    description: "Accessible time input label",
  },
  "notifications.pushRemoveRange": {
    message: "Remover intervalo de tempo",
    description: "Accessible remove button label",
  },
  "notifications.pushAddRange": {
    message: "Adicionar intervalo de tempo",
    description: "Add range button",
  },
  "notifications.pushOverrides": {
    message: "Substituições por serviço",
    description: "Overrides heading",
  },
  "notifications.pushOverridesHint": {
    message:
      "Campos omitidos herdam as configurações acima; uma lista de eventos vazia e explícita silencia esse serviço.",
    description: "Override semantics hint",
  },
  "notifications.pushRemoveOverride": {
    message: "Remover substituição do serviço",
    description: "Accessible remove override label",
  },
  "notifications.pushOverrideEnabled": {
    message: "Entrega",
    description: "Override enabled field",
  },
  "notifications.pushInherit": {
    message: "Herdar",
    description: "Override option",
  },
  "notifications.pushOn": {
    message: "Ativado",
    description: "Override option",
  },
  "notifications.pushOff": {
    message: "Desativado",
    description: "Override option",
  },
  "notifications.pushExactEvents": {
    message: "Substituir por uma lista exata de eventos",
    description: "Exact event filter toggle",
  },
  "notifications.pushAddOverride": {
    message: "Adicionar uma substituição de serviço…",
    description: "Add override selector",
  },
  "notifications.pushSave": {
    message: "Salvar configurações de push",
    description: "Save button",
  },
  "notifications.pushUnsaved": {
    message: "Alterações não salvas",
    description:
      "Hint beside Save push settings while the push form differs from the saved settings (w4/435)",
  },
  "notifications.pushDiscardTitle": {
    message: "Descartar as alterações nas configurações de push?",
    description:
      "Navigation guard title when the push form has unsaved changes (w4/435)",
  },
  "notifications.pushDiscardBody": {
    message: "Estas alterações não foram salvas e serão perdidas se você sair.",
    description: "Navigation guard explanation for unsaved push settings",
  },
  "notifications.pushKeepEditing": {
    message: "Continuar editando",
    description: "Cancel the navigation and stay on the push form",
  },
  "notifications.pushDiscard": {
    message: "Descartar e sair",
    description: "Confirm leaving and dropping unsaved push settings",
  },
  "notifications.pushSaved": {
    message: "Configurações de push salvas",
    description: "Success toast",
  },
  "notifications.pushUpdateError": {
    message: "Não foi possível salvar as configurações de push",
    description: "Failure toast",
  },
  "notifications.pushErrorTitle": {
    message: "Não foi possível carregar as configurações de push",
    description: "Push load error title",
  },
  "notifications.pushInvalidTimezone": {
    message: "Informe um fuso horário IANA válido, como America/Los_Angeles.",
    description: "Timezone validation",
  },
  "notifications.pushInvalidDeferral": {
    message: "O adiamento máximo deve estar entre 0,5 e 168 horas.",
    description: "Deferral validation",
  },
  "notifications.pushInvalidEvents": {
    message: "O filtro de eventos contém um valor não compatível.",
    description: "Event validation",
  },
  "notifications.pushInvalidUrgency": {
    message: "Selecione uma urgência válida.",
    description: "Urgency validation",
  },
  "notifications.pushInvalidRange": {
    message:
      "Todo intervalo de tempo precisa de dias da semana, horários de início/fim válidos e extremidades diferentes.",
    description: "Range validation",
  },
  "notifications.pushTooManyRules": {
    message: "Há intervalos de agenda ou substituições de serviço demais.",
    description: "Rule count validation",
  },
  "notifications.pushInvalidService": {
    message:
      "As substituições de serviço devem ser únicas e referenciar um serviço válido.",
    description: "Service validation",
  },
  "notifications.pushEmptyOverride": {
    message:
      "Cada substituição de serviço deve alterar a entrega, a urgência ou os eventos.",
    description: "Empty override validation",
  },
  "notifications.workspaceTitle": {
    message: "Notificações",
    description:
      "Workspace Settings → Notifications card title (the workspace notification policy)",
  },
  "notifications.workspaceDescription": {
    message:
      "Padrões para todos os serviços. Um serviço pode substituí-los em suas Configurações.",
    description: "Workspace Settings → Notifications card description",
  },
  "notifications.workspaceReadOnly": {
    message:
      "Somente administradores do workspace podem alterar estes padrões.",
    description: "Workspace notification policy: read-only note for non-admins",
  },
  "notifications.workspaceEmailLabel": {
    message: "Notificações por e-mail",
    description: "Workspace notification policy: email destination switch",
  },
  "notifications.workspaceEmailHint": {
    message:
      "Desativado, nenhum e-mail de notificação é enviado, independentemente da configuração de um serviço.",
    description: "Workspace notification policy: email destination hint",
  },
  "notifications.workspaceLevelLabel": {
    message: "Notificações padrão dos serviços",
    description: "Workspace notification policy: level radio group label",
  },
  "notifications.workspaceLevelHint": {
    message: "O que os serviços que usam o padrão do workspace enviam.",
    description: "Workspace notification policy: level radio group hint",
  },
  "notifications.levelAll": {
    message: "Todas as notificações",
    description: "Notification level: every notification",
  },
  "notifications.levelAllHint": {
    message:
      "Falhas, além de deploys iniciando e entrando no ar, e recuperações.",
    description: "Notification level hint: all",
  },
  "notifications.levelFailure": {
    message: "Somente notificações de falha",
    description: "Notification level: failures only (Render's default)",
  },
  "notifications.levelFailureHint": {
    message:
      "Deploys com falha, serviços instáveis, execuções de cron com falha, discos cheios.",
    description: "Notification level hint: failure",
  },
  "notifications.levelNone": {
    message: "Nenhuma",
    description: "Notification level: nothing",
  },
  "notifications.levelNoneHint": {
    message: "Não enviar notificações para estes serviços.",
    description: "Notification level hint: none",
  },
  "notifications.workspaceSave": {
    message: "Salvar",
    description: "Workspace notification policy: save button",
  },
  "notifications.workspaceSaved": {
    message: "Padrões de notificação salvos",
    description: "Workspace notification policy: success toast",
  },
  "notifications.workspaceSaveFailed": {
    message:
      "Não foi possível salvar os padrões de notificação. Tente novamente.",
    description: "Workspace notification policy: save error",
  },
  "notifications.workspaceLoadFailed": {
    message:
      "Não foi possível carregar os padrões de notificação do workspace.",
    description: "Workspace notification policy: load error",
  },
  "notifications.workspaceRetry": {
    message: "Tentar novamente",
    description: "Workspace notification policy: retry button",
  },
  "notifications.overridesTitle": {
    message: "Substituições de notificação",
    description:
      "Workspace notifications: overrides list title (Render's label)",
  },
  "notifications.overridesDescription": {
    message:
      "Serviços cuja configuração de notificação difere do padrão do workspace.",
    description: "Workspace notifications: overrides list description",
  },
  "notifications.overridesEmpty": {
    message: "Nenhum serviço substitui o padrão do workspace.",
    description: "Workspace notifications: no overrides",
  },
  "notifications.overridesError": {
    message: "Não foi possível carregar os serviços que substituem o padrão.",
    description: "Workspace notifications: overrides failed to load",
  },
};

export default ptNotifications;
