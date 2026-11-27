import type { TranslationEntry } from "@/i18n";

const esProjects: Record<string, TranslationEntry> = {
  "projects.overviewTitle": {
    message: "Resumen",
    description: "Page title of the Overview page (workspace homepage)",
  },
  "projects.projectsHeading": {
    message: "Proyectos",
    description: "Section heading above the Overview page's project grid",
  },
  "projects.ungroupedHeading": {
    message: "Recursos sin agrupar",
    description:
      "Section heading above the Overview page's ungrouped resource table",
  },
  "projects.allResourcesHeading": {
    message: "Todos los recursos",
    description:
      "Heading above a project page's full resource table, below the environments section",
  },
  "projects.filterTypeLabel": {
    message: "Tipo de recurso",
    description: "Accessible label for project environment type filters",
  },
  "projects.filterAll": {
    message: "Todos",
    description: "All project environment resource types",
  },
  "projects.filterServices": {
    message: "Servicios",
    description: "Service resource filter",
  },
  "projects.filterDatabases": {
    message: "Bases de datos",
    description: "Database resource filter",
  },
  "projects.filterKeyValues": {
    message: "Key Value",
    description: "Key-value resource filter",
  },
  "projects.filterEnvGroups": {
    message: "Grupos de entorno",
    description: "Environment-group resource filter",
  },
  "projects.searchLabel": {
    message: "Buscar recursos en {environment}",
    description: "Accessible project environment resource-search label",
  },
  "projects.searchPlaceholder": {
    message: "Buscar recursos en {environment}",
    description: "Project environment resource-search placeholder",
  },
  "projects.noMatchesTitle": {
    message: "Ningún recurso coincide",
    description: "Project environment filtered no-results title",
  },
  "projects.noMatchesBody": {
    message: "Prueba con otra búsqueda o tipo de recurso.",
    description: "Project environment filtered no-results body",
  },
  "projects.eyebrow": {
    message: "Proyecto",
    description: "Small uppercase label above a project page's name heading",
  },
  "projects.cardEmpty": {
    message: "Aún no hay recursos",
    description: "Project card status line when the project has no resources",
  },
  "projects.cardHealthy": {
    message: "Todos los recursos en ejecución",
    description: "Project card status line when every resource is healthy",
  },
  "projects.cardInProgress_one": {
    message: "{count} recurso en curso",
    description:
      "Project card status line when resources are provisioning or deploying without a failure (singular)",
  },
  "projects.cardInProgress_many": {
    message: "{count} de recursos en curso",
    description:
      "Project card status line when resources are provisioning or deploying without a failure (plural)",
  },
  "projects.cardInProgress_other": {
    message: "{count} recursos en curso",
    description:
      "Project card status line when resources are provisioning or deploying without a failure (plural)",
  },
  "projects.cardUnhealthy_one": {
    message: "{count} recurso requiere atención",
    description: "Project card status line when one resource is unhealthy",
  },
  "projects.cardUnhealthy_many": {
    message: "{count} de recursos requieren atención",
    description:
      "Project card status line when more than one resource is unhealthy",
  },
  "projects.cardUnhealthy_other": {
    message: "{count} recursos requieren atención",
    description:
      "Project card status line when more than one resource is unhealthy",
  },
  "projects.newProjectCard": {
    message: "Crear nuevo proyecto",
    description:
      'The dashed "+ create project" tile at the end of the project grid',
  },
  "projects.errorTitle": {
    message: "No se pudieron cargar los recursos",
    description: "Error state heading on the unified Projects page",
  },
  "projects.errorBody": {
    message:
      "Algo salió mal al cargar tus recursos. Inténtalo de nuevo en un momento.",
    description: "Error state body on the unified Projects page",
  },
  "projects.emptyTitle": {
    message: "Aún no hay recursos",
    description: "Empty state heading on the unified Projects page",
  },
  "projects.emptyBody": {
    message:
      "Crea un servicio, una base de datos o un almacén Key Value para empezar.",
    description: "Empty state body on the unified Projects page",
  },
  "projects.emptyCreateService": {
    message: "Crear un servicio",
    description: "Primary action of the empty Projects page (ADR075 D1)",
  },
  "projects.newButton": {
    message: "Nuevo",
    description: "Button that opens the new-resource menu",
  },
  "projects.newService": {
    message: "Nuevo servicio",
    description: "New-resource menu item that starts the service create wizard",
  },
  "projects.newDatabase": {
    message: "Nueva base de datos",
    description: "New-resource menu item that opens the database create dialog",
  },
  "projects.newKeyValue": {
    message: "Nuevo Key Value",
    description: "New-resource menu item that starts the key value create form",
  },
  "projects.colName": {
    message: "Nombre",
    description: "Merged resource table column header",
  },
  "projects.colType": {
    message: "Tipo",
    description: "Merged resource table column header",
  },
  "projects.colStatus": {
    message: "Estado",
    description: "Merged resource table column header",
  },
  "projects.colCreated": {
    message: "Creado",
    description: "Merged resource table column header",
  },
  "projects.colRuntime": {
    message: "Runtime",
    description:
      "Authoritative runtime/product column in a Project resource table",
  },
  "projects.colRegion": {
    message: "Región",
    description:
      "Explicit installation placement column in a Project resource table",
  },
  "projects.colUpdated": {
    message: "Actualizado",
    description:
      "Authoritative resource update-time column in a Project resource table",
  },
  "projects.colActions": {
    message: "Acciones",
    description: "Merged resource table column header (screen-reader only)",
  },
  "projects.typeService": {
    message: "Servicio",
    description: "Type badge for a service row in the merged resource table",
  },
  "projects.typeDatabase": {
    message: "Base de datos",
    description: "Type badge for a database row in the merged resource table",
  },
  "projects.typeKeyValue": {
    message: "Key Value",
    description: "Type badge for a key value row in the merged resource table",
  },
  "projects.typeEnvGroup": {
    message: "Grupo de entorno",
    description: "Type badge for an environment-group row",
  },
  "projects.metadataNotApplicable": {
    message: "No aplicable",
    description:
      "Accessible label for a status, runtime or region cell on an environment group row, which has none of them (w4/549)",
  },
  "projects.metadataUnavailable": {
    message: "No disponible",
    description:
      "Accessible label for an unavailable runtime, region, status, or timestamp",
  },
  "projects.selectAllVisible": {
    message: "Seleccionar todos los recursos visibles",
    description: "Accessible label for the Project table header checkbox",
  },
  "projects.selectResource": {
    message: "Seleccionar {name}",
    description: "Accessible label for one Project resource row checkbox",
  },
  "projects.selectedCount": {
    message: "Seleccionados: {count}",
    description: "Bulk action toolbar selection count",
  },
  "projects.moveTargetLabel": {
    message: "Mover los recursos seleccionados al entorno",
    description: "Accessible label for the bulk Move target selector",
  },
  "projects.moveTargetPlaceholder": {
    message: "Elegir entorno",
    description: "Placeholder for the bulk Move target selector",
  },
  "projects.moveSelected": {
    message: "Mover",
    description:
      "Bulk action that moves selected resources to another Environment",
  },
  "projects.bulkResume": {
    message: "Reanudar",
    description:
      "Selection toolbar action that resumes every selected suspended resource (Render's bulk Resume, w4/m550)",
  },
  "projects.bulkSuspend": {
    message: "Suspender",
    description:
      "Selection toolbar action that suspends every selected running resource (Render's bulk Suspend, w4/m550)",
  },
  "projects.bulkSuspendTitle": {
    message: "Suspender servicios",
    description: "Title of the bulk Suspend dialog",
  },
  "projects.bulkSuspendBody": {
    message:
      "Los siguientes servicios se suspenderán y dejarán de funcionar de inmediato. Puedes deshacerlo reanudando los servicios.",
    description:
      "Body of the bulk Suspend dialog, above the list of resources it suspends",
  },
  "projects.bulkSuspendConfirm": {
    message: "Suspender servicios",
    description: "Confirm button of the bulk Suspend dialog",
  },
  "projects.bulkResumeTitle": {
    message: "Reanudar servicios",
    description: "Title of the bulk Resume dialog",
  },
  "projects.bulkResumeBody": {
    message: "Se reanudarán los siguientes servicios:",
    description:
      "Body of the bulk Resume dialog, above the list of resources it resumes",
  },
  "projects.bulkResumeConfirm": {
    message: "Reanudar servicios",
    description: "Confirm button of the bulk Resume dialog",
  },
  "projects.bulkRetry": {
    message: "Reintentar",
    description:
      "Button on a bulk Suspend/Resume row whose call failed; runs that row again",
  },
  "projects.bulkRetryRow": {
    message: "Reintentar {name}",
    description:
      "Accessible label of a row's Retry button ({name} = the resource)",
  },
  "projects.bulkRowSuspending": {
    message: "Suspendiendo",
    description:
      "Screen-reader state of a bulk Suspend row whose call is in flight",
  },
  "projects.bulkRowSuspended": {
    message: "Suspendido",
    description: "Screen-reader state of a bulk Suspend row that succeeded",
  },
  "projects.bulkRowResuming": {
    message: "Reanudando",
    description:
      "Screen-reader state of a bulk Resume row whose call is in flight",
  },
  "projects.bulkRowResumed": {
    message: "Reanudado",
    description: "Screen-reader state of a bulk Resume row that succeeded",
  },
  "projects.bulkRowError": {
    message: "La solicitud falló. Inténtalo de nuevo.",
    description:
      "Fallback reason on a bulk Suspend/Resume row whose call failed without a server reason",
  },
  "projects.newProjectButton": {
    message: "Nuevo proyecto",
    description: "Button that opens the new-project dialog",
  },
  "projects.createTitle": {
    message: "Nuevo proyecto",
    description: "New-project dialog title",
  },
  "projects.createDescription": {
    message:
      "Agrupa servicios, bases de datos y almacenes Key Value. Todo proyecto empieza con un entorno.",
    description: "New-project dialog description",
  },
  "projects.fieldName": {
    message: "Nombre",
    description: "New-project dialog name field label",
  },
  "projects.fieldFirstEnvironment": {
    message: "Nombre del primer entorno",
    description:
      "New-project dialog: name of the environment created with the project (w4/m264, Render parity)",
  },
  "projects.fieldNamePlaceholder": {
    message: "my-project",
    description: "New-project dialog name field placeholder",
  },
  "projects.createCancel": {
    message: "Cancelar",
    description: "New-project dialog cancel button",
  },
  "projects.createSubmit": {
    message: "Crear proyecto",
    description: "New-project dialog submit button",
  },
  "projects.createSuccess": {
    message: "Proyecto “{name}” creado.",
    description: "Toast shown after a project is created",
  },
  "projects.createError": {
    message: "No se pudo crear el proyecto “{name}”.",
    description: "Toast shown when creating a project fails",
  },
  "projects.editNameButton": {
    message: "Editar",
    description:
      "Accessible label for the project page's inline pencil-edit button, and the Settings page's Project Name \"Edit\" button",
  },
  "projects.renameFieldLabel": {
    message: "Nombre del proyecto",
    description:
      'Accessible/visible label for the project rename field, on both the overview dialog and the Settings card. Deliberately more specific than projects.fieldName\'s bare "Name": these controls are identified by value alone otherwise (w4/134)',
  },
  "projects.renameTitle": {
    message: "Renombrar proyecto",
    description: "Rename-project dialog title",
  },
  "projects.renameSubmit": {
    message: "Guardar",
    description: "Rename-project dialog submit button",
  },
  "projects.renameSuccess": {
    message: "Proyecto renombrado a “{name}”.",
    description: "Toast shown after a project is renamed",
  },
  "projects.renameRefreshFailed": {
    message:
      "Proyecto renombrado a “{name}”, pero esta página no pudo actualizarse. Inténtalo de nuevo o recarga para ver el nombre guardado.",
    description:
      "Warning after the rename saved but re-reading the project failed; the rename must not be retried or reported as failed (w4/m186)",
  },
  "projects.renameError": {
    message: "No se pudo renombrar el proyecto a “{name}”.",
    description: "Toast shown when renaming a project fails",
  },
  "projects.settingsTitle": {
    message: "Configuración del proyecto",
    description: "Project settings page heading",
  },
  "projects.nameCardTitle": {
    message: "Nombre del proyecto",
    description: "Project settings page's name card title",
  },
  "projects.nameCardDescription": {
    message: "Un nombre único para tu proyecto",
    description: "Project settings page's name card description",
  },
  "projects.deleteCardTitle": {
    message: "Eliminar proyecto",
    description: "Project settings page's danger-zone card title",
  },
  "projects.deleteCardDescription": {
    message:
      "Se eliminan sus entornos y todos sus servicios, bases de datos y almacenes Key Value. Sus grupos de entorno quedan sin asignar. Esta acción no se puede deshacer.",
    description: "Project settings page's danger-zone card description",
  },
  "projects.deleteCardButton": {
    message: "Eliminar proyecto",
    description:
      "Project settings page's danger-zone card button, and the delete-confirmation dialog's confirm button",
  },
  "projects.deleteConfirmTitle": {
    message: "¿Eliminar el proyecto “{name}”?",
    description: "Delete-project confirmation dialog title",
  },
  "projects.deleteConfirmBody": {
    message:
      "Se eliminan sus entornos y su configuración. Sus grupos de entorno quedan sin asignar, no se eliminan.",
    description: "Delete-project confirmation dialog body",
  },
  "projects.deleteConfirmBodyResources": {
    message:
      "Eliminar este proyecto elimina todos sus entornos y sus servicios, bases de datos y almacenes Key Value: {resources}. Sus grupos de entorno quedan sin asignar, no se eliminan. Esta acción no se puede deshacer.",
    description:
      "Delete-project confirmation body naming every service, database and key value store it deletes, as the environment delete does (w4/m264, w6/m290, w4/542)",
  },
  "projects.deleteSuccess": {
    message: "Proyecto “{name}” eliminado.",
    description: "Toast shown after a project is deleted",
  },
  "projects.deleteError": {
    message: "No se pudo eliminar el proyecto “{name}”.",
    description: "Toast shown when deleting a project fails",
  },
  "projects.moveToProject": {
    message: "Mover al proyecto",
    description:
      "Row-actions submenu label for assigning a resource to a project",
  },
  "projects.removeFromProject": {
    message: "Quitar del proyecto",
    description:
      "Row-actions submenu item that unassigns a resource from its project",
  },
  "projects.moveSuccess": {
    message: "“{name}” movido a “{project}”.",
    description: "Toast shown after moving a resource to a project",
  },
  "projects.moveError": {
    message: "No se pudo mover “{name}”.",
    description: "Toast shown when moving a resource to a project fails",
  },
  "projects.removeSuccess": {
    message: "“{name}” quitado de su proyecto.",
    description: "Toast shown after unassigning a resource from its project",
  },
  "projects.removeError": {
    message: "No se pudo quitar “{name}” de su proyecto.",
    description: "Toast shown when unassigning a resource fails",
  },
  "projects.editProjectName": {
    message: "Editar el nombre del proyecto",
    description:
      "Screen-reader text of the project page's name link, which opens Project Settings with the Name card editing (Render's words, w1/m682)",
  },
  "projects.addEnvironment": {
    message: "Añadir entorno",
    description:
      "The project page header's button and the card under a project's only environment that open the new-environment dialog (Render's words, w1/m682)",
  },
  "projects.environmentLimitTitle": {
    message: "Consigue más entornos con {plan}",
    description:
      "Title of the paywall Add environment opens at the plan's environments per project; {plan} is the plan name (Render's words, w1/m682)",
  },
  "projects.environmentLimitBody": {
    message:
      "Mejora tu plan para tener entornos ilimitados y todas las funciones del plan {plan}.",
    description:
      "Body of the environments-per-project paywall; {plan} is the plan name (Render's words, w1/m682)",
  },
  "projects.environmentLimitUpgrade": {
    message: "Mejorar plan",
    description:
      "The environments paywall's button to the workspace's change-plan dialog",
  },
  "projects.environmentLimitLearnMore": {
    message: "Más información",
    description:
      "The environments paywall's link to the projects and environments docs",
  },
};

export default esProjects;
