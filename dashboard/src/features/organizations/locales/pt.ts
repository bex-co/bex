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
};

export default ptOrganizations;
