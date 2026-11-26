import type { TranslationEntry } from "@/i18n";

// Account and workspace avatars (w1/m648, docs/render-artifacts/avatars.md).
const deAvatars: Record<string, TranslationEntry> = {
  "avatars.rowLabel": {
    message: "Avatar",
    description:
      "Label of the Avatar row in Account Settings and Workspace Settings (Render's 'Avatar')",
  },
  "avatars.edit": {
    message: "Bearbeiten",
    description: "Button that opens the avatar's menu (Render's 'Edit')",
  },
  "avatars.uploadImage": {
    message: "Bild hochladen",
    description:
      "Avatar menu item that opens the file picker (Render's 'Upload image')",
  },
  "avatars.changeMonogram": {
    message: "Monogramm ändern",
    description:
      "Avatar menu item that opens the Edit Monogram dialog (Render's 'Change monogram')",
  },
  "avatars.resetToDefault": {
    message: "Auf Standard zurücksetzen",
    description:
      "Avatar menu item that removes the uploaded avatar (Render's 'Reset to default')",
  },
  "avatars.invalidFileTypeTitle": {
    message: "Ungültiger Dateityp",
    description: "Toast title when the chosen file is not PNG or JPEG",
  },
  "avatars.invalidFileTypeBody": {
    message: "Wähle eine JPG-, JPEG- oder PNG-Datei aus.",
    description: "Toast body when the chosen file is not PNG or JPEG",
  },
  "avatars.fileTooLargeTitle": {
    message: "Datei zu groß",
    description: "Toast title when the chosen file is over 20 MB",
  },
  "avatars.fileTooLargeBody": {
    message: "Wähle eine Datei unter 20 MB aus.",
    description: "Toast body when the chosen file is over 20 MB",
  },
  "avatars.unreadableFileTitle": {
    message: "Datei kann nicht gelesen werden",
    description: "Toast title when the browser cannot read the chosen file",
  },
  "avatars.unreadableFileBody": {
    message: "Der Browser konnte deine Datei nicht lesen. Versuche es erneut.",
    description: "Toast body when the browser cannot read the chosen file",
  },
  "avatars.uploadFailedTitle": {
    message: "Datei kann nicht hochgeladen werden",
    description: "Toast title when uploading the avatar fails",
  },
  "avatars.deleteFailedTitle": {
    message: "Datei kann nicht gelöscht werden",
    description: "Toast title when resetting the avatar fails",
  },
  "avatars.connectionFailedBody": {
    message:
      "Beim Verbinden mit dem Server ist ein Problem aufgetreten. Versuche es erneut.",
    description: "Toast body when uploading or resetting the avatar fails",
  },
  "avatars.monogramFailedTitle": {
    message: "Monogramm kann nicht erstellt werden",
    description: "Toast title when drawing the monogram fails",
  },
  "avatars.monogramFailedBody": {
    message:
      "Beim Erstellen deines Monogramms ist ein Problem aufgetreten. Versuche es erneut.",
    description: "Toast body when drawing the monogram fails",
  },
  "avatars.cropTitle": {
    message: "Avatar zuschneiden",
    description:
      "Title of the dialog that crops an uploaded image (Render's 'Crop Avatar')",
  },
  "avatars.cropHint": {
    message: "Zieh das Bild, um es zu verschieben. Mit dem Regler zoomst du.",
    description:
      "Accessible name of the crop dialog's zoom slider and its image area",
  },
  "avatars.zoomOut": {
    message: "Verkleinern",
    description: "Crop dialog button that zooms out",
  },
  "avatars.zoomIn": {
    message: "Vergrößern",
    description: "Crop dialog button that zooms in",
  },
  "avatars.cancel": {
    message: "Abbrechen",
    description: "Dialog button that closes without saving",
  },
  "avatars.save": {
    message: "Speichern",
    description: "Dialog button that saves the avatar",
  },
  "avatars.monogramTitle": {
    message: "Monogramm bearbeiten",
    description:
      "Title of the dialog that sets a monogram (Render's 'Edit Monogram')",
  },
  "avatars.monogramText": {
    message: "Text",
    description:
      "Label of the monogram's one-character text field (Render's 'Text')",
  },
  "avatars.monogramColor": {
    message: "Farbe",
    description: "Label of the monogram's color choice (Render's 'Color')",
  },
  "avatars.colorBlue": {
    message: "Blau",
    description: "Monogram color name",
  },
  "avatars.colorGreen": {
    message: "Grün",
    description: "Monogram color name",
  },
  "avatars.colorLimeGreen": {
    message: "Limettengrün",
    description: "Monogram color name",
  },
  "avatars.colorOrange": {
    message: "Orange",
    description: "Monogram color name",
  },
  "avatars.colorPink": {
    message: "Rosa",
    description: "Monogram color name",
  },
  "avatars.colorPurple": {
    message: "Lila",
    description: "Monogram color name",
  },
  "avatars.colorRed": {
    message: "Rot",
    description: "Monogram color name",
  },
  "avatars.colorYellow": {
    message: "Gelb",
    description: "Monogram color name",
  },
  "avatars.uploadedAlt": {
    message: "Hochgeladener Avatar von {email}",
    description:
      "Alt text of a person's uploaded avatar (Render's 'User-uploaded Avatar for <email>')",
  },
  "avatars.workspaceUploadedAlt": {
    message: "Hochgeladener Avatar",
    description:
      "Alt text of a workspace's uploaded avatar (Render's 'User-uploaded Avatar')",
  },
  "avatars.gravatarAlt": {
    message: "Gravatar von {email}",
    description:
      "Alt text of a person's Gravatar (Render's 'Gravatar for <email>')",
  },
  "avatars.workspaceNoPermission": {
    message: "Nur Workspace-Admins können das Workspace-Bild festlegen.",
    description:
      "Tooltip on a workspace avatar's disabled Edit button for a member who is not an admin",
  },
};

export default deAvatars;
