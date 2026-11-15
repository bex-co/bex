import type { TranslationEntry } from "@/i18n";

const esTeam: Record<string, TranslationEntry> = {
  "team.title": {
    message: "Equipo",
    description: "Settings Team section card title",
  },
  "team.description": {
    message:
      "Las personas de este espacio de trabajo y lo que pueden hacer. Invita a compañeros por correo electrónico y asigna un rol; los roles se aplican en todos los recursos.",
    description: "Settings Team section card description",
  },
  "team.searchLabel": {
    message: "Buscar miembros",
    description: "Accessible label for the accepted-member search field",
  },
  "team.searchPlaceholder": {
    message: "Buscar miembros",
    description: "Placeholder for the accepted-member search field",
  },
  "team.emptyTitle": {
    message: "Aún no hay miembros",
    description: "Accepted-member table empty-state title",
  },
  "team.emptyBody": {
    message:
      "Invita a alguien para empezar a colaborar en este espacio de trabajo.",
    description: "Accepted-member table empty-state description",
  },
  "team.noMatchesTitle": {
    message: "Ningún miembro coincide",
    description: "Accepted-member search no-results title",
  },
  "team.noMatchesBody": {
    message: "Prueba con otro correo electrónico o identidad.",
    description: "Accepted-member search no-results description",
  },
  "team.colMember": {
    message: "Miembro",
    description: "Team table column header — the member identity",
  },
  "team.colRole": {
    message: "Rol",
    description: "Team table column header — the member's role",
  },
  "team.colEmail": {
    message: "Correo electrónico",
    description: "Pending invites table column header",
  },
  "team.pendingTitle": {
    message: "Invitaciones pendientes",
    description: "Heading above the pending-invites table",
  },
  "team.invite": {
    message: "Invitar",
    description: "Button that opens the invite dialog",
  },
  "team.inviteTitle": {
    message: "Invitar a un compañero",
    description: "Invite dialog title",
  },
  "team.inviteDescription": {
    message:
      "Recibirá un correo y se unirá a este espacio de trabajo la primera vez que inicie sesión con esa dirección.",
    description: "Invite dialog description",
  },
  "team.fieldEmail": {
    message: "Dirección de correo electrónico",
    description: "Invite dialog email field label",
  },
  "team.fieldEmailPlaceholder": {
    message: "teammate@example.com",
    description: "Invite dialog email field placeholder",
  },
  "team.fieldEmailInvalid": {
    message:
      "Introduce una dirección de correo válida, p. ej. teammate@example.com.",
    description: "Invite dialog validation message for a malformed email",
  },
  "team.fieldRole": {
    message: "Rol",
    description: "Invite dialog role field label",
  },
  "team.inviteCancel": {
    message: "Cancelar",
    description: "Invite dialog cancel button",
  },
  "team.inviteSubmit": {
    message: "Enviar invitación",
    description: "Invite dialog submit button",
  },
  "team.inviteSuccess": {
    message: "Invitación enviada a {email}",
    description: "Toast after a successful invite",
  },
  "team.inviteError": {
    message: "No se pudo invitar a {email}",
    description: "Toast after a failed invite",
  },
  "team.inviteErrorPlanTitle": {
    message: "Tu plan no admite esta invitación",
    description:
      "Title of the inline alert when the workspace plan's member cap or role set blocks an invite",
  },
  "team.inviteErrorPlanCta": {
    message: "Cambiar de plan",
    description:
      "Link from the blocked-invite alert to the workspace settings plan section",
  },
  "team.inviteErrorPlanLimitSeats": {
    message:
      "El plan {plan} está limitado a {limit} miembro(s) del espacio de trabajo. Mejora tu plan para invitar a más.",
    description:
      "Plan seat-cap refusal in the invite dialog — shown when accepted members + pending invites reach the plan maximum",
  },
  "team.inviteErrorPlanLimitRole": {
    message:
      "El plan {plan} no ofrece este rol. Mejora tu plan para acceder a todos los roles.",
    description:
      "Plan role-gate refusal in the invite dialog — shown when the chosen role isn't available on the current plan",
  },
  "team.remove": {
    message: "Quitar",
    description: "Remove-member button / confirm label",
  },
  "team.roleFor": {
    message: "Rol de {identity}",
    description:
      "Accessible name of a member row's role picker, naming the member (w4/150)",
  },
  "team.removeMember": {
    message: "Quitar a {identity}",
    description:
      "Accessible name of a member row's remove control, naming the member (w4/150)",
  },
  "team.removeTitle": {
    message: "¿Quitar miembro?",
    description: "Remove-member confirmation dialog title",
  },
  "team.removeConfirm": {
    message:
      "{identity} perderá el acceso a este espacio de trabajo de inmediato.",
    description: "Remove-member confirmation dialog body",
  },
  "team.removeRevokesKeys": {
    message:
      "Esto también revoca las claves de API que {identity} creó aquí, lo que puede interrumpir las automatizaciones que se autentican con ellas.",
    description:
      "Remove-member dialog warning (w2/m163): removal disposes of the member's machine credentials, so an admin must see the cost before confirming rather than discovering it as an outage",
  },
  "team.removeCancel": {
    message: "Cancelar",
    description: "Remove-member confirmation cancel button",
  },
  "team.removeSuccess": {
    message: "Miembro quitado",
    description: "Toast after a successful remove",
  },
  "team.removeError": {
    message: "No se pudo quitar a ese miembro",
    description: "Toast after a failed remove",
  },
  "team.roleChanged": {
    message: "Rol actualizado",
    description: "Toast after a successful role change",
  },
  "team.roleChangeError": {
    message: "No se pudo cambiar ese rol",
    description: "Toast after a failed role change",
  },
  "team.lastAdminError": {
    message: "Un espacio de trabajo debe conservar al menos un administrador.",
    description: "Toast when the last-admin guard refuses a demote/remove",
  },
  "team.revokeInvite": {
    message: "Revocar",
    description: "Revoke pending-invite button",
  },
  "team.resendInvite": {
    message: "Reenviar",
    description: "Resend pending-invite button (fresh email, refreshed expiry)",
  },
  "team.resendInviteSuccess": {
    message: "Invitación reenviada a {email}",
    description: "Toast after a successful invite resend",
  },
  "team.resendInviteError": {
    message: "No se pudo reenviar esa invitación",
    description: "Toast after a failed invite resend",
  },
  "team.seatUsage": {
    message: "{used} de {limit} puestos",
    description:
      "Seat usage in the Team card title on a limited plan — accepted members plus pending invites over the plan cap",
  },
  "team.seatCount_one": {
    message: "{count} puesto en uso",
    description:
      "Seat count in the Team card title on an unlimited plan (no cap to show, singular)",
  },
  "team.seatCount_many": {
    message: "{count} de puestos en uso",
    description:
      "Seat count in the Team card title on an unlimited plan (no cap to show, plural)",
  },
  "team.seatCount_other": {
    message: "{count} puestos en uso",
    description:
      "Seat count in the Team card title on an unlimited plan (no cap to show, plural)",
  },
  "team.seatsFullBody": {
    message:
      "Este espacio de trabajo ha usado todos los puestos que ofrece su plan. Mejora tu plan para invitar a más miembros.",
    description:
      "Invite dialog wall when seats are exhausted before composing an invite",
  },
  "team.mfaEnabled": {
    message: "2FA",
    description:
      "Badge on a member row whose account has a second factor enrolled",
  },
  "team.leaveTitle": {
    message: "Salir del espacio de trabajo",
    description: "Title of the leave-workspace card in workspace settings",
  },
  "team.leaveDescription": {
    message:
      "Renuncia a tu pertenencia a {workspace}. Perderás el acceso a sus servicios y datos, y las claves de API que creaste aquí dejarán de funcionar. Un administrador puede volver a invitarte.",
    description: "Description of the leave-workspace card",
  },
  "team.leaveAction": {
    message: "Salir del espacio de trabajo",
    description: "Label of the leave-workspace button",
  },
  "team.leaveConfirmTitle": {
    message: "¿Salir de este espacio de trabajo?",
    description: "Title of the leave-workspace confirmation dialog",
  },
  "team.leaveConfirm": {
    message:
      "Perderás el acceso a {workspace} y se revocarán las claves de API que creaste en él. Solo podrás volver a unirte si un administrador te invita.",
    description: "Body of the leave-workspace confirmation dialog",
  },
  "team.leaveCancel": {
    message: "Cancelar",
    description: "Cancel label in the leave-workspace confirmation dialog",
  },
  "team.leaveErrorTitle": {
    message: "No se pudo salir del espacio de trabajo",
    description: "Title of the inline error when leaving is refused",
  },
  "team.leaveError": {
    message: "No se pudo salir del espacio de trabajo. Inténtalo de nuevo.",
    description: "Fallback message when leaving fails without a server reason",
  },
  "team.leaveLastAdminReason": {
    message:
      "Eres el último administrador. Nombra primero administrador a otra persona, para que el espacio de trabajo siga teniendo uno.",
    description: "Reason the Leave action is disabled for the last admin",
  },
  "team.you": {
    message: "Tú",
    description: "Badge marking the caller's own row in the members table",
  },
  "team.removeSelfReason": {
    message:
      "No puedes quitarte a ti mismo. Sal del espacio de trabajo en su lugar.",
    description:
      "Reason the Remove control is disabled on the caller's own row",
  },
  "team.changeOwnRoleReason": {
    message:
      "No puedes cambiar tu propio rol. Pide a otro administrador que lo cambie.",
    description: "Reason the role picker is disabled on the caller's own row",
  },
  "team.mfaEnabledTooltip": {
    message: "Autenticación de dos factores activada",
    description: "Tooltip for the member-row 2FA badge",
  },
  "team.identityUnresolved": {
    message: "Sin resolver",
    description:
      "Badge on a member row whose Kratos identity lookup missed (w4/070)",
  },
  "team.identityUnresolvedTooltip": {
    message:
      "No se pudo consultar la cuenta de este miembro. Sigue ocupando un puesto hasta que un administrador lo quite.",
    description: "Tooltip for the unresolved-identity member badge",
  },
  "team.inviteAccepted": {
    message: "Te has unido a {workspace}",
    description: "Toast after an emailed invite link is redeemed successfully",
  },
  "team.inviteAcceptedAlready": {
    message: "Esa invitación ya se usó",
    description: "Toast when an invite token was already redeemed",
  },
  "team.inviteAcceptExpired": {
    message: "Esa invitación ha caducado: pide una nueva",
    description: "Toast when an invite token is past its expiry",
  },
  "team.inviteAcceptError": {
    message: "No se pudo aceptar esa invitación",
    description: "Toast when redeeming an invite token fails",
  },
  "team.revokeInviteSuccess": {
    message: "Invitación revocada",
    description: "Toast after a successful invite revoke",
  },
  "team.revokeInviteError": {
    message: "No se pudo revocar esa invitación",
    description: "Toast after a failed invite revoke",
  },
  "team.errorTitle": {
    message: "No se pudo cargar el equipo",
    description: "Team panel generic error title",
  },
  "team.errorBody": {
    message:
      "Algo salió mal al cargar los miembros de este espacio de trabajo. Inténtalo de nuevo.",
    description: "Team panel generic error body",
  },
  "team.role.VIEWER": {
    message: "Lector",
    description: "Role label — read-only",
  },
  "team.role.CONTRIBUTOR": {
    message: "Colaborador",
    description: "Role label — works on resources, no sensitive fields",
  },
  "team.role.DEVELOPER": {
    message: "Desarrollador",
    description: "Role label — full resource access",
  },
  "team.role.ADMIN": {
    message: "Administrador",
    description: "Role label — full access incl. settings, members, billing",
  },
  "team.role.BILLING": {
    message: "Facturación",
    description: "Role label — billing only",
  },
  "team.twoFactorRequiredNotEnabled": {
    message: "2FA requerida, no activada",
    description:
      "Badge on a member row while the workspace requires 2FA and the member has none (w22/m402)",
  },
  "team.twoFactorRequiredNotEnabledTooltip": {
    message:
      "Este espacio de trabajo requiere autenticación de dos factores. Este miembro no puede acceder hasta que active 2FA.",
    description: "Tooltip for the required-but-not-enabled member badge",
  },
};

export default esTeam;
