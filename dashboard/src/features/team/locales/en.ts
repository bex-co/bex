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
  "team.searchLabel": {
    message: "Search members",
    description: "Accessible label for the accepted-member search field",
  },
  "team.searchPlaceholder": {
    message: "Search members",
    description: "Placeholder for the accepted-member search field",
  },
  "team.emptyTitle": {
    message: "No members yet",
    description: "Accepted-member table empty-state title",
  },
  "team.emptyBody": {
    message: "Invite someone to start collaborating in this workspace.",
    description: "Accepted-member table empty-state description",
  },
  "team.noMatchesTitle": {
    message: "No matching members",
    description: "Accepted-member search no-results title",
  },
  "team.noMatchesBody": {
    message: "Try a different email or identity.",
    description: "Accepted-member search no-results description",
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
  "team.inviteErrorPlanTitle": {
    message: "Your plan can't take this invite",
    description:
      "Title of the inline alert when the workspace plan's member cap or role set blocks an invite",
  },
  "team.inviteErrorPlanCta": {
    message: "Change plan",
    description:
      "Link from the blocked-invite alert to the workspace settings plan section",
  },
  "team.inviteErrorPlanLimitSeats": {
    message:
      "The {plan} plan is limited to {limit} workspace member(s). Upgrade to invite more.",
    description:
      "Plan seat-cap refusal in the invite dialog — shown when accepted members + pending invites reach the plan maximum",
  },
  "team.inviteErrorPlanLimitRole": {
    message:
      "The {plan} plan doesn't offer this role. Upgrade to access all roles.",
    description:
      "Plan role-gate refusal in the invite dialog — shown when the chosen role isn't available on the current plan",
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
    message: "{identity} will lose access to this workspace immediately.",
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
  "team.resendInvite": {
    message: "Resend",
    description: "Resend pending-invite button (fresh email, refreshed expiry)",
  },
  "team.resendInviteSuccess": {
    message: "Invitation re-sent to {email}",
    description: "Toast after a successful invite resend",
  },
  "team.resendInviteError": {
    message: "Couldn't resend that invitation",
    description: "Toast after a failed invite resend",
  },
  "team.seatUsage": {
    message: "{used} of {limit} seats",
    description:
      "Seat usage in the Team card title on a limited plan — accepted members plus pending invites over the plan cap",
  },
  "team.seatCount": {
    message: "{used} seats used",
    description:
      "Seat count in the Team card title on an unlimited plan (no cap to show)",
  },
  "team.seatsFullBody": {
    message:
      "This workspace has used every seat its plan offers. Upgrade to invite more members.",
    description:
      "Invite dialog wall when seats are exhausted before composing an invite",
  },
  "team.mfaEnabled": {
    message: "2FA",
    description:
      "Badge on a member row whose account has a second factor enrolled",
  },
  "team.mfaEnabledTooltip": {
    message: "Two-factor authentication enabled",
    description: "Tooltip for the member-row 2FA badge",
  },
  "team.inviteAccepted": {
    message: "You've joined {workspace}",
    description: "Toast after an emailed invite link is redeemed successfully",
  },
  "team.inviteAcceptedAlready": {
    message: "That invitation was already used",
    description: "Toast when an invite token was already redeemed",
  },
  "team.inviteAcceptExpired": {
    message: "That invitation has expired — ask for a new one",
    description: "Toast when an invite token is past its expiry",
  },
  "team.inviteAcceptError": {
    message: "Couldn't accept that invitation",
    description: "Toast when redeeming an invite token fails",
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
    message:
      "Something went wrong loading this workspace's members. Try again.",
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
