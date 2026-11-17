import type { TranslationEntry } from "@/i18n";

const ruOrganizations: Record<string, TranslationEntry> = {
  "organizations.switcherOrganization": {
    message: "Организация",
    description:
      "Workspace switcher: the sublabel under an organization's name, which opens the org's Workspaces page",
  },
  "organizations.navLabel": {
    message: "Страницы организации",
    description:
      "Accessible name of the org page navigation (Workspaces, Members, Settings)",
  },
  "organizations.navWorkspaces": {
    message: "Рабочие пространства",
    description: "Org page navigation: the Workspaces page",
  },
  "organizations.navMembers": {
    message: "Участники",
    description: "Org page navigation: the Members page",
  },
  "organizations.navSettings": {
    message: "Настройки",
    description: "Org page navigation: the Settings page",
  },
  "organizations.untitled": {
    message: "Организация",
    description: "Org page title when the organization could not be read",
  },
  "organizations.workspacesTitle": {
    message: "Рабочие пространства организации",
    description: "Document title of the org's Workspaces page",
  },
  "organizations.membersTitle": {
    message: "Участники организации",
    description: "Document title of the org's Members page",
  },
  "organizations.settingsTitle": {
    message: "Настройки организации",
    description: "Document title of the org's Settings page",
  },
  "organizations.workspacesDescription": {
    message:
      "В организации может быть сколько угодно рабочих пространств, и каждое работает на тарифе организации без доплаты.",
    description:
      "Org Workspaces page header description (Render's organizations doc)",
  },
  "organizations.membersDescription": {
    message:
      "Владельцы управляют организацией, участники присоединяются к её рабочим пространствам, а гости имеют доступ только к одному рабочему пространству.",
    description: "Org Members page header description",
  },
  "organizations.settingsDescription": {
    message: "Название организации и её журнал аудита.",
    description: "Org Settings page header description",
  },
  "organizations.roleOwner": {
    message: "Владелец",
    description: "Org role: Owner (Render's Org Owner)",
  },
  "organizations.roleMember": {
    message: "Участник",
    description: "Org role: Member (a standard org member)",
  },
  "organizations.roleGuest": {
    message: "Гость",
    description: "Org role: Guest (reaches a single workspace)",
  },
  "organizations.notFoundTitle": {
    message: "Организация не найдена",
    description: "Org pages: ORGANIZATION_NOT_FOUND state title",
  },
  "organizations.notFoundBody": {
    message: "Её не существует, или вы не состоите в ней.",
    description: "Org pages: ORGANIZATION_NOT_FOUND state body",
  },
  "organizations.errorTitle": {
    message: "Не удалось загрузить организацию",
    description: "Org pages: read failure state title",
  },
  "organizations.errorBody": {
    message: "Попробуйте ещё раз через минуту.",
    description: "Org pages: read failure state body",
  },

  "organizations.workspacesCardTitle": {
    message: "Рабочие пространства",
    description: "Org Workspaces page: the workspace list card title",
  },
  "organizations.workspacesCardDescription": {
    message:
      "К публичному рабочему пространству может присоединиться любой участник. Для пространства только по приглашению нужно приглашение одного из его администраторов.",
    description: "Org Workspaces page: the workspace list card description",
  },
  "organizations.colWorkspace": {
    message: "Рабочее пространство",
    description: "Org workspace table column: the workspace name",
  },
  "organizations.colPrivacy": {
    message: "Доступ",
    description: "Org workspace table column: Public or Invite-only",
  },
  "organizations.primary": {
    message: "Основное",
    description:
      "Badge on the org's primary workspace (the one that holds the org's plan and billing)",
  },
  "organizations.workspacesEmptyTitle": {
    message: "Нет рабочих пространств для показа",
    description: "Org Workspaces page: empty list title",
  },
  "organizations.workspacesEmptyBody": {
    message:
      "Здесь появятся рабочие пространства этой организации, которые вам доступны.",
    description: "Org Workspaces page: empty list body",
  },
  "organizations.open": {
    message: "Открыть",
    description:
      "Org workspace row action: switch to a workspace the caller belongs to",
  },
  "organizations.join": {
    message: "Присоединиться",
    description:
      "Org workspace row action: a Member or Owner adds themselves to a Public workspace (Developer role)",
  },
  "organizations.joinAsAdmin": {
    message: "Присоединиться как администратор",
    description:
      "Org workspace row action: an Owner adds themselves to any org workspace as admin",
  },
  "organizations.joined": {
    message: "Вы присоединились к {workspace}.",
    description: "Toast after joining a Public workspace",
  },
  "organizations.joinedAsAdmin": {
    message: "Теперь вы администратор {workspace}.",
    description: "Toast after an Owner joins a workspace as admin",
  },
  "organizations.joinFailed": {
    message: "Не удалось присоединиться к {workspace}.",
    description: "Toast when joining a workspace failed for no stated reason",
  },
  "organizations.newWorkspace": {
    message: "Новое рабочее пространство",
    description:
      "Org Workspaces page: the Owner's button that opens the create dialog (Render: + New Workspace)",
  },
  "organizations.createTitle": {
    message: "Новое рабочее пространство в {organization}",
    description: "Create org workspace dialog title",
  },
  "organizations.createDescription": {
    message:
      "Вы станете его первым администратором и сможете сразу создавать сервисы.",
    description: "Create org workspace dialog description",
  },
  "organizations.fieldName": {
    message: "Название",
    description: "Create workspace / rename organization: the name field label",
  },
  "organizations.fieldPrivacy": {
    message: "Доступ",
    description: "Create workspace dialog: the privacy choice's legend",
  },
  "organizations.planIncluded": {
    message: "Входит в тариф вашей организации",
    description:
      "Create workspace dialog: the new workspace takes the org's plan, no price and no payment step",
  },
  "organizations.planIncludedDetail": {
    message: "Тариф {plan}, без доплаты",
    description:
      "Create workspace dialog: the org's plan name under 'Included in your organization's plan'",
  },
  "organizations.createSubmit": {
    message: "Создать рабочее пространство",
    description: "Create org workspace dialog: submit button (Render's label)",
  },
  "organizations.cancel": {
    message: "Отмена",
    description: "Org dialogs: cancel button",
  },
  "organizations.workspaceCreated": {
    message: "Рабочее пространство {workspace} создано.",
    description: "Toast after creating an org workspace",
  },
  "organizations.createFailed": {
    message: "Не удалось создать рабочее пространство.",
    description: "Create org workspace: failure with no stated reason",
  },

  "organizations.membersCardTitle": {
    message: "Участники",
    description: "Org Members page: the member list card title",
  },
  "organizations.membersCardDescription": {
    message:
      "Все, кто состоит в организации, и их роль в ней. Роли в рабочих пространствах задаются в каждом пространстве.",
    description: "Org Members page: the member list card description",
  },
  "organizations.colMember": {
    message: "Участник",
    description: "Org member table column: the person",
  },
  "organizations.colRole": {
    message: "Роль",
    description: "Org member table column: the org role",
  },
  "organizations.guestWorkspace": {
    message: "Рабочее пространство: {workspace}",
    description: "Org member table: the one workspace a Guest belongs to",
  },
  "organizations.guestNoWorkspace": {
    message: "Пока не в рабочем пространстве",
    description: "Org member table: a Guest who belongs to no workspace",
  },
  "organizations.roleFor": {
    message: "Роль {identity} в организации",
    description: "Accessible name of an org member's role select",
  },
  "organizations.removeMember": {
    message: "Удалить {identity} из организации",
    description: "Accessible name of an org member row's remove button",
  },
  "organizations.removeTitle": {
    message: "Удалить из организации?",
    description: "Org member remove confirmation title",
  },
  "organizations.removeConfirm": {
    message:
      "{identity} потеряет доступ к организации и ко всем её рабочим пространствам, а его API-ключи в этих пространствах будут отозваны.",
    description: "Org member remove confirmation description",
  },
  "organizations.remove": {
    message: "Удалить",
    description: "Org member remove confirmation: confirm button",
  },
  "organizations.removed": {
    message: "{identity} удалён из организации.",
    description: "Toast after removing an org member",
  },
  "organizations.removeFailed": {
    message: "Не удалось удалить {identity}.",
    description: "Org member removal failure with no stated reason",
  },
  "organizations.roleChanged": {
    message: "Новая роль {identity}: {role}.",
    description: "Toast after changing an org member's role",
  },
  "organizations.roleChangeFailed": {
    message: "Не удалось изменить роль.",
    description: "Org role change failure with no stated reason",
  },
  "organizations.membersForbiddenTitle": {
    message: "Гости не видят участников",
    description:
      "Org Members page: a Guest's state (the API answers FORBIDDEN)",
  },
  "organizations.membersForbiddenBody": {
    message:
      "Видеть, кто состоит в организации, могут только её владельцы и участники.",
    description: "Org Members page: a Guest's state body",
  },
  "organizations.membersErrorTitle": {
    message: "Не удалось загрузить участников",
    description: "Org Members page: read failure title",
  },
  "organizations.invite": {
    message: "Пригласить",
    description: "Org Members page: the Owner's invite button",
  },
  "organizations.inviteTitle": {
    message: "Пригласить в организацию",
    description: "Org invite dialog title",
  },
  "organizations.inviteDescription": {
    message:
      "Человек получит письмо со ссылкой, чтобы присоединиться как владелец или участник. Администраторы рабочих пространств приглашают людей в свои пространства сами.",
    description: "Org invite dialog description",
  },
  "organizations.fieldEmail": {
    message: "Адрес email",
    description: "Org invite dialog: email field label",
  },
  "organizations.fieldEmailInvalid": {
    message: "Введите корректный адрес email.",
    description: "Org invite dialog: malformed email",
  },
  "organizations.fieldRole": {
    message: "Роль",
    description: "Org invite dialog: role field label",
  },
  "organizations.inviteSubmit": {
    message: "Отправить приглашение",
    description: "Org invite dialog: submit button",
  },
  "organizations.invited": {
    message: "Приглашение отправлено на {email}.",
    description: "Toast after sending an org invite",
  },
  "organizations.inviteFailed": {
    message: "Не удалось отправить приглашение.",
    description: "Org invite failure with no stated reason",
  },
  "organizations.pendingTitle": {
    message: "Ожидающие приглашения",
    description: "Org Members page: the pending invite list heading",
  },
  "organizations.colEmail": {
    message: "Почта",
    description: "Org pending invite table column: the invitee's email",
  },
  "organizations.revokeInvite": {
    message: "Отозвать",
    description: "Org pending invite row action",
  },
  "organizations.inviteRevoked": {
    message: "Приглашение для {email} отозвано.",
    description: "Toast after revoking an org invite",
  },
  "organizations.revokeFailed": {
    message: "Не удалось отозвать приглашение.",
    description: "Org invite revoke failure with no stated reason",
  },

  "organizations.nameTitle": {
    message: "Название организации",
    description: "Org Settings page: the name card title",
  },
  "organizations.nameDescription": {
    message:
      "Отображается в списке рабочих пространств у всех участников организации.",
    description: "Org Settings page: the name card description",
  },
  "organizations.nameSave": {
    message: "Сохранить",
    description: "Org Settings page: save the new name",
  },
  "organizations.nameSaved": {
    message: "Организация переименована в {name}.",
    description: "Toast after renaming the org",
  },
  "organizations.renameFailed": {
    message: "Не удалось переименовать организацию.",
    description: "Org rename failure with no stated reason",
  },
  "organizations.nameOwnerOnly": {
    message: "Переименовать организацию могут только владельцы.",
    description:
      "Org Settings page: why the name is read-only for a Member or Guest",
  },
  "organizations.auditTitle": {
    message: "Журналы аудита",
    description:
      "Org Settings page: the audit log section title (Render's label)",
  },
  "organizations.auditDescription": {
    message:
      "Экспортируйте в CSV события самой организации: участники, рабочие пространства, настройки. Их нет в журнале ни одного рабочего пространства, а один экспорт охватывает не более {days} дней.",
    description: "Org Settings page: the audit log section description",
  },

  "organizations.errorNotFound": {
    message: "Этой организации не существует, или вы не состоите в ней.",
    description: "ORGANIZATION_NOT_FOUND refusal",
  },
  "organizations.errorForbidden": {
    message: "У вас нет прав на это действие в этой организации.",
    description: "FORBIDDEN refusal of an org action",
  },
  "organizations.errorLastOwner": {
    message:
      "У организации должен быть хотя бы один владелец. Сначала сделайте владельцем кого-то другого.",
    description: "ORGANIZATION_LAST_OWNER refusal",
  },
  "organizations.errorMemberLastAdmin": {
    message:
      "Это последний администратор {workspace}. Сначала назначьте администратором кого-то другого.",
    description: "ORGANIZATION_MEMBER_LAST_ADMIN refusal, naming the workspace",
  },
  "organizations.errorAlreadyMember": {
    message: "Этот человек уже состоит в организации.",
    description: "ORGANIZATION_ALREADY_MEMBER refusal of an org invite",
  },
  "organizations.errorGuestSingleWorkspace": {
    message:
      "Гость состоит только в одном рабочем пространстве организации. Сделайте его участником, чтобы дать доступ к другим.",
    description: "ORGANIZATION_GUEST_SINGLE_WORKSPACE refusal",
  },
  "organizations.errorOwnRole": {
    message:
      "Вы не можете изменить свою роль в организации. Попросите другого владельца.",
    description: "CANNOT_CHANGE_OWN_ROLE refusal",
  },
  "organizations.errorRemoveSelf": {
    message: "Вы не можете удалить себя из организации.",
    description: "CANNOT_REMOVE_SELF refusal",
  },
  "organizations.errorInvalidPrivacy": {
    message: "Выберите «Публичное» или «Только по приглашению».",
    description: "INVALID_PRIVACY refusal",
  },
};

export default ruOrganizations;
