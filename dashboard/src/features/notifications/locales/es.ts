import type { TranslationEntry } from "@/i18n";

const esNotifications: Record<string, TranslationEntry> = {
  "notifications.title": {
    message: "Notificaciones",
    description: "Settings Notifications section card title",
  },
  "notifications.description": {
    message:
      "Recibe un correo cuando un despliegue de uno de tus servicios comience, tenga éxito o falle, y cuando algo falle en tiempo de ejecución. Son tus propias preferencias: cada miembro del espacio de trabajo configura las suyas de forma independiente.",
    description: "Settings Notifications section card description",
  },
  "notifications.deployStarted": {
    message: "Despliegue iniciado",
    description: "Toggle label",
  },
  "notifications.deployStartedHint": {
    message:
      "Envíame un correo cuando comience un despliegue. Solo se envía para los servicios que envían todas las notificaciones.",
    description: "Toggle hint",
  },
  "notifications.deploySucceeded": {
    message: "Despliegue correcto",
    description: "Toggle label",
  },
  "notifications.deploySucceededHint": {
    message:
      "Envíame un correo cuando un despliegue pase a estar activo. Solo se envía para los servicios que envían todas las notificaciones.",
    description: "Toggle hint",
  },
  "notifications.deployFailed": {
    message: "Fallos",
    description: "Toggle label",
  },
  "notifications.deployFailedHint": {
    message:
      "Envíame un correo cuando falle un despliegue, un servicio deje de estar en buen estado, falle una ejecución de cron, falle la descarga de una imagen o falle la sincronización de un Blueprint.",
    description: "Toggle hint",
  },
  "notifications.errorTitle": {
    message: "No se pudo cargar la configuración de notificaciones",
    description: "Generic error title",
  },
  "notifications.errorBody": {
    message: "Algo salió mal. Inténtalo de nuevo.",
    description: "Generic error body",
  },
  "notifications.updateError": {
    message: "No se pudo guardar tu preferencia",
    description: "Toast on a failed update",
  },
  "notifications.pushTitle": {
    message: "Notificaciones push móviles",
    description: "Push settings card title",
  },
  "notifications.bexExtension": {
    message: "extensión de bex",
    description: "Badge distinguishing native push from Render parity",
  },
  "notifications.pushDescription": {
    message:
      "Controla las alertas móviles de bex, su urgencia, el horario de entrega y las excepciones por servicio. Las preferencias de correo de arriba no cambian.",
    description: "Push settings description",
  },
  "notifications.pushEnabled": {
    message: "Activar notificaciones push móviles",
    description: "Push enabled toggle",
  },
  "notifications.pushEnabledHint": {
    message: "Envía alertas a tus dispositivos móviles de bex registrados.",
    description: "Push enabled hint",
  },
  "notifications.pushUnavailable": {
    message:
      "La entrega de notificaciones push no está configurada en este servidor de bex",
    description: "Disabled server status",
  },
  "notifications.pushUnavailableHint": {
    message:
      "Puedes preparar y guardar la política ahora, pero los dispositivos registrados no podrán recibir notificaciones hasta que un operador configure el proveedor de push.",
    description: "Disabled server status hint",
  },
  "notifications.webPushTitle": {
    message: "Notificaciones del navegador",
    description: "Web push settings card title",
  },
  "notifications.webPushDescription": {
    message:
      "Recibe las mismas alertas en este navegador. Los filtros de eventos y las horas de silencio son los que definiste en Notificaciones push móviles; no hay una segunda política.",
    description: "Web push settings description",
  },
  "notifications.webPushUnsupported": {
    message: "Este navegador no admite notificaciones push web.",
    description: "Unsupported browser",
  },
  "notifications.webPushUnconfigured": {
    message:
      "Este servidor de bex no ha activado las notificaciones push del navegador.",
    description: "Server VAPID unset",
  },
  "notifications.webPushDenied": {
    message:
      "Las notificaciones están bloqueadas para este sitio. Permítelas en la configuración del navegador y vuelve a intentarlo.",
    description: "Permission denied",
  },
  "notifications.webPushError": {
    message:
      "No se pudieron actualizar las notificaciones del navegador. Inténtalo de nuevo.",
    description: "Subscribe error",
  },
  "notifications.webPushOn": {
    message: "Este navegador recibirá las alertas de bex.",
    description: "Subscribed state",
  },
  "notifications.webPushOff": {
    message: "Este navegador no está suscrito.",
    description: "Unsubscribed state",
  },
  "notifications.webPushEnable": {
    message: "Activar en este navegador",
    description: "Subscribe button",
  },
  "notifications.webPushDisable": {
    message: "Desactivar en este navegador",
    description: "Unsubscribe button",
  },
  "notifications.pushEvents": {
    message: "Filtros de eventos",
    description: "Push event section",
  },
  "notifications.pushEventsHint": {
    message:
      "Solo los tipos de evento seleccionados con una urgencia igual o superior a la urgencia mínima pueden generar una notificación push.",
    description: "Push events hint",
  },
  "notifications.pushEventNotSentYet": {
    message: "Aún no se envía: hoy nada envía este evento.",
    description:
      "Under a push event that no part of bex emits yet (it is reserved for an upcoming feature); selecting it is kept but has no effect",
  },
  "notifications.pushEventNotSentYetShort": {
    message: "Aún no se envía",
    description:
      "Compact form of the not-sent-yet note, under a push event in a per-service override's event grid",
  },
  "notifications.pushEventFiltered": {
    message: "No se envía por push mientras la urgencia mínima sea {minimum}.",
    description:
      "Under a selected push event whose urgency is below the minimum ({minimum} = urgency name, e.g. 'Important')",
  },
  "notifications.pushFilteredTitle": {
    message: "Algunos eventos seleccionados no se enviarán por push",
    description:
      "Heading of the save-area summary listing selected push events the minimum urgency drops",
  },
  "notifications.pushFilteredEvent": {
    message:
      "{event} es {urgency} y no se enviará por push mientras la urgencia mínima sea {minimum}.",
    description:
      "One summary line ({event} = event name, {urgency} = its urgency, {minimum} = the minimum urgency)",
  },
  "notifications.pushFilteredServiceEvent": {
    message:
      "{service}: {event} es {urgency} y no se enviará por push mientras la urgencia mínima de este servicio sea {minimum}.",
    description:
      "One summary line for a service override ({service} = service name; others as pushFilteredEvent)",
  },
  "notifications.pushOverrideFiltered_one": {
    message:
      "{count} evento seleccionado no se enviará por push para este servicio mientras su urgencia mínima sea {minimum}.",
    description:
      "Under a service override whose own policy drops selected events ({minimum} = urgency name)",
  },
  "notifications.pushOverrideFiltered_many": {
    message:
      "{count} de eventos seleccionados no se enviarán por push para este servicio mientras su urgencia mínima sea {minimum}.",
    description:
      "Under a service override whose own policy drops selected events ({minimum} = urgency name)",
  },
  "notifications.pushOverrideFiltered_other": {
    message:
      "{count} eventos seleccionados no se enviarán por push para este servicio mientras su urgencia mínima sea {minimum}.",
    description:
      "Under a service override whose own policy drops selected events ({minimum} = urgency name)",
  },
  "notifications.pushEventDeployFailed": {
    message: "Despliegue fallido",
    description: "Push event",
  },
  "notifications.pushEventServerFailed": {
    message: "El servicio se bloqueó",
    description: "Push event",
  },
  "notifications.pushEventCronFailed": {
    message: "Ejecución de cron fallida",
    description: "Push event",
  },
  "notifications.pushEventDeployStarted": {
    message: "Despliegue iniciado",
    description: "Push event",
  },
  "notifications.pushEventDeploySucceeded": {
    message: "Despliegue correcto",
    description: "Push event",
  },
  "notifications.pushEventServerAvailable": {
    message: "Servicio recuperado",
    description: "Push event",
  },
  "notifications.pushEventServiceSuspended": {
    message: "Servicio suspendido",
    description: "Push event",
  },
  "notifications.pushEventServiceResumed": {
    message: "Servicio reanudado",
    description: "Push event",
  },
  "notifications.pushEventAgentNeedsDecision": {
    message: "El agente necesita una decisión",
    description: "Push event",
  },
  "notifications.pushEventAgentPrReady": {
    message: "PR del agente listo",
    description: "Push event",
  },
  "notifications.pushEventAgentFailed": {
    message: "Sesión del agente fallida",
    description: "Push event",
  },
  "notifications.pushEventAgentAutomationPaused": {
    message: "Automatización en pausa",
    description: "Push event: the platform paused an agent automation",
  },
  "notifications.pushEventPostgresUnavailable": {
    message: "Base de datos no disponible",
    description: "Push event",
  },
  "notifications.pushEventPostgresAvailable": {
    message: "Base de datos recuperada",
    description: "Push event",
  },
  "notifications.pushEventKeyValueUnhealthy": {
    message: "Key Value en mal estado",
    description: "Push event",
  },
  "notifications.pushEventKeyValueAvailable": {
    message: "Key Value recuperado",
    description: "Push event",
  },
  "notifications.pushEventPostgresBackupFailed": {
    message: "Falló la copia de seguridad de la base de datos",
    description: "Push event",
  },
  "notifications.pushEventPostgresRestoreFailed": {
    message: "Falló la restauración de la base de datos",
    description: "Push event",
  },
  "notifications.pushEventPostgresUpgradeFailed": {
    message: "Falló la actualización de la base de datos",
    description: "Push event",
  },
  "notifications.pushMinimumUrgency": {
    message: "Urgencia mínima",
    description: "Urgency field",
  },
  "notifications.pushUrgencyRoutine": {
    message: "Rutinaria",
    description: "Urgency option",
  },
  "notifications.pushUrgencyImportant": {
    message: "Importante",
    description: "Urgency option",
  },
  "notifications.pushUrgencyCritical": {
    message: "Crítica",
    description: "Urgency option",
  },
  "notifications.pushTimezone": {
    message: "Zona horaria IANA",
    description: "Timezone field",
  },
  "notifications.pushTimezonePlaceholder": {
    message: "America/Los_Angeles",
    description: "Timezone placeholder",
  },
  "notifications.pushMaxDeferral": {
    message: "Aplazamiento máximo (horas)",
    description: "Deferral field",
  },
  "notifications.pushWorkingHours": {
    message: "Horario laboral",
    description: "Working hours heading",
  },
  "notifications.pushWorkingHoursHint": {
    message:
      "Si se define, las alertas no críticas esperan hasta uno de estos intervalos.",
    description: "Working hours hint",
  },
  "notifications.pushQuietHours": {
    message: "Horas de silencio",
    description: "Quiet hours heading",
  },
  "notifications.pushQuietHoursHint": {
    message:
      "Las alertas no críticas esperan mientras haya un intervalo de silencio activo.",
    description: "Quiet hours hint",
  },
  "notifications.weekdayMonday": {
    message: "Lun",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayTuesday": {
    message: "Mar",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayWednesday": {
    message: "Mié",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayThursday": {
    message: "Jue",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayFriday": {
    message: "Vie",
    description: "Weekday abbreviation",
  },
  "notifications.weekdaySaturday": {
    message: "Sáb",
    description: "Weekday abbreviation",
  },
  "notifications.weekdaySunday": {
    message: "Dom",
    description: "Weekday abbreviation",
  },
  "notifications.pushRangeStart": {
    message: "Inicio del intervalo",
    description: "Accessible time input label",
  },
  "notifications.pushRangeEnd": {
    message: "Fin del intervalo",
    description: "Accessible time input label",
  },
  "notifications.pushRemoveRange": {
    message: "Quitar intervalo de tiempo",
    description: "Accessible remove button label",
  },
  "notifications.pushAddRange": {
    message: "Añadir intervalo de tiempo",
    description: "Add range button",
  },
  "notifications.pushOverrides": {
    message: "Excepciones por servicio",
    description: "Overrides heading",
  },
  "notifications.pushOverridesHint": {
    message:
      "Los campos omitidos heredan la configuración de arriba; una lista de eventos vacía silencia ese servicio.",
    description: "Override semantics hint",
  },
  "notifications.pushRemoveOverride": {
    message: "Quitar excepción del servicio",
    description: "Accessible remove override label",
  },
  "notifications.pushOverrideEnabled": {
    message: "Entrega",
    description: "Override enabled field",
  },
  "notifications.pushInherit": {
    message: "Heredar",
    description: "Override option",
  },
  "notifications.pushOn": {
    message: "Activada",
    description: "Override option",
  },
  "notifications.pushOff": {
    message: "Desactivada",
    description: "Override option",
  },
  "notifications.pushExactEvents": {
    message: "Anular con una lista exacta de eventos",
    description: "Exact event filter toggle",
  },
  "notifications.pushAddOverride": {
    message: "Añadir una excepción de servicio…",
    description: "Add override selector",
  },
  "notifications.pushSave": {
    message: "Guardar configuración de push",
    description: "Save button",
  },
  "notifications.pushUnsaved": {
    message: "Cambios sin guardar",
    description:
      "Hint beside Save push settings while the push form differs from the saved settings (w4/435)",
  },
  "notifications.pushDiscardTitle": {
    message: "¿Descartar los cambios en la configuración de push?",
    description:
      "Navigation guard title when the push form has unsaved changes (w4/435)",
  },
  "notifications.pushDiscardBody": {
    message: "Estos cambios no se han guardado y se perderán si te vas.",
    description: "Navigation guard explanation for unsaved push settings",
  },
  "notifications.pushKeepEditing": {
    message: "Seguir editando",
    description: "Cancel the navigation and stay on the push form",
  },
  "notifications.pushDiscard": {
    message: "Descartar y salir",
    description: "Confirm leaving and dropping unsaved push settings",
  },
  "notifications.pushSaved": {
    message: "Configuración de push guardada",
    description: "Success toast",
  },
  "notifications.pushUpdateError": {
    message: "No se pudo guardar la configuración de push",
    description: "Failure toast",
  },
  "notifications.pushErrorTitle": {
    message: "No se pudo cargar la configuración de push",
    description: "Push load error title",
  },
  "notifications.pushInvalidTimezone": {
    message:
      "Introduce una zona horaria IANA válida, como America/Los_Angeles.",
    description: "Timezone validation",
  },
  "notifications.pushInvalidDeferral": {
    message: "El aplazamiento máximo debe estar entre 0.5 y 168 horas.",
    description: "Deferral validation",
  },
  "notifications.pushInvalidEvents": {
    message: "El filtro de eventos contiene un valor no admitido.",
    description: "Event validation",
  },
  "notifications.pushInvalidUrgency": {
    message: "Selecciona una urgencia válida.",
    description: "Urgency validation",
  },
  "notifications.pushInvalidRange": {
    message:
      "Cada intervalo de tiempo necesita días de la semana, horas de inicio y fin válidas, y extremos distintos.",
    description: "Range validation",
  },
  "notifications.pushTooManyRules": {
    message: "Hay demasiados intervalos de horario o excepciones de servicio.",
    description: "Rule count validation",
  },
  "notifications.pushInvalidService": {
    message:
      "Las excepciones de servicio deben ser únicas y hacer referencia a un servicio válido.",
    description: "Service validation",
  },
  "notifications.pushEmptyOverride": {
    message:
      "Cada excepción de servicio debe cambiar la entrega, la urgencia o los eventos.",
    description: "Empty override validation",
  },
  "notifications.workspaceTitle": {
    message: "Notificaciones",
    description:
      "Workspace Settings → Notifications card title (the workspace notification policy)",
  },
  "notifications.workspaceDescription": {
    message:
      "Valores predeterminados para todos los servicios. Un servicio puede anularlos en su configuración.",
    description: "Workspace Settings → Notifications card description",
  },
  "notifications.workspaceReadOnly": {
    message:
      "Solo los administradores del espacio de trabajo pueden cambiar estos valores predeterminados.",
    description: "Workspace notification policy: read-only note for non-admins",
  },
  "notifications.workspaceEmailLabel": {
    message: "Notificaciones por correo",
    description: "Workspace notification policy: email destination switch",
  },
  "notifications.workspaceEmailHint": {
    message:
      "Si está desactivado, no se envía ningún correo de notificación, sin importar la configuración de cada servicio.",
    description: "Workspace notification policy: email destination hint",
  },
  "notifications.workspaceLevelLabel": {
    message: "Notificaciones predeterminadas de los servicios",
    description: "Workspace notification policy: level radio group label",
  },
  "notifications.workspaceLevelHint": {
    message:
      "Lo que envían los servicios que usan el valor predeterminado del espacio de trabajo.",
    description: "Workspace notification policy: level radio group hint",
  },
  "notifications.levelAll": {
    message: "Todas las notificaciones",
    description: "Notification level: every notification",
  },
  "notifications.levelAllHint": {
    message:
      "Fallos, además de despliegues que comienzan y pasan a estar activos, y recuperaciones.",
    description: "Notification level hint: all",
  },
  "notifications.levelFailure": {
    message: "Solo notificaciones de fallos",
    description: "Notification level: failures only (Render's default)",
  },
  "notifications.levelFailureHint": {
    message:
      "Despliegues fallidos, servicios en mal estado, ejecuciones de cron fallidas y discos llenos.",
    description: "Notification level hint: failure",
  },
  "notifications.levelNone": {
    message: "Ninguna",
    description: "Notification level: nothing",
  },
  "notifications.levelNoneHint": {
    message: "No enviar notificaciones de estos servicios.",
    description: "Notification level hint: none",
  },
  "notifications.workspaceSave": {
    message: "Guardar",
    description: "Workspace notification policy: save button",
  },
  "notifications.workspaceSaved": {
    message: "Valores predeterminados de notificaciones guardados",
    description: "Workspace notification policy: success toast",
  },
  "notifications.workspaceSaveFailed": {
    message:
      "No se pudieron guardar los valores predeterminados de notificaciones. Inténtalo de nuevo.",
    description: "Workspace notification policy: save error",
  },
  "notifications.workspaceLoadFailed": {
    message:
      "No se pudieron cargar los valores predeterminados de notificaciones del espacio de trabajo.",
    description: "Workspace notification policy: load error",
  },
  "notifications.workspaceRetry": {
    message: "Reintentar",
    description: "Workspace notification policy: retry button",
  },
  "notifications.overridesTitle": {
    message: "Excepciones de notificaciones",
    description:
      "Workspace notifications: overrides list title (Render's label)",
  },
  "notifications.overridesDescription": {
    message:
      "Servicios cuya configuración de notificaciones difiere del valor predeterminado del espacio de trabajo.",
    description: "Workspace notifications: overrides list description",
  },
  "notifications.overridesEmpty": {
    message:
      "Ningún servicio anula el valor predeterminado del espacio de trabajo.",
    description: "Workspace notifications: no overrides",
  },
  "notifications.overridesError": {
    message:
      "No se pudieron cargar los servicios que anulan el valor predeterminado.",
    description: "Workspace notifications: overrides failed to load",
  },
};

export default esNotifications;
