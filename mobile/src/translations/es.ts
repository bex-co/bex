export const es = {
  freshness: {
    notLoaded: "Esperando la primera actualización",
    current: "Actualizado",
    stale: "Desactualizado",
  },
  access: {
    offline: "Vuelve a conectarte para comprobar el acceso antes de continuar.",
    checking: "Comprobando el acceso…",
    unavailable: "No pudimos comprobar tu acceso. Inténtalo de nuevo.",
    changed: "Tu acceso en este espacio de trabajo ha cambiado.",
    cannotOpen: "Este elemento no está disponible con tu acceso actual.",
    restrictedTitle: "Restringido",
    backToStatus: "Volver a Estado",
  },
  navigation: {
    status: "Estado",
    activity: "Actividad",
    sessions: "Sesiones",
    notifications: "Alertas",
  },
  notifications: {
    inboxChecking: "Comprobando el acceso y actualizando las alertas…",
    inboxError:
      "No se pudieron actualizar las alertas. Comprueba tu conexión e inténtalo de nuevo.",
    benefit:
      "Recibe en tu teléfono las actualizaciones importantes de servicios y despliegues.",
    emptyTitle: "Aún no hay alertas",
    title: "Alertas",
    body: "Alertas operativas recientes guardadas en este dispositivo.",
    settings: "Notificaciones push",
    permissionExplanation:
      "bex solo solicita permiso de notificaciones cuando tocas Activar. Puedes cambiar el permiso más tarde en la configuración del sistema.",
    enable: "Activar",
    disable: "Desactivar",
    recent: "Recientes (%{count})",
    markAllRead: "Marcar todo como leído",
    empty:
      "Las actualizaciones de servicios y despliegues aparecerán aquí cuando se reciban en este dispositivo.",
    open: "Abrir notificación: %{title}",
    states: {
      checking: "Comprobando permiso…",
      unconfigured:
        "Las notificaciones push no están configuradas en esta compilación.",
      unavailable:
        "La entrega de notificaciones push está desactivada en este servidor de bex.",
      disabled: "Desactivadas",
      denied: "Permiso denegado",
      revoked: "El permiso se revocó en la configuración del sistema",
      enabling: "Activando…",
      enabled: "Activadas",
      offline: "Sin conexión: el registro se reparará al volver a conectarte",
      error: "No se pudo completar el registro",
    },
  },
  status: {
    overview: "Estado de los recursos",
    healthy: "Correcto",
    review: "Revisar",
    unknown: "Desconocido",
    title: "Estado",
    body: "Revisa tus servicios y detecta lo que necesita atención.",
  },
  usageGlance: {
    title: "Uso del mes hasta la fecha",
    states: {
      complete: "Datos completos",
      partial: "Datos parciales",
      unknown: "Cobertura sin confirmar",
      unavailable: "No disponible",
      "healthy-empty": "Completo · sin uso",
    },
    meters: {
      instance_seconds: "Ejecución",
      egress_bytes: "Ancho de banda de salida",
      build_seconds: "Tiempo de build",
      storage_gb_seconds: "Almacenamiento de datos",
      sandbox_compute_seconds: "Cómputo de agentes",
    },
    empty: "Los datos completos no informan de ningún uso distinto de cero.",
    noEvidence: "Los totales de uso aún no están disponibles.",
    through: "Medido hasta %{time}",
    degraded: "Algunas fuentes de uso están retrasadas: %{sources}",
    refreshUnavailable:
      "La actualización no está disponible. Se muestran los últimos totales recibidos.",
    loading: "Cargando el uso del mes hasta la fecha…",
  },
  activity: {
    explore: "Explorar la actividad del servicio",
    title: "Actividad",
    body: "Elige un servicio para inspeccionar despliegues, eventos, métricas y logs en vivo de solo lectura.",
    timelineTitle: "Despliegues y eventos",
    deploy: "Despliegue",
    empty: "Aún no hay despliegues ni eventos.",
    loadMore: "Cargar actividad anterior",
  },
  resources: {
    search: "Buscar recursos",
    clearSearch: "Borrar búsqueda",
    resetFilters: "Borrar filtros",
    noMatches: "Ningún recurso coincide",
    service: "Servicio",
    unknownStatus: "desconocido",
    updatedAt: "Actualizado %{time}",
    waitingForUpdate: "Esperando la primera actualización",
    filterLabel: "Filtrar recursos por tipo",
    filters: {
      all: "Todos",
      service: "Servicios",
      database: "Postgres",
      keyValue: "Key Value",
    },
    partialError:
      "No se pudieron actualizar algunos datos de recursos. Los datos visibles pueden estar desactualizados.",
    loadError:
      "No se pudieron cargar los recursos. Desliza hacia abajo para intentarlo de nuevo.",
    loading: "Cargando el estado de los recursos…",
    emptyTitle: "No hay recursos",
    emptyBody: "Este espacio de trabajo aún no tiene recursos supervisados.",
    emptyFilter: "Ningún recurso coincide con este filtro.",
    ungrouped: "Recursos del espacio de trabajo",
    latestDeploy: "Último despliegue %{id}",
    resourceAccessibility: "%{name}, %{type}, estado %{status}",
  },
  service: {
    title: "Servicio",
    overview: "Estado actual",
    region: "Región",
    replicas: "Instancias",
    revision: "Revisión",
    latestDeploy: "Último despliegue",
    partialError:
      "No se pudo actualizar parte de la evidencia del servicio. Los datos visibles pueden estar desactualizados.",
    loadError: "No se pudo cargar la evidencia del servicio.",
  },
  cron: {
    title: "Historial de ejecuciones de cron",
    description:
      "Ejecuciones recientes y sus tiempos, según el servidor. Ejecutar ahora reemplaza cualquier ejecución activa.",
    runNow: "Ejecutar ahora",
    cancelCurrent: "Cancelar ejecución actual",
    suspended:
      "Ejecutar ahora no está disponible mientras este servicio está suspendido. Aún se puede cancelar una ejecución ya activa.",
    refreshRequired:
      "Es posible que la acción anterior se haya confirmado. Actualiza el historial correctamente antes de confirmar otra acción.",
    refresh: "Actualizar historial de ejecuciones",
    refreshing: "Actualizando…",
    loadError: "No se pudo cargar el historial de ejecuciones de cron.",
    empty: "Aún no hay ejecuciones de cron.",
    loadMore: "Cargar ejecuciones anteriores",
    openLogs: "Abrir los logs generales del servicio",
    started: "Inicio: %{time}",
    finished: "Fin: %{time}",
    duration: "Duración: %{duration}",
    notStarted: "no informado",
    status: {
      pending: "Pendiente",
      running: "En ejecución",
      successful: "Correcta",
      failed: "Fallida",
      canceled: "Cancelada",
      unknown: "Desconocido",
    },
    feedback: {
      ambiguous:
        "La respuesta terminó sin un resultado confirmado. Actualiza el historial de ejecuciones antes de revisar y confirmar otra acción.",
      convergenceTimeout:
        "El servidor aceptó la acción, pero el historial de ejecuciones no convergió antes de que terminara el sondeo. Actualiza el historial antes de otra acción.",
    },
  },
  metrics: {
    title: "Resumen de métricas",
    window: "Última hora · se actualiza cada 30 segundos",
    cpu: "CPU",
    memory: "Memoria",
    network: "Red de salida",
    loading: "Cargando %{metric}",
    unavailable: "La fuente de métricas no está configurada.",
    error: "No se pudo cargar esta métrica.",
    partial: "Muestras parciales: se muestran los últimos valores válidos.",
    degraded: "Puede estar subestimado: %{sources}",
    noData: "No hay muestras en este intervalo.",
    accessibilitySummary:
      "%{metric}, actual %{value}, %{points} muestras en la última hora",
  },
  events: {
    unknown: "Evento: %{type}",
    noType: "tipo desconocido",
    branchTransition: "Rama %{from} → %{to}",
    countTransition: "Instancias %{from} → %{to}",
    reason: "Motivo: %{reason}",
    types: {
      deploy_started: "Despliegue iniciado",
      deploy_ended: "Despliegue finalizado",
      build_started: "Build iniciado",
      build_ended: "Build finalizado",
      pre_deploy_started: "Predespliegue iniciado",
      pre_deploy_ended: "Predespliegue finalizado",
      server_available: "El servicio pasó a estar disponible",
      server_unavailable: "El servicio pasó a no estar disponible",
      plan_changed: "Plan cambiado",
      branch_changed: "Rama cambiada",
      instance_count_changed: "Número de instancias cambiado",
      autoscaling_config_changed: "Escalado automático cambiado",
      job_run_started: "Ejecución de job iniciada",
      job_run_ended: "Ejecución de job finalizada",
    },
  },
  datastores: {
    overview: "Estado actual",
    postgres: "Postgres",
    keyValue: "Key Value",
    plan: "Plan",
    region: "Región",
    id: "ID del recurso",
    loadError:
      "No se pudo actualizar este almacén de datos. El estado visible puede estar desactualizado; desliza hacia abajo para intentarlo de nuevo.",
  },
  postgresInsights: {
    title: "Información de Postgres",
    connection: "Accesibilidad de la base de datos",
    capacity: "Capacidad",
    logicalSize: "Tamaño lógico de la base de datos",
    diskUsed: "Disco usado",
    diskCapacity: "Capacidad del disco",
    connections: "Conexiones",
    tables: "Señales de escaneo de tablas",
    tableSummary: "%{size} · seq %{sequential} · idx %{index} · dead %{dead}",
    noTableData: "No se devolvieron observaciones de tablas.",
    sourceUnavailable: "Esta fuente de información no está configurada.",
    transportError: "Esta solicitud de información no pudo llegar al servidor.",
    state: {
      loading: "Comprobando",
      empty: "Sin observaciones",
      current: "Actualizado",
      stale: "Desactualizado",
      degraded: "Parcial",
      "source-unavailable": "Fuente no disponible",
      "transport-error": "Error de conexión",
    },
    connectionState: {
      loading: "Comprobando la respuesta de la base de datos…",
      empty: "Sin observaciones de la base de datos",
      current: "La base de datos respondió",
      stale: "La última respuesta está desactualizada",
      degraded: "Solo se completó una parte de la comprobación",
      "source-unavailable":
        "Fuente de información de la base de datos no disponible",
      "transport-error":
        "La comprobación de la base de datos no pudo llegar al servidor",
    },
  },
  keyValueInsights: {
    title: "Información de Key Value",
    refreshing: "Actualizando la información de Key Value",
    window: "Última hora · se actualiza cada 30 segundos",
    connectionHealth: "Disponibilidad (no es un ping de protocolo)",
    storage: "Almacenamiento usado / capacidad",
    memory: "Memoria usada",
    connections: "Clientes conectados",
    accessibilitySummary:
      "%{metric}: %{value}, basado en %{points} observaciones",
    health: {
      available: "Disponible",
      creating: "Creándose",
      unavailable: "No disponible",
      suspended: "Suspendido",
      unknown: "Desconocido",
    },
  },
  safeActions: {
    cardTitle: "Acciones seguras",
    title: "Confirmar acción",
    message:
      "Confirma esta acción exacta y su objetivo. La app no reintentará una mutación automáticamente.",
    serverConfirmation:
      "Este recurso protegido requiere una segunda confirmación. Frase del servidor: %{confirmation}",
    confirm: "Confirmar",
    cancel: "Cancelar",
    pending: "Enviando…",
    noneAvailable:
      "No hay ninguna acción segura disponible en el estado actual.",
    dismiss: "Descartar",
    refreshFirst: "Actualizar primero",
    actions: {
      triggerDeploy: "Desplegar lo último",
      cancelDeploy: "Cancelar despliegue activo",
      rollbackDeploy: "Revertir despliegue",
      restartService: "Reiniciar servicio",
      suspendService: "Suspender servicio",
      resumeService: "Reanudar servicio",
      restartDatabase: "Reiniciar Postgres",
      suspendDatabase: "Suspender Postgres",
      resumeDatabase: "Reanudar Postgres",
      suspendKeyValue: "Suspender Key Value",
      resumeKeyValue: "Reanudar Key Value",
    },
    feedback: {
      success:
        "El servidor aceptó la acción. El estado actualizado seguirá refrescándose desde el servidor.",
      acceptedUnverified:
        "El servidor aceptó la acción, pero esta API no puede demostrar que el reinicio se completó. Sigue comprobando el estado actual.",
      authorizationDenied:
        "No tienes autorización para realizar esta acción. No se dio por hecho ningún éxito.",
      conflict:
        "El estado del recurso cambió, por lo que la acción no se envió. Actualiza y revísalo de nuevo.",
      timeoutUnknown:
        "La respuesta agotó el tiempo de espera y es posible que se haya confirmado. Actualiza el estado del servidor antes de crear otra confirmación.",
      auditPending:
        "La acción se confirmó, pero su resultado de auditoría sigue pendiente.",
      auditUnavailable:
        "El resultado de la acción está incompleto porque el estado de auditoría no está disponible.",
      failed:
        "La acción falló. El estado actual del servidor no se modificó localmente.",
      canceled: "La acción pendiente se canceló localmente.",
    },
  },
  resourceActions: {
    blocked: {
      protectedConfirmation:
        "Este recurso protegido necesita una confirmación adicional antes de poder ejecutar esta acción.",
      suspended:
        "No disponible mientras el recurso está suspendido. Reanúdalo primero.",
      noActiveDeploy:
        "Ahora mismo no hay ningún despliegue activo que cancelar.",
      noActiveRun: "Ahora mismo no hay ninguna ejecución activa que cancelar.",
      noEligibleRollbackTarget:
        "Ningún despliegue anterior es apto como destino de reversión.",
      billingBlocked:
        "La facturación requiere atención antes de poder ejecutar esta acción. Resuélvelo en el panel.",
      unavailable:
        "El servidor no pudo confirmar esta acción en este momento. Inténtalo de nuevo.",
      generic:
        "El servidor no permite esta acción en este momento. Inténtalo de nuevo.",
    },
  },
  sessions: {
    title: "Sesiones de agente",
    body: "Asigna y supervisa el trabajo de agentes de programación desde tu teléfono.",
    gated: "Requiere la fase 1 de ADR047",
  },
  agentSessions: {
    heading: "Tus agentes, de un vistazo",
    description:
      "Sigue el progreso, revisa resultados o inicia una nueva tarea.",
    newShort: "Nueva",
    loading: "Cargando sesiones…",
    refreshError:
      "No se pudo actualizar. Se muestran las sesiones cargadas anteriormente.",
    unavailable: "No se pudieron cargar las sesiones. Inténtalo de nuevo.",
    emptyTitle: "Aún no hay sesiones de agente",
    emptyBody:
      "Asigna una tarea de agente de programación a un repositorio desde tu teléfono.",
    phase: {
      creating: "Iniciando",
      running: "En ejecución",
      resuming: "Reanudando",
      redispatching: "Reenviando",
      completed: "Completada",
      failed: "Fallida",
      canceling: "Cancelando",
      canceled: "Cancelada",
      unknown: "Desconocido",
    },
    composer: {
      new: "Nueva sesión",
      title: "Nueva sesión de agente",
      close: "Cerrar",
      repo: "Repositorio",
      agent: "Agente",
      agentAuto: "Auto",
      model: "Modelo",
      modelDefault: "%{model} (predeterminado)",
      pickerDone: "Listo",
      pickerCancel: "Cancelar",
      searchRepo: "Buscar repositorios",
      searchEmpty: "Ningún repositorio coincide",
      prompt: "Tarea",
      promptPlaceholder: "Pregunta lo que quieras",
      submit: "Asignar tarea",
      submitted: "Sesión asignada.",
      openSession: "Abrir sesión",
      noRepos: "No hay repositorios conectados.",
      setupTitle: "Termina la configuración en el escritorio",
      needGithub: "Conecta GitHub en el escritorio para abrir pull requests.",
      needModelKey:
        "Un administrador del espacio de trabajo debe añadir una clave de proveedor de modelos en Configuración → Agentes del espacio de trabajo en el panel web para ejecutar sesiones.",
      needDesktop: "No se pudieron cargar las sesiones. Inténtalo de nuevo.",
      connectGithub: "Conectar GitHub",
    },
    detail: {
      overview: "Resumen",
      phase: "Fase",
      updated: "Actualizado %{time}",
      unavailable: "Esta sesión no está disponible.",
      notFound: "Sesión no encontrada.",
      failure: "Fallo",
      deliveryNote: "Los cambios no se entregaron por completo",
      pullRequest: "Pull request",
      draftPr: "Pull request en borrador",
      openPr: "Abrir en GitHub",
      cancel: "Cancelar sesión",
      refreshError:
        "No se pudo actualizar. Se muestran los detalles de sesión cargados anteriormente.",
      stale:
        "Los detalles de la sesión pueden estar desactualizados. Desliza hacia abajo para actualizar.",
    },
  },
  auth: {
    loadingTitle: "Abriendo bex",
    loadingBody: "Restaurando tu sesión segura.",
    signInTitle: "Inicia sesión en bex",
    signInBody:
      "Continúa en el navegador del sistema. La app nunca ve tu contraseña ni tu respuesta de MFA.",
    signInAction: "Continuar de forma segura",
    signInError:
      "El inicio de sesión no se completó. Comprueba tu conexión e inténtalo de nuevo.",
    callbackTitle: "Finalizando el inicio de sesión",
    callbackBody: "Protegiendo tu sesión y abriendo bex.",
    expiredTitle: "La sesión necesita conexión",
    expiredBody:
      "Tu token de acceso caducó mientras bex estaba sin conexión. Vuelve a conectarte para renovarlo de forma segura.",
    retry: "Reintentar",
    signOut: "Cerrar sesión",
  },
  invite: {
    title: "Invitación al espacio de trabajo",
    loading: "Protegiendo esta invitación en tu dispositivo.",
    signInBody:
      "Inicia sesión en el navegador del sistema y vuelve aquí para revisar la invitación. El enlace puede unir a la cuenta que elijas, sea cual sea su dirección de correo.",
    signIn: "Inicia sesión para revisar",
    signInError:
      "El inicio de sesión no se completó. La invitación sigue protegida en este dispositivo para que puedas intentarlo de nuevo.",
    readyBody:
      "Al aceptar, tu cuenta actual de bex obtiene acceso al espacio de trabajo identificado por este enlace de un solo uso. En esta aceptación directa no se comprueba la dirección de correo invitada.",
    accept: "Aceptar invitación",
    accepting: "Aceptando esta invitación una sola vez. Mantén la app abierta.",
    retryTitle: "Invitación no confirmada",
    retry: "Intentar aceptar de nuevo",
    retryable: {
      transport:
        "La respuesta terminó antes de que bex pudiera confirmar el resultado. La invitación sigue protegida; reintenta explícitamente cuando tengas conexión.",
      unavailable:
        "bex no está disponible temporalmente. La invitación sigue protegida; reintenta explícitamente cuando tengas conexión.",
    },
    acceptedTitle: "Invitación aceptada",
    acceptedBody: "Te has unido a %{workspace}.",
    refreshFailed:
      "La invitación se aceptó, pero no se pudo actualizar la lista de espacios de trabajo. Al volver a abrir Estado se reintentará.",
    openWorkspace: "Abrir Estado",
    failedTitle: "Invitación no disponible",
    empty: "Esta ruta no contiene ninguna invitación pendiente.",
    failures: {
      invalid: "Este enlace de invitación no es válido o ya no existe.",
      expired:
        "Esta invitación ha caducado. Pide a un administrador del espacio de trabajo que la reenvíe.",
      "already-accepted": "Esta invitación de un solo uso ya se aceptó.",
      "plan-limit":
        "El plan actual del espacio de trabajo o su límite de puestos no permite aceptar esta invitación. Pide a un administrador que lo resuelva y reenvíe la invitación.",
      authorization:
        "Tu cuenta actual no está autorizada para aceptar esta invitación.",
      "subject-changed":
        "La cuenta con sesión iniciada cambió, por lo que se borró la invitación guardada. Abre de nuevo el enlace original con la cuenta prevista.",
      failed:
        "bex rechazó esta invitación. No se dio por hecha ninguna pertenencia.",
      storage:
        "Este dispositivo no pudo proteger ni borrar la invitación. La app no intentará aceptarla.",
    },
  },
  workspace: {
    current: "Espacio de trabajo",
    choose: "Elegir espacio de trabajo",
    switchLabel:
      "Cambiar de espacio de trabajo. Espacio de trabajo actual: %{name}",
    confirm: "Cambiar",
    cancel: "Cancelar",
    offline:
      "Sin conexión. Se muestra el espacio de trabajo actual sin datos del inquilino en caché.",
    loadingTitle: "Abriendo tu espacio de trabajo",
    loadingBody: "Comprobando tu pertenencia al espacio de trabajo de bex.",
    switchingTitle: "Cambiando de espacio de trabajo",
    switchingBody:
      "Borrando el espacio de trabajo anterior antes de abrir el siguiente.",
    emptyTitle: "Aún no hay espacio de trabajo",
    emptyBody:
      "Únete a un espacio de trabajo o crea uno en bex.co y vuelve a intentarlo.",
    errorTitle: "Espacio de trabajo no disponible",
    errorBody: "bex no pudo cargar tu espacio de trabajo. Inténtalo de nuevo.",
    offlineTitle: "El espacio de trabajo necesita conexión",
    offlineBody:
      "Vuelve a conectarte para cargar los datos del espacio de trabajo de forma segura.",
  },
  drawer: {
    openMenu: "Abrir menú",
    openMenuHint: "Muestra tus espacios de trabajo y tu cuenta",
    menuLabel: "Menú de espacio de trabajo y cuenta",
    opened: "Menú abierto",
    closeBackdrop: "Cerrar menú",
    workspaces: "Espacios de trabajo",
    workspaceRow: "%{name}, %{status}",
    noStatus: "sin rol ni plan",
    roles: {
      admin: "Administrador",
      developer: "Desarrollador",
      contributor: "Colaborador",
      viewer: "Lector",
      billing: "Facturación",
    },
    plans: {
      hobby: "Hobby",
      pro: "Pro",
      scale: "Scale",
      enterprise: "Enterprise",
      free: "Gratis",
    },
    signedIn: "Sesión iniciada",
    identityLoading: "Cargando tu cuenta…",
    identityOffline:
      "Sin conexión: los datos de la cuenta no están disponibles",
    identityUnavailable: "Datos de la cuenta no disponibles",
    identityRetry: "Reintentar",
    identityAccessibility: "Sesión iniciada como %{primary} %{secondary}",
    logout: "Cerrar sesión",
    logoutConfirmTitle: "¿Cerrar sesión?",
    logoutConfirmMessage:
      "Tendrás que volver a iniciar sesión para supervisar tus espacios de trabajo.",
  },
  settings: {
    title: "Configuración",
    open: "Abrir configuración",
    close: "Cerrar configuración",
    colorScheme: "Esquema de color",
    themeSystem: "Sistema",
    themeLight: "Claro",
    themeDark: "Oscuro",
    language: "Idioma",
    version: "Versión %{version}",
  },
  deepLink: {
    invalidTitle: "Enlace de bex no válido",
    invalidBody:
      "Este enlace no contiene un identificador de recurso de bex válido.",
    serviceTitle: "Servicio",
    serviceBody:
      "Los detalles de supervisión del servicio llegan en el próximo hito.",
    sessionTitle: "Sesión de agente",
    sessionBody:
      "Los detalles de supervisión de sesiones llegan con las operaciones de agentes.",
  },
  logs: {
    title: "Logs en vivo",
    description:
      "Ponte al día con los logs recientes de la app, el build, el predespliegue y las solicitudes, y luego sigue la salida en vivo sin abrir un shell.",
    open: "Inspeccionar logs",
    back: "Volver a la evidencia del servicio",
    searchLabel: "Buscar en los logs",
    searchPlaceholder: "Buscar mensajes de log",
    pause: "Pausar",
    follow: "Seguir",
    historyOnly:
      "Solo historial: estos filtros no se pueden aplicar al seguimiento en vivo.",
    storeUnavailable:
      "Estos filtros de logs requieren el almacén de logs duradero.",
    unavailable:
      "Los logs no están disponibles. Comprueba tu conexión e inténtalo de nuevo.",
    empty: "No hay líneas de log en este intervalo.",
    type: {
      all: "Todos",
      app: "App",
      build: "Build",
      predeploy: "Predespliegue",
      request: "Solicitud",
    },
    phase: {
      idle: "Seguimiento de logs detenido",
      catching_up: "Cargando logs recientes…",
      connecting: "Conectando con los logs en vivo…",
      live: "En vivo",
      reconnecting: "Reconectando con los logs en vivo…",
      history_only: "Solo historial",
      error: "Seguimiento de logs no disponible",
    },
  },
  common: {
    back: "Volver",
    cancel: "Cancelar",
    notFoundTitle: "Esta pantalla no existe",
    backToStatus: "Volver a Estado",
    seeAll: "Ver todo",
    notEnoughChartData: "No hay datos suficientes",
  },
};
