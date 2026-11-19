import type { TranslationEntry } from "@/i18n";

const ptApiKeys: Record<string, TranslationEntry> = {
  "apiKeys.title": {
    message: "Chaves de API",
    description: "Settings API Keys section card title",
  },
  "apiKeys.description": {
    message:
      "Credenciais de máquina para scripts e agentes. Cada chave só pode usar as permissões escolhidas para ela, nunca mais do que o papel de membro atual de quem a criou permite, e apenas neste workspace. Cada membro vê e gerencia as chaves que criou; Developers e Admins veem todas as chaves do workspace.",
    description:
      "Settings API Keys section card description: what a key is, that it uses only its chosen permissions within its creator's current role in this workspace (w9/m496), and who sees which keys: every member their own, Developers and Admins every key (w9/m514)",
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
      "Envie-a como Authorization: Bearer em cada requisição. A chave só pode usar as permissões que você escolheu para ela, nunca mais do que o seu papel de membro atual neste workspace permite, então trate-a como uma senha.",
    description:
      "Mint dialog copy under the key: how to send it, and that it uses only the chosen permissions, capped by the creator's current member role in this workspace (w9/m496)",
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
  "apiKeys.colPermissions": {
    message: "Permissões",
    description: "API Keys table column header: what each key may do (w9/m496)",
  },
  "apiKeys.permSummaryAll": {
    message: "Todas",
    description:
      "Key row permissions summary: the key has every permission its creator's member role allows (Render's All)",
  },
  "apiKeys.permSummaryReadOnly": {
    message: "Somente leitura",
    description:
      "Key row permissions summary: every permission the key has is at Read, so it can change nothing",
  },
  "apiKeys.permSummaryNone": {
    message: "Sem permissões",
    description:
      "Key row permissions summary: the key has no permissions (Clear all)",
  },
  "apiKeys.permSummaryCustom": {
    message: "Personalizadas",
    description:
      "Key row permissions summary: the key has a chosen set of permissions; followed by their count",
  },
  "apiKeys.editPermissionsNamed": {
    message: "Alterar as permissões de {name}",
    description:
      "Gear button aria-label on a key row; opens the permissions editor",
  },
  "apiKeys.editPermissionsTitle": {
    message: "Editar permissões",
    description: "Permissions editor dialog title",
  },
  "apiKeys.editPermissionsDescription": {
    message:
      "Altere o que {name} pode usar e clique em Salvar alterações. As novas permissões valem na hora.",
    description:
      "Permissions editor dialog description; {name} is the key's name",
  },
  "apiKeys.editPermissionsCancel": {
    message: "Cancelar",
    description: "Permissions editor dialog cancel button",
  },
  "apiKeys.editPermissionsSave": {
    message: "Salvar alterações",
    description: "Permissions editor dialog save button (Render's words)",
  },
  "apiKeys.permissionsSaved": {
    message: "Permissões de {name} atualizadas",
    description: "Toast after a key's permissions are saved",
  },
  "apiKeys.permissionsSaveError": {
    message: "Não foi possível atualizar as permissões de {name}",
    description:
      "Toast when saving a key's permissions fails (the server's reason is shown instead when it gives one)",
  },
  "apiKeys.permIntro": {
    message: "Selecione as permissões que esta chave de API pode usar.",
    description:
      "Permission picker heading in the create dialog and the permissions editor (Render's words)",
  },
  "apiKeys.permPresetAll": {
    message: "Todas",
    description:
      "Picker preset button: grant every permission the member role allows (Render's All)",
  },
  "apiKeys.permPresetReadOnly": {
    message: "Somente leitura",
    description:
      "Picker preset button: grant Read on every permission that offers it (Render's Read only)",
  },
  "apiKeys.permPresetClear": {
    message: "Limpar tudo",
    description:
      "Picker preset button: remove every permission from the key (Render's Clear all)",
  },
  "apiKeys.permAllHelp": {
    message:
      "Todas concede cada permissão que o papel de membro de quem criou a chave permite, para todos os recursos, e acompanha esse papel se ele mudar.",
    description: "Picker note while the All preset is selected: what All means",
  },
  "apiKeys.permNoneHelp": {
    message: "Nenhuma permissão selecionada. A chave só pode se identificar.",
    description: "Picker note when every permission is None (Clear all)",
  },
  "apiKeys.permAreaToggle": {
    message: "Conceder permissões de {area}",
    description:
      "Switch aria-label on one area of the picker; {area} is Render's area name, e.g. Services (kept in English)",
  },
  "apiKeys.permLevelNone": {
    message: "Nenhuma",
    description: "Picker option on a permission: the key gets none of it",
  },
  "apiKeys.permLevelGrants": {
    message: "{level} concede:",
    description:
      "Picker label over the actions a permission's chosen level grants; {level} is Render's level name, e.g. Read & write (kept in English)",
  },
  "apiKeys.permRequiresRole.viewer": {
    message: "{level} exige o papel Visualizador.",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Viewer",
  },
  "apiKeys.permRequiresRole.contributor": {
    message: "{level} exige o papel Colaborador.",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Contributor",
  },
  "apiKeys.permRequiresRole.developer": {
    message: "{level} exige o papel Desenvolvedor.",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Developer",
  },
  "apiKeys.permRequiresRole.admin": {
    message: "{level} exige o papel Administrador.",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Admin",
  },
  "apiKeys.permRequiresHigherRole": {
    message: "{level} exige um papel de membro superior.",
    description:
      "Reason beside a level the viewer's role cannot grant, when bex-api names a role the dashboard does not know",
  },
  "apiKeys.permCatalogLoading": {
    message: "Carregando permissões…",
    description:
      "Accessible name of the picker while its permission list loads",
  },
  "apiKeys.permCatalogErrorCreate": {
    message:
      "Não foi possível carregar a lista de permissões. Uma chave criada agora recebe todas as permissões.",
    description:
      "Create dialog note when the permission list fails to load: creating still works and grants All",
  },
  "apiKeys.permCatalogErrorEdit": {
    message: "Não foi possível carregar a lista de permissões.",
    description:
      "Permissions editor note when the permission list fails to load, beside a Try again button",
  },
  "apiKeys.permSummaryCount_one": {
    message: "{count} permissão",
    description:
      "Key row permissions summary beside Custom: how many permissions the key has",
  },
  "apiKeys.permSummaryCount_many": {
    message: "{count} de permissões",
    description:
      "Key row permissions summary beside Custom: how many permissions the key has",
  },
  "apiKeys.permSummaryCount_other": {
    message: "{count} permissões",
    description:
      "Key row permissions summary beside Custom: how many permissions the key has",
  },
};

export default ptApiKeys;
