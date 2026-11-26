# Render: account and workspace avatars

Captured 2026-11-26 for `w1/m648` from Render's public dashboard bundle (read without sign-in), and checked the same day in a headed browser signed in to a Render Pro workspace: Account Settings → Profile → Avatar (its Edit menu offers "Upload image" and "Change monogram"), the "Edit Monogram" dialog (a Text field, eight color swatches, Cancel and Save), Workspace Settings → General → Avatar (a monogram), the account menu showing a Gravatar and the workspace switcher showing the workspace's monogram. Nothing was saved, and nothing from that workspace is recorded here.

**Evidence.** The chunks are `AvatarRow-*.js` (the row, its menu, the crop dialog, the upload and the toasts), `EditMonogramModal-*.js`, `UserSettings-*.js` (Account Settings → Profile), `WorkspaceSettings-*.js` (Workspace Settings → General), `owner-*.js` (the operations), `Avatar-*.js` and `WorkspaceAvatar-*.js` (the two renderers) and `avatar-utils-*.js` (Gravatar, the palette, the hash and the miss cache). The hashes change with every Render deploy, so find the chunks by name from `https://dashboard.render.com/`'s `assets/index-*.js` ([metrics-page.md § Status-code label](metrics-page.md#status-code-label-w3080-2026-11-20) describes the method). None of them imports a feature-flag hook, so every Render workspace has avatars ([dashboard-operations.md](dashboard-operations.md)).

Render's docs ([render.com/docs/render-dashboard](https://render.com/docs/render-dashboard), read 2026-11-25): "You can upload a custom image or set a text monogram." "By default, Render uses the Gravatar image for your account's email address. If you don't have a Gravatar image, Render sets a text monogram using the first letter of your email address." "Only workspace admins can set the workspace image." "By default, Render sets a text monogram using the first letter of your workspace's name."

## One stored image per owner

A monogram is not a setting. Saving the Edit Monogram dialog renders the monogram element to a canvas, encodes it as a JPEG and uploads it through the same presigned upload as a photo. If the chosen letter and color equal the default monogram, Save calls the reset instead. So an owner has an uploaded image or nothing, and the server keeps no monogram choice.

## The operations

```graphql
query avatarPresignedUpload(
  $avatarOwnerId: String!
  $avatarType: AvatarType!
  $avatarSizeBytes: Int!
) {
  avatarPresignedUpload(
    avatarOwnerId: $avatarOwnerId
    avatarType: $avatarType
    avatarSizeBytes: $avatarSizeBytes
  ) {
    url
    verb
  }
}

mutation deleteAvatar($avatarOwnerId: String!, $avatarType: AvatarType!) {
  deleteAvatar(avatarOwnerId: $avatarOwnerId, avatarType: $avatarType)
}
```

- `avatarPresignedUpload` is a query, not a mutation. The browser then calls `fetch(url, { method: verb, body: blob, headers: { "Content-Type": "image/jpeg" } })`.
- `AvatarType` is `ACCOUNT | WORKSPACE`. There is no organization value: Render has no organization avatar anyone can set. The organization pages only show the avatars of its members and workspaces (`memberAvatarsForOrganization`, `workspaceAvatarsForOrganization`, `avatarsForWorkspaceMembers`, `organizationsForUserWithAvatars`).
- `avatarOwnerId` is the user's id for `ACCOUNT` and the workspace's id for `WORKSPACE`.
- The image is read back as `avatarDownload { url }` (type `PresignedDownload`) on `user`, on `owner` (`currentWorkspaceForNav`), on `workspacesForUser`, and on the member and workspace objects of the organization queries.
- After an upload, the dashboard puts the uploaded blob's data URL in its cache as `avatarDownload.url`, so the new image shows at once, then refetches. A workspace upload also refetches `currentWorkspaceForNav` and `organizationsForUserWithAvatars`.

## The Avatar row

- Account Settings → Profile holds Full Name, Email, then **Avatar**. Workspace Settings → General holds Name, Email, then **Avatar** (its permission is `updateWorkspaceAvatar`), then Plan.
- The row shows the avatar at `xlarge` (64 px) and an **Edit** button (a pencil icon and "Edit"). A member without permission sees the button disabled, with the permission message as its tooltip.
- Edit opens a menu: **Upload image**, **Change monogram**, and, only when the avatar showing is an upload (a saved monogram counts), **Reset to default** in the danger style.
- Reset when the avatar showing is not an upload does nothing on the server.

