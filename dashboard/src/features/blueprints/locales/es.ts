import type { TranslationEntry } from "@/i18n";

const esBlueprints: Record<string, TranslationEntry> = {
  "blueprints.resourceType": {
    message: "Blueprint",
    description: "Blueprint resource type used in document titles",
  },
  "blueprints.pageTitle": {
    message: "Blueprints",
    description: "Blueprints list page heading and document title",
  },
  "blueprints.cardTitle": {
    message: "Blueprints",
    description: "Blueprints table card title",
  },
  "blueprints.colName": {
    message: "Nombre",
    description:
      "Blueprints table column header — blueprint name (derived from repo)",
  },
  "blueprints.colRepo": {
    message: "Repositorio",
    description: "Blueprints table column header — source repo URL",
  },
  "blueprints.colBranch": {
    message: "Rama",
    description: "Blueprints table column header — tracked branch",
  },
  "blueprints.colStatus": {
    message: "Estado",
    description: "Blueprints table column header",
  },
  "blueprints.colUpdated": {
    message: "Última actualización",
    description: "Blueprints table column header — updatedAt relative age",
  },
  "blueprints.createButton": {
    message: "Nueva instancia de Blueprint",
    description: "Button label that opens the New Blueprint dialog",
  },
  "blueprints.createTitle": {
    message: "Nueva instancia de Blueprint",
    description: "New Blueprint page title",
  },
  "blueprints.createDescription": {
    message:
      "Conecta un repositorio que contenga un Blueprint render.yaml para crear y sincronizar un conjunto completo de servicios.",
    description: "New Blueprint page subtitle under the title",
  },
  "blueprints.createSourceTitle": {
    message: "Conectar un repositorio",
    description:
      "New Blueprint page — heading above the GitHub/Public Git source tabs (Render parity)",
  },
  "blueprints.createBranchLabel": {
    message: "Rama",
    description: "New Blueprint page — branch combobox label",
  },
  "blueprints.createBranchPlaceholder": {
    message: "main",
    description: "New Blueprint page — branch combobox placeholder",
  },
  "blueprints.createBranchHint": {
    message: "La rama del repositorio que contiene el archivo render.yaml.",
    description: "New Blueprint page — branch field helper text",
  },
  "blueprints.createPathLabel": {
    message: "Ruta del Blueprint",
    description: "New Blueprint page — render.yaml path input label",
  },
  "blueprints.createNamePlaceholder": {
    message: "my-stack",
    description: "New Blueprint page — blueprint name placeholder",
  },
  "blueprints.createPathPlaceholder": {
    message: "render.yaml",
    description: "New Blueprint page — manifest path placeholder",
  },
  "blueprints.createPathHint": {
    message:
      "La ruta del archivo Blueprint en tu repositorio; sirve cualquier archivo .yaml/.yml (p. ej. infra/bex/stack.yaml). Por defecto es render.yaml en la raíz de tu repositorio.",
    description: "New Blueprint page — manifest path helper text",
  },
  "blueprints.createNameLabel": {
    message: "Nombre del Blueprint",
    description: "New Blueprint page — name input label",
  },
  "blueprints.createNameTaken": {
    message:
      "Ya existe un Blueprint llamado {name} en este espacio de trabajo. Elige otro nombre.",
    description:
      "New Blueprint page — shown under the name field when Deploy was refused because an approved Blueprint already has the name (w8/250)",
  },
  "blueprints.createAction": {
    message: "Desplegar Blueprint",
    description: "New Blueprint dialog submit button label",
  },
  "blueprints.createCancel": {
    message: "Cancelar",
    description: "New Blueprint dialog cancel button label",
  },
  "blueprints.createSuccess": {
    message: "Blueprint creado",
    description: "Toast shown after a successful createBlueprint call",
  },
  "blueprints.createError": {
    message: "Falló la creación del Blueprint",
    description: "Toast shown when createBlueprint returns an error",
  },
  "blueprints.previewTitle": {
    message: "Revisar la configuración del Blueprint",
    description:
      "New Blueprint page — heading of the pre-create fetch + validate section",
  },
  "blueprints.previewSelectSource": {
    message:
      "Selecciona un repositorio y una rama para revisar su archivo Blueprint antes de desplegar.",
    description: "Review section placeholder before a source is chosen",
  },
  "blueprints.previewLoading": {
    message: "Obteniendo {path} de tu repositorio…",
    description: "Review section loading text while the manifest is fetched",
  },
  "blueprints.previewNotFoundTitle": {
    message: "Archivo Blueprint no encontrado",
    description: "Review section error title when the manifest fetch fails",
  },
  "blueprints.previewNotFoundBody": {
    message: "No se encontró el archivo Blueprint {path} en la rama {branch}.",
    description:
      "Review section fallback error body when the backend returns no message",
  },
  "blueprints.previewFileNotFoundTitle": {
    message: "Archivo Blueprint no encontrado",
    description:
      "Preview failure title: the repo and branch resolved, the file did not",
  },
  "blueprints.previewFileNotFoundBody": {
    message:
      "{path} no existe en la rama {branch}. Comprueba la ruta o haz commit del archivo en esa rama.",
    description: "Preview failure body for file_not_found",
  },
  "blueprints.previewBranchNotFoundTitle": {
    message: "Rama no encontrada",
    description: "Preview failure title for branch_not_found",
  },
  "blueprints.previewBranchNotFoundBody": {
    message: "La rama {branch} no existe en este repositorio.",
    description: "Preview failure body for branch_not_found",
  },
  "blueprints.previewRepoNotFoundTitle": {
    message: "Repositorio no encontrado",
    description: "Preview failure title for repo_not_found_or_no_access",
  },
  "blueprints.previewRepoNotFoundBody": {
    message:
      "Este repositorio no existe, o la GitHub App de bex no puede acceder a él. Comprueba el repositorio y el acceso de la app a los repositorios.",
    description: "Preview failure body for repo_not_found_or_no_access",
  },
  "blueprints.previewAccessDeniedTitle": {
    message: "Acceso denegado",
    description: "Preview failure title for access_denied",
  },
  "blueprints.previewAccessDeniedBody": {
    message:
      "La GitHub App de bex no está autorizada para leer este repositorio. Vuelve a comprobar la instalación y los repositorios a los que tiene acceso.",
    description: "Preview failure body for access_denied",
  },
  "blueprints.previewPublicRepoNotFoundBody": {
    message:
      "Este repositorio no existe en {host}, o no es público. bex lee un Blueprint allí desde un repositorio público.",
    description:
      "Preview failure body for repo_not_found_or_no_access on gitlab.com or bitbucket.org, read anonymously; {host} is that host",
  },
  "blueprints.previewPublicAccessDeniedBody": {
    message:
      "{host} no permitió que bex leyera este repositorio. bex lee un Blueprint allí desde un repositorio público.",
    description:
      "Preview failure body for access_denied on gitlab.com or bitbucket.org, read anonymously; {host} is that host",
  },
  "blueprints.previewHostUnsupportedTitle": {
    message: "Host de Git no compatible",
    description:
      "Preview failure title for a repository on a Git host bex reads no Blueprint from",
  },
  "blueprints.previewHostUnsupportedBody": {
    message:
      "Como Render, bex lee un Blueprint desde un repositorio público en github.com, gitlab.com o bitbucket.org. {host} no es uno de ellos.",
    description:
      "Preview failure body for a repository on a Git host bex reads no Blueprint from; {host} is that host",
  },
  "blueprints.previewRateLimitedTitle": {
    message: "{host} está limitando las solicitudes de bex",
    description:
      "Preview failure title for rate_limited; {host} is GitHub or the repository's host",
  },
  "blueprints.previewRateLimitedBody": {
    message:
      "Hay demasiadas solicitudes a {host} en este momento. Inténtalo de nuevo en unos minutos.",
    description:
      "Preview failure body for rate_limited; {host} is GitHub or the repository's host",
  },
  "blueprints.previewAmbiguousTitle": {
    message: "Dos archivos Blueprint",
    description: "Preview failure title for ambiguous_filename",
  },
  "blueprints.previewAmbiguousBody": {
    message:
      "Este repositorio tiene render.yaml y bex.yml en la rama {branch}. Elimina uno o indica el archivo explícitamente arriba.",
    description: "Preview failure body for ambiguous_filename",
  },
  "blueprints.previewUnavailableTitle": {
    message: "No se pudo conectar con {host}",
    description:
      "Preview failure title for unavailable; {host} is GitHub or the repository's host",
  },
  "blueprints.previewUnavailableBody": {
    message:
      "bex no pudo leer el archivo Blueprint desde {host}. Inténtalo de nuevo en unos minutos.",
    description:
      "Preview failure body for unavailable; {host} is GitHub or the repository's host",
  },
  "blueprints.previewInvalidPathExtension": {
    message: "La ruta del Blueprint debe ser un archivo .yaml o .yml.",
    description: "Inline path-field error for invalid_path, wrong extension",
  },
  "blueprints.previewInvalidPathShape": {
    message:
      "La ruta del Blueprint debe ser una ruta limpia relativa al repositorio (sin barra inicial, sin ..).",
    description: "Inline path-field error for invalid_path, bad shape",
  },
  "blueprints.previewRetry": {
    message: "Reintentar",
    description: "Review section retry button after a failed manifest fetch",
  },
  "blueprints.takeoverResourceTitle": {
    message: "¿Hacerte cargo de “{name}”?",
    description:
      "Takeover dialog title when a resource is managed by another Blueprint (BLUEPRINT_RESOURCE_CONFLICT)",
  },
  "blueprints.takeoverResourceBody": {
    message:
      "{kind} “{name}” está gestionado por el Blueprint {owner}. Si continúas, se transfiere a este Blueprint. Escribe el comando de abajo para continuar.",
    description: "Takeover dialog body for a Blueprint resource conflict",
  },
  "blueprints.takeoverResourceBodyOtherOwner": {
    message:
      "{kind} “{name}” está gestionado por otro Blueprint. Si continúas, se transfiere a este Blueprint. Escribe el comando de abajo para continuar.",
    description:
      "Takeover dialog body for a Blueprint resource conflict whose owning Blueprint bex-api could not name",
  },
  "blueprints.previewConflictBody": {
    message:
      "Otro Blueprint ya gestiona algunos de estos recursos. Al desplegar se transfieren a este Blueprint; en el siguiente paso confirmarás exactamente qué se toma.",
    description:
      "Preview alert body on /blueprints/new when the only problem is a takeover conflict (replaces the raw API message)",
  },
  "blueprints.previewConflict": {
    message: "Ya está gestionado: al desplegar se tomará el control",
    description:
      "Alert title on /blueprints/new when the only preview problem is a resource another Blueprint manages, which an explicit takeover confirmation resolves (w4/m125)",
  },
  "blueprints.previewMatchTitle": {
    message: "Coincide con recursos existentes",
    description:
      "Notice title on /blueprints/new when the Blueprint file names resources that already exist in the workspace (w8/m153, Render parity)",
  },
  "blueprints.previewMatchBody": {
    message:
      "Este Blueprint coincide con recursos que ya existen en tu espacio de trabajo. Los recursos nuevos se crearán con el sufijo “-{suffix}” añadido a cada nombre. Los recursos existentes no se modifican.",
    description:
      "Notice body on /blueprints/new for matched resources; {suffix} is the four-character resource name suffix, shown after a hyphen as it appears in a name",
  },
  "blueprints.previewMatchListLabel": {
    message: "Ya están en tu espacio de trabajo:",
    description:
      "Notice on /blueprints/new — label above the list of existing resources the Blueprint file matches",
  },
  "blueprints.previewInvalid": {
    message: "El archivo Blueprint tiene errores",
    description:
      "Review section error title when the fetched manifest fails validation",
  },
  "blueprints.previewValid_one": {
    message:
      "El archivo Blueprint se analizó correctamente: cambiará {count} recurso.",
    description:
      "Review section success line, with the plan's total resource count (singular)",
  },
  "blueprints.previewValid_many": {
    message:
      "El archivo Blueprint se analizó correctamente: cambiarán {count} de recursos.",
    description:
      "Review section success line, with the plan's total resource count (plural)",
  },
  "blueprints.previewValid_other": {
    message:
      "El archivo Blueprint se analizó correctamente: cambiarán {count} recursos.",
    description:
      "Review section success line, with the plan's total resource count (plural)",
  },
  "blueprints.previewValidEnvironments_one": {
    message:
      "El archivo Blueprint se analizó correctamente: cambiará {count} entorno.",
    description:
      "Review section success line when only environments change (w9/095, singular)",
  },
  "blueprints.previewValidEnvironments_many": {
    message:
      "El archivo Blueprint se analizó correctamente: cambiarán {count} de entornos.",
    description:
      "Review section success line when only environments change (w9/095, plural)",
  },
  "blueprints.previewValidEnvironments_other": {
    message:
      "El archivo Blueprint se analizó correctamente: cambiarán {count} entornos.",
    description:
      "Review section success line when only environments change (w9/095, plural)",
  },
  "blueprints.previewValidWithEnvironments_one": {
    message:
      "El archivo Blueprint se analizó correctamente: cambiarán {count} recurso y {environments}.",
    description:
      "Review section success line: changing resources (singular) and blueprints.previewEnvironmentCount (w9/095)",
  },
  "blueprints.previewValidWithEnvironments_many": {
    message:
      "El archivo Blueprint se analizó correctamente: cambiarán {count} de recursos y {environments}.",
    description:
      "Review section success line: changing resources (plural) and blueprints.previewEnvironmentCount (w9/095)",
  },
  "blueprints.previewValidWithEnvironments_other": {
    message:
      "El archivo Blueprint se analizó correctamente: cambiarán {count} recursos y {environments}.",
    description:
      "Review section success line: changing resources (plural) and blueprints.previewEnvironmentCount (w9/095)",
  },
  "blueprints.previewEnvironmentCount_one": {
    message: "{count} entorno",
    description:
      "The changing environments inside blueprints.previewValidWithEnvironments (singular)",
  },
  "blueprints.previewEnvironmentCount_many": {
    message: "{count} de entornos",
    description:
      "The changing environments inside blueprints.previewValidWithEnvironments (plural)",
  },
  "blueprints.previewEnvironmentCount_other": {
    message: "{count} entornos",
    description:
      "The changing environments inside blueprints.previewValidWithEnvironments (plural)",
  },
  "blueprints.errorAtLineColumn": {
    message: "línea {line}, columna {column}",
    description: "Source location beside a Blueprint validation error (w4/151)",
  },
  "blueprints.errorAtLine": {
    message: "línea {line}",
    description:
      "Source location beside a Blueprint validation error when only the line is known (w4/151)",
  },
  "blueprints.previewNoChanges": {
    message:
      "El archivo Blueprint se analizó correctamente: sin cambios. Todos los recursos ya coinciden con este archivo.",
    description:
      "Review section success line when every planned action is a no-op (w4/m138)",
  },
  "blueprints.previewOpCreate": {
    message: "Crear",
    description:
      "Plan row badge: the resource does not exist yet and will be created",
  },
  "blueprints.previewOpUpdate": {
    message: "Actualizar",
    description: "Plan row badge: an existing resource will be changed",
  },
  "blueprints.previewOpNoop": {
    message: "Sin cambios",
    description: "Plan row badge: the resource already matches the file",
  },
  "blueprints.previewOpError": {
    message: "No se puede aplicar",
    description:
      "Plan row badge: the planner refused this resource (reason below)",
  },
  "blueprints.previewChangedFields": {
    message: "Cambios: {fields}",
    description:
      "Plan row detail for an update: the changed field paths (never values)",
  },
  "blueprints.previewKindService": {
    message: "Servicio",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindPostgres": {
    message: "Postgres",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindKeyValue": {
    message: "Key Value",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindEnvGroup": {
    message: "Grupo de entorno",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindBuildSource": {
    message: "Origen de build",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindEnvironment": {
    message: "Entorno",
    description: "Plan row resource kind: a projects[].environments[] entry",
  },
  "blueprints.previewServices": {
    message: "Servicios",
    description: "Review section plan group label — services in the manifest",
  },
  "blueprints.previewDatabases": {
    message: "Bases de datos",
    description: "Review section plan group label — databases in the manifest",
  },
  "blueprints.previewKeyValue": {
    message: "Key Value",
    description:
      "Review section plan group label — key-value instances in the manifest",
  },
  "blueprints.previewEnvGroups": {
    message: "Grupos de entorno",
    description: "Review section plan group label — env groups in the manifest",
  },
  "blueprints.previewManualSyncNote": {
    message:
      "bex no recibe los pushes de este repositorio, así que los cambios posteriores en tu archivo Blueprint no se sincronizarán automáticamente. Sincroniza el Blueprint después de cada cambio o conecta el repositorio mediante la GitHub App.",
    description:
      "Review section note for a repo whose pushes can't reach bex (the preview's autoSync is false): the Blueprint is created without auto-sync",
  },
  "blueprints.previewAutoSyncNote": {
    message:
      "Todas las actualizaciones futuras de tu archivo Blueprint en esta rama se sincronizarán automáticamente. Puedes pausar la sincronización automática después de la creación.",
    description:
      "Review section note about auto-sync being enabled by default (Render parity)",
  },
  "blueprints.previewError": {
    message: "No se pudo cargar la vista previa del Blueprint.",
    description:
      "Review section error shown when the preview query itself fails (network)",
  },
  "blueprints.promptTitle": {
    message: "Valores secretos",
    description:
      "Review section — heading for the sync:false env var prompt inputs",
  },
  "blueprints.promptHint": {
    message:
      "Este Blueprint declara variables de entorno con sync: false: sus valores solo viven en bex, nunca en el repositorio. Establécelos ahora; se asignan una sola vez y las sincronizaciones posteriores nunca los sobrescriben.",
    description: "sync:false prompt section helper text",
  },
  "blueprints.promptEmptyWarning": {
    message:
      "Los valores en blanco se despliegan como no establecidos: es posible que el servicio falle hasta que los establezcas en su pestaña Entorno.",
    description:
      "Warning below the sync:false inputs when at least one is blank",
  },
  "blueprints.pricingTitle": {
    message: "Precio estimado",
    description: "Review section — estimated-pricing panel heading",
  },
  "blueprints.pricingSubtitle": {
    message:
      "Los costos de cómputo se prorratean por segundo. La facturación se realiza al comienzo del mes.",
    description:
      "Estimated-pricing panel subtitle explaining billing semantics (Render parity)",
  },
  "blueprints.pricingLineAmount": {
    message: "({tier}) ${amount} / mes",
    description:
      "Estimated-pricing row amount — plan label + monthly dollar cost",
  },
  "blueprints.pricingLineBreakdown": {
    message: "Instancia ${instance} + disco ({gb} GB) ${storage}",
    description:
      "Estimated-pricing datastore row sub-line — instance vs provisioned-storage cost breakdown",
  },
  "blueprints.pricingVariable": {
    message: "Variable",
    description:
      "Estimated-pricing row amount for resources whose cost depends on runtime behavior",
  },
  "blueprints.pricingReasonAutoscaling": {
    message: "Escalado automático",
    description:
      "Estimated-pricing variable-row badge — service has autoscaling enabled",
  },
  "blueprints.pricingReasonMultiInstance": {
    message: "Varias instancias",
    description:
      "Estimated-pricing variable-row badge — service declares numInstances > 1",
  },
  "blueprints.pricingReasonCron": {
    message: "Cron Job",
    description:
      "Estimated-pricing variable-row badge — cron jobs bill only while runs execute",
  },
  "blueprints.pricingTotalLabel": {
    message: "Total",
    description: "Estimated-pricing panel total row label",
  },
  "blueprints.pricingTotalAmount": {
    message: "${amount}{marker} al mes",
    description:
      "Estimated-pricing panel total amount; marker is an asterisk when variable costs are excluded",
  },
  "blueprints.pricingExclusions": {
    message: "* Sin incluir {items}.",
    description:
      "Estimated-pricing footnote listing the variable costs excluded from the total",
  },
  "blueprints.pricingExcludeAutoscaling": {
    message: "escalado automático",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingExcludeMultiInstance": {
    message: "instancias fijas adicionales",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingExcludeCron": {
    message: "Cron Jobs",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingEstimateNote": {
    message:
      "Solo es una estimación: el almacenamiento se calcula según el tamaño aprovisionado; tu factura mide el uso real.",
    description:
      "Estimated-pricing disclaimer: forward estimate is an upper bound of metered billing",
  },
  "blueprints.generateButton": {
    message: "Generar Blueprint",
    description: "Blueprints list header — opens the export dialog (w8/m22)",
  },
  "blueprints.generateTitle": {
    message: "Generar Blueprint",
    description: "Generate dialog title",
  },
  "blueprints.generateDescription": {
    message:
      "Selecciona recursos existentes para exportarlos como un render.yaml que puedas confirmar en un repositorio y conectar como Blueprint.",
    description: "Generate dialog description",
  },
  "blueprints.generateEmptyHint": {
    message: "Selecciona al menos un recurso a partir del cual generar.",
    description: "Generate dialog hint when nothing is selected",
  },
  "blueprints.generateAction": {
    message: "Generar",
    description: "Generate dialog primary action",
  },
  "blueprints.generateBack": {
    message: "Atrás",
    description: "Generate dialog — return from preview to selection",
  },
  "blueprints.generateCopy": {
    message: "Copiar",
    description: "Generate dialog — copy the yaml to the clipboard",
  },
  "blueprints.generateCopied": {
    message: "render.yaml copiado",
    description: "Toast after copying the generated manifest",
  },
  "blueprints.generateCopyFailed": {
    message:
      "No se pudo copiar render.yaml. Permite el acceso al portapapeles e inténtalo de nuevo, o descárgalo.",
    description:
      "Toast when the browser refuses to copy the generated manifest to the clipboard",
  },
  "blueprints.generateDownload": {
    message: "Descargar render.yaml",
    description: "Generate dialog — download the manifest file",
  },
  "blueprints.generateSecretsNote": {
    message:
      "Los valores almacenados nunca se exportan. A las variables de servicio marcadas con sync: false se les pide el valor cuando se crea el Blueprint por primera vez. Las variables marcadas con generateValue, incluidas las claves de grupos de entorno exportadas, conservan el valor existente o reciben uno generado si falta.",
    description: "Generate dialog note under the yaml preview",
  },
  "blueprints.generateError": {
    message: "No se pudo generar el Blueprint",
    description: "Toast when the generate query fails",
  },
  "blueprints.emptyTitle": {
    message: "Aún no has creado ninguna instancia de Blueprint.",
    description: "Blueprints list empty-state heading",
  },
  "blueprints.emptyBody": {
    message:
      "Los Blueprints se registran automáticamente cada vez que despliegas un render.yaml respaldado por un repositorio. Una vez desplegado, tu conjunto de servicios aparece aquí y puedes sincronizarlo o validarlo.",
    description:
      "Blueprints list empty-state body explaining auto-registration",
  },
  "blueprints.loadingBody": {
    message: "Cargando Blueprints…",
    description: "Blueprints list loading state label",
  },
  "blueprints.errorTitle": {
    message: "No se pudieron cargar los Blueprints",
    description: "Blueprints list error state heading",
  },
  "blueprints.statusActive": {
    message: "Activo",
    description: "Blueprint status badge — row is active",
  },
  "blueprints.statusInSync": {
    message: "Sincronizado",
    description: "Blueprint status badge — git-connected and in sync",
  },
  "blueprints.statusSyncing": {
    message: "Sincronizando",
    description: "Blueprint status badge — sync in progress",
  },
  "blueprints.statusError": {
    message: "Error",
    description: "Blueprint status badge — last sync failed",
  },
  "blueprints.statusPaused": {
    message: "En pausa",
    description: "Blueprint status badge — auto-sync paused",
  },
  "blueprints.statusUnknown": {
    message: "{status}",
    description: "Blueprint status badge fallback — raw status value",
  },
  "blueprints.resourceSuffixLabel": {
    message: "Sufijo del nombre de los recursos",
    description:
      "Tooltip and screen-reader label of the suffix shown beside a Blueprint's name in the list and on its detail page (w8/m153)",
  },
  "blueprints.detailTitle": {
    message: "{name} · Blueprints",
    description: "Blueprint detail page document title",
  },
  "blueprints.metaRepo": {
    message: "Repositorio",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaBranch": {
    message: "Rama",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaPath": {
    message: "Ruta del manifiesto",
    description:
      "Blueprint detail metadata label — path to render.yaml in repo",
  },
  "blueprints.metaAutoSync": {
    message: "Sincronización automática",
    description: "Blueprint detail metadata label — auto-sync on push toggle",
  },
  "blueprints.metaCreated": {
    message: "Creado",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaUpdated": {
    message: "Última sincronización",
    description: "Blueprint detail metadata label",
  },
  "blueprints.autoSyncOn": {
    message: "Activada",
    description: "Auto-sync toggle label — enabled",
  },
  "blueprints.autoSyncOff": {
    message: "Desactivada",
    description: "Auto-sync toggle label — disabled",
  },
  "blueprints.resourcesTitle": {
    message: "Recursos gestionados",
    description:
      "Blueprint detail section heading listing services/databases managed by this blueprint",
  },
  "blueprints.resourcesEmpty": {
    message:
      "Aún no hay recursos: sincroniza el Blueprint para aplicar tu render.yaml.",
    description: "Blueprint managed-resources empty state (never synced)",
  },
  "blueprints.resourcesEmptyAwaitingApproval": {
    message:
      "Aún no hay recursos: aprueba la sincronización planificada para crear los recursos de tu render.yaml.",
    description:
      "Blueprint managed-resources empty state while the Blueprint awaits approval of its planned sync",
  },
  "blueprints.resourcesEmptyAfterError": {
    message:
      "No hay recursos reclamados. La última sincronización falló: consulta el historial de sincronizaciones para ver el error y vuelve a intentarlo tras corregir el Blueprint.",
    description:
      "Blueprint managed-resources empty state when status is error after a sync",
  },
  "blueprints.syncHistoryTitle": {
    message: "Historial de sincronizaciones",
    description: "Blueprint detail section heading for the sync run table",
  },
  "blueprints.syncHistoryEmpty": {
    message: "Aún no hay sincronizaciones.",
    description: "Blueprint sync history empty state",
  },
  "blueprints.syncColCommit": {
    message: "Commit",
    description: "Sync history table column — commit SHA",
  },
  "blueprints.syncColState": {
    message: "Estado",
    description: "Sync history table column — sync run state",
  },
  "blueprints.syncColStarted": {
    message: "Inicio",
    description: "Sync history table column — sync run start time",
  },
  "blueprints.syncColCompleted": {
    message: "Finalización",
    description: "Sync history table column — sync run completion time",
  },
  "blueprints.detachPreviewTitle": {
    message: "Los recursos dejarán de estar gestionados",
    description: "Blueprint resource detachment notice.",
  },
  "blueprints.detachSuccessTitle": {
    message: "Sincronización completada: recursos desvinculados",
    description: "Blueprint resource detachment notice.",
  },
  "blueprints.detachEstimateNote": {
    message: "Sus costos no están incluidos en la estimación de abajo.",
    description:
      "Detached resources are outside the proposed manifest estimate.",
  },
  "blueprints.detachWarning": {
    message:
      "Estos recursos siguen en ejecución y pueden seguir generando cargos. Este Blueprint dejará de gestionarlos.",
    description: "Blueprint resource detachment notice.",
  },
  "blueprints.syncColError": {
    message: "Detalles",
    description: "Sync history table column — errors and durable sync notes",
  },
  "blueprints.manifestTitle": {
    message: "Manifiesto render.yaml",
    description: "Blueprint detail section heading for the stored manifest",
  },
  "blueprints.plannedTitle": {
    message: "Sincronización planificada",
    description:
      "Blueprint detail: heading of the planned-sync card a created Blueprint shows",
  },
  "blueprints.plannedBody": {
    message:
      "Este Blueprint se creó con un plan. Revisa qué cambiará al aprobar en el commit {commit} y luego aprueba para aplicarlo.",
    description:
      "Planned-sync card: explains plan-then-approve; {commit} is a short commit SHA",
  },
  "blueprints.plannedActions": {
    message: "Cambios planificados",
    description: "Planned-sync card: accessible label of the action list",
  },
  "blueprints.plannedAction": {
    message: "{operation} {kind} {name}",
    description:
      "Planned-sync card: one plan action, e.g. 'create service api'",
  },
  "blueprints.plannedActionsUnavailable": {
    message:
      "Este plan se registró antes de que los planes conservaran sus cambios. Vuelve a planificar para verlos, o aprueba para aplicarlo.",
    description:
      "Planned-sync card: a plan stored without its actions (recorded before migration 0187)",
  },
  "blueprints.plannedManualEnvVars": {
    message:
      "Estos valores no están en el archivo Blueprint (sync: false). Introdúcelos para aprobar.",
    description:
      "Planned-sync card: intro to the required manual env var fields",
  },
  "blueprints.plannedManualEnvVarLabel": {
    message: "{service}: {key}",
    description: "Planned-sync card: label of one manual env var field",
  },
  "blueprints.plannedApprove": {
    message: "Aprobar",
    description: "Planned-sync card: button that applies the planned sync",
  },
  "blueprints.plannedReplan": {
    message: "Volver a planificar",
    description:
      "Planned-sync card: button that plans a new sync at the branch head",
  },
  "blueprints.plannedNone": {
    message:
      "No hay ningún plan esperando aprobación. Vuelve a planificar para leer el último commit de la rama.",
    description:
      "Planned-sync card: shown when a created Blueprint has no approvable plan",
  },
  "blueprints.plannedApproved": {
    message: "Sincronización planificada aprobada",
    description: "Toast after approving a planned sync",
  },
  "blueprints.plannedApproveFailed": {
    message: "No se pudo aprobar la sincronización planificada",
    description: "Toast when approving fails",
  },
  "blueprints.plannedChanged": {
    message:
      "Alguien volvió a planificar este Blueprint. Revisa el plan más reciente y luego apruébalo.",
    description:
      "Toast when the plan on screen was replaced before it was approved (BLUEPRINT_PLAN_CHANGED)",
  },
  "blueprints.plannedReplanFailed": {
    message: "No se pudo planificar una nueva sincronización",
    description: "Toast when re-planning fails",
  },
  "blueprints.syncButton": {
    message: "Sincronizar",
    description: "Button label that triggers an idempotent blueprint re-apply",
  },
  "blueprints.syncConfirmTitle": {
    message: "¿Sincronizar el Blueprint?",
    description: "Sync confirm dialog title",
  },
  "blueprints.syncConfirmBody": {
    message:
      "Esto aplica render.yaml exactamente tal como está en el commit que se muestra abajo, no un push posterior a la rama. Los recursos que ya coinciden se dejan como están.",
    description: "Sync confirm dialog description",
  },
  "blueprints.syncConfirmAction": {
    message: "Sincronizar",
    description: "Sync confirm dialog primary action button label",
  },
  "blueprints.syncCancel": {
    message: "Cancelar",
    description: "Sync confirm dialog cancel button label",
  },
  "blueprints.syncPreviewLoading": {
    message: "Calculando el plan de sincronización a partir del repositorio…",
    description: "Pre-sync dialog — loading line while blueprintPreview runs",
  },
  "blueprints.syncPreviewInvalid": {
    message:
      "El archivo Blueprint que hay ahora en la rama tiene problemas: sincronizar ahora puede fallar o recurrir al manifiesto almacenado:",
    description:
      "Pre-sync dialog — heading above validation errors from the fetched manifest",
  },
  "blueprints.syncPreviewUnavailable": {
    message:
      "No se pudo calcular el plan de sincronización (vista previa no disponible). Actualiza cuando la vista previa se complete: la sincronización requiere un commit revisado.",
    description:
      "Pre-sync dialog — preview transport failure; sync is disabled until a valid preview pins a commit (w8/m41)",
  },
  "blueprints.syncPreviewCommit": {
    message: "Commit revisado {sha}",
    description: "Pre-sync dialog — short SHA of the previewed Git revision",
  },
  "blueprints.syncRefreshPreview": {
    message: "Actualizar vista previa",
    description:
      "Pre-sync dialog — refetch Git + re-validate after BLUEPRINT_SOURCE_CHANGED or an invalid preview",
  },
  "blueprints.syncSourceChanged": {
    message:
      "El origen del Blueprint cambió desde tu revisión: actualiza la vista previa y confirma de nuevo",
    description:
      "Toast when syncBlueprint refuses with BLUEPRINT_SOURCE_CHANGED (path/repo drift or stale pin)",
  },
  "blueprints.syncSuccess": {
    message: "Blueprint sincronizado",
    description: "Toast shown after a successful syncBlueprint call",
  },
  "blueprints.syncError": {
    message: "La sincronización falló",
    description: "Toast shown when syncBlueprint returns an error",
  },
  "blueprints.syncBusy": {
    message:
      "Esta sincronización ya no controla la ejecución (ocupada, interrumpida o reemplazada): inicia una nueva sincronización; el trabajo parcial nunca se reproduce",
    description:
      "Toast shown when syncBlueprint refuses with BLUEPRINT_SYNC_BUSY (admission race, mid-apply fence, or disconnect-busy)",
  },
  "blueprints.editField": {
    message: "Editar {field}",
    description: "Aria label for the inline-edit pencil button",
  },
  "blueprints.saveField": {
    message: "Guardar {field}",
    description: "Aria label for the inline-edit save button",
  },
  "blueprints.cancelEdit": {
    message: "Cancelar edición",
    description: "Aria label for the inline-edit cancel button",
  },
  "blueprints.pathInvalid": {
    message:
      "Debe ser una ruta .yaml o .yml limpia relativa al repositorio (sin barra inicial, sin '..').",
    description: "Inline path editor client-side validation message",
  },
  "blueprints.updateSuccess": {
    message: "Blueprint actualizado",
    description: "Toast shown after a successful updateBlueprint call",
  },
  "blueprints.updateError": {
    message: "La actualización falló",
    description: "Toast shown when updateBlueprint returns an error",
  },
  "blueprints.disconnectButton": {
    message: "Desconectar",
    description:
      "Button that disconnects a blueprint from its Git repo (stops auto-sync, keeps resources)",
  },
  "blueprints.disconnectTitle": {
    message: "¿Desconectar el Blueprint?",
    description: "Disconnect confirm dialog title",
  },
  "blueprints.disconnectBody": {
    message:
      "Esto detiene la sincronización automática en cada push y quita el Blueprint de tu lista. Los recursos ya desplegados no se tocan.",
    description: "Disconnect confirm dialog description",
  },
  "blueprints.disconnectAction": {
    message: "Desconectar",
    description: "Disconnect confirm dialog primary action button label",
  },
  "blueprints.disconnectCancel": {
    message: "Cancelar",
    description: "Disconnect confirm dialog cancel button label",
  },
  "blueprints.disconnectSuccess": {
    message: "Blueprint desconectado",
    description: "Toast shown after a successful disconnectBlueprint call",
  },
  "blueprints.disconnectError": {
    message: "La desconexión falló",
    description: "Toast shown when disconnectBlueprint returns an error",
  },
  "blueprints.disconnectBusy": {
    message:
      "Hay una sincronización aplicándose: vuelve a intentar desconectar cuando termine",
    description:
      "Toast shown when disconnectBlueprint refuses with BLUEPRINT_SYNC_BUSY",
  },
  "blueprints.validateTitle": {
    message: "Validar",
    description: "Validate panel section heading",
  },
  "blueprints.validateRun": {
    message: "Ejecutar validación",
    description: "Validate panel button label",
  },
  "blueprints.validateValid": {
    message: "El manifiesto es válido: no se encontraron errores.",
    description: "Validate result: manifest parsed successfully",
  },
  "blueprints.validateInvalid": {
    message: "El manifiesto tiene errores:",
    description:
      "Validate result: manifest has parse errors — followed by the error list",
  },
  "blueprints.validateNoResult": {
    message: "Aún no hay resultados.",
    description: "Validate panel placeholder before first run",
  },
  "blueprints.stateCreated": {
    message: "Pendiente de aprobación",
    description:
      "Sync state (and Blueprint status) created: a planned sync waiting for approval (w2/m256); never a queued run.",
  },
  "blueprints.stateRunning": {
    message: "En ejecución",
    description: "Sync-run state: the run is in progress.",
  },
  "blueprints.stateSuccess": {
    message: "Correcto",
    description: "Sync-run state: the run completed successfully.",
  },
};

export default esBlueprints;
