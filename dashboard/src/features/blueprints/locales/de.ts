import type { TranslationEntry } from "@/i18n";

const deBlueprints: Record<string, TranslationEntry> = {
  "blueprints.resourceType": {
    message: "Blueprint",
    description: "Blueprint resource type used in document titles",
  },
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
    message: "Zuletzt aktualisiert",
    description: "Blueprints table column header — updatedAt relative age",
  },
  "blueprints.createButton": {
    message: "Neue Blueprint-Instanz",
    description: "Button label that opens the New Blueprint dialog",
  },
  "blueprints.createTitle": {
    message: "Neue Blueprint-Instanz",
    description: "New Blueprint page title",
  },
  "blueprints.createDescription": {
    message:
      "Verbinde ein Repository mit einem render.yaml-Blueprint, um einen vollständigen Stack aus Services zu erstellen und zu synchronisieren.",
    description: "New Blueprint page subtitle under the title",
  },
  "blueprints.createSourceTitle": {
    message: "Repository verbinden",
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
    message: "Der Repository-Branch mit der render.yaml-Datei.",
    description: "New Blueprint page — branch field helper text",
  },
  "blueprints.createPathLabel": {
    message: "Blueprint-Pfad",
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
      "Der Pfad zur Blueprint-Datei in deinem Repository — jede .yaml-/.yml-Datei funktioniert (z. B. infra/bex/stack.yaml). Standard ist render.yaml im Stammverzeichnis deines Repositorys.",
    description: "New Blueprint page — manifest path helper text",
  },
  "blueprints.createNameLabel": {
    message: "Blueprint-Name",
    description: "New Blueprint page — name input label",
  },
  "blueprints.createNameTaken": {
    message:
      "In diesem Workspace gibt es bereits einen Blueprint namens {name}. Wähle einen anderen Namen.",
    description:
      "New Blueprint page — shown under the name field when Deploy was refused because an approved Blueprint already has the name (w8/250)",
  },
  "blueprints.createAction": {
    message: "Blueprint deployen",
    description: "New Blueprint dialog submit button label",
  },
  "blueprints.createCancel": {
    message: "Abbrechen",
    description: "New Blueprint dialog cancel button label",
  },
  "blueprints.createSuccess": {
    message: "Blueprint erstellt",
    description: "Toast shown after a successful createBlueprint call",
  },
  "blueprints.createError": {
    message: "Blueprint konnte nicht erstellt werden",
    description: "Toast shown when createBlueprint returns an error",
  },
  "blueprints.previewTitle": {
    message: "Blueprint-Konfiguration prüfen",
    description:
      "New Blueprint page — heading of the pre-create fetch + validate section",
  },
  "blueprints.previewSelectSource": {
    message:
      "Wähle ein Repository und einen Branch, um die Blueprint-Datei vor dem Deployen zu prüfen.",
    description: "Review section placeholder before a source is chosen",
  },
  "blueprints.previewLoading": {
    message: "{path} wird aus deinem Repository abgerufen…",
    description: "Review section loading text while the manifest is fetched",
  },
  "blueprints.previewNotFoundTitle": {
    message: "Blueprint-Datei nicht gefunden",
    description: "Review section error title when the manifest fetch fails",
  },
  "blueprints.previewNotFoundBody": {
    message:
      "Blueprint-Datei {path} wurde auf dem Branch {branch} nicht gefunden.",
    description:
      "Review section fallback error body when the backend returns no message",
  },
  "blueprints.previewFileNotFoundTitle": {
    message: "Blueprint-Datei nicht gefunden",
    description:
      "Preview failure title: the repo and branch resolved, the file did not",
  },
  "blueprints.previewFileNotFoundBody": {
    message:
      "{path} existiert auf dem Branch {branch} nicht. Prüfe den Pfad oder committe die Datei auf diesen Branch.",
    description: "Preview failure body for file_not_found",
  },
  "blueprints.previewBranchNotFoundTitle": {
    message: "Branch nicht gefunden",
    description: "Preview failure title for branch_not_found",
  },
  "blueprints.previewBranchNotFoundBody": {
    message: "Der Branch {branch} existiert in diesem Repository nicht.",
    description: "Preview failure body for branch_not_found",
  },
  "blueprints.previewRepoNotFoundTitle": {
    message: "Repository nicht gefunden",
    description: "Preview failure title for repo_not_found_or_no_access",
  },
  "blueprints.previewRepoNotFoundBody": {
    message:
      "Dieses Repository existiert nicht, oder die GitHub App von bex kann nicht darauf zugreifen. Prüfe das Repository und den Repository-Zugriff der App.",
    description: "Preview failure body for repo_not_found_or_no_access",
  },
  "blueprints.previewAccessDeniedTitle": {
    message: "Zugriff verweigert",
    description: "Preview failure title for access_denied",
  },
  "blueprints.previewAccessDeniedBody": {
    message:
      "Die GitHub App von bex ist nicht berechtigt, dieses Repository zu lesen. Prüfe die Installation und die Repositories, auf die sie Zugriff hat.",
    description: "Preview failure body for access_denied",
  },
  "blueprints.previewPublicRepoNotFoundBody": {
    message:
      "Dieses Repository existiert auf {host} nicht oder ist nicht öffentlich. bex liest einen Blueprint dort aus einem öffentlichen Repository.",
    description:
      "Preview failure body for repo_not_found_or_no_access on gitlab.com or bitbucket.org, read anonymously; {host} is that host",
  },
  "blueprints.previewPublicAccessDeniedBody": {
    message:
      "{host} hat bex das Lesen dieses Repositorys verweigert. bex liest einen Blueprint dort aus einem öffentlichen Repository.",
    description:
      "Preview failure body for access_denied on gitlab.com or bitbucket.org, read anonymously; {host} is that host",
  },
  "blueprints.previewHostUnsupportedTitle": {
    message: "Git-Host nicht unterstützt",
    description:
      "Preview failure title for a repository on a Git host bex reads no Blueprint from",
  },
  "blueprints.previewHostUnsupportedBody": {
    message:
      "bex liest einen Blueprint wie Render aus einem öffentlichen Repository auf github.com, gitlab.com oder bitbucket.org. {host} gehört nicht dazu.",
    description:
      "Preview failure body for a repository on a Git host bex reads no Blueprint from; {host} is that host",
  },
  "blueprints.previewRateLimitedTitle": {
    message: "{host} begrenzt die Anfragen von bex",
    description:
      "Preview failure title for rate_limited; {host} is GitHub or the repository's host",
  },
  "blueprints.previewRateLimitedBody": {
    message:
      "Gerade zu viele Anfragen an {host}. Versuche es in ein paar Minuten erneut.",
    description:
      "Preview failure body for rate_limited; {host} is GitHub or the repository's host",
  },
  "blueprints.previewAmbiguousTitle": {
    message: "Zwei Blueprint-Dateien",
    description: "Preview failure title for ambiguous_filename",
  },
  "blueprints.previewAmbiguousBody": {
    message:
      "Dieses Repository enthält auf dem Branch {branch} sowohl render.yaml als auch bex.yml. Entferne eine der Dateien oder gib den Dateinamen oben ausdrücklich an.",
    description: "Preview failure body for ambiguous_filename",
  },
  "blueprints.previewUnavailableTitle": {
    message: "{host} nicht erreichbar",
    description:
      "Preview failure title for unavailable; {host} is GitHub or the repository's host",
  },
  "blueprints.previewUnavailableBody": {
    message:
      "bex konnte die Blueprint-Datei nicht von {host} lesen. Versuche es in ein paar Minuten erneut.",
    description:
      "Preview failure body for unavailable; {host} is GitHub or the repository's host",
  },
  "blueprints.previewInvalidPathExtension": {
    message: "Der Blueprint-Pfad muss auf eine .yaml- oder .yml-Datei zeigen.",
    description: "Inline path-field error for invalid_path, wrong extension",
  },
  "blueprints.previewInvalidPathShape": {
    message:
      "Der Blueprint-Pfad muss ein sauberer, relativer Repository-Pfad sein (kein führender Schrägstrich, kein ..).",
    description: "Inline path-field error for invalid_path, bad shape",
  },
  "blueprints.previewRetry": {
    message: "Erneut versuchen",
    description: "Review section retry button after a failed manifest fetch",
  },
  "blueprints.takeoverResourceTitle": {
    message: "„{name}“ übernehmen?",
    description:
      "Takeover dialog title when a resource is managed by another Blueprint (BLUEPRINT_RESOURCE_CONFLICT)",
  },
  "blueprints.takeoverResourceBody": {
    message:
      "{kind} „{name}“ wird vom Blueprint {owner} verwaltet. Wenn du fortfährst, wird es auf diesen Blueprint übertragen. Gib den Befehl unten ein, um fortzufahren.",
    description: "Takeover dialog body for a Blueprint resource conflict",
  },
  "blueprints.takeoverResourceBodyOtherOwner": {
    message:
      "{kind} „{name}“ wird von einem anderen Blueprint verwaltet. Wenn du fortfährst, wird es auf diesen Blueprint übertragen. Gib den Befehl unten ein, um fortzufahren.",
    description:
      "Takeover dialog body for a Blueprint resource conflict whose owning Blueprint bex-api could not name",
  },
  "blueprints.previewConflictBody": {
    message:
      "Ein anderer Blueprint verwaltet bereits einige dieser Ressourcen. Beim Deployen werden sie auf diesen Blueprint übertragen; im nächsten Schritt bestätigst du genau, was übernommen wird.",
    description:
      "Preview alert body on /blueprint/new when the only problem is a takeover conflict (replaces the raw API message)",
  },
  "blueprints.previewConflict": {
    message: "Bereits verwaltet — beim Deployen wird es übernommen",
    description:
      "Alert title on /blueprint/new when the only preview problem is a resource another Blueprint manages, which an explicit takeover confirmation resolves (w4/m125)",
  },
  "blueprints.previewMatchTitle": {
    message: "Entspricht vorhandenen Ressourcen",
    description:
      "Notice title on /blueprint/new when the Blueprint file names resources that already exist in the workspace (w8/m153, Render parity)",
  },
  "blueprints.previewMatchBody": {
    message:
      "Dieser Blueprint entspricht Ressourcen, die in deinem Workspace bereits existieren. Neue Ressourcen werden mit dem Suffix „-{suffix}“ am Namen erstellt. Die vorhandenen Ressourcen bleiben unverändert.",
    description:
      "Notice body on /blueprint/new for matched resources; {suffix} is the four-character resource name suffix, shown after a hyphen as it appears in a name",
  },
  "blueprints.previewMatchListLabel": {
    message: "Bereits in deinem Workspace:",
    description:
      "Notice on /blueprint/new — label above the list of existing resources the Blueprint file matches",
  },
  "blueprints.previewInvalid": {
    message: "Blueprint-Datei enthält Fehler",
    description:
      "Review section error title when the fetched manifest fails validation",
  },
  "blueprints.previewValid_one": {
    message:
      "Blueprint-Datei erfolgreich geparst — {count} Ressource wird geändert.",
    description:
      "Review section success line, with the plan's total resource count (singular)",
  },
  "blueprints.previewValid_other": {
    message:
      "Blueprint-Datei erfolgreich geparst — {count} Ressourcen werden geändert.",
    description:
      "Review section success line, with the plan's total resource count (plural)",
  },
  "blueprints.previewValidEnvironments_one": {
    message:
      "Blueprint-Datei erfolgreich geparst — {count} Umgebung wird geändert.",
    description:
      "Review section success line when only environments change (w9/095, singular)",
  },
  "blueprints.previewValidEnvironments_other": {
    message:
      "Blueprint-Datei erfolgreich geparst — {count} Umgebungen werden geändert.",
    description:
      "Review section success line when only environments change (w9/095, plural)",
  },
  "blueprints.previewValidWithEnvironments_one": {
    message:
      "Blueprint-Datei erfolgreich geparst — {count} Ressource und {environments} werden geändert.",
    description:
      "Review section success line: changing resources (singular) and blueprints.previewEnvironmentCount (w9/095)",
  },
  "blueprints.previewValidWithEnvironments_other": {
    message:
      "Blueprint-Datei erfolgreich geparst — {count} Ressourcen und {environments} werden geändert.",
    description:
      "Review section success line: changing resources (plural) and blueprints.previewEnvironmentCount (w9/095)",
  },
  "blueprints.previewEnvironmentCount_one": {
    message: "{count} Umgebung",
    description:
      "The changing environments inside blueprints.previewValidWithEnvironments (singular)",
  },
  "blueprints.previewEnvironmentCount_other": {
    message: "{count} Umgebungen",
    description:
      "The changing environments inside blueprints.previewValidWithEnvironments (plural)",
  },
  "blueprints.errorAtLineColumn": {
    message: "Zeile {line}, Spalte {column}",
    description: "Source location beside a Blueprint validation error (w4/151)",
  },
  "blueprints.errorAtLine": {
    message: "Zeile {line}",
    description:
      "Source location beside a Blueprint validation error when only the line is known (w4/151)",
  },
  "blueprints.previewNoChanges": {
    message:
      "Blueprint-Datei erfolgreich geparst — keine Änderungen. Jede Ressource entspricht bereits dieser Datei.",
    description:
      "Review section success line when every planned action is a no-op (w4/m138)",
  },
  "blueprints.previewOpCreate": {
    message: "Erstellen",
    description:
      "Plan row badge: the resource does not exist yet and will be created",
  },
  "blueprints.previewOpUpdate": {
    message: "Aktualisieren",
    description: "Plan row badge: an existing resource will be changed",
  },
  "blueprints.previewOpNoop": {
    message: "Keine Änderung",
    description: "Plan row badge: the resource already matches the file",
  },
  "blueprints.previewOpError": {
    message: "Nicht anwendbar",
    description:
      "Plan row badge: the planner refused this resource (reason below)",
  },
  "blueprints.previewChangedFields": {
    message: "Änderungen: {fields}",
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
    message: "Umgebungsgruppe",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindBuildSource": {
    message: "Build-Quelle",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindEnvironment": {
    message: "Umgebung",
    description: "Plan row resource kind: a projects[].environments[] entry",
  },
  "blueprints.previewServices": {
    message: "Services",
    description: "Review section plan group label — services in the manifest",
  },
  "blueprints.previewDatabases": {
    message: "Datenbanken",
    description: "Review section plan group label — databases in the manifest",
  },
  "blueprints.previewKeyValue": {
    message: "Key Value",
    description:
      "Review section plan group label — key-value instances in the manifest",
  },
  "blueprints.previewEnvGroups": {
    message: "Umgebungsgruppen",
    description: "Review section plan group label — env groups in the manifest",
  },
  "blueprints.previewManualSyncNote": {
    message:
      "bex erhält die Pushes dieses Repositorys nicht, daher werden spätere Änderungen an deiner Blueprint-Datei nicht automatisch synchronisiert. Synchronisiere den Blueprint nach jeder Änderung oder verbinde das Repository über die GitHub App.",
    description:
      "Review section note for a repo whose pushes can't reach bex (the preview's autoSync is false): the Blueprint is created without auto-sync",
  },
  "blueprints.previewAutoSyncNote": {
    message:
      "Alle künftigen Änderungen an deiner Blueprint-Datei auf diesem Branch werden automatisch synchronisiert. Du kannst die Auto-Synchronisierung nach dem Erstellen pausieren.",
    description:
      "Review section note about auto-sync being enabled by default (Render parity)",
  },
  "blueprints.previewError": {
    message: "Die Blueprint-Vorschau konnte nicht geladen werden.",
    description:
      "Review section error shown when the preview query itself fails (network)",
  },
  "blueprints.promptTitle": {
    message: "Secret-Werte",
    description:
      "Review section — heading for the sync:false env var prompt inputs",
  },
  "blueprints.promptHint": {
    message:
      "Dieser Blueprint deklariert Umgebungsvariablen mit sync: false — ihre Werte liegen nur in bex, nie im Repository. Lege sie jetzt fest; sie werden einmalig gesetzt und spätere Synchronisierungen überschreiben sie nie.",
    description: "sync:false prompt section helper text",
  },
  "blueprints.promptEmptyWarning": {
    message:
      "Leere Werte werden als nicht gesetzt deployt — der Service kann fehlschlagen, bis du sie im Tab „Umgebung“ festlegst.",
    description:
      "Warning below the sync:false inputs when at least one is blank",
  },
  "blueprints.pricingTitle": {
    message: "Geschätzte Preise",
    description: "Review section — estimated-pricing panel heading",
  },
  "blueprints.pricingSubtitle": {
    message:
      "Rechenkosten werden sekundengenau anteilig berechnet. Die Abrechnung erfolgt zu Monatsbeginn.",
    description:
      "Estimated-pricing panel subtitle explaining billing semantics (Render parity)",
  },
  "blueprints.pricingLineAmount": {
    message: "({tier}) ${amount} / Monat",
    description:
      "Estimated-pricing row amount — plan label + monthly dollar cost",
  },
  "blueprints.pricingLineBreakdown": {
    message: "Instanz ${instance} + Disk ({gb} GB) ${storage}",
    description:
      "Estimated-pricing datastore row sub-line — instance vs provisioned-storage cost breakdown",
  },
  "blueprints.pricingVariable": {
    message: "Variabel",
    description:
      "Estimated-pricing row amount for resources whose cost depends on runtime behavior",
  },
  "blueprints.pricingReasonAutoscaling": {
    message: "Autoscaling",
    description:
      "Estimated-pricing variable-row badge — service has autoscaling enabled",
  },
  "blueprints.pricingReasonMultiInstance": {
    message: "Mehrere Instanzen",
    description:
      "Estimated-pricing variable-row badge — service declares numInstances > 1",
  },
  "blueprints.pricingReasonCron": {
    message: "Cron Job",
    description:
      "Estimated-pricing variable-row badge — cron jobs bill only while runs execute",
  },
  "blueprints.pricingTotalLabel": {
    message: "Gesamt",
    description: "Estimated-pricing panel total row label",
  },
  "blueprints.pricingTotalAmount": {
    message: "${amount}{marker} pro Monat",
    description:
      "Estimated-pricing panel total amount; marker is an asterisk when variable costs are excluded",
  },
  "blueprints.pricingExclusions": {
    message: "* Ohne {items}.",
    description:
      "Estimated-pricing footnote listing the variable costs excluded from the total",
  },
  "blueprints.pricingExcludeAutoscaling": {
    message: "Autoscaling",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingExcludeMultiInstance": {
    message: "zusätzliche feste Instanzen",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingExcludeCron": {
    message: "Cron Jobs",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingEstimateNote": {
    message:
      "Nur eine Schätzung — Speicher wird nach bereitgestellter Größe berechnet; deine Rechnung misst die tatsächliche Nutzung.",
    description:
      "Estimated-pricing disclaimer: forward estimate is an upper bound of metered billing",
  },
  "blueprints.generateButton": {
    message: "Blueprint generieren",
    description: "Blueprints list header — opens the export dialog (w8/m22)",
  },
  "blueprints.generateTitle": {
    message: "Blueprint generieren",
    description: "Generate dialog title",
  },
  "blueprints.generateDescription": {
    message:
      "Wähle vorhandene Ressourcen aus, die als render.yaml exportiert werden, die du in ein Repository committen und als Blueprint verbinden kannst.",
    description: "Generate dialog description",
  },
  "blueprints.generateEmptyHint": {
    message: "Wähle mindestens eine Ressource zum Generieren aus.",
    description: "Generate dialog hint when nothing is selected",
  },
  "blueprints.generateAction": {
    message: "Generieren",
    description: "Generate dialog primary action",
  },
  "blueprints.generateBack": {
    message: "Zurück",
    description: "Generate dialog — return from preview to selection",
  },
  "blueprints.generateCopy": {
    message: "Kopieren",
    description: "Generate dialog — copy the yaml to the clipboard",
  },
  "blueprints.generateCopied": {
    message: "render.yaml kopiert",
    description: "Toast after copying the generated manifest",
  },
  "blueprints.generateCopyFailed": {
    message:
      "render.yaml konnte nicht kopiert werden. Erlaube den Zugriff auf die Zwischenablage und versuche es erneut, oder lade die Datei herunter.",
    description:
      "Toast when the browser refuses to copy the generated manifest to the clipboard",
  },
  "blueprints.generateDownload": {
    message: "render.yaml herunterladen",
    description: "Generate dialog — download the manifest file",
  },
  "blueprints.generateSecretsNote": {
    message:
      "Gespeicherte Werte werden nie exportiert. Für Service-Variablen mit sync: false wirst du beim ersten Erstellen des Blueprints gefragt. Variablen mit generateValue — einschließlich exportierter Schlüssel von Umgebungsgruppen — behalten einen vorhandenen Wert oder erhalten einen generierten, falls keiner existiert.",
    description: "Generate dialog note under the yaml preview",
  },
  "blueprints.generateError": {
    message: "Der Blueprint konnte nicht generiert werden",
    description: "Toast when the generate query fails",
  },
  "blueprints.emptyTitle": {
    message: "Du hast noch keine Blueprint-Instanzen erstellt.",
    description: "Blueprints list empty-state heading",
  },
  "blueprints.emptyBody": {
    message:
      "Blueprints werden automatisch registriert, sobald du eine repository-basierte render.yaml deployst. Nach dem Deployment erscheint dein Stack hier, und du kannst ihn synchronisieren oder validieren.",
    description:
      "Blueprints list empty-state body explaining auto-registration",
  },
  "blueprints.loadingBody": {
    message: "Blueprints werden geladen…",
    description: "Blueprints list loading state label",
  },
  "blueprints.errorTitle": {
    message: "Blueprints konnten nicht geladen werden",
    description: "Blueprints list error state heading",
  },
  "blueprints.statusActive": {
    message: "Aktiv",
    description: "Blueprint status badge — row is active",
  },
  "blueprints.statusInSync": {
    message: "Synchron",
    description: "Blueprint status badge — git-connected and in sync",
  },
  "blueprints.statusSyncing": {
    message: "Wird synchronisiert",
    description: "Blueprint status badge — sync in progress",
  },
  "blueprints.statusError": {
    message: "Fehler",
    description: "Blueprint status badge — last sync failed",
  },
  "blueprints.statusPaused": {
    message: "Pausiert",
    description: "Blueprint status badge — auto-sync paused",
  },
  "blueprints.statusUnknown": {
    message: "{status}",
    description: "Blueprint status badge fallback — raw status value",
  },
  "blueprints.resourceSuffixLabel": {
    message: "Suffix des Ressourcennamens",
    description:
      "Tooltip and screen-reader label of the suffix shown beside a Blueprint's name in the list and on its detail page (w8/m153)",
  },
  "blueprints.detailTitle": {
    message: "{name} · Blueprints",
    description: "Blueprint detail page document title",
  },
  "blueprints.metaRepo": {
    message: "Repository",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaCreated": {
    message: "Erstellt",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaUpdated": {
    message: "Zuletzt synchronisiert",
    description: "Blueprint detail metadata label",
  },
  "blueprints.detailsTitle": {
    message: "Details",
    description:
      "Blueprint Settings page: title of bex's card of read-only facts (repository, created, last synced) under Render's Settings card (w3/m679)",
  },
  "blueprints.settingsNameHint": {
    message: "Ein eindeutiger Name für deinen Blueprint.",
    description:
      "Blueprint Settings card: help under Blueprint Name (Render's copy, w3/m679)",
  },
  "blueprints.settingsRequired": {
    message: "Erforderlich",
    description:
      "Blueprint Settings card: shown under a row left empty (Render's copy, w3/m679)",
  },
  "blueprints.settingsNameTaken": {
    message: "Name bereits vergeben",
    description:
      "Blueprint Settings card: shown under Blueprint Name when another Blueprint of the workspace has the name (Render's copy, w3/m679)",
  },
  "blueprints.settingsAutoSync": {
    message: "Auto-Sync",
    description:
      "Blueprint Settings card: the Auto Sync row's label (Render's copy, w3/m679)",
  },
  "blueprints.settingsAutoSyncHint": {
    message:
      "Änderungen an deiner Blueprint-Datei automatisch synchronisieren? Wähle „Nein“, um Synchronisierungen manuell auszuführen.",
    description:
      "Blueprint Settings card: help under Auto Sync; quotes the No option (Render's copy, w3/m679)",
  },
  "blueprints.settingsYes": {
    message: "Ja",
    description:
      "Blueprint Settings card: Auto Sync option, on (Render's copy, w3/m679)",
  },
  "blueprints.settingsNo": {
    message: "Nein",
    description:
      "Blueprint Settings card: Auto Sync option, off (Render's copy, w3/m679)",
  },
  "blueprints.settingsBranch": {
    message: "Branch",
    description: "Blueprint Settings card: the Branch row's label (w3/m679)",
  },
  "blueprints.settingsBranchHint": {
    message: "Der Git-Branch, der gebaut und deployt wird.",
    description:
      "Blueprint Settings card: help under Branch (Render's copy, w3/m679)",
  },
  "blueprints.settingsBranchNotFound": {
    message:
      "Branch in diesem Repository nicht gefunden. Wähle einen anderen Branch.",
    description:
      "Blueprint Settings card: shown under Branch when the repository has no such branch (Render's copy, w3/m679)",
  },
  "blueprints.settingsPath": {
    message: "Blueprint-Pfad",
    description:
      "Blueprint Settings card: the Blueprint Path row's label (Render's copy, w3/m679)",
  },
  "blueprints.settingsPathHint": {
    message:
      "Der Pfad zur Blueprint-Datei in deinem Repository (z. B. infra/render.yaml). Standard ist render.yaml im Stammverzeichnis deines Repositorys.",
    description:
      "Blueprint Settings card: help under Blueprint Path (Render's copy, w3/m679)",
  },
  "blueprints.settingsFileNotFound": {
    message:
      "Blueprint-Datei „{path}“ im Branch „{branch}“ nicht gefunden. Prüfe, ob die Datei in diesem Branch existiert.",
    description:
      "Blueprint Settings card: shown under Branch or Blueprint Path when the Blueprint file is not at that path on that branch (Render's copy, w3/m679)",
  },
  "blueprints.settingsUpdateFailed": {
    message:
      "Blueprint konnte nicht aktualisiert werden. Bitte versuche es erneut.",
    description:
      "Blueprint Settings card: shown under a row whose save failed with no reason from the server (Render's copy, w3/m679)",
  },
  "blueprints.disconnectNoSync": {
    message:
      "Ressourcen werden nicht mehr automatisch aus deiner Blueprint-Datei synchronisiert.",
    description:
      "Disconnect Blueprint modal: first sentence (Render's copy, w3/m679)",
  },
  "blueprints.disconnectKeeps": {
    message:
      "Die Datei selbst wird nicht gelöscht. Die verwalteten Ressourcen werden nicht gelöscht. Du kannst deine Datei jederzeit später über die Blueprints-Seite wieder verbinden.",
    description:
      "Disconnect Blueprint modal: second sentence (Render's copy, w3/m679)",
  },
  "blueprints.disconnectConfirm": {
    message:
      "Möchtest du deine Blueprint-Spezifikation {name} wirklich trennen?",
    description:
      "Disconnect Blueprint modal: the question, with the Blueprint's name in bold (Render's copy, w3/m679)",
  },
  "blueprints.resourcesEmpty": {
    message: "Keine Ressourcen von diesem Blueprint verwaltet",
    description:
      "A Blueprint's Resources page when it manages nothing, Render's words (w3/m680)",
  },
  "blueprints.resourcesFileProblems_one": {
    message:
      "Beim Synchronisieren mit deiner Blueprint-Datei ist ein Problem aufgetreten:",
    description:
      "A Blueprint's Resources page when its newest sync failed on its file: the line above the file's errors, Render's 'There was a problem' / 'There were a few problems' (w3/m680)",
  },
  "blueprints.resourcesFileProblems_other": {
    message:
      "Beim Synchronisieren mit deiner Blueprint-Datei sind einige Probleme aufgetreten:",
    description:
      "A Blueprint's Resources page when its newest sync failed on its file: the line above the file's errors, Render's 'There was a problem' / 'There were a few problems' (w3/m680)",
  },
  "blueprints.colBuildSourceName": {
    message: "Name der Build-Quelle",
    description:
      "A Blueprint's Resources page: the Build Sources table's name column, Render's 'Build Source Name' (w3/m680)",
  },
  "blueprints.buildSourceUnlinked": {
    message: "Nicht verknüpft",
    description:
      "A Blueprint's Build Sources table: badge on a build source no service builds from, Render's 'Unlinked' (w3/m680)",
  },
  "blueprints.buildSourcesUngrouped": {
    message: "Nicht gruppiert",
    description:
      "A Blueprint's Build Sources table: the row over the build sources in no project, Render's 'Ungrouped' (w3/m680)",
  },
  "blueprints.syncHistoryEmpty": {
    message: "Noch keine Synchronisierungen.",
    description: "Syncs page: no syncs yet, Render's wording (w3/m678)",
  },
  "blueprints.detachPreviewTitle": {
    message: "Ressourcen werden nicht mehr verwaltet",
    description: "Blueprint resource detachment notice.",
  },
  "blueprints.detachEstimateNote": {
    message: "Ihre Kosten sind in der Schätzung unten nicht enthalten.",
    description:
      "Detached resources are outside the proposed manifest estimate.",
  },
  "blueprints.detachWarning": {
    message:
      "Diese Ressourcen laufen weiter und können weiterhin Kosten verursachen. Dieser Blueprint verwaltet sie nicht mehr.",
    description: "Blueprint resource detachment notice.",
  },
  "blueprints.manifestTitle": {
    message: "render.yaml-Manifest",
    description: "Blueprint detail section heading for the stored manifest",
  },
  "blueprints.plannedActions": {
    message: "Geplante Änderungen",
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
      "Diese Werte stehen nicht in der Blueprint-Datei (sync: false). Gib sie zur Genehmigung ein.",
    description:
      "A waiting sync's page: intro to the fields for the sync: false values approving it needs (w3/m678)",
  },
  "blueprints.plannedManualEnvVarLabel": {
    message: "{service}: {key}",
    description:
      "A waiting sync's page: label of one sync: false value's field (w3/m678)",
  },
  "blueprints.plannedApprove": {
    message: "Genehmigen",
    description:
      "A waiting sync's page: the button that applies the planned sync, Render's Approve (w3/m678)",
  },
  "blueprints.plannedChanged": {
    message:
      "Jemand hat diesen Blueprint erneut geplant. Prüfe den neuesten Plan und genehmige ihn dann.",
    description:
      "A sync's page: inline alert when the plan on screen was replaced before it was approved (BLUEPRINT_PLAN_CHANGED)",
  },
  "blueprints.plannedReplanFailed": {
    message: "Eine neue Synchronisierung konnte nicht geplant werden",
    description:
      "Sync Error banner: Manual sync could not plan a sync and the server gave no reason (w3/m678)",
  },
  "blueprints.syncButton": {
    message: "Manuell synchronisieren",
    description:
      "The Blueprint header's one action, Render's Manual sync: plans a sync and opens its page (w3/m678)",
  },
  "blueprints.editField": {
    message: "{field} bearbeiten",
    description: "Aria label for the inline-edit pencil button",
  },
  "blueprints.pathInvalid": {
    message:
      "Muss ein sauberer, relativer Repository-Pfad zu einer .yaml- oder .yml-Datei sein (kein führender Schrägstrich, kein '..').",
    description: "Inline path editor client-side validation message",
  },
  "blueprints.updateSuccess": {
    message: "Blueprint aktualisiert",
    description: "Toast shown after a successful updateBlueprint call",
  },
  "blueprints.disconnectButton": {
    message: "Blueprint trennen",
    description:
      "Blueprint Settings page: the critical button that opens the Disconnect Blueprint modal (Render's copy, w3/m679)",
  },
  "blueprints.disconnectTitle": {
    message: "Blueprint trennen",
    description: "Disconnect Blueprint modal title (Render's copy, w3/m679)",
  },
  "blueprints.disconnectAction": {
    message: "Blueprint trennen",
    description:
      "Disconnect Blueprint modal: the critical confirm button (Render's copy, w3/m679)",
  },
  "blueprints.disconnectCancel": {
    message: "Abbrechen",
    description: "Disconnect confirm dialog cancel button label",
  },
  "blueprints.disconnectSuccess": {
    message: "Blueprint getrennt",
    description: "Toast shown after a successful disconnectBlueprint call",
  },
  "blueprints.disconnectError": {
    message: "Trennen fehlgeschlagen",
    description: "Toast shown when disconnectBlueprint returns an error",
  },
  "blueprints.disconnectBusy": {
    message:
      "Gerade wird eine Synchronisierung angewendet — versuche das Trennen erneut, nachdem sie abgeschlossen ist",
    description:
      "Toast shown when disconnectBlueprint refuses with BLUEPRINT_SYNC_BUSY",
  },
  "blueprints.validateTitle": {
    message: "Validieren",
    description: "Validate panel section heading",
  },
  "blueprints.validateRun": {
    message: "Validierung ausführen",
    description: "Validate panel button label",
  },
  "blueprints.validateValid": {
    message: "Das Manifest ist gültig — keine Fehler gefunden.",
    description: "Validate result: manifest parsed successfully",
  },
  "blueprints.validateInvalid": {
    message: "Das Manifest enthält Fehler:",
    description:
      "Validate result: manifest has parse errors — followed by the error list",
  },
  "blueprints.validateNoResult": {
    message: "Noch kein Ergebnis.",
    description: "Validate panel placeholder before first run",
  },
  "blueprints.stateCreated": {
    message: "Wartet auf Genehmigung",
    description:
      "Sync state (and Blueprint status) created: a planned sync waiting for approval (w2/m256); never a queued run.",
  },
  "blueprints.stateRunning": {
    message: "Läuft",
    description: "Sync-run state: the run is in progress.",
  },
  "blueprints.stateSuccess": {
    message: "Erfolgreich",
    description: "Sync-run state: the run completed successfully.",
  },
  "blueprints.createNameDescription": {
    message: "Ein eindeutiger Name für diese Blueprint-Instanz.",
    description: "Create-Blueprint Name row description",
  },
  "blueprints.createBlockedSource": {
    message: "Wählen Sie ein Repository, um fortzufahren.",
    description:
      "Create-Blueprint action bar: why Deploy is held while no repository is chosen",
  },
  "blueprints.createBlockedBranch": {
    message: "Geben Sie einen Branch ein, um fortzufahren.",
    description:
      "Create-Blueprint action bar: why Deploy is held while the branch is empty",
  },
  "blueprints.createBlockedReviewing": {
    message: "Blueprint-Datei wird geprüft …",
    description:
      "Create-Blueprint action bar: why Deploy is held while the Blueprint file is being reviewed",
  },
  "blueprints.createBlockedReview": {
    message:
      "Beheben Sie die bei der Prüfung gefundenen Probleme, um fortzufahren.",
    description:
      "Create-Blueprint action bar: why Deploy is held while the review found problems in the Blueprint file",
  },
  "blueprints.navResources": {
    message: "Ressourcen",
    description:
      "Blueprint sidebar entry and page heading for the resources the Blueprint manages (Render's Resources, /blueprints/<id>/resources, w3/m677)",
  },
  "blueprints.navSyncs": {
    message: "Synchronisierungen",
    description:
      "Blueprint sidebar entry and page heading for the Blueprint's syncs (Render's Syncs, /blueprints/<id>/syncs, w3/m677)",
  },
  "blueprints.navSettings": {
    message: "Einstellungen",
    description:
      "Blueprint sidebar entry and page heading for the Blueprint's settings (Render's Settings, /blueprints/<id>/settings, w3/m677)",
  },
  "blueprints.resourcesViewSyncs": {
    message: "Zu den Synchronisierungen",
    description:
      "Link from a Blueprint's empty Resources page to its Syncs page, where the waiting plan or the failed sync is (w3/m677)",
  },
  "blueprints.syncViewDetails": {
    message: "Details anzeigen",
    description:
      "Sync card: link to the sync's own page (Render's View details, w3/m678)",
  },
  "blueprints.syncsLoadMore": {
    message: "Mehr laden",
    description:
      "Syncs page: button reading the next page of older syncs (w3/m678)",
  },
  "blueprints.syncsLoadMoreError": {
    message: "Ältere Synchronisierungen konnten nicht geladen werden.",
    description:
      "Syncs page: toast when reading an older page of syncs fails (w3/m678)",
  },
  "blueprints.syncPageTitle": {
    message: "Synchronisierung: {sha}",
    description:
      "A sync's page heading, Render's 'Sync: <short commit id>'; {sha} is the 7-character commit id (w3/m678)",
  },
  "blueprints.syncPageTitleNoCommit": {
    message: "Synchronisierung",
    description:
      "A sync's page heading when the sync read no commit, Render's 'Sync' (w3/m678)",
  },
  "blueprints.syncDocumentTitle": {
    message: "Synchronisierung",
    description:
      "Document title's page name on a sync's page: 'Sync ・ <name> ・ Blueprint' (w3/m678)",
  },
  "blueprints.syncUpToDate": {
    message: "Ressourcen sind bereits auf dem neuesten Stand",
    description:
      "A sync's page when its plan has no actions, Render's wording (w3/m678)",
  },
  "blueprints.syncNotFullySynced": {
    message: "Wir konnten diese Ressourcen nicht vollständig synchronisieren.",
    description:
      "A failed sync's page with no actions and no errors, Render's critical line (w3/m678)",
  },
  "blueprints.syncNotFullySyncedBody": {
    message:
      "Wenn dieser Fehler weiterhin auftritt, wende dich an den Support.",
    description:
      "Under blueprints.syncNotFullySynced: what to do if it keeps happening (w3/m678)",
  },
  "blueprints.syncActionsNotRecorded": {
    message:
      "Diese Synchronisierung wurde erfasst, bevor Synchronisierungen ihre Aktionen speicherten, daher können sie nicht angezeigt werden.",
    description:
      "A sync's page for a sync recorded before every sync kept its actions (w3/m678)",
  },
  "blueprints.syncActionCreate": {
    message: "{kind} {name} erstellen",
    description:
      "A sync's planned action: create a resource; {kind} is its kind label, {name} its name, drawn bold (w3/m678)",
  },
  "blueprints.syncActionUpdate": {
    message: "{kind} {name} aktualisieren",
    description:
      "A sync's planned action: change a resource; {kind} is its kind label, {name} its name (w3/m678)",
  },
  "blueprints.syncActionDetach": {
    message: "{kind} {name} nicht mehr verwalten",
    description:
      "A sync's planned action: stop managing a resource the file no longer names; {kind}, {name} (w3/m678)",
  },
  "blueprints.syncActionError": {
    message: "{kind} {name} kann nicht angewendet werden",
    description:
      "A sync's planned action the planner refused; {kind}, {name}; the reason follows (w3/m678)",
  },
  "blueprints.syncActionDetachNote": {
    message: "Sie läuft weiter und kann Kosten verursachen.",
    description:
      "Under a 'stop managing' action: the resource is not deleted (w3/m678)",
  },
  "blueprints.syncActionStateUnknown": {
    message: "Status unbekannt",
    description:
      "Accessible name of a planned action's state icon when bex can't say which state it reached (a failed sync's actions, w3/m678)",
  },
  "blueprints.syncApproveFailed": {
    message:
      "Diese Synchronisierung konnte nicht genehmigt werden. Versuche es erneut.",
    description:
      "A sync's page: inline alert when Approve fails without the server's reason, Render's wording (w3/m678)",
  },
  "blueprints.syncNotFound": {
    message:
      "Diese Synchronisierung existiert nicht. Eine geplante Synchronisierung, die nicht innerhalb von 24 Stunden genehmigt wird, wird gelöscht.",
    description:
      "A sync's page for an id that is not one of this Blueprint's syncs, e.g. a plan deleted after 24 hours (w3/m678)",
  },
  "blueprints.syncErrorBannerTitle": {
    message: "Synchronisierungsfehler",
    description:
      "Title of Render's dismissible banner when Manual sync could not plan a sync (w3/m678)",
  },
  "blueprints.syncErrorBannerDismiss": {
    message: "Schließen",
    description:
      "Accessible name of the Sync Error banner's close button (w3/m678)",
  },
  "blueprints.syncErrorsIntro_one": {
    message:
      "Beim Synchronisieren mit deinem Blueprint ist ein Problem aufgetreten:",
    description:
      "A sync's page: the line above its errors, Render's 'There was a problem' / 'There were a few problems' (w3/m678)",
  },
  "blueprints.syncErrorsIntro_other": {
    message:
      "Beim Synchronisieren mit deinem Blueprint sind einige Probleme aufgetreten:",
    description:
      "A sync's page: the line above its errors, Render's 'There was a problem' / 'There were a few problems' (w3/m678)",
  },
};

export default deBlueprints;
