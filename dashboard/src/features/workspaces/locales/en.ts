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
  "workspaces.switcherBilling": {
    message: "Billing",
    description: "Switcher menu item linking to /billing",
  },
  "workspaces.switcherSettings": {
    message: "Workspace Settings",
    description: "Switcher menu item linking to /workspace/settings",
  },
  "workspaces.switcherNew": {
    message: "+ New Workspace",
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
    message: "$0/mo",
    description:
      "Workspace plan card billing label (pricing.yaml hobby usdPerMonth)",
  },
  "workspaces.planHobbyDescription": {
    message: "For individuals getting started.",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planHobbyBulletMembers": {
    message: "1 member",
    description: "Hobby plan LimitsFor.MaxMembers bullet",
  },
  "workspaces.planHobbyBulletServices": {
    message: "Up to 25 services",
    description: "Hobby plan LimitsFor.MaxServices bullet",
  },
  "workspaces.planHobbyBulletWorkspaces": {
    message: "5 Hobby workspaces per user",
    description: "Hobby plan LimitsFor.MaxWorkspacesPerUser bullet",
  },
  "workspaces.planProName": {
    message: "Pro",
    description: "Workspace plan card name",
  },
  "workspaces.planProBilling": {
    message: "$17.50/mo",
    description:
      "Workspace plan card billing label (pricing.yaml pro usdPerMonth)",
  },
  "workspaces.planProDescription": {
    message: "For small teams shipping together.",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planProBulletMembers": {
    message: "Unlimited members",
    description: "Pro plan unlimited MaxMembers bullet",
  },
  "workspaces.planProBulletServices": {
    message: "Unlimited services",
    description: "Pro plan unlimited MaxServices bullet",
  },
  "workspaces.planScaleName": {
    message: "Scale",
    description: "Workspace plan card name",
  },
  "workspaces.planScaleBilling": {
    message: "$349.30/mo",
    description:
      "Workspace plan card billing label (pricing.yaml scale usdPerMonth)",
  },
  "workspaces.planScaleDescription": {
    message: "For growing teams that need extra roles.",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planScaleBulletMembers": {
    message: "Unlimited members",
    description: "Scale plan unlimited MaxMembers bullet",
  },
  "workspaces.planScaleBulletServices": {
    message: "Unlimited services",
    description: "Scale plan unlimited MaxServices bullet",
  },
  "workspaces.planScaleBulletRoles": {
    message: "Extra roles (Contributor, Viewer, Billing)",
    description: "Scale plan AllowedRoles beyond Pro bullet",
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
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planEnterpriseBulletLimits": {
    message: "Custom limits",
    description: "Enterprise plan custom-limits bullet",
  },
  "workspaces.planEnterpriseBulletSupport": {
    message: "Custom support",
    description: "Enterprise plan custom-support bullet",
  },
  "workspaces.planSelect": {
    message: "Select plan",
    description: "Unselected plan card action label",
  },
  "workspaces.planSelected": {
    message: "Plan selected",
    description: "Selected plan card action label",
  },
  "workspaces.planUsageBillingNote": {
    message:
      "Service and datastore usage is billed separately by resource tier.",
    description: "Billing note below the workspace plan picker",
  },
  "workspaces.createTitle": {
    message: "Create a workspace",
    description: "/new/workspace page heading",
  },
  "workspaces.createDescription": {
    message: "Choose its details, billing contact, plan, and payment method.",
    description: "/new/workspace page subtitle",
  },
  "workspaces.detailsTitle": {
    message: "Workspace Details",
    description: "/new/workspace details section heading",
  },
  "workspaces.billingEmail": {
    message: "Billing Email",
    description: "/new/workspace billing email label",
  },
  "workspaces.billingEmailHelp": {
    message: "Receipts and billing notices for this workspace go here.",
    description: "Editable paid-plan billing email help",
  },
  "workspaces.billingEmailHobbyHelp": {
    message: "For Hobby workspaces, billing email is your account email.",
    description: "Read-only Hobby billing email help",
  },
  "workspaces.billingEmailError": {
    message: "Enter a valid billing email.",
    description: "Billing email validation error",
  },
  "workspaces.fieldSlug": {
    message: "Workspace slug",
    description: "/new/workspace slug field label",
  },
  "workspaces.fieldSlugHelp": {
    message:
      "Used in URLs and resource names. Lowercase letters, numbers, and hyphens, 1–30 characters.",
    description: "/new/workspace slug helper text",
  },
  "workspaces.paymentTitle": {
    message: "Payment Method",
    description: "/new/workspace payment panel heading",
  },
  "workspaces.paymentDescription": {
    message:
      "Billing is unique to each workspace. This payment method belongs only to the new workspace.",
    description: "/new/workspace workspace-specific payment copy",
  },
  "workspaces.paymentRequired": {
    message: "A payment method is required for this workspace.",
    description: "Required payment policy copy",
  },
  "workspaces.paymentOptional": {
    message: "Add a payment method now, or continue without one.",
    description: "Optional payment policy copy",
  },
  "workspaces.paymentSelfHosted": {
    message: "Payment collection is disabled on this self-hosted installation.",
    description: "Billing-off create-flow copy",
  },
  "workspaces.paymentAdd": {
    message: "Add payment method",
    description: "Open Payment Element action",
  },
  "workspaces.paymentSave": {
    message: "Save payment method",
    description: "Confirm SetupIntent action",
  },
  "workspaces.paymentAdded": {
    message: "Payment method verified for this workspace.",
    description: "Successful payment setup state",
  },
  "workspaces.paymentError": {
    message: "We couldn't verify that payment method. Try again.",
    description: "Payment Element fallback error",
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
