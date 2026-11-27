import type { TranslationEntry } from "@/i18n";

const esKeyValue: Record<string, TranslationEntry> = {
  "keyvalue.resourceType": {
    message: "Key Value",
    description: "Key Value resource type used in document titles",
  },
  "keyvalue.statTotal": {
    message: "Total de almacenes Key Value",
    description: "Key Value page stat card label",
  },
  "keyvalue.statAvailable": {
    message: "Disponibles",
    description: "Key Value page stat card label (healthy stores)",
  },
  "keyvalue.statCreating": {
    message: "Creándose",
    description: "Key Value page stat card label (provisioning stores)",
  },
  "keyvalue.cardTitle": {
    message: "Key Value",
    description: "Key Value table card title",
  },
  "keyvalue.colName": {
    message: "Nombre",
    description: "Key Value table column header",
  },
  "keyvalue.colStatus": {
    message: "Estado",
    description: "Key Value table column header",
  },
  "keyvalue.colPlan": {
    message: "Plan de cómputo",
    description: "Key Value table column header (plan / tier)",
  },
  "keyvalue.colVersion": {
    message: "Versión",
    description: "Key Value table column header (Valkey version)",
  },
  "keyvalue.colCreated": {
    message: "Creado",
    description: "Key Value table column header (relative age from createdAt)",
  },
  "keyvalue.colActions": {
    message: "Acciones",
    description: "Key Value table actions column header (screen-reader only)",
  },
  "keyvalue.statusAvailable": {
    message: "Disponible",
    description: "Key Value status badge (Valkey instance healthy)",
  },
  "keyvalue.statusCreating": {
    message: "Creándose",
    description: "Key Value status badge (provisioning)",
  },
  "keyvalue.statusRestarting": {
    message: "Reiniciándose",
    description:
      "Key Value status badge (Render's config_restart: an established store restarting after a config change, restart, or resume — not a first provision)",
  },
  "keyvalue.statusUnavailable": {
    message: "No disponible",
    description: "Key Value status badge (provisioning failed)",
  },
  "keyvalue.statusSuspended": {
    message: "Suspendido",
    description:
      "Key Value status badge (hibernated; suspension wins over the status enum)",
  },
  "keyvalue.statusUnknown": {
    message: "Desconocido",
    description: "Key Value status badge for an unrecognized status",
  },
  "keyvalue.statusReasonStatefulSetReadFailed": {
    message:
      "No se pudo comprobar el estado de la carga de trabajo del Key Value.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StatefulSetReadFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStatefulSetFailed": {
    message:
      "No se pudo actualizar la carga de trabajo del Key Value, por lo que no se aplicó el último cambio.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StatefulSetFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPVCReadFailed": {
    message: "No se pudo comprobar el estado del volumen de almacenamiento.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PVCReadFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPVCResizeFailed": {
    message:
      "No se pudo cambiar el tamaño del volumen de almacenamiento del Key Value.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PVCResizeFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonSecretFailed": {
    message: "No se pudieron guardar los detalles de conexión del Key Value.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's SecretFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonCredentialSecretFailed": {
    message: "No se pudieron guardar las credenciales del Key Value.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's CredentialSecretFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonSecretRecreateFailed": {
    message:
      "No se pudieron reconstruir los detalles de conexión del Key Value.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's SecretRecreateFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonServiceFailed": {
    message: "No se pudo configurar la dirección interna del Key Value.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's ServiceFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonNetworkPolicyFailed": {
    message: "No se pudo aplicar la política de red del Key Value.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's NetworkPolicyFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonBackupNetworkPolicyFailed": {
    message:
      "No se pudo aplicar la política de red del trabajo de copia de seguridad.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's BackupNetworkPolicyFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonBackupCronJobFailed": {
    message:
      "No se pudo actualizar la programación de la copia de seguridad diaria.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's BackupCronJobFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonTLSIssuerMissing": {
    message:
      "No se puede habilitar el acceso público: no hay ningún emisor de certificados configurado.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's TLSIssuerMissing Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonCertificateFailed": {
    message: "No se pudo emitir el certificado para el acceso público.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's CertificateFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonCertificateCleanupFailed": {
    message: "No se pudo eliminar el certificado del acceso público.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's CertificateCleanupFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageClassMissing": {
    message:
      "El volumen de almacenamiento no tiene clase de almacenamiento, por lo que no puede crecer.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageClassMissing Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageClassNotFound": {
    message:
      "No se encontró la clase de almacenamiento del volumen, por lo que no puede crecer.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageClassNotFound Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageClassNotExpandable": {
    message: "La clase de almacenamiento del volumen no permite que crezca.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageClassNotExpandable Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageBlockedByQuota": {
    message:
      "El volumen de almacenamiento no puede crecer: la cuota de almacenamiento del espacio de trabajo no tiene margen.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageBlockedByQuota Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPersistenceTransitionFailed": {
    message:
      "Falló la preparación de los datos almacenados para el inicio; los datos se conservan y se reintentará.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PersistenceTransitionFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPersistenceSourceUnknown": {
    message:
      "Se desconoce el modo de persistencia de los datos almacenados, por lo que el Key Value no puede iniciarse hasta que un operador de la plataforma lo confirme. No se modificó ningún dato.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PersistenceSourceUnknown Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageShrinkRejected": {
    message:
      "El almacenamiento de Key Value solo crece: el volumen no puede ser menor que su tamaño actual.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageShrinkRejected Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.errorTitle": {
    message: "No se pudieron cargar los almacenes Key Value",
    description: "Key Value list error card title",
  },
  "keyvalue.errorBody": {
    message:
      "La solicitud a bex-api falló. Comprueba tu conexión y vuelve a intentarlo.",
    description: "Key Value list error card body",
  },
  "keyvalue.emptyTitle": {
    message: "Aún no hay almacenes Key Value",
    description: "Key Value list empty state title",
  },
  "keyvalue.emptyBody": {
    message: "Crea tu primer almacén Key Value administrado y aparecerá aquí.",
    description: "Key Value list empty state body",
  },
  "keyvalue.actionsMenu": {
    message: "Acciones para {name}",
    description: "Accessible label for the per-row actions trigger",
  },
  "keyvalue.actionDelete": {
    message: "Eliminar",
    description: "Row action: permanently delete the Key Value store",
  },
  "keyvalue.deleteConfirmTitle": {
    message: "Eliminar instancia de Key Value",
    description: "Delete-confirmation dialog title",
  },
  "keyvalue.deleteConfirmBody": {
    message:
      "Esta acción no se puede deshacer. ¿Seguro que quieres eliminar esta instancia de Key Value?",
    description:
      "Delete-confirmation dialog body (verbatim Render copy, docs/render-artifacts/key-value.md)",
  },
  "keyvalue.deleteConfirmPrompt": {
    message: "Escribe {phrase} a continuación para confirmar.",
    description:
      "Body prompt naming Render's exact sudo delete phrase (rendered bold by SudoCommandField)",
  },
  "keyvalue.deleteCancel": {
    message: "Cancelar",
    description: "Delete-confirmation dialog cancel button",
  },
  "keyvalue.deleteConfirm": {
    message: "Eliminar instancia de Key Value",
    description: "Delete-confirmation dialog confirm button",
  },
  "keyvalue.deleteSuccess": {
    message: "Eliminando {name}…",
    description: "Toast after a delete request is accepted",
  },
  "keyvalue.deleteError": {
    message: "No se pudo eliminar {name}. Inténtalo de nuevo.",
    description: "Toast when a delete request fails",
  },
  "keyvalue.createButton": {
    message: "Nuevo Key Value",
    description: "Button that navigates to the create-Key-Value page",
  },
  "keyvalue.createTitle": {
    message: "Nuevo Key Value",
    description: "Create-Key-Value page title",
  },
  "keyvalue.createDescription": {
    message:
      "Aprovisiona una instancia administrada de Valkey (compatible con Redis).",
    description: "Create-Key-Value page description",
  },
  "keyvalue.fieldName": {
    message: "Nombre",
    description: "Create-Key-Value form field label (store name)",
  },
  "keyvalue.fieldNamePlaceholder": {
    message: "example-key-value-name",
    description: "Create-Key-Value name input placeholder",
  },
  "keyvalue.fieldNameError": {
    message:
      "Introduce un nombre de hasta 100 caracteres, sin saltos de línea ni caracteres de control.",
    description: "Create-Key-Value name validation message",
  },
  "keyvalue.fieldPlan": {
    message: "Plan de cómputo",
    description: "Create-Key-Value form field label (plan / tier)",
  },
  "keyvalue.fieldVersion": {
    message: "Versión de Valkey",
    description: "Create-Key-Value form field label (major version)",
  },
  "keyvalue.fieldVersionDefault": {
    message: "Predeterminada (la más reciente)",
    description: "Create-Key-Value version select default option",
  },
  "keyvalue.fieldMaxmemoryPolicy": {
    message: "Política de maxmemory",
    description: "Create-Key-Value form field label (eviction policy)",
  },
  "keyvalue.fieldMaxmemoryRecommended": {
    message: "(recomendada para cachés)",
    description:
      "Suffix on the recommended maxmemory policy option (allkeys-lru)",
  },
  "keyvalue.fieldMaxmemoryPolicyHint": {
    message:
      "Cómo se expulsan las claves cuando el almacén alcanza su límite de memoria.",
    description: "Create-Key-Value maxmemory policy helper text",
  },
  "keyvalue.fieldPersistenceMode": {
    message: "Modo de persistencia",
    description: "Create-Key-Value form field label (persistence)",
  },
  "keyvalue.fieldPersistenceModeHint": {
    message:
      "Cómo se persisten los datos en disco para que sobrevivan a un reinicio.",
    description: "Create-Key-Value persistence mode helper text",
  },
  "keyvalue.persistenceJournalSnapshot": {
    message: "Journal + Snapshot",
    description: "Persistence mode option: AOF journal plus RDB snapshots",
  },
  "keyvalue.persistenceSnapshot": {
    message: "Solo Snapshot",
    description: "Persistence mode option: RDB snapshots only",
  },
  "keyvalue.persistenceOff": {
    message: "Desactivada",
    description: "Persistence mode option: no persistence",
  },
  "keyvalue.fieldPublic": {
    message: "Acceso público",
    description: "Create-Key-Value form field label (external endpoint toggle)",
  },
  "keyvalue.fieldPublicHint": {
    message: "Permite conexiones desde fuera de tu red privada mediante TLS.",
    description: "Create-Key-Value public toggle helper text",
  },
  "keyvalue.createCancel": {
    message: "Cancelar",
    description: "Create-Key-Value page cancel button",
  },
  "keyvalue.createSubmit": {
    message: "Crear instancia de Key Value",
    description: "Create-Key-Value page submit button",
  },
  "keyvalue.createSuccess": {
    message: "Creando {name}…",
    description:
      "Toast after a create request is accepted (provisioning is async)",
  },
  "keyvalue.createError": {
    message: "No se pudo crear {name}. Inténtalo de nuevo.",
    description: "Toast when a create request fails",
  },
  "keyvalue.capLimitTitle": {
    message: "Se alcanzó el límite de Key Value",
    description:
      "Alert title when the workspace's KV creation cap is hit (w7/m9)",
  },
  "keyvalue.capLimitUpgrade": {
    message: "Mejorar plan",
    description: "Upgrade CTA button inside the KV cap-limit Alert (w7/m9)",
  },
  "keyvalue.metaTitle": {
    message: "Detalles",
    description: "Key Value detail metadata card title",
  },
  "keyvalue.metaId": {
    message: "ID",
    description: "Key Value detail metadata row label (immutable red- id)",
  },
  "keyvalue.metaStatus": {
    message: "Estado",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaPlan": {
    message: "Plan de cómputo",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaConnectionLimit": {
    message: "Límite de conexiones",
    description:
      "Key Value detail Details card row label: the plan's client connection limit, as Render's Key Value Instance card shows it",
  },
  "keyvalue.metaVersion": {
    message: "Versión",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaPublic": {
    message: "Acceso público",
    description: "Key Value detail metadata row label (external endpoint)",
  },
  "keyvalue.metaExternalHost": {
    message: "Host externo",
    description: "Key Value detail metadata row label (SNI hostname)",
  },
  "keyvalue.metaRegion": {
    message: "Región",
    description: "Key Value detail metadata row label (platform placement)",
  },
  "keyvalue.metaCreated": {
    message: "Creado",
    description: "Key Value detail metadata row label (relative age)",
  },
  "keyvalue.yes": {
    message: "Sí",
    description: "Metadata value for a true boolean field",
  },
  "keyvalue.publicPending": {
    message: "Sí (abriendo endpoint externo: certificado pendiente)",
    description:
      "Details value for Public access while the store's certificate is pending, so it does not read as reachable (w4/m357)",
  },
  "keyvalue.publicFailed": {
    message: "Sí (aún no accesible: falló la emisión del certificado)",
    description:
      "Details value for Public access when the store's certificate failed to issue (w4/m357)",
  },
  "keyvalue.no": {
    message: "No",
    description: "Metadata value for a false boolean field",
  },
  "keyvalue.connTitle": {
    message: "Conexiones",
    description: "Connection-info panel card title",
  },
  "keyvalue.connDescription": {
    message:
      "Cadenas de conexión de este almacén Key Value. Solo se muestran cuando lo pides, nunca de forma automática.",
    description: "Connection-info panel card description",
  },
  "keyvalue.internalAuthOff": {
    message: "Las conexiones internas no requieren contraseña.",
    description: "Connections panel: internal auth off",
  },
  "keyvalue.internalAuthOn": {
    message: "Las conexiones internas requieren la contraseña.",
    description: "Connections panel: internal auth on",
  },
  "keyvalue.internalAuthEnable": {
    message: "Habilitar autenticación interna",
    description: "Button and confirm label (Render's wording)",
  },
  "keyvalue.internalAuthConfirmTitle": {
    message: "¿Habilitar la autenticación interna?",
    description: "Confirm dialog title",
  },
  "keyvalue.internalAuthConfirmBody": {
    message:
      "Todas las conexiones internas necesitarán la contraseña. Las conexiones existentes que usen la URL sin autenticación dejarán de funcionar, y los servicios vinculados adoptarán la nueva URL en su próximo despliegue; actualízalos a la URL que aparece en la información de conexión. Esta acción no se puede deshacer.",
    description: "Confirm dialog body",
  },
  "keyvalue.internalAuthEnabled": {
    message: "Autenticación interna habilitada para {name}.",
    description: "Toast after enabling",
  },
  "keyvalue.internalAuthError": {
    message:
      "No se pudo habilitar la autenticación interna. Inténtalo de nuevo.",
    description: "Toast when enabling fails",
  },
  "keyvalue.connReveal": {
    message: "Mostrar información de conexión",
    description: "Button that fetches the connection info on demand",
  },
  "keyvalue.connHide": {
    message: "Ocultar información de conexión",
    description: "Button that clears the revealed connection info",
  },
  "keyvalue.connInternal": {
    message: "URL interna de Key Value",
    description: "Connection-info field label (in-cluster redis:// URL)",
  },
  "keyvalue.connExternal": {
    message: "URL externa de Key Value",
    description: "Connection-info field label (public rediss:// URL)",
  },
  "keyvalue.connExternalPending": {
    message:
      "Se está habilitando el acceso externo: se está emitiendo un certificado de confianza pública (suele tardar unos minutos). La URL externa y el comando de la CLI conectan cuando el almacén acepta conexiones TLS; hasta entonces solo es accesible desde tus servicios, con la URL interna. Oculta y vuelve a mostrar para actualizar.",
    description:
      "Shown under the external URL while a public store's TLS listener is still opening (w9/m521)",
  },
  "keyvalue.connExternalFailed": {
    message:
      "La URL externa y el comando de la CLI aún no conectan: falló la emisión del certificado de confianza pública de este almacén, y bex lo reintenta automáticamente. Hasta que se logre, el almacén solo es accesible desde tus servicios, con la URL interna.",
    description:
      "Shown under the external URL when a public store's certificate failed to issue (w4/m357, w9/m521)",
  },
  "keyvalue.connExternalUnavailable": {
    message:
      "No es público. Añade una regla de IP de entrada en Redes para habilitar el acceso externo.",
    description:
      "Shown instead of the external URL and the CLI command when the store isn't public (w9/134)",
  },
  "keyvalue.connCli": {
    message: "Comando de Valkey CLI",
    description:
      "Connection-info field label (ready-to-run valkey-cli command)",
  },
  "keyvalue.connErrorTitle": {
    message: "No se pudo cargar la información de conexión",
    description: "Connection-info panel error title",
  },
  "keyvalue.connErrorBody": {
    message:
      "Puede que el almacén aún se esté aprovisionando o que no tengas permiso para ver sus credenciales.",
    description: "Connection-info panel error body",
  },
  "keyvalue.copied": {
    message: "Copiado al portapapeles",
    description: "Toast after copying a connection field",
  },
  "keyvalue.copyError": {
    message: "No se pudo copiar al portapapeles",
    description: "Toast when clipboard copy fails",
  },
  "keyvalue.dangerDelete": {
    message: "Eliminar instancia de Key Value",
    description: "Discoverable delete action at the bottom of Key Value Info",
  },
  "keyvalue.actionSuspend": {
    message: "Suspender instancia de Key Value",
    description: "Suspend action button label",
  },
  "keyvalue.actionResume": {
    message: "Reanudar instancia de Key Value",
    description: "Resume action button label",
  },
  "keyvalue.confirmSuspendTitle": {
    message: "Suspender instancia de Key Value",
    description: "Suspend-confirmation dialog title",
  },
  "keyvalue.confirmSuspendBody": {
    message: "¿Seguro que quieres suspender esta instancia de Key Value?",
    description: "Suspend-confirmation dialog description",
  },
  "keyvalue.confirmSuspendDetail": {
    message:
      "Confirma que quieres suspender {name}. Las conexiones activas se cerrarán y podrás reanudarla en cualquier momento. La conservación de datos depende del modo de persistencia: Desactivada pierde todos los datos; Solo Snapshot conserva los datos hasta el último snapshot; Journal + Snapshot conserva las escrituras hasta aproximadamente el último segundo.",
    description: "Suspend-confirmation consequences and named instance",
  },
  "keyvalue.confirmSuspendPrompt": {
    message: "Escribe {phrase} a continuación para confirmar.",
    description:
      "Body prompt naming Render's exact Key Value suspend phrase (rendered bold by SudoCommandField)",
  },
  "keyvalue.confirmCancel": {
    message: "Cancelar",
    description: "Suspend-confirmation dialog cancel button",
  },
  "keyvalue.toastSuspendSuccess": {
    message: "Suspendiendo {name}…",
    description: "Toast after a suspend request is accepted",
  },
  "keyvalue.toastSuspendError": {
    message: "No se pudo suspender {name}. Inténtalo de nuevo.",
    description: "Toast when a suspend request fails",
  },
  "keyvalue.toastResumeSuccess": {
    message: "Reanudando {name}…",
    description: "Toast after a resume request is accepted",
  },
  "keyvalue.toastResumeError": {
    message: "No se pudo reanudar {name}. Inténtalo de nuevo.",
    description: "Toast when a resume request fails",
  },
  "keyvalue.planTitle": {
    message: "Plan de cómputo",
    description: "Key Value detail plan-picker card title",
  },
  "keyvalue.planDescription": {
    message:
      "Cambia a un plan de cómputo mayor; una instancia de Key Value no puede pasar a una menor. El cambio reinicia el almacén, que no está disponible durante uno o dos minutos. Los datos sobreviven según el modo de persistencia; un almacén con la persistencia desactivada pierde todas las claves.",
    description: "Key Value detail plan-picker card description",
  },
  "keyvalue.planDowngradeUnavailable": {
    message:
      "Una instancia de Key Value no puede pasar a un plan de cómputo menor. Crea una instancia nueva con este plan y traslada tus datos a ella.",
    description:
      "Tooltip on a disabled plan card smaller than the store's current plan: Render does not downgrade a Key Value instance (w8/249)",
  },
  "keyvalue.planPausedReason": {
    message:
      "El plan de Key Value “{name}” no está disponible temporalmente; elige {largest} o un plan menor.",
    description:
      "A paused Key Value plan's tooltip and screen-reader reason, composed in the dashboard's language; matches bex-api's English unavailableReason (w4/552)",
  },
  "keyvalue.planPickerSave": {
    message: "Guardar",
    description: "Key Value plan-picker save button",
  },
  "keyvalue.planPickerCancel": {
    message: "Cancelar",
    description: "Key Value plan-picker cancel / reset button",
  },
  "keyvalue.reviewComputeLead": {
    message: "Revisa tu selección de cómputo y almacenamiento.",
    description:
      "Key Value plan review: the lead of a compute plan change's review (compute and storage, as the review lists both)",
  },
  "keyvalue.reviewUnavailable": {
    message:
      "Tu instancia de Key Value no estará disponible mientras se actualiza.",
    description:
      "Key Value plan review warning: the store restarts to apply the plan (Render's copy); the data-loss sentence follows it when persistence is Off",
  },
  "keyvalue.reviewUpgradeCompute": {
    message: "Mejorar cómputo",
    description:
      "Key Value plan review: the button that applies a plan change (Render's copy; Key Value plans only go up)",
  },
  "keyvalue.storageTitle": {
    message: "Almacenamiento",
    description:
      "Key Value plan review: the Storage line's label (the store's billed storage)",
  },
  "keyvalue.storageSize": {
    message: "{size} GB",
    description:
      "Key Value plan review: the Storage line's figure, the billed size in GB",
  },
  "keyvalue.restartConfirmDataLoss": {
    message:
      "La persistencia está desactivada, por lo que el reinicio elimina todas las claves de {name}. Esta acción no se puede deshacer.",
    description:
      "Restart confirm (plan change): a Persistence Off store loses all data on the restart (w4/m263)",
  },
  "keyvalue.restartConfirmDataLossUnknown": {
    message:
      "Se desconoce el modo de persistencia de este almacén. Si está desactivada, el reinicio elimina todas las claves de {name}. Esta acción no se puede deshacer.",
    description:
      "Restart confirm when the persistence mode could not be read: the typed phrase is still required",
  },
  "keyvalue.planPickerSuccess": {
    message: "Actualizando el plan a {name}…",
    description: "Toast after a plan update is accepted",
  },
  "keyvalue.planPickerError": {
    message: "No se pudo actualizar el plan. Inténtalo de nuevo.",
    description: "Toast when a plan update fails",
  },
  "keyvalue.maxmemoryTitle": {
    message: "Política de maxmemory",
    description: "Key Value detail eviction-policy card title",
  },
  "keyvalue.maxmemoryDescription": {
    message:
      "Cambia cómo se expulsan las claves cuando el almacén alcanza su límite de memoria. La nueva política se aplica al almacén en ejecución sin reiniciarlo.",
    description: "Key Value detail eviction-policy card description",
  },
  "keyvalue.maxmemoryLabel": {
    message: "Política de maxmemory",
    description: "Accessible label for the eviction-policy select",
  },
  "keyvalue.maxmemoryEdit": {
    message: "Editar política de maxmemory",
    description:
      "Accessible label for the maxmemory-policy edit-in-place pencil",
  },
  "keyvalue.maxmemorySuccess": {
    message: "Actualizando la política de maxmemory a {policy}…",
    description: "Toast after an eviction-policy update is accepted",
  },
  "keyvalue.maxmemoryError": {
    message:
      "No se pudo actualizar la política de maxmemory. Inténtalo de nuevo.",
    description: "Toast when an eviction-policy update fails",
  },
  "keyvalue.persistenceTitle": {
    message: "Modo de persistencia",
    description: "Key Value detail persistence-mode card title",
  },
  "keyvalue.persistenceDescription": {
    message:
      "Cambiar de modo reinicia el almacén e interrumpe las conexiones mientras se preparan los datos. Cambiar entre Journal + Snapshot y Solo Snapshot conserva los datos. Cambiar a Desactivada o desde ella descarta todos los datos.",
    description: "Key Value detail persistence-mode card description",
  },
  "keyvalue.persistenceLabel": {
    message: "Modo de persistencia",
    description: "Accessible label for the persistence-mode select",
  },
  "keyvalue.persistenceEdit": {
    message: "Editar modo de persistencia",
    description:
      "Accessible label for the persistence-mode edit-in-place pencil",
  },
  "keyvalue.persistenceSuccess": {
    message: "Actualizando el modo de persistencia a {mode}…",
    description: "Toast after a persistence-mode update is accepted",
  },
  "keyvalue.persistenceConfirmTitle": {
    message: "¿Eliminar todos los datos y cambiar la persistencia?",
    description: "Destructive persistence-mode confirmation title",
  },
  "keyvalue.persistenceConfirmDescription": {
    message:
      "Cambiar a Desactivada o desde ella elimina todos los datos de {name} y reinicia el almacén. Esta acción no se puede deshacer.",
    description: "Destructive persistence-mode confirmation description",
  },
  "keyvalue.persistenceConfirmAction": {
    message: "Eliminar datos y cambiar de modo",
    description: "Destructive persistence-mode confirmation action",
  },
  "keyvalue.persistenceError": {
    message:
      "No se pudo actualizar el modo de persistencia. Inténtalo de nuevo.",
    description: "Toast when a persistence-mode update fails",
  },
  "keyvalue.networkingTitle": {
    message: "Redes",
    description: "Detail-page Networking card title (IP allowlist)",
  },
  "keyvalue.networkingDescription": {
    message: "Restringe qué IP de origen pueden acceder al endpoint externo.",
    description: "Networking card subtitle",
  },
  "keyvalue.networkingHint": {
    message:
      "Guardar reglas CIDR habilita el acceso externo para las IP de origen que coincidan. Guardar una lista vacía bloquea las conexiones externas. El acceso interno no cambia.",
    description: "Networking IP allowlist helper text",
  },
  "keyvalue.networkingHintRestart": {
    message:
      "Guardar reglas CIDR habilita el acceso externo para las IP de origen que coincidan. Este almacén nunca ha tenido acceso externo, por lo que el primer guardado lo reinicia y las conexiones se interrumpen hasta un minuto. Guardar una lista vacía bloquea las conexiones externas.",
    description:
      "Networking IP allowlist helper text when enabling external access restarts the store (its first enable)",
  },
  "keyvalue.networkingRestartConfirmTitle": {
    message: "¿Eliminar todos los datos y habilitar el acceso externo?",
    description:
      "Confirmation title when a first external-access enable restarts a persistence-Off store",
  },
  "keyvalue.networkingRestartConfirmDescription": {
    message:
      "Habilitar el acceso externo reinicia {name} por primera vez. Su modo de persistencia es Desactivada, por lo que el reinicio elimina todos los datos. Esta acción no se puede deshacer.",
    description:
      "Confirmation description when a first external-access enable restarts a persistence-Off store",
  },
  "keyvalue.networkingRestartConfirmAction": {
    message: "Eliminar datos y guardar reglas",
    description:
      "Confirmation action when a first external-access enable restarts a persistence-Off store",
  },
  "keyvalue.networkingInternalOnly": {
    message:
      "Las conexiones externas están deshabilitadas. Guarda una o más reglas de IP para habilitar el acceso.",
    description: "External access is disabled; saving rules enables it",
  },
  "keyvalue.networkingEmpty": {
    message:
      "No hay reglas de IP. Guarda una lista vacía para bloquear las conexiones externas.",
    description: "Empty datastore allowlist draft and the effect of saving it",
  },
  "keyvalue.networkingAdd": {
    message: "Añadir",
    description: "Networking button to add a CIDR to the draft allowlist",
  },
  "keyvalue.networkingEntryDescription": {
    message: "Descripción (opcional)",
    description:
      "Placeholder for the optional per-entry allowlist description input",
  },
  "keyvalue.networkingCIDR": {
    message: "Nuevo bloque CIDR",
    description: "Accessible label for the add-row CIDR input",
  },
  "keyvalue.networkingCIDRRule": {
    message: "Bloque CIDR de la regla {number}",
    description: "Accessible label for an existing allowlist CIDR row",
  },
  "keyvalue.networkingDescriptionLabel": {
    message: "Descripción de la nueva regla",
    description: "Accessible label for the add-row description input",
  },
  "keyvalue.networkingDescriptionRule": {
    message: "Descripción de la regla {number}",
    description: "Accessible label for an existing allowlist description row",
  },
  "keyvalue.networkingRemove": {
    message: "Quitar {cidr}",
    description: "Accessible label to remove a CIDR chip",
  },
  "keyvalue.networkingMoveUp": {
    message: "Subir {cidr}",
    description: "Accessible label to move an allowlist entry earlier",
  },
  "keyvalue.networkingMoveDown": {
    message: "Bajar {cidr}",
    description: "Accessible label to move an allowlist entry later",
  },
  "keyvalue.networkingInvalid": {
    message: "Introduce bloques CIDR IPv4 o IPv6 válidos y únicos.",
    description: "Validation error for Key Value allowlist entries",
  },
  "keyvalue.networkingSave": {
    message: "Guardar lista",
    description: "Networking button to persist the allowlist",
  },
  "keyvalue.networkingSaved": {
    message: "Lista actualizada.",
    description: "Toast after saving the IP allowlist",
  },
  "keyvalue.networkingError": {
    message: "No se pudo guardar la lista. Inténtalo de nuevo.",
    description: "Toast when saving the allowlist fails",
  },
  "keyvalue.nameEdit": {
    message: "Editar nombre de Key Value",
    description: "Accessible label for the Key Value name edit-in-place pencil",
  },
  "keyvalue.nameDescription": {
    message:
      "Cambia el nombre para mostrar. El ID de Key Value y todos los detalles de conexión siguen igual.",
    description: "Key Value detail rename row description",
  },
  "keyvalue.nameSuccess": {
    message: "Almacén Key Value renombrado a {name}.",
    description: "Toast after a Key Value rename succeeds",
  },
  "keyvalue.nameError": {
    message: "No se pudo renombrar el almacén Key Value. Inténtalo de nuevo.",
    description: "Toast when a Key Value rename fails unexpectedly",
  },
  "keyvalue.nameConflict": {
    message:
      "Ya existe un almacén Key Value con ese nombre en este espacio de trabajo.",
    description: "Toast when a Key Value rename collides with another name",
  },
  "keyvalue.nameInvalid": {
    message:
      "Introduce un nombre de hasta 100 caracteres, sin saltos de línea ni caracteres de control.",
    description: "Key Value rename validation message",
  },
  // --- Sidebar and page headings (w3/m649) ---
  "keyvalue.navInfo": {
    message: "Información",
    description:
      "Key Value sidebar entry and page heading: the instance's Info page (general details, plan, connections, networking)",
  },
  "keyvalue.navLogs": {
    message: "Logs",
    description:
      "Key Value sidebar entry and page heading: the instance's Logs page",
  },
  "keyvalue.navMetrics": {
    message: "Métricas",
    description:
      "Key Value sidebar entry and page heading: the instance's Metrics page",
  },
  "keyvalue.logsRangeLabel": {
    message: "Intervalo de tiempo",
    description: "Accessible label for the log time-range select",
  },
  "keyvalue.logsAllInstances": {
    message: "Todas las instancias",
    description: "Default option in the instance filter (no filter applied)",
  },
  "keyvalue.logsSearchPlaceholder": {
    message: "Buscar en los logs…",
    description: "Placeholder for the log text-search input",
  },
  "keyvalue.logsLoading": {
    message: "Cargando logs…",
    description: "Loading state message while fetching logs",
  },
  "keyvalue.logsEmptyTitle": {
    message: "No hay líneas de log",
    description: "Empty state title when no log lines are returned",
  },
  "keyvalue.logsEmptyBody": {
    message:
      "No hay líneas de log en este intervalo de tiempo. Valkey registra eventos del espacio de claves, consultas lentas y mensajes de inicio.",
    description: "Empty state body when no log lines and no filters active",
  },
  "keyvalue.logsUnavailableTitle": {
    message: "Logs no disponibles",
    description: "Empty state title when the logs source is not configured",
  },
  "keyvalue.logsUnavailableBody": {
    message:
      "La fuente de logs de la plataforma no está configurada. Contacta con el administrador de tu plataforma para habilitar BEX_LOKI_URL.",
    description: "Empty state body when BEX_LOKI_URL is not set",
  },
  "keyvalue.logsUnauthorizedTitle": {
    message: "Acceso denegado",
    description: "Empty state title when the caller lacks can_view_logs",
  },
  "keyvalue.logsUnauthorizedBody": {
    message: "No tienes permiso para ver los logs de este almacén Key Value.",
    description: "Empty state body for a 403 on the logs query",
  },
  "keyvalue.fieldNameDescription": {
    message: "Un nombre único para tu instancia de Key Value.",
    description: "Create-Key-Value Name row description",
  },
  "keyvalue.fieldVersionDescription": {
    message: "La versión principal de Valkey que ejecuta la instancia.",
    description: "Create-Key-Value Version row description",
  },
  "keyvalue.fieldPlanDescription": {
    message:
      "Determina la RAM disponible y el límite de conexiones de la instancia.",
    description:
      "Create-Key-Value plan row description (Render: the plan determines RAM and connection limit)",
  },
  "keyvalue.connectInternalTitle": {
    message: "URL interna de Redis",
    description:
      "Header Connect menu, Internal tab: the private-network URL's title (Render's copy, which names Key Value by its type name Redis, w3/m675)",
  },
  "keyvalue.connectExternalTitle": {
    message: "URL externa de Redis",
    description:
      "Header Connect menu, External tab: the public URL's title (Render's copy, w3/m675)",
  },
  "keyvalue.connectCliTitle": {
    message: "Comando de Redis CLI",
    description:
      "Header Connect menu, External tab: the `bex kv-cli <id>` command's title (Render's copy, w3/m675)",
  },
};

export default esKeyValue;
