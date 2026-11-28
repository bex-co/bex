import type { TranslationEntry } from "@/i18n";

const enWorkspaces: Record<string, TranslationEntry> = {
  "workspaces.idCopied": {
    message: "Workspace ID copied",
    description: "Toast after copying the workspace ID",
  },
  "workspaces.idCopyFailed": {
    message: "Couldn't copy the workspace ID",
    description: "Toast when copying the workspace ID fails",
  },
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
    message: "$349.30/mo per organization",
    description:
      "Workspace plan card billing label (pricing.yaml scale usdPerMonth, per: organization; w7/m464)",
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
    message: "Custom terms per organization",
    description:
      "Workspace plan card billing label: Enterprise is charged once per organization like Scale (w7/m464)",
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
      "Plan link on Workspace Settings, the plan gates and the webhooks gate, opening Render's plan page (w3/m708)",
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
  "workspaces.loginRequirementStatusRequired": {
    message: "Required",
    description:
      "Read-only status shown to a non-admin when the workspace requires 2FA or Google login",
  },
  "workspaces.loginRequirementStatusNotRequired": {
    message: "Not required",
    description:
      "Read-only status shown to a non-admin when the workspace does not require 2FA or Google login",
  },
  "workspaces.loginRequirementAdminOnly": {
    message: "Only workspace admins can change this setting.",
    description:
      "Last sentence of the 2FA and Google login cards' descriptions: who can change the setting",
  },
  "workspaces.twoFactorProPlanOnly": {
    message:
      "Enforcing two-factor authentication requires the Pro plan or higher. A workspace that already enforces it keeps it until an admin turns it off.",
    description:
      "Notice on the two-factor card below the Pro plan, and the refusal for code PLAN_FEATURE_REQUIRED (Render's plan table, w7/070)",
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
  "workspaces.loginBlockedSwitchLabel": {
    message: "Switch to another workspace",
    description:
      "Label of the workspace picker on the 2FA and Google login blocked pages",
  },
  "workspaces.loginBlockedSwitchPlaceholder": {
    message: "Choose a workspace",
    description:
      "Placeholder of the workspace picker on the 2FA and Google login blocked pages",
  },
  "workspaces.loginBlockedNoOtherWorkspaces": {
    message: "No other workspaces",
    description:
      "Placeholder of the blocked pages' workspace picker when the member belongs to no other workspace",
  },
  "workspaces.googleLoginTitle": {
    message: "Google login",
    description:
      "Title of the Google login card on the workspace settings page, and its quick-nav label (w7/m465)",
  },
  "workspaces.googleLoginDescription": {
    message:
      "Members who don't sign in with the Google account of their bex email address can't access this workspace's resources or settings. API keys and SSH keys created while signed in another way are blocked too.",
    description:
      "Description under the Google login card title; follows Render's login-settings wording",
  },
  "workspaces.googleLoginSwitchLabel": {
    message: "Enforce Google login",
    description:
      "Label of the switch that requires every member to sign in with Google (Render's control name)",
  },
  "workspaces.googleLoginProPlanOnly": {
    message:
      "Enforcing Google login requires the Pro plan or higher. A workspace that already enforces it keeps it until an admin turns it off.",
    description:
      "Notice on the Google login card below the Pro plan, and the refusal for code PLAN_FEATURE_REQUIRED (Render's plan table)",
  },
  "workspaces.googleLoginEnabledToast": {
    message: "Google login is now required in {name}",
    description: "Toast after an admin turns the Google login requirement on",
  },
  "workspaces.googleLoginDisabledToast": {
    message: "Google login is no longer required in {name}",
    description: "Toast after an admin turns the Google login requirement off",
  },
  "workspaces.googleLoginErrorTitle": {
    message: "Couldn't change the Google login setting",
    description:
      "Title of the inline error when the Google login switch is refused",
  },
  "workspaces.googleLoginError": {
    message: "Couldn't change the Google login setting. Try again.",
    description:
      "Generic message when the Google login switch fails without a recognised code",
  },
  "workspaces.googleLoginErrorAdminNotSignedIn": {
    message:
      "Sign in with the Google account of your bex email address before requiring Google login in this workspace, or you would lock yourself out.",
    description:
      "Refusal for code WORKSPACE_GOOGLE_LOGIN_ADMIN_NOT_SIGNED_IN: the acting admin is not signed in with Google",
  },
  "workspaces.googleLoginErrorUnavailable": {
    message:
      "Google login enforcement is unavailable on this installation, so it can't be turned on.",
    description:
      "Refusal for code WORKSPACE_GOOGLE_LOGIN_UNAVAILABLE: the deployment cannot look up Google accounts",
  },
  "workspaces.googleLoginSignIn": {
    message: "Sign in with Google",
    description:
      "Button that signs the session in again with Google and returns to the page: on the blocked page and in the admin's lock-out refusal",
  },
  "workspaces.googleLoginSignInError": {
    message: "Couldn't start signing in with Google. Try again.",
    description: "Shown when Kratos did not start the Google sign-in",
  },
  "workspaces.googleLoginBlockedTitle": {
    message: "{workspace} requires Google login",
    description:
      "Heading of the page a member who did not sign in with Google sees (w7/m465)",
  },
  "workspaces.googleLoginBlockedBody": {
    message:
      "An admin of this workspace requires every member to sign in with Google. Until you sign in with the Google account of your bex email address, you can't access this workspace's resources or settings, and API keys and SSH keys you created while signed in another way can't be used with it.",
    description: "Why the Google login blocked page is shown",
  },
  "workspaces.googleLoginBlockedHint": {
    message: "You'll come back to this page once you've signed in.",
    description: "Hint under the blocked page's Sign in with Google button",
  },
  "workspaces.requireTwoFactorConfirmTitle": {
    message: "Require two-factor authentication in {workspace}?",
    description:
      "Title of the confirmation before an admin requires 2FA in the workspace (w7/m471)",
  },
  "workspaces.requireTwoFactorConfirm": {
    message:
      "Members without two-factor authentication lose access to this workspace until they turn it on, and their API keys and SSH keys stop working with it.",
    description:
      "Body of the confirmation before an admin requires 2FA in the workspace (w7/m471)",
  },
  "workspaces.requireGoogleLoginConfirmTitle": {
    message: "Require Google login in {workspace}?",
    description:
      "Title of the confirmation before an admin requires Google login in the workspace (w7/m471)",
  },
  "workspaces.requireGoogleLoginConfirm": {
    message:
      "Members who aren't signed in with the Google account of their bex email address lose access to this workspace until they sign in that way, and API keys and SSH keys created while signed in another way stop working with it.",
    description:
      "Body of the confirmation before an admin requires Google login in the workspace (w7/m471)",
  },
  "workspaces.requireConfirmAction": {
    message: "Require",
    description:
      "Confirm button of the workspace's require-2FA and require-Google-login confirmations (w7/m471)",
  },
  "workspaces.loginImpactRegionLabel": {
    message: "Who would lose access",
    description:
      "Accessible name of the confirmation's list of members and keys a login requirement would block (w7/m471)",
  },
  "workspaces.loginImpactChecking": {
    message: "Checking who would lose access…",
    description:
      "Screen-reader text while the confirmation checks who a login requirement would block",
  },
  "workspaces.loginImpactError": {
    message: "Couldn't check who would lose access. You can still turn it on.",
    description:
      "Shown in the confirmation when bex could not check who a login requirement would block; confirming stays possible",
  },
  "workspaces.loginImpactNobody": {
    message:
      "Nobody would lose access: every member meets this requirement, and no API key or SSH key would stop working.",
    description:
      "Shown in the confirmation when a login requirement would block nobody",
  },
  "workspaces.loginImpactMembersTwoFactor": {
    message:
      "Each listed member will lose access until they turn on two-factor authentication. Each listed API key and SSH key will stop working until its owner does.",
    description:
      "Sentence above the members and keys requiring 2FA would block, after Render's SSO enforcement list",
  },
  "workspaces.loginImpactMembersGoogleLogin": {
    message:
      "Each listed member will lose access until they sign in with the Google account of their bex email address. Each listed API key and SSH key will stop working; its owner can create a new one while signed in with Google.",
    description:
      "Sentence above the members and keys requiring Google login would block, after Render's SSO enforcement list",
  },
  "workspaces.loginImpactLosesAccess": {
    message: "Loses access",
    description: "Badge on a listed member who would lose access",
  },
  "workspaces.loginImpactKeepsAccess": {
    message: "Keeps access",
    description:
      "Badge on a listed member who keeps access while their listed keys stop working",
  },
  "workspaces.loginImpactApiKey": {
    message: "API key {name}",
    description:
      "A listed API key that would stop working; {name} is the key's name",
  },
  "workspaces.loginImpactApiKeyInWorkspace": {
    message: "API key {name} in {workspace}",
    description:
      "A listed API key on an org's confirmation; {workspace} is the workspace it belongs to",
  },
  "workspaces.loginImpactSshKey": {
    message: "SSH key {name}",
    description:
      "A listed SSH key that would stop working; {name} is the key's name",
  },
  "workspaces.loginImpactUnattributed": {
    message:
      "API keys with no recorded creator, or whose creator is no longer a member, will stop working:",
    description:
      "Heading of the API keys no current member created, which would stop working",
  },
  "workspaces.loginImpactGoogleSessions": {
    message:
      "Members who meet it but are signed in another way right now will be asked to sign in with Google.",
    description:
      "One line under the Google login confirmation's list: who the list cannot show",
  },
  "workspaces.loginNoticeMembers_one": {
    message: "{count} member",
    description:
      "The bold count in the notice under an enforced login requirement: how many members do not meet it (w1/m647, Render's wording)",
  },
  "workspaces.loginNoticeMembers_other": {
    message: "{count} members",
    description:
      "The bold count in the notice under an enforced login requirement: how many members do not meet it (w1/m647, Render's wording)",
  },
  "workspaces.loginNoticeTwoFactor_one": {
    message:
      "{members} of your team has not enabled two-factor authentication yet.",
    description:
      "Notice under enforced 2FA when members have not enabled it; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeTwoFactor_other": {
    message:
      "{members} of your team have not enabled two-factor authentication yet.",
    description:
      "Notice under enforced 2FA when members have not enabled it; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeGoogleLogin_one": {
    message: "{members} of your team has not signed in with Google yet.",
    description:
      "Notice under enforced Google login when members have not signed in with Google; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeGoogleLogin_other": {
    message: "{members} of your team have not signed in with Google yet.",
    description:
      "Notice under enforced Google login when members have not signed in with Google; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeViewMembers_one": {
    message: "View member",
    description:
      "Button on the notice that opens the list of members who do not meet the requirement (Render's wording)",
  },
  "workspaces.loginNoticeViewMembers_other": {
    message: "View members",
    description:
      "Button on the notice that opens the list of members who do not meet the requirement (Render's wording)",
  },
  "workspaces.loginNoticeAllMembers": {
    message: "All members",
    description:
      "The bold words of the all-compliant notice (Render's wording)",
  },
  "workspaces.loginNoticeAllTwoFactor": {
    message: "{members} of your team have enabled two-factor authentication!",
    description:
      "Notice under enforced 2FA when every member has enabled it; {members} is the bold 'All members' (Render's wording)",
  },
  "workspaces.loginNoticeAllGoogleLogin": {
    message: "{members} of your team have signed in with Google!",
    description:
      "Notice under enforced Google login when every member has signed in with Google; {members} is the bold 'All members' (Render's wording)",
  },
  "workspaces.loginMembersTitleTwoFactor": {
    message: "Members without two-factor authentication",
    description:
      "Title of the dialog listing members without 2FA (Render's wording)",
  },
  "workspaces.loginMembersTitleGoogleLogin": {
    message: "Members without Google sign-on",
    description:
      "Title of the dialog listing members who have not signed in with Google (Render's wording)",
  },
  "workspaces.loginMembersDescription_one": {
    message:
      "This member is currently prohibited from accessing this team workspace and the resources within it.",
    description:
      "Description of the dialog listing members who do not meet the enforced requirement (Render's wording)",
  },
  "workspaces.loginMembersDescription_other": {
    message:
      "These members are currently prohibited from accessing this team workspace and the resources within it.",
    description:
      "Description of the dialog listing members who do not meet the enforced requirement (Render's wording)",
  },
  "workspaces.twoFactorRemindTitle_one": {
    message: "Remind this user to enable 2FA?",
    description:
      "Title of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindTitle_other": {
    message: "Remind these users to enable 2FA?",
    description:
      "Title of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindBody_one": {
    message: "Send out an email reminder to this user.",
    description:
      "Body of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindBody_other": {
    message: "Send out email reminders to these users.",
    description:
      "Body of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindAction_one": {
    message: "Send reminder",
    description: "Button that emails the 2FA reminders (Render's wording)",
  },
  "workspaces.twoFactorRemindAction_other": {
    message: "Send reminders",
    description: "Button that emails the 2FA reminders (Render's wording)",
  },
  "workspaces.twoFactorRemindError": {
    message:
      "We're sorry. We weren't able to send reminders to these team members. Try again?",
    description:
      "Shown in the members dialog when the reminders could not be sent (Render's wording)",
  },
  "workspaces.twoFactorRemindSentToast": {
    message:
      "Email reminders sent to members without two-factor authentication!",
    description: "Toast after the 2FA reminders are sent (Render's wording)",
  },
  "workspaces.twoFactorRemindLimitTitle": {
    message: "Remind limit has been reached!",
    description:
      "Title of the dialog shown when reminders were already sent today (Render's wording)",
  },
  "workspaces.twoFactorRemindLimitBody": {
    message:
      "It looks like we've already sent out reminders for today. You can send reminders again tomorrow.",
    description:
      "Body of the dialog shown when reminders were already sent today; the day is UTC (Render's wording)",
  },
  "workspaces.twoFactorConfirmReminder_one": {
    message: "A reminder to enable 2FA will be sent to this member.",
    description:
      "Line in the require-2FA confirmation: turning it on emails a reminder to each member listed (Render's wording)",
  },
  "workspaces.twoFactorConfirmReminder_other": {
    message: "Reminders to enable 2FA will be sent to these members.",
    description:
      "Line in the require-2FA confirmation: turning it on emails a reminder to each member listed (Render's wording)",
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
  "workspaces.orgDowngradePersonRole": {
    message: "{person} ({role})",
    description:
      "A member or invitee in a downgrade precondition: {person} is their id or email, {role} the role to change (w7/m464)",
  },
  "workspaces.orgDowngradeIpRuleWorkspace": {
    message: "this workspace",
    description:
      "The workspace's own inbound IP rules, in the inbound_ip_rules precondition (w7/m464)",
  },
  "workspaces.orgDowngradeIpRuleEnvironment": {
    message: "environment {name}",
    description:
      "An environment with inbound IP rules, in the inbound_ip_rules precondition (w7/m464)",
  },
  "workspaces.orgDowngradeIpRuleService": {
    message: "service {name}",
    description:
      "A service with inbound IP rules, in the inbound_ip_rules precondition (w7/m464)",
  },
  "workspaces.orgPlanOnPrimaryError": {
    message:
      "This workspace's plan is its organization's plan. Change it on the organization's primary workspace.",
    description:
      "Refusal for code ORGANIZATION_PLAN_ON_PRIMARY: a secondary workspace's plan changes only on the org's primary (w7/m464)",
  },
  "workspaces.orgPlanSecondaryNote": {
    message:
      "Included in your organization's plan. The organization's primary workspace manages its plan and billing.",
    description:
      "Workspace Settings and Billing on an org's secondary workspace: its plan is the org's, managed on the primary (ADR115, w7/m464)",
  },
  "workspaces.orgPlanPrimarySettingsLink": {
    message: "Go to the primary workspace",
    description:
      "Link from a secondary workspace's settings to the org's primary workspace settings (w7/m464)",
  },
  "workspaces.orgPlanPrimaryBillingLink": {
    message: "Go to the primary workspace's billing",
    description:
      "Button on a secondary workspace's Billing plan card linking to the org's primary workspace's Billing (w7/m464)",
  },
  "workspaces.privacyTitle": {
    message: "Workspace privacy",
    description:
      "Workspace Settings → Workspace privacy card title, radio group name and section link (w7/m464)",
  },
  "workspaces.privacyDescription": {
    message: "Choose who in your organization can join this workspace.",
    description: "Description under the Workspace privacy card title (w7/m464)",
  },
  "workspaces.privacyPublic": {
    message: "Public",
    description: "The Public privacy setting's label, Render's name (w7/m464)",
  },
  "workspaces.privacyPublicDescription": {
    message:
      "Any member of your organization can add themselves to this workspace.",
    description:
      "What the Public privacy setting allows, Render's wording (render.com/docs/organizations, w7/m464)",
  },
  "workspaces.privacyInviteOnly": {
    message: "Invite-only",
    description:
      "The Invite-only privacy setting's label, Render's name (w7/m464)",
  },
  "workspaces.privacyInviteOnlyDescription": {
    message: "Only workspace admins can invite people to this workspace.",
    description:
      "What the Invite-only privacy setting allows, Render's wording (render.com/docs/organizations, w7/m464)",
  },
  "workspaces.privacySave": {
    message: "Save",
    description: "Saves the selected workspace privacy (w7/m464)",
  },
  "workspaces.privacySaving": {
    message: "Saving…",
    description: "The Save button while the workspace privacy saves (w7/m464)",
  },
  "workspaces.privacySaved": {
    message: "Workspace privacy saved",
    description: "Toast after setWorkspacePrivacy succeeds (w7/m464)",
  },
  "workspaces.privacyError": {
    message: "Couldn't save the workspace privacy. Please try again.",
    description:
      "Fallback toast when a privacy save fails without a server reason (w7/m464)",
  },
  "workspaces.loginRequirementRequiredByOrganization": {
    message: "Required by {organization} in all of its workspaces.",
    description:
      "Workspace Settings, under the 2FA or Google login switch: why it is on and locked, the workspace's organization requires it in all of its workspaces (w7/m466)",
  },
  "workspaces.loginRequirementOrganizationSettingsLink": {
    message: "Change it in the organization's settings",
    description:
      "Link for an org Owner after the required-by-organization note, to the organization's Settings page (w7/m466)",
  },
  "workspaces.twoFactorBlockedOrganizationTitle": {
    message:
      "{organization} requires two-factor authentication in all of its workspaces",
    description:
      "Heading of the 2FA blocked page when the organization's setting refuses the member, on a workspace or on the org's own pages (w7/m466)",
  },
  "workspaces.twoFactorBlockedOrganizationBody": {
    message:
      "An Owner of this organization requires every member to use two-factor authentication (2FA) in the organization and all of its workspaces. Until you turn it on, you can't access them, and your API keys and SSH keys can't be used with them.",
    description:
      "Why the 2FA blocked page is shown when the organization's setting refuses the member (w7/m466)",
  },
  "workspaces.googleLoginBlockedOrganizationTitle": {
    message: "{organization} requires Google login in all of its workspaces",
    description:
      "Heading of the Google login blocked page when the organization's setting refuses the member (w7/m466)",
  },
  "workspaces.googleLoginBlockedOrganizationBody": {
    message:
      "An Owner of this organization requires every member to sign in with Google in the organization and all of its workspaces. Until you sign in with the Google account of your bex email address, you can't access them, and API keys and SSH keys you created while signed in another way can't be used with them.",
    description:
      "Why the Google login blocked page is shown when the organization's setting refuses the member (w7/m466)",
  },
  "workspaces.updatePlanPageTitle": {
    message: "Change plan",
    description: "Document title of the plan pages (w3/m708)",
  },
  "workspaces.updatePlanTitle": {
    message: "Change plan for {name}",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): page title with the workspace's name",
  },
  "workspaces.updatePlanConfirmTitle": {
    message: "Change Plan for {name}",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): page title with the workspace's name (Render capitalizes Plan here)",
  },
  "workspaces.updatePlanBack": {
    message: "Back",
    description:
      "Accessible label of the plan pages' Back button, and the confirmation's Back action (w3/m708)",
  },
  "workspaces.updatePlanSelectTitle": {
    message: "Select a plan",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): heading of the plan cards",
  },
  "workspaces.updatePlanCompare": {
    message: "Learn more about or compare plans by visiting our {link}.",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): line under Select a plan; {link} is the updatePlanPricingLink link to bex's pricing page",
  },
  "workspaces.updatePlanPricingLink": {
    message: "pricing page",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): the link in updatePlanCompare",
  },
  "workspaces.updatePlanPaymentTitle": {
    message: "Payment Method",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): heading of the payment method section",
  },
  "workspaces.updatePlanCardRequired": {
    message:
      "This workspace plan requires a card for paid services and compute.",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): notice while a plan other than Hobby is selected",
  },
  "workspaces.updatePlanHowBillingWorks": {
    message: "How billing works",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): link beside updatePlanCardRequired to bex's billing docs",
  },
  "workspaces.updatePlanWorkspaceBillingTitle": {
    message: "Workspace billing",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): heading beside the card on file",
  },
  "workspaces.updatePlanWorkspaceBillingBody": {
    message:
      "Billing is unique to each workspace. Your card will be charged for fees associated with paid services or compute in this workspace.",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): line under updatePlanWorkspaceBillingTitle",
  },
  "workspaces.updatePlanErrorsSummary": {
    message:
      "There are some errors in your selection. Please review and fix them to continue.",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): summary under the plan rules a selection breaks",
  },
  "workspaces.updatePlanErrorPrefix": {
    message: "An error occurred. {message}",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): a refused change; {message} is the reason",
  },
  "workspaces.updatePlanPaymentRequired": {
    message: "Add a valid card to upgrade to a paid plan.",
    description:
      "The plan pages' message when bex-api refuses a paid plan with PAYMENT_REQUIRED (w3/m708)",
  },
  "workspaces.updatePlanServerError": {
    message: "Server error during submission. Please try again",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): fallback reason when an upgrade fails without one",
  },
  "workspaces.updatePlanSubmit": {
    message: "Select plan",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): submit button",
  },
  "workspaces.updatePlanCancel": {
    message: "Cancel",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): cancel button, back to where the page was opened from",
  },
  "workspaces.updatePlanNoPermission": {
    message:
      "Your role can’t change this workspace’s plan. Ask a workspace admin.",
    description:
      "The plan pages' notice for a member who cannot change the plan (can_manage) (w3/m708)",
  },
  "workspaces.planCurrentBadge": {
    message: "Current plan",
    description:
      "Badge on the workspace's own plan card on the plan page (Render's TeamPlanPicker, w3/m708)",
  },
  "workspaces.planNewBadge": {
    message: "New plan",
    description:
      "Badge on the chosen plan card on the plan page (Render's TeamPlanPicker, w3/m708)",
  },
  "workspaces.planContact": {
    message: "Get in touch",
    description:
      "Enterprise card's contact action on the plan page, Render's Get in touch (w3/m708)",
  },
  "workspaces.updatePlanSummaryTitle": {
    message: "New plan summary",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): heading of the plan comparison and the card on file",
  },
  "workspaces.updatePlanConfirmDowngradeTitle": {
    message: "Confirm downgrade",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): heading of the typed confirmation a downgrade asks for",
  },
  "workspaces.updatePlanConfirmChangesIncluding": {
    message: "To initiate the changes to your plan, including:",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): lead-in above the list of what a downgrade changes",
  },
  "workspaces.updatePlanConfirmTypeAfterList": {
    message: "Type {phrase} in the text box and click the confirm button.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): the typed phrase after the list of changes; {phrase} is 'sudo downgrade workspace <name>' (rendered bold)",
  },
  "workspaces.updatePlanConfirmType": {
    message:
      "To initiate the changes to your plan, type {phrase} in the text box and click the confirm button.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): the typed phrase when the downgrade lists no changes; {phrase} is 'sudo downgrade workspace <name>' (rendered bold)",
  },
  "workspaces.updatePlanConfirmSubmit": {
    message: "Confirm",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): confirm button",
  },
  "workspaces.updatePlanErrorTitle": {
    message: "An error occurred.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): title of a refused change",
  },
  "workspaces.updatePlanError": {
    message: "Error updating plan.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): fallback reason when the change fails without one",
  },
  "workspaces.updatePlanRemovesOrganization": {
    message: "The organization will be removed.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): bold start of the change an org primary's downgrade below Scale makes (ADR115 D6)",
  },
  "workspaces.updatePlanRemovesOrganizationDetail": {
    message:
      "Its org-level features end, and this workspace continues as a standalone workspace.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): rest of the org removal line after updatePlanRemovesOrganization",
  },
  "workspaces.planUpdated": {
    message: "Your workspace is now on the {plan} plan.",
    description:
      "Toast after a plan change, Render's plan-updated toast; {plan} is the plan's name (w3/m708)",
  },
  "workspaces.planUpdatedWelcome": {
    message: "Welcome to the {plan} plan.",
    description:
      "Toast after a change to Scale, Render's welcome; {plan} is the plan's name (w3/m708)",
  },
  "workspaces.planComparisonCurrent": {
    message: "Current Plan",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): badge on the current plan's side of the comparison",
  },
  "workspaces.planComparisonNew": {
    message: "New Plan",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): badge on the new plan's side of the comparison",
  },
  "workspaces.planComparisonHighlights": {
    message: "Highlights",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): label above each side's highlights (shown in capitals)",
  },
  "workspaces.planHighlightJustForMe": {
    message: "Just for me",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Hobby's one seat",
  },
  "workspaces.planHighlightUnlimitedMembers": {
    message: "Unlimited team members",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): unlimited seats",
  },
  "workspaces.planHighlightIncludedDomains": {
    message: "{included} custom domains included, then {price}/domain",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): custom domains included before they are metered; {included} is the count, {price} bex's price per domain (ADR030 §16)",
  },
  "workspaces.planHighlightUnlimitedDomains": {
    message: "Unlimited custom domains",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): every custom domain included (Enterprise)",
  },
  "workspaces.planHighlightTwoEnvironments": {
    message: "2 environments per project",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Hobby's environments per project",
  },
  "workspaces.planHighlightUnlimitedEnvironments": {
    message: "Unlimited projects and environments",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): unlimited projects and environments",
  },
  "workspaces.planHighlightRetention": {
    message: "{days}-day window for logs and metrics",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): how far back logs and metrics reach; {days} is the number of days",
  },
  "workspaces.planHighlightBasicLogs": {
    message: "Basic logs and metrics",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Hobby's logs and metrics, without request logs",
  },
  "workspaces.planHighlightRequestLogs": {
    message: "HTTP request logs and response time metrics",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Pro's HTTP request logs and response time metrics",
  },
  "workspaces.planHighlightKeptBuilds": {
    message: "{builds} retained builds for instant rollback",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): builds kept for rollback; {builds} is the count (core.KeptBuilds)",
  },
  "workspaces.planHighlightAuditLogs": {
    message: "Workspace audit logs",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): workspace audit logs (Pro and higher)",
  },
  "workspaces.planHighlightOrganization": {
    message:
      "An organization, with more workspaces, extra roles and inbound IP rules",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): what an organization adds (Scale and higher, ADR115)",
  },
  "workspaces.planBlockerMemberLimitTitle": {
    message: "Too many users in team.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when the workspace has more people than the plan seats",
  },
  "workspaces.planBlockerMemberLimitOne": {
    message: "{plan} workspaces can only have 1 user.",
    description:
      "Render's plan rule on its plan page (w3/m708): a plan with one seat; {plan} is the plan's name",
  },
  "workspaces.planBlockerMemberLimitMany": {
    message: "{plan} workspaces can only have up to {limit} users.",
    description:
      "Render's plan rule on its plan page (w3/m708): a plan with several seats; {plan} is the plan's name, {limit} its seats",
  },
  "workspaces.planBlockerUpdateTeamMembers": {
    message: "Update team members",
    description:
      "Render's plan rule on its plan page (w3/m708): link to Workspace Settings' Team section",
  },
  "workspaces.planBlockerServiceLimitTitle": {
    message: "Too many services in workspace.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when the workspace has more services than the plan allows",
  },
  "workspaces.planBlockerServiceLimit": {
    message: "{plan} workspaces can only have up to {limit} services.",
    description:
      "Render's plan rule on its plan page (w3/m708): the plan's service cap; {plan} is the plan's name, {limit} its cap",
  },
  "workspaces.planBlockerUpdateServices": {
    message: "Update services",
    description:
      "Render's plan rule on its plan page (w3/m708): link to the workspace's services",
  },
  "workspaces.planBlockerWorkspaceLimitTitle": {
    message: "Too many Hobby workspaces.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when the caller already has the most Hobby workspaces allowed",
  },
  "workspaces.planBlockerWorkspaceLimit": {
    message:
      "You cannot have more than {limit} Hobby workspaces. Please upgrade or delete another Hobby workspace.",
    description:
      "Render's plan rule on its plan page (w3/m708): the cap on a person's Hobby workspaces; {limit} is the cap",
  },
  "workspaces.planBlockerWorkspaceLimitsDocs": {
    message: "Workspace limits",
    description:
      "Render's plan rule on its plan page (w3/m708): link to bex's docs on workspace limits",
  },
  "workspaces.planBlockerWorkspacesTitle": {
    message: "Too many workspaces.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when an org primary's downgrade leaves secondary workspaces",
  },
  "workspaces.planBlockerWorkspaces": {
    message: "{plan} doesn’t support more than one workspace.",
    description:
      "Render's plan rule on its plan page (w3/m708): a plan without an organization; {plan} is the plan's name",
  },
  "workspaces.planBlockerUpdateWorkspaces": {
    message: "Update workspaces",
    description:
      "Render's plan rule on its plan page (w3/m708): link to the organization's workspaces",
  },
  "workspaces.planBlockerRolesTitle": {
    message: "Cannot downgrade while using unavailable roles",
    description:
      "Render's plan rule on its plan page (w3/m708): title when members or invites hold roles the plan does not offer",
  },
  "workspaces.planBlockerRolesOne": {
    message:
      "{plan} doesn’t support the {roles} role. To downgrade your workspace, first change affected team members and pending invites to Developer or Admin.",
    description:
      "Render's plan rule on its plan page (w3/m708): one role the plan lacks; {plan} is the plan's name, {roles} the role",
  },
  "workspaces.planBlockerRolesMany": {
    message:
      "{plan} doesn’t support the {roles} roles. To downgrade your workspace, first change affected team members and pending invites to Developer or Admin.",
    description:
      "Render's plan rule on its plan page (w3/m708): several roles the plan lacks; {plan} is the plan's name, {roles} the roles",
  },
  "workspaces.planBlockerIpRulesTitle": {
    message: "IP allow lists must be removed before downgrading.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when inbound IP rules stand in the way (Render's words for them on its confirmation)",
  },
  "workspaces.planBlockerIpRules": {
    message:
      "Remove all IP allow list configurations from your workspace, environments, and services before changing your plan.",
    description:
      "Render's plan rule on its plan page (w3/m708): what to remove for planBlockerIpRulesTitle",
  },
  "workspaces.planBlockerUpdateIpRules": {
    message: "Update inbound IP rules",
    description:
      "Render's plan rule on its plan page (w3/m708): link to Workspace Settings' inbound IP rules",
  },
  "workspaces.planBlockerOtherTitle": {
    message: "This plan can’t be selected yet.",
    description:
      "Render's plan rule on its plan page (w3/m708): title of a refusal kind this dashboard does not know yet",
  },
  "workspaces.planBlockerOther": {
    message: "This workspace doesn’t meet the {plan} plan’s limits yet.",
    description:
      "Render's plan rule on its plan page (w3/m708): line under planBlockerOtherTitle; {plan} is the plan's name",
  },
};

export default enWorkspaces;
