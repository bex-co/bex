import type { TranslationEntry } from "@/i18n";

const esEnvironments: Record<string, TranslationEntry> = {
  "environments.heading": {
    message: "Entornos",
    description: "Section heading above a project page's environments list",
  },
  "environments.selectorLabel": {
    message: "Entorno seleccionado",
    description:
      "Accessible label for the Project overview Environment selector",
  },
  "environments.unassignedOption": {
    message: "Sin asignar",
    description: "Project resources that belong to no Environment",
  },
  "environments.newButton": {
    message: "Nuevo entorno",
    description: "Button that opens the new-environment dialog",
  },
  "environments.emptyBody": {
    message:
      "Aún no hay entornos. Crea uno (p. ej., staging o production) para agrupar los recursos de este proyecto.",
    description: "Empty state shown when a project has no environments",
  },
  "environments.errorBody": {
    message:
      "Algo salió mal al cargar los entornos. Inténtalo de nuevo en unos instantes.",
    description: "Error state shown when the environments query fails",
  },
  "environments.refreshErrorBody": {
    message:
      "No se pudieron actualizar los entornos. Se muestran los últimos datos cargados.",
    description:
      "Inline notice when a background refresh of environments fails while the previously loaded environments stay on screen",
  },
  "environments.resourceCount_one": {
    message: "{count} recurso",
    description:
      "Count of visible services+databases+key-value instances assigned to an environment, next to its name (singular)",
  },
  "environments.resourceCount_many": {
    message: "{count} de recursos",
    description:
      "Count of visible services+databases+key-value instances assigned to an environment, next to its name (plural)",
  },
  "environments.resourceCount_other": {
    message: "{count} recursos",
    description:
      "Count of visible services+databases+key-value instances assigned to an environment, next to its name (plural)",
  },
  "environments.manageButton": {
    message: "Administrar recursos",
    description:
      "Button on an environment card that opens the manage-resources dialog",
  },
  "environments.moreActions": {
    message: "Más acciones",
    description:
      "Accessible label for an environment card's overflow (•••) menu",
  },
  "environments.renameAction": {
    message: "Renombrar",
    description: "Environment overflow-menu item that opens the rename dialog",
  },
  "environments.deleteAction": {
    message: "Eliminar",
    description:
      "Environment overflow-menu item that opens the delete confirmation, and the confirm button",
  },
  "environments.cardEmpty": {
    message: "Aún no hay recursos en este entorno.",
    description:
      "Shown inside an environment card when it has no assigned resources",
  },
  "environments.createTitle": {
    message: "Nuevo entorno",
    description: "New-environment dialog title",
  },
  "environments.createDescription": {
    message:
      "Agrupa un subconjunto de los recursos de este proyecto bajo un nombre, como staging o production.",
    description: "New-environment dialog description",
  },
  "environments.fieldName": {
    message: "Nombre",
    description: "New-environment dialog name field label",
  },
  "environments.fieldNamePlaceholder": {
    message: "staging",
    description: "New-environment dialog name field placeholder",
  },
  "environments.cancel": {
    message: "Cancelar",
    description: "Cancel button shared by the environment dialogs",
  },
  "environments.createSubmit": {
    message: "Crear entorno",
    description: "New-environment dialog submit button",
  },
  "environments.createSuccess": {
    message: "Entorno “{name}” creado.",
    description: "Toast shown after an environment is created",
  },
  "environments.createRefreshFailed": {
    message:
      "Entorno “{name}” creado, pero no se pudo actualizar la lista. Inténtalo de nuevo o recarga para verlo.",
    description:
      "Warning after the environment was created but re-reading the list failed; creation must not be retried or reported as failed (w4/231)",
  },
  "environments.createError": {
    message: "No se pudo crear el entorno “{name}”.",
    description: "Toast shown when creating an environment fails",
  },
  "environments.renameFieldLabel": {
    message: "Nombre del entorno",
    description:
      'Accessible/visible label for the environment rename field. Deliberately more specific than environments.fieldName\'s bare "Name" (w4/134)',
  },
  "environments.renameTitle": {
    message: "Renombrar entorno",
    description: "Rename-environment dialog title",
  },
  "environments.renameSubmit": {
    message: "Guardar",
    description: "Rename-environment dialog submit button",
  },
  "environments.renameSuccess": {
    message: "Entorno renombrado a “{name}”.",
    description: "Toast shown after an environment is renamed",
  },
  "environments.renameError": {
    message: "No se pudo renombrar el entorno a “{name}”.",
    description: "Toast shown when renaming an environment fails",
  },
  "environments.deleteConfirmTitle": {
    message: "¿Eliminar el entorno “{name}”?",
    description: "Delete-environment confirmation dialog title",
  },
  "environments.deleteConfirmBody": {
    message:
      "Sus grupos de entorno se conservan pero quedan sin asignar: dejan de pertenecer a este entorno o proyecto y permanecen en Grupos de entorno. Esta acción no se puede deshacer.",
    description: "Delete-environment confirmation dialog body",
  },
  "environments.deleteConfirmBodyResources": {
    message:
      "Eliminar este entorno elimina todos sus servicios, bases de datos y almacenes de Key Value: {resources}. Sus grupos de entorno se conservan pero quedan sin asignar: dejan de pertenecer a este entorno o proyecto y permanecen en Grupos de entorno. Esta acción no se puede deshacer.",
    description:
      "Delete-environment confirmation body when services, databases or key value stores would be deleted (w4/m264, w6/m290, Render parity)",
  },
  "environments.deleteSuccess": {
    message: "Entorno “{name}” eliminado.",
    description: "Toast shown after an environment is deleted",
  },
  "environments.deleteRefreshFailed": {
    message:
      "Entorno “{name}” eliminado, pero no se pudo actualizar la lista. Inténtalo de nuevo o recarga para actualizarla.",
    description:
      "Warning after the environment was deleted but re-reading the list failed; deletion must not be retried or reported as failed (w4/290)",
  },
  "environments.deleteError": {
    message: "No se pudo eliminar el entorno “{name}”.",
    description: "Toast shown when deleting an environment fails",
  },
  "environments.manageTitle": {
    message: "Administrar recursos en “{name}”",
    description: "Manage-resources dialog title",
  },
  "environments.manageDescription": {
    message:
      "Marca los recursos que pertenecen a este entorno. Asignar un recurso también lo añade a este proyecto.",
    description: "Manage-resources dialog description",
  },
  "environments.tabServices": {
    message: "Servicios",
    description: "Manage-resources dialog tab label for the services checklist",
  },
  "environments.tabDatabases": {
    message: "Bases de datos",
    description:
      "Manage-resources dialog tab label for the databases checklist",
  },
  "environments.tabKeyValues": {
    message: "Key Value",
    description:
      "Manage-resources dialog tab label for the key-value checklist",
  },
  "environments.manageNoServices": {
    message: "Este espacio de trabajo aún no tiene servicios que asignar.",
    description:
      "Manage-resources dialog empty state when the workspace has no services",
  },
  "environments.manageNoDatabases": {
    message: "Este espacio de trabajo aún no tiene bases de datos que asignar.",
    description:
      "Manage-resources dialog empty state when the workspace has no databases",
  },
  "environments.manageNoKeyValues": {
    message:
      "Este espacio de trabajo aún no tiene instancias de Key Value que asignar.",
    description:
      "Manage-resources dialog empty state when the workspace has no key-value instances",
  },
  "environments.manageResourceIn": {
    message: "En {environment}",
    description:
      "Under a Manage resources row: the other environment the resource is in now ({environment} = its name)",
  },
  "environments.markerProtected": {
    message: "Protegido",
    description:
      "Marker: the environment the resource would leave is protected",
  },
  "environments.markerIsolated": {
    message: "Aislado",
    description:
      "Marker: the environment the resource would leave has network isolation",
  },
  "environments.markerAllowList": {
    message: "Lista de IP permitidas",
    description:
      "Marker: the environment the resource would leave restricts inbound IPs",
  },
  "environments.manageMovesOut": {
    message: "Sale de {environment}: {resources}",
    description:
      "Before Save: the selected resources this save moves out of another environment ({resources} = comma-separated names)",
  },
  "environments.manageNeedsAdminToMove": {
    message: "Solo un administrador puede sacarlo de {environment}.",
    description:
      "Why a Manage resources row is disabled: its current environment is admin-managed",
  },
  "environments.manageUnsaved": {
    message: "Cambios sin guardar",
    description:
      "Accessible description of a Manage resources tab whose selection differs from the saved membership",
  },
  "environments.manageSubmit": {
    message: "Guardar",
    description: "Manage-resources dialog submit button",
  },
  "environments.assignSuccess": {
    message: "Servicios de “{name}” actualizados.",
    description: "Toast shown after an environment's services are updated",
  },
  "environments.strandedMoveTitle": {
    message: "Este traslado deja grupos vinculados atrás",
    description:
      "Confirmation title shown before a move that strands an environment-scoped group link (w4/m238)",
  },
  "environments.strandedMoveBody": {
    message:
      "Estos grupos de entorno siguen vinculados al servicio que se traslada, y sus variables y archivos siguen aplicándose a él hasta que el grupo se desvincule o se mueva:",
    description:
      "Confirmation body: the stranded groups keep feeding the service after the move",
  },
  "environments.strandedMoveConfirm": {
    message: "Mover de todos modos",
    description: "Confirm button on the stranded-groups move confirmation",
  },
  "environments.strandedScopeWorkspace": {
    message: "Espacio de trabajo",
    description:
      "Scope label for a stranded group that is scoped to the whole workspace",
  },
  "environments.strandedScopeUnknown": {
    message:
      "Aún no se pudieron comprobar los grupos de entorno vinculados: espera a que cargue la lista e inténtalo de nuevo.",
    description:
      "Toast shown when a move is attempted before the group/scope reads resolve",
  },
  "environments.assignError": {
    message: "No se pudieron actualizar los servicios de “{name}”.",
    description: "Toast shown when updating an environment's services fails",
  },
  "environments.assignDatabasesSuccess": {
    message: "Bases de datos de “{name}” actualizadas.",
    description: "Toast shown after an environment's databases are updated",
  },
  "environments.assignDatabasesError": {
    message: "No se pudieron actualizar las bases de datos de “{name}”.",
    description: "Toast shown when updating an environment's databases fails",
  },
  "environments.assignDatabasesRefreshFailed": {
    message:
      "Bases de datos de “{name}” actualizadas, pero no se pudo actualizar la lista. Inténtalo de nuevo o recarga para verlas.",
    description:
      "Warning after the database membership was saved but re-reading the environments list failed; the write must not be retried or reported as failed (w4/325)",
  },
  "environments.assignKeyValuesSuccess": {
    message: "Instancias de Key Value de “{name}” actualizadas.",
    description:
      "Toast shown after an environment's key-value instances are updated",
  },
  "environments.assignKeyValuesRefreshFailed": {
    message:
      "Instancias de Key Value de “{name}” actualizadas, pero no se pudo actualizar la lista. Inténtalo de nuevo o recarga para verlas.",
    description:
      "Warning after the key-value membership was saved but re-reading the environments list failed; the write must not be retried or reported as failed (w4/263)",
  },
  "environments.assignKeyValuesError": {
    message:
      "No se pudieron actualizar las instancias de Key Value de “{name}”.",
    description:
      "Toast shown when updating an environment's key-value instances fails",
  },
  "environments.tabEnvGroups": {
    message: "Grupos de entorno",
    description: "Manage-resources dialog tab label for environment groups",
  },
  "environments.manageNoEnvGroups": {
    message:
      "Este espacio de trabajo aún no tiene grupos de entorno que asignar.",
    description: "Environment-groups checklist empty state",
  },
  "environments.assignEnvGroupsSuccess": {
    message: "Grupos de entorno de “{name}” actualizados.",
    description: "Toast shown after environment-group membership is updated",
  },
  "environments.assignEnvGroupsPlacementUnread": {
    message:
      "No se pudo confirmar dónde están ahora los grupos de “{name}”. Inténtalo de nuevo o recarga antes de volver a moverlos.",
    description:
      "Warning after an environment-group move failed and re-reading the groups' placement failed too; its action re-reads, it never moves again (w4/m206)",
  },
  "environments.assignEnvGroupsError": {
    message: "No se pudieron actualizar los grupos de entorno de “{name}”.",
    description:
      "Toast shown when environment-group membership fails to update",
  },
  "environments.settingsAction": {
    message: "Toda la configuración",
    description: "Environment overflow-menu action opening ACL settings",
  },
  "environments.settingsTitle": {
    message: "Configuración de “{name}”",
    description: "Environment ACL settings dialog title",
  },
  "environments.settingsDescription": {
    message:
      "Administra los permisos, el aislamiento de la red privada y las reglas de IP de entrada.",
    description: "Environment ACL settings dialog description",
  },
  "environments.protectedBadge": {
    message: "Protegido",
    description: "Badge shown beside a protected environment name",
  },
  "environments.protectedLabel": {
    message: "Entorno protegido",
    description: "Protected-status setting label",
  },
  "environments.protectedHint": {
    message:
      "Solo los administradores del espacio de trabajo pueden eliminar, suspender, reanudar o activar y desactivar el modo de mantenimiento de los recursos de este entorno.",
    description: "Protected-status setting explanation",
  },
  "environments.isolationLabel": {
    message: "Bloquear conexiones entre entornos",
    description: "Private-network isolation setting label",
  },
  "environments.isolationProOnly": {
    message:
      "El aislamiento de red requiere el plan Pro o superior. Desactívalo aquí si se habilitó antes.",
    description: "Isolation setting explanation on a Hobby workspace (w8/m138)",
  },
  "environments.isolationHint": {
    message:
      "Impide que el tráfico de la red privada cruce el límite de este entorno, incluido el que va a sus instancias de Postgres y Key Value. Sus URL externas siguen sus reglas de IP.",
    description: "Private-network isolation setting explanation",
  },
  "environments.ipAllowListLabel": {
    message: "Restricciones de IP de entrada",
    description: "Environment inbound-IP allowlist heading",
  },
  "environments.ipAllowListHint": {
    message:
      "Solo los rangos CIDR de la lista pueden acceder a los servicios públicos, los Static Sites y los almacenes de datos de este entorno. Además, el origen debe cumplir las reglas de IP propias de cada recurso.",
    description: "Environment inbound-IP allowlist explanation",
  },
  "environments.ipAllowListOpen": {
    message:
      "Sin reglas: los miembros públicos de este entorno DENEGARÁN todo el tráfico. Añade 0.0.0.0/0 y ::/0 para permitirlo todo.",
    description: "Empty environment inbound-IP allowlist state (deny-all)",
  },
  "environments.ipAllowListRemove": {
    message: "Quitar {cidr}",
    description: "Accessible label for an inbound-IP rule remove button",
  },
  "environments.ipAllowListRuleCIDR": {
    message: "Bloque CIDR de la regla {number}",
    description: "Accessible label for an existing inbound-IP rule CIDR",
  },
  "environments.ipAllowListRuleDescription": {
    message: "Descripción de la regla {number}",
    description: "Accessible label for an existing inbound-IP rule description",
  },
  "environments.ipAllowListNewCIDR": {
    message: "Nuevo bloque CIDR",
    description: "Accessible label for a new inbound-IP rule CIDR",
  },
  "environments.ipAllowListNewDescription": {
    message: "Descripción de la nueva regla",
    description: "Accessible label for a new inbound-IP rule description",
  },
  "environments.ipAllowListDescriptionPlaceholder": {
    message: "Descripción (opcional)",
    description: "Placeholder for an inbound-IP rule description",
  },
  "environments.ipAllowListScalePlanOnly": {
    message:
      "Las reglas de IP de entrada para entornos requieren el plan Scale o superior. Las reglas ya definidas siguen aplicándose y se pueden restablecer.",
    description:
      "Hint replacing the editable IP rules below the Scale plan (w8/m137)",
  },
  "environments.ipAllowListResetAllowAll": {
    message: "Restablecer para permitir todo",
    description:
      "Button resetting an environment's inbound IP rules to allow-all below the Scale plan (w8/m137)",
  },
  "environments.ipAllowListInvalid": {
    message: "Introduce bloques CIDR IPv4 o IPv6 válidos y únicos.",
    description:
      "Inline validation when an environment allow list block is not a unique, valid CIDR",
  },
  "environments.ipAllowListAdd": {
    message: "Añadir origen",
    description: "Button adding an inbound-IP CIDR rule",
  },
  "environments.settingsSave": {
    message: "Guardar",
    description: "Environment ACL settings save button",
  },
  "environments.aclSaveSuccess": {
    message: "Configuración de “{name}” guardada.",
    description: "Toast shown after an Environment ACL is saved",
  },
  "environments.aclSaveError": {
    message: "No se pudo guardar la configuración de “{name}”.",
    description: "Toast shown when an Environment ACL save fails",
  },
  "environments.resourceCountLoading": {
    message: "Contando…",
    description:
      "Environment card header while its resource lists are still loading, instead of a count",
  },
  "environments.resourceCountUnknown": {
    message: "Recuento no disponible",
    description:
      "Environment card header when a resource list has not answered, instead of a count",
  },
  "environments.resourcesUnavailable": {
    message:
      "Algunos recursos no se pudieron cargar, por lo que este entorno puede estar incompleto o desactualizado.",
    description:
      "Inline alert on a Project environment card when a resource list failed to load; offers Try again",
  },
  "environments.resourcesLoading": {
    message: "Cargando recursos…",
    description:
      "Environment card line while its resource lists are still loading, instead of the empty message",
  },
  "environments.assignmentTitle": {
    message: "Proyecto",
    description: "Shared create-form Project/Environment selector heading",
  },
  "environments.assignmentProject": {
    message: "Proyecto",
    description: "Accessible label for a create-form Project selector",
  },
  "environments.assignmentProjectNone": {
    message: "Sin proyecto",
    description: "Unassigned Project selector option",
  },
  "environments.assignmentEnvironment": {
    message: "Entorno",
    description: "Accessible label for a create-form Environment selector",
  },
  "environments.assignmentEnvironmentNone": {
    message: "Sin entorno",
    description: "Unassigned Environment selector option",
  },
  "environments.assignmentSelectedLoading": {
    message: "Cargando…",
    description:
      "Label for a kept Project/Environment selection while its list is still loading",
  },
  "environments.assignmentSelectedUnavailable": {
    message: "Seleccionado (nombre no disponible)",
    description:
      "Label for a kept Project/Environment selection whose list failed to load",
  },
  "environments.assignmentReadError": {
    message:
      "No se pudieron cargar los proyectos o entornos. Inténtalo de nuevo; tu selección se conserva.",
    description:
      "Inline alert when the create form's Project or Environment list fails to load; the selection is kept",
  },
  "environments.assignmentHint": {
    message:
      "Al crear un recurso, seleccionar un proyecto filtra sus entornos. Elige un entorno para añadir el recurso a ese proyecto. Si dejas seleccionado Sin entorno, el recurso se crea fuera de cualquier proyecto. Si el proyecto aún no tiene entornos, crea uno primero desde la página del proyecto.",
    description: "Shared create-form Environment-assignment hint",
  },
  "environments.protectedPlacementAdminOnly": {
    message:
      "Solo un administrador puede crear recursos en un entorno protegido o moverlos hacia dentro o fuera de él.",
    description:
      "Why a create or move control that targets (or leaves) a protected environment is disabled for a non-admin, and the message for the same server refusal (w3/m281)",
  },
  "environments.protectedOptionAdminOnly": {
    message: "{environment} (solo administradores)",
    description:
      "A protected environment in a create or move picker that a non-admin cannot choose ({environment} = its name)",
  },
  "environments.assignmentDescription": {
    message: "Agrégalo a un proyecto y un entorno al crearlo.",
    description: "Shared create-form Project row description",
  },
  "environments.assignmentNoEnvironments": {
    message:
      "Este proyecto aún no tiene entornos. Crea uno primero desde la página del proyecto.",
    description:
      "Shared create-form note when the chosen Project has no Environment to add the resource to",
  },
  "environments.assignmentPickEnvironment": {
    message:
      "Elige un entorno para añadir el recurso a este proyecto. Con «Sin entorno», se crea fuera de cualquier proyecto.",
    description:
      "Shared create-form note when a Project is chosen but No environment is: only an Environment adds the resource to the Project",
  },
  "environments.assignmentEnvironmentPickProject": {
    message: "Elige primero un proyecto",
    description:
      "Shared create-form Environment select while no Project is chosen (it is disabled until one is)",
  },
};

export default esEnvironments;
