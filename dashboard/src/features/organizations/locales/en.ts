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
};

export default enOrganizations;
