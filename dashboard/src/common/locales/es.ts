import type { TranslationEntry } from "@/i18n";

const esCommon: Record<string, TranslationEntry> = {
  "common.navRouter": {
    message: "Router",
    description: "Router navigation item",
  },
  "common.protectedConfirmationTitle": {
    message: "Se requiere confirmación del entorno protegido",
    description:
      "Title for the retry dialog shown after a protected environment blocks a destructive action",
  },
  "common.protectedConfirmationBody": {
    message:
      "{name} pertenece a un entorno protegido. Escribe el comando exacto que aparece abajo para continuar.",
    description:
      "Resource-neutral explanation for a protected environment confirmation gate",
  },
  "common.protectedConfirmationPrompt": {
    message: "Escribe {phrase} abajo para confirmar.",
    description:
      "Prompt naming the server-issued protected environment confirmation phrase",
  },
  "common.confirmPhrasePrompt": {
    message: "Para continuar, escribe {phrase}",
    description:
      "Label above the typed-confirmation input in a phrase-gated confirm dialog.",
  },
  "common.close": {
    message: "Cerrar",
    description:
      "Accessible name of the shared dialog's built-in close (X) button.",
  },
  "common.sidebarTitle": {
    message: "Barra lateral",
    description: "Screen-reader title of the mobile navigation drawer.",
  },
  "common.sidebarMobileDescription": {
    message: "Muestra la barra lateral móvil.",
    description: "Screen-reader description of the mobile navigation drawer.",
  },
  "common.skipToMainContent": {
    message: "Saltar al contenido principal",
    description:
      "Label of the first-tab skip link that moves keyboard focus past the sidebar and header to the page content.",
  },
  "common.sidebarToggle": {
    message: "Alternar barra lateral",
    description:
      "Accessible name of the button that opens or collapses the navigation sidebar.",
  },
  "common.sidebarResize": {
    message: "Cambiar el tamaño de la barra lateral",
    description: "Accessible name of the sidebar's draggable resize separator.",
  },
  "common.sidebarResizeHint": {
    message: "Arrastra para cambiar el tamaño · doble clic para alternar",
    description: "Hover tooltip on the sidebar's resize separator.",
  },
  "common.commandSuggestions": {
    message: "Sugerencias",
    description:
      "Accessible name of a search or picker results list (the command menu's listbox).",
  },
  "common.comboboxPlaceholder": {
    message: "Selecciona una opción",
    description:
      "Fallback placeholder of a searchable picker when its caller supplies none.",
  },
  "common.comboboxEmpty": {
    message: "No se encontraron resultados",
    description:
      "Fallback text when a searchable picker's filter matches no option.",
  },
  "common.comboboxUseCustom": {
    message: "Pulsa Intro para usar “{value}”",
    description:
      "Hint under a picker's empty results: Enter keeps the typed value as entered. {value} is the typed text.",
  },
  "common.comboboxClear": {
    message: "Borrar entrada",
    description: "Accessible name of a searchable picker's clear (X) button.",
  },
  "common.cancel": {
    message: "Cancelar",
    description:
      "Generic Cancel button — dismisses a dialog or an inline form without saving.",
  },
  "common.pricePerRunMinute": {
    message: "${price}/min de tiempo de ejecución",
    description:
      "A cron job's USD price per minute of run time, on a Change Compute Plan card (w4/546): it is billed per second only while a run executes; price is an API-supplied decimal without the currency symbol",
  },
  "common.pricePerMinute": {
    message: "${price}/min",
    description:
      "A cron job's USD price per minute of run time on a create-form plan card, short to fit beside the plan name; the form's hint says it is billed only while a run executes (w4/546). price is an API-supplied decimal without the currency symbol",
  },
  "common.pricePerMonth": {
    message: "${price}/mes",
    description:
      "Monthly USD price shown on a resource plan card; price is an API-supplied decimal without the currency symbol",
  },
  "common.planUnavailable": {
    message: "No disponible",
    description:
      "Shown in place of the price on a disabled plan card whose compute plan is paused (it cannot be selected for now); the server's full reason is the card's tooltip",
  },
  "common.planConnections_one": {
    message: "{formatted} conexión",
    description:
      "A plan card's connection-limit line, under its RAM: the client connections the plan accepts (Render's Key Value cards read \"50 Connections\"); {formatted} is the locale-formatted count",
  },
  "common.planConnections_many": {
    message: "{formatted} de conexiones",
    description:
      "A plan card's connection-limit line, under its RAM: the client connections the plan accepts (Render's Key Value cards read \"50 Connections\"); {formatted} is the locale-formatted count",
  },
  "common.planConnections_other": {
    message: "{formatted} conexiones",
    description:
      "A plan card's connection-limit line, under its RAM: the client connections the plan accepts (Render's Key Value cards read \"50 Connections\"); {formatted} is the locale-formatted count",
  },
  "common.protectedConfirmationRetry": {
    message: "Confirmar y guardar",
    description:
      "Confirm button in the protected environment retry dialog raised by a settings save",
  },
  "common.protectedConfirmationCancel": {
    message: "Cancelar",
    description:
      "Cancel button in the protected environment confirmation dialog",
  },
  "common.navEnvGroups": {
    message: "Grupos de entorno",
    description: "Workspace sidebar link to environment groups",
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
      "Despliega y opera aplicaciones en una infraestructura que controlas con bex, la alternativa a Render de código abierto y nativa de IA.",
    description:
      "Generic dashboard description used in description, Open Graph, and Twitter metadata",
  },
  "common.headImageAlt": {
    message: "Logotipo del panel de bex",
    description: "Alternative text for the dashboard's generic social image",
  },
  "common.loading": {
    message: "Cargando…",
    description: "Generic loading state label",
  },
  "common.topbarBreadcrumbs": {
    message: "Ruta de navegación",
    description: "Accessible label for the dashboard topbar hierarchy",
  },
  "common.topbarSearch": {
    message: "Buscar",
    description: "Open the workspace-wide topbar search",
  },
  "common.copyCode": {
    message: "Copiar código",
    description: "Accessible name of the copy button on a code block",
  },
  "common.topbarSearchPlaceholder": {
    message: "Busca páginas y recursos…",
    description: "Placeholder in the workspace-wide topbar command search",
  },
  "common.topbarSearchDescription": {
    message: "Busca páginas del panel y recursos del espacio de trabajo.",
    description: "Accessible description for the topbar command search",
  },
  "common.topbarSearchEmpty": {
    message: "No hay páginas ni recursos que coincidan.",
    description: "Empty state in the workspace-wide topbar command search",
  },
  "common.copyField": {
    message: "Copiar {field}",
    description:
      "Accessible name for a copy control when the caller only has a field label (m101/t014)",
  },
  "common.topbarNavigation": {
    message: "Navegación",
    description: "Page-links group in the workspace-wide topbar search",
  },
  "common.topbarResources": {
    message: "Recursos",
    description: "Workspace-resources group in the topbar search",
  },
  "common.topbarNew": {
    message: "Nuevo",
    description: "Persistent topbar menu for creating a resource",
  },
  "common.topbarCreate": {
    message: "Crear un recurso",
    description: "Heading in the persistent topbar create menu",
  },
  "common.topbarHelp": {
    message: "Ayuda y recursos",
    description: "Accessible label for the topbar help menu",
  },
  "common.topbarDocumentation": {
    message: "Documentación",
    description: "Topbar help-menu link to bex documentation",
  },
  "common.topbarCliGuide": {
    message: "Guía de la CLI",
    description: "Topbar help-menu link to the bex CLI guide",
  },
  "common.topbarRepository": {
    message: "Repositorio de GitHub",
    description: "Topbar help-menu link to the bex source repository",
  },
  "common.topbarWorkspaceSettings": {
    message: "Configuración del espacio de trabajo",
    description: "Workspace-settings label in topbar navigation and search",
  },
  "common.searchShortcutEnvVarsDocs": {
    message: "Variables de entorno",
    description:
      "Search shortcut to the docs page on environment variables and secrets (Render's 'Environment variables' shortcut)",
  },
  "common.searchShortcutEnvVarsDocsDescription": {
    message: "Define variables de entorno y secretos para un servicio",
    description:
      "One-line description beside the environment variables docs search shortcut",
  },
  "common.searchShortcutCustomDomainsDocs": {
    message: "Dominios personalizados",
    description: "Search shortcut to the custom domains docs page",
  },
  "common.searchShortcutCustomDomainsDocsDescription": {
    message: "Añade un dominio personalizado a un servicio",
    description:
      "One-line description beside the custom domains docs search shortcut",
  },
  "common.searchShortcutGitCredentials": {
    message: "Credenciales de Git",
    description:
      "Search shortcut to Account Settings → Integrations, where GitHub is connected",
  },
  "common.searchShortcutGitCredentialsDescription": {
    message: "Conecta GitHub",
    description:
      "One-line description beside the Git credentials search shortcut; bex connects GitHub only",
  },
  "common.searchShortcutAuditLogs": {
    message: "Registros de auditoría",
    description: "Search shortcut to Workspace Settings → Audit Logs",
  },
  "common.searchShortcutAuditLogsDescription": {
    message: "Exporta registros de auditoría",
    description: "One-line description beside the Audit Logs search shortcut",
  },
  "common.searchShortcutApiKeys": {
    message: "Claves de API",
    description: "Search shortcut to Account Settings → API Keys",
  },
  "common.searchShortcutApiKeysDescription": {
    message: "Crea claves de API",
    description: "One-line description beside the API Keys search shortcut",
  },
  "common.searchShortcutSshKeys": {
    message: "Claves SSH",
    description: "Search shortcut to Account Settings → SSH Public Keys",
  },
  "common.searchShortcutSshKeysDescription": {
    message: "Añade claves públicas SSH",
    description: "One-line description beside the SSH Keys search shortcut",
  },
  "common.searchShortcutCliTokens": {
    message: "Tokens de la CLI",
    description:
      "Search shortcut to Account Settings → Connected agents, which lists the CLI's tokens",
  },
  "common.searchShortcutCliTokensDescription": {
    message: "Revisa y revoca tokens de la CLI",
    description: "One-line description beside the CLI Tokens search shortcut",
  },
  "common.searchShortcutRegistryCredentials": {
    message: "Credenciales de registros de contenedores",
    description: "Search shortcut to Workspace Settings → Registry Credentials",
  },
  "common.searchShortcutRegistryCredentialsDescription": {
    message: "Gestiona credenciales para imágenes externas",
    description:
      "One-line description beside the container registry credentials search shortcut",
  },
  "common.searchShortcutMcpServer": {
    message: "Servidor MCP",
    description: "Search shortcut to the MCP server docs page",
  },
  "common.searchShortcutMcpServerDescription": {
    message: "Gestiona tus servicios de bex desde apps de IA",
    description: "One-line description beside the MCP Server search shortcut",
  },
  "common.topbarSwitchProject": {
    message: "Cambiar de proyecto",
    description: "Heading in a project breadcrumb dropdown",
  },
  "common.topbarSwitchEnvironment": {
    message: "Cambiar de entorno",
    description: "Heading in an environment breadcrumb dropdown",
  },
  "common.topbarSwitchService": {
    message: "Cambiar de servicio",
    description: "Heading in a service breadcrumb dropdown",
  },
  "common.topbarAllResources": {
    message: "Todos los recursos",
    description: "Service breadcrumb menu link back to workspace resources",
  },
  "common.topbarProjectResource": {
    message: "Proyecto",
    description: "Project kind label in workspace-wide search results",
  },
  "common.topbarServiceResource": {
    message: "Servicio",
    description: "Service kind label in workspace-wide search results",
  },
  "common.sudoCommandLabel": {
    message: "Comando sudo",
    description:
      "Input label on Render-style destructive type-to-confirm gates (live capture: docs/render-artifacts/workspace-lifecycle.md)",
  },
  "common.navIntegrationsGroup": {
    message: "Integraciones",
    description:
      "Sidebar nav section label (webhooks, notifications) — Render's grouping",
  },
  "common.navWorkspaceGroup": {
    message: "Espacio de trabajo",
    description:
      "Sidebar nav section label (usage, settings) — Render's grouping",
  },
  "common.navMonitorGroup": {
    message: "Monitorización",
    description:
      "Service sidebar section label (logs, metrics) — Render's grouping",
  },
  "common.navWebhooks": {
    message: "Webhooks",
    description: "Sidebar nav link to the outbound event webhooks page",
  },
  "common.labelInvalid": {
    message:
      "Escribe un nombre de hasta 100 caracteres, sin saltos de línea ni caracteres de control.",
    description:
      "Inline validation under a free-form name field (project, environment, group, webhook, credential, Blueprint)",
  },
  "common.navNotifications": {
    message: "Notificaciones",
    description: "Sidebar nav link to the notification settings page",
  },
  "common.navProjects": {
    message: "Proyectos",
    description:
      "Sidebar nav link to the unified projects page (services, databases, key value grouped together)",
  },
  "common.navUsage": {
    message: "Facturación",
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
    message: "Automatizaciones",
    description:
      "Sidebar nav link to the scheduled agent automations page (/automations)",
  },
  "common.navSettings": {
    message: "Configuración",
    description:
      "Sidebar nav link to the workspace settings page (the sidebar is workspace-scoped; account settings hang off the user menu)",
  },
  "common.navBackToDashboard": {
    message: "Panel",
    description:
      "Contextual project sidebar's back link label, shown on a project's Overview page (links up to the workspace Overview)",
  },
  "common.navBackToProject": {
    message: "Proyecto",
    description:
      "Contextual project sidebar's back link label, shown on a project's Settings page (links up to the project's Overview)",
  },
  "common.navProjectOverview": {
    message: "Resumen",
    description:
      "Contextual project sidebar nav link to the project's own Overview page",
  },
  "common.navObservabilityGroup": {
    message: "Observabilidad",
    description:
      "Contextual project sidebar section label above the project's log views (Render's OBSERVABILITY group, w3/m505)",
  },
  "common.navManageGroup": {
    message: "Administrar",
    description: "Contextual project sidebar section label above Settings",
  },
  "common.changeLanguage": {
    message: "Cambiar idioma",
    description: "Accessible label for the language switcher button",
  },
  "common.userMenuSettings": {
    message: "Configuración de la cuenta",
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
    message: "Oscuro",
    description: "Dark theme option label",
  },
  "common.userMenuThemeSystem": {
    message: "Sistema",
    description: "System theme option label",
  },
  "common.themeNotSaved": {
    message: "No se guardó tu tema. Se ha restaurado el tema anterior.",
    description:
      "Toast when a theme change could not be saved to the account (w3/m531); the previous theme is put back",
  },
  "common.userMenuLogOut": {
    message: "Cerrar sesión",
    description: "User menu item that signs the user out",
  },
  "common.userMenuOpen": {
    message: "Menú de la cuenta",
    description:
      "Accessible name of the avatar button that opens the account menu (w4/407)",
  },
  "common.userMenuOpenFor": {
    message: "Menú de la cuenta de {name}",
    description:
      "Accessible name of the avatar button that opens the account menu, naming the signed-in identity (w4/407)",
  },
  "common.goHome": {
    message: "Ir al inicio",
    description: "Button label that navigates back to the home page",
  },
  "common.goBack": {
    message: "Volver",
    description: "Button label that navigates to the previous page",
  },
  "common.planCatalogUnavailable": {
    message:
      "No se pudieron cargar los planes, así que todavía no se puede elegir ninguno. Tus demás datos se conservan.",
    description:
      "A plan picker whose catalog read failed or returned no plans; shown with a Try again button that re-reads only the catalog (w4/m218)",
  },
  "common.tryAgain": {
    message: "Intentar de nuevo",
    description: "Button label that retries after an error",
  },
  "common.notFoundTitle": {
    message: "Página no encontrada",
    description: "404 page heading",
  },
  "common.notFoundDescription": {
    message: "La página que buscas no existe o se ha movido.",
    description: "404 page explanatory text",
  },
  "common.errorTitle": {
    message: "Algo salió mal",
    description: "Generic error page heading",
  },
  "common.errorDefaultMessage": {
    message: "Algo salió mal.",
    description: "Fallback error message when none is provided",
  },
  "common.loadErrorRetry": {
    message:
      "Algo salió mal al cargar. Actualiza la página para intentarlo de nuevo.",
    description:
      "Body of a panel whose read failed with no server reason to show (a network failure or throttling)",
  },
  "common.resourceNotFoundToast": {
    message: "Ese recurso no existe o se eliminó.",
    description:
      "Toast shown after a dead resource URL redirects to the home page (w9/m55)",
  },
  "common.resourceErrorBody": {
    message:
      "No se pudo cargar el recurso. Comprueba la API e inténtalo de nuevo.",
    description:
      "Body text of a detail page's inline error state when the resource query failed",
  },
  "common.sessionExpiredTitle": {
    message: "Tu sesión ha caducado",
    description:
      "Heading of the inline state shown when a resource query fails with a 401 (expired session)",
  },
  "common.sessionExpiredBody": {
    message:
      "Inicia sesión de nuevo para continuar: volverás directamente a esta página.",
    description:
      "Body text of the expired-session inline state, reassuring the user their place is preserved",
  },
  "common.signIn": {
    message: "Iniciar sesión",
    description:
      "Button label that sends an expired session back to the login page",
  },
  "common.colActions": {
    message: "Acciones",
    description:
      "Screen-reader-only header for a table's trailing row-actions column",
  },
  "common.justNow": {
    message: "justo ahora",
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
    message: "Marca de tiempo",
    description:
      "Timestamp tooltip: label of the row showing the instant as Unix milliseconds",
  },
  "common.timeFromPrevious": {
    message: "Desde la línea anterior",
    description:
      "Timestamp tooltip (w4/716): label of a log line's row showing the time since the line before it, Render's 'From previous'",
  },
  "common.timeFromStart": {
    message: "Desde el inicio",
    description:
      "Timestamp tooltip (w4/716): label of a deploy log line's row showing the time since the deploy began, Render's 'From start'",
  },
  "common.timeRowLabel": {
    message: "{label}:",
    description:
      "Timestamp tooltip (w4/716): a row's label followed by Render's colon; {label} is the zone ('PST'), UTC, Timestamp, From previous or From start",
  },
  "common.timeCopied": {
    message: "¡Copiado!",
    description:
      "Timestamp tooltip (w4/716): toast after a row's copy button copied its value, Render's 'Copied!'",
  },
  "common.timeCopyFailed": {
    message: "No se pudo copiar la hora",
    description:
      "Timestamp tooltip (w4/716): toast when the clipboard refused a row's value",
  },
  "common.ipAllowListProxiedTitle_one": {
    message:
      "La lista de permitidos no puede ver a los clientes de {count} dominio personalizado",
    description:
      "Title of the warning shown under an inbound IP allowlist when one of the custom domains it covers is proxied by Cloudflare (w1/m171)",
  },
  "common.ipAllowListProxiedTitle_many": {
    message:
      "La lista de permitidos no puede ver a los clientes de {count} de dominios personalizados",
    description:
      "Title of the warning shown under an inbound IP allowlist when several custom domains it covers are proxied by Cloudflare (w1/m171)",
  },
  "common.ipAllowListProxiedTitle_other": {
    message:
      "La lista de permitidos no puede ver a los clientes de {count} dominios personalizados",
    description:
      "Title of the warning shown under an inbound IP allowlist when several custom domains it covers are proxied by Cloudflare (w1/m171)",
  },
  "common.ipAllowListProxiedBody_one": {
    message:
      "Este dominio pasa por el proxy de Cloudflare, por lo que la lista de permitidos coincide con la dirección de Cloudflare allí, no con la del visitante. La lista se guarda, pero en este dominio bloqueará a visitantes que querías permitir:",
    description:
      "Body of the Cloudflare-proxied allowlist warning, followed by the one proxied hostname (w1/m171)",
  },
  "common.ipAllowListProxiedBody_many": {
    message:
      "Estos dominios pasan por el proxy de Cloudflare, por lo que la lista de permitidos coincide con la dirección de Cloudflare allí, no con la del visitante. La lista se guarda, pero en estos dominios bloqueará a visitantes que querías permitir:",
    description:
      "Body of the Cloudflare-proxied allowlist warning, followed by the list of proxied hostnames (w1/m171)",
  },
  "common.ipAllowListProxiedBody_other": {
    message:
      "Estos dominios pasan por el proxy de Cloudflare, por lo que la lista de permitidos coincide con la dirección de Cloudflare allí, no con la del visitante. La lista se guarda, pero en estos dominios bloqueará a visitantes que querías permitir:",
    description:
      "Body of the Cloudflare-proxied allowlist warning, followed by the list of proxied hostnames (w1/m171)",
  },
  "common.ipAllowListProxiedFix": {
    message:
      "Para filtrar las direcciones reales de los clientes, establece el registro DNS en solo DNS (nube gris) en Cloudflare.",
    description:
      "How to fix the Cloudflare-proxied allowlist warning: switch the record from proxied (orange cloud) to DNS only (w1/m171)",
  },
  "common.relativeAgeNow": {
    message: "ahora",
    description: "Compact relative age under one minute (w4/m188)",
  },
  "common.relativeAgeMinutes": {
    message: "{n} min",
    description: "Compact relative age in whole minutes, e.g. 4m (w4/m188)",
  },
  "common.relativeAgeHours": {
    message: "{n} h",
    description: "Compact relative age in whole hours, e.g. 3h (w4/m188)",
  },
  "common.relativeAgeDays": {
    message: "{n} d",
    description: "Compact relative age in whole days, e.g. 5d (w4/m188)",
  },
  "common.relativeAgeMonths": {
    message: "{n} mes",
    description: "Compact relative age in 30-day months, e.g. 2mo (w4/m188)",
  },
  "common.relativeAgeYears": {
    message: "{n} a",
    description: "Compact relative age in 365-day years, e.g. 1y (w4/m188)",
  },
  "common.relativeUntilNow": {
    message: "ahora",
    description:
      "Countdown to a future instant less than one minute away (w4/m188)",
  },
  "common.relativeUntilMinutes": {
    message: "en {n} min",
    description: "Compact countdown in whole minutes, e.g. in 5m (w4/m188)",
  },
  "common.relativeUntilHours": {
    message: "en {n} h",
    description: "Compact countdown in whole hours, e.g. in 3h (w4/m188)",
  },
  "common.relativeUntilDays": {
    message: "en {n} d",
    description: "Compact countdown in whole days, e.g. in 83d (w4/m188)",
  },
  "common.openBilling": {
    message: "Abrir facturación",
    description:
      "Link from a plan notice (a Pro-and-higher feature on Hobby) to the Billing page (w8/m138)",
  },
  "common.billingSettlingTitle": {
    message: "Finalizando el uso final…",
    description:
      "Title while a deletion waits for the workspace's final usage to be charged (BILLING_SETTLING); retried automatically",
  },
  "common.billingSettlingDescription": {
    message:
      "bex está cobrando el uso final antes de eliminar. Esto puede tardar uno o dos minutos; se reintenta automáticamente, así que mantén esta página abierta.",
    description:
      "Body while a deletion waits for the final usage to be charged (BILLING_SETTLING)",
  },
  "common.billingSettlingTimeout": {
    message:
      "El uso final todavía se está procesando. Intenta eliminar de nuevo en unos minutos.",
    description:
      "Deletion error after BILLING_SETTLING retries ran out (about three minutes)",
  },
  "common.balanceDueTitle": {
    message: "Saldo pendiente",
    description:
      "Title of the alert when a deletion is refused because the workspace has an unpaid balance (BILLING_BALANCE_DUE)",
  },
  "common.balanceDueAmount": {
    message:
      "Este espacio de trabajo tiene un saldo pendiente de {amount}. Paga la factura y vuelve a eliminarlo.",
    description: "Balance-due deletion refusal with the amount",
  },
  "common.balanceDue": {
    message:
      "Este espacio de trabajo tiene un saldo pendiente. Paga la factura y vuelve a eliminarlo.",
    description: "Balance-due deletion refusal without an amount",
  },
  "common.balanceDueNamedAmount": {
    message:
      "El espacio de trabajo {name} tiene un saldo pendiente de {amount}. Paga la factura y vuelve a eliminar tu cuenta.",
    description:
      "Account-deletion refusal naming the workspace with the unpaid balance and its amount",
  },
  "common.balanceDueNamed": {
    message:
      "El espacio de trabajo {name} tiene un saldo pendiente. Paga la factura y vuelve a eliminar tu cuenta.",
    description:
      "Account-deletion refusal naming the workspace with the unpaid balance",
  },
  "common.balanceDuePay": {
    message: "Pagar factura",
    description: "Link to Stripe's hosted invoice page, opens in a new tab",
  },
  "common.regionFrankfurt": {
    message: "Fráncfort, Alemania",
    description:
      "Display name of Render's `frankfurt` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionOhio": {
    message: "Ohio, EE. UU.",
    description:
      "Display name of Render's `ohio` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionOregon": {
    message: "Oregón, EE. UU.",
    description:
      "Display name of Render's `oregon` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionSingapore": {
    message: "Singapur",
    description:
      "Display name of Render's `singapore` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionVirginia": {
    message: "Virginia, EE. UU.",
    description:
      "Display name of Render's `virginia` region, shown where a resource's region is read (w4/693)",
  },
  "common.createOptional": {
    message: "Opcional",
    description:
      "Mark beside a create-page row label whose field may be left blank (Render's 'Optional')",
  },
  "common.createAdvanced": {
    message: "Avanzado",
    description: "Heading of the collapsed Advanced card on a create page",
  },
  "common.planListShowAll": {
    message: "Mostrar todos los planes",
    description: "Button under a create page's plan list that shows every plan",
  },
  "common.planListShowFewer": {
    message: "Mostrar menos planes",
    description:
      "Button under an expanded plan list that folds it back to the first plans",
  },
  "common.createBlockedName": {
    message: "Introduce un nombre para continuar.",
    description:
      "Create page action bar: why Submit is held while the name is empty or invalid",
  },
  "common.createBlockedFields": {
    message: "Completa o corrige los campos de arriba para continuar.",
    description:
      "Create page action bar: why Submit is held while a field is missing or invalid",
  },
  "common.createTypePostgres": {
    message: "Postgres",
    description:
      "Postgres in the New menu and a create page's type switcher (Render's product name)",
  },
  "common.createTypeSwitcher": {
    message: "{type}: elegir un tipo de servicio",
    description:
      "Accessible name of the breadcrumb menu that switches a create page to another resource type ({type} = the current page's heading)",
  },
  "common.createTypeProject": {
    message: "Proyecto",
    description: "Project in the New menu (Render's bare 'Project')",
  },
  "common.createTypeBlueprint": {
    message: "Blueprint",
    description: "Blueprint in the New menu (Render's bare 'Blueprint')",
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
    message: "Elige un nombre que no esté en uso para continuar.",
    description:
      "Create page action bar: why Submit is held while the name is already in use",
  },
  "common.createRequired": {
    message: "Obligatorio",
    description:
      "Mark beside a create-page row label whose field must be filled before Submit",
  },
  "common.createBlockedLoadingPlans": {
    message: "Cargando planes…",
    description:
      "Create page action bar: Submit waits only for the plan catalog to load (not the user's fault)",
  },
};

export default esCommon;
