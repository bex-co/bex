import type { TranslationEntry } from "@/i18n";

const deDeploys: Record<string, TranslationEntry> = {
  "deploys.statusCreated": {
    message: "Erstellt",
    description: "Deploy status: created",
  },
  "deploys.statusQueued": {
    message: "In Warteschlange",
    description: "Deploy status: queued",
  },
  "deploys.statusBuildInProgress": {
    message: "Build läuft",
    description: "Deploy status: build_in_progress",
  },
  "deploys.statusBuildFailed": {
    message: "Build fehlgeschlagen",
    description: "Deploy status: build_failed",
  },
  "deploys.statusPreDeployInProgress": {
    message: "Pre-Deploy läuft",
    description: "Deploy status: pre_deploy_in_progress",
  },
  "deploys.statusPreDeployFailed": {
    message: "Pre-Deploy fehlgeschlagen",
    description: "Deploy status: pre_deploy_failed",
  },
  "deploys.statusUpdateInProgress": {
    message: "In Bearbeitung",
    description: "Deploy status: update_in_progress",
  },
  "deploys.statusUpdateFailed": {
    message: "Fehlgeschlagen",
    description: "Deploy status: update_failed",
  },
  "deploys.statusLive": {
    message: "Live",
    description: "Deploy status: live",
  },
  "deploys.statusCanceled": {
    message: "Abgebrochen",
    description: "Deploy status: canceled",
  },
  "deploys.statusDeactivated": {
    message: "Deaktiviert",
    description: "Deploy status: deactivated",
  },
  "deploys.statusUnknown": {
    message: "Unbekannt",
    description: "Deploy status: unrecognized backend value",
  },
  "deploys.statusLabel": {
    message: "Deployment-Status",
    description:
      "Deploy detail header: explicit label distinguishing deploy status from service phase",
  },
  "deploys.imageDigestCopy": {
    message: "Image-Digest kopieren",
    description:
      "Deploy header: accessible label of the button copying the image digest the deploy ran",
  },
  "deploys.imageDigestCopied": {
    message: "Image-Digest kopiert",
    description: "Deploy header: toast after copying the image digest",
  },
  "deploys.imageDigestCopyError": {
    message: "Kopieren in die Zwischenablage fehlgeschlagen",
    description: "Deploy header: toast when copying the image digest fails",
  },
  "deploys.created": {
    message: "Erstellt",
    description: "Deploy header: created-at label",
  },
  "deploys.updated": {
    message: "Aktualisiert",
    description: "Label for the deploy's latest stored transition time",
  },
  "deploys.started": {
    message: "Gestartet",
    description: "Deploy header: started-at label",
  },
  "deploys.finished": {
    message: "Beendet",
    description: "Deploy header: finished-at label",
  },
  "deploys.duration": {
    message: "Dauer",
    description: "Deploy header: elapsed build/deploy time label",
  },
  "deploys.durationValue": {
    message: "Dauer {duration}",
    description: "Deploy row: elapsed build/deploy time",
  },
  "deploys.durationInProgress": {
    message: "Dauer läuft",
    description: "Deploy row: elapsed time is still running",
  },
  "deploys.durationActive": {
    message: "Läuft",
    description:
      "Deploys tab Duration column: bare running-elapsed marker (the column header supplies the label)",
  },
  "deploys.deployedAt": {
    message: "Deployt {timestamp}",
    description:
      'Deploy row: how long ago a live/deactivated deploy went live — {timestamp} is elapsed text like "2 hours ago" (its finish time; createdAt fallback)',
  },
  "deploys.createdAt": {
    message: "Erstellt {timestamp}",
    description:
      'Deploy row: how long ago a deploy that hasn\'t finished (created/queued/in-progress) was created — {timestamp} is elapsed text like "2 minutes ago"',
  },
  "deploys.canceledAt": {
    message: "Abgebrochen {timestamp}",
    description:
      'Deploy row: how long ago a canceled deploy was canceled — {timestamp} is elapsed text like "3 days ago" (its finish time; createdAt fallback)',
  },
  "deploys.failedAt": {
    message: "Fehlgeschlagen {timestamp}",
    description:
      'Deploy row: how long ago a build/pre-deploy/update failure landed — {timestamp} is elapsed text like "3 days ago" (its finish time; createdAt fallback)',
  },
  "deploys.notYet": {
    message: "—",
    description:
      "Deploy header: placeholder for a timestamp that hasn't happened yet",
  },
  "deploys.triggerCreate": {
    message: "Erstes Deployment",
    description: "Deploy header: trigger=create label",
  },
  "deploys.triggerApi": {
    message: "API-Deployment",
    description: "Deploy header: trigger=api label (an API-key deploy, w4/413)",
  },
  "deploys.triggerManual": {
    message: "Manuelles Deployment",
    description:
      "Deploy header: trigger=manual label (a dashboard click, w4/413)",
  },
  "deploys.triggerRollback": {
    message: "Rollback auf {deployId}",
    description:
      "Deploy header: trigger label for a rollback deploy, naming the restored deploy (id must stay literal — never wrap in CSS capitalize)",
  },
  "deploys.triggerDeployHook": {
    message: "Deploy Hook",
    description: "Deploy row: trigger=deploy_hook label (secret-URL trigger)",
  },
  "deploys.triggerBlueprint": {
    message: "Blueprint-Sync",
    description: "Deploy row: trigger=blueprint label (Blueprint sync deploy)",
  },
  "deploys.triggerNewCommit": {
    message: "Neuer Commit",
    description: "Deploy row: trigger=new_commit label (git push redeploy)",
  },
  "deploys.triggerConfigChange": {
    message: "Konfigurationsänderung",
    description:
      "Deploy row: trigger=config_change label (an env var, secret file, or env-group edit that deployed)",
  },
  "deploys.triggerSettingsChange": {
    message: "Änderung der Einstellungen",
    description:
      "Deploy row: trigger=settings_change label (a Settings save that deployed, or a plan change; w8/m135)",
  },
  "deploys.triggerRestart": {
    message: "Neustart",
    description:
      "Deploy row: trigger=restart label (a Restart roll of the live release, w4/379)",
  },
  "deploys.triggerDeployedByRender": {
    message: "Plattform-Rebuild",
    description:
      "Deploy row: trigger=deployed_by_render label (the platform rebuilt the live commit, e.g. to ship a runtime security patch)",
  },
  "deploys.logSearchPlaceholder": {
    message: "Logs durchsuchen…",
    description: "Deploy detail page: log search input placeholder",
  },
  "deploys.logTypeFilter": {
    message: "Log-Typ",
    description:
      "Deploy log viewer: aria-label of the log-type filter dropdown (Render's All/Application/Build selector)",
  },
  "deploys.logTypeAll": {
    message: "Alle Logs",
    description: "Deploy log viewer type filter: show every log type",
  },
  "deploys.logTypeApp": {
    message: "Anwendungs-Logs",
    description:
      "Deploy log viewer type filter: app + pre-deploy lines, matching Render's Application bucket",
  },
  "deploys.logTypeBuild": {
    message: "Build-Logs",
    description: "Deploy log viewer type filter: build lines only",
  },
  "deploys.buildLogsStoreUnavailable": {
    message: "Historische Build-Logs sind nicht verfügbar",
    description:
      "Deploy detail page: title shown when the durable log store isn't wired",
  },
  "deploys.buildLogsStoreUnavailableBody": {
    message:
      "Dieses Dashboard ist nicht mit einem dauerhaften Log-Speicher verbunden. Anwendungs-Logs können für dieses Deployment trotzdem erscheinen.",
    description:
      "Deploy detail page: accurate explanation of a storeless build-log response",
  },
  "deploys.detailRefreshError": {
    message:
      "Dieses Deployment konnte nicht aktualisiert werden. Die zuletzt geladenen Details werden angezeigt.",
    description:
      "Deploy detail page: a background refresh of an already loaded deploy failed; the page keeps the last good details and offers a retry",
  },
  "deploys.logPartiallyLoaded": {
    message:
      "Nur ein Teil dieses Zeitraums ist geladen. Lade ältere Einträge, um frühere Zeilen zu durchsuchen.",
    description:
      "Deploy log viewer banner while older history remains unloaded. The panel's search and type/instance filters run over loaded lines only, so it must not claim a number of matches",
  },
  "deploys.logStepEndMissing": {
    message:
      "Dieses Deployment wurde abgebrochen, bevor der Pre-Deploy-Schritt beendet war, und die Abschlusszeit des Schritts wurde nicht aufgezeichnet, daher liegt seine spätere Ausgabe möglicherweise außerhalb dieses Fensters. Wähle in den Log-Optionen einen größeren Zeitraum, um sie zu sehen.",
    description:
      "Deploy log viewer: a canceled deploy's continued pre-deploy step has no recorded end, so the deploy window may omit its later output",
  },
  "deploys.logStepEndUnavailable": {
    message:
      "Es konnte nicht geprüft werden, wann der Pre-Deploy-Schritt dieses Deployments beendet wurde, daher liegt die spätere Ausgabe des Schritts möglicherweise außerhalb dieses Fensters. Versuche es erneut oder wähle in den Log-Optionen einen größeren Zeitraum.",
    description:
      "Deploy log viewer: reading a canceled deploy's continued pre-deploy step end failed; offers a retry",
  },
  "deploys.timelineTitle": {
    message: "Status-Zeitleiste",
    description: "Deploy detail page: status-timeline card title",
  },
  "deploys.timelineCreated": {
    message: "Deployment erstellt",
    description: "Deploy timeline: deploy row was created",
  },
  "deploys.timelineStarted": {
    message: "Deployment gestartet",
    description: "Deploy timeline: backend-provided startedAt timestamp",
  },
  "deploys.timelineInProgress": {
    message: "Deployment läuft",
    description: "Deploy timeline: current non-terminal deploy status",
  },
  "deploys.timelineLive": {
    message: "Deployment live",
    description: "Deploy timeline: successful terminal state",
  },
  "deploys.timelineFailed": {
    message: "Deployment fehlgeschlagen",
    description: "Deploy timeline: failed terminal state",
  },
  "deploys.timelineCanceled": {
    message: "Deployment abgebrochen",
    description: "Deploy timeline: canceled terminal state",
  },
  "deploys.timelineDeactivated": {
    message: "Deployment deaktiviert",
    description: "Deploy timeline: deactivated terminal state",
  },
  "deploys.timelineEventsUnavailable": {
    message:
      "Service-Ereignisse sind nicht verfügbar; es werden nur die Zeitstempel des Deployments angezeigt.",
    description:
      "Deploy timeline: graceful fallback when service-events query fails",
  },
  "deploys.autoDeployOffTitle": {
    message: "Auto-Deploy ist aus",
    description:
      "Deploys page banner title when the service's auto-deploy is off (Render, w3/064)",
  },
  "deploys.autoDeployOffBody": {
    message:
      "Pushes auf {branch} deployen diesen Service nicht. Deploye ihn manuell oder schalte Auto-Deploy wieder ein.",
    description: "Deploys page banner body; {branch} is the tracked git branch",
  },
  "deploys.autoDeployOffBodyNoBranch": {
    message:
      "Pushes deployen diesen Service nicht. Deploye ihn manuell oder schalte Auto-Deploy wieder ein.",
    description: "Deploys page banner body when the branch is unknown",
  },
  "deploys.autoDeployOffSettings": {
    message: "Auto-Deploy-Einstellungen",
    description: "Deploys page banner link to the service Settings page",
  },
  "deploys.deploysManualTitle": {
    message: "Deployments sind manuell",
    description:
      "Deploys page banner title for a repo whose pushes can't reach bex (pushDeliveryMethod unavailable)",
  },
  "deploys.deploysManualBody": {
    message:
      "bex empfängt die Pushes dieses Repositorys nicht, daher deployen sie diesen Service nicht. Deploye ihn manuell oder mit dem Deploy Hook, oder verbinde das Repository über die GitHub-App.",
    description:
      "Deploys page banner body for a repo whose pushes can't reach bex: no Auto-Deploy setting can turn deploys on push on (Render: a public Git URL must be deployed manually)",
  },
  "deploys.columnDeploy": {
    message: "Deployment",
    description:
      "Deploys tab table: first column header (status, id, commit, timestamp)",
  },
  "deploys.columnTrigger": {
    message: "Auslöser",
    description: "Deploys tab table: column header for what triggered a deploy",
  },
  "deploys.columnDuration": {
    message: "Dauer",
    description:
      "Deploys tab table: column header for elapsed build/deploy time",
  },
  "deploys.columnActions": {
    message: "Aktionen",
    description:
      "Deploys tab table: accessible-only header over the Cancel/Rollback action column",
  },
  "deploys.listEmpty": {
    message: "Noch keine Deployments.",
    description: "Deploys tab: empty state with no status filter",
  },
  "deploys.listEmptyFiltered": {
    message: "Keine Deployments entsprechen dem ausgewählten Status.",
    description: "Deploys tab: empty state while a status filter is active",
  },
  "deploys.listStatusFilterLabel": {
    message: "Nach Status filtern",
    description: "Deploys tab: aria-label of the status filter dropdown",
  },
  "deploys.listStatusAll": {
    message: "Alle Status",
    description: "Deploys tab: status filter option matching every deploy",
  },
  "deploys.listSearchPlaceholder": {
    message: "Suchen…",
    description:
      "Deploys tab: placeholder for client-side deploy-history search",
  },
  "deploys.listSearchLabel": {
    message: "Geladene Deployments und Commits durchsuchen",
    description: "Deploys tab: accessible label for deploy-history search",
  },
  "deploys.listClearSearch": {
    message: "Suche löschen",
    description:
      "Deploys tab: button under the no-match message that empties the search box",
  },
  "deploys.listMatchCount": {
    message: "Treffer: {matched} von {total} geladenen",
    description:
      "Deploys tab: count shown while a search is typed; matched = loaded deploys that match, total = loaded deploys",
  },
  "deploys.listLoadErrorTitle": {
    message: "Deploys konnten nicht geladen werden",
    description:
      "Deploys tab: heading of the error state when the deploy history could not be read; a Try again button follows",
  },
  "deploys.listLoadErrorBody": {
    message:
      "Deine Deploys sind nicht betroffen. Prüfe deine Verbindung und versuche es erneut.",
    description:
      "Deploys tab: body of the error state when the deploy history could not be read and the server gave no message; a Try again button follows",
  },
  "deploys.listEmptySearch": {
    message: "Keine geladenen Deployments entsprechen deiner Suche.",
    description:
      "Deploys tab: empty state when client-side search has no match",
  },
  "deploys.listCount_one": {
    message: "{count} Deployment",
    description: "Deploys tab: complete deploy count (singular)",
  },
  "deploys.listCount_other": {
    message: "{count} Deployments",
    description: "Deploys tab: complete deploy count (plural)",
  },
  "deploys.listCountLoaded_one": {
    message: "{count} Deployment geladen",
    description:
      "Deploys tab: loaded count when more server pages remain (singular)",
  },
  "deploys.listCountLoaded_other": {
    message: "{count} Deployments geladen",
    description:
      "Deploys tab: loaded count when more server pages remain (plural)",
  },
  "deploys.listLoadMore": {
    message: "Mehr laden",
    description: "Deploys tab: button fetching the next page of deploy history",
  },
  "deploys.listLoadMoreError": {
    message: "Ältere Deployments konnten nicht geladen werden.",
    description:
      "Deploys tab: toast when reading an older page of deploy history fails",
  },
  "deploys.logOptions": {
    message: "Log-Optionen",
    description:
      "Deploy log viewer: aria-label/tooltip of the options menu button",
  },
  "deploys.logRangeLabel": {
    message: "Zeitraum",
    description:
      "Deploy log viewer options menu: heading over the time-range choices",
  },
  "deploys.logRangeDeploy": {
    message: "Deployment-Zeitraum",
    description:
      "Deploy log viewer: the default time range — the deploy's own createdAt..finishedAt window",
  },
  "deploys.logRangeLast15m": {
    message: "Letzte 15 Minuten",
    description: "Deploy log viewer: relative time-range option (?r=15m)",
  },
  "deploys.logRangeLast1h": {
    message: "Letzte Stunde",
    description: "Deploy log viewer: relative time-range option (?r=1h)",
  },
  "deploys.logRangeLast6h": {
    message: "Letzte 6 Stunden",
    description: "Deploy log viewer: relative time-range option (?r=6h)",
  },
  "deploys.logRangeLast24h": {
    message: "Letzte 24 Stunden",
    description: "Deploy log viewer: relative time-range option (?r=24h)",
  },
  "deploys.logRangeLast7d": {
    message: "Letzte 7 Tage",
    description: "Deploy log viewer: relative time-range option (?r=7d)",
  },
  "deploys.logWrap": {
    message: "Zeilen umbrechen",
    description:
      "Deploy log viewer options menu: toggle wrapping long log lines vs horizontal scroll",
  },
  "deploys.logTimestamps": {
    message: "Zeitstempel anzeigen",
    description:
      "Deploy log viewer options menu: toggle the per-line timestamp column",
  },
  "deploys.logMaximize": {
    message: "Maximieren",
    description:
      "Deploy log viewer: button expanding the viewer to fill the screen",
  },
  "deploys.logMinimize": {
    message: "Vollbild beenden",
    description:
      "Deploy log viewer: button restoring the maximized viewer to its inline size",
  },
  "deploys.reason.http_port_waiting": {
    message:
      "Warten auf den Start des Containers, bevor sein HTTP-Port erkannt wird",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_waiting, w4/566)",
  },
  "deploys.reason.http_port_detecting": {
    message: "HTTP-Port des Containers wird erkannt",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_detecting, w4/566)",
  },
  "deploys.reason.http_port_limit": {
    message:
      "Der erkannte HTTP-Port überschreitet das Limit von 75 Ports des Services; setze PORT explizit",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_limit, w4/566)",
  },
  "deploys.reason.http_port_detected": {
    message: "HTTP auf Port {port} erkannt; wird darüber bereitgestellt",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_detected, w4/566); params are the sentence's values",
  },
  "deploys.reason.http_port_not_answering": {
    message:
      "Port {port} hat eine Verbindung angenommen, aber noch kein HTTP beantwortet; der Scan wird fortgesetzt, während der Container startet",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_not_answering, w4/566); params are the sentence's values",
  },
  "deploys.reason.http_port_none_yet": {
    message:
      "Noch kein Port antwortet auf HTTP; der Scan wird fortgesetzt, während der Container startet",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_none_yet, w4/566)",
  },
  "deploys.reason.http_port_fallback": {
    message:
      "Kein Port hat auf HTTP geantwortet; Fallback-Port {port} wird verwendet. Setze PORT auf den HTTP-Listener, falls er an einen anderen Port bindet",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_fallback, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_not_found": {
    message:
      "das Image {image} wurde in seiner Registry nicht gefunden: Das Repository oder der Tag existiert nicht. Prüfe die Image-URL und den Tag oder deploye einen Digest, der existiert",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_not_found, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_credential_refused": {
    message:
      "{host} hat die Registry-Zugangsdaten dieses Services für das Image {image} abgelehnt (HTTP {status}). Prüfe Benutzernamen und Token der Zugangsdaten oder entferne die Zugangsdaten von diesem Service, um ein öffentliches Image anonym zu pullen",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_credential_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_access_refused": {
    message:
      "die Registry hat den Zugriff auf das Image {image} verweigert. Wenn das Image privat ist, füge diesem Service Registry-Zugangsdaten für {host} hinzu",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_access_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_read_refused": {
    message:
      "die Registry hat das Lesen des Images {image} verweigert (HTTP {status}). Prüfe die Image-URL und den Tag",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_read_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_registry_unreachable": {
    message:
      "die Registry {host} hat {minutes} Minuten lang nicht auf das Lesen des Images {image} geantwortet. Prüfe, ob die Registry erreichbar ist, und deploye dann erneut",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_registry_unreachable, w4/692); params are the sentence's values",
  },
  "deploys.reason.build_timed_out": {
    message:
      "der Build wurde nicht innerhalb des Build-Zeitfensters beendet; prüfe die Build-Logs",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason build_timed_out, w4/566)",
  },
  "deploys.reason.pre_deploy_timed_out": {
    message:
      "der Pre-Deploy-Befehl wurde nicht innerhalb seines Zeitfensters beendet; prüfe die Pre-Deploy-Logs",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_timed_out, w4/566)",
  },
  "deploys.reason.health_timed_out": {
    message:
      "das Deployment wurde nicht innerhalb des Health-Gate-Zeitfensters gesund; prüfe die Service-Logs",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason health_timed_out, w4/566)",
  },
  "deploys.reason.startup_out_of_memory": {
    message:
      "der Arbeitsspeicher der Instanz ging beim Start aus (Limit {memory} für ihren Compute-Plan; Exit-Code {code}) — reduziere den Speicherverbrauch oder wähle einen größeren Compute-Plan",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_out_of_memory, w4/630); params are the sentence's values: memory is the compute plan's memory limit (e.g. 512Mi), code the exit code",
  },
  "deploys.reason.probe_http_waiting": {
    message:
      "der Container läuft, aber sein {kind}-Health-Check war noch nicht erfolgreich, daher wartet der Rollout: GET {path} auf Port {port}. Prüfe, ob der Pfad auf dem eigenen Port des Services einen 2xx- oder 3xx-Status zurückgibt.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_waiting, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_waiting_restarted": {
    message:
      "der Container läuft, aber sein {kind}-Health-Check war noch nicht erfolgreich, daher wartet der Rollout: GET {path} auf Port {port}. Der Liveness-Check hat den Container bereits {restarts}-mal neu gestartet, der Check schlägt also fehl und ist nicht nur langsam. Prüfe, ob der Pfad auf dem eigenen Port des Services einen 2xx- oder 3xx-Status zurückgibt.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_waiting_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_failed": {
    message:
      "der Container läuft, aber sein {kind}-Health-Check war innerhalb des Rollout-Zeitfensters nie erfolgreich, daher ist das Deployment fehlgeschlagen: GET {path} auf Port {port}. Prüfe, ob der Pfad auf dem eigenen Port des Services einen 2xx- oder 3xx-Status zurückgibt.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_failed, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_failed_restarted": {
    message:
      "der Container läuft, aber sein {kind}-Health-Check war innerhalb des Rollout-Zeitfensters nie erfolgreich, daher ist das Deployment fehlgeschlagen: GET {path} auf Port {port}. Der Liveness-Check hat den Container bereits {restarts}-mal neu gestartet, der Check schlägt also fehl und ist nicht nur langsam. Prüfe, ob der Pfad auf dem eigenen Port des Services einen 2xx- oder 3xx-Status zurückgibt.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_failed_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_waiting": {
    message:
      "der Container läuft, aber sein {kind}-Health-Check war noch nicht erfolgreich, daher wartet der Rollout: ein TCP-Connect auf Port {port}. Es ist kein Health-Check-Pfad gesetzt, daher ist der Check ein TCP-Connect, der nur fragt, ob der Prozess auf diesem Port lauscht.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_waiting, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_waiting_restarted": {
    message:
      "der Container läuft, aber sein {kind}-Health-Check war noch nicht erfolgreich, daher wartet der Rollout: ein TCP-Connect auf Port {port}. Der Liveness-Check hat den Container bereits {restarts}-mal neu gestartet, der Check schlägt also fehl und ist nicht nur langsam. Es ist kein Health-Check-Pfad gesetzt, daher ist der Check ein TCP-Connect, der nur fragt, ob der Prozess auf diesem Port lauscht.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_waiting_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_failed": {
    message:
      "der Container läuft, aber sein {kind}-Health-Check war innerhalb des Rollout-Zeitfensters nie erfolgreich, daher ist das Deployment fehlgeschlagen: ein TCP-Connect auf Port {port}. Es ist kein Health-Check-Pfad gesetzt, daher ist der Check ein TCP-Connect, der nur fragt, ob der Prozess auf diesem Port lauscht.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_failed, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_failed_restarted": {
    message:
      "der Container läuft, aber sein {kind}-Health-Check war innerhalb des Rollout-Zeitfensters nie erfolgreich, daher ist das Deployment fehlgeschlagen: ein TCP-Connect auf Port {port}. Der Liveness-Check hat den Container bereits {restarts}-mal neu gestartet, der Check schlägt also fehl und ist nicht nur langsam. Es ist kein Health-Check-Pfad gesetzt, daher ist der Check ein TCP-Connect, der nur fragt, ob der Prozess auf diesem Port lauscht.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_failed_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.rollout_deadline_exceeded": {
    message: "der Rollout wurde nicht innerhalb der Fortschrittsfrist gesund",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason rollout_deadline_exceeded, w4/591)",
  },
  "deploys.reason.pre_deploy_window_exceeded": {
    message:
      "der Pre-Deploy-Befehl wurde nicht innerhalb seines {minutes}-Minuten-Fensters beendet; prüfe die Pre-Deploy-Logs",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_window_exceeded, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_out_of_memory": {
    message:
      "dem Pre-Deploy-Befehl ist der Arbeitsspeicher ausgegangen (Exit-Code {code}); prüfe die Pre-Deploy-Logs",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_out_of_memory, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_exited": {
    message:
      "der Pre-Deploy-Befehl wurde mit Code {code} beendet; prüfe die Pre-Deploy-Logs",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_exited, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_failed": {
    message:
      "der Pre-Deploy-Befehl ist fehlgeschlagen; prüfe die Pre-Deploy-Logs",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_failed, w4/591)",
  },
  "deploys.reason.pre_deploy_could_not_launch": {
    message:
      "der Pre-Deploy-Befehl konnte nicht gestartet werden: Die Container-Runtime konnte sein Programm nicht starten; das Programm wurde nie ausgeführt, daher gibt es keine Pre-Deploy-Logs",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_could_not_launch, w4/591)",
  },
  "deploys.reason.build_never_started": {
    message: "der Build wurde nie gestartet: {cause}",
    description:
      "A deploy failure reason by its code (bex-api DeployReason build_never_started, w4/591); {cause} is the operator's wait, whose sentences are worded as parts (w5/m356)",
  },
  "deploys.reason.build_abandoned": {
    message:
      "der Build wurde nicht innerhalb des Build-Zeitfensters beendet, und der App-Datensatz dieses Services konnte nicht gelesen werden, um den Grund zu erklären",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason build_abandoned, w4/591)",
  },
  "deploys.reason.deploy_abandoned": {
    message:
      "das Deployment wurde nicht innerhalb seines Zeitfensters beendet, und der App-Datensatz dieses Services konnte nicht gelesen werden, um den Grund zu erklären",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason deploy_abandoned, w4/591)",
  },
  "deploys.reason.platform_error": {
    message:
      "bex konnte diesen Release wegen eines Plattformfehlers nicht anwenden, nicht wegen deines Codes oder deiner Einstellungen. Wende dich mit der Referenz {ref} an den Support",
    description:
      "A deploy failure reason by its code (bex-api DeployReason platform_error, w9/090): the platform refused one of bex's own changes; {ref} is the support reference, shown as-is",
  },
  "deploys.reason.canceled_suspended": {
    message: "Abgebrochen: Der Service wurde pausiert",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_suspended, w4/591)",
  },
  "deploys.reason.superseded_by_newer_release": {
    message: "Durch einen neueren Release ersetzt",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason superseded_by_newer_release, w4/591)",
  },
  "deploys.reason.superseded_by": {
    message: "Ersetzt durch {deploy}",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason superseded_by, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_by_restart": {
    message:
      "Abgebrochen durch den Neustart {deploy}, der den laufenden Release beibehalten hat: Was dieses Deployment anwenden sollte, wurde nicht deployt",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_by_restart, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_along_with": {
    message: "Zusammen mit {deploy} abgebrochen",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_along_with, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_along_with_newer": {
    message: "Zusammen mit einem neueren Deployment abgebrochen",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_along_with_newer, w4/591)",
  },
  "deploys.reason.restart_service_gone": {
    message: "der Service existiert nicht mehr",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason restart_service_gone, w4/591)",
  },
  "deploys.reason.restart_wait_timed_out": {
    message:
      "das Deployment, auf das dieser Neustart gewartet hat, wurde nicht innerhalb von {wait} beendet",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason restart_wait_timed_out, w4/591); params are the sentence's values",
  },
  "deploys.reason.deploy_wait_timed_out": {
    message:
      "das Deployment, auf das dieses wartete, wurde nicht innerhalb von {wait} abgeschlossen",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason deploy_wait_timed_out, w4/652): a Rollback or another deploy that waited for the deploy in flight; params are the sentence's values",
  },
  "deploys.reason.probe_other_waiting": {
    message:
      "der Container läuft, aber sein {kind}-Health-Check war noch nicht erfolgreich, daher wartet der Rollout: der konfigurierte Health-Check. Prüfe, ob der Pfad auf dem eigenen Port des Services einen 2xx- oder 3xx-Status zurückgibt.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_waiting, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_waiting_restarted": {
    message:
      "der Container läuft, aber sein {kind}-Health-Check war noch nicht erfolgreich, daher wartet der Rollout: der konfigurierte Health-Check. Der Liveness-Check hat den Container bereits {restarts}-mal neu gestartet, der Check schlägt also fehl und ist nicht nur langsam. Prüfe, ob der Pfad auf dem eigenen Port des Services einen 2xx- oder 3xx-Status zurückgibt.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_waiting_restarted, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_failed": {
    message:
      "der Container läuft, aber sein {kind}-Health-Check war innerhalb des Rollout-Zeitfensters nie erfolgreich, daher ist das Deployment fehlgeschlagen: der konfigurierte Health-Check. Prüfe, ob der Pfad auf dem eigenen Port des Services einen 2xx- oder 3xx-Status zurückgibt.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_failed, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_failed_restarted": {
    message:
      "der Container läuft, aber sein {kind}-Health-Check war innerhalb des Rollout-Zeitfensters nie erfolgreich, daher ist das Deployment fehlgeschlagen: der konfigurierte Health-Check. Der Liveness-Check hat den Container bereits {restarts}-mal neu gestartet, der Check schlägt also fehl und ist nicht nur langsam. Prüfe, ob der Pfad auf dem eigenen Port des Services einen 2xx- oder 3xx-Status zurückgibt.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_failed_restarted, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.hint_image_default_port": {
    message:
      " Auf Port {port} hat nichts eine TCP-Verbindung angenommen; das ist der Standard, weil das Image keine TCP-Ports deklariert. Der Server muss Host 0.0.0.0 auf dem Port in PORT ({port}) binden.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_image_default_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_image_declared_ports": {
    message:
      " Auf Port {port} hat nichts eine TCP-Verbindung angenommen; das ist der primäre Port, der aus den deklarierten TCP-Ports des Images ({ports}) gewählt wurde. Binde den Server an 0.0.0.0 auf einem dieser Ports oder setze den Port des Services auf den, auf dem er lauscht.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_image_declared_ports, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_env_port_only": {
    message:
      " Auf Port {port} hat nichts eine TCP-Verbindung angenommen; er wurde durch die Umgebungsvariable PORT des Services gesetzt, und das Image deklariert keinen anderen TCP-Port. Binde den Server an 0.0.0.0 auf Port {port} oder ändere den Port des Services auf den, auf dem er lauscht.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_env_port_only, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_configured_port_only": {
    message:
      " Auf Port {port} hat nichts eine TCP-Verbindung angenommen; er wurde durch den konfigurierten Port des Services gesetzt, und das Image deklariert keinen anderen TCP-Port. Binde den Server an 0.0.0.0 auf Port {port} oder ändere den Port des Services auf den, auf dem er lauscht.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_configured_port_only, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_env_port_declared": {
    message:
      " Auf Port {port} hat nichts eine TCP-Verbindung angenommen; er wurde durch die Umgebungsvariable PORT des Services gesetzt, und das Image deklariert TCP-Ports ({ports}). Ändere die Umgebungsvariable PORT auf den Port, auf dem er lauscht, oder entferne sie, um den Port des Images zu verwenden.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_env_port_declared, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_configured_port_declared": {
    message:
      " Auf Port {port} hat nichts eine TCP-Verbindung angenommen; er wurde durch den konfigurierten Port des Services gesetzt, und das Image deklariert TCP-Ports ({ports}). Ändere den Port des Services in den Einstellungen auf den, auf dem er lauscht, oder entferne ihn, um den Port des Images zu verwenden.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_configured_port_declared, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_background_worker": {
    message:
      " Wenn der Service auf keinem Port Traffic empfangen muss, erstelle stattdessen einen Background Worker.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_background_worker, w5/m356); en starts with the space that joins it to the sentence before",
  },
  "deploys.reason.hint_no_http_discovered": {
    message:
      " Beim Erkennen während des Starts hat kein Port auf HTTP geantwortet.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_http_discovered, w5/m356); en starts with the space that joins it to the sentence before",
  },
  "deploys.reason.hint_listening": {
    message: " Der Prozess lauscht auf Port {port}.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_adopted": {
    message:
      " Der Prozess lauscht auf Port {port}, den der Service jetzt verwendet.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_adopted, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_ports": {
    message:
      " Der Prozess lauscht auf den Ports {ports}, nicht auf Port {port}; setze den Port des Services (oder seine Variable PORT) auf den, der HTTP bedient.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_ports, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_primary": {
    message:
      " Der Prozess lauscht auf Port {port}, den der Service jetzt als primären Port verwendet.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_primary, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_port": {
    message:
      " Auf der Instanz wurde kein offener Port erkannt: Der Server muss Host 0.0.0.0 auf dem Port in PORT ({port}) binden.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_port_before": {
    message:
      " Auf der Instanz wurde kein offener Port erkannt: Der Server muss Host 0.0.0.0 auf dem Port in PORT ({port}) binden; zuvor lauschte er auf Port {previous}.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_port_before, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_service_port": {
    message:
      " Auf der Instanz wurde kein offener Port erkannt: Der Server muss Host 0.0.0.0 auf dem Port des Services ({port}) binden.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_service_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.queued_build_limit": {
    message:
      "{noun} hat {active}/{limit} gleichzeitige Builds aktiv; Warten auf einen freien Platz",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_build_limit, w5/m356): a full concurrent-build cap; {noun} is the cap's scope, worded by deploys.reasonParam.noun.*",
  },
  "deploys.reason.queued_cluster_full": {
    message:
      "die Build-Kapazität des Clusters ist ausgelastet; Warten auf einen freien Platz",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_cluster_full, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_slot_taken": {
    message:
      "ein gleichzeitiger Dispatch hat den letzten Build-Platz belegt; Warten auf einen freien Platz",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_slot_taken, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_database": {
    message:
      "Warten, bis die verknüpfte Datenbank {database} verfügbar ist, bevor der Build startet",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_database, w5/m356): what the build waited on; params are the sentence's values",
  },
  "deploys.reason.queued_datastore": {
    message:
      "Warten auf die Verbindungsdetails eines verknüpften Datastores, bevor der Build startet",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_datastore, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_build_credential": {
    message:
      "Warten, bis die Registry die Build-Zugangsdaten dieser App akzeptiert",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_build_credential, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_pull_credential": {
    message:
      "Warten, bis die Registry die Pull-Zugangsdaten dieser App akzeptiert",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_pull_credential, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity": {
    message: "Warten auf Build-Kapazität: {detail}",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity, w5/m356): waiting for build capacity; {detail} is the build plane's own wait, shown as-is",
  },
  "deploys.reason.queued_capacity_pod": {
    message:
      "Warten auf Build-Kapazität: Build-Job wurde abgesendet; Warten auf die Erstellung seines Build-Pods",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_pod, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity_no_node": {
    message: "Warten auf Build-Kapazität: Noch kein Build-Node hat dafür Platz",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_no_node, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity_short": {
    message:
      "Warten auf Build-Kapazität: Noch kein Build-Node hat dafür Platz (nicht genug freie {resources})",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_short, w5/m356): what the build waited on; params are the sentence's values",
  },
  "deploys.reasonParam.kind.startup": {
    message: "Startup",
    description:
      "A deploy reason's {kind} value startup (w4/591): which health check a stalled rollout is waiting on",
  },
  "deploys.reasonParam.kind.readiness": {
    message: "Readiness",
    description:
      "A deploy reason's {kind} value readiness (w4/591): which health check a stalled rollout is waiting on",
  },
  "deploys.reasonParam.noun.workspace": {
    message: "Workspace",
    description:
      "A deploy reason's {noun} value workspace (w5/m356): the scope of a full concurrent-build cap",
  },
  "deploys.reasonWait.duration": {
    message: "{units}",
    description:
      "A deferred restart's wait, restart_wait_timed_out's {wait} (w5/m356): {wait} is bex-api's Go duration (3h0m0s), which en keeps so the sentence reads as the API's; {units} is the duration in words (deploys.reasonWait.hours and the rest), which other languages read",
  },
  "deploys.reasonWait.hours": {
    message: "{hours} Std.",
    description: "A deferred restart's wait in words (w5/m356): its hours",
  },
  "deploys.reasonWait.minutes": {
    message: "{minutes} Min.",
    description: "A deferred restart's wait in words (w5/m356): its minutes",
  },
  "deploys.reasonWait.seconds": {
    message: "{seconds} Sek.",
    description: "A deferred restart's wait in words (w5/m356): its seconds",
  },
};

export default deDeploys;
