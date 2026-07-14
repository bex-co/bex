import type { TranslationEntry } from "@/i18n";

const zhEnvGroups: Record<string, TranslationEntry> = {
  "envGroups.pageTitle": {
    message: "环境变量组",
    description: "Workspace env-groups page title",
  },
  "envGroups.pageDescription": {
    message: "在多个服务之间共享环境变量和密钥文件。",
    description: "Workspace env-groups page subtitle",
  },
  "envGroups.newButton": {
    message: "新建环境变量组",
    description: "Open the create env-group dialog",
  },
  "envGroups.createTitle": {
    message: "创建环境变量组",
    description: "Create env-group dialog title",
  },
  "envGroups.createDescription": {
    message: "先创建组，然后添加变量、文件和关联服务。",
    description: "Create env-group dialog description",
  },
  "envGroups.nameLabel": {
    message: "组名称",
    description: "Env-group name field label",
  },
  "envGroups.namePlaceholder": {
    message: "shared-production",
    description: "Env-group name placeholder",
  },
  "envGroups.invalidName": {
    message: "请输入组名称。",
    description: "Invalid env-group name message",
  },
  "envGroups.cancel": {
    message: "取消",
    description: "Cancel an env-group dialog",
  },
  "envGroups.createSubmit": {
    message: "创建环境变量组",
    description: "Create env-group submit button",
  },
  "envGroups.emptyTitle": {
    message: "暂无环境变量组",
    description: "Workspace env-groups empty-state title",
  },
  "envGroups.emptyBody": {
    message: "创建一个组，即使尚未关联服务，也可以先管理共享配置。",
    description: "Workspace env-groups empty-state body",
  },
  "envGroups.varCount": {
    message: "{count} 个变量",
    description: "Env-group variable count",
  },
  "envGroups.fileCount": {
    message: "{count} 个密钥文件",
    description: "Env-group secret-file count",
  },
  "envGroups.serviceCount": {
    message: "{count} 个关联服务",
    description: "Env-group linked-service count",
  },
  "envGroups.unavailableTitle": {
    message: "环境变量组不可用",
    description: "Secret store unavailable state title",
  },
  "envGroups.unavailableBody": {
    message: "此部署未配置密钥存储。",
    description: "Secret store unavailable state body",
  },
  "envGroups.forbiddenTitle": {
    message: "无权访问",
    description: "Env-group forbidden state title",
  },
  "envGroups.forbiddenBody": {
    message: "你没有查看或管理环境变量组的权限。",
    description: "Env-group forbidden state body",
  },
  "envGroups.genericTitle": {
    message: "无法加载环境变量组",
    description: "Env-group generic query error title",
  },
  "envGroups.genericBody": {
    message: "出错了，请重试。",
    description: "Env-group generic query error body",
  },
  "envGroups.errorTitle": {
    message: "无法加载组内容",
    description: "Env-group editor error title",
  },
  "envGroups.errorBody": {
    message: "出错了，请重试。",
    description: "Env-group editor error body",
  },
  "envGroups.notFoundTitle": {
    message: "找不到环境变量组",
    description: "Missing env-group detail title",
  },
  "envGroups.notFoundBody": {
    message: "不存在 ID 为 {id} 的环境变量组。",
    description: "Missing env-group detail body",
  },
  "envGroups.backToList": {
    message: "返回环境变量组",
    description: "Detail-page back button label",
  },
  "envGroups.renameButton": {
    message: "重命名",
    description: "Open rename env-group dialog",
  },
  "envGroups.renameTitle": {
    message: "重命名环境变量组",
    description: "Rename env-group dialog title",
  },
  "envGroups.renameDescription": {
    message: "重命名会保留组的变量、文件和服务关联。",
    description: "Rename env-group dialog description",
  },
  "envGroups.renameSubmit": {
    message: "保存名称",
    description: "Rename env-group submit button",
  },
  "envGroups.deleteButton": {
    message: "删除",
    description: "Open delete env-group dialog",
  },
  "envGroups.deleteTitle": {
    message: "删除 {name}？",
    description: "Delete env-group dialog title",
  },
  "envGroups.deleteDescription": {
    message: "此操作会永久删除该组，并从所有服务取消关联。",
    description: "Delete env-group dialog warning",
  },
  "envGroups.deletePrompt": {
    message: "输入 {id} 以确认。",
    description: "Delete env-group typed-confirm prompt",
  },
  "envGroups.deleteConfirm": {
    message: "删除环境变量组",
    description: "Delete env-group confirm button",
  },
  "envGroups.varsTitle": {
    message: "环境变量",
    description: "Env-group variable editor title",
  },
  "envGroups.varsDescription": {
    message: "变量值会被加密，并应用到每个关联服务。",
    description: "Env-group variable editor description",
  },
  "envGroups.varsEmptyTitle": {
    message: "暂无环境变量",
    description: "Env-group variable editor empty title",
  },
  "envGroups.varsEmptyBody": {
    message: "现在即可添加变量，无需先关联服务。",
    description: "Env-group variable editor empty body",
  },
  "envGroups.varDeleteConfirmBody": {
    message: "所有关联服务都将在没有此变量的情况下重新部署。",
    description: "Delete env-group variable warning",
  },
  "envGroups.filesTitle": {
    message: "密钥文件",
    description: "Env-group secret-file editor title",
  },
  "envGroups.filesDescription": {
    message: "加密文件会挂载到每个关联服务的 /etc/secrets。",
    description: "Env-group secret-file editor description",
  },
  "envGroups.filesEmptyTitle": {
    message: "暂无密钥文件",
    description: "Env-group secret-file editor empty title",
  },
  "envGroups.filesEmptyBody": {
    message: "现在即可添加文件，无需先关联服务。",
    description: "Env-group secret-file editor empty body",
  },
  "envGroups.fileDeleteConfirmBody": {
    message: "所有关联服务都将在没有此文件的情况下重新部署。",
    description: "Delete env-group secret-file warning",
  },
  "envGroups.servicesTitle": {
    message: "关联服务",
    description: "Env-group linked-services card title",
  },
  "envGroups.servicesDescription": {
    message: "关联或取消关联都会重新部署受影响的服务。",
    description: "Env-group linked-services card description",
  },
  "envGroups.selectService": {
    message: "选择服务",
    description: "Link-service selector placeholder",
  },
  "envGroups.linkButton": {
    message: "关联服务",
    description: "Link selected service to env group",
  },
  "envGroups.unlinkButton": {
    message: "取消关联",
    description: "Unlink service from env group",
  },
  "envGroups.noLinkedServices": {
    message: "此组尚未关联任何服务，但仍可完整编辑。",
    description: "No linked services state",
  },
  "envGroups.servicesLoadError": {
    message: "无法加载工作区服务。现有链接仍显示在下方。",
    description: "Linked-services inventory error",
  },
  "envGroups.rolloutNote": {
    message: "关联服务正在重新部署以应用更改。",
    description: "Env-group write rollout toast detail",
  },
  "envGroups.createSuccess": {
    message: "已创建 {name}",
    description: "Env-group create success toast",
  },
  "envGroups.createError": {
    message: "无法创建 {name}",
    description: "Env-group create error toast",
  },
  "envGroups.renameSuccess": {
    message: "已重命名为 {name}",
    description: "Env-group rename success toast",
  },
  "envGroups.renameError": {
    message: "无法重命名该组",
    description: "Env-group rename error toast",
  },
  "envGroups.deleteSuccess": {
    message: "环境变量组已删除",
    description: "Env-group delete success toast",
  },
  "envGroups.deleteError": {
    message: "无法删除该组",
    description: "Env-group delete error toast",
  },
  "envGroups.linkSuccess": {
    message: "服务已关联",
    description: "Env-group link success toast",
  },
  "envGroups.linkError": {
    message: "无法关联服务",
    description: "Env-group link error toast",
  },
  "envGroups.unlinkSuccess": {
    message: "服务已取消关联",
    description: "Env-group unlink success toast",
  },
  "envGroups.unlinkError": {
    message: "无法取消关联服务",
    description: "Env-group unlink error toast",
  },
  "envGroups.varSaveSuccess": {
    message: "已保存 {key}",
    description: "Env-group variable save success toast",
  },
  "envGroups.varsSaveSuccess": {
    message: "环境变量已保存",
    description: "Env-group replace-all variables success toast",
  },
  "envGroups.varsSaveError": {
    message: "无法保存环境变量",
    description: "Env-group replace-all variables error toast",
  },
  "envGroups.varSaveError": {
    message: "无法保存 {key}",
    description: "Env-group variable save error toast",
  },
  "envGroups.varDeleteSuccess": {
    message: "已删除 {key}",
    description: "Env-group variable delete success toast",
  },
  "envGroups.varDeleteError": {
    message: "无法删除 {key}",
    description: "Env-group variable delete error toast",
  },
  "envGroups.fileSaveSuccess": {
    message: "已保存 {name}",
    description: "Env-group file save success toast",
  },
  "envGroups.fileSaveError": {
    message: "无法保存 {name}",
    description: "Env-group file save error toast",
  },
  "envGroups.fileDeleteSuccess": {
    message: "已删除 {name}",
    description: "Env-group file delete success toast",
  },
  "envGroups.fileDeleteError": {
    message: "无法删除 {name}",
    description: "Env-group file delete error toast",
  },
};

export default zhEnvGroups;
