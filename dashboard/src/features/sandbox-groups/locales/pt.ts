import type { TranslationEntry } from "@/i18n";

const ptSandboxGroups: Record<string, TranslationEntry> = {
  "sandboxGroups.createTitle": {
    message: "Novo grupo de sandbox",
    description:
      "Title of the new-sandbox-group dialog and its entry in the + New menus",
  },
  "sandboxGroups.createDescription": {
    message:
      "Um grupo de sandbox limita quantas de suas sandboxes podem estar ativas ao mesmo tempo. Os nomes dos grupos devem ser exclusivos no seu workspace.",
    description: "New-sandbox-group dialog description",
  },
  "sandboxGroups.fieldName": {
    message: "Nome",
    description: "New-sandbox-group dialog name field label",
  },
  "sandboxGroups.fieldNamePlaceholder": {
    message: "my-sandbox-group",
    description: "Placeholder for the new sandbox group's name",
  },
  "sandboxGroups.fieldRegion": {
    message: "Região",
    description:
      "Label of the region row in the created sandbox group's confirmation",
  },
  "sandboxGroups.createCancel": {
    message: "Cancelar",
    description: "Cancel button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createSubmit": {
    message: "Criar grupo de sandbox",
    description: "Submit button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createError": {
    message:
      "Não foi possível criar o grupo de sandbox {name}. Tente novamente.",
    description: "Toast shown when creating a sandbox group fails",
  },
  "sandboxGroups.errorNameTaken": {
    message:
      "Já existe um grupo de sandbox chamado “{name}” neste workspace. Escolha outro nome.",
    description:
      "Inline error under the name field when the group name is already taken",
  },
  "sandboxGroups.errorNameInvalid": {
    message:
      "O nome do grupo deve ter de 1 a {max} caracteres, sem espaços no início ou no fim e sem caracteres de controle.",
    description: "Inline error under the name field when the name is refused",
  },
  "sandboxGroups.errorLimitReached_one": {
    message: "Este workspace já tem o máximo de {count} grupo de sandbox.",
    description:
      "Inline alert when the workspace is at its sandbox group limit (singular)",
  },
  "sandboxGroups.errorLimitReached_many": {
    message: "Este workspace já tem o máximo de {count} de grupos de sandbox.",
    description:
      "Inline alert when the workspace is at its sandbox group limit",
  },
  "sandboxGroups.errorLimitReached_other": {
    message: "Este workspace já tem o máximo de {count} grupos de sandbox.",
    description:
      "Inline alert when the workspace is at its sandbox group limit",
  },
  "sandboxGroups.errorLimitReachedGeneric": {
    message: "Este workspace atingiu o limite de grupos de sandbox.",
    description:
      "Inline alert for the sandbox group limit when the server names no number",
  },
  "sandboxGroups.errorLimitTitle": {
    message: "Limite de grupos de sandbox atingido",
    description: "Heading of the inline sandbox-group-limit alert",
  },
  "sandboxGroups.createdTitle": {
    message: "Grupo de sandbox criado",
    description: "Dialog title once a sandbox group has been created",
  },
  "sandboxGroups.createdDescription": {
    message:
      "{name} está pronto. Passe o ID dele como sandboxGroupId ao criar uma sandbox nele.",
    description: "Dialog description once a sandbox group has been created",
  },
  "sandboxGroups.createdDone": {
    message: "Concluir",
    description: "Button that closes the created-sandbox-group confirmation",
  },
  "sandboxGroups.fieldId": {
    message: "ID",
    description: "Label of the new sandbox group's ID in the confirmation",
  },
  "sandboxGroups.fieldDefault": {
    message: "Grupo padrão",
    description:
      "Label of the confirmation row that says whether the group is the workspace's default",
  },
  "sandboxGroups.defaultYes": {
    message: "Sim",
    description: "The created group is the workspace's default sandbox group",
  },
  "sandboxGroups.defaultNo": {
    message: "Não",
    description:
      "The created group is not the workspace's default sandbox group",
  },
  "sandboxGroups.fieldConcurrencyLimit": {
    message: "Limite de concorrência",
    description:
      "Label of the confirmation row with the most active sandboxes the group allows",
  },
  "sandboxGroups.copyId": {
    message: "Copiar ID do grupo de sandbox",
    description: "Accessible label of the copy button next to the group's ID",
  },
  "sandboxGroups.copied": {
    message: "ID do grupo de sandbox copiado",
    description: "Toast after copying the sandbox group ID",
  },
  "sandboxGroups.copyError": {
    message: "Não foi possível copiar o ID do grupo de sandbox",
    description: "Toast when copying the sandbox group ID fails",
  },
  "sandboxGroups.environmentLoading": {
    message: "Carregando ambientes",
    description:
      "Accessible name of the loading placeholder shown where the new-sandbox-group dialog's Environment select will appear",
  },
  "sandboxGroups.environmentsError": {
    message:
      "Não foi possível carregar seus ambientes. Você ainda pode criar o grupo sem um ambiente.",
    description:
      "Shown in the new-sandbox-group dialog when the workspace's environments could not be read; the group can still be created without one",
  },
  "sandboxGroups.errorEnvironmentGone": {
    message: "Esse ambiente não existe mais. Escolha outro ou nenhum.",
    description:
      "Inline error under the Environment select when the chosen environment was deleted before the group was created",
  },
  "sandboxGroups.pageTitle": {
    message: "Grupos de sandbox",
    description: "Heading for the workspace sandbox groups page",
  },
  "sandboxGroups.pageDescription": {
    message:
      "Gerencie os limites de concorrência e as credenciais de saída intermediadas de cada grupo.",
    description: "Subtitle for the workspace sandbox groups page",
  },
  "sandboxGroups.resourceType": {
    message: "Grupo de sandbox",
    description: "Singular resource type name for a sandbox group",
  },
  "sandboxGroups.backToList": {
    message: "Voltar para grupos de sandbox",
    description: "Accessible label of the back link on a sandbox group detail page",
  },
  "sandboxGroups.defaultBadge": {
    message: "Padrão",
    description: "Badge indicating the workspace default sandbox group",
  },
  "sandboxGroups.statusReady": {
    message: "Pronto",
    description:
      "Status of a sandbox group row in a project environment's resource table; Render shows every group as Ready",
  },
  "sandboxGroups.environmentUpdated": {
    message: "Grupos de sandbox de “{name}” atualizados.",
    description:
      "Toast after sandbox groups were moved into or out of an environment; {name} is the environment",
  },
  "sandboxGroups.environmentUpdateError": {
    message:
      "Não foi possível mover o grupo de sandbox {group}. Tente novamente.",
    description:
      "Toast when moving a sandbox group into or out of an environment fails and the server gave no reason to show",
  },
  "sandboxGroups.listLoadError": {
    message: "Não foi possível carregar os grupos de sandbox. Tente novamente.",
    description: "Error state when loading sandbox groups fails",
  },
  "sandboxGroups.listEmptyTitle": {
    message: "Nenhum grupo de sandbox ainda",
    description: "Empty state heading when the workspace has no sandbox groups",
  },
  "sandboxGroups.listEmptyDescription": {
    message:
      "Crie um grupo de sandbox para definir limites de concorrência e configurar credenciais de saída.",
    description: "Empty state body when the workspace has no sandbox groups",
  },
  "sandboxGroups.manageCredentialsButton": {
    message: "Credenciais",
    description: "Button on each sandbox group row linking to its detail and credentials",
  },
  "sandboxGroups.detailLoadError": {
    message: "Não foi possível carregar este grupo de sandbox. Tente novamente.",
    description: "Error state when loading a single sandbox group fails",
  },
  "sandboxGroups.credSectionTitle": {
    message: "Credenciais de saída",
    description: "Card title for the sandbox group credentials panel",
  },
  "sandboxGroups.credSectionDescription": {
    message:
      "Segredos intermediados injetados em solicitações HTTPS de saída correspondentes dos sandboxes deste grupo. Os valores são somente gravação e nunca são exibidos novamente.",
    description: "Card subtitle for the sandbox group credentials panel",
  },
  "sandboxGroups.credAddButton": {
    message: "Adicionar credencial",
    description: "Button to open the create sandbox group credential dialog",
  },
  "sandboxGroups.credLoadError": {
    message: "Não foi possível carregar as credenciais do grupo. Tente novamente.",
    description: "Error state when fetching sandbox group credentials fails",
  },
  "sandboxGroups.credEmptyTitle": {
    message: "Nenhuma credencial configurada",
    description: "Empty state heading when a sandbox group has no credentials",
  },
  "sandboxGroups.credEmptyDescriptionAdmin": {
    message:
      "Adicione uma credencial para injetar autenticação em solicitações de saída sem expor o segredo ao código do sandbox.",
    description: "Empty state body for admins when no credentials exist",
  },
  "sandboxGroups.credEmptyDescriptionReadOnly": {
    message:
      "Um administrador do espaço de trabalho pode adicionar credenciais de saída para este grupo.",
    description: "Empty state body for non-admins when no credentials exist",
  },
  "sandboxGroups.credColId": {
    message: "ID da credencial",
    description: "Table column header for credential ID",
  },
  "sandboxGroups.credColDomain": {
    message: "Domínio",
    description: "Table column header for destination domain",
  },
  "sandboxGroups.credColInjection": {
    message: "Injeção",
    description: "Table column header for injection method and target",
  },
  "sandboxGroups.credColRouting": {
    message: "Roteamento",
    description: "Table column header for HTTP methods and path filters",
  },
  "sandboxGroups.credColEnabled": {
    message: "Ativa",
    description: "Table column header for enabled toggle or status",
  },
  "sandboxGroups.credColActions": {
    message: "Ações",
    description: "Table column header for row actions",
  },
  "sandboxGroups.credAllMethods": {
    message: "Todos os métodos",
    description: "Badge shown when a credential matches all HTTP methods",
  },
  "sandboxGroups.credAllPaths": {
    message: "Todos os caminhos",
    description: "Badge shown when a credential matches all request paths",
  },
  "sandboxGroups.credStatusEnabled": {
    message: "Ativa",
    description: "Read-only badge when a credential is enabled",
  },
  "sandboxGroups.credStatusDisabled": {
    message: "Inativa",
    description: "Read-only badge when a credential is disabled",
  },
  "sandboxGroups.credToggleEnabledLabel": {
    message: "Alternar estado de {id}",
    description: "Accessible label for the enabled switch on a credential row",
  },
  "sandboxGroups.credVersionsActionLabel": {
    message: "Ver versões de {id}",
    description: "Accessible label for the version history icon button",
  },
  "sandboxGroups.credEditActionLabel": {
    message: "Editar credencial {id}",
    description: "Accessible label for the edit credential icon button",
  },
  "sandboxGroups.credDeleteActionLabel": {
    message: "Excluir credencial {id}",
    description: "Accessible label for the delete credential icon button",
  },
  "sandboxGroups.credVersionsButton": {
    message: "Versões",
    description: "Mobile button label to open credential version history",
  },
  "sandboxGroups.credEditButton": {
    message: "Editar",
    description: "Mobile button label to edit a credential",
  },
  "sandboxGroups.credDeleteButton": {
    message: "Excluir",
    description: "Button label to delete a credential",
  },
  "sandboxGroups.credCreateTitle": {
    message: "Adicionar credencial",
    description: "Dialog title when creating a sandbox group credential",
  },
  "sandboxGroups.credCreateDescription": {
    message: "Configure uma credencial de saída somente gravação para este grupo de sandbox.",
    description: "Dialog description when creating a sandbox group credential",
  },
  "sandboxGroups.credEditTitle": {
    message: "Editar {id}",
    description: "Dialog title when editing a sandbox group credential",
  },
  "sandboxGroups.credEditDescriptionAdmin": {
    message:
      "Atualize roteamento, injeção, domínio ou rotacione o valor secreto. Deixe em branco para manter o segredo atual.",
    description: "Dialog description when an admin edits a credential",
  },
  "sandboxGroups.credEditDescriptionDeveloper": {
    message:
      "Atualize injeção, métodos, caminhos ou estado ativo. Alterar o domínio ou o segredo exige permissão de administrador.",
    description: "Dialog description when a developer edits a credential",
  },
  "sandboxGroups.credFieldId": {
    message: "ID da credencial",
    description: "Form label for credential ID",
  },
  "sandboxGroups.credFieldIdHelp": {
    message: "Identificador sensível a maiúsculas com letras, números, ., _, ~ ou -. Não pode começar com `rnd-`.",
    description: "Help text for credential ID format",
  },
  "sandboxGroups.credFieldDomain": {
    message: "Domínio de destino",
    description: "Form label for destination hostname",
  },
  "sandboxGroups.credAdminOnlyDomainSecret": {
    message: "Somente administradores podem alterar o domínio ou o valor secreto.",
    description: "Helper text shown to developers on read-only domain field",
  },
  "sandboxGroups.credFieldValue": {
    message: "Valor secreto",
    description: "Form label for secret value on create",
  },
  "sandboxGroups.credFieldValuePlaceholder": {
    message: "Cole o valor secreto",
    description: "Placeholder for secret value input on create",
  },
  "sandboxGroups.credFieldReplacementValue": {
    message: "Novo valor secreto (opcional)",
    description: "Form label for rotating secret value on edit",
  },
  "sandboxGroups.credFieldReplacementValuePlaceholder": {
    message: "Deixe em branco para manter o segredo atual",
    description: "Placeholder for optional secret rotation input on edit",
  },
  "sandboxGroups.credFieldValueWriteOnlyHelp": {
    message: "Somente gravação (até 5 KiB). Nunca será exibido novamente após salvar.",
    description: "Help text explaining write-only secret handling",
  },
  "sandboxGroups.credFieldInjectionType": {
    message: "Tipo de injeção",
    description: "Form label for injection type select",
  },
  "sandboxGroups.credInjectionHeader": {
    message: "Cabeçalho HTTP",
    description: "Injection type option for HTTP header",
  },
  "sandboxGroups.credInjectionBasic": {
    message: "Autenticação básica",
    description: "Injection type option for HTTP Basic authentication",
  },
  "sandboxGroups.credInjectionCookie": {
    message: "Cookie HTTP",
    description: "Injection type option for HTTP cookie",
  },
  "sandboxGroups.credInjectionQuery": {
    message: "Parâmetro de consulta",
    description: "Injection type option for URL query parameter",
  },
  "sandboxGroups.credFieldInjectionName": {
    message: "Nome de destino",
    description: "Form label for header, cookie, or query parameter name",
  },
  "sandboxGroups.credBasicHelp": {
    message: "Usa o cabeçalho `Authorization` automaticamente. Use o formato `username:password`.",
    description: "Helper text shown when basic auth injection is selected",
  },
  "sandboxGroups.credFieldTemplate": {
    message: "Modelo de valor (opcional)",
    description: "Form label for optional value template",
  },
  "sandboxGroups.credFieldTemplateHelp": {
    message: "O padrão é {token}. Inclua de 1 a 5 ocorrências de {token}.",
    description: "Help text for injection template placeholder syntax",
  },
  "sandboxGroups.credFieldMethods": {
    message: "Métodos HTTP",
    description: "Form label for allowed HTTP methods multi-select",
  },
  "sandboxGroups.credFieldMethodsHelp": {
    message: "Deixe todos desmarcados para corresponder a qualquer método HTTP.",
    description: "Help text for HTTP methods selection",
  },
  "sandboxGroups.credFieldPaths": {
    message: "Padrões de caminho (um por linha)",
    description: "Form label for path patterns textarea",
  },
  "sandboxGroups.credFieldPathsHelp": {
    message: "Caminhos absolutos começando com /. Aceita curingas * e ** de segmento inteiro. Deixe vazio para todos os caminhos.",
    description: "Help text for path pattern syntax",
  },
  "sandboxGroups.credFieldEnabled": {
    message: "Ativa",
    description: "Form label for credential enabled switch",
  },
  "sandboxGroups.credFieldEnabledHelp": {
    message: "Credenciais inativas permanecem no grupo, mas nunca são injetadas.",
    description: "Help text for credential enabled switch",
  },
  "sandboxGroups.credCreateSubmit": {
    message: "Criar credencial",
    description: "Submit button label for creating a credential",
  },
  "sandboxGroups.credEditSubmit": {
    message: "Salvar alterações",
    description: "Submit button label for editing a credential",
  },
  "sandboxGroups.credCreatedToast": {
    message: "Credencial {id} criada.",
    description: "Success toast after creating a credential",
  },
  "sandboxGroups.credUpdatedToast": {
    message: "Credencial {id} atualizada.",
    description: "Success toast after updating a credential",
  },
  "sandboxGroups.credDeletedToast": {
    message: "Credencial {id} excluída.",
    description: "Success toast after deleting a credential",
  },
  "sandboxGroups.credRestoredToast": {
    message: "{id} restaurada a partir da versão v{version}.",
    description: "Success toast after restoring a credential version",
  },
  "sandboxGroups.credCreateError": {
    message: "Não foi possível criar a credencial. Verifique os campos.",
    description: "Fallback error when creating a credential fails",
  },
  "sandboxGroups.credUpdateError": {
    message: "Não foi possível atualizar a credencial. Tente novamente.",
    description: "Fallback error when updating a credential fails",
  },
  "sandboxGroups.credDeleteError": {
    message: "Não foi possível excluir a credencial. Tente novamente.",
    description: "Fallback error when deleting a credential fails",
  },
  "sandboxGroups.credRestoreError": {
    message: "Não foi possível restaurar a versão da credencial. Tente novamente.",
    description: "Fallback error when restoring a credential version fails",
  },
  "sandboxGroups.credErrorExists": {
    message: 'Já existe uma credencial com o ID "{id}" neste grupo.',
    description: "Inline error when creating a duplicate credential ID",
  },
  "sandboxGroups.credErrorBusy": {
    message: "Outra atualização de credenciais está em andamento neste grupo. Tente novamente em instantes.",
    description: "Error when concurrent credential writes contend on the group lock",
  },
  "sandboxGroups.credErrorUnavailable": {
    message: "O armazenamento de credenciais está indisponível no momento.",
    description: "Error when OpenBao credential storage is unconfigured or unreachable",
  },
  "sandboxGroups.credErrorVersionNotFound": {
    message: "Essa versão da credencial não está mais retida.",
    description: "Error when restoring an evicted or unknown version",
  },
  "sandboxGroups.credVersionsTitle": {
    message: "Histórico de versões · {id}",
    description: "Dialog title for credential version history",
  },
  "sandboxGroups.credVersionsDescription": {
    message: "Retém a configuração atual e até 3 configurações anteriores distintas mais recentes.",
    description: "Dialog subtitle explaining version retention policy",
  },
  "sandboxGroups.credVersionsError": {
    message: "Não foi possível carregar o histórico de versões. Tente novamente.",
    description: "Error state when loading credential versions fails",
  },
  "sandboxGroups.credVersionsEmpty": {
    message: "Nenhuma versão retida encontrada.",
    description: "Empty state for credential versions dialog",
  },
  "sandboxGroups.credVersionCurrent": {
    message: "Atual",
    description: "Badge marking the active credential version",
  },
  "sandboxGroups.credVersionRestoredFrom": {
    message: "Restaurada de v{version}",
    description: "Badge showing which prior version was restored",
  },
  "sandboxGroups.credFingerprintLabel": {
    message: "Impressão digital",
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
    message: "Restaurar {id} para v{version}?",
    description: "Confirmation dialog title when restoring a credential version",
  },
  "sandboxGroups.credRestoreConfirmDescription": {
    message: "Isso cria uma nova versão atual para {id} usando a configuração e o segredo de v{version}.",
    description: "Confirmation dialog body when restoring a credential version",
  },
  "sandboxGroups.credDeleteConfirmTitle": {
    message: "Excluir credencial {id}?",
    description: "Confirmation dialog title when deleting a credential",
  },
  "sandboxGroups.credDeleteConfirmDescription": {
    message: "Excluir {id} remove permanentemente o segredo armazenado e todas as versões retidas.",
    description: "Confirmation dialog body when deleting a credential",
  },
  "sandboxGroups.credValIdRequired": {
    message: "O ID da credencial é obrigatório.",
    description: "Validation error when credential ID is empty",
  },
  "sandboxGroups.credValIdInvalid": {
    message: "Use apenas letras, números, pontos, sublinhados, tis e hifens.",
    description: "Validation error when credential ID has disallowed characters",
  },
  "sandboxGroups.credValIdReserved": {
    message: 'Os IDs ".", ".." e valores iniciados por `rnd-` são reservados.',
    description: "Validation error when credential ID is reserved",
  },
  "sandboxGroups.credValDomainRequired": {
    message: "O domínio de destino é obrigatório.",
    description: "Validation error when domain is empty",
  },
  "sandboxGroups.credValDomainInvalid": {
    message: "Informe um nome DNS público válido sem curingas, portas ou caminhos.",
    description: "Validation error when domain format is invalid",
  },
  "sandboxGroups.credValDomainPrivate": {
    message: "Nomes de host locais, privados ou do cluster não são permitidos.",
    description: "Validation error when domain is private or cluster-local",
  },
  "sandboxGroups.credValValueRequired": {
    message: "O valor secreto é obrigatório.",
    description: "Validation error when secret value is empty on create",
  },
  "sandboxGroups.credValValueTooLong": {
    message: "O valor secreto deve ter no máximo {maxBytes} bytes.",
    description: "Validation error when secret value exceeds 5 KiB",
  },
  "sandboxGroups.credValValueNul": {
    message: "O valor secreto não pode conter bytes NUL.",
    description: "Validation error when secret value contains a NUL byte",
  },
  "sandboxGroups.credValInjectionNameRequired": {
    message: "O nome de destino é obrigatório para este tipo de injeção.",
    description: "Validation error when header/cookie/query name is missing",
  },
  "sandboxGroups.credValInjectionNameForbidden": {
    message: "A autenticação básica não pode especificar um nome de destino.",
    description: "Validation error when basic injection specifies a name",
  },
  "sandboxGroups.credValInjectionHeaderForbidden": {
    message: "Cabeçalhos de roteamento e conexão não podem ser injetados.",
    description: "Validation error when a forbidden HTTP header is chosen",
  },
  "sandboxGroups.credValInjectionNameInvalid": {
    message: "Informe um nome de destino válido.",
    description: "Validation error when target name has invalid characters",
  },
  "sandboxGroups.credValInjectionTemplateCount": {
    message: "O modelo deve conter entre 1 e {max} ocorrências de {token}.",
    description: "Validation error when template placeholder count is out of range",
  },
  "sandboxGroups.credValInjectionBasicFormat": {
    message: "O valor renderizado da autenticação básica deve ter o formato `username:password`.",
    description: "Validation error when basic auth rendered value lacks username:password",
  },
  "sandboxGroups.credValInjectionHeaderControl": {
    message: "O valor renderizado do cabeçalho não pode conter quebras de linha nem bytes NUL.",
    description: "Validation error when header value contains CR/LF/NUL",
  },
  "sandboxGroups.credValInjectionCookieInvalid": {
    message: "O valor renderizado do cookie não pode conter ponto e vírgula nem quebras de linha.",
    description: "Validation error when cookie value contains semicolons or CR/LF",
  },
  "sandboxGroups.credValPathsInvalid": {
    message: "Cada caminho deve começar com /, omitir ? ou # e usar curingas * ou ** de segmento inteiro.",
    description: "Validation error when a path pattern is invalid",
  },
};

export default ptSandboxGroups;
