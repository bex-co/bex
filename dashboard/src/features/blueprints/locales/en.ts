import type { TranslationEntry } from "@/i18n";

const enBlueprints: Record<string, TranslationEntry> = {
  "blueprints.resourceType": {
    message: "Blueprint",
    description: "Blueprint resource type used in document titles",
  },
  // --- List page ---
  "blueprints.pageTitle": {
    message: "Blueprints",
    description: "Blueprints list page heading and document title",
  },
  "blueprints.cardTitle": {
    message: "Blueprints",
    description: "Blueprints table card title",
  },
  "blueprints.colName": {
    message: "Name",
    description:
      "Blueprints table column header — blueprint name (derived from repo)",
  },
  "blueprints.colRepo": {
    message: "Repository",
    description: "Blueprints table column header — source repo URL",
  },
  "blueprints.colBranch": {
    message: "Branch",
    description: "Blueprints table column header — tracked branch",
  },
  "blueprints.colStatus": {
    message: "Status",
    description: "Blueprints table column header",
  },
  "blueprints.colUpdated": {
    message: "Last Updated",
    description: "Blueprints table column header — updatedAt relative age",
  },
  // --- Create action ---
  "blueprints.createButton": {
    message: "New Blueprint Instance",
    description: "Button label that opens the New Blueprint dialog",
  },
  "blueprints.createTitle": {
    message: "New Blueprint Instance",
    description: "New Blueprint page title",
  },
  "blueprints.createDescription": {
    message:
      "Connect a repository containing a render.yaml Blueprint to create and sync a full stack of services.",
    description: "New Blueprint page subtitle under the title",
  },
  "blueprints.createSourceTitle": {
    message: "Connect a repository",
    description:
      "New Blueprint page — heading above the GitHub/Public Git source tabs (Render parity)",
  },
  "blueprints.createBranchLabel": {
    message: "Branch",
    description: "New Blueprint page — branch combobox label",
  },
  "blueprints.createBranchPlaceholder": {
    message: "main",
    description: "New Blueprint page — branch combobox placeholder",
  },
  "blueprints.createBranchHint": {
    message: "The repository branch with the render.yaml file.",
    description: "New Blueprint page — branch field helper text",
  },
  "blueprints.createPathLabel": {
    message: "Blueprint Path",
    description: "New Blueprint page — render.yaml path input label",
  },
  "blueprints.createNamePlaceholder": {
    message: "my-stack",
    description: "New Blueprint page — blueprint name placeholder",
  },
  "blueprints.createPathPlaceholder": {
    message: "render.yaml",
    description: "New Blueprint page — manifest path placeholder",
  },
  "blueprints.createPathHint": {
    message:
      "The path to the Blueprint file in your repo — any .yaml/.yml file works (e.g. infra/bex/stack.yaml). Defaults to render.yaml at the root of your repository.",
    description: "New Blueprint page — manifest path helper text",
  },
  "blueprints.createNameLabel": {
    message: "Blueprint Name",
    description: "New Blueprint page — name input label",
  },
  "blueprints.createNameTaken": {
    message:
      "A Blueprint named {name} already exists in this workspace. Choose another name.",
    description:
      "New Blueprint page — shown under the name field when Deploy was refused because an approved Blueprint already has the name (w8/250)",
  },
  "blueprints.createAction": {
    message: "Deploy Blueprint",
    description: "New Blueprint dialog submit button label",
  },
  "blueprints.createCancel": {
    message: "Cancel",
    description: "New Blueprint dialog cancel button label",
  },
  "blueprints.createSuccess": {
    message: "Blueprint created",
    description: "Toast shown after a successful createBlueprint call",
  },
  "blueprints.createError": {
    message: "Blueprint creation failed",
    description: "Toast shown when createBlueprint returns an error",
  },
  // --- Pre-create review (Render's "Review Blueprint configurations") ---
  "blueprints.previewTitle": {
    message: "Review Blueprint configuration",
    description:
      "New Blueprint page — heading of the pre-create fetch + validate section",
  },
  "blueprints.previewSelectSource": {
    message:
      "Select a repository and branch to review its Blueprint file before deploying.",
    description: "Review section placeholder before a source is chosen",
  },
  "blueprints.previewLoading": {
    message: "Fetching {path} from your repository…",
    description: "Review section loading text while the manifest is fetched",
  },
  "blueprints.previewNotFoundTitle": {
    message: "Blueprint file not found",
    description: "Review section error title when the manifest fetch fails",
  },
  "blueprints.previewNotFoundBody": {
    message: "Blueprint file {path} not found on {branch} branch.",
    description:
      "Review section fallback error body when the backend returns no message",
  },
  "blueprints.previewFileNotFoundTitle": {
    message: "Blueprint file not found",
    description:
      "Preview failure title: the repo and branch resolved, the file did not",
  },
  "blueprints.previewFileNotFoundBody": {
    message:
      "{path} does not exist on branch {branch}. Check the path, or commit the file to that branch.",
    description: "Preview failure body for file_not_found",
  },
  "blueprints.previewBranchNotFoundTitle": {
    message: "Branch not found",
    description: "Preview failure title for branch_not_found",
  },
  "blueprints.previewBranchNotFoundBody": {
    message: "Branch {branch} does not exist in this repository.",
    description: "Preview failure body for branch_not_found",
  },
  "blueprints.previewRepoNotFoundTitle": {
    message: "Repository not found",
    description: "Preview failure title for repo_not_found_or_no_access",
  },
  "blueprints.previewRepoNotFoundBody": {
    message:
      "This repository does not exist, or bex's GitHub app cannot access it. Check the repository and the app's repository access.",
    description: "Preview failure body for repo_not_found_or_no_access",
  },
  "blueprints.previewAccessDeniedTitle": {
    message: "Access denied",
    description: "Preview failure title for access_denied",
  },
  "blueprints.previewAccessDeniedBody": {
    message:
      "bex's GitHub app is not authorized to read this repository. Re-check the installation and the repositories it is granted.",
    description: "Preview failure body for access_denied",
  },
  "blueprints.previewPublicRepoNotFoundBody": {
    message:
      "This repository does not exist on {host}, or it is not public. bex reads a Blueprint there from a public repository.",
    description:
      "Preview failure body for repo_not_found_or_no_access on gitlab.com or bitbucket.org, read anonymously; {host} is that host",
  },
  "blueprints.previewPublicAccessDeniedBody": {
    message:
      "{host} did not let bex read this repository. bex reads a Blueprint there from a public repository.",
    description:
      "Preview failure body for access_denied on gitlab.com or bitbucket.org, read anonymously; {host} is that host",
  },
  "blueprints.previewHostUnsupportedTitle": {
    message: "Git host not supported",
    description:
      "Preview failure title for a repository on a Git host bex reads no Blueprint from",
  },
  "blueprints.previewHostUnsupportedBody": {
    message:
      "bex reads a Blueprint from a public repository on github.com, gitlab.com or bitbucket.org, as Render does. {host} is not one of them.",
    description:
      "Preview failure body for a repository on a Git host bex reads no Blueprint from; {host} is that host",
  },
  "blueprints.previewRateLimitedTitle": {
    message: "{host} is rate-limiting bex",
    description:
      "Preview failure title for rate_limited; {host} is GitHub or the repository's host",
  },
  "blueprints.previewRateLimitedBody": {
    message:
      "Too many requests to {host} right now. Try again in a few minutes.",
    description:
      "Preview failure body for rate_limited; {host} is GitHub or the repository's host",
  },
  "blueprints.previewAmbiguousTitle": {
    message: "Two Blueprint files",
    description: "Preview failure title for ambiguous_filename",
  },
  "blueprints.previewAmbiguousBody": {
    message:
      "This repository has both render.yaml and bex.yml on branch {branch}. Remove one, or name the file explicitly above.",
    description: "Preview failure body for ambiguous_filename",
  },
  "blueprints.previewUnavailableTitle": {
    message: "Could not reach {host}",
    description:
      "Preview failure title for unavailable; {host} is GitHub or the repository's host",
  },
  "blueprints.previewUnavailableBody": {
    message:
      "bex could not read the Blueprint file from {host}. Try again in a few minutes.",
    description:
      "Preview failure body for unavailable; {host} is GitHub or the repository's host",
  },
  "blueprints.previewInvalidPathExtension": {
    message: "The Blueprint path must be a .yaml or .yml file.",
    description: "Inline path-field error for invalid_path, wrong extension",
  },
  "blueprints.previewInvalidPathShape": {
    message:
      "The Blueprint path must be a clean, repository-relative path (no leading slash, no ..).",
    description: "Inline path-field error for invalid_path, bad shape",
  },
  "blueprints.previewRetry": {
    message: "Retry",
    description: "Review section retry button after a failed manifest fetch",
  },
  "blueprints.takeoverResourceTitle": {
    message: "Take over “{name}”?",
    description:
      "Takeover dialog title when a resource is managed by another Blueprint (BLUEPRINT_RESOURCE_CONFLICT)",
  },
  "blueprints.takeoverResourceBody": {
    message:
      "{kind} “{name}” is managed by Blueprint {owner}. Continuing transfers it to this Blueprint. Type the command below to continue.",
    description: "Takeover dialog body for a Blueprint resource conflict",
  },
  "blueprints.takeoverResourceBodyOtherOwner": {
    message:
      "{kind} “{name}” is managed by another Blueprint. Continuing transfers it to this Blueprint. Type the command below to continue.",
    description:
      "Takeover dialog body for a Blueprint resource conflict whose owning Blueprint bex-api could not name",
  },
  "blueprints.previewConflictBody": {
    message:
      "Another Blueprint already manages some of these resources. Deploying transfers them to this Blueprint; you'll confirm exactly what is taken over on the next step.",
    description:
      "Preview alert body on /blueprint/new when the only problem is a takeover conflict (replaces the raw API message)",
  },
  "blueprints.previewConflict": {
    message: "Already managed — deploying will take it over",
    description:
      "Alert title on /blueprint/new when the only preview problem is a resource another Blueprint manages, which an explicit takeover confirmation resolves (w4/m125)",
  },
  "blueprints.previewMatchTitle": {
    message: "Matches existing resources",
    description:
      "Notice title on /blueprint/new when the Blueprint file names resources that already exist in the workspace (w8/m153, Render parity)",
  },
  "blueprints.previewMatchBody": {
    message:
      "This Blueprint matches resources that already exist in your workspace. New resources will be created with the suffix “-{suffix}” added to each name. The existing resources are not changed.",
    description:
      "Notice body on /blueprint/new for matched resources; {suffix} is the four-character resource name suffix, shown after a hyphen as it appears in a name",
  },
  "blueprints.previewMatchListLabel": {
    message: "Already in your workspace:",
    description:
      "Notice on /blueprint/new — label above the list of existing resources the Blueprint file matches",
  },
  "blueprints.previewInvalid": {
    message: "Blueprint file has errors",
    description:
      "Review section error title when the fetched manifest fails validation",
  },
  "blueprints.previewValid_one": {
    message:
      "Blueprint file parsed successfully — {count} resource will change.",
    description:
      "Review section success line, with the plan's total resource count (singular)",
  },
  "blueprints.previewValid_other": {
    message:
      "Blueprint file parsed successfully — {count} resources will change.",
    description:
      "Review section success line, with the plan's total resource count (plural)",
  },
  "blueprints.previewValidEnvironments_one": {
    message:
      "Blueprint file parsed successfully — {count} environment will change.",
    description:
      "Review section success line when only environments change (w9/095, singular)",
  },
  "blueprints.previewValidEnvironments_other": {
    message:
      "Blueprint file parsed successfully — {count} environments will change.",
    description:
      "Review section success line when only environments change (w9/095, plural)",
  },
  "blueprints.previewValidWithEnvironments_one": {
    message:
      "Blueprint file parsed successfully — {count} resource and {environments} will change.",
    description:
      "Review section success line: changing resources (singular) and blueprints.previewEnvironmentCount (w9/095)",
  },
  "blueprints.previewValidWithEnvironments_other": {
    message:
      "Blueprint file parsed successfully — {count} resources and {environments} will change.",
    description:
      "Review section success line: changing resources (plural) and blueprints.previewEnvironmentCount (w9/095)",
  },
  "blueprints.previewEnvironmentCount_one": {
    message: "{count} environment",
    description:
      "The changing environments inside blueprints.previewValidWithEnvironments (singular)",
  },
  "blueprints.previewEnvironmentCount_other": {
    message: "{count} environments",
    description:
      "The changing environments inside blueprints.previewValidWithEnvironments (plural)",
  },
  "blueprints.errorAtLineColumn": {
    message: "line {line}, column {column}",
    description: "Source location beside a Blueprint validation error (w4/151)",
  },
  "blueprints.errorAtLine": {
    message: "line {line}",
    description:
      "Source location beside a Blueprint validation error when only the line is known (w4/151)",
  },
  "blueprints.previewNoChanges": {
    message:
      "Blueprint file parsed successfully — no changes. Every resource already matches this file.",
    description:
      "Review section success line when every planned action is a no-op (w4/m138)",
  },
  "blueprints.previewOpCreate": {
    message: "Create",
    description:
      "Plan row badge: the resource does not exist yet and will be created",
  },
  "blueprints.previewOpUpdate": {
    message: "Update",
    description: "Plan row badge: an existing resource will be changed",
  },
  "blueprints.previewOpNoop": {
    message: "No change",
    description: "Plan row badge: the resource already matches the file",
  },
  "blueprints.previewOpError": {
    message: "Can't apply",
    description:
      "Plan row badge: the planner refused this resource (reason below)",
  },
  "blueprints.previewChangedFields": {
    message: "Changes: {fields}",
    description:
      "Plan row detail for an update: the changed field paths (never values)",
  },
  "blueprints.previewKindService": {
    message: "Service",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindPostgres": {
    message: "Postgres",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindKeyValue": {
    message: "Key Value",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindEnvGroup": {
    message: "Environment group",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindBuildSource": {
    message: "Build source",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindEnvironment": {
    message: "Environment",
    description: "Plan row resource kind: a projects[].environments[] entry",
  },
  "blueprints.previewServices": {
    message: "Services",
    description: "Review section plan group label — services in the manifest",
  },
  "blueprints.previewDatabases": {
    message: "Databases",
    description: "Review section plan group label — databases in the manifest",
  },
  "blueprints.previewKeyValue": {
    message: "Key Value",
    description:
      "Review section plan group label — key-value instances in the manifest",
  },
  "blueprints.previewEnvGroups": {
    message: "Environment Groups",
    description: "Review section plan group label — env groups in the manifest",
  },
  "blueprints.previewManualSyncNote": {
    message:
      "bex doesn't receive this repo's pushes, so later changes to your Blueprint file won't sync automatically. Sync the Blueprint after each change, or connect the repo through the GitHub app.",
    description:
      "Review section note for a repo whose pushes can't reach bex (the preview's autoSync is false): the Blueprint is created without auto-sync",
  },
  "blueprints.previewAutoSyncNote": {
    message:
      "All future updates to your Blueprint file on this branch will be synced automatically. You can pause auto-sync after creation.",
    description:
      "Review section note about auto-sync being enabled by default (Render parity)",
  },
  "blueprints.previewError": {
    message: "Couldn't load the Blueprint preview.",
    description:
      "Review section error shown when the preview query itself fails (network)",
  },
  // --- sync:false secret prompts (w8/m21, Render's create-time prompt) ---
  "blueprints.promptTitle": {
    message: "Secret values",
    description:
      "Review section — heading for the sync:false env var prompt inputs",
  },
  "blueprints.promptHint": {
    message:
      "This Blueprint declares environment variables with sync: false — their values live only in bex, never in the repo. Set them now; they are seeded once and later syncs never overwrite them.",
    description: "sync:false prompt section helper text",
  },
  "blueprints.promptEmptyWarning": {
    message:
      "Blank values deploy as unset — the service may fail until you set them in its Environment tab.",
    description:
      "Warning below the sync:false inputs when at least one is blank",
  },
  // --- Estimated pricing (Render's Blueprint pricing panel, w8/m18) ---
  "blueprints.pricingTitle": {
    message: "Estimated pricing",
    description: "Review section — estimated-pricing panel heading",
  },
  "blueprints.pricingSubtitle": {
    message:
      "Compute costs are prorated by the second. Billing occurs at the start of the month.",
    description:
      "Estimated-pricing panel subtitle explaining billing semantics (Render parity)",
  },
  "blueprints.pricingLineAmount": {
    message: "({tier}) ${amount} / month",
    description:
      "Estimated-pricing row amount — plan label + monthly dollar cost",
  },
  "blueprints.pricingLineBreakdown": {
    message: "Instance ${instance} + Disk ({gb} GB) ${storage}",
    description:
      "Estimated-pricing datastore row sub-line — instance vs provisioned-storage cost breakdown",
  },
  "blueprints.pricingVariable": {
    message: "Variable",
    description:
      "Estimated-pricing row amount for resources whose cost depends on runtime behavior",
  },
  "blueprints.pricingReasonAutoscaling": {
    message: "Autoscaling",
    description:
      "Estimated-pricing variable-row badge — service has autoscaling enabled",
  },
  "blueprints.pricingReasonMultiInstance": {
    message: "Multiple instances",
    description:
      "Estimated-pricing variable-row badge — service declares numInstances > 1",
  },
  "blueprints.pricingReasonCron": {
    message: "Cron job",
    description:
      "Estimated-pricing variable-row badge — cron jobs bill only while runs execute",
  },
  "blueprints.pricingTotalLabel": {
    message: "Total",
    description: "Estimated-pricing panel total row label",
  },
  "blueprints.pricingTotalAmount": {
    message: "${amount}{marker} per month",
    description:
      "Estimated-pricing panel total amount; marker is an asterisk when variable costs are excluded",
  },
  "blueprints.pricingExclusions": {
    message: "* Excluding {items}.",
    description:
      "Estimated-pricing footnote listing the variable costs excluded from the total",
  },
  "blueprints.pricingExcludeAutoscaling": {
    message: "autoscaling",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingExcludeMultiInstance": {
    message: "additional fixed instances",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingExcludeCron": {
    message: "cron jobs",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingEstimateNote": {
    message:
      "Estimate only — storage is priced on provisioned size; your invoice meters actual usage.",
    description:
      "Estimated-pricing disclaimer: forward estimate is an upper bound of metered billing",
  },
  "blueprints.generateButton": {
    message: "Generate Blueprint",
    description: "Blueprints list header — opens the export dialog (w8/m22)",
  },
  "blueprints.generateTitle": {
    message: "Generate Blueprint",
    description: "Generate dialog title",
  },
  "blueprints.generateDescription": {
    message:
      "Select existing resources to export as a render.yaml you can commit to a repo and connect as a Blueprint.",
    description: "Generate dialog description",
  },
  "blueprints.generateEmptyHint": {
    message: "Select at least one resource to generate from.",
    description: "Generate dialog hint when nothing is selected",
  },
  "blueprints.generateAction": {
    message: "Generate",
    description: "Generate dialog primary action",
  },
  "blueprints.generateBack": {
    message: "Back",
    description: "Generate dialog — return from preview to selection",
  },
  "blueprints.generateCopy": {
    message: "Copy",
    description: "Generate dialog — copy the yaml to the clipboard",
  },
  "blueprints.generateCopied": {
    message: "render.yaml copied",
    description: "Toast after copying the generated manifest",
  },
  "blueprints.generateCopyFailed": {
    message:
      "Couldn't copy render.yaml. Allow clipboard access and try again, or download it.",
    description:
      "Toast when the browser refuses to copy the generated manifest to the clipboard",
  },
  "blueprints.generateDownload": {
    message: "Download render.yaml",
    description: "Generate dialog — download the manifest file",
  },
  "blueprints.generateSecretsNote": {
    message:
      "Stored values are never exported. Service variables marked sync: false are prompted for when the Blueprint is first created. Variables marked generateValue — including exported environment-group keys — keep an existing value, or get a generated one if missing.",
    description: "Generate dialog note under the yaml preview",
  },
  "blueprints.generateError": {
    message: "Couldn't generate the Blueprint",
    description: "Toast when the generate query fails",
  },
  // --- Empty state ---
  "blueprints.emptyTitle": {
    message: "You haven't created any Blueprint instances yet.",
    description: "Blueprints list empty-state heading",
  },
  "blueprints.emptyBody": {
    message:
      "Blueprints auto-register whenever you deploy a repo-backed render.yaml. Once deployed, your stack appears here and you can sync or validate it.",
    description:
      "Blueprints list empty-state body explaining auto-registration",
  },
  // --- Loading / error ---
  "blueprints.loadingBody": {
    message: "Loading blueprints…",
    description: "Blueprints list loading state label",
  },
  "blueprints.errorTitle": {
    message: "Couldn't load blueprints",
    description: "Blueprints list error state heading",
  },
  // --- Status badges ---
  "blueprints.statusActive": {
    message: "Active",
    description: "Blueprint status badge — row is active",
  },
  "blueprints.statusInSync": {
    message: "In Sync",
    description: "Blueprint status badge — git-connected and in sync",
  },
  "blueprints.statusSyncing": {
    message: "Syncing",
    description: "Blueprint status badge — sync in progress",
  },
  "blueprints.statusError": {
    message: "Error",
    description: "Blueprint status badge — last sync failed",
  },
  "blueprints.statusPaused": {
    message: "Paused",
    description: "Blueprint status badge — auto-sync paused",
  },
  "blueprints.statusUnknown": {
    message: "{status}",
    description: "Blueprint status badge fallback — raw status value",
  },
  "blueprints.resourceSuffixLabel": {
    message: "Resource name suffix",
    description:
      "Tooltip and screen-reader label of the suffix shown beside a Blueprint's name in the list and on its detail page (w8/m153)",
  },
  // --- Detail page ---
  "blueprints.detailTitle": {
    message: "{name} · Blueprints",
    description: "Blueprint detail page document title",
  },
  "blueprints.metaRepo": {
    message: "Repository",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaCreated": {
    message: "Created",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaUpdated": {
    message: "Last synced",
    description: "Blueprint detail metadata label",
  },
  "blueprints.detailsTitle": {
    message: "Details",
    description:
      "Blueprint Settings page: title of bex's card of read-only facts (repository, created, last synced) under Render's Settings card (w3/m679)",
  },
  "blueprints.settingsNameHint": {
    message: "A unique name for your Blueprint.",
    description:
      "Blueprint Settings card: help under Blueprint Name (Render's copy, w3/m679)",
  },
  "blueprints.settingsRequired": {
    message: "Required",
    description:
      "Blueprint Settings card: shown under a row left empty (Render's copy, w3/m679)",
  },
  "blueprints.settingsNameTaken": {
    message: "Name already in use",
    description:
      "Blueprint Settings card: shown under Blueprint Name when another Blueprint of the workspace has the name (Render's copy, w3/m679)",
  },
  "blueprints.settingsAutoSync": {
    message: "Auto Sync",
    description:
      "Blueprint Settings card: the Auto Sync row's label (Render's copy, w3/m679)",
  },
  "blueprints.settingsAutoSyncHint": {
    message:
      'Automatically sync changes to your Blueprint file? Select "No" to handle syncs manually.',
    description:
      "Blueprint Settings card: help under Auto Sync; quotes the No option (Render's copy, w3/m679)",
  },
  "blueprints.settingsYes": {
    message: "Yes",
    description:
      "Blueprint Settings card: Auto Sync option, on (Render's copy, w3/m679)",
  },
  "blueprints.settingsNo": {
    message: "No",
    description:
      "Blueprint Settings card: Auto Sync option, off (Render's copy, w3/m679)",
  },
  "blueprints.settingsBranch": {
    message: "Branch",
    description: "Blueprint Settings card: the Branch row's label (w3/m679)",
  },
  "blueprints.settingsBranchHint": {
    message: "The Git branch to build and deploy.",
    description:
      "Blueprint Settings card: help under Branch (Render's copy, w3/m679)",
  },
  "blueprints.settingsBranchNotFound": {
    message: "Branch not found in this repository. Choose another branch.",
    description:
      "Blueprint Settings card: shown under Branch when the repository has no such branch (Render's copy, w3/m679)",
  },
  "blueprints.settingsPath": {
    message: "Blueprint Path",
    description:
      "Blueprint Settings card: the Blueprint Path row's label (Render's copy, w3/m679)",
  },
  "blueprints.settingsPathHint": {
    message:
      "The path to the Blueprint file in your repo (e.g., infra/render.yaml). Defaults to render.yaml at the root of your repository.",
    description:
      "Blueprint Settings card: help under Blueprint Path (Render's copy, w3/m679)",
  },
  "blueprints.settingsFileNotFound": {
    message:
      'Blueprint file "{path}" not found on "{branch}" branch. Please check that the file exists on this branch.',
    description:
      "Blueprint Settings card: shown under Branch or Blueprint Path when the Blueprint file is not at that path on that branch (Render's copy, w3/m679)",
  },
  "blueprints.settingsUpdateFailed": {
    message: "Failed to update Blueprint. Please try again.",
    description:
      "Blueprint Settings card: shown under a row whose save failed with no reason from the server (Render's copy, w3/m679)",
  },
  "blueprints.disconnectNoSync": {
    message:
      "Resources will no longer sync automatically from your Blueprint file.",
    description:
      "Disconnect Blueprint modal: first sentence (Render's copy, w3/m679)",
  },
  "blueprints.disconnectKeeps": {
    message:
      "This will not delete the file itself. This will not delete the managed resources. You can always connect your file later from the Blueprint page.",
    description:
      "Disconnect Blueprint modal: second sentence (Render's copy, w3/m679)",
  },
  "blueprints.disconnectConfirm": {
    message: "Are you sure you want to disconnect your Blueprint spec {name}?",
    description:
      "Disconnect Blueprint modal: the question, with the Blueprint's name in bold (Render's copy, w3/m679)",
  },
  "blueprints.resourcesEmpty": {
    message: "No resources managed by this Blueprint",
    description:
      "A Blueprint's Resources page when it manages nothing, Render's words (w3/m680)",
  },
  "blueprints.resourcesFileProblems_one": {
    message: "There was a problem syncing with your Blueprint file:",
    description:
      "A Blueprint's Resources page when its newest sync failed on its file: the line above the file's errors, Render's 'There was a problem' / 'There were a few problems' (w3/m680)",
  },
  "blueprints.resourcesFileProblems_other": {
    message: "There were a few problems syncing with your Blueprint file:",
    description:
      "A Blueprint's Resources page when its newest sync failed on its file: the line above the file's errors, Render's 'There was a problem' / 'There were a few problems' (w3/m680)",
  },
  "blueprints.colBuildSourceName": {
    message: "Build Source Name",
    description:
      "A Blueprint's Resources page: the Build Sources table's name column, Render's 'Build Source Name' (w3/m680)",
  },
  "blueprints.buildSourceUnlinked": {
    message: "Unlinked",
    description:
      "A Blueprint's Build Sources table: badge on a build source no service builds from, Render's 'Unlinked' (w3/m680)",
  },
  "blueprints.buildSourcesUngrouped": {
    message: "Ungrouped",
    description:
      "A Blueprint's Build Sources table: the row over the build sources in no project, Render's 'Ungrouped' (w3/m680)",
  },
  // --- Syncs (w3/m678) ---
  "blueprints.syncHistoryEmpty": {
    message: "No syncs yet.",
    description: "Syncs page: no syncs yet, Render's wording (w3/m678)",
  },
  "blueprints.detachPreviewTitle": {
    message: "Resources will stop being managed",
    description: "Blueprint resource detachment notice.",
  },
  "blueprints.detachEstimateNote": {
    message: "Their costs are not included in the estimate below.",
    description:
      "Detached resources are outside the proposed manifest estimate.",
  },
  "blueprints.detachWarning": {
    message:
      "These resources remain running and may continue to incur charges. This Blueprint will no longer manage them.",
    description: "Blueprint resource detachment notice.",
  },
  "blueprints.manifestTitle": {
    message: "render.yaml manifest",
    description: "Blueprint detail section heading for the stored manifest",
  },
  // --- A sync's page (w3/m678) ---
  "blueprints.plannedActions": {
    message: "Planned changes",
    description:
      "A sync's page: accessible label of its list of planned actions (w3/m678)",
  },
  "blueprints.plannedAction": {
    message: "{operation} {kind} {name}",
    description:
      "Planned-sync card: one plan action, e.g. 'create service api'",
  },
  "blueprints.plannedManualEnvVars": {
    message:
      "These values are not in the Blueprint file (sync: false). Enter them to approve.",
    description:
      "A waiting sync's page: intro to the fields for the sync: false values approving it needs (w3/m678)",
  },
  "blueprints.plannedManualEnvVarLabel": {
    message: "{service}: {key}",
    description:
      "A waiting sync's page: label of one sync: false value's field (w3/m678)",
  },
  "blueprints.plannedApprove": {
    message: "Approve",
    description:
      "A waiting sync's page: the button that applies the planned sync, Render's Approve (w3/m678)",
  },
  "blueprints.plannedChanged": {
    message:
      "Someone planned this Blueprint again. Review the newest plan, then approve it.",
    description:
      "A sync's page: inline alert when the plan on screen was replaced before it was approved (BLUEPRINT_PLAN_CHANGED)",
  },
  "blueprints.plannedReplanFailed": {
    message: "Could not plan a new sync",
    description:
      "Sync Error banner: Manual sync could not plan a sync and the server gave no reason (w3/m678)",
  },
  // --- Sync action ---
  "blueprints.syncButton": {
    message: "Manual sync",
    description:
      "The Blueprint header's one action, Render's Manual sync: plans a sync and opens its page (w3/m678)",
  },
  // --- Update action ---
  "blueprints.editField": {
    message: "Edit {field}",
    description: "Aria label for the inline-edit pencil button",
  },
  "blueprints.pathInvalid": {
    message:
      "Must be a clean repository-relative .yaml or .yml path (no leading slash, no '..').",
    description: "Inline path editor client-side validation message",
  },
  "blueprints.updateSuccess": {
    message: "Blueprint updated",
    description: "Toast shown after a successful updateBlueprint call",
  },
  // --- Disconnect action ---
  "blueprints.disconnectButton": {
    message: "Disconnect Blueprint",
    description:
      "Blueprint Settings page: the critical button that opens the Disconnect Blueprint modal (Render's copy, w3/m679)",
  },
  "blueprints.disconnectTitle": {
    message: "Disconnect Blueprint",
    description: "Disconnect Blueprint modal title (Render's copy, w3/m679)",
  },
  "blueprints.disconnectAction": {
    message: "Disconnect Blueprint",
    description:
      "Disconnect Blueprint modal: the critical confirm button (Render's copy, w3/m679)",
  },
  "blueprints.disconnectCancel": {
    message: "Cancel",
    description: "Disconnect confirm dialog cancel button label",
  },
  "blueprints.disconnectSuccess": {
    message: "Blueprint disconnected",
    description: "Toast shown after a successful disconnectBlueprint call",
  },
  "blueprints.disconnectError": {
    message: "Disconnect failed",
    description: "Toast shown when disconnectBlueprint returns an error",
  },
  "blueprints.disconnectBusy": {
    message: "A sync is currently applying — retry disconnect after it settles",
    description:
      "Toast shown when disconnectBlueprint refuses with BLUEPRINT_SYNC_BUSY",
  },
  // --- Validate action ---
  "blueprints.validateTitle": {
    message: "Validate",
    description: "Validate panel section heading",
  },
  "blueprints.validateRun": {
    message: "Run validate",
    description: "Validate panel button label",
  },
  "blueprints.validateValid": {
    message: "Manifest is valid — no errors found.",
    description: "Validate result: manifest parsed successfully",
  },
  "blueprints.validateInvalid": {
    message: "Manifest has errors:",
    description:
      "Validate result: manifest has parse errors — followed by the error list",
  },
  "blueprints.validateNoResult": {
    message: "No result yet.",
    description: "Validate panel placeholder before first run",
  },
  "blueprints.stateCreated": {
    message: "Awaiting approval",
    description:
      "Sync state (and Blueprint status) created: a planned sync waiting for approval (w2/m256); never a queued run.",
  },
  "blueprints.stateRunning": {
    message: "Running",
    description: "Sync-run state: the run is in progress.",
  },
  "blueprints.stateSuccess": {
    message: "Success",
    description: "Sync-run state: the run completed successfully.",
  },
  "blueprints.createNameDescription": {
    message: "A unique name for this Blueprint instance.",
    description: "Create-Blueprint Name row description",
  },
  "blueprints.createBlockedSource": {
    message: "Choose a repository to continue.",
    description:
      "Create-Blueprint action bar: why Deploy is held while no repository is chosen",
  },
  "blueprints.createBlockedBranch": {
    message: "Enter a branch to continue.",
    description:
      "Create-Blueprint action bar: why Deploy is held while the branch is empty",
  },
  "blueprints.createBlockedReviewing": {
    message: "Reviewing the Blueprint file…",
    description:
      "Create-Blueprint action bar: why Deploy is held while the Blueprint file is being reviewed",
  },
  "blueprints.createBlockedReview": {
    message: "Resolve the problems the review found to continue.",
    description:
      "Create-Blueprint action bar: why Deploy is held while the review found problems in the Blueprint file",
  },
  "blueprints.navResources": {
    message: "Resources",
    description:
      "Blueprint sidebar entry and page heading for the resources the Blueprint manages (Render's Resources, /blueprints/<id>/resources, w3/m677)",
  },
  "blueprints.navSyncs": {
    message: "Syncs",
    description:
      "Blueprint sidebar entry and page heading for the Blueprint's syncs (Render's Syncs, /blueprints/<id>/syncs, w3/m677)",
  },
  "blueprints.navSettings": {
    message: "Settings",
    description:
      "Blueprint sidebar entry and page heading for the Blueprint's settings (Render's Settings, /blueprints/<id>/settings, w3/m677)",
  },
  "blueprints.resourcesViewSyncs": {
    message: "Go to Syncs",
    description:
      "Link from a Blueprint's empty Resources page to its Syncs page, where the waiting plan or the failed sync is (w3/m677)",
  },
  "blueprints.syncViewDetails": {
    message: "View details",
    description:
      "Sync card: link to the sync's own page (Render's View details, w3/m678)",
  },
  "blueprints.syncsLoadMore": {
    message: "Load more",
    description:
      "Syncs page: button reading the next page of older syncs (w3/m678)",
  },
  "blueprints.syncsLoadMoreError": {
    message: "Couldn't load older syncs.",
    description:
      "Syncs page: toast when reading an older page of syncs fails (w3/m678)",
  },
  "blueprints.syncPageTitle": {
    message: "Sync: {sha}",
    description:
      "A sync's page heading, Render's 'Sync: <short commit id>'; {sha} is the 7-character commit id (w3/m678)",
  },
  "blueprints.syncPageTitleNoCommit": {
    message: "Sync",
    description:
      "A sync's page heading when the sync read no commit, Render's 'Sync' (w3/m678)",
  },
  "blueprints.syncDocumentTitle": {
    message: "Sync",
    description:
      "Document title's page name on a sync's page: 'Sync ・ <name> ・ Blueprint' (w3/m678)",
  },
  "blueprints.syncUpToDate": {
    message: "Resources already up to date",
    description:
      "A sync's page when its plan has no actions, Render's wording (w3/m678)",
  },
  "blueprints.syncNotFullySynced": {
    message: "We weren't able to fully sync these resources.",
    description:
      "A failed sync's page with no actions and no errors, Render's critical line (w3/m678)",
  },
  "blueprints.syncNotFullySyncedBody": {
    message: "If you continue to see this error, contact support.",
    description:
      "Under blueprints.syncNotFullySynced: what to do if it keeps happening (w3/m678)",
  },
  "blueprints.syncActionsNotRecorded": {
    message:
      "This sync was recorded before syncs kept their actions, so they can't be shown.",
    description:
      "A sync's page for a sync recorded before every sync kept its actions (w3/m678)",
  },
  "blueprints.syncActionCreate": {
    message: "Create {kind} {name}",
    description:
      "A sync's planned action: create a resource; {kind} is its kind label, {name} its name, drawn bold (w3/m678)",
  },
  "blueprints.syncActionUpdate": {
    message: "Update {kind} {name}",
    description:
      "A sync's planned action: change a resource; {kind} is its kind label, {name} its name (w3/m678)",
  },
  "blueprints.syncActionDetach": {
    message: "Stop managing {kind} {name}",
    description:
      "A sync's planned action: stop managing a resource the file no longer names; {kind}, {name} (w3/m678)",
  },
  "blueprints.syncActionError": {
    message: "Can't apply {kind} {name}",
    description:
      "A sync's planned action the planner refused; {kind}, {name}; the reason follows (w3/m678)",
  },
  "blueprints.syncActionDetachNote": {
    message: "It keeps running and may incur charges.",
    description:
      "Under a 'stop managing' action: the resource is not deleted (w3/m678)",
  },
  "blueprints.syncActionStateUnknown": {
    message: "State unknown",
    description:
      "Accessible name of a planned action's state icon when bex can't say which state it reached (a failed sync's actions, w3/m678)",
  },
  "blueprints.syncApproveFailed": {
    message: "Couldn't approve this sync. Try again.",
    description:
      "A sync's page: inline alert when Approve fails without the server's reason, Render's wording (w3/m678)",
  },
  "blueprints.syncNotFound": {
    message:
      "This sync doesn't exist. A planned sync not approved within 24 hours is deleted.",
    description:
      "A sync's page for an id that is not one of this Blueprint's syncs, e.g. a plan deleted after 24 hours (w3/m678)",
  },
  "blueprints.syncErrorBannerTitle": {
    message: "Sync Error",
    description:
      "Title of Render's dismissible banner when Manual sync could not plan a sync (w3/m678)",
  },
  "blueprints.syncErrorBannerDismiss": {
    message: "Dismiss",
    description:
      "Accessible name of the Sync Error banner's close button (w3/m678)",
  },
  "blueprints.syncErrorsIntro_one": {
    message: "There was a problem syncing with your Blueprint:",
    description:
      "A sync's page: the line above its errors, Render's 'There was a problem' / 'There were a few problems' (w3/m678)",
  },
  "blueprints.syncErrorsIntro_other": {
    message: "There were a few problems syncing with your Blueprint:",
    description:
      "A sync's page: the line above its errors, Render's 'There was a problem' / 'There were a few problems' (w3/m678)",
  },
};

export default enBlueprints;
