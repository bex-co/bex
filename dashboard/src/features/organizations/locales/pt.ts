import type { TranslationEntry } from "@/i18n";

const ptOrganizations: Record<string, TranslationEntry> = {
  "organizations.switcherOrganization": {
    message: "Organização",
    description:
      "Workspace switcher: the sublabel under an organization's name, which opens the org's Workspaces page",
  },
  "organizations.navLabel": {
    message: "Páginas da organização",
    description:
      "Accessible name of the org page navigation (Workspaces, Members, Settings)",
  },
  "organizations.navWorkspaces": {
    message: "Workspaces",
    description: "Org page navigation: the Workspaces page",
  },
  "organizations.navMembers": {
    message: "Membros",
    description: "Org page navigation: the Members page",
  },
  "organizations.navSettings": {
    message: "Configurações",
    description: "Org page navigation: the Settings page",
  },
  "organizations.untitled": {
    message: "Organização",
    description: "Org page title when the organization could not be read",
  },
  "organizations.workspacesTitle": {
    message: "Workspaces da organização",
    description: "Document title of the org's Workspaces page",
  },
  "organizations.membersTitle": {
    message: "Membros da organização",
    description: "Document title of the org's Members page",
  },
  "organizations.settingsTitle": {
    message: "Configurações da organização",
    description: "Document title of the org's Settings page",
  },
  "organizations.workspacesDescription": {
    message:
      "Uma organização comporta workspaces ilimitados, cada um no plano da organização, sem custo adicional.",
    description:
      "Org Workspaces page header description (Render's organizations doc)",
  },
  "organizations.membersDescription": {
    message:
      "Proprietários gerenciam a organização, membros entram nos workspaces dela e convidados acessam um único workspace.",
    description: "Org Members page header description",
  },
  "organizations.settingsDescription": {
    message: "O nome da organização e o log de auditoria dela.",
    description: "Org Settings page header description",
  },
  "organizations.roleOwner": {
    message: "Proprietário",
    description: "Org role: Owner (Render's Org Owner)",
  },
  "organizations.roleMember": {
    message: "Membro",
    description: "Org role: Member (a standard org member)",
  },
  "organizations.roleGuest": {
    message: "Convidado",
    description: "Org role: Guest (reaches a single workspace)",
  },
  "organizations.notFoundTitle": {
    message: "Organização não encontrada",
    description: "Org pages: ORGANIZATION_NOT_FOUND state title",
  },
  "organizations.notFoundBody": {
    message: "Ela não existe ou você não é um dos membros dela.",
    description: "Org pages: ORGANIZATION_NOT_FOUND state body",
  },
  "organizations.errorTitle": {
    message: "Não foi possível carregar a organização",
    description: "Org pages: read failure state title",
  },
  "organizations.errorBody": {
    message: "Tente novamente em instantes.",
    description: "Org pages: read failure state body",
  },

  "organizations.workspacesCardTitle": {
    message: "Workspaces",
    description: "Org Workspaces page: the workspace list card title",
  },
  "organizations.workspacesCardDescription": {
    message:
      "Qualquer membro pode entrar em um workspace público. Um workspace só por convite exige o convite de um dos administradores dele.",
    description: "Org Workspaces page: the workspace list card description",
  },
  "organizations.colWorkspace": {
    message: "Workspace",
    description: "Org workspace table column: the workspace name",
  },
  "organizations.colPrivacy": {
    message: "Privacidade",
    description: "Org workspace table column: Public or Invite-only",
  },
  "organizations.primary": {
    message: "Principal",
    description:
      "Badge on the org's primary workspace (the one that holds the org's plan and billing)",
  },
  "organizations.workspacesEmptyTitle": {
    message: "Nenhum workspace para mostrar",
    description: "Org Workspaces page: empty list title",
  },
  "organizations.workspacesEmptyBody": {
    message: "Os workspaces que você pode ver nesta organização aparecem aqui.",
    description: "Org Workspaces page: empty list body",
  },
  "organizations.open": {
    message: "Abrir",
    description:
      "Org workspace row action: switch to a workspace the caller belongs to",
  },
  "organizations.join": {
    message: "Entrar",
    description:
      "Org workspace row action: a Member or Owner adds themselves to a Public workspace (Developer role)",
  },
  "organizations.joinAsAdmin": {
    message: "Entrar como administrador",
    description:
      "Org workspace row action: an Owner adds themselves to any org workspace as admin",
  },
  "organizations.joined": {
    message: "Você entrou em {workspace}.",
    description: "Toast after joining a Public workspace",
  },
  "organizations.joinedAsAdmin": {
    message: "Agora você é administrador de {workspace}.",
    description: "Toast after an Owner joins a workspace as admin",
  },
  "organizations.joinFailed": {
    message: "Não foi possível entrar em {workspace}.",
    description: "Toast when joining a workspace failed for no stated reason",
  },
  "organizations.newWorkspace": {
    message: "Novo workspace",
    description:
      "Org Workspaces page: the Owner's button that opens the create dialog (Render: + New Workspace)",
  },
  "organizations.createTitle": {
    message: "Novo workspace em {organization}",
    description: "Create org workspace dialog title",
  },
  "organizations.createDescription": {
    message:
      "Você será o primeiro administrador e poderá começar a criar serviços na hora.",
    description: "Create org workspace dialog description",
  },
  "organizations.fieldName": {
    message: "Nome",
    description: "Create workspace / rename organization: the name field label",
  },
  "organizations.fieldPrivacy": {
    message: "Privacidade",
    description: "Create workspace dialog: the privacy choice's legend",
  },
  "organizations.planIncluded": {
    message: "Incluído no plano da sua organização",
    description:
      "Create workspace dialog: the new workspace takes the org's plan, no price and no payment step",
  },
  "organizations.planIncludedDetail": {
    message: "Plano {plan}, sem custo adicional",
    description:
      "Create workspace dialog: the org's plan name under 'Included in your organization's plan'",
  },
  "organizations.createSubmit": {
    message: "Criar workspace",
    description: "Create org workspace dialog: submit button (Render's label)",
  },
  "organizations.cancel": {
    message: "Cancelar",
    description: "Org dialogs: cancel button",
  },
  "organizations.workspaceCreated": {
    message: "{workspace} foi criado.",
    description: "Toast after creating an org workspace",
  },
  "organizations.createFailed": {
    message: "Não foi possível criar o workspace.",
    description: "Create org workspace: failure with no stated reason",
  },

  "organizations.membersCardTitle": {
    message: "Membros",
    description: "Org Members page: the member list card title",
  },
  "organizations.membersCardDescription": {
    message:
      "Todas as pessoas da organização e a função de cada uma nela. As funções de workspace são definidas em cada workspace.",
    description: "Org Members page: the member list card description",
  },
  "organizations.colMember": {
    message: "Membro",
    description: "Org member table column: the person",
  },
  "organizations.colRole": {
    message: "Função",
    description: "Org member table column: the org role",
  },
  "organizations.guestWorkspace": {
    message: "Workspace: {workspace}",
    description: "Org member table: the one workspace a Guest belongs to",
  },
  "organizations.guestNoWorkspace": {
    message: "Ainda sem workspace",
    description: "Org member table: a Guest who belongs to no workspace",
  },
  "organizations.roleFor": {
    message: "Função na organização de {identity}",
    description: "Accessible name of an org member's role select",
  },
  "organizations.removeMember": {
    message: "Remover {identity} da organização",
    description: "Accessible name of an org member row's remove button",
  },
  "organizations.removeTitle": {
    message: "Remover da organização?",
    description: "Org member remove confirmation title",
  },
  "organizations.removeConfirm": {
    message:
      "{identity} perde o acesso à organização e a todos os workspaces dela, e as chaves de API dessa pessoa nesses workspaces são revogadas.",
    description: "Org member remove confirmation description",
  },
  "organizations.remove": {
    message: "Remover",
    description: "Org member remove confirmation: confirm button",
  },
  "organizations.removed": {
    message: "{identity} foi removido da organização.",
    description: "Toast after removing an org member",
  },
  "organizations.removeFailed": {
    message: "Não foi possível remover {identity}.",
    description: "Org member removal failure with no stated reason",
  },
  "organizations.roleChanged": {
    message: "Agora {identity} é {role}.",
    description: "Toast after changing an org member's role",
  },
  "organizations.roleChangeFailed": {
    message: "Não foi possível alterar a função.",
    description: "Org role change failure with no stated reason",
  },
  "organizations.membersForbiddenTitle": {
    message: "Convidados não veem os membros",
    description:
      "Org Members page: a Guest's state (the API answers FORBIDDEN)",
  },
  "organizations.membersForbiddenBody": {
    message:
      "Só proprietários e membros da organização podem ver quem faz parte dela.",
    description: "Org Members page: a Guest's state body",
  },
  "organizations.membersErrorTitle": {
    message: "Não foi possível carregar os membros",
    description: "Org Members page: read failure title",
  },
  "organizations.invite": {
    message: "Convidar",
    description: "Org Members page: the Owner's invite button",
  },
  "organizations.inviteTitle": {
    message: "Convidar para a organização",
    description: "Org invite dialog title",
  },
  "organizations.inviteDescription": {
    message:
      "A pessoa recebe um e-mail com um link para entrar como proprietário ou membro. Os administradores de cada workspace convidam pessoas para os próprios workspaces.",
    description: "Org invite dialog description",
  },
  "organizations.fieldEmail": {
    message: "Endereço de e-mail",
    description: "Org invite dialog: email field label",
  },
  "organizations.fieldEmailInvalid": {
    message: "Digite um endereço de e-mail válido.",
    description: "Org invite dialog: malformed email",
  },
  "organizations.fieldRole": {
    message: "Função",
    description: "Org invite dialog: role field label",
  },
  "organizations.inviteSubmit": {
    message: "Enviar convite",
    description: "Org invite dialog: submit button",
  },
  "organizations.invited": {
    message: "{email} foi convidado.",
    description: "Toast after sending an org invite",
  },
  "organizations.inviteFailed": {
    message: "Não foi possível enviar o convite.",
    description: "Org invite failure with no stated reason",
  },
  "organizations.pendingTitle": {
    message: "Convites pendentes",
    description: "Org Members page: the pending invite list heading",
  },
  "organizations.colEmail": {
    message: "E-mail",
    description: "Org pending invite table column: the invitee's email",
  },
  "organizations.revokeInvite": {
    message: "Revogar",
    description: "Org pending invite row action",
  },
  "organizations.inviteRevoked": {
    message: "O convite para {email} foi revogado.",
    description: "Toast after revoking an org invite",
  },
  "organizations.revokeFailed": {
    message: "Não foi possível revogar o convite.",
    description: "Org invite revoke failure with no stated reason",
  },

  "organizations.nameTitle": {
    message: "Nome da organização",
    description: "Org Settings page: the name card title",
  },
  "organizations.nameDescription": {
    message:
      "Aparece no menu de workspaces para todas as pessoas da organização.",
    description: "Org Settings page: the name card description",
  },
  "organizations.nameSave": {
    message: "Salvar",
    description: "Org Settings page: save the new name",
  },
  "organizations.nameSaved": {
    message: "A organização agora se chama {name}.",
    description: "Toast after renaming the org",
  },
  "organizations.renameFailed": {
    message: "Não foi possível renomear a organização.",
    description: "Org rename failure with no stated reason",
  },
  "organizations.nameOwnerOnly": {
    message: "Só proprietários podem renomear a organização.",
    description:
      "Org Settings page: why the name is read-only for a Member or Guest",
  },
  "organizations.auditTitle": {
    message: "Logs de auditoria",
    description:
      "Org Settings page: the audit log section title (Render's label)",
  },
  "organizations.auditDescription": {
    message:
      "Exporte como CSV os eventos da própria organização, como membros, workspaces e configurações. Eles não estão no log de nenhum workspace, e cada exportação cobre no máximo {days} dias.",
    description: "Org Settings page: the audit log section description",
  },

  "organizations.errorNotFound": {
    message: "Esta organização não existe ou você não é um dos membros dela.",
    description: "ORGANIZATION_NOT_FOUND refusal",
  },
  "organizations.errorForbidden": {
    message: "Você não tem permissão para fazer isso nesta organização.",
    description: "FORBIDDEN refusal of an org action",
  },
  "organizations.errorLastOwner": {
    message:
      "Uma organização precisa de pelo menos um proprietário. Torne outra pessoa proprietária primeiro.",
    description: "ORGANIZATION_LAST_OWNER refusal",
  },
  "organizations.errorMemberLastAdmin": {
    message:
      "Essa pessoa é a última administradora de {workspace}. Torne outra pessoa administradora primeiro.",
    description: "ORGANIZATION_MEMBER_LAST_ADMIN refusal, naming the workspace",
  },
  "organizations.errorAlreadyMember": {
    message: "Essa pessoa já está na organização.",
    description: "ORGANIZATION_ALREADY_MEMBER refusal of an org invite",
  },
  "organizations.errorGuestSingleWorkspace": {
    message:
      "Um convidado pertence a um único workspace da organização. Torne essa pessoa membro para dar acesso a mais.",
    description: "ORGANIZATION_GUEST_SINGLE_WORKSPACE refusal",
  },
  "organizations.errorOwnRole": {
    message:
      "Você não pode alterar a sua própria função na organização. Peça a outro proprietário para alterá-la.",
    description: "CANNOT_CHANGE_OWN_ROLE refusal",
  },
  "organizations.errorRemoveSelf": {
    message: "Você não pode remover a si mesmo da organização.",
    description: "CANNOT_REMOVE_SELF refusal",
  },
  "organizations.errorInvalidPrivacy": {
    message: "Escolha Público ou Só por convite.",
    description: "INVALID_PRIVACY refusal",
  },
  "organizations.securityTitle": {
    message: "Segurança",
    description:
      "Org Settings page: title of the login requirements card (Render's section name, where it puts Require Login Method; w7/m466)",
  },
  "organizations.securityDescription": {
    message:
      "Como os membros devem entrar, em todos os workspaces da organização e nas páginas dela, além das configurações de cada workspace.",
    description: "Org Settings page: what the Security card's settings do",
  },
  "organizations.securityOwnerOnly": {
    message: "Só proprietários podem alterar estas configurações.",
    description:
      "Org Settings page: why the Security settings are read-only for a Member or Guest",
  },
  "organizations.requireTwoFactorLabel": {
    message: "Exigir autenticação em duas etapas",
    description:
      "Org Settings page: label of the switch that requires 2FA in every workspace of the org",
  },
  "organizations.requireTwoFactorDescription": {
    message:
      "Membros sem autenticação em duas etapas (2FA) não podem acessar a organização nem nenhum dos workspaces dela até ativá-la. As chaves de API e chaves SSH deles também ficam bloqueadas até lá.",
    description: "Org Settings page: what requiring 2FA does",
  },
  "organizations.requireGoogleLoginLabel": {
    message: "Exigir login com Google",
    description:
      "Org Settings page: label of the switch that requires signing in with Google in every workspace of the org",
  },
  "organizations.requireGoogleLoginDescription": {
    message:
      "Membros que não entram com a conta Google do seu e-mail do bex não podem acessar a organização nem nenhum dos workspaces dela. As chaves de API e as chaves SSH criadas com outro login também ficam bloqueadas.",
    description: "Org Settings page: what requiring Google login does",
  },
  "organizations.requireTwoFactorConfirmTitle": {
    message: "Exigir autenticação em duas etapas em {organization}?",
    description:
      "Title of the confirmation before an Owner requires 2FA in the org",
  },
  "organizations.requireTwoFactorConfirm": {
    message:
      "Membros sem autenticação em duas etapas perdem o acesso à organização e a todos os workspaces dela até ativá-la, e as chaves de API e chaves SSH deles deixam de funcionar com eles. A configuração própria de cada workspace continua como está.",
    description:
      "Body of the confirmation before an Owner requires 2FA in the org",
  },
  "organizations.requireGoogleLoginConfirmTitle": {
    message: "Exigir login com Google em {organization}?",
    description:
      "Title of the confirmation before an Owner requires Google login in the org",
  },
  "organizations.requireGoogleLoginConfirm": {
    message:
      "Membros que não entraram com a conta Google do seu e-mail do bex perdem o acesso à organização e a todos os workspaces dela até entrarem assim, e as chaves de API e chaves SSH criadas com outro login deixam de funcionar com eles. A configuração própria de cada workspace continua como está.",
    description:
      "Body of the confirmation before an Owner requires Google login in the org",
  },
  "organizations.requireConfirmAction": {
    message: "Exigir",
    description:
      "Confirm button of the require-2FA and require-Google-login confirmations",
  },
  "organizations.requireTwoFactorEnabledToast": {
    message:
      "A autenticação em duas etapas agora é exigida em todos os workspaces de {name}",
    description: "Toast after an Owner turns the org's 2FA requirement on",
  },
  "organizations.requireTwoFactorDisabledToast": {
    message: "{name} não exige mais autenticação em duas etapas",
    description: "Toast after an Owner turns the org's 2FA requirement off",
  },
  "organizations.requireGoogleLoginEnabledToast": {
    message:
      "O login com Google agora é exigido em todos os workspaces de {name}",
    description:
      "Toast after an Owner turns the org's Google login requirement on",
  },
  "organizations.requireGoogleLoginDisabledToast": {
    message: "{name} não exige mais login com Google",
    description:
      "Toast after an Owner turns the org's Google login requirement off",
  },
  "organizations.requireTwoFactorErrorOwnerNotEnrolled": {
    message:
      "Ative a autenticação em duas etapas na sua própria conta antes de exigi-la nesta organização, ou você ficaria sem acesso.",
    description:
      "Refusal for code WORKSPACE_2FA_ADMIN_NOT_ENROLLED on the org setting: the acting Owner has no second factor",
  },
  "organizations.requireGoogleLoginErrorOwnerNotSignedIn": {
    message:
      "Entre com a conta Google do seu e-mail do bex antes de exigir o login com Google nesta organização, ou você ficaria sem acesso.",
    description:
      "Refusal for code WORKSPACE_GOOGLE_LOGIN_ADMIN_NOT_SIGNED_IN on the org setting: the acting Owner is not signed in with Google",
  },
};

export default ptOrganizations;
