import type { TranslationEntry } from "@/i18n";

const esDeploys: Record<string, TranslationEntry> = {
  "deploys.statusCreated": {
    message: "Creado",
    description: "Deploy status: created",
  },
  "deploys.statusQueued": {
    message: "En cola",
    description: "Deploy status: queued",
  },
  "deploys.statusBuildInProgress": {
    message: "Compilando",
    description: "Deploy status: build_in_progress",
  },
  "deploys.statusBuildFailed": {
    message: "Build fallido",
    description: "Deploy status: build_failed",
  },
  "deploys.statusPreDeployInProgress": {
    message: "Pre-despliegue en curso",
    description: "Deploy status: pre_deploy_in_progress",
  },
  "deploys.statusPreDeployFailed": {
    message: "Pre-despliegue fallido",
    description: "Deploy status: pre_deploy_failed",
  },
  "deploys.statusUpdateInProgress": {
    message: "En curso",
    description: "Deploy status: update_in_progress",
  },
  "deploys.statusUpdateFailed": {
    message: "Fallido",
    description: "Deploy status: update_failed",
  },
  "deploys.statusLive": {
    message: "Activo",
    description: "Deploy status: live",
  },
  "deploys.statusCanceled": {
    message: "Cancelado",
    description: "Deploy status: canceled",
  },
  "deploys.statusDeactivated": {
    message: "Desactivado",
    description: "Deploy status: deactivated",
  },
  "deploys.statusUnknown": {
    message: "Desconocido",
    description: "Deploy status: unrecognized backend value",
  },
  "deploys.statusLabel": {
    message: "Estado del despliegue",
    description:
      "Deploy detail header: explicit label distinguishing deploy status from service phase",
  },
  "deploys.imageDigestCopy": {
    message: "Copiar digest de la imagen",
    description:
      "Deploy header: accessible label of the button copying the image digest the deploy ran",
  },
  "deploys.imageDigestCopied": {
    message: "Digest de la imagen copiado",
    description: "Deploy header: toast after copying the image digest",
  },
  "deploys.imageDigestCopyError": {
    message: "No se pudo copiar al portapapeles",
    description: "Deploy header: toast when copying the image digest fails",
  },
  "deploys.created": {
    message: "Creado",
    description: "Deploy header: created-at label",
  },
  "deploys.updated": {
    message: "Actualizado",
    description: "Label for the deploy's latest stored transition time",
  },
  "deploys.started": {
    message: "Iniciado",
    description: "Deploy header: started-at label",
  },
  "deploys.finished": {
    message: "Finalizado",
    description: "Deploy header: finished-at label",
  },
  "deploys.duration": {
    message: "Duración",
    description: "Deploy header: elapsed build/deploy time label",
  },
  "deploys.durationValue": {
    message: "Duración {duration}",
    description: "Deploy row: elapsed build/deploy time",
  },
  "deploys.durationInProgress": {
    message: "Duración en curso",
    description: "Deploy row: elapsed time is still running",
  },
  "deploys.durationActive": {
    message: "En curso",
    description:
      "Deploys tab Duration column: bare running-elapsed marker (the column header supplies the label)",
  },
  "deploys.deployedAt": {
    message: "Desplegado {timestamp}",
    description:
      'Deploy row: how long ago a live/deactivated deploy went live — {timestamp} is elapsed text like "2 hours ago" (its finish time; createdAt fallback)',
  },
  "deploys.createdAt": {
    message: "Creado {timestamp}",
    description:
      'Deploy row: how long ago a deploy that hasn\'t finished (created/queued/in-progress) was created — {timestamp} is elapsed text like "2 minutes ago"',
  },
  "deploys.canceledAt": {
    message: "Cancelado {timestamp}",
    description:
      'Deploy row: how long ago a canceled deploy was canceled — {timestamp} is elapsed text like "3 days ago" (its finish time; createdAt fallback)',
  },
  "deploys.failedAt": {
    message: "Falló {timestamp}",
    description:
      'Deploy row: how long ago a build/pre-deploy/update failure landed — {timestamp} is elapsed text like "3 days ago" (its finish time; createdAt fallback)',
  },
  "deploys.notYet": {
    message: "—",
    description:
      "Deploy header: placeholder for a timestamp that hasn't happened yet",
  },
  "deploys.triggerCreate": {
    message: "Primer despliegue",
    description: "Deploy header: trigger=create label",
  },
  "deploys.triggerApi": {
    message: "Despliegue por API",
    description: "Deploy header: trigger=api label (an API-key deploy, w4/413)",
  },
  "deploys.triggerManual": {
    message: "Despliegue manual",
    description:
      "Deploy header: trigger=manual label (a dashboard click, w4/413)",
  },
  "deploys.triggerRollback": {
    message: "Reversión a {deployId}",
    description:
      "Deploy header: trigger label for a rollback deploy, naming the restored deploy (id must stay literal — never wrap in CSS capitalize)",
  },
  "deploys.triggerDeployHook": {
    message: "Deploy Hook",
    description: "Deploy row: trigger=deploy_hook label (secret-URL trigger)",
  },
  "deploys.triggerBlueprint": {
    message: "Sincronización de Blueprint",
    description: "Deploy row: trigger=blueprint label (Blueprint sync deploy)",
  },
  "deploys.triggerNewCommit": {
    message: "Nuevo commit",
    description: "Deploy row: trigger=new_commit label (git push redeploy)",
  },
  "deploys.triggerConfigChange": {
    message: "Cambio de configuración",
    description:
      "Deploy row: trigger=config_change label (an env var, secret file, or env-group edit that deployed)",
  },
  "deploys.triggerSettingsChange": {
    message: "Cambio de configuración",
    description:
      "Deploy row: trigger=settings_change label (a Settings save that deployed, or a plan change; w8/m135)",
  },
  "deploys.triggerRestart": {
    message: "Reinicio",
    description:
      "Deploy row: trigger=restart label (a Restart roll of the live release, w4/379)",
  },
  "deploys.triggerDeployedByRender": {
    message: "Reconstrucción de la plataforma",
    description:
      "Deploy row: trigger=deployed_by_render label (the platform rebuilt the live commit, e.g. to ship a runtime security patch)",
  },
  "deploys.logSearchPlaceholder": {
    message: "Buscar en los logs…",
    description: "Deploy detail page: log search input placeholder",
  },
  "deploys.logTypeFilter": {
    message: "Tipo de log",
    description:
      "Deploy log viewer: aria-label of the log-type filter dropdown (Render's All/Application/Build selector)",
  },
  "deploys.logTypeAll": {
    message: "Todos los logs",
    description: "Deploy log viewer type filter: show every log type",
  },
  "deploys.logTypeApp": {
    message: "Logs de la aplicación",
    description:
      "Deploy log viewer type filter: app + pre-deploy lines, matching Render's Application bucket",
  },
  "deploys.logTypeBuild": {
    message: "Logs de build",
    description: "Deploy log viewer type filter: build lines only",
  },
  "deploys.buildLogsStoreUnavailable": {
    message: "Los logs de build históricos no están disponibles",
    description:
      "Deploy detail page: title shown when the durable log store isn't wired",
  },
  "deploys.buildLogsStoreUnavailableBody": {
    message:
      "Este panel no está conectado a un almacenamiento de logs duradero. Aun así, pueden aparecer los logs de la aplicación de este despliegue.",
    description:
      "Deploy detail page: accurate explanation of a storeless build-log response",
  },
  "deploys.detailRefreshError": {
    message:
      "No se pudo actualizar este despliegue. Se muestran los últimos detalles cargados.",
    description:
      "Deploy detail page: a background refresh of an already loaded deploy failed; the page keeps the last good details and offers a retry",
  },
  "deploys.logPartiallyLoaded": {
    message:
      "Solo se ha cargado una parte de este intervalo de tiempo. Carga los más antiguos para buscar en líneas anteriores.",
    description:
      "Deploy log viewer banner while older history remains unloaded. The panel's search and type/instance filters run over loaded lines only, so it must not claim a number of matches",
  },
  "deploys.logStepEndMissing": {
    message:
      "Este despliegue se canceló antes de que terminara su paso de pre-despliegue y no se registró la hora de finalización del paso, por lo que su salida posterior puede quedar fuera de esta ventana. Elige un intervalo más amplio en Opciones de log para verla.",
    description:
      "Deploy log viewer: a canceled deploy's continued pre-deploy step has no recorded end, so the deploy window may omit its later output",
  },
  "deploys.logStepEndUnavailable": {
    message:
      "No se pudo comprobar cuándo terminó el paso de pre-despliegue de este despliegue, por lo que la salida posterior del paso puede quedar fuera de esta ventana. Inténtalo de nuevo o elige un intervalo más amplio en Opciones de log.",
    description:
      "Deploy log viewer: reading a canceled deploy's continued pre-deploy step end failed; offers a retry",
  },
  "deploys.timelineTitle": {
    message: "Cronología de estados",
    description: "Deploy detail page: status-timeline card title",
  },
  "deploys.timelineCreated": {
    message: "Despliegue creado",
    description: "Deploy timeline: deploy row was created",
  },
  "deploys.timelineStarted": {
    message: "Despliegue iniciado",
    description: "Deploy timeline: backend-provided startedAt timestamp",
  },
  "deploys.timelineInProgress": {
    message: "Despliegue en curso",
    description: "Deploy timeline: current non-terminal deploy status",
  },
  "deploys.timelineLive": {
    message: "Despliegue activo",
    description: "Deploy timeline: successful terminal state",
  },
  "deploys.timelineFailed": {
    message: "Despliegue fallido",
    description: "Deploy timeline: failed terminal state",
  },
  "deploys.timelineCanceled": {
    message: "Despliegue cancelado",
    description: "Deploy timeline: canceled terminal state",
  },
  "deploys.timelineDeactivated": {
    message: "Despliegue desactivado",
    description: "Deploy timeline: deactivated terminal state",
  },
  "deploys.timelineEventsUnavailable": {
    message:
      "Los eventos del servicio no están disponibles; solo se muestran las marcas de tiempo del despliegue.",
    description:
      "Deploy timeline: graceful fallback when service-events query fails",
  },
  "deploys.autoDeployOffTitle": {
    message: "El despliegue automático está desactivado",
    description:
      "Deploys page banner title when the service's auto-deploy is off (Render, w3/064)",
  },
  "deploys.autoDeployOffBody": {
    message:
      "Los push a {branch} no despliegan este servicio. Despliégalo manualmente o vuelve a activar el despliegue automático.",
    description: "Deploys page banner body; {branch} is the tracked git branch",
  },
  "deploys.autoDeployOffBodyNoBranch": {
    message:
      "Los push no despliegan este servicio. Despliégalo manualmente o vuelve a activar el despliegue automático.",
    description: "Deploys page banner body when the branch is unknown",
  },
  "deploys.autoDeployOffSettings": {
    message: "Configuración de despliegue automático",
    description: "Deploys page banner link to the service Settings page",
  },
  "deploys.deploysManualTitle": {
    message: "Los despliegues son manuales",
    description:
      "Deploys page banner title for a repo whose pushes can't reach bex (pushDeliveryMethod unavailable)",
  },
  "deploys.deploysManualBody": {
    message:
      "bex no recibe los push de este repositorio, por lo que no despliegan este servicio. Despliégalo manualmente o con el Deploy Hook, o conecta el repositorio mediante la app de GitHub.",
    description:
      "Deploys page banner body for a repo whose pushes can't reach bex: no Auto-Deploy setting can turn deploys on push on (Render: a public Git URL must be deployed manually)",
  },
  "deploys.listTitle": {
    message: "Despliegues",
    description: "Deploys tab: card title over the deploy-history list",
  },
  "deploys.buildsTitle": {
    message: "Builds",
    description:
      "A cron job's Builds tab: card title over its build history, Render's Builds page (w2/m360)",
  },
  "deploys.columnDeploy": {
    message: "Despliegue",
    description:
      "Deploys tab table: first column header (status, id, commit, timestamp)",
  },
  "deploys.columnTrigger": {
    message: "Origen",
    description: "Deploys tab table: column header for what triggered a deploy",
  },
  "deploys.columnDuration": {
    message: "Duración",
    description:
      "Deploys tab table: column header for elapsed build/deploy time",
  },
  "deploys.columnActions": {
    message: "Acciones",
    description:
      "Deploys tab table: accessible-only header over the Cancel/Rollback action column",
  },
  "deploys.listEmpty": {
    message: "Aún no hay despliegues.",
    description: "Deploys tab: empty state with no status filter",
  },
  "deploys.listEmptyFiltered": {
    message: "Ningún despliegue coincide con el estado seleccionado.",
    description: "Deploys tab: empty state while a status filter is active",
  },
  "deploys.listStatusFilterLabel": {
    message: "Filtrar por estado",
    description: "Deploys tab: aria-label of the status filter dropdown",
  },
  "deploys.listStatusAll": {
    message: "Todos los estados",
    description: "Deploys tab: status filter option matching every deploy",
  },
  "deploys.listSearchPlaceholder": {
    message: "Buscar…",
    description:
      "Deploys tab: placeholder for client-side deploy-history search",
  },
  "deploys.listSearchLabel": {
    message: "Buscar en despliegues y commits cargados",
    description: "Deploys tab: accessible label for deploy-history search",
  },
  "deploys.listClearSearch": {
    message: "Borrar búsqueda",
    description:
      "Deploys tab: button under the no-match message that empties the search box",
  },
  "deploys.listMatchCount": {
    message: "Coincidencias: {matched} de {total} cargados",
    description:
      "Deploys tab: count shown while a search is typed; matched = loaded deploys that match, total = loaded deploys",
  },
  "deploys.listLoadErrorTitle": {
    message: "No se pudieron cargar los despliegues",
    description:
      "Deploys tab: heading of the error state when the deploy history could not be read; a Try again button follows",
  },
  "deploys.listLoadErrorBody": {
    message:
      "Tus despliegues no se ven afectados. Revisa tu conexión e inténtalo de nuevo.",
    description:
      "Deploys tab: body of the error state when the deploy history could not be read and the server gave no message; a Try again button follows",
  },
  "deploys.listEmptySearch": {
    message: "Ningún despliegue cargado coincide con tu búsqueda.",
    description:
      "Deploys tab: empty state when client-side search has no match",
  },
  "deploys.listCount_one": {
    message: "{count} despliegue",
    description: "Deploys tab: complete deploy count (singular)",
  },
  "deploys.listCount_many": {
    message: "{count} de despliegues",
    description: "Deploys tab: complete deploy count (plural)",
  },
  "deploys.listCount_other": {
    message: "{count} despliegues",
    description: "Deploys tab: complete deploy count (plural)",
  },
  "deploys.listCountLoaded_one": {
    message: "{count} despliegue cargado",
    description:
      "Deploys tab: loaded count when more server pages remain (singular)",
  },
  "deploys.listCountLoaded_many": {
    message: "{count} de despliegues cargados",
    description:
      "Deploys tab: loaded count when more server pages remain (plural)",
  },
  "deploys.listCountLoaded_other": {
    message: "{count} despliegues cargados",
    description:
      "Deploys tab: loaded count when more server pages remain (plural)",
  },
  "deploys.listLoadMore": {
    message: "Cargar más",
    description: "Deploys tab: button fetching the next page of deploy history",
  },
  "deploys.listLoadMoreError": {
    message: "No se pudieron cargar los despliegues anteriores.",
    description:
      "Deploys tab: toast when reading an older page of deploy history fails",
  },
  "deploys.logOptions": {
    message: "Opciones de log",
    description:
      "Deploy log viewer: aria-label/tooltip of the options menu button",
  },
  "deploys.logRangeLabel": {
    message: "Intervalo de tiempo",
    description:
      "Deploy log viewer options menu: heading over the time-range choices",
  },
  "deploys.logRangeDeploy": {
    message: "Ventana del despliegue",
    description:
      "Deploy log viewer: the default time range — the deploy's own createdAt..finishedAt window",
  },
  "deploys.logRangeLast15m": {
    message: "Últimos 15 minutos",
    description: "Deploy log viewer: relative time-range option (?r=15m)",
  },
  "deploys.logRangeLast1h": {
    message: "Última hora",
    description: "Deploy log viewer: relative time-range option (?r=1h)",
  },
  "deploys.logRangeLast6h": {
    message: "Últimas 6 horas",
    description: "Deploy log viewer: relative time-range option (?r=6h)",
  },
  "deploys.logRangeLast24h": {
    message: "Últimas 24 horas",
    description: "Deploy log viewer: relative time-range option (?r=24h)",
  },
  "deploys.logRangeLast7d": {
    message: "Últimos 7 días",
    description: "Deploy log viewer: relative time-range option (?r=7d)",
  },
  "deploys.logWrap": {
    message: "Ajustar líneas",
    description:
      "Deploy log viewer options menu: toggle wrapping long log lines vs horizontal scroll",
  },
  "deploys.logTimestamps": {
    message: "Mostrar marcas de tiempo",
    description:
      "Deploy log viewer options menu: toggle the per-line timestamp column",
  },
  "deploys.logMaximize": {
    message: "Maximizar",
    description:
      "Deploy log viewer: button expanding the viewer to fill the screen",
  },
  "deploys.logMinimize": {
    message: "Salir de pantalla completa",
    description:
      "Deploy log viewer: button restoring the maximized viewer to its inline size",
  },
  "deploys.reason.http_port_waiting": {
    message:
      "Esperando a que el contenedor se inicie antes de detectar su puerto HTTP",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_waiting, w4/566)",
  },
  "deploys.reason.http_port_detecting": {
    message: "Detectando el puerto HTTP del contenedor",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_detecting, w4/566)",
  },
  "deploys.reason.http_port_limit": {
    message:
      "El puerto HTTP detectado supera el límite de 75 puertos del servicio; define PORT explícitamente",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_limit, w4/566)",
  },
  "deploys.reason.http_port_detected": {
    message: "Se detectó HTTP en el puerto {port}; se está sirviendo",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_detected, w4/566); params are the sentence's values",
  },
  "deploys.reason.http_port_not_answering": {
    message:
      "El puerto {port} aceptó una conexión pero aún no ha respondido por HTTP; se sigue explorando mientras el contenedor se inicia",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_not_answering, w4/566); params are the sentence's values",
  },
  "deploys.reason.http_port_none_yet": {
    message:
      "Ningún puerto responde aún por HTTP; se sigue explorando mientras el contenedor se inicia",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_none_yet, w4/566)",
  },
  "deploys.reason.http_port_fallback": {
    message:
      "Ningún puerto respondió por HTTP; se usa el puerto alternativo {port}. Define PORT con el listener HTTP si se enlaza en otro puerto",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_fallback, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_not_found": {
    message:
      "la imagen {image} no se encontró en su registro: el repositorio o la etiqueta no existe. Comprueba la URL y la etiqueta de la imagen, o despliega un digest que exista",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_not_found, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_credential_refused": {
    message:
      "{host} rechazó la credencial de registro de este servicio para la imagen {image} (HTTP {status}). Comprueba el nombre de usuario y el token de la credencial, o quita la credencial de este servicio para descargar una imagen pública de forma anónima",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_credential_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_access_refused": {
    message:
      "el registro rechazó el acceso a la imagen {image}. Si la imagen es privada, añade una credencial de registro para {host} a este servicio",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_access_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_read_refused": {
    message:
      "el registro rechazó leer la imagen {image} (HTTP {status}). Comprueba la URL y la etiqueta de la imagen",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_read_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_registry_unreachable": {
    message:
      "el registro {host} no respondió a la lectura de la imagen {image} durante {minutes} minutos. Comprueba que el registro sea accesible y vuelve a desplegar",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_registry_unreachable, w4/692); params are the sentence's values",
  },
  "deploys.reason.build_timed_out": {
    message:
      "el build no terminó dentro de su ventana de tiempo; revisa los logs de build",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason build_timed_out, w4/566)",
  },
  "deploys.reason.pre_deploy_timed_out": {
    message:
      "el comando previo al despliegue no terminó dentro de su ventana de tiempo; revisa los logs de pre-despliegue",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_timed_out, w4/566)",
  },
  "deploys.reason.health_timed_out": {
    message:
      "el despliegue no pasó a estado saludable dentro de la ventana de la comprobación de estado; revisa los logs del servicio",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason health_timed_out, w4/566)",
  },
  "deploys.reason.startup_out_of_memory": {
    message:
      "la instancia se quedó sin memoria durante el arranque (límite de {memory} en su tipo de instancia; código de salida {code}): reduce el uso de memoria o elige un tipo de instancia mayor",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_out_of_memory, w4/630); params are the sentence's values: memory is the instance type's memory limit (e.g. 512Mi), code the exit code",
  },
  "deploys.reason.probe_http_waiting": {
    message:
      "el contenedor está en ejecución pero su comprobación de estado {kind} no ha tenido éxito, por lo que el despliegue está esperando: GET {path} en el puerto {port}. Comprueba que la ruta devuelva un estado 2xx o 3xx en el puerto propio del servicio.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_waiting, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_waiting_restarted": {
    message:
      "el contenedor está en ejecución pero su comprobación de estado {kind} no ha tenido éxito, por lo que el despliegue está esperando: GET {path} en el puerto {port}. La comprobación de liveness ya ha reiniciado el contenedor {restarts} vez/veces, por lo que la comprobación está fallando y no es simplemente lenta. Comprueba que la ruta devuelva un estado 2xx o 3xx en el puerto propio del servicio.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_waiting_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_failed": {
    message:
      "el contenedor está en ejecución pero su comprobación de estado {kind} nunca tuvo éxito dentro de la ventana del despliegue, por lo que este falló: GET {path} en el puerto {port}. Comprueba que la ruta devuelva un estado 2xx o 3xx en el puerto propio del servicio.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_failed, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_failed_restarted": {
    message:
      "el contenedor está en ejecución pero su comprobación de estado {kind} nunca tuvo éxito dentro de la ventana del despliegue, por lo que este falló: GET {path} en el puerto {port}. La comprobación de liveness ya ha reiniciado el contenedor {restarts} vez/veces, por lo que la comprobación está fallando y no es simplemente lenta. Comprueba que la ruta devuelva un estado 2xx o 3xx en el puerto propio del servicio.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_failed_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_waiting": {
    message:
      "el contenedor está en ejecución pero su comprobación de estado {kind} no ha tenido éxito, por lo que el despliegue está esperando: una conexión TCP al puerto {port}. No hay ninguna Health Check Path definida, así que la comprobación es una conexión TCP, que solo verifica si el proceso está escuchando en ese puerto.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_waiting, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_waiting_restarted": {
    message:
      "el contenedor está en ejecución pero su comprobación de estado {kind} no ha tenido éxito, por lo que el despliegue está esperando: una conexión TCP al puerto {port}. La comprobación de liveness ya ha reiniciado el contenedor {restarts} vez/veces, por lo que la comprobación está fallando y no es simplemente lenta. No hay ninguna Health Check Path definida, así que la comprobación es una conexión TCP, que solo verifica si el proceso está escuchando en ese puerto.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_waiting_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_failed": {
    message:
      "el contenedor está en ejecución pero su comprobación de estado {kind} nunca tuvo éxito dentro de la ventana del despliegue, por lo que este falló: una conexión TCP al puerto {port}. No hay ninguna Health Check Path definida, así que la comprobación es una conexión TCP, que solo verifica si el proceso está escuchando en ese puerto.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_failed, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_failed_restarted": {
    message:
      "el contenedor está en ejecución pero su comprobación de estado {kind} nunca tuvo éxito dentro de la ventana del despliegue, por lo que este falló: una conexión TCP al puerto {port}. La comprobación de liveness ya ha reiniciado el contenedor {restarts} vez/veces, por lo que la comprobación está fallando y no es simplemente lenta. No hay ninguna Health Check Path definida, así que la comprobación es una conexión TCP, que solo verifica si el proceso está escuchando en ese puerto.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_failed_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.rollout_deadline_exceeded": {
    message:
      "el despliegue no pasó a estado saludable dentro del plazo de progreso",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason rollout_deadline_exceeded, w4/591)",
  },
  "deploys.reason.pre_deploy_window_exceeded": {
    message:
      "el comando previo al despliegue no terminó dentro de su ventana de {minutes} minutos; revisa los logs de pre-despliegue",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_window_exceeded, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_out_of_memory": {
    message:
      "el comando previo al despliegue se quedó sin memoria (código de salida {code}); revisa los logs de pre-despliegue",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_out_of_memory, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_exited": {
    message:
      "el comando previo al despliegue terminó con el código {code}; revisa los logs de pre-despliegue",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_exited, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_failed": {
    message:
      "el comando previo al despliegue falló; revisa los logs de pre-despliegue",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_failed, w4/591)",
  },
  "deploys.reason.pre_deploy_could_not_launch": {
    message:
      "el comando previo al despliegue no pudo iniciarse: el entorno de ejecución de contenedores no pudo lanzar su programa; el programa nunca se ejecutó, por lo que no hay logs de pre-despliegue",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_could_not_launch, w4/591)",
  },
  "deploys.reason.build_never_started": {
    message: "el build nunca se inició: {cause}",
    description:
      "A deploy failure reason by its code (bex-api DeployReason build_never_started, w4/591); {cause} is the operator's wait, whose sentences are worded as parts (w5/m356)",
  },
  "deploys.reason.build_abandoned": {
    message:
      "el build no terminó dentro de su ventana de tiempo y no se pudo leer el registro App de este servicio para explicar el motivo",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason build_abandoned, w4/591)",
  },
  "deploys.reason.deploy_abandoned": {
    message:
      "el despliegue no terminó dentro de su ventana de tiempo y no se pudo leer el registro App de este servicio para explicar el motivo",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason deploy_abandoned, w4/591)",
  },
  "deploys.reason.platform_error": {
    message:
      "bex no pudo aplicar esta versión debido a un error de la plataforma, no a un problema con tu código o configuración. Contacta con soporte con la referencia {ref}",
    description:
      "A deploy failure reason by its code (bex-api DeployReason platform_error, w9/090): the platform refused one of bex's own changes; {ref} is the support reference, shown as-is",
  },
  "deploys.reason.canceled_suspended": {
    message: "Cancelado: el servicio fue suspendido",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_suspended, w4/591)",
  },
  "deploys.reason.superseded_by_newer_release": {
    message: "Reemplazado por una versión más reciente",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason superseded_by_newer_release, w4/591)",
  },
  "deploys.reason.superseded_by": {
    message: "Reemplazado por {deploy}",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason superseded_by, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_by_restart": {
    message:
      "Cancelado por el reinicio {deploy}, que mantuvo la versión en ejecución: lo que este despliegue estaba aplicando no se desplegó",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_by_restart, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_along_with": {
    message: "Cancelado junto con {deploy}",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_along_with, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_along_with_newer": {
    message: "Cancelado junto con un despliegue más reciente",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_along_with_newer, w4/591)",
  },
  "deploys.reason.restart_service_gone": {
    message: "el servicio ya no existe",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason restart_service_gone, w4/591)",
  },
  "deploys.reason.restart_wait_timed_out": {
    message:
      "el despliegue que este reinicio esperaba no terminó dentro de {wait}",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason restart_wait_timed_out, w4/591); params are the sentence's values",
  },
  "deploys.reason.deploy_wait_timed_out": {
    message: "el despliegue que este esperaba no terminó en {wait}",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason deploy_wait_timed_out, w4/652): a Rollback or another deploy that waited for the deploy in flight; params are the sentence's values",
  },
  "deploys.reason.probe_other_waiting": {
    message:
      "el contenedor está en ejecución pero su comprobación de estado {kind} no ha tenido éxito, por lo que el despliegue está esperando: la comprobación de estado configurada. Comprueba que la ruta devuelva un estado 2xx o 3xx en el puerto propio del servicio.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_waiting, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_waiting_restarted": {
    message:
      "el contenedor está en ejecución pero su comprobación de estado {kind} no ha tenido éxito, por lo que el despliegue está esperando: la comprobación de estado configurada. La comprobación de liveness ya ha reiniciado el contenedor {restarts} vez/veces, por lo que la comprobación está fallando y no es simplemente lenta. Comprueba que la ruta devuelva un estado 2xx o 3xx en el puerto propio del servicio.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_waiting_restarted, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_failed": {
    message:
      "el contenedor está en ejecución pero su comprobación de estado {kind} nunca tuvo éxito dentro de la ventana del despliegue, por lo que este falló: la comprobación de estado configurada. Comprueba que la ruta devuelva un estado 2xx o 3xx en el puerto propio del servicio.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_failed, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_failed_restarted": {
    message:
      "el contenedor está en ejecución pero su comprobación de estado {kind} nunca tuvo éxito dentro de la ventana del despliegue, por lo que este falló: la comprobación de estado configurada. La comprobación de liveness ya ha reiniciado el contenedor {restarts} vez/veces, por lo que la comprobación está fallando y no es simplemente lenta. Comprueba que la ruta devuelva un estado 2xx o 3xx en el puerto propio del servicio.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_failed_restarted, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.hint_image_default_port": {
    message:
      " Nada aceptó una conexión TCP en el puerto {port}, el predeterminado porque la imagen no declara puertos TCP. El servidor debe enlazarse al host 0.0.0.0 en el puerto indicado en PORT ({port}).",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_image_default_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_image_declared_ports": {
    message:
      " Nada aceptó una conexión TCP en el puerto {port}, el puerto principal elegido entre los puertos TCP declarados por la imagen ({ports}). Enlaza el servidor a 0.0.0.0 en uno de esos puertos o define el puerto del servicio con aquel en el que escucha.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_image_declared_ports, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_env_port_only": {
    message:
      " Nada aceptó una conexión TCP en el puerto {port}, definido por la variable de entorno PORT del servicio; la imagen no declara ningún otro puerto TCP. Enlaza el servidor a 0.0.0.0 en el puerto {port} o cambia el puerto del servicio por aquel en el que escucha.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_env_port_only, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_configured_port_only": {
    message:
      " Nada aceptó una conexión TCP en el puerto {port}, definido por el puerto configurado del servicio; la imagen no declara ningún otro puerto TCP. Enlaza el servidor a 0.0.0.0 en el puerto {port} o cambia el puerto del servicio por aquel en el que escucha.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_configured_port_only, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_env_port_declared": {
    message:
      " Nada aceptó una conexión TCP en el puerto {port}, definido por la variable de entorno PORT del servicio; la imagen declara puertos TCP ({ports}). Cambia la variable de entorno PORT por el puerto en el que escucha o elimínala para usar el puerto de la imagen.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_env_port_declared, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_configured_port_declared": {
    message:
      " Nada aceptó una conexión TCP en el puerto {port}, definido por el puerto configurado del servicio; la imagen declara puertos TCP ({ports}). Cambia el puerto del servicio en Configuración por aquel en el que escucha o elimínalo para usar el puerto de la imagen.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_configured_port_declared, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_background_worker": {
    message:
      " Si el servicio no necesita recibir tráfico en ningún puerto, crea un Background Worker en su lugar.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_background_worker, w5/m356); en starts with the space that joins it to the sentence before",
  },
  "deploys.reason.hint_no_http_discovered": {
    message:
      " Ningún puerto respondió por HTTP durante la detección al iniciar.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_http_discovered, w5/m356); en starts with the space that joins it to the sentence before",
  },
  "deploys.reason.hint_listening": {
    message: " El proceso está escuchando en el puerto {port}.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_adopted": {
    message:
      " El proceso está escuchando en el puerto {port}, que el servicio usa ahora.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_adopted, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_ports": {
    message:
      " El proceso está escuchando en los puertos {ports}, no en el puerto {port}; define el puerto del servicio (o su variable PORT) con el que sirve HTTP.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_ports, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_primary": {
    message:
      " El proceso está escuchando en el puerto {port}, que el servicio usa ahora como su puerto principal.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_primary, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_port": {
    message:
      " No se detectó ningún puerto abierto en la instancia: el servidor debe enlazarse al host 0.0.0.0 en el puerto indicado en PORT ({port}).",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_port_before": {
    message:
      " No se detectó ningún puerto abierto en la instancia: el servidor debe enlazarse al host 0.0.0.0 en el puerto indicado en PORT ({port}); antes escuchaba en el puerto {previous}.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_port_before, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_service_port": {
    message:
      " No se detectó ningún puerto abierto en la instancia: el servidor debe enlazarse al host 0.0.0.0 en el puerto del servicio ({port}).",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_service_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.queued_build_limit": {
    message:
      "el {noun} tiene {active}/{limit} builds simultáneos activos; esperando un hueco",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_build_limit, w5/m356): a full concurrent-build cap; {noun} is the cap's scope, worded by deploys.reasonParam.noun.*",
  },
  "deploys.reason.queued_cluster_full": {
    message: "la capacidad de build del clúster está llena; esperando un hueco",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_cluster_full, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_slot_taken": {
    message:
      "un envío simultáneo ocupó el último hueco de build; esperando un hueco",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_slot_taken, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_database": {
    message:
      "Esperando a que la base de datos vinculada {database} esté disponible antes de compilar",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_database, w5/m356): what the build waited on; params are the sentence's values",
  },
  "deploys.reason.queued_datastore": {
    message:
      "Esperando los detalles de conexión de un almacén de datos vinculado antes de compilar",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_datastore, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_build_credential": {
    message:
      "Esperando a que el registro acepte la credencial de build de esta app",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_build_credential, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_pull_credential": {
    message:
      "Esperando a que el registro acepte la credencial de descarga de esta app",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_pull_credential, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity": {
    message: "esperando capacidad de build: {detail}",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity, w5/m356): waiting for build capacity; {detail} is the build plane's own wait, shown as-is",
  },
  "deploys.reason.queued_capacity_pod": {
    message:
      "esperando capacidad de build: trabajo de build enviado; esperando a que se cree su pod de build",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_pod, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity_no_node": {
    message:
      "esperando capacidad de build: ningún nodo de build tiene espacio todavía",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_no_node, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity_short": {
    message:
      "esperando capacidad de build: ningún nodo de build tiene espacio todavía (no hay suficiente {resources} libre)",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_short, w5/m356): what the build waited on; params are the sentence's values",
  },
  "deploys.reasonParam.kind.startup": {
    message: "de inicio",
    description:
      "A deploy reason's {kind} value startup (w4/591): which health check a stalled rollout is waiting on",
  },
  "deploys.reasonParam.kind.readiness": {
    message: "de preparación",
    description:
      "A deploy reason's {kind} value readiness (w4/591): which health check a stalled rollout is waiting on",
  },
  "deploys.reasonParam.noun.workspace": {
    message: "espacio de trabajo",
    description:
      "A deploy reason's {noun} value workspace (w5/m356): the scope of a full concurrent-build cap",
  },
  "deploys.reasonWait.duration": {
    message: "{units}",
    description:
      "A deferred restart's wait, restart_wait_timed_out's {wait} (w5/m356): {wait} is bex-api's Go duration (3h0m0s), which en keeps so the sentence reads as the API's; {units} is the duration in words (deploys.reasonWait.hours and the rest), which other languages read",
  },
  "deploys.reasonWait.hours": {
    message: "{hours} h",
    description: "A deferred restart's wait in words (w5/m356): its hours",
  },
  "deploys.reasonWait.minutes": {
    message: "{minutes} min",
    description: "A deferred restart's wait in words (w5/m356): its minutes",
  },
  "deploys.reasonWait.seconds": {
    message: "{seconds} s",
    description: "A deferred restart's wait in words (w5/m356): its seconds",
  },
};

export default esDeploys;
