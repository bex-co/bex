import type { TranslationEntry } from "@/i18n";

const esDatabases: Record<string, TranslationEntry> = {
  "databases.resourceType": {
    message: "Base de datos",
    description: "Managed Postgres resource type used in document titles",
  },
  "databases.statTotal": {
    message: "Total de bases de datos",
    description: "Databases page stat card label",
  },
  "databases.statAvailable": {
    message: "Disponibles",
    description: "Databases page stat card label (healthy databases)",
  },
  "databases.statCreating": {
    message: "Creándose",
    description: "Databases page stat card label (provisioning databases)",
  },
  "databases.cardTitle": {
    message: "Bases de datos",
    description: "Databases table card title",
  },
  "databases.colName": {
    message: "Nombre",
    description: "Databases table column header",
  },
  "databases.colStatus": {
    message: "Estado",
    description: "Databases table column header",
  },
  "databases.colPlan": {
    message: "Plan",
    description: "Databases table column header (compute plan / tier)",
  },
  "databases.colVersion": {
    message: "Versión",
    description: "Databases table column header (PostgreSQL major version)",
  },
  "databases.colStorage": {
    message: "Almacenamiento",
    description: "Databases table column header (disk size)",
  },
  "databases.colCreated": {
    message: "Creada",
    description: "Databases table column header (relative age from createdAt)",
  },
  "databases.colActions": {
    message: "Acciones",
    description: "Databases table actions column header (screen-reader only)",
  },
  "databases.statusAvailable": {
    message: "Disponible",
    description: "Database status badge (CNPG cluster healthy)",
  },
  "databases.statusCreating": {
    message: "Creándose",
    description: "Database status badge (provisioning)",
  },
  "databases.statusRestarting": {
    message: "Reiniciándose",
    description:
      "Database status badge (Render's config_restart: an established store restarting after a config change, restart, or resume — not a first provision)",
  },
  "databases.statusUpgrading": {
    message: "Actualizándose",
    description: "Database status badge (offline PostgreSQL major upgrade)",
  },
  "databases.statusUnavailable": {
    message: "No disponible",
    description: "Database status badge (provisioning failed)",
  },
  "databases.statusSuspended": {
    message: "Suspendida",
    description:
      "Database status badge (hibernated; suspension wins over the status enum)",
  },
  "databases.statusUnknown": {
    message: "Desconocido",
    description: "Database status badge for an unrecognized status",
  },
  "databases.errorTitle": {
    message: "No se pudieron cargar las bases de datos",
    description: "Databases list error card title",
  },
  "databases.errorBody": {
    message:
      "La solicitud a bex-api falló. Comprueba tu conexión y vuelve a intentarlo.",
    description: "Databases list error card body",
  },
  "databases.emptyTitle": {
    message: "Aún no hay bases de datos",
    description: "Databases list empty state title",
  },
  "databases.emptyBody": {
    message: "Crea tu primer Postgres administrado y aparecerá aquí.",
    description: "Databases list empty state body",
  },
  "databases.actionsMenu": {
    message: "Acciones para {name}",
    description: "Accessible label for the per-row actions trigger",
  },
  "databases.actionDelete": {
    message: "Eliminar",
    description: "Row action: permanently delete the database",
  },
  "databases.deleteConfirmTitle": {
    message: "¿Eliminar {name}?",
    description: "Delete-confirmation dialog title",
  },
  "databases.deleteConfirmBody": {
    message:
      "Esta acción no se puede deshacer. ¿Seguro que quieres eliminar esta instancia de Postgres?",
    description: "Delete-confirmation dialog body (Render-style copy)",
  },
  "databases.deleteConfirmPrompt": {
    message: "Escribe {phrase} a continuación para confirmar.",
    description:
      "Body prompt naming the exact 'sudo delete postgres <name>' phrase (rendered bold by SudoCommandField)",
  },
  "databases.deleteCancel": {
    message: "Cancelar",
    description: "Delete-confirmation dialog cancel button",
  },
  "databases.deleteConfirm": {
    message: "Eliminar base de datos",
    description: "Delete-confirmation dialog confirm button",
  },
  "databases.deleteSuccess": {
    message: "Eliminando {name}…",
    description: "Toast after a delete request is accepted",
  },
  "databases.deleteError": {
    message: "No se pudo eliminar {name}. Inténtalo de nuevo.",
    description: "Toast when a delete request fails",
  },
  "databases.createTitle": {
    message: "Nuevo Postgres",
    description: "Create-database dialog title",
  },
  "databases.createDescription": {
    message: "Aprovisiona una instancia administrada de PostgreSQL.",
    description: "Create-database dialog description",
  },
  "databases.fieldName": {
    message: "Nombre",
    description: "Create-database form field label (database name)",
  },
  "databases.fieldNamePlaceholder": {
    message: "my-database",
    description: "Create-database name input placeholder",
  },
  "databases.fieldNameError": {
    message:
      "Introduce un nombre de hasta 100 caracteres, sin saltos de línea ni caracteres de control.",
    description: "Create-database name validation message",
  },
  "databases.fieldDatabaseName": {
    message: "Base de datos",
    description: "Create-database physical PostgreSQL database label",
  },
  "databases.fieldDatabaseNamePlaceholder": {
    message: "orders_data",
    description: "Create-database physical PostgreSQL database placeholder",
  },
  "databases.fieldDatabaseUser": {
    message: "Usuario",
    description: "Create-database physical PostgreSQL owner-role label",
  },
  "databases.fieldDatabaseUserPlaceholder": {
    message: "orders_owner",
    description: "Create-database physical PostgreSQL owner-role placeholder",
  },
  "databases.fieldPhysicalNameReserved": {
    message: "“{name}” está reservado por PostgreSQL. Elige otro nombre.",
    description:
      "Create-database / add-user validation for a name the managed PostgreSQL service reserves (postgres, template0/1, streaming_replica, public, none, pg_*, cnpg_*)",
  },
  "databases.fieldPhysicalNameError": {
    message:
      "Usa hasta 63 letras minúsculas, dígitos y guiones bajos; empieza por una letra o un guion bajo.",
    description: "Create-database physical PostgreSQL identifier validation",
  },
  "databases.fieldPlan": {
    message: "Plan de cómputo",
    description: "Create-database form field label (plan / tier)",
  },
  "databases.fieldVersion": {
    message: "Versión de PostgreSQL",
    description: "Create-database form field label (major version)",
  },
  "databases.fieldVersionDefault": {
    message: "Predeterminada (la más reciente)",
    description: "Create-database version select default option",
  },
  "databases.fieldDisk": {
    message: "Almacenamiento",
    description: "Create-database form field label (storage size)",
  },
  "databases.fieldDiskFixed": {
    message:
      "Este plan incluye un disco fijo de {size} GB. Elige un plan mayor para definir el almacenamiento tú mismo.",
    description:
      "Create-database hint under a plan's fixed disk size, e.g. Free (w4/376)",
  },
  "databases.fieldDiskError": {
    message: "Introduce un número entero entre {min} y {max} GB.",
    description:
      "Create-database validation message when the disk size is below the plan floor or above the volume cap",
  },
  "databases.fieldPublic": {
    message: "Acceso público",
    description: "Create-database form field label (external endpoint toggle)",
  },
  "databases.fieldPublicHint": {
    message:
      "Acepta conexiones con credenciales válidas desde cualquier dirección IP hasta que las restrinjas en Control de acceso.",
    description:
      "Create-database public toggle helper text; the toggle starts on, as a Render database is reachable by default (w4/553)",
  },
  "databases.fieldPublicHintOff": {
    message:
      "Las conexiones externas están bloqueadas. Los servicios de la red privada aún pueden conectarse.",
    description:
      "Create-database public toggle helper text once switched off: the database is created private (w4/553)",
  },
  "databases.fieldHighAvailability": {
    message: "Alta disponibilidad",
    description: "Create-database high availability toggle label (w2/m171)",
  },
  "databases.fieldHighAvailabilityHint": {
    message:
      "Ejecuta una réplica en espera en otro nodo que toma el relevo si el primario falla. La réplica en espera se factura igual que el primario.",
    description:
      "Create-database HA toggle helper text on a plan that offers HA",
  },
  "databases.fieldHighAvailabilityPlanUnsupported": {
    message:
      "{name} no ofrece alta disponibilidad. Elige un plan con al menos 1 CPU.",
    description:
      "Create-database HA toggle helper text when the selected plan is below 1 CPU (Render's rule)",
  },
  "databases.createCancel": {
    message: "Cancelar",
    description: "Create-database dialog cancel button",
  },
  "databases.createSubmit": {
    message: "Crear base de datos",
    description: "Create-database dialog submit button",
  },
  "databases.createSuccess": {
    message: "Creando {name}…",
    description:
      "Toast after a create request is accepted (provisioning is async)",
  },
  "databases.createError": {
    message: "No se pudo crear {name}. Inténtalo de nuevo.",
    description: "Toast when a create request fails",
  },
  "databases.capLimitTitle": {
    message: "Se alcanzó el límite de Postgres",
    description:
      "Alert title when the workspace's Postgres creation cap is hit (w7/m9)",
  },
  "databases.capLimitUpgrade": {
    message: "Mejorar plan",
    description:
      "Upgrade CTA button inside the Postgres cap-limit Alert (w7/m9)",
  },
  "databases.metaTitle": {
    message: "Detalles",
    description: "Database detail metadata card title",
  },
  "databases.statusReasonClusterFailed": {
    message:
      "El clúster de la base de datos rechazó su configuración, por lo que no se pudo aplicar el último cambio.",
    description:
      "Why an unavailable database is unavailable: the operator's ClusterFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonClusterReadFailed": {
    message: "No se pudo leer el estado del clúster de la base de datos.",
    description:
      "Why an unavailable database is unavailable: the operator's ClusterReadFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonNetworkPolicyFailed": {
    message: "No se pudo aplicar la política de red de la base de datos.",
    description:
      "Why an unavailable database is unavailable: the operator's NetworkPolicyFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonDiskAutoscalingFailed": {
    message: "No se pudo aplicar el escalado automático del disco.",
    description:
      "Why an unavailable database is unavailable: the operator's DiskAutoscalingFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonExportFailed": {
    message: "No se pudo iniciar una exportación de la base de datos.",
    description:
      "Why an unavailable database is unavailable: the operator's ExportFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonPoolerFailed": {
    message: "No se pudo aprovisionar el pooler de conexiones.",
    description:
      "Why an unavailable database is unavailable: the operator's PoolerFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonExtensionsFailed": {
    message:
      "No se pudieron instalar las extensiones administradas de la base de datos, por lo que aún no está disponible.",
    description:
      "Why an unavailable database is unavailable: the operator's ExtensionsFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonPostUpgradeBackupFailed": {
    message:
      "No se pudo realizar la copia de seguridad posterior a la actualización de versión.",
    description:
      "Why an unavailable database is unavailable: the operator's PostUpgradeBackupFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonMajorVersionUpgradeFailed": {
    message:
      "La actualización de versión mayor falló; la base de datos permanece en su versión actual.",
    description:
      "Why an unavailable database is unavailable: the operator's MajorVersionUpgradeFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonStorageShrinkRejected": {
    message:
      "El almacenamiento de Postgres solo crece: el disco no puede bajar de los {size} GB asignados.",
    description:
      "Why an unavailable database is unavailable: the operator's StorageShrinkRejected Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonRecoveryUnavailable": {
    message:
      "No se puede restaurar la base de datos: no hay ningún almacén de copias de seguridad configurado.",
    description:
      "Why an unavailable database is unavailable: the operator's RecoveryUnavailable Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonBackupStoreUnavailable": {
    message:
      "No se pudo configurar el almacenamiento de copias de seguridad de la base de datos.",
    description:
      "Why an unavailable database is unavailable: the operator's BackupStoreUnavailable Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonScheduledBackupFailed": {
    message: "No se pudo programar la copia de seguridad diaria.",
    description:
      "Why an unavailable database is unavailable: the operator's ScheduledBackupFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonScheduledBackupCleanupFailed": {
    message:
      "No se pudo eliminar la programación de la copia de seguridad diaria.",
    description:
      "Why an unavailable database is unavailable: the operator's ScheduledBackupCleanupFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.metaStatus": {
    message: "Estado",
    description: "Database detail metadata row label",
  },
  "databases.metaPlan": {
    message: "Plan de cómputo",
    description: "Database detail metadata row label",
  },
  "databases.metaVersion": {
    message: "Versión",
    description: "Database detail metadata row label",
  },
  "databases.versionLabel": {
    message: "PostgreSQL {version}",
    description: "Formatted PostgreSQL major version label",
  },
  "databases.versionUpgradeAction": {
    message: "Actualizar",
    description: "Button opening the PostgreSQL major-version upgrade dialog",
  },
  "databases.versionUpgrading": {
    message: "Actualización en curso",
    description:
      "Version-row status while the database is offline for pg_upgrade",
  },
  "databases.versionUpgradeTitle": {
    message: "Actualizar PostgreSQL",
    description: "PostgreSQL major-version upgrade dialog title",
  },
  "databases.versionUpgradeDescription": {
    message:
      "Actualiza esta base de datos de PostgreSQL {version} a una versión mayor más reciente compatible.",
    description: "PostgreSQL major-version upgrade dialog description",
  },
  "databases.versionUpgradeTarget": {
    message: "Versión de destino",
    description: "PostgreSQL major-version upgrade target selector label",
  },
  "databases.versionUpgradeDowntime": {
    message:
      "La base de datos no estará disponible mientras se ejecuta la actualización sin conexión. Prueba antes la compatibilidad y programa el tiempo de inactividad; las bases de datos grandes pueden tardar hasta una hora.",
    description:
      "Downtime warning in the PostgreSQL major-version upgrade dialog",
  },
  "databases.versionUpgradeErrorTitle": {
    message: "Actualización bloqueada",
    description:
      "Inline backend-guard error title in the version upgrade dialog",
  },
  "databases.versionUpgradeCancel": {
    message: "Cancelar",
    description: "PostgreSQL major-version upgrade dialog cancel button",
  },
  "databases.versionUpgradeConfirm": {
    message: "Actualizar a PostgreSQL {version}",
    description: "PostgreSQL major-version upgrade dialog confirmation button",
  },
  "databases.versionUpgradeAccepted": {
    message: "Se inició la actualización a PostgreSQL {version}.",
    description: "Toast after a PostgreSQL major-version upgrade is accepted",
  },
  "databases.versionUpgradeError": {
    message:
      "No se pudo iniciar la actualización de PostgreSQL. Inténtalo de nuevo.",
    description: "Fallback error when the PostgreSQL upgrade mutation fails",
  },
  "databases.metaDatabaseName": {
    message: "Base de datos",
    description: "Database detail metadata row label (the normalized db name)",
  },
  "databases.metaDatabaseUser": {
    message: "Usuario",
    description: "Database detail metadata row label (owner role)",
  },
  "databases.metaStorage": {
    message: "Almacenamiento",
    description: "Database detail metadata row label (disk size)",
  },
  "databases.diskAutoscalingLabel": {
    message: "Escalado automático del disco",
    description: "Label beside the database disk-usage chart toggle",
  },
  "databases.diskAutoscalingSize": {
    message: "{current} GB actuales · {cap} GB máx.",
    description: "Current and maximum Postgres disk size beside autoscaling",
  },
  "databases.diskAutoscalingHint": {
    message:
      "Cuando el disco está lleno al 90 %, el almacenamiento crece un 50 %, redondeado hacia arriba a múltiplos de 5 GB. Los aumentos son permanentes y se limitan a uno cada 12 horas.",
    description: "Accessible explanation of Postgres disk autoscaling",
  },
  "databases.diskAutoscalingCurrentSize": {
    message: "{current} GB actuales",
    description: "Current disk size while plan limits are unavailable",
  },
  "databases.diskAutoscalingPlanUnsupported": {
    message:
      "Este plan no admite el escalado automático del disco. Una configuración existente se puede desactivar.",
    description: "Plan restriction beside the disk autoscaling switch",
  },
  "databases.diskAutoscalingEnabled": {
    message: "Escalado automático del disco activado.",
    description: "Toast after enabling Postgres disk autoscaling",
  },
  "databases.diskAutoscalingDisabled": {
    message: "Escalado automático del disco desactivado.",
    description: "Toast after disabling Postgres disk autoscaling",
  },
  "databases.diskAutoscalingError": {
    message:
      "No se pudo actualizar el escalado automático del disco. Inténtalo de nuevo.",
    description: "Toast when the disk-autoscaling mutation fails",
  },
  "databases.metaHighAvailability": {
    message: "Alta disponibilidad",
    description:
      "Database detail metadata row label (HA — single-instance MVP is No)",
  },
  "databases.metaPublic": {
    message: "Acceso público",
    description: "Database detail metadata row label (external endpoint)",
  },
  "databases.metaExternalHost": {
    message: "Host externo",
    description: "Database detail metadata row label (SNI hostname)",
  },
  "databases.metaRegion": {
    message: "Región",
    description: "Database detail metadata row label (platform placement)",
  },
  "databases.metaCreated": {
    message: "Creada",
    description: "Database detail metadata row label (relative age)",
  },
  "databases.metaExpires": {
    message: "Caduca",
    description:
      "Details row: when a free database expires (Render's expiresAt, w8/m126)",
  },
  "databases.expirySoon": {
    message:
      "Esta base de datos gratuita caduca el {expiresAt} y después dejará de funcionar.",
    description: "Warning within 7 days of a free database's expiry",
  },
  "databases.expiryExpired": {
    message:
      "Esta base de datos gratuita ha caducado y está suspendida. Ella y sus datos se eliminarán el {deleteAt}.",
    description:
      "Warning on an expired free database during its 14-day grace period",
  },
  "databases.expiryUpgrade": {
    message: "Cambiar a un plan de pago",
    description: "Link from the expiry warning to the Plan section",
  },
  "databases.expiryHintSoon": {
    message: "Caduca pronto",
    description: "List-row hint on a free database within 7 days of expiry",
  },
  "databases.expiryHintExpired": {
    message: "Caducada",
    description: "List-row hint on an expired free database",
  },
  "databases.statusReasonFreeExpired": {
    message:
      "Esta base de datos gratuita ha caducado. Cámbiala a un plan de pago para restaurarla.",
    description:
      "Why a suspended database is suspended: its free plan expired (bex-api statusReasonCode POSTGRES_FREE_EXPIRED, w8/m126)",
  },
  "databases.yes": {
    message: "Sí",
    description: "Metadata value for a true boolean field",
  },
  "databases.no": {
    message: "No",
    description: "Metadata value for a false boolean field",
  },
  "databases.connTitle": {
    message: "Conexiones",
    description: "Connection-info panel card title",
  },
  "databases.connDescription": {
    message:
      "Cadenas de conexión y contraseña de la base de datos. Solo se muestran cuando lo pides, nunca de forma automática.",
    description: "Connection-info panel card description",
  },
  "databases.connReveal": {
    message: "Mostrar información de conexión",
    description: "Button that fetches the connection info on demand",
  },
  "databases.connHide": {
    message: "Ocultar información de conexión",
    description: "Button that clears the revealed connection info",
  },
  "databases.connLogicalReplicationTitle": {
    message: "Replicación lógica",
    description:
      "Heading of the logical-replication note on the Connections card",
  },
  "databases.connLogicalReplicationBody": {
    message:
      "Desactivada de forma predeterminada. En un workspace Pro o superior, una base de datos con al menos 10 GB de almacenamiento puede publicar sus cambios a suscriptores dentro o fuera de bex. Para activarla, contacta con soporte indicando el ID de esta base de datos, la credencial con la que se conectan los suscriptores, los esquemas que leen y si se publican todas las tablas. Después crea publicaciones con CREATE PUBLICATION … FOR TABLE como usuario de esta base de datos y conecta los suscriptores a la URL externa, nunca a la del pooler. También puede suscribirse a otra base de datos dentro o fuera de bex: solicítalo de la misma forma indicando la credencial que crea las suscripciones y ejecuta CREATE SUBSCRIPTION cuando existan las tablas (en PostgreSQL 15 y anteriores, soporte crea la suscripción por ti).",
    description:
      "Logical-replication note on the Connections card: off by default, enabled on request (Render's rules)",
  },
  "databases.connPassword": {
    message: "Contraseña",
    description: "Connection-info field label (database password)",
  },
  "databases.connShowPassword": {
    message: "Mostrar contraseña",
    description: "Accessible label to unmask the password",
  },
  "databases.connHidePassword": {
    message: "Ocultar contraseña",
    description: "Accessible label to re-mask the password",
  },
  "databases.connInternal": {
    message: "Cadena de conexión interna",
    description: "Connection-info field label (in-cluster URL)",
  },
  "databases.connExternal": {
    message: "Cadena de conexión externa",
    description: "Connection-info field label (public URL, TLS)",
  },
  "databases.connPsql": {
    message: "Comando psql",
    description: "Connection-info field label (ready-to-run psql command)",
  },
  "databases.connCaLabel": {
    message: "Certificado CA del servidor",
    description: "Connection-info field label (private server CA bundle)",
  },
  "databases.connCaDownload": {
    message: "Descargar certificado CA",
    description: "Button downloading the server CA PEM file",
  },
  "databases.connCaHelp": {
    message:
      "Las conexiones externas verifican el certificado de esta base de datos con su CA de servidor privada (sslmode=verify-full sigue siendo obligatorio). Descarga {file} y apunta tu cliente al archivo guardado; para el comando psql o la cadena de conexión externa, añade este prefijo:",
    description:
      "Explanation above the PGSSLROOTCERT example; {file} is the downloaded CA filename",
  },
  "databases.connCaSnippet": {
    message: "Ejemplo de prefijo de entorno que apunta a la CA descargada",
    description:
      "Accessible label for the copyable PGSSLROOTCERT example snippet",
  },
  "databases.connCertPendingTitle": {
    message: "Se está emitiendo el certificado de confianza pública",
    description:
      "Connections card note title while a public database's Let's Encrypt certificate is still pending (w4/m357)",
  },
  "databases.connCertPendingBody": {
    message:
      "Se está emitiendo un certificado de confianza pública (suele tardar unos minutos). Hasta entonces, conéctate con la CA de abajo o con sslmode=require. Cuando se emita, la URL externa no necesitará CA, como en Render; oculta y vuelve a mostrar para actualizar.",
    description:
      "Connections card note body while the public database certificate is pending",
  },
  "databases.connCertFailedTitle": {
    message: "Aún no se emitió el certificado de confianza pública",
    description:
      "Connections card note title when issuing a public database's certificate failed (w4/m357)",
  },
  "databases.connCertFailedBody": {
    message:
      "Falló la emisión del certificado de confianza pública de esta base de datos, y bex lo reintenta automáticamente. Mientras tanto, la CA funciona: conéctate con la CA de abajo o con sslmode=require.",
    description:
      "Connections card note body when the public database certificate failed to issue",
  },
  "databases.connErrorTitle": {
    message: "No se pudo cargar la información de conexión",
    description: "Connection-info panel error title",
  },
  "databases.connErrorBody": {
    message:
      "Puede que la base de datos aún se esté aprovisionando o que no tengas permiso para ver sus credenciales.",
    description: "Connection-info panel error body",
  },
  "databases.copied": {
    message: "Copiado al portapapeles",
    description: "Toast after copying a connection field",
  },
  "databases.copyError": {
    message: "No se pudo copiar al portapapeles",
    description: "Toast when clipboard copy fails",
  },
  "databases.loading": {
    message: "Cargando…",
    description: "Generic loading placeholder in a detail panel",
  },
  "databases.dangerDelete": {
    message: "Eliminar base de datos",
    description: "Detail-page bottom action: permanently delete the database",
  },
  "databases.dangerRestart": {
    message: "Reiniciar base de datos",
    description: "Detail-page bottom action: rolling restart of the primary",
  },
  "databases.dangerSuspend": {
    message: "Suspender base de datos",
    description:
      "Detail-page bottom action: hibernate the database (stop compute, keep data)",
  },
  "databases.dangerResume": {
    message: "Reanudar base de datos",
    description: "Detail-page bottom action: wake a suspended database",
  },
  "databases.actionSuspend": {
    message: "Suspender",
    description: "Row action: hibernate the database (stop compute, keep data)",
  },
  "databases.actionResume": {
    message: "Reanudar",
    description: "Row action: wake a suspended database",
  },
  "databases.actionRestart": {
    message: "Reiniciar",
    description: "Row action: rolling restart of the primary",
  },
  "databases.suspendConfirmTitle": {
    message: "¿Suspender {name}?",
    description: "Suspend-confirmation dialog title",
  },
  "databases.suspendConfirmBody": {
    message:
      "El cómputo se detiene y la base de datos deja de ser accesible, pero sus datos se conservan. Reanúdala cuando quieras para recuperarla.",
    description: "Suspend-confirmation dialog body",
  },
  "databases.restartConfirmTitle": {
    message: "¿Reiniciar {name}?",
    description: "Restart-confirmation dialog title",
  },
  "databases.restartConfirmBody": {
    message:
      "Se reinicia el primario. Las conexiones se interrumpen brevemente mientras vuelve a estar disponible.",
    description: "Restart-confirmation dialog body",
  },
  "databases.toastSuspendSuccess": {
    message: "Suspendiendo {name}…",
    description: "Toast after a suspend request is accepted",
  },
  "databases.toastResumeSuccess": {
    message: "Reanudando {name}…",
    description: "Toast after a resume request is accepted",
  },
  "databases.toastRestartSuccess": {
    message: "Reiniciando {name}…",
    description: "Toast after a restart request is accepted",
  },
  "databases.toastLifecycleError": {
    message: "No se pudo completar la acción en {name}. Inténtalo de nuevo.",
    description: "Toast when a lifecycle verb fails",
  },
  "databases.recoveryTitle": {
    message: "Recuperación",
    description: "Recovery panel card title",
  },
  "databases.recoveryDescription": {
    message:
      "Recuperación a un punto en el tiempo y copias de seguridad. Restaurar siempre crea una base de datos nueva; esta nunca se modifica.",
    description: "Recovery panel card description",
  },
  "databases.recoveryDisabled": {
    message:
      "Las copias de seguridad no están habilitadas para este plan, por lo que la recuperación no está disponible. Cambia a un plan con copias de seguridad para habilitar la recuperación a un punto en el tiempo.",
    description: "Recovery panel state when the plan has no backups",
  },
  "databases.recoveryUnavailable": {
    message:
      "No se pudo leer la información de recuperación. Inténtalo de nuevo.",
    description: "Recovery panel state when backup metadata could not be read",
  },
  "databases.recoveryEarliest": {
    message: "Punto de restauración más antiguo",
    description: "Recovery panel field (earliest recoverable time)",
  },
  "databases.recoveryLatest": {
    message: "Punto de restauración más reciente",
    description: "Recovery panel field (latest recoverable time)",
  },
  "databases.recoveryWindowNotEstablished": {
    message: "Aún no establecido",
    description: "Recovery panel value before the PITR window is established",
  },
  "databases.recoveryRestore": {
    message: "Restaurar en una instancia nueva",
    description: "Recovery panel button that opens the restore dialog",
  },
  "databases.recoveryRestoreUnavailable": {
    message:
      "La restauración estará disponible cuando se establezca el primer punto de recuperabilidad.",
    description: "Reason restore is disabled before the PITR window opens",
  },
  "databases.recoveryCreateExport": {
    message: "Crear exportación",
    description: "Recovery panel button that triggers an on-demand export",
  },
  "databases.recoveryBackups": {
    message: "Copias de seguridad",
    description: "Recovery panel backup-list heading",
  },
  "databases.recoveryNoBackups": {
    message: "Aún no hay copias de seguridad.",
    description: "Recovery panel empty backup list",
  },
  "databases.recoveryExportScope": {
    message:
      "Una exportación contiene solo la base de datos {database}. Las demás bases de datos creadas en esta instancia no se incluyen.",
    description:
      "Recovery panel note naming the one database a logical export contains (w8/258)",
  },
  "databases.recoveryExports": {
    message: "Exportaciones",
    description: "Recovery panel export-list heading",
  },
  "databases.recoveryNoExports": {
    message: "Aún no hay exportaciones.",
    description: "Recovery panel empty export list",
  },
  "databases.recoveryDownloadExport": {
    message: "Descargar",
    description: "Download an available logical database export",
  },
  "databases.recoveryDownloadUnavailable": {
    message: "Esta exportación ya no está disponible para descargar.",
    description: "Toast when Download finds the export is no longer available",
  },
  "databases.recoveryDownloadError": {
    message: "No se pudo obtener el enlace de descarga. Inténtalo de nuevo.",
    description: "Toast when the download link of an export cannot be fetched",
  },
  "databases.recoveryExportInProgress": {
    message: "Ya hay una exportación en curso.",
    description: "Reason the create-export button is disabled",
  },
  "databases.recoveryExportPreparing": {
    message: "La exportación aún se está preparando.",
    description: "Reason an in-progress export cannot be downloaded",
  },
  "databases.recoveryExportExpired": {
    message: "Esta exportación ha caducado y su artefacto se eliminó.",
    description: "Reason an expired export cannot be downloaded",
  },
  "databases.recoveryExportUnavailable": {
    message: "Esta exportación no está disponible para su descarga.",
    description: "Fallback reason an export cannot be downloaded",
  },
  "databases.recoveryExportFailure": {
    message: "La exportación falló: {reason}",
    description: "Logical export failure reason",
  },
  "databases.recoveryExportStatusCreated": {
    message: "Creada",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusRunning": {
    message: "En ejecución",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusAvailable": {
    message: "Disponible",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusFailed": {
    message: "Fallida",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusExpiring": {
    message: "Por caducar",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusExpired": {
    message: "Caducada",
    description: "Logical export status",
  },
  "databases.recoveryRestoreTitle": {
    message: "Restaurar en una base de datos nueva",
    description: "Restore dialog title",
  },
  "databases.recoveryRestoreBody": {
    message:
      "Aprovisiona una base de datos nueva restaurada a partir de las copias de seguridad de esta. El origen no se modifica.",
    description: "Restore dialog body",
  },
  "databases.recoveryRestoreName": {
    message: "Nombre de la nueva base de datos",
    description: "Restore dialog field label",
  },
  "databases.recoveryRestoreTime": {
    message: "Punto en el tiempo (opcional)",
    description: "Restore dialog target-time field label",
  },
  "databases.recoveryRestoreTimeHint": {
    message: "Déjalo vacío para restaurar el último punto disponible.",
    description: "Restore dialog target-time helper text",
  },
  "databases.recoveryRestoreTimeZone": {
    message: "Las horas están en tu zona horaria local ({zone}).",
    description: "Restore dialog target-time zone note",
  },
  "databases.recoveryRestoreTimeOutOfWindow": {
    message:
      "Elige una hora entre el punto de restauración más antiguo y hace diez minutos.",
    description:
      "Restore dialog target-time out-of-window error (the window ends ten minutes before now, w7/m453)",
  },
  "databases.recoveryRetentionWindow_one": {
    message:
      "En el plan {plan} puedes restaurar cualquier momento del último {count} día.",
    description:
      "Recovery panel hint: how far back the workspace plan may restore a Postgres (w7/m453, from bex-api databaseRecoveryInfo.retention)",
  },
  "databases.recoveryRetentionWindow_many": {
    message:
      "En el plan {plan} puedes restaurar cualquier momento de los últimos {count} de días.",
    description:
      "Recovery panel hint: how far back the workspace plan may restore a Postgres (w7/m453, from bex-api databaseRecoveryInfo.retention)",
  },
  "databases.recoveryRetentionWindow_other": {
    message:
      "En el plan {plan} puedes restaurar cualquier momento de los últimos {count} días.",
    description:
      "Recovery panel hint: how far back the workspace plan may restore a Postgres (w7/m453, from bex-api databaseRecoveryInfo.retention)",
  },
  "databases.recoveryRetentionUpgrade_one": {
    message: "{plan} conserva {count} día.",
    description:
      "Recovery panel hint: the cheapest plan that restores further back, and its days (w7/m453)",
  },
  "databases.recoveryRetentionUpgrade_many": {
    message: "{plan} conserva {count} de días.",
    description:
      "Recovery panel hint: the cheapest plan that restores further back, and its days (w7/m453)",
  },
  "databases.recoveryRetentionUpgrade_other": {
    message: "{plan} conserva {count} días.",
    description:
      "Recovery panel hint: the cheapest plan that restores further back, and its days (w7/m453)",
  },
  "databases.recoveryRestoreConfirm": {
    message: "Restaurar",
    description: "Restore dialog confirm button",
  },
  "databases.recoveryExportStarted": {
    message: "Exportación iniciada.",
    description: "Toast after an export is triggered",
  },
  "databases.recoveryExportError": {
    message: "No se pudo iniciar la exportación. Inténtalo de nuevo.",
    description: "Toast when an export request fails",
  },
  "databases.recoveryRestoreStarted": {
    message: "Restaurando en {name}…",
    description: "Toast after a restore request is accepted",
  },
  "databases.recoveryRestoreError": {
    message: "No se pudo restaurar. Inténtalo de nuevo.",
    description: "Toast when a restore request fails",
  },
  "databases.accessTitle": {
    message: "Control de acceso",
    description: "Access-control panel card title",
  },
  "databases.accessDescription": {
    message:
      "Restringe el endpoint externo, agrupa conexiones y administra los usuarios de la base de datos.",
    description: "Access-control panel card description",
  },
  "databases.accessAllowList": {
    message: "Lista de IP permitidas",
    description:
      "Access panel section heading (external-endpoint CIDR allowlist)",
  },
  "databases.accessAllowListHint": {
    message:
      "Guardar reglas CIDR habilita el acceso externo para las IP de origen que coincidan. Guardar una lista vacía bloquea las conexiones externas. El acceso interno no cambia.",
    description: "Access panel IP allowlist helper text",
  },
  "databases.accessAllowListInternalOnly": {
    message:
      "Las conexiones externas están deshabilitadas. Guarda una o más reglas de IP para habilitar el acceso.",
    description: "External access is disabled; saving rules enables it",
  },
  "databases.accessAllowListEmpty": {
    message:
      "No hay reglas de IP. Guarda una lista vacía para bloquear las conexiones externas.",
    description: "Empty datastore allowlist draft and the effect of saving it",
  },
  "databases.accessAllowListAdd": {
    message: "Añadir",
    description: "Access panel button to add a CIDR to the draft allowlist",
  },
  "databases.accessAllowListDescription": {
    message: "Descripción (opcional)",
    description:
      "Placeholder for the optional per-entry allowlist description input",
  },
  "databases.accessAllowListCIDR": {
    message: "Nuevo bloque CIDR",
    description: "Accessible label for the add-row CIDR input",
  },
  "databases.accessAllowListCIDRRule": {
    message: "Bloque CIDR de la regla {number}",
    description: "Accessible label for an existing allowlist CIDR row",
  },
  "databases.accessAllowListDescriptionLabel": {
    message: "Descripción de la nueva regla",
    description: "Accessible label for the add-row description input",
  },
  "databases.accessAllowListDescriptionRule": {
    message: "Descripción de la regla {number}",
    description: "Accessible label for an existing allowlist description row",
  },
  "databases.accessAllowListRemove": {
    message: "Quitar {cidr}",
    description: "Accessible label to remove a CIDR chip",
  },
  "databases.accessAllowListMoveUp": {
    message: "Subir {cidr}",
    description: "Accessible label to move an allowlist entry earlier",
  },
  "databases.accessAllowListMoveDown": {
    message: "Bajar {cidr}",
    description: "Accessible label to move an allowlist entry later",
  },
  "databases.accessAllowListInvalid": {
    message: "Introduce bloques CIDR IPv4 o IPv6 válidos y únicos.",
    description: "Validation error for database allowlist entries",
  },
  "databases.accessAllowListSave": {
    message: "Guardar lista",
    description: "Access panel button to persist the allowlist",
  },
  "databases.accessAllowListSaved": {
    message: "Lista actualizada.",
    description: "Toast after saving the IP allowlist",
  },
  "databases.accessAllowListError": {
    message: "No se pudo guardar la lista. Inténtalo de nuevo.",
    description: "Toast when saving the allowlist fails",
  },
  "databases.accessCredentials": {
    message: "Credenciales",
    description:
      "Access panel section heading (Postgres credentials, Render's Credentials section)",
  },
  "databases.accessCredentialsHint": {
    message:
      "Añade una nueva credencial predeterminada para rotar la contraseña de la base de datos. La información de conexión cambia a ella y los servicios vinculados la adoptan en su próxima sincronización de Blueprint. Las credenciales anteriores siguen funcionando hasta que las elimines.",
    description: "Access panel credentials helper text",
  },
  "databases.accessCredentialDefault": {
    message: "Predeterminada",
    description: "Badge on the database's default credential",
  },
  "databases.accessCredentialAdd": {
    message: "Nueva credencial predeterminada",
    description:
      "Access panel button to create a credential that becomes the default",
  },
  "databases.accessCredentialName": {
    message: "Nombre de usuario de la nueva credencial",
    description: "Accessible label for the new-credential username input",
  },
  "databases.accessCredentialDelete": {
    message: "Eliminar {name}",
    description: "Accessible label to delete a database credential",
  },
  "databases.accessCredentialDeleteTitle": {
    message: "¿Eliminar la credencial {name}?",
    description:
      "Title of the confirmation before deleting a database credential (w4/551)",
  },
  "databases.accessCredentialDeleteBody": {
    message:
      "Todas las conexiones que usan {name} fallan de inmediato, incluidos los servicios cuyas variables de entorno contienen su cadena de conexión. Esta acción no se puede deshacer.",
    description:
      "Body of the credential delete confirmation: retiring the login breaks its clients at once (w4/551)",
  },
  "databases.accessCredentialDeleteConfirm": {
    message: "Eliminar credencial",
    description:
      "Confirm button of the credential delete confirmation (w4/551)",
  },
  "databases.accessCredentialDefaultLocked": {
    message:
      "La credencial predeterminada no se puede eliminar. Añade primero una nueva credencial predeterminada.",
    description: "Why the delete button is disabled on the default credential",
  },
  "databases.accessCredentialCreated": {
    message:
      "{name} es ahora la credencial predeterminada. Su contraseña está en la información de conexión.",
    description: "Toast after creating a credential",
  },
  "databases.accessCredentialError": {
    message: "No se pudo crear la credencial. Inténtalo de nuevo.",
    description: "Toast when creating a credential fails",
  },
  "databases.accessCredentialDeleted": {
    message: "Se solicitó la revocación de {name}.",
    description:
      "Toast after deleting a credential: the login is retired asynchronously on the next reconcile, so this confirms accepted intent, not a completed revocation",
  },
  "databases.accessCredentialDeleteError": {
    message: "No se pudo eliminar {name}. Inténtalo de nuevo.",
    description: "Toast when deleting a credential fails",
  },
  "databases.accessPooler": {
    message: "Agrupación de conexiones",
    description: "Access panel section heading (PgBouncer pooler strings)",
  },
  "databases.accessPoolerHint": {
    message:
      "Cadenas de conexión agrupadas a través de PgBouncer, para clientes con muchas conexiones.",
    description: "Access panel pooler helper text",
  },
  "databases.accessPoolerReveal": {
    message: "Mostrar cadenas agrupadas",
    description: "Access panel button to fetch the pooled connection strings",
  },
  "databases.accessPoolerDisabled": {
    message:
      "Activa la agrupación de conexiones más arriba para obtener cadenas de conexión agrupadas.",
    description:
      "Access panel shown when no pooler is enabled; points at the Connection Pool switch in the section heading (w2/m307)",
  },
  "databases.accessPoolerInternal": {
    message: "Conexión agrupada interna",
    description: "Access panel pooled-string label (in-cluster)",
  },
  "databases.accessPoolerPlanUnsupported": {
    message:
      "Este plan no admite la agrupación de conexiones. Elige un plan que la admita antes de habilitar un pooler.",
    description: "Pooler empty state on an unsupported plan",
  },
  "databases.accessPoolerExternal": {
    message: "Conexión agrupada externa",
    description: "Access panel pooled-string label (public)",
  },
  "databases.accessPoolerReplicaInternal": {
    message: "Conexión agrupada interna, réplica de lectura {name}",
    description:
      "Access panel pooled-string label for a read replica's in-cluster pooled connection (w6/m458)",
  },
  "databases.accessPoolerReplicaExternal": {
    message: "Conexión agrupada externa, réplica de lectura {name}",
    description:
      "Access panel pooled-string label for a read replica's public pooled connection (w6/m458)",
  },
  "databases.accessPoolerError": {
    message: "No se pudieron cargar las cadenas de conexión agrupadas.",
    description: "Toast when revealing pooled strings fails",
  },
  "databases.accessPoolerStarting": {
    message:
      "La agrupación de conexiones se está iniciando. Las cadenas de conexión agrupadas aparecerán aquí cuando PgBouncer esté listo.",
    description:
      "Access panel when pooling is on but the operator has not published the pooler hosts yet (w2/m307)",
  },
  "databases.poolerToggleLabel": {
    message: "Pool de conexiones",
    description:
      "Accessible label of the Connection Pool switch (Render's Info > Connection Pool, w2/m307)",
  },
  "databases.poolerEnableTitle": {
    message: "¿Habilitar la agrupación de conexiones para {name}?",
    description:
      "Connection-pool enable confirmation title; {name} is the database",
  },
  "databases.poolerDisableTitle": {
    message: "¿Deshabilitar la agrupación de conexiones para {name}?",
    description:
      "Connection-pool disable confirmation title; {name} is the database",
  },
  "databases.poolerEnableBody": {
    message:
      "Habilitar la agrupación de conexiones reinicia la base de datos, lo que provoca unos minutos de inactividad. Las cadenas de conexión agrupadas aparecerán en esta sección cuando PgBouncer esté listo.",
    description:
      "Connection-pool enable confirmation body for a database without high availability (Render: enabling restarts the database)",
  },
  "databases.poolerEnableBodyHA": {
    message:
      "Habilitar la agrupación de conexiones conmuta por error a la réplica en espera en lugar de reiniciar la base de datos, por lo que las conexiones abiertas se interrumpen brevemente. Las cadenas de conexión agrupadas aparecerán en esta sección cuando PgBouncer esté listo.",
    description:
      "Connection-pool enable confirmation body when high availability is on (Render: a failover instead of a restart)",
  },
  "databases.poolerDisableBody": {
    message:
      "Deshabilitar la agrupación de conexiones reinicia la base de datos, lo que provoca unos minutos de inactividad. Las cadenas de conexión agrupadas dejarán de funcionar.",
    description:
      "Connection-pool disable confirmation body for a database without high availability",
  },
  "databases.poolerDisableBodyHA": {
    message:
      "Deshabilitar la agrupación de conexiones conmuta por error a la réplica en espera en lugar de reiniciar la base de datos, por lo que las conexiones abiertas se interrumpen brevemente. Las cadenas de conexión agrupadas dejarán de funcionar.",
    description:
      "Connection-pool disable confirmation body when high availability is on",
  },
  "databases.poolerEnableConfirm": {
    message: "Habilitar pool de conexiones",
    description: "Connection-pool enable confirmation button (Render's label)",
  },
  "databases.poolerDisableConfirm": {
    message: "Deshabilitar pool de conexiones",
    description: "Connection-pool disable confirmation button",
  },
  "databases.poolerEnableSuccess": {
    message: "Se está habilitando la agrupación de conexiones para {name}.",
    description: "Toast after the connection-pool enable was accepted",
  },
  "databases.poolerDisableSuccess": {
    message: "Se está deshabilitando la agrupación de conexiones para {name}.",
    description: "Toast after the connection-pool disable was accepted",
  },
  "databases.poolerEnableError": {
    message: "No se pudo habilitar la agrupación de conexiones para {name}.",
    description: "Fallback toast when enabling the connection pool fails",
  },
  "databases.poolerDisableError": {
    message: "No se pudo deshabilitar la agrupación de conexiones para {name}.",
    description: "Fallback toast when disabling the connection pool fails",
  },
  "databases.haTitle": {
    message: "Alta disponibilidad",
    description: "HA panel card title",
  },
  "databases.haDescription": {
    message:
      "Clúster replicado con conmutación por error automática. Cuando está habilitada, siempre hay una réplica en espera lista para tomar el relevo.",
    description: "HA panel card description",
  },
  "databases.haStatus": {
    message: "Estado",
    description: "HA panel status row label",
  },
  "databases.haEnabled": {
    message: "Habilitada",
    description: "HA panel status value when HA is active",
  },
  "databases.haDisabled": {
    message: "Deshabilitada",
    description: "HA panel status value when HA is off",
  },
  "databases.haFailover": {
    message: "Activar conmutación por error",
    description: "HA panel button to initiate a planned switchover",
  },
  "databases.haFailoverConfirmTitle": {
    message: "¿Activar la conmutación por error en {name}?",
    description: "Failover-confirmation dialog title",
  },
  "databases.haFailoverConfirmBody": {
    message:
      "Esto promueve una réplica en espera a primario. Las conexiones se interrumpen brevemente durante el cambio. La base de datos sigue disponible y no se pierden datos.",
    description: "Failover-confirmation dialog body",
  },
  "databases.haFailoverCancel": {
    message: "Cancelar",
    description: "Failover-confirmation dialog cancel button",
  },
  "databases.haFailoverConfirm": {
    message: "Activar conmutación por error",
    description: "Failover-confirmation dialog confirm button",
  },
  "databases.haFailoverSuccess": {
    message: "Conmutación por error activada en {name}.",
    description: "Toast after a failover request is accepted",
  },
  "databases.haFailoverError": {
    message:
      "No se pudo activar la conmutación por error en {name}. Inténtalo de nuevo.",
    description: "Toast when a failover request fails",
  },
  "databases.haReadReplicas": {
    message: "Réplicas de lectura",
    description: "HA panel section heading for named read replicas",
  },
  "databases.haReadReplicasEmpty": {
    message: "No hay réplicas de lectura con nombre.",
    description: "HA panel empty state for read replicas",
  },
  "databases.haReplicaInternal": {
    message: "Interna",
    description: "HA panel replica connection label (in-cluster host)",
  },
  "databases.haReplicaExternal": {
    message: "Externa",
    description: "HA panel replica connection label (public SNI host)",
  },
  "databases.haAddReplica": {
    message: "Añadir réplica de lectura",
    description:
      "HA panel button that opens the add-read-replica dialog (w5/m138)",
  },
  "databases.haAddReplicaTitle": {
    message: "Añadir una réplica de lectura",
    description: "Add-read-replica dialog title",
  },
  "databases.haAddReplicaBody": {
    message:
      "Una réplica de lectura es una instancia independiente y facturable que recibe datos del primario y atiende consultas de solo lectura en su propia dirección.",
    description: "Add-read-replica dialog description",
  },
  "databases.haReplicaNameLabel": {
    message: "Nombre",
    description: "Add-read-replica dialog name field label",
  },
  "databases.haReplicaNameHint": {
    message:
      "Letras minúsculas, dígitos y guiones, hasta {max} caracteres. Pasa a formar parte del nombre de host de la réplica.",
    description: "Add-read-replica name hint; max is the API's length limit",
  },
  "databases.haReplicaNameInvalid": {
    message:
      "Usa entre 1 y {max} letras minúsculas, dígitos o guiones, empezando y terminando con una letra o un dígito.",
    description:
      "Add-read-replica name error; mirrors the API's POSTGRES_READ_REPLICA_NAME_INVALID",
  },
  "databases.haReplicaNameDuplicate": {
    message:
      "Esta base de datos ya tiene una réplica de lectura llamada {name}.",
    description: "Add-read-replica name error for a name already in use",
  },
  "databases.haAddReplicaSubmit": {
    message: "Añadir réplica",
    description: "Add-read-replica dialog submit button",
  },
  "databases.haAddReplicaSuccess": {
    message: "Se está creando la réplica de lectura {name}.",
    description: "Toast after a read replica is added",
  },
  "databases.haAddReplicaError": {
    message: "No se pudo añadir la réplica de lectura {name}.",
    description: "Toast when adding a read replica fails",
  },
  "databases.haRemoveReplica": {
    message: "Quitar",
    description: "HA panel per-replica remove button",
  },
  "databases.haRemoveReplicaTitle": {
    message: "¿Quitar la réplica de lectura {name}?",
    description: "Remove-read-replica confirmation title",
  },
  "databases.haRemoveReplicaBody": {
    message:
      "Su instancia se elimina y sus cadenas de conexión dejan de funcionar. El primario no se ve afectado.",
    description: "Remove-read-replica confirmation body",
  },
  "databases.haRemoveReplicaConfirm": {
    message: "Quitar réplica",
    description: "Remove-read-replica confirm button",
  },
  "databases.haRemoveReplicaSuccess": {
    message: "Se está quitando la réplica de lectura {name}.",
    description: "Toast after a read replica is removed",
  },
  "databases.haRemoveReplicaError": {
    message: "No se pudo quitar la réplica de lectura {name}.",
    description: "Toast when removing a read replica fails",
  },
  "databases.haReplicaPlanUnsupported": {
    message:
      "Las réplicas de lectura no están disponibles en el plan de esta base de datos.",
    description: "Why Add read replica is disabled: the plan allows none",
  },
  "databases.haReplicaLimitReached": {
    message: "Esta base de datos tiene el máximo de {max} réplicas de lectura.",
    description:
      "Why Add read replica is disabled: the plan's replica limit is reached",
  },
  "databases.haReplicaStorageTooSmall": {
    message:
      "Las réplicas de lectura necesitan al menos {min} GB de almacenamiento; esta base de datos tiene {current} GB.",
    description:
      "Why Add read replica is disabled: storage is below the replica minimum",
  },
  "databases.haNotEnabled": {
    message:
      "La alta disponibilidad no está habilitada para esta base de datos.",
    description: "HA panel line when HA is off on a plan that offers it",
  },
  "databases.haPlanUnsupported": {
    message:
      "La alta disponibilidad requiere un plan con al menos 1 CPU. El plan de esta base de datos no la ofrece.",
    description:
      "HA panel state when HA is off and the current plan is below 1 CPU (w8/m43, Render's rule)",
  },
  "databases.haToggleLabel": {
    message: "Alta disponibilidad",
    description: "Accessible name of the HA panel's on/off switch (w2/m171)",
  },
  "databases.haStarting": {
    message:
      "Se está iniciando una réplica en espera en otro nodo. La conmutación por error estará disponible cuando esté lista.",
    description:
      "HA panel line while HA is on but the standby is not ready yet",
  },
  "databases.haStopping": {
    message:
      "La alta disponibilidad se está desactivando. Se está quitando la réplica en espera.",
    description: "HA panel line while HA is off but the standby still runs",
  },
  "databases.haEnableTitle": {
    message: "¿Habilitar la alta disponibilidad en {name}?",
    description: "HA enable confirmation title",
  },
  "databases.haEnableBody": {
    message:
      "Se inicia una instancia en espera en otro nodo que toma el relevo automáticamente si el primario falla. Se factura igual que el primario.",
    description: "HA enable confirmation body",
  },
  "databases.haEnablePrice": {
    message: "La réplica en espera añade {price}.",
    description:
      "HA enable confirmation price note; {price} is a formatted monthly price",
  },
  "databases.haEnableConfirm": {
    message: "Habilitar alta disponibilidad",
    description: "HA enable confirmation button",
  },
  "databases.haDisableTitle": {
    message: "¿Deshabilitar la alta disponibilidad en {name}?",
    description: "HA disable confirmation title",
  },
  "databases.haDisableBody": {
    message:
      "Se quita la réplica en espera. Si el primario falla, la base de datos no estará disponible hasta que se recupere.",
    description: "HA disable confirmation body",
  },
  "databases.haDisableConfirm": {
    message: "Deshabilitar alta disponibilidad",
    description: "HA disable confirmation button",
  },
  "databases.haEnableSuccess": {
    message: "Se está habilitando la alta disponibilidad en {name}.",
    description: "Toast after HA was enabled",
  },
  "databases.haDisableSuccess": {
    message: "Se está deshabilitando la alta disponibilidad en {name}.",
    description: "Toast after HA was disabled",
  },
  "databases.haEnableError": {
    message: "No se pudo habilitar la alta disponibilidad en {name}.",
    description: "Fallback toast when enabling HA fails",
  },
  "databases.haDisableError": {
    message: "No se pudo deshabilitar la alta disponibilidad en {name}.",
    description: "Fallback toast when disabling HA fails",
  },
  "databases.planPickerHAUnsupported": {
    message:
      "{name} no admite alta disponibilidad (requiere al menos 1 CPU). Deshabilita primero la alta disponibilidad.",
    description:
      "Plan picker refusal when an HA database selects a plan below 1 CPU; mirrors the API message (w8/m43)",
  },
  "databases.planPickerStorageUnsupported": {
    message:
      "{name} permite hasta {max} GB de almacenamiento. Esta base de datos tiene {current} GB, que no se pueden reducir. Elige un plan con suficiente almacenamiento.",
    description: "Plan change blocked by the database's existing storage",
  },
  "databases.planPickerReplicasUnsupported": {
    message:
      "Límite de réplicas de lectura para {name}: {max}. Esta base de datos tiene {current}.",
    description: "Plan change blocked by the database's read replicas",
  },
  "databases.planPickerAutoscalingUnsupported": {
    message:
      "{name} no admite el escalado automático del disco. Desactiva primero el escalado automático del disco.",
    description: "Plan change blocked while disk autoscaling is enabled",
  },
  "databases.planPickerPoolerUnsupported": {
    message:
      "{name} no admite la agrupación de conexiones. Deshabilita primero el pooler.",
    description: "Plan change blocked while a connection pooler is enabled",
  },
  "databases.planTitle": {
    message: "Plan de cómputo",
    description: "Database detail plan-picker card title",
  },
  "databases.planDescription": {
    message:
      "Cambia el plan de cómputo. Una base de datos sin réplica en espera se reinicia para aplicarlo y no está disponible durante uno o dos minutos. Tus datos se conservan.",
    description: "Database detail plan-picker card description",
  },
  "databases.planPickerSave": {
    message: "Guardar",
    description: "Database plan-picker save button",
  },
  "databases.planPickerCancel": {
    message: "Cancelar",
    description: "Database plan-picker cancel / reset button",
  },
  "databases.planPickerConfirmTitle": {
    message: "¿Cambiar a {name}?",
    description: "Database plan-picker confirm dialog title",
  },
  "databases.planPickerConfirmBody": {
    message:
      "La base de datos se reinicia para aplicar el nuevo plan de cómputo y no está disponible durante uno o dos minutos. Las conexiones se interrumpen. Tus datos se conservan.",
    description: "Database plan-picker confirm dialog body",
  },
  "databases.planPickerConfirmPrice": {
    message: "El nuevo plan cuesta ${price}/mes.",
    description:
      "Plan-change confirm: the new plan's monthly price, from the API's price sheet (w4/156)",
  },
  "databases.planPickerSuccess": {
    message: "Actualizando el plan a {name}…",
    description: "Toast after a plan update is accepted",
  },
  "databases.planPickerError": {
    message: "No se pudo actualizar el plan. Inténtalo de nuevo.",
    description: "Toast when a plan update fails",
  },
  "databases.storageTitle": {
    message: "Almacenamiento",
    description:
      "Plan section Storage row heading (Render's Plan page Storage section, w2/m307)",
  },
  "databases.storageSize": {
    message: "{size} GB",
    description: "Plan section Storage row: the allocated storage",
  },
  "databases.storageFixed": {
    message:
      "Las bases de datos gratuitas tienen almacenamiento fijo. Cambia a un plan de pago para añadir almacenamiento.",
    description:
      "Why the Storage Edit button is disabled on a free database (bex-api POSTGRES_STORAGE_PLAN_UNSUPPORTED)",
  },
  "databases.storageAlmostFull": {
    message:
      "Tu base de datos está casi llena. Esto puede provocar errores al leer o escribir en tu base de datos e impedir las copias de seguridad automáticas. Considera liberar espacio en tu base de datos o {action}.",
    description:
      "Render's almost-full warning at the top of a Postgres database's Info, Plan and Recovery pages (w3/m656): more than 80% of its storage used and less than 5 GiB left. {action} is databases.storageAlmostFullIncrease or databases.storageAlmostFullUpgrade, drawn as a link to the Plan page, or databases.storageAlmostFullSupport",
  },
  "databases.storageAlmostFullIncrease": {
    message: "aumentar su almacenamiento",
    description:
      "The almost-full warning's ending for storage that can grow: a link to the Plan page's Storage control (Render's copy)",
  },
  "databases.storageAlmostFullUpgrade": {
    message: "cambiar a un plan de cómputo con más almacenamiento",
    description:
      "The almost-full warning's ending for a plan whose storage is fixed: a link to the Plan page's plan picker (Render's copy)",
  },
  "databases.storageAlmostFullSupport": {
    message:
      "contactar con soporte para crear un plan de cómputo personalizado",
    description:
      "The almost-full warning's ending for storage already at its plan's maximum, which no plan grows further (Render's copy); plain text",
  },
  "databases.storageEdit": {
    message: "Editar",
    description:
      "Plan section Storage row button that opens the storage dialog",
  },
  "databases.storageEditAria": {
    message: "Editar almacenamiento",
    description: "Accessible name of the Storage row's Edit button",
  },
  "databases.storageEditTitle": {
    message: "Editar el almacenamiento de {name}",
    description: "Storage dialog title; {name} is the database",
  },
  "databases.storageEditBody": {
    message:
      "Introduce cualquier múltiplo de {step} GB mayor que los {current} GB actuales. El almacenamiento no se puede reducir una vez que crece.",
    description:
      "Storage dialog description (Render: storage only grows, in 5 GB steps)",
  },
  "databases.storageEditLabel": {
    message: "Almacenamiento (GB)",
    description: "Storage dialog size field label",
  },
  "databases.storageEditStep": {
    message: "Introduce un múltiplo entero de {step} GB.",
    description: "Storage dialog validation: not a whole multiple of 5 GB",
  },
  "databases.storageEditNotGreater": {
    message:
      "Introduce un valor mayor que los {current} GB actuales. El almacenamiento no se puede reducir.",
    description:
      "Storage dialog validation: the size is not above the current storage",
  },
  "databases.storageEditTooLarge": {
    message: "{name} permite hasta {max} GB de almacenamiento.",
    description:
      "Storage dialog validation: the size is above the plan's maximum; {name} is the plan",
  },
  "databases.storageEditSave": {
    message: "Guardar",
    description: "Storage dialog submit button",
  },
  "databases.storageEditSuccess": {
    message: "Aumentando el almacenamiento a {size} GB…",
    description: "Toast after a storage increase is accepted",
  },
  "databases.storageEditError": {
    message: "No se pudo actualizar el almacenamiento. Inténtalo de nuevo.",
    description:
      "Storage dialog fallback error when the server gives no reason",
  },
  "databases.nameEdit": {
    message: "Editar nombre de la base de datos",
    description: "Accessible label for the database-name edit-in-place pencil",
  },
  "databases.nameDescription": {
    message:
      "Cambia el nombre para mostrar. El ID de la base de datos y todos los detalles de conexión siguen igual.",
    description: "Database detail rename row description",
  },
  "databases.nameSuccess": {
    message: "Base de datos renombrada a {name}.",
    description: "Toast after a database rename succeeds",
  },
  "databases.nameError": {
    message: "No se pudo renombrar la base de datos. Inténtalo de nuevo.",
    description: "Toast when a database rename fails unexpectedly",
  },
  "databases.nameConflict": {
    message:
      "Ya existe una base de datos con ese nombre en este espacio de trabajo.",
    description: "Toast when a database rename collides with another name",
  },
  "databases.nameInvalid": {
    message:
      "Introduce un nombre de hasta 100 caracteres, sin saltos de línea ni caracteres de control.",
    description: "Database rename validation message",
  },
  "databases.insightsTitle": {
    message: "Insights",
    description: "Database insights panel card title",
  },
  "databases.insightsDescription": {
    message:
      "Introspección en vivo de procesos, rendimiento de consultas, almacenamiento y configuración.",
    description: "Database insights panel card description",
  },
  "databases.insightsRefresh": {
    message: "Actualizar insights",
    description: "Accessible label for the insights refresh button",
  },
  "databases.insightsUnavailable": {
    message:
      "No disponible: puede que la base de datos aún se esté aprovisionando.",
    description: "Insights sub-section error state",
  },
  "databases.insightsSizeTitle": {
    message: "Almacenamiento",
    description: "Insights panel — database/table sizes sub-section heading",
  },
  "databases.insightsNoSizes": {
    message: "Aún no hay datos de tamaño.",
    description: "Insights storage empty state",
  },
  "databases.insightsProcessesTitle": {
    message: "Procesos activos",
    description: "Insights panel — pg_stat_activity sub-section heading",
  },
  "databases.insightsNoProcesses": {
    message: "No hay procesos activos.",
    description: "Insights processes empty state",
  },
  "databases.insightsTopQueriesTitle": {
    message: "Consultas principales",
    description: "Insights panel — pg_stat_statements sub-section heading",
  },
  "databases.insightsNoTopQueries": {
    message:
      "Aún no hay consultas. Las sentencias que ejecuten tu aplicación y la consola SQL aparecerán aquí.",
    description:
      "Insights top-queries empty state: the read succeeded and nothing the owner ran is in the list. A failed read, including a missing pg_stat_statements, shows insightsUnavailable instead, so this text must not blame the extension",
  },
  "databases.insightsQueryHidden": {
    message: "Oculta",
    description:
      "Insights — query cell for a row whose SQL PostgreSQL hides from the reading role",
  },
  "databases.insightsMaskedNote": {
    message:
      "Algunas filas pertenecen a otros roles de la base de datos, por lo que PostgreSQL oculta su SQL en esta vista. Todo lo demás de la fila es real.",
    description:
      "Insights — explanation shown when at least one process or top-query row has masked SQL",
  },
  "databases.insightsTableScansTitle": {
    message: "Escaneos de tablas",
    description: "Insights panel — pg_stat_user_tables sub-section heading",
  },
  "databases.insightsNoTableScans": {
    message: "Aún no hay tablas de usuario con estadísticas de escaneo.",
    description: "Insights table-scans empty state",
  },
  "databases.insightsParamsDeclaredValue": {
    message: "Valor declarado",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsObservedValue": {
    message: "Valor observado",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsObservedHint": {
    message:
      "Valor en vivo; no confirma que se haya aplicado el valor solicitado.",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsNotObserved": {
    message:
      "No se observó la anulación solicitada. Puede estar pendiente o ser inválida.",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsObservationUnavailable": {
    message: "No se puede verificar esta anulación ahora.",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsTitle": {
    message: "Anulaciones de parámetros",
    description:
      "Insights panel — heading for the parameters this database declares (read-only)",
  },
  "databases.insightsParamsDescription": {
    message:
      "Valores que esta base de datos declaró antes de que se retiraran las anulaciones de parámetros. Siguen aplicándose, pero ya no se pueden cambiar aquí; define valores de sesión predeterminados con ALTER DATABASE … SET en la consola SQL.",
    description:
      "Explains that the declared parameters are read-only legacy settings that still apply, and how to set session defaults instead",
  },
  "databases.insightsEffectiveParamsTitle": {
    message: "Parámetros no predeterminados",
    description: "Insights panel — pg_settings sub-section heading",
  },
  "databases.insightsEffectiveParamsDescription": {
    message:
      "Todos los valores vigentes que no están en su valor predeterminado, incluidos los que gestiona la plataforma. Solo lectura.",
    description:
      "Explains that the pg_settings table is observed configuration, mostly platform-owned, and cannot be edited",
  },
  "databases.insightsNoParams": {
    message: "Todos los parámetros tienen sus valores predeterminados.",
    description: "Insights parameter-overrides empty state",
  },
  "databases.insightsColTable": {
    message: "Tabla",
    description: "Insights table column header (table name)",
  },
  "databases.insightsColRelation": {
    message: "Tabla o índice",
    description:
      "Insights sizes column header (a table's or an index's name; tables and indexes are listed together)",
  },
  "databases.insightsIndexOn": {
    message: "índice en {table}",
    description:
      "Insights sizes: suffix naming the table an index row belongs to; {table} is the quoted schema-qualified table name",
  },
  "databases.insightsColSize": {
    message: "Tamaño",
    description: "Insights table column header (relation size)",
  },
  "databases.insightsColUser": {
    message: "Usuario",
    description: "Insights processes column header (pg role)",
  },
  "databases.insightsColState": {
    message: "Estado",
    description: "Insights processes column header (active/idle/…)",
  },
  "databases.insightsColDuration": {
    message: "Duración",
    description: "Insights processes column header (seconds since query start)",
  },
  "databases.insightsColQuery": {
    message: "Consulta",
    description: "Insights column header (SQL text, truncated)",
  },
  "databases.insightsColCalls": {
    message: "Llamadas",
    description: "Insights top-queries column header (invocation count)",
  },
  "databases.insightsColTotalTime": {
    message: "Tiempo total",
    description: "Insights top-queries column header (total exec time)",
  },
  "databases.insightsColMeanTime": {
    message: "Tiempo medio",
    description: "Insights top-queries column header (mean exec time per call)",
  },
  "databases.insightsColSeqScans": {
    message: "Escaneos secuenciales",
    description: "Insights table-scans column header (sequential scan count)",
  },
  "databases.insightsColIdxScans": {
    message: "Escaneos de índice",
    description: "Insights table-scans column header (index scan count)",
  },
  "databases.insightsColLiveRows": {
    message: "Filas activas",
    description:
      "Insights table-scans column header (estimated live row count)",
  },
  "databases.insightsColParam": {
    message: "Parámetro",
    description:
      "Insights parameter-overrides column header (pg_settings.name)",
  },
  "databases.insightsColSetting": {
    message: "Valor",
    description:
      "Insights parameter-overrides column header (pg_settings.setting)",
  },
  "databases.insightsColSource": {
    message: "Origen",
    description:
      "Insights parameter-overrides column header (pg_settings.source)",
  },
  "databases.sqlTitle": {
    message: "Consola SQL",
    description: "Database detail SQL console card title",
  },
  "databases.sqlDescription": {
    message:
      "Ejecuta una sola sentencia SQL en esta base de datos. Los resultados se limitan a 500 filas y las consultas agotan el tiempo de espera a los 10 segundos.",
    description: "Database SQL console safety summary",
  },
  "databases.sqlEditorLabel": {
    message: "Consulta SQL",
    description: "Accessible label for the SQL editor",
  },
  "databases.sqlShortcut": {
    message: "Pulsa Ctrl+Enter o ⌘+Enter para ejecutar",
    description: "SQL console keyboard shortcut hint",
  },
  "databases.sqlCreating": {
    message:
      "Esta base de datos aún se está creando. El SQL se podrá ejecutar cuando esté Disponible; tu consulta y el historial se conservan.",
    description: "SQL console notice while the database is creating (w4/177)",
  },
  "databases.sqlUpgrading": {
    message:
      "Se está ejecutando una actualización de PostgreSQL sin conexión. El SQL volverá a ejecutarse cuando la base de datos esté Disponible; tu consulta y el historial se conservan.",
    description: "SQL console notice while the database is upgrading (w4/177)",
  },
  "databases.sqlRestarting": {
    message:
      "Esta base de datos se está reiniciando. Espera a que esté Disponible para ejecutar SQL; tu consulta y el historial se conservan.",
    description: "SQL console notice while the database is restarting (w4/177)",
  },
  "databases.sqlSuspended": {
    message:
      "Esta base de datos está suspendida. Reanúdala para ejecutar SQL; tu consulta y el historial se conservan.",
    description: "SQL console notice while the database is suspended",
  },
  "databases.sqlRun": {
    message: "Ejecutar consulta",
    description: "SQL console run button",
  },
  "databases.sqlRunning": {
    message: "Ejecutando…",
    description: "SQL console run button while a query is executing",
  },
  "databases.sqlUnknownError": {
    message: "No se pudo ejecutar la consulta. Inténtalo de nuevo.",
    description: "SQL console fallback error",
  },
  "databases.sqlReturnedRows_one": {
    message: "{count} fila devuelta",
    description: "SQL console SELECT result count (singular)",
  },
  "databases.sqlReturnedRows_many": {
    message: "{count} de filas devueltas",
    description: "SQL console SELECT result count (plural)",
  },
  "databases.sqlReturnedRows_other": {
    message: "{count} filas devueltas",
    description: "SQL console SELECT result count (plural)",
  },
  "databases.sqlAffectedRows_one": {
    message: "{count} fila afectada",
    description: "SQL console write result count (singular)",
  },
  "databases.sqlAffectedRows_many": {
    message: "{count} de filas afectadas",
    description: "SQL console write result count (plural)",
  },
  "databases.sqlAffectedRows_other": {
    message: "{count} filas afectadas",
    description: "SQL console write result count (plural)",
  },
  "databases.sqlTruncated": {
    message: "Resultado limitado a {limit} filas",
    description: "SQL console backend row-cap notice",
  },
  "databases.sqlNull": {
    message: "NULL",
    description: "SQL console null cell value",
  },
  "databases.sqlNullLabel": {
    message: "SQL NULL (sin valor)",
    description:
      "SQL console accessible meaning of a null cell, distinct from the literal text NULL",
  },
  "databases.sqlNoRows": {
    message: "La consulta no devolvió ninguna fila.",
    description: "SQL console empty result message",
  },
  "databases.sqlPrevious": {
    message: "Anterior",
    description: "SQL result pagination previous button",
  },
  "databases.sqlNext": {
    message: "Siguiente",
    description: "SQL result pagination next button",
  },
  "databases.sqlPage": {
    message: "Página {page} de {pages}",
    description: "SQL result pagination position",
  },
  "databases.sqlHistory": {
    message: "Historial de consultas",
    description: "SQL console session history heading",
  },
  "databases.sqlClearHistory": {
    message: "Borrar",
    description: "SQL console clear-history button",
  },
  "databases.sqlNoHistory": {
    message:
      "Las consultas ejecutadas en esta sesión del navegador aparecen aquí.",
    description: "SQL console empty history message",
  },
  "databases.sqlConfirmTitle": {
    message: "¿Ejecutar una sentencia que puede modificar datos?",
    description: "SQL console write confirmation title",
  },
  "databases.sqlConfirmDescription": {
    message:
      "Esta sentencia no se reconoce como de solo lectura. Puede cambiar o eliminar datos de la base de datos y bex no puede deshacerlo.",
    description: "SQL console write confirmation warning",
  },
  "databases.sqlConfirmCancel": {
    message: "Cancelar",
    description: "SQL console write confirmation cancel button",
  },
  "databases.sqlConfirmRun": {
    message: "Ejecutar sentencia",
    description: "SQL console confirmed write action",
  },
  // --- Sidebar and page headings (w3/m641) ---
  "databases.navInfo": {
    message: "Información",
    description:
      "Postgres sidebar entry and page heading: the database's Info page (general details, connections, access, SQL console)",
  },
  "databases.navLogs": {
    message: "Logs",
    description:
      "Postgres sidebar entry and page heading: the database's Logs page",
  },
  "databases.navMetrics": {
    message: "Métricas",
    description:
      "Postgres sidebar entry and page heading: the database's Metrics page",
  },
  "databases.navPlan": {
    message: "Plan",
    description:
      "Postgres sidebar entry and page heading: the database's Plan page (compute plan, storage, high availability)",
  },
  "databases.navApps": {
    message: "Apps",
    description:
      "Postgres sidebar entry and page heading: the database's Apps page (pgAdmin, PgHero)",
  },
  "databases.navRecovery": {
    message: "Recuperación",
    description:
      "Postgres sidebar entry and page heading: the database's Recovery page (point-in-time recovery, exports)",
  },
  "databases.logsRangeLabel": {
    message: "Intervalo de tiempo",
    description: "Managed Postgres log range filter label",
  },
  "databases.logsInstanceLabel": {
    message: "Instancia de la base de datos",
    description: "Managed Postgres log instance filter label",
  },
  "databases.logsAllInstances": {
    message: "Todas las instancias",
    description: "Managed Postgres log instance filter all option",
  },
  "databases.logsSearchPlaceholder": {
    message: "Buscar en los logs de la base de datos",
    description: "Managed Postgres log search placeholder",
  },
  "databases.logsLoading": {
    message: "Cargando logs de la base de datos…",
    description: "Managed Postgres logs loading state",
  },
  "databases.logsEmptyTitle": {
    message: "Aún no hay logs de la base de datos",
    description: "Managed Postgres logs empty-state title",
  },
  "databases.logsEmptyBody": {
    message:
      "La salida de Postgres de este intervalo de tiempo aparecerá aquí.",
    description: "Managed Postgres logs empty-state body",
  },
  "databases.logsUnavailableTitle": {
    message: "Los logs de la base de datos no están configurados",
    description: "Managed Postgres logs unavailable-state title",
  },
  "databases.logsUnavailableBody": {
    message:
      "Esta instalación no tiene configurado ni un historial de logs duradero ni una fuente de logs de pods de CNPG en vivo.",
    description: "Managed Postgres logs unavailable-state body",
  },
  "databases.logsUnauthorizedTitle": {
    message: "No puedes ver estos logs",
    description: "Managed Postgres logs unauthorized-state title",
  },
  "databases.logsUnauthorizedBody": {
    message:
      "Los logs de la base de datos requieren acceso de colaborador en este espacio de trabajo.",
    description: "Managed Postgres logs unauthorized-state body",
  },
  "databases.logsErrorTitle": {
    message: "No se pudieron cargar los logs de la base de datos",
    description: "Managed Postgres logs generic error title",
  },
  "databases.fieldNameDescription": {
    message: "Un nombre único para tu instancia de Postgres.",
    description: "Create-Postgres Name row description",
  },
  "databases.fieldDatabaseNameDescription": {
    message:
      "El dbname de Postgres; solo se define al crearla. Déjalo vacío para generar uno.",
    description:
      "Create-Postgres Database row description: the physical dbname, create-time only",
  },
  "databases.fieldDatabaseUserDescription": {
    message:
      "El usuario de Postgres propietario de la base de datos; solo se define al crearla. Déjalo vacío para generar uno.",
    description:
      "Create-Postgres User row description: the physical owner role, create-time only",
  },
  "databases.fieldPlanDescription": {
    message:
      "Define la RAM y la CPU de tu base de datos. Puedes cambiar el cómputo más adelante.",
    description: "Create-Postgres plan row description (Render's Compute)",
  },
  "databases.fieldDiskDescription": {
    message:
      "La capacidad de tu base de datos, en GB. Puedes aumentarla más adelante, pero no reducirla.",
    description:
      "Create-Postgres Storage row description (Render: storage only grows)",
  },
  "databases.fieldDiskUnit": {
    message: "GB",
    description: "Unit after the create-Postgres storage input",
  },
  "databases.fieldVersionDescription": {
    message: "La versión principal de PostgreSQL que ejecuta la base de datos.",
    description: "Create-Postgres version row description",
  },
  // --- Apps: admin apps (Render's Postgres Apps page, w4/m551) ---
  "databases.adminAppsTitle": {
    message: "Apps",
    description:
      "Database page Apps section title and navigation label (Render's Postgres Apps page)",
  },
  "databases.adminAppsDescription": {
    message:
      "Despliega una app de administración como servicio web en la región y el entorno de esta base de datos, conectada a ella por la red privada.",
    description: "Database page Apps section description",
  },
  "databases.adminAppsErrorTitle": {
    message: "No se pudieron cargar las apps",
    description: "Database Apps section load error title",
  },
  "databases.adminAppsErrorBody": {
    message:
      "Algo salió mal al cargar las apps de esta base de datos. Inténtalo de nuevo en un momento.",
    description: "Database Apps section load error body",
  },
  "databases.adminAppPgAdminDescription": {
    message:
      "Administración general de PostgreSQL. Gestiona esquemas, tablas e índices. Ejecuta consultas puntuales y consulta sus planes de ejecución.",
    description: "pgAdmin card description (Render's copy)",
  },
  "databases.adminAppPgHeroDescription": {
    message:
      "Un panel de rendimiento para tu base de datos. Supervisa el uso de recursos, analiza las conexiones activas e inspecciona las consultas recientes para encontrar cuellos de botella.",
    description: "PgHero card description (Render's copy)",
  },
  "databases.adminAppDocs": {
    message: "Leer la documentación",
    description: "Admin app card link to the app's documentation",
  },
  "databases.adminAppDeploy": {
    message: "Desplegar app",
    description: "Admin app card button that opens the deploy dialog",
  },
  "databases.adminAppOpen": {
    message: "Abrir app",
    description:
      "Deployed admin app card button that opens the app in a new tab",
  },
  "databases.adminAppManage": {
    message: "Gestionar",
    description: "Deployed admin app card link to its service page",
  },
  "databases.adminAppDialogTitle": {
    message: "Crear una app de {app}",
    description: "Admin app deploy dialog title; {app} is pgAdmin or PgHero",
  },
  "databases.adminAppDialogDescription": {
    message:
      "Esto desplegará un nuevo servicio web en la misma región y entorno que tu base de datos. Puedes configurarlo como cualquier otro servicio web.",
    description: "Admin app deploy dialog body",
  },
  "databases.adminAppAuthentication": {
    message: "Autenticación",
    description: "Admin app deploy dialog login section heading",
  },
  "databases.adminAppAuthenticationHint": {
    message:
      "Hemos generado automáticamente un nombre de usuario y una contraseña para tu instancia de {app}. Puedes personalizarlos abajo.",
    description:
      "Admin app deploy dialog login section text; {app} is the app name",
  },
  "databases.adminAppUsername": {
    message: "Nombre de usuario",
    description: "Admin app deploy dialog username field label",
  },
  "databases.adminAppPassword": {
    message: "Contraseña",
    description: "Admin app deploy dialog password field label",
  },
  "databases.adminAppShow": {
    message: "Mostrar {field}",
    description:
      "Accessible label of the button that unmasks a login field; {field} is its label",
  },
  "databases.adminAppHide": {
    message: "Ocultar {field}",
    description:
      "Accessible label of the button that masks a login field; {field} is its label",
  },
  "databases.adminAppMonthlyCharges": {
    message: "Cargos mensuales",
    description: "Admin app deploy dialog billing section heading",
  },
  "databases.adminAppMonthlyChargesHint": {
    message:
      "Las instancias de servicio web se facturan de forma prorrateada por segundo. Los cargos de abajo solo reflejan los de {app}.",
    description:
      "Admin app deploy dialog billing section text; {app} is the app name",
  },
  "databases.adminAppBillingStarter": {
    message: "Servicio web - instancia Starter",
    description:
      "Admin app deploy dialog billing line for a Starter web service",
  },
  "databases.adminAppBillingFree": {
    message: "Servicio web - instancia Free",
    description: "Admin app deploy dialog billing line for a Free web service",
  },
  "databases.adminAppBillingDisk": {
    message: "Disco - {size} GB",
    description:
      "Admin app deploy dialog billing line for its disk; {size} is in GB",
  },
  "databases.adminAppBillingPrice": {
    message: "${usd} / mes",
    description:
      "Admin app deploy dialog billing line price; {usd} is a dollar amount",
  },
  "databases.adminAppDeploySubmit": {
    message: "Desplegar {app}",
    description: "Admin app deploy dialog submit button; {app} is the app name",
  },
  "databases.adminAppDeployed": {
    message: "{app} desplegado en {service}.",
    description:
      "Toast after an admin app deploys; {service} is its new service's name",
  },
  "databases.adminAppDeployError": {
    message: "No se pudo crear la app. ¿Intentarlo de nuevo?",
    description: "Admin app deploy dialog generic failure",
  },
  "databases.adminAppPaymentRequired": {
    message: "Añade un método de pago a este workspace para desplegar {app}.",
    description:
      "Admin app deploy dialog message when billing onboarding was closed without a payment method",
  },
  "databases.adminAppErrorTitle": {
    message: "Error",
    description: "Admin app deploy dialog error title",
  },
};

export default esDatabases;
