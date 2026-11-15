import type { TranslationEntry } from "@/i18n";

const ptRegistryCredentials: Record<string, TranslationEntry> = {
  "registryCredentials.title": {
    message: "Credenciais de registro",
    description:
      "Settings Integrations → Registry Credentials section card title",
  },
  "registryCredentials.description": {
    message:
      "Credenciais para registros de imagens externos privados (Docker Hub, GHCR, GitLab Container Registry, ECR etc.), para que um serviço com imagem existente possa fazer pull deles.",
    description: "Settings Registry Credentials section card description",
  },
  "registryCredentials.colName": {
    message: "Nome",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colUsername": {
    message: "Usuário",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colStatus": {
    message: "Expiração",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colCreated": {
    message: "Criada em",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.emptyTitle": {
    message: "Nenhuma credencial de registro",
    description: "Registry Credentials empty-state title",
  },
  "registryCredentials.emptyBody": {
    message:
      "Adicione uma credencial para que um serviço possa fazer pull de uma imagem de um registro privado.",
    description: "Registry Credentials empty-state body",
  },
  "registryCredentials.forbiddenTitle": {
    message: "Não autorizado",
    description:
      "Registry Credentials state when the caller lacks permission (403)",
  },
  "registryCredentials.forbiddenBody": {
    message:
      "Você não tem permissão para gerenciar as credenciais de registro deste workspace.",
    description: "Registry Credentials forbidden-state body",
  },
  "registryCredentials.errorTitle": {
    message: "Não foi possível carregar as credenciais de registro",
    description: "Registry Credentials generic error title",
  },
  "registryCredentials.errorBody": {
    message: "Algo deu errado. Tente novamente.",
    description: "Registry Credentials generic error body",
  },
  "registryCredentials.create": {
    message: "Adicionar credencial",
    description: "Button that opens the create dialog",
  },
  "registryCredentials.createTitle": {
    message: "Adicionar credencial de registro",
    description: "Create dialog title",
  },
  "registryCredentials.createDescription": {
    message:
      "Armazenada com segurança. A senha/token nunca será exibida novamente depois disto.",
    description: "Create dialog description",
  },
  "registryCredentials.fieldHost": {
    message: "Host do registro",
    description: "Create dialog host field label",
  },
  "registryCredentials.fieldHostPlaceholder": {
    message: "ex.: ghcr.io, docker.io, registry.gitlab.com",
    description: "Create dialog host field placeholder",
  },
  "registryCredentials.fieldUsername": {
    message: "Usuário",
    description: "Create dialog username field label",
  },
  "registryCredentials.fieldAuthToken": {
    message: "Senha ou token de acesso",
    description: "Create dialog secret field label",
  },
  "registryCredentials.hostInvalid": {
    message:
      "Informe um host de registro como ghcr.io ou registry.example.com:5000, sem caminho.",
    description:
      "Inline error under the Registry host when it is not a bare host[:port] (w21/053)",
  },
  "registryCredentials.hostTooLong": {
    message: "No máximo 253 bytes",
    description: "Inline error under an over-long Registry host (w21/053)",
  },
  "registryCredentials.usernameControl": {
    message: "Não pode conter caracteres de controle",
    description:
      "Inline error under a registry username with a control character (w21/053)",
  },
  "registryCredentials.usernameTooLong": {
    message: "No máximo 253 bytes",
    description: "Inline error under an over-long registry username (w21/053)",
  },
  "registryCredentials.authTokenTooLong": {
    message: "No máximo 64 KiB",
    description:
      "Inline error under a registry password or token over the 64 KiB limit (w21/053)",
  },
  "registryCredentials.fieldName": {
    message: "Nome de exibição",
    description: "Create dialog optional display-name field label",
  },
  "registryCredentials.fieldNameOptional": {
    message: "(opcional — o padrão é o host)",
    description: "Create dialog display-name field's optional hint",
  },
  "registryCredentials.createCancel": {
    message: "Cancelar",
    description: "Create dialog cancel button",
  },
  "registryCredentials.createSubmit": {
    message: "Adicionar",
    description: "Create dialog submit button",
  },
  "registryCredentials.createSuccess": {
    message: "Credencial adicionada para {host}",
    description: "Toast on a successful create",
  },
  "registryCredentials.createError": {
    message: "Não foi possível adicionar a credencial para {host}",
    description: "Toast on a failed create",
  },
  "registryCredentials.delete": {
    message: "Excluir",
    description: "Row action / confirmation button to delete a credential",
  },
  "registryCredentials.deleteNamed": {
    message: "Excluir credencial de registro {name}",
    description:
      "Row delete button aria-label identifying the registry credential",
  },
  "registryCredentials.edit": {
    message: "Editar",
    description: "Row action to open the edit-credential dialog",
  },
  "registryCredentials.editTitle": {
    message: "Editar credencial de registro",
    description: "Edit-credential dialog title",
  },
  "registryCredentials.editDescription": {
    message:
      "Renomeie a credencial, altere o usuário ou faça a rotação do token. O host do registro não pode ser alterado.",
    description: "Edit-credential dialog description",
  },
  "registryCredentials.editSubmit": {
    message: "Salvar alterações",
    description: "Edit-credential dialog submit button",
  },
  "registryCredentials.fieldHostImmutable": {
    message: "O host do registro não pode ser alterado após a criação.",
    description: "Hint under the read-only host field in the edit dialog",
  },
  "registryCredentials.fieldAuthTokenKeep": {
    message: "Deixe em branco para manter o token atual",
    description: "Placeholder for the token field in the edit dialog",
  },
  "registryCredentials.fieldAuthTokenKeepHint": {
    message:
      "O token armazenado nunca é exibido. Informe um novo valor apenas para fazer a rotação.",
    description: "Hint under the token field in the edit dialog",
  },
  "registryCredentials.updateSuccess": {
    message: "Credencial de registro atualizada.",
    description: "Toast after an edit succeeds",
  },
  "registryCredentials.updateError": {
    message: "Não foi possível atualizar a credencial de registro.",
    description: "Toast after an edit fails",
  },
  "registryCredentials.deleteConfirmTitle": {
    message: "Excluir {name}?",
    description: "Delete-confirmation dialog title",
  },
  "registryCredentials.deleteConfirmBody": {
    message:
      "Só é possível excluir uma credencial que nenhum serviço usa — remova-a de todos os serviços primeiro, ou a exclusão será recusada. Isso não pode ser desfeito.",
    description: "Delete-confirmation dialog body",
  },
  "registryCredentials.deleteCancel": {
    message: "Cancelar",
    description: "Delete-confirmation dialog cancel button",
  },
  "registryCredentials.deleteSuccess": {
    message: "{name} excluída",
    description: "Toast on a successful delete",
  },
  "registryCredentials.deleteError": {
    message: "Não foi possível excluir {name}",
    description: "Toast on a failed delete",
  },
  "registryCredentials.expired": {
    message: "Expirada",
    description:
      "Expiry-status badge for a past-expiry credential (w2/m14/t007)",
  },
  "registryCredentials.expiringSoon": {
    message: "Expira em breve",
    description: "Expiry-status badge for a credential nearing expiry",
  },
  "registryCredentials.expiresOn": {
    message: "Expira em {date}",
    description:
      "Expiry-status text for an active credential with a future expiry",
  },
  "registryCredentials.neverExpires": {
    message: "Nunca expira",
    description: "Expiry-status text for a credential with no expiry set",
  },
};

export default ptRegistryCredentials;
