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
  "environments.renameAction": {
    message: "Umbenennen",
    description: "Environment overflow-menu item that opens the rename dialog",
  },
  "environments.deleteAction": {
    message: "Löschen",
    description:
      "Environment overflow-menu item that opens the delete confirmation, and the confirm button",
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
  "environments.renameFieldLabel": {
    message: "Name der Umgebung",
    description:
      'Accessible/visible label for the environment rename field. Deliberately more specific than environments.fieldName\'s bare "Name" (w4/134)',
  },
  "environments.renameTitle": {
    message: "Umgebung umbenennen",
    description: "Rename-environment dialog title",
  },
  "environments.renameSubmit": {
    message: "Speichern",
    description: "Rename-environment dialog submit button",
  },
  "environments.renameSuccess": {
    message: "Umgebung in „{name}“ umbenannt.",
    description: "Toast shown after an environment is renamed",
  },
  "environments.renameError": {
    message: "Umgebung konnte nicht in „{name}“ umbenannt werden.",
    description: "Toast shown when renaming an environment fails",
  },
  "environments.deleteConfirmTitle": {
    message: "Umgebung „{name}“ löschen?",
    description: "Delete-environment confirmation dialog title",
  },
  "environments.deleteConfirmBody": {
    message:
      "Ihre Umgebungsgruppen bleiben erhalten, werden aber nicht zugewiesen: Sie gehören nicht mehr zu dieser Umgebung oder diesem Projekt und bleiben unter Umgebungsgruppen. Diese Aktion kann nicht rückgängig gemacht werden.",
    description: "Delete-environment confirmation dialog body",
  },
  "environments.deleteConfirmBodyResources": {
    message:
      "Beim Löschen dieser Umgebung werden alle ihre Services, Datenbanken und Key-Value-Instanzen gelöscht: {resources}. Ihre Umgebungsgruppen bleiben erhalten, werden aber nicht zugewiesen: Sie gehören nicht mehr zu dieser Umgebung oder diesem Projekt und bleiben unter Umgebungsgruppen. Diese Aktion kann nicht rückgängig gemacht werden.",
    description:
      "Delete-environment confirmation body when services, databases or key value stores would be deleted (w4/m264, w6/m290, Render parity)",
  },
  "environments.deleteSuccess": {
    message: "Umgebung „{name}“ gelöscht.",
    description: "Toast shown after an environment is deleted",
  },
  "environments.deleteRefreshFailed": {
    message:
      "Umgebung „{name}“ gelöscht, aber die Liste konnte nicht aktualisiert werden. Versuche es erneut oder lade die Seite neu, um sie zu aktualisieren.",
    description:
      "Warning after the environment was deleted but re-reading the list failed; deletion must not be retried or reported as failed (w4/290)",
  },
  "environments.deleteError": {
    message: "Umgebung „{name}“ konnte nicht gelöscht werden.",
    description: "Toast shown when deleting an environment fails",
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
  "environments.settingsTitle": {
    message: "Einstellungen für „{name}“",
    description: "Environment ACL settings dialog title",
  },
  "environments.settingsDescription": {
    message:
      "Verwalte Berechtigungen, Isolation des privaten Netzwerks und eingehende IP-Regeln.",
    description: "Environment ACL settings dialog description",
  },
  "environments.protectedBadge": {
    message: "Geschützt",
    description: "Badge shown beside a protected environment name",
  },
  "environments.protectedLabel": {
    message: "Geschützte Umgebung",
    description: "Protected-status setting label",
  },
  "environments.protectedHint": {
    message:
      "Nur Workspace-Admins können die Ressourcen dieser Umgebung löschen, pausieren, fortsetzen oder ihren Wartungsmodus ein- und ausschalten.",
    description: "Protected-status setting explanation",
  },
  "environments.isolationLabel": {
    message: "Umgebungsübergreifende Verbindungen blockieren",
    description: "Private-network isolation setting label",
  },
  "environments.isolationProOnly": {
    message:
      "Netzwerkisolation erfordert den Pro-Tarif oder höher. Schalte sie hier aus, falls sie zuvor aktiviert war.",
    description: "Isolation setting explanation on a Hobby workspace (w8/m138)",
  },
  "environments.isolationHint": {
    message:
      "Verhindere, dass privater Netzwerkverkehr die Grenze dieser Umgebung überschreitet, auch zu ihren Postgres- und Key-Value-Instanzen. Deren externe URLs folgen weiterhin ihren IP-Regeln.",
    description: "Private-network isolation setting explanation",
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
  "environments.settingsSave": {
    message: "Speichern",
    description: "Environment ACL settings save button",
  },
  "environments.aclSaveSuccess": {
    message: "Einstellungen für „{name}“ gespeichert.",
    description: "Toast shown after an Environment ACL is saved",
  },
  "environments.aclSaveError": {
    message: "Einstellungen für „{name}“ konnten nicht gespeichert werden.",
    description: "Toast shown when an Environment ACL save fails",
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
};

export default deEnvironments;
