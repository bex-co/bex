# Render: who an enforced login requirement locks out, and 2FA reminders

Captured 2026-11-26 for `w1/m647` from Render's public dashboard bundle (read without sign-in), and checked the same day in a headed browser signed in to a Render Pro workspace. That workspace has one member, so the notice could not be seen live, and enforcement was not toggled: the live check confirmed the Security section's two switches ("Enforce Two-Factor Authentication", "Enforce Single Sign-On with Google") and nothing else. Nothing from that workspace is recorded here.

**Evidence.** The `WorkspaceSettings-*.js` chunk holds the Security section, its dialogs and their sentences, split into JSX fragments that are reassembled below. The `user-*.js` chunk holds the mutation. The hashes change with every Render deploy, so find the chunks by name from `https://dashboard.render.com/`'s `assets/index-*.js` ([metrics-page.md § Status-code label](metrics-page.md#status-code-label-w3080-2026-11-20) describes the method). The only feature-flag hook `WorkspaceSettings` imports serves its Early Access section, whose list is empty, so every Render workspace has this flow ([dashboard-operations.md](dashboard-operations.md)).

bex's implementation is [ADR012 §9d](../ADR012-auth.md#9d-who-an-enforced-requirement-locks-out-and-2fa-reminders-w1m647).

## Who sees it

- The Security section renders only for a member with Render's `updateTeam2FA` or `updateTeamAllowedLoginMethods` permission (an admin). Everyone else sees no Security section, so no notice.
- Below the Pro plan the section is disabled, with "Security features require a Pro plan or higher to enable" ("a Professional plan" on Render's legacy plans) and an Upgrade button.
- Render computes who is missing in the browser: from the team's members, those whose `user.otpEnabled` is false (2FA) or whose `user.hasGoogleId` is false (Google). Every member is checked, the acting admin included.

## The mutation

```graphql
mutation sendReminder2FAEnforcementNotificationMutation($teamId: String!) {
  sendReminder2FAEnforcementNotification(teamId: $teamId)
}
```

Its result is not read. An error whose message is exactly `rate limited` opens the limit dialog; any other error shows the dialog's apology. Google login has no reminder operation.

## Turning it on

The precondition dialog, before the confirmation, when the admin does not meet the requirement themselves:

| Requirement | Heading | Body | Buttons |
| --- | --- | --- | --- |
| 2FA | "You do not have two-factor authentication enabled" | "You cannot enforce two-factor authentication for your team until you've enabled it for yourself." | Close · "Go to settings" (`/u/settings#security`) |
| Google | "You haven't signed in with Google" | "You cannot enforce single sign-on for your team until you've signed in with Google." | Close · "Sign in with Google" |

The confirmation:

- Title: "Confirm two-factor authentication requirement across team"; Google: "Confirm single sign-on with Google requirement across team".
- Description, with someone missing: "Members who do not have two-factor authentication enabled will **lose access** to the workspace and its resources." Google: "Members who have not signed in with Google will **lose access** to the workspace and its resources."
- Description, with nobody missing: "Members will need to enable two-factor authentication to access this team and its resources going forward." Google: "Members will need to sign in with Google to access this team and its resources going forward."
- With someone missing, a heading: "**N** members will be losing access to this workspace" ("1 member will be losing access to this workspace").
- Under it, for 2FA only: "Reminders to enable 2FA will be sent to these members." For one: "A reminder to enable 2FA will be sent to this member."
- The members' list: an avatar and the email, in a box that scrolls past 12.5 rem.
- On failure: "We're sorry. We weren't able to enforce two-factor authentication for this team. Try again?" Google: "… enforce single sign-on with Google for this team. Try again?"
- Buttons: Cancel · "Confirm and enforce".

The confirmation's reminder line is the evidence that enforcing 2FA sends the reminders itself: the dashboard sends nothing after the enforce mutation, so the server does.

## While it is enforced

Under the switch, when at least one member does not meet it, an inline warning:

| Requirement | Count 1 | Count N |
| --- | --- | --- |
| 2FA | "**1 member** of your team has not enabled two-factor authentication yet." | "**N members** of your team have not enabled two-factor authentication yet." |
| Google | "**1 member** of your team has not signed in with Google yet." | "**N members** of your team have not signed in with Google yet." |

Its button (secondary, small): "View member" / "View members".

When every member meets it, a success callout: "**All members** of your team have enabled two-factor authentication!" Google: "**All members** of your team have signed in with Google!"

When it is not enforced, neither shows.

## The members dialog

- Title: "Members without two-factor authentication"; Google: "Members without Google sign-on".
- Description: "These members are currently prohibited from accessing this team workspace and the resources within it." For one: "This member is currently prohibited from accessing this team workspace and the resources within it."
- The list: an avatar and the email of each, scrolling past 12.5 rem.
- For 2FA only, an informative callout. Title: "Remind these users to enable 2FA?" ("Remind this user to enable 2FA?"). Message: "Send out email reminders to these users." ("Send out an email reminder to this user."). Button (secondary, small): "Send reminders" ("Send reminder"), disabled with a spinner while it sends.
- On a failure other than the limit: "We're sorry. We weren't able to send reminders to these team members. Try again?"
- Footer: Close.

After a send:

- Success: the dialog closes, and a toast reads "Email reminders sent to members without two-factor authentication!"
- `rate limited`: the dialog closes, and a small dialog opens with a warning icon, the heading "Remind limit has been reached!", the text "It looks like we've already sent out reminders for today. You can send reminders again tomorrow." and a Close button.

## What the bundle does not say

- **The limit's day.** "for today … again tomorrow" names no time zone. bex counts the UTC day.
- **Whether enforcing counts toward the limit.** The dashboard cannot tell. bex counts it: the batch turning 2FA on is the day's batch.
- **The email.** Its subject and body are the server's, outside the bundle. bex writes its own, naming the workspace and linking to account security.
- **Audit.** Render's audit log documents no reminder event. bex records one row per reminder sent, under its own verb.
