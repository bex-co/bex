import type { TranslationEntry } from "@/i18n";

const deEnvironments: Record<string, TranslationEntry> = {
  "environments.heading": {
    message: "Umgebungen",
    description: "Section heading above a project page's environments list",
  },
  "environments.selectorLabel": {
    message: "Ausgewählte Umgebung",
    description:
      "Accessible label for the Project overview Environment selector",
  },
  "environments.unassignedOption": {
    message: "Nicht zugewiesen",
    description: "Project resources that belong to no Environment",
  },
  "environments.newButton": {
    message: "Neue Umgebung",
    description: "Button that opens the new-environment dialog",
  },
  "environments.emptyBody": {
    message:
      "Noch keine Umgebungen. Erstelle eine (z. B. staging oder production), um die Ressourcen dieses Projekts zu gruppieren.",
    description: "Empty state shown when a project has no environments",
  },
  "environments.errorBody": {
    message:
      "Beim Laden der Umgebungen ist etwas schiefgelaufen. Versuche es gleich noch einmal.",
    description: "Error state shown when the environments query fails",
  },
  "environments.refreshErrorBody": {
    message:
      "Die Umgebungen konnten nicht aktualisiert werden. Die zuletzt geladenen Daten werden angezeigt.",
    description:
      "Inline notice when a background refresh of environments fails while the previously loaded environments stay on screen",
  },
  "environments.resourceCount_one": {
    message: "{count} Ressource",
    description:
      "Count of visible services+databases+key-value instances assigned to an environment, next to its name (singular)",
  },
  "environments.resourceCount_other": {
    message: "{count} Ressourcen",
    description:
      "Count of visible services+databases+key-value instances assigned to an environment, next to its name (plural)",
  },
  "environments.manageButton": {
    message: "Ressourcen verwalten",
    description:
      "Button on an environment card that opens the manage-resources dialog",
  },
  "environments.moreActions": {
    message: "Weitere Aktionen",
    description:
      "Accessible label for an environment card's overflow (•••) menu",
  },
  "environments.cardEmpty": {
    message: "Noch keine Ressourcen in dieser Umgebung.",
    description:
      "Shown inside an environment card when it has no assigned resources",
  },
  "environments.createTitle": {
    message: "Neue Umgebung",
    description: "New-environment dialog title",
  },
  "environments.createDescription": {
    message:
      "Gruppiere eine Auswahl der Ressourcen dieses Projekts unter einem Namen, z. B. staging oder production.",
    description: "New-environment dialog description",
  },
  "environments.fieldName": {
    message: "Name",
    description: "New-environment dialog name field label",
  },
  "environments.fieldNamePlaceholder": {
    message: "staging",
    description: "New-environment dialog name field placeholder",
  },
  "environments.cancel": {
    message: "Abbrechen",
    description: "Cancel button shared by the environment dialogs",
  },
  "environments.createSubmit": {
    message: "Umgebung erstellen",
    description: "New-environment dialog submit button",
  },
  "environments.createSuccess": {
    message: "Umgebung „{name}“ erstellt.",
    description: "Toast shown after an environment is created",
  },
  "environments.createRefreshFailed": {
    message:
      "Umgebung „{name}“ erstellt, aber die Liste konnte nicht aktualisiert werden. Versuche es erneut oder lade die Seite neu, um sie zu sehen.",
    description:
      "Warning after the environment was created but re-reading the list failed; creation must not be retried or reported as failed (w4/231)",
  },
  "environments.createError": {
    message: "Umgebung „{name}“ konnte nicht erstellt werden.",
    description: "Toast shown when creating an environment fails",
  },
  "environments.deleteSuccess": {
    message: "Umgebung {name} erfolgreich gelöscht.",
    description:
      "Toast shown after an environment is deleted (Render's words, w1/m683)",
  },
  "environments.deleteRefreshFailed": {
    message:
      "Umgebung „{name}“ gelöscht, aber die Liste konnte nicht aktualisiert werden. Versuche es erneut oder lade die Seite neu, um sie zu aktualisieren.",
    description:
      "Warning after the environment was deleted but re-reading the list failed; deletion must not be retried or reported as failed (w4/290)",
  },
  "environments.manageTitle": {
    message: "Ressourcen in „{name}“ verwalten",
    description: "Manage-resources dialog title",
  },
  "environments.manageDescription": {
    message:
      "Wähle die Ressourcen aus, die zu dieser Umgebung gehören. Beim Zuweisen einer Ressource wird sie auch diesem Projekt hinzugefügt.",
    description: "Manage-resources dialog description",
  },
  "environments.tabServices": {
    message: "Services",
    description: "Manage-resources dialog tab label for the services checklist",
  },
  "environments.tabDatabases": {
    message: "Datenbanken",
    description:
      "Manage-resources dialog tab label for the databases checklist",
  },
  "environments.tabKeyValues": {
    message: "Key Value",
    description:
      "Manage-resources dialog tab label for the key-value checklist",
  },
  "environments.manageNoServices": {
    message: "Dieser Workspace hat noch keine Services zum Zuweisen.",
    description:
      "Manage-resources dialog empty state when the workspace has no services",
  },
  "environments.manageNoDatabases": {
    message: "Dieser Workspace hat noch keine Datenbanken zum Zuweisen.",
    description:
      "Manage-resources dialog empty state when the workspace has no databases",
  },
  "environments.manageNoKeyValues": {
    message:
      "Dieser Workspace hat noch keine Key-Value-Instanzen zum Zuweisen.",
    description:
      "Manage-resources dialog empty state when the workspace has no key-value instances",
  },
  "environments.manageResourceIn": {
    message: "In {environment}",
    description:
      "Under a Manage resources row: the other environment the resource is in now ({environment} = its name)",
  },
  "environments.markerProtected": {
    message: "Geschützt",
    description:
      "Marker: the environment the resource would leave is protected",
  },
  "environments.markerIsolated": {
    message: "Isoliert",
    description:
      "Marker: the environment the resource would leave has network isolation",
  },
  "environments.markerAllowList": {
    message: "IP-Allowlist",
    description:
      "Marker: the environment the resource would leave restricts inbound IPs",
  },
  "environments.manageMovesOut": {
    message: "Wird aus {environment} verschoben: {resources}",
    description:
      "Before Save: the selected resources this save moves out of another environment ({resources} = comma-separated names)",
  },
  "environments.manageNeedsAdminToMove": {
    message: "Nur ein Admin kann sie aus {environment} verschieben.",
    description:
      "Why a Manage resources row is disabled: its current environment is admin-managed",
  },
  "environments.manageUnsaved": {
    message: "Nicht gespeicherte Änderungen",
    description:
      "Accessible description of a Manage resources tab whose selection differs from the saved membership",
  },
  "environments.manageSubmit": {
    message: "Speichern",
    description: "Manage-resources dialog submit button",
  },
  "environments.assignSuccess": {
    message: "Services für „{name}“ aktualisiert.",
    description: "Toast shown after an environment's services are updated",
  },
  "environments.strandedMoveTitle": {
    message: "Dieses Verschieben lässt verknüpfte Gruppen zurück",
    description:
      "Confirmation title shown before a move that strands an environment-scoped group link (w4/m238)",
  },
  "environments.strandedMoveBody": {
    message:
      "Diese Umgebungsgruppen bleiben mit dem verschobenen Service verknüpft, und ihre Variablen und Dateien gelten weiter für ihn, bis die Gruppe getrennt oder verschoben wird:",
    description:
      "Confirmation body: the stranded groups keep feeding the service after the move",
  },
  "environments.strandedMoveConfirm": {
    message: "Trotzdem verschieben",
    description: "Confirm button on the stranded-groups move confirmation",
  },
  "environments.strandedScopeWorkspace": {
    message: "Workspace",
    description:
      "Scope label for a stranded group that is scoped to the whole workspace",
  },
  "environments.strandedScopeUnknown": {
    message:
      "Verknüpfte Umgebungsgruppen konnten noch nicht geprüft werden – warte, bis die Liste geladen ist, und versuche es erneut.",
    description:
      "Toast shown when a move is attempted before the group/scope reads resolve",
  },
  "environments.assignError": {
    message: "Services für „{name}“ konnten nicht aktualisiert werden.",
    description: "Toast shown when updating an environment's services fails",
  },
  "environments.assignDatabasesSuccess": {
    message: "Datenbanken für „{name}“ aktualisiert.",
    description: "Toast shown after an environment's databases are updated",
  },
  "environments.assignDatabasesError": {
    message: "Datenbanken für „{name}“ konnten nicht aktualisiert werden.",
    description: "Toast shown when updating an environment's databases fails",
  },
  "environments.assignDatabasesRefreshFailed": {
    message:
      "Datenbanken für „{name}“ aktualisiert, aber die Liste konnte nicht aktualisiert werden. Versuche es erneut oder lade die Seite neu, um sie zu sehen.",
    description:
      "Warning after the database membership was saved but re-reading the environments list failed; the write must not be retried or reported as failed (w4/325)",
  },
  "environments.assignKeyValuesSuccess": {
    message: "Key-Value-Instanzen für „{name}“ aktualisiert.",
    description:
      "Toast shown after an environment's key-value instances are updated",
  },
  "environments.assignKeyValuesRefreshFailed": {
    message:
      "Key-Value-Instanzen für „{name}“ aktualisiert, aber die Liste konnte nicht aktualisiert werden. Versuche es erneut oder lade die Seite neu, um sie zu sehen.",
    description:
      "Warning after the key-value membership was saved but re-reading the environments list failed; the write must not be retried or reported as failed (w4/263)",
  },
  "environments.assignKeyValuesError": {
    message:
      "Key-Value-Instanzen für „{name}“ konnten nicht aktualisiert werden.",
    description:
      "Toast shown when updating an environment's key-value instances fails",
  },
  "environments.tabEnvGroups": {
    message: "Umgebungsgruppen",
    description: "Manage-resources dialog tab label for environment groups",
  },
  "environments.manageNoEnvGroups": {
    message: "Dieser Workspace hat noch keine Umgebungsgruppen zum Zuweisen.",
    description: "Environment-groups checklist empty state",
  },
  "environments.tabSandboxGroups": {
    message: "Sandbox-Gruppen",
    description:
      "Manage-resources dialog tab label for the workspace's sandbox groups",
  },
  "environments.manageNoSandboxGroups": {
    message: "Dieser Workspace hat noch keine Sandbox-Gruppen zum Zuweisen.",
    description: "Sandbox-groups checklist empty state",
  },
  "environments.assignEnvGroupsSuccess": {
    message: "Umgebungsgruppen für „{name}“ aktualisiert.",
    description: "Toast shown after environment-group membership is updated",
  },
  "environments.assignEnvGroupsPlacementUnread": {
    message:
      "Es konnte nicht bestätigt werden, wo die Gruppen für „{name}“ jetzt sind. Versuche es erneut oder lade die Seite neu, bevor du sie noch einmal verschiebst.",
    description:
      "Warning after an environment-group move failed and re-reading the groups' placement failed too; its action re-reads, it never moves again (w4/m206)",
  },
  "environments.assignEnvGroupsError": {
    message: "Umgebungsgruppen für „{name}“ konnten nicht aktualisiert werden.",
    description:
      "Toast shown when environment-group membership fails to update",
  },
  "environments.settingsAction": {
    message: "Alle Einstellungen",
    description: "Environment overflow-menu action opening ACL settings",
  },
  "environments.protectedBadge": {
    message: "Geschützt",
    description: "Badge shown beside a protected environment name",
  },
  "environments.ipAllowListLabel": {
    message: "Einschränkungen für eingehende IPs",
    description: "Environment inbound-IP allowlist heading",
  },
  "environments.ipAllowListHint": {
    message:
      "Nur die aufgeführten CIDR-Bereiche können die öffentlichen Services, Static Sites und Datastores dieser Umgebung erreichen. Eine Quelle muss außerdem die eigenen IP-Regeln jeder Ressource erfüllen.",
    description: "Environment inbound-IP allowlist explanation",
  },
  "environments.ipAllowListOpen": {
    message:
      "Keine Regeln – öffentliche Mitglieder dieser Umgebung lehnen jeden Traffic ab. Füge 0.0.0.0/0 und ::/0 hinzu, um alles zu erlauben.",
    description: "Empty environment inbound-IP allowlist state (deny-all)",
  },
  "environments.ipAllowListRemove": {
    message: "{cidr} entfernen",
    description: "Accessible label for an inbound-IP rule remove button",
  },
  "environments.ipAllowListRuleCIDR": {
    message: "CIDR-Block für Regel {number}",
    description: "Accessible label for an existing inbound-IP rule CIDR",
  },
  "environments.ipAllowListRuleDescription": {
    message: "Beschreibung für Regel {number}",
    description: "Accessible label for an existing inbound-IP rule description",
  },
  "environments.ipAllowListNewCIDR": {
    message: "Neuer CIDR-Block",
    description: "Accessible label for a new inbound-IP rule CIDR",
  },
  "environments.ipAllowListNewDescription": {
    message: "Beschreibung der neuen Regel",
    description: "Accessible label for a new inbound-IP rule description",
  },
  "environments.ipAllowListDescriptionPlaceholder": {
    message: "Beschreibung (optional)",
    description: "Placeholder for an inbound-IP rule description",
  },
  "environments.ipAllowListScalePlanOnly": {
    message:
      "Eingehende IP-Regeln für Umgebungen erfordern den Scale-Tarif oder höher. Bereits gesetzte Regeln bleiben wirksam und können zurückgesetzt werden.",
    description:
      "Hint replacing the editable IP rules below the Scale plan (w8/m137)",
  },
  "environments.ipAllowListResetAllowAll": {
    message: "Auf „Alle erlauben“ zurücksetzen",
    description:
      "Button resetting an environment's inbound IP rules to allow-all below the Scale plan (w8/m137)",
  },
  "environments.ipAllowListInvalid": {
    message: "Gib eindeutige, gültige IPv4- oder IPv6-CIDR-Blöcke ein.",
    description:
      "Inline validation when an environment allow list block is not a unique, valid CIDR",
  },
  "environments.ipAllowListAdd": {
    message: "Quelle hinzufügen",
    description: "Button adding an inbound-IP CIDR rule",
  },
  "environments.resourceCountLoading": {
    message: "Wird gezählt…",
    description:
      "Environment card header while its resource lists are still loading, instead of a count",
  },
  "environments.resourceCountUnknown": {
    message: "Anzahl nicht verfügbar",
    description:
      "Environment card header when a resource list has not answered, instead of a count",
  },
  "environments.resourcesUnavailable": {
    message:
      "Einige Ressourcen konnten nicht geladen werden, daher ist diese Umgebung möglicherweise unvollständig oder veraltet.",
    description:
      "Inline alert on a Project environment card when a resource list failed to load; offers Try again",
  },
  "environments.resourcesLoading": {
    message: "Ressourcen werden geladen…",
    description:
      "Environment card line while its resource lists are still loading, instead of the empty message",
  },
  "environments.assignmentTitle": {
    message: "Projekt",
    description: "Shared create-form Project/Environment selector heading",
  },
  "environments.assignmentProject": {
    message: "Projekt",
    description: "Accessible label for a create-form Project selector",
  },
  "environments.assignmentProjectNone": {
    message: "Kein Projekt",
    description: "Unassigned Project selector option",
  },
  "environments.assignmentEnvironment": {
    message: "Umgebung",
    description: "Accessible label for a create-form Environment selector",
  },
  "environments.assignmentEnvironmentNone": {
    message: "Keine Umgebung",
    description: "Unassigned Environment selector option",
  },
  "environments.assignmentSelectedLoading": {
    message: "Wird geladen…",
    description:
      "Label for a kept Project/Environment selection while its list is still loading",
  },
  "environments.assignmentSelectedUnavailable": {
    message: "Ausgewählt (Name nicht verfügbar)",
    description:
      "Label for a kept Project/Environment selection whose list failed to load",
  },
  "environments.assignmentReadError": {
    message:
      "Projekte oder Umgebungen konnten nicht geladen werden. Versuche es erneut; deine Auswahl bleibt erhalten.",
    description:
      "Inline alert when the create form's Project or Environment list fails to load; the selection is kept",
  },
  "environments.assignmentHint": {
    message:
      "Beim Erstellen einer Ressource filtert die Auswahl eines Projekts dessen Umgebungen. Wähle eine Umgebung, um die Ressource zu diesem Projekt hinzuzufügen. Bleibt „Keine Umgebung“ ausgewählt, wird die Ressource außerhalb jedes Projekts erstellt. Hat das Projekt noch keine Umgebung, erstelle zuerst eine auf der Seite des Projekts.",
    description: "Shared create-form Environment-assignment hint",
  },
  "environments.protectedPlacementAdminOnly": {
    message:
      "Nur ein Admin kann Ressourcen in einer geschützten Umgebung erstellen oder sie in oder aus ihr verschieben.",
    description:
      "Why a create or move control that targets (or leaves) a protected environment is disabled for a non-admin, and the message for the same server refusal (w3/m281)",
  },
  "environments.protectedOptionAdminOnly": {
    message: "{environment} (nur Admins)",
    description:
      "A protected environment in a create or move picker that a non-admin cannot choose ({environment} = its name)",
  },
  "environments.assignmentDescription": {
    message:
      "Fügen Sie es nach dem Erstellen einem Projekt und einer Umgebung hinzu.",
    description: "Shared create-form Project row description",
  },
  "environments.assignmentNoEnvironments": {
    message:
      "Dieses Projekt hat noch keine Umgebung. Legen Sie zuerst auf der Projektseite eine an.",
    description:
      "Shared create-form note when the chosen Project has no Environment to add the resource to",
  },
  "environments.assignmentPickEnvironment": {
    message:
      "Wählen Sie eine Umgebung, um die Ressource diesem Projekt hinzuzufügen. Mit „Keine Umgebung“ wird sie außerhalb jedes Projekts erstellt.",
    description:
      "Shared create-form note when a Project is chosen but No environment is: only an Environment adds the resource to the Project",
  },
  "environments.assignmentEnvironmentPickProject": {
    message: "Zuerst ein Projekt wählen",
    description:
      "Shared create-form Environment select while no Project is chosen (it is disabled until one is)",
  },
  "environments.protectedDescription": {
    message: "Geschützte Umgebung",
    description:
      "Line under a protected environment's section heading on the project page (Render's words, w1/m682)",
  },
  "environments.emptyTitle": {
    message: "{name} ist leer",
    description:
      "Heading of an environment section with no services, Postgres or Key Value; {name} is the environment (Render's words, w1/m682)",
  },
  "environments.emptyBodyCreate": {
    message: "Starte deine Umgebung, indem du einen neuen Service erstellst.",
    description:
      "An empty environment's sentence when the workspace has nothing to move into it (Render's words, w1/m682)",
  },
  "environments.emptyBodyCreateOrMove": {
    message:
      "Starte deine Umgebung, indem du einen neuen Service erstellst oder einen vorhandenen verschiebst.",
    description:
      "An empty environment's sentence when the workspace has resources that could move into it (Render's words, w1/m682)",
  },
  "environments.createNewService": {
    message: "Neuen Service erstellen",
    description:
      "An empty environment's button to the create flow with the project and environment chosen (Render's words, w1/m682)",
  },
  "environments.moveExistingServices": {
    message: "Vorhandene Services verschieben",
    description:
      "An empty environment's button that moves existing resources into it (Render's words, w1/m682)",
  },
  "environments.noEnvGroups": {
    message: "Diese Umgebung hat keine Umgebungsgruppen.",
    description:
      "An environment section's Env Groups view when the environment has none (Render's words, w1/m682)",
  },
  "environments.unassignedDescription": {
    message: "Ressourcen dieses Projekts, die in keiner Umgebung sind.",
    description:
      "Line under the project page's Unassigned section, which lists the project's resources in no environment (a bex section; Render requires an environment)",
  },
  "environments.settingsPageTitle": {
    message: "Umgebungseinstellungen",
    description:
      "Heading of an environment's settings page, /project/<id>/environment/<envId>/settings (Render's words, w1/m683)",
  },
  "environments.settingsNavigation": {
    message: "Bereiche der Umgebungseinstellungen",
    description:
      "Accessible name of the environment settings page's section navigation (w1/m683)",
  },
  "environments.settingsEdit": {
    message: "Bearbeiten",
    description:
      "Button on an environment settings card that makes its field editable (Render's words, w1/m683)",
  },
  "environments.settingsCardSave": {
    message: "Speichern",
    description:
      "Button that saves an environment settings card's change (Render's words, w1/m683)",
  },
  "environments.settingsUpdateError": {
    message:
      "Beim Aktualisieren der Umgebung ist ein Fehler aufgetreten. Bitte versuche es erneut.",
    description:
      "Inline error on an environment settings card when the save failed for a reason the server did not name (Render's words, w1/m683)",
  },
  "environments.nameCardTitle": {
    message: "Name",
    description:
      "Title of the environment settings page's Name card, and its navigation entry (Render's words, w1/m683)",
  },
  "environments.nameCardDescription": {
    message: "Ein eindeutiger Name für deine Umgebung",
    description:
      "Description of the environment settings page's Name card (Render's words, w1/m683)",
  },
  "environments.nameFieldLabel": {
    message: "Name der Umgebung",
    description:
      "Accessible label of the Name card's field (Render's words, w1/m683)",
  },
  "environments.nameRequired": {
    message: "Name der Umgebung ist erforderlich",
    description:
      "Error under the Name card's field when it is saved empty (Render's words, w1/m683)",
  },
  "environments.nameInUse": {
    message:
      "Der Name wird bereits von einer anderen Umgebung in diesem Projekt verwendet",
    description:
      "Error under the Name card's field when another environment of the project has the name (Render's words, w1/m683)",
  },
  "environments.permissionsCardTitle": {
    message: "Berechtigungen",
    description:
      "Title of the environment settings page's Permissions card, and its navigation entry (Render's words, w1/m683)",
  },
  "environments.permissionsDefaultBody": {
    message:
      "Standardmäßig können alle Teammitglieder Ressourcen löschen und pausieren, Services verschieben und das Netzwerk von Datenbanken verwalten.",
    description:
      "First paragraph of the Permissions card (Render's words, w1/m683)",
  },
  "environments.permissionsProtectedBody": {
    message:
      "Ist die Umgebung als {protectedEnvironment} markiert, können nur Admins Ressourcen löschen und pausieren, Services verschieben und das Netzwerk von Datenbanken verwalten.",
    description:
      "Second paragraph of the Permissions card; {protectedEnvironment} is a link whose text is environments.permissionsProtectedLink (Render's words, w1/m683)",
  },
  "environments.permissionsProtectedLink": {
    message: "geschützte Umgebung",
    description:
      "Link text inside environments.permissionsProtectedBody, to the projects and environments docs (Render's words, w1/m683)",
  },
  "environments.permissionsLegend": {
    message: "Berechtigungen der Umgebung",
    description:
      "Accessible name of the Permissions card's Default / Protected choice (Render's words, w1/m683)",
  },
  "environments.permissionsDefault": {
    message: "Standard",
    description:
      "The Permissions card's option for an unprotected environment (Render's words, w1/m683)",
  },
  "environments.permissionsDefaultHint": {
    message: "Für Staging und Entwicklung",
    description:
      "Line under the Permissions card's Default option (Render's words, w1/m683)",
  },
  "environments.permissionsProtected": {
    message: "Geschützt",
    description:
      "The Permissions card's option for a protected environment (Render's words, w1/m683)",
  },
  "environments.permissionsProtectedHint": {
    message: "Ideal für die Produktion",
    description:
      "Line under the Permissions card's Protected option (Render's words, w1/m683)",
  },
  "environments.removeRestrictionsTitle": {
    message: "Einschränkungen aufheben",
    description:
      "Title of the dialog that confirms unprotecting an environment, or allowing its cross-environment connections again (Render's words, w1/m683)",
  },
  "environments.removeProtectionBody": {
    message:
      "Wenn du die Einschränkungen für die Umgebung {name} aufhebst, können alle Teammitglieder destruktive Aktionen ausführen. Möchtest du das wirklich tun?",
    description:
      "Body of the dialog that confirms unprotecting an environment; {name} is the environment (Render's words, w1/m683)",
  },
  "environments.removeRestrictionsConfirm": {
    message: "Ja, Einschränkungen aufheben",
    description:
      "Confirm button that unprotects an environment, or allows its cross-environment connections again (Render's words, w1/m683)",
  },
  "environments.connectionsNavLabel": {
    message: "Verbindungen",
    description:
      "Navigation entry of the environment settings page's Cross-Environment Connections card (Render's words, w1/m683)",
  },
  "environments.connectionsCardTitle": {
    message: "Umgebungsübergreifende Verbindungen",
    description:
      "Title of the environment settings page's connections card (Render's words, w1/m683)",
  },
  "environments.connectionsBody": {
    message:
      "Verhindere, dass privater Netzwerkverkehr die Grenze der Umgebung überschreitet, und erlaube weiterhin Datenverkehr zwischen Services in dieser Umgebung.",
    description:
      "First paragraph of the Cross-Environment Connections card (Render's words, w1/m683)",
  },
  "environments.connectionsBodyOutside": {
    message:
      "Services in dieser Umgebung können nicht mit bex-Services außerhalb der Umgebung kommunizieren.",
    description:
      "Second paragraph of the Cross-Environment Connections card (Render's words, with bex for Render, w1/m683)",
  },
  "environments.connectionsLegend": {
    message: "Umgebungsübergreifende Verbindungen",
    description:
      "Accessible name of the Cross-Environment Connections card's Allowed / Blocked choice (w1/m683)",
  },
  "environments.connectionsAllowed": {
    message: "Erlaubt",
    description:
      "The Cross-Environment Connections card's option for an environment whose private traffic may cross its boundary (Render's words, w1/m683)",
  },
  "environments.connectionsAllowedHint": {
    message: "Standard für neue Umgebungen",
    description:
      "Line under the Cross-Environment Connections card's Allowed option (Render's words, w1/m683)",
  },
  "environments.connectionsBlocked": {
    message: "Blockiert",
    description:
      "The Cross-Environment Connections card's option for an isolated environment (Render's words, w1/m683)",
  },
  "environments.connectionsBlockedHint": {
    message: "Empfohlen für Produktionsumgebungen",
    description:
      "Line under the Cross-Environment Connections card's Blocked option (Render's words, w1/m683)",
  },
  "environments.connectionsPaywall": {
    message:
      "Blockiere umgebungsübergreifende Verbindungen mit einem Upgrade auf den {plan}-Tarif.",
    description:
      "Notice on the Cross-Environment Connections card below Pro; {plan} is the Pro plan's name (Render's words, w1/m683)",
  },
  "environments.upgrade": {
    message: "Upgraden",
    description:
      "Button or link that opens the change-plan dialog from an environment's plan gate (Render's words, w1/m683)",
  },
  "environments.isolateTitle": {
    message: "Verbindungen einschränken",
    description:
      "Title of the dialog that confirms blocking an environment's cross-environment connections, and its confirm button (Render's words, w1/m683)",
  },
  "environments.isolateDescription": {
    message:
      "Bestätige, dass du die Verbindungen für {environment} isolieren möchtest",
    description:
      "Description of the dialog that confirms blocking or allowing an environment's cross-environment connections; {environment} is <project> / <environment> (Render's words, w1/m683)",
  },
  "environments.isolateRestrictBody": {
    message:
      "Blockiere privaten Netzwerkverkehr von und zu deinen bex-Services außerhalb dieser Umgebung. Das kann Services stören, die auf umgebungsübergreifenden Datenverkehr angewiesen sind.",
    description:
      "First paragraph of the dialog that confirms blocking cross-environment connections (Render's words, with bex for Render, w1/m683)",
  },
  "environments.isolateRestrictRestart": {
    message:
      "Bestehende offene Netzwerkverbindungen sind nicht betroffen. Starte die Services in dieser Umgebung neu, damit alle bestehenden Verbindungen sicher geschlossen werden.",
    description:
      "Second paragraph of the dialog that confirms blocking cross-environment connections (Render's words, w1/m683)",
  },
  "environments.isolateRemoveBody": {
    message:
      "Privater Netzwerkverkehr von und zu deinen bex-Services außerhalb dieser Umgebung wird nicht mehr blockiert.",
    description:
      "Body of the dialog that confirms allowing cross-environment connections again (Render's words, with bex for Render, w1/m683)",
  },
  "environments.isolateQuestion": {
    message: "Möchtest du das wirklich tun?",
    description:
      "Last line of the dialog that confirms blocking or allowing cross-environment connections (Render's words, w1/m683)",
  },
  "environments.isolationBlockedToast": {
    message: "Umgebungsübergreifende Verbindungen für {name} blockiert.",
    description:
      "Toast after the section menu's toggle blocks an environment's cross-environment connections (Render's words, w1/m683)",
  },
  "environments.isolationAllowedToast": {
    message: "Umgebungsübergreifende Verbindungen für {name} erlaubt.",
    description:
      "Toast after the section menu's toggle allows an environment's cross-environment connections again (Render's words, w1/m683)",
  },
  "environments.menuBlockConnections": {
    message: "Umgebungsübergreifende Verbindungen blockieren",
    description:
      "The environment section's ••• menu toggle that isolates the environment (Render's words, w1/m683)",
  },
  "environments.menuBlockConnectionsHint": {
    message:
      "Verhindere, dass privater Netzwerkverkehr die Grenze der Umgebung überschreitet.",
    description:
      "Line under the ••• menu's Block cross-environment connections toggle (Render's words, w1/m683)",
  },
  "environments.menuUpgradeToUnlock": {
    message: "{upgrade}, um diese Funktion freizuschalten",
    description:
      "The ••• menu's plan gate on Block cross-environment connections below Pro; {upgrade} is a link whose text is environments.upgrade (Render's words, w1/m683)",
  },
  "environments.envGroupsCardTitle": {
    message: "Umgebungsgruppen",
    description:
      "Title of the environment settings page's Environment Groups card, and its navigation entry (Render's words, w1/m683)",
  },
  "environments.envGroupsNew": {
    message: "Neue Umgebungsgruppe",
    description:
      "Button on the Environment Groups card that creates an env group in the environment (Render's words, w1/m683)",
  },
  "environments.envGroupsSearch": {
    message: "Umgebungsgruppen durchsuchen",
    description:
      "Placeholder and accessible name of the Environment Groups card's search (Render's words, w1/m683)",
  },
  "environments.envGroupsNone": {
    message: "Keine Umgebungsgruppen gefunden",
    description:
      "The Environment Groups card when the environment has no env groups (Render's words, w1/m683)",
  },
  "environments.networkingCardTitle": {
    message: "Netzwerk",
    description:
      "Title of the environment settings page's Networking card (inbound IP rules), and its navigation entry (Render's words, w1/m683)",
  },
  "environments.ipAllowListMoveUp": {
    message: "{cidr} nach oben verschieben",
    description:
      "Accessible name of an environment IP rule's move-up button; {cidr} is the rule's CIDR block",
  },
  "environments.ipAllowListMoveDown": {
    message: "{cidr} nach unten verschieben",
    description:
      "Accessible name of an environment IP rule's move-down button; {cidr} is the rule's CIDR block",
  },
  "environments.ipAllowListAllowsAll": {
    message: "Offen für alle Quell-IPs",
    description:
      "The Networking card below Scale when the environment's rules are the allow-all pair",
  },
  "environments.ipAllowListSaved": {
    message: "Eingehende IP-Regeln für {name} gespeichert.",
    description:
      "Toast after the Networking card saves an environment's inbound IP rules",
  },
  "environments.deleteCardTitle": {
    message: "Umgebung löschen",
    description:
      "Title of the environment settings page's delete card, and its navigation entry (Render's words, w1/m683)",
  },
  "environments.deleteCardDescription": {
    message:
      "Diese Umgebung wird einschließlich ihrer Services endgültig gelöscht. Diese Aktion kann nicht rückgängig gemacht werden.",
    description:
      "Description of the Delete Environment card (Render's words, w1/m683)",
  },
  "environments.deleteLastEnvironment": {
    message:
      "Diese Umgebung kann nicht gelöscht werden, da Projekte mindestens eine Umgebung benötigen. Erwäge stattdessen, das {project} zu löschen.",
    description:
      "Notice on the Delete Environment card for a project's only environment; {project} is a link whose text is environments.deleteLastEnvironmentProject (Render's words, w1/m683)",
  },
  "environments.deleteLastEnvironmentProject": {
    message: "Projekt",
    description:
      "Link text inside environments.deleteLastEnvironment, to the project's settings (Render's words, w1/m683)",
  },
  "environments.deleteButton": {
    message: "Umgebung löschen",
    description:
      "The Delete Environment card's button, the confirmation dialog's title and its confirm button (Render's words, w1/m683)",
  },
  "environments.deleteDialogBody": {
    message:
      "Diese Umgebung wird zusammen mit allen ihren Services endgültig gelöscht. Wenn du Services behalten möchtest, verschiebe sie zuerst aus der Umgebung.",
    description:
      "Body of the dialog that confirms deleting an environment (Render's words, w1/m683)",
  },
  "environments.deleteDialogEnvGroupsKept": {
    message:
      "Ihre Umgebungsgruppen bleiben erhalten, werden aber nicht zugewiesen: Sie gehören nicht mehr zu dieser Umgebung oder diesem Projekt und bleiben unter Umgebungsgruppen.",
    description:
      "Added to the delete dialog's body when the environment holds env groups, which bex keeps (w1/m683)",
  },
  "environments.deleteFailed": {
    message:
      "Die Umgebung konnte nicht gelöscht werden. Bitte versuche es gleich noch einmal. Entschuldigung!",
    description:
      "Error in the delete dialog when the delete failed for a reason the server did not name (Render's words, w1/m683)",
  },
  "environments.deleteCardDenied": {
    message: "Löschen der Umgebung",
    description:
      "Title of the card that replaces Delete Environment for a member who may not delete the environment (Render's words, w1/m683)",
  },
  "environments.protectedDeleteAdminOnly": {
    message:
      "Nur ein Admin kann eine geschützte Umgebung löschen oder eine mit Netzwerkisolation oder eingehenden IP-Regeln.",
    description:
      "Why a non-Admin cannot delete an environment that is protected, isolated or has inbound IP rules (w1/m683)",
  },
};

export default deEnvironments;
