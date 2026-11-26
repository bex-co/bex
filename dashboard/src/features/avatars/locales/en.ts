import type { TranslationEntry } from "@/i18n";

// Account and workspace avatars (w1/m648, docs/render-artifacts/avatars.md).
const enAvatars: Record<string, TranslationEntry> = {
  "avatars.rowLabel": {
    message: "Avatar",
    description:
      "Label of the Avatar row in Account Settings and Workspace Settings (Render's 'Avatar')",
  },
  "avatars.edit": {
    message: "Edit",
    description: "Button that opens the avatar's menu (Render's 'Edit')",
  },
  "avatars.uploadImage": {
    message: "Upload image",
    description:
      "Avatar menu item that opens the file picker (Render's 'Upload image')",
  },
  "avatars.changeMonogram": {
    message: "Change monogram",
    description:
      "Avatar menu item that opens the Edit Monogram dialog (Render's 'Change monogram')",
  },
  "avatars.resetToDefault": {
    message: "Reset to default",
    description:
      "Avatar menu item that removes the uploaded avatar (Render's 'Reset to default')",
  },
  "avatars.invalidFileTypeTitle": {
    message: "Invalid file type",
    description: "Toast title when the chosen file is not PNG or JPEG",
  },
  "avatars.invalidFileTypeBody": {
    message: "Please select a JPG, JPEG, or PNG file.",
    description: "Toast body when the chosen file is not PNG or JPEG",
  },
  "avatars.fileTooLargeTitle": {
    message: "File too large",
    description: "Toast title when the chosen file is over 20 MB",
  },
  "avatars.fileTooLargeBody": {
    message: "Please select a file under 20 MB.",
    description: "Toast body when the chosen file is over 20 MB",
  },
  "avatars.unreadableFileTitle": {
    message: "Unable to read file",
    description: "Toast title when the browser cannot read the chosen file",
  },
  "avatars.unreadableFileBody": {
    message: "The browser could not read your file. Please try again.",
    description: "Toast body when the browser cannot read the chosen file",
  },
  "avatars.uploadFailedTitle": {
    message: "Unable to upload file",
    description: "Toast title when uploading the avatar fails",
  },
  "avatars.deleteFailedTitle": {
    message: "Unable to delete file",
    description: "Toast title when resetting the avatar fails",
  },
  "avatars.connectionFailedBody": {
    message: "There was a problem connecting to the server. Please try again.",
    description: "Toast body when uploading or resetting the avatar fails",
  },
  "avatars.monogramFailedTitle": {
    message: "Unable to generate Monogram",
    description: "Toast title when drawing the monogram fails",
  },
  "avatars.monogramFailedBody": {
    message: "There was a problem generating your monogram. Please try again.",
    description: "Toast body when drawing the monogram fails",
  },
  "avatars.cropTitle": {
    message: "Crop Avatar",
    description:
      "Title of the dialog that crops an uploaded image (Render's 'Crop Avatar')",
  },
  "avatars.cropHint": {
    message: "Drag image to move. Use slider to zoom.",
    description:
      "Accessible name of the crop dialog's zoom slider and its image area",
  },
  "avatars.zoomOut": {
    message: "Zoom out",
    description: "Crop dialog button that zooms out",
  },
  "avatars.zoomIn": {
    message: "Zoom in",
    description: "Crop dialog button that zooms in",
  },
  "avatars.cancel": {
    message: "Cancel",
    description: "Dialog button that closes without saving",
  },
  "avatars.save": {
    message: "Save",
    description: "Dialog button that saves the avatar",
  },
  "avatars.monogramTitle": {
    message: "Edit Monogram",
    description:
      "Title of the dialog that sets a monogram (Render's 'Edit Monogram')",
  },
  "avatars.monogramText": {
    message: "Text",
    description:
      "Label of the monogram's one-character text field (Render's 'Text')",
  },
  "avatars.monogramColor": {
    message: "Color",
    description: "Label of the monogram's color choice (Render's 'Color')",
  },
  "avatars.colorBlue": {
    message: "Blue",
    description: "Monogram color name",
  },
  "avatars.colorGreen": {
    message: "Green",
    description: "Monogram color name",
  },
  "avatars.colorLimeGreen": {
    message: "Lime green",
    description: "Monogram color name",
  },
  "avatars.colorOrange": {
    message: "Orange",
    description: "Monogram color name",
  },
  "avatars.colorPink": {
    message: "Pink",
    description: "Monogram color name",
  },
  "avatars.colorPurple": {
    message: "Purple",
    description: "Monogram color name",
  },
  "avatars.colorRed": {
    message: "Red",
    description: "Monogram color name",
  },
  "avatars.colorYellow": {
    message: "Yellow",
    description: "Monogram color name",
  },
  "avatars.uploadedAlt": {
    message: "User-uploaded Avatar for {email}",
    description:
      "Alt text of a person's uploaded avatar (Render's 'User-uploaded Avatar for <email>')",
  },
  "avatars.workspaceUploadedAlt": {
    message: "User-uploaded Avatar",
    description:
      "Alt text of a workspace's uploaded avatar (Render's 'User-uploaded Avatar')",
  },
  "avatars.gravatarAlt": {
    message: "Gravatar for {email}",
    description:
      "Alt text of a person's Gravatar (Render's 'Gravatar for <email>')",
  },
  "avatars.workspaceNoPermission": {
    message: "Only workspace admins can set the workspace image.",
    description:
      "Tooltip on a workspace avatar's disabled Edit button for a member who is not an admin",
  },
};

export default enAvatars;
