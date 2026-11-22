import type { TranslationEntry } from "@/i18n";

const deCommon: Record<string, TranslationEntry> = {
  "common.navRouter": {
    message: "Router",
    description: "Router navigation item",
  },
  "common.protectedConfirmationTitle": {
    message: "Bestätigung für geschützte Umgebung erforderlich",
    description:
      "Title for the retry dialog shown after a protected environment blocks a destructive action",
  },
  "common.protectedConfirmationBody": {
    message:
      "{name} gehört zu einer geschützten Umgebung. Gib unten den genauen Befehl ein, um fortzufahren.",
    description:
      "Resource-neutral explanation for a protected environment confirmation gate",
  },
  "common.protectedConfirmationPrompt": {
    message: "Gib unten {phrase} zur Bestätigung ein.",
    description:
      "Prompt naming the server-issued protected environment confirmation phrase",
  },
  "common.confirmPhrasePrompt": {
    message: "Gib zum Fortfahren {phrase} ein",
    description:
      "Label above the typed-confirmation input in a phrase-gated confirm dialog.",
  },
  "common.close": {
    message: "Schließen",
    description:
      "Accessible name of the shared dialog's built-in close (X) button.",
  },
  "common.sidebarTitle": {
    message: "Seitenleiste",
    description: "Screen-reader title of the mobile navigation drawer.",
  },
  "common.sidebarMobileDescription": {
    message: "Zeigt die mobile Seitenleiste an.",
    description: "Screen-reader description of the mobile navigation drawer.",
  },
  "common.skipToMainContent": {
    message: "Zum Hauptinhalt springen",
    description:
      "Label of the first-tab skip link that moves keyboard focus past the sidebar and header to the page content.",
  },
  "common.sidebarToggle": {
    message: "Seitenleiste umschalten",
    description:
      "Accessible name of the button that opens or collapses the navigation sidebar.",
  },
  "common.sidebarResize": {
    message: "Größe der Seitenleiste ändern",
    description: "Accessible name of the sidebar's draggable resize separator.",
  },
  "common.sidebarResizeHint": {
    message: "Ziehen zum Ändern der Größe · Doppelklick zum Umschalten",
    description: "Hover tooltip on the sidebar's resize separator.",
  },
  "common.commandSuggestions": {
    message: "Vorschläge",
    description:
      "Accessible name of a search or picker results list (the command menu's listbox).",
  },
  "common.comboboxPlaceholder": {
    message: "Option auswählen",
    description:
      "Fallback placeholder of a searchable picker when its caller supplies none.",
  },
  "common.comboboxEmpty": {
    message: "Keine Ergebnisse gefunden",
    description:
      "Fallback text when a searchable picker's filter matches no option.",
  },
  "common.comboboxUseCustom": {
    message: "Drücke die Eingabetaste, um „{value}“ zu verwenden",
    description:
      "Hint under a picker's empty results: Enter keeps the typed value as entered. {value} is the typed text.",
  },
  "common.comboboxClear": {
    message: "Eingabe löschen",
    description: "Accessible name of a searchable picker's clear (X) button.",
  },
  "common.cancel": {
    message: "Abbrechen",
    description:
      "Generic Cancel button — dismisses a dialog or an inline form without saving.",
  },
  "common.pricePerRunMinute": {
    message: "${price}/Min. Laufzeit",
    description:
      "A cron job's USD price per minute of run time, on a Change Compute Plan card (w4/546): it is billed per second only while a run executes; price is an API-supplied decimal without the currency symbol",
  },
  "common.pricePerMinute": {
    message: "${price}/Min.",
    description:
      "A cron job's USD price per minute of run time on a create-form plan card, short to fit beside the plan name; the form's hint says it is billed only while a run executes (w4/546). price is an API-supplied decimal without the currency symbol",
  },
  "common.pricePerMonth": {
    message: "${price}/Monat",
    description:
      "Monthly USD price shown on a resource plan card; price is an API-supplied decimal without the currency symbol",
  },
  "common.planUnavailable": {
    message: "Nicht verfügbar",
    description:
      "Shown in place of the price on a disabled plan card whose compute plan is paused (it cannot be selected for now); the server's full reason is the card's tooltip",
  },
  "common.planConnections_one": {
    message: "{formatted} Verbindung",
    description:
      "A plan card's connection-limit line, under its RAM: the client connections the plan accepts (Render's Key Value cards read \"50 Connections\"); {formatted} is the locale-formatted count",
  },
  "common.planConnections_other": {
    message: "{formatted} Verbindungen",
    description:
      "A plan card's connection-limit line, under its RAM: the client connections the plan accepts (Render's Key Value cards read \"50 Connections\"); {formatted} is the locale-formatted count",
  },
  "common.protectedConfirmationRetry": {
    message: "Bestätigen und speichern",
    description:
      "Confirm button in the protected environment retry dialog raised by a settings save",
  },
  "common.protectedConfirmationCancel": {
    message: "Abbrechen",
    description:
      "Cancel button in the protected environment confirmation dialog",
  },
  "common.navEnvGroups": {
    message: "Umgebungsgruppen",
    description: "Workspace sidebar link to environment groups",
  },
  "common.navSandboxGroups": {
    message: "Sandbox-Gruppen",
    description: "Workspace sidebar link to sandbox groups",
  },
  "common.appName": {
    message: "bex",
    description: "Product name shown in the dashboard chrome",
  },
  "common.headDescription": {
    message:
      "Deploye und betreibe Anwendungen auf Infrastruktur, die du kontrollierst, mit bex, der quelloffenen, KI-nativen Render-Alternative.",
    description:
      "Generic dashboard description used in description, Open Graph, and Twitter metadata",
  },
  "common.headImageAlt": {
    message: "bex-Dashboard-Logo",
    description: "Alternative text for the dashboard's generic social image",
  },
  "common.loading": {
    message: "Wird geladen…",
    description: "Generic loading state label",
  },
  "common.topbarBreadcrumbs": {
    message: "Brotkrümelnavigation",
    description: "Accessible label for the dashboard topbar hierarchy",
  },
  "common.topbarSearch": {
    message: "Suchen",
    description: "Open the workspace-wide topbar search",
  },
  "common.copyCode": {
    message: "Code kopieren",
    description: "Accessible name of the copy button on a code block",
  },
  "common.topbarSearchPlaceholder": {
    message: "Seiten und Ressourcen durchsuchen…",
    description: "Placeholder in the workspace-wide topbar command search",
  },
  "common.topbarSearchDescription": {
    message: "Durchsuche Dashboard-Seiten und Workspace-Ressourcen.",
    description: "Accessible description for the topbar command search",
  },
  "common.topbarSearchEmpty": {
    message: "Keine passenden Seiten oder Ressourcen.",
    description: "Empty state in the workspace-wide topbar command search",
  },
  "common.copyField": {
    message: "{field} kopieren",
    description:
      "Accessible name for a copy control when the caller only has a field label (m101/t014)",
  },
  "common.topbarNavigation": {
    message: "Navigation",
    description: "Page-links group in the workspace-wide topbar search",
  },
  "common.topbarResources": {
    message: "Ressourcen",
    description: "Workspace-resources group in the topbar search",
  },
  "common.topbarNew": {
    message: "Neu",
    description: "Persistent topbar menu for creating a resource",
  },
  "common.topbarCreate": {
    message: "Ressource erstellen",
    description: "Heading in the persistent topbar create menu",
  },
  "common.topbarHelp": {
    message: "Hilfe und Ressourcen",
    description: "Accessible label for the topbar help menu",
  },
  "common.topbarDocumentation": {
    message: "Dokumentation",
    description: "Topbar help-menu link to bex documentation",
  },
  "common.topbarCliGuide": {
    message: "CLI-Leitfaden",
    description: "Topbar help-menu link to the bex CLI guide",
  },
  "common.topbarRepository": {
    message: "GitHub-Repository",
    description: "Topbar help-menu link to the bex source repository",
  },
  "common.topbarWorkspaceSettings": {
    message: "Workspace-Einstellungen",
    description: "Workspace-settings label in topbar navigation and search",
  },
  "common.topbarSwitchProject": {
    message: "Projekt wechseln",
    description: "Heading in a project breadcrumb dropdown",
  },
  "common.topbarSwitchEnvironment": {
    message: "Umgebung wechseln",
    description: "Heading in an environment breadcrumb dropdown",
  },
  "common.topbarSwitchService": {
    message: "Service wechseln",
    description: "Heading in a service breadcrumb dropdown",
  },
  "common.topbarAllResources": {
    message: "Alle Ressourcen",
    description: "Service breadcrumb menu link back to workspace resources",
  },
  "common.topbarProjectResource": {
    message: "Projekt",
    description: "Project kind label in workspace-wide search results",
  },
  "common.topbarServiceResource": {
    message: "Service",
    description: "Service kind label in workspace-wide search results",
  },
  "common.sudoCommandLabel": {
    message: "Sudo-Befehl",
    description:
      "Input label on Render-style destructive type-to-confirm gates (live capture: docs/render-artifacts/workspace-lifecycle.md)",
  },
  "common.navIntegrationsGroup": {
    message: "Integrationen",
    description:
      "Sidebar nav section label (webhooks, notifications) — Render's grouping",
  },
  "common.navWorkspaceGroup": {
    message: "Workspace",
    description:
      "Sidebar nav section label (usage, settings) — Render's grouping",
  },
  "common.navMonitorGroup": {
    message: "Überwachung",
    description:
      "Service sidebar section label (logs, metrics) — Render's grouping",
  },
  "common.navWebhooks": {
    message: "Webhooks",
    description: "Sidebar nav link to the outbound event webhooks page",
  },
  "common.labelInvalid": {
    message:
      "Gib einen Namen mit bis zu 100 Zeichen ein, ohne Zeilenumbrüche oder Steuerzeichen.",
    description:
      "Inline validation under a free-form name field (project, environment, group, webhook, credential, Blueprint)",
  },
  "common.navNotifications": {
    message: "Benachrichtigungen",
    description: "Sidebar nav link to the notification settings page",
  },
  "common.navProjects": {
    message: "Projekte",
    description:
      "Sidebar nav link to the unified projects page (services, databases, key value grouped together)",
  },
  "common.navUsage": {
    message: "Abrechnung",
    description:
      "Sidebar nav link to the workspace billing page (/billing, renamed from Usage in w5/m70; key kept to avoid churn)",
  },
  "common.navBlueprints": {
    message: "Blueprints",
    description:
      "Sidebar nav link to the blueprints management page (IaC stacks auto-registered on deploy)",
  },
  "common.navAgents": {
    message: "Agents",
    description:
      "Sidebar nav link to the cloud coding-agent sessions page (/agents)",
  },
  "common.navAutomations": {
    message: "Automatisierungen",
    description:
      "Sidebar nav link to the scheduled agent automations page (/automations)",
  },
  "common.navSettings": {
    message: "Einstellungen",
    description:
      "Sidebar nav link to the workspace settings page (the sidebar is workspace-scoped; account settings hang off the user menu)",
  },
  "common.navBackToDashboard": {
    message: "Dashboard",
    description:
      "Contextual project sidebar's back link label, shown on a project's Overview page (links up to the workspace Overview)",
  },
  "common.navBackToProject": {
    message: "Projekt",
    description:
      "Contextual project sidebar's back link label, shown on a project's Settings page (links up to the project's Overview)",
  },
  "common.navProjectOverview": {
    message: "Übersicht",
    description:
      "Contextual project sidebar nav link to the project's own Overview page",
  },
  "common.navObservabilityGroup": {
    message: "Observability",
    description:
      "Contextual project sidebar section label above the project's log views (Render's OBSERVABILITY group, w3/m505)",
  },
  "common.navManageGroup": {
    message: "Verwalten",
    description: "Contextual project sidebar section label above Settings",
  },
  "common.changeLanguage": {
    message: "Sprache ändern",
    description: "Accessible label for the language switcher button",
  },
  "common.userMenuSettings": {
    message: "Kontoeinstellungen",
    description:
      "User menu item that navigates to account settings (/settings)",
  },
  "common.userMenuTheme": {
    message: "Design",
    description: "User menu submenu label for theme selection",
  },
  "common.userMenuLanguage": {
    message: "Sprache",
    description: "User menu submenu label for language selection",
  },
  "common.userMenuThemeLight": {
    message: "Hell",
    description: "Light theme option label",
  },
  "common.userMenuThemeDark": {
    message: "Dunkel",
    description: "Dark theme option label",
  },
  "common.userMenuThemeSystem": {
    message: "System",
    description: "System theme option label",
  },
  "common.themeNotSaved": {
    message:
      "Dein Design wurde nicht gespeichert. Das vorherige Design ist wieder aktiv.",
    description:
      "Toast when a theme change could not be saved to the account (w3/m531); the previous theme is put back",
  },
  "common.userMenuLogOut": {
    message: "Abmelden",
    description: "User menu item that signs the user out",
  },
  "common.userMenuOpen": {
    message: "Kontomenü",
    description:
      "Accessible name of the avatar button that opens the account menu (w4/407)",
  },
  "common.userMenuOpenFor": {
    message: "Kontomenü für {name}",
    description:
      "Accessible name of the avatar button that opens the account menu, naming the signed-in identity (w4/407)",
  },
  "common.goHome": {
    message: "Zur Startseite",
    description: "Button label that navigates back to the home page",
  },
  "common.goBack": {
    message: "Zurück",
    description: "Button label that navigates to the previous page",
  },
  "common.planCatalogUnavailable": {
    message:
      "Die Tarife konnten nicht geladen werden, daher kann noch keiner ausgewählt werden. Deine übrigen Eingaben bleiben erhalten.",
    description:
      "A plan picker whose catalog read failed or returned no plans; shown with a Try again button that re-reads only the catalog (w4/m218)",
  },
  "common.tryAgain": {
    message: "Erneut versuchen",
    description: "Button label that retries after an error",
  },
  "common.notFoundTitle": {
    message: "Seite nicht gefunden",
    description: "404 page heading",
  },
  "common.notFoundDescription": {
    message: "Die gesuchte Seite existiert nicht oder wurde verschoben.",
    description: "404 page explanatory text",
  },
  "common.errorTitle": {
    message: "Etwas ist schiefgelaufen",
    description: "Generic error page heading",
  },
  "common.errorDefaultMessage": {
    message: "Etwas ist schiefgelaufen.",
    description: "Fallback error message when none is provided",
  },
  "common.loadErrorRetry": {
    message:
      "Beim Laden ist etwas schiefgelaufen. Lade die Seite neu, um es erneut zu versuchen.",
    description:
      "Body of a panel whose read failed with no server reason to show (a network failure or throttling)",
  },
  "common.resourceNotFoundToast": {
    message: "Diese Ressource existiert nicht oder wurde gelöscht.",
    description:
      "Toast shown after a dead resource URL redirects to the home page (w9/m55)",
  },
  "common.resourceErrorBody": {
    message:
      "Die Ressource konnte nicht geladen werden. Prüfe die API und versuche es erneut.",
    description:
      "Body text of a detail page's inline error state when the resource query failed",
  },
  "common.sessionExpiredTitle": {
    message: "Deine Sitzung ist abgelaufen",
    description:
      "Heading of the inline state shown when a resource query fails with a 401 (expired session)",
  },
  "common.sessionExpiredBody": {
    message:
      "Melde dich erneut an, um weiterzumachen — du kommst direkt zu dieser Seite zurück.",
    description:
      "Body text of the expired-session inline state, reassuring the user their place is preserved",
  },
  "common.signIn": {
    message: "Anmelden",
    description:
      "Button label that sends an expired session back to the login page",
  },
  "common.colActions": {
    message: "Aktionen",
    description:
      "Screen-reader-only header for a table's trailing row-actions column",
  },
  "common.justNow": {
    message: "gerade eben",
    description:
      'Elapsed-time text for an instant under a minute old ("Deployed just now")',
  },
  "common.timeLocal": {
    message: "Lokal",
    description:
      "Timestamp tooltip: label of the row showing the instant in the viewer's local timezone",
  },
  "common.timeUtc": {
    message: "UTC",
    description:
      "Timestamp tooltip: label of the row showing the instant in UTC",
  },
  "common.timeUnix": {
    message: "Zeitstempel",
    description:
      "Timestamp tooltip: label of the row showing the instant as Unix seconds",
  },
  "common.ipAllowListProxiedTitle_one": {
    message:
      "Die Allowlist kann Clients auf {count} benutzerdefinierten Domain nicht sehen",
    description:
      "Title of the warning shown under an inbound IP allowlist when one of the custom domains it covers is proxied by Cloudflare (w1/m171)",
  },
  "common.ipAllowListProxiedTitle_other": {
    message:
      "Die Allowlist kann Clients auf {count} benutzerdefinierten Domains nicht sehen",
    description:
      "Title of the warning shown under an inbound IP allowlist when several custom domains it covers are proxied by Cloudflare (w1/m171)",
  },
  "common.ipAllowListProxiedBody_one": {
    message:
      "Diese Domain läuft über den Cloudflare-Proxy, daher gleicht die Allowlist dort die Adresse von Cloudflare ab, nicht die des Besuchers. Die Liste ist gespeichert, blockiert auf dieser Domain aber Besucher, die du zulassen wolltest:",
    description:
      "Body of the Cloudflare-proxied allowlist warning, followed by the one proxied hostname (w1/m171)",
  },
  "common.ipAllowListProxiedBody_other": {
    message:
      "Diese Domains laufen über den Cloudflare-Proxy, daher gleicht die Allowlist dort die Adresse von Cloudflare ab, nicht die des Besuchers. Die Liste ist gespeichert, blockiert auf diesen Domains aber Besucher, die du zulassen wolltest:",
    description:
      "Body of the Cloudflare-proxied allowlist warning, followed by the list of proxied hostnames (w1/m171)",
  },
  "common.ipAllowListProxiedFix": {
    message:
      "Um echte Client-Adressen zu filtern, stelle den DNS-Eintrag in Cloudflare auf „Nur DNS“ (graue Wolke).",
    description:
      "How to fix the Cloudflare-proxied allowlist warning: switch the record from proxied (orange cloud) to DNS only (w1/m171)",
  },
  "common.relativeAgeNow": {
    message: "jetzt",
    description: "Compact relative age under one minute (w4/m188)",
  },
  "common.relativeAgeMinutes": {
    message: "{n} Min.",
    description: "Compact relative age in whole minutes, e.g. 4m (w4/m188)",
  },
  "common.relativeAgeHours": {
    message: "{n} Std.",
    description: "Compact relative age in whole hours, e.g. 3h (w4/m188)",
  },
  "common.relativeAgeDays": {
    message: "{n} T.",
    description: "Compact relative age in whole days, e.g. 5d (w4/m188)",
  },
  "common.relativeAgeMonths": {
    message: "{n} Mon.",
    description: "Compact relative age in 30-day months, e.g. 2mo (w4/m188)",
  },
  "common.relativeAgeYears": {
    message: "{n} J.",
    description: "Compact relative age in 365-day years, e.g. 1y (w4/m188)",
  },
  "common.relativeUntilNow": {
    message: "jetzt",
    description:
      "Countdown to a future instant less than one minute away (w4/m188)",
  },
  "common.relativeUntilMinutes": {
    message: "in {n} Min.",
    description: "Compact countdown in whole minutes, e.g. in 5m (w4/m188)",
  },
  "common.relativeUntilHours": {
    message: "in {n} Std.",
    description: "Compact countdown in whole hours, e.g. in 3h (w4/m188)",
  },
  "common.relativeUntilDays": {
    message: "in {n} T.",
    description: "Compact countdown in whole days, e.g. in 83d (w4/m188)",
  },
  "common.openBilling": {
    message: "Abrechnung öffnen",
    description:
      "Link from a plan notice (a Pro-and-higher feature on Hobby) to the Billing page (w8/m138)",
  },
  "common.billingSettlingTitle": {
    message: "Abschließende Nutzung wird abgerechnet…",
    description:
      "Title while a deletion waits for the workspace's final usage to be charged (BILLING_SETTLING); retried automatically",
  },
  "common.billingSettlingDescription": {
    message:
      "bex rechnet die abschließende Nutzung ab, bevor gelöscht wird. Das kann ein oder zwei Minuten dauern; es wird automatisch erneut versucht, lass diese Seite also geöffnet.",
    description:
      "Body while a deletion waits for the final usage to be charged (BILLING_SETTLING)",
  },
  "common.billingSettlingTimeout": {
    message:
      "Die abschließende Nutzung wird noch verarbeitet. Versuche das Löschen in einigen Minuten erneut.",
    description:
      "Deletion error after BILLING_SETTLING retries ran out (about three minutes)",
  },
  "common.balanceDueTitle": {
    message: "Offener Betrag",
    description:
      "Title of the alert when a deletion is refused because the workspace has an unpaid balance (BILLING_BALANCE_DUE)",
  },
  "common.balanceDueAmount": {
    message:
      "Dieser Workspace hat einen offenen Betrag von {amount}. Bezahle die Rechnung und lösche ihn dann erneut.",
    description: "Balance-due deletion refusal with the amount",
  },
  "common.balanceDue": {
    message:
      "Dieser Workspace hat einen offenen Betrag. Bezahle die Rechnung und lösche ihn dann erneut.",
    description: "Balance-due deletion refusal without an amount",
  },
  "common.balanceDueNamedAmount": {
    message:
      "Der Workspace {name} hat einen offenen Betrag von {amount}. Bezahle die Rechnung und lösche dann dein Konto erneut.",
    description:
      "Account-deletion refusal naming the workspace with the unpaid balance and its amount",
  },
  "common.balanceDueNamed": {
    message:
      "Der Workspace {name} hat einen offenen Betrag. Bezahle die Rechnung und lösche dann dein Konto erneut.",
    description:
      "Account-deletion refusal naming the workspace with the unpaid balance",
  },
  "common.balanceDuePay": {
    message: "Rechnung bezahlen",
    description: "Link to Stripe's hosted invoice page, opens in a new tab",
  },
  "common.regionFrankfurt": {
    message: "Frankfurt, Deutschland",
    description:
      "Display name of Render's `frankfurt` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionOhio": {
    message: "Ohio, USA",
    description:
      "Display name of Render's `ohio` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionOregon": {
    message: "Oregon, USA",
    description:
      "Display name of Render's `oregon` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionSingapore": {
    message: "Singapur",
    description:
      "Display name of Render's `singapore` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionVirginia": {
    message: "Virginia, USA",
    description:
      "Display name of Render's `virginia` region, shown where a resource's region is read (w4/693)",
  },
  "common.createOptional": {
    message: "Optional",
    description:
      "Mark beside a create-page row label whose field may be left blank (Render's 'Optional')",
  },
  "common.createAdvanced": {
    message: "Erweitert",
    description: "Heading of the collapsed Advanced card on a create page",
  },
  "common.planListShowAll": {
    message: "Alle Pläne anzeigen",
    description: "Button under a create page's plan list that shows every plan",
  },
  "common.planListShowFewer": {
    message: "Weniger Pläne anzeigen",
    description:
      "Button under an expanded plan list that folds it back to the first plans",
  },
  "common.createBlockedName": {
    message: "Geben Sie einen Namen ein, um fortzufahren.",
    description:
      "Create page action bar: why Submit is held while the name is empty or invalid",
  },
  "common.createBlockedFields": {
    message:
      "Füllen Sie die Felder oben aus oder korrigieren Sie sie, um fortzufahren.",
    description:
      "Create page action bar: why Submit is held while a field is missing or invalid",
  },
  "common.createTypePostgres": {
    message: "Postgres",
    description:
      "Postgres in the New menu and a create page's type switcher (Render's product name)",
  },
  "common.createTypeSwitcher": {
    message: "{type}: Servicetyp wählen",
    description:
      "Accessible name of the breadcrumb menu that switches a create page to another resource type ({type} = the current page's heading)",
  },
  "common.createTypeProject": {
    message: "Projekt",
    description: "Project in the New menu (Render's bare 'Project')",
  },
  "common.createTypeBlueprint": {
    message: "Blueprint",
    description: "Blueprint in the New menu (Render's bare 'Blueprint')",
  },
  "common.createTypeSandboxGroup": {
    message: "Sandbox-Gruppe",
    description:
      "Sandbox Group in the New menu (Render's bare 'Sandbox Group')",
  },
  "common.createTypeWebhook": {
    message: "Webhook",
    description:
      "Webhook in the New menu (bex's own create, after Render's entries)",
  },
  "common.createBlockedNameTaken": {
    message: "Wählen Sie einen noch nicht verwendeten Namen, um fortzufahren.",
    description:
      "Create page action bar: why Submit is held while the name is already in use",
  },
  "common.createRequired": {
    message: "Erforderlich",
    description:
      "Mark beside a create-page row label whose field must be filled before Submit",
  },
  "common.createBlockedLoadingPlans": {
    message: "Pläne werden geladen …",
    description:
      "Create page action bar: Submit waits only for the plan catalog to load (not the user's fault)",
  },
};

export default deCommon;
