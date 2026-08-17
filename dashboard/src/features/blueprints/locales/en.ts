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
    message: "New Blueprint",
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
    message: "Blueprint path",
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
  "blueprints.previewRetry": {
    message: "Retry",
    description: "Review section retry button after a failed manifest fetch",
  },
  "blueprints.previewInvalid": {
    message: "Blueprint file has errors",
    description:
      "Review section error title when the fetched manifest fails validation",
  },
  "blueprints.previewValid": {
    message: "Blueprint file parsed successfully — {count} resources to sync.",
    description:
      "Review section success line, with the plan's total resource count",
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
  "blueprints.generateDownload": {
    message: "Download render.yaml",
    description: "Generate dialog — download the manifest file",
  },
  "blueprints.generateSecretsNote": {
    message:
      "Secret values are never exported — secret-backed variables appear as sync: false and are prompted for when the Blueprint is first created.",
    description: "Generate dialog note under the yaml preview",
  },
  "blueprints.generateError": {
    message: "Couldn't generate the Blueprint",
    description: "Toast when the generate query fails",
  },
  // --- Empty state ---
  "blueprints.emptyTitle": {
    message: "No blueprints yet",
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
  // --- Detail page ---
  "blueprints.detailTitle": {
    message: "{name} · Blueprints",
    description: "Blueprint detail page document title",
  },
  "blueprints.metaRepo": {
    message: "Repository",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaBranch": {
    message: "Branch",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaPath": {
    message: "Manifest path",
    description:
      "Blueprint detail metadata label — path to render.yaml in repo",
  },
  "blueprints.metaAutoSync": {
    message: "Auto-sync",
    description: "Blueprint detail metadata label — auto-sync on push toggle",
  },
  "blueprints.metaCreated": {
    message: "Created",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaUpdated": {
    message: "Last synced",
    description: "Blueprint detail metadata label",
  },
  "blueprints.autoSyncOn": {
    message: "On",
    description: "Auto-sync toggle label — enabled",
  },
  "blueprints.autoSyncOff": {
    message: "Off",
    description: "Auto-sync toggle label — disabled",
  },
  // --- Resources section ---
  "blueprints.resourcesTitle": {
    message: "Managed Resources",
    description:
      "Blueprint detail section heading listing services/databases managed by this blueprint",
  },
  "blueprints.resourcesEmpty": {
    message: "No resources yet — sync the Blueprint to apply your render.yaml.",
    description: "Blueprint managed-resources empty state",
  },
  // --- Sync history ---
  "blueprints.syncHistoryTitle": {
    message: "Sync History",
    description: "Blueprint detail section heading for the sync run table",
  },
  "blueprints.syncHistoryEmpty": {
    message: "No syncs yet.",
    description: "Blueprint sync history empty state",
  },
  "blueprints.syncColCommit": {
    message: "Commit",
    description: "Sync history table column — commit SHA",
  },
  "blueprints.syncColState": {
    message: "State",
    description: "Sync history table column — sync run state",
  },
  "blueprints.syncColStarted": {
    message: "Started",
    description: "Sync history table column — sync run start time",
  },
  "blueprints.syncColCompleted": {
    message: "Completed",
    description: "Sync history table column — sync run completion time",
  },
  "blueprints.manifestTitle": {
    message: "render.yaml manifest",
    description: "Blueprint detail section heading for the stored manifest",
  },
  // --- Sync action ---
  "blueprints.syncButton": {
    message: "Sync",
    description: "Button label that triggers an idempotent blueprint re-apply",
  },
  "blueprints.syncConfirmTitle": {
    message: "Sync blueprint?",
    description: "Sync confirm dialog title",
  },
  "blueprints.syncConfirmBody": {
    message:
      "This re-applies the stored render.yaml to your workspace. The apply is idempotent — resources that already match the manifest are not replaced.",
    description: "Sync confirm dialog description",
  },
  "blueprints.syncConfirmAction": {
    message: "Sync",
    description: "Sync confirm dialog primary action button label",
  },
  "blueprints.syncCancel": {
    message: "Cancel",
    description: "Sync confirm dialog cancel button label",
  },
  "blueprints.syncPreviewLoading": {
    message: "Computing the sync plan from the repository…",
    description: "Pre-sync dialog — loading line while blueprintPreview runs",
  },
  "blueprints.syncPreviewInvalid": {
    message:
      "The Blueprint file currently on the branch has problems — syncing now may fail or fall back to the stored manifest:",
    description:
      "Pre-sync dialog — heading above validation errors from the fetched manifest",
  },
  "blueprints.syncPreviewUnavailable": {
    message:
      "Couldn't compute the sync plan (preview unavailable). You can still sync — the backend re-validates before applying.",
    description:
      "Pre-sync dialog — graceful-degrade warning when the preview query fails",
  },
  "blueprints.syncSuccess": {
    message: "Blueprint synced",
    description: "Toast shown after a successful syncBlueprint call",
  },
  "blueprints.syncError": {
    message: "Sync failed",
    description: "Toast shown when syncBlueprint returns an error",
  },
  // --- Update action ---
  "blueprints.editField": {
    message: "Edit {field}",
    description: "Aria label for the inline-edit pencil button",
  },
  "blueprints.saveField": {
    message: "Save {field}",
    description: "Aria label for the inline-edit save button",
  },
  "blueprints.cancelEdit": {
    message: "Cancel edit",
    description: "Aria label for the inline-edit cancel button",
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
  "blueprints.updateError": {
    message: "Update failed",
    description: "Toast shown when updateBlueprint returns an error",
  },
  // --- Disconnect action ---
  "blueprints.disconnectButton": {
    message: "Disconnect",
    description:
      "Button that disconnects a blueprint from its Git repo (stops auto-sync, keeps resources)",
  },
  "blueprints.disconnectTitle": {
    message: "Disconnect blueprint?",
    description: "Disconnect confirm dialog title",
  },
  "blueprints.disconnectBody": {
    message:
      "This stops auto-sync on push and removes the blueprint from your list. Resources already deployed remain untouched.",
    description: "Disconnect confirm dialog description",
  },
  "blueprints.disconnectAction": {
    message: "Disconnect",
    description: "Disconnect confirm dialog primary action button label",
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
};

export default enBlueprints;
