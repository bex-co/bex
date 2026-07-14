import type { TranslationEntry } from "@/i18n";

const enEnvironments: Record<string, TranslationEntry> = {
  "environments.heading": {
    message: "Environments",
    description: "Section heading above a project page's environments list",
  },
  "environments.newButton": {
    message: "New Environment",
    description: "Button that opens the new-environment dialog",
  },
  "environments.emptyBody": {
    message:
      "No environments yet. Create one (e.g. staging or production) to group this project's resources.",
    description: "Empty state shown when a project has no environments",
  },
  "environments.errorBody": {
    message: "Something went wrong loading environments. Try again shortly.",
    description: "Error state shown when the environments query fails",
  },
  "environments.resourceCount": {
    message: "{count} resource(s)",
    description:
      "Count of services+databases+key-value instances assigned to an environment, next to its name",
  },
  "environments.manageButton": {
    message: "Manage resources",
    description: "Button on an environment card that opens the manage-resources dialog",
  },
  "environments.moreActions": {
    message: "More actions",
    description: "Accessible label for an environment card's overflow (•••) menu",
  },
  "environments.renameAction": {
    message: "Rename",
    description: "Environment overflow-menu item that opens the rename dialog",
  },
  "environments.deleteAction": {
    message: "Delete",
    description:
      "Environment overflow-menu item that opens the delete confirmation, and the confirm button",
  },
  "environments.cardEmpty": {
    message: "No resources in this environment yet.",
    description: "Shown inside an environment card when it has no assigned resources",
  },
  "environments.createTitle": {
    message: "New Environment",
    description: "New-environment dialog title",
  },
  "environments.createDescription": {
    message:
      "Group a subset of this project's resources under a name, like staging or production.",
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
    message: "Cancel",
    description: "Cancel button shared by the environment dialogs",
  },
  "environments.createSubmit": {
    message: "Create Environment",
    description: "New-environment dialog submit button",
  },
  "environments.createSuccess": {
    message: 'Environment "{name}" created.',
    description: "Toast shown after an environment is created",
  },
  "environments.createError": {
    message: 'Failed to create environment "{name}".',
    description: "Toast shown when creating an environment fails",
  },
  "environments.renameTitle": {
    message: "Rename environment",
    description: "Rename-environment dialog title",
  },
  "environments.renameSubmit": {
    message: "Save",
    description: "Rename-environment dialog submit button",
  },
  "environments.renameSuccess": {
    message: 'Environment renamed to "{name}".',
    description: "Toast shown after an environment is renamed",
  },
  "environments.renameError": {
    message: 'Failed to rename environment to "{name}".',
    description: "Toast shown when renaming an environment fails",
  },
  "environments.deleteConfirmTitle": {
    message: 'Delete environment "{name}"?',
    description: "Delete-environment confirmation dialog title",
  },
  "environments.deleteConfirmBody": {
    message:
      "Its services, databases, and key-value stores stay in the project and keep running — they just lose this environment label. This action cannot be undone.",
    description: "Delete-environment confirmation dialog body",
  },
  "environments.deleteSuccess": {
    message: 'Environment "{name}" deleted.',
    description: "Toast shown after an environment is deleted",
  },
  "environments.deleteError": {
    message: 'Failed to delete environment "{name}".',
    description: "Toast shown when deleting an environment fails",
  },
  "environments.manageTitle": {
    message: 'Manage resources in "{name}"',
    description: "Manage-resources dialog title",
  },
  "environments.manageDescription": {
    message:
      "Check the resources that belong to this environment. Assigning a resource also adds it to this project.",
    description: "Manage-resources dialog description",
  },
  "environments.tabServices": {
    message: "Services",
    description: "Manage-resources dialog tab label for the services checklist",
  },
  "environments.tabDatabases": {
    message: "Databases",
    description: "Manage-resources dialog tab label for the databases checklist",
  },
  "environments.tabKeyValues": {
    message: "Key Value",
    description: "Manage-resources dialog tab label for the key-value checklist",
  },
  "environments.manageNoServices": {
    message: "This workspace has no services to assign yet.",
    description: "Manage-resources dialog empty state when the workspace has no services",
  },
  "environments.manageNoDatabases": {
    message: "This workspace has no databases to assign yet.",
    description: "Manage-resources dialog empty state when the workspace has no databases",
  },
  "environments.manageNoKeyValues": {
    message: "This workspace has no key-value instances to assign yet.",
    description: "Manage-resources dialog empty state when the workspace has no key-value instances",
  },
  "environments.manageSubmit": {
    message: "Save",
    description: "Manage-resources dialog submit button",
  },
  "environments.assignSuccess": {
    message: 'Services for "{name}" updated.',
    description: "Toast shown after an environment's services are updated",
  },
  "environments.assignError": {
    message: 'Failed to update services for "{name}".',
    description: "Toast shown when updating an environment's services fails",
  },
  "environments.assignDatabasesSuccess": {
    message: 'Databases for "{name}" updated.',
    description: "Toast shown after an environment's databases are updated",
  },
  "environments.assignDatabasesError": {
    message: 'Failed to update databases for "{name}".',
    description: "Toast shown when updating an environment's databases fails",
  },
  "environments.assignKeyValuesSuccess": {
    message: 'Key-value instances for "{name}" updated.',
    description: "Toast shown after an environment's key-value instances are updated",
  },
  "environments.assignKeyValuesError": {
    message: 'Failed to update key-value instances for "{name}".',
    description: "Toast shown when updating an environment's key-value instances fails",
  },
};

export default enEnvironments;
