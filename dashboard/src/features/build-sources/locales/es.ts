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
};

export default esBuildSources;
