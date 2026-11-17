import type { TranslationEntry } from "@/i18n";

const esOrganizations: Record<string, TranslationEntry> = {
  "organizations.switcherOrganization": {
    message: "Organización",
    description:
      "Workspace switcher: the sublabel under an organization's name, which opens the org's Workspaces page",
  },
  "organizations.navLabel": {
    message: "Páginas de la organización",
    description:
      "Accessible name of the org page navigation (Workspaces, Members, Settings)",
  },
  "organizations.navWorkspaces": {
    message: "Espacios de trabajo",
    description: "Org page navigation: the Workspaces page",
  },
  "organizations.navMembers": {
    message: "Miembros",
    description: "Org page navigation: the Members page",
  },
  "organizations.navSettings": {
    message: "Configuración",
    description: "Org page navigation: the Settings page",
  },
  "organizations.untitled": {
    message: "Organización",
    description: "Org page title when the organization could not be read",
  },
  "organizations.workspacesTitle": {
    message: "Espacios de trabajo de la organización",
    description: "Document title of the org's Workspaces page",
  },
  "organizations.membersTitle": {
    message: "Miembros de la organización",
    description: "Document title of the org's Members page",
  },
  "organizations.settingsTitle": {
    message: "Configuración de la organización",
    description: "Document title of the org's Settings page",
  },
  "organizations.workspacesDescription": {
    message:
      "Una organización admite espacios de trabajo ilimitados, cada uno con el plan de la organización y sin costo adicional.",
    description:
      "Org Workspaces page header description (Render's organizations doc)",
  },
  "organizations.membersDescription": {
    message:
      "Los propietarios administran la organización, los miembros se unen a sus espacios de trabajo y los invitados acceden a un solo espacio de trabajo.",
    description: "Org Members page header description",
  },
  "organizations.settingsDescription": {
    message: "El nombre de la organización y su registro de auditoría.",
    description: "Org Settings page header description",
  },
  "organizations.roleOwner": {
    message: "Propietario",
    description: "Org role: Owner (Render's Org Owner)",
  },
  "organizations.roleMember": {
    message: "Miembro",
    description: "Org role: Member (a standard org member)",
  },
  "organizations.roleGuest": {
    message: "Invitado",
    description: "Org role: Guest (reaches a single workspace)",
  },
  "organizations.notFoundTitle": {
    message: "No se encontró la organización",
    description: "Org pages: ORGANIZATION_NOT_FOUND state title",
  },
  "organizations.notFoundBody": {
    message: "No existe o no eres uno de sus miembros.",
    description: "Org pages: ORGANIZATION_NOT_FOUND state body",
  },
  "organizations.errorTitle": {
    message: "No se pudo cargar la organización",
    description: "Org pages: read failure state title",
  },
  "organizations.errorBody": {
    message: "Vuelve a intentarlo en un momento.",
    description: "Org pages: read failure state body",
  },

  "organizations.workspacesCardTitle": {
    message: "Espacios de trabajo",
    description: "Org Workspaces page: the workspace list card title",
  },
  "organizations.workspacesCardDescription": {
    message:
      "Cualquier miembro puede unirse a un espacio de trabajo público. Uno solo por invitación requiere la invitación de uno de sus administradores.",
    description: "Org Workspaces page: the workspace list card description",
  },
  "organizations.colWorkspace": {
    message: "Espacio de trabajo",
    description: "Org workspace table column: the workspace name",
  },
  "organizations.colPrivacy": {
    message: "Privacidad",
    description: "Org workspace table column: Public or Invite-only",
  },
  "organizations.primary": {
    message: "Principal",
    description:
      "Badge on the org's primary workspace (the one that holds the org's plan and billing)",
  },
  "organizations.workspacesEmptyTitle": {
    message: "No hay espacios de trabajo que mostrar",
    description: "Org Workspaces page: empty list title",
  },
  "organizations.workspacesEmptyBody": {
    message:
      "Aquí aparecen los espacios de trabajo que puedes ver en esta organización.",
    description: "Org Workspaces page: empty list body",
  },
  "organizations.open": {
    message: "Abrir",
    description:
      "Org workspace row action: switch to a workspace the caller belongs to",
  },
  "organizations.join": {
    message: "Unirse",
    description:
      "Org workspace row action: a Member or Owner adds themselves to a Public workspace (Developer role)",
  },
  "organizations.joinAsAdmin": {
    message: "Unirse como administrador",
    description:
      "Org workspace row action: an Owner adds themselves to any org workspace as admin",
  },
  "organizations.joined": {
    message: "Te uniste a {workspace}.",
    description: "Toast after joining a Public workspace",
  },
  "organizations.joinedAsAdmin": {
    message: "Ahora eres administrador de {workspace}.",
    description: "Toast after an Owner joins a workspace as admin",
  },
  "organizations.joinFailed": {
    message: "No se pudo unir a {workspace}.",
    description: "Toast when joining a workspace failed for no stated reason",
  },
  "organizations.newWorkspace": {
    message: "Nuevo espacio de trabajo",
    description:
      "Org Workspaces page: the Owner's button that opens the create dialog (Render: + New Workspace)",
  },
  "organizations.createTitle": {
    message: "Nuevo espacio de trabajo en {organization}",
    description: "Create org workspace dialog title",
  },
  "organizations.createDescription": {
    message:
      "Serás su primer administrador y podrás empezar a crear servicios de inmediato.",
    description: "Create org workspace dialog description",
  },
  "organizations.fieldName": {
    message: "Nombre",
    description: "Create workspace / rename organization: the name field label",
  },
  "organizations.fieldPrivacy": {
    message: "Privacidad",
    description: "Create workspace dialog: the privacy choice's legend",
  },
  "organizations.planIncluded": {
    message: "Incluido en el plan de tu organización",
    description:
      "Create workspace dialog: the new workspace takes the org's plan, no price and no payment step",
  },
  "organizations.planIncludedDetail": {
    message: "Plan {plan}, sin costo adicional",
    description:
      "Create workspace dialog: the org's plan name under 'Included in your organization's plan'",
  },
  "organizations.createSubmit": {
    message: "Crear espacio de trabajo",
    description: "Create org workspace dialog: submit button (Render's label)",
  },
  "organizations.cancel": {
    message: "Cancelar",
    description: "Org dialogs: cancel button",
  },
  "organizations.workspaceCreated": {
    message: "Se creó {workspace}.",
    description: "Toast after creating an org workspace",
  },
  "organizations.createFailed": {
    message: "No se pudo crear el espacio de trabajo.",
    description: "Create org workspace: failure with no stated reason",
  },

  "organizations.membersCardTitle": {
    message: "Miembros",
    description: "Org Members page: the member list card title",
  },
  "organizations.membersCardDescription": {
    message:
      "Todas las personas de la organización y su rol en ella. Los roles de cada espacio de trabajo se definen en ese espacio.",
    description: "Org Members page: the member list card description",
  },
  "organizations.colMember": {
    message: "Miembro",
    description: "Org member table column: the person",
  },
  "organizations.colRole": {
    message: "Rol",
    description: "Org member table column: the org role",
  },
  "organizations.guestWorkspace": {
    message: "Espacio de trabajo: {workspace}",
    description: "Org member table: the one workspace a Guest belongs to",
  },
  "organizations.guestNoWorkspace": {
    message: "Aún sin espacio de trabajo",
    description: "Org member table: a Guest who belongs to no workspace",
  },
  "organizations.roleFor": {
    message: "Rol en la organización de {identity}",
    description: "Accessible name of an org member's role select",
  },
  "organizations.removeMember": {
    message: "Quitar a {identity} de la organización",
    description: "Accessible name of an org member row's remove button",
  },
  "organizations.removeTitle": {
    message: "¿Quitar de la organización?",
    description: "Org member remove confirmation title",
  },
  "organizations.removeConfirm": {
    message:
      "{identity} pierde el acceso a la organización y a todos sus espacios de trabajo, y se revocan sus claves de API en esos espacios.",
    description: "Org member remove confirmation description",
  },
  "organizations.remove": {
    message: "Quitar",
    description: "Org member remove confirmation: confirm button",
  },
  "organizations.removed": {
    message: "Se quitó a {identity} de la organización.",
    description: "Toast after removing an org member",
  },
  "organizations.removeFailed": {
    message: "No se pudo quitar a {identity}.",
    description: "Org member removal failure with no stated reason",
  },
  "organizations.roleChanged": {
    message: "Ahora {identity} es {role}.",
    description: "Toast after changing an org member's role",
  },
  "organizations.roleChangeFailed": {
    message: "No se pudo cambiar el rol.",
    description: "Org role change failure with no stated reason",
  },
  "organizations.membersForbiddenTitle": {
    message: "Los invitados no ven a los miembros",
    description:
      "Org Members page: a Guest's state (the API answers FORBIDDEN)",
  },
  "organizations.membersForbiddenBody": {
    message:
      "Solo los propietarios y los miembros de la organización pueden ver quién pertenece a ella.",
    description: "Org Members page: a Guest's state body",
  },
  "organizations.membersErrorTitle": {
    message: "No se pudieron cargar los miembros",
    description: "Org Members page: read failure title",
  },
  "organizations.invite": {
    message: "Invitar",
    description: "Org Members page: the Owner's invite button",
  },
  "organizations.inviteTitle": {
    message: "Invitar a la organización",
    description: "Org invite dialog title",
  },
  "organizations.inviteDescription": {
    message:
      "Recibirá un correo con un enlace para unirse como propietario o como miembro. Los administradores de cada espacio de trabajo invitan a personas a sus propios espacios.",
    description: "Org invite dialog description",
  },
  "organizations.fieldEmail": {
    message: "Dirección de correo electrónico",
    description: "Org invite dialog: email field label",
  },
  "organizations.fieldEmailInvalid": {
    message: "Introduce una dirección de correo válida.",
    description: "Org invite dialog: malformed email",
  },
  "organizations.fieldRole": {
    message: "Rol",
    description: "Org invite dialog: role field label",
  },
  "organizations.inviteSubmit": {
    message: "Enviar invitación",
    description: "Org invite dialog: submit button",
  },
  "organizations.invited": {
    message: "Se invitó a {email}.",
    description: "Toast after sending an org invite",
  },
  "organizations.inviteFailed": {
    message: "No se pudo enviar la invitación.",
    description: "Org invite failure with no stated reason",
  },
  "organizations.pendingTitle": {
    message: "Invitaciones pendientes",
    description: "Org Members page: the pending invite list heading",
  },
  "organizations.colEmail": {
    message: "Correo",
    description: "Org pending invite table column: the invitee's email",
  },
  "organizations.revokeInvite": {
    message: "Revocar",
    description: "Org pending invite row action",
  },
  "organizations.inviteRevoked": {
    message: "Se revocó la invitación a {email}.",
    description: "Toast after revoking an org invite",
  },
  "organizations.revokeFailed": {
    message: "No se pudo revocar la invitación.",
    description: "Org invite revoke failure with no stated reason",
  },

  "organizations.nameTitle": {
    message: "Nombre de la organización",
    description: "Org Settings page: the name card title",
  },
  "organizations.nameDescription": {
    message:
      "Aparece en el menú de espacios de trabajo para todas las personas de la organización.",
    description: "Org Settings page: the name card description",
  },
  "organizations.nameSave": {
    message: "Guardar",
    description: "Org Settings page: save the new name",
  },
  "organizations.nameSaved": {
    message: "La organización ahora se llama {name}.",
    description: "Toast after renaming the org",
  },
  "organizations.renameFailed": {
    message: "No se pudo cambiar el nombre de la organización.",
    description: "Org rename failure with no stated reason",
  },
  "organizations.nameOwnerOnly": {
    message:
      "Solo los propietarios pueden cambiar el nombre de la organización.",
    description:
      "Org Settings page: why the name is read-only for a Member or Guest",
  },
  "organizations.auditTitle": {
    message: "Registros de auditoría",
    description:
      "Org Settings page: the audit log section title (Render's label)",
  },
  "organizations.auditDescription": {
    message:
      "Exporta como CSV los eventos propios de la organización, como miembros, espacios de trabajo y configuración. No están en el registro de ningún espacio de trabajo, y cada exportación abarca como máximo {days} días.",
    description: "Org Settings page: the audit log section description",
  },

  "organizations.errorNotFound": {
    message: "Esta organización no existe o no eres uno de sus miembros.",
    description: "ORGANIZATION_NOT_FOUND refusal",
  },
  "organizations.errorForbidden": {
    message: "No tienes permiso para hacer eso en esta organización.",
    description: "FORBIDDEN refusal of an org action",
  },
  "organizations.errorLastOwner": {
    message:
      "Una organización necesita al menos un propietario. Primero haz propietario a otra persona.",
    description: "ORGANIZATION_LAST_OWNER refusal",
  },
  "organizations.errorMemberLastAdmin": {
    message:
      "Es el último administrador de {workspace}. Primero haz administrador a otra persona.",
    description: "ORGANIZATION_MEMBER_LAST_ADMIN refusal, naming the workspace",
  },
  "organizations.errorAlreadyMember": {
    message: "Esa persona ya está en la organización.",
    description: "ORGANIZATION_ALREADY_MEMBER refusal of an org invite",
  },
  "organizations.errorGuestSingleWorkspace": {
    message:
      "Un invitado pertenece a un solo espacio de trabajo de la organización. Hazlo miembro para darle acceso a más.",
    description: "ORGANIZATION_GUEST_SINGLE_WORKSPACE refusal",
  },
  "organizations.errorOwnRole": {
    message:
      "No puedes cambiar tu propio rol en la organización. Pídele a otro propietario que lo cambie.",
    description: "CANNOT_CHANGE_OWN_ROLE refusal",
  },
  "organizations.errorRemoveSelf": {
    message: "No puedes quitarte a ti mismo de la organización.",
    description: "CANNOT_REMOVE_SELF refusal",
  },
  "organizations.errorInvalidPrivacy": {
    message: "Elige Público o Solo por invitación.",
    description: "INVALID_PRIVACY refusal",
  },
};

export default esOrganizations;
