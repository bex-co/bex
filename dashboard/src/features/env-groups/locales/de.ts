import type { TranslationEntry } from "@/i18n";

const deEnvGroups: Record<string, TranslationEntry> = {
  "envGroups.resourceType": {
    message: "Umgebungsgruppe",
    description: "Environment Group resource type used in document titles",
  },
  "envGroups.pageTitle": {
    message: "Umgebungsgruppen",
    description: "Workspace env-groups page title",
  },
  "envGroups.pageDescription": {
    message:
      "Teile Umgebungsvariablen und Secret-Dateien über mehrere Services hinweg.",
    description: "Workspace env-groups page subtitle",
  },
  "envGroups.newButton": {
    message: "Neue Umgebungsgruppe",
    description: "Open the create env-group dialog",
  },
  "envGroups.createTitle": {
    message: "Umgebungsgruppe erstellen",
    description: "Create env-group dialog title",
  },
  "envGroups.createDescription": {
    message:
      "Füge jetzt Variablen, Secret-Dateien und Service-Verknüpfungen hinzu oder lass sie leer.",
    description: "Create env-group dialog description",
  },
  "envGroups.createVarsTitle": {
    message: "Umgebungsvariablen",
    description: "Initial environment variables section title",
  },
  "envGroups.createVarsDescription": {
    message:
      "Lege Klartextwerte fest oder generiere bei der Erstellung starke Secrets.",
    description: "Initial environment variables section description",
  },
  "envGroups.varKeyLabel": {
    message: "Schlüssel",
    description: "Initial environment variable key label",
  },
  "envGroups.varKeyPlaceholder": {
    message: "API_TOKEN",
    description: "Initial environment variable key placeholder",
  },
  "envGroups.varValueLabel": {
    message: "Wert",
    description: "Initial environment variable value label",
  },
  "envGroups.varValuePlaceholder": {
    message: "Secret-Wert",
    description: "Initial environment variable value placeholder",
  },
  "envGroups.generateValue": {
    message: "Generieren",
    description: "Generate an initial environment variable value",
  },
  "envGroups.addInitialVar": {
    message: "Umgebungsvariable hinzufügen",
    description: "Add an initial environment variable row",
  },
  "envGroups.importEnv": {
    message: "Aus .env importieren",
    description:
      "Import initial environment variables from dotenv text or file",
  },
  "envGroups.createFilesTitle": {
    message: "Secret-Dateien",
    description: "Initial secret files section title",
  },
  "envGroups.createFilesDescription": {
    message: "Dateien werden verschlüsselt und unter /etc/secrets eingebunden.",
    description: "Initial secret files section description",
  },
  "envGroups.fileNameLabel": {
    message: "Dateiname",
    description: "Initial secret file name label",
  },
  "envGroups.fileNamePlaceholder": {
    message: "credentials.json",
    description: "Initial secret file name placeholder",
  },
  "envGroups.fileContentLabel": {
    message: "Inhalt",
    description: "Initial secret file contents label",
  },
  "envGroups.fileContentPlaceholder": {
    message: "Dateiinhalt",
    description: "Initial secret file contents placeholder",
  },
  "envGroups.addInitialFile": {
    message: "Secret-Datei hinzufügen",
    description: "Add an initial secret file row",
  },
  "envGroups.createServicesTitle": {
    message: "Verknüpfte Services",
    description: "Initial service links section title",
  },
  "envGroups.createServicesDescription": {
    message:
      "Ausgewählte Services erhalten diese Gruppe, sobald sie erstellt ist.",
    description: "Initial service links section description",
  },
  "envGroups.servicesLoading": {
    message: "Workspace-Services werden geladen…",
    description: "Initial service links loading state",
  },
  "envGroups.createScopeLoading": {
    message: "Umgebungen und Services werden geladen…",
    description:
      "Create dialog waits for a confirmed scope and link candidates",
  },
  "envGroups.createScopeError": {
    message:
      "Die Umgebung oder die Services konnten nicht geladen werden. Versuche es erneut, bevor du die Gruppe erstellst.",
    description:
      "Create dialog blocks submission when scope or service lookup fails",
  },
  "envGroups.noServicesToLink": {
    message: "Dieser Workspace hat noch keine Services zum Verknüpfen.",
    description: "Initial service links empty state",
  },
  "envGroups.noServicesInScope": {
    message:
      "In dieser Umgebung gibt es keine Services. Services können nur mit einer Gruppe in ihrer eigenen Umgebung verknüpft werden.",
    description: "Initial service links empty state for the picked scope",
  },
  "envGroups.invalidInitialContents": {
    message:
      "Korrigiere ungültige Variablenschlüssel oder Dateinamen, bevor du die Gruppe erstellst.",
    description: "Invalid initial env-group contents message",
  },
  "envGroups.nameLabel": {
    message: "Gruppenname",
    description: "Env-group name field label",
  },
  "envGroups.namePlaceholder": {
    message: "shared-production",
    description: "Env-group name placeholder",
  },
  "envGroups.invalidName": {
    message: "Gib einen Gruppennamen ein.",
    description: "Invalid env-group name message",
  },
  "envGroups.cancel": {
    message: "Abbrechen",
    description: "Cancel an env-group dialog",
  },
  "envGroups.createSubmit": {
    message: "Umgebungsgruppe erstellen",
    description: "Create env-group submit button",
  },
  "envGroups.emptyTitle": {
    message: "Keine Umgebungsgruppen",
    description: "Workspace env-groups empty-state title",
  },
  "envGroups.emptyBody": {
    message:
      "Erstelle eine Gruppe, um gemeinsame Konfiguration zu verwalten, bevor du sie mit einem Service verknüpfst.",
    description: "Workspace env-groups empty-state body",
  },
  "envGroups.serviceCount_one": {
    message: "{count} verknüpfter Service",
    description: "Env-group linked-service count (singular)",
  },
  "envGroups.serviceCount_other": {
    message: "{count} verknüpfte Services",
    description: "Env-group linked-service count (plural)",
  },
  "envGroups.metadataTitle": {
    message: "Gruppen-Metadaten",
    description: "Env-group metadata card title",
  },
  "envGroups.ownerLabel": {
    message: "Owner",
    description: "Env-group owner id label",
  },
  "envGroups.environmentLabel": {
    message: "Umgebung",
    description: "Environment Group scope label",
  },
  "envGroups.workspaceLabel": {
    message: "Workspace",
    description: "Destination workspace label",
  },
  "envGroups.workspaceScope": {
    message: "Workspace (keine Umgebung)",
    description: "Standalone Environment Group scope",
  },
  "envGroups.environmentLookupLoading": {
    message: "Umgebung wird geladen…",
    description:
      "Group metadata Environment value while the scope lookup is still loading",
  },
  "envGroups.environmentLookupFailed": {
    message: "Umgebung konnte nicht geladen werden",
    description:
      "Group metadata Environment value when the scope lookup request failed",
  },
  "envGroups.unknownEnvironment": {
    message: "Unbekannte Umgebung ({id})",
    description: "Deleted or inaccessible Environment fallback",
  },
  "envGroups.createdAtLabel": {
    message: "Erstellt",
    description: "Env-group created timestamp label",
  },
  "envGroups.updatedAtLabel": {
    message: "Aktualisiert",
    description: "Env-group updated timestamp label",
  },
  "envGroups.availabilityLabel": {
    message: "Status",
    description: "Env-group availability label when busy or repair-required",
  },
  "envGroups.availabilityBusy": {
    message: "Speichern läuft – versuche es gleich erneut",
    description: "Busy environment-group availability message",
  },
  "envGroups.availabilityRepairRequired": {
    message:
      "Wiederherstellung nötig – wende dich an den Support, falls dies bestehen bleibt",
    description: "Repair-required environment-group availability message",
  },
  "envGroups.unavailableTitle": {
    message: "Umgebungsgruppen nicht verfügbar",
    description: "Secret store unavailable state title",
  },
  "envGroups.unavailableBody": {
    message:
      "Der Secret-Speicher ist für dieses Deployment nicht konfiguriert.",
    description: "Secret store unavailable state body",
  },
  "envGroups.forbiddenTitle": {
    message: "Nicht autorisiert",
    description: "Env-group forbidden state title",
  },
  "envGroups.forbiddenBody": {
    message:
      "Du hast keine Berechtigung, Umgebungsgruppen anzuzeigen oder zu verwalten.",
    description: "Env-group forbidden state body",
  },
  "envGroups.genericTitle": {
    message: "Umgebungsgruppen konnten nicht geladen werden",
    description: "Env-group generic query error title",
  },
  "envGroups.genericBody": {
    message: "Etwas ist schiefgelaufen. Bitte versuche es erneut.",
    description: "Env-group generic query error body",
  },
  "envGroups.errorTitle": {
    message: "Gruppeninhalte konnten nicht geladen werden",
    description: "Env-group editor error title",
  },
  "envGroups.errorBody": {
    message: "Etwas ist schiefgelaufen. Bitte versuche es erneut.",
    description: "Env-group editor error body",
  },
  "envGroups.backToList": {
    message: "Zurück zu den Umgebungsgruppen",
    description: "Detail-page back button label",
  },
  "envGroups.renameButton": {
    message: "Umbenennen",
    description: "Open rename env-group dialog",
  },
  "envGroups.manageButton": {
    message: "Verwalten",
    description: "Open Environment Group management menu",
  },
  "envGroups.moveButton": {
    message: "Gruppe verschieben",
    description: "Open Environment Group move dialog",
  },
  "envGroups.moveTitle": {
    message: "Umgebungsgruppe verschieben",
    description: "Environment Group move dialog title",
  },
  "envGroups.moveDescription": {
    message:
      "Wähle eine kompatible Umgebung oder gib diese Gruppe an den Workspace-Bereich zurück.",
    description: "Environment Group move dialog description",
  },
  "envGroups.moveSubmit": {
    message: "Gruppe verschieben",
    description: "Submit Environment Group move",
  },
  "envGroups.moveSuccess": {
    message: "Umgebungsgruppe verschoben",
    description: "Environment Group move success toast",
  },
  "envGroups.moveError": {
    message: "Die Gruppe konnte nicht verschoben werden",
    description: "Environment Group move error toast",
  },
  "envGroups.cloneButton": {
    message: "Gruppe klonen",
    description: "Open Environment Group clone dialog",
  },
  "envGroups.cloneTitle": {
    message: "Umgebungsgruppe klonen",
    description: "Environment Group clone dialog title",
  },
  "envGroups.cloneDescription": {
    message:
      "Kopiere Variablen und Dateien in eine neue Gruppe. Service-Verknüpfungen werden nicht kopiert.",
    description: "Environment Group clone dialog description",
  },
  "envGroups.cloneSubmit": {
    message: "Gruppe klonen",
    description: "Submit Environment Group clone",
  },
  "envGroups.cloneSuccess": {
    message: "{name} geklont",
    description: "Environment Group clone success toast",
  },
  "envGroups.cloneError": {
    message: "Die Gruppe konnte nicht geklont werden",
    description: "Environment Group clone error toast",
  },
  "envGroups.environmentPlaceholder": {
    message: "Umgebung auswählen",
    description: "Environment scope selector placeholder",
  },
  "envGroups.renameTitle": {
    message: "Umgebungsgruppe umbenennen",
    description: "Rename env-group dialog title",
  },
  "envGroups.renameDescription": {
    message:
      "Beim Umbenennen bleiben die Variablen, Dateien und Service-Verknüpfungen der Gruppe erhalten.",
    description: "Rename env-group dialog description",
  },
  "envGroups.renameSubmit": {
    message: "Namen speichern",
    description: "Rename env-group submit button",
  },
  "envGroups.deleteButton": {
    message: "Löschen",
    description: "Open delete env-group dialog",
  },
  "envGroups.deleteTitle": {
    message: "{name} löschen?",
    description: "Delete env-group dialog title",
  },
  "envGroups.deleteDescription": {
    message:
      "Diese Aktion kann nicht rückgängig gemacht werden. Möchtest du diese Umgebungsgruppe wirklich löschen? Sie wird von jedem Service getrennt.",
    description: "Delete env-group dialog warning (Render-style copy)",
  },
  "envGroups.deletePrompt": {
    message: "Gib unten {phrase} ein, um zu bestätigen.",
    description:
      "Body prompt naming the exact 'sudo delete env group <name>' phrase (rendered bold by SudoCommandField)",
  },
  "envGroups.deleteConfirm": {
    message: "Umgebungsgruppe löschen",
    description: "Delete env-group confirm button",
  },
  "envGroups.varsTitle": {
    message: "Umgebungsvariablen",
    description: "Env-group variable editor title",
  },
  "envGroups.varsDescription": {
    message:
      "Werte werden verschlüsselt und auf jeden verknüpften Service angewendet.",
    description: "Env-group variable editor description",
  },
  "envGroups.varsEmptyTitle": {
    message: "Keine Umgebungsvariablen",
    description: "Env-group variable editor empty title",
  },
  "envGroups.varsEmptyBody": {
    message:
      "Füge jetzt eine Variable hinzu; die Gruppe muss vorher nicht verknüpft sein.",
    description: "Env-group variable editor empty body",
  },
  "envGroups.varDeleteConfirmBody": {
    message:
      "Verknüpfte Services werden ohne diese Variable neu deployt. Ausgenommen ist ein Repository-basierter Service mit deaktiviertem Auto-Deploy: Er liefert weiter seinen aktuellen Release aus und übernimmt die Änderung bei seinem nächsten Deployment.",
    description: "Delete env-group variable warning",
  },
  "envGroups.filesTitle": {
    message: "Secret-Dateien",
    description: "Env-group secret-file editor title",
  },
  "envGroups.filesDescription": {
    message:
      "Verschlüsselte Dateien werden in jeden verknüpften Service unter /etc/secrets eingebunden.",
    description: "Env-group secret-file editor description",
  },
  "envGroups.filesEmptyTitle": {
    message: "Keine Secret-Dateien",
    description: "Env-group secret-file editor empty title",
  },
  "envGroups.filesEmptyBody": {
    message:
      "Füge jetzt eine Datei hinzu; die Gruppe muss vorher nicht verknüpft sein.",
    description: "Env-group secret-file editor empty body",
  },
  "envGroups.fileDeleteConfirmBody": {
    message:
      "Verknüpfte Services werden ohne diese Datei neu deployt. Ausgenommen ist ein Repository-basierter Service mit deaktiviertem Auto-Deploy: Er liefert weiter seinen aktuellen Release aus und übernimmt die Änderung bei seinem nächsten Deployment.",
    description: "Delete env-group secret-file warning",
  },
  "envGroups.servicesTitle": {
    message: "Verknüpfte Services",
    description: "Env-group linked-services card title",
  },
  "envGroups.servicesDescription": {
    message:
      "Das Verknüpfen oder Trennen ändert die Service-Konfiguration. Ein Repository-basierter Service mit deaktiviertem Auto-Deploy übernimmt die Änderung bei seinem nächsten Deployment; jeder andere Service deployt sie sofort.",
    description: "Env-group linked-services card description",
  },
  "envGroups.selectService": {
    message: "Service auswählen",
    description: "Link-service selector placeholder",
  },
  "envGroups.linkButton": {
    message: "Service verknüpfen",
    description: "Link selected service to env group",
  },
  "envGroups.unlinkButton": {
    message: "Trennen",
    description: "Unlink service from env group",
  },
  "envGroups.unlinkServiceAria": {
    message: "{name} trennen",
    description:
      "Accessible name of a linked-service row's Unlink control, naming the service (w4/150)",
  },
  "envGroups.noLinkedServices": {
    message:
      "Diese Gruppe ist noch mit keinem Service verknüpft. Sie ist trotzdem vollständig bearbeitbar.",
    description: "No linked services state",
  },
  "envGroups.servicesLoadError": {
    message:
      "Workspace-Services konnten nicht geladen werden. Bestehende Verknüpfungen bleiben unten verfügbar.",
    description: "Linked-services inventory error",
  },
  "envGroups.incompatibleLink": {
    message: "Außerhalb der Umgebung dieser Gruppe verknüpft",
    description: "Warning on an existing incompatible Environment Group link",
  },
  "envGroups.searchPlaceholder": {
    message: "Umgebungsgruppen durchsuchen",
    description: "Environment Group list search placeholder",
  },
  "envGroups.searchLabel": {
    message: "Umgebungsgruppen nach Namen durchsuchen",
    description: "Accessible Environment Group search label",
  },
  "envGroups.resetSearch": {
    message: "Suche zurücksetzen",
    description: "Clear Environment Group search",
  },
  "envGroups.noMatchesTitle": {
    message: "Keine passenden Umgebungsgruppen",
    description: "Environment Group search empty-state title",
  },
  "envGroups.noMatchesBody": {
    message: "Versuche einen anderen Namen oder setze die Suche zurück.",
    description: "Environment Group search empty-state body",
  },
  "envGroups.colName": {
    message: "Name der Umgebungsgruppe",
    description: "Environment Group table name column",
  },
  "envGroups.colEnvironment": {
    message: "Umgebung",
    description: "Environment Group table Environment column",
  },
  "envGroups.colEnvVars": {
    message: "Umgebungsvariablen",
    description: "Environment Group table variable-count column",
  },
  "envGroups.colSecretFiles": {
    message: "Secret-Dateien",
    description: "Environment Group table secret-file-count column",
  },
  "envGroups.colUpdated": {
    message: "Aktualisiert",
    description: "Environment Group table updated-time column",
  },
  "envGroups.rolloutNote": {
    message:
      "Verknüpfte Services werden neu deployt, um die Änderung anzuwenden. Ausgenommen ist ein Repository-basierter Service mit deaktiviertem Auto-Deploy, der sie bei seinem nächsten Deployment übernimmt.",
    description: "Env-group write rollout toast detail",
  },
  "envGroups.linkChangeNote": {
    message:
      "Die Konfiguration des Services hat sich geändert. Ein Repository-basierter Service mit deaktiviertem Auto-Deploy übernimmt sie bei seinem nächsten Deployment; jeder andere Service deployt sie sofort.",
    description: "Env-group link/unlink success toast detail",
  },
  "envGroups.createSuccess": {
    message: "{name} erstellt",
    description: "Env-group create success toast",
  },
  "envGroups.createError": {
    message: "{name} konnte nicht erstellt werden",
    description: "Env-group create error toast",
  },
  "envGroups.renameSuccess": {
    message: "Umbenannt in {name}",
    description: "Env-group rename success toast",
  },
  "envGroups.renameError": {
    message: "Die Gruppe konnte nicht umbenannt werden",
    description: "Env-group rename error toast",
  },
  "envGroups.deleteSuccess": {
    message: "Umgebungsgruppe gelöscht",
    description: "Env-group delete success toast",
  },
  "envGroups.deleteError": {
    message: "Die Gruppe konnte nicht gelöscht werden",
    description: "Env-group delete error toast",
  },
  "envGroups.linkSuccess": {
    message: "Service verknüpft",
    description: "Env-group link success toast",
  },
  "envGroups.linkError": {
    message: "Der Service konnte nicht verknüpft werden",
    description: "Env-group link error toast",
  },
  "envGroups.unlinkSuccess": {
    message: "Service getrennt",
    description: "Env-group unlink success toast",
  },
  "envGroups.unlinkError": {
    message: "Der Service konnte nicht getrennt werden",
    description: "Env-group unlink error toast",
  },
  "envGroups.varSaveSuccess": {
    message: "{key} gespeichert",
    description: "Env-group variable save success toast",
  },
  "envGroups.varsSaveSuccess": {
    message: "Umgebungsvariablen gespeichert",
    description: "Env-group replace-all variables success toast",
  },
  "envGroups.varsSaveError": {
    message: "Umgebungsvariablen konnten nicht gespeichert werden",
    description: "Env-group replace-all variables error toast",
  },
  "envGroups.varSaveError": {
    message: "{key} konnte nicht gespeichert werden",
    description: "Env-group variable save error toast",
  },
  "envGroups.varDeleteSuccess": {
    message: "{key} entfernt",
    description: "Env-group variable delete success toast",
  },
  "envGroups.varDeleteError": {
    message: "{key} konnte nicht entfernt werden",
    description: "Env-group variable delete error toast",
  },
};

export default deEnvGroups;
