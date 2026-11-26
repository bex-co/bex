import type { TranslationEntry } from "@/i18n";

// Account and workspace avatars (w1/m648, docs/render-artifacts/avatars.md).
const ptAvatars: Record<string, TranslationEntry> = {
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
    message: "Enviar imagem",
    description:
      "Avatar menu item that opens the file picker (Render's 'Upload image')",
  },
  "avatars.changeMonogram": {
    message: "Alterar monograma",
    description:
      "Avatar menu item that opens the Edit Monogram dialog (Render's 'Change monogram')",
  },
  "avatars.resetToDefault": {
    message: "Redefinir para o padrão",
    description:
      "Avatar menu item that removes the uploaded avatar (Render's 'Reset to default')",
  },
  "avatars.invalidFileTypeTitle": {
    message: "Tipo de arquivo inválido",
    description: "Toast title when the chosen file is not PNG or JPEG",
  },
  "avatars.invalidFileTypeBody": {
    message: "Selecione um arquivo JPG, JPEG ou PNG.",
    description: "Toast body when the chosen file is not PNG or JPEG",
  },
  "avatars.fileTooLargeTitle": {
    message: "Arquivo muito grande",
    description: "Toast title when the chosen file is over 20 MB",
  },
  "avatars.fileTooLargeBody": {
    message: "Selecione um arquivo com menos de 20 MB.",
    description: "Toast body when the chosen file is over 20 MB",
  },
  "avatars.unreadableFileTitle": {
    message: "Não foi possível ler o arquivo",
    description: "Toast title when the browser cannot read the chosen file",
  },
  "avatars.unreadableFileBody": {
    message: "O navegador não conseguiu ler seu arquivo. Tente novamente.",
    description: "Toast body when the browser cannot read the chosen file",
  },
  "avatars.uploadFailedTitle": {
    message: "Não foi possível enviar o arquivo",
    description: "Toast title when uploading the avatar fails",
  },
  "avatars.deleteFailedTitle": {
    message: "Não foi possível excluir o arquivo",
    description: "Toast title when resetting the avatar fails",
  },
  "avatars.connectionFailedBody": {
    message: "Houve um problema ao conectar ao servidor. Tente novamente.",
    description: "Toast body when uploading or resetting the avatar fails",
  },
  "avatars.monogramFailedTitle": {
    message: "Não foi possível gerar o monograma",
    description: "Toast title when drawing the monogram fails",
  },
  "avatars.monogramFailedBody": {
    message: "Houve um problema ao gerar seu monograma. Tente novamente.",
    description: "Toast body when drawing the monogram fails",
  },
  "avatars.cropTitle": {
    message: "Recortar avatar",
    description:
      "Title of the dialog that crops an uploaded image (Render's 'Crop Avatar')",
  },
  "avatars.cropHint": {
    message:
      "Arraste a imagem para movê-la. Use o controle deslizante para ampliar.",
    description:
      "Accessible name of the crop dialog's zoom slider and its image area",
  },
  "avatars.zoomOut": {
    message: "Diminuir zoom",
    description: "Crop dialog button that zooms out",
  },
  "avatars.zoomIn": {
    message: "Aumentar zoom",
    description: "Crop dialog button that zooms in",
  },
  "avatars.cancel": {
    message: "Cancelar",
    description: "Dialog button that closes without saving",
  },
  "avatars.save": {
    message: "Salvar",
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
    message: "Cor",
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
    message: "Verde-limão",
    description: "Monogram color name",
  },
  "avatars.colorOrange": {
    message: "Laranja",
    description: "Monogram color name",
  },
  "avatars.colorPink": {
    message: "Rosa",
    description: "Monogram color name",
  },
  "avatars.colorPurple": {
    message: "Roxo",
    description: "Monogram color name",
  },
  "avatars.colorRed": {
    message: "Vermelho",
    description: "Monogram color name",
  },
  "avatars.colorYellow": {
    message: "Amarelo",
    description: "Monogram color name",
  },
  "avatars.uploadedAlt": {
    message: "Avatar enviado por {email}",
    description:
      "Alt text of a person's uploaded avatar (Render's 'User-uploaded Avatar for <email>')",
  },
  "avatars.workspaceUploadedAlt": {
    message: "Avatar enviado",
    description:
      "Alt text of a workspace's uploaded avatar (Render's 'User-uploaded Avatar')",
  },
  "avatars.gravatarAlt": {
    message: "Gravatar de {email}",
    description:
      "Alt text of a person's Gravatar (Render's 'Gravatar for <email>')",
  },
  "avatars.workspaceNoPermission": {
    message:
      "Somente administradores do workspace podem definir a imagem do workspace.",
    description:
      "Tooltip on a workspace avatar's disabled Edit button for a member who is not an admin",
  },
};

export default ptAvatars;
