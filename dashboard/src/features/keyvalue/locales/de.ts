import type { TranslationEntry } from "@/i18n";

const deKeyValue: Record<string, TranslationEntry> = {
  "keyvalue.resourceType": {
    message: "Key Value",
    description: "Key Value resource type used in document titles",
  },
  "keyvalue.statTotal": {
    message: "Key-Value-Stores gesamt",
    description: "Key Value page stat card label",
  },
  "keyvalue.statAvailable": {
    message: "Verfügbar",
    description: "Key Value page stat card label (healthy stores)",
  },
  "keyvalue.statCreating": {
    message: "Wird erstellt",
    description: "Key Value page stat card label (provisioning stores)",
  },
  "keyvalue.cardTitle": {
    message: "Key Value",
    description: "Key Value table card title",
  },
  "keyvalue.colName": {
    message: "Name",
    description: "Key Value table column header",
  },
  "keyvalue.colStatus": {
    message: "Status",
    description: "Key Value table column header",
  },
  "keyvalue.colPlan": {
    message: "Compute-Plan",
    description: "Key Value table column header (plan / tier)",
  },
  "keyvalue.colVersion": {
    message: "Version",
    description: "Key Value table column header (Valkey version)",
  },
  "keyvalue.colCreated": {
    message: "Erstellt",
    description: "Key Value table column header (relative age from createdAt)",
  },
  "keyvalue.colActions": {
    message: "Aktionen",
    description: "Key Value table actions column header (screen-reader only)",
  },
  "keyvalue.statusAvailable": {
    message: "Verfügbar",
    description: "Key Value status badge (Valkey instance healthy)",
  },
  "keyvalue.statusCreating": {
    message: "Wird erstellt",
    description: "Key Value status badge (provisioning)",
  },
  "keyvalue.statusRestarting": {
    message: "Wird neu gestartet",
    description:
      "Key Value status badge (Render's config_restart: an established store restarting after a config change, restart, or resume — not a first provision)",
  },
  "keyvalue.statusUnavailable": {
    message: "Nicht verfügbar",
    description: "Key Value status badge (provisioning failed)",
  },
  "keyvalue.statusSuspended": {
    message: "Pausiert",
    description:
      "Key Value status badge (hibernated; suspension wins over the status enum)",
  },
  "keyvalue.statusUnknown": {
    message: "Unbekannt",
    description: "Key Value status badge for an unrecognized status",
  },
  "keyvalue.statusReasonStatefulSetReadFailed": {
    message: "Der Workload-Status des Key Value konnte nicht geprüft werden.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StatefulSetReadFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStatefulSetFailed": {
    message:
      "Der Workload des Key Value konnte nicht aktualisiert werden, daher wurde die letzte Änderung nicht angewendet.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StatefulSetFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPVCReadFailed": {
    message: "Der Status des Speichervolumes konnte nicht geprüft werden.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PVCReadFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPVCResizeFailed": {
    message: "Das Speichervolume des Key Value konnte nicht vergrößert werden.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PVCResizeFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonSecretFailed": {
    message:
      "Die Verbindungsdetails des Key Value konnten nicht gespeichert werden.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's SecretFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonCredentialSecretFailed": {
    message: "Die Zugangsdaten des Key Value konnten nicht gespeichert werden.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's CredentialSecretFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonSecretRecreateFailed": {
    message:
      "Die Verbindungsdetails des Key Value konnten nicht neu aufgebaut werden.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's SecretRecreateFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonServiceFailed": {
    message:
      "Die interne Adresse des Key Value konnte nicht eingerichtet werden.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's ServiceFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonNetworkPolicyFailed": {
    message:
      "Die Netzwerkrichtlinie des Key Value konnte nicht angewendet werden.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's NetworkPolicyFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonBackupNetworkPolicyFailed": {
    message:
      "Die Netzwerkrichtlinie des Backup-Jobs konnte nicht angewendet werden.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's BackupNetworkPolicyFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonBackupCronJobFailed": {
    message:
      "Der Zeitplan für das tägliche Backup konnte nicht aktualisiert werden.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's BackupCronJobFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonTLSIssuerMissing": {
    message:
      "Öffentlicher Zugriff kann nicht aktiviert werden: Es ist kein Zertifikatsaussteller konfiguriert.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's TLSIssuerMissing Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonCertificateFailed": {
    message:
      "Das Zertifikat für den öffentlichen Zugriff konnte nicht ausgestellt werden.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's CertificateFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonCertificateCleanupFailed": {
    message:
      "Das Zertifikat für den öffentlichen Zugriff konnte nicht entfernt werden.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's CertificateCleanupFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageClassMissing": {
    message:
      "Das Speichervolume hat keine Storage-Klasse und kann daher nicht wachsen.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageClassMissing Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageClassNotFound": {
    message:
      "Die Storage-Klasse des Speichervolumes wurde nicht gefunden, daher kann es nicht wachsen.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageClassNotFound Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageClassNotExpandable": {
    message: "Die Storage-Klasse des Speichervolumes erlaubt kein Wachstum.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageClassNotExpandable Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageBlockedByQuota": {
    message:
      "Das Speichervolume kann nicht wachsen: Das Speicherkontingent des Workspace hat dafür keinen Platz.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageBlockedByQuota Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPersistenceTransitionFailed": {
    message:
      "Die gespeicherten Daten konnten nicht für den Start vorbereitet werden; die Daten bleiben erhalten, und es wird erneut versucht.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PersistenceTransitionFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPersistenceSourceUnknown": {
    message:
      "Der Persistenzmodus der gespeicherten Daten ist unbekannt, daher kann der Key Value erst starten, wenn ein Plattformbetreiber ihn bestätigt. Es wurden keine Daten verändert.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PersistenceSourceUnknown Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageShrinkRejected": {
    message:
      "Key-Value-Speicher kann nur wachsen: Das Volume kann nicht kleiner als seine aktuelle Größe werden.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageShrinkRejected Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.errorTitle": {
    message: "Key-Value-Stores konnten nicht geladen werden",
    description: "Key Value list error card title",
  },
  "keyvalue.errorBody": {
    message:
      "Die Anfrage an bex-api ist fehlgeschlagen. Prüfe deine Verbindung und versuche es erneut.",
    description: "Key Value list error card body",
  },
  "keyvalue.emptyTitle": {
    message: "Noch keine Key-Value-Stores",
    description: "Key Value list empty state title",
  },
  "keyvalue.emptyBody": {
    message:
      "Erstelle deinen ersten verwalteten Key-Value-Store, dann erscheint er hier.",
    description: "Key Value list empty state body",
  },
  "keyvalue.actionsMenu": {
    message: "Aktionen für {name}",
    description: "Accessible label for the per-row actions trigger",
  },
  "keyvalue.actionDelete": {
    message: "Löschen",
    description: "Row action: permanently delete the Key Value store",
  },
  "keyvalue.deleteConfirmTitle": {
    message: "Key-Value-Instanz löschen",
    description: "Delete-confirmation dialog title",
  },
  "keyvalue.deleteConfirmBody": {
    message:
      "Diese Aktion kann nicht rückgängig gemacht werden. Möchtest du diese Key-Value-Instanz wirklich löschen?",
    description:
      "Delete-confirmation dialog body (verbatim Render copy, docs/render-artifacts/key-value.md)",
  },
  "keyvalue.deleteConfirmPrompt": {
    message: "Gib unten {phrase} ein, um zu bestätigen.",
    description:
      "Body prompt naming Render's exact sudo delete phrase (rendered bold by SudoCommandField)",
  },
  "keyvalue.deleteCancel": {
    message: "Abbrechen",
    description: "Delete-confirmation dialog cancel button",
  },
  "keyvalue.deleteConfirm": {
    message: "Key-Value-Instanz löschen",
    description: "Delete-confirmation dialog confirm button",
  },
  "keyvalue.deleteSuccess": {
    message: "{name} wird gelöscht…",
    description: "Toast after a delete request is accepted",
  },
  "keyvalue.deleteError": {
    message: "{name} konnte nicht gelöscht werden. Bitte versuche es erneut.",
    description: "Toast when a delete request fails",
  },
  "keyvalue.createButton": {
    message: "Neuer Key Value",
    description: "Button that navigates to the create-Key-Value page",
  },
  "keyvalue.createTitle": {
    message: "Neuer Key Value",
    description: "Create-Key-Value page title",
  },
  "keyvalue.createDescription": {
    message: "Stelle eine verwaltete Valkey-Instanz (Redis-kompatibel) bereit.",
    description: "Create-Key-Value page description",
  },
  "keyvalue.fieldName": {
    message: "Name",
    description: "Create-Key-Value form field label (store name)",
  },
  "keyvalue.fieldNamePlaceholder": {
    message: "example-key-value-name",
    description: "Create-Key-Value name input placeholder",
  },
  "keyvalue.fieldNameError": {
    message:
      "Gib einen Namen mit höchstens 100 Zeichen ein, ohne Zeilenumbrüche oder Steuerzeichen.",
    description: "Create-Key-Value name validation message",
  },
  "keyvalue.fieldPlan": {
    message: "Compute-Plan",
    description: "Create-Key-Value form field label (plan / tier)",
  },
  "keyvalue.fieldMaxmemoryPolicy": {
    message: "Maxmemory-Policy",
    description: "Create-Key-Value form field label (eviction policy)",
  },
  "keyvalue.fieldMaxmemoryRecommended": {
    message: "(empfohlen für Caches)",
    description:
      "Suffix on the recommended maxmemory policy option (allkeys-lru)",
  },
  "keyvalue.fieldMaxmemoryPolicyHint": {
    message:
      "Wie Keys verdrängt werden, sobald der Store sein Speicherlimit erreicht.",
    description: "Create-Key-Value maxmemory policy helper text",
  },
  "keyvalue.fieldPersistenceMode": {
    message: "Persistenzmodus",
    description: "Create-Key-Value form field label (persistence)",
  },
  "keyvalue.fieldPersistenceModeHint": {
    message:
      "Wie Daten auf die Disk geschrieben werden, damit sie einen Neustart überstehen.",
    description: "Create-Key-Value persistence mode helper text",
  },
  "keyvalue.persistenceJournalSnapshot": {
    message: "Journal + Snapshot",
    description: "Persistence mode option: AOF journal plus RDB snapshots",
  },
  "keyvalue.persistenceSnapshot": {
    message: "Nur Snapshot",
    description: "Persistence mode option: RDB snapshots only",
  },
  "keyvalue.persistenceOff": {
    message: "Aus",
    description: "Persistence mode option: no persistence",
  },
  "keyvalue.fieldPublic": {
    message: "Öffentlicher Zugriff",
    description: "Create-Key-Value form field label (external endpoint toggle)",
  },
  "keyvalue.fieldPublicHint": {
    message:
      "Verbindungen von außerhalb Ihres privaten Netzwerks über TLS erlauben.",
    description: "Create-Key-Value public toggle helper text",
  },
  "keyvalue.createCancel": {
    message: "Abbrechen",
    description: "Create-Key-Value page cancel button",
  },
  "keyvalue.createSubmit": {
    message: "Key-Value-Instanz erstellen",
    description: "Create-Key-Value page submit button",
  },
  "keyvalue.createSuccess": {
    message: "{name} wird erstellt…",
    description:
      "Toast after a create request is accepted (provisioning is async)",
  },
  "keyvalue.createError": {
    message: "{name} konnte nicht erstellt werden. Bitte versuche es erneut.",
    description: "Toast when a create request fails",
  },
  "keyvalue.capLimitTitle": {
    message: "Key-Value-Limit erreicht",
    description:
      "Alert title when the workspace's KV creation cap is hit (w7/m9)",
  },
  "keyvalue.capLimitUpgrade": {
    message: "Tarif upgraden",
    description: "Upgrade CTA button inside the KV cap-limit Alert (w7/m9)",
  },
  "keyvalue.metaTitle": {
    message: "Details",
    description: "Key Value detail metadata card title",
  },
  "keyvalue.metaId": {
    message: "ID",
    description: "Key Value detail metadata row label (immutable red- id)",
  },
  "keyvalue.metaStatus": {
    message: "Status",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaPlan": {
    message: "Compute-Plan",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaConnectionLimit": {
    message: "Verbindungslimit",
    description:
      "Key Value detail Details card row label: the plan's client connection limit, as Render's Key Value Instance card shows it",
  },
  "keyvalue.metaVersion": {
    message: "Version",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaPublic": {
    message: "Öffentlicher Zugriff",
    description: "Key Value detail metadata row label (external endpoint)",
  },
  "keyvalue.metaExternalHost": {
    message: "Externer Host",
    description: "Key Value detail metadata row label (SNI hostname)",
  },
  "keyvalue.metaRegion": {
    message: "Region",
    description: "Key Value detail metadata row label (platform placement)",
  },
  "keyvalue.metaCreated": {
    message: "Erstellt",
    description: "Key Value detail metadata row label (relative age)",
  },
  "keyvalue.yes": {
    message: "Ja",
    description: "Metadata value for a true boolean field",
  },
  "keyvalue.publicPending": {
    message: "Ja (externer Endpunkt wird geöffnet: Zertifikat ausstehend)",
    description:
      "Details value for Public access while the store's certificate is pending, so it does not read as reachable (w4/m357)",
  },
  "keyvalue.publicFailed": {
    message:
      "Ja (noch nicht erreichbar: Zertifikatsausstellung fehlgeschlagen)",
    description:
      "Details value for Public access when the store's certificate failed to issue (w4/m357)",
  },
  "keyvalue.no": {
    message: "Nein",
    description: "Metadata value for a false boolean field",
  },
  "keyvalue.connTitle": {
    message: "Verbindungen",
    description: "Connection-info panel card title",
  },
  "keyvalue.connDescription": {
    message:
      "Verbindungs-Strings für diesen Key-Value-Store. Werden nur auf Anfrage angezeigt – nie automatisch.",
    description: "Connection-info panel card description",
  },
  "keyvalue.internalAuthOff": {
    message: "Interne Verbindungen erfordern kein Passwort.",
    description: "Connections panel: internal auth off",
  },
  "keyvalue.internalAuthOn": {
    message: "Interne Verbindungen erfordern das Passwort.",
    description: "Connections panel: internal auth on",
  },
  "keyvalue.internalAuthEnable": {
    message: "Interne Authentifizierung aktivieren",
    description: "Button and confirm label (Render's wording)",
  },
  "keyvalue.internalAuthConfirmTitle": {
    message: "Interne Authentifizierung aktivieren?",
    description: "Confirm dialog title",
  },
  "keyvalue.internalAuthConfirmBody": {
    message:
      "Jede interne Verbindung benötigt das Passwort. Bestehende Verbindungen, die die nicht authentifizierte URL verwenden, brechen ab, und verknüpfte Services übernehmen die neue URL bei ihrem nächsten Deployment; aktualisiere sie auf die URL aus den Verbindungsinfos. Das kann nicht rückgängig gemacht werden.",
    description: "Confirm dialog body",
  },
  "keyvalue.internalAuthEnabled": {
    message: "Interne Authentifizierung für {name} aktiviert.",
    description: "Toast after enabling",
  },
  "keyvalue.internalAuthError": {
    message:
      "Die interne Authentifizierung konnte nicht aktiviert werden. Bitte versuche es erneut.",
    description: "Toast when enabling fails",
  },
  "keyvalue.connReveal": {
    message: "Verbindungsinfos anzeigen",
    description: "Button that fetches the connection info on demand",
  },
  "keyvalue.connHide": {
    message: "Verbindungsinfos ausblenden",
    description: "Button that clears the revealed connection info",
  },
  "keyvalue.connInternal": {
    message: "Interne Key-Value-URL",
    description: "Connection-info field label (in-cluster redis:// URL)",
  },
  "keyvalue.connExternal": {
    message: "Externe Key-Value-URL",
    description: "Connection-info field label (public rediss:// URL)",
  },
  "keyvalue.connExternalPending": {
    message:
      "Der externe Zugriff wird aktiviert: Ein öffentlich vertrauenswürdiges Zertifikat wird ausgestellt (meist wenige Minuten). Die externe URL und der CLI-Befehl verbinden, sobald der Store TLS-Verbindungen akzeptiert; bis dahin ist er nur von deinen Services aus erreichbar, mit der internen URL. Blende sie aus und wieder ein, um zu aktualisieren.",
    description:
      "Shown under the external URL while a public store's TLS listener is still opening (w9/m521)",
  },
  "keyvalue.connExternalFailed": {
    message:
      "Die externe URL und der CLI-Befehl verbinden noch nicht: Die Ausstellung des öffentlich vertrauenswürdigen Zertifikats dieses Stores ist fehlgeschlagen, und bex versucht es automatisch erneut. Bis es gelingt, ist der Store nur von deinen Services aus erreichbar, mit der internen URL.",
    description:
      "Shown under the external URL when a public store's certificate failed to issue (w4/m357, w9/m521)",
  },
  "keyvalue.connExternalUnavailable": {
    message:
      "Nicht öffentlich. Füge unter Netzwerk eine eingehende IP-Regel hinzu, um den externen Zugriff zu aktivieren.",
    description:
      "Shown instead of the external URL and the CLI command when the store isn't public (w9/134)",
  },
  "keyvalue.connCli": {
    message: "Valkey-CLI-Befehl",
    description:
      "Connection-info field label (ready-to-run valkey-cli command)",
  },
  "keyvalue.connErrorTitle": {
    message: "Verbindungsinfos konnten nicht geladen werden",
    description: "Connection-info panel error title",
  },
  "keyvalue.connErrorBody": {
    message:
      "Der Store wird möglicherweise noch bereitgestellt, oder du hast keine Berechtigung, seine Zugangsdaten einzusehen.",
    description: "Connection-info panel error body",
  },
  "keyvalue.copied": {
    message: "In die Zwischenablage kopiert",
    description: "Toast after copying a connection field",
  },
  "keyvalue.copyError": {
    message: "Kopieren in die Zwischenablage fehlgeschlagen",
    description: "Toast when clipboard copy fails",
  },
  "keyvalue.dangerDelete": {
    message: "Key-Value-Instanz löschen",
    description: "Discoverable delete action at the bottom of Key Value Info",
  },
  "keyvalue.actionSuspend": {
    message: "Key-Value-Instanz pausieren",
    description: "Suspend action button label",
  },
  "keyvalue.actionResume": {
    message: "Key-Value-Instanz fortsetzen",
    description: "Resume action button label",
  },
  "keyvalue.confirmSuspendTitle": {
    message: "Key-Value-Instanz pausieren",
    description: "Suspend-confirmation dialog title",
  },
  "keyvalue.confirmSuspendBody": {
    message: "Möchtest du diese Key-Value-Instanz wirklich pausieren?",
    description: "Suspend-confirmation dialog description",
  },
  "keyvalue.confirmSuspendDetail": {
    message:
      "Bitte bestätige, dass du {name} pausieren möchtest. Aktive Verbindungen werden getrennt, und du kannst die Instanz jederzeit fortsetzen. Die Datenaufbewahrung folgt dem Persistenzmodus: Bei „Aus“ gehen alle Daten verloren; bei „Nur Snapshot“ bleiben die Daten bis zum letzten Snapshot erhalten; bei „Journal + Snapshot“ bleiben Schreibvorgänge bis etwa zur letzten Sekunde erhalten.",
    description: "Suspend-confirmation consequences and named instance",
  },
  "keyvalue.confirmSuspendPrompt": {
    message: "Gib unten {phrase} ein, um zu bestätigen.",
    description:
      "Body prompt naming Render's exact Key Value suspend phrase (rendered bold by SudoCommandField)",
  },
  "keyvalue.confirmCancel": {
    message: "Abbrechen",
    description: "Suspend-confirmation dialog cancel button",
  },
  "keyvalue.toastSuspendSuccess": {
    message: "{name} wird pausiert…",
    description: "Toast after a suspend request is accepted",
  },
  "keyvalue.toastSuspendError": {
    message: "{name} konnte nicht pausiert werden. Bitte versuche es erneut.",
    description: "Toast when a suspend request fails",
  },
  "keyvalue.toastResumeSuccess": {
    message: "{name} wird fortgesetzt…",
    description: "Toast after a resume request is accepted",
  },
  "keyvalue.toastResumeError": {
    message:
      "{name} konnte nicht fortgesetzt werden. Bitte versuche es erneut.",
    description: "Toast when a resume request fails",
  },
  "keyvalue.planTitle": {
    message: "Compute-Plan",
    description: "Key Value detail plan-picker card title",
  },
  "keyvalue.planDescription": {
    message:
      "Wechsle zu einem größeren Compute-Plan; eine Key-Value-Instanz kann nicht auf einen kleineren wechseln. Die Änderung startet den Store neu, er ist dann ein bis zwei Minuten nicht verfügbar. Daten bleiben gemäß dem Persistenzmodus erhalten; ein Store mit Persistenz „Aus“ verliert jeden Key.",
    description: "Key Value detail plan-picker card description",
  },
  "keyvalue.planDowngradeUnavailable": {
    message:
      "Eine Key-Value-Instanz kann nicht auf einen kleineren Compute-Plan wechseln. Erstelle eine neue Instanz mit diesem Plan und verschiebe deine Daten dorthin.",
    description:
      "Tooltip on a disabled plan card smaller than the store's current plan: Render does not downgrade a Key Value instance (w8/249)",
  },
  "keyvalue.planPausedReason": {
    message:
      "Der Key-Value-Tarif „{name}“ ist vorübergehend nicht verfügbar; wähle {largest} oder einen kleineren Tarif.",
    description:
      "A paused Key Value plan's tooltip and screen-reader reason, composed in the dashboard's language; matches bex-api's English unavailableReason (w4/552)",
  },
  "keyvalue.planPickerSave": {
    message: "Speichern",
    description: "Key Value plan-picker save button",
  },
  "keyvalue.planPickerCancel": {
    message: "Abbrechen",
    description: "Key Value plan-picker cancel / reset button",
  },
  "keyvalue.reviewComputeLead": {
    message: "Prüfe deine Compute-Auswahl.",
    description:
      "Key Value plan review: the lead of a compute plan change's review (Render's copy)",
  },
  "keyvalue.reviewUnavailable": {
    message:
      "Deine Key-Value-Instanz ist während der Aktualisierung nicht verfügbar.",
    description:
      "Key Value plan review warning: the store restarts to apply the plan (Render's copy); the data-loss sentence follows it when persistence is Off",
  },
  "keyvalue.reviewUpgradeCompute": {
    message: "Compute upgraden",
    description:
      "Key Value plan review: the button that applies a plan change (Render's copy; Key Value plans only go up)",
  },
  "keyvalue.restartConfirmDataLoss": {
    message:
      "Die Persistenz ist „Aus“, daher löscht der Neustart jeden Key in {name}. Das kann nicht rückgängig gemacht werden.",
    description:
      "Restart confirm (plan change): a Persistence Off store loses all data on the restart (w4/m263)",
  },
  "keyvalue.restartConfirmDataLossUnknown": {
    message:
      "Der Persistenzmodus dieses Stores ist unbekannt. Wenn er „Aus“ ist, löscht der Neustart jeden Key in {name}. Das kann nicht rückgängig gemacht werden.",
    description:
      "Restart confirm when the persistence mode could not be read: the typed phrase is still required",
  },
  "keyvalue.planPickerSuccess": {
    message: "Tarif wird auf {name} geändert…",
    description: "Toast after a plan update is accepted",
  },
  "keyvalue.planPickerError": {
    message:
      "Der Tarif konnte nicht geändert werden. Bitte versuche es erneut.",
    description: "Toast when a plan update fails",
  },
  "keyvalue.maxmemoryTitle": {
    message: "Maxmemory-Policy",
    description: "Key Value detail eviction-policy card title",
  },
  "keyvalue.maxmemoryDescription": {
    message:
      "Ändere, wie Keys verdrängt werden, sobald der Store sein Speicherlimit erreicht. Die neue Policy gilt für den laufenden Store ohne Neustart.",
    description: "Key Value detail eviction-policy card description",
  },
  "keyvalue.maxmemoryLabel": {
    message: "Maxmemory-Policy",
    description: "Accessible label for the eviction-policy select",
  },
  "keyvalue.maxmemoryEdit": {
    message: "Maxmemory-Policy bearbeiten",
    description:
      "Accessible label for the maxmemory-policy edit-in-place pencil",
  },
  "keyvalue.maxmemorySuccess": {
    message: "Maxmemory-Policy wird auf {policy} geändert…",
    description: "Toast after an eviction-policy update is accepted",
  },
  "keyvalue.maxmemoryError": {
    message:
      "Die Maxmemory-Policy konnte nicht aktualisiert werden. Bitte versuche es erneut.",
    description: "Toast when an eviction-policy update fails",
  },
  "keyvalue.persistenceTitle": {
    message: "Persistenzmodus",
    description: "Key Value detail persistence-mode card title",
  },
  "keyvalue.persistenceDescription": {
    message:
      "Beim Wechsel des Modus wird der Store neu gestartet, und Verbindungen werden unterbrochen, während die Daten vorbereitet werden. Der Wechsel zwischen „Journal + Snapshot“ und „Nur Snapshot“ erhält die Daten. Der Wechsel zu oder von „Aus“ verwirft alle Daten.",
    description: "Key Value detail persistence-mode card description",
  },
  "keyvalue.persistenceLabel": {
    message: "Persistenzmodus",
    description: "Accessible label for the persistence-mode select",
  },
  "keyvalue.persistenceEdit": {
    message: "Persistenzmodus bearbeiten",
    description:
      "Accessible label for the persistence-mode edit-in-place pencil",
  },
  "keyvalue.persistenceSuccess": {
    message: "Persistenzmodus wird auf {mode} geändert…",
    description: "Toast after a persistence-mode update is accepted",
  },
  "keyvalue.persistenceConfirmTitle": {
    message: "Alle Daten löschen und Persistenz ändern?",
    description: "Destructive persistence-mode confirmation title",
  },
  "keyvalue.persistenceConfirmDescription": {
    message:
      "Der Wechsel zu oder von „Aus“ löscht alle Daten in {name} und startet den Store neu. Das kann nicht rückgängig gemacht werden.",
    description: "Destructive persistence-mode confirmation description",
  },
  "keyvalue.persistenceConfirmAction": {
    message: "Daten löschen und Modus ändern",
    description: "Destructive persistence-mode confirmation action",
  },
  "keyvalue.persistenceError": {
    message:
      "Der Persistenzmodus konnte nicht aktualisiert werden. Bitte versuche es erneut.",
    description: "Toast when a persistence-mode update fails",
  },
  "keyvalue.networkingTitle": {
    message: "Netzwerk",
    description: "Detail-page Networking card title (IP allowlist)",
  },
  "keyvalue.networkingDescription": {
    message:
      "Beschränke, welche Quell-IPs den externen Endpunkt erreichen können.",
    description: "Networking card subtitle",
  },
  "keyvalue.networkingHint": {
    message:
      "Das Speichern von CIDR-Regeln aktiviert den externen Zugriff für passende Quell-IPs. Das Speichern einer leeren Liste blockiert externe Verbindungen. Der interne Zugriff bleibt unverändert.",
    description: "Networking IP allowlist helper text",
  },
  "keyvalue.networkingHintRestart": {
    message:
      "Das Speichern von CIDR-Regeln aktiviert den externen Zugriff für passende Quell-IPs. Dieser Store hatte noch nie externen Zugriff, daher startet ihn das erste Speichern neu, und Verbindungen brechen bis zu eine Minute ab. Das Speichern einer leeren Liste blockiert externe Verbindungen.",
    description:
      "Networking IP allowlist helper text when enabling external access restarts the store (its first enable)",
  },
  "keyvalue.networkingRestartConfirmTitle": {
    message: "Alle Daten löschen und externen Zugriff aktivieren?",
    description:
      "Confirmation title when a first external-access enable restarts a persistence-Off store",
  },
  "keyvalue.networkingRestartConfirmDescription": {
    message:
      "Das Aktivieren des externen Zugriffs startet {name} zum ersten Mal neu. Sein Persistenzmodus ist „Aus“, daher löscht der Neustart alle Daten. Das kann nicht rückgängig gemacht werden.",
    description:
      "Confirmation description when a first external-access enable restarts a persistence-Off store",
  },
  "keyvalue.networkingRestartConfirmAction": {
    message: "Daten löschen und Regeln speichern",
    description:
      "Confirmation action when a first external-access enable restarts a persistence-Off store",
  },
  "keyvalue.networkingInternalOnly": {
    message:
      "Externe Verbindungen sind deaktiviert. Speichere eine oder mehrere IP-Regeln, um den Zugriff zu aktivieren.",
    description: "External access is disabled; saving rules enables it",
  },
  "keyvalue.networkingEmpty": {
    message:
      "Keine IP-Regeln. Speichere eine leere Liste, um externe Verbindungen zu blockieren.",
    description: "Empty datastore allowlist draft and the effect of saving it",
  },
  "keyvalue.networkingAdd": {
    message: "Hinzufügen",
    description: "Networking button to add a CIDR to the draft allowlist",
  },
  "keyvalue.networkingEntryDescription": {
    message: "Beschreibung (optional)",
    description:
      "Placeholder for the optional per-entry allowlist description input",
  },
  "keyvalue.networkingCIDR": {
    message: "Neuer CIDR-Block",
    description: "Accessible label for the add-row CIDR input",
  },
  "keyvalue.networkingCIDRRule": {
    message: "CIDR-Block für Regel {number}",
    description: "Accessible label for an existing allowlist CIDR row",
  },
  "keyvalue.networkingDescriptionLabel": {
    message: "Beschreibung der neuen Regel",
    description: "Accessible label for the add-row description input",
  },
  "keyvalue.networkingDescriptionRule": {
    message: "Beschreibung für Regel {number}",
    description: "Accessible label for an existing allowlist description row",
  },
  "keyvalue.networkingRemove": {
    message: "{cidr} entfernen",
    description: "Accessible label to remove a CIDR chip",
  },
  "keyvalue.networkingMoveUp": {
    message: "{cidr} nach oben verschieben",
    description: "Accessible label to move an allowlist entry earlier",
  },
  "keyvalue.networkingMoveDown": {
    message: "{cidr} nach unten verschieben",
    description: "Accessible label to move an allowlist entry later",
  },
  "keyvalue.networkingInvalid": {
    message: "Gib eindeutige, gültige IPv4- oder IPv6-CIDR-Blöcke ein.",
    description: "Validation error for Key Value allowlist entries",
  },
  "keyvalue.networkingSave": {
    message: "Allowlist speichern",
    description: "Networking button to persist the allowlist",
  },
  "keyvalue.networkingSaved": {
    message: "Allowlist aktualisiert.",
    description: "Toast after saving the IP allowlist",
  },
  "keyvalue.networkingError": {
    message:
      "Die Allowlist konnte nicht gespeichert werden. Bitte versuche es erneut.",
    description: "Toast when saving the allowlist fails",
  },
  "keyvalue.nameEdit": {
    message: "Key-Value-Namen bearbeiten",
    description: "Accessible label for the Key Value name edit-in-place pencil",
  },
  "keyvalue.nameDescription": {
    message:
      "Ändere den Anzeigenamen. Die Key-Value-ID und alle Verbindungsdetails bleiben gleich.",
    description: "Key Value detail rename row description",
  },
  "keyvalue.nameSuccess": {
    message: "Key-Value-Store in {name} umbenannt.",
    description: "Toast after a Key Value rename succeeds",
  },
  "keyvalue.nameError": {
    message:
      "Der Key-Value-Store konnte nicht umbenannt werden. Bitte versuche es erneut.",
    description: "Toast when a Key Value rename fails unexpectedly",
  },
  "keyvalue.nameConflict": {
    message:
      "In diesem Workspace gibt es bereits einen Key-Value-Store mit diesem Namen.",
    description: "Toast when a Key Value rename collides with another name",
  },
  "keyvalue.nameInvalid": {
    message:
      "Gib einen Namen mit höchstens 100 Zeichen ein, ohne Zeilenumbrüche oder Steuerzeichen.",
    description: "Key Value rename validation message",
  },
  // --- Sidebar and page headings (w3/m649) ---
  "keyvalue.navInfo": {
    message: "Info",
    description:
      "Key Value sidebar entry and page heading: the instance's Info page (general details, plan, connections, networking)",
  },
  "keyvalue.navLogs": {
    message: "Logs",
    description:
      "Key Value sidebar entry and page heading: the instance's Logs page",
  },
  "keyvalue.navMetrics": {
    message: "Metriken",
    description:
      "Key Value sidebar entry and page heading: the instance's Metrics page",
  },
  "keyvalue.logsRangeLabel": {
    message: "Zeitraum",
    description: "Accessible label for the log time-range select",
  },
  "keyvalue.logsAllInstances": {
    message: "Alle Instanzen",
    description: "Default option in the instance filter (no filter applied)",
  },
  "keyvalue.logsSearchPlaceholder": {
    message: "Logs durchsuchen…",
    description: "Placeholder for the log text-search input",
  },
  "keyvalue.logsLoading": {
    message: "Logs werden geladen…",
    description: "Loading state message while fetching logs",
  },
  "keyvalue.logsEmptyTitle": {
    message: "Keine Log-Zeilen",
    description: "Empty state title when no log lines are returned",
  },
  "keyvalue.logsEmptyBody": {
    message:
      "Keine Log-Zeilen in diesem Zeitraum. Valkey protokolliert Key-Space-Ereignisse, langsame Abfragen und Startmeldungen.",
    description: "Empty state body when no log lines and no filters active",
  },
  "keyvalue.logsUnavailableTitle": {
    message: "Logs nicht verfügbar",
    description: "Empty state title when the logs source is not configured",
  },
  "keyvalue.logsUnavailableBody": {
    message:
      "Die Log-Quelle der Plattform ist nicht konfiguriert. Wende dich an deinen Plattform-Admin, um BEX_LOKI_URL zu aktivieren.",
    description: "Empty state body when BEX_LOKI_URL is not set",
  },
  "keyvalue.logsUnauthorizedTitle": {
    message: "Zugriff verweigert",
    description: "Empty state title when the caller lacks can_view_logs",
  },
  "keyvalue.logsUnauthorizedBody": {
    message:
      "Du hast keine Berechtigung, die Logs dieses Key-Value-Stores anzusehen.",
    description: "Empty state body for a 403 on the logs query",
  },
  "keyvalue.fieldNameDescription": {
    message: "Ein eindeutiger Name für Ihre Key-Value-Instanz.",
    description: "Create-Key-Value Name row description",
  },
  "keyvalue.fieldPlanDescription": {
    message:
      "Bestimmt den verfügbaren Arbeitsspeicher und das Verbindungslimit der Instanz.",
    description:
      "Create-Key-Value plan row description (Render: the plan determines RAM and connection limit)",
  },
  "keyvalue.connectInternalTitle": {
    message: "Interne Redis-URL",
    description:
      "Header Connect menu, Internal tab: the private-network URL's title (Render's copy, which names Key Value by its type name Redis, w3/m675)",
  },
  "keyvalue.connectExternalTitle": {
    message: "Externe Redis-URL",
    description:
      "Header Connect menu, External tab: the public URL's title (Render's copy, w3/m675)",
  },
  "keyvalue.connectCliTitle": {
    message: "Redis-CLI-Befehl",
    description:
      "Header Connect menu, External tab: the `bex kv-cli <id>` command's title (Render's copy, w3/m675)",
  },
};

export default deKeyValue;
