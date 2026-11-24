import type { TranslationEntry } from "@/i18n";

const ptLogs: Record<string, TranslationEntry> = {
  "logs.rangeLabel": {
    message: "Intervalo do histórico de logs",
    description: "Accessible label for the relative log-history range controls",
  },
  "logs.rangeLiveTail": {
    message: "Ao vivo",
    description:
      "Log explorers (w3/m537): the first entry of the time range picker, Render's Live tail; picking it streams new lines, and every other range is a snapshot",
  },
  "logs.retentionWindow_one": {
    message: "Os logs são mantidos por {count} dia no plano {plan}.",
    description:
      "Log range picker hint: how many days the workspace plan keeps logs (w9/m389, from bex-api logRetention)",
  },
  "logs.retentionWindow_many": {
    message: "Os logs são mantidos por {count} de dias no plano {plan}.",
    description:
      "Log range picker hint: how many days the workspace plan keeps logs (w9/m389, from bex-api logRetention)",
  },
  "logs.retentionWindow_other": {
    message: "Os logs são mantidos por {count} dias no plano {plan}.",
    description:
      "Log range picker hint: how many days the workspace plan keeps logs (w9/m389, from bex-api logRetention)",
  },
  "logs.retentionUpgrade_one": {
    message: "Faça upgrade para {plan} para manter {count} dia.",
    description:
      "Log range picker hint: the cheapest plan that keeps logs longer, and its days (w9/m389)",
  },
  "logs.retentionUpgrade_many": {
    message: "Faça upgrade para {plan} para manter {count} de dias.",
    description:
      "Log range picker hint: the cheapest plan that keeps logs longer, and its days (w9/m389)",
  },
  "logs.retentionUpgrade_other": {
    message: "Faça upgrade para {plan} para manter {count} dias.",
    description:
      "Log range picker hint: the cheapest plan that keeps logs longer, and its days (w9/m389)",
  },
  "logs.typeLabel": {
    message: "Tipo de log",
    description: "Accessible label for the log-type filter dropdown",
  },
  "logs.typeAll": {
    message: "Todos os logs",
    description: "Log-type filter option: no type filter",
  },
  "logs.typeApplication": {
    message: "Logs da aplicação",
    description: "Log-type filter option: application (app) logs",
  },
  "logs.typeRequest": {
    message: "Logs de requisições",
    description:
      "Log-type filter option: request logs (Traefik access lines, from the durable store)",
  },
  "logs.levelLabel": {
    message: "Nível",
    description: "Accessible label for the log-level filter dropdown",
  },
  "logs.levelAll": {
    message: "Todos os níveis",
    description: "Level filter option: no level filter",
  },
  "logs.methodLabel": {
    message: "Método",
    description: "Accessible label for the HTTP-method filter dropdown",
  },
  "logs.methodAll": {
    message: "Todos os métodos",
    description: "Method filter option: no method filter",
  },
  "logs.statusCodeLabel": {
    message: "Código de status",
    description: "Accessible label for the HTTP-status filter dropdown",
  },
  "logs.statusCodeAll": {
    message: "Todos os status",
    description: "Status-code filter option: no status filter",
  },
  "logs.instanceLabel": {
    message: "Instância",
    description: "Accessible label for the instance (replica) filter dropdown",
  },
  "logs.instanceAll": {
    message: "Todas as instâncias",
    description: "Instance filter option: no instance filter",
  },
  "logs.filterByInstance": {
    message: "Filtrar logs pela instância {instance}",
    description:
      "Accessible label for the clickable instance slug on an application log line",
  },
  "logs.pathLabel": {
    message: "Caminho da requisição",
    description: "Accessible label for the request-path filter input",
  },
  "logs.filtersButton": {
    message: "Filtros",
    description:
      "Trigger for the structured-filters popover (level/method/status/instance/path)",
  },
  "logs.chipRemove": {
    message: "Remover filtro {label}",
    description:
      "Accessible label for an active-filter chip's clear button; {label} is the filter's field name",
  },
  "logs.pathPlaceholder": {
    message: "Filtrar por caminho",
    description: "Placeholder for the request-path filter input",
  },
  "logs.hostLabel": {
    message: "Host da requisição",
    description:
      "Accessible label for the request-host filter input and its chip (Render's host log filter, w4/538)",
  },
  "logs.hostPlaceholder": {
    message: "Filtrar por host",
    description: "Placeholder for the request-host filter input",
  },
  "logs.liveUnsupported": {
    message:
      "O acompanhamento ao vivo não está disponível — método, status, caminho e host filtram logs de requisições, então os logs do app não podem ser acompanhados com eles.",
    description:
      "Note when the chosen log type cannot carry an active filter, so live tail is off (a request filter on app logs)",
  },
  "logs.storeRequiredTitle": {
    message: "Os logs de requisições precisam do armazenamento de logs",
    description:
      "Empty-state title when a request/structured-filter query hits a deployment with no durable store (503)",
  },
  "logs.storeRequiredBody": {
    message:
      "Os logs de requisições e os filtros de nível/status/método/caminho leem do armazenamento de logs durável, que não está configurado aqui. Eles ficam disponíveis onde o armazenamento estiver conectado.",
    description: "Empty-state body for the store-unavailable (503) state",
  },
  "logs.requestPlanTitle": {
    message: "Os logs de requisições HTTP exigem o plano Pro",
    description:
      "Notice title in place of Request logs on a Hobby workspace (PLAN_FEATURE_REQUIRED, w8/m138)",
  },
  "logs.requestPlanBody": {
    message:
      "Os logs de requisições estão disponíveis no plano Pro e superiores. Este workspace está no plano Hobby, então mostra apenas logs da aplicação e de build.",
    description:
      "Notice body in place of Request logs on a Hobby workspace (w8/m138)",
  },
  "logs.searchPlaceholder": {
    message: "Pesquisar logs",
    description: "Placeholder for the log search box (text filter)",
  },
  "logs.linesLabel": {
    message: "Linhas de log",
    description:
      "Accessible name of the scrollable log list, which takes keyboard focus so arrow keys and Page Up/Down scroll it",
  },
  "logs.linesKeysHint": {
    message:
      "Use as setas para cima e para baixo para mover entre linhas, Enter para mostrar só a instância dessa linha e Shift+F10 para abrir o menu dela.",
    description:
      "Logs tab: screen-reader description of the focused log list's keys - arrows move the row cursor, Enter shows only that line's instance, Shift+F10 opens the line's menu (w3/m528)",
  },
  "logs.linesKeysShort": {
    message:
      "↑↓ mover entre linhas · Enter: só esta instância · Shift+F10: menu",
    description:
      "Logs tab: the keys of the focused log list, shown in its corner while it has keyboard focus - arrows move between lines, Enter shows only that line's instance, Shift+F10 opens the line's menu",
  },
  "logs.returnToBottom": {
    message: "Voltar ao fim",
    description:
      "Log list (w3/m528): accessible name and tooltip of Render's icon-only Return to Bottom button, shown while the reader is scrolled away from the newest line (ascending order)",
  },
  "logs.returnToTop": {
    message: "Voltar ao topo",
    description:
      "Log list (w3/m528): accessible name and tooltip of Render's icon-only Return to Top button, shown while the reader is scrolled away from the newest line (descending order)",
  },
  "logs.newLogs": {
    message: "Logs novos",
    description:
      "Log list (w3/m528): Render's held-lines button while a live tail holds fewer than 10 new lines back from a reader scrolled away",
  },
  "logs.newLogsCount_one": {
    message: "{count}+ log novo",
    description:
      "Log list (w3/m528): Render's held-lines button for 10 or more held lines; count is the held count rounded up to a multiple of 5, at most 100, followed by a plus sign",
  },
  "logs.newLogsCount_many": {
    message: "{count}+ de logs novos",
    description:
      "Log list (w3/m528): Render's held-lines button for 10 or more held lines; count is the held count rounded up to a multiple of 5, at most 100, followed by a plus sign",
  },
  "logs.newLogsCount_other": {
    message: "{count}+ logs novos",
    description:
      "Log list (w3/m528): Render's held-lines button for 10 or more held lines; count is the held count rounded up to a multiple of 5, at most 100, followed by a plus sign",
  },
  "logs.newLogsInLiveTail": {
    message: "Há logs novos ao vivo",
    description:
      "Log list (w3/m528): Render's button on a preset range when a new line arrived; clicking it picks Live tail (w3/m537)",
  },
  "logs.switchToLiveTail": {
    message: "Mudar para Ao vivo",
    description:
      "Log list (w3/m528): tooltip of the New logs in Live Tail button",
  },
  "logs.dismiss": {
    message: "Dispensar",
    description:
      "Log list (w3/m528): accessible name of the x that hides the New logs in Live Tail button",
  },
  "logs.lineMenu": {
    message: "Opções da linha",
    description:
      "Log list (w3/m528): accessible name and tooltip of the row menu button (...) on a hovered log line",
  },
  "logs.copyLogLine": {
    message: "Copiar linha de log",
    description:
      "Log row menu (w3/m528): copies the line without its time, as Render's Copy log line",
  },
  "logs.lineCopied": {
    message: "Copiado!",
    description:
      "Log row menu (w3/m528): toast after Copy log line, Render's 'Copied!'",
  },
  "logs.lineCopyFailed": {
    message: "Não foi possível copiar a linha de log",
    description:
      "Log row menu (w3/m528): toast when the clipboard refused the copied line",
  },
  "logs.viewInContext": {
    message: "Ver no contexto",
    description:
      "Log row menu (w3/m528): opens a new tab on the same explorer, unfiltered, 10 minutes either side of the line",
  },
  "logs.sshIntoInstance": {
    message: "Acessar a instância por SSH",
    description:
      "Log row menu (w3/m528): opens the service's Shell on the line's instance in a new tab",
  },
  "logs.sshInstanceNotRunning": {
    message:
      "Você só pode acessar por SSH instâncias em execução. Esta não está disponível.",
    description:
      "Log row menu (w3/m528): Render's tooltip on a disabled SSH into instance item, for an instance that is no longer running",
  },
  "logs.linesKeysHintMenu": {
    message:
      "Use as setas para cima e para baixo para mover entre linhas e Shift+F10 para abrir o menu de uma linha.",
    description:
      "Log list without an instance filter (w3/m528): screen-reader description of its keys - arrows move the row cursor, Shift+F10 opens the line's menu",
  },
  "logs.linesKeysShortMenu": {
    message: "↑↓ mover entre linhas · Shift+F10: menu",
    description:
      "Log list without an instance filter (w3/m528): its keys, shown in its corner while it has keyboard focus",
  },
  "logs.themeSettings": {
    message: "Configurações de tema",
    description:
      "Log explorer Options menu (w3/m528): Render's last item, a link to the Appearance section of account Settings",
  },
  "logs.endOfLog": {
    message: "Fim do log",
    description:
      "Closing line after the last row of a log that will not grow (a finished deploy's), so a short log reads as complete",
  },
  "logs.loading": {
    message: "Carregando logs…",
    description: "Shown while the first historical page is loading",
  },
  "logs.loadingOlder": {
    message: "Carregando logs mais antigos…",
    description:
      "Shown at the top of the pane while a page-back request is in flight",
  },
  "logs.loadOlder": {
    message: "Carregar mais antigos",
    description:
      "Button in the truncation notice that fetches the next older page — the keyboard/underfilled-pane path to what scrolling to the top does",
  },
  "logs.truncatedNotice": {
    message:
      "Apenas parte deste intervalo de tempo foi carregada. Carregue os mais antigos para continuar.",
    description:
      "Banner when hasMore is true: more of the selected range remains unloaded, however many older pages are already shown, so it names no line count (w4/280)",
  },
  "logs.partialSearchNotice": {
    message:
      "Até agora só a parte mais recente deste intervalo foi pesquisada — carregue os mais antigos para continuar pesquisando.",
    description:
      "Banner when a time-boxed search returned less than a page but hasMore is true: the older part of the range is unsearched, not empty (w4/m140)",
  },
  "logs.timeoutTitle": {
    message: "Esta pesquisa demorou demais",
    description:
      "State title when the logs query ran out of the server's time budget (QUERY_TIMEOUT, w4/m140)",
  },
  "logs.timeoutBody": {
    message:
      "Tente um intervalo de tempo menor ou um termo de pesquisa mais específico.",
    description: "State body under logs.timeoutTitle",
  },
  "logs.streaming": {
    message: "Ao vivo — recebendo novas linhas",
    description: "Status under the log list when the SSE tail is connected",
  },
  "logs.disconnected": {
    message: "Acompanhamento ao vivo desconectado — reconectando…",
    description: "Banner when the SSE stream drops",
  },
  "logs.emptyTitle": {
    message: "Nenhum log neste intervalo de tempo",
    description:
      "Empty-state title when the selected (always-bounded) history range holds no lines. Never claims the service has never logged — the range can't establish that (w6/m111)",
  },
  "logs.emptyBody": {
    message: "Nenhuma linha de log neste intervalo de tempo.",
    description:
      "Empty-state body with no filters applied — honest about the bounded window, not the service's whole history (w6/m111)",
  },
  "logs.emptyFilteredBody": {
    message: "Nenhum log corresponde a estes filtros.",
    description:
      "Empty-state body when a type/text/structured filter yields nothing",
  },
  "logs.emptyFilteredTitle": {
    message: "Nenhum log correspondente",
    description:
      "Empty-state title when a type/text/structured filter yields nothing",
  },
  "logs.errorTitle": {
    message: "Não foi possível carregar os logs",
    description: "Error-state title when the logs query fails",
  },
  "logs.errorBody": {
    message:
      "A solicitação para este intervalo falhou. Tente novamente ou escolha um intervalo menor.",
    description:
      "Logs tab: body when the log history read failed, under logs.errorTitle; the server's own reason follows as a detail line",
  },
  "logs.errorBodyLive": {
    message:
      "As novas linhas continuarão aparecendo aqui à medida que chegarem. Tente novamente para carregar o histórico deste intervalo.",
    description:
      "Logs tab: body when the history read failed while the live tail still streams \u2014 new lines will still land in the panel",
  },
  "logs.filtersDone": {
    message: "Concluído",
    description: "Logs tab: button closing the Filters panel",
  },
  "logs.emptyBodyRange": {
    message:
      "Tente um intervalo maior ou mude para Ao vivo para acompanhar as novas linhas.",
    description:
      "Logs tab: body when the chosen range holds no lines, under logs.emptyTitle \u2014 what to try next",
  },
  "logs.clearFilters": {
    message: "Limpar filtros",
    description:
      "Logs tab: button in the no-match state that clears the search and every filter",
  },
  "logs.errorTitleWithReason": {
    message: "Não foi possível carregar os logs: {reason}",
    description:
      "Inline banner over loaded log lines when a later read was refused; reason is the server's refusal",
  },
  "logs.observabilityViewEyebrow": {
    message: "Visualização de observabilidade",
    description:
      "Eyebrow over a project observability view's name (Render's OBSERVABILITY VIEW, w3/m505)",
  },
  "logs.applicationLogsTitle": {
    message: "Logs de aplicação",
    description:
      "Document title part for a project's Application Logs view page (w3/m505)",
  },
  "logs.requestLogsTitle": {
    message: "Logs de requisições",
    description:
      "Document title part for a project's Request Logs view page (w3/m505)",
  },
  "logs.projectScopeEmptyTitle": {
    message: "Ainda não há nada para ler nesta visualização",
    description:
      "Project log view state: the view's scope holds no resource to read (w3/m505)",
  },
  "logs.projectScopeEmptyBody": {
    message:
      "Esta visualização lê os serviços, cron jobs, bancos de dados Postgres e instâncias Key Value do projeto, e ainda não inclui nenhum.",
    description:
      "Project Application Logs view with no service, cron job, Postgres or Key Value in it (w3/m505)",
  },
  "logs.projectRequestScopeEmptyBody": {
    message:
      "Os logs de requisições leem os serviços web do projeto, e ainda não há nenhum nesta visualização.",
    description:
      "Project Request Logs view with no web service in it (w3/m505)",
  },
  "logs.projectEmptyTitle": {
    message: "Nenhum log neste intervalo",
    description:
      "Project log view state: its resources logged nothing in the selected range (w3/m505)",
  },
  "logs.projectEmptyBody": {
    message:
      "Nenhum recurso desta visualização registrou nada no intervalo selecionado.",
    description:
      "Project log view state body: no line in the range from any of its resources (w3/m505)",
  },
  "logs.viewNotFoundTitle": {
    message: "Visualização não encontrada",
    description:
      "Project observability page whose savedSearchId names no view of this project (w3/m505)",
  },
  "logs.viewNotFoundBody": {
    message:
      "Este projeto não tem visualização neste link. Ela pode ter sido excluída.",
    description: "Body of the view-not-found state (w3/m505)",
  },
  "logs.viewOpen": {
    message: "Abrir {name}",
    description:
      "Link from the view-not-found state to the project's default view of the same type; name is the view's name",
  },
  "logs.viewLoadErrorTitle": {
    message: "Não foi possível carregar esta visualização",
    description:
      "Project observability page whose saved searches could not be read (w3/m505)",
  },
  "logs.viewLoadErrorBody": {
    message:
      "Não foi possível ler as visualizações do projeto. Recarregue a página para tentar novamente.",
    description: "Body of the view load failure (w3/m505)",
  },
  "logs.newView": {
    message: "Nova visualização",
    description:
      "Project sidebar button that opens the TYPE menu for a new observability view (w3/m507, Render's '+ New View')",
  },
  "logs.newViewTypeHeading": {
    message: "Tipo",
    description:
      "Heading of the New View menu listing view types (Render's TYPE)",
  },
  "logs.editView": {
    message: "Editar visualização",
    description:
      "Observability view header button that opens the Edit view modal (w3/m507)",
  },
  "logs.duplicateView": {
    message: "Duplicar",
    description:
      "Observability view header button that copies the view (w3/m507)",
  },
  "logs.deleteView": {
    message: "Excluir visualização",
    description:
      "Accessible name and confirm label of the observability view's trash button (w3/m507)",
  },
  "logs.deleteViewTitle": {
    message: "Excluir {name}?",
    description:
      "Title of the confirmation before deleting an observability view; name is the view's name",
  },
  "logs.deleteViewBody": {
    message:
      "A visualização é excluída para todos no projeto. Os recursos do projeto e seus logs não são afetados.",
    description:
      "Body of the delete-view confirmation: views are shared by the whole project (w3/m507)",
  },
  "logs.viewDeleted": {
    message: "{name} foi excluída",
    description:
      "Toast after an observability view was deleted; name is the view's name",
  },
  "logs.deleteViewFailed": {
    message: "Não foi possível excluir a visualização",
    description:
      "Toast when deleting an observability view failed without a server reason",
  },
  "logs.viewDuplicated": {
    message: "{name} foi criada",
    description: "Toast after a view was duplicated; name is the copy's name",
  },
  "logs.duplicateViewFailed": {
    message: "Não foi possível duplicar a visualização",
    description:
      "Toast when duplicating an observability view failed without a server reason",
  },
  "logs.editViewTitle": {
    message: "Editar visualização",
    description: "Title of Render's Edit view modal (w3/m507)",
  },
  "logs.newViewTitle": {
    message: "Nova visualização",
    description: "Title of the view modal when New View opened it (w3/m507)",
  },
  "logs.viewNameLabel": {
    message: "Nome",
    description: "Label of the view name field in the Edit view modal",
  },
  "logs.viewNameRequired": {
    message: "Dê um nome à visualização.",
    description: "Inline refusal of a blank view name",
  },
  "logs.viewFiltersLabel": {
    message: "Filtros",
    description: "Heading of the filter rows in the Edit view modal",
  },
  "logs.viewAddFilter": {
    message: "Adicionar filtro",
    description: "Accessible name of the Edit view modal's add (+) button",
  },
  "logs.viewNoFilters": {
    message: "Sem filtros: a visualização lê todos os recursos do projeto.",
    description:
      "Edit view modal with no filter rows: the view reads the whole project",
  },
  "logs.viewSharedNotice": {
    message:
      "Editar os filtros atualizará a visualização para todos no projeto.",
    description:
      "Render's notice under the Edit view filters: a view is shared by the whole project",
  },
  "logs.viewSaveFailed": {
    message: "Não foi possível salvar a visualização. Tente novamente.",
    description:
      "Edit view modal error when saving failed without a server reason",
  },
  "logs.viewClose": {
    message: "Fechar",
    description:
      "Edit view modal button that discards the changes (Render's Close)",
  },
  "logs.viewDone": {
    message: "Concluir",
    description: "Edit view modal button that saves the view (Render's Done)",
  },
  "logs.viewFilterField": {
    message: "Campo",
    description: "Accessible name of a filter row's Field picker",
  },
  "logs.viewFilterOperator": {
    message: "Operador",
    description: "Accessible name of a filter row's Operator picker",
  },
  "logs.viewFilterValuesFor": {
    message: "Valores de {field}",
    description:
      "Accessible name of a filter row's Values picker; field is the row's field",
  },
  "logs.viewValuesRequired": {
    message: "Escolha pelo menos um valor.",
    description: "Inline refusal of a filter row with no value",
  },
  "logs.viewRemoveFilter": {
    message: "Remover o filtro {field}",
    description:
      "Accessible name of a filter row's trash button; field is the row's field",
  },
  "logs.viewOperatorIs": {
    message: "é",
    description:
      "Filter operator IS for a single-valued field (Environment, Time Range)",
  },
  "logs.viewOperatorIsOneOf": {
    message: "é um de",
    description: "Filter operator IS for a field that takes several values",
  },
  "logs.viewOperatorIsNotOneOf": {
    message: "não é um de",
    description: "Filter operator IS_NOT",
  },
  "logs.viewOperatorContains": {
    message: "contém",
    description: "Filter operator CONTAINS, Text Search's only operator",
  },
  "logs.viewValuesPlaceholder": {
    message: "Selecione valores",
    description:
      "Placeholder of a filter row's Values picker with nothing chosen",
  },
  "logs.viewNoResources": {
    message: "Não há recursos neste projeto",
    description:
      "Resource picker of a project with no resource the view can read",
  },
  "logs.viewValueTypeHint": {
    message: "Digite um valor e pressione Enter",
    description: "Placeholder of a typed filter value (Text Search's terms)",
  },
  "logs.viewValueRemove": {
    message: "Remover {value}",
    description:
      "Accessible name of a typed filter value's remove button; value is the value",
  },
  "logs.viewValueSearch": {
    message: "Pesquise ou digite um valor",
    description:
      "Placeholder of the search box in a filter's value picker fed by the project's logs (Level, Instance, Method, Status Code, Host, Path)",
  },
  "logs.viewValueAdd": {
    message: "Adicionar “{value}”",
    description:
      "Picker item that adds the typed text as a filter value; value is the typed text",
  },
  "logs.viewValuesLoading": {
    message: "Lendo os logs do projeto…",
    description:
      "Value picker status while the project's logs are read for the field's values",
  },
  "logs.viewValuesUnavailable": {
    message:
      "Não foi possível ler os logs do projeto. Digite um valor para adicioná-lo.",
    description:
      "Value picker status when the project's logs could not be read for the field's values",
  },
  "logs.viewValuesNoneSeen": {
    message:
      "Nenhum nos logs do projeto dos últimos 7 dias. Digite um valor para adicioná-lo.",
    description:
      "Value picker status when the project's logs hold no value of the field in the last 7 days",
  },
  "logs.viewFieldEnvironment": {
    message: "Ambiente",
    description: "Filter field Environment in the Edit view modal",
  },
  "logs.viewFieldResource": {
    message: "Recurso",
    description: "Filter field Resource in the Edit view modal",
  },
  "logs.viewFieldResourceType": {
    message: "Tipo de recurso",
    description: "Filter field Resource Type in the Edit view modal",
  },
  "logs.viewFieldText": {
    message: "Busca de texto",
    description: "Filter field Text Search in the Edit view modal",
  },
  "logs.viewFieldTimeRange": {
    message: "Intervalo de tempo",
    description: "Filter field Time Range in the Edit view modal",
  },
  "logs.viewResourceTypeService": {
    message: "Serviços",
    description: "Resource Type value Services",
  },
  "logs.viewResourceTypeCronJob": {
    message: "Cron jobs",
    description: "Resource Type value Cron jobs",
  },
  "logs.viewResourceTypePostgres": {
    message: "Postgres",
    description: "Resource Type value Postgres",
  },
  "logs.viewResourceTypeKeyValue": {
    message: "Key Value",
    description: "Resource Type value Key Value (the product name)",
  },
  "logs.lineLevel": {
    message: "Nível: {level}",
    description:
      "Log line level gutter (w3/m509): accessible name and hover text of the level icon leading a log row; {level} is the line's level word (info, warning, error, \u2026)",
  },
  "logs.copied": {
    message: "Logs copiados",
    description:
      "Log explorer (w3/m509): toast after Options > Copy logs wrote the lines shown to the clipboard",
  },
  "logs.copyFailed": {
    message: "Não foi possível copiar os logs",
    description:
      "Log explorer (w3/m509): toast when the clipboard refused the copied logs",
  },
  "logs.copyLogs": {
    message: "Copiar logs",
    description:
      "Log explorer Options menu (w3/m509): item copying every line shown to the clipboard (Render's Copy logs)",
  },
  "logs.options": {
    message: "Opções de log",
    description:
      "Log explorer (w3/m509): accessible name and tooltip of the Options (\u2026) menu button",
  },
  "logs.orderBy": {
    message: "Ordenar por",
    description:
      "Log explorer Options menu (w3/m509): heading of the line order choice (Render's Order by)",
  },
  "logs.orderAscending": {
    message: "Crescente",
    description:
      "Log explorer Options menu (w3/m509): order choice, oldest line first and the newest at the bottom",
  },
  "logs.orderDescending": {
    message: "Decrescente",
    description:
      "Log explorer Options menu (w3/m509): order choice, newest line first at the top",
  },
  "logs.display": {
    message: "Exibição:",
    description:
      "Log explorer Options menu (w3/088): the Display submenu's label, before the chosen display in bold (Render's `Display: Expand everything`)",
  },
  "logs.displaySettings": {
    message: "Configurações de exibição",
    description:
      "Log explorer Options menu (w3/088): heading of the Display choices (Render's Display settings)",
  },
  "logs.expandEverything": {
    message: "Expandir tudo",
    description:
      "Log explorer Options ▸ Display (w3/088): every log line shown whole, the default (Render's Expand everything)",
  },
  "logs.expandErrors": {
    message: "Expandir erros",
    description:
      "Log explorer Options ▸ Display (w3/088): error, critical, alert and emergency lines shown whole, every other line on one line (Render's Expand errors)",
  },
  "logs.collapseEverything": {
    message: "Recolher tudo",
    description:
      "Log explorer Options ▸ Display (w3/088): every log line on one line, cut off with an ellipsis (Render's Collapse everything)",
  },
  "logs.maximize": {
    message: "Maximizar",
    description:
      "Log explorer (w3/m509): button and menu item expanding the log to fill the screen (Render's Maximize)",
  },
  "logs.minimize": {
    message: "Sair da tela cheia",
    description:
      "Log explorer (w3/m509): button and menu item restoring the maximized log to its place in the page",
  },
  "logs.maximizeHint": {
    message: "Maximizar (M)",
    description:
      "Log explorer (w3/m509): tooltip of the Maximize button, naming its M shortcut",
  },
  "logs.minimizeHint": {
    message: "Sair da tela cheia (M ou Esc)",
    description:
      "Log explorer (w3/m509): tooltip of the button leaving full screen, naming the M and Escape keys that also leave it",
  },
  "logs.clearedTitle": {
    message: "Logs limpos",
    description:
      "Log explorer (w3/m509): title over a live tail whose lines the reader cleared with Shift+L",
  },
  "logs.clearedBody": {
    message: "As novas linhas aparecem aqui conforme chegam.",
    description:
      "Log explorer (w3/m509): body under the cleared live tail's title",
  },
  "logs.clearedRestore": {
    message: "Mostrar linhas anteriores",
    description:
      "Log explorer (w3/m509): button under a cleared live tail bringing back the lines Shift+L cleared",
  },
  "logs.clearLogs": {
    message: "Limpar logs",
    description:
      "Log explorer Options menu (w3/092): item clearing a live tail's lines, as Shift+L does (Render's Clear logs); offered only while the tail is live",
  },
};

export default ptLogs;
