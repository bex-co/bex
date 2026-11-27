import type { TranslationEntry } from "@/i18n";

const deDatabases: Record<string, TranslationEntry> = {
  "databases.resourceType": {
    message: "Datenbank",
    description: "Managed Postgres resource type used in document titles",
  },
  "databases.statTotal": {
    message: "Datenbanken gesamt",
    description: "Databases page stat card label",
  },
  "databases.statAvailable": {
    message: "Verfügbar",
    description: "Databases page stat card label (healthy databases)",
  },
  "databases.statCreating": {
    message: "Wird erstellt",
    description: "Databases page stat card label (provisioning databases)",
  },
  "databases.cardTitle": {
    message: "Datenbanken",
    description: "Databases table card title",
  },
  "databases.colName": {
    message: "Name",
    description: "Databases table column header",
  },
  "databases.colStatus": {
    message: "Status",
    description: "Databases table column header",
  },
  "databases.colPlan": {
    message: "Tarif",
    description: "Databases table column header (compute plan / tier)",
  },
  "databases.colVersion": {
    message: "Version",
    description: "Databases table column header (PostgreSQL major version)",
  },
  "databases.colStorage": {
    message: "Speicher",
    description: "Databases table column header (disk size)",
  },
  "databases.colCreated": {
    message: "Erstellt",
    description: "Databases table column header (relative age from createdAt)",
  },
  "databases.colActions": {
    message: "Aktionen",
    description: "Databases table actions column header (screen-reader only)",
  },
  "databases.statusAvailable": {
    message: "Verfügbar",
    description: "Database status badge (CNPG cluster healthy)",
  },
  "databases.statusCreating": {
    message: "Wird erstellt",
    description: "Database status badge (provisioning)",
  },
  "databases.statusRestarting": {
    message: "Wird neu gestartet",
    description:
      "Database status badge (Render's config_restart: an established store restarting after a config change, restart, or resume — not a first provision)",
  },
  "databases.statusUpgrading": {
    message: "Wird aktualisiert",
    description: "Database status badge (offline PostgreSQL major upgrade)",
  },
  "databases.statusUnavailable": {
    message: "Nicht verfügbar",
    description: "Database status badge (provisioning failed)",
  },
  "databases.statusSuspended": {
    message: "Pausiert",
    description:
      "Database status badge (hibernated; suspension wins over the status enum)",
  },
  "databases.statusUnknown": {
    message: "Unbekannt",
    description: "Database status badge for an unrecognized status",
  },
  "databases.errorTitle": {
    message: "Datenbanken konnten nicht geladen werden",
    description: "Databases list error card title",
  },
  "databases.errorBody": {
    message:
      "Die Anfrage an bex-api ist fehlgeschlagen. Prüfe deine Verbindung und versuche es erneut.",
    description: "Databases list error card body",
  },
  "databases.emptyTitle": {
    message: "Noch keine Datenbanken",
    description: "Databases list empty state title",
  },
  "databases.emptyBody": {
    message:
      "Erstelle dein erstes verwaltetes Postgres, dann erscheint es hier.",
    description: "Databases list empty state body",
  },
  "databases.actionsMenu": {
    message: "Aktionen für {name}",
    description: "Accessible label for the per-row actions trigger",
  },
  "databases.actionDelete": {
    message: "Löschen",
    description: "Row action: permanently delete the database",
  },
  "databases.deleteConfirmTitle": {
    message: "{name} löschen?",
    description: "Delete-confirmation dialog title",
  },
  "databases.deleteConfirmBody": {
    message:
      "Diese Aktion kann nicht rückgängig gemacht werden. Möchtest du diese Postgres-Instanz wirklich löschen?",
    description: "Delete-confirmation dialog body (Render-style copy)",
  },
  "databases.deleteConfirmPrompt": {
    message: "Gib unten {phrase} ein, um zu bestätigen.",
    description:
      "Body prompt naming the exact 'sudo delete postgres <name>' phrase (rendered bold by SudoCommandField)",
  },
  "databases.deleteCancel": {
    message: "Abbrechen",
    description: "Delete-confirmation dialog cancel button",
  },
  "databases.deleteConfirm": {
    message: "Datenbank löschen",
    description: "Delete-confirmation dialog confirm button",
  },
  "databases.deleteSuccess": {
    message: "{name} wird gelöscht…",
    description: "Toast after a delete request is accepted",
  },
  "databases.deleteError": {
    message: "{name} konnte nicht gelöscht werden. Bitte versuche es erneut.",
    description: "Toast when a delete request fails",
  },
  "databases.createTitle": {
    message: "Neues Postgres",
    description: "Create-database dialog title",
  },
  "databases.createDescription": {
    message: "Stelle eine verwaltete PostgreSQL-Instanz bereit.",
    description: "Create-database dialog description",
  },
  "databases.fieldName": {
    message: "Name",
    description: "Create-database form field label (database name)",
  },
  "databases.fieldNamePlaceholder": {
    message: "my-database",
    description: "Create-database name input placeholder",
  },
  "databases.fieldNameError": {
    message:
      "Gib einen Namen mit höchstens 100 Zeichen ein, ohne Zeilenumbrüche oder Steuerzeichen.",
    description: "Create-database name validation message",
  },
  "databases.fieldDatabaseName": {
    message: "Datenbank",
    description: "Create-database physical PostgreSQL database label",
  },
  "databases.fieldDatabaseNamePlaceholder": {
    message: "orders_data",
    description: "Create-database physical PostgreSQL database placeholder",
  },
  "databases.fieldDatabaseUser": {
    message: "Benutzer",
    description: "Create-database physical PostgreSQL owner-role label",
  },
  "databases.fieldDatabaseUserPlaceholder": {
    message: "orders_owner",
    description: "Create-database physical PostgreSQL owner-role placeholder",
  },
  "databases.fieldPhysicalNameReserved": {
    message:
      "„{name}“ ist von PostgreSQL reserviert. Wähle einen anderen Namen.",
    description:
      "Create-database / add-user validation for a name the managed PostgreSQL service reserves (postgres, template0/1, streaming_replica, public, none, pg_*, cnpg_*)",
  },
  "databases.fieldPhysicalNameError": {
    message:
      "Verwende bis zu 63 Kleinbuchstaben, Ziffern und Unterstriche; beginne mit einem Buchstaben oder Unterstrich.",
    description: "Create-database physical PostgreSQL identifier validation",
  },
  "databases.fieldPlan": {
    message: "Compute-Plan",
    description: "Create-database form field label (plan / tier)",
  },
  "databases.fieldVersion": {
    message: "PostgreSQL-Version",
    description: "Create-database form field label (major version)",
  },
  "databases.fieldVersionDefault": {
    message: "Standard (neueste)",
    description: "Create-database version select default option",
  },
  "databases.fieldDisk": {
    message: "Speicher",
    description: "Create-database form field label (storage size)",
  },
  "databases.fieldDiskFixed": {
    message:
      "Dieser Plan enthält eine feste Festplatte mit {size} GB. Wählen Sie einen größeren Plan, um den Speicher selbst festzulegen.",
    description:
      "Create-database hint under a plan's fixed disk size, e.g. Free (w4/376)",
  },
  "databases.fieldDiskError": {
    message: "Gib eine ganze Zahl zwischen {min} und {max} GB ein.",
    description:
      "Create-database validation message when the disk size is below the plan floor or above the volume cap",
  },
  "databases.fieldPublic": {
    message: "Öffentlicher Zugriff",
    description: "Create-database form field label (external endpoint toggle)",
  },
  "databases.fieldPublicHint": {
    message:
      "Akzeptiert Verbindungen mit gültigen Zugangsdaten von jeder IP-Adresse, bis du sie unter Zugriffskontrolle einschränkst.",
    description:
      "Create-database public toggle helper text; the toggle starts on, as a Render database is reachable by default (w4/553)",
  },
  "databases.fieldPublicHintOff": {
    message:
      "Externe Verbindungen sind blockiert. Services im privaten Netzwerk können weiterhin eine Verbindung herstellen.",
    description:
      "Create-database public toggle helper text once switched off: the database is created private (w4/553)",
  },
  "databases.fieldHighAvailability": {
    message: "Hochverfügbarkeit",
    description: "Create-database high availability toggle label (w2/m171)",
  },
  "databases.fieldHighAvailabilityHint": {
    message:
      "Betreibe einen Standby auf einem anderen Node, der übernimmt, wenn der Primary ausfällt. Der Standby wird wie der Primary abgerechnet.",
    description:
      "Create-database HA toggle helper text on a plan that offers HA",
  },
  "databases.fieldHighAvailabilityPlanUnsupported": {
    message:
      "{name} bietet keine Hochverfügbarkeit. Wähle einen Tarif mit mindestens 1 CPU.",
    description:
      "Create-database HA toggle helper text when the selected plan is below 1 CPU (Render's rule)",
  },
  "databases.createCancel": {
    message: "Abbrechen",
    description: "Create-database dialog cancel button",
  },
  "databases.createSubmit": {
    message: "Datenbank erstellen",
    description: "Create-database dialog submit button",
  },
  "databases.createSuccess": {
    message: "{name} wird erstellt…",
    description:
      "Toast after a create request is accepted (provisioning is async)",
  },
  "databases.createError": {
    message: "{name} konnte nicht erstellt werden. Bitte versuche es erneut.",
    description: "Toast when a create request fails",
  },
  "databases.capLimitTitle": {
    message: "Postgres-Limit erreicht",
    description:
      "Alert title when the workspace's Postgres creation cap is hit (w7/m9)",
  },
  "databases.capLimitUpgrade": {
    message: "Tarif upgraden",
    description:
      "Upgrade CTA button inside the Postgres cap-limit Alert (w7/m9)",
  },
  "databases.metaTitle": {
    message: "Details",
    description: "Database detail metadata card title",
  },
  "databases.statusReasonClusterFailed": {
    message:
      "Der Datenbank-Cluster hat seine Konfiguration abgelehnt, daher konnte die letzte Änderung nicht angewendet werden.",
    description:
      "Why an unavailable database is unavailable: the operator's ClusterFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonClusterReadFailed": {
    message: "Der Zustand des Datenbank-Clusters konnte nicht gelesen werden.",
    description:
      "Why an unavailable database is unavailable: the operator's ClusterReadFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonNetworkPolicyFailed": {
    message:
      "Die Netzwerkrichtlinie der Datenbank konnte nicht angewendet werden.",
    description:
      "Why an unavailable database is unavailable: the operator's NetworkPolicyFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonDiskAutoscalingFailed": {
    message: "Das Disk-Autoscaling konnte nicht angewendet werden.",
    description:
      "Why an unavailable database is unavailable: the operator's DiskAutoscalingFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonExportFailed": {
    message: "Ein Datenbankexport konnte nicht gestartet werden.",
    description:
      "Why an unavailable database is unavailable: the operator's ExportFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonPoolerFailed": {
    message: "Der Connection-Pooler konnte nicht bereitgestellt werden.",
    description:
      "Why an unavailable database is unavailable: the operator's PoolerFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonExtensionsFailed": {
    message:
      "Die verwalteten Erweiterungen der Datenbank konnten nicht installiert werden, daher ist sie noch nicht verfügbar.",
    description:
      "Why an unavailable database is unavailable: the operator's ExtensionsFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonPostUpgradeBackupFailed": {
    message:
      "Das Backup nach dem Versions-Upgrade konnte nicht erstellt werden.",
    description:
      "Why an unavailable database is unavailable: the operator's PostUpgradeBackupFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonMajorVersionUpgradeFailed": {
    message:
      "Das Major-Version-Upgrade ist fehlgeschlagen; die Datenbank bleibt auf ihrer aktuellen Version.",
    description:
      "Why an unavailable database is unavailable: the operator's MajorVersionUpgradeFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonStorageShrinkRejected": {
    message:
      "Postgres-Speicher kann nur wachsen: Die Disk darf nicht unter die zugewiesenen {size} GB sinken.",
    description:
      "Why an unavailable database is unavailable: the operator's StorageShrinkRejected Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonRecoveryUnavailable": {
    message:
      "Die Datenbank kann nicht wiederhergestellt werden: Es ist kein Backup-Speicher konfiguriert.",
    description:
      "Why an unavailable database is unavailable: the operator's RecoveryUnavailable Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonBackupStoreUnavailable": {
    message:
      "Der Backup-Speicher der Datenbank konnte nicht eingerichtet werden.",
    description:
      "Why an unavailable database is unavailable: the operator's BackupStoreUnavailable Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonScheduledBackupFailed": {
    message: "Das tägliche Backup konnte nicht geplant werden.",
    description:
      "Why an unavailable database is unavailable: the operator's ScheduledBackupFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonScheduledBackupCleanupFailed": {
    message:
      "Der Zeitplan für das tägliche Backup konnte nicht entfernt werden.",
    description:
      "Why an unavailable database is unavailable: the operator's ScheduledBackupCleanupFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.metaStatus": {
    message: "Status",
    description: "Database detail metadata row label",
  },
  "databases.metaPlan": {
    message: "Compute-Plan",
    description: "Database detail metadata row label",
  },
  "databases.metaVersion": {
    message: "Version",
    description: "Database detail metadata row label",
  },
  "databases.versionLabel": {
    message: "PostgreSQL {version}",
    description: "Formatted PostgreSQL major version label",
  },
  "databases.versionUpgradeDowntime": {
    message:
      "Die Datenbank und alle Replikate sind während des Upgrades vorübergehend nicht verfügbar.",
    description:
      "Version upgrade page: first sentence of the downtime notice under Upgrade this database",
  },
  "databases.versionUpgradeConfirm": {
    message: "Auf PostgreSQL {version} upgraden",
    description:
      "Version upgrade page: the button that starts the upgrade of this database",
  },
  "databases.versionUpgradeAccepted": {
    message: "Upgrade auf PostgreSQL {version} wird gestartet",
    description: "Toast after a PostgreSQL major-version upgrade is accepted",
  },
  "databases.versionUpgradeError": {
    message:
      "Das PostgreSQL-Upgrade konnte nicht gestartet werden. Bitte versuche es erneut.",
    description: "Fallback error when the PostgreSQL upgrade mutation fails",
  },
  "databases.versionUpgradeDowntimeDetail": {
    message:
      "Das dauert in der Regel weniger als eine Stunde. Die Datenbank und ihre Replikate sind bis zum Ende des Upgrades nicht verfügbar.",
    description: "Version upgrade page: second sentence of the downtime notice",
  },
  "databases.versionUpgradeHeading": {
    message: "Postgres-Versionsupgrades",
    description:
      "Version upgrade page title and document title (Render: Postgres Version Upgrades)",
  },
  "databases.versionUpgradeCurrent": {
    message: "Aktuelle Version: PostgreSQL {version}",
    description:
      "Version upgrade page: badge naming the database's current major version",
  },
  "databases.versionUpgradeLatest": {
    message:
      "Diese Datenbank läuft bereits auf der neuesten unterstützten PostgreSQL-Version.",
    description:
      "Version upgrade page: shown when the database already runs the latest major",
  },
  "databases.versionUpgradeStepsTitle": {
    message: "Schritte zum Upgrade",
    description: "Version upgrade page: heading over the upgrade steps",
  },
  "databases.versionUpgradeStepsDocs": {
    message: "Weitere Details zum Upgrade findest du in der {docs}.",
    description:
      "Version upgrade page: sentence after the steps heading; {docs} is the See the docs link",
  },
  "databases.versionUpgradeDocsLink": {
    message: "Dokumentation",
    description: "Version upgrade page: link to the upgrade documentation",
  },
  "databases.versionUpgradeChangesTitle": {
    message:
      "Ein Upgrade nimmt diese Änderungen an deiner Datenbank vor, die sich nicht rückgängig machen lassen:",
    description:
      "Version upgrade page: title of the list of changes an upgrade makes for good",
  },
  "databases.versionUpgradeChangePitr": {
    message:
      "Point-in-Time Recovery beginnt mit dem Upgrade neu: Die Datenbank lässt sich nicht auf einen Zeitpunkt davor wiederherstellen.",
    description:
      "Version upgrade page: irreversible change — point-in-time recovery restarts at the upgrade",
  },
  "databases.versionUpgradeChangeNoDowngrade": {
    message: "Die Datenbank kann nicht zu PostgreSQL {version} zurückkehren.",
    description:
      "Version upgrade page: irreversible change — no return to the current major",
  },
  "databases.versionUpgradeBlockedTitle": {
    message: "Erweiterungen blockieren das Upgrade",
    description:
      "Version upgrade page: title of the notice listing extensions that block the upgrade",
  },
  "databases.versionUpgradeBlockedBody": {
    message:
      "Eine Datenbank mit diesen aktivierten Erweiterungen kann nicht upgegradet werden. Entferne sie zuerst (DROP EXTENSION):",
    description:
      "Version upgrade page: explanation above the list of blocking extensions",
  },
  "databases.versionUpgradeStepsLabel": {
    message: "Upgrade-Schritte",
    description:
      "Version upgrade page: accessible name of the ordered list of upgrade steps",
  },
  "databases.versionUpgradeCloneStep": {
    message: "Einen Klon dieser Datenbank upgraden (empfohlen)",
    description:
      "Version upgrade page: title of the first step (Render: Upgrade a clone of this database (recommended))",
  },
  "databases.versionUpgradeCloneBody": {
    message:
      "Führe ein Test-Upgrade auf einer Wegwerfkopie durch, um sicherzugehen, dass es erfolgreich abschließt.",
    description: "Version upgrade page: why to clone first",
  },
  "databases.versionUpgradeCloneBilling": {
    message:
      "Klone werden genauso abgerechnet wie jede andere Datenbank mit denselben Spezifikationen. Lösche den Klon, wenn du mit dem Testen fertig bist.",
    description: "Version upgrade page: clone billing reminder",
  },
  "databases.versionUpgradeCloneAction": {
    message: "Diese Datenbank klonen",
    description:
      "Version upgrade page: button that creates the point-in-time clone",
  },
  "databases.versionUpgradeClonePitr": {
    message:
      "Du kannst die Datenbank klonen, sobald Point-in-Time Recovery bereit ist.",
    description:
      "Version upgrade page: tooltip while the clone button waits for point-in-time recovery",
  },
  "databases.versionUpgradeCloneCreated": {
    message: "Eine geklonte Datenbank wurde erstellt: {name}.",
    description:
      "Version upgrade page: toast after the clone is created; {name} is the clone",
  },
  "databases.versionUpgradeCloneView": {
    message: "Postgres-Klon ansehen",
    description: "Version upgrade page: link to the clone's own upgrade page",
  },
  "databases.versionUpgradeCloneError": {
    message: "Der Klon konnte nicht erstellt werden. Bitte versuche es erneut.",
    description:
      "Version upgrade page: fallback error when the clone cannot be created",
  },
  "databases.versionUpgradeThisStep": {
    message: "Diese Datenbank upgraden",
    description:
      "Version upgrade page: title of the step that upgrades this database",
  },
  "databases.versionUpgradeRunning": {
    message: "Upgrade läuft...",
    description:
      "Version upgrade page: upgrade button label while the upgrade runs",
  },
  "databases.versionUpgradeNotReady": {
    message:
      "Das Upgrade ist möglich, sobald die Datenbank verfügbar und Point-in-Time Recovery eingerichtet ist.",
    description:
      "Version upgrade page: tooltip while the upgrade button waits for the database and point-in-time recovery",
  },
  "databases.versionUpgradeReleaseNotes": {
    message: "Versionshinweise lesen",
    description:
      "Version upgrade page: link to PostgreSQL's release notes for the target major",
  },
  "databases.versionUpgradeStatusLabel": {
    message: "Upgrade-Status und Logs",
    description:
      "Version upgrade page: accessible name of a run's status and logs section",
  },
  "databases.versionUpgradeStateInProgress": {
    message: "läuft",
    description: "Version upgrade page: run state badge",
  },
  "databases.versionUpgradeStateSucceeded": {
    message: "erfolgreich",
    description: "Version upgrade page: run state badge",
  },
  "databases.versionUpgradeStateFailed": {
    message: "fehlgeschlagen",
    description: "Version upgrade page: run state badge",
  },
  "databases.versionUpgradeLogsEmpty": {
    message: "Logs, die zu deinen Suchparametern passen, erscheinen hier.",
    description:
      "Version upgrade page: empty logs explanation for a finished run",
  },
  "databases.versionUpgradeLogsEmptyRunning": {
    message:
      "Logs, die zu deinen Suchparametern passen, erscheinen hier. Nach dem Start des Upgrades kann es einige Minuten dauern, bis Logs erscheinen.",
    description:
      "Version upgrade page: empty logs explanation while the run is in progress",
  },
  "databases.versionUpgradeLastAttempt": {
    message: "Letzter Upgrade-Versuch",
    description:
      "Version upgrade page: title of the card holding the last finished run",
  },
  "databases.versionUpgradeSucceededTitle": {
    message: "Upgrade erfolgreich",
    description: "Toast title when an upgrade the page watched succeeds",
  },
  "databases.versionUpgradeSucceededBody": {
    message: "Deine Datenbank wurde auf Postgres {version} aktualisiert.",
    description: "Toast body when an upgrade the page watched succeeds",
  },
  "databases.versionUpgradeFailedTitle": {
    message: "Upgrade fehlgeschlagen",
    description: "Toast title when an upgrade the page watched fails",
  },
  "databases.versionUpgradeFailedBannerTitle": {
    message: "Upgrade fehlgeschlagen.",
    description:
      "Banner title on Info and the upgrade page for a failed upgrade nobody dismissed",
  },
  "databases.versionUpgradeFailedBody": {
    message:
      "Deine Datenbank läuft weiterhin auf ihrer aktuellen Version. Prüfe die Upgrade-Logs, behebe das Problem und versuche es dann erneut.",
    description: "Banner and toast body for a failed upgrade",
  },
  "databases.versionUpgradeDismiss": {
    message: "Fehler ausblenden",
    description:
      "Button that acknowledges a failed upgrade (Render: Dismiss error)",
  },
  "databases.versionUpgradeDismissError": {
    message:
      "Der Fehler konnte nicht ausgeblendet werden. Bitte versuche es erneut.",
    description: "Fallback error when acknowledging a failed upgrade fails",
  },
  "databases.versionUpgradeAvailableTitle": {
    message: "PostgreSQL {version} ist verfügbar.",
    description: "Info banner title when a newer PostgreSQL major is available",
  },
  "databases.versionUpgradeAvailableBody": {
    message: "Upgrade für bessere Performance und zusätzliche Funktionen.",
    description: "Info banner body when a newer PostgreSQL major is available",
  },
  "databases.versionUpgradeViewOptions": {
    message: "Upgrade-Optionen ansehen",
    description: "Info banner button that opens the version upgrade page",
  },
  "databases.versionUpgradeAvailableDismiss": {
    message: "Ausblenden",
    description:
      "Accessible name of the button that dismisses the newer-major banner",
  },
  "databases.versionUpgradeLink": {
    message: "Version upgraden",
    description:
      "Info Version row link when a newer major is available (Render: Upgrade version)",
  },
  "databases.versionUpgradeViewLink": {
    message: "Upgrade ansehen",
    description:
      "Info Version row link while an upgrade runs (Render: View upgrade)",
  },
  "databases.versionUpgradeReviewLink": {
    message: "Versionsdetails ansehen",
    description:
      "Info Version row link on the latest major (Render: Review version details)",
  },
  "databases.versionUpgradeCloneOf": {
    message: "Diese Datenbank ist ein Klon von {name}.",
    description:
      "Banner title on a clone made for a test upgrade; {name} is its source database",
  },
  "databases.versionUpgradeCloneOfBody": {
    message:
      "Klone werden genauso abgerechnet wie jede andere Datenbank mit denselben Spezifikationen. Lösche diesen Klon, wenn du mit dem Testen fertig bist.",
    description: "Banner body on a clone made for a test upgrade",
  },
  "databases.versionUpgradeDeleteClone": {
    message: "Diesen Klon löschen",
    description: "Clone banner button that opens the clone's delete action",
  },
  "databases.versionUpgradeGoToOriginal": {
    message: "Zur ursprünglichen Datenbank",
    description: "Clone banner link to the source database's upgrade page",
  },
  "databases.metaDatabaseName": {
    message: "Datenbank",
    description: "Database detail metadata row label (the normalized db name)",
  },
  "databases.metaDatabaseUser": {
    message: "Benutzer",
    description: "Database detail metadata row label (owner role)",
  },
  "databases.metaStorage": {
    message: "Speicher",
    description: "Database detail metadata row label (disk size)",
  },
  "databases.diskAutoscalingLabel": {
    message: "Disk-Autoscaling",
    description: "Label beside the database disk-usage chart toggle",
  },
  "databases.diskAutoscalingSize": {
    message: "{current} GB aktuell · {cap} GB max.",
    description: "Current and maximum Postgres disk size beside autoscaling",
  },
  "databases.diskAutoscalingHint": {
    message:
      "Bei 90 % Füllstand wächst der Speicher um 50 %, aufgerundet auf 5 GB. Vergrößerungen sind dauerhaft und nur einmal alle 12 Stunden möglich.",
    description: "Accessible explanation of Postgres disk autoscaling",
  },
  "databases.diskAutoscalingCurrentSize": {
    message: "{current} GB aktuell",
    description: "Current disk size while plan limits are unavailable",
  },
  "databases.diskAutoscalingPlanUnsupported": {
    message:
      "Dieser Tarif unterstützt kein Disk-Autoscaling. Eine bestehende Einstellung kann deaktiviert werden.",
    description: "Plan restriction beside the disk autoscaling switch",
  },
  "databases.diskAutoscalingEnabled": {
    message: "Disk-Autoscaling aktiviert.",
    description: "Toast after enabling Postgres disk autoscaling",
  },
  "databases.diskAutoscalingDisabled": {
    message: "Disk-Autoscaling deaktiviert.",
    description: "Toast after disabling Postgres disk autoscaling",
  },
  "databases.diskAutoscalingError": {
    message:
      "Disk-Autoscaling konnte nicht aktualisiert werden. Bitte versuche es erneut.",
    description: "Toast when the disk-autoscaling mutation fails",
  },
  "databases.metaHighAvailability": {
    message: "Hochverfügbarkeit",
    description:
      "Database detail metadata row label (HA — single-instance MVP is No)",
  },
  "databases.metaPublic": {
    message: "Öffentlicher Zugriff",
    description: "Database detail metadata row label (external endpoint)",
  },
  "databases.metaExternalHost": {
    message: "Externer Host",
    description: "Database detail metadata row label (SNI hostname)",
  },
  "databases.metaRegion": {
    message: "Region",
    description: "Database detail metadata row label (platform placement)",
  },
  "databases.metaCreated": {
    message: "Erstellt",
    description: "Database detail metadata row label (relative age)",
  },
  "databases.metaExpires": {
    message: "Läuft ab",
    description:
      "Details row: when a free database expires (Render's expiresAt, w8/m126)",
  },
  "databases.expirySoon": {
    message:
      "Diese kostenlose Datenbank läuft am {expiresAt} ab und wird dann nicht mehr bereitgestellt.",
    description: "Warning within 7 days of a free database's expiry",
  },
  "databases.expiryExpired": {
    message:
      "Diese kostenlose Datenbank ist abgelaufen und pausiert. Sie und ihre Daten werden am {deleteAt} gelöscht.",
    description:
      "Warning on an expired free database during its 14-day grace period",
  },
  "databases.expiryUpgrade": {
    message: "Auf einen kostenpflichtigen Tarif upgraden",
    description: "Link from the expiry warning to the Plan section",
  },
  "databases.expiryHintSoon": {
    message: "Läuft bald ab",
    description: "List-row hint on a free database within 7 days of expiry",
  },
  "databases.expiryHintExpired": {
    message: "Abgelaufen",
    description: "List-row hint on an expired free database",
  },
  "databases.statusReasonFreeExpired": {
    message:
      "Diese kostenlose Datenbank ist abgelaufen. Upgrade sie auf einen kostenpflichtigen Tarif, um sie wiederherzustellen.",
    description:
      "Why a suspended database is suspended: its free plan expired (bex-api statusReasonCode POSTGRES_FREE_EXPIRED, w8/m126)",
  },
  "databases.yes": {
    message: "Ja",
    description: "Metadata value for a true boolean field",
  },
  "databases.no": {
    message: "Nein",
    description: "Metadata value for a false boolean field",
  },
  "databases.connTitle": {
    message: "Verbindungen",
    description: "Connection-info panel card title",
  },
  "databases.connDescription": {
    message:
      "Verbindungs-Strings und das Datenbankpasswort. Werden nur auf Anfrage angezeigt – nie automatisch.",
    description: "Connection-info panel card description",
  },
  "databases.connReveal": {
    message: "Verbindungsinfos anzeigen",
    description: "Button that fetches the connection info on demand",
  },
  "databases.connHide": {
    message: "Verbindungsinfos ausblenden",
    description: "Button that clears the revealed connection info",
  },
  "databases.connLogicalReplicationTitle": {
    message: "Logische Replikation",
    description:
      "Heading of the logical-replication note on the Connections card",
  },
  "databases.connLogicalReplicationBody": {
    message:
      "Standardmäßig aus. Ab einem Pro-Workspace kann eine Datenbank mit mindestens 10 GB Speicher ihre Änderungen an Subscriber innerhalb oder außerhalb von bex veröffentlichen. Zum Aktivieren wende dich an den Support und nenne die ID dieser Datenbank, die Zugangsdaten, mit denen sich Subscriber verbinden, die Schemas, die sie lesen, und ob alle Tabellen veröffentlicht werden sollen. Erstelle danach Publikationen mit CREATE PUBLICATION … FOR TABLE als Benutzer dieser Datenbank und verbinde Subscriber mit der externen URL, nie mit der Pooler-URL. Sie kann auch eine andere Datenbank innerhalb oder außerhalb von bex abonnieren: Frag auf dieselbe Weise an, nenne die Zugangsdaten, die Subscriptions anlegen, und führe CREATE SUBSCRIPTION aus, sobald die Tabellen existieren (bei PostgreSQL 15 und älter legt der Support die Subscription für dich an).",
    description:
      "Logical-replication note on the Connections card: off by default, enabled on request (Render's rules)",
  },
  "databases.connPassword": {
    message: "Passwort",
    description: "Connection-info field label (database password)",
  },
  "databases.connShowPassword": {
    message: "Passwort anzeigen",
    description: "Accessible label to unmask the password",
  },
  "databases.connHidePassword": {
    message: "Passwort ausblenden",
    description: "Accessible label to re-mask the password",
  },
  "databases.connInternal": {
    message: "Interner Verbindungs-String",
    description: "Connection-info field label (in-cluster URL)",
  },
  "databases.connExternal": {
    message: "Externer Verbindungs-String",
    description: "Connection-info field label (public URL, TLS)",
  },
  "databases.connPsql": {
    message: "psql-Befehl",
    description: "Connection-info field label (ready-to-run psql command)",
  },
  "databases.connCaLabel": {
    message: "Server-CA-Zertifikat",
    description: "Connection-info field label (private server CA bundle)",
  },
  "databases.connCaDownload": {
    message: "CA-Zertifikat herunterladen",
    description: "Button downloading the server CA PEM file",
  },
  "databases.connCaHelp": {
    message:
      "Externe Verbindungen prüfen das Zertifikat dieser Datenbank gegen ihre private Server-CA (sslmode=verify-full bleibt erforderlich). Lade {file} herunter und verweise deinen Client auf die gespeicherte Datei – setze dafür beim psql-Befehl oder beim externen Verbindungs-String Folgendes voran:",
    description:
      "Explanation above the PGSSLROOTCERT example; {file} is the downloaded CA filename",
  },
  "databases.connCaSnippet": {
    message:
      "Beispiel für ein Umgebungs-Präfix, das auf die heruntergeladene CA zeigt",
    description:
      "Accessible label for the copyable PGSSLROOTCERT example snippet",
  },
  "databases.connCertPendingTitle": {
    message: "Öffentlich vertrauenswürdiges Zertifikat wird ausgestellt",
    description:
      "Connections card note title while a public database's Let's Encrypt certificate is still pending (w4/m357)",
  },
  "databases.connCertPendingBody": {
    message:
      "Ein öffentlich vertrauenswürdiges Zertifikat wird ausgestellt (meist wenige Minuten). Bis dahin verbindest du dich mit der CA unten oder mit sslmode=require. Sobald es ausgestellt ist, braucht die externe URL keine CA, wie bei Render; blende sie aus und wieder ein, um zu aktualisieren.",
    description:
      "Connections card note body while the public database certificate is pending",
  },
  "databases.connCertFailedTitle": {
    message: "Öffentlich vertrauenswürdiges Zertifikat noch nicht ausgestellt",
    description:
      "Connections card note title when issuing a public database's certificate failed (w4/m357)",
  },
  "databases.connCertFailedBody": {
    message:
      "Die Ausstellung des öffentlich vertrauenswürdigen Zertifikats dieser Datenbank ist fehlgeschlagen, und bex versucht es automatisch erneut. Der CA-Weg funktioniert in der Zwischenzeit: Verbinde dich mit der CA unten oder mit sslmode=require.",
    description:
      "Connections card note body when the public database certificate failed to issue",
  },
  "databases.connErrorTitle": {
    message: "Verbindungsinfos konnten nicht geladen werden",
    description: "Connection-info panel error title",
  },
  "databases.connErrorBody": {
    message:
      "Die Datenbank wird möglicherweise noch bereitgestellt, oder du hast keine Berechtigung, ihre Zugangsdaten einzusehen.",
    description: "Connection-info panel error body",
  },
  "databases.copied": {
    message: "In die Zwischenablage kopiert",
    description: "Toast after copying a connection field",
  },
  "databases.copyError": {
    message: "Kopieren in die Zwischenablage fehlgeschlagen",
    description: "Toast when clipboard copy fails",
  },
  "databases.loading": {
    message: "Wird geladen…",
    description: "Generic loading placeholder in a detail panel",
  },
  "databases.dangerDelete": {
    message: "Datenbank löschen",
    description: "Detail-page bottom action: permanently delete the database",
  },
  "databases.dangerRestart": {
    message: "Datenbank neu starten",
    description: "Detail-page bottom action: rolling restart of the primary",
  },
  "databases.dangerSuspend": {
    message: "Datenbank pausieren",
    description:
      "Detail-page bottom action: hibernate the database (stop compute, keep data)",
  },
  "databases.dangerResume": {
    message: "Datenbank fortsetzen",
    description: "Detail-page bottom action: wake a suspended database",
  },
  "databases.actionSuspend": {
    message: "Pausieren",
    description: "Row action: hibernate the database (stop compute, keep data)",
  },
  "databases.actionResume": {
    message: "Fortsetzen",
    description: "Row action: wake a suspended database",
  },
  "databases.actionRestart": {
    message: "Neu starten",
    description: "Row action: rolling restart of the primary",
  },
  "databases.suspendConfirmTitle": {
    message: "{name} pausieren?",
    description: "Suspend-confirmation dialog title",
  },
  "databases.suspendConfirmBody": {
    message:
      "Die Compute-Ressourcen werden gestoppt und die Datenbank ist nicht erreichbar, ihre Daten bleiben aber erhalten. Setze sie jederzeit fort, um sie wieder zu aktivieren.",
    description: "Suspend-confirmation dialog body",
  },
  "databases.restartConfirmTitle": {
    message: "{name} neu starten?",
    description: "Restart-confirmation dialog title",
  },
  "databases.restartConfirmBody": {
    message:
      "Der Primary wird neu gestartet. Verbindungen brechen kurz ab, bis er wieder da ist.",
    description: "Restart-confirmation dialog body",
  },
  "databases.toastSuspendSuccess": {
    message: "{name} wird pausiert…",
    description: "Toast after a suspend request is accepted",
  },
  "databases.toastResumeSuccess": {
    message: "{name} wird fortgesetzt…",
    description: "Toast after a resume request is accepted",
  },
  "databases.toastRestartSuccess": {
    message: "{name} wird neu gestartet…",
    description: "Toast after a restart request is accepted",
  },
  "databases.toastLifecycleError": {
    message:
      "Die Aktion für {name} konnte nicht abgeschlossen werden. Bitte versuche es erneut.",
    description: "Toast when a lifecycle verb fails",
  },
  "databases.recoveryTitle": {
    message: "Wiederherstellung",
    description: "Recovery panel card title",
  },
  "databases.recoveryDescription": {
    message:
      "Point-in-Time-Recovery und Backups. Eine Wiederherstellung erstellt immer eine neue Datenbank – diese wird nie verändert.",
    description: "Recovery panel card description",
  },
  "databases.recoveryDisabled": {
    message:
      "Backups sind für diesen Tarif nicht aktiviert, daher ist keine Wiederherstellung verfügbar. Wechsle auf einen Tarif mit Backups, um Point-in-Time-Recovery zu aktivieren.",
    description: "Recovery panel state when the plan has no backups",
  },
  "databases.recoveryUnavailable": {
    message:
      "Wiederherstellungsinformationen konnten nicht gelesen werden. Bitte versuche es erneut.",
    description: "Recovery panel state when backup metadata could not be read",
  },
  "databases.recoveryEarliest": {
    message: "Frühester Wiederherstellungspunkt",
    description: "Recovery panel field (earliest recoverable time)",
  },
  "databases.recoveryLatest": {
    message: "Spätester Wiederherstellungspunkt",
    description: "Recovery panel field (latest recoverable time)",
  },
  "databases.recoveryWindowNotEstablished": {
    message: "Noch nicht festgelegt",
    description: "Recovery panel value before the PITR window is established",
  },
  "databases.recoveryRestore": {
    message: "In neue Instanz wiederherstellen",
    description: "Recovery panel button that opens the restore dialog",
  },
  "databases.recoveryRestoreUnavailable": {
    message:
      "Die Wiederherstellung ist verfügbar, sobald der erste Wiederherstellungspunkt festgelegt ist.",
    description: "Reason restore is disabled before the PITR window opens",
  },
  "databases.recoveryCreateExport": {
    message: "Export erstellen",
    description: "Recovery panel button that triggers an on-demand export",
  },
  "databases.recoveryBackups": {
    message: "Backups",
    description: "Recovery panel backup-list heading",
  },
  "databases.recoveryNoBackups": {
    message: "Noch keine Backups.",
    description: "Recovery panel empty backup list",
  },
  "databases.recoveryExportScope": {
    message:
      "Ein Export enthält nur die Datenbank {database}. Andere in dieser Instanz erstellte Datenbanken sind nicht enthalten.",
    description:
      "Recovery panel note naming the one database a logical export contains (w8/258)",
  },
  "databases.recoveryExports": {
    message: "Exporte",
    description: "Recovery panel export-list heading",
  },
  "databases.recoveryNoExports": {
    message: "Noch keine Exporte.",
    description: "Recovery panel empty export list",
  },
  "databases.recoveryDownloadExport": {
    message: "Herunterladen",
    description: "Download an available logical database export",
  },
  "databases.recoveryDownloadUnavailable": {
    message: "Dieser Export steht nicht mehr zum Download bereit.",
    description: "Toast when Download finds the export is no longer available",
  },
  "databases.recoveryDownloadError": {
    message:
      "Der Download-Link konnte nicht abgerufen werden. Bitte versuche es erneut.",
    description: "Toast when the download link of an export cannot be fetched",
  },
  "databases.recoveryExportInProgress": {
    message: "Ein Export läuft bereits.",
    description: "Reason the create-export button is disabled",
  },
  "databases.recoveryExportPreparing": {
    message: "Der Export wird noch vorbereitet.",
    description: "Reason an in-progress export cannot be downloaded",
  },
  "databases.recoveryExportExpired": {
    message: "Dieser Export ist abgelaufen und sein Artefakt wurde entfernt.",
    description: "Reason an expired export cannot be downloaded",
  },
  "databases.recoveryExportUnavailable": {
    message: "Dieser Export steht nicht zum Download bereit.",
    description: "Fallback reason an export cannot be downloaded",
  },
  "databases.recoveryExportFailure": {
    message: "Export fehlgeschlagen: {reason}",
    description: "Logical export failure reason",
  },
  "databases.recoveryExportStatusCreated": {
    message: "Erstellt",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusRunning": {
    message: "Läuft",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusAvailable": {
    message: "Verfügbar",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusFailed": {
    message: "Fehlgeschlagen",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusExpiring": {
    message: "Läuft ab",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusExpired": {
    message: "Abgelaufen",
    description: "Logical export status",
  },
  "databases.recoveryRestoreTitle": {
    message: "In neue Datenbank wiederherstellen",
    description: "Restore dialog title",
  },
  "databases.recoveryRestoreBody": {
    message:
      "Stelle eine neue Datenbank bereit, die aus den Backups dieser Datenbank wiederhergestellt wird. Die Quelle bleibt unangetastet.",
    description: "Restore dialog body",
  },
  "databases.recoveryRestoreName": {
    message: "Name der neuen Datenbank",
    description: "Restore dialog field label",
  },
  "databases.recoveryRestoreTime": {
    message: "Zeitpunkt (optional)",
    description: "Restore dialog target-time field label",
  },
  "databases.recoveryRestoreTimeHint": {
    message:
      "Lass das Feld leer, um den neuesten verfügbaren Punkt wiederherzustellen.",
    description: "Restore dialog target-time helper text",
  },
  "databases.recoveryRestoreTimeZone": {
    message: "Zeiten beziehen sich auf deine lokale Zeitzone ({zone}).",
    description: "Restore dialog target-time zone note",
  },
  "databases.recoveryRestoreTimeOutOfWindow": {
    message:
      "Wähle einen Zeitpunkt zwischen dem frühesten Wiederherstellungspunkt und vor zehn Minuten.",
    description:
      "Restore dialog target-time out-of-window error (the window ends ten minutes before now, w7/m453)",
  },
  "databases.recoveryRetentionWindow_one": {
    message:
      "Im {plan}-Tarif lässt sich jeder Zeitpunkt bis zu {count} Tag zurück wiederherstellen.",
    description:
      "Recovery panel hint: how far back the workspace plan may restore a Postgres (w7/m453, from bex-api databaseRecoveryInfo.retention)",
  },
  "databases.recoveryRetentionWindow_other": {
    message:
      "Im {plan}-Tarif lässt sich jeder Zeitpunkt bis zu {count} Tage zurück wiederherstellen.",
    description:
      "Recovery panel hint: how far back the workspace plan may restore a Postgres (w7/m453, from bex-api databaseRecoveryInfo.retention)",
  },
  "databases.recoveryRetentionUpgrade_one": {
    message: "{plan} reicht {count} Tag zurück.",
    description:
      "Recovery panel hint: the cheapest plan that restores further back, and its days (w7/m453)",
  },
  "databases.recoveryRetentionUpgrade_other": {
    message: "{plan} reicht {count} Tage zurück.",
    description:
      "Recovery panel hint: the cheapest plan that restores further back, and its days (w7/m453)",
  },
  "databases.recoveryRestoreConfirm": {
    message: "Wiederherstellen",
    description: "Restore dialog confirm button",
  },
  "databases.recoveryExportStarted": {
    message: "Export gestartet.",
    description: "Toast after an export is triggered",
  },
  "databases.recoveryExportError": {
    message:
      "Der Export konnte nicht gestartet werden. Bitte versuche es erneut.",
    description: "Toast when an export request fails",
  },
  "databases.recoveryRestoreStarted": {
    message: "Wiederherstellung in {name}…",
    description: "Toast after a restore request is accepted",
  },
  "databases.recoveryRestoreError": {
    message: "Wiederherstellung fehlgeschlagen. Bitte versuche es erneut.",
    description: "Toast when a restore request fails",
  },
  "databases.accessTitle": {
    message: "Zugriffskontrolle",
    description: "Access-control panel card title",
  },
  "databases.accessDescription": {
    message:
      "Beschränke den externen Endpunkt, poole Verbindungen und verwalte Datenbankbenutzer.",
    description: "Access-control panel card description",
  },
  "databases.accessAllowList": {
    message: "IP-Allowlist",
    description:
      "Access panel section heading (external-endpoint CIDR allowlist)",
  },
  "databases.accessAllowListHint": {
    message:
      "Das Speichern von CIDR-Regeln aktiviert den externen Zugriff für passende Quell-IPs. Das Speichern einer leeren Liste blockiert externe Verbindungen. Der interne Zugriff bleibt unverändert.",
    description: "Access panel IP allowlist helper text",
  },
  "databases.accessAllowListInternalOnly": {
    message:
      "Externe Verbindungen sind deaktiviert. Speichere eine oder mehrere IP-Regeln, um den Zugriff zu aktivieren.",
    description: "External access is disabled; saving rules enables it",
  },
  "databases.accessAllowListEmpty": {
    message:
      "Keine IP-Regeln. Speichere eine leere Liste, um externe Verbindungen zu blockieren.",
    description: "Empty datastore allowlist draft and the effect of saving it",
  },
  "databases.accessAllowListAdd": {
    message: "Hinzufügen",
    description: "Access panel button to add a CIDR to the draft allowlist",
  },
  "databases.accessAllowListDescription": {
    message: "Beschreibung (optional)",
    description:
      "Placeholder for the optional per-entry allowlist description input",
  },
  "databases.accessAllowListCIDR": {
    message: "Neuer CIDR-Block",
    description: "Accessible label for the add-row CIDR input",
  },
  "databases.accessAllowListCIDRRule": {
    message: "CIDR-Block für Regel {number}",
    description: "Accessible label for an existing allowlist CIDR row",
  },
  "databases.accessAllowListDescriptionLabel": {
    message: "Beschreibung der neuen Regel",
    description: "Accessible label for the add-row description input",
  },
  "databases.accessAllowListDescriptionRule": {
    message: "Beschreibung für Regel {number}",
    description: "Accessible label for an existing allowlist description row",
  },
  "databases.accessAllowListRemove": {
    message: "{cidr} entfernen",
    description: "Accessible label to remove a CIDR chip",
  },
  "databases.accessAllowListMoveUp": {
    message: "{cidr} nach oben verschieben",
    description: "Accessible label to move an allowlist entry earlier",
  },
  "databases.accessAllowListMoveDown": {
    message: "{cidr} nach unten verschieben",
    description: "Accessible label to move an allowlist entry later",
  },
  "databases.accessAllowListInvalid": {
    message: "Gib eindeutige, gültige IPv4- oder IPv6-CIDR-Blöcke ein.",
    description: "Validation error for database allowlist entries",
  },
  "databases.accessAllowListSave": {
    message: "Allowlist speichern",
    description: "Access panel button to persist the allowlist",
  },
  "databases.accessAllowListSaved": {
    message: "Allowlist aktualisiert.",
    description: "Toast after saving the IP allowlist",
  },
  "databases.accessAllowListError": {
    message:
      "Die Allowlist konnte nicht gespeichert werden. Bitte versuche es erneut.",
    description: "Toast when saving the allowlist fails",
  },
  "databases.accessCredentials": {
    message: "Zugangsdaten",
    description:
      "Access panel section heading (Postgres credentials, Render's Credentials section)",
  },
  "databases.accessCredentialsHint": {
    message:
      "Füge eine neue Standard-Zugangsdaten hinzu, um das Datenbankpasswort zu rotieren. Die Verbindungsinfos wechseln darauf, und verknüpfte Services übernehmen sie beim nächsten Blueprint-Sync. Ältere Zugangsdaten funktionieren weiter, bis du sie löschst.",
    description: "Access panel credentials helper text",
  },
  "databases.accessCredentialDefault": {
    message: "Standard",
    description: "Badge on the database's default credential",
  },
  "databases.accessCredentialAdd": {
    message: "Neue Standard-Zugangsdaten",
    description:
      "Access panel button to create a credential that becomes the default",
  },
  "databases.accessCredentialName": {
    message: "Benutzername der neuen Zugangsdaten",
    description: "Accessible label for the new-credential username input",
  },
  "databases.accessCredentialDelete": {
    message: "{name} löschen",
    description: "Accessible label to delete a database credential",
  },
  "databases.accessCredentialDeleteTitle": {
    message: "Zugangsdaten {name} löschen?",
    description:
      "Title of the confirmation before deleting a database credential (w4/551)",
  },
  "databases.accessCredentialDeleteBody": {
    message:
      "Jede Verbindung, die {name} verwendet, schlägt sofort fehl – auch bei Services, deren Umgebungsvariablen den zugehörigen Verbindungs-String enthalten. Das kann nicht rückgängig gemacht werden.",
    description:
      "Body of the credential delete confirmation: retiring the login breaks its clients at once (w4/551)",
  },
  "databases.accessCredentialDeleteConfirm": {
    message: "Zugangsdaten löschen",
    description:
      "Confirm button of the credential delete confirmation (w4/551)",
  },
  "databases.accessCredentialDefaultLocked": {
    message:
      "Die Standard-Zugangsdaten können nicht gelöscht werden. Füge zuerst neue Standard-Zugangsdaten hinzu.",
    description: "Why the delete button is disabled on the default credential",
  },
  "databases.accessCredentialCreated": {
    message:
      "{name} ist jetzt der Standardzugang. Das Passwort findest du in den Verbindungsinfos.",
    description: "Toast after creating a credential",
  },
  "databases.accessCredentialError": {
    message:
      "Die Zugangsdaten konnten nicht erstellt werden. Bitte versuche es erneut.",
    description: "Toast when creating a credential fails",
  },
  "databases.accessCredentialDeleted": {
    message: "Widerruf für {name} angefordert.",
    description:
      "Toast after deleting a credential: the login is retired asynchronously on the next reconcile, so this confirms accepted intent, not a completed revocation",
  },
  "databases.accessCredentialDeleteError": {
    message: "{name} konnte nicht gelöscht werden. Bitte versuche es erneut.",
    description: "Toast when deleting a credential fails",
  },
  "databases.accessPooler": {
    message: "Connection Pooling",
    description: "Access panel section heading (PgBouncer pooler strings)",
  },
  "databases.accessPoolerHint": {
    message:
      "Gepoolte Verbindungs-Strings über PgBouncer, für Clients mit vielen Verbindungen.",
    description: "Access panel pooler helper text",
  },
  "databases.accessPoolerReveal": {
    message: "Gepoolte Strings anzeigen",
    description: "Access panel button to fetch the pooled connection strings",
  },
  "databases.accessPoolerDisabled": {
    message:
      "Aktiviere oben Connection Pooling, um gepoolte Verbindungs-Strings zu erhalten.",
    description:
      "Access panel shown when no pooler is enabled; points at the Connection Pool switch in the section heading (w2/m307)",
  },
  "databases.accessPoolerInternal": {
    message: "Interne gepoolte Verbindung",
    description: "Access panel pooled-string label (in-cluster)",
  },
  "databases.accessPoolerPlanUnsupported": {
    message:
      "Dieser Tarif unterstützt kein Connection Pooling. Wähle einen Tarif, der es unterstützt, bevor du einen Pooler aktivierst.",
    description: "Pooler empty state on an unsupported plan",
  },
  "databases.accessPoolerExternal": {
    message: "Externe gepoolte Verbindung",
    description: "Access panel pooled-string label (public)",
  },
  "databases.accessPoolerReplicaInternal": {
    message: "Interne gepoolte Verbindung, Lesereplikat {name}",
    description:
      "Access panel pooled-string label for a read replica's in-cluster pooled connection (w6/m458)",
  },
  "databases.accessPoolerReplicaExternal": {
    message: "Externe gepoolte Verbindung, Lesereplikat {name}",
    description:
      "Access panel pooled-string label for a read replica's public pooled connection (w6/m458)",
  },
  "databases.accessPoolerError": {
    message: "Die gepoolten Verbindungs-Strings konnten nicht geladen werden.",
    description: "Toast when revealing pooled strings fails",
  },
  "databases.accessPoolerStarting": {
    message:
      "Connection Pooling wird gestartet. Die gepoolten Verbindungs-Strings erscheinen hier, sobald PgBouncer bereit ist.",
    description:
      "Access panel when pooling is on but the operator has not published the pooler hosts yet (w2/m307)",
  },
  "databases.poolerToggleLabel": {
    message: "Connection Pool",
    description:
      "Accessible label of the Connection Pool switch (Render's Info > Connection Pool, w2/m307)",
  },
  "databases.poolerEnableTitle": {
    message: "Connection Pooling für {name} aktivieren?",
    description:
      "Connection-pool enable confirmation title; {name} is the database",
  },
  "databases.poolerDisableTitle": {
    message: "Connection Pooling für {name} deaktivieren?",
    description:
      "Connection-pool disable confirmation title; {name} is the database",
  },
  "databases.poolerEnableBody": {
    message:
      "Beim Aktivieren von Connection Pooling wird die Datenbank neu gestartet, was einige Minuten Downtime verursacht. Die gepoolten Verbindungs-Strings erscheinen in diesem Abschnitt, sobald PgBouncer bereit ist.",
    description:
      "Connection-pool enable confirmation body for a database without high availability (Render: enabling restarts the database)",
  },
  "databases.poolerEnableBodyHA": {
    message:
      "Beim Aktivieren von Connection Pooling wird auf den Standby umgeschaltet, statt die Datenbank neu zu starten, daher brechen offene Verbindungen kurz ab. Die gepoolten Verbindungs-Strings erscheinen in diesem Abschnitt, sobald PgBouncer bereit ist.",
    description:
      "Connection-pool enable confirmation body when high availability is on (Render: a failover instead of a restart)",
  },
  "databases.poolerDisableBody": {
    message:
      "Beim Deaktivieren von Connection Pooling wird die Datenbank neu gestartet, was einige Minuten Downtime verursacht. Die gepoolten Verbindungs-Strings funktionieren dann nicht mehr.",
    description:
      "Connection-pool disable confirmation body for a database without high availability",
  },
  "databases.poolerDisableBodyHA": {
    message:
      "Beim Deaktivieren von Connection Pooling wird auf den Standby umgeschaltet, statt die Datenbank neu zu starten, daher brechen offene Verbindungen kurz ab. Die gepoolten Verbindungs-Strings funktionieren dann nicht mehr.",
    description:
      "Connection-pool disable confirmation body when high availability is on",
  },
  "databases.poolerEnableConfirm": {
    message: "Connection Pool aktivieren",
    description: "Connection-pool enable confirmation button (Render's label)",
  },
  "databases.poolerDisableConfirm": {
    message: "Connection Pool deaktivieren",
    description: "Connection-pool disable confirmation button",
  },
  "databases.poolerEnableSuccess": {
    message: "Connection Pooling wird für {name} aktiviert.",
    description: "Toast after the connection-pool enable was accepted",
  },
  "databases.poolerDisableSuccess": {
    message: "Connection Pooling wird für {name} deaktiviert.",
    description: "Toast after the connection-pool disable was accepted",
  },
  "databases.poolerEnableError": {
    message: "Connection Pooling konnte für {name} nicht aktiviert werden.",
    description: "Fallback toast when enabling the connection pool fails",
  },
  "databases.poolerDisableError": {
    message: "Connection Pooling konnte für {name} nicht deaktiviert werden.",
    description: "Fallback toast when disabling the connection pool fails",
  },
  "databases.haTitle": {
    message: "Hochverfügbarkeit",
    description: "HA panel card title",
  },
  "databases.haDescription": {
    message:
      "Replizierter Cluster mit automatischem Failover. Wenn aktiviert, steht immer ein Standby zur Übernahme bereit.",
    description: "HA panel card description",
  },
  "databases.haStatus": {
    message: "Status",
    description: "HA panel status row label",
  },
  "databases.haEnabled": {
    message: "Aktiviert",
    description: "HA panel status value when HA is active",
  },
  "databases.haDisabled": {
    message: "Deaktiviert",
    description: "HA panel status value when HA is off",
  },
  "databases.haFailover": {
    message: "Failover auslösen",
    description: "HA panel button to initiate a planned switchover",
  },
  "databases.haFailoverConfirmTitle": {
    message: "Failover auf {name} auslösen?",
    description: "Failover-confirmation dialog title",
  },
  "databases.haFailoverConfirmBody": {
    message:
      "Dadurch wird ein Standby zum Primary befördert. Verbindungen brechen während der Umschaltung kurz ab. Die Datenbank bleibt verfügbar – es gehen keine Daten verloren.",
    description: "Failover-confirmation dialog body",
  },
  "databases.haFailoverCancel": {
    message: "Abbrechen",
    description: "Failover-confirmation dialog cancel button",
  },
  "databases.haFailoverConfirm": {
    message: "Failover auslösen",
    description: "Failover-confirmation dialog confirm button",
  },
  "databases.haFailoverSuccess": {
    message: "Failover auf {name} ausgelöst.",
    description: "Toast after a failover request is accepted",
  },
  "databases.haFailoverError": {
    message:
      "Failover auf {name} konnte nicht ausgelöst werden. Bitte versuche es erneut.",
    description: "Toast when a failover request fails",
  },
  "databases.haReadReplicas": {
    message: "Lesereplikate",
    description: "HA panel section heading for named read replicas",
  },
  "databases.haReadReplicasEmpty": {
    message: "Keine benannten Lesereplikate.",
    description: "HA panel empty state for read replicas",
  },
  "databases.haReplicaInternal": {
    message: "Intern",
    description: "HA panel replica connection label (in-cluster host)",
  },
  "databases.haReplicaExternal": {
    message: "Extern",
    description: "HA panel replica connection label (public SNI host)",
  },
  "databases.haAddReplica": {
    message: "Lesereplikat hinzufügen",
    description:
      "HA panel button that opens the add-read-replica dialog (w5/m138)",
  },
  "databases.haAddReplicaTitle": {
    message: "Lesereplikat hinzufügen",
    description: "Add-read-replica dialog title",
  },
  "databases.haAddReplicaBody": {
    message:
      "Ein Lesereplikat ist eine eigene, kostenpflichtige Instanz, die vom Primary streamt und schreibgeschützte Abfragen unter ihrer eigenen Adresse bedient.",
    description: "Add-read-replica dialog description",
  },
  "databases.haReplicaNameLabel": {
    message: "Name",
    description: "Add-read-replica dialog name field label",
  },
  "databases.haReplicaNameHint": {
    message:
      "Kleinbuchstaben, Ziffern und Bindestriche, bis zu {max} Zeichen. Der Name wird Teil des Hostnamens des Replikats.",
    description: "Add-read-replica name hint; max is the API's length limit",
  },
  "databases.haReplicaNameInvalid": {
    message:
      "Verwende 1–{max} Kleinbuchstaben, Ziffern oder Bindestriche und beginne und ende mit einem Buchstaben oder einer Ziffer.",
    description:
      "Add-read-replica name error; mirrors the API's POSTGRES_READ_REPLICA_NAME_INVALID",
  },
  "databases.haReplicaNameDuplicate": {
    message:
      "Diese Datenbank hat bereits ein Lesereplikat mit dem Namen {name}.",
    description: "Add-read-replica name error for a name already in use",
  },
  "databases.haAddReplicaSubmit": {
    message: "Replikat hinzufügen",
    description: "Add-read-replica dialog submit button",
  },
  "databases.haAddReplicaSuccess": {
    message: "Lesereplikat {name} wird erstellt.",
    description: "Toast after a read replica is added",
  },
  "databases.haAddReplicaError": {
    message: "Lesereplikat {name} konnte nicht hinzugefügt werden.",
    description: "Toast when adding a read replica fails",
  },
  "databases.haRemoveReplica": {
    message: "Entfernen",
    description: "HA panel per-replica remove button",
  },
  "databases.haRemoveReplicaTitle": {
    message: "Lesereplikat {name} entfernen?",
    description: "Remove-read-replica confirmation title",
  },
  "databases.haRemoveReplicaBody": {
    message:
      "Die Instanz wird gelöscht und ihre Verbindungs-Strings funktionieren nicht mehr. Der Primary bleibt unberührt.",
    description: "Remove-read-replica confirmation body",
  },
  "databases.haRemoveReplicaConfirm": {
    message: "Replikat entfernen",
    description: "Remove-read-replica confirm button",
  },
  "databases.haRemoveReplicaSuccess": {
    message: "Lesereplikat {name} wird entfernt.",
    description: "Toast after a read replica is removed",
  },
  "databases.haRemoveReplicaError": {
    message: "Lesereplikat {name} konnte nicht entfernt werden.",
    description: "Toast when removing a read replica fails",
  },
  "databases.haReplicaPlanUnsupported": {
    message: "Lesereplikate sind im Tarif dieser Datenbank nicht verfügbar.",
    description: "Why Add read replica is disabled: the plan allows none",
  },
  "databases.haReplicaLimitReached": {
    message:
      "Diese Datenbank hat die maximale Anzahl von {max} Lesereplikaten.",
    description:
      "Why Add read replica is disabled: the plan's replica limit is reached",
  },
  "databases.haReplicaStorageTooSmall": {
    message:
      "Lesereplikate benötigen mindestens {min} GB Speicher; diese Datenbank hat {current} GB.",
    description:
      "Why Add read replica is disabled: storage is below the replica minimum",
  },
  "databases.haNotEnabled": {
    message: "Hochverfügbarkeit ist für diese Datenbank nicht aktiviert.",
    description: "HA panel line when HA is off on a plan that offers it",
  },
  "databases.haPlanUnsupported": {
    message:
      "Hochverfügbarkeit erfordert einen Tarif mit mindestens 1 CPU. Der Tarif dieser Datenbank bietet sie nicht.",
    description:
      "HA panel state when HA is off and the current plan is below 1 CPU (w8/m43, Render's rule)",
  },
  "databases.haToggleLabel": {
    message: "Hochverfügbarkeit",
    description: "Accessible name of the HA panel's on/off switch (w2/m171)",
  },
  "databases.haStarting": {
    message:
      "Ein Standby wird auf einem anderen Node gestartet. Failover ist verfügbar, sobald er bereit ist.",
    description:
      "HA panel line while HA is on but the standby is not ready yet",
  },
  "databases.haStopping": {
    message: "Hochverfügbarkeit wird ausgeschaltet. Der Standby wird entfernt.",
    description: "HA panel line while HA is off but the standby still runs",
  },
  "databases.haEnableTitle": {
    message: "Hochverfügbarkeit für {name} aktivieren?",
    description: "HA enable confirmation title",
  },
  "databases.haEnableBody": {
    message:
      "Eine Standby-Instanz wird auf einem anderen Node gestartet und übernimmt automatisch, wenn der Primary ausfällt. Sie wird wie der Primary abgerechnet.",
    description: "HA enable confirmation body",
  },
  "databases.haEnablePrice": {
    message: "Der Standby kostet zusätzlich {price}.",
    description:
      "HA enable confirmation price note; {price} is a formatted monthly price",
  },
  "databases.haEnableConfirm": {
    message: "Hochverfügbarkeit aktivieren",
    description: "HA enable confirmation button",
  },
  "databases.haDisableTitle": {
    message: "Hochverfügbarkeit für {name} deaktivieren?",
    description: "HA disable confirmation title",
  },
  "databases.haDisableBody": {
    message:
      "Der Standby wird entfernt. Wenn der Primary ausfällt, ist die Datenbank nicht verfügbar, bis er sich erholt.",
    description: "HA disable confirmation body",
  },
  "databases.haDisableConfirm": {
    message: "Hochverfügbarkeit deaktivieren",
    description: "HA disable confirmation button",
  },
  "databases.haEnableSuccess": {
    message: "Hochverfügbarkeit wird für {name} aktiviert.",
    description: "Toast after HA was enabled",
  },
  "databases.haDisableSuccess": {
    message: "Hochverfügbarkeit wird für {name} deaktiviert.",
    description: "Toast after HA was disabled",
  },
  "databases.haEnableError": {
    message: "Hochverfügbarkeit konnte für {name} nicht aktiviert werden.",
    description: "Fallback toast when enabling HA fails",
  },
  "databases.haDisableError": {
    message: "Hochverfügbarkeit konnte für {name} nicht deaktiviert werden.",
    description: "Fallback toast when disabling HA fails",
  },
  "databases.planPickerHAUnsupported": {
    message:
      "{name} unterstützt keine Hochverfügbarkeit (sie erfordert mindestens 1 CPU). Deaktiviere zuerst die Hochverfügbarkeit.",
    description:
      "Plan picker refusal when an HA database selects a plan below 1 CPU; mirrors the API message (w8/m43)",
  },
  "databases.planPickerStorageUnsupported": {
    message:
      "{name} erlaubt bis zu {max} GB Speicher. Diese Datenbank hat {current} GB, was nicht reduziert werden kann. Wähle einen Tarif mit ausreichend Speicher.",
    description: "Plan change blocked by the database's existing storage",
  },
  "databases.planPickerReplicasUnsupported": {
    message:
      "Limit für Lesereplikate bei {name}: {max}. Diese Datenbank hat {current}.",
    description: "Plan change blocked by the database's read replicas",
  },
  "databases.planPickerAutoscalingUnsupported": {
    message:
      "{name} unterstützt kein Disk-Autoscaling. Schalte zuerst das Disk-Autoscaling aus.",
    description: "Plan change blocked while disk autoscaling is enabled",
  },
  "databases.planPickerPoolerUnsupported": {
    message:
      "{name} unterstützt kein Connection Pooling. Deaktiviere zuerst den Pooler.",
    description: "Plan change blocked while a connection pooler is enabled",
  },
  "databases.planTitle": {
    message: "Compute-Plan",
    description: "Database detail plan-picker card title",
  },
  "databases.planDescription": {
    message:
      "Ändere den Compute-Plan. Eine Datenbank ohne Standby wird dafür neu gestartet und ist ein bis zwei Minuten nicht verfügbar. Deine Daten bleiben erhalten.",
    description: "Database detail plan-picker card description",
  },
  "databases.planPickerSave": {
    message: "Speichern",
    description: "Database plan-picker save button",
  },
  "databases.planPickerCancel": {
    message: "Abbrechen",
    description: "Database plan-picker cancel / reset button",
  },
  "databases.reviewComputeLead": {
    message: "Prüfe deine Auswahl für Compute und Speicher.",
    description:
      "Postgres plan review: the lead of a compute plan change's review (Render's copy)",
  },
  "databases.reviewStorageLead": {
    message: "Prüfe deine Speicherauswahl.",
    description:
      "Postgres plan review: the lead of a storage change's review (Render's copy)",
  },
  "databases.reviewEditHighAvailability": {
    message: "Hochverfügbarkeit bearbeiten",
    description:
      "Postgres plan review: accessible name of the High availability line's Edit button, which goes to the High Availability section",
  },
  "databases.reviewReplicaCount_one": {
    message: "{count} Replikat",
    description:
      "Postgres plan review: the Read replicas line's figure, the database's replica count",
  },
  "databases.reviewReplicaCount_other": {
    message: "{count} Replikate",
    description:
      "Postgres plan review: the Read replicas line's figure, the database's replica count",
  },
  "databases.reviewStorageInstances_one": {
    message: "{size} GB (x{count} Instanz)",
    description:
      'Postgres plan review: the Storage line\'s figure when the size is billed on several instances (primary, HA standby, read replicas); Render writes "GB (x3 instances)"',
  },
  "databases.reviewStorageInstances_other": {
    message: "{size} GB (x{count} Instanzen)",
    description:
      'Postgres plan review: the Storage line\'s figure when the size is billed on several instances (primary, HA standby, read replicas); Render writes "GB (x3 instances)"',
  },
  "databases.reviewUnavailableSeconds": {
    message:
      "Deine Datenbank ist während der Aktualisierung einige Sekunden lang nicht verfügbar.",
    description:
      "Postgres plan review warning: a database with high availability fails over to its standby, so the outage is seconds (Render's copy)",
  },
  "databases.reviewUnavailableMinutes": {
    message:
      "Deine Datenbank ist während der Aktualisierung einige Minuten lang nicht verfügbar.",
    description:
      "Postgres plan review warning: a database without high availability restarts its only instance, so the outage is minutes (Render's copy)",
  },
  "databases.reviewUnavailableReplicasSeconds": {
    message:
      "Deine Datenbank und ihre Replikate sind während der Aktualisierung einige Sekunden lang nicht verfügbar.",
    description:
      "Postgres plan review warning: as reviewUnavailableSeconds, for a database with read replicas, which restart too (Render's copy)",
  },
  "databases.reviewUnavailableReplicasMinutes": {
    message:
      "Deine Datenbank und ihre Replikate sind während der Aktualisierung einige Minuten lang nicht verfügbar.",
    description:
      "Postgres plan review warning: as reviewUnavailableMinutes, for a database with read replicas, which restart too (Render's copy)",
  },
  "databases.reviewUpdateCompute": {
    message: "Compute aktualisieren",
    description:
      "Postgres plan review: the button that applies a compute plan change (Render's copy)",
  },
  "databases.reviewUpdateStorage": {
    message: "Speicher aktualisieren",
    description:
      "Postgres plan review: the button that applies a storage change (Render's copy)",
  },
  "databases.planPickerSuccess": {
    message: "Tarif wird auf {name} geändert…",
    description: "Toast after a plan update is accepted",
  },
  "databases.planPickerError": {
    message:
      "Der Tarif konnte nicht geändert werden. Bitte versuche es erneut.",
    description: "Toast when a plan update fails",
  },
  "databases.storageTitle": {
    message: "Speicher",
    description:
      "Plan section Storage row heading (Render's Plan page Storage section, w2/m307)",
  },
  "databases.storageSize": {
    message: "{size} GB",
    description: "Plan section Storage row: the allocated storage",
  },
  "databases.storageFixed": {
    message:
      "Kostenlose Datenbanken haben festen Speicher. Wechsle auf einen kostenpflichtigen Tarif, um Speicher hinzuzufügen.",
    description:
      "Why the Storage Edit button is disabled on a free database (bex-api POSTGRES_STORAGE_PLAN_UNSUPPORTED)",
  },
  "databases.storageAlmostFull": {
    message:
      "Deine Datenbank ist fast voll. Das kann zu Fehlern beim Lesen oder Schreiben deiner Datenbank führen und automatische Backups verhindern. Gib Speicherplatz in deiner Datenbank frei oder {action}.",
    description:
      "Render's almost-full warning at the top of a Postgres database's Info, Plan and Recovery pages (w3/m656): more than 80% of its storage used and less than 5 GiB left. {action} is databases.storageAlmostFullIncrease or databases.storageAlmostFullUpgrade, drawn as a link to the Plan page, or databases.storageAlmostFullSupport",
  },
  "databases.storageAlmostFullIncrease": {
    message: "vergrößere ihren Speicher",
    description:
      "The almost-full warning's ending for storage that can grow: a link to the Plan page's Storage control (Render's copy)",
  },
  "databases.storageAlmostFullUpgrade": {
    message: "wechsle zu einem Compute-Plan mit mehr Speicher",
    description:
      "The almost-full warning's ending for a plan whose storage is fixed: a link to the Plan page's plan picker (Render's copy)",
  },
  "databases.storageAlmostFullSupport": {
    message:
      "wende dich an den Support, um einen individuellen Compute-Plan zu erstellen",
    description:
      "The almost-full warning's ending for storage already at its plan's maximum, which no plan grows further (Render's copy); plain text",
  },
  "databases.storageEdit": {
    message: "Bearbeiten",
    description:
      "Plan section Storage row button that opens the storage dialog",
  },
  "databases.storageEditAria": {
    message: "Speicher bearbeiten",
    description: "Accessible name of the Storage row's Edit button",
  },
  "databases.storageEditTitle": {
    message: "Speicher für {name} bearbeiten",
    description: "Storage dialog title; {name} is the database",
  },
  "databases.storageEditBody": {
    message:
      "Gib ein beliebiges Vielfaches von {step} GB ein, das größer als die aktuellen {current} GB ist. Speicher kann nach einer Vergrößerung nicht mehr verkleinert werden.",
    description:
      "Storage dialog description (Render: storage only grows, in 5 GB steps)",
  },
  "databases.storageEditLabel": {
    message: "Speicher (GB)",
    description: "Storage dialog size field label",
  },
  "databases.storageEditStep": {
    message: "Gib ein ganzzahliges Vielfaches von {step} GB ein.",
    description: "Storage dialog validation: not a whole multiple of 5 GB",
  },
  "databases.storageEditNotGreater": {
    message:
      "Gib mehr als die aktuellen {current} GB ein. Speicher kann nicht verkleinert werden.",
    description:
      "Storage dialog validation: the size is not above the current storage",
  },
  "databases.storageEditTooLarge": {
    message: "{name} erlaubt bis zu {max} GB Speicher.",
    description:
      "Storage dialog validation: the size is above the plan's maximum; {name} is the plan",
  },
  "databases.storageEditSave": {
    message: "Speichern",
    description: "Storage dialog submit button",
  },
  "databases.storageEditSuccess": {
    message: "Speicher wird auf {size} GB erhöht…",
    description: "Toast after a storage increase is accepted",
  },
  "databases.storageEditError": {
    message:
      "Der Speicher konnte nicht aktualisiert werden. Bitte versuche es erneut.",
    description:
      "Storage dialog fallback error when the server gives no reason",
  },
  "databases.nameEdit": {
    message: "Datenbanknamen bearbeiten",
    description: "Accessible label for the database-name edit-in-place pencil",
  },
  "databases.nameDescription": {
    message:
      "Ändere den Anzeigenamen. Die Datenbank-ID und alle Verbindungsdetails bleiben gleich.",
    description: "Database detail rename row description",
  },
  "databases.nameSuccess": {
    message: "Datenbank in {name} umbenannt.",
    description: "Toast after a database rename succeeds",
  },
  "databases.nameError": {
    message:
      "Die Datenbank konnte nicht umbenannt werden. Bitte versuche es erneut.",
    description: "Toast when a database rename fails unexpectedly",
  },
  "databases.nameConflict": {
    message:
      "In diesem Workspace gibt es bereits eine Datenbank mit diesem Namen.",
    description: "Toast when a database rename collides with another name",
  },
  "databases.nameInvalid": {
    message:
      "Gib einen Namen mit höchstens 100 Zeichen ein, ohne Zeilenumbrüche oder Steuerzeichen.",
    description: "Database rename validation message",
  },
  "databases.insightsTitle": {
    message: "Insights",
    description: "Database insights panel card title",
  },
  "databases.insightsDescription": {
    message:
      "Live-Einblick in Prozesse, Abfrageleistung, Speicher und Konfiguration.",
    description: "Database insights panel card description",
  },
  "databases.insightsRefresh": {
    message: "Insights aktualisieren",
    description: "Accessible label for the insights refresh button",
  },
  "databases.insightsUnavailable": {
    message:
      "Nicht verfügbar – die Datenbank wird möglicherweise noch bereitgestellt.",
    description: "Insights sub-section error state",
  },
  "databases.insightsSizeTitle": {
    message: "Speicher",
    description: "Insights panel — database/table sizes sub-section heading",
  },
  "databases.insightsNoSizes": {
    message: "Noch keine Größendaten.",
    description: "Insights storage empty state",
  },
  "databases.insightsProcessesTitle": {
    message: "Aktive Prozesse",
    description: "Insights panel — pg_stat_activity sub-section heading",
  },
  "databases.insightsNoProcesses": {
    message: "Keine aktiven Prozesse.",
    description: "Insights processes empty state",
  },
  "databases.insightsTopQueriesTitle": {
    message: "Top-Abfragen",
    description: "Insights panel — pg_stat_statements sub-section heading",
  },
  "databases.insightsNoTopQueries": {
    message:
      "Noch keine Abfragen. Statements, die deine Anwendung und die SQL-Konsole ausführen, erscheinen hier.",
    description:
      "Insights top-queries empty state: the read succeeded and nothing the owner ran is in the list. A failed read, including a missing pg_stat_statements, shows insightsUnavailable instead, so this text must not blame the extension",
  },
  "databases.insightsQueryHidden": {
    message: "Ausgeblendet",
    description:
      "Insights — query cell for a row whose SQL PostgreSQL hides from the reading role",
  },
  "databases.insightsMaskedNote": {
    message:
      "Einige Zeilen gehören zu anderen Datenbankrollen, daher blendet PostgreSQL ihr SQL in dieser Ansicht aus. Alles andere in der Zeile ist echt.",
    description:
      "Insights — explanation shown when at least one process or top-query row has masked SQL",
  },
  "databases.insightsTableScansTitle": {
    message: "Tabellen-Scans",
    description: "Insights panel — pg_stat_user_tables sub-section heading",
  },
  "databases.insightsNoTableScans": {
    message: "Noch keine Benutzertabellen mit Scan-Statistiken.",
    description: "Insights table-scans empty state",
  },
  "databases.insightsParamsDeclaredValue": {
    message: "Deklarierter Wert",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsObservedValue": {
    message: "Beobachtete Einstellung",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsObservedHint": {
    message:
      "Live-Einstellung; sie bestätigt nicht, dass der angeforderte Wert angewendet wurde.",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsNotObserved": {
    message:
      "Angeforderte Überschreibung nicht beobachtet. Sie ist möglicherweise ausstehend oder ungültig.",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsObservationUnavailable": {
    message: "Diese Überschreibung kann gerade nicht überprüft werden.",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsTitle": {
    message: "Parameter-Überschreibungen",
    description:
      "Insights panel — heading for the parameters this database declares (read-only)",
  },
  "databases.insightsParamsDescription": {
    message:
      "Einstellungen, die diese Datenbank deklariert hat, bevor Parameter-Überschreibungen eingestellt wurden. Sie gelten weiterhin, können hier aber nicht mehr geändert werden; lege Sitzungs-Standardwerte mit ALTER DATABASE … SET in der SQL-Konsole fest.",
    description:
      "Explains that the declared parameters are read-only legacy settings that still apply, and how to set session defaults instead",
  },
  "databases.insightsEffectiveParamsTitle": {
    message: "Nicht-Standard-Parameter",
    description: "Insights panel — pg_settings sub-section heading",
  },
  "databases.insightsEffectiveParamsDescription": {
    message:
      "Jede aktuell wirksame Einstellung, die nicht auf ihrem Standardwert steht, einschließlich der von der Plattform verwalteten. Schreibgeschützt.",
    description:
      "Explains that the pg_settings table is observed configuration, mostly platform-owned, and cannot be edited",
  },
  "databases.insightsNoParams": {
    message: "Alle Parameter stehen auf ihren Standardwerten.",
    description: "Insights parameter-overrides empty state",
  },
  "databases.insightsColTable": {
    message: "Tabelle",
    description: "Insights table column header (table name)",
  },
  "databases.insightsColRelation": {
    message: "Tabelle oder Index",
    description:
      "Insights sizes column header (a table's or an index's name; tables and indexes are listed together)",
  },
  "databases.insightsIndexOn": {
    message: "Index auf {table}",
    description:
      "Insights sizes: suffix naming the table an index row belongs to; {table} is the quoted schema-qualified table name",
  },
  "databases.insightsColSize": {
    message: "Größe",
    description: "Insights table column header (relation size)",
  },
  "databases.insightsColUser": {
    message: "Benutzer",
    description: "Insights processes column header (pg role)",
  },
  "databases.insightsColState": {
    message: "Zustand",
    description: "Insights processes column header (active/idle/…)",
  },
  "databases.insightsColDuration": {
    message: "Dauer",
    description: "Insights processes column header (seconds since query start)",
  },
  "databases.insightsColQuery": {
    message: "Abfrage",
    description: "Insights column header (SQL text, truncated)",
  },
  "databases.insightsColCalls": {
    message: "Aufrufe",
    description: "Insights top-queries column header (invocation count)",
  },
  "databases.insightsColTotalTime": {
    message: "Gesamtzeit",
    description: "Insights top-queries column header (total exec time)",
  },
  "databases.insightsColMeanTime": {
    message: "Mittlere Zeit",
    description: "Insights top-queries column header (mean exec time per call)",
  },
  "databases.insightsColSeqScans": {
    message: "Seq-Scans",
    description: "Insights table-scans column header (sequential scan count)",
  },
  "databases.insightsColIdxScans": {
    message: "Index-Scans",
    description: "Insights table-scans column header (index scan count)",
  },
  "databases.insightsColLiveRows": {
    message: "Live-Zeilen",
    description:
      "Insights table-scans column header (estimated live row count)",
  },
  "databases.insightsColParam": {
    message: "Parameter",
    description:
      "Insights parameter-overrides column header (pg_settings.name)",
  },
  "databases.insightsColSetting": {
    message: "Einstellung",
    description:
      "Insights parameter-overrides column header (pg_settings.setting)",
  },
  "databases.insightsColSource": {
    message: "Quelle",
    description:
      "Insights parameter-overrides column header (pg_settings.source)",
  },
  "databases.sqlTitle": {
    message: "SQL-Konsole",
    description: "Database detail SQL console card title",
  },
  "databases.sqlDescription": {
    message:
      "Führe ein einzelnes SQL-Statement für diese Datenbank aus. Ergebnisse sind auf 500 Zeilen begrenzt, und Abfragen laufen nach 10 Sekunden in ein Timeout.",
    description: "Database SQL console safety summary",
  },
  "databases.sqlEditorLabel": {
    message: "SQL-Abfrage",
    description: "Accessible label for the SQL editor",
  },
  "databases.sqlShortcut": {
    message: "Drücke Ctrl+Enter oder ⌘+Enter zum Ausführen",
    description: "SQL console keyboard shortcut hint",
  },
  "databases.sqlCreating": {
    message:
      "Diese Datenbank wird noch erstellt. SQL läuft, sobald sie verfügbar ist; deine Abfrage und der Verlauf bleiben erhalten.",
    description: "SQL console notice while the database is creating (w4/177)",
  },
  "databases.sqlUpgrading": {
    message:
      "Ein Offline-Upgrade von PostgreSQL läuft. SQL läuft wieder, sobald die Datenbank verfügbar ist; deine Abfrage und der Verlauf bleiben erhalten.",
    description: "SQL console notice while the database is upgrading (w4/177)",
  },
  "databases.sqlRestarting": {
    message:
      "Diese Datenbank wird neu gestartet. Warte, bis sie verfügbar ist, um SQL auszuführen; deine Abfrage und der Verlauf bleiben erhalten.",
    description: "SQL console notice while the database is restarting (w4/177)",
  },
  "databases.sqlSuspended": {
    message:
      "Diese Datenbank ist pausiert. Setze sie fort, um SQL auszuführen; deine Abfrage und der Verlauf bleiben erhalten.",
    description: "SQL console notice while the database is suspended",
  },
  "databases.sqlRun": {
    message: "Abfrage ausführen",
    description: "SQL console run button",
  },
  "databases.sqlRunning": {
    message: "Läuft…",
    description: "SQL console run button while a query is executing",
  },
  "databases.sqlUnknownError": {
    message:
      "Die Abfrage konnte nicht ausgeführt werden. Bitte versuche es erneut.",
    description: "SQL console fallback error",
  },
  "databases.sqlReturnedRows_one": {
    message: "{count} Zeile zurückgegeben",
    description: "SQL console SELECT result count (singular)",
  },
  "databases.sqlReturnedRows_other": {
    message: "{count} Zeilen zurückgegeben",
    description: "SQL console SELECT result count (plural)",
  },
  "databases.sqlAffectedRows_one": {
    message: "{count} Zeile betroffen",
    description: "SQL console write result count (singular)",
  },
  "databases.sqlAffectedRows_other": {
    message: "{count} Zeilen betroffen",
    description: "SQL console write result count (plural)",
  },
  "databases.sqlTruncated": {
    message: "Ergebnis auf {limit} Zeilen begrenzt",
    description: "SQL console backend row-cap notice",
  },
  "databases.sqlNull": {
    message: "NULL",
    description: "SQL console null cell value",
  },
  "databases.sqlNullLabel": {
    message: "SQL NULL (kein Wert)",
    description:
      "SQL console accessible meaning of a null cell, distinct from the literal text NULL",
  },
  "databases.sqlNoRows": {
    message: "Die Abfrage hat keine Zeilen zurückgegeben.",
    description: "SQL console empty result message",
  },
  "databases.sqlPrevious": {
    message: "Zurück",
    description: "SQL result pagination previous button",
  },
  "databases.sqlNext": {
    message: "Weiter",
    description: "SQL result pagination next button",
  },
  "databases.sqlPage": {
    message: "Seite {page} von {pages}",
    description: "SQL result pagination position",
  },
  "databases.sqlHistory": {
    message: "Abfrageverlauf",
    description: "SQL console session history heading",
  },
  "databases.sqlClearHistory": {
    message: "Leeren",
    description: "SQL console clear-history button",
  },
  "databases.sqlNoHistory": {
    message: "In dieser Browser-Sitzung ausgeführte Abfragen erscheinen hier.",
    description: "SQL console empty history message",
  },
  "databases.sqlConfirmTitle": {
    message: "Ein Statement ausführen, das Daten ändern kann?",
    description: "SQL console write confirmation title",
  },
  "databases.sqlConfirmDescription": {
    message:
      "Dieses Statement wird nicht als schreibgeschützt erkannt. Es kann Datenbankdaten ändern oder löschen und kann von bex nicht rückgängig gemacht werden.",
    description: "SQL console write confirmation warning",
  },
  "databases.sqlConfirmCancel": {
    message: "Abbrechen",
    description: "SQL console write confirmation cancel button",
  },
  "databases.sqlConfirmRun": {
    message: "Statement ausführen",
    description: "SQL console confirmed write action",
  },
  // --- Sidebar and page headings (w3/m641) ---
  "databases.navInfo": {
    message: "Info",
    description:
      "Postgres sidebar entry and page heading: the database's Info page (general details, connections, access, SQL console)",
  },
  "databases.navLogs": {
    message: "Logs",
    description:
      "Postgres sidebar entry and page heading: the database's Logs page",
  },
  "databases.navMetrics": {
    message: "Metriken",
    description:
      "Postgres sidebar entry and page heading: the database's Metrics page",
  },
  "databases.navPlan": {
    message: "Plan",
    description:
      "Postgres sidebar entry and page heading: the database's Plan page (compute plan, storage, high availability)",
  },
  "databases.navApps": {
    message: "Apps",
    description:
      "Postgres sidebar entry and page heading: the database's Apps page (pgAdmin, PgHero)",
  },
  "databases.navRecovery": {
    message: "Wiederherstellung",
    description:
      "Postgres sidebar entry and page heading: the database's Recovery page (point-in-time recovery, exports)",
  },
  "databases.logsRangeLabel": {
    message: "Zeitraum",
    description: "Managed Postgres log range filter label",
  },
  "databases.logsInstanceLabel": {
    message: "Datenbankinstanz",
    description: "Managed Postgres log instance filter label",
  },
  "databases.logsAllInstances": {
    message: "Alle Instanzen",
    description: "Managed Postgres log instance filter all option",
  },
  "databases.logsSearchPlaceholder": {
    message: "Datenbank-Logs durchsuchen",
    description: "Managed Postgres log search placeholder",
  },
  "databases.logsLoading": {
    message: "Datenbank-Logs werden geladen…",
    description: "Managed Postgres logs loading state",
  },
  "databases.logsEmptyTitle": {
    message: "Noch keine Datenbank-Logs",
    description: "Managed Postgres logs empty-state title",
  },
  "databases.logsEmptyBody": {
    message: "Postgres-Ausgaben in diesem Zeitraum erscheinen hier.",
    description: "Managed Postgres logs empty-state body",
  },
  "databases.logsUnavailableTitle": {
    message: "Datenbank-Logs sind nicht konfiguriert",
    description: "Managed Postgres logs unavailable-state title",
  },
  "databases.logsUnavailableBody": {
    message:
      "In dieser Installation ist weder ein dauerhafter Log-Verlauf noch eine Live-CNPG-Pod-Log-Quelle konfiguriert.",
    description: "Managed Postgres logs unavailable-state body",
  },
  "databases.logsUnauthorizedTitle": {
    message: "Du kannst diese Logs nicht ansehen",
    description: "Managed Postgres logs unauthorized-state title",
  },
  "databases.logsUnauthorizedBody": {
    message:
      "Datenbank-Logs erfordern Contributor-Zugriff in diesem Workspace.",
    description: "Managed Postgres logs unauthorized-state body",
  },
  "databases.logsErrorTitle": {
    message: "Datenbank-Logs konnten nicht geladen werden",
    description: "Managed Postgres logs generic error title",
  },
  "databases.fieldNameDescription": {
    message: "Ein eindeutiger Name für Ihre Postgres-Instanz.",
    description: "Create-Postgres Name row description",
  },
  "databases.fieldDatabaseNameDescription": {
    message:
      "Der dbname in Postgres, nur beim Erstellen festlegbar. Leer lassen, um einen zu erzeugen.",
    description:
      "Create-Postgres Database row description: the physical dbname, create-time only",
  },
  "databases.fieldDatabaseUserDescription": {
    message:
      "Der Postgres-Benutzer, dem die Datenbank gehört, nur beim Erstellen festlegbar. Leer lassen, um einen zu erzeugen.",
    description:
      "Create-Postgres User row description: the physical owner role, create-time only",
  },
  "databases.fieldPlanDescription": {
    message:
      "Legen Sie RAM und CPU Ihrer Datenbank fest. Die Compute-Größe können Sie später ändern.",
    description: "Create-Postgres plan row description (Render's Compute)",
  },
  "databases.fieldDiskDescription": {
    message:
      "Die Kapazität Ihrer Datenbank in GB. Sie können den Speicher später vergrößern, aber nicht verkleinern.",
    description:
      "Create-Postgres Storage row description (Render: storage only grows)",
  },
  "databases.fieldDiskUnit": {
    message: "GB",
    description: "Unit after the create-Postgres storage input",
  },
  "databases.fieldVersionDescription": {
    message: "Die PostgreSQL-Hauptversion, die die Datenbank ausführt.",
    description: "Create-Postgres version row description",
  },
  // --- Apps: admin apps (Render's Postgres Apps page, w4/m551) ---
  "databases.adminAppsTitle": {
    message: "Apps",
    description:
      "Database page Apps section title and navigation label (Render's Postgres Apps page)",
  },
  "databases.adminAppsDescription": {
    message:
      "Stelle eine Admin-App als Webservice in der Region und Umgebung dieser Datenbank bereit, über das private Netzwerk mit ihr verbunden.",
    description: "Database page Apps section description",
  },
  "databases.adminAppsErrorTitle": {
    message: "Apps konnten nicht geladen werden",
    description: "Database Apps section load error title",
  },
  "databases.adminAppsErrorBody": {
    message:
      "Beim Laden der Apps dieser Datenbank ist ein Fehler aufgetreten. Versuche es gleich noch einmal.",
    description: "Database Apps section load error body",
  },
  "databases.adminAppPgAdminDescription": {
    message:
      "Allgemeine PostgreSQL-Administration. Verwalte Schemas, Tabellen und Indizes. Führe einmalige Abfragen aus und sieh dir ihre Abfragepläne an.",
    description: "pgAdmin card description (Render's copy)",
  },
  "databases.adminAppPgHeroDescription": {
    message:
      "Ein Performance-Dashboard für deine Datenbank. Überwache die Ressourcennutzung, analysiere aktive Verbindungen und untersuche aktuelle Abfragen, um Engpässe zu finden.",
    description: "PgHero card description (Render's copy)",
  },
  "databases.adminAppDocs": {
    message: "Dokumentation lesen",
    description: "Admin app card link to the app's documentation",
  },
  "databases.adminAppDeploy": {
    message: "App bereitstellen",
    description: "Admin app card button that opens the deploy dialog",
  },
  "databases.adminAppOpen": {
    message: "App öffnen",
    description:
      "Deployed admin app card button that opens the app in a new tab",
  },
  "databases.adminAppManage": {
    message: "Verwalten",
    description: "Deployed admin app card link to its service page",
  },
  "databases.adminAppDialogTitle": {
    message: "{app}-App erstellen",
    description: "Admin app deploy dialog title; {app} is pgAdmin or PgHero",
  },
  "databases.adminAppDialogDescription": {
    message:
      "Dadurch wird ein neuer Webservice in derselben Region und Umgebung wie deine Datenbank bereitgestellt. Du kannst ihn wie jeden anderen Webservice konfigurieren.",
    description: "Admin app deploy dialog body",
  },
  "databases.adminAppAuthentication": {
    message: "Authentifizierung",
    description: "Admin app deploy dialog login section heading",
  },
  "databases.adminAppAuthenticationHint": {
    message:
      "Wir haben automatisch einen Benutzernamen und ein Passwort für deine {app}-Instanz erzeugt. Du kannst sie unten anpassen.",
    description:
      "Admin app deploy dialog login section text; {app} is the app name",
  },
  "databases.adminAppUsername": {
    message: "Benutzername",
    description: "Admin app deploy dialog username field label",
  },
  "databases.adminAppPassword": {
    message: "Passwort",
    description: "Admin app deploy dialog password field label",
  },
  "databases.adminAppShow": {
    message: "{field} anzeigen",
    description:
      "Accessible label of the button that unmasks a login field; {field} is its label",
  },
  "databases.adminAppHide": {
    message: "{field} ausblenden",
    description:
      "Accessible label of the button that masks a login field; {field} is its label",
  },
  "databases.adminAppMonthlyCharges": {
    message: "Monatliche Kosten",
    description: "Admin app deploy dialog billing section heading",
  },
  "databases.adminAppMonthlyChargesHint": {
    message:
      "Webservice-Instanzen werden sekundengenau anteilig abgerechnet. Die folgenden Kosten betreffen nur {app}.",
    description:
      "Admin app deploy dialog billing section text; {app} is the app name",
  },
  "databases.adminAppBillingStarter": {
    message: "Webservice – Starter-Instanz",
    description:
      "Admin app deploy dialog billing line for a Starter web service",
  },
  "databases.adminAppBillingFree": {
    message: "Webservice – Free-Instanz",
    description: "Admin app deploy dialog billing line for a Free web service",
  },
  "databases.adminAppBillingDisk": {
    message: "Disk – {size} GB",
    description:
      "Admin app deploy dialog billing line for its disk; {size} is in GB",
  },
  "databases.adminAppBillingPrice": {
    message: "${usd} / Monat",
    description:
      "Admin app deploy dialog billing line price; {usd} is a dollar amount",
  },
  "databases.adminAppDeploySubmit": {
    message: "{app} bereitstellen",
    description: "Admin app deploy dialog submit button; {app} is the app name",
  },
  "databases.adminAppDeployed": {
    message: "{app} wurde als {service} bereitgestellt.",
    description:
      "Toast after an admin app deploys; {service} is its new service's name",
  },
  "databases.adminAppDeployError": {
    message: "Die App konnte nicht erstellt werden. Erneut versuchen?",
    description: "Admin app deploy dialog generic failure",
  },
  "databases.adminAppPaymentRequired": {
    message:
      "Füge diesem Workspace eine Zahlungsmethode hinzu, um {app} bereitzustellen.",
    description:
      "Admin app deploy dialog message when billing onboarding was closed without a payment method",
  },
  "databases.adminAppErrorTitle": {
    message: "Fehler",
    description: "Admin app deploy dialog error title",
  },
  "databases.connectInternalTitle": {
    message: "Interne Datenbank-URL",
    description:
      "Header Connect menu, Internal tab: the private-network URL's title (Render's copy, w3/m675)",
  },
  "databases.connectExternalTitle": {
    message: "Externe Datenbank-URL",
    description:
      "Header Connect menu, External tab: the public URL's title (Render's copy, w3/m675)",
  },
  "databases.connectCliTitle": {
    message: "PSQL-Befehl",
    description:
      "Header Connect menu, External tab: the `bex psql <id>` command's title (Render's copy, w3/m675)",
  },
};

export default deDatabases;
