import type { TranslationEntry } from "@/i18n";

const deBuildSources: Record<string, TranslationEntry> = {
  "buildSources.pageTitle": {
    message: "Build-Quellen",
    description: "Title of the Build Sources list page",
  },
  "buildSources.pageDescription": {
    message:
      "Erstelle einmal aus einem Git-Repository oder Docker-Image und stelle das Artefakt für mehrere Dienste bereit.",
    description: "Subtitle of the Build Sources list page",
  },
  "buildSources.resourceType": {
    message: "Build-Quelle",
    description: "Singular resource type label shown in global search results",
  },
  "buildSources.newButton": {
    message: "Neue Build-Quelle",
    description: "Primary CTA button to create a new Build Source",
  },
  "buildSources.backToList": {
    message: "Zurück zu Build-Quellen",
    description: "Back button label returning to /build-sources",
  },
  "buildSources.viewAll": {
    message: "Alle anzeigen",
    description: "Link from Project Overview Build Sources section to /build-sources",
  },
  "buildSources.viewBuildSources": {
    message: "Build-Quellen anzeigen",
    description: "Button in partial-link alert navigating to /build-sources",
  },
  "buildSources.emptyTitle": {
    message: "Noch keine Build-Quellen",
    description: "Empty state heading when the workspace has no build sources",
  },
  "buildSources.emptyDescription": {
    message:
      "Erstelle eine Build-Quelle, um einen einzelnen Git-Build oder ein Docker-Image für Web Services, Private Services, Background Workers und Cron Jobs zu teilen.",
    description: "Empty state explanation on /build-sources",
  },
  "buildSources.refreshError": {
    message:
      "Build-Quellen konnten nicht aktualisiert werden. Zwischengespeicherte Ergebnisse werden angezeigt.",
    description: "Inline error banner when a background refresh fails over cached rows",
  },
  "buildSources.searchPlaceholder": {
    message: "Nach Name, Repository, Image oder Projekt suchen…",
    description: "Placeholder for the /build-sources search input",
  },
  "buildSources.searchLabel": {
    message: "Build-Quellen durchsuchen",
    description: "Accessible label for the /build-sources search input",
  },
  "buildSources.resetFilters": {
    message: "Filter zurücksetzen",
    description: "Button to clear active search and kind filters on /build-sources",
  },
  "buildSources.filterKindLabel": {
    message: "Nach Quelltyp filtern",
    description: "Accessible label for the source-kind filter button group",
  },
  "buildSources.filterAll": {
    message: "Alle",
    description: "Filter button showing both Git and Docker image build sources",
  },
  "buildSources.kindGit": {
    message: "Git",
    description: "Source kind badge and filter label for Git-backed build sources",
  },
  "buildSources.kindImage": {
    message: "Docker-Image",
    description: "Source kind badge and filter label for Docker-image-backed build sources",
  },
  "buildSources.noMatchesTitle": {
    message: "Keine passenden Build-Quellen",
    description: "Heading when search or kind filter matches zero build sources",
  },
  "buildSources.noMatchesDescription": {
    message: "Passe deine Suche an oder setze den Quelltyp-Filter zurück.",
    description: "Body copy when search or kind filter matches zero build sources",
  },
  "buildSources.colName": {
    message: "Name",
    description: "Table column header for Build Source name",
  },
  "buildSources.colSource": {
    message: "Quelle",
    description: "Table column header for Git repo/branch or Docker image",
  },
  "buildSources.colRegion": {
    message: "Region",
    description: "Table column header for Build Source region",
  },
  "buildSources.colLinkedServices": {
    message: "Verknüpfte Dienste",
    description: "Table column header for linked service count",
  },
  "buildSources.colProject": {
    message: "Projekt",
    description: "Table column header for optional project scope",
  },
  "buildSources.colUpdated": {
    message: "Aktualisiert",
    description: "Table column header for last updated timestamp",
  },
  "buildSources.copyId": {
    message: "Build-Quellen-ID kopieren",
    description: "Accessible label for copying a bds- ID",
  },
  "buildSources.copied": {
    message: "Kopiert",
    description: "Copy button confirmation tooltip",
  },
  "buildSources.copyError": {
    message: "Kopieren fehlgeschlagen",
    description: "Copy button failure tooltip",
  },
  "buildSources.linkedServicesCount_one": {
    message: "{count} Dienst",
    description: "Singular count of services linked to a build source",
  },
  "buildSources.linkedServicesCount_other": {
    message: "{count} Dienste",
    description: "Plural count of services linked to a build source",
  },
  "buildSources.workspaceScope": {
    message: "Workspace",
    description: "Label when a build source is workspace-wide (not scoped to a project)",
  },
  "buildSources.createTitle": {
    message: "Neue Build-Quelle",
    description: "Page heading of /build-sources/new",
  },
  "buildSources.createDescription": {
    message:
      "Konfiguriere einen gemeinsamen Git-Build oder ein Docker-Image, aus dem mehrere Dienste bereitgestellt werden können.",
    description: "Subtitle of /build-sources/new",
  },
  "buildSources.createSubmit": {
    message: "Build-Quelle erstellen",
    description: "Submit button label on /build-sources/new",
  },
  "buildSources.createSuccess": {
    message: 'Build-Quelle „{name}“ erstellt',
    description: "Toast shown after creating a Build Source",
  },
  "buildSources.createError": {
    message:
      "Build-Quelle konnte nicht erstellt werden. Bitte versuche es erneut.",
    description: "Fallback error toast when createBuildSource fails",
  },
  "buildSources.updateSuccess": {
    message: 'Build-Quelle „{name}“ aktualisiert',
    description: "Toast shown after updating a Build Source",
  },
  "buildSources.updateError": {
    message:
      "Build-Quelle konnte nicht aktualisiert werden. Bitte versuche es erneut.",
    description: "Fallback error toast when updateBuildSource fails",
  },
  "buildSources.deleteSuccess": {
    message: 'Build-Quelle „{name}“ gelöscht',
    description: "Toast shown after deleting a Build Source",
  },
  "buildSources.deleteError": {
    message:
      "Build-Quelle konnte nicht gelöscht werden. Bitte versuche es erneut.",
    description: "Fallback error toast when deleteBuildSource fails",
  },
  "buildSources.triggerBuildSuccess": {
    message: "Build gestartet",
    description: "Toast shown after triggering a Build Source build",
  },
  "buildSources.triggerBuildError": {
    message: "Build konnte nicht gestartet werden. Bitte versuche es erneut.",
    description: "Fallback error toast when triggerBuild fails",
  },
  "buildSources.createPermissionDenied": {
    message:
      "Du benötigst Entwickler- oder Admin-Rechte in diesem Workspace, um eine Build-Quelle zu erstellen.",
    description: "Read-only notice shown on /build-sources/new when can_create is false",
  },
  "buildSources.createBlockedSource": {
    message: "Wähle ein Git-Repository oder Docker-Image aus, um fortzufahren.",
    description: "Action bar hint while no source is selected",
  },
  "buildSources.partiallyLinkedTitle": {
    message: "Build-Quelle mit teilweisen Dienstverknüpfungen erstellt",
    description: "Alert heading when createBuildSource returns 409 BUILD_SOURCE_PARTIALLY_LINKED",
  },
  "buildSources.partiallyLinkedDescription": {
    message:
      "Die Build-Quelle {id} wurde erstellt, aber nicht alle ausgewählten Dienste konnten verknüpft werden.",
    description: "Alert summary naming the newly created bds- ID on partial link conflict",
  },
  "buildSources.partiallyLinkedSucceeded": {
    message: "Verknüpft: {services}",
    description: "List of services that were successfully linked before the conflict",
  },
  "buildSources.partiallyLinkedFailed": {
    message: "Verknüpfung fehlgeschlagen: {service}",
    description: "The service whose link failed during create",
  },
  "buildSources.fieldSource": {
    message: "Quelle",
    description: "Create form label for Git repository vs Docker image picker",
  },
  "buildSources.fieldSourceDescription": {
    message:
      "Wähle ein verbundenes GitHub-Repository, eine öffentliche Git-URL oder ein bestehendes Docker-Image.",
    description: "Help text for the source picker row on /build-sources/new",
  },
  "buildSources.fieldName": {
    message: "Name",
    description: "Create form label for Build Source name",
  },
  "buildSources.fieldNameDescription": {
    message: "Eindeutiger Name innerhalb dieses Workspaces.",
    description: "Help text for Build Source name",
  },
  "buildSources.fieldNamePlaceholder": {
    message: "my-shared-api",
    description: "Placeholder for Build Source name input",
  },
  "buildSources.errorNameTaken": {
    message:
      'Eine Build-Quelle mit dem Namen „{name}“ existiert in diesem Workspace bereits.',
    description: "Inline error when BUILD_SOURCE_NAME_TAKEN is returned",
  },
  "buildSources.fieldProject": {
    message: "Projekt",
    description: "Create form label for optional Project scope",
  },
  "buildSources.fieldProjectDescription": {
    message: "Optional auf ein einzelnes Projekt beschränken.",
    description: "Help text for optional Project selector",
  },
  "buildSources.fieldProjectNone": {
    message: "Keines (Workspace-weit)",
    description: "Option leaving the build source unscoped at workspace level",
  },
  "buildSources.fieldRuntime": {
    message: "Laufzeit",
    description: "Create form label for Git build source runtime",
  },
  "buildSources.fieldRuntimeDescription": {
    message: "Wähle eine native Laufzeit oder Docker.",
    description: "Help text for Git build source runtime selector",
  },
  "buildSources.fieldBranch": {
    message: "Branch",
    description: "Create form label for Git branch",
  },
  "buildSources.fieldBranchDescription": {
    message:
      "Der Git-Branch für den Build. Standardmäßig der Standard-Branch des Repositories.",
    description: "Help text for Git branch input",
  },
  "buildSources.fieldRegion": {
    message: "Region",
    description: "Create form label for Git build source region",
  },
  "buildSources.fieldRegionDescription": {
    message:
      "Git-Build-Quellen müssen in derselben Region liegen wie verknüpfte Dienste.",
    description: "Help text for Git build source region selector",
  },
  "buildSources.fieldRegionDefault": {
    message: "Standardregion des Workspaces",
    description: "Option leaving the build source region unset to use the platform default",
  },
  "buildSources.fieldGitRegistryDescription": {
    message:
      "Optionale Zugangsdaten zum Abrufen privater Basis-Images bei Docker-Builds.",
    description: "Help text for registry credential on Docker Git build sources",
  },
  "buildSources.fieldEnvGroups": {
    message: "Umgebungsgruppen zur Build-Zeit",
    description: "Label for selecting workspace env groups in build-time environment",
  },
  "buildSources.fieldEnvGroupsDescription": {
    message:
      "Verknüpfe Workspace-weite Umgebungsgruppen für die Build-Zeit. Umgebungsgebundene Gruppen sind nicht zulässig.",
    description: "Help text for build-time env group checkboxes",
  },
  "buildSources.fieldEnvGroupsEmpty": {
    message: "Keine Workspace-weiten Umgebungsgruppen verfügbar.",
    description: "Empty message when no workspace-level env groups exist",
  },
  "buildSources.linkServicesTitle": {
    message: "Bestehende Dienste verknüpfen",
    description: "Section heading for optional initial serviceIds linking",
  },
  "buildSources.linkServicesDescription": {
    message:
      "Verknüpfe optional bestehende Web Services, Private Services, Background Workers oder Cron Jobs. Static Sites unterstützen keine Build-Quellen.",
    description: "Section description for optional initial serviceIds linking",
  },
  "buildSources.linkServicesEmpty": {
    message: "Noch keine verknüpfbaren Dienste in diesem Workspace.",
    description: "Empty message when workspace has no linkable services",
  },
  "buildSources.projectSectionTitle": {
    message: "Build-Quellen",
    description: "Heading of the Build Sources section on Project Overview",
  },
  "buildSources.projectLoadError": {
    message: "Build-Quellen für dieses Projekt konnten nicht geladen werden.",
    description: "Error text inside Project Overview Build Sources section",
  },
  "buildSources.projectEmptyDescription": {
    message: "Diesem Projekt sind noch keine Build-Quellen zugeordnet.",
    description: "Empty state text inside Project Overview Build Sources section",
  },
};

export default deBuildSources;
