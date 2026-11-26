import type { TranslationEntry } from "@/i18n";

// Account and workspace avatars (w1/m648, docs/render-artifacts/avatars.md).
const koAvatars: Record<string, TranslationEntry> = {
  "avatars.rowLabel": {
    message: "아바타",
    description:
      "Label of the Avatar row in Account Settings and Workspace Settings (Render's 'Avatar')",
  },
  "avatars.edit": {
    message: "편집",
    description: "Button that opens the avatar's menu (Render's 'Edit')",
  },
  "avatars.uploadImage": {
    message: "이미지 업로드",
    description:
      "Avatar menu item that opens the file picker (Render's 'Upload image')",
  },
  "avatars.changeMonogram": {
    message: "모노그램 변경",
    description:
      "Avatar menu item that opens the Edit Monogram dialog (Render's 'Change monogram')",
  },
  "avatars.resetToDefault": {
    message: "기본값으로 재설정",
    description:
      "Avatar menu item that removes the uploaded avatar (Render's 'Reset to default')",
  },
  "avatars.invalidFileTypeTitle": {
    message: "잘못된 파일 형식",
    description: "Toast title when the chosen file is not PNG or JPEG",
  },
  "avatars.invalidFileTypeBody": {
    message: "JPG, JPEG 또는 PNG 파일을 선택하세요.",
    description: "Toast body when the chosen file is not PNG or JPEG",
  },
  "avatars.fileTooLargeTitle": {
    message: "파일이 너무 큽니다",
    description: "Toast title when the chosen file is over 20 MB",
  },
  "avatars.fileTooLargeBody": {
    message: "20MB 미만의 파일을 선택하세요.",
    description: "Toast body when the chosen file is over 20 MB",
  },
  "avatars.unreadableFileTitle": {
    message: "파일을 읽을 수 없습니다",
    description: "Toast title when the browser cannot read the chosen file",
  },
  "avatars.unreadableFileBody": {
    message: "브라우저가 파일을 읽을 수 없습니다. 다시 시도하세요.",
    description: "Toast body when the browser cannot read the chosen file",
  },
  "avatars.uploadFailedTitle": {
    message: "파일을 업로드할 수 없습니다",
    description: "Toast title when uploading the avatar fails",
  },
  "avatars.deleteFailedTitle": {
    message: "파일을 삭제할 수 없습니다",
    description: "Toast title when resetting the avatar fails",
  },
  "avatars.connectionFailedBody": {
    message: "서버에 연결하는 중 문제가 발생했습니다. 다시 시도하세요.",
    description: "Toast body when uploading or resetting the avatar fails",
  },
  "avatars.monogramFailedTitle": {
    message: "모노그램을 생성할 수 없습니다",
    description: "Toast title when drawing the monogram fails",
  },
  "avatars.monogramFailedBody": {
    message: "모노그램을 생성하는 중 문제가 발생했습니다. 다시 시도하세요.",
    description: "Toast body when drawing the monogram fails",
  },
  "avatars.cropTitle": {
    message: "아바타 자르기",
    description:
      "Title of the dialog that crops an uploaded image (Render's 'Crop Avatar')",
  },
  "avatars.cropHint": {
    message: "이미지를 드래그하여 이동하고 슬라이더로 확대/축소하세요.",
    description:
      "Accessible name of the crop dialog's zoom slider and its image area",
  },
  "avatars.zoomOut": {
    message: "축소",
    description: "Crop dialog button that zooms out",
  },
  "avatars.zoomIn": {
    message: "확대",
    description: "Crop dialog button that zooms in",
  },
  "avatars.cancel": {
    message: "취소",
    description: "Dialog button that closes without saving",
  },
  "avatars.save": {
    message: "저장",
    description: "Dialog button that saves the avatar",
  },
  "avatars.monogramTitle": {
    message: "모노그램 편집",
    description:
      "Title of the dialog that sets a monogram (Render's 'Edit Monogram')",
  },
  "avatars.monogramText": {
    message: "텍스트",
    description:
      "Label of the monogram's one-character text field (Render's 'Text')",
  },
  "avatars.monogramColor": {
    message: "색상",
    description: "Label of the monogram's color choice (Render's 'Color')",
  },
  "avatars.colorBlue": {
    message: "파란색",
    description: "Monogram color name",
  },
  "avatars.colorGreen": {
    message: "초록색",
    description: "Monogram color name",
  },
  "avatars.colorLimeGreen": {
    message: "라임 그린",
    description: "Monogram color name",
  },
  "avatars.colorOrange": {
    message: "주황색",
    description: "Monogram color name",
  },
  "avatars.colorPink": {
    message: "분홍색",
    description: "Monogram color name",
  },
  "avatars.colorPurple": {
    message: "보라색",
    description: "Monogram color name",
  },
  "avatars.colorRed": {
    message: "빨간색",
    description: "Monogram color name",
  },
  "avatars.colorYellow": {
    message: "노란색",
    description: "Monogram color name",
  },
  "avatars.uploadedAlt": {
    message: "{email}님이 업로드한 아바타",
    description:
      "Alt text of a person's uploaded avatar (Render's 'User-uploaded Avatar for <email>')",
  },
  "avatars.workspaceUploadedAlt": {
    message: "업로드한 아바타",
    description:
      "Alt text of a workspace's uploaded avatar (Render's 'User-uploaded Avatar')",
  },
  "avatars.gravatarAlt": {
    message: "{email}의 Gravatar",
    description:
      "Alt text of a person's Gravatar (Render's 'Gravatar for <email>')",
  },
  "avatars.workspaceNoPermission": {
    message: "워크스페이스 관리자만 워크스페이스 이미지를 설정할 수 있습니다.",
    description:
      "Tooltip on a workspace avatar's disabled Edit button for a member who is not an admin",
  },
};

export default koAvatars;
