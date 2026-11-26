import type { TranslationEntry } from "@/i18n";

const pt: Record<string, TranslationEntry> = {
  "sandboxGroupPages.navSandboxes": {
    message: "Sandboxes",
    description: "Sidebar entry and heading: a sandbox group's sandboxes",
  },
  "sandboxGroupPages.navSnapshots": {
    message: "Snapshots",
    description: "Sidebar entry and heading: a sandbox group's snapshots",
  },
  "sandboxGroupPages.navImages": {
    message: "Imagens personalizadas",
    description:
      "Sidebar entry and heading: the images imported into a sandbox group",
  },
  "sandboxGroupPages.navSettings": {
    message: "Configurações",
    description: "Sidebar entry and heading: a sandbox group's settings",
  },
  "sandboxGroupPages.statusCreating": {
    message: "Criando",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusRunning": {
    message: "Em execução",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusResuming": {
    message: "Retomando",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusSuspending": {
    message: "Suspendendo",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusSuspended": {
    message: "Suspenso",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusErrored": {
    message: "Com erro",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusTerminated": {
    message: "Encerrado",
    description: "Sandbox status",
  },
  "sandboxGroupPages.rangeLabel": {
    message: "Intervalo de tempo",
    description: "Accessible name of the time range picker",
  },
  "sandboxGroupPages.rangeHour": {
    message: "Última hora",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeDay": {
    message: "Últimas 24 horas",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeWeek": {
    message: "Últimos 7 dias",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeMonth": {
    message: "Últimos 30 dias",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeAll": {
    message: "Todo o período",
    description: "Time range option",
  },
  "sandboxGroupPages.searchSandboxes": {
    message: "Pesquisar por nome, ID do sandbox ou status",
    description: "Placeholder of the Sandboxes search",
  },
  "sandboxGroupPages.searchSnapshots": {
    message: "Pesquisar snapshots",
    description: "Placeholder of the Snapshots search",
  },
  "sandboxGroupPages.searchImages": {
    message: "Pesquisar imagens",
    description: "Placeholder of the Custom images search",
  },
  "sandboxGroupPages.pauseLive": {
    message: "Pausar atualizações ao vivo",
    description: "Button that stops the list refreshing",
  },
  "sandboxGroupPages.resumeLive": {
    message: "Retomar atualizações ao vivo",
    description: "Button that restarts the list refreshing",
  },
  "sandboxGroupPages.showHideColumns": {
    message: "Mostrar ou ocultar colunas",
    description: "Accessible name of the columns menu",
  },
  "sandboxGroupPages.summaryTotal": {
    message: "Total",
    description: "Summary: runs that ended in the time range",
  },
  "sandboxGroupPages.summaryErrored": {
    message: "Com erro",
    description: "Summary: share of runs that ended in an error",
  },
  "sandboxGroupPages.summaryErroredTip": {
    message:
      "Proporção de sandboxes no intervalo de tempo que terminaram com erro.",
    description: "Tooltip of the Errored summary",
  },
  "sandboxGroupPages.summaryRunning": {
    message: "Em execução",
    description: "Summary: sandboxes active now",
  },
  "sandboxGroupPages.summaryRunningTip": {
    message:
      "Sendo criados, em execução, sendo retomados ou sendo suspensos agora, qualquer que seja o intervalo de tempo. Eles contam para o limite de concorrência.",
    description: "Tooltip of the Running summary",
  },
  "sandboxGroupPages.loadRunsError": {
    message: "Não foi possível carregar as execuções de sandboxes",
    description: "The run chart failed to load",
  },
  "sandboxGroupPages.runsChartLabel": {
    message:
      "Execuções de sandboxes encerradas: {succeeded} bem-sucedidas, {errored} com erro",
    description: "Accessible name of the run chart",
  },
  "sandboxGroupPages.runsSucceeded": {
    message: "Bem-sucedidas",
    description: "Chart legend: runs that ended without an error",
  },
  "sandboxGroupPages.runsErrored": {
    message: "Com erro",
    description: "Chart legend: runs that ended in an error",
  },
  "sandboxGroupPages.selectedCount_one": {
    message: "{count} sandbox selecionado",
    description: "How many rows are selected",
  },
  "sandboxGroupPages.selectedCount_many": {
    message: "{count} de sandboxes selecionados",
    description: "How many rows are selected",
  },
  "sandboxGroupPages.selectedCount_other": {
    message: "{count} sandboxes selecionados",
    description: "How many rows are selected",
  },
  "sandboxGroupPages.terminateSelected": {
    message: "Encerrar",
    description: "Button: terminate the selected sandboxes",
  },
  "sandboxGroupPages.terminateTitle_one": {
    message: "Encerrar {count} sandbox?",
    description: "Terminate confirmation title",
  },
  "sandboxGroupPages.terminateTitle_many": {
    message: "Encerrar {count} de sandboxes?",
    description: "Terminate confirmation title",
  },
  "sandboxGroupPages.terminateTitle_other": {
    message: "Encerrar {count} sandboxes?",
    description: "Terminate confirmation title",
  },
  "sandboxGroupPages.terminateDescription_one": {
    message:
      "{count} sandbox para, e tudo o que estiver nele e não estiver em um snapshot será perdido.",
    description: "Terminate confirmation body",
  },
  "sandboxGroupPages.terminateDescription_many": {
    message:
      "Os {count} de sandboxes param, e tudo o que estiver neles e não estiver em um snapshot será perdido.",
    description: "Terminate confirmation body",
  },
  "sandboxGroupPages.terminateDescription_other": {
    message:
      "Os {count} sandboxes param, e tudo o que estiver neles e não estiver em um snapshot será perdido.",
    description: "Terminate confirmation body",
  },
  "sandboxGroupPages.terminateConfirm": {
    message: "Encerrar",
    description: "Terminate confirmation button",
  },
  "sandboxGroupPages.terminateFailed_one": {
    message: "Não foi possível encerrar {count} sandbox. Tente novamente.",
    description: "Toast: a terminate failed",
  },
  "sandboxGroupPages.terminateFailed_many": {
    message: "Não foi possível encerrar {count} de sandboxes. Tente novamente.",
    description: "Toast: terminates failed",
  },
  "sandboxGroupPages.terminateFailed_other": {
    message: "Não foi possível encerrar {count} sandboxes. Tente novamente.",
    description: "Toast: terminates failed",
  },
  "sandboxGroupPages.terminateStarted_one": {
    message: "Encerrando {count} sandbox",
    description: "Toast: a terminate was accepted",
  },
  "sandboxGroupPages.terminateStarted_many": {
    message: "Encerrando {count} de sandboxes",
    description: "Toast: terminates were accepted",
  },
  "sandboxGroupPages.terminateStarted_other": {
    message: "Encerrando {count} sandboxes",
    description: "Toast: terminates were accepted",
  },
  "sandboxGroupPages.terminateSandbox": {
    message: "Encerrar sandbox",
    description: "Drawer menu item",
  },
  "sandboxGroupPages.sandboxActions": {
    message: "Ações do sandbox",
    description: "Accessible name of the drawer's menu",
  },
  "sandboxGroupPages.loadSandboxError": {
    message: "Não foi possível carregar o sandbox",
    description: "A sandbox's drawer failed to load",
  },
  "sandboxGroupPages.loadSandboxesError": {
    message: "Não foi possível carregar os sandboxes",
    description: "The list failed to load",
  },
  "sandboxGroupPages.noMatchingSandboxes": {
    message: "Nenhum sandbox correspondente",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.noMatchingSandboxesHelp": {
    message: "Tente outra pesquisa ou amplie o intervalo de tempo.",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.clearFilters": {
    message: "Limpar filtros",
    description: "Button: reset the search and time range",
  },
  "sandboxGroupPages.firstSandboxTitle": {
    message: "Crie seu primeiro sandbox",
    description: "Empty state title of the Sandboxes page",
  },
  "sandboxGroupPages.firstSandboxDescription": {
    message:
      "Inicie um sandbox pela CLI ou configure seu código para criar sandboxes de forma programática com os SDKs de TypeScript e Python.",
    description: "Empty state body of the Sandboxes page",
  },
  "sandboxGroupPages.starterCli": {
    message: "CLI",
    description: "Starter tab: the command line",
  },
  "sandboxGroupPages.starterSdk": {
    message: "SDK",
    description: "Starter tab: the SDKs",
  },
  "sandboxGroupPages.starterInstallCli": {
    message: "Instale ou atualize para a CLI mais recente",
    description: "Starter step",
  },
  "sandboxGroupPages.starterHomebrew": {
    message: "Homebrew",
    description: "Install method",
  },
  "sandboxGroupPages.starterDirect": {
    message: "Download direto",
    description: "Install method",
  },
  "sandboxGroupPages.starterLogin": {
    message: "Faça login e use este workspace",
    description: "Starter step",
  },
  "sandboxGroupPages.starterCreateSandbox": {
    message: "Crie um sandbox com as configurações padrão",
    description: "Starter step",
  },
  "sandboxGroupPages.starterApiKey": {
    message: "Defina sua chave de API",
    description: "Starter step",
  },
  "sandboxGroupPages.starterApiKeyHelp": {
    message:
      "Execute isto no terminal em que você vai executar seu código. O SDK lê sua chave da variável de ambiente RENDER_API_KEY. Substitua o valor de exemplo por uma chave de API do bex.",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterInstallSdk": {
    message: "Configure um projeto e instale o SDK",
    description: "Starter step",
  },
  "sandboxGroupPages.starterPythonNeeds": {
    message:
      "Requer Python 3.10 ou posterior. -U atualiza um SDK mais antigo; a versão 1.4.0 e anteriores não têm suporte a sandboxes.",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterNodeNeeds": {
    message:
      "Requer Node.js 18 ou posterior. @latest também atualiza um SDK mais antigo que já esteja instalado.",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterFirstSandboxFile": {
    message: "Inicialize seu primeiro sandbox",
    description: "Starter step",
  },
  "sandboxGroupPages.starterRun": {
    message: "Execute seu código localmente",
    description: "Starter step",
  },
  "sandboxGroupPages.starterTypeScript": {
    message: "TypeScript",
    description: "SDK language",
  },
  "sandboxGroupPages.starterPython": {
    message: "Python",
    description: "SDK language",
  },
  "sandboxGroupPages.colSandbox": { message: "Sandbox", description: "Column" },
  "sandboxGroupPages.colStatus": { message: "Status", description: "Column" },
  "sandboxGroupPages.colCreated": { message: "Criado", description: "Column" },
  "sandboxGroupPages.colDuration": {
    message: "Duração",
    description: "Column",
  },
  "sandboxGroupPages.colPlan": { message: "Plano", description: "Column" },
  "sandboxGroupPages.colSource": {
    message: "Iniciado a partir de",
    description: "Column: the image or snapshot a sandbox started from",
  },
  "sandboxGroupPages.selectAll": {
    message: "Selecionar todos os sandboxes desta página",
    description: "Accessible name of the select-all checkbox",
  },
  "sandboxGroupPages.selectSandbox": {
    message: "Selecionar {name}",
    description: "Accessible name of a row's checkbox",
  },
  "sandboxGroupPages.baseImage": {
    message: "Imagem base",
    description: "A sandbox started from the default image",
  },
  "sandboxGroupPages.importedImage": {
    message: "Imagem importada",
    description: "A sandbox started from an imported image",
  },
  "sandboxGroupPages.previousPage": {
    message: "Anterior",
    description: "Paging button",
  },
  "sandboxGroupPages.nextPage": {
    message: "Próxima",
    description: "Paging button",
  },
  "sandboxGroupPages.switchedToRunning": {
    message:
      "O sandbox selecionado parou. Mudamos para um que está em execução.",
    description: "Toast when the open sandbox stops",
  },
  "sandboxGroupPages.copied": { message: "Copiado", description: "Copy toast" },
  "sandboxGroupPages.copyError": {
    message: "Não foi possível copiar",
    description: "Copy failed toast",
  },
  "sandboxGroupPages.copySandboxId": {
    message: "Copiar ID do sandbox",
    description: "Copy button",
  },
  "sandboxGroupPages.copySnapshotId": {
    message: "Copiar ID do snapshot",
    description: "Copy button",
  },
  "sandboxGroupPages.tabEvents": {
    message: "Eventos",
    description: "Drawer tab",
  },
  "sandboxGroupPages.tabSnapshots": {
    message: "Snapshots",
    description: "Drawer tab",
  },
  "sandboxGroupPages.tabDetails": {
    message: "Detalhes",
    description: "Drawer tab and section",
  },
  "sandboxGroupPages.tabVersions": {
    message: "Versões",
    description: "Image drawer tab",
  },
  "sandboxGroupPages.sandboxGoneTitle": {
    message: "Sandbox não encontrado",
    description: "A sandbox that does not exist here",
  },
  "sandboxGroupPages.sandboxGoneDescription": {
    message: "{id} não é um sandbox que você pode ver neste grupo.",
    description: "A sandbox that does not exist here",
  },
  "sandboxGroupPages.loadingEvents": {
    message: "Carregando eventos",
    description: "Events tab loading",
  },
  "sandboxGroupPages.loadEventsError": {
    message:
      "Não foi possível carregar os eventos. Tente novamente em instantes.",
    description: "Events tab error",
  },
  "sandboxGroupPages.noEvents": {
    message: "Nenhum evento ainda.",
    description: "Events tab empty",
  },
  "sandboxGroupPages.loadEarlierEvents": {
    message: "Carregar eventos anteriores",
    description: "Button: the next page of events",
  },
  "sandboxGroupPages.loadingEarlierEvents": {
    message: "Carregando eventos anteriores",
    description: "Loading the next page of events",
  },
  "sandboxGroupPages.eventCreated": {
    message: "Criado",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventReady": {
    message: "Pronto",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventSuspending": {
    message: "Suspendendo",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventSuspended": {
    message: "Suspenso",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventResuming": {
    message: "Retomando",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventTerminated": {
    message: "Encerrado",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventErrored": {
    message: "Com erro",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventCommand": {
    message: "Comando ({operation})",
    description: "An execution event that ran a command",
  },
  "sandboxGroupPages.eventFileTransfer": {
    message: "Transferência de arquivo ({operation})",
    description: "An execution event that moved a file",
  },
  "sandboxGroupPages.exitCode": {
    message: "Código de saída {code}",
    description: "A command's exit code",
  },
  "sandboxGroupPages.eventSnapshot": {
    message: "Snapshot",
    description: "A snapshot event",
  },
  "sandboxGroupPages.viewSnapshot": {
    message: "Ver snapshot",
    description: "Link to a snapshot's drawer",
  },
  "sandboxGroupPages.triggerSdk": {
    message: "SDK",
    description: "Trigger: the SDK",
  },
  "sandboxGroupPages.triggerCli": {
    message: "CLI",
    description: "Trigger: the CLI",
  },
  "sandboxGroupPages.triggerApi": {
    message: "API",
    description: "Trigger: the API",
  },
  "sandboxGroupPages.triggerDashboard": {
    message: "Painel",
    description: "Trigger: the dashboard",
  },
  "sandboxGroupPages.triggerIdleTimeout": {
    message: "Tempo limite de inatividade",
    description: "Trigger: the idle timeout",
  },
  "sandboxGroupPages.triggerSystem": {
    message: "bex",
    description: "Trigger: the platform itself",
  },
  "sandboxGroupPages.triggerConnection": {
    message: "Novo comando ou transferência de arquivo",
    description: "Trigger: a resume a command started",
  },
  "sandboxGroupPages.loading": {
    message: "Carregando",
    description: "Loading text",
  },
  "sandboxGroupPages.loadSnapshotsError": {
    message: "Não foi possível carregar os snapshots",
    description: "Snapshots failed to load",
  },
  "sandboxGroupPages.noSnapshotsRunning": {
    message:
      "Este sandbox ainda não tem snapshots. Capture um com a CLI ou um SDK.",
    description: "Snapshots tab of a running sandbox",
  },
  "sandboxGroupPages.noSnapshotsEnded": {
    message: "Nenhum snapshot foi feito deste sandbox.",
    description: "Snapshots tab of an ended sandbox",
  },
  "sandboxGroupPages.detailSandboxId": {
    message: "ID do sandbox",
    description: "Detail label",
  },
  "sandboxGroupPages.detailGroup": {
    message: "Grupo",
    description: "Detail label",
  },
  "sandboxGroupPages.detailPlan": {
    message: "Plano",
    description: "Detail label",
  },
  "sandboxGroupPages.planSize": {
    message: "{plan} ({cpus} CPU, {memory} GB de memória)",
    description: "A plan with its size",
  },
  "sandboxGroupPages.detailCreated": {
    message: "Criado",
    description: "Detail label",
  },
  "sandboxGroupPages.detailEnded": {
    message: "Encerrado",
    description: "Detail label",
  },
  "sandboxGroupPages.detailDuration": {
    message: "Duração",
    description: "Detail label",
  },
  "sandboxGroupPages.detailTimeout": {
    message: "Tempo limite",
    description: "Detail label",
  },
  "sandboxGroupPages.detailIdleTimeout": {
    message: "Tempo limite de inatividade",
    description: "Detail label",
  },
  "sandboxGroupPages.off": {
    message: "Desativado",
    description: "A timeout that is off",
  },
  "sandboxGroupPages.detailNetwork": {
    message: "Rede",
    description: "Detail label",
  },
  "sandboxGroupPages.networkAllowAll": {
    message: "Permitir tudo",
    description: "Network policy",
  },
  "sandboxGroupPages.networkDenyAll": {
    message: "Negar tudo",
    description: "Network policy",
  },
  "sandboxGroupPages.networkAllowList": {
    message: "Lista de permissões",
    description: "Network policy",
  },
  "sandboxGroupPages.detailRegion": {
    message: "Região",
    description: "Detail label",
  },
  "sandboxGroupPages.detailSource": {
    message: "Iniciado a partir de",
    description: "Detail label",
  },
  "sandboxGroupPages.goToSnapshot": {
    message: "Ir para o snapshot",
    description: "Link to the snapshot a sandbox started from",
  },
  "sandboxGroupPages.goToImage": {
    message: "Ir para a imagem",
    description: "Link to the image a sandbox started from",
  },
  "sandboxGroupPages.detailTrigger": {
    message: "Gatilho",
    description: "Detail label: what started the sandbox",
  },
  "sandboxGroupPages.detailUser": {
    message: "Usuário",
    description: "Detail label: who started the sandbox",
  },
  "sandboxGroupPages.detailError": {
    message: "Erro",
    description: "Detail label",
  },
  "sandboxGroupPages.errorUnknown": {
    message:
      "O bex não conseguiu identificar por que este sandbox apresentou erro. Tente novamente ou entre em contato com o suporte informando o ID do sandbox.",
    description: "An errored sandbox with no known reason",
  },
  "sandboxGroupPages.snapshotCapturing": {
    message: "Capturando",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotAvailable": {
    message: "Disponível",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotFailed": {
    message: "Falhou",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotFailureInternal": {
    message: "Um erro interno interrompeu a captura.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureTimeout": {
    message: "Esta captura atingiu o tempo limite.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureSourceTerminated": {
    message: "O sandbox de origem foi encerrado antes de a captura terminar.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureSourceErrored": {
    message: "O sandbox de origem apresentou erro antes de a captura terminar.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailedHelp": {
    message:
      "Crie um novo snapshot para tentar novamente. Novos sandboxes não podem ser iniciados a partir dele.",
    description: "After a snapshot failure",
  },
  "sandboxGroupPages.noMatchingSnapshots": {
    message: "Nenhum snapshot correspondente",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.firstSnapshotTitle": {
    message: "Crie seu primeiro snapshot",
    description: "Empty state title of the Snapshots page",
  },
  "sandboxGroupPages.firstSnapshotDescription": {
    message:
      "Capture um snapshot para salvar os arquivos de um sandbox. Inicie novos sandboxes a partir do mesmo ponto sem configurar tudo de novo.",
    description: "Empty state body of the Snapshots page",
  },
  "sandboxGroupPages.firstSnapshotNeedsRunning": {
    message: "Você precisa de um sandbox em execução para criar um snapshot.",
    description: "Empty state note",
  },
  "sandboxGroupPages.starterSnapshotSandbox": {
    message: "Faça um snapshot de um sandbox em execução",
    description: "Starter step",
  },
  "sandboxGroupPages.starterStartFromSnapshot": {
    message: "Crie sandboxes a partir do snapshot",
    description: "Starter step",
  },
  "sandboxGroupPages.starterSnapshotCode": {
    message: "Configure seu código",
    description: "Starter step",
  },
  "sandboxGroupPages.colSnapshot": {
    message: "Snapshot",
    description: "Column",
  },
  "sandboxGroupPages.colSourceSandbox": {
    message: "Sandbox de origem",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colSize": {
    message: "Tamanho",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colCaptured": {
    message: "Capturado",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colExpires": {
    message: "Expira",
    description: "Column and detail label",
  },
  "sandboxGroupPages.never": {
    message: "Nunca",
    description: "A snapshot that never expires",
  },
  "sandboxGroupPages.snapshotGoneTitle": {
    message: "Snapshot não encontrado",
    description: "A snapshot that does not exist here",
  },
  "sandboxGroupPages.snapshotGoneDescription": {
    message:
      "{id} não é um snapshot que você pode ver neste workspace. Ele pode ter sido excluído ou ter expirado.",
    description: "A snapshot that does not exist here",
  },
  "sandboxGroupPages.detailType": {
    message: "Tipo",
    description: "Detail label: snapshot kind",
  },
  "sandboxGroupPages.kindFilesystem": {
    message: "Sistema de arquivos",
    description: "Snapshot kind",
  },
  "sandboxGroupPages.kindRuntime": {
    message: "Runtime",
    description: "Snapshot kind",
  },
  "sandboxGroupPages.detailResourceConstraint": {
    message: "Restrição de recursos",
    description: "Detail label: the plan a snapshot needs",
  },
  "sandboxGroupPages.detailLifecycle": {
    message: "Ciclo de vida",
    description: "Detail label",
  },
  "sandboxGroupPages.lifecycleKept": {
    message: "Mantido até ser excluído",
    description: "A snapshot with no time to live",
  },
  "sandboxGroupPages.lifecycleAfterUse": {
    message: "Expira {days} dias após o último uso",
    description: "A snapshot's time to live",
  },
  "sandboxGroupPages.noSandboxesFromSnapshot": {
    message: "Nenhum sandbox foi iniciado a partir deste snapshot.",
    description: "Snapshot drawer's sandboxes",
  },
  "sandboxGroupPages.restoreTitle": {
    message: "Inicie um sandbox a partir deste snapshot",
    description: "Snapshot drawer starter code",
  },
  "sandboxGroupPages.deleteSnapshot": {
    message: "Excluir snapshot",
    description: "Button",
  },
  "sandboxGroupPages.deleteSnapshotTitle": {
    message: "Excluir snapshot?",
    description: "Confirmation title",
  },
  "sandboxGroupPages.deleteSnapshotDescription": {
    message:
      "Isso exclui {name} permanentemente. Novos sandboxes não poderão ser iniciados a partir dele. Os sandboxes já iniciados a partir dele continuam em execução.",
    description: "Confirmation body",
  },
  "sandboxGroupPages.deleteSnapshotError": {
    message: "Não foi possível excluir o snapshot. Tente novamente.",
    description: "Toast",
  },
  "sandboxGroupPages.snapshotDeleted": {
    message: "Snapshot {name} excluído",
    description: "Toast",
  },
  "sandboxGroupPages.imageQueued": {
    message: "Na fila",
    description: "Image status",
  },
  "sandboxGroupPages.imageImporting": {
    message: "Importando",
    description: "Image status",
  },
  "sandboxGroupPages.imageAvailable": {
    message: "Disponível",
    description: "Image status",
  },
  "sandboxGroupPages.imageUnavailable": {
    message: "Indisponível",
    description: "Image status: no version can start a sandbox",
  },
  "sandboxGroupPages.imageFailed": {
    message: "Falhou",
    description: "Image version status",
  },
  "sandboxGroupPages.imageLatest": {
    message: "Mais recente",
    description: "The newest available version of an image",
  },
  "sandboxGroupPages.imageImportingHelp": {
    message:
      "Os sandboxes são iniciados a partir da versão disponível mais recente até que esta termine.",
    description: "A version still importing",
  },
  "sandboxGroupPages.imageFailureTimeout": {
    message: "A importação demorou demais.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureRetries": {
    message: "A importação falhou após várias tentativas.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureCredentials": {
    message:
      "O bex não conseguiu baixar a imagem. Verifique as credenciais do registro.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureNotFound": {
    message:
      "O bex não encontrou a imagem. Verifique o nome e a tag da imagem.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailurePull": {
    message: "O bex não conseguiu baixar a imagem.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailurePlatform": {
    message:
      "A imagem é para {platform}. Os sandboxes só são compatíveis com linux/amd64.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureMissing": {
    message:
      "Falta {what} na imagem, e os sandboxes precisam disso para iniciar. Use uma imagem com shell, como uma variante slim ou Alpine.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureConvert": {
    message:
      "O bex não conseguiu converter a imagem. Tente importá-la novamente.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureLayers": {
    message: "O bex não conseguiu ler as camadas da imagem.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureStart": {
    message: "Não foi possível iniciar a importação.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureStopped": {
    message: "A importação parou antes de terminar.",
    description: "Image failure",
  },
  "sandboxGroupPages.loadImagesError": {
    message: "Não foi possível carregar as imagens",
    description: "Images failed to load",
  },
  "sandboxGroupPages.noMatchingImages": {
    message: "Nenhuma imagem correspondente",
    description: "Empty state with a search",
  },
  "sandboxGroupPages.firstImageTitle": {
    message: "Importe sua primeira imagem",
    description: "Empty state title of the Custom images page",
  },
  "sandboxGroupPages.firstImageDescription": {
    message:
      "Importe uma imagem de um registro para que os sandboxes possam ser iniciados a partir dela. Importe uma com a CLI ou o SDK e depois inicie sandboxes a partir dela.",
    description: "Empty state body of the Custom images page",
  },
  "sandboxGroupPages.starterImportImage": {
    message: "Importe uma imagem (linux/amd64)",
    description: "Starter step",
  },
  "sandboxGroupPages.starterStartFromImage": {
    message: "Crie sandboxes a partir da imagem",
    description: "Starter step",
  },
  "sandboxGroupPages.starterImportCode": {
    message: "Importe imagens",
    description: "Starter step",
  },
  "sandboxGroupPages.colImage": { message: "Imagem", description: "Column" },
  "sandboxGroupPages.colSourceImage": {
    message: "Origem",
    description: "Column: the registry image",
  },
  "sandboxGroupPages.colLatestAvailable": {
    message: "Mais recente disponível",
    description: "Column: the newest available version's digest",
  },
  "sandboxGroupPages.colUpdated": {
    message: "Atualizada",
    description: "Column: the newest import",
  },
  "sandboxGroupPages.imageGoneTitle": {
    message: "Imagem não encontrada",
    description: "An image name with no versions",
  },
  "sandboxGroupPages.imageGoneDescription": {
    message: "Este grupo não tem nenhuma imagem chamada {name}.",
    description: "An image name with no versions",
  },
  "sandboxGroupPages.detailLatestAvailable": {
    message: "Versão disponível mais recente",
    description: "Detail label",
  },
  "sandboxGroupPages.detailImageSource": {
    message: "Origem",
    description: "Detail label: the registry",
  },
  "sandboxGroupPages.detailImage": {
    message: "Imagem",
    description: "Detail label",
  },
  "sandboxGroupPages.detailDigest": {
    message: "Digest",
    description: "Detail label",
  },
  "sandboxGroupPages.detailVersionId": {
    message: "ID da versão",
    description: "Detail label",
  },
  "sandboxGroupPages.noSandboxesFromImage": {
    message: "Nenhum sandbox foi iniciado a partir desta imagem.",
    description: "Image drawer's sandboxes",
  },
  "sandboxGroupPages.general": {
    message: "Geral",
    description: "Settings section",
  },
  "sandboxGroupPages.makeDefault": {
    message: "Tornar padrão",
    description: "Button: make this the workspace's default group",
  },
  "sandboxGroupPages.madeDefault": {
    message: "{name} agora é o grupo de sandbox padrão",
    description: "Toast",
  },
  "sandboxGroupPages.save": { message: "Salvar", description: "Button" },
  "sandboxGroupPages.limitSaved": {
    message: "Limite de concorrência salvo",
    description: "Toast",
  },
  "sandboxGroupPages.brokeredCredentials": {
    message: "Credenciais intermediadas",
    description: "What a group delete removes",
  },
  "sandboxGroupPages.deleteGroup": {
    message: "Excluir grupo de sandbox",
    description: "Button and section",
  },
  "sandboxGroupPages.deleteGroupHelp": {
    message:
      "Exclua este grupo, com seus sandboxes, snapshots, imagens e credenciais.",
    description: "Delete section help",
  },
  "sandboxGroupPages.deleteGroupDefault": {
    message:
      "O grupo padrão do workspace não pode ser excluído. Primeiro, torne outro grupo o padrão.",
    description: "Delete section help for the default group",
  },
  "sandboxGroupPages.deleteGroupTitle": {
    message: "Excluir grupo de sandbox?",
    description: "Confirmation title",
  },
  "sandboxGroupPages.deleteGroupDescription": {
    message:
      "Este grupo de sandbox, {name}, será excluído permanentemente, junto com o histórico de execuções, os logs e as configurações:",
    description: "Confirmation body",
  },
  "sandboxGroupPages.deleteGroupStops": {
    message:
      "Tudo o que usa este grupo via SDK, CLI ou API vai parar de funcionar.",
    description: "Confirmation body",
  },
  "sandboxGroupPages.groupDeleted": {
    message: "Grupo de sandbox {name} excluído",
    description: "Toast",
  },
  "sandboxGroupPages.linkLoadError": {
    message: "Não foi possível abrir o link. Tente novamente em instantes.",
    description: "A link by id failed to resolve",
  },
  "sandboxGroupPages.backToGroups": {
    message: "Ir para os grupos de sandbox",
    description: "Button",
  },
};

export default pt;
