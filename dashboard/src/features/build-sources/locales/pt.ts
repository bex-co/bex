import type { TranslationEntry } from "@/i18n";

const ptBuildSources: Record<string, TranslationEntry> = {
  "buildSources.pageTitle": {
    message: "Fontes de build",
    description: "Title of the Build Sources list page",
  },
  "buildSources.pageDescription": {
    message:
      "Compile uma vez a partir de um repositório Git ou imagem Docker e implante o artefato em vários serviços.",
    description: "Subtitle of the Build Sources list page",
  },
  "buildSources.resourceType": {
    message: "Fonte de build",
    description: "Singular resource type label shown in global search results",
  },
  "buildSources.newButton": {
    message: "Nova fonte de build",
    description: "Primary CTA button to create a new Build Source",
  },
  "buildSources.backToList": {
    message: "Voltar para fontes de build",
    description: "Back button label returning to /build-sources",
  },
  "buildSources.viewAll": {
    message: "Ver todas",
    description: "Link from Project Overview Build Sources section to /build-sources",
  },
  "buildSources.viewBuildSources": {
    message: "Ver fontes de build",
    description: "Button in partial-link alert navigating to /build-sources",
  },
  "buildSources.emptyTitle": {
    message: "Nenhuma fonte de build ainda",
    description: "Empty state heading when the workspace has no build sources",
  },
  "buildSources.emptyDescription": {
    message:
      "Crie uma fonte de build para compartilhar um único build Git ou imagem Docker entre Web Services, Private Services, Background Workers e Cron Jobs.",
    description: "Empty state explanation on /build-sources",
  },
  "buildSources.refreshError": {
    message:
      "Não foi possível atualizar as fontes de build. Exibindo resultados em cache.",
    description: "Inline error banner when a background refresh fails over cached rows",
  },
  "buildSources.searchPlaceholder": {
    message: "Buscar por nome, repositório, imagem ou projeto…",
    description: "Placeholder for the /build-sources search input",
  },
  "buildSources.searchLabel": {
    message: "Buscar fontes de build",
    description: "Accessible label for the /build-sources search input",
  },
  "buildSources.resetFilters": {
    message: "Limpar filtros",
    description: "Button to clear active search and kind filters on /build-sources",
  },
  "buildSources.filterKindLabel": {
    message: "Filtrar por tipo de fonte",
    description: "Accessible label for the source-kind filter button group",
  },
  "buildSources.filterAll": {
    message: "Todas",
    description: "Filter button showing both Git and Docker image build sources",
  },
  "buildSources.kindGit": {
    message: "Git",
    description: "Source kind badge and filter label for Git-backed build sources",
  },
  "buildSources.kindImage": {
    message: "Imagem Docker",
    description: "Source kind badge and filter label for Docker-image-backed build sources",
  },
  "buildSources.noMatchesTitle": {
    message: "Nenhuma fonte de build encontrada",
    description: "Heading when search or kind filter matches zero build sources",
  },
  "buildSources.noMatchesDescription": {
    message: "Tente ajustar sua busca ou limpar o filtro de tipo de fonte.",
    description: "Body copy when search or kind filter matches zero build sources",
  },
  "buildSources.colName": {
    message: "Nome",
    description: "Table column header for Build Source name",
  },
  "buildSources.colSource": {
    message: "Fonte",
    description: "Table column header for Git repo/branch or Docker image",
  },
  "buildSources.colRegion": {
    message: "Região",
    description: "Table column header for Build Source region",
  },
  "buildSources.colLinkedServices": {
    message: "Serviços vinculados",
    description: "Table column header for linked service count",
  },
  "buildSources.colProject": {
    message: "Projeto",
    description: "Table column header for optional project scope",
  },
  "buildSources.colUpdated": {
    message: "Atualizado",
    description: "Table column header for last updated timestamp",
  },
  "buildSources.copyId": {
    message: "Copiar ID da fonte de build",
    description: "Accessible label for copying a bds- ID",
  },
  "buildSources.copied": {
    message: "Copiado",
    description: "Copy button confirmation tooltip",
  },
  "buildSources.copyError": {
    message: "Não foi possível copiar",
    description: "Copy button failure tooltip",
  },
  "buildSources.linkedServicesCount_one": {
    message: "{count} serviço",
    description: "Singular count of services linked to a build source",
  },
  "buildSources.linkedServicesCount_many": {
    message: "{count} de serviços",
    description: "Many count of services linked to a build source",
  },
  "buildSources.linkedServicesCount_other": {
    message: "{count} serviços",
    description: "Plural count of services linked to a build source",
  },
  "buildSources.workspaceScope": {
    message: "Workspace",
    description: "Label when a build source is workspace-wide (not scoped to a project)",
  },
  "buildSources.createTitle": {
    message: "Nova fonte de build",
    description: "Page heading of /build-sources/new",
  },
  "buildSources.createDescription": {
    message:
      "Configure um build de repositório Git ou imagem Docker compartilhada para vários serviços.",
    description: "Subtitle of /build-sources/new",
  },
  "buildSources.createSubmit": {
    message: "Criar fonte de build",
    description: "Submit button label on /build-sources/new",
  },
  "buildSources.createSuccess": {
    message: 'Fonte de build "{name}" criada',
    description: "Toast shown after creating a Build Source",
  },
  "buildSources.createError": {
    message: "Não foi possível criar a fonte de build. Tente novamente.",
    description: "Fallback error toast when createBuildSource fails",
  },
  "buildSources.updateSuccess": {
    message: 'Fonte de build "{name}" atualizada',
    description: "Toast shown after updating a Build Source",
  },
  "buildSources.updateError": {
    message: "Não foi possível atualizar a fonte de build. Tente novamente.",
    description: "Fallback error toast when updateBuildSource fails",
  },
  "buildSources.deleteSuccess": {
    message: 'Fonte de build "{name}" excluída',
    description: "Toast shown after deleting a Build Source",
  },
  "buildSources.deleteError": {
    message: "Não foi possível excluir a fonte de build. Tente novamente.",
    description: "Fallback error toast when deleteBuildSource fails",
  },
  "buildSources.triggerBuildSuccess": {
    message: "Build iniciado",
    description: "Toast shown after triggering a Build Source build",
  },
  "buildSources.triggerBuildError": {
    message: "Não foi possível iniciar o build. Tente novamente.",
    description: "Fallback error toast when triggerBuild fails",
  },
  "buildSources.createPermissionDenied": {
    message:
      "Você precisa de permissão de Developer ou Admin neste workspace para criar uma fonte de build.",
    description: "Read-only notice shown on /build-sources/new when can_create is false",
  },
  "buildSources.createBlockedSource": {
    message: "Escolha um repositório Git ou imagem Docker para continuar.",
    description: "Action bar hint while no source is selected",
  },
  "buildSources.partiallyLinkedTitle": {
    message: "Fonte de build criada com vínculos parciais",
    description: "Alert heading when createBuildSource returns 409 BUILD_SOURCE_PARTIALLY_LINKED",
  },
  "buildSources.partiallyLinkedDescription": {
    message:
      "A fonte de build {id} foi criada, mas nem todos os serviços selecionados puderam ser vinculados.",
    description: "Alert summary naming the newly created bds- ID on partial link conflict",
  },
  "buildSources.partiallyLinkedSucceeded": {
    message: "Vinculados: {services}",
    description: "List of services that were successfully linked before the conflict",
  },
  "buildSources.partiallyLinkedFailed": {
    message: "Falha ao vincular: {service}",
    description: "The service whose link failed during create",
  },
  "buildSources.fieldSource": {
    message: "Fonte",
    description: "Create form label for Git repository vs Docker image picker",
  },
  "buildSources.fieldSourceDescription": {
    message:
      "Escolha um repositório GitHub conectado, uma URL Git pública ou uma imagem Docker existente.",
    description: "Help text for the source picker row on /build-sources/new",
  },
  "buildSources.fieldName": {
    message: "Nome",
    description: "Create form label for Build Source name",
  },
  "buildSources.fieldNameDescription": {
    message: "Nome único dentro deste workspace.",
    description: "Help text for Build Source name",
  },
  "buildSources.fieldNamePlaceholder": {
    message: "my-shared-api",
    description: "Placeholder for Build Source name input",
  },
  "buildSources.errorNameTaken": {
    message: 'Já existe uma fonte de build chamada "{name}" neste workspace.',
    description: "Inline error when BUILD_SOURCE_NAME_TAKEN is returned",
  },
  "buildSources.fieldProject": {
    message: "Projeto",
    description: "Create form label for optional Project scope",
  },
  "buildSources.fieldProjectDescription": {
    message: "Opcionalmente restrinja esta fonte de build a um único projeto.",
    description: "Help text for optional Project selector",
  },
  "buildSources.fieldProjectNone": {
    message: "Nenhum (todo o workspace)",
    description: "Option leaving the build source unscoped at workspace level",
  },
  "buildSources.fieldRuntime": {
    message: "Runtime",
    description: "Create form label for Git build source runtime",
  },
  "buildSources.fieldRuntimeDescription": {
    message: "Escolha um runtime nativo ou Docker.",
    description: "Help text for Git build source runtime selector",
  },
  "buildSources.fieldBranch": {
    message: "Branch",
    description: "Create form label for Git branch",
  },
  "buildSources.fieldBranchDescription": {
    message:
      "Branch Git usada para compilar. Por padrão usa a branch principal do repositório.",
    description: "Help text for Git branch input",
  },
  "buildSources.fieldRegion": {
    message: "Região",
    description: "Create form label for Git build source region",
  },
  "buildSources.fieldRegionDescription": {
    message:
      "Fontes de build Git devem estar na mesma região dos serviços vinculados.",
    description: "Help text for Git build source region selector",
  },
  "buildSources.fieldRegionDefault": {
    message: "Região padrão do workspace",
    description: "Option leaving the build source region unset to use the platform default",
  },
  "buildSources.fieldGitRegistryDescription": {
    message:
      "Credencial opcional para baixar imagens base privadas durante builds Docker.",
    description: "Help text for registry credential on Docker Git build sources",
  },
  "buildSources.fieldEnvGroups": {
    message: "Grupos de ambiente no build",
    description: "Label for selecting workspace env groups in build-time environment",
  },
  "buildSources.fieldEnvGroupsDescription": {
    message:
      "Vincule grupos de ambiente no nível do workspace durante o build. Grupos restritos a um ambiente não são elegíveis.",
    description: "Help text for build-time env group checkboxes",
  },
  "buildSources.fieldEnvGroupsEmpty": {
    message: "Nenhum grupo de ambiente no nível do workspace disponível.",
    description: "Empty message when no workspace-level env groups exist",
  },
  "buildSources.linkServicesTitle": {
    message: "Vincular serviços existentes",
    description: "Section heading for optional initial serviceIds linking",
  },
  "buildSources.linkServicesDescription": {
    message:
      "Vincule opcionalmente Web Services, Private Services, Background Workers ou Cron Jobs existentes. Static Sites não aceitam fontes de build.",
    description: "Section description for optional initial serviceIds linking",
  },
  "buildSources.linkServicesEmpty": {
    message: "Ainda não há serviços vinculáveis neste workspace.",
    description: "Empty message when workspace has no linkable services",
  },
  "buildSources.projectSectionTitle": {
    message: "Fontes de build",
    description: "Heading of the Build Sources section on Project Overview",
  },
  "buildSources.projectLoadError": {
    message: "Não foi possível carregar as fontes de build deste projeto.",
    description: "Error text inside Project Overview Build Sources section",
  },
  "buildSources.projectEmptyDescription": {
    message: "Ainda não há fontes de build vinculadas a este projeto.",
    description: "Empty state text inside Project Overview Build Sources section",
  },
  "buildSources.viewBuildSource": {
    message: "Abrir fonte de build",
    description: "Button navigating to a newly created Build Source detail page",
  },
  "buildSources.detailTabsLabel": {
    message: "Seções da fonte de build",
    description: "Accessible label for the Build Source detail tab navigation",
  },
  "buildSources.tabBuilds": {
    message: "Compilações",
    description: "Tab label for Build Source build history",
  },
  "buildSources.tabServices": {
    message: "Serviços",
    description: "Tab label for Build Source linked services",
  },
  "buildSources.tabEnvironment": {
    message: "Ambiente",
    description: "Tab label for Build Source build-time environment",
  },
  "buildSources.tabSettings": {
    message: "Configurações",
    description: "Tab label for Build Source settings and danger zone",
  },
  "buildSources.triggerBuildButton": {
    message: "Iniciar build",
    description: "Primary CTA to trigger a new build on a Git build source",
  },
  "buildSources.triggerSubmitting": {
    message: "Iniciando…",
    description: "Submit button label while triggerBuild mutation is in flight",
  },
  "buildSources.triggerDialogTitle": {
    message: "Iniciar build",
    description: "Title of the Trigger Build modal dialog",
  },
  "buildSources.triggerDialogDescription": {
    message: 'Inicie um novo build para "{name}" e escolha quais serviços vinculados devem implantar o resultado.',
    description: "Description of the Trigger Build modal dialog",
  },
  "buildSources.triggerScopeLegend": {
    message: "Implantar após o build ser concluído",
    description: "Legend for the deploy.serviceScope radio group",
  },
  "buildSources.triggerScopeAutoDeploy": {
    message: "Implantar automaticamente serviços vinculados",
    description: "Radio option for deploy.serviceScope=autodeploy_only",
  },
  "buildSources.triggerScopeAutoDeployHint": {
    message: "Implanta todos os serviços vinculados com auto-deploy ativado.",
    description: "Hint text for deploy.serviceScope=autodeploy_only",
  },
  "buildSources.triggerScopeNone": {
    message: "Apenas compilar (sem implantar)",
    description: "Radio option for deploy.serviceScope=none",
  },
  "buildSources.triggerScopeNoneHint": {
    message: "Gera o artefato do build sem implantar nenhum serviço vinculado.",
    description: "Hint text for deploy.serviceScope=none",
  },
  "buildSources.triggerScopeCustom": {
    message: "Serviços selecionados",
    description: "Radio option for deploy.serviceScope=custom",
  },
  "buildSources.triggerScopeCustomHint": {
    message: "Escolha serviços vinculados específicos para implantar após o sucesso do build.",
    description: "Hint text for deploy.serviceScope=custom",
  },
  "buildSources.triggerScopeCustomNoServices": {
    message: "Ainda não há serviços vinculados a esta fonte de build.",
    description: "Message inside custom deploy scope when serviceLinks is empty",
  },
  "buildSources.triggerScopeCustomSelectOne": {
    message: "Selecione pelo menos um serviço para implantar.",
    description: "Validation hint when custom scope has zero checked services",
  },
  "buildSources.triggerClearCache": {
    message: "Limpar cache de build",
    description: "Checkbox label for clearCache=clear",
  },
  "buildSources.triggerClearCacheHint": {
    message: "Compila do zero sem reutilizar camadas em cache.",
    description: "Hint text for clearCache checkbox",
  },
  "buildSources.triggerResultTitle": {
    message: "Build iniciado",
    description: "Confirmation heading in Trigger Build dialog after success",
  },
  "buildSources.triggerResultDescription": {
    message: "O build {id} foi colocado na fila.",
    description: "Confirmation summary naming the new bld- ID",
  },
  "buildSources.triggerResultDeploysTitle_one": {
    message: "{count} implantação de serviço na fila",
    description: "Singular heading listing fan-out deploys queued by triggerBuild",
  },
  "buildSources.triggerResultDeploysTitle_many": {
    message: "{count} de implantações de serviço na fila",
    description: "Many heading listing fan-out deploys queued by triggerBuild",
  },
  "buildSources.triggerResultDeploysTitle_other": {
    message: "{count} implantações de serviço na fila",
    description: "Plural heading listing fan-out deploys queued by triggerBuild",
  },
  "buildSources.triggerResultNoDeploys": {
    message: "Nenhuma implantação de serviço foi enfileirada para este build.",
    description: "Message when triggerBuild returned an empty deploys array",
  },
  "buildSources.triggerResultSkippedTitle_one": {
    message: "{count} serviço vinculado ignorado",
    description: "Singular alert heading when triggerBuild returns skippedServices",
  },
  "buildSources.triggerResultSkippedTitle_many": {
    message: "{count} de serviços vinculados ignorados",
    description: "Many alert heading when triggerBuild returns skippedServices",
  },
  "buildSources.triggerResultSkippedTitle_other": {
    message: "{count} serviços vinculados ignorados",
    description: "Plural alert heading when triggerBuild returns skippedServices",
  },
  "buildSources.triggerResultViewLogs": {
    message: "Ver logs do build",
    description: "Primary action navigating from Trigger Build confirmation to Build Detail",
  },
  "buildSources.skippedReasonSuspended": {
    message: "Suspenso",
    description: "Badge label when a service was skipped because it is suspended",
  },
  "buildSources.buildStatusSucceeded": {
    message: "Concluído",
    description: "Build status badge for succeeded builds",
  },
  "buildSources.buildStatusFailed": {
    message: "Falhou",
    description: "Build status badge for failed builds",
  },
  "buildSources.buildStatusCanceled": {
    message: "Cancelado",
    description: "Build status badge for canceled builds",
  },
  "buildSources.buildStatusInProgress": {
    message: "Compilando",
    description: "Build status badge for in-progress builds",
  },
  "buildSources.buildStatusCreated": {
    message: "Na fila",
    description: "Build status badge for newly queued builds",
  },
  "buildSources.buildStatusUnknown": {
    message: "Desconhecido",
    description: "Fallback build status badge",
  },
  "buildSources.buildsEmptyTitle": {
    message: "Nenhum build ainda",
    description: "Empty state heading on the Builds tab",
  },
  "buildSources.buildsEmptyGitDescription": {
    message: "Inicie um build manualmente ou faça push para a branch acompanhada para gerar o primeiro artefato.",
    description: "Empty state description on Builds tab for Git build sources",
  },
  "buildSources.buildsEmptyImageDescription": {
    message: "Fontes de imagem Docker registram revisões de imagem quando os serviços vinculados são implantados.",
    description: "Empty state description on Builds tab for image build sources",
  },
  "buildSources.buildsRefreshError": {
    message: "Não foi possível atualizar os builds. Exibindo resultados em cache.",
    description: "Inline error banner when polling builds fails over cached rows",
  },
  "buildSources.buildsLoadMore": {
    message: "Carregar mais builds",
    description: "Pagination button loading older builds",
  },
  "buildSources.colBuildId": {
    message: "ID do build",
    description: "Table header for build ID column",
  },
  "buildSources.colStatus": {
    message: "Situação",
    description: "Table header for build status column",
  },
  "buildSources.colArtifact": {
    message: "Commit / Imagem",
    description: "Table header for commit SHA or image digest column",
  },
  "buildSources.colDuration": {
    message: "Duração",
    description: "Table header for build duration column",
  },
  "buildSources.colDeploys": {
    message: "Implantações",
    description: "Table header for fan-out deploys count column",
  },
  "buildSources.colStarted": {
    message: "Iniciado",
    description: "Table header for build start timestamp column",
  },
  "buildSources.buildDeploysCount_one": {
    message: "{count} implantação",
    description: "Singular badge count of fan-out deploys on a build",
  },
  "buildSources.buildDeploysCount_many": {
    message: "{count} de implantações",
    description: "Many badge count of fan-out deploys on a build",
  },
  "buildSources.buildDeploysCount_other": {
    message: "{count} implantações",
    description: "Plural badge count of fan-out deploys on a build",
  },
  "buildSources.buildSkippedCount_one": {
    message: "{count} ignorado",
    description: "Singular badge count of skipped services on a build",
  },
  "buildSources.buildSkippedCount_many": {
    message: "{count} de ignorados",
    description: "Many badge count of skipped services on a build",
  },
  "buildSources.buildSkippedCount_other": {
    message: "{count} ignorados",
    description: "Plural badge count of skipped services on a build",
  },
  "buildSources.backToBuilds": {
    message: "Voltar para Compilações",
    description: "Back link from Build Detail to the Builds tab",
  },
  "buildSources.copyBuildId": {
    message: "Copiar ID do build",
    description: "Accessible label for copying a bld- ID",
  },
  "buildSources.buildDetailSubtitle": {
    message: "Build de {name}",
    description: "Subtitle under the bld- ID on Build Detail",
  },
  "buildSources.buildDetailRefreshError": {
    message: "Não foi possível atualizar os detalhes do build. Exibindo o último estado carregado.",
    description: "Inline error banner when build detail polling fails",
  },
  "buildSources.factStarted": {
    message: "Iniciado",
    description: "Fact label for build start timestamp",
  },
  "buildSources.factCommit": {
    message: "Revisão Git",
    description: "Fact label for build commit SHA",
  },
  "buildSources.factArtifact": {
    message: "Artefato",
    description: "Fact label for built image reference and digest",
  },
  "buildSources.fanoutTitle": {
    message: "Implantações de serviços vinculados",
    description: "Card heading listing fan-out deploys on Build Detail",
  },
  "buildSources.fanoutEmpty": {
    message: "Nenhum serviço foi implantado a partir deste build ainda.",
    description: "Empty message inside fan-out deploys card",
  },
  "buildSources.buildLogsSearchLabel": {
    message: "Filtrar logs do build",
    description: "Accessible label for search input in BuildLogPanel",
  },
  "buildSources.buildLogsEmptyTitle": {
    message: "Ainda não há logs de build",
    description: "Empty state heading in BuildLogPanel",
  },
  "buildSources.buildLogsEmptyBody": {
    message: "Os logs deste build aparecerão aqui assim que ele começar a gerar saída.",
    description: "Empty state body in BuildLogPanel",
  },
  "buildSources.imageBuildLogsNotice": {
    message: "Fontes de imagem Docker reutilizam uma imagem existente e não geram logs de build.",
    description: "Notice shown on Build Detail for image-backed build sources",
  },
  "buildSources.servicesTabTitle": {
    message: "Serviços vinculados",
    description: "Heading of the Linked Services tab",
  },
  "buildSources.servicesTabDescription": {
    message: "Os serviços vinculados a esta fonte de build implantam os artefatos produzidos pelos seus builds.",
    description: "Description of the Linked Services tab",
  },
  "buildSources.linkExistingService": {
    message: "Vincular serviço existente",
    description: "Button opening the Link Existing Service dialog",
  },
  "buildSources.servicesEmptyTitle": {
    message: "Nenhum serviço vinculado",
    description: "Empty state heading on Linked Services tab",
  },
  "buildSources.servicesEmptyDescription": {
    message: "Vincule Web Services, Private Services, Background Workers ou Cron Jobs existentes para compartilhar builds desta fonte.",
    description: "Empty state description on Linked Services tab",
  },
  "buildSources.autoDeployOn": {
    message: "Auto-deploy ativado",
    description: "Badge indicating a linked service auto-deploys new builds",
  },
  "buildSources.autoDeployOff": {
    message: "Auto-deploy desativado",
    description: "Badge indicating a linked service requires manual deploys",
  },
  "buildSources.changeOrUnlinkSource": {
    message: "Alterar origem",
    description: "Link to a service's settings page to switch or unlink its source",
  },
  "buildSources.linkDialogTitle": {
    message: "Vincular serviço existente",
    description: "Heading of the Link Existing Service dialog",
  },
  "buildSources.linkDialogDescription": {
    message: 'Vincule um serviço existente do espaço de trabalho a "{name}". A vinculação salva a origem sem disparar uma implantação.',
    description: "Description of the Link Existing Service dialog",
  },
  "buildSources.linkDialogNoEligible": {
    message: "Todos os serviços elegíveis neste espaço de trabalho já estão vinculados ou ainda não há nenhum.",
    description: "Message when no linkable unlinked services exist",
  },
  "buildSources.linkDialogServiceLabel": {
    message: "Serviço",
    description: "Select label in Link Existing Service dialog",
  },
  "buildSources.linkDialogServicePlaceholder": {
    message: "Escolha um serviço…",
    description: "Select placeholder in Link Existing Service dialog",
  },
  "buildSources.linkDialogRegionMismatch": {
    message: "Região incompatível: esta fonte Git está em {sourceRegion}, enquanto o serviço selecionado está em {serviceRegion}.",
    description: "Inline error when linking a Git build source to a service in another region",
  },
  "buildSources.linkDialogStartCommandHint": {
    message: "Obrigatório para fontes com runtime nativo para indicar como o serviço deve iniciar.",
    description: "Help text for startCommand input in Link Existing Service dialog",
  },
  "buildSources.linkDialogSubmit": {
    message: "Vincular serviço",
    description: "Submit button label in Link Existing Service dialog",
  },
  "buildSources.envImageUnsupportedTitle": {
    message: "O ambiente de build não está disponível para fontes de imagem Docker",
    description: "Fallback heading if an image build source visits /environment",
  },
  "buildSources.envImageUnsupportedBody": {
    message: "Apenas fontes de build baseadas em Git executam builds com variáveis de ambiente e arquivos secretos.",
    description: "Fallback body if an image build source visits /environment",
  },
  "buildSources.envSaveOnlyNote": {
    message: "As alterações são salvas sem iniciar um build e entram em vigor no próximo build.",
    description: "Top note on Build-time Environment tab",
  },
  "buildSources.envVarsDescription": {
    message: "Disponíveis durante os builds desta fonte. As alterações entram em vigor no próximo build.",
    description: "Environment Variables card description on Build-time Environment tab",
  },
  "buildSources.secretFilesDescription": {
    message: "Montados durante os builds desta fonte. As alterações entram em vigor no próximo build.",
    description: "Secret Files card description on Build-time Environment tab",
  },
  "buildSources.envGroupsPanelDescription": {
    message: "Grupos de ambiente do espaço de trabalho vinculados ao build. As alterações se aplicam no próximo build sem implantar.",
    description: "Environment Groups card description on Build-time Environment tab",
  },
  "buildSources.envGroupsNoneLinked": {
    message: "Nenhum grupo de ambiente está vinculado a esta fonte de build.",
    description: "Empty state when no env groups are linked to a build source",
  },
  "buildSources.settingsIdentityTitle": {
    message: "Identidade e escopo",
    description: "Heading of read-only identity card on Settings tab",
  },
  "buildSources.settingsIdentityDescription": {
    message: "Identificadores imutáveis e escopo desta fonte de build.",
    description: "Description of read-only identity card on Settings tab",
  },
  "buildSources.settingsFieldId": {
    message: "ID da fonte de build",
    description: "Label for read-only bds- ID field on Settings tab",
  },
  "buildSources.settingsConfigTitle": {
    message: "Configuração de build",
    description: "Heading of editable settings card on Settings tab",
  },
  "buildSources.settingsNoBuildNote": {
    message: "Salvar as configurações atualiza esta fonte de build sem iniciar um build.",
    description: "Inline note explaining Settings edits do not rebuild",
  },
  "buildSources.settingsFieldRepoUrl": {
    message: "URL do repositório",
    description: "Input label for Git repository URL on Settings tab",
  },
  "buildSources.settingsInvalidRepoUrl": {
    message: "Informe uma URL de repositório Git válida.",
    description: "Validation error for malformed repo URL on Settings tab",
  },
  "buildSources.settingsInvalidImageUrl": {
    message: "Informe uma referência de imagem Docker válida.",
    description: "Validation error for malformed image URL on Settings tab",
  },
  "buildSources.settingsSaveButton": {
    message: "Salvar alterações",
    description: "Submit button label on Settings tab",
  },
  "buildSources.settingsSaving": {
    message: "Salvando…",
    description: "Submit button label while updateBuildSource is in flight",
  },
  "buildSources.dangerZoneTitle": {
    message: "Zona de perigo",
    description: "Heading of Delete Build Source card on Settings tab",
  },
  "buildSources.dangerZoneDescription": {
    message: "Exclua permanentemente esta fonte de build e sua configuração de build. Esta ação não pode ser desfeita.",
    description: "Description of Delete Build Source card",
  },
  "buildSources.deleteBlockedTitle_one": {
    message: "Desvincule {count} serviço antes de excluir",
    description: "Singular alert heading blocking delete when services remain linked",
  },
  "buildSources.deleteBlockedTitle_many": {
    message: "Desvincule {count} de serviços antes de excluir",
    description: "Many alert heading blocking delete when services remain linked",
  },
  "buildSources.deleteBlockedTitle_other": {
    message: "Desvincule {count} serviços antes de excluir",
    description: "Plural alert heading blocking delete when services remain linked",
  },
  "buildSources.deleteBlockedDescription": {
    message: "Uma fonte de build não pode ser excluída enquanto houver serviços vinculados a ela. Mude primeiro cada serviço vinculado para outra origem:",
    description: "Explanation under delete-blocked alert",
  },
  "buildSources.deleteButton": {
    message: "Excluir fonte de build",
    description: "Destructive button opening the delete confirmation dialog",
  },
  "buildSources.deleteConfirmTitle": {
    message: 'Excluir "{name}"?',
    description: "Title of Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmBody": {
    message: "Isso exclui permanentemente a fonte de build e seu ambiente de build. O histórico e a configuração não podem ser recuperados.",
    description: "Body of Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmPrompt": {
    message: "Digite {phrase} abaixo para confirmar.",
    description: "Sudo command prompt inside Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmButton": {
    message: "Excluir fonte de build",
    description: "Confirm action label inside Delete Build Source confirmation dialog",
  },
};

export default ptBuildSources;
