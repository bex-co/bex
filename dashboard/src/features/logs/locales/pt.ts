import type { TranslationEntry } from "@/i18n";

const ptLogs: Record<string, TranslationEntry> = {
  "logs.rangeLabel": {
    message: "Intervalo do histórico de logs",
    description: "Accessible label for the relative log-history range controls",
  },
  "logs.rangeHistoryNote": {
    message:
      "O intervalo limita o histórico. O modo ao vivo acrescenta novas linhas conforme elas chegam.",
    description:
      "Explains how the bounded history window interacts with live tail",
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
      "O acompanhamento ao vivo está desativado — método, status, caminho e host filtram logs de requisições, então os logs do app não podem ser acompanhados com eles.",
    description:
      "Note when the chosen log type cannot carry an active filter, so live tail is off (a request filter on app logs)",
  },
  "logs.liveWindowClosed": {
    message:
      "O acompanhamento ao vivo está desativado — este intervalo personalizado já terminou, então nenhuma linha nova pode chegar nele.",
    description:
      "Note when the selected custom time range ended before it was applied, so live tail is off",
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
  "logs.live": {
    message: "Ao vivo",
    description: "Label for the live-tail on/off toggle",
  },
  "logs.jumpToLatest": {
    message: "Ir para o mais recente",
    description: "Button to re-pin autoscroll to the newest log line",
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
  "logs.paused": {
    message: "Acompanhamento ao vivo pausado",
    description: "Status under the log list when live tail is toggled off",
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
  "logs.errorTitleWithReason": {
    message: "Não foi possível carregar os logs: {reason}",
    description:
      "Inline banner over loaded log lines when a later read was refused; reason is the server's refusal",
  },
};

export default ptLogs;
