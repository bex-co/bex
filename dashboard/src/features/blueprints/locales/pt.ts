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
      "Preview alert body on /blueprints/new when the only problem is a takeover conflict (replaces the raw API message)",
  },
  "blueprints.previewConflict": {
    message: "Já gerenciado — o deploy irá assumi-lo",
    description:
      "Alert title on /blueprints/new when the only preview problem is a resource another Blueprint manages, which an explicit takeover confirmation resolves (w4/m125)",
  },
  "blueprints.previewMatchTitle": {
    message: "Corresponde a recursos existentes",
    description:
      "Notice title on /blueprints/new when the Blueprint file names resources that already exist in the workspace (w8/m153, Render parity)",
  },
  "blueprints.previewMatchBody": {
    message:
      "Este Blueprint corresponde a recursos que já existem no seu workspace. Os novos recursos serão criados com o sufixo “-{suffix}” adicionado a cada nome. Os recursos existentes não são alterados.",
    description:
      "Notice body on /blueprints/new for matched resources; {suffix} is the four-character resource name suffix, shown after a hyphen as it appears in a name",
  },
  "blueprints.previewMatchListLabel": {
    message: "Já no seu workspace:",
    description:
      "Notice on /blueprints/new — label above the list of existing resources the Blueprint file matches",
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
    description: "Blueprints list header — opens the export dialog (w8/m22)",
  },
  "blueprints.generateTitle": {
    message: "Gerar Blueprint",
    description: "Generate dialog title",
  },
  "blueprints.generateDescription": {
    message:
      "Selecione recursos existentes para exportar como um render.yaml que você pode fazer commit em um repositório e conectar como Blueprint.",
    description: "Generate dialog description",
  },
  "blueprints.generateEmptyHint": {
    message: "Selecione pelo menos um recurso para gerar.",
    description: "Generate dialog hint when nothing is selected",
  },
  "blueprints.generateAction": {
    message: "Gerar",
    description: "Generate dialog primary action",
  },
  "blueprints.generateBack": {
    message: "Voltar",
    description: "Generate dialog — return from preview to selection",
  },
  "blueprints.generateCopy": {
    message: "Copiar",
    description: "Generate dialog — copy the yaml to the clipboard",
  },
  "blueprints.generateCopied": {
    message: "render.yaml copiado",
    description: "Toast after copying the generated manifest",
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
  "blueprints.generateSecretsNote": {
    message:
      "Os valores armazenados nunca são exportados. As variáveis de serviço marcadas com sync: false são solicitadas quando o Blueprint é criado pela primeira vez. As variáveis marcadas com generateValue — incluindo as chaves de grupos de ambiente exportadas — mantêm o valor existente ou recebem um valor gerado se estiverem ausentes.",
    description: "Generate dialog note under the yaml preview",
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
  "blueprints.metaBranch": {
    message: "Branch",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaPath": {
    message: "Caminho do manifesto",
    description:
      "Blueprint detail metadata label — path to render.yaml in repo",
  },
  "blueprints.metaAutoSync": {
    message: "Sincronização automática",
    description: "Blueprint detail metadata label — auto-sync on push toggle",
  },
  "blueprints.metaCreated": {
    message: "Criado",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaUpdated": {
    message: "Última sincronização",
    description: "Blueprint detail metadata label",
  },
  "blueprints.autoSyncOn": {
    message: "Ativada",
    description: "Auto-sync toggle label — enabled",
  },
  "blueprints.autoSyncOff": {
    message: "Desativada",
    description: "Auto-sync toggle label — disabled",
  },
  "blueprints.resourcesTitle": {
    message: "Recursos gerenciados",
    description:
      "Blueprint detail section heading listing services/databases managed by this blueprint",
  },
  "blueprints.resourcesEmpty": {
    message:
      "Nenhum recurso ainda — sincronize o Blueprint para aplicar seu render.yaml.",
    description: "Blueprint managed-resources empty state (never synced)",
  },
  "blueprints.resourcesEmptyAwaitingApproval": {
    message:
      "Nenhum recurso ainda — aprove a sincronização planejada para criar os recursos do seu render.yaml.",
    description:
      "Blueprint managed-resources empty state while the Blueprint awaits approval of its planned sync",
  },
  "blueprints.resourcesEmptyAfterError": {
    message:
      "Nenhum recurso assumido. A última sincronização falhou — confira o erro no Histórico de sincronizações e tente novamente depois de corrigir o Blueprint.",
    description:
      "Blueprint managed-resources empty state when status is error after a sync",
  },
  "blueprints.syncHistoryTitle": {
    message: "Histórico de sincronizações",
    description: "Blueprint detail section heading for the sync run table",
  },
  "blueprints.syncHistoryEmpty": {
    message: "Nenhuma sincronização ainda.",
    description: "Blueprint sync history empty state",
  },
  "blueprints.syncColCommit": {
    message: "Commit",
    description: "Sync history table column — commit SHA",
  },
  "blueprints.syncColState": {
    message: "Estado",
    description: "Sync history table column — sync run state",
  },
  "blueprints.syncColStarted": {
    message: "Iniciada",
    description: "Sync history table column — sync run start time",
  },
  "blueprints.syncColCompleted": {
    message: "Concluída",
    description: "Sync history table column — sync run completion time",
  },
  "blueprints.detachPreviewTitle": {
    message: "Os recursos deixarão de ser gerenciados",
    description: "Blueprint resource detachment notice.",
  },
  "blueprints.detachSuccessTitle": {
    message: "Sincronização concluída — recursos desvinculados",
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
  "blueprints.syncColError": {
    message: "Detalhes",
    description: "Sync history table column — errors and durable sync notes",
  },
  "blueprints.manifestTitle": {
    message: "Manifesto render.yaml",
    description: "Blueprint detail section heading for the stored manifest",
  },
  "blueprints.plannedTitle": {
    message: "Sincronização planejada",
    description:
      "Blueprint detail: heading of the planned-sync card a created Blueprint shows",
  },
  "blueprints.plannedBody": {
    message:
      "Este Blueprint foi criado com um plano. Revise o que a aprovação alterará no commit {commit} e depois aprove para aplicá-lo.",
    description:
      "Planned-sync card: explains plan-then-approve; {commit} is a short commit SHA",
  },
  "blueprints.plannedActions": {
    message: "Alterações planejadas",
    description: "Planned-sync card: accessible label of the action list",
  },
  "blueprints.plannedAction": {
    message: "{operation} {kind} {name}",
    description:
      "Planned-sync card: one plan action, e.g. 'create service api'",
  },
  "blueprints.plannedActionsUnavailable": {
    message:
      "Este plano foi registrado antes de os planos guardarem suas alterações. Planeje novamente para vê-las, ou aprove para aplicá-lo.",
    description:
      "Planned-sync card: a plan stored without its actions (recorded before migration 0187)",
  },
  "blueprints.plannedManualEnvVars": {
    message:
      "Estes valores não estão no arquivo de Blueprint (sync: false). Insira-os para aprovar.",
    description:
      "Planned-sync card: intro to the required manual env var fields",
  },
  "blueprints.plannedManualEnvVarLabel": {
    message: "{service}: {key}",
    description: "Planned-sync card: label of one manual env var field",
  },
  "blueprints.plannedApprove": {
    message: "Aprovar",
    description: "Planned-sync card: button that applies the planned sync",
  },
  "blueprints.plannedReplan": {
    message: "Planejar novamente",
    description:
      "Planned-sync card: button that plans a new sync at the branch head",
  },
  "blueprints.plannedNone": {
    message:
      "Nenhum plano aguarda aprovação. Planeje novamente para ler o head da branch.",
    description:
      "Planned-sync card: shown when a created Blueprint has no approvable plan",
  },
  "blueprints.plannedApproved": {
    message: "Sincronização planejada aprovada",
    description: "Toast after approving a planned sync",
  },
  "blueprints.plannedApproveFailed": {
    message: "Não foi possível aprovar a sincronização planejada",
    description: "Toast when approving fails",
  },
  "blueprints.plannedChanged": {
    message:
      "Alguém planejou este Blueprint novamente. Revise o plano mais recente e depois aprove-o.",
    description:
      "Toast when the plan on screen was replaced before it was approved (BLUEPRINT_PLAN_CHANGED)",
  },
  "blueprints.plannedReplanFailed": {
    message: "Não foi possível planejar uma nova sincronização",
    description: "Toast when re-planning fails",
  },
  "blueprints.syncButton": {
    message: "Sincronizar",
    description: "Button label that triggers an idempotent blueprint re-apply",
  },
  "blueprints.syncConfirmTitle": {
    message: "Sincronizar Blueprint?",
    description: "Sync confirm dialog title",
  },
  "blueprints.syncConfirmBody": {
    message:
      "Isso aplica o render.yaml exatamente como estava no commit mostrado abaixo — não um push mais recente na branch. Os recursos que já correspondem a ele são mantidos como estão.",
    description: "Sync confirm dialog description",
  },
  "blueprints.syncConfirmAction": {
    message: "Sincronizar",
    description: "Sync confirm dialog primary action button label",
  },
  "blueprints.syncCancel": {
    message: "Cancelar",
    description: "Sync confirm dialog cancel button label",
  },
  "blueprints.syncPreviewLoading": {
    message: "Calculando o plano de sincronização a partir do repositório…",
    description: "Pre-sync dialog — loading line while blueprintPreview runs",
  },
  "blueprints.syncPreviewInvalid": {
    message:
      "O arquivo de Blueprint atualmente na branch tem problemas — sincronizar agora pode falhar ou recorrer ao manifesto armazenado:",
    description:
      "Pre-sync dialog — heading above validation errors from the fetched manifest",
  },
  "blueprints.syncPreviewUnavailable": {
    message:
      "Não foi possível calcular o plano de sincronização (prévia indisponível). Atualize depois que a prévia for bem-sucedida — a sincronização exige um commit revisado.",
    description:
      "Pre-sync dialog — preview transport failure; sync is disabled until a valid preview pins a commit (w8/m41)",
  },
  "blueprints.syncPreviewCommit": {
    message: "Commit revisado {sha}",
    description: "Pre-sync dialog — short SHA of the previewed Git revision",
  },
  "blueprints.syncRefreshPreview": {
    message: "Atualizar prévia",
    description:
      "Pre-sync dialog — refetch Git + re-validate after BLUEPRINT_SOURCE_CHANGED or an invalid preview",
  },
  "blueprints.syncSourceChanged": {
    message:
      "A origem do Blueprint mudou desde a sua revisão — atualize a prévia e confirme novamente",
    description:
      "Toast when syncBlueprint refuses with BLUEPRINT_SOURCE_CHANGED (path/repo drift or stale pin)",
  },
  "blueprints.syncSuccess": {
    message: "Blueprint sincronizado",
    description: "Toast shown after a successful syncBlueprint call",
  },
  "blueprints.syncError": {
    message: "Falha na sincronização",
    description: "Toast shown when syncBlueprint returns an error",
  },
  "blueprints.syncBusy": {
    message:
      "Esta sincronização não controla mais a execução (ocupada, interrompida ou substituída) — inicie uma nova sincronização; o trabalho parcial nunca é reexecutado",
    description:
      "Toast shown when syncBlueprint refuses with BLUEPRINT_SYNC_BUSY (admission race, mid-apply fence, or disconnect-busy)",
  },
  "blueprints.editField": {
    message: "Editar {field}",
    description: "Aria label for the inline-edit pencil button",
  },
  "blueprints.saveField": {
    message: "Salvar {field}",
    description: "Aria label for the inline-edit save button",
  },
  "blueprints.cancelEdit": {
    message: "Cancelar edição",
    description: "Aria label for the inline-edit cancel button",
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
  "blueprints.updateError": {
    message: "Falha na atualização",
    description: "Toast shown when updateBlueprint returns an error",
  },
  "blueprints.disconnectButton": {
    message: "Desconectar",
    description:
      "Button that disconnects a blueprint from its Git repo (stops auto-sync, keeps resources)",
  },
  "blueprints.disconnectTitle": {
    message: "Desconectar Blueprint?",
    description: "Disconnect confirm dialog title",
  },
  "blueprints.disconnectBody": {
    message:
      "Isso interrompe a sincronização automática no push e remove o Blueprint da sua lista. Os recursos já implantados permanecem intocados.",
    description: "Disconnect confirm dialog description",
  },
  "blueprints.disconnectAction": {
    message: "Desconectar",
    description: "Disconnect confirm dialog primary action button label",
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
};

export default ptBlueprints;
