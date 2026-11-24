import type { TranslationEntry } from "@/i18n";

const esBuildSources: Record<string, TranslationEntry> = {
  "buildSources.pageTitle": {
    message: "Fuentes de compilación",
    description: "Title of the Build Sources list page",
  },
  "buildSources.pageDescription": {
    message:
      "Compila una vez desde un repositorio Git o una imagen Docker y despliega el artefacto en varios servicios.",
    description: "Subtitle of the Build Sources list page",
  },
  "buildSources.resourceType": {
    message: "Fuente de compilación",
    description: "Singular resource type label shown in global search results",
  },
  "buildSources.newButton": {
    message: "Nueva fuente de compilación",
    description: "Primary CTA button to create a new Build Source",
  },
  "buildSources.backToList": {
    message: "Volver a fuentes de compilación",
    description: "Back button label returning to /build-sources",
  },
  "buildSources.viewAll": {
    message: "Ver todas",
    description: "Link from Project Overview Build Sources section to /build-sources",
  },
  "buildSources.viewBuildSources": {
    message: "Ver fuentes de compilación",
    description: "Button in partial-link alert navigating to /build-sources",
  },
  "buildSources.emptyTitle": {
    message: "Aún no hay fuentes de compilación",
    description: "Empty state heading when the workspace has no build sources",
  },
  "buildSources.emptyDescription": {
    message:
      "Crea una fuente de compilación para compartir una compilación Git o imagen Docker entre Web Services, Private Services, Background Workers y Cron Jobs.",
    description: "Empty state explanation on /build-sources",
  },
  "buildSources.refreshError": {
    message:
      "No se pudieron actualizar las fuentes de compilación. Mostrando resultados en caché.",
    description: "Inline error banner when a background refresh fails over cached rows",
  },
  "buildSources.searchPlaceholder": {
    message: "Buscar por nombre, repositorio, imagen o proyecto…",
    description: "Placeholder for the /build-sources search input",
  },
  "buildSources.searchLabel": {
    message: "Buscar fuentes de compilación",
    description: "Accessible label for the /build-sources search input",
  },
  "buildSources.resetFilters": {
    message: "Restablecer filtros",
    description: "Button to clear active search and kind filters on /build-sources",
  },
  "buildSources.filterKindLabel": {
    message: "Filtrar por tipo de fuente",
    description: "Accessible label for the source-kind filter button group",
  },
  "buildSources.filterAll": {
    message: "Todas",
    description: "Filter button showing both Git and Docker image build sources",
  },
  "buildSources.kindGit": {
    message: "Git",
    description: "Source kind badge and filter label for Git-backed build sources",
  },
  "buildSources.kindImage": {
    message: "Imagen Docker",
    description: "Source kind badge and filter label for Docker-image-backed build sources",
  },
  "buildSources.noMatchesTitle": {
    message: "No hay fuentes de compilación coincidentes",
    description: "Heading when search or kind filter matches zero build sources",
  },
  "buildSources.noMatchesDescription": {
    message: "Prueba a ajustar tu búsqueda o borrar el filtro de tipo.",
    description: "Body copy when search or kind filter matches zero build sources",
  },
  "buildSources.colName": {
    message: "Nombre",
    description: "Table column header for Build Source name",
  },
  "buildSources.colSource": {
    message: "Fuente",
    description: "Table column header for Git repo/branch or Docker image",
  },
  "buildSources.colRegion": {
    message: "Región",
    description: "Table column header for Build Source region",
  },
  "buildSources.colLinkedServices": {
    message: "Servicios vinculados",
    description: "Table column header for linked service count",
  },
  "buildSources.colProject": {
    message: "Proyecto",
    description: "Table column header for optional project scope",
  },
  "buildSources.colUpdated": {
    message: "Actualizado",
    description: "Table column header for last updated timestamp",
  },
  "buildSources.copyId": {
    message: "Copiar ID de la fuente de compilación",
    description: "Accessible label for copying a bds- ID",
  },
  "buildSources.copied": {
    message: "Copiado",
    description: "Copy button confirmation tooltip",
  },
  "buildSources.copyError": {
    message: "No se pudo copiar",
    description: "Copy button failure tooltip",
  },
  "buildSources.linkedServicesCount_one": {
    message: "{count} servicio",
    description: "Singular count of services linked to a build source",
  },
  "buildSources.linkedServicesCount_many": {
    message: "{count} de servicios",
    description: "Many count of services linked to a build source",
  },
  "buildSources.linkedServicesCount_other": {
    message: "{count} servicios",
    description: "Plural count of services linked to a build source",
  },
  "buildSources.workspaceScope": {
    message: "Espacio de trabajo",
    description: "Label when a build source is workspace-wide (not scoped to a project)",
  },
  "buildSources.createTitle": {
    message: "Nueva fuente de compilación",
    description: "Page heading of /build-sources/new",
  },
  "buildSources.createDescription": {
    message:
      "Configura una compilación Git o una imagen Docker compartida desde la que varios servicios puedan desplegar.",
    description: "Subtitle of /build-sources/new",
  },
  "buildSources.createSubmit": {
    message: "Crear fuente de compilación",
    description: "Submit button label on /build-sources/new",
  },
  "buildSources.createSuccess": {
    message: 'Fuente de compilación "{name}" creada',
    description: "Toast shown after creating a Build Source",
  },
  "buildSources.createError": {
    message: "No se pudo crear la fuente de compilación. Inténtalo de nuevo.",
    description: "Fallback error toast when createBuildSource fails",
  },
  "buildSources.updateSuccess": {
    message: 'Fuente de compilación "{name}" actualizada',
    description: "Toast shown after updating a Build Source",
  },
  "buildSources.updateError": {
    message: "No se pudo actualizar la fuente de compilación. Inténtalo de nuevo.",
    description: "Fallback error toast when updateBuildSource fails",
  },
  "buildSources.deleteSuccess": {
    message: 'Fuente de compilación "{name}" eliminada',
    description: "Toast shown after deleting a Build Source",
  },
  "buildSources.deleteError": {
    message: "No se pudo eliminar la fuente de compilación. Inténtalo de nuevo.",
    description: "Fallback error toast when deleteBuildSource fails",
  },
  "buildSources.triggerBuildSuccess": {
    message: "Compilación iniciada",
    description: "Toast shown after triggering a Build Source build",
  },
  "buildSources.triggerBuildError": {
    message: "No se pudo iniciar la compilación. Inténtalo de nuevo.",
    description: "Fallback error toast when triggerBuild fails",
  },
  "buildSources.createPermissionDenied": {
    message:
      "Necesitas permisos de Developer o Admin en este espacio de trabajo para crear una fuente de compilación.",
    description: "Read-only notice shown on /build-sources/new when can_create is false",
  },
  "buildSources.createBlockedSource": {
    message: "Elige un repositorio Git o una imagen Docker para continuar.",
    description: "Action bar hint while no source is selected",
  },
  "buildSources.partiallyLinkedTitle": {
    message: "Fuente de compilación creada con vínculos parciales",
    description: "Alert heading when createBuildSource returns 409 BUILD_SOURCE_PARTIALLY_LINKED",
  },
  "buildSources.partiallyLinkedDescription": {
    message:
      "Se creó la fuente de compilación {id}, pero no se pudieron vincular todos los servicios seleccionados.",
    description: "Alert summary naming the newly created bds- ID on partial link conflict",
  },
  "buildSources.partiallyLinkedSucceeded": {
    message: "Vinculados: {services}",
    description: "List of services that were successfully linked before the conflict",
  },
  "buildSources.partiallyLinkedFailed": {
    message: "Error al vincular: {service}",
    description: "The service whose link failed during create",
  },
  "buildSources.fieldSource": {
    message: "Fuente",
    description: "Create form label for Git repository vs Docker image picker",
  },
  "buildSources.fieldSourceDescription": {
    message:
      "Elige un repositorio de GitHub conectado, una URL pública de Git o una imagen Docker existente.",
    description: "Help text for the source picker row on /build-sources/new",
  },
  "buildSources.fieldName": {
    message: "Nombre",
    description: "Create form label for Build Source name",
  },
  "buildSources.fieldNameDescription": {
    message: "Nombre único dentro de este espacio de trabajo.",
    description: "Help text for Build Source name",
  },
  "buildSources.fieldNamePlaceholder": {
    message: "my-shared-api",
    description: "Placeholder for Build Source name input",
  },
  "buildSources.errorNameTaken": {
    message:
      'Ya existe una fuente de compilación llamada "{name}" en este espacio de trabajo.',
    description: "Inline error when BUILD_SOURCE_NAME_TAKEN is returned",
  },
  "buildSources.fieldProject": {
    message: "Proyecto",
    description: "Create form label for optional Project scope",
  },
  "buildSources.fieldProjectDescription": {
    message: "Opcionalmente limita esta fuente de compilación a un solo proyecto.",
    description: "Help text for optional Project selector",
  },
  "buildSources.fieldProjectNone": {
    message: "Ninguno (todo el espacio de trabajo)",
    description: "Option leaving the build source unscoped at workspace level",
  },
  "buildSources.fieldRuntime": {
    message: "Entorno de ejecución",
    description: "Create form label for Git build source runtime",
  },
  "buildSources.fieldRuntimeDescription": {
    message: "Elige un entorno nativo o Docker.",
    description: "Help text for Git build source runtime selector",
  },
  "buildSources.fieldBranch": {
    message: "Rama",
    description: "Create form label for Git branch",
  },
  "buildSources.fieldBranchDescription": {
    message:
      "Rama Git desde la que compilar. Por defecto usa la rama principal del repositorio.",
    description: "Help text for Git branch input",
  },
  "buildSources.fieldRegion": {
    message: "Región",
    description: "Create form label for Git build source region",
  },
  "buildSources.fieldRegionDescription": {
    message:
      "Las fuentes Git deben coincidir con la región de los servicios vinculados.",
    description: "Help text for Git build source region selector",
  },
  "buildSources.fieldRegionDefault": {
    message: "Región predeterminada del espacio de trabajo",
    description: "Option leaving the build source region unset to use the platform default",
  },
  "buildSources.fieldGitRegistryDescription": {
    message:
      "Credencial opcional para descargar imágenes base privadas durante compilaciones Docker.",
    description: "Help text for registry credential on Docker Git build sources",
  },
  "buildSources.fieldEnvGroups": {
    message: "Grupos de entorno en compilación",
    description: "Label for selecting workspace env groups in build-time environment",
  },
  "buildSources.fieldEnvGroupsDescription": {
    message:
      "Vincula grupos de entorno a nivel de espacio de trabajo en tiempo de compilación. Los grupos limitados a un entorno no son elegibles.",
    description: "Help text for build-time env group checkboxes",
  },
  "buildSources.fieldEnvGroupsEmpty": {
    message: "No hay grupos de entorno a nivel de espacio de trabajo.",
    description: "Empty message when no workspace-level env groups exist",
  },
  "buildSources.linkServicesTitle": {
    message: "Vincular servicios existentes",
    description: "Section heading for optional initial serviceIds linking",
  },
  "buildSources.linkServicesDescription": {
    message:
      "Vincula opcionalmente Web Services, Private Services, Background Workers o Cron Jobs existentes. Los Static Sites no admiten fuentes de compilación.",
    description: "Section description for optional initial serviceIds linking",
  },
  "buildSources.linkServicesEmpty": {
    message: "Aún no hay servicios vinculables en este espacio de trabajo.",
    description: "Empty message when workspace has no linkable services",
  },
  "buildSources.projectSectionTitle": {
    message: "Fuentes de compilación",
    description: "Heading of the Build Sources section on Project Overview",
  },
  "buildSources.projectLoadError": {
    message: "No se pudieron cargar las fuentes de compilación de este proyecto.",
    description: "Error text inside Project Overview Build Sources section",
  },
  "buildSources.projectEmptyDescription": {
    message: "Aún no hay fuentes de compilación asociadas a este proyecto.",
    description: "Empty state text inside Project Overview Build Sources section",
  },
  "buildSources.viewBuildSource": {
    message: "Abrir fuente de compilación",
    description: "Button navigating to a newly created Build Source detail page",
  },
  "buildSources.detailTabsLabel": {
    message: "Secciones de la fuente de compilación",
    description: "Accessible label for the Build Source detail tab navigation",
  },
  "buildSources.tabBuilds": {
    message: "Compilaciones",
    description: "Tab label for Build Source build history",
  },
  "buildSources.tabServices": {
    message: "Servicios",
    description: "Tab label for Build Source linked services",
  },
  "buildSources.tabEnvironment": {
    message: "Entorno",
    description: "Tab label for Build Source build-time environment",
  },
  "buildSources.tabSettings": {
    message: "Configuración",
    description: "Tab label for Build Source settings and danger zone",
  },
  "buildSources.triggerBuildButton": {
    message: "Iniciar compilación",
    description: "Primary CTA to trigger a new build on a Git build source",
  },
  "buildSources.triggerSubmitting": {
    message: "Iniciando…",
    description: "Submit button label while triggerBuild mutation is in flight",
  },
  "buildSources.triggerDialogTitle": {
    message: "Iniciar compilación",
    description: "Title of the Trigger Build modal dialog",
  },
  "buildSources.triggerDialogDescription": {
    message: 'Inicia una nueva compilación para "{name}" y elige qué servicios vinculados desplegarán el resultado.',
    description: "Description of the Trigger Build modal dialog",
  },
  "buildSources.triggerScopeLegend": {
    message: "Desplegar tras completar la compilación",
    description: "Legend for the deploy.serviceScope radio group",
  },
  "buildSources.triggerScopeAutoDeploy": {
    message: "Desplegar automáticamente los servicios vinculados",
    description: "Radio option for deploy.serviceScope=autodeploy_only",
  },
  "buildSources.triggerScopeAutoDeployHint": {
    message: "Despliega todos los servicios vinculados con despliegue automático activado.",
    description: "Hint text for deploy.serviceScope=autodeploy_only",
  },
  "buildSources.triggerScopeNone": {
    message: "Solo compilar (sin desplegar)",
    description: "Radio option for deploy.serviceScope=none",
  },
  "buildSources.triggerScopeNoneHint": {
    message: "Genera el artefacto de compilación sin desplegar ningún servicio vinculado.",
    description: "Hint text for deploy.serviceScope=none",
  },
  "buildSources.triggerScopeCustom": {
    message: "Servicios seleccionados",
    description: "Radio option for deploy.serviceScope=custom",
  },
  "buildSources.triggerScopeCustomHint": {
    message: "Elige qué servicios vinculados desplegar cuando finalice la compilación.",
    description: "Hint text for deploy.serviceScope=custom",
  },
  "buildSources.triggerScopeCustomNoServices": {
    message: "Todavía no hay servicios vinculados a esta fuente de compilación.",
    description: "Message inside custom deploy scope when serviceLinks is empty",
  },
  "buildSources.triggerScopeCustomSelectOne": {
    message: "Selecciona al menos un servicio para desplegar.",
    description: "Validation hint when custom scope has zero checked services",
  },
  "buildSources.triggerClearCache": {
    message: "Limpiar caché de compilación",
    description: "Checkbox label for clearCache=clear",
  },
  "buildSources.triggerClearCacheHint": {
    message: "Compila desde cero sin reutilizar capas en caché.",
    description: "Hint text for clearCache checkbox",
  },
  "buildSources.triggerResultTitle": {
    message: "Compilación iniciada",
    description: "Confirmation heading in Trigger Build dialog after success",
  },
  "buildSources.triggerResultDescription": {
    message: "La compilación {id} se ha puesto en cola.",
    description: "Confirmation summary naming the new bld- ID",
  },
  "buildSources.triggerResultDeploysTitle_one": {
    message: "{count} despliegue de servicio en cola",
    description: "Singular heading listing fan-out deploys queued by triggerBuild",
  },
  "buildSources.triggerResultDeploysTitle_many": {
    message: "{count} de despliegues de servicio en cola",
    description: "Many heading listing fan-out deploys queued by triggerBuild",
  },
  "buildSources.triggerResultDeploysTitle_other": {
    message: "{count} despliegues de servicio en cola",
    description: "Plural heading listing fan-out deploys queued by triggerBuild",
  },
  "buildSources.triggerResultNoDeploys": {
    message: "No se puso en cola ningún despliegue de servicio para esta compilación.",
    description: "Message when triggerBuild returned an empty deploys array",
  },
  "buildSources.triggerResultSkippedTitle_one": {
    message: "{count} servicio vinculado omitido",
    description: "Singular alert heading when triggerBuild returns skippedServices",
  },
  "buildSources.triggerResultSkippedTitle_many": {
    message: "{count} de servicios vinculados omitidos",
    description: "Many alert heading when triggerBuild returns skippedServices",
  },
  "buildSources.triggerResultSkippedTitle_other": {
    message: "{count} servicios vinculados omitidos",
    description: "Plural alert heading when triggerBuild returns skippedServices",
  },
  "buildSources.triggerResultViewLogs": {
    message: "Ver registros de compilación",
    description: "Primary action navigating from Trigger Build confirmation to Build Detail",
  },
  "buildSources.skippedReasonSuspended": {
    message: "Suspendido",
    description: "Badge label when a service was skipped because it is suspended",
  },
  "buildSources.buildStatusSucceeded": {
    message: "Completada",
    description: "Build status badge for succeeded builds",
  },
  "buildSources.buildStatusFailed": {
    message: "Fallida",
    description: "Build status badge for failed builds",
  },
  "buildSources.buildStatusCanceled": {
    message: "Cancelada",
    description: "Build status badge for canceled builds",
  },
  "buildSources.buildStatusInProgress": {
    message: "Compilando",
    description: "Build status badge for in-progress builds",
  },
  "buildSources.buildStatusCreated": {
    message: "En cola",
    description: "Build status badge for newly queued builds",
  },
  "buildSources.buildStatusUnknown": {
    message: "Desconocido",
    description: "Fallback build status badge",
  },
  "buildSources.buildsEmptyTitle": {
    message: "Todavía no hay compilaciones",
    description: "Empty state heading on the Builds tab",
  },
  "buildSources.buildsEmptyGitDescription": {
    message: "Inicia una compilación manualmente o haz push a la rama rastreada para crear tu primer artefacto.",
    description: "Empty state description on Builds tab for Git build sources",
  },
  "buildSources.buildsEmptyImageDescription": {
    message: "Las fuentes de imagen Docker registran revisiones de imagen cuando se despliegan los servicios vinculados.",
    description: "Empty state description on Builds tab for image build sources",
  },
  "buildSources.buildsRefreshError": {
    message: "No se pudieron actualizar las compilaciones. Se muestran resultados en caché.",
    description: "Inline error banner when polling builds fails over cached rows",
  },
  "buildSources.buildsLoadMore": {
    message: "Cargar más compilaciones",
    description: "Pagination button loading older builds",
  },
  "buildSources.colBuildId": {
    message: "ID de compilación",
    description: "Table header for build ID column",
  },
  "buildSources.colStatus": {
    message: "Estado",
    description: "Table header for build status column",
  },
  "buildSources.colArtifact": {
    message: "Commit / Imagen",
    description: "Table header for commit SHA or image digest column",
  },
  "buildSources.colDuration": {
    message: "Duración",
    description: "Table header for build duration column",
  },
  "buildSources.colDeploys": {
    message: "Despliegues",
    description: "Table header for fan-out deploys count column",
  },
  "buildSources.colStarted": {
    message: "Iniciada",
    description: "Table header for build start timestamp column",
  },
  "buildSources.buildDeploysCount_one": {
    message: "{count} despliegue",
    description: "Singular badge count of fan-out deploys on a build",
  },
  "buildSources.buildDeploysCount_many": {
    message: "{count} de despliegues",
    description: "Many badge count of fan-out deploys on a build",
  },
  "buildSources.buildDeploysCount_other": {
    message: "{count} despliegues",
    description: "Plural badge count of fan-out deploys on a build",
  },
  "buildSources.buildSkippedCount_one": {
    message: "{count} omitido",
    description: "Singular badge count of skipped services on a build",
  },
  "buildSources.buildSkippedCount_many": {
    message: "{count} de omitidos",
    description: "Many badge count of skipped services on a build",
  },
  "buildSources.buildSkippedCount_other": {
    message: "{count} omitidos",
    description: "Plural badge count of skipped services on a build",
  },
  "buildSources.backToBuilds": {
    message: "Volver a Compilaciones",
    description: "Back link from Build Detail to the Builds tab",
  },
  "buildSources.copyBuildId": {
    message: "Copiar ID de compilación",
    description: "Accessible label for copying a bld- ID",
  },
  "buildSources.buildDetailSubtitle": {
    message: "Compilación de {name}",
    description: "Subtitle under the bld- ID on Build Detail",
  },
  "buildSources.buildDetailRefreshError": {
    message: "No se pudieron actualizar los detalles de la compilación. Se muestra el último estado cargado.",
    description: "Inline error banner when build detail polling fails",
  },
  "buildSources.factStarted": {
    message: "Iniciada",
    description: "Fact label for build start timestamp",
  },
  "buildSources.factCommit": {
    message: "Confirmación",
    description: "Fact label for build commit SHA",
  },
  "buildSources.factArtifact": {
    message: "Artefacto",
    description: "Fact label for built image reference and digest",
  },
  "buildSources.fanoutTitle": {
    message: "Despliegues de servicios vinculados",
    description: "Card heading listing fan-out deploys on Build Detail",
  },
  "buildSources.fanoutEmpty": {
    message: "Todavía no se ha desplegado ningún servicio desde esta compilación.",
    description: "Empty message inside fan-out deploys card",
  },
  "buildSources.buildLogsSearchLabel": {
    message: "Filtrar registros de compilación",
    description: "Accessible label for search input in BuildLogPanel",
  },
  "buildSources.buildLogsEmptyTitle": {
    message: "Todavía no hay registros de compilación",
    description: "Empty state heading in BuildLogPanel",
  },
  "buildSources.buildLogsEmptyBody": {
    message: "Los registros de esta compilación aparecerán aquí en cuanto comience a generar salida.",
    description: "Empty state body in BuildLogPanel",
  },
  "buildSources.imageBuildLogsNotice": {
    message: "Las fuentes de imagen Docker reutilizan una imagen existente y no generan registros de compilación.",
    description: "Notice shown on Build Detail for image-backed build sources",
  },
  "buildSources.servicesTabTitle": {
    message: "Servicios vinculados",
    description: "Heading of the Linked Services tab",
  },
  "buildSources.servicesTabDescription": {
    message: "Los servicios vinculados a esta fuente de compilación despliegan los artefactos generados por sus compilaciones.",
    description: "Description of the Linked Services tab",
  },
  "buildSources.linkExistingService": {
    message: "Vincular servicio existente",
    description: "Button opening the Link Existing Service dialog",
  },
  "buildSources.servicesEmptyTitle": {
    message: "Sin servicios vinculados",
    description: "Empty state heading on Linked Services tab",
  },
  "buildSources.servicesEmptyDescription": {
    message: "Vincula Web Services, Private Services, Background Workers o Cron Jobs existentes para compartir compilaciones de esta fuente.",
    description: "Empty state description on Linked Services tab",
  },
  "buildSources.autoDeployOn": {
    message: "Despliegue automático activado",
    description: "Badge indicating a linked service auto-deploys new builds",
  },
  "buildSources.autoDeployOff": {
    message: "Despliegue automático desactivado",
    description: "Badge indicating a linked service requires manual deploys",
  },
  "buildSources.changeOrUnlinkSource": {
    message: "Cambiar fuente",
    description: "Link to a service's settings page to switch or unlink its source",
  },
  "buildSources.linkDialogTitle": {
    message: "Vincular servicio existente",
    description: "Heading of the Link Existing Service dialog",
  },
  "buildSources.linkDialogDescription": {
    message: 'Asocia un servicio existente del espacio de trabajo a "{name}". Vincular guarda la fuente sin iniciar un despliegue.',
    description: "Description of the Link Existing Service dialog",
  },
  "buildSources.linkDialogNoEligible": {
    message: "Todos los servicios elegibles de este espacio de trabajo ya están vinculados o todavía no existe ninguno.",
    description: "Message when no linkable unlinked services exist",
  },
  "buildSources.linkDialogServiceLabel": {
    message: "Servicio",
    description: "Select label in Link Existing Service dialog",
  },
  "buildSources.linkDialogServicePlaceholder": {
    message: "Elige un servicio…",
    description: "Select placeholder in Link Existing Service dialog",
  },
  "buildSources.linkDialogRegionMismatch": {
    message: "La región no coincide: esta fuente Git está en {sourceRegion}, mientras que el servicio seleccionado está en {serviceRegion}.",
    description: "Inline error when linking a Git build source to a service in another region",
  },
  "buildSources.linkDialogStartCommandHint": {
    message: "Obligatorio para fuentes con entorno de ejecución nativo para indicar cómo inicia el servicio.",
    description: "Help text for startCommand input in Link Existing Service dialog",
  },
  "buildSources.linkDialogSubmit": {
    message: "Vincular servicio",
    description: "Submit button label in Link Existing Service dialog",
  },
  "buildSources.envImageUnsupportedTitle": {
    message: "El entorno de compilación no está disponible para fuentes de imagen Docker",
    description: "Fallback heading if an image build source visits /environment",
  },
  "buildSources.envImageUnsupportedBody": {
    message: "Solo las fuentes de compilación basadas en Git ejecutan compilaciones con variables de entorno y archivos secretos.",
    description: "Fallback body if an image build source visits /environment",
  },
  "buildSources.envSaveOnlyNote": {
    message: "Los cambios se guardan sin iniciar una compilación y se aplican en la siguiente.",
    description: "Top note on Build-time Environment tab",
  },
  "buildSources.envVarsDescription": {
    message: "Disponibles durante las compilaciones de esta fuente. Los cambios surten efecto en la próxima compilación.",
    description: "Environment Variables card description on Build-time Environment tab",
  },
  "buildSources.secretFilesDescription": {
    message: "Se montan durante las compilaciones de esta fuente. Los cambios surten efecto en la próxima compilación.",
    description: "Secret Files card description on Build-time Environment tab",
  },
  "buildSources.envGroupsPanelDescription": {
    message: "Grupos de entorno a nivel de espacio de trabajo vinculados a la compilación. Los cambios se aplican en la siguiente compilación sin desplegar.",
    description: "Environment Groups card description on Build-time Environment tab",
  },
  "buildSources.envGroupsNoneLinked": {
    message: "No hay grupos de entorno vinculados a esta fuente de compilación.",
    description: "Empty state when no env groups are linked to a build source",
  },
  "buildSources.settingsIdentityTitle": {
    message: "Identidad y alcance",
    description: "Heading of read-only identity card on Settings tab",
  },
  "buildSources.settingsIdentityDescription": {
    message: "Identificadores inmutables y alcance de esta fuente de compilación.",
    description: "Description of read-only identity card on Settings tab",
  },
  "buildSources.settingsFieldId": {
    message: "ID de fuente de compilación",
    description: "Label for read-only bds- ID field on Settings tab",
  },
  "buildSources.settingsConfigTitle": {
    message: "Configuración de compilación",
    description: "Heading of editable settings card on Settings tab",
  },
  "buildSources.settingsNoBuildNote": {
    message: "Guardar la configuración actualiza esta fuente de compilación sin iniciar una compilación.",
    description: "Inline note explaining Settings edits do not rebuild",
  },
  "buildSources.settingsFieldRepoUrl": {
    message: "URL del repositorio",
    description: "Input label for Git repository URL on Settings tab",
  },
  "buildSources.settingsInvalidRepoUrl": {
    message: "Introduce una URL de repositorio Git válida.",
    description: "Validation error for malformed repo URL on Settings tab",
  },
  "buildSources.settingsInvalidImageUrl": {
    message: "Introduce una referencia de imagen Docker válida.",
    description: "Validation error for malformed image URL on Settings tab",
  },
  "buildSources.settingsSaveButton": {
    message: "Guardar cambios",
    description: "Submit button label on Settings tab",
  },
  "buildSources.settingsSaving": {
    message: "Guardando…",
    description: "Submit button label while updateBuildSource is in flight",
  },
  "buildSources.dangerZoneTitle": {
    message: "Zona de peligro",
    description: "Heading of Delete Build Source card on Settings tab",
  },
  "buildSources.dangerZoneDescription": {
    message: "Elimina permanentemente esta fuente de compilación y su configuración de compilación. Esta acción no se puede deshacer.",
    description: "Description of Delete Build Source card",
  },
  "buildSources.deleteBlockedTitle_one": {
    message: "Desvincula {count} servicio antes de eliminar",
    description: "Singular alert heading blocking delete when services remain linked",
  },
  "buildSources.deleteBlockedTitle_many": {
    message: "Desvincula {count} de servicios antes de eliminar",
    description: "Many alert heading blocking delete when services remain linked",
  },
  "buildSources.deleteBlockedTitle_other": {
    message: "Desvincula {count} servicios antes de eliminar",
    description: "Plural alert heading blocking delete when services remain linked",
  },
  "buildSources.deleteBlockedDescription": {
    message: "No se puede eliminar una fuente de compilación mientras existan servicios vinculados. Cambia primero cada servicio vinculado a otra fuente:",
    description: "Explanation under delete-blocked alert",
  },
  "buildSources.deleteButton": {
    message: "Eliminar fuente de compilación",
    description: "Destructive button opening the delete confirmation dialog",
  },
  "buildSources.deleteConfirmTitle": {
    message: '¿Eliminar "{name}"?',
    description: "Title of Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmBody": {
    message: "Esto elimina permanentemente la fuente de compilación y su entorno de compilación. El historial y la configuración no se pueden recuperar.",
    description: "Body of Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmPrompt": {
    message: "Escribe {phrase} abajo para confirmar.",
    description: "Sudo command prompt inside Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmButton": {
    message: "Eliminar fuente de compilación",
    description: "Confirm action label inside Delete Build Source confirmation dialog",
  },
};

export default esBuildSources;
