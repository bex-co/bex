import type { TranslationEntry } from "@/i18n";

const ptApiKeys: Record<string, TranslationEntry> = {
  "apiKeys.title": {
    message: "Chaves de API",
    description: "Settings API Keys section card title",
  },
  "apiKeys.description": {
    message:
      "Credenciais de máquina para scripts e agentes. Compartilhadas em todo o workspace — quem pode gerenciar chaves vê todas as chaves aqui, não apenas as próprias.",
    description: "Settings API Keys section card description",
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
      "API Keys table column header — OAuth2 client_id paired with the secret",
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
    description:
      "API Keys table column header — when a token for the key was last used",
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
    description: "Mint dialog title (secret-shown step)",
  },
  "apiKeys.createdWarning": {
    message:
      "Copie o ID do cliente e o segredo agora — você não poderá ver o segredo novamente.",
    description: "Mint dialog warning (secret-shown step)",
  },
  "apiKeys.fieldClientId": {
    message: "ID do cliente",
    description: "Mint dialog label for the OAuth2 client_id",
  },
  "apiKeys.fieldSecret": {
    message: "Segredo do cliente",
    description: "Mint dialog label for the one-time client_secret",
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
      "Este é um cliente OAuth2 client_credentials, não um bearer token. Troque-o em {tokenEndpoint}:\n\ngrant_type=client_credentials&client_id=…&client_secret=…\n\nNão envie um scope: a chave atua com autoridade total neste workspace, inclusive sobre strings de conexão, então trate-a como uma senha. Use o access_token retornado como Authorization: Bearer.",
    description:
      "Mint dialog usage copy for the client_credentials exchange (token endpoint from config)",
  },
  "apiKeys.createdDone": {
    message: "Concluir",
    description: "Mint dialog close button (secret-shown step)",
  },
  "apiKeys.copy": {
    message: "Copiar segredo",
    description: "Copy-to-clipboard icon button label",
  },
  "apiKeys.copied": {
    message: "Copiado para a área de transferência",
    description: "Toast on a successful secret copy",
  },
  "apiKeys.copyError": {
    message: "Não foi possível copiar para a área de transferência",
    description: "Toast on a failed secret copy",
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
