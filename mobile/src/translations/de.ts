export const de = {
  freshness: {
    notLoaded: "Warte auf das erste Update",
    current: "Aktuell",
    stale: "Veraltet",
  },
  access: {
    offline:
      "Stelle die Verbindung wieder her, um den Zugriff zu prüfen, bevor du fortfährst.",
    checking: "Zugriff wird geprüft…",
    unavailable:
      "Dein Zugriff konnte nicht geprüft werden. Versuche es erneut.",
    changed: "Dein Zugriff in diesem Workspace hat sich geändert.",
    cannotOpen:
      "Dieses Element ist mit deinem aktuellen Zugriff nicht verfügbar.",
    restrictedTitle: "Eingeschränkt",
    backToStatus: "Zurück zum Status",
  },
  navigation: {
    status: "Status",
    activity: "Aktivität",
    sessions: "Sitzungen",
    notifications: "Warnungen",
  },
  notifications: {
    inboxChecking: "Zugriff wird geprüft und Warnungen werden aktualisiert…",
    inboxError:
      "Warnungen konnten nicht aktualisiert werden. Prüfe deine Verbindung und versuche es erneut.",
    benefit:
      "Erhalte wichtige Service- und Deployment-Updates auf deinem Handy.",
    emptyTitle: "Noch keine Warnungen",
    title: "Warnungen",
    body: "Aktuelle Betriebswarnungen, die auf diesem Gerät gespeichert sind.",
    settings: "Push-Benachrichtigungen",
    permissionExplanation:
      "bex fragt die Benachrichtigungsberechtigung nur an, wenn du auf Aktivieren tippst. Du kannst die Berechtigung später in den Systemeinstellungen ändern.",
    enable: "Aktivieren",
    disable: "Deaktivieren",
    recent: "Aktuell (%{count})",
    markAllRead: "Alle als gelesen markieren",
    empty:
      "Service- und Deployment-Updates erscheinen hier, sobald sie auf diesem Gerät eingehen.",
    open: "Benachrichtigung öffnen: %{title}",
    states: {
      checking: "Berechtigung wird geprüft…",
      unconfigured: "Push ist für diesen Build nicht konfiguriert.",
      unavailable: "Push-Zustellung ist auf diesem bex-Server deaktiviert.",
      disabled: "Aus",
      denied: "Berechtigung verweigert",
      revoked: "Berechtigung wurde in den Systemeinstellungen widerrufen",
      enabling: "Wird aktiviert…",
      enabled: "An",
      offline:
        "Offline — die Registrierung wird nach dem Wiederverbinden repariert",
      error: "Die Registrierung konnte nicht abgeschlossen werden",
    },
  },
  status: {
    overview: "Ressourcenzustand",
    healthy: "Fehlerfrei",
    review: "Prüfen",
    unknown: "Unbekannt",
    title: "Status",
    body: "Prüfe deine Services und erkenne, was Aufmerksamkeit braucht.",
  },
  usageGlance: {
    title: "Nutzung seit Monatsbeginn",
    states: {
      complete: "Vollständige Daten",
      partial: "Teilweise Daten",
      unknown: "Abdeckung nicht bestätigt",
      unavailable: "Nicht verfügbar",
      "healthy-empty": "Vollständig · keine Nutzung",
    },
    meters: {
      instance_seconds: "Laufzeit",
      egress_bytes: "Ausgehende Bandbreite",
      build_seconds: "Build-Zeit",
      storage_gb_seconds: "Datastore-Speicher",
      sandbox_compute_seconds: "Agent-Compute",
    },
    empty: "Die vollständigen Daten zeigen keine Nutzung über null.",
    noEvidence: "Nutzungssummen sind noch nicht verfügbar.",
    through: "Gemessen bis %{time}",
    degraded: "Einige Nutzungsquellen sind verzögert: %{sources}",
    refreshUnavailable:
      "Aktualisieren ist nicht verfügbar. Die zuletzt empfangenen Summen werden angezeigt.",
    loading: "Nutzung seit Monatsbeginn wird geladen…",
  },
  activity: {
    explore: "Service-Aktivität erkunden",
    title: "Aktivität",
    body: "Wähle einen Service, um Deployments, Ereignisse, Metriken und schreibgeschützte Live-Logs einzusehen.",
    timelineTitle: "Deployments und Ereignisse",
    deploy: "Deployment",
    empty: "Noch keine Deployments oder Ereignisse.",
    loadMore: "Frühere Aktivität laden",
  },
  resources: {
    search: "Ressourcen suchen",
    clearSearch: "Suche löschen",
    resetFilters: "Filter zurücksetzen",
    noMatches: "Keine passenden Ressourcen",
    service: "Service",
    unknownStatus: "unbekannt",
    updatedAt: "Aktualisiert %{time}",
    waitingForUpdate: "Warte auf das erste Update",
    filterLabel: "Ressourcen nach Typ filtern",
    filters: {
      all: "Alle",
      service: "Services",
      database: "Postgres",
      keyValue: "Key Value",
    },
    partialError:
      "Einige Ressourcendaten konnten nicht aktualisiert werden. Angezeigte Daten sind möglicherweise veraltet.",
    loadError:
      "Ressourcen konnten nicht geladen werden. Zieh nach unten, um es erneut zu versuchen.",
    loading: "Ressourcenzustand wird geladen…",
    emptyTitle: "Keine Ressourcen",
    emptyBody: "Dieser Workspace hat noch keine überwachten Ressourcen.",
    emptyFilter: "Keine Ressourcen passen zu diesem Filter.",
    ungrouped: "Workspace-Ressourcen",
    latestDeploy: "Neuestes Deployment %{id}",
    resourceAccessibility: "%{name}, %{type}, Status %{status}",
  },
  service: {
    title: "Service",
    overview: "Aktueller Zustand",
    region: "Region",
    replicas: "Instanzen",
    revision: "Revision",
    latestDeploy: "Neuestes Deployment",
    partialError:
      "Einige Service-Nachweise konnten nicht aktualisiert werden. Angezeigte Daten sind möglicherweise veraltet.",
    loadError: "Service-Nachweise konnten nicht geladen werden.",
  },
  cron: {
    title: "Cron-Ausführungsverlauf",
    description:
      "Letzte Ausführungen und Zeiten vom Server. Jetzt ausführen ersetzt jede aktuell aktive Ausführung.",
    runNow: "Jetzt ausführen",
    cancelCurrent: "Aktuelle Ausführung abbrechen",
    suspended:
      "Jetzt ausführen ist nicht verfügbar, solange dieser Service pausiert ist. Eine bereits aktive Ausführung kann weiterhin abgebrochen werden.",
    refreshRequired:
      "Die vorherige Aktion wurde möglicherweise übernommen. Aktualisiere den Verlauf erfolgreich, bevor du eine weitere Aktion bestätigst.",
    refresh: "Ausführungsverlauf aktualisieren",
    refreshing: "Wird aktualisiert…",
    loadError: "Der Cron-Ausführungsverlauf konnte nicht geladen werden.",
    empty: "Noch keine Cron-Ausführungen.",
    loadMore: "Frühere Ausführungen laden",
    openLogs: "Allgemeine Service-Logs öffnen",
    started: "Gestartet: %{time}",
    finished: "Beendet: %{time}",
    duration: "Dauer: %{duration}",
    notStarted: "nicht gemeldet",
    status: {
      pending: "Ausstehend",
      running: "Läuft",
      successful: "Erfolgreich",
      failed: "Fehlgeschlagen",
      canceled: "Abgebrochen",
      unknown: "Unbekannt",
    },
    feedback: {
      ambiguous:
        "Die Antwort endete ohne bestätigtes Ergebnis. Aktualisiere den Ausführungsverlauf, bevor du prüfst und eine weitere Aktion bestätigst.",
      convergenceTimeout:
        "Der Server hat die Aktion angenommen, aber der Ausführungsverlauf war beim Ende des Pollings nicht konsistent. Aktualisiere den Verlauf, bevor du eine weitere Aktion ausführst.",
    },
  },
  metrics: {
    title: "Metrik-Snapshot",
    window: "Letzte Stunde · Aktualisierung alle 30 Sekunden",
    cpu: "CPU",
    memory: "Arbeitsspeicher",
    network: "Ausgehendes Netzwerk",
    loading: "%{metric} wird geladen",
    unavailable: "Die Metrikquelle ist nicht konfiguriert.",
    error: "Diese Metrik konnte nicht geladen werden.",
    partial:
      "Teilweise Messwerte — die neuesten gültigen Werte werden angezeigt.",
    degraded: "Möglicherweise zu niedrig erfasst: %{sources}",
    noData: "Keine Messwerte in diesem Zeitfenster.",
    accessibilitySummary:
      "%{metric}, aktuell %{value}, %{points} Messwerte in der letzten Stunde",
  },
  events: {
    unknown: "Ereignis: %{type}",
    noType: "unbekannter Typ",
    branchTransition: "Branch %{from} → %{to}",
    countTransition: "Instanzen %{from} → %{to}",
    reason: "Grund: %{reason}",
    types: {
      deploy_started: "Deployment gestartet",
      deploy_ended: "Deployment abgeschlossen",
      build_started: "Build gestartet",
      build_ended: "Build abgeschlossen",
      pre_deploy_started: "Pre-Deploy gestartet",
      pre_deploy_ended: "Pre-Deploy abgeschlossen",
      server_available: "Service wurde verfügbar",
      server_unavailable: "Service wurde nicht verfügbar",
      plan_changed: "Tarif geändert",
      branch_changed: "Branch geändert",
      instance_count_changed: "Instanzanzahl geändert",
      autoscaling_config_changed: "Autoscaling geändert",
      job_run_started: "Job-Ausführung gestartet",
      job_run_ended: "Job-Ausführung abgeschlossen",
    },
  },
  datastores: {
    overview: "Aktueller Zustand",
    postgres: "Postgres",
    keyValue: "Key Value",
    plan: "Tarif",
    region: "Region",
    id: "Ressourcen-ID",
    loadError:
      "Dieser Datastore konnte nicht aktualisiert werden. Der angezeigte Zustand ist möglicherweise veraltet; zieh nach unten, um es erneut zu versuchen.",
  },
  postgresInsights: {
    title: "Postgres-Einblicke",
    connection: "Erreichbarkeit der Datenbank",
    capacity: "Kapazität",
    logicalSize: "Logische Datenbankgröße",
    diskUsed: "Disk belegt",
    diskCapacity: "Disk-Kapazität",
    connections: "Verbindungen",
    tables: "Signale zu Tabellenscans",
    tableSummary: "%{size} · seq %{sequential} · idx %{index} · dead %{dead}",
    noTableData: "Es wurden keine Tabellenbeobachtungen zurückgegeben.",
    sourceUnavailable: "Diese Einblicksquelle ist nicht konfiguriert.",
    transportError: "Diese Einblicksanfrage konnte den Server nicht erreichen.",
    state: {
      loading: "Wird geprüft",
      empty: "Keine Beobachtungen",
      current: "Aktuell",
      stale: "Veraltet",
      degraded: "Teilweise",
      "source-unavailable": "Quelle nicht verfügbar",
      "transport-error": "Verbindungsfehler",
    },
    connectionState: {
      loading: "Antwort der Datenbank wird geprüft…",
      empty: "Keine Datenbankbeobachtungen",
      current: "Datenbank hat geantwortet",
      stale: "Letzte Antwort ist veraltet",
      degraded: "Nur ein Teil der Prüfung wurde abgeschlossen",
      "source-unavailable": "Datenbank-Einblicksquelle nicht verfügbar",
      "transport-error":
        "Die Datenbankprüfung konnte den Server nicht erreichen",
    },
  },
  keyValueInsights: {
    title: "Key-Value-Einblicke",
    refreshing: "Key-Value-Einblicke werden aktualisiert",
    window: "Letzte Stunde · Aktualisierung alle 30 Sekunden",
    connectionHealth: "Verfügbarkeit (kein Protokoll-Ping)",
    storage: "Speicher belegt / Kapazität",
    memory: "Arbeitsspeicher belegt",
    connections: "Verbundene Clients",
    accessibilitySummary:
      "%{metric}: %{value}, basierend auf %{points} Beobachtungen",
    health: {
      available: "Verfügbar",
      creating: "Wird erstellt",
      unavailable: "Nicht verfügbar",
      suspended: "Pausiert",
      unknown: "Unbekannt",
    },
  },
  safeActions: {
    cardTitle: "Sichere Aktionen",
    title: "Aktion bestätigen",
    message:
      "Bestätige genau diese Aktion und dieses Ziel. Die App wiederholt eine Änderung nicht automatisch.",
    serverConfirmation:
      "Diese geschützte Ressource erfordert eine zweite Bestätigung. Server-Phrase: %{confirmation}",
    confirm: "Bestätigen",
    cancel: "Abbrechen",
    pending: "Wird gesendet…",
    noneAvailable: "Im aktuellen Zustand ist keine sichere Aktion verfügbar.",
    dismiss: "Schließen",
    refreshFirst: "Zuerst aktualisieren",
    actions: {
      triggerDeploy: "Neuestes deployen",
      cancelDeploy: "Aktives Deployment abbrechen",
      rollbackDeploy: "Deployment zurücksetzen",
      restartService: "Service neu starten",
      suspendService: "Service pausieren",
      resumeService: "Service fortsetzen",
      restartDatabase: "Postgres neu starten",
      suspendDatabase: "Postgres pausieren",
      resumeDatabase: "Postgres fortsetzen",
      suspendKeyValue: "Key Value pausieren",
      resumeKeyValue: "Key Value fortsetzen",
    },
    feedback: {
      success:
        "Der Server hat die Aktion angenommen. Der aktualisierte Zustand wird weiterhin vom Server abgerufen.",
      acceptedUnverified:
        "Der Server hat die Aktion angenommen, aber diese API kann nicht belegen, dass der Neustart abgeschlossen wurde. Prüfe weiter den aktuellen Zustand.",
      authorizationDenied:
        "Du bist nicht berechtigt, diese Aktion auszuführen. Es wurde kein Erfolg angenommen.",
      conflict:
        "Der Ressourcenzustand hat sich geändert, daher wurde die Aktion nicht gesendet. Aktualisiere und prüfe sie erneut.",
      timeoutUnknown:
        "Die Antwort ist in ein Timeout gelaufen und wurde möglicherweise übernommen. Aktualisiere den Serverzustand, bevor du eine weitere Bestätigung erstellst.",
      auditPending:
        "Die Aktion wurde übernommen, aber ihr Audit-Ergebnis steht noch aus.",
      auditUnavailable:
        "Das Ergebnis der Aktion ist unvollständig, weil der Audit-Zustand nicht verfügbar ist.",
      failed:
        "Die Aktion ist fehlgeschlagen. Der aktuelle Serverzustand wurde lokal nicht verändert.",
      canceled: "Die ausstehende Aktion wurde lokal abgebrochen.",
    },
  },
  resourceActions: {
    blocked: {
      protectedConfirmation:
        "Diese geschützte Ressource braucht eine zusätzliche Bestätigung, bevor diese Aktion ausgeführt werden kann.",
      suspended:
        "Nicht verfügbar, solange die Ressource pausiert ist. Setze sie zuerst fort.",
      noActiveDeploy: "Derzeit gibt es kein aktives Deployment zum Abbrechen.",
      noActiveRun: "Derzeit gibt es keine aktive Ausführung zum Abbrechen.",
      noEligibleRollbackTarget:
        "Kein früheres Deployment kommt als Rollback-Ziel infrage.",
      billingBlocked:
        "Die Abrechnung braucht Aufmerksamkeit, bevor diese Aktion ausgeführt werden kann. Löse das im Dashboard.",
      unavailable:
        "Der Server konnte diese Aktion gerade nicht bestätigen. Versuche es erneut.",
      generic:
        "Der Server erlaubt diese Aktion gerade nicht. Versuche es erneut.",
    },
  },
  sessions: {
    title: "Agent-Sitzungen",
    body: "Weise Coding-Agent-Aufgaben zu und beaufsichtige sie von deinem Handy aus.",
    gated: "Erfordert ADR047 Phase 1",
  },
  agentSessions: {
    heading: "Deine Agents auf einen Blick",
    description:
      "Verfolge den Fortschritt, prüfe Ergebnisse oder starte eine neue Aufgabe.",
    newShort: "Neu",
    loading: "Sitzungen werden geladen…",
    refreshError:
      "Aktualisieren nicht möglich. Zuvor geladene Sitzungen werden angezeigt.",
    unavailable: "Sitzungen konnten nicht geladen werden. Versuche es erneut.",
    emptyTitle: "Noch keine Agent-Sitzungen",
    emptyBody:
      "Weise einem Repository von deinem Handy aus eine Coding-Agent-Aufgabe zu.",
    phase: {
      creating: "Wird gestartet",
      running: "Läuft",
      resuming: "Wird fortgesetzt",
      redispatching: "Wird neu zugewiesen",
      completed: "Abgeschlossen",
      failed: "Fehlgeschlagen",
      canceling: "Wird abgebrochen",
      canceled: "Abgebrochen",
      unknown: "Unbekannt",
    },
    composer: {
      new: "Neue Sitzung",
      title: "Neue Agent-Sitzung",
      close: "Schließen",
      repo: "Repository",
      agent: "Agent",
      agentAuto: "Automatisch",
      searchRepo: "Repositories suchen",
      searchEmpty: "Keine passenden Repositories",
      prompt: "Aufgabe",
      promptPlaceholder: "Frag, was du willst",
      submit: "Aufgabe zuweisen",
      submitted: "Sitzung zugewiesen.",
      openSession: "Sitzung öffnen",
      noRepos: "Keine verbundenen Repositories.",
      setupTitle: "Einrichtung am Desktop abschließen",
      needGithub: "Verbinde GitHub am Desktop, um Pull Requests zu öffnen.",
      needModelKey:
        "Ein Workspace-Admin muss im Web-Dashboard unter Workspace-Einstellungen → Agents einen Modellanbieter-Schlüssel hinzufügen, damit Sitzungen laufen können.",
      needDesktop:
        "Sitzungen konnten nicht geladen werden. Versuche es erneut.",
      connectGithub: "GitHub verbinden",
    },
    detail: {
      overview: "Übersicht",
      phase: "Phase",
      updated: "Aktualisiert %{time}",
      unavailable: "Diese Sitzung ist nicht verfügbar.",
      notFound: "Sitzung nicht gefunden.",
      failure: "Fehler",
      deliveryNote: "Änderungen nicht vollständig übermittelt",
      pullRequest: "Pull Request",
      draftPr: "Draft-Pull-Request",
      openPr: "In GitHub öffnen",
      cancel: "Sitzung abbrechen",
      refreshError:
        "Aktualisieren nicht möglich. Zuvor geladene Sitzungsdetails werden angezeigt.",
      stale:
        "Die Sitzungsdetails sind möglicherweise veraltet. Zieh nach unten, um zu aktualisieren.",
    },
  },
  auth: {
    loadingTitle: "bex wird geöffnet",
    loadingBody: "Deine sichere Sitzung wird wiederhergestellt.",
    signInTitle: "Bei bex anmelden",
    signInBody:
      "Fahre in deinem System-Browser fort. Die App sieht nie dein Passwort oder deine MFA-Antwort.",
    signInAction: "Sicher fortfahren",
    signInError:
      "Die Anmeldung wurde nicht abgeschlossen. Prüfe deine Verbindung und versuche es erneut.",
    callbackTitle: "Anmeldung wird abgeschlossen",
    callbackBody: "Deine Sitzung wird gesichert und bex wird geöffnet.",
    expiredTitle: "Sitzung braucht eine Verbindung",
    expiredBody:
      "Dein Zugriffstoken ist abgelaufen, während bex offline war. Stelle die Verbindung wieder her, um es sicher zu erneuern.",
    retry: "Erneut versuchen",
    signOut: "Abmelden",
  },
  invite: {
    title: "Workspace-Einladung",
    loading: "Diese Einladung wird auf deinem Gerät gesichert.",
    signInBody:
      "Melde dich in deinem System-Browser an und kehre dann hierher zurück, um die Einladung zu prüfen. Der Link kann dem Konto beitreten, das du wählst, unabhängig von dessen E-Mail-Adresse.",
    signIn: "Zum Prüfen anmelden",
    signInError:
      "Die Anmeldung wurde nicht abgeschlossen. Die Einladung bleibt auf diesem Gerät gesichert, sodass du es erneut versuchen kannst.",
    readyBody:
      "Mit dem Annehmen erhält dein aktuelles bex-Konto Zugriff auf den Workspace, der durch diesen Einmal-Link bezeichnet wird. Die eingeladene E-Mail-Adresse wird bei dieser direkten Annahme nicht geprüft.",
    accept: "Einladung annehmen",
    accepting:
      "Diese Einladung wird einmalig angenommen. Lass die App geöffnet.",
    retryTitle: "Einladung nicht bestätigt",
    retry: "Erneut annehmen",
    retryable: {
      transport:
        "Die Antwort endete, bevor bex das Ergebnis bestätigen konnte. Die Einladung bleibt gesichert; versuche es ausdrücklich erneut, wenn du verbunden bist.",
      unavailable:
        "bex ist vorübergehend nicht verfügbar. Die Einladung bleibt gesichert; versuche es ausdrücklich erneut, wenn du verbunden bist.",
    },
    acceptedTitle: "Einladung angenommen",
    acceptedBody: "Du bist %{workspace} beigetreten.",
    refreshFailed:
      "Die Einladung wurde angenommen, aber die Workspace-Liste konnte nicht aktualisiert werden. Beim erneuten Öffnen von Status wird es noch einmal versucht.",
    openWorkspace: "Status öffnen",
    failedTitle: "Einladung nicht verfügbar",
    empty: "Diese Route enthält keine ausstehende Einladung.",
    failures: {
      invalid: "Dieser Einladungslink ist ungültig oder existiert nicht mehr.",
      expired:
        "Diese Einladung ist abgelaufen. Bitte einen Workspace-Admin, sie erneut zu senden.",
      "already-accepted": "Diese Einmal-Einladung wurde bereits angenommen.",
      "plan-limit":
        "Der aktuelle Tarif oder das Platzlimit des Workspace lässt diese Einladung nicht zu. Bitte einen Admin, das zu lösen und die Einladung erneut zu senden.",
      authorization:
        "Dein aktuelles Konto ist nicht berechtigt, diese Einladung anzunehmen.",
      "subject-changed":
        "Das angemeldete Konto hat sich geändert, daher wurde die gespeicherte Einladung gelöscht. Öffne den ursprünglichen Link erneut mit dem vorgesehenen Konto.",
      failed:
        "bex hat diese Einladung abgelehnt. Es wurde keine Mitgliedschaft angenommen.",
      storage:
        "Dieses Gerät konnte die Einladung nicht sichern oder löschen. Die App versucht keine Annahme.",
    },
  },
  workspace: {
    current: "Workspace",
    choose: "Workspace auswählen",
    switchLabel: "Workspace wechseln. Aktueller Workspace: %{name}",
    confirm: "Wechseln",
    cancel: "Abbrechen",
    offline:
      "Offline. Der aktuelle Workspace wird ohne zwischengespeicherte Tenant-Daten angezeigt.",
    loadingTitle: "Dein Workspace wird geöffnet",
    loadingBody: "Deine bex-Workspace-Mitgliedschaft wird geprüft.",
    switchingTitle: "Workspace wird gewechselt",
    switchingBody:
      "Der vorherige Workspace wird geleert, bevor der nächste geöffnet wird.",
    emptyTitle: "Noch kein Workspace",
    emptyBody:
      "Tritt auf bex.co einem Workspace bei oder erstelle einen und versuche es dann erneut.",
    errorTitle: "Workspace nicht verfügbar",
    errorBody: "bex konnte deinen Workspace nicht laden. Versuche es erneut.",
    offlineTitle: "Workspace braucht eine Verbindung",
    offlineBody:
      "Stelle die Verbindung wieder her, um Workspace-Daten sicher zu laden.",
  },
  drawer: {
    openMenu: "Menü öffnen",
    openMenuHint: "Zeigt deine Workspaces und dein Konto",
    menuLabel: "Menü für Workspace und Konto",
    opened: "Menü geöffnet",
    closeBackdrop: "Menü schließen",
    workspaces: "Workspaces",
    workspaceRow: "%{name}, %{status}",
    noStatus: "keine Rolle oder kein Tarif",
    roles: {
      admin: "Admin",
      developer: "Entwickler",
      contributor: "Contributor",
      viewer: "Betrachter",
      billing: "Abrechnung",
    },
    plans: {
      hobby: "Hobby",
      pro: "Pro",
      scale: "Scale",
      enterprise: "Enterprise",
      free: "Free",
    },
    signedIn: "Angemeldet",
    identityLoading: "Dein Konto wird geladen…",
    identityOffline: "Offline — Kontodetails nicht verfügbar",
    identityUnavailable: "Kontodetails nicht verfügbar",
    identityRetry: "Erneut versuchen",
    identityAccessibility: "Angemeldet als %{primary} %{secondary}",
    logout: "Abmelden",
    logoutConfirmTitle: "Abmelden?",
    logoutConfirmMessage:
      "Du musst dich erneut anmelden, um deine Workspaces zu beaufsichtigen.",
  },
  settings: {
    title: "Einstellungen",
    open: "Einstellungen öffnen",
    close: "Einstellungen schließen",
    colorScheme: "Farbschema",
    themeSystem: "System",
    themeLight: "Hell",
    themeDark: "Dunkel",
    language: "Sprache",
    version: "Version %{version}",
  },
  deepLink: {
    invalidTitle: "Ungültiger bex-Link",
    invalidBody: "Dieser Link enthält keine gültige bex-Ressourcenkennung.",
    serviceTitle: "Service",
    serviceBody:
      "Details zur Service-Überwachung folgen im nächsten Meilenstein.",
    sessionTitle: "Agent-Sitzung",
    sessionBody:
      "Details zur Sitzungsüberwachung folgen mit den Agent-Operationen.",
  },
  logs: {
    title: "Live-Logs",
    description:
      "Hole aktuelle App-, Build-, Pre-Deploy- und Request-Logs nach und verfolge dann die Live-Ausgabe, ohne eine Shell zu öffnen.",
    open: "Logs ansehen",
    back: "Zurück zu den Service-Nachweisen",
    searchLabel: "Logs durchsuchen",
    searchPlaceholder: "Log-Meldungen durchsuchen",
    pause: "Pausieren",
    follow: "Folgen",
    historyOnly:
      "Nur Verlauf — diese Filter können nicht auf den Live-Tail angewendet werden.",
    storeUnavailable:
      "Diese Log-Filter erfordern den dauerhaften Log-Speicher.",
    unavailable:
      "Logs sind nicht verfügbar. Prüfe deine Verbindung und versuche es erneut.",
    empty: "Keine Log-Zeilen in diesem Zeitraum.",
    type: {
      all: "Alle",
      app: "App",
      build: "Build",
      predeploy: "Pre-Deploy",
      request: "Request",
    },
    phase: {
      idle: "Log-Tail gestoppt",
      catching_up: "Aktuelle Logs werden geladen…",
      connecting: "Verbindung zu Live-Logs wird hergestellt…",
      live: "Live",
      reconnecting: "Verbindung zu Live-Logs wird wiederhergestellt…",
      history_only: "Nur Verlauf",
      error: "Log-Tail nicht verfügbar",
    },
  },
  common: {
    back: "Zurück",
    cancel: "Abbrechen",
    notFoundTitle: "Dieser Bildschirm existiert nicht",
    backToStatus: "Zurück zum Status",
    seeAll: "Alle anzeigen",
    notEnoughChartData: "Nicht genügend Daten",
  },
};
