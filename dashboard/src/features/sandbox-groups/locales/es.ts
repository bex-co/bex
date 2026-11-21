import type { TranslationEntry } from "@/i18n";

const esSandboxGroups: Record<string, TranslationEntry> = {
  "sandboxGroups.createTitle": {
    message: "Nuevo grupo de sandbox",
    description:
      "Title of the new-sandbox-group dialog and its entry in the + New menus",
  },
  "sandboxGroups.createDescription": {
    message:
      "Un grupo de sandbox limita cuántos de sus sandboxes pueden estar activos a la vez. Los nombres de grupo deben ser únicos dentro de tu espacio de trabajo.",
    description: "New-sandbox-group dialog description",
  },
  "sandboxGroups.fieldName": {
    message: "Nombre",
    description: "New-sandbox-group dialog name field label",
  },
  "sandboxGroups.fieldNamePlaceholder": {
    message: "my-sandbox-group",
    description: "Placeholder for the new sandbox group's name",
  },
  "sandboxGroups.fieldRegion": {
    message: "Región",
    description:
      "Label of the region row in the created sandbox group's confirmation",
  },
  "sandboxGroups.createCancel": {
    message: "Cancelar",
    description: "Cancel button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createSubmit": {
    message: "Crear grupo de sandbox",
    description: "Submit button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createError": {
    message: "No se pudo crear el grupo de sandbox {name}. Inténtalo de nuevo.",
    description: "Toast shown when creating a sandbox group fails",
  },
  "sandboxGroups.errorNameTaken": {
    message:
      "Ya existe un grupo de sandbox llamado “{name}” en este espacio de trabajo. Elige otro nombre.",
    description:
      "Inline error under the name field when the group name is already taken",
  },
  "sandboxGroups.errorNameInvalid": {
    message:
      "El nombre del grupo debe tener de 1 a {max} caracteres, sin espacios al principio ni al final y sin caracteres de control.",
    description: "Inline error under the name field when the name is refused",
  },
  "sandboxGroups.errorLimitReached_one": {
    message:
      "Este espacio de trabajo ya tiene el máximo de {count} grupo de sandbox.",
    description:
      "Inline alert when the workspace is at its sandbox group limit (singular)",
  },
  "sandboxGroups.errorLimitReached_many": {
    message:
      "Este espacio de trabajo ya tiene el máximo de {count} de grupos de sandbox.",
    description:
      "Inline alert when the workspace is at its sandbox group limit",
  },
  "sandboxGroups.errorLimitReached_other": {
    message:
      "Este espacio de trabajo ya tiene el máximo de {count} grupos de sandbox.",
    description:
      "Inline alert when the workspace is at its sandbox group limit",
  },
  "sandboxGroups.errorLimitReachedGeneric": {
    message:
      "Este espacio de trabajo ha alcanzado su límite de grupos de sandbox.",
    description:
      "Inline alert for the sandbox group limit when the server names no number",
  },
  "sandboxGroups.errorLimitTitle": {
    message: "Se alcanzó el límite de grupos de sandbox",
    description: "Heading of the inline sandbox-group-limit alert",
  },
  "sandboxGroups.createdTitle": {
    message: "Grupo de sandbox creado",
    description: "Dialog title once a sandbox group has been created",
  },
  "sandboxGroups.createdDescription": {
    message:
      "{name} está listo. Pasa su ID como sandboxGroupId cuando crees un sandbox en él.",
    description: "Dialog description once a sandbox group has been created",
  },
  "sandboxGroups.createdDone": {
    message: "Listo",
    description: "Button that closes the created-sandbox-group confirmation",
  },
  "sandboxGroups.fieldId": {
    message: "ID",
    description: "Label of the new sandbox group's ID in the confirmation",
  },
  "sandboxGroups.fieldDefault": {
    message: "Grupo predeterminado",
    description:
      "Label of the confirmation row that says whether the group is the workspace's default",
  },
  "sandboxGroups.defaultYes": {
    message: "Sí",
    description: "The created group is the workspace's default sandbox group",
  },
  "sandboxGroups.defaultNo": {
    message: "No",
    description:
      "The created group is not the workspace's default sandbox group",
  },
  "sandboxGroups.fieldConcurrencyLimit": {
    message: "Límite de concurrencia",
    description:
      "Label of the confirmation row with the most active sandboxes the group allows",
  },
  "sandboxGroups.copyId": {
    message: "Copiar ID del grupo de sandbox",
    description: "Accessible label of the copy button next to the group's ID",
  },
  "sandboxGroups.copied": {
    message: "ID del grupo de sandbox copiado",
    description: "Toast after copying the sandbox group ID",
  },
  "sandboxGroups.copyError": {
    message: "No se pudo copiar el ID del grupo de sandbox",
    description: "Toast when copying the sandbox group ID fails",
  },
  "sandboxGroups.environmentLoading": {
    message: "Cargando entornos",
    description:
      "Accessible name of the loading placeholder shown where the new-sandbox-group dialog's Environment select will appear",
  },
  "sandboxGroups.environmentsError": {
    message:
      "No se pudieron cargar tus entornos. Aun así puedes crear el grupo sin entorno.",
    description:
      "Shown in the new-sandbox-group dialog when the workspace's environments could not be read; the group can still be created without one",
  },
  "sandboxGroups.errorEnvironmentGone": {
    message: "Ese entorno ya no existe. Elige otro o ninguno.",
    description:
      "Inline error under the Environment select when the chosen environment was deleted before the group was created",
  },
  "sandboxGroups.pageTitle": {
    message: "Grupos de sandbox",
    description: "Heading for the workspace sandbox groups page",
  },
  "sandboxGroups.pageDescription": {
    message:
      "Administra los límites de concurrencia y las credenciales salientes intermediadas de cada grupo.",
    description: "Subtitle for the workspace sandbox groups page",
  },
  "sandboxGroups.resourceType": {
    message: "Grupo de sandbox",
    description: "Singular resource type name for a sandbox group",
  },
  "sandboxGroups.backToList": {
    message: "Volver a los grupos de sandbox",
    description: "Accessible label of the back link on a sandbox group detail page",
  },
  "sandboxGroups.defaultBadge": {
    message: "Predeterminado",
    description: "Badge indicating the workspace default sandbox group",
  },
  "sandboxGroups.listLoadError": {
    message: "No se pudieron cargar los grupos de sandbox. Inténtalo de nuevo.",
    description: "Error state when loading sandbox groups fails",
  },
  "sandboxGroups.listEmptyTitle": {
    message: "Todavía no hay grupos de sandbox",
    description: "Empty state heading when the workspace has no sandbox groups",
  },
  "sandboxGroups.listEmptyDescription": {
    message:
      "Crea un grupo de sandbox para definir límites de concurrencia y configurar credenciales salientes.",
    description: "Empty state body when the workspace has no sandbox groups",
  },
  "sandboxGroups.manageCredentialsButton": {
    message: "Credenciales",
    description: "Button on each sandbox group row linking to its detail and credentials",
  },
  "sandboxGroups.detailLoadError": {
    message: "No se pudo cargar este grupo de sandbox. Inténtalo de nuevo.",
    description: "Error state when loading a single sandbox group fails",
  },
  "sandboxGroups.credSectionTitle": {
    message: "Credenciales salientes",
    description: "Card title for the sandbox group credentials panel",
  },
  "sandboxGroups.credSectionDescription": {
    message:
      "Secretos intermediados que se inyectan en solicitudes HTTPS salientes coincidentes desde sandboxes de este grupo. Los valores son de solo escritura y nunca se vuelven a mostrar.",
    description: "Card subtitle for the sandbox group credentials panel",
  },
  "sandboxGroups.credAddButton": {
    message: "Agregar credencial",
    description: "Button to open the create sandbox group credential dialog",
  },
  "sandboxGroups.credLoadError": {
    message: "No se pudieron cargar las credenciales del grupo. Inténtalo de nuevo.",
    description: "Error state when fetching sandbox group credentials fails",
  },
  "sandboxGroups.credEmptyTitle": {
    message: "No hay credenciales configuradas",
    description: "Empty state heading when a sandbox group has no credentials",
  },
  "sandboxGroups.credEmptyDescriptionAdmin": {
    message:
      "Agrega una credencial para inyectar autenticación en solicitudes salientes sin exponer el secreto al código del sandbox.",
    description: "Empty state body for admins when no credentials exist",
  },
  "sandboxGroups.credEmptyDescriptionReadOnly": {
    message:
      "Un administrador del espacio de trabajo puede agregar credenciales salientes para este grupo.",
    description: "Empty state body for non-admins when no credentials exist",
  },
  "sandboxGroups.credColId": {
    message: "ID de credencial",
    description: "Table column header for credential ID",
  },
  "sandboxGroups.credColDomain": {
    message: "Dominio",
    description: "Table column header for destination domain",
  },
  "sandboxGroups.credColInjection": {
    message: "Inyección",
    description: "Table column header for injection method and target",
  },
  "sandboxGroups.credColRouting": {
    message: "Enrutamiento",
    description: "Table column header for HTTP methods and path filters",
  },
  "sandboxGroups.credColEnabled": {
    message: "Habilitada",
    description: "Table column header for enabled toggle or status",
  },
  "sandboxGroups.credColActions": {
    message: "Acciones",
    description: "Table column header for row actions",
  },
  "sandboxGroups.credAllMethods": {
    message: "Todos los métodos",
    description: "Badge shown when a credential matches all HTTP methods",
  },
  "sandboxGroups.credAllPaths": {
    message: "Todas las rutas",
    description: "Badge shown when a credential matches all request paths",
  },
  "sandboxGroups.credStatusEnabled": {
    message: "Habilitada",
    description: "Read-only badge when a credential is enabled",
  },
  "sandboxGroups.credStatusDisabled": {
    message: "Deshabilitada",
    description: "Read-only badge when a credential is disabled",
  },
  "sandboxGroups.credToggleEnabledLabel": {
    message: "Cambiar estado de {id}",
    description: "Accessible label for the enabled switch on a credential row",
  },
  "sandboxGroups.credVersionsActionLabel": {
    message: "Ver versiones de {id}",
    description: "Accessible label for the version history icon button",
  },
  "sandboxGroups.credEditActionLabel": {
    message: "Editar credencial {id}",
    description: "Accessible label for the edit credential icon button",
  },
  "sandboxGroups.credDeleteActionLabel": {
    message: "Eliminar credencial {id}",
    description: "Accessible label for the delete credential icon button",
  },
  "sandboxGroups.credVersionsButton": {
    message: "Versiones",
    description: "Mobile button label to open credential version history",
  },
  "sandboxGroups.credEditButton": {
    message: "Editar",
    description: "Mobile button label to edit a credential",
  },
  "sandboxGroups.credDeleteButton": {
    message: "Eliminar",
    description: "Button label to delete a credential",
  },
  "sandboxGroups.credCreateTitle": {
    message: "Agregar credencial",
    description: "Dialog title when creating a sandbox group credential",
  },
  "sandboxGroups.credCreateDescription": {
    message: "Configura una credencial saliente de solo escritura para este grupo de sandbox.",
    description: "Dialog description when creating a sandbox group credential",
  },
  "sandboxGroups.credEditTitle": {
    message: "Editar {id}",
    description: "Dialog title when editing a sandbox group credential",
  },
  "sandboxGroups.credEditDescriptionAdmin": {
    message:
      "Actualiza el enrutamiento, la inyección, el dominio o rota el secreto. Déjalo vacío para conservar el secreto actual.",
    description: "Dialog description when an admin edits a credential",
  },
  "sandboxGroups.credEditDescriptionDeveloper": {
    message:
      "Actualiza la inyección, los métodos, las rutas o el estado habilitado. Cambiar el dominio o el secreto requiere rol de administrador.",
    description: "Dialog description when a developer edits a credential",
  },
  "sandboxGroups.credFieldId": {
    message: "ID de credencial",
    description: "Form label for credential ID",
  },
  "sandboxGroups.credFieldIdHelp": {
    message: "Identificador sensible a mayúsculas con letras, números, ., _, ~ o -. No puede empezar por `rnd-`.",
    description: "Help text for credential ID format",
  },
  "sandboxGroups.credFieldDomain": {
    message: "Dominio de destino",
    description: "Form label for destination hostname",
  },
  "sandboxGroups.credAdminOnlyDomainSecret": {
    message: "Solo los administradores pueden cambiar el dominio o el valor secreto.",
    description: "Helper text shown to developers on read-only domain field",
  },
  "sandboxGroups.credFieldValue": {
    message: "Valor secreto",
    description: "Form label for secret value on create",
  },
  "sandboxGroups.credFieldValuePlaceholder": {
    message: "Pega el valor secreto",
    description: "Placeholder for secret value input on create",
  },
  "sandboxGroups.credFieldReplacementValue": {
    message: "Nuevo valor secreto (opcional)",
    description: "Form label for rotating secret value on edit",
  },
  "sandboxGroups.credFieldReplacementValuePlaceholder": {
    message: "Déjalo vacío para conservar el secreto actual",
    description: "Placeholder for optional secret rotation input on edit",
  },
  "sandboxGroups.credFieldValueWriteOnlyHelp": {
    message: "Solo escritura (hasta 5 KiB). No volverá a mostrarse después de guardar.",
    description: "Help text explaining write-only secret handling",
  },
  "sandboxGroups.credFieldInjectionType": {
    message: "Tipo de inyección",
    description: "Form label for injection type select",
  },
  "sandboxGroups.credInjectionHeader": {
    message: "Encabezado HTTP",
    description: "Injection type option for HTTP header",
  },
  "sandboxGroups.credInjectionBasic": {
    message: "Autenticación básica",
    description: "Injection type option for HTTP Basic authentication",
  },
  "sandboxGroups.credInjectionCookie": {
    message: "Cookie HTTP",
    description: "Injection type option for HTTP cookie",
  },
  "sandboxGroups.credInjectionQuery": {
    message: "Parámetro de consulta",
    description: "Injection type option for URL query parameter",
  },
  "sandboxGroups.credFieldInjectionName": {
    message: "Nombre de destino",
    description: "Form label for header, cookie, or query parameter name",
  },
  "sandboxGroups.credBasicHelp": {
    message: "Usa el encabezado `Authorization` automáticamente. Usa el formato `username:password`.",
    description: "Helper text shown when basic auth injection is selected",
  },
  "sandboxGroups.credFieldTemplate": {
    message: "Plantilla de valor (opcional)",
    description: "Form label for optional value template",
  },
  "sandboxGroups.credFieldTemplateHelp": {
    message: "Por defecto es {token}. Incluye entre 1 y 5 apariciones de {token}.",
    description: "Help text for injection template placeholder syntax",
  },
  "sandboxGroups.credFieldMethods": {
    message: "Métodos HTTP",
    description: "Form label for allowed HTTP methods multi-select",
  },
  "sandboxGroups.credFieldMethodsHelp": {
    message: "Deja todos sin seleccionar para coincidir con cualquier método HTTP.",
    description: "Help text for HTTP methods selection",
  },
  "sandboxGroups.credFieldPaths": {
    message: "Patrones de ruta (uno por línea)",
    description: "Form label for path patterns textarea",
  },
  "sandboxGroups.credFieldPathsHelp": {
    message: "Rutas absolutas que empiezan por /. Admite comodines * y ** de segmento completo. Déjalo vacío para todas las rutas.",
    description: "Help text for path pattern syntax",
  },
  "sandboxGroups.credFieldEnabled": {
    message: "Habilitada",
    description: "Form label for credential enabled switch",
  },
  "sandboxGroups.credFieldEnabledHelp": {
    message: "Las credenciales deshabilitadas permanecen en el grupo pero nunca se inyectan.",
    description: "Help text for credential enabled switch",
  },
  "sandboxGroups.credCreateSubmit": {
    message: "Crear credencial",
    description: "Submit button label for creating a credential",
  },
  "sandboxGroups.credEditSubmit": {
    message: "Guardar cambios",
    description: "Submit button label for editing a credential",
  },
  "sandboxGroups.credCreatedToast": {
    message: "Credencial {id} creada.",
    description: "Success toast after creating a credential",
  },
  "sandboxGroups.credUpdatedToast": {
    message: "Credencial {id} actualizada.",
    description: "Success toast after updating a credential",
  },
  "sandboxGroups.credDeletedToast": {
    message: "Credencial {id} eliminada.",
    description: "Success toast after deleting a credential",
  },
  "sandboxGroups.credRestoredToast": {
    message: "Se restauró {id} desde la versión v{version}.",
    description: "Success toast after restoring a credential version",
  },
  "sandboxGroups.credCreateError": {
    message: "No se pudo crear la credencial. Revisa tus datos.",
    description: "Fallback error when creating a credential fails",
  },
  "sandboxGroups.credUpdateError": {
    message: "No se pudo actualizar la credencial. Inténtalo de nuevo.",
    description: "Fallback error when updating a credential fails",
  },
  "sandboxGroups.credDeleteError": {
    message: "No se pudo eliminar la credencial. Inténtalo de nuevo.",
    description: "Fallback error when deleting a credential fails",
  },
  "sandboxGroups.credRestoreError": {
    message: "No se pudo restaurar la versión de la credencial. Inténtalo de nuevo.",
    description: "Fallback error when restoring a credential version fails",
  },
  "sandboxGroups.credErrorExists": {
    message: 'Ya existe una credencial con el ID "{id}" en este grupo.',
    description: "Inline error when creating a duplicate credential ID",
  },
  "sandboxGroups.credErrorBusy": {
    message: "Hay otra actualización de credenciales en curso en este grupo. Reintenta en unos segundos.",
    description: "Error when concurrent credential writes contend on the group lock",
  },
  "sandboxGroups.credErrorUnavailable": {
    message: "El almacenamiento de credenciales no está disponible en este momento.",
    description: "Error when OpenBao credential storage is unconfigured or unreachable",
  },
  "sandboxGroups.credErrorVersionNotFound": {
    message: "Esa versión de la credencial ya no se conserva.",
    description: "Error when restoring an evicted or unknown version",
  },
  "sandboxGroups.credVersionsTitle": {
    message: "Historial de versiones · {id}",
    description: "Dialog title for credential version history",
  },
  "sandboxGroups.credVersionsDescription": {
    message: "Conserva la configuración actual más hasta 3 configuraciones anteriores distintas.",
    description: "Dialog subtitle explaining version retention policy",
  },
  "sandboxGroups.credVersionsError": {
    message: "No se pudo cargar el historial de versiones. Inténtalo de nuevo.",
    description: "Error state when loading credential versions fails",
  },
  "sandboxGroups.credVersionsEmpty": {
    message: "No se encontraron versiones conservadas.",
    description: "Empty state for credential versions dialog",
  },
  "sandboxGroups.credVersionCurrent": {
    message: "Actual",
    description: "Badge marking the active credential version",
  },
  "sandboxGroups.credVersionRestoredFrom": {
    message: "Restaurada desde v{version}",
    description: "Badge showing which prior version was restored",
  },
  "sandboxGroups.credFingerprintLabel": {
    message: "Huella digital",
    description: "Label before the truncated SHA-256 fingerprint",
  },
  "sandboxGroups.credUpdatedBy": {
    message: "por {actor}",
    description: "Actor attribution on a credential version row",
  },
  "sandboxGroups.credRestoreAction": {
    message: "Restaurar",
    description: "Button label to restore a prior credential version",
  },
  "sandboxGroups.credRestoreConfirmTitle": {
    message: "¿Restaurar {id} a v{version}?",
    description: "Confirmation dialog title when restoring a credential version",
  },
  "sandboxGroups.credRestoreConfirmDescription": {
    message: "Esto crea una nueva versión actual para {id} con la configuración y el secreto de v{version}.",
    description: "Confirmation dialog body when restoring a credential version",
  },
  "sandboxGroups.credDeleteConfirmTitle": {
    message: "¿Eliminar la credencial {id}?",
    description: "Confirmation dialog title when deleting a credential",
  },
  "sandboxGroups.credDeleteConfirmDescription": {
    message: "Eliminar {id} purga permanentemente su secreto almacenado y todas las versiones conservadas.",
    description: "Confirmation dialog body when deleting a credential",
  },
  "sandboxGroups.credValIdRequired": {
    message: "El ID de credencial es obligatorio.",
    description: "Validation error when credential ID is empty",
  },
  "sandboxGroups.credValIdInvalid": {
    message: "Usa solo letras, números, puntos, guiones bajos, virgulillas y guiones.",
    description: "Validation error when credential ID has disallowed characters",
  },
  "sandboxGroups.credValIdReserved": {
    message: 'Los IDs ".", ".." y los que empiezan por `rnd-` están reservados.',
    description: "Validation error when credential ID is reserved",
  },
  "sandboxGroups.credValDomainRequired": {
    message: "El dominio de destino es obligatorio.",
    description: "Validation error when domain is empty",
  },
  "sandboxGroups.credValDomainInvalid": {
    message: "Ingresa un nombre DNS público válido sin comodines, puertos ni rutas.",
    description: "Validation error when domain format is invalid",
  },
  "sandboxGroups.credValDomainPrivate": {
    message: "No se permiten nombres de host locales, privados ni del clúster.",
    description: "Validation error when domain is private or cluster-local",
  },
  "sandboxGroups.credValValueRequired": {
    message: "El valor secreto es obligatorio.",
    description: "Validation error when secret value is empty on create",
  },
  "sandboxGroups.credValValueTooLong": {
    message: "El valor secreto debe tener como máximo {maxBytes} bytes.",
    description: "Validation error when secret value exceeds 5 KiB",
  },
  "sandboxGroups.credValValueNul": {
    message: "El valor secreto no puede contener bytes NUL.",
    description: "Validation error when secret value contains a NUL byte",
  },
  "sandboxGroups.credValInjectionNameRequired": {
    message: "El nombre de destino es obligatorio para este tipo de inyección.",
    description: "Validation error when header/cookie/query name is missing",
  },
  "sandboxGroups.credValInjectionNameForbidden": {
    message: "La autenticación básica no puede especificar un nombre de destino.",
    description: "Validation error when basic injection specifies a name",
  },
  "sandboxGroups.credValInjectionHeaderForbidden": {
    message: "No se pueden inyectar encabezados de enrutamiento ni conexión.",
    description: "Validation error when a forbidden HTTP header is chosen",
  },
  "sandboxGroups.credValInjectionNameInvalid": {
    message: "Ingresa un nombre de destino válido.",
    description: "Validation error when target name has invalid characters",
  },
  "sandboxGroups.credValInjectionTemplateCount": {
    message: "La plantilla debe incluir entre 1 y {max} apariciones de {token}.",
    description: "Validation error when template placeholder count is out of range",
  },
  "sandboxGroups.credValInjectionBasicFormat": {
    message: "El valor renderizado de autenticación básica debe tener la forma `username:password`.",
    description: "Validation error when basic auth rendered value lacks username:password",
  },
  "sandboxGroups.credValInjectionHeaderControl": {
    message: "El valor renderizado del encabezado no puede contener saltos de línea ni bytes NUL.",
    description: "Validation error when header value contains CR/LF/NUL",
  },
  "sandboxGroups.credValInjectionCookieInvalid": {
    message: "El valor renderizado de la cookie no puede contener punto y coma ni saltos de línea.",
    description: "Validation error when cookie value contains semicolons or CR/LF",
  },
  "sandboxGroups.credValPathsInvalid": {
    message: "Cada ruta debe empezar con /, omitir ? o # y usar comodines * o ** de segmento completo.",
    description: "Validation error when a path pattern is invalid",
  },
};

export default esSandboxGroups;
