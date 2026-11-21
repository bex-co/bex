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
      "Preview alert body on /blueprints/new when the only problem is a takeover conflict (replaces the raw API message)",
  },
  "blueprints.previewConflict": {
    message: "Bereits verwaltet — beim Deployen wird es übernommen",
    description:
      "Alert title on /blueprints/new when the only preview problem is a resource another Blueprint manages, which an explicit takeover confirmation resolves (w4/m125)",
  },
  "blueprints.previewMatchTitle": {
    message: "Entspricht vorhandenen Ressourcen",
    description:
      "Notice title on /blueprints/new when the Blueprint file names resources that already exist in the workspace (w8/m153, Render parity)",
  },
  "blueprints.previewMatchBody": {
    message:
      "Dieser Blueprint entspricht Ressourcen, die in deinem Workspace bereits existieren. Neue Ressourcen werden mit dem Suffix „-{suffix}“ am Namen erstellt. Die vorhandenen Ressourcen bleiben unverändert.",
    description:
      "Notice body on /blueprints/new for matched resources; {suffix} is the four-character resource name suffix, shown after a hyphen as it appears in a name",
  },
  "blueprints.previewMatchListLabel": {
    message: "Bereits in deinem Workspace:",
    description:
      "Notice on /blueprints/new — label above the list of existing resources the Blueprint file matches",
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
  "blueprints.metaBranch": {
    message: "Branch",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaPath": {
    message: "Manifest-Pfad",
    description:
      "Blueprint detail metadata label — path to render.yaml in repo",
  },
  "blueprints.metaAutoSync": {
    message: "Auto-Synchronisierung",
    description: "Blueprint detail metadata label — auto-sync on push toggle",
  },
  "blueprints.metaCreated": {
    message: "Erstellt",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaUpdated": {
    message: "Zuletzt synchronisiert",
    description: "Blueprint detail metadata label",
  },
  "blueprints.autoSyncOn": {
    message: "An",
    description: "Auto-sync toggle label — enabled",
  },
  "blueprints.autoSyncOff": {
    message: "Aus",
    description: "Auto-sync toggle label — disabled",
  },
  "blueprints.resourcesTitle": {
    message: "Verwaltete Ressourcen",
    description:
      "Blueprint detail section heading listing services/databases managed by this blueprint",
  },
  "blueprints.resourcesEmpty": {
    message:
      "Noch keine Ressourcen — synchronisiere den Blueprint, um deine render.yaml anzuwenden.",
    description: "Blueprint managed-resources empty state (never synced)",
  },
  "blueprints.resourcesEmptyAwaitingApproval": {
    message:
      "Noch keine Ressourcen — genehmige die geplante Synchronisierung, um die Ressourcen in deiner render.yaml zu erstellen.",
    description:
      "Blueprint managed-resources empty state while the Blueprint awaits approval of its planned sync",
  },
  "blueprints.resourcesEmptyAfterError": {
    message:
      "Keine beanspruchten Ressourcen. Die letzte Synchronisierung ist fehlgeschlagen — prüfe den Sync-Verlauf auf den Fehler und versuche es nach der Korrektur des Blueprints erneut.",
    description:
      "Blueprint managed-resources empty state when status is error after a sync",
  },
  "blueprints.syncHistoryTitle": {
    message: "Sync-Verlauf",
    description: "Blueprint detail section heading for the sync run table",
  },
  "blueprints.syncHistoryEmpty": {
    message: "Noch keine Synchronisierungen.",
    description: "Blueprint sync history empty state",
  },
  "blueprints.syncColCommit": {
    message: "Commit",
    description: "Sync history table column — commit SHA",
  },
  "blueprints.syncColState": {
    message: "Zustand",
    description: "Sync history table column — sync run state",
  },
  "blueprints.syncColStarted": {
    message: "Gestartet",
    description: "Sync history table column — sync run start time",
  },
  "blueprints.syncColCompleted": {
    message: "Abgeschlossen",
    description: "Sync history table column — sync run completion time",
  },
  "blueprints.detachPreviewTitle": {
    message: "Ressourcen werden nicht mehr verwaltet",
    description: "Blueprint resource detachment notice.",
  },
  "blueprints.detachSuccessTitle": {
    message: "Synchronisierung abgeschlossen — Ressourcen getrennt",
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
  "blueprints.syncColError": {
    message: "Details",
    description: "Sync history table column — errors and durable sync notes",
  },
  "blueprints.manifestTitle": {
    message: "render.yaml-Manifest",
    description: "Blueprint detail section heading for the stored manifest",
  },
  "blueprints.plannedTitle": {
    message: "Geplante Synchronisierung",
    description:
      "Blueprint detail: heading of the planned-sync card a Blueprint shows while a plan waits for approval",
  },
  "blueprints.plannedBody": {
    message:
      "Dieser Plan wartet auf Genehmigung. Prüfe, was die Genehmigung bei Commit {commit} ändert, und genehmige dann, um den Plan anzuwenden.",
    description:
      "Planned-sync card: explains plan-then-approve; {commit} is a short commit SHA",
  },
  "blueprints.plannedActions": {
    message: "Geplante Änderungen",
    description: "Planned-sync card: accessible label of the action list",
  },
  "blueprints.plannedAction": {
    message: "{operation} {kind} {name}",
    description:
      "Planned-sync card: one plan action, e.g. 'create service api'",
  },
  "blueprints.plannedActionsUnavailable": {
    message:
      "Dieser Plan wurde aufgezeichnet, bevor Pläne ihre Änderungen speicherten. Plane erneut, um sie zu sehen, oder genehmige, um ihn anzuwenden.",
    description:
      "Planned-sync card: a plan stored without its actions (recorded before migration 0187)",
  },
  "blueprints.plannedManualEnvVars": {
    message:
      "Diese Werte stehen nicht in der Blueprint-Datei (sync: false). Gib sie zur Genehmigung ein.",
    description:
      "Planned-sync card: intro to the required manual env var fields",
  },
  "blueprints.plannedManualEnvVarLabel": {
    message: "{service}: {key}",
    description: "Planned-sync card: label of one manual env var field",
  },
  "blueprints.plannedApprove": {
    message: "Genehmigen",
    description: "Planned-sync card: button that applies the planned sync",
  },
  "blueprints.plannedReplan": {
    message: "Erneut planen",
    description:
      "Planned-sync card: button that plans a new sync at the branch head",
  },
  "blueprints.plannedNone": {
    message:
      "Es wartet kein Plan auf Genehmigung. Plane erneut, um den Branch-Stand zu lesen.",
    description:
      "Planned-sync card: shown when a created Blueprint has no approvable plan",
  },
  "blueprints.plannedApproved": {
    message: "Geplante Synchronisierung genehmigt",
    description: "Toast after approving a planned sync",
  },
  "blueprints.plannedApproveFailed": {
    message: "Die geplante Synchronisierung konnte nicht genehmigt werden",
    description: "Toast when approving fails",
  },
  "blueprints.plannedChanged": {
    message:
      "Jemand hat diesen Blueprint erneut geplant. Prüfe den neuesten Plan und genehmige ihn dann.",
    description:
      "Toast when the plan on screen was replaced before it was approved (BLUEPRINT_PLAN_CHANGED)",
  },
  "blueprints.plannedReplanFailed": {
    message: "Eine neue Synchronisierung konnte nicht geplant werden",
    description: "Toast when re-planning fails",
  },
  "blueprints.syncButton": {
    message: "Synchronisieren",
    description: "Button label that triggers an idempotent blueprint re-apply",
  },
  "blueprints.syncConfirmTitle": {
    message: "Blueprint synchronisieren?",
    description: "Sync confirm dialog title",
  },
  "blueprints.syncConfirmBody": {
    message:
      "Dadurch wird render.yaml exakt im Stand des unten gezeigten Commits angewendet — nicht ein neuerer Push auf den Branch. Ressourcen, die bereits übereinstimmen, bleiben unverändert.",
    description: "Sync confirm dialog description",
  },
  "blueprints.syncConfirmAction": {
    message: "Synchronisieren",
    description: "Sync confirm dialog primary action button label",
  },
  "blueprints.syncCancel": {
    message: "Abbrechen",
    description: "Sync confirm dialog cancel button label",
  },
  "blueprints.syncPreviewLoading": {
    message: "Sync-Plan wird aus dem Repository berechnet…",
    description: "Pre-sync dialog — loading line while blueprintPreview runs",
  },
  "blueprints.syncPreviewInvalid": {
    message:
      "Die aktuell auf dem Branch liegende Blueprint-Datei hat Probleme — eine Synchronisierung jetzt kann fehlschlagen oder auf das gespeicherte Manifest zurückfallen:",
    description:
      "Pre-sync dialog — heading above validation errors from the fetched manifest",
  },
  "blueprints.syncPreviewUnavailable": {
    message:
      "Der Sync-Plan konnte nicht berechnet werden (Vorschau nicht verfügbar). Aktualisiere, nachdem die Vorschau erfolgreich war — die Synchronisierung erfordert einen geprüften Commit.",
    description:
      "Pre-sync dialog — preview transport failure; sync is disabled until a valid preview pins a commit (w8/m41)",
  },
  "blueprints.syncPreviewCommit": {
    message: "Geprüfter Commit {sha}",
    description: "Pre-sync dialog — short SHA of the previewed Git revision",
  },
  "blueprints.syncRefreshPreview": {
    message: "Vorschau aktualisieren",
    description:
      "Pre-sync dialog — refetch Git + re-validate after BLUEPRINT_SOURCE_CHANGED or an invalid preview",
  },
  "blueprints.syncSourceChanged": {
    message:
      "Die Blueprint-Quelle hat sich seit deiner Prüfung geändert — aktualisiere die Vorschau und bestätige erneut",
    description:
      "Toast when syncBlueprint refuses with BLUEPRINT_SOURCE_CHANGED (path/repo drift or stale pin)",
  },
  "blueprints.syncSuccess": {
    message: "Blueprint synchronisiert",
    description: "Toast shown after a successful syncBlueprint call",
  },
  "blueprints.syncError": {
    message: "Synchronisierung fehlgeschlagen",
    description: "Toast shown when syncBlueprint returns an error",
  },
  "blueprints.syncBusy": {
    message:
      "Diese Synchronisierung hat die Ausführung nicht mehr inne (ausgelastet, unterbrochen oder abgelöst) — starte eine neue Synchronisierung; Teilarbeit wird nie wiederholt",
    description:
      "Toast shown when syncBlueprint refuses with BLUEPRINT_SYNC_BUSY (admission race, mid-apply fence, or disconnect-busy)",
  },
  "blueprints.editField": {
    message: "{field} bearbeiten",
    description: "Aria label for the inline-edit pencil button",
  },
  "blueprints.saveField": {
    message: "{field} speichern",
    description: "Aria label for the inline-edit save button",
  },
  "blueprints.cancelEdit": {
    message: "Bearbeitung abbrechen",
    description: "Aria label for the inline-edit cancel button",
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
  "blueprints.updateError": {
    message: "Aktualisierung fehlgeschlagen",
    description: "Toast shown when updateBlueprint returns an error",
  },
  "blueprints.disconnectButton": {
    message: "Trennen",
    description:
      "Button that disconnects a blueprint from its Git repo (stops auto-sync, keeps resources)",
  },
  "blueprints.disconnectTitle": {
    message: "Blueprint trennen?",
    description: "Disconnect confirm dialog title",
  },
  "blueprints.disconnectBody": {
    message:
      "Dadurch wird die Auto-Synchronisierung bei Push beendet und der Blueprint aus deiner Liste entfernt. Bereits deployte Ressourcen bleiben unberührt.",
    description: "Disconnect confirm dialog description",
  },
  "blueprints.disconnectAction": {
    message: "Trennen",
    description: "Disconnect confirm dialog primary action button label",
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
};

export default deBlueprints;
