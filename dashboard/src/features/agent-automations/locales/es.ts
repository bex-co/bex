import type { TranslationEntry } from "@/i18n";

const esAgentAutomations: Record<string, TranslationEntry> = {
  "agentAutomations.pageTitle": {
    message: "Automatizaciones",
    description:
      "Page title and sidebar label of the scheduled agent automations page",
  },
  "agentAutomations.detailTitle": {
    message: "Automatización",
    description: "Browser tab title of one automation's page",
  },
  "agentAutomations.pageDescription": {
    message:
      "Inicia una sesión de agente según una programación. Cada ejecución usa la tarea y la configuración que guardes aquí.",
    description: "Automations list page description",
  },
  "agentAutomations.create": {
    message: "Crear automatización",
    description:
      "Create automation button on the list page and in the empty state",
  },
  "agentAutomations.createTitle": {
    message: "Crear automatización",
    description: "Heading of the create form",
  },
  "agentAutomations.editTitle": {
    message: "Editar automatización",
    description: "Heading of the edit form",
  },
  "agentAutomations.createSubmit": {
    message: "Crear automatización",
    description: "Create form submit button",
  },
  "agentAutomations.editSubmit": {
    message: "Guardar cambios",
    description: "Edit form submit button",
  },
  "agentAutomations.createDenied": {
    message:
      "Solo los miembros con el rol de desarrollador o superior pueden crear automatizaciones.",
    description: "Create page notice for a member who cannot create",
  },
  "agentAutomations.backToList": {
    message: "Volver a las automatizaciones",
    description: "Accessible label of the back link to the automations list",
  },
  "agentAutomations.backToAutomation": {
    message: "Volver a la automatización",
    description: "Button returning from the edit page to the automation's page",
  },
  "agentAutomations.tabMine": {
    message: "Mías",
    description: "List tab: automations the viewer created",
  },
  "agentAutomations.tabAll": {
    message: "Todas",
    description: "List tab: every automation in the workspace",
  },
  "agentAutomations.emptyMineTitle": {
    message: "Aún no tienes automatizaciones",
    description: "Empty state title on the Mine tab",
  },
  "agentAutomations.emptyAllTitle": {
    message: "Este espacio de trabajo aún no tiene automatizaciones",
    description: "Empty state title on the All tab",
  },
  "agentAutomations.emptyBody": {
    message:
      "Una automatización inicia una sesión de agente según una programación, por ejemplo cada día laborable a las 09:00, con la tarea, el repositorio y los límites que elijas.",
    description: "Empty state explanation",
  },
  "agentAutomations.loadMore": {
    message: "Cargar más",
    description: "Load the next page of automations or runs",
  },
  "agentAutomations.loadFailed": {
    message: "No se pudieron cargar las automatizaciones.",
    description: "Load error",
  },
  "agentAutomations.retry": {
    message: "Reintentar",
    description: "Retry button on a load error",
  },
  "agentAutomations.forbidden": {
    message:
      "Solo los miembros con el rol de desarrollador o superior pueden usar las automatizaciones.",
    description: "Load error for a member without access",
  },
  "agentAutomations.notFound": {
    message: "Esta automatización no existe o se eliminó.",
    description: "Load error for a missing automation",
  },
  "agentAutomations.noTrigger": {
    message: "Sin programación",
    description: "Row text when an automation has no schedule",
  },
  "agentAutomations.nextRun": {
    message: "Próxima ejecución",
    description: "Label before the next run time",
  },
  "agentAutomations.lastRun": {
    message: "Última ejecución",
    description: "Label before the last run time",
  },
  "agentAutomations.neverRun": {
    message: "nunca",
    description: "Last run value when it has not run",
  },
  "agentAutomations.lastSession": {
    message: "Ver sesión",
    description: "Link to the session the last run started",
  },
  "agentAutomations.pauseLabel": {
    message: "Pausar {name}",
    description: "Accessible name of the switch that pauses an automation",
  },
  "agentAutomations.resumeLabel": {
    message: "Reanudar {name}",
    description: "Accessible name of the switch that resumes an automation",
  },
  "agentAutomations.pausedBadge": {
    message: "En pausa",
    description: "Badge on an automation a person paused",
  },
  "agentAutomations.ended": {
    message: "Finalizada",
    description: "Badge on an automation with no future run",
  },
  "agentAutomations.active": {
    message: "Activa",
    description: "Status of a running automation",
  },
  "agentAutomations.paused.repeated_failures": {
    message: "En pausa: fallos repetidos",
    description: "Badge: paused by the platform after repeated failed runs",
  },
  "agentAutomations.paused.creator_lost_access": {
    message: "En pausa: el creador perdió el acceso",
    description:
      "Badge: paused because the creator can no longer start sessions",
  },
  "agentAutomations.paused.template_invalid": {
    message: "En pausa: plantilla no válida",
    description:
      "Badge: paused because the session template no longer validates",
  },
  "agentAutomations.paused.unknown": {
    message: "En pausa por la plataforma",
    description: "Badge for a pause reason this version does not know",
  },
  "agentAutomations.pausedExplain.repeated_failures": {
    message:
      "Esta automatización se pausó tras {failures} ejecuciones fallidas seguidas. Revisa el historial de ejecuciones, corrige la causa y reanúdala.",
    description: "Explanation on the detail page for a repeated-failures pause",
  },
  "agentAutomations.pausedExplain.creator_lost_access": {
    message:
      "El miembro que creó esta automatización ya no tiene acceso para iniciar sesiones. Un administrador del espacio de trabajo puede hacerse cargo de ella, lo que la reanuda.",
    description:
      "Explanation on the detail page for a creator-lost-access pause",
  },
  "agentAutomations.pausedExplain.template_invalid": {
    message:
      "La plantilla de la sesión ya no supera la validación, por ejemplo porque cambió un secreto o un ajuste del modelo. Su creador puede editarla y luego reanudarla.",
    description: "Explanation on the detail page for a template-invalid pause",
  },
  "agentAutomations.pausedExplain.unknown": {
    message:
      "La plataforma pausó esta automatización. Reanúdala cuando se haya corregido la causa.",
    description: "Explanation for a pause reason this version does not know",
  },
  "agentAutomations.pausedToast": {
    message: "{name} en pausa.",
    description: "Toast after pausing an automation",
  },
  "agentAutomations.resumedToast": {
    message: "{name} reanudada.",
    description: "Toast after resuming an automation",
  },
  "agentAutomations.runStartedToast": {
    message: "{name} inició una sesión.",
    description: "Toast after Run now started a session",
  },
  "agentAutomations.runSettledToast": {
    message: "Se ejecutó {name}: {outcome}.",
    description: "Toast after Run now ended with an outcome other than started",
  },
  "agentAutomations.tookOverToast": {
    message: "Ahora eres propietario de {name}.",
    description: "Toast after an admin takes an automation over",
  },
  "agentAutomations.deletedToast": {
    message: "{name} eliminada.",
    description: "Toast after deleting an automation",
  },
  "agentAutomations.createdToast": {
    message: "{name} creada.",
    description: "Toast after creating an automation",
  },
  "agentAutomations.updatedToast": {
    message: "{name} guardada.",
    description: "Toast after editing an automation",
  },
  "agentAutomations.actionFailed": {
    message: "No funcionó. Inténtalo de nuevo.",
    description: "Generic failure toast for an automation action",
  },
  "agentAutomations.saveFailed": {
    message: "No se pudo guardar la automatización. Inténtalo de nuevo.",
    description: "Generic failure message when saving the form",
  },
  "agentAutomations.actionsLabel": {
    message: "Acciones de la automatización",
    description: "Accessible label of the action button group",
  },
  "agentAutomations.runNow": {
    message: "Ejecutar ahora",
    description: "Button: fire the automation now",
  },
  "agentAutomations.runNowPausedHint": {
    message: "Reanuda esta automatización antes de ejecutarla.",
    description:
      "Tooltip on Run now while the automation is paused by the platform",
  },
  "agentAutomations.pause": {
    message: "Pausar",
    description: "Button: pause the automation",
  },
  "agentAutomations.resume": {
    message: "Reanudar",
    description: "Button: resume the automation",
  },
  "agentAutomations.edit": {
    message: "Editar",
    description: "Button: edit the automation",
  },
  "agentAutomations.editCreatorOnly": {
    message: "Solo el miembro que creó esta automatización puede editarla.",
    description: "Tooltip and notice: editing is for the creator only",
  },
  "agentAutomations.takeOver": {
    message: "Hacerme cargo",
    description: "Button: an admin becomes the automation's creator",
  },
  "agentAutomations.takeOverAdminOnly": {
    message:
      "Solo un administrador del espacio de trabajo que no haya creado esta automatización puede hacerse cargo de ella.",
    description: "Tooltip on a disabled Take over button",
  },
  "agentAutomations.takeOverTitle": {
    message: "¿Hacerte cargo de {name}?",
    description: "Take over confirmation title",
  },
  "agentAutomations.takeOverBody": {
    message:
      "Las ejecuciones futuras iniciarán sesiones como tú, con tu acceso, y la plantilla se comprobará de nuevo con tu acceso. Una automatización pausada porque su creador perdió el acceso se reanuda.",
    description: "Take over confirmation body",
  },
  "agentAutomations.delete": {
    message: "Eliminar",
    description: "Button: delete the automation",
  },
  "agentAutomations.deleteTitle": {
    message: "¿Eliminar {name}?",
    description: "Delete confirmation title",
  },
  "agentAutomations.deleteBody": {
    message:
      "Se eliminan la automatización y su historial de ejecuciones. Las sesiones que ya inició se conservan.",
    description: "Delete confirmation body",
  },
  "agentAutomations.settingsTitle": {
    message: "Configuración",
    description: "Settings card title on the detail page",
  },
  "agentAutomations.row.status": {
    message: "Estado",
    description: "Settings row label",
  },
  "agentAutomations.row.schedule": {
    message: "Programación",
    description: "Settings row label",
  },
  "agentAutomations.row.nextRun": {
    message: "Próxima ejecución",
    description: "Settings row label",
  },
  "agentAutomations.row.lastRun": {
    message: "Última ejecución",
    description: "Settings row label",
  },
  "agentAutomations.row.createdBy": {
    message: "Creada por",
    description: "Settings row label",
  },
  "agentAutomations.row.repo": {
    message: "Repositorio",
    description: "Settings row label",
  },
  "agentAutomations.row.agent": {
    message: "Agente",
    description: "Settings row label",
  },
  "agentAutomations.row.spendLimit": {
    message: "Límite de gasto por ejecución",
    description: "Settings row label",
  },
  "agentAutomations.row.emailNotifications": {
    message: "Notificaciones por correo",
    description: "Settings row label",
  },
  "agentAutomations.row.secrets": {
    message: "Secretos",
    description: "Settings row label",
  },
  "agentAutomations.row.skills": {
    message: "Skills",
    description: "Settings row label",
  },
  "agentAutomations.taskTitle": {
    message: "Tarea",
    description: "Task card title on the detail page",
  },
  "agentAutomations.noRepo": {
    message: "Ninguno (sesión solo de chat)",
    description: "Repository value when the template has none",
  },
  "agentAutomations.noSpendLimit": {
    message: "Sin límite",
    description: "Spend limit value when the template has none",
  },
  "agentAutomations.none": {
    message: "Ninguno",
    description: "Empty value for secrets or skills",
  },
  "agentAutomations.runsTitle": {
    message: "Historial de ejecuciones",
    description: "Run history card title",
  },
  "agentAutomations.runsDescription": {
    message:
      "Todas las ejecuciones programadas y manuales, las más recientes primero.",
    description: "Run history card description",
  },
  "agentAutomations.runsEmpty": {
    message: "Aún no hay ejecuciones.",
    description: "Run history empty state",
  },
  "agentAutomations.runs.fireAt": {
    message: "Hora de disparo",
    description: "Run table column",
  },
  "agentAutomations.runs.outcome": {
    message: "Resultado",
    description: "Run table column",
  },
  "agentAutomations.runs.session": {
    message: "Sesión",
    description: "Run table column",
  },
  "agentAutomations.runs.missed": {
    message: "Omitidas",
    description: "Run table column: ticks skipped while the scheduler was down",
  },
  "agentAutomations.runs.error": {
    message: "Error",
    description: "Run table column",
  },
  "agentAutomations.runs.openSession": {
    message: "Abrir sesión",
    description: "Link to the session a run started",
  },
  "agentAutomations.outcome.started": {
    message: "Iniciada",
    description: "Run outcome: a session was started",
  },
  "agentAutomations.outcome.claimed": {
    message: "En curso",
    description: "Run outcome: claimed, not yet settled",
  },
  "agentAutomations.outcome.skipped_overlap": {
    message: "Omitida",
    description:
      "Run outcome: skipped because the previous session was still running",
  },
  "agentAutomations.outcome.failed_create": {
    message: "Fallida",
    description: "Run outcome: the session could not be created",
  },
  "agentAutomations.outcome.denied": {
    message: "Denegada",
    description:
      "Run outcome: the creator was no longer allowed to start the session",
  },
  "agentAutomations.outcome.didNotStart": {
    message: "No se inició",
    description:
      "Run outcome: the run started a session whose sandbox did not start for a cause on bex's side (w10/m497); the cause shows in the Error column. Not a failed create",
  },
  "agentAutomations.outcome.unknown": {
    message: "Desconocido",
    description: "Run outcome this version does not know",
  },
  "agentAutomations.section.details": {
    message: "Detalles",
    description: "Form section title",
  },
  "agentAutomations.section.schedule": {
    message: "Programación",
    description: "Form section title",
  },
  "agentAutomations.section.scheduleDescription": {
    message:
      "Cuándo se ejecuta la automatización. La zona horaria determina qué significan las 09:00.",
    description: "Form section description",
  },
  "agentAutomations.section.session": {
    message: "Sesión",
    description: "Form section title",
  },
  "agentAutomations.section.sessionDescription": {
    message:
      "La sesión de agente que inicia cada ejecución, con las mismas opciones que una sesión nueva en la página Agentes.",
    description: "Form section description",
  },
  "agentAutomations.field.name": {
    message: "Nombre",
    description: "Form field label",
  },
  "agentAutomations.field.frequency": {
    message: "Repetir",
    description: "Form field label",
  },
  "agentAutomations.field.minute": {
    message: "Minuto de la hora",
    description: "Form field label",
  },
  "agentAutomations.field.at": {
    message: "Hora",
    description: "Form field label",
  },
  "agentAutomations.field.weekdays": {
    message: "Días",
    description: "Form field label",
  },
  "agentAutomations.field.runAt": {
    message: "Fecha y hora",
    description: "Form field label",
  },
  "agentAutomations.field.cron": {
    message: "Expresión cron",
    description: "Form field label",
  },
  "agentAutomations.field.timezone": {
    message: "Zona horaria",
    description: "Form field label",
  },
  "agentAutomations.field.endsOn": {
    message: "Termina el (opcional)",
    description: "Form field label",
  },
  "agentAutomations.field.task": {
    message: "Tarea",
    description: "Form field label",
  },
  "agentAutomations.field.repo": {
    message: "Repositorio",
    description: "Form field label",
  },
  "agentAutomations.field.agent": {
    message: "Agente",
    description: "Form field label",
  },
  "agentAutomations.field.spendLimit": {
    message: "Límite de gasto por ejecución (USD)",
    description: "Form field label",
  },
  "agentAutomations.field.emailNotifications": {
    message: "Notificaciones por correo",
    description: "Form field label",
  },
  "agentAutomations.field.secrets": {
    message: "Secretos",
    description: "Form field label",
  },
  "agentAutomations.field.skills": {
    message: "Skills",
    description: "Form field label",
  },
  "agentAutomations.frequency.hour": {
    message: "Cada hora",
    description: "Frequency option",
  },
  "agentAutomations.frequency.day": {
    message: "Cada día",
    description: "Frequency option",
  },
  "agentAutomations.frequency.week": {
    message: "Cada semana",
    description: "Frequency option",
  },
  "agentAutomations.frequency.once": {
    message: "Una vez",
    description: "Frequency option",
  },
  "agentAutomations.frequency.custom": {
    message: "Personalizada (cron)",
    description: "Frequency option",
  },
  "agentAutomations.emailNotifications.always": {
    message: "Siempre",
    description: "Email notification option: every run emails the creator",
  },
  "agentAutomations.emailNotifications.on_failure": {
    message: "En caso de fallo",
    description:
      "Email notification option: only failed runs email the creator",
  },
  "agentAutomations.emailNotifications.on_success": {
    message: "En caso de éxito",
    description:
      "Email notification option: only succeeded runs email the creator",
  },
  "agentAutomations.emailNotifications.never": {
    message: "Nunca",
    description: "Email notification option: no run emails the creator",
  },
  "agentAutomations.weekday.mon": {
    message: "Lun",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.tue": {
    message: "Mar",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.wed": {
    message: "Mié",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.thu": {
    message: "Jue",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.fri": {
    message: "Vie",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.sat": {
    message: "Sáb",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.sun": {
    message: "Dom",
    description: "Weekday toggle",
  },
  "agentAutomations.help.name": {
    message:
      "Letras minúsculas, dígitos y guiones simples, hasta 64 caracteres.",
    description: "Name field help",
  },
  "agentAutomations.help.minute": {
    message: "De 0 a 59.",
    description: "Minute field help",
  },
  "agentAutomations.help.runAt": {
    message: "Se interpreta en la zona horaria elegida abajo ({zone}).",
    description: "Run-at field help",
  },
  "agentAutomations.help.cron": {
    message:
      "Cinco campos: minuto hora día-del-mes mes día-de-la-semana. Las ejecuciones no pueden estar a menos de 5 minutos de distancia.",
    description: "Cron field help",
  },
  "agentAutomations.help.endsOn": {
    message:
      "El último día en que puede ejecutarse. Déjalo vacío para que se ejecute hasta que lo pauses.",
    description: "Ends-on field help",
  },
  "agentAutomations.help.task": {
    message:
      "Lo que el agente debe hacer en cada ejecución. Menciona el resultado que esperas.",
    description: "Task field help",
  },
  "agentAutomations.help.repo": {
    message:
      "Opcional. Sin repositorio, cada ejecución es una sesión solo de chat. Cada ejecución trabaja en una rama bex-agent nueva.",
    description: "Repository field help",
  },
  "agentAutomations.help.spendLimit": {
    message:
      "Un tope de gasto del modelo para cada ejecución. Obligatorio cuando las ejecuciones usan el modelo de la plataforma.",
    description: "Spend limit field help",
  },
  "agentAutomations.help.emailNotifications": {
    message:
      "Qué ejecuciones te envían un correo. Una pausa de la plataforma siempre lo hace.",
    description: "Email notifications field help",
  },
  "agentAutomations.help.secrets": {
    message:
      "Secretos del espacio de trabajo que el agente recibe como variables de entorno. Añádelos en la configuración del espacio de trabajo.",
    description: "Secrets field help",
  },
  "agentAutomations.noSecrets": {
    message: "Sin secretos",
    description: "Secrets row when none are chosen",
  },
  "agentAutomations.noSkills": {
    message: "Sin skills",
    description: "Skills row when none are chosen",
  },
  "agentAutomations.placeholder.name": {
    message: "nightly-dependency-check",
    description: "Name field placeholder (an example slug)",
  },
  "agentAutomations.placeholder.repo": {
    message: "owner/repository",
    description: "Repository field placeholder",
  },
  "agentAutomations.summary": {
    message: "Se ejecuta: {schedule}",
    description: "Live summary under the schedule picker",
  },
  "agentAutomations.schedule.hour": {
    message: "Cada hora en el minuto :{minute}",
    description: "Schedule in words, hourly",
  },
  "agentAutomations.schedule.day": {
    message: "Cada día a las {at}",
    description: "Schedule in words, daily",
  },
  "agentAutomations.schedule.week": {
    message: "Cada {days} a las {at}",
    description: "Schedule in words, weekly",
  },
  "agentAutomations.schedule.once": {
    message: "Una vez el {when}",
    description: "Schedule in words, one-off",
  },
  "agentAutomations.schedule.custom": {
    message: "Cron {cron}",
    description: "Schedule in words, cron",
  },
  "agentAutomations.schedule.until": {
    message: "hasta el {date}",
    description: "Schedule end date suffix",
  },
  "agentAutomations.problem.nameRequired": {
    message: "Escribe un nombre.",
    description: "Form validation",
  },
  "agentAutomations.problem.nameInvalid": {
    message:
      "Usa letras minúsculas, dígitos y guiones simples, hasta 64 caracteres.",
    description: "Form validation",
  },
  "agentAutomations.problem.taskRequired": {
    message: "Describe la tarea.",
    description: "Form validation",
  },
  "agentAutomations.problem.spendLimit": {
    message: "Escribe una cantidad mayor que 0 y hasta 100000, o déjalo vacío.",
    description: "Form validation",
  },
  "agentAutomations.problem.skills": {
    message:
      "Los argumentos de un skill deben ser una sola línea de 4 KB como máximo.",
    description: "Form validation",
  },
  "agentAutomations.problem.timezone": {
    message: "Elige una zona horaria válida.",
    description: "Form validation",
  },
  "agentAutomations.problem.minute": {
    message: "Escribe un minuto de 0 a 59.",
    description: "Form validation",
  },
  "agentAutomations.problem.at": {
    message: "Escribe una hora.",
    description: "Form validation",
  },
  "agentAutomations.problem.weekdays": {
    message: "Elige al menos un día.",
    description: "Form validation",
  },
  "agentAutomations.problem.runAtRequired": {
    message: "Elige una fecha y una hora.",
    description: "Form validation",
  },
  "agentAutomations.problem.runAtPast": {
    message: "Elige una hora en el futuro.",
    description: "Form validation",
  },
  "agentAutomations.problem.cron": {
    message:
      "Escribe cinco campos: minuto hora día-del-mes mes día-de-la-semana.",
    description: "Form validation",
  },
  "agentAutomations.problem.endsOn": {
    message: "Escribe una fecha válida.",
    description: "Form validation",
  },
  "agentAutomations.problem.endsOnPast": {
    message: "Elige una fecha en el futuro.",
    description: "Form validation",
  },
  "agentAutomations.startedBy": {
    message: "Iniciada por una automatización",
    description: "Session page header note linking back to the automation",
  },
};

export default esAgentAutomations;
