import type { TranslationEntry } from "@/i18n";

const ptEnvironments: Record<string, TranslationEntry> = {
  "environments.heading": {
    message: "Ambientes",
    description: "Section heading above a project page's environments list",
  },
  "environments.selectorLabel": {
    message: "Ambiente selecionado",
    description:
      "Accessible label for the Project overview Environment selector",
  },
  "environments.unassignedOption": {
    message: "Não atribuído",
    description: "Project resources that belong to no Environment",
  },
  "environments.newButton": {
    message: "Novo ambiente",
    description: "Button that opens the new-environment dialog",
  },
  "environments.emptyBody": {
    message:
      "Nenhum ambiente ainda. Crie um (ex.: staging ou production) para agrupar os recursos deste projeto.",
    description: "Empty state shown when a project has no environments",
  },
  "environments.errorBody": {
    message:
      "Algo deu errado ao carregar os ambientes. Tente novamente em instantes.",
    description: "Error state shown when the environments query fails",
  },
  "environments.refreshErrorBody": {
    message:
      "Não foi possível atualizar os ambientes. Exibindo os últimos dados carregados.",
    description:
      "Inline notice when a background refresh of environments fails while the previously loaded environments stay on screen",
  },
  "environments.resourceCount_one": {
    message: "{count} recurso",
    description:
      "Count of visible services+databases+key-value instances assigned to an environment, next to its name (singular)",
  },
  "environments.resourceCount_many": {
    message: "{count} de recursos",
    description:
      "Count of visible services+databases+key-value instances assigned to an environment, next to its name (plural)",
  },
  "environments.resourceCount_other": {
    message: "{count} recursos",
    description:
      "Count of visible services+databases+key-value instances assigned to an environment, next to its name (plural)",
  },
  "environments.manageButton": {
    message: "Gerenciar recursos",
    description:
      "Button on an environment card that opens the manage-resources dialog",
  },
  "environments.moreActions": {
    message: "Mais ações",
    description:
      "Accessible label for an environment card's overflow (•••) menu",
  },
  "environments.renameAction": {
    message: "Renomear",
    description: "Environment overflow-menu item that opens the rename dialog",
  },
  "environments.deleteAction": {
    message: "Excluir",
    description:
      "Environment overflow-menu item that opens the delete confirmation, and the confirm button",
  },
  "environments.cardEmpty": {
    message: "Nenhum recurso neste ambiente ainda.",
    description:
      "Shown inside an environment card when it has no assigned resources",
  },
  "environments.createTitle": {
    message: "Novo ambiente",
    description: "New-environment dialog title",
  },
  "environments.createDescription": {
    message:
      "Agrupe um subconjunto dos recursos deste projeto sob um nome, como staging ou production.",
    description: "New-environment dialog description",
  },
  "environments.fieldName": {
    message: "Nome",
    description: "New-environment dialog name field label",
  },
  "environments.fieldNamePlaceholder": {
    message: "staging",
    description: "New-environment dialog name field placeholder",
  },
  "environments.cancel": {
    message: "Cancelar",
    description: "Cancel button shared by the environment dialogs",
  },
  "environments.createSubmit": {
    message: "Criar ambiente",
    description: "New-environment dialog submit button",
  },
  "environments.createSuccess": {
    message: "Ambiente “{name}” criado.",
    description: "Toast shown after an environment is created",
  },
  "environments.createRefreshFailed": {
    message:
      "Ambiente “{name}” criado, mas não foi possível atualizar a lista. Tente novamente ou recarregue para vê-lo.",
    description:
      "Warning after the environment was created but re-reading the list failed; creation must not be retried or reported as failed (w4/231)",
  },
  "environments.createError": {
    message: "Falha ao criar o ambiente “{name}”.",
    description: "Toast shown when creating an environment fails",
  },
  "environments.renameFieldLabel": {
    message: "Nome do ambiente",
    description:
      'Accessible/visible label for the environment rename field. Deliberately more specific than environments.fieldName\'s bare "Name" (w4/134)',
  },
  "environments.renameTitle": {
    message: "Renomear ambiente",
    description: "Rename-environment dialog title",
  },
  "environments.renameSubmit": {
    message: "Salvar",
    description: "Rename-environment dialog submit button",
  },
  "environments.renameSuccess": {
    message: "Ambiente renomeado para “{name}”.",
    description: "Toast shown after an environment is renamed",
  },
  "environments.renameError": {
    message: "Falha ao renomear o ambiente para “{name}”.",
    description: "Toast shown when renaming an environment fails",
  },
  "environments.deleteConfirmTitle": {
    message: "Excluir o ambiente “{name}”?",
    description: "Delete-environment confirmation dialog title",
  },
  "environments.deleteConfirmBody": {
    message:
      "Os grupos de ambiente dele são mantidos, mas ficam sem atribuição: deixam de pertencer a este ambiente ou projeto e permanecem em Grupos de ambiente. Esta ação não pode ser desfeita.",
    description: "Delete-environment confirmation dialog body",
  },
  "environments.deleteConfirmBodyResources": {
    message:
      "Excluir este ambiente exclui todos os seus serviços, bancos de dados e instâncias de Key Value: {resources}. Os grupos de ambiente dele são mantidos, mas ficam sem atribuição: deixam de pertencer a este ambiente ou projeto e permanecem em Grupos de ambiente. Esta ação não pode ser desfeita.",
    description:
      "Delete-environment confirmation body when services, databases or key value stores would be deleted (w4/m264, w6/m290, Render parity)",
  },
  "environments.deleteSuccess": {
    message: "Ambiente “{name}” excluído.",
    description: "Toast shown after an environment is deleted",
  },
  "environments.deleteRefreshFailed": {
    message:
      "Ambiente “{name}” excluído, mas não foi possível atualizar a lista. Tente novamente ou recarregue para atualizá-la.",
    description:
      "Warning after the environment was deleted but re-reading the list failed; deletion must not be retried or reported as failed (w4/290)",
  },
  "environments.deleteError": {
    message: "Falha ao excluir o ambiente “{name}”.",
    description: "Toast shown when deleting an environment fails",
  },
  "environments.manageTitle": {
    message: "Gerenciar recursos em “{name}”",
    description: "Manage-resources dialog title",
  },
  "environments.manageDescription": {
    message:
      "Marque os recursos que pertencem a este ambiente. Atribuir um recurso também o adiciona a este projeto.",
    description: "Manage-resources dialog description",
  },
  "environments.tabServices": {
    message: "Serviços",
    description: "Manage-resources dialog tab label for the services checklist",
  },
  "environments.tabDatabases": {
    message: "Bancos de dados",
    description:
      "Manage-resources dialog tab label for the databases checklist",
  },
  "environments.tabKeyValues": {
    message: "Key Value",
    description:
      "Manage-resources dialog tab label for the key-value checklist",
  },
  "environments.manageNoServices": {
    message: "Este workspace ainda não tem serviços para atribuir.",
    description:
      "Manage-resources dialog empty state when the workspace has no services",
  },
  "environments.manageNoDatabases": {
    message: "Este workspace ainda não tem bancos de dados para atribuir.",
    description:
      "Manage-resources dialog empty state when the workspace has no databases",
  },
  "environments.manageNoKeyValues": {
    message:
      "Este workspace ainda não tem instâncias de Key Value para atribuir.",
    description:
      "Manage-resources dialog empty state when the workspace has no key-value instances",
  },
  "environments.manageResourceIn": {
    message: "Em {environment}",
    description:
      "Under a Manage resources row: the other environment the resource is in now ({environment} = its name)",
  },
  "environments.markerProtected": {
    message: "Protegido",
    description:
      "Marker: the environment the resource would leave is protected",
  },
  "environments.markerIsolated": {
    message: "Isolado",
    description:
      "Marker: the environment the resource would leave has network isolation",
  },
  "environments.markerAllowList": {
    message: "Lista de IPs permitidos",
    description:
      "Marker: the environment the resource would leave restricts inbound IPs",
  },
  "environments.manageMovesOut": {
    message: "Sai de {environment}: {resources}",
    description:
      "Before Save: the selected resources this save moves out of another environment ({resources} = comma-separated names)",
  },
  "environments.manageNeedsAdminToMove": {
    message: "Somente um administrador pode tirá-lo de {environment}.",
    description:
      "Why a Manage resources row is disabled: its current environment is admin-managed",
  },
  "environments.manageUnsaved": {
    message: "Alterações não salvas",
    description:
      "Accessible description of a Manage resources tab whose selection differs from the saved membership",
  },
  "environments.manageSubmit": {
    message: "Salvar",
    description: "Manage-resources dialog submit button",
  },
  "environments.assignSuccess": {
    message: "Serviços de “{name}” atualizados.",
    description: "Toast shown after an environment's services are updated",
  },
  "environments.strandedMoveTitle": {
    message: "Esta mudança deixa grupos vinculados para trás",
    description:
      "Confirmation title shown before a move that strands an environment-scoped group link (w4/m238)",
  },
  "environments.strandedMoveBody": {
    message:
      "Estes grupos de ambiente continuam vinculados ao serviço que está sendo movido, e suas variáveis e arquivos continuam sendo aplicados a ele até o grupo ser desvinculado ou movido:",
    description:
      "Confirmation body: the stranded groups keep feeding the service after the move",
  },
  "environments.strandedMoveConfirm": {
    message: "Mover mesmo assim",
    description: "Confirm button on the stranded-groups move confirmation",
  },
  "environments.strandedScopeWorkspace": {
    message: "Workspace",
    description:
      "Scope label for a stranded group that is scoped to the whole workspace",
  },
  "environments.strandedScopeUnknown": {
    message:
      "Ainda não foi possível verificar os grupos de ambiente vinculados — aguarde a lista carregar e tente novamente.",
    description:
      "Toast shown when a move is attempted before the group/scope reads resolve",
  },
  "environments.assignError": {
    message: "Falha ao atualizar os serviços de “{name}”.",
    description: "Toast shown when updating an environment's services fails",
  },
  "environments.assignDatabasesSuccess": {
    message: "Bancos de dados de “{name}” atualizados.",
    description: "Toast shown after an environment's databases are updated",
  },
  "environments.assignDatabasesError": {
    message: "Falha ao atualizar os bancos de dados de “{name}”.",
    description: "Toast shown when updating an environment's databases fails",
  },
  "environments.assignDatabasesRefreshFailed": {
    message:
      "Bancos de dados de “{name}” atualizados, mas não foi possível atualizar a lista. Tente novamente ou recarregue para vê-los.",
    description:
      "Warning after the database membership was saved but re-reading the environments list failed; the write must not be retried or reported as failed (w4/325)",
  },
  "environments.assignKeyValuesSuccess": {
    message: "Instâncias de Key Value de “{name}” atualizadas.",
    description:
      "Toast shown after an environment's key-value instances are updated",
  },
  "environments.assignKeyValuesRefreshFailed": {
    message:
      "Instâncias de Key Value de “{name}” atualizadas, mas não foi possível atualizar a lista. Tente novamente ou recarregue para vê-las.",
    description:
      "Warning after the key-value membership was saved but re-reading the environments list failed; the write must not be retried or reported as failed (w4/263)",
  },
  "environments.assignKeyValuesError": {
    message: "Falha ao atualizar as instâncias de Key Value de “{name}”.",
    description:
      "Toast shown when updating an environment's key-value instances fails",
  },
  "environments.tabEnvGroups": {
    message: "Grupos de ambiente",
    description: "Manage-resources dialog tab label for environment groups",
  },
  "environments.manageNoEnvGroups": {
    message: "Este workspace ainda não tem grupos de ambiente para atribuir.",
    description: "Environment-groups checklist empty state",
  },
  "environments.tabSandboxGroups": {
    message: "Grupos de sandbox",
    description:
      "Manage-resources dialog tab label for the workspace's sandbox groups",
  },
  "environments.manageNoSandboxGroups": {
    message: "Este workspace ainda não tem grupos de sandbox para atribuir.",
    description: "Sandbox-groups checklist empty state",
  },
  "environments.assignEnvGroupsSuccess": {
    message: "Grupos de ambiente de “{name}” atualizados.",
    description: "Toast shown after environment-group membership is updated",
  },
  "environments.assignEnvGroupsPlacementUnread": {
    message:
      "Não foi possível confirmar onde estão agora os grupos de “{name}”. Tente novamente ou recarregue antes de movê-los de novo.",
    description:
      "Warning after an environment-group move failed and re-reading the groups' placement failed too; its action re-reads, it never moves again (w4/m206)",
  },
  "environments.assignEnvGroupsError": {
    message: "Falha ao atualizar os grupos de ambiente de “{name}”.",
    description:
      "Toast shown when environment-group membership fails to update",
  },
  "environments.settingsAction": {
    message: "Todas as configurações",
    description: "Environment overflow-menu action opening ACL settings",
  },
  "environments.settingsTitle": {
    message: "Configurações de “{name}”",
    description: "Environment ACL settings dialog title",
  },
  "environments.settingsDescription": {
    message:
      "Gerencie permissões, isolamento da rede privada e regras de IP de entrada.",
    description: "Environment ACL settings dialog description",
  },
  "environments.protectedBadge": {
    message: "Protegido",
    description: "Badge shown beside a protected environment name",
  },
  "environments.protectedLabel": {
    message: "Ambiente protegido",
    description: "Protected-status setting label",
  },
  "environments.protectedHint": {
    message:
      "Somente administradores do workspace podem excluir, suspender, retomar ou ativar e desativar o modo de manutenção dos recursos deste ambiente.",
    description: "Protected-status setting explanation",
  },
  "environments.isolationLabel": {
    message: "Bloquear conexões entre ambientes",
    description: "Private-network isolation setting label",
  },
  "environments.isolationProOnly": {
    message:
      "O isolamento de rede exige o plano Pro ou superior. Desative-o aqui se ele foi ativado antes.",
    description: "Isolation setting explanation on a Hobby workspace (w8/m138)",
  },
  "environments.isolationHint": {
    message:
      "Impeça que o tráfego da rede privada cruze o limite deste ambiente, inclusive para suas instâncias de Postgres e Key Value. As URLs externas delas continuam seguindo suas regras de IP.",
    description: "Private-network isolation setting explanation",
  },
  "environments.ipAllowListLabel": {
    message: "Restrições de IP de entrada",
    description: "Environment inbound-IP allowlist heading",
  },
  "environments.ipAllowListHint": {
    message:
      "Somente os intervalos CIDR listados podem acessar os serviços públicos, Static Sites e datastores deste ambiente. Uma origem também precisa passar pelas regras de IP de cada recurso.",
    description: "Environment inbound-IP allowlist explanation",
  },
  "environments.ipAllowListOpen": {
    message:
      "Nenhuma regra — os membros públicos deste ambiente vão NEGAR todo o tráfego. Adicione 0.0.0.0/0 e ::/0 para permitir tudo.",
    description: "Empty environment inbound-IP allowlist state (deny-all)",
  },
  "environments.ipAllowListRemove": {
    message: "Remover {cidr}",
    description: "Accessible label for an inbound-IP rule remove button",
  },
  "environments.ipAllowListRuleCIDR": {
    message: "Bloco CIDR da regra {number}",
    description: "Accessible label for an existing inbound-IP rule CIDR",
  },
  "environments.ipAllowListRuleDescription": {
    message: "Descrição da regra {number}",
    description: "Accessible label for an existing inbound-IP rule description",
  },
  "environments.ipAllowListNewCIDR": {
    message: "Novo bloco CIDR",
    description: "Accessible label for a new inbound-IP rule CIDR",
  },
  "environments.ipAllowListNewDescription": {
    message: "Descrição da nova regra",
    description: "Accessible label for a new inbound-IP rule description",
  },
  "environments.ipAllowListDescriptionPlaceholder": {
    message: "Descrição (opcional)",
    description: "Placeholder for an inbound-IP rule description",
  },
  "environments.ipAllowListScalePlanOnly": {
    message:
      "As regras de IP de entrada para ambientes exigem o plano Scale ou superior. As regras já definidas continuam sendo aplicadas e podem ser redefinidas.",
    description:
      "Hint replacing the editable IP rules below the Scale plan (w8/m137)",
  },
  "environments.ipAllowListResetAllowAll": {
    message: "Redefinir para permitir tudo",
    description:
      "Button resetting an environment's inbound IP rules to allow-all below the Scale plan (w8/m137)",
  },
  "environments.ipAllowListInvalid": {
    message: "Informe blocos CIDR IPv4 ou IPv6 válidos e únicos.",
    description:
      "Inline validation when an environment allow list block is not a unique, valid CIDR",
  },
  "environments.ipAllowListAdd": {
    message: "Adicionar origem",
    description: "Button adding an inbound-IP CIDR rule",
  },
  "environments.settingsSave": {
    message: "Salvar",
    description: "Environment ACL settings save button",
  },
  "environments.aclSaveSuccess": {
    message: "Configurações de “{name}” salvas.",
    description: "Toast shown after an Environment ACL is saved",
  },
  "environments.aclSaveError": {
    message: "Falha ao salvar as configurações de “{name}”.",
    description: "Toast shown when an Environment ACL save fails",
  },
  "environments.resourceCountLoading": {
    message: "Contando…",
    description:
      "Environment card header while its resource lists are still loading, instead of a count",
  },
  "environments.resourceCountUnknown": {
    message: "Contagem indisponível",
    description:
      "Environment card header when a resource list has not answered, instead of a count",
  },
  "environments.resourcesUnavailable": {
    message:
      "Alguns recursos não puderam ser carregados, então este ambiente pode estar incompleto ou desatualizado.",
    description:
      "Inline alert on a Project environment card when a resource list failed to load; offers Try again",
  },
  "environments.resourcesLoading": {
    message: "Carregando recursos…",
    description:
      "Environment card line while its resource lists are still loading, instead of the empty message",
  },
  "environments.assignmentTitle": {
    message: "Projeto",
    description: "Shared create-form Project/Environment selector heading",
  },
  "environments.assignmentProject": {
    message: "Projeto",
    description: "Accessible label for a create-form Project selector",
  },
  "environments.assignmentProjectNone": {
    message: "Nenhum projeto",
    description: "Unassigned Project selector option",
  },
  "environments.assignmentEnvironment": {
    message: "Ambiente",
    description: "Accessible label for a create-form Environment selector",
  },
  "environments.assignmentEnvironmentNone": {
    message: "Nenhum ambiente",
    description: "Unassigned Environment selector option",
  },
  "environments.assignmentSelectedLoading": {
    message: "Carregando…",
    description:
      "Label for a kept Project/Environment selection while its list is still loading",
  },
  "environments.assignmentSelectedUnavailable": {
    message: "Selecionado (nome indisponível)",
    description:
      "Label for a kept Project/Environment selection whose list failed to load",
  },
  "environments.assignmentReadError": {
    message:
      "Não foi possível carregar os projetos ou ambientes. Tente novamente; qualquer seleção que você fez é mantida.",
    description:
      "Inline alert when the create form's Project or Environment list fails to load; the selection is kept",
  },
  "environments.assignmentHint": {
    message:
      "Ao criar um recurso, selecionar um projeto filtra os ambientes dele. Escolha um ambiente para adicionar o recurso a esse projeto. Deixar Nenhum ambiente selecionado cria o recurso fora de qualquer projeto. Se o projeto ainda não tem ambiente, crie um primeiro na página do projeto.",
    description: "Shared create-form Environment-assignment hint",
  },
  "environments.protectedPlacementAdminOnly": {
    message:
      "Somente um administrador pode criar recursos em um ambiente protegido ou movê-los para dentro ou para fora dele.",
    description:
      "Why a create or move control that targets (or leaves) a protected environment is disabled for a non-admin, and the message for the same server refusal (w3/m281)",
  },
  "environments.protectedOptionAdminOnly": {
    message: "{environment} (somente administradores)",
    description:
      "A protected environment in a create or move picker that a non-admin cannot choose ({environment} = its name)",
  },
  "environments.assignmentDescription": {
    message: "Adicione-o a um projeto e a um ambiente quando for criado.",
    description: "Shared create-form Project row description",
  },
  "environments.assignmentNoEnvironments": {
    message:
      "Este projeto ainda não tem ambientes. Crie um primeiro na página do projeto.",
    description:
      "Shared create-form note when the chosen Project has no Environment to add the resource to",
  },
  "environments.assignmentPickEnvironment": {
    message:
      "Escolha um ambiente para adicionar o recurso a este projeto. Com “Nenhum ambiente”, ele é criado fora de qualquer projeto.",
    description:
      "Shared create-form note when a Project is chosen but No environment is: only an Environment adds the resource to the Project",
  },
  "environments.assignmentEnvironmentPickProject": {
    message: "Escolha um projeto primeiro",
    description:
      "Shared create-form Environment select while no Project is chosen (it is disabled until one is)",
  },
};

export default ptEnvironments;
