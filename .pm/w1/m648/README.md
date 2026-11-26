# w1 · m648 — Users and workspaces have avatars, as on Render

**Worker:** worker1 **Goal:** a user can upload an avatar image, set a text monogram or reset to the default (Gravatar, then a monogram of the email's first letter). A workspace admin can do the same for the workspace (default: a monogram of its name's first letter). The avatars show where Render shows them. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Capture Render's avatars: operations, limits, defaults, display sites](t001.md) | 30m | — |
| t002 | [Storage and GraphQL: upload, monogram, reset, avatar URLs](t002.md) | 2h30m | t001 |
| t003 | [Avatar rows in Account Settings and Workspace Settings](t003.md) | 1h45m | t002 |
| t004 | [Show avatars where Render shows them](t004.md) | 1h30m | t003 |
| t005 | [Production walk and docs](t005.md) | 30m | t004 |
| t006 | [Render parity](t006.md) | 20m | t005 |
| t007 | [Simplify](t007.md) | 20m | t006 |
| t008 | [Test coverage](t008.md) | 45m | t006 |
| t009 | [Closeout](t009.md) | 10m | t008 |

## Definition of done

1. On production, the QA user can do each of these, and each change shows in the account menu without a reload:
   - upload a PNG avatar;
   - replace it with a monogram;
   - reset it to the default.
2. As admin of `bex-canary`, the QA user can set and reset the workspace avatar. The workspace switcher shows it. A non-admin sees the row without the controls.
3. A user with no Gravatar and no upload shows the monogram of the email's first letter, in a color derived from the email.
4. An oversized or non-image upload is refused with a message.
5. ADR024's "bex has no avatar upload anywhere" is replaced, and ADR018 has an Avatars row.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one more aspect of render parity for w4` (2026-11-25), item 5. On the user's request it was checked in a headed browser the same day and filed to w1.
  - **Render's docs:** [render.com/docs/render-dashboard](https://render.com/docs/render-dashboard), read 2026-11-25:
    - "You can upload a custom image or set a text monogram."
    - "By default, Render uses the Gravatar image for your account's email address. If you don't have a Gravatar image, Render sets a text monogram using the first letter of your email address."
    - "Only workspace admins can set the workspace image."
    - "By default, Render sets a text monogram using the first letter of your workspace's name."
  - **Render's bundle** (public, no flag gate):
    - `AvatarRow-*.js` and `UserSettings-*.js`: image upload, monogram and reset, with `defaultMonogram` taking the email's first letter, upper-cased, and a color hashed from the email;
    - `owner-*.js`: `avatarPresignedUpload`, `deleteAvatar`, `user { avatarDownload { url } }`, `updateTeamProfile` and `organizationsForUserWithAvatars`;
    - `avatar-utils-*.js`: the Gravatar URL `//en.gravatar.com/avatar/<sha256 of the trimmed, lower-cased email>?s=…&d=404`, with a 5-minute cache of misses before it falls back to the monogram.
  - **bex today:** checked on production on 2026-11-25. Account Settings and Workspace Settings have no avatar row, and the account menu shows an initial fallback (`dashboard/src/common/components/user-nav.tsx`). [ADR024](../../../docs/ADR024-members.md) says "bex has no avatar upload anywhere", and ADR018 has no row for avatars and no sign-off for that divergence.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); identity and workspace settings (`docs/ADR018/identity-workspaces-account.md`).
- **Expected outcome:** people and workspaces are recognizable at a glance in the menu, the switcher, member lists and the "triggered by" rows, as on Render.
- **Why now:** an unrecorded divergence on two settings pages everyone opens. The parity rule requires matching it or recording it with sign-off, and no sign-off exists.
- **Gravatar:** the default follows Render. The browser requests a hash of the user's email from gravatar.com, so the dashboard's CSP `img-src` must allow `en.gravatar.com`. A monogram-only default would be a divergence that needs the user's sign-off in ADR018. If wanted, file it as a separate follow-up rather than holding this milestone (AGENTS.md: never park parity on an optional decision).
- **Render parity included:** GraphQL and dashboard change. Render's REST API and MCP have no avatar operation, and the parity task confirms bex adds none.
