import type { TranslationEntry } from "@/i18n";

const ptProjects: Record<string, TranslationEntry> = {
  "projects.overviewTitle": {
    message: "Visão geral",
    description: "Page title of the Overview page (workspace homepage)",
  },
  "projects.projectsHeading": {
    message: "Projetos",
    description: "Section heading above the Overview page's project grid",
  },
  "projects.ungroupedHeading": {
    message: "Recursos sem grupo",
    description:
      "Section heading above the Overview page's ungrouped resource table",
  },
  "projects.allResourcesHeading": {
    message: "Todos os recursos",
    description:
      "Heading above a project page's full resource table, below the environments section",
  },
  "projects.filterTypeLabel": {
    message: "Tipo de recurso",
    description: "Accessible label for project environment type filters",
  },
  "projects.filterAll": {
    message: "Todos",
    description: "All project environment resource types",
  },
  "projects.filterServices": {
    message: "Serviços",
    description: "Service resource filter",
  },
  "projects.filterDatabases": {
    message: "Bancos de dados",
    description: "Database resource filter",
  },
  "projects.filterKeyValues": {
    message: "Key Values",
    description: "Key-value resource filter",
  },
  "projects.filterEnvGroups": {
    message: "Grupos de ambiente",
    description: "Environment-group resource filter",
  },
  "projects.searchLabel": {
    message: "Pesquisar recursos em {environment}",
    description: "Accessible project environment resource-search label",
  },
  "projects.searchPlaceholder": {
    message: "Pesquisar recursos em {environment}",
    description: "Project environment resource-search placeholder",
  },
  "projects.noMatchesTitle": {
    message: "Nenhum recurso correspondente",
    description: "Project environment filtered no-results title",
  },
  "projects.noMatchesBody": {
    message: "Tente outra pesquisa ou outro tipo de recurso.",
    description: "Project environment filtered no-results body",
  },
  "projects.eyebrow": {
    message: "Projeto",
    description: "Small uppercase label above a project page's name heading",
  },
  "projects.cardEmpty": {
    message: "Nenhum recurso ainda",
    description: "Project card status line when the project has no resources",
  },
  "projects.cardHealthy": {
    message: "Todos os recursos em execução",
    description: "Project card status line when every resource is healthy",
  },
  "projects.cardInProgress_one": {
    message: "{count} recurso em andamento",
    description:
      "Project card status line when resources are provisioning or deploying without a failure (singular)",
  },
  "projects.cardInProgress_many": {
    message: "{count} de recursos em andamento",
    description:
      "Project card status line when resources are provisioning or deploying without a failure (plural)",
  },
  "projects.cardInProgress_other": {
    message: "{count} recursos em andamento",
    description:
      "Project card status line when resources are provisioning or deploying without a failure (plural)",
  },
  "projects.cardUnhealthy_one": {
    message: "{count} recurso precisa de atenção",
    description: "Project card status line when one resource is unhealthy",
  },
  "projects.cardUnhealthy_many": {
    message: "{count} de recursos precisam de atenção",
    description:
      "Project card status line when more than one resource is unhealthy",
  },
  "projects.cardUnhealthy_other": {
    message: "{count} recursos precisam de atenção",
    description:
      "Project card status line when more than one resource is unhealthy",
  },
  "projects.newProjectCard": {
    message: "Criar novo projeto",
    description:
      'The dashed "+ create project" tile at the end of the project grid',
  },
  "projects.errorTitle": {
    message: "Não foi possível carregar os recursos",
    description: "Error state heading on the unified Projects page",
  },
  "projects.errorBody": {
    message:
      "Algo deu errado ao carregar seus recursos. Tente novamente em instantes.",
    description: "Error state body on the unified Projects page",
  },
  "projects.emptyTitle": {
    message: "Nenhum recurso ainda",
    description: "Empty state heading on the unified Projects page",
  },
  "projects.emptyBody": {
    message: "Crie um serviço, um banco de dados ou um Key Value para começar.",
    description: "Empty state body on the unified Projects page",
  },
  "projects.emptyCreateService": {
    message: "Criar um serviço",
    description: "Primary action of the empty Projects page (ADR075 D1)",
  },
  "projects.newButton": {
    message: "Novo",
    description: "Button that opens the new-resource menu",
  },
  "projects.newService": {
    message: "Novo serviço",
    description: "New-resource menu item that starts the service create wizard",
  },
  "projects.newDatabase": {
    message: "Novo banco de dados",
    description: "New-resource menu item that opens the database create dialog",
  },
  "projects.newKeyValue": {
    message: "Novo Key Value",
    description: "New-resource menu item that starts the key value create form",
  },
  "projects.colName": {
    message: "Nome",
    description: "Merged resource table column header",
  },
  "projects.colType": {
    message: "Tipo",
    description: "Merged resource table column header",
  },
  "projects.colStatus": {
    message: "Status",
    description: "Merged resource table column header",
  },
  "projects.colCreated": {
    message: "Criado em",
    description: "Merged resource table column header",
  },
  "projects.colRuntime": {
    message: "Runtime",
    description:
      "Authoritative runtime/product column in a Project resource table",
  },
  "projects.colRegion": {
    message: "Região",
    description:
      "Explicit installation placement column in a Project resource table",
  },
  "projects.colUpdated": {
    message: "Atualizado em",
    description:
      "Authoritative resource update-time column in a Project resource table",
  },
  "projects.colActions": {
    message: "Ações",
    description: "Merged resource table column header (screen-reader only)",
  },
  "projects.typeService": {
    message: "Serviço",
    description: "Type badge for a service row in the merged resource table",
  },
  "projects.typeDatabase": {
    message: "Banco de dados",
    description: "Type badge for a database row in the merged resource table",
  },
  "projects.typeKeyValue": {
    message: "Key Value",
    description: "Type badge for a key value row in the merged resource table",
  },
  "projects.typeEnvGroup": {
    message: "Grupo de ambiente",
    description: "Type badge for an environment-group row",
  },
  "projects.metadataNotApplicable": {
    message: "Não se aplica",
    description:
      "Accessible label for a status, runtime or region cell on an environment group row, which has none of them (w4/549)",
  },
  "projects.metadataUnavailable": {
    message: "Não disponível",
    description:
      "Accessible label for an unavailable runtime, region, status, or timestamp",
  },
  "projects.selectAllVisible": {
    message: "Selecionar todos os recursos visíveis",
    description: "Accessible label for the Project table header checkbox",
  },
  "projects.selectResource": {
    message: "Selecionar {name}",
    description: "Accessible label for one Project resource row checkbox",
  },
  "projects.selectedCount": {
    message: "Selecionados: {count}",
    description: "Bulk action toolbar selection count",
  },
  "projects.moveTargetLabel": {
    message: "Mover os recursos selecionados para o ambiente",
    description: "Accessible label for the bulk Move target selector",
  },
  "projects.moveTargetPlaceholder": {
    message: "Escolha o ambiente",
    description: "Placeholder for the bulk Move target selector",
  },
  "projects.moveSelected": {
    message: "Mover",
    description:
      "Bulk action that moves selected resources to another Environment",
  },
  "projects.bulkResume": {
    message: "Retomar",
    description:
      "Selection toolbar action that resumes every selected suspended resource (Render's bulk Resume, w4/m550)",
  },
  "projects.bulkSuspend": {
    message: "Suspender",
    description:
      "Selection toolbar action that suspends every selected running resource (Render's bulk Suspend, w4/m550)",
  },
  "projects.bulkAdminOnly": {
    message: "Esta ação é restrita a administradores.",
    description:
      "Tooltip on a disabled bulk Suspend/Resume for a non-admin in a protected environment (Render's copy)",
  },
  "projects.bulkSuspendTitle": {
    message: "Suspender serviços",
    description: "Title of the bulk Suspend dialog",
  },
  "projects.bulkSuspendBody": {
    message:
      "Os serviços a seguir serão suspensos e deixarão de funcionar imediatamente. Você pode desfazer isso retomando os serviços.",
    description:
      "Body of the bulk Suspend dialog, above the list of resources it suspends",
  },
  "projects.bulkSuspendConfirm": {
    message: "Suspender serviços",
    description: "Confirm button of the bulk Suspend dialog",
  },
  "projects.bulkResumeTitle": {
    message: "Retomar serviços",
    description: "Title of the bulk Resume dialog",
  },
  "projects.bulkResumeBody": {
    message: "Os serviços a seguir serão retomados:",
    description:
      "Body of the bulk Resume dialog, above the list of resources it resumes",
  },
  "projects.bulkResumeConfirm": {
    message: "Retomar serviços",
    description: "Confirm button of the bulk Resume dialog",
  },
  "projects.bulkRetry": {
    message: "Tentar novamente",
    description:
      "Button on a bulk Suspend/Resume row whose call failed; runs that row again",
  },
  "projects.bulkRetryRow": {
    message: "Tentar {name} novamente",
    description:
      "Accessible label of a row's Retry button ({name} = the resource)",
  },
  "projects.bulkRowSuspending": {
    message: "Suspendendo",
    description:
      "Screen-reader state of a bulk Suspend row whose call is in flight",
  },
  "projects.bulkRowSuspended": {
    message: "Suspenso",
    description: "Screen-reader state of a bulk Suspend row that succeeded",
  },
  "projects.bulkRowResuming": {
    message: "Retomando",
    description:
      "Screen-reader state of a bulk Resume row whose call is in flight",
  },
  "projects.bulkRowResumed": {
    message: "Retomado",
    description: "Screen-reader state of a bulk Resume row that succeeded",
  },
  "projects.bulkRowError": {
    message: "A solicitação falhou. Tente novamente.",
    description:
      "Fallback reason on a bulk Suspend/Resume row whose call failed without a server reason",
  },
  "projects.newProjectButton": {
    message: "Novo projeto",
    description: "Button that opens the new-project dialog",
  },
  "projects.createTitle": {
    message: "Novo projeto",
    description: "New-project dialog title",
  },
  "projects.createDescription": {
    message:
      "Agrupe serviços, bancos de dados e Key Values. Todo projeto começa com um ambiente.",
    description: "New-project dialog description",
  },
  "projects.fieldName": {
    message: "Nome",
    description: "New-project dialog name field label",
  },
  "projects.fieldFirstEnvironment": {
    message: "Nome do primeiro ambiente",
    description:
      "New-project dialog: name of the environment created with the project (w4/m264, Render parity)",
  },
  "projects.fieldNamePlaceholder": {
    message: "my-project",
    description: "New-project dialog name field placeholder",
  },
  "projects.createCancel": {
    message: "Cancelar",
    description: "New-project dialog cancel button",
  },
  "projects.createSubmit": {
    message: "Criar projeto",
    description: "New-project dialog submit button",
  },
  "projects.createSuccess": {
    message: "Projeto “{name}” criado.",
    description: "Toast shown after a project is created",
  },
  "projects.createError": {
    message: "Falha ao criar o projeto “{name}”.",
    description: "Toast shown when creating a project fails",
  },
  "projects.editNameButton": {
    message: "Editar",
    description:
      "Accessible label for the project page's inline pencil-edit button, and the Settings page's Project Name \"Edit\" button",
  },
  "projects.renameFieldLabel": {
    message: "Nome do projeto",
    description:
      'Accessible/visible label for the project rename field, on both the overview dialog and the Settings card. Deliberately more specific than projects.fieldName\'s bare "Name": these controls are identified by value alone otherwise (w4/134)',
  },
  "projects.renameTitle": {
    message: "Renomear projeto",
    description: "Rename-project dialog title",
  },
  "projects.renameSubmit": {
    message: "Salvar",
    description: "Rename-project dialog submit button",
  },
  "projects.renameSuccess": {
    message: "Projeto renomeado para “{name}”.",
    description: "Toast shown after a project is renamed",
  },
  "projects.renameRefreshFailed": {
    message:
      "Projeto renomeado para “{name}”, mas esta página não conseguiu atualizar. Tente novamente ou recarregue para ver o nome salvo.",
    description:
      "Warning after the rename saved but re-reading the project failed; the rename must not be retried or reported as failed (w4/m186)",
  },
  "projects.renameError": {
    message: "Falha ao renomear o projeto para “{name}”.",
    description: "Toast shown when renaming a project fails",
  },
  "projects.settingsTitle": {
    message: "Configurações do projeto",
    description: "Project settings page heading",
  },
  "projects.nameCardTitle": {
    message: "Nome do projeto",
    description: "Project settings page's name card title",
  },
  "projects.nameCardDescription": {
    message: "Um nome exclusivo para o seu projeto",
    description: "Project settings page's name card description",
  },
  "projects.deleteCardTitle": {
    message: "Excluir projeto",
    description: "Project settings page's danger-zone card title",
  },
  "projects.deleteCardDescription": {
    message:
      "Seus ambientes e todos os serviços, bancos de dados e Key Values deles são excluídos junto. Seus grupos de ambiente ficam sem atribuição. Esta ação não pode ser desfeita.",
    description: "Project settings page's danger-zone card description",
  },
  "projects.deleteCardButton": {
    message: "Excluir projeto",
    description:
      "Project settings page's danger-zone card button, and the delete-confirmation dialog's confirm button",
  },
  "projects.deleteConfirmTitle": {
    message: "Excluir o projeto “{name}”?",
    description: "Delete-project confirmation dialog title",
  },
  "projects.deleteConfirmBody": {
    message:
      "Seus ambientes e as configurações deles são excluídos. Seus grupos de ambiente ficam sem atribuição, não são excluídos.",
    description: "Delete-project confirmation dialog body",
  },
  "projects.deleteConfirmBodyResources": {
    message:
      "Excluir este projeto exclui todos os seus ambientes e os serviços, bancos de dados e Key Values deles: {resources}. Seus grupos de ambiente ficam sem atribuição, não são excluídos. Esta ação não pode ser desfeita.",
    description:
      "Delete-project confirmation body naming every service, database and key value store it deletes, as the environment delete does (w4/m264, w6/m290, w4/542)",
  },
  "projects.deleteSuccess": {
    message: "Projeto “{name}” excluído.",
    description: "Toast shown after a project is deleted",
  },
  "projects.deleteError": {
    message: "Falha ao excluir o projeto “{name}”.",
    description: "Toast shown when deleting a project fails",
  },
  "projects.moveToProject": {
    message: "Mover para o projeto",
    description:
      "Row-actions submenu label for assigning a resource to a project",
  },
  "projects.removeFromProject": {
    message: "Remover do projeto",
    description:
      "Row-actions submenu item that unassigns a resource from its project",
  },
  "projects.moveSuccess": {
    message: "“{name}” movido para “{project}”.",
    description: "Toast shown after moving a resource to a project",
  },
  "projects.moveError": {
    message: "Falha ao mover “{name}”.",
    description: "Toast shown when moving a resource to a project fails",
  },
  "projects.removeSuccess": {
    message: "“{name}” removido do projeto.",
    description: "Toast shown after unassigning a resource from its project",
  },
  "projects.removeError": {
    message: "Falha ao remover “{name}” do projeto.",
    description: "Toast shown when unassigning a resource fails",
  },
};

export default ptProjects;
