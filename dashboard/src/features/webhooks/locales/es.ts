import type { TranslationEntry } from "@/i18n";

const esWebhooks: Record<string, TranslationEntry> = {
  "webhooks.title": {
    message: "Webhooks",
    description: "Settings Integrations Webhooks section card title",
  },
  "webhooks.description": {
    message:
      "Envía notificaciones de eventos firmadas (despliegues, suspensiones, reinicios, escalado) a tus propios endpoints, sin sondeo. El secreto de firma de cada webhook está en su configuración.",
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
    message: "Activado",
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
    message: "Busca por nombre, destino o evento",
    description: "Endpoint list search helper text",
  },
  "webhooks.searchEndpointsEmpty": {
    message: "Ningún webhook coincide con tu búsqueda.",
    description: "Endpoint list empty state after local search",
  },
  "webhooks.emptyTitle": {
    message: "No hay webhooks",
    description: "Webhooks empty-state title",
  },
  "webhooks.emptyBody": {
    message:
      "Añade un endpoint para recibir notificaciones cuando tus servicios se desplieguen, se reinicien o escalen.",
    description: "Webhooks empty-state body",
  },
  "webhooks.forbiddenTitle": {
    message: "No autorizado",
    description: "Webhooks state when the caller lacks permission (403)",
  },
  "webhooks.forbiddenBody": {
    message:
      "No tienes permiso para ver los webhooks de este espacio de trabajo.",
    description: "Webhooks forbidden-state body",
  },
  "webhooks.errorTitle": {
    message: "No se pudieron cargar los webhooks",
    description: "Webhooks generic error title",
  },
  "webhooks.errorBody": {
    message: "Algo salió mal. Inténtalo de nuevo.",
    description: "Webhooks generic error body",
  },
  "webhooks.proOnly": {
    message:
      "Los webhooks requieren el plan Pro o superior. Los endpoints existentes siguen entregando.",
    description: "Webhooks card description on a Hobby workspace (w8/m138)",
  },
  "webhooks.create": {
    message: "Añadir webhook",
    description: "Button that opens the create dialog",
  },
  "webhooks.createTitle": {
    message: "Añadir un webhook",
    description: "Create dialog title",
  },
  "webhooks.createDescription": {
    message:
      "bex enviará por POST una carga JSON firmada a esta URL cada vez que ocurra un evento suscrito.",
    description: "Create dialog description",
  },
  "webhooks.createEnabledHelp": {
    message:
      "Empieza a entregar de inmediato. Puedes activarlo más tarde desde la configuración.",
    description: "Create page initial enabled-state helper copy",
  },
  "webhooks.fieldName": {
    message: "Nombre",
    description: "Create dialog name field label",
  },
  "webhooks.fieldNamePlaceholder": {
    message: "p. ej. deploy-alerts-slack-bot",
    description: "Create dialog name field placeholder",
  },
  "webhooks.fieldUrl": {
    message: "URL de destino",
    description: "Create dialog URL field label",
  },
  "webhooks.fieldEvents": {
    message: "Eventos que enviar",
    description: "Create dialog event-type checklist label",
  },
  "webhooks.eventsLoading": {
    message: "Cargando tipos de evento…",
    description: "Create dialog while the event-type vocabulary loads",
  },
  "webhooks.eventsLoadError": {
    message:
      "No se pudieron cargar los tipos de evento. Reintenta antes de guardar.",
    description: "Actionable event-catalog query failure",
  },
  "webhooks.eventsRetry": {
    message: "Reintentar tipos de evento",
    description: "Retry button for the event-catalog query",
  },
  "webhooks.eventsEmpty": {
    message:
      "No hay tipos de evento disponibles para suscribirse en este momento.",
    description: "Event picker state when the served catalog is empty",
  },
  "webhooks.formUnavailableEvents": {
    message: "No se puede crear hasta que se carguen los tipos de evento.",
    description:
      "Create submit disabled reason while the catalog is unavailable",
  },
  "webhooks.formErrorsSummary": {
    message: "Corrige los campos resaltados e inténtalo de nuevo.",
    description: "Accessible webhook form validation summary",
  },
  "webhooks.validation.nameRequired": {
    message: "Introduce un nombre para el webhook.",
    description: "Field-local required-name validation",
  },
  "webhooks.validation.nameDuplicate": {
    message: "Ya existe un webhook con este nombre.",
    description: "Field-local duplicate-name validation",
  },
  "webhooks.validation.urlRequired": {
    message: "Introduce una URL de destino.",
    description: "Field-local required URL validation",
  },
  "webhooks.validation.urlHTTPS": {
    message: "Introduce una URL HTTPS absoluta sin credenciales incrustadas.",
    description: "Field-local webhook URL validation",
  },
  "webhooks.validation.eventsRequired": {
    message: "Selecciona al menos un evento.",
    description: "Field-local event selection validation",
  },
  "webhooks.validation.eventsInvalid": {
    message:
      "La selección de eventos cambió. Recarga el catálogo y elige de nuevo.",
    description: "Server-coded stale/invalid event filter",
  },
  "webhooks.errors.planLimit": {
    message:
      "Los webhooks están disponibles en el plan Pro y superiores. Este espacio de trabajo tiene el plan Hobby, por lo que no puede añadir un endpoint.",
    description:
      "Server-coded plan refusal of a webhook create on a Hobby workspace (WEBHOOK_PLAN_LIMIT, w4/425)",
  },
  "webhooks.planGate": {
    message:
      "Los webhooks están disponibles en el plan Pro y superiores. Mejora este espacio de trabajo para añadir un endpoint.",
    description:
      "Notice in place of the create form and the empty list on a Hobby workspace (w4/425)",
  },
  "webhooks.errors.endpointLimit": {
    message:
      "Este espacio de trabajo ha alcanzado su límite de endpoints de webhook.",
    description: "Server-coded endpoint quota refusal",
  },
  "webhooks.errors.endpointDisabled": {
    message: "Activa este endpoint antes de reenviar una entrega.",
    description: "Server-coded resend refusal for a disabled endpoint",
  },
  "webhooks.errors.deliveryPending": {
    message: "Ya hay un intento de entrega en cola. Espera a que termine.",
    description: "Server-coded duplicate pending resend refusal",
  },
  "webhooks.errors.staleState": {
    message:
      "Este webhook cambió o se eliminó. Actualiza e inténtalo de nuevo.",
    description: "Safe stale endpoint/attempt refusal",
  },
  "webhooks.manageRequired": {
    message:
      "Puedes inspeccionar este webhook, pero solo los gestores del espacio de trabajo pueden modificarlo.",
    description: "Read-only member notice on webhook management surfaces",
  },
  "webhooks.latestNever": {
    message: "Nunca enviado",
    description: "Endpoint with no completed delivery attempt",
  },
  "webhooks.latestDelivered": {
    message: "Correcta",
    description: "Latest endpoint attempt succeeded",
  },
  "webhooks.latestFailed": {
    message: "Fallida",
    description: "Latest endpoint attempt failed terminally",
  },
  "webhooks.latestRetrying": {
    message: "Reintentando",
    description: "Latest endpoint attempt failed with a retry pending",
  },
  "webhooks.createCancel": {
    message: "Cancelar",
    description: "Create dialog cancel button",
  },
  "webhooks.createSubmit": {
    message: "Crear webhook",
    description: "Create page submit button (Render: 'Create Webhook')",
  },
  "webhooks.createSuccess": {
    message: "Webhook creado",
    description: "Toast after a successful create",
  },
  "webhooks.createError": {
    message: "No se pudo crear el webhook",
    description: "Toast after a failed create",
  },
  "webhooks.copy": {
    message: "Copiar secreto de firma",
    description: "Copy-secret button label",
  },
  "webhooks.copied": {
    message: "Secreto copiado al portapapeles",
    description: "Toast after copying the secret",
  },
  "webhooks.copyError": {
    message: "No se pudo copiar: selecciona el texto y cópialo manualmente",
    description: "Toast when clipboard write fails",
  },
  "webhooks.toggle": {
    message: "Activado",
    description: "Accessible label of the per-endpoint enable/disable switch",
  },
  "webhooks.enableSuccess": {
    message: "{name} activado",
    description: "Toast after enabling an endpoint",
  },
  "webhooks.disableSuccess": {
    message: "{name} desactivado",
    description: "Toast after disabling an endpoint",
  },
  "webhooks.toggleError": {
    message: "No se pudo actualizar {name}",
    description: "Toast after a failed enable/disable",
  },
  "webhooks.delete": {
    message: "Eliminar",
    description: "Delete action label (icon button + confirm button)",
  },
  "webhooks.deleteConfirmTitle": {
    message: "¿Eliminar {name}?",
    description: "Delete confirmation dialog title",
  },
  "webhooks.deleteConfirmBody": {
    message:
      "No se enviarán más eventos a este endpoint y se eliminará su historial de entregas.",
    description: "Delete confirmation dialog body",
  },
  "webhooks.deleteCancel": {
    message: "Cancelar",
    description: "Delete confirmation cancel button",
  },
  "webhooks.deleteSuccess": {
    message: "{name} eliminado",
    description: "Toast after a successful delete",
  },
  "webhooks.deleteError": {
    message: "No se pudo eliminar {name}",
    description: "Toast after a failed delete",
  },
  "webhooks.history": {
    message: "Historial de entregas",
    description: "Accessible label of the per-endpoint history button",
  },
  "webhooks.historyTitle": {
    message: "Entregas — {name}",
    description: "Delivery-history dialog title",
  },
  "webhooks.historyBody": {
    message:
      "Cada evento enviado a este endpoint, del más reciente al más antiguo. Las entregas fallidas se reintentan con un retroceso exponencial; si fallan repetidamente, el endpoint se desactiva.",
    description: "Delivery-history dialog description",
  },
  "webhooks.historyEmptyTitle": {
    message: "Aún no hay entregas",
    description: "Delivery-history empty-state title",
  },
  "webhooks.historyEmptyBody": {
    message:
      "Provoca un evento suscrito, por ejemplo un despliegue, para verlo aquí.",
    description: "Delivery-history empty-state body",
  },
  "webhooks.historyFilteredEmptySuccessfulTitle": {
    message: "No hay entregas correctas",
    description:
      "Delivery-history state when the Successful tab (and any date range) matches nothing (w4/146)",
  },
  "webhooks.historyFilteredEmptyFailedTitle": {
    message: "No hay entregas fallidas",
    description:
      "Delivery-history state when the Failed tab (and any date range) matches nothing (w4/146)",
  },
  "webhooks.historyFilteredEmptyRangeTitle": {
    message: "No hay entregas en este intervalo",
    description:
      "Delivery-history state when the Sent after/before range matches nothing (w4/146)",
  },
  "webhooks.historyFilteredEmptyBody": {
    message:
      "Otras entregas pueden coincidir con un estado o intervalo de tiempo distinto.",
    description: "Body under a filtered-empty delivery history (w4/146)",
  },
  "webhooks.historyClearFilters": {
    message: "Borrar filtros",
    description:
      "Resets the delivery-history status tab and date range (w4/146)",
  },
  "webhooks.historyErrorTitle": {
    message: "No se pudieron cargar las entregas",
    description: "Delivery-history error-state title",
  },
  "webhooks.historyErrorBody": {
    message: "Algo salió mal. Inténtalo de nuevo.",
    description: "Delivery-history error-state body",
  },
  "webhooks.colEvent": {
    message: "Evento",
    description: "Delivery-history table column header",
  },
  "webhooks.colService": {
    message: "Servicio",
    description: "Delivery-history table column header",
  },
  "webhooks.serviceDeleted": {
    message: "(eliminado)",
    description:
      "Marker beside a delivery's recorded service name when that service no longer exists, so the name cannot be linked",
  },
  "webhooks.colStatus": {
    message: "Estado",
    description: "Delivery-history table column header",
  },
  "webhooks.colAttempts": {
    message: "Intento",
    description:
      "Delivery-history table column header — one-based attempt number",
  },
  "webhooks.colResponse": {
    message: "Respuesta",
    description: "Delivery-history table column header — last HTTP status",
  },
  "webhooks.colWhen": {
    message: "Cuándo",
    description:
      "Delivery-history table column header — relative and exact send time",
  },
  "webhooks.colActions": {
    message: "Acciones",
    description: "Accessible delivery-history table actions column header",
  },
  "webhooks.status.pending": {
    message: "Pendiente",
    description: "Delivery status badge — queued or between retries",
  },
  "webhooks.status.delivered": {
    message: "Entregado",
    description: "Delivery status badge — endpoint answered 2xx",
  },
  "webhooks.status.failed": {
    message: "Fallido",
    description: "Delivery status badge — this individual attempt failed",
  },
  "webhooks.httpStatus": {
    message: "HTTP {status}",
    description: "Delivery attempt HTTP status summary",
  },
  "webhooks.openSourceEvent": {
    message: "{label} — abrir evento de origen {id}",
    description:
      "Accessible label for the event hydration link; it starts with the link's visible text, the event type (WCAG 2.5.3)",
  },
  "webhooks.requestPayload": {
    message: "Carga de la solicitud",
    description: "Expanded delivery-attempt request evidence heading",
  },
  "webhooks.endpointResponse": {
    message: "Respuesta del endpoint",
    description: "Expanded delivery-attempt response evidence heading",
  },
  "webhooks.noRequestEvidence": {
    message: "No hay evidencia de la solicitud disponible.",
    description: "Expanded attempt with no retained request body",
  },
  "webhooks.noResponseEvidence": {
    message: "No se registró ninguna respuesta.",
    description: "Expanded pending/transport attempt with no endpoint body",
  },
  "webhooks.attemptIdentity": {
    message: "Intento {attemptId} · evento de origen {eventId}",
    description: "Immutable attempt and stable source-event identity",
  },
  "webhooks.parentDeliveryStatus": {
    message: "Estado de la notificación: {status}",
    description:
      "Logical notification state shown separately from attempt outcome",
  },
  "webhooks.retryScheduled": {
    message: "Próximo intento automático: {date}",
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
    message: "¿Reenviar este webhook?",
    description: "Manual resend confirmation title",
  },
  "webhooks.resendConfirmBody": {
    message:
      "Esto pone en cola un nuevo intento firmado con el mismo evento de origen y la misma carga de solicitud.",
    description: "Manual resend confirmation consequence",
  },
  "webhooks.resendCancel": {
    message: "Cancelar",
    description: "Manual resend confirmation cancel action",
  },
  "webhooks.resendSuccess": {
    message: "Reenvío del webhook en cola",
    description: "Toast after a manual resend reservation succeeds",
  },
  "webhooks.resendError": {
    message: "No se pudo reenviar este webhook",
    description: "Toast after a manual resend is refused or fails",
  },
  "webhooks.loadMore": {
    message: "Cargar más",
    description: "Delivery-history pagination button",
  },
  "webhooks.selectedCount_one": {
    message: "{count} evento seleccionado",
    description: "Event-picker live selection counter (singular)",
  },
  "webhooks.selectedCount_many": {
    message: "{count} de eventos seleccionados",
    description: "Event-picker live selection counter (plural)",
  },
  "webhooks.selectedCount_other": {
    message: "{count} eventos seleccionados",
    description: "Event-picker live selection counter (plural)",
  },
  "webhooks.searchEvents": {
    message: "Buscar eventos",
    description: "Event-picker search box placeholder + accessible label",
  },
  "webhooks.searchNoMatches": {
    message: "Ningún evento coincide con tu búsqueda",
    description:
      "Event-picker empty state while a search filters everything out",
  },
  "webhooks.allEvents": {
    message: "Todos los eventos",
    description: "Event-picker tri-state master checkbox label",
  },
  "webhooks.groupToggle": {
    message: "Alternar los eventos de {group}",
    description: "Accessible label of a group's expand/collapse chevron",
  },
  "webhooks.group.deploy": {
    message: "Despliegue",
    description: "Event-picker group — deploy lifecycle events",
  },
  "webhooks.group.autoDeploy": {
    message: "Despliegue automático",
    description: "Event-picker group — automatic deploy setting events",
  },
  "webhooks.group.serviceAvailability": {
    message: "Disponibilidad del servicio",
    description: "Event-picker group — observed service availability events",
  },
  "webhooks.group.scaling": {
    message: "Escalado",
    description: "Event-picker group — service scaling events",
  },
  "webhooks.group.cronJobRun": {
    message: "Ejecución de Cron Job",
    description: "Event-picker group — cron job run events",
  },
  "webhooks.group.maintenanceMode": {
    message: "Modo de mantenimiento",
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
    message: "Suspensión",
    description: "Event-picker group — suspend/resume events",
  },
  "webhooks.group.agentSession": {
    message: "Agentes en la nube",
    description: "Event-picker group — cloud coding agent session events",
  },
  "webhooks.group.other": {
    message: "Otros",
    description:
      "Event-picker fallback group for served keys the catalog doesn't know yet",
  },
  "webhooks.event.deploy_started": {
    message: "Despliegue iniciado",
    description: "Event label — deploy_started",
  },
  "webhooks.event.branch_deleted": {
    message: "Rama eliminada",
    description: "Webhook event label",
  },
  "webhooks.event.build_started": {
    message: "Build iniciado",
    description: "Webhook event label",
  },
  "webhooks.event.build_ended": {
    message: "Build finalizado",
    description: "Webhook event label",
  },
  "webhooks.event.pre_deploy_started": {
    message: "Predespliegue iniciado",
    description: "Webhook event label",
  },
  "webhooks.event.pre_deploy_ended": {
    message: "Predespliegue finalizado",
    description: "Webhook event label",
  },
  "webhooks.event.job_run_ended": {
    message: "Ejecución de job finalizada",
    description: "Webhook event label",
  },
  "webhooks.event.auto_deploy_enabled": {
    message: "Despliegue automático activado",
    description: "Webhook event label",
  },
  "webhooks.event.auto_deploy_disabled": {
    message: "Despliegue automático desactivado",
    description: "Webhook event label",
  },
  "webhooks.event.deploy_ended": {
    message: "Despliegue finalizado",
    description: "Event label — deploy_ended",
  },
  "webhooks.event.zero_downtime_redeploy_started": {
    message: "Redespliegue sin interrupciones iniciado",
    description:
      "Event label — zero_downtime_redeploy_started: a Bex-initiated zero-downtime deploy started for a service",
  },
  "webhooks.event.zero_downtime_redeploy_ended": {
    message: "Redespliegue sin interrupciones finalizado",
    description:
      "Event label — zero_downtime_redeploy_ended: a Bex-initiated zero-downtime deploy completed for a service",
  },
  "webhooks.event.image_pull_failed": {
    message: "Falló la descarga de la imagen",
    description: "Event label — image_pull_failed",
  },
  "webhooks.event.artifact_fetch_failed": {
    message: "Error al obtener el artefacto",
    description:
      "Event label — artifact_fetch_failed: a build published by a build source could not be fetched",
  },
  "webhooks.event.artifact_source_changed": {
    message: "Origen de compilación cambiado",
    description:
      "Event label — artifact_source_changed: a service's linked build source changed",
  },
  "webhooks.event.pipeline_minutes_exhausted": {
    message: "Minutos de pipeline agotados",
    description: "Event label — pipeline_minutes_exhausted",
  },
  "webhooks.event.edge_cache_enabled": {
    message: "Caché perimetral activada",
    description:
      "Event label — edge_cache_enabled (API-only on Render; shown on subscriptions and deliveries, not in the picker)",
  },
  "webhooks.event.edge_cache_disabled": {
    message: "Caché perimetral desactivada",
    description:
      "Event label — edge_cache_disabled (API-only on Render; shown on subscriptions and deliveries, not in the picker)",
  },
  "webhooks.event.edge_cache_purged": {
    message: "Caché perimetral purgada",
    description:
      "Event label — edge_cache_purged (API-only on Render; shown on subscriptions and deliveries, not in the picker)",
  },
  "webhooks.event.commit_ignored": {
    message: "Commit ignorado",
    description: "Event label — commit_ignored",
  },
  "webhooks.event.server_failed": {
    message: "Servidor fallido",
    description: "Event label — server_failed",
  },
  "webhooks.event.server_hardware_failure": {
    message: "Fallo de hardware del servidor",
    description: "Event label — server_hardware_failure",
  },
  "webhooks.event.server_available": {
    message: "Servidor disponible",
    description: "Event label — server_available",
  },
  "webhooks.event.autoscaling_started": {
    message: "Escalado automático iniciado",
    description: "Event label — autoscaling_started",
  },
  "webhooks.event.autoscaling_ended": {
    message: "Escalado automático finalizado",
    description: "Event label — autoscaling_ended",
  },
  "webhooks.event.branch_changed": {
    message: "Rama cambiada",
    description: "Event label — branch_changed bex extension",
  },
  "webhooks.event.cron_job_run_started": {
    message: "Ejecución de Cron Job iniciada",
    description: "Event label — cron_job_run_started",
  },
  "webhooks.event.cron_job_run_ended": {
    message: "Ejecución de Cron Job finalizada",
    description: "Event label — cron_job_run_ended",
  },
  "webhooks.event.maintenance_mode_enabled": {
    message: "Modo de mantenimiento activado",
    description: "Event label — maintenance_mode_enabled",
  },
  "webhooks.event.maintenance_mode_uri_updated": {
    message: "URI del modo de mantenimiento actualizada",
    description: "Event label — maintenance_mode_uri_updated",
  },
  "webhooks.event.disk_created": {
    message: "Disco creado",
    description: "Event label — disk_created",
  },
  "webhooks.event.disk_updated": {
    message: "Disco actualizado",
    description: "Event label — disk_updated",
  },
  "webhooks.event.disk_deleted": {
    message: "Disco eliminado",
    description: "Event label — disk_deleted",
  },
  "webhooks.event.disk_snapshot_restored": {
    message: "Snapshot de disco restaurado",
    description: "Event label — disk_snapshot_restored",
  },
  "webhooks.event.postgres_created": {
    message: "Postgres creado",
    description: "Event label — postgres_created",
  },
  "webhooks.event.postgres_restarted": {
    message: "Postgres reiniciado",
    description: "Event label — postgres_restarted",
  },
  "webhooks.event.postgres_credentials_created": {
    message: "Credenciales de Postgres creadas",
    description: "Event label — postgres_credentials_created",
  },
  "webhooks.event.postgres_credentials_deleted": {
    message: "Credenciales de Postgres eliminadas",
    description: "Event label — postgres_credentials_deleted",
  },
  "webhooks.event.postgres_backup_started": {
    message: "Copia de seguridad de Postgres iniciada",
    description: "Event label — postgres_backup_started",
  },
  "webhooks.event.postgres_available": {
    message: "Postgres disponible",
    description: "Event label — postgres_available",
  },
  "webhooks.event.postgres_unavailable": {
    message: "Postgres no disponible",
    description: "Event label — postgres_unavailable",
  },
  "webhooks.event.postgres_ha_status_changed": {
    message: "Alta disponibilidad de Postgres cambiada",
    description: "Event label — postgres_ha_status_changed",
  },
  "webhooks.event.postgres_cluster_leader_changed": {
    message: "Líder del clúster de Postgres cambiado",
    description: "Event label — postgres_cluster_leader_changed",
  },
  "webhooks.event.postgres_connection_pool_enabled_changed": {
    message: "Pool de conexiones de Postgres cambiado",
    description: "Event label — postgres_connection_pool_enabled_changed",
  },
  "webhooks.event.postgres_disk_size_changed": {
    message: "Tamaño de disco de Postgres cambiado",
    description: "Event label — postgres_disk_size_changed",
  },
  "webhooks.event.postgres_disk_autoscaling_enabled_changed": {
    message: "Escalado automático de disco de Postgres cambiado",
    description: "Event label — postgres_disk_autoscaling_enabled_changed",
  },
  "webhooks.event.postgres_read_replicas_changed": {
    message: "Réplicas de lectura de Postgres cambiadas",
    description: "Event label — postgres_read_replicas_changed",
  },
  "webhooks.event.postgres_read_replica_stale": {
    message: "Réplica de lectura de Postgres desactualizada",
    description: "Event label — postgres_read_replica_stale",
  },
  "webhooks.event.postgres_backup_completed": {
    message: "Copia de seguridad de Postgres completada",
    description: "Event label — postgres_backup_completed",
  },
  "webhooks.event.postgres_backup_failed": {
    message: "Copia de seguridad de Postgres fallida",
    description: "Event label — postgres_backup_failed",
  },
  "webhooks.event.postgres_pitr_checkpoint_started": {
    message: "Checkpoint PITR de Postgres iniciado",
    description: "Event label — postgres_pitr_checkpoint_started",
  },
  "webhooks.event.postgres_pitr_checkpoint_completed": {
    message: "Checkpoint PITR de Postgres completado",
    description: "Event label — postgres_pitr_checkpoint_completed",
  },
  "webhooks.event.postgres_pitr_checkpoint_failed": {
    message: "Checkpoint PITR de Postgres fallido",
    description: "Event label — postgres_pitr_checkpoint_failed",
  },
  "webhooks.event.postgres_wal_archive_failed": {
    message: "Archivado de WAL de Postgres fallido",
    description: "Event label — postgres_wal_archive_failed",
  },
  "webhooks.event.postgres_restore_succeeded": {
    message: "Restauración de Postgres correcta",
    description: "Event label — postgres_restore_succeeded",
  },
  "webhooks.event.postgres_restore_failed": {
    message: "Restauración de Postgres fallida",
    description: "Event label — postgres_restore_failed",
  },
  "webhooks.event.postgres_upgrade_started": {
    message: "Actualización de Postgres iniciada",
    description: "Event label — postgres_upgrade_started",
  },
  "webhooks.event.postgres_upgrade_succeeded": {
    message: "Actualización de Postgres correcta",
    description: "Event label — postgres_upgrade_succeeded",
  },
  "webhooks.event.postgres_upgrade_failed": {
    message: "Actualización de Postgres fallida",
    description: "Event label — postgres_upgrade_failed",
  },
  "webhooks.event.key_value_available": {
    message: "Key Value disponible",
    description: "Event label — key_value_available",
  },
  "webhooks.event.key_value_unhealthy": {
    message: "Key Value en mal estado",
    description: "Event label — key_value_unhealthy",
  },
  "webhooks.event.key_value_config_restart": {
    message: "Key Value reiniciado por un cambio de configuración",
    description: "Event label — key_value_config_restart",
  },
  "webhooks.event.service_suspended": {
    message: "Servicio suspendido",
    description: "Event label — service_suspended",
  },
  "webhooks.event.service_resumed": {
    message: "Servicio reanudado",
    description: "Event label — service_resumed",
  },
  "webhooks.event.service_hibernated": {
    message: "El servicio se durmió",
    description: "Event label — service_hibernated",
  },
  "webhooks.event.service_woken": {
    message: "El servicio se despertó",
    description: "Event label — service_woken",
  },
  "webhooks.event.agent_session_turn_succeeded": {
    message: "Turno de sesión de agente correcto",
    description: "Event label — agent_session_turn_succeeded",
  },
  "webhooks.event.agent_session_turn_failed": {
    message: "Turno de sesión de agente fallido",
    description: "Event label — agent_session_turn_failed",
  },
  "webhooks.event.agent_session_needs_input": {
    message: "La sesión de agente necesita información",
    description: "Event label — agent_session_needs_input",
  },
  "webhooks.event.service_moved": {
    message: "Servicio movido",
    description:
      "Event label — service_moved (bex extension: project/environment placement change)",
  },
  "webhooks.event.server_restarted": {
    message: "Servidor reiniciado",
    description: "Event label — server_restarted",
  },
  "webhooks.event.instance_count_changed": {
    message: "Número de instancias cambiado",
    description: "Event label — instance_count_changed",
  },
  "webhooks.event.autoscaling_config_changed": {
    message: "Configuración de escalado automático cambiada",
    description: "Event label — autoscaling_config_changed",
  },
  "webhooks.event.plan_changed": {
    message: "Tipo de instancia cambiado",
    description:
      "Event label — plan_changed (bex's slot for Render's Instance Type Changed)",
  },
  "webhooks.newTitle": {
    message: "Crear un nuevo webhook",
    description: "Create page heading (Render: 'Create a new Webhook')",
  },
  "webhooks.newMenuLabel": {
    message: "Nuevo webhook",
    description:
      "+ New menu entry; worded like the other entries, unlike the create page heading",
  },
  "webhooks.backToList": {
    message: "Volver a webhooks",
    description: "Accessible label of the create/detail pages' back link",
  },
  "webhooks.fieldNameHelp": {
    message: "Un nombre único para este webhook.",
    description: "Create page name-field helper copy",
  },
  "webhooks.fieldUrlHelp": {
    message: "bex envía cada notificación a esta URL como una solicitud POST.",
    description: "Create page URL-field helper copy",
  },
  "webhooks.fieldUrlPlaceholder": {
    message: "https://example.com/webhooks/bex",
    description: "Create page URL-field placeholder",
  },
  "webhooks.fieldEventsHelp": {
    message:
      "Elige qué eventos de tu espacio de trabajo activarán una notificación de webhook.",
    description: "Create page events-field helper copy",
  },
  "webhooks.detailKicker": {
    message: "Webhook",
    description: "Detail page header kicker above the endpoint name",
  },
  "webhooks.idLabel": {
    message: "ID del webhook:",
    description: "Detail header id row label",
  },
  "webhooks.copyId": {
    message: "Copiar ID del webhook",
    description: "Accessible label of the header id copy button",
  },
  "webhooks.copyUrl": {
    message: "Copiar URL del endpoint",
    description: "Accessible label of the header URL copy button",
  },
  "webhooks.copiedGeneric": {
    message: "Copiado al portapapeles",
    description: "Toast after copying the id or URL",
  },
  "webhooks.showMore": {
    message: "Mostrar {count} más",
    description: "Event-chip expander on the detail header",
  },
  "webhooks.showLess": {
    message: "Mostrar menos",
    description: "Event-chip collapser on the detail header",
  },
  "webhooks.createdByOn": {
    message: "Creado por {creator} el {date}",
    description: "Detail header provenance line when the creator is known",
  },
  "webhooks.createdOn": {
    message: "Creado el {date}",
    description: "Detail header provenance line without a creator",
  },
  "webhooks.tabActivity": {
    message: "Actividad",
    description: "Detail page tab — delivery history",
  },
  "webhooks.tabSettings": {
    message: "Configuración",
    description: "Detail page tab — edit + delete",
  },
  "webhooks.recentDeliveries": {
    message: "Entregas recientes",
    description: "Activity tab heading",
  },
  "webhooks.recentDeliveriesHint": {
    message:
      "Cada fila es un intento de entrega. Los nuevos intentos aparecen automáticamente mientras esta pestaña está visible.",
    description: "Activity tab heading hint for immutable attempts + polling",
  },
  "webhooks.refresh": {
    message: "Actualizar",
    description: "Accessible label of the Activity refresh button",
  },
  "webhooks.filterAll": {
    message: "Todas",
    description: "Activity delivery filter tab",
  },
  "webhooks.filterSuccessful": {
    message: "Correctas",
    description: "Activity delivery filter tab — delivered only",
  },
  "webhooks.filterFailed": {
    message: "Fallidas",
    description: "Activity delivery filter tab — failed only",
  },
  "webhooks.sentAfter": {
    message: "Enviadas después de",
    description: "Activity delivery-history lower timestamp bound",
  },
  "webhooks.sentBefore": {
    message: "Enviadas antes de",
    description: "Activity delivery-history upper timestamp bound",
  },
  "webhooks.destinationBlocked": {
    message: "Destino no permitido",
    description:
      "Delivery result label when bex refused to dial the destination: it resolves to a loopback, private, link-local or cluster-internal address (w21/046)",
  },
  "webhooks.destinationUnreachableNotice": {
    message:
      "Esta dirección no es accesible desde internet, por lo que todas las entregas a ella fallarán. Usa una dirección HTTPS pública.",
    description:
      "Non-blocking notice under the webhook Destination URL when its host is a loopback, private, link-local or cluster-internal address (w21/046)",
  },
  "webhooks.transportError": {
    message: "Error de transporte",
    description: "Delivery evidence label when no HTTP response was received",
  },
  "webhooks.enabledBadge": {
    message: "Activado",
    description: "Detail header status badge while enabled",
  },
  "webhooks.disabledBadge": {
    message: "Desactivado",
    description: "Detail header status badge while disabled",
  },
  "webhooks.view": {
    message: "Ver webhook",
    description: "Accessible label of the list row's open-detail link",
  },
  "webhooks.settingsGeneral": {
    message: "General",
    description: "Settings tab first section heading",
  },
  "webhooks.disabledReasonAuto": {
    message: "Desactivado automáticamente tras fallos de entrega repetidos.",
    description:
      "Reason shown beside a webhook the platform disabled after repeated delivery failures",
  },
  "webhooks.settingsStatus": {
    message: "Estado",
    description: "Settings status-toggle label",
  },
  "webhooks.settingsStatusHelp": {
    message:
      "Mientras están desactivados, los webhooks no envían ninguna notificación.",
    description: "Settings status-toggle helper copy",
  },
  "webhooks.settingsEvents": {
    message: "Eventos suscritos",
    description: "Settings events section heading",
  },
  "webhooks.saveChanges": {
    message: "Guardar cambios",
    description: "Settings submit button (disabled until dirty)",
  },
  "webhooks.updateSuccess": {
    message: "Webhook actualizado",
    description: "Toast after a successful settings save",
  },
  "webhooks.updateError": {
    message: "No se pudo actualizar el webhook",
    description: "Toast after a failed settings save",
  },
  "webhooks.secretLabel": {
    message: "Secreto de firma",
    description: "Settings signing-secret row label",
  },
  "webhooks.secretHelp": {
    message:
      "Un token que se usa para firmar y verificar las cargas de las solicitudes. Solo los administradores del espacio de trabajo pueden verlo.",
    description: "Settings signing-secret row help (Render's wording)",
  },
  "webhooks.secretShow": {
    message: "Mostrar secreto",
    description: "Settings button that reveals the masked signing secret",
  },
  "webhooks.secretHide": {
    message: "Ocultar secreto",
    description: "Settings button that masks the revealed signing secret",
  },
  "webhooks.deleteSection": {
    message: "Eliminar webhook",
    description: "Settings danger-zone heading",
  },
  "webhooks.deleteTypeToConfirm": {
    message: "Escribe {command} abajo para confirmar.",
    description: "Type-to-confirm instruction in the delete dialog",
  },
  "webhooks.deleteCommandLabel": {
    message: "Confirmación",
    description: "Accessible label of the type-to-confirm input",
  },
};

export default esWebhooks;
