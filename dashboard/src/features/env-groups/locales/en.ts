import type { TranslationEntry } from "@/i18n";

const enEnvGroups: Record<string, TranslationEntry> = {
  "envGroups.pageTitle": {
    message: "Environment Groups",
    description: "Workspace env-groups page title",
  },
  "envGroups.pageDescription": {
    message: "Share environment variables and secret files across services.",
    description: "Workspace env-groups page subtitle",
  },
  "envGroups.newButton": {
    message: "New Environment Group",
    description: "Open the create env-group dialog",
  },
  "envGroups.createTitle": {
    message: "Create environment group",
    description: "Create env-group dialog title",
  },
  "envGroups.createDescription": {
    message:
      "Create the group first, then add variables, files, and linked services.",
    description: "Create env-group dialog description",
  },
  "envGroups.nameLabel": {
    message: "Group name",
    description: "Env-group name field label",
  },
  "envGroups.namePlaceholder": {
    message: "shared-production",
    description: "Env-group name placeholder",
  },
  "envGroups.invalidName": {
    message: "Enter a group name.",
    description: "Invalid env-group name message",
  },
  "envGroups.cancel": {
    message: "Cancel",
    description: "Cancel an env-group dialog",
  },
  "envGroups.createSubmit": {
    message: "Create Environment Group",
    description: "Create env-group submit button",
  },
  "envGroups.emptyTitle": {
    message: "No environment groups",
    description: "Workspace env-groups empty-state title",
  },
  "envGroups.emptyBody": {
    message:
      "Create a group to manage shared configuration before linking it to a service.",
    description: "Workspace env-groups empty-state body",
  },
  "envGroups.varCount": {
    message: "{count} variable(s)",
    description: "Env-group variable count",
  },
  "envGroups.fileCount": {
    message: "{count} secret file(s)",
    description: "Env-group secret-file count",
  },
  "envGroups.serviceCount": {
    message: "{count} linked service(s)",
    description: "Env-group linked-service count",
  },
  "envGroups.unavailableTitle": {
    message: "Environment groups unavailable",
    description: "Secret store unavailable state title",
  },
  "envGroups.unavailableBody": {
    message: "The secret store isn't configured for this deployment.",
    description: "Secret store unavailable state body",
  },
  "envGroups.forbiddenTitle": {
    message: "Not authorized",
    description: "Env-group forbidden state title",
  },
  "envGroups.forbiddenBody": {
    message: "You don't have permission to view or manage environment groups.",
    description: "Env-group forbidden state body",
  },
  "envGroups.genericTitle": {
    message: "Couldn't load environment groups",
    description: "Env-group generic query error title",
  },
  "envGroups.genericBody": {
    message: "Something went wrong. Please try again.",
    description: "Env-group generic query error body",
  },
  "envGroups.errorTitle": {
    message: "Couldn't load group contents",
    description: "Env-group editor error title",
  },
  "envGroups.errorBody": {
    message: "Something went wrong. Please try again.",
    description: "Env-group editor error body",
  },
  "envGroups.notFoundTitle": {
    message: "Environment group not found",
    description: "Missing env-group detail title",
  },
  "envGroups.notFoundBody": {
    message: "No environment group exists with id {id}.",
    description: "Missing env-group detail body",
  },
  "envGroups.backToList": {
    message: "Back to environment groups",
    description: "Detail-page back button label",
  },
  "envGroups.renameButton": {
    message: "Rename",
    description: "Open rename env-group dialog",
  },
  "envGroups.renameTitle": {
    message: "Rename environment group",
    description: "Rename env-group dialog title",
  },
  "envGroups.renameDescription": {
    message: "Renaming keeps the group's variables, files, and service links.",
    description: "Rename env-group dialog description",
  },
  "envGroups.renameSubmit": {
    message: "Save name",
    description: "Rename env-group submit button",
  },
  "envGroups.deleteButton": {
    message: "Delete",
    description: "Open delete env-group dialog",
  },
  "envGroups.deleteTitle": {
    message: "Delete {name}?",
    description: "Delete env-group dialog title",
  },
  "envGroups.deleteDescription": {
    message:
      "This permanently removes the group and unlinks it from every service.",
    description: "Delete env-group dialog warning",
  },
  "envGroups.deletePrompt": {
    message: "Type {id} to confirm.",
    description: "Delete env-group typed-confirm prompt",
  },
  "envGroups.deleteConfirm": {
    message: "Delete Environment Group",
    description: "Delete env-group confirm button",
  },
  "envGroups.varsTitle": {
    message: "Environment Variables",
    description: "Env-group variable editor title",
  },
  "envGroups.varsDescription": {
    message: "Values are encrypted and applied to every linked service.",
    description: "Env-group variable editor description",
  },
  "envGroups.varsEmptyTitle": {
    message: "No environment variables",
    description: "Env-group variable editor empty title",
  },
  "envGroups.varsEmptyBody": {
    message: "Add a variable now; the group doesn't need to be linked first.",
    description: "Env-group variable editor empty body",
  },
  "envGroups.varDeleteConfirmBody": {
    message: "Every linked service will redeploy without this variable.",
    description: "Delete env-group variable warning",
  },
  "envGroups.filesTitle": {
    message: "Secret Files",
    description: "Env-group secret-file editor title",
  },
  "envGroups.filesDescription": {
    message:
      "Encrypted files are mounted into every linked service at /etc/secrets.",
    description: "Env-group secret-file editor description",
  },
  "envGroups.filesEmptyTitle": {
    message: "No secret files",
    description: "Env-group secret-file editor empty title",
  },
  "envGroups.filesEmptyBody": {
    message: "Add a file now; the group doesn't need to be linked first.",
    description: "Env-group secret-file editor empty body",
  },
  "envGroups.fileDeleteConfirmBody": {
    message: "Every linked service will redeploy without this file.",
    description: "Delete env-group secret-file warning",
  },
  "envGroups.servicesTitle": {
    message: "Linked Services",
    description: "Env-group linked-services card title",
  },
  "envGroups.servicesDescription": {
    message: "Linking or unlinking redeploys the affected service.",
    description: "Env-group linked-services card description",
  },
  "envGroups.selectService": {
    message: "Select a service",
    description: "Link-service selector placeholder",
  },
  "envGroups.linkButton": {
    message: "Link Service",
    description: "Link selected service to env group",
  },
  "envGroups.unlinkButton": {
    message: "Unlink",
    description: "Unlink service from env group",
  },
  "envGroups.noLinkedServices": {
    message:
      "This group isn't linked to any services yet. It is still fully editable.",
    description: "No linked services state",
  },
  "envGroups.servicesLoadError": {
    message:
      "Couldn't load workspace services. Existing links remain available below.",
    description: "Linked-services inventory error",
  },
  "envGroups.rolloutNote": {
    message: "Linked services are redeploying to apply the change.",
    description: "Env-group write rollout toast detail",
  },
  "envGroups.createSuccess": {
    message: "Created {name}",
    description: "Env-group create success toast",
  },
  "envGroups.createError": {
    message: "Couldn't create {name}",
    description: "Env-group create error toast",
  },
  "envGroups.renameSuccess": {
    message: "Renamed to {name}",
    description: "Env-group rename success toast",
  },
  "envGroups.renameError": {
    message: "Couldn't rename the group",
    description: "Env-group rename error toast",
  },
  "envGroups.deleteSuccess": {
    message: "Environment group deleted",
    description: "Env-group delete success toast",
  },
  "envGroups.deleteError": {
    message: "Couldn't delete the group",
    description: "Env-group delete error toast",
  },
  "envGroups.linkSuccess": {
    message: "Service linked",
    description: "Env-group link success toast",
  },
  "envGroups.linkError": {
    message: "Couldn't link the service",
    description: "Env-group link error toast",
  },
  "envGroups.unlinkSuccess": {
    message: "Service unlinked",
    description: "Env-group unlink success toast",
  },
  "envGroups.unlinkError": {
    message: "Couldn't unlink the service",
    description: "Env-group unlink error toast",
  },
  "envGroups.varSaveSuccess": {
    message: "Saved {key}",
    description: "Env-group variable save success toast",
  },
  "envGroups.varsSaveSuccess": {
    message: "Environment variables saved",
    description: "Env-group replace-all variables success toast",
  },
  "envGroups.varsSaveError": {
    message: "Couldn't save environment variables",
    description: "Env-group replace-all variables error toast",
  },
  "envGroups.varSaveError": {
    message: "Couldn't save {key}",
    description: "Env-group variable save error toast",
  },
  "envGroups.varDeleteSuccess": {
    message: "Removed {key}",
    description: "Env-group variable delete success toast",
  },
  "envGroups.varDeleteError": {
    message: "Couldn't remove {key}",
    description: "Env-group variable delete error toast",
  },
  "envGroups.fileSaveSuccess": {
    message: "Saved {name}",
    description: "Env-group file save success toast",
  },
  "envGroups.fileSaveError": {
    message: "Couldn't save {name}",
    description: "Env-group file save error toast",
  },
  "envGroups.fileDeleteSuccess": {
    message: "Removed {name}",
    description: "Env-group file delete success toast",
  },
  "envGroups.fileDeleteError": {
    message: "Couldn't remove {name}",
    description: "Env-group file delete error toast",
  },
};

export default enEnvGroups;
