import type { TranslationEntry } from "@/i18n";

const esServices: Record<string, TranslationEntry> = {
  "services.protectedConfirmationTitle": {
    message: "Confirmación de entorno protegido",
    description: "Title of a protected-service destructive-action retry dialog",
  },
  "services.protectedConfirmationBody": {
    message:
      "“{name}” pertenece a un entorno protegido. Introduce el comando sudo exacto devuelto por bex-api para continuar.",
    description: "Body of a protected-service destructive-action retry dialog",
  },
  "services.protectedConfirmationPrompt": {
    message: "Escribe {phrase} a continuación para confirmar.",
    description:
      "Body prompt naming the exact protected-action confirmation phrase (rendered bold by SudoCommandField)",
  },
  "services.connect": {
    message: "Conectar",
    description: "Open the service connection menu",
  },
  "services.connectSSH": {
    message: "SSH",
    description: "SSH section label in the service connection menu",
  },
  "services.openBrowserTerminal": {
    message: "Abrir un terminal en el navegador",
    description:
      "RequiresSshKey gate — zero-setup second door to the Web Shell (no SSH key)",
  },
  "services.connectInternal": {
    message: "Interna",
    description:
      "Internal-address section label in the service connection menu (Render's Connect → Internal tab)",
  },
  "services.connectInternalFreeNote": {
    message:
      "Los Web Services gratuitos no pueden recibir tráfico de red privada. Mejora tu plan para usar la dirección interna.",
    description:
      "Connect → Internal note for a free public web service: Render's Free plan does not support receiving private network traffic, so a sibling's internal call never reaches it (w8/m130)",
  },
  "services.headerServiceAddress": {
    message: "Dirección del servicio",
    description:
      "Label before a private service's internal address in the detail header (Render's private-service wording)",
  },
  "services.openPorts": {
    message: "Puertos abiertos",
    description:
      "Label before the TCP ports a private service's hostname serves, beside its Service Address and in Connect → Internal (Render's serviceDetails.openPorts, w6/m148)",
  },
  "services.openPortsPrimary": {
    message: "principal",
    description:
      "Marker on the open port that is the service's primary port — the <slug>:<port> suffix of its Service Address (w6/m148)",
  },
  "services.internalCopy": {
    message: "Copiar dirección interna",
    description: "Copy service internal address button",
  },
  "services.internalCopied": {
    message: "Dirección interna copiada",
    description: "Successful internal address copy",
  },
  "services.internalCopyError": {
    message: "No se pudo copiar la dirección interna",
    description: "Failed internal address copy",
  },
  "services.sshCopy": {
    message: "Copiar comando SSH",
    description: "Copy service SSH command button",
  },
  "services.sshCopied": {
    message: "Comando SSH copiado",
    description: "Successful SSH command copy",
  },
  "services.sshCopyError": {
    message: "No se pudo copiar el comando SSH",
    description: "Failed SSH command copy",
  },
  "services.sshUnavailable": {
    message: "SSH no está disponible",
    description: "Service header state when no SSH address is advertised",
  },
  "services.sshUnavailableHint": {
    message:
      "SSH requiere un Web Service, Private Service o Background Worker de pago en ejecución y un gateway activo.",
    description: "Explanation for a service without an SSH address",
  },
  "services.sshUnavailableSuspended": {
    message:
      "Este servicio está suspendido. Reanúdalo para conectarte por SSH.",
    description: "Why SSH is unavailable on a suspended service (w4/143)",
  },
  "services.sshUnavailableSuspendedFree": {
    message:
      "Este servicio está suspendido y SSH necesita un plan de cómputo de pago. Reanudarlo restablece el tráfico; los servicios gratuitos siguen sin incluir acceso a shell.",
    description:
      "Why SSH is unavailable on a suspended Free service: both blockers, without implying a resume grants SSH (w4/m183)",
  },
  "services.sshUnavailableFree": {
    message:
      "SSH necesita un plan de cómputo de pago. Los servicios gratuitos no incluyen acceso a shell.",
    description: "Why SSH is unavailable on a Free-plan service (w4/143)",
  },
  "services.sshRemedyResume": {
    message: "Reanudar servicio",
    description:
      "Link from an SSH-unavailable state to the service's suspend/resume setting (w4/143)",
  },
  "services.sshRemedyChangePlan": {
    message: "Cambiar plan de cómputo",
    description:
      "Link from an SSH-unavailable state on a Free service to its compute-plan page (w4/143)",
  },
  "services.shellDescription": {
    message:
      "Abre un shell en una instancia en ejecución del servicio desde tu terminal.",
    description: "Running-instance SSH connection page description",
  },
  "services.shellConnectionTitle": {
    message: "Conectar con SSH",
    description: "SSH connection card title",
  },
  "services.shellConnectionDescription": {
    message:
      "bex se conecta a una instancia lista existente. No crea una instancia de shell aparte.",
    description: "SSH running-instance behavior explanation",
  },
  "services.shellCommand": {
    message: "Comando SSH",
    description: "Label for a copy-ready service SSH command",
  },
  "services.shellManageKeys": {
    message: "Gestionar claves públicas SSH",
    description: "Link from a service shell page to account SSH key settings",
  },
  "services.shellSessionLifecycle": {
    message:
      "El comando selecciona una instancia lista. Reiniciar, volver a desplegar o suspender el servicio cierra la sesión.",
    description: "SSH session selection and lifecycle guidance",
  },
  "services.shellUnavailableTitle": {
    message: "El acceso a shell no está disponible",
    description: "Unavailable SSH connection card title",
  },
  "services.shellUnavailableBody": {
    message:
      "El acceso a shell requiere un Web Service, Private Service o Background Worker de pago en ejecución y un gateway SSH activo.",
    description: "Unavailable SSH connection card explanation",
  },
  "services.shellWebTitle": {
    message: "Shell web",
    description: "In-browser terminal card title",
  },
  "services.shellWebDescription": {
    message:
      "Ejecuta comandos dentro de una instancia en ejecución desde tu navegador.",
    description: "In-browser terminal card description",
  },
  "services.shellConnecting": {
    message: "Conectando a una instancia en ejecución…",
    description: "Web shell status while the terminal is connecting",
  },
  "services.shellConnected": {
    message: "Conectado",
    description: "Web shell status when the terminal stream is live",
  },
  "services.shellClosed": {
    message: "Sesión cerrada",
    description: "Web shell status when the terminal stream has ended",
  },
  "services.shellErrorStatus": {
    message: "Error de conexión",
    description:
      "Web shell status when the session failed to connect or errored",
  },
  "services.shellEnded": {
    message: "Sesión terminada",
    description:
      "Web shell status when the server ended a live session for a stated reason (access, suspension, instance stopped, time limit)",
  },
  "services.shellEndRevoked": {
    message: "Tu acceso a este servicio terminó.",
    description:
      "Web shell end reason: the caller lost access to the service mid-session (deliberately unspecific)",
  },
  "services.shellEndSuspended": {
    message:
      "El servicio se suspendió, lo que terminó esta sesión. Reanúdalo para abrir una nueva.",
    description: "Web shell end reason: the service was suspended",
  },
  "services.shellEndIneligible": {
    message:
      "Este servicio ya no tiene acceso a shell (cambió su plan o tipo), lo que terminó esta sesión.",
    description:
      "Web shell end reason: the service moved to the Free plan or a type without instances",
  },
  "services.shellEndInstanceGone": {
    message:
      "La instancia en la que se ejecutaba esta sesión se detuvo. Vuelve a conectarte para abrir un shell en una instancia en ejecución.",
    description:
      "Web shell end reason: the attached instance stopped (restart, or the old instance leaving after a deploy went live)",
  },
  "services.shellEndTimeout": {
    message:
      "La sesión alcanzó su límite de tiempo. Vuelve a conectarte para seguir trabajando.",
    description: "Web shell end reason: the session hit its maximum duration",
  },
  "services.shellEndUnverified": {
    message:
      "No se pudo volver a comprobar tu acceso, así que la sesión terminó. Vuelve a conectarte para intentarlo de nuevo.",
    description:
      "Web shell end reason: the periodic access check could not complete",
  },
  "services.shellReconnect": {
    message: "Reconectar",
    description: "Button that re-opens a closed web shell session",
  },
  "services.shellConnect": {
    message: "Iniciar shell",
    description: "Button that opens a web shell session",
  },
  "services.shellErrorGeneric": {
    message: "No se pudo iniciar una sesión de shell.",
    description: "Web shell generic connection error",
  },
  "services.shellErrorUnavailable": {
    message:
      "El shell en el navegador no está activado en esta plataforma. Usa el comando SSH de abajo.",
    description:
      "Web shell error when the browser transport is unconfigured (503)",
  },
  "services.shellInstanceLabel": {
    message: "Instancia",
    description: "Label for the web shell instance picker",
  },
  "services.shellInstanceAny": {
    message: "Cualquier instancia lista",
    description:
      "Web shell instance picker option that selects a random ready replica",
  },
  "services.shellInstanceSelect": {
    message: "Selecciona una instancia",
    description: "Placeholder for the web shell instance picker",
  },
  "services.shellSshFallback": {
    message: "¿Prefieres tu propio terminal? Usa el comando SSH de abajo.",
    description: "Hint pointing web shell users to the copy-ready SSH command",
  },
  "services.actions": {
    message: "Acciones",
    description: "Accessible heading for service configuration row actions",
  },
  "services.statTotal": {
    message: "Total de servicios",
    description: "Services page stat card label",
  },
  "services.statRunning": {
    message: "En ejecución",
    description: "Services page stat card label",
  },
  "services.statSuspended": {
    message: "Suspendidos",
    description: "Services page stat card label",
  },
  "services.cardTitle": {
    message: "Servicios",
    description:
      "Services table card title, also used as the metrics page back-link",
  },
  "services.colName": {
    message: "Nombre",
    description: "Services table column header",
  },
  "services.colStatus": {
    message: "Estado",
    description: "Services table column header",
  },
  "services.colUrl": {
    message: "URL",
    description: "Services table column header",
  },
  "services.colBlueprintName": {
    message: "Nombre en el Blueprint",
    description: "Immutable service name used to match Blueprint resources.",
  },
  "services.blueprintNameHelp": {
    message:
      "Usa este nombre inmutable en services[].name en el manifiesto de tu Blueprint. Los nombres para mostrar y los nombres de host públicos pueden ser distintos.",
    description: "Immutable service name used to match Blueprint resources.",
  },
  "services.colSlug": {
    message: "Slug",
    description:
      "Service detail header fact (globally-unique platform-host segment, Render's slug field)",
  },
  "services.instancesSuspended": {
    message: "0 ({n} al reanudar)",
    description:
      "The header's Instances fact for a suspended service, which runs none; n is its configured count (w4/547)",
  },
  "services.instancesSleeping": {
    message: "0 ({n} al despertar)",
    description:
      "The header's Instances fact for a sleeping Free service, which runs none; n is its configured count (w4/547)",
  },
  "services.colInstances": {
    message: "Instancias",
    description: "Services table column header (replica count — bex-native)",
  },
  "services.colRevision": {
    message: "Revisión",
    description: "Services table column header (active revision — bex-native)",
  },
  "services.colCreated": {
    message: "Creado",
    description: "Services table column header (relative age from createdAt)",
  },
  "services.colActions": {
    message: "Acciones",
    description: "Services table actions column header (screen-reader only)",
  },
  "services.statusRunning": {
    message: "En ejecución",
    description: "Services table status badge",
  },
  "services.statusSuspended": {
    message: "Suspendido",
    description: "Services table status badge",
  },
  "services.statusSleeping": {
    message: "En reposo",
    description:
      "Services status badge: a free-tier App auto-hibernated after idle (bex extension)",
  },
  "services.undeployedChangesSuspendedHint": {
    message:
      "Los cambios guardados aún no están activos. Reanuda el servicio y luego despliega para aplicarlos.",
    description:
      "Notice when a suspended service has saved settings that differ from the release it serves: no deploy can run until it resumes",
  },
  "services.undeployedChangesHint": {
    message:
      "Los cambios guardados aún no están activos. Usa un despliegue estándar para aplicarlos.",
    description:
      "Notice when saved settings differ from the running release, including Save only, cancellation, rollback or restart",
  },
  "services.undeployedChangesApplyingHint": {
    message:
      "Los cambios guardados se activarán cuando termine el despliegue en curso.",
    description:
      "Notice while an in-flight deploy already carries the saved settings, so no further deploy is needed (w4/m184)",
  },
  "services.undeployedChangesOverrideHint": {
    message:
      "Este despliegue ejecuta {running}, elegido para este despliegue. La imagen guardada es {saved}; un despliegue estándar vuelve a ella.",
    description:
      "Notice when a per-deploy image override (imageUrl/imgURL) runs a different image than the saved one: nothing was saved, and a standard deploy reverts to the saved image (w4/414)",
  },
  "services.statusSleepingHint": {
    message:
      "En reposo para ahorrar recursos: se activa con la siguiente solicitud a su URL pública.",
    description:
      "Hint next to the Sleeping badge explaining free-tier auto-sleep + wake-on-request",
  },
  "services.statusPending": {
    message: "Pendiente",
    description: "Services table status badge",
  },
  "services.statusBuilding": {
    message: "Compilando",
    description: "Services table status badge",
  },
  "services.statusDeploying": {
    message: "Desplegando",
    description: "Services table status badge",
  },
  "services.statusCanceled": {
    message: "Cancelado",
    description:
      "Services table status badge for a service whose first-ever deploy the user canceled",
  },
  "services.statusDeleting": {
    message: "Eliminando",
    description:
      "Services status badge for a service whose deletion is finalizing",
  },
  "services.statusFailed": {
    message: "Fallido",
    description: "Services table status badge",
  },
  "services.statusUnknown": {
    message: "Desconocido",
    description: "Services table status badge for an unrecognized phase",
  },
  "services.actionsMenu": {
    message: "Acciones de {name}",
    description: "Accessible label for the per-row actions trigger",
  },
  "services.actionSuspend": {
    message: "Suspender",
    description: "Row action: park the service",
  },
  "services.actionResume": {
    message: "Reanudar",
    description: "Row action: bring a suspended service back",
  },
  "services.actionRestart": {
    message: "Reiniciar",
    description:
      "Row action: restart the service on the commit or image it is running",
  },
  "services.confirmSuspendTitle": {
    message: "¿Suspender {name}?",
    description: "Suspend confirmation dialog title",
  },
  "services.confirmSuspendBody": {
    message:
      "El servicio se reduce a cero y deja de atender tráfico. Su URL y sus certificados se conservan, y puedes reanudarlo en cualquier momento.",
    description:
      "Suspend confirmation dialog body, for the types served at a public host",
  },
  "services.confirmSuspendBodyNoUrl": {
    message:
      "El servicio se reduce a cero y deja de ejecutarse. Puedes reanudarlo en cualquier momento.",
    description:
      "Suspend confirmation dialog body, for private services and background workers — neither has a public URL or certificates to keep",
  },
  "services.confirmSuspendBodyStatic": {
    message:
      "El sitio deja de servirse y responde 503 mientras esté suspendido. Su URL y sus certificados se conservan, y puedes reanudarlo en cualquier momento.",
    description:
      "Suspend confirmation dialog body for a static site, which has no instances to scale to zero (w4/380)",
  },
  "services.suspendCronDescription": {
    message:
      "Suspender este Cron Job pausa las próximas ejecuciones programadas. Una ejecución activa continúa. Para detenerla, selecciona Cancelar en la página Ejecuciones.",
    description:
      "Cron suspension explanation in Settings and suspend confirmations; suspension does not cancel an active run",
  },
  "services.confirmRestartTitle": {
    message: "¿Reiniciar {name}?",
    description: "Restart confirmation dialog title",
  },
  "services.confirmRestartBody": {
    message:
      "{name} se reinicia con el commit o la imagen que ejecuta ahora. Los commits enviados desde entonces no se despliegan. Las instancias nuevas reemplazan a las antiguas cuando están en buen estado.",
    description:
      "Restart confirmation dialog body (service header and services list). A restart keeps the running release, as on Render; it never picks up newer commits",
  },
  "services.confirmRestartBodyStatic": {
    message:
      "{name} se vuelve a publicar desde el commit que sirve ahora. Los commits enviados desde entonces no se despliegan.",
    description:
      "Restart confirmation body for a static site, which has no instances to replace (w4/m141)",
  },
  "services.confirmCancel": {
    message: "Cancelar",
    description: "Confirmation dialog cancel button",
  },
  "services.toastSuspendSuccess": {
    message: "Suspendiendo {name}…",
    description: "Toast shown after a suspend request is accepted",
  },
  "services.toastResumed": {
    message: "{name} se ha reanudado.",
    description:
      "Toast after a resume is accepted, Render's words (w3/m643); {name} is drawn in bold",
  },
  "services.toastResumeError": {
    message: "Se produjo un error al reanudar {name}. Inténtalo de nuevo.",
    description:
      "Toast when a resume fails and the server names no reason, Render's words (w3/m643); {name} is drawn in bold",
  },
  "services.toastRestartSuccess": {
    message: "Reiniciando {name}…",
    description: "Toast shown after a restart request is accepted",
  },
  "services.toastError": {
    message: "No se pudo actualizar {name}. Inténtalo de nuevo.",
    description: "Toast shown when a lifecycle action fails",
  },
  "services.errorTitle": {
    message: "No se pudieron cargar los servicios",
    description: "Services list error card title",
  },
  "services.errorBody": {
    message:
      "Falló la solicitud a bex-api. Comprueba tu conexión y vuelve a intentarlo.",
    description: "Services list error card body",
  },
  "services.detailErrorTitle": {
    message: "No se pudo cargar el servicio",
    description: "Service detail query error card title",
  },
  "services.detailErrorBody": {
    message:
      "Falló la solicitud a bex-api. Es posible que el servicio aún exista.",
    description: "Service detail query error card body",
  },
  "services.emptyTitle": {
    message: "Aún no hay servicios",
    description: "Services list empty state title",
  },
  "services.emptyBody": {
    message: "Despliega tu primera App y aparecerá aquí.",
    description: "Services list empty state body",
  },
  "services.headerServiceId": {
    message: "ID del servicio:",
    description: "Service-detail header metadata label for the service id",
  },
  "services.headerServicePhase": {
    message: "Servicio",
    description:
      "Service-detail header label distinguishing operator/App phase from deploy status",
  },
  "services.headerLatestDeploy": {
    message: "Último despliegue",
    description: "Service-detail header label for the newest deploy status",
  },
  "services.headerRuntime": {
    message: "Runtime",
    description: "Service-detail header label for the selected runtime",
  },
  "services.headerMoreFacts": {
    message: "Detalles",
    description:
      "Service-detail header toggle, phones only, that shows or hides the slug, instances, revision and age line",
  },
  "services.headerLastRun": {
    message: "Última ejecución correcta",
    description:
      "Service-detail header: a cron job's second fact line, before the last successful run's date (Render's \"Last successful run <date>\", w3/m555)",
  },
  "services.headerLastRunUtc": {
    message: "UTC: {time}",
    description:
      'Service-detail header: the tooltip on a cron job\'s last successful run date, shown only when the viewer\'s zone is not UTC; {time} is the instant in UTC as "Nov 21 05:00:00 PM" (Render\'s "UTC: <date>", w3/m643)',
  },
  "services.headerRuns": {
    message: "Ejecuta",
    description:
      "Service-detail header: the word before a cron job's command on its first fact line (Render's \"Runs <command> <schedule>\", w3/m555)",
  },
  "services.headerRunsDockerCmd": {
    message: "{cmd} de {dockerfile}",
    description:
      "Service-detail header: what a Docker or image cron job with no command override runs; {cmd} is the code word CMD and {dockerfile} the Dockerfile's path, both shown as code (Render's \"CMD in Dockerfile\", w3/m555)",
  },
  "services.headerNoSuccessfulRuns": {
    message: "Aún no hay ejecuciones correctas.",
    description:
      "Service-detail header: a cron job's second fact line before its first successful run (Render's text, w3/m555)",
  },
  "services.suspendedNoticeManual": {
    message: "Suspendido manualmente",
    description:
      "Title of the notice under a suspended service's header when its owner suspended it (Render's \"Manually suspended\", w3/m555, w3/107)",
  },
  "services.suspendedNoticeByYou": {
    message: "Suspendido por ti",
    description:
      "Title of the notice under a suspended service's header when its owner suspended it in a Hobby workspace; every other plan reads services.suspendedNoticeManual (Render's \"Suspended by you\", w3/m643)",
  },
  "services.suspendedNoticeTitle": {
    message: "Suspendido",
    description:
      "Title of the notice under a suspended service's header when no suspender is known (Render's fallback \"Suspended\", w3/m555, w3/107)",
  },
  "services.suspendedNoticeByPlatform": {
    message: "Suspendido por bex",
    description:
      "Title of the notice under a suspended service's header when the platform suspended it for spent Free instance hours or bandwidth (the billing suspender; Render's \"Suspended by Render\", w3/107)",
  },
  "services.suspendedNoticeCrashLoop": {
    message: "Este servicio se suspendió por fallos repetidos",
    description:
      "Title of the notice under a suspended service's header when the platform suspended it for a crash loop (stuck_crashlooping; Render's title, w3/107)",
  },
  "services.suspendedNoticeBody": {
    message: "No se te cobra por los servicios suspendidos.",
    description:
      "Body of the notice under a suspended service's header (Render's text, w3/m555, w3/107)",
  },
  "services.suspendedNoticeResume": {
    message: "Reanudar servicio",
    description:
      "The notice's inline action that resumes a suspended service (Render's \"Resume service\", w3/m555, w3/107)",
  },
  "services.headerCopyServiceId": {
    message: "Copiar ID del servicio",
    description: "Accessible label for the header's service-id copy button",
  },
  "services.headerCopyUrl": {
    message: "Copiar URL del servicio",
    description: "Accessible label for the header's live-URL copy button",
  },
  "services.headerCopied": {
    message: "Copiado al portapapeles",
    description: "Toast after copying a value from the service-detail header",
  },
  "services.headerCopyError": {
    message: "No se pudo copiar al portapapeles",
    description: "Toast when a service-detail header copy fails",
  },
  "services.navLogs": {
    message: "Logs",
    description: "Service-detail nav item (logs tab)",
  },
  "services.navMetrics": {
    message: "Métricas",
    description: "Service-detail nav item (metrics tab)",
  },
  "services.navRedirects": {
    message: "Redirecciones/Reescrituras",
    description:
      "Service-detail nav item (static-site route rules page, Render's label)",
  },
  "services.navHeaders": {
    message: "Encabezados",
    description:
      "Service-detail nav item (static-site response headers page, Render's label)",
  },
  "services.scalingTitle": {
    message: "Escalado automático",
    description: "Scaling tab section heading",
  },
  "services.autoscaleMinInstances": {
    message: "Instancias mínimas",
    description:
      "Accessible name of the minimum-instances number input beside the autoscaling range slider",
  },
  "services.autoscaleMaxInstances": {
    message: "Instancias máximas",
    description:
      "Accessible name of the maximum-instances number input beside the autoscaling range slider",
  },
  "services.scalingEnabled": {
    message: "Activar escalado automático",
    description: "Autoscaling enable/disable toggle label",
  },
  "services.scalingMinInstances": {
    message: "Mín. de instancias",
    description: "Autoscaling min instances input label",
  },
  "services.scalingMaxInstances": {
    message: "Máx. de instancias",
    description: "Autoscaling max instances input label",
  },
  "services.scalingTargetCPU": {
    message: "% objetivo de CPU",
    description: "Autoscaling target CPU utilization input label",
  },
  "services.scalingTargetMemory": {
    message: "% objetivo de memoria",
    description: "Autoscaling target memory utilization input label",
  },
  "services.scalingValidationMinMax": {
    message: "El mínimo de instancias debe ser ≤ el máximo de instancias.",
    description: "Autoscaling form validation when min exceeds max",
  },
  "services.scalingValidationTargets": {
    message:
      "Debe activarse al menos un objetivo de utilización (CPU o memoria).",
    description:
      "Autoscaling form validation when both CPU and memory targets are off",
  },
  "services.scalingSave": {
    message: "Guardar",
    description: "Autoscaling form save button",
  },
  "services.scalingSaved": {
    message: "Configuración de escalado automático guardada.",
    description: "Autoscaling save success toast",
  },
  "services.scalingDisabled": {
    message: "Escalado automático desactivado.",
    description: "Autoscaling disable success toast",
  },
  "services.scalingError": {
    message: "No se pudo actualizar la configuración de escalado automático.",
    description: "Autoscaling save error toast",
  },
  "services.scalingDescription": {
    message:
      "Escala este servicio automáticamente hacia arriba o hacia abajo según la utilización de CPU y memoria.",
    description: "Autoscaling card description",
  },
  "services.scalingOn": {
    message: "Escalado automático activado",
    description: "Autoscaling main toggle label when enabled",
  },
  "services.scalingPaidOnly": {
    message:
      "El escalado automático está disponible en los planes de pago. Mejora tu plan para activarlo.",
    description: "Autoscaling card explanation for a free service",
  },
  "services.scalingPaidOnlyInactive": {
    message:
      "La configuración de escalado automático está guardada pero inactiva en el plan Free. Se aplicará cuando este servicio pase a un plan de pago. Desactiva el escalado automático para quitar la configuración guardada.",
    description:
      "Autoscaling card explanation for a free service carrying a stored autoscaling config (w4/365)",
  },
  "services.scalingProPlanOnly": {
    message:
      "El escalado automático está disponible en el plan de espacio de trabajo Pro y superiores. Este espacio de trabajo está en Hobby.",
    description:
      "Autoscaling card notice on a Hobby workspace, followed by an Open Billing link (w7/m429)",
  },
  "services.scalingProPlanOnlyRunning": {
    message:
      "El escalado automático está disponible en el plan de espacio de trabajo Pro y superiores. Este espacio de trabajo está en Hobby: la política de abajo sigue funcionando y se puede desactivar, pero cambiarla o volver a activarla requiere Pro.",
    description:
      "Autoscaling card notice on a Hobby workspace whose service already autoscales, followed by an Open Billing link (w7/m429)",
  },
  "services.scalingMetricsUnavailableTitle": {
    message:
      "El escalado automático está en pausa: las métricas de utilización no están disponibles",
    description:
      "Autoscaling card notice title while the operator cannot read the service's CPU and memory utilization (w4/m364)",
  },
  "services.scalingMetricsUnavailableBody": {
    message:
      "El servicio mantiene su número actual de instancias hasta que se pueda volver a leer la utilización de CPU y memoria. Tu configuración de escalado automático está guardada.",
    description:
      "Autoscaling card notice body while utilization metrics are unavailable (w4/m364)",
  },
  "services.scalingOff": {
    message: "Escalado automático desactivado",
    description: "Autoscaling main toggle label when disabled",
  },
  "services.scalingInstancesTitle": {
    message: "Número de instancias",
    description: "Autoscaling instances range-slider section heading",
  },
  "services.scalingInstancesHint": {
    message:
      "bex escala el número de instancias de este servicio dentro del intervalo que especifiques.",
    description: "Autoscaling instances range-slider section description",
  },
  "services.scalingCPUTitle": {
    message: "Utilización de CPU objetivo",
    description: "Autoscaling CPU metric section heading",
  },
  "services.scalingCPUHint": {
    message:
      "Si la utilización media de CPU está muy por encima o por debajo de este valor, bex añade o quita instancias en consecuencia.",
    description: "Autoscaling CPU metric section description",
  },
  "services.scalingMemoryTitle": {
    message: "Utilización de memoria objetivo",
    description: "Autoscaling memory metric section heading",
  },
  "services.scalingMemoryHint": {
    message:
      "Si la utilización media de memoria está muy por encima o por debajo de este valor, bex añade o quita instancias en consecuencia.",
    description: "Autoscaling memory metric section description",
  },
  "services.scalingCancel": {
    message: "Cancelar",
    description: "Autoscaling form cancel button",
  },
  "services.scalingSaveChanges": {
    message: "Guardar cambios",
    description: "Autoscaling form save-changes button",
  },
  "services.scalingDisableConfirmTitle": {
    message: "¿Desactivar el escalado automático?",
    description: "Autoscaling disable confirmation dialog title (w7/m43)",
  },
  "services.scalingDisableConfirmBody": {
    message:
      "Tu servicio ejecutará el número fijo de instancias especificado en Escalado manual.",
    description: "Autoscaling disable confirmation dialog body",
  },
  "services.scalingDisableConfirmAction": {
    message: "Desactivar",
    description: "Autoscaling disable confirmation dialog confirm button",
  },
  "services.scalingManualTitle": {
    message: "Escalado manual",
    description: "Scaling page: manual instance-count card title (w7/m43)",
  },
  "services.scalingManualDescription": {
    message:
      "Ejecuta varias instancias con balanceo de carga automático. Todas las instancias usan el mismo plan de cómputo y se facturan en consecuencia.",
    description: "Scaling page: manual instance-count card description",
  },
  "services.scalingManualDescriptionFree": {
    message:
      "Los servicios gratuitos ejecutan una sola instancia. Mejora a un plan de pago para ejecutar varias instancias con balanceo de carga.",
    description:
      "Scaling page: manual instance-count card description on the free plan",
  },
  "services.scalingManualInstances": {
    message: "Instancias",
    description: "Scaling page: manual instance-count slider label",
  },
  "services.scalingMetricsTitle": {
    message: "Métricas recientes",
    description: "Scaling page: recent-metrics section title (w7/m43)",
  },
  "services.scalingMetricsNote": {
    message: "Mostrando las métricas de las últimas 48 horas.",
    description: "Scaling page: recent-metrics window note",
  },
  "services.scalingMetricsViewAll": {
    message: "Ver todas las métricas.",
    description: "Scaling page: link to the full Metrics tab",
  },
  "services.scalingMetricsMemory": {
    message: "Utilización media de memoria",
    description: "Scaling page: averaged memory utilization chart title",
  },
  "services.scalingMetricsCPU": {
    message: "Utilización media de CPU",
    description: "Scaling page: averaged CPU utilization chart title",
  },
  "services.scalingMetricsAcross": {
    message: "En todas las instancias",
    description: "Scaling page: recent-metrics chart subtitle",
  },
  "services.scalingMetricsEmpty": {
    message: "No se capturaron datos en las últimas 48 horas",
    description: "Scaling page: recent-metrics empty state",
  },
  "services.navEnvironment": {
    message: "Entorno",
    description: "Service-detail nav item (environment variables tab)",
  },
  "services.envTitle": {
    message: "Variables de entorno",
    description: "Environment tab card title",
  },
  "services.envDescription": {
    message:
      "Define la configuración y los secretos específicos del entorno y lee esos valores desde tu código.",
    description: "Environment tab card description",
  },
  "services.envColKey": {
    message: "Clave",
    description: "Environment table column header (variable name)",
  },
  "services.envColValue": {
    message: "Valor",
    description: "Environment table column header (variable value)",
  },
  "services.envManifestManaged": {
    message: "Gestionada por el Blueprint",
    description:
      "Badge on an environment row whose value the service's render.yaml manifest owns",
  },
  "services.envManifestManagedBody": {
    message:
      "Esta variable está declarada en el manifiesto render.yaml del servicio, que controla su valor. Edita el manifiesto y sincroniza el Blueprint para cambiarla, o decláralo con `sync: false` para que la gestione el panel.",
    description:
      "Explanation under a manifest-managed environment row: why it cannot be edited here and what to do instead",
  },
  "services.envShowSecret": {
    message: "Mostrar valor",
    description: "Environment row button to reveal a masked value",
  },
  "services.envHideSecret": {
    message: "Ocultar valor",
    description: "Environment row button to re-mask a revealed value",
  },
  "services.envRevealError": {
    message: "No se pudo cargar el valor.",
    description: "Environment row inline error when a value reveal fails",
  },
  "services.envEmptyTitle": {
    message: "Sin variables de entorno",
    description: "Environment tab empty-state title",
  },
  "services.envEmptyBody": {
    message: "Añade una variable para configurar este servicio.",
    description: "Environment tab empty-state body",
  },
  "services.envUnavailableTitle": {
    message: "Variables de entorno no disponibles",
    description:
      "Environment tab state when the secret store is unconfigured (503)",
  },
  "services.envUnavailableBody": {
    message: "El almacén de secretos no está configurado en este despliegue.",
    description: "Environment tab unavailable-state body",
  },
  "services.envForbiddenTitle": {
    message: "No autorizado",
    description: "Environment tab state when the caller lacks permission (403)",
  },
  "services.envForbiddenBody": {
    message:
      "No tienes permiso para ver las variables de entorno de este servicio.",
    description: "Environment tab forbidden-state body",
  },
  "services.envErrorTitle": {
    message: "No se pudieron cargar las variables de entorno",
    description: "Environment tab generic error title",
  },
  "services.envErrorBody": {
    message: "Algo salió mal. Inténtalo de nuevo.",
    description: "Environment tab generic error body",
  },
  "services.envAdd": {
    message: "Añadir variable",
    description: "Environment tab button to open the add-variable form",
  },
  "services.envExport": {
    message: "Exportar",
    description:
      "Environment tab button that downloads all service variables as dotenv",
  },
  "services.envExportSuccess": {
    message: "Entorno exportado",
    description:
      "Toast after every current environment value was safely exported",
  },
  "services.envExportError": {
    message:
      "No se pudo exportar el entorno completo. No se descargó ningún archivo.",
    description:
      "Fail-closed toast when any environment value cannot be revealed",
  },
  "services.envEdit": {
    message: "Editar",
    description: "Environment row button to edit a variable's value",
  },
  "services.envDelete": {
    message: "Eliminar",
    description: "Environment row button to remove a variable",
  },
  "services.envValueFor": {
    message: "Valor de {name}",
    description:
      "Accessible name for a Value textbox once its Key is set (m101/t013)",
  },
  "services.envDeleteOne": {
    message: "Eliminar {name}",
    description:
      "Accessible name for a Delete button once its Key is set (m101/t013)",
  },
  "services.envSave": {
    message: "Guardar",
    description: "Environment add/edit form save button",
  },
  "services.envCancel": {
    message: "Cancelar",
    description: "Environment add/edit form cancel button",
  },
  "services.envKeyPlaceholder": {
    message: "NAME_OF_VARIABLE",
    description: "Environment add-variable key input placeholder",
  },
  "services.envValuePlaceholder": {
    message: "valor",
    description: "Environment value input placeholder",
  },
  "services.envGenerate": {
    message: "Generar",
    description: "Generate a cryptographically random environment value",
  },
  "services.envGeneratePlaceholder": {
    message: "Se genera de forma segura al guardar",
    description:
      "Environment value placeholder while server generation is selected",
  },
  "services.envInvalidKey": {
    message:
      "Usa hasta 253 letras, dígitos, puntos, guiones y guiones bajos; no “.” ni empezar con “..”.",
    description:
      "Environment add-variable validation message for an invalid key",
  },
  "services.envDeleteConfirmTitle": {
    message: "¿Quitar {key}?",
    description: "Environment delete-confirmation dialog title",
  },
  "services.envDeleteConfirmBody": {
    message: "El servicio se volverá a desplegar sin esta variable.",
    description: "Environment delete-confirmation dialog body",
  },
  "services.envRolloutNote": {
    message:
      "El servicio se está volviendo a desplegar para aplicar el cambio.",
    description:
      "Toast description after an env-var write (bex rolls the pods)",
  },
  "services.envSaveSuccess": {
    message: "{key} guardada",
    description: "Toast on a successful env-var add/update",
  },
  "services.envSaveError": {
    message: "No se pudo guardar {key}",
    description: "Toast on a failed env-var add/update",
  },
  "services.envDeleteSuccess": {
    message: "{key} quitada",
    description: "Toast on a successful env-var delete",
  },
  "services.envDeleteError": {
    message: "No se pudo quitar {key}",
    description: "Toast on a failed env-var delete",
  },
  "services.secretFilesTitle": {
    message: "Archivos secretos",
    description: "Environment tab secret-files section title",
  },
  "services.secretFilesDescription": {
    message:
      "Guarda archivos con contenido secreto, como una clave privada o un archivo .env. Tras el próximo despliegue, tu servicio lee cada uno en /etc/secrets/<nombre-de-archivo>.",
    description: "Environment tab secret-files section description",
  },
  "services.secretFileColName": {
    message: "Nombre del archivo",
    description: "Secret-files table column header (file name)",
  },
  "services.secretFileColContent": {
    message: "Contenido",
    description: "Secret-files table column header (file body)",
  },
  "services.secretFileRevealError": {
    message:
      "No se pudo cargar el contenido de este archivo secreto. Cierra el diálogo e inténtalo de nuevo.",
    description:
      "Shown in the secret-file dialog when reading the stored contents fails.",
  },
  "services.secretFilesEmptyTitle": {
    message: "Sin archivos secretos",
    description: "Secret-files empty-state title",
  },
  "services.secretFilesEmptyBody": {
    message: "Añade un archivo para montar contenido secreto en este servicio.",
    description: "Secret-files empty-state body",
  },
  "services.secretFilesUnavailableTitle": {
    message: "Archivos secretos no disponibles",
    description:
      "Secret-files state when the secret store is unconfigured (503)",
  },
  "services.secretFilesUnavailableBody": {
    message: "El almacén de secretos no está configurado en este despliegue.",
    description: "Secret-files unavailable-state body",
  },
  "services.secretFilesForbiddenTitle": {
    message: "No autorizado",
    description: "Secret-files state when the caller lacks permission (403)",
  },
  "services.secretFilesForbiddenBody": {
    message:
      "No tienes permiso para ver los archivos secretos de este servicio.",
    description: "Secret-files forbidden-state body",
  },
  "services.secretFilesErrorTitle": {
    message: "No se pudieron cargar los archivos secretos",
    description: "Secret-files generic error title",
  },
  "services.secretFilesErrorBody": {
    message: "Algo salió mal. Inténtalo de nuevo.",
    description: "Secret-files generic error body",
  },
  "services.secretFileAdd": {
    message: "Añadir archivo secreto",
    description: "Secret-files button to open the add-file form",
  },
  "services.secretFileNamePlaceholder": {
    message: "filename.ext",
    description: "Secret-files add-file name input placeholder",
  },
  "services.secretFileContentPlaceholder": {
    message: "contenido del archivo",
    description: "Secret-files content input placeholder",
  },
  "services.secretFileInvalidName": {
    message:
      "Usa hasta 253 letras, dígitos, puntos, guiones y guiones bajos; no “.” ni “..”.",
    description: "Secret-files add-file validation message for an invalid name",
  },
  "services.secretFileNameHint": {
    message:
      "Usa hasta 253 letras, dígitos, puntos, guiones y guiones bajos; no “.” ni “..”.",
    description:
      "Neutral file-name rule under a new secret-file row before the user types or blurs — same rule as secretFileInvalidName but helper styling, not an error",
  },
  "services.secretFileDeleteConfirmTitle": {
    message: "¿Quitar {name}?",
    description: "Secret-file delete-confirmation dialog title",
  },
  "services.secretFileDeleteConfirmBody": {
    message: "El servicio se volverá a desplegar sin este archivo.",
    description: "Secret-file delete-confirmation dialog body",
  },
  "services.secretFileSaveSuccess": {
    message: "{name} guardado",
    description: "Toast on a successful secret-file add/update",
  },
  "services.secretFileSaveError": {
    message: "No se pudo guardar {name}",
    description: "Toast on a failed secret-file add/update",
  },
  "services.secretFileDeleteSuccess": {
    message: "{name} quitado",
    description: "Toast on a successful secret-file delete",
  },
  "services.secretFileDeleteError": {
    message: "No se pudo quitar {name}",
    description: "Toast on a failed secret-file delete",
  },
  "services.envGroupsTitle": {
    message: "Grupos de entorno",
    description: "Environment tab env-groups section title",
  },
  "services.envGroupsDescription": {
    message:
      "Conjuntos reutilizables de variables de entorno y archivos secretos que puedes vincular a este y a otros servicios.",
    description: "Environment tab env-groups section description",
  },
  "services.envGroupsEmptyTitle": {
    message: "Sin grupos de entorno",
    description: "Env-groups empty-state title",
  },
  "services.envGroupsEmptyBody": {
    message: "Crea un grupo para compartir configuración entre servicios.",
    description: "Env-groups empty-state body",
  },
  "services.envGroupsUnavailableTitle": {
    message: "Grupos de entorno no disponibles",
    description: "Env-groups state when the secret store is unconfigured (503)",
  },
  "services.envGroupsUnavailableBody": {
    message: "El almacén de secretos no está configurado en este despliegue.",
    description: "Env-groups unavailable-state body",
  },
  "services.envGroupsForbiddenTitle": {
    message: "No autorizado",
    description: "Env-groups state when the caller lacks permission (403)",
  },
  "services.envGroupsForbiddenBody": {
    message: "No tienes permiso para ver los grupos de entorno.",
    description: "Env-groups forbidden-state body",
  },
  "services.envGroupsErrorTitle": {
    message: "No se pudieron cargar los grupos de entorno",
    description: "Env-groups generic error title",
  },
  "services.envGroupsErrorBody": {
    message: "Algo salió mal. Inténtalo de nuevo.",
    description: "Env-groups generic error body",
  },
  "services.envGroupCreate": {
    message: "Crear grupo",
    description: "Env-groups button to open the create-group form",
  },
  "services.envGroupCreateSubmit": {
    message: "Crear",
    description: "Env-groups create-group form submit button",
  },
  "services.envGroupNamePlaceholder": {
    message: "group-name",
    description: "Env-groups create-group name input placeholder",
  },
  "services.envGroupNameLabel": {
    message: "Nombre del grupo",
    description: "Env-groups create-group name input accessible label",
  },
  "services.envGroupInvalidName": {
    message: "Introduce un nombre de grupo.",
    description:
      "Env-groups create-group validation message for an invalid name",
  },
  "services.envGroupLinked": {
    message: "Vinculado",
    description:
      "Env-groups badge: this group is linked to the current service",
  },
  "services.envGroupWorkspaceScope": {
    message: "Espacio de trabajo",
    description: "Scope label for a workspace-scoped environment group",
  },
  "services.envGroupOutOfScope": {
    message:
      "Con alcance {groupScope}, pero este servicio está en {serviceScope}; sus variables y archivos se siguen aplicando.",
    description:
      "Notice on a linked group whose scope no longer matches the service (w4/m238)",
  },
  "services.envGroupOutOfScopeNoEnvironment": {
    message:
      "Con alcance {groupScope}, pero este servicio no tiene entorno; sus variables y archivos se siguen aplicando.",
    description:
      "Notice on a linked group scoped to an environment while the service has none (w4/m238)",
  },
  "services.envGroupEmptyContents": {
    message: "Aún no hay variables ni archivos.",
    description: "Env-groups: shown when a group has no vars or secret files",
  },
  "services.envGroupKeyOverridden": {
    message: "Anulada por la variable de entorno {key} del propio servicio",
    description:
      "Tooltip on a linked group's key badge when a service variable of the same name shadows it at runtime (service wins)",
  },
  "services.envGroupFileOverridden": {
    message: "Anulado por el archivo secreto {name} del propio servicio",
    description:
      "Tooltip on a linked group's secret-file badge when a service secret file of the same name shadows it at runtime (service wins)",
  },
  "services.envGroupKeyShadowedByGroup": {
    message: "Reemplazada por {group}, que se creó más tarde",
    description:
      "Tooltip on a linked group's key badge when a MORE recently created linked group defines the same key and wins it",
  },
  "services.envGroupFileShadowedByGroup": {
    message: "Reemplazado por {group}, que se creó más tarde",
    description:
      "Tooltip on a linked group's secret-file badge when a MORE recently created linked group defines the same file name and wins it",
  },
  "services.envGroupOverriddenNote": {
    message:
      "Las claves tachadas están anuladas por las variables de entorno del propio servicio.",
    description:
      "Legend under a linked group's key list when at least one key is shadowed by a service variable",
  },
  "services.envGroupLink": {
    message: "Vincular",
    description: "Env-groups button: attach this group to the current service",
  },
  "services.envGroupUnlink": {
    message: "Desvincular",
    description:
      "Env-groups button: detach this group from the current service",
  },
  "services.envGroupLinkAria": {
    message: "Vincular {name}",
    description:
      "Accessible name of an env-group row's Link button on the service Environment tab, naming the group (w4/537)",
  },
  "services.envGroupUnlinkAria": {
    message: "Desvincular {name}",
    description:
      "Accessible name of an env-group row's Unlink button on the service Environment tab, naming the group (w4/537)",
  },
  "services.envGroupDelete": {
    message: "Eliminar",
    description: "Env-groups action: delete the group",
  },
  "services.envGroupDeleteConfirmTitle": {
    message: "¿Eliminar {name}?",
    description: "Env-group delete-confirmation dialog title",
  },
  "services.envGroupDeleteConfirmBody": {
    message:
      "El grupo se quita de todos los servicios a los que está vinculado. Esta acción no se puede deshacer.",
    description: "Env-group delete-confirmation dialog body",
  },
  "services.envGroupCreateSuccess": {
    message: "{name} creado",
    description: "Toast on a successful env-group create",
  },
  "services.envGroupCreateError": {
    message: "No se pudo crear {name}",
    description: "Toast on a failed env-group create",
  },
  "services.envGroupDeleteSuccess": {
    message: "Grupo eliminado",
    description: "Toast on a successful env-group delete",
  },
  "services.envGroupDeleteError": {
    message: "No se pudo eliminar el grupo",
    description: "Toast on a failed env-group delete",
  },
  "services.envGroupLinkSuccess": {
    message: "Grupo vinculado",
    description: "Toast on a successful env-group link",
  },
  "services.envGroupLinkError": {
    message: "No se pudo vincular el grupo",
    description: "Toast on a failed env-group link",
  },
  "services.envGroupUnlinkSuccess": {
    message: "Grupo desvinculado",
    description: "Toast on a successful env-group unlink",
  },
  "services.envGroupUnlinkError": {
    message: "No se pudo desvincular el grupo",
    description: "Toast on a failed env-group unlink",
  },
  "services.navSettings": {
    message: "Configuración",
    description: "Service-detail nav item (settings tab)",
  },
  "services.settingsNavigation": {
    message: "Secciones de configuración",
    description: "Accessible label for the service settings section navigation",
  },
  "services.settingsDescription": {
    message:
      "Configura el nombre, el tamaño de instancia y otros ajustes de este servicio.",
    description: "Settings tab card description",
  },
  "services.generalTitle": {
    message: "General",
    description:
      "Settings tab: first card heading, Render's 'General' section (w5/m52)",
  },
  "services.regionLabel": {
    message: "Región",
    description:
      "Settings General card: read-only platform placement row label",
  },
  "services.regionHint": {
    message:
      "La región donde se ejecuta tu servicio. Los servicios de la misma región pueden comunicarse por la red privada.",
    description: "Settings General card: Region row explanatory copy",
  },
  "services.editRowSave": {
    message: "Guardar cambios",
    description:
      "Shared editable-field row: save button shown while editing (Render parity)",
  },
  "services.editRowCancel": {
    message: "Cancelar",
    description: "Shared editable-field row: cancel button shown while editing",
  },
  "services.editRowOptional": {
    message: "Opcional",
    description:
      "Shared editable-field row: badge next to a label for an optional field",
  },
  "services.displayNameLabel": {
    message: "Nombre del servicio",
    description:
      "Settings tab row label for the mutable human-facing service name",
  },
  "services.displayNameHint": {
    message:
      "El ID del servicio sigue siendo {id}; las URL y la infraestructura no cambian.",
    description:
      "Settings tab explanation that a display-name change preserves identity",
  },
  "services.displayNameEdit": {
    message: "Editar nombre del servicio",
    description: "Accessible label for the service-name edit button",
  },
  "services.displayNameSuccess": {
    message: "Servicio renombrado a “{name}”.",
    description: "Toast after setDisplayName succeeds",
  },
  "services.displayNameCleared": {
    message: "Nombre personalizado del servicio borrado.",
    description: "Toast after clearing displayName",
  },
  "services.displayNameRefreshFailed": {
    message:
      "Renombrado a “{name}”, pero esta página no pudo actualizarse. Inténtalo de nuevo o recarga para ver el nombre guardado.",
    description:
      "Warning after a service rename committed but the page could not re-read the saved name; offers Try again",
  },
  "services.displayNameUnpublished": {
    message:
      "El nuevo nombre se guardó, pero esta página aún no pudo mostrarlo. Usa Intentar de nuevo en el aviso o recarga antes de volver a editarlo.",
    description:
      "Why the service name row is locked after a rename saved but its re-read failed",
  },
  "services.displayNameError": {
    message: "No se pudo renombrar el servicio. Inténtalo de nuevo.",
    description: "Toast after setDisplayName fails",
  },
  "services.settingsInstanceType": {
    message: "Plan de cómputo",
    description: "Settings tab row label for the App's current plan/tier",
  },
  "services.settingsNoInstanceType": {
    message: "Sin plan de cómputo definido",
    description: "Settings tab state for an untiered (bare-CR) App",
  },
  "services.settingsUpdate": {
    message: "Actualizar",
    description: "Settings tab link to the compute-plan picker",
  },
  "services.settingsIdleTimeout": {
    message: "Tiempo de inactividad",
    description:
      "Settings tab: label for the free-tier auto-sleep window control",
  },
  "services.settingsIdleTimeoutHint": {
    message:
      "Los servicios gratuitos entran en reposo tras este tiempo sin solicitudes HTTP públicas ni mensajes WebSocket, y se activan con la siguiente solicitud pública. Las solicitudes internas de otros servicios ni los mantienen activos ni los despiertan.",
    description: "Settings tab: idle-timeout control help text (bex extension)",
  },
  "services.settingsIdleTimeoutPaid": {
    message:
      "Los servicios de pago permanecen siempre activos y nunca entran en reposo.",
    description: "Settings tab: shown instead of the control on a paid plan",
  },
  "services.idleTimeoutDefault": {
    message: "Valor predeterminado de la plataforma",
    description: "Idle-timeout option: 0 seconds = the operator's own window",
  },
  "services.idleTimeoutMinutes": {
    message: "{minutes} min",
    description: "Idle-timeout option label in minutes",
  },
  "services.idleTimeoutHours": {
    message: "{hours} h",
    description: "Idle-timeout option label in hours",
  },
  "services.idleTimeoutSeconds": {
    message: "{seconds} s",
    description: "Idle-timeout option label in seconds (non-round values)",
  },
  "services.idleTimeoutSuccess": {
    message: "Tiempo de inactividad actualizado.",
    description: "Toast after setIdleTimeout succeeds",
  },
  "services.idleTimeoutError": {
    message: "No se pudo actualizar el tiempo de inactividad.",
    description: "Toast after setIdleTimeout fails",
  },
  "services.settingsMaxShutdownDelay": {
    message: "Retraso máximo de apagado",
    description: "Settings row label for the graceful SIGTERM window",
  },
  "services.settingsMaxShutdownDelayHint": {
    message:
      "Espera de 1 a 300 segundos tras SIGTERM antes de forzar la detención del proceso (por defecto 30).",
    description: "Settings row help for the shutdown-delay range",
  },
  "services.maxShutdownDelayDiskReason": {
    message:
      "No puedes cambiar el retraso de apagado de un servicio con un disco persistente.",
    description:
      "Settings hint replacing the range on a service with a disk (Render's rule, render.com/docs/disks); a link to the Disk tab follows",
  },
  "services.maxShutdownDelayDiskLink": {
    message: "Ver disco",
    description:
      "Link after maxShutdownDelayDiskReason to the service's Disk tab",
  },
  "services.maxShutdownDelayEdit": {
    message: "Editar retraso máximo de apagado",
    description: "Accessible label for the shutdown-delay edit button",
  },
  "services.maxShutdownDelaySuccess": {
    message:
      "Retraso máximo de apagado actualizado. Se inició un nuevo despliegue.",
    description: "Toast after setMaxShutdownDelay succeeds",
  },
  "services.maxShutdownDelayRangeError": {
    message: "Introduce un número entero de segundos entre {min} y {max}.",
    description:
      "Inline validation message for a max shutdown delay outside 1–300 or not a whole number (w4/494)",
  },
  "services.maxShutdownDelayError": {
    message: "No se pudo actualizar el retraso máximo de apagado.",
    description: "Toast after setMaxShutdownDelay fails",
  },
  "services.settingsHealthChecksTitle": {
    message: "Comprobaciones de estado",
    description: "Settings tab: Health Checks section card title",
  },
  "services.settingsHealthChecksDescription": {
    message:
      "Configura la ruta HTTP que bex consulta periódicamente para supervisar tu servicio.",
    description: "Settings tab: Health Checks section card description",
  },
  "services.settingsHealthCheckPath": {
    message: "Ruta de comprobación de estado",
    description: "Settings tab: health-check path row label",
  },
  "services.settingsHealthCheckPathHint": {
    message:
      "Una ruta HTTP que bex consulta para decidir si tu servicio está en buen estado; debe responder 2xx o 3xx en 5 segundos, y 60 segundos de fallos consecutivos reinician la instancia: apúntala a una ruta barata. Déjala vacía para comprobar solo que el proceso escucha en su puerto, la opción adecuada cuando el servicio no tiene una ruta barata que devuelva 2xx.",
    description: "Settings tab: health-check path row hint text",
  },
  "services.settingsHealthCheckPathPlaceholder": {
    message: "Vacía: comprobar solo el puerto",
    description: "Settings tab: health-check path input placeholder",
  },
  "services.settingsHealthCheckPathEdit": {
    message: "Editar ruta de comprobación de estado",
    description:
      "Settings tab: accessible label for the health-check path edit-pencil button",
  },
  "services.healthCheckPathSuccess": {
    message:
      "Ruta de comprobación de estado actualizada. Se inició un nuevo despliegue.",
    description: "Toast after setHealthCheckPath succeeds",
  },
  "services.healthCheckPathError": {
    message: "No se pudo actualizar la ruta de comprobación de estado.",
    description: "Toast after setHealthCheckPath fails",
  },
  "services.settingsPortTitle": {
    message: "Puerto",
    description: "Settings tab: Port section card title",
  },
  "services.settingsPortDescription": {
    message:
      "El puerto en el que escucha tu contenedor. bex dirige el tráfico hacia él y lo inyecta como $PORT.",
    description: "Settings tab: Port section card description",
  },
  "services.settingsPort": {
    message: "Puerto",
    description: "Settings tab: service port row label",
  },
  "services.settingsPortHint": {
    message:
      "El puerto en el que escucha tu contenedor (por defecto {port}). bex dirige el tráfico hacia él y lo inyecta como $PORT. Una variable de entorno PORT en el servicio lo anula, como en Render. Guardar inicia un nuevo despliegue y reinicia las instancias.",
    description: "Settings tab: service port row hint text",
  },
  "services.settingsPortDetected": {
    message: "Detectado: {port}",
    description:
      "Settings tab: port placeholder for a service whose serving port bex detected from the listener its process binds, while it names no port of its own (w8/338)",
  },
  "services.settingsPortDetectedHint": {
    message:
      "bex detectó que tu servicio escucha en el puerto {port} y dirige el tráfico hacia él. $PORT sigue siendo {defaultPort}, como en Render. Introduce un puerto para fijarlo: bex dirigirá el tráfico hacia él y lo inyectará como $PORT. Guardar inicia un nuevo despliegue y reinicia las instancias.",
    description:
      "Settings tab: service port row hint for a service serving on a detected port; PORT stays the default (w8/177, w8/267, w8/338)",
  },
  "services.settingsPortImageHint": {
    message:
      "Este servicio toma sus puertos de las instrucciones EXPOSE de su imagen. Definir un puerto aquí selecciona el listener principal (el puerto al que bex dirige el tráfico, comprueba el estado e inyecta como $PORT), mientras que los demás puertos TCP declarados por la imagen siguen publicados. Bórralo para volver a usar el puerto de la imagen. Guardar inicia un nuevo despliegue y reinicia las instancias.",
    description:
      "Settings tab: service port row hint for an image-port service (a Docker-built or prebuilt-image web or private service) whose ports come from its image's EXPOSE instructions; a blank port returns it to the image's (w6/m148, w4/m243, w4/m323)",
  },
  "services.settingsPortEdit": {
    message: "Editar puerto",
    description:
      "Settings tab: accessible label for the service port edit-pencil button",
  },
  "services.portRangeError": {
    message: "Introduce un puerto entre {min} y {max}.",
    description:
      "Validation message for a port outside the range the container can bind",
  },
  "services.portSuccess": {
    message: "Puerto actualizado. Se está desplegando una nueva versión.",
    description: "Toast after setPort succeeds",
  },
  "services.portError": {
    message: "No se pudo actualizar el puerto.",
    description: "Toast after setPort fails",
  },
  "services.settingsNotificationsTitle": {
    message: "Notificaciones",
    description: "Settings tab: Notifications section card title",
  },
  "services.settingsNotificationsDescription": {
    message:
      "Elige qué notificaciones de despliegue recibir para este servicio.",
    description: "Settings tab: Notifications section card description",
  },
  "services.notificationsLabel": {
    message: "Notificaciones del servicio",
    description: "Settings tab: service notification policy label",
  },
  "services.notificationsHint": {
    message:
      "Anula el valor predeterminado del espacio de trabajo para este servicio. El valor predeterminado se define en Configuración del espacio de trabajo → Notificaciones.",
    description: "Settings tab: service notification policy explanation",
  },
  "services.notificationsEdit": {
    message: "Editar notificaciones del servicio",
    description: "Accessible label for the service-notifications edit button",
  },
  "services.notificationsOptionDefault": {
    message: "Usar el valor predeterminado del espacio de trabajo ({level})",
    description:
      "Service notification option: inherit workspace default; {level} is the workspace's saved level (e.g. Only failure notifications)",
  },
  "services.notificationsOptionDefaultUnknown": {
    message: "Usar el valor predeterminado del espacio de trabajo",
    description:
      "Service notification option: inherit workspace default, while the workspace level is loading or unreadable",
  },
  "services.notificationsOptionAll": {
    message: "Todas las notificaciones",
    description: "Service notification option: all lifecycle mail",
  },
  "services.notificationsOptionFailure": {
    message: "Solo notificaciones de fallos",
    description: "Service notification option: failures only",
  },
  "services.notificationsOptionNone": {
    message: "Ninguna",
    description: "Service notification option: no mail",
  },
  "services.notificationsSuccess": {
    message: "Configuración de notificaciones del servicio actualizada.",
    description: "Toast after notification policy update succeeds",
  },
  "services.notificationsError": {
    message:
      "No se pudo actualizar la configuración de notificaciones del servicio.",
    description: "Toast after notification policy update fails",
  },
  "services.notifyOnFailLabel": {
    message: "Notificaciones de fallos de despliegue",
    description: "Settings tab: notifyOnFail row label",
  },
  "services.notifyOnFailHint": {
    message:
      "Por defecto se respeta la preferencia de notificaciones de cada miembro; puedes forzarla a activada o desactivada solo para este servicio.",
    description: "Settings tab: notifyOnFail row hint text",
  },
  "services.notifyOnFailOptionDefault": {
    message: "Usar la preferencia del miembro",
    description: "notifyOnFail select option: default",
  },
  "services.notifyOnFailOptionNotify": {
    message: "Notificar siempre",
    description: "notifyOnFail select option: notify",
  },
  "services.notifyOnFailOptionIgnore": {
    message: "No notificar nunca",
    description: "notifyOnFail select option: ignore",
  },
  "services.notifyOnFailSuccess": {
    message: "Configuración de notificaciones actualizada.",
    description: "Toast after setNotifyOnFail succeeds",
  },
  "services.notifyOnFailError": {
    message: "No se pudo actualizar la configuración de notificaciones.",
    description: "Toast after setNotifyOnFail fails",
  },
  "services.planPickerTitle": {
    message: "Plan de cómputo",
    description:
      "Accessible name of the Compute section's plan cards, shown after Edit (w6/m483)",
  },
  "services.computeTitle": {
    message: "Cómputo",
    description:
      "Compute page: title of the section holding the service's compute plan (Render: 'Under Compute, click Edit', w6/m483)",
  },
  "services.computePlanEdit": {
    message: "Editar",
    description:
      "Compute section button that opens the compute plan cards (w6/m483)",
  },
  "services.planPickerFreeGroup": {
    message: "Gratuito",
    description:
      "Plan-picker section label separating the Free tier from paid tiers",
  },
  "services.planPickerFreeSmtp": {
    message:
      "Los servicios gratuitos no pueden enviar correo por los puertos SMTP 25, 465 ni 587.",
    description:
      "Plan picker note under the Free group: free instances are refused outbound SMTP, as on Render (w8/m127)",
  },
  "services.planPickerPaidSmtp": {
    message:
      "Los servicios de pago pueden enviar correo por el puerto 587 o 2525 con STARTTLS. Los puertos 25 y 465 no están disponibles en ningún plan.",
    description:
      "Plan picker note under the Paid group: outbound SMTP ports paid services may use (w8/m127)",
  },
  "services.planPickerLegacyFreeWorker": {
    message:
      "Este Background Worker está en el plan gratuito, que ya no pueden usar los Background Workers. Sigue en ejecución. Al elegir un plan de cómputo de pago sale del plan gratuito y no puede volver.",
    description:
      "Plan picker notice: this legacy service type is still on Free, which is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerUntieredWorker": {
    message:
      "Este Background Worker no tiene plan de cómputo, y los Background Workers ya no pueden usar el plan gratuito. Sigue en ejecución. Elige un plan de cómputo de pago para asignarle uno.",
    description:
      "Plan picker notice: this service type has no compute plan, and Free is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerLegacyFreePrivateService": {
    message:
      "Este Private Service está en el plan gratuito, que ya no pueden usar los Private Services. Sigue en ejecución. Al elegir un plan de cómputo de pago sale del plan gratuito y no puede volver.",
    description:
      "Plan picker notice: this legacy service type is still on Free, which is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerUntieredPrivateService": {
    message:
      "Este Private Service no tiene plan de cómputo, y los Private Services ya no pueden usar el plan gratuito. Sigue en ejecución. Elige un plan de cómputo de pago para asignarle uno.",
    description:
      "Plan picker notice: this service type has no compute plan, and Free is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerLegacyFreeCronJob": {
    message:
      "Este Cron Job está en el plan gratuito, que ya no pueden usar los Cron Jobs. Sigue en ejecución. Al elegir un plan de cómputo de pago sale del plan gratuito y no puede volver.",
    description:
      "Plan picker notice: this legacy service type is still on Free, which is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerUntieredCronJob": {
    message:
      "Este Cron Job no tiene plan de cómputo, y los Cron Jobs ya no pueden usar el plan gratuito. Sigue en ejecución. Elige un plan de cómputo de pago para asignarle uno.",
    description:
      "Plan picker notice: this service type has no compute plan, and Free is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerPaidGroup": {
    message: "De pago",
    description: "Plan-picker section label for the paid tier ladder",
  },
  "services.planCurrent": {
    message: "Actual",
    description:
      "Plan picker: badge on the instance type the service is on now",
  },
  "services.planPickerCancel": {
    message: "Cancelar",
    description: "Plan-picker footer button: discard the selection",
  },
  "services.planPickerSave": {
    message: "Guardar",
    description:
      'Compute section footer button: confirm the plan change ("Select a new compute plan and click Save", render.com/docs/compute-plans)',
  },
  "services.planPickerConfirmTitle": {
    message: "¿Cambiar el plan de cómputo a {name}?",
    description: "Plan-change confirm dialog title",
  },
  "services.planPickerConfirmBody": {
    message:
      "El servicio se redimensiona y se actualiza sin tiempo de inactividad: las solicitudes en curso terminan antes de reemplazar las instancias antiguas.",
    description: "Plan-change confirm dialog body",
  },
  "services.instanceTypePausedReason": {
    message:
      "“{name}” no está disponible temporalmente; elige {largest} o un plan de cómputo menor.",
    description:
      "A paused compute plan's tooltip and screen-reader reason, composed in the dashboard's language from the tier and the largest still offered; matches bex-api's English unavailableReason (w4/552)",
  },
  "services.planPickerConfirmBodyCron": {
    message:
      "Las ejecuciones que empiecen después del cambio usan el nuevo plan de cómputo.",
    description:
      "Plan-change confirm body for a cron job, which serves no requests: its next run takes the new size (w4/548)",
  },
  "services.planPickerConfirmBodyNeverLive": {
    message:
      "El servicio se ejecuta en el nuevo plan de cómputo. Aún no sirve nada: pasa a estar activo cuando un despliegue tenga éxito.",
    description:
      "Plan-change confirm body for a service that has never gone live (w4/548)",
  },
  "services.planPickerConfirmRunPrice": {
    message:
      "Este plan de cómputo cuesta ${price} por minuto de ejecución, facturado por segundo mientras se ejecuta. El cargo mínimo es de ${minimum} al mes por Cron Job.",
    description:
      "Plan-change confirm for a cron job: the new compute plan's run-time rate (w4/546); minimum is the cron job minimum in USD without the currency symbol (w9/m448)",
  },
  "services.createFieldPlanCronHint": {
    message:
      "Un Cron Job se factura por segundo mientras se ejecuta. El cargo mínimo es de ${minimum} al mes por Cron Job.",
    description:
      "Under the create form's compute plans for a cron job, which are priced per minute of run time (w4/546); minimum is the cron job minimum in USD without the currency symbol (w9/m448)",
  },
  "services.planPickerConfirmPrice": {
    message: "Este plan de cómputo cuesta ${price}/mes.",
    description:
      "Plan-change confirm: the new compute plan's monthly price, from the API's price sheet (w4/400)",
  },
  "services.planPickerSuccess": {
    message: "Plan de cómputo actualizado a {name}",
    description: "Toast on a successful plan change",
  },
  "services.planPickerError": {
    message: "No se pudo actualizar el plan de cómputo. Inténtalo de nuevo.",
    description: "Toast on a failed plan change",
  },
  "services.planPickerErrorTitle": {
    message: "No se pudieron cargar los planes de cómputo",
    description: "Plan-picker error state title (instanceTypes query failed)",
  },
  "services.planPickerErrorBody": {
    message:
      "Falló la solicitud a bex-api. Comprueba tu conexión y vuelve a intentarlo.",
    description: "Plan-picker error state body",
  },
  "services.domainsTitle": {
    message: "Dominios personalizados",
    description: "Settings tab custom-domains section title",
  },
  "services.domainsDescription": {
    message:
      "Apunta a este servicio los dominios personalizados de tu propiedad.",
    description: "Settings tab custom-domains section description",
  },
  "services.domainColName": {
    message: "Nombre",
    description: "Custom-domains table column header (the FQDN)",
  },
  "services.domainColVerified": {
    message: "Estado de verificación",
    description: "Custom-domains table column header (DNS verification)",
  },
  "services.domainColCertificate": {
    message: "Estado del certificado",
    description:
      "Custom-domains table column header (TLS certificate serving state)",
  },
  "services.domainColActions": {
    message: "Acciones",
    description:
      "Custom-domains table actions column header (screen-reader only)",
  },
  "services.domainVerified": {
    message: "Verificado",
    description:
      "Custom-domains status badge: DNS or TLS verification is complete",
  },
  "services.domainCertActive": {
    message: "Activo",
    description:
      "Custom-domains status badge: certificate issued and serving traffic",
  },
  "services.domainCertBlockedLabel": {
    message: "Certificado bloqueado:",
    description:
      "Label before cert-manager's verbatim reason a custom domain's TLS certificate has not been issued",
  },
  "services.domainCertBlockedHint": {
    message:
      "El registro DNS debe ser solo DNS (sin proxy) y apuntar al nombre de host de bex de tu servicio, o la comprobación del certificado no podrá alcanzarlo.",
    description:
      "Fix-it guidance shown with a blocked-certificate reason: proxied records (e.g. Cloudflare's orange cloud) break the ACME HTTP-01 challenge",
  },
  "services.domainPending": {
    message: "Pendiente",
    description:
      "Custom-domains status badge: DNS or certificate is not complete",
  },
  "services.domainActionsMenu": {
    message: "Acciones de {domain}",
    description: "Accessible label for the per-domain actions trigger",
  },
  "services.domainDelete": {
    message: "Eliminar",
    description: "Custom-domains row action: remove the domain",
  },
  "services.domainCancel": {
    message: "Cancelar",
    description: "Custom-domains dialog cancel button",
  },
  "services.domainDeleteConfirmTitle": {
    message: "¿Eliminar {name}?",
    description: "Custom-domain delete-confirmation dialog title",
  },
  "services.domainDeleteConfirmBody": {
    message:
      "El servicio deja de servir este dominio. Se quita su regla de Ingress y el certificado TLS se deja caducar. Esta acción no se puede deshacer.",
    description: "Custom-domain delete-confirmation dialog body",
  },
  "services.domainDeletePairConfirmBody": {
    message:
      "Esto elimina {name} y su redirección añadida automáticamente {sibling}. Se quitan ambas reglas de Ingress y sus certificados TLS se dejan caducar. Esta acción no se puede deshacer.",
    description:
      "Custom-domain delete confirmation when deleting a canonical host also removes its generated redirecting sibling",
  },
  "services.domainDeleteGeneratedConfirmBody": {
    message:
      "Esto elimina solo {name}. {canonical} sigue siendo el dominio canónico y no redirige. Se quita la regla de Ingress del dominio eliminado y su certificado TLS se deja caducar. Esta acción no se puede deshacer.",
    description:
      "Custom-domain delete confirmation when deleting only the generated redirecting sibling",
  },
  "services.domainDeletePendingConfirmBody": {
    message:
      "Esto elimina la solicitud pendiente de {name}. Nunca se verificó, por lo que no se sirvió nada en él y no se emitió ningún certificado.",
    description:
      "Custom-domain delete confirmation for a domain still unverified: it was never routed (w4/142)",
  },
  "services.domainDeletePendingPairConfirmBody": {
    message:
      "Esto elimina las solicitudes pendientes de {name} y {sibling}. Ninguno se verificó, por lo que no se sirvió nada en ellos y no se emitió ningún certificado.",
    description:
      "Custom-domain delete confirmation for a still-pending domain and its auto-paired sibling (w4/142)",
  },
  "services.domainDeletePendingGeneratedConfirmBody": {
    message:
      "Esto elimina solo la solicitud pendiente de {name}. {canonical} sigue como dominio canónico. Nunca se verificó, por lo que no se sirvió nada en él y no se emitió ningún certificado.",
    description:
      "Custom-domain delete confirmation for a still-pending generated sibling: only it is removed, its canonical claim stays (w4/539)",
  },
  "services.domainAdd": {
    message: "Añadir dominio personalizado",
    description: "Custom-domains button to open the add-domain dialog",
  },
  "services.domainAddTitle": {
    message: "Añadir un dominio personalizado",
    description: "Add-domain dialog title",
  },
  "services.domainAddDescription": {
    message:
      "Introduce un dominio de tu propiedad. Apunta su DNS a este servicio y bex emitirá un certificado TLS automáticamente.",
    description: "Add-domain dialog description",
  },
  "services.domainPlaceholder": {
    message: "www.example.com",
    description: "Add-domain FQDN input placeholder",
  },
  "services.domainInvalid": {
    message: "Introduce un dominio válido, p. ej. www.example.com.",
    description: "Add-domain validation message for a malformed hostname",
  },
  "services.domainAddButton": {
    message: "Añadir dominio",
    description: "Add-domain dialog submit button",
  },
  "services.domainAddPairSuccess": {
    message: "Se añadieron {name} y {sibling}",
    description:
      "Toast on a custom-domain add that auto-paired its www/apex sibling (w4/618)",
  },
  "services.domainAddSuccess": {
    message: "Se añadió {name}",
    description: "Toast on a successful custom-domain add",
  },
  "services.domainAddError": {
    message: "No se pudo añadir {name}",
    description: "Toast on a failed custom-domain add",
  },
  "services.domainAddConflict": {
    message: "{name} ya lo usa otro servicio",
    description:
      "Toast when a custom-domain add is rejected because the host is registered on another service (409)",
  },
  "services.domainAddReserved": {
    message: "{name} es un nombre de host reservado de la plataforma",
    description:
      "Toast when a custom-domain add is rejected because the host is a platform-owned name (400)",
  },
  "services.domainAddOwnPlatformHost": {
    message:
      "{name} ya es el subdominio de plataforma de este servicio, por lo que no se puede añadir como dominio personalizado",
    description:
      "Toast when a custom-domain add names the service's own platform subdomain (400 CUSTOM_DOMAIN_OWN_PLATFORM_HOST, w4/558)",
  },
  "services.domainDeletePairSuccess": {
    message: "Se quitaron {name} y {sibling}",
    description:
      "Toast on a custom-domain delete that also removed its auto-paired sibling (w4/618)",
  },
  "services.domainDeleteSuccess": {
    message: "Se quitó {name}",
    description: "Toast on a successful custom-domain delete",
  },
  "services.domainDeleteError": {
    message: "No se pudo quitar {name}",
    description: "Toast on a failed custom-domain delete",
  },
  "services.domainPropagateNote": {
    message: "El DNS y el certificado TLS se propagan en segundo plano.",
    description:
      "Toast description after a custom-domain add (async convergence)",
  },
  "services.domainDnsToggle": {
    message: "Mostrar la configuración de DNS de {domain}",
    description:
      "aria-label for the per-domain DNS-instructions disclosure toggle",
  },
  "services.domainDnsTitle": {
    message: "Configuración de DNS",
    description: "Heading of the per-domain DNS-instructions panel",
  },
  "services.domainDnsSubdomainGuidance": {
    message:
      "Crea este registro en la zona {zone} de tu proveedor de DNS. El host es relativo a esa zona. La verificación y el certificado TLS se hacen automáticamente cuando resuelva.",
    description:
      "Guidance line above the DNS record for a subdomain — names the zone Host is relative to (w4/092)",
  },
  "services.domainDnsWildcardChallenge": {
    message:
      "Un dominio comodín también necesita este registro. Permite a bex emitir y renovar el certificado TLS del dominio.",
    description:
      "Above a wildcard custom domain's second DNS record, its _acme-challenge CNAME (w4/m326; Render's docs say it enables certificate issuance and renewal)",
  },
  "services.domainDnsApexFallback": {
    message:
      "¿Sin ALIAS/ANAME ni aplanamiento de CNAME? Crea en su lugar un registro A para @ con una de las direcciones de borde actuales de abajo. Estas direcciones pueden cambiar; un registro ALIAS/ANAME sigue funcionando cuando lo hacen.",
    description:
      "A-record fallback under an apex domain's DNS instructions, shown with the resolved edge addresses (w4/378)",
  },
  "services.domainDnsCaaHint": {
    message:
      "Si tu dominio tiene registros CAA, añade registros para las autoridades de certificación de bex: Let's Encrypt (letsencrypt.org) y Google Trust Services (pki.goog). Un dominio comodín también necesita un registro issuewild para cada una.",
    description:
      "Under every custom domain's DNS records: the CAA values bex's two certificate authorities need, as Render's custom-domains page names its own (w21/m618; the bex-domains CLI skill says the same)",
  },
  "services.domainDnsApexGuidance": {
    message:
      "Los dominios raíz no pueden usar un CNAME simple: crea este registro ALIAS/ANAME (o de aplanamiento de CNAME) en la zona {zone} (@ significa la raíz de la zona). Si tu proveedor no admite ninguno, crea un registro A para @ con las direcciones IP a las que resuelve el destino. La verificación y TLS se hacen automáticamente cuando resuelva, y bex configura por ti la redirección www ↔ dominio raíz.",
    description:
      "Guidance line above the DNS record for an apex domain — names the zone Host is relative to (w4/092)",
  },
  "services.domainRecordType": {
    message: "Tipo",
    description: "Label for the DNS record type field (CNAME/ALIAS)",
  },
  "services.domainRecordHost": {
    message: "Host",
    description: "Label for the DNS record host/name field",
  },
  "services.domainRecordTarget": {
    message: "Destino",
    description: "Label for the DNS record target/value field",
  },
  "services.domainCopyHost": {
    message: "Copiar el host {type} de {domain}",
    description: "Accessible label for copying a DNS host field",
  },
  "services.domainCopyTarget": {
    message: "Copiar el destino {type} de {domain}",
    description: "Accessible label for copying a DNS target/value field",
  },
  "services.domainDnsUnavailable": {
    message:
      "El destino DNS aún no está disponible; vuelve a comprobarlo cuando el servicio esté en ejecución.",
    description: "Shown when the backend couldn't derive the DNS record target",
  },
  "services.domainRecheck": {
    message: "Volver a comprobar",
    description: "Button that re-checks a domain's DNS/certificate status",
  },
  "services.domainRecheckAria": {
    message: "Volver a comprobar {domain}",
    description:
      "Accessible name of a custom domain's DNS-panel Re-check button, naming the domain (w4/540)",
  },
  "services.domainCopied": {
    message: "Copiado al portapapeles",
    description: "Toast when a DNS record value is copied",
  },
  "services.domainCopyError": {
    message: "No se pudo copiar al portapapeles",
    description: "Toast when copying a DNS record value fails",
  },
  "services.domainAddedTitle": {
    message: "Dominio añadido: configura el DNS",
    description: "Title of the post-add DNS-record step in the add dialog",
  },
  "services.domainAddedDescription": {
    message:
      "Crea el registro DNS en tu proveedor; la verificación se completa automáticamente cuando resuelva.",
    description: "Subtitle of the post-add DNS-record step in the add dialog",
  },
  "services.domainRedirectsTo": {
    message: "Redirige a {canonical}",
    description:
      "Note under an auto-paired sibling domain showing its canonical redirect target",
  },
  "services.domainRedirectsHere": {
    message: "{sibling} redirige aquí",
    description:
      "Note under a canonical domain showing which auto-paired sibling redirects to it",
  },
  "services.domainPairedDnsTitle": {
    message:
      "{sibling} se añadió automáticamente y redirige a {canonical}: configura también su DNS",
    description:
      "Heading of the second DNS-record block in the add dialog when the add auto-paired a www<->apex sibling (w6/m23)",
  },
  "services.domainDone": {
    message: "Listo",
    description: "Button closing the post-add DNS-record step",
  },
  "services.domainVerifySuccess": {
    message: "{name} verificado",
    description: "Toast when a re-check finds the domain verified",
  },
  "services.domainVerifyPending": {
    message:
      "{name} aún no está verificado; puede que el DNS todavía se esté propagando.",
    description: "Toast when a re-check finds the domain still pending",
  },
  "services.domainVerifyError": {
    message: "No se pudo volver a comprobar {name}.",
    description: "Toast when the re-check request fails",
  },
  "services.domainsEmptyTitle": {
    message: "Sin dominios personalizados",
    description: "Custom-domains empty-state title",
  },
  "services.domainsEmptyBody": {
    message:
      "Añade un dominio de tu propiedad para servir este servicio desde él.",
    description: "Custom-domains empty-state body",
  },
  "services.domainsErrorTitle": {
    message: "No se pudieron cargar los dominios personalizados",
    description: "Custom-domains generic error title",
  },
  "services.domainsErrorBody": {
    message:
      "Falló la solicitud a bex-api. Comprueba tu conexión y vuelve a intentarlo.",
    description: "Custom-domains generic error body",
  },
  "services.platformSubdomainTitle": {
    message: "Subdominio de la plataforma",
    description: "Settings tab platform-subdomain section title",
  },
  "services.platformSubdomainDescription": {
    message:
      "Controla si tu servicio responde en su subdominio de la plataforma bex además de en los dominios personalizados que hayas configurado.",
    description: "Settings tab platform-subdomain section description",
  },
  "services.platformSubdomainEnabled": {
    message: "Activado",
    description: "Platform-subdomain toggle label when the subdomain is active",
  },
  "services.platformSubdomainDisabled": {
    message: "Desactivado",
    description:
      "Platform-subdomain toggle label when the subdomain is disabled",
  },
  "services.platformSubdomainPending": {
    message:
      "La URL de la plataforma se asigna cuando el servicio está en ejecución.",
    description: "Platform-subdomain state when the service has no URL yet",
  },
  "services.platformSubdomainDisabledNote": {
    message:
      "El subdominio de la plataforma está desactivado. Tu servicio solo es accesible mediante dominios personalizados.",
    description:
      "Platform-subdomain note shown when the policy is set to disabled",
  },
  "services.platformSubdomainToggleLabel": {
    message: "Alternar el subdominio de la plataforma",
    description: "Accessible label for the platform-subdomain Switch",
  },
  "services.platformSubdomainNeedsVerifiedDomain": {
    message:
      "Verifica un dominio personalizado antes de desactivar el subdominio de la plataforma, para que el servicio conserve una dirección.",
    description:
      "Hint beside the platform-subdomain switch when no custom domain is verified yet (w4/142)",
  },
  "services.platformSubdomainNeedsVerifiedDomainPending": {
    message:
      "{names} deben terminar de verificarse antes de poder desactivar el subdominio de la plataforma.",
    description:
      "Hint beside the platform-subdomain switch when custom domains exist but are all still pending verification (w4/142)",
  },
  "services.subdomainPolicySuccess": {
    message: "Configuración del subdominio de la plataforma actualizada.",
    description: "Toast on successful setSubdomainPolicy mutation",
  },
  "services.subdomainPolicyError": {
    message:
      "No se pudo actualizar la configuración del subdominio de la plataforma. Inténtalo de nuevo.",
    description: "Toast on failed setSubdomainPolicy mutation",
  },
  "services.maintenanceModeTitle": {
    message: "Modo de mantenimiento",
    description: "Settings tab maintenance-mode section title",
  },
  "services.maintenanceModeDescription": {
    message:
      "Deja este servicio fuera de línea tras una página de mantenimiento sin suspenderlo: sus pods siguen en ejecución y solo se redirige el tráfico público.",
    description: "Settings tab maintenance-mode section description",
  },
  "services.maintenanceModeEnabled": {
    message: "Activado",
    description: "Maintenance-mode toggle label when active",
  },
  "services.maintenanceModeDisabled": {
    message: "Desactivado",
    description: "Maintenance-mode toggle label when inactive",
  },
  "services.maintenanceModeSwitchHint": {
    message:
      "Todos los hosts que sirve este servicio responderán con la página de mantenimiento.",
    description: "Maintenance-mode section hint text next to the switch",
  },
  "services.maintenanceModePaidOnly": {
    message:
      "El modo de mantenimiento está disponible en los planes de pago de Web Service.",
    description: "Maintenance mode free-plan eligibility note",
  },
  "services.maintenanceModeToggleLabel": {
    message: "Alternar el modo de mantenimiento",
    description: "Accessible label for the maintenance-mode Switch",
  },
  "services.maintenanceModeUriLabel": {
    message: "URL de la página de mantenimiento personalizada",
    description: "Label for the maintenance-mode custom-page URL field",
  },
  "services.maintenanceModeUriPlaceholder": {
    message: "https://status.example.com/maintenance (opcional)",
    description: "Placeholder for the maintenance-mode custom-page URL field",
  },
  "services.maintenanceModeUriHint": {
    message:
      "Se obtiene y se sirve en lugar de la página predeterminada. Déjalo vacío para usar la página de mantenimiento predeterminada de bex. No debe apuntar a la propia URL de este servicio.",
    description: "Hint text under the maintenance-mode custom-page URL field",
  },
  "services.maintenanceModeUriEdit": {
    message: "Editar URL de la página de mantenimiento",
    description:
      "Accessible label for the maintenance-page URL edit-pencil button",
  },
  "services.maintenanceModeEnableAction": {
    message: "Activar el modo de mantenimiento",
    description: "Confirm-dialog action button for enabling maintenance mode",
  },
  "services.confirmMaintenanceModeTitle": {
    message: "¿Activar el modo de mantenimiento?",
    description: "Confirm-dialog title for enabling maintenance mode",
  },
  "services.confirmMaintenanceModeBody": {
    message:
      "{name} mostrará una página de mantenimiento a todos los visitantes hasta que lo desactives. Los pods del servicio siguen en ejecución.",
    description: "Confirm-dialog body for enabling maintenance mode",
  },
  "services.maintenanceModeEnabledSuccess": {
    message: "Modo de mantenimiento activado.",
    description: "Toast on successfully enabling maintenance mode",
  },
  "services.maintenanceModeDisabledSuccess": {
    message: "Modo de mantenimiento desactivado.",
    description: "Toast on successfully disabling maintenance mode",
  },
  "services.maintenancePageSavedSuccess": {
    message: "Página de mantenimiento guardada.",
    description: "Toast on saving only the custom maintenance page URL",
  },
  "services.maintenancePageClearedSuccess": {
    message: "Página de mantenimiento predeterminada restaurada.",
    description: "Toast on clearing the custom maintenance page URL",
  },
  "services.maintenanceModeError": {
    message:
      "No se pudo actualizar el modo de mantenimiento. Inténtalo de nuevo.",
    description: "Toast on failed setMaintenanceMode mutation",
  },
  "services.edgeCachingTitle": {
    message: "Caché de borde",
    description: "Settings tab edge-caching section title (Render parity)",
  },
  "services.edgeCachingDescription": {
    message:
      "Almacena las respuestas en caché en el borde para que las solicitudes repetidas se sirvan sin llegar a este servicio.",
    description: "Settings tab edge-caching section description",
  },
  "services.edgeCachingFileTypesLabel": {
    message: "Tipos de archivo almacenables en caché",
    description: "Edge-caching radio group label",
  },
  "services.edgeCachingOptionNone": {
    message: "Ninguno",
    description: "Edge-caching option: no-cache profile",
  },
  "services.edgeCachingOptionNoneDesc": {
    message: "Sirve cada respuesta desde tu servicio.",
    description: "Edge-caching no-cache option description",
  },
  "services.edgeCachingOptionStatic": {
    message: "Archivos estáticos comunes",
    description: "Edge-caching option: origin-controlled profile",
  },
  "services.edgeCachingOptionStaticDesc": {
    message:
      "Almacena en caché los tipos de archivo estático comunes, como CSS, JavaScript, imágenes y fuentes, salvo que el encabezado Cache-Control de la respuesta indique otra cosa.",
    description: "Edge-caching origin-controlled option description",
  },
  "services.edgeCachingOptionAll": {
    message: "Todos los archivos",
    description: "Edge-caching option: origin-controlled-all profile",
  },
  "services.edgeCachingOptionAllDesc": {
    message:
      "Almacena en caché todas las respuestas aptas, salvo que su encabezado Cache-Control indique otra cosa. Asegúrate de que el contenido dinámico defina Cache-Control para que no se almacene en caché.",
    description:
      "Edge-caching origin-controlled-all option description, with the dynamic-content warning",
  },
  "services.edgeCachingPurgeOnDeploy": {
    message: "La caché se purga en cada despliegue exitoso.",
    description: "Note under the edge-caching options",
  },
  "services.edgeCachingPaidOnly": {
    message:
      "La caché de borde no está disponible para los Web Services gratuitos.",
    description: "Edge-caching free-plan eligibility note",
  },
  "services.edgeCachingSave": {
    message: "Guardar caché de borde",
    description: "Edge-caching save button",
  },
  "services.edgeCachingSaving": {
    message: "Guardando…",
    description: "Edge-caching save button while the mutation is in flight",
  },
  "services.edgeCachingSaved": {
    message: "Caché de borde actualizada.",
    description: "Toast on successful setCacheProfile mutation",
  },
  "services.edgeCachingError": {
    message: "No se pudo actualizar la caché de borde. Inténtalo de nuevo.",
    description: "Toast on failed setCacheProfile mutation",
  },
  "services.edgeCachingPurgeAction": {
    message: "Purgar caché",
    description: "Edge-caching purge button and confirm action",
  },
  "services.edgeCachingPurgeConfirmTitle": {
    message: "¿Purgar la caché de borde?",
    description: "Confirm-dialog title for purging the edge cache",
  },
  "services.edgeCachingPurgeConfirmBody": {
    message:
      "Se descartan todas las respuestas en caché. Hasta que la caché se vuelva a calentar, las solicitudes llegan a tu servicio.",
    description: "Confirm-dialog body for purging the edge cache",
  },
  "services.edgeCachingPurged": {
    message: "Purga de caché iniciada.",
    description: "Toast on successful purgeServiceCache mutation",
  },
  "services.edgeCachingPurgeError": {
    message: "No se pudo purgar la caché. Inténtalo de nuevo.",
    description: "Toast on failed purgeServiceCache mutation",
  },
  "services.runtimeTitle": {
    message: "Runtime",
    description: "Settings tab runtime section title (Render parity)",
  },
  "services.runtimeDescription": {
    message: "El entorno en el que bex compila y ejecuta este servicio.",
    description: "Settings tab runtime section description",
  },
  "services.runtimeLabel": {
    message: "Runtime",
    description: "Settings runtime select label",
  },
  "services.runtimeNote": {
    message:
      "Cambiar el runtime recompila y vuelve a desplegar el servicio. Los runtimes nativos usan el comando de build y el comando de inicio de abajo.",
    description: "Note beside the Settings runtime select",
  },
  "services.runtimePlaceholder": {
    message: "Selecciona un runtime",
    description:
      "Settings runtime select placeholder for a legacy service with no runtime recorded",
  },
  "services.runtimeSave": {
    message: "Guardar runtime",
    description: "Settings runtime save button",
  },
  "services.runtimeSaving": {
    message: "Guardando…",
    description: "Settings runtime save button while the mutation is in flight",
  },
  "services.runtimeConfirmTitle": {
    message: "¿Cambiar el runtime a {runtime}?",
    description: "Confirm-dialog title for changing a service's runtime",
  },
  "services.runtimeConfirmBody": {
    message:
      "El servicio se recompila con el runtime {runtime} y se vuelve a desplegar. Esto inicia un nuevo despliegue.",
    description: "Confirm-dialog body for changing a service's runtime",
  },
  "services.runtimeConfirmAction": {
    message: "Cambiar runtime",
    description: "Confirm-dialog action for changing a service's runtime",
  },
  "services.runtimeSaved": {
    message: "Runtime actualizado. Se inició un nuevo despliegue.",
    description: "Toast on successful setRuntime mutation",
  },
  "services.runtimeError": {
    message: "No se pudo cambiar el runtime. Inténtalo de nuevo.",
    description: "Toast on failed setRuntime mutation without a server message",
  },
  "services.maintenanceModeBannerTitle": {
    message: "El modo de mantenimiento está activado",
    description: "Service-detail header banner title while in maintenance",
  },
  "services.maintenanceModeBannerBody": {
    message:
      "Los visitantes ven una página de mantenimiento en lugar de este servicio. Los pods siguen en ejecución: desactiva el modo de mantenimiento en Configuración para reanudar el servicio normal.",
    description: "Service-detail header banner body while in maintenance",
  },
  "services.deployTitle": {
    message: "Despliegue",
    description: "Cron job Settings tab: Deploy section title (Render parity)",
  },
  "services.deployDescription": {
    message: "Cómo se ejecuta este Cron Job.",
    description: "Cron job Settings tab: Deploy section description",
  },
  "services.deploySuccess": {
    message: "Configuración del Cron Job guardada.",
    description: "Toast after updateCronJob succeeds",
  },
  "services.cronScheduleSaved": {
    message: "La próxima ejecución sigue la nueva programación.",
    description:
      "Toast description after saving a cron job's schedule alone: the next run follows it, nothing deploys (w4/593)",
  },
  "services.cronCommandSaved": {
    message:
      "Un despliegue está aplicando el nuevo comando; las ejecuciones posteriores lo usan.",
    description:
      "Toast description after saving a cron job's command: the save rolls a deploy that applies it (w4/593)",
  },
  "services.deployError": {
    message:
      "No se pudo guardar la configuración del Cron Job. Inténtalo de nuevo.",
    description: "Toast after updateCronJob fails",
  },
  "services.deployRefreshFailed": {
    message:
      "Configuración del Cron Job guardada, pero esta página no pudo actualizarse. Inténtalo de nuevo o recarga para ver la configuración guardada.",
    description:
      "Warning after a cron save committed but its authoritative re-read failed; the save must not be retried or reported as failed (w4/276)",
  },
  "services.deployRefreshPending": {
    message:
      "Configuración guardada: esperando una lectura reciente antes de más ediciones.",
    description:
      "Reason shown while cron rows stay unwritable after a committed save whose refresh failed (w4/276)",
  },
  "services.deployScheduleLabel": {
    message: "Programación",
    description: "Cron job Settings tab: Deploy section schedule field label",
  },
  "services.deployScheduleHint": {
    message:
      "Ejecuta este comando según esta programación (crontab de 5 campos, en UTC).",
    description: "Cron job Settings tab: Deploy section schedule help text",
  },
  "services.deploySchedulePreview": {
    message: "{description} · se ejecuta en UTC",
    description:
      "Cron job Settings tab: live human-readable description of the schedule",
  },
  "services.deploySchedulePlaceholder": {
    message: "0 * * * *",
    description: "Cron job Deploy section: schedule input placeholder",
  },
  "services.deployScheduleEdit": {
    message: "Editar programación",
    description: "Accessible label for the cron schedule edit button",
  },
  "services.deployCommandEdit": {
    message: "Editar comando",
    description: "Accessible label for the cron command edit button",
  },
  "services.cronScheduleNeverFires": {
    message:
      "Esta programación nunca se activa: ninguna fecha coincide con su día del mes y mes (p. ej., 31 2 es el 31 de febrero).",
    description:
      "Cron schedule validation error when the schedule parses but no date ever matches (w4/197)",
  },
  "services.deployScheduleError": {
    message:
      "Introduce una expresión cron válida de 5 campos, p. ej. 0 * * * *.",
    description: "Cron job Deploy section: schedule validation error",
  },
  "services.deployScheduleRequired": {
    message: "La programación es obligatoria.",
    description: "Cron job Deploy section: schedule required validation error",
  },
  "services.deployCommandLabel": {
    message: "Comando",
    description: "Cron job Settings tab: Deploy section command field label",
  },
  "services.deployCommandPlaceholder": {
    message: "p. ej. python script.py",
    description: "Cron job Deploy section: command input placeholder",
  },
  "services.deployCommandHint": {
    message:
      "Anula el punto de entrada predeterminado de la imagen. Déjalo en blanco para ejecutar el comando propio de la imagen.",
    description: "Cron job Deploy section: command field help text",
  },
  "services.deployCommandNativeHint": {
    message:
      "El comando que ejecuta la tarea en cada programación, como comando de inicio de su runtime.",
    description:
      "Cron job Deploy section: command field help text for a native-runtime cron, whose command is its start command (w8/m69)",
  },
  "services.deployCommandRequired": {
    message: "El comando es obligatorio.",
    description:
      "Cron job Deploy section: command required validation error for a native-runtime cron (w8/m69)",
  },
  "services.buildTitle": {
    message: "Build",
    description:
      "Settings tab: Build card title, Render's split Build section (w5/m52)",
  },
  "services.buildDeployNav": {
    message: "Build y despliegue",
    description:
      "Settings section index: the one entry for the Build and Deploy cards, which the page renders as one section (Render names it Build & Deploy)",
  },
  "services.buildDescription": {
    message:
      "Configura cómo bex compila tu servicio a partir de tu repositorio.",
    description: "Settings tab: Build card description",
  },
  "services.deploySectionTitle": {
    message: "Despliegue",
    description:
      "Settings tab: Deploy card title, Render's split Deploy section (w5/m52)",
  },
  "services.deploySectionDescription": {
    message: "Configura cómo bex despliega y ejecuta tu servicio.",
    description: "Settings tab: Deploy card description",
  },
  "services.buildDeploySourceLabel": {
    message: "Origen",
    description: "Build & Deploy: repo field label",
  },
  "services.buildDeploySourceHint": {
    message:
      "El repositorio Git a partir del cual se compila este servicio. Elige uno de tus repositorios conectados o pega la URL de un repositorio.",
    description: "Build & Deploy: Source (repo) field help text (w5/m54)",
  },
  "services.buildDeploySourcePlaceholder": {
    message: "https://github.com/owner/repo",
    description: "Build & Deploy: Source combobox input placeholder",
  },
  "services.buildDeploySourceEdit": {
    message: "Editar origen",
    description: "Accessible label for the Source edit-pencil button",
  },
  "services.buildDeploySourceConfirmTitle": {
    message: "¿Cambiar el origen a {value}?",
    description: "Build & Deploy: Source-change confirm dialog title",
  },
  "services.buildDeployBranchLabel": {
    message: "Rama",
    description: "Build & Deploy: branch field label",
  },
  "services.buildDeployBranchHint": {
    message: "La rama de Git que se compila y despliega.",
    description: "Build & Deploy: branch field hint (Render's copy)",
  },
  "services.buildDeployBranchEmpty": {
    message: "main",
    description:
      "Build & Deploy: branch value shown when unset (the backend default)",
  },
  "services.buildDeployBranchPlaceholder": {
    message: "main",
    description: "Build & Deploy: branch input placeholder",
  },
  "services.buildDeployBranchEdit": {
    message: "Editar rama",
    description: "Build & Deploy: branch edit-button aria label",
  },
  "services.buildDeployBranchConfirmTitle": {
    message: "¿Cambiar la rama a “{value}”?",
    description: "Build & Deploy: branch confirm-dialog title",
  },
  "services.buildDeployRootDirLabel": {
    message: "Directorio raíz",
    description: "Build & Deploy: root-directory field label",
  },
  "services.buildDeployRootDirOptional": {
    message: "Opcional",
    description:
      "Build & Deploy: badge next to the Root Directory label (Render parity)",
  },
  "services.buildDeployRootDirHint": {
    message:
      "Si se define, los builds se ejecutan desde este subdirectorio en lugar de la raíz del repositorio. Los cambios de código fuera de él no activan un despliegue automático, salvo que coincidan con una ruta incluida de los filtros de build. Se usa sobre todo con un monorepo.",
    description: "Build & Deploy: root-directory field help text",
  },
  "services.buildDeployConfirmRoot": {
    message: "la raíz del repositorio",
    description:
      "Build & Deploy: mid-sentence phrase for the confirm dialog title when clearing rootDir to empty (a dedicated key, not a lowercased buildDeployRootDirEmpty, since that transform doesn't hold in every language)",
  },
  "services.buildDeployRootDirPlaceholder": {
    message: "p. ej. backend",
    description: "Build & Deploy: root-directory input placeholder",
  },
  "services.buildDeployEdit": {
    message: "Editar directorio raíz",
    description: "Build & Deploy: accessible label for the edit-pencil button",
  },
  "services.buildDeployCancel": {
    message: "Cancelar",
    description: "Build & Deploy: root-directory inline-edit cancel button",
  },
  "services.buildDeployConfirmTitle": {
    message: "¿Cambiar el directorio raíz a {value}?",
    description: "Build & Deploy: root-directory change confirm dialog title",
  },
  "services.buildDeployConfirmBody": {
    message:
      "El servicio se recompila y se vuelve a desplegar, limitado al nuevo directorio: las solicitudes en curso terminan antes de reemplazar las instancias antiguas.",
    description: "Build & Deploy: root-directory change confirm dialog body",
  },
  "services.buildDeployConfirmBodyStatic": {
    message:
      "Esto inicia una recompilación desde el nuevo directorio. Tu sitio sigue sirviendo su versión publicada actual hasta que el nuevo build se publique correctamente.",
    description:
      "Build & Deploy: a live static site's root-directory change confirm body (atomic publish, no instances; w4/548)",
  },
  "services.buildDeployConfirmBodyNeverLive": {
    message:
      "El servicio se compila y despliega desde el nuevo directorio. Aún no sirve nada: pasa a estar activo cuando un despliegue tenga éxito.",
    description:
      "Build & Deploy: root-directory change confirm body for a service that has never gone live, so no current version keeps serving (w4/548)",
  },
  "services.buildDeploySuccess": {
    message: "Directorio raíz actualizado.",
    description: "Toast after setRootDir succeeds",
  },
  "services.buildDeployError": {
    message: "No se pudo actualizar el directorio raíz. Inténtalo de nuevo.",
    description: "Toast after setRootDir fails",
  },
  "services.sourceUpdateSuccess": {
    message: "Origen actualizado.",
    description:
      "Toast after a repository, branch, or image source update saves without triggering a deploy (e.g. suspended service or unbuilt build source)",
  },
  "services.sourceUpdateDeploying": {
    message: "Origen actualizado. Iniciando un nuevo despliegue.",
    description:
      "Toast after Update Source saves and immediately triggers a new deploy",
  },
  "services.sourceUpdateError": {
    message: "No se pudo actualizar el origen. Inténtalo de nuevo.",
    description:
      "Fallback toast when a source update fails without a server reason",
  },
  "services.sourceTitle": {
    message: "Origen",
    description: "Source card title",
  },
  "services.sourceRepoDescription": {
    message:
      "El repositorio Git y la rama a partir de los cuales se compila este servicio.",
    description: "Source card description for a repository-backed service",
  },
  "services.sourceImageDescription": {
    message: "La imagen de contenedor prediseñada que despliega este servicio.",
    description: "Source card description for an image-backed service",
  },
  "services.sourceImageLabel": {
    message: "Imagen",
    description: "Source card: container image path field label",
  },
  "services.sourceDialogDescription": {
    message:
      "Selecciona un repositorio Git, un origen de build existente o una imagen Docker. Al enviar se inicia un nuevo despliegue.",
    description:
      "Update Source dialog description for non-static services (Render parity)",
  },
  "services.sourceDialogDescriptionStatic": {
    message:
      "Selecciona un repositorio Git para este Static Site. Al enviar se inicia un nuevo despliegue.",
    description:
      "Update Source dialog description for static sites (Render parity)",
  },
  "services.sourceMissing": {
    message: "No configurado",
    description: "Source card fallback when no source value is available",
  },
  "services.sourceRepoAccessLostTitle": {
    message: "La app de GitHub de bex no puede leer este repositorio",
    description:
      "Settings → Source warning title: the GitHub App connected this service's repository and can no longer read it (repoAccessLost, w10/m490)",
  },
  "services.sourceRepoAccessLostBody": {
    message:
      "Los pushes ya no despliegan este servicio. Si el repositorio se movió a otra cuenta de GitHub, instala la app de GitHub en esa cuenta; si no, vuelve a dar a la app acceso al repositorio.",
    description:
      "Settings → Source warning body: pushes stop reaching bex once the GitHub App loses the repository (moved to an account without the app, or the grant was revoked); what to do (w10/m490)",
  },
  "services.sourceEdit": {
    message: "Editar",
    description: "Button opening the Update Source dialog",
  },
  "services.sourceDialogTitle": {
    message: "Actualizar origen",
    description: "Update Source dialog title",
  },
  "services.sourcePickerLabel": {
    message: "Elige un origen",
    description: "Label above the source kind picker in Update Source",
  },
  "services.sourceBranchHint": {
    message: "Elige una rama o introduce una que no aparezca en la lista.",
    description: "Branch picker free-text fallback hint",
  },
  "services.sourceBranchEmpty": {
    message: "Ninguna rama coincide. Introduce el nombre de una rama.",
    description: "Empty state for the Update Source branch picker",
  },
  "services.sourceTabBuildSource": {
    message: "Origen de build",
    description: "Update Source tab: build the service from a build source",
  },
  "services.sourceBuildSourceDescription": {
    message:
      "El origen de build que despliega este servicio. Su build se comparte con todos los servicios vinculados a él.",
    description:
      "Source card description for a service linked to a build source",
  },
  "services.sourceBuildSourceLabel": {
    message: "Origen de build",
    description: "Source card and Update Source: build source field label",
  },
  "services.sourceBuildSourceBuildsFrom": {
    message: "Se compila desde",
    description:
      "Source card: the repository or image the linked build source builds from",
  },
  "services.sourceBuildSourceEmpty": {
    message:
      "Este espacio de trabajo aún no tiene orígenes de build. Crea uno con la API o MCP y luego vincula aquí los servicios a él.",
    description: "Update Source build source tab: empty state",
  },
  "services.sourceBuildSourceLoadError": {
    message:
      "No se pudieron cargar los orígenes de build de este espacio de trabajo. Cierra el diálogo e inténtalo de nuevo.",
    description: "Update Source build source tab: the list failed to load",
  },
  "services.sourceBuildSourcePlaceholder": {
    message: "Selecciona un origen de build",
    description: "Update Source build source picker placeholder",
  },
  "services.sourceBuildSourceStartHint": {
    message:
      "Este origen de build usa un runtime nativo, por lo que el servicio necesita su propio comando de inicio.",
    description:
      "Update Source: why a native-runtime build source needs a start command",
  },
  "services.sourceBuildCommandOptionalHint": {
    message:
      "Opcional. Déjalo vacío para publicar directamente los archivos del repositorio.",
    description:
      "Update Source static-site Build Command helper: build command is optional",
  },
  "services.sourcePublishDirectoryRequired": {
    message: "Introduce un directorio de publicación.",
    description:
      "Update Source static-site Publish Directory validation error when blank",
  },
  "services.sourceDeploy": {
    message: "Desplegar",
    description:
      "Primary submit button in Update Source for Git and Build Source tabs",
  },
  "services.sourceDeploying": {
    message: "Desplegando",
    description:
      "In-flight submit button label in Update Source for Git and Build Source tabs",
  },
  "services.sourceConnect": {
    message: "Conectar",
    description:
      "Primary submit button in Update Source for the Existing Image tab",
  },
  "services.sourceConnecting": {
    message: "Conectando",
    description:
      "In-flight submit button label in Update Source for the Existing Image tab",
  },
  "services.startCommandLabel": {
    message: "Comando de inicio",
    description: "Build & Deploy: native-runtime start-command label",
  },
  "services.dockerCommandLabel": {
    message: "Comando de Docker",
    description: "Build & Deploy: Docker CMD override label (Render wording)",
  },
  "services.startCommandHint": {
    message: "El comando que inicia este servicio tras un build correcto.",
    description: "Build & Deploy: native start-command help text",
  },
  "services.dockerCommandHint": {
    message:
      "Anula el CMD del Dockerfile. Déjalo en blanco para usar el comando predeterminado de la imagen.",
    description: "Build & Deploy: Docker Command help text",
  },
  "services.startCommandConfirmEmpty": {
    message: "el comando predeterminado",
    description: "Build & Deploy: empty command phrase in confirmation title",
  },
  "services.startCommandPlaceholder": {
    message: "p. ej. npm start",
    description: "Build & Deploy: start-command input placeholder",
  },
  "services.startCommandEdit": {
    message: "Editar comando de inicio",
    description: "Build & Deploy: accessible command edit button label",
  },
  "services.dockerCommandEdit": {
    message: "Editar comando de Docker",
    description: "Build & Deploy: accessible Docker Command edit button label",
  },
  "services.dockerCommandPlaceholder": {
    message: "p. ej. bin/server",
    description: "Build & Deploy: Docker Command input placeholder",
  },
  "services.startCommandConfirmTitle": {
    message: "¿Cambiar el comando de inicio a {value}?",
    description: "Build & Deploy: command-change confirmation title",
  },
  "services.dockerCommandConfirmTitle": {
    message: "¿Cambiar el comando de Docker a {value}?",
    description: "Build & Deploy: Docker Command confirmation title",
  },
  "services.startCommandConfirmBody": {
    message:
      "El servicio se vuelve a desplegar con el nuevo comando. Las solicitudes en curso terminan antes de reemplazar las instancias antiguas.",
    description: "Build & Deploy: command-change confirmation body",
  },
  "services.startCommandConfirmBodyNeverLive": {
    message:
      "El servicio se despliega con el nuevo comando. Aún no sirve nada: pasa a estar activo cuando un despliegue tenga éxito.",
    description:
      "Deploy: command change confirm body for a service that has never gone live (w4/548)",
  },
  "services.startCommandSuccess": {
    message: "Comando de inicio actualizado.",
    description: "Toast after setStartCommand succeeds",
  },
  "services.startCommandError": {
    message: "No se pudo actualizar el comando de inicio. Inténtalo de nuevo.",
    description: "Toast after setStartCommand fails",
  },
  "services.dockerCommandSuccess": {
    message: "Comando de Docker actualizado.",
    description:
      "Toast after a Dockerfile or Existing Image service's Docker Command is saved",
  },
  "services.dockerCommandError": {
    message: "No se pudo actualizar el comando de Docker. Inténtalo de nuevo.",
    description:
      "Toast when saving a Dockerfile or Existing Image service's Docker Command fails",
  },
  "services.buildCommandLabel": {
    message: "Comando de build",
    description:
      "Build & Deploy: build-command field label (all service types' settings + the New Service form)",
  },
  "services.buildCommandHint": {
    message:
      "El comando que compila tu servicio (p. ej., npm run build). Déjalo en blanco para usar el valor predeterminado del runtime.",
    description:
      "Build & Deploy: build-command help text — shown for every service type, so it must not assume static-site output",
  },
  "services.buildCommandConfirmEmpty": {
    message: "Valor predeterminado del runtime",
    description:
      "Build & Deploy: empty build-command phrase in confirmation title",
  },
  "services.buildCommandPlaceholder": {
    message: "npm run build",
    description: "Build & Deploy: build-command input placeholder",
  },
  "services.buildCommandEdit": {
    message: "Editar comando de build",
    description: "Build & Deploy: accessible build-command edit button label",
  },
  "services.buildCommandConfirmTitle": {
    message: "¿Cambiar el comando de build a {value}?",
    description: "Build & Deploy: build-command change confirmation title",
  },
  "services.buildCommandConfirmBody": {
    message:
      "El servicio se volverá a desplegar con el nuevo comando de build. Las solicitudes existentes se drenarán antes de reemplazar las instancias antiguas.",
    description: "Build & Deploy: build-command change confirmation body",
  },
  "services.buildCommandConfirmBodyNeverLive": {
    message:
      "El servicio se compila y despliega con el nuevo comando de build. Aún no sirve nada: pasa a estar activo cuando un despliegue tenga éxito.",
    description:
      "Build & Deploy: build-command change confirm body for a service that has never gone live (w4/548)",
  },
  "services.buildCommandConfirmBodyStatic": {
    message:
      "Esto inicia una recompilación con el nuevo comando de build. Tu sitio sigue sirviendo su versión publicada actual hasta que el nuevo build se publique correctamente.",
    description:
      "Build & Deploy: static-site build-command change confirmation body (atomic publish, no instances)",
  },
  "services.staticBuildConfirmBodyNeverLive": {
    message:
      "Esto inicia un build con el cambio. Tu sitio aún no está publicado: pasa a estar activo cuando este build tenga éxito.",
    description:
      "Build & Deploy: a static site's build-command or root-directory change confirm body when the site has never been published (w4/548)",
  },
  "services.buildCommandSuccess": {
    message: "Comando de build actualizado.",
    description: "Toast after setBuildCommand succeeds",
  },
  "services.buildCommandError": {
    message: "No se pudo actualizar el comando de build. Inténtalo de nuevo.",
    description: "Toast after setBuildCommand fails",
  },
  "services.dockerfilePathLabel": {
    message: "Ruta del Dockerfile",
    description: "Build & Deploy: Dockerfile-path field label",
  },
  "services.dockerfilePathHint": {
    message:
      "Ruta al Dockerfile, relativa al directorio raíz. Déjala en blanco para usar Dockerfile.",
    description: "Build & Deploy: Dockerfile-path help text",
  },
  "services.dockerfilePathConfirmEmpty": {
    message: "el Dockerfile predeterminado",
    description: "Build & Deploy: empty Dockerfile-path confirmation phrase",
  },
  "services.dockerfilePathPlaceholder": {
    message: "p. ej. docker/Dockerfile.prod",
    description: "Build & Deploy: Dockerfile-path input placeholder",
  },
  "services.dockerfilePathEdit": {
    message: "Editar ruta del Dockerfile",
    description: "Build & Deploy: accessible Dockerfile-path edit label",
  },
  "services.dockerfilePathConfirmTitle": {
    message: "¿Cambiar la ruta del Dockerfile a {value}?",
    description: "Build & Deploy: Dockerfile-path confirmation title",
  },
  "services.dockerfilePathConfirmBody": {
    message:
      "El servicio se recompila desde el Dockerfile seleccionado y despliega la imagen resultante.",
    description: "Build & Deploy: Dockerfile-path confirmation body",
  },
  "services.dockerfilePathSuccess": {
    message: "Ruta del Dockerfile actualizada.",
    description: "Toast after setDockerfilePath succeeds",
  },
  "services.dockerfilePathError": {
    message:
      "No se pudo actualizar la ruta del Dockerfile. Inténtalo de nuevo.",
    description: "Toast after setDockerfilePath fails",
  },
  "services.dockerContextLabel": {
    message: "Directorio de contexto de build de Docker",
    description: "Build & Deploy: Docker build context field label (w8/m128)",
  },
  "services.dockerContextHint": {
    message:
      "Contexto de build, relativo al directorio raíz. Déjalo en blanco para usar el directorio raíz.",
    description: "Build & Deploy: Docker build context help text",
  },
  "services.dockerContextConfirmEmpty": {
    message: "el directorio raíz",
    description:
      "Build & Deploy: empty Docker build context confirmation phrase",
  },
  "services.dockerContextPlaceholder": {
    message: "p. ej. .",
    description: "Build & Deploy: Docker build context input placeholder",
  },
  "services.dockerContextEdit": {
    message: "Editar directorio de contexto de build de Docker",
    description: "Build & Deploy: accessible Docker build context edit label",
  },
  "services.dockerContextConfirmTitle": {
    message: "¿Cambiar el directorio de contexto de build de Docker a {value}?",
    description: "Build & Deploy: Docker build context confirmation title",
  },
  "services.dockerContextConfirmBody": {
    message:
      "El servicio se recompila con el nuevo contexto de build y despliega la imagen resultante.",
    description: "Build & Deploy: Docker build context confirmation body",
  },
  "services.dockerContextSuccess": {
    message: "Directorio de contexto de build de Docker actualizado.",
    description: "Toast after setDockerContext succeeds",
  },
  "services.dockerContextError": {
    message:
      "No se pudo actualizar el directorio de contexto de build de Docker. Inténtalo de nuevo.",
    description: "Toast after setDockerContext fails",
  },
  "services.buildFilterLabel": {
    message: "Filtros de build",
    description: "Build & Deploy: label for the build-filters editor",
  },
  "services.buildFilterHint": {
    message:
      "Activa un despliegue solo cuando un push de git cambie archivos coincidentes. Las rutas son patrones glob relativos a la raíz del repositorio, aunque haya un directorio raíz definido (p. ej. src/**, **/*.md).",
    description: "Build & Deploy: help text for the build-filters editor",
  },
  "services.buildFilterIncludedTitle": {
    message: "Rutas incluidas",
    description: "Build & Deploy: title for the included-paths list",
  },
  "services.buildFilterIncludedHint": {
    message:
      "Un push despliega solo si un archivo modificado coincide con alguna de estas, incluso fuera del directorio raíz. Vacío significa que se incluyen todas las rutas dentro del directorio raíz.",
    description: "Build & Deploy: help text for the included-paths list",
  },
  "services.buildFilterIncludedPlaceholder": {
    message: "p. ej. src/**",
    description: "Build & Deploy: placeholder for an included-path input",
  },
  "services.buildFilterAddIncluded": {
    message: "Añadir ruta incluida",
    description: "Build & Deploy: add-row button for the included-paths list",
  },
  "services.buildFilterRemoveIncluded": {
    message: "Quitar ruta incluida",
    description: "Build & Deploy: remove-row label for an included path",
  },
  "services.buildFilterIgnoredTitle": {
    message: "Rutas ignoradas",
    description: "Build & Deploy: title for the ignored-paths list",
  },
  "services.buildFilterIgnoredHint": {
    message:
      "Un archivo modificado que coincida con alguna de estas nunca activa un despliegue, aunque también coincida con una ruta incluida.",
    description: "Build & Deploy: help text for the ignored-paths list",
  },
  "services.buildFilterIgnoredPlaceholder": {
    message: "p. ej. docs/**",
    description: "Build & Deploy: placeholder for an ignored-path input",
  },
  "services.buildFilterAddIgnored": {
    message: "Añadir ruta ignorada",
    description: "Build & Deploy: add-row button for the ignored-paths list",
  },
  "services.buildFilterRemoveIgnored": {
    message: "Quitar ruta ignorada",
    description: "Build & Deploy: remove-row label for an ignored path",
  },
  "services.buildPathTooLong": {
    message: "Máximo 512 bytes",
    description:
      "Inline error under a Root Directory, Dockerfile Path, Docker Build Context or Build Filter pattern over the server's 512-byte limit; a Chinese character is 3 bytes (w21/045)",
  },
  "services.buildPathControlChar": {
    message: "No debe contener un carácter de control ni una tabulación",
    description:
      "Inline error under a build path or Build Filter pattern with a control character (w21/045)",
  },
  "services.buildPathBackslash": {
    message: "Usa / como separador de ruta, no \\",
    description:
      "Inline error under a build path or Build Filter pattern with a backslash (w21/045)",
  },
  "services.buildPathAbsolute": {
    message: "Debe ser relativa al repositorio y no empezar con /",
    description:
      "Inline error under a build path or Build Filter pattern that starts with / (w21/045)",
  },
  "services.buildPathDotDot": {
    message: "Debe permanecer dentro del repositorio: sin segmentos ..",
    description:
      "Inline error under a build path or Build Filter pattern with a .. segment (w21/045)",
  },
  "services.buildPathEmptySegment": {
    message: "No debe tener segmentos vacíos: sin // ni / al final",
    description:
      "Inline error under a Dockerfile Path or Docker Build Context with an empty path segment; the Root Directory normalizes these away (w21/045)",
  },
  "services.buildFilterUnclosedClass": {
    message: "Un [ nunca se cierra con un ]",
    description:
      "Inline error under a Build Filter glob with an unclosed character class (w21/045)",
  },
  "services.healthCheckPathSlash": {
    message:
      "Debe empezar con /, o estar vacía para una comprobación solo del puerto",
    description:
      "Inline error under a Health Check Path that does not start with / (w21/045)",
  },
  "services.buildFilterSave": {
    message: "Guardar filtros de build",
    description: "Build & Deploy: save button for the build-filters editor",
  },
  "services.buildFilterSuccess": {
    message: "Filtros de build actualizados.",
    description: "Toast after setBuildFilter succeeds",
  },
  "services.buildFilterError": {
    message:
      "No se pudieron actualizar los filtros de build. Inténtalo de nuevo.",
    description: "Toast after setBuildFilter fails",
  },
  "services.preDeployLabel": {
    message: "Comando previo al despliegue",
    description: "Build & Deploy: label for the pre-deploy command field",
  },
  "services.preDeployHint": {
    message:
      "Se ejecuta una vez sobre la nueva imagen antes de que sirva tráfico (p. ej. una migración de base de datos). Un código de salida distinto de cero hace fallar el despliegue y mantiene activa la versión anterior.",
    description: "Build & Deploy: help text for the pre-deploy command field",
  },
  "services.preDeployPaidOnly": {
    message:
      "Los comandos previos al despliegue están disponibles en los planes de pago.",
    description:
      "Build & Deploy: why the pre-deploy command field is disabled on a Free service (Render: paid web services, private services and background workers; w8/m131)",
  },
  "services.preDeployPlaceholder": {
    message: "p. ej. npm run migrate",
    description: "Build & Deploy: placeholder for the pre-deploy command input",
  },
  "services.preDeployEdit": {
    message: "Editar comando previo al despliegue",
    description:
      "Build & Deploy: accessible label for the pre-deploy edit-pencil button",
  },
  "services.healthCheckPathConfirmTitle": {
    message: "¿Cambiar la ruta de comprobación de estado a {value}?",
    description:
      "Settings: Health Check Path change confirmation title (w4/616)",
  },
  "services.healthCheckPathConfirmEmpty": {
    message: "una comprobación TCP",
    description:
      "Settings: empty Health Check Path phrase in its confirmation title",
  },
  "services.maxShutdownDelayConfirmTitle": {
    message: "¿Cambiar el retraso máximo de apagado a {value} segundos?",
    description:
      "Settings: Max shutdown delay change confirmation title (w4/616)",
  },
  "services.preDeployConfirmTitle": {
    message: "¿Cambiar el comando previo al despliegue a {value}?",
    description:
      "Settings: Pre-Deploy Command change confirmation title (w4/616)",
  },
  "services.preDeployConfirmEmpty": {
    message: "ningún comando previo al despliegue",
    description:
      "Settings: empty Pre-Deploy Command phrase in its confirmation title",
  },
  "services.settingRedeployConfirmBody": {
    message:
      "El servicio se vuelve a desplegar con la nueva configuración. Las solicitudes en curso terminan antes de reemplazar las instancias antiguas.",
    description:
      "Settings: confirmation body for a setting whose save starts a deploy (w4/616)",
  },
  "services.settingRedeployConfirmBodyNeverLive": {
    message:
      "El servicio se despliega con la nueva configuración. Aún no sirve nada: pasa a estar activo cuando un despliegue tenga éxito.",
    description:
      "Settings: deploying-setting confirmation body for a service that has never gone live (w4/616)",
  },
  "services.preDeployConfirmBody": {
    message:
      "El servicio se vuelve a desplegar y el nuevo comando se ejecuta antes de que la nueva versión pase a estar activa. Las solicitudes en curso terminan antes de reemplazar las instancias antiguas.",
    description:
      "Settings: Pre-Deploy Command change confirmation body (w4/616)",
  },
  "services.preDeployConfirmBodyNeverLive": {
    message:
      "El servicio se despliega y el nuevo comando se ejecuta antes de que la versión pase a estar activa. Aún no sirve nada: pasa a estar activo cuando un despliegue tenga éxito.",
    description:
      "Settings: Pre-Deploy Command confirmation body for a service that has never gone live (w4/616)",
  },
  "services.preDeploySuccess": {
    message:
      "Comando previo al despliegue actualizado. Se inició un nuevo despliegue.",
    description: "Toast after setPreDeployCommand succeeds",
  },
  "services.preDeployError": {
    message:
      "No se pudo actualizar el comando previo al despliegue. Inténtalo de nuevo.",
    description: "Toast after setPreDeployCommand fails",
  },
  "services.autoDeployLabel": {
    message: "Despliegue automático",
    description: "Build & Deploy: label for the auto-deploy toggle",
  },
  "services.autoDeployOnCommit": {
    message: "En cada commit",
    description:
      "Auto-Deploy select option: redeploy on every matching git push (Render's autoDeployTrigger 'commit', w5/m53)",
  },
  "services.autoDeployAfterChecks": {
    message: "Tras superar las comprobaciones de CI",
    description:
      "Auto-Deploy select option: redeploy a pushed commit once all its GitHub checks pass (Render's autoDeployTrigger 'checksPass', w1/m181)",
  },
  "services.autoDeployOff": {
    message: "Desactivado",
    description:
      "Auto-Deploy select option: never redeploy on push (Render's autoDeployTrigger 'off')",
  },
  "services.autoDeployEdit": {
    message: "Editar despliegue automático",
    description:
      "Accessible label for the Auto-Deploy select's edit-pencil button",
  },
  "services.autoDeployViaGitHub": {
    message:
      "Un push a la rama seguida vuelve a desplegar automáticamente mediante la app de GitHub.",
    description:
      "Build & Deploy: source indicator when the connected GitHub App installation grants THIS repo (pushDeliveryMethod github_app)",
  },
  "services.autoDeployViaWebhook": {
    message:
      "Un push solo vuelve a desplegar si el webhook de git manual del repositorio está configurado con el BEX_WEBHOOK_SECRET de esta instalación.",
    description:
      "Build & Deploy: source indicator when the GitHub App cannot deliver a push for this repo — no connection, another git host, or a repo the installation does not grant (pushDeliveryMethod manual_webhook)",
  },
  "services.autoDeployUnavailable": {
    message:
      "El despliegue automático no puede ejecutarse para este repositorio: GitHub no envía sus pushes a bex. Conecta el repositorio mediante la app de GitHub o activa los despliegues con el Deploy Hook.",
    description:
      "Build & Deploy: source indicator when no push can reach bex for this repo — the GitHub App does not cover it and this install rejects the shared manual webhook key (pushDeliveryMethod unavailable, w4/483)",
  },
  "services.autoDeployDeliveryUnknown": {
    message:
      "Comprobando cómo llega a bex un push a la rama seguida… Si esto persiste, GitHub no es accesible y no se puede confirmar el método de entrega.",
    description:
      "Build & Deploy: source indicator while the per-repo push-delivery check is loading or returned unknown (GitHub unreachable) — states the uncertainty instead of naming a mechanism (w6/m99)",
  },
  "services.autoDeployOnSuccess": {
    message: "Despliegue automático activado.",
    description: "Toast after enabling auto-deploy",
  },
  "services.autoDeployChecksSuccess": {
    message:
      "El despliegue automático ahora espera a que pasen las comprobaciones de CI.",
    description: "Toast after choosing After CI Checks Pass",
  },
  "services.autoDeployOffSuccess": {
    message: "Despliegue automático desactivado.",
    description: "Toast after disabling auto-deploy",
  },
  "services.autoDeployError": {
    message: "No se pudo cambiar el despliegue automático. Inténtalo de nuevo.",
    description: "Toast after setAutoDeploy fails",
  },
  "services.deployHookTitle": {
    message: "Deploy Hook",
    description: "Settings tab: secret Deploy Hook section title",
  },
  "services.deployHookDescription": {
    message: "Activa un despliegue desde CI con una única URL secreta.",
    description: "Settings tab: Deploy Hook section description",
  },
  "services.deployHookURLLabel": {
    message: "URL del Deploy Hook",
    description: "Accessible label for the secret Deploy Hook URL field",
  },
  "services.deployHookReveal": {
    message: "Mostrar la URL del Deploy Hook",
    description: "Accessible label for the reveal-secret button",
  },
  "services.deployHookHide": {
    message: "Ocultar la URL del Deploy Hook",
    description: "Accessible label for the hide-secret button",
  },
  "services.deployHookCopy": {
    message: "Copiar la URL del Deploy Hook",
    description: "Accessible label for the Deploy Hook copy button",
  },
  "services.deployHookCopied": {
    message: "URL del Deploy Hook copiada.",
    description: "Toast after copying the Deploy Hook URL",
  },
  "services.deployHookCopyError": {
    message: "No se pudo copiar la URL del Deploy Hook.",
    description: "Toast after Deploy Hook URL clipboard failure",
  },
  "services.deployHookSecretHint": {
    message:
      "Mantén esta URL en secreto. Cualquiera que la tenga puede desplegar este servicio sin una clave de API.",
    description: "Security warning below the Deploy Hook URL",
  },
  "services.deployHookRegenerate": {
    message: "Regenerar Hook",
    description: "Deploy Hook rotation button",
  },
  "services.deployHookRegenerateTitle": {
    message: "¿Regenerar el Deploy Hook?",
    description: "Deploy Hook rotation confirmation title",
  },
  "services.deployHookRegenerateWarning": {
    message:
      "La URL actual dejará de funcionar de inmediato. Actualiza todos los sistemas de CI, cron jobs e integraciones que la usen.",
    description: "Deploy Hook rotation confirmation warning",
  },
  "services.deployHookCancel": {
    message: "Cancelar",
    description: "Deploy Hook rotation confirmation cancel button",
  },
  "services.deployHookRegenerateConfirm": {
    message: "Regenerar",
    description: "Deploy Hook rotation confirmation action",
  },
  "services.deployHookRegenerated": {
    message: "Deploy Hook regenerado. La URL anterior ya no funciona.",
    description: "Toast after successful Deploy Hook rotation",
  },
  "services.deployHookRegenerateError": {
    message: "No se pudo regenerar el Deploy Hook. Inténtalo de nuevo.",
    description: "Toast after Deploy Hook rotation fails",
  },
  "services.deployHookLoadError": {
    message: "No se pudo cargar la URL del Deploy Hook.",
    description: "Deploy Hook section query error",
  },
  "services.colType": {
    message: "Tipo",
    description: "Services table column header (service type)",
  },
  "services.typeWeb": {
    message: "Web Service",
    description: "Service-type badge: an HTTP service exposed at a URL",
  },
  "services.typePrivate": {
    message: "Private Service",
    description:
      "Service-type badge: an HTTP service reachable only in-cluster",
  },
  "services.typeWorker": {
    message: "Background Worker",
    description: "Service-type badge: runs with no HTTP port/URL",
  },
  "services.typeCron": {
    message: "Cron Job",
    description: "Service-type badge: runs a command on a schedule",
  },
  "services.typeStatic": {
    message: "Static Site",
    description:
      "Service-type badge: built output served from the object-store origin",
  },
  "services.staticTitle": {
    message: "Static Site",
    description:
      "Settings section title for a static site's publish dir + edge rules",
  },
  "services.staticDescription": {
    message:
      "El directorio de salida publicado y las reglas de borde que se aplican al servirlo.",
    description: "Static Site settings section description",
  },
  "services.staticEdgeRulesHint": {
    message:
      "Las redirecciones/reescrituras y los encabezados de respuesta tienen sus propias páginas:",
    description:
      "Settings hint pointing at the dedicated Redirects/Rewrites + Headers pages",
  },
  "services.staticCancel": {
    message: "Cancelar",
    description: "Cancel an inline edit",
  },
  "services.publishPathEscapes": {
    message:
      "Esta ruta sale del repositorio. Un directorio de publicación se resuelve desde el directorio raíz, así que quita los segmentos ..",
    description:
      "Inline error under a repo-backed static site's publish directory whose .. segments leave the checkout (w21/050)",
  },
  "services.publishPathControlChar": {
    message: "No debe contener un carácter de control ni un salto de línea",
    description:
      "Inline error under a static site's publish directory with a control character (w21/050)",
  },
  "services.publishPathLabel": {
    message: "Directorio de publicación",
    description: "Label for a static site's publishPath",
  },
  "services.publishPathPlaceholder": {
    message: "dist",
    description: "Placeholder for the publish directory input",
  },
  "services.publishPathEdit": {
    message: "Editar directorio de publicación",
    description:
      "Accessible label for the publish-directory edit-pencil button",
  },
  "services.publishPathHint": {
    message:
      "El directorio de salida del build que se sirve como raíz del sitio (p. ej. dist, build, public). Cambiarlo vuelve a publicar el sitio.",
    description: "Help text under the publish directory field",
  },
  "services.publishPathSaved": {
    message: "Directorio de publicación actualizado",
    description: "Toast after saving a static site's publishPath",
  },
  "services.publishPathRepublishNote": {
    message: "El sitio se volverá a publicar en breve.",
    description: "Toast description after changing publishPath",
  },
  "services.publishPathError": {
    message: "No se pudo actualizar el directorio de publicación",
    description: "Error toast for a failed publishPath change",
  },
  "services.routesTitle": {
    message: "Redirecciones y reescrituras",
    description: "Title for the static-site routes editor",
  },
  "services.routesHint": {
    message:
      "Se evalúan en orden y gana la primera coincidencia. Una redirección devuelve 301; una reescritura sirve otra ruta (un fallback de SPA reescribe /* a /index.html). Un segmento :name captura un segmento de ruta, p. ej. /blog/posts/:postid → /blog/:postid.",
    description: "Help text for the routes editor",
  },
  "services.routeAdd": {
    message: "Añadir regla",
    description: "Add a route rule",
  },
  "services.routeType": {
    message: "Tipo",
    description: "Route type column",
  },
  "services.routeSource": {
    message: "Origen",
    description: "Route source-path column",
  },
  "services.routeDestination": {
    message: "Destino",
    description: "Route destination-path column",
  },
  "services.routeRewrite": {
    message: "Reescritura",
    description: "Route type: rewrite",
  },
  "services.routeRedirect": {
    message: "Redirección",
    description: "Route type: redirect",
  },
  "services.routeTypeAria": {
    message: "Tipo de la regla {n}",
    description:
      "A route rule's type select (aria-label), named by its row as Remove rule {n} is (w4/541)",
  },
  "services.routeSourceAria": {
    message: "Origen de la regla {n}",
    description:
      "A route rule's source field (aria-label), named by its row (w4/541)",
  },
  "services.routeDestinationAria": {
    message: "Destino de la regla {n}",
    description:
      "A route rule's destination field (aria-label), named by its row (w4/541)",
  },
  "services.routeRemove": {
    message: "Quitar la regla {n}",
    description: "Remove a route rule (aria-label)",
  },
  "services.routesSave": {
    message: "Guardar rutas",
    description: "Save the routes list",
  },
  "services.staticRuleRequired": {
    message: "Obligatorio",
    description: "Inline error under an empty static-site rule field (w4/145)",
  },
  "services.staticRulePathSlash": {
    message: "Debe ser una ruta que empiece con /",
    description:
      "Inline error under a static-site rule path that doesn't start with / (w4/145)",
  },
  "services.staticRuleExternalHost": {
    message:
      "Usa una URL http(s) simple con un host fijo, p. ej. https://example.org/new",
    description:
      "Inline error under a redirect destination that is an absolute URL without a literal host (w8/162)",
  },
  "services.staticRuleLocalPath": {
    message: "Debe ser una ruta de este sitio, no //host",
    description:
      "Inline error under a redirect/rewrite destination that is a network-path reference (w4/145)",
  },
  "services.staticRuleDestinationPathOrUrl": {
    message: "Debe ser una ruta o una URL http(s)",
    description:
      "Inline error under a redirect/rewrite destination whose first segment reads as a scheme other than http(s), such as javascript: (w33/007)",
  },
  "services.staticRuleFixRows": {
    message: "Corrige las filas marcadas para guardar.",
    description:
      "Note beside a disabled Save while a static-site rule row is invalid (w4/145)",
  },
  "services.staticRuleEmptyRow": {
    message: "Rellena o quita la fila vacía para guardar.",
    description:
      "Note beside a disabled Save while a static-site rule row is still completely empty (w4/145)",
  },
  "services.staticRuleRow": {
    message: "Fila {row}:",
    description:
      "Prefix naming the 1-based rule row in a server refusal, replacing its zero-based wire index (w4/145)",
  },
  "services.staticRulePathTooLong": {
    message: "Máximo 2048 bytes",
    description:
      "Inline error under an over-long static-site rule path; the server counts UTF-8 bytes, so a Chinese character is 3 (w4/402, w21/043)",
  },
  "services.staticRuleSourceRepeatsPlaceholder": {
    message: "Cada :name solo puede aparecer una vez en el origen",
    description:
      "Inline error under a redirect/rewrite source that declares the same :name placeholder twice (w21/043)",
  },
  "services.staticRuleUndeclaredPlaceholder": {
    message: "Usa un :name que el origen no declara",
    description:
      "Inline error under a redirect/rewrite destination whose :name placeholder (other than :splat) the source never declares (w21/043)",
  },
  "services.staticRuleHeaderNameTooLong": {
    message: "Máximo 256 caracteres",
    description: "Inline error under an over-long header name (w4/402)",
  },
  "services.staticRuleHeaderNameInvalid": {
    message: "No es un nombre de encabezado HTTP válido",
    description:
      "Inline error under a header name with characters a header name can't hold (w4/402)",
  },
  "services.staticRuleHeaderNameReserved": {
    message: "Este encabezado lo define el servidor y no se puede anular",
    description:
      "Inline error under a reserved header name such as Content-Length (w4/402)",
  },
  "services.staticRuleHeaderValueTooLong": {
    message: "Máximo 4096 bytes",
    description: "Inline error under an over-long header value (w4/402)",
  },
  "services.staticRuleHeaderValueInvalid": {
    message: "Contiene un carácter que un valor de encabezado no admite",
    description:
      "Inline error under a header value with a control character (w4/402)",
  },
  "services.staticRuleCookieDomain": {
    message: "Set-Cookie no puede definir un Domain en alojamiento compartido",
    description:
      "Inline error under a Set-Cookie value carrying a Domain attribute (w4/402)",
  },
  "services.staticRoutesSaved": {
    message: "Rutas actualizadas",
    description: "Toast after saving routes",
  },
  "services.staticRoutesError": {
    message: "No se pudieron actualizar las rutas",
    description: "Error toast for a failed routes save",
  },
  "services.headersTitle": {
    message: "Encabezados de respuesta",
    description: "Title for the static-site custom-headers editor",
  },
  "services.headersHint": {
    message:
      "Encabezados de respuesta personalizados que se añaden a las respuestas cuya ruta coincide. Los encabezados de estructura del cuerpo y de conexión, como Content-Length, los define el servidor y no se pueden anular. Define Content-Encoding solo en archivos que estén realmente almacenados comprimidos.",
    description: "Help text for the headers editor",
  },
  "services.headerAdd": {
    message: "Añadir encabezado",
    description: "Add a custom header",
  },
  "services.headerPath": {
    message: "Ruta",
    description: "Header path column",
  },
  "services.headerName": {
    message: "Nombre",
    description: "Header name column",
  },
  "services.headerValue": {
    message: "Valor",
    description: "Header value column",
  },
  "services.headerPathAria": {
    message: "Ruta del encabezado {n}",
    description:
      "A header rule's path field (aria-label), named by its row as Remove header {n} is (w4/541)",
  },
  "services.headerNameAria": {
    message: "Nombre del encabezado {n}",
    description:
      "A header rule's name field (aria-label), named by its row (w4/541)",
  },
  "services.headerValueAria": {
    message: "Valor del encabezado {n}",
    description:
      "A header rule's value field (aria-label), named by its row (w4/541)",
  },
  "services.headerRemove": {
    message: "Quitar el encabezado {n}",
    description: "Remove a header (aria-label)",
  },
  "services.headersSave": {
    message: "Guardar encabezados",
    description: "Save the headers list",
  },
  "services.staticHeadersSaved": {
    message: "Encabezados actualizados",
    description: "Toast after saving headers",
  },
  "services.staticHeadersError": {
    message: "No se pudieron actualizar los encabezados",
    description: "Error toast for a failed headers save",
  },
  "services.typeUnknown": {
    message: "Servicio",
    description: "Service-type badge fallback for an unrecognized type",
  },
  "services.cronRunColDuration": {
    message: "Duración",
    description:
      "Cron Runs page: the Duration column's header, and a row's duration for assistive tech (Render's DURATION, w3/m556)",
  },
  "services.cronRunStatusSucceeded": {
    message: "Ejecución correcta",
    description:
      "Cron Runs page: a successful run's status, in its icon's tooltip (Render's text, w3/m556)",
  },
  "services.cronRunStatusFailed": {
    message: "Ejecución fallida",
    description:
      "Cron Runs page: a failed run's status, in its icon's tooltip (Render's text, w3/m556)",
  },
  "services.cronRunStatusCanceled": {
    message: "Cancelada",
    description:
      "Cron Runs page: a canceled run's status in its icon's tooltip, and its line when no one is named as canceling it (Render's text, w3/m556)",
  },
  "services.cronRunStatusInProgress": {
    message: "En curso",
    description:
      "Cron Runs page: the status of a run that has not ended, started or not, in its icon's tooltip (Render's text, w3/m556)",
  },
  "services.cronRunStatusLastSuccessful": {
    message: "Última ejecución correcta",
    description:
      "Cron Runs page: the newest successful run's status, in its icon's tooltip (Render's text, w3/m556)",
  },
  "services.cronRunCancel": {
    message: "Cancelar",
    description:
      "Cron Runs page: a run in progress's Cancel button, which cancels it at once (Render's text, w3/m556)",
  },
  "services.cronRunCancelError": {
    message: "No se pudo cancelar la ejecución de cron.",
    description: "Toast after cron run cancellation fails",
  },
  "services.cronTriggerRun": {
    message: "Iniciar ejecución",
    description: "Button that fires an immediate cron run (Render parity)",
  },
  "services.cronTriggerError": {
    message:
      "No se pudo iniciar una ejecución. Inténtalo de nuevo en un momento.",
    description:
      "Toast, kept until dismissed, when Trigger Run fails and the server gave no message (Render's text, w3/m555)",
  },
  "services.cronTriggerForbidden": {
    message: "No tienes permiso para ejecutar este Cron Job.",
    description:
      "Toast, kept until dismissed, when Trigger Run is refused for permission (Render's text, w3/m555)",
  },
  "services.cronTriggerNoBuild": {
    message: "No se detectó ninguna build correcta. Consulta {deploys}.",
    description:
      "Tooltip of a disabled Trigger Run on a repo-backed cron job whose build never succeeded; {deploys} is a link to its Deploys page, named by services.navDeploys (Render's text, w3/m555)",
  },
  "services.cronTriggerSuspended": {
    message: "Reanuda este Cron Job para iniciar una ejecución.",
    description:
      "Tooltip of a disabled Trigger Run while the cron job is suspended, shown in the Runs page's empty states (Render's text, w3/m556)",
  },
  "services.cronRunsLoadMore": {
    message: "Cargar más",
    description: "Cron run history pagination button",
  },
  "services.cronRunsLoadingMore": {
    message: "Cargando…",
    description: "Cron run history pagination busy label",
  },
  "services.cronRunsLoadError": {
    message: "No se pudieron cargar las ejecuciones de cron.",
    description: "Cron run history read error",
  },
  "services.cronRunsLoadFailed": {
    message:
      "Lo sentimos. No pudimos cargar las ejecuciones de este Cron Job. ¿Recargas e intentas de nuevo?",
    description:
      "Cron Runs page: the warning shown in place of the list when the runs could not be read (Render's text, w3/m556)",
  },
  "services.cronRunsSearchPlaceholder": {
    message: "Buscar ejecuciones del Cron Job",
    description:
      "Cron Runs page: the search field's placeholder and name (Render's text, w3/m556)",
  },
  "services.cronRunsSearchClear": {
    message: "Borrar búsqueda",
    description:
      "Cron Runs page: the search field's clear button, for assistive tech (w3/m556)",
  },
  "services.cronRunsColRun": {
    message: "Ejecución",
    description:
      "Cron Runs page: the list's first column header, before the count (Render's RUN, w3/m556)",
  },
  "services.cronRunsColTrigger": {
    message: "Origen",
    description:
      "Cron Runs page: the Trigger column's header, and a row's trigger for assistive tech (Render's TRIGGER, w3/m556)",
  },
  "services.cronRunTitle": {
    message: "Ejecución del Cron Job",
    description: "Cron Runs page: a run row's title (Render's text, w3/m556)",
  },
  "services.cronRunStartedAgo": {
    message: "Ejecutado {timestamp}",
    description:
      'Cron Runs page: when a run started, under its title; {timestamp} is elapsed text like "2 hours ago" (Render\'s "Run 21h ago", w3/m556)',
  },
  "services.cronRunBy": {
    message: "por {user}",
    description:
      'Cron Runs page: after when a run started, the email of the user who triggered it (Render\'s "by <user>", w3/m556)',
  },
  "services.cronRunByYou": {
    message: "por ti",
    description:
      'Cron Runs page: after when a run started, when the viewer triggered it (Render\'s "by you", w3/m556)',
  },
  "services.cronRunCanceledBy": {
    message: "Cancelada por {user}",
    description:
      "Cron Runs page: a canceled run's line naming the email of the user who canceled it (Render's text, w3/m556)",
  },
  "services.cronRunCanceledByYou": {
    message: "Cancelada por ti",
    description:
      "Cron Runs page: a canceled run's line when the viewer canceled it (Render's \"Canceled by you\", w3/m556)",
  },
  "services.cronRunTriggerScheduled": {
    message: "Programada",
    description:
      "Cron Runs page: the trigger of a run the schedule started (Render's text, w3/m556)",
  },
  "services.cronRunTriggerManual": {
    message: "Manual",
    description:
      "Cron Runs page: the trigger of a run a user started (Render's text, w3/m556)",
  },
  "services.cronRunsNoMatchTitle": {
    message: "No hay resultados",
    description:
      "Cron Runs page: title when the search matches none of the loaded runs (Render's text, w3/m556)",
  },
  "services.cronRunsNoMatchBody": {
    message:
      "¿No encuentras lo que buscas? Prueba a ajustar la búsqueda y revisa si hay errores tipográficos.",
    description:
      "Cron Runs page: body when the search matches none of the loaded runs (Render's text, w3/m556)",
  },
  "services.cronRunsResetSearch": {
    message: "Restablecer búsqueda",
    description:
      "Cron Runs page: the button that clears a search that matched nothing (Render's text, w3/m556)",
  },
  "services.cronRunsEmptyRecentTitle": {
    message: "Este Cron Job no se ha ejecutado en los últimos 90 días.",
    description:
      "Cron Runs page: title when a cron job whose build succeeded lists no runs (Render's text, w3/m556)",
  },
  "services.cronRunsEmptyRecentBody": {
    message:
      "Puedes iniciar una ejecución manualmente o esperar a la siguiente ejecución programada.",
    description:
      "Cron Runs page: body when a cron job whose build succeeded lists no runs, above Trigger Run (Render's text, w3/m556)",
  },
  "services.cronRunsEmptyNeverTitle": {
    message: "Este Cron Job aún no se ha ejecutado.",
    description:
      "Cron Runs page: title when a cron job whose build never succeeded has no runs (Render's text, w3/m556)",
  },
  "services.cronRunsEmptyNeverBody": {
    message:
      "Se ejecutará cuando termine su primera build, o puedes iniciar una ejecución manualmente.",
    description:
      "Cron Runs page: body when a cron job whose build never succeeded has no runs, above Trigger Run (Render's text, w3/m556)",
  },
  "services.cronRunsCount_one": {
    message: "{count} ejecución",
    description:
      "Cron Runs page: the list's count, for assistive tech, beside RUN (Render's \"30 runs\", w3/m556)",
  },
  "services.cronRunsCount_many": {
    message: "{count} de ejecuciones",
    description:
      "Cron Runs page: the list's count, for assistive tech, beside RUN (Render's \"30 runs\", w3/m556)",
  },
  "services.cronRunsCount_other": {
    message: "{count} ejecuciones",
    description:
      "Cron Runs page: the list's count, for assistive tech, beside RUN (Render's \"30 runs\", w3/m556)",
  },
  "services.suspendCardTitle": {
    message: "Suspender servicio",
    description: "Settings tab suspend section title",
  },
  "services.suspendCardDescription": {
    message:
      "Suspender tu servicio lo apaga y deja de servir tráfico. Su URL y sus certificados se conservan, y puedes reanudarlo en cualquier momento.",
    description:
      "Settings tab suspend section description, for the types served at a public host",
  },
  "services.suspendCardDescriptionNoUrl": {
    message:
      "Suspender tu servicio lo apaga y deja de ejecutarse. Puedes reanudarlo en cualquier momento.",
    description:
      "Settings tab suspend section description, for private services and background workers — neither has a public URL or certificates to keep",
  },
  "services.resumeCardTitle": {
    message: "Reanudar servicio",
    description:
      "Settings tab resume section title (shown when service is suspended)",
  },
  "services.resumeCardDescription": {
    message:
      "Reanudar tu servicio lo vuelve a poner en línea y empieza a servir tráfico de nuevo.",
    description: "Settings tab resume section description",
  },
  "services.resumeCardDescriptionCrashLoop": {
    message:
      "Suspendimos este servicio porque no logró iniciarse ni estar en buen estado de forma repetida durante 24 horas. Suele deberse a un error de configuración, un fallo en el código de la aplicación o un plan de cómputo demasiado pequeño. Corrige la causa y luego reanúdalo para volver a ponerlo en línea.",
    description:
      "Settings tab resume section description for a service the platform suspended for a crash loop (stuck_crashlooping)",
  },
  "services.resumeCardDescriptionCron": {
    message: "Reanudar este Cron Job reanuda las ejecuciones programadas.",
    description: "Settings tab resume section description for a cron job",
  },
  "services.dangerZoneTitle": {
    message: "Zona de peligro",
    description: "Settings tab delete section title (destructive)",
  },
  "services.dangerZoneDescription": {
    message:
      "Eliminar un servicio lo quita de forma permanente, junto con su despliegue y su URL. Esta acción no se puede deshacer.",
    description:
      "Settings tab delete section description, for the types served at a public host",
  },
  "services.dangerZoneDescriptionNoUrl": {
    message:
      "Eliminar un servicio lo quita de forma permanente, junto con su despliegue. Esta acción no se puede deshacer.",
    description:
      "Settings tab delete section description, for private services, background workers and cron jobs — none of which have a URL to remove",
  },
  "services.deleteButton": {
    message: "Eliminar servicio",
    description: "Danger-zone button that opens the delete-confirm dialog",
  },
  "services.deleteConfirmTitle": {
    message: "¿Eliminar {name}?",
    description: "Delete-confirm dialog title",
  },
  "services.deleteConfirmBody": {
    message:
      "Esta acción no se puede deshacer. ¿Seguro que quieres eliminar este servicio ({type})?",
    description:
      "Delete-confirm dialog body ({type} = lowercase Render type words, e.g. 'web service')",
  },
  "services.deleteConfirmPrompt": {
    message: "Escribe {phrase} a continuación para confirmar.",
    description:
      "Body prompt naming Render's exact 'sudo delete <type> <name>' phrase (rendered bold by SudoCommandField)",
  },
  "services.deleteCancel": {
    message: "Cancelar",
    description: "Delete-confirm dialog cancel button",
  },
  "services.deleteConfirm": {
    message: "Eliminar servicio",
    description:
      "Delete-confirm dialog submit button (armed once the name matches)",
  },
  "services.deleteSuccess": {
    message: "{name} eliminado",
    description: "Toast on a successful service delete",
  },
  "services.deleteError": {
    message: "No se pudo eliminar {name}",
    description: "Toast on a failed service delete",
  },
  "services.newServiceButton": {
    message: "Nuevo servicio",
    description:
      "Button on the services list page that opens the create wizard",
  },
  "services.createTitle": {
    message: "Nuevo servicio",
    description: "Create-wizard page title",
  },
  "services.createWebTitle": {
    message: "Nuevo Web Service",
    description:
      "Create-wizard page title when the web_service type is selected",
  },
  "services.createWebDescription": {
    message:
      "Despliega un Web Service desde un repositorio Git o una imagen de Docker.",
    description:
      "Create-wizard page subtitle when the web_service type is selected",
  },
  "services.createPrivateTitle": {
    message: "Nuevo Private Service",
    description:
      "Create-wizard page title when the private_service type is selected",
  },
  "services.createPrivateDescription": {
    message:
      "Despliega un Private Service desde un repositorio Git o una imagen de Docker.",
    description:
      "Create-wizard page subtitle when the private_service type is selected",
  },
  "services.createWorkerTitle": {
    message: "Nuevo Background Worker",
    description:
      "Create-wizard page title when the background_worker type is selected",
  },
  "services.createWorkerDescription": {
    message:
      "Despliega un Background Worker desde un repositorio Git o una imagen de Docker.",
    description:
      "Create-wizard page subtitle when the background_worker type is selected",
  },
  "services.createStaticTitle": {
    message: "Nuevo Static Site",
    description:
      "Create-wizard page title when the static_site type is selected",
  },
  "services.createStaticDescription": {
    message: "Compila y despliega un Static Site desde un repositorio Git.",
    description:
      "Create-wizard page subtitle when the static_site type is selected",
  },
  "services.createCronTitle": {
    message: "Nuevo Cron Job",
    description: "Create-wizard page title when the cron_job type is selected",
  },
  "services.createCronDescription": {
    message: "Ejecuta un comando de forma recurrente según una programación.",
    description:
      "Create-wizard page subtitle when the cron_job type is selected",
  },
  "services.createSourceTitle": {
    message: "Código fuente",
    description: "Create-wizard source-picker section label",
  },
  "services.createTabGitHub": {
    message: "GitHub",
    description: "Create-wizard source-tab label for connected GitHub repos",
  },
  "services.createTabPublicGit": {
    message: "URL de Git pública",
    description: "Create-wizard source-tab label for a public git URL",
  },
  "services.createTabImage": {
    message: "Imagen existente",
    description: "Create-wizard source-tab label for a pre-built Docker image",
  },
  "services.createRepoSearchShort": {
    message: "Buscar…",
    description:
      "Repository search placeholder where the field is narrow (a phone); the field keeps its full accessible name",
  },
  "services.createRepoCurrent": {
    message: "Actual",
    description:
      "Update Source repository list: tag on the repository the service uses now, shown once another one is chosen",
  },
  "services.createRepoSearchPlaceholder": {
    message: "Buscar repositorios…",
    description: "Create-wizard GitHub tab repo-search input placeholder",
  },
  "services.createRepoPrivateBadge": {
    message: "Privado",
    description: "Badge on a private GitHub repo row in the repo picker",
  },
  "services.createRepoEmpty": {
    message: "No se encontraron repositorios.",
    description:
      "Create-wizard GitHub tab empty state (no repos in the installation)",
  },
  "services.createRepoNoMatch": {
    message: "Ningún repositorio coincide con tu búsqueda.",
    description:
      "Create-wizard GitHub tab empty state when the search filter returns nothing",
  },
  "services.createRepoEmptyHint": {
    message:
      "Da a bex acceso a un repositorio en GitHub o conecta otra cuenta.",
    description:
      "Source picker GitHub tab, under the empty state: what to do when the connected accounts grant no repository. Credentials is the menu beside the search field",
  },
  "services.createRepoLoadError": {
    message: "No se pudieron cargar tus repositorios.",
    description:
      "Source picker GitHub tab: the repository list could not be read, shown with a Try again button. Not the same as having no repositories",
  },
  "services.createRepoListLabel": {
    message: "Repositorios",
    description:
      "Accessible name of the repository list in the source picker's GitHub tab",
  },
  "services.createRepoClearSearch": {
    message: "Borrar búsqueda",
    description:
      "Source picker GitHub tab: button under the no-match message that empties the repository search field",
  },
  "services.sourceNoChangesDeploy": {
    message: "No hay cambios que desplegar.",
    description:
      "Update Source dialog footer, beside a disabled Deploy button (the GitHub, Public Git and Build source tabs): the dialog still shows the service's current source and build settings, so there is nothing to deploy",
  },
  "services.sourceNoChangesConnect": {
    message: "No hay cambios que conectar.",
    description:
      "Update Source dialog footer, beside a disabled Connect button (the Existing Image tab): the dialog still shows the image and registry credential the service already runs, so there is nothing to connect",
  },
  "services.sourceSelectRepo": {
    message: "Selecciona un repositorio.",
    description:
      "Update Source dialog footer: why Deploy is disabled on the GitHub tab before a repository is picked",
  },
  "services.sourceSelectBuildSource": {
    message: "Selecciona un origen de build.",
    description:
      "Update Source dialog footer: why Deploy is disabled on the Build source tab before one is picked",
  },
  "services.sourceEnterRepoUrl": {
    message: "Introduce la URL de un repositorio.",
    description:
      "Update Source dialog footer: why Deploy is disabled on the Public Git URL tab while the URL field is empty",
  },
  "services.sourceEnterImage": {
    message: "Introduce una imagen.",
    description:
      "Update Source dialog footer: why Connect is disabled on the Existing Image tab while the image field is empty",
  },
  "services.createRepoOpenCredentials": {
    message: "Abrir Credenciales",
    description:
      "Source picker GitHub tab: button under the empty repository list that opens the Credentials menu beside the search field",
  },
  "services.sourcePending": {
    message: "Nuevo origen: {source}",
    description:
      "Update Source dialog footer: names what the primary button (Deploy, or Connect on the Existing Image tab) will deploy. {source} is a repository and branch, an image, or a build source's name",
  },
  "services.sourceBranchLocked": {
    message: "Disponible cuando elijas un repositorio.",
    description:
      "Update Source dialog: helper under the disabled Branch field before a repository is picked or a valid URL entered",
  },
  "services.sourceNoBuildSources": {
    message: "No hay ningún origen de build para elegir.",
    description:
      "Update Source dialog footer: why Deploy is disabled on the Build source tab when the workspace has no build sources",
  },
  "services.sourceFixField": {
    message: "Completa los campos de arriba para continuar.",
    description:
      "Update Source dialog footer: why the primary button (Deploy, or Connect on the Existing Image tab) is disabled while a field is malformed (its own message is under it) or a required one is empty",
  },
  "services.createRepoNoMatchHint": {
    message:
      "¿No aparece? Da a bex acceso a él en GitHub o conecta otra cuenta.",
    description:
      "Source picker GitHub tab, under the no-match message: the usual cause is a repository the connected accounts do not grant. Shown with Clear search and Open Credentials buttons",
  },
  "services.sourceBranchLockedUrl": {
    message: "Disponible cuando introduzcas la URL de un repositorio.",
    description:
      "Update Source dialog, Public Git URL tab: helper under the disabled Branch field before a valid URL is entered",
  },
  "services.createGitConnectPromptTitle": {
    message: "Conectar GitHub",
    description:
      "Create-wizard GitHub tab connect-prompt heading (no GitHub connection yet)",
  },
  "services.createGitConnectPromptBody": {
    message:
      "Conecta tu cuenta de GitHub para desplegar desde repositorios privados o públicos.",
    description:
      "Create-wizard GitHub tab connect-prompt body (no GitHub connection yet)",
  },
  "services.createPublicUrlLabel": {
    message: "URL del repositorio",
    description: "Create-wizard Public Git URL tab input label",
  },
  "services.createPublicUrlPlaceholder": {
    message: "https://github.com/you/your-repo",
    description: "Create-wizard Public Git URL tab input placeholder",
  },
  "services.createPublicUrlError": {
    message:
      "Introduce una URL https:// o una URL SSH de GitHub (git@github.com:owner/repo.git).",
    description: "Create-wizard Public Git URL tab validation message",
  },
  "services.createPublicUrlFileError": {
    message: "Usa la URL del repositorio, no la de una página de archivo.",
    description:
      "Create-wizard Public Git URL: a GitHub file (blob) page was pasted, which names no repository to build (w4/576)",
  },
  "services.createImageLabel": {
    message: "Imagen de Docker",
    description: "Create-wizard Existing Image tab input label",
  },
  "services.createImagePlaceholder": {
    message: "docker.io/library/nginx:latest",
    description: "Create-wizard Existing Image tab input placeholder",
  },
  "services.createImagePortHint": {
    message:
      "bex usa el puerto que declara la imagen (EXPOSE). Define el puerto abajo solo si la imagen escucha en un puerto que no declara.",
    description:
      "Create-wizard Existing Image tab hint: set the port below to the one the image listens on (w9/011, w6/m149)",
  },
  "services.createImageMalformed": {
    message:
      "Usa una referencia de imagen de Docker, p. ej. docker.io/library/nginx:latest — una ruta en minúsculas con una etiqueta o un digest opcionales.",
    description: "Inline error under a malformed Docker Image field (w4/406)",
  },
  "services.createImageTooLong": {
    message: "Máximo 512 bytes",
    description: "Inline error under an over-long Docker Image field (w4/406)",
  },
  "services.createImagePrivateHost": {
    message:
      "El host del registro debe ser público: no se pueden usar localhost ni direcciones privadas o de enlace local.",
    description:
      "Inline error under a Docker Image field naming a private registry host (w4/406)",
  },
  "services.createFieldPort": {
    message: "Puerto",
    description: "Create wizard: service port field label",
  },
  "services.createFieldPortFromImage": {
    message: "Desde la imagen",
    description:
      "Create wizard: port placeholder for an image-port service (a Docker-built or prebuilt-image web or private service), whose ports come from its image (w6/m148, w4/m243, w4/m323)",
  },
  "services.createFieldPortImageHint": {
    message:
      "Si lo dejas vacío, el puerto que declara la imagen pasa a ser $PORT.",
    description:
      "Create wizard: port hint for an image-port private service (Docker-built or prebuilt image), whose primary is also its $PORT — blank takes the image's declared ports (w6/m148, w4/m323, w4/615)",
  },
  "services.createFieldPortImageHintWeb": {
    message:
      "Déjalo vacío para enrutar el tráfico al puerto que declara la imagen; la variable $PORT sigue siendo 10000 salvo que definas un puerto.",
    description:
      "Create wizard: port hint for an image-port web service (Docker-built or prebuilt image) — the selected port gets the traffic, while $PORT stays Render's 10000 unless set (w4/m243, w4/615)",
  },
  "services.createFieldPortHint": {
    message:
      "Tu contenedor debe escuchar en el puerto {port}; bex enruta el tráfico allí y lo define como $PORT.",
    description: "Create wizard: service port field hint",
  },
  "services.createRegistryCredentialLabel": {
    message: "Credencial de registro",
    description: "Create-wizard private-image registry credential label",
  },
  "services.createRegistryCredentialPlaceholder": {
    message: "Selecciona una credencial de registro",
    description: "Create-wizard registry credential picker placeholder",
  },
  "services.createRegistryCredentialNone": {
    message: "Sin credencial — imagen pública",
    description: "Create-wizard registry credential picker empty option",
  },
  "services.createRegistryCredentialDescription": {
    message:
      "Elige una credencial guardada del espacio de trabajo cuando la imagen o la imagen base del Dockerfile sea privada.",
    description: "Create-wizard registry credential picker help text",
  },
  "services.registryCredentialEmpty": {
    message: "No hay credenciales de registro guardadas.",
    description: "Registry credential selector empty state",
  },
  "services.registryCredentialListError": {
    message: "No se pudieron cargar las credenciales de registro.",
    description: "Registry credential selector load error",
  },
  "services.registryCredentialManage": {
    message: "Gestionar credenciales de registro",
    description: "Link from a service credential selector to account settings",
  },
  "services.registryCredentialSettingsTitle": {
    message: "Credencial del registro de imágenes",
    description: "Service Settings private-image credential card title",
  },
  "services.registryCredentialSettingsDescription": {
    message:
      "Elige la credencial guardada que se usa para descargar esta imagen privada o la imagen base del Dockerfile.",
    description: "Service Settings private-image credential card description",
  },
  "services.registryCredentialSave": {
    message: "Guardar credencial",
    description: "Service Settings registry credential save action",
  },
  "services.registryCredentialSaved": {
    message: "Credencial de registro actualizada.",
    description: "Toast after attaching or changing a registry credential",
  },
  "services.registryCredentialCleared": {
    message: "Credencial de registro eliminada.",
    description: "Toast after clearing a registry credential",
  },
  "services.registryCredentialError": {
    message: "No se pudo actualizar la credencial de registro.",
    description: "Toast after a registry credential update fails",
  },
  "services.headerRegistryCredential": {
    message: "Credencial de registro:",
    description: "Service detail header label for a bound registry credential",
  },
  "services.createSettingsTitle": {
    message: "Configuración",
    description: "Create-wizard settings section heading",
  },
  "services.createFieldName": {
    message: "Nombre",
    description: "Create-wizard name input label",
  },
  "services.createFieldNamePlaceholder": {
    message: "my-service",
    description: "Create-wizard name input placeholder",
  },
  "services.createFieldNameError": {
    message:
      "Introduce un nombre de hasta 100 caracteres, sin saltos de línea ni caracteres de control.",
    description: "Create-wizard name validation message",
  },
  "services.createFieldNameResourceName": {
    message: "Nombre del recurso: {name}",
    description:
      "Create-wizard preview of the resource name (slug) bex derives from a free-form service name (w5/m153)",
  },
  "services.createFieldNameTaken": {
    message: "El nombre ya está en uso",
    description:
      "Create-wizard inline error when the service name is already taken in the current workspace (w4/m19)",
  },
  "services.createFieldNameUseSuggestion": {
    message: "Usar {name}",
    description:
      "Create-wizard button offering the suggested free name in place of a taken one (w4/m19)",
  },
  "services.createFieldNameChecking": {
    message: "Comprobando disponibilidad…",
    description:
      "Create-wizard transient message while the debounced name-availability check is in flight (w4/m19)",
  },
  "services.createFieldBranch": {
    message: "Rama",
    description: "Create-wizard branch input label (git sources)",
  },
  "services.createFieldBranchPlaceholder": {
    message: "Rama predeterminada del repositorio",
    description:
      "Create-wizard branch input placeholder when the repo's default is not known client-side: a blank branch builds the repository's default branch (w8/m76)",
  },
  "services.createFieldRootDir": {
    message: "Directorio raíz",
    description: "Create-wizard root-directory input label",
  },
  "services.createFieldRootDirPlaceholder": {
    message: "p. ej. backend",
    description: "Create-wizard root-directory input placeholder",
  },
  "services.createFieldRootDirHint": {
    message:
      "Subdirectorio desde el que compilar. Déjalo en blanco para usar la raíz del repositorio.",
    description: "Create-wizard root-directory input hint text",
  },
  "services.createFieldRuntime": {
    message: "Entorno de ejecución",
    description: "Create-wizard Render-compatible runtime selector label",
  },
  "services.createFieldBuildCommand": {
    message: "Comando de build",
    description: "Create-wizard Render-compatible build command label",
  },
  "services.createFieldStartCommand": {
    message: "Comando de inicio",
    description: "Create-wizard Render-compatible start command label",
  },
  "services.createFieldDockerfilePath": {
    message: "Ruta del Dockerfile",
    description: "Create-wizard Dockerfile-path label for the Docker runtime",
  },
  "services.createFieldDockerfilePathPlaceholder": {
    message: "Dockerfile",
    description: "Create-wizard Dockerfile-path placeholder",
  },
  "services.createFieldDockerfilePathHint": {
    message:
      "Ruta relativa al directorio raíz. Déjalo en blanco para usar Dockerfile.",
    description: "Create-wizard Dockerfile-path help text",
  },
  "services.createFieldDockerCommand": {
    message: "Comando de Docker",
    description: "Create-wizard Docker CMD override label (Render wording)",
  },
  "services.createFieldDockerCommandPlaceholder": {
    message: "Usar el CMD del Dockerfile",
    description: "Create-wizard optional Docker Command placeholder",
  },
  "services.imageCommandPlaceholder": {
    message: "Usar el comando predeterminado de la imagen",
    description:
      "Create-wizard and settings placeholder for an Existing Image service's optional Docker Command",
  },
  "services.imageCommandHint": {
    message:
      "Reemplaza el comando predeterminado de la imagen (su CMD). Déjalo en blanco para usar el predeterminado.",
    description: "Existing Image service Settings: Docker Command help text",
  },
  "services.createRuntimeNode": {
    message: "Node",
    description: "Create-wizard Node runtime option",
  },
  "services.createRuntimePython": {
    message: "Python 3",
    description: "Create-wizard Python runtime option",
  },
  "services.createRuntimeGo": {
    message: "Go",
    description: "Create-wizard Go runtime option",
  },
  "services.createRuntimeRuby": {
    message: "Ruby",
    description: "Create-wizard Ruby runtime option",
  },
  "services.createRuntimeRust": {
    message: "Rust",
    description: "Create-wizard Rust runtime option",
  },
  "services.createRuntimeElixir": {
    message: "Elixir",
    description: "Create-wizard Elixir runtime option",
  },
  "services.createRuntimeDocker": {
    message: "Docker",
    description: "Create-wizard Docker runtime option",
  },
  "services.createFieldPlan": {
    message: "Plan de cómputo",
    description: "Create-wizard plan-picker section label",
  },
  "services.createFieldAutoDeploy": {
    message: "Despliegue automático al hacer push",
    description: "Create-wizard auto-deploy toggle label",
  },
  "services.createFieldAutoDeployHint": {
    message:
      "Vuelve a desplegar automáticamente cuando haces push a esta rama.",
    description: "Create-wizard auto-deploy toggle hint text",
  },
  "services.createCancel": {
    message: "Cancelar",
    description: "Create-wizard cancel button",
  },
  "services.createSubmit": {
    message: "Desplegar servicio",
    description: "Create-wizard submit button",
  },
  "services.createSuccess": {
    message: "Desplegando {name}…",
    description: "Toast shown after createService succeeds",
  },
  "services.createError": {
    message: "No se pudo crear {name}. Inténtalo de nuevo.",
    description: "Toast shown after createService fails",
  },
  "services.scaleSuccess_one": {
    message: "Escalando a {count} instancia…",
    description:
      "Toast acknowledging that scaleService accepted the desired count; convergence is still asynchronous (singular)",
  },
  "services.scaleSuccess_many": {
    message: "Escalando a {count} de instancias…",
    description:
      "Toast acknowledging that scaleService accepted the desired count; convergence is still asynchronous (plural)",
  },
  "services.scaleSuccess_other": {
    message: "Escalando a {count} instancias…",
    description:
      "Toast acknowledging that scaleService accepted the desired count; convergence is still asynchronous (plural)",
  },
  "services.scaleError": {
    message: "No se pudo actualizar el número de instancias.",
    description: "Toast shown after scaleService fails",
  },
  "services.createTypePickerTitle": {
    message: "Tipo de servicio",
    description: "Label above the service type picker in the create wizard",
  },
  "services.createTypeWebDesc": {
    message: "Expón tu servicio en una URL pública",
    description:
      "Description shown under the Web Service type card in the create wizard",
  },
  "services.createTypePrivateDesc": {
    message: "Accesible solo dentro de la red de la plataforma",
    description: "Description shown under the Private Service type card",
  },
  "services.createTypeWorkerDesc": {
    message: "Ejecuta procesamiento en segundo plano sin puerto ni URL",
    description: "Description shown under the Background Worker type card",
  },
  "services.createTypeCronDesc": {
    message: "Ejecuta un comando de forma periódica",
    description: "Description shown under the Cron Job type card",
  },
  "services.createTypeStaticDesc": {
    message:
      "Compila y sirve un sitio estático desde almacenamiento de objetos",
    description: "Description shown under the Static Site type card",
  },
  "services.createFieldSchedule": {
    message: "Programación",
    description: "Label for the cron schedule field in the create wizard",
  },
  "services.createFieldSchedulePlaceholder": {
    message: "0 0 * * *",
    description: "Placeholder for the cron schedule field",
  },
  "services.createFieldScheduleHint": {
    message:
      "Una expresión crontab de 5 campos (minuto hora día mes día de la semana). Se ejecuta en UTC.",
    description: "Hint text under the schedule field",
  },
  "services.createFieldSchedulePreview": {
    message: "{description} · se ejecuta en UTC",
    description:
      "Live human-readable description of a valid schedule, shown under the field",
  },
  "services.createFieldScheduleError": {
    message:
      "Introduce una expresión cron válida de 5 campos, p. ej. 0 0 * * *.",
    description: "Validation error for an invalid cron expression",
  },
  "services.createFieldCommand": {
    message: "Comando",
    description: "Label for the command field in the create wizard",
  },
  "services.createFieldCommandPlaceholder": {
    message: "python script.py",
    description: "Placeholder for the command field",
  },
  "services.createFieldCommandHint": {
    message: "El comando que se ejecuta en cada invocación programada.",
    description: "Hint text under the command field",
  },
  "services.createFieldPublishPath": {
    message: "Directorio de publicación",
    description: "Label for the publish directory field in the create wizard",
  },
  "services.createFieldPublishPathPlaceholder": {
    message: "p. ej., dist",
    description: "Placeholder for the publish directory field",
  },
  "services.createFieldPublishPathHint": {
    message:
      "La carpeta con tu sitio compilado, relativa al directorio raíz (p. ej., dist, o . para el propio directorio raíz).",
    description: "Hint text under the publish directory field",
  },
  "services.createFieldPublishPathRequired": {
    message:
      "Obligatorio. Introduce el directorio de salida compilado que se sirve como raíz del sitio, relativo al directorio raíz (p. ej. dist, build, public, o . para el propio directorio raíz — la raíz del repositorio cuando el directorio raíz está en blanco).",
    description:
      "Hint under the publish directory field while it is blank: the field is required, the placeholder is only an example",
  },
  "services.createNoPublicUrlNote": {
    message: "Este tipo de servicio no tiene URL pública.",
    description:
      "Note shown for private/worker types that don't produce a public URL",
  },
  "services.createFieldEnvVarsTitle": {
    message: "Variables de entorno",
    description: "Section heading for env vars in the create wizard",
  },
  "services.createFieldEnvVarsAdd": {
    message: "Añadir variable",
    description: "Button to add an env var row in the create wizard",
  },
  "services.envRowKey": {
    message: "Clave de la variable {n}",
    description:
      "A create-flow variable row's key field (aria-label), named by its row as its Remove button is (w4/544)",
  },
  "services.envRowValue": {
    message: "Valor de la variable {n}",
    description:
      "A create-flow variable row's value field (aria-label), named by its row (w4/544)",
  },
  "services.envRowGenerate": {
    message: "Generar valor para la variable {n}",
    description:
      "A create-flow variable row's Generate button (aria-label), named by its row (w4/544)",
  },
  "services.envRowRemove": {
    message: "Quitar la variable {n}",
    description:
      "A create-flow variable row's Remove button (aria-label), named by its row (w4/544)",
  },
  "services.fileRowName": {
    message: "Nombre del archivo secreto {n}",
    description:
      "A create-flow secret file row's name field (aria-label), named by its row and starting with the dialog's visible \"File name\" label (w4/544)",
  },
  "services.fileRowContent": {
    message: "Contenido del archivo secreto {n}",
    description:
      "A create-flow secret file row's contents field (aria-label), named by its row (w4/544)",
  },
  "services.fileRowRemove": {
    message: "Quitar el archivo secreto {n}",
    description:
      "A create-flow secret file row's Remove button (aria-label), named by its row (w4/544)",
  },
  "services.createFieldEnvVarsKey": {
    message: "Clave",
    description: "Label for the env var key column in the create wizard",
  },
  "services.createFieldEnvVarsValue": {
    message: "Valor",
    description: "Label for the env var value column in the create wizard",
  },
  "services.createFieldEnvVarsKeyPlaceholder": {
    message: "KEY_NAME",
    description: "Placeholder for the env var key input in the create wizard",
  },
  "services.createFieldEnvVarsValuePlaceholder": {
    message: "valor",
    description: "Placeholder for the env var value input in the create wizard",
  },
  "services.createFieldEnvVarsKeyError": {
    message:
      "Usa hasta 253 letras, dígitos, puntos, guiones y guiones bajos; no '.' ni un nombre que empiece por '..'.",
    description:
      "Error shown when an env var key is invalid in the create wizard",
  },
  "services.createFieldSecretFilesTitle": {
    message: "Archivos secretos",
    description: "Section heading for secret files in the create wizard",
  },
  "services.createFieldSecretFilesHint": {
    message:
      "Se montan en modo de solo lectura en /etc/secrets desde el primer despliegue.",
    description: "Hint for create-time secret files",
  },
  "services.createFieldSecretFilesAdd": {
    message: "Añadir archivo secreto",
    description: "Button to add a create-time secret file",
  },
  "services.createFieldSecretFilesNamePlaceholder": {
    message: "credentials.json",
    description: "Placeholder for a secret file name",
  },
  "services.createFieldSecretFilesContentPlaceholder": {
    message: "Pega el contenido secreto",
    description: "Placeholder for secret file contents",
  },
  "services.createFieldSecretFilesNameError": {
    message:
      "Usa solo letras, dígitos, puntos, guiones y guiones bajos; no se permiten . ni ..",
    description: "Invalid secret file name error",
  },
  "services.createFieldEnvironmentTitle": {
    message: "Proyecto y entorno",
    description: "Create wizard grouping section title",
  },
  "services.createFieldProject": {
    message: "Proyecto",
    description: "Accessible label for the create project picker",
  },
  "services.createFieldProjectNone": {
    message: "Sin proyecto",
    description: "Unassigned option in the project picker",
  },
  "services.createFieldEnvironment": {
    message: "Entorno",
    description: "Accessible label for the create environment picker",
  },
  "services.createFieldEnvironmentNone": {
    message: "Sin entorno",
    description: "Unassigned option in the environment picker",
  },
  "services.createFieldEnvironmentHint": {
    message:
      "Al seleccionar un entorno, el servicio también se añade a su proyecto principal.",
    description: "Hint for create-time environment assignment",
  },
  "services.navEvents": {
    message: "Eventos",
    description: "Service-detail nav item (events tab)",
  },
  "services.navDeploys": {
    message: "Despliegues",
    description:
      "Service-detail nav item (dedicated deploy-history tab, w9/002)",
  },
  "services.navRuns": {
    message: "Ejecuciones",
    description:
      "Service-detail nav item for a cron job: its run history, Render's Runs page (w2/m360)",
  },
  "services.navDisk": {
    message: "Disco",
    description:
      "Service-detail nav item (persistent disk page); Render's label is singular",
  },
  "services.diskDescription": {
    message:
      "Conecta un SSD para conservar los datos del sistema de archivos de tu servicio entre despliegues. Los discos se cobran a $0.175/GB al mes.",
    description:
      "Disk tab card description; bex's rate is 30% below Render's $0.25",
  },
  "services.diskEmptyTitle": {
    message: "Ningún disco conectado",
    description: "Disk tab empty state title",
  },
  "services.diskEmptyBody": {
    message:
      "Solo se conservan, entre despliegues y reinicios, los archivos que están en la ruta de montaje del disco.",
    description: "Disk tab empty state body",
  },
  "services.diskPaidOnly": {
    message:
      "Los discos requieren un plan de cómputo de pago. Mejora el plan de este servicio para conectar uno.",
    description: "Shown instead of the add button on a free service",
  },
  "services.diskAddAction": {
    message: "Añadir disco",
    description: "Button that opens and submits the add-disk form",
  },
  "services.createDiskRemove": {
    message: "Quitar disco",
    description:
      "Create form Advanced: drops the drafted disk (Render's 'Remove disk', w4/m652)",
  },
  "services.diskLoadErrorTitle": {
    message: "No se pudo cargar el disco de este servicio",
    description: "Disk tab error state title",
  },
  "services.diskWarningsTitle": {
    message: "Ten en cuenta lo siguiente:",
    description: "Heading above the five add-disk warnings (Render's wording)",
  },
  "services.diskWarningZeroDowntime": {
    message:
      "Conectar un disco desactiva los despliegues sin tiempo de inactividad del servicio.",
    description: "Add-disk warning 1",
  },
  "services.diskWarningSingleInstance": {
    message:
      "Los servicios con un disco conectado no pueden escalar a varias instancias.",
    description: "Add-disk warning 2",
  },
  "services.diskWarningOnePerService": {
    message: "Puedes conectar un máximo de un disco por servicio.",
    description: "Add-disk warning 3",
  },
  "services.diskWarningMountPathOnly": {
    message:
      "Solo se conservan los archivos que están en la ruta de montaje de tu disco.",
    description: "Add-disk warning 4",
  },
  "services.diskWarningNoSharing": {
    message: "Otros servicios no pueden acceder al disco de este servicio.",
    description: "Add-disk warning 5",
  },
  "services.diskMountPathLabel": {
    message: "Ruta de montaje",
    description: "Add-disk form field label",
  },
  "services.diskMountPathHint": {
    message:
      "La ruta de montaje absoluta del disco. Solo se conservan entre despliegues los archivos que están en esta ruta. No puede ser el directorio raíz (/).",
    description: "Add-disk mount path helper text",
  },
  "services.diskMountPathRequired": {
    message: "Introduce una ruta de montaje.",
    description: "Client-side validation: empty mount path",
  },
  "services.diskMountPathAbsolute": {
    message: "La ruta de montaje debe ser absoluta (empezar por /).",
    description: "Client-side validation: relative mount path",
  },
  "services.diskMountPathNotRoot": {
    message:
      "La ruta de montaje no puede ser el directorio raíz ni terminar con una barra.",
    description: "Client-side validation: root or trailing slash",
  },
  "services.diskMountPathReserved": {
    message:
      "Esa ruta está reservada por la plataforma. Monta en su lugar un subdirectorio de ella.",
    description: "Client-side validation: reserved mount path",
  },
  "services.diskSizeLabel": {
    message: "Tamaño",
    description: "Add-disk size field label",
  },
  "services.diskSizeHint": {
    message:
      "Puedes aumentar el tamaño más adelante, pero no reducirlo. Te recomendamos empezar con el valor más bajo que sirva para tu caso de uso.",
    description: "Add-disk size helper text (Render's wording)",
  },
  "services.diskSizeRange": {
    message: "Introduce un número entero de GB entre {min} y {max}.",
    description:
      "Create form: the disk size is not a whole number in bex-api's range",
  },
  "services.diskSizeChip": {
    message: "{size} GB",
    description: "Size quick-select chip and size display",
  },
  "services.diskAddSuccess": {
    message: "Disco conectado. El servicio se está volviendo a desplegar.",
    description: "Toast after a disk is attached",
  },
  "services.diskGrowLabel": {
    message: "Aumentar tamaño (GB)",
    description: "Grow control label on an attached disk",
  },
  "services.diskGrowAction": {
    message: "Aumentar tamaño",
    description: "Grow button",
  },
  "services.diskGrowHint": {
    message:
      "Aumentar el tamaño no causa tiempo de inactividad, y el almacenamiento añadido está disponible en pocos segundos. Un disco nunca se puede reducir.",
    description: "Grow helper text in Render's wording",
  },
  "services.diskResizeSuccess": {
    message: "Tamaño del disco aumentado.",
    description: "Toast after a successful grow",
  },
  "services.diskDeleteAction": {
    message: "Eliminar disco",
    description: "Destructive action on an attached disk",
  },
  "services.diskDeleteTitle": {
    message: "¿Eliminar este disco?",
    description: "Delete confirmation title",
  },
  "services.diskDeleteWarning": {
    message:
      "Se perderán todos los datos del disco y el servicio perderá el acceso a él de inmediato. Sus snapshots también se eliminan. Esta acción no se puede deshacer.",
    description: "Delete confirmation body",
  },
  "services.diskDeleteConfirm": {
    message: "Eliminar disco",
    description: "Delete confirmation action",
  },
  "services.diskDeleteSuccess": {
    message: "Disco eliminado.",
    description: "Toast after a disk is deleted",
  },
  "services.diskSnapshotsTitle": {
    message: "Snapshots",
    description: "Snapshots card title",
  },
  "services.diskSnapshotsDescription": {
    message:
      "Se toma un snapshot de este disco cada día y se conserva durante al menos siete días.",
    description: "Snapshots card description",
  },
  "services.diskSnapshotsNotConfiguredTitle": {
    message: "Los snapshots no están configurados",
    description:
      "Snapshots card heading when the deployment has no snapshot object store configured (not a failure).",
  },
  "services.diskSnapshotsNotConfiguredBody": {
    message:
      "Este despliegue de bex no tiene configurado un almacén de objetos para los snapshots de disco, por lo que no se toma ninguno. Tus datos no se ven afectados: pide a un operador que los habilite.",
    description:
      "Explains that snapshots are unconfigured rather than broken, and that an operator enables them.",
  },
  "services.diskSnapshotsEmptyTitle": {
    message: "Aún no hay snapshots",
    description: "Snapshots empty state title",
  },
  "services.diskSnapshotsEmptyBody": {
    message: "El primer snapshot se toma en la próxima ejecución nocturna.",
    description: "Snapshots empty state body",
  },
  "services.diskSnapshotsFailedTitle": {
    message: "El último snapshot falló",
    description: "A disk's most recent nightly snapshot attempt failed",
  },
  "services.diskSnapshotsFailedReadBody": {
    message:
      "No se pudo leer una parte de este disco, por lo que no se tomó el snapshot y los cambios recientes no tienen copia de seguridad. Los snapshots existentes no se ven afectados. Si vuelve a ocurrir, contacta con soporte.",
    description: "Failed snapshot: the volume could not be read",
  },
  "services.diskSnapshotsFailedUploadBody": {
    message:
      "No se pudo guardar el snapshot en el almacenamiento, por lo que los cambios recientes no tienen copia de seguridad. Los snapshots existentes no se ven afectados; la próxima ejecución nocturna lo intentará de nuevo.",
    description: "Failed snapshot: the object store did not accept it",
  },
  "services.diskSnapshotsFailedConfigBody": {
    message:
      "El trabajo de snapshot no pudo iniciarse correctamente, por lo que los cambios recientes no tienen copia de seguridad. Los snapshots existentes no se ven afectados. Contacta con soporte.",
    description: "Failed snapshot: the job was misconfigured",
  },
  "services.diskSnapshotsFailedOtherBody": {
    message:
      "El último intento de snapshot falló, por lo que los cambios recientes no tienen copia de seguridad. Los snapshots existentes no se ven afectados. Si vuelve a ocurrir, contacta con soporte.",
    description: "Failed snapshot: an unrecognized stage",
  },
  "services.diskSnapshotsOverdueTitle": {
    message: "Ningún snapshot reciente",
    description: "A disk's nightly snapshot did not arrive when expected",
  },
  "services.diskSnapshotsOverdueBody": {
    message:
      "El snapshot nocturno previsto para {dueBy} no se ha tomado, por lo que los cambios recientes en este disco no tienen copia de seguridad. Los snapshots existentes no se ven afectados. Si esto continúa, contacta con soporte.",
    description:
      "Explains an overdue nightly snapshot; {dueBy} is a local date/time",
  },
  "services.diskSnapshotsNotRunningTitle": {
    message: "Los snapshots no se están ejecutando",
    description:
      "The platform could not schedule this disk's nightly snapshots",
  },
  "services.diskSnapshotsNotRunningBody": {
    message:
      "bex no pudo programar los snapshots nocturnos de este disco, por lo que los cambios nuevos no tienen copia de seguridad. Los snapshots existentes no se ven afectados. Contacta con soporte.",
    description:
      "Explains that nightly snapshots cannot be scheduled and whom to contact",
  },
  "services.diskSnapshotsUnavailableTitle": {
    message: "Los snapshots no están disponibles",
    description: "Snapshots error state title",
  },
  "services.diskRestoreDatabaseWarning": {
    message:
      "No restaures un disco para recuperar una base de datos que se ejecuta en él: restaurar archivos bajo una base de datos en ejecución puede dejarla dañada. Usa en su lugar la copia de seguridad y la restauración de esa base de datos.",
    description:
      "Render's database-recovery warning, carried verbatim in meaning",
  },
  "services.diskRestoreAction": {
    message: "Restaurar",
    description: "Restore button on a snapshot row",
  },
  "services.diskRestoreTitle": {
    message: "¿Restaurar este snapshot?",
    description: "Restore confirmation title",
  },
  "services.diskRestoreWarning": {
    message:
      "El servicio se detiene, el disco se reemplaza por este snapshot y el servicio se reinicia. Todo lo escrito después del snapshot se pierde. Esta acción no se puede deshacer.",
    description: "Restore confirmation body",
  },
  "services.diskRestoreConfirm": {
    message: "Restaurar disco",
    description: "Restore confirmation action",
  },
  "services.diskUnsupportedType": {
    message:
      "Los discos persistentes necesitan una instancia de larga duración donde montarse, por lo que están disponibles en Web Services, Private Services y Background Workers.",
    description:
      "Empty-state body on the Disk tab for a service type that cannot have a disk.",
  },
  "services.diskSudoPrompt": {
    message: "Para continuar, escribe {phrase}",
    description:
      "Label above the typed-confirmation input for an irreversible disk action.",
  },
  "services.diskConfigTitle": {
    message: "Configuración del disco",
    description:
      "Heading of the card holding the disk's mount path and size settings.",
  },
  "services.diskConfigDescription": {
    message: "Configura la ruta de montaje y el tamaño de este disco.",
    description: "Subtitle of the disk configuration card.",
  },
  "services.diskSizeUnit": {
    message: "GB",
    description: "Unit suffix shown beside the disk size input.",
  },
  "services.diskEditAction": {
    message: "Editar",
    description: "Button that unlocks the disk size field for editing.",
  },
  "services.diskProvisionedLabel": {
    message: "Aprovisionado: {size} GB",
    description:
      "Provisioned disk size shown beside the usage chart heading; this is the size billing meters.",
  },
  "services.diskDeleteCardTitle": {
    message: "Eliminar disco",
    description:
      "Heading of the delete-disk card. Distinct from diskDeleteTitle, which is the confirmation dialog's question.",
  },
  "services.diskDeleteCardDescription": {
    message:
      "El disco se desconectará de este servicio y todos sus datos se eliminarán de forma permanente.",
    description: "Subtitle of the delete-disk card, stating what is lost.",
  },
  "services.diskUsageTitle": {
    message: "Uso del disco",
    description: "Heading of the disk usage chart on a service's Disk tab.",
  },
  "services.diskUsageDescription": {
    message:
      "Bytes escritos en el volumen montado, frente a su capacidad aprovisionada. La facturación mide el tamaño aprovisionado, no los bytes usados.",
    description:
      "Subtitle of the disk usage chart, stating that billing uses provisioned size.",
  },
  "services.diskRestoreStarted": {
    message: "Restaurando el disco. El servicio se detiene hasta que termine.",
    description: "Toast after a restore is requested",
  },
  "services.navShell": {
    message: "Shell",
    description: "Service-detail nav item (running-instance SSH page)",
  },
  "services.navCompute": {
    message: "Cómputo",
    description:
      "Service sidebar link to the Compute page: the compute plan, manual scaling and autoscaling, as on Render (w6/m483)",
  },
  "services.eventsCount_one": {
    message: "{count} evento",
    description:
      "Accessible label for the number of visible service events (singular)",
  },
  "services.eventsCount_many": {
    message: "{count} de eventos",
    description:
      "Accessible label for the number of visible service events (plural)",
  },
  "services.eventsCount_other": {
    message: "{count} eventos",
    description:
      "Accessible label for the number of visible service events (plural)",
  },
  "services.eventsCountAtLeast_one": {
    message: "{count}+ eventos",
    description:
      "Accessible label for the event count badge while older events remain unread (singular)",
  },
  "services.eventsCountAtLeast_many": {
    message: "{count}+ eventos",
    description:
      "Accessible label for the event count badge while older events remain unread (plural)",
  },
  "services.eventsCountAtLeast_other": {
    message: "{count}+ eventos",
    description:
      "Accessible label for the event count badge while older events remain unread (plural)",
  },
  "services.eventsEmptyTitle": {
    message: "Aún no hay eventos",
    description: "Events tab empty-state title",
  },
  "services.eventsEmpty": {
    message: "Los despliegues y cambios del servicio aparecerán aquí.",
    description: "Events tab empty-state description",
  },
  "services.eventsErrorTitle": {
    message: "Los eventos no están disponibles",
    description: "Events tab query-error title",
  },
  "services.eventsErrorDescription": {
    message: "No pudimos cargar los eventos de este servicio.",
    description: "Events tab query-error description",
  },
  "services.eventsRetry": {
    message: "Reintentar",
    description: "Events tab query-error retry button",
  },
  "services.eventsLoadOlder": {
    message: "Cargar eventos anteriores",
    description: "Button that loads the next page or earlier event window",
  },
  "services.eventsLoadingOlder": {
    message: "Cargando eventos anteriores…",
    description: "Busy label while an older service-event page is loading",
  },
  "services.eventsHistoryEnd": {
    message: "Has llegado al principio del historial de este servicio.",
    description: "Terminal label after all retained service events are loaded",
  },
  "services.eventsFilter": {
    message: "Filtrar eventos",
    description: "Button opening the service event-type filter",
  },
  "services.eventsFilterSelected": {
    message: "Tipos de evento: {count}",
    description: "Event filter button with selected-type count",
  },
  "services.eventsFilterSearch": {
    message: "Buscar tipos de evento",
    description: "Search input for service event types",
  },
  "services.eventsFilterShowAll": {
    message: "Mostrar todos los eventos",
    description:
      "Events tab: button in the filtered-to-nothing state that shows every event type again",
  },
  "services.eventsShownOf": {
    message: "Mostrando {shown} de {total}",
    description:
      "Events tab: count over the feed while a filter hides some rows; {total} is the loaded count, '20+' while older events remain",
  },
  "services.eventsFilterAll": {
    message: "Todos los eventos",
    description: "Master service event filter option",
  },
  "services.eventsFilterNoMatches": {
    message: "Ningún tipo de evento coincide con tu búsqueda.",
    description: "Empty search result in the event filter",
  },
  "services.eventsFilterEmptyTitle": {
    message: "Ningún evento coincide",
    description: "Title when loaded history has no selected event types",
  },
  "services.eventsFilterEmpty": {
    message: "Elige más tipos de evento o carga un historial más antiguo.",
    description: "Body when loaded history has no selected event types",
  },
  "services.eventsFilterNoneTitle": {
    message: "No hay tipos de evento seleccionados",
    description:
      "Events tab: title when the filter has every event type deselected",
  },
  "services.eventsFilterNone": {
    message: "Elige al menos un tipo para ver eventos.",
    description:
      "Events tab: body when the filter has every event type deselected",
  },
  "services.eventsFilterGroup.deploy": {
    message: "Despliegue",
    description: "Deploy event filter group",
  },
  "services.eventsFilterGroup.serviceStatus": {
    message: "Estado del servicio",
    description: "Service-status event filter group",
  },
  "services.eventsFilterGroup.scaling": {
    message: "Escalado",
    description: "Scaling event filter group",
  },
  "services.eventsFilterGroup.maintenanceMode": {
    message: "Modo de mantenimiento",
    description: "Maintenance-mode event filter group",
  },
  "services.eventsFilterGroup.edgeCache": {
    message: "Caché perimetral",
    description: "Edge-cache event filter group",
  },
  "services.eventsFilterGroup.configuration": {
    message: "Configuración",
    description: "bex configuration event filter group",
  },
  "services.eventsFilterGroup.other": {
    message: "Otros",
    description:
      "Event filter group holding types the API returned that the dashboard catalog does not know yet",
  },
  "services.eventsActor": {
    message: "por {actor}",
    description: "Actor attribution shown on a service event",
  },
  "services.eventsDeployReference": {
    message: "Despliegue {id}",
    description: "Deploy identifier shown on a deploy activity row",
  },
  "services.eventsTriggerRollback": {
    message: "Reversión",
    description: "Deploy event trigger: rollback",
  },
  "services.eventsTriggerFirstBuild": {
    message: "Primer despliegue",
    description: "Deploy event trigger: the service's first deploy",
  },
  "services.eventsTriggerManual": {
    message: "Despliegue manual",
    description: "Deploy event trigger: manual",
  },
  "services.eventsTriggerEnvUpdated": {
    message: "Cambio de configuración",
    description:
      "Deploy event trigger: an environment change (env var, secret file, or env-group edit); a Settings save no longer sets envUpdated (w8/m135)",
  },
  "services.eventsTriggerDeployHook": {
    message: "Hook de despliegue",
    description:
      "Deploy event trigger: a POST to the service's secret deploy-hook URL",
  },
  "services.eventsTriggerClearCache": {
    message: "Caché borrada",
    description: "Deploy event trigger: build-cache clear",
  },
  "services.eventsTriggerDeployedByRender": {
    message: "Despliegue de la plataforma",
    description: "Deploy event trigger: platform initiated",
  },
  "services.eventsTypeDeployStarted": {
    message: "Despliegue iniciado",
    description: "Service activity type: deploy started",
  },
  "services.eventsTypeDeployFinished": {
    message: "Despliegue finalizado",
    description: "Service activity type: deploy finished",
  },
  "services.eventsTypeZeroDowntimeRedeployStarted": {
    message: "Redespliegue sin interrupciones iniciado",
    description:
      "Service activity type: a Bex-initiated zero-downtime deploy started for a service",
  },
  "services.eventsTypeZeroDowntimeRedeployEnded": {
    message: "Redespliegue sin interrupciones finalizado",
    description:
      "Service activity type: a Bex-initiated zero-downtime deploy completed for a service",
  },
  "services.eventsRedeployTrigger.node_replacement": {
    message: "Nodo reemplazado",
    description:
      "zero_downtime_redeploy_started trigger: drain-guard rolled the service off a node being replaced",
  },
  "services.eventsRedeployTrigger.node_consolidation": {
    message: "Nodo eliminado por autoescalado",
    description:
      "zero_downtime_redeploy_started trigger: drain-guard rolled the service off a node marked for Cluster Autoscaler scale-down",
  },
  "services.eventsTypePipelineMinutesExhausted": {
    message: "Minutos de pipeline agotados",
    description:
      "Service activity type: a build refused because the workspace used its pipeline minutes",
  },
  "services.eventsTypeEdgeCacheEnabled": {
    message: "Caché perimetral activada",
    description: "Service activity type: edge caching turned on",
  },
  "services.eventsTypeEdgeCacheDisabled": {
    message: "Caché perimetral desactivada",
    description: "Service activity type: edge caching turned off",
  },
  "services.eventsTypeEdgeCachePurged": {
    message: "Caché perimetral purgada",
    description: "Service activity type: edge cache purged",
  },
  "services.eventsTypeImagePullFailed": {
    message: "Falló la descarga de la imagen",
    description: "Service activity type: image pull failed",
  },
  "services.eventsTypeArtifactFetchFailed": {
    message: "Error al obtener el artefacto",
    description:
      "Service activity type: a build published by a build source could not be fetched",
  },
  "services.eventsTypeArtifactSourceChanged": {
    message: "Origen de compilación cambiado",
    description:
      "Service activity type: the service's linked build source changed",
  },
  "services.eventsArtifactSourceChanged": {
    message: "Origen de compilación {from} → {to}",
    description:
      "Detail line on artifact_source_changed when switching between two build sources",
  },
  "services.eventsArtifactSourceLinked": {
    message: "Vinculado al origen de compilación {to}",
    description:
      "Detail line on artifact_source_changed when newly linked to a build source",
  },
  "services.eventsArtifactSourceUnlinked": {
    message: "Desvinculado del origen de compilación {from}",
    description:
      "Detail line on artifact_source_changed when detached from a build source",
  },
  "services.eventsBuildReference": {
    message: "Compilación {id}",
    description:
      "Build identifier shown on an artifact_fetch_failed activity row",
  },
  "services.eventsTypeSuspended": {
    message: "Suspendido",
    description: "Service activity type: service suspended",
  },
  "services.eventsTypeResumed": {
    message: "Reanudado",
    description: "Service activity type: service resumed",
  },
  "services.eventsTypeRestarted": {
    message: "Servidor reiniciado",
    description: "Service activity type: service restarted",
  },
  "services.eventsTypeInstanceFailed": {
    message: "Falló la instancia",
    description: "Service activity type: running instance became unhealthy",
  },
  "services.eventsTypeDiskUsageHigh": {
    message: "Uso de disco alto",
    description:
      "Service activity type: a service disk is over 80% full (service_disk_usage_high)",
  },
  "services.eventsTypeDiskUsageRecovered": {
    message: "Uso de disco recuperado",
    description:
      "Service activity type: a service disk dropped back below 80% (service_disk_usage_recovered)",
  },
  "services.eventsDiskUsage": {
    message:
      "Disco {disk} en {mountPath}: {usage}% usado (umbral {threshold}%)",
    description: "Disk usage event detail line",
  },
  "services.eventsTypeHardwareFailure": {
    message: "Fallo de hardware",
    description:
      "Service activity type: an instance was lost with the machine it ran on",
  },
  "services.eventsTypeServiceSuspended": {
    message: "Servicio suspendido",
    description: "Service activity type: suspension converged",
  },
  "services.eventsTypeServiceResumed": {
    message: "Servicio reanudado",
    description: "Service activity type: resume converged",
  },
  "services.eventsTypeServiceHibernated": {
    message: "El servicio se durmió",
    description:
      "Service activity type: free-tier idle auto-hibernation — deliberately worded apart from a user-driven suspension",
  },
  "services.eventsTypeServiceWoken": {
    message: "El servicio se despertó",
    description:
      "Service activity type: a sleeping free-tier service woke on an incoming request",
  },
  "services.eventsTypeServiceRecovered": {
    message: "Servicio recuperado",
    description: "Service activity type: unhealthy service recovered",
  },
  "services.eventsTypePlanChanged": {
    message: "Tipo de instancia cambiado",
    description: "Service activity type: plan changed",
  },
  "services.eventsTypeInstanceCountChanged": {
    message: "Número de instancias cambiado",
    description: "Service activity type: manual scale",
  },
  "services.eventsTypeAutoscalingChanged": {
    message: "Configuración de escalado automático cambiada",
    description: "Service activity type: autoscaling configuration changed",
  },
  "services.eventsTypeAutoscalingStarted": {
    message: "Escalado automático iniciado",
    description: "Service activity type: autoscaler began a replica change",
  },
  "services.eventsTypeAutoscalingEnded": {
    message: "Escalado automático finalizado",
    description: "Service activity type: autoscaler replica change converged",
  },
  "services.eventsTypeBranchChanged": {
    message: "Rama cambiada",
    description: "Service activity type: tracked Git branch changed",
  },
  "services.eventsTypeCommitIgnored": {
    message: "Commit ignorado",
    description: "Service activity type: matching Git push did not deploy",
  },
  "services.eventsTypeJobStarted": {
    message: "Job iniciado",
    description: "Service activity type: one-off job started",
  },
  "services.eventsTypeJobCanceled": {
    message: "Job cancelado",
    description: "Service activity type: one-off job canceled",
  },
  "services.eventsTypeJobRunEnded": {
    message: "Job finalizado",
    description: "Service activity type: one-off job finished",
  },
  "services.eventsTypeBuildStarted": {
    message: "Build iniciado",
    description: "Service activity type: image build started",
  },
  "services.eventsTypeBuildEnded": {
    message: "Build finalizado",
    description: "Service activity type: image build finished",
  },
  "services.eventsTypePreDeployStarted": {
    message: "Predespliegue iniciado",
    description: "Service activity type: pre-deploy command started",
  },
  "services.eventsTypePreDeployEnded": {
    message: "Predespliegue finalizado",
    description: "Service activity type: pre-deploy command finished",
  },
  "services.eventsTypeBranchDeleted": {
    message: "Rama eliminada",
    description: "Service activity type: tracked Git branch deleted",
  },
  "services.eventsTypeMaintenanceModeChanged": {
    message: "Configuración del modo de mantenimiento cambiada",
    description: "Service activity type: maintenance mode setting changed",
  },
  "services.eventsTypeMaintenanceModeUriUpdated": {
    message: "URI del modo de mantenimiento actualizada",
    description: "Service activity type: maintenance mode URI changed",
  },
  "services.eventsInstanceReference": {
    message: "Instancia {id}",
    description: "Instance identifier shown on a service event",
  },
  "services.eventsReplicaChange": {
    message: "Instancias {from} → {to}",
    description:
      "Instance count from/to summary: an autoscaling start/end's replicas or an instance_count_changed event's counts",
  },
  "services.eventsAutoscalingChange": {
    message: "Escalado automático {from} → {to}",
    description:
      "autoscaling_config_changed detail line; from/to are services.eventsAutoscalingOff or services.eventsAutoscalingRange (w4/568)",
  },
  "services.eventsAutoscalingOff": {
    message: "Desactivado",
    description: "One side of an autoscaling change: autoscaling off (w4/568)",
  },
  "services.eventsAutoscalingRange": {
    message: "{min}–{max} instancias",
    description:
      "One side of an autoscaling change: its minimum–maximum instances (w4/568)",
  },
  "services.eventsAutoscalingRangeCPU": {
    message: "{min}–{max} instancias, CPU {cpu}%",
    description:
      "One side of an autoscaling change: its instance range and CPU target (w4/m335)",
  },
  "services.eventsAutoscalingRangeMemory": {
    message: "{min}–{max} instancias, memoria {memory}%",
    description:
      "One side of an autoscaling change: its instance range and memory target (w4/m335)",
  },
  "services.eventsAutoscalingRangeCPUMemory": {
    message: "{min}–{max} instancias, CPU {cpu}%, memoria {memory}%",
    description:
      "One side of an autoscaling change: its instance range and both targets (w4/m335)",
  },
  "services.eventsPlanChange": {
    message: "Plan de cómputo {from} → {to}",
    description:
      "plan_changed detail line, by the compute plans' names (w4/568, w6/m483)",
  },
  "services.eventsBranchChange": {
    message: "Rama {from} → {to}",
    description: "Tracked branch from/to summary",
  },
  "services.eventsAutoDeployDisabledReason.setting_change": {
    message: "Desactivado en la configuración",
    description:
      "auto_deploy_disabled reason: the auto-deploy setting was turned off (w4/m431)",
  },
  "services.eventsAutoDeployDisabledReason.rollback": {
    message:
      "Desactivado por un rollback, para que los pushes posteriores no lo reemplacen",
    description:
      "auto_deploy_disabled reason: a dashboard rollback turned auto-deploy off (w4/m431)",
  },
  "services.eventsAutoDeployDisabledReason.manual_deploy": {
    message:
      "Desactivado por el deploy de un commit específico, para que los pushes posteriores no lo reemplacen",
    description:
      "auto_deploy_disabled reason: a deploy of a specific commit from the dashboard or a deploy hook turned auto-deploy off (w4/m431)",
  },
  "services.eventsBranchDeleted": {
    message: "Rama {branch} eliminada — despliegue automático desactivado",
    description: "Deleted tracked branch summary",
  },
  "services.eventsStatus.succeeded": {
    message: "Correcto",
    description: "Lifecycle-step outcome badge: succeeded",
  },
  "services.eventsStatus.failed": {
    message: "Fallido",
    description: "Lifecycle-step outcome badge: failed",
  },
  "services.eventsStatus.canceled": {
    message: "Cancelado",
    description: "Lifecycle-step outcome badge: canceled",
  },
  "services.eventsReason.superseded": {
    message: "Un despliegue más reciente reemplazó este despliegue.",
    description:
      "Deploy/build ended reason: a newer deploy superseded this one",
  },
  "services.eventsReason.image_pull_backoff": {
    message: "El entorno de ejecución no pudo descargar la imagen configurada.",
    description: "Bounded public image-pull failure reason",
  },
  "services.eventsReason.readiness_failed": {
    message: "Una instancia en ejecución no superó su comprobación de estado.",
    description:
      "Bounded public readiness failure reason; names the health check, as Settings and REST do (w4/m327)",
  },
  "services.eventsReason.root_directory": {
    message:
      "Ningún archivo modificado coincidió con el directorio raíz del servicio.",
    description: "Commit ignored because of root-directory scoping",
  },
  "services.eventsReason.build_filter": {
    message:
      "Los archivos modificados no superaron los filtros de build del servicio.",
    description: "Commit ignored because of build filters",
  },
  "services.eventsReason.skip_phrase": {
    message: "El mensaje del commit pidió omitir este despliegue.",
    description: "Commit ignored because of a Render skip phrase",
  },
  "services.eventsReason.non_zero_exit": {
    message: "La ejecución terminó con el estado {exitCode}.",
    description:
      "Cron run failure reason: the container exited non-zero (w4/m114)",
  },
  "services.eventsReason.oom_killed": {
    message: "La ejecución se quedó sin memoria y se detuvo.",
    description:
      "Cron run failure reason: the container was OOM-killed (w4/m114)",
  },
  "services.eventsReason.instance.non_zero_exit": {
    message:
      "El proceso de la instancia terminó con el estado {exitCode} y se reinició.",
    description:
      "Instance failed reason for a running service: its process crashed (w8/m158)",
  },
  "services.eventsReason.instance.oom_killed": {
    message:
      "La instancia se quedó sin memoria (límite de {memory}) y se reinició.",
    description:
      "Instance failed reason for a running service: killed at its plan's memory limit (w8/m158)",
  },
  "services.eventsReason.instance.readiness_failed_result": {
    message:
      "Una instancia en ejecución no superó su comprobación de estado: {result}.",
    description:
      'Instance failed reason for a running service whose health check failed, naming what the check got back (w6/m634); {result} is Render\'s English message ("HTTP health check failed with status code 503"), shown verbatim in every language as Render shows it',
  },
  "services.eventsReason.instance.oom_killed_no_limit": {
    message: "La instancia se quedó sin memoria y se reinició.",
    description:
      "Instance failed reason for a running service: killed for memory, limit unknown (w8/m158)",
  },
  "services.eventsReason.early_exit": {
    message:
      "El proceso de la instancia terminó y se reinició. Se espera que un servicio siga en ejecución.",
    description:
      "Instance failed reason for a running service: its process ended with status 0 (w8/m158)",
  },
  "services.eventsReason.evicted": {
    message: "La ejecución fue expulsada de su nodo antes de terminar.",
    description: "Cron run failure reason: the run's pod was evicted (w4/m114)",
  },
  "services.eventsReason.timed_out": {
    message: "La ejecución alcanzó el límite de 12 horas y se detuvo.",
    description:
      "Cron run failure reason: the run exceeded its deadline (w4/m114)",
  },
  "services.eventsTypeCronRunStarted": {
    message: "Ejecución de cron iniciada",
    description: "Service activity type: cron run started",
  },
  "services.eventsTypeCronRunFinished": {
    message: "Ejecución de cron finalizada",
    description: "Service activity type: cron run finished",
  },
  "services.eventsTypeEnvVarsChanged": {
    message: "Variables de entorno cambiadas",
    description: "Service activity type: environment variables changed",
  },
  "services.eventsTypeEnvironmentChanged": {
    message: "Configuración del entorno cambiada",
    description:
      "Service activity type: environment variables and secret files changed",
  },
  "services.eventsTypeServiceMoved": {
    message: "Servicio movido",
    description:
      "Service activity type: the service was reassigned between projects or environments (service_moved) — distinct from the env-var fact above",
  },
  "services.eventsProjectChange": {
    message: "Proyecto {from} → {to}",
    description:
      "service_moved detail line: previous and new project name (its id once it no longer exists), or the placement-none placeholder (w4/561)",
  },
  "services.eventsEnvironmentChange": {
    message: "Entorno {from} → {to}",
    description:
      "service_moved detail line: previous and new environment name (its id once it no longer exists), or the placement-none placeholder (w4/561)",
  },
  "services.eventsPlacementNone": {
    message: "ninguno",
    description:
      "Placeholder for an absent side of a service_moved event (assigned from / removed to nothing) or of a plan_changed event (no plan before)",
  },
  "services.eventsTypeEnvGroupLinked": {
    message: "Grupo de entorno vinculado",
    description: "Service activity type: environment group linked",
  },
  "services.eventsTypeEnvGroupUnlinked": {
    message: "Grupo de entorno desvinculado",
    description: "Service activity type: environment group unlinked",
  },
  "services.eventsEnvGroupLinkedNamed": {
    message: "Grupo de entorno {name} vinculado",
    description:
      "Service activity title for env_group_linked naming the group, by its name at the time (w4/563)",
  },
  "services.eventsEnvGroupUnlinkedNamed": {
    message: "Grupo de entorno {name} desvinculado",
    description:
      "Service activity title for env_group_unlinked naming the group, by its name at the time (w4/563)",
  },
  "services.eventsCustomDomainAddedNamed": {
    message: "Dominio personalizado {name} añadido",
    description:
      "Service activity title for custom_domain_added naming the domain's hostname (w4/m337)",
  },
  "services.eventsCustomDomainRemovedNamed": {
    message: "Dominio personalizado {name} eliminado",
    description:
      "Service activity title for custom_domain_removed naming the domain's hostname (w4/m337)",
  },
  "services.eventsCustomDomainVerifiedNamed": {
    message: "Dominio personalizado {name} verificado",
    description:
      "Service activity title for custom_domain_verified naming the domain's hostname (w4/m337)",
  },
  "services.eventsDisplayNameChange": {
    message: "Nombre {from} → {to}",
    description:
      "display_name_changed detail line: the service's name before and after the rename (w4/m337)",
  },
  "services.eventsTypeAutoDeployChanged": {
    message: "Despliegue automático actualizado",
    description: "Service activity type: auto-deploy setting changed",
  },
  "services.eventsTypeIdleTimeoutChanged": {
    message: "Tiempo de inactividad actualizado",
    description: "Service activity type: idle timeout changed",
  },
  "services.eventsTypeDisplayNameChanged": {
    message: "Nombre visible cambiado",
    description: "Service activity type: display name changed",
  },
  "services.eventsTypeCustomDomainAdded": {
    message: "Dominio personalizado añadido",
    description: "Service activity type: custom domain added",
  },
  "services.eventsTypeCustomDomainRemoved": {
    message: "Dominio personalizado eliminado",
    description: "Service activity type: custom domain removed",
  },
  "services.eventsTypeCustomDomainVerified": {
    message: "Dominio personalizado verificado",
    description: "Service activity type: custom domain verified",
  },
  "services.eventsTypeDiskCreated": {
    message: "Disco creado",
    description: "Service activity type: persistent disk created",
  },
  "services.eventsTypeDiskUpdated": {
    message: "Disco actualizado",
    description: "Service activity type: persistent disk updated",
  },
  "services.eventsTypeDiskDeleted": {
    message: "Disco eliminado",
    description: "Service activity type: persistent disk deleted",
  },
  "services.eventsTypeDiskSnapshotRestored": {
    message: "Snapshot de disco restaurado",
    description:
      "Service activity type: persistent disk restored from a snapshot",
  },
  "services.eventsTypeNotificationsChanged": {
    message: "Notificaciones de fallos actualizadas",
    description: "Service activity type: failure notification setting changed",
  },
  "services.eventsTypeSubdomainPolicyChanged": {
    message: "Subdominio de la plataforma actualizado",
    description: "Service activity type: platform subdomain policy changed",
  },
  "services.eventsTypeStaticSiteChanged": {
    message: "Configuración del sitio estático cambiada",
    description: "Service activity type: static-site configuration changed",
  },
  "services.eventsFilterType.autoDeployEnabled": {
    message: "Despliegue automático activado",
    description: "Events filter option for the auto deploy enabled event type",
  },
  "services.eventsFilterType.autoDeployDisabled": {
    message: "Despliegue automático desactivado",
    description: "Events filter option for the auto deploy disabled event type",
  },
  "services.eventsFilterType.autoDeployChanged": {
    message: "Despliegue automático actualizado (heredado)",
    description: "Events filter option for the auto deploy changed event type",
  },
  "services.eventsFilterType.publishPathChanged": {
    message: "Directorio de publicación cambiado",
    description: "Events filter option for the publish path changed event type",
  },
  "services.eventsFilterType.routesChanged": {
    message: "Redirecciones y reescrituras cambiadas",
    description: "Events filter option for the routes changed event type",
  },
  "services.eventsFilterType.headersChanged": {
    message: "Encabezados cambiados",
    description: "Events filter option for the headers changed event type",
  },
  "services.eventsFilterType.rootDirectoryChanged": {
    message: "Directorio raíz cambiado",
    description:
      "Events filter option for the root directory changed event type",
  },
  "services.eventsFilterType.dockerfilePathChanged": {
    message: "Ruta del Dockerfile cambiada",
    description:
      "Events filter option for the dockerfile path changed event type",
  },
  "services.eventsFilterType.dockerContextChanged": {
    message: "Contexto de build de Docker cambiado",
    description:
      "Events filter option for the docker context changed event type (w8/m128)",
  },
  "services.eventsFilterType.buildFilterChanged": {
    message: "Filtro de build cambiado",
    description: "Events filter option for the build filter changed event type",
  },
  "services.eventsFilterType.commandsChanged": {
    message: "Comandos cambiados",
    description: "Events filter option for the commands changed event type",
  },
  "services.eventsFilterType.sourceChanged": {
    message: "Origen cambiado",
    description: "Events filter option for the source changed event type",
  },
  "services.eventsFilterType.preDeployCommandChanged": {
    message: "Comando previo al despliegue cambiado",
    description:
      "Events filter option for the pre deploy command changed event type",
  },
  "services.eventsFilterType.maxShutdownDelayChanged": {
    message: "Retraso máximo de apagado cambiado",
    description:
      "Events filter option for the max shutdown delay changed event type",
  },
  "services.eventsFilterType.deployHookRegenerated": {
    message: "Hook de despliegue regenerado",
    description:
      "Events filter option for the deploy hook regenerated event type",
  },
  "services.eventsTypeBuildSettingsChanged": {
    message: "Configuración de build y despliegue cambiada",
    description: "Service activity type: build or deploy configuration changed",
  },
  "services.eventsTypePortChanged": {
    message: "Puerto cambiado",
    description: "Service activity type: the service's listening port changed",
  },
  "services.eventsTypeServiceChanged": {
    message: "Configuración del servicio cambiada",
    description: "Fallback service activity type",
  },
  "services.eventsManualDeploy": {
    message: "Despliegue manual",
    description: "Button to trigger a new deploy",
  },
  "services.deployMenuLatestCommit": {
    message: "Desplegar el último commit",
    description:
      "Manual Deploy dropdown item, repo-backed service: rebuild and redeploy from the branch's HEAD",
  },
  "services.deployMenuLatestImage": {
    message: "Desplegar la última imagen",
    description:
      "Manual Deploy dropdown item, image-backed service (no repo to rebuild from)",
  },
  "services.deployMenuClearCache": {
    message: "Borrar la caché de build y desplegar",
    description:
      "Manual Deploy dropdown item (Render parity): rebuild without importing prior build-cache layers when registry caching is enabled; still exports a fresh cache for later deploys",
  },
  "services.deployMenuRestart": {
    message: "Reiniciar servicio",
    description:
      "Manual Deploy dropdown item: restart on the commit or image that is running; never deploys newer commits",
  },
  "services.deployMenuSpecificCommit": {
    message: "Desplegar un commit concreto",
    description:
      "Manual Deploy dropdown item, repo-backed service (Render parity): deploy a commit by SHA and turn auto-deploy off",
  },
  "services.deployCommitTitle": {
    message: "Desplegar un commit concreto",
    description: "Title of the dialog that deploys a commit by SHA",
  },
  "services.deployCommitBody": {
    message:
      "Despliega el commit que introduzcas y desactiva el despliegue automático, para que los push posteriores no lo reemplacen. Puedes volver a activar el despliegue automático en la configuración.",
    description:
      "Body of the deploy-a-specific-commit dialog. Render's dashboard disables automatic deploys when a specific commit is deployed",
  },
  "services.deployCommitLabel": {
    message: "SHA del commit",
    description:
      "Label of the commit SHA input in the deploy-a-specific-commit dialog",
  },
  "services.deployCommitInvalid": {
    message: "Introduce un SHA de commit: de 7 a 40 caracteres hexadecimales.",
    description: "Inline error when the entered commit SHA is malformed",
  },
  "services.deployCommitConfirm": {
    message: "Desplegar commit",
    description: "Confirm button of the deploy-a-specific-commit dialog",
  },
  "services.restartServiceSuccess": {
    message: "Reinicio iniciado en la versión en ejecución.",
    description: "Toast after the header's Restart service is accepted",
  },
  "services.restartServiceError": {
    message: "No se pudo reiniciar el servicio.",
    description:
      "Toast when the header's Restart service fails and the server gave no reason",
  },
  "services.eventsCancelDeploy": {
    message: "Cancelar",
    description: "Button to cancel an in-progress deploy",
  },
  "services.eventsCancelConfirmTitle": {
    message: "¿Cancelar este despliegue?",
    description: "Cancel deploy confirm dialog title",
  },
  "services.eventsCancelConfirmBody": {
    message:
      "Se detendrá el despliegue en curso. Si un despliegue anterior se completó correctamente, sigue activo; si no, no habrá nada sirviéndose. Los cambios que guardaste siguen guardados y se publicarán con el próximo despliegue.",
    description:
      "Cancel deploy confirm dialog body when it is unknown whether an earlier deploy is live, so both outcomes are stated (w4/334)",
  },
  "services.eventsCancelConfirmBodyFirstDeploy": {
    message:
      "Se detendrá el despliegue en curso. Este es el primer despliegue del servicio, por lo que aún no hay ninguna versión activa: al cancelarlo, el servicio quedará sin nada que servir.",
    description:
      "Cancel deploy confirm dialog body — the service's first deploy, nothing is serving yet (w4/103)",
  },
  "services.eventsRollback": {
    message: "Revertir",
    description: "Button to roll back to a specific deploy",
  },
  "services.eventsRollbackAria": {
    message: "Revertir a {target}",
    description:
      "Accessible name of a deploy row's Rollback control, naming the deploy it restores (w4/m141)",
  },
  "services.eventsCancelDeployAria": {
    message: "Cancelar el despliegue {target}",
    description:
      "Accessible name of a deploy row's Cancel control, naming the deploy (w4/m141)",
  },
  "services.eventsConfirmProceed": {
    message: "Continuar",
    description: "Confirm dialog proceed button",
  },
  "services.eventsConfirmCancel": {
    message: "Volver",
    description: "Confirm dialog cancel button",
  },
  "services.triggerDeploySuccess": {
    message: "Despliegue iniciado.",
    description: "Toast after triggerDeploy succeeds",
  },
  "services.triggerDeployError": {
    message: "No se pudo iniciar el despliegue.",
    description: "Toast after triggerDeploy fails",
  },
  "services.cancelDeploySuccess": {
    message: "Despliegue cancelado.",
    description: "Toast after cancelDeploy succeeds",
  },
  "services.cancelDeployError": {
    message: "No se pudo cancelar el despliegue.",
    description: "Toast after cancelDeploy fails",
  },
  "services.rollbackSuccess": {
    message: "Reversión iniciada.",
    description: "Toast after rollbackService succeeds",
  },
  "services.rollbackError": {
    message: "No se pudo revertir.",
    description: "Toast after rollbackService fails",
  },
  "services.eventsStatusLive": {
    message: "Activo",
    description: "Deploy status: live",
  },
  "services.eventsStatusInProgress": {
    message: "En curso",
    description: "Deploy status: update_in_progress",
  },
  "services.eventsStatusFailed": {
    message: "Fallido",
    description: "Deploy status: update_failed",
  },
  "services.eventsStatusCanceled": {
    message: "Cancelado",
    description: "Deploy status: canceled",
  },
  "services.eventsPreDeployRunning": {
    message: "Comando previo al despliegue en ejecución",
    description: "Deploy row: the pre-deploy step is in progress",
  },
  "services.eventsPreDeploySucceeded": {
    message: "Comando previo al despliegue completado",
    description: "Deploy row: the pre-deploy step passed",
  },
  "services.eventsPreDeployCanceled": {
    message: "Comando previo al despliegue cancelado",
    description:
      "Deploy row: the deploy ended (superseded or canceled) while its pre-deploy step was still running",
  },
  "services.eventsPreDeployFailed": {
    message: "Comando previo al despliegue fallido",
    description:
      "Deploy row: the pre-deploy step failed (distinct from a health-check failure)",
  },
  "services.eventsRolledBackFrom": {
    message: "Revertido desde {target}",
    description: "Deploy row: provenance note when trigger=rollback",
  },
  "services.capLimitTitle": {
    message: "Límite de servicios alcanzado",
    description:
      "Alert title when the workspace's service creation cap is hit (w7/m9)",
  },
  "services.capLimitUpgrade": {
    message: "Mejorar plan",
    description: "Upgrade CTA button inside the cap-limit Alert (w7/m9)",
  },
  "services.networkingTitle": {
    message: "Redes",
    description: "Settings Networking card title (w7/m32)",
  },
  "services.networkingDescription": {
    message: "Restringe el tráfico HTTP entrante a estos CIDR de origen.",
    description: "Settings Networking card description (w7/m32)",
  },
  "services.networkingScalePlanOnly": {
    message:
      "Las reglas de IP de entrada para Web Services y Static Sites requieren el plan Scale o superior. Las reglas ya definidas siguen aplicándose y se pueden reemplazar por la regla que permite todo.",
    description:
      "Hint replacing the editable IP rules below the Scale plan (w8/m137)",
  },
  "services.networkingClearAll": {
    message: "Permitir todo el tráfico",
    description:
      "Button replacing every inbound IP rule with the allow-all rule below the Scale plan (w8/m137, w8/m157)",
  },
  "services.networkingHint": {
    message:
      "Introduce un bloque CIDR (p. ej. 203.0.113.0/24) y pulsa Añadir. Sin reglas, se bloquea todo el tráfico entrante; la regla 0.0.0.0/0 permite todas las IP de origen.",
    description: "Hint below the CIDR list in the Networking card (w7/m32)",
  },
  "services.networkingOpen": {
    message: "Abierto a todas las IP de origen",
    description:
      "Shown below the Scale plan when the service has only the allow-all rule (Render default, w7/m32, w8/m157)",
  },
  "services.networkingBlocked": {
    message:
      "Sin reglas: se bloquea todo el tráfico entrante a este servicio. Añade 0.0.0.0/0 para permitir todas las IP de origen.",
    description:
      "Warning shown when a service's inbound IP rule list is empty, saved or about to be saved (w8/m157)",
  },
  "services.networkingAdd": {
    message: "Añadir entrada a la lista de permitidos",
    description: "Button to add a CIDR to the draft list (w7/m32)",
  },
  "services.networkingEntryDescription": {
    message: "Descripción (opcional)",
    description: "Placeholder for a service allowlist entry description",
  },
  "services.networkingCIDR": {
    message: "Nuevo bloque CIDR",
    description: "Accessible label for the add-row CIDR input",
  },
  "services.networkingCIDRRule": {
    message: "Bloque CIDR de la regla {number}",
    description: "Accessible label for an existing allowlist CIDR row",
  },
  "services.networkingDescriptionLabel": {
    message: "Descripción de la nueva regla",
    description: "Accessible label for the add-row description input",
  },
  "services.networkingDescriptionRule": {
    message: "Descripción de la regla {number}",
    description: "Accessible label for an existing allowlist description row",
  },
  "services.networkingInvalid": {
    message: "Introduce un bloque CIDR IPv4 o IPv6 válido.",
    description: "Validation error for an invalid service allowlist CIDR",
  },
  "services.networkingSave": {
    message: "Guardar lista de permitidos",
    description: "Button to persist the CIDR list (w7/m32)",
  },
  "services.networkingRemove": {
    message: "Quitar {cidr}",
    description:
      "Accessible label on the trash icon next to a CIDR tag (w7/m32)",
  },
  "services.networkingMoveUp": {
    message: "Subir {cidr}",
    description: "Accessible label to move an allowlist entry earlier",
  },
  "services.networkingMoveDown": {
    message: "Bajar {cidr}",
    description: "Accessible label to move an allowlist entry later",
  },
  "services.networkingSaved": {
    message: "Lista de IP permitidas actualizada",
    description: "Toast on successful setServiceIpAllowList mutation (w7/m32)",
  },
  "services.networkingError": {
    message:
      "No se pudo actualizar la lista de IP permitidas. Inténtalo de nuevo.",
    description: "Toast on failed setServiceIpAllowList mutation (w7/m32)",
  },
  "services.outboundIpsTitle": {
    message: "IP de salida",
    description: "Settings card title for service egress IPs (w8/010)",
  },
  "services.outboundIpsDescription": {
    message:
      "Direcciones de salida compartidas de este servicio. Son las IP públicas actuales de los nodos del pool del inquilino y pueden cambiar cuando el autoscaler añade o quita nodos.",
    description:
      "Outbound IPs card helper — truthful shared-pool semantics (w8/010)",
  },
  "services.outboundIpsEmpty": {
    message:
      "Aún no se ha informado de ninguna IP de salida (habitual en clústeres locales sin ExternalIP en los nodos).",
    description: "Honest empty state when outboundIps.ips is empty (w8/010)",
  },
  "services.outboundIpsCopy": {
    message: "Copiar {ip}",
    description:
      "Accessible label on the copy button for one outbound IP (w8/010)",
  },
  "services.outboundIpsCopied": {
    message: "Copiado",
    description: "Toast after copying an outbound IP (w8/010)",
  },
  "services.outboundIpsCopyError": {
    message: "No se pudo copiar",
    description: "Toast when clipboard write fails for an outbound IP (w8/010)",
  },
  "services.environmentPageDescription": {
    message:
      "Todo lo que este servicio lee en tiempo de ejecución. Los valores permanecen ocultos hasta que muestras uno.",
    description: "Service Environment page introduction",
  },
  "services.environmentEdit": {
    message: "Editar",
    description: "Enter the coherent service environment draft",
  },
  "services.environmentMaskedValue": {
    message: "Valor secreto oculto",
    description: "Accessible label for an unrevealed secret",
  },
  "services.environmentUnchangedMasked": {
    message: "Sin cambios (oculto)",
    description: "Placeholder for an opaque unchanged draft value",
  },
  "services.environmentDuplicateKey": {
    message: "Cada clave de variable debe ser única.",
    description: "Draft validation for a duplicate environment key",
  },
  "services.environmentValueRequired": {
    message: "Introduce un valor para esta variable nueva.",
    description: "Draft validation for a new variable without a value",
  },
  "services.environmentValueNul": {
    message:
      "Un valor no puede contener un byte NUL, que el entorno de un proceso no admite. Usa un archivo secreto para contenido binario.",
    description:
      "Draft validation under an environment variable value with a NUL byte (w21/047)",
  },
  "services.environmentValueTooLarge": {
    message:
      "Una entrada KEY=value puede tener como máximo 131.071 bytes: un proceso no puede iniciarse con más. Usa un archivo secreto para cargas grandes.",
    description:
      "Draft validation under an environment variable whose KEY=value entry is over the process limit; the server counts UTF-8 bytes, so a Chinese character is 3 (w21/047)",
  },
  "services.environmentPortInvalid": {
    message:
      "PORT debe ser un número entero de 1 a 65535, y no 18012, 18013 ni 19099, que la plataforma reserva.",
    description:
      "Draft validation under a PORT environment variable the operator would ignore (w21/047)",
  },
  "services.environmentLimit": {
    message:
      "Las variables de entorno están limitadas a 500 y 512 KiB en total por servicio o grupo.",
    description:
      "Draft validation when the environment variables would exceed bex-api's per-service quota",
  },
  "services.environmentStagedDelete": {
    message: "Se quitará",
    description: "Badge on a staged environment deletion",
  },
  "services.environmentUndo": {
    message: "Deshacer",
    description: "Undo a staged environment deletion",
  },
  "services.environmentUnsavedTitle": {
    message: "Cambios sin guardar",
    description: "Combined draft save bar heading",
  },
  "services.environmentRestoredTitle": {
    message: "Cambios sin guardar restaurados",
    description:
      "Heading of the banner shown when an unsaved env draft is recovered in this tab, after a reload or a session-expiry sign-in (w3/m80)",
  },
  "services.environmentRestoredBody": {
    message:
      "Se restauraron tus ediciones de entorno sin guardar. Revísalas y luego guárdalas o descártalas.",
    description:
      "Body of the restored-draft banner. Names no cause: an ordinary reload restores the draft as a sign-in does (w4/337)",
  },
  "services.environmentRestoredDiscard": {
    message: "Descartar cambios restaurados",
    description: "Button that throws away a recovered env draft",
  },
  "services.environmentUnsavedVariables_one": {
    message: "{count} variable modificada",
    description:
      "Draft variable-operation count, composed with the file count at the call site (singular)",
  },
  "services.environmentUnsavedVariables_many": {
    message: "{count} de variables modificadas",
    description:
      "Draft variable-operation count, composed with the file count at the call site (plural)",
  },
  "services.environmentUnsavedVariables_other": {
    message: "{count} variables modificadas",
    description:
      "Draft variable-operation count, composed with the file count at the call site (plural)",
  },
  "services.environmentUnsavedFiles_one": {
    message: "{count} archivo modificado",
    description:
      "Draft file-operation count, composed with the variable count at the call site (singular)",
  },
  "services.environmentUnsavedFiles_many": {
    message: "{count} de archivos modificados",
    description:
      "Draft file-operation count, composed with the variable count at the call site (plural)",
  },
  "services.environmentUnsavedFiles_other": {
    message: "{count} archivos modificados",
    description:
      "Draft file-operation count, composed with the variable count at the call site (plural)",
  },
  "services.environmentSaveOptions": {
    message: "Opciones para guardar el entorno",
    description: "Accessible label for the split save menu",
  },
  "services.environmentSaveOnly": {
    message: "Solo guardar",
    description: "Persist and project without rolling the service",
  },
  "services.environmentSaveDeploy": {
    message: "Guardar y desplegar",
    description: "Persist and roll the current image once",
  },
  "services.environmentSaveRebuild": {
    message: "Guardar, recompilar y desplegar",
    description: "Persist then start one source build and deploy",
  },
  "services.environmentSaveOnlySuccess": {
    message:
      "Entorno guardado sin desplegar. Usa un despliegue estándar para aplicar los cambios pendientes.",
    description:
      "Save-only success feedback while the operator checks saved/runtime differences; does not assert that a no-op or revert has pending changes",
  },
  "services.environmentSaveDeploySuccess": {
    message: "Entorno guardado y despliegue iniciado",
    description: "Toast after save-and-deploy succeeds",
  },
  "services.environmentSavedRefreshFailed": {
    message:
      "Entorno guardado, pero esta página no pudo actualizarse. Recárgala para comprobar los valores más recientes y el estado del despliegue.",
    description:
      "A committed environment patch must not be retried because a follow-up read failed",
  },
  "services.environmentSaveError": {
    message: "No se pudo guardar el entorno. Tu borrador sigue aquí.",
    description: "Batch environment save failure",
  },
  "services.environmentDraftStale": {
    message:
      "Es posible que este grupo de entorno haya cambiado desde que empezaste tu borrador, por lo que no se guardó. Copia lo que necesites, descarta el borrador y vuelve a editar para partir de la versión más reciente.",
    description:
      "Refusal when a group draft's base revision is no longer current (ENV_GROUP_REVISION_CONFLICT) or predates revision tracking (w4/m161)",
  },
  "services.environmentSavedDeployFailedTitle": {
    message: "Configuración guardada; despliegue incompleto",
    description: "Partial rebuild failure heading",
  },
  "services.environmentSavedDeployFailedBody": {
    message:
      "Tus cambios de entorno están guardados. Reintenta solo el despliegue incompleto cuando estés listo.",
    description: "Partial rebuild failure recovery explanation",
  },
  "services.environmentRetryDeploy": {
    message: "Reintentar despliegue",
    description: "Retry only the failed deploy phase",
  },
  "services.environmentDiscardTitle": {
    message: "¿Descartar los cambios del entorno?",
    description: "Dirty navigation guard title",
  },
  "services.environmentDiscardBody": {
    message: "Estos cambios no se han guardado y se perderán si te vas.",
    description: "Dirty navigation guard explanation",
  },
  "services.environmentKeepEditing": {
    message: "Seguir editando",
    description: "Cancel dirty navigation",
  },
  "services.environmentDiscard": {
    message: "Descartar y salir",
    description: "Confirm dirty navigation",
  },
  "services.envReveal": {
    message: "Mostrar",
    description: "Reveal one masked environment value",
  },
  "services.envRevealOne": {
    message: "Mostrar {name}",
    description:
      'aria-label of the per-row Reveal button for one env var or secret file; starts with the visible text "Reveal" (w4/499)',
  },
  "services.envHideOne": {
    message: "Ocultar {name}",
    description:
      'aria-label of the per-row Hide button for one env var or secret file; starts with the visible text "Hide" (w4/499)',
  },
  "services.envHide": {
    message: "Ocultar",
    description: "Mask one revealed environment value",
  },
  "services.envCopy": {
    message: "Copiar variables de entorno",
    description: "Copy the complete dotenv export",
  },
  "services.envDownload": {
    message: "Descargar .env",
    description: "Download the complete dotenv export",
  },
  "services.envCopySuccess": {
    message: "Entorno copiado",
    description: "Toast after the complete dotenv export is copied",
  },
  "services.envCopyOne": {
    message: "Copiar {name}",
    description:
      "aria-label of the per-row copy button for one env var or secret file — distinct from the bulk envCopy dotenv export",
  },
  "services.envCopyOneSuccess": {
    message: "{name} copiado",
    description:
      "Toast after one env var value or secret file's contents is copied — distinct from the bulk envCopySuccess",
  },
  "services.envAddVariable": {
    message: "Añadir variable",
    description: "Add one blank draft variable",
  },
  "services.envAddGenerated": {
    message: "Secreto generado",
    description: "Add one previewable generated secret",
  },
  "services.envImport": {
    message: "Importar desde .env",
    description: "Open the dotenv import dialog",
  },
  "services.envImportTitle": {
    message: "Importar variables de entorno",
    description: "Dotenv import dialog title",
  },
  "services.envImportDescription": {
    message:
      "Pega asignaciones dotenv o elige un archivo de texto. Los valores importados permanecen en este borrador hasta que guardes.",
    description: "Dotenv import dialog explanation",
  },
  "services.envImportTextLabel": {
    message: "Contenido de dotenv",
    description: "Accessible label for the dotenv import textarea",
  },
  "services.envImportPlaceholder": {
    message: "API_URL=https://example.com\nSECRET=replace-me",
    description: "Dotenv import textarea placeholder",
  },
  "services.envImportChooseFile": {
    message: "Elegir un archivo",
    description: "Dotenv file picker button",
  },
  "services.envImportAdd": {
    message: "Añadir variables",
    description: "Stage parsed dotenv variables",
  },
  "services.envImportLineError": {
    message: "La línea {line} no es una asignación KEY=value.",
    description: "Line-numbered dotenv parse error",
  },
  "services.envImportKeyError": {
    message:
      "Línea {line}: el nombre debe tener hasta 253 letras, dígitos, puntos, guiones y guiones bajos.",
    description:
      "Dotenv import error: the variable name on a line is invalid (w4/159)",
  },
  "services.envImportQuoteError": {
    message: "Las comillas abiertas en la línea {line} nunca se cierran.",
    description: "Dotenv import error: a quoted value never closes (w4/159)",
  },
  "services.envImportTrailingError": {
    message: "La línea {line} tiene texto después de las comillas de cierre.",
    description:
      "Dotenv import error: text after a quoted value's closing quote (w4/159)",
  },
  "services.envImportReading": {
    message: "Leyendo archivo…",
    description:
      "Status while a chosen dotenv file is being read; Add waits for it",
  },
  "services.envImportFileError": {
    message: "Elige un archivo de texto legible de no más de 1 MiB.",
    description: "Dotenv file read or size error",
  },
  "services.secretFileUpload": {
    message: "Subir archivos",
    description: "Stage multiple local text files",
  },
  "services.secretFileSkippedTitle_one": {
    message: "Se omitió {count} archivo:",
    description:
      "Secret-file upload: heading over the one skipped file (w4/628)",
  },
  "services.secretFileSkippedTitle_many": {
    message: "Se omitieron {count} de archivos:",
    description: "Secret-file upload: heading over the skipped files (w4/628)",
  },
  "services.secretFileSkippedTitle_other": {
    message: "Se omitieron {count} archivos:",
    description: "Secret-file upload: heading over the skipped files (w4/628)",
  },
  "services.secretFileSkippedName": {
    message:
      "el nombre solo puede usar letras, dígitos, '.', '-' y '_' (hasta 253 caracteres).",
    description: "Secret-file upload: why a file was skipped — its name",
  },
  "services.secretFileSkippedSize": {
    message: "es mayor de 1 MiB.",
    description: "Secret-file upload: why a file was skipped — its size",
  },
  "services.secretFileSkippedEncoding": {
    message:
      "no es texto UTF-8. Para un archivo binario (keystore, .p12, certificado DER), súbelo codificado en base64 y decodifícalo al iniciar, p. ej. base64 -d /etc/secrets/keystore.b64 > /tmp/keystore.p12.",
    description: "Secret-file upload: why a file was skipped — not UTF-8 text",
  },
  "services.secretFileSkippedDuplicate": {
    message: "ya hay un archivo con este nombre en la lista.",
    description: "Secret-file upload: why a file was skipped — duplicate name",
  },
  "services.secretFileDuplicateName": {
    message: "Cada nombre de archivo secreto debe ser único.",
    description: "Draft validation for a duplicate file name",
  },
  "services.secretFileContentRequired": {
    message: "Añade contenido para este archivo nuevo.",
    description: "Draft validation for missing new-file content",
  },
  "services.secretFileLimit": {
    message:
      "Los archivos secretos están limitados a 500 por servicio o grupo, y su tamaño combinado a 1 MiB.",
    description:
      "Draft validation when the secret files would exceed bex-api's per-service quota: 500 files, 1 MiB of contents (names not counted), Render's 1 MB (w4/m363)",
  },
  "services.secretFileViewContent": {
    message: "Ver contenido",
    description: "Reveal or inspect secret-file contents in a dialog",
  },
  "services.secretFileViewContentOne": {
    message: "Ver contenido de {name}",
    description:
      "Accessible name for View contents once the file has a name (m101/t013)",
  },
  "services.secretFileEditContent": {
    message: "Editar contenido",
    description: "Edit staged secret-file contents in a dialog",
  },
  "services.secretFileEditContentOne": {
    message: "Editar contenido de {name}",
    description:
      "Accessible name for Edit contents once the file has a name (m101/t013)",
  },
  "services.secretFileContentDialogTitle": {
    message: "Contenido de {name}",
    description: "Secret-file content dialog title",
  },
  "services.secretFileContentDialogDescription": {
    message:
      "Este archivo se monta en modo de solo lectura en /etc/secrets después del próximo despliegue.",
    description: "Secret-file content dialog explanation",
  },
  "services.secretFileContentDialogDescriptionDocker": {
    message:
      "Este archivo se monta en modo de solo lectura en /etc/secrets después del próximo despliegue. Un build de Docker también lo encuentra en la raíz de su contexto de build: inclúyelo en .dockerignore para mantenerlo fuera de tu imagen.",
    description:
      "Secret-file content dialog explanation for a Docker-runtime service: the build context also receives the file, as on Render (w8/253)",
  },
  "services.secretFileLoadingContent": {
    message: "Cargando contenido…",
    description: "Secret-file content reveal progress",
  },
  "services.secretFileContentDone": {
    message: "Listo",
    description: "Close and stage secret-file content edits",
  },
  "services.secretFileUntitled": {
    message: "archivo sin título",
    description: "Fallback title for a new unnamed secret file",
  },
  "services.secretFileDelete": {
    message: "Eliminar archivo secreto",
    description: "Accessible label for a staged secret-file deletion",
  },
  "services.secretFileDeleteOne": {
    message: "Eliminar el archivo secreto {name}",
    description:
      "Accessible name for Delete secret file once the file has a name (m101/t013)",
  },
  "services.envGroupsLinkedTitle": {
    message: "Grupos de entorno vinculados",
    description: "Service-side environment group card title",
  },
  "services.envGroupsLinkedCount": {
    message: "Vinculados ({count})",
    description: "Count of groups linked to this service",
  },
  "services.envGroupsAvailableCount": {
    message: "Disponibles para vincular ({count})",
    description: "Count of groups available to link",
  },
  "services.envGroupsNoneLinked": {
    message: "No hay grupos de entorno vinculados a este servicio.",
    description: "Service group empty linked state",
  },
  "services.envGroupsAvailableTitle": {
    message: "Disponibles para vincular",
    description:
      "Available-groups heading while the service's environment is still resolving (no count yet)",
  },
  "services.envGroupsScopeLoading": {
    message: "Comprobando qué grupos comparten el entorno de este servicio…",
    description:
      "Available groups wait for the service's environment scope (w4/217)",
  },
  "services.envGroupsOrderReading": {
    message: "Confirmando qué grupo prevalece…",
    description:
      "Link order is being re-read after a link change; no winner is claimed until it lands (w4/322)",
  },
  "services.envGroupsOrderFailed": {
    message:
      "Vinculado, pero no se pudo confirmar qué grupo prevalece. Recarga el orden para verlo.",
    description:
      "The post-write Server read failed; the write stands, precedence stays unclaimed until a retry succeeds (w4/322)",
  },
  "services.envGroupsScopeError": {
    message: "No se pudo comprobar qué grupos puede vincular este servicio.",
    description:
      "Scope index failed; nothing is offered until a retry succeeds (w4/217)",
  },
  "services.envGroupsScopeEnvironmentMissing": {
    message:
      "El entorno de este servicio no está en la lista de entornos cargada, por lo que no se ofrece ningún grupo para vincular. Inténtalo de nuevo para actualizarla.",
    description:
      "The service reports an Environment the authorized scope index lacks even after a refresh; never falls back to Workspace (w4/220)",
  },
  "services.envGroupsNoneInScope": {
    message:
      "Ningún grupo sin vincular comparte el entorno de este servicio. Crea uno aquí o mueve un grupo a este entorno.",
    description:
      "Unlinked groups exist, but none in this service's environment scope (w4/217)",
  },
  "services.envGroupsNoneAvailable": {
    message:
      "Todos los grupos de entorno del espacio de trabajo ya están vinculados.",
    description: "Service group empty available state",
  },
  "services.envGroupsNoneAvailableCreate": {
    message:
      "Aún no hay grupos de entorno en el espacio de trabajo. Crea uno con variables, archivos y este servicio ya seleccionados.",
    description: "Service group create-first empty state",
  },
  "services.cronEveryMinute": {
    message: "Cada minuto",
    description: "Cron schedule preview phrase (w4/m188): fires every minute",
  },
  "services.cronEveryNMinutes": {
    message: "Cada {n} minutos",
    description:
      "Cron schedule preview phrase (w4/m188): uniform minute step; n is the step",
  },
  "services.cronEveryHour": {
    message: "Cada hora",
    description:
      "Cron schedule preview phrase (w4/m188): fires at minute 0 of every hour",
  },
  "services.cronEveryNHours": {
    message: "Cada {n} horas",
    description:
      "Cron schedule preview phrase (w4/m188): uniform hour step; n is the step",
  },
  "services.cronHourlyAtMinute": {
    message: "Cada hora en el minuto {minute}",
    description:
      "Cron schedule preview phrase (w4/m188): fires once an hour at the given minute",
  },
  "services.cronDailyAt": {
    message: "Todos los días a las {time}",
    description: "Cron schedule preview phrase (w4/m188): daily at HH:MM (UTC)",
  },
  "services.cronWeeklyAt": {
    message: "Cada {day} a las {time}",
    description:
      "Cron schedule preview phrase (w4/m188): weekly; day is a services.cronDay* weekday name, time is HH:MM (UTC)",
  },
  "services.cronMonthlyAt": {
    message: "El día {day} de cada mes a las {time}",
    description:
      "Cron schedule preview phrase (w4/m188): monthly on a day of the month at HH:MM (UTC)",
  },
  "services.cronDaySunday": {
    message: "domingo",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayMonday": {
    message: "lunes",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayTuesday": {
    message: "martes",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayWednesday": {
    message: "miércoles",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayThursday": {
    message: "jueves",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayFriday": {
    message: "viernes",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDaySaturday": {
    message: "sábado",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.environmentEditingTitle": {
    message: "Editando el entorno",
    description:
      "Edit bar title while the environment draft is open and holds no change yet (the bar says Unsaved changes once it does)",
  },
  "services.environmentNoChanges": {
    message: "Aún no hay cambios.",
    description:
      "Edit bar line while the open environment draft holds no change: what to do next",
  },
  "services.envAddOptions": {
    message: "Más formas de añadir una variable",
    description:
      "Accessible name of the chevron beside Add variable that opens the other ways to add (generated secret, .env import)",
  },
  "services.secretFilesReloadWithVariables": {
    message:
      "Se vuelven a cargar con las variables cuando lo intentas de nuevo arriba.",
    description:
      "Secret Files card line when both lists failed to load: the variables card's Try again reloads the files too, so this card has no button of its own",
  },
  "services.createSubmitWeb": {
    message: "Desplegar Web Service",
    description:
      "Create-wizard submit button for a web service (Render's 'Deploy web service')",
  },
  "services.createSubmitPrivate": {
    message: "Desplegar Private Service",
    description: "Create-wizard submit button for a private service",
  },
  "services.createSubmitWorker": {
    message: "Desplegar Background Worker",
    description: "Create-wizard submit button for a background worker",
  },
  "services.createSubmitCron": {
    message: "Desplegar Cron Job",
    description:
      "Create-wizard submit button for a cron job (Render's 'Deploy cron job')",
  },
  "services.createSubmitStatic": {
    message: "Desplegar Static Site",
    description: "Create-wizard submit button for a static site",
  },
  "services.createSourceEdit": {
    message: "Editar",
    description:
      "Button on the chosen repository's summary row that reopens the source picker",
  },
  "services.createSourceEditLabel": {
    message: "Cambiar el origen",
    description:
      "Accessible name of the Edit button on the chosen repository's summary row ({repo} = owner/name)",
  },
  "services.createBlockedSource": {
    message: "Elige un origen para continuar.",
    description:
      "Create-wizard action bar: why Deploy is held while no source is chosen",
  },
  "services.createFieldNameDescription": {
    message: "Un nombre único para este servicio.",
    description: "Create-wizard Name row description",
  },
  "services.createFieldRuntimeDescription": {
    message: "Elige el entorno de ejecución de este servicio.",
    description: "Create-wizard Language/runtime row description",
  },
  "services.createFieldBranchDescription": {
    message: "La rama de Git que se compila y despliega.",
    description: "Create-wizard Branch row description",
  },
  "services.createFieldBuildCommandDescription": {
    message: "Este comando compila tu app antes de cada despliegue.",
    description:
      "Create-wizard Build Command row description (native runtimes)",
  },
  "services.createFieldStartCommandDescription": {
    message: "Este comando inicia tu app en cada despliegue.",
    description: "Create-wizard Start Command row description",
  },
  "services.createFieldDockerCommandDescription": {
    message:
      "Opcional: reemplaza el CMD y el ENTRYPOINT de la imagen por otro comando para iniciar el servicio.",
    description: "Create-wizard Docker Command row description (Advanced)",
  },
  "services.createFieldPlanDescription": {
    message:
      "La instancia en la que se ejecuta tu servicio. Puedes cambiarla en cualquier momento.",
    description: "Create-wizard compute plan row description",
  },
  "services.createEnvVarsDescription": {
    message:
      "Define configuración y secretos específicos del entorno (como claves de API) y lee esos valores desde tu código.",
    description: "Create-wizard Environment Variables card description",
  },
  "services.createBlockedPublishPath": {
    message: "Introduce un directorio de publicación para continuar.",
    description:
      "Create-wizard action bar: Deploy is held until a static site has a publish directory",
  },
  "services.createBlockedSchedule": {
    message: "Introduce una programación válida para continuar.",
    description:
      "Create-wizard action bar: Deploy is held until a cron job has a valid schedule",
  },
  "services.createBlockedCommands": {
    message: "Introduce los comandos de compilación e inicio para continuar.",
    description:
      "Create-wizard action bar: Deploy is held until the native runtime's build and start commands are filled",
  },
  "services.createBlockedPort": {
    message: "Introduce un puerto válido para continuar.",
    description:
      "Create-wizard action bar: Deploy is held while the port is out of range",
  },
  "services.createBlockedDisk": {
    message:
      "Introduce una ruta de montaje y un tamaño de disco válidos para continuar.",
    description:
      "Create-wizard action bar: Deploy is held while a drafted disk has no valid mount path or size (w4/m652)",
  },
  "services.createBuildSourceRegionNotice": {
    message:
      "Los servicios vinculados a este origen de build de Git se crean en {region} para coincidir con el origen de build.",
    description:
      "Create service wizard: notice explaining that a git Build Source determines the new service's region; {region} is the Render region name",
  },
  "services.createBuildSourceAutoDeployHint": {
    message:
      "Despliega este servicio automáticamente cada vez que finalice con éxito una nueva compilación del origen de build vinculado.",
    description:
      "Create service wizard: Auto-Deploy toggle description when creating from a Build Source",
  },
  "services.buildSourceKindGit": {
    message: "Repositorio Git",
    description: "Badge or label for a git-backed Build Source",
  },
  "services.buildSourceKindImage": {
    message: "Imagen de contenedor",
    description: "Badge or label for an image-backed Build Source",
  },
  "services.sourceBuildSourceIdLabel": {
    message: "ID del origen de build",
    description: "Accessible field name for copying a Build Source ID",
  },
  "services.sourceBuildSourceIdCopied": {
    message: "ID del origen de build copiado",
    description: "Toast shown after copying a Build Source ID",
  },
  "services.sourceBuildSourceIdCopyFailed": {
    message: "No se pudo copiar el ID del origen de build",
    description: "Toast shown when copying a Build Source ID fails",
  },
  "services.deployMenuLatestBuild": {
    message: "Desplegar la última compilación",
    description:
      "Manual Deploy dropdown item for a Build-Source-linked service: deploy the latest succeeded build from the Build Source",
  },
  "services.deployMenuSpecificBuild": {
    message: "Desplegar una compilación específica…",
    description:
      "Manual Deploy dropdown item for a Build-Source-linked service: pick a specific succeeded build (bld-…) to deploy",
  },
  "services.deployBuildTitle": {
    message: "Desplegar una compilación específica",
    description: "Title of the dialog to pick a specific Build Source build",
  },
  "services.deployBuildBody": {
    message:
      "Elige una compilación exitosa del origen de build vinculado para desplegarla en este servicio.",
    description:
      "Description in the dialog to pick a specific Build Source build",
  },
  "services.deployBuildConfirm": {
    message: "Desplegar compilación",
    description: "Confirm button label in the deploy-a-specific-build dialog",
  },
  "services.deployBuildListLabel": {
    message: "Compilaciones del origen de build vinculado",
    description:
      "Accessible label for the build list inside the deploy-a-specific-build dialog",
  },
  "services.deployBuildEmpty": {
    message:
      "Este origen de build aún no tiene compilaciones. Ejecuta primero una compilación en el origen de build.",
    description:
      "Empty state in the deploy-a-specific-build dialog when no builds exist",
  },
  "services.deployBuildLoadError": {
    message: "No se pudieron cargar las compilaciones de este origen de build.",
    description:
      "Error message in the deploy-a-specific-build dialog when fetching builds fails",
  },
  "services.deployBuildNoSucceeded": {
    message: "Todavía no hay compilaciones exitosas listas para desplegar.",
    description:
      "Notice in the deploy-a-specific-build dialog when builds exist but none succeeded",
  },
  "services.buildStatusCreated": {
    message: "En cola",
    description: "Build Source build status badge: created/queued",
  },
  "services.buildStatusInProgress": {
    message: "Compilando",
    description: "Build Source build status badge: in_progress",
  },
  "services.buildStatusSucceeded": {
    message: "Exitosa",
    description: "Build Source build status badge: succeeded",
  },
  "services.buildStatusFailed": {
    message: "Fallida",
    description: "Build Source build status badge: failed",
  },
  "services.buildStatusCanceled": {
    message: "Cancelada",
    description: "Build Source build status badge: canceled",
  },
};

export default esServices;
