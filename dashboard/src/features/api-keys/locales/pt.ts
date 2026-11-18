import type { TranslationEntry } from "@/i18n";

const ptApiKeys: Record<string, TranslationEntry> = {
  "apiKeys.title": {
    message: "Chaves de API",
    description: "Settings API Keys section card title",
  },
  "apiKeys.description": {
    message:
      "Credenciais de máquina para scripts e agentes. Cada chave atua com o papel de membro atual de quem a criou, apenas neste workspace. Compartilhadas em todo o workspace — quem pode gerenciar chaves vê todas as chaves aqui, não apenas as próprias.",
    description:
      "Settings API Keys section card description: what a key is, that it acts with its creator's current role in this workspace, and that the list is shared",
  },
  "apiKeys.cliGuide": {
    message: "Configure a CLI.",
    description: "Link from the API Keys card to the bex CLI setup guide",
  },
  "apiKeys.colName": {
    message: "Nome",
    description: "API Keys table column header",
  },
  "apiKeys.colClientId": {
    message: "ID do cliente",
    description:
      "API Keys table column header: the key's id, which is also its OAuth2 client_id",
  },
  "apiKeys.colCreated": {
    message: "Criada",
    description: "API Keys table column header",
  },
  "apiKeys.colCreatedBy": {
    message: "Criada por",
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
    message: "Nenhuma chave de API",
    description: "API Keys empty-state title",
  },
  "apiKeys.emptyBody": {
    message: "Crie uma chave para autenticar um script ou agente.",
    description: "API Keys empty-state body",
  },
  "apiKeys.forbiddenTitle": {
    message: "Não autorizado",
    description: "API Keys state when the caller lacks permission (403)",
  },
  "apiKeys.forbiddenBody": {
    message:
      "Você não tem permissão para gerenciar as chaves de API deste workspace.",
    description: "API Keys forbidden-state body",
  },
  "apiKeys.errorTitle": {
    message: "Não foi possível carregar as chaves de API",
    description: "API Keys generic error title",
  },
  "apiKeys.errorBody": {
    message: "Algo deu errado. Tente novamente.",
    description: "API Keys generic error body",
  },
  "apiKeys.create": {
    message: "Criar chave de API",
    description: "Button that opens the mint dialog",
  },
  "apiKeys.createTitle": {
    message: "Criar chave de API",
    description: "Mint dialog title (name step)",
  },
  "apiKeys.createDescription": {
    message: "Dê um nome a esta chave para reconhecê-la depois.",
    description: "Mint dialog description (name step)",
  },
  "apiKeys.fieldName": {
    message: "Nome",
    description: "Mint dialog name field label",
  },
  "apiKeys.fieldNamePlaceholder": {
    message: "ex.: deploy-agent",
    description: "Mint dialog name field placeholder",
  },
  "apiKeys.createCancel": {
    message: "Cancelar",
    description: "Mint dialog cancel button (name step)",
  },
  "apiKeys.createSubmit": {
    message: "Criar",
    description: "Mint dialog submit button (name step)",
  },
  "apiKeys.createdTitle": {
    message: "Chave de API criada",
    description: "Mint dialog title (key-shown step)",
  },
  "apiKeys.createdWarning": {
    message: "Copie a chave agora. Você não poderá vê-la novamente.",
    description:
      "Mint dialog warning (key-shown step): the key is shown only once",
  },
  "apiKeys.fieldKey": {
    message: "Chave de API",
    description:
      "Mint dialog label for the key itself, the one string sent as Authorization: Bearer",
  },
  "apiKeys.copyKey": {
    message: "Copiar chave de API",
    description: "Copy button aria-label for the key",
  },
  "apiKeys.keyHelp": {
    message:
      "Envie-a como Authorization: Bearer em cada requisição. A chave atua com o seu papel de membro atual neste workspace e o acompanha se o seu papel mudar, então trate-a como uma senha.",
    description:
      "Mint dialog copy under the key: how to send it, and that it acts with the creator's current member role in this workspace and follows it",
  },
  "apiKeys.curlTitle": {
    message: "Exemplo de requisição",
    description:
      "Mint dialog label for the curl example that lists services with the key",
  },
  "apiKeys.copyCurl": {
    message: "Copiar exemplo de requisição",
    description: "Copy button aria-label for the curl example",
  },
  "apiKeys.oauthDetails": {
    message: "Credenciais de cliente OAuth",
    description:
      "Mint dialog title of the collapsed detail holding the client ID, the client secret and the optional token exchange",
  },
  "apiKeys.fieldClientId": {
    message: "ID do cliente",
    description:
      "Mint dialog label for the OAuth2 client_id, inside the OAuth client credentials detail",
  },
  "apiKeys.fieldSecret": {
    message: "Segredo do cliente",
    description:
      "Mint dialog label for the one-time client_secret, inside the OAuth client credentials detail",
  },
  "apiKeys.copyClientId": {
    message: "Copiar ID do cliente",
    description: "Copy button aria-label for the client_id",
  },
  "apiKeys.copyClientIdNamed": {
    message: "Copiar ID do cliente de {name}",
    description: "Copy button aria-label for one API client row's client_id",
  },
  "apiKeys.copySecret": {
    message: "Copiar segredo do cliente",
    description: "Copy button aria-label for the client_secret",
  },
  "apiKeys.exchangeHelp": {
    message:
      "Esta chave também funciona como um cliente OAuth2. Se você preferir tokens de curta duração, troque o ID do cliente e o segredo do cliente por um token de acesso em {tokenEndpoint}:\n\ngrant_type=client_credentials&client_id=…&client_secret=…\n\nNão envie um scope. Use o access_token retornado como Authorization: Bearer no lugar da chave.",
    description:
      "Mint dialog copy inside the OAuth client credentials detail: the optional client_credentials exchange (token endpoint from config)",
  },
  "apiKeys.createdDone": {
    message: "Concluir",
    description: "Mint dialog close button (key-shown step)",
  },
  "apiKeys.copied": {
    message: "Copiado para a área de transferência",
    description:
      "Toast on a successful copy (the key, the example request, the client ID or the secret)",
  },
  "apiKeys.copyError": {
    message: "Não foi possível copiar para a área de transferência",
    description: "Toast on a failed copy",
  },
  "apiKeys.createSuccess": {
    message: "{name} criada",
    description: "Toast on a successful mint",
  },
  "apiKeys.createError": {
    message: "Não foi possível criar {name}",
    description: "Toast on a failed mint",
  },
  "apiKeys.revoke": {
    message: "Revogar",
    description: "Row action / confirmation button to revoke a key",
  },
  "apiKeys.revokeNamed": {
    message: "Revogar a chave de API {name}",
    description: "Row revoke button aria-label identifying the API key",
  },
  "apiKeys.revokeConfirmTitle": {
    message: "Revogar {name}?",
    description: "Revoke-confirmation dialog title",
  },
  "apiKeys.revokeConfirmBody": {
    message:
      "Tudo que se autentica com esta chave deixará de funcionar imediatamente. Esta ação não pode ser desfeita.",
    description: "Revoke-confirmation dialog body",
  },
  "apiKeys.revokeCancel": {
    message: "Cancelar",
    description: "Revoke-confirmation dialog cancel button",
  },
  "apiKeys.revokeSuccess": {
    message: "{name} revogada",
    description: "Toast on a successful revoke",
  },
  "apiKeys.revokeError": {
    message: "Não foi possível revogar {name}",
    description: "Toast on a failed revoke",
  },
};

export default ptApiKeys;
