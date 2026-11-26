import type { TranslationEntry } from "@/i18n";

const koOrganizations: Record<string, TranslationEntry> = {
  "organizations.switcherOrganization": {
    message: "조직",
    description:
      "Workspace switcher: the sublabel under an organization's name, which opens the org's Workspaces page",
  },
  "organizations.navLabel": {
    message: "조직 페이지",
    description:
      "Accessible name of the org page navigation (Workspaces, Members, Settings)",
  },
  "organizations.navWorkspaces": {
    message: "워크스페이스",
    description: "Org page navigation: the Workspaces page",
  },
  "organizations.navMembers": {
    message: "멤버",
    description: "Org page navigation: the Members page",
  },
  "organizations.navSettings": {
    message: "설정",
    description: "Org page navigation: the Settings page",
  },
  "organizations.untitled": {
    message: "조직",
    description: "Org page title when the organization could not be read",
  },
  "organizations.workspacesTitle": {
    message: "조직 워크스페이스",
    description: "Document title of the org's Workspaces page",
  },
  "organizations.membersTitle": {
    message: "조직 멤버",
    description: "Document title of the org's Members page",
  },
  "organizations.settingsTitle": {
    message: "조직 설정",
    description: "Document title of the org's Settings page",
  },
  "organizations.workspacesDescription": {
    message:
      "조직에는 워크스페이스를 무제한으로 둘 수 있으며, 각 워크스페이스는 추가 비용 없이 조직의 플랜을 사용합니다.",
    description:
      "Org Workspaces page header description (Render's organizations doc)",
  },
  "organizations.membersDescription": {
    message:
      "소유자는 조직을 관리하고, 멤버는 조직의 워크스페이스에 참여하며, 게스트는 하나의 워크스페이스에만 접근합니다.",
    description: "Org Members page header description",
  },
  "organizations.settingsDescription": {
    message: "조직의 이름과 감사 로그입니다.",
    description: "Org Settings page header description",
  },
  "organizations.roleOwner": {
    message: "소유자",
    description: "Org role: Owner (Render's Org Owner)",
  },
  "organizations.roleMember": {
    message: "멤버",
    description: "Org role: Member (a standard org member)",
  },
  "organizations.roleGuest": {
    message: "게스트",
    description: "Org role: Guest (reaches a single workspace)",
  },
  "organizations.notFoundTitle": {
    message: "조직을 찾을 수 없습니다",
    description: "Org pages: ORGANIZATION_NOT_FOUND state title",
  },
  "organizations.notFoundBody": {
    message: "존재하지 않거나, 이 조직의 멤버가 아닙니다.",
    description: "Org pages: ORGANIZATION_NOT_FOUND state body",
  },
  "organizations.errorTitle": {
    message: "조직을 불러오지 못했습니다",
    description: "Org pages: read failure state title",
  },
  "organizations.errorBody": {
    message: "잠시 후 다시 시도하세요.",
    description: "Org pages: read failure state body",
  },

  "organizations.workspacesCardTitle": {
    message: "워크스페이스",
    description: "Org Workspaces page: the workspace list card title",
  },
  "organizations.workspacesCardDescription": {
    message:
      "모든 멤버가 공개 워크스페이스에 참여할 수 있습니다. 초대 전용 워크스페이스는 관리자의 초대가 필요합니다.",
    description: "Org Workspaces page: the workspace list card description",
  },
  "organizations.colWorkspace": {
    message: "워크스페이스",
    description: "Org workspace table column: the workspace name",
  },
  "organizations.colPrivacy": {
    message: "공개 범위",
    description: "Org workspace table column: Public or Invite-only",
  },
  "organizations.primary": {
    message: "기본",
    description:
      "Badge on the org's primary workspace (the one that holds the org's plan and billing)",
  },
  "organizations.workspacesEmptyTitle": {
    message: "표시할 워크스페이스가 없습니다",
    description: "Org Workspaces page: empty list title",
  },
  "organizations.workspacesEmptyBody": {
    message: "이 조직에서 볼 수 있는 워크스페이스가 여기에 표시됩니다.",
    description: "Org Workspaces page: empty list body",
  },
  "organizations.open": {
    message: "열기",
    description:
      "Org workspace row action: switch to a workspace the caller belongs to",
  },
  "organizations.join": {
    message: "참여",
    description:
      "Org workspace row action: a Member or Owner adds themselves to a Public workspace (Developer role)",
  },
  "organizations.joinAsAdmin": {
    message: "관리자로 참여",
    description:
      "Org workspace row action: an Owner adds themselves to any org workspace as admin",
  },
  "organizations.joined": {
    message: "{workspace}에 참여했습니다.",
    description: "Toast after joining a Public workspace",
  },
  "organizations.joinedAsAdmin": {
    message: "이제 {workspace}의 관리자입니다.",
    description: "Toast after an Owner joins a workspace as admin",
  },
  "organizations.joinFailed": {
    message: "{workspace}에 참여하지 못했습니다.",
    description: "Toast when joining a workspace failed for no stated reason",
  },
  "organizations.newWorkspace": {
    message: "새 워크스페이스",
    description:
      "Org Workspaces page: the Owner's button that opens the create dialog (Render: + New Workspace)",
  },
  "organizations.createTitle": {
    message: "{organization}에 새 워크스페이스 만들기",
    description: "Create org workspace dialog title",
  },
  "organizations.createDescription": {
    message: "첫 번째 관리자가 되며 바로 서비스를 만들 수 있습니다.",
    description: "Create org workspace dialog description",
  },
  "organizations.fieldName": {
    message: "이름",
    description: "Create workspace / rename organization: the name field label",
  },
  "organizations.fieldPrivacy": {
    message: "공개 범위",
    description: "Create workspace dialog: the privacy choice's legend",
  },
  "organizations.planIncluded": {
    message: "조직 플랜에 포함됨",
    description:
      "Create workspace dialog: the new workspace takes the org's plan, no price and no payment step",
  },
  "organizations.planIncludedDetail": {
    message: "{plan} 플랜, 추가 비용 없음",
    description:
      "Create workspace dialog: the org's plan name under 'Included in your organization's plan'",
  },
  "organizations.createSubmit": {
    message: "워크스페이스 만들기",
    description: "Create org workspace dialog: submit button (Render's label)",
  },
  "organizations.cancel": {
    message: "취소",
    description: "Org dialogs: cancel button",
  },
  "organizations.workspaceCreated": {
    message: "{workspace}을(를) 만들었습니다.",
    description: "Toast after creating an org workspace",
  },
  "organizations.createFailed": {
    message: "워크스페이스를 만들지 못했습니다.",
    description: "Create org workspace: failure with no stated reason",
  },

  "organizations.membersCardTitle": {
    message: "멤버",
    description: "Org Members page: the member list card title",
  },
  "organizations.membersCardDescription": {
    message:
      "조직의 모든 사람과 그들의 조직 역할입니다. 워크스페이스 역할은 각 워크스페이스에서 설정합니다.",
    description: "Org Members page: the member list card description",
  },
  "organizations.colMember": {
    message: "멤버",
    description: "Org member table column: the person",
  },
  "organizations.colRole": {
    message: "역할",
    description: "Org member table column: the org role",
  },
  "organizations.guestWorkspace": {
    message: "워크스페이스: {workspace}",
    description: "Org member table: the one workspace a Guest belongs to",
  },
  "organizations.guestNoWorkspace": {
    message: "아직 워크스페이스에 속하지 않음",
    description: "Org member table: a Guest who belongs to no workspace",
  },
  "organizations.roleFor": {
    message: "{identity}의 조직 역할",
    description: "Accessible name of an org member's role select",
  },
  "organizations.removeMember": {
    message: "{identity}을(를) 조직에서 제거",
    description: "Accessible name of an org member row's remove button",
  },
  "organizations.removeTitle": {
    message: "조직에서 제거할까요?",
    description: "Org member remove confirmation title",
  },
  "organizations.removeConfirm": {
    message:
      "{identity}은(는) 조직과 모든 조직 워크스페이스에 대한 접근 권한을 잃으며, 해당 워크스페이스의 API 키도 취소됩니다.",
    description: "Org member remove confirmation description",
  },
  "organizations.remove": {
    message: "제거",
    description: "Org member remove confirmation: confirm button",
  },
  "organizations.removed": {
    message: "{identity}을(를) 조직에서 제거했습니다.",
    description: "Toast after removing an org member",
  },
  "organizations.removeFailed": {
    message: "{identity}을(를) 제거하지 못했습니다.",
    description: "Org member removal failure with no stated reason",
  },
  "organizations.roleChanged": {
    message: "{identity}의 역할이 이제 {role}입니다.",
    description: "Toast after changing an org member's role",
  },
  "organizations.roleChangeFailed": {
    message: "역할을 변경하지 못했습니다.",
    description: "Org role change failure with no stated reason",
  },
  "organizations.membersForbiddenTitle": {
    message: "게스트에게는 멤버 목록이 표시되지 않습니다",
    description:
      "Org Members page: a Guest's state (the API answers FORBIDDEN)",
  },
  "organizations.membersForbiddenBody": {
    message: "조직의 소유자와 멤버만 조직에 누가 속해 있는지 볼 수 있습니다.",
    description: "Org Members page: a Guest's state body",
  },
  "organizations.membersErrorTitle": {
    message: "멤버를 불러오지 못했습니다",
    description: "Org Members page: read failure title",
  },
  "organizations.invite": {
    message: "초대",
    description: "Org Members page: the Owner's invite button",
  },
  "organizations.inviteTitle": {
    message: "조직에 초대",
    description: "Org invite dialog title",
  },
  "organizations.inviteDescription": {
    message:
      "소유자 또는 멤버로 참여할 수 있는 링크가 담긴 이메일이 발송됩니다. 워크스페이스 관리자는 자신의 워크스페이스에 사람을 초대합니다.",
    description: "Org invite dialog description",
  },
  "organizations.fieldEmail": {
    message: "이메일 주소",
    description: "Org invite dialog: email field label",
  },
  "organizations.fieldEmailInvalid": {
    message: "올바른 이메일 주소를 입력하세요.",
    description: "Org invite dialog: malformed email",
  },
  "organizations.fieldRole": {
    message: "역할",
    description: "Org invite dialog: role field label",
  },
  "organizations.inviteSubmit": {
    message: "초대 보내기",
    description: "Org invite dialog: submit button",
  },
  "organizations.invited": {
    message: "{email}을(를) 초대했습니다.",
    description: "Toast after sending an org invite",
  },
  "organizations.inviteFailed": {
    message: "초대를 보내지 못했습니다.",
    description: "Org invite failure with no stated reason",
  },
  "organizations.pendingTitle": {
    message: "대기 중인 초대",
    description: "Org Members page: the pending invite list heading",
  },
  "organizations.colEmail": {
    message: "이메일",
    description: "Org pending invite table column: the invitee's email",
  },
  "organizations.revokeInvite": {
    message: "취소",
    description: "Org pending invite row action",
  },
  "organizations.inviteRevoked": {
    message: "{email}에게 보낸 초대를 취소했습니다.",
    description: "Toast after revoking an org invite",
  },
  "organizations.revokeFailed": {
    message: "초대를 취소하지 못했습니다.",
    description: "Org invite revoke failure with no stated reason",
  },

  "organizations.nameTitle": {
    message: "조직 이름",
    description: "Org Settings page: the name card title",
  },
  "organizations.nameDescription": {
    message: "조직의 모든 사람에게 워크스페이스 드롭다운에 표시됩니다.",
    description: "Org Settings page: the name card description",
  },
  "organizations.nameSave": {
    message: "저장",
    description: "Org Settings page: save the new name",
  },
  "organizations.nameSaved": {
    message: "조직 이름을 {name}(으)로 변경했습니다.",
    description: "Toast after renaming the org",
  },
  "organizations.renameFailed": {
    message: "조직 이름을 변경하지 못했습니다.",
    description: "Org rename failure with no stated reason",
  },
  "organizations.nameOwnerOnly": {
    message: "소유자만 조직 이름을 변경할 수 있습니다.",
    description:
      "Org Settings page: why the name is read-only for a Member or Guest",
  },
  "organizations.auditTitle": {
    message: "감사 로그",
    description:
      "Org Settings page: the audit log section title (Render's label)",
  },
  "organizations.auditDescription": {
    message:
      "멤버, 워크스페이스, 설정 등 조직 자체의 이벤트를 CSV로 내보냅니다. 이 이벤트는 어떤 워크스페이스 로그에도 없으며, 한 번에 최대 {days}일까지 내보낼 수 있습니다.",
    description: "Org Settings page: the audit log section description",
  },

  "organizations.errorNotFound": {
    message: "이 조직이 존재하지 않거나, 이 조직의 멤버가 아닙니다.",
    description: "ORGANIZATION_NOT_FOUND refusal",
  },
  "organizations.errorForbidden": {
    message: "이 조직에서 해당 작업을 할 권한이 없습니다.",
    description: "FORBIDDEN refusal of an org action",
  },
  "organizations.errorLastOwner": {
    message:
      "조직에는 소유자가 최소 한 명 있어야 합니다. 먼저 다른 사람을 소유자로 지정하세요.",
    description: "ORGANIZATION_LAST_OWNER refusal",
  },
  "organizations.errorMemberLastAdmin": {
    message:
      "{workspace}의 마지막 관리자입니다. 먼저 다른 사람을 관리자로 지정하세요.",
    description: "ORGANIZATION_MEMBER_LAST_ADMIN refusal, naming the workspace",
  },
  "organizations.errorAlreadyMember": {
    message: "이 사람은 이미 조직에 속해 있습니다.",
    description: "ORGANIZATION_ALREADY_MEMBER refusal of an org invite",
  },
  "organizations.errorGuestSingleWorkspace": {
    message:
      "게스트는 조직의 워크스페이스 하나에만 속합니다. 더 많은 워크스페이스에 추가하려면 멤버로 지정하세요.",
    description: "ORGANIZATION_GUEST_SINGLE_WORKSPACE refusal",
  },
  "organizations.errorOwnRole": {
    message:
      "자신의 조직 역할은 변경할 수 없습니다. 다른 소유자에게 변경을 요청하세요.",
    description: "CANNOT_CHANGE_OWN_ROLE refusal",
  },
  "organizations.errorRemoveSelf": {
    message: "자신을 조직에서 제거할 수 없습니다.",
    description: "CANNOT_REMOVE_SELF refusal",
  },
  "organizations.errorInvalidPrivacy": {
    message: "공개 또는 초대 전용을 선택하세요.",
    description: "INVALID_PRIVACY refusal",
  },
  "organizations.securityTitle": {
    message: "보안",
    description:
      "Org Settings page: title of the login requirements card (Render's section name, where it puts Require Login Method; w7/m466)",
  },
  "organizations.securityDescription": {
    message:
      "조직의 모든 워크스페이스와 조직 자체 페이지에서 멤버가 로그인해야 하는 방식입니다. 각 워크스페이스의 자체 설정에 더해 적용됩니다.",
    description: "Org Settings page: what the Security card's settings do",
  },
  "organizations.securityOwnerOnly": {
    message: "소유자만 이 설정을 변경할 수 있습니다.",
    description:
      "Org Settings page: why the Security settings are read-only for a Member or Guest",
  },
  "organizations.requireTwoFactorLabel": {
    message: "2단계 인증 요구",
    description:
      "Org Settings page: label of the switch that requires 2FA in every workspace of the org",
  },
  "organizations.requireTwoFactorDescription": {
    message:
      "2단계 인증(2FA)을 사용하지 않는 멤버는 2단계 인증을 켜기 전까지 조직과 조직의 워크스페이스에 접근할 수 없습니다. 그때까지 API 키와 SSH 키도 차단됩니다.",
    description: "Org Settings page: what requiring 2FA does",
  },
  "organizations.requireGoogleLoginLabel": {
    message: "Google 로그인 요구",
    description:
      "Org Settings page: label of the switch that requires signing in with Google in every workspace of the org",
  },
  "organizations.requireGoogleLoginDescription": {
    message:
      "bex 이메일 주소의 Google 계정으로 로그인하지 않은 멤버는 조직과 조직의 워크스페이스에 접근할 수 없습니다. 다른 방법으로 로그인한 상태에서 만든 API 키와 SSH 키도 차단됩니다.",
    description: "Org Settings page: what requiring Google login does",
  },
  "organizations.requireTwoFactorConfirmTitle": {
    message: "{organization}에서 2단계 인증을 요구할까요?",
    description:
      "Title of the confirmation before an Owner requires 2FA in the org",
  },
  "organizations.requireTwoFactorConfirm": {
    message:
      "2단계 인증을 사용하지 않는 멤버는 2단계 인증을 켜기 전까지 조직과 모든 워크스페이스에 접근할 수 없게 되며, API 키와 SSH 키도 더 이상 작동하지 않습니다. 각 워크스페이스의 자체 설정은 그대로 유지됩니다.",
    description:
      "Body of the confirmation before an Owner requires 2FA in the org",
  },
  "organizations.requireGoogleLoginConfirmTitle": {
    message: "{organization}에서 Google 로그인을 요구할까요?",
    description:
      "Title of the confirmation before an Owner requires Google login in the org",
  },
  "organizations.requireGoogleLoginConfirm": {
    message:
      "bex 이메일 주소의 Google 계정으로 로그인하지 않은 멤버는 그렇게 로그인하기 전까지 조직과 모든 워크스페이스에 접근할 수 없게 되며, 다른 방법으로 로그인한 상태에서 만든 API 키와 SSH 키도 더 이상 작동하지 않습니다. 각 워크스페이스의 자체 설정은 그대로 유지됩니다.",
    description:
      "Body of the confirmation before an Owner requires Google login in the org",
  },
  "organizations.requireConfirmAction": {
    message: "요구",
    description:
      "Confirm button of the require-2FA and require-Google-login confirmations",
  },
  "organizations.requireTwoFactorEnabledToast": {
    message: "이제 {name}의 모든 워크스페이스에서 2단계 인증이 필요합니다",
    description: "Toast after an Owner turns the org's 2FA requirement on",
  },
  "organizations.requireTwoFactorDisabledToast": {
    message: "{name}에서 더 이상 2단계 인증을 요구하지 않습니다",
    description: "Toast after an Owner turns the org's 2FA requirement off",
  },
  "organizations.requireGoogleLoginEnabledToast": {
    message: "이제 {name}의 모든 워크스페이스에서 Google 로그인이 필요합니다",
    description:
      "Toast after an Owner turns the org's Google login requirement on",
  },
  "organizations.requireGoogleLoginDisabledToast": {
    message: "{name}에서 더 이상 Google 로그인을 요구하지 않습니다",
    description:
      "Toast after an Owner turns the org's Google login requirement off",
  },
  "organizations.requireTwoFactorErrorOwnerNotEnrolled": {
    message:
      "이 조직에서 2단계 인증을 요구하기 전에 먼저 본인 계정의 2단계 인증을 켜세요. 그렇지 않으면 본인이 접근할 수 없게 됩니다.",
    description:
      "Refusal for code WORKSPACE_2FA_ADMIN_NOT_ENROLLED on the org setting: the acting Owner has no second factor",
  },
  "organizations.requireGoogleLoginErrorOwnerNotSignedIn": {
    message:
      "이 조직에서 Google 로그인을 요구하기 전에 먼저 bex 이메일 주소의 Google 계정으로 로그인하세요. 그렇지 않으면 본인이 접근할 수 없게 됩니다.",
    description:
      "Refusal for code WORKSPACE_GOOGLE_LOGIN_ADMIN_NOT_SIGNED_IN on the org setting: the acting Owner is not signed in with Google",
  },
  "organizations.auditNoPermission": {
    message: "이 작업을 수행할 권한이 없습니다.",
    description:
      "Org Settings → Audit Logs: what a Member or Guest reads in place of the CSV export (Render's text)",
  },
  "organizations.auditKeysTitle": {
    message: "감사 로그 API 키",
    description:
      "Org Settings → Audit Logs: the API keys section heading (Render's)",
  },
  "organizations.auditKeysDescription": {
    message:
      "이 조직과 모든 워크스페이스의 감사 로그를 읽기 전용으로 볼 수 있는 API 키를 만드세요.",
    description:
      "Org Settings → Audit Logs: the API keys section description (Render's)",
  },
  "organizations.auditKeysCreate": {
    message: "API 키 만들기",
    description:
      "Org Settings → Audit Logs: the button that opens the create dialog",
  },
  "organizations.auditKeysNoPermission": {
    message: "이 조직의 API 키를 관리할 권한이 없습니다.",
    description:
      "Org Settings → Audit Logs: what a Member or Guest reads under the disabled Create API key button (Render's text)",
  },
  "organizations.auditKeysLoading": {
    message: "API 키를 불러오는 중…",
    description: "Org Settings → Audit Logs: while the org's API keys load",
  },
  "organizations.auditKeysLoadFailed": {
    message: "조직의 API 키를 불러올 수 없습니다.",
    description:
      "Org Settings → Audit Logs: the org's API keys could not be read",
  },
  "organizations.auditKeysRetry": {
    message: "다시 시도",
    description:
      "Org Settings → Audit Logs: reload the org's API keys after a failure",
  },
  "organizations.auditKeysEmpty": {
    message: "발급된 API 키가 없습니다.",
    description: "Org Settings → Audit Logs: the org has no API keys",
  },
  "organizations.auditKeysColName": {
    message: "이름",
    description: "Org API keys table: the key's name column",
  },
  "organizations.auditKeysColKey": {
    message: "키",
    description:
      "Org API keys table: the key's id column, shown truncated as Render's key prefix",
  },
  "organizations.auditKeysColCreated": {
    message: "생성일",
    description: "Org API keys table: when the key was created",
  },
  "organizations.auditKeysColLastUsed": {
    message: "마지막 사용",
    description: "Org API keys table: when the key was last used",
  },
  "organizations.auditKeysNeverUsed": {
    message: "사용 안 함",
    description: "Org API keys table: a key that has not been used",
  },
  "organizations.auditKeysActions": {
    message: "{label} 작업",
    description: "Accessible name of an org API key row's actions menu",
  },
  "organizations.auditKeysRevoke": {
    message: "취소",
    description: "Org API key row menu: revoke the key",
  },
  "organizations.auditKeysRevokeTitle": {
    message: "API 키 취소",
    description: "Org API key revoke confirmation: title",
  },
  "organizations.auditKeysRevokeBody": {
    message: "“{label}” 키는 더 이상 조직의 감사 로그에 접근할 수 없습니다.",
    description:
      "Org API key revoke confirmation: what revoking does (Render's text)",
  },
  "organizations.auditKeysRevokeCancel": {
    message: "닫기",
    description: "Org API key revoke confirmation: cancel",
  },
  "organizations.auditKeysRevokeConfirm": {
    message: "API 키 취소",
    description: "Org API key revoke confirmation: confirm",
  },
  "organizations.auditKeysRevokeFailed": {
    message: "API 키를 취소할 수 없습니다. 다시 시도하세요.",
    description: "Org API key revoke failed",
  },
  "organizations.auditKeysCreateTitle": {
    message: "API 키 만들기",
    description: "Org API key create dialog: title before the key exists",
  },
  "organizations.auditKeysCreatedTitle": {
    message: "API 키가 만들어졌습니다",
    description: "Org API key create dialog: title once the key exists",
  },
  "organizations.auditKeysCreateHint": {
    message: "API 키에 “agent”나 “laptop”처럼 고유한 이름을 지정하세요.",
    description: "Org API key create dialog: the name hint (Render's text)",
  },
  "organizations.auditKeysNameLabel": {
    message: "API 키 이름",
    description: "Org API key create dialog: the name field's label",
  },
  "organizations.auditKeysNamePlaceholder": {
    message: "이름",
    description: "Org API key create dialog: the name field's placeholder",
  },
  "organizations.auditKeysGenerate": {
    message: "API 키 생성",
    description: "Org API key create dialog: submit",
  },
  "organizations.auditKeysClose": {
    message: "닫기",
    description: "Org API key create dialog: close before a key exists",
  },
  "organizations.auditKeysDone": {
    message: "완료",
    description: "Org API key create dialog: close once the key is shown",
  },
  "organizations.auditKeysSaveNow": {
    message: "지금 이 API 키를 저장하세요. 다시 볼 수 없습니다.",
    description:
      "Org API key create dialog: the key is shown only once (Render's text)",
  },
  "organizations.auditKeysKeyLabel": {
    message: "API 키",
    description: "Org API key create dialog: the label of the new key's field",
  },
  "organizations.auditKeysCopy": {
    message: "API 키 복사",
    description: "Org API key create dialog: copy the new key",
  },
  "organizations.auditKeysCopied": {
    message: "복사했습니다",
    description: "Org API key create dialog: the key was copied",
  },
  "organizations.auditKeysCopyFailed": {
    message: "키를 복사할 수 없습니다.",
    description: "Org API key create dialog: copying the key failed",
  },
  "organizations.auditKeysCreateFailed": {
    message: "API 키를 만들 수 없습니다. 다시 시도하세요.",
    description: "Org API key create failed",
  },
  "organizations.auditKeysTooMany": {
    message: "키를 너무 많이 만들었습니다. 나중에 다시 시도하세요.",
    description: "Org API key create refused: too many keys (Render's text)",
  },
  "organizations.memberActions": {
    message: "{identity} 작업",
    description: "Accessible name of an org member row's actions menu",
  },
  "organizations.viewDetails": {
    message: "세부 정보 보기",
    description: "Org member row menu: open the member's details",
  },
  "organizations.removeMemberItem": {
    message: "멤버 제거",
    description: "Org member row menu: remove the member from the org",
  },
  "organizations.detailsOwner": {
    message: "소유자",
    description: "Org member details: shown under an Org Owner's name",
  },
  "organizations.detailsWorkspaces": {
    message: "워크스페이스",
    description: "Org member details: the member's workspaces section heading",
  },
  "organizations.detailsNoWorkspaces": {
    message: "워크스페이스 없음",
    description: "Org member details: the member belongs to no workspace",
  },
  "organizations.detailsNoWorkspacesBody": {
    message: "이 멤버는 아직 워크스페이스에 초대되지 않았습니다.",
    description: "Org member details: the member belongs to no workspace",
  },
  "organizations.detailsLoginStatus": {
    message: "로그인 및 인증 상태",
    description:
      "Org member details: the login methods and 2FA section heading (Render's)",
  },
  "organizations.detailsLoginStatusLoading": {
    message: "로그인 상태를 불러오는 중…",
    description: "Org member details: while the member's login status loads",
  },
  "organizations.loginMethodPassword": {
    message: "비밀번호",
    description: "Org member details: the password login method",
  },
  "organizations.loginSignedIn": {
    message: "로그인함",
    description:
      "Org member details: the member has signed in with this method",
  },
  "organizations.loginNotSignedIn": {
    message: "로그인 안 함",
    description:
      "Org member details: the member has not signed in with this method",
  },
  "organizations.loginTwoFactor": {
    message: "2단계 인증",
    description: "Org member details: the 2FA row",
  },
  "organizations.loginTwoFactorEnabled": {
    message: "사용",
    description: "Org member details: the member has 2FA",
  },
  "organizations.loginTwoFactorNotEnabled": {
    message: "사용 안 함",
    description: "Org member details: the member has no 2FA",
  },
  "organizations.detailsClose": {
    message: "닫기",
    description: "Org member details: close the dialog",
  },
};

export default koOrganizations;
