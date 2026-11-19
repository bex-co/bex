import type { TranslationEntry } from "@/i18n";

const deWebhooks: Record<string, TranslationEntry> = {
  "webhooks.title": {
    message: "Webhooks",
    description: "Settings Integrations Webhooks section card title",
  },
  "webhooks.description": {
    message:
      "Sende signierte Ereignisbenachrichtigungen (Deployments, Pausierungen, Neustarts, Skalierung) an deine eigenen Endpunkte — ohne Polling. Das Signing Secret jedes Webhooks findest du in seinen Einstellungen.",
    description: "Settings Webhooks section card description",
  },
  "webhooks.colEndpoint": {
    message: "Endpunkt",
    description: "Webhooks table column header — name + destination URL",
  },
  "webhooks.colEvents": {
    message: "Ereignisse",
    description: "Webhooks table column header — subscribed event types",
  },
  "webhooks.colEnabled": {
    message: "Aktiviert",
    description: "Webhooks table column header — the enable/disable switch",
  },
  "webhooks.colLatest": {
    message: "Letzte Zustellung",
    description: "Webhooks table column header — newest immutable attempt",
  },
  "webhooks.searchEndpoints": {
    message: "Webhooks suchen",
    description: "Endpoint list search label and placeholder",
  },
  "webhooks.searchEndpointsHelp": {
    message: "Nach Name, Ziel oder Ereignis suchen",
    description: "Endpoint list search helper text",
  },
  "webhooks.searchEndpointsEmpty": {
    message: "Keine Webhooks passen zu deiner Suche.",
    description: "Endpoint list empty state after local search",
  },
  "webhooks.emptyTitle": {
    message: "Keine Webhooks",
    description: "Webhooks empty-state title",
  },
  "webhooks.emptyBody": {
    message:
      "Füge einen Endpunkt hinzu, um benachrichtigt zu werden, wenn deine Services deployt, neu gestartet oder skaliert werden.",
    description: "Webhooks empty-state body",
  },
  "webhooks.forbiddenTitle": {
    message: "Nicht autorisiert",
    description: "Webhooks state when the caller lacks permission (403)",
  },
  "webhooks.forbiddenBody": {
    message:
      "Du hast keine Berechtigung, die Webhooks dieses Workspace anzuzeigen.",
    description: "Webhooks forbidden-state body",
  },
  "webhooks.errorTitle": {
    message: "Webhooks konnten nicht geladen werden",
    description: "Webhooks generic error title",
  },
  "webhooks.errorBody": {
    message: "Etwas ist schiefgelaufen. Bitte versuche es erneut.",
    description: "Webhooks generic error body",
  },
  "webhooks.proOnly": {
    message:
      "Webhooks erfordern den Tarif Pro oder höher. Bestehende Endpunkte werden weiterhin beliefert.",
    description: "Webhooks card description on a Hobby workspace (w8/m138)",
  },
  "webhooks.create": {
    message: "Webhook hinzufügen",
    description: "Button that opens the create dialog",
  },
  "webhooks.createTitle": {
    message: "Webhook hinzufügen",
    description: "Create dialog title",
  },
  "webhooks.createDescription": {
    message:
      "bex sendet per POST eine signierte JSON-Payload an diese URL, sobald ein abonniertes Ereignis eintritt.",
    description: "Create dialog description",
  },
  "webhooks.createEnabledHelp": {
    message:
      "Sofort mit der Zustellung beginnen. Du kannst ihn später in den Einstellungen aktivieren.",
    description: "Create page initial enabled-state helper copy",
  },
  "webhooks.fieldName": {
    message: "Name",
    description: "Create dialog name field label",
  },
  "webhooks.fieldNamePlaceholder": {
    message: "z. B. deploy-alerts-slack-bot",
    description: "Create dialog name field placeholder",
  },
  "webhooks.fieldUrl": {
    message: "Ziel-URL",
    description: "Create dialog URL field label",
  },
  "webhooks.fieldEvents": {
    message: "Zu sendende Ereignisse",
    description: "Create dialog event-type checklist label",
  },
  "webhooks.eventsLoading": {
    message: "Ereignistypen werden geladen…",
    description: "Create dialog while the event-type vocabulary loads",
  },
  "webhooks.eventsLoadError": {
    message:
      "Ereignistypen konnten nicht geladen werden. Versuche es erneut, bevor du speicherst.",
    description: "Actionable event-catalog query failure",
  },
  "webhooks.eventsRetry": {
    message: "Ereignistypen erneut laden",
    description: "Retry button for the event-catalog query",
  },
  "webhooks.eventsEmpty": {
    message: "Derzeit sind keine abonnierbaren Ereignistypen verfügbar.",
    description: "Event picker state when the served catalog is empty",
  },
  "webhooks.formUnavailableEvents": {
    message: "Erstellen ist erst möglich, wenn die Ereignistypen geladen sind.",
    description:
      "Create submit disabled reason while the catalog is unavailable",
  },
  "webhooks.formErrorsSummary": {
    message: "Korrigiere die markierten Felder und versuche es erneut.",
    description: "Accessible webhook form validation summary",
  },
  "webhooks.validation.nameRequired": {
    message: "Gib einen Webhook-Namen ein.",
    description: "Field-local required-name validation",
  },
  "webhooks.validation.nameDuplicate": {
    message: "Ein Webhook mit diesem Namen existiert bereits.",
    description: "Field-local duplicate-name validation",
  },
  "webhooks.validation.urlRequired": {
    message: "Gib eine Ziel-URL ein.",
    description: "Field-local required URL validation",
  },
  "webhooks.validation.urlHTTPS": {
    message: "Gib eine absolute HTTPS-URL ohne eingebettete Zugangsdaten ein.",
    description: "Field-local webhook URL validation",
  },
  "webhooks.validation.eventsRequired": {
    message: "Wähle mindestens ein Ereignis aus.",
    description: "Field-local event selection validation",
  },
  "webhooks.validation.eventsInvalid": {
    message:
      "Die Ereignisauswahl hat sich geändert. Lade den Katalog neu und wähle erneut.",
    description: "Server-coded stale/invalid event filter",
  },
  "webhooks.errors.planLimit": {
    message:
      "Webhooks sind im Tarif Pro und höher verfügbar. Dieser Workspace nutzt Hobby und kann daher keinen Endpunkt hinzufügen.",
    description:
      "Server-coded plan refusal of a webhook create on a Hobby workspace (WEBHOOK_PLAN_LIMIT, w4/425)",
  },
  "webhooks.planGate": {
    message:
      "Webhooks sind im Tarif Pro und höher verfügbar. Upgrade diesen Workspace, um einen Endpunkt hinzuzufügen.",
    description:
      "Notice in place of the create form and the empty list on a Hobby workspace (w4/425)",
  },
  "webhooks.errors.endpointLimit": {
    message: "Dieser Workspace hat sein Limit an Webhook-Endpunkten erreicht.",
    description: "Server-coded endpoint quota refusal",
  },
  "webhooks.errors.endpointDisabled": {
    message:
      "Aktiviere diesen Endpunkt, bevor du eine Zustellung erneut sendest.",
    description: "Server-coded resend refusal for a disabled endpoint",
  },
  "webhooks.errors.deliveryPending": {
    message:
      "Ein Zustellversuch ist bereits in der Warteschlange. Warte, bis er abgeschlossen ist.",
    description: "Server-coded duplicate pending resend refusal",
  },
  "webhooks.errors.staleState": {
    message:
      "Dieser Webhook wurde geändert oder entfernt. Aktualisiere die Seite und versuche es erneut.",
    description: "Safe stale endpoint/attempt refusal",
  },
  "webhooks.manageRequired": {
    message:
      "Du kannst diesen Webhook einsehen, aber nur Workspace-Manager können ihn ändern.",
    description: "Read-only member notice on webhook management surfaces",
  },
  "webhooks.latestNever": {
    message: "Nie gesendet",
    description: "Endpoint with no completed delivery attempt",
  },
  "webhooks.latestDelivered": {
    message: "Erfolgreich",
    description: "Latest endpoint attempt succeeded",
  },
  "webhooks.latestFailed": {
    message: "Fehlgeschlagen",
    description: "Latest endpoint attempt failed terminally",
  },
  "webhooks.latestRetrying": {
    message: "Neuer Versuch läuft",
    description: "Latest endpoint attempt failed with a retry pending",
  },
  "webhooks.createCancel": {
    message: "Abbrechen",
    description: "Create dialog cancel button",
  },
  "webhooks.createSubmit": {
    message: "Webhook erstellen",
    description: "Create page submit button (Render: 'Create Webhook')",
  },
  "webhooks.createSuccess": {
    message: "Webhook erstellt",
    description: "Toast after a successful create",
  },
  "webhooks.createError": {
    message: "Der Webhook konnte nicht erstellt werden",
    description: "Toast after a failed create",
  },
  "webhooks.copy": {
    message: "Signing Secret kopieren",
    description: "Copy-secret button label",
  },
  "webhooks.copied": {
    message: "Secret in die Zwischenablage kopiert",
    description: "Toast after copying the secret",
  },
  "webhooks.copyError": {
    message:
      "Kopieren fehlgeschlagen — markiere den Text und kopiere ihn manuell",
    description: "Toast when clipboard write fails",
  },
  "webhooks.toggle": {
    message: "Aktiviert",
    description: "Accessible label of the per-endpoint enable/disable switch",
  },
  "webhooks.enableSuccess": {
    message: "{name} aktiviert",
    description: "Toast after enabling an endpoint",
  },
  "webhooks.disableSuccess": {
    message: "{name} deaktiviert",
    description: "Toast after disabling an endpoint",
  },
  "webhooks.toggleError": {
    message: "{name} konnte nicht aktualisiert werden",
    description: "Toast after a failed enable/disable",
  },
  "webhooks.delete": {
    message: "Löschen",
    description: "Delete action label (icon button + confirm button)",
  },
  "webhooks.deleteConfirmTitle": {
    message: "{name} löschen?",
    description: "Delete confirmation dialog title",
  },
  "webhooks.deleteConfirmBody": {
    message:
      "An diesen Endpunkt werden keine weiteren Ereignisse gesendet und sein Zustellverlauf wird entfernt.",
    description: "Delete confirmation dialog body",
  },
  "webhooks.deleteCancel": {
    message: "Abbrechen",
    description: "Delete confirmation cancel button",
  },
  "webhooks.deleteSuccess": {
    message: "{name} gelöscht",
    description: "Toast after a successful delete",
  },
  "webhooks.deleteError": {
    message: "{name} konnte nicht gelöscht werden",
    description: "Toast after a failed delete",
  },
  "webhooks.history": {
    message: "Zustellverlauf",
    description: "Accessible label of the per-endpoint history button",
  },
  "webhooks.historyTitle": {
    message: "Zustellungen — {name}",
    description: "Delivery-history dialog title",
  },
  "webhooks.historyBody": {
    message:
      "Jedes an diesen Endpunkt gesendete Ereignis, neueste zuerst. Fehlgeschlagene Zustellungen werden mit exponentiellem Backoff wiederholt; bei wiederholtem Fehlschlagen wird der Endpunkt deaktiviert.",
    description: "Delivery-history dialog description",
  },
  "webhooks.historyEmptyTitle": {
    message: "Noch keine Zustellungen",
    description: "Delivery-history empty-state title",
  },
  "webhooks.historyEmptyBody": {
    message:
      "Löse ein abonniertes Ereignis aus — zum Beispiel ein Deployment —, um es hier zu sehen.",
    description: "Delivery-history empty-state body",
  },
  "webhooks.historyFilteredEmptySuccessfulTitle": {
    message: "Keine erfolgreichen Zustellungen",
    description:
      "Delivery-history state when the Successful tab (and any date range) matches nothing (w4/146)",
  },
  "webhooks.historyFilteredEmptyFailedTitle": {
    message: "Keine fehlgeschlagenen Zustellungen",
    description:
      "Delivery-history state when the Failed tab (and any date range) matches nothing (w4/146)",
  },
  "webhooks.historyFilteredEmptyRangeTitle": {
    message: "Keine Zustellungen in diesem Zeitraum",
    description:
      "Delivery-history state when the Sent after/before range matches nothing (w4/146)",
  },
  "webhooks.historyFilteredEmptyBody": {
    message:
      "Andere Zustellungen passen möglicherweise zu einem anderen Status oder Zeitraum.",
    description: "Body under a filtered-empty delivery history (w4/146)",
  },
  "webhooks.historyClearFilters": {
    message: "Filter zurücksetzen",
    description:
      "Resets the delivery-history status tab and date range (w4/146)",
  },
  "webhooks.historyErrorTitle": {
    message: "Zustellungen konnten nicht geladen werden",
    description: "Delivery-history error-state title",
  },
  "webhooks.historyErrorBody": {
    message: "Etwas ist schiefgelaufen. Bitte versuche es erneut.",
    description: "Delivery-history error-state body",
  },
  "webhooks.colEvent": {
    message: "Ereignis",
    description: "Delivery-history table column header",
  },
  "webhooks.colService": {
    message: "Service",
    description: "Delivery-history table column header",
  },
  "webhooks.serviceDeleted": {
    message: "(gelöscht)",
    description:
      "Marker beside a delivery's recorded service name when that service no longer exists, so the name cannot be linked",
  },
  "webhooks.colStatus": {
    message: "Status",
    description: "Delivery-history table column header",
  },
  "webhooks.colAttempts": {
    message: "Versuch",
    description:
      "Delivery-history table column header — one-based attempt number",
  },
  "webhooks.colResponse": {
    message: "Antwort",
    description: "Delivery-history table column header — last HTTP status",
  },
  "webhooks.colWhen": {
    message: "Zeitpunkt",
    description:
      "Delivery-history table column header — relative and exact send time",
  },
  "webhooks.colActions": {
    message: "Aktionen",
    description: "Accessible delivery-history table actions column header",
  },
  "webhooks.status.pending": {
    message: "Ausstehend",
    description: "Delivery status badge — queued or between retries",
  },
  "webhooks.status.delivered": {
    message: "Zugestellt",
    description: "Delivery status badge — endpoint answered 2xx",
  },
  "webhooks.status.failed": {
    message: "Fehlgeschlagen",
    description: "Delivery status badge — this individual attempt failed",
  },
  "webhooks.httpStatus": {
    message: "HTTP {status}",
    description: "Delivery attempt HTTP status summary",
  },
  "webhooks.openSourceEvent": {
    message: "{label} — Quell-Ereignis {id} öffnen",
    description:
      "Accessible label for the event hydration link; it starts with the link's visible text, the event type (WCAG 2.5.3)",
  },
  "webhooks.requestPayload": {
    message: "Request-Payload",
    description: "Expanded delivery-attempt request evidence heading",
  },
  "webhooks.endpointResponse": {
    message: "Antwort des Endpunkts",
    description: "Expanded delivery-attempt response evidence heading",
  },
  "webhooks.noRequestEvidence": {
    message: "Keine Request-Daten verfügbar.",
    description: "Expanded attempt with no retained request body",
  },
  "webhooks.noResponseEvidence": {
    message: "Es wurde keine Antwort aufgezeichnet.",
    description: "Expanded pending/transport attempt with no endpoint body",
  },
  "webhooks.attemptIdentity": {
    message: "Versuch {attemptId} · Quellereignis {eventId}",
    description: "Immutable attempt and stable source-event identity",
  },
  "webhooks.parentDeliveryStatus": {
    message: "Benachrichtigungsstatus: {status}",
    description:
      "Logical notification state shown separately from attempt outcome",
  },
  "webhooks.retryScheduled": {
    message: "Nächster automatischer Versuch: {date}",
    description: "Parent notification retry schedule",
  },
  "webhooks.resend": {
    message: "Erneut senden",
    description: "Manual resend action for a failed attempt",
  },
  "webhooks.resending": {
    message: "Wird erneut gesendet…",
    description: "Manual resend action in progress",
  },
  "webhooks.resendConfirmTitle": {
    message: "Diesen Webhook erneut senden?",
    description: "Manual resend confirmation title",
  },
  "webhooks.resendConfirmBody": {
    message:
      "Dadurch wird ein neuer signierter Versuch mit demselben Quellereignis und derselben Request-Payload eingereiht.",
    description: "Manual resend confirmation consequence",
  },
  "webhooks.resendCancel": {
    message: "Abbrechen",
    description: "Manual resend confirmation cancel action",
  },
  "webhooks.resendSuccess": {
    message: "Erneutes Senden des Webhooks eingereiht",
    description: "Toast after a manual resend reservation succeeds",
  },
  "webhooks.resendError": {
    message: "Dieser Webhook konnte nicht erneut gesendet werden",
    description: "Toast after a manual resend is refused or fails",
  },
  "webhooks.loadMore": {
    message: "Mehr laden",
    description: "Delivery-history pagination button",
  },
  "webhooks.selectedCount_one": {
    message: "{count} Ereignis ausgewählt",
    description: "Event-picker live selection counter (singular)",
  },
  "webhooks.selectedCount_other": {
    message: "{count} Ereignisse ausgewählt",
    description: "Event-picker live selection counter (plural)",
  },
  "webhooks.searchEvents": {
    message: "Nach Ereignissen suchen",
    description: "Event-picker search box placeholder + accessible label",
  },
  "webhooks.searchNoMatches": {
    message: "Keine Ereignisse passen zu deiner Suche",
    description:
      "Event-picker empty state while a search filters everything out",
  },
  "webhooks.allEvents": {
    message: "Alle Ereignisse",
    description: "Event-picker tri-state master checkbox label",
  },
  "webhooks.groupToggle": {
    message: "{group}-Ereignisse ein- oder ausklappen",
    description: "Accessible label of a group's expand/collapse chevron",
  },
  "webhooks.group.deploy": {
    message: "Deployment",
    description: "Event-picker group — deploy lifecycle events",
  },
  "webhooks.group.autoDeploy": {
    message: "Auto-Deploy",
    description: "Event-picker group — automatic deploy setting events",
  },
  "webhooks.group.serviceAvailability": {
    message: "Service-Verfügbarkeit",
    description: "Event-picker group — observed service availability events",
  },
  "webhooks.group.scaling": {
    message: "Skalierung",
    description: "Event-picker group — service scaling events",
  },
  "webhooks.group.cronJobRun": {
    message: "Cron-Job-Ausführung",
    description: "Event-picker group — cron job run events",
  },
  "webhooks.group.maintenanceMode": {
    message: "Wartungsmodus",
    description: "Event-picker group — maintenance mode events",
  },
  "webhooks.group.disk": {
    message: "Disk",
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
    message: "Pausierung",
    description: "Event-picker group — suspend/resume events",
  },
  "webhooks.group.agentSession": {
    message: "Cloud-Agents",
    description: "Event-picker group — cloud coding agent session events",
  },
  "webhooks.group.other": {
    message: "Sonstige",
    description:
      "Event-picker fallback group for served keys the catalog doesn't know yet",
  },
  "webhooks.event.deploy_started": {
    message: "Deployment gestartet",
    description: "Event label — deploy_started",
  },
  "webhooks.event.branch_deleted": {
    message: "Branch gelöscht",
    description: "Webhook event label",
  },
  "webhooks.event.build_started": {
    message: "Build gestartet",
    description: "Webhook event label",
  },
  "webhooks.event.build_ended": {
    message: "Build beendet",
    description: "Webhook event label",
  },
  "webhooks.event.pre_deploy_started": {
    message: "Pre-Deploy gestartet",
    description: "Webhook event label",
  },
  "webhooks.event.pre_deploy_ended": {
    message: "Pre-Deploy beendet",
    description: "Webhook event label",
  },
  "webhooks.event.job_run_ended": {
    message: "Job-Ausführung beendet",
    description: "Webhook event label",
  },
  "webhooks.event.auto_deploy_enabled": {
    message: "Auto-Deploy aktiviert",
    description: "Webhook event label",
  },
  "webhooks.event.auto_deploy_disabled": {
    message: "Auto-Deploy deaktiviert",
    description: "Webhook event label",
  },
  "webhooks.event.deploy_ended": {
    message: "Deployment beendet",
    description: "Event label — deploy_ended",
  },
  "webhooks.event.zero_downtime_redeploy_started": {
    message: "Unterbrechungsfreies Redeployment gestartet",
    description:
      "Event label — zero_downtime_redeploy_started: a Bex-initiated zero-downtime deploy started for a service",
  },
  "webhooks.event.zero_downtime_redeploy_ended": {
    message: "Unterbrechungsfreies Redeployment beendet",
    description:
      "Event label — zero_downtime_redeploy_ended: a Bex-initiated zero-downtime deploy completed for a service",
  },
  "webhooks.event.image_pull_failed": {
    message: "Image-Pull fehlgeschlagen",
    description: "Event label — image_pull_failed",
  },
  "webhooks.event.artifact_fetch_failed": {
    message: "Artefaktabruf fehlgeschlagen",
    description:
      "Event label — artifact_fetch_failed: a build published by a build source could not be fetched",
  },
  "webhooks.event.artifact_source_changed": {
    message: "Build-Quelle geändert",
    description:
      "Event label — artifact_source_changed: a service's linked build source changed",
  },
  "webhooks.event.pipeline_minutes_exhausted": {
    message: "Pipeline-Minuten aufgebraucht",
    description: "Event label — pipeline_minutes_exhausted",
  },
  "webhooks.event.edge_cache_enabled": {
    message: "Edge-Cache aktiviert",
    description:
      "Event label — edge_cache_enabled (API-only on Render; shown on subscriptions and deliveries, not in the picker)",
  },
  "webhooks.event.edge_cache_disabled": {
    message: "Edge-Cache deaktiviert",
    description:
      "Event label — edge_cache_disabled (API-only on Render; shown on subscriptions and deliveries, not in the picker)",
  },
  "webhooks.event.edge_cache_purged": {
    message: "Edge-Cache geleert",
    description:
      "Event label — edge_cache_purged (API-only on Render; shown on subscriptions and deliveries, not in the picker)",
  },
  "webhooks.event.commit_ignored": {
    message: "Commit ignoriert",
    description: "Event label — commit_ignored",
  },
  "webhooks.event.server_failed": {
    message: "Server fehlgeschlagen",
    description: "Event label — server_failed",
  },
  "webhooks.event.server_hardware_failure": {
    message: "Server-Hardwareausfall",
    description: "Event label — server_hardware_failure",
  },
  "webhooks.event.server_available": {
    message: "Server verfügbar",
    description: "Event label — server_available",
  },
  "webhooks.event.autoscaling_started": {
    message: "Autoscaling gestartet",
    description: "Event label — autoscaling_started",
  },
  "webhooks.event.autoscaling_ended": {
    message: "Autoscaling beendet",
    description: "Event label — autoscaling_ended",
  },
  "webhooks.event.branch_changed": {
    message: "Branch geändert",
    description: "Event label — branch_changed bex extension",
  },
  "webhooks.event.cron_job_run_started": {
    message: "Cron-Job-Ausführung gestartet",
    description: "Event label — cron_job_run_started",
  },
  "webhooks.event.cron_job_run_ended": {
    message: "Cron-Job-Ausführung beendet",
    description: "Event label — cron_job_run_ended",
  },
  "webhooks.event.maintenance_mode_enabled": {
    message: "Wartungsmodus aktiviert",
    description: "Event label — maintenance_mode_enabled",
  },
  "webhooks.event.maintenance_mode_uri_updated": {
    message: "Wartungsmodus-URI aktualisiert",
    description: "Event label — maintenance_mode_uri_updated",
  },
  "webhooks.event.disk_created": {
    message: "Disk erstellt",
    description: "Event label — disk_created",
  },
  "webhooks.event.disk_updated": {
    message: "Disk aktualisiert",
    description: "Event label — disk_updated",
  },
  "webhooks.event.disk_deleted": {
    message: "Disk gelöscht",
    description: "Event label — disk_deleted",
  },
  "webhooks.event.disk_snapshot_restored": {
    message: "Disk-Snapshot wiederhergestellt",
    description: "Event label — disk_snapshot_restored",
  },
  "webhooks.event.postgres_created": {
    message: "Postgres erstellt",
    description: "Event label — postgres_created",
  },
  "webhooks.event.postgres_restarted": {
    message: "Postgres neu gestartet",
    description: "Event label — postgres_restarted",
  },
  "webhooks.event.postgres_credentials_created": {
    message: "Postgres-Zugangsdaten erstellt",
    description: "Event label — postgres_credentials_created",
  },
  "webhooks.event.postgres_credentials_deleted": {
    message: "Postgres-Zugangsdaten gelöscht",
    description: "Event label — postgres_credentials_deleted",
  },
  "webhooks.event.postgres_backup_started": {
    message: "Postgres-Backup gestartet",
    description: "Event label — postgres_backup_started",
  },
  "webhooks.event.postgres_available": {
    message: "Postgres verfügbar",
    description: "Event label — postgres_available",
  },
  "webhooks.event.postgres_unavailable": {
    message: "Postgres nicht verfügbar",
    description: "Event label — postgres_unavailable",
  },
  "webhooks.event.postgres_ha_status_changed": {
    message: "Postgres-Hochverfügbarkeit geändert",
    description: "Event label — postgres_ha_status_changed",
  },
  "webhooks.event.postgres_cluster_leader_changed": {
    message: "Postgres-Cluster-Leader geändert",
    description: "Event label — postgres_cluster_leader_changed",
  },
  "webhooks.event.postgres_connection_pool_enabled_changed": {
    message: "Postgres-Connection-Pool geändert",
    description: "Event label — postgres_connection_pool_enabled_changed",
  },
  "webhooks.event.postgres_disk_size_changed": {
    message: "Postgres-Disk-Größe geändert",
    description: "Event label — postgres_disk_size_changed",
  },
  "webhooks.event.postgres_disk_autoscaling_enabled_changed": {
    message: "Postgres-Disk-Autoscaling geändert",
    description: "Event label — postgres_disk_autoscaling_enabled_changed",
  },
  "webhooks.event.postgres_read_replicas_changed": {
    message: "Postgres-Lesereplikate geändert",
    description: "Event label — postgres_read_replicas_changed",
  },
  "webhooks.event.postgres_backup_completed": {
    message: "Postgres-Backup abgeschlossen",
    description: "Event label — postgres_backup_completed",
  },
  "webhooks.event.postgres_backup_failed": {
    message: "Postgres-Backup fehlgeschlagen",
    description: "Event label — postgres_backup_failed",
  },
  "webhooks.event.postgres_pitr_checkpoint_started": {
    message: "Postgres-PITR-Checkpoint gestartet",
    description: "Event label — postgres_pitr_checkpoint_started",
  },
  "webhooks.event.postgres_pitr_checkpoint_completed": {
    message: "Postgres-PITR-Checkpoint abgeschlossen",
    description: "Event label — postgres_pitr_checkpoint_completed",
  },
  "webhooks.event.postgres_pitr_checkpoint_failed": {
    message: "Postgres-PITR-Checkpoint fehlgeschlagen",
    description: "Event label — postgres_pitr_checkpoint_failed",
  },
  "webhooks.event.postgres_wal_archive_failed": {
    message: "Postgres-WAL-Archivierung fehlgeschlagen",
    description: "Event label — postgres_wal_archive_failed",
  },
  "webhooks.event.postgres_restore_succeeded": {
    message: "Postgres-Wiederherstellung erfolgreich",
    description: "Event label — postgres_restore_succeeded",
  },
  "webhooks.event.postgres_restore_failed": {
    message: "Postgres-Wiederherstellung fehlgeschlagen",
    description: "Event label — postgres_restore_failed",
  },
  "webhooks.event.postgres_upgrade_started": {
    message: "Postgres-Upgrade gestartet",
    description: "Event label — postgres_upgrade_started",
  },
  "webhooks.event.postgres_upgrade_succeeded": {
    message: "Postgres-Upgrade erfolgreich",
    description: "Event label — postgres_upgrade_succeeded",
  },
  "webhooks.event.postgres_upgrade_failed": {
    message: "Postgres-Upgrade fehlgeschlagen",
    description: "Event label — postgres_upgrade_failed",
  },
  "webhooks.event.key_value_available": {
    message: "Key Value verfügbar",
    description: "Event label — key_value_available",
  },
  "webhooks.event.key_value_unhealthy": {
    message: "Key Value fehlerhaft",
    description: "Event label — key_value_unhealthy",
  },
  "webhooks.event.key_value_config_restart": {
    message: "Key Value wegen Konfigurationsänderung neu gestartet",
    description: "Event label — key_value_config_restart",
  },
  "webhooks.event.service_suspended": {
    message: "Service pausiert",
    description: "Event label — service_suspended",
  },
  "webhooks.event.service_resumed": {
    message: "Service fortgesetzt",
    description: "Event label — service_resumed",
  },
  "webhooks.event.service_hibernated": {
    message: "Service ist eingeschlafen",
    description: "Event label — service_hibernated",
  },
  "webhooks.event.service_woken": {
    message: "Service ist aufgewacht",
    description: "Event label — service_woken",
  },
  "webhooks.event.agent_session_turn_succeeded": {
    message: "Agent-Sitzungsrunde erfolgreich",
    description: "Event label — agent_session_turn_succeeded",
  },
  "webhooks.event.agent_session_turn_failed": {
    message: "Agent-Sitzungsrunde fehlgeschlagen",
    description: "Event label — agent_session_turn_failed",
  },
  "webhooks.event.agent_session_needs_input": {
    message: "Agent-Sitzung benötigt Eingabe",
    description: "Event label — agent_session_needs_input",
  },
  "webhooks.event.service_moved": {
    message: "Service verschoben",
    description:
      "Event label — service_moved (bex extension: project/environment placement change)",
  },
  "webhooks.event.server_restarted": {
    message: "Server neu gestartet",
    description: "Event label — server_restarted",
  },
  "webhooks.event.instance_count_changed": {
    message: "Instanzanzahl geändert",
    description: "Event label — instance_count_changed",
  },
  "webhooks.event.autoscaling_config_changed": {
    message: "Autoscaling-Konfiguration geändert",
    description: "Event label — autoscaling_config_changed",
  },
  "webhooks.event.plan_changed": {
    message: "Tarif geändert",
    description:
      "Event label — plan_changed (bex's slot for Render's Instance Type Changed)",
  },
  "webhooks.newTitle": {
    message: "Neuen Webhook erstellen",
    description: "Create page heading (Render: 'Create a new Webhook')",
  },
  "webhooks.newMenuLabel": {
    message: "Neuer Webhook",
    description:
      "+ New menu entry; worded like the other entries, unlike the create page heading",
  },
  "webhooks.backToList": {
    message: "Zurück zu den Webhooks",
    description: "Accessible label of the create/detail pages' back link",
  },
  "webhooks.fieldNameHelp": {
    message: "Ein eindeutiger Name für diesen Webhook.",
    description: "Create page name-field helper copy",
  },
  "webhooks.fieldUrlHelp": {
    message: "bex sendet jede Benachrichtigung als POST-Request an diese URL.",
    description: "Create page URL-field helper copy",
  },
  "webhooks.fieldUrlPlaceholder": {
    message: "https://example.com/webhooks/bex",
    description: "Create page URL-field placeholder",
  },
  "webhooks.fieldEventsHelp": {
    message:
      "Wähle, welche Ereignisse in deinem Workspace eine Webhook-Benachrichtigung auslösen.",
    description: "Create page events-field helper copy",
  },
  "webhooks.detailKicker": {
    message: "Webhook",
    description: "Detail page header kicker above the endpoint name",
  },
  "webhooks.idLabel": {
    message: "Webhook-ID:",
    description: "Detail header id row label",
  },
  "webhooks.copyId": {
    message: "Webhook-ID kopieren",
    description: "Accessible label of the header id copy button",
  },
  "webhooks.copyUrl": {
    message: "Endpunkt-URL kopieren",
    description: "Accessible label of the header URL copy button",
  },
  "webhooks.copiedGeneric": {
    message: "In die Zwischenablage kopiert",
    description: "Toast after copying the id or URL",
  },
  "webhooks.showMore": {
    message: "{count} weitere anzeigen",
    description: "Event-chip expander on the detail header",
  },
  "webhooks.showLess": {
    message: "Weniger anzeigen",
    description: "Event-chip collapser on the detail header",
  },
  "webhooks.createdByOn": {
    message: "Erstellt von {creator} am {date}",
    description: "Detail header provenance line when the creator is known",
  },
  "webhooks.createdOn": {
    message: "Erstellt am {date}",
    description: "Detail header provenance line without a creator",
  },
  "webhooks.tabActivity": {
    message: "Aktivität",
    description: "Detail page tab — delivery history",
  },
  "webhooks.tabSettings": {
    message: "Einstellungen",
    description: "Detail page tab — edit + delete",
  },
  "webhooks.recentDeliveries": {
    message: "Letzte Zustellungen",
    description: "Activity tab heading",
  },
  "webhooks.recentDeliveriesHint": {
    message:
      "Jede Zeile ist ein Zustellversuch. Neue Versuche erscheinen automatisch, solange dieser Tab sichtbar ist.",
    description: "Activity tab heading hint for immutable attempts + polling",
  },
  "webhooks.refresh": {
    message: "Aktualisieren",
    description: "Accessible label of the Activity refresh button",
  },
  "webhooks.filterAll": {
    message: "Alle",
    description: "Activity delivery filter tab",
  },
  "webhooks.filterSuccessful": {
    message: "Erfolgreich",
    description: "Activity delivery filter tab — delivered only",
  },
  "webhooks.filterFailed": {
    message: "Fehlgeschlagen",
    description: "Activity delivery filter tab — failed only",
  },
  "webhooks.sentAfter": {
    message: "Gesendet nach",
    description: "Activity delivery-history lower timestamp bound",
  },
  "webhooks.sentBefore": {
    message: "Gesendet vor",
    description: "Activity delivery-history upper timestamp bound",
  },
  "webhooks.destinationBlocked": {
    message: "Ziel nicht erlaubt",
    description:
      "Delivery result label when bex refused to dial the destination: it resolves to a loopback, private, link-local or cluster-internal address (w21/046)",
  },
  "webhooks.destinationUnreachableNotice": {
    message:
      "Diese Adresse ist aus dem Internet nicht erreichbar, daher schlägt jede Zustellung dorthin fehl. Verwende eine öffentliche HTTPS-Adresse.",
    description:
      "Non-blocking notice under the webhook Destination URL when its host is a loopback, private, link-local or cluster-internal address (w21/046)",
  },
  "webhooks.transportError": {
    message: "Transportfehler",
    description: "Delivery evidence label when no HTTP response was received",
  },
  "webhooks.enabledBadge": {
    message: "Aktiviert",
    description: "Detail header status badge while enabled",
  },
  "webhooks.disabledBadge": {
    message: "Deaktiviert",
    description: "Detail header status badge while disabled",
  },
  "webhooks.view": {
    message: "Webhook anzeigen",
    description: "Accessible label of the list row's open-detail link",
  },
  "webhooks.settingsGeneral": {
    message: "Allgemein",
    description: "Settings tab first section heading",
  },
  "webhooks.disabledReasonAuto": {
    message: "Nach wiederholten Zustellfehlern automatisch deaktiviert.",
    description:
      "Reason shown beside a webhook the platform disabled after repeated delivery failures",
  },
  "webhooks.settingsStatus": {
    message: "Status",
    description: "Settings status-toggle label",
  },
  "webhooks.settingsStatusHelp": {
    message: "Deaktivierte Webhooks senden keine Benachrichtigungen.",
    description: "Settings status-toggle helper copy",
  },
  "webhooks.settingsEvents": {
    message: "Abonnierte Ereignisse",
    description: "Settings events section heading",
  },
  "webhooks.saveChanges": {
    message: "Änderungen speichern",
    description: "Settings submit button (disabled until dirty)",
  },
  "webhooks.updateSuccess": {
    message: "Webhook aktualisiert",
    description: "Toast after a successful settings save",
  },
  "webhooks.updateError": {
    message: "Der Webhook konnte nicht aktualisiert werden",
    description: "Toast after a failed settings save",
  },
  "webhooks.secretLabel": {
    message: "Signing Secret",
    description: "Settings signing-secret row label",
  },
  "webhooks.secretHelp": {
    message:
      "Ein Token zum Signieren und Verifizieren von Request-Payloads. Nur Workspace-Admins können es sehen.",
    description: "Settings signing-secret row help (Render's wording)",
  },
  "webhooks.secretShow": {
    message: "Secret anzeigen",
    description: "Settings button that reveals the masked signing secret",
  },
  "webhooks.secretHide": {
    message: "Secret ausblenden",
    description: "Settings button that masks the revealed signing secret",
  },
  "webhooks.deleteSection": {
    message: "Webhook löschen",
    description: "Settings danger-zone heading",
  },
  "webhooks.deleteTypeToConfirm": {
    message: "Gib zur Bestätigung unten {command} ein.",
    description: "Type-to-confirm instruction in the delete dialog",
  },
  "webhooks.deleteCommandLabel": {
    message: "Bestätigung",
    description: "Accessible label of the type-to-confirm input",
  },
};

export default deWebhooks;
