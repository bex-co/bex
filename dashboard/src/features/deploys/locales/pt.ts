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
  "deploys.duration": {
    message: "Duração",
    description: "Deploy header: elapsed build/deploy time label",
  },
  "deploys.factStatus": {
    message: "Status",
    description:
      "Deploy page: label of the deploy's Status fact (Render's STATUS)",
  },
  "deploys.factDeployed": {
    message: "Implantado",
    description:
      "Deploy page: label of the fact giving when the deploy was created (Render's DEPLOYED)",
  },
  "deploys.factSource": {
    message: "Origem",
    description:
      "Deploy page: label of the fact naming the commit or image the deploy ran (Render's SOURCE)",
  },
  "deploys.factPreDeploy": {
    message: "Pré-deploy",
    description:
      "Deploy page: label of the fact giving the outcome of the deploy's pre-deploy command",
  },
  "deploys.preDeployRunning": {
    message: "Em execução",
    description:
      "Deploy page: Pre-deploy fact value while the pre-deploy command runs",
  },
  "deploys.preDeploySucceeded": {
    message: "Bem-sucedido",
    description:
      "Deploy page: Pre-deploy fact value once the pre-deploy command succeeded",
  },
  "deploys.preDeployFailed": {
    message: "Falhou",
    description:
      "Deploy page: Pre-deploy fact value when the pre-deploy command failed",
  },
  "deploys.preDeployCanceled": {
    message: "Cancelado",
    description:
      "Deploy page: Pre-deploy fact value when the pre-deploy command was canceled",
  },
  "deploys.idLabel": {
    message: "ID do deploy",
    description:
      "Deploy page: the deploy's id, named in its copy button's accessible name",
  },
  "deploys.idCopied": {
    message: "ID do deploy copiado",
    description: "Deploy page: toast after copying the deploy's id",
  },
  "deploys.idCopyFailed": {
    message: "Não foi possível copiar o ID do deploy",
    description: "Deploy page: toast when copying the deploy's id fails",
  },
  "deploys.loadErrorTitle": {
    message: "Não foi possível carregar este deploy",
    description:
      "Deploy page: title of the state shown when the deploy itself could not be read",
  },
  "deploys.loadErrorBody": {
    message:
      "Não foi possível ler este deploy agora. O deploy em si não foi afetado.",
    description:
      "Deploy page: body of the state shown when the deploy itself could not be read; says the deploy is unaffected",
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
  "deploys.triggerServiceResumed": {
    message: "Serviço retomado",
    description:
      "Deploy row: trigger=service_resumed label (the deploy a Resume opened on the release the service was serving, Render's service_resumed, w3/m643)",
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
    message: "Pesquisar deploys e commits",
    description:
      "Placeholder of the deploy list's search field on every service's Deploys page (Render's text, w3/m555)",
  },
  "deploys.listSearchLabel": {
    message: "Pesquisar deploys e commits carregados",
    description: "Deploys tab: accessible label for deploy-history search",
  },
  "deploys.manualBuild": {
    message: "Build manual",
    description:
      "Button beside a repo-backed cron job's deploy search that builds its latest commit and opens the new build (Render's Manual Build, w3/m555)",
  },
  "deploys.listClearSearch": {
    message: "Limpar busca",
    description:
      "Deploys tab: button under the no-match message that empties the search box",
  },
  "deploys.listMatchCount": {
    message: "Correspondências: {matched} de {total} carregados",
    description:
      "Deploys tab: count shown while a search is typed; matched = loaded deploys that match, total = loaded deploys",
  },
  "deploys.listLoadErrorTitle": {
    message: "Não foi possível carregar os deploys",
    description:
      "Deploys tab: heading of the error state when the deploy history could not be read; a Try again button follows",
  },
  "deploys.listLoadErrorBody": {
    message:
      "Seus deploys não foram afetados. Verifique sua conexão e tente novamente.",
    description:
      "Deploys tab: body of the error state when the deploy history could not be read and the server gave no message; a Try again button follows",
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
  "deploys.reason.http_port_localhost_only": {
    message:
      "Nenhuma porta aberta foi detectada em 0.0.0.0, mas há portas abertas em localhost ({ports}); faça o servidor escutar o host 0.0.0.0. Continuando a varredura enquanto o contêiner inicia",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_localhost_only, w8/388); params are the sentence's values: ports is the comma-separated list of ports the process listens on only at localhost",
  },
  "deploys.reason.http_port_none_http": {
    message:
      "Há portas abertas em 0.0.0.0 ({ports}), mas nenhuma responde HTTP ainda; continuando a varredura enquanto o contêiner inicia. Se o serviço não precisa receber tráfego HTTP público, crie um Private Service",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_none_http, w8/388); params are the sentence's values: ports is the comma-separated list of open ports that answer no HTTP",
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
  "deploys.reason.image_registry_unreachable": {
    message:
      "o registry {host} não respondeu à leitura da imagem {image} por {minutes} minutos. Aguarde e faça o deploy novamente, ou verifique a página de status do provedor do registry",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_registry_unreachable, w4/692); params are the sentence's values",
  },
  "deploys.reason.image_registry_unavailable": {
    message:
      "o registry {host} está indisponível e não conseguiu servir a imagem {image}. Aguarde e faça o deploy novamente, ou verifique a página de status do provedor do registry",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_registry_unavailable, w4/m669): Render's Registry unavailable cause; params are the sentence's values",
  },
  "deploys.reason.image_registry_rate_limited": {
    message:
      "o registry {host} limitou o pull da imagem {image} pelo limite de requisições (HTTP 429). Aguarde o limite ser redefinido, use uma credencial de registry de uma conta com limite maior ou espelhe a imagem em outro registry",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_registry_rate_limited, w4/m669): Render's Rate limited (429 error) cause with its three remedies; params are the sentence's values",
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
      "a instância ficou sem memória durante a inicialização (limite de {memory} no plano de computação; código de saída {code}) — reduza o uso de memória ou escolha um plano de computação maior",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_out_of_memory, w4/630); params are the sentence's values: memory is the compute plan's memory limit (e.g. 512Mi), code the exit code",
  },
  "deploys.reason.startup_crash_restarting": {
    message:
      "o contêiner encerrou logo após iniciar e está reiniciando repetidamente (último código de saída {code}) — verifique os logs do serviço para ver a saída da falha.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_crash_restarting, w8/386); params are the sentence's values: code is the exit code, hint the port advice (a part)",
  },
  "deploys.reason.startup_crash_stopped": {
    message:
      "o contêiner encerrou logo após iniciar e sua instância parou (último código de saída {code}) — verifique os logs do serviço para ver a saída da falha.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_crash_stopped, w8/386); params are the sentence's values: code is the exit code, hint the port advice (a part)",
  },
  "deploys.reason.startup_early_exit_restarting": {
    message:
      'o contêiner encerrou logo após iniciar e está reiniciando repetidamente (último código de saída 0): o processo encerrou "com sucesso", mas cedo demais. Um serviço roda continuamente e não deve encerrar até receber SIGTERM; use um Cron Job para uma tarefa que termina quando seu trabalho acaba. Verifique nos logs do serviço por que ele encerrou.{hint}',
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_early_exit_restarting, w8/386); the start command exited with code 0; hint is the port advice (a part)",
  },
  "deploys.reason.startup_early_exit_stopped": {
    message:
      'o contêiner encerrou logo após iniciar e sua instância parou (último código de saída 0): o processo encerrou "com sucesso", mas cedo demais. Um serviço roda continuamente e não deve encerrar até receber SIGTERM; use um Cron Job para uma tarefa que termina quando seu trabalho acaba. Verifique nos logs do serviço por que ele encerrou.{hint}',
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_early_exit_stopped, w8/386); the start command exited with code 0; hint is the port advice (a part)",
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
  "deploys.reason.probe_open_port_waiting": {
    message:
      "o contêiner está em execução, mas o health check {kind} ainda não teve sucesso, então o rollout está aguardando: uma conexão TCP com qualquer porta aberta da instância. Nenhum Health Check Path está definido, então a verificação é uma conexão TCP, que apenas pergunta se o processo está escutando em pelo menos uma porta.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_open_port_waiting, w8/384): the stall of Render's default check, a TCP connect to any open port, which a first web image release with no Health Check Path runs; params are the sentence's values",
  },
  "deploys.reason.probe_open_port_waiting_restarted": {
    message:
      "o contêiner está em execução, mas o health check {kind} ainda não teve sucesso, então o rollout está aguardando: uma conexão TCP com qualquer porta aberta da instância. O health check de liveness já reiniciou o contêiner {restarts} vez(es), então a verificação está falhando, e não apenas lenta. Nenhum Health Check Path está definido, então a verificação é uma conexão TCP, que apenas pergunta se o processo está escutando em pelo menos uma porta.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_open_port_waiting_restarted, w8/384): the stall of Render's default check, a TCP connect to any open port, which a first web image release with no Health Check Path runs; params are the sentence's values",
  },
  "deploys.reason.probe_open_port_failed": {
    message:
      "o contêiner está em execução, mas o health check {kind} nunca teve sucesso dentro da janela de rollout, então o deploy falhou: uma conexão TCP com qualquer porta aberta da instância. Nenhum Health Check Path está definido, então a verificação é uma conexão TCP, que apenas pergunta se o processo está escutando em pelo menos uma porta.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_open_port_failed, w8/384): the stall of Render's default check, a TCP connect to any open port, which a first web image release with no Health Check Path runs; params are the sentence's values",
  },
  "deploys.reason.probe_open_port_failed_restarted": {
    message:
      "o contêiner está em execução, mas o health check {kind} nunca teve sucesso dentro da janela de rollout, então o deploy falhou: uma conexão TCP com qualquer porta aberta da instância. O health check de liveness já reiniciou o contêiner {restarts} vez(es), então a verificação está falhando, e não apenas lenta. Nenhum Health Check Path está definido, então a verificação é uma conexão TCP, que apenas pergunta se o processo está escutando em pelo menos uma porta.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_open_port_failed_restarted, w8/384): the stall of Render's default check, a TCP connect to any open port, which a first web image release with no Health Check Path runs; params are the sentence's values",
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
  "deploys.reason.hint_env_port_closed": {
    message:
      " Nada aceitou uma conexão TCP na porta {port}, definida pela variável de ambiente PORT do serviço. Vincule o servidor a 0.0.0.0 na porta {port} ou altere a variável de ambiente PORT para a porta em que ele escuta.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_env_port_closed, w8/416): a web service's PORT names the one port Render scans; en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_last_result": {
    message: " Último resultado: {result}.",
    description:
      'What the failing health check last got back, which the operator appends to a health-check stall (bex-api DeployReason part hint_last_result, w6/m634); {result} is Render\'s English message ("HTTP health check failed with status code 503"), shown verbatim in every language as Render shows it; en starts with the space that joins it to the sentence before',
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
  "deploys.reason.hint_no_open_port_localhost": {
    message:
      " Nenhuma porta aberta foi detectada em 0.0.0.0, mas há portas abertas em localhost ({ports}): o servidor deve escutar o host 0.0.0.0 na porta de PORT ({port}).",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_port_localhost, w8/388); en starts with the space that joins it to the sentence before; params are the sentence's values: ports is the comma-separated list of ports the process listens on only at localhost",
  },
  "deploys.reason.hint_no_open_http_port": {
    message:
      " Há portas abertas em 0.0.0.0 ({ports}), mas nenhuma responde HTTP: o servidor deve servir HTTP na porta de PORT ({port}). Se o serviço não precisa receber tráfego HTTP público, crie um Private Service.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_http_port, w8/388); en starts with the space that joins it to the sentence before; params are the sentence's values: ports is the comma-separated list of open ports that answer no HTTP",
  },
  "deploys.reason.hint_reserved_port": {
    message:
      " O processo serve HTTP na porta {reserved}, que é reservada: vincule o servidor a 0.0.0.0 na porta de PORT ({port}). As portas reservadas são 18012, 18013 e 19099.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_reserved_port, w1/m659); en starts with the space that joins it to the sentence before; params are the sentence's values: reserved is the reserved port the process serves HTTP on, port the one in PORT",
  },
  "deploys.reason.hint_crash_port_bind": {
    message:
      " Se a falha for de vinculação de porta: o processo deve escutar em $PORT ({port}).",
    description:
      "A startup crash's port advice, by its code (bex-api DeployReason part hint_crash_port_bind, w8/386); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_early_exit_port": {
    message:
      " O servidor deve continuar escutando em $PORT ({port}) até receber SIGTERM.",
    description:
      "A startup crash's port advice, by its code (bex-api DeployReason part hint_early_exit_port, w8/386); en starts with the space that joins it to the sentence before; params are the sentence's values",
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
  "deploys.factBuild": {
    message: "Compilação",
    description:
      "Deploy page: label of the fact naming the Build Source build ID (bld-…) this deploy ran",
  },
  "deploys.factBuildSource": {
    message: "Origem de build",
    description:
      "Deploy page: label of the fact naming the linked Build Source (name and bds-… ID)",
  },
  "deploys.buildIdLabel": {
    message: "ID do build",
    description: "Deploy page: accessible field name for copying the Build ID",
  },
  "deploys.buildIdCopied": {
    message: "ID do build copiado",
    description: "Deploy page: toast shown after copying the Build ID",
  },
  "deploys.buildIdCopyFailed": {
    message: "Não foi possível copiar o ID do build",
    description: "Deploy page: toast shown when copying the Build ID fails",
  },
  "deploys.buildSourceIdLabel": {
    message: "ID da origem de build",
    description:
      "Deploy page: accessible field name for copying the Build Source ID",
  },
  "deploys.buildSourceIdCopied": {
    message: "ID da origem de build copiado",
    description: "Deploy page: toast shown after copying the Build Source ID",
  },
  "deploys.buildSourceIdCopyFailed": {
    message: "Não foi possível copiar o ID da origem de build",
    description:
      "Deploy page: toast shown when copying the Build Source ID fails",
  },
  "deploys.noticesTitle": {
    message: "Avisos",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): the section heading over the list, shown in small capitals",
  },
  "deploys.noticeTroubleshoot": {
    message:
      "Leia nossa {link} para ver formas comuns de resolver problemas no seu deploy.",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): the line under a failed deploy's reason; {link} is the noticeTroubleshootLink link to bex's troubleshooting docs",
  },
  "deploys.noticeTroubleshootLink": {
    message: "documentação",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): the link text inside noticeTroubleshoot",
  },
  "deploys.noticeBuildCacheCleared": {
    message: "Cache de build limpo",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): the deploy was opened with Clear build cache & deploy",
  },
  "deploys.noticeFreePlan": {
    message: "Alterado para um plano de computação gratuito",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a settings deploy that moved the service from a paid plan to the free one",
  },
  "deploys.noticeMaintenance": {
    message:
      "A bex executa periodicamente manutenções sem tempo de inatividade para garantir a confiabilidade da plataforma.",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a deploy the platform itself started (trigger deployed_by_render)",
  },
  "deploys.noticeRolledBackTo": {
    message: "Rollback para {deploy}",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a rollback deploy; {deploy} is the target deploy's id, a link",
  },
  "deploys.noticeBuildGone": {
    message:
      "O rollback não está disponível porque este build não existe mais.",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a past deploy whose build is no longer stored",
  },
  "deploys.noticeSyncedFrom": {
    message: "Sincronizado de {blueprint}",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a deploy a Blueprint sync opened; {blueprint} is the Blueprint's id, a link",
  },
  "deploys.noticeInProgress": {
    message: "{deploy} está em andamento",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a queued deploy; {deploy} is the noticeAnotherDeploy link to the deploy in progress",
  },
  "deploys.noticeAnotherDeploy": {
    message: "Outro deploy",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): the link text inside noticeInProgress",
  },
  "deploys.noticeCancelPolicy": {
    message:
      "Para evitar cancelar deploys ativos quando novos são acionados, ajuste sua {policy}",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): under a deploy a newer one canceled, when the workspace's Overlapping Deploy Policy is Override; {policy} is the noticePolicyLink link to Workspace Settings",
  },
  "deploys.noticeQueuePolicy": {
    message: "Para evitar deploys na fila, ajuste sua {policy}",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): under a queued deploy, when the workspace's Overlapping Deploy Policy is Wait; {policy} is the noticePolicyLink link to Workspace Settings",
  },
  "deploys.noticePolicyLink": {
    message: "política de deploys sobrepostos.",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): the link text inside noticeCancelPolicy and noticeQueuePolicy, the workspace's Overlapping Deploy Policy",
  },
  "deploys.noticeHealthWaiting": {
    message:
      "Aguardando a {link} interna retornar um código de resposta de sucesso",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a deploy rolling out on a service with a Health Check Path; {link} is the noticeHealthLink link to the health check docs",
  },
  "deploys.noticeHealthWaitingAt": {
    message:
      "Aguardando a {link} interna retornar um código de resposta de sucesso em: {address}",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): noticeHealthWaiting with the address checked; {address} is host:port/path in code type",
  },
  "deploys.noticeHealthTimedOut": {
    message:
      "Tempo esgotado aguardando a {link} interna retornar um código de resposta de sucesso",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a failed deploy that ran out of its rollout window, on a service with a Health Check Path; {link} as in noticeHealthWaiting",
  },
  "deploys.noticeHealthTimedOutAt": {
    message:
      "Tempo esgotado aguardando a {link} interna retornar um código de resposta de sucesso em: {address}",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): noticeHealthTimedOut with the address checked; {address} is host:port/path in code type",
  },
  "deploys.noticeHealthLink": {
    message: "verificação de integridade",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): the link text inside the health check notices",
  },
  "deploys.reservedPortWarning": {
    message:
      "Parece que seu serviço pode estar escutando na porta {port}, que é uma porta reservada para {link} Configure seu serviço para escutar em uma porta que não esteja entre nossas portas reservadas: {ports}.",
    description:
      "Deploy page banner (w1/m659, Render's free-plan warning): the instance serves HTTP on a reserved port; {port} is that port, {link} the reservedPortWarningLink link to bex's pricing, {ports} the reserved ports",
  },
  "deploys.reservedPortWarningLink": {
    message: "planos de computação gratuitos.",
    description: "The link text inside reservedPortWarning (w1/m659)",
  },
  "deploys.rollbackTitle": {
    message: "Confirmar rollback",
    description:
      "Rollback page (w1/m646): the block's title, Render's \"Confirm Rollback\"",
  },
  "deploys.rollbackLead": {
    message: "O rollback reverte determinadas alterações de configuração.",
    description:
      "Rollback page: the block's lead sentence, before the Learn more link",
  },
  "deploys.rollbackLearnMore": {
    message: "Saiba mais.",
    description:
      "Rollback page: link to the rollbacks documentation after the lead sentence",
  },
  "deploys.rollbackLoading": {
    message: "Carregando as alterações de configuração",
    description:
      "Rollback page: accessible name of the spinner while the comparison loads",
  },
  "deploys.rollbackUnavailable": {
    message:
      "Não foi possível carregar a configuração do deploy que está no ar. Fazer rollback para este deploy pode causar alterações de configuração inesperadas.",
    description:
      "Rollback page: Render's sentence when the comparison failed or either deploy's configuration is not recorded",
  },
  "deploys.rollbackNoChanges": {
    message: "Nenhuma alteração de configuração desde este deploy.",
    description:
      "Rollback page: nothing differs between the live deploy and this one",
  },
  "deploys.rollbackRowStartCommand": {
    message: "Comando de início",
    description: "Rollback page: row label for a changed start command",
  },
  "deploys.rollbackRowPreDeployCommand": {
    message: "Comando de pré-deploy",
    description: "Rollback page: row label for a changed pre-deploy command",
  },
  "deploys.rollbackRowDockerCommand": {
    message: "Comando do Docker",
    description: "Rollback page: row label for a changed Docker command",
  },
  "deploys.rollbackRowHealthCheckPath": {
    message: "Caminho do health check",
    description: "Rollback page: row label for a changed health check path",
  },
  "deploys.rollbackRowInstances": {
    message: "Instâncias",
    description: "Rollback page: row label for a changed instance count",
  },
  "deploys.rollbackRowEnvVars": {
    message: "Variáveis de ambiente",
    description:
      "Rollback page: row label for the changed environment variables",
  },
  "deploys.rollbackRowEnvGroup": {
    message: "Grupo de ambiente",
    description: "Rollback page: row label for environment group changes",
  },
  "deploys.rollbackRowSecretFiles": {
    message: "Arquivos secretos",
    description: "Rollback page: row label for changed secret files",
  },
  "deploys.rollbackRowPlan": {
    message: "Plano",
    description: "Rollback page: row label for the plan warning",
  },
  "deploys.rollbackNone": {
    message: "nenhum",
    description:
      "Rollback page: a command or path that is not set, on either side of a change",
  },
  "deploys.rollbackChange": {
    message: "Alteração de configuração",
    description: "Rollback page: accessible name of one before/after change",
  },
  "deploys.rollbackRemoved": {
    message: "Removido",
    description:
      "Rollback page: accessible name of a change's minus line (the live value)",
  },
  "deploys.rollbackAdded": {
    message: "Adicionado",
    description:
      "Rollback page: accessible name of a change's plus line (the value after the rollback)",
  },
  "deploys.rollbackEnvKey": {
    message: "Chave",
    description:
      "Rollback page: environment variable table's key column header",
  },
  "deploys.rollbackEnvValue": {
    message: "Valor",
    description:
      "Rollback page: environment variable table's value column header; every value is masked",
  },
  "deploys.rollbackSecretFileAdded": {
    message:
      "O arquivo secreto {name} foi adicionado após este deploy. O rollback o ignorará.",
    description:
      "Rollback page: a secret file that did not exist at this deploy; {name} is the file name",
  },
  "deploys.rollbackSecretFileDeleted": {
    message:
      "O arquivo secreto {name} foi excluído após este deploy. O rollback restaurará o arquivo.",
    description:
      "Rollback page: a secret file deleted since this deploy; {name} is the file name",
  },
  "deploys.rollbackSecretFileChanged": {
    message:
      "O arquivo secreto {name} foi alterado após este deploy. O rollback restaurará o conteúdo original.",
    description:
      "Rollback page: a secret file whose contents changed since this deploy; {name} is the file name",
  },
  "deploys.rollbackGroupUnlink": {
    message:
      "O rollback desvinculará o grupo de ambiente {name}, que foi vinculado ao serviço após este deploy.",
    description:
      "Rollback page: an environment group linked since this deploy; {name} is the group name",
  },
  "deploys.rollbackGroupLink": {
    message:
      "O rollback vinculará o grupo de ambiente {name}, que foi desvinculado do serviço após este deploy.",
    description:
      "Rollback page: an environment group unlinked since this deploy; {name} is the group name",
  },
  "deploys.rollbackGroupDeleted_one": {
    message:
      "Este deploy estava vinculado a {n} grupo de ambiente que foi excluído desde então. Ele não pode ser restaurado.",
    description:
      "Rollback page: one group this deploy linked was deleted; {n} is the count",
  },
  "deploys.rollbackGroupDeleted_many": {
    message:
      "Este deploy estava vinculado a {n} de grupos de ambiente que foram excluídos desde então. Eles não podem ser restaurados.",
    description:
      "Rollback page: several groups this deploy linked were deleted; {n} is the count",
  },
  "deploys.rollbackGroupDeleted_other": {
    message:
      "Este deploy estava vinculado a {n} grupos de ambiente que foram excluídos desde então. Eles não podem ser restaurados.",
    description:
      "Rollback page: several groups this deploy linked were deleted; {n} is the count",
  },
  "deploys.rollbackGroupCurrentValues": {
    message:
      "O rollback usará as chaves e os valores atuais dos grupos de ambiente vinculados a este deploy.",
    description:
      "Rollback page: shown whenever this deploy linked any environment group",
  },
  "deploys.rollbackPlanWarning": {
    message:
      "Seu plano atual tem alocações de CPU e memória menores que as do plano usado originalmente neste deploy. Talvez seja necessário mudar para outro plano para que o rollback seja bem-sucedido.",
    description:
      "Rollback page: the saved plan is smaller than the one this deploy ran on",
  },
  "deploys.rollbackStaticNotice": {
    message: "As alterações em {what} não serão revertidas.",
    description:
      "Rollback page, static site: {what} is deploys.rollbackStaticNoticeWhat, in bold",
  },
  "deploys.rollbackStaticNoticeWhat": {
    message: "redirecionamentos, reescritas e cabeçalhos",
    description: "Rollback page, static site: what a rollback keeps current",
  },
  "deploys.rollbackAutoDeployTitle": {
    message: "Deploy automático",
    description:
      "Rollback page: title of the warning that the rollback turns auto-deploy off",
  },
  "deploys.rollbackAutoDeployBody": {
    message: "Será desativado neste serviço para evitar deploys acidentais.",
    description:
      "Rollback page: the auto-deploy warning's text, after its title",
  },
  "deploys.rollbackConfirm": {
    message: "Fazer rollback para este deploy",
    description: "Rollback page: the button that starts the rollback",
  },
  "deploys.rollbackCancel": {
    message: "Cancelar",
    description:
      "Rollback page: the button that goes back to the deploy without rolling back",
  },
  "deploys.rollbackErrorTitle": {
    message: "Erro:",
    description: "Rollback page: label before the reason a rollback failed",
  },
};

export default ptDeploys;
