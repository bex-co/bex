import type { TranslationEntry } from "@/i18n";

const ptDeploys: Record<string, TranslationEntry> = {
  "deploys.statusCreated": {
    message: "Criado",
    description: "Deploy status: created",
  },
  "deploys.statusQueued": {
    message: "Na fila",
    description: "Deploy status: queued",
  },
  "deploys.statusBuildInProgress": {
    message: "Fazendo build",
    description: "Deploy status: build_in_progress",
  },
  "deploys.statusBuildFailed": {
    message: "Falha no build",
    description: "Deploy status: build_failed",
  },
  "deploys.statusPreDeployInProgress": {
    message: "Pré-deploy em andamento",
    description: "Deploy status: pre_deploy_in_progress",
  },
  "deploys.statusPreDeployFailed": {
    message: "Falha no pré-deploy",
    description: "Deploy status: pre_deploy_failed",
  },
  "deploys.statusUpdateInProgress": {
    message: "Em andamento",
    description: "Deploy status: update_in_progress",
  },
  "deploys.statusUpdateFailed": {
    message: "Falhou",
    description: "Deploy status: update_failed",
  },
  "deploys.statusLive": {
    message: "No ar",
    description: "Deploy status: live",
  },
  "deploys.statusCanceled": {
    message: "Cancelado",
    description: "Deploy status: canceled",
  },
  "deploys.statusDeactivated": {
    message: "Desativado",
    description: "Deploy status: deactivated",
  },
  "deploys.statusUnknown": {
    message: "Desconhecido",
    description: "Deploy status: unrecognized backend value",
  },
  "deploys.statusLabel": {
    message: "Status do deploy",
    description:
      "Deploy detail header: explicit label distinguishing deploy status from service phase",
  },
  "deploys.imageDigestCopy": {
    message: "Copiar digest da imagem",
    description:
      "Deploy header: accessible label of the button copying the image digest the deploy ran",
  },
  "deploys.imageDigestCopied": {
    message: "Digest da imagem copiado",
    description: "Deploy header: toast after copying the image digest",
  },
  "deploys.imageDigestCopyError": {
    message: "Não foi possível copiar para a área de transferência",
    description: "Deploy header: toast when copying the image digest fails",
  },
  "deploys.created": {
    message: "Criado",
    description: "Deploy header: created-at label",
  },
  "deploys.updated": {
    message: "Atualizado",
    description: "Label for the deploy's latest stored transition time",
  },
  "deploys.started": {
    message: "Iniciado",
    description: "Deploy header: started-at label",
  },
  "deploys.finished": {
    message: "Concluído",
    description: "Deploy header: finished-at label",
  },
  "deploys.duration": {
    message: "Duração",
    description: "Deploy header: elapsed build/deploy time label",
  },
  "deploys.durationValue": {
    message: "Duração {duration}",
    description: "Deploy row: elapsed build/deploy time",
  },
  "deploys.durationInProgress": {
    message: "Duração em andamento",
    description: "Deploy row: elapsed time is still running",
  },
  "deploys.durationActive": {
    message: "Em andamento",
    description:
      "Deploys tab Duration column: bare running-elapsed marker (the column header supplies the label)",
  },
  "deploys.deployedAt": {
    message: "Deploy feito {timestamp}",
    description:
      'Deploy row: how long ago a live/deactivated deploy went live — {timestamp} is elapsed text like "2 hours ago" (its finish time; createdAt fallback)',
  },
  "deploys.createdAt": {
    message: "Criado {timestamp}",
    description:
      'Deploy row: how long ago a deploy that hasn\'t finished (created/queued/in-progress) was created — {timestamp} is elapsed text like "2 minutes ago"',
  },
  "deploys.canceledAt": {
    message: "Cancelado {timestamp}",
    description:
      'Deploy row: how long ago a canceled deploy was canceled — {timestamp} is elapsed text like "3 days ago" (its finish time; createdAt fallback)',
  },
  "deploys.failedAt": {
    message: "Falhou {timestamp}",
    description:
      'Deploy row: how long ago a build/pre-deploy/update failure landed — {timestamp} is elapsed text like "3 days ago" (its finish time; createdAt fallback)',
  },
  "deploys.notYet": {
    message: "—",
    description:
      "Deploy header: placeholder for a timestamp that hasn't happened yet",
  },
  "deploys.triggerCreate": {
    message: "Primeiro deploy",
    description: "Deploy header: trigger=create label",
  },
  "deploys.triggerApi": {
    message: "Deploy via API",
    description: "Deploy header: trigger=api label (an API-key deploy, w4/413)",
  },
  "deploys.triggerManual": {
    message: "Deploy manual",
    description:
      "Deploy header: trigger=manual label (a dashboard click, w4/413)",
  },
  "deploys.triggerRollback": {
    message: "Rollback para {deployId}",
    description:
      "Deploy header: trigger label for a rollback deploy, naming the restored deploy (id must stay literal — never wrap in CSS capitalize)",
  },
  "deploys.triggerDeployHook": {
    message: "Deploy Hook",
    description: "Deploy row: trigger=deploy_hook label (secret-URL trigger)",
  },
  "deploys.triggerBlueprint": {
    message: "Sincronização do Blueprint",
    description: "Deploy row: trigger=blueprint label (Blueprint sync deploy)",
  },
  "deploys.triggerNewCommit": {
    message: "Novo commit",
    description: "Deploy row: trigger=new_commit label (git push redeploy)",
  },
  "deploys.triggerConfigChange": {
    message: "Alteração de configuração",
    description:
      "Deploy row: trigger=config_change label (an env var, secret file, or env-group edit that deployed)",
  },
  "deploys.triggerSettingsChange": {
    message: "Alteração de configurações",
    description:
      "Deploy row: trigger=settings_change label (a Settings save that deployed, or a plan change; w8/m135)",
  },
  "deploys.triggerRestart": {
    message: "Reinício",
    description:
      "Deploy row: trigger=restart label (a Restart roll of the live release, w4/379)",
  },
  "deploys.triggerDeployedByRender": {
    message: "Rebuild da plataforma",
    description:
      "Deploy row: trigger=deployed_by_render label (the platform rebuilt the live commit, e.g. to ship a runtime security patch)",
  },
  "deploys.logSearchPlaceholder": {
    message: "Pesquisar logs…",
    description: "Deploy detail page: log search input placeholder",
  },
  "deploys.logTypeFilter": {
    message: "Tipo de log",
    description:
      "Deploy log viewer: aria-label of the log-type filter dropdown (Render's All/Application/Build selector)",
  },
  "deploys.logTypeAll": {
    message: "Todos os logs",
    description: "Deploy log viewer type filter: show every log type",
  },
  "deploys.logTypeApp": {
    message: "Logs da aplicação",
    description:
      "Deploy log viewer type filter: app + pre-deploy lines, matching Render's Application bucket",
  },
  "deploys.logTypeBuild": {
    message: "Logs de build",
    description: "Deploy log viewer type filter: build lines only",
  },
  "deploys.buildLogsStoreUnavailable": {
    message: "Os logs de build históricos não estão disponíveis",
    description:
      "Deploy detail page: title shown when the durable log store isn't wired",
  },
  "deploys.buildLogsStoreUnavailableBody": {
    message:
      "Este dashboard não está conectado a um armazenamento de logs durável. Os logs da aplicação ainda podem aparecer para este deploy.",
    description:
      "Deploy detail page: accurate explanation of a storeless build-log response",
  },
  "deploys.detailRefreshError": {
    message:
      "Não foi possível atualizar este deploy. Exibindo os últimos detalhes carregados.",
    description:
      "Deploy detail page: a background refresh of an already loaded deploy failed; the page keeps the last good details and offers a retry",
  },
  "deploys.logPartiallyLoaded": {
    message:
      "Apenas parte deste intervalo de tempo foi carregada. Carregue as mais antigas para pesquisar linhas anteriores.",
    description:
      "Deploy log viewer banner while older history remains unloaded. The panel's search and type/instance filters run over loaded lines only, so it must not claim a number of matches",
  },
  "deploys.logStepEndMissing": {
    message:
      "Este deploy foi cancelado antes de a etapa de pré-deploy terminar, e o horário de conclusão da etapa não foi registrado, então a saída posterior pode ficar fora desta janela. Escolha um intervalo maior em Opções de log para vê-la.",
    description:
      "Deploy log viewer: a canceled deploy's continued pre-deploy step has no recorded end, so the deploy window may omit its later output",
  },
  "deploys.logStepEndUnavailable": {
    message:
      "Não foi possível verificar quando a etapa de pré-deploy deste deploy terminou, então a saída posterior da etapa pode ficar fora desta janela. Tente novamente ou escolha um intervalo maior em Opções de log.",
    description:
      "Deploy log viewer: reading a canceled deploy's continued pre-deploy step end failed; offers a retry",
  },
  "deploys.timelineTitle": {
    message: "Linha do tempo de status",
    description: "Deploy detail page: status-timeline card title",
  },
  "deploys.timelineCreated": {
    message: "Deploy criado",
    description: "Deploy timeline: deploy row was created",
  },
  "deploys.timelineStarted": {
    message: "Deploy iniciado",
    description: "Deploy timeline: backend-provided startedAt timestamp",
  },
  "deploys.timelineInProgress": {
    message: "Deploy em andamento",
    description: "Deploy timeline: current non-terminal deploy status",
  },
  "deploys.timelineLive": {
    message: "Deploy no ar",
    description: "Deploy timeline: successful terminal state",
  },
  "deploys.timelineFailed": {
    message: "Deploy falhou",
    description: "Deploy timeline: failed terminal state",
  },
  "deploys.timelineCanceled": {
    message: "Deploy cancelado",
    description: "Deploy timeline: canceled terminal state",
  },
  "deploys.timelineDeactivated": {
    message: "Deploy desativado",
    description: "Deploy timeline: deactivated terminal state",
  },
  "deploys.timelineEventsUnavailable": {
    message:
      "Os eventos do serviço não estão disponíveis; exibindo apenas os horários do deploy.",
    description:
      "Deploy timeline: graceful fallback when service-events query fails",
  },
  "deploys.autoDeployOffTitle": {
    message: "O deploy automático está desativado",
    description:
      "Deploys page banner title when the service's auto-deploy is off (Render, w3/064)",
  },
  "deploys.autoDeployOffBody": {
    message:
      "Pushes para {branch} não fazem deploy deste serviço. Faça o deploy manualmente ou reative o deploy automático.",
    description: "Deploys page banner body; {branch} is the tracked git branch",
  },
  "deploys.autoDeployOffBodyNoBranch": {
    message:
      "Pushes não fazem deploy deste serviço. Faça o deploy manualmente ou reative o deploy automático.",
    description: "Deploys page banner body when the branch is unknown",
  },
  "deploys.autoDeployOffSettings": {
    message: "Configurações de deploy automático",
    description: "Deploys page banner link to the service Settings page",
  },
  "deploys.deploysManualTitle": {
    message: "Os deploys são manuais",
    description:
      "Deploys page banner title for a repo whose pushes can't reach bex (pushDeliveryMethod unavailable)",
  },
  "deploys.deploysManualBody": {
    message:
      "O bex não recebe os pushes deste repositório, então eles não fazem deploy deste serviço. Faça o deploy manualmente ou com o Deploy Hook, ou conecte o repositório pelo app do GitHub.",
    description:
      "Deploys page banner body for a repo whose pushes can't reach bex: no Auto-Deploy setting can turn deploys on push on (Render: a public Git URL must be deployed manually)",
  },
  "deploys.listTitle": {
    message: "Deploys",
    description: "Deploys tab: card title over the deploy-history list",
  },
  "deploys.buildsTitle": {
    message: "Builds",
    description:
      "A cron job's Builds tab: card title over its build history, Render's Builds page (w2/m360)",
  },
  "deploys.columnDeploy": {
    message: "Deploy",
    description:
      "Deploys tab table: first column header (status, id, commit, timestamp)",
  },
  "deploys.columnTrigger": {
    message: "Gatilho",
    description: "Deploys tab table: column header for what triggered a deploy",
  },
  "deploys.columnDuration": {
    message: "Duração",
    description:
      "Deploys tab table: column header for elapsed build/deploy time",
  },
  "deploys.columnActions": {
    message: "Ações",
    description:
      "Deploys tab table: accessible-only header over the Cancel/Rollback action column",
  },
  "deploys.listEmpty": {
    message: "Nenhum deploy ainda.",
    description: "Deploys tab: empty state with no status filter",
  },
  "deploys.listEmptyFiltered": {
    message: "Nenhum deploy corresponde ao status selecionado.",
    description: "Deploys tab: empty state while a status filter is active",
  },
  "deploys.listStatusFilterLabel": {
    message: "Filtrar por status",
    description: "Deploys tab: aria-label of the status filter dropdown",
  },
  "deploys.listStatusAll": {
    message: "Todos os status",
    description: "Deploys tab: status filter option matching every deploy",
  },
  "deploys.listSearchPlaceholder": {
    message: "Pesquisar deploys e commits carregados…",
    description:
      "Deploys tab: placeholder for client-side deploy-history search",
  },
  "deploys.listSearchLabel": {
    message: "Pesquisar deploys e commits carregados",
    description: "Deploys tab: accessible label for deploy-history search",
  },
  "deploys.listEmptySearch": {
    message: "Nenhum deploy carregado corresponde à sua pesquisa.",
    description:
      "Deploys tab: empty state when client-side search has no match",
  },
  "deploys.listCount_one": {
    message: "{count} deploy",
    description: "Deploys tab: complete deploy count (singular)",
  },
  "deploys.listCount_many": {
    message: "{count} de deploys",
    description: "Deploys tab: complete deploy count (plural)",
  },
  "deploys.listCount_other": {
    message: "{count} deploys",
    description: "Deploys tab: complete deploy count (plural)",
  },
  "deploys.listCountLoaded_one": {
    message: "{count} deploy carregado",
    description:
      "Deploys tab: loaded count when more server pages remain (singular)",
  },
  "deploys.listCountLoaded_many": {
    message: "{count} de deploys carregados",
    description:
      "Deploys tab: loaded count when more server pages remain (plural)",
  },
  "deploys.listCountLoaded_other": {
    message: "{count} deploys carregados",
    description:
      "Deploys tab: loaded count when more server pages remain (plural)",
  },
  "deploys.listLoadMore": {
    message: "Carregar mais",
    description: "Deploys tab: button fetching the next page of deploy history",
  },
  "deploys.listLoadMoreError": {
    message: "Não foi possível carregar deploys mais antigos.",
    description:
      "Deploys tab: toast when reading an older page of deploy history fails",
  },
  "deploys.logOptions": {
    message: "Opções de log",
    description:
      "Deploy log viewer: aria-label/tooltip of the options menu button",
  },
  "deploys.logRangeLabel": {
    message: "Intervalo de tempo",
    description:
      "Deploy log viewer options menu: heading over the time-range choices",
  },
  "deploys.logRangeDeploy": {
    message: "Janela do deploy",
    description:
      "Deploy log viewer: the default time range — the deploy's own createdAt..finishedAt window",
  },
  "deploys.logRangeLast15m": {
    message: "Últimos 15 minutos",
    description: "Deploy log viewer: relative time-range option (?r=15m)",
  },
  "deploys.logRangeLast1h": {
    message: "Última hora",
    description: "Deploy log viewer: relative time-range option (?r=1h)",
  },
  "deploys.logRangeLast6h": {
    message: "Últimas 6 horas",
    description: "Deploy log viewer: relative time-range option (?r=6h)",
  },
  "deploys.logRangeLast24h": {
    message: "Últimas 24 horas",
    description: "Deploy log viewer: relative time-range option (?r=24h)",
  },
  "deploys.logRangeLast7d": {
    message: "Últimos 7 dias",
    description: "Deploy log viewer: relative time-range option (?r=7d)",
  },
  "deploys.logWrap": {
    message: "Quebrar linhas",
    description:
      "Deploy log viewer options menu: toggle wrapping long log lines vs horizontal scroll",
  },
  "deploys.logTimestamps": {
    message: "Mostrar timestamps",
    description:
      "Deploy log viewer options menu: toggle the per-line timestamp column",
  },
  "deploys.logMaximize": {
    message: "Maximizar",
    description:
      "Deploy log viewer: button expanding the viewer to fill the screen",
  },
  "deploys.logMinimize": {
    message: "Sair da tela cheia",
    description:
      "Deploy log viewer: button restoring the maximized viewer to its inline size",
  },
  "deploys.reason.http_port_waiting": {
    message: "aguardando o contêiner iniciar antes de detectar sua porta HTTP",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_waiting, w4/566)",
  },
  "deploys.reason.http_port_detecting": {
    message: "detectando a porta HTTP do contêiner",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_detecting, w4/566)",
  },
  "deploys.reason.http_port_limit": {
    message:
      "a porta HTTP detectada excede o limite de 75 portas do serviço; defina PORT explicitamente",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_limit, w4/566)",
  },
  "deploys.reason.http_port_detected": {
    message: "HTTP detectado na porta {port}; atendendo por ela",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_detected, w4/566); params are the sentence's values",
  },
  "deploys.reason.http_port_not_answering": {
    message:
      "A porta {port} aceitou uma conexão, mas ainda não respondeu HTTP; continuando a varredura enquanto o contêiner inicia",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_not_answering, w4/566); params are the sentence's values",
  },
  "deploys.reason.http_port_none_yet": {
    message:
      "Nenhuma porta responde HTTP ainda; continuando a varredura enquanto o contêiner inicia",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_none_yet, w4/566)",
  },
  "deploys.reason.http_port_fallback": {
    message:
      "Nenhuma porta respondeu HTTP; usando a porta de fallback {port}. Defina PORT como o listener HTTP se ele escutar em outra porta",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_fallback, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_not_found": {
    message:
      "a imagem {image} não foi encontrada no registry: o repositório ou a tag não existe. Verifique a URL e a tag da imagem, ou faça deploy de um digest que exista",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_not_found, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_credential_refused": {
    message:
      "{host} recusou a credencial de registro deste serviço para a imagem {image} (HTTP {status}). Verifique o usuário e o token da credencial, ou remova a credencial deste serviço para baixar uma imagem pública anonimamente",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_credential_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_access_refused": {
    message:
      "o registry recusou o acesso à imagem {image}. Se a imagem for privada, adicione uma credencial de registro para {host} a este serviço",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_access_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_read_refused": {
    message:
      "o registry recusou a leitura da imagem {image} (HTTP {status}). Verifique a URL e a tag da imagem",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_read_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.build_timed_out": {
    message:
      "o build não terminou dentro da janela de build; verifique os logs de build",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason build_timed_out, w4/566)",
  },
  "deploys.reason.pre_deploy_timed_out": {
    message:
      "o comando de pré-deploy não terminou dentro da janela; verifique os logs de pré-deploy",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_timed_out, w4/566)",
  },
  "deploys.reason.health_timed_out": {
    message:
      "o deploy não ficou saudável dentro da janela do health gate; verifique os logs do serviço",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason health_timed_out, w4/566)",
  },
  "deploys.reason.startup_out_of_memory": {
    message:
      "a instância ficou sem memória durante a inicialização (limite de {memory} no tipo de instância; código de saída {code}) — reduza o uso de memória ou escolha um tipo de instância maior",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_out_of_memory, w4/630); params are the sentence's values: memory is the instance type's memory limit (e.g. 512Mi), code the exit code",
  },
  "deploys.reason.probe_http_waiting": {
    message:
      "o contêiner está em execução, mas o health check {kind} ainda não teve sucesso, então o rollout está aguardando: GET {path} na porta {port}. Verifique se o caminho retorna um status 2xx ou 3xx na porta do próprio serviço.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_waiting, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_waiting_restarted": {
    message:
      "o contêiner está em execução, mas o health check {kind} ainda não teve sucesso, então o rollout está aguardando: GET {path} na porta {port}. O health check de liveness já reiniciou o contêiner {restarts} vez(es), então a verificação está falhando, e não apenas lenta. Verifique se o caminho retorna um status 2xx ou 3xx na porta do próprio serviço.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_waiting_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_failed": {
    message:
      "o contêiner está em execução, mas o health check {kind} nunca teve sucesso dentro da janela de rollout, então o deploy falhou: GET {path} na porta {port}. Verifique se o caminho retorna um status 2xx ou 3xx na porta do próprio serviço.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_failed, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_failed_restarted": {
    message:
      "o contêiner está em execução, mas o health check {kind} nunca teve sucesso dentro da janela de rollout, então o deploy falhou: GET {path} na porta {port}. O health check de liveness já reiniciou o contêiner {restarts} vez(es), então a verificação está falhando, e não apenas lenta. Verifique se o caminho retorna um status 2xx ou 3xx na porta do próprio serviço.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_failed_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_waiting": {
    message:
      "o contêiner está em execução, mas o health check {kind} ainda não teve sucesso, então o rollout está aguardando: uma conexão TCP com a porta {port}. Nenhum Health Check Path está definido, então a verificação é uma conexão TCP, que apenas pergunta se o processo está escutando nessa porta.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_waiting, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_waiting_restarted": {
    message:
      "o contêiner está em execução, mas o health check {kind} ainda não teve sucesso, então o rollout está aguardando: uma conexão TCP com a porta {port}. O health check de liveness já reiniciou o contêiner {restarts} vez(es), então a verificação está falhando, e não apenas lenta. Nenhum Health Check Path está definido, então a verificação é uma conexão TCP, que apenas pergunta se o processo está escutando nessa porta.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_waiting_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_failed": {
    message:
      "o contêiner está em execução, mas o health check {kind} nunca teve sucesso dentro da janela de rollout, então o deploy falhou: uma conexão TCP com a porta {port}. Nenhum Health Check Path está definido, então a verificação é uma conexão TCP, que apenas pergunta se o processo está escutando nessa porta.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_failed, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_failed_restarted": {
    message:
      "o contêiner está em execução, mas o health check {kind} nunca teve sucesso dentro da janela de rollout, então o deploy falhou: uma conexão TCP com a porta {port}. O health check de liveness já reiniciou o contêiner {restarts} vez(es), então a verificação está falhando, e não apenas lenta. Nenhum Health Check Path está definido, então a verificação é uma conexão TCP, que apenas pergunta se o processo está escutando nessa porta.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_failed_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.rollout_deadline_exceeded": {
    message: "o rollout não ficou saudável dentro do prazo de progresso",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason rollout_deadline_exceeded, w4/591)",
  },
  "deploys.reason.pre_deploy_window_exceeded": {
    message:
      "o comando de pré-deploy não terminou dentro da janela de {minutes} minutos; verifique os logs de pré-deploy",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_window_exceeded, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_out_of_memory": {
    message:
      "o comando de pré-deploy ficou sem memória (código de saída {code}); verifique os logs de pré-deploy",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_out_of_memory, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_exited": {
    message:
      "o comando de pré-deploy terminou com o código {code}; verifique os logs de pré-deploy",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_exited, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_failed": {
    message: "o comando de pré-deploy falhou; verifique os logs de pré-deploy",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_failed, w4/591)",
  },
  "deploys.reason.pre_deploy_could_not_launch": {
    message:
      "o comando de pré-deploy não pôde iniciar: o runtime de contêineres não conseguiu executar seu programa; o programa nunca rodou, então não há logs de pré-deploy",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_could_not_launch, w4/591)",
  },
  "deploys.reason.build_never_started": {
    message: "o build nunca iniciou: {cause}",
    description:
      "A deploy failure reason by its code (bex-api DeployReason build_never_started, w4/591); {cause} is the operator's wait, whose sentences are worded as parts (w5/m356)",
  },
  "deploys.reason.build_abandoned": {
    message:
      "o build não terminou dentro da janela de build, e o registro App deste serviço não pôde ser lido para explicar o motivo",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason build_abandoned, w4/591)",
  },
  "deploys.reason.deploy_abandoned": {
    message:
      "o deploy não terminou dentro da janela, e o registro App deste serviço não pôde ser lido para explicar o motivo",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason deploy_abandoned, w4/591)",
  },
  "deploys.reason.platform_error": {
    message:
      "o bex não pôde aplicar esta release por um erro da plataforma, não por um problema no seu código ou nas suas configurações. Entre em contato com o suporte informando a referência {ref}",
    description:
      "A deploy failure reason by its code (bex-api DeployReason platform_error, w9/090): the platform refused one of bex's own changes; {ref} is the support reference, shown as-is",
  },
  "deploys.reason.canceled_suspended": {
    message: "Cancelado: o serviço foi suspenso",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_suspended, w4/591)",
  },
  "deploys.reason.superseded_by_newer_release": {
    message: "Substituído por uma release mais recente",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason superseded_by_newer_release, w4/591)",
  },
  "deploys.reason.superseded_by": {
    message: "Substituído por {deploy}",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason superseded_by, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_by_restart": {
    message:
      "Cancelado pelo reinício {deploy}, que manteve a release em execução: o que este deploy estava aplicando não foi implantado",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_by_restart, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_along_with": {
    message: "Cancelado junto com {deploy}",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_along_with, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_along_with_newer": {
    message: "Cancelado junto com um deploy mais recente",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_along_with_newer, w4/591)",
  },
  "deploys.reason.restart_service_gone": {
    message: "o serviço não existe mais",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason restart_service_gone, w4/591)",
  },
  "deploys.reason.restart_wait_timed_out": {
    message: "o deploy que este reinício aguardava não terminou em {wait}",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason restart_wait_timed_out, w4/591); params are the sentence's values",
  },
  "deploys.reason.deploy_wait_timed_out": {
    message: "o deploy que este aguardava não terminou em {wait}",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason deploy_wait_timed_out, w4/652): a Rollback or another deploy that waited for the deploy in flight; params are the sentence's values",
  },
  "deploys.reason.probe_other_waiting": {
    message:
      "o contêiner está em execução, mas o health check {kind} ainda não teve sucesso, então o rollout está aguardando: o health check configurado. Verifique se o caminho retorna um status 2xx ou 3xx na porta do próprio serviço.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_waiting, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_waiting_restarted": {
    message:
      "o contêiner está em execução, mas o health check {kind} ainda não teve sucesso, então o rollout está aguardando: o health check configurado. O health check de liveness já reiniciou o contêiner {restarts} vez(es), então a verificação está falhando, e não apenas lenta. Verifique se o caminho retorna um status 2xx ou 3xx na porta do próprio serviço.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_waiting_restarted, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_failed": {
    message:
      "o contêiner está em execução, mas o health check {kind} nunca teve sucesso dentro da janela de rollout, então o deploy falhou: o health check configurado. Verifique se o caminho retorna um status 2xx ou 3xx na porta do próprio serviço.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_failed, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_failed_restarted": {
    message:
      "o contêiner está em execução, mas o health check {kind} nunca teve sucesso dentro da janela de rollout, então o deploy falhou: o health check configurado. O health check de liveness já reiniciou o contêiner {restarts} vez(es), então a verificação está falhando, e não apenas lenta. Verifique se o caminho retorna um status 2xx ou 3xx na porta do próprio serviço.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_failed_restarted, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.hint_image_default_port": {
    message:
      " Nada aceitou uma conexão TCP na porta {port}, o padrão porque a imagem não declara portas TCP. O servidor deve escutar o host 0.0.0.0 na porta de PORT ({port}).",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_image_default_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_image_declared_ports": {
    message:
      " Nada aceitou uma conexão TCP na porta {port}, a porta principal escolhida entre as portas TCP declaradas pela imagem ({ports}). Faça o servidor escutar 0.0.0.0 em uma dessas portas, ou defina a porta do serviço como aquela em que ele escuta.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_image_declared_ports, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_env_port_only": {
    message:
      " Nada aceitou uma conexão TCP na porta {port}, definida pela variável de ambiente PORT do serviço; a imagem não declara nenhuma outra porta TCP. Faça o servidor escutar 0.0.0.0 na porta {port}, ou altere a porta do serviço para aquela em que ele escuta.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_env_port_only, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_configured_port_only": {
    message:
      " Nada aceitou uma conexão TCP na porta {port}, definida pela porta configurada do serviço; a imagem não declara nenhuma outra porta TCP. Faça o servidor escutar 0.0.0.0 na porta {port}, ou altere a porta do serviço para aquela em que ele escuta.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_configured_port_only, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_env_port_declared": {
    message:
      " Nada aceitou uma conexão TCP na porta {port}, definida pela variável de ambiente PORT do serviço; a imagem declara portas TCP ({ports}). Altere a variável de ambiente PORT para a porta em que ele escuta, ou remova-a para usar a porta da imagem.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_env_port_declared, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_configured_port_declared": {
    message:
      " Nada aceitou uma conexão TCP na porta {port}, definida pela porta configurada do serviço; a imagem declara portas TCP ({ports}). Altere a porta do serviço em Configurações para aquela em que ele escuta, ou remova-a para usar a porta da imagem.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_configured_port_declared, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_background_worker": {
    message:
      " Se o serviço não precisa receber tráfego em nenhuma porta, crie um Background Worker.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_background_worker, w5/m356); en starts with the space that joins it to the sentence before",
  },
  "deploys.reason.hint_no_http_discovered": {
    message:
      " Nenhuma porta respondeu HTTP durante a descoberta na inicialização.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_http_discovered, w5/m356); en starts with the space that joins it to the sentence before",
  },
  "deploys.reason.hint_listening": {
    message: " O processo está escutando na porta {port}.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_adopted": {
    message:
      " O processo está escutando na porta {port}, que o serviço agora usa.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_adopted, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_ports": {
    message:
      " O processo está escutando nas portas {ports}, e não na porta {port}; defina a porta do serviço (ou sua variável PORT) como a que atende HTTP.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_ports, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_primary": {
    message:
      " O processo está escutando na porta {port}, que o serviço agora usa como porta principal.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_primary, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_port": {
    message:
      " Nenhuma porta aberta foi detectada na instância: o servidor deve escutar o host 0.0.0.0 na porta de PORT ({port}).",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_port_before": {
    message:
      " Nenhuma porta aberta foi detectada na instância: o servidor deve escutar o host 0.0.0.0 na porta de PORT ({port}); antes ele escutava na porta {previous}.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_port_before, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_service_port": {
    message:
      " Nenhuma porta aberta foi detectada na instância: o servidor deve escutar o host 0.0.0.0 na porta do serviço ({port}).",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_service_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.queued_build_limit": {
    message:
      "{noun} tem {active}/{limit} builds simultâneos ativos; aguardando uma vaga",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_build_limit, w5/m356): a full concurrent-build cap; {noun} is the cap's scope, worded by deploys.reasonParam.noun.*",
  },
  "deploys.reason.queued_cluster_full": {
    message:
      "a capacidade de build do cluster está esgotada; aguardando uma vaga",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_cluster_full, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_slot_taken": {
    message:
      "um despacho simultâneo ocupou a última vaga de build; aguardando uma vaga",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_slot_taken, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_database": {
    message:
      "Aguardando o banco de dados vinculado {database} ficar disponível antes do build",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_database, w5/m356): what the build waited on; params are the sentence's values",
  },
  "deploys.reason.queued_datastore": {
    message:
      "Aguardando os detalhes de conexão de um datastore vinculado antes do build",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_datastore, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_build_credential": {
    message: "Aguardando o registry aceitar a credencial de build deste app",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_build_credential, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_pull_credential": {
    message: "Aguardando o registry aceitar a credencial de pull deste app",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_pull_credential, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity": {
    message: "aguardando capacidade de build: {detail}",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity, w5/m356): waiting for build capacity; {detail} is the build plane's own wait, shown as-is",
  },
  "deploys.reason.queued_capacity_pod": {
    message:
      "aguardando capacidade de build: job de build despachado; aguardando a criação do pod de build",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_pod, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity_no_node": {
    message:
      "aguardando capacidade de build: nenhum nó de build tem espaço para ele ainda",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_no_node, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity_short": {
    message:
      "aguardando capacidade de build: nenhum nó de build tem espaço para ele ainda (sem {resources} livre suficiente)",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_short, w5/m356): what the build waited on; params are the sentence's values",
  },
  "deploys.reasonParam.kind.startup": {
    message: "de inicialização",
    description:
      "A deploy reason's {kind} value startup (w4/591): which health check a stalled rollout is waiting on",
  },
  "deploys.reasonParam.kind.readiness": {
    message: "de prontidão",
    description:
      "A deploy reason's {kind} value readiness (w4/591): which health check a stalled rollout is waiting on",
  },
  "deploys.reasonParam.noun.workspace": {
    message: "o workspace",
    description:
      "A deploy reason's {noun} value workspace (w5/m356): the scope of a full concurrent-build cap",
  },
  "deploys.reasonWait.duration": {
    message: "{units}",
    description:
      "A deferred restart's wait, restart_wait_timed_out's {wait} (w5/m356): {wait} is bex-api's Go duration (3h0m0s), which en keeps so the sentence reads as the API's; {units} is the duration in words (deploys.reasonWait.hours and the rest), which other languages read",
  },
  "deploys.reasonWait.hours": {
    message: "{hours} h",
    description: "A deferred restart's wait in words (w5/m356): its hours",
  },
  "deploys.reasonWait.minutes": {
    message: "{minutes} min",
    description: "A deferred restart's wait in words (w5/m356): its minutes",
  },
  "deploys.reasonWait.seconds": {
    message: "{seconds} s",
    description: "A deferred restart's wait in words (w5/m356): its seconds",
  },
};

export default ptDeploys;
