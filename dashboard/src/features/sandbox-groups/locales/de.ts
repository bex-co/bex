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
};

export default deSandboxGroups;
