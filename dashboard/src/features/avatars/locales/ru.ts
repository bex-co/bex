import type { TranslationEntry } from "@/i18n";

// Account and workspace avatars (w1/m648, docs/render-artifacts/avatars.md).
const ruAvatars: Record<string, TranslationEntry> = {
  "avatars.rowLabel": {
    message: "Аватар",
    description:
      "Label of the Avatar row in Account Settings and Workspace Settings (Render's 'Avatar')",
  },
  "avatars.edit": {
    message: "Изменить",
    description: "Button that opens the avatar's menu (Render's 'Edit')",
  },
  "avatars.uploadImage": {
    message: "Загрузить изображение",
    description:
      "Avatar menu item that opens the file picker (Render's 'Upload image')",
  },
  "avatars.changeMonogram": {
    message: "Изменить монограмму",
    description:
      "Avatar menu item that opens the Edit Monogram dialog (Render's 'Change monogram')",
  },
  "avatars.resetToDefault": {
    message: "Сбросить по умолчанию",
    description:
      "Avatar menu item that removes the uploaded avatar (Render's 'Reset to default')",
  },
  "avatars.invalidFileTypeTitle": {
    message: "Недопустимый тип файла",
    description: "Toast title when the chosen file is not PNG or JPEG",
  },
  "avatars.invalidFileTypeBody": {
    message: "Выберите файл JPG, JPEG или PNG.",
    description: "Toast body when the chosen file is not PNG or JPEG",
  },
  "avatars.fileTooLargeTitle": {
    message: "Файл слишком большой",
    description: "Toast title when the chosen file is over 20 MB",
  },
  "avatars.fileTooLargeBody": {
    message: "Выберите файл размером меньше 20 МБ.",
    description: "Toast body when the chosen file is over 20 MB",
  },
  "avatars.unreadableFileTitle": {
    message: "Не удалось прочитать файл",
    description: "Toast title when the browser cannot read the chosen file",
  },
  "avatars.unreadableFileBody": {
    message: "Браузер не смог прочитать ваш файл. Попробуйте ещё раз.",
    description: "Toast body when the browser cannot read the chosen file",
  },
  "avatars.uploadFailedTitle": {
    message: "Не удалось загрузить файл",
    description: "Toast title when uploading the avatar fails",
  },
  "avatars.deleteFailedTitle": {
    message: "Не удалось удалить файл",
    description: "Toast title when resetting the avatar fails",
  },
  "avatars.connectionFailedBody": {
    message: "Не удалось подключиться к серверу. Попробуйте ещё раз.",
    description: "Toast body when uploading or resetting the avatar fails",
  },
  "avatars.monogramFailedTitle": {
    message: "Не удалось создать монограмму",
    description: "Toast title when drawing the monogram fails",
  },
  "avatars.monogramFailedBody": {
    message: "При создании монограммы возникла проблема. Попробуйте ещё раз.",
    description: "Toast body when drawing the monogram fails",
  },
  "avatars.cropTitle": {
    message: "Обрезать аватар",
    description:
      "Title of the dialog that crops an uploaded image (Render's 'Crop Avatar')",
  },
  "avatars.cropHint": {
    message:
      "Перетащите изображение, чтобы сдвинуть его. Масштаб меняется ползунком.",
    description:
      "Accessible name of the crop dialog's zoom slider and its image area",
  },
  "avatars.zoomOut": {
    message: "Уменьшить",
    description: "Crop dialog button that zooms out",
  },
  "avatars.zoomIn": {
    message: "Увеличить",
    description: "Crop dialog button that zooms in",
  },
  "avatars.cancel": {
    message: "Отмена",
    description: "Dialog button that closes without saving",
  },
  "avatars.save": {
    message: "Сохранить",
    description: "Dialog button that saves the avatar",
  },
  "avatars.monogramTitle": {
    message: "Изменить монограмму",
    description:
      "Title of the dialog that sets a monogram (Render's 'Edit Monogram')",
  },
  "avatars.monogramText": {
    message: "Текст",
    description:
      "Label of the monogram's one-character text field (Render's 'Text')",
  },
  "avatars.monogramColor": {
    message: "Цвет",
    description: "Label of the monogram's color choice (Render's 'Color')",
  },
  "avatars.colorBlue": {
    message: "Синий",
    description: "Monogram color name",
  },
  "avatars.colorGreen": {
    message: "Зелёный",
    description: "Monogram color name",
  },
  "avatars.colorLimeGreen": {
    message: "Лаймовый",
    description: "Monogram color name",
  },
  "avatars.colorOrange": {
    message: "Оранжевый",
    description: "Monogram color name",
  },
  "avatars.colorPink": {
    message: "Розовый",
    description: "Monogram color name",
  },
  "avatars.colorPurple": {
    message: "Фиолетовый",
    description: "Monogram color name",
  },
  "avatars.colorRed": {
    message: "Красный",
    description: "Monogram color name",
  },
  "avatars.colorYellow": {
    message: "Жёлтый",
    description: "Monogram color name",
  },
  "avatars.uploadedAlt": {
    message: "Аватар, загруженный {email}",
    description:
      "Alt text of a person's uploaded avatar (Render's 'User-uploaded Avatar for <email>')",
  },
  "avatars.workspaceUploadedAlt": {
    message: "Загруженный аватар",
    description:
      "Alt text of a workspace's uploaded avatar (Render's 'User-uploaded Avatar')",
  },
  "avatars.gravatarAlt": {
    message: "Gravatar для {email}",
    description:
      "Alt text of a person's Gravatar (Render's 'Gravatar for <email>')",
  },
  "avatars.workspaceNoPermission": {
    message:
      "Изображение рабочего пространства могут менять только его администраторы.",
    description:
      "Tooltip on a workspace avatar's disabled Edit button for a member who is not an admin",
  },
};

export default ruAvatars;
