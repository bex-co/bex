import type { TranslationEntry } from "@/i18n";

const zhOrganizations: Record<string, TranslationEntry> = {
  "organizations.switcherOrganization": {
    message: "组织",
    description:
      "Workspace switcher: the sublabel under an organization's name, which opens the org's Workspaces page",
  },
  "organizations.navLabel": {
    message: "组织页面",
    description:
      "Accessible name of the org page navigation (Workspaces, Members, Settings)",
  },
  "organizations.navWorkspaces": {
    message: "工作区",
    description: "Org page navigation: the Workspaces page",
  },
  "organizations.navMembers": {
    message: "成员",
    description: "Org page navigation: the Members page",
  },
  "organizations.navSettings": {
    message: "设置",
    description: "Org page navigation: the Settings page",
  },
  "organizations.untitled": {
    message: "组织",
    description: "Org page title when the organization could not be read",
  },
  "organizations.workspacesTitle": {
    message: "组织工作区",
    description: "Document title of the org's Workspaces page",
  },
  "organizations.membersTitle": {
    message: "组织成员",
    description: "Document title of the org's Members page",
  },
  "organizations.settingsTitle": {
    message: "组织设置",
    description: "Document title of the org's Settings page",
  },
  "organizations.workspacesDescription": {
    message: "一个组织可容纳无限个工作区，每个都使用组织的套餐，无需额外费用。",
    description:
      "Org Workspaces page header description (Render's organizations doc)",
  },
  "organizations.membersDescription": {
    message: "所有者管理组织，成员加入其工作区，访客只能访问一个工作区。",
    description: "Org Members page header description",
  },
  "organizations.settingsDescription": {
    message: "组织的名称及其审计日志。",
    description: "Org Settings page header description",
  },
  "organizations.roleOwner": {
    message: "所有者",
    description: "Org role: Owner (Render's Org Owner)",
  },
  "organizations.roleMember": {
    message: "成员",
    description: "Org role: Member (a standard org member)",
  },
  "organizations.roleGuest": {
    message: "访客",
    description: "Org role: Guest (reaches a single workspace)",
  },
  "organizations.notFoundTitle": {
    message: "找不到该组织",
    description: "Org pages: ORGANIZATION_NOT_FOUND state title",
  },
  "organizations.notFoundBody": {
    message: "它不存在，或者你不是它的成员。",
    description: "Org pages: ORGANIZATION_NOT_FOUND state body",
  },
  "organizations.errorTitle": {
    message: "无法加载该组织",
    description: "Org pages: read failure state title",
  },
  "organizations.errorBody": {
    message: "请稍后再试。",
    description: "Org pages: read failure state body",
  },

  "organizations.workspacesCardTitle": {
    message: "工作区",
    description: "Org Workspaces page: the workspace list card title",
  },
  "organizations.workspacesCardDescription": {
    message:
      "任何成员都可以加入公开工作区。仅限邀请的工作区需要其管理员发出邀请。",
    description: "Org Workspaces page: the workspace list card description",
  },
  "organizations.colWorkspace": {
    message: "工作区",
    description: "Org workspace table column: the workspace name",
  },
  "organizations.colPrivacy": {
    message: "访问权限",
    description: "Org workspace table column: Public or Invite-only",
  },
  "organizations.primary": {
    message: "主工作区",
    description:
      "Badge on the org's primary workspace (the one that holds the org's plan and billing)",
  },
  "organizations.workspacesEmptyTitle": {
    message: "没有可显示的工作区",
    description: "Org Workspaces page: empty list title",
  },
  "organizations.workspacesEmptyBody": {
    message: "你在此组织中能看到的工作区会显示在这里。",
    description: "Org Workspaces page: empty list body",
  },
  "organizations.open": {
    message: "打开",
    description:
      "Org workspace row action: switch to a workspace the caller belongs to",
  },
  "organizations.join": {
    message: "加入",
    description:
      "Org workspace row action: a Member or Owner adds themselves to a Public workspace (Developer role)",
  },
  "organizations.joinAsAdmin": {
    message: "以管理员身份加入",
    description:
      "Org workspace row action: an Owner adds themselves to any org workspace as admin",
  },
  "organizations.joined": {
    message: "你已加入 {workspace}。",
    description: "Toast after joining a Public workspace",
  },
  "organizations.joinedAsAdmin": {
    message: "你现在是 {workspace} 的管理员。",
    description: "Toast after an Owner joins a workspace as admin",
  },
  "organizations.joinFailed": {
    message: "无法加入 {workspace}。",
    description: "Toast when joining a workspace failed for no stated reason",
  },
  "organizations.newWorkspace": {
    message: "新建工作区",
    description:
      "Org Workspaces page: the Owner's button that opens the create dialog (Render: + New Workspace)",
  },
  "organizations.createTitle": {
    message: "在 {organization} 中新建工作区",
    description: "Create org workspace dialog title",
  },
  "organizations.createDescription": {
    message: "你将成为它的第一位管理员，并可以立即开始创建服务。",
    description: "Create org workspace dialog description",
  },
  "organizations.fieldName": {
    message: "名称",
    description: "Create workspace / rename organization: the name field label",
  },
  "organizations.fieldPrivacy": {
    message: "访问权限",
    description: "Create workspace dialog: the privacy choice's legend",
  },
  "organizations.planIncluded": {
    message: "已包含在你的组织套餐中",
    description:
      "Create workspace dialog: the new workspace takes the org's plan, no price and no payment step",
  },
  "organizations.planIncludedDetail": {
    message: "{plan} 套餐，无需额外费用",
    description:
      "Create workspace dialog: the org's plan name under 'Included in your organization's plan'",
  },
  "organizations.createSubmit": {
    message: "创建工作区",
    description: "Create org workspace dialog: submit button (Render's label)",
  },
  "organizations.cancel": {
    message: "取消",
    description: "Org dialogs: cancel button",
  },
  "organizations.workspaceCreated": {
    message: "已创建 {workspace}。",
    description: "Toast after creating an org workspace",
  },
  "organizations.createFailed": {
    message: "无法创建该工作区。",
    description: "Create org workspace: failure with no stated reason",
  },

  "organizations.membersCardTitle": {
    message: "成员",
    description: "Org Members page: the member list card title",
  },
  "organizations.membersCardDescription": {
    message: "组织中的每个人及其组织角色。工作区角色在各个工作区中设置。",
    description: "Org Members page: the member list card description",
  },
  "organizations.colMember": {
    message: "成员",
    description: "Org member table column: the person",
  },
  "organizations.colRole": {
    message: "角色",
    description: "Org member table column: the org role",
  },
  "organizations.guestWorkspace": {
    message: "工作区：{workspace}",
    description: "Org member table: the one workspace a Guest belongs to",
  },
  "organizations.guestNoWorkspace": {
    message: "尚未加入任何工作区",
    description: "Org member table: a Guest who belongs to no workspace",
  },
  "organizations.roleFor": {
    message: "{identity} 的组织角色",
    description: "Accessible name of an org member's role select",
  },
  "organizations.removeMember": {
    message: "将 {identity} 移出组织",
    description: "Accessible name of an org member row's remove button",
  },
  "organizations.removeTitle": {
    message: "要移出组织吗？",
    description: "Org member remove confirmation title",
  },
  "organizations.removeConfirm": {
    message:
      "{identity} 将失去对该组织及其所有工作区的访问权限，其在这些工作区中的 API 密钥也会被撤销。",
    description: "Org member remove confirmation description",
  },
  "organizations.remove": {
    message: "移出",
    description: "Org member remove confirmation: confirm button",
  },
  "organizations.removed": {
    message: "已将 {identity} 移出组织。",
    description: "Toast after removing an org member",
  },
  "organizations.removeFailed": {
    message: "无法移出 {identity}。",
    description: "Org member removal failure with no stated reason",
  },
  "organizations.roleChanged": {
    message: "{identity} 现在是{role}。",
    description: "Toast after changing an org member's role",
  },
  "organizations.roleChangeFailed": {
    message: "无法更改该角色。",
    description: "Org role change failure with no stated reason",
  },
  "organizations.membersForbiddenTitle": {
    message: "访客看不到成员列表",
    description:
      "Org Members page: a Guest's state (the API answers FORBIDDEN)",
  },
  "organizations.membersForbiddenBody": {
    message: "只有组织的所有者和成员可以看到组织中有哪些人。",
    description: "Org Members page: a Guest's state body",
  },
  "organizations.membersErrorTitle": {
    message: "无法加载成员",
    description: "Org Members page: read failure title",
  },
  "organizations.invite": {
    message: "邀请",
    description: "Org Members page: the Owner's invite button",
  },
  "organizations.inviteTitle": {
    message: "邀请加入组织",
    description: "Org invite dialog title",
  },
  "organizations.inviteDescription": {
    message:
      "对方会收到一封邮件，可通过其中的链接以所有者或成员身份加入。工作区管理员可以邀请他人加入自己的工作区。",
    description: "Org invite dialog description",
  },
  "organizations.fieldEmail": {
    message: "邮箱地址",
    description: "Org invite dialog: email field label",
  },
  "organizations.fieldEmailInvalid": {
    message: "请输入有效的邮箱地址。",
    description: "Org invite dialog: malformed email",
  },
  "organizations.fieldRole": {
    message: "角色",
    description: "Org invite dialog: role field label",
  },
  "organizations.inviteSubmit": {
    message: "发送邀请",
    description: "Org invite dialog: submit button",
  },
  "organizations.invited": {
    message: "已邀请 {email}。",
    description: "Toast after sending an org invite",
  },
  "organizations.inviteFailed": {
    message: "无法发送邀请。",
    description: "Org invite failure with no stated reason",
  },
  "organizations.pendingTitle": {
    message: "待接受的邀请",
    description: "Org Members page: the pending invite list heading",
  },
  "organizations.colEmail": {
    message: "邮箱",
    description: "Org pending invite table column: the invitee's email",
  },
  "organizations.revokeInvite": {
    message: "撤销",
    description: "Org pending invite row action",
  },
  "organizations.inviteRevoked": {
    message: "已撤销发给 {email} 的邀请。",
    description: "Toast after revoking an org invite",
  },
  "organizations.revokeFailed": {
    message: "无法撤销该邀请。",
    description: "Org invite revoke failure with no stated reason",
  },

  "organizations.nameTitle": {
    message: "组织名称",
    description: "Org Settings page: the name card title",
  },
  "organizations.nameDescription": {
    message: "组织中的每个人都会在工作区下拉菜单中看到它。",
    description: "Org Settings page: the name card description",
  },
  "organizations.nameSave": {
    message: "保存",
    description: "Org Settings page: save the new name",
  },
  "organizations.nameSaved": {
    message: "组织已重命名为 {name}。",
    description: "Toast after renaming the org",
  },
  "organizations.renameFailed": {
    message: "无法重命名该组织。",
    description: "Org rename failure with no stated reason",
  },
  "organizations.nameOwnerOnly": {
    message: "只有所有者可以重命名组织。",
    description:
      "Org Settings page: why the name is read-only for a Member or Guest",
  },
  "organizations.auditTitle": {
    message: "审计日志",
    description:
      "Org Settings page: the audit log section title (Render's label)",
  },
  "organizations.auditDescription": {
    message:
      "将组织自身的事件（如成员、工作区和设置的变更）导出为 CSV。这些事件不在任何工作区的日志中，每次导出最多涵盖 {days} 天。",
    description: "Org Settings page: the audit log section description",
  },

  "organizations.errorNotFound": {
    message: "该组织不存在，或者你不是它的成员。",
    description: "ORGANIZATION_NOT_FOUND refusal",
  },
  "organizations.errorForbidden": {
    message: "你无权在此组织中执行该操作。",
    description: "FORBIDDEN refusal of an org action",
  },
  "organizations.errorLastOwner": {
    message: "组织至少需要一位所有者。请先将其他人设为所有者。",
    description: "ORGANIZATION_LAST_OWNER refusal",
  },
  "organizations.errorMemberLastAdmin": {
    message:
      "对方是 {workspace} 的最后一位管理员。请先将其他人设为该工作区的管理员。",
    description: "ORGANIZATION_MEMBER_LAST_ADMIN refusal, naming the workspace",
  },
  "organizations.errorAlreadyMember": {
    message: "此人已在该组织中。",
    description: "ORGANIZATION_ALREADY_MEMBER refusal of an org invite",
  },
  "organizations.errorGuestSingleWorkspace": {
    message:
      "访客只能属于组织中的一个工作区。将对方设为成员才能加入更多工作区。",
    description: "ORGANIZATION_GUEST_SINGLE_WORKSPACE refusal",
  },
  "organizations.errorOwnRole": {
    message: "你不能更改自己的组织角色。请让另一位所有者来更改。",
    description: "CANNOT_CHANGE_OWN_ROLE refusal",
  },
  "organizations.errorRemoveSelf": {
    message: "你不能将自己移出组织。",
    description: "CANNOT_REMOVE_SELF refusal",
  },
  "organizations.errorInvalidPrivacy": {
    message: "请选择公开或仅限邀请。",
    description: "INVALID_PRIVACY refusal",
  },
};

export default zhOrganizations;
