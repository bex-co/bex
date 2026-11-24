import type { TranslationEntry } from "@/i18n";

const esCapabilities: Record<string, TranslationEntry> = {
  "capabilities.reasonCanCreate": {
    message:
      "Tu rol no puede hacer este cambio. Pregunta a un administrador del espacio de trabajo.",
    description:
      "Tooltip/hint on a create-gated service, environment, or datastore control for a member without can_create",
  },
  "capabilities.reasonCanOperate": {
    message:
      "Tu rol solo puede ver este servicio. Pide a un administrador del espacio de trabajo que haga los cambios.",
    description:
      "Tooltip/hint on an operate-gated lifecycle control (e.g. a service restart) for a viewer without can_operate",
  },
  "capabilities.reasonCanViewSensitive": {
    message:
      "Tu rol no puede revelar valores secretos. Pregunta a un administrador del espacio de trabajo.",
    description:
      "Tooltip on a reveal control (connection string, env value) for a member without can_view_sensitive",
  },
  "capabilities.reasonCanManageBilling": {
    message:
      "Tu rol no puede administrar la facturación. Pregunta a un administrador del espacio de trabajo o a un miembro de facturación.",
    description:
      "Tooltip on a billing control for a member without can_manage_billing",
  },
  "capabilities.reasonCanManage": {
    message:
      "Tu rol no puede administrar miembros. Pregunta a un administrador del espacio de trabajo.",
    description:
      "Tooltip on a member-management control for a member without can_manage",
  },
  "capabilities.actionDenied": {
    message:
      "Tu rol no puede realizar esta acción. Pregunta a un administrador del espacio de trabajo.",
    description:
      "Tooltip on a service/deploy action refused by the server action decision",
  },
  "capabilities.actionAdminOnly": {
    message: "Esta acción está limitada a los administradores.",
    description:
      "Tooltip on an action refused to a non-admin in a protected environment: Suspend or Resume, per resource and in bulk (Render's copy, w4/m578); a datastore's connection-info reveal and IP rules, and an environment group's variables, secret files, links and Delete (w4/m594)",
  },
  "capabilities.actionUnavailable": {
    message:
      "El servidor no pudo confirmar esta acción en este momento. Inténtalo de nuevo.",
    description:
      "Tooltip when an action projection is unavailable or returned an unknown outcome",
  },
  "capabilities.actionChecking": {
    message: "Comprobando si puedes realizar esta acción…",
    description:
      "Tooltip while a service/deploy action decision is still loading",
  },
  "capabilities.blockedProtectedConfirmation": {
    message:
      "Este recurso protegido necesita una confirmación adicional antes de poder ejecutar esta acción.",
    description: "Blocked reason for protected_confirmation_required",
  },
  "capabilities.blockedSuspended": {
    message:
      "No disponible mientras el servicio está suspendido. Reanúdalo primero.",
    description: "Blocked reason for suspended precondition",
  },
  "capabilities.blockedNotSuspended": {
    message:
      "Este recurso ya está en ejecución, así que no hay nada que reanudar.",
    description: "Blocked reason for not_suspended precondition",
  },
  "capabilities.blockedNoActiveDeploy": {
    message: "No hay ningún despliegue activo que cancelar en este momento.",
    description: "Blocked reason for no_active_deploy",
  },
  "capabilities.blockedNoActiveRun": {
    message: "No hay ninguna ejecución activa que cancelar en este momento.",
    description: "Blocked reason for no_active_run",
  },
  "capabilities.blockedNoEligibleRollbackTarget": {
    message: "Ningún despliegue anterior es apto como destino de reversión.",
    description: "Blocked reason for no_eligible_rollback_target",
  },
  "capabilities.blockedBillingBlocked": {
    message:
      "La facturación necesita atención antes de poder ejecutar esta acción. Resuélvelo en Facturación.",
    description: "Blocked reason for billing_blocked",
  },
  "capabilities.blockedExpired": {
    message:
      "El plan gratuito venció, por lo que este recurso sigue suspendido. Cambia a un plan de pago para reanudarlo.",
    description: "Blocked reason for expired precondition",
  },
  "capabilities.blockedUnavailable": {
    message:
      "El servidor no pudo confirmar esta acción en este momento. Inténtalo de nuevo.",
    description: "Blocked reason for unavailable precondition",
  },
  "capabilities.blockedGeneric": {
    message:
      "El servidor no permite esta acción en este momento. Inténtalo de nuevo.",
    description: "Fallback blocked reason",
  },
  "capabilities.grantUnavailable": {
    message:
      "No se pudieron actualizar los permisos. Inténtalo de nuevo; esto no es un cambio de rol.",
    description:
      "Recovery copy when a capability grant check fails or is stale (m144)",
  },
  "capabilities.grantStale": {
    message:
      "Los permisos necesitan actualizarse antes de poder ejecutar esta acción.",
    description:
      "Copy when a capability answer is older than the freshness bound",
  },
};

export default esCapabilities;
