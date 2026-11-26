# w1 · m647 — Once 2FA or Google login is enforced, admins see who is locked out and can remind them, as on Render

**Worker:** worker1 **Goal:** with a login requirement enforced, Workspace Settings keeps showing Render's notice of how many members do not meet it. A dialog lists them, and for 2FA an admin can email reminders, at most once per workspace per day, as on Render. Members locked out by a new enforcement get a reminder automatically. **Status:** todo (t001–t004, t006–t008 done; t005's dev-1 walk next)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Capture Render's enforcement notice, member list and reminders](done/t001.md) — **DONE** | 30m | — |
| t002 | [GraphQL: non-compliant members and a once-a-day reminder mutation](done/t002.md) — **DONE** | 1h30m | t001 |
| t003 | [The reminder email](done/t003.md) — **DONE** | 1h | t002 |
| t004 | [Workspace Settings: notice, member dialog, Send reminders](done/t004.md) — **DONE** | 1h30m | t002 |
| t005 | [Verify on dev-N with Mailpit and on production](t005.md) | 45m | t003, t004 |
| t006 | [Render parity](done/t006.md) — **DONE** | 20m | t005 |
| t007 | [Simplify](done/t007.md) — **DONE** | 20m | t006 |
| t008 | [Test coverage](done/t008.md) — **DONE** | 45m | t006 |
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

## Render parity (t006, 2026-11-26)

Checked against [workspace-2fa-reminders.md](../../../docs/render-artifacts/workspace-2fa-reminders.md), Render's dashboard bundle and a headed look at a Render Pro workspace's Security section (one member, so its notice could not show live).

| Surface | Render | bex | Verdict |
| --- | --- | --- | --- |
| Notice, someone missing | "**N members** of your team have not enabled two-factor authentication yet." / Google "… signed in with Google yet.", singular forms, "View member(s)" | Same words, singular and plural, 8 languages | match |
| Notice, all compliant | "**All members** of your team have enabled two-factor authentication!" / "… signed in with Google!" | Same | match |
| Who sees it | Admins (the Security section is theirs), only while enforced | Admins, while the workspace or its org requires it; other members see no notice | match (an org requirement is bex's §9c) |
| Member dialog | Title, "These members are / This member is currently prohibited …", list, Close | Same words; list shows the email | avatars missing: w1/m648 |
| 2FA reminder callout | "Remind these users / this user to enable 2FA?", "Send out (an) email reminder(s) to …", "Send reminder(s)" | Same | match |
| Google reminder | none | none | match |
| Toast | "Email reminders sent to members without two-factor authentication!" | Same | match |
| Daily limit | `rate limited` → "Remind limit has been reached!" / "It looks like we've already sent out reminders for today. You can send reminders again tomorrow." | 429 `WORKSPACE_2FA_REMINDER_LIMIT` → the same dialog; the day is UTC (Render names no zone) | match; the zone recorded in ADR012 §9d |
| Send failure | "We're sorry. We weren't able to send reminders to these team members. Try again?" | Same, in the dialog | match |
| Enforcing 2FA | Confirmation says "Reminders to enable 2FA will be sent to these members."; the server sends them | Same line in the confirmation; `setWorkspaceRequireTwoFactor` sends the day's batch | match |
| Confirmation title, description, button; precondition dialog | "Confirm two-factor authentication requirement across team", "Confirm and enforce", a dialog before it when the admin lacks 2FA | w7/m471's own words and an inline refusal | drift: filed as [w1/158](../158.md) |
| GraphQL | `sendReminder2FAEnforcementNotification(teamId)`; the list from team members' `otpEnabled`/`hasGoogleId` | `sendWorkspaceTwoFactorReminders(workspaceId)`; `workspaceNonCompliantMembers(workspaceId, requirement)` (admin only) | bex names, ledger row `matched` |
| REST, MCP | no operation | none added | match |
| Audit | no event | one row per reminder, bex verb `workspaces.SendTwoFactorReminders` | bex addition (DoD 4), recorded in ADR018 |

ADR018's two enforcement rows ([identity-workspaces-account.md](../../../docs/ADR018/identity-workspaces-account.md)) and the dashboard operations ledger record the result.
