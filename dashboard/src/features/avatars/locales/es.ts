import type { TranslationEntry } from "@/i18n";

// Account and workspace avatars (w1/m648, docs/render-artifacts/avatars.md).
const esAvatars: Record<string, TranslationEntry> = {
  "avatars.rowLabel": {
    message: "Avatar",
    description:
      "Label of the Avatar row in Account Settings and Workspace Settings (Render's 'Avatar')",
  },
  "avatars.edit": {
    message: "Editar",
    description: "Button that opens the avatar's menu (Render's 'Edit')",
  },
  "avatars.uploadImage": {
    message: "Subir imagen",
    description:
      "Avatar menu item that opens the file picker (Render's 'Upload image')",
  },
  "avatars.changeMonogram": {
    message: "Cambiar monograma",
    description:
      "Avatar menu item that opens the Edit Monogram dialog (Render's 'Change monogram')",
  },
  "avatars.resetToDefault": {
    message: "Restablecer predeterminado",
    description:
      "Avatar menu item that removes the uploaded avatar (Render's 'Reset to default')",
  },
  "avatars.invalidFileTypeTitle": {
    message: "Tipo de archivo no válido",
    description: "Toast title when the chosen file is not PNG or JPEG",
  },
  "avatars.invalidFileTypeBody": {
    message: "Selecciona un archivo JPG, JPEG o PNG.",
    description: "Toast body when the chosen file is not PNG or JPEG",
  },
  "avatars.fileTooLargeTitle": {
    message: "Archivo demasiado grande",
    description: "Toast title when the chosen file is over 20 MB",
  },
  "avatars.fileTooLargeBody": {
    message: "Selecciona un archivo de menos de 20 MB.",
    description: "Toast body when the chosen file is over 20 MB",
  },
  "avatars.unreadableFileTitle": {
    message: "No se puede leer el archivo",
    description: "Toast title when the browser cannot read the chosen file",
  },
  "avatars.unreadableFileBody": {
    message: "El navegador no pudo leer tu archivo. Inténtalo de nuevo.",
    description: "Toast body when the browser cannot read the chosen file",
  },
  "avatars.uploadFailedTitle": {
    message: "No se puede subir el archivo",
    description: "Toast title when uploading the avatar fails",
  },
  "avatars.deleteFailedTitle": {
    message: "No se puede eliminar el archivo",
    description: "Toast title when resetting the avatar fails",
  },
  "avatars.connectionFailedBody": {
    message:
      "Hubo un problema al conectar con el servidor. Inténtalo de nuevo.",
    description: "Toast body when uploading or resetting the avatar fails",
  },
  "avatars.monogramFailedTitle": {
    message: "No se puede generar el monograma",
    description: "Toast title when drawing the monogram fails",
  },
  "avatars.monogramFailedBody": {
    message: "Hubo un problema al generar tu monograma. Inténtalo de nuevo.",
    description: "Toast body when drawing the monogram fails",
  },
  "avatars.cropTitle": {
    message: "Recortar avatar",
    description:
      "Title of the dialog that crops an uploaded image (Render's 'Crop Avatar')",
  },
  "avatars.cropHint": {
    message:
      "Arrastra la imagen para moverla. Usa el control deslizante para hacer zoom.",
    description:
      "Accessible name of the crop dialog's zoom slider and its image area",
  },
  "avatars.zoomOut": {
    message: "Alejar",
    description: "Crop dialog button that zooms out",
  },
  "avatars.zoomIn": {
    message: "Acercar",
    description: "Crop dialog button that zooms in",
  },
  "avatars.cancel": {
    message: "Cancelar",
    description: "Dialog button that closes without saving",
  },
  "avatars.save": {
    message: "Guardar",
    description: "Dialog button that saves the avatar",
  },
  "avatars.monogramTitle": {
    message: "Editar monograma",
    description:
      "Title of the dialog that sets a monogram (Render's 'Edit Monogram')",
  },
  "avatars.monogramText": {
    message: "Texto",
    description:
      "Label of the monogram's one-character text field (Render's 'Text')",
  },
  "avatars.monogramColor": {
    message: "Color",
    description: "Label of the monogram's color choice (Render's 'Color')",
  },
  "avatars.colorBlue": {
    message: "Azul",
    description: "Monogram color name",
  },
  "avatars.colorGreen": {
    message: "Verde",
    description: "Monogram color name",
  },
  "avatars.colorLimeGreen": {
    message: "Verde lima",
    description: "Monogram color name",
  },
  "avatars.colorOrange": {
    message: "Naranja",
    description: "Monogram color name",
  },
  "avatars.colorPink": {
    message: "Rosa",
    description: "Monogram color name",
  },
  "avatars.colorPurple": {
    message: "Morado",
    description: "Monogram color name",
  },
  "avatars.colorRed": {
    message: "Rojo",
    description: "Monogram color name",
  },
  "avatars.colorYellow": {
    message: "Amarillo",
    description: "Monogram color name",
  },
  "avatars.uploadedAlt": {
    message: "Avatar subido por {email}",
    description:
      "Alt text of a person's uploaded avatar (Render's 'User-uploaded Avatar for <email>')",
  },
  "avatars.workspaceUploadedAlt": {
    message: "Avatar subido",
    description:
      "Alt text of a workspace's uploaded avatar (Render's 'User-uploaded Avatar')",
  },
  "avatars.gravatarAlt": {
    message: "Gravatar de {email}",
    description:
      "Alt text of a person's Gravatar (Render's 'Gravatar for <email>')",
  },
  "avatars.workspaceNoPermission": {
    message: "Solo los administradores del workspace pueden cambiar su imagen.",
    description:
      "Tooltip on a workspace avatar's disabled Edit button for a member who is not an admin",
  },
};

export default esAvatars;
