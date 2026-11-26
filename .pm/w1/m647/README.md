# w1 · m647 — Once 2FA or Google login is enforced, admins see who is locked out and can remind them, as on Render

**Worker:** worker1 **Goal:** with a login requirement enforced, Workspace Settings keeps showing Render's notice of how many members do not meet it. A dialog lists them, and for 2FA an admin can email reminders, at most once per workspace per day, as on Render. Members locked out by a new enforcement get a reminder automatically. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Capture Render's enforcement notice, member list and reminders](t001.md) | 30m | — |
| t002 | [GraphQL: non-compliant members and a once-a-day reminder mutation](t002.md) | 1h30m | t001 |
| t003 | [The reminder email](t003.md) | 1h | t002 |
| t004 | [Workspace Settings: notice, member dialog, Send reminders](t004.md) | 1h30m | t002 |
| t005 | [Verify on dev-N with Mailpit and on production](t005.md) | 45m | t003, t004 |
| t006 | [Render parity](t006.md) | 20m | t005 |
| t007 | [Simplify](t007.md) | 20m | t006 |
| t008 | [Test coverage](t008.md) | 45m | t006 |
| t009 | [Closeout](t009.md) | 10m | t008 |

## Definition of done

1. On a dev-N stack with two members, an admin enforces 2FA. The other member, who has no 2FA, gets the reminder email in Mailpit. The notice and the member dialog name that member. A second Send reminders the same UTC day shows Render's limit message, and no email goes out.
2. With Google login enforced, the notice and the list appear without a reminder action, as on Render.
3. On production, the notice and the dialog render for a disposable workspace with a non-compliant member. This holds if the run can field a second production identity; if it cannot, t005 records that, and the dev-N walk stands as the acceptance.
4. Every reminder sent leaves an audit-log entry.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one more aspect of render parity for w4` (2026-11-25), item 4. On the user's request it was checked in a headed browser the same day and filed to w1.
  - **Render's strings:** read from Render's public dashboard bundle (`WorkspaceSettings-*.js`), where they are assembled from fragments:
    - "… will be losing access to this workspace";
    - "A reminder … to enable 2FA will be sent to …" and "Confirm and enforce";
    - "Members without two-factor authentication" and "Members without Google sign-on";
    - "… currently prohibited from accessing this team workspace and the resources within it.";
    - "Send out an email reminder to this user";
    - "Remind limit has been reached!";
    - "Email reminders sent to members without two-factor authentication!";
    - "… of your team have enabled two-factor authentication!".
  - **Render's mutation** is `sendReminder2FAEnforcementNotification(teamId)` (`user-*.js`).
  - **No feature flag:** the only flag hook WorkspaceSettings imports serves its Early Access section, whose list is empty. Every Render workspace has this flow.
  - **bex today:** checked on production on 2026-11-25. The Two-factor authentication and Google login cards each hold an Enforce switch, and nothing else. The affected members are listed only in the confirmation at the moment of enforcing ([w7/m471](../../w7/done/m471/README.md), `login-requirement-impact-list.tsx`), and no reminder email exists in `lego/backend/internal/email`.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first). This extends ADR018's "Workspace-enforced 2FA" and "Workspace-enforced Google login" rows (`docs/ADR018/identity-workspaces-account.md`, w22/m402 and w7/m465).
- **Expected outcome:**
  - After enforcing, an admin sees who is locked out and can nudge them, without having to remember who was on the confirmation list.
  - Locked-out members learn why from an email, not from a refusal page.
- **Why now:** enforcement shipped in November (w22/m402, w7/m465, w7/m471). Every workspace that turns it on now strands members silently, which Render avoids.
- **Render parity included:** GraphQL, dashboard and email change. Render's REST and MCP have no such operation, and the parity task confirms they stay absent.
