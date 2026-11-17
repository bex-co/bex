import type { TranslationEntry } from "@/i18n";

const esEnvGroups: Record<string, TranslationEntry> = {
  "envGroups.resourceType": {
    message: "Grupo de entorno",
    description: "Environment Group resource type used in document titles",
  },
  "envGroups.pageTitle": {
    message: "Grupos de entorno",
    description: "Workspace env-groups page title",
  },
  "envGroups.pageDescription": {
    message:
      "Comparte variables de entorno y archivos secretos entre servicios.",
    description: "Workspace env-groups page subtitle",
  },
  "envGroups.newButton": {
    message: "Nuevo grupo de entorno",
    description: "Open the create env-group dialog",
  },
  "envGroups.createTitle": {
    message: "Crear grupo de entorno",
    description: "Create env-group dialog title",
  },
  "envGroups.createDescription": {
    message:
      "Añade ahora variables, archivos secretos y vínculos de servicio, o déjalos vacíos.",
    description: "Create env-group dialog description",
  },
  "envGroups.createVarsTitle": {
    message: "Variables de entorno",
    description: "Initial environment variables section title",
  },
  "envGroups.createVarsDescription": {
    message:
      "Define valores literales o genera secretos robustos durante la creación.",
    description: "Initial environment variables section description",
  },
  "envGroups.varKeyLabel": {
    message: "Clave",
    description: "Initial environment variable key label",
  },
  "envGroups.varKeyPlaceholder": {
    message: "API_TOKEN",
    description: "Initial environment variable key placeholder",
  },
  "envGroups.varValueLabel": {
    message: "Valor",
    description: "Initial environment variable value label",
  },
  "envGroups.varValuePlaceholder": {
    message: "Valor secreto",
    description: "Initial environment variable value placeholder",
  },
  "envGroups.generateValue": {
    message: "Generar",
    description: "Generate an initial environment variable value",
  },
  "envGroups.addInitialVar": {
    message: "Añadir variable de entorno",
    description: "Add an initial environment variable row",
  },
  "envGroups.importEnv": {
    message: "Importar desde .env",
    description:
      "Import initial environment variables from dotenv text or file",
  },
  "envGroups.createFilesTitle": {
    message: "Archivos secretos",
    description: "Initial secret files section title",
  },
  "envGroups.createFilesDescription": {
    message: "Los archivos se cifran y se montan en /etc/secrets.",
    description: "Initial secret files section description",
  },
  "envGroups.fileNameLabel": {
    message: "Nombre del archivo",
    description: "Initial secret file name label",
  },
  "envGroups.fileNamePlaceholder": {
    message: "credentials.json",
    description: "Initial secret file name placeholder",
  },
  "envGroups.fileContentLabel": {
    message: "Contenido",
    description: "Initial secret file contents label",
  },
  "envGroups.fileContentPlaceholder": {
    message: "Contenido del archivo",
    description: "Initial secret file contents placeholder",
  },
  "envGroups.addInitialFile": {
    message: "Añadir archivo secreto",
    description: "Add an initial secret file row",
  },
  "envGroups.createServicesTitle": {
    message: "Servicios vinculados",
    description: "Initial service links section title",
  },
  "envGroups.createServicesDescription": {
    message:
      "Los servicios seleccionados reciben este grupo en cuanto se crea.",
    description: "Initial service links section description",
  },
  "envGroups.servicesLoading": {
    message: "Cargando servicios del espacio de trabajo…",
    description: "Initial service links loading state",
  },
  "envGroups.createScopeLoading": {
    message: "Cargando entornos y servicios…",
    description:
      "Create dialog waits for a confirmed scope and link candidates",
  },
  "envGroups.createScopeError": {
    message:
      "No se pudo cargar el entorno o los servicios. Reintenta antes de crear el grupo.",
    description:
      "Create dialog blocks submission when scope or service lookup fails",
  },
  "envGroups.noServicesToLink": {
    message: "Este espacio de trabajo aún no tiene servicios que vincular.",
    description: "Initial service links empty state",
  },
  "envGroups.noServicesInScope": {
    message:
      "Ningún servicio está en este entorno. Los servicios solo se pueden vincular a un grupo de su propio entorno.",
    description: "Initial service links empty state for the picked scope",
  },
  "envGroups.invalidInitialContents": {
    message:
      "Corrige las claves de variable o los nombres de archivo no válidos antes de crear el grupo.",
    description: "Invalid initial env-group contents message",
  },
  "envGroups.nameLabel": {
    message: "Nombre del grupo",
    description: "Env-group name field label",
  },
  "envGroups.namePlaceholder": {
    message: "shared-production",
    description: "Env-group name placeholder",
  },
  "envGroups.invalidName": {
    message: "Introduce un nombre de grupo.",
    description: "Invalid env-group name message",
  },
  "envGroups.cancel": {
    message: "Cancelar",
    description: "Cancel an env-group dialog",
  },
  "envGroups.createSubmit": {
    message: "Crear grupo de entorno",
    description: "Create env-group submit button",
  },
  "envGroups.emptyTitle": {
    message: "No hay grupos de entorno",
    description: "Workspace env-groups empty-state title",
  },
  "envGroups.emptyBody": {
    message:
      "Crea un grupo para administrar la configuración compartida antes de vincularlo a un servicio.",
    description: "Workspace env-groups empty-state body",
  },
  "envGroups.serviceCount_one": {
    message: "{count} servicio vinculado",
    description: "Env-group linked-service count (singular)",
  },
  "envGroups.serviceCount_many": {
    message: "{count} de servicios vinculados",
    description: "Env-group linked-service count (plural)",
  },
  "envGroups.serviceCount_other": {
    message: "{count} servicios vinculados",
    description: "Env-group linked-service count (plural)",
  },
  "envGroups.metadataTitle": {
    message: "Metadatos del grupo",
    description: "Env-group metadata card title",
  },
  "envGroups.ownerLabel": {
    message: "Propietario",
    description: "Env-group owner id label",
  },
  "envGroups.environmentLabel": {
    message: "Entorno",
    description: "Environment Group scope label",
  },
  "envGroups.workspaceLabel": {
    message: "Espacio de trabajo",
    description: "Destination workspace label",
  },
  "envGroups.workspaceScope": {
    message: "Espacio de trabajo (sin entorno)",
    description: "Standalone Environment Group scope",
  },
  "envGroups.environmentLookupLoading": {
    message: "Cargando entorno…",
    description:
      "Group metadata Environment value while the scope lookup is still loading",
  },
  "envGroups.environmentLookupFailed": {
    message: "No se pudo cargar el entorno",
    description:
      "Group metadata Environment value when the scope lookup request failed",
  },
  "envGroups.unknownEnvironment": {
    message: "Entorno desconocido ({id})",
    description: "Deleted or inaccessible Environment fallback",
  },
  "envGroups.createdAtLabel": {
    message: "Creado",
    description: "Env-group created timestamp label",
  },
  "envGroups.updatedAtLabel": {
    message: "Actualizado",
    description: "Env-group updated timestamp label",
  },
  "envGroups.availabilityLabel": {
    message: "Estado",
    description: "Env-group availability label when busy or repair-required",
  },
  "envGroups.availabilityBusy": {
    message: "Guardado en curso: reintenta en breve",
    description: "Busy environment-group availability message",
  },
  "envGroups.availabilityRepairRequired": {
    message: "Se necesita recuperación: contacta con soporte si persiste",
    description: "Repair-required environment-group availability message",
  },
  "envGroups.unavailableTitle": {
    message: "Grupos de entorno no disponibles",
    description: "Secret store unavailable state title",
  },
  "envGroups.unavailableBody": {
    message: "El almacén de secretos no está configurado para este despliegue.",
    description: "Secret store unavailable state body",
  },
  "envGroups.forbiddenTitle": {
    message: "No autorizado",
    description: "Env-group forbidden state title",
  },
  "envGroups.forbiddenBody": {
    message: "No tienes permiso para ver ni administrar grupos de entorno.",
    description: "Env-group forbidden state body",
  },
  "envGroups.genericTitle": {
    message: "No se pudieron cargar los grupos de entorno",
    description: "Env-group generic query error title",
  },
  "envGroups.genericBody": {
    message: "Algo salió mal. Inténtalo de nuevo.",
    description: "Env-group generic query error body",
  },
  "envGroups.errorTitle": {
    message: "No se pudo cargar el contenido del grupo",
    description: "Env-group editor error title",
  },
  "envGroups.errorBody": {
    message: "Algo salió mal. Inténtalo de nuevo.",
    description: "Env-group editor error body",
  },
  "envGroups.backToList": {
    message: "Volver a los grupos de entorno",
    description: "Detail-page back button label",
  },
  "envGroups.renameButton": {
    message: "Renombrar",
    description: "Open rename env-group dialog",
  },
  "envGroups.manageButton": {
    message: "Administrar",
    description: "Open Environment Group management menu",
  },
  "envGroups.moveButton": {
    message: "Mover grupo",
    description: "Open Environment Group move dialog",
  },
  "envGroups.moveTitle": {
    message: "Mover grupo de entorno",
    description: "Environment Group move dialog title",
  },
  "envGroups.moveDescription": {
    message:
      "Elige un entorno compatible o devuelve este grupo al ámbito del espacio de trabajo.",
    description: "Environment Group move dialog description",
  },
  "envGroups.moveSubmit": {
    message: "Mover grupo",
    description: "Submit Environment Group move",
  },
  "envGroups.moveSuccess": {
    message: "Grupo de entorno movido",
    description: "Environment Group move success toast",
  },
  "envGroups.moveError": {
    message: "No se pudo mover el grupo",
    description: "Environment Group move error toast",
  },
  "envGroups.cloneButton": {
    message: "Clonar grupo",
    description: "Open Environment Group clone dialog",
  },
  "envGroups.cloneTitle": {
    message: "Clonar grupo de entorno",
    description: "Environment Group clone dialog title",
  },
  "envGroups.cloneDescription": {
    message:
      "Copia las variables y los archivos a un grupo nuevo. Los vínculos de servicio no se copian.",
    description: "Environment Group clone dialog description",
  },
  "envGroups.cloneSubmit": {
    message: "Clonar grupo",
    description: "Submit Environment Group clone",
  },
  "envGroups.cloneSuccess": {
    message: "{name} clonado",
    description: "Environment Group clone success toast",
  },
  "envGroups.cloneError": {
    message: "No se pudo clonar el grupo",
    description: "Environment Group clone error toast",
  },
  "envGroups.environmentPlaceholder": {
    message: "Selecciona un entorno",
    description: "Environment scope selector placeholder",
  },
  "envGroups.renameTitle": {
    message: "Renombrar grupo de entorno",
    description: "Rename env-group dialog title",
  },
  "envGroups.renameDescription": {
    message:
      "Renombrar conserva las variables, los archivos y los vínculos de servicio del grupo.",
    description: "Rename env-group dialog description",
  },
  "envGroups.renameSubmit": {
    message: "Guardar nombre",
    description: "Rename env-group submit button",
  },
  "envGroups.deleteButton": {
    message: "Eliminar",
    description: "Open delete env-group dialog",
  },
  "envGroups.deleteTitle": {
    message: "¿Eliminar {name}?",
    description: "Delete env-group dialog title",
  },
  "envGroups.deleteDescription": {
    message:
      "Esta acción no se puede deshacer. ¿Seguro que quieres eliminar este grupo de entorno? Se desvinculará de todos los servicios.",
    description: "Delete env-group dialog warning (Render-style copy)",
  },
  "envGroups.deletePrompt": {
    message: "Escribe {phrase} a continuación para confirmar.",
    description:
      "Body prompt naming the exact 'sudo delete env group <name>' phrase (rendered bold by SudoCommandField)",
  },
  "envGroups.deleteConfirm": {
    message: "Eliminar grupo de entorno",
    description: "Delete env-group confirm button",
  },
  "envGroups.varsTitle": {
    message: "Variables de entorno",
    description: "Env-group variable editor title",
  },
  "envGroups.varsDescription": {
    message:
      "Los valores se cifran y se aplican a todos los servicios vinculados.",
    description: "Env-group variable editor description",
  },
  "envGroups.varsEmptyTitle": {
    message: "No hay variables de entorno",
    description: "Env-group variable editor empty title",
  },
  "envGroups.varsEmptyBody": {
    message:
      "Añade una variable ahora; el grupo no necesita estar vinculado antes.",
    description: "Env-group variable editor empty body",
  },
  "envGroups.varDeleteConfirmBody": {
    message:
      "Los servicios vinculados se vuelven a desplegar sin esta variable, excepto un servicio basado en repositorio con el despliegue automático desactivado, que sigue sirviendo su versión actual y aplica el cambio en su próximo despliegue.",
    description: "Delete env-group variable warning",
  },
  "envGroups.filesTitle": {
    message: "Archivos secretos",
    description: "Env-group secret-file editor title",
  },
  "envGroups.filesDescription": {
    message:
      "Los archivos cifrados se montan en todos los servicios vinculados en /etc/secrets.",
    description: "Env-group secret-file editor description",
  },
  "envGroups.filesEmptyTitle": {
    message: "No hay archivos secretos",
    description: "Env-group secret-file editor empty title",
  },
  "envGroups.filesEmptyBody": {
    message:
      "Añade un archivo ahora; el grupo no necesita estar vinculado antes.",
    description: "Env-group secret-file editor empty body",
  },
  "envGroups.fileDeleteConfirmBody": {
    message:
      "Los servicios vinculados se vuelven a desplegar sin este archivo, excepto un servicio basado en repositorio con el despliegue automático desactivado, que sigue sirviendo su versión actual y aplica el cambio en su próximo despliegue.",
    description: "Delete env-group secret-file warning",
  },
  "envGroups.servicesTitle": {
    message: "Servicios vinculados",
    description: "Env-group linked-services card title",
  },
  "envGroups.servicesDescription": {
    message:
      "Vincular un servicio lo despliega ahora con los valores del grupo, sea cual sea su configuración de despliegue automático. Desvincularlo cambia su configuración: un servicio basado en repositorio con el despliegue automático desactivado lo aplica con su próximo despliegue; los demás servicios lo despliegan ahora.",
    description: "Env-group linked-services card description",
  },
  "envGroups.selectService": {
    message: "Selecciona un servicio",
    description: "Link-service selector placeholder",
  },
  "envGroups.linkButton": {
    message: "Vincular servicio",
    description: "Link selected service to env group",
  },
  "envGroups.unlinkButton": {
    message: "Desvincular",
    description: "Unlink service from env group",
  },
  "envGroups.unlinkServiceAria": {
    message: "Desvincular {name}",
    description:
      "Accessible name of a linked-service row's Unlink control, naming the service (w4/150)",
  },
  "envGroups.noLinkedServices": {
    message:
      "Este grupo aún no está vinculado a ningún servicio. Sigue siendo totalmente editable.",
    description: "No linked services state",
  },
  "envGroups.servicesLoadError": {
    message:
      "No se pudieron cargar los servicios del espacio de trabajo. Los vínculos existentes siguen disponibles abajo.",
    description: "Linked-services inventory error",
  },
  "envGroups.incompatibleLink": {
    message: "Vinculado fuera del entorno de este grupo",
    description: "Warning on an existing incompatible Environment Group link",
  },
  "envGroups.searchPlaceholder": {
    message: "Buscar grupos de entorno",
    description: "Environment Group list search placeholder",
  },
  "envGroups.searchLabel": {
    message: "Buscar grupos de entorno por nombre",
    description: "Accessible Environment Group search label",
  },
  "envGroups.resetSearch": {
    message: "Restablecer búsqueda",
    description: "Clear Environment Group search",
  },
  "envGroups.noMatchesTitle": {
    message: "Ningún grupo de entorno coincide",
    description: "Environment Group search empty-state title",
  },
  "envGroups.noMatchesBody": {
    message: "Prueba con otro nombre o restablece la búsqueda.",
    description: "Environment Group search empty-state body",
  },
  "envGroups.colName": {
    message: "Nombre del grupo de entorno",
    description: "Environment Group table name column",
  },
  "envGroups.colEnvironment": {
    message: "Entorno",
    description: "Environment Group table Environment column",
  },
  "envGroups.colEnvVars": {
    message: "Variables de entorno",
    description: "Environment Group table variable-count column",
  },
  "envGroups.colSecretFiles": {
    message: "Archivos secretos",
    description: "Environment Group table secret-file-count column",
  },
  "envGroups.colUpdated": {
    message: "Actualizado",
    description: "Environment Group table updated-time column",
  },
  "envGroups.rolloutNote": {
    message:
      "Los servicios vinculados se están volviendo a desplegar para aplicar el cambio, excepto un servicio basado en repositorio con el despliegue automático desactivado, que lo aplica con su próximo despliegue.",
    description: "Env-group write rollout toast detail",
  },
  "envGroups.linkChangeNote": {
    message:
      "El servicio se despliega ahora con los valores del grupo, sea cual sea su configuración de despliegue automático.",
    description: "Env-group link success toast detail",
  },
  "envGroups.unlinkChangeNote": {
    message:
      "La configuración del servicio cambió. Un servicio basado en repositorio con el despliegue automático desactivado la aplica con su próximo despliegue; cualquier otro servicio la despliega ahora.",
    description: "Env-group unlink success toast detail",
  },
  "envGroups.createSuccess": {
    message: "{name} creado",
    description: "Env-group create success toast",
  },
  "envGroups.createError": {
    message: "No se pudo crear {name}",
    description: "Env-group create error toast",
  },
  "envGroups.renameSuccess": {
    message: "Renombrado a {name}",
    description: "Env-group rename success toast",
  },
  "envGroups.renameError": {
    message: "No se pudo renombrar el grupo",
    description: "Env-group rename error toast",
  },
  "envGroups.deleteSuccess": {
    message: "Grupo de entorno eliminado",
    description: "Env-group delete success toast",
  },
  "envGroups.deleteError": {
    message: "No se pudo eliminar el grupo",
    description: "Env-group delete error toast",
  },
  "envGroups.linkSuccess": {
    message: "Servicio vinculado",
    description: "Env-group link success toast",
  },
  "envGroups.linkError": {
    message: "No se pudo vincular el servicio",
    description: "Env-group link error toast",
  },
  "envGroups.unlinkSuccess": {
    message: "Servicio desvinculado",
    description: "Env-group unlink success toast",
  },
  "envGroups.unlinkError": {
    message: "No se pudo desvincular el servicio",
    description: "Env-group unlink error toast",
  },
  "envGroups.varSaveSuccess": {
    message: "{key} guardada",
    description: "Env-group variable save success toast",
  },
  "envGroups.varsSaveSuccess": {
    message: "Variables de entorno guardadas",
    description: "Env-group replace-all variables success toast",
  },
  "envGroups.varsSaveError": {
    message: "No se pudieron guardar las variables de entorno",
    description: "Env-group replace-all variables error toast",
  },
  "envGroups.varSaveError": {
    message: "No se pudo guardar {key}",
    description: "Env-group variable save error toast",
  },
  "envGroups.varDeleteSuccess": {
    message: "{key} eliminada",
    description: "Env-group variable delete success toast",
  },
  "envGroups.varDeleteError": {
    message: "No se pudo eliminar {key}",
    description: "Env-group variable delete error toast",
  },
};

export default esEnvGroups;
