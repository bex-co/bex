import type { TranslationEntry } from "@/i18n";

const enBuildSources: Record<string, TranslationEntry> = {
  "buildSources.pageTitle": {
    message: "Build Sources",
    description: "Title of the Build Sources list page",
  },
  "buildSources.pageDescription": {
    message:
      "Build once from a Git repository or Docker image and deploy the artifact across multiple services.",
    description: "Subtitle of the Build Sources list page",
  },
  "buildSources.resourceType": {
    message: "Build Source",
    description: "Singular resource type label shown in global search results",
  },
  "buildSources.newButton": {
    message: "New Build Source",
    description: "Primary CTA button to create a new Build Source",
  },
  "buildSources.backToList": {
    message: "Back to Build Sources",
    description: "Back button label returning to /build-sources",
  },
  "buildSources.viewAll": {
    message: "View all",
    description: "Link from Project Overview Build Sources section to /build-sources",
  },
  "buildSources.viewBuildSources": {
    message: "View Build Sources",
    description: "Button in partial-link alert navigating to /build-sources",
  },
  "buildSources.emptyTitle": {
    message: "No build sources yet",
    description: "Empty state heading when the workspace has no build sources",
  },
  "buildSources.emptyDescription": {
    message:
      "Create a build source to share a single Git build or Docker image across web services, private services, workers, and cron jobs.",
    description: "Empty state explanation on /build-sources",
  },
  "buildSources.refreshError": {
    message: "Couldn't refresh build sources. Showing cached results.",
    description: "Inline error banner when a background refresh fails over cached rows",
  },
  "buildSources.searchPlaceholder": {
    message: "Search by name, repository, image, or project…",
    description: "Placeholder for the /build-sources search input",
  },
  "buildSources.searchLabel": {
    message: "Search build sources",
    description: "Accessible label for the /build-sources search input",
  },
  "buildSources.resetFilters": {
    message: "Reset filters",
    description: "Button to clear active search and kind filters on /build-sources",
  },
  "buildSources.filterKindLabel": {
    message: "Filter by source kind",
    description: "Accessible label for the source-kind filter button group",
  },
  "buildSources.filterAll": {
    message: "All",
    description: "Filter button showing both Git and Docker image build sources",
  },
  "buildSources.kindGit": {
    message: "Git",
    description: "Source kind badge and filter label for Git-backed build sources",
  },
  "buildSources.kindImage": {
    message: "Docker image",
    description: "Source kind badge and filter label for Docker-image-backed build sources",
  },
  "buildSources.noMatchesTitle": {
    message: "No matching build sources",
    description: "Heading when search or kind filter matches zero build sources",
  },
  "buildSources.noMatchesDescription": {
    message: "Try adjusting your search query or clearing the source-kind filter.",
    description: "Body copy when search or kind filter matches zero build sources",
  },
  "buildSources.colName": {
    message: "Name",
    description: "Table column header for Build Source name",
  },
  "buildSources.colSource": {
    message: "Source",
    description: "Table column header for Git repo/branch or Docker image",
  },
  "buildSources.colRegion": {
    message: "Region",
    description: "Table column header for Build Source region",
  },
  "buildSources.colLinkedServices": {
    message: "Linked Services",
    description: "Table column header for linked service count",
  },
  "buildSources.colProject": {
    message: "Project",
    description: "Table column header for optional project scope",
  },
  "buildSources.colUpdated": {
    message: "Updated",
    description: "Table column header for last updated timestamp",
  },
  "buildSources.copyId": {
    message: "Copy build source ID",
    description: "Accessible label for copying a bds- ID",
  },
  "buildSources.copied": {
    message: "Copied",
    description: "Copy button confirmation tooltip",
  },
  "buildSources.copyError": {
    message: "Couldn't copy",
    description: "Copy button failure tooltip",
  },
  "buildSources.linkedServicesCount_one": {
    message: "{count} service",
    description: "Singular count of services linked to a build source",
  },
  "buildSources.linkedServicesCount_other": {
    message: "{count} services",
    description: "Plural count of services linked to a build source",
  },
  "buildSources.workspaceScope": {
    message: "Workspace",
    description: "Label when a build source is workspace-wide (not scoped to a project)",
  },
  "buildSources.createTitle": {
    message: "New Build Source",
    description: "Page heading of /build-sources/new",
  },
  "buildSources.createDescription": {
    message:
      "Configure a shared Git repository build or Docker image that multiple services can deploy from.",
    description: "Subtitle of /build-sources/new",
  },
  "buildSources.createSubmit": {
    message: "Create Build Source",
    description: "Submit button label on /build-sources/new",
  },
  "buildSources.createSuccess": {
    message: 'Created build source "{name}"',
    description: "Toast shown after creating a Build Source",
  },
  "buildSources.createError": {
    message: "Couldn't create build source. Please try again.",
    description: "Fallback error toast when createBuildSource fails",
  },
  "buildSources.updateSuccess": {
    message: 'Updated build source "{name}"',
    description: "Toast shown after updating a Build Source",
  },
  "buildSources.updateError": {
    message: "Couldn't update build source. Please try again.",
    description: "Fallback error toast when updateBuildSource fails",
  },
  "buildSources.deleteSuccess": {
    message: 'Deleted build source "{name}"',
    description: "Toast shown after deleting a Build Source",
  },
  "buildSources.deleteError": {
    message: "Couldn't delete build source. Please try again.",
    description: "Fallback error toast when deleteBuildSource fails",
  },
  "buildSources.triggerBuildSuccess": {
    message: "Build triggered",
    description: "Toast shown after triggering a Build Source build",
  },
  "buildSources.triggerBuildError": {
    message: "Couldn't trigger build. Please try again.",
    description: "Fallback error toast when triggerBuild fails",
  },
  "buildSources.createPermissionDenied": {
    message:
      "You need Developer or Admin permissions in this workspace to create a build source.",
    description: "Read-only notice shown on /build-sources/new when can_create is false",
  },
  "buildSources.createBlockedSource": {
    message: "Choose a Git repository or Docker image to continue.",
    description: "Action bar hint while no source is selected",
  },
  "buildSources.partiallyLinkedTitle": {
    message: "Build source created with partial service links",
    description: "Alert heading when createBuildSource returns 409 BUILD_SOURCE_PARTIALLY_LINKED",
  },
  "buildSources.partiallyLinkedDescription": {
    message:
      "Build source {id} was created, but not all selected services could be linked.",
    description: "Alert summary naming the newly created bds- ID on partial link conflict",
  },
  "buildSources.partiallyLinkedSucceeded": {
    message: "Linked: {services}",
    description: "List of services that were successfully linked before the conflict",
  },
  "buildSources.partiallyLinkedFailed": {
    message: "Failed to link: {service}",
    description: "The service whose link failed during create",
  },
  "buildSources.fieldSource": {
    message: "Source",
    description: "Create form label for Git repository vs Docker image picker",
  },
  "buildSources.fieldSourceDescription": {
    message: "Choose a connected GitHub repository, a public Git URL, or an existing Docker image.",
    description: "Help text for the source picker row on /build-sources/new",
  },
  "buildSources.fieldName": {
    message: "Name",
    description: "Create form label for Build Source name",
  },
  "buildSources.fieldNameDescription": {
    message: "Unique name within this workspace.",
    description: "Help text for Build Source name",
  },
  "buildSources.fieldNamePlaceholder": {
    message: "my-shared-api",
    description: "Placeholder for Build Source name input",
  },
  "buildSources.errorNameTaken": {
    message: 'A build source named "{name}" already exists in this workspace.',
    description: "Inline error when BUILD_SOURCE_NAME_TAKEN is returned",
  },
  "buildSources.fieldProject": {
    message: "Project",
    description: "Create form label for optional Project scope",
  },
  "buildSources.fieldProjectDescription": {
    message: "Optionally scope this build source to a single project.",
    description: "Help text for optional Project selector",
  },
  "buildSources.fieldProjectNone": {
    message: "None (workspace-wide)",
    description: "Option leaving the build source unscoped at workspace level",
  },
  "buildSources.fieldRuntime": {
    message: "Runtime",
    description: "Create form label for Git build source runtime",
  },
  "buildSources.fieldRuntimeDescription": {
    message: "Choose a native runtime or Docker.",
    description: "Help text for Git build source runtime selector",
  },
  "buildSources.fieldBranch": {
    message: "Branch",
    description: "Create form label for Git branch",
  },
  "buildSources.fieldBranchDescription": {
    message: "The Git branch to build from. Defaults to the repository default branch.",
    description: "Help text for Git branch input",
  },
  "buildSources.fieldRegion": {
    message: "Region",
    description: "Create form label for Git build source region",
  },
  "buildSources.fieldRegionDescription": {
    message: "Git build sources must match the region of linked services.",
    description: "Help text for Git build source region selector",
  },
  "buildSources.fieldRegionDefault": {
    message: "Default workspace region",
    description: "Option leaving the build source region unset to use the platform default",
  },
  "buildSources.fieldGitRegistryDescription": {
    message: "Optional credential for pulling private base images during Docker builds.",
    description: "Help text for registry credential on Docker Git build sources",
  },
  "buildSources.fieldEnvGroups": {
    message: "Build-time Environment Groups",
    description: "Label for selecting workspace env groups in build-time environment",
  },
  "buildSources.fieldEnvGroupsDescription": {
    message:
      "Link workspace-level environment groups into build time. Environment-scoped groups are not eligible.",
    description: "Help text for build-time env group checkboxes",
  },
  "buildSources.fieldEnvGroupsEmpty": {
    message: "No workspace-level environment groups available.",
    description: "Empty message when no workspace-level env groups exist",
  },
  "buildSources.linkServicesTitle": {
    message: "Link existing services",
    description: "Section heading for optional initial serviceIds linking",
  },
  "buildSources.linkServicesDescription": {
    message:
      "Optionally link existing web services, private services, background workers, or cron jobs. Static sites cannot use build sources.",
    description: "Section description for optional initial serviceIds linking",
  },
  "buildSources.linkServicesEmpty": {
    message: "No linkable services in this workspace yet.",
    description: "Empty message when workspace has no linkable services",
  },
  "buildSources.projectSectionTitle": {
    message: "Build Sources",
    description: "Heading of the Build Sources section on Project Overview",
  },
  "buildSources.projectLoadError": {
    message: "Couldn't load build sources for this project.",
    description: "Error text inside Project Overview Build Sources section",
  },
  "buildSources.projectEmptyDescription": {
    message: "No build sources are scoped to this project yet.",
    description: "Empty state text inside Project Overview Build Sources section",
  },
  "buildSources.viewBuildSource": {
    message: "Open Build Source",
    description: "Button navigating to a newly created Build Source detail page",
  },
  "buildSources.detailTabsLabel": {
    message: "Build source sections",
    description: "Accessible label for the Build Source detail tab navigation",
  },
  "buildSources.tabBuilds": {
    message: "Builds",
    description: "Tab label for Build Source build history",
  },
  "buildSources.tabServices": {
    message: "Services",
    description: "Tab label for Build Source linked services",
  },
  "buildSources.tabEnvironment": {
    message: "Environment",
    description: "Tab label for Build Source build-time environment",
  },
  "buildSources.tabSettings": {
    message: "Settings",
    description: "Tab label for Build Source settings and danger zone",
  },
  "buildSources.triggerBuildButton": {
    message: "Trigger Build",
    description: "Primary CTA to trigger a new build on a Git build source",
  },
  "buildSources.triggerSubmitting": {
    message: "Triggering…",
    description: "Submit button label while triggerBuild mutation is in flight",
  },
  "buildSources.triggerDialogTitle": {
    message: "Trigger Build",
    description: "Title of the Trigger Build modal dialog",
  },
  "buildSources.triggerDialogDescription": {
    message: 'Start a new build for "{name}" and choose which linked services should deploy the result.',
    description: "Description of the Trigger Build modal dialog",
  },
  "buildSources.triggerScopeLegend": {
    message: "Deploy after build succeeds",
    description: "Legend for the deploy.serviceScope radio group",
  },
  "buildSources.triggerScopeAutoDeploy": {
    message: "Auto-deploy linked services",
    description: "Radio option for deploy.serviceScope=autodeploy_only",
  },
  "buildSources.triggerScopeAutoDeployHint": {
    message: "Deploy every linked service with auto-deploy enabled.",
    description: "Hint text for deploy.serviceScope=autodeploy_only",
  },
  "buildSources.triggerScopeNone": {
    message: "Build only (don't deploy)",
    description: "Radio option for deploy.serviceScope=none",
  },
  "buildSources.triggerScopeNoneHint": {
    message: "Produce the build artifact without deploying any linked service.",
    description: "Hint text for deploy.serviceScope=none",
  },
  "buildSources.triggerScopeCustom": {
    message: "Selected services",
    description: "Radio option for deploy.serviceScope=custom",
  },
  "buildSources.triggerScopeCustomHint": {
    message: "Choose specific linked services to deploy once the build succeeds.",
    description: "Hint text for deploy.serviceScope=custom",
  },
  "buildSources.triggerScopeCustomNoServices": {
    message: "No services are linked to this build source yet.",
    description: "Message inside custom deploy scope when serviceLinks is empty",
  },
  "buildSources.triggerScopeCustomSelectOne": {
    message: "Select at least one service to deploy.",
    description: "Validation hint when custom scope has zero checked services",
  },
  "buildSources.triggerClearCache": {
    message: "Clear build cache",
    description: "Checkbox label for clearCache=clear",
  },
  "buildSources.triggerClearCacheHint": {
    message: "Build from scratch without reusing cached layers.",
    description: "Hint text for clearCache checkbox",
  },
  "buildSources.triggerResultTitle": {
    message: "Build started",
    description: "Confirmation heading in Trigger Build dialog after success",
  },
  "buildSources.triggerResultDescription": {
    message: "Build {id} has been queued.",
    description: "Confirmation summary naming the new bld- ID",
  },
  "buildSources.triggerResultDeploysTitle_one": {
    message: "{count} service deploy queued",
    description: "Singular heading listing fan-out deploys queued by triggerBuild",
  },
  "buildSources.triggerResultDeploysTitle_other": {
    message: "{count} service deploys queued",
    description: "Plural heading listing fan-out deploys queued by triggerBuild",
  },
  "buildSources.triggerResultNoDeploys": {
    message: "No service deploys were queued for this build.",
    description: "Message when triggerBuild returned an empty deploys array",
  },
  "buildSources.triggerResultSkippedTitle_one": {
    message: "{count} linked service skipped",
    description: "Singular alert heading when triggerBuild returns skippedServices",
  },
  "buildSources.triggerResultSkippedTitle_other": {
    message: "{count} linked services skipped",
    description: "Plural alert heading when triggerBuild returns skippedServices",
  },
  "buildSources.triggerResultViewLogs": {
    message: "View build logs",
    description: "Primary action navigating from Trigger Build confirmation to Build Detail",
  },
  "buildSources.skippedReasonSuspended": {
    message: "Suspended",
    description: "Badge label when a service was skipped because it is suspended",
  },
  "buildSources.buildStatusSucceeded": {
    message: "Succeeded",
    description: "Build status badge for succeeded builds",
  },
  "buildSources.buildStatusFailed": {
    message: "Failed",
    description: "Build status badge for failed builds",
  },
  "buildSources.buildStatusCanceled": {
    message: "Canceled",
    description: "Build status badge for canceled builds",
  },
  "buildSources.buildStatusInProgress": {
    message: "Building",
    description: "Build status badge for in-progress builds",
  },
  "buildSources.buildStatusCreated": {
    message: "Queued",
    description: "Build status badge for newly queued builds",
  },
  "buildSources.buildStatusUnknown": {
    message: "Unknown",
    description: "Fallback build status badge",
  },
  "buildSources.buildsEmptyTitle": {
    message: "No builds yet",
    description: "Empty state heading on the Builds tab",
  },
  "buildSources.buildsEmptyGitDescription": {
    message: "Trigger a build manually or push to the tracked branch to create your first build artifact.",
    description: "Empty state description on Builds tab for Git build sources",
  },
  "buildSources.buildsEmptyImageDescription": {
    message: "Docker image build sources record image revisions when linked services deploy.",
    description: "Empty state description on Builds tab for image build sources",
  },
  "buildSources.buildsRefreshError": {
    message: "Couldn't refresh builds. Showing cached results.",
    description: "Inline error banner when polling builds fails over cached rows",
  },
  "buildSources.buildsLoadMore": {
    message: "Load more builds",
    description: "Pagination button loading older builds",
  },
  "buildSources.colBuildId": {
    message: "Build ID",
    description: "Table header for build ID column",
  },
  "buildSources.colStatus": {
    message: "Status",
    description: "Table header for build status column",
  },
  "buildSources.colArtifact": {
    message: "Commit / Image",
    description: "Table header for commit SHA or image digest column",
  },
  "buildSources.colDuration": {
    message: "Duration",
    description: "Table header for build duration column",
  },
  "buildSources.colDeploys": {
    message: "Deploys",
    description: "Table header for fan-out deploys count column",
  },
  "buildSources.colStarted": {
    message: "Started",
    description: "Table header for build start timestamp column",
  },
  "buildSources.buildDeploysCount_one": {
    message: "{count} deploy",
    description: "Singular badge count of fan-out deploys on a build",
  },
  "buildSources.buildDeploysCount_other": {
    message: "{count} deploys",
    description: "Plural badge count of fan-out deploys on a build",
  },
  "buildSources.buildSkippedCount_one": {
    message: "{count} skipped",
    description: "Singular badge count of skipped services on a build",
  },
  "buildSources.buildSkippedCount_other": {
    message: "{count} skipped",
    description: "Plural badge count of skipped services on a build",
  },
  "buildSources.backToBuilds": {
    message: "Back to Builds",
    description: "Back link from Build Detail to the Builds tab",
  },
  "buildSources.copyBuildId": {
    message: "Copy build ID",
    description: "Accessible label for copying a bld- ID",
  },
  "buildSources.buildDetailSubtitle": {
    message: "Build for {name}",
    description: "Subtitle under the bld- ID on Build Detail",
  },
  "buildSources.buildDetailRefreshError": {
    message: "Couldn't refresh build details. Showing last loaded state.",
    description: "Inline error banner when build detail polling fails",
  },
  "buildSources.factStarted": {
    message: "Started",
    description: "Fact label for build start timestamp",
  },
  "buildSources.factCommit": {
    message: "Commit",
    description: "Fact label for build commit SHA",
  },
  "buildSources.factArtifact": {
    message: "Artifact",
    description: "Fact label for built image reference and digest",
  },
  "buildSources.fanoutTitle": {
    message: "Linked service deploys",
    description: "Card heading listing fan-out deploys on Build Detail",
  },
  "buildSources.fanoutEmpty": {
    message: "No services were deployed from this build yet.",
    description: "Empty message inside fan-out deploys card",
  },
  "buildSources.buildLogsSearchLabel": {
    message: "Filter build logs",
    description: "Accessible label for search input in BuildLogPanel",
  },
  "buildSources.buildLogsEmptyTitle": {
    message: "No build logs yet",
    description: "Empty state heading in BuildLogPanel",
  },
  "buildSources.buildLogsEmptyBody": {
    message: "Logs for this build will appear here as soon as the build starts emitting output.",
    description: "Empty state body in BuildLogPanel",
  },
  "buildSources.imageBuildLogsNotice": {
    message: "Docker image build sources reuse an existing image and do not produce build logs.",
    description: "Notice shown on Build Detail for image-backed build sources",
  },
  "buildSources.servicesTabTitle": {
    message: "Linked Services",
    description: "Heading of the Linked Services tab",
  },
  "buildSources.servicesTabDescription": {
    message: "Services linked to this build source deploy artifacts produced by its builds.",
    description: "Description of the Linked Services tab",
  },
  "buildSources.linkExistingService": {
    message: "Link existing service",
    description: "Button opening the Link Existing Service dialog",
  },
  "buildSources.servicesEmptyTitle": {
    message: "No linked services",
    description: "Empty state heading on Linked Services tab",
  },
  "buildSources.servicesEmptyDescription": {
    message: "Link existing web services, private services, background workers, or cron jobs to share builds from this source.",
    description: "Empty state description on Linked Services tab",
  },
  "buildSources.autoDeployOn": {
    message: "Auto-deploy on",
    description: "Badge indicating a linked service auto-deploys new builds",
  },
  "buildSources.autoDeployOff": {
    message: "Auto-deploy off",
    description: "Badge indicating a linked service requires manual deploys",
  },
  "buildSources.changeOrUnlinkSource": {
    message: "Change source",
    description: "Link to a service's settings page to switch or unlink its source",
  },
  "buildSources.linkDialogTitle": {
    message: "Link existing service",
    description: "Heading of the Link Existing Service dialog",
  },
  "buildSources.linkDialogDescription": {
    message: 'Attach an existing workspace service to "{name}". Linking saves the source without triggering a deploy.',
    description: "Description of the Link Existing Service dialog",
  },
  "buildSources.linkDialogNoEligible": {
    message: "All eligible services in this workspace are already linked, or none exist yet.",
    description: "Message when no linkable unlinked services exist",
  },
  "buildSources.linkDialogServiceLabel": {
    message: "Service",
    description: "Select label in Link Existing Service dialog",
  },
  "buildSources.linkDialogServicePlaceholder": {
    message: "Choose a service…",
    description: "Select placeholder in Link Existing Service dialog",
  },
  "buildSources.linkDialogRegionMismatch": {
    message: "Region mismatch: this Git build source is in {sourceRegion}, while the selected service is in {serviceRegion}.",
    description: "Inline error when linking a Git build source to a service in another region",
  },
  "buildSources.linkDialogStartCommandHint": {
    message: "Required for native-runtime build sources so the service knows how to start.",
    description: "Help text for startCommand input in Link Existing Service dialog",
  },
  "buildSources.linkDialogSubmit": {
    message: "Link service",
    description: "Submit button label in Link Existing Service dialog",
  },
  "buildSources.envImageUnsupportedTitle": {
    message: "Build-time environment is not available for Docker image sources",
    description: "Fallback heading if an image build source visits /environment",
  },
  "buildSources.envImageUnsupportedBody": {
    message: "Only Git-backed build sources run builds with build-time environment variables and secret files.",
    description: "Fallback body if an image build source visits /environment",
  },
  "buildSources.envSaveOnlyNote": {
    message: "Changes are saved without starting a build and apply on the next build.",
    description: "Top note on Build-time Environment tab",
  },
  "buildSources.envVarsDescription": {
    message: "Available during builds of this build source. Changes take effect on the next build.",
    description: "Environment Variables card description on Build-time Environment tab",
  },
  "buildSources.secretFilesDescription": {
    message: "Mounted during builds of this build source. Changes take effect on the next build.",
    description: "Secret Files card description on Build-time Environment tab",
  },
  "buildSources.envGroupsPanelDescription": {
    message: "Workspace-level environment groups linked into build time. Changes apply on the next build without deploying.",
    description: "Environment Groups card description on Build-time Environment tab",
  },
  "buildSources.envGroupsNoneLinked": {
    message: "No environment groups are linked to this build source.",
    description: "Empty state when no env groups are linked to a build source",
  },
  "buildSources.settingsIdentityTitle": {
    message: "Identity & Scope",
    description: "Heading of read-only identity card on Settings tab",
  },
  "buildSources.settingsIdentityDescription": {
    message: "Immutable identifiers and scope for this build source.",
    description: "Description of read-only identity card on Settings tab",
  },
  "buildSources.settingsFieldId": {
    message: "Build Source ID",
    description: "Label for read-only bds- ID field on Settings tab",
  },
  "buildSources.settingsConfigTitle": {
    message: "Build Configuration",
    description: "Heading of editable settings card on Settings tab",
  },
  "buildSources.settingsNoBuildNote": {
    message: "Saving settings updates this build source without starting a build.",
    description: "Inline note explaining Settings edits do not rebuild",
  },
  "buildSources.settingsFieldRepoUrl": {
    message: "Repository URL",
    description: "Input label for Git repository URL on Settings tab",
  },
  "buildSources.settingsInvalidRepoUrl": {
    message: "Enter a valid Git repository URL.",
    description: "Validation error for malformed repo URL on Settings tab",
  },
  "buildSources.settingsInvalidImageUrl": {
    message: "Enter a valid Docker image reference.",
    description: "Validation error for malformed image URL on Settings tab",
  },
  "buildSources.settingsSaveButton": {
    message: "Save changes",
    description: "Submit button label on Settings tab",
  },
  "buildSources.settingsSaving": {
    message: "Saving…",
    description: "Submit button label while updateBuildSource is in flight",
  },
  "buildSources.dangerZoneTitle": {
    message: "Danger Zone",
    description: "Heading of Delete Build Source card on Settings tab",
  },
  "buildSources.dangerZoneDescription": {
    message: "Permanently delete this build source and its build-time configuration. This action cannot be undone.",
    description: "Description of Delete Build Source card",
  },
  "buildSources.deleteBlockedTitle_one": {
    message: "Unlink {count} service before deleting",
    description: "Singular alert heading blocking delete when services remain linked",
  },
  "buildSources.deleteBlockedTitle_other": {
    message: "Unlink {count} services before deleting",
    description: "Plural alert heading blocking delete when services remain linked",
  },
  "buildSources.deleteBlockedDescription": {
    message: "A build source cannot be deleted while services still reference it. Switch each linked service to another source first:",
    description: "Explanation under delete-blocked alert",
  },
  "buildSources.deleteButton": {
    message: "Delete Build Source",
    description: "Destructive button opening the delete confirmation dialog",
  },
  "buildSources.deleteConfirmTitle": {
    message: 'Delete "{name}"?',
    description: "Title of Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmBody": {
    message: "This permanently deletes the build source and its build-time environment. Build history and linked configuration cannot be recovered.",
    description: "Body of Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmPrompt": {
    message: "Type {phrase} below to confirm.",
    description: "Sudo command prompt inside Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmButton": {
    message: "Delete Build Source",
    description: "Confirm action label inside Delete Build Source confirmation dialog",
  },
};

export default enBuildSources;
