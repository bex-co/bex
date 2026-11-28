import type { TranslationEntry } from "@/i18n";

const ptCommon: Record<string, TranslationEntry> = {
  "common.navRouter": {
    message: "Router",
    description: "Router navigation item",
  },
  "common.protectedConfirmationTitle": {
    message: "Confirmação de ambiente protegido necessária",
    description:
      "Title for the retry dialog shown after a protected environment blocks a destructive action",
  },
  "common.protectedConfirmationBody": {
    message:
      "{name} pertence a um ambiente protegido. Digite o comando exato abaixo para continuar.",
    description:
      "Resource-neutral explanation for a protected environment confirmation gate",
  },
  "common.protectedConfirmationPrompt": {
    message: "Digite {phrase} abaixo para confirmar.",
    description:
      "Prompt naming the server-issued protected environment confirmation phrase",
  },
  "common.confirmPhrasePrompt": {
    message: "Para continuar, digite {phrase}",
    description:
      "Label above the typed-confirmation input in a phrase-gated confirm dialog.",
  },
  "common.close": {
    message: "Fechar",
    description:
      "Accessible name of the shared dialog's built-in close (X) button.",
  },
  "common.sidebarTitle": {
    message: "Barra lateral",
    description: "Screen-reader title of the mobile navigation drawer.",
  },
  "common.sidebarMobileDescription": {
    message: "Exibe a barra lateral para dispositivos móveis.",
    description: "Screen-reader description of the mobile navigation drawer.",
  },
  "common.skipToMainContent": {
    message: "Pular para o conteúdo principal",
    description:
      "Label of the first-tab skip link that moves keyboard focus past the sidebar and header to the page content.",
  },
  "common.sidebarToggle": {
    message: "Alternar barra lateral",
    description:
      "Accessible name of the button that opens or collapses the navigation sidebar.",
  },
  "common.sidebarResize": {
    message: "Redimensionar barra lateral",
    description: "Accessible name of the sidebar's draggable resize separator.",
  },
  "common.sidebarResizeHint": {
    message: "Arraste para redimensionar · clique duas vezes para alternar",
    description: "Hover tooltip on the sidebar's resize separator.",
  },
  "common.commandSuggestions": {
    message: "Sugestões",
    description:
      "Accessible name of a search or picker results list (the command menu's listbox).",
  },
  "common.comboboxPlaceholder": {
    message: "Selecione uma opção",
    description:
      "Fallback placeholder of a searchable picker when its caller supplies none.",
  },
  "common.comboboxEmpty": {
    message: "Nenhum resultado encontrado",
    description:
      "Fallback text when a searchable picker's filter matches no option.",
  },
  "common.comboboxUseCustom": {
    message: 'Pressione Enter para usar "{value}"',
    description:
      "Hint under a picker's empty results: Enter keeps the typed value as entered. {value} is the typed text.",
  },
  "common.comboboxClear": {
    message: "Limpar campo",
    description: "Accessible name of a searchable picker's clear (X) button.",
  },
  "common.cancel": {
    message: "Cancelar",
    description:
      "Generic Cancel button — dismisses a dialog or an inline form without saving.",
  },
  "common.pricePerRunMinute": {
    message: "${price}/min de tempo de execução",
    description:
      "A cron job's USD price per minute of run time, on a Change Compute Plan card (w4/546): it is billed per second only while a run executes; price is an API-supplied decimal without the currency symbol",
  },
  "common.pricePerMinute": {
    message: "${price}/min",
    description:
      "A cron job's USD price per minute of run time on a create-form plan card, short to fit beside the plan name; the form's hint says it is billed only while a run executes (w4/546). price is an API-supplied decimal without the currency symbol",
  },
  "common.pricePerMonth": {
    message: "${price}/mês",
    description:
      "Monthly USD price shown on a resource plan card; price is an API-supplied decimal without the currency symbol",
  },
  "common.reviewChangesTitle": {
    message: "Aplicar as alterações?",
    description:
      "Title of Render's plan review dialog, shown before a compute or storage change applies (w3/m674)",
  },
  "common.reviewChangesTotal": {
    message: "Total",
    description:
      "Plan review dialog: the label of the monthly total under the priced lines",
  },
  "common.reviewEditComputePlan": {
    message: "Editar plano de computação",
    description:
      "Plan review dialog: accessible name of the Compute plan line's Edit button, which returns to the plan picker",
  },
  "common.planUnavailable": {
    message: "Indisponível",
    description:
      "Shown in place of the price on a disabled plan card whose compute plan is paused (it cannot be selected for now); the server's full reason is the card's tooltip",
  },
  "common.planConnections_one": {
    message: "{formatted} conexão",
    description:
      "A plan card's connection-limit line, under its RAM: the client connections the plan accepts (Render's Key Value cards read \"50 Connections\"); {formatted} is the locale-formatted count",
  },
  "common.planConnections_many": {
    message: "{formatted} de conexões",
    description:
      "A plan card's connection-limit line, under its RAM: the client connections the plan accepts (Render's Key Value cards read \"50 Connections\"); {formatted} is the locale-formatted count",
  },
  "common.planConnections_other": {
    message: "{formatted} conexões",
    description:
      "A plan card's connection-limit line, under its RAM: the client connections the plan accepts (Render's Key Value cards read \"50 Connections\"); {formatted} is the locale-formatted count",
  },
  "common.protectedConfirmationRetry": {
    message: "Confirmar e salvar",
    description:
      "Confirm button in the protected environment retry dialog raised by a settings save",
  },
  "common.protectedConfirmationCancel": {
    message: "Cancelar",
    description:
      "Cancel button in the protected environment confirmation dialog",
  },
  "common.navEnvGroups": {
    message: "Grupos de ambiente",
    description: "Workspace sidebar link to environment groups",
  },
  "common.navBuildSources": {
    message: "Fontes de build",
    description: "Workspace sidebar link to build sources",
  },
  "common.navSandboxGroups": {
    message: "Grupos de sandbox",
    description: "Workspace sidebar link to sandbox groups",
  },
  "common.appName": {
    message: "bex",
    description: "Product name shown in the dashboard chrome",
  },
  "common.headDescription": {
    message:
      "Faça deploy e opere aplicações na infraestrutura que você controla com o bex, a alternativa open source e nativa de IA ao Render.",
    description:
      "Generic dashboard description used in description, Open Graph, and Twitter metadata",
  },
  "common.headImageAlt": {
    message: "Logo do painel do bex",
    description: "Alternative text for the dashboard's generic social image",
  },
  "common.loading": {
    message: "Carregando…",
    description: "Generic loading state label",
  },
  "common.topbarBreadcrumbs": {
    message: "Trilha de navegação",
    description: "Accessible label for the dashboard topbar hierarchy",
  },
  "common.topbarSearch": {
    message: "Pesquisar",
    description: "Open the workspace-wide topbar search",
  },
  "common.copyCode": {
    message: "Copiar código",
    description: "Accessible name of the copy button on a code block",
  },
  "common.topbarSearchPlaceholder": {
    message: "Pesquisar páginas e recursos…",
    description: "Placeholder in the workspace-wide topbar command search",
  },
  "common.topbarSearchDescription": {
    message: "Pesquise páginas do painel e recursos do workspace.",
    description: "Accessible description for the topbar command search",
  },
  "common.topbarSearchEmpty": {
    message: "Nenhuma página ou recurso correspondente.",
    description: "Empty state in the workspace-wide topbar command search",
  },
  "common.copyField": {
    message: "Copiar {field}",
    description:
      "Accessible name for a copy control when the caller only has a field label (m101/t014)",
  },
  "common.topbarNavigation": {
    message: "Navegação",
    description: "Page-links group in the workspace-wide topbar search",
  },
  "common.topbarResources": {
    message: "Recursos",
    description: "Workspace-resources group in the topbar search",
  },
  "common.topbarNew": {
    message: "Novo",
    description: "Persistent topbar menu for creating a resource",
  },
  "common.topbarCreate": {
    message: "Criar um recurso",
    description: "Heading in the persistent topbar create menu",
  },
  "common.topbarHelp": {
    message: "Ajuda e recursos",
    description: "Accessible label for the topbar help menu",
  },
  "common.topbarDocumentation": {
    message: "Documentação",
    description: "Topbar help-menu link to bex documentation",
  },
  "common.topbarCliGuide": {
    message: "Guia da CLI",
    description: "Topbar help-menu link to the bex CLI guide",
  },
  "common.topbarRepository": {
    message: "Repositório do GitHub",
    description: "Topbar help-menu link to the bex source repository",
  },
  "common.topbarWorkspaceSettings": {
    message: "Configurações do workspace",
    description: "Workspace-settings label in topbar navigation and search",
  },
  "common.searchShortcutEnvVarsDocs": {
    message: "Variáveis de ambiente",
    description:
      "Search shortcut to the docs page on environment variables and secrets (Render's 'Environment variables' shortcut)",
  },
  "common.searchShortcutEnvVarsDocsDescription": {
    message: "Defina variáveis de ambiente e segredos para um serviço",
    description:
      "One-line description beside the environment variables docs search shortcut",
  },
  "common.searchShortcutCustomDomainsDocs": {
    message: "Domínios personalizados",
    description: "Search shortcut to the custom domains docs page",
  },
  "common.searchShortcutCustomDomainsDocsDescription": {
    message: "Adicione um domínio personalizado a um serviço",
    description:
      "One-line description beside the custom domains docs search shortcut",
  },
  "common.searchShortcutGitCredentials": {
    message: "Credenciais do Git",
    description:
      "Search shortcut to Account Settings → Integrations, where GitHub is connected",
  },
  "common.searchShortcutGitCredentialsDescription": {
    message: "Conecte o GitHub",
    description:
      "One-line description beside the Git credentials search shortcut; bex connects GitHub only",
  },
  "common.searchShortcutAuditLogs": {
    message: "Logs de auditoria",
    description: "Search shortcut to Workspace Settings → Audit Logs",
  },
  "common.searchShortcutAuditLogsDescription": {
    message: "Exporte logs de auditoria",
    description: "One-line description beside the Audit Logs search shortcut",
  },
  "common.searchShortcutApiKeys": {
    message: "Chaves de API",
    description: "Search shortcut to Account Settings → API Keys",
  },
  "common.searchShortcutApiKeysDescription": {
    message: "Crie chaves de API",
    description: "One-line description beside the API Keys search shortcut",
  },
  "common.searchShortcutSshKeys": {
    message: "Chaves SSH",
    description: "Search shortcut to Account Settings → SSH Public Keys",
  },
  "common.searchShortcutSshKeysDescription": {
    message: "Adicione chaves públicas SSH",
    description: "One-line description beside the SSH Keys search shortcut",
  },
  "common.searchShortcutCliTokens": {
    message: "Tokens da CLI",
    description:
      "Search shortcut to Account Settings → Connected agents, which lists the CLI's tokens",
  },
  "common.searchShortcutCliTokensDescription": {
    message: "Revise e revogue tokens da CLI",
    description: "One-line description beside the CLI Tokens search shortcut",
  },
  "common.searchShortcutRegistryCredentials": {
    message: "Credenciais de registros de contêiner",
    description: "Search shortcut to Workspace Settings → Registry Credentials",
  },
  "common.searchShortcutRegistryCredentialsDescription": {
    message: "Gerencie credenciais para imagens externas",
    description:
      "One-line description beside the container registry credentials search shortcut",
  },
  "common.searchShortcutMcpServer": {
    message: "Servidor MCP",
    description: "Search shortcut to the MCP server docs page",
  },
  "common.searchShortcutMcpServerDescription": {
    message: "Gerencie seus serviços bex a partir de apps de IA",
    description: "One-line description beside the MCP Server search shortcut",
  },
  "common.topbarSwitchProject": {
    message: "Trocar de projeto",
    description: "Heading in a project breadcrumb dropdown",
  },
  "common.topbarSwitchEnvironment": {
    message: "Trocar de ambiente",
    description: "Heading in an environment breadcrumb dropdown",
  },
  "common.topbarSwitchService": {
    message: "Trocar de serviço",
    description: "Heading in a service breadcrumb dropdown",
  },
  "common.topbarAllResources": {
    message: "Todos os recursos",
    description: "Service breadcrumb menu link back to workspace resources",
  },
  "common.topbarProjectResource": {
    message: "Projeto",
    description: "Project kind label in workspace-wide search results",
  },
  "common.topbarServiceResource": {
    message: "Serviço",
    description: "Service kind label in workspace-wide search results",
  },
  "common.sudoCommandLabel": {
    message: "Comando sudo",
    description:
      "Input label on Render-style destructive type-to-confirm gates (live capture: docs/render-artifacts/workspace-lifecycle.md)",
  },
  "common.navIntegrationsGroup": {
    message: "Integrações",
    description:
      "Sidebar nav section label (webhooks, notifications) — Render's grouping",
  },
  "common.navWorkspaceGroup": {
    message: "Workspace",
    description:
      "Sidebar nav section label (usage, settings) — Render's grouping",
  },
  "common.navMonitorGroup": {
    message: "Monitoramento",
    description:
      "Service sidebar section label (logs, metrics) — Render's grouping",
  },
  "common.navWebhooks": {
    message: "Webhooks",
    description: "Sidebar nav link to the outbound event webhooks page",
  },
  "common.labelInvalid": {
    message:
      "Insira um nome de até 100 caracteres, sem quebras de linha nem caracteres de controle.",
    description:
      "Inline validation under a free-form name field (project, environment, group, webhook, credential, Blueprint)",
  },
  "common.navNotifications": {
    message: "Notificações",
    description: "Sidebar nav link to the notification settings page",
  },
  "common.navProjects": {
    message: "Projetos",
    description:
      "Sidebar nav link to the unified projects page (services, databases, key value grouped together)",
  },
  "common.navUsage": {
    message: "Faturamento",
    description:
      "Sidebar nav link to the workspace billing page (/billing, renamed from Usage in w5/m70; key kept to avoid churn)",
  },
  "common.navBlueprints": {
    message: "Blueprints",
    description:
      "Sidebar nav link to the blueprints management page (IaC stacks auto-registered on deploy)",
  },
  "common.navAgents": {
    message: "Agentes",
    description:
      "Sidebar nav link to the cloud coding-agent sessions page (/agents)",
  },
  "common.navAutomations": {
    message: "Automações",
    description:
      "Sidebar nav link to the scheduled agent automations page (/automations)",
  },
  "common.navSettings": {
    message: "Configurações",
    description:
      "Sidebar nav link to the workspace settings page (the sidebar is workspace-scoped; account settings hang off the user menu)",
  },
  "common.navBackToDashboard": {
    message: "Painel",
    description:
      "Contextual project sidebar's back link label, shown on a project's Overview page (links up to the workspace Overview)",
  },
  "common.navBackToProject": {
    message: "Projeto",
    description:
      "Contextual project sidebar's back link label, shown on a project's Settings page (links up to the project's Overview)",
  },
  "common.navBackToEnvironment": {
    message: "Ambiente",
    description:
      "Contextual project sidebar's back link label, shown on an environment's settings page (links up to the environment's section on the project page, Render's words, w1/m683)",
  },
  "common.navProjectOverview": {
    message: "Visão geral",
    description:
      "Contextual project sidebar nav link to the project's own Overview page",
  },
  "common.navObservabilityGroup": {
    message: "Observabilidade",
    description:
      "Contextual project sidebar section label above the project's log views (Render's OBSERVABILITY group, w3/m505)",
  },
  "common.navManageGroup": {
    message: "Gerenciar",
    description: "Contextual project sidebar section label above Settings",
  },
  "common.changeLanguage": {
    message: "Alterar idioma",
    description: "Accessible label for the language switcher button",
  },
  "common.userMenuSettings": {
    message: "Configurações da conta",
    description:
      "User menu item that navigates to account settings (/settings)",
  },
  "common.userMenuTheme": {
    message: "Tema",
    description: "User menu submenu label for theme selection",
  },
  "common.userMenuLanguage": {
    message: "Idioma",
    description: "User menu submenu label for language selection",
  },
  "common.userMenuThemeLight": {
    message: "Claro",
    description: "Light theme option label",
  },
  "common.userMenuThemeDark": {
    message: "Escuro",
    description: "Dark theme option label",
  },
  "common.userMenuThemeSystem": {
    message: "Sistema",
    description: "System theme option label",
  },
  "common.themeNotSaved": {
    message: "Seu tema não foi salvo. O tema anterior foi restaurado.",
    description:
      "Toast when a theme change could not be saved to the account (w3/m531); the previous theme is put back",
  },
  "common.userMenuLogOut": {
    message: "Sair",
    description: "User menu item that signs the user out",
  },
  "common.userMenuOpen": {
    message: "Menu da conta",
    description:
      "Accessible name of the avatar button that opens the account menu (w4/407)",
  },
  "common.userMenuOpenFor": {
    message: "Menu da conta de {name}",
    description:
      "Accessible name of the avatar button that opens the account menu, naming the signed-in identity (w4/407)",
  },
  "common.goHome": {
    message: "Ir para o início",
    description: "Button label that navigates back to the home page",
  },
  "common.goBack": {
    message: "Voltar",
    description: "Button label that navigates to the previous page",
  },
  "common.planCatalogUnavailable": {
    message:
      "Não foi possível carregar os planos, então nenhum pode ser escolhido por enquanto. Suas outras entradas foram mantidas.",
    description:
      "A plan picker whose catalog read failed or returned no plans; shown with a Try again button that re-reads only the catalog (w4/m218)",
  },
  "common.tryAgain": {
    message: "Tentar novamente",
    description: "Button label that retries after an error",
  },
  "common.notFoundTitle": {
    message: "Página não encontrada",
    description: "404 page heading",
  },
  "common.notFoundDescription": {
    message: "A página que você procura não existe ou foi movida.",
    description: "404 page explanatory text",
  },
  "common.errorTitle": {
    message: "Algo deu errado",
    description: "Generic error page heading",
  },
  "common.errorDefaultMessage": {
    message: "Algo deu errado.",
    description: "Fallback error message when none is provided",
  },
  "common.loadErrorRetry": {
    message:
      "Algo deu errado ao carregar. Atualize a página para tentar novamente.",
    description:
      "Body of a panel whose read failed with no server reason to show (a network failure or throttling)",
  },
  "common.serviceNotFoundTitle": {
    message: "Serviço não encontrado",
    description:
      "Title of a service page whose id leads nowhere (Render's ResourceNotFound, w3/m706)",
  },
  "common.serviceDeletedTitle": {
    message: "Serviço excluído",
    description:
      "Title of a service page whose service the dashboard had read and is now gone (w3/m706)",
  },
  "common.cronJobNotFoundTitle": {
    message: "Cron job não encontrado",
    description: "Title of a cron job page whose id leads nowhere (w3/m706)",
  },
  "common.cronJobDeletedTitle": {
    message: "Cron job excluído",
    description:
      "Title of a cron job page whose cron job the dashboard had read and is now gone (w3/m706)",
  },
  "common.sandboxGroupNotFoundTitle": {
    message: "Grupo de sandbox não encontrado",
    description:
      "Title of a sandbox group page whose id leads nowhere (w3/m706)",
  },
  "common.sandboxGroupDeletedTitle": {
    message: "Grupo de sandbox excluído",
    description:
      "Title of a sandbox group page whose group the dashboard had read and is now gone (w3/m706)",
  },
  "common.deployNotFoundTitle": {
    message: "Deploy não encontrado",
    description:
      "Title of a deploy page whose deploy id leads nowhere under a live service (w3/m706)",
  },
  "common.buildNotFoundTitle": {
    message: "Compilação não encontrada",
    description:
      "Title of a build page whose build id leads nowhere under a live build source (w3/m706)",
  },
  "common.resourceNotFoundBody": {
    message:
      "Não encontramos {id}. Ele pode ter sido excluído ou o link pode estar incorreto.",
    description:
      "Body of a resource page whose id leads nowhere; {id} is the id, drawn as a code chip (w3/m706)",
  },
  "common.resourceDeletedBody": {
    message: "{name} ({id}) foi excluído.",
    description:
      "Body of a resource page whose resource the dashboard had read and is now gone; {name} is its name, {id} its id (w3/m706)",
  },
  "common.resourceNotFoundForBody": {
    message: "Não encontramos {id} para {parent}. O link pode estar incorreto.",
    description:
      "Body of a deploy or build page whose id leads nowhere; {id} is the id, {parent} the service or build source it was opened under (w3/m706)",
  },
  "common.goToProject": {
    message: "Ir para o projeto",
    description:
      "Button on a missing resource's page that opens the project it was in (w3/m706)",
  },
  "common.returnHome": {
    message: "Voltar ao início",
    description:
      "Button on a missing, denied or failed resource's page that opens the home page (w3/m706)",
  },
  "common.viewDeploys": {
    message: "Ver deploys",
    description:
      "Button on a missing deploy's page that opens its service's deploys (w3/m706)",
  },
  "common.viewBuilds": {
    message: "Ver compilações",
    description:
      "Button on a missing build's page that opens its build source's builds (w3/m706)",
  },
  "common.goToWebhooks": {
    message: "Ir para Webhooks",
    description:
      "Button on a missing or denied webhook's page that opens the webhooks list (w3/m706)",
  },
  "common.resourceDoesNotExist": {
    message: "{id} não existe.",
    description:
      "Body of the Page not found state a resource page draws for a dead id; {id} is the id (w3/m706)",
  },
  "common.accessDeniedTitle": {
    message: "Acesso negado",
    description: "Title of a resource page whose read was refused (w3/m706)",
  },
  "common.accessDeniedBody": {
    message: "Você não tem acesso a {id}.",
    description:
      "Body of a resource page whose read was refused; {id} is the id (w3/m706)",
  },
  "common.accessDeniedBodyNoId": {
    message: "Você não tem acesso a este recurso.",
    description: "Body of a refused page that names no id (w3/m706)",
  },
  "common.resourceLoadFailedTitle": {
    message: "Ocorreu um erro",
    description: "Title of a resource page whose read failed (w3/m706)",
  },
  "common.resourceLoadFailedBody": {
    message: "Não foi possível carregar {id}.",
    description:
      "Body of a resource page whose read failed; {id} is the id (w3/m706)",
  },
  "common.copyId": {
    message: "Copiar ID",
    description:
      "Accessible name of the copy button beside an id in a resource state (w3/m706)",
  },
  "common.copyIdError": {
    message: "Não foi possível copiar o ID",
    description:
      "Toast when copying an id from a resource state fails (w3/m706)",
  },
  "common.resourceErrorBody": {
    message:
      "Não foi possível carregar o recurso. Verifique a API e tente novamente.",
    description:
      "Body text of a detail page's inline error state when the resource query failed",
  },
  "common.sessionExpiredTitle": {
    message: "Sua sessão expirou",
    description:
      "Heading of the inline state shown when a resource query fails with a 401 (expired session)",
  },
  "common.sessionExpiredBody": {
    message:
      "Entre novamente para continuar — você voltará direto para esta página.",
    description:
      "Body text of the expired-session inline state, reassuring the user their place is preserved",
  },
  "common.signIn": {
    message: "Entrar",
    description:
      "Button label that sends an expired session back to the login page",
  },
  "common.colActions": {
    message: "Ações",
    description:
      "Screen-reader-only header for a table's trailing row-actions column",
  },
  "common.justNow": {
    message: "agora mesmo",
    description:
      'Elapsed-time text for an instant under a minute old ("Deployed just now")',
  },
  "common.timeLocal": {
    message: "Local",
    description:
      "Timestamp tooltip: label of the row showing the instant in the viewer's local timezone",
  },
  "common.timeUtc": {
    message: "UTC",
    description:
      "Timestamp tooltip: label of the row showing the instant in UTC",
  },
  "common.timeUnix": {
    message: "Timestamp",
    description:
      "Timestamp tooltip: label of the row showing the instant as Unix milliseconds",
  },
  "common.timeFromPrevious": {
    message: "Desde a linha anterior",
    description:
      "Timestamp tooltip (w4/716): label of a log line's row showing the time since the line before it, Render's 'From previous'",
  },
  "common.timeFromStart": {
    message: "Desde o início",
    description:
      "Timestamp tooltip (w4/716): label of a deploy log line's row showing the time since the deploy began, Render's 'From start'",
  },
  "common.timeRowLabel": {
    message: "{label}:",
    description:
      "Timestamp tooltip (w4/716): a row's label followed by Render's colon; {label} is the zone ('PST'), UTC, Timestamp, From previous or From start",
  },
  "common.timeCopied": {
    message: "Copiado!",
    description:
      "Timestamp tooltip (w4/716): toast after a row's copy button copied its value, Render's 'Copied!'",
  },
  "common.timeCopyFailed": {
    message: "Não foi possível copiar o horário",
    description:
      "Timestamp tooltip (w4/716): toast when the clipboard refused a row's value",
  },
  "common.ipAllowListProxiedTitle_one": {
    message:
      "A lista de permissões não consegue ver os clientes em {count} domínio personalizado",
    description:
      "Title of the warning shown under an inbound IP allowlist when one of the custom domains it covers is proxied by Cloudflare (w1/m171)",
  },
  "common.ipAllowListProxiedTitle_many": {
    message:
      "A lista de permissões não consegue ver os clientes em {count} de domínios personalizados",
    description:
      "Title of the warning shown under an inbound IP allowlist when several custom domains it covers are proxied by Cloudflare (w1/m171)",
  },
  "common.ipAllowListProxiedTitle_other": {
    message:
      "A lista de permissões não consegue ver os clientes em {count} domínios personalizados",
    description:
      "Title of the warning shown under an inbound IP allowlist when several custom domains it covers are proxied by Cloudflare (w1/m171)",
  },
  "common.ipAllowListProxiedBody_one": {
    message:
      "Este domínio passa pelo proxy da Cloudflare, então a lista de permissões compara o endereço da Cloudflare, e não o do visitante. A lista é salva, mas neste domínio ela bloqueará visitantes que você pretendia permitir:",
    description:
      "Body of the Cloudflare-proxied allowlist warning, followed by the one proxied hostname (w1/m171)",
  },
  "common.ipAllowListProxiedBody_many": {
    message:
      "Estes domínios passam pelo proxy da Cloudflare, então a lista de permissões compara o endereço da Cloudflare, e não o do visitante. A lista é salva, mas nestes domínios ela bloqueará visitantes que você pretendia permitir:",
    description:
      "Body of the Cloudflare-proxied allowlist warning, followed by the list of proxied hostnames (w1/m171)",
  },
  "common.ipAllowListProxiedBody_other": {
    message:
      "Estes domínios passam pelo proxy da Cloudflare, então a lista de permissões compara o endereço da Cloudflare, e não o do visitante. A lista é salva, mas nestes domínios ela bloqueará visitantes que você pretendia permitir:",
    description:
      "Body of the Cloudflare-proxied allowlist warning, followed by the list of proxied hostnames (w1/m171)",
  },
  "common.ipAllowListProxiedFix": {
    message:
      "Para filtrar os endereços reais dos clientes, defina o registro DNS como somente DNS (nuvem cinza) na Cloudflare.",
    description:
      "How to fix the Cloudflare-proxied allowlist warning: switch the record from proxied (orange cloud) to DNS only (w1/m171)",
  },
  "common.inboundIpAllowedRanges_one": {
    message: "intervalo de IP é permitido de fora da sua rede privada.",
    description:
      "Status line of a Networking panel after a count badge (Render's, w4/m710): how many IPv4 ranges every level of inbound IP rules (workspace, environment, the resource's own) allows together",
  },
  "common.inboundIpAllowedRanges_many": {
    message: "de intervalos de IP são permitidos de fora da sua rede privada.",
    description:
      "Status line of a Networking panel after a count badge (Render's, w4/m710): how many IPv4 ranges every level of inbound IP rules (workspace, environment, the resource's own) allows together",
  },
  "common.inboundIpAllowedRanges_other": {
    message: "intervalos de IP são permitidos de fora da sua rede privada.",
    description:
      "Status line of a Networking panel after a count badge (Render's, w4/m710): how many IPv4 ranges every level of inbound IP rules (workspace, environment, the resource's own) allows together",
  },
  "common.inboundIpValidateLabel": {
    message: "Validar endereço IP",
    description:
      "Label of the Networking panel's address check field (Render's Validate IP address, w4/m710)",
  },
  "common.inboundIpCheckEmpty": {
    message: "Digite um endereço IP",
    description:
      "Validate IP address field, nothing typed yet (Render's tooltip, w4/m710)",
  },
  "common.inboundIpCheckInvalid": {
    message: "Este endereço IP não é válido",
    description:
      "Validate IP address field: the text is not an IPv4 address (Render's, w4/m710)",
  },
  "common.inboundIpCheckAllowed": {
    message: "O acesso é permitido para este endereço IP",
    description:
      "Validate IP address field: every level of inbound IP rules admits the address (Render's, w4/m710)",
  },
  "common.inboundIpRefusedWorkspace": {
    message:
      "O acesso não é permitido para este endereço IP pelas regras de IP de entrada do seu workspace.",
    description:
      "Validate IP address field: the workspace's inbound IP rules refuse the address (Render's, w4/m710)",
  },
  "common.inboundIpRefusedEnvironment": {
    message:
      "O acesso não é permitido para este endereço IP pelas regras de IP de entrada do seu ambiente.",
    description:
      "Validate IP address field: the environment's inbound IP rules refuse the address (Render's, w4/m710)",
  },
  "common.inboundIpRefusedOwn": {
    message:
      "O acesso não é permitido para este endereço IP pelas regras de IP de entrada do seu {type}.",
    description:
      "Validate IP address field: the resource's own (unsaved) inbound IP rules refuse the address; {type} is its type, e.g. Web Service, PostgreSQL, Key Value, Environment (Render's, w4/m710)",
  },
  "common.inboundIpAffectedBy": {
    message: "Afetado por:",
    description:
      "Label before the breadcrumb of a higher level of inbound IP rules that also applies to this resource (Render's 'Affected by:', w4/m710)",
  },
  "common.inboundIpLevelWorkspace": {
    message: "Workspace",
    description:
      "Breadcrumb item naming the workspace level of inbound IP rules (Render's, w4/m710)",
  },
  "common.inboundIpLevelEnvironment": {
    message: "Ambiente",
    description:
      "Breadcrumb item naming the environment level of inbound IP rules, and an environment's type in the own-rules heading and refusal (Render's, w4/m710)",
  },
  "common.inboundIpViewRules": {
    message: "Ver regras",
    description:
      "Link, opening in a new tab, to where a higher level of inbound IP rules is edited (Render's View rules, w4/m710)",
  },
  "common.inboundIpSource": {
    message: "Origem",
    description:
      "Column header of a read-only inbound IP rules table: the CIDR range (Render's Source, w4/m710)",
  },
  "common.inboundIpDescription": {
    message: "Descrição",
    description:
      "Column header of a read-only inbound IP rules table: the rule's description (Render's, w4/m710)",
  },
  "common.inboundIpBlockedWorkspace": {
    message:
      "As regras de IP de entrada do workspace bloqueiam todo o tráfego da internet.",
    description:
      "Warning in place of the table when the workspace has no inbound IP rule (Render's, w4/m710)",
  },
  "common.inboundIpBlockedEnvironment": {
    message:
      "As regras de IP de entrada do ambiente bloqueiam todo o tráfego da internet.",
    description:
      "Warning in place of the table when the environment has no inbound IP rule (Render's, w4/m710)",
  },
  "common.inboundIpOwnRules": {
    message: "Regras de IP de entrada de {type}",
    description:
      "Heading above a resource's own editable inbound IP rules; {type} is its type, e.g. Web Service, PostgreSQL, Key Value, Environment (Render's, w4/m710)",
  },
  "common.inboundIpTypePostgres": {
    message: "PostgreSQL",
    description:
      "A Postgres database's type as the inbound IP rules heading and refusal name it (Render's userFacingType, w4/m710)",
  },
  "common.inboundIpLoadError": {
    message:
      "Não foi possível carregar as regras de IP de entrada do workspace e do ambiente que também se aplicam. Atualize a página para tentar de novo.",
    description:
      "Shown in place of the workspace and environment inbound IP rules when one of them could not be read (w4/m710)",
  },
  "common.relativeAgeNow": {
    message: "agora",
    description: "Compact relative age under one minute (w4/m188)",
  },
  "common.relativeAgeMinutes": {
    message: "{n}min",
    description: "Compact relative age in whole minutes, e.g. 4m (w4/m188)",
  },
  "common.relativeAgeHours": {
    message: "{n}h",
    description: "Compact relative age in whole hours, e.g. 3h (w4/m188)",
  },
  "common.relativeAgeDays": {
    message: "{n}d",
    description: "Compact relative age in whole days, e.g. 5d (w4/m188)",
  },
  "common.relativeAgeMonths": {
    message: "{n}m",
    description: "Compact relative age in 30-day months, e.g. 2mo (w4/m188)",
  },
  "common.relativeAgeYears": {
    message: "{n}a",
    description: "Compact relative age in 365-day years, e.g. 1y (w4/m188)",
  },
  "common.relativeUntilNow": {
    message: "agora",
    description:
      "Countdown to a future instant less than one minute away (w4/m188)",
  },
  "common.relativeUntilMinutes": {
    message: "em {n}min",
    description: "Compact countdown in whole minutes, e.g. in 5m (w4/m188)",
  },
  "common.relativeUntilHours": {
    message: "em {n}h",
    description: "Compact countdown in whole hours, e.g. in 3h (w4/m188)",
  },
  "common.relativeUntilDays": {
    message: "em {n}d",
    description: "Compact countdown in whole days, e.g. in 83d (w4/m188)",
  },
  "common.openBilling": {
    message: "Abrir faturamento",
    description:
      "Link from a plan notice (a Pro-and-higher feature on Hobby) to the Billing page (w8/m138)",
  },
  "common.billingSettlingTitle": {
    message: "Finalizando o uso final…",
    description:
      "Title while a deletion waits for the workspace's final usage to be charged (BILLING_SETTLING); retried automatically",
  },
  "common.billingSettlingDescription": {
    message:
      "O bex está cobrando o uso final antes de excluir. Isso pode levar um ou dois minutos; ele tenta novamente de forma automática, então mantenha esta página aberta.",
    description:
      "Body while a deletion waits for the final usage to be charged (BILLING_SETTLING)",
  },
  "common.billingSettlingTimeout": {
    message:
      "O uso final ainda está sendo processado. Tente excluir novamente em alguns minutos.",
    description:
      "Deletion error after BILLING_SETTLING retries ran out (about three minutes)",
  },
  "common.balanceDueTitle": {
    message: "Saldo em aberto",
    description:
      "Title of the alert when a deletion is refused because the workspace has an unpaid balance (BILLING_BALANCE_DUE)",
  },
  "common.balanceDueAmount": {
    message:
      "Este workspace tem um saldo em aberto de {amount}. Pague a fatura e depois exclua-o novamente.",
    description: "Balance-due deletion refusal with the amount",
  },
  "common.balanceDue": {
    message:
      "Este workspace tem um saldo em aberto. Pague a fatura e depois exclua-o novamente.",
    description: "Balance-due deletion refusal without an amount",
  },
  "common.balanceDueNamedAmount": {
    message:
      "O workspace {name} tem um saldo em aberto de {amount}. Pague a fatura e depois exclua sua conta novamente.",
    description:
      "Account-deletion refusal naming the workspace with the unpaid balance and its amount",
  },
  "common.balanceDueNamed": {
    message:
      "O workspace {name} tem um saldo em aberto. Pague a fatura e depois exclua sua conta novamente.",
    description:
      "Account-deletion refusal naming the workspace with the unpaid balance",
  },
  "common.balanceDuePay": {
    message: "Pagar fatura",
    description: "Link to Stripe's hosted invoice page, opens in a new tab",
  },
  "common.regionFrankfurt": {
    message: "Frankfurt, Alemanha",
    description:
      "Display name of Render's `frankfurt` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionOhio": {
    message: "Ohio, EUA",
    description:
      "Display name of Render's `ohio` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionOregon": {
    message: "Oregon, EUA",
    description:
      "Display name of Render's `oregon` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionSingapore": {
    message: "Singapura",
    description:
      "Display name of Render's `singapore` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionVirginia": {
    message: "Virgínia, EUA",
    description:
      "Display name of Render's `virginia` region, shown where a resource's region is read (w4/693)",
  },
  "common.createOptional": {
    message: "Opcional",
    description:
      "Mark beside a create-page row label whose field may be left blank (Render's 'Optional')",
  },
  "common.createAdvanced": {
    message: "Avançado",
    description: "Heading of the collapsed Advanced card on a create page",
  },
  "common.planListShowAll": {
    message: "Mostrar todos os planos",
    description: "Button under a create page's plan list that shows every plan",
  },
  "common.planListShowFewer": {
    message: "Mostrar menos planos",
    description:
      "Button under an expanded plan list that folds it back to the first plans",
  },
  "common.createBlockedName": {
    message: "Insira um nome para continuar.",
    description:
      "Create page action bar: why Submit is held while the name is empty or invalid",
  },
  "common.createBlockedFields": {
    message: "Preencha ou corrija os campos acima para continuar.",
    description:
      "Create page action bar: why Submit is held while a field is missing or invalid",
  },
  "common.createTypePostgres": {
    message: "Postgres",
    description:
      "Postgres in the New menu and a create page's type switcher (Render's product name)",
  },
  "common.createTypeSwitcher": {
    message: "{type}: escolher um tipo de serviço",
    description:
      "Accessible name of the breadcrumb menu that switches a create page to another resource type ({type} = the current page's heading)",
  },
  "common.createTypeProject": {
    message: "Projeto",
    description: "Project in the New menu (Render's bare 'Project')",
  },
  "common.createTypeBlueprint": {
    message: "Blueprint",
    description: "Blueprint in the New menu (Render's bare 'Blueprint')",
  },
  "common.createTypeBuildSource": {
    message: "Fonte de build",
    description: "Build Source in the New menu",
  },
  "common.createTypeSandboxGroup": {
    message: "Grupo de sandbox",
    description:
      "Sandbox Group in the New menu (Render's bare 'Sandbox Group')",
  },
  "common.createTypeWebhook": {
    message: "Webhook",
    description:
      "Webhook in the New menu (bex's own create, after Render's entries)",
  },
  "common.createBlockedNameTaken": {
    message: "Escolha um nome que não esteja em uso para continuar.",
    description:
      "Create page action bar: why Submit is held while the name is already in use",
  },
  "common.createRequired": {
    message: "Obrigatório",
    description:
      "Mark beside a create-page row label whose field must be filled before Submit",
  },
  "common.createBlockedLoadingPlans": {
    message: "Carregando planos…",
    description:
      "Create page action bar: Submit waits only for the plan catalog to load (not the user's fault)",
  },
  "common.datastoreViewDocs": {
    message: "Ver documentação",
    description:
      "A Postgres's or Key Value's header link to bex's docs for it, opening in a new tab (Render's View docs, w3/m675)",
  },
  "common.datastoreConnect": {
    message: "Conectar",
    description:
      "A Postgres's or Key Value's header button opening its connection strings (Render's Connect, w3/m675)",
  },
  "common.datastoreConnectInternal": {
    message: "Interna",
    description:
      "Connect menu tab: the private-network URL (Render's Internal tab)",
  },
  "common.datastoreConnectExternal": {
    message: "Externa",
    description:
      "Connect menu tab: the public URL and the CLI command (Render's External tab)",
  },
  "common.datastoreConnectInternalBody": {
    message:
      "Todos os seus serviços da bex na mesma região podem {link} pela rede privada.",
    description:
      "Connect menu's Internal line; {link} is common.datastoreConnectInternalLink, linking the docs",
  },
  "common.datastoreConnectInternalLink": {
    message: "se comunicar internamente",
    description: "Linked words inside common.datastoreConnectInternalBody",
  },
  "common.datastoreConnectExternalBody": {
    message: "Conecte-se a partir de {link} da bex.",
    description:
      "Connect menu's External line once an IP rule admits traffic; {link} is common.datastoreConnectExternalLink, linking the docs",
  },
  "common.datastoreConnectExternalLink": {
    message: "serviços fora",
    description: "Linked words inside common.datastoreConnectExternalBody",
  },
  "common.datastoreConnectExternalClosed": {
    message:
      "Tráfego externo não permitido. Adicione endereços IP na seção {link}.",
    description:
      "Connect menu's External tab while no IP rule admits traffic (Render's note); {link} is the Info card holding the IP rules, named as that card is",
  },
  "common.datastoreConnectCliBody": {
    message: "Conecte-se usando a {link}",
    description:
      "Connect menu's line above the CLI command; {link} is common.datastoreConnectCliLink, linking the CLI docs",
  },
  "common.datastoreConnectCliLink": {
    message: "CLI da bex",
    description:
      "Linked words inside common.datastoreConnectCliBody: the bex command-line tool",
  },
  "common.datastoreConnectRetry": {
    message: "Falha ao carregar. Tentar novamente?",
    description:
      "Accessible name and tooltip of the retry button on a Connect menu value that failed to load",
  },
};

export default ptCommon;
