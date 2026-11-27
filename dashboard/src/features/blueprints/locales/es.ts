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
      "Preview alert body on /blueprint/new when the only problem is a takeover conflict (replaces the raw API message)",
  },
  "blueprints.previewConflict": {
    message: "Ya está gestionado: al desplegar se tomará el control",
    description:
      "Alert title on /blueprint/new when the only preview problem is a resource another Blueprint manages, which an explicit takeover confirmation resolves (w4/m125)",
  },
  "blueprints.previewMatchTitle": {
    message: "Coincide con recursos existentes",
    description:
      "Notice title on /blueprint/new when the Blueprint file names resources that already exist in the workspace (w8/m153, Render parity)",
  },
  "blueprints.previewMatchBody": {
    message:
      "Este Blueprint coincide con recursos que ya existen en tu espacio de trabajo. Los recursos nuevos se crearán con el sufijo “-{suffix}” añadido a cada nombre. Los recursos existentes no se modifican.",
    description:
      "Notice body on /blueprint/new for matched resources; {suffix} is the four-character resource name suffix, shown after a hyphen as it appears in a name",
  },
  "blueprints.previewMatchListLabel": {
    message: "Ya están en tu espacio de trabajo:",
    description:
      "Notice on /blueprint/new — label above the list of existing resources the Blueprint file matches",
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
  "blueprints.metaCreated": {
    message: "Creado",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaUpdated": {
    message: "Última sincronización",
    description: "Blueprint detail metadata label",
  },
  "blueprints.detailsTitle": {
    message: "Detalles",
    description:
      "Blueprint Settings page: title of bex's card of read-only facts (repository, created, last synced) under Render's Settings card (w3/m679)",
  },
  "blueprints.settingsNameHint": {
    message: "Un nombre único para tu Blueprint.",
    description:
      "Blueprint Settings card: help under Blueprint Name (Render's copy, w3/m679)",
  },
  "blueprints.settingsRequired": {
    message: "Obligatorio",
    description:
      "Blueprint Settings card: shown under a row left empty (Render's copy, w3/m679)",
  },
  "blueprints.settingsNameTaken": {
    message: "El nombre ya está en uso",
    description:
      "Blueprint Settings card: shown under Blueprint Name when another Blueprint of the workspace has the name (Render's copy, w3/m679)",
  },
  "blueprints.settingsAutoSync": {
    message: "Sincronización automática",
    description:
      "Blueprint Settings card: the Auto Sync row's label (Render's copy, w3/m679)",
  },
  "blueprints.settingsAutoSyncHint": {
    message:
      '¿Sincronizar automáticamente los cambios de tu archivo Blueprint? Selecciona "No" para gestionar las sincronizaciones manualmente.',
    description:
      "Blueprint Settings card: help under Auto Sync; quotes the No option (Render's copy, w3/m679)",
  },
  "blueprints.settingsYes": {
    message: "Sí",
    description:
      "Blueprint Settings card: Auto Sync option, on (Render's copy, w3/m679)",
  },
  "blueprints.settingsNo": {
    message: "No",
    description:
      "Blueprint Settings card: Auto Sync option, off (Render's copy, w3/m679)",
  },
  "blueprints.settingsBranch": {
    message: "Rama",
    description: "Blueprint Settings card: the Branch row's label (w3/m679)",
  },
  "blueprints.settingsBranchHint": {
    message: "La rama de Git que se compila y despliega.",
    description:
      "Blueprint Settings card: help under Branch (Render's copy, w3/m679)",
  },
  "blueprints.settingsBranchNotFound": {
    message: "No se encontró la rama en este repositorio. Elige otra rama.",
    description:
      "Blueprint Settings card: shown under Branch when the repository has no such branch (Render's copy, w3/m679)",
  },
  "blueprints.settingsPath": {
    message: "Ruta del Blueprint",
    description:
      "Blueprint Settings card: the Blueprint Path row's label (Render's copy, w3/m679)",
  },
  "blueprints.settingsPathHint": {
    message:
      "La ruta del archivo Blueprint en tu repositorio (p. ej., infra/render.yaml). Por defecto es render.yaml en la raíz de tu repositorio.",
    description:
      "Blueprint Settings card: help under Blueprint Path (Render's copy, w3/m679)",
  },
  "blueprints.settingsFileNotFound": {
    message:
      'No se encontró el archivo Blueprint "{path}" en la rama "{branch}". Comprueba que el archivo existe en esta rama.',
    description:
      "Blueprint Settings card: shown under Branch or Blueprint Path when the Blueprint file is not at that path on that branch (Render's copy, w3/m679)",
  },
  "blueprints.settingsUpdateFailed": {
    message: "No se pudo actualizar el Blueprint. Inténtalo de nuevo.",
    description:
      "Blueprint Settings card: shown under a row whose save failed with no reason from the server (Render's copy, w3/m679)",
  },
  "blueprints.disconnectNoSync": {
    message:
      "Los recursos dejarán de sincronizarse automáticamente desde tu archivo Blueprint.",
    description:
      "Disconnect Blueprint modal: first sentence (Render's copy, w3/m679)",
  },
  "blueprints.disconnectKeeps": {
    message:
      "Esto no eliminará el archivo. Esto no eliminará los recursos gestionados. Siempre puedes volver a conectar tu archivo más adelante desde la página de Blueprints.",
    description:
      "Disconnect Blueprint modal: second sentence (Render's copy, w3/m679)",
  },
  "blueprints.disconnectConfirm": {
    message:
      "¿Seguro que quieres desconectar tu especificación de Blueprint {name}?",
    description:
      "Disconnect Blueprint modal: the question, with the Blueprint's name in bold (Render's copy, w3/m679)",
  },
  "blueprints.resourcesEmpty": {
    message: "No hay recursos administrados por este Blueprint",
    description:
      "A Blueprint's Resources page when it manages nothing, Render's words (w3/m680)",
  },
  "blueprints.resourcesFileProblems_one": {
    message: "Hubo un problema al sincronizar con tu archivo de Blueprint:",
    description:
      "A Blueprint's Resources page when its newest sync failed on its file: the line above the file's errors, Render's 'There was a problem' / 'There were a few problems' (w3/m680)",
  },
  "blueprints.resourcesFileProblems_many": {
    message:
      "Hubo algunos problemas al sincronizar con tu archivo de Blueprint:",
    description:
      "A Blueprint's Resources page when its newest sync failed on its file: the line above the file's errors, Render's 'There was a problem' / 'There were a few problems' (w3/m680)",
  },
  "blueprints.resourcesFileProblems_other": {
    message:
      "Hubo algunos problemas al sincronizar con tu archivo de Blueprint:",
    description:
      "A Blueprint's Resources page when its newest sync failed on its file: the line above the file's errors, Render's 'There was a problem' / 'There were a few problems' (w3/m680)",
  },
  "blueprints.colBuildSourceName": {
    message: "Nombre de la fuente de compilación",
    description:
      "A Blueprint's Resources page: the Build Sources table's name column, Render's 'Build Source Name' (w3/m680)",
  },
  "blueprints.buildSourceUnlinked": {
    message: "Sin vincular",
    description:
      "A Blueprint's Build Sources table: badge on a build source no service builds from, Render's 'Unlinked' (w3/m680)",
  },
  "blueprints.buildSourcesUngrouped": {
    message: "Sin agrupar",
    description:
      "A Blueprint's Build Sources table: the row over the build sources in no project, Render's 'Ungrouped' (w3/m680)",
  },
  "blueprints.syncHistoryEmpty": {
    message: "Aún no hay sincronizaciones.",
    description: "Syncs page: no syncs yet, Render's wording (w3/m678)",
  },
  "blueprints.detachPreviewTitle": {
    message: "Los recursos dejarán de estar gestionados",
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
  "blueprints.manifestTitle": {
    message: "Manifiesto render.yaml",
    description: "Blueprint detail section heading for the stored manifest",
  },
  "blueprints.plannedActions": {
    message: "Cambios planificados",
    description:
      "A sync's page: accessible label of its list of planned actions (w3/m678)",
  },
  "blueprints.plannedAction": {
    message: "{operation} {kind} {name}",
    description:
      "Planned-sync card: one plan action, e.g. 'create service api'",
  },
  "blueprints.plannedManualEnvVars": {
    message:
      "Estos valores no están en el archivo Blueprint (sync: false). Introdúcelos para aprobar.",
    description:
      "A waiting sync's page: intro to the fields for the sync: false values approving it needs (w3/m678)",
  },
  "blueprints.plannedManualEnvVarLabel": {
    message: "{service}: {key}",
    description:
      "A waiting sync's page: label of one sync: false value's field (w3/m678)",
  },
  "blueprints.plannedApprove": {
    message: "Aprobar",
    description:
      "A waiting sync's page: the button that applies the planned sync, Render's Approve (w3/m678)",
  },
  "blueprints.plannedChanged": {
    message:
      "Alguien volvió a planificar este Blueprint. Revisa el plan más reciente y luego apruébalo.",
    description:
      "A sync's page: inline alert when the plan on screen was replaced before it was approved (BLUEPRINT_PLAN_CHANGED)",
  },
  "blueprints.plannedReplanFailed": {
    message: "No se pudo planificar una nueva sincronización",
    description:
      "Sync Error banner: Manual sync could not plan a sync and the server gave no reason (w3/m678)",
  },
  "blueprints.syncButton": {
    message: "Sincronización manual",
    description:
      "The Blueprint header's one action, Render's Manual sync: plans a sync and opens its page (w3/m678)",
  },
  "blueprints.editField": {
    message: "Editar {field}",
    description: "Aria label for the inline-edit pencil button",
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
  "blueprints.disconnectButton": {
    message: "Desconectar Blueprint",
    description:
      "Blueprint Settings page: the critical button that opens the Disconnect Blueprint modal (Render's copy, w3/m679)",
  },
  "blueprints.disconnectTitle": {
    message: "Desconectar Blueprint",
    description: "Disconnect Blueprint modal title (Render's copy, w3/m679)",
  },
  "blueprints.disconnectAction": {
    message: "Desconectar Blueprint",
    description:
      "Disconnect Blueprint modal: the critical confirm button (Render's copy, w3/m679)",
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
  "blueprints.createNameDescription": {
    message: "Un nombre único para esta instancia de Blueprint.",
    description: "Create-Blueprint Name row description",
  },
  "blueprints.createBlockedSource": {
    message: "Elige un repositorio para continuar.",
    description:
      "Create-Blueprint action bar: why Deploy is held while no repository is chosen",
  },
  "blueprints.createBlockedBranch": {
    message: "Introduce una rama para continuar.",
    description:
      "Create-Blueprint action bar: why Deploy is held while the branch is empty",
  },
  "blueprints.createBlockedReviewing": {
    message: "Revisando el archivo del Blueprint…",
    description:
      "Create-Blueprint action bar: why Deploy is held while the Blueprint file is being reviewed",
  },
  "blueprints.createBlockedReview": {
    message: "Resuelve los problemas que encontró la revisión para continuar.",
    description:
      "Create-Blueprint action bar: why Deploy is held while the review found problems in the Blueprint file",
  },
  "blueprints.navResources": {
    message: "Recursos",
    description:
      "Blueprint sidebar entry and page heading for the resources the Blueprint manages (Render's Resources, /blueprints/<id>/resources, w3/m677)",
  },
  "blueprints.navSyncs": {
    message: "Sincronizaciones",
    description:
      "Blueprint sidebar entry and page heading for the Blueprint's syncs (Render's Syncs, /blueprints/<id>/syncs, w3/m677)",
  },
  "blueprints.navSettings": {
    message: "Configuración",
    description:
      "Blueprint sidebar entry and page heading for the Blueprint's settings (Render's Settings, /blueprints/<id>/settings, w3/m677)",
  },
  "blueprints.resourcesViewSyncs": {
    message: "Ir a Sincronizaciones",
    description:
      "Link from a Blueprint's empty Resources page to its Syncs page, where the waiting plan or the failed sync is (w3/m677)",
  },
  "blueprints.syncViewDetails": {
    message: "Ver detalles",
    description:
      "Sync card: link to the sync's own page (Render's View details, w3/m678)",
  },
  "blueprints.syncsLoadMore": {
    message: "Cargar más",
    description:
      "Syncs page: button reading the next page of older syncs (w3/m678)",
  },
  "blueprints.syncsLoadMoreError": {
    message: "No se pudieron cargar las sincronizaciones anteriores.",
    description:
      "Syncs page: toast when reading an older page of syncs fails (w3/m678)",
  },
  "blueprints.syncPageTitle": {
    message: "Sincronización: {sha}",
    description:
      "A sync's page heading, Render's 'Sync: <short commit id>'; {sha} is the 7-character commit id (w3/m678)",
  },
  "blueprints.syncPageTitleNoCommit": {
    message: "Sincronización",
    description:
      "A sync's page heading when the sync read no commit, Render's 'Sync' (w3/m678)",
  },
  "blueprints.syncDocumentTitle": {
    message: "Sincronización",
    description:
      "Document title's page name on a sync's page: 'Sync ・ <name> ・ Blueprint' (w3/m678)",
  },
  "blueprints.syncUpToDate": {
    message: "Los recursos ya están actualizados",
    description:
      "A sync's page when its plan has no actions, Render's wording (w3/m678)",
  },
  "blueprints.syncNotFullySynced": {
    message: "No pudimos sincronizar completamente estos recursos.",
    description:
      "A failed sync's page with no actions and no errors, Render's critical line (w3/m678)",
  },
  "blueprints.syncNotFullySyncedBody": {
    message: "Si sigues viendo este error, contacta con soporte.",
    description:
      "Under blueprints.syncNotFullySynced: what to do if it keeps happening (w3/m678)",
  },
  "blueprints.syncActionsNotRecorded": {
    message:
      "Esta sincronización se registró antes de que las sincronizaciones guardaran sus acciones, así que no se pueden mostrar.",
    description:
      "A sync's page for a sync recorded before every sync kept its actions (w3/m678)",
  },
  "blueprints.syncActionCreate": {
    message: "Crear {kind} {name}",
    description:
      "A sync's planned action: create a resource; {kind} is its kind label, {name} its name, drawn bold (w3/m678)",
  },
  "blueprints.syncActionUpdate": {
    message: "Actualizar {kind} {name}",
    description:
      "A sync's planned action: change a resource; {kind} is its kind label, {name} its name (w3/m678)",
  },
  "blueprints.syncActionDetach": {
    message: "Dejar de administrar {kind} {name}",
    description:
      "A sync's planned action: stop managing a resource the file no longer names; {kind}, {name} (w3/m678)",
  },
  "blueprints.syncActionError": {
    message: "No se puede aplicar {kind} {name}",
    description:
      "A sync's planned action the planner refused; {kind}, {name}; the reason follows (w3/m678)",
  },
  "blueprints.syncActionDetachNote": {
    message: "Sigue en ejecución y puede generar cargos.",
    description:
      "Under a 'stop managing' action: the resource is not deleted (w3/m678)",
  },
  "blueprints.syncActionStateUnknown": {
    message: "Estado desconocido",
    description:
      "Accessible name of a planned action's state icon when bex can't say which state it reached (a failed sync's actions, w3/m678)",
  },
  "blueprints.syncApproveFailed": {
    message: "No se pudo aprobar esta sincronización. Inténtalo de nuevo.",
    description:
      "A sync's page: inline alert when Approve fails without the server's reason, Render's wording (w3/m678)",
  },
  "blueprints.syncNotFound": {
    message:
      "Esta sincronización no existe. Una sincronización planificada que no se aprueba en 24 horas se elimina.",
    description:
      "A sync's page for an id that is not one of this Blueprint's syncs, e.g. a plan deleted after 24 hours (w3/m678)",
  },
  "blueprints.syncErrorBannerTitle": {
    message: "Error de sincronización",
    description:
      "Title of Render's dismissible banner when Manual sync could not plan a sync (w3/m678)",
  },
  "blueprints.syncErrorBannerDismiss": {
    message: "Descartar",
    description:
      "Accessible name of the Sync Error banner's close button (w3/m678)",
  },
  "blueprints.syncErrorsIntro_one": {
    message: "Hubo un problema al sincronizar con tu Blueprint:",
    description:
      "A sync's page: the line above its errors, Render's 'There was a problem' / 'There were a few problems' (w3/m678)",
  },
  "blueprints.syncErrorsIntro_many": {
    message: "Hubo algunos problemas al sincronizar con tu Blueprint:",
    description:
      "A sync's page: the line above its errors, Render's 'There was a problem' / 'There were a few problems' (w3/m678)",
  },
  "blueprints.syncErrorsIntro_other": {
    message: "Hubo algunos problemas al sincronizar con tu Blueprint:",
    description:
      "A sync's page: the line above its errors, Render's 'There was a problem' / 'There were a few problems' (w3/m678)",
  },
};

export default esBlueprints;
