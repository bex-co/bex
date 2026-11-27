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
  "environments.deleteSuccess": {
    message: "Entorno {name} eliminado correctamente.",
    description:
      "Toast shown after an environment is deleted (Render's words, w1/m683)",
  },
  "environments.deleteRefreshFailed": {
    message:
      "Entorno “{name}” eliminado, pero no se pudo actualizar la lista. Inténtalo de nuevo o recarga para actualizarla.",
    description:
      "Warning after the environment was deleted but re-reading the list failed; deletion must not be retried or reported as failed (w4/290)",
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
  "environments.tabSandboxGroups": {
    message: "Grupos de sandbox",
    description:
      "Manage-resources dialog tab label for the workspace's sandbox groups",
  },
  "environments.manageNoSandboxGroups": {
    message:
      "Este espacio de trabajo aún no tiene grupos de sandbox que asignar.",
    description: "Sandbox-groups checklist empty state",
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
  "environments.protectedBadge": {
    message: "Protegido",
    description: "Badge shown beside a protected environment name",
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
  "environments.protectedDescription": {
    message: "Entorno protegido",
    description:
      "Line under a protected environment's section heading on the project page (Render's words, w1/m682)",
  },
  "environments.emptyTitle": {
    message: "{name} está vacío",
    description:
      "Heading of an environment section with no services, Postgres or Key Value; {name} is the environment (Render's words, w1/m682)",
  },
  "environments.emptyBodyCreate": {
    message: "Pon en marcha tu entorno creando un servicio nuevo.",
    description:
      "An empty environment's sentence when the workspace has nothing to move into it (Render's words, w1/m682)",
  },
  "environments.emptyBodyCreateOrMove": {
    message:
      "Pon en marcha tu entorno creando un servicio nuevo o moviendo uno existente.",
    description:
      "An empty environment's sentence when the workspace has resources that could move into it (Render's words, w1/m682)",
  },
  "environments.createNewService": {
    message: "Crear servicio nuevo",
    description:
      "An empty environment's button to the create flow with the project and environment chosen (Render's words, w1/m682)",
  },
  "environments.moveExistingServices": {
    message: "Mover servicios existentes",
    description:
      "An empty environment's button that moves existing resources into it (Render's words, w1/m682)",
  },
  "environments.noEnvGroups": {
    message: "Este entorno no tiene grupos de entorno.",
    description:
      "An environment section's Env Groups view when the environment has none (Render's words, w1/m682)",
  },
  "environments.unassignedDescription": {
    message: "Recursos de este proyecto que no están en ningún entorno.",
    description:
      "Line under the project page's Unassigned section, which lists the project's resources in no environment (a bex section; Render requires an environment)",
  },
  "environments.settingsPageTitle": {
    message: "Configuración del entorno",
    description:
      "Heading of an environment's settings page, /project/<id>/environment/<envId>/settings (Render's words, w1/m683)",
  },
  "environments.settingsNavigation": {
    message: "Secciones de la configuración del entorno",
    description:
      "Accessible name of the environment settings page's section navigation (w1/m683)",
  },
  "environments.settingsEdit": {
    message: "Editar",
    description:
      "Button on an environment settings card that makes its field editable (Render's words, w1/m683)",
  },
  "environments.settingsCardSave": {
    message: "Guardar",
    description:
      "Button that saves an environment settings card's change (Render's words, w1/m683)",
  },
  "environments.settingsUpdateError": {
    message:
      "Se produjo un error al actualizar el entorno. Inténtalo de nuevo.",
    description:
      "Inline error on an environment settings card when the save failed for a reason the server did not name (Render's words, w1/m683)",
  },
  "environments.nameCardTitle": {
    message: "Nombre",
    description:
      "Title of the environment settings page's Name card, and its navigation entry (Render's words, w1/m683)",
  },
  "environments.nameCardDescription": {
    message: "Un nombre único para tu entorno",
    description:
      "Description of the environment settings page's Name card (Render's words, w1/m683)",
  },
  "environments.nameFieldLabel": {
    message: "Nombre del entorno",
    description:
      "Accessible label of the Name card's field (Render's words, w1/m683)",
  },
  "environments.nameRequired": {
    message: "El nombre del entorno es obligatorio",
    description:
      "Error under the Name card's field when it is saved empty (Render's words, w1/m683)",
  },
  "environments.nameInUse": {
    message: "Otro entorno de este proyecto ya usa este nombre",
    description:
      "Error under the Name card's field when another environment of the project has the name (Render's words, w1/m683)",
  },
  "environments.permissionsCardTitle": {
    message: "Permisos",
    description:
      "Title of the environment settings page's Permissions card, and its navigation entry (Render's words, w1/m683)",
  },
  "environments.permissionsDefaultBody": {
    message:
      "De forma predeterminada, todos los miembros del equipo pueden eliminar y suspender recursos, mover servicios y administrar la red de las bases de datos.",
    description:
      "First paragraph of the Permissions card (Render's words, w1/m683)",
  },
  "environments.permissionsProtectedBody": {
    message:
      "Si se marca como {protectedEnvironment}, solo los administradores pueden eliminar y suspender recursos, mover servicios y administrar la red de las bases de datos.",
    description:
      "Second paragraph of the Permissions card; {protectedEnvironment} is a link whose text is environments.permissionsProtectedLink (Render's words, w1/m683)",
  },
  "environments.permissionsProtectedLink": {
    message: "entorno protegido",
    description:
      "Link text inside environments.permissionsProtectedBody, to the projects and environments docs (Render's words, w1/m683)",
  },
  "environments.permissionsLegend": {
    message: "Permisos del entorno",
    description:
      "Accessible name of the Permissions card's Default / Protected choice (Render's words, w1/m683)",
  },
  "environments.permissionsDefault": {
    message: "Predeterminado",
    description:
      "The Permissions card's option for an unprotected environment (Render's words, w1/m683)",
  },
  "environments.permissionsDefaultHint": {
    message: "Para staging y desarrollo",
    description:
      "Line under the Permissions card's Default option (Render's words, w1/m683)",
  },
  "environments.permissionsProtected": {
    message: "Protegido",
    description:
      "The Permissions card's option for a protected environment (Render's words, w1/m683)",
  },
  "environments.permissionsProtectedHint": {
    message: "Ideal para producción",
    description:
      "Line under the Permissions card's Protected option (Render's words, w1/m683)",
  },
  "environments.removeRestrictionsTitle": {
    message: "Quitar restricciones",
    description:
      "Title of the dialog that confirms unprotecting an environment, or allowing its cross-environment connections again (Render's words, w1/m683)",
  },
  "environments.removeProtectionBody": {
    message:
      "Quitar las restricciones del entorno {name} permitirá que todos los miembros del equipo realicen acciones destructivas. ¿Seguro que quieres hacerlo?",
    description:
      "Body of the dialog that confirms unprotecting an environment; {name} is the environment (Render's words, w1/m683)",
  },
  "environments.removeRestrictionsConfirm": {
    message: "Sí, quitar restricciones",
    description:
      "Confirm button that unprotects an environment, or allows its cross-environment connections again (Render's words, w1/m683)",
  },
  "environments.connectionsNavLabel": {
    message: "Conexiones",
    description:
      "Navigation entry of the environment settings page's Cross-Environment Connections card (Render's words, w1/m683)",
  },
  "environments.connectionsCardTitle": {
    message: "Conexiones entre entornos",
    description:
      "Title of the environment settings page's connections card (Render's words, w1/m683)",
  },
  "environments.connectionsBody": {
    message:
      "Impide que el tráfico de la red privada cruce el límite del entorno, sin dejar de permitir el tráfico entre los servicios de este entorno.",
    description:
      "First paragraph of the Cross-Environment Connections card (Render's words, w1/m683)",
  },
  "environments.connectionsBodyOutside": {
    message:
      "Los servicios de este entorno no podrán comunicarse con servicios de bex fuera de él.",
    description:
      "Second paragraph of the Cross-Environment Connections card (Render's words, with bex for Render, w1/m683)",
  },
  "environments.connectionsLegend": {
    message: "Conexiones entre entornos",
    description:
      "Accessible name of the Cross-Environment Connections card's Allowed / Blocked choice (w1/m683)",
  },
  "environments.connectionsAllowed": {
    message: "Permitidas",
    description:
      "The Cross-Environment Connections card's option for an environment whose private traffic may cross its boundary (Render's words, w1/m683)",
  },
  "environments.connectionsAllowedHint": {
    message: "Predeterminado para entornos nuevos",
    description:
      "Line under the Cross-Environment Connections card's Allowed option (Render's words, w1/m683)",
  },
  "environments.connectionsBlocked": {
    message: "Bloqueadas",
    description:
      "The Cross-Environment Connections card's option for an isolated environment (Render's words, w1/m683)",
  },
  "environments.connectionsBlockedHint": {
    message: "Recomendado para entornos de producción",
    description:
      "Line under the Cross-Environment Connections card's Blocked option (Render's words, w1/m683)",
  },
  "environments.connectionsPaywall": {
    message:
      "Mejora al plan {plan} para bloquear las conexiones entre entornos.",
    description:
      "Notice on the Cross-Environment Connections card below Pro; {plan} is the Pro plan's name (Render's words, w1/m683)",
  },
  "environments.upgrade": {
    message: "Mejorar plan",
    description:
      "Button or link that opens the change-plan dialog from an environment's plan gate (Render's words, w1/m683)",
  },
  "environments.isolateTitle": {
    message: "Restringir conexiones",
    description:
      "Title of the dialog that confirms blocking an environment's cross-environment connections, and its confirm button (Render's words, w1/m683)",
  },
  "environments.isolateDescription": {
    message: "Confirma que quieres aislar las conexiones de {environment}",
    description:
      "Description of the dialog that confirms blocking or allowing an environment's cross-environment connections; {environment} is <project> / <environment> (Render's words, w1/m683)",
  },
  "environments.isolateRestrictBody": {
    message:
      "Bloquea el tráfico de la red privada hacia y desde tus servicios de bex fuera de este entorno. Esto puede interrumpir los servicios que dependen del tráfico entre entornos.",
    description:
      "First paragraph of the dialog that confirms blocking cross-environment connections (Render's words, with bex for Render, w1/m683)",
  },
  "environments.isolateRestrictRestart": {
    message:
      "Las conexiones de red ya abiertas no se verán afectadas. Reinicia los servicios de este entorno para asegurarte de que se cierren todas las conexiones existentes.",
    description:
      "Second paragraph of the dialog that confirms blocking cross-environment connections (Render's words, w1/m683)",
  },
  "environments.isolateRemoveBody": {
    message:
      "El tráfico de la red privada hacia y desde tus servicios de bex fuera de este entorno dejará de estar bloqueado.",
    description:
      "Body of the dialog that confirms allowing cross-environment connections again (Render's words, with bex for Render, w1/m683)",
  },
  "environments.isolateQuestion": {
    message: "¿Seguro que quieres hacerlo?",
    description:
      "Last line of the dialog that confirms blocking or allowing cross-environment connections (Render's words, w1/m683)",
  },
  "environments.isolationBlockedToast": {
    message: "Conexiones entre entornos bloqueadas para {name}.",
    description:
      "Toast after the section menu's toggle blocks an environment's cross-environment connections (Render's words, w1/m683)",
  },
  "environments.isolationAllowedToast": {
    message: "Conexiones entre entornos permitidas para {name}.",
    description:
      "Toast after the section menu's toggle allows an environment's cross-environment connections again (Render's words, w1/m683)",
  },
  "environments.menuBlockConnections": {
    message: "Bloquear conexiones entre entornos",
    description:
      "The environment section's ••• menu toggle that isolates the environment (Render's words, w1/m683)",
  },
  "environments.menuBlockConnectionsHint": {
    message:
      "Impide que el tráfico de la red privada cruce el límite del entorno.",
    description:
      "Line under the ••• menu's Block cross-environment connections toggle (Render's words, w1/m683)",
  },
  "environments.menuUpgradeToUnlock": {
    message: "{upgrade} para desbloquear esta función",
    description:
      "The ••• menu's plan gate on Block cross-environment connections below Pro; {upgrade} is a link whose text is environments.upgrade (Render's words, w1/m683)",
  },
  "environments.envGroupsCardTitle": {
    message: "Grupos de entorno",
    description:
      "Title of the environment settings page's Environment Groups card, and its navigation entry (Render's words, w1/m683)",
  },
  "environments.envGroupsNew": {
    message: "Nuevo grupo de entorno",
    description:
      "Button on the Environment Groups card that creates an env group in the environment (Render's words, w1/m683)",
  },
  "environments.envGroupsSearch": {
    message: "Buscar grupos de entorno",
    description:
      "Placeholder and accessible name of the Environment Groups card's search (Render's words, w1/m683)",
  },
  "environments.envGroupsNone": {
    message: "No se encontraron grupos de entorno",
    description:
      "The Environment Groups card when the environment has no env groups (Render's words, w1/m683)",
  },
  "environments.networkingCardTitle": {
    message: "Redes",
    description:
      "Title of the environment settings page's Networking card (inbound IP rules), and its navigation entry (Render's words, w1/m683)",
  },
  "environments.ipAllowListMoveUp": {
    message: "Subir {cidr}",
    description:
      "Accessible name of an environment IP rule's move-up button; {cidr} is the rule's CIDR block",
  },
  "environments.ipAllowListMoveDown": {
    message: "Bajar {cidr}",
    description:
      "Accessible name of an environment IP rule's move-down button; {cidr} is the rule's CIDR block",
  },
  "environments.ipAllowListAllowsAll": {
    message: "Abierto a todas las IP de origen",
    description:
      "The Networking card below Scale when the environment's rules are the allow-all pair",
  },
  "environments.ipAllowListSaved": {
    message: "Reglas de IP de entrada de {name} guardadas.",
    description:
      "Toast after the Networking card saves an environment's inbound IP rules",
  },
  "environments.deleteCardTitle": {
    message: "Eliminar entorno",
    description:
      "Title of the environment settings page's delete card, and its navigation entry (Render's words, w1/m683)",
  },
  "environments.deleteCardDescription": {
    message:
      "Este entorno, incluidos sus servicios, se eliminará de forma permanente. Esta acción no se puede deshacer.",
    description:
      "Description of the Delete Environment card (Render's words, w1/m683)",
  },
  "environments.deleteLastEnvironment": {
    message:
      "Este entorno no se puede eliminar porque los proyectos necesitan al menos un entorno. Considera eliminar el {project} en su lugar.",
    description:
      "Notice on the Delete Environment card for a project's only environment; {project} is a link whose text is environments.deleteLastEnvironmentProject (Render's words, w1/m683)",
  },
  "environments.deleteLastEnvironmentProject": {
    message: "proyecto",
    description:
      "Link text inside environments.deleteLastEnvironment, to the project's settings (Render's words, w1/m683)",
  },
  "environments.deleteButton": {
    message: "Eliminar entorno",
    description:
      "The Delete Environment card's button, the confirmation dialog's title and its confirm button (Render's words, w1/m683)",
  },
  "environments.deleteDialogBody": {
    message:
      "Este entorno se eliminará de forma permanente, junto con todos sus servicios. Para conservar algún servicio, sácalo primero del entorno.",
    description:
      "Body of the dialog that confirms deleting an environment (Render's words, w1/m683)",
  },
  "environments.deleteDialogEnvGroupsKept": {
    message:
      "Sus grupos de entorno se conservan pero quedan sin asignar: dejan de pertenecer a este entorno o proyecto y permanecen en Grupos de entorno.",
    description:
      "Added to the delete dialog's body when the environment holds env groups, which bex keeps (w1/m683)",
  },
  "environments.deleteFailed": {
    message:
      "No se pudo eliminar el entorno. Inténtalo de nuevo en un momento. ¡Lo sentimos!",
    description:
      "Error in the delete dialog when the delete failed for a reason the server did not name (Render's words, w1/m683)",
  },
  "environments.deleteCardDenied": {
    message: "Eliminación del entorno",
    description:
      "Title of the card that replaces Delete Environment for a member who may not delete the environment (Render's words, w1/m683)",
  },
  "environments.protectedDeleteAdminOnly": {
    message:
      "Solo un administrador puede eliminar un entorno protegido o uno con aislamiento de red o reglas de IP de entrada.",
    description:
      "Why a non-Admin cannot delete an environment that is protected, isolated or has inbound IP rules (w1/m683)",
  },
};

export default esEnvironments;
