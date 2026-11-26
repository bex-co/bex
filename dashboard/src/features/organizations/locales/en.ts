import type { TranslationEntry } from "@/i18n";

const enOrganizations: Record<string, TranslationEntry> = {
  "organizations.switcherOrganization": {
    message: "Organization",
    description:
      "Workspace switcher: the sublabel under an organization's name, which opens the org's Workspaces page",
  },
  "organizations.navLabel": {
    message: "Organization pages",
    description:
      "Accessible name of the org page navigation (Workspaces, Members, Settings)",
  },
  "organizations.navWorkspaces": {
    message: "Workspaces",
    description: "Org page navigation: the Workspaces page",
  },
  "organizations.navMembers": {
    message: "Members",
    description: "Org page navigation: the Members page",
  },
  "organizations.navSettings": {
    message: "Settings",
    description: "Org page navigation: the Settings page",
  },
  "organizations.untitled": {
    message: "Organization",
    description: "Org page title when the organization could not be read",
  },
  "organizations.workspacesTitle": {
    message: "Organization Workspaces",
    description: "Document title of the org's Workspaces page",
  },
  "organizations.membersTitle": {
    message: "Organization Members",
    description: "Document title of the org's Members page",
  },
  "organizations.settingsTitle": {
    message: "Organization Settings",
    description: "Document title of the org's Settings page",
  },
  "organizations.workspacesDescription": {
    message:
      "An organization holds unlimited workspaces, each on the organization's plan at no additional cost.",
    description:
      "Org Workspaces page header description (Render's organizations doc)",
  },
  "organizations.membersDescription": {
    message:
      "Owners manage the organization, Members join its workspaces, and Guests reach a single workspace.",
    description: "Org Members page header description",
  },
  "organizations.settingsDescription": {
    message: "The organization's name and its audit log.",
    description: "Org Settings page header description",
  },
  "organizations.roleOwner": {
    message: "Owner",
    description: "Org role: Owner (Render's Org Owner)",
  },
  "organizations.roleMember": {
    message: "Member",
    description: "Org role: Member (a standard org member)",
  },
  "organizations.roleGuest": {
    message: "Guest",
    description: "Org role: Guest (reaches a single workspace)",
  },
  "organizations.notFoundTitle": {
    message: "Organization not found",
    description: "Org pages: ORGANIZATION_NOT_FOUND state title",
  },
  "organizations.notFoundBody": {
    message: "It doesn't exist, or you're not one of its members.",
    description: "Org pages: ORGANIZATION_NOT_FOUND state body",
  },
  "organizations.errorTitle": {
    message: "Couldn't load the organization",
    description: "Org pages: read failure state title",
  },
  "organizations.errorBody": {
    message: "Try again in a moment.",
    description: "Org pages: read failure state body",
  },

  "organizations.workspacesCardTitle": {
    message: "Workspaces",
    description: "Org Workspaces page: the workspace list card title",
  },
  "organizations.workspacesCardDescription": {
    message:
      "Any Member can join a Public workspace. An Invite-only workspace takes an invite from one of its admins.",
    description: "Org Workspaces page: the workspace list card description",
  },
  "organizations.colWorkspace": {
    message: "Workspace",
    description: "Org workspace table column: the workspace name",
  },
  "organizations.colPrivacy": {
    message: "Privacy",
    description: "Org workspace table column: Public or Invite-only",
  },
  "organizations.primary": {
    message: "Primary",
    description:
      "Badge on the org's primary workspace (the one that holds the org's plan and billing)",
  },
  "organizations.workspacesEmptyTitle": {
    message: "No workspaces to show",
    description: "Org Workspaces page: empty list title",
  },
  "organizations.workspacesEmptyBody": {
    message: "Workspaces you can see in this organization appear here.",
    description: "Org Workspaces page: empty list body",
  },
  "organizations.open": {
    message: "Open",
    description:
      "Org workspace row action: switch to a workspace the caller belongs to",
  },
  "organizations.join": {
    message: "Join",
    description:
      "Org workspace row action: a Member or Owner adds themselves to a Public workspace (Developer role)",
  },
  "organizations.joinAsAdmin": {
    message: "Join as admin",
    description:
      "Org workspace row action: an Owner adds themselves to any org workspace as admin",
  },
  "organizations.joined": {
    message: "You joined {workspace}.",
    description: "Toast after joining a Public workspace",
  },
  "organizations.joinedAsAdmin": {
    message: "You're now an admin of {workspace}.",
    description: "Toast after an Owner joins a workspace as admin",
  },
  "organizations.joinFailed": {
    message: "Couldn't join {workspace}.",
    description: "Toast when joining a workspace failed for no stated reason",
  },
  "organizations.newWorkspace": {
    message: "New Workspace",
    description:
      "Org Workspaces page: the Owner's button that opens the create dialog (Render: + New Workspace)",
  },
  "organizations.createTitle": {
    message: "New workspace in {organization}",
    description: "Create org workspace dialog title",
  },
  "organizations.createDescription": {
    message:
      "You become its first admin and can start creating services right away.",
    description: "Create org workspace dialog description",
  },
  "organizations.fieldName": {
    message: "Name",
    description: "Create workspace / rename organization: the name field label",
  },
  "organizations.fieldPrivacy": {
    message: "Privacy",
    description: "Create workspace dialog: the privacy choice's legend",
  },
  "organizations.planIncluded": {
    message: "Included in your organization's plan",
    description:
      "Create workspace dialog: the new workspace takes the org's plan, no price and no payment step",
  },
  "organizations.planIncludedDetail": {
    message: "{plan} plan, at no additional cost",
    description:
      "Create workspace dialog: the org's plan name under 'Included in your organization's plan'",
  },
  "organizations.createSubmit": {
    message: "Create Workspace",
    description: "Create org workspace dialog: submit button (Render's label)",
  },
  "organizations.cancel": {
    message: "Cancel",
    description: "Org dialogs: cancel button",
  },
  "organizations.workspaceCreated": {
    message: "Created {workspace}.",
    description: "Toast after creating an org workspace",
  },
  "organizations.createFailed": {
    message: "Couldn't create the workspace.",
    description: "Create org workspace: failure with no stated reason",
  },

  "organizations.membersCardTitle": {
    message: "Members",
    description: "Org Members page: the member list card title",
  },
  "organizations.membersCardDescription": {
    message:
      "Everyone in the organization and their organization role. Workspace roles are set in each workspace.",
    description: "Org Members page: the member list card description",
  },
  "organizations.colMember": {
    message: "Member",
    description: "Org member table column: the person",
  },
  "organizations.colRole": {
    message: "Role",
    description: "Org member table column: the org role",
  },
  "organizations.guestWorkspace": {
    message: "Workspace: {workspace}",
    description: "Org member table: the one workspace a Guest belongs to",
  },
  "organizations.guestNoWorkspace": {
    message: "Not in a workspace yet",
    description: "Org member table: a Guest who belongs to no workspace",
  },
  "organizations.roleFor": {
    message: "Organization role for {identity}",
    description: "Accessible name of an org member's role select",
  },
  "organizations.removeMember": {
    message: "Remove {identity} from the organization",
    description: "Accessible name of an org member row's remove button",
  },
  "organizations.removeTitle": {
    message: "Remove from the organization?",
    description: "Org member remove confirmation title",
  },
  "organizations.removeConfirm": {
    message:
      "{identity} loses access to the organization and to every one of its workspaces, and their API keys in those workspaces are revoked.",
    description: "Org member remove confirmation description",
  },
  "organizations.remove": {
    message: "Remove",
    description: "Org member remove confirmation: confirm button",
  },
  "organizations.removed": {
    message: "Removed {identity} from the organization.",
    description: "Toast after removing an org member",
  },
  "organizations.removeFailed": {
    message: "Couldn't remove {identity}.",
    description: "Org member removal failure with no stated reason",
  },
  "organizations.roleChanged": {
    message: "{identity} is now {role}.",
    description: "Toast after changing an org member's role",
  },
  "organizations.roleChangeFailed": {
    message: "Couldn't change the role.",
    description: "Org role change failure with no stated reason",
  },
  "organizations.membersForbiddenTitle": {
    message: "Members are hidden from Guests",
    description:
      "Org Members page: a Guest's state (the API answers FORBIDDEN)",
  },
  "organizations.membersForbiddenBody": {
    message:
      "Only Owners and Members of the organization can see who belongs to it.",
    description: "Org Members page: a Guest's state body",
  },
  "organizations.membersErrorTitle": {
    message: "Couldn't load the members",
    description: "Org Members page: read failure title",
  },
  "organizations.invite": {
    message: "Invite",
    description: "Org Members page: the Owner's invite button",
  },
  "organizations.inviteTitle": {
    message: "Invite to the organization",
    description: "Org invite dialog title",
  },
  "organizations.inviteDescription": {
    message:
      "They get an email with a link to join as an Owner or a Member. Workspace admins invite people to their own workspaces.",
    description: "Org invite dialog description",
  },
  "organizations.fieldEmail": {
    message: "Email address",
    description: "Org invite dialog: email field label",
  },
  "organizations.fieldEmailInvalid": {
    message: "Enter a valid email address.",
    description: "Org invite dialog: malformed email",
  },
  "organizations.fieldRole": {
    message: "Role",
    description: "Org invite dialog: role field label",
  },
  "organizations.inviteSubmit": {
    message: "Send invite",
    description: "Org invite dialog: submit button",
  },
  "organizations.invited": {
    message: "Invited {email}.",
    description: "Toast after sending an org invite",
  },
  "organizations.inviteFailed": {
    message: "Couldn't send the invite.",
    description: "Org invite failure with no stated reason",
  },
  "organizations.pendingTitle": {
    message: "Pending invites",
    description: "Org Members page: the pending invite list heading",
  },
  "organizations.colEmail": {
    message: "Email",
    description: "Org pending invite table column: the invitee's email",
  },
  "organizations.revokeInvite": {
    message: "Revoke",
    description: "Org pending invite row action",
  },
  "organizations.inviteRevoked": {
    message: "Revoked the invite to {email}.",
    description: "Toast after revoking an org invite",
  },
  "organizations.revokeFailed": {
    message: "Couldn't revoke the invite.",
    description: "Org invite revoke failure with no stated reason",
  },

  "organizations.nameTitle": {
    message: "Organization name",
    description: "Org Settings page: the name card title",
  },
  "organizations.nameDescription": {
    message: "Shown in the workspace dropdown to everyone in the organization.",
    description: "Org Settings page: the name card description",
  },
  "organizations.nameSave": {
    message: "Save",
    description: "Org Settings page: save the new name",
  },
  "organizations.nameSaved": {
    message: "Renamed the organization to {name}.",
    description: "Toast after renaming the org",
  },
  "organizations.renameFailed": {
    message: "Couldn't rename the organization.",
    description: "Org rename failure with no stated reason",
  },
  "organizations.nameOwnerOnly": {
    message: "Only Owners can rename the organization.",
    description:
      "Org Settings page: why the name is read-only for a Member or Guest",
  },
  "organizations.auditTitle": {
    message: "Audit Logs",
    description:
      "Org Settings page: the audit log section title (Render's label)",
  },
  "organizations.auditDescription": {
    message:
      "Export the organization's own events, such as members, workspaces and settings, as CSV. They are not in any workspace's log, and each export spans at most {days} days.",
    description: "Org Settings page: the audit log section description",
  },

  "organizations.errorNotFound": {
    message:
      "This organization doesn't exist, or you're not one of its members.",
    description: "ORGANIZATION_NOT_FOUND refusal",
  },
  "organizations.errorForbidden": {
    message: "You don't have permission to do that in this organization.",
    description: "FORBIDDEN refusal of an org action",
  },
  "organizations.errorLastOwner": {
    message:
      "An organization needs at least one Owner. Make someone else an Owner first.",
    description: "ORGANIZATION_LAST_OWNER refusal",
  },
  "organizations.errorMemberLastAdmin": {
    message:
      "They're the last admin of {workspace}. Make someone else its admin first.",
    description: "ORGANIZATION_MEMBER_LAST_ADMIN refusal, naming the workspace",
  },
  "organizations.errorAlreadyMember": {
    message: "That person is already in the organization.",
    description: "ORGANIZATION_ALREADY_MEMBER refusal of an org invite",
  },
  "organizations.errorGuestSingleWorkspace": {
    message:
      "A Guest belongs to one workspace of the organization. Make them a Member to give them more.",
    description: "ORGANIZATION_GUEST_SINGLE_WORKSPACE refusal",
  },
  "organizations.errorOwnRole": {
    message:
      "You can't change your own organization role. Ask another Owner to change it.",
    description: "CANNOT_CHANGE_OWN_ROLE refusal",
  },
  "organizations.errorRemoveSelf": {
    message: "You can't remove yourself from the organization.",
    description: "CANNOT_REMOVE_SELF refusal",
  },
  "organizations.errorInvalidPrivacy": {
    message: "Choose Public or Invite-only.",
    description: "INVALID_PRIVACY refusal",
  },
  "organizations.securityTitle": {
    message: "Security",
    description:
      "Org Settings page: title of the login requirements card (Render's section name, where it puts Require Login Method; w7/m466)",
  },
  "organizations.securityDescription": {
    message:
      "How members must sign in, in every workspace of the organization and on its own pages, on top of each workspace's own settings.",
    description: "Org Settings page: what the Security card's settings do",
  },
  "organizations.securityOwnerOnly": {
    message: "Only Owners can change these settings.",
    description:
      "Org Settings page: why the Security settings are read-only for a Member or Guest",
  },
  "organizations.requireTwoFactorLabel": {
    message: "Require two-factor authentication",
    description:
      "Org Settings page: label of the switch that requires 2FA in every workspace of the org",
  },
  "organizations.requireTwoFactorDescription": {
    message:
      "Members without two-factor authentication (2FA) can't access the organization or any of its workspaces until they enable it. Their API keys and SSH keys are blocked until then too.",
    description: "Org Settings page: what requiring 2FA does",
  },
  "organizations.requireGoogleLoginLabel": {
    message: "Require Google login",
    description:
      "Org Settings page: label of the switch that requires signing in with Google in every workspace of the org",
  },
  "organizations.requireGoogleLoginDescription": {
    message:
      "Members who don't sign in with the Google account of their bex email address can't access the organization or any of its workspaces. API keys and SSH keys created while signed in another way are blocked too.",
    description: "Org Settings page: what requiring Google login does",
  },
  "organizations.requireTwoFactorConfirmTitle": {
    message: "Require two-factor authentication in {organization}?",
    description:
      "Title of the confirmation before an Owner requires 2FA in the org",
  },
  "organizations.requireTwoFactorConfirm": {
    message:
      "Members without two-factor authentication lose access to the organization and all of its workspaces until they turn it on, and their API keys and SSH keys stop working with them. Each workspace's own setting stays as it is.",
    description:
      "Body of the confirmation before an Owner requires 2FA in the org",
  },
  "organizations.requireGoogleLoginConfirmTitle": {
    message: "Require Google login in {organization}?",
    description:
      "Title of the confirmation before an Owner requires Google login in the org",
  },
  "organizations.requireGoogleLoginConfirm": {
    message:
      "Members who aren't signed in with the Google account of their bex email address lose access to the organization and all of its workspaces until they sign in that way, and API keys and SSH keys created while signed in another way stop working with them. Each workspace's own setting stays as it is.",
    description:
      "Body of the confirmation before an Owner requires Google login in the org",
  },
  "organizations.requireConfirmAction": {
    message: "Require",
    description:
      "Confirm button of the require-2FA and require-Google-login confirmations",
  },
  "organizations.requireTwoFactorEnabledToast": {
    message:
      "Two-factor authentication is now required in every workspace of {name}",
    description: "Toast after an Owner turns the org's 2FA requirement on",
  },
  "organizations.requireTwoFactorDisabledToast": {
    message: "{name} no longer requires two-factor authentication",
    description: "Toast after an Owner turns the org's 2FA requirement off",
  },
  "organizations.requireGoogleLoginEnabledToast": {
    message: "Google login is now required in every workspace of {name}",
    description:
      "Toast after an Owner turns the org's Google login requirement on",
  },
  "organizations.requireGoogleLoginDisabledToast": {
    message: "{name} no longer requires Google login",
    description:
      "Toast after an Owner turns the org's Google login requirement off",
  },
  "organizations.requireTwoFactorErrorOwnerNotEnrolled": {
    message:
      "Turn on two-factor authentication for your own account before requiring it in this organization, or you would lock yourself out.",
    description:
      "Refusal for code WORKSPACE_2FA_ADMIN_NOT_ENROLLED on the org setting: the acting Owner has no second factor",
  },
  "organizations.requireGoogleLoginErrorOwnerNotSignedIn": {
    message:
      "Sign in with the Google account of your bex email address before requiring Google login in this organization, or you would lock yourself out.",
    description:
      "Refusal for code WORKSPACE_GOOGLE_LOGIN_ADMIN_NOT_SIGNED_IN on the org setting: the acting Owner is not signed in with Google",
  },
  "organizations.auditNoPermission": {
    message: "You do not have permission to perform this action.",
    description:
      "Org Settings → Audit Logs: what a Member or Guest reads in place of the CSV export (Render's text)",
  },
  "organizations.auditKeysTitle": {
    message: "Audit Log API Keys",
    description:
      "Org Settings → Audit Logs: the API keys section heading (Render's)",
  },
  "organizations.auditKeysDescription": {
    message:
      "Create API keys with read-only access to all Audit Logs across this organization and its workspaces.",
    description:
      "Org Settings → Audit Logs: the API keys section description (Render's)",
  },
  "organizations.auditKeysCreate": {
    message: "Create API key",
    description:
      "Org Settings → Audit Logs: the button that opens the create dialog",
  },
  "organizations.auditKeysNoPermission": {
    message:
      "You do not have permission to manage this organization's API keys.",
    description:
      "Org Settings → Audit Logs: what a Member or Guest reads under the disabled Create API key button (Render's text)",
  },
  "organizations.auditKeysLoading": {
    message: "Loading API keys…",
    description: "Org Settings → Audit Logs: while the org's API keys load",
  },
  "organizations.auditKeysLoadFailed": {
    message: "Unable to load your organization's API keys.",
    description:
      "Org Settings → Audit Logs: the org's API keys could not be read",
  },
  "organizations.auditKeysRetry": {
    message: "Retry",
    description:
      "Org Settings → Audit Logs: reload the org's API keys after a failure",
  },
  "organizations.auditKeysEmpty": {
    message: "No provisioned API keys.",
    description: "Org Settings → Audit Logs: the org has no API keys",
  },
  "organizations.auditKeysColName": {
    message: "Name",
    description: "Org API keys table: the key's name column",
  },
  "organizations.auditKeysColKey": {
    message: "Key",
    description:
      "Org API keys table: the key's id column, shown truncated as Render's key prefix",
  },
  "organizations.auditKeysColCreated": {
    message: "Created",
    description: "Org API keys table: when the key was created",
  },
  "organizations.auditKeysColLastUsed": {
    message: "Last used",
    description: "Org API keys table: when the key was last used",
  },
  "organizations.auditKeysNeverUsed": {
    message: "Never",
    description: "Org API keys table: a key that has not been used",
  },
  "organizations.auditKeysActions": {
    message: "Actions for {label}",
    description: "Accessible name of an org API key row's actions menu",
  },
  "organizations.auditKeysRevoke": {
    message: "Revoke",
    description: "Org API key row menu: revoke the key",
  },
  "organizations.auditKeysRevokeTitle": {
    message: "Revoke API key",
    description: "Org API key revoke confirmation: title",
  },
  "organizations.auditKeysRevokeBody": {
    message:
      "“{label}” will no longer be able to access your organization's Audit Logs.",
    description:
      "Org API key revoke confirmation: what revoking does (Render's text)",
  },
  "organizations.auditKeysRevokeCancel": {
    message: "Cancel",
    description: "Org API key revoke confirmation: cancel",
  },
  "organizations.auditKeysRevokeConfirm": {
    message: "Revoke API key",
    description: "Org API key revoke confirmation: confirm",
  },
  "organizations.auditKeysRevokeFailed": {
    message: "Unable to revoke your API key. Please try again.",
    description: "Org API key revoke failed",
  },
  "organizations.auditKeysCreateTitle": {
    message: "Create an API key",
    description: "Org API key create dialog: title before the key exists",
  },
  "organizations.auditKeysCreatedTitle": {
    message: "API key created",
    description: "Org API key create dialog: title once the key exists",
  },
  "organizations.auditKeysCreateHint": {
    message: "Give your API key a unique name, like “agent” or “laptop”.",
    description: "Org API key create dialog: the name hint (Render's text)",
  },
  "organizations.auditKeysNameLabel": {
    message: "API key name",
    description: "Org API key create dialog: the name field's label",
  },
  "organizations.auditKeysNamePlaceholder": {
    message: "name",
    description: "Org API key create dialog: the name field's placeholder",
  },
  "organizations.auditKeysGenerate": {
    message: "Generate API key",
    description: "Org API key create dialog: submit",
  },
  "organizations.auditKeysClose": {
    message: "Close",
    description: "Org API key create dialog: close before a key exists",
  },
  "organizations.auditKeysDone": {
    message: "Done",
    description: "Org API key create dialog: close once the key is shown",
  },
  "organizations.auditKeysSaveNow": {
    message: "Save this API key now. You won’t be able to see it again.",
    description:
      "Org API key create dialog: the key is shown only once (Render's text)",
  },
  "organizations.auditKeysKeyLabel": {
    message: "API key",
    description: "Org API key create dialog: the label of the new key's field",
  },
  "organizations.auditKeysCopy": {
    message: "Copy API key",
    description: "Org API key create dialog: copy the new key",
  },
  "organizations.auditKeysCopied": {
    message: "Copied",
    description: "Org API key create dialog: the key was copied",
  },
  "organizations.auditKeysCopyFailed": {
    message: "Couldn't copy the key.",
    description: "Org API key create dialog: copying the key failed",
  },
  "organizations.auditKeysCreateFailed": {
    message: "Unable to create your API key. Please try again.",
    description: "Org API key create failed",
  },
  "organizations.auditKeysTooMany": {
    message: "You have created too many keys. Please try again later.",
    description: "Org API key create refused: too many keys (Render's text)",
  },
  "organizations.memberActions": {
    message: "Actions for {identity}",
    description: "Accessible name of an org member row's actions menu",
  },
  "organizations.viewDetails": {
    message: "View details",
    description: "Org member row menu: open the member's details",
  },
  "organizations.removeMemberItem": {
    message: "Remove member",
    description: "Org member row menu: remove the member from the org",
  },
  "organizations.detailsOwner": {
    message: "Owner",
    description: "Org member details: shown under an Org Owner's name",
  },
  "organizations.detailsWorkspaces": {
    message: "Workspaces",
    description: "Org member details: the member's workspaces section heading",
  },
  "organizations.detailsNoWorkspaces": {
    message: "No workspaces",
    description: "Org member details: the member belongs to no workspace",
  },
  "organizations.detailsNoWorkspacesBody": {
    message: "This member has not been invited to a workspace.",
    description: "Org member details: the member belongs to no workspace",
  },
  "organizations.detailsLoginStatus": {
    message: "Login & Authentication Status",
    description:
      "Org member details: the login methods and 2FA section heading (Render's)",
  },
  "organizations.detailsLoginStatusLoading": {
    message: "Loading login status…",
    description: "Org member details: while the member's login status loads",
  },
  "organizations.loginMethodPassword": {
    message: "Password",
    description: "Org member details: the password login method",
  },
  "organizations.loginSignedIn": {
    message: "Signed in",
    description:
      "Org member details: the member has signed in with this method",
  },
  "organizations.loginNotSignedIn": {
    message: "Not signed in",
    description:
      "Org member details: the member has not signed in with this method",
  },
  "organizations.loginTwoFactor": {
    message: "Two-factor Authentication",
    description: "Org member details: the 2FA row",
  },
  "organizations.loginTwoFactorEnabled": {
    message: "Enabled",
    description: "Org member details: the member has 2FA",
  },
  "organizations.loginTwoFactorNotEnabled": {
    message: "Not enabled",
    description: "Org member details: the member has no 2FA",
  },
  "organizations.detailsClose": {
    message: "Close",
    description: "Org member details: close the dialog",
  },
};

export default enOrganizations;
