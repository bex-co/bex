import type { TranslationEntry } from "@/i18n";

const deServices: Record<string, TranslationEntry> = {
  "services.protectedConfirmationTitle": {
    message: "Bestätigung für geschützte Umgebung",
    description: "Title of a protected-service destructive-action retry dialog",
  },
  "services.protectedConfirmationBody": {
    message:
      "„{name}“ gehört zu einer geschützten Umgebung. Gib den exakten sudo-Befehl ein, den bex-api zurückgegeben hat, um fortzufahren.",
    description: "Body of a protected-service destructive-action retry dialog",
  },
  "services.protectedConfirmationPrompt": {
    message: "Gib unten {phrase} zur Bestätigung ein.",
    description:
      "Body prompt naming the exact protected-action confirmation phrase (rendered bold by SudoCommandField)",
  },
  "services.connect": {
    message: "Verbinden",
    description: "Open the service connection menu",
  },
  "services.connectSSH": {
    message: "SSH",
    description: "SSH section label in the service connection menu",
  },
  "services.openBrowserTerminal": {
    message: "Browser-Terminal öffnen",
    description:
      "RequiresSshKey gate — zero-setup second door to the Web Shell (no SSH key)",
  },
  "services.connectInternal": {
    message: "Intern",
    description:
      "Internal-address section label in the service connection menu (Render's Connect → Internal tab)",
  },
  "services.connectInternalFreeNote": {
    message:
      "Kostenlose Web Services können keinen privaten Netzwerkverkehr empfangen. Upgrade deinen Tarif, um die interne Adresse zu nutzen.",
    description:
      "Connect → Internal note for a free public web service: Render's Free plan does not support receiving private network traffic, so a sibling's internal call never reaches it (w8/m130)",
  },
  "services.headerServiceAddress": {
    message: "Service-Adresse",
    description:
      "Label before a private service's internal address in the detail header (Render's private-service wording)",
  },
  "services.openPorts": {
    message: "Offene Ports",
    description:
      "Label before the TCP ports a private service's hostname serves, beside its Service Address and in Connect → Internal (Render's serviceDetails.openPorts, w6/m148)",
  },
  "services.openPortsPrimary": {
    message: "primär",
    description:
      "Marker on the open port that is the service's primary port — the <slug>:<port> suffix of its Service Address (w6/m148)",
  },
  "services.internalCopy": {
    message: "Interne Adresse kopieren",
    description: "Copy service internal address button",
  },
  "services.internalCopied": {
    message: "Interne Adresse kopiert",
    description: "Successful internal address copy",
  },
  "services.internalCopyError": {
    message: "Interne Adresse konnte nicht kopiert werden",
    description: "Failed internal address copy",
  },
  "services.sshCopy": {
    message: "SSH-Befehl kopieren",
    description: "Copy service SSH command button",
  },
  "services.sshCopied": {
    message: "SSH-Befehl kopiert",
    description: "Successful SSH command copy",
  },
  "services.sshCopyError": {
    message: "SSH-Befehl konnte nicht kopiert werden",
    description: "Failed SSH command copy",
  },
  "services.sshUnavailable": {
    message: "SSH ist nicht verfügbar",
    description: "Service header state when no SSH address is advertised",
  },
  "services.sshUnavailableHint": {
    message:
      "SSH erfordert einen laufenden kostenpflichtigen Web Service, Private Service oder Background Worker und ein aktives Gateway.",
    description: "Explanation for a service without an SSH address",
  },
  "services.sshUnavailableSuspended": {
    message:
      "Dieser Service ist pausiert. Setze ihn fort, um dich per SSH zu verbinden.",
    description: "Why SSH is unavailable on a suspended service (w4/143)",
  },
  "services.sshUnavailableSuspendedFree": {
    message:
      "Dieser Service ist pausiert, und SSH erfordert einen kostenpflichtigen Compute-Plan. Durch das Fortsetzen wird der Traffic wiederhergestellt; kostenlose Services enthalten weiterhin keinen Shell-Zugriff.",
    description:
      "Why SSH is unavailable on a suspended Free service: both blockers, without implying a resume grants SSH (w4/m183)",
  },
  "services.sshUnavailableFree": {
    message:
      "SSH erfordert einen kostenpflichtigen Compute-Plan. Kostenlose Services enthalten keinen Shell-Zugriff.",
    description: "Why SSH is unavailable on a Free-plan service (w4/143)",
  },
  "services.sshRemedyResume": {
    message: "Service fortsetzen",
    description:
      "Link from an SSH-unavailable state to the service's suspend/resume setting (w4/143)",
  },
  "services.sshRemedyChangePlan": {
    message: "Compute-Plan ändern",
    description:
      "Link from an SSH-unavailable state on a Free service to its compute-plan page (w4/143)",
  },
  "services.shellDescription": {
    message:
      "Öffne von deinem Terminal aus eine Shell auf einer laufenden Service-Instanz.",
    description: "Running-instance SSH connection page description",
  },
  "services.shellConnectionTitle": {
    message: "Mit SSH verbinden",
    description: "SSH connection card title",
  },
  "services.shellConnectionDescription": {
    message:
      "bex verbindet sich mit einer vorhandenen, bereiten Instanz. Es wird keine separate Shell-Instanz erstellt.",
    description: "SSH running-instance behavior explanation",
  },
  "services.shellCommand": {
    message: "SSH-Befehl",
    description: "Label for a copy-ready service SSH command",
  },
  "services.shellManageKeys": {
    message: "Öffentliche SSH-Schlüssel verwalten",
    description: "Link from a service shell page to account SSH key settings",
  },
  "services.shellSessionLifecycle": {
    message:
      "Der Befehl wählt eine bereite Instanz aus. Ein Neustart, erneutes Deployen oder Pausieren des Service beendet die Sitzung.",
    description: "SSH session selection and lifecycle guidance",
  },
  "services.shellUnavailableTitle": {
    message: "Shell-Zugriff ist nicht verfügbar",
    description: "Unavailable SSH connection card title",
  },
  "services.shellUnavailableBody": {
    message:
      "Shell-Zugriff erfordert einen laufenden kostenpflichtigen Web Service, Private Service oder Background Worker und ein aktives SSH-Gateway.",
    description: "Unavailable SSH connection card explanation",
  },
  "services.shellWebTitle": {
    message: "Web Shell",
    description: "In-browser terminal card title",
  },
  "services.shellWebDescription": {
    message:
      "Führe Befehle in einer laufenden Instanz direkt in deinem Browser aus.",
    description: "In-browser terminal card description",
  },
  "services.shellConnecting": {
    message: "Verbindung zu einer laufenden Instanz wird hergestellt…",
    description: "Web shell status while the terminal is connecting",
  },
  "services.shellConnected": {
    message: "Verbunden",
    description: "Web shell status when the terminal stream is live",
  },
  "services.shellClosed": {
    message: "Sitzung beendet",
    description: "Web shell status when the terminal stream has ended",
  },
  "services.shellErrorStatus": {
    message: "Verbindungsfehler",
    description:
      "Web shell status when the session failed to connect or errored",
  },
  "services.shellEnded": {
    message: "Sitzung beendet",
    description:
      "Web shell status when the server ended a live session for a stated reason (access, suspension, instance stopped, time limit)",
  },
  "services.shellEndRevoked": {
    message: "Dein Zugriff auf diesen Service wurde beendet.",
    description:
      "Web shell end reason: the caller lost access to the service mid-session (deliberately unspecific)",
  },
  "services.shellEndSuspended": {
    message:
      "Der Service wurde pausiert, wodurch diese Sitzung beendet wurde. Setze ihn fort, um eine neue zu öffnen.",
    description: "Web shell end reason: the service was suspended",
  },
  "services.shellEndIneligible": {
    message:
      "Dieser Service hat keinen Shell-Zugriff mehr (sein Tarif oder Typ hat sich geändert), wodurch diese Sitzung beendet wurde.",
    description:
      "Web shell end reason: the service moved to the Free plan or a type without instances",
  },
  "services.shellEndInstanceGone": {
    message:
      "Die Instanz, auf der diese Sitzung lief, wurde gestoppt. Verbinde dich erneut, um eine Shell auf einer laufenden Instanz zu öffnen.",
    description:
      "Web shell end reason: the attached instance stopped (restart, or the old instance leaving after a deploy went live)",
  },
  "services.shellEndTimeout": {
    message:
      "Die Sitzung hat ihr Zeitlimit erreicht. Verbinde dich erneut, um weiterzuarbeiten.",
    description: "Web shell end reason: the session hit its maximum duration",
  },
  "services.shellEndUnverified": {
    message:
      "Dein Zugriff konnte nicht erneut geprüft werden, daher wurde die Sitzung beendet. Verbinde dich erneut, um es nochmals zu versuchen.",
    description:
      "Web shell end reason: the periodic access check could not complete",
  },
  "services.shellReconnect": {
    message: "Neu verbinden",
    description: "Button that re-opens a closed web shell session",
  },
  "services.shellConnect": {
    message: "Shell starten",
    description: "Button that opens a web shell session",
  },
  "services.shellErrorGeneric": {
    message: "Shell-Sitzung konnte nicht gestartet werden.",
    description: "Web shell generic connection error",
  },
  "services.shellErrorUnavailable": {
    message:
      "Die Shell im Browser ist auf dieser Plattform nicht aktiviert. Nutze stattdessen den SSH-Befehl unten.",
    description:
      "Web shell error when the browser transport is unconfigured (503)",
  },
  "services.shellInstanceLabel": {
    message: "Instanz",
    description: "Label for the web shell instance picker",
  },
  "services.shellInstanceAny": {
    message: "Beliebige bereite Instanz",
    description:
      "Web shell instance picker option that selects a random ready replica",
  },
  "services.shellInstanceSelect": {
    message: "Instanz auswählen",
    description: "Placeholder for the web shell instance picker",
  },
  "services.shellSshFallback": {
    message: "Lieber dein eigenes Terminal? Nutze den SSH-Befehl unten.",
    description: "Hint pointing web shell users to the copy-ready SSH command",
  },
  "services.actions": {
    message: "Aktionen",
    description: "Accessible heading for service configuration row actions",
  },
  "services.statTotal": {
    message: "Services gesamt",
    description: "Services page stat card label",
  },
  "services.statRunning": {
    message: "Laufend",
    description: "Services page stat card label",
  },
  "services.statSuspended": {
    message: "Pausiert",
    description: "Services page stat card label",
  },
  "services.cardTitle": {
    message: "Services",
    description:
      "Services table card title, also used as the metrics page back-link",
  },
  "services.colName": {
    message: "Name",
    description: "Services table column header",
  },
  "services.colStatus": {
    message: "Status",
    description: "Services table column header",
  },
  "services.colUrl": {
    message: "URL",
    description: "Services table column header",
  },
  "services.colBlueprintName": {
    message: "Blueprint-Name",
    description: "Immutable service name used to match Blueprint resources.",
  },
  "services.blueprintNameHelp": {
    message:
      "Verwende diesen unveränderlichen Namen in services[].name in deinem Blueprint-Manifest. Anzeigenamen und öffentliche Hostnamen können abweichen.",
    description: "Immutable service name used to match Blueprint resources.",
  },
  "services.colSlug": {
    message: "Slug",
    description:
      "Service detail header fact (globally-unique platform-host segment, Render's slug field)",
  },
  "services.instancesSuspended": {
    message: "0 ({n} nach Fortsetzen)",
    description:
      "The header's Instances fact for a suspended service, which runs none; n is its configured count (w4/547)",
  },
  "services.instancesSleeping": {
    message: "0 ({n} im aktiven Zustand)",
    description:
      "The header's Instances fact for a sleeping Free service, which runs none; n is its configured count (w4/547)",
  },
  "services.colInstances": {
    message: "Instanzen",
    description: "Services table column header (replica count — bex-native)",
  },
  "services.colRevision": {
    message: "Revision",
    description: "Services table column header (active revision — bex-native)",
  },
  "services.colCreated": {
    message: "Erstellt",
    description: "Services table column header (relative age from createdAt)",
  },
  "services.colActions": {
    message: "Aktionen",
    description: "Services table actions column header (screen-reader only)",
  },
  "services.statusRunning": {
    message: "Läuft",
    description: "Services table status badge",
  },
  "services.statusSuspended": {
    message: "Pausiert",
    description: "Services table status badge",
  },
  "services.statusSleeping": {
    message: "Im Ruhezustand",
    description:
      "Services status badge: a free-tier App auto-hibernated after idle (bex extension)",
  },
  "services.undeployedChangesSuspendedHint": {
    message:
      "Gespeicherte Änderungen sind noch nicht live. Setze den Service fort und deploye dann, um sie anzuwenden.",
    description:
      "Notice when a suspended service has saved settings that differ from the release it serves: no deploy can run until it resumes",
  },
  "services.undeployedChangesHint": {
    message:
      "Gespeicherte Änderungen sind noch nicht live. Nutze ein Standard-Deployment, um sie anzuwenden.",
    description:
      "Notice when saved settings differ from the running release, including Save only, cancellation, rollback or restart",
  },
  "services.undeployedChangesApplyingHint": {
    message:
      "Gespeicherte Änderungen gehen live, sobald das aktuelle Deployment abgeschlossen ist.",
    description:
      "Notice while an in-flight deploy already carries the saved settings, so no further deploy is needed (w4/m184)",
  },
  "services.undeployedChangesOverrideHint": {
    message:
      "Dieses Deployment führt {running} aus, ausgewählt für dieses Deployment. Das gespeicherte Image ist {saved}; ein Standard-Deployment kehrt dazu zurück.",
    description:
      "Notice when a per-deploy image override (imageUrl/imgURL) runs a different image than the saved one: nothing was saved, and a standard deploy reverts to the saved image (w4/414)",
  },
  "services.statusSleepingHint": {
    message:
      "Im Ruhezustand, um Ressourcen zu sparen — wacht bei der nächsten Anfrage an die öffentliche URL auf.",
    description:
      "Hint next to the Sleeping badge explaining free-tier auto-sleep + wake-on-request",
  },
  "services.statusPending": {
    message: "Ausstehend",
    description: "Services table status badge",
  },
  "services.statusBuilding": {
    message: "Build läuft",
    description: "Services table status badge",
  },
  "services.statusDeploying": {
    message: "Wird deployt",
    description: "Services table status badge",
  },
  "services.statusCanceled": {
    message: "Abgebrochen",
    description:
      "Services table status badge for a service whose first-ever deploy the user canceled",
  },
  "services.statusDeleting": {
    message: "Wird gelöscht",
    description:
      "Services status badge for a service whose deletion is finalizing",
  },
  "services.statusFailed": {
    message: "Fehlgeschlagen",
    description: "Services table status badge",
  },
  "services.statusUnknown": {
    message: "Unbekannt",
    description: "Services table status badge for an unrecognized phase",
  },
  "services.actionsMenu": {
    message: "Aktionen für {name}",
    description: "Accessible label for the per-row actions trigger",
  },
  "services.actionSuspend": {
    message: "Pausieren",
    description: "Row action: park the service",
  },
  "services.actionResume": {
    message: "Fortsetzen",
    description: "Row action: bring a suspended service back",
  },
  "services.actionRestart": {
    message: "Neu starten",
    description:
      "Row action: restart the service on the commit or image it is running",
  },
  "services.confirmSuspendTitle": {
    message: "{name} pausieren?",
    description: "Suspend confirmation dialog title",
  },
  "services.confirmSuspendBody": {
    message:
      "Der Service wird auf null herunterskaliert und liefert keinen Traffic mehr aus. Seine URL und Zertifikate bleiben erhalten, und du kannst ihn jederzeit fortsetzen.",
    description:
      "Suspend confirmation dialog body, for the types served at a public host",
  },
  "services.confirmSuspendBodyNoUrl": {
    message:
      "Der Service wird auf null herunterskaliert und läuft nicht mehr. Du kannst ihn jederzeit fortsetzen.",
    description:
      "Suspend confirmation dialog body, for private services and background workers — neither has a public URL or certificates to keep",
  },
  "services.confirmSuspendBodyStatic": {
    message:
      "Die Site liefert nichts mehr aus und antwortet während der Pause mit 503. Ihre URL und Zertifikate bleiben erhalten, und du kannst sie jederzeit fortsetzen.",
    description:
      "Suspend confirmation dialog body for a static site, which has no instances to scale to zero (w4/380)",
  },
  "services.suspendCronDescription": {
    message:
      "Das Pausieren dieses Cron Jobs hält künftige geplante Läufe an. Ein aktiver Lauf wird fortgesetzt. Um ihn zu stoppen, wähle auf der Seite „Läufe“ die Option „Abbrechen“.",
    description:
      "Cron suspension explanation in Settings and suspend confirmations; suspension does not cancel an active run",
  },
  "services.confirmRestartTitle": {
    message: "{name} neu starten?",
    description: "Restart confirmation dialog title",
  },
  "services.confirmRestartBody": {
    message:
      "{name} wird mit dem Commit oder Image neu gestartet, der bzw. das aktuell läuft. Seitdem gepushte Commits werden nicht deployt. Neue Instanzen ersetzen die alten, sobald sie fehlerfrei laufen.",
    description:
      "Restart confirmation dialog body (service header and services list). A restart keeps the running release, as on Render; it never picks up newer commits",
  },
  "services.confirmRestartBodyStatic": {
    message:
      "{name} wird erneut mit dem Commit veröffentlicht, der aktuell ausgeliefert wird. Seitdem gepushte Commits werden nicht deployt.",
    description:
      "Restart confirmation body for a static site, which has no instances to replace (w4/m141)",
  },
  "services.confirmCancel": {
    message: "Abbrechen",
    description: "Confirmation dialog cancel button",
  },
  "services.toastSuspendSuccess": {
    message: "{name} wird pausiert…",
    description: "Toast shown after a suspend request is accepted",
  },
  "services.toastResumed": {
    message: "{name} wurde fortgesetzt.",
    description:
      "Toast after a resume is accepted, Render's words (w3/m643); {name} is drawn in bold",
  },
  "services.toastResumeError": {
    message:
      "Beim Fortsetzen von {name} ist ein Fehler aufgetreten. Versuche es erneut.",
    description:
      "Toast when a resume fails and the server names no reason, Render's words (w3/m643); {name} is drawn in bold",
  },
  "services.toastRestartSuccess": {
    message: "{name} wird neu gestartet…",
    description: "Toast shown after a restart request is accepted",
  },
  "services.toastError": {
    message:
      "{name} konnte nicht aktualisiert werden. Bitte versuche es erneut.",
    description: "Toast shown when a lifecycle action fails",
  },
  "services.errorTitle": {
    message: "Services konnten nicht geladen werden",
    description: "Services list error card title",
  },
  "services.errorBody": {
    message:
      "Die Anfrage an bex-api ist fehlgeschlagen. Prüfe deine Verbindung und versuche es erneut.",
    description: "Services list error card body",
  },
  "services.detailErrorTitle": {
    message: "Service konnte nicht geladen werden",
    description: "Service detail query error card title",
  },
  "services.detailErrorBody": {
    message:
      "Die Anfrage an bex-api ist fehlgeschlagen. Der Service existiert möglicherweise noch.",
    description: "Service detail query error card body",
  },
  "services.emptyTitle": {
    message: "Noch keine Services",
    description: "Services list empty state title",
  },
  "services.emptyBody": {
    message: "Deploye deine erste App, und sie erscheint hier.",
    description: "Services list empty state body",
  },
  "services.headerServiceId": {
    message: "Service-ID:",
    description: "Service-detail header metadata label for the service id",
  },
  "services.headerServicePhase": {
    message: "Service",
    description:
      "Service-detail header label distinguishing operator/App phase from deploy status",
  },
  "services.headerLatestDeploy": {
    message: "Letztes Deployment",
    description: "Service-detail header label for the newest deploy status",
  },
  "services.headerRuntime": {
    message: "Runtime",
    description: "Service-detail header label for the selected runtime",
  },
  "services.headerMoreFacts": {
    message: "Einzelheiten",
    description:
      "Service-detail header toggle, phones only, that shows or hides the slug, instances, revision and age line",
  },
  "services.headerLastRun": {
    message: "Letzter erfolgreicher Lauf",
    description:
      "Service-detail header: a cron job's second fact line, before the last successful run's date (Render's \"Last successful run <date>\", w3/m555)",
  },
  "services.headerLastRunUtc": {
    message: "UTC: {time}",
    description:
      'Service-detail header: the tooltip on a cron job\'s last successful run date, shown only when the viewer\'s zone is not UTC; {time} is the instant in UTC as "Nov 21 05:00:00 PM" (Render\'s "UTC: <date>", w3/m643)',
  },
  "services.headerRuns": {
    message: "Startet",
    description:
      "Service-detail header: the word before a cron job's command on its first fact line (Render's \"Runs <command> <schedule>\", w3/m555)",
  },
  "services.headerRunsDockerCmd": {
    message: "{cmd} aus {dockerfile}",
    description:
      "Service-detail header: what a Docker or image cron job with no command override runs; {cmd} is the code word CMD and {dockerfile} the Dockerfile's path, both shown as code (Render's \"CMD in Dockerfile\", w3/m555)",
  },
  "services.headerNoSuccessfulRuns": {
    message: "Noch keine erfolgreichen Läufe.",
    description:
      "Service-detail header: a cron job's second fact line before its first successful run (Render's text, w3/m555)",
  },
  "services.suspendedNoticeManual": {
    message: "Manuell pausiert",
    description:
      "Title of the notice under a suspended service's header when its owner suspended it (Render's \"Manually suspended\", w3/m555, w3/107)",
  },
  "services.suspendedNoticeByYou": {
    message: "Von dir pausiert",
    description:
      "Title of the notice under a suspended service's header when its owner suspended it in a Hobby workspace; every other plan reads services.suspendedNoticeManual (Render's \"Suspended by you\", w3/m643)",
  },
  "services.suspendedNoticeTitle": {
    message: "Pausiert",
    description:
      "Title of the notice under a suspended service's header when no suspender is known (Render's fallback \"Suspended\", w3/m555, w3/107)",
  },
  "services.suspendedNoticeByPlatform": {
    message: "Von bex pausiert",
    description:
      "Title of the notice under a suspended service's header when the platform suspended it for spent Free instance hours or bandwidth (the billing suspender; Render's \"Suspended by Render\", w3/107)",
  },
  "services.suspendedNoticeCrashLoop": {
    message: "Dieser Service wurde wegen wiederholter Abstürze pausiert",
    description:
      "Title of the notice under a suspended service's header when the platform suspended it for a crash loop (stuck_crashlooping; Render's title, w3/107)",
  },
  "services.suspendedNoticeBody": {
    message: "Pausierte Services werden dir nicht berechnet.",
    description:
      "Body of the notice under a suspended service's header (Render's text, w3/m555, w3/107)",
  },
  "services.suspendedNoticeResume": {
    message: "Service fortsetzen",
    description:
      "The notice's inline action that resumes a suspended service (Render's \"Resume service\", w3/m555, w3/107)",
  },
  "services.headerCopyServiceId": {
    message: "Service-ID kopieren",
    description: "Accessible label for the header's service-id copy button",
  },
  "services.headerCopyUrl": {
    message: "Service-URL kopieren",
    description: "Accessible label for the header's live-URL copy button",
  },
  "services.headerCopied": {
    message: "In die Zwischenablage kopiert",
    description: "Toast after copying a value from the service-detail header",
  },
  "services.headerCopyError": {
    message: "Kopieren in die Zwischenablage fehlgeschlagen",
    description: "Toast when a service-detail header copy fails",
  },
  "services.navLogs": {
    message: "Logs",
    description: "Service-detail nav item (logs tab)",
  },
  "services.navMetrics": {
    message: "Metriken",
    description: "Service-detail nav item (metrics tab)",
  },
  "services.navRedirects": {
    message: "Redirects/Rewrites",
    description:
      "Service-detail nav item (static-site route rules page, Render's label)",
  },
  "services.navHeaders": {
    message: "Header",
    description:
      "Service-detail nav item (static-site response headers page, Render's label)",
  },
  "services.scalingTitle": {
    message: "Autoscaling",
    description: "Scaling tab section heading",
  },
  "services.autoscaleMinInstances": {
    message: "Minimale Instanzen",
    description:
      "Accessible name of the minimum-instances number input beside the autoscaling range slider",
  },
  "services.autoscaleMaxInstances": {
    message: "Maximale Instanzen",
    description:
      "Accessible name of the maximum-instances number input beside the autoscaling range slider",
  },
  "services.scalingEnabled": {
    message: "Autoscaling aktivieren",
    description: "Autoscaling enable/disable toggle label",
  },
  "services.scalingMinInstances": {
    message: "Min. Instanzen",
    description: "Autoscaling min instances input label",
  },
  "services.scalingMaxInstances": {
    message: "Max. Instanzen",
    description: "Autoscaling max instances input label",
  },
  "services.scalingTargetCPU": {
    message: "Ziel-CPU %",
    description: "Autoscaling target CPU utilization input label",
  },
  "services.scalingTargetMemory": {
    message: "Ziel-Arbeitsspeicher %",
    description: "Autoscaling target memory utilization input label",
  },
  "services.scalingValidationMinMax": {
    message: "Min. Instanzen müssen ≤ max. Instanzen sein.",
    description: "Autoscaling form validation when min exceeds max",
  },
  "services.scalingValidationTargets": {
    message:
      "Mindestens ein Auslastungsziel (CPU oder Arbeitsspeicher) muss aktiviert sein.",
    description:
      "Autoscaling form validation when both CPU and memory targets are off",
  },
  "services.scalingSave": {
    message: "Speichern",
    description: "Autoscaling form save button",
  },
  "services.scalingSaved": {
    message: "Autoscaling-Einstellungen gespeichert.",
    description: "Autoscaling save success toast",
  },
  "services.scalingDisabled": {
    message: "Autoscaling deaktiviert.",
    description: "Autoscaling disable success toast",
  },
  "services.scalingError": {
    message: "Autoscaling-Einstellungen konnten nicht aktualisiert werden.",
    description: "Autoscaling save error toast",
  },
  "services.scalingDescription": {
    message:
      "Skaliere diesen Service automatisch hoch oder runter, basierend auf CPU- und Arbeitsspeicherauslastung.",
    description: "Autoscaling card description",
  },
  "services.scalingOn": {
    message: "Autoscaling an",
    description: "Autoscaling main toggle label when enabled",
  },
  "services.scalingPaidOnly": {
    message:
      "Autoscaling ist in kostenpflichtigen Tarifen verfügbar. Upgrade deinen Tarif, um es zu aktivieren.",
    description: "Autoscaling card explanation for a free service",
  },
  "services.scalingPaidOnlyInactive": {
    message:
      "Autoscaling-Einstellungen sind gespeichert, im Free-Tarif aber inaktiv. Sie greifen, sobald dieser Service in einen kostenpflichtigen Tarif wechselt. Schalte Autoscaling aus, um die gespeicherten Einstellungen zu entfernen.",
    description:
      "Autoscaling card explanation for a free service carrying a stored autoscaling config (w4/365)",
  },
  "services.scalingProPlanOnly": {
    message:
      "Autoscaling ist im Pro-Workspace-Tarif und höher verfügbar. Dieser Workspace nutzt Hobby.",
    description:
      "Autoscaling card notice on a Hobby workspace, followed by an Open Billing link (w7/m429)",
  },
  "services.scalingProPlanOnlyRunning": {
    message:
      "Autoscaling ist im Pro-Workspace-Tarif und höher verfügbar. Dieser Workspace nutzt Hobby: Die Richtlinie unten läuft weiter und kann ausgeschaltet werden, aber sie zu ändern oder wieder einzuschalten erfordert Pro.",
    description:
      "Autoscaling card notice on a Hobby workspace whose service already autoscales, followed by an Open Billing link (w7/m429)",
  },
  "services.scalingMetricsUnavailableTitle": {
    message:
      "Autoscaling ist pausiert: Auslastungsmetriken sind nicht verfügbar",
    description:
      "Autoscaling card notice title while the operator cannot read the service's CPU and memory utilization (w4/m364)",
  },
  "services.scalingMetricsUnavailableBody": {
    message:
      "Der Service behält seine aktuelle Instanzanzahl, bis CPU- und Speicherauslastung wieder gelesen werden können. Deine Autoscaling-Einstellungen sind gespeichert.",
    description:
      "Autoscaling card notice body while utilization metrics are unavailable (w4/m364)",
  },
  "services.scalingOff": {
    message: "Autoscaling aus",
    description: "Autoscaling main toggle label when disabled",
  },
  "services.scalingInstancesTitle": {
    message: "Anzahl der Instanzen",
    description: "Autoscaling instances range-slider section heading",
  },
  "services.scalingInstancesHint": {
    message:
      "bex skaliert die Anzahl der Instanzen für diesen Service innerhalb des Bereichs, den du angibst.",
    description: "Autoscaling instances range-slider section description",
  },
  "services.scalingCPUTitle": {
    message: "Ziel-CPU-Auslastung",
    description: "Autoscaling CPU metric section heading",
  },
  "services.scalingCPUHint": {
    message:
      "Liegt die durchschnittliche CPU-Auslastung deutlich über oder unter diesem Wert, fügt bex entsprechend Instanzen hinzu oder entfernt sie.",
    description: "Autoscaling CPU metric section description",
  },
  "services.scalingMemoryTitle": {
    message: "Ziel-Arbeitsspeicherauslastung",
    description: "Autoscaling memory metric section heading",
  },
  "services.scalingMemoryHint": {
    message:
      "Liegt die durchschnittliche Arbeitsspeicherauslastung deutlich über oder unter diesem Wert, fügt bex entsprechend Instanzen hinzu oder entfernt sie.",
    description: "Autoscaling memory metric section description",
  },
  "services.scalingCancel": {
    message: "Abbrechen",
    description: "Autoscaling form cancel button",
  },
  "services.scalingSaveChanges": {
    message: "Änderungen speichern",
    description: "Autoscaling form save-changes button",
  },
  "services.scalingDisableConfirmTitle": {
    message: "Autoscaling deaktivieren?",
    description: "Autoscaling disable confirmation dialog title (w7/m43)",
  },
  "services.scalingDisableConfirmBody": {
    message:
      "Dein Service läuft mit der festen Anzahl von Instanzen, die unter „Manuelle Skalierung“ angegeben ist.",
    description: "Autoscaling disable confirmation dialog body",
  },
  "services.scalingDisableConfirmAction": {
    message: "Deaktivieren",
    description: "Autoscaling disable confirmation dialog confirm button",
  },
  "services.scalingManualTitle": {
    message: "Manuelle Skalierung",
    description: "Scaling page: manual instance-count card title (w7/m43)",
  },
  "services.scalingManualDescription": {
    message:
      "Betreibe mehrere Instanzen, die automatisch lastverteilt werden. Alle Instanzen nutzen denselben Compute-Plan und werden entsprechend abgerechnet.",
    description: "Scaling page: manual instance-count card description",
  },
  "services.scalingManualDescriptionFree": {
    message:
      "Kostenlose Services laufen mit einer einzelnen Instanz. Upgrade auf einen kostenpflichtigen Tarif, um mehrere lastverteilte Instanzen zu betreiben.",
    description:
      "Scaling page: manual instance-count card description on the free plan",
  },
  "services.scalingManualInstances": {
    message: "Instanzen",
    description: "Scaling page: manual instance-count slider label",
  },
  "services.scalingMetricsTitle": {
    message: "Aktuelle Metriken",
    description: "Scaling page: recent-metrics section title (w7/m43)",
  },
  "services.scalingMetricsNote": {
    message: "Angezeigt werden die Metriken der letzten 48 Stunden.",
    description: "Scaling page: recent-metrics window note",
  },
  "services.scalingMetricsViewAll": {
    message: "Alle Metriken ansehen.",
    description: "Scaling page: link to the full Metrics tab",
  },
  "services.scalingMetricsMemory": {
    message: "Durchschnittliche Arbeitsspeicherauslastung",
    description: "Scaling page: averaged memory utilization chart title",
  },
  "services.scalingMetricsCPU": {
    message: "Durchschnittliche CPU-Auslastung",
    description: "Scaling page: averaged CPU utilization chart title",
  },
  "services.scalingMetricsAcross": {
    message: "Über alle Instanzen",
    description: "Scaling page: recent-metrics chart subtitle",
  },
  "services.scalingMetricsEmpty": {
    message: "In den letzten 48 Stunden wurden keine Daten erfasst",
    description: "Scaling page: recent-metrics empty state",
  },
  "services.navEnvironment": {
    message: "Umgebung",
    description: "Service-detail nav item (environment variables tab)",
  },
  "services.envTitle": {
    message: "Umgebungsvariablen",
    description: "Environment tab card title",
  },
  "services.envDescription": {
    message:
      "Lege umgebungsspezifische Konfiguration und Secrets fest und lies diese Werte dann aus deinem Code.",
    description: "Environment tab card description",
  },
  "services.envColKey": {
    message: "Schlüssel",
    description: "Environment table column header (variable name)",
  },
  "services.envColValue": {
    message: "Wert",
    description: "Environment table column header (variable value)",
  },
  "services.envManifestManaged": {
    message: "Von Blueprint verwaltet",
    description:
      "Badge on an environment row whose value the service's render.yaml manifest owns",
  },
  "services.envManifestManagedBody": {
    message:
      "Diese Variable ist im render.yaml-Manifest des Service deklariert, das ihren Wert verwaltet. Bearbeite das Manifest und synchronisiere den Blueprint, um sie zu ändern, oder deklariere sie mit `sync: false`, damit stattdessen das Dashboard sie verwaltet.",
    description:
      "Explanation under a manifest-managed environment row: why it cannot be edited here and what to do instead",
  },
  "services.envShowSecret": {
    message: "Wert anzeigen",
    description: "Environment row button to reveal a masked value",
  },
  "services.envHideSecret": {
    message: "Wert ausblenden",
    description: "Environment row button to re-mask a revealed value",
  },
  "services.envRevealError": {
    message: "Der Wert konnte nicht geladen werden.",
    description: "Environment row inline error when a value reveal fails",
  },
  "services.envEmptyTitle": {
    message: "Keine Umgebungsvariablen",
    description: "Environment tab empty-state title",
  },
  "services.envEmptyBody": {
    message: "Füge eine Variable hinzu, um diesen Service zu konfigurieren.",
    description: "Environment tab empty-state body",
  },
  "services.envUnavailableTitle": {
    message: "Umgebungsvariablen nicht verfügbar",
    description:
      "Environment tab state when the secret store is unconfigured (503)",
  },
  "services.envUnavailableBody": {
    message:
      "Der Secret-Speicher ist für dieses Deployment nicht konfiguriert.",
    description: "Environment tab unavailable-state body",
  },
  "services.envForbiddenTitle": {
    message: "Nicht berechtigt",
    description: "Environment tab state when the caller lacks permission (403)",
  },
  "services.envForbiddenBody": {
    message:
      "Du hast keine Berechtigung, die Umgebungsvariablen dieses Service einzusehen.",
    description: "Environment tab forbidden-state body",
  },
  "services.envErrorTitle": {
    message: "Umgebungsvariablen konnten nicht geladen werden",
    description: "Environment tab generic error title",
  },
  "services.envErrorBody": {
    message: "Etwas ist schiefgelaufen. Bitte versuche es erneut.",
    description: "Environment tab generic error body",
  },
  "services.envAdd": {
    message: "Variable hinzufügen",
    description: "Environment tab button to open the add-variable form",
  },
  "services.envExport": {
    message: "Exportieren",
    description:
      "Environment tab button that downloads all service variables as dotenv",
  },
  "services.envExportSuccess": {
    message: "Umgebung exportiert",
    description:
      "Toast after every current environment value was safely exported",
  },
  "services.envExportError": {
    message:
      "Die vollständige Umgebung konnte nicht exportiert werden. Es wurde keine Datei heruntergeladen.",
    description:
      "Fail-closed toast when any environment value cannot be revealed",
  },
  "services.envEdit": {
    message: "Bearbeiten",
    description: "Environment row button to edit a variable's value",
  },
  "services.envDelete": {
    message: "Löschen",
    description: "Environment row button to remove a variable",
  },
  "services.envValueFor": {
    message: "Wert für {name}",
    description:
      "Accessible name for a Value textbox once its Key is set (m101/t013)",
  },
  "services.envDeleteOne": {
    message: "{name} löschen",
    description:
      "Accessible name for a Delete button once its Key is set (m101/t013)",
  },
  "services.envSave": {
    message: "Speichern",
    description: "Environment add/edit form save button",
  },
  "services.envCancel": {
    message: "Abbrechen",
    description: "Environment add/edit form cancel button",
  },
  "services.envKeyPlaceholder": {
    message: "NAME_OF_VARIABLE",
    description: "Environment add-variable key input placeholder",
  },
  "services.envValuePlaceholder": {
    message: "Wert",
    description: "Environment value input placeholder",
  },
  "services.envGenerate": {
    message: "Generieren",
    description: "Generate a cryptographically random environment value",
  },
  "services.envGeneratePlaceholder": {
    message: "Wird beim Speichern sicher generiert",
    description:
      "Environment value placeholder while server generation is selected",
  },
  "services.envInvalidKey": {
    message:
      "Verwende bis zu 253 Buchstaben, Ziffern, Punkte, Bindestriche und Unterstriche; nicht '.' und nicht mit '..' beginnend.",
    description:
      "Environment add-variable validation message for an invalid key",
  },
  "services.envDeleteConfirmTitle": {
    message: "{key} entfernen?",
    description: "Environment delete-confirmation dialog title",
  },
  "services.envDeleteConfirmBody": {
    message: "Der Service wird ohne diese Variable neu deployt.",
    description: "Environment delete-confirmation dialog body",
  },
  "services.envRolloutNote": {
    message: "Der Service wird neu deployt, um die Änderung anzuwenden.",
    description:
      "Toast description after an env-var write (bex rolls the pods)",
  },
  "services.envSaveSuccess": {
    message: "{key} gespeichert",
    description: "Toast on a successful env-var add/update",
  },
  "services.envSaveError": {
    message: "{key} konnte nicht gespeichert werden",
    description: "Toast on a failed env-var add/update",
  },
  "services.envDeleteSuccess": {
    message: "{key} entfernt",
    description: "Toast on a successful env-var delete",
  },
  "services.envDeleteError": {
    message: "{key} konnte nicht entfernt werden",
    description: "Toast on a failed env-var delete",
  },
  "services.secretFilesTitle": {
    message: "Secret-Dateien",
    description: "Environment tab secret-files section title",
  },
  "services.secretFilesDescription": {
    message:
      "Speichere Dateien mit geheimen Inhalten, etwa einen privaten Schlüssel oder eine .env-Datei. Nach dem nächsten Deployment liest dein Service jede unter /etc/secrets/<dateiname>.",
    description: "Environment tab secret-files section description",
  },
  "services.secretFileColName": {
    message: "Dateiname",
    description: "Secret-files table column header (file name)",
  },
  "services.secretFileColContent": {
    message: "Inhalt",
    description: "Secret-files table column header (file body)",
  },
  "services.secretFileRevealError": {
    message:
      "Der Inhalt dieser Secret-Datei konnte nicht geladen werden. Schließe den Dialog und versuche es erneut.",
    description:
      "Shown in the secret-file dialog when reading the stored contents fails.",
  },
  "services.secretFilesEmptyTitle": {
    message: "Keine Secret-Dateien",
    description: "Secret-files empty-state title",
  },
  "services.secretFilesEmptyBody": {
    message:
      "Füge eine Datei hinzu, um geheime Inhalte in diesen Service einzubinden.",
    description: "Secret-files empty-state body",
  },
  "services.secretFilesUnavailableTitle": {
    message: "Secret-Dateien nicht verfügbar",
    description:
      "Secret-files state when the secret store is unconfigured (503)",
  },
  "services.secretFilesUnavailableBody": {
    message:
      "Der Secret-Speicher ist für dieses Deployment nicht konfiguriert.",
    description: "Secret-files unavailable-state body",
  },
  "services.secretFilesForbiddenTitle": {
    message: "Nicht berechtigt",
    description: "Secret-files state when the caller lacks permission (403)",
  },
  "services.secretFilesForbiddenBody": {
    message:
      "Du hast keine Berechtigung, die Secret-Dateien dieses Service einzusehen.",
    description: "Secret-files forbidden-state body",
  },
  "services.secretFilesErrorTitle": {
    message: "Secret-Dateien konnten nicht geladen werden",
    description: "Secret-files generic error title",
  },
  "services.secretFilesErrorBody": {
    message: "Etwas ist schiefgelaufen. Bitte versuche es erneut.",
    description: "Secret-files generic error body",
  },
  "services.secretFileAdd": {
    message: "Secret-Datei hinzufügen",
    description: "Secret-files button to open the add-file form",
  },
  "services.secretFileNamePlaceholder": {
    message: "dateiname.ext",
    description: "Secret-files add-file name input placeholder",
  },
  "services.secretFileContentPlaceholder": {
    message: "Dateiinhalt",
    description: "Secret-files content input placeholder",
  },
  "services.secretFileInvalidName": {
    message:
      "Verwende bis zu 253 Buchstaben, Ziffern, Punkte, Bindestriche und Unterstriche; nicht '.' oder '..'.",
    description: "Secret-files add-file validation message for an invalid name",
  },
  "services.secretFileNameHint": {
    message:
      "Verwende bis zu 253 Buchstaben, Ziffern, Punkte, Bindestriche und Unterstriche; nicht '.' oder '..'.",
    description:
      "Neutral file-name rule under a new secret-file row before the user types or blurs — same rule as secretFileInvalidName but helper styling, not an error",
  },
  "services.secretFileDeleteConfirmTitle": {
    message: "{name} entfernen?",
    description: "Secret-file delete-confirmation dialog title",
  },
  "services.secretFileDeleteConfirmBody": {
    message: "Der Service wird ohne diese Datei neu deployt.",
    description: "Secret-file delete-confirmation dialog body",
  },
  "services.secretFileSaveSuccess": {
    message: "{name} gespeichert",
    description: "Toast on a successful secret-file add/update",
  },
  "services.secretFileSaveError": {
    message: "{name} konnte nicht gespeichert werden",
    description: "Toast on a failed secret-file add/update",
  },
  "services.secretFileDeleteSuccess": {
    message: "{name} entfernt",
    description: "Toast on a successful secret-file delete",
  },
  "services.secretFileDeleteError": {
    message: "{name} konnte nicht entfernt werden",
    description: "Toast on a failed secret-file delete",
  },
  "services.envGroupsTitle": {
    message: "Umgebungsgruppen",
    description: "Environment tab env-groups section title",
  },
  "services.envGroupsDescription": {
    message:
      "Wiederverwendbare Bündel aus Umgebungsvariablen und Secret-Dateien, die du mit diesem und anderen Services verknüpfen kannst.",
    description: "Environment tab env-groups section description",
  },
  "services.envGroupsEmptyTitle": {
    message: "Keine Umgebungsgruppen",
    description: "Env-groups empty-state title",
  },
  "services.envGroupsEmptyBody": {
    message:
      "Erstelle eine Gruppe, um Konfiguration über Services hinweg zu teilen.",
    description: "Env-groups empty-state body",
  },
  "services.envGroupsUnavailableTitle": {
    message: "Umgebungsgruppen nicht verfügbar",
    description: "Env-groups state when the secret store is unconfigured (503)",
  },
  "services.envGroupsUnavailableBody": {
    message:
      "Der Secret-Speicher ist für dieses Deployment nicht konfiguriert.",
    description: "Env-groups unavailable-state body",
  },
  "services.envGroupsForbiddenTitle": {
    message: "Nicht berechtigt",
    description: "Env-groups state when the caller lacks permission (403)",
  },
  "services.envGroupsForbiddenBody": {
    message: "Du hast keine Berechtigung, Umgebungsgruppen einzusehen.",
    description: "Env-groups forbidden-state body",
  },
  "services.envGroupsErrorTitle": {
    message: "Umgebungsgruppen konnten nicht geladen werden",
    description: "Env-groups generic error title",
  },
  "services.envGroupsErrorBody": {
    message: "Etwas ist schiefgelaufen. Bitte versuche es erneut.",
    description: "Env-groups generic error body",
  },
  "services.envGroupCreate": {
    message: "Gruppe erstellen",
    description: "Env-groups button to open the create-group form",
  },
  "services.envGroupCreateSubmit": {
    message: "Erstellen",
    description: "Env-groups create-group form submit button",
  },
  "services.envGroupNamePlaceholder": {
    message: "gruppenname",
    description: "Env-groups create-group name input placeholder",
  },
  "services.envGroupNameLabel": {
    message: "Gruppenname",
    description: "Env-groups create-group name input accessible label",
  },
  "services.envGroupInvalidName": {
    message: "Gib einen Gruppennamen ein.",
    description:
      "Env-groups create-group validation message for an invalid name",
  },
  "services.envGroupLinked": {
    message: "Verknüpft",
    description:
      "Env-groups badge: this group is linked to the current service",
  },
  "services.envGroupWorkspaceScope": {
    message: "Workspace",
    description: "Scope label for a workspace-scoped environment group",
  },
  "services.envGroupOutOfScope": {
    message:
      "Gültig für {groupScope}, aber dieser Service gehört zu {serviceScope} — seine Variablen und Dateien gelten weiterhin.",
    description:
      "Notice on a linked group whose scope no longer matches the service (w4/m238)",
  },
  "services.envGroupOutOfScopeNoEnvironment": {
    message:
      "Gültig für {groupScope}, aber dieser Service hat keine Umgebung — seine Variablen und Dateien gelten weiterhin.",
    description:
      "Notice on a linked group scoped to an environment while the service has none (w4/m238)",
  },
  "services.envGroupEmptyContents": {
    message: "Noch keine Variablen oder Dateien.",
    description: "Env-groups: shown when a group has no vars or secret files",
  },
  "services.envGroupKeyOverridden": {
    message:
      "Überschrieben durch die eigene Umgebungsvariable {key} dieses Service",
    description:
      "Tooltip on a linked group's key badge when a service variable of the same name shadows it at runtime (service wins)",
  },
  "services.envGroupFileOverridden": {
    message:
      "Überschrieben durch die eigene Secret-Datei {name} dieses Service",
    description:
      "Tooltip on a linked group's secret-file badge when a service secret file of the same name shadows it at runtime (service wins)",
  },
  "services.envGroupKeyShadowedByGroup": {
    message: "Überschrieben von {group}, das später erstellt wurde",
    description:
      "Tooltip on a linked group's key badge when a MORE recently created linked group defines the same key and wins it",
  },
  "services.envGroupFileShadowedByGroup": {
    message: "Überschrieben von {group}, das später erstellt wurde",
    description:
      "Tooltip on a linked group's secret-file badge when a MORE recently created linked group defines the same file name and wins it",
  },
  "services.envGroupOverriddenNote": {
    message:
      "Durchgestrichene Schlüssel werden durch die eigenen Umgebungsvariablen dieses Service überschrieben.",
    description:
      "Legend under a linked group's key list when at least one key is shadowed by a service variable",
  },
  "services.envGroupLink": {
    message: "Verknüpfen",
    description: "Env-groups button: attach this group to the current service",
  },
  "services.envGroupUnlink": {
    message: "Verknüpfung aufheben",
    description:
      "Env-groups button: detach this group from the current service",
  },
  "services.envGroupLinkAria": {
    message: "{name} verknüpfen",
    description:
      "Accessible name of an env-group row's Link button on the service Environment tab, naming the group (w4/537)",
  },
  "services.envGroupUnlinkAria": {
    message: "Verknüpfung mit {name} aufheben",
    description:
      "Accessible name of an env-group row's Unlink button on the service Environment tab, naming the group (w4/537)",
  },
  "services.envGroupDelete": {
    message: "Löschen",
    description: "Env-groups action: delete the group",
  },
  "services.envGroupDeleteConfirmTitle": {
    message: "{name} löschen?",
    description: "Env-group delete-confirmation dialog title",
  },
  "services.envGroupDeleteConfirmBody": {
    message:
      "Die Gruppe wird aus jedem Service entfernt, mit dem sie verknüpft ist. Das lässt sich nicht rückgängig machen.",
    description: "Env-group delete-confirmation dialog body",
  },
  "services.envGroupCreateSuccess": {
    message: "{name} erstellt",
    description: "Toast on a successful env-group create",
  },
  "services.envGroupCreateError": {
    message: "{name} konnte nicht erstellt werden",
    description: "Toast on a failed env-group create",
  },
  "services.envGroupDeleteSuccess": {
    message: "Gruppe gelöscht",
    description: "Toast on a successful env-group delete",
  },
  "services.envGroupDeleteError": {
    message: "Die Gruppe konnte nicht gelöscht werden",
    description: "Toast on a failed env-group delete",
  },
  "services.envGroupLinkSuccess": {
    message: "Gruppe verknüpft",
    description: "Toast on a successful env-group link",
  },
  "services.envGroupLinkError": {
    message: "Die Gruppe konnte nicht verknüpft werden",
    description: "Toast on a failed env-group link",
  },
  "services.envGroupUnlinkSuccess": {
    message: "Verknüpfung mit der Gruppe aufgehoben",
    description: "Toast on a successful env-group unlink",
  },
  "services.envGroupUnlinkError": {
    message: "Die Verknüpfung mit der Gruppe konnte nicht aufgehoben werden",
    description: "Toast on a failed env-group unlink",
  },
  "services.navSettings": {
    message: "Einstellungen",
    description: "Service-detail nav item (settings tab)",
  },
  "services.settingsNavigation": {
    message: "Einstellungsbereiche",
    description: "Accessible label for the service settings section navigation",
  },
  "services.settingsDescription": {
    message:
      "Konfiguriere Name, Instanzgröße und weitere Einstellungen dieses Service.",
    description: "Settings tab card description",
  },
  "services.generalTitle": {
    message: "Allgemein",
    description:
      "Settings tab: first card heading, Render's 'General' section (w5/m52)",
  },
  "services.regionLabel": {
    message: "Region",
    description:
      "Settings General card: read-only platform placement row label",
  },
  "services.regionHint": {
    message:
      "Die Region, in der dein Service läuft. Services in derselben Region können über das private Netzwerk kommunizieren.",
    description: "Settings General card: Region row explanatory copy",
  },
  "services.editRowSave": {
    message: "Änderungen speichern",
    description:
      "Shared editable-field row: save button shown while editing (Render parity)",
  },
  "services.editRowCancel": {
    message: "Abbrechen",
    description: "Shared editable-field row: cancel button shown while editing",
  },
  "services.editRowOptional": {
    message: "Optional",
    description:
      "Shared editable-field row: badge next to a label for an optional field",
  },
  "services.displayNameLabel": {
    message: "Service-Name",
    description:
      "Settings tab row label for the mutable human-facing service name",
  },
  "services.displayNameHint": {
    message:
      "Die Service-ID bleibt {id}; URLs und Infrastruktur ändern sich nicht.",
    description:
      "Settings tab explanation that a display-name change preserves identity",
  },
  "services.displayNameEdit": {
    message: "Service-Namen bearbeiten",
    description: "Accessible label for the service-name edit button",
  },
  "services.displayNameSuccess": {
    message: "Service in „{name}“ umbenannt.",
    description: "Toast after setDisplayName succeeds",
  },
  "services.displayNameCleared": {
    message: "Benutzerdefinierter Service-Name entfernt.",
    description: "Toast after clearing displayName",
  },
  "services.displayNameRefreshFailed": {
    message:
      "In „{name}“ umbenannt, aber diese Seite konnte nicht aktualisiert werden. Versuche es erneut oder lade neu, um den gespeicherten Namen zu sehen.",
    description:
      "Warning after a service rename committed but the page could not re-read the saved name; offers Try again",
  },
  "services.displayNameUnpublished": {
    message:
      "Der neue Name wurde gespeichert, konnte auf dieser Seite aber noch nicht angezeigt werden. Nutze „Erneut versuchen“ im Hinweis oder lade neu, bevor du ihn erneut bearbeitest.",
    description:
      "Why the service name row is locked after a rename saved but its re-read failed",
  },
  "services.displayNameError": {
    message:
      "Der Service konnte nicht umbenannt werden. Bitte versuche es erneut.",
    description: "Toast after setDisplayName fails",
  },
  "services.settingsInstanceType": {
    message: "Compute-Plan",
    description: "Settings tab row label for the App's current plan/tier",
  },
  "services.settingsNoInstanceType": {
    message: "Kein Compute-Plan festgelegt",
    description: "Settings tab state for an untiered (bare-CR) App",
  },
  "services.settingsUpdate": {
    message: "Ändern",
    description: "Settings tab link to the compute-plan picker",
  },
  "services.settingsIdleTimeout": {
    message: "Leerlauf-Timeout",
    description:
      "Settings tab: label for the free-tier auto-sleep window control",
  },
  "services.settingsIdleTimeoutHint": {
    message:
      "Kostenlose Services gehen in den Ruhezustand, wenn sie so lange keine öffentlichen HTTP-Anfragen oder WebSocket-Nachrichten erhalten, und wachen bei der nächsten öffentlichen Anfrage auf. Interne Anfragen anderer Services halten sie weder wach noch wecken sie auf.",
    description: "Settings tab: idle-timeout control help text (bex extension)",
  },
  "services.settingsIdleTimeoutPaid": {
    message:
      "Kostenpflichtige Services sind immer aktiv und gehen nie in den Ruhezustand.",
    description: "Settings tab: shown instead of the control on a paid plan",
  },
  "services.idleTimeoutDefault": {
    message: "Plattform-Standard",
    description: "Idle-timeout option: 0 seconds = the operator's own window",
  },
  "services.idleTimeoutMinutes": {
    message: "{minutes} Min.",
    description: "Idle-timeout option label in minutes",
  },
  "services.idleTimeoutHours": {
    message: "{hours} Std.",
    description: "Idle-timeout option label in hours",
  },
  "services.idleTimeoutSeconds": {
    message: "{seconds} Sek.",
    description: "Idle-timeout option label in seconds (non-round values)",
  },
  "services.idleTimeoutSuccess": {
    message: "Leerlauf-Timeout aktualisiert.",
    description: "Toast after setIdleTimeout succeeds",
  },
  "services.idleTimeoutError": {
    message: "Das Leerlauf-Timeout konnte nicht aktualisiert werden.",
    description: "Toast after setIdleTimeout fails",
  },
  "services.settingsMaxShutdownDelay": {
    message: "Max. Shutdown-Verzögerung",
    description: "Settings row label for the graceful SIGTERM window",
  },
  "services.settingsMaxShutdownDelayHint": {
    message:
      "Warte 1–300 Sekunden nach SIGTERM, bevor der Prozess zwangsweise beendet wird (Standard 30).",
    description: "Settings row help for the shutdown-delay range",
  },
  "services.maxShutdownDelayDiskReason": {
    message:
      "Die Shutdown-Verzögerung eines Service mit persistenter Disk kann nicht geändert werden.",
    description:
      "Settings hint replacing the range on a service with a disk (Render's rule, render.com/docs/disks); a link to the Disk tab follows",
  },
  "services.maxShutdownDelayDiskLink": {
    message: "Disk anzeigen",
    description:
      "Link after maxShutdownDelayDiskReason to the service's Disk tab",
  },
  "services.maxShutdownDelayEdit": {
    message: "Max. Shutdown-Verzögerung bearbeiten",
    description: "Accessible label for the shutdown-delay edit button",
  },
  "services.maxShutdownDelaySuccess": {
    message:
      "Max. Shutdown-Verzögerung aktualisiert. Ein neues Deployment wurde gestartet.",
    description: "Toast after setMaxShutdownDelay succeeds",
  },
  "services.maxShutdownDelayRangeError": {
    message: "Gib eine ganze Zahl von Sekunden zwischen {min} und {max} ein.",
    description:
      "Inline validation message for a max shutdown delay outside 1–300 or not a whole number (w4/494)",
  },
  "services.maxShutdownDelayError": {
    message: "Die max. Shutdown-Verzögerung konnte nicht aktualisiert werden.",
    description: "Toast after setMaxShutdownDelay fails",
  },
  "services.settingsHealthChecksTitle": {
    message: "Health Checks",
    description: "Settings tab: Health Checks section card title",
  },
  "services.settingsHealthChecksDescription": {
    message:
      "Konfiguriere den HTTP-Pfad, den bex regelmäßig abfragt, um deinen Service zu überwachen.",
    description: "Settings tab: Health Checks section card description",
  },
  "services.settingsHealthCheckPath": {
    message: "Health-Check-Pfad",
    description: "Settings tab: health-check path row label",
  },
  "services.settingsHealthCheckPathHint": {
    message:
      "Ein HTTP-Pfad, den bex abfragt, um zu entscheiden, ob dein Service fehlerfrei ist; er muss innerhalb von 5 Sekunden mit 2xx oder 3xx antworten, und 60 Sekunden aufeinanderfolgender Fehler starten die Instanz neu — verweise auf eine günstige Route. Lass das Feld leer, um nur zu prüfen, ob der Prozess an seinem Port lauscht — die richtige Wahl, wenn der Service keine günstige Route hat, die 2xx zurückgibt.",
    description: "Settings tab: health-check path row hint text",
  },
  "services.settingsHealthCheckPathPlaceholder": {
    message: "Leer — nur den Port prüfen",
    description: "Settings tab: health-check path input placeholder",
  },
  "services.settingsHealthCheckPathEdit": {
    message: "Health-Check-Pfad bearbeiten",
    description:
      "Settings tab: accessible label for the health-check path edit-pencil button",
  },
  "services.healthCheckPathSuccess": {
    message:
      "Health-Check-Pfad aktualisiert. Ein neues Deployment wurde gestartet.",
    description: "Toast after setHealthCheckPath succeeds",
  },
  "services.healthCheckPathError": {
    message: "Der Health-Check-Pfad konnte nicht aktualisiert werden.",
    description: "Toast after setHealthCheckPath fails",
  },
  "services.settingsPortTitle": {
    message: "Port",
    description: "Settings tab: Port section card title",
  },
  "services.settingsPortDescription": {
    message:
      "Der Port, auf dem dein Container lauscht. bex leitet Traffic dorthin und übergibt ihn als $PORT.",
    description: "Settings tab: Port section card description",
  },
  "services.settingsPort": {
    message: "Port",
    description: "Settings tab: service port row label",
  },
  "services.settingsPortHint": {
    message:
      "Der Port, auf dem dein Container lauscht (Standard {port}). bex leitet Traffic dorthin und übergibt ihn als $PORT. Eine Umgebungsvariable PORT am Service überschreibt ihn, wie bei Render. Durch das Speichern wird ein neues Deployment gestartet und die Instanzen werden neu gestartet.",
    description: "Settings tab: service port row hint text",
  },
  "services.settingsPortDetected": {
    message: "Erkannt: {port}",
    description:
      "Settings tab: port placeholder for a service whose serving port bex detected from the listener its process binds, while it names no port of its own (w8/338)",
  },
  "services.settingsPortDetectedHint": {
    message:
      "bex hat erkannt, dass dein Service auf Port {port} lauscht, und leitet Traffic dorthin. $PORT bleibt {defaultPort}, wie bei Render. Gib einen Port ein, um ihn stattdessen festzulegen: bex leitet dann Traffic dorthin und übergibt ihn als $PORT. Durch das Speichern wird ein neues Deployment gestartet und die Instanzen werden neu gestartet.",
    description:
      "Settings tab: service port row hint for a service serving on a detected port; PORT stays the default (w8/177, w8/267, w8/338)",
  },
  "services.settingsPortImageHint": {
    message:
      "Dieser Service übernimmt seine Ports aus den EXPOSE-Anweisungen seines Images. Ein hier festgelegter Port bestimmt den primären Listener — den Port, an den bex Traffic leitet, den es per Health Check prüft und als $PORT übergibt —, während die übrigen im Image deklarierten TCP-Ports veröffentlicht bleiben. Leere das Feld, um wieder den Port des Images zu verwenden. Durch das Speichern wird ein neues Deployment gestartet und die Instanzen werden neu gestartet.",
    description:
      "Settings tab: service port row hint for an image-port service (a Docker-built or prebuilt-image web or private service) whose ports come from its image's EXPOSE instructions; a blank port returns it to the image's (w6/m148, w4/m243, w4/m323)",
  },
  "services.settingsPortEdit": {
    message: "Port bearbeiten",
    description:
      "Settings tab: accessible label for the service port edit-pencil button",
  },
  "services.portRangeError": {
    message: "Gib einen Port zwischen {min} und {max} ein.",
    description:
      "Validation message for a port outside the range the container can bind",
  },
  "services.portSuccess": {
    message: "Port aktualisiert. Ein neues Deployment wird ausgerollt.",
    description: "Toast after setPort succeeds",
  },
  "services.portError": {
    message: "Der Port konnte nicht aktualisiert werden.",
    description: "Toast after setPort fails",
  },
  "services.settingsNotificationsTitle": {
    message: "Benachrichtigungen",
    description: "Settings tab: Notifications section card title",
  },
  "services.settingsNotificationsDescription": {
    message:
      "Wähle, welche Deployment-Benachrichtigungen du für diesen Service erhalten möchtest.",
    description: "Settings tab: Notifications section card description",
  },
  "services.notificationsLabel": {
    message: "Service-Benachrichtigungen",
    description: "Settings tab: service notification policy label",
  },
  "services.notificationsHint": {
    message:
      "Überschreibe den Workspace-Standard für diesen Service. Der Workspace-Standard wird unter Workspace-Einstellungen → Benachrichtigungen festgelegt.",
    description: "Settings tab: service notification policy explanation",
  },
  "services.notificationsEdit": {
    message: "Service-Benachrichtigungen bearbeiten",
    description: "Accessible label for the service-notifications edit button",
  },
  "services.notificationsOptionDefault": {
    message: "Workspace-Standard verwenden ({level})",
    description:
      "Service notification option: inherit workspace default; {level} is the workspace's saved level (e.g. Only failure notifications)",
  },
  "services.notificationsOptionDefaultUnknown": {
    message: "Workspace-Standard verwenden",
    description:
      "Service notification option: inherit workspace default, while the workspace level is loading or unreadable",
  },
  "services.notificationsOptionAll": {
    message: "Alle Benachrichtigungen",
    description: "Service notification option: all lifecycle mail",
  },
  "services.notificationsOptionFailure": {
    message: "Nur Fehlerbenachrichtigungen",
    description: "Service notification option: failures only",
  },
  "services.notificationsOptionNone": {
    message: "Keine",
    description: "Service notification option: no mail",
  },
  "services.notificationsSuccess": {
    message: "Service-Benachrichtigungseinstellung aktualisiert.",
    description: "Toast after notification policy update succeeds",
  },
  "services.notificationsError": {
    message:
      "Die Service-Benachrichtigungseinstellung konnte nicht aktualisiert werden.",
    description: "Toast after notification policy update fails",
  },
  "services.notifyOnFailLabel": {
    message: "Benachrichtigungen bei fehlgeschlagenem Deployment",
    description: "Settings tab: notifyOnFail row label",
  },
  "services.notifyOnFailHint": {
    message:
      "Standardmäßig gilt die persönliche Benachrichtigungseinstellung jedes Mitglieds; du kannst sie nur für diesen Service erzwingen oder abschalten.",
    description: "Settings tab: notifyOnFail row hint text",
  },
  "services.notifyOnFailOptionDefault": {
    message: "Einstellung des Mitglieds verwenden",
    description: "notifyOnFail select option: default",
  },
  "services.notifyOnFailOptionNotify": {
    message: "Immer benachrichtigen",
    description: "notifyOnFail select option: notify",
  },
  "services.notifyOnFailOptionIgnore": {
    message: "Nie benachrichtigen",
    description: "notifyOnFail select option: ignore",
  },
  "services.notifyOnFailSuccess": {
    message: "Benachrichtigungseinstellung aktualisiert.",
    description: "Toast after setNotifyOnFail succeeds",
  },
  "services.notifyOnFailError": {
    message:
      "Die Benachrichtigungseinstellung konnte nicht aktualisiert werden.",
    description: "Toast after setNotifyOnFail fails",
  },
  "services.planPickerTitle": {
    message: "Compute-Plan",
    description:
      "Accessible name of the Compute section's plan cards, shown after Edit (w6/m483)",
  },
  "services.computeTitle": {
    message: "Rechenleistung",
    description:
      "Compute page: title of the section holding the service's compute plan (Render: 'Under Compute, click Edit', w6/m483)",
  },
  "services.computePlanEdit": {
    message: "Bearbeiten",
    description:
      "Compute section button that opens the compute plan cards (w6/m483)",
  },
  "services.planPickerFreeGroup": {
    message: "Kostenlos",
    description:
      "Plan-picker section label separating the Free tier from paid tiers",
  },
  "services.planPickerFreeSmtp": {
    message:
      "Kostenlose Services können keine E-Mails über die SMTP-Ports 25, 465 oder 587 senden.",
    description:
      "Plan picker note under the Free group: free instances are refused outbound SMTP, as on Render (w8/m127)",
  },
  "services.planPickerPaidSmtp": {
    message:
      "Kostenpflichtige Services können E-Mails über Port 587 oder 2525 mit STARTTLS senden. Die Ports 25 und 465 sind in jedem Tarif nicht verfügbar.",
    description:
      "Plan picker note under the Paid group: outbound SMTP ports paid services may use (w8/m127)",
  },
  "services.planPickerLegacyFreeWorker": {
    message:
      "Dieser Background Worker nutzt Free, was Background Worker nicht mehr verwenden können. Er läuft weiter. Mit der Wahl eines kostenpflichtigen Compute-Plans wechselt er von Free weg und kann nicht zurück.",
    description:
      "Plan picker notice: this legacy service type is still on Free, which is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerUntieredWorker": {
    message:
      "Dieser Background Worker hat keinen Compute-Plan, und Background Worker können Free nicht mehr verwenden. Er läuft weiter. Wähle einen kostenpflichtigen Compute-Plan, um ihm einen zuzuweisen.",
    description:
      "Plan picker notice: this service type has no compute plan, and Free is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerLegacyFreePrivateService": {
    message:
      "Dieser Private Service nutzt Free, was Private Services nicht mehr verwenden können. Er läuft weiter. Mit der Wahl eines kostenpflichtigen Compute-Plans wechselt er von Free weg und kann nicht zurück.",
    description:
      "Plan picker notice: this legacy service type is still on Free, which is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerUntieredPrivateService": {
    message:
      "Dieser Private Service hat keinen Compute-Plan, und Private Services können Free nicht mehr verwenden. Er läuft weiter. Wähle einen kostenpflichtigen Compute-Plan, um ihm einen zuzuweisen.",
    description:
      "Plan picker notice: this service type has no compute plan, and Free is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerLegacyFreeCronJob": {
    message:
      "Dieser Cron Job nutzt Free, was Cron Jobs nicht mehr verwenden können. Er läuft weiter. Mit der Wahl eines kostenpflichtigen Compute-Plans wechselt er von Free weg und kann nicht zurück.",
    description:
      "Plan picker notice: this legacy service type is still on Free, which is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerUntieredCronJob": {
    message:
      "Dieser Cron Job hat keinen Compute-Plan, und Cron Jobs können Free nicht mehr verwenden. Er läuft weiter. Wähle einen kostenpflichtigen Compute-Plan, um ihm einen zuzuweisen.",
    description:
      "Plan picker notice: this service type has no compute plan, and Free is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerPaidGroup": {
    message: "Kostenpflichtig",
    description: "Plan-picker section label for the paid tier ladder",
  },
  "services.planCurrent": {
    message: "Aktuell",
    description:
      "Plan picker: badge on the instance type the service is on now",
  },
  "services.planPickerCancel": {
    message: "Abbrechen",
    description: "Plan-picker footer button: discard the selection",
  },
  "services.planPickerSave": {
    message: "Speichern",
    description:
      'Compute section footer button: confirm the plan change ("Select a new compute plan and click Save", render.com/docs/compute-plans)',
  },
  "services.planPickerConfirmTitle": {
    message: "Compute-Plan auf {name} ändern?",
    description: "Plan-change confirm dialog title",
  },
  "services.planPickerConfirmBody": {
    message:
      "Der Service wird ohne Ausfallzeit in der Größe angepasst und ausgerollt — laufende Anfragen werden beendet, bevor alte Instanzen ersetzt werden.",
    description: "Plan-change confirm dialog body",
  },
  "services.instanceTypePausedReason": {
    message:
      "„{name}“ ist vorübergehend nicht verfügbar; wähle {largest} oder einen kleineren Compute-Plan.",
    description:
      "A paused compute plan's tooltip and screen-reader reason, composed in the dashboard's language from the tier and the largest still offered; matches bex-api's English unavailableReason (w4/552)",
  },
  "services.planPickerConfirmBodyCron": {
    message:
      "Läufe, die nach der Änderung starten, nutzen den neuen Compute-Plan.",
    description:
      "Plan-change confirm body for a cron job, which serves no requests: its next run takes the new size (w4/548)",
  },
  "services.planPickerConfirmBodyNeverLive": {
    message:
      "Der Service läuft auf dem neuen Compute-Plan. Noch wird nichts ausgeliefert — er geht live, sobald ein Deployment erfolgreich ist.",
    description:
      "Plan-change confirm body for a service that has never gone live (w4/548)",
  },
  "services.planPickerConfirmRunPrice": {
    message:
      "Dieser Compute-Plan kostet ${price} pro Minute Laufzeit, sekundengenau abgerechnet, solange ein Lauf ausgeführt wird. Der Mindestbetrag liegt bei ${minimum} pro Monat und Cron Job.",
    description:
      "Plan-change confirm for a cron job: the new compute plan's run-time rate (w4/546); minimum is the cron job minimum in USD without the currency symbol (w9/m448)",
  },
  "services.createFieldPlanCronHint": {
    message:
      "Ein Cron Job wird sekundengenau abgerechnet, solange ein Lauf ausgeführt wird. Der Mindestbetrag liegt bei ${minimum} pro Monat und Cron Job.",
    description:
      "Under the create form's compute plans for a cron job, which are priced per minute of run time (w4/546); minimum is the cron job minimum in USD without the currency symbol (w9/m448)",
  },
  "services.planPickerConfirmPrice": {
    message: "Dieser Compute-Plan kostet ${price}/Monat.",
    description:
      "Plan-change confirm: the new compute plan's monthly price, from the API's price sheet (w4/400)",
  },
  "services.planPickerSuccess": {
    message: "Compute-Plan auf {name} aktualisiert",
    description: "Toast on a successful plan change",
  },
  "services.planPickerError": {
    message:
      "Der Compute-Plan konnte nicht aktualisiert werden. Bitte versuche es erneut.",
    description: "Toast on a failed plan change",
  },
  "services.planPickerErrorTitle": {
    message: "Compute-Pläne konnten nicht geladen werden",
    description: "Plan-picker error state title (instanceTypes query failed)",
  },
  "services.planPickerErrorBody": {
    message:
      "Die Anfrage an bex-api ist fehlgeschlagen. Prüfe deine Verbindung und versuche es erneut.",
    description: "Plan-picker error state body",
  },
  "services.domainsTitle": {
    message: "Benutzerdefinierte Domains",
    description: "Settings tab custom-domains section title",
  },
  "services.domainsDescription": {
    message: "Richte Domains, die dir gehören, auf diesen Service aus.",
    description: "Settings tab custom-domains section description",
  },
  "services.domainColName": {
    message: "Name",
    description: "Custom-domains table column header (the FQDN)",
  },
  "services.domainColVerified": {
    message: "Verifizierungsstatus",
    description: "Custom-domains table column header (DNS verification)",
  },
  "services.domainColCertificate": {
    message: "Zertifikatsstatus",
    description:
      "Custom-domains table column header (TLS certificate serving state)",
  },
  "services.domainColActions": {
    message: "Aktionen",
    description:
      "Custom-domains table actions column header (screen-reader only)",
  },
  "services.domainVerified": {
    message: "Verifiziert",
    description:
      "Custom-domains status badge: DNS or TLS verification is complete",
  },
  "services.domainCertActive": {
    message: "Aktiv",
    description:
      "Custom-domains status badge: certificate issued and serving traffic",
  },
  "services.domainCertBlockedLabel": {
    message: "Zertifikat blockiert:",
    description:
      "Label before cert-manager's verbatim reason a custom domain's TLS certificate has not been issued",
  },
  "services.domainCertBlockedHint": {
    message:
      "Der DNS-Eintrag muss „nur DNS“ sein (nicht über einen Proxy laufen) und auf den bex-Hostnamen deines Service zeigen, sonst kann die Zertifikatsprüfung ihn nicht erreichen.",
    description:
      "Fix-it guidance shown with a blocked-certificate reason: proxied records (e.g. Cloudflare's orange cloud) break the ACME HTTP-01 challenge",
  },
  "services.domainPending": {
    message: "Ausstehend",
    description:
      "Custom-domains status badge: DNS or certificate is not complete",
  },
  "services.domainActionsMenu": {
    message: "Aktionen für {domain}",
    description: "Accessible label for the per-domain actions trigger",
  },
  "services.domainDelete": {
    message: "Löschen",
    description: "Custom-domains row action: remove the domain",
  },
  "services.domainCancel": {
    message: "Abbrechen",
    description: "Custom-domains dialog cancel button",
  },
  "services.domainDeleteConfirmTitle": {
    message: "{name} löschen?",
    description: "Custom-domain delete-confirmation dialog title",
  },
  "services.domainDeleteConfirmBody": {
    message:
      "Der Service liefert diese Domain nicht mehr aus. Seine Ingress-Regel wird entfernt und das TLS-Zertifikat läuft aus. Das lässt sich nicht rückgängig machen.",
    description: "Custom-domain delete-confirmation dialog body",
  },
  "services.domainDeletePairConfirmBody": {
    message:
      "Dadurch werden {name} und die automatisch hinzugefügte Weiterleitung {sibling} entfernt. Beide Ingress-Regeln werden entfernt und ihre TLS-Zertifikate laufen aus. Das lässt sich nicht rückgängig machen.",
    description:
      "Custom-domain delete confirmation when deleting a canonical host also removes its generated redirecting sibling",
  },
  "services.domainDeleteGeneratedConfirmBody": {
    message:
      "Dadurch wird nur {name} entfernt. {canonical} bleibt die kanonische Domain und leitet nicht weiter. Die Ingress-Regel der entfernten Domain wird entfernt und ihr TLS-Zertifikat läuft aus. Das lässt sich nicht rückgängig machen.",
    description:
      "Custom-domain delete confirmation when deleting only the generated redirecting sibling",
  },
  "services.domainDeletePendingConfirmBody": {
    message:
      "Dadurch wird der ausstehende Anspruch auf {name} entfernt. Er wurde nie verifiziert, daher wurde darüber nichts ausgeliefert und kein Zertifikat ausgestellt.",
    description:
      "Custom-domain delete confirmation for a domain still unverified: it was never routed (w4/142)",
  },
  "services.domainDeletePendingPairConfirmBody": {
    message:
      "Dadurch werden die ausstehenden Ansprüche auf {name} und {sibling} entfernt. Keiner wurde verifiziert, daher wurde darüber nichts ausgeliefert und kein Zertifikat ausgestellt.",
    description:
      "Custom-domain delete confirmation for a still-pending domain and its auto-paired sibling (w4/142)",
  },
  "services.domainDeletePendingGeneratedConfirmBody": {
    message:
      "Dadurch wird nur der ausstehende Anspruch auf {name} entfernt. {canonical} bleibt die kanonische Domain. Er wurde nie verifiziert, daher wurde darüber nichts ausgeliefert und kein Zertifikat ausgestellt.",
    description:
      "Custom-domain delete confirmation for a still-pending generated sibling: only it is removed, its canonical claim stays (w4/539)",
  },
  "services.domainAdd": {
    message: "Benutzerdefinierte Domain hinzufügen",
    description: "Custom-domains button to open the add-domain dialog",
  },
  "services.domainAddTitle": {
    message: "Benutzerdefinierte Domain hinzufügen",
    description: "Add-domain dialog title",
  },
  "services.domainAddDescription": {
    message:
      "Gib eine Domain ein, die dir gehört. Richte ihr DNS auf diesen Service aus, und bex stellt automatisch ein TLS-Zertifikat aus.",
    description: "Add-domain dialog description",
  },
  "services.domainPlaceholder": {
    message: "www.example.com",
    description: "Add-domain FQDN input placeholder",
  },
  "services.domainInvalid": {
    message: "Gib eine gültige Domain ein, z. B. www.example.com.",
    description: "Add-domain validation message for a malformed hostname",
  },
  "services.domainAddButton": {
    message: "Domain hinzufügen",
    description: "Add-domain dialog submit button",
  },
  "services.domainAddPairSuccess": {
    message: "{name} und {sibling} hinzugefügt",
    description:
      "Toast on a custom-domain add that auto-paired its www/apex sibling (w4/618)",
  },
  "services.domainAddSuccess": {
    message: "{name} hinzugefügt",
    description: "Toast on a successful custom-domain add",
  },
  "services.domainAddError": {
    message: "{name} konnte nicht hinzugefügt werden",
    description: "Toast on a failed custom-domain add",
  },
  "services.domainAddConflict": {
    message: "{name} wird bereits von einem anderen Service verwendet",
    description:
      "Toast when a custom-domain add is rejected because the host is registered on another service (409)",
  },
  "services.domainAddReserved": {
    message: "{name} ist ein reservierter Plattform-Hostname",
    description:
      "Toast when a custom-domain add is rejected because the host is a platform-owned name (400)",
  },
  "services.domainAddOwnPlatformHost": {
    message:
      "{name} ist bereits die Plattform-Subdomain dieses Service und kann nicht als benutzerdefinierte Domain hinzugefügt werden",
    description:
      "Toast when a custom-domain add names the service's own platform subdomain (400 CUSTOM_DOMAIN_OWN_PLATFORM_HOST, w4/558)",
  },
  "services.domainDeletePairSuccess": {
    message: "{name} und {sibling} entfernt",
    description:
      "Toast on a custom-domain delete that also removed its auto-paired sibling (w4/618)",
  },
  "services.domainDeleteSuccess": {
    message: "{name} entfernt",
    description: "Toast on a successful custom-domain delete",
  },
  "services.domainDeleteError": {
    message: "{name} konnte nicht entfernt werden",
    description: "Toast on a failed custom-domain delete",
  },
  "services.domainPropagateNote": {
    message: "DNS und das TLS-Zertifikat werden im Hintergrund verteilt.",
    description:
      "Toast description after a custom-domain add (async convergence)",
  },
  "services.domainDnsToggle": {
    message: "DNS-Einrichtung für {domain} anzeigen",
    description:
      "aria-label for the per-domain DNS-instructions disclosure toggle",
  },
  "services.domainDnsTitle": {
    message: "DNS-Einrichtung",
    description: "Heading of the per-domain DNS-instructions panel",
  },
  "services.domainDnsSubdomainGuidance": {
    message:
      "Erstelle diesen Eintrag in der Zone {zone} bei deinem DNS-Anbieter. Der Host ist relativ zu dieser Zone. Verifizierung und TLS-Zertifikat erfolgen automatisch, sobald er aufgelöst wird.",
    description:
      "Guidance line above the DNS record for a subdomain — names the zone Host is relative to (w4/092)",
  },
  "services.domainDnsWildcardChallenge": {
    message:
      "Eine Wildcard-Domain braucht außerdem diesen Eintrag. Damit kann bex das TLS-Zertifikat der Domain ausstellen und erneuern.",
    description:
      "Above a wildcard custom domain's second DNS record, its _acme-challenge CNAME (w4/m326; Render's docs say it enables certificate issuance and renewal)",
  },
  "services.domainDnsApexFallback": {
    message:
      "Kein ALIAS/ANAME oder CNAME-Flattening? Erstelle stattdessen einen A-Eintrag für @ mit einer der unten aufgeführten aktuellen Edge-Adressen. Diese Adressen können sich ändern — ein ALIAS/ANAME-Eintrag funktioniert auch dann weiter.",
    description:
      "A-record fallback under an apex domain's DNS instructions, shown with the resolved edge addresses (w4/378)",
  },
  "services.domainDnsCaaHint": {
    message:
      "Hat deine Domain CAA-Einträge, füge Einträge für die Zertifizierungsstellen von bex hinzu: Let's Encrypt (letsencrypt.org) und Google Trust Services (pki.goog). Eine Wildcard-Domain braucht für jede zusätzlich einen issuewild-Eintrag.",
    description:
      "Under every custom domain's DNS records: the CAA values bex's two certificate authorities need, as Render's custom-domains page names its own (w21/m618; the bex-domains CLI skill says the same)",
  },
  "services.domainDnsApexGuidance": {
    message:
      "Apex-Domains können keinen einfachen CNAME verwenden — erstelle diesen ALIAS/ANAME-Eintrag (oder CNAME-Flattening-Eintrag) in der Zone {zone} (@ steht für den Zone-Apex). Wenn dein Anbieter keines von beidem unterstützt, erstelle einen A-Eintrag für @ mit den IP-Adressen, in die das Ziel aufgelöst wird. Verifizierung und TLS erfolgen automatisch, sobald er aufgelöst wird, und bex richtet die Weiterleitung www ↔ Apex für dich ein.",
    description:
      "Guidance line above the DNS record for an apex domain — names the zone Host is relative to (w4/092)",
  },
  "services.domainRecordType": {
    message: "Typ",
    description: "Label for the DNS record type field (CNAME/ALIAS)",
  },
  "services.domainRecordHost": {
    message: "Host",
    description: "Label for the DNS record host/name field",
  },
  "services.domainRecordTarget": {
    message: "Ziel",
    description: "Label for the DNS record target/value field",
  },
  "services.domainCopyHost": {
    message: "{type}-Host für {domain} kopieren",
    description: "Accessible label for copying a DNS host field",
  },
  "services.domainCopyTarget": {
    message: "{type}-Ziel für {domain} kopieren",
    description: "Accessible label for copying a DNS target/value field",
  },
  "services.domainDnsUnavailable": {
    message:
      "Das DNS-Ziel ist noch nicht verfügbar — prüfe erneut, sobald der Service läuft.",
    description: "Shown when the backend couldn't derive the DNS record target",
  },
  "services.domainRecheck": {
    message: "Erneut prüfen",
    description: "Button that re-checks a domain's DNS/certificate status",
  },
  "services.domainRecheckAria": {
    message: "{domain} erneut prüfen",
    description:
      "Accessible name of a custom domain's DNS-panel Re-check button, naming the domain (w4/540)",
  },
  "services.domainCopied": {
    message: "In die Zwischenablage kopiert",
    description: "Toast when a DNS record value is copied",
  },
  "services.domainCopyError": {
    message: "Kopieren in die Zwischenablage fehlgeschlagen",
    description: "Toast when copying a DNS record value fails",
  },
  "services.domainAddedTitle": {
    message: "Domain hinzugefügt — richte DNS ein",
    description: "Title of the post-add DNS-record step in the add dialog",
  },
  "services.domainAddedDescription": {
    message:
      "Erstelle den DNS-Eintrag bei deinem Anbieter; die Verifizierung wird automatisch abgeschlossen, sobald er aufgelöst wird.",
    description: "Subtitle of the post-add DNS-record step in the add dialog",
  },
  "services.domainRedirectsTo": {
    message: "Leitet weiter an {canonical}",
    description:
      "Note under an auto-paired sibling domain showing its canonical redirect target",
  },
  "services.domainRedirectsHere": {
    message: "{sibling} leitet hierher weiter",
    description:
      "Note under a canonical domain showing which auto-paired sibling redirects to it",
  },
  "services.domainPairedDnsTitle": {
    message:
      "{sibling} wurde automatisch hinzugefügt und leitet an {canonical} weiter — richte auch dafür DNS ein",
    description:
      "Heading of the second DNS-record block in the add dialog when the add auto-paired a www<->apex sibling (w6/m23)",
  },
  "services.domainDone": {
    message: "Fertig",
    description: "Button closing the post-add DNS-record step",
  },
  "services.domainVerifySuccess": {
    message: "{name} verifiziert",
    description: "Toast when a re-check finds the domain verified",
  },
  "services.domainVerifyPending": {
    message:
      "{name} ist noch nicht verifiziert — DNS wird möglicherweise noch verteilt.",
    description: "Toast when a re-check finds the domain still pending",
  },
  "services.domainVerifyError": {
    message: "{name} konnte nicht erneut geprüft werden.",
    description: "Toast when the re-check request fails",
  },
  "services.domainsEmptyTitle": {
    message: "Keine benutzerdefinierten Domains",
    description: "Custom-domains empty-state title",
  },
  "services.domainsEmptyBody": {
    message:
      "Füge eine Domain hinzu, die dir gehört, um diesen Service darüber auszuliefern.",
    description: "Custom-domains empty-state body",
  },
  "services.domainsErrorTitle": {
    message: "Benutzerdefinierte Domains konnten nicht geladen werden",
    description: "Custom-domains generic error title",
  },
  "services.domainsErrorBody": {
    message:
      "Die Anfrage an bex-api ist fehlgeschlagen. Prüfe deine Verbindung und versuche es erneut.",
    description: "Custom-domains generic error body",
  },
  "services.platformSubdomainTitle": {
    message: "Plattform-Subdomain",
    description: "Settings tab platform-subdomain section title",
  },
  "services.platformSubdomainDescription": {
    message:
      "Lege fest, ob dein Service zusätzlich zu den konfigurierten benutzerdefinierten Domains auch unter seiner bex-Plattform-Subdomain antwortet.",
    description: "Settings tab platform-subdomain section description",
  },
  "services.platformSubdomainEnabled": {
    message: "Aktiviert",
    description: "Platform-subdomain toggle label when the subdomain is active",
  },
  "services.platformSubdomainDisabled": {
    message: "Deaktiviert",
    description:
      "Platform-subdomain toggle label when the subdomain is disabled",
  },
  "services.platformSubdomainPending": {
    message: "Die Plattform-URL wird zugewiesen, sobald der Service läuft.",
    description: "Platform-subdomain state when the service has no URL yet",
  },
  "services.platformSubdomainDisabledNote": {
    message:
      "Die Plattform-Subdomain ist deaktiviert. Dein Service ist nur über benutzerdefinierte Domains erreichbar.",
    description:
      "Platform-subdomain note shown when the policy is set to disabled",
  },
  "services.platformSubdomainToggleLabel": {
    message: "Plattform-Subdomain umschalten",
    description: "Accessible label for the platform-subdomain Switch",
  },
  "services.platformSubdomainNeedsVerifiedDomain": {
    message:
      "Verifiziere eine benutzerdefinierte Domain, bevor du die Plattform-Subdomain abschaltest, damit der Service eine Adresse behält.",
    description:
      "Hint beside the platform-subdomain switch when no custom domain is verified yet (w4/142)",
  },
  "services.platformSubdomainNeedsVerifiedDomainPending": {
    message:
      "{names} muss die Verifizierung abschließen, bevor die Plattform-Subdomain abgeschaltet werden kann.",
    description:
      "Hint beside the platform-subdomain switch when custom domains exist but are all still pending verification (w4/142)",
  },
  "services.subdomainPolicySuccess": {
    message: "Einstellung der Plattform-Subdomain aktualisiert.",
    description: "Toast on successful setSubdomainPolicy mutation",
  },
  "services.subdomainPolicyError": {
    message:
      "Die Einstellung der Plattform-Subdomain konnte nicht aktualisiert werden. Versuche es erneut.",
    description: "Toast on failed setSubdomainPolicy mutation",
  },
  "services.maintenanceModeTitle": {
    message: "Wartungsmodus",
    description: "Settings tab maintenance-mode section title",
  },
  "services.maintenanceModeDescription": {
    message:
      "Nimm diesen Service hinter einer Wartungsseite offline, ohne ihn zu pausieren — seine Pods laufen weiter, nur der öffentliche Traffic wird umgeleitet.",
    description: "Settings tab maintenance-mode section description",
  },
  "services.maintenanceModeEnabled": {
    message: "Aktiviert",
    description: "Maintenance-mode toggle label when active",
  },
  "services.maintenanceModeDisabled": {
    message: "Deaktiviert",
    description: "Maintenance-mode toggle label when inactive",
  },
  "services.maintenanceModeSwitchHint": {
    message:
      "Jeder Host, den dieser Service bedient, antwortet mit der Wartungsseite.",
    description: "Maintenance-mode section hint text next to the switch",
  },
  "services.maintenanceModePaidOnly": {
    message:
      "Der Wartungsmodus ist in kostenpflichtigen Web-Service-Tarifen verfügbar.",
    description: "Maintenance mode free-plan eligibility note",
  },
  "services.maintenanceModeToggleLabel": {
    message: "Wartungsmodus umschalten",
    description: "Accessible label for the maintenance-mode Switch",
  },
  "services.maintenanceModeUriLabel": {
    message: "URL der benutzerdefinierten Wartungsseite",
    description: "Label for the maintenance-mode custom-page URL field",
  },
  "services.maintenanceModeUriPlaceholder": {
    message: "https://status.example.com/maintenance (optional)",
    description: "Placeholder for the maintenance-mode custom-page URL field",
  },
  "services.maintenanceModeUriHint": {
    message:
      "Wird abgerufen und anstelle der Standardseite ausgeliefert. Lass das Feld leer, um die Standard-Wartungsseite von bex zu verwenden. Darf nicht auf die eigene URL dieses Service zeigen.",
    description: "Hint text under the maintenance-mode custom-page URL field",
  },
  "services.maintenanceModeUriEdit": {
    message: "URL der Wartungsseite bearbeiten",
    description:
      "Accessible label for the maintenance-page URL edit-pencil button",
  },
  "services.maintenanceModeEnableAction": {
    message: "Wartungsmodus aktivieren",
    description: "Confirm-dialog action button for enabling maintenance mode",
  },
  "services.confirmMaintenanceModeTitle": {
    message: "Wartungsmodus aktivieren?",
    description: "Confirm-dialog title for enabling maintenance mode",
  },
  "services.confirmMaintenanceModeBody": {
    message:
      "{name} zeigt jedem Besucher eine Wartungsseite, bis du dies deaktivierst. Die Pods des Service laufen weiter.",
    description: "Confirm-dialog body for enabling maintenance mode",
  },
  "services.maintenanceModeEnabledSuccess": {
    message: "Wartungsmodus aktiviert.",
    description: "Toast on successfully enabling maintenance mode",
  },
  "services.maintenanceModeDisabledSuccess": {
    message: "Wartungsmodus deaktiviert.",
    description: "Toast on successfully disabling maintenance mode",
  },
  "services.maintenancePageSavedSuccess": {
    message: "Wartungsseite gespeichert.",
    description: "Toast on saving only the custom maintenance page URL",
  },
  "services.maintenancePageClearedSuccess": {
    message: "Standard-Wartungsseite wiederhergestellt.",
    description: "Toast on clearing the custom maintenance page URL",
  },
  "services.maintenanceModeError": {
    message:
      "Der Wartungsmodus konnte nicht aktualisiert werden. Versuche es erneut.",
    description: "Toast on failed setMaintenanceMode mutation",
  },
  "services.edgeCachingTitle": {
    message: "Edge-Caching",
    description: "Settings tab edge-caching section title (Render parity)",
  },
  "services.edgeCachingDescription": {
    message:
      "Cache Antworten am Edge, damit wiederholte Anfragen ausgeliefert werden, ohne diesen Service zu erreichen.",
    description: "Settings tab edge-caching section description",
  },
  "services.edgeCachingFileTypesLabel": {
    message: "Cachebare Dateitypen",
    description: "Edge-caching radio group label",
  },
  "services.edgeCachingOptionNone": {
    message: "Keine",
    description: "Edge-caching option: no-cache profile",
  },
  "services.edgeCachingOptionNoneDesc": {
    message: "Jede Antwort wird von deinem Service ausgeliefert.",
    description: "Edge-caching no-cache option description",
  },
  "services.edgeCachingOptionStatic": {
    message: "Gängige statische Dateien",
    description: "Edge-caching option: origin-controlled profile",
  },
  "services.edgeCachingOptionStaticDesc": {
    message:
      "Cache gängige statische Dateitypen wie CSS, JavaScript, Bilder und Schriftarten, sofern der Cache-Control-Header der Antwort nichts anderes vorgibt.",
    description: "Edge-caching origin-controlled option description",
  },
  "services.edgeCachingOptionAll": {
    message: "Alle Dateien",
    description: "Edge-caching option: origin-controlled-all profile",
  },
  "services.edgeCachingOptionAllDesc": {
    message:
      "Cache jede geeignete Antwort, sofern ihr Cache-Control-Header nichts anderes vorgibt. Stelle sicher, dass dynamische Inhalte Cache-Control setzen, damit sie nicht gecacht werden.",
    description:
      "Edge-caching origin-controlled-all option description, with the dynamic-content warning",
  },
  "services.edgeCachingPurgeOnDeploy": {
    message: "Der Cache wird bei jedem erfolgreichen Deployment geleert.",
    description: "Note under the edge-caching options",
  },
  "services.edgeCachingPaidOnly": {
    message: "Edge-Caching ist für kostenlose Web Services nicht verfügbar.",
    description: "Edge-caching free-plan eligibility note",
  },
  "services.edgeCachingSave": {
    message: "Edge-Caching speichern",
    description: "Edge-caching save button",
  },
  "services.edgeCachingSaving": {
    message: "Speichern…",
    description: "Edge-caching save button while the mutation is in flight",
  },
  "services.edgeCachingSaved": {
    message: "Edge-Caching aktualisiert.",
    description: "Toast on successful setCacheProfile mutation",
  },
  "services.edgeCachingError": {
    message:
      "Edge-Caching konnte nicht aktualisiert werden. Versuche es erneut.",
    description: "Toast on failed setCacheProfile mutation",
  },
  "services.edgeCachingPurgeAction": {
    message: "Cache leeren",
    description: "Edge-caching purge button and confirm action",
  },
  "services.edgeCachingPurgeConfirmTitle": {
    message: "Edge-Cache leeren?",
    description: "Confirm-dialog title for purging the edge cache",
  },
  "services.edgeCachingPurgeConfirmBody": {
    message:
      "Jede gecachte Antwort wird verworfen. Bis der Cache wieder aufgewärmt ist, gehen Anfragen an deinen Service.",
    description: "Confirm-dialog body for purging the edge cache",
  },
  "services.edgeCachingPurged": {
    message: "Cache-Leerung gestartet.",
    description: "Toast on successful purgeServiceCache mutation",
  },
  "services.edgeCachingPurgeError": {
    message: "Der Cache konnte nicht geleert werden. Versuche es erneut.",
    description: "Toast on failed purgeServiceCache mutation",
  },
  "services.runtimeTitle": {
    message: "Runtime",
    description: "Settings tab runtime section title (Render parity)",
  },
  "services.runtimeDescription": {
    message: "Die Umgebung, in der bex diesen Service baut und ausführt.",
    description: "Settings tab runtime section description",
  },
  "services.runtimeLabel": {
    message: "Runtime",
    description: "Settings runtime select label",
  },
  "services.runtimeNote": {
    message:
      "Beim Ändern der Runtime wird der Service neu gebaut und neu deployt. Native Runtimes verwenden den Build-Befehl und Startbefehl unten.",
    description: "Note beside the Settings runtime select",
  },
  "services.runtimePlaceholder": {
    message: "Runtime auswählen",
    description:
      "Settings runtime select placeholder for a legacy service with no runtime recorded",
  },
  "services.runtimeSave": {
    message: "Runtime speichern",
    description: "Settings runtime save button",
  },
  "services.runtimeSaving": {
    message: "Speichern…",
    description: "Settings runtime save button while the mutation is in flight",
  },
  "services.runtimeConfirmTitle": {
    message: "Runtime auf {runtime} ändern?",
    description: "Confirm-dialog title for changing a service's runtime",
  },
  "services.runtimeConfirmBody": {
    message:
      "Der Service wird mit der Runtime {runtime} neu gebaut und neu deployt. Dadurch wird ein neues Deployment gestartet.",
    description: "Confirm-dialog body for changing a service's runtime",
  },
  "services.runtimeConfirmAction": {
    message: "Runtime ändern",
    description: "Confirm-dialog action for changing a service's runtime",
  },
  "services.runtimeSaved": {
    message: "Runtime aktualisiert. Ein neues Deployment wurde gestartet.",
    description: "Toast on successful setRuntime mutation",
  },
  "services.runtimeError": {
    message: "Die Runtime konnte nicht geändert werden. Versuche es erneut.",
    description: "Toast on failed setRuntime mutation without a server message",
  },
  "services.maintenanceModeBannerTitle": {
    message: "Wartungsmodus ist aktiv",
    description: "Service-detail header banner title while in maintenance",
  },
  "services.maintenanceModeBannerBody": {
    message:
      "Besucher sehen eine Wartungsseite statt dieses Service. Die Pods laufen weiter — deaktiviere den Wartungsmodus in den Einstellungen, um wieder normal auszuliefern.",
    description: "Service-detail header banner body while in maintenance",
  },
  "services.deployTitle": {
    message: "Deployment",
    description: "Cron job Settings tab: Deploy section title (Render parity)",
  },
  "services.deployDescription": {
    message: "Wie dieser Cron Job läuft.",
    description: "Cron job Settings tab: Deploy section description",
  },
  "services.deploySuccess": {
    message: "Cron-Job-Einstellungen gespeichert.",
    description: "Toast after updateCronJob succeeds",
  },
  "services.cronScheduleSaved": {
    message: "Der nächste Lauf folgt dem neuen Zeitplan.",
    description:
      "Toast description after saving a cron job's schedule alone: the next run follows it, nothing deploys (w4/593)",
  },
  "services.cronCommandSaved": {
    message:
      "Ein Deployment wendet den neuen Befehl an; spätere Läufe verwenden ihn.",
    description:
      "Toast description after saving a cron job's command: the save rolls a deploy that applies it (w4/593)",
  },
  "services.deployError": {
    message:
      "Cron-Job-Einstellungen konnten nicht gespeichert werden. Bitte versuche es erneut.",
    description: "Toast after updateCronJob fails",
  },
  "services.deployRefreshFailed": {
    message:
      "Cron-Job-Einstellungen gespeichert, aber diese Seite konnte nicht aktualisiert werden. Versuche es erneut oder lade neu, um die gespeicherten Einstellungen zu sehen.",
    description:
      "Warning after a cron save committed but its authoritative re-read failed; the save must not be retried or reported as failed (w4/276)",
  },
  "services.deployRefreshPending": {
    message:
      "Einstellungen gespeichert — warte auf einen frischen Lesevorgang, bevor du weiter bearbeitest.",
    description:
      "Reason shown while cron rows stay unwritable after a committed save whose refresh failed (w4/276)",
  },
  "services.deployScheduleLabel": {
    message: "Zeitplan",
    description: "Cron job Settings tab: Deploy section schedule field label",
  },
  "services.deployScheduleHint": {
    message:
      "Führt diesen Befehl nach diesem Zeitplan aus (Crontab mit 5 Feldern, in UTC).",
    description: "Cron job Settings tab: Deploy section schedule help text",
  },
  "services.deploySchedulePreview": {
    message: "{description} · läuft in UTC",
    description:
      "Cron job Settings tab: live human-readable description of the schedule",
  },
  "services.deploySchedulePlaceholder": {
    message: "0 * * * *",
    description: "Cron job Deploy section: schedule input placeholder",
  },
  "services.deployScheduleEdit": {
    message: "Zeitplan bearbeiten",
    description: "Accessible label for the cron schedule edit button",
  },
  "services.deployCommandEdit": {
    message: "Befehl bearbeiten",
    description: "Accessible label for the cron command edit button",
  },
  "services.cronScheduleNeverFires": {
    message:
      "Dieser Zeitplan wird nie ausgelöst: Kein Datum passt zu seinem Tag des Monats und seinem Monat (z. B. ist 31 2 der 31. Februar).",
    description:
      "Cron schedule validation error when the schedule parses but no date ever matches (w4/197)",
  },
  "services.deployScheduleError": {
    message:
      "Gib einen gültigen Cron-Ausdruck mit 5 Feldern ein, z. B. 0 * * * *.",
    description: "Cron job Deploy section: schedule validation error",
  },
  "services.deployScheduleRequired": {
    message: "Der Zeitplan ist erforderlich.",
    description: "Cron job Deploy section: schedule required validation error",
  },
  "services.deployCommandLabel": {
    message: "Befehl",
    description: "Cron job Settings tab: Deploy section command field label",
  },
  "services.deployCommandPlaceholder": {
    message: "z. B. python script.py",
    description: "Cron job Deploy section: command input placeholder",
  },
  "services.deployCommandHint": {
    message:
      "Überschreibt den Standard-Einstiegspunkt des Images. Lass das Feld leer, um den eigenen Befehl des Images auszuführen.",
    description: "Cron job Deploy section: command field help text",
  },
  "services.deployCommandNativeHint": {
    message:
      "Der Befehl, den der Job bei jedem Zeitplan ausführt, als Startbefehl seiner Runtime.",
    description:
      "Cron job Deploy section: command field help text for a native-runtime cron, whose command is its start command (w8/m69)",
  },
  "services.deployCommandRequired": {
    message: "Der Befehl ist erforderlich.",
    description:
      "Cron job Deploy section: command required validation error for a native-runtime cron (w8/m69)",
  },
  "services.buildTitle": {
    message: "Build",
    description:
      "Settings tab: Build card title, Render's split Build section (w5/m52)",
  },
  "services.buildDeployNav": {
    message: "Build & Deployment",
    description:
      "Settings section index: the one entry for the Build and Deploy cards, which the page renders as one section (Render names it Build & Deploy)",
  },
  "services.buildDescription": {
    message: "Konfiguriere, wie bex deinen Service aus deinem Repository baut.",
    description: "Settings tab: Build card description",
  },
  "services.deploySectionTitle": {
    message: "Deployment",
    description:
      "Settings tab: Deploy card title, Render's split Deploy section (w5/m52)",
  },
  "services.deploySectionDescription": {
    message: "Konfiguriere, wie bex deinen Service deployt und ausführt.",
    description: "Settings tab: Deploy card description",
  },
  "services.buildDeploySourceLabel": {
    message: "Quelle",
    description: "Build & Deploy: repo field label",
  },
  "services.buildDeploySourceHint": {
    message:
      "Das Git-Repository, aus dem dieser Service gebaut wird. Wähle eines deiner verbundenen Repositorys oder füge eine Repository-URL ein.",
    description: "Build & Deploy: Source (repo) field help text (w5/m54)",
  },
  "services.buildDeploySourcePlaceholder": {
    message: "https://github.com/owner/repo",
    description: "Build & Deploy: Source combobox input placeholder",
  },
  "services.buildDeploySourceEdit": {
    message: "Quelle bearbeiten",
    description: "Accessible label for the Source edit-pencil button",
  },
  "services.buildDeploySourceConfirmTitle": {
    message: "Quelle auf {value} wechseln?",
    description: "Build & Deploy: Source-change confirm dialog title",
  },
  "services.buildDeployBranchLabel": {
    message: "Branch",
    description: "Build & Deploy: branch field label",
  },
  "services.buildDeployBranchHint": {
    message: "Der Git-Branch, der gebaut und deployt wird.",
    description: "Build & Deploy: branch field hint (Render's copy)",
  },
  "services.buildDeployBranchEmpty": {
    message: "main",
    description:
      "Build & Deploy: branch value shown when unset (the backend default)",
  },
  "services.buildDeployBranchPlaceholder": {
    message: "main",
    description: "Build & Deploy: branch input placeholder",
  },
  "services.buildDeployBranchEdit": {
    message: "Branch bearbeiten",
    description: "Build & Deploy: branch edit-button aria label",
  },
  "services.buildDeployBranchConfirmTitle": {
    message: "Branch auf „{value}“ ändern?",
    description: "Build & Deploy: branch confirm-dialog title",
  },
  "services.buildDeployRootDirLabel": {
    message: "Stammverzeichnis",
    description: "Build & Deploy: root-directory field label",
  },
  "services.buildDeployRootDirOptional": {
    message: "Optional",
    description:
      "Build & Deploy: badge next to the Root Directory label (Render parity)",
  },
  "services.buildDeployRootDirHint": {
    message:
      "Wenn festgelegt, laufen Builds aus diesem Unterverzeichnis statt aus dem Repository-Stamm. Codeänderungen außerhalb davon lösen kein Auto-Deployment aus, außer sie passen zu einem eingeschlossenen Pfad der Build-Filter. Wird meist bei einem Monorepo verwendet.",
    description: "Build & Deploy: root-directory field help text",
  },
  "services.buildDeployConfirmRoot": {
    message: "den Repository-Stamm",
    description:
      "Build & Deploy: mid-sentence phrase for the confirm dialog title when clearing rootDir to empty (a dedicated key, not a lowercased buildDeployRootDirEmpty, since that transform doesn't hold in every language)",
  },
  "services.buildDeployRootDirPlaceholder": {
    message: "z. B. backend",
    description: "Build & Deploy: root-directory input placeholder",
  },
  "services.buildDeployEdit": {
    message: "Stammverzeichnis bearbeiten",
    description: "Build & Deploy: accessible label for the edit-pencil button",
  },
  "services.buildDeployCancel": {
    message: "Abbrechen",
    description: "Build & Deploy: root-directory inline-edit cancel button",
  },
  "services.buildDeployConfirmTitle": {
    message: "Stammverzeichnis auf {value} ändern?",
    description: "Build & Deploy: root-directory change confirm dialog title",
  },
  "services.buildDeployConfirmBody": {
    message:
      "Der Service wird neu gebaut und neu deployt, auf das neue Verzeichnis beschränkt — laufende Anfragen werden beendet, bevor alte Instanzen ersetzt werden.",
    description: "Build & Deploy: root-directory change confirm dialog body",
  },
  "services.buildDeployConfirmBodyStatic": {
    message:
      "Dadurch startet ein Neu-Build aus dem neuen Verzeichnis. Deine Site liefert weiterhin ihre aktuell veröffentlichte Version aus, bis der neue Build erfolgreich veröffentlicht wurde.",
    description:
      "Build & Deploy: a live static site's root-directory change confirm body (atomic publish, no instances; w4/548)",
  },
  "services.buildDeployConfirmBodyNeverLive": {
    message:
      "Der Service wird aus dem neuen Verzeichnis gebaut und deployt. Noch wird nichts ausgeliefert — er geht live, sobald ein Deployment erfolgreich ist.",
    description:
      "Build & Deploy: root-directory change confirm body for a service that has never gone live, so no current version keeps serving (w4/548)",
  },
  "services.buildDeploySuccess": {
    message: "Stammverzeichnis aktualisiert.",
    description: "Toast after setRootDir succeeds",
  },
  "services.buildDeployError": {
    message:
      "Das Stammverzeichnis konnte nicht aktualisiert werden. Bitte versuche es erneut.",
    description: "Toast after setRootDir fails",
  },
  "services.sourceUpdateSuccess": {
    message: "Quelle aktualisiert.",
    description:
      "Toast after a repository, branch, or image source update saves without triggering a deploy (e.g. suspended service or unbuilt build source)",
  },
  "services.sourceUpdateDeploying": {
    message: "Quelle aktualisiert. Neues Deployment wird gestartet.",
    description:
      "Toast after Update Source saves and immediately triggers a new deploy",
  },
  "services.sourceUpdateError": {
    message:
      "Die Quelle konnte nicht aktualisiert werden. Bitte versuche es erneut.",
    description:
      "Fallback toast when a source update fails without a server reason",
  },
  "services.sourceTitle": {
    message: "Quelle",
    description: "Source card title",
  },
  "services.sourceRepoDescription": {
    message:
      "Das Git-Repository und der Branch, aus denen dieser Service gebaut wird.",
    description: "Source card description for a repository-backed service",
  },
  "services.sourceImageDescription": {
    message: "Das vorgefertigte Container-Image, das dieser Service deployt.",
    description: "Source card description for an image-backed service",
  },
  "services.sourceImageLabel": {
    message: "Image",
    description: "Source card: container image path field label",
  },
  "services.sourceDialogDescription": {
    message:
      "Wähle ein Git-Repository, eine vorhandene Build-Quelle oder ein Docker-Image. Beim Absenden startet ein neues Deployment.",
    description:
      "Update Source dialog description for non-static services (Render parity)",
  },
  "services.sourceDialogDescriptionStatic": {
    message:
      "Wähle ein Git-Repository für diese Static Site. Beim Absenden startet ein neues Deployment.",
    description:
      "Update Source dialog description for static sites (Render parity)",
  },
  "services.sourceMissing": {
    message: "Nicht konfiguriert",
    description: "Source card fallback when no source value is available",
  },
  "services.sourceRepoAccessLostTitle": {
    message: "Die GitHub-App von bex kann dieses Repository nicht lesen",
    description:
      "Settings → Source warning title: the GitHub App connected this service's repository and can no longer read it (repoAccessLost, w10/m490)",
  },
  "services.sourceRepoAccessLostBody": {
    message:
      "Pushes deployen diesen Service nicht mehr. Wurde das Repository in ein anderes GitHub-Konto verschoben, installiere die GitHub-App in diesem Konto; andernfalls gib der App wieder Zugriff auf das Repository.",
    description:
      "Settings → Source warning body: pushes stop reaching bex once the GitHub App loses the repository (moved to an account without the app, or the grant was revoked); what to do (w10/m490)",
  },
  "services.sourceEdit": {
    message: "Bearbeiten",
    description: "Button opening the Update Source dialog",
  },
  "services.sourceDialogTitle": {
    message: "Quelle aktualisieren",
    description: "Update Source dialog title",
  },
  "services.sourcePickerLabel": {
    message: "Quelle auswählen",
    description: "Label above the source kind picker in Update Source",
  },
  "services.sourceBranchHint": {
    message: "Wähle einen Branch oder gib einen ein, der nicht aufgeführt ist.",
    description: "Branch picker free-text fallback hint",
  },
  "services.sourceBranchEmpty": {
    message: "Keine passenden Branches. Gib einen Branch-Namen ein.",
    description: "Empty state for the Update Source branch picker",
  },
  "services.sourceTabBuildSource": {
    message: "Build-Quelle",
    description: "Update Source tab: build the service from a build source",
  },
  "services.sourceBuildSourceDescription": {
    message:
      "Die Build-Quelle, die dieser Service deployt. Ihr Build wird mit jedem damit verknüpften Service geteilt.",
    description:
      "Source card description for a service linked to a build source",
  },
  "services.sourceBuildSourceLabel": {
    message: "Build-Quelle",
    description: "Source card and Update Source: build source field label",
  },
  "services.sourceBuildSourceBuildsFrom": {
    message: "Gebaut aus",
    description:
      "Source card: the repository or image the linked build source builds from",
  },
  "services.sourceBuildSourceEmpty": {
    message:
      "Dieser Workspace hat noch keine Build-Quellen. Erstelle eine über die API oder MCP und verknüpfe hier Services damit.",
    description: "Update Source build source tab: empty state",
  },
  "services.sourceBuildSourceLoadError": {
    message:
      "Die Build-Quellen dieses Workspace konnten nicht geladen werden. Schließe den Dialog und versuche es erneut.",
    description: "Update Source build source tab: the list failed to load",
  },
  "services.sourceBuildSourcePlaceholder": {
    message: "Build-Quelle auswählen",
    description: "Update Source build source picker placeholder",
  },
  "services.sourceBuildSourceStartHint": {
    message:
      "Diese Build-Quelle verwendet eine native Runtime, daher braucht der Service einen eigenen Startbefehl.",
    description:
      "Update Source: why a native-runtime build source needs a start command",
  },
  "services.sourceBuildCommandOptionalHint": {
    message:
      "Optional. Leer lassen, um die Dateien des Repositorys direkt zu veröffentlichen.",
    description:
      "Update Source static-site Build Command helper: build command is optional",
  },
  "services.sourcePublishDirectoryRequired": {
    message: "Gib ein Veröffentlichungsverzeichnis ein.",
    description:
      "Update Source static-site Publish Directory validation error when blank",
  },
  "services.sourceDeploy": {
    message: "Deployen",
    description:
      "Primary submit button in Update Source for Git and Build Source tabs",
  },
  "services.sourceDeploying": {
    message: "Wird deployt",
    description:
      "In-flight submit button label in Update Source for Git and Build Source tabs",
  },
  "services.sourceConnect": {
    message: "Verbinden",
    description:
      "Primary submit button in Update Source for the Existing Image tab",
  },
  "services.sourceConnecting": {
    message: "Wird verbunden",
    description:
      "In-flight submit button label in Update Source for the Existing Image tab",
  },
  "services.startCommandLabel": {
    message: "Startbefehl",
    description: "Build & Deploy: native-runtime start-command label",
  },
  "services.dockerCommandLabel": {
    message: "Docker-Befehl",
    description: "Build & Deploy: Docker CMD override label (Render wording)",
  },
  "services.startCommandHint": {
    message:
      "Der Befehl, der diesen Service nach einem erfolgreichen Build startet.",
    description: "Build & Deploy: native start-command help text",
  },
  "services.dockerCommandHint": {
    message:
      "Überschreibt den CMD im Dockerfile. Lass das Feld leer, um den Standardbefehl des Images zu verwenden.",
    description: "Build & Deploy: Docker Command help text",
  },
  "services.startCommandConfirmEmpty": {
    message: "den Standardbefehl",
    description: "Build & Deploy: empty command phrase in confirmation title",
  },
  "services.startCommandPlaceholder": {
    message: "z. B. npm start",
    description: "Build & Deploy: start-command input placeholder",
  },
  "services.startCommandEdit": {
    message: "Startbefehl bearbeiten",
    description: "Build & Deploy: accessible command edit button label",
  },
  "services.dockerCommandEdit": {
    message: "Docker-Befehl bearbeiten",
    description: "Build & Deploy: accessible Docker Command edit button label",
  },
  "services.dockerCommandPlaceholder": {
    message: "z. B. bin/server",
    description: "Build & Deploy: Docker Command input placeholder",
  },
  "services.startCommandConfirmTitle": {
    message: "Startbefehl auf {value} ändern?",
    description: "Build & Deploy: command-change confirmation title",
  },
  "services.dockerCommandConfirmTitle": {
    message: "Docker-Befehl auf {value} ändern?",
    description: "Build & Deploy: Docker Command confirmation title",
  },
  "services.startCommandConfirmBody": {
    message:
      "Der Service wird mit dem neuen Befehl neu deployt. Laufende Anfragen werden beendet, bevor alte Instanzen ersetzt werden.",
    description: "Build & Deploy: command-change confirmation body",
  },
  "services.startCommandConfirmBodyNeverLive": {
    message:
      "Der Service wird mit dem neuen Befehl deployt. Noch wird nichts ausgeliefert — er geht live, sobald ein Deployment erfolgreich ist.",
    description:
      "Deploy: command change confirm body for a service that has never gone live (w4/548)",
  },
  "services.startCommandSuccess": {
    message: "Startbefehl aktualisiert.",
    description: "Toast after setStartCommand succeeds",
  },
  "services.startCommandError": {
    message:
      "Der Startbefehl konnte nicht aktualisiert werden. Bitte versuche es erneut.",
    description: "Toast after setStartCommand fails",
  },
  "services.dockerCommandSuccess": {
    message: "Docker-Befehl aktualisiert.",
    description:
      "Toast after a Dockerfile or Existing Image service's Docker Command is saved",
  },
  "services.dockerCommandError": {
    message:
      "Der Docker-Befehl konnte nicht aktualisiert werden. Bitte versuche es erneut.",
    description:
      "Toast when saving a Dockerfile or Existing Image service's Docker Command fails",
  },
  "services.buildCommandLabel": {
    message: "Build-Befehl",
    description:
      "Build & Deploy: build-command field label (all service types' settings + the New Service form)",
  },
  "services.buildCommandHint": {
    message:
      "Der Befehl, der deinen Service baut (z. B. npm run build). Lass das Feld leer, um den Standard der Runtime zu verwenden.",
    description:
      "Build & Deploy: build-command help text — shown for every service type, so it must not assume static-site output",
  },
  "services.buildCommandConfirmEmpty": {
    message: "Runtime-Standard",
    description:
      "Build & Deploy: empty build-command phrase in confirmation title",
  },
  "services.buildCommandPlaceholder": {
    message: "npm run build",
    description: "Build & Deploy: build-command input placeholder",
  },
  "services.buildCommandEdit": {
    message: "Build-Befehl bearbeiten",
    description: "Build & Deploy: accessible build-command edit button label",
  },
  "services.buildCommandConfirmTitle": {
    message: "Build-Befehl auf {value} ändern?",
    description: "Build & Deploy: build-command change confirmation title",
  },
  "services.buildCommandConfirmBody": {
    message:
      "Der Service wird mit dem neuen Build-Befehl neu deployt. Bestehende Anfragen werden abgearbeitet, bevor alte Instanzen ersetzt werden.",
    description: "Build & Deploy: build-command change confirmation body",
  },
  "services.buildCommandConfirmBodyNeverLive": {
    message:
      "Der Service wird mit dem neuen Build-Befehl gebaut und deployt. Noch wird nichts ausgeliefert — er geht live, sobald ein Deployment erfolgreich ist.",
    description:
      "Build & Deploy: build-command change confirm body for a service that has never gone live (w4/548)",
  },
  "services.buildCommandConfirmBodyStatic": {
    message:
      "Dadurch startet ein Neu-Build mit dem neuen Build-Befehl. Deine Site liefert weiterhin ihre aktuell veröffentlichte Version aus, bis der neue Build erfolgreich veröffentlicht wurde.",
    description:
      "Build & Deploy: static-site build-command change confirmation body (atomic publish, no instances)",
  },
  "services.staticBuildConfirmBodyNeverLive": {
    message:
      "Dadurch startet ein Build mit der Änderung. Deine Site ist noch nicht veröffentlicht — sie geht live, sobald dieser Build erfolgreich ist.",
    description:
      "Build & Deploy: a static site's build-command or root-directory change confirm body when the site has never been published (w4/548)",
  },
  "services.buildCommandSuccess": {
    message: "Build-Befehl aktualisiert.",
    description: "Toast after setBuildCommand succeeds",
  },
  "services.buildCommandError": {
    message:
      "Der Build-Befehl konnte nicht aktualisiert werden. Bitte versuche es erneut.",
    description: "Toast after setBuildCommand fails",
  },
  "services.dockerfilePathLabel": {
    message: "Dockerfile-Pfad",
    description: "Build & Deploy: Dockerfile-path field label",
  },
  "services.dockerfilePathHint": {
    message:
      "Pfad zum Dockerfile, relativ zum Stammverzeichnis. Lass das Feld leer, um Dockerfile zu verwenden.",
    description: "Build & Deploy: Dockerfile-path help text",
  },
  "services.dockerfilePathConfirmEmpty": {
    message: "das Standard-Dockerfile",
    description: "Build & Deploy: empty Dockerfile-path confirmation phrase",
  },
  "services.dockerfilePathPlaceholder": {
    message: "z. B. docker/Dockerfile.prod",
    description: "Build & Deploy: Dockerfile-path input placeholder",
  },
  "services.dockerfilePathEdit": {
    message: "Dockerfile-Pfad bearbeiten",
    description: "Build & Deploy: accessible Dockerfile-path edit label",
  },
  "services.dockerfilePathConfirmTitle": {
    message: "Dockerfile-Pfad auf {value} ändern?",
    description: "Build & Deploy: Dockerfile-path confirmation title",
  },
  "services.dockerfilePathConfirmBody": {
    message:
      "Der Service wird aus dem ausgewählten Dockerfile neu gebaut und das resultierende Image deployt.",
    description: "Build & Deploy: Dockerfile-path confirmation body",
  },
  "services.dockerfilePathSuccess": {
    message: "Dockerfile-Pfad aktualisiert.",
    description: "Toast after setDockerfilePath succeeds",
  },
  "services.dockerfilePathError": {
    message:
      "Der Dockerfile-Pfad konnte nicht aktualisiert werden. Bitte versuche es erneut.",
    description: "Toast after setDockerfilePath fails",
  },
  "services.dockerContextLabel": {
    message: "Docker-Build-Kontextverzeichnis",
    description: "Build & Deploy: Docker build context field label (w8/m128)",
  },
  "services.dockerContextHint": {
    message:
      "Build-Kontext, relativ zum Stammverzeichnis. Lass das Feld leer, um das Stammverzeichnis zu verwenden.",
    description: "Build & Deploy: Docker build context help text",
  },
  "services.dockerContextConfirmEmpty": {
    message: "das Stammverzeichnis",
    description:
      "Build & Deploy: empty Docker build context confirmation phrase",
  },
  "services.dockerContextPlaceholder": {
    message: "z. B. .",
    description: "Build & Deploy: Docker build context input placeholder",
  },
  "services.dockerContextEdit": {
    message: "Docker-Build-Kontextverzeichnis bearbeiten",
    description: "Build & Deploy: accessible Docker build context edit label",
  },
  "services.dockerContextConfirmTitle": {
    message: "Docker-Build-Kontextverzeichnis auf {value} ändern?",
    description: "Build & Deploy: Docker build context confirmation title",
  },
  "services.dockerContextConfirmBody": {
    message:
      "Der Service wird mit dem neuen Build-Kontext neu gebaut und das resultierende Image deployt.",
    description: "Build & Deploy: Docker build context confirmation body",
  },
  "services.dockerContextSuccess": {
    message: "Docker-Build-Kontextverzeichnis aktualisiert.",
    description: "Toast after setDockerContext succeeds",
  },
  "services.dockerContextError": {
    message:
      "Das Docker-Build-Kontextverzeichnis konnte nicht aktualisiert werden. Bitte versuche es erneut.",
    description: "Toast after setDockerContext fails",
  },
  "services.buildFilterLabel": {
    message: "Build-Filter",
    description: "Build & Deploy: label for the build-filters editor",
  },
  "services.buildFilterHint": {
    message:
      "Löse nur dann ein Deployment aus, wenn ein Git-Push passende Dateien ändert. Pfade sind Glob-Muster relativ zum Repository-Stamm, auch wenn ein Stammverzeichnis festgelegt ist (z. B. src/**, **/*.md).",
    description: "Build & Deploy: help text for the build-filters editor",
  },
  "services.buildFilterIncludedTitle": {
    message: "Eingeschlossene Pfade",
    description: "Build & Deploy: title for the included-paths list",
  },
  "services.buildFilterIncludedHint": {
    message:
      "Ein Push deployt nur, wenn eine geänderte Datei zu einem dieser Muster passt, auch außerhalb des Stammverzeichnisses. Leer bedeutet, dass jeder Pfad im Stammverzeichnis eingeschlossen ist.",
    description: "Build & Deploy: help text for the included-paths list",
  },
  "services.buildFilterIncludedPlaceholder": {
    message: "z. B. src/**",
    description: "Build & Deploy: placeholder for an included-path input",
  },
  "services.buildFilterAddIncluded": {
    message: "Eingeschlossenen Pfad hinzufügen",
    description: "Build & Deploy: add-row button for the included-paths list",
  },
  "services.buildFilterRemoveIncluded": {
    message: "Eingeschlossenen Pfad entfernen",
    description: "Build & Deploy: remove-row label for an included path",
  },
  "services.buildFilterIgnoredTitle": {
    message: "Ignorierte Pfade",
    description: "Build & Deploy: title for the ignored-paths list",
  },
  "services.buildFilterIgnoredHint": {
    message:
      "Eine geänderte Datei, die zu einem dieser Muster passt, löst nie ein Deployment aus, auch wenn sie zugleich zu einem eingeschlossenen Pfad passt.",
    description: "Build & Deploy: help text for the ignored-paths list",
  },
  "services.buildFilterIgnoredPlaceholder": {
    message: "z. B. docs/**",
    description: "Build & Deploy: placeholder for an ignored-path input",
  },
  "services.buildFilterAddIgnored": {
    message: "Ignorierten Pfad hinzufügen",
    description: "Build & Deploy: add-row button for the ignored-paths list",
  },
  "services.buildFilterRemoveIgnored": {
    message: "Ignorierten Pfad entfernen",
    description: "Build & Deploy: remove-row label for an ignored path",
  },
  "services.buildPathTooLong": {
    message: "Höchstens 512 Bytes",
    description:
      "Inline error under a Root Directory, Dockerfile Path, Docker Build Context or Build Filter pattern over the server's 512-byte limit; a Chinese character is 3 bytes (w21/045)",
  },
  "services.buildPathControlChar": {
    message: "Darf kein Steuerzeichen und keinen Tabulator enthalten",
    description:
      "Inline error under a build path or Build Filter pattern with a control character (w21/045)",
  },
  "services.buildPathBackslash": {
    message: "Verwende / als Pfadtrenner, nicht \\",
    description:
      "Inline error under a build path or Build Filter pattern with a backslash (w21/045)",
  },
  "services.buildPathAbsolute": {
    message: "Muss relativ zum Repository sein und darf nicht mit / beginnen",
    description:
      "Inline error under a build path or Build Filter pattern that starts with / (w21/045)",
  },
  "services.buildPathDotDot": {
    message: "Muss im Repository bleiben: kein .. Segment",
    description:
      "Inline error under a build path or Build Filter pattern with a .. segment (w21/045)",
  },
  "services.buildPathEmptySegment": {
    message:
      "Darf kein leeres Segment enthalten: kein // und kein abschließendes /",
    description:
      "Inline error under a Dockerfile Path or Docker Build Context with an empty path segment; the Root Directory normalizes these away (w21/045)",
  },
  "services.buildFilterUnclosedClass": {
    message: "Eine [ wird nie durch ein ] geschlossen",
    description:
      "Inline error under a Build Filter glob with an unclosed character class (w21/045)",
  },
  "services.healthCheckPathSlash": {
    message: "Muss mit / beginnen oder für eine reine Port-Prüfung leer sein",
    description:
      "Inline error under a Health Check Path that does not start with / (w21/045)",
  },
  "services.buildFilterSave": {
    message: "Build-Filter speichern",
    description: "Build & Deploy: save button for the build-filters editor",
  },
  "services.buildFilterSuccess": {
    message: "Build-Filter aktualisiert.",
    description: "Toast after setBuildFilter succeeds",
  },
  "services.buildFilterError": {
    message:
      "Die Build-Filter konnten nicht aktualisiert werden. Bitte versuche es erneut.",
    description: "Toast after setBuildFilter fails",
  },
  "services.preDeployLabel": {
    message: "Pre-Deploy-Befehl",
    description: "Build & Deploy: label for the pre-deploy command field",
  },
  "services.preDeployHint": {
    message:
      "Wird einmal mit dem neuen Image ausgeführt, bevor es Traffic bedient (z. B. eine Datenbankmigration). Ein Exit-Code ungleich null lässt das Deployment fehlschlagen und die vorherige Version live.",
    description: "Build & Deploy: help text for the pre-deploy command field",
  },
  "services.preDeployPaidOnly": {
    message: "Pre-Deploy-Befehle sind in kostenpflichtigen Tarifen verfügbar.",
    description:
      "Build & Deploy: why the pre-deploy command field is disabled on a Free service (Render: paid web services, private services and background workers; w8/m131)",
  },
  "services.preDeployPlaceholder": {
    message: "z. B. npm run migrate",
    description: "Build & Deploy: placeholder for the pre-deploy command input",
  },
  "services.preDeployEdit": {
    message: "Pre-Deploy-Befehl bearbeiten",
    description:
      "Build & Deploy: accessible label for the pre-deploy edit-pencil button",
  },
  "services.healthCheckPathConfirmTitle": {
    message: "Health-Check-Pfad auf {value} ändern?",
    description:
      "Settings: Health Check Path change confirmation title (w4/616)",
  },
  "services.healthCheckPathConfirmEmpty": {
    message: "eine TCP-Prüfung",
    description:
      "Settings: empty Health Check Path phrase in its confirmation title",
  },
  "services.maxShutdownDelayConfirmTitle": {
    message: "Max. Shutdown-Verzögerung auf {value} Sekunden ändern?",
    description:
      "Settings: Max shutdown delay change confirmation title (w4/616)",
  },
  "services.preDeployConfirmTitle": {
    message: "Pre-Deploy-Befehl auf {value} ändern?",
    description:
      "Settings: Pre-Deploy Command change confirmation title (w4/616)",
  },
  "services.preDeployConfirmEmpty": {
    message: "keinen Pre-Deploy-Befehl",
    description:
      "Settings: empty Pre-Deploy Command phrase in its confirmation title",
  },
  "services.settingRedeployConfirmBody": {
    message:
      "Der Service wird mit der neuen Einstellung neu deployt. Laufende Anfragen werden beendet, bevor alte Instanzen ersetzt werden.",
    description:
      "Settings: confirmation body for a setting whose save starts a deploy (w4/616)",
  },
  "services.settingRedeployConfirmBodyNeverLive": {
    message:
      "Der Service wird mit der neuen Einstellung deployt. Noch wird nichts ausgeliefert — er geht live, sobald ein Deployment erfolgreich ist.",
    description:
      "Settings: deploying-setting confirmation body for a service that has never gone live (w4/616)",
  },
  "services.preDeployConfirmBody": {
    message:
      "Der Service wird neu deployt, und der neue Befehl läuft, bevor das neue Release live geht. Laufende Anfragen werden beendet, bevor alte Instanzen ersetzt werden.",
    description:
      "Settings: Pre-Deploy Command change confirmation body (w4/616)",
  },
  "services.preDeployConfirmBodyNeverLive": {
    message:
      "Der Service wird deployt, und der neue Befehl läuft, bevor das Release live geht. Noch wird nichts ausgeliefert — er geht live, sobald ein Deployment erfolgreich ist.",
    description:
      "Settings: Pre-Deploy Command confirmation body for a service that has never gone live (w4/616)",
  },
  "services.preDeploySuccess": {
    message:
      "Pre-Deploy-Befehl aktualisiert. Ein neues Deployment wurde gestartet.",
    description: "Toast after setPreDeployCommand succeeds",
  },
  "services.preDeployError": {
    message:
      "Der Pre-Deploy-Befehl konnte nicht aktualisiert werden. Bitte versuche es erneut.",
    description: "Toast after setPreDeployCommand fails",
  },
  "services.autoDeployLabel": {
    message: "Auto-Deploy",
    description: "Build & Deploy: label for the auto-deploy toggle",
  },
  "services.autoDeployOnCommit": {
    message: "Bei Commit",
    description:
      "Auto-Deploy select option: redeploy on every matching git push (Render's autoDeployTrigger 'commit', w5/m53)",
  },
  "services.autoDeployAfterChecks": {
    message: "Nach bestandenen CI-Prüfungen",
    description:
      "Auto-Deploy select option: redeploy a pushed commit once all its GitHub checks pass (Render's autoDeployTrigger 'checksPass', w1/m181)",
  },
  "services.autoDeployOff": {
    message: "Aus",
    description:
      "Auto-Deploy select option: never redeploy on push (Render's autoDeployTrigger 'off')",
  },
  "services.autoDeployEdit": {
    message: "Auto-Deploy bearbeiten",
    description:
      "Accessible label for the Auto-Deploy select's edit-pencil button",
  },
  "services.autoDeployViaGitHub": {
    message:
      "Ein Push auf den verfolgten Branch deployt automatisch über die GitHub-App.",
    description:
      "Build & Deploy: source indicator when the connected GitHub App installation grants THIS repo (pushDeliveryMethod github_app)",
  },
  "services.autoDeployViaWebhook": {
    message:
      "Ein Push deployt nur, wenn der manuelle Git-Webhook des Repositorys mit dem BEX_WEBHOOK_SECRET dieser Installation konfiguriert ist.",
    description:
      "Build & Deploy: source indicator when the GitHub App cannot deliver a push for this repo — no connection, another git host, or a repo the installation does not grant (pushDeliveryMethod manual_webhook)",
  },
  "services.autoDeployUnavailable": {
    message:
      "Auto-Deploy kann für dieses Repository nicht laufen: GitHub sendet bex seine Pushes nicht. Verbinde das Repository über die GitHub-App oder löse Deployments mit dem Deploy Hook aus.",
    description:
      "Build & Deploy: source indicator when no push can reach bex for this repo — the GitHub App does not cover it and this install rejects the shared manual webhook key (pushDeliveryMethod unavailable, w4/483)",
  },
  "services.autoDeployDeliveryUnknown": {
    message:
      "Es wird geprüft, wie ein Push auf den verfolgten Branch bex erreicht… Wenn das anhält, ist GitHub nicht erreichbar und die Zustellmethode kann nicht bestätigt werden.",
    description:
      "Build & Deploy: source indicator while the per-repo push-delivery check is loading or returned unknown (GitHub unreachable) — states the uncertainty instead of naming a mechanism (w6/m99)",
  },
  "services.autoDeployOnSuccess": {
    message: "Auto-Deploy eingeschaltet.",
    description: "Toast after enabling auto-deploy",
  },
  "services.autoDeployChecksSuccess": {
    message: "Auto-Deploy wartet jetzt auf bestandene CI-Prüfungen.",
    description: "Toast after choosing After CI Checks Pass",
  },
  "services.autoDeployOffSuccess": {
    message: "Auto-Deploy ausgeschaltet.",
    description: "Toast after disabling auto-deploy",
  },
  "services.autoDeployError": {
    message:
      "Auto-Deploy konnte nicht geändert werden. Bitte versuche es erneut.",
    description: "Toast after setAutoDeploy fails",
  },
  "services.deployHookTitle": {
    message: "Deploy Hook",
    description: "Settings tab: secret Deploy Hook section title",
  },
  "services.deployHookDescription": {
    message: "Löse ein Deployment aus CI mit einer einzigen geheimen URL aus.",
    description: "Settings tab: Deploy Hook section description",
  },
  "services.deployHookURLLabel": {
    message: "Deploy-Hook-URL",
    description: "Accessible label for the secret Deploy Hook URL field",
  },
  "services.deployHookReveal": {
    message: "Deploy-Hook-URL anzeigen",
    description: "Accessible label for the reveal-secret button",
  },
  "services.deployHookHide": {
    message: "Deploy-Hook-URL ausblenden",
    description: "Accessible label for the hide-secret button",
  },
  "services.deployHookCopy": {
    message: "Deploy-Hook-URL kopieren",
    description: "Accessible label for the Deploy Hook copy button",
  },
  "services.deployHookCopied": {
    message: "Deploy-Hook-URL kopiert.",
    description: "Toast after copying the Deploy Hook URL",
  },
  "services.deployHookCopyError": {
    message: "Die Deploy-Hook-URL konnte nicht kopiert werden.",
    description: "Toast after Deploy Hook URL clipboard failure",
  },
  "services.deployHookSecretHint": {
    message:
      "Halte diese URL geheim. Jeder, der sie hat, kann diesen Service ohne API-Schlüssel deployen.",
    description: "Security warning below the Deploy Hook URL",
  },
  "services.deployHookRegenerate": {
    message: "Hook neu generieren",
    description: "Deploy Hook rotation button",
  },
  "services.deployHookRegenerateTitle": {
    message: "Deploy Hook neu generieren?",
    description: "Deploy Hook rotation confirmation title",
  },
  "services.deployHookRegenerateWarning": {
    message:
      "Die aktuelle URL funktioniert sofort nicht mehr. Aktualisiere jedes CI-System, jeden Cron Job und jede Integration, die sie verwendet.",
    description: "Deploy Hook rotation confirmation warning",
  },
  "services.deployHookCancel": {
    message: "Abbrechen",
    description: "Deploy Hook rotation confirmation cancel button",
  },
  "services.deployHookRegenerateConfirm": {
    message: "Neu generieren",
    description: "Deploy Hook rotation confirmation action",
  },
  "services.deployHookRegenerated": {
    message: "Deploy Hook neu generiert. Die alte URL funktioniert nicht mehr.",
    description: "Toast after successful Deploy Hook rotation",
  },
  "services.deployHookRegenerateError": {
    message:
      "Der Deploy Hook konnte nicht neu generiert werden. Bitte versuche es erneut.",
    description: "Toast after Deploy Hook rotation fails",
  },
  "services.deployHookLoadError": {
    message: "Die Deploy-Hook-URL konnte nicht geladen werden.",
    description: "Deploy Hook section query error",
  },
  "services.colType": {
    message: "Typ",
    description: "Services table column header (service type)",
  },
  "services.typeWeb": {
    message: "Web Service",
    description: "Service-type badge: an HTTP service exposed at a URL",
  },
  "services.typePrivate": {
    message: "Private Service",
    description:
      "Service-type badge: an HTTP service reachable only in-cluster",
  },
  "services.typeWorker": {
    message: "Background Worker",
    description: "Service-type badge: runs with no HTTP port/URL",
  },
  "services.typeCron": {
    message: "Cron Job",
    description: "Service-type badge: runs a command on a schedule",
  },
  "services.typeStatic": {
    message: "Static Site",
    description:
      "Service-type badge: built output served from the object-store origin",
  },
  "services.staticTitle": {
    message: "Static Site",
    description:
      "Settings section title for a static site's publish dir + edge rules",
  },
  "services.staticDescription": {
    message:
      "Das veröffentlichte Ausgabeverzeichnis und die Edge-Regeln, die bei der Auslieferung angewendet werden.",
    description: "Static Site settings section description",
  },
  "services.staticEdgeRulesHint": {
    message: "Redirects/Rewrites und Antwort-Header haben eigene Seiten:",
    description:
      "Settings hint pointing at the dedicated Redirects/Rewrites + Headers pages",
  },
  "services.staticCancel": {
    message: "Abbrechen",
    description: "Cancel an inline edit",
  },
  "services.publishPathEscapes": {
    message:
      "Dieser Pfad verlässt das Repository. Ein Veröffentlichungsverzeichnis wird vom Stammverzeichnis aus aufgelöst, entferne daher die .. Segmente.",
    description:
      "Inline error under a repo-backed static site's publish directory whose .. segments leave the checkout (w21/050)",
  },
  "services.publishPathControlChar": {
    message: "Darf kein Steuerzeichen und keinen Zeilenumbruch enthalten",
    description:
      "Inline error under a static site's publish directory with a control character (w21/050)",
  },
  "services.publishPathLabel": {
    message: "Veröffentlichungsverzeichnis",
    description: "Label for a static site's publishPath",
  },
  "services.publishPathPlaceholder": {
    message: "dist",
    description: "Placeholder for the publish directory input",
  },
  "services.publishPathEdit": {
    message: "Veröffentlichungsverzeichnis bearbeiten",
    description:
      "Accessible label for the publish-directory edit-pencil button",
  },
  "services.publishPathHint": {
    message:
      "Das Build-Ausgabeverzeichnis, das als Stamm der Site ausgeliefert wird (z. B. dist, build, public). Beim Ändern wird die Site neu veröffentlicht.",
    description: "Help text under the publish directory field",
  },
  "services.publishPathSaved": {
    message: "Veröffentlichungsverzeichnis aktualisiert",
    description: "Toast after saving a static site's publishPath",
  },
  "services.publishPathRepublishNote": {
    message: "Die Site wird in Kürze neu veröffentlicht.",
    description: "Toast description after changing publishPath",
  },
  "services.publishPathError": {
    message:
      "Das Veröffentlichungsverzeichnis konnte nicht aktualisiert werden",
    description: "Error toast for a failed publishPath change",
  },
  "services.routesTitle": {
    message: "Redirects & Rewrites",
    description: "Title for the static-site routes editor",
  },
  "services.routesHint": {
    message:
      "Wird der Reihe nach abgeglichen, der erste Treffer gewinnt. Ein Redirect gibt 301 zurück; ein Rewrite liefert einen anderen Pfad aus (ein SPA-Fallback schreibt /* auf /index.html um). Ein :name-Segment erfasst ein Pfadsegment, z. B. /blog/posts/:postid → /blog/:postid.",
    description: "Help text for the routes editor",
  },
  "services.routeAdd": {
    message: "Regel hinzufügen",
    description: "Add a route rule",
  },
  "services.routeType": {
    message: "Typ",
    description: "Route type column",
  },
  "services.routeSource": {
    message: "Quelle",
    description: "Route source-path column",
  },
  "services.routeDestination": {
    message: "Ziel",
    description: "Route destination-path column",
  },
  "services.routeRewrite": {
    message: "Rewrite",
    description: "Route type: rewrite",
  },
  "services.routeRedirect": {
    message: "Redirect",
    description: "Route type: redirect",
  },
  "services.routeTypeAria": {
    message: "Typ von Regel {n}",
    description:
      "A route rule's type select (aria-label), named by its row as Remove rule {n} is (w4/541)",
  },
  "services.routeSourceAria": {
    message: "Quelle von Regel {n}",
    description:
      "A route rule's source field (aria-label), named by its row (w4/541)",
  },
  "services.routeDestinationAria": {
    message: "Ziel von Regel {n}",
    description:
      "A route rule's destination field (aria-label), named by its row (w4/541)",
  },
  "services.routeRemove": {
    message: "Regel {n} entfernen",
    description: "Remove a route rule (aria-label)",
  },
  "services.routesSave": {
    message: "Routen speichern",
    description: "Save the routes list",
  },
  "services.staticRuleRequired": {
    message: "Erforderlich",
    description: "Inline error under an empty static-site rule field (w4/145)",
  },
  "services.staticRulePathSlash": {
    message: "Muss ein Pfad sein, der mit / beginnt",
    description:
      "Inline error under a static-site rule path that doesn't start with / (w4/145)",
  },
  "services.staticRuleExternalHost": {
    message:
      "Verwende eine einfache http(s)-URL mit festem Host, z. B. https://example.org/new",
    description:
      "Inline error under a redirect destination that is an absolute URL without a literal host (w8/162)",
  },
  "services.staticRuleLocalPath": {
    message: "Muss ein Pfad auf dieser Site sein, nicht //host",
    description:
      "Inline error under a redirect/rewrite destination that is a network-path reference (w4/145)",
  },
  "services.staticRuleDestinationPathOrUrl": {
    message: "Muss ein Pfad oder eine http(s)-URL sein",
    description:
      "Inline error under a redirect/rewrite destination whose first segment reads as a scheme other than http(s), such as javascript: (w33/007)",
  },
  "services.staticRuleFixRows": {
    message: "Korrigiere die markierten Zeilen, um zu speichern.",
    description:
      "Note beside a disabled Save while a static-site rule row is invalid (w4/145)",
  },
  "services.staticRuleEmptyRow": {
    message: "Fülle die leere Zeile aus oder entferne sie, um zu speichern.",
    description:
      "Note beside a disabled Save while a static-site rule row is still completely empty (w4/145)",
  },
  "services.staticRuleRow": {
    message: "Zeile {row}:",
    description:
      "Prefix naming the 1-based rule row in a server refusal, replacing its zero-based wire index (w4/145)",
  },
  "services.staticRulePathTooLong": {
    message: "Höchstens 2048 Bytes",
    description:
      "Inline error under an over-long static-site rule path; the server counts UTF-8 bytes, so a Chinese character is 3 (w4/402, w21/043)",
  },
  "services.staticRuleSourceRepeatsPlaceholder": {
    message: "Jeder :name darf in der Quelle nur einmal vorkommen",
    description:
      "Inline error under a redirect/rewrite source that declares the same :name placeholder twice (w21/043)",
  },
  "services.staticRuleUndeclaredPlaceholder": {
    message: "Verwendet einen :name, den die Quelle nicht deklariert",
    description:
      "Inline error under a redirect/rewrite destination whose :name placeholder (other than :splat) the source never declares (w21/043)",
  },
  "services.staticRuleHeaderNameTooLong": {
    message: "Höchstens 256 Zeichen",
    description: "Inline error under an over-long header name (w4/402)",
  },
  "services.staticRuleHeaderNameInvalid": {
    message: "Kein gültiger HTTP-Header-Name",
    description:
      "Inline error under a header name with characters a header name can't hold (w4/402)",
  },
  "services.staticRuleHeaderNameReserved": {
    message:
      "Dieser Header wird vom Server gesetzt und kann nicht überschrieben werden",
    description:
      "Inline error under a reserved header name such as Content-Length (w4/402)",
  },
  "services.staticRuleHeaderValueTooLong": {
    message: "Höchstens 4096 Bytes",
    description: "Inline error under an over-long header value (w4/402)",
  },
  "services.staticRuleHeaderValueInvalid": {
    message: "Enthält ein Zeichen, das ein Header-Wert nicht enthalten kann",
    description:
      "Inline error under a header value with a control character (w4/402)",
  },
  "services.staticRuleCookieDomain": {
    message: "Set-Cookie kann bei geteiltem Hosting keine Domain setzen",
    description:
      "Inline error under a Set-Cookie value carrying a Domain attribute (w4/402)",
  },
  "services.staticRoutesSaved": {
    message: "Routen aktualisiert",
    description: "Toast after saving routes",
  },
  "services.staticRoutesError": {
    message: "Die Routen konnten nicht aktualisiert werden",
    description: "Error toast for a failed routes save",
  },
  "services.headersTitle": {
    message: "Antwort-Header",
    description: "Title for the static-site custom-headers editor",
  },
  "services.headersHint": {
    message:
      "Benutzerdefinierte Antwort-Header, die Antworten hinzugefügt werden, deren Pfad passt. Header zur Body-Rahmung und Verbindung wie Content-Length werden vom Server gesetzt und können nicht überschrieben werden. Setze Content-Encoding nur für Dateien, die tatsächlich komprimiert gespeichert sind.",
    description: "Help text for the headers editor",
  },
  "services.headerAdd": {
    message: "Header hinzufügen",
    description: "Add a custom header",
  },
  "services.headerPath": {
    message: "Pfad",
    description: "Header path column",
  },
  "services.headerName": {
    message: "Name",
    description: "Header name column",
  },
  "services.headerValue": {
    message: "Wert",
    description: "Header value column",
  },
  "services.headerPathAria": {
    message: "Pfad von Header {n}",
    description:
      "A header rule's path field (aria-label), named by its row as Remove header {n} is (w4/541)",
  },
  "services.headerNameAria": {
    message: "Name von Header {n}",
    description:
      "A header rule's name field (aria-label), named by its row (w4/541)",
  },
  "services.headerValueAria": {
    message: "Wert von Header {n}",
    description:
      "A header rule's value field (aria-label), named by its row (w4/541)",
  },
  "services.headerRemove": {
    message: "Header {n} entfernen",
    description: "Remove a header (aria-label)",
  },
  "services.headersSave": {
    message: "Header speichern",
    description: "Save the headers list",
  },
  "services.staticHeadersSaved": {
    message: "Header aktualisiert",
    description: "Toast after saving headers",
  },
  "services.staticHeadersError": {
    message: "Die Header konnten nicht aktualisiert werden",
    description: "Error toast for a failed headers save",
  },
  "services.typeUnknown": {
    message: "Service",
    description: "Service-type badge fallback for an unrecognized type",
  },
  "services.cronRunColDuration": {
    message: "Dauer",
    description:
      "Cron Runs page: the Duration column's header, and a row's duration for assistive tech (Render's DURATION, w3/m556)",
  },
  "services.cronRunStatusSucceeded": {
    message: "Lauf erfolgreich",
    description:
      "Cron Runs page: a successful run's status, in its icon's tooltip (Render's text, w3/m556)",
  },
  "services.cronRunStatusFailed": {
    message: "Lauf fehlgeschlagen",
    description:
      "Cron Runs page: a failed run's status, in its icon's tooltip (Render's text, w3/m556)",
  },
  "services.cronRunStatusCanceled": {
    message: "Abgebrochen",
    description:
      "Cron Runs page: a canceled run's status in its icon's tooltip, and its line when no one is named as canceling it (Render's text, w3/m556)",
  },
  "services.cronRunStatusInProgress": {
    message: "Läuft",
    description:
      "Cron Runs page: the status of a run that has not ended, started or not, in its icon's tooltip (Render's text, w3/m556)",
  },
  "services.cronRunStatusLastSuccessful": {
    message: "Letzter erfolgreicher Lauf",
    description:
      "Cron Runs page: the newest successful run's status, in its icon's tooltip (Render's text, w3/m556)",
  },
  "services.cronRunCancel": {
    message: "Abbrechen",
    description:
      "Cron Runs page: a run in progress's Cancel button, which cancels it at once (Render's text, w3/m556)",
  },
  "services.cronRunCancelError": {
    message: "Der Cron-Lauf konnte nicht abgebrochen werden.",
    description: "Toast after cron run cancellation fails",
  },
  "services.cronTriggerRun": {
    message: "Lauf auslösen",
    description: "Button that fires an immediate cron run (Render parity)",
  },
  "services.cronTriggerError": {
    message:
      "Der Lauf konnte nicht ausgelöst werden. Versuch es gleich noch einmal.",
    description:
      "Toast, kept until dismissed, when Trigger Run fails and the server gave no message (Render's text, w3/m555)",
  },
  "services.cronTriggerForbidden": {
    message: "Du hast keine Berechtigung, diesen Cron Job auszuführen.",
    description:
      "Toast, kept until dismissed, when Trigger Run is refused for permission (Render's text, w3/m555)",
  },
  "services.cronTriggerNoBuild": {
    message: "Kein erfolgreicher Build gefunden. Sieh dir {deploys} an.",
    description:
      "Tooltip of a disabled Trigger Run on a repo-backed cron job whose build never succeeded; {deploys} is a link to its Deploys page, named by services.navDeploys (Render's text, w3/m555)",
  },
  "services.cronTriggerSuspended": {
    message: "Setze diesen Cron Job fort, um einen Lauf auszulösen.",
    description:
      "Tooltip of a disabled Trigger Run while the cron job is suspended, shown in the Runs page's empty states (Render's text, w3/m556)",
  },
  "services.cronRunsLoadMore": {
    message: "Mehr laden",
    description: "Cron run history pagination button",
  },
  "services.cronRunsLoadingMore": {
    message: "Wird geladen…",
    description: "Cron run history pagination busy label",
  },
  "services.cronRunsLoadError": {
    message: "Cron-Läufe konnten nicht geladen werden.",
    description: "Cron run history read error",
  },
  "services.cronRunsLoadFailed": {
    message:
      "Leider konnten die Läufe dieses Cron Jobs nicht geladen werden. Lade die Seite neu und versuch es noch einmal.",
    description:
      "Cron Runs page: the warning shown in place of the list when the runs could not be read (Render's text, w3/m556)",
  },
  "services.cronRunsSearchPlaceholder": {
    message: "Cron-Job-Läufe durchsuchen",
    description:
      "Cron Runs page: the search field's placeholder and name (Render's text, w3/m556)",
  },
  "services.cronRunsSearchClear": {
    message: "Suche löschen",
    description:
      "Cron Runs page: the search field's clear button, for assistive tech (w3/m556)",
  },
  "services.cronRunsColRun": {
    message: "Lauf",
    description:
      "Cron Runs page: the list's first column header, before the count (Render's RUN, w3/m556)",
  },
  "services.cronRunsColTrigger": {
    message: "Auslöser",
    description:
      "Cron Runs page: the Trigger column's header, and a row's trigger for assistive tech (Render's TRIGGER, w3/m556)",
  },
  "services.cronRunTitle": {
    message: "Cron-Job-Lauf",
    description: "Cron Runs page: a run row's title (Render's text, w3/m556)",
  },
  "services.cronRunStartedAgo": {
    message: "Gelaufen {timestamp}",
    description:
      'Cron Runs page: when a run started, under its title; {timestamp} is elapsed text like "2 hours ago" (Render\'s "Run 21h ago", w3/m556)',
  },
  "services.cronRunBy": {
    message: "von {user}",
    description:
      'Cron Runs page: after when a run started, the email of the user who triggered it (Render\'s "by <user>", w3/m556)',
  },
  "services.cronRunByYou": {
    message: "von dir",
    description:
      'Cron Runs page: after when a run started, when the viewer triggered it (Render\'s "by you", w3/m556)',
  },
  "services.cronRunCanceledBy": {
    message: "Abgebrochen von {user}",
    description:
      "Cron Runs page: a canceled run's line naming the email of the user who canceled it (Render's text, w3/m556)",
  },
  "services.cronRunCanceledByYou": {
    message: "Von dir abgebrochen",
    description:
      "Cron Runs page: a canceled run's line when the viewer canceled it (Render's \"Canceled by you\", w3/m556)",
  },
  "services.cronRunTriggerScheduled": {
    message: "Geplant",
    description:
      "Cron Runs page: the trigger of a run the schedule started (Render's text, w3/m556)",
  },
  "services.cronRunTriggerManual": {
    message: "Manuell",
    description:
      "Cron Runs page: the trigger of a run a user started (Render's text, w3/m556)",
  },
  "services.cronRunsNoMatchTitle": {
    message: "Keine passenden Ergebnisse",
    description:
      "Cron Runs page: title when the search matches none of the loaded runs (Render's text, w3/m556)",
  },
  "services.cronRunsNoMatchBody": {
    message:
      "Nicht gefunden, was du suchst? Passe deine Suche an und prüfe sie auf Tippfehler.",
    description:
      "Cron Runs page: body when the search matches none of the loaded runs (Render's text, w3/m556)",
  },
  "services.cronRunsResetSearch": {
    message: "Suche zurücksetzen",
    description:
      "Cron Runs page: the button that clears a search that matched nothing (Render's text, w3/m556)",
  },
  "services.cronRunsEmptyRecentTitle": {
    message: "Dieser Cron Job ist in den letzten 90 Tagen nicht gelaufen.",
    description:
      "Cron Runs page: title when a cron job whose build succeeded lists no runs (Render's text, w3/m556)",
  },
  "services.cronRunsEmptyRecentBody": {
    message:
      "Du kannst einen Lauf manuell auslösen oder auf den nächsten geplanten Lauf warten.",
    description:
      "Cron Runs page: body when a cron job whose build succeeded lists no runs, above Trigger Run (Render's text, w3/m556)",
  },
  "services.cronRunsEmptyNeverTitle": {
    message: "Dieser Cron Job ist noch nicht gelaufen.",
    description:
      "Cron Runs page: title when a cron job whose build never succeeded has no runs (Render's text, w3/m556)",
  },
  "services.cronRunsEmptyNeverBody": {
    message:
      "Er läuft, sobald sein erster Build fertig ist, oder du löst einen Lauf manuell aus.",
    description:
      "Cron Runs page: body when a cron job whose build never succeeded has no runs, above Trigger Run (Render's text, w3/m556)",
  },
  "services.cronRunsCount_one": {
    message: "{count} Lauf",
    description:
      "Cron Runs page: the list's count, for assistive tech, beside RUN (Render's \"30 runs\", w3/m556)",
  },
  "services.cronRunsCount_other": {
    message: "{count} Läufe",
    description:
      "Cron Runs page: the list's count, for assistive tech, beside RUN (Render's \"30 runs\", w3/m556)",
  },
  "services.suspendCardTitle": {
    message: "Service pausieren",
    description: "Settings tab suspend section title",
  },
  "services.suspendCardDescription": {
    message:
      "Wenn du deinen Service pausierst, wird er heruntergefahren und liefert keinen Traffic mehr aus. Seine URL und Zertifikate bleiben erhalten, und du kannst ihn jederzeit fortsetzen.",
    description:
      "Settings tab suspend section description, for the types served at a public host",
  },
  "services.suspendCardDescriptionNoUrl": {
    message:
      "Wenn du deinen Service pausierst, wird er heruntergefahren und läuft nicht mehr. Du kannst ihn jederzeit fortsetzen.",
    description:
      "Settings tab suspend section description, for private services and background workers — neither has a public URL or certificates to keep",
  },
  "services.resumeCardTitle": {
    message: "Service fortsetzen",
    description:
      "Settings tab resume section title (shown when service is suspended)",
  },
  "services.resumeCardDescription": {
    message:
      "Wenn du deinen Service fortsetzt, geht er wieder online und liefert wieder Traffic aus.",
    description: "Settings tab resume section description",
  },
  "services.resumeCardDescriptionCrashLoop": {
    message:
      "Wir haben diesen Service pausiert, weil er innerhalb von 24 Stunden wiederholt nicht starten und fehlerfrei werden konnte. Häufige Ursachen sind ein Konfigurationsfehler, ein Fehler im Anwendungscode oder ein zu kleiner Compute-Plan. Behebe die Ursache und setze ihn dann fort, um ihn wieder online zu bringen.",
    description:
      "Settings tab resume section description for a service the platform suspended for a crash loop (stuck_crashlooping)",
  },
  "services.resumeCardDescriptionCron": {
    message:
      "Wenn du diesen Cron Job fortsetzt, werden die geplanten Läufe wieder aufgenommen.",
    description: "Settings tab resume section description for a cron job",
  },
  "services.dangerZoneTitle": {
    message: "Gefahrenzone",
    description: "Settings tab delete section title (destructive)",
  },
  "services.dangerZoneDescription": {
    message:
      "Beim Löschen eines Service werden er, sein Deployment und seine URL dauerhaft entfernt. Das lässt sich nicht rückgängig machen.",
    description:
      "Settings tab delete section description, for the types served at a public host",
  },
  "services.dangerZoneDescriptionNoUrl": {
    message:
      "Beim Löschen eines Service werden er und sein Deployment dauerhaft entfernt. Das lässt sich nicht rückgängig machen.",
    description:
      "Settings tab delete section description, for private services, background workers and cron jobs — none of which have a URL to remove",
  },
  "services.deleteButton": {
    message: "Service löschen",
    description: "Danger-zone button that opens the delete-confirm dialog",
  },
  "services.deleteConfirmTitle": {
    message: "{name} löschen?",
    description: "Delete-confirm dialog title",
  },
  "services.deleteConfirmBody": {
    message:
      "Diese Aktion kann nicht rückgängig gemacht werden. Möchtest du diesen Service ({type}) wirklich löschen?",
    description:
      "Delete-confirm dialog body ({type} = lowercase Render type words, e.g. 'web service')",
  },
  "services.deleteConfirmPrompt": {
    message: "Gib unten {phrase} zur Bestätigung ein.",
    description:
      "Body prompt naming Render's exact 'sudo delete <type> <name>' phrase (rendered bold by SudoCommandField)",
  },
  "services.deleteCancel": {
    message: "Abbrechen",
    description: "Delete-confirm dialog cancel button",
  },
  "services.deleteConfirm": {
    message: "Service löschen",
    description:
      "Delete-confirm dialog submit button (armed once the name matches)",
  },
  "services.deleteSuccess": {
    message: "{name} gelöscht",
    description: "Toast on a successful service delete",
  },
  "services.deleteError": {
    message: "{name} konnte nicht gelöscht werden",
    description: "Toast on a failed service delete",
  },
  "services.newServiceButton": {
    message: "Neuer Service",
    description:
      "Button on the services list page that opens the create wizard",
  },
  "services.createTitle": {
    message: "Neuer Service",
    description: "Create-wizard page title",
  },
  "services.createWebTitle": {
    message: "Neuer Web Service",
    description:
      "Create-wizard page title when the web_service type is selected",
  },
  "services.createWebDescription": {
    message:
      "Deploye einen Web Service aus einem Git-Repository oder Docker-Image.",
    description:
      "Create-wizard page subtitle when the web_service type is selected",
  },
  "services.createPrivateTitle": {
    message: "Neuer Private Service",
    description:
      "Create-wizard page title when the private_service type is selected",
  },
  "services.createPrivateDescription": {
    message:
      "Deploye einen Private Service aus einem Git-Repository oder Docker-Image.",
    description:
      "Create-wizard page subtitle when the private_service type is selected",
  },
  "services.createWorkerTitle": {
    message: "Neuer Background Worker",
    description:
      "Create-wizard page title when the background_worker type is selected",
  },
  "services.createWorkerDescription": {
    message:
      "Deploye einen Background Worker aus einem Git-Repository oder Docker-Image.",
    description:
      "Create-wizard page subtitle when the background_worker type is selected",
  },
  "services.createStaticTitle": {
    message: "Neue Static Site",
    description:
      "Create-wizard page title when the static_site type is selected",
  },
  "services.createStaticDescription": {
    message: "Baue und deploye eine Static Site aus einem Git-Repository.",
    description:
      "Create-wizard page subtitle when the static_site type is selected",
  },
  "services.createCronTitle": {
    message: "Neuer Cron Job",
    description: "Create-wizard page title when the cron_job type is selected",
  },
  "services.createCronDescription": {
    message: "Führe einen Befehl nach einem wiederkehrenden Zeitplan aus.",
    description:
      "Create-wizard page subtitle when the cron_job type is selected",
  },
  "services.createSourceTitle": {
    message: "Quellcode",
    description: "Create-wizard source-picker section label",
  },
  "services.createTabGitHub": {
    message: "GitHub",
    description: "Create-wizard source-tab label for connected GitHub repos",
  },
  "services.createTabPublicGit": {
    message: "Öffentliche Git-URL",
    description: "Create-wizard source-tab label for a public git URL",
  },
  "services.createTabImage": {
    message: "Vorhandenes Image",
    description: "Create-wizard source-tab label for a pre-built Docker image",
  },
  "services.createRepoSearchShort": {
    message: "Suchen…",
    description:
      "Repository search placeholder where the field is narrow (a phone); the field keeps its full accessible name",
  },
  "services.createRepoCurrent": {
    message: "Aktuell",
    description:
      "Update Source repository list: tag on the repository the service uses now, shown once another one is chosen",
  },
  "services.createRepoSearchPlaceholder": {
    message: "Repositorys durchsuchen…",
    description: "Create-wizard GitHub tab repo-search input placeholder",
  },
  "services.createRepoPrivateBadge": {
    message: "Privat",
    description: "Badge on a private GitHub repo row in the repo picker",
  },
  "services.createRepoEmpty": {
    message: "Keine Repositories gefunden.",
    description:
      "Create-wizard GitHub tab empty state (no repos in the installation)",
  },
  "services.createRepoNoMatch": {
    message: "Keine Repositories passen zu deiner Suche.",
    description:
      "Create-wizard GitHub tab empty state when the search filter returns nothing",
  },
  "services.createRepoEmptyHint": {
    message:
      "Gib bex auf GitHub Zugriff auf ein Repository oder verbinde ein weiteres Konto.",
    description:
      "Source picker GitHub tab, under the empty state: what to do when the connected accounts grant no repository. Credentials is the menu beside the search field",
  },
  "services.createRepoLoadError": {
    message: "Deine Repositories konnten nicht geladen werden.",
    description:
      "Source picker GitHub tab: the repository list could not be read, shown with a Try again button. Not the same as having no repositories",
  },
  "services.createRepoListLabel": {
    message: "Repositories",
    description:
      "Accessible name of the repository list in the source picker's GitHub tab",
  },
  "services.createRepoClearSearch": {
    message: "Suche löschen",
    description:
      "Source picker GitHub tab: button under the no-match message that empties the repository search field",
  },
  "services.sourceNoChangesDeploy": {
    message: "Keine Änderungen zu deployen.",
    description:
      "Update Source dialog footer, beside a disabled Deploy button (the GitHub, Public Git and Build source tabs): the dialog still shows the service's current source and build settings, so there is nothing to deploy",
  },
  "services.sourceNoChangesConnect": {
    message: "Keine Änderungen zu verbinden.",
    description:
      "Update Source dialog footer, beside a disabled Connect button (the Existing Image tab): the dialog still shows the image and registry credential the service already runs, so there is nothing to connect",
  },
  "services.sourceSelectRepo": {
    message: "Wähle ein Repository aus.",
    description:
      "Update Source dialog footer: why Deploy is disabled on the GitHub tab before a repository is picked",
  },
  "services.sourceSelectBuildSource": {
    message: "Wähle eine Build-Quelle aus.",
    description:
      "Update Source dialog footer: why Deploy is disabled on the Build source tab before one is picked",
  },
  "services.sourceEnterRepoUrl": {
    message: "Gib eine Repository-URL ein.",
    description:
      "Update Source dialog footer: why Deploy is disabled on the Public Git URL tab while the URL field is empty",
  },
  "services.sourceEnterImage": {
    message: "Gib ein Image an.",
    description:
      "Update Source dialog footer: why Connect is disabled on the Existing Image tab while the image field is empty",
  },
  "services.createRepoOpenCredentials": {
    message: "Zugangsdaten öffnen",
    description:
      "Source picker GitHub tab: button under the empty repository list that opens the Credentials menu beside the search field",
  },
  "services.sourcePending": {
    message: "Neue Quelle: {source}",
    description:
      "Update Source dialog footer: names what the primary button (Deploy, or Connect on the Existing Image tab) will deploy. {source} is a repository and branch, an image, or a build source's name",
  },
  "services.sourceBranchLocked": {
    message: "Verfügbar, sobald du ein Repository gewählt hast.",
    description:
      "Update Source dialog: helper under the disabled Branch field before a repository is picked or a valid URL entered",
  },
  "services.sourceNoBuildSources": {
    message: "Keine Build-Quelle zur Auswahl.",
    description:
      "Update Source dialog footer: why Deploy is disabled on the Build source tab when the workspace has no build sources",
  },
  "services.sourceFixField": {
    message: "Fülle die Felder oben aus, um fortzufahren.",
    description:
      "Update Source dialog footer: why the primary button (Deploy, or Connect on the Existing Image tab) is disabled while a field is malformed (its own message is under it) or a required one is empty",
  },
  "services.createRepoNoMatchHint": {
    message:
      "Nicht aufgeführt? Gib bex auf GitHub Zugriff darauf oder verbinde ein weiteres Konto.",
    description:
      "Source picker GitHub tab, under the no-match message: the usual cause is a repository the connected accounts do not grant. Shown with Clear search and Open Credentials buttons",
  },
  "services.sourceBranchLockedUrl": {
    message: "Verfügbar, sobald du eine Repository-URL eingegeben hast.",
    description:
      "Update Source dialog, Public Git URL tab: helper under the disabled Branch field before a valid URL is entered",
  },
  "services.createGitConnectPromptTitle": {
    message: "GitHub verbinden",
    description:
      "Create-wizard GitHub tab connect-prompt heading (no GitHub connection yet)",
  },
  "services.createGitConnectPromptBody": {
    message:
      "Verbinde dein GitHub-Konto, um aus privaten oder öffentlichen Repositories zu deployen.",
    description:
      "Create-wizard GitHub tab connect-prompt body (no GitHub connection yet)",
  },
  "services.createPublicUrlLabel": {
    message: "Repository-URL",
    description: "Create-wizard Public Git URL tab input label",
  },
  "services.createPublicUrlPlaceholder": {
    message: "https://github.com/you/your-repo",
    description: "Create-wizard Public Git URL tab input placeholder",
  },
  "services.createPublicUrlError": {
    message:
      "Gib eine URL mit https:// oder eine GitHub-SSH-URL (git@github.com:owner/repo.git) ein.",
    description: "Create-wizard Public Git URL tab validation message",
  },
  "services.createPublicUrlFileError": {
    message: "Verwende die Repository-URL, nicht die URL einer Dateiseite.",
    description:
      "Create-wizard Public Git URL: a GitHub file (blob) page was pasted, which names no repository to build (w4/576)",
  },
  "services.createImageLabel": {
    message: "Docker-Image",
    description: "Create-wizard Existing Image tab input label",
  },
  "services.createImagePlaceholder": {
    message: "docker.io/library/nginx:latest",
    description: "Create-wizard Existing Image tab input placeholder",
  },
  "services.createImagePortHint": {
    message:
      "bex verwendet den Port, den das Image deklariert (EXPOSE). Lege den Port unten nur fest, wenn das Image auf einem Port lauscht, den es nicht deklariert.",
    description:
      "Create-wizard Existing Image tab hint: set the port below to the one the image listens on (w9/011, w6/m149)",
  },
  "services.createImageMalformed": {
    message:
      "Verwende eine Docker-Image-Referenz, z. B. docker.io/library/nginx:latest — ein Pfad in Kleinbuchstaben mit optionalem Tag oder Digest.",
    description: "Inline error under a malformed Docker Image field (w4/406)",
  },
  "services.createImageTooLong": {
    message: "Höchstens 512 Bytes",
    description: "Inline error under an over-long Docker Image field (w4/406)",
  },
  "services.createImagePrivateHost": {
    message:
      "Der Registry-Host muss öffentlich sein — localhost sowie private oder Link-Local-Adressen können nicht verwendet werden.",
    description:
      "Inline error under a Docker Image field naming a private registry host (w4/406)",
  },
  "services.createFieldPort": {
    message: "Port",
    description: "Create wizard: service port field label",
  },
  "services.createFieldPortFromImage": {
    message: "Aus dem Image",
    description:
      "Create wizard: port placeholder for an image-port service (a Docker-built or prebuilt-image web or private service), whose ports come from its image (w6/m148, w4/m243, w4/m323)",
  },
  "services.createFieldPortImageHint": {
    message: "Leer lassen, dann wird der im Image deklarierte Port zu $PORT.",
    description:
      "Create wizard: port hint for an image-port private service (Docker-built or prebuilt image), whose primary is also its $PORT — blank takes the image's declared ports (w6/m148, w4/m323, w4/615)",
  },
  "services.createFieldPortImageHintWeb": {
    message:
      "Leer lassen, um den Traffic an den im Image deklarierten Port zu leiten; die Variable $PORT bleibt 10000, sofern Sie keinen Port setzen.",
    description:
      "Create wizard: port hint for an image-port web service (Docker-built or prebuilt image) — the selected port gets the traffic, while $PORT stays Render's 10000 unless set (w4/m243, w4/615)",
  },
  "services.createFieldPortHint": {
    message:
      "Ihr Container muss auf Port {port} lauschen; bex leitet den Traffic dorthin und setzt ihn als $PORT.",
    description: "Create wizard: service port field hint",
  },
  "services.createRegistryCredentialLabel": {
    message: "Registry-Zugangsdaten",
    description: "Create-wizard private-image registry credential label",
  },
  "services.createRegistryCredentialPlaceholder": {
    message: "Registry-Zugangsdaten auswählen",
    description: "Create-wizard registry credential picker placeholder",
  },
  "services.createRegistryCredentialNone": {
    message: "Keine Zugangsdaten — öffentliches Image",
    description: "Create-wizard registry credential picker empty option",
  },
  "services.createRegistryCredentialDescription": {
    message:
      "Wähle gespeicherte Workspace-Zugangsdaten, wenn das Image oder das Basis-Image des Dockerfiles privat ist.",
    description: "Create-wizard registry credential picker help text",
  },
  "services.registryCredentialEmpty": {
    message: "Keine gespeicherten Registry-Zugangsdaten vorhanden.",
    description: "Registry credential selector empty state",
  },
  "services.registryCredentialListError": {
    message: "Registry-Zugangsdaten konnten nicht geladen werden.",
    description: "Registry credential selector load error",
  },
  "services.registryCredentialManage": {
    message: "Registry-Zugangsdaten verwalten",
    description: "Link from a service credential selector to account settings",
  },
  "services.registryCredentialSettingsTitle": {
    message: "Registry-Zugangsdaten für Images",
    description: "Service Settings private-image credential card title",
  },
  "services.registryCredentialSettingsDescription": {
    message:
      "Wähle die gespeicherten Zugangsdaten, mit denen dieses private Image oder das Basis-Image des Dockerfiles geladen wird.",
    description: "Service Settings private-image credential card description",
  },
  "services.registryCredentialSave": {
    message: "Zugangsdaten speichern",
    description: "Service Settings registry credential save action",
  },
  "services.registryCredentialSaved": {
    message: "Registry-Zugangsdaten aktualisiert.",
    description: "Toast after attaching or changing a registry credential",
  },
  "services.registryCredentialCleared": {
    message: "Registry-Zugangsdaten entfernt.",
    description: "Toast after clearing a registry credential",
  },
  "services.registryCredentialError": {
    message: "Registry-Zugangsdaten konnten nicht aktualisiert werden.",
    description: "Toast after a registry credential update fails",
  },
  "services.headerRegistryCredential": {
    message: "Registry-Zugangsdaten:",
    description: "Service detail header label for a bound registry credential",
  },
  "services.createSettingsTitle": {
    message: "Einstellungen",
    description: "Create-wizard settings section heading",
  },
  "services.createFieldName": {
    message: "Name",
    description: "Create-wizard name input label",
  },
  "services.createFieldNamePlaceholder": {
    message: "my-service",
    description: "Create-wizard name input placeholder",
  },
  "services.createFieldNameError": {
    message:
      "Gib einen Namen mit höchstens 100 Zeichen ohne Zeilenumbrüche oder Steuerzeichen ein.",
    description: "Create-wizard name validation message",
  },
  "services.createFieldNameResourceName": {
    message: "Ressourcenname: {name}",
    description:
      "Create-wizard preview of the resource name (slug) bex derives from a free-form service name (w5/m153)",
  },
  "services.createFieldNameTaken": {
    message: "Name wird bereits verwendet",
    description:
      "Create-wizard inline error when the service name is already taken in the current workspace (w4/m19)",
  },
  "services.createFieldNameUseSuggestion": {
    message: "{name} verwenden",
    description:
      "Create-wizard button offering the suggested free name in place of a taken one (w4/m19)",
  },
  "services.createFieldNameChecking": {
    message: "Verfügbarkeit wird geprüft…",
    description:
      "Create-wizard transient message while the debounced name-availability check is in flight (w4/m19)",
  },
  "services.createFieldBranch": {
    message: "Branch",
    description: "Create-wizard branch input label (git sources)",
  },
  "services.createFieldBranchPlaceholder": {
    message: "Standard-Branch des Repositorys",
    description:
      "Create-wizard branch input placeholder when the repo's default is not known client-side: a blank branch builds the repository's default branch (w8/m76)",
  },
  "services.createFieldRootDir": {
    message: "Stammverzeichnis",
    description: "Create-wizard root-directory input label",
  },
  "services.createFieldRootDirPlaceholder": {
    message: "z. B. backend",
    description: "Create-wizard root-directory input placeholder",
  },
  "services.createFieldRootDirHint": {
    message:
      "Unterverzeichnis, aus dem gebaut wird. Leer lassen, um das Repository-Root zu verwenden.",
    description: "Create-wizard root-directory input hint text",
  },
  "services.createFieldRuntime": {
    message: "Runtime",
    description: "Create-wizard Render-compatible runtime selector label",
  },
  "services.createFieldBuildCommand": {
    message: "Build-Befehl",
    description: "Create-wizard Render-compatible build command label",
  },
  "services.createFieldStartCommand": {
    message: "Startbefehl",
    description: "Create-wizard Render-compatible start command label",
  },
  "services.createFieldDockerfilePath": {
    message: "Dockerfile-Pfad",
    description: "Create-wizard Dockerfile-path label for the Docker runtime",
  },
  "services.createFieldDockerfilePathPlaceholder": {
    message: "Dockerfile",
    description: "Create-wizard Dockerfile-path placeholder",
  },
  "services.createFieldDockerfilePathHint": {
    message: "Pfad relativ zum Stammverzeichnis. Leer lassen für Dockerfile.",
    description: "Create-wizard Dockerfile-path help text",
  },
  "services.createFieldDockerCommand": {
    message: "Docker-Befehl",
    description: "Create-wizard Docker CMD override label (Render wording)",
  },
  "services.createFieldDockerCommandPlaceholder": {
    message: "CMD aus dem Dockerfile verwenden",
    description: "Create-wizard optional Docker Command placeholder",
  },
  "services.imageCommandPlaceholder": {
    message: "Standardbefehl des Images verwenden",
    description:
      "Create-wizard and settings placeholder for an Existing Image service's optional Docker Command",
  },
  "services.imageCommandHint": {
    message:
      "Überschreibt den Standardbefehl des Images (sein CMD). Leer lassen, um den Standard zu verwenden.",
    description: "Existing Image service Settings: Docker Command help text",
  },
  "services.createRuntimeNode": {
    message: "Node",
    description: "Create-wizard Node runtime option",
  },
  "services.createRuntimePython": {
    message: "Python 3",
    description: "Create-wizard Python runtime option",
  },
  "services.createRuntimeGo": {
    message: "Go",
    description: "Create-wizard Go runtime option",
  },
  "services.createRuntimeRuby": {
    message: "Ruby",
    description: "Create-wizard Ruby runtime option",
  },
  "services.createRuntimeRust": {
    message: "Rust",
    description: "Create-wizard Rust runtime option",
  },
  "services.createRuntimeElixir": {
    message: "Elixir",
    description: "Create-wizard Elixir runtime option",
  },
  "services.createRuntimeDocker": {
    message: "Docker",
    description: "Create-wizard Docker runtime option",
  },
  "services.createFieldPlan": {
    message: "Compute-Plan",
    description: "Create-wizard plan-picker section label",
  },
  "services.createFieldAutoDeploy": {
    message: "Auto-Deploy bei Push",
    description: "Create-wizard auto-deploy toggle label",
  },
  "services.createFieldAutoDeployHint": {
    message: "Deployt automatisch neu, wenn du auf diesen Branch pushst.",
    description: "Create-wizard auto-deploy toggle hint text",
  },
  "services.createCancel": {
    message: "Abbrechen",
    description: "Create-wizard cancel button",
  },
  "services.createSubmit": {
    message: "Service deployen",
    description: "Create-wizard submit button",
  },
  "services.createSuccess": {
    message: "{name} wird deployt…",
    description: "Toast shown after createService succeeds",
  },
  "services.createError": {
    message: "{name} konnte nicht erstellt werden. Bitte versuche es erneut.",
    description: "Toast shown after createService fails",
  },
  "services.scaleSuccess_one": {
    message: "Skalierung auf {count} Instanz…",
    description:
      "Toast acknowledging that scaleService accepted the desired count; convergence is still asynchronous (singular)",
  },
  "services.scaleSuccess_other": {
    message: "Skalierung auf {count} Instanzen…",
    description:
      "Toast acknowledging that scaleService accepted the desired count; convergence is still asynchronous (plural)",
  },
  "services.scaleError": {
    message: "Die Instanzanzahl konnte nicht aktualisiert werden.",
    description: "Toast shown after scaleService fails",
  },
  "services.createTypePickerTitle": {
    message: "Service-Typ",
    description: "Label above the service type picker in the create wizard",
  },
  "services.createTypeWebDesc": {
    message: "Stelle deinen Service unter einer öffentlichen URL bereit",
    description:
      "Description shown under the Web Service type card in the create wizard",
  },
  "services.createTypePrivateDesc": {
    message: "Nur innerhalb des Plattform-Netzwerks erreichbar",
    description: "Description shown under the Private Service type card",
  },
  "services.createTypeWorkerDesc": {
    message: "Führe Hintergrundverarbeitung ohne Port oder URL aus",
    description: "Description shown under the Background Worker type card",
  },
  "services.createTypeCronDesc": {
    message: "Führe einen Befehl nach einem wiederkehrenden Zeitplan aus",
    description: "Description shown under the Cron Job type card",
  },
  "services.createTypeStaticDesc": {
    message:
      "Baue eine statische Website und liefere sie aus dem Objektspeicher aus",
    description: "Description shown under the Static Site type card",
  },
  "services.createFieldSchedule": {
    message: "Zeitplan",
    description: "Label for the cron schedule field in the create wizard",
  },
  "services.createFieldSchedulePlaceholder": {
    message: "0 0 * * *",
    description: "Placeholder for the cron schedule field",
  },
  "services.createFieldScheduleHint": {
    message:
      "Ein Crontab-Ausdruck mit 5 Feldern (Minute Stunde Tag Monat Wochentag). Läuft in UTC.",
    description: "Hint text under the schedule field",
  },
  "services.createFieldSchedulePreview": {
    message: "{description} · läuft in UTC",
    description:
      "Live human-readable description of a valid schedule, shown under the field",
  },
  "services.createFieldScheduleError": {
    message:
      "Gib einen gültigen Cron-Ausdruck mit 5 Feldern ein, z. B. 0 0 * * *.",
    description: "Validation error for an invalid cron expression",
  },
  "services.createFieldCommand": {
    message: "Befehl",
    description: "Label for the command field in the create wizard",
  },
  "services.createFieldCommandPlaceholder": {
    message: "python script.py",
    description: "Placeholder for the command field",
  },
  "services.createFieldCommandHint": {
    message: "Der Befehl, der bei jeder geplanten Ausführung läuft.",
    description: "Hint text under the command field",
  },
  "services.createFieldPublishPath": {
    message: "Veröffentlichungsverzeichnis",
    description: "Label for the publish directory field in the create wizard",
  },
  "services.createFieldPublishPathPlaceholder": {
    message: "z. B. dist",
    description: "Placeholder for the publish directory field",
  },
  "services.createFieldPublishPathHint": {
    message:
      "Der Ordner mit Ihrer gebauten Site, relativ zum Stammverzeichnis (z. B. dist, oder . für das Stammverzeichnis selbst).",
    description: "Hint text under the publish directory field",
  },
  "services.createFieldPublishPathRequired": {
    message:
      "Erforderlich. Gib das Verzeichnis mit der gebauten Ausgabe an, das als Root der Website ausgeliefert wird, relativ zum Stammverzeichnis (z. B. dist, build, public oder . für das Stammverzeichnis selbst — das Repository-Root, wenn das Stammverzeichnis leer ist).",
    description:
      "Hint under the publish directory field while it is blank: the field is required, the placeholder is only an example",
  },
  "services.createNoPublicUrlNote": {
    message: "Dieser Service-Typ hat keine öffentliche URL.",
    description:
      "Note shown for private/worker types that don't produce a public URL",
  },
  "services.createFieldEnvVarsTitle": {
    message: "Umgebungsvariablen",
    description: "Section heading for env vars in the create wizard",
  },
  "services.createFieldEnvVarsAdd": {
    message: "Variable hinzufügen",
    description: "Button to add an env var row in the create wizard",
  },
  "services.envRowKey": {
    message: "Schlüssel von Variable {n}",
    description:
      "A create-flow variable row's key field (aria-label), named by its row as its Remove button is (w4/544)",
  },
  "services.envRowValue": {
    message: "Wert von Variable {n}",
    description:
      "A create-flow variable row's value field (aria-label), named by its row (w4/544)",
  },
  "services.envRowGenerate": {
    message: "Wert für Variable {n} generieren",
    description:
      "A create-flow variable row's Generate button (aria-label), named by its row (w4/544)",
  },
  "services.envRowRemove": {
    message: "Variable {n} entfernen",
    description:
      "A create-flow variable row's Remove button (aria-label), named by its row (w4/544)",
  },
  "services.fileRowName": {
    message: "Dateiname für Secret-Datei {n}",
    description:
      "A create-flow secret file row's name field (aria-label), named by its row and starting with the dialog's visible \"File name\" label (w4/544)",
  },
  "services.fileRowContent": {
    message: "Inhalt von Secret-Datei {n}",
    description:
      "A create-flow secret file row's contents field (aria-label), named by its row (w4/544)",
  },
  "services.fileRowRemove": {
    message: "Secret-Datei {n} entfernen",
    description:
      "A create-flow secret file row's Remove button (aria-label), named by its row (w4/544)",
  },
  "services.createFieldEnvVarsKey": {
    message: "Schlüssel",
    description: "Label for the env var key column in the create wizard",
  },
  "services.createFieldEnvVarsValue": {
    message: "Wert",
    description: "Label for the env var value column in the create wizard",
  },
  "services.createFieldEnvVarsKeyPlaceholder": {
    message: "KEY_NAME",
    description: "Placeholder for the env var key input in the create wizard",
  },
  "services.createFieldEnvVarsValuePlaceholder": {
    message: "Wert",
    description: "Placeholder for the env var value input in the create wizard",
  },
  "services.createFieldEnvVarsKeyError": {
    message:
      "Verwende bis zu 253 Buchstaben, Ziffern, Punkte, Bindestriche und Unterstriche; nicht '.' und nicht mit '..' beginnend.",
    description:
      "Error shown when an env var key is invalid in the create wizard",
  },
  "services.createFieldSecretFilesTitle": {
    message: "Secret-Dateien",
    description: "Section heading for secret files in the create wizard",
  },
  "services.createFieldSecretFilesHint": {
    message:
      "Ab dem ersten Deployment schreibgeschützt unter /etc/secrets eingebunden.",
    description: "Hint for create-time secret files",
  },
  "services.createFieldSecretFilesAdd": {
    message: "Secret-Datei hinzufügen",
    description: "Button to add a create-time secret file",
  },
  "services.createFieldSecretFilesNamePlaceholder": {
    message: "credentials.json",
    description: "Placeholder for a secret file name",
  },
  "services.createFieldSecretFilesContentPlaceholder": {
    message: "Inhalt des Secrets einfügen",
    description: "Placeholder for secret file contents",
  },
  "services.createFieldSecretFilesNameError": {
    message:
      "Verwende nur Buchstaben, Ziffern, Punkte, Bindestriche und Unterstriche; . und .. sind nicht erlaubt.",
    description: "Invalid secret file name error",
  },
  "services.createFieldEnvironmentTitle": {
    message: "Projekt und Umgebung",
    description: "Create wizard grouping section title",
  },
  "services.createFieldProject": {
    message: "Projekt",
    description: "Accessible label for the create project picker",
  },
  "services.createFieldProjectNone": {
    message: "Kein Projekt",
    description: "Unassigned option in the project picker",
  },
  "services.createFieldEnvironment": {
    message: "Umgebung",
    description: "Accessible label for the create environment picker",
  },
  "services.createFieldEnvironmentNone": {
    message: "Keine Umgebung",
    description: "Unassigned option in the environment picker",
  },
  "services.createFieldEnvironmentHint": {
    message:
      "Wenn du eine Umgebung auswählst, wird der Service auch dem übergeordneten Projekt hinzugefügt.",
    description: "Hint for create-time environment assignment",
  },
  "services.navEvents": {
    message: "Ereignisse",
    description: "Service-detail nav item (events tab)",
  },
  "services.navDeploys": {
    message: "Deployments",
    description:
      "Service-detail nav item (dedicated deploy-history tab, w9/002)",
  },
  "services.navRuns": {
    message: "Ausführungen",
    description:
      "Service-detail nav item for a cron job: its run history, Render's Runs page (w2/m360)",
  },
  "services.navDisk": {
    message: "Disk",
    description:
      "Service-detail nav item (persistent disk page); Render's label is singular",
  },
  "services.diskDescription": {
    message:
      "Binde eine SSD ein, um die Dateisystemdaten deines Services über Deployments hinweg zu erhalten. Disks kosten 0,175 $/GB pro Monat.",
    description:
      "Disk tab card description; bex's rate is 30% below Render's $0.25",
  },
  "services.diskEmptyTitle": {
    message: "Keine Disk angebunden",
    description: "Disk tab empty state title",
  },
  "services.diskEmptyBody": {
    message:
      "Nur Dateien unter dem Mount-Pfad der Disk bleiben über Deployments und Neustarts hinweg erhalten.",
    description: "Disk tab empty state body",
  },
  "services.diskPaidOnly": {
    message:
      "Disks erfordern einen kostenpflichtigen Compute-Plan. Upgrade den Tarif dieses Services, um eine Disk anzubinden.",
    description: "Shown instead of the add button on a free service",
  },
  "services.diskAddAction": {
    message: "Disk hinzufügen",
    description: "Button that opens and submits the add-disk form",
  },
  "services.createDiskRemove": {
    message: "Disk entfernen",
    description:
      "Create form Advanced: drops the drafted disk (Render's 'Remove disk', w4/m652)",
  },
  "services.createEnvGroupsDescription": {
    message:
      "Umgebungsgruppen sind Sammlungen von Umgebungsvariablen und Secret-Dateien, die Sie für mehrere Dienste gemeinsam nutzen können.",
    description:
      "Create form Advanced: Linked Environment Groups' description, Render's copy (w4/731)",
  },
  "services.createEnvGroupPlaceholder": {
    message: "Umgebungsgruppe auswählen…",
    description:
      "Create form Advanced: placeholder of a Linked Environment Groups picker with no group chosen (Render's 'Select an environment group…', w4/731)",
  },
  "services.createEnvGroupRowLabel": {
    message: "Verknüpfte Umgebungsgruppe {n}",
    description:
      "Create form Advanced: accessible name of the picker holding the nth linked environment group (w4/731)",
  },
  "services.createEnvGroupAddLabel": {
    message: "Umgebungsgruppe verknüpfen",
    description:
      "Create form Advanced: accessible name of the empty picker that links one more environment group (w4/731)",
  },
  "services.diskLoadErrorTitle": {
    message: "Die Disk dieses Services konnte nicht geladen werden",
    description: "Disk tab error state title",
  },
  "services.diskWarningsTitle": {
    message: "Beachte Folgendes:",
    description: "Heading above the five add-disk warnings (Render's wording)",
  },
  "services.diskWarningZeroDowntime": {
    message:
      "Das Anbinden einer Disk deaktiviert Zero-Downtime-Deployments für den Service.",
    description: "Add-disk warning 1",
  },
  "services.diskWarningSingleInstance": {
    message:
      "Services mit angebundener Disk können nicht auf mehrere Instanzen skaliert werden.",
    description: "Add-disk warning 2",
  },
  "services.diskWarningOnePerService": {
    message: "Du kannst höchstens eine Disk pro Service anbinden.",
    description: "Add-disk warning 3",
  },
  "services.diskWarningMountPathOnly": {
    message:
      "Nur Dateien unter dem Mount-Pfad deiner Disk werden dauerhaft gespeichert.",
    description: "Add-disk warning 4",
  },
  "services.diskWarningNoSharing": {
    message:
      "Andere Services können nicht auf die Disk dieses Services zugreifen.",
    description: "Add-disk warning 5",
  },
  "services.diskMountPathLabel": {
    message: "Mount-Pfad",
    description: "Add-disk form field label",
  },
  "services.diskMountPathHint": {
    message:
      "Der absolute Mount-Pfad der Disk. Nur Dateien unter diesem Pfad bleiben über Deployments hinweg erhalten. Darf nicht das Root-Verzeichnis (/) sein.",
    description: "Add-disk mount path helper text",
  },
  "services.diskMountPathRequired": {
    message: "Gib einen Mount-Pfad ein.",
    description: "Client-side validation: empty mount path",
  },
  "services.diskMountPathAbsolute": {
    message: "Der Mount-Pfad muss absolut sein (mit / beginnen).",
    description: "Client-side validation: relative mount path",
  },
  "services.diskMountPathNotRoot": {
    message:
      "Der Mount-Pfad darf nicht das Root-Verzeichnis sein und nicht mit einem Schrägstrich enden.",
    description: "Client-side validation: root or trailing slash",
  },
  "services.diskMountPathReserved": {
    message:
      "Dieser Pfad ist von der Plattform reserviert. Binde stattdessen ein Unterverzeichnis davon ein.",
    description: "Client-side validation: reserved mount path",
  },
  "services.diskSizeLabel": {
    message: "Größe",
    description: "Add-disk size field label",
  },
  "services.diskSizeHint": {
    message:
      "Du kannst die Größe später erhöhen, aber nicht verringern. Wir empfehlen, mit dem niedrigsten Wert zu beginnen, der zu deinem Anwendungsfall passt.",
    description: "Add-disk size helper text (Render's wording)",
  },
  "services.diskSizeRange": {
    message: "Gib eine ganze Zahl von {min} bis {max} GB ein.",
    description:
      "Create form: the disk size is not a whole number in bex-api's range",
  },
  "services.diskSizeAboveMax": {
    message: "Für Größen über 1 TB wende dich bitte an den Support.",
    description:
      "Disk size above Render's 1 TB cap, on the create form and the Disk page (Render's words; bex has no support address to link)",
  },
  "services.diskSizeChip": {
    message: "{size} GB",
    description: "Size quick-select chip and size display",
  },
  "services.diskAddSuccess": {
    message: "Disk angebunden. Der Service wird neu deployt.",
    description: "Toast after a disk is attached",
  },
  "services.diskGrowLabel": {
    message: "Größe erhöhen (GB)",
    description: "Grow control label on an attached disk",
  },
  "services.diskGrowAction": {
    message: "Größe erhöhen",
    description: "Grow button",
  },
  "services.diskGrowHint": {
    message:
      "Die Vergrößerung verursacht keine Downtime, und der zusätzliche Speicher steht innerhalb weniger Sekunden zur Verfügung. Eine Disk kann nie verkleinert werden.",
    description: "Grow helper text in Render's wording",
  },
  "services.diskResizeSuccess": {
    message: "Disk-Größe erhöht.",
    description: "Toast after a successful grow",
  },
  "services.diskDeleteAction": {
    message: "Disk löschen",
    description: "Destructive action on an attached disk",
  },
  "services.diskDeleteTitle": {
    message: "Diese Disk löschen?",
    description: "Delete confirmation title",
  },
  "services.diskDeleteWarning": {
    message:
      "Alle Daten auf der Disk gehen verloren und der Service verliert sofort den Zugriff darauf. Auch ihre Snapshots werden gelöscht. Das kann nicht rückgängig gemacht werden.",
    description: "Delete confirmation body",
  },
  "services.diskDeleteConfirm": {
    message: "Disk löschen",
    description: "Delete confirmation action",
  },
  "services.diskDeleteSuccess": {
    message: "Disk gelöscht.",
    description: "Toast after a disk is deleted",
  },
  "services.diskSnapshotsTitle": {
    message: "Snapshots",
    description: "Snapshots card title",
  },
  "services.diskSnapshotsDescription": {
    message:
      "Von dieser Disk wird täglich ein Snapshot erstellt, der mindestens sieben Tage aufbewahrt wird.",
    description: "Snapshots card description",
  },
  "services.diskSnapshotsNotConfiguredTitle": {
    message: "Snapshots sind nicht eingerichtet",
    description:
      "Snapshots card heading when the deployment has no snapshot object store configured (not a failure).",
  },
  "services.diskSnapshotsNotConfiguredBody": {
    message:
      "Diese bex-Installation hat keinen Objektspeicher für Disk-Snapshots konfiguriert, daher werden keine erstellt. Deine Daten sind davon nicht betroffen — bitte einen Operator, sie zu aktivieren.",
    description:
      "Explains that snapshots are unconfigured rather than broken, and that an operator enables them.",
  },
  "services.diskSnapshotsEmptyTitle": {
    message: "Noch keine Snapshots",
    description: "Snapshots empty state title",
  },
  "services.diskSnapshotsEmptyBody": {
    message: "Der erste Snapshot wird beim nächsten nächtlichen Lauf erstellt.",
    description: "Snapshots empty state body",
  },
  "services.diskSnapshotsFailedTitle": {
    message: "Letzter Snapshot fehlgeschlagen",
    description: "A disk's most recent nightly snapshot attempt failed",
  },
  "services.diskSnapshotsFailedReadBody": {
    message:
      "Ein Teil dieser Disk konnte nicht gelesen werden, daher wurde kein Snapshot erstellt und aktuelle Änderungen sind nicht gesichert. Vorhandene Snapshots sind nicht betroffen. Wenn das erneut passiert, wende dich an den Support.",
    description: "Failed snapshot: the volume could not be read",
  },
  "services.diskSnapshotsFailedUploadBody": {
    message:
      "Der Snapshot konnte nicht im Speicher abgelegt werden, daher sind aktuelle Änderungen nicht gesichert. Vorhandene Snapshots sind nicht betroffen; der nächste nächtliche Lauf versucht es erneut.",
    description: "Failed snapshot: the object store did not accept it",
  },
  "services.diskSnapshotsFailedConfigBody": {
    message:
      "Der Snapshot-Job konnte nicht korrekt starten, daher sind aktuelle Änderungen nicht gesichert. Vorhandene Snapshots sind nicht betroffen. Wende dich an den Support.",
    description: "Failed snapshot: the job was misconfigured",
  },
  "services.diskSnapshotsFailedOtherBody": {
    message:
      "Der letzte Snapshot-Versuch ist fehlgeschlagen, daher sind aktuelle Änderungen nicht gesichert. Vorhandene Snapshots sind nicht betroffen. Wenn das erneut passiert, wende dich an den Support.",
    description: "Failed snapshot: an unrecognized stage",
  },
  "services.diskSnapshotsOverdueTitle": {
    message: "Kein aktueller Snapshot",
    description: "A disk's nightly snapshot did not arrive when expected",
  },
  "services.diskSnapshotsOverdueBody": {
    message:
      "Der bis {dueBy} erwartete nächtliche Snapshot wurde nicht erstellt, daher sind aktuelle Änderungen an dieser Disk nicht gesichert. Vorhandene Snapshots sind nicht betroffen. Wenn das anhält, wende dich an den Support.",
    description:
      "Explains an overdue nightly snapshot; {dueBy} is a local date/time",
  },
  "services.diskSnapshotsNotRunningTitle": {
    message: "Snapshots laufen nicht",
    description:
      "The platform could not schedule this disk's nightly snapshots",
  },
  "services.diskSnapshotsNotRunningBody": {
    message:
      "bex konnte die nächtlichen Snapshots dieser Disk nicht planen, daher werden neue Änderungen nicht gesichert. Vorhandene Snapshots sind nicht betroffen. Wende dich an den Support.",
    description:
      "Explains that nightly snapshots cannot be scheduled and whom to contact",
  },
  "services.diskSnapshotsUnavailableTitle": {
    message: "Snapshots sind nicht verfügbar",
    description: "Snapshots error state title",
  },
  "services.diskRestoreDatabaseWarning": {
    message:
      "Stelle keine Disk wieder her, um eine darauf laufende Datenbank zu retten — das Wiederherstellen von Dateien unter einer laufenden Datenbank kann sie beschädigen. Verwende stattdessen das eigene Backup und die Wiederherstellung dieser Datenbank.",
    description:
      "Render's database-recovery warning, carried verbatim in meaning",
  },
  "services.diskRestoreAction": {
    message: "Wiederherstellen",
    description: "Restore button on a snapshot row",
  },
  "services.diskRestoreTitle": {
    message: "Diesen Snapshot wiederherstellen?",
    description: "Restore confirmation title",
  },
  "services.diskRestoreWarning": {
    message:
      "Der Service wird gestoppt, die Disk durch diesen Snapshot ersetzt und der Service neu gestartet. Alles, was nach dem Snapshot geschrieben wurde, geht verloren. Das kann nicht rückgängig gemacht werden.",
    description: "Restore confirmation body",
  },
  "services.diskRestoreConfirm": {
    message: "Disk wiederherstellen",
    description: "Restore confirmation action",
  },
  "services.diskUnsupportedType": {
    message:
      "Persistente Disks benötigen eine dauerhaft laufende Instanz zum Einbinden, daher sind sie bei Web Services, Private Services und Background Workern verfügbar.",
    description:
      "Empty-state body on the Disk tab for a service type that cannot have a disk.",
  },
  "services.diskSudoPrompt": {
    message: "Zum Fortfahren gib {phrase} ein",
    description:
      "Label above the typed-confirmation input for an irreversible disk action.",
  },
  "services.diskConfigTitle": {
    message: "Disk-Konfiguration",
    description:
      "Heading of the card holding the disk's mount path and size settings.",
  },
  "services.diskConfigDescription": {
    message: "Konfiguriere Mount-Pfad und Größe dieser Disk.",
    description: "Subtitle of the disk configuration card.",
  },
  "services.diskSizeUnit": {
    message: "GB",
    description: "Unit suffix shown beside the disk size input.",
  },
  "services.diskEditAction": {
    message: "Bearbeiten",
    description: "Button that unlocks the disk size field for editing.",
  },
  "services.diskProvisionedLabel": {
    message: "{size} GB bereitgestellt",
    description:
      "Provisioned disk size shown beside the usage chart heading; this is the size billing meters.",
  },
  "services.diskDeleteCardTitle": {
    message: "Disk löschen",
    description:
      "Heading of the delete-disk card. Distinct from diskDeleteTitle, which is the confirmation dialog's question.",
  },
  "services.diskDeleteCardDescription": {
    message:
      "Die Disk wird von diesem Service getrennt und alle Daten darauf werden dauerhaft gelöscht.",
    description: "Subtitle of the delete-disk card, stating what is lost.",
  },
  "services.diskUsageTitle": {
    message: "Disk-Nutzung",
    description: "Heading of the disk usage chart on a service's Disk tab.",
  },
  "services.diskUsageDescription": {
    message:
      "Auf das eingebundene Volume geschriebene Bytes im Verhältnis zur bereitgestellten Kapazität. Abgerechnet wird die bereitgestellte Größe, nicht die belegten Bytes.",
    description:
      "Subtitle of the disk usage chart, stating that billing uses provisioned size.",
  },
  "services.diskRestoreStarted": {
    message:
      "Disk wird wiederhergestellt. Der Service bleibt gestoppt, bis der Vorgang abgeschlossen ist.",
    description: "Toast after a restore is requested",
  },
  "services.navShell": {
    message: "Shell",
    description: "Service-detail nav item (running-instance SSH page)",
  },
  "services.navCompute": {
    message: "Rechenleistung",
    description:
      "Service sidebar link to the Compute page: the compute plan, manual scaling and autoscaling, as on Render (w6/m483)",
  },
  "services.eventsCount_one": {
    message: "{count} Ereignis",
    description:
      "Accessible label for the number of visible service events (singular)",
  },
  "services.eventsCount_other": {
    message: "{count} Ereignisse",
    description:
      "Accessible label for the number of visible service events (plural)",
  },
  "services.eventsCountAtLeast_one": {
    message: "{count}+ Ereignisse",
    description:
      "Accessible label for the event count badge while older events remain unread (singular)",
  },
  "services.eventsCountAtLeast_other": {
    message: "{count}+ Ereignisse",
    description:
      "Accessible label for the event count badge while older events remain unread (plural)",
  },
  "services.eventsEmptyTitle": {
    message: "Noch keine Ereignisse",
    description: "Events tab empty-state title",
  },
  "services.eventsEmpty": {
    message: "Deployments und Service-Änderungen erscheinen hier.",
    description: "Events tab empty-state description",
  },
  "services.eventsErrorTitle": {
    message: "Ereignisse sind nicht verfügbar",
    description: "Events tab query-error title",
  },
  "services.eventsErrorDescription": {
    message: "Die Ereignisse dieses Services konnten nicht geladen werden.",
    description: "Events tab query-error description",
  },
  "services.eventsRetry": {
    message: "Erneut versuchen",
    description: "Events tab query-error retry button",
  },
  "services.eventsLoadOlder": {
    message: "Ältere Ereignisse laden",
    description: "Button that loads the next page or earlier event window",
  },
  "services.eventsLoadingOlder": {
    message: "Ältere Ereignisse werden geladen…",
    description: "Busy label while an older service-event page is loading",
  },
  "services.eventsHistoryEnd": {
    message: "Du hast den Anfang des Verlaufs dieses Services erreicht.",
    description: "Terminal label after all retained service events are loaded",
  },
  "services.eventsFilter": {
    message: "Ereignisse filtern",
    description: "Button opening the service event-type filter",
  },
  "services.eventsFilterSelected": {
    message: "Ereignistypen: {count}",
    description: "Event filter button with selected-type count",
  },
  "services.eventsFilterSearch": {
    message: "Ereignistypen durchsuchen",
    description: "Search input for service event types",
  },
  "services.eventsFilterShowAll": {
    message: "Alle Ereignisse anzeigen",
    description:
      "Events tab: button in the filtered-to-nothing state that shows every event type again",
  },
  "services.eventsShownOf": {
    message: "{shown} von {total} angezeigt",
    description:
      "Events tab: count over the feed while a filter hides some rows; {total} is the loaded count, '20+' while older events remain",
  },
  "services.eventsFilterAll": {
    message: "Alle Ereignisse",
    description: "Master service event filter option",
  },
  "services.eventsFilterNoMatches": {
    message: "Keine Ereignistypen passen zu deiner Suche.",
    description: "Empty search result in the event filter",
  },
  "services.eventsFilterEmptyTitle": {
    message: "Keine passenden Ereignisse",
    description: "Title when loaded history has no selected event types",
  },
  "services.eventsFilterEmpty": {
    message: "Wähle weitere Ereignistypen oder lade älteren Verlauf.",
    description: "Body when loaded history has no selected event types",
  },
  "services.eventsFilterNoneTitle": {
    message: "Keine Ereignistypen ausgewählt",
    description:
      "Events tab: title when the filter has every event type deselected",
  },
  "services.eventsFilterNone": {
    message: "Wähle mindestens einen Typ, um Ereignisse zu sehen.",
    description:
      "Events tab: body when the filter has every event type deselected",
  },
  "services.eventsFilterGroup.deploy": {
    message: "Deployment",
    description: "Deploy event filter group",
  },
  "services.eventsFilterGroup.serviceStatus": {
    message: "Service-Status",
    description: "Service-status event filter group",
  },
  "services.eventsFilterGroup.scaling": {
    message: "Skalierung",
    description: "Scaling event filter group",
  },
  "services.eventsFilterGroup.maintenanceMode": {
    message: "Wartungsmodus",
    description: "Maintenance-mode event filter group",
  },
  "services.eventsFilterGroup.edgeCache": {
    message: "Edge-Cache",
    description: "Edge-cache event filter group",
  },
  "services.eventsFilterGroup.configuration": {
    message: "Konfiguration",
    description: "bex configuration event filter group",
  },
  "services.eventsFilterGroup.other": {
    message: "Sonstige",
    description:
      "Event filter group holding types the API returned that the dashboard catalog does not know yet",
  },
  "services.eventsActor": {
    message: "von {actor}",
    description: "Actor attribution shown on a service event",
  },
  "services.eventsDeployReference": {
    message: "Deployment {id}",
    description: "Deploy identifier shown on a deploy activity row",
  },
  "services.eventsTriggerRollback": {
    message: "Rollback",
    description: "Deploy event trigger: rollback",
  },
  "services.eventsTriggerFirstBuild": {
    message: "Erstes Deployment",
    description: "Deploy event trigger: the service's first deploy",
  },
  "services.eventsTriggerManual": {
    message: "Manuelles Deployment",
    description: "Deploy event trigger: manual",
  },
  "services.eventsTriggerEnvUpdated": {
    message: "Konfigurationsänderung",
    description:
      "Deploy event trigger: an environment change (env var, secret file, or env-group edit); a Settings save no longer sets envUpdated (w8/m135)",
  },
  "services.eventsTriggerDeployHook": {
    message: "Deploy Hook",
    description:
      "Deploy event trigger: a POST to the service's secret deploy-hook URL",
  },
  "services.eventsTriggerClearCache": {
    message: "Cache geleert",
    description: "Deploy event trigger: build-cache clear",
  },
  "services.eventsTriggerDeployedByRender": {
    message: "Plattform-Deployment",
    description: "Deploy event trigger: platform initiated",
  },
  "services.eventsTypeDeployStarted": {
    message: "Deployment gestartet",
    description: "Service activity type: deploy started",
  },
  "services.eventsTypeDeployFinished": {
    message: "Deployment beendet",
    description: "Service activity type: deploy finished",
  },
  "services.eventsTypeZeroDowntimeRedeployStarted": {
    message: "Unterbrechungsfreies Redeployment gestartet",
    description:
      "Service activity type: a Bex-initiated zero-downtime deploy started for a service",
  },
  "services.eventsTypeZeroDowntimeRedeployEnded": {
    message: "Unterbrechungsfreies Redeployment beendet",
    description:
      "Service activity type: a Bex-initiated zero-downtime deploy completed for a service",
  },
  "services.eventsRedeployTrigger.node_replacement": {
    message: "Knoten ersetzt",
    description:
      "zero_downtime_redeploy_started trigger: drain-guard rolled the service off a node being replaced",
  },
  "services.eventsRedeployTrigger.node_consolidation": {
    message: "Knoten durch Autoscaling entfernt",
    description:
      "zero_downtime_redeploy_started trigger: drain-guard rolled the service off a node marked for Cluster Autoscaler scale-down",
  },
  "services.eventsTypePipelineMinutesExhausted": {
    message: "Pipeline-Minuten aufgebraucht",
    description:
      "Service activity type: a build refused because the workspace used its pipeline minutes",
  },
  "services.eventsTypeEdgeCacheEnabled": {
    message: "Edge-Cache aktiviert",
    description: "Service activity type: edge caching turned on",
  },
  "services.eventsTypeEdgeCacheDisabled": {
    message: "Edge-Cache deaktiviert",
    description: "Service activity type: edge caching turned off",
  },
  "services.eventsTypeEdgeCachePurged": {
    message: "Edge-Cache geleert",
    description: "Service activity type: edge cache purged",
  },
  "services.eventsTypeImagePullFailed": {
    message: "Image-Pull fehlgeschlagen",
    description: "Service activity type: image pull failed",
  },
  "services.eventsTypeArtifactFetchFailed": {
    message: "Artefaktabruf fehlgeschlagen",
    description:
      "Service activity type: a build published by a build source could not be fetched",
  },
  "services.eventsTypeArtifactSourceChanged": {
    message: "Build-Quelle geändert",
    description:
      "Service activity type: the service's linked build source changed",
  },
  "services.eventsArtifactSourceChanged": {
    message: "Build-Quelle {from} → {to}",
    description:
      "Detail line on artifact_source_changed when switching between two build sources",
  },
  "services.eventsArtifactSourceLinked": {
    message: "Mit Build-Quelle {to} verknüpft",
    description:
      "Detail line on artifact_source_changed when newly linked to a build source",
  },
  "services.eventsArtifactSourceUnlinked": {
    message: "Verknüpfung mit Build-Quelle {from} getrennt",
    description:
      "Detail line on artifact_source_changed when detached from a build source",
  },
  "services.eventsBuildReference": {
    message: "Build {id}",
    description:
      "Build identifier shown on an artifact_fetch_failed activity row",
  },
  "services.eventsTypeSuspended": {
    message: "Pausiert",
    description: "Service activity type: service suspended",
  },
  "services.eventsTypeResumed": {
    message: "Fortgesetzt",
    description: "Service activity type: service resumed",
  },
  "services.eventsTypeRestarted": {
    message: "Server neu gestartet",
    description: "Service activity type: service restarted",
  },
  "services.eventsTypeInstanceFailed": {
    message: "Instanz fehlgeschlagen",
    description: "Service activity type: running instance became unhealthy",
  },
  "services.eventsTypeDiskUsageHigh": {
    message: "Disk-Auslastung hoch",
    description:
      "Service activity type: a service disk is over 80% full (service_disk_usage_high)",
  },
  "services.eventsTypeDiskUsageRecovered": {
    message: "Disk-Auslastung normalisiert",
    description:
      "Service activity type: a service disk dropped back below 80% (service_disk_usage_recovered)",
  },
  "services.eventsDiskUsage": {
    message:
      "Disk {disk} unter {mountPath}: {usage} % belegt (Schwelle {threshold} %)",
    description: "Disk usage event detail line",
  },
  "services.eventsTypeHardwareFailure": {
    message: "Hardwareausfall",
    description:
      "Service activity type: an instance was lost with the machine it ran on",
  },
  "services.eventsTypeServiceSuspended": {
    message: "Service pausiert",
    description: "Service activity type: suspension converged",
  },
  "services.eventsTypeServiceResumed": {
    message: "Service fortgesetzt",
    description: "Service activity type: resume converged",
  },
  "services.eventsTypeServiceHibernated": {
    message: "Service ist eingeschlafen",
    description:
      "Service activity type: free-tier idle auto-hibernation — deliberately worded apart from a user-driven suspension",
  },
  "services.eventsTypeServiceWoken": {
    message: "Service ist aufgewacht",
    description:
      "Service activity type: a sleeping free-tier service woke on an incoming request",
  },
  "services.eventsTypeServiceRecovered": {
    message: "Service erholt",
    description: "Service activity type: unhealthy service recovered",
  },
  "services.eventsTypePlanChanged": {
    message: "Instanztyp geändert",
    description: "Service activity type: plan changed",
  },
  "services.eventsTypeInstanceCountChanged": {
    message: "Instanzanzahl geändert",
    description: "Service activity type: manual scale",
  },
  "services.eventsTypeAutoscalingChanged": {
    message: "Autoscaling-Konfiguration geändert",
    description: "Service activity type: autoscaling configuration changed",
  },
  "services.eventsTypeAutoscalingStarted": {
    message: "Autoscaling gestartet",
    description: "Service activity type: autoscaler began a replica change",
  },
  "services.eventsTypeAutoscalingEnded": {
    message: "Autoscaling beendet",
    description: "Service activity type: autoscaler replica change converged",
  },
  "services.eventsTypeBranchChanged": {
    message: "Branch geändert",
    description: "Service activity type: tracked Git branch changed",
  },
  "services.eventsTypeCommitIgnored": {
    message: "Commit ignoriert",
    description: "Service activity type: matching Git push did not deploy",
  },
  "services.eventsTypeJobStarted": {
    message: "Job gestartet",
    description: "Service activity type: one-off job started",
  },
  "services.eventsTypeJobCanceled": {
    message: "Job abgebrochen",
    description: "Service activity type: one-off job canceled",
  },
  "services.eventsTypeJobRunEnded": {
    message: "Job beendet",
    description: "Service activity type: one-off job finished",
  },
  "services.eventsTypeBuildStarted": {
    message: "Build gestartet",
    description: "Service activity type: image build started",
  },
  "services.eventsTypeBuildEnded": {
    message: "Build beendet",
    description: "Service activity type: image build finished",
  },
  "services.eventsTypePreDeployStarted": {
    message: "Pre-Deploy gestartet",
    description: "Service activity type: pre-deploy command started",
  },
  "services.eventsTypePreDeployEnded": {
    message: "Pre-Deploy beendet",
    description: "Service activity type: pre-deploy command finished",
  },
  "services.eventsTypeBranchDeleted": {
    message: "Branch gelöscht",
    description: "Service activity type: tracked Git branch deleted",
  },
  "services.eventsTypeMaintenanceModeChanged": {
    message: "Wartungsmodus-Konfiguration geändert",
    description: "Service activity type: maintenance mode setting changed",
  },
  "services.eventsTypeMaintenanceModeUriUpdated": {
    message: "Wartungsmodus-URI aktualisiert",
    description: "Service activity type: maintenance mode URI changed",
  },
  "services.eventsInstanceReference": {
    message: "Instanz {id}",
    description: "Instance identifier shown on a service event",
  },
  "services.eventsReplicaChange": {
    message: "Instanzen {from} → {to}",
    description:
      "Instance count from/to summary: an autoscaling start/end's replicas or an instance_count_changed event's counts",
  },
  "services.eventsAutoscalingChange": {
    message: "Autoscaling {from} → {to}",
    description:
      "autoscaling_config_changed detail line; from/to are services.eventsAutoscalingOff or services.eventsAutoscalingRange (w4/568)",
  },
  "services.eventsAutoscalingOff": {
    message: "Aus",
    description: "One side of an autoscaling change: autoscaling off (w4/568)",
  },
  "services.eventsAutoscalingRange": {
    message: "{min}–{max} Instanzen",
    description:
      "One side of an autoscaling change: its minimum–maximum instances (w4/568)",
  },
  "services.eventsAutoscalingRangeCPU": {
    message: "{min}–{max} Instanzen, CPU {cpu} %",
    description:
      "One side of an autoscaling change: its instance range and CPU target (w4/m335)",
  },
  "services.eventsAutoscalingRangeMemory": {
    message: "{min}–{max} Instanzen, Arbeitsspeicher {memory} %",
    description:
      "One side of an autoscaling change: its instance range and memory target (w4/m335)",
  },
  "services.eventsAutoscalingRangeCPUMemory": {
    message: "{min}–{max} Instanzen, CPU {cpu} %, Arbeitsspeicher {memory} %",
    description:
      "One side of an autoscaling change: its instance range and both targets (w4/m335)",
  },
  "services.eventsPlanChange": {
    message: "Compute-Plan {from} → {to}",
    description:
      "plan_changed detail line, by the compute plans' names (w4/568, w6/m483)",
  },
  "services.eventsBranchChange": {
    message: "Branch {from} → {to}",
    description: "Tracked branch from/to summary",
  },
  "services.eventsAutoDeployDisabledReason.setting_change": {
    message: "In den Einstellungen deaktiviert",
    description:
      "auto_deploy_disabled reason: the auto-deploy setting was turned off (w4/m431)",
  },
  "services.eventsAutoDeployDisabledReason.rollback": {
    message:
      "Durch ein Rollback deaktiviert, damit spätere Pushes es nicht ersetzen",
    description:
      "auto_deploy_disabled reason: a dashboard rollback turned auto-deploy off (w4/m431)",
  },
  "services.eventsAutoDeployDisabledReason.manual_deploy": {
    message:
      "Durch das Deployen eines bestimmten Commits deaktiviert, damit spätere Pushes ihn nicht ersetzen",
    description:
      "auto_deploy_disabled reason: a deploy of a specific commit from the dashboard or a deploy hook turned auto-deploy off (w4/m431)",
  },
  "services.eventsBranchDeleted": {
    message: "Branch {branch} gelöscht — Auto-Deploy deaktiviert",
    description: "Deleted tracked branch summary",
  },
  "services.eventsStatus.succeeded": {
    message: "Erfolgreich",
    description: "Lifecycle-step outcome badge: succeeded",
  },
  "services.eventsStatus.failed": {
    message: "Fehlgeschlagen",
    description: "Lifecycle-step outcome badge: failed",
  },
  "services.eventsStatus.canceled": {
    message: "Abgebrochen",
    description: "Lifecycle-step outcome badge: canceled",
  },
  "services.eventsReason.superseded": {
    message: "Ein neueres Deployment hat dieses Deployment ersetzt.",
    description:
      "Deploy/build ended reason: a newer deploy superseded this one",
  },
  "services.eventsReason.image_pull_backoff": {
    message: "Die Runtime konnte das konfigurierte Image nicht laden.",
    description: "Bounded public image-pull failure reason",
  },
  "services.eventsReason.readiness_failed": {
    message: "Eine laufende Instanz hat ihren Health Check nicht bestanden.",
    description:
      "Bounded public readiness failure reason; names the health check, as Settings and REST do (w4/m327)",
  },
  "services.eventsReason.root_directory": {
    message: "Keine geänderte Datei passte zum Stammverzeichnis des Services.",
    description: "Commit ignored because of root-directory scoping",
  },
  "services.eventsReason.build_filter": {
    message:
      "Die geänderten Dateien haben die Build-Filter des Services nicht bestanden.",
    description: "Commit ignored because of build filters",
  },
  "services.eventsReason.skip_phrase": {
    message:
      "Die Commit-Nachricht hat angefordert, dieses Deployment zu überspringen.",
    description: "Commit ignored because of a Render skip phrase",
  },
  "services.eventsReason.non_zero_exit": {
    message: "Die Ausführung wurde mit Status {exitCode} beendet.",
    description:
      "Cron run failure reason: the container exited non-zero (w4/m114)",
  },
  "services.eventsReason.oom_killed": {
    message:
      "Der Ausführung ging der Arbeitsspeicher aus und sie wurde gestoppt.",
    description:
      "Cron run failure reason: the container was OOM-killed (w4/m114)",
  },
  "services.eventsReason.instance.non_zero_exit": {
    message:
      "Der Prozess der Instanz wurde mit Status {exitCode} beendet und neu gestartet.",
    description:
      "Instance failed reason for a running service: its process crashed (w8/m158)",
  },
  "services.eventsReason.instance.oom_killed": {
    message:
      "Der Instanz ging der Arbeitsspeicher aus ({memory} Limit) und sie wurde neu gestartet.",
    description:
      "Instance failed reason for a running service: killed at its plan's memory limit (w8/m158)",
  },
  "services.eventsReason.instance.readiness_failed_result": {
    message:
      "Eine laufende Instanz hat ihren Health Check nicht bestanden: {result}.",
    description:
      'Instance failed reason for a running service whose health check failed, naming what the check got back (w6/m634); {result} is Render\'s English message ("HTTP health check failed with status code 503"), shown verbatim in every language as Render shows it',
  },
  "services.eventsReason.instance.oom_killed_no_limit": {
    message:
      "Der Instanz ging der Arbeitsspeicher aus und sie wurde neu gestartet.",
    description:
      "Instance failed reason for a running service: killed for memory, limit unknown (w8/m158)",
  },
  "services.eventsReason.early_exit": {
    message:
      "Der Prozess der Instanz wurde beendet und neu gestartet. Ein Service soll dauerhaft laufen.",
    description:
      "Instance failed reason for a running service: its process ended with status 0 (w8/m158)",
  },
  "services.eventsReason.evicted": {
    message:
      "Die Ausführung wurde von ihrem Node verdrängt, bevor sie fertig war.",
    description: "Cron run failure reason: the run's pod was evicted (w4/m114)",
  },
  "services.eventsReason.timed_out": {
    message:
      "Die Ausführung hat das 12-Stunden-Limit erreicht und wurde gestoppt.",
    description:
      "Cron run failure reason: the run exceeded its deadline (w4/m114)",
  },
  "services.eventsTypeCronRunStarted": {
    message: "Cron-Ausführung gestartet",
    description: "Service activity type: cron run started",
  },
  "services.eventsTypeCronRunFinished": {
    message: "Cron-Ausführung abgeschlossen",
    description: "Service activity type: cron run finished",
  },
  "services.eventsTypeEnvVarsChanged": {
    message: "Umgebungsvariablen geändert",
    description: "Service activity type: environment variables changed",
  },
  "services.eventsTypeEnvironmentChanged": {
    message: "Umgebungskonfiguration geändert",
    description:
      "Service activity type: environment variables and secret files changed",
  },
  "services.eventsTypeServiceMoved": {
    message: "Service verschoben",
    description:
      "Service activity type: the service was reassigned between projects or environments (service_moved) — distinct from the env-var fact above",
  },
  "services.eventsProjectChange": {
    message: "Projekt {from} → {to}",
    description:
      "service_moved detail line: previous and new project name (its id once it no longer exists), or the placement-none placeholder (w4/561)",
  },
  "services.eventsEnvironmentChange": {
    message: "Umgebung {from} → {to}",
    description:
      "service_moved detail line: previous and new environment name (its id once it no longer exists), or the placement-none placeholder (w4/561)",
  },
  "services.eventsPlacementNone": {
    message: "keine",
    description:
      "Placeholder for an absent side of a service_moved event (assigned from / removed to nothing) or of a plan_changed event (no plan before)",
  },
  "services.eventsTypeEnvGroupLinked": {
    message: "Umgebungsgruppe verknüpft",
    description: "Service activity type: environment group linked",
  },
  "services.eventsTypeEnvGroupUnlinked": {
    message: "Umgebungsgruppe getrennt",
    description: "Service activity type: environment group unlinked",
  },
  "services.eventsEnvGroupLinkedNamed": {
    message: "Umgebungsgruppe {name} verknüpft",
    description:
      "Service activity title for env_group_linked naming the group, by its name at the time (w4/563)",
  },
  "services.eventsEnvGroupUnlinkedNamed": {
    message: "Umgebungsgruppe {name} getrennt",
    description:
      "Service activity title for env_group_unlinked naming the group, by its name at the time (w4/563)",
  },
  "services.eventsCustomDomainAddedNamed": {
    message: "Benutzerdefinierte Domain {name} hinzugefügt",
    description:
      "Service activity title for custom_domain_added naming the domain's hostname (w4/m337)",
  },
  "services.eventsCustomDomainRemovedNamed": {
    message: "Benutzerdefinierte Domain {name} entfernt",
    description:
      "Service activity title for custom_domain_removed naming the domain's hostname (w4/m337)",
  },
  "services.eventsCustomDomainVerifiedNamed": {
    message: "Benutzerdefinierte Domain {name} verifiziert",
    description:
      "Service activity title for custom_domain_verified naming the domain's hostname (w4/m337)",
  },
  "services.eventsDisplayNameChange": {
    message: "Name {from} → {to}",
    description:
      "display_name_changed detail line: the service's name before and after the rename (w4/m337)",
  },
  "services.eventsTypeAutoDeployChanged": {
    message: "Auto-Deploy aktualisiert",
    description: "Service activity type: auto-deploy setting changed",
  },
  "services.eventsTypeIdleTimeoutChanged": {
    message: "Leerlauf-Timeout aktualisiert",
    description: "Service activity type: idle timeout changed",
  },
  "services.eventsTypeDisplayNameChanged": {
    message: "Anzeigename geändert",
    description: "Service activity type: display name changed",
  },
  "services.eventsTypeCustomDomainAdded": {
    message: "Benutzerdefinierte Domain hinzugefügt",
    description: "Service activity type: custom domain added",
  },
  "services.eventsTypeCustomDomainRemoved": {
    message: "Benutzerdefinierte Domain entfernt",
    description: "Service activity type: custom domain removed",
  },
  "services.eventsTypeCustomDomainVerified": {
    message: "Benutzerdefinierte Domain verifiziert",
    description: "Service activity type: custom domain verified",
  },
  "services.eventsTypeDiskCreated": {
    message: "Disk erstellt",
    description: "Service activity type: persistent disk created",
  },
  "services.eventsTypeDiskUpdated": {
    message: "Disk aktualisiert",
    description: "Service activity type: persistent disk updated",
  },
  "services.eventsTypeDiskDeleted": {
    message: "Disk gelöscht",
    description: "Service activity type: persistent disk deleted",
  },
  "services.eventsTypeDiskSnapshotRestored": {
    message: "Disk-Snapshot wiederhergestellt",
    description:
      "Service activity type: persistent disk restored from a snapshot",
  },
  "services.eventsTypeNotificationsChanged": {
    message: "Fehler-Benachrichtigungen aktualisiert",
    description: "Service activity type: failure notification setting changed",
  },
  "services.eventsTypeSubdomainPolicyChanged": {
    message: "Plattform-Subdomain aktualisiert",
    description: "Service activity type: platform subdomain policy changed",
  },
  "services.eventsTypeStaticSiteChanged": {
    message: "Static-Site-Einstellungen geändert",
    description: "Service activity type: static-site configuration changed",
  },
  "services.eventsFilterType.autoDeployEnabled": {
    message: "Auto-Deploy aktiviert",
    description: "Events filter option for the auto deploy enabled event type",
  },
  "services.eventsFilterType.autoDeployDisabled": {
    message: "Auto-Deploy deaktiviert",
    description: "Events filter option for the auto deploy disabled event type",
  },
  "services.eventsFilterType.autoDeployChanged": {
    message: "Auto-Deploy aktualisiert (veraltet)",
    description: "Events filter option for the auto deploy changed event type",
  },
  "services.eventsFilterType.publishPathChanged": {
    message: "Veröffentlichungsverzeichnis geändert",
    description: "Events filter option for the publish path changed event type",
  },
  "services.eventsFilterType.routesChanged": {
    message: "Weiterleitungen und Rewrites geändert",
    description: "Events filter option for the routes changed event type",
  },
  "services.eventsFilterType.headersChanged": {
    message: "Header geändert",
    description: "Events filter option for the headers changed event type",
  },
  "services.eventsFilterType.rootDirectoryChanged": {
    message: "Stammverzeichnis geändert",
    description:
      "Events filter option for the root directory changed event type",
  },
  "services.eventsFilterType.dockerfilePathChanged": {
    message: "Dockerfile-Pfad geändert",
    description:
      "Events filter option for the dockerfile path changed event type",
  },
  "services.eventsFilterType.dockerContextChanged": {
    message: "Docker-Build-Kontext geändert",
    description:
      "Events filter option for the docker context changed event type (w8/m128)",
  },
  "services.eventsFilterType.buildFilterChanged": {
    message: "Build-Filter geändert",
    description: "Events filter option for the build filter changed event type",
  },
  "services.eventsFilterType.commandsChanged": {
    message: "Befehle geändert",
    description: "Events filter option for the commands changed event type",
  },
  "services.eventsFilterType.sourceChanged": {
    message: "Quelle geändert",
    description: "Events filter option for the source changed event type",
  },
  "services.eventsFilterType.preDeployCommandChanged": {
    message: "Pre-Deploy-Befehl geändert",
    description:
      "Events filter option for the pre deploy command changed event type",
  },
  "services.eventsFilterType.maxShutdownDelayChanged": {
    message: "Maximale Shutdown-Verzögerung geändert",
    description:
      "Events filter option for the max shutdown delay changed event type",
  },
  "services.eventsFilterType.deployHookRegenerated": {
    message: "Deploy Hook neu generiert",
    description:
      "Events filter option for the deploy hook regenerated event type",
  },
  "services.eventsTypeBuildSettingsChanged": {
    message: "Build- und Deployment-Einstellungen geändert",
    description: "Service activity type: build or deploy configuration changed",
  },
  "services.eventsTypePortChanged": {
    message: "Port geändert",
    description: "Service activity type: the service's listening port changed",
  },
  "services.eventsTypeServiceChanged": {
    message: "Service-Einstellungen geändert",
    description: "Fallback service activity type",
  },
  "services.eventsManualDeploy": {
    message: "Manuelles Deployment",
    description: "Button to trigger a new deploy",
  },
  "services.deployMenuLatestCommit": {
    message: "Neuesten Commit deployen",
    description:
      "Manual Deploy dropdown item, repo-backed service: rebuild and redeploy from the branch's HEAD",
  },
  "services.deployMenuLatestImage": {
    message: "Neuestes Image deployen",
    description:
      "Manual Deploy dropdown item, image-backed service (no repo to rebuild from)",
  },
  "services.deployMenuClearCache": {
    message: "Build-Cache leeren und deployen",
    description:
      "Manual Deploy dropdown item (Render parity): rebuild without importing prior build-cache layers when registry caching is enabled; still exports a fresh cache for later deploys",
  },
  "services.deployMenuRestart": {
    message: "Service neu starten",
    description:
      "Manual Deploy dropdown item: restart on the commit or image that is running; never deploys newer commits",
  },
  "services.deployMenuSpecificCommit": {
    message: "Bestimmten Commit deployen",
    description:
      "Manual Deploy dropdown item, repo-backed service (Render parity): deploy a commit by SHA and turn auto-deploy off",
  },
  "services.deployCommitTitle": {
    message: "Bestimmten Commit deployen",
    description: "Title of the dialog that deploys a commit by SHA",
  },
  "services.deployCommitBody": {
    message:
      "Deployt den eingegebenen Commit und deaktiviert Auto-Deploy, damit spätere Pushes ihn nicht ersetzen. Du kannst Auto-Deploy in den Einstellungen wieder aktivieren.",
    description:
      "Body of the deploy-a-specific-commit dialog. Render's dashboard disables automatic deploys when a specific commit is deployed",
  },
  "services.deployCommitLabel": {
    message: "Commit-SHA",
    description:
      "Label of the commit SHA input in the deploy-a-specific-commit dialog",
  },
  "services.deployCommitInvalid": {
    message: "Gib einen Commit-SHA ein: 7 bis 40 Hexadezimalzeichen.",
    description: "Inline error when the entered commit SHA is malformed",
  },
  "services.deployCommitConfirm": {
    message: "Commit deployen",
    description: "Confirm button of the deploy-a-specific-commit dialog",
  },
  "services.restartServiceSuccess": {
    message: "Neustart des laufenden Releases gestartet.",
    description: "Toast after the header's Restart service is accepted",
  },
  "services.restartServiceError": {
    message: "Der Service konnte nicht neu gestartet werden.",
    description:
      "Toast when the header's Restart service fails and the server gave no reason",
  },
  "services.eventsCancelDeploy": {
    message: "Abbrechen",
    description: "Button to cancel an in-progress deploy",
  },
  "services.eventsCancelConfirmTitle": {
    message: "Dieses Deployment abbrechen?",
    description: "Cancel deploy confirm dialog title",
  },
  "services.eventsCancelConfirmBody": {
    message:
      "Das laufende Deployment wird gestoppt. Wenn ein früheres Deployment erfolgreich war, bleibt es live; andernfalls wird nichts ausgeliefert. Gespeicherte Änderungen bleiben erhalten und werden mit dem nächsten Deployment ausgeliefert.",
    description:
      "Cancel deploy confirm dialog body when it is unknown whether an earlier deploy is live, so both outcomes are stated (w4/334)",
  },
  "services.eventsCancelConfirmBodyFirstDeploy": {
    message:
      "Das laufende Deployment wird gestoppt. Dies ist das erste Deployment des Services, daher ist noch keine Version live — durch das Abbrechen liefert der Service nichts aus.",
    description:
      "Cancel deploy confirm dialog body — the service's first deploy, nothing is serving yet (w4/103)",
  },
  "services.eventsRollback": {
    message: "Rollback",
    description: "Button to roll back to a specific deploy",
  },
  "services.eventsRollbackAria": {
    message: "Rollback auf {target}",
    description:
      "Accessible name of a deploy row's Rollback control, naming the deploy it restores (w4/m141)",
  },
  "services.eventsCancelDeployAria": {
    message: "Deployment {target} abbrechen",
    description:
      "Accessible name of a deploy row's Cancel control, naming the deploy (w4/m141)",
  },
  "services.eventsConfirmProceed": {
    message: "Fortfahren",
    description: "Confirm dialog proceed button",
  },
  "services.eventsConfirmCancel": {
    message: "Zurück",
    description: "Confirm dialog cancel button",
  },
  "services.triggerDeploySuccess": {
    message: "Deployment ausgelöst.",
    description: "Toast after triggerDeploy succeeds",
  },
  "services.triggerDeployError": {
    message: "Deployment konnte nicht ausgelöst werden.",
    description: "Toast after triggerDeploy fails",
  },
  "services.cancelDeploySuccess": {
    message: "Deployment abgebrochen.",
    description: "Toast after cancelDeploy succeeds",
  },
  "services.cancelDeployError": {
    message: "Deployment konnte nicht abgebrochen werden.",
    description: "Toast after cancelDeploy fails",
  },
  "services.rollbackSuccess": {
    message: "Rollback ausgelöst.",
    description: "Toast after rollbackService succeeds",
  },
  "services.rollbackError": {
    message: "Rollback ist fehlgeschlagen.",
    description: "Toast after rollbackService fails",
  },
  "services.eventsStatusLive": {
    message: "Live",
    description: "Deploy status: live",
  },
  "services.eventsStatusInProgress": {
    message: "In Bearbeitung",
    description: "Deploy status: update_in_progress",
  },
  "services.eventsStatusFailed": {
    message: "Fehlgeschlagen",
    description: "Deploy status: update_failed",
  },
  "services.eventsStatusCanceled": {
    message: "Abgebrochen",
    description: "Deploy status: canceled",
  },
  "services.eventsPreDeployRunning": {
    message: "Pre-Deploy-Befehl läuft",
    description: "Deploy row: the pre-deploy step is in progress",
  },
  "services.eventsPreDeploySucceeded": {
    message: "Pre-Deploy-Befehl erfolgreich",
    description: "Deploy row: the pre-deploy step passed",
  },
  "services.eventsPreDeployCanceled": {
    message: "Pre-Deploy-Befehl abgebrochen",
    description:
      "Deploy row: the deploy ended (superseded or canceled) while its pre-deploy step was still running",
  },
  "services.eventsPreDeployFailed": {
    message: "Pre-Deploy-Befehl fehlgeschlagen",
    description:
      "Deploy row: the pre-deploy step failed (distinct from a health-check failure)",
  },
  "services.eventsRolledBackFrom": {
    message: "Zurückgesetzt von {target}",
    description: "Deploy row: provenance note when trigger=rollback",
  },
  "services.capLimitTitle": {
    message: "Service-Limit erreicht",
    description:
      "Alert title when the workspace's service creation cap is hit (w7/m9)",
  },
  "services.capLimitUpgrade": {
    message: "Tarif upgraden",
    description: "Upgrade CTA button inside the cap-limit Alert (w7/m9)",
  },
  "services.networkingTitle": {
    message: "Netzwerk",
    description: "Settings Networking card title (w7/m32)",
  },
  "services.networkingDescription": {
    message: "Beschränke eingehenden HTTP-Traffic auf diese Quell-CIDRs.",
    description: "Settings Networking card description (w7/m32)",
  },
  "services.networkingScalePlanOnly": {
    message:
      "Eingehende IP-Regeln für Web Services und Static Sites erfordern den Tarif Scale oder höher. Bereits festgelegte Regeln bleiben aktiv und können durch die Regel „alles erlauben“ ersetzt werden.",
    description:
      "Hint replacing the editable IP rules below the Scale plan (w8/m137)",
  },
  "services.networkingClearAll": {
    message: "Gesamten Traffic erlauben",
    description:
      "Button replacing every inbound IP rule with the allow-all rule below the Scale plan (w8/m137, w8/m157)",
  },
  "services.networkingHint": {
    message:
      "Gib einen CIDR-Block ein (z. B. 203.0.113.0/24) und drücke auf Hinzufügen. Ohne Regeln wird gesamter eingehender Traffic blockiert; die Regel 0.0.0.0/0 erlaubt alle Quell-IPs.",
    description: "Hint below the CIDR list in the Networking card (w7/m32)",
  },
  "services.networkingOpen": {
    message: "Offen für alle Quell-IPs",
    description:
      "Shown below the Scale plan when the service has only the allow-all rule (Render default, w7/m32, w8/m157)",
  },
  "services.networkingBlocked": {
    message:
      "Keine Regeln: Gesamter eingehender Traffic zu diesem Service wird blockiert. Füge 0.0.0.0/0 hinzu, um alle Quell-IPs zu erlauben.",
    description:
      "Warning shown when a service's inbound IP rule list is empty, saved or about to be saved (w8/m157)",
  },
  "services.networkingAdd": {
    message: "Allowlist-Eintrag hinzufügen",
    description: "Button to add a CIDR to the draft list (w7/m32)",
  },
  "services.networkingEntryDescription": {
    message: "Beschreibung (optional)",
    description: "Placeholder for a service allowlist entry description",
  },
  "services.networkingCIDR": {
    message: "Neuer CIDR-Block",
    description: "Accessible label for the add-row CIDR input",
  },
  "services.networkingCIDRRule": {
    message: "CIDR-Block für Regel {number}",
    description: "Accessible label for an existing allowlist CIDR row",
  },
  "services.networkingDescriptionLabel": {
    message: "Beschreibung der neuen Regel",
    description: "Accessible label for the add-row description input",
  },
  "services.networkingDescriptionRule": {
    message: "Beschreibung für Regel {number}",
    description: "Accessible label for an existing allowlist description row",
  },
  "services.networkingInvalid": {
    message: "Gib einen gültigen IPv4- oder IPv6-CIDR-Block ein.",
    description: "Validation error for an invalid service allowlist CIDR",
  },
  "services.networkingSave": {
    message: "Allowlist speichern",
    description: "Button to persist the CIDR list (w7/m32)",
  },
  "services.networkingRemove": {
    message: "{cidr} entfernen",
    description:
      "Accessible label on the trash icon next to a CIDR tag (w7/m32)",
  },
  "services.networkingMoveUp": {
    message: "{cidr} nach oben verschieben",
    description: "Accessible label to move an allowlist entry earlier",
  },
  "services.networkingMoveDown": {
    message: "{cidr} nach unten verschieben",
    description: "Accessible label to move an allowlist entry later",
  },
  "services.networkingSaved": {
    message: "IP-Allowlist aktualisiert",
    description: "Toast on successful setServiceIpAllowList mutation (w7/m32)",
  },
  "services.networkingError": {
    message:
      "Die IP-Allowlist konnte nicht aktualisiert werden. Bitte versuche es erneut.",
    description: "Toast on failed setServiceIpAllowList mutation (w7/m32)",
  },
  "services.outboundIpsTitle": {
    message: "Ausgehende IPs",
    description: "Settings card title for service egress IPs (w8/010)",
  },
  "services.outboundIpsDescription": {
    message:
      "Gemeinsam genutzte Egress-Adressen für diesen Service. Es sind die aktuellen öffentlichen Node-IPs des Tenant-Pools; sie können sich ändern, wenn der Autoscaler Nodes hinzufügt oder entfernt.",
    description:
      "Outbound IPs card helper — truthful shared-pool semantics (w8/010)",
  },
  "services.outboundIpsEmpty": {
    message:
      "Noch keine ausgehenden IPs gemeldet (üblich bei lokalen Clustern ohne Node-ExternalIPs).",
    description: "Honest empty state when outboundIps.ips is empty (w8/010)",
  },
  "services.outboundIpsCopy": {
    message: "{ip} kopieren",
    description:
      "Accessible label on the copy button for one outbound IP (w8/010)",
  },
  "services.outboundIpsCopied": {
    message: "Kopiert",
    description: "Toast after copying an outbound IP (w8/010)",
  },
  "services.outboundIpsCopyError": {
    message: "Kopieren fehlgeschlagen",
    description: "Toast when clipboard write fails for an outbound IP (w8/010)",
  },
  "services.environmentPageDescription": {
    message:
      "Alles, was dieser Service zur Laufzeit liest. Werte bleiben verdeckt, bis du einen anzeigst.",
    description: "Service Environment page introduction",
  },
  "services.environmentEdit": {
    message: "Bearbeiten",
    description: "Enter the coherent service environment draft",
  },
  "services.environmentMaskedValue": {
    message: "Maskierter Secret-Wert",
    description: "Accessible label for an unrevealed secret",
  },
  "services.environmentUnchangedMasked": {
    message: "Unverändert (maskiert)",
    description: "Placeholder for an opaque unchanged draft value",
  },
  "services.environmentDuplicateKey": {
    message: "Jeder Variablenschlüssel muss eindeutig sein.",
    description: "Draft validation for a duplicate environment key",
  },
  "services.environmentValueRequired": {
    message: "Gib einen Wert für diese neue Variable ein.",
    description: "Draft validation for a new variable without a value",
  },
  "services.environmentValueNul": {
    message:
      "Ein Wert darf kein NUL-Byte enthalten, das eine Prozessumgebung nicht aufnehmen kann. Verwende für Binärinhalte eine Secret-Datei.",
    description:
      "Draft validation under an environment variable value with a NUL byte (w21/047)",
  },
  "services.environmentValueTooLarge": {
    message:
      "Ein KEY=value-Eintrag darf höchstens 131.071 Bytes groß sein: Ein Prozess kann mit mehr nicht starten. Verwende für große Inhalte eine Secret-Datei.",
    description:
      "Draft validation under an environment variable whose KEY=value entry is over the process limit; the server counts UTF-8 bytes, so a Chinese character is 3 (w21/047)",
  },
  "services.environmentPortInvalid": {
    message:
      "PORT muss eine ganze Zahl von 1 bis 65535 sein und darf nicht 18012, 18013 oder 19099 lauten, die von der Plattform reserviert sind.",
    description:
      "Draft validation under a PORT environment variable the operator would ignore (w21/047)",
  },
  "services.environmentLimit": {
    message:
      "Umgebungsvariablen sind auf 500 und insgesamt 512 KiB pro Service oder Gruppe begrenzt.",
    description:
      "Draft validation when the environment variables would exceed bex-api's per-service quota",
  },
  "services.environmentStagedDelete": {
    message: "Wird entfernt",
    description: "Badge on a staged environment deletion",
  },
  "services.environmentUndo": {
    message: "Rückgängig",
    description: "Undo a staged environment deletion",
  },
  "services.environmentUnsavedTitle": {
    message: "Nicht gespeicherte Änderungen",
    description: "Combined draft save bar heading",
  },
  "services.environmentRestoredTitle": {
    message: "Nicht gespeicherte Änderungen wiederhergestellt",
    description:
      "Heading of the banner shown when an unsaved env draft is recovered in this tab, after a reload or a session-expiry sign-in (w3/m80)",
  },
  "services.environmentRestoredBody": {
    message:
      "Deine nicht gespeicherten Umgebungsänderungen wurden wiederhergestellt. Prüfe sie und speichere oder verwirf sie dann.",
    description:
      "Body of the restored-draft banner. Names no cause: an ordinary reload restores the draft as a sign-in does (w4/337)",
  },
  "services.environmentRestoredDiscard": {
    message: "Wiederhergestellte Änderungen verwerfen",
    description: "Button that throws away a recovered env draft",
  },
  "services.environmentUnsavedVariables_one": {
    message: "{count} Variable geändert",
    description:
      "Draft variable-operation count, composed with the file count at the call site (singular)",
  },
  "services.environmentUnsavedVariables_other": {
    message: "{count} Variablen geändert",
    description:
      "Draft variable-operation count, composed with the file count at the call site (plural)",
  },
  "services.environmentUnsavedFiles_one": {
    message: "{count} Datei geändert",
    description:
      "Draft file-operation count, composed with the variable count at the call site (singular)",
  },
  "services.environmentUnsavedFiles_other": {
    message: "{count} Dateien geändert",
    description:
      "Draft file-operation count, composed with the variable count at the call site (plural)",
  },
  "services.environmentSaveOptions": {
    message: "Speicheroptionen für die Umgebung",
    description: "Accessible label for the split save menu",
  },
  "services.environmentSaveOnly": {
    message: "Nur speichern",
    description: "Persist and project without rolling the service",
  },
  "services.environmentSaveDeploy": {
    message: "Speichern und deployen",
    description: "Persist and roll the current image once",
  },
  "services.environmentSaveRebuild": {
    message: "Speichern, neu bauen und deployen",
    description: "Persist then start one source build and deploy",
  },
  "services.environmentSaveOnlySuccess": {
    message:
      "Umgebung ohne Deployment gespeichert. Verwende ein reguläres Deployment, um ausstehende Änderungen anzuwenden.",
    description:
      "Save-only success feedback while the operator checks saved/runtime differences; does not assert that a no-op or revert has pending changes",
  },
  "services.environmentSaveDeploySuccess": {
    message: "Umgebung gespeichert und Deployment gestartet",
    description: "Toast after save-and-deploy succeeds",
  },
  "services.environmentSavedRefreshFailed": {
    message:
      "Umgebung gespeichert, aber diese Seite konnte nicht aktualisiert werden. Lade sie neu, um die neuesten Werte und den Deployment-Status zu prüfen.",
    description:
      "A committed environment patch must not be retried because a follow-up read failed",
  },
  "services.environmentSaveError": {
    message:
      "Die Umgebung konnte nicht gespeichert werden. Dein Entwurf ist weiterhin vorhanden.",
    description: "Batch environment save failure",
  },
  "services.environmentDraftStale": {
    message:
      "Diese Umgebungsgruppe hat sich möglicherweise geändert, seit du den Entwurf begonnen hast, daher wurde der Entwurf nicht gespeichert. Kopiere, was du brauchst, verwirf den Entwurf und bearbeite erneut, um von der neuesten Version auszugehen.",
    description:
      "Refusal when a group draft's base revision is no longer current (ENV_GROUP_REVISION_CONFLICT) or predates revision tracking (w4/m161)",
  },
  "services.environmentSavedDeployFailedTitle": {
    message: "Konfiguration gespeichert; Rollout unvollständig",
    description: "Partial rebuild failure heading",
  },
  "services.environmentSavedDeployFailedBody": {
    message:
      "Deine Umgebungsänderungen sind gespeichert. Wiederhole nur den unvollständigen Rollout, wenn du bereit bist.",
    description: "Partial rebuild failure recovery explanation",
  },
  "services.environmentRetryDeploy": {
    message: "Rollout wiederholen",
    description: "Retry only the failed deploy phase",
  },
  "services.environmentDiscardTitle": {
    message: "Umgebungsänderungen verwerfen?",
    description: "Dirty navigation guard title",
  },
  "services.environmentDiscardBody": {
    message:
      "Diese Änderungen sind nicht gespeichert und gehen verloren, wenn du die Seite verlässt.",
    description: "Dirty navigation guard explanation",
  },
  "services.environmentKeepEditing": {
    message: "Weiter bearbeiten",
    description: "Cancel dirty navigation",
  },
  "services.environmentDiscard": {
    message: "Verwerfen und verlassen",
    description: "Confirm dirty navigation",
  },
  "services.envReveal": {
    message: "Anzeigen",
    description: "Reveal one masked environment value",
  },
  "services.envRevealOne": {
    message: "{name} anzeigen",
    description:
      'aria-label of the per-row Reveal button for one env var or secret file; starts with the visible text "Reveal" (w4/499)',
  },
  "services.envHideOne": {
    message: "{name} ausblenden",
    description:
      'aria-label of the per-row Hide button for one env var or secret file; starts with the visible text "Hide" (w4/499)',
  },
  "services.envHide": {
    message: "Ausblenden",
    description: "Mask one revealed environment value",
  },
  "services.envCopy": {
    message: "Umgebungsvariablen kopieren",
    description: "Copy the complete dotenv export",
  },
  "services.envDownload": {
    message: ".env herunterladen",
    description: "Download the complete dotenv export",
  },
  "services.envCopySuccess": {
    message: "Umgebung kopiert",
    description: "Toast after the complete dotenv export is copied",
  },
  "services.envCopyOne": {
    message: "{name} kopieren",
    description:
      "aria-label of the per-row copy button for one env var or secret file — distinct from the bulk envCopy dotenv export",
  },
  "services.envCopyOneSuccess": {
    message: "{name} kopiert",
    description:
      "Toast after one env var value or secret file's contents is copied — distinct from the bulk envCopySuccess",
  },
  "services.envAddVariable": {
    message: "Variable hinzufügen",
    description: "Add one blank draft variable",
  },
  "services.envAddGenerated": {
    message: "Generiertes Secret",
    description: "Add one previewable generated secret",
  },
  "services.envImport": {
    message: "Aus .env importieren",
    description: "Open the dotenv import dialog",
  },
  "services.envImportTitle": {
    message: "Umgebungsvariablen importieren",
    description: "Dotenv import dialog title",
  },
  "services.envImportDescription": {
    message:
      "Füge Dotenv-Zuweisungen ein oder wähle eine Textdatei. Importierte Werte bleiben in diesem Entwurf, bis du speicherst.",
    description: "Dotenv import dialog explanation",
  },
  "services.envImportTextLabel": {
    message: "Dotenv-Inhalt",
    description: "Accessible label for the dotenv import textarea",
  },
  "services.envImportPlaceholder": {
    message: "API_URL=https://example.com\nSECRET=replace-me",
    description: "Dotenv import textarea placeholder",
  },
  "services.envImportChooseFile": {
    message: "Datei auswählen",
    description: "Dotenv file picker button",
  },
  "services.envImportAdd": {
    message: "Variablen hinzufügen",
    description: "Stage parsed dotenv variables",
  },
  "services.envImportLineError": {
    message: "Zeile {line} ist keine KEY=value-Zuweisung.",
    description: "Line-numbered dotenv parse error",
  },
  "services.envImportKeyError": {
    message:
      "Zeile {line}: Der Name darf höchstens 253 Buchstaben, Ziffern, Punkte, Bindestriche und Unterstriche enthalten.",
    description:
      "Dotenv import error: the variable name on a line is invalid (w4/159)",
  },
  "services.envImportQuoteError": {
    message:
      "Das in Zeile {line} geöffnete Anführungszeichen wird nie geschlossen.",
    description: "Dotenv import error: a quoted value never closes (w4/159)",
  },
  "services.envImportTrailingError": {
    message:
      "Zeile {line} enthält Text nach dem schließenden Anführungszeichen.",
    description:
      "Dotenv import error: text after a quoted value's closing quote (w4/159)",
  },
  "services.envImportReading": {
    message: "Datei wird gelesen…",
    description:
      "Status while a chosen dotenv file is being read; Add waits for it",
  },
  "services.envImportFileError": {
    message: "Wähle eine lesbare Textdatei mit höchstens 1 MiB.",
    description: "Dotenv file read or size error",
  },
  "services.secretFileUpload": {
    message: "Dateien hochladen",
    description: "Stage multiple local text files",
  },
  "services.secretFileSkippedTitle_one": {
    message: "{count} Datei wurde übersprungen:",
    description:
      "Secret-file upload: heading over the one skipped file (w4/628)",
  },
  "services.secretFileSkippedTitle_other": {
    message: "{count} Dateien wurden übersprungen:",
    description: "Secret-file upload: heading over the skipped files (w4/628)",
  },
  "services.secretFileSkippedName": {
    message:
      "der Name darf nur Buchstaben, Ziffern, '.', '-' und '_' enthalten (bis zu 253 Zeichen).",
    description: "Secret-file upload: why a file was skipped — its name",
  },
  "services.secretFileSkippedSize": {
    message: "sie ist größer als 1 MiB.",
    description: "Secret-file upload: why a file was skipped — its size",
  },
  "services.secretFileSkippedEncoding": {
    message:
      "sie ist kein UTF-8-Text. Lade eine Binärdatei (Keystore, .p12, DER-Zertifikat) Base64-kodiert hoch und dekodiere sie beim Start, z. B. base64 -d /etc/secrets/keystore.b64 > /tmp/keystore.p12.",
    description: "Secret-file upload: why a file was skipped — not UTF-8 text",
  },
  "services.secretFileSkippedDuplicate": {
    message: "eine Datei mit diesem Namen ist bereits in der Liste.",
    description: "Secret-file upload: why a file was skipped — duplicate name",
  },
  "services.secretFileDuplicateName": {
    message: "Jeder Name einer Secret-Datei muss eindeutig sein.",
    description: "Draft validation for a duplicate file name",
  },
  "services.secretFileContentRequired": {
    message: "Füge Inhalt für diese neue Datei hinzu.",
    description: "Draft validation for missing new-file content",
  },
  "services.secretFileLimit": {
    message:
      "Secret-Dateien sind auf 500 pro Service oder Gruppe begrenzt, ihre Gesamtgröße auf 1 MiB.",
    description:
      "Draft validation when the secret files would exceed bex-api's per-service quota: 500 files, 1 MiB of contents (names not counted), Render's 1 MB (w4/m363)",
  },
  "services.secretFileViewContent": {
    message: "Inhalt anzeigen",
    description: "Reveal or inspect secret-file contents in a dialog",
  },
  "services.secretFileViewContentOne": {
    message: "Inhalt von {name} anzeigen",
    description:
      "Accessible name for View contents once the file has a name (m101/t013)",
  },
  "services.secretFileEditContent": {
    message: "Inhalt bearbeiten",
    description: "Edit staged secret-file contents in a dialog",
  },
  "services.secretFileEditContentOne": {
    message: "Inhalt von {name} bearbeiten",
    description:
      "Accessible name for Edit contents once the file has a name (m101/t013)",
  },
  "services.secretFileContentDialogTitle": {
    message: "Inhalt von {name}",
    description: "Secret-file content dialog title",
  },
  "services.secretFileContentDialogDescription": {
    message:
      "Diese Datei wird nach dem nächsten Deployment schreibgeschützt unter /etc/secrets eingebunden.",
    description: "Secret-file content dialog explanation",
  },
  "services.secretFileContentDialogDescriptionDocker": {
    message:
      "Diese Datei wird nach dem nächsten Deployment schreibgeschützt unter /etc/secrets eingebunden. Ein Docker-Build findet sie außerdem im Root seines Build-Kontexts: Trage sie in .dockerignore ein, damit sie nicht in dein Image gelangt.",
    description:
      "Secret-file content dialog explanation for a Docker-runtime service: the build context also receives the file, as on Render (w8/253)",
  },
  "services.secretFileLoadingContent": {
    message: "Inhalt wird geladen…",
    description: "Secret-file content reveal progress",
  },
  "services.secretFileContentDone": {
    message: "Fertig",
    description: "Close and stage secret-file content edits",
  },
  "services.secretFileUntitled": {
    message: "unbenannte Datei",
    description: "Fallback title for a new unnamed secret file",
  },
  "services.secretFileDelete": {
    message: "Secret-Datei löschen",
    description: "Accessible label for a staged secret-file deletion",
  },
  "services.secretFileDeleteOne": {
    message: "Secret-Datei {name} löschen",
    description:
      "Accessible name for Delete secret file once the file has a name (m101/t013)",
  },
  "services.envGroupsLinkedTitle": {
    message: "Verknüpfte Umgebungsgruppen",
    description: "Service-side environment group card title",
  },
  "services.envGroupsLinkedCount": {
    message: "Verknüpft ({count})",
    description: "Count of groups linked to this service",
  },
  "services.envGroupsAvailableCount": {
    message: "Zum Verknüpfen verfügbar ({count})",
    description: "Count of groups available to link",
  },
  "services.envGroupsNoneLinked": {
    message: "Mit diesem Service sind keine Umgebungsgruppen verknüpft.",
    description: "Service group empty linked state",
  },
  "services.envGroupsAvailableTitle": {
    message: "Zum Verknüpfen verfügbar",
    description:
      "Available-groups heading while the service's environment is still resolving (no count yet)",
  },
  "services.envGroupsScopeLoading": {
    message:
      "Es wird geprüft, welche Gruppen die Umgebung dieses Services teilen…",
    description:
      "Available groups wait for the service's environment scope (w4/217)",
  },
  "services.envGroupsOrderReading": {
    message: "Es wird bestätigt, welche verknüpfte Gruppe Vorrang hat…",
    description:
      "Link order is being re-read after a link change; no winner is claimed until it lands (w4/322)",
  },
  "services.envGroupsOrderFailed": {
    message:
      "Verknüpft, aber es konnte nicht bestätigt werden, welche Gruppe Vorrang hat. Lade die Reihenfolge neu, um sie zu sehen.",
    description:
      "The post-write Server read failed; the write stands, precedence stays unclaimed until a retry succeeds (w4/322)",
  },
  "services.envGroupsScopeError": {
    message:
      "Es ließ sich nicht prüfen, welche Gruppen dieser Service verknüpfen kann.",
    description:
      "Scope index failed; nothing is offered until a retry succeeds (w4/217)",
  },
  "services.envGroupsScopeEnvironmentMissing": {
    message:
      "Die Umgebung dieses Services ist nicht in der geladenen Umgebungsliste enthalten, daher werden keine Gruppen zum Verknüpfen angeboten. Versuche es erneut, um sie zu aktualisieren.",
    description:
      "The service reports an Environment the authorized scope index lacks even after a refresh; never falls back to Workspace (w4/220)",
  },
  "services.envGroupsNoneInScope": {
    message:
      "Keine nicht verknüpften Gruppen teilen die Umgebung dieses Services. Erstelle hier eine oder verschiebe eine Gruppe in diese Umgebung.",
    description:
      "Unlinked groups exist, but none in this service's environment scope (w4/217)",
  },
  "services.envGroupsNoneAvailable": {
    message: "Alle Umgebungsgruppen des Workspace sind bereits verknüpft.",
    description: "Service group empty available state",
  },
  "services.envGroupsNoneAvailableCreate": {
    message:
      "Noch keine Umgebungsgruppen im Workspace. Erstelle eine, wobei Variablen, Dateien und dieser Service bereits ausgewählt sind.",
    description: "Service group create-first empty state",
  },
  "services.cronEveryMinute": {
    message: "Jede Minute",
    description: "Cron schedule preview phrase (w4/m188): fires every minute",
  },
  "services.cronEveryNMinutes": {
    message: "Alle {n} Minuten",
    description:
      "Cron schedule preview phrase (w4/m188): uniform minute step; n is the step",
  },
  "services.cronEveryHour": {
    message: "Jede Stunde",
    description:
      "Cron schedule preview phrase (w4/m188): fires at minute 0 of every hour",
  },
  "services.cronEveryNHours": {
    message: "Alle {n} Stunden",
    description:
      "Cron schedule preview phrase (w4/m188): uniform hour step; n is the step",
  },
  "services.cronHourlyAtMinute": {
    message: "Jede Stunde bei Minute {minute}",
    description:
      "Cron schedule preview phrase (w4/m188): fires once an hour at the given minute",
  },
  "services.cronDailyAt": {
    message: "Jeden Tag um {time}",
    description: "Cron schedule preview phrase (w4/m188): daily at HH:MM (UTC)",
  },
  "services.cronWeeklyAt": {
    message: "Jeden {day} um {time}",
    description:
      "Cron schedule preview phrase (w4/m188): weekly; day is a services.cronDay* weekday name, time is HH:MM (UTC)",
  },
  "services.cronMonthlyAt": {
    message: "Am {day}. jedes Monats um {time}",
    description:
      "Cron schedule preview phrase (w4/m188): monthly on a day of the month at HH:MM (UTC)",
  },
  "services.cronDaySunday": {
    message: "Sonntag",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayMonday": {
    message: "Montag",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayTuesday": {
    message: "Dienstag",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayWednesday": {
    message: "Mittwoch",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayThursday": {
    message: "Donnerstag",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayFriday": {
    message: "Freitag",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDaySaturday": {
    message: "Samstag",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.environmentEditingTitle": {
    message: "Umgebung wird bearbeitet",
    description:
      "Edit bar title while the environment draft is open and holds no change yet (the bar says Unsaved changes once it does)",
  },
  "services.environmentNoChanges": {
    message: "Noch keine Änderungen.",
    description:
      "Edit bar line while the open environment draft holds no change: what to do next",
  },
  "services.envAddOptions": {
    message: "Weitere Wege, eine Variable hinzuzufügen",
    description:
      "Accessible name of the chevron beside Add variable that opens the other ways to add (generated secret, .env import)",
  },
  "services.secretFilesReloadWithVariables": {
    message:
      "Sie werden mit den Variablen neu geladen, wenn du es oben erneut versuchst.",
    description:
      "Secret Files card line when both lists failed to load: the variables card's Try again reloads the files too, so this card has no button of its own",
  },
  "services.createSubmitWeb": {
    message: "Web Service deployen",
    description:
      "Create-wizard submit button for a web service (Render's 'Deploy web service')",
  },
  "services.createSubmitPrivate": {
    message: "Private Service deployen",
    description: "Create-wizard submit button for a private service",
  },
  "services.createSubmitWorker": {
    message: "Background Worker deployen",
    description: "Create-wizard submit button for a background worker",
  },
  "services.createSubmitCron": {
    message: "Cron Job deployen",
    description:
      "Create-wizard submit button for a cron job (Render's 'Deploy cron job')",
  },
  "services.createSubmitStatic": {
    message: "Static Site deployen",
    description: "Create-wizard submit button for a static site",
  },
  "services.createSourceEdit": {
    message: "Bearbeiten",
    description:
      "Button on the chosen repository's summary row that reopens the source picker",
  },
  "services.createSourceEditLabel": {
    message: "Quelle ändern",
    description:
      "Accessible name of the Edit button on the chosen repository's summary row ({repo} = owner/name)",
  },
  "services.createBlockedSource": {
    message: "Wählen Sie eine Quelle, um fortzufahren.",
    description:
      "Create-wizard action bar: why Deploy is held while no source is chosen",
  },
  "services.createFieldNameDescription": {
    message: "Ein eindeutiger Name für diesen Service.",
    description: "Create-wizard Name row description",
  },
  "services.createFieldRuntimeDescription": {
    message: "Wählen Sie die Laufzeitumgebung für diesen Service.",
    description: "Create-wizard Language/runtime row description",
  },
  "services.createFieldBranchDescription": {
    message: "Der Git-Branch, der gebaut und deployt wird.",
    description: "Create-wizard Branch row description",
  },
  "services.createFieldBuildCommandDescription": {
    message: "Dieser Befehl baut Ihre App vor jedem Deploy.",
    description:
      "Create-wizard Build Command row description (native runtimes)",
  },
  "services.createFieldStartCommandDescription": {
    message: "Dieser Befehl startet Ihre App bei jedem Deploy.",
    description: "Create-wizard Start Command row description",
  },
  "services.createFieldDockerCommandDescription": {
    message:
      "Optional: CMD und ENTRYPOINT des Images durch einen anderen Startbefehl ersetzen.",
    description: "Create-wizard Docker Command row description (Advanced)",
  },
  "services.createFieldPlanDescription": {
    message:
      "Die Instanz, auf der Ihr Service läuft. Sie lässt sich jederzeit ändern.",
    description: "Create-wizard compute plan row description",
  },
  "services.createEnvVarsDescription": {
    message:
      "Legen Sie umgebungsspezifische Konfiguration und Secrets (etwa API-Schlüssel) fest und lesen Sie die Werte in Ihrem Code.",
    description: "Create-wizard Environment Variables card description",
  },
  "services.createBlockedPublishPath": {
    message: "Geben Sie ein Veröffentlichungsverzeichnis ein, um fortzufahren.",
    description:
      "Create-wizard action bar: Deploy is held until a static site has a publish directory",
  },
  "services.createBlockedSchedule": {
    message: "Geben Sie einen gültigen Zeitplan ein, um fortzufahren.",
    description:
      "Create-wizard action bar: Deploy is held until a cron job has a valid schedule",
  },
  "services.createBlockedCommands": {
    message: "Geben Sie Build- und Startbefehl ein, um fortzufahren.",
    description:
      "Create-wizard action bar: Deploy is held until the native runtime's build and start commands are filled",
  },
  "services.createBlockedPort": {
    message: "Geben Sie einen gültigen Port ein, um fortzufahren.",
    description:
      "Create-wizard action bar: Deploy is held while the port is out of range",
  },
  "services.createBlockedDisk": {
    message:
      "Geben Sie einen gültigen Mount-Pfad und eine gültige Größe für die Disk ein, um fortzufahren.",
    description:
      "Create-wizard action bar: Deploy is held while a drafted disk has no valid mount path or size (w4/m652)",
  },
  "services.createBuildSourceRegionNotice": {
    message:
      "Services, die mit dieser Git-Build-Quelle verknüpft sind, werden passend zur Build-Quelle in {region} erstellt.",
    description:
      "Create service wizard: notice explaining that a git Build Source determines the new service's region; {region} is the Render region name",
  },
  "services.createBuildSourceAutoDeployHint": {
    message:
      "Deployt diesen Service automatisch, sobald ein neuer Build der verknüpften Build-Quelle erfolgreich abgeschlossen wird.",
    description:
      "Create service wizard: Auto-Deploy toggle description when creating from a Build Source",
  },
  "services.buildSourceKindGit": {
    message: "Git-Repository",
    description: "Badge or label for a git-backed Build Source",
  },
  "services.buildSourceKindImage": {
    message: "Container-Image",
    description: "Badge or label for an image-backed Build Source",
  },
  "services.sourceBuildSourceIdLabel": {
    message: "Build-Quellen-ID",
    description: "Accessible field name for copying a Build Source ID",
  },
  "services.sourceBuildSourceIdCopied": {
    message: "Build-Quellen-ID kopiert",
    description: "Toast shown after copying a Build Source ID",
  },
  "services.sourceBuildSourceIdCopyFailed": {
    message: "Build-Quellen-ID konnte nicht kopiert werden",
    description: "Toast shown when copying a Build Source ID fails",
  },
  "services.deployMenuLatestBuild": {
    message: "Neuesten Build deployen",
    description:
      "Manual Deploy dropdown item for a Build-Source-linked service: deploy the latest succeeded build from the Build Source",
  },
  "services.deployMenuSpecificBuild": {
    message: "Bestimmten Build deployen…",
    description:
      "Manual Deploy dropdown item for a Build-Source-linked service: pick a specific succeeded build (bld-…) to deploy",
  },
  "services.deployBuildTitle": {
    message: "Bestimmten Build deployen",
    description: "Title of the dialog to pick a specific Build Source build",
  },
  "services.deployBuildBody": {
    message:
      "Wähle einen erfolgreichen Build aus der verknüpften Build-Quelle, um ihn für diesen Service zu deployen.",
    description:
      "Description in the dialog to pick a specific Build Source build",
  },
  "services.deployBuildConfirm": {
    message: "Build deployen",
    description: "Confirm button label in the deploy-a-specific-build dialog",
  },
  "services.deployBuildListLabel": {
    message: "Builds der verknüpften Build-Quelle",
    description:
      "Accessible label for the build list inside the deploy-a-specific-build dialog",
  },
  "services.deployBuildEmpty": {
    message:
      "Diese Build-Quelle hat noch keine Builds. Starte zuerst einen Build für die Build-Quelle.",
    description:
      "Empty state in the deploy-a-specific-build dialog when no builds exist",
  },
  "services.deployBuildLoadError": {
    message: "Builds für diese Build-Quelle konnten nicht geladen werden.",
    description:
      "Error message in the deploy-a-specific-build dialog when fetching builds fails",
  },
  "services.deployBuildNoSucceeded": {
    message: "Es stehen noch keine erfolgreichen Builds zum Deployen bereit.",
    description:
      "Notice in the deploy-a-specific-build dialog when builds exist but none succeeded",
  },
  "services.buildStatusCreated": {
    message: "In Warteschlange",
    description: "Build Source build status badge: created/queued",
  },
  "services.buildStatusInProgress": {
    message: "Wird gebaut",
    description: "Build Source build status badge: in_progress",
  },
  "services.buildStatusSucceeded": {
    message: "Erfolgreich",
    description: "Build Source build status badge: succeeded",
  },
  "services.buildStatusFailed": {
    message: "Fehlgeschlagen",
    description: "Build Source build status badge: failed",
  },
  "services.buildStatusCanceled": {
    message: "Abgebrochen",
    description: "Build Source build status badge: canceled",
  },
};

export default deServices;
