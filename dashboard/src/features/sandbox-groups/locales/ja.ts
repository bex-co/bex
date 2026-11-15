import type { TranslationEntry } from "@/i18n";

const jaSandboxGroups: Record<string, TranslationEntry> = {
  "sandboxGroups.createTitle": {
    message: "新しいサンドボックスグループ",
    description:
      "Title of the new-sandbox-group dialog and its entry in the + New menus",
  },
  "sandboxGroups.createDescription": {
    message:
      "サンドボックスグループは、グループ内で同時にアクティブにできるサンドボックスの数に上限を設けます。グループ名はワークスペース内で一意である必要があります。",
    description: "New-sandbox-group dialog description",
  },
  "sandboxGroups.fieldName": {
    message: "名前",
    description: "New-sandbox-group dialog name field label",
  },
  "sandboxGroups.fieldNamePlaceholder": {
    message: "my-sandbox-group",
    description: "Placeholder for the new sandbox group's name",
  },
  "sandboxGroups.fieldRegion": {
    message: "リージョン",
    description:
      "Label of the region row in the created sandbox group's confirmation",
  },
  "sandboxGroups.createCancel": {
    message: "キャンセル",
    description: "Cancel button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createSubmit": {
    message: "サンドボックスグループを作成",
    description: "Submit button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createError": {
    message:
      "サンドボックスグループ {name} を作成できませんでした。もう一度お試しください。",
    description: "Toast shown when creating a sandbox group fails",
  },
  "sandboxGroups.errorNameTaken": {
    message:
      "「{name}」という名前のサンドボックスグループはこのワークスペースにすでに存在します。別の名前を指定してください。",
    description:
      "Inline error under the name field when the group name is already taken",
  },
  "sandboxGroups.errorNameInvalid": {
    message:
      "グループ名は 1 〜 {max} 文字で、先頭と末尾に空白を含めず、制御文字も使用できません。",
    description: "Inline error under the name field when the name is refused",
  },
  "sandboxGroups.errorLimitReached_other": {
    message:
      "このワークスペースはサンドボックスグループの上限である {count} 個にすでに達しています。",
    description:
      "Inline alert when the workspace is at its sandbox group limit",
  },
  "sandboxGroups.errorLimitReachedGeneric": {
    message: "このワークスペースはサンドボックスグループの上限に達しています。",
    description:
      "Inline alert for the sandbox group limit when the server names no number",
  },
  "sandboxGroups.errorLimitTitle": {
    message: "サンドボックスグループの上限に達しました",
    description: "Heading of the inline sandbox-group-limit alert",
  },
  "sandboxGroups.createdTitle": {
    message: "サンドボックスグループを作成しました",
    description: "Dialog title once a sandbox group has been created",
  },
  "sandboxGroups.createdDescription": {
    message:
      "{name} の準備ができました。このグループにサンドボックスを作成するときは、その ID を sandboxGroupId として渡してください。",
    description: "Dialog description once a sandbox group has been created",
  },
  "sandboxGroups.createdDone": {
    message: "完了",
    description: "Button that closes the created-sandbox-group confirmation",
  },
  "sandboxGroups.fieldId": {
    message: "ID",
    description: "Label of the new sandbox group's ID in the confirmation",
  },
  "sandboxGroups.fieldDefault": {
    message: "デフォルトグループ",
    description:
      "Label of the confirmation row that says whether the group is the workspace's default",
  },
  "sandboxGroups.defaultYes": {
    message: "はい",
    description: "The created group is the workspace's default sandbox group",
  },
  "sandboxGroups.defaultNo": {
    message: "いいえ",
    description:
      "The created group is not the workspace's default sandbox group",
  },
  "sandboxGroups.fieldConcurrencyLimit": {
    message: "同時実行数の上限",
    description:
      "Label of the confirmation row with the most active sandboxes the group allows",
  },
  "sandboxGroups.copyId": {
    message: "サンドボックスグループ ID をコピー",
    description: "Accessible label of the copy button next to the group's ID",
  },
  "sandboxGroups.copied": {
    message: "サンドボックスグループ ID をコピーしました",
    description: "Toast after copying the sandbox group ID",
  },
  "sandboxGroups.copyError": {
    message: "サンドボックスグループ ID をコピーできませんでした",
    description: "Toast when copying the sandbox group ID fails",
  },
  "sandboxGroups.environmentLoading": {
    message: "環境を読み込み中",
    description:
      "Accessible name of the loading placeholder shown where the new-sandbox-group dialog's Environment select will appear",
  },
  "sandboxGroups.environmentsError": {
    message:
      "環境を読み込めませんでした。環境を指定せずにグループを作成することはできます。",
    description:
      "Shown in the new-sandbox-group dialog when the workspace's environments could not be read; the group can still be created without one",
  },
  "sandboxGroups.errorEnvironmentGone": {
    message:
      "その環境はもう存在しません。別の環境を選ぶか、環境なしを選んでください。",
    description:
      "Inline error under the Environment select when the chosen environment was deleted before the group was created",
  },
};

export default jaSandboxGroups;
