import type { TranslationEntry } from "@/i18n";

const deSandboxGroups: Record<string, TranslationEntry> = {
  "sandboxGroups.createTitle": {
    message: "Neue Sandbox-Gruppe",
    description:
      "Title of the new-sandbox-group dialog and its entry in the + New menus",
  },
  "sandboxGroups.createDescription": {
    message:
      "Eine Sandbox-Gruppe begrenzt, wie viele ihrer Sandboxes gleichzeitig aktiv sein können. Gruppennamen müssen innerhalb deines Workspace eindeutig sein.",
    description: "New-sandbox-group dialog description",
  },
  "sandboxGroups.fieldName": {
    message: "Name",
    description: "New-sandbox-group dialog name field label",
  },
  "sandboxGroups.fieldNamePlaceholder": {
    message: "meine-sandbox-gruppe",
    description: "Placeholder for the new sandbox group's name",
  },
  "sandboxGroups.fieldRegion": {
    message: "Region",
    description:
      "Label of the region row in the created sandbox group's confirmation",
  },
  "sandboxGroups.createCancel": {
    message: "Abbrechen",
    description: "Cancel button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createSubmit": {
    message: "Sandbox-Gruppe erstellen",
    description: "Submit button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createError": {
    message:
      "Sandbox-Gruppe {name} konnte nicht erstellt werden. Bitte versuche es erneut.",
    description: "Toast shown when creating a sandbox group fails",
  },
  "sandboxGroups.errorNameTaken": {
    message:
      "In diesem Workspace gibt es bereits eine Sandbox-Gruppe namens „{name}“. Wähle einen anderen Namen.",
    description:
      "Inline error under the name field when the group name is already taken",
  },
  "sandboxGroups.errorNameInvalid": {
    message:
      "Ein Gruppenname muss 1 bis {max} Zeichen lang sein, ohne führende oder nachgestellte Leerzeichen und ohne Steuerzeichen.",
    description: "Inline error under the name field when the name is refused",
  },
  "sandboxGroups.errorLimitReached_one": {
    message:
      "Dieser Workspace hat bereits das Maximum von {count} Sandbox-Gruppe.",
    description:
      "Inline alert when the workspace is at its sandbox group limit (singular)",
  },
  "sandboxGroups.errorLimitReached_other": {
    message:
      "Dieser Workspace hat bereits das Maximum von {count} Sandbox-Gruppen.",
    description:
      "Inline alert when the workspace is at its sandbox group limit",
  },
  "sandboxGroups.errorLimitReachedGeneric": {
    message: "Dieser Workspace hat sein Limit für Sandbox-Gruppen erreicht.",
    description:
      "Inline alert for the sandbox group limit when the server names no number",
  },
  "sandboxGroups.errorLimitTitle": {
    message: "Limit für Sandbox-Gruppen erreicht",
    description: "Heading of the inline sandbox-group-limit alert",
  },
  "sandboxGroups.createdTitle": {
    message: "Sandbox-Gruppe erstellt",
    description: "Dialog title once a sandbox group has been created",
  },
  "sandboxGroups.createdDescription": {
    message:
      "{name} ist bereit. Übergib ihre ID als sandboxGroupId, wenn du darin eine Sandbox erstellst.",
    description: "Dialog description once a sandbox group has been created",
  },
  "sandboxGroups.createdDone": {
    message: "Fertig",
    description: "Button that closes the created-sandbox-group confirmation",
  },
  "sandboxGroups.fieldId": {
    message: "ID",
    description: "Label of the new sandbox group's ID in the confirmation",
  },
  "sandboxGroups.fieldDefault": {
    message: "Standardgruppe",
    description:
      "Label of the confirmation row that says whether the group is the workspace's default",
  },
  "sandboxGroups.defaultYes": {
    message: "Ja",
    description: "The created group is the workspace's default sandbox group",
  },
  "sandboxGroups.defaultNo": {
    message: "Nein",
    description:
      "The created group is not the workspace's default sandbox group",
  },
  "sandboxGroups.fieldConcurrencyLimit": {
    message: "Parallelitätslimit",
    description:
      "Label of the confirmation row with the most active sandboxes the group allows",
  },
  "sandboxGroups.copyId": {
    message: "Sandbox-Gruppen-ID kopieren",
    description: "Accessible label of the copy button next to the group's ID",
  },
  "sandboxGroups.copied": {
    message: "Sandbox-Gruppen-ID kopiert",
    description: "Toast after copying the sandbox group ID",
  },
  "sandboxGroups.copyError": {
    message: "Sandbox-Gruppen-ID konnte nicht kopiert werden",
    description: "Toast when copying the sandbox group ID fails",
  },
  "sandboxGroups.environmentLoading": {
    message: "Umgebungen werden geladen",
    description:
      "Accessible name of the loading placeholder shown where the new-sandbox-group dialog's Environment select will appear",
  },
  "sandboxGroups.environmentsError": {
    message:
      "Deine Umgebungen konnten nicht geladen werden. Du kannst die Gruppe trotzdem ohne Umgebung erstellen.",
    description:
      "Shown in the new-sandbox-group dialog when the workspace's environments could not be read; the group can still be created without one",
  },
  "sandboxGroups.errorEnvironmentGone": {
    message:
      "Diese Umgebung existiert nicht mehr. Wähle eine andere oder keine.",
    description:
      "Inline error under the Environment select when the chosen environment was deleted before the group was created",
  },
  "sandboxGroups.pageTitle": {
    message: "Sandbox-Gruppen",
    description: "Heading for the workspace sandbox groups page",
  },
  "sandboxGroups.pageDescription": {
    message:
      "Verwalte Parallelitätslimits und vermittelte ausgehende Zugangsdaten für jede Gruppe.",
    description: "Subtitle for the workspace sandbox groups page",
  },
  "sandboxGroups.resourceType": {
    message: "Sandbox-Gruppe",
    description: "Singular resource type name for a sandbox group",
  },
  "sandboxGroups.backToList": {
    message: "Zurück zu den Sandbox-Gruppen",
    description: "Accessible label of the back link on a sandbox group detail page",
  },
  "sandboxGroups.defaultBadge": {
    message: "Standard",
    description: "Badge indicating the workspace default sandbox group",
  },
  "sandboxGroups.listLoadError": {
    message: "Sandbox-Gruppen konnten nicht geladen werden. Bitte versuche es erneut.",
    description: "Error state when loading sandbox groups fails",
  },
  "sandboxGroups.listEmptyTitle": {
    message: "Noch keine Sandbox-Gruppen",
    description: "Empty state heading when the workspace has no sandbox groups",
  },
  "sandboxGroups.listEmptyDescription": {
    message:
      "Erstelle eine Sandbox-Gruppe, um Parallelitätslimits und ausgehende Zugangsdaten zu konfigurieren.",
    description: "Empty state body when the workspace has no sandbox groups",
  },
  "sandboxGroups.manageCredentialsButton": {
    message: "Zugangsdaten",
    description: "Button on each sandbox group row linking to its detail and credentials",
  },
  "sandboxGroups.detailLoadError": {
    message: "Diese Sandbox-Gruppe konnte nicht geladen werden. Bitte versuche es erneut.",
    description: "Error state when loading a single sandbox group fails",
  },
  "sandboxGroups.credSectionTitle": {
    message: "Ausgehende Zugangsdaten",
    description: "Card title for the sandbox group credentials panel",
  },
  "sandboxGroups.credSectionDescription": {
    message:
      "Vermittelte Geheimnisse, die in passende ausgehende HTTPS-Anfragen aus Sandboxen dieser Gruppe eingefügt werden. Gespeicherte Werte sind schreibgeschützt und werden nie wieder angezeigt.",
    description: "Card subtitle for the sandbox group credentials panel",
  },
  "sandboxGroups.credAddButton": {
    message: "Zugangsdaten hinzufügen",
    description: "Button to open the create sandbox group credential dialog",
  },
  "sandboxGroups.credLoadError": {
    message: "Zugangsdaten der Sandbox-Gruppe konnten nicht geladen werden. Bitte versuche es erneut.",
    description: "Error state when fetching sandbox group credentials fails",
  },
  "sandboxGroups.credEmptyTitle": {
    message: "Keine Zugangsdaten konfiguriert",
    description: "Empty state heading when a sandbox group has no credentials",
  },
  "sandboxGroups.credEmptyDescriptionAdmin": {
    message:
      "Füge Zugangsdaten hinzu, um Authentifizierung in ausgehende Anfragen einzufügen, ohne das Geheimnis im Sandbox-Code offenzulegen.",
    description: "Empty state body for admins when no credentials exist",
  },
  "sandboxGroups.credEmptyDescriptionReadOnly": {
    message: "Ein Workspace-Admin kann ausgehende Zugangsdaten für diese Gruppe hinzufügen.",
    description: "Empty state body for non-admins when no credentials exist",
  },
  "sandboxGroups.credColId": {
    message: "Zugangsdaten-ID",
    description: "Table column header for credential ID",
  },
  "sandboxGroups.credColDomain": {
    message: "Zieldomain",
    description: "Table column header for destination domain",
  },
  "sandboxGroups.credColInjection": {
    message: "Injektion",
    description: "Table column header for injection method and target",
  },
  "sandboxGroups.credColRouting": {
    message: "Weiterleitung",
    description: "Table column header for HTTP methods and path filters",
  },
  "sandboxGroups.credColEnabled": {
    message: "Aktiviert",
    description: "Table column header for enabled toggle or status",
  },
  "sandboxGroups.credColActions": {
    message: "Aktionen",
    description: "Table column header for row actions",
  },
  "sandboxGroups.credAllMethods": {
    message: "Alle Methoden",
    description: "Badge shown when a credential matches all HTTP methods",
  },
  "sandboxGroups.credAllPaths": {
    message: "Alle Pfade",
    description: "Badge shown when a credential matches all request paths",
  },
  "sandboxGroups.credStatusEnabled": {
    message: "Aktiviert",
    description: "Read-only badge when a credential is enabled",
  },
  "sandboxGroups.credStatusDisabled": {
    message: "Deaktiviert",
    description: "Read-only badge when a credential is disabled",
  },
  "sandboxGroups.credToggleEnabledLabel": {
    message: "Status für {id} umschalten",
    description: "Accessible label for the enabled switch on a credential row",
  },
  "sandboxGroups.credVersionsActionLabel": {
    message: "Versionen für {id} anzeigen",
    description: "Accessible label for the version history icon button",
  },
  "sandboxGroups.credEditActionLabel": {
    message: "Zugangsdaten {id} bearbeiten",
    description: "Accessible label for the edit credential icon button",
  },
  "sandboxGroups.credDeleteActionLabel": {
    message: "Zugangsdaten {id} löschen",
    description: "Accessible label for the delete credential icon button",
  },
  "sandboxGroups.credVersionsButton": {
    message: "Versionen",
    description: "Mobile button label to open credential version history",
  },
  "sandboxGroups.credEditButton": {
    message: "Bearbeiten",
    description: "Mobile button label to edit a credential",
  },
  "sandboxGroups.credDeleteButton": {
    message: "Löschen",
    description: "Button label to delete a credential",
  },
  "sandboxGroups.credCreateTitle": {
    message: "Zugangsdaten hinzufügen",
    description: "Dialog title when creating a sandbox group credential",
  },
  "sandboxGroups.credCreateDescription": {
    message: "Konfiguriere schreibgeschützte ausgehende Zugangsdaten für diese Sandbox-Gruppe.",
    description: "Dialog description when creating a sandbox group credential",
  },
  "sandboxGroups.credEditTitle": {
    message: "{id} bearbeiten",
    description: "Dialog title when editing a sandbox group credential",
  },
  "sandboxGroups.credEditDescriptionAdmin": {
    message:
      "Aktualisiere Weiterleitung, Injektion, Domain oder rotiere das Geheimnis. Lass das Geheimnis leer, um den aktuellen Wert beizubehalten.",
    description: "Dialog description when an admin edits a credential",
  },
  "sandboxGroups.credEditDescriptionDeveloper": {
    message:
      "Aktualisiere Injektion, Methoden, Pfade oder Aktivierung. Das Ändern von Domain oder Geheimnis erfordert Admin-Rechte.",
    description: "Dialog description when a developer edits a credential",
  },
  "sandboxGroups.credFieldId": {
    message: "Zugangsdaten-ID",
    description: "Form label for credential ID",
  },
  "sandboxGroups.credFieldIdHelp": {
    message: "Groß-/Kleinschreibung beachten: Buchstaben, Ziffern, ., _, ~ oder -. Darf nicht mit `rnd-` beginnen.",
    description: "Help text for credential ID format",
  },
  "sandboxGroups.credFieldDomain": {
    message: "Zieldomain",
    description: "Form label for destination hostname",
  },
  "sandboxGroups.credAdminOnlyDomainSecret": {
    message: "Nur Workspace-Admins können Domain oder Geheimnis ändern.",
    description: "Helper text shown to developers on read-only domain field",
  },
  "sandboxGroups.credFieldValue": {
    message: "Geheimwert",
    description: "Form label for secret value on create",
  },
  "sandboxGroups.credFieldValuePlaceholder": {
    message: "Geheimwert einfügen",
    description: "Placeholder for secret value input on create",
  },
  "sandboxGroups.credFieldReplacementValue": {
    message: "Neuer Geheimwert (optional)",
    description: "Form label for rotating secret value on edit",
  },
  "sandboxGroups.credFieldReplacementValuePlaceholder": {
    message: "Leer lassen, um den aktuellen Wert zu behalten",
    description: "Placeholder for optional secret rotation input on edit",
  },
  "sandboxGroups.credFieldValueWriteOnlyHelp": {
    message: "Nur Schreiben (bis zu 5 KiB). Wird nach dem Speichern nie wieder angezeigt.",
    description: "Help text explaining write-only secret handling",
  },
  "sandboxGroups.credFieldInjectionType": {
    message: "Injektionstyp",
    description: "Form label for injection type select",
  },
  "sandboxGroups.credInjectionHeader": {
    message: "HTTP-Header",
    description: "Injection type option for HTTP header",
  },
  "sandboxGroups.credInjectionBasic": {
    message: "Basic-Authentifizierung",
    description: "Injection type option for HTTP Basic authentication",
  },
  "sandboxGroups.credInjectionCookie": {
    message: "HTTP-Cookie",
    description: "Injection type option for HTTP cookie",
  },
  "sandboxGroups.credInjectionQuery": {
    message: "Abfrageparameter",
    description: "Injection type option for URL query parameter",
  },
  "sandboxGroups.credFieldInjectionName": {
    message: "Zielname",
    description: "Form label for header, cookie, or query parameter name",
  },
  "sandboxGroups.credBasicHelp": {
    message: "Verwendet automatisch den `Authorization`-Header. Gib `username:password` an.",
    description: "Helper text shown when basic auth injection is selected",
  },
  "sandboxGroups.credFieldTemplate": {
    message: "Wertvorlage (optional)",
    description: "Form label for optional value template",
  },
  "sandboxGroups.credFieldTemplateHelp": {
    message: "Standard ist {token}. Verwende 1 bis 5 Vorkommen von {token}.",
    description: "Help text for injection template placeholder syntax",
  },
  "sandboxGroups.credFieldMethods": {
    message: "HTTP-Methoden",
    description: "Form label for allowed HTTP methods multi-select",
  },
  "sandboxGroups.credFieldMethodsHelp": {
    message: "Lass alle abgewählt, um jede HTTP-Methode zuzulassen.",
    description: "Help text for HTTP methods selection",
  },
  "sandboxGroups.credFieldPaths": {
    message: "Pfadmuster (eines pro Zeile)",
    description: "Form label for path patterns textarea",
  },
  "sandboxGroups.credFieldPathsHelp": {
    message: "Absolute Pfade ab /. Unterstützt Segment-Platzhalter * und **. Leer lassen für alle Pfade.",
    description: "Help text for path pattern syntax",
  },
  "sandboxGroups.credFieldEnabled": {
    message: "Aktiviert",
    description: "Form label for credential enabled switch",
  },
  "sandboxGroups.credFieldEnabledHelp": {
    message: "Deaktivierte Zugangsdaten bleiben in der Gruppe, werden aber nicht eingefügt.",
    description: "Help text for credential enabled switch",
  },
  "sandboxGroups.credCreateSubmit": {
    message: "Zugangsdaten erstellen",
    description: "Submit button label for creating a credential",
  },
  "sandboxGroups.credEditSubmit": {
    message: "Änderungen speichern",
    description: "Submit button label for editing a credential",
  },
  "sandboxGroups.credCreatedToast": {
    message: "Zugangsdaten {id} erstellt.",
    description: "Success toast after creating a credential",
  },
  "sandboxGroups.credUpdatedToast": {
    message: "Zugangsdaten {id} aktualisiert.",
    description: "Success toast after updating a credential",
  },
  "sandboxGroups.credDeletedToast": {
    message: "Zugangsdaten {id} gelöscht.",
    description: "Success toast after deleting a credential",
  },
  "sandboxGroups.credRestoredToast": {
    message: "{id} aus Version v{version} wiederhergestellt.",
    description: "Success toast after restoring a credential version",
  },
  "sandboxGroups.credCreateError": {
    message: "Zugangsdaten konnten nicht erstellt werden. Bitte prüfe deine Eingaben.",
    description: "Fallback error when creating a credential fails",
  },
  "sandboxGroups.credUpdateError": {
    message: "Zugangsdaten konnten nicht aktualisiert werden. Bitte versuche es erneut.",
    description: "Fallback error when updating a credential fails",
  },
  "sandboxGroups.credDeleteError": {
    message: "Zugangsdaten konnten nicht gelöscht werden. Bitte versuche es erneut.",
    description: "Fallback error when deleting a credential fails",
  },
  "sandboxGroups.credRestoreError": {
    message: "Version konnte nicht wiederhergestellt werden. Bitte versuche es erneut.",
    description: "Fallback error when restoring a credential version fails",
  },
  "sandboxGroups.credErrorExists": {
    message: 'Zugangsdaten mit der ID „{id}“ existieren in dieser Gruppe bereits.',
    description: "Inline error when creating a duplicate credential ID",
  },
  "sandboxGroups.credErrorBusy": {
    message: "In dieser Gruppe läuft bereits eine andere Aktualisierung. Bitte gleich erneut versuchen.",
    description: "Error when concurrent credential writes contend on the group lock",
  },
  "sandboxGroups.credErrorUnavailable": {
    message: "Der Zugangsdatenspeicher ist derzeit nicht verfügbar.",
    description: "Error when OpenBao credential storage is unconfigured or unreachable",
  },
  "sandboxGroups.credErrorVersionNotFound": {
    message: "Diese Version wird nicht mehr aufbewahrt.",
    description: "Error when restoring an evicted or unknown version",
  },
  "sandboxGroups.credVersionsTitle": {
    message: "Versionsverlauf · {id}",
    description: "Dialog title for credential version history",
  },
  "sandboxGroups.credVersionsDescription": {
    message: "Speichert die aktuelle Konfiguration sowie bis zu 3 verschiedene vorherige Konfigurationen.",
    description: "Dialog subtitle explaining version retention policy",
  },
  "sandboxGroups.credVersionsError": {
    message: "Versionsverlauf konnte nicht geladen werden. Bitte versuche es erneut.",
    description: "Error state when loading credential versions fails",
  },
  "sandboxGroups.credVersionsEmpty": {
    message: "Keine gespeicherten Versionen gefunden.",
    description: "Empty state for credential versions dialog",
  },
  "sandboxGroups.credVersionCurrent": {
    message: "Aktuell",
    description: "Badge marking the active credential version",
  },
  "sandboxGroups.credVersionRestoredFrom": {
    message: "Wiederhergestellt aus v{version}",
    description: "Badge showing which prior version was restored",
  },
  "sandboxGroups.credFingerprintLabel": {
    message: "Fingerabdruck",
    description: "Label before the truncated SHA-256 fingerprint",
  },
  "sandboxGroups.credUpdatedBy": {
    message: "von {actor}",
    description: "Actor attribution on a credential version row",
  },
  "sandboxGroups.credRestoreAction": {
    message: "Wiederherstellen",
    description: "Button label to restore a prior credential version",
  },
  "sandboxGroups.credRestoreConfirmTitle": {
    message: "{id} auf v{version} wiederherstellen?",
    description: "Confirmation dialog title when restoring a credential version",
  },
  "sandboxGroups.credRestoreConfirmDescription": {
    message: "Erstellt eine neue aktuelle Version für {id} mit Konfiguration und Geheimnis aus v{version}.",
    description: "Confirmation dialog body when restoring a credential version",
  },
  "sandboxGroups.credDeleteConfirmTitle": {
    message: "Zugangsdaten {id} löschen?",
    description: "Confirmation dialog title when deleting a credential",
  },
  "sandboxGroups.credDeleteConfirmDescription": {
    message: "Durch das Löschen von {id} werden das gespeicherte Geheimnis und alle Versionen endgültig entfernt.",
    description: "Confirmation dialog body when deleting a credential",
  },
  "sandboxGroups.credValIdRequired": {
    message: "Die Zugangsdaten-ID ist erforderlich.",
    description: "Validation error when credential ID is empty",
  },
  "sandboxGroups.credValIdInvalid": {
    message: "Verwende nur Buchstaben, Ziffern, Punkte, Unterstriche, Tilden und Bindestriche.",
    description: "Validation error when credential ID has disallowed characters",
  },
  "sandboxGroups.credValIdReserved": {
    message: 'Die IDs „.“, „..“ und Werte mit Präfix `rnd-` sind reserviert.',
    description: "Validation error when credential ID is reserved",
  },
  "sandboxGroups.credValDomainRequired": {
    message: "Die Zieldomain ist erforderlich.",
    description: "Validation error when domain is empty",
  },
  "sandboxGroups.credValDomainInvalid": {
    message: "Gib einen gültigen öffentlichen DNS-Hostnamen ohne Platzhalter, Ports oder Pfade ein.",
    description: "Validation error when domain format is invalid",
  },
  "sandboxGroups.credValDomainPrivate": {
    message: "Lokale, private und Cluster-Hostnamen sind nicht erlaubt.",
    description: "Validation error when domain is private or cluster-local",
  },
  "sandboxGroups.credValValueRequired": {
    message: "Der Geheimwert ist erforderlich.",
    description: "Validation error when secret value is empty on create",
  },
  "sandboxGroups.credValValueTooLong": {
    message: "Der Geheimwert darf höchstens {maxBytes} Bytes groß sein.",
    description: "Validation error when secret value exceeds 5 KiB",
  },
  "sandboxGroups.credValValueNul": {
    message: "Der Geheimwert darf keine NUL-Bytes enthalten.",
    description: "Validation error when secret value contains a NUL byte",
  },
  "sandboxGroups.credValInjectionNameRequired": {
    message: "Für diesen Injektionstyp ist ein Zielname erforderlich.",
    description: "Validation error when header/cookie/query name is missing",
  },
  "sandboxGroups.credValInjectionNameForbidden": {
    message: "Bei Basic-Authentifizierung darf kein Zielname angegeben werden.",
    description: "Validation error when basic injection specifies a name",
  },
  "sandboxGroups.credValInjectionHeaderForbidden": {
    message: "Routing- und Verbindungsheader können nicht injiziert werden.",
    description: "Validation error when a forbidden HTTP header is chosen",
  },
  "sandboxGroups.credValInjectionNameInvalid": {
    message: "Gib einen gültigen Zielnamen ein.",
    description: "Validation error when target name has invalid characters",
  },
  "sandboxGroups.credValInjectionTemplateCount": {
    message: "Die Vorlage muss 1 bis {max} Vorkommen von {token} enthalten.",
    description: "Validation error when template placeholder count is out of range",
  },
  "sandboxGroups.credValInjectionBasicFormat": {
    message: "Der gerenderte Wert für Basic-Auth muss das Format `username:password` haben.",
    description: "Validation error when basic auth rendered value lacks username:password",
  },
  "sandboxGroups.credValInjectionHeaderControl": {
    message: "Der gerenderte Header-Wert darf keine Zeilenumbrüche oder NUL-Bytes enthalten.",
    description: "Validation error when header value contains CR/LF/NUL",
  },
  "sandboxGroups.credValInjectionCookieInvalid": {
    message: "Der gerenderte Cookie-Wert darf keine Semikolons oder Zeilenumbrüche enthalten.",
    description: "Validation error when cookie value contains semicolons or CR/LF",
  },
  "sandboxGroups.credValPathsInvalid": {
    message: "Jeder Pfad muss mit / beginnen, darf kein ? oder # enthalten und nur ganze Segment-Platzhalter * oder ** verwenden.",
    description: "Validation error when a path pattern is invalid",
  },
};

export default deSandboxGroups;
