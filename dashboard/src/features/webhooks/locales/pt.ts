import type { TranslationEntry } from "@/i18n";

const ptWebhooks: Record<string, TranslationEntry> = {
  "webhooks.title": {
    message: "Webhooks",
    description: "Settings Integrations Webhooks section card title",
  },
  "webhooks.description": {
    message:
      "Envie notificações de eventos assinadas (deploys, suspensões, reinicializações, escalonamento) para os seus próprios endpoints — sem polling. O segredo de assinatura de cada webhook fica nas configurações dele.",
    description: "Settings Webhooks section card description",
  },
  "webhooks.colEndpoint": {
    message: "Endpoint",
    description: "Webhooks table column header — name + destination URL",
  },
  "webhooks.colEvents": {
    message: "Eventos",
    description: "Webhooks table column header — subscribed event types",
  },
  "webhooks.colEnabled": {
    message: "Ativado",
    description: "Webhooks table column header — the enable/disable switch",
  },
  "webhooks.colLatest": {
    message: "Última entrega",
    description: "Webhooks table column header — newest immutable attempt",
  },
  "webhooks.searchEndpoints": {
    message: "Buscar webhooks",
    description: "Endpoint list search label and placeholder",
  },
  "webhooks.searchEndpointsHelp": {
    message: "Busque por nome, destino ou evento",
    description: "Endpoint list search helper text",
  },
  "webhooks.searchEndpointsEmpty": {
    message: "Nenhum webhook corresponde à sua busca.",
    description: "Endpoint list empty state after local search",
  },
  "webhooks.emptyTitle": {
    message: "Nenhum webhook",
    description: "Webhooks empty-state title",
  },
  "webhooks.emptyBody": {
    message:
      "Adicione um endpoint para ser notificado quando seus serviços fizerem deploy, reiniciarem ou escalarem.",
    description: "Webhooks empty-state body",
  },
  "webhooks.forbiddenTitle": {
    message: "Não autorizado",
    description: "Webhooks state when the caller lacks permission (403)",
  },
  "webhooks.forbiddenBody": {
    message: "Você não tem permissão para ver os webhooks deste workspace.",
    description: "Webhooks forbidden-state body",
  },
  "webhooks.errorTitle": {
    message: "Não foi possível carregar os webhooks",
    description: "Webhooks generic error title",
  },
  "webhooks.errorBody": {
    message: "Algo deu errado. Tente novamente.",
    description: "Webhooks generic error body",
  },
  "webhooks.proOnly": {
    message:
      "Webhooks exigem o plano Pro ou superior. Os endpoints existentes continuam recebendo entregas.",
    description: "Webhooks card description on a Hobby workspace (w8/m138)",
  },
  "webhooks.create": {
    message: "Adicionar webhook",
    description: "Button that opens the create dialog",
  },
  "webhooks.createTitle": {
    message: "Adicionar um webhook",
    description: "Create dialog title",
  },
  "webhooks.createDescription": {
    message:
      "O bex enviará um POST com um payload JSON assinado para esta URL sempre que um evento assinado ocorrer.",
    description: "Create dialog description",
  },
  "webhooks.createEnabledHelp": {
    message:
      "Começa a entregar imediatamente. Você pode ativá-lo depois em Configurações.",
    description: "Create page initial enabled-state helper copy",
  },
  "webhooks.fieldName": {
    message: "Nome",
    description: "Create dialog name field label",
  },
  "webhooks.fieldNamePlaceholder": {
    message: "ex.: deploy-alerts-slack-bot",
    description: "Create dialog name field placeholder",
  },
  "webhooks.fieldUrl": {
    message: "URL de destino",
    description: "Create dialog URL field label",
  },
  "webhooks.fieldEvents": {
    message: "Eventos a enviar",
    description: "Create dialog event-type checklist label",
  },
  "webhooks.eventsLoading": {
    message: "Carregando tipos de evento…",
    description: "Create dialog while the event-type vocabulary loads",
  },
  "webhooks.eventsLoadError": {
    message:
      "Não foi possível carregar os tipos de evento. Tente novamente antes de salvar.",
    description: "Actionable event-catalog query failure",
  },
  "webhooks.eventsRetry": {
    message: "Tentar carregar os tipos de evento novamente",
    description: "Retry button for the event-catalog query",
  },
  "webhooks.eventsEmpty": {
    message: "Nenhum tipo de evento assinável está disponível no momento.",
    description: "Event picker state when the served catalog is empty",
  },
  "webhooks.formUnavailableEvents": {
    message:
      "A criação fica indisponível até que os tipos de evento sejam carregados.",
    description:
      "Create submit disabled reason while the catalog is unavailable",
  },
  "webhooks.formErrorsSummary": {
    message: "Corrija os campos destacados e tente novamente.",
    description: "Accessible webhook form validation summary",
  },
  "webhooks.validation.nameRequired": {
    message: "Informe um nome para o webhook.",
    description: "Field-local required-name validation",
  },
  "webhooks.validation.nameDuplicate": {
    message: "Já existe um webhook com este nome.",
    description: "Field-local duplicate-name validation",
  },
  "webhooks.validation.urlRequired": {
    message: "Informe uma URL de destino.",
    description: "Field-local required URL validation",
  },
  "webhooks.validation.urlHTTPS": {
    message: "Informe uma URL HTTPS absoluta sem credenciais embutidas.",
    description: "Field-local webhook URL validation",
  },
  "webhooks.validation.eventsRequired": {
    message: "Selecione pelo menos um evento.",
    description: "Field-local event selection validation",
  },
  "webhooks.validation.eventsInvalid": {
    message:
      "A seleção de eventos mudou. Recarregue o catálogo e escolha novamente.",
    description: "Server-coded stale/invalid event filter",
  },
  "webhooks.errors.planLimit": {
    message:
      "Webhooks estão disponíveis no plano Pro e superiores. Este workspace está no Hobby, então não pode adicionar um endpoint.",
    description:
      "Server-coded plan refusal of a webhook create on a Hobby workspace (WEBHOOK_PLAN_LIMIT, w4/425)",
  },
  "webhooks.planGate": {
    message:
      "Webhooks estão disponíveis no plano Pro e superiores. Faça upgrade deste workspace para adicionar um endpoint.",
    description:
      "Notice in place of the create form and the empty list on a Hobby workspace (w4/425)",
  },
  "webhooks.errors.endpointLimit": {
    message: "Este workspace atingiu o limite de endpoints de webhook.",
    description: "Server-coded endpoint quota refusal",
  },
  "webhooks.errors.endpointDisabled": {
    message: "Ative este endpoint antes de reenviar uma entrega.",
    description: "Server-coded resend refusal for a disabled endpoint",
  },
  "webhooks.errors.deliveryPending": {
    message: "Uma tentativa de entrega já está na fila. Aguarde a conclusão.",
    description: "Server-coded duplicate pending resend refusal",
  },
  "webhooks.errors.staleState": {
    message:
      "Este webhook foi alterado ou removido. Atualize e tente novamente.",
    description: "Safe stale endpoint/attempt refusal",
  },
  "webhooks.manageRequired": {
    message:
      "Você pode inspecionar este webhook, mas somente gerentes do workspace podem alterá-lo.",
    description: "Read-only member notice on webhook management surfaces",
  },
  "webhooks.latestNever": {
    message: "Nunca enviado",
    description: "Endpoint with no completed delivery attempt",
  },
  "webhooks.latestDelivered": {
    message: "Bem-sucedida",
    description: "Latest endpoint attempt succeeded",
  },
  "webhooks.latestFailed": {
    message: "Falhou",
    description: "Latest endpoint attempt failed terminally",
  },
  "webhooks.latestRetrying": {
    message: "Tentando novamente",
    description: "Latest endpoint attempt failed with a retry pending",
  },
  "webhooks.createCancel": {
    message: "Cancelar",
    description: "Create dialog cancel button",
  },
  "webhooks.createSubmit": {
    message: "Criar webhook",
    description: "Create page submit button (Render: 'Create Webhook')",
  },
  "webhooks.createSuccess": {
    message: "Webhook criado",
    description: "Toast after a successful create",
  },
  "webhooks.createError": {
    message: "Não foi possível criar o webhook",
    description: "Toast after a failed create",
  },
  "webhooks.copy": {
    message: "Copiar segredo de assinatura",
    description: "Copy-secret button label",
  },
  "webhooks.copied": {
    message: "Segredo copiado para a área de transferência",
    description: "Toast after copying the secret",
  },
  "webhooks.copyError": {
    message: "Não foi possível copiar — selecione o texto e copie manualmente",
    description: "Toast when clipboard write fails",
  },
  "webhooks.toggle": {
    message: "Ativado",
    description: "Accessible label of the per-endpoint enable/disable switch",
  },
  "webhooks.enableSuccess": {
    message: "{name} ativado",
    description: "Toast after enabling an endpoint",
  },
  "webhooks.disableSuccess": {
    message: "{name} desativado",
    description: "Toast after disabling an endpoint",
  },
  "webhooks.toggleError": {
    message: "Não foi possível atualizar {name}",
    description: "Toast after a failed enable/disable",
  },
  "webhooks.delete": {
    message: "Excluir",
    description: "Delete action label (icon button + confirm button)",
  },
  "webhooks.deleteConfirmTitle": {
    message: "Excluir {name}?",
    description: "Delete confirmation dialog title",
  },
  "webhooks.deleteConfirmBody": {
    message:
      "Nenhum outro evento será enviado a este endpoint, e o histórico de entregas dele será removido.",
    description: "Delete confirmation dialog body",
  },
  "webhooks.deleteCancel": {
    message: "Cancelar",
    description: "Delete confirmation cancel button",
  },
  "webhooks.deleteSuccess": {
    message: "{name} excluído",
    description: "Toast after a successful delete",
  },
  "webhooks.deleteError": {
    message: "Não foi possível excluir {name}",
    description: "Toast after a failed delete",
  },
  "webhooks.history": {
    message: "Histórico de entregas",
    description: "Accessible label of the per-endpoint history button",
  },
  "webhooks.historyTitle": {
    message: "Entregas — {name}",
    description: "Delivery-history dialog title",
  },
  "webhooks.historyBody": {
    message:
      "Cada evento enviado a este endpoint, do mais recente ao mais antigo. As entregas com falha são repetidas com backoff exponencial; falhas repetidas desativam o endpoint.",
    description: "Delivery-history dialog description",
  },
  "webhooks.historyEmptyTitle": {
    message: "Ainda não há entregas",
    description: "Delivery-history empty-state title",
  },
  "webhooks.historyEmptyBody": {
    message:
      "Dispare um evento assinado — um deploy, por exemplo — para vê-lo aqui.",
    description: "Delivery-history empty-state body",
  },
  "webhooks.historyFilteredEmptySuccessfulTitle": {
    message: "Nenhuma entrega bem-sucedida",
    description:
      "Delivery-history state when the Successful tab (and any date range) matches nothing (w4/146)",
  },
  "webhooks.historyFilteredEmptyFailedTitle": {
    message: "Nenhuma entrega com falha",
    description:
      "Delivery-history state when the Failed tab (and any date range) matches nothing (w4/146)",
  },
  "webhooks.historyFilteredEmptyRangeTitle": {
    message: "Nenhuma entrega neste intervalo",
    description:
      "Delivery-history state when the Sent after/before range matches nothing (w4/146)",
  },
  "webhooks.historyFilteredEmptyBody": {
    message:
      "Outras entregas podem corresponder a um status ou intervalo de tempo diferente.",
    description: "Body under a filtered-empty delivery history (w4/146)",
  },
  "webhooks.historyClearFilters": {
    message: "Limpar filtros",
    description:
      "Resets the delivery-history status tab and date range (w4/146)",
  },
  "webhooks.historyErrorTitle": {
    message: "Não foi possível carregar as entregas",
    description: "Delivery-history error-state title",
  },
  "webhooks.historyErrorBody": {
    message: "Algo deu errado. Tente novamente.",
    description: "Delivery-history error-state body",
  },
  "webhooks.colEvent": {
    message: "Evento",
    description: "Delivery-history table column header",
  },
  "webhooks.colService": {
    message: "Serviço",
    description: "Delivery-history table column header",
  },
  "webhooks.serviceDeleted": {
    message: "(excluído)",
    description:
      "Marker beside a delivery's recorded service name when that service no longer exists, so the name cannot be linked",
  },
  "webhooks.colStatus": {
    message: "Status",
    description: "Delivery-history table column header",
  },
  "webhooks.colAttempts": {
    message: "Tentativa",
    description:
      "Delivery-history table column header — one-based attempt number",
  },
  "webhooks.colResponse": {
    message: "Resposta",
    description: "Delivery-history table column header — last HTTP status",
  },
  "webhooks.colWhen": {
    message: "Quando",
    description:
      "Delivery-history table column header — relative and exact send time",
  },
  "webhooks.colActions": {
    message: "Ações",
    description: "Accessible delivery-history table actions column header",
  },
  "webhooks.status.pending": {
    message: "Pendente",
    description: "Delivery status badge — queued or between retries",
  },
  "webhooks.status.delivered": {
    message: "Entregue",
    description: "Delivery status badge — endpoint answered 2xx",
  },
  "webhooks.status.failed": {
    message: "Falhou",
    description: "Delivery status badge — this individual attempt failed",
  },
  "webhooks.httpStatus": {
    message: "HTTP {status}",
    description: "Delivery attempt HTTP status summary",
  },
  "webhooks.openSourceEvent": {
    message: "{label} — abrir o evento de origem {id}",
    description:
      "Accessible label for the event hydration link; it starts with the link's visible text, the event type (WCAG 2.5.3)",
  },
  "webhooks.requestPayload": {
    message: "Payload da requisição",
    description: "Expanded delivery-attempt request evidence heading",
  },
  "webhooks.endpointResponse": {
    message: "Resposta do endpoint",
    description: "Expanded delivery-attempt response evidence heading",
  },
  "webhooks.noRequestEvidence": {
    message: "Não há evidência da requisição disponível.",
    description: "Expanded attempt with no retained request body",
  },
  "webhooks.noResponseEvidence": {
    message: "Nenhuma resposta foi registrada.",
    description: "Expanded pending/transport attempt with no endpoint body",
  },
  "webhooks.attemptIdentity": {
    message: "Tentativa {attemptId} · evento de origem {eventId}",
    description: "Immutable attempt and stable source-event identity",
  },
  "webhooks.parentDeliveryStatus": {
    message: "Estado da notificação: {status}",
    description:
      "Logical notification state shown separately from attempt outcome",
  },
  "webhooks.retryScheduled": {
    message: "Próxima tentativa automática: {date}",
    description: "Parent notification retry schedule",
  },
  "webhooks.resend": {
    message: "Reenviar",
    description: "Manual resend action for a failed attempt",
  },
  "webhooks.resending": {
    message: "Reenviando…",
    description: "Manual resend action in progress",
  },
  "webhooks.resendConfirmTitle": {
    message: "Reenviar este webhook?",
    description: "Manual resend confirmation title",
  },
  "webhooks.resendConfirmBody": {
    message:
      "Isso enfileira uma nova tentativa assinada com o mesmo evento de origem e o mesmo payload da requisição.",
    description: "Manual resend confirmation consequence",
  },
  "webhooks.resendCancel": {
    message: "Cancelar",
    description: "Manual resend confirmation cancel action",
  },
  "webhooks.resendSuccess": {
    message: "Reenvio do webhook enfileirado",
    description: "Toast after a manual resend reservation succeeds",
  },
  "webhooks.resendError": {
    message: "Não foi possível reenviar este webhook",
    description: "Toast after a manual resend is refused or fails",
  },
  "webhooks.loadMore": {
    message: "Carregar mais",
    description: "Delivery-history pagination button",
  },
  "webhooks.selectedCount_one": {
    message: "{count} evento selecionado",
    description: "Event-picker live selection counter (singular)",
  },
  "webhooks.selectedCount_many": {
    message: "{count} de eventos selecionados",
    description: "Event-picker live selection counter (plural)",
  },
  "webhooks.selectedCount_other": {
    message: "{count} eventos selecionados",
    description: "Event-picker live selection counter (plural)",
  },
  "webhooks.searchEvents": {
    message: "Buscar eventos",
    description: "Event-picker search box placeholder + accessible label",
  },
  "webhooks.searchNoMatches": {
    message: "Nenhum evento corresponde à sua busca",
    description:
      "Event-picker empty state while a search filters everything out",
  },
  "webhooks.allEvents": {
    message: "Todos os eventos",
    description: "Event-picker tri-state master checkbox label",
  },
  "webhooks.groupToggle": {
    message: "Alternar eventos de {group}",
    description: "Accessible label of a group's expand/collapse chevron",
  },
  "webhooks.group.deploy": {
    message: "Deploy",
    description: "Event-picker group — deploy lifecycle events",
  },
  "webhooks.group.autoDeploy": {
    message: "Deploy automático",
    description: "Event-picker group — automatic deploy setting events",
  },
  "webhooks.group.serviceAvailability": {
    message: "Disponibilidade do serviço",
    description: "Event-picker group — observed service availability events",
  },
  "webhooks.group.scaling": {
    message: "Escalonamento",
    description: "Event-picker group — service scaling events",
  },
  "webhooks.group.cronJobRun": {
    message: "Execução de Cron Job",
    description: "Event-picker group — cron job run events",
  },
  "webhooks.group.maintenanceMode": {
    message: "Modo de manutenção",
    description: "Event-picker group — maintenance mode events",
  },
  "webhooks.group.disk": {
    message: "Disco",
    description: "Event-picker group — persistent disk events",
  },
  "webhooks.group.postgres": {
    message: "Postgres",
    description: "Event-picker group — managed Postgres events",
  },
  "webhooks.group.keyValue": {
    message: "Key Value",
    description: "Event-picker group — managed Key Value events",
  },
  "webhooks.group.suspension": {
    message: "Suspensão",
    description: "Event-picker group — suspend/resume events",
  },
  "webhooks.group.agentSession": {
    message: "Agentes na nuvem",
    description: "Event-picker group — cloud coding agent session events",
  },
  "webhooks.group.other": {
    message: "Outros",
    description:
      "Event-picker fallback group for served keys the catalog doesn't know yet",
  },
  "webhooks.event.deploy_started": {
    message: "Deploy iniciado",
    description: "Event label — deploy_started",
  },
  "webhooks.event.branch_deleted": {
    message: "Branch excluído",
    description: "Webhook event label",
  },
  "webhooks.event.build_started": {
    message: "Build iniciado",
    description: "Webhook event label",
  },
  "webhooks.event.build_ended": {
    message: "Build concluído",
    description: "Webhook event label",
  },
  "webhooks.event.pre_deploy_started": {
    message: "Pré-deploy iniciado",
    description: "Webhook event label",
  },
  "webhooks.event.pre_deploy_ended": {
    message: "Pré-deploy concluído",
    description: "Webhook event label",
  },
  "webhooks.event.job_run_ended": {
    message: "Execução de job concluída",
    description: "Webhook event label",
  },
  "webhooks.event.auto_deploy_enabled": {
    message: "Deploy automático ativado",
    description: "Webhook event label",
  },
  "webhooks.event.auto_deploy_disabled": {
    message: "Deploy automático desativado",
    description: "Webhook event label",
  },
  "webhooks.event.deploy_ended": {
    message: "Deploy concluído",
    description: "Event label — deploy_ended",
  },
  "webhooks.event.image_pull_failed": {
    message: "Falha ao baixar a imagem",
    description: "Event label — image_pull_failed",
  },
  "webhooks.event.pipeline_minutes_exhausted": {
    message: "Minutos de pipeline esgotados",
    description: "Event label — pipeline_minutes_exhausted",
  },
  "webhooks.event.edge_cache_enabled": {
    message: "Cache de borda ativado",
    description:
      "Event label — edge_cache_enabled (API-only on Render; shown on subscriptions and deliveries, not in the picker)",
  },
  "webhooks.event.edge_cache_disabled": {
    message: "Cache de borda desativado",
    description:
      "Event label — edge_cache_disabled (API-only on Render; shown on subscriptions and deliveries, not in the picker)",
  },
  "webhooks.event.edge_cache_purged": {
    message: "Cache de borda limpo",
    description:
      "Event label — edge_cache_purged (API-only on Render; shown on subscriptions and deliveries, not in the picker)",
  },
  "webhooks.event.commit_ignored": {
    message: "Commit ignorado",
    description: "Event label — commit_ignored",
  },
  "webhooks.event.server_failed": {
    message: "Falha no servidor",
    description: "Event label — server_failed",
  },
  "webhooks.event.server_hardware_failure": {
    message: "Falha de hardware do servidor",
    description: "Event label — server_hardware_failure",
  },
  "webhooks.event.server_available": {
    message: "Servidor disponível",
    description: "Event label — server_available",
  },
  "webhooks.event.autoscaling_started": {
    message: "Escalonamento automático iniciado",
    description: "Event label — autoscaling_started",
  },
  "webhooks.event.autoscaling_ended": {
    message: "Escalonamento automático concluído",
    description: "Event label — autoscaling_ended",
  },
  "webhooks.event.branch_changed": {
    message: "Branch alterado",
    description: "Event label — branch_changed bex extension",
  },
  "webhooks.event.cron_job_run_started": {
    message: "Execução de Cron Job iniciada",
    description: "Event label — cron_job_run_started",
  },
  "webhooks.event.cron_job_run_ended": {
    message: "Execução de Cron Job concluída",
    description: "Event label — cron_job_run_ended",
  },
  "webhooks.event.maintenance_mode_enabled": {
    message: "Modo de manutenção ativado",
    description: "Event label — maintenance_mode_enabled",
  },
  "webhooks.event.maintenance_mode_uri_updated": {
    message: "URI do modo de manutenção atualizada",
    description: "Event label — maintenance_mode_uri_updated",
  },
  "webhooks.event.disk_created": {
    message: "Disco criado",
    description: "Event label — disk_created",
  },
  "webhooks.event.disk_updated": {
    message: "Disco atualizado",
    description: "Event label — disk_updated",
  },
  "webhooks.event.disk_deleted": {
    message: "Disco excluído",
    description: "Event label — disk_deleted",
  },
  "webhooks.event.disk_snapshot_restored": {
    message: "Snapshot do disco restaurado",
    description: "Event label — disk_snapshot_restored",
  },
  "webhooks.event.postgres_created": {
    message: "Postgres criado",
    description: "Event label — postgres_created",
  },
  "webhooks.event.postgres_restarted": {
    message: "Postgres reiniciado",
    description: "Event label — postgres_restarted",
  },
  "webhooks.event.postgres_credentials_created": {
    message: "Credenciais do Postgres criadas",
    description: "Event label — postgres_credentials_created",
  },
  "webhooks.event.postgres_credentials_deleted": {
    message: "Credenciais do Postgres excluídas",
    description: "Event label — postgres_credentials_deleted",
  },
  "webhooks.event.postgres_backup_started": {
    message: "Backup do Postgres iniciado",
    description: "Event label — postgres_backup_started",
  },
  "webhooks.event.postgres_available": {
    message: "Postgres disponível",
    description: "Event label — postgres_available",
  },
  "webhooks.event.postgres_unavailable": {
    message: "Postgres indisponível",
    description: "Event label — postgres_unavailable",
  },
  "webhooks.event.postgres_ha_status_changed": {
    message: "Alta disponibilidade do Postgres alterada",
    description: "Event label — postgres_ha_status_changed",
  },
  "webhooks.event.postgres_cluster_leader_changed": {
    message: "Líder do cluster Postgres alterado",
    description: "Event label — postgres_cluster_leader_changed",
  },
  "webhooks.event.postgres_connection_pool_enabled_changed": {
    message: "Pool de conexões do Postgres alterado",
    description: "Event label — postgres_connection_pool_enabled_changed",
  },
  "webhooks.event.postgres_disk_size_changed": {
    message: "Tamanho do disco do Postgres alterado",
    description: "Event label — postgres_disk_size_changed",
  },
  "webhooks.event.postgres_disk_autoscaling_enabled_changed": {
    message: "Escalonamento automático do disco do Postgres alterado",
    description: "Event label — postgres_disk_autoscaling_enabled_changed",
  },
  "webhooks.event.postgres_read_replicas_changed": {
    message: "Réplicas de leitura do Postgres alteradas",
    description: "Event label — postgres_read_replicas_changed",
  },
  "webhooks.event.postgres_backup_completed": {
    message: "Backup do Postgres concluído",
    description: "Event label — postgres_backup_completed",
  },
  "webhooks.event.postgres_backup_failed": {
    message: "Backup do Postgres falhou",
    description: "Event label — postgres_backup_failed",
  },
  "webhooks.event.postgres_pitr_checkpoint_started": {
    message: "Checkpoint PITR do Postgres iniciado",
    description: "Event label — postgres_pitr_checkpoint_started",
  },
  "webhooks.event.postgres_pitr_checkpoint_completed": {
    message: "Checkpoint PITR do Postgres concluído",
    description: "Event label — postgres_pitr_checkpoint_completed",
  },
  "webhooks.event.postgres_pitr_checkpoint_failed": {
    message: "Checkpoint PITR do Postgres falhou",
    description: "Event label — postgres_pitr_checkpoint_failed",
  },
  "webhooks.event.postgres_wal_archive_failed": {
    message: "Arquivamento de WAL do Postgres falhou",
    description: "Event label — postgres_wal_archive_failed",
  },
  "webhooks.event.postgres_restore_succeeded": {
    message: "Restauração do Postgres bem-sucedida",
    description: "Event label — postgres_restore_succeeded",
  },
  "webhooks.event.postgres_restore_failed": {
    message: "Restauração do Postgres falhou",
    description: "Event label — postgres_restore_failed",
  },
  "webhooks.event.postgres_upgrade_started": {
    message: "Atualização do Postgres iniciada",
    description: "Event label — postgres_upgrade_started",
  },
  "webhooks.event.postgres_upgrade_succeeded": {
    message: "Atualização do Postgres bem-sucedida",
    description: "Event label — postgres_upgrade_succeeded",
  },
  "webhooks.event.postgres_upgrade_failed": {
    message: "Atualização do Postgres falhou",
    description: "Event label — postgres_upgrade_failed",
  },
  "webhooks.event.key_value_available": {
    message: "Key Value disponível",
    description: "Event label — key_value_available",
  },
  "webhooks.event.key_value_unhealthy": {
    message: "Key Value com problemas",
    description: "Event label — key_value_unhealthy",
  },
  "webhooks.event.key_value_config_restart": {
    message: "Key Value reiniciado por mudança de configuração",
    description: "Event label — key_value_config_restart",
  },
  "webhooks.event.service_suspended": {
    message: "Serviço suspenso",
    description: "Event label — service_suspended",
  },
  "webhooks.event.service_resumed": {
    message: "Serviço retomado",
    description: "Event label — service_resumed",
  },
  "webhooks.event.service_hibernated": {
    message: "O serviço entrou em repouso",
    description: "Event label — service_hibernated",
  },
  "webhooks.event.service_woken": {
    message: "O serviço acordou",
    description: "Event label — service_woken",
  },
  "webhooks.event.agent_session_turn_succeeded": {
    message: "Turno da sessão do agente bem-sucedido",
    description: "Event label — agent_session_turn_succeeded",
  },
  "webhooks.event.agent_session_turn_failed": {
    message: "Turno da sessão do agente falhou",
    description: "Event label — agent_session_turn_failed",
  },
  "webhooks.event.agent_session_needs_input": {
    message: "Sessão do agente precisa de entrada",
    description: "Event label — agent_session_needs_input",
  },
  "webhooks.event.service_moved": {
    message: "Serviço movido",
    description:
      "Event label — service_moved (bex extension: project/environment placement change)",
  },
  "webhooks.event.server_restarted": {
    message: "Servidor reiniciado",
    description: "Event label — server_restarted",
  },
  "webhooks.event.instance_count_changed": {
    message: "Número de instâncias alterado",
    description: "Event label — instance_count_changed",
  },
  "webhooks.event.autoscaling_config_changed": {
    message: "Configuração de escalonamento automático alterada",
    description: "Event label — autoscaling_config_changed",
  },
  "webhooks.event.plan_changed": {
    message: "Plano alterado",
    description:
      "Event label — plan_changed (bex's slot for Render's Instance Type Changed)",
  },
  "webhooks.newTitle": {
    message: "Criar um novo webhook",
    description: "Create page heading (Render: 'Create a new Webhook')",
  },
  "webhooks.newMenuLabel": {
    message: "Novo webhook",
    description:
      "+ New menu entry; worded like the other entries, unlike the create page heading",
  },
  "webhooks.backToList": {
    message: "Voltar para webhooks",
    description: "Accessible label of the create/detail pages' back link",
  },
  "webhooks.fieldNameHelp": {
    message: "Um nome único para este webhook.",
    description: "Create page name-field helper copy",
  },
  "webhooks.fieldUrlHelp": {
    message:
      "O bex envia cada notificação para esta URL como uma requisição POST.",
    description: "Create page URL-field helper copy",
  },
  "webhooks.fieldUrlPlaceholder": {
    message: "https://example.com/webhooks/bex",
    description: "Create page URL-field placeholder",
  },
  "webhooks.fieldEventsHelp": {
    message:
      "Escolha quais eventos do seu workspace dispararão uma notificação de webhook.",
    description: "Create page events-field helper copy",
  },
  "webhooks.detailKicker": {
    message: "Webhook",
    description: "Detail page header kicker above the endpoint name",
  },
  "webhooks.idLabel": {
    message: "ID do webhook:",
    description: "Detail header id row label",
  },
  "webhooks.copyId": {
    message: "Copiar ID do webhook",
    description: "Accessible label of the header id copy button",
  },
  "webhooks.copyUrl": {
    message: "Copiar URL do endpoint",
    description: "Accessible label of the header URL copy button",
  },
  "webhooks.copiedGeneric": {
    message: "Copiado para a área de transferência",
    description: "Toast after copying the id or URL",
  },
  "webhooks.showMore": {
    message: "Mostrar mais {count}",
    description: "Event-chip expander on the detail header",
  },
  "webhooks.showLess": {
    message: "Mostrar menos",
    description: "Event-chip collapser on the detail header",
  },
  "webhooks.createdByOn": {
    message: "Criado por {creator} em {date}",
    description: "Detail header provenance line when the creator is known",
  },
  "webhooks.createdOn": {
    message: "Criado em {date}",
    description: "Detail header provenance line without a creator",
  },
  "webhooks.tabActivity": {
    message: "Atividade",
    description: "Detail page tab — delivery history",
  },
  "webhooks.tabSettings": {
    message: "Configurações",
    description: "Detail page tab — edit + delete",
  },
  "webhooks.recentDeliveries": {
    message: "Entregas recentes",
    description: "Activity tab heading",
  },
  "webhooks.recentDeliveriesHint": {
    message:
      "Cada linha é uma tentativa de entrega. Novas tentativas aparecem automaticamente enquanto esta aba estiver visível.",
    description: "Activity tab heading hint for immutable attempts + polling",
  },
  "webhooks.refresh": {
    message: "Atualizar",
    description: "Accessible label of the Activity refresh button",
  },
  "webhooks.filterAll": {
    message: "Todas",
    description: "Activity delivery filter tab",
  },
  "webhooks.filterSuccessful": {
    message: "Bem-sucedidas",
    description: "Activity delivery filter tab — delivered only",
  },
  "webhooks.filterFailed": {
    message: "Com falha",
    description: "Activity delivery filter tab — failed only",
  },
  "webhooks.sentAfter": {
    message: "Enviado depois de",
    description: "Activity delivery-history lower timestamp bound",
  },
  "webhooks.sentBefore": {
    message: "Enviado antes de",
    description: "Activity delivery-history upper timestamp bound",
  },
  "webhooks.destinationBlocked": {
    message: "Destino não permitido",
    description:
      "Delivery result label when bex refused to dial the destination: it resolves to a loopback, private, link-local or cluster-internal address (w21/046)",
  },
  "webhooks.destinationUnreachableNotice": {
    message:
      "Este endereço não é acessível pela internet, então toda entrega para ele falhará. Use um endereço HTTPS público.",
    description:
      "Non-blocking notice under the webhook Destination URL when its host is a loopback, private, link-local or cluster-internal address (w21/046)",
  },
  "webhooks.transportError": {
    message: "Erro de transporte",
    description: "Delivery evidence label when no HTTP response was received",
  },
  "webhooks.enabledBadge": {
    message: "Ativado",
    description: "Detail header status badge while enabled",
  },
  "webhooks.disabledBadge": {
    message: "Desativado",
    description: "Detail header status badge while disabled",
  },
  "webhooks.view": {
    message: "Ver webhook",
    description: "Accessible label of the list row's open-detail link",
  },
  "webhooks.settingsGeneral": {
    message: "Geral",
    description: "Settings tab first section heading",
  },
  "webhooks.disabledReasonAuto": {
    message: "Desativado automaticamente após falhas de entrega repetidas.",
    description:
      "Reason shown beside a webhook the platform disabled after repeated delivery failures",
  },
  "webhooks.settingsStatus": {
    message: "Status",
    description: "Settings status-toggle label",
  },
  "webhooks.settingsStatusHelp": {
    message:
      "Webhooks não enviam nenhuma notificação enquanto estiverem desativados.",
    description: "Settings status-toggle helper copy",
  },
  "webhooks.settingsEvents": {
    message: "Eventos assinados",
    description: "Settings events section heading",
  },
  "webhooks.saveChanges": {
    message: "Salvar alterações",
    description: "Settings submit button (disabled until dirty)",
  },
  "webhooks.updateSuccess": {
    message: "Webhook atualizado",
    description: "Toast after a successful settings save",
  },
  "webhooks.updateError": {
    message: "Não foi possível atualizar o webhook",
    description: "Toast after a failed settings save",
  },
  "webhooks.secretLabel": {
    message: "Segredo de assinatura",
    description: "Settings signing-secret row label",
  },
  "webhooks.secretHelp": {
    message:
      "Um token usado para assinar e verificar os payloads das requisições. Somente administradores do workspace podem vê-lo.",
    description: "Settings signing-secret row help (Render's wording)",
  },
  "webhooks.secretShow": {
    message: "Mostrar segredo",
    description: "Settings button that reveals the masked signing secret",
  },
  "webhooks.secretHide": {
    message: "Ocultar segredo",
    description: "Settings button that masks the revealed signing secret",
  },
  "webhooks.deleteSection": {
    message: "Excluir webhook",
    description: "Settings danger-zone heading",
  },
  "webhooks.deleteTypeToConfirm": {
    message: "Digite {command} abaixo para confirmar.",
    description: "Type-to-confirm instruction in the delete dialog",
  },
  "webhooks.deleteCommandLabel": {
    message: "Confirmação",
    description: "Accessible label of the type-to-confirm input",
  },
};

export default ptWebhooks;