## Upload image

- A hidden `<input type="file" accept="image/png, image/jpeg">`.
- A file whose type is neither `image/png` nor `image/jpeg`: error toast **Invalid file type** / "Please select a JPG, JPEG, or PNG file."
- The file is read as a data URL. One longer than 20,971,520 characters: **File too large** / "Please select a file under 20 MB."
- A read that does not produce a string: **Unable to read file** / "The browser could not read your file. Please try again."
- Otherwise the **Crop Avatar** dialog opens: a 400 × 400 cropper with a round crop of aspect 1, no grid, the image fitted to contain; zoom from 1 to 5 in steps of 0.1 through a slider labelled "Drag image to move. Use slider to zoom." (label hidden) between **Zoom out** and **Zoom in** buttons; footer **Cancel** and **Save**.
- Save draws the crop onto a 128 × 128 canvas filled white first, and encodes it as `image/jpeg`. That blob is what is uploaded, so whatever the source file, the stored avatar is a small JPEG.
- A failed presign or PUT: **Unable to upload file** / "There was a problem connecting to the server. Please try again."

## Change monogram

- The **Edit Monogram** dialog: a 64 px preview of the monogram, a **Text** field, and a **Color** radio group of eight round swatches, each labelled with its color's name. Footer **Cancel** and **Save**.
- The field keeps only its first character, upper-cased. Save is disabled unless the text is exactly one character (after trimming) and a palette color is chosen.
- It opens on the default monogram: the letter and color the owner shows today without an upload.
- A failure drawing it: **Unable to generate Monogram** / "There was a problem generating your monogram. Please try again."
- A failed reset: **Unable to delete file** / "There was a problem connecting to the server. Please try again."

## Defaults

| Owner | Default | Monogram letter | Monogram color hashed from |
| --- | --- | --- | --- |
| Account | its Gravatar, else a monogram | the email's first letter, upper-cased | the email (trimmed, lower-cased) |
| Workspace | a monogram | the first letter of the workspace's name (its email when it has no name), upper-cased | the workspace's id |

- **Palette**, in order: blue, green, lime green, orange, pink, purple, red, yellow, each Tailwind's `bg-<color>-100 text-<color>-700`.
- **Hash**: Java's string hash, `h = (h << 5) - h + charCode` over the UTF-16 code units in 32-bit arithmetic, then `Math.abs(h % 8)` picks the color. An empty string is index 0.
- **Gravatar**: `//en.gravatar.com/avatar/<sha256 of the trimmed, lower-cased email>?s=<size in px, doubled on a high-DPI screen>&r=<rating, default x>&d=404`. Sizes: tiny 12, mini 16, small 20, medium 24, large 32, xlarge 64.
- **Miss cache**: a Gravatar that fails to load is remembered for 5 minutes (`3e5` ms); within that time the monogram shows without a request. A hit is remembered for the page's lifetime. While the probe is in flight, the monogram shows.
- **Shape**: avatars are round. The dialog's hidden render (the one uploaded) is the square `xlarge` monogram.
- An uploaded image that fails to load falls back to the default.
- Alt text: "User-uploaded Avatar for <email>" (an upload), "Gravatar for <email>" (a Gravatar).

## Where Render shows avatars

The chunks that render the `Avatar` component: `CancelReasonText` (who canceled a deploy), `Connect`, `EditMonogramModal`, `FailureReasonFragment`, `Members` (team members), `OAuthConsentGate`, `ServiceSharing`, `Settings`, `UserSettings`, `WorkspaceAvatar`, `WorkspaceSettings` (the avatar row, registry credentials' "created by", the 2FA and Google member lists), `Workspaces`, `event-timeline-utils` (who triggered, canceled, suspended or resumed, on the events feed), `index` (the account menu and the workspace switcher) and `page-templates`. `avatarDownload` is also read by `ApiTokenHeader`, `ArtifactDrawer`, `BuildContainer`, `BuildDetails`, `CronJob` and `CronJobDeployContainer` (a cron run's "triggered by") and `NewServiceSourceCode`.

## REST and MCP

Render's public REST API has no avatar route, and its MCP server no avatar tool.
