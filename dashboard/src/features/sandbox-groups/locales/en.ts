import type { TranslationEntry } from "@/i18n";

const enSandboxGroups: Record<string, TranslationEntry> = {
  "sandboxGroups.createTitle": {
    message: "New Sandbox Group",
    description:
      "Title of the new-sandbox-group dialog and its entry in the + New menus",
  },
  "sandboxGroups.createDescription": {
    message:
      "A sandbox group caps how many of its sandboxes can be active at once. Group names must be unique within your workspace.",
    description: "New-sandbox-group dialog description",
  },
  "sandboxGroups.fieldName": {
    message: "Name",
    description: "New-sandbox-group dialog name field label",
  },
  "sandboxGroups.fieldNamePlaceholder": {
    message: "my-sandbox-group",
    description: "Placeholder for the new sandbox group's name",
  },
  "sandboxGroups.fieldRegion": {
    message: "Region",
    description:
      "Label of the region row in the created sandbox group's confirmation",
  },
  "sandboxGroups.createCancel": {
    message: "Cancel",
    description: "Cancel button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createSubmit": {
    message: "Create Sandbox Group",
    description: "Submit button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createError": {
    message: "Couldn't create sandbox group {name}. Please try again.",
    description: "Toast shown when creating a sandbox group fails",
  },
  "sandboxGroups.errorNameTaken": {
    message:
      'A sandbox group named "{name}" already exists in this workspace. Choose a different name.',
    description:
      "Inline error under the name field when the group name is already taken",
  },
  "sandboxGroups.errorNameInvalid": {
    message:
      "A group name must be 1 to {max} characters, with no leading or trailing space and no control characters.",
    description: "Inline error under the name field when the name is refused",
  },
  "sandboxGroups.errorLimitReached_one": {
    message: "This workspace already has the maximum of {count} sandbox group.",
    description:
      "Inline alert when the workspace is at its sandbox group limit (singular)",
  },
  "sandboxGroups.errorLimitReached_other": {
    message:
      "This workspace already has the maximum of {count} sandbox groups.",
    description:
      "Inline alert when the workspace is at its sandbox group limit",
  },
  "sandboxGroups.errorLimitReachedGeneric": {
    message: "This workspace has reached its sandbox group limit.",
    description:
      "Inline alert for the sandbox group limit when the server names no number",
  },
  "sandboxGroups.errorLimitTitle": {
    message: "Sandbox group limit reached",
    description: "Heading of the inline sandbox-group-limit alert",
  },
  "sandboxGroups.createdTitle": {
    message: "Sandbox group created",
    description: "Dialog title once a sandbox group has been created",
  },
  "sandboxGroups.createdDescription": {
    message:
      "{name} is ready. Pass its ID as sandboxGroupId when you create a sandbox in it.",
    description: "Dialog description once a sandbox group has been created",
  },
  "sandboxGroups.createdDone": {
    message: "Done",
    description: "Button that closes the created-sandbox-group confirmation",
  },
  "sandboxGroups.fieldId": {
    message: "ID",
    description: "Label of the new sandbox group's ID in the confirmation",
  },
  "sandboxGroups.fieldDefault": {
    message: "Default group",
    description:
      "Label of the confirmation row that says whether the group is the workspace's default",
  },
  "sandboxGroups.defaultYes": {
    message: "Yes",
    description: "The created group is the workspace's default sandbox group",
  },
  "sandboxGroups.defaultNo": {
    message: "No",
    description:
      "The created group is not the workspace's default sandbox group",
  },
  "sandboxGroups.fieldConcurrencyLimit": {
    message: "Concurrency limit",
    description:
      "Label of the confirmation row with the most active sandboxes the group allows",
  },
  "sandboxGroups.copyId": {
    message: "Copy sandbox group ID",
    description: "Accessible label of the copy button next to the group's ID",
  },
  "sandboxGroups.copied": {
    message: "Sandbox group ID copied",
    description: "Toast after copying the sandbox group ID",
  },
  "sandboxGroups.copyError": {
    message: "Couldn't copy the sandbox group ID",
    description: "Toast when copying the sandbox group ID fails",
  },
};

export default enSandboxGroups;
