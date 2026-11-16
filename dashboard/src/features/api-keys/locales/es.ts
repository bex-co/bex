import type { TranslationEntry } from "@/i18n";

const esApiKeys: Record<string, TranslationEntry> = {
  "apiKeys.title": {
    message: "Claves de API",
    description: "Settings API Keys section card title",
  },
  "apiKeys.description": {
    message:
      "Credenciales de máquina para scripts y agentes. Se comparten en todo el espacio de trabajo: quien pueda administrar claves ve todas las claves aquí, no solo las suyas.",
    description: "Settings API Keys section card description",
  },
  "apiKeys.cliGuide": {
    message: "Configura la CLI.",
    description: "Link from the API Keys card to the bex CLI setup guide",
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
      "Envíala como Authorization: Bearer en cada solicitud. La clave actúa con plena autoridad en este espacio de trabajo, incluidas las cadenas de conexión, así que trátala como una contraseña.",
    description:
      "Mint dialog copy under the key: how to send it, and that it carries full authority in the workspace",
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
};

export default esApiKeys;
