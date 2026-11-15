import type { TranslationEntry } from "@/i18n";

const esInvites: Record<string, TranslationEntry> = {
  "invites.title": {
    message: "Invitación al espacio de trabajo",
    description: "Invitation review: title",
  },
  "invites.authenticate": {
    message:
      "Crea una cuenta o inicia sesión para revisar tu invitación al espacio de trabajo.",
    description: "Invitation review: authenticate",
  },
  "invites.signUp": {
    message: "Registrarse",
    description: "Invitation review: signUp",
  },
  "invites.signIn": {
    message: "Iniciar sesión",
    description: "Invitation review: signIn",
  },
  "invites.joinTitle": {
    message: "Unirte a {workspace}",
    description: "Invitation review: joinTitle",
  },
  "invites.memberTitle": {
    message: "Eres miembro de {workspace}",
    description: "Invitation review: memberTitle",
  },
  "invites.role": {
    message: "Te unirás como {role}.",
    description: "Invitation review: role",
  },
  "invites.memberRole": {
    message: "Tu rol es {role}.",
    description: "Invitation review: memberRole",
  },
  "invites.inviter": {
    message: "Invitado por {email}",
    description: "Invitation review: inviter",
  },
  "invites.account": {
    message: "Sesión iniciada como {email}",
    description: "Invitation review: account",
  },
  "invites.join": {
    message: "Unirse al espacio de trabajo",
    description: "Invitation review: join",
  },
  "invites.joining": {
    message: "Uniéndote…",
    description: "Invitation review: joining",
  },
  "invites.open": {
    message: "Abrir espacio de trabajo",
    description: "Invitation review: open",
  },
  "invites.opening": {
    message: "Abriendo…",
    description: "Invitation review: opening",
  },
  "invites.notNow": {
    message: "Ahora no",
    description: "Invitation review: notNow",
  },
  "invites.continue": {
    message: "Continuar a bex",
    description: "Invitation review: continue",
  },
  "invites.unavailableTitle": {
    message: "Invitación no disponible",
    description: "Invitation review: unavailableTitle",
  },
  "invites.invalid": {
    message:
      "Este enlace de invitación no es válido o fue revocado. Pide al administrador del espacio de trabajo una nueva invitación.",
    description: "Invitation review: invalid",
  },
  "invites.expired": {
    message:
      "Esta invitación ha caducado. Pide al administrador del espacio de trabajo que la reenvíe.",
    description: "Invitation review: expired",
  },
  "invites.used": {
    message:
      "Esta invitación ya se usó. Inicia sesión con la cuenta que se unió o pide una nueva invitación.",
    description: "Invitation review: used",
  },
  "invites.planLimit": {
    message:
      "Este espacio de trabajo no puede aceptar otro miembro con su plan actual. Pide al administrador del espacio de trabajo que revise su plan y sus plazas, y vuelve a intentarlo.",
    description: "Invitation review: planLimit",
  },
  "invites.retryError": {
    message:
      "No pudimos completar esa solicitud. Tu invitación está guardada en esta pestaña. Inténtalo de nuevo.",
    description: "Invitation review: retryError",
  },
  "invites.accessPending": {
    message:
      "Tu membresía está confirmada, pero el acceso al espacio de trabajo aún no está listo. Intenta abrir el espacio de trabajo de nuevo.",
    description: "Invitation review: accessPending",
  },
  "invites.storageUnavailable": {
    message:
      "No pudimos guardar tu invitación en este navegador. Permite el almacenamiento del sitio y vuelve a abrir el enlace de invitación.",
    description: "Invitation review: storageUnavailable",
  },
  "invites.retry": {
    message: "Reintentar",
    description: "Invitation review: retry",
  },
};

export default esInvites;
