import type { TranslationEntry } from "@/i18n";

const enTeam: Record<string, TranslationEntry> = {
  "team.title": {
    message: "Team",
    description: "Settings Team section card title",
  },
  "team.description": {
    message:
      "The people in this workspace and what they can do. Invite teammates by email and assign a role; roles are enforced across every resource.",
    description: "Settings Team section card description",
  },
  "team.colMember": {
    message: "Member",
    description: "Team table column header — the member identity",
  },
  "team.colRole": {
    message: "Role",
    description: "Team table column header — the member's role",
  },
  "team.colEmail": {
    message: "Email",
    description: "Pending invites table column header",
  },
  "team.pendingTitle": {
    message: "Pending invites",
    description: "Heading above the pending-invites table",
  },
  "team.invite": {
    message: "Invite",
    description: "Button that opens the invite dialog",
  },
  "team.inviteTitle": {
    message: "Invite a teammate",
    description: "Invite dialog title",
  },
  "team.inviteDescription": {
    message:
      "They'll get an email and join this workspace the first time they sign in with that address.",
    description: "Invite dialog description",
  },
  "team.fieldEmail": {
    message: "Email address",
    description: "Invite dialog email field label",
  },
  "team.fieldEmailPlaceholder": {
    message: "teammate@example.com",
    description: "Invite dialog email field placeholder",
  },
  "team.fieldRole": {
    message: "Role",
    description: "Invite dialog role field label",
  },
  "team.inviteCancel": {
    message: "Cancel",
    description: "Invite dialog cancel button",
  },
  "team.inviteSubmit": {
    message: "Send invite",
    description: "Invite dialog submit button",
  },
  "team.inviteSuccess": {
    message: "Invitation sent to {email}",
    description: "Toast after a successful invite",
  },
  "team.inviteError": {
    message: "Couldn't invite {email}",
    description: "Toast after a failed invite",
  },
  "team.inviteErrorPlan": {
    message: "Your plan's member limit is reached — upgrade to invite more.",
    description: "Toast when the workspace plan's member cap blocks an invite",
  },
  "team.remove": {
    message: "Remove",
    description: "Remove-member button / confirm label",
  },
  "team.removeTitle": {
    message: "Remove member?",
    description: "Remove-member confirmation dialog title",
  },
  "team.removeConfirm": {
    message: "{subject} will lose access to this workspace immediately.",
    description: "Remove-member confirmation dialog body",
  },
  "team.removeCancel": {
    message: "Cancel",
    description: "Remove-member confirmation cancel button",
  },
  "team.removeSuccess": {
    message: "Member removed",
    description: "Toast after a successful remove",
  },
  "team.removeError": {
    message: "Couldn't remove that member",
    description: "Toast after a failed remove",
  },
  "team.roleChanged": {
    message: "Role updated",
    description: "Toast after a successful role change",
  },
  "team.roleChangeError": {
    message: "Couldn't change that role",
    description: "Toast after a failed role change",
  },
  "team.lastAdminError": {
    message: "A workspace must keep at least one admin.",
    description: "Toast when the last-admin guard refuses a demote/remove",
  },
  "team.revokeInvite": {
    message: "Revoke",
    description: "Revoke pending-invite button",
  },
  "team.revokeInviteSuccess": {
    message: "Invitation revoked",
    description: "Toast after a successful invite revoke",
  },
  "team.revokeInviteError": {
    message: "Couldn't revoke that invitation",
    description: "Toast after a failed invite revoke",
  },
  "team.errorTitle": {
    message: "Couldn't load the team",
    description: "Team panel generic error title",
  },
  "team.errorBody": {
    message: "Something went wrong loading this workspace's members. Try again.",
    description: "Team panel generic error body",
  },
  "team.role.VIEWER": {
    message: "Viewer",
    description: "Role label — read-only",
  },
  "team.role.CONTRIBUTOR": {
    message: "Contributor",
    description: "Role label — works on resources, no sensitive fields",
  },
  "team.role.DEVELOPER": {
    message: "Developer",
    description: "Role label — full resource access",
  },
  "team.role.ADMIN": {
    message: "Admin",
    description: "Role label — full access incl. settings, members, billing",
  },
  "team.role.BILLING": {
    message: "Billing",
    description: "Role label — billing only",
  },
};

export default enTeam;
