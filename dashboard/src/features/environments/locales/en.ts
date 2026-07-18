import type { TranslationEntry } from "@/i18n";

const enEnvironments: Record<string, TranslationEntry> = {
  "environments.heading": {
    message: "Environments",
    description: "Section heading above a project page's environments list",
  },
  "environments.selectorLabel": {
    message: "Selected environment",
    description:
      "Accessible label for the Project overview Environment selector",
  },
  "environments.unassignedOption": {
    message: "Unassigned",
    description: "Project resources that belong to no Environment",
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
      "Count of visible services+databases+key-value instances assigned to an environment, next to its name",
  },
  "environments.manageButton": {
    message: "Manage resources",
    description:
      "Button on an environment card that opens the manage-resources dialog",
  },
  "environments.moreActions": {
    message: "More actions",
    description:
      "Accessible label for an environment card's overflow (•••) menu",
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
    description:
      "Shown inside an environment card when it has no assigned resources",
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
    description:
      "Manage-resources dialog tab label for the databases checklist",
  },
  "environments.tabKeyValues": {
    message: "Key Value",
    description:
      "Manage-resources dialog tab label for the key-value checklist",
  },
  "environments.manageNoServices": {
    message: "This workspace has no services to assign yet.",
    description:
      "Manage-resources dialog empty state when the workspace has no services",
  },
  "environments.manageNoDatabases": {
    message: "This workspace has no databases to assign yet.",
    description:
      "Manage-resources dialog empty state when the workspace has no databases",
  },
  "environments.manageNoKeyValues": {
    message: "This workspace has no key-value instances to assign yet.",
    description:
      "Manage-resources dialog empty state when the workspace has no key-value instances",
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
    description:
      "Toast shown after an environment's key-value instances are updated",
  },
  "environments.assignKeyValuesError": {
    message: 'Failed to update key-value instances for "{name}".',
    description:
      "Toast shown when updating an environment's key-value instances fails",
  },
  "environments.tabEnvGroups": {
    message: "Env Groups",
    description: "Manage-resources dialog tab label for environment groups",
  },
  "environments.manageNoEnvGroups": {
    message: "This workspace has no environment groups to assign yet.",
    description: "Environment-groups checklist empty state",
  },
  "environments.assignEnvGroupsSuccess": {
    message: 'Environment groups for "{name}" updated.',
    description: "Toast shown after environment-group membership is updated",
  },
  "environments.assignEnvGroupsError": {
    message: 'Failed to update environment groups for "{name}".',
    description:
      "Toast shown when environment-group membership fails to update",
  },
  "environments.settingsAction": {
    message: "All settings",
    description: "Environment overflow-menu action opening ACL settings",
  },
  "environments.settingsTitle": {
    message: 'Settings for "{name}"',
    description: "Environment ACL settings dialog title",
  },
  "environments.settingsDescription": {
    message:
      "Manage permissions, private-network isolation, and inbound IP rules.",
    description: "Environment ACL settings dialog description",
  },
  "environments.protectedBadge": {
    message: "Protected",
    description: "Badge shown beside a protected environment name",
  },
  "environments.protectedLabel": {
    message: "Protected environment",
    description: "Protected-status setting label",
  },
  "environments.protectedHint": {
    message:
      "Require an explicit sudo confirmation for destructive service actions.",
    description: "Protected-status setting explanation",
  },
  "environments.isolationLabel": {
    message: "Block cross-environment connections",
    description: "Private-network isolation setting label",
  },
  "environments.isolationHint": {
    message:
      "Prevent private network traffic from crossing this environment boundary.",
    description: "Private-network isolation setting explanation",
  },
  "environments.ipAllowListLabel": {
    message: "Inbound IP restrictions",
    description: "Environment inbound-IP allowlist heading",
  },
  "environments.ipAllowListHint": {
    message:
      "Only the listed CIDR ranges can reach this environment's public services, static sites, and datastores. A source must also pass each resource's own IP rules.",
    description: "Environment inbound-IP allowlist explanation",
  },
  "environments.ipAllowListOpen": {
    message:
      "No rules — public members of this environment will DENY all traffic. Add 0.0.0.0/0 and ::/0 to allow everything.",
    description: "Empty environment inbound-IP allowlist state (deny-all)",
  },
  "environments.ipAllowListRemove": {
    message: "Remove {cidr}",
    description: "Accessible label for an inbound-IP rule remove button",
  },
  "environments.ipAllowListRuleCIDR": {
    message: "CIDR block for rule {number}",
    description: "Accessible label for an existing inbound-IP rule CIDR",
  },
  "environments.ipAllowListRuleDescription": {
    message: "Description for rule {number}",
    description: "Accessible label for an existing inbound-IP rule description",
  },
  "environments.ipAllowListNewCIDR": {
    message: "New CIDR block",
    description: "Accessible label for a new inbound-IP rule CIDR",
  },
  "environments.ipAllowListNewDescription": {
    message: "New rule description",
    description: "Accessible label for a new inbound-IP rule description",
  },
  "environments.ipAllowListDescriptionPlaceholder": {
    message: "Description (optional)",
    description: "Placeholder for an inbound-IP rule description",
  },
  "environments.ipAllowListAdd": {
    message: "Add source",
    description: "Button adding an inbound-IP CIDR rule",
  },
  "environments.settingsSave": {
    message: "Save",
    description: "Environment ACL settings save button",
  },
  "environments.aclSaveSuccess": {
    message: 'Settings for "{name}" saved.',
    description: "Toast shown after an Environment ACL is saved",
  },
  "environments.aclSaveError": {
    message: 'Failed to save settings for "{name}".',
    description: "Toast shown when an Environment ACL save fails",
  },
  "environments.assignmentTitle": {
    message: "Project and Environment",
    description: "Shared create-form Project/Environment selector heading",
  },
  "environments.assignmentProject": {
    message: "Project",
    description: "Accessible label for a create-form Project selector",
  },
  "environments.assignmentProjectNone": {
    message: "No project",
    description: "Unassigned Project selector option",
  },
  "environments.assignmentEnvironment": {
    message: "Environment",
    description: "Accessible label for a create-form Environment selector",
  },
  "environments.assignmentEnvironmentNone": {
    message: "No environment",
    description: "Unassigned Environment selector option",
  },
  "environments.assignmentHint": {
    message:
      "Optional. Selecting an environment also adds the resource to its project.",
    description: "Shared create-form Environment-assignment hint",
  },
};

export default enEnvironments;
