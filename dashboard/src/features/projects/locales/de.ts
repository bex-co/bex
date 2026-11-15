import type { TranslationEntry } from "@/i18n";

const deProjects: Record<string, TranslationEntry> = {
  "projects.overviewTitle": {
    message: "Übersicht",
    description: "Page title of the Overview page (workspace homepage)",
  },
  "projects.projectsHeading": {
    message: "Projekte",
    description: "Section heading above the Overview page's project grid",
  },
  "projects.ungroupedHeading": {
    message: "Nicht gruppierte Ressourcen",
    description:
      "Section heading above the Overview page's ungrouped resource table",
  },
  "projects.allResourcesHeading": {
    message: "Alle Ressourcen",
    description:
      "Heading above a project page's full resource table, below the environments section",
  },
  "projects.filterTypeLabel": {
    message: "Ressourcentyp",
    description: "Accessible label for project environment type filters",
  },
  "projects.filterAll": {
    message: "Alle",
    description: "All project environment resource types",
  },
  "projects.filterServices": {
    message: "Services",
    description: "Service resource filter",
  },
  "projects.filterDatabases": {
    message: "Datenbanken",
    description: "Database resource filter",
  },
  "projects.filterKeyValues": {
    message: "Key Values",
    description: "Key-value resource filter",
  },
  "projects.filterEnvGroups": {
    message: "Umgebungsgruppen",
    description: "Environment-group resource filter",
  },
  "projects.searchLabel": {
    message: "Ressourcen in {environment} suchen",
    description: "Accessible project environment resource-search label",
  },
  "projects.searchPlaceholder": {
    message: "Ressourcen in {environment} suchen",
    description: "Project environment resource-search placeholder",
  },
  "projects.noMatchesTitle": {
    message: "Keine passenden Ressourcen",
    description: "Project environment filtered no-results title",
  },
  "projects.noMatchesBody": {
    message: "Versuche eine andere Suche oder einen anderen Ressourcentyp.",
    description: "Project environment filtered no-results body",
  },
  "projects.eyebrow": {
    message: "Projekt",
    description: "Small uppercase label above a project page's name heading",
  },
  "projects.cardEmpty": {
    message: "Noch keine Ressourcen",
    description: "Project card status line when the project has no resources",
  },
  "projects.cardHealthy": {
    message: "Alle Ressourcen laufen",
    description: "Project card status line when every resource is healthy",
  },
  "projects.cardInProgress_one": {
    message: "{count} Ressource in Bearbeitung",
    description:
      "Project card status line when resources are provisioning or deploying without a failure (singular)",
  },
  "projects.cardInProgress_other": {
    message: "{count} Ressourcen in Bearbeitung",
    description:
      "Project card status line when resources are provisioning or deploying without a failure (plural)",
  },
  "projects.cardUnhealthy_one": {
    message: "{count} Ressource braucht Aufmerksamkeit",
    description: "Project card status line when one resource is unhealthy",
  },
  "projects.cardUnhealthy_other": {
    message: "{count} Ressourcen brauchen Aufmerksamkeit",
    description:
      "Project card status line when more than one resource is unhealthy",
  },
  "projects.newProjectCard": {
    message: "Neues Projekt erstellen",
    description:
      'The dashed "+ create project" tile at the end of the project grid',
  },
  "projects.errorTitle": {
    message: "Ressourcen konnten nicht geladen werden",
    description: "Error state heading on the unified Projects page",
  },
  "projects.errorBody": {
    message:
      "Beim Laden deiner Ressourcen ist etwas schiefgelaufen. Versuche es in Kürze erneut.",
    description: "Error state body on the unified Projects page",
  },
  "projects.emptyTitle": {
    message: "Noch keine Ressourcen",
    description: "Empty state heading on the unified Projects page",
  },
  "projects.emptyBody": {
    message:
      "Erstelle einen Service, eine Datenbank oder einen Key-Value-Store, um loszulegen.",
    description: "Empty state body on the unified Projects page",
  },
  "projects.emptyCreateService": {
    message: "Service erstellen",
    description: "Primary action of the empty Projects page (ADR075 D1)",
  },
  "projects.newButton": {
    message: "Neu",
    description: "Button that opens the new-resource menu",
  },
  "projects.newService": {
    message: "Neuer Service",
    description: "New-resource menu item that starts the service create wizard",
  },
  "projects.newDatabase": {
    message: "Neue Datenbank",
    description: "New-resource menu item that opens the database create dialog",
  },
  "projects.newKeyValue": {
    message: "Neuer Key Value",
    description: "New-resource menu item that starts the key value create form",
  },
  "projects.colName": {
    message: "Name",
    description: "Merged resource table column header",
  },
  "projects.colType": {
    message: "Typ",
    description: "Merged resource table column header",
  },
  "projects.colStatus": {
    message: "Status",
    description: "Merged resource table column header",
  },
  "projects.colCreated": {
    message: "Erstellt",
    description: "Merged resource table column header",
  },
  "projects.colRuntime": {
    message: "Runtime",
    description:
      "Authoritative runtime/product column in a Project resource table",
  },
  "projects.colRegion": {
    message: "Region",
    description:
      "Explicit installation placement column in a Project resource table",
  },
  "projects.colUpdated": {
    message: "Aktualisiert",
    description:
      "Authoritative resource update-time column in a Project resource table",
  },
  "projects.colActions": {
    message: "Aktionen",
    description: "Merged resource table column header (screen-reader only)",
  },
  "projects.typeService": {
    message: "Service",
    description: "Type badge for a service row in the merged resource table",
  },
  "projects.typeDatabase": {
    message: "Datenbank",
    description: "Type badge for a database row in the merged resource table",
  },
  "projects.typeKeyValue": {
    message: "Key Value",
    description: "Type badge for a key value row in the merged resource table",
  },
  "projects.typeEnvGroup": {
    message: "Umgebungsgruppe",
    description: "Type badge for an environment-group row",
  },
  "projects.metadataNotApplicable": {
    message: "Nicht zutreffend",
    description:
      "Accessible label for a status, runtime or region cell on an environment group row, which has none of them (w4/549)",
  },
  "projects.metadataUnavailable": {
    message: "Nicht verfügbar",
    description:
      "Accessible label for an unavailable runtime, region, status, or timestamp",
  },
  "projects.selectAllVisible": {
    message: "Alle sichtbaren Ressourcen auswählen",
    description: "Accessible label for the Project table header checkbox",
  },
  "projects.selectResource": {
    message: "{name} auswählen",
    description: "Accessible label for one Project resource row checkbox",
  },
  "projects.selectedCount": {
    message: "Ausgewählt: {count}",
    description: "Bulk action toolbar selection count",
  },
  "projects.moveTargetLabel": {
    message: "Ausgewählte Ressourcen in Umgebung verschieben",
    description: "Accessible label for the bulk Move target selector",
  },
  "projects.moveTargetPlaceholder": {
    message: "Umgebung auswählen",
    description: "Placeholder for the bulk Move target selector",
  },
  "projects.moveSelected": {
    message: "Verschieben",
    description:
      "Bulk action that moves selected resources to another Environment",
  },
  "projects.newProjectButton": {
    message: "Neues Projekt",
    description: "Button that opens the new-project dialog",
  },
  "projects.createTitle": {
    message: "Neues Projekt",
    description: "New-project dialog title",
  },
  "projects.createDescription": {
    message:
      "Fasse Services, Datenbanken und Key-Value-Stores zusammen. Jedes Projekt beginnt mit einer Umgebung.",
    description: "New-project dialog description",
  },
  "projects.fieldName": {
    message: "Name",
    description: "New-project dialog name field label",
  },
  "projects.fieldFirstEnvironment": {
    message: "Name der ersten Umgebung",
    description:
      "New-project dialog: name of the environment created with the project (w4/m264, Render parity)",
  },
  "projects.fieldNamePlaceholder": {
    message: "mein-projekt",
    description: "New-project dialog name field placeholder",
  },
  "projects.createCancel": {
    message: "Abbrechen",
    description: "New-project dialog cancel button",
  },
  "projects.createSubmit": {
    message: "Projekt erstellen",
    description: "New-project dialog submit button",
  },
  "projects.createSuccess": {
    message: "Projekt „{name}“ erstellt.",
    description: "Toast shown after a project is created",
  },
  "projects.createError": {
    message: "Projekt „{name}“ konnte nicht erstellt werden.",
    description: "Toast shown when creating a project fails",
  },
  "projects.editNameButton": {
    message: "Bearbeiten",
    description:
      "Accessible label for the project page's inline pencil-edit button, and the Settings page's Project Name \"Edit\" button",
  },
  "projects.renameFieldLabel": {
    message: "Projektname",
    description:
      'Accessible/visible label for the project rename field, on both the overview dialog and the Settings card. Deliberately more specific than projects.fieldName\'s bare "Name": these controls are identified by value alone otherwise (w4/134)',
  },
  "projects.renameTitle": {
    message: "Projekt umbenennen",
    description: "Rename-project dialog title",
  },
  "projects.renameSubmit": {
    message: "Speichern",
    description: "Rename-project dialog submit button",
  },
  "projects.renameSuccess": {
    message: "Projekt in „{name}“ umbenannt.",
    description: "Toast shown after a project is renamed",
  },
  "projects.renameRefreshFailed": {
    message:
      "Projekt in „{name}“ umbenannt, aber diese Seite konnte nicht aktualisiert werden. Versuche es erneut oder lade neu, um den gespeicherten Namen zu sehen.",
    description:
      "Warning after the rename saved but re-reading the project failed; the rename must not be retried or reported as failed (w4/m186)",
  },
  "projects.renameError": {
    message: "Projekt konnte nicht in „{name}“ umbenannt werden.",
    description: "Toast shown when renaming a project fails",
  },
  "projects.settingsTitle": {
    message: "Projekteinstellungen",
    description: "Project settings page heading",
  },
  "projects.nameCardTitle": {
    message: "Projektname",
    description: "Project settings page's name card title",
  },
  "projects.nameCardDescription": {
    message: "Ein eindeutiger Name für dein Projekt",
    description: "Project settings page's name card description",
  },
  "projects.deleteCardTitle": {
    message: "Projekt löschen",
    description: "Project settings page's danger-zone card title",
  },
  "projects.deleteCardDescription": {
    message:
      "Seine Umgebungen und alle zugehörigen Services, Datenbanken und Key-Value-Stores werden mit gelöscht. Seine Umgebungsgruppen werden keinem Projekt mehr zugeordnet. Diese Aktion kann nicht rückgängig gemacht werden.",
    description: "Project settings page's danger-zone card description",
  },
  "projects.deleteCardButton": {
    message: "Projekt löschen",
    description:
      "Project settings page's danger-zone card button, and the delete-confirmation dialog's confirm button",
  },
  "projects.deleteConfirmTitle": {
    message: "Projekt „{name}“ löschen?",
    description: "Delete-project confirmation dialog title",
  },
  "projects.deleteConfirmBody": {
    message:
      "Seine Umgebungen und deren Konfiguration werden gelöscht. Seine Umgebungsgruppen werden keinem Projekt mehr zugeordnet, aber nicht gelöscht.",
    description: "Delete-project confirmation dialog body",
  },
  "projects.deleteConfirmBodyResources": {
    message:
      "Beim Löschen dieses Projekts werden alle seine Umgebungen und deren Services, Datenbanken und Key-Value-Stores gelöscht: {resources}. Seine Umgebungsgruppen werden keinem Projekt mehr zugeordnet, aber nicht gelöscht. Diese Aktion kann nicht rückgängig gemacht werden.",
    description:
      "Delete-project confirmation body naming every service, database and key value store it deletes, as the environment delete does (w4/m264, w6/m290, w4/542)",
  },
  "projects.deleteSuccess": {
    message: "Projekt „{name}“ gelöscht.",
    description: "Toast shown after a project is deleted",
  },
  "projects.deleteError": {
    message: "Projekt „{name}“ konnte nicht gelöscht werden.",
    description: "Toast shown when deleting a project fails",
  },
  "projects.moveToProject": {
    message: "In Projekt verschieben",
    description:
      "Row-actions submenu label for assigning a resource to a project",
  },
  "projects.removeFromProject": {
    message: "Aus Projekt entfernen",
    description:
      "Row-actions submenu item that unassigns a resource from its project",
  },
  "projects.moveSuccess": {
    message: "„{name}“ nach „{project}“ verschoben.",
    description: "Toast shown after moving a resource to a project",
  },
  "projects.moveError": {
    message: "„{name}“ konnte nicht verschoben werden.",
    description: "Toast shown when moving a resource to a project fails",
  },
  "projects.removeSuccess": {
    message: "„{name}“ aus seinem Projekt entfernt.",
    description: "Toast shown after unassigning a resource from its project",
  },
  "projects.removeError": {
    message: "„{name}“ konnte nicht aus seinem Projekt entfernt werden.",
    description: "Toast shown when unassigning a resource fails",
  },
};

export default deProjects;
