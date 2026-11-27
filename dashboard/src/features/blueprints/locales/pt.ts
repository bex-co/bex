import type { TranslationEntry } from "@/i18n";

const ptBlueprints: Record<string, TranslationEntry> = {
  "blueprints.resourceType": {
    message: "Blueprint",
    description: "Blueprint resource type used in document titles",
  },
  "blueprints.pageTitle": {
    message: "Blueprints",
    description: "Blueprints list page heading and document title",
  },
  "blueprints.cardTitle": {
    message: "Blueprints",
    description: "Blueprints table card title",
  },
  "blueprints.colName": {
    message: "Nome",
    description:
      "Blueprints table column header — blueprint name (derived from repo)",
  },
  "blueprints.colRepo": {
    message: "Repositório",
    description: "Blueprints table column header — source repo URL",
  },
  "blueprints.colBranch": {
    message: "Branch",
    description: "Blueprints table column header — tracked branch",
  },
  "blueprints.colStatus": {
    message: "Status",
    description: "Blueprints table column header",
  },
  "blueprints.colUpdated": {
    message: "Última atualização",
    description: "Blueprints table column header — updatedAt relative age",
  },
  "blueprints.createButton": {
    message: "Nova instância de Blueprint",
    description: "Button label that opens the New Blueprint dialog",
  },
  "blueprints.createTitle": {
    message: "Nova instância de Blueprint",
    description: "New Blueprint page title",
  },
  "blueprints.createDescription": {
    message:
      "Conecte um repositório que contenha um Blueprint render.yaml para criar e sincronizar uma stack completa de serviços.",
    description: "New Blueprint page subtitle under the title",
  },
  "blueprints.createSourceTitle": {
    message: "Conectar um repositório",
    description:
      "New Blueprint page — heading above the GitHub/Public Git source tabs (Render parity)",
  },
  "blueprints.createBranchLabel": {
    message: "Branch",
    description: "New Blueprint page — branch combobox label",
  },
  "blueprints.createBranchPlaceholder": {
    message: "main",
    description: "New Blueprint page — branch combobox placeholder",
  },
  "blueprints.createBranchHint": {
    message: "A branch do repositório com o arquivo render.yaml.",
    description: "New Blueprint page — branch field helper text",
  },
  "blueprints.createPathLabel": {
    message: "Caminho do Blueprint",
    description: "New Blueprint page — render.yaml path input label",
  },
  "blueprints.createNamePlaceholder": {
    message: "my-stack",
    description: "New Blueprint page — blueprint name placeholder",
  },
  "blueprints.createPathPlaceholder": {
    message: "render.yaml",
    description: "New Blueprint page — manifest path placeholder",
  },
  "blueprints.createPathHint": {
    message:
      "O caminho do arquivo de Blueprint no seu repositório — qualquer arquivo .yaml/.yml funciona (ex.: infra/bex/stack.yaml). O padrão é render.yaml na raiz do repositório.",
    description: "New Blueprint page — manifest path helper text",
  },
  "blueprints.createNameLabel": {
    message: "Nome do Blueprint",
    description: "New Blueprint page — name input label",
  },
  "blueprints.createNameTaken": {
    message:
      "Já existe um Blueprint chamado {name} neste workspace. Escolha outro nome.",
    description:
      "New Blueprint page — shown under the name field when Deploy was refused because an approved Blueprint already has the name (w8/250)",
  },
  "blueprints.createAction": {
    message: "Fazer deploy do Blueprint",
    description: "New Blueprint dialog submit button label",
  },
  "blueprints.createCancel": {
    message: "Cancelar",
    description: "New Blueprint dialog cancel button label",
  },
  "blueprints.createSuccess": {
    message: "Blueprint criado",
    description: "Toast shown after a successful createBlueprint call",
  },
  "blueprints.createError": {
    message: "Falha ao criar o Blueprint",
    description: "Toast shown when createBlueprint returns an error",
  },
  "blueprints.previewTitle": {
    message: "Revisar a configuração do Blueprint",
    description:
      "New Blueprint page — heading of the pre-create fetch + validate section",
  },
  "blueprints.previewSelectSource": {
    message:
      "Selecione um repositório e uma branch para revisar o arquivo de Blueprint antes do deploy.",
    description: "Review section placeholder before a source is chosen",
  },
  "blueprints.previewLoading": {
    message: "Buscando {path} no seu repositório…",
    description: "Review section loading text while the manifest is fetched",
  },
  "blueprints.previewNotFoundTitle": {
    message: "Arquivo de Blueprint não encontrado",
    description: "Review section error title when the manifest fetch fails",
  },
  "blueprints.previewNotFoundBody": {
    message: "Arquivo de Blueprint {path} não encontrado na branch {branch}.",
    description:
      "Review section fallback error body when the backend returns no message",
  },
  "blueprints.previewFileNotFoundTitle": {
    message: "Arquivo de Blueprint não encontrado",
    description:
      "Preview failure title: the repo and branch resolved, the file did not",
  },
  "blueprints.previewFileNotFoundBody": {
    message:
      "{path} não existe na branch {branch}. Verifique o caminho ou faça commit do arquivo nessa branch.",
    description: "Preview failure body for file_not_found",
  },
  "blueprints.previewBranchNotFoundTitle": {
    message: "Branch não encontrada",
    description: "Preview failure title for branch_not_found",
  },
  "blueprints.previewBranchNotFoundBody": {
    message: "A branch {branch} não existe neste repositório.",
    description: "Preview failure body for branch_not_found",
  },
  "blueprints.previewRepoNotFoundTitle": {
    message: "Repositório não encontrado",
    description: "Preview failure title for repo_not_found_or_no_access",
  },
  "blueprints.previewRepoNotFoundBody": {
    message:
      "Este repositório não existe, ou o app do GitHub do bex não consegue acessá-lo. Verifique o repositório e o acesso do app aos repositórios.",
    description: "Preview failure body for repo_not_found_or_no_access",
  },
  "blueprints.previewAccessDeniedTitle": {
    message: "Acesso negado",
    description: "Preview failure title for access_denied",
  },
  "blueprints.previewAccessDeniedBody": {
    message:
      "O app do GitHub do bex não está autorizado a ler este repositório. Verifique novamente a instalação e os repositórios aos quais ela tem acesso.",
    description: "Preview failure body for access_denied",
  },
  "blueprints.previewPublicRepoNotFoundBody": {
    message:
      "Este repositório não existe em {host}, ou não é público. O bex lê um Blueprint lá a partir de um repositório público.",
    description:
      "Preview failure body for repo_not_found_or_no_access on gitlab.com or bitbucket.org, read anonymously; {host} is that host",
  },
  "blueprints.previewPublicAccessDeniedBody": {
    message:
      "{host} não permitiu que o bex lesse este repositório. O bex lê um Blueprint lá a partir de um repositório público.",
    description:
      "Preview failure body for access_denied on gitlab.com or bitbucket.org, read anonymously; {host} is that host",
  },
  "blueprints.previewHostUnsupportedTitle": {
    message: "Host Git não suportado",
    description:
      "Preview failure title for a repository on a Git host bex reads no Blueprint from",
  },
  "blueprints.previewHostUnsupportedBody": {
    message:
      "Como o Render, o bex lê um Blueprint de um repositório público no github.com, gitlab.com ou bitbucket.org. {host} não é um deles.",
    description:
      "Preview failure body for a repository on a Git host bex reads no Blueprint from; {host} is that host",
  },
  "blueprints.previewRateLimitedTitle": {
    message: "{host} está limitando as requisições do bex",
    description:
      "Preview failure title for rate_limited; {host} is GitHub or the repository's host",
  },
  "blueprints.previewRateLimitedBody": {
    message:
      "Há requisições demais para {host} no momento. Tente novamente em alguns minutos.",
    description:
      "Preview failure body for rate_limited; {host} is GitHub or the repository's host",
  },
  "blueprints.previewAmbiguousTitle": {
    message: "Dois arquivos de Blueprint",
    description: "Preview failure title for ambiguous_filename",
  },
  "blueprints.previewAmbiguousBody": {
    message:
      "Este repositório tem render.yaml e bex.yml na branch {branch}. Remova um deles ou informe o arquivo explicitamente acima.",
    description: "Preview failure body for ambiguous_filename",
  },
  "blueprints.previewUnavailableTitle": {
    message: "Não foi possível acessar {host}",
    description:
      "Preview failure title for unavailable; {host} is GitHub or the repository's host",
  },
  "blueprints.previewUnavailableBody": {
    message:
      "O bex não conseguiu ler o arquivo de Blueprint em {host}. Tente novamente em alguns minutos.",
    description:
      "Preview failure body for unavailable; {host} is GitHub or the repository's host",
  },
  "blueprints.previewInvalidPathExtension": {
    message: "O caminho do Blueprint deve ser um arquivo .yaml ou .yml.",
    description: "Inline path-field error for invalid_path, wrong extension",
  },
  "blueprints.previewInvalidPathShape": {
    message:
      "O caminho do Blueprint deve ser um caminho limpo, relativo ao repositório (sem barra inicial e sem ..).",
    description: "Inline path-field error for invalid_path, bad shape",
  },
  "blueprints.previewRetry": {
    message: "Tentar novamente",
    description: "Review section retry button after a failed manifest fetch",
  },
  "blueprints.takeoverResourceTitle": {
    message: "Assumir “{name}”?",
    description:
      "Takeover dialog title when a resource is managed by another Blueprint (BLUEPRINT_RESOURCE_CONFLICT)",
  },
  "blueprints.takeoverResourceBody": {
    message:
      "{kind} “{name}” é gerenciado pelo Blueprint {owner}. Continuar o transfere para este Blueprint. Digite o comando abaixo para continuar.",
    description: "Takeover dialog body for a Blueprint resource conflict",
  },
  "blueprints.takeoverResourceBodyOtherOwner": {
    message:
      "{kind} “{name}” é gerenciado por outro Blueprint. Continuar o transfere para este Blueprint. Digite o comando abaixo para continuar.",
    description:
      "Takeover dialog body for a Blueprint resource conflict whose owning Blueprint bex-api could not name",
  },
  "blueprints.previewConflictBody": {
    message:
      "Outro Blueprint já gerencia alguns destes recursos. O deploy os transfere para este Blueprint; você confirmará exatamente o que será assumido na próxima etapa.",
    description:
      "Preview alert body on /blueprint/new when the only problem is a takeover conflict (replaces the raw API message)",
  },
  "blueprints.previewConflict": {
    message: "Já gerenciado — o deploy irá assumi-lo",
    description:
      "Alert title on /blueprint/new when the only preview problem is a resource another Blueprint manages, which an explicit takeover confirmation resolves (w4/m125)",
  },
  "blueprints.previewMatchTitle": {
    message: "Corresponde a recursos existentes",
    description:
      "Notice title on /blueprint/new when the Blueprint file names resources that already exist in the workspace (w8/m153, Render parity)",
  },
  "blueprints.previewMatchBody": {
    message:
      "Este Blueprint corresponde a recursos que já existem no seu workspace. Os novos recursos serão criados com o sufixo “-{suffix}” adicionado a cada nome. Os recursos existentes não são alterados.",
    description:
      "Notice body on /blueprint/new for matched resources; {suffix} is the four-character resource name suffix, shown after a hyphen as it appears in a name",
  },
  "blueprints.previewMatchListLabel": {
    message: "Já no seu workspace:",
    description:
      "Notice on /blueprint/new — label above the list of existing resources the Blueprint file matches",
  },
  "blueprints.previewInvalid": {
    message: "O arquivo de Blueprint tem erros",
    description:
      "Review section error title when the fetched manifest fails validation",
  },
  "blueprints.previewValid_one": {
    message:
      "Arquivo de Blueprint analisado com sucesso — {count} recurso será alterado.",
    description:
      "Review section success line, with the plan's total resource count (singular)",
  },
  "blueprints.previewValid_many": {
    message:
      "Arquivo de Blueprint analisado com sucesso — {count} de recursos serão alterados.",
    description:
      "Review section success line, with the plan's total resource count (plural)",
  },
  "blueprints.previewValid_other": {
    message:
      "Arquivo de Blueprint analisado com sucesso — {count} recursos serão alterados.",
    description:
      "Review section success line, with the plan's total resource count (plural)",
  },
  "blueprints.previewValidEnvironments_one": {
    message:
      "Arquivo de Blueprint analisado com sucesso — {count} ambiente será alterado.",
    description:
      "Review section success line when only environments change (w9/095, singular)",
  },
  "blueprints.previewValidEnvironments_many": {
    message:
      "Arquivo de Blueprint analisado com sucesso — {count} de ambientes serão alterados.",
    description:
      "Review section success line when only environments change (w9/095, plural)",
  },
  "blueprints.previewValidEnvironments_other": {
    message:
      "Arquivo de Blueprint analisado com sucesso — {count} ambientes serão alterados.",
    description:
      "Review section success line when only environments change (w9/095, plural)",
  },
  "blueprints.previewValidWithEnvironments_one": {
    message:
      "Arquivo de Blueprint analisado com sucesso — {count} recurso e {environments} serão alterados.",
    description:
      "Review section success line: changing resources (singular) and blueprints.previewEnvironmentCount (w9/095)",
  },
  "blueprints.previewValidWithEnvironments_many": {
    message:
      "Arquivo de Blueprint analisado com sucesso — {count} de recursos e {environments} serão alterados.",
    description:
      "Review section success line: changing resources (plural) and blueprints.previewEnvironmentCount (w9/095)",
  },
  "blueprints.previewValidWithEnvironments_other": {
    message:
      "Arquivo de Blueprint analisado com sucesso — {count} recursos e {environments} serão alterados.",
    description:
      "Review section success line: changing resources (plural) and blueprints.previewEnvironmentCount (w9/095)",
  },
  "blueprints.previewEnvironmentCount_one": {
    message: "{count} ambiente",
    description:
      "The changing environments inside blueprints.previewValidWithEnvironments (singular)",
  },
  "blueprints.previewEnvironmentCount_many": {
    message: "{count} de ambientes",
    description:
      "The changing environments inside blueprints.previewValidWithEnvironments (plural)",
  },
  "blueprints.previewEnvironmentCount_other": {
    message: "{count} ambientes",
    description:
      "The changing environments inside blueprints.previewValidWithEnvironments (plural)",
  },
  "blueprints.errorAtLineColumn": {
    message: "linha {line}, coluna {column}",
    description: "Source location beside a Blueprint validation error (w4/151)",
  },
  "blueprints.errorAtLine": {
    message: "linha {line}",
    description:
      "Source location beside a Blueprint validation error when only the line is known (w4/151)",
  },
  "blueprints.previewNoChanges": {
    message:
      "Arquivo de Blueprint analisado com sucesso — sem alterações. Todos os recursos já correspondem a este arquivo.",
    description:
      "Review section success line when every planned action is a no-op (w4/m138)",
  },
  "blueprints.previewOpCreate": {
    message: "Criar",
    description:
      "Plan row badge: the resource does not exist yet and will be created",
  },
  "blueprints.previewOpUpdate": {
    message: "Atualizar",
    description: "Plan row badge: an existing resource will be changed",
  },
  "blueprints.previewOpNoop": {
    message: "Sem alteração",
    description: "Plan row badge: the resource already matches the file",
  },
  "blueprints.previewOpError": {
    message: "Não é possível aplicar",
    description:
      "Plan row badge: the planner refused this resource (reason below)",
  },
  "blueprints.previewChangedFields": {
    message: "Alterações: {fields}",
    description:
      "Plan row detail for an update: the changed field paths (never values)",
  },
  "blueprints.previewKindService": {
    message: "Serviço",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindPostgres": {
    message: "Postgres",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindKeyValue": {
    message: "Key Value",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindEnvGroup": {
    message: "Grupo de ambiente",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindBuildSource": {
    message: "Origem do build",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindEnvironment": {
    message: "Ambiente",
    description: "Plan row resource kind: a projects[].environments[] entry",
  },
  "blueprints.previewServices": {
    message: "Serviços",
    description: "Review section plan group label — services in the manifest",
  },
  "blueprints.previewDatabases": {
    message: "Bancos de dados",
    description: "Review section plan group label — databases in the manifest",
  },
  "blueprints.previewKeyValue": {
    message: "Key Value",
    description:
      "Review section plan group label — key-value instances in the manifest",
  },
  "blueprints.previewEnvGroups": {
    message: "Grupos de ambiente",
    description: "Review section plan group label — env groups in the manifest",
  },
  "blueprints.previewManualSyncNote": {
    message:
      "O bex não recebe os pushes deste repositório, então as alterações futuras no seu arquivo de Blueprint não serão sincronizadas automaticamente. Sincronize o Blueprint após cada alteração, ou conecte o repositório pelo app do GitHub.",
    description:
      "Review section note for a repo whose pushes can't reach bex (the preview's autoSync is false): the Blueprint is created without auto-sync",
  },
  "blueprints.previewAutoSyncNote": {
    message:
      "Todas as atualizações futuras do seu arquivo de Blueprint nesta branch serão sincronizadas automaticamente. Você pode pausar a sincronização automática depois da criação.",
    description:
      "Review section note about auto-sync being enabled by default (Render parity)",
  },
  "blueprints.previewError": {
    message: "Não foi possível carregar a prévia do Blueprint.",
    description:
      "Review section error shown when the preview query itself fails (network)",
  },
  "blueprints.promptTitle": {
    message: "Valores secretos",
    description:
      "Review section — heading for the sync:false env var prompt inputs",
  },
  "blueprints.promptHint": {
    message:
      "Este Blueprint declara variáveis de ambiente com sync: false — os valores delas ficam apenas no bex, nunca no repositório. Defina-os agora; eles são definidos uma única vez e as sincronizações seguintes nunca os sobrescrevem.",
    description: "sync:false prompt section helper text",
  },
  "blueprints.promptEmptyWarning": {
    message:
      "Valores em branco são implantados como não definidos — o serviço pode falhar até você defini-los na aba Ambiente dele.",
    description:
      "Warning below the sync:false inputs when at least one is blank",
  },
  "blueprints.pricingTitle": {
    message: "Preço estimado",
    description: "Review section — estimated-pricing panel heading",
  },
  "blueprints.pricingSubtitle": {
    message:
      "Os custos de computação são calculados proporcionalmente por segundo. A cobrança ocorre no início do mês.",
    description:
      "Estimated-pricing panel subtitle explaining billing semantics (Render parity)",
  },
  "blueprints.pricingLineAmount": {
    message: "({tier}) ${amount} / mês",
    description:
      "Estimated-pricing row amount — plan label + monthly dollar cost",
  },
  "blueprints.pricingLineBreakdown": {
    message: "Instância ${instance} + Disco ({gb} GB) ${storage}",
    description:
      "Estimated-pricing datastore row sub-line — instance vs provisioned-storage cost breakdown",
  },
  "blueprints.pricingVariable": {
    message: "Variável",
    description:
      "Estimated-pricing row amount for resources whose cost depends on runtime behavior",
  },
  "blueprints.pricingReasonAutoscaling": {
    message: "Escalonamento automático",
    description:
      "Estimated-pricing variable-row badge — service has autoscaling enabled",
  },
  "blueprints.pricingReasonMultiInstance": {
    message: "Várias instâncias",
    description:
      "Estimated-pricing variable-row badge — service declares numInstances > 1",
  },
  "blueprints.pricingReasonCron": {
    message: "Cron Job",
    description:
      "Estimated-pricing variable-row badge — cron jobs bill only while runs execute",
  },
  "blueprints.pricingTotalLabel": {
    message: "Total",
    description: "Estimated-pricing panel total row label",
  },
  "blueprints.pricingTotalAmount": {
    message: "${amount}{marker} por mês",
    description:
      "Estimated-pricing panel total amount; marker is an asterisk when variable costs are excluded",
  },
  "blueprints.pricingExclusions": {
    message: "* Exceto {items}.",
    description:
      "Estimated-pricing footnote listing the variable costs excluded from the total",
  },
  "blueprints.pricingExcludeAutoscaling": {
    message: "escalonamento automático",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingExcludeMultiInstance": {
    message: "instâncias fixas adicionais",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingExcludeCron": {
    message: "Cron Jobs",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingEstimateNote": {
    message:
      "Apenas uma estimativa — o armazenamento é cobrado pelo tamanho provisionado; sua fatura mede o uso real.",
    description:
      "Estimated-pricing disclaimer: forward estimate is an upper bound of metered billing",
  },
  "blueprints.generateButton": {
    message: "Gerar Blueprint",
    description:
      "Generate Blueprint: the selection bar's, a resource row menu's and a project environment menu's entry that opens the Create Blueprint page (w3/m681)",
  },
  "blueprints.generatePageTitle": {
    message: "Criar um novo Blueprint a partir dos seus serviços",
    description:
      "Create Blueprint page title and document title, opened from a selection bar or a resource row's menu (w3/m681)",
  },
  "blueprints.generatePageTitleEnvironment": {
    message: "Criar um novo Blueprint a partir de {project} / {environment}",
    description:
      "Create Blueprint page title and document title, opened from a project environment's menu: the project and environment names",
  },
  "blueprints.generateBreadcrumb": {
    message: "Criar Blueprint",
    description:
      "Topbar breadcrumb of the Create Blueprint page (/blueprint/generate)",
  },
  "blueprints.generateStepDownload": {
    message: "Baixar o render.yaml gerado",
    description: "Create Blueprint page: step 1 title",
  },
  "blueprints.generateIncludes_one": {
    message: "Inclui {count} serviço:",
    description:
      "Create Blueprint page: step 1 sentence before the list of exported resource names (services, databases and Key Value all count)",
  },
  "blueprints.generateIncludes_many": {
    message: "Inclui {count} de serviços:",
    description:
      "Create Blueprint page: step 1 sentence before the list of exported resource names (services, databases and Key Value all count)",
  },
  "blueprints.generateIncludes_other": {
    message: "Inclui {count} serviços:",
    description:
      "Create Blueprint page: step 1 sentence before the list of exported resource names (services, databases and Key Value all count)",
  },
  "blueprints.generating": {
    message: "Gerando render.yaml...",
    description: "Create Blueprint page: step 1 while render.yaml is generated",
  },
  "blueprints.generateCopiedLabel": {
    message: "Copiado!",
    description:
      "Create Blueprint page: the Copy button's label for 2 seconds after copying render.yaml",
  },
  "blueprints.generateStepCommit": {
    message: "Adicionar render.yaml ao Git",
    description: "Create Blueprint page: step 2 title",
  },
  "blueprints.generateStepCommitBody": {
    message:
      "Faça commit do render.yaml no diretório raiz do seu repositório e faça push para o seu provedor Git.",
    description: "Create Blueprint page: step 2 text above the git commands",
  },
  "blueprints.generateStepConnect": {
    message: "Conectar seu novo Blueprint",
    description: "Create Blueprint page: step 3 title",
  },
  "blueprints.generateStepConnectBody": {
    message: "Crie uma nova instância de Blueprint e conecte seu repositório.",
    description: "Create Blueprint page: step 3 text, followed by a docs link",
  },
  "blueprints.generateSeeDocs": {
    message: "Consulte a documentação.",
    description: "Create Blueprint page: step 3 link to the Blueprint docs",
  },
  "blueprints.generateConnect": {
    message: "Conectar um novo Blueprint",
    description:
      "Create Blueprint page: step 3 button, opens the New Blueprint Instance page in a new tab",
  },
  "blueprints.generateHelpfulResources": {
    message: "Recursos úteis",
    description:
      "Create Blueprint page: heading of the links card beside the steps",
  },
  "blueprints.generateLearnBlueprints": {
    message: "Saiba mais sobre Blueprints",
    description:
      "Create Blueprint page: link to the Blueprint docs in the Helpful Resources card",
  },
  "blueprints.copied": {
    message: "Copiado",
    description: "Toast after copying a code snippet on a Blueprint page",
  },
  "blueprints.generateCopy": {
    message: "Copiar",
    description: "Generate dialog — copy the yaml to the clipboard",
  },
  "blueprints.generateCopyFailed": {
    message:
      "Não foi possível copiar o render.yaml. Permita o acesso à área de transferência e tente novamente, ou baixe o arquivo.",
    description:
      "Toast when the browser refuses to copy the generated manifest to the clipboard",
  },
  "blueprints.generateDownload": {
    message: "Baixar render.yaml",
    description: "Generate dialog — download the manifest file",
  },
  "blueprints.generateError": {
    message: "Não foi possível gerar o Blueprint",
    description: "Toast when the generate query fails",
  },
  "blueprints.emptyTitle": {
    message: "Você ainda não criou nenhuma instância de Blueprint.",
    description: "Blueprints list empty-state heading",
  },
  "blueprints.emptyBody": {
    message:
      "Os Blueprints são registrados automaticamente sempre que você faz deploy de um render.yaml hospedado em repositório. Depois do deploy, sua stack aparece aqui e você pode sincronizá-la ou validá-la.",
    description:
      "Blueprints list empty-state body explaining auto-registration",
  },
  "blueprints.loadingBody": {
    message: "Carregando Blueprints…",
    description: "Blueprints list loading state label",
  },
  "blueprints.errorTitle": {
    message: "Não foi possível carregar os Blueprints",
    description: "Blueprints list error state heading",
  },
  "blueprints.statusActive": {
    message: "Ativo",
    description: "Blueprint status badge — row is active",
  },
  "blueprints.statusInSync": {
    message: "Sincronizado",
    description: "Blueprint status badge — git-connected and in sync",
  },
  "blueprints.statusSyncing": {
    message: "Sincronizando",
    description: "Blueprint status badge — sync in progress",
  },
  "blueprints.statusError": {
    message: "Erro",
    description: "Blueprint status badge — last sync failed",
  },
  "blueprints.statusPaused": {
    message: "Pausado",
    description: "Blueprint status badge — auto-sync paused",
  },
  "blueprints.statusUnknown": {
    message: "{status}",
    description: "Blueprint status badge fallback — raw status value",
  },
  "blueprints.resourceSuffixLabel": {
    message: "Sufixo do nome do recurso",
    description:
      "Tooltip and screen-reader label of the suffix shown beside a Blueprint's name in the list and on its detail page (w8/m153)",
  },
  "blueprints.detailTitle": {
    message: "{name} · Blueprints",
    description: "Blueprint detail page document title",
  },
  "blueprints.metaRepo": {
    message: "Repositório",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaCreated": {
    message: "Criado",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaUpdated": {
    message: "Última sincronização",
    description: "Blueprint detail metadata label",
  },
  "blueprints.detailsTitle": {
    message: "Detalhes",
    description:
      "Blueprint Settings page: title of bex's card of read-only facts (repository, created, last synced) under Render's Settings card (w3/m679)",
  },
  "blueprints.settingsNameHint": {
    message: "Um nome exclusivo para o seu Blueprint.",
    description:
      "Blueprint Settings card: help under Blueprint Name (Render's copy, w3/m679)",
  },
  "blueprints.settingsRequired": {
    message: "Obrigatório",
    description:
      "Blueprint Settings card: shown under a row left empty (Render's copy, w3/m679)",
  },
  "blueprints.settingsNameTaken": {
    message: "Nome já em uso",
    description:
      "Blueprint Settings card: shown under Blueprint Name when another Blueprint of the workspace has the name (Render's copy, w3/m679)",
  },
  "blueprints.settingsAutoSync": {
    message: "Sincronização automática",
    description:
      "Blueprint Settings card: the Auto Sync row's label (Render's copy, w3/m679)",
  },
  "blueprints.settingsAutoSyncHint": {
    message:
      'Sincronizar automaticamente as alterações no seu arquivo de Blueprint? Selecione "Não" para fazer as sincronizações manualmente.',
    description:
      "Blueprint Settings card: help under Auto Sync; quotes the No option (Render's copy, w3/m679)",
  },
  "blueprints.settingsYes": {
    message: "Sim",
    description:
      "Blueprint Settings card: Auto Sync option, on (Render's copy, w3/m679)",
  },
  "blueprints.settingsNo": {
    message: "Não",
    description:
      "Blueprint Settings card: Auto Sync option, off (Render's copy, w3/m679)",
  },
  "blueprints.settingsBranch": {
    message: "Branch",
    description: "Blueprint Settings card: the Branch row's label (w3/m679)",
  },
  "blueprints.settingsBranchHint": {
    message: "O branch do Git a ser compilado e implantado.",
    description:
      "Blueprint Settings card: help under Branch (Render's copy, w3/m679)",
  },
  "blueprints.settingsBranchNotFound": {
    message: "Branch não encontrado neste repositório. Escolha outro branch.",
    description:
      "Blueprint Settings card: shown under Branch when the repository has no such branch (Render's copy, w3/m679)",
  },
  "blueprints.settingsPath": {
    message: "Caminho do Blueprint",
    description:
      "Blueprint Settings card: the Blueprint Path row's label (Render's copy, w3/m679)",
  },
  "blueprints.settingsPathHint": {
    message:
      "O caminho do arquivo de Blueprint no seu repositório (ex.: infra/render.yaml). O padrão é render.yaml na raiz do repositório.",
    description:
      "Blueprint Settings card: help under Blueprint Path (Render's copy, w3/m679)",
  },
  "blueprints.settingsFileNotFound": {
    message:
      'Arquivo de Blueprint "{path}" não encontrado no branch "{branch}". Verifique se o arquivo existe neste branch.',
    description:
      "Blueprint Settings card: shown under Branch or Blueprint Path when the Blueprint file is not at that path on that branch (Render's copy, w3/m679)",
  },
  "blueprints.settingsUpdateFailed": {
    message: "Não foi possível atualizar o Blueprint. Tente novamente.",
    description:
      "Blueprint Settings card: shown under a row whose save failed with no reason from the server (Render's copy, w3/m679)",
  },
  "blueprints.disconnectNoSync": {
    message:
      "Os recursos deixarão de ser sincronizados automaticamente a partir do seu arquivo de Blueprint.",
    description:
      "Disconnect Blueprint modal: first sentence (Render's copy, w3/m679)",
  },
  "blueprints.disconnectKeeps": {
    message:
      "Isso não exclui o arquivo em si. Isso não exclui os recursos gerenciados. Você pode conectar seu arquivo novamente a qualquer momento pela página de Blueprints.",
    description:
      "Disconnect Blueprint modal: second sentence (Render's copy, w3/m679)",
  },
  "blueprints.disconnectConfirm": {
    message:
      "Tem certeza de que deseja desconectar a especificação de Blueprint {name}?",
    description:
      "Disconnect Blueprint modal: the question, with the Blueprint's name in bold (Render's copy, w3/m679)",
  },
  "blueprints.resourcesEmpty": {
    message: "Nenhum recurso gerenciado por este Blueprint",
    description:
      "A Blueprint's Resources page when it manages nothing, Render's words (w3/m680)",
  },
  "blueprints.resourcesFileProblems_one": {
    message: "Houve um problema ao sincronizar com seu arquivo de Blueprint:",
    description:
      "A Blueprint's Resources page when its newest sync failed on its file: the line above the file's errors, Render's 'There was a problem' / 'There were a few problems' (w3/m680)",
  },
  "blueprints.resourcesFileProblems_many": {
    message:
      "Houve alguns problemas ao sincronizar com seu arquivo de Blueprint:",
    description:
      "A Blueprint's Resources page when its newest sync failed on its file: the line above the file's errors, Render's 'There was a problem' / 'There were a few problems' (w3/m680)",
  },
  "blueprints.resourcesFileProblems_other": {
    message:
      "Houve alguns problemas ao sincronizar com seu arquivo de Blueprint:",
    description:
      "A Blueprint's Resources page when its newest sync failed on its file: the line above the file's errors, Render's 'There was a problem' / 'There were a few problems' (w3/m680)",
  },
  "blueprints.colBuildSourceName": {
    message: "Nome da fonte de build",
    description:
      "A Blueprint's Resources page: the Build Sources table's name column, Render's 'Build Source Name' (w3/m680)",
  },
  "blueprints.buildSourceUnlinked": {
    message: "Não vinculado",
    description:
      "A Blueprint's Build Sources table: badge on a build source no service builds from, Render's 'Unlinked' (w3/m680)",
  },
  "blueprints.buildSourcesUngrouped": {
    message: "Sem grupo",
    description:
      "A Blueprint's Build Sources table: the row over the build sources in no project, Render's 'Ungrouped' (w3/m680)",
  },
  "blueprints.syncHistoryEmpty": {
    message: "Nenhuma sincronização ainda.",
    description: "Syncs page: no syncs yet, Render's wording (w3/m678)",
  },
  "blueprints.detachPreviewTitle": {
    message: "Os recursos deixarão de ser gerenciados",
    description: "Blueprint resource detachment notice.",
  },
  "blueprints.detachEstimateNote": {
    message: "Os custos deles não estão incluídos na estimativa abaixo.",
    description:
      "Detached resources are outside the proposed manifest estimate.",
  },
  "blueprints.detachWarning": {
    message:
      "Estes recursos continuam em execução e podem continuar gerando cobranças. Este Blueprint não os gerenciará mais.",
    description: "Blueprint resource detachment notice.",
  },
  "blueprints.manifestTitle": {
    message: "Manifesto render.yaml",
    description: "Blueprint detail section heading for the stored manifest",
  },
  "blueprints.plannedActions": {
    message: "Alterações planejadas",
    description:
      "A sync's page: accessible label of its list of planned actions (w3/m678)",
  },
  "blueprints.plannedAction": {
    message: "{operation} {kind} {name}",
    description:
      "Planned-sync card: one plan action, e.g. 'create service api'",
  },
  "blueprints.plannedManualEnvVars": {
    message:
      "Estes valores não estão no arquivo de Blueprint (sync: false). Insira-os para aprovar.",
    description:
      "A waiting sync's page: intro to the fields for the sync: false values approving it needs (w3/m678)",
  },
  "blueprints.plannedManualEnvVarLabel": {
    message: "{service}: {key}",
    description:
      "A waiting sync's page: label of one sync: false value's field (w3/m678)",
  },
  "blueprints.plannedApprove": {
    message: "Aprovar",
    description:
      "A waiting sync's page: the button that applies the planned sync, Render's Approve (w3/m678)",
  },
  "blueprints.plannedChanged": {
    message:
      "Alguém planejou este Blueprint novamente. Revise o plano mais recente e depois aprove-o.",
    description:
      "A sync's page: inline alert when the plan on screen was replaced before it was approved (BLUEPRINT_PLAN_CHANGED)",
  },
  "blueprints.plannedReplanFailed": {
    message: "Não foi possível planejar uma nova sincronização",
    description:
      "Sync Error banner: Manual sync could not plan a sync and the server gave no reason (w3/m678)",
  },
  "blueprints.syncButton": {
    message: "Sincronização manual",
    description:
      "The Blueprint header's one action, Render's Manual sync: plans a sync and opens its page (w3/m678)",
  },
  "blueprints.editField": {
    message: "Editar {field}",
    description: "Aria label for the inline-edit pencil button",
  },
  "blueprints.pathInvalid": {
    message:
      "Deve ser um caminho .yaml ou .yml limpo, relativo ao repositório (sem barra inicial e sem '..').",
    description: "Inline path editor client-side validation message",
  },
  "blueprints.updateSuccess": {
    message: "Blueprint atualizado",
    description: "Toast shown after a successful updateBlueprint call",
  },
  "blueprints.disconnectButton": {
    message: "Desconectar Blueprint",
    description:
      "Blueprint Settings page: the critical button that opens the Disconnect Blueprint modal (Render's copy, w3/m679)",
  },
  "blueprints.disconnectTitle": {
    message: "Desconectar Blueprint",
    description: "Disconnect Blueprint modal title (Render's copy, w3/m679)",
  },
  "blueprints.disconnectAction": {
    message: "Desconectar Blueprint",
    description:
      "Disconnect Blueprint modal: the critical confirm button (Render's copy, w3/m679)",
  },
  "blueprints.disconnectCancel": {
    message: "Cancelar",
    description: "Disconnect confirm dialog cancel button label",
  },
  "blueprints.disconnectSuccess": {
    message: "Blueprint desconectado",
    description: "Toast shown after a successful disconnectBlueprint call",
  },
  "blueprints.disconnectError": {
    message: "Falha ao desconectar",
    description: "Toast shown when disconnectBlueprint returns an error",
  },
  "blueprints.disconnectBusy": {
    message:
      "Uma sincronização está sendo aplicada — tente desconectar novamente quando ela terminar",
    description:
      "Toast shown when disconnectBlueprint refuses with BLUEPRINT_SYNC_BUSY",
  },
  "blueprints.validateTitle": {
    message: "Validar",
    description: "Validate panel section heading",
  },
  "blueprints.validateRun": {
    message: "Executar validação",
    description: "Validate panel button label",
  },
  "blueprints.validateValid": {
    message: "O manifesto é válido — nenhum erro encontrado.",
    description: "Validate result: manifest parsed successfully",
  },
  "blueprints.validateInvalid": {
    message: "O manifesto tem erros:",
    description:
      "Validate result: manifest has parse errors — followed by the error list",
  },
  "blueprints.validateNoResult": {
    message: "Nenhum resultado ainda.",
    description: "Validate panel placeholder before first run",
  },
  "blueprints.stateCreated": {
    message: "Aguardando aprovação",
    description:
      "Sync state (and Blueprint status) created: a planned sync waiting for approval (w2/m256); never a queued run.",
  },
  "blueprints.stateRunning": {
    message: "Em execução",
    description: "Sync-run state: the run is in progress.",
  },
  "blueprints.stateSuccess": {
    message: "Sucesso",
    description: "Sync-run state: the run completed successfully.",
  },
  "blueprints.createNameDescription": {
    message: "Um nome único para esta instância do Blueprint.",
    description: "Create-Blueprint Name row description",
  },
  "blueprints.createBlockedSource": {
    message: "Escolha um repositório para continuar.",
    description:
      "Create-Blueprint action bar: why Deploy is held while no repository is chosen",
  },
  "blueprints.createBlockedBranch": {
    message: "Insira uma branch para continuar.",
    description:
      "Create-Blueprint action bar: why Deploy is held while the branch is empty",
  },
  "blueprints.createBlockedReviewing": {
    message: "Revisando o arquivo do Blueprint…",
    description:
      "Create-Blueprint action bar: why Deploy is held while the Blueprint file is being reviewed",
  },
  "blueprints.createBlockedReview": {
    message: "Resolva os problemas encontrados na revisão para continuar.",
    description:
      "Create-Blueprint action bar: why Deploy is held while the review found problems in the Blueprint file",
  },
  "blueprints.navResources": {
    message: "Recursos",
    description:
      "Blueprint sidebar entry and page heading for the resources the Blueprint manages (Render's Resources, /blueprints/<id>/resources, w3/m677)",
  },
  "blueprints.navSyncs": {
    message: "Sincronizações",
    description:
      "Blueprint sidebar entry and page heading for the Blueprint's syncs (Render's Syncs, /blueprints/<id>/syncs, w3/m677)",
  },
  "blueprints.navSettings": {
    message: "Configurações",
    description:
      "Blueprint sidebar entry and page heading for the Blueprint's settings (Render's Settings, /blueprints/<id>/settings, w3/m677)",
  },
  "blueprints.resourcesViewSyncs": {
    message: "Ir para Sincronizações",
    description:
      "Link from a Blueprint's empty Resources page to its Syncs page, where the waiting plan or the failed sync is (w3/m677)",
  },
  "blueprints.syncViewDetails": {
    message: "Ver detalhes",
    description:
      "Sync card: link to the sync's own page (Render's View details, w3/m678)",
  },
  "blueprints.syncsLoadMore": {
    message: "Carregar mais",
    description:
      "Syncs page: button reading the next page of older syncs (w3/m678)",
  },
  "blueprints.syncsLoadMoreError": {
    message: "Não foi possível carregar as sincronizações anteriores.",
    description:
      "Syncs page: toast when reading an older page of syncs fails (w3/m678)",
  },
  "blueprints.syncPageTitle": {
    message: "Sincronização: {sha}",
    description:
      "A sync's page heading, Render's 'Sync: <short commit id>'; {sha} is the 7-character commit id (w3/m678)",
  },
  "blueprints.syncPageTitleNoCommit": {
    message: "Sincronização",
    description:
      "A sync's page heading when the sync read no commit, Render's 'Sync' (w3/m678)",
  },
  "blueprints.syncDocumentTitle": {
    message: "Sincronização",
    description:
      "Document title's page name on a sync's page: 'Sync ・ <name> ・ Blueprint' (w3/m678)",
  },
  "blueprints.syncUpToDate": {
    message: "Os recursos já estão atualizados",
    description:
      "A sync's page when its plan has no actions, Render's wording (w3/m678)",
  },
  "blueprints.syncNotFullySynced": {
    message: "Não conseguimos sincronizar completamente estes recursos.",
    description:
      "A failed sync's page with no actions and no errors, Render's critical line (w3/m678)",
  },
  "blueprints.syncNotFullySyncedBody": {
    message: "Se continuar vendo este erro, entre em contato com o suporte.",
    description:
      "Under blueprints.syncNotFullySynced: what to do if it keeps happening (w3/m678)",
  },
  "blueprints.syncActionsNotRecorded": {
    message:
      "Esta sincronização foi registrada antes de as sincronizações guardarem suas ações, por isso elas não podem ser exibidas.",
    description:
      "A sync's page for a sync recorded before every sync kept its actions (w3/m678)",
  },
  "blueprints.syncActionCreate": {
    message: "Criar {kind} {name}",
    description:
      "A sync's planned action: create a resource; {kind} is its kind label, {name} its name, drawn bold (w3/m678)",
  },
  "blueprints.syncActionUpdate": {
    message: "Atualizar {kind} {name}",
    description:
      "A sync's planned action: change a resource; {kind} is its kind label, {name} its name (w3/m678)",
  },
  "blueprints.syncActionDetach": {
    message: "Deixar de gerenciar {kind} {name}",
    description:
      "A sync's planned action: stop managing a resource the file no longer names; {kind}, {name} (w3/m678)",
  },
  "blueprints.syncActionError": {
    message: "Não é possível aplicar {kind} {name}",
    description:
      "A sync's planned action the planner refused; {kind}, {name}; the reason follows (w3/m678)",
  },
  "blueprints.syncActionDetachNote": {
    message: "Ele continua em execução e pode gerar cobranças.",
    description:
      "Under a 'stop managing' action: the resource is not deleted (w3/m678)",
  },
  "blueprints.syncActionStateUnknown": {
    message: "Estado desconhecido",
    description:
      "Accessible name of a planned action's state icon when bex can't say which state it reached (a failed sync's actions, w3/m678)",
  },
  "blueprints.syncApproveFailed": {
    message: "Não foi possível aprovar esta sincronização. Tente novamente.",
    description:
      "A sync's page: inline alert when Approve fails without the server's reason, Render's wording (w3/m678)",
  },
  "blueprints.syncNotFound": {
    message:
      "Esta sincronização não existe. Uma sincronização planejada não aprovada em 24 horas é excluída.",
    description:
      "A sync's page for an id that is not one of this Blueprint's syncs, e.g. a plan deleted after 24 hours (w3/m678)",
  },
  "blueprints.syncErrorBannerTitle": {
    message: "Erro de sincronização",
    description:
      "Title of Render's dismissible banner when Manual sync could not plan a sync (w3/m678)",
  },
  "blueprints.syncErrorBannerDismiss": {
    message: "Dispensar",
    description:
      "Accessible name of the Sync Error banner's close button (w3/m678)",
  },
  "blueprints.syncErrorsIntro_one": {
    message: "Houve um problema ao sincronizar com seu Blueprint:",
    description:
      "A sync's page: the line above its errors, Render's 'There was a problem' / 'There were a few problems' (w3/m678)",
  },
  "blueprints.syncErrorsIntro_many": {
    message: "Houve alguns problemas ao sincronizar com seu Blueprint:",
    description:
      "A sync's page: the line above its errors, Render's 'There was a problem' / 'There were a few problems' (w3/m678)",
  },
  "blueprints.syncErrorsIntro_other": {
    message: "Houve alguns problemas ao sincronizar com seu Blueprint:",
    description:
      "A sync's page: the line above its errors, Render's 'There was a problem' / 'There were a few problems' (w3/m678)",
  },
};

export default ptBlueprints;
