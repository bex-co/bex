import type { TranslationEntry } from "@/i18n";

const enWorkspaces: Record<string, TranslationEntry> = {
  "workspaces.switcherEmpty": {
    message: "Select a workspace",
    description: "Switcher trigger label before any workspace is selected",
  },
  "workspaces.switcherLabel": {
    message: "Workspaces",
    description: "Switcher dropdown label above the workspace list",
  },
  "workspaces.switcherSettings": {
    message: "Workspace Settings",
    description: "Switcher menu item linking to /workspace/settings",
  },
  "workspaces.switcherNew": {
    message: "New Workspace",
    description: "Switcher menu item linking to /new/workspace",
  },
  "workspaces.newTitle": {
    message: "New Workspace",
    description: "New-workspace page document title",
  },
  "workspaces.planPickerLabel": {
    message: "Plan",
    description: "Accessible label for the plan-card radiogroup",
  },
  "workspaces.planHobbyName": {
    message: "Hobby",
    description: "Workspace plan card name",
  },
  "workspaces.planHobbyBilling": {
    message: "No workspace fee",
    description: "Workspace plan card billing label",
  },
  "workspaces.planHobbyDescription": {
    message: "1 member, up to 25 services, 5 Hobby workspaces per user.",
    description: "Workspace plan card description",
  },
  "workspaces.planProName": {
    message: "Pro",
    description: "Workspace plan card name",
  },
  "workspaces.planProBilling": {
    message: "No workspace fee",
    description: "Workspace plan card billing label",
  },
  "workspaces.planProDescription": {
    message: "Unlimited members and services.",
    description: "Workspace plan card description",
  },
  "workspaces.planScaleName": {
    message: "Scale",
    description: "Workspace plan card name",
  },
  "workspaces.planScaleBilling": {
    message: "No workspace fee",
    description: "Workspace plan card billing label",
  },
  "workspaces.planScaleDescription": {
    message: "Unlimited members and services, extra roles.",
    description: "Workspace plan card description",
  },
  "workspaces.planEnterpriseName": {
    message: "Enterprise",
    description: "Workspace plan card name",
  },
  "workspaces.planEnterpriseBilling": {
    message: "Custom terms",
    description: "Workspace plan card billing label",
  },
  "workspaces.planEnterpriseDescription": {
    message: "Custom limits and support.",
    description: "Workspace plan card description",
  },
  "workspaces.planUsageBillingNote": {
    message:
      "Service and datastore usage is billed separately by resource tier.",
    description: "Billing note below the workspace plan picker",
  },
  "workspaces.createTitle": {
    message: "Create a workspace",
    description: "/new/workspace card title",
  },
  "workspaces.createDescription": {
    message: "Give it a name and pick a plan.",
    description: "/new/workspace card description",
  },
  "workspaces.fieldName": {
    message: "Name",
    description: "Workspace name field label (shared by create + settings)",
  },
  "workspaces.fieldNamePlaceholder": {
    message: "e.g. acme-staging",
    description: "Workspace name field placeholder",
  },
  "workspaces.fieldNameError": {
    message:
      "Lowercase letters, numbers, and hyphens only, 1-30 characters, no leading/trailing hyphen.",
    description: "Workspace name validation error",
  },
  "workspaces.fieldPlan": {
    message: "Plan",
    description: "Workspace plan field label (create picker + settings badge)",
  },
  "workspaces.createErrorTitle": {
    message: "Couldn't create workspace",
    description: "/new/workspace inline error alert title",
  },
  "workspaces.createCancel": {
    message: "Cancel",
    description: "/new/workspace cancel button",
  },
  "workspaces.createSubmit": {
    message: "Create Workspace",
    description: "/new/workspace submit button",
  },
  "workspaces.createSuccess": {
    message: "Created {name}",
    description: "Toast on a successful workspace create",
  },
  "workspaces.createError": {
    message: "Couldn't create the workspace",
    description: "Fallback toast/inline message on a failed create",
  },
  "workspaces.settingsTitle": {
    message: "Workspace settings",
    description: "Workspace settings page and card title",
  },
  "workspaces.settingsDescription": {
    message: "Rename this workspace or review its plan and metadata.",
    description: "Workspace settings card description",
  },
  "workspaces.settingsEmpty": {
    message: "No workspace selected.",
    description: "Workspace settings page empty state",
  },
  "workspaces.settingsNavigation": {
    message: "Settings sections",
    description:
      "Accessible label for the workspace settings section navigation",
  },
  "workspaces.renameSubmit": {
    message: "Save",
    description: "Workspace rename form submit button",
  },
  "workspaces.renameErrorTitle": {
    message: "Couldn't rename workspace",
    description: "Workspace rename inline error alert title",
  },
  "workspaces.renameSuccess": {
    message: "Renamed to {name}",
    description: "Toast on a successful rename",
  },
  "workspaces.renameError": {
    message: "Couldn't rename the workspace",
    description: "Fallback toast/inline message on a failed rename",
  },
  "workspaces.fieldId": {
    message: "Workspace ID",
    description: "Workspace settings metadata field label",
  },
  "workspaces.fieldCreatedAt": {
    message: "Created",
    description: "Workspace settings metadata field label",
  },
  "workspaces.dangerZoneTitle": {
    message: "Danger Zone",
    description: "Workspace settings delete section title",
  },
  "workspaces.dangerZoneDescription": {
    message:
      "This will delete all of your workspace's resources and data. All services, datastores, and environment variables will be lost. This can't be undone.",
    description: "Workspace settings delete section description",
  },
  "workspaces.deleteConfirmLabel": {
    message: "Type {phrase} below to confirm.",
    description:
      "Body prompt naming the exact 'sudo delete workspace <name>' phrase (rendered bold by SudoCommandField)",
  },
  "workspaces.deleteErrorTitle": {
    message: "Couldn't delete workspace",
    description: "Delete danger-zone inline error alert title",
  },
  "workspaces.deleteSubmit": {
    message: "Delete Workspace",
    description: "Delete danger-zone submit button",
  },
  "workspaces.deleteSuccess": {
    message: "Deleted {name}",
    description: "Toast on a successful delete",
  },
  "workspaces.deleteError": {
    message: "Couldn't delete the workspace",
    description: "Fallback toast/inline message on a failed delete",
  },
  "workspaces.changePlanTrigger": {
    message: "Change plan",
    description:
      "Workspace settings plan-badge link opening the change-plan dialog",
  },
  "workspaces.changePlanTitle": {
    message: "Change plan",
    description: "Change-plan dialog title",
  },
  "workspaces.changePlanDescription": {
    message:
      "Pick a new plan for this workspace. No payment step — the plan changes immediately.",
    description: "Change-plan dialog description",
  },
  "workspaces.changePlanCancel": {
    message: "Cancel",
    description: "Change-plan dialog cancel button",
  },
  "workspaces.changePlanSubmit": {
    message: "Change Plan",
    description: "Change-plan dialog submit button",
  },
  "workspaces.changePlanErrorTitle": {
    message: "Couldn't change plan",
    description: "Change-plan dialog inline error alert title",
  },
  "workspaces.changePlanSuccess": {
    message: "Changed plan to {plan}",
    description: "Toast on a successful plan change",
  },
  "workspaces.changePlanError": {
    message: "Couldn't change the workspace plan",
    description: "Fallback toast/inline message on a failed plan change",
  },
  "workspaces.generalTitle": {
    message: "General",
    description: "Title of the general card on the workspace settings page",
  },
};

export default enWorkspaces;
