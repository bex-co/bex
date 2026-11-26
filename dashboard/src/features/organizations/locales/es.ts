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
  "organizations.securityTitle": {
    message: "Seguridad",
    description:
      "Org Settings page: title of the login requirements card (Render's section name, where it puts Require Login Method; w7/m466)",
  },
  "organizations.securityDescription": {
    message:
      "Cómo deben iniciar sesión los miembros, en todos los espacios de trabajo de la organización y en sus propias páginas, además de la configuración de cada espacio de trabajo.",
    description: "Org Settings page: what the Security card's settings do",
  },
  "organizations.securityOwnerOnly": {
    message: "Solo los propietarios pueden cambiar esta configuración.",
    description:
      "Org Settings page: why the Security settings are read-only for a Member or Guest",
  },
  "organizations.requireTwoFactorLabel": {
    message: "Exigir autenticación de dos factores",
    description:
      "Org Settings page: label of the switch that requires 2FA in every workspace of the org",
  },
  "organizations.requireTwoFactorDescription": {
    message:
      "Los miembros sin autenticación de dos factores (2FA) no pueden acceder a la organización ni a ninguno de sus espacios de trabajo hasta que la activen. Sus claves de API y claves SSH también quedan bloqueadas hasta entonces.",
    description: "Org Settings page: what requiring 2FA does",
  },
  "organizations.requireGoogleLoginLabel": {
    message: "Exigir inicio de sesión con Google",
    description:
      "Org Settings page: label of the switch that requires signing in with Google in every workspace of the org",
  },
  "organizations.requireGoogleLoginDescription": {
    message:
      "Los miembros que no inician sesión con la cuenta de Google de su correo electrónico de bex no pueden acceder a la organización ni a ninguno de sus espacios de trabajo. Las claves de API y claves SSH creadas con otro inicio de sesión también quedan bloqueadas.",
    description: "Org Settings page: what requiring Google login does",
  },
  "organizations.requireTwoFactorConfirmTitle": {
    message: "¿Exigir autenticación de dos factores en {organization}?",
    description:
      "Title of the confirmation before an Owner requires 2FA in the org",
  },
  "organizations.requireTwoFactorConfirm": {
    message:
      "Los miembros sin autenticación de dos factores pierden el acceso a la organización y a todos sus espacios de trabajo hasta que la activen, y sus claves de API y claves SSH dejan de funcionar con ellos. La configuración propia de cada espacio de trabajo se mantiene.",
    description:
      "Body of the confirmation before an Owner requires 2FA in the org",
  },
  "organizations.requireGoogleLoginConfirmTitle": {
    message: "¿Exigir inicio de sesión con Google en {organization}?",
    description:
      "Title of the confirmation before an Owner requires Google login in the org",
  },
  "organizations.requireGoogleLoginConfirm": {
    message:
      "Los miembros que no hayan iniciado sesión con la cuenta de Google de su correo electrónico de bex pierden el acceso a la organización y a todos sus espacios de trabajo hasta que inicien sesión así, y las claves de API y claves SSH creadas con otro inicio de sesión dejan de funcionar con ellos. La configuración propia de cada espacio de trabajo se mantiene.",
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
      "Ahora se exige autenticación de dos factores en todos los espacios de trabajo de {name}",
    description: "Toast after an Owner turns the org's 2FA requirement on",
  },
  "organizations.requireTwoFactorDisabledToast": {
    message: "{name} ya no exige autenticación de dos factores",
    description: "Toast after an Owner turns the org's 2FA requirement off",
  },
  "organizations.requireGoogleLoginEnabledToast": {
    message:
      "Ahora se exige iniciar sesión con Google en todos los espacios de trabajo de {name}",
    description:
      "Toast after an Owner turns the org's Google login requirement on",
  },
  "organizations.requireGoogleLoginDisabledToast": {
    message: "{name} ya no exige iniciar sesión con Google",
    description:
      "Toast after an Owner turns the org's Google login requirement off",
  },
  "organizations.requireTwoFactorErrorOwnerNotEnrolled": {
    message:
      "Activa la autenticación de dos factores en tu propia cuenta antes de exigirla en esta organización, o te quedarías fuera.",
    description:
      "Refusal for code WORKSPACE_2FA_ADMIN_NOT_ENROLLED on the org setting: the acting Owner has no second factor",
  },
  "organizations.requireGoogleLoginErrorOwnerNotSignedIn": {
    message:
      "Inicia sesión con la cuenta de Google de tu correo electrónico de bex antes de exigirlo en esta organización, o te quedarías fuera.",
    description:
      "Refusal for code WORKSPACE_GOOGLE_LOGIN_ADMIN_NOT_SIGNED_IN on the org setting: the acting Owner is not signed in with Google",
  },
  "organizations.auditNoPermission": {
    message: "No tienes permiso para realizar esta acción.",
    description:
      "Org Settings → Audit Logs: what a Member or Guest reads in place of the CSV export (Render's text)",
  },
  "organizations.auditKeysTitle": {
    message: "Claves de API de registros de auditoría",
    description:
      "Org Settings → Audit Logs: the API keys section heading (Render's)",
  },
  "organizations.auditKeysDescription": {
    message:
      "Crea claves de API con acceso de solo lectura a todos los registros de auditoría de esta organización y sus espacios de trabajo.",
    description:
      "Org Settings → Audit Logs: the API keys section description (Render's)",
  },
  "organizations.auditKeysCreate": {
    message: "Crear clave de API",
    description:
      "Org Settings → Audit Logs: the button that opens the create dialog",
  },
  "organizations.auditKeysNoPermission": {
    message:
      "No tienes permiso para gestionar las claves de API de esta organización.",
    description:
      "Org Settings → Audit Logs: what a Member or Guest reads under the disabled Create API key button (Render's text)",
  },
  "organizations.auditKeysLoading": {
    message: "Cargando claves de API…",
    description: "Org Settings → Audit Logs: while the org's API keys load",
  },
  "organizations.auditKeysLoadFailed": {
    message: "No se pudieron cargar las claves de API de tu organización.",
    description:
      "Org Settings → Audit Logs: the org's API keys could not be read",
  },
  "organizations.auditKeysRetry": {
    message: "Reintentar",
    description:
      "Org Settings → Audit Logs: reload the org's API keys after a failure",
  },
  "organizations.auditKeysEmpty": {
    message: "No hay claves de API emitidas.",
    description: "Org Settings → Audit Logs: the org has no API keys",
  },
  "organizations.auditKeysColName": {
    message: "Nombre",
    description: "Org API keys table: the key's name column",
  },
  "organizations.auditKeysColKey": {
    message: "Clave",
    description:
      "Org API keys table: the key's id column, shown truncated as Render's key prefix",
  },
  "organizations.auditKeysColCreated": {
    message: "Creada",
    description: "Org API keys table: when the key was created",
  },
  "organizations.auditKeysColLastUsed": {
    message: "Último uso",
    description: "Org API keys table: when the key was last used",
  },
  "organizations.auditKeysNeverUsed": {
    message: "Nunca",
    description: "Org API keys table: a key that has not been used",
  },
  "organizations.auditKeysActions": {
    message: "Acciones de {label}",
    description: "Accessible name of an org API key row's actions menu",
  },
  "organizations.auditKeysRevoke": {
    message: "Revocar",
    description: "Org API key row menu: revoke the key",
  },
  "organizations.auditKeysRevokeTitle": {
    message: "Revocar clave de API",
    description: "Org API key revoke confirmation: title",
  },
  "organizations.auditKeysRevokeBody": {
    message:
      "“{label}” ya no podrá acceder a los registros de auditoría de tu organización.",
    description:
      "Org API key revoke confirmation: what revoking does (Render's text)",
  },
  "organizations.auditKeysRevokeCancel": {
    message: "Cancelar",
    description: "Org API key revoke confirmation: cancel",
  },
  "organizations.auditKeysRevokeConfirm": {
    message: "Revocar clave de API",
    description: "Org API key revoke confirmation: confirm",
  },
  "organizations.auditKeysRevokeFailed": {
    message: "No se pudo revocar tu clave de API. Inténtalo de nuevo.",
    description: "Org API key revoke failed",
  },
  "organizations.auditKeysCreateTitle": {
    message: "Crear una clave de API",
    description: "Org API key create dialog: title before the key exists",
  },
  "organizations.auditKeysCreatedTitle": {
    message: "Clave de API creada",
    description: "Org API key create dialog: title once the key exists",
  },
  "organizations.auditKeysCreateHint": {
    message: "Dale a tu clave de API un nombre único, como “agent” o “laptop”.",
    description: "Org API key create dialog: the name hint (Render's text)",
  },
  "organizations.auditKeysNameLabel": {
    message: "Nombre de la clave de API",
    description: "Org API key create dialog: the name field's label",
  },
  "organizations.auditKeysNamePlaceholder": {
    message: "nombre",
    description: "Org API key create dialog: the name field's placeholder",
  },
  "organizations.auditKeysGenerate": {
    message: "Generar clave de API",
    description: "Org API key create dialog: submit",
  },
  "organizations.auditKeysClose": {
    message: "Cerrar",
    description: "Org API key create dialog: close before a key exists",
  },
  "organizations.auditKeysDone": {
    message: "Listo",
    description: "Org API key create dialog: close once the key is shown",
  },
  "organizations.auditKeysSaveNow": {
    message: "Guarda esta clave de API ahora. No podrás volver a verla.",
    description:
      "Org API key create dialog: the key is shown only once (Render's text)",
  },
  "organizations.auditKeysKeyLabel": {
    message: "Clave de API",
    description: "Org API key create dialog: the label of the new key's field",
  },
  "organizations.auditKeysCopy": {
    message: "Copiar clave de API",
    description: "Org API key create dialog: copy the new key",
  },
  "organizations.auditKeysCopied": {
    message: "Copiada",
    description: "Org API key create dialog: the key was copied",
  },
  "organizations.auditKeysCopyFailed": {
    message: "No se pudo copiar la clave.",
    description: "Org API key create dialog: copying the key failed",
  },
  "organizations.auditKeysCreateFailed": {
    message: "No se pudo crear tu clave de API. Inténtalo de nuevo.",
    description: "Org API key create failed",
  },
  "organizations.auditKeysTooMany": {
    message: "Has creado demasiadas claves. Inténtalo de nuevo más tarde.",
    description: "Org API key create refused: too many keys (Render's text)",
  },
  "organizations.memberActions": {
    message: "Acciones de {identity}",
    description: "Accessible name of an org member row's actions menu",
  },
  "organizations.viewDetails": {
    message: "Ver detalles",
    description: "Org member row menu: open the member's details",
  },
  "organizations.removeMemberItem": {
    message: "Quitar miembro",
    description: "Org member row menu: remove the member from the org",
  },
  "organizations.detailsOwner": {
    message: "Propietario",
    description: "Org member details: shown under an Org Owner's name",
  },
  "organizations.detailsWorkspaces": {
    message: "Espacios de trabajo",
    description: "Org member details: the member's workspaces section heading",
  },
  "organizations.detailsNoWorkspaces": {
    message: "Sin espacios de trabajo",
    description: "Org member details: the member belongs to no workspace",
  },
  "organizations.detailsNoWorkspacesBody": {
    message: "Este miembro no ha sido invitado a ningún espacio de trabajo.",
    description: "Org member details: the member belongs to no workspace",
  },
  "organizations.detailsLoginStatus": {
    message: "Estado de inicio de sesión y autenticación",
    description:
      "Org member details: the login methods and 2FA section heading (Render's)",
  },
  "organizations.detailsLoginStatusLoading": {
    message: "Cargando el estado de inicio de sesión…",
    description: "Org member details: while the member's login status loads",
  },
  "organizations.loginMethodPassword": {
    message: "Contraseña",
    description: "Org member details: the password login method",
  },
  "organizations.loginSignedIn": {
    message: "Ha iniciado sesión",
    description:
      "Org member details: the member has signed in with this method",
  },
  "organizations.loginNotSignedIn": {
    message: "No ha iniciado sesión",
    description:
      "Org member details: the member has not signed in with this method",
  },
  "organizations.loginTwoFactor": {
    message: "Autenticación de dos factores",
    description: "Org member details: the 2FA row",
  },
  "organizations.loginTwoFactorEnabled": {
    message: "Activada",
    description: "Org member details: the member has 2FA",
  },
  "organizations.loginTwoFactorNotEnabled": {
    message: "No activada",
    description: "Org member details: the member has no 2FA",
  },
  "organizations.detailsClose": {
    message: "Cerrar",
    description: "Org member details: close the dialog",
  },
};

export default esOrganizations;
