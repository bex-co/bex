import type { TranslationEntry } from "@/i18n";

const esLogs: Record<string, TranslationEntry> = {
  "logs.rangeLabel": {
    message: "Intervalo del historial de logs",
    description: "Accessible label for the relative log-history range controls",
  },
  "logs.rangeHistoryNote": {
    message:
      "El intervalo limita el historial. El modo en vivo añade las líneas nuevas a medida que llegan.",
    description:
      "Explains how the bounded history window interacts with live tail",
  },
  "logs.retentionWindow_one": {
    message: "Los logs se conservan {count} día en el plan {plan}.",
    description:
      "Log range picker hint: how many days the workspace plan keeps logs (w9/m389, from bex-api logRetention)",
  },
  "logs.retentionWindow_many": {
    message: "Los logs se conservan {count} de días en el plan {plan}.",
    description:
      "Log range picker hint: how many days the workspace plan keeps logs (w9/m389, from bex-api logRetention)",
  },
  "logs.retentionWindow_other": {
    message: "Los logs se conservan {count} días en el plan {plan}.",
    description:
      "Log range picker hint: how many days the workspace plan keeps logs (w9/m389, from bex-api logRetention)",
  },
  "logs.retentionUpgrade_one": {
    message: "Cambia al plan {plan} para conservar {count} día.",
    description:
      "Log range picker hint: the cheapest plan that keeps logs longer, and its days (w9/m389)",
  },
  "logs.retentionUpgrade_many": {
    message: "Cambia al plan {plan} para conservar {count} de días.",
    description:
      "Log range picker hint: the cheapest plan that keeps logs longer, and its days (w9/m389)",
  },
  "logs.retentionUpgrade_other": {
    message: "Cambia al plan {plan} para conservar {count} días.",
    description:
      "Log range picker hint: the cheapest plan that keeps logs longer, and its days (w9/m389)",
  },
  "logs.typeLabel": {
    message: "Tipo de log",
    description: "Accessible label for the log-type filter dropdown",
  },
  "logs.typeAll": {
    message: "Todos los logs",
    description: "Log-type filter option: no type filter",
  },
  "logs.typeApplication": {
    message: "Logs de la aplicación",
    description: "Log-type filter option: application (app) logs",
  },
  "logs.typeRequest": {
    message: "Logs de solicitudes",
    description:
      "Log-type filter option: request logs (Traefik access lines, from the durable store)",
  },
  "logs.levelLabel": {
    message: "Nivel",
    description: "Accessible label for the log-level filter dropdown",
  },
  "logs.levelAll": {
    message: "Todos los niveles",
    description: "Level filter option: no level filter",
  },
  "logs.methodLabel": {
    message: "Método",
    description: "Accessible label for the HTTP-method filter dropdown",
  },
  "logs.methodAll": {
    message: "Todos los métodos",
    description: "Method filter option: no method filter",
  },
  "logs.statusCodeLabel": {
    message: "Código de estado",
    description: "Accessible label for the HTTP-status filter dropdown",
  },
  "logs.statusCodeAll": {
    message: "Todos los estados",
    description: "Status-code filter option: no status filter",
  },
  "logs.instanceLabel": {
    message: "Instancia",
    description: "Accessible label for the instance (replica) filter dropdown",
  },
  "logs.instanceAll": {
    message: "Todas las instancias",
    description: "Instance filter option: no instance filter",
  },
  "logs.filterByInstance": {
    message: "Filtrar logs por la instancia {instance}",
    description:
      "Accessible label for the clickable instance slug on an application log line",
  },
  "logs.pathLabel": {
    message: "Ruta de la solicitud",
    description: "Accessible label for the request-path filter input",
  },
  "logs.filtersButton": {
    message: "Filtros",
    description:
      "Trigger for the structured-filters popover (level/method/status/instance/path)",
  },
  "logs.chipRemove": {
    message: "Quitar el filtro {label}",
    description:
      "Accessible label for an active-filter chip's clear button; {label} is the filter's field name",
  },
  "logs.pathPlaceholder": {
    message: "Filtrar por ruta",
    description: "Placeholder for the request-path filter input",
  },
  "logs.hostLabel": {
    message: "Host de la solicitud",
    description:
      "Accessible label for the request-host filter input and its chip (Render's host log filter, w4/538)",
  },
  "logs.hostPlaceholder": {
    message: "Filtrar por host",
    description: "Placeholder for the request-host filter input",
  },
  "logs.liveUnsupported": {
    message:
      "El seguimiento en vivo está desactivado: el método, el estado, la ruta y el host filtran logs de solicitudes, por lo que los logs de la aplicación no se pueden seguir con ellos.",
    description:
      "Note when the chosen log type cannot carry an active filter, so live tail is off (a request filter on app logs)",
  },
  "logs.liveWindowClosed": {
    message:
      "El seguimiento en vivo está desactivado: este intervalo personalizado ya terminó, por lo que no pueden llegar líneas nuevas.",
    description:
      "Note when the selected custom time range ended before it was applied, so live tail is off",
  },
  "logs.storeRequiredTitle": {
    message: "Los logs de solicitudes necesitan el almacén de logs",
    description:
      "Empty-state title when a request/structured-filter query hits a deployment with no durable store (503)",
  },
  "logs.storeRequiredBody": {
    message:
      "Los logs de solicitudes y los filtros de nivel, estado, método y ruta leen del almacén de logs duradero, que aquí no está configurado. Están disponibles donde el almacén está conectado.",
    description: "Empty-state body for the store-unavailable (503) state",
  },
  "logs.requestPlanTitle": {
    message: "Los logs de solicitudes HTTP requieren el plan Pro",
    description:
      "Notice title in place of Request logs on a Hobby workspace (PLAN_FEATURE_REQUIRED, w8/m138)",
  },
  "logs.requestPlanBody": {
    message:
      "Los logs de solicitudes están disponibles en el plan Pro y superiores. Este espacio de trabajo está en Hobby, por lo que solo muestra logs de la aplicación y de build.",
    description:
      "Notice body in place of Request logs on a Hobby workspace (w8/m138)",
  },
  "logs.searchPlaceholder": {
    message: "Buscar en los logs",
    description: "Placeholder for the log search box (text filter)",
  },
  "logs.live": {
    message: "En vivo",
    description: "Label for the live-tail on/off toggle",
  },
  "logs.jumpToLatest": {
    message: "Ir al último",
    description: "Button to re-pin autoscroll to the newest log line",
  },
  "logs.linesLabel": {
    message: "Líneas de log",
    description:
      "Accessible name of the scrollable log list, which takes keyboard focus so arrow keys and Page Up/Down scroll it",
  },
  "logs.linesKeysHint": {
    message:
      "Usa las flechas arriba y abajo para moverte entre líneas, e Intro para mostrar solo la instancia de esa línea.",
    description:
      "Logs list: screen-reader hint on the focusable log region, naming its row keys",
  },
  "logs.linesKeysShort": {
    message: "↑↓ moverse entre líneas · Intro: solo esta instancia",
    description:
      "Logs tab: the keys of the focused log list, shown in its corner while it has keyboard focus — arrows move between lines, Enter shows only that line's instance",
  },
  "logs.endOfLog": {
    message: "Fin del log",
    description:
      "Closing line after the last row of a log that will not grow (a finished deploy's), so a short log reads as complete",
  },
  "logs.loading": {
    message: "Cargando logs…",
    description: "Shown while the first historical page is loading",
  },
  "logs.loadingOlder": {
    message: "Cargando logs anteriores…",
    description:
      "Shown at the top of the pane while a page-back request is in flight",
  },
  "logs.loadOlder": {
    message: "Cargar anteriores",
    description:
      "Button in the truncation notice that fetches the next older page — the keyboard/underfilled-pane path to what scrolling to the top does",
  },
  "logs.truncatedNotice": {
    message:
      "Solo se ha cargado una parte de este intervalo de tiempo. Carga los anteriores para continuar.",
    description:
      "Banner when hasMore is true: more of the selected range remains unloaded, however many older pages are already shown, so it names no line count (w4/280)",
  },
  "logs.partialSearchNotice": {
    message:
      "Hasta ahora solo se ha buscado en la parte más reciente de este intervalo; carga los anteriores para seguir buscando.",
    description:
      "Banner when a time-boxed search returned less than a page but hasMore is true: the older part of the range is unsearched, not empty (w4/m140)",
  },
  "logs.timeoutTitle": {
    message: "Esta búsqueda tardó demasiado",
    description:
      "State title when the logs query ran out of the server's time budget (QUERY_TIMEOUT, w4/m140)",
  },
  "logs.timeoutBody": {
    message:
      "Prueba con un intervalo de tiempo más corto o un término de búsqueda más específico.",
    description: "State body under logs.timeoutTitle",
  },
  "logs.streaming": {
    message: "En vivo: recibiendo líneas nuevas",
    description: "Status under the log list when the SSE tail is connected",
  },
  "logs.paused": {
    message: "Seguimiento en vivo en pausa",
    description: "Status under the log list when live tail is toggled off",
  },
  "logs.disconnected": {
    message: "Seguimiento en vivo desconectado: reconectando…",
    description: "Banner when the SSE stream drops",
  },
  "logs.emptyTitle": {
    message: "No hay logs en este intervalo de tiempo",
    description:
      "Empty-state title when the selected (always-bounded) history range holds no lines. Never claims the service has never logged — the range can't establish that (w6/m111)",
  },
  "logs.emptyBody": {
    message: "Ninguna línea de log queda dentro de este intervalo de tiempo.",
    description:
      "Empty-state body with no filters applied — honest about the bounded window, not the service's whole history (w6/m111)",
  },
  "logs.emptyFilteredBody": {
    message: "Ningún log coincide con estos filtros.",
    description:
      "Empty-state body when a type/text/structured filter yields nothing",
  },
  "logs.emptyFilteredTitle": {
    message: "Ningún log coincide",
    description:
      "Empty-state title when a type/text/structured filter yields nothing",
  },
  "logs.errorTitle": {
    message: "No se pudieron cargar los logs",
    description: "Error-state title when the logs query fails",
  },
  "logs.errorBody": {
    message:
      "La solicitud para este intervalo falló. Vuelve a intentarlo o elige un intervalo más corto.",
    description:
      "Logs tab: body when the log history read failed, under logs.errorTitle; the server's own reason follows as a detail line",
  },
  "logs.errorBodyLive": {
    message:
      "Las líneas nuevas seguirán apareciendo aquí a medida que lleguen. Vuelve a intentarlo para cargar el historial de este intervalo.",
    description:
      "Logs tab: body when the history read failed while the live tail still streams \u2014 new lines will still land in the panel",
  },
  "logs.filtersDone": {
    message: "Listo",
    description: "Logs tab: button closing the Filters panel",
  },
  "logs.emptyBodyRange": {
    message:
      "Prueba un intervalo más largo o activa En vivo para ver las líneas nuevas.",
    description:
      "Logs tab: body when the chosen range holds no lines, under logs.emptyTitle \u2014 what to try next",
  },
  "logs.turnOnLive": {
    message: "Activar En vivo",
    description:
      "Logs tab: button in the empty state that switches the live tail on",
  },
  "logs.clearFilters": {
    message: "Borrar filtros",
    description:
      "Logs tab: button in the no-match state that clears the search and every filter",
  },
  "logs.errorTitleWithReason": {
    message: "No se pudieron cargar los logs: {reason}",
    description:
      "Inline banner over loaded log lines when a later read was refused; reason is the server's refusal",
  },
  "logs.observabilityViewEyebrow": {
    message: "Vista de observabilidad",
    description:
      "Eyebrow over a project observability view's name (Render's OBSERVABILITY VIEW, w3/m505)",
  },
  "logs.applicationLogsTitle": {
    message: "Logs de aplicación",
    description:
      "Document title part for a project's Application Logs view page (w3/m505)",
  },
  "logs.requestLogsTitle": {
    message: "Logs de solicitudes",
    description:
      "Document title part for a project's Request Logs view page (w3/m505)",
  },
  "logs.projectScopeEmptyTitle": {
    message: "Todavía no hay nada que leer en esta vista",
    description:
      "Project log view state: the view's scope holds no resource to read (w3/m505)",
  },
  "logs.projectScopeEmptyBody": {
    message:
      "Esta vista lee los servicios, cron jobs, bases de datos Postgres e instancias Key Value del proyecto, y todavía no incluye ninguno.",
    description:
      "Project Application Logs view with no service, cron job, Postgres or Key Value in it (w3/m505)",
  },
  "logs.projectRequestScopeEmptyBody": {
    message:
      "Los logs de solicitudes leen los servicios web del proyecto, y todavía no hay ninguno en esta vista.",
    description:
      "Project Request Logs view with no web service in it (w3/m505)",
  },
  "logs.projectEmptyTitle": {
    message: "No hay logs en este intervalo",
    description:
      "Project log view state: its resources logged nothing in the selected range (w3/m505)",
  },
  "logs.projectEmptyBody": {
    message:
      "Ningún recurso de esta vista registró nada en el intervalo seleccionado.",
    description:
      "Project log view state body: no line in the range from any of its resources (w3/m505)",
  },
  "logs.viewNotFoundTitle": {
    message: "Vista no encontrada",
    description:
      "Project observability page whose savedSearchId names no view of this project (w3/m505)",
  },
  "logs.viewNotFoundBody": {
    message:
      "Este proyecto no tiene ninguna vista en este enlace. Puede que se haya eliminado.",
    description: "Body of the view-not-found state (w3/m505)",
  },
  "logs.viewOpen": {
    message: "Abrir {name}",
    description:
      "Link from the view-not-found state to the project's default view of the same type; name is the view's name",
  },
  "logs.viewLoadErrorTitle": {
    message: "No se pudo cargar esta vista",
    description:
      "Project observability page whose saved searches could not be read (w3/m505)",
  },
  "logs.viewLoadErrorBody": {
    message:
      "No se pudieron leer las vistas del proyecto. Recarga la página para volver a intentarlo.",
    description: "Body of the view load failure (w3/m505)",
  },
};

export default esLogs;
