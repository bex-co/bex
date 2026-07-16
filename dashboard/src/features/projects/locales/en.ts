import type { TranslationEntry } from "@/i18n";

const enProjects: Record<string, TranslationEntry> = {
  "projects.overviewTitle": {
    message: "Overview",
    description: "Page title of the Overview page (workspace homepage)",
  },
  "projects.projectsHeading": {
    message: "Projects",
    description: "Section heading above the Overview page's project grid",
  },
  "projects.ungroupedHeading": {
    message: "Ungrouped Resources",
    description: "Section heading above the Overview page's ungrouped resource table",
  },
  "projects.allResourcesHeading": {
    message: "All resources",
    description:
      "Heading above a project page's full resource table, below the environments section",
  },
  "projects.filterTypeLabel": {
    message: "Resource type",
    description: "Accessible label for project environment type filters",
  },
  "projects.filterAll": {
    message: "All",
    description: "All project environment resource types",
  },
  "projects.filterServices": {
    message: "Services",
    description: "Service resource filter",
  },
  "projects.filterDatabases": {
    message: "Databases",
    description: "Database resource filter",
  },
  "projects.filterKeyValues": {
    message: "Key Values",
    description: "Key-value resource filter",
  },
  "projects.filterEnvGroups": {
    message: "Env Groups",
    description: "Environment-group resource filter",
  },
  "projects.searchLabel": {
    message: "Search resources in {environment}",
    description: "Accessible project environment resource-search label",
  },
  "projects.searchPlaceholder": {
    message: "Search resources in {environment}",
    description: "Project environment resource-search placeholder",
  },
  "projects.noMatchesTitle": {
    message: "No matching resources",
    description: "Project environment filtered no-results title",
  },
  "projects.noMatchesBody": {
    message: "Try a different search or resource type.",
    description: "Project environment filtered no-results body",
  },
  "projects.eyebrow": {
    message: "Project",
    description: "Small uppercase label above a project page's name heading",
  },
  "projects.cardEmpty": {
    message: "No resources yet",
    description: "Project card status line when the project has no resources",
  },
  "projects.cardHealthy": {
    message: "All resources running",
    description: "Project card status line when every resource is healthy",
  },
  "projects.cardUnhealthy": {
    message: "{count} resource(s) need attention",
    description: "Project card status line when one or more resources are unhealthy",
  },
  "projects.newProjectCard": {
    message: "Create new project",
    description: "The dashed \"+ create project\" tile at the end of the project grid",
  },
  "projects.notFound": {
    message: "Project not found.",
    description: "Shown on a project page when the id doesn't match any project",
  },
  "projects.errorTitle": {
    message: "Couldn't load resources",
    description: "Error state heading on the unified Projects page",
  },
  "projects.errorBody": {
    message: "Something went wrong loading your resources. Try again shortly.",
    description: "Error state body on the unified Projects page",
  },
  "projects.emptyTitle": {
    message: "No resources yet",
    description: "Empty state heading on the unified Projects page",
  },
  "projects.emptyBody": {
    message: "Create a service, database, or key value store to get started.",
    description: "Empty state body on the unified Projects page",
  },
  "projects.newButton": {
    message: "New",
    description: "Button that opens the new-resource menu",
  },
  "projects.newService": {
    message: "New Service",
    description: "New-resource menu item that starts the service create wizard",
  },
  "projects.newDatabase": {
    message: "New Database",
    description: "New-resource menu item that opens the database create dialog",
  },
  "projects.newKeyValue": {
    message: "New Key Value",
    description: "New-resource menu item that starts the key value create form",
  },
  "projects.colName": {
    message: "Name",
    description: "Merged resource table column header",
  },
  "projects.colType": {
    message: "Type",
    description: "Merged resource table column header",
  },
  "projects.colStatus": {
    message: "Status",
    description: "Merged resource table column header",
  },
  "projects.colCreated": {
    message: "Created",
    description: "Merged resource table column header",
  },
  "projects.colActions": {
    message: "Actions",
    description: "Merged resource table column header (screen-reader only)",
  },
  "projects.typeService": {
    message: "Service",
    description: "Type badge for a service row in the merged resource table",
  },
  "projects.typeDatabase": {
    message: "Database",
    description: "Type badge for a database row in the merged resource table",
  },
  "projects.typeKeyValue": {
    message: "Key Value",
    description: "Type badge for a key value row in the merged resource table",
  },
  "projects.newProjectButton": {
    message: "New Project",
    description: "Button that opens the new-project dialog",
  },
  "projects.createTitle": {
    message: "New Project",
    description: "New-project dialog title",
  },
  "projects.createDescription": {
    message: "Group existing services, databases, and key value stores together.",
    description: "New-project dialog description",
  },
  "projects.fieldName": {
    message: "Name",
    description: "New-project dialog name field label",
  },
  "projects.fieldNamePlaceholder": {
    message: "my-project",
    description: "New-project dialog name field placeholder",
  },
  "projects.createCancel": {
    message: "Cancel",
    description: "New-project dialog cancel button",
  },
  "projects.createSubmit": {
    message: "Create Project",
    description: "New-project dialog submit button",
  },
  "projects.createSuccess": {
    message: 'Project "{name}" created.',
    description: "Toast shown after a project is created",
  },
  "projects.createError": {
    message: 'Failed to create project "{name}".',
    description: "Toast shown when creating a project fails",
  },
  "projects.editNameButton": {
    message: "Edit",
    description:
      "Accessible label for the project page's inline pencil-edit button, and the Settings page's Project Name \"Edit\" button",
  },
  "projects.renameTitle": {
    message: "Rename project",
    description: "Rename-project dialog title",
  },
  "projects.renameSubmit": {
    message: "Save",
    description: "Rename-project dialog submit button",
  },
  "projects.renameSuccess": {
    message: 'Project renamed to "{name}".',
    description: "Toast shown after a project is renamed",
  },
  "projects.renameError": {
    message: 'Failed to rename project to "{name}".',
    description: "Toast shown when renaming a project fails",
  },
  "projects.settingsTitle": {
    message: "Project settings",
    description: "Project settings page heading",
  },
  "projects.nameCardTitle": {
    message: "Project Name",
    description: "Project settings page's name card title",
  },
  "projects.nameCardDescription": {
    message: "A unique name for your project",
    description: "Project settings page's name card description",
  },
  "projects.deleteCardTitle": {
    message: "Delete Project",
    description: "Project settings page's danger-zone card title",
  },
  "projects.deleteCardDescription": {
    message:
      "Only the project grouping is deleted — its services, databases, and key value stores become unassigned, not deleted. This action cannot be undone.",
    description: "Project settings page's danger-zone card description",
  },
  "projects.deleteCardButton": {
    message: "Delete Project",
    description:
      "Project settings page's danger-zone card button, and the delete-confirmation dialog's confirm button",
  },
  "projects.deleteConfirmTitle": {
    message: 'Delete project "{name}"?',
    description: "Delete-project confirmation dialog title",
  },
  "projects.deleteConfirmBody": {
    message:
      "Its services, databases, and key value stores become unassigned — nothing is deleted.",
    description: "Delete-project confirmation dialog body",
  },
  "projects.deleteSuccess": {
    message: 'Project "{name}" deleted.',
    description: "Toast shown after a project is deleted",
  },
  "projects.deleteError": {
    message: 'Failed to delete project "{name}".',
    description: "Toast shown when deleting a project fails",
  },
  "projects.moveToProject": {
    message: "Move to project",
    description: "Row-actions submenu label for assigning a resource to a project",
  },
  "projects.removeFromProject": {
    message: "Remove from project",
    description: "Row-actions submenu item that unassigns a resource from its project",
  },
  "projects.moveSuccess": {
    message: '"{name}" moved to "{project}".',
    description: "Toast shown after moving a resource to a project",
  },
  "projects.moveError": {
    message: 'Failed to move "{name}".',
    description: "Toast shown when moving a resource to a project fails",
  },
  "projects.removeSuccess": {
    message: '"{name}" removed from its project.',
    description: "Toast shown after unassigning a resource from its project",
  },
  "projects.removeError": {
    message: 'Failed to remove "{name}" from its project.',
    description: "Toast shown when unassigning a resource fails",
  },
};

export default enProjects;
