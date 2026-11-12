import type { TranslationEntry } from "@/i18n";

const zhSandboxGroups: Record<string, TranslationEntry> = {
  "sandboxGroups.createTitle": {
    message: "新建沙箱组",
    description:
      "Title of the new-sandbox-group dialog and its entry in the + New menus",
  },
  "sandboxGroups.createDescription": {
    message:
      "沙箱组限制其中同时处于活动状态的沙箱数量。沙箱组名称在你的工作区内必须唯一。",
    description: "New-sandbox-group dialog description",
  },
  "sandboxGroups.fieldName": {
    message: "名称",
    description: "New-sandbox-group dialog name field label",
  },
  "sandboxGroups.fieldNamePlaceholder": {
    message: "my-sandbox-group",
    description: "Placeholder for the new sandbox group's name",
  },
  "sandboxGroups.fieldRegion": {
    message: "区域",
    description:
      "Label of the fixed region row in the new-sandbox-group dialog",
  },
  "sandboxGroups.regionOregon": {
    message: "俄勒冈（美国西部）",
    description: "The one region a sandbox group runs in, shown read-only",
  },
  "sandboxGroups.regionHelp": {
    message: "沙箱组仅在俄勒冈区域运行。",
    description: "Help text under the fixed region row",
  },
  "sandboxGroups.createCancel": {
    message: "取消",
    description: "Cancel button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createSubmit": {
    message: "创建沙箱组",
    description: "Submit button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createError": {
    message: "无法创建沙箱组 {name}，请重试。",
    description: "Toast shown when creating a sandbox group fails",
  },
  "sandboxGroups.errorNameTaken": {
    message: "此工作区中已存在名为“{name}”的沙箱组，请换一个名称。",
    description:
      "Inline error under the name field when the group name is already taken",
  },
  "sandboxGroups.errorNameInvalid": {
    message:
      "沙箱组名称必须为 1 到 {max} 个字符，且不能以空格开头或结尾，也不能包含控制字符。",
    description: "Inline error under the name field when the name is refused",
  },
  "sandboxGroups.errorLimitReached_other": {
    message: "此工作区的沙箱组数量已达上限（{count} 个）。",
    description:
      "Inline alert when the workspace is at its sandbox group limit",
  },
  "sandboxGroups.errorLimitReachedGeneric": {
    message: "此工作区的沙箱组数量已达上限。",
    description:
      "Inline alert for the sandbox group limit when the server names no number",
  },
  "sandboxGroups.errorLimitTitle": {
    message: "已达沙箱组上限",
    description: "Heading of the inline sandbox-group-limit alert",
  },
  "sandboxGroups.createdTitle": {
    message: "沙箱组已创建",
    description: "Dialog title once a sandbox group has been created",
  },
  "sandboxGroups.createdDescription": {
    message:
      "{name} 已就绪。在其中创建沙箱时，请将其 ID 作为 sandboxGroupId 传入。",
    description: "Dialog description once a sandbox group has been created",
  },
  "sandboxGroups.createdDone": {
    message: "完成",
    description: "Button that closes the created-sandbox-group confirmation",
  },
  "sandboxGroups.fieldId": {
    message: "ID",
    description: "Label of the new sandbox group's ID in the confirmation",
  },
  "sandboxGroups.fieldDefault": {
    message: "默认组",
    description:
      "Label of the confirmation row that says whether the group is the workspace's default",
  },
  "sandboxGroups.defaultYes": {
    message: "是",
    description: "The created group is the workspace's default sandbox group",
  },
  "sandboxGroups.defaultNo": {
    message: "否",
    description:
      "The created group is not the workspace's default sandbox group",
  },
  "sandboxGroups.fieldConcurrencyLimit": {
    message: "并发上限",
    description:
      "Label of the confirmation row with the most active sandboxes the group allows",
  },
  "sandboxGroups.copyId": {
    message: "复制沙箱组 ID",
    description: "Accessible label of the copy button next to the group's ID",
  },
  "sandboxGroups.copied": {
    message: "已复制沙箱组 ID",
    description: "Toast after copying the sandbox group ID",
  },
  "sandboxGroups.copyError": {
    message: "无法复制沙箱组 ID",
    description: "Toast when copying the sandbox group ID fails",
  },
};

export default zhSandboxGroups;
