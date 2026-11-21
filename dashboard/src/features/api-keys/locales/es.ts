import type { TranslationEntry } from "@/i18n";

const esApiKeys: Record<string, TranslationEntry> = {
  "apiKeys.title": {
    message: "Claves de API",
    description: "Settings API Keys section card title",
  },
  "apiKeys.description": {
    message:
      "Credenciales de máquina para scripts y agentes. Cada clave solo puede usar los permisos elegidos para ella, nunca más de lo que permite el rol de miembro actual de quien la creó, y solo en este espacio de trabajo. Ves y administras las claves que creaste; un Admin también puede ver y revocar las claves de todos los miembros.",
    description:
      "Settings API Keys section card description: what a key is, that it uses only its chosen permissions within its creator's current role in this workspace (w9/m496), and who sees which keys: every member their own, as on Render's Account Settings, and an Admin every member's too (w9/m539)",
  },
  "apiKeys.cliGuide": {
    message: "Configura la CLI.",
    description: "Link from the API Keys card to the bex CLI setup guide",
  },
  "apiKeys.scopeLabel": {
    message: "Qué claves de API mostrar",
    description:
      "Accessible name of an Admin's switch between their own API keys and every member's (w9/m539)",
  },
  "apiKeys.scopeOwn": {
    message: "Tus claves",
    description: "API Keys switch option: the keys the caller created",
  },
  "apiKeys.scopeAll": {
    message: "Claves de todos los miembros",
    description:
      "API Keys switch option, an Admin's only: every member's keys in the workspace",
  },
  "apiKeys.colName": {
    message: "Nombre",
    description: "API Keys table column header",
  },
  "apiKeys.colClientId": {
    message: "ID de cliente",
    description:
      "API Keys table column header: the key's id, which is also its OAuth2 client_id",
  },
  "apiKeys.colCreated": {
    message: "Creada",
    description: "API Keys table column header",
  },
  "apiKeys.colCreatedBy": {
    message: "Creada por",
    description: "API Keys table column header — who minted the key",
  },
  "apiKeys.colLastUsed": {
    message: "Último uso",
    description: "API Keys table column header: when the key was last used",
  },
  "apiKeys.neverUsed": {
    message: "Nunca",
    description: "API Keys last-used cell when the key has never been used",
  },
  "apiKeys.emptyTitle": {
    message: "No hay claves de API",
    description: "API Keys empty-state title",
  },
  "apiKeys.emptyBody": {
    message: "Crea una clave para autenticar un script o un agente.",
    description: "API Keys empty-state body",
  },
  "apiKeys.forbiddenTitle": {
    message: "No autorizado",
    description: "API Keys state when the caller lacks permission (403)",
  },
  "apiKeys.forbiddenBody": {
    message:
      "No tienes permiso para administrar las claves de API de este espacio de trabajo.",
    description: "API Keys forbidden-state body",
  },
  "apiKeys.errorTitle": {
    message: "No se pudieron cargar las claves de API",
    description: "API Keys generic error title",
  },
  "apiKeys.errorBody": {
    message: "Algo salió mal. Inténtalo de nuevo.",
    description: "API Keys generic error body",
  },
  "apiKeys.create": {
    message: "Crear clave de API",
    description: "Button that opens the mint dialog",
  },
  "apiKeys.createTitle": {
    message: "Crear clave de API",
    description: "Mint dialog title (name step)",
  },
  "apiKeys.createDescription": {
    message: "Ponle un nombre a esta clave para reconocerla más adelante.",
    description: "Mint dialog description (name step)",
  },
  "apiKeys.fieldName": {
    message: "Nombre",
    description: "Mint dialog name field label",
  },
  "apiKeys.fieldNamePlaceholder": {
    message: "p. ej. deploy-agent",
    description: "Mint dialog name field placeholder",
  },
  "apiKeys.createCancel": {
    message: "Cancelar",
    description: "Mint dialog cancel button (name step)",
  },
  "apiKeys.createSubmit": {
    message: "Crear",
    description: "Mint dialog submit button (name step)",
  },
  "apiKeys.createdTitle": {
    message: "Clave de API creada",
    description: "Mint dialog title (key-shown step)",
  },
  "apiKeys.createdWarning": {
    message: "Copia la clave ahora. No podrás volver a verla.",
    description:
      "Mint dialog warning (key-shown step): the key is shown only once",
  },
  "apiKeys.fieldKey": {
    message: "Clave de API",
    description:
      "Mint dialog label for the key itself, the one string sent as Authorization: Bearer",
  },
  "apiKeys.copyKey": {
    message: "Copiar clave de API",
    description: "Copy button aria-label for the key",
  },
  "apiKeys.keyHelp": {
    message:
      "Envíala como Authorization: Bearer en cada solicitud. La clave solo puede usar los permisos que elegiste para ella, nunca más de lo que permite tu rol de miembro actual en este espacio de trabajo, así que trátala como una contraseña.",
    description:
      "Mint dialog copy under the key: how to send it, and that it uses only the chosen permissions, capped by the creator's current member role in this workspace (w9/m496)",
  },
  "apiKeys.curlTitle": {
    message: "Solicitud de ejemplo",
    description:
      "Mint dialog label for the curl example that lists services with the key",
  },
  "apiKeys.copyCurl": {
    message: "Copiar solicitud de ejemplo",
    description: "Copy button aria-label for the curl example",
  },
  "apiKeys.oauthDetails": {
    message: "Credenciales de cliente OAuth",
    description:
      "Mint dialog title of the collapsed detail holding the client ID, the client secret and the optional token exchange",
  },
  "apiKeys.fieldClientId": {
    message: "ID de cliente",
    description:
      "Mint dialog label for the OAuth2 client_id, inside the OAuth client credentials detail",
  },
  "apiKeys.fieldSecret": {
    message: "Secreto de cliente",
    description:
      "Mint dialog label for the one-time client_secret, inside the OAuth client credentials detail",
  },
  "apiKeys.copyClientId": {
    message: "Copiar ID de cliente",
    description: "Copy button aria-label for the client_id",
  },
  "apiKeys.copyClientIdNamed": {
    message: "Copiar ID de cliente de {name}",
    description: "Copy button aria-label for one API client row's client_id",
  },
  "apiKeys.copySecret": {
    message: "Copiar secreto de cliente",
    description: "Copy button aria-label for the client_secret",
  },
  "apiKeys.exchangeHelp": {
    message:
      "Esta clave también funciona como cliente OAuth2. Si prefieres tokens de corta duración, cambia el ID de cliente y el secreto de cliente por un token de acceso en {tokenEndpoint}:\n\ngrant_type=client_credentials&client_id=…&client_secret=…\n\nNo envíes un scope. Usa el access_token devuelto como Authorization: Bearer en lugar de la clave.",
    description:
      "Mint dialog copy inside the OAuth client credentials detail: the optional client_credentials exchange (token endpoint from config)",
  },
  "apiKeys.createdDone": {
    message: "Listo",
    description: "Mint dialog close button (key-shown step)",
  },
  "apiKeys.copied": {
    message: "Copiado al portapapeles",
    description:
      "Toast on a successful copy (the key, the example request, the client ID or the secret)",
  },
  "apiKeys.copyError": {
    message: "No se pudo copiar al portapapeles",
    description: "Toast on a failed copy",
  },
  "apiKeys.createSuccess": {
    message: "{name} creada",
    description: "Toast on a successful mint",
  },
  "apiKeys.createError": {
    message: "No se pudo crear {name}",
    description: "Toast on a failed mint",
  },
  "apiKeys.revoke": {
    message: "Revocar",
    description: "Row action / confirmation button to revoke a key",
  },
  "apiKeys.revokeNamed": {
    message: "Revocar la clave de API {name}",
    description: "Row revoke button aria-label identifying the API key",
  },
  "apiKeys.revokeConfirmTitle": {
    message: "¿Revocar {name}?",
    description: "Revoke-confirmation dialog title",
  },
  "apiKeys.revokeConfirmBody": {
    message:
      "Todo lo que se autentique con esta clave dejará de funcionar de inmediato. Esta acción no se puede deshacer.",
    description: "Revoke-confirmation dialog body",
  },
  "apiKeys.revokeConfirmBodyOther": {
    message:
      "{creator} creó esta clave. Todo lo que se autentique con ella dejará de funcionar de inmediato. Esta acción no se puede deshacer.",
    description:
      "Revoke-confirmation dialog body for another member's key in an Admin's every-member view; {creator} is who created it",
  },
  "apiKeys.revokeCancel": {
    message: "Cancelar",
    description: "Revoke-confirmation dialog cancel button",
  },
  "apiKeys.revokeSuccess": {
    message: "{name} revocada",
    description: "Toast on a successful revoke",
  },
  "apiKeys.revokeError": {
    message: "No se pudo revocar {name}",
    description: "Toast on a failed revoke",
  },
  "apiKeys.colPermissions": {
    message: "Permisos",
    description: "API Keys table column header: what each key may do (w9/m496)",
  },
  "apiKeys.permSummaryAll": {
    message: "Todos",
    description:
      "Key row permissions summary: the key has every permission its creator's member role allows (Render's All)",
  },
  "apiKeys.permSummaryReadOnly": {
    message: "Solo lectura",
    description:
      "Key row permissions summary: every permission the key has is at Read, so it can change nothing",
  },
  "apiKeys.permSummaryNone": {
    message: "Sin permisos",
    description:
      "Key row permissions summary: the key has no permissions (Clear all)",
  },
  "apiKeys.permSummaryCustom": {
    message: "Personalizados",
    description:
      "Key row permissions summary: the key has a chosen set of permissions; followed by their count",
  },
  "apiKeys.editPermissionsNamed": {
    message: "Cambiar los permisos de {name}",
    description:
      "Gear button aria-label on a key row; opens the permissions editor",
  },
  "apiKeys.editPermissionsTitle": {
    message: "Editar permisos",
    description: "Permissions editor dialog title",
  },
  "apiKeys.editPermissionsDescription": {
    message:
      "Cambia lo que puede usar {name} y haz clic en Guardar cambios. Los nuevos permisos se aplican de inmediato.",
    description:
      "Permissions editor dialog description; {name} is the key's name",
  },
  "apiKeys.editPermissionsCancel": {
    message: "Cancelar",
    description: "Permissions editor dialog cancel button",
  },
  "apiKeys.editPermissionsSave": {
    message: "Guardar cambios",
    description: "Permissions editor dialog save button (Render's words)",
  },
  "apiKeys.permissionsSaved": {
    message: "Se actualizaron los permisos de {name}",
    description: "Toast after a key's permissions are saved",
  },
  "apiKeys.permissionsSaveError": {
    message: "No se pudieron actualizar los permisos de {name}",
    description:
      "Toast when saving a key's permissions fails (the server's reason is shown instead when it gives one)",
  },
  "apiKeys.permIntro": {
    message: "Selecciona los permisos que puede usar esta clave de API.",
    description:
      "Permission picker heading in the create dialog and the permissions editor (Render's words)",
  },
  "apiKeys.permPresetAll": {
    message: "Todos",
    description:
      "Picker preset button: grant every permission the member role allows (Render's All)",
  },
  "apiKeys.permPresetReadOnly": {
    message: "Solo lectura",
    description:
      "Picker preset button: grant Read on every permission that offers it (Render's Read only)",
  },
  "apiKeys.permPresetClear": {
    message: "Quitar todos",
    description:
      "Picker preset button: remove every permission from the key (Render's Clear all)",
  },
  "apiKeys.permAllHelp": {
    message:
      "Todos concede cada permiso que permite el rol de miembro de quien creó la clave, para cada recurso, y sigue ese rol si cambia.",
    description: "Picker note while the All preset is selected: what All means",
  },
  "apiKeys.permNoneHelp": {
    message:
      "No hay permisos seleccionados. La clave solo puede identificarse a sí misma.",
    description: "Picker note when every permission is None (Clear all)",
  },
  "apiKeys.permAreaToggle": {
    message: "Conceder permisos de {area}",
    description:
      "Switch aria-label on one area of the picker; {area} is Render's area name, e.g. Services (kept in English)",
  },
  "apiKeys.permLevelNone": {
    message: "Ninguno",
    description: "Picker option on a permission: the key gets none of it",
  },
  "apiKeys.permLevelGrants": {
    message: "{level} concede:",
    description:
      "Picker label over the actions a permission's chosen level grants; {level} is Render's level name, e.g. Read & write (kept in English)",
  },
  "apiKeys.permRequiresRole.viewer": {
    message: "{level} requiere el rol Lector.",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Viewer",
  },
  "apiKeys.permRequiresRole.contributor": {
    message: "{level} requiere el rol Colaborador.",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Contributor",
  },
  "apiKeys.permRequiresRole.developer": {
    message: "{level} requiere el rol Desarrollador.",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Developer",
  },
  "apiKeys.permRequiresRole.admin": {
    message: "{level} requiere el rol Administrador.",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Admin",
  },
  "apiKeys.permRequiresHigherRole": {
    message: "{level} requiere un rol de miembro superior.",
    description:
      "Reason beside a level the viewer's role cannot grant, when bex-api names a role the dashboard does not know",
  },
  "apiKeys.permCatalogLoading": {
    message: "Cargando permisos…",
    description:
      "Accessible name of the picker while its permission list loads",
  },
  "apiKeys.permCatalogErrorCreate": {
    message:
      "No se pudo cargar la lista de permisos. Una clave que crees ahora recibirá todos los permisos.",
    description:
      "Create dialog note when the permission list fails to load: creating still works and grants All",
  },
  "apiKeys.permCatalogErrorEdit": {
    message: "No se pudo cargar la lista de permisos.",
    description:
      "Permissions editor note when the permission list fails to load, beside a Try again button",
  },
  "apiKeys.permSummaryCount_one": {
    message: "{count} permiso",
    description:
      "Key row permissions summary beside Custom: how many permissions the key has",
  },
  "apiKeys.permSummaryCount_many": {
    message: "{count} de permisos",
    description:
      "Key row permissions summary beside Custom: how many permissions the key has",
  },
  "apiKeys.permSummaryCount_other": {
    message: "{count} permisos",
    description:
      "Key row permissions summary beside Custom: how many permissions the key has",
  },
};

export default esApiKeys;
