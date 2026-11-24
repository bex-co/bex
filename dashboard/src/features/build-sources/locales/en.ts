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
};

export default enBuildSources;
