import type { TranslationEntry } from "@/i18n";

const ptEnvGroups: Record<string, TranslationEntry> = {
  "envGroups.resourceType": {
    message: "Grupo de ambiente",
    description: "Environment Group resource type used in document titles",
  },
  "envGroups.pageTitle": {
    message: "Grupos de ambiente",
    description: "Workspace env-groups page title",
  },
  "envGroups.pageDescription": {
    message:
      "Compartilhe variáveis de ambiente e arquivos secretos entre serviços.",
    description: "Workspace env-groups page subtitle",
  },
  "envGroups.newButton": {
    message: "Novo grupo de ambiente",
    description: "Open the create env-group dialog",
  },
  "envGroups.createTitle": {
    message: "Criar grupo de ambiente",
    description: "Create env-group dialog title",
  },
  "envGroups.createDescription": {
    message:
      "Adicione variáveis, arquivos secretos e vínculos de serviços agora ou deixe tudo vazio.",
    description: "Create env-group dialog description",
  },
  "envGroups.createVarsTitle": {
    message: "Variáveis de ambiente",
    description: "Initial environment variables section title",
  },
  "envGroups.createVarsDescription": {
    message:
      "Defina valores literais ou gere segredos fortes durante a criação.",
    description: "Initial environment variables section description",
  },
  "envGroups.varKeyLabel": {
    message: "Chave",
    description: "Initial environment variable key label",
  },
  "envGroups.varKeyPlaceholder": {
    message: "API_TOKEN",
    description: "Initial environment variable key placeholder",
  },
  "envGroups.varValueLabel": {
    message: "Valor",
    description: "Initial environment variable value label",
  },
  "envGroups.varValuePlaceholder": {
    message: "Valor secreto",
    description: "Initial environment variable value placeholder",
  },
  "envGroups.generateValue": {
    message: "Gerar",
    description: "Generate an initial environment variable value",
  },
  "envGroups.addInitialVar": {
    message: "Adicionar variável de ambiente",
    description: "Add an initial environment variable row",
  },
  "envGroups.importEnv": {
    message: "Importar de .env",
    description:
      "Import initial environment variables from dotenv text or file",
  },
  "envGroups.createFilesTitle": {
    message: "Arquivos secretos",
    description: "Initial secret files section title",
  },
  "envGroups.createFilesDescription": {
    message: "Os arquivos são criptografados e montados em /etc/secrets.",
    description: "Initial secret files section description",
  },
  "envGroups.fileNameLabel": {
    message: "Nome do arquivo",
    description: "Initial secret file name label",
  },
  "envGroups.fileNamePlaceholder": {
    message: "credentials.json",
    description: "Initial secret file name placeholder",
  },
  "envGroups.fileContentLabel": {
    message: "Conteúdo",
    description: "Initial secret file contents label",
  },
  "envGroups.fileContentPlaceholder": {
    message: "Conteúdo do arquivo",
    description: "Initial secret file contents placeholder",
  },
  "envGroups.addInitialFile": {
    message: "Adicionar arquivo secreto",
    description: "Add an initial secret file row",
  },
  "envGroups.createServicesTitle": {
    message: "Serviços vinculados",
    description: "Initial service links section title",
  },
  "envGroups.createServicesDescription": {
    message:
      "Os serviços selecionados recebem este grupo assim que ele é criado.",
    description: "Initial service links section description",
  },
  "envGroups.servicesLoading": {
    message: "Carregando serviços do workspace…",
    description: "Initial service links loading state",
  },
  "envGroups.createScopeLoading": {
    message: "Carregando ambientes e serviços…",
    description:
      "Create dialog waits for a confirmed scope and link candidates",
  },
  "envGroups.createScopeError": {
    message:
      "Não foi possível carregar o ambiente ou os serviços. Tente novamente antes de criar o grupo.",
    description:
      "Create dialog blocks submission when scope or service lookup fails",
  },
  "envGroups.noServicesToLink": {
    message: "Este workspace ainda não tem serviços para vincular.",
    description: "Initial service links empty state",
  },
  "envGroups.noServicesInScope": {
    message:
      "Nenhum serviço neste ambiente. Os serviços só podem ser vinculados a um grupo do próprio ambiente.",
    description: "Initial service links empty state for the picked scope",
  },
  "envGroups.invalidInitialContents": {
    message:
      "Corrija as chaves de variáveis ou os nomes de arquivos inválidos antes de criar o grupo.",
    description: "Invalid initial env-group contents message",
  },
  "envGroups.nameLabel": {
    message: "Nome do grupo",
    description: "Env-group name field label",
  },
  "envGroups.namePlaceholder": {
    message: "shared-production",
    description: "Env-group name placeholder",
  },
  "envGroups.invalidName": {
    message: "Informe um nome para o grupo.",
    description: "Invalid env-group name message",
  },
  "envGroups.cancel": {
    message: "Cancelar",
    description: "Cancel an env-group dialog",
  },
  "envGroups.createSubmit": {
    message: "Criar grupo de ambiente",
    description: "Create env-group submit button",
  },
  "envGroups.emptyTitle": {
    message: "Nenhum grupo de ambiente",
    description: "Workspace env-groups empty-state title",
  },
  "envGroups.emptyBody": {
    message:
      "Crie um grupo para gerenciar a configuração compartilhada antes de vinculá-lo a um serviço.",
    description: "Workspace env-groups empty-state body",
  },
  "envGroups.serviceCount_one": {
    message: "{count} serviço vinculado",
    description: "Env-group linked-service count (singular)",
  },
  "envGroups.serviceCount_many": {
    message: "{count} de serviços vinculados",
    description: "Env-group linked-service count (plural)",
  },
  "envGroups.serviceCount_other": {
    message: "{count} serviços vinculados",
    description: "Env-group linked-service count (plural)",
  },
  "envGroups.metadataTitle": {
    message: "Metadados do grupo",
    description: "Env-group metadata card title",
  },
  "envGroups.ownerLabel": {
    message: "Proprietário",
    description: "Env-group owner id label",
  },
  "envGroups.environmentLabel": {
    message: "Ambiente",
    description: "Environment Group scope label",
  },
  "envGroups.workspaceLabel": {
    message: "Workspace",
    description: "Destination workspace label",
  },
  "envGroups.workspaceScope": {
    message: "Workspace (sem ambiente)",
    description: "Standalone Environment Group scope",
  },
  "envGroups.environmentLookupLoading": {
    message: "Carregando ambiente…",
    description:
      "Group metadata Environment value while the scope lookup is still loading",
  },
  "envGroups.environmentLookupFailed": {
    message: "Não foi possível carregar o ambiente",
    description:
      "Group metadata Environment value when the scope lookup request failed",
  },
  "envGroups.unknownEnvironment": {
    message: "Ambiente desconhecido ({id})",
    description: "Deleted or inaccessible Environment fallback",
  },
  "envGroups.createdAtLabel": {
    message: "Criado",
    description: "Env-group created timestamp label",
  },
  "envGroups.updatedAtLabel": {
    message: "Atualizado",
    description: "Env-group updated timestamp label",
  },
  "envGroups.availabilityLabel": {
    message: "Status",
    description: "Env-group availability label when busy or repair-required",
  },
  "envGroups.availabilityBusy": {
    message: "Salvamento em andamento — tente novamente em instantes",
    description: "Busy environment-group availability message",
  },
  "envGroups.availabilityRepairRequired": {
    message: "Recuperação necessária — contate o suporte se isso persistir",
    description: "Repair-required environment-group availability message",
  },
  "envGroups.unavailableTitle": {
    message: "Grupos de ambiente indisponíveis",
    description: "Secret store unavailable state title",
  },
  "envGroups.unavailableBody": {
    message:
      "O armazenamento de segredos não está configurado para esta instalação.",
    description: "Secret store unavailable state body",
  },
  "envGroups.forbiddenTitle": {
    message: "Sem autorização",
    description: "Env-group forbidden state title",
  },
  "envGroups.forbiddenBody": {
    message: "Você não tem permissão para ver ou gerenciar grupos de ambiente.",
    description: "Env-group forbidden state body",
  },
  "envGroups.genericTitle": {
    message: "Não foi possível carregar os grupos de ambiente",
    description: "Env-group generic query error title",
  },
  "envGroups.genericBody": {
    message: "Algo deu errado. Tente novamente.",
    description: "Env-group generic query error body",
  },
  "envGroups.errorTitle": {
    message: "Não foi possível carregar o conteúdo do grupo",
    description: "Env-group editor error title",
  },
  "envGroups.errorBody": {
    message: "Algo deu errado. Tente novamente.",
    description: "Env-group editor error body",
  },
  "envGroups.backToList": {
    message: "Voltar para os grupos de ambiente",
    description: "Detail-page back button label",
  },
  "envGroups.renameButton": {
    message: "Renomear",
    description: "Open rename env-group dialog",
  },
  "envGroups.manageButton": {
    message: "Gerenciar",
    description: "Open Environment Group management menu",
  },
  "envGroups.moveButton": {
    message: "Mover grupo",
    description: "Open Environment Group move dialog",
  },
  "envGroups.moveTitle": {
    message: "Mover grupo de ambiente",
    description: "Environment Group move dialog title",
  },
  "envGroups.moveDescription": {
    message:
      "Escolha um ambiente compatível ou devolva este grupo ao escopo do workspace.",
    description: "Environment Group move dialog description",
  },
  "envGroups.moveSubmit": {
    message: "Mover grupo",
    description: "Submit Environment Group move",
  },
  "envGroups.moveSuccess": {
    message: "Grupo de ambiente movido",
    description: "Environment Group move success toast",
  },
  "envGroups.moveError": {
    message: "Não foi possível mover o grupo",
    description: "Environment Group move error toast",
  },
  "envGroups.cloneButton": {
    message: "Clonar grupo",
    description: "Open Environment Group clone dialog",
  },
  "envGroups.cloneTitle": {
    message: "Clonar grupo de ambiente",
    description: "Environment Group clone dialog title",
  },
  "envGroups.cloneDescription": {
    message:
      "Copie variáveis e arquivos para um novo grupo. Os vínculos de serviços não são copiados.",
    description: "Environment Group clone dialog description",
  },
  "envGroups.cloneSubmit": {
    message: "Clonar grupo",
    description: "Submit Environment Group clone",
  },
  "envGroups.cloneSuccess": {
    message: "{name} clonado",
    description: "Environment Group clone success toast",
  },
  "envGroups.cloneError": {
    message: "Não foi possível clonar o grupo",
    description: "Environment Group clone error toast",
  },
  "envGroups.environmentPlaceholder": {
    message: "Selecione um ambiente",
    description: "Environment scope selector placeholder",
  },
  "envGroups.renameTitle": {
    message: "Renomear grupo de ambiente",
    description: "Rename env-group dialog title",
  },
  "envGroups.renameDescription": {
    message:
      "Renomear mantém as variáveis, os arquivos e os vínculos de serviços do grupo.",
    description: "Rename env-group dialog description",
  },
  "envGroups.renameSubmit": {
    message: "Salvar nome",
    description: "Rename env-group submit button",
  },
  "envGroups.deleteButton": {
    message: "Excluir",
    description: "Open delete env-group dialog",
  },
  "envGroups.deleteTitle": {
    message: "Excluir {name}?",
    description: "Delete env-group dialog title",
  },
  "envGroups.deleteDescription": {
    message:
      "Esta ação não pode ser desfeita. Tem certeza de que deseja excluir este grupo de ambiente? Ele será desvinculado de todos os serviços.",
    description: "Delete env-group dialog warning (Render-style copy)",
  },
  "envGroups.deletePrompt": {
    message: "Digite {phrase} abaixo para confirmar.",
    description:
      "Body prompt naming the exact 'sudo delete env group <name>' phrase (rendered bold by SudoCommandField)",
  },
  "envGroups.deleteConfirm": {
    message: "Excluir grupo de ambiente",
    description: "Delete env-group confirm button",
  },
  "envGroups.varsTitle": {
    message: "Variáveis de ambiente",
    description: "Env-group variable editor title",
  },
  "envGroups.varsDescription": {
    message:
      "Os valores são criptografados e aplicados a todos os serviços vinculados.",
    description: "Env-group variable editor description",
  },
  "envGroups.varsEmptyTitle": {
    message: "Nenhuma variável de ambiente",
    description: "Env-group variable editor empty title",
  },
  "envGroups.varsEmptyBody": {
    message:
      "Adicione uma variável agora; o grupo não precisa estar vinculado antes.",
    description: "Env-group variable editor empty body",
  },
  "envGroups.varDeleteConfirmBody": {
    message:
      "Os serviços vinculados fazem novo deploy sem esta variável, exceto um serviço baseado em repositório com deploy automático desativado, que continua atendendo com a release atual e aplica a mudança no próximo deploy.",
    description: "Delete env-group variable warning",
  },
  "envGroups.filesTitle": {
    message: "Arquivos secretos",
    description: "Env-group secret-file editor title",
  },
  "envGroups.filesDescription": {
    message:
      "Os arquivos criptografados são montados em todos os serviços vinculados em /etc/secrets.",
    description: "Env-group secret-file editor description",
  },
  "envGroups.filesEmptyTitle": {
    message: "Nenhum arquivo secreto",
    description: "Env-group secret-file editor empty title",
  },
  "envGroups.filesEmptyBody": {
    message:
      "Adicione um arquivo agora; o grupo não precisa estar vinculado antes.",
    description: "Env-group secret-file editor empty body",
  },
  "envGroups.fileDeleteConfirmBody": {
    message:
      "Os serviços vinculados fazem novo deploy sem este arquivo, exceto um serviço baseado em repositório com deploy automático desativado, que continua atendendo com a release atual e aplica a mudança no próximo deploy.",
    description: "Delete env-group secret-file warning",
  },
  "envGroups.servicesTitle": {
    message: "Serviços vinculados",
    description: "Env-group linked-services card title",
  },
  "envGroups.servicesDescription": {
    message:
      "Vincular ou desvincular altera a configuração do serviço. Um serviço baseado em repositório com deploy automático desativado aplica a mudança no próximo deploy; os demais serviços fazem o deploy agora.",
    description: "Env-group linked-services card description",
  },
  "envGroups.selectService": {
    message: "Selecione um serviço",
    description: "Link-service selector placeholder",
  },
  "envGroups.linkButton": {
    message: "Vincular serviço",
    description: "Link selected service to env group",
  },
  "envGroups.unlinkButton": {
    message: "Desvincular",
    description: "Unlink service from env group",
  },
  "envGroups.unlinkServiceAria": {
    message: "Desvincular {name}",
    description:
      "Accessible name of a linked-service row's Unlink control, naming the service (w4/150)",
  },
  "envGroups.noLinkedServices": {
    message:
      "Este grupo ainda não está vinculado a nenhum serviço. Ele continua totalmente editável.",
    description: "No linked services state",
  },
  "envGroups.servicesLoadError": {
    message:
      "Não foi possível carregar os serviços do workspace. Os vínculos existentes continuam disponíveis abaixo.",
    description: "Linked-services inventory error",
  },
  "envGroups.incompatibleLink": {
    message: "Vinculado fora do ambiente deste grupo",
    description: "Warning on an existing incompatible Environment Group link",
  },
  "envGroups.searchPlaceholder": {
    message: "Pesquisar grupos de ambiente",
    description: "Environment Group list search placeholder",
  },
  "envGroups.searchLabel": {
    message: "Pesquisar grupos de ambiente por nome",
    description: "Accessible Environment Group search label",
  },
  "envGroups.resetSearch": {
    message: "Limpar pesquisa",
    description: "Clear Environment Group search",
  },
  "envGroups.noMatchesTitle": {
    message: "Nenhum grupo de ambiente correspondente",
    description: "Environment Group search empty-state title",
  },
  "envGroups.noMatchesBody": {
    message: "Tente outro nome ou limpe a pesquisa.",
    description: "Environment Group search empty-state body",
  },
  "envGroups.colName": {
    message: "Nome do grupo de ambiente",
    description: "Environment Group table name column",
  },
  "envGroups.colEnvironment": {
    message: "Ambiente",
    description: "Environment Group table Environment column",
  },
  "envGroups.colEnvVars": {
    message: "Variáveis de ambiente",
    description: "Environment Group table variable-count column",
  },
  "envGroups.colSecretFiles": {
    message: "Arquivos secretos",
    description: "Environment Group table secret-file-count column",
  },
  "envGroups.colUpdated": {
    message: "Atualizado",
    description: "Environment Group table updated-time column",
  },
  "envGroups.rolloutNote": {
    message:
      "Os serviços vinculados estão fazendo novo deploy para aplicar a mudança, exceto um serviço baseado em repositório com deploy automático desativado, que a aplica no próximo deploy.",
    description: "Env-group write rollout toast detail",
  },
  "envGroups.linkChangeNote": {
    message:
      "A configuração do serviço mudou. Um serviço baseado em repositório com deploy automático desativado a aplica no próximo deploy; qualquer outro serviço faz o deploy agora.",
    description: "Env-group link/unlink success toast detail",
  },
  "envGroups.createSuccess": {
    message: "{name} criado",
    description: "Env-group create success toast",
  },
  "envGroups.createError": {
    message: "Não foi possível criar {name}",
    description: "Env-group create error toast",
  },
  "envGroups.renameSuccess": {
    message: "Renomeado para {name}",
    description: "Env-group rename success toast",
  },
  "envGroups.renameError": {
    message: "Não foi possível renomear o grupo",
    description: "Env-group rename error toast",
  },
  "envGroups.deleteSuccess": {
    message: "Grupo de ambiente excluído",
    description: "Env-group delete success toast",
  },
  "envGroups.deleteError": {
    message: "Não foi possível excluir o grupo",
    description: "Env-group delete error toast",
  },
  "envGroups.linkSuccess": {
    message: "Serviço vinculado",
    description: "Env-group link success toast",
  },
  "envGroups.linkError": {
    message: "Não foi possível vincular o serviço",
    description: "Env-group link error toast",
  },
  "envGroups.unlinkSuccess": {
    message: "Serviço desvinculado",
    description: "Env-group unlink success toast",
  },
  "envGroups.unlinkError": {
    message: "Não foi possível desvincular o serviço",
    description: "Env-group unlink error toast",
  },
  "envGroups.varSaveSuccess": {
    message: "{key} salvo",
    description: "Env-group variable save success toast",
  },
  "envGroups.varsSaveSuccess": {
    message: "Variáveis de ambiente salvas",
    description: "Env-group replace-all variables success toast",
  },
  "envGroups.varsSaveError": {
    message: "Não foi possível salvar as variáveis de ambiente",
    description: "Env-group replace-all variables error toast",
  },
  "envGroups.varSaveError": {
    message: "Não foi possível salvar {key}",
    description: "Env-group variable save error toast",
  },
  "envGroups.varDeleteSuccess": {
    message: "{key} removido",
    description: "Env-group variable delete success toast",
  },
  "envGroups.varDeleteError": {
    message: "Não foi possível remover {key}",
    description: "Env-group variable delete error toast",
  },
};

export default ptEnvGroups;
