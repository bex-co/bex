import type { TranslationEntry } from "@/i18n";

const jaOrganizations: Record<string, TranslationEntry> = {
  "organizations.switcherOrganization": {
    message: "組織",
    description:
      "Workspace switcher: the sublabel under an organization's name, which opens the org's Workspaces page",
  },
  "organizations.navLabel": {
    message: "組織のページ",
    description:
      "Accessible name of the org page navigation (Workspaces, Members, Settings)",
  },
  "organizations.navWorkspaces": {
    message: "ワークスペース",
    description: "Org page navigation: the Workspaces page",
  },
  "organizations.navMembers": {
    message: "メンバー",
    description: "Org page navigation: the Members page",
  },
  "organizations.navSettings": {
    message: "設定",
    description: "Org page navigation: the Settings page",
  },
  "organizations.untitled": {
    message: "組織",
    description: "Org page title when the organization could not be read",
  },
  "organizations.workspacesTitle": {
    message: "組織のワークスペース",
    description: "Document title of the org's Workspaces page",
  },
  "organizations.membersTitle": {
    message: "組織のメンバー",
    description: "Document title of the org's Members page",
  },
  "organizations.settingsTitle": {
    message: "組織の設定",
    description: "Document title of the org's Settings page",
  },
  "organizations.workspacesDescription": {
    message:
      "組織にはワークスペースを無制限に置けます。各ワークスペースは追加費用なしで組織のプランを使います。",
    description:
      "Org Workspaces page header description (Render's organizations doc)",
  },
  "organizations.membersDescription": {
    message:
      "オーナーは組織を管理し、メンバーは組織のワークスペースに参加し、ゲストは 1 つのワークスペースだけにアクセスします。",
    description: "Org Members page header description",
  },
  "organizations.settingsDescription": {
    message: "組織の名前と監査ログです。",
    description: "Org Settings page header description",
  },
  "organizations.roleOwner": {
    message: "オーナー",
    description: "Org role: Owner (Render's Org Owner)",
  },
  "organizations.roleMember": {
    message: "メンバー",
    description: "Org role: Member (a standard org member)",
  },
  "organizations.roleGuest": {
    message: "ゲスト",
    description: "Org role: Guest (reaches a single workspace)",
  },
  "organizations.notFoundTitle": {
    message: "組織が見つかりません",
    description: "Org pages: ORGANIZATION_NOT_FOUND state title",
  },
  "organizations.notFoundBody": {
    message: "存在しないか、あなたがこの組織のメンバーではありません。",
    description: "Org pages: ORGANIZATION_NOT_FOUND state body",
  },
  "organizations.errorTitle": {
    message: "組織を読み込めませんでした",
    description: "Org pages: read failure state title",
  },
  "organizations.errorBody": {
    message: "しばらくしてからもう一度お試しください。",
    description: "Org pages: read failure state body",
  },

  "organizations.workspacesCardTitle": {
    message: "ワークスペース",
    description: "Org Workspaces page: the workspace list card title",
  },
  "organizations.workspacesCardDescription": {
    message:
      "公開ワークスペースにはどのメンバーも参加できます。招待制のワークスペースには管理者からの招待が必要です。",
    description: "Org Workspaces page: the workspace list card description",
  },
  "organizations.colWorkspace": {
    message: "ワークスペース",
    description: "Org workspace table column: the workspace name",
  },
  "organizations.colPrivacy": {
    message: "公開範囲",
    description: "Org workspace table column: Public or Invite-only",
  },
  "organizations.primary": {
    message: "プライマリ",
    description:
      "Badge on the org's primary workspace (the one that holds the org's plan and billing)",
  },
  "organizations.workspacesEmptyTitle": {
    message: "表示するワークスペースがありません",
    description: "Org Workspaces page: empty list title",
  },
  "organizations.workspacesEmptyBody": {
    message: "この組織で表示できるワークスペースがここに並びます。",
    description: "Org Workspaces page: empty list body",
  },
  "organizations.open": {
    message: "開く",
    description:
      "Org workspace row action: switch to a workspace the caller belongs to",
  },
  "organizations.join": {
    message: "参加",
    description:
      "Org workspace row action: a Member or Owner adds themselves to a Public workspace (Developer role)",
  },
  "organizations.joinAsAdmin": {
    message: "管理者として参加",
    description:
      "Org workspace row action: an Owner adds themselves to any org workspace as admin",
  },
  "organizations.joined": {
    message: "{workspace} に参加しました。",
    description: "Toast after joining a Public workspace",
  },
  "organizations.joinedAsAdmin": {
    message: "{workspace} の管理者になりました。",
    description: "Toast after an Owner joins a workspace as admin",
  },
  "organizations.joinFailed": {
    message: "{workspace} に参加できませんでした。",
    description: "Toast when joining a workspace failed for no stated reason",
  },
  "organizations.newWorkspace": {
    message: "新しいワークスペース",
    description:
      "Org Workspaces page: the Owner's button that opens the create dialog (Render: + New Workspace)",
  },
  "organizations.createTitle": {
    message: "{organization} に新しいワークスペースを作成",
    description: "Create org workspace dialog title",
  },
  "organizations.createDescription": {
    message: "あなたが最初の管理者になり、すぐにサービスを作成できます。",
    description: "Create org workspace dialog description",
  },
  "organizations.fieldName": {
    message: "名前",
    description: "Create workspace / rename organization: the name field label",
  },
  "organizations.fieldPrivacy": {
    message: "公開範囲",
    description: "Create workspace dialog: the privacy choice's legend",
  },
  "organizations.planIncluded": {
    message: "組織のプランに含まれます",
    description:
      "Create workspace dialog: the new workspace takes the org's plan, no price and no payment step",
  },
  "organizations.planIncludedDetail": {
    message: "{plan} プラン、追加費用なし",
    description:
      "Create workspace dialog: the org's plan name under 'Included in your organization's plan'",
  },
  "organizations.createSubmit": {
    message: "ワークスペースを作成",
    description: "Create org workspace dialog: submit button (Render's label)",
  },
  "organizations.cancel": {
    message: "キャンセル",
    description: "Org dialogs: cancel button",
  },
  "organizations.workspaceCreated": {
    message: "{workspace} を作成しました。",
    description: "Toast after creating an org workspace",
  },
  "organizations.createFailed": {
    message: "ワークスペースを作成できませんでした。",
    description: "Create org workspace: failure with no stated reason",
  },

  "organizations.membersCardTitle": {
    message: "メンバー",
    description: "Org Members page: the member list card title",
  },
  "organizations.membersCardDescription": {
    message:
      "組織に属するすべての人と、その組織ロールです。ワークスペースのロールは各ワークスペースで設定します。",
    description: "Org Members page: the member list card description",
  },
  "organizations.colMember": {
    message: "メンバー",
    description: "Org member table column: the person",
  },
  "organizations.colRole": {
    message: "ロール",
    description: "Org member table column: the org role",
  },
  "organizations.guestWorkspace": {
    message: "ワークスペース: {workspace}",
    description: "Org member table: the one workspace a Guest belongs to",
  },
  "organizations.guestNoWorkspace": {
    message: "まだワークスペースに所属していません",
    description: "Org member table: a Guest who belongs to no workspace",
  },
  "organizations.roleFor": {
    message: "{identity} の組織ロール",
    description: "Accessible name of an org member's role select",
  },
  "organizations.removeMember": {
    message: "{identity} を組織から削除",
    description: "Accessible name of an org member row's remove button",
  },
  "organizations.removeTitle": {
    message: "組織から削除しますか？",
    description: "Org member remove confirmation title",
  },
  "organizations.removeConfirm": {
    message:
      "{identity} は組織とそのすべてのワークスペースにアクセスできなくなり、それらのワークスペースでの API キーも取り消されます。",
    description: "Org member remove confirmation description",
  },
  "organizations.remove": {
    message: "削除",
    description: "Org member remove confirmation: confirm button",
  },
  "organizations.removed": {
    message: "{identity} を組織から削除しました。",
    description: "Toast after removing an org member",
  },
  "organizations.removeFailed": {
    message: "{identity} を削除できませんでした。",
    description: "Org member removal failure with no stated reason",
  },
  "organizations.roleChanged": {
    message: "{identity} のロールを{role}に変更しました。",
    description: "Toast after changing an org member's role",
  },
  "organizations.roleChangeFailed": {
    message: "ロールを変更できませんでした。",
    description: "Org role change failure with no stated reason",
  },
  "organizations.membersForbiddenTitle": {
    message: "ゲストにはメンバーが表示されません",
    description:
      "Org Members page: a Guest's state (the API answers FORBIDDEN)",
  },
  "organizations.membersForbiddenBody": {
    message:
      "組織に誰が属しているかを見られるのは、オーナーとメンバーだけです。",
    description: "Org Members page: a Guest's state body",
  },
  "organizations.membersErrorTitle": {
    message: "メンバーを読み込めませんでした",
    description: "Org Members page: read failure title",
  },
  "organizations.invite": {
    message: "招待",
    description: "Org Members page: the Owner's invite button",
  },
  "organizations.inviteTitle": {
    message: "組織に招待",
    description: "Org invite dialog title",
  },
  "organizations.inviteDescription": {
    message:
      "オーナーまたはメンバーとして参加するためのリンクがメールで届きます。ワークスペースの管理者は自分のワークスペースに人を招待します。",
    description: "Org invite dialog description",
  },
  "organizations.fieldEmail": {
    message: "メールアドレス",
    description: "Org invite dialog: email field label",
  },
  "organizations.fieldEmailInvalid": {
    message: "有効なメールアドレスを入力してください。",
    description: "Org invite dialog: malformed email",
  },
  "organizations.fieldRole": {
    message: "ロール",
    description: "Org invite dialog: role field label",
  },
  "organizations.inviteSubmit": {
    message: "招待を送信",
    description: "Org invite dialog: submit button",
  },
  "organizations.invited": {
    message: "{email} を招待しました。",
    description: "Toast after sending an org invite",
  },
  "organizations.inviteFailed": {
    message: "招待を送信できませんでした。",
    description: "Org invite failure with no stated reason",
  },
  "organizations.pendingTitle": {
    message: "保留中の招待",
    description: "Org Members page: the pending invite list heading",
  },
  "organizations.colEmail": {
    message: "メール",
    description: "Org pending invite table column: the invitee's email",
  },
  "organizations.revokeInvite": {
    message: "取り消す",
    description: "Org pending invite row action",
  },
  "organizations.inviteRevoked": {
    message: "{email} への招待を取り消しました。",
    description: "Toast after revoking an org invite",
  },
  "organizations.revokeFailed": {
    message: "招待を取り消せませんでした。",
    description: "Org invite revoke failure with no stated reason",
  },

  "organizations.nameTitle": {
    message: "組織名",
    description: "Org Settings page: the name card title",
  },
  "organizations.nameDescription": {
    message: "組織のすべての人のワークスペースのドロップダウンに表示されます。",
    description: "Org Settings page: the name card description",
  },
  "organizations.nameSave": {
    message: "保存",
    description: "Org Settings page: save the new name",
  },
  "organizations.nameSaved": {
    message: "組織名を {name} に変更しました。",
    description: "Toast after renaming the org",
  },
  "organizations.renameFailed": {
    message: "組織名を変更できませんでした。",
    description: "Org rename failure with no stated reason",
  },
  "organizations.nameOwnerOnly": {
    message: "組織名を変更できるのはオーナーだけです。",
    description:
      "Org Settings page: why the name is read-only for a Member or Guest",
  },
  "organizations.auditTitle": {
    message: "監査ログ",
    description:
      "Org Settings page: the audit log section title (Render's label)",
  },
  "organizations.auditDescription": {
    message:
      "メンバー、ワークスペース、設定など、組織そのもののイベントを CSV でエクスポートします。これらはどのワークスペースのログにも含まれず、1 回のエクスポートは最大 {days} 日分です。",
    description: "Org Settings page: the audit log section description",
  },

  "organizations.errorNotFound": {
    message:
      "この組織は存在しないか、あなたはこの組織のメンバーではありません。",
    description: "ORGANIZATION_NOT_FOUND refusal",
  },
  "organizations.errorForbidden": {
    message: "この組織でその操作を行う権限がありません。",
    description: "FORBIDDEN refusal of an org action",
  },
  "organizations.errorLastOwner": {
    message:
      "組織には少なくとも 1 人のオーナーが必要です。先に別の人をオーナーにしてください。",
    description: "ORGANIZATION_LAST_OWNER refusal",
  },
  "organizations.errorMemberLastAdmin": {
    message:
      "この人は {workspace} の最後の管理者です。先に別の人を管理者にしてください。",
    description: "ORGANIZATION_MEMBER_LAST_ADMIN refusal, naming the workspace",
  },
  "organizations.errorAlreadyMember": {
    message: "この人はすでに組織に所属しています。",
    description: "ORGANIZATION_ALREADY_MEMBER refusal of an org invite",
  },
  "organizations.errorGuestSingleWorkspace": {
    message:
      "ゲストは組織のワークスペース 1 つだけに所属します。複数のワークスペースに参加させるには、メンバーにしてください。",
    description: "ORGANIZATION_GUEST_SINGLE_WORKSPACE refusal",
  },
  "organizations.errorOwnRole": {
    message:
      "自分の組織ロールは変更できません。別のオーナーに変更を依頼してください。",
    description: "CANNOT_CHANGE_OWN_ROLE refusal",
  },
  "organizations.errorRemoveSelf": {
    message: "自分自身を組織から削除することはできません。",
    description: "CANNOT_REMOVE_SELF refusal",
  },
  "organizations.errorInvalidPrivacy": {
    message: "公開か招待制のどちらかを選んでください。",
    description: "INVALID_PRIVACY refusal",
  },
};

export default jaOrganizations;
