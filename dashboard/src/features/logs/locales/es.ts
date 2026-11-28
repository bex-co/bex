import type { TranslationEntry } from "@/i18n";

const esLogs: Record<string, TranslationEntry> = {
  "logs.rangeLabel": {
    message: "Intervalo del historial de logs",
    description: "Accessible label for the relative log-history range controls",
  },
  "logs.rangeLiveTail": {
    message: "En vivo",
    description:
      "Log explorers (w3/m537): the first entry of the time range picker, Render's Live tail; picking it streams new lines, and every other range is a snapshot",
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
  "logs.typeRequestPlanRequired": {
    message: "Requiere el plan Pro o superior.",
    description:
      "Tooltip on the disabled Request logs option of a web service's Log type dropdown on a plan without request logs (Hobby), Render's words (w3/122); followed by the logs.typeRequestUpgrade link",
  },
  "logs.typeRequestUpgrade": {
    message: "Mejora tu plan",
    description:
      "Link after logs.typeRequestPlanRequired in the disabled Request logs option's tooltip; opens the plan page (Render's words, w3/122)",
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
      "El seguimiento en vivo no está disponible: el método, el estado, la ruta y el host filtran logs de solicitudes, por lo que los logs de la aplicación no se pueden seguir con ellos.",
    description:
      "Note when the chosen log type cannot carry an active filter, so live tail is off (a request filter on app logs)",
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
  "logs.linesLabel": {
    message: "Líneas de log",
    description:
      "Accessible name of the scrollable log list, which takes keyboard focus so arrow keys and Page Up/Down scroll it",
  },
  "logs.linesKeysHint": {
    message:
      "Usa las flechas arriba y abajo para moverte entre líneas, Intro para mostrar solo la instancia de esa línea y Mayús+F10 para abrir su menú.",
    description:
      "Logs tab: screen-reader description of the focused log list's keys - arrows move the row cursor, Enter shows only that line's instance, Shift+F10 opens the line's menu (w3/m528)",
  },
  "logs.linesKeysShort": {
    message:
      "↑↓ moverse entre líneas · Intro: solo esta instancia · Mayús+F10: menú",
    description:
      "Logs tab: the keys of the focused log list, shown in its corner while it has keyboard focus - arrows move between lines, Enter shows only that line's instance, Shift+F10 opens the line's menu",
  },
  "logs.returnToBottom": {
    message: "Volver al final",
    description:
      "Log list (w3/m528): accessible name and tooltip of Render's icon-only Return to Bottom button, shown while the reader is scrolled away from the newest line (ascending order)",
  },
  "logs.returnToTop": {
    message: "Volver al inicio",
    description:
      "Log list (w3/m528): accessible name and tooltip of Render's icon-only Return to Top button, shown while the reader is scrolled away from the newest line (descending order)",
  },
  "logs.newLogs": {
    message: "Logs nuevos",
    description:
      "Log list (w3/m528): Render's held-lines button while a live tail holds fewer than 10 new lines back from a reader scrolled away",
  },
  "logs.newLogsCount_one": {
    message: "{count}+ log nuevo",
    description:
      "Log list (w3/m528): Render's held-lines button for 10 or more held lines; count is the held count rounded up to a multiple of 5, at most 100, followed by a plus sign",
  },
  "logs.newLogsCount_many": {
    message: "{count}+ de logs nuevos",
    description:
      "Log list (w3/m528): Render's held-lines button for 10 or more held lines; count is the held count rounded up to a multiple of 5, at most 100, followed by a plus sign",
  },
  "logs.newLogsCount_other": {
    message: "{count}+ logs nuevos",
    description:
      "Log list (w3/m528): Render's held-lines button for 10 or more held lines; count is the held count rounded up to a multiple of 5, at most 100, followed by a plus sign",
  },
  "logs.newLogsInLiveTail": {
    message: "Hay logs nuevos en vivo",
    description:
      "Log list (w3/m528): Render's button on a preset range when a new line arrived; clicking it picks Live tail (w3/m537)",
  },
  "logs.switchToLiveTail": {
    message: "Cambiar a En vivo",
    description:
      "Log list (w3/m528): tooltip of the New logs in Live Tail button",
  },
  "logs.dismiss": {
    message: "Descartar",
    description:
      "Log list (w3/m528): accessible name of the x that hides the New logs in Live Tail button",
  },
  "logs.lineMenu": {
    message: "Opciones de la línea",
    description:
      "Log list (w3/m528): accessible name and tooltip of the row menu button (...) on a hovered log line",
  },
  "logs.copyLogLine": {
    message: "Copiar línea de log",
    description:
      "Log row menu (w3/m528): copies the line without its time, as Render's Copy log line",
  },
  "logs.lineCopied": {
    message: "¡Copiado!",
    description:
      "Log row menu (w3/m528): toast after Copy log line, Render's 'Copied!'",
  },
  "logs.lineCopyFailed": {
    message: "No se pudo copiar la línea de log",
    description:
      "Log row menu (w3/m528): toast when the clipboard refused the copied line",
  },
  "logs.viewInContext": {
    message: "Ver en contexto",
    description:
      "Log row menu (w3/m528): opens a new tab on the same explorer, unfiltered, 10 minutes either side of the line",
  },
  "logs.sshIntoInstance": {
    message: "Conectarte por SSH a la instancia",
    description:
      "Log row menu (w3/m528): opens the service's Shell on the line's instance in a new tab",
  },
  "logs.sshInstanceNotRunning": {
    message:
      "Solo puedes conectarte por SSH a instancias en ejecución. Esta no está disponible.",
    description:
      "Log row menu (w3/m528): Render's tooltip on a disabled SSH into instance item, for an instance that is no longer running",
  },
  "logs.linesKeysHintMenu": {
    message:
      "Usa las flechas arriba y abajo para moverte entre líneas, y Mayús+F10 para abrir el menú de una línea.",
    description:
      "Log list without an instance filter (w3/m528): screen-reader description of its keys - arrows move the row cursor, Shift+F10 opens the line's menu",
  },
  "logs.linesKeysShortMenu": {
    message: "↑↓ moverse entre líneas · Mayús+F10: menú",
    description:
      "Log list without an instance filter (w3/m528): its keys, shown in its corner while it has keyboard focus",
  },
  "logs.themeSettings": {
    message: "Ajustes de tema",
    description:
      "Log explorer Options menu (w3/m528): Render's last item, a link to the Appearance section of account Settings",
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
      "Prueba un intervalo más largo o cambia a En vivo para ver las líneas nuevas.",
    description:
      "Logs tab: body when the chosen range holds no lines, under logs.emptyTitle \u2014 what to try next",
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
  "logs.newView": {
    message: "Nueva vista",
    description:
      "Project sidebar button that opens the TYPE menu for a new observability view (w3/m507, Render's '+ New View')",
  },
  "logs.newViewTypeHeading": {
    message: "Tipo",
    description:
      "Heading of the New View menu listing view types (Render's TYPE)",
  },
  "logs.editView": {
    message: "Editar vista",
    description:
      "Observability view header button that opens the Edit view modal (w3/m507)",
  },
  "logs.duplicateView": {
    message: "Duplicar",
    description:
      "Observability view header button that copies the view (w3/m507)",
  },
  "logs.deleteView": {
    message: "Eliminar vista",
    description:
      "Accessible name and confirm label of the observability view's trash button (w3/m507)",
  },
  "logs.deleteViewTitle": {
    message: "¿Eliminar {name}?",
    description:
      "Title of the confirmation before deleting an observability view; name is the view's name",
  },
  "logs.deleteViewBody": {
    message:
      "La vista se elimina para todos en el proyecto. Los recursos del proyecto y sus logs no se ven afectados.",
    description:
      "Body of the delete-view confirmation: views are shared by the whole project (w3/m507)",
  },
  "logs.viewDeleted": {
    message: "Se eliminó {name}",
    description:
      "Toast after an observability view was deleted; name is the view's name",
  },
  "logs.deleteViewFailed": {
    message: "No se pudo eliminar la vista",
    description:
      "Toast when deleting an observability view failed without a server reason",
  },
  "logs.viewDuplicated": {
    message: "Se creó {name}",
    description: "Toast after a view was duplicated; name is the copy's name",
  },
  "logs.duplicateViewFailed": {
    message: "No se pudo duplicar la vista",
    description:
      "Toast when duplicating an observability view failed without a server reason",
  },
  "logs.editViewTitle": {
    message: "Editar vista",
    description: "Title of Render's Edit view modal (w3/m507)",
  },
  "logs.newViewTitle": {
    message: "Nueva vista",
    description: "Title of the view modal when New View opened it (w3/m507)",
  },
  "logs.viewNameLabel": {
    message: "Nombre",
    description: "Label of the view name field in the Edit view modal",
  },
  "logs.viewNameRequired": {
    message: "Ponle un nombre a la vista.",
    description: "Inline refusal of a blank view name",
  },
  "logs.viewFiltersLabel": {
    message: "Filtros",
    description: "Heading of the filter rows in the Edit view modal",
  },
  "logs.viewAddFilter": {
    message: "Añadir filtro",
    description: "Accessible name of the Edit view modal's add (+) button",
  },
  "logs.viewNoFilters": {
    message: "Sin filtros: la vista lee todos los recursos del proyecto.",
    description:
      "Edit view modal with no filter rows: the view reads the whole project",
  },
  "logs.viewSharedNotice": {
    message:
      "Editar los filtros actualizará la vista para todos en el proyecto.",
    description:
      "Render's notice under the Edit view filters: a view is shared by the whole project",
  },
  "logs.viewSaveFailed": {
    message: "No se pudo guardar la vista. Inténtalo de nuevo.",
    description:
      "Edit view modal error when saving failed without a server reason",
  },
  "logs.viewClose": {
    message: "Cerrar",
    description:
      "Edit view modal button that discards the changes (Render's Close)",
  },
  "logs.viewDone": {
    message: "Listo",
    description: "Edit view modal button that saves the view (Render's Done)",
  },
  "logs.viewFilterField": {
    message: "Campo",
    description: "Accessible name of a filter row's Field picker",
  },
  "logs.viewFilterOperator": {
    message: "Operador",
    description: "Accessible name of a filter row's Operator picker",
  },
  "logs.viewFilterValuesFor": {
    message: "Valores de {field}",
    description:
      "Accessible name of a filter row's Values picker; field is the row's field",
  },
  "logs.viewValuesRequired": {
    message: "Elige al menos un valor.",
    description: "Inline refusal of a filter row with no value",
  },
  "logs.viewRemoveFilter": {
    message: "Quitar el filtro {field}",
    description:
      "Accessible name of a filter row's trash button; field is the row's field",
  },
  "logs.viewOperatorIs": {
    message: "es",
    description:
      "Filter operator IS for a single-valued field (Environment, Time Range)",
  },
  "logs.viewOperatorIsOneOf": {
    message: "es uno de",
    description: "Filter operator IS for a field that takes several values",
  },
  "logs.viewOperatorIsNotOneOf": {
    message: "no es uno de",
    description: "Filter operator IS_NOT",
  },
  "logs.viewOperatorContains": {
    message: "contiene",
    description: "Filter operator CONTAINS, Text Search's only operator",
  },
  "logs.viewValuesPlaceholder": {
    message: "Selecciona valores",
    description:
      "Placeholder of a filter row's Values picker with nothing chosen",
  },
  "logs.viewNoResources": {
    message: "No hay recursos en este proyecto",
    description:
      "Resource picker of a project with no resource the view can read",
  },
  "logs.viewValueTypeHint": {
    message: "Escribe un valor y pulsa Enter",
    description: "Placeholder of a typed filter value (Text Search's terms)",
  },
  "logs.viewValueRemove": {
    message: "Quitar {value}",
    description:
      "Accessible name of a typed filter value's remove button; value is the value",
  },
  "logs.viewValueSearch": {
    message: "Busca o escribe un valor",
    description:
      "Placeholder of the search box in a filter's value picker fed by the project's logs (Level, Instance, Method, Status Code, Host, Path)",
  },
  "logs.viewValueAdd": {
    message: "Añadir «{value}»",
    description:
      "Picker item that adds the typed text as a filter value; value is the typed text",
  },
  "logs.viewValuesLoading": {
    message: "Leyendo los logs del proyecto…",
    description:
      "Value picker status while the project's logs are read for the field's values",
  },
  "logs.viewValuesUnavailable": {
    message:
      "No se pudieron leer los logs del proyecto. Escribe un valor para añadirlo.",
    description:
      "Value picker status when the project's logs could not be read for the field's values",
  },
  "logs.viewValuesNoneSeen": {
    message:
      "Ninguno en los logs del proyecto de los últimos 7 días. Escribe un valor para añadirlo.",
    description:
      "Value picker status when the project's logs hold no value of the field in the last 7 days",
  },
  "logs.viewFieldEnvironment": {
    message: "Entorno",
    description: "Filter field Environment in the Edit view modal",
  },
  "logs.viewFieldResource": {
    message: "Recurso",
    description: "Filter field Resource in the Edit view modal",
  },
  "logs.viewFieldResourceType": {
    message: "Tipo de recurso",
    description: "Filter field Resource Type in the Edit view modal",
  },
  "logs.viewFieldText": {
    message: "Búsqueda de texto",
    description: "Filter field Text Search in the Edit view modal",
  },
  "logs.viewFieldTimeRange": {
    message: "Rango de tiempo",
    description: "Filter field Time Range in the Edit view modal",
  },
  "logs.viewResourceTypeService": {
    message: "Servicios",
    description: "Resource Type value Services",
  },
  "logs.viewResourceTypeCronJob": {
    message: "Cron jobs",
    description: "Resource Type value Cron jobs",
  },
  "logs.viewResourceTypePostgres": {
    message: "Postgres",
    description: "Resource Type value Postgres",
  },
  "logs.viewResourceTypeKeyValue": {
    message: "Key Value",
    description: "Resource Type value Key Value (the product name)",
  },
  "logs.lineLevel": {
    message: "Nivel: {level}",
    description:
      "Log line level gutter (w3/m509): accessible name and hover text of the level icon leading a log row; {level} is the line's level word (info, warning, error, \u2026)",
  },
  "logs.copied": {
    message: "Logs copiados",
    description:
      "Log explorer (w3/m509): toast after Options > Copy logs wrote the lines shown to the clipboard",
  },
  "logs.copyFailed": {
    message: "No se pudieron copiar los logs",
    description:
      "Log explorer (w3/m509): toast when the clipboard refused the copied logs",
  },
  "logs.copyLogs": {
    message: "Copiar logs",
    description:
      "Log explorer Options menu (w3/m509): item copying every line shown to the clipboard (Render's Copy logs)",
  },
  "logs.options": {
    message: "Opciones de log",
    description:
      "Log explorer (w3/m509): accessible name and tooltip of the Options (\u2026) menu button",
  },
  "logs.orderBy": {
    message: "Ordenar por",
    description:
      "Log explorer Options menu (w3/m509): heading of the line order choice (Render's Order by)",
  },
  "logs.orderAscending": {
    message: "Ascendente",
    description:
      "Log explorer Options menu (w3/m509): order choice, oldest line first and the newest at the bottom",
  },
  "logs.orderDescending": {
    message: "Descendente",
    description:
      "Log explorer Options menu (w3/m509): order choice, newest line first at the top",
  },
  "logs.display": {
    message: "Visualización:",
    description:
      "Log explorer Options menu (w3/088): the Display submenu's label, before the chosen display in bold (Render's `Display: Expand everything`)",
  },
  "logs.displaySettings": {
    message: "Ajustes de visualización",
    description:
      "Log explorer Options menu (w3/088): heading of the Display choices (Render's Display settings)",
  },
  "logs.expandEverything": {
    message: "Expandir todo",
    description:
      "Log explorer Options ▸ Display (w3/088): every log line shown whole, the default (Render's Expand everything)",
  },
  "logs.expandErrors": {
    message: "Expandir errores",
    description:
      "Log explorer Options ▸ Display (w3/088): error, critical, alert and emergency lines shown whole, every other line on one line (Render's Expand errors)",
  },
  "logs.collapseEverything": {
    message: "Contraer todo",
    description:
      "Log explorer Options ▸ Display (w3/088): every log line on one line, cut off with an ellipsis (Render's Collapse everything)",
  },
  "logs.maximize": {
    message: "Maximizar",
    description:
      "Log explorer (w3/m509): button and menu item expanding the log to fill the screen (Render's Maximize)",
  },
  "logs.minimize": {
    message: "Salir de pantalla completa",
    description:
      "Log explorer (w3/m509): button and menu item restoring the maximized log to its place in the page",
  },
  "logs.maximizeHint": {
    message: "Maximizar (M)",
    description:
      "Log explorer (w3/m509): tooltip of the Maximize button, naming its M shortcut",
  },
  "logs.minimizeHint": {
    message: "Salir de pantalla completa (M o Esc)",
    description:
      "Log explorer (w3/m509): tooltip of the button leaving full screen, naming the M and Escape keys that also leave it",
  },
  "logs.clearedTitle": {
    message: "Logs borrados",
    description:
      "Log explorer (w3/m509): title over a live tail whose lines the reader cleared with Shift+L",
  },
  "logs.clearedBody": {
    message: "Las líneas nuevas aparecen aquí a medida que llegan.",
    description:
      "Log explorer (w3/m509): body under the cleared live tail's title",
  },
  "logs.clearedRestore": {
    message: "Mostrar líneas anteriores",
    description:
      "Log explorer (w3/m509): button under a cleared live tail bringing back the lines Shift+L cleared",
  },
  "logs.clearLogs": {
    message: "Borrar logs",
    description:
      "Log explorer Options menu (w3/092): item clearing a live tail's lines, as Shift+L does (Render's Clear logs); offered only while the tail is live",
  },
};

export default esLogs;
