import type { TranslationEntry } from "@/i18n";

const enInvites: Record<string, TranslationEntry> = {
  "invites.title": {
    message: "Workspace invitation",
    description: "Invitation review: title",
  },
  "invites.authenticate": {
    message:
      "Create an account or sign in to review your workspace invitation.",
    description: "Invitation review: authenticate",
  },
  "invites.signUp": {
    message: "Sign up",
    description: "Invitation review: signUp",
  },
  "invites.signIn": {
    message: "Sign in",
    description: "Invitation review: signIn",
  },
  "invites.joinTitle": {
    message: "Join {workspace}",
    description: "Invitation review: joinTitle",
  },
  "invites.memberTitle": {
    message: "You’re a member of {workspace}",
    description: "Invitation review: memberTitle",
  },
  "invites.role": {
    message: "You’ll join as a {role}.",
    description: "Invitation review: role",
  },
  "invites.memberRole": {
    message: "Your role is {role}.",
    description: "Invitation review: memberRole",
  },
  "invites.inviter": {
    message: "Invited by {email}",
    description: "Invitation review: inviter",
  },
  "invites.account": {
    message: "Signed in as {email}",
    description: "Invitation review: account",
  },
  "invites.join": {
    message: "Join workspace",
    description: "Invitation review: join",
  },
  "invites.joining": {
    message: "Joining…",
    description: "Invitation review: joining",
  },
  "invites.open": {
    message: "Open workspace",
    description: "Invitation review: open",
  },
  "invites.opening": {
    message: "Opening…",
    description: "Invitation review: opening",
  },
  "invites.notNow": {
    message: "Not now",
    description: "Invitation review: notNow",
  },
  "invites.continue": {
    message: "Continue to bex",
    description: "Invitation review: continue",
  },
  "invites.unavailableTitle": {
    message: "Invitation unavailable",
    description: "Invitation review: unavailableTitle",
  },
  "invites.invalid": {
    message:
      "This invitation link is invalid or was revoked. Ask the workspace admin for a new invitation.",
    description: "Invitation review: invalid",
  },
  "invites.expired": {
    message:
      "This invitation has expired. Ask the workspace admin to resend it.",
    description: "Invitation review: expired",
  },
  "invites.used": {
    message:
      "This invitation has already been used. Sign in with the account that joined, or ask for a new invitation.",
    description: "Invitation review: used",
  },
  "invites.planLimit": {
    message:
      "This workspace cannot accept another member on its current plan. Ask the workspace admin to check its plan and seats, then retry.",
    description: "Invitation review: planLimit",
  },
  "invites.retryError": {
    message:
      "We couldn’t complete that request. Your invitation is saved in this tab. Please try again.",
    description: "Invitation review: retryError",
  },
  "invites.accessPending": {
    message:
      "Your membership is confirmed, but workspace access isn’t ready yet. Try opening the workspace again.",
    description: "Invitation review: accessPending",
  },
  "invites.storageUnavailable": {
    message:
      "We couldn’t save your invitation in this browser. Allow site storage, then reopen the invitation link.",
    description: "Invitation review: storageUnavailable",
  },
  "invites.retry": {
    message: "Try again",
    description: "Invitation review: retry",
  },
};

export default enInvites;
