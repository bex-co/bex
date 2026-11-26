import type { TranslationEntry } from "@/i18n";

// Account and workspace avatars (w1/m648, docs/render-artifacts/avatars.md).
const zhAvatars: Record<string, TranslationEntry> = {
  "avatars.rowLabel": {
    message: "头像",
    description:
      "Label of the Avatar row in Account Settings and Workspace Settings (Render's 'Avatar')",
  },
  "avatars.edit": {
    message: "编辑",
    description: "Button that opens the avatar's menu (Render's 'Edit')",
  },
  "avatars.uploadImage": {
    message: "上传图片",
    description:
      "Avatar menu item that opens the file picker (Render's 'Upload image')",
  },
  "avatars.changeMonogram": {
    message: "更改字母头像",
    description:
      "Avatar menu item that opens the Edit Monogram dialog (Render's 'Change monogram')",
  },
  "avatars.resetToDefault": {
    message: "恢复默认",
    description:
      "Avatar menu item that removes the uploaded avatar (Render's 'Reset to default')",
  },
  "avatars.invalidFileTypeTitle": {
    message: "文件类型无效",
    description: "Toast title when the chosen file is not PNG or JPEG",
  },
  "avatars.invalidFileTypeBody": {
    message: "请选择 JPG、JPEG 或 PNG 文件。",
    description: "Toast body when the chosen file is not PNG or JPEG",
  },
  "avatars.fileTooLargeTitle": {
    message: "文件过大",
    description: "Toast title when the chosen file is over 20 MB",
  },
  "avatars.fileTooLargeBody": {
    message: "请选择小于 20 MB 的文件。",
    description: "Toast body when the chosen file is over 20 MB",
  },
  "avatars.unreadableFileTitle": {
    message: "无法读取文件",
    description: "Toast title when the browser cannot read the chosen file",
  },
  "avatars.unreadableFileBody": {
    message: "浏览器无法读取你的文件，请重试。",
    description: "Toast body when the browser cannot read the chosen file",
  },
  "avatars.uploadFailedTitle": {
    message: "无法上传文件",
    description: "Toast title when uploading the avatar fails",
  },
  "avatars.deleteFailedTitle": {
    message: "无法删除文件",
    description: "Toast title when resetting the avatar fails",
  },
  "avatars.connectionFailedBody": {
    message: "连接服务器时出现问题，请重试。",
    description: "Toast body when uploading or resetting the avatar fails",
  },
  "avatars.monogramFailedTitle": {
    message: "无法生成字母头像",
    description: "Toast title when drawing the monogram fails",
  },
  "avatars.monogramFailedBody": {
    message: "生成字母头像时出现问题，请重试。",
    description: "Toast body when drawing the monogram fails",
  },
  "avatars.cropTitle": {
    message: "裁剪头像",
    description:
      "Title of the dialog that crops an uploaded image (Render's 'Crop Avatar')",
  },
  "avatars.cropHint": {
    message: "拖动图片以移动，使用滑块缩放。",
    description:
      "Accessible name of the crop dialog's zoom slider and its image area",
  },
  "avatars.zoomOut": {
    message: "缩小",
    description: "Crop dialog button that zooms out",
  },
  "avatars.zoomIn": {
    message: "放大",
    description: "Crop dialog button that zooms in",
  },
  "avatars.cancel": {
    message: "取消",
    description: "Dialog button that closes without saving",
  },
  "avatars.save": {
    message: "保存",
    description: "Dialog button that saves the avatar",
  },
  "avatars.monogramTitle": {
    message: "编辑字母头像",
    description:
      "Title of the dialog that sets a monogram (Render's 'Edit Monogram')",
  },
  "avatars.monogramText": {
    message: "文字",
    description:
      "Label of the monogram's one-character text field (Render's 'Text')",
  },
  "avatars.monogramColor": {
    message: "颜色",
    description: "Label of the monogram's color choice (Render's 'Color')",
  },
  "avatars.colorBlue": {
    message: "蓝色",
    description: "Monogram color name",
  },
  "avatars.colorGreen": {
    message: "绿色",
    description: "Monogram color name",
  },
  "avatars.colorLimeGreen": {
    message: "青柠绿",
    description: "Monogram color name",
  },
  "avatars.colorOrange": {
    message: "橙色",
    description: "Monogram color name",
  },
  "avatars.colorPink": {
    message: "粉色",
    description: "Monogram color name",
  },
  "avatars.colorPurple": {
    message: "紫色",
    description: "Monogram color name",
  },
  "avatars.colorRed": {
    message: "红色",
    description: "Monogram color name",
  },
  "avatars.colorYellow": {
    message: "黄色",
    description: "Monogram color name",
  },
  "avatars.uploadedAlt": {
    message: "{email} 上传的头像",
    description:
      "Alt text of a person's uploaded avatar (Render's 'User-uploaded Avatar for <email>')",
  },
  "avatars.workspaceUploadedAlt": {
    message: "上传的头像",
    description:
      "Alt text of a workspace's uploaded avatar (Render's 'User-uploaded Avatar')",
  },
  "avatars.gravatarAlt": {
    message: "{email} 的 Gravatar",
    description:
      "Alt text of a person's Gravatar (Render's 'Gravatar for <email>')",
  },
  "avatars.workspaceNoPermission": {
    message: "只有工作区管理员可以设置工作区图片。",
    description:
      "Tooltip on a workspace avatar's disabled Edit button for a member who is not an admin",
  },
};

export default zhAvatars;
