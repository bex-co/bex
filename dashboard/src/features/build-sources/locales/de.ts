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
  "buildSources.viewBuildSource": {
    message: "Build-Quelle öffnen",
    description: "Button navigating to a newly created Build Source detail page",
  },
  "buildSources.detailTabsLabel": {
    message: "Bereiche der Build-Quelle",
    description: "Accessible label for the Build Source detail tab navigation",
  },
  "buildSources.tabBuilds": {
    message: "Build-Verlauf",
    description: "Tab label for Build Source build history",
  },
  "buildSources.tabServices": {
    message: "Dienste",
    description: "Tab label for Build Source linked services",
  },
  "buildSources.tabEnvironment": {
    message: "Umgebung",
    description: "Tab label for Build Source build-time environment",
  },
  "buildSources.tabSettings": {
    message: "Einstellungen",
    description: "Tab label for Build Source settings and danger zone",
  },
  "buildSources.triggerBuildButton": {
    message: "Build auslösen",
    description: "Primary CTA to trigger a new build on a Git build source",
  },
  "buildSources.triggerSubmitting": {
    message: "Wird ausgelöst…",
    description: "Submit button label while triggerBuild mutation is in flight",
  },
  "buildSources.triggerDialogTitle": {
    message: "Build auslösen",
    description: "Title of the Trigger Build modal dialog",
  },
  "buildSources.triggerDialogDescription": {
    message: 'Starte einen neuen Build für „{name}“ und wähle aus, welche verknüpften Dienste das Ergebnis deployen sollen.',
    description: "Description of the Trigger Build modal dialog",
  },
  "buildSources.triggerScopeLegend": {
    message: "Nach erfolgreichem Build deployen",
    description: "Legend for the deploy.serviceScope radio group",
  },
  "buildSources.triggerScopeAutoDeploy": {
    message: "Verknüpfte Dienste automatisch deployen",
    description: "Radio option for deploy.serviceScope=autodeploy_only",
  },
  "buildSources.triggerScopeAutoDeployHint": {
    message: "Deployt alle verknüpften Dienste mit aktiviertem Auto-Deploy.",
    description: "Hint text for deploy.serviceScope=autodeploy_only",
  },
  "buildSources.triggerScopeNone": {
    message: "Nur bauen (nicht deployen)",
    description: "Radio option for deploy.serviceScope=none",
  },
  "buildSources.triggerScopeNoneHint": {
    message: "Erstellt das Build-Artefakt, ohne verknüpfte Dienste zu deployen.",
    description: "Hint text for deploy.serviceScope=none",
  },
  "buildSources.triggerScopeCustom": {
    message: "Ausgewählte Dienste",
    description: "Radio option for deploy.serviceScope=custom",
  },
  "buildSources.triggerScopeCustomHint": {
    message: "Wähle bestimmte verknüpfte Dienste aus, die nach dem Build deployt werden.",
    description: "Hint text for deploy.serviceScope=custom",
  },
  "buildSources.triggerScopeCustomNoServices": {
    message: "Mit dieser Build-Quelle sind noch keine Dienste verknüpft.",
    description: "Message inside custom deploy scope when serviceLinks is empty",
  },
  "buildSources.triggerScopeCustomSelectOne": {
    message: "Wähle mindestens einen Dienst zum Deployen aus.",
    description: "Validation hint when custom scope has zero checked services",
  },
  "buildSources.triggerClearCache": {
    message: "Build-Cache leeren",
    description: "Checkbox label for clearCache=clear",
  },
  "buildSources.triggerClearCacheHint": {
    message: "Baut komplett neu, ohne zwischengespeicherte Layer wiederzuverwenden.",
    description: "Hint text for clearCache checkbox",
  },
  "buildSources.triggerResultTitle": {
    message: "Build gestartet",
    description: "Confirmation heading in Trigger Build dialog after success",
  },
  "buildSources.triggerResultDescription": {
    message: "Build {id} wurde in die Warteschlange gestellt.",
    description: "Confirmation summary naming the new bld- ID",
  },
  "buildSources.triggerResultDeploysTitle_one": {
    message: "{count} Dienst-Deploy eingereiht",
    description: "Singular heading listing fan-out deploys queued by triggerBuild",
  },
  "buildSources.triggerResultDeploysTitle_other": {
    message: "{count} Dienst-Deploys eingereiht",
    description: "Plural heading listing fan-out deploys queued by triggerBuild",
  },
  "buildSources.triggerResultNoDeploys": {
    message: "Für diesen Build wurden keine Dienst-Deploys eingereiht.",
    description: "Message when triggerBuild returned an empty deploys array",
  },
  "buildSources.triggerResultSkippedTitle_one": {
    message: "{count} verknüpfter Dienst übersprungen",
    description: "Singular alert heading when triggerBuild returns skippedServices",
  },
  "buildSources.triggerResultSkippedTitle_other": {
    message: "{count} verknüpfte Dienste übersprungen",
    description: "Plural alert heading when triggerBuild returns skippedServices",
  },
  "buildSources.triggerResultViewLogs": {
    message: "Build-Logs anzeigen",
    description: "Primary action navigating from Trigger Build confirmation to Build Detail",
  },
  "buildSources.skippedReasonSuspended": {
    message: "Pausiert",
    description: "Badge label when a service was skipped because it is suspended",
  },
  "buildSources.buildStatusSucceeded": {
    message: "Erfolgreich",
    description: "Build status badge for succeeded builds",
  },
  "buildSources.buildStatusFailed": {
    message: "Fehlgeschlagen",
    description: "Build status badge for failed builds",
  },
  "buildSources.buildStatusCanceled": {
    message: "Abgebrochen",
    description: "Build status badge for canceled builds",
  },
  "buildSources.buildStatusInProgress": {
    message: "Wird gebaut",
    description: "Build status badge for in-progress builds",
  },
  "buildSources.buildStatusCreated": {
    message: "In Warteschlange",
    description: "Build status badge for newly queued builds",
  },
  "buildSources.buildStatusUnknown": {
    message: "Unbekannt",
    description: "Fallback build status badge",
  },
  "buildSources.buildsEmptyTitle": {
    message: "Noch keine Builds",
    description: "Empty state heading on the Builds tab",
  },
  "buildSources.buildsEmptyGitDescription": {
    message: "Löse manuell einen Build aus oder pushe auf den verfolgten Branch, um dein erstes Build-Artefakt zu erstellen.",
    description: "Empty state description on Builds tab for Git build sources",
  },
  "buildSources.buildsEmptyImageDescription": {
    message: "Docker-Image-Build-Quellen zeichnen Image-Revisionen auf, sobald verknüpfte Dienste deployt werden.",
    description: "Empty state description on Builds tab for image build sources",
  },
  "buildSources.buildsRefreshError": {
    message: "Builds konnten nicht aktualisiert werden. Zwischengespeicherte Ergebnisse werden angezeigt.",
    description: "Inline error banner when polling builds fails over cached rows",
  },
  "buildSources.buildsLoadMore": {
    message: "Weitere Builds laden",
    description: "Pagination button loading older builds",
  },
  "buildSources.colBuildId": {
    message: "Build-ID",
    description: "Table header for build ID column",
  },
  "buildSources.colStatus": {
    message: "Zustand",
    description: "Table header for build status column",
  },
  "buildSources.colArtifact": {
    message: "Commit / Image",
    description: "Table header for commit SHA or image digest column",
  },
  "buildSources.colDuration": {
    message: "Dauer",
    description: "Table header for build duration column",
  },
  "buildSources.colDeploys": {
    message: "Deployments",
    description: "Table header for fan-out deploys count column",
  },
  "buildSources.colStarted": {
    message: "Gestartet",
    description: "Table header for build start timestamp column",
  },
  "buildSources.buildDeploysCount_one": {
    message: "{count} Deployment",
    description: "Singular badge count of fan-out deploys on a build",
  },
  "buildSources.buildDeploysCount_other": {
    message: "{count} Deployments",
    description: "Plural badge count of fan-out deploys on a build",
  },
  "buildSources.buildSkippedCount_one": {
    message: "{count} übersprungen",
    description: "Singular badge count of skipped services on a build",
  },
  "buildSources.buildSkippedCount_other": {
    message: "{count} übersprungen",
    description: "Plural badge count of skipped services on a build",
  },
  "buildSources.backToBuilds": {
    message: "Zurück zum Build-Verlauf",
    description: "Back link from Build Detail to the Builds tab",
  },
  "buildSources.copyBuildId": {
    message: "Build-ID kopieren",
    description: "Accessible label for copying a bld- ID",
  },
  "buildSources.buildDetailSubtitle": {
    message: "Build für {name}",
    description: "Subtitle under the bld- ID on Build Detail",
  },
  "buildSources.buildDetailRefreshError": {
    message: "Build-Details konnten nicht aktualisiert werden. Letzter geladener Stand wird angezeigt.",
    description: "Inline error banner when build detail polling fails",
  },
  "buildSources.factStarted": {
    message: "Gestartet",
    description: "Fact label for build start timestamp",
  },
  "buildSources.factCommit": {
    message: "Git-Commit",
    description: "Fact label for build commit SHA",
  },
  "buildSources.factArtifact": {
    message: "Artefakt",
    description: "Fact label for built image reference and digest",
  },
  "buildSources.fanoutTitle": {
    message: "Deploys verknüpfter Dienste",
    description: "Card heading listing fan-out deploys on Build Detail",
  },
  "buildSources.fanoutEmpty": {
    message: "Aus diesem Build wurden noch keine Dienste deployt.",
    description: "Empty message inside fan-out deploys card",
  },
  "buildSources.buildLogsSearchLabel": {
    message: "Build-Logs filtern",
    description: "Accessible label for search input in BuildLogPanel",
  },
  "buildSources.buildLogsEmptyTitle": {
    message: "Noch keine Build-Logs",
    description: "Empty state heading in BuildLogPanel",
  },
  "buildSources.buildLogsEmptyBody": {
    message: "Logs für diesen Build erscheinen hier, sobald der Build Ausgaben schreibt.",
    description: "Empty state body in BuildLogPanel",
  },
  "buildSources.imageBuildLogsNotice": {
    message: "Docker-Image-Build-Quellen verwenden ein vorhandenes Image weiter und erzeugen keine Build-Logs.",
    description: "Notice shown on Build Detail for image-backed build sources",
  },
  "buildSources.servicesTabTitle": {
    message: "Verknüpfte Dienste",
    description: "Heading of the Linked Services tab",
  },
  "buildSources.servicesTabDescription": {
    message: "Dienste, die mit dieser Build-Quelle verknüpft sind, deployen die von ihren Builds erzeugten Artefakte.",
    description: "Description of the Linked Services tab",
  },
  "buildSources.linkExistingService": {
    message: "Bestehenden Dienst verknüpfen",
    description: "Button opening the Link Existing Service dialog",
  },
  "buildSources.servicesEmptyTitle": {
    message: "Keine verknüpften Dienste",
    description: "Empty state heading on Linked Services tab",
  },
  "buildSources.servicesEmptyDescription": {
    message: "Verknüpfe bestehende Web Services, Private Services, Background Workers oder Cron Jobs, um Builds aus dieser Quelle zu teilen.",
    description: "Empty state description on Linked Services tab",
  },
  "buildSources.autoDeployOn": {
    message: "Auto-Deploy an",
    description: "Badge indicating a linked service auto-deploys new builds",
  },
  "buildSources.autoDeployOff": {
    message: "Auto-Deploy aus",
    description: "Badge indicating a linked service requires manual deploys",
  },
  "buildSources.changeOrUnlinkSource": {
    message: "Quelle ändern",
    description: "Link to a service's settings page to switch or unlink its source",
  },
  "buildSources.linkDialogTitle": {
    message: "Bestehenden Dienst verknüpfen",
    description: "Heading of the Link Existing Service dialog",
  },
  "buildSources.linkDialogDescription": {
    message: 'Verbinde einen bestehenden Workspace-Dienst mit „{name}“. Beim Verknüpfen wird die Quelle gespeichert, ohne ein Deploy auszulösen.',
    description: "Description of the Link Existing Service dialog",
  },
  "buildSources.linkDialogNoEligible": {
    message: "Alle zulässigen Dienste in diesem Workspace sind bereits verknüpft oder noch nicht vorhanden.",
    description: "Message when no linkable unlinked services exist",
  },
  "buildSources.linkDialogServiceLabel": {
    message: "Dienst",
    description: "Select label in Link Existing Service dialog",
  },
  "buildSources.linkDialogServicePlaceholder": {
    message: "Dienst auswählen…",
    description: "Select placeholder in Link Existing Service dialog",
  },
  "buildSources.linkDialogRegionMismatch": {
    message: "Region stimmt nicht überein: Diese Git-Build-Quelle liegt in {sourceRegion}, der ausgewählte Dienst jedoch in {serviceRegion}.",
    description: "Inline error when linking a Git build source to a service in another region",
  },
  "buildSources.linkDialogStartCommandHint": {
    message: "Für Build-Quellen mit nativer Runtime erforderlich, damit der Dienst gestartet werden kann.",
    description: "Help text for startCommand input in Link Existing Service dialog",
  },
  "buildSources.linkDialogSubmit": {
    message: "Dienst verknüpfen",
    description: "Submit button label in Link Existing Service dialog",
  },
  "buildSources.envImageUnsupportedTitle": {
    message: "Build-Zeit-Umgebung ist für Docker-Image-Quellen nicht verfügbar",
    description: "Fallback heading if an image build source visits /environment",
  },
  "buildSources.envImageUnsupportedBody": {
    message: "Nur Git-basierte Build-Quellen führen Builds mit Build-Zeit-Umgebungsvariablen und Secret-Dateien aus.",
    description: "Fallback body if an image build source visits /environment",
  },
  "buildSources.envSaveOnlyNote": {
    message: "Änderungen werden ohne Build gespeichert und gelten ab dem nächsten Build.",
    description: "Top note on Build-time Environment tab",
  },
  "buildSources.envVarsDescription": {
    message: "Während der Builds dieser Build-Quelle verfügbar. Änderungen gelten ab dem nächsten Build.",
    description: "Environment Variables card description on Build-time Environment tab",
  },
  "buildSources.secretFilesDescription": {
    message: "Während der Builds dieser Build-Quelle eingebunden. Änderungen gelten ab dem nächsten Build.",
    description: "Secret Files card description on Build-time Environment tab",
  },
  "buildSources.envGroupsPanelDescription": {
    message: "Workspace-weite Umgebungsgruppen für die Build-Zeit. Änderungen gelten ab dem nächsten Build ohne Deploy.",
    description: "Environment Groups card description on Build-time Environment tab",
  },
  "buildSources.envGroupsNoneLinked": {
    message: "Mit dieser Build-Quelle sind keine Umgebungsgruppen verknüpft.",
    description: "Empty state when no env groups are linked to a build source",
  },
  "buildSources.settingsIdentityTitle": {
    message: "Identität & Bereich",
    description: "Heading of read-only identity card on Settings tab",
  },
  "buildSources.settingsIdentityDescription": {
    message: "Unveränderliche Kennungen und Geltungsbereich dieser Build-Quelle.",
    description: "Description of read-only identity card on Settings tab",
  },
  "buildSources.settingsFieldId": {
    message: "Build-Quellen-ID",
    description: "Label for read-only bds- ID field on Settings tab",
  },
  "buildSources.settingsConfigTitle": {
    message: "Build-Konfiguration",
    description: "Heading of editable settings card on Settings tab",
  },
  "buildSources.settingsNoBuildNote": {
    message: "Das Speichern der Einstellungen aktualisiert diese Build-Quelle, ohne einen Build zu starten.",
    description: "Inline note explaining Settings edits do not rebuild",
  },
  "buildSources.settingsFieldRepoUrl": {
    message: "Repository-URL",
    description: "Input label for Git repository URL on Settings tab",
  },
  "buildSources.settingsInvalidRepoUrl": {
    message: "Gib eine gültige Git-Repository-URL ein.",
    description: "Validation error for malformed repo URL on Settings tab",
  },
  "buildSources.settingsInvalidImageUrl": {
    message: "Gib eine gültige Docker-Image-Referenz ein.",
    description: "Validation error for malformed image URL on Settings tab",
  },
  "buildSources.settingsSaveButton": {
    message: "Änderungen speichern",
    description: "Submit button label on Settings tab",
  },
  "buildSources.settingsSaving": {
    message: "Wird gespeichert…",
    description: "Submit button label while updateBuildSource is in flight",
  },
  "buildSources.dangerZoneTitle": {
    message: "Gefahrenzone",
    description: "Heading of Delete Build Source card on Settings tab",
  },
  "buildSources.dangerZoneDescription": {
    message: "Lösche diese Build-Quelle und ihre Build-Zeit-Konfiguration dauerhaft. Diese Aktion kann nicht rückgängig gemacht werden.",
    description: "Description of Delete Build Source card",
  },
  "buildSources.deleteBlockedTitle_one": {
    message: "Trenne zuerst {count} Dienst vor dem Löschen",
    description: "Singular alert heading blocking delete when services remain linked",
  },
  "buildSources.deleteBlockedTitle_other": {
    message: "Trenne zuerst {count} Dienste vor dem Löschen",
    description: "Plural alert heading blocking delete when services remain linked",
  },
  "buildSources.deleteBlockedDescription": {
    message: "Eine Build-Quelle kann nicht gelöscht werden, solange noch Dienste darauf verweisen. Stelle zuerst jeden verknüpften Dienst auf eine andere Quelle um:",
    description: "Explanation under delete-blocked alert",
  },
  "buildSources.deleteButton": {
    message: "Build-Quelle löschen",
    description: "Destructive button opening the delete confirmation dialog",
  },
  "buildSources.deleteConfirmTitle": {
    message: "„{name}“ löschen?",
    description: "Title of Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmBody": {
    message: "Dadurch werden die Build-Quelle und ihre Build-Zeit-Umgebung dauerhaft gelöscht. Build-Verlauf und Konfiguration können nicht wiederhergestellt werden.",
    description: "Body of Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmPrompt": {
    message: "Gib unten {phrase} zur Bestätigung ein.",
    description: "Sudo command prompt inside Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmButton": {
    message: "Build-Quelle löschen",
    description: "Confirm action label inside Delete Build Source confirmation dialog",
  },
};

export default deBuildSources;
