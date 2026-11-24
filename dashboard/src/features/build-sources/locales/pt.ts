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
};

export default ptBuildSources;
