import type { TranslationEntry } from "@/i18n";

const es: Record<string, TranslationEntry> = {
  "sandboxGroupPages.navSandboxes": {
    message: "Sandboxes",
    description: "Sidebar entry and heading: a sandbox group's sandboxes",
  },
  "sandboxGroupPages.navSnapshots": {
    message: "Snapshots",
    description: "Sidebar entry and heading: a sandbox group's snapshots",
  },
  "sandboxGroupPages.navImages": {
    message: "Imágenes personalizadas",
    description:
      "Sidebar entry and heading: the images imported into a sandbox group",
  },
  "sandboxGroupPages.navSettings": {
    message: "Configuración",
    description: "Sidebar entry and heading: a sandbox group's settings",
  },
  "sandboxGroupPages.statusCreating": {
    message: "Creándose",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusRunning": {
    message: "En ejecución",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusResuming": {
    message: "Reanudando",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusSuspending": {
    message: "Suspendiendo",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusSuspended": {
    message: "Suspendido",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusErrored": {
    message: "Con error",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusTerminated": {
    message: "Terminado",
    description: "Sandbox status",
  },
  "sandboxGroupPages.rangeLabel": {
    message: "Intervalo de tiempo",
    description: "Accessible name of the time range picker",
  },
  "sandboxGroupPages.rangeHour": {
    message: "Última hora",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeDay": {
    message: "Últimas 24 horas",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeWeek": {
    message: "Últimos 7 días",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeMonth": {
    message: "Últimos 30 días",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeAll": {
    message: "Todo el periodo",
    description: "Time range option",
  },
  "sandboxGroupPages.searchSandboxes": {
    message: "Busca por nombre, ID del sandbox o estado",
    description: "Placeholder of the Sandboxes search",
  },
  "sandboxGroupPages.searchSnapshots": {
    message: "Busca snapshots",
    description: "Placeholder of the Snapshots search",
  },
  "sandboxGroupPages.searchImages": {
    message: "Busca imágenes",
    description: "Placeholder of the Custom images search",
  },
  "sandboxGroupPages.pauseLive": {
    message: "Pausar actualizaciones en vivo",
    description: "Button that stops the list refreshing",
  },
  "sandboxGroupPages.resumeLive": {
    message: "Reanudar actualizaciones en vivo",
    description: "Button that restarts the list refreshing",
  },
  "sandboxGroupPages.showHideColumns": {
    message: "Mostrar u ocultar columnas",
    description: "Accessible name of the columns menu",
  },
  "sandboxGroupPages.summaryTotal": {
    message: "Total",
    description: "Summary: runs that ended in the time range",
  },
  "sandboxGroupPages.summaryErrored": {
    message: "Con error",
    description: "Summary: share of runs that ended in an error",
  },
  "sandboxGroupPages.summaryErroredTip": {
    message:
      "Proporción de sandboxes del intervalo de tiempo que terminaron con un error.",
    description: "Tooltip of the Errored summary",
  },
  "sandboxGroupPages.summaryRunning": {
    message: "En ejecución",
    description: "Summary: sandboxes active now",
  },
  "sandboxGroupPages.summaryRunningTip": {
    message:
      "Creándose, en ejecución, reanudándose o suspendiéndose ahora, sea cual sea el intervalo de tiempo. Cuentan para el límite de concurrencia.",
    description: "Tooltip of the Running summary",
  },
  "sandboxGroupPages.loadRunsError": {
    message: "No se pudieron cargar las ejecuciones de sandboxes",
    description: "The run chart failed to load",
  },
  "sandboxGroupPages.runsChartLabel": {
    message:
      "Ejecuciones de sandboxes finalizadas: {succeeded} correctas, {errored} con error",
    description: "Accessible name of the run chart",
  },
  "sandboxGroupPages.runsSucceeded": {
    message: "Correctas",
    description: "Chart legend: runs that ended without an error",
  },
  "sandboxGroupPages.runsErrored": {
    message: "Con error",
    description: "Chart legend: runs that ended in an error",
  },
  "sandboxGroupPages.selectedCount_one": {
    message: "{count} sandbox seleccionado",
    description: "How many rows are selected",
  },
  "sandboxGroupPages.selectedCount_many": {
    message: "{count} de sandboxes seleccionados",
    description: "How many rows are selected",
  },
  "sandboxGroupPages.selectedCount_other": {
    message: "{count} sandboxes seleccionados",
    description: "How many rows are selected",
  },
  "sandboxGroupPages.terminateSelected": {
    message: "Terminar",
    description: "Button: terminate the selected sandboxes",
  },
  "sandboxGroupPages.terminateTitle_one": {
    message: "¿Terminar {count} sandbox?",
    description: "Terminate confirmation title",
  },
  "sandboxGroupPages.terminateTitle_many": {
    message: "¿Terminar {count} de sandboxes?",
    description: "Terminate confirmation title",
  },
  "sandboxGroupPages.terminateTitle_other": {
    message: "¿Terminar {count} sandboxes?",
    description: "Terminate confirmation title",
  },
  "sandboxGroupPages.terminateDescription_one": {
    message:
      "{count} sandbox se detiene y se pierde todo lo que contenga que no esté en un snapshot.",
    description: "Terminate confirmation body",
  },
  "sandboxGroupPages.terminateDescription_many": {
    message:
      "Los {count} de sandboxes se detienen y se pierde todo lo que contengan que no esté en un snapshot.",
    description: "Terminate confirmation body",
  },
  "sandboxGroupPages.terminateDescription_other": {
    message:
      "Los {count} sandboxes se detienen y se pierde todo lo que contengan que no esté en un snapshot.",
    description: "Terminate confirmation body",
  },
  "sandboxGroupPages.terminateConfirm": {
    message: "Terminar",
    description: "Terminate confirmation button",
  },
  "sandboxGroupPages.terminateFailed_one": {
    message: "No se pudo terminar {count} sandbox. Inténtalo de nuevo.",
    description: "Toast: a terminate failed",
  },
  "sandboxGroupPages.terminateFailed_many": {
    message:
      "No se pudieron terminar {count} de sandboxes. Inténtalo de nuevo.",
    description: "Toast: terminates failed",
  },
  "sandboxGroupPages.terminateFailed_other": {
    message: "No se pudieron terminar {count} sandboxes. Inténtalo de nuevo.",
    description: "Toast: terminates failed",
  },
  "sandboxGroupPages.terminateStarted_one": {
    message: "Terminando {count} sandbox",
    description: "Toast: a terminate was accepted",
  },
  "sandboxGroupPages.terminateStarted_many": {
    message: "Terminando {count} de sandboxes",
    description: "Toast: terminates were accepted",
  },
  "sandboxGroupPages.terminateStarted_other": {
    message: "Terminando {count} sandboxes",
    description: "Toast: terminates were accepted",
  },
  "sandboxGroupPages.terminateSandbox": {
    message: "Terminar sandbox",
    description: "Drawer menu item",
  },
  "sandboxGroupPages.sandboxActions": {
    message: "Acciones del sandbox",
    description: "Accessible name of the drawer's menu",
  },
  "sandboxGroupPages.loadSandboxError": {
    message: "No se pudo cargar el sandbox",
    description: "A sandbox's drawer failed to load",
  },
  "sandboxGroupPages.loadSandboxesError": {
    message: "No se pudieron cargar los sandboxes",
    description: "The list failed to load",
  },
  "sandboxGroupPages.noMatchingSandboxes": {
    message: "No hay sandboxes que coincidan",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.noMatchingSandboxesHelp": {
    message: "Prueba otra búsqueda o amplía el intervalo de tiempo.",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.clearFilters": {
    message: "Borrar filtros",
    description: "Button: reset the search and time range",
  },
  "sandboxGroupPages.firstSandboxTitle": {
    message: "Crea tu primer sandbox",
    description: "Empty state title of the Sandboxes page",
  },
  "sandboxGroupPages.firstSandboxDescription": {
    message:
      "Inicia un sandbox desde la CLI o configura tu código para crear sandboxes mediante programación con los SDK de TypeScript y Python.",
    description: "Empty state body of the Sandboxes page",
  },
  "sandboxGroupPages.starterCli": {
    message: "CLI",
    description: "Starter tab: the command line",
  },
  "sandboxGroupPages.starterSdk": {
    message: "SDK",
    description: "Starter tab: the SDKs",
  },
  "sandboxGroupPages.starterInstallCli": {
    message: "Instala o actualiza a la última versión de la CLI",
    description: "Starter step",
  },
  "sandboxGroupPages.starterHomebrew": {
    message: "Homebrew",
    description: "Install method",
  },
  "sandboxGroupPages.starterDirect": {
    message: "Descarga directa",
    description: "Install method",
  },
  "sandboxGroupPages.starterLogin": {
    message: "Inicia sesión y usa este espacio de trabajo",
    description: "Starter step",
  },
  "sandboxGroupPages.starterCreateSandbox": {
    message: "Crea un sandbox con los valores predeterminados",
    description: "Starter step",
  },
  "sandboxGroupPages.starterApiKey": {
    message: "Configura tu clave de API",
    description: "Starter step",
  },
  "sandboxGroupPages.starterApiKeyHelp": {
    message:
      "Ejecuta esto en la terminal desde la que ejecutarás tu código. El SDK lee tu clave de la variable de entorno RENDER_API_KEY. Sustituye el valor de ejemplo por una clave de API de bex.",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterInstallSdk": {
    message: "Configura un proyecto e instala el SDK",
    description: "Starter step",
  },
  "sandboxGroupPages.starterPythonNeeds": {
    message:
      "Requiere Python 3.10 o posterior. -U actualiza un SDK anterior; la versión 1.4.0 y anteriores no admiten sandboxes.",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterNodeNeeds": {
    message:
      "Requiere Node.js 18 o posterior. @latest también actualiza un SDK anterior que ya esté instalado.",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterFirstSandboxFile": {
    message: "Inicializa tu primer sandbox",
    description: "Starter step",
  },
  "sandboxGroupPages.starterRun": {
    message: "Ejecuta tu código en local",
    description: "Starter step",
  },
  "sandboxGroupPages.starterTypeScript": {
    message: "TypeScript",
    description: "SDK language",
  },
  "sandboxGroupPages.starterPython": {
    message: "Python",
    description: "SDK language",
  },
  "sandboxGroupPages.colSandbox": { message: "Sandbox", description: "Column" },
  "sandboxGroupPages.colStatus": { message: "Estado", description: "Column" },
  "sandboxGroupPages.colCreated": { message: "Creado", description: "Column" },
  "sandboxGroupPages.colDuration": {
    message: "Duración",
    description: "Column",
  },
  "sandboxGroupPages.colPlan": { message: "Plan", description: "Column" },
  "sandboxGroupPages.colSource": {
    message: "Iniciado desde",
    description: "Column: the image or snapshot a sandbox started from",
  },
  "sandboxGroupPages.selectAll": {
    message: "Seleccionar todos los sandboxes de esta página",
    description: "Accessible name of the select-all checkbox",
  },
  "sandboxGroupPages.selectSandbox": {
    message: "Seleccionar {name}",
    description: "Accessible name of a row's checkbox",
  },
  "sandboxGroupPages.baseImage": {
    message: "Imagen base",
    description: "A sandbox started from the default image",
  },
  "sandboxGroupPages.importedImage": {
    message: "Imagen importada",
    description: "A sandbox started from an imported image",
  },
  "sandboxGroupPages.previousPage": {
    message: "Anterior",
    description: "Paging button",
  },
  "sandboxGroupPages.nextPage": {
    message: "Siguiente",
    description: "Paging button",
  },
  "sandboxGroupPages.switchedToRunning": {
    message:
      "El sandbox que seleccionaste se detuvo. Se cambió a uno en ejecución.",
    description: "Toast when the open sandbox stops",
  },
  "sandboxGroupPages.copied": { message: "Copiado", description: "Copy toast" },
  "sandboxGroupPages.copyError": {
    message: "No se pudo copiar",
    description: "Copy failed toast",
  },
  "sandboxGroupPages.copySandboxId": {
    message: "Copiar ID del sandbox",
    description: "Copy button",
  },
  "sandboxGroupPages.copySnapshotId": {
    message: "Copiar ID del snapshot",
    description: "Copy button",
  },
  "sandboxGroupPages.tabEvents": {
    message: "Eventos",
    description: "Drawer tab",
  },
  "sandboxGroupPages.tabSnapshots": {
    message: "Snapshots",
    description: "Drawer tab",
  },
  "sandboxGroupPages.tabDetails": {
    message: "Detalles",
    description: "Drawer tab and section",
  },
  "sandboxGroupPages.tabVersions": {
    message: "Versiones",
    description: "Image drawer tab",
  },
  "sandboxGroupPages.sandboxGoneTitle": {
    message: "No se encontró el sandbox",
    description: "A sandbox that does not exist here",
  },
  "sandboxGroupPages.sandboxGoneDescription": {
    message: "{id} no es un sandbox que puedas ver en este grupo.",
    description: "A sandbox that does not exist here",
  },
  "sandboxGroupPages.loadingEvents": {
    message: "Cargando eventos",
    description: "Events tab loading",
  },
  "sandboxGroupPages.loadEventsError": {
    message:
      "No se pudieron cargar los eventos. Inténtalo de nuevo en un momento.",
    description: "Events tab error",
  },
  "sandboxGroupPages.noEvents": {
    message: "Todavía no hay eventos.",
    description: "Events tab empty",
  },
  "sandboxGroupPages.loadEarlierEvents": {
    message: "Cargar eventos anteriores",
    description: "Button: the next page of events",
  },
  "sandboxGroupPages.loadingEarlierEvents": {
    message: "Cargando eventos anteriores",
    description: "Loading the next page of events",
  },
  "sandboxGroupPages.eventCreated": {
    message: "Creado",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventReady": {
    message: "Listo",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventSuspending": {
    message: "Suspendiendo",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventSuspended": {
    message: "Suspendido",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventResuming": {
    message: "Reanudando",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventTerminated": {
    message: "Terminado",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventErrored": {
    message: "Con error",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventCommand": {
    message: "Comando ({operation})",
    description: "An execution event that ran a command",
  },
  "sandboxGroupPages.eventFileTransfer": {
    message: "Transferencia de archivos ({operation})",
    description: "An execution event that moved a file",
  },
  "sandboxGroupPages.exitCode": {
    message: "Código de salida {code}",
    description: "A command's exit code",
  },
  "sandboxGroupPages.eventSnapshot": {
    message: "Snapshot",
    description: "A snapshot event",
  },
  "sandboxGroupPages.viewSnapshot": {
    message: "Ver snapshot",
    description: "Link to a snapshot's drawer",
  },
  "sandboxGroupPages.triggerSdk": {
    message: "SDK",
    description: "Trigger: the SDK",
  },
  "sandboxGroupPages.triggerCli": {
    message: "CLI",
    description: "Trigger: the CLI",
  },
  "sandboxGroupPages.triggerApi": {
    message: "API",
    description: "Trigger: the API",
  },
  "sandboxGroupPages.triggerDashboard": {
    message: "Panel",
    description: "Trigger: the dashboard",
  },
  "sandboxGroupPages.triggerIdleTimeout": {
    message: "Tiempo de inactividad",
    description: "Trigger: the idle timeout",
  },
  "sandboxGroupPages.triggerSystem": {
    message: "bex",
    description: "Trigger: the platform itself",
  },
  "sandboxGroupPages.triggerConnection": {
    message: "Nuevo comando o transferencia de archivos",
    description: "Trigger: a resume a command started",
  },
  "sandboxGroupPages.loading": {
    message: "Cargando",
    description: "Loading text",
  },
  "sandboxGroupPages.loadSnapshotsError": {
    message: "No se pudieron cargar los snapshots",
    description: "Snapshots failed to load",
  },
  "sandboxGroupPages.noSnapshotsRunning": {
    message:
      "Este sandbox todavía no tiene snapshots. Captura uno con la CLI o un SDK.",
    description: "Snapshots tab of a running sandbox",
  },
  "sandboxGroupPages.noSnapshotsEnded": {
    message: "No se tomaron snapshots de este sandbox.",
    description: "Snapshots tab of an ended sandbox",
  },
  "sandboxGroupPages.detailSandboxId": {
    message: "ID del sandbox",
    description: "Detail label",
  },
  "sandboxGroupPages.detailGroup": {
    message: "Grupo",
    description: "Detail label",
  },
  "sandboxGroupPages.detailPlan": {
    message: "Plan",
    description: "Detail label",
  },
  "sandboxGroupPages.planSize": {
    message: "{plan} ({cpus} CPU, {memory} GB de memoria)",
    description: "A plan with its size",
  },
  "sandboxGroupPages.detailCreated": {
    message: "Creado",
    description: "Detail label",
  },
  "sandboxGroupPages.detailEnded": {
    message: "Finalizado",
    description: "Detail label",
  },
  "sandboxGroupPages.detailDuration": {
    message: "Duración",
    description: "Detail label",
  },
  "sandboxGroupPages.detailTimeout": {
    message: "Tiempo de espera",
    description: "Detail label",
  },
  "sandboxGroupPages.detailIdleTimeout": {
    message: "Tiempo de inactividad",
    description: "Detail label",
  },
  "sandboxGroupPages.off": {
    message: "Desactivado",
    description: "A timeout that is off",
  },
  "sandboxGroupPages.detailNetwork": {
    message: "Red",
    description: "Detail label",
  },
  "sandboxGroupPages.networkAllowAll": {
    message: "Permitir todo",
    description: "Network policy",
  },
  "sandboxGroupPages.networkDenyAll": {
    message: "Denegar todo",
    description: "Network policy",
  },
  "sandboxGroupPages.networkAllowList": {
    message: "Lista de permitidos",
    description: "Network policy",
  },
  "sandboxGroupPages.detailRegion": {
    message: "Región",
    description: "Detail label",
  },
  "sandboxGroupPages.detailSource": {
    message: "Iniciado desde",
    description: "Detail label",
  },
  "sandboxGroupPages.goToSnapshot": {
    message: "Ir al snapshot",
    description: "Link to the snapshot a sandbox started from",
  },
  "sandboxGroupPages.goToImage": {
    message: "Ir a la imagen",
    description: "Link to the image a sandbox started from",
  },
  "sandboxGroupPages.detailTrigger": {
    message: "Origen",
    description: "Detail label: what started the sandbox",
  },
  "sandboxGroupPages.detailUser": {
    message: "Usuario",
    description: "Detail label: who started the sandbox",
  },
  "sandboxGroupPages.detailError": {
    message: "Error",
    description: "Detail label",
  },
  "sandboxGroupPages.errorUnknown": {
    message:
      "bex no pudo determinar por qué falló este sandbox. Inténtalo de nuevo o contacta con soporte indicando el ID del sandbox.",
    description: "An errored sandbox with no known reason",
  },
  "sandboxGroupPages.snapshotCapturing": {
    message: "Capturando",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotAvailable": {
    message: "Disponible",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotFailed": {
    message: "Fallido",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotFailureInternal": {
    message: "Un error interno detuvo la captura.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureTimeout": {
    message: "Se agotó el tiempo de esta captura.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureSourceTerminated": {
    message:
      "El sandbox de origen se terminó antes de que finalizara la captura.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureSourceErrored": {
    message: "El sandbox de origen falló antes de que finalizara la captura.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailedHelp": {
    message:
      "Crea un snapshot nuevo para intentarlo otra vez. Los sandboxes nuevos no pueden iniciarse desde este.",
    description: "After a snapshot failure",
  },
  "sandboxGroupPages.noMatchingSnapshots": {
    message: "No hay snapshots que coincidan",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.firstSnapshotTitle": {
    message: "Crea tu primer snapshot",
    description: "Empty state title of the Snapshots page",
  },
  "sandboxGroupPages.firstSnapshotDescription": {
    message:
      "Captura un snapshot para guardar los archivos de un sandbox. Inicia sandboxes nuevos desde el mismo punto sin tener que configurarlo todo otra vez.",
    description: "Empty state body of the Snapshots page",
  },
  "sandboxGroupPages.firstSnapshotNeedsRunning": {
    message: "Necesitas un sandbox en ejecución para crear un snapshot.",
    description: "Empty state note",
  },
  "sandboxGroupPages.starterSnapshotSandbox": {
    message: "Crea un snapshot de un sandbox en ejecución",
    description: "Starter step",
  },
  "sandboxGroupPages.starterStartFromSnapshot": {
    message: "Crea sandboxes a partir del snapshot",
    description: "Starter step",
  },
  "sandboxGroupPages.starterSnapshotCode": {
    message: "Configura tu código",
    description: "Starter step",
  },
  "sandboxGroupPages.colSnapshot": {
    message: "Snapshot",
    description: "Column",
  },
  "sandboxGroupPages.colSourceSandbox": {
    message: "Sandbox de origen",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colSize": {
    message: "Tamaño",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colCaptured": {
    message: "Capturado",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colExpires": {
    message: "Caduca",
    description: "Column and detail label",
  },
  "sandboxGroupPages.never": {
    message: "Nunca",
    description: "A snapshot that never expires",
  },
  "sandboxGroupPages.snapshotGoneTitle": {
    message: "No se encontró el snapshot",
    description: "A snapshot that does not exist here",
  },
  "sandboxGroupPages.snapshotGoneDescription": {
    message:
      "{id} no es un snapshot que puedas ver en este espacio de trabajo. Puede que se haya eliminado o que haya caducado.",
    description: "A snapshot that does not exist here",
  },
  "sandboxGroupPages.detailType": {
    message: "Tipo",
    description: "Detail label: snapshot kind",
  },
  "sandboxGroupPages.kindFilesystem": {
    message: "Sistema de archivos",
    description: "Snapshot kind",
  },
  "sandboxGroupPages.kindRuntime": {
    message: "Runtime",
    description: "Snapshot kind",
  },
  "sandboxGroupPages.detailResourceConstraint": {
    message: "Restricción de recursos",
    description: "Detail label: the plan a snapshot needs",
  },
  "sandboxGroupPages.detailLifecycle": {
    message: "Ciclo de vida",
    description: "Detail label",
  },
  "sandboxGroupPages.lifecycleKept": {
    message: "Se conserva hasta que se elimine",
    description: "A snapshot with no time to live",
  },
  "sandboxGroupPages.lifecycleAfterUse": {
    message: "Caduca {days} días después de su último uso",
    description: "A snapshot's time to live",
  },
  "sandboxGroupPages.noSandboxesFromSnapshot": {
    message: "Ningún sandbox se ha iniciado desde este snapshot.",
    description: "Snapshot drawer's sandboxes",
  },
  "sandboxGroupPages.restoreTitle": {
    message: "Inicia un sandbox desde este snapshot",
    description: "Snapshot drawer starter code",
  },
  "sandboxGroupPages.deleteSnapshot": {
    message: "Eliminar snapshot",
    description: "Button",
  },
  "sandboxGroupPages.deleteSnapshotTitle": {
    message: "¿Eliminar el snapshot?",
    description: "Confirmation title",
  },
  "sandboxGroupPages.deleteSnapshotDescription": {
    message:
      "Esto elimina {name} de forma permanente. Los sandboxes nuevos no podrán iniciarse desde él. Los sandboxes que ya se iniciaron desde él siguen en ejecución.",
    description: "Confirmation body",
  },
  "sandboxGroupPages.deleteSnapshotError": {
    message: "No se pudo eliminar el snapshot. Inténtalo de nuevo.",
    description: "Toast",
  },
  "sandboxGroupPages.snapshotDeleted": {
    message: "Snapshot {name} eliminado",
    description: "Toast",
  },
  "sandboxGroupPages.imageQueued": {
    message: "En cola",
    description: "Image status",
  },
  "sandboxGroupPages.imageImporting": {
    message: "Importando",
    description: "Image status",
  },
  "sandboxGroupPages.imageAvailable": {
    message: "Disponible",
    description: "Image status",
  },
  "sandboxGroupPages.imageUnavailable": {
    message: "No disponible",
    description: "Image status: no version can start a sandbox",
  },
  "sandboxGroupPages.imageFailed": {
    message: "Fallida",
    description: "Image version status",
  },
  "sandboxGroupPages.imageLatest": {
    message: "Más reciente",
    description: "The newest available version of an image",
  },
  "sandboxGroupPages.imageImportingHelp": {
    message:
      "Los sandboxes se inician desde la versión disponible más reciente hasta que esta termine.",
    description: "A version still importing",
  },
  "sandboxGroupPages.imageFailureTimeout": {
    message: "La importación tardó demasiado.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureRetries": {
    message: "La importación falló tras varios intentos.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureCredentials": {
    message:
      "bex no pudo descargar la imagen. Revisa las credenciales del registro.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureNotFound": {
    message:
      "bex no encontró la imagen. Revisa el nombre y la etiqueta de la imagen.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailurePull": {
    message: "bex no pudo descargar la imagen.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailurePlatform": {
    message:
      "La imagen es para {platform}. Los sandboxes solo admiten linux/amd64.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureMissing": {
    message:
      "A la imagen le falta {what}, que los sandboxes necesitan para iniciarse. Usa una imagen con shell, como una variante slim o Alpine.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureConvert": {
    message: "bex no pudo convertir la imagen. Intenta importarla de nuevo.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureLayers": {
    message: "bex no pudo leer las capas de la imagen.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureStart": {
    message: "No se pudo iniciar la importación.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureStopped": {
    message: "La importación se detuvo antes de terminar.",
    description: "Image failure",
  },
  "sandboxGroupPages.loadImagesError": {
    message: "No se pudieron cargar las imágenes",
    description: "Images failed to load",
  },
  "sandboxGroupPages.noMatchingImages": {
    message: "No hay imágenes que coincidan",
    description: "Empty state with a search",
  },
  "sandboxGroupPages.firstImageTitle": {
    message: "Importa tu primera imagen",
    description: "Empty state title of the Custom images page",
  },
  "sandboxGroupPages.firstImageDescription": {
    message:
      "Importa una imagen desde un registro para que los sandboxes puedan iniciarse desde ella. Impórtala con la CLI o el SDK y luego inicia sandboxes desde ella.",
    description: "Empty state body of the Custom images page",
  },
  "sandboxGroupPages.starterImportImage": {
    message: "Importa una imagen (linux/amd64)",
    description: "Starter step",
  },
  "sandboxGroupPages.starterStartFromImage": {
    message: "Crea sandboxes a partir de la imagen",
    description: "Starter step",
  },
  "sandboxGroupPages.starterImportCode": {
    message: "Importa imágenes",
    description: "Starter step",
  },
  "sandboxGroupPages.colImage": { message: "Imagen", description: "Column" },
  "sandboxGroupPages.colSourceImage": {
    message: "Origen",
    description: "Column: the registry image",
  },
  "sandboxGroupPages.colLatestAvailable": {
    message: "Última disponible",
    description: "Column: the newest available version's digest",
  },
  "sandboxGroupPages.colUpdated": {
    message: "Actualizada",
    description: "Column: the newest import",
  },
  "sandboxGroupPages.imageGoneTitle": {
    message: "No se encontró la imagen",
    description: "An image name with no versions",
  },
  "sandboxGroupPages.imageGoneDescription": {
    message: "Este grupo no tiene ninguna imagen llamada {name}.",
    description: "An image name with no versions",
  },
  "sandboxGroupPages.detailLatestAvailable": {
    message: "Última versión disponible",
    description: "Detail label",
  },
  "sandboxGroupPages.detailImageSource": {
    message: "Origen",
    description: "Detail label: the registry",
  },
  "sandboxGroupPages.detailImage": {
    message: "Imagen",
    description: "Detail label",
  },
  "sandboxGroupPages.detailDigest": {
    message: "Digest",
    description: "Detail label",
  },
  "sandboxGroupPages.detailVersionId": {
    message: "ID de versión",
    description: "Detail label",
  },
  "sandboxGroupPages.noSandboxesFromImage": {
    message: "Ningún sandbox se ha iniciado desde esta imagen.",
    description: "Image drawer's sandboxes",
  },
  "sandboxGroupPages.general": {
    message: "General",
    description: "Settings section",
  },
  "sandboxGroupPages.makeDefault": {
    message: "Establecer como predeterminado",
    description: "Button: make this the workspace's default group",
  },
  "sandboxGroupPages.madeDefault": {
    message: "{name} es ahora el grupo de sandbox predeterminado",
    description: "Toast",
  },
  "sandboxGroupPages.save": { message: "Guardar", description: "Button" },
  "sandboxGroupPages.limitSaved": {
    message: "Límite de concurrencia guardado",
    description: "Toast",
  },
  "sandboxGroupPages.brokeredCredentials": {
    message: "Credenciales intermediadas",
    description: "What a group delete removes",
  },
  "sandboxGroupPages.deleteGroup": {
    message: "Eliminar grupo de sandbox",
    description: "Button and section",
  },
  "sandboxGroupPages.deleteGroupHelp": {
    message:
      "Elimina este grupo, con sus sandboxes, snapshots, imágenes y credenciales.",
    description: "Delete section help",
  },
  "sandboxGroupPages.deleteGroupDefault": {
    message:
      "El grupo predeterminado del espacio de trabajo no se puede eliminar. Primero establece otro grupo como predeterminado.",
    description: "Delete section help for the default group",
  },
  "sandboxGroupPages.deleteGroupTitle": {
    message: "¿Eliminar el grupo de sandbox?",
    description: "Confirmation title",
  },
  "sandboxGroupPages.deleteGroupDescription": {
    message:
      "Este grupo de sandbox, {name}, se eliminará de forma permanente, junto con su historial de ejecuciones, sus logs y su configuración:",
    description: "Confirmation body",
  },
  "sandboxGroupPages.deleteGroupStops": {
    message:
      "Todo lo que use este grupo mediante el SDK, la CLI o la API dejará de funcionar.",
    description: "Confirmation body",
  },
  "sandboxGroupPages.groupDeleted": {
    message: "Grupo de sandbox {name} eliminado",
    description: "Toast",
  },
  "sandboxGroupPages.linkLoadError": {
    message: "No se pudo abrir el enlace. Inténtalo de nuevo en un momento.",
    description: "A link by id failed to resolve",
  },
  "sandboxGroupPages.backToGroups": {
    message: "Ir a los grupos de sandbox",
    description: "Button",
  },
};

export default es;
