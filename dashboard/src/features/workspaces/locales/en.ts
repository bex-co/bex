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
      "A unique key for the CLI and API. Lowercase letters, numbers, and hyphens, 1–30 characters.",
    description: "/new/workspace slug helper text",
  },
  "workspaces.fieldSlugError": {
    message:
      "Lowercase letters, numbers, and hyphens only, 1-30 characters, no leading/trailing hyphen.",
    description: "Workspace slug validation error",
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
  "workspaces.fieldDisplayNamePlaceholder": {
    message: "e.g. Acme Staging",
    description: "/new/workspace freeform name placeholder",
  },
  "workspaces.fieldNamePlaceholder": {
    message: "e.g. acme-staging",
    description: "Workspace name field placeholder",
  },
  "workspaces.fieldNameError": {
    message: "Enter a name of 1–100 characters without line breaks.",
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
  "workspaces.createCancelError": {
    message: "Couldn't cancel workspace creation. Try again.",
    description:
      "/new/workspace toast when cancelling the pending creation fails",
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
  "workspaces.creationLoadError": {
    message: "Couldn't load workspace setup. Refresh the page to try again.",
    description:
      "Inline message when the workspace-creation plan policy or a resumed setup attempt fails to load",
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
  "workspaces.ipRulesTitle": {
    message: "Inbound IP Rules",
    description:
      "Workspace Settings → Inbound IP Rules card title and section link (w9/m388)",
  },
  "workspaces.ipRulesDescription": {
    message:
      "Choose which IP addresses can reach every web service, static site, Postgres database and Key Value instance in this workspace. These rules apply on top of environment and service rules, so a source must be allowed at each level. Private services, background workers and cron jobs are not affected.",
    description:
      "Inbound IP Rules card description: scope and layering of workspace rules (Render's workspace-level inbound IP rules, w9/m388)",
  },
  "workspaces.ipRulesHint": {
    message:
      "Enter a CIDR block (e.g. 203.0.113.0/24) and press Add source. With no rules, all inbound traffic is blocked; 0.0.0.0/0 and ::/0 allow every IPv4 and IPv6 source.",
    description: "Hint above the editable workspace inbound IP rules (w9/m388)",
  },
  "workspaces.ipRulesBlocked": {
    message:
      "No rules: all inbound traffic to this workspace's services and datastores is blocked. Add 0.0.0.0/0 and ::/0 to allow all sources.",
    description:
      "Warning shown when the workspace rule list is empty (deny-all), saved or about to be saved (w9/m388)",
  },
  "workspaces.ipRulesOpen": {
    message: "Open to all source IPs",
    description:
      "Shown in the read-only view when the workspace rules restrict nothing (Render's allow-all default, w9/m388)",
  },
  "workspaces.ipRulesScalePlanOnly": {
    message:
      "Inbound IP rules for a workspace require the Scale plan or higher. Rules already set stay enforced and can be reset to allow all.",
    description:
      "Notice replacing the editable workspace rules below the Scale plan (w9/m388)",
  },
  "workspaces.ipRulesAdminOnly": {
    message: "Only workspace Admins can change these rules.",
    description:
      "Notice for a member who cannot edit the workspace inbound IP rules, and the forbidden-save explanation (w9/m388)",
  },
  "workspaces.ipRulesPlanRefused": {
    message:
      "This workspace's plan doesn't include inbound IP rules. Upgrade to Scale or higher to restrict traffic; you can always reset the rules to allow all.",
    description:
      "Inline explanation when bex-api refuses a workspace rule change with PLAN_FEATURE_REQUIRED (w9/m388)",
  },
  "workspaces.ipRulesUnreadable": {
    message: "Couldn't load this workspace's inbound IP rules.",
    description:
      "Shown in place of the rules when the workspace inbound IP rules read fails (w9/m388)",
  },
  "workspaces.ipRulesResetAllowAll": {
    message: "Reset to allow all",
    description:
      "Button replacing the workspace inbound IP rules with the allow-all pair below the Scale plan (w9/m388)",
  },
  "workspaces.ipRulesCIDR": {
    message: "New CIDR block",
    description:
      "Accessible label for the add-row CIDR input of the workspace inbound IP rules",
  },
  "workspaces.ipRulesCIDRRule": {
    message: "CIDR block for rule {number}",
    description:
      "Accessible label for an existing workspace inbound IP rule CIDR",
  },
  "workspaces.ipRulesDescriptionLabel": {
    message: "New rule description",
    description:
      "Accessible label for the add-row description input of the workspace inbound IP rules",
  },
  "workspaces.ipRulesDescriptionRule": {
    message: "Description for rule {number}",
    description:
      "Accessible label for an existing workspace inbound IP rule description",
  },
  "workspaces.ipRulesDescriptionPlaceholder": {
    message: "Description (optional)",
    description: "Placeholder for a workspace inbound IP rule description",
  },
  "workspaces.ipRulesAdd": {
    message: "Add source",
    description: "Button adding a CIDR to the draft workspace inbound IP rules",
  },
  "workspaces.ipRulesCancel": {
    message: "Cancel",
    description:
      "Button discarding unsaved changes to the workspace inbound IP rules",
  },
  "workspaces.ipRulesSave": {
    message: "Save rules",
    description:
      "Button saving the workspace inbound IP rules (setWorkspaceIPAllowList)",
  },
  "workspaces.ipRulesInvalid": {
    message: "Enter unique, valid IPv4 or IPv6 CIDR blocks.",
    description:
      "Inline validation when a workspace rule block is not a unique, valid CIDR",
  },
  "workspaces.ipRulesRemove": {
    message: "Remove {cidr}",
    description:
      "Accessible label for a workspace inbound IP rule remove button",
  },
  "workspaces.ipRulesMoveUp": {
    message: "Move {cidr} up",
    description: "Accessible label to move a workspace inbound IP rule earlier",
  },
  "workspaces.ipRulesMoveDown": {
    message: "Move {cidr} down",
    description: "Accessible label to move a workspace inbound IP rule later",
  },
  "workspaces.ipRulesSaved": {
    message: "Inbound IP rules saved",
    description: "Toast after setWorkspaceIPAllowList succeeds (w9/m388)",
  },
  "workspaces.ipRulesSaveErrorTitle": {
    message: "Couldn't save inbound IP rules",
    description:
      "Title of the inline alert when a workspace rule change is refused (w9/m388)",
  },
  "workspaces.ipRulesSaveError": {
    message: "Couldn't save the inbound IP rules. Please try again.",
    description:
      "Fallback inline message when a workspace rule change fails without a server reason (w9/m388)",
  },
  "workspaces.twoFactorTitle": {
    message: "Two-factor authentication",
    description:
      "Title of the two-factor card on the workspace settings page, and its quick-nav label (w22/m402)",
  },
  "workspaces.twoFactorDescription": {
    message:
      "Members without two-factor authentication (2FA) can't access this workspace's resources or settings until they enable it. Their API keys and SSH keys are blocked until then too.",
    description:
      "Description under the two-factor card title; follows Render's login-settings wording",
  },
  "workspaces.twoFactorSwitchLabel": {
    message: "Enforce two-factor authentication (2FA)",
    description:
      "Label of the switch that requires 2FA of every member (Render's control name)",
  },
  "workspaces.twoFactorStatusRequired": {
    message: "Required",
    description:
      "Read-only status shown to a non-admin when the workspace requires 2FA",
  },
  "workspaces.twoFactorStatusNotRequired": {
    message: "Not required",
    description:
      "Read-only status shown to a non-admin when the workspace does not require 2FA",
  },
  "workspaces.twoFactorAdminOnly": {
    message: "Only workspace admins can change this setting.",
    description: "Hint shown to a non-admin in place of the switch",
  },
  "workspaces.twoFactorEnabledToast": {
    message: "Two-factor authentication is now required in {name}",
    description: "Toast after an admin turns the 2FA requirement on",
  },
  "workspaces.twoFactorDisabledToast": {
    message: "Two-factor authentication is no longer required in {name}",
    description: "Toast after an admin turns the 2FA requirement off",
  },
  "workspaces.twoFactorErrorTitle": {
    message: "Couldn't change the two-factor setting",
    description: "Title of the inline error when the 2FA switch is refused",
  },
  "workspaces.twoFactorError": {
    message: "Couldn't change the two-factor setting. Try again.",
    description:
      "Generic message when the 2FA switch fails without a recognised code",
  },
  "workspaces.twoFactorErrorAdminNotEnrolled": {
    message:
      "Turn on two-factor authentication for your own account before requiring it in this workspace, or you would lock yourself out.",
    description:
      "Refusal for code WORKSPACE_2FA_ADMIN_NOT_ENROLLED: the acting admin has no second factor",
  },
  "workspaces.twoFactorErrorUnavailable": {
    message:
      "Two-factor enforcement is unavailable on this installation, so it can't be turned on.",
    description:
      "Refusal for code WORKSPACE_2FA_UNAVAILABLE: the deployment has no identity lookup",
  },
  "workspaces.twoFactorEnrollLink": {
    message: "Set up two-factor authentication",
    description:
      "Link to account security, in the admin-not-enrolled refusal and on the blocked page",
  },
  "workspaces.twoFactorBlockedTitle": {
    message: "{workspace} requires two-factor authentication",
    description: "Heading of the page a blocked member sees (w22/m402)",
  },
  "workspaces.twoFactorBlockedBody": {
    message:
      "An admin of this workspace requires every member to use two-factor authentication (2FA). Until you turn it on, you can't access this workspace's resources or settings, and your API keys and SSH keys can't be used with it.",
    description: "Why the blocked page is shown",
  },
  "workspaces.twoFactorBlockedRecheck": {
    message: "I've turned it on, check again",
    description:
      "Button on the blocked page that retries the workspace after enrolling",
  },
  "workspaces.twoFactorBlockedRecheckHint": {
    message:
      "After you turn on 2FA, it can take up to a minute for access to return.",
    description: "Hint under the blocked page's check-again button",
  },
  "workspaces.twoFactorBlockedSwitchLabel": {
    message: "Switch to another workspace",
    description: "Label of the workspace picker on the blocked page",
  },
  "workspaces.twoFactorBlockedSwitchPlaceholder": {
    message: "Choose a workspace",
    description: "Placeholder of the workspace picker on the blocked page",
  },
  "workspaces.twoFactorBlockedNoOtherWorkspaces": {
    message: "No other workspaces",
    description:
      "Placeholder of the blocked page's workspace picker when the member belongs to no other workspace",
  },
  "workspaces.overlappingDeployPolicyTitle": {
    message: "Overlapping Deploy Policy",
    description:
      "Workspace Settings section title, Render's name for the setting (w4/653)",
  },
  "workspaces.overlappingDeployPolicyDescription": {
    message:
      "Choose what happens when a service's deploy is triggered while another deploy of it is in progress.",
    description:
      "Workspace Settings → Overlapping Deploy Policy description (w4/653)",
  },
  "workspaces.overlappingDeployPolicyWait": {
    message: "Wait",
    description: "The Wait policy's label, Render's wording (w4/653)",
  },
  "workspaces.overlappingDeployPolicyWaitDescription": {
    message:
      "Allow the in-progress deploy to finish, then proceed directly to the most recently triggered deploy.",
    description:
      "The Wait policy's sentence, Render's wording (render.com/docs/deploys, w4/653)",
  },
  "workspaces.overlappingDeployPolicyOverride": {
    message: "Override",
    description: "The Override policy's label, Render's wording (w4/653)",
  },
  "workspaces.overlappingDeployPolicyOverrideDescription": {
    message: "Immediately cancel the in-progress deploy and start the new one.",
    description:
      "The Override policy's sentence, Render's wording (render.com/docs/deploys, w4/653)",
  },
  "workspaces.overlappingDeployPolicySave": {
    message: "Save",
    description: "Saves the selected Overlapping Deploy Policy (w4/653)",
  },
  "workspaces.overlappingDeployPolicySaving": {
    message: "Saving…",
    description: "The Save button while the policy saves (w4/653)",
  },
  "workspaces.overlappingDeployPolicySaved": {
    message: "Overlapping deploy policy saved",
    description:
      "Toast after setWorkspaceOverlappingDeployPolicy succeeds (w4/653)",
  },
  "workspaces.overlappingDeployPolicyError": {
    message: "Couldn't save the overlapping deploy policy. Please try again.",
    description:
      "Fallback toast when a policy save fails without a server reason (w4/653)",
  },
  "workspaces.overlappingDeployPolicyAdminOnly": {
    message: "Only workspace Admins can change this policy.",
    description:
      "Shown in place of Save to a member who cannot manage the workspace (w4/653)",
  },
};

export default enWorkspaces;
