import type { TranslationEntry } from "@/i18n";

const koSandboxGroups: Record<string, TranslationEntry> = {
  "sandboxGroups.createTitle": {
    message: "새 샌드박스 그룹",
    description:
      "Title of the new-sandbox-group dialog and its entry in the + New menus",
  },
  "sandboxGroups.createDescription": {
    message:
      "샌드박스 그룹은 그룹에 속한 샌드박스를 동시에 몇 개까지 활성화할 수 있는지 제한합니다. 그룹 이름은 워크스페이스 내에서 고유해야 합니다.",
    description: "New-sandbox-group dialog description",
  },
  "sandboxGroups.fieldName": {
    message: "이름",
    description: "New-sandbox-group dialog name field label",
  },
  "sandboxGroups.fieldNamePlaceholder": {
    message: "my-sandbox-group",
    description: "Placeholder for the new sandbox group's name",
  },
  "sandboxGroups.fieldRegion": {
    message: "리전",
    description:
      "Label of the region row in the created sandbox group's confirmation",
  },
  "sandboxGroups.createCancel": {
    message: "취소",
    description: "Cancel button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createSubmit": {
    message: "샌드박스 그룹 만들기",
    description: "Submit button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createError": {
    message: "{name} 샌드박스 그룹을 만들지 못했습니다. 다시 시도하세요.",
    description: "Toast shown when creating a sandbox group fails",
  },
  "sandboxGroups.errorNameTaken": {
    message:
      '이 워크스페이스에 이름이 "{name}"인 샌드박스 그룹이 이미 있습니다. 다른 이름을 선택하세요.',
    description:
      "Inline error under the name field when the group name is already taken",
  },
  "sandboxGroups.errorNameInvalid": {
    message:
      "그룹 이름은 1자에서 {max}자 사이여야 하며, 앞뒤 공백과 제어 문자를 포함할 수 없습니다.",
    description: "Inline error under the name field when the name is refused",
  },
  "sandboxGroups.errorLimitReached_other": {
    message:
      "이 워크스페이스에는 이미 최대 {count}개의 샌드박스 그룹이 있습니다.",
    description:
      "Inline alert when the workspace is at its sandbox group limit",
  },
  "sandboxGroups.errorLimitReachedGeneric": {
    message: "이 워크스페이스가 샌드박스 그룹 한도에 도달했습니다.",
    description:
      "Inline alert for the sandbox group limit when the server names no number",
  },
  "sandboxGroups.errorLimitTitle": {
    message: "샌드박스 그룹 한도 도달",
    description: "Heading of the inline sandbox-group-limit alert",
  },
  "sandboxGroups.createdTitle": {
    message: "샌드박스 그룹을 만들었습니다",
    description: "Dialog title once a sandbox group has been created",
  },
  "sandboxGroups.createdDescription": {
    message:
      "{name} 그룹이 준비되었습니다. 이 그룹에 샌드박스를 만들 때 그룹의 ID를 sandboxGroupId로 전달하세요.",
    description: "Dialog description once a sandbox group has been created",
  },
  "sandboxGroups.createdDone": {
    message: "완료",
    description: "Button that closes the created-sandbox-group confirmation",
  },
  "sandboxGroups.fieldId": {
    message: "ID",
    description: "Label of the new sandbox group's ID in the confirmation",
  },
  "sandboxGroups.fieldDefault": {
    message: "기본 그룹",
    description:
      "Label of the confirmation row that says whether the group is the workspace's default",
  },
  "sandboxGroups.defaultYes": {
    message: "예",
    description: "The created group is the workspace's default sandbox group",
  },
  "sandboxGroups.defaultNo": {
    message: "아니요",
    description:
      "The created group is not the workspace's default sandbox group",
  },
  "sandboxGroups.fieldConcurrencyLimit": {
    message: "동시 실행 한도",
    description:
      "Label of the confirmation row with the most active sandboxes the group allows",
  },
  "sandboxGroups.copyId": {
    message: "샌드박스 그룹 ID 복사",
    description: "Accessible label of the copy button next to the group's ID",
  },
  "sandboxGroups.copied": {
    message: "샌드박스 그룹 ID를 복사했습니다",
    description: "Toast after copying the sandbox group ID",
  },
  "sandboxGroups.copyError": {
    message: "샌드박스 그룹 ID를 복사하지 못했습니다",
    description: "Toast when copying the sandbox group ID fails",
  },
};

export default koSandboxGroups;
