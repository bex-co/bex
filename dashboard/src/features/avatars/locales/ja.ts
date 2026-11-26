import type { TranslationEntry } from "@/i18n";

// Account and workspace avatars (w1/m648, docs/render-artifacts/avatars.md).
const jaAvatars: Record<string, TranslationEntry> = {
  "avatars.rowLabel": {
    message: "アバター",
    description:
      "Label of the Avatar row in Account Settings and Workspace Settings (Render's 'Avatar')",
  },
  "avatars.edit": {
    message: "編集",
    description: "Button that opens the avatar's menu (Render's 'Edit')",
  },
  "avatars.uploadImage": {
    message: "画像をアップロード",
    description:
      "Avatar menu item that opens the file picker (Render's 'Upload image')",
  },
  "avatars.changeMonogram": {
    message: "モノグラムを変更",
    description:
      "Avatar menu item that opens the Edit Monogram dialog (Render's 'Change monogram')",
  },
  "avatars.resetToDefault": {
    message: "デフォルトに戻す",
    description:
      "Avatar menu item that removes the uploaded avatar (Render's 'Reset to default')",
  },
  "avatars.invalidFileTypeTitle": {
    message: "無効なファイル形式です",
    description: "Toast title when the chosen file is not PNG or JPEG",
  },
  "avatars.invalidFileTypeBody": {
    message: "JPG、JPEG、PNG のいずれかのファイルを選択してください。",
    description: "Toast body when the chosen file is not PNG or JPEG",
  },
  "avatars.fileTooLargeTitle": {
    message: "ファイルが大きすぎます",
    description: "Toast title when the chosen file is over 20 MB",
  },
  "avatars.fileTooLargeBody": {
    message: "20 MB 未満のファイルを選択してください。",
    description: "Toast body when the chosen file is over 20 MB",
  },
  "avatars.unreadableFileTitle": {
    message: "ファイルを読み込めません",
    description: "Toast title when the browser cannot read the chosen file",
  },
  "avatars.unreadableFileBody": {
    message:
      "ブラウザがファイルを読み込めませんでした。もう一度お試しください。",
    description: "Toast body when the browser cannot read the chosen file",
  },
  "avatars.uploadFailedTitle": {
    message: "ファイルをアップロードできません",
    description: "Toast title when uploading the avatar fails",
  },
  "avatars.deleteFailedTitle": {
    message: "ファイルを削除できません",
    description: "Toast title when resetting the avatar fails",
  },
  "avatars.connectionFailedBody": {
    message: "サーバーへの接続中に問題が発生しました。もう一度お試しください。",
    description: "Toast body when uploading or resetting the avatar fails",
  },
  "avatars.monogramFailedTitle": {
    message: "モノグラムを生成できません",
    description: "Toast title when drawing the monogram fails",
  },
  "avatars.monogramFailedBody": {
    message: "モノグラムの生成中に問題が発生しました。もう一度お試しください。",
    description: "Toast body when drawing the monogram fails",
  },
  "avatars.cropTitle": {
    message: "アバターを切り抜く",
    description:
      "Title of the dialog that crops an uploaded image (Render's 'Crop Avatar')",
  },
  "avatars.cropHint": {
    message: "画像をドラッグして移動し、スライダーで拡大・縮小します。",
    description:
      "Accessible name of the crop dialog's zoom slider and its image area",
  },
  "avatars.zoomOut": {
    message: "縮小",
    description: "Crop dialog button that zooms out",
  },
  "avatars.zoomIn": {
    message: "拡大",
    description: "Crop dialog button that zooms in",
  },
  "avatars.cancel": {
    message: "キャンセル",
    description: "Dialog button that closes without saving",
  },
  "avatars.save": {
    message: "保存",
    description: "Dialog button that saves the avatar",
  },
  "avatars.monogramTitle": {
    message: "モノグラムを編集",
    description:
      "Title of the dialog that sets a monogram (Render's 'Edit Monogram')",
  },
  "avatars.monogramText": {
    message: "テキスト",
    description:
      "Label of the monogram's one-character text field (Render's 'Text')",
  },
  "avatars.monogramColor": {
    message: "色",
    description: "Label of the monogram's color choice (Render's 'Color')",
  },
  "avatars.colorBlue": {
    message: "青",
    description: "Monogram color name",
  },
  "avatars.colorGreen": {
    message: "緑",
    description: "Monogram color name",
  },
  "avatars.colorLimeGreen": {
    message: "ライムグリーン",
    description: "Monogram color name",
  },
  "avatars.colorOrange": {
    message: "オレンジ",
    description: "Monogram color name",
  },
  "avatars.colorPink": {
    message: "ピンク",
    description: "Monogram color name",
  },
  "avatars.colorPurple": {
    message: "紫",
    description: "Monogram color name",
  },
  "avatars.colorRed": {
    message: "赤",
    description: "Monogram color name",
  },
  "avatars.colorYellow": {
    message: "黄",
    description: "Monogram color name",
  },
  "avatars.uploadedAlt": {
    message: "{email} がアップロードしたアバター",
    description:
      "Alt text of a person's uploaded avatar (Render's 'User-uploaded Avatar for <email>')",
  },
  "avatars.workspaceUploadedAlt": {
    message: "アップロードされたアバター",
    description:
      "Alt text of a workspace's uploaded avatar (Render's 'User-uploaded Avatar')",
  },
  "avatars.gravatarAlt": {
    message: "{email} の Gravatar",
    description:
      "Alt text of a person's Gravatar (Render's 'Gravatar for <email>')",
  },
  "avatars.workspaceNoPermission": {
    message:
      "ワークスペースの画像を設定できるのはワークスペース管理者だけです。",
    description:
      "Tooltip on a workspace avatar's disabled Edit button for a member who is not an admin",
  },
};

export default jaAvatars;
