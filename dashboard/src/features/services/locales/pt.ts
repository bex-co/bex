import type { TranslationEntry } from "@/i18n";

const ptServices: Record<string, TranslationEntry> = {
  "services.protectedConfirmationTitle": {
    message: "Confirmação de ambiente protegido",
    description: "Title of a protected-service destructive-action retry dialog",
  },
  "services.protectedConfirmationBody": {
    message:
      "“{name}” pertence a um ambiente protegido. Digite o comando sudo exato retornado pelo bex-api para continuar.",
    description: "Body of a protected-service destructive-action retry dialog",
  },
  "services.protectedConfirmationPrompt": {
    message: "Digite {phrase} abaixo para confirmar.",
    description:
      "Body prompt naming the exact protected-action confirmation phrase (rendered bold by SudoCommandField)",
  },
  "services.connect": {
    message: "Conectar",
    description: "Open the service connection menu",
  },
  "services.connectSSH": {
    message: "SSH",
    description: "SSH section label in the service connection menu",
  },
  "services.openBrowserTerminal": {
    message: "Abrir um terminal no navegador",
    description:
      "RequiresSshKey gate — zero-setup second door to the Web Shell (no SSH key)",
  },
  "services.connectInternal": {
    message: "Interno",
    description:
      "Internal-address section label in the service connection menu (Render's Connect → Internal tab)",
  },
  "services.connectInternalFreeNote": {
    message:
      "Web Services gratuitos não podem receber tráfego de rede privada. Faça upgrade para usar o endereço interno.",
    description:
      "Connect → Internal note for a free public web service: Render's Free plan does not support receiving private network traffic, so a sibling's internal call never reaches it (w8/m130)",
  },
  "services.headerServiceAddress": {
    message: "Endereço do serviço",
    description:
      "Label before a private service's internal address in the detail header (Render's private-service wording)",
  },
  "services.openPorts": {
    message: "Portas abertas",
    description:
      "Label before the TCP ports a private service's hostname serves, beside its Service Address and in Connect → Internal (Render's serviceDetails.openPorts, w6/m148)",
  },
  "services.openPortsPrimary": {
    message: "primária",
    description:
      "Marker on the open port that is the service's primary port — the <slug>:<port> suffix of its Service Address (w6/m148)",
  },
  "services.internalCopy": {
    message: "Copiar endereço interno",
    description: "Copy service internal address button",
  },
  "services.internalCopied": {
    message: "Endereço interno copiado",
    description: "Successful internal address copy",
  },
  "services.internalCopyError": {
    message: "Não foi possível copiar o endereço interno",
    description: "Failed internal address copy",
  },
  "services.sshCopy": {
    message: "Copiar comando SSH",
    description: "Copy service SSH command button",
  },
  "services.sshCopied": {
    message: "Comando SSH copiado",
    description: "Successful SSH command copy",
  },
  "services.sshCopyError": {
    message: "Não foi possível copiar o comando SSH",
    description: "Failed SSH command copy",
  },
  "services.sshUnavailable": {
    message: "SSH não está disponível",
    description: "Service header state when no SSH address is advertised",
  },
  "services.sshUnavailableHint": {
    message:
      "O SSH exige um serviço web, privado ou em segundo plano pago e em execução, além de um gateway ativo.",
    description: "Explanation for a service without an SSH address",
  },
  "services.sshUnavailableSuspended": {
    message: "Este serviço está suspenso. Retome-o para conectar por SSH.",
    description: "Why SSH is unavailable on a suspended service (w4/143)",
  },
  "services.sshUnavailableSuspendedFree": {
    message:
      "Este serviço está suspenso, e o SSH exige um plano de computação pago. Retomar restaura o tráfego; serviços gratuitos continuam sem acesso ao shell.",
    description:
      "Why SSH is unavailable on a suspended Free service: both blockers, without implying a resume grants SSH (w4/m183)",
  },
  "services.sshUnavailableFree": {
    message:
      "O SSH exige um plano de computação pago. Serviços gratuitos não incluem acesso ao shell.",
    description: "Why SSH is unavailable on a Free-plan service (w4/143)",
  },
  "services.sshRemedyResume": {
    message: "Retomar serviço",
    description:
      "Link from an SSH-unavailable state to the service's suspend/resume setting (w4/143)",
  },
  "services.sshRemedyChangePlan": {
    message: "Alterar plano de computação",
    description:
      "Link from an SSH-unavailable state on a Free service to its compute-plan page (w4/143)",
  },
  "services.shellDescription": {
    message:
      "Abra um shell em uma instância de serviço em execução a partir do seu terminal.",
    description: "Running-instance SSH connection page description",
  },
  "services.shellConnectionTitle": {
    message: "Conectar com SSH",
    description: "SSH connection card title",
  },
  "services.shellConnectionDescription": {
    message:
      "O bex se conecta a uma instância pronta já existente. Ele não cria uma instância de shell separada.",
    description: "SSH running-instance behavior explanation",
  },
  "services.shellCommand": {
    message: "Comando SSH",
    description: "Label for a copy-ready service SSH command",
  },
  "services.shellManageKeys": {
    message: "Gerenciar chaves públicas SSH",
    description: "Link from a service shell page to account SSH key settings",
  },
  "services.shellSessionLifecycle": {
    message:
      "O comando seleciona uma instância pronta. Reiniciar, refazer o deploy ou suspender o serviço encerra a sessão.",
    description: "SSH session selection and lifecycle guidance",
  },
  "services.shellUnavailableTitle": {
    message: "O acesso ao shell não está disponível",
    description: "Unavailable SSH connection card title",
  },
  "services.shellUnavailableBody": {
    message:
      "O acesso ao shell exige um serviço web, privado ou em segundo plano pago e em execução, além de um gateway SSH ativo.",
    description: "Unavailable SSH connection card explanation",
  },
  "services.shellWebTitle": {
    message: "Web Shell",
    description: "In-browser terminal card title",
  },
  "services.shellWebDescription": {
    message:
      "Execute comandos dentro de uma instância em execução a partir do navegador.",
    description: "In-browser terminal card description",
  },
  "services.shellConnecting": {
    message: "Conectando a uma instância em execução…",
    description: "Web shell status while the terminal is connecting",
  },
  "services.shellConnected": {
    message: "Conectado",
    description: "Web shell status when the terminal stream is live",
  },
  "services.shellClosed": {
    message: "Sessão encerrada",
    description: "Web shell status when the terminal stream has ended",
  },
  "services.shellErrorStatus": {
    message: "Erro de conexão",
    description:
      "Web shell status when the session failed to connect or errored",
  },
  "services.shellEnded": {
    message: "Sessão encerrada",
    description:
      "Web shell status when the server ended a live session for a stated reason (access, suspension, instance stopped, time limit)",
  },
  "services.shellEndRevoked": {
    message: "Seu acesso a este serviço foi encerrado.",
    description:
      "Web shell end reason: the caller lost access to the service mid-session (deliberately unspecific)",
  },
  "services.shellEndSuspended": {
    message:
      "O serviço foi suspenso, o que encerrou esta sessão. Retome-o para abrir uma nova.",
    description: "Web shell end reason: the service was suspended",
  },
  "services.shellEndIneligible": {
    message:
      "Este serviço não tem mais acesso ao shell (o plano ou o tipo dele mudou), o que encerrou esta sessão.",
    description:
      "Web shell end reason: the service moved to the Free plan or a type without instances",
  },
  "services.shellEndInstanceGone": {
    message:
      "A instância em que esta sessão estava sendo executada parou. Reconecte-se para abrir um shell em uma instância em execução.",
    description:
      "Web shell end reason: the attached instance stopped (restart, or the old instance leaving after a deploy went live)",
  },
  "services.shellEndTimeout": {
    message:
      "A sessão atingiu o limite de tempo. Reconecte-se para continuar trabalhando.",
    description: "Web shell end reason: the session hit its maximum duration",
  },
  "services.shellEndUnverified": {
    message:
      "Não foi possível verificar seu acesso novamente, então a sessão foi encerrada. Reconecte-se para tentar de novo.",
    description:
      "Web shell end reason: the periodic access check could not complete",
  },
  "services.shellReconnect": {
    message: "Reconectar",
    description: "Button that re-opens a closed web shell session",
  },
  "services.shellConnect": {
    message: "Iniciar shell",
    description: "Button that opens a web shell session",
  },
  "services.shellErrorGeneric": {
    message: "Não foi possível iniciar uma sessão de shell.",
    description: "Web shell generic connection error",
  },
  "services.shellErrorUnavailable": {
    message:
      "O shell no navegador não está ativado nesta plataforma. Use o comando SSH abaixo.",
    description:
      "Web shell error when the browser transport is unconfigured (503)",
  },
  "services.shellInstanceLabel": {
    message: "Instância",
    description: "Label for the web shell instance picker",
  },
  "services.shellInstanceAny": {
    message: "Qualquer instância pronta",
    description:
      "Web shell instance picker option that selects a random ready replica",
  },
  "services.shellInstanceSelect": {
    message: "Selecione uma instância",
    description: "Placeholder for the web shell instance picker",
  },
  "services.shellSshFallback": {
    message: "Prefere o seu próprio terminal? Use o comando SSH abaixo.",
    description: "Hint pointing web shell users to the copy-ready SSH command",
  },
  "services.actions": {
    message: "Ações",
    description: "Accessible heading for service configuration row actions",
  },
  "services.statTotal": {
    message: "Total de serviços",
    description: "Services page stat card label",
  },
  "services.statRunning": {
    message: "Em execução",
    description: "Services page stat card label",
  },
  "services.statSuspended": {
    message: "Suspensos",
    description: "Services page stat card label",
  },
  "services.cardTitle": {
    message: "Serviços",
    description:
      "Services table card title, also used as the metrics page back-link",
  },
  "services.colName": {
    message: "Nome",
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
    message: "Nome no Blueprint",
    description: "Immutable service name used to match Blueprint resources.",
  },
  "services.blueprintNameHelp": {
    message:
      "Use este nome imutável em services[].name no manifesto do seu Blueprint. Os nomes de exibição e os hostnames públicos podem ser diferentes.",
    description: "Immutable service name used to match Blueprint resources.",
  },
  "services.colSlug": {
    message: "Slug",
    description:
      "Service detail header fact (globally-unique platform-host segment, Render's slug field)",
  },
  "services.instancesSuspended": {
    message: "0 ({n} ao retomar)",
    description:
      "The header's Instances fact for a suspended service, which runs none; n is its configured count (w4/547)",
  },
  "services.instancesSleeping": {
    message: "0 ({n} ao despertar)",
    description:
      "The header's Instances fact for a sleeping Free service, which runs none; n is its configured count (w4/547)",
  },
  "services.colInstances": {
    message: "Instâncias",
    description: "Services table column header (replica count — bex-native)",
  },
  "services.colRevision": {
    message: "Revisão",
    description: "Services table column header (active revision — bex-native)",
  },
  "services.colCreated": {
    message: "Criado em",
    description: "Services table column header (relative age from createdAt)",
  },
  "services.colActions": {
    message: "Ações",
    description: "Services table actions column header (screen-reader only)",
  },
  "services.statusRunning": {
    message: "Em execução",
    description: "Services table status badge",
  },
  "services.statusSuspended": {
    message: "Suspenso",
    description: "Services table status badge",
  },
  "services.statusSleeping": {
    message: "Em repouso",
    description:
      "Services status badge: a free-tier App auto-hibernated after idle (bex extension)",
  },
  "services.undeployedChangesSuspendedHint": {
    message:
      "As alterações salvas ainda não estão no ar. Retome o serviço e depois faça um deploy para aplicá-las.",
    description:
      "Notice when a suspended service has saved settings that differ from the release it serves: no deploy can run until it resumes",
  },
  "services.undeployedChangesHint": {
    message:
      "As alterações salvas ainda não estão no ar. Use um deploy padrão para aplicá-las.",
    description:
      "Notice when saved settings differ from the running release, including Save only, cancellation, rollback or restart",
  },
  "services.undeployedChangesApplyingHint": {
    message:
      "As alterações salvas entram no ar quando o deploy atual terminar.",
    description:
      "Notice while an in-flight deploy already carries the saved settings, so no further deploy is needed (w4/m184)",
  },
  "services.undeployedChangesOverrideHint": {
    message:
      "Este deploy executa {running}, escolhida para este deploy. A imagem salva é {saved}; um deploy padrão volta a ela.",
    description:
      "Notice when a per-deploy image override (imageUrl/imgURL) runs a different image than the saved one: nothing was saved, and a standard deploy reverts to the saved image (w4/414)",
  },
  "services.statusSleepingHint": {
    message:
      "Em repouso para economizar recursos — desperta na próxima requisição à sua URL pública.",
    description:
      "Hint next to the Sleeping badge explaining free-tier auto-sleep + wake-on-request",
  },
  "services.statusPending": {
    message: "Pendente",
    description: "Services table status badge",
  },
  "services.statusBuilding": {
    message: "Em build",
    description: "Services table status badge",
  },
  "services.statusDeploying": {
    message: "Em deploy",
    description: "Services table status badge",
  },
  "services.statusCanceled": {
    message: "Cancelado",
    description:
      "Services table status badge for a service whose first-ever deploy the user canceled",
  },
  "services.statusDeleting": {
    message: "Excluindo",
    description:
      "Services status badge for a service whose deletion is finalizing",
  },
  "services.statusFailed": {
    message: "Com falha",
    description: "Services table status badge",
  },
  "services.statusUnknown": {
    message: "Desconhecido",
    description: "Services table status badge for an unrecognized phase",
  },
  "services.actionsMenu": {
    message: "Ações para {name}",
    description: "Accessible label for the per-row actions trigger",
  },
  "services.actionSuspend": {
    message: "Suspender",
    description: "Row action: park the service",
  },
  "services.actionResume": {
    message: "Retomar",
    description: "Row action: bring a suspended service back",
  },
  "services.actionRestart": {
    message: "Reiniciar",
    description:
      "Row action: restart the service on the commit or image it is running",
  },
  "services.confirmSuspendTitle": {
    message: "Suspender {name}?",
    description: "Suspend confirmation dialog title",
  },
  "services.confirmSuspendBody": {
    message:
      "O serviço escala para zero e deixa de atender tráfego. Sua URL e seus certificados são mantidos, e você pode retomá-lo a qualquer momento.",
    description:
      "Suspend confirmation dialog body, for the types served at a public host",
  },
  "services.confirmSuspendBodyNoUrl": {
    message:
      "O serviço escala para zero e deixa de ser executado. Você pode retomá-lo a qualquer momento.",
    description:
      "Suspend confirmation dialog body, for private services and background workers — neither has a public URL or certificates to keep",
  },
  "services.confirmSuspendBodyStatic": {
    message:
      "O site deixa de ser servido e responde 503 enquanto estiver suspenso. Sua URL e seus certificados são mantidos, e você pode retomá-lo a qualquer momento.",
    description:
      "Suspend confirmation dialog body for a static site, which has no instances to scale to zero (w4/380)",
  },
  "services.suspendCronDescription": {
    message:
      "Suspender este Cron Job pausa as próximas execuções agendadas. Uma execução ativa continua. Para interrompê-la, selecione Cancelar na página Execuções.",
    description:
      "Cron suspension explanation in Settings and suspend confirmations; suspension does not cancel an active run",
  },
  "services.confirmRestartTitle": {
    message: "Reiniciar {name}?",
    description: "Restart confirmation dialog title",
  },
  "services.confirmRestartBody": {
    message:
      "{name} reinicia no commit ou na imagem que está em execução agora. Os commits enviados desde então não passam por deploy. As novas instâncias substituem as antigas assim que estiverem saudáveis.",
    description:
      "Restart confirmation dialog body (service header and services list). A restart keeps the running release, as on Render; it never picks up newer commits",
  },
  "services.confirmRestartBodyStatic": {
    message:
      "{name} é publicado novamente a partir do commit que está sendo servido agora. Os commits enviados desde então não passam por deploy.",
    description:
      "Restart confirmation body for a static site, which has no instances to replace (w4/m141)",
  },
  "services.confirmCancel": {
    message: "Cancelar",
    description: "Confirmation dialog cancel button",
  },
  "services.toastSuspendSuccess": {
    message: "Suspendendo {name}…",
    description: "Toast shown after a suspend request is accepted",
  },
  "services.toastResumeSuccess": {
    message: "Retomando {name}…",
    description: "Toast shown after a resume request is accepted",
  },
  "services.toastRestartSuccess": {
    message: "Reiniciando {name}…",
    description: "Toast shown after a restart request is accepted",
  },
  "services.toastError": {
    message: "Não foi possível atualizar {name}. Tente novamente.",
    description: "Toast shown when a lifecycle action fails",
  },
  "services.errorTitle": {
    message: "Não foi possível carregar os serviços",
    description: "Services list error card title",
  },
  "services.errorBody": {
    message:
      "A requisição ao bex-api falhou. Verifique sua conexão e tente novamente.",
    description: "Services list error card body",
  },
  "services.detailErrorTitle": {
    message: "Não foi possível carregar o serviço",
    description: "Service detail query error card title",
  },
  "services.detailErrorBody": {
    message: "A requisição ao bex-api falhou. O serviço ainda pode existir.",
    description: "Service detail query error card body",
  },
  "services.emptyTitle": {
    message: "Nenhum serviço ainda",
    description: "Services list empty state title",
  },
  "services.emptyBody": {
    message: "Faça o deploy do seu primeiro App e ele aparecerá aqui.",
    description: "Services list empty state body",
  },
  "services.headerServiceId": {
    message: "ID do serviço:",
    description: "Service-detail header metadata label for the service id",
  },
  "services.headerServicePhase": {
    message: "Serviço",
    description:
      "Service-detail header label distinguishing operator/App phase from deploy status",
  },
  "services.headerLatestDeploy": {
    message: "Último deploy",
    description: "Service-detail header label for the newest deploy status",
  },
  "services.headerRuntime": {
    message: "Runtime",
    description: "Service-detail header label for the selected runtime",
  },
  "services.headerMoreFacts": {
    message: "Detalhes",
    description:
      "Service-detail header toggle, phones only, that shows or hides the slug, instances, revision and age line",
  },
  "services.headerLastRun": {
    message: "Última execução bem-sucedida",
    description:
      "Service-detail header: a cron job's second fact line, before the last successful run's date (Render's \"Last successful run <date>\", w3/m555)",
  },
  "services.headerRuns": {
    message: "Executa",
    description:
      "Service-detail header: the word before a cron job's command on its first fact line (Render's \"Runs <command> <schedule>\", w3/m555)",
  },
  "services.headerRunsDockerCmd": {
    message: "{cmd} do {dockerfile}",
    description:
      "Service-detail header: what a Docker or image cron job with no command override runs; {cmd} is the code word CMD and {dockerfile} the Dockerfile's path, both shown as code (Render's \"CMD in Dockerfile\", w3/m555)",
  },
  "services.headerNoSuccessfulRuns": {
    message: "Ainda não há execuções bem-sucedidas.",
    description:
      "Service-detail header: a cron job's second fact line before its first successful run (Render's text, w3/m555)",
  },
  "services.suspendedNoticeManual": {
    message: "Suspenso manualmente",
    description:
      "Title of the notice under a suspended cron job's header when its owner suspended it (Render's \"Manually suspended\", w3/m555)",
  },
  "services.suspendedNoticeTitle": {
    message: "Suspenso",
    description:
      "Title of the notice under a suspended cron job's header when the platform suspended it (Render's fallback \"Suspended\", w3/m555)",
  },
  "services.suspendedNoticeBody": {
    message: "Você não é cobrado por serviços suspensos.",
    description:
      "Body of the notice under a suspended cron job's header (Render's text, w3/m555)",
  },
  "services.suspendedNoticeResume": {
    message: "Retomar serviço",
    description:
      "The notice's inline action that resumes a suspended cron job (Render's \"Resume service\", w3/m555)",
  },
  "services.headerCopyServiceId": {
    message: "Copiar ID do serviço",
    description: "Accessible label for the header's service-id copy button",
  },
  "services.headerCopyUrl": {
    message: "Copiar URL do serviço",
    description: "Accessible label for the header's live-URL copy button",
  },
  "services.headerCopied": {
    message: "Copiado para a área de transferência",
    description: "Toast after copying a value from the service-detail header",
  },
  "services.headerCopyError": {
    message: "Não foi possível copiar para a área de transferência",
    description: "Toast when a service-detail header copy fails",
  },
  "services.navLogs": {
    message: "Logs",
    description: "Service-detail nav item (logs tab)",
  },
  "services.navMetrics": {
    message: "Métricas",
    description: "Service-detail nav item (metrics tab)",
  },
  "services.navRedirects": {
    message: "Redirecionamentos/Reescritas",
    description:
      "Service-detail nav item (static-site route rules page, Render's label)",
  },
  "services.navHeaders": {
    message: "Cabeçalhos",
    description:
      "Service-detail nav item (static-site response headers page, Render's label)",
  },
  "services.scalingTitle": {
    message: "Escalonamento automático",
    description: "Scaling tab section heading",
  },
  "services.autoscaleMinInstances": {
    message: "Mínimo de instâncias",
    description:
      "Accessible name of the minimum-instances number input beside the autoscaling range slider",
  },
  "services.autoscaleMaxInstances": {
    message: "Máximo de instâncias",
    description:
      "Accessible name of the maximum-instances number input beside the autoscaling range slider",
  },
  "services.scalingEnabled": {
    message: "Ativar escalonamento automático",
    description: "Autoscaling enable/disable toggle label",
  },
  "services.scalingMinInstances": {
    message: "Mín. de instâncias",
    description: "Autoscaling min instances input label",
  },
  "services.scalingMaxInstances": {
    message: "Máx. de instâncias",
    description: "Autoscaling max instances input label",
  },
  "services.scalingTargetCPU": {
    message: "% de CPU alvo",
    description: "Autoscaling target CPU utilization input label",
  },
  "services.scalingTargetMemory": {
    message: "% de memória alvo",
    description: "Autoscaling target memory utilization input label",
  },
  "services.scalingValidationMinMax": {
    message: "O mín. de instâncias deve ser ≤ ao máx. de instâncias.",
    description: "Autoscaling form validation when min exceeds max",
  },
  "services.scalingValidationTargets": {
    message:
      "Pelo menos uma meta de utilização (CPU ou memória) deve estar ativada.",
    description:
      "Autoscaling form validation when both CPU and memory targets are off",
  },
  "services.scalingSave": {
    message: "Salvar",
    description: "Autoscaling form save button",
  },
  "services.scalingSaved": {
    message: "Configurações de escalonamento automático salvas.",
    description: "Autoscaling save success toast",
  },
  "services.scalingDisabled": {
    message: "Escalonamento automático desativado.",
    description: "Autoscaling disable success toast",
  },
  "services.scalingError": {
    message: "Falha ao atualizar as configurações de escalonamento automático.",
    description: "Autoscaling save error toast",
  },
  "services.scalingDescription": {
    message:
      "Aumente ou reduza este serviço automaticamente com base na utilização de CPU e memória.",
    description: "Autoscaling card description",
  },
  "services.scalingOn": {
    message: "Escalonamento automático ativado",
    description: "Autoscaling main toggle label when enabled",
  },
  "services.scalingPaidOnly": {
    message:
      "O escalonamento automático está disponível em planos pagos. Faça upgrade para ativá-lo.",
    description: "Autoscaling card explanation for a free service",
  },
  "services.scalingPaidOnlyInactive": {
    message:
      "As configurações de escalonamento automático estão salvas, mas inativas no plano Free. Elas passam a valer quando este serviço migrar para um plano pago. Desative o escalonamento automático para remover as configurações salvas.",
    description:
      "Autoscaling card explanation for a free service carrying a stored autoscaling config (w4/365)",
  },
  "services.scalingProPlanOnly": {
    message:
      "O escalonamento automático está disponível no plano de workspace Pro e superiores. Este workspace está no plano Hobby.",
    description:
      "Autoscaling card notice on a Hobby workspace, followed by an Open Billing link (w7/m429)",
  },
  "services.scalingProPlanOnlyRunning": {
    message:
      "O escalonamento automático está disponível no plano de workspace Pro e superiores. Este workspace está no plano Hobby: a política abaixo continua em execução e pode ser desativada, mas alterá-la ou reativá-la exige o Pro.",
    description:
      "Autoscaling card notice on a Hobby workspace whose service already autoscales, followed by an Open Billing link (w7/m429)",
  },
  "services.scalingMetricsUnavailableTitle": {
    message:
      "O escalonamento automático está pausado: as métricas de utilização não estão disponíveis",
    description:
      "Autoscaling card notice title while the operator cannot read the service's CPU and memory utilization (w4/m364)",
  },
  "services.scalingMetricsUnavailableBody": {
    message:
      "O serviço mantém o número atual de instâncias até que a utilização de CPU e de memória possa ser lida novamente. Suas configurações de escalonamento automático estão salvas.",
    description:
      "Autoscaling card notice body while utilization metrics are unavailable (w4/m364)",
  },
  "services.scalingOff": {
    message: "Escalonamento automático desativado",
    description: "Autoscaling main toggle label when disabled",
  },
  "services.scalingInstancesTitle": {
    message: "Número de instâncias",
    description: "Autoscaling instances range-slider section heading",
  },
  "services.scalingInstancesHint": {
    message:
      "O bex ajusta o número de instâncias deste serviço dentro do intervalo que você especificar.",
    description: "Autoscaling instances range-slider section description",
  },
  "services.scalingCPUTitle": {
    message: "Utilização de CPU alvo",
    description: "Autoscaling CPU metric section heading",
  },
  "services.scalingCPUHint": {
    message:
      "Se a utilização média de CPU ficar bem acima ou abaixo deste valor, o bex adiciona ou remove instâncias de acordo.",
    description: "Autoscaling CPU metric section description",
  },
  "services.scalingMemoryTitle": {
    message: "Utilização de memória alvo",
    description: "Autoscaling memory metric section heading",
  },
  "services.scalingMemoryHint": {
    message:
      "Se a utilização média de memória ficar bem acima ou abaixo deste valor, o bex adiciona ou remove instâncias de acordo.",
    description: "Autoscaling memory metric section description",
  },
  "services.scalingCancel": {
    message: "Cancelar",
    description: "Autoscaling form cancel button",
  },
  "services.scalingSaveChanges": {
    message: "Salvar alterações",
    description: "Autoscaling form save-changes button",
  },
  "services.scalingDisableConfirmTitle": {
    message: "Desativar o escalonamento automático?",
    description: "Autoscaling disable confirmation dialog title (w7/m43)",
  },
  "services.scalingDisableConfirmBody": {
    message:
      "Seu serviço executará o número fixo de instâncias especificado em Escalonamento manual.",
    description: "Autoscaling disable confirmation dialog body",
  },
  "services.scalingDisableConfirmAction": {
    message: "Desativar",
    description: "Autoscaling disable confirmation dialog confirm button",
  },
  "services.scalingManualTitle": {
    message: "Escalonamento manual",
    description: "Scaling page: manual instance-count card title (w7/m43)",
  },
  "services.scalingManualDescription": {
    message:
      "Execute várias instâncias com balanceamento de carga automático. Todas as instâncias usam o mesmo plano de computação e são cobradas de acordo.",
    description: "Scaling page: manual instance-count card description",
  },
  "services.scalingManualDescriptionFree": {
    message:
      "Serviços gratuitos executam uma única instância. Faça upgrade para um plano pago para executar várias instâncias com balanceamento de carga.",
    description:
      "Scaling page: manual instance-count card description on the free plan",
  },
  "services.scalingManualInstances": {
    message: "Instâncias",
    description: "Scaling page: manual instance-count slider label",
  },
  "services.scalingMetricsTitle": {
    message: "Métricas recentes",
    description: "Scaling page: recent-metrics section title (w7/m43)",
  },
  "services.scalingMetricsNote": {
    message: "Exibindo as métricas das últimas 48 horas.",
    description: "Scaling page: recent-metrics window note",
  },
  "services.scalingMetricsViewAll": {
    message: "Ver todas as métricas.",
    description: "Scaling page: link to the full Metrics tab",
  },
  "services.scalingMetricsMemory": {
    message: "Utilização média de memória",
    description: "Scaling page: averaged memory utilization chart title",
  },
  "services.scalingMetricsCPU": {
    message: "Utilização média de CPU",
    description: "Scaling page: averaged CPU utilization chart title",
  },
  "services.scalingMetricsAcross": {
    message: "Em todas as instâncias",
    description: "Scaling page: recent-metrics chart subtitle",
  },
  "services.scalingMetricsEmpty": {
    message: "Nenhum dado coletado nas últimas 48 horas",
    description: "Scaling page: recent-metrics empty state",
  },
  "services.navEnvironment": {
    message: "Ambiente",
    description: "Service-detail nav item (environment variables tab)",
  },
  "services.envTitle": {
    message: "Variáveis de ambiente",
    description: "Environment tab card title",
  },
  "services.envDescription": {
    message:
      "Defina configurações e segredos específicos do ambiente e leia esses valores no seu código.",
    description: "Environment tab card description",
  },
  "services.envColKey": {
    message: "Chave",
    description: "Environment table column header (variable name)",
  },
  "services.envColValue": {
    message: "Valor",
    description: "Environment table column header (variable value)",
  },
  "services.envManifestManaged": {
    message: "Gerenciada pelo Blueprint",
    description:
      "Badge on an environment row whose value the service's render.yaml manifest owns",
  },
  "services.envManifestManagedBody": {
    message:
      "Esta variável é declarada no manifesto render.yaml do serviço, que controla o valor dela. Edite o manifesto e sincronize o Blueprint para alterá-la, ou declare-a com `sync: false` para que o painel passe a controlá-la.",
    description:
      "Explanation under a manifest-managed environment row: why it cannot be edited here and what to do instead",
  },
  "services.envShowSecret": {
    message: "Mostrar valor",
    description: "Environment row button to reveal a masked value",
  },
  "services.envHideSecret": {
    message: "Ocultar valor",
    description: "Environment row button to re-mask a revealed value",
  },
  "services.envRevealError": {
    message: "Não foi possível carregar o valor.",
    description: "Environment row inline error when a value reveal fails",
  },
  "services.envEmptyTitle": {
    message: "Nenhuma variável de ambiente",
    description: "Environment tab empty-state title",
  },
  "services.envEmptyBody": {
    message: "Adicione uma variável para configurar este serviço.",
    description: "Environment tab empty-state body",
  },
  "services.envUnavailableTitle": {
    message: "Variáveis de ambiente indisponíveis",
    description:
      "Environment tab state when the secret store is unconfigured (503)",
  },
  "services.envUnavailableBody": {
    message:
      "O armazenamento de segredos não está configurado para este deployment.",
    description: "Environment tab unavailable-state body",
  },
  "services.envForbiddenTitle": {
    message: "Não autorizado",
    description: "Environment tab state when the caller lacks permission (403)",
  },
  "services.envForbiddenBody": {
    message:
      "Você não tem permissão para ver as variáveis de ambiente deste serviço.",
    description: "Environment tab forbidden-state body",
  },
  "services.envErrorTitle": {
    message: "Não foi possível carregar as variáveis de ambiente",
    description: "Environment tab generic error title",
  },
  "services.envErrorBody": {
    message: "Algo deu errado. Tente novamente.",
    description: "Environment tab generic error body",
  },
  "services.envAdd": {
    message: "Adicionar variável",
    description: "Environment tab button to open the add-variable form",
  },
  "services.envExport": {
    message: "Exportar",
    description:
      "Environment tab button that downloads all service variables as dotenv",
  },
  "services.envExportSuccess": {
    message: "Ambiente exportado",
    description:
      "Toast after every current environment value was safely exported",
  },
  "services.envExportError": {
    message:
      "Não foi possível exportar o ambiente completo. Nenhum arquivo foi baixado.",
    description:
      "Fail-closed toast when any environment value cannot be revealed",
  },
  "services.envEdit": {
    message: "Editar",
    description: "Environment row button to edit a variable's value",
  },
  "services.envDelete": {
    message: "Excluir",
    description: "Environment row button to remove a variable",
  },
  "services.envValueFor": {
    message: "Valor de {name}",
    description:
      "Accessible name for a Value textbox once its Key is set (m101/t013)",
  },
  "services.envDeleteOne": {
    message: "Excluir {name}",
    description:
      "Accessible name for a Delete button once its Key is set (m101/t013)",
  },
  "services.envSave": {
    message: "Salvar",
    description: "Environment add/edit form save button",
  },
  "services.envCancel": {
    message: "Cancelar",
    description: "Environment add/edit form cancel button",
  },
  "services.envKeyPlaceholder": {
    message: "NAME_OF_VARIABLE",
    description: "Environment add-variable key input placeholder",
  },
  "services.envValuePlaceholder": {
    message: "valor",
    description: "Environment value input placeholder",
  },
  "services.envGenerate": {
    message: "Gerar",
    description: "Generate a cryptographically random environment value",
  },
  "services.envGeneratePlaceholder": {
    message: "Gerado com segurança ao salvar",
    description:
      "Environment value placeholder while server generation is selected",
  },
  "services.envInvalidKey": {
    message:
      "Use até 253 letras, dígitos, pontos, hifens e sublinhados; não use '.' nem comece com '..'.",
    description:
      "Environment add-variable validation message for an invalid key",
  },
  "services.envDeleteConfirmTitle": {
    message: "Remover {key}?",
    description: "Environment delete-confirmation dialog title",
  },
  "services.envDeleteConfirmBody": {
    message: "O serviço passará por um novo deploy sem esta variável.",
    description: "Environment delete-confirmation dialog body",
  },
  "services.envRolloutNote": {
    message:
      "O serviço está passando por um novo deploy para aplicar a alteração.",
    description:
      "Toast description after an env-var write (bex rolls the pods)",
  },
  "services.envSaveSuccess": {
    message: "{key} salva",
    description: "Toast on a successful env-var add/update",
  },
  "services.envSaveError": {
    message: "Não foi possível salvar {key}",
    description: "Toast on a failed env-var add/update",
  },
  "services.envDeleteSuccess": {
    message: "{key} removida",
    description: "Toast on a successful env-var delete",
  },
  "services.envDeleteError": {
    message: "Não foi possível remover {key}",
    description: "Toast on a failed env-var delete",
  },
  "services.secretFilesTitle": {
    message: "Arquivos secretos",
    description: "Environment tab secret-files section title",
  },
  "services.secretFilesDescription": {
    message:
      "Guarde arquivos com conteúdo secreto, como uma chave privada ou um arquivo .env. Após o próximo deploy, seu serviço lê cada um em /etc/secrets/<nome-do-arquivo>.",
    description: "Environment tab secret-files section description",
  },
  "services.secretFileColName": {
    message: "Nome do arquivo",
    description: "Secret-files table column header (file name)",
  },
  "services.secretFileColContent": {
    message: "Conteúdo",
    description: "Secret-files table column header (file body)",
  },
  "services.secretFileRevealError": {
    message:
      "Não foi possível carregar o conteúdo deste arquivo secreto. Feche a caixa de diálogo e tente novamente.",
    description:
      "Shown in the secret-file dialog when reading the stored contents fails.",
  },
  "services.secretFilesEmptyTitle": {
    message: "Nenhum arquivo secreto",
    description: "Secret-files empty-state title",
  },
  "services.secretFilesEmptyBody": {
    message: "Adicione um arquivo para montar conteúdo secreto neste serviço.",
    description: "Secret-files empty-state body",
  },
  "services.secretFilesUnavailableTitle": {
    message: "Arquivos secretos indisponíveis",
    description:
      "Secret-files state when the secret store is unconfigured (503)",
  },
  "services.secretFilesUnavailableBody": {
    message:
      "O armazenamento de segredos não está configurado para este deployment.",
    description: "Secret-files unavailable-state body",
  },
  "services.secretFilesForbiddenTitle": {
    message: "Não autorizado",
    description: "Secret-files state when the caller lacks permission (403)",
  },
  "services.secretFilesForbiddenBody": {
    message:
      "Você não tem permissão para ver os arquivos secretos deste serviço.",
    description: "Secret-files forbidden-state body",
  },
  "services.secretFilesErrorTitle": {
    message: "Não foi possível carregar os arquivos secretos",
    description: "Secret-files generic error title",
  },
  "services.secretFilesErrorBody": {
    message: "Algo deu errado. Tente novamente.",
    description: "Secret-files generic error body",
  },
  "services.secretFileAdd": {
    message: "Adicionar arquivo secreto",
    description: "Secret-files button to open the add-file form",
  },
  "services.secretFileNamePlaceholder": {
    message: "filename.ext",
    description: "Secret-files add-file name input placeholder",
  },
  "services.secretFileContentPlaceholder": {
    message: "conteúdo do arquivo",
    description: "Secret-files content input placeholder",
  },
  "services.secretFileInvalidName": {
    message:
      "Use até 253 letras, dígitos, pontos, hifens e sublinhados; não use '.' nem '..'.",
    description: "Secret-files add-file validation message for an invalid name",
  },
  "services.secretFileNameHint": {
    message:
      "Use até 253 letras, dígitos, pontos, hifens e sublinhados; não use '.' nem '..'.",
    description:
      "Neutral file-name rule under a new secret-file row before the user types or blurs — same rule as secretFileInvalidName but helper styling, not an error",
  },
  "services.secretFileDeleteConfirmTitle": {
    message: "Remover {name}?",
    description: "Secret-file delete-confirmation dialog title",
  },
  "services.secretFileDeleteConfirmBody": {
    message: "O serviço passará por um novo deploy sem este arquivo.",
    description: "Secret-file delete-confirmation dialog body",
  },
  "services.secretFileSaveSuccess": {
    message: "{name} salvo",
    description: "Toast on a successful secret-file add/update",
  },
  "services.secretFileSaveError": {
    message: "Não foi possível salvar {name}",
    description: "Toast on a failed secret-file add/update",
  },
  "services.secretFileDeleteSuccess": {
    message: "{name} removido",
    description: "Toast on a successful secret-file delete",
  },
  "services.secretFileDeleteError": {
    message: "Não foi possível remover {name}",
    description: "Toast on a failed secret-file delete",
  },
  "services.envGroupsTitle": {
    message: "Grupos de ambiente",
    description: "Environment tab env-groups section title",
  },
  "services.envGroupsDescription": {
    message:
      "Conjuntos reutilizáveis de variáveis de ambiente e arquivos secretos que você pode vincular a este e a outros serviços.",
    description: "Environment tab env-groups section description",
  },
  "services.envGroupsEmptyTitle": {
    message: "Nenhum grupo de ambiente",
    description: "Env-groups empty-state title",
  },
  "services.envGroupsEmptyBody": {
    message: "Crie um grupo para compartilhar configurações entre serviços.",
    description: "Env-groups empty-state body",
  },
  "services.envGroupsUnavailableTitle": {
    message: "Grupos de ambiente indisponíveis",
    description: "Env-groups state when the secret store is unconfigured (503)",
  },
  "services.envGroupsUnavailableBody": {
    message:
      "O armazenamento de segredos não está configurado para este deployment.",
    description: "Env-groups unavailable-state body",
  },
  "services.envGroupsForbiddenTitle": {
    message: "Não autorizado",
    description: "Env-groups state when the caller lacks permission (403)",
  },
  "services.envGroupsForbiddenBody": {
    message: "Você não tem permissão para ver os grupos de ambiente.",
    description: "Env-groups forbidden-state body",
  },
  "services.envGroupsErrorTitle": {
    message: "Não foi possível carregar os grupos de ambiente",
    description: "Env-groups generic error title",
  },
  "services.envGroupsErrorBody": {
    message: "Algo deu errado. Tente novamente.",
    description: "Env-groups generic error body",
  },
  "services.envGroupCreate": {
    message: "Criar grupo",
    description: "Env-groups button to open the create-group form",
  },
  "services.envGroupCreateSubmit": {
    message: "Criar",
    description: "Env-groups create-group form submit button",
  },
  "services.envGroupNamePlaceholder": {
    message: "group-name",
    description: "Env-groups create-group name input placeholder",
  },
  "services.envGroupNameLabel": {
    message: "Nome do grupo",
    description: "Env-groups create-group name input accessible label",
  },
  "services.envGroupInvalidName": {
    message: "Informe um nome de grupo.",
    description:
      "Env-groups create-group validation message for an invalid name",
  },
  "services.envGroupLinked": {
    message: "Vinculado",
    description:
      "Env-groups badge: this group is linked to the current service",
  },
  "services.envGroupWorkspaceScope": {
    message: "Workspace",
    description: "Scope label for a workspace-scoped environment group",
  },
  "services.envGroupOutOfScope": {
    message:
      "Com escopo em {groupScope}, mas este serviço está em {serviceScope} — as variáveis e os arquivos dele continuam valendo.",
    description:
      "Notice on a linked group whose scope no longer matches the service (w4/m238)",
  },
  "services.envGroupOutOfScopeNoEnvironment": {
    message:
      "Com escopo em {groupScope}, mas este serviço não tem ambiente — as variáveis e os arquivos dele continuam valendo.",
    description:
      "Notice on a linked group scoped to an environment while the service has none (w4/m238)",
  },
  "services.envGroupEmptyContents": {
    message: "Nenhuma variável ou arquivo ainda.",
    description: "Env-groups: shown when a group has no vars or secret files",
  },
  "services.envGroupKeyOverridden": {
    message: "Substituída pela variável de ambiente {key} do próprio serviço",
    description:
      "Tooltip on a linked group's key badge when a service variable of the same name shadows it at runtime (service wins)",
  },
  "services.envGroupFileOverridden": {
    message: "Substituído pelo arquivo secreto {name} do próprio serviço",
    description:
      "Tooltip on a linked group's secret-file badge when a service secret file of the same name shadows it at runtime (service wins)",
  },
  "services.envGroupKeyShadowedByGroup": {
    message: "Substituída por {group}, que foi criado depois",
    description:
      "Tooltip on a linked group's key badge when a MORE recently created linked group defines the same key and wins it",
  },
  "services.envGroupFileShadowedByGroup": {
    message: "Substituído por {group}, que foi criado depois",
    description:
      "Tooltip on a linked group's secret-file badge when a MORE recently created linked group defines the same file name and wins it",
  },
  "services.envGroupOverriddenNote": {
    message:
      "As chaves riscadas são substituídas pelas variáveis de ambiente do próprio serviço.",
    description:
      "Legend under a linked group's key list when at least one key is shadowed by a service variable",
  },
  "services.envGroupLink": {
    message: "Vincular",
    description: "Env-groups button: attach this group to the current service",
  },
  "services.envGroupUnlink": {
    message: "Desvincular",
    description:
      "Env-groups button: detach this group from the current service",
  },
  "services.envGroupLinkAria": {
    message: "Vincular {name}",
    description:
      "Accessible name of an env-group row's Link button on the service Environment tab, naming the group (w4/537)",
  },
  "services.envGroupUnlinkAria": {
    message: "Desvincular {name}",
    description:
      "Accessible name of an env-group row's Unlink button on the service Environment tab, naming the group (w4/537)",
  },
  "services.envGroupDelete": {
    message: "Excluir",
    description: "Env-groups action: delete the group",
  },
  "services.envGroupDeleteConfirmTitle": {
    message: "Excluir {name}?",
    description: "Env-group delete-confirmation dialog title",
  },
  "services.envGroupDeleteConfirmBody": {
    message:
      "O grupo é removido de todos os serviços aos quais está vinculado. Isso não pode ser desfeito.",
    description: "Env-group delete-confirmation dialog body",
  },
  "services.envGroupCreateSuccess": {
    message: "{name} criado",
    description: "Toast on a successful env-group create",
  },
  "services.envGroupCreateError": {
    message: "Não foi possível criar {name}",
    description: "Toast on a failed env-group create",
  },
  "services.envGroupDeleteSuccess": {
    message: "Grupo excluído",
    description: "Toast on a successful env-group delete",
  },
  "services.envGroupDeleteError": {
    message: "Não foi possível excluir o grupo",
    description: "Toast on a failed env-group delete",
  },
  "services.envGroupLinkSuccess": {
    message: "Grupo vinculado",
    description: "Toast on a successful env-group link",
  },
  "services.envGroupLinkError": {
    message: "Não foi possível vincular o grupo",
    description: "Toast on a failed env-group link",
  },
  "services.envGroupUnlinkSuccess": {
    message: "Grupo desvinculado",
    description: "Toast on a successful env-group unlink",
  },
  "services.envGroupUnlinkError": {
    message: "Não foi possível desvincular o grupo",
    description: "Toast on a failed env-group unlink",
  },
  "services.navSettings": {
    message: "Configurações",
    description: "Service-detail nav item (settings tab)",
  },
  "services.settingsNavigation": {
    message: "Seções de configurações",
    description: "Accessible label for the service settings section navigation",
  },
  "services.settingsDescription": {
    message:
      "Configure o nome, o tamanho da instância e outras configurações deste serviço.",
    description: "Settings tab card description",
  },
  "services.generalTitle": {
    message: "Geral",
    description:
      "Settings tab: first card heading, Render's 'General' section (w5/m52)",
  },
  "services.regionLabel": {
    message: "Região",
    description:
      "Settings General card: read-only platform placement row label",
  },
  "services.regionHint": {
    message:
      "A região em que o seu serviço é executado. Serviços na mesma região podem se comunicar pela rede privada.",
    description: "Settings General card: Region row explanatory copy",
  },
  "services.editRowSave": {
    message: "Salvar alterações",
    description:
      "Shared editable-field row: save button shown while editing (Render parity)",
  },
  "services.editRowCancel": {
    message: "Cancelar",
    description: "Shared editable-field row: cancel button shown while editing",
  },
  "services.editRowOptional": {
    message: "Opcional",
    description:
      "Shared editable-field row: badge next to a label for an optional field",
  },
  "services.displayNameLabel": {
    message: "Nome do serviço",
    description:
      "Settings tab row label for the mutable human-facing service name",
  },
  "services.displayNameHint": {
    message:
      "O ID do serviço continua sendo {id}; as URLs e a infraestrutura não mudam.",
    description:
      "Settings tab explanation that a display-name change preserves identity",
  },
  "services.displayNameEdit": {
    message: "Editar nome do serviço",
    description: "Accessible label for the service-name edit button",
  },
  "services.displayNameSuccess": {
    message: "Serviço renomeado para “{name}”.",
    description: "Toast after setDisplayName succeeds",
  },
  "services.displayNameCleared": {
    message: "Nome personalizado do serviço removido.",
    description: "Toast after clearing displayName",
  },
  "services.displayNameRefreshFailed": {
    message:
      "Renomeado para “{name}”, mas esta página não conseguiu atualizar. Tente novamente ou recarregue para ver o nome salvo.",
    description:
      "Warning after a service rename committed but the page could not re-read the saved name; offers Try again",
  },
  "services.displayNameUnpublished": {
    message:
      "O novo nome foi salvo, mas esta página ainda não conseguiu exibi-lo. Use Tentar novamente no aviso ou recarregue antes de editá-lo de novo.",
    description:
      "Why the service name row is locked after a rename saved but its re-read failed",
  },
  "services.displayNameError": {
    message: "Não foi possível renomear o serviço. Tente novamente.",
    description: "Toast after setDisplayName fails",
  },
  "services.settingsInstanceType": {
    message: "Plano de computação",
    description: "Settings tab row label for the App's current plan/tier",
  },
  "services.settingsNoInstanceType": {
    message: "Nenhum plano de computação definido",
    description: "Settings tab state for an untiered (bare-CR) App",
  },
  "services.settingsUpdate": {
    message: "Atualizar",
    description: "Settings tab link to the compute-plan picker",
  },
  "services.settingsIdleTimeout": {
    message: "Tempo limite de inatividade",
    description:
      "Settings tab: label for the free-tier auto-sleep window control",
  },
  "services.settingsIdleTimeoutHint": {
    message:
      "Serviços gratuitos entram em repouso após esse tempo sem requisições HTTP públicas ou mensagens WebSocket e despertam na próxima requisição pública. Requisições internas de outros serviços não os mantêm acordados nem os despertam.",
    description: "Settings tab: idle-timeout control help text (bex extension)",
  },
  "services.settingsIdleTimeoutPaid": {
    message: "Serviços pagos ficam sempre ativos e nunca entram em repouso.",
    description: "Settings tab: shown instead of the control on a paid plan",
  },
  "services.idleTimeoutDefault": {
    message: "Padrão da plataforma",
    description: "Idle-timeout option: 0 seconds = the operator's own window",
  },
  "services.idleTimeoutMinutes": {
    message: "{minutes} min",
    description: "Idle-timeout option label in minutes",
  },
  "services.idleTimeoutHours": {
    message: "{hours} h",
    description: "Idle-timeout option label in hours",
  },
  "services.idleTimeoutSeconds": {
    message: "{seconds} s",
    description: "Idle-timeout option label in seconds (non-round values)",
  },
  "services.idleTimeoutSuccess": {
    message: "Tempo limite de inatividade atualizado.",
    description: "Toast after setIdleTimeout succeeds",
  },
  "services.idleTimeoutError": {
    message: "Não foi possível atualizar o tempo limite de inatividade.",
    description: "Toast after setIdleTimeout fails",
  },
  "services.settingsMaxShutdownDelay": {
    message: "Atraso máximo de desligamento",
    description: "Settings row label for the graceful SIGTERM window",
  },
  "services.settingsMaxShutdownDelayHint": {
    message:
      "Aguarde de 1 a 300 segundos após o SIGTERM antes de forçar a parada do processo (padrão 30).",
    description: "Settings row help for the shutdown-delay range",
  },
  "services.maxShutdownDelayDiskReason": {
    message:
      "Não é possível alterar o atraso de desligamento de um serviço com disco persistente.",
    description:
      "Settings hint replacing the range on a service with a disk (Render's rule, render.com/docs/disks); a link to the Disk tab follows",
  },
  "services.maxShutdownDelayDiskLink": {
    message: "Ver disco",
    description:
      "Link after maxShutdownDelayDiskReason to the service's Disk tab",
  },
  "services.maxShutdownDelayEdit": {
    message: "Editar atraso máximo de desligamento",
    description: "Accessible label for the shutdown-delay edit button",
  },
  "services.maxShutdownDelaySuccess": {
    message:
      "Atraso máximo de desligamento atualizado. Um novo deploy foi iniciado.",
    description: "Toast after setMaxShutdownDelay succeeds",
  },
  "services.maxShutdownDelayRangeError": {
    message: "Informe um número inteiro de segundos de {min} a {max}.",
    description:
      "Inline validation message for a max shutdown delay outside 1–300 or not a whole number (w4/494)",
  },
  "services.maxShutdownDelayError": {
    message: "Não foi possível atualizar o atraso máximo de desligamento.",
    description: "Toast after setMaxShutdownDelay fails",
  },
  "services.settingsHealthChecksTitle": {
    message: "Health checks",
    description: "Settings tab: Health Checks section card title",
  },
  "services.settingsHealthChecksDescription": {
    message:
      "Configure o caminho HTTP que o bex consulta periodicamente para monitorar o seu serviço.",
    description: "Settings tab: Health Checks section card description",
  },
  "services.settingsHealthCheckPath": {
    message: "Caminho do health check",
    description: "Settings tab: health-check path row label",
  },
  "services.settingsHealthCheckPathHint": {
    message:
      "Um caminho HTTP que o bex consulta para decidir se o seu serviço está saudável; ele deve responder 2xx ou 3xx em até 5 segundos, e 60 segundos de falhas consecutivas reiniciam a instância — aponte para uma rota leve. Deixe vazio para verificar apenas se o processo está escutando na porta — a escolha certa quando o serviço não tem uma rota leve que retorne 2xx.",
    description: "Settings tab: health-check path row hint text",
  },
  "services.settingsHealthCheckPathPlaceholder": {
    message: "Vazio — verificar apenas a porta",
    description: "Settings tab: health-check path input placeholder",
  },
  "services.settingsHealthCheckPathEdit": {
    message: "Editar caminho do health check",
    description:
      "Settings tab: accessible label for the health-check path edit-pencil button",
  },
  "services.healthCheckPathSuccess": {
    message: "Caminho do health check atualizado. Um novo deploy foi iniciado.",
    description: "Toast after setHealthCheckPath succeeds",
  },
  "services.healthCheckPathError": {
    message: "Não foi possível atualizar o caminho do health check.",
    description: "Toast after setHealthCheckPath fails",
  },
  "services.settingsPortTitle": {
    message: "Porta",
    description: "Settings tab: Port section card title",
  },
  "services.settingsPortDescription": {
    message:
      "A porta em que o seu contêiner escuta. O bex encaminha o tráfego para ela e a injeta como $PORT.",
    description: "Settings tab: Port section card description",
  },
  "services.settingsPort": {
    message: "Porta",
    description: "Settings tab: service port row label",
  },
  "services.settingsPortHint": {
    message:
      "A porta em que o seu contêiner escuta (padrão {port}). O bex encaminha o tráfego para ela e a injeta como $PORT. Uma variável de ambiente PORT no serviço a substitui, como no Render. Salvar inicia um novo deploy e reinicia as instâncias.",
    description: "Settings tab: service port row hint text",
  },
  "services.settingsPortDetected": {
    message: "Detectada: {port}",
    description:
      "Settings tab: port placeholder for a service whose serving port bex detected from the listener its process binds, while it names no port of its own (w8/338)",
  },
  "services.settingsPortDetectedHint": {
    message:
      "O bex detectou que o seu serviço escuta na porta {port} e encaminha o tráfego para ela. $PORT continua {defaultPort}, como no Render. Informe uma porta para defini-la: o bex passa a encaminhar o tráfego para ela e a injeta como $PORT. Salvar inicia um novo deploy e reinicia as instâncias.",
    description:
      "Settings tab: service port row hint for a service serving on a detected port; PORT stays the default (w8/177, w8/267, w8/338)",
  },
  "services.settingsPortImageHint": {
    message:
      "Este serviço obtém suas portas das instruções EXPOSE da imagem. Definir uma porta aqui seleciona o listener primário — a porta para a qual o bex encaminha o tráfego, faz health checks e injeta como $PORT — enquanto as outras portas TCP declaradas pela imagem continuam publicadas. Limpe o campo para voltar a usar a porta da imagem. Salvar inicia um novo deploy e reinicia as instâncias.",
    description:
      "Settings tab: service port row hint for an image-port service (a Docker-built or prebuilt-image web or private service) whose ports come from its image's EXPOSE instructions; a blank port returns it to the image's (w6/m148, w4/m243, w4/m323)",
  },
  "services.settingsPortEdit": {
    message: "Editar porta",
    description:
      "Settings tab: accessible label for the service port edit-pencil button",
  },
  "services.portRangeError": {
    message: "Informe uma porta entre {min} e {max}.",
    description:
      "Validation message for a port outside the range the container can bind",
  },
  "services.portSuccess": {
    message: "Porta atualizada. Um novo deploy está sendo implantado.",
    description: "Toast after setPort succeeds",
  },
  "services.portError": {
    message: "Não foi possível atualizar a porta.",
    description: "Toast after setPort fails",
  },
  "services.settingsNotificationsTitle": {
    message: "Notificações",
    description: "Settings tab: Notifications section card title",
  },
  "services.settingsNotificationsDescription": {
    message: "Escolha quais notificações de deploy receber para este serviço.",
    description: "Settings tab: Notifications section card description",
  },
  "services.notificationsLabel": {
    message: "Notificações do serviço",
    description: "Settings tab: service notification policy label",
  },
  "services.notificationsHint": {
    message:
      "Substitua o padrão do workspace para este serviço. O padrão do workspace é definido em Configurações do workspace → Notificações.",
    description: "Settings tab: service notification policy explanation",
  },
  "services.notificationsEdit": {
    message: "Editar notificações do serviço",
    description: "Accessible label for the service-notifications edit button",
  },
  "services.notificationsOptionDefault": {
    message: "Usar o padrão do workspace ({level})",
    description:
      "Service notification option: inherit workspace default; {level} is the workspace's saved level (e.g. Only failure notifications)",
  },
  "services.notificationsOptionDefaultUnknown": {
    message: "Usar o padrão do workspace",
    description:
      "Service notification option: inherit workspace default, while the workspace level is loading or unreadable",
  },
  "services.notificationsOptionAll": {
    message: "Todas as notificações",
    description: "Service notification option: all lifecycle mail",
  },
  "services.notificationsOptionFailure": {
    message: "Somente notificações de falha",
    description: "Service notification option: failures only",
  },
  "services.notificationsOptionNone": {
    message: "Nenhuma",
    description: "Service notification option: no mail",
  },
  "services.notificationsSuccess": {
    message: "Configuração de notificações do serviço atualizada.",
    description: "Toast after notification policy update succeeds",
  },
  "services.notificationsError": {
    message:
      "Não foi possível atualizar a configuração de notificações do serviço.",
    description: "Toast after notification policy update fails",
  },
  "services.notifyOnFailLabel": {
    message: "Notificações de falha de deploy",
    description: "Settings tab: notifyOnFail row label",
  },
  "services.notifyOnFailHint": {
    message:
      "O padrão segue a preferência de notificação de cada membro; você pode forçar a ativação ou a desativação apenas para este serviço.",
    description: "Settings tab: notifyOnFail row hint text",
  },
  "services.notifyOnFailOptionDefault": {
    message: "Usar a preferência do membro",
    description: "notifyOnFail select option: default",
  },
  "services.notifyOnFailOptionNotify": {
    message: "Sempre notificar",
    description: "notifyOnFail select option: notify",
  },
  "services.notifyOnFailOptionIgnore": {
    message: "Nunca notificar",
    description: "notifyOnFail select option: ignore",
  },
  "services.notifyOnFailSuccess": {
    message: "Configuração de notificações atualizada.",
    description: "Toast after setNotifyOnFail succeeds",
  },
  "services.notifyOnFailError": {
    message: "Não foi possível atualizar a configuração de notificações.",
    description: "Toast after setNotifyOnFail fails",
  },
  "services.planPickerTitle": {
    message: "Plano de computação",
    description:
      "Accessible name of the Compute section's plan cards, shown after Edit (w6/m483)",
  },
  "services.computeTitle": {
    message: "Computação",
    description:
      "Compute page: title of the section holding the service's compute plan (Render: 'Under Compute, click Edit', w6/m483)",
  },
  "services.computePlanEdit": {
    message: "Editar",
    description:
      "Compute section button that opens the compute plan cards (w6/m483)",
  },
  "services.planPickerFreeGroup": {
    message: "Free",
    description:
      "Plan-picker section label separating the Free tier from paid tiers",
  },
  "services.planPickerFreeSmtp": {
    message:
      "Serviços gratuitos não podem enviar e-mail pelas portas SMTP 25, 465 ou 587.",
    description:
      "Plan picker note under the Free group: free instances are refused outbound SMTP, as on Render (w8/m127)",
  },
  "services.planPickerPaidSmtp": {
    message:
      "Serviços pagos podem enviar e-mail pela porta 587 ou 2525 com STARTTLS. As portas 25 e 465 não estão disponíveis em nenhum plano.",
    description:
      "Plan picker note under the Paid group: outbound SMTP ports paid services may use (w8/m127)",
  },
  "services.planPickerLegacyFreeWorker": {
    message:
      "Este Background Worker está no plano Free, que Background Workers não podem mais usar. Ele continua em execução. Escolher um plano de computação pago o tira do Free, e ele não poderá voltar.",
    description:
      "Plan picker notice: this legacy service type is still on Free, which is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerUntieredWorker": {
    message:
      "Este Background Worker não tem plano de computação, e Background Workers não podem mais usar o Free. Ele continua em execução. Escolha um plano de computação pago para atribuir um a ele.",
    description:
      "Plan picker notice: this service type has no compute plan, and Free is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerLegacyFreePrivateService": {
    message:
      "Este Private Service está no plano Free, que Private Services não podem mais usar. Ele continua em execução. Escolher um plano de computação pago o tira do Free, e ele não poderá voltar.",
    description:
      "Plan picker notice: this legacy service type is still on Free, which is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerUntieredPrivateService": {
    message:
      "Este Private Service não tem plano de computação, e Private Services não podem mais usar o Free. Ele continua em execução. Escolha um plano de computação pago para atribuir um a ele.",
    description:
      "Plan picker notice: this service type has no compute plan, and Free is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerLegacyFreeCronJob": {
    message:
      "Este Cron Job está no plano Free, que Cron Jobs não podem mais usar. Ele continua em execução. Escolher um plano de computação pago o tira do Free, e ele não poderá voltar.",
    description:
      "Plan picker notice: this legacy service type is still on Free, which is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerUntieredCronJob": {
    message:
      "Este Cron Job não tem plano de computação, e Cron Jobs não podem mais usar o Free. Ele continua em execução. Escolha um plano de computação pago para atribuir um a ele.",
    description:
      "Plan picker notice: this service type has no compute plan, and Free is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerPaidGroup": {
    message: "Pago",
    description: "Plan-picker section label for the paid tier ladder",
  },
  "services.planCurrent": {
    message: "Atual",
    description:
      "Plan picker: badge on the instance type the service is on now",
  },
  "services.planPickerCancel": {
    message: "Cancelar",
    description: "Plan-picker footer button: discard the selection",
  },
  "services.planPickerSave": {
    message: "Salvar",
    description:
      'Compute section footer button: confirm the plan change ("Select a new compute plan and click Save", render.com/docs/compute-plans)',
  },
  "services.planPickerConfirmTitle": {
    message: "Alterar o plano de computação para {name}?",
    description: "Plan-change confirm dialog title",
  },
  "services.planPickerConfirmBody": {
    message:
      "O serviço é redimensionado e atualizado sem tempo de inatividade — as requisições em andamento terminam antes de as instâncias antigas serem substituídas.",
    description: "Plan-change confirm dialog body",
  },
  "services.instanceTypePausedReason": {
    message:
      "“{name}” está temporariamente indisponível; escolha {largest} ou um plano de computação menor.",
    description:
      "A paused compute plan's tooltip and screen-reader reason, composed in the dashboard's language from the tier and the largest still offered; matches bex-api's English unavailableReason (w4/552)",
  },
  "services.planPickerConfirmBodyCron": {
    message:
      "As execuções iniciadas após a alteração usam o novo plano de computação.",
    description:
      "Plan-change confirm body for a cron job, which serves no requests: its next run takes the new size (w4/548)",
  },
  "services.planPickerConfirmBodyNeverLive": {
    message:
      "O serviço é executado no novo plano de computação. Nada está sendo servido ainda — ele entra no ar quando um deploy for bem-sucedido.",
    description:
      "Plan-change confirm body for a service that has never gone live (w4/548)",
  },
  "services.planPickerConfirmRunPrice": {
    message:
      "Este plano de computação custa ${price} por minuto de execução, cobrado por segundo enquanto uma execução está em andamento. A cobrança mínima é de ${minimum} por mês por Cron Job.",
    description:
      "Plan-change confirm for a cron job: the new compute plan's run-time rate (w4/546); minimum is the cron job minimum in USD without the currency symbol (w9/m448)",
  },
  "services.createFieldPlanCronHint": {
    message:
      "Um Cron Job é cobrado por segundo enquanto uma execução está em andamento. A cobrança mínima é de ${minimum} por mês por Cron Job.",
    description:
      "Under the create form's compute plans for a cron job, which are priced per minute of run time (w4/546); minimum is the cron job minimum in USD without the currency symbol (w9/m448)",
  },
  "services.planPickerConfirmPrice": {
    message: "Este plano de computação custa ${price}/mês.",
    description:
      "Plan-change confirm: the new compute plan's monthly price, from the API's price sheet (w4/400)",
  },
  "services.planPickerSuccess": {
    message: "Plano de computação atualizado para {name}",
    description: "Toast on a successful plan change",
  },
  "services.planPickerError": {
    message:
      "Não foi possível atualizar o plano de computação. Tente novamente.",
    description: "Toast on a failed plan change",
  },
  "services.planPickerErrorTitle": {
    message: "Não foi possível carregar os planos de computação",
    description: "Plan-picker error state title (instanceTypes query failed)",
  },
  "services.planPickerErrorBody": {
    message:
      "A requisição ao bex-api falhou. Verifique sua conexão e tente novamente.",
    description: "Plan-picker error state body",
  },
  "services.domainsTitle": {
    message: "Domínios personalizados",
    description: "Settings tab custom-domains section title",
  },
  "services.domainsDescription": {
    message:
      "Aponte domínios personalizados que você possui para este serviço.",
    description: "Settings tab custom-domains section description",
  },
  "services.domainColName": {
    message: "Nome",
    description: "Custom-domains table column header (the FQDN)",
  },
  "services.domainColVerified": {
    message: "Status de verificação",
    description: "Custom-domains table column header (DNS verification)",
  },
  "services.domainColCertificate": {
    message: "Status do certificado",
    description:
      "Custom-domains table column header (TLS certificate serving state)",
  },
  "services.domainColActions": {
    message: "Ações",
    description:
      "Custom-domains table actions column header (screen-reader only)",
  },
  "services.domainVerified": {
    message: "Verificado",
    description:
      "Custom-domains status badge: DNS or TLS verification is complete",
  },
  "services.domainCertActive": {
    message: "Ativo",
    description:
      "Custom-domains status badge: certificate issued and serving traffic",
  },
  "services.domainCertBlockedLabel": {
    message: "Certificado bloqueado:",
    description:
      "Label before cert-manager's verbatim reason a custom domain's TLS certificate has not been issued",
  },
  "services.domainCertBlockedHint": {
    message:
      "O registro DNS deve ser somente DNS (sem proxy) e apontar para o hostname bex do seu serviço, ou a verificação do certificado não conseguirá alcançá-lo.",
    description:
      "Fix-it guidance shown with a blocked-certificate reason: proxied records (e.g. Cloudflare's orange cloud) break the ACME HTTP-01 challenge",
  },
  "services.domainPending": {
    message: "Pendente",
    description:
      "Custom-domains status badge: DNS or certificate is not complete",
  },
  "services.domainActionsMenu": {
    message: "Ações para {domain}",
    description: "Accessible label for the per-domain actions trigger",
  },
  "services.domainDelete": {
    message: "Excluir",
    description: "Custom-domains row action: remove the domain",
  },
  "services.domainCancel": {
    message: "Cancelar",
    description: "Custom-domains dialog cancel button",
  },
  "services.domainDeleteConfirmTitle": {
    message: "Excluir {name}?",
    description: "Custom-domain delete-confirmation dialog title",
  },
  "services.domainDeleteConfirmBody": {
    message:
      "O serviço deixa de servir este domínio. A regra de Ingress dele é removida e o certificado TLS é deixado para expirar. Isso não pode ser desfeito.",
    description: "Custom-domain delete-confirmation dialog body",
  },
  "services.domainDeletePairConfirmBody": {
    message:
      "Isso remove {name} e o redirecionamento adicionado automaticamente, {sibling}. As duas regras de Ingress são removidas e os certificados TLS são deixados para expirar. Isso não pode ser desfeito.",
    description:
      "Custom-domain delete confirmation when deleting a canonical host also removes its generated redirecting sibling",
  },
  "services.domainDeleteGeneratedConfirmBody": {
    message:
      "Isso remove apenas {name}. {canonical} continua sendo o domínio canônico e não redireciona. A regra de Ingress do domínio removido é removida e o certificado TLS dele é deixado para expirar. Isso não pode ser desfeito.",
    description:
      "Custom-domain delete confirmation when deleting only the generated redirecting sibling",
  },
  "services.domainDeletePendingConfirmBody": {
    message:
      "Isso remove a solicitação pendente de {name}. Ele nunca foi verificado, então nada foi servido nele e nenhum certificado foi emitido.",
    description:
      "Custom-domain delete confirmation for a domain still unverified: it was never routed (w4/142)",
  },
  "services.domainDeletePendingPairConfirmBody": {
    message:
      "Isso remove as solicitações pendentes de {name} e {sibling}. Nenhum dos dois foi verificado, então nada foi servido neles e nenhum certificado foi emitido.",
    description:
      "Custom-domain delete confirmation for a still-pending domain and its auto-paired sibling (w4/142)",
  },
  "services.domainDeletePendingGeneratedConfirmBody": {
    message:
      "Isso remove apenas a solicitação pendente de {name}. {canonical} permanece como o domínio canônico. Ele nunca foi verificado, então nada foi servido nele e nenhum certificado foi emitido.",
    description:
      "Custom-domain delete confirmation for a still-pending generated sibling: only it is removed, its canonical claim stays (w4/539)",
  },
  "services.domainAdd": {
    message: "Adicionar domínio personalizado",
    description: "Custom-domains button to open the add-domain dialog",
  },
  "services.domainAddTitle": {
    message: "Adicionar um domínio personalizado",
    description: "Add-domain dialog title",
  },
  "services.domainAddDescription": {
    message:
      "Informe um domínio que você possui. Aponte o DNS dele para este serviço, e o bex emite um certificado TLS automaticamente.",
    description: "Add-domain dialog description",
  },
  "services.domainPlaceholder": {
    message: "www.example.com",
    description: "Add-domain FQDN input placeholder",
  },
  "services.domainInvalid": {
    message: "Informe um domínio válido, ex.: www.example.com.",
    description: "Add-domain validation message for a malformed hostname",
  },
  "services.domainAddButton": {
    message: "Adicionar domínio",
    description: "Add-domain dialog submit button",
  },
  "services.domainAddPairSuccess": {
    message: "{name} e {sibling} adicionados",
    description:
      "Toast on a custom-domain add that auto-paired its www/apex sibling (w4/618)",
  },
  "services.domainAddSuccess": {
    message: "{name} adicionado",
    description: "Toast on a successful custom-domain add",
  },
  "services.domainAddError": {
    message: "Não foi possível adicionar {name}",
    description: "Toast on a failed custom-domain add",
  },
  "services.domainAddConflict": {
    message: "{name} já está em uso por outro serviço",
    description:
      "Toast when a custom-domain add is rejected because the host is registered on another service (409)",
  },
  "services.domainAddReserved": {
    message: "{name} é um hostname reservado da plataforma",
    description:
      "Toast when a custom-domain add is rejected because the host is a platform-owned name (400)",
  },
  "services.domainAddOwnPlatformHost": {
    message:
      "{name} já é o subdomínio da plataforma deste serviço, então não pode ser adicionado como domínio personalizado",
    description:
      "Toast when a custom-domain add names the service's own platform subdomain (400 CUSTOM_DOMAIN_OWN_PLATFORM_HOST, w4/558)",
  },
  "services.domainDeletePairSuccess": {
    message: "{name} e {sibling} removidos",
    description:
      "Toast on a custom-domain delete that also removed its auto-paired sibling (w4/618)",
  },
  "services.domainDeleteSuccess": {
    message: "{name} removido",
    description: "Toast on a successful custom-domain delete",
  },
  "services.domainDeleteError": {
    message: "Não foi possível remover {name}",
    description: "Toast on a failed custom-domain delete",
  },
  "services.domainPropagateNote": {
    message: "O DNS e o certificado TLS se propagam em segundo plano.",
    description:
      "Toast description after a custom-domain add (async convergence)",
  },
  "services.domainDnsToggle": {
    message: "Mostrar a configuração de DNS de {domain}",
    description:
      "aria-label for the per-domain DNS-instructions disclosure toggle",
  },
  "services.domainDnsTitle": {
    message: "Configuração de DNS",
    description: "Heading of the per-domain DNS-instructions panel",
  },
  "services.domainDnsSubdomainGuidance": {
    message:
      "Crie este registro na zona {zone} no seu provedor de DNS. O Host é relativo a essa zona. A verificação e o certificado TLS acontecem automaticamente assim que ele for resolvido.",
    description:
      "Guidance line above the DNS record for a subdomain — names the zone Host is relative to (w4/092)",
  },
  "services.domainDnsWildcardChallenge": {
    message:
      "Um domínio curinga também precisa deste registro. Ele permite que o bex emita e renove o certificado TLS do domínio.",
    description:
      "Above a wildcard custom domain's second DNS record, its _acme-challenge CNAME (w4/m326; Render's docs say it enables certificate issuance and renewal)",
  },
  "services.domainDnsApexFallback": {
    message:
      "Sem ALIAS/ANAME nem CNAME flattening? Crie um registro A para @ com um dos endereços de borda atuais abaixo. Esses endereços podem mudar — um registro ALIAS/ANAME continua funcionando quando isso acontece.",
    description:
      "A-record fallback under an apex domain's DNS instructions, shown with the resolved edge addresses (w4/378)",
  },
  "services.domainDnsApexGuidance": {
    message:
      "Domínios apex não podem usar um CNAME simples — crie este registro ALIAS/ANAME (ou de CNAME flattening) na zona {zone} (@ significa o apex da zona). Se o seu provedor não oferecer nenhum dos dois, crie um registro A para @ usando os endereços IP para os quais o Destino é resolvido. A verificação e o TLS acontecem automaticamente assim que ele for resolvido, e o bex configura o redirecionamento www ↔ apex para você.",
    description:
      "Guidance line above the DNS record for an apex domain — names the zone Host is relative to (w4/092)",
  },
  "services.domainRecordType": {
    message: "Tipo",
    description: "Label for the DNS record type field (CNAME/ALIAS)",
  },
  "services.domainRecordHost": {
    message: "Host",
    description: "Label for the DNS record host/name field",
  },
  "services.domainRecordTarget": {
    message: "Destino",
    description: "Label for the DNS record target/value field",
  },
  "services.domainCopyHost": {
    message: "Copiar o host {type} de {domain}",
    description: "Accessible label for copying a DNS host field",
  },
  "services.domainCopyTarget": {
    message: "Copiar o destino {type} de {domain}",
    description: "Accessible label for copying a DNS target/value field",
  },
  "services.domainDnsUnavailable": {
    message:
      "O destino de DNS ainda não está disponível — verifique novamente quando o serviço estiver em execução.",
    description: "Shown when the backend couldn't derive the DNS record target",
  },
  "services.domainRecheck": {
    message: "Verificar novamente",
    description: "Button that re-checks a domain's DNS/certificate status",
  },
  "services.domainRecheckAria": {
    message: "Verificar {domain} novamente",
    description:
      "Accessible name of a custom domain's DNS-panel Re-check button, naming the domain (w4/540)",
  },
  "services.domainCopied": {
    message: "Copiado para a área de transferência",
    description: "Toast when a DNS record value is copied",
  },
  "services.domainCopyError": {
    message: "Não foi possível copiar para a área de transferência",
    description: "Toast when copying a DNS record value fails",
  },
  "services.domainAddedTitle": {
    message: "Domínio adicionado — configure o DNS",
    description: "Title of the post-add DNS-record step in the add dialog",
  },
  "services.domainAddedDescription": {
    message:
      "Crie o registro DNS no seu provedor; a verificação é concluída automaticamente assim que ele for resolvido.",
    description: "Subtitle of the post-add DNS-record step in the add dialog",
  },
  "services.domainRedirectsTo": {
    message: "Redireciona para {canonical}",
    description:
      "Note under an auto-paired sibling domain showing its canonical redirect target",
  },
  "services.domainRedirectsHere": {
    message: "{sibling} redireciona para cá",
    description:
      "Note under a canonical domain showing which auto-paired sibling redirects to it",
  },
  "services.domainPairedDnsTitle": {
    message:
      "{sibling} foi adicionado automaticamente e redireciona para {canonical} — configure o DNS dele também",
    description:
      "Heading of the second DNS-record block in the add dialog when the add auto-paired a www<->apex sibling (w6/m23)",
  },
  "services.domainDone": {
    message: "Concluir",
    description: "Button closing the post-add DNS-record step",
  },
  "services.domainVerifySuccess": {
    message: "{name} verificado",
    description: "Toast when a re-check finds the domain verified",
  },
  "services.domainVerifyPending": {
    message:
      "{name} ainda não foi verificado — o DNS pode ainda estar se propagando.",
    description: "Toast when a re-check finds the domain still pending",
  },
  "services.domainVerifyError": {
    message: "Não foi possível verificar {name} novamente.",
    description: "Toast when the re-check request fails",
  },
  "services.domainsEmptyTitle": {
    message: "Nenhum domínio personalizado",
    description: "Custom-domains empty-state title",
  },
  "services.domainsEmptyBody": {
    message:
      "Adicione um domínio que você possui para servir este serviço por ele.",
    description: "Custom-domains empty-state body",
  },
  "services.domainsErrorTitle": {
    message: "Não foi possível carregar os domínios personalizados",
    description: "Custom-domains generic error title",
  },
  "services.domainsErrorBody": {
    message:
      "A requisição ao bex-api falhou. Verifique sua conexão e tente novamente.",
    description: "Custom-domains generic error body",
  },
  "services.platformSubdomainTitle": {
    message: "Subdomínio da plataforma",
    description: "Settings tab platform-subdomain section title",
  },
  "services.platformSubdomainDescription": {
    message:
      "Controle se o seu serviço responde no subdomínio da plataforma bex, além de quaisquer domínios personalizados que você configurou.",
    description: "Settings tab platform-subdomain section description",
  },
  "services.platformSubdomainEnabled": {
    message: "Ativado",
    description: "Platform-subdomain toggle label when the subdomain is active",
  },
  "services.platformSubdomainDisabled": {
    message: "Desativado",
    description:
      "Platform-subdomain toggle label when the subdomain is disabled",
  },
  "services.platformSubdomainPending": {
    message:
      "A URL da plataforma é atribuída quando o serviço estiver em execução.",
    description: "Platform-subdomain state when the service has no URL yet",
  },
  "services.platformSubdomainDisabledNote": {
    message:
      "O subdomínio da plataforma está desativado. Seu serviço só pode ser acessado por domínios personalizados.",
    description:
      "Platform-subdomain note shown when the policy is set to disabled",
  },
  "services.platformSubdomainToggleLabel": {
    message: "Alternar subdomínio da plataforma",
    description: "Accessible label for the platform-subdomain Switch",
  },
  "services.platformSubdomainNeedsVerifiedDomain": {
    message:
      "Verifique um domínio personalizado antes de desativar o subdomínio da plataforma, para que o serviço mantenha um endereço.",
    description:
      "Hint beside the platform-subdomain switch when no custom domain is verified yet (w4/142)",
  },
  "services.platformSubdomainNeedsVerifiedDomainPending": {
    message:
      "{names} precisa concluir a verificação antes que o subdomínio da plataforma possa ser desativado.",
    description:
      "Hint beside the platform-subdomain switch when custom domains exist but are all still pending verification (w4/142)",
  },
  "services.subdomainPolicySuccess": {
    message: "Configuração do subdomínio da plataforma atualizada.",
    description: "Toast on successful setSubdomainPolicy mutation",
  },
  "services.subdomainPolicyError": {
    message:
      "Não foi possível atualizar a configuração do subdomínio da plataforma. Tente novamente.",
    description: "Toast on failed setSubdomainPolicy mutation",
  },
  "services.maintenanceModeTitle": {
    message: "Modo de manutenção",
    description: "Settings tab maintenance-mode section title",
  },
  "services.maintenanceModeDescription": {
    message:
      "Tire este serviço do ar atrás de uma página de manutenção sem suspendê-lo — os pods continuam em execução, apenas o tráfego público é redirecionado.",
    description: "Settings tab maintenance-mode section description",
  },
  "services.maintenanceModeEnabled": {
    message: "Ativado",
    description: "Maintenance-mode toggle label when active",
  },
  "services.maintenanceModeDisabled": {
    message: "Desativado",
    description: "Maintenance-mode toggle label when inactive",
  },
  "services.maintenanceModeSwitchHint": {
    message:
      "Todos os hosts servidos por este serviço responderão com a página de manutenção.",
    description: "Maintenance-mode section hint text next to the switch",
  },
  "services.maintenanceModePaidOnly": {
    message:
      "O modo de manutenção está disponível em planos pagos de Web Service.",
    description: "Maintenance mode free-plan eligibility note",
  },
  "services.maintenanceModeToggleLabel": {
    message: "Alternar modo de manutenção",
    description: "Accessible label for the maintenance-mode Switch",
  },
  "services.maintenanceModeUriLabel": {
    message: "URL da página de manutenção personalizada",
    description: "Label for the maintenance-mode custom-page URL field",
  },
  "services.maintenanceModeUriPlaceholder": {
    message: "https://status.example.com/maintenance (opcional)",
    description: "Placeholder for the maintenance-mode custom-page URL field",
  },
  "services.maintenanceModeUriHint": {
    message:
      "É buscada e servida no lugar da página padrão. Deixe vazio para usar a página de manutenção padrão do bex. Não pode apontar para a própria URL deste serviço.",
    description: "Hint text under the maintenance-mode custom-page URL field",
  },
  "services.maintenanceModeUriEdit": {
    message: "Editar URL da página de manutenção",
    description:
      "Accessible label for the maintenance-page URL edit-pencil button",
  },
  "services.maintenanceModeEnableAction": {
    message: "Ativar modo de manutenção",
    description: "Confirm-dialog action button for enabling maintenance mode",
  },
  "services.confirmMaintenanceModeTitle": {
    message: "Ativar o modo de manutenção?",
    description: "Confirm-dialog title for enabling maintenance mode",
  },
  "services.confirmMaintenanceModeBody": {
    message:
      "{name} exibirá uma página de manutenção para todos os visitantes até você desativar isto. Os pods do serviço continuam em execução.",
    description: "Confirm-dialog body for enabling maintenance mode",
  },
  "services.maintenanceModeEnabledSuccess": {
    message: "Modo de manutenção ativado.",
    description: "Toast on successfully enabling maintenance mode",
  },
  "services.maintenanceModeDisabledSuccess": {
    message: "Modo de manutenção desativado.",
    description: "Toast on successfully disabling maintenance mode",
  },
  "services.maintenancePageSavedSuccess": {
    message: "Página de manutenção salva.",
    description: "Toast on saving only the custom maintenance page URL",
  },
  "services.maintenancePageClearedSuccess": {
    message: "Página de manutenção padrão restaurada.",
    description: "Toast on clearing the custom maintenance page URL",
  },
  "services.maintenanceModeError": {
    message:
      "Não foi possível atualizar o modo de manutenção. Tente novamente.",
    description: "Toast on failed setMaintenanceMode mutation",
  },
  "services.edgeCachingTitle": {
    message: "Cache de borda",
    description: "Settings tab edge-caching section title (Render parity)",
  },
  "services.edgeCachingDescription": {
    message:
      "Armazene respostas em cache na borda para que requisições repetidas sejam atendidas sem chegar a este serviço.",
    description: "Settings tab edge-caching section description",
  },
  "services.edgeCachingFileTypesLabel": {
    message: "Tipos de arquivo que podem ir para o cache",
    description: "Edge-caching radio group label",
  },
  "services.edgeCachingOptionNone": {
    message: "Nenhum",
    description: "Edge-caching option: no-cache profile",
  },
  "services.edgeCachingOptionNoneDesc": {
    message: "Atender todas as respostas a partir do seu serviço.",
    description: "Edge-caching no-cache option description",
  },
  "services.edgeCachingOptionStatic": {
    message: "Arquivos estáticos comuns",
    description: "Edge-caching option: origin-controlled profile",
  },
  "services.edgeCachingOptionStaticDesc": {
    message:
      "Armazene em cache tipos comuns de arquivos estáticos, como CSS, JavaScript, imagens e fontes, a menos que o cabeçalho Cache-Control da resposta diga o contrário.",
    description: "Edge-caching origin-controlled option description",
  },
  "services.edgeCachingOptionAll": {
    message: "Todos os arquivos",
    description: "Edge-caching option: origin-controlled-all profile",
  },
  "services.edgeCachingOptionAllDesc": {
    message:
      "Armazene em cache toda resposta elegível, a menos que o cabeçalho Cache-Control dela diga o contrário. Garanta que o conteúdo dinâmico defina Cache-Control para não ser armazenado em cache.",
    description:
      "Edge-caching origin-controlled-all option description, with the dynamic-content warning",
  },
  "services.edgeCachingPurgeOnDeploy": {
    message: "O cache é limpo a cada deploy bem-sucedido.",
    description: "Note under the edge-caching options",
  },
  "services.edgeCachingPaidOnly": {
    message:
      "O cache de borda não está disponível para Web Services gratuitos.",
    description: "Edge-caching free-plan eligibility note",
  },
  "services.edgeCachingSave": {
    message: "Salvar cache de borda",
    description: "Edge-caching save button",
  },
  "services.edgeCachingSaving": {
    message: "Salvando…",
    description: "Edge-caching save button while the mutation is in flight",
  },
  "services.edgeCachingSaved": {
    message: "Cache de borda atualizado.",
    description: "Toast on successful setCacheProfile mutation",
  },
  "services.edgeCachingError": {
    message: "Não foi possível atualizar o cache de borda. Tente novamente.",
    description: "Toast on failed setCacheProfile mutation",
  },
  "services.edgeCachingPurgeAction": {
    message: "Limpar cache",
    description: "Edge-caching purge button and confirm action",
  },
  "services.edgeCachingPurgeConfirmTitle": {
    message: "Limpar o cache de borda?",
    description: "Confirm-dialog title for purging the edge cache",
  },
  "services.edgeCachingPurgeConfirmBody": {
    message:
      "Todas as respostas em cache são descartadas. Até o cache aquecer de novo, as requisições vão para o seu serviço.",
    description: "Confirm-dialog body for purging the edge cache",
  },
  "services.edgeCachingPurged": {
    message: "Limpeza do cache iniciada.",
    description: "Toast on successful purgeServiceCache mutation",
  },
  "services.edgeCachingPurgeError": {
    message: "Não foi possível limpar o cache. Tente novamente.",
    description: "Toast on failed purgeServiceCache mutation",
  },
  "services.runtimeTitle": {
    message: "Runtime",
    description: "Settings tab runtime section title (Render parity)",
  },
  "services.runtimeDescription": {
    message: "O ambiente em que o bex faz o build e executa este serviço.",
    description: "Settings tab runtime section description",
  },
  "services.runtimeLabel": {
    message: "Runtime",
    description: "Settings runtime select label",
  },
  "services.runtimeNote": {
    message:
      "Alterar o runtime refaz o build e o deploy do serviço. Os runtimes nativos usam o comando de build e o comando de início abaixo.",
    description: "Note beside the Settings runtime select",
  },
  "services.runtimePlaceholder": {
    message: "Selecione um runtime",
    description:
      "Settings runtime select placeholder for a legacy service with no runtime recorded",
  },
  "services.runtimeSave": {
    message: "Salvar runtime",
    description: "Settings runtime save button",
  },
  "services.runtimeSaving": {
    message: "Salvando…",
    description: "Settings runtime save button while the mutation is in flight",
  },
  "services.runtimeConfirmTitle": {
    message: "Alterar o runtime para {runtime}?",
    description: "Confirm-dialog title for changing a service's runtime",
  },
  "services.runtimeConfirmBody": {
    message:
      "O serviço refaz o build com o runtime {runtime} e passa por um novo deploy. Isso inicia um novo deploy.",
    description: "Confirm-dialog body for changing a service's runtime",
  },
  "services.runtimeConfirmAction": {
    message: "Alterar runtime",
    description: "Confirm-dialog action for changing a service's runtime",
  },
  "services.runtimeSaved": {
    message: "Runtime atualizado. Um novo deploy foi iniciado.",
    description: "Toast on successful setRuntime mutation",
  },
  "services.runtimeError": {
    message: "Não foi possível alterar o runtime. Tente novamente.",
    description: "Toast on failed setRuntime mutation without a server message",
  },
  "services.maintenanceModeBannerTitle": {
    message: "O modo de manutenção está ativado",
    description: "Service-detail header banner title while in maintenance",
  },
  "services.maintenanceModeBannerBody": {
    message:
      "Os visitantes veem uma página de manutenção no lugar deste serviço. Os pods continuam em execução — desative o modo de manutenção em Configurações para retomar o atendimento normal.",
    description: "Service-detail header banner body while in maintenance",
  },
  "services.deployTitle": {
    message: "Deploy",
    description: "Cron job Settings tab: Deploy section title (Render parity)",
  },
  "services.deployDescription": {
    message: "Como este Cron Job é executado.",
    description: "Cron job Settings tab: Deploy section description",
  },
  "services.deploySuccess": {
    message: "Configurações do Cron Job salvas.",
    description: "Toast after updateCronJob succeeds",
  },
  "services.cronScheduleSaved": {
    message: "A próxima execução segue o novo agendamento.",
    description:
      "Toast description after saving a cron job's schedule alone: the next run follows it, nothing deploys (w4/593)",
  },
  "services.cronCommandSaved": {
    message:
      "Um deploy está aplicando o novo comando; as execuções seguintes o usam.",
    description:
      "Toast description after saving a cron job's command: the save rolls a deploy that applies it (w4/593)",
  },
  "services.deployError": {
    message:
      "Não foi possível salvar as configurações do Cron Job. Tente novamente.",
    description: "Toast after updateCronJob fails",
  },
  "services.deployRefreshFailed": {
    message:
      "Configurações do Cron Job salvas, mas esta página não conseguiu atualizar. Tente novamente ou recarregue para ver as configurações salvas.",
    description:
      "Warning after a cron save committed but its authoritative re-read failed; the save must not be retried or reported as failed (w4/276)",
  },
  "services.deployRefreshPending": {
    message:
      "Configurações salvas — aguardando uma leitura atualizada antes de novas edições.",
    description:
      "Reason shown while cron rows stay unwritable after a committed save whose refresh failed (w4/276)",
  },
  "services.deployScheduleLabel": {
    message: "Agendamento",
    description: "Cron job Settings tab: Deploy section schedule field label",
  },
  "services.deployScheduleHint": {
    message:
      "Executa este comando neste agendamento (crontab de 5 campos, em UTC).",
    description: "Cron job Settings tab: Deploy section schedule help text",
  },
  "services.deploySchedulePreview": {
    message: "{description} · executa em UTC",
    description:
      "Cron job Settings tab: live human-readable description of the schedule",
  },
  "services.deploySchedulePlaceholder": {
    message: "0 * * * *",
    description: "Cron job Deploy section: schedule input placeholder",
  },
  "services.deployScheduleEdit": {
    message: "Editar agendamento",
    description: "Accessible label for the cron schedule edit button",
  },
  "services.deployCommandEdit": {
    message: "Editar comando",
    description: "Accessible label for the cron command edit button",
  },
  "services.cronScheduleNeverFires": {
    message:
      "Este agendamento nunca dispara: nenhuma data corresponde ao dia do mês e ao mês dele (ex.: 31 2 seria 31 de fevereiro).",
    description:
      "Cron schedule validation error when the schedule parses but no date ever matches (w4/197)",
  },
  "services.deployScheduleError": {
    message: "Informe uma expressão cron válida de 5 campos, ex.: 0 * * * *.",
    description: "Cron job Deploy section: schedule validation error",
  },
  "services.deployScheduleRequired": {
    message: "O agendamento é obrigatório.",
    description: "Cron job Deploy section: schedule required validation error",
  },
  "services.deployCommandLabel": {
    message: "Comando",
    description: "Cron job Settings tab: Deploy section command field label",
  },
  "services.deployCommandPlaceholder": {
    message: "ex.: python script.py",
    description: "Cron job Deploy section: command input placeholder",
  },
  "services.deployCommandHint": {
    message:
      "Substitui o ponto de entrada padrão da imagem. Deixe em branco para executar o comando da própria imagem.",
    description: "Cron job Deploy section: command field help text",
  },
  "services.deployCommandNativeHint": {
    message:
      "O comando que o job executa a cada agendamento, como o comando de início do runtime dele.",
    description:
      "Cron job Deploy section: command field help text for a native-runtime cron, whose command is its start command (w8/m69)",
  },
  "services.deployCommandRequired": {
    message: "O comando é obrigatório.",
    description:
      "Cron job Deploy section: command required validation error for a native-runtime cron (w8/m69)",
  },
  "services.buildTitle": {
    message: "Build",
    description:
      "Settings tab: Build card title, Render's split Build section (w5/m52)",
  },
  "services.buildDeployNav": {
    message: "Build e deploy",
    description:
      "Settings section index: the one entry for the Build and Deploy cards, which the page renders as one section (Render names it Build & Deploy)",
  },
  "services.buildDescription": {
    message:
      "Configure como o bex faz o build do seu serviço a partir do seu repositório.",
    description: "Settings tab: Build card description",
  },
  "services.deploySectionTitle": {
    message: "Deploy",
    description:
      "Settings tab: Deploy card title, Render's split Deploy section (w5/m52)",
  },
  "services.deploySectionDescription": {
    message: "Configure como o bex faz o deploy e executa o seu serviço.",
    description: "Settings tab: Deploy card description",
  },
  "services.buildDeploySourceLabel": {
    message: "Origem",
    description: "Build & Deploy: repo field label",
  },
  "services.buildDeploySourceHint": {
    message:
      "O repositório Git a partir do qual este serviço faz o build. Escolha um dos seus repositórios conectados ou cole a URL de um repositório.",
    description: "Build & Deploy: Source (repo) field help text (w5/m54)",
  },
  "services.buildDeploySourcePlaceholder": {
    message: "https://github.com/owner/repo",
    description: "Build & Deploy: Source combobox input placeholder",
  },
  "services.buildDeploySourceEdit": {
    message: "Editar origem",
    description: "Accessible label for the Source edit-pencil button",
  },
  "services.buildDeploySourceConfirmTitle": {
    message: "Mudar a origem para {value}?",
    description: "Build & Deploy: Source-change confirm dialog title",
  },
  "services.buildDeployBranchLabel": {
    message: "Branch",
    description: "Build & Deploy: branch field label",
  },
  "services.buildDeployBranchHint": {
    message: "O branch do Git a ser compilado e implantado.",
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
    message: "Editar branch",
    description: "Build & Deploy: branch edit-button aria label",
  },
  "services.buildDeployBranchConfirmTitle": {
    message: "Mudar o branch para “{value}”?",
    description: "Build & Deploy: branch confirm-dialog title",
  },
  "services.buildDeployRootDirLabel": {
    message: "Diretório raiz",
    description: "Build & Deploy: root-directory field label",
  },
  "services.buildDeployRootDirOptional": {
    message: "Opcional",
    description:
      "Build & Deploy: badge next to the Root Directory label (Render parity)",
  },
  "services.buildDeployRootDirHint": {
    message:
      "Se definido, os builds são executados a partir deste subdiretório em vez da raiz do repositório. Alterações de código fora dele não disparam um deploy automático, a menos que correspondam a um caminho incluído dos filtros de build. Muito usado com monorepos.",
    description: "Build & Deploy: root-directory field help text",
  },
  "services.buildDeployConfirmRoot": {
    message: "a raiz do repositório",
    description:
      "Build & Deploy: mid-sentence phrase for the confirm dialog title when clearing rootDir to empty (a dedicated key, not a lowercased buildDeployRootDirEmpty, since that transform doesn't hold in every language)",
  },
  "services.buildDeployRootDirPlaceholder": {
    message: "ex.: backend",
    description: "Build & Deploy: root-directory input placeholder",
  },
  "services.buildDeployEdit": {
    message: "Editar diretório raiz",
    description: "Build & Deploy: accessible label for the edit-pencil button",
  },
  "services.buildDeployCancel": {
    message: "Cancelar",
    description: "Build & Deploy: root-directory inline-edit cancel button",
  },
  "services.buildDeployConfirmTitle": {
    message: "Mudar o diretório raiz para {value}?",
    description: "Build & Deploy: root-directory change confirm dialog title",
  },
  "services.buildDeployConfirmBody": {
    message:
      "O serviço refaz o build e o deploy, restritos ao novo diretório — as requisições em andamento terminam antes de as instâncias antigas serem substituídas.",
    description: "Build & Deploy: root-directory change confirm dialog body",
  },
  "services.buildDeployConfirmBodyStatic": {
    message:
      "Isso inicia um novo build a partir do novo diretório. Seu site continua servindo a versão publicada atual até o novo build ser publicado com sucesso.",
    description:
      "Build & Deploy: a live static site's root-directory change confirm body (atomic publish, no instances; w4/548)",
  },
  "services.buildDeployConfirmBodyNeverLive": {
    message:
      "O serviço faz o build e o deploy a partir do novo diretório. Nada está sendo servido ainda — ele entra no ar quando um deploy for bem-sucedido.",
    description:
      "Build & Deploy: root-directory change confirm body for a service that has never gone live, so no current version keeps serving (w4/548)",
  },
  "services.buildDeploySuccess": {
    message: "Diretório raiz atualizado.",
    description: "Toast after setRootDir succeeds",
  },
  "services.buildDeployError": {
    message: "Não foi possível atualizar o diretório raiz. Tente novamente.",
    description: "Toast after setRootDir fails",
  },
  "services.sourceUpdateSuccess": {
    message: "Origem atualizada.",
    description:
      "Toast after a repository, branch, or image source update saves without triggering a deploy (e.g. suspended service or unbuilt build source)",
  },
  "services.sourceUpdateDeploying": {
    message: "Origem atualizada. Iniciando um novo deploy.",
    description:
      "Toast after Update Source saves and immediately triggers a new deploy",
  },
  "services.sourceUpdateError": {
    message: "Não foi possível atualizar a origem. Tente novamente.",
    description:
      "Fallback toast when a source update fails without a server reason",
  },
  "services.sourceTitle": {
    message: "Origem",
    description: "Source card title",
  },
  "services.sourceRepoDescription": {
    message:
      "O repositório Git e o branch a partir dos quais este serviço faz o build.",
    description: "Source card description for a repository-backed service",
  },
  "services.sourceImageDescription": {
    message: "A imagem de contêiner pré-compilada que este serviço implanta.",
    description: "Source card description for an image-backed service",
  },
  "services.sourceImageLabel": {
    message: "Imagem",
    description: "Source card: container image path field label",
  },
  "services.sourceDialogDescription": {
    message:
      "Selecione um repositório Git, uma origem de build existente ou uma imagem Docker. Enviar inicia um novo deploy.",
    description:
      "Update Source dialog description for non-static services (Render parity)",
  },
  "services.sourceDialogDescriptionStatic": {
    message:
      "Selecione um repositório Git para este Static Site. Enviar inicia um novo deploy.",
    description:
      "Update Source dialog description for static sites (Render parity)",
  },
  "services.sourceMissing": {
    message: "Não configurada",
    description: "Source card fallback when no source value is available",
  },
  "services.sourceRepoAccessLostTitle": {
    message: "O app do GitHub da bex não consegue ler este repositório",
    description:
      "Settings → Source warning title: the GitHub App connected this service's repository and can no longer read it (repoAccessLost, w10/m490)",
  },
  "services.sourceRepoAccessLostBody": {
    message:
      "Os pushes não implantam mais este serviço. Se o repositório foi movido para outra conta do GitHub, instale o app do GitHub nessa conta; caso contrário, conceda novamente ao app acesso ao repositório.",
    description:
      "Settings → Source warning body: pushes stop reaching bex once the GitHub App loses the repository (moved to an account without the app, or the grant was revoked); what to do (w10/m490)",
  },
  "services.sourceEdit": {
    message: "Editar",
    description: "Button opening the Update Source dialog",
  },
  "services.sourceDialogTitle": {
    message: "Atualizar origem",
    description: "Update Source dialog title",
  },
  "services.sourcePickerLabel": {
    message: "Escolha uma origem",
    description: "Label above the source kind picker in Update Source",
  },
  "services.sourceBranchHint": {
    message: "Escolha um branch ou informe um que não esteja na lista.",
    description: "Branch picker free-text fallback hint",
  },
  "services.sourceBranchEmpty": {
    message: "Nenhum branch correspondente. Informe o nome de um branch.",
    description: "Empty state for the Update Source branch picker",
  },
  "services.sourceTabBuildSource": {
    message: "Origem de build",
    description: "Update Source tab: build the service from a build source",
  },
  "services.sourceBuildSourceDescription": {
    message:
      "A origem de build que este serviço implanta. O build dela é compartilhado com todos os serviços vinculados a ela.",
    description:
      "Source card description for a service linked to a build source",
  },
  "services.sourceBuildSourceLabel": {
    message: "Origem de build",
    description: "Source card and Update Source: build source field label",
  },
  "services.sourceBuildSourceBuildsFrom": {
    message: "Build a partir de",
    description:
      "Source card: the repository or image the linked build source builds from",
  },
  "services.sourceBuildSourceEmpty": {
    message:
      "Este workspace ainda não tem origens de build. Crie uma pela API ou pelo MCP e vincule serviços a ela aqui.",
    description: "Update Source build source tab: empty state",
  },
  "services.sourceBuildSourceLoadError": {
    message:
      "Não foi possível carregar as origens de build deste workspace. Feche a caixa de diálogo e tente novamente.",
    description: "Update Source build source tab: the list failed to load",
  },
  "services.sourceBuildSourcePlaceholder": {
    message: "Selecione uma origem de build",
    description: "Update Source build source picker placeholder",
  },
  "services.sourceBuildSourceStartHint": {
    message:
      "Esta origem de build usa um runtime nativo, então o serviço precisa do próprio comando de início.",
    description:
      "Update Source: why a native-runtime build source needs a start command",
  },
  "services.sourceBuildCommandOptionalHint": {
    message:
      "Opcional. Deixe em branco para publicar diretamente os arquivos do repositório.",
    description:
      "Update Source static-site Build Command helper: build command is optional",
  },
  "services.sourcePublishDirectoryRequired": {
    message: "Informe um diretório de publicação.",
    description:
      "Update Source static-site Publish Directory validation error when blank",
  },
  "services.sourceDeploy": {
    message: "Fazer deploy",
    description:
      "Primary submit button in Update Source for Git and Build Source tabs",
  },
  "services.sourceDeploying": {
    message: "Fazendo deploy",
    description:
      "In-flight submit button label in Update Source for Git and Build Source tabs",
  },
  "services.sourceConnect": {
    message: "Conectar",
    description:
      "Primary submit button in Update Source for the Existing Image tab",
  },
  "services.sourceConnecting": {
    message: "Conectando",
    description:
      "In-flight submit button label in Update Source for the Existing Image tab",
  },
  "services.startCommandLabel": {
    message: "Comando de início",
    description: "Build & Deploy: native-runtime start-command label",
  },
  "services.dockerCommandLabel": {
    message: "Comando do Docker",
    description: "Build & Deploy: Docker CMD override label (Render wording)",
  },
  "services.startCommandHint": {
    message: "O comando que inicia este serviço após um build bem-sucedido.",
    description: "Build & Deploy: native start-command help text",
  },
  "services.dockerCommandHint": {
    message:
      "Substitui o CMD do Dockerfile. Deixe em branco para usar o comando padrão da imagem.",
    description: "Build & Deploy: Docker Command help text",
  },
  "services.startCommandConfirmEmpty": {
    message: "o comando padrão",
    description: "Build & Deploy: empty command phrase in confirmation title",
  },
  "services.startCommandPlaceholder": {
    message: "ex.: npm start",
    description: "Build & Deploy: start-command input placeholder",
  },
  "services.startCommandEdit": {
    message: "Editar comando de início",
    description: "Build & Deploy: accessible command edit button label",
  },
  "services.dockerCommandEdit": {
    message: "Editar comando do Docker",
    description: "Build & Deploy: accessible Docker Command edit button label",
  },
  "services.dockerCommandPlaceholder": {
    message: "ex.: bin/server",
    description: "Build & Deploy: Docker Command input placeholder",
  },
  "services.startCommandConfirmTitle": {
    message: "Mudar o comando de início para {value}?",
    description: "Build & Deploy: command-change confirmation title",
  },
  "services.dockerCommandConfirmTitle": {
    message: "Mudar o comando do Docker para {value}?",
    description: "Build & Deploy: Docker Command confirmation title",
  },
  "services.startCommandConfirmBody": {
    message:
      "O serviço passa por um novo deploy com o novo comando. As requisições em andamento terminam antes de as instâncias antigas serem substituídas.",
    description: "Build & Deploy: command-change confirmation body",
  },
  "services.startCommandConfirmBodyNeverLive": {
    message:
      "O serviço passa por deploy com o novo comando. Nada está sendo servido ainda — ele entra no ar quando um deploy for bem-sucedido.",
    description:
      "Deploy: command change confirm body for a service that has never gone live (w4/548)",
  },
  "services.startCommandSuccess": {
    message: "Comando de início atualizado.",
    description: "Toast after setStartCommand succeeds",
  },
  "services.startCommandError": {
    message: "Não foi possível atualizar o comando de início. Tente novamente.",
    description: "Toast after setStartCommand fails",
  },
  "services.dockerCommandSuccess": {
    message: "Comando do Docker atualizado.",
    description:
      "Toast after a Dockerfile or Existing Image service's Docker Command is saved",
  },
  "services.dockerCommandError": {
    message: "Não foi possível atualizar o comando do Docker. Tente novamente.",
    description:
      "Toast when saving a Dockerfile or Existing Image service's Docker Command fails",
  },
  "services.buildCommandLabel": {
    message: "Comando de build",
    description:
      "Build & Deploy: build-command field label (all service types' settings + the New Service form)",
  },
  "services.buildCommandHint": {
    message:
      "O comando que faz o build do seu serviço (ex.: npm run build). Deixe em branco para usar o padrão do runtime.",
    description:
      "Build & Deploy: build-command help text — shown for every service type, so it must not assume static-site output",
  },
  "services.buildCommandConfirmEmpty": {
    message: "Padrão do runtime",
    description:
      "Build & Deploy: empty build-command phrase in confirmation title",
  },
  "services.buildCommandPlaceholder": {
    message: "npm run build",
    description: "Build & Deploy: build-command input placeholder",
  },
  "services.buildCommandEdit": {
    message: "Editar comando de build",
    description: "Build & Deploy: accessible build-command edit button label",
  },
  "services.buildCommandConfirmTitle": {
    message: "Mudar o comando de build para {value}?",
    description: "Build & Deploy: build-command change confirmation title",
  },
  "services.buildCommandConfirmBody": {
    message:
      "O serviço passará por um novo deploy com o novo comando de build. As requisições existentes serão concluídas antes de as instâncias antigas serem substituídas.",
    description: "Build & Deploy: build-command change confirmation body",
  },
  "services.buildCommandConfirmBodyNeverLive": {
    message:
      "O serviço faz o build e o deploy com o novo comando de build. Nada está sendo servido ainda — ele entra no ar quando um deploy for bem-sucedido.",
    description:
      "Build & Deploy: build-command change confirm body for a service that has never gone live (w4/548)",
  },
  "services.buildCommandConfirmBodyStatic": {
    message:
      "Isso inicia um novo build com o novo comando de build. Seu site continua servindo a versão publicada atual até o novo build ser publicado com sucesso.",
    description:
      "Build & Deploy: static-site build-command change confirmation body (atomic publish, no instances)",
  },
  "services.staticBuildConfirmBodyNeverLive": {
    message:
      "Isso inicia um build com a alteração. Seu site ainda não foi publicado — ele entra no ar quando este build for bem-sucedido.",
    description:
      "Build & Deploy: a static site's build-command or root-directory change confirm body when the site has never been published (w4/548)",
  },
  "services.buildCommandSuccess": {
    message: "Comando de build atualizado.",
    description: "Toast after setBuildCommand succeeds",
  },
  "services.buildCommandError": {
    message: "Não foi possível atualizar o comando de build. Tente novamente.",
    description: "Toast after setBuildCommand fails",
  },
  "services.dockerfilePathLabel": {
    message: "Caminho do Dockerfile",
    description: "Build & Deploy: Dockerfile-path field label",
  },
  "services.dockerfilePathHint": {
    message:
      "Caminho do Dockerfile, relativo ao diretório raiz. Deixe em branco para usar Dockerfile.",
    description: "Build & Deploy: Dockerfile-path help text",
  },
  "services.dockerfilePathConfirmEmpty": {
    message: "o Dockerfile padrão",
    description: "Build & Deploy: empty Dockerfile-path confirmation phrase",
  },
  "services.dockerfilePathPlaceholder": {
    message: "ex.: docker/Dockerfile.prod",
    description: "Build & Deploy: Dockerfile-path input placeholder",
  },
  "services.dockerfilePathEdit": {
    message: "Editar caminho do Dockerfile",
    description: "Build & Deploy: accessible Dockerfile-path edit label",
  },
  "services.dockerfilePathConfirmTitle": {
    message: "Mudar o caminho do Dockerfile para {value}?",
    description: "Build & Deploy: Dockerfile-path confirmation title",
  },
  "services.dockerfilePathConfirmBody": {
    message:
      "O serviço refaz o build a partir do Dockerfile selecionado e implanta a imagem resultante.",
    description: "Build & Deploy: Dockerfile-path confirmation body",
  },
  "services.dockerfilePathSuccess": {
    message: "Caminho do Dockerfile atualizado.",
    description: "Toast after setDockerfilePath succeeds",
  },
  "services.dockerfilePathError": {
    message:
      "Não foi possível atualizar o caminho do Dockerfile. Tente novamente.",
    description: "Toast after setDockerfilePath fails",
  },
  "services.dockerContextLabel": {
    message: "Diretório de contexto de build do Docker",
    description: "Build & Deploy: Docker build context field label (w8/m128)",
  },
  "services.dockerContextHint": {
    message:
      "Contexto de build, relativo ao diretório raiz. Deixe em branco para usar o diretório raiz.",
    description: "Build & Deploy: Docker build context help text",
  },
  "services.dockerContextConfirmEmpty": {
    message: "o diretório raiz",
    description:
      "Build & Deploy: empty Docker build context confirmation phrase",
  },
  "services.dockerContextPlaceholder": {
    message: "ex.: .",
    description: "Build & Deploy: Docker build context input placeholder",
  },
  "services.dockerContextEdit": {
    message: "Editar diretório de contexto de build do Docker",
    description: "Build & Deploy: accessible Docker build context edit label",
  },
  "services.dockerContextConfirmTitle": {
    message: "Mudar o diretório de contexto de build do Docker para {value}?",
    description: "Build & Deploy: Docker build context confirmation title",
  },
  "services.dockerContextConfirmBody": {
    message:
      "O serviço refaz o build com o novo contexto de build e implanta a imagem resultante.",
    description: "Build & Deploy: Docker build context confirmation body",
  },
  "services.dockerContextSuccess": {
    message: "Diretório de contexto de build do Docker atualizado.",
    description: "Toast after setDockerContext succeeds",
  },
  "services.dockerContextError": {
    message:
      "Não foi possível atualizar o diretório de contexto de build do Docker. Tente novamente.",
    description: "Toast after setDockerContext fails",
  },
  "services.buildFilterLabel": {
    message: "Filtros de build",
    description: "Build & Deploy: label for the build-filters editor",
  },
  "services.buildFilterHint": {
    message:
      "Dispare um deploy somente quando um git push alterar arquivos correspondentes. Os caminhos são padrões glob relativos à raiz do repositório, mesmo com um diretório raiz definido (ex.: src/**, **/*.md).",
    description: "Build & Deploy: help text for the build-filters editor",
  },
  "services.buildFilterIncludedTitle": {
    message: "Caminhos incluídos",
    description: "Build & Deploy: title for the included-paths list",
  },
  "services.buildFilterIncludedHint": {
    message:
      "Um push dispara deploy somente quando um arquivo alterado corresponde a um destes, mesmo fora do diretório raiz. Vazio significa que todos os caminhos dentro do diretório raiz estão incluídos.",
    description: "Build & Deploy: help text for the included-paths list",
  },
  "services.buildFilterIncludedPlaceholder": {
    message: "ex.: src/**",
    description: "Build & Deploy: placeholder for an included-path input",
  },
  "services.buildFilterAddIncluded": {
    message: "Adicionar caminho incluído",
    description: "Build & Deploy: add-row button for the included-paths list",
  },
  "services.buildFilterRemoveIncluded": {
    message: "Remover caminho incluído",
    description: "Build & Deploy: remove-row label for an included path",
  },
  "services.buildFilterIgnoredTitle": {
    message: "Caminhos ignorados",
    description: "Build & Deploy: title for the ignored-paths list",
  },
  "services.buildFilterIgnoredHint": {
    message:
      "Um arquivo alterado que corresponda a um destes nunca dispara um deploy, mesmo que também corresponda a um caminho incluído.",
    description: "Build & Deploy: help text for the ignored-paths list",
  },
  "services.buildFilterIgnoredPlaceholder": {
    message: "ex.: docs/**",
    description: "Build & Deploy: placeholder for an ignored-path input",
  },
  "services.buildFilterAddIgnored": {
    message: "Adicionar caminho ignorado",
    description: "Build & Deploy: add-row button for the ignored-paths list",
  },
  "services.buildFilterRemoveIgnored": {
    message: "Remover caminho ignorado",
    description: "Build & Deploy: remove-row label for an ignored path",
  },
  "services.buildPathTooLong": {
    message: "No máximo 512 bytes",
    description:
      "Inline error under a Root Directory, Dockerfile Path, Docker Build Context or Build Filter pattern over the server's 512-byte limit; a Chinese character is 3 bytes (w21/045)",
  },
  "services.buildPathControlChar": {
    message: "Não pode conter um caractere de controle nem uma tabulação",
    description:
      "Inline error under a build path or Build Filter pattern with a control character (w21/045)",
  },
  "services.buildPathBackslash": {
    message: "Use / como separador de caminho, não \\",
    description:
      "Inline error under a build path or Build Filter pattern with a backslash (w21/045)",
  },
  "services.buildPathAbsolute": {
    message: "Deve ser relativo ao repositório, sem começar com /",
    description:
      "Inline error under a build path or Build Filter pattern that starts with / (w21/045)",
  },
  "services.buildPathDotDot": {
    message: "Deve permanecer dentro do repositório: sem segmento ..",
    description:
      "Inline error under a build path or Build Filter pattern with a .. segment (w21/045)",
  },
  "services.buildPathEmptySegment": {
    message: "Não pode ter segmento vazio: sem // e sem / no final",
    description:
      "Inline error under a Dockerfile Path or Docker Build Context with an empty path segment; the Root Directory normalizes these away (w21/045)",
  },
  "services.buildFilterUnclosedClass": {
    message: "Um [ nunca é fechado por um ]",
    description:
      "Inline error under a Build Filter glob with an unclosed character class (w21/045)",
  },
  "services.healthCheckPathSlash": {
    message: "Deve começar com /, ou ficar vazio para verificar apenas a porta",
    description:
      "Inline error under a Health Check Path that does not start with / (w21/045)",
  },
  "services.buildFilterSave": {
    message: "Salvar filtros de build",
    description: "Build & Deploy: save button for the build-filters editor",
  },
  "services.buildFilterSuccess": {
    message: "Filtros de build atualizados.",
    description: "Toast after setBuildFilter succeeds",
  },
  "services.buildFilterError": {
    message: "Não foi possível atualizar os filtros de build. Tente novamente.",
    description: "Toast after setBuildFilter fails",
  },
  "services.preDeployLabel": {
    message: "Comando de pré-deploy",
    description: "Build & Deploy: label for the pre-deploy command field",
  },
  "services.preDeployHint": {
    message:
      "Executa uma vez na nova imagem antes de ela receber tráfego (ex.: uma migração de banco de dados). Uma saída diferente de zero faz o deploy falhar e mantém a versão anterior no ar.",
    description: "Build & Deploy: help text for the pre-deploy command field",
  },
  "services.preDeployPaidOnly": {
    message: "Os comandos de pré-deploy estão disponíveis em planos pagos.",
    description:
      "Build & Deploy: why the pre-deploy command field is disabled on a Free service (Render: paid web services, private services and background workers; w8/m131)",
  },
  "services.preDeployPlaceholder": {
    message: "ex.: npm run migrate",
    description: "Build & Deploy: placeholder for the pre-deploy command input",
  },
  "services.preDeployEdit": {
    message: "Editar comando de pré-deploy",
    description:
      "Build & Deploy: accessible label for the pre-deploy edit-pencil button",
  },
  "services.healthCheckPathConfirmTitle": {
    message: "Mudar o caminho do health check para {value}?",
    description:
      "Settings: Health Check Path change confirmation title (w4/616)",
  },
  "services.healthCheckPathConfirmEmpty": {
    message: "uma verificação TCP",
    description:
      "Settings: empty Health Check Path phrase in its confirmation title",
  },
  "services.maxShutdownDelayConfirmTitle": {
    message: "Mudar o atraso máximo de desligamento para {value} segundos?",
    description:
      "Settings: Max shutdown delay change confirmation title (w4/616)",
  },
  "services.preDeployConfirmTitle": {
    message: "Mudar o comando de pré-deploy para {value}?",
    description:
      "Settings: Pre-Deploy Command change confirmation title (w4/616)",
  },
  "services.preDeployConfirmEmpty": {
    message: "nenhum comando de pré-deploy",
    description:
      "Settings: empty Pre-Deploy Command phrase in its confirmation title",
  },
  "services.settingRedeployConfirmBody": {
    message:
      "O serviço passa por um novo deploy com a nova configuração. As requisições em andamento terminam antes de as instâncias antigas serem substituídas.",
    description:
      "Settings: confirmation body for a setting whose save starts a deploy (w4/616)",
  },
  "services.settingRedeployConfirmBodyNeverLive": {
    message:
      "O serviço passa por deploy com a nova configuração. Nada está sendo servido ainda — ele entra no ar quando um deploy for bem-sucedido.",
    description:
      "Settings: deploying-setting confirmation body for a service that has never gone live (w4/616)",
  },
  "services.preDeployConfirmBody": {
    message:
      "O serviço passa por um novo deploy, e o novo comando é executado antes de a nova versão entrar no ar. As requisições em andamento terminam antes de as instâncias antigas serem substituídas.",
    description:
      "Settings: Pre-Deploy Command change confirmation body (w4/616)",
  },
  "services.preDeployConfirmBodyNeverLive": {
    message:
      "O serviço passa por deploy, e o novo comando é executado antes de a versão entrar no ar. Nada está sendo servido ainda — ele entra no ar quando um deploy for bem-sucedido.",
    description:
      "Settings: Pre-Deploy Command confirmation body for a service that has never gone live (w4/616)",
  },
  "services.preDeploySuccess": {
    message: "Comando de pré-deploy atualizado. Um novo deploy foi iniciado.",
    description: "Toast after setPreDeployCommand succeeds",
  },
  "services.preDeployError": {
    message:
      "Não foi possível atualizar o comando de pré-deploy. Tente novamente.",
    description: "Toast after setPreDeployCommand fails",
  },
  "services.autoDeployLabel": {
    message: "Deploy automático",
    description: "Build & Deploy: label for the auto-deploy toggle",
  },
  "services.autoDeployOnCommit": {
    message: "A cada commit",
    description:
      "Auto-Deploy select option: redeploy on every matching git push (Render's autoDeployTrigger 'commit', w5/m53)",
  },
  "services.autoDeployAfterChecks": {
    message: "Após a aprovação nas verificações de CI",
    description:
      "Auto-Deploy select option: redeploy a pushed commit once all its GitHub checks pass (Render's autoDeployTrigger 'checksPass', w1/m181)",
  },
  "services.autoDeployOff": {
    message: "Desativado",
    description:
      "Auto-Deploy select option: never redeploy on push (Render's autoDeployTrigger 'off')",
  },
  "services.autoDeployEdit": {
    message: "Editar deploy automático",
    description:
      "Accessible label for the Auto-Deploy select's edit-pencil button",
  },
  "services.autoDeployViaGitHub": {
    message:
      "Um push no branch monitorado dispara um novo deploy automaticamente pelo app do GitHub.",
    description:
      "Build & Deploy: source indicator when the connected GitHub App installation grants THIS repo (pushDeliveryMethod github_app)",
  },
  "services.autoDeployViaWebhook": {
    message:
      "Um push só dispara um novo deploy se o webhook git manual do repositório estiver configurado com o BEX_WEBHOOK_SECRET desta instalação.",
    description:
      "Build & Deploy: source indicator when the GitHub App cannot deliver a push for this repo — no connection, another git host, or a repo the installation does not grant (pushDeliveryMethod manual_webhook)",
  },
  "services.autoDeployUnavailable": {
    message:
      "O deploy automático não pode ser executado para este repositório: o GitHub não envia os pushes dele ao bex. Conecte o repositório pelo app do GitHub ou dispare deploys com o Deploy Hook.",
    description:
      "Build & Deploy: source indicator when no push can reach bex for this repo — the GitHub App does not cover it and this install rejects the shared manual webhook key (pushDeliveryMethod unavailable, w4/483)",
  },
  "services.autoDeployDeliveryUnknown": {
    message:
      "Verificando como um push no branch monitorado chega ao bex… Se isto persistir, o GitHub está inacessível e o método de entrega não pode ser confirmado.",
    description:
      "Build & Deploy: source indicator while the per-repo push-delivery check is loading or returned unknown (GitHub unreachable) — states the uncertainty instead of naming a mechanism (w6/m99)",
  },
  "services.autoDeployOnSuccess": {
    message: "Deploy automático ativado.",
    description: "Toast after enabling auto-deploy",
  },
  "services.autoDeployChecksSuccess": {
    message:
      "O deploy automático agora aguarda a aprovação nas verificações de CI.",
    description: "Toast after choosing After CI Checks Pass",
  },
  "services.autoDeployOffSuccess": {
    message: "Deploy automático desativado.",
    description: "Toast after disabling auto-deploy",
  },
  "services.autoDeployError": {
    message: "Não foi possível alterar o deploy automático. Tente novamente.",
    description: "Toast after setAutoDeploy fails",
  },
  "services.deployHookTitle": {
    message: "Deploy Hook",
    description: "Settings tab: secret Deploy Hook section title",
  },
  "services.deployHookDescription": {
    message: "Dispare um deploy a partir do CI com uma única URL secreta.",
    description: "Settings tab: Deploy Hook section description",
  },
  "services.deployHookURLLabel": {
    message: "URL do Deploy Hook",
    description: "Accessible label for the secret Deploy Hook URL field",
  },
  "services.deployHookReveal": {
    message: "Mostrar URL do Deploy Hook",
    description: "Accessible label for the reveal-secret button",
  },
  "services.deployHookHide": {
    message: "Ocultar URL do Deploy Hook",
    description: "Accessible label for the hide-secret button",
  },
  "services.deployHookCopy": {
    message: "Copiar URL do Deploy Hook",
    description: "Accessible label for the Deploy Hook copy button",
  },
  "services.deployHookCopied": {
    message: "URL do Deploy Hook copiada.",
    description: "Toast after copying the Deploy Hook URL",
  },
  "services.deployHookCopyError": {
    message: "Não foi possível copiar a URL do Deploy Hook.",
    description: "Toast after Deploy Hook URL clipboard failure",
  },
  "services.deployHookSecretHint": {
    message:
      "Mantenha esta URL em segredo. Quem a tiver pode fazer deploy deste serviço sem uma chave de API.",
    description: "Security warning below the Deploy Hook URL",
  },
  "services.deployHookRegenerate": {
    message: "Gerar hook novamente",
    description: "Deploy Hook rotation button",
  },
  "services.deployHookRegenerateTitle": {
    message: "Gerar o Deploy Hook novamente?",
    description: "Deploy Hook rotation confirmation title",
  },
  "services.deployHookRegenerateWarning": {
    message:
      "A URL atual deixará de funcionar imediatamente. Atualize todos os sistemas de CI, cron jobs e integrações que a usam.",
    description: "Deploy Hook rotation confirmation warning",
  },
  "services.deployHookCancel": {
    message: "Cancelar",
    description: "Deploy Hook rotation confirmation cancel button",
  },
  "services.deployHookRegenerateConfirm": {
    message: "Gerar novamente",
    description: "Deploy Hook rotation confirmation action",
  },
  "services.deployHookRegenerated": {
    message: "Deploy Hook gerado novamente. A URL antiga não funciona mais.",
    description: "Toast after successful Deploy Hook rotation",
  },
  "services.deployHookRegenerateError": {
    message: "Não foi possível gerar o Deploy Hook novamente. Tente novamente.",
    description: "Toast after Deploy Hook rotation fails",
  },
  "services.deployHookLoadError": {
    message: "Não foi possível carregar a URL do Deploy Hook.",
    description: "Deploy Hook section query error",
  },
  "services.colType": {
    message: "Tipo",
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
      "O diretório de saída publicado e as regras de borda aplicadas ao servir o site.",
    description: "Static Site settings section description",
  },
  "services.staticEdgeRulesHint": {
    message:
      "Redirecionamentos/reescritas e cabeçalhos de resposta têm suas próprias páginas:",
    description:
      "Settings hint pointing at the dedicated Redirects/Rewrites + Headers pages",
  },
  "services.staticCancel": {
    message: "Cancelar",
    description: "Cancel an inline edit",
  },
  "services.publishPathEscapes": {
    message:
      "Este caminho sai do repositório. Um diretório de publicação é resolvido a partir do diretório raiz, então remova os segmentos “..”.",
    description:
      "Inline error under a repo-backed static site's publish directory whose .. segments leave the checkout (w21/050)",
  },
  "services.publishPathControlChar": {
    message: "Não pode conter um caractere de controle nem uma quebra de linha",
    description:
      "Inline error under a static site's publish directory with a control character (w21/050)",
  },
  "services.publishPathLabel": {
    message: "Diretório de publicação",
    description: "Label for a static site's publishPath",
  },
  "services.publishPathPlaceholder": {
    message: "dist",
    description: "Placeholder for the publish directory input",
  },
  "services.publishPathEdit": {
    message: "Editar diretório de publicação",
    description:
      "Accessible label for the publish-directory edit-pencil button",
  },
  "services.publishPathHint": {
    message:
      "O diretório de saída do build servido como a raiz do site (ex.: dist, build, public). Alterá-lo publica o site novamente.",
    description: "Help text under the publish directory field",
  },
  "services.publishPathSaved": {
    message: "Diretório de publicação atualizado",
    description: "Toast after saving a static site's publishPath",
  },
  "services.publishPathRepublishNote": {
    message: "O site será publicado novamente em instantes.",
    description: "Toast description after changing publishPath",
  },
  "services.publishPathError": {
    message: "Não foi possível atualizar o diretório de publicação",
    description: "Error toast for a failed publishPath change",
  },
  "services.routesTitle": {
    message: "Redirecionamentos e reescritas",
    description: "Title for the static-site routes editor",
  },
  "services.routesHint": {
    message:
      "As regras são avaliadas em ordem, e a primeira correspondência vence. Um redirecionamento retorna 301; uma reescrita serve outro caminho (um fallback de SPA reescreve /* para /index.html). Um segmento :name captura um segmento de caminho, ex.: /blog/posts/:postid → /blog/:postid.",
    description: "Help text for the routes editor",
  },
  "services.routeAdd": {
    message: "Adicionar regra",
    description: "Add a route rule",
  },
  "services.routeType": {
    message: "Tipo",
    description: "Route type column",
  },
  "services.routeSource": {
    message: "Origem",
    description: "Route source-path column",
  },
  "services.routeDestination": {
    message: "Destino",
    description: "Route destination-path column",
  },
  "services.routeRewrite": {
    message: "Reescrita",
    description: "Route type: rewrite",
  },
  "services.routeRedirect": {
    message: "Redirecionamento",
    description: "Route type: redirect",
  },
  "services.routeTypeAria": {
    message: "Tipo da regra {n}",
    description:
      "A route rule's type select (aria-label), named by its row as Remove rule {n} is (w4/541)",
  },
  "services.routeSourceAria": {
    message: "Origem da regra {n}",
    description:
      "A route rule's source field (aria-label), named by its row (w4/541)",
  },
  "services.routeDestinationAria": {
    message: "Destino da regra {n}",
    description:
      "A route rule's destination field (aria-label), named by its row (w4/541)",
  },
  "services.routeRemove": {
    message: "Remover regra {n}",
    description: "Remove a route rule (aria-label)",
  },
  "services.routesSave": {
    message: "Salvar rotas",
    description: "Save the routes list",
  },
  "services.staticRuleRequired": {
    message: "Obrigatório",
    description: "Inline error under an empty static-site rule field (w4/145)",
  },
  "services.staticRulePathSlash": {
    message: "Deve ser um caminho começando com /",
    description:
      "Inline error under a static-site rule path that doesn't start with / (w4/145)",
  },
  "services.staticRuleExternalHost": {
    message:
      "Use uma URL http(s) simples com um host fixo, ex.: https://example.org/new",
    description:
      "Inline error under a redirect destination that is an absolute URL without a literal host (w8/162)",
  },
  "services.staticRuleLocalPath": {
    message: "Deve ser um caminho neste site, não //host",
    description:
      "Inline error under a redirect/rewrite destination that is a network-path reference (w4/145)",
  },
  "services.staticRuleDestinationPathOrUrl": {
    message: "Deve ser um caminho ou uma URL http(s)",
    description:
      "Inline error under a redirect/rewrite destination whose first segment reads as a scheme other than http(s), such as javascript: (w33/007)",
  },
  "services.staticRuleFixRows": {
    message: "Corrija as linhas marcadas para salvar.",
    description:
      "Note beside a disabled Save while a static-site rule row is invalid (w4/145)",
  },
  "services.staticRuleEmptyRow": {
    message: "Preencha ou remova a linha vazia para salvar.",
    description:
      "Note beside a disabled Save while a static-site rule row is still completely empty (w4/145)",
  },
  "services.staticRuleRow": {
    message: "Linha {row}:",
    description:
      "Prefix naming the 1-based rule row in a server refusal, replacing its zero-based wire index (w4/145)",
  },
  "services.staticRulePathTooLong": {
    message: "No máximo 2048 bytes",
    description:
      "Inline error under an over-long static-site rule path; the server counts UTF-8 bytes, so a Chinese character is 3 (w4/402, w21/043)",
  },
  "services.staticRuleSourceRepeatsPlaceholder": {
    message: "Cada :name pode aparecer apenas uma vez na origem",
    description:
      "Inline error under a redirect/rewrite source that declares the same :name placeholder twice (w21/043)",
  },
  "services.staticRuleUndeclaredPlaceholder": {
    message: "Usa um :name que a origem não declara",
    description:
      "Inline error under a redirect/rewrite destination whose :name placeholder (other than :splat) the source never declares (w21/043)",
  },
  "services.staticRuleHeaderNameTooLong": {
    message: "No máximo 256 caracteres",
    description: "Inline error under an over-long header name (w4/402)",
  },
  "services.staticRuleHeaderNameInvalid": {
    message: "Não é um nome de cabeçalho HTTP válido",
    description:
      "Inline error under a header name with characters a header name can't hold (w4/402)",
  },
  "services.staticRuleHeaderNameReserved": {
    message:
      "Este cabeçalho é definido pelo servidor e não pode ser substituído",
    description:
      "Inline error under a reserved header name such as Content-Length (w4/402)",
  },
  "services.staticRuleHeaderValueTooLong": {
    message: "No máximo 4096 bytes",
    description: "Inline error under an over-long header value (w4/402)",
  },
  "services.staticRuleHeaderValueInvalid": {
    message: "Contém um caractere que um valor de cabeçalho não pode ter",
    description:
      "Inline error under a header value with a control character (w4/402)",
  },
  "services.staticRuleCookieDomain": {
    message:
      "Set-Cookie não pode definir um Domain em hospedagem compartilhada",
    description:
      "Inline error under a Set-Cookie value carrying a Domain attribute (w4/402)",
  },
  "services.staticRoutesSaved": {
    message: "Rotas atualizadas",
    description: "Toast after saving routes",
  },
  "services.staticRoutesError": {
    message: "Não foi possível atualizar as rotas",
    description: "Error toast for a failed routes save",
  },
  "services.headersTitle": {
    message: "Cabeçalhos de resposta",
    description: "Title for the static-site custom-headers editor",
  },
  "services.headersHint": {
    message:
      "Cabeçalhos de resposta personalizados adicionados às respostas cujo caminho corresponde. Cabeçalhos de enquadramento do corpo e de conexão, como Content-Length, são definidos pelo servidor e não podem ser substituídos. Defina Content-Encoding apenas em arquivos que realmente estão armazenados compactados.",
    description: "Help text for the headers editor",
  },
  "services.headerAdd": {
    message: "Adicionar cabeçalho",
    description: "Add a custom header",
  },
  "services.headerPath": {
    message: "Caminho",
    description: "Header path column",
  },
  "services.headerName": {
    message: "Nome",
    description: "Header name column",
  },
  "services.headerValue": {
    message: "Valor",
    description: "Header value column",
  },
  "services.headerPathAria": {
    message: "Caminho do cabeçalho {n}",
    description:
      "A header rule's path field (aria-label), named by its row as Remove header {n} is (w4/541)",
  },
  "services.headerNameAria": {
    message: "Nome do cabeçalho {n}",
    description:
      "A header rule's name field (aria-label), named by its row (w4/541)",
  },
  "services.headerValueAria": {
    message: "Valor do cabeçalho {n}",
    description:
      "A header rule's value field (aria-label), named by its row (w4/541)",
  },
  "services.headerRemove": {
    message: "Remover cabeçalho {n}",
    description: "Remove a header (aria-label)",
  },
  "services.headersSave": {
    message: "Salvar cabeçalhos",
    description: "Save the headers list",
  },
  "services.staticHeadersSaved": {
    message: "Cabeçalhos atualizados",
    description: "Toast after saving headers",
  },
  "services.staticHeadersError": {
    message: "Não foi possível atualizar os cabeçalhos",
    description: "Error toast for a failed headers save",
  },
  "services.typeUnknown": {
    message: "Serviço",
    description: "Service-type badge fallback for an unrecognized type",
  },
  "services.cronRunColDuration": {
    message: "Duração",
    description:
      "Cron Runs page: the Duration column's header, and a row's duration for assistive tech (Render's DURATION, w3/m556)",
  },
  "services.cronRunStatusSucceeded": {
    message: "Execução bem-sucedida",
    description:
      "Cron Runs page: a successful run's status, in its icon's tooltip (Render's text, w3/m556)",
  },
  "services.cronRunStatusFailed": {
    message: "Execução com falha",
    description:
      "Cron Runs page: a failed run's status, in its icon's tooltip (Render's text, w3/m556)",
  },
  "services.cronRunStatusCanceled": {
    message: "Cancelada",
    description:
      "Cron Runs page: a canceled run's status in its icon's tooltip, and its line when no one is named as canceling it (Render's text, w3/m556)",
  },
  "services.cronRunStatusInProgress": {
    message: "Em andamento",
    description:
      "Cron Runs page: the status of a run that has not ended, started or not, in its icon's tooltip (Render's text, w3/m556)",
  },
  "services.cronRunStatusLastSuccessful": {
    message: "Última execução bem-sucedida",
    description:
      "Cron Runs page: the newest successful run's status, in its icon's tooltip (Render's text, w3/m556)",
  },
  "services.cronRunCancel": {
    message: "Cancelar",
    description:
      "Cron Runs page: a run in progress's Cancel button, which cancels it at once (Render's text, w3/m556)",
  },
  "services.cronRunCancelError": {
    message: "Não foi possível cancelar a execução do cron.",
    description: "Toast after cron run cancellation fails",
  },
  "services.cronTriggerRun": {
    message: "Disparar execução",
    description: "Button that fires an immediate cron run (Render parity)",
  },
  "services.cronTriggerError": {
    message:
      "Não foi possível disparar uma execução. Tente novamente em instantes.",
    description:
      "Toast, kept until dismissed, when Trigger Run fails and the server gave no message (Render's text, w3/m555)",
  },
  "services.cronTriggerForbidden": {
    message: "Você não tem permissão para executar este Cron Job.",
    description:
      "Toast, kept until dismissed, when Trigger Run is refused for permission (Render's text, w3/m555)",
  },
  "services.cronTriggerNoBuild": {
    message: "Nenhuma build bem-sucedida detectada. Veja {deploys}.",
    description:
      "Tooltip of a disabled Trigger Run on a repo-backed cron job whose build never succeeded; {deploys} is a link to its Deploys page, named by services.navDeploys (Render's text, w3/m555)",
  },
  "services.cronTriggerSuspended": {
    message: "Retome este Cron Job para disparar uma execução.",
    description:
      "Tooltip of a disabled Trigger Run while the cron job is suspended, shown in the Runs page's empty states (Render's text, w3/m556)",
  },
  "services.cronRunsLoadMore": {
    message: "Carregar mais",
    description: "Cron run history pagination button",
  },
  "services.cronRunsLoadingMore": {
    message: "Carregando…",
    description: "Cron run history pagination busy label",
  },
  "services.cronRunsLoadError": {
    message: "Não foi possível carregar as execuções do cron.",
    description: "Cron run history read error",
  },
  "services.cronRunsLoadFailed": {
    message:
      "Desculpe. Não foi possível carregar as execuções deste Cron Job. Atualize e tente novamente.",
    description:
      "Cron Runs page: the warning shown in place of the list when the runs could not be read (Render's text, w3/m556)",
  },
  "services.cronRunsSearchPlaceholder": {
    message: "Pesquisar execuções do Cron Job",
    description:
      "Cron Runs page: the search field's placeholder and name (Render's text, w3/m556)",
  },
  "services.cronRunsSearchClear": {
    message: "Limpar pesquisa",
    description:
      "Cron Runs page: the search field's clear button, for assistive tech (w3/m556)",
  },
  "services.cronRunsColRun": {
    message: "Execução",
    description:
      "Cron Runs page: the list's first column header, before the count (Render's RUN, w3/m556)",
  },
  "services.cronRunsColTrigger": {
    message: "Gatilho",
    description:
      "Cron Runs page: the Trigger column's header, and a row's trigger for assistive tech (Render's TRIGGER, w3/m556)",
  },
  "services.cronRunTitle": {
    message: "Execução do Cron Job",
    description: "Cron Runs page: a run row's title (Render's text, w3/m556)",
  },
  "services.cronRunStartedAgo": {
    message: "Executado {timestamp}",
    description:
      'Cron Runs page: when a run started, under its title; {timestamp} is elapsed text like "2 hours ago" (Render\'s "Run 21h ago", w3/m556)',
  },
  "services.cronRunBy": {
    message: "por {user}",
    description:
      'Cron Runs page: after when a run started, the email of the user who triggered it (Render\'s "by <user>", w3/m556)',
  },
  "services.cronRunByYou": {
    message: "por você",
    description:
      'Cron Runs page: after when a run started, when the viewer triggered it (Render\'s "by you", w3/m556)',
  },
  "services.cronRunCanceledBy": {
    message: "Cancelada por {user}",
    description:
      "Cron Runs page: a canceled run's line naming the email of the user who canceled it (Render's text, w3/m556)",
  },
  "services.cronRunCanceledByYou": {
    message: "Cancelada por você",
    description:
      "Cron Runs page: a canceled run's line when the viewer canceled it (Render's \"Canceled by you\", w3/m556)",
  },
  "services.cronRunTriggerScheduled": {
    message: "Agendada",
    description:
      "Cron Runs page: the trigger of a run the schedule started (Render's text, w3/m556)",
  },
  "services.cronRunTriggerManual": {
    message: "Manual",
    description:
      "Cron Runs page: the trigger of a run a user started (Render's text, w3/m556)",
  },
  "services.cronRunsNoMatchTitle": {
    message: "Nenhum resultado correspondente",
    description:
      "Cron Runs page: title when the search matches none of the loaded runs (Render's text, w3/m556)",
  },
  "services.cronRunsNoMatchBody": {
    message:
      "Não encontrou o que procura? Tente ajustar a pesquisa e verificar se há erros de digitação.",
    description:
      "Cron Runs page: body when the search matches none of the loaded runs (Render's text, w3/m556)",
  },
  "services.cronRunsResetSearch": {
    message: "Redefinir pesquisa",
    description:
      "Cron Runs page: the button that clears a search that matched nothing (Render's text, w3/m556)",
  },
  "services.cronRunsEmptyRecentTitle": {
    message: "Este Cron Job não foi executado nos últimos 90 dias.",
    description:
      "Cron Runs page: title when a cron job whose build succeeded lists no runs (Render's text, w3/m556)",
  },
  "services.cronRunsEmptyRecentBody": {
    message:
      "Você pode disparar uma execução manualmente ou aguardar a próxima execução agendada.",
    description:
      "Cron Runs page: body when a cron job whose build succeeded lists no runs, above Trigger Run (Render's text, w3/m556)",
  },
  "services.cronRunsEmptyNeverTitle": {
    message: "Este Cron Job ainda não foi executado.",
    description:
      "Cron Runs page: title when a cron job whose build never succeeded has no runs (Render's text, w3/m556)",
  },
  "services.cronRunsEmptyNeverBody": {
    message:
      "Ele será executado quando a primeira build terminar, ou você pode disparar uma execução manualmente.",
    description:
      "Cron Runs page: body when a cron job whose build never succeeded has no runs, above Trigger Run (Render's text, w3/m556)",
  },
  "services.cronRunsCount_one": {
    message: "{count} execução",
    description:
      "Cron Runs page: the list's count, for assistive tech, beside RUN (Render's \"30 runs\", w3/m556)",
  },
  "services.cronRunsCount_many": {
    message: "{count} de execuções",
    description:
      "Cron Runs page: the list's count, for assistive tech, beside RUN (Render's \"30 runs\", w3/m556)",
  },
  "services.cronRunsCount_other": {
    message: "{count} execuções",
    description:
      "Cron Runs page: the list's count, for assistive tech, beside RUN (Render's \"30 runs\", w3/m556)",
  },
  "services.suspendCardTitle": {
    message: "Suspender serviço",
    description: "Settings tab suspend section title",
  },
  "services.suspendCardDescription": {
    message:
      "Suspender o seu serviço o desliga e o impede de atender tráfego. Sua URL e seus certificados são mantidos, e você pode retomá-lo a qualquer momento.",
    description:
      "Settings tab suspend section description, for the types served at a public host",
  },
  "services.suspendCardDescriptionNoUrl": {
    message:
      "Suspender o seu serviço o desliga e o impede de ser executado. Você pode retomá-lo a qualquer momento.",
    description:
      "Settings tab suspend section description, for private services and background workers — neither has a public URL or certificates to keep",
  },
  "services.resumeCardTitle": {
    message: "Retomar serviço",
    description:
      "Settings tab resume section title (shown when service is suspended)",
  },
  "services.resumeCardDescription": {
    message:
      "Retomar o seu serviço o coloca de volta no ar e ele volta a atender tráfego.",
    description: "Settings tab resume section description",
  },
  "services.resumeCardDescriptionCrashLoop": {
    message:
      "Suspendemos este serviço porque ele falhou repetidamente ao iniciar e ficar saudável ao longo de 24 horas. Isso costuma ser causado por um erro de configuração, um bug no código da aplicação ou um plano de computação subdimensionado. Corrija a causa e depois retome-o para colocá-lo de volta no ar.",
    description:
      "Settings tab resume section description for a service the platform suspended for a crash loop (stuck_crashlooping)",
  },
  "services.resumeCardDescriptionCron": {
    message: "Retomar este Cron Job retoma as execuções agendadas.",
    description: "Settings tab resume section description for a cron job",
  },
  "services.dangerZoneTitle": {
    message: "Zona de perigo",
    description: "Settings tab delete section title (destructive)",
  },
  "services.dangerZoneDescription": {
    message:
      "Excluir um serviço remove permanentemente o serviço, o deployment e a URL dele. Isso não pode ser desfeito.",
    description:
      "Settings tab delete section description, for the types served at a public host",
  },
  "services.dangerZoneDescriptionNoUrl": {
    message:
      "Excluir um serviço remove permanentemente o serviço e o deployment dele. Isso não pode ser desfeito.",
    description:
      "Settings tab delete section description, for private services, background workers and cron jobs — none of which have a URL to remove",
  },
  "services.deleteButton": {
    message: "Excluir serviço",
    description: "Danger-zone button that opens the delete-confirm dialog",
  },
  "services.deleteConfirmTitle": {
    message: "Excluir {name}?",
    description: "Delete-confirm dialog title",
  },
  "services.deleteConfirmBody": {
    message:
      "Esta ação não pode ser desfeita. Tem certeza de que deseja excluir este serviço ({type})?",
    description:
      "Delete-confirm dialog body ({type} = lowercase Render type words, e.g. 'web service')",
  },
  "services.deleteConfirmPrompt": {
    message: "Digite {phrase} abaixo para confirmar.",
    description:
      "Body prompt naming Render's exact 'sudo delete <type> <name>' phrase (rendered bold by SudoCommandField)",
  },
  "services.deleteCancel": {
    message: "Cancelar",
    description: "Delete-confirm dialog cancel button",
  },
  "services.deleteConfirm": {
    message: "Excluir serviço",
    description:
      "Delete-confirm dialog submit button (armed once the name matches)",
  },
  "services.deleteSuccess": {
    message: "{name} excluído",
    description: "Toast on a successful service delete",
  },
  "services.deleteError": {
    message: "Não foi possível excluir {name}",
    description: "Toast on a failed service delete",
  },
  "services.newServiceButton": {
    message: "Novo serviço",
    description:
      "Button on the services list page that opens the create wizard",
  },
  "services.createTitle": {
    message: "Novo serviço",
    description: "Create-wizard page title",
  },
  "services.createWebTitle": {
    message: "Novo Web Service",
    description:
      "Create-wizard page title when the web_service type is selected",
  },
  "services.createWebDescription": {
    message:
      "Faça o deploy de um serviço web a partir de um repositório Git ou de uma imagem Docker.",
    description:
      "Create-wizard page subtitle when the web_service type is selected",
  },
  "services.createPrivateTitle": {
    message: "Novo Private Service",
    description:
      "Create-wizard page title when the private_service type is selected",
  },
  "services.createPrivateDescription": {
    message:
      "Faça o deploy de um serviço privado a partir de um repositório Git ou de uma imagem Docker.",
    description:
      "Create-wizard page subtitle when the private_service type is selected",
  },
  "services.createWorkerTitle": {
    message: "Novo Background Worker",
    description:
      "Create-wizard page title when the background_worker type is selected",
  },
  "services.createWorkerDescription": {
    message:
      "Faça o deploy de um worker em segundo plano a partir de um repositório Git ou de uma imagem Docker.",
    description:
      "Create-wizard page subtitle when the background_worker type is selected",
  },
  "services.createStaticTitle": {
    message: "Novo Static Site",
    description:
      "Create-wizard page title when the static_site type is selected",
  },
  "services.createStaticDescription": {
    message:
      "Faça o build e o deploy de um site estático a partir de um repositório Git.",
    description:
      "Create-wizard page subtitle when the static_site type is selected",
  },
  "services.createCronTitle": {
    message: "Novo Cron Job",
    description: "Create-wizard page title when the cron_job type is selected",
  },
  "services.createCronDescription": {
    message: "Execute um comando em um agendamento recorrente.",
    description:
      "Create-wizard page subtitle when the cron_job type is selected",
  },
  "services.createSourceTitle": {
    message: "Código-fonte",
    description: "Create-wizard source-picker section label",
  },
  "services.createTabGitHub": {
    message: "GitHub",
    description: "Create-wizard source-tab label for connected GitHub repos",
  },
  "services.createTabPublicGit": {
    message: "URL Git pública",
    description: "Create-wizard source-tab label for a public git URL",
  },
  "services.createTabImage": {
    message: "Imagem existente",
    description: "Create-wizard source-tab label for a pre-built Docker image",
  },
  "services.createRepoSearchShort": {
    message: "Buscar…",
    description:
      "Repository search placeholder where the field is narrow (a phone); the field keeps its full accessible name",
  },
  "services.createRepoCurrent": {
    message: "Atual",
    description:
      "Update Source repository list: tag on the repository the service uses now, shown once another one is chosen",
  },
  "services.createRepoSearchPlaceholder": {
    message: "Pesquisar repositórios…",
    description: "Create-wizard GitHub tab repo-search input placeholder",
  },
  "services.createRepoPrivateBadge": {
    message: "Privado",
    description: "Badge on a private GitHub repo row in the repo picker",
  },
  "services.createRepoEmpty": {
    message: "Nenhum repositório encontrado.",
    description:
      "Create-wizard GitHub tab empty state (no repos in the installation)",
  },
  "services.createRepoNoMatch": {
    message: "Nenhum repositório corresponde à sua busca.",
    description:
      "Create-wizard GitHub tab empty state when the search filter returns nothing",
  },
  "services.createRepoEmptyHint": {
    message:
      "Dê ao bex acesso a um repositório no GitHub ou conecte outra conta.",
    description:
      "Source picker GitHub tab, under the empty state: what to do when the connected accounts grant no repository. Credentials is the menu beside the search field",
  },
  "services.createRepoLoadError": {
    message: "Não foi possível carregar seus repositórios.",
    description:
      "Source picker GitHub tab: the repository list could not be read, shown with a Try again button. Not the same as having no repositories",
  },
  "services.createRepoListLabel": {
    message: "Repositórios",
    description:
      "Accessible name of the repository list in the source picker's GitHub tab",
  },
  "services.createRepoClearSearch": {
    message: "Limpar busca",
    description:
      "Source picker GitHub tab: button under the no-match message that empties the repository search field",
  },
  "services.sourceNoChanges": {
    message: "Nenhuma alteração para salvar.",
    description:
      "Update Source dialog footer: why Update source is disabled when the dialog still shows the service's current source",
  },
  "services.sourceSelectRepo": {
    message: "Selecione um repositório.",
    description:
      "Update Source dialog footer: why Update source is disabled on the GitHub tab before a repository is picked",
  },
  "services.sourceSelectBuildSource": {
    message: "Selecione uma origem de build.",
    description:
      "Update Source dialog footer: why Update source is disabled on the Build source tab before one is picked",
  },
  "services.sourceEnterRepoUrl": {
    message: "Informe a URL de um repositório.",
    description:
      "Update Source dialog footer: why Update source is disabled on the Public Git URL tab while the URL field is empty",
  },
  "services.sourceEnterImage": {
    message: "Informe uma imagem.",
    description:
      "Update Source dialog footer: why Update source is disabled on the Existing Image tab while the image field is empty",
  },
  "services.createRepoOpenCredentials": {
    message: "Abrir Credenciais",
    description:
      "Source picker GitHub tab: button under the empty repository list that opens the Credentials menu beside the search field",
  },
  "services.sourcePending": {
    message: "Nova origem: {source}",
    description:
      "Update Source dialog footer: names what Update source will save. {source} is a repository and branch, an image, or a build source's name",
  },
  "services.sourceBranchLocked": {
    message: "Disponível quando você escolher um repositório.",
    description:
      "Update Source dialog: helper under the disabled Branch field before a repository is picked or a valid URL entered",
  },
  "services.sourceNoBuildSources": {
    message: "Nenhuma origem de build para escolher.",
    description:
      "Update Source dialog footer: why Update source is disabled on the Build source tab when the workspace has no build sources",
  },
  "services.sourceFixField": {
    message: "Preencha os campos acima para continuar.",
    description:
      "Update Source dialog footer: why Update source is disabled while a field is malformed (its own message is under it) or a required one is empty",
  },
  "services.createRepoNoMatchHint": {
    message:
      "Não está na lista? Dê ao bex acesso a ele no GitHub ou conecte outra conta.",
    description:
      "Source picker GitHub tab, under the no-match message: the usual cause is a repository the connected accounts do not grant. Shown with Clear search and Open Credentials buttons",
  },
  "services.sourceBranchLockedUrl": {
    message: "Disponível quando você informar a URL de um repositório.",
    description:
      "Update Source dialog, Public Git URL tab: helper under the disabled Branch field before a valid URL is entered",
  },
  "services.createGitConnectPromptTitle": {
    message: "Conectar GitHub",
    description:
      "Create-wizard GitHub tab connect-prompt heading (no GitHub connection yet)",
  },
  "services.createGitConnectPromptBody": {
    message:
      "Conecte sua conta do GitHub para fazer deploy de repositórios privados ou públicos.",
    description:
      "Create-wizard GitHub tab connect-prompt body (no GitHub connection yet)",
  },
  "services.createPublicUrlLabel": {
    message: "URL do repositório",
    description: "Create-wizard Public Git URL tab input label",
  },
  "services.createPublicUrlPlaceholder": {
    message: "https://github.com/you/your-repo",
    description: "Create-wizard Public Git URL tab input placeholder",
  },
  "services.createPublicUrlError": {
    message:
      "Informe uma URL https:// ou uma URL SSH do GitHub (git@github.com:owner/repo.git).",
    description: "Create-wizard Public Git URL tab validation message",
  },
  "services.createPublicUrlFileError": {
    message: "Use a URL do repositório, não a de uma página de arquivo.",
    description:
      "Create-wizard Public Git URL: a GitHub file (blob) page was pasted, which names no repository to build (w4/576)",
  },
  "services.createImageLabel": {
    message: "Imagem Docker",
    description: "Create-wizard Existing Image tab input label",
  },
  "services.createImagePlaceholder": {
    message: "docker.io/library/nginx:latest",
    description: "Create-wizard Existing Image tab input placeholder",
  },
  "services.createImagePortHint": {
    message:
      "O bex usa a porta que a imagem declara (EXPOSE). Defina a porta abaixo somente se a imagem escutar em uma porta que ela não declara.",
    description:
      "Create-wizard Existing Image tab hint: set the port below to the one the image listens on (w9/011, w6/m149)",
  },
  "services.createImageMalformed": {
    message:
      "Use uma referência de imagem Docker, ex.: docker.io/library/nginx:latest — um caminho em minúsculas com tag ou digest opcional.",
    description: "Inline error under a malformed Docker Image field (w4/406)",
  },
  "services.createImageTooLong": {
    message: "No máximo 512 bytes",
    description: "Inline error under an over-long Docker Image field (w4/406)",
  },
  "services.createImagePrivateHost": {
    message:
      "O host do registry deve ser público — localhost e endereços privados ou link-local não podem ser usados.",
    description:
      "Inline error under a Docker Image field naming a private registry host (w4/406)",
  },
  "services.createFieldPort": {
    message: "Porta",
    description: "Create wizard: service port field label",
  },
  "services.createFieldPortFromImage": {
    message: "Da imagem",
    description:
      "Create wizard: port placeholder for an image-port service (a Docker-built or prebuilt-image web or private service), whose ports come from its image (w6/m148, w4/m243, w4/m323)",
  },
  "services.createFieldPortImageHint": {
    message: "Deixe em branco e a porta que a imagem declara se torna $PORT.",
    description:
      "Create wizard: port hint for an image-port private service (Docker-built or prebuilt image), whose primary is also its $PORT — blank takes the image's declared ports (w6/m148, w4/m323, w4/615)",
  },
  "services.createFieldPortImageHintWeb": {
    message:
      "Deixe em branco para encaminhar o tráfego à porta que a imagem declara; a variável $PORT continua 10000, a menos que você defina uma porta.",
    description:
      "Create wizard: port hint for an image-port web service (Docker-built or prebuilt image) — the selected port gets the traffic, while $PORT stays Render's 10000 unless set (w4/m243, w4/615)",
  },
  "services.createFieldPortHint": {
    message:
      "Seu contêiner deve escutar na porta {port}; o bex encaminha o tráfego para ela e a define como $PORT.",
    description: "Create wizard: service port field hint",
  },
  "services.createRegistryCredentialLabel": {
    message: "Credencial de registro",
    description: "Create-wizard private-image registry credential label",
  },
  "services.createRegistryCredentialPlaceholder": {
    message: "Selecione uma credencial de registro",
    description: "Create-wizard registry credential picker placeholder",
  },
  "services.createRegistryCredentialNone": {
    message: "Nenhuma credencial — imagem pública",
    description: "Create-wizard registry credential picker empty option",
  },
  "services.createRegistryCredentialDescription": {
    message:
      "Escolha uma credencial armazenada do workspace quando a imagem ou a imagem base do Dockerfile for privada.",
    description: "Create-wizard registry credential picker help text",
  },
  "services.registryCredentialEmpty": {
    message: "Não há credenciais de registro armazenadas disponíveis.",
    description: "Registry credential selector empty state",
  },
  "services.registryCredentialListError": {
    message: "Não foi possível carregar as credenciais de registro.",
    description: "Registry credential selector load error",
  },
  "services.registryCredentialManage": {
    message: "Gerenciar credenciais de registro",
    description: "Link from a service credential selector to account settings",
  },
  "services.registryCredentialSettingsTitle": {
    message: "Credencial de registro de imagens",
    description: "Service Settings private-image credential card title",
  },
  "services.registryCredentialSettingsDescription": {
    message:
      "Escolha a credencial armazenada usada para baixar esta imagem privada ou a imagem base do Dockerfile.",
    description: "Service Settings private-image credential card description",
  },
  "services.registryCredentialSave": {
    message: "Salvar credencial",
    description: "Service Settings registry credential save action",
  },
  "services.registryCredentialSaved": {
    message: "Credencial de registro atualizada.",
    description: "Toast after attaching or changing a registry credential",
  },
  "services.registryCredentialCleared": {
    message: "Credencial de registro removida.",
    description: "Toast after clearing a registry credential",
  },
  "services.registryCredentialError": {
    message: "Não foi possível atualizar a credencial de registro.",
    description: "Toast after a registry credential update fails",
  },
  "services.headerRegistryCredential": {
    message: "Credencial de registro:",
    description: "Service detail header label for a bound registry credential",
  },
  "services.createSettingsTitle": {
    message: "Configurações",
    description: "Create-wizard settings section heading",
  },
  "services.createFieldName": {
    message: "Nome",
    description: "Create-wizard name input label",
  },
  "services.createFieldNamePlaceholder": {
    message: "my-service",
    description: "Create-wizard name input placeholder",
  },
  "services.createFieldNameError": {
    message:
      "Informe um nome de até 100 caracteres, sem quebras de linha nem caracteres de controle.",
    description: "Create-wizard name validation message",
  },
  "services.createFieldNameResourceName": {
    message: "Nome do recurso: {name}",
    description:
      "Create-wizard preview of the resource name (slug) bex derives from a free-form service name (w5/m153)",
  },
  "services.createFieldNameTaken": {
    message: "O nome já está em uso",
    description:
      "Create-wizard inline error when the service name is already taken in the current workspace (w4/m19)",
  },
  "services.createFieldNameUseSuggestion": {
    message: "Usar {name}",
    description:
      "Create-wizard button offering the suggested free name in place of a taken one (w4/m19)",
  },
  "services.createFieldNameChecking": {
    message: "Verificando disponibilidade…",
    description:
      "Create-wizard transient message while the debounced name-availability check is in flight (w4/m19)",
  },
  "services.createFieldBranch": {
    message: "Branch",
    description: "Create-wizard branch input label (git sources)",
  },
  "services.createFieldBranchPlaceholder": {
    message: "Branch padrão do repositório",
    description:
      "Create-wizard branch input placeholder when the repo's default is not known client-side: a blank branch builds the repository's default branch (w8/m76)",
  },
  "services.createFieldRootDir": {
    message: "Diretório raiz",
    description: "Create-wizard root-directory input label",
  },
  "services.createFieldRootDirPlaceholder": {
    message: "ex.: backend",
    description: "Create-wizard root-directory input placeholder",
  },
  "services.createFieldRootDirHint": {
    message:
      "Subdiretório a partir do qual fazer o build. Deixe em branco para usar a raiz do repositório.",
    description: "Create-wizard root-directory input hint text",
  },
  "services.createFieldRuntime": {
    message: "Runtime",
    description: "Create-wizard Render-compatible runtime selector label",
  },
  "services.createFieldBuildCommand": {
    message: "Comando de build",
    description: "Create-wizard Render-compatible build command label",
  },
  "services.createFieldStartCommand": {
    message: "Comando de início",
    description: "Create-wizard Render-compatible start command label",
  },
  "services.createFieldDockerfilePath": {
    message: "Caminho do Dockerfile",
    description: "Create-wizard Dockerfile-path label for the Docker runtime",
  },
  "services.createFieldDockerfilePathPlaceholder": {
    message: "Dockerfile",
    description: "Create-wizard Dockerfile-path placeholder",
  },
  "services.createFieldDockerfilePathHint": {
    message:
      "Caminho relativo ao Diretório raiz. Deixe em branco para Dockerfile.",
    description: "Create-wizard Dockerfile-path help text",
  },
  "services.createFieldDockerCommand": {
    message: "Comando Docker",
    description: "Create-wizard Docker CMD override label (Render wording)",
  },
  "services.createFieldDockerCommandPlaceholder": {
    message: "Usar o CMD do Dockerfile",
    description: "Create-wizard optional Docker Command placeholder",
  },
  "services.imageCommandPlaceholder": {
    message: "Usar o comando padrão da imagem",
    description:
      "Create-wizard and settings placeholder for an Existing Image service's optional Docker Command",
  },
  "services.imageCommandHint": {
    message:
      "Substitui o comando padrão da imagem (seu CMD). Deixe em branco para usar o padrão.",
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
    message: "Plano de computação",
    description: "Create-wizard plan-picker section label",
  },
  "services.createFieldAutoDeploy": {
    message: "Deploy automático a cada push",
    description: "Create-wizard auto-deploy toggle label",
  },
  "services.createFieldAutoDeployHint": {
    message:
      "Faz um novo deploy automaticamente quando você faz push para este branch.",
    description: "Create-wizard auto-deploy toggle hint text",
  },
  "services.createCancel": {
    message: "Cancelar",
    description: "Create-wizard cancel button",
  },
  "services.createSubmit": {
    message: "Fazer deploy do serviço",
    description: "Create-wizard submit button",
  },
  "services.createSuccess": {
    message: "Fazendo deploy de {name}…",
    description: "Toast shown after createService succeeds",
  },
  "services.createError": {
    message: "Não foi possível criar {name}. Tente novamente.",
    description: "Toast shown after createService fails",
  },
  "services.scaleSuccess_one": {
    message: "Escalando para {count} instância…",
    description:
      "Toast acknowledging that scaleService accepted the desired count; convergence is still asynchronous (singular)",
  },
  "services.scaleSuccess_many": {
    message: "Escalando para {count} de instâncias…",
    description:
      "Toast acknowledging that scaleService accepted the desired count; convergence is still asynchronous (plural)",
  },
  "services.scaleSuccess_other": {
    message: "Escalando para {count} instâncias…",
    description:
      "Toast acknowledging that scaleService accepted the desired count; convergence is still asynchronous (plural)",
  },
  "services.scaleError": {
    message: "Falha ao atualizar o número de instâncias.",
    description: "Toast shown after scaleService fails",
  },
  "services.createTypePickerTitle": {
    message: "Tipo de serviço",
    description: "Label above the service type picker in the create wizard",
  },
  "services.createTypeWebDesc": {
    message: "Exponha seu serviço em uma URL pública",
    description:
      "Description shown under the Web Service type card in the create wizard",
  },
  "services.createTypePrivateDesc": {
    message: "Acessível somente dentro da rede da plataforma",
    description: "Description shown under the Private Service type card",
  },
  "services.createTypeWorkerDesc": {
    message: "Execute processamento em segundo plano sem porta nem URL",
    description: "Description shown under the Background Worker type card",
  },
  "services.createTypeCronDesc": {
    message: "Execute um comando em um agendamento recorrente",
    description: "Description shown under the Cron Job type card",
  },
  "services.createTypeStaticDesc": {
    message:
      "Faça o build e sirva um site estático a partir de armazenamento de objetos",
    description: "Description shown under the Static Site type card",
  },
  "services.createFieldSchedule": {
    message: "Agendamento",
    description: "Label for the cron schedule field in the create wizard",
  },
  "services.createFieldSchedulePlaceholder": {
    message: "0 0 * * *",
    description: "Placeholder for the cron schedule field",
  },
  "services.createFieldScheduleHint": {
    message:
      "Uma expressão crontab de 5 campos (minuto hora dia mês dia da semana). Executa em UTC.",
    description: "Hint text under the schedule field",
  },
  "services.createFieldSchedulePreview": {
    message: "{description} · executa em UTC",
    description:
      "Live human-readable description of a valid schedule, shown under the field",
  },
  "services.createFieldScheduleError": {
    message: "Informe uma expressão cron válida de 5 campos, ex.: 0 0 * * *.",
    description: "Validation error for an invalid cron expression",
  },
  "services.createFieldCommand": {
    message: "Comando",
    description: "Label for the command field in the create wizard",
  },
  "services.createFieldCommandPlaceholder": {
    message: "python script.py",
    description: "Placeholder for the command field",
  },
  "services.createFieldCommandHint": {
    message: "O comando a executar em cada invocação agendada.",
    description: "Hint text under the command field",
  },
  "services.createFieldPublishPath": {
    message: "Diretório de publicação",
    description: "Label for the publish directory field in the create wizard",
  },
  "services.createFieldPublishPathPlaceholder": {
    message: "ex.: dist",
    description: "Placeholder for the publish directory field",
  },
  "services.createFieldPublishPathHint": {
    message:
      "A pasta com o site compilado, relativa ao diretório raiz (ex.: dist, ou . para o próprio diretório raiz).",
    description: "Hint text under the publish directory field",
  },
  "services.createFieldPublishPathRequired": {
    message:
      "Obrigatório. Informe o diretório de saída do build a servir como raiz do site, relativo ao Diretório raiz (ex.: dist, build, public, ou . para o próprio Diretório raiz — a raiz do repositório quando o Diretório raiz está em branco).",
    description:
      "Hint under the publish directory field while it is blank: the field is required, the placeholder is only an example",
  },
  "services.createNoPublicUrlNote": {
    message: "Este tipo de serviço não tem URL pública.",
    description:
      "Note shown for private/worker types that don't produce a public URL",
  },
  "services.createFieldEnvVarsTitle": {
    message: "Variáveis de ambiente",
    description: "Section heading for env vars in the create wizard",
  },
  "services.createFieldEnvVarsAdd": {
    message: "Adicionar variável",
    description: "Button to add an env var row in the create wizard",
  },
  "services.envRowKey": {
    message: "Chave da variável {n}",
    description:
      "A create-flow variable row's key field (aria-label), named by its row as its Remove button is (w4/544)",
  },
  "services.envRowValue": {
    message: "Valor da variável {n}",
    description:
      "A create-flow variable row's value field (aria-label), named by its row (w4/544)",
  },
  "services.envRowGenerate": {
    message: "Gerar valor para a variável {n}",
    description:
      "A create-flow variable row's Generate button (aria-label), named by its row (w4/544)",
  },
  "services.envRowRemove": {
    message: "Remover variável {n}",
    description:
      "A create-flow variable row's Remove button (aria-label), named by its row (w4/544)",
  },
  "services.fileRowName": {
    message: "Nome do arquivo secreto {n}",
    description:
      "A create-flow secret file row's name field (aria-label), named by its row and starting with the dialog's visible \"File name\" label (w4/544)",
  },
  "services.fileRowContent": {
    message: "Conteúdo do arquivo secreto {n}",
    description:
      "A create-flow secret file row's contents field (aria-label), named by its row (w4/544)",
  },
  "services.fileRowRemove": {
    message: "Remover arquivo secreto {n}",
    description:
      "A create-flow secret file row's Remove button (aria-label), named by its row (w4/544)",
  },
  "services.createFieldEnvVarsKey": {
    message: "Chave",
    description: "Label for the env var key column in the create wizard",
  },
  "services.createFieldEnvVarsValue": {
    message: "Valor",
    description: "Label for the env var value column in the create wizard",
  },
  "services.createFieldEnvVarsKeyPlaceholder": {
    message: "KEY_NAME",
    description: "Placeholder for the env var key input in the create wizard",
  },
  "services.createFieldEnvVarsValuePlaceholder": {
    message: "valor",
    description: "Placeholder for the env var value input in the create wizard",
  },
  "services.createFieldEnvVarsKeyError": {
    message:
      "Use até 253 letras, dígitos, pontos, hifens e underscores; não use '.' nem comece com '..'.",
    description:
      "Error shown when an env var key is invalid in the create wizard",
  },
  "services.createFieldSecretFilesTitle": {
    message: "Arquivos secretos",
    description: "Section heading for secret files in the create wizard",
  },
  "services.createFieldSecretFilesHint": {
    message:
      "Montados como somente leitura em /etc/secrets desde o primeiro deploy.",
    description: "Hint for create-time secret files",
  },
  "services.createFieldSecretFilesAdd": {
    message: "Adicionar arquivo secreto",
    description: "Button to add a create-time secret file",
  },
  "services.createFieldSecretFilesNamePlaceholder": {
    message: "credentials.json",
    description: "Placeholder for a secret file name",
  },
  "services.createFieldSecretFilesContentPlaceholder": {
    message: "Cole o conteúdo secreto",
    description: "Placeholder for secret file contents",
  },
  "services.createFieldSecretFilesNameError": {
    message:
      "Use apenas letras, dígitos, pontos, hifens e underscores; . e .. não são permitidos.",
    description: "Invalid secret file name error",
  },
  "services.createFieldEnvironmentTitle": {
    message: "Projeto e ambiente",
    description: "Create wizard grouping section title",
  },
  "services.createFieldProject": {
    message: "Projeto",
    description: "Accessible label for the create project picker",
  },
  "services.createFieldProjectNone": {
    message: "Sem projeto",
    description: "Unassigned option in the project picker",
  },
  "services.createFieldEnvironment": {
    message: "Ambiente",
    description: "Accessible label for the create environment picker",
  },
  "services.createFieldEnvironmentNone": {
    message: "Sem ambiente",
    description: "Unassigned option in the environment picker",
  },
  "services.createFieldEnvironmentHint": {
    message:
      "Selecionar um ambiente também adiciona o serviço ao projeto pai dele.",
    description: "Hint for create-time environment assignment",
  },
  "services.navEvents": {
    message: "Eventos",
    description: "Service-detail nav item (events tab)",
  },
  "services.navDeploys": {
    message: "Deploys",
    description:
      "Service-detail nav item (dedicated deploy-history tab, w9/002)",
  },
  "services.navRuns": {
    message: "Execuções",
    description:
      "Service-detail nav item for a cron job: its run history, Render's Runs page (w2/m360)",
  },
  "services.navDisk": {
    message: "Disco",
    description:
      "Service-detail nav item (persistent disk page); Render's label is singular",
  },
  "services.diskDescription": {
    message:
      "Anexe um SSD para manter os dados do sistema de arquivos do seu serviço entre deploys. Os discos são cobrados a US$ 0,175/GB por mês.",
    description:
      "Disk tab card description; bex's rate is 30% below Render's $0.25",
  },
  "services.diskEmptyTitle": {
    message: "Nenhum disco anexado",
    description: "Disk tab empty state title",
  },
  "services.diskEmptyBody": {
    message:
      "Somente os arquivos no caminho de montagem do disco são preservados entre deploys e reinicializações.",
    description: "Disk tab empty state body",
  },
  "services.diskPaidOnly": {
    message:
      "Discos exigem um plano de computação pago. Faça upgrade do plano deste serviço para anexar um.",
    description: "Shown instead of the add button on a free service",
  },
  "services.diskAddAction": {
    message: "Adicionar disco",
    description: "Button that opens and submits the add-disk form",
  },
  "services.diskLoadErrorTitle": {
    message: "Não foi possível carregar o disco deste serviço",
    description: "Disk tab error state title",
  },
  "services.diskWarningsTitle": {
    message: "Observe o seguinte:",
    description: "Heading above the five add-disk warnings (Render's wording)",
  },
  "services.diskWarningZeroDowntime": {
    message: "Anexar um disco desativa os deploys sem downtime para o serviço.",
    description: "Add-disk warning 1",
  },
  "services.diskWarningSingleInstance": {
    message:
      "Serviços com um disco anexado não podem escalar para várias instâncias.",
    description: "Add-disk warning 2",
  },
  "services.diskWarningOnePerService": {
    message: "Você pode anexar no máximo um disco por serviço.",
    description: "Add-disk warning 3",
  },
  "services.diskWarningMountPathOnly": {
    message:
      "Somente os arquivos no caminho de montagem do seu disco são mantidos.",
    description: "Add-disk warning 4",
  },
  "services.diskWarningNoSharing": {
    message: "Outros serviços não podem acessar o disco deste serviço.",
    description: "Add-disk warning 5",
  },
  "services.diskMountPathLabel": {
    message: "Caminho de montagem",
    description: "Add-disk form field label",
  },
  "services.diskMountPathHint": {
    message:
      "O caminho de montagem absoluto do disco. Somente os arquivos neste caminho são mantidos entre deploys. Não pode ser o diretório raiz (/).",
    description: "Add-disk mount path helper text",
  },
  "services.diskMountPathRequired": {
    message: "Informe um caminho de montagem.",
    description: "Client-side validation: empty mount path",
  },
  "services.diskMountPathAbsolute": {
    message: "O caminho de montagem deve ser absoluto (começar com /).",
    description: "Client-side validation: relative mount path",
  },
  "services.diskMountPathNotRoot": {
    message:
      "O caminho de montagem não pode ser o diretório raiz nem terminar com barra.",
    description: "Client-side validation: root or trailing slash",
  },
  "services.diskMountPathReserved": {
    message:
      "Esse caminho é reservado pela plataforma. Monte um subdiretório dele.",
    description: "Client-side validation: reserved mount path",
  },
  "services.diskSizeLabel": {
    message: "Tamanho",
    description: "Add-disk size field label",
  },
  "services.diskSizeHint": {
    message:
      "Você pode aumentar o tamanho depois, mas não pode diminuí-lo. Recomendamos começar com o menor valor que atenda ao seu caso de uso.",
    description: "Add-disk size helper text (Render's wording)",
  },
  "services.diskSizeChip": {
    message: "{size} GB",
    description: "Size quick-select chip and size display",
  },
  "services.diskAddSuccess": {
    message: "Disco anexado. O serviço está sendo reimplantado.",
    description: "Toast after a disk is attached",
  },
  "services.diskGrowLabel": {
    message: "Aumentar tamanho (GB)",
    description: "Grow control label on an attached disk",
  },
  "services.diskGrowAction": {
    message: "Aumentar tamanho",
    description: "Grow button",
  },
  "services.diskGrowHint": {
    message:
      "O aumento de tamanho não causa downtime, e o armazenamento adicional fica disponível em poucos segundos. Um disco nunca pode ser reduzido.",
    description: "Grow helper text in Render's wording",
  },
  "services.diskResizeSuccess": {
    message: "Tamanho do disco aumentado.",
    description: "Toast after a successful grow",
  },
  "services.diskDeleteAction": {
    message: "Excluir disco",
    description: "Destructive action on an attached disk",
  },
  "services.diskDeleteTitle": {
    message: "Excluir este disco?",
    description: "Delete confirmation title",
  },
  "services.diskDeleteWarning": {
    message:
      "Todos os dados do disco serão perdidos e o serviço perderá o acesso a ele imediatamente. Os snapshots dele também serão excluídos. Isso não pode ser desfeito.",
    description: "Delete confirmation body",
  },
  "services.diskDeleteConfirm": {
    message: "Excluir disco",
    description: "Delete confirmation action",
  },
  "services.diskDeleteSuccess": {
    message: "Disco excluído.",
    description: "Toast after a disk is deleted",
  },
  "services.diskSnapshotsTitle": {
    message: "Snapshots",
    description: "Snapshots card title",
  },
  "services.diskSnapshotsDescription": {
    message:
      "Um snapshot deste disco é feito diariamente e mantido por pelo menos sete dias.",
    description: "Snapshots card description",
  },
  "services.diskSnapshotsNotConfiguredTitle": {
    message: "Os snapshots não estão configurados",
    description:
      "Snapshots card heading when the deployment has no snapshot object store configured (not a failure).",
  },
  "services.diskSnapshotsNotConfiguredBody": {
    message:
      "Este deploy do bex não foi configurado com um armazenamento de objetos para snapshots de disco, então nenhum está sendo feito. Seus dados não são afetados — peça a um operador para ativá-los.",
    description:
      "Explains that snapshots are unconfigured rather than broken, and that an operator enables them.",
  },
  "services.diskSnapshotsEmptyTitle": {
    message: "Ainda não há snapshots",
    description: "Snapshots empty state title",
  },
  "services.diskSnapshotsEmptyBody": {
    message: "O primeiro snapshot é feito na próxima execução noturna.",
    description: "Snapshots empty state body",
  },
  "services.diskSnapshotsFailedTitle": {
    message: "O último snapshot falhou",
    description: "A disk's most recent nightly snapshot attempt failed",
  },
  "services.diskSnapshotsFailedReadBody": {
    message:
      "Parte deste disco não pôde ser lida, então nenhum snapshot foi feito e as alterações recentes não têm backup. Os snapshots existentes não são afetados. Se acontecer de novo, entre em contato com o suporte.",
    description: "Failed snapshot: the volume could not be read",
  },
  "services.diskSnapshotsFailedUploadBody": {
    message:
      "O snapshot não pôde ser salvo no armazenamento, então as alterações recentes não têm backup. Os snapshots existentes não são afetados; a próxima execução noturna tentará novamente.",
    description: "Failed snapshot: the object store did not accept it",
  },
  "services.diskSnapshotsFailedConfigBody": {
    message:
      "O job de snapshot não pôde ser iniciado corretamente, então as alterações recentes não têm backup. Os snapshots existentes não são afetados. Entre em contato com o suporte.",
    description: "Failed snapshot: the job was misconfigured",
  },
  "services.diskSnapshotsFailedOtherBody": {
    message:
      "A última tentativa de snapshot falhou, então as alterações recentes não têm backup. Os snapshots existentes não são afetados. Se acontecer de novo, entre em contato com o suporte.",
    description: "Failed snapshot: an unrecognized stage",
  },
  "services.diskSnapshotsOverdueTitle": {
    message: "Nenhum snapshot recente",
    description: "A disk's nightly snapshot did not arrive when expected",
  },
  "services.diskSnapshotsOverdueBody": {
    message:
      "O snapshot noturno esperado até {dueBy} não foi feito, então as alterações recentes neste disco não têm backup. Os snapshots existentes não são afetados. Se isso persistir, entre em contato com o suporte.",
    description:
      "Explains an overdue nightly snapshot; {dueBy} is a local date/time",
  },
  "services.diskSnapshotsNotRunningTitle": {
    message: "Os snapshots não estão em execução",
    description:
      "The platform could not schedule this disk's nightly snapshots",
  },
  "services.diskSnapshotsNotRunningBody": {
    message:
      "O bex não conseguiu agendar os snapshots noturnos deste disco, então as novas alterações não estão tendo backup. Os snapshots existentes não são afetados. Entre em contato com o suporte.",
    description:
      "Explains that nightly snapshots cannot be scheduled and whom to contact",
  },
  "services.diskSnapshotsUnavailableTitle": {
    message: "Os snapshots estão indisponíveis",
    description: "Snapshots error state title",
  },
  "services.diskRestoreDatabaseWarning": {
    message:
      "Não restaure um disco para recuperar um banco de dados que roda nele — restaurar arquivos por baixo de um banco de dados em execução pode corrompê-lo. Use o backup e a restauração do próprio banco de dados.",
    description:
      "Render's database-recovery warning, carried verbatim in meaning",
  },
  "services.diskRestoreAction": {
    message: "Restaurar",
    description: "Restore button on a snapshot row",
  },
  "services.diskRestoreTitle": {
    message: "Restaurar este snapshot?",
    description: "Restore confirmation title",
  },
  "services.diskRestoreWarning": {
    message:
      "O serviço é interrompido, o disco é substituído por este snapshot e o serviço é reiniciado. Tudo o que foi gravado após o snapshot é perdido. Isso não pode ser desfeito.",
    description: "Restore confirmation body",
  },
  "services.diskRestoreConfirm": {
    message: "Restaurar disco",
    description: "Restore confirmation action",
  },
  "services.diskUnsupportedType": {
    message:
      "Discos persistentes precisam de uma instância de longa duração para serem montados, então estão disponíveis em Web Services, Private Services e Background Workers.",
    description:
      "Empty-state body on the Disk tab for a service type that cannot have a disk.",
  },
  "services.diskSudoPrompt": {
    message: "Para continuar, digite {phrase}",
    description:
      "Label above the typed-confirmation input for an irreversible disk action.",
  },
  "services.diskConfigTitle": {
    message: "Configuração do disco",
    description:
      "Heading of the card holding the disk's mount path and size settings.",
  },
  "services.diskConfigDescription": {
    message: "Configure o caminho de montagem e o tamanho deste disco.",
    description: "Subtitle of the disk configuration card.",
  },
  "services.diskSizeUnit": {
    message: "GB",
    description: "Unit suffix shown beside the disk size input.",
  },
  "services.diskEditAction": {
    message: "Editar",
    description: "Button that unlocks the disk size field for editing.",
  },
  "services.diskProvisionedLabel": {
    message: "{size} GB provisionados",
    description:
      "Provisioned disk size shown beside the usage chart heading; this is the size billing meters.",
  },
  "services.diskDeleteCardTitle": {
    message: "Excluir disco",
    description:
      "Heading of the delete-disk card. Distinct from diskDeleteTitle, which is the confirmation dialog's question.",
  },
  "services.diskDeleteCardDescription": {
    message:
      "O disco será desanexado deste serviço e todos os dados nele serão excluídos permanentemente.",
    description: "Subtitle of the delete-disk card, stating what is lost.",
  },
  "services.diskUsageTitle": {
    message: "Uso do disco",
    description: "Heading of the disk usage chart on a service's Disk tab.",
  },
  "services.diskUsageDescription": {
    message:
      "Bytes gravados no volume montado, em relação à capacidade provisionada. O faturamento mede o tamanho provisionado, não os bytes usados.",
    description:
      "Subtitle of the disk usage chart, stating that billing uses provisioned size.",
  },
  "services.diskRestoreStarted": {
    message: "Restaurando o disco. O serviço fica parado até a conclusão.",
    description: "Toast after a restore is requested",
  },
  "services.navShell": {
    message: "Shell",
    description: "Service-detail nav item (running-instance SSH page)",
  },
  "services.navCompute": {
    message: "Computação",
    description:
      "Service sidebar link to the Compute page: the compute plan, manual scaling and autoscaling, as on Render (w6/m483)",
  },
  "services.eventsCount_one": {
    message: "{count} evento",
    description:
      "Accessible label for the number of visible service events (singular)",
  },
  "services.eventsCount_many": {
    message: "{count} de eventos",
    description:
      "Accessible label for the number of visible service events (plural)",
  },
  "services.eventsCount_other": {
    message: "{count} eventos",
    description:
      "Accessible label for the number of visible service events (plural)",
  },
  "services.eventsCountAtLeast_one": {
    message: "{count}+ eventos",
    description:
      "Accessible label for the event count badge while older events remain unread (singular)",
  },
  "services.eventsCountAtLeast_many": {
    message: "{count}+ eventos",
    description:
      "Accessible label for the event count badge while older events remain unread (plural)",
  },
  "services.eventsCountAtLeast_other": {
    message: "{count}+ eventos",
    description:
      "Accessible label for the event count badge while older events remain unread (plural)",
  },
  "services.eventsEmptyTitle": {
    message: "Ainda não há eventos",
    description: "Events tab empty-state title",
  },
  "services.eventsEmpty": {
    message: "Os deploys e as alterações do serviço aparecerão aqui.",
    description: "Events tab empty-state description",
  },
  "services.eventsErrorTitle": {
    message: "Os eventos não estão disponíveis",
    description: "Events tab query-error title",
  },
  "services.eventsErrorDescription": {
    message: "Não foi possível carregar os eventos deste serviço.",
    description: "Events tab query-error description",
  },
  "services.eventsRetry": {
    message: "Tentar novamente",
    description: "Events tab query-error retry button",
  },
  "services.eventsLoadOlder": {
    message: "Carregar eventos mais antigos",
    description: "Button that loads the next page or earlier event window",
  },
  "services.eventsLoadingOlder": {
    message: "Carregando eventos mais antigos…",
    description: "Busy label while an older service-event page is loading",
  },
  "services.eventsHistoryEnd": {
    message: "Você chegou ao início do histórico deste serviço.",
    description: "Terminal label after all retained service events are loaded",
  },
  "services.eventsFilter": {
    message: "Filtrar eventos",
    description: "Button opening the service event-type filter",
  },
  "services.eventsFilterSelected": {
    message: "Tipos de evento: {count}",
    description: "Event filter button with selected-type count",
  },
  "services.eventsFilterSearch": {
    message: "Buscar tipos de evento",
    description: "Search input for service event types",
  },
  "services.eventsFilterShowAll": {
    message: "Mostrar todos os eventos",
    description:
      "Events tab: button in the filtered-to-nothing state that shows every event type again",
  },
  "services.eventsShownOf": {
    message: "Mostrando {shown} de {total}",
    description:
      "Events tab: count over the feed while a filter hides some rows; {total} is the loaded count, '20+' while older events remain",
  },
  "services.eventsFilterAll": {
    message: "Todos os eventos",
    description: "Master service event filter option",
  },
  "services.eventsFilterNoMatches": {
    message: "Nenhum tipo de evento corresponde à sua busca.",
    description: "Empty search result in the event filter",
  },
  "services.eventsFilterEmptyTitle": {
    message: "Nenhum evento correspondente",
    description: "Title when loaded history has no selected event types",
  },
  "services.eventsFilterEmpty": {
    message:
      "Escolha mais tipos de evento ou carregue um histórico mais antigo.",
    description: "Body when loaded history has no selected event types",
  },
  "services.eventsFilterNoneTitle": {
    message: "Nenhum tipo de evento selecionado",
    description:
      "Events tab: title when the filter has every event type deselected",
  },
  "services.eventsFilterNone": {
    message: "Escolha pelo menos um tipo para ver eventos.",
    description:
      "Events tab: body when the filter has every event type deselected",
  },
  "services.eventsFilterGroup.deploy": {
    message: "Deploy",
    description: "Deploy event filter group",
  },
  "services.eventsFilterGroup.serviceStatus": {
    message: "Status do serviço",
    description: "Service-status event filter group",
  },
  "services.eventsFilterGroup.scaling": {
    message: "Escalonamento",
    description: "Scaling event filter group",
  },
  "services.eventsFilterGroup.maintenanceMode": {
    message: "Modo de manutenção",
    description: "Maintenance-mode event filter group",
  },
  "services.eventsFilterGroup.edgeCache": {
    message: "Cache de borda",
    description: "Edge-cache event filter group",
  },
  "services.eventsFilterGroup.configuration": {
    message: "Configuração",
    description: "bex configuration event filter group",
  },
  "services.eventsFilterGroup.other": {
    message: "Outros",
    description:
      "Event filter group holding types the API returned that the dashboard catalog does not know yet",
  },
  "services.eventsActor": {
    message: "por {actor}",
    description: "Actor attribution shown on a service event",
  },
  "services.eventsDeployReference": {
    message: "Deploy {id}",
    description: "Deploy identifier shown on a deploy activity row",
  },
  "services.eventsTriggerRollback": {
    message: "Rollback",
    description: "Deploy event trigger: rollback",
  },
  "services.eventsTriggerFirstBuild": {
    message: "Primeiro deploy",
    description: "Deploy event trigger: the service's first deploy",
  },
  "services.eventsTriggerManual": {
    message: "Deploy manual",
    description: "Deploy event trigger: manual",
  },
  "services.eventsTriggerEnvUpdated": {
    message: "Alteração de configuração",
    description:
      "Deploy event trigger: an environment change (env var, secret file, or env-group edit); a Settings save no longer sets envUpdated (w8/m135)",
  },
  "services.eventsTriggerDeployHook": {
    message: "Deploy Hook",
    description:
      "Deploy event trigger: a POST to the service's secret deploy-hook URL",
  },
  "services.eventsTriggerClearCache": {
    message: "Cache limpo",
    description: "Deploy event trigger: build-cache clear",
  },
  "services.eventsTriggerDeployedByRender": {
    message: "Deploy da plataforma",
    description: "Deploy event trigger: platform initiated",
  },
  "services.eventsTypeDeployStarted": {
    message: "Deploy iniciado",
    description: "Service activity type: deploy started",
  },
  "services.eventsTypeDeployFinished": {
    message: "Deploy concluído",
    description: "Service activity type: deploy finished",
  },
  "services.eventsTypeZeroDowntimeRedeployStarted": {
    message: "Reimplantação sem tempo de inatividade iniciada",
    description:
      "Service activity type: a Bex-initiated zero-downtime deploy started for a service",
  },
  "services.eventsTypeZeroDowntimeRedeployEnded": {
    message: "Reimplantação sem tempo de inatividade concluída",
    description:
      "Service activity type: a Bex-initiated zero-downtime deploy completed for a service",
  },
  "services.eventsRedeployTrigger.node_replacement": {
    message: "Nó substituído",
    description:
      "zero_downtime_redeploy_started trigger: drain-guard rolled the service off a node being replaced",
  },
  "services.eventsRedeployTrigger.node_consolidation": {
    message: "Nó removido pelo escalonamento automático",
    description:
      "zero_downtime_redeploy_started trigger: drain-guard rolled the service off a node marked for Cluster Autoscaler scale-down",
  },
  "services.eventsTypePipelineMinutesExhausted": {
    message: "Minutos de pipeline esgotados",
    description:
      "Service activity type: a build refused because the workspace used its pipeline minutes",
  },
  "services.eventsTypeEdgeCacheEnabled": {
    message: "Cache de borda ativado",
    description: "Service activity type: edge caching turned on",
  },
  "services.eventsTypeEdgeCacheDisabled": {
    message: "Cache de borda desativado",
    description: "Service activity type: edge caching turned off",
  },
  "services.eventsTypeEdgeCachePurged": {
    message: "Cache de borda limpo",
    description: "Service activity type: edge cache purged",
  },
  "services.eventsTypeImagePullFailed": {
    message: "Falha ao baixar a imagem",
    description: "Service activity type: image pull failed",
  },
  "services.eventsTypeArtifactFetchFailed": {
    message: "Falha ao obter artefato",
    description:
      "Service activity type: a build published by a build source could not be fetched",
  },
  "services.eventsTypeArtifactSourceChanged": {
    message: "Origem de build alterada",
    description:
      "Service activity type: the service's linked build source changed",
  },
  "services.eventsArtifactSourceChanged": {
    message: "Origem de build {from} → {to}",
    description:
      "Detail line on artifact_source_changed when switching between two build sources",
  },
  "services.eventsArtifactSourceLinked": {
    message: "Vinculado à origem de build {to}",
    description:
      "Detail line on artifact_source_changed when newly linked to a build source",
  },
  "services.eventsArtifactSourceUnlinked": {
    message: "Desvinculado da origem de build {from}",
    description:
      "Detail line on artifact_source_changed when detached from a build source",
  },
  "services.eventsBuildReference": {
    message: "Build {id}",
    description:
      "Build identifier shown on an artifact_fetch_failed activity row",
  },
  "services.eventsTypeSuspended": {
    message: "Suspenso",
    description: "Service activity type: service suspended",
  },
  "services.eventsTypeResumed": {
    message: "Retomado",
    description: "Service activity type: service resumed",
  },
  "services.eventsTypeRestarted": {
    message: "Servidor reiniciado",
    description: "Service activity type: service restarted",
  },
  "services.eventsTypeInstanceFailed": {
    message: "Falha na instância",
    description: "Service activity type: running instance became unhealthy",
  },
  "services.eventsTypeDiskUsageHigh": {
    message: "Uso de disco alto",
    description:
      "Service activity type: a service disk is over 80% full (service_disk_usage_high)",
  },
  "services.eventsTypeDiskUsageRecovered": {
    message: "Uso de disco normalizado",
    description:
      "Service activity type: a service disk dropped back below 80% (service_disk_usage_recovered)",
  },
  "services.eventsDiskUsage": {
    message:
      "Disco {disk} em {mountPath}: {usage}% usado (limite {threshold}%)",
    description: "Disk usage event detail line",
  },
  "services.eventsTypeHardwareFailure": {
    message: "Falha de hardware",
    description:
      "Service activity type: an instance was lost with the machine it ran on",
  },
  "services.eventsTypeServiceSuspended": {
    message: "Serviço suspenso",
    description: "Service activity type: suspension converged",
  },
  "services.eventsTypeServiceResumed": {
    message: "Serviço retomado",
    description: "Service activity type: resume converged",
  },
  "services.eventsTypeServiceHibernated": {
    message: "O serviço entrou em repouso",
    description:
      "Service activity type: free-tier idle auto-hibernation — deliberately worded apart from a user-driven suspension",
  },
  "services.eventsTypeServiceWoken": {
    message: "O serviço acordou",
    description:
      "Service activity type: a sleeping free-tier service woke on an incoming request",
  },
  "services.eventsTypeServiceRecovered": {
    message: "Serviço recuperado",
    description: "Service activity type: unhealthy service recovered",
  },
  "services.eventsTypePlanChanged": {
    message: "Tipo de instância alterado",
    description: "Service activity type: plan changed",
  },
  "services.eventsTypeInstanceCountChanged": {
    message: "Número de instâncias alterado",
    description: "Service activity type: manual scale",
  },
  "services.eventsTypeAutoscalingChanged": {
    message: "Configuração de escalonamento automático alterada",
    description: "Service activity type: autoscaling configuration changed",
  },
  "services.eventsTypeAutoscalingStarted": {
    message: "Escalonamento automático iniciado",
    description: "Service activity type: autoscaler began a replica change",
  },
  "services.eventsTypeAutoscalingEnded": {
    message: "Escalonamento automático concluído",
    description: "Service activity type: autoscaler replica change converged",
  },
  "services.eventsTypeBranchChanged": {
    message: "Branch alterado",
    description: "Service activity type: tracked Git branch changed",
  },
  "services.eventsTypeCommitIgnored": {
    message: "Commit ignorado",
    description: "Service activity type: matching Git push did not deploy",
  },
  "services.eventsTypeJobStarted": {
    message: "Job iniciado",
    description: "Service activity type: one-off job started",
  },
  "services.eventsTypeJobCanceled": {
    message: "Job cancelado",
    description: "Service activity type: one-off job canceled",
  },
  "services.eventsTypeJobRunEnded": {
    message: "Job concluído",
    description: "Service activity type: one-off job finished",
  },
  "services.eventsTypeBuildStarted": {
    message: "Build iniciado",
    description: "Service activity type: image build started",
  },
  "services.eventsTypeBuildEnded": {
    message: "Build concluído",
    description: "Service activity type: image build finished",
  },
  "services.eventsTypePreDeployStarted": {
    message: "Pré-deploy iniciado",
    description: "Service activity type: pre-deploy command started",
  },
  "services.eventsTypePreDeployEnded": {
    message: "Pré-deploy concluído",
    description: "Service activity type: pre-deploy command finished",
  },
  "services.eventsTypeBranchDeleted": {
    message: "Branch excluído",
    description: "Service activity type: tracked Git branch deleted",
  },
  "services.eventsTypeMaintenanceModeChanged": {
    message: "Configuração do modo de manutenção alterada",
    description: "Service activity type: maintenance mode setting changed",
  },
  "services.eventsTypeMaintenanceModeUriUpdated": {
    message: "URI do modo de manutenção atualizada",
    description: "Service activity type: maintenance mode URI changed",
  },
  "services.eventsInstanceReference": {
    message: "Instância {id}",
    description: "Instance identifier shown on a service event",
  },
  "services.eventsReplicaChange": {
    message: "Instâncias {from} → {to}",
    description:
      "Instance count from/to summary: an autoscaling start/end's replicas or an instance_count_changed event's counts",
  },
  "services.eventsAutoscalingChange": {
    message: "Escalonamento automático {from} → {to}",
    description:
      "autoscaling_config_changed detail line; from/to are services.eventsAutoscalingOff or services.eventsAutoscalingRange (w4/568)",
  },
  "services.eventsAutoscalingOff": {
    message: "Desativado",
    description: "One side of an autoscaling change: autoscaling off (w4/568)",
  },
  "services.eventsAutoscalingRange": {
    message: "{min}–{max} instâncias",
    description:
      "One side of an autoscaling change: its minimum–maximum instances (w4/568)",
  },
  "services.eventsAutoscalingRangeCPU": {
    message: "{min}–{max} instâncias, CPU {cpu}%",
    description:
      "One side of an autoscaling change: its instance range and CPU target (w4/m335)",
  },
  "services.eventsAutoscalingRangeMemory": {
    message: "{min}–{max} instâncias, memória {memory}%",
    description:
      "One side of an autoscaling change: its instance range and memory target (w4/m335)",
  },
  "services.eventsAutoscalingRangeCPUMemory": {
    message: "{min}–{max} instâncias, CPU {cpu}%, memória {memory}%",
    description:
      "One side of an autoscaling change: its instance range and both targets (w4/m335)",
  },
  "services.eventsPlanChange": {
    message: "Plano de computação {from} → {to}",
    description:
      "plan_changed detail line, by the compute plans' names (w4/568, w6/m483)",
  },
  "services.eventsBranchChange": {
    message: "Branch {from} → {to}",
    description: "Tracked branch from/to summary",
  },
  "services.eventsAutoDeployDisabledReason.setting_change": {
    message: "Desativado nas configurações",
    description:
      "auto_deploy_disabled reason: the auto-deploy setting was turned off (w4/m431)",
  },
  "services.eventsAutoDeployDisabledReason.rollback": {
    message:
      "Desativado por um rollback, para que pushes posteriores não o substituam",
    description:
      "auto_deploy_disabled reason: a dashboard rollback turned auto-deploy off (w4/m431)",
  },
  "services.eventsAutoDeployDisabledReason.manual_deploy": {
    message:
      "Desativado pelo deploy de um commit específico, para que pushes posteriores não o substituam",
    description:
      "auto_deploy_disabled reason: a deploy of a specific commit from the dashboard or a deploy hook turned auto-deploy off (w4/m431)",
  },
  "services.eventsBranchDeleted": {
    message: "Branch {branch} excluído — deploy automático desativado",
    description: "Deleted tracked branch summary",
  },
  "services.eventsStatus.succeeded": {
    message: "Bem-sucedido",
    description: "Lifecycle-step outcome badge: succeeded",
  },
  "services.eventsStatus.failed": {
    message: "Falhou",
    description: "Lifecycle-step outcome badge: failed",
  },
  "services.eventsStatus.canceled": {
    message: "Cancelado",
    description: "Lifecycle-step outcome badge: canceled",
  },
  "services.eventsReason.superseded": {
    message: "Um deploy mais recente substituiu este deploy.",
    description:
      "Deploy/build ended reason: a newer deploy superseded this one",
  },
  "services.eventsReason.image_pull_backoff": {
    message: "O runtime não conseguiu baixar a imagem configurada.",
    description: "Bounded public image-pull failure reason",
  },
  "services.eventsReason.readiness_failed": {
    message: "Uma instância em execução falhou no health check.",
    description:
      "Bounded public readiness failure reason; names the health check, as Settings and REST do (w4/m327)",
  },
  "services.eventsReason.root_directory": {
    message:
      "Nenhum arquivo alterado correspondeu ao diretório raiz do serviço.",
    description: "Commit ignored because of root-directory scoping",
  },
  "services.eventsReason.build_filter": {
    message:
      "Os arquivos alterados não passaram pelos filtros de build do serviço.",
    description: "Commit ignored because of build filters",
  },
  "services.eventsReason.skip_phrase": {
    message: "A mensagem do commit pediu que este deploy fosse ignorado.",
    description: "Commit ignored because of a Render skip phrase",
  },
  "services.eventsReason.non_zero_exit": {
    message: "A execução terminou com o status {exitCode}.",
    description:
      "Cron run failure reason: the container exited non-zero (w4/m114)",
  },
  "services.eventsReason.oom_killed": {
    message: "A execução ficou sem memória e foi interrompida.",
    description:
      "Cron run failure reason: the container was OOM-killed (w4/m114)",
  },
  "services.eventsReason.instance.non_zero_exit": {
    message:
      "O processo da instância terminou com o status {exitCode} e foi reiniciado.",
    description:
      "Instance failed reason for a running service: its process crashed (w8/m158)",
  },
  "services.eventsReason.instance.oom_killed": {
    message:
      "A instância ficou sem memória (limite de {memory}) e foi reiniciada.",
    description:
      "Instance failed reason for a running service: killed at its plan's memory limit (w8/m158)",
  },
  "services.eventsReason.instance.oom_killed_no_limit": {
    message: "A instância ficou sem memória e foi reiniciada.",
    description:
      "Instance failed reason for a running service: killed for memory, limit unknown (w8/m158)",
  },
  "services.eventsReason.early_exit": {
    message:
      "O processo da instância terminou e foi reiniciado. Espera-se que um serviço continue em execução.",
    description:
      "Instance failed reason for a running service: its process ended with status 0 (w8/m158)",
  },
  "services.eventsReason.evicted": {
    message: "A execução foi removida do nó antes de terminar.",
    description: "Cron run failure reason: the run's pod was evicted (w4/m114)",
  },
  "services.eventsReason.timed_out": {
    message: "A execução atingiu o limite de 12 horas e foi interrompida.",
    description:
      "Cron run failure reason: the run exceeded its deadline (w4/m114)",
  },
  "services.eventsTypeCronRunStarted": {
    message: "Execução do cron iniciada",
    description: "Service activity type: cron run started",
  },
  "services.eventsTypeCronRunFinished": {
    message: "Execução do cron concluída",
    description: "Service activity type: cron run finished",
  },
  "services.eventsTypeEnvVarsChanged": {
    message: "Variáveis de ambiente alteradas",
    description: "Service activity type: environment variables changed",
  },
  "services.eventsTypeEnvironmentChanged": {
    message: "Configuração do ambiente alterada",
    description:
      "Service activity type: environment variables and secret files changed",
  },
  "services.eventsTypeServiceMoved": {
    message: "Serviço movido",
    description:
      "Service activity type: the service was reassigned between projects or environments (service_moved) — distinct from the env-var fact above",
  },
  "services.eventsProjectChange": {
    message: "Projeto {from} → {to}",
    description:
      "service_moved detail line: previous and new project name (its id once it no longer exists), or the placement-none placeholder (w4/561)",
  },
  "services.eventsEnvironmentChange": {
    message: "Ambiente {from} → {to}",
    description:
      "service_moved detail line: previous and new environment name (its id once it no longer exists), or the placement-none placeholder (w4/561)",
  },
  "services.eventsPlacementNone": {
    message: "nenhum",
    description:
      "Placeholder for an absent side of a service_moved event (assigned from / removed to nothing) or of a plan_changed event (no plan before)",
  },
  "services.eventsTypeEnvGroupLinked": {
    message: "Grupo de ambiente vinculado",
    description: "Service activity type: environment group linked",
  },
  "services.eventsTypeEnvGroupUnlinked": {
    message: "Grupo de ambiente desvinculado",
    description: "Service activity type: environment group unlinked",
  },
  "services.eventsEnvGroupLinkedNamed": {
    message: "Grupo de ambiente {name} vinculado",
    description:
      "Service activity title for env_group_linked naming the group, by its name at the time (w4/563)",
  },
  "services.eventsEnvGroupUnlinkedNamed": {
    message: "Grupo de ambiente {name} desvinculado",
    description:
      "Service activity title for env_group_unlinked naming the group, by its name at the time (w4/563)",
  },
  "services.eventsCustomDomainAddedNamed": {
    message: "Domínio personalizado {name} adicionado",
    description:
      "Service activity title for custom_domain_added naming the domain's hostname (w4/m337)",
  },
  "services.eventsCustomDomainRemovedNamed": {
    message: "Domínio personalizado {name} removido",
    description:
      "Service activity title for custom_domain_removed naming the domain's hostname (w4/m337)",
  },
  "services.eventsCustomDomainVerifiedNamed": {
    message: "Domínio personalizado {name} verificado",
    description:
      "Service activity title for custom_domain_verified naming the domain's hostname (w4/m337)",
  },
  "services.eventsDisplayNameChange": {
    message: "Nome {from} → {to}",
    description:
      "display_name_changed detail line: the service's name before and after the rename (w4/m337)",
  },
  "services.eventsTypeAutoDeployChanged": {
    message: "Deploy automático atualizado",
    description: "Service activity type: auto-deploy setting changed",
  },
  "services.eventsTypeIdleTimeoutChanged": {
    message: "Timeout de inatividade atualizado",
    description: "Service activity type: idle timeout changed",
  },
  "services.eventsTypeDisplayNameChanged": {
    message: "Nome de exibição alterado",
    description: "Service activity type: display name changed",
  },
  "services.eventsTypeCustomDomainAdded": {
    message: "Domínio personalizado adicionado",
    description: "Service activity type: custom domain added",
  },
  "services.eventsTypeCustomDomainRemoved": {
    message: "Domínio personalizado removido",
    description: "Service activity type: custom domain removed",
  },
  "services.eventsTypeCustomDomainVerified": {
    message: "Domínio personalizado verificado",
    description: "Service activity type: custom domain verified",
  },
  "services.eventsTypeDiskCreated": {
    message: "Disco criado",
    description: "Service activity type: persistent disk created",
  },
  "services.eventsTypeDiskUpdated": {
    message: "Disco atualizado",
    description: "Service activity type: persistent disk updated",
  },
  "services.eventsTypeDiskDeleted": {
    message: "Disco excluído",
    description: "Service activity type: persistent disk deleted",
  },
  "services.eventsTypeDiskSnapshotRestored": {
    message: "Snapshot do disco restaurado",
    description:
      "Service activity type: persistent disk restored from a snapshot",
  },
  "services.eventsTypeNotificationsChanged": {
    message: "Notificações de falha atualizadas",
    description: "Service activity type: failure notification setting changed",
  },
  "services.eventsTypeSubdomainPolicyChanged": {
    message: "Subdomínio da plataforma atualizado",
    description: "Service activity type: platform subdomain policy changed",
  },
  "services.eventsTypeStaticSiteChanged": {
    message: "Configurações do site estático alteradas",
    description: "Service activity type: static-site configuration changed",
  },
  "services.eventsFilterType.autoDeployEnabled": {
    message: "Deploy automático ativado",
    description: "Events filter option for the auto deploy enabled event type",
  },
  "services.eventsFilterType.autoDeployDisabled": {
    message: "Deploy automático desativado",
    description: "Events filter option for the auto deploy disabled event type",
  },
  "services.eventsFilterType.autoDeployChanged": {
    message: "Deploy automático atualizado (legado)",
    description: "Events filter option for the auto deploy changed event type",
  },
  "services.eventsFilterType.publishPathChanged": {
    message: "Diretório de publicação alterado",
    description: "Events filter option for the publish path changed event type",
  },
  "services.eventsFilterType.routesChanged": {
    message: "Redirecionamentos e reescritas alterados",
    description: "Events filter option for the routes changed event type",
  },
  "services.eventsFilterType.headersChanged": {
    message: "Headers alterados",
    description: "Events filter option for the headers changed event type",
  },
  "services.eventsFilterType.rootDirectoryChanged": {
    message: "Diretório raiz alterado",
    description:
      "Events filter option for the root directory changed event type",
  },
  "services.eventsFilterType.dockerfilePathChanged": {
    message: "Caminho do Dockerfile alterado",
    description:
      "Events filter option for the dockerfile path changed event type",
  },
  "services.eventsFilterType.dockerContextChanged": {
    message: "Contexto de build do Docker alterado",
    description:
      "Events filter option for the docker context changed event type (w8/m128)",
  },
  "services.eventsFilterType.buildFilterChanged": {
    message: "Filtro de build alterado",
    description: "Events filter option for the build filter changed event type",
  },
  "services.eventsFilterType.commandsChanged": {
    message: "Comandos alterados",
    description: "Events filter option for the commands changed event type",
  },
  "services.eventsFilterType.sourceChanged": {
    message: "Origem alterada",
    description: "Events filter option for the source changed event type",
  },
  "services.eventsFilterType.preDeployCommandChanged": {
    message: "Comando de pré-deploy alterado",
    description:
      "Events filter option for the pre deploy command changed event type",
  },
  "services.eventsFilterType.maxShutdownDelayChanged": {
    message: "Atraso máximo de encerramento alterado",
    description:
      "Events filter option for the max shutdown delay changed event type",
  },
  "services.eventsFilterType.deployHookRegenerated": {
    message: "Deploy hook regenerado",
    description:
      "Events filter option for the deploy hook regenerated event type",
  },
  "services.eventsTypeBuildSettingsChanged": {
    message: "Configurações de build e deploy alteradas",
    description: "Service activity type: build or deploy configuration changed",
  },
  "services.eventsTypePortChanged": {
    message: "Porta alterada",
    description: "Service activity type: the service's listening port changed",
  },
  "services.eventsTypeServiceChanged": {
    message: "Configurações do serviço alteradas",
    description: "Fallback service activity type",
  },
  "services.eventsManualDeploy": {
    message: "Deploy manual",
    description: "Button to trigger a new deploy",
  },
  "services.deployMenuLatestCommit": {
    message: "Fazer deploy do último commit",
    description:
      "Manual Deploy dropdown item, repo-backed service: rebuild and redeploy from the branch's HEAD",
  },
  "services.deployMenuLatestImage": {
    message: "Fazer deploy da última imagem",
    description:
      "Manual Deploy dropdown item, image-backed service (no repo to rebuild from)",
  },
  "services.deployMenuClearCache": {
    message: "Limpar cache de build e fazer deploy",
    description:
      "Manual Deploy dropdown item (Render parity): rebuild without importing prior build-cache layers when registry caching is enabled; still exports a fresh cache for later deploys",
  },
  "services.deployMenuRestart": {
    message: "Reiniciar serviço",
    description:
      "Manual Deploy dropdown item: restart on the commit or image that is running; never deploys newer commits",
  },
  "services.deployMenuSpecificCommit": {
    message: "Fazer deploy de um commit específico",
    description:
      "Manual Deploy dropdown item, repo-backed service (Render parity): deploy a commit by SHA and turn auto-deploy off",
  },
  "services.deployCommitTitle": {
    message: "Fazer deploy de um commit específico",
    description: "Title of the dialog that deploys a commit by SHA",
  },
  "services.deployCommitBody": {
    message:
      "Faz o deploy do commit informado e desativa o deploy automático, para que pushes posteriores não o substituam. Você pode reativar o deploy automático em Configurações.",
    description:
      "Body of the deploy-a-specific-commit dialog. Render's dashboard disables automatic deploys when a specific commit is deployed",
  },
  "services.deployCommitLabel": {
    message: "SHA do commit",
    description:
      "Label of the commit SHA input in the deploy-a-specific-commit dialog",
  },
  "services.deployCommitInvalid": {
    message: "Informe um SHA de commit: de 7 a 40 caracteres hexadecimais.",
    description: "Inline error when the entered commit SHA is malformed",
  },
  "services.deployCommitConfirm": {
    message: "Fazer deploy do commit",
    description: "Confirm button of the deploy-a-specific-commit dialog",
  },
  "services.restartServiceSuccess": {
    message: "Reinicialização iniciada na versão em execução.",
    description: "Toast after the header's Restart service is accepted",
  },
  "services.restartServiceError": {
    message: "Não foi possível reiniciar o serviço.",
    description:
      "Toast when the header's Restart service fails and the server gave no reason",
  },
  "services.eventsCancelDeploy": {
    message: "Cancelar",
    description: "Button to cancel an in-progress deploy",
  },
  "services.eventsCancelConfirmTitle": {
    message: "Cancelar este deploy?",
    description: "Cancel deploy confirm dialog title",
  },
  "services.eventsCancelConfirmBody": {
    message:
      "O deploy em andamento será interrompido. Se um deploy anterior foi bem-sucedido, ele continua no ar; caso contrário, nada estará sendo servido. As alterações que você salvou continuam salvas e serão aplicadas no próximo deploy.",
    description:
      "Cancel deploy confirm dialog body when it is unknown whether an earlier deploy is live, so both outcomes are stated (w4/334)",
  },
  "services.eventsCancelConfirmBodyFirstDeploy": {
    message:
      "O deploy em andamento será interrompido. Este é o primeiro deploy do serviço, então nenhuma versão está no ar ainda — cancelar deixa o serviço sem nada sendo servido.",
    description:
      "Cancel deploy confirm dialog body — the service's first deploy, nothing is serving yet (w4/103)",
  },
  "services.eventsRollback": {
    message: "Rollback",
    description: "Button to roll back to a specific deploy",
  },
  "services.eventsRollbackConfirmTitle": {
    message: "Fazer rollback para este deploy?",
    description: "Rollback confirm dialog title",
  },
  "services.eventsRollbackConfirmBody": {
    message:
      "O serviço executará a imagem e a configuração retida deste deploy. As configurações salvas permanecem inalteradas e são aplicadas no próximo deploy padrão. O deploy automático será desativado. Deploys mais antigos sem configuração retida restauram somente a imagem.",
    description: "Rollback confirm dialog body",
  },
  "services.eventsRollbackConfirmBodyCommit": {
    message:
      "O serviço executará a imagem deste deploy (criada a partir de {commit}) e a configuração retida. As configurações salvas permanecem inalteradas e são aplicadas no próximo deploy padrão. O deploy automático será desativado. Deploys mais antigos sem configuração retida restauram somente a imagem.",
    description:
      "Rollback confirm dialog body naming the commit being restored",
  },
  "services.eventsRollbackConfirmBodyNoAutoDeploy": {
    message:
      "O serviço executará a imagem e a configuração retida deste deploy. As configurações salvas permanecem inalteradas e são aplicadas no próximo deploy padrão. Deploys mais antigos sem configuração retida restauram somente a imagem.",
    description:
      "Rollback confirm dialog body when the service's auto-deploy is already off (always so for an image-backed service), so no auto-deploy sentence (w4/554)",
  },
  "services.eventsRollbackConfirmBodyCommitNoAutoDeploy": {
    message:
      "O serviço executará a imagem deste deploy (criada a partir de {commit}) e a configuração retida. As configurações salvas permanecem inalteradas e são aplicadas no próximo deploy padrão. Deploys mais antigos sem configuração retida restauram somente a imagem.",
    description:
      "Rollback confirm dialog body naming the commit, when the service's auto-deploy is already off (w4/554)",
  },
  "services.eventsRollbackConfirmBodyStatic": {
    message:
      "O site será publicado novamente a partir deste deploy. As configurações salvas permanecem inalteradas e o deploy automático será desativado.",
    description:
      "Rollback confirm dialog body for a static site: it re-publishes the deploy's files, it has no image or instances (w4/m141)",
  },
  "services.eventsRollbackConfirmBodyStaticCommit": {
    message:
      "O site será publicado novamente a partir deste deploy, {commit}. As configurações salvas permanecem inalteradas e o deploy automático será desativado.",
    description:
      "Static-site rollback confirm dialog body naming the commit being re-published (w4/m141)",
  },
  "services.eventsRollbackConfirmTagPull": {
    message:
      "{image} é uma tag: o rollback baixa a imagem que ela indica agora, que pode ser diferente da que esta implantação executou, e falha se a tag não existir mais.",
    description:
      "Rollback confirm sentence for a deploy whose image is a registry tag, not a digest: Render pulls the tag again (render.com/docs/rollbacks, w4/m432)",
  },
  "services.eventsRollbackAria": {
    message: "Rollback para {target}",
    description:
      "Accessible name of a deploy row's Rollback control, naming the deploy it restores (w4/m141)",
  },
  "services.eventsCancelDeployAria": {
    message: "Cancelar deploy {target}",
    description:
      "Accessible name of a deploy row's Cancel control, naming the deploy (w4/m141)",
  },
  "services.eventsConfirmProceed": {
    message: "Continuar",
    description: "Confirm dialog proceed button",
  },
  "services.eventsConfirmCancel": {
    message: "Voltar",
    description: "Confirm dialog cancel button",
  },
  "services.triggerDeploySuccess": {
    message: "Deploy acionado.",
    description: "Toast after triggerDeploy succeeds",
  },
  "services.triggerDeployError": {
    message: "Não foi possível acionar o deploy.",
    description: "Toast after triggerDeploy fails",
  },
  "services.cancelDeploySuccess": {
    message: "Deploy cancelado.",
    description: "Toast after cancelDeploy succeeds",
  },
  "services.cancelDeployError": {
    message: "Não foi possível cancelar o deploy.",
    description: "Toast after cancelDeploy fails",
  },
  "services.rollbackSuccess": {
    message: "Rollback acionado.",
    description: "Toast after rollbackService succeeds",
  },
  "services.rollbackError": {
    message: "Não foi possível fazer o rollback.",
    description: "Toast after rollbackService fails",
  },
  "services.eventsStatusLive": {
    message: "No ar",
    description: "Deploy status: live",
  },
  "services.eventsStatusInProgress": {
    message: "Em andamento",
    description: "Deploy status: update_in_progress",
  },
  "services.eventsStatusFailed": {
    message: "Falhou",
    description: "Deploy status: update_failed",
  },
  "services.eventsStatusCanceled": {
    message: "Cancelado",
    description: "Deploy status: canceled",
  },
  "services.eventsPreDeployRunning": {
    message: "Comando de pré-deploy em execução",
    description: "Deploy row: the pre-deploy step is in progress",
  },
  "services.eventsPreDeploySucceeded": {
    message: "Comando de pré-deploy bem-sucedido",
    description: "Deploy row: the pre-deploy step passed",
  },
  "services.eventsPreDeployCanceled": {
    message: "Comando de pré-deploy cancelado",
    description:
      "Deploy row: the deploy ended (superseded or canceled) while its pre-deploy step was still running",
  },
  "services.eventsPreDeployFailed": {
    message: "Comando de pré-deploy falhou",
    description:
      "Deploy row: the pre-deploy step failed (distinct from a health-check failure)",
  },
  "services.eventsRolledBackFrom": {
    message: "Rollback de {target}",
    description: "Deploy row: provenance note when trigger=rollback",
  },
  "services.capLimitTitle": {
    message: "Limite de serviços atingido",
    description:
      "Alert title when the workspace's service creation cap is hit (w7/m9)",
  },
  "services.capLimitUpgrade": {
    message: "Fazer upgrade do plano",
    description: "Upgrade CTA button inside the cap-limit Alert (w7/m9)",
  },
  "services.networkingTitle": {
    message: "Rede",
    description: "Settings Networking card title (w7/m32)",
  },
  "services.networkingDescription": {
    message: "Restrinja o tráfego HTTP de entrada a estes CIDRs de origem.",
    description: "Settings Networking card description (w7/m32)",
  },
  "services.networkingScalePlanOnly": {
    message:
      "As regras de IP de entrada para Web Services e Static Sites exigem o plano Scale ou superior. As regras já definidas continuam em vigor e podem ser substituídas pela regra de permitir tudo.",
    description:
      "Hint replacing the editable IP rules below the Scale plan (w8/m137)",
  },
  "services.networkingClearAll": {
    message: "Permitir todo o tráfego",
    description:
      "Button replacing every inbound IP rule with the allow-all rule below the Scale plan (w8/m137, w8/m157)",
  },
  "services.networkingHint": {
    message:
      "Informe um bloco CIDR (ex.: 203.0.113.0/24) e pressione Adicionar. Sem regras, todo o tráfego de entrada é bloqueado; a regra 0.0.0.0/0 permite todos os IPs de origem.",
    description: "Hint below the CIDR list in the Networking card (w7/m32)",
  },
  "services.networkingOpen": {
    message: "Aberto a todos os IPs de origem",
    description:
      "Shown below the Scale plan when the service has only the allow-all rule (Render default, w7/m32, w8/m157)",
  },
  "services.networkingBlocked": {
    message:
      "Sem regras: todo o tráfego de entrada para este serviço está bloqueado. Adicione 0.0.0.0/0 para permitir todos os IPs de origem.",
    description:
      "Warning shown when a service's inbound IP rule list is empty, saved or about to be saved (w8/m157)",
  },
  "services.networkingAdd": {
    message: "Adicionar entrada à lista de permissões",
    description: "Button to add a CIDR to the draft list (w7/m32)",
  },
  "services.networkingEntryDescription": {
    message: "Descrição (opcional)",
    description: "Placeholder for a service allowlist entry description",
  },
  "services.networkingCIDR": {
    message: "Novo bloco CIDR",
    description: "Accessible label for the add-row CIDR input",
  },
  "services.networkingCIDRRule": {
    message: "Bloco CIDR da regra {number}",
    description: "Accessible label for an existing allowlist CIDR row",
  },
  "services.networkingDescriptionLabel": {
    message: "Descrição da nova regra",
    description: "Accessible label for the add-row description input",
  },
  "services.networkingDescriptionRule": {
    message: "Descrição da regra {number}",
    description: "Accessible label for an existing allowlist description row",
  },
  "services.networkingInvalid": {
    message: "Informe um bloco CIDR IPv4 ou IPv6 válido.",
    description: "Validation error for an invalid service allowlist CIDR",
  },
  "services.networkingSave": {
    message: "Salvar lista de permissões",
    description: "Button to persist the CIDR list (w7/m32)",
  },
  "services.networkingRemove": {
    message: "Remover {cidr}",
    description:
      "Accessible label on the trash icon next to a CIDR tag (w7/m32)",
  },
  "services.networkingMoveUp": {
    message: "Mover {cidr} para cima",
    description: "Accessible label to move an allowlist entry earlier",
  },
  "services.networkingMoveDown": {
    message: "Mover {cidr} para baixo",
    description: "Accessible label to move an allowlist entry later",
  },
  "services.networkingSaved": {
    message: "Lista de permissões de IP atualizada",
    description: "Toast on successful setServiceIpAllowList mutation (w7/m32)",
  },
  "services.networkingError": {
    message: "Falha ao atualizar a lista de permissões de IP. Tente novamente.",
    description: "Toast on failed setServiceIpAllowList mutation (w7/m32)",
  },
  "services.outboundIpsTitle": {
    message: "IPs de saída",
    description: "Settings card title for service egress IPs (w8/010)",
  },
  "services.outboundIpsDescription": {
    message:
      "Endereços de saída compartilhados deste serviço. São os IPs públicos atuais dos nós do pool do tenant e podem mudar conforme o autoscaler adiciona ou remove nós.",
    description:
      "Outbound IPs card helper — truthful shared-pool semantics (w8/010)",
  },
  "services.outboundIpsEmpty": {
    message:
      "Nenhum IP de saída informado ainda (comum em clusters locais sem ExternalIPs nos nós).",
    description: "Honest empty state when outboundIps.ips is empty (w8/010)",
  },
  "services.outboundIpsCopy": {
    message: "Copiar {ip}",
    description:
      "Accessible label on the copy button for one outbound IP (w8/010)",
  },
  "services.outboundIpsCopied": {
    message: "Copiado",
    description: "Toast after copying an outbound IP (w8/010)",
  },
  "services.outboundIpsCopyError": {
    message: "Não foi possível copiar",
    description: "Toast when clipboard write fails for an outbound IP (w8/010)",
  },
  "services.environmentPageDescription": {
    message:
      "Tudo o que este serviço lê em tempo de execução. Os valores ficam ocultos até você revelar um.",
    description: "Service Environment page introduction",
  },
  "services.environmentEdit": {
    message: "Editar",
    description: "Enter the coherent service environment draft",
  },
  "services.environmentMaskedValue": {
    message: "Valor secreto mascarado",
    description: "Accessible label for an unrevealed secret",
  },
  "services.environmentUnchangedMasked": {
    message: "Sem alteração (mascarado)",
    description: "Placeholder for an opaque unchanged draft value",
  },
  "services.environmentDuplicateKey": {
    message: "Cada chave de variável deve ser única.",
    description: "Draft validation for a duplicate environment key",
  },
  "services.environmentValueRequired": {
    message: "Informe um valor para esta nova variável.",
    description: "Draft validation for a new variable without a value",
  },
  "services.environmentValueNul": {
    message:
      "Um valor não pode conter um byte NUL, que o ambiente de um processo não consegue transportar. Use um arquivo secreto para conteúdo binário.",
    description:
      "Draft validation under an environment variable value with a NUL byte (w21/047)",
  },
  "services.environmentValueTooLarge": {
    message:
      "Uma entrada KEY=value pode ter no máximo 131.071 bytes: um processo não consegue iniciar com mais que isso. Use um arquivo secreto para conteúdos grandes.",
    description:
      "Draft validation under an environment variable whose KEY=value entry is over the process limit; the server counts UTF-8 bytes, so a Chinese character is 3 (w21/047)",
  },
  "services.environmentPortInvalid": {
    message:
      "PORT deve ser um número inteiro de 1 a 65535 e não pode ser 18012, 18013 ou 19099, que a plataforma reserva.",
    description:
      "Draft validation under a PORT environment variable the operator would ignore (w21/047)",
  },
  "services.environmentLimit": {
    message:
      "As variáveis de ambiente são limitadas a 500 e 512 KiB no total por serviço ou grupo.",
    description:
      "Draft validation when the environment variables would exceed bex-api's per-service quota",
  },
  "services.environmentStagedDelete": {
    message: "Será removida",
    description: "Badge on a staged environment deletion",
  },
  "services.environmentUndo": {
    message: "Desfazer",
    description: "Undo a staged environment deletion",
  },
  "services.environmentUnsavedTitle": {
    message: "Alterações não salvas",
    description: "Combined draft save bar heading",
  },
  "services.environmentRestoredTitle": {
    message: "Alterações não salvas restauradas",
    description:
      "Heading of the banner shown when an unsaved env draft is recovered in this tab, after a reload or a session-expiry sign-in (w3/m80)",
  },
  "services.environmentRestoredBody": {
    message:
      "Suas edições de ambiente não salvas foram restauradas. Revise-as e depois salve ou descarte.",
    description:
      "Body of the restored-draft banner. Names no cause: an ordinary reload restores the draft as a sign-in does (w4/337)",
  },
  "services.environmentRestoredDiscard": {
    message: "Descartar alterações restauradas",
    description: "Button that throws away a recovered env draft",
  },
  "services.environmentUnsavedVariables_one": {
    message: "{count} variável alterada",
    description:
      "Draft variable-operation count, composed with the file count at the call site (singular)",
  },
  "services.environmentUnsavedVariables_many": {
    message: "{count} de variáveis alteradas",
    description:
      "Draft variable-operation count, composed with the file count at the call site (plural)",
  },
  "services.environmentUnsavedVariables_other": {
    message: "{count} variáveis alteradas",
    description:
      "Draft variable-operation count, composed with the file count at the call site (plural)",
  },
  "services.environmentUnsavedFiles_one": {
    message: "{count} arquivo alterado",
    description:
      "Draft file-operation count, composed with the variable count at the call site (singular)",
  },
  "services.environmentUnsavedFiles_many": {
    message: "{count} de arquivos alterados",
    description:
      "Draft file-operation count, composed with the variable count at the call site (plural)",
  },
  "services.environmentUnsavedFiles_other": {
    message: "{count} arquivos alterados",
    description:
      "Draft file-operation count, composed with the variable count at the call site (plural)",
  },
  "services.environmentSaveOptions": {
    message: "Opções para salvar o ambiente",
    description: "Accessible label for the split save menu",
  },
  "services.environmentSaveOnly": {
    message: "Somente salvar",
    description: "Persist and project without rolling the service",
  },
  "services.environmentSaveDeploy": {
    message: "Salvar e fazer deploy",
    description: "Persist and roll the current image once",
  },
  "services.environmentSaveRebuild": {
    message: "Salvar, refazer o build e fazer deploy",
    description: "Persist then start one source build and deploy",
  },
  "services.environmentSaveOnlySuccess": {
    message:
      "Ambiente salvo sem deploy. Use um deploy padrão para aplicar as alterações pendentes.",
    description:
      "Save-only success feedback while the operator checks saved/runtime differences; does not assert that a no-op or revert has pending changes",
  },
  "services.environmentSaveDeploySuccess": {
    message: "Ambiente salvo e deploy iniciado",
    description: "Toast after save-and-deploy succeeds",
  },
  "services.environmentSavedRefreshFailed": {
    message:
      "Ambiente salvo, mas esta página não conseguiu atualizar. Recarregue para conferir os valores mais recentes e o status do deploy.",
    description:
      "A committed environment patch must not be retried because a follow-up read failed",
  },
  "services.environmentSaveError": {
    message: "Não foi possível salvar o ambiente. Seu rascunho continua aqui.",
    description: "Batch environment save failure",
  },
  "services.environmentDraftStale": {
    message:
      "Este grupo de ambiente pode ter mudado desde que seu rascunho começou, então o rascunho não foi salvo. Copie o que precisar, descarte o rascunho e edite novamente para partir da versão mais recente.",
    description:
      "Refusal when a group draft's base revision is no longer current (ENV_GROUP_REVISION_CONFLICT) or predates revision tracking (w4/m161)",
  },
  "services.environmentSavedDeployFailedTitle": {
    message: "Configuração salva; implantação incompleta",
    description: "Partial rebuild failure heading",
  },
  "services.environmentSavedDeployFailedBody": {
    message:
      "Suas alterações de ambiente foram armazenadas. Tente novamente somente a implantação incompleta quando estiver pronto.",
    description: "Partial rebuild failure recovery explanation",
  },
  "services.environmentRetryDeploy": {
    message: "Tentar implantação novamente",
    description: "Retry only the failed deploy phase",
  },
  "services.environmentDiscardTitle": {
    message: "Descartar alterações de ambiente?",
    description: "Dirty navigation guard title",
  },
  "services.environmentDiscardBody": {
    message: "Estas alterações não foram salvas e serão perdidas se você sair.",
    description: "Dirty navigation guard explanation",
  },
  "services.environmentKeepEditing": {
    message: "Continuar editando",
    description: "Cancel dirty navigation",
  },
  "services.environmentDiscard": {
    message: "Descartar e sair",
    description: "Confirm dirty navigation",
  },
  "services.envReveal": {
    message: "Revelar",
    description: "Reveal one masked environment value",
  },
  "services.envRevealOne": {
    message: "Revelar {name}",
    description:
      'aria-label of the per-row Reveal button for one env var or secret file; starts with the visible text "Reveal" (w4/499)',
  },
  "services.envHideOne": {
    message: "Ocultar {name}",
    description:
      'aria-label of the per-row Hide button for one env var or secret file; starts with the visible text "Hide" (w4/499)',
  },
  "services.envHide": {
    message: "Ocultar",
    description: "Mask one revealed environment value",
  },
  "services.envCopy": {
    message: "Copiar variáveis de ambiente",
    description: "Copy the complete dotenv export",
  },
  "services.envDownload": {
    message: "Baixar .env",
    description: "Download the complete dotenv export",
  },
  "services.envCopySuccess": {
    message: "Ambiente copiado",
    description: "Toast after the complete dotenv export is copied",
  },
  "services.envCopyOne": {
    message: "Copiar {name}",
    description:
      "aria-label of the per-row copy button for one env var or secret file — distinct from the bulk envCopy dotenv export",
  },
  "services.envCopyOneSuccess": {
    message: "{name} copiado",
    description:
      "Toast after one env var value or secret file's contents is copied — distinct from the bulk envCopySuccess",
  },
  "services.envAddVariable": {
    message: "Adicionar variável",
    description: "Add one blank draft variable",
  },
  "services.envAddGenerated": {
    message: "Segredo gerado",
    description: "Add one previewable generated secret",
  },
  "services.envImport": {
    message: "Importar de .env",
    description: "Open the dotenv import dialog",
  },
  "services.envImportTitle": {
    message: "Importar variáveis de ambiente",
    description: "Dotenv import dialog title",
  },
  "services.envImportDescription": {
    message:
      "Cole atribuições dotenv ou escolha um arquivo de texto. Os valores importados ficam neste rascunho até você salvar.",
    description: "Dotenv import dialog explanation",
  },
  "services.envImportTextLabel": {
    message: "Conteúdo do dotenv",
    description: "Accessible label for the dotenv import textarea",
  },
  "services.envImportPlaceholder": {
    message: "API_URL=https://example.com\nSECRET=replace-me",
    description: "Dotenv import textarea placeholder",
  },
  "services.envImportChooseFile": {
    message: "Escolher um arquivo",
    description: "Dotenv file picker button",
  },
  "services.envImportAdd": {
    message: "Adicionar variáveis",
    description: "Stage parsed dotenv variables",
  },
  "services.envImportLineError": {
    message: "A linha {line} não é uma atribuição KEY=value.",
    description: "Line-numbered dotenv parse error",
  },
  "services.envImportKeyError": {
    message:
      "Linha {line}: o nome deve ter até 253 letras, dígitos, pontos, hifens e underscores.",
    description:
      "Dotenv import error: the variable name on a line is invalid (w4/159)",
  },
  "services.envImportQuoteError": {
    message: "As aspas abertas na linha {line} nunca são fechadas.",
    description: "Dotenv import error: a quoted value never closes (w4/159)",
  },
  "services.envImportTrailingError": {
    message: "A linha {line} tem texto após as aspas de fechamento.",
    description:
      "Dotenv import error: text after a quoted value's closing quote (w4/159)",
  },
  "services.envImportReading": {
    message: "Lendo arquivo…",
    description:
      "Status while a chosen dotenv file is being read; Add waits for it",
  },
  "services.envImportFileError": {
    message: "Escolha um arquivo de texto legível de no máximo 1 MiB.",
    description: "Dotenv file read or size error",
  },
  "services.secretFileUpload": {
    message: "Enviar arquivos",
    description: "Stage multiple local text files",
  },
  "services.secretFileSkippedTitle_one": {
    message: "{count} arquivo foi ignorado:",
    description:
      "Secret-file upload: heading over the one skipped file (w4/628)",
  },
  "services.secretFileSkippedTitle_many": {
    message: "{count} de arquivos foram ignorados:",
    description: "Secret-file upload: heading over the skipped files (w4/628)",
  },
  "services.secretFileSkippedTitle_other": {
    message: "{count} arquivos foram ignorados:",
    description: "Secret-file upload: heading over the skipped files (w4/628)",
  },
  "services.secretFileSkippedName": {
    message:
      "o nome só pode usar letras, dígitos, '.', '-' e '_' (até 253 caracteres).",
    description: "Secret-file upload: why a file was skipped — its name",
  },
  "services.secretFileSkippedSize": {
    message: "ele é maior que 1 MiB.",
    description: "Secret-file upload: why a file was skipped — its size",
  },
  "services.secretFileSkippedEncoding": {
    message:
      "ele não é texto UTF-8. Para um arquivo binário (keystore, .p12, certificado DER), envie-o codificado em base64 e decodifique-o na inicialização, ex.: base64 -d /etc/secrets/keystore.b64 > /tmp/keystore.p12.",
    description: "Secret-file upload: why a file was skipped — not UTF-8 text",
  },
  "services.secretFileSkippedDuplicate": {
    message: "já existe um arquivo com este nome na lista.",
    description: "Secret-file upload: why a file was skipped — duplicate name",
  },
  "services.secretFileDuplicateName": {
    message: "Cada nome de arquivo secreto deve ser único.",
    description: "Draft validation for a duplicate file name",
  },
  "services.secretFileContentRequired": {
    message: "Adicione o conteúdo deste novo arquivo.",
    description: "Draft validation for missing new-file content",
  },
  "services.secretFileLimit": {
    message:
      "Os arquivos secretos são limitados a 500 por serviço ou grupo, e o tamanho combinado deles a 1 MiB.",
    description:
      "Draft validation when the secret files would exceed bex-api's per-service quota: 500 files, 1 MiB of contents (names not counted), Render's 1 MB (w4/m363)",
  },
  "services.secretFileViewContent": {
    message: "Ver conteúdo",
    description: "Reveal or inspect secret-file contents in a dialog",
  },
  "services.secretFileViewContentOne": {
    message: "Ver conteúdo de {name}",
    description:
      "Accessible name for View contents once the file has a name (m101/t013)",
  },
  "services.secretFileEditContent": {
    message: "Editar conteúdo",
    description: "Edit staged secret-file contents in a dialog",
  },
  "services.secretFileEditContentOne": {
    message: "Editar conteúdo de {name}",
    description:
      "Accessible name for Edit contents once the file has a name (m101/t013)",
  },
  "services.secretFileContentDialogTitle": {
    message: "Conteúdo de {name}",
    description: "Secret-file content dialog title",
  },
  "services.secretFileContentDialogDescription": {
    message:
      "Este arquivo é montado como somente leitura em /etc/secrets após o próximo deploy.",
    description: "Secret-file content dialog explanation",
  },
  "services.secretFileContentDialogDescriptionDocker": {
    message:
      "Este arquivo é montado como somente leitura em /etc/secrets após o próximo deploy. Um build Docker também o encontra na raiz do contexto de build: liste-o em .dockerignore para mantê-lo fora da sua imagem.",
    description:
      "Secret-file content dialog explanation for a Docker-runtime service: the build context also receives the file, as on Render (w8/253)",
  },
  "services.secretFileLoadingContent": {
    message: "Carregando conteúdo…",
    description: "Secret-file content reveal progress",
  },
  "services.secretFileContentDone": {
    message: "Concluir",
    description: "Close and stage secret-file content edits",
  },
  "services.secretFileUntitled": {
    message: "arquivo sem nome",
    description: "Fallback title for a new unnamed secret file",
  },
  "services.secretFileDelete": {
    message: "Excluir arquivo secreto",
    description: "Accessible label for a staged secret-file deletion",
  },
  "services.secretFileDeleteOne": {
    message: "Excluir arquivo secreto {name}",
    description:
      "Accessible name for Delete secret file once the file has a name (m101/t013)",
  },
  "services.envGroupsLinkedTitle": {
    message: "Grupos de ambiente vinculados",
    description: "Service-side environment group card title",
  },
  "services.envGroupsLinkedCount": {
    message: "Vinculados ({count})",
    description: "Count of groups linked to this service",
  },
  "services.envGroupsAvailableCount": {
    message: "Disponíveis para vincular ({count})",
    description: "Count of groups available to link",
  },
  "services.envGroupsNoneLinked": {
    message: "Nenhum grupo de ambiente está vinculado a este serviço.",
    description: "Service group empty linked state",
  },
  "services.envGroupsAvailableTitle": {
    message: "Disponíveis para vincular",
    description:
      "Available-groups heading while the service's environment is still resolving (no count yet)",
  },
  "services.envGroupsScopeLoading": {
    message: "Verificando quais grupos compartilham o ambiente deste serviço…",
    description:
      "Available groups wait for the service's environment scope (w4/217)",
  },
  "services.envGroupsOrderReading": {
    message: "Confirmando qual grupo prevalece…",
    description:
      "Link order is being re-read after a link change; no winner is claimed until it lands (w4/322)",
  },
  "services.envGroupsOrderFailed": {
    message:
      "Vinculado, mas não foi possível confirmar qual grupo prevalece. Recarregue a ordem para vê-la.",
    description:
      "The post-write Server read failed; the write stands, precedence stays unclaimed until a retry succeeds (w4/322)",
  },
  "services.envGroupsScopeError": {
    message:
      "Não foi possível verificar quais grupos este serviço pode vincular.",
    description:
      "Scope index failed; nothing is offered until a retry succeeds (w4/217)",
  },
  "services.envGroupsScopeEnvironmentMissing": {
    message:
      "O ambiente deste serviço não está na lista de ambientes carregada, então nenhum grupo é oferecido para vincular. Tente novamente para atualizá-la.",
    description:
      "The service reports an Environment the authorized scope index lacks even after a refresh; never falls back to Workspace (w4/220)",
  },
  "services.envGroupsNoneInScope": {
    message:
      "Nenhum grupo desvinculado compartilha o ambiente deste serviço. Crie um aqui ou mova um grupo para este ambiente.",
    description:
      "Unlinked groups exist, but none in this service's environment scope (w4/217)",
  },
  "services.envGroupsNoneAvailable": {
    message: "Todos os grupos de ambiente do workspace já estão vinculados.",
    description: "Service group empty available state",
  },
  "services.envGroupsNoneAvailableCreate": {
    message:
      "Ainda não há grupos de ambiente no workspace. Crie um com variáveis, arquivos e este serviço já selecionado.",
    description: "Service group create-first empty state",
  },
  "services.cronEveryMinute": {
    message: "A cada minuto",
    description: "Cron schedule preview phrase (w4/m188): fires every minute",
  },
  "services.cronEveryNMinutes": {
    message: "A cada {n} minutos",
    description:
      "Cron schedule preview phrase (w4/m188): uniform minute step; n is the step",
  },
  "services.cronEveryHour": {
    message: "A cada hora",
    description:
      "Cron schedule preview phrase (w4/m188): fires at minute 0 of every hour",
  },
  "services.cronEveryNHours": {
    message: "A cada {n} horas",
    description:
      "Cron schedule preview phrase (w4/m188): uniform hour step; n is the step",
  },
  "services.cronHourlyAtMinute": {
    message: "A cada hora, no minuto {minute}",
    description:
      "Cron schedule preview phrase (w4/m188): fires once an hour at the given minute",
  },
  "services.cronDailyAt": {
    message: "Todos os dias às {time}",
    description: "Cron schedule preview phrase (w4/m188): daily at HH:MM (UTC)",
  },
  "services.cronWeeklyAt": {
    message: "Semanalmente, {day} às {time}",
    description:
      "Cron schedule preview phrase (w4/m188): weekly; day is a services.cronDay* weekday name, time is HH:MM (UTC)",
  },
  "services.cronMonthlyAt": {
    message: "No dia {day} de todo mês às {time}",
    description:
      "Cron schedule preview phrase (w4/m188): monthly on a day of the month at HH:MM (UTC)",
  },
  "services.cronRunsEveryMinute": {
    message: "a cada minuto",
    description:
      'Cron job header, after "Runs <command>": the schedule every minute (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronRunsEveryNMinutes": {
    message: "a cada {n} minutos",
    description:
      'Cron job header, after "Runs <command>": every {n} minutes (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronRunsEveryHour": {
    message: "a cada hora",
    description:
      'Cron job header, after "Runs <command>": every hour on the hour (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronRunsHourlyAtMinute": {
    message: "aos {minute} minutos de cada hora",
    description:
      'Cron job header, after "Runs <command>": hourly at minute {minute} (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronRunsEveryNHours": {
    message: "aos {minute} minutos da hora, a cada {n} horas",
    description:
      'Cron job header, after "Runs <command>": at minute {minute}, every {n} hours (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronRunsDailyAt": {
    message: "às {time} UTC",
    description:
      'Cron job header, after "Runs <command>": daily at {time} ("09:00 AM" in English, the 24-hour clock elsewhere), UTC (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronRunsWeeklyAt": {
    message: "às {time} UTC ({day})",
    description:
      'Cron job header, after "Runs <command>": weekly on {day} (a services.cronDay* name) at {time}, UTC (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronRunsMonthlyAt": {
    message: "às {time} UTC, no dia {day} do mês",
    description:
      'Cron job header, after "Runs <command>": monthly on day {day} at {time}, UTC (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronDaySunday": {
    message: "domingo",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayMonday": {
    message: "segunda-feira",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayTuesday": {
    message: "terça-feira",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayWednesday": {
    message: "quarta-feira",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayThursday": {
    message: "quinta-feira",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayFriday": {
    message: "sexta-feira",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDaySaturday": {
    message: "sábado",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.environmentEditingTitle": {
    message: "Editando o ambiente",
    description:
      "Edit bar title while the environment draft is open and holds no change yet (the bar says Unsaved changes once it does)",
  },
  "services.environmentNoChanges": {
    message: "Ainda não há alterações.",
    description:
      "Edit bar line while the open environment draft holds no change: what to do next",
  },
  "services.envAddOptions": {
    message: "Mais formas de adicionar uma variável",
    description:
      "Accessible name of the chevron beside Add variable that opens the other ways to add (generated secret, .env import)",
  },
  "services.secretFilesReloadWithVariables": {
    message:
      "Eles são recarregados com as variáveis quando você tenta de novo acima.",
    description:
      "Secret Files card line when both lists failed to load: the variables card's Try again reloads the files too, so this card has no button of its own",
  },
  "services.createSubmitWeb": {
    message: "Fazer deploy do Web Service",
    description:
      "Create-wizard submit button for a web service (Render's 'Deploy web service')",
  },
  "services.createSubmitPrivate": {
    message: "Fazer deploy do Private Service",
    description: "Create-wizard submit button for a private service",
  },
  "services.createSubmitWorker": {
    message: "Fazer deploy do Background Worker",
    description: "Create-wizard submit button for a background worker",
  },
  "services.createSubmitCron": {
    message: "Fazer deploy do Cron Job",
    description:
      "Create-wizard submit button for a cron job (Render's 'Deploy cron job')",
  },
  "services.createSubmitStatic": {
    message: "Fazer deploy do Static Site",
    description: "Create-wizard submit button for a static site",
  },
  "services.createSourceEdit": {
    message: "Editar",
    description:
      "Button on the chosen repository's summary row that reopens the source picker",
  },
  "services.createSourceEditLabel": {
    message: "Alterar a origem",
    description:
      "Accessible name of the Edit button on the chosen repository's summary row ({repo} = owner/name)",
  },
  "services.createBlockedSource": {
    message: "Escolha uma origem para continuar.",
    description:
      "Create-wizard action bar: why Deploy is held while no source is chosen",
  },
  "services.createFieldNameDescription": {
    message: "Um nome único para este serviço.",
    description: "Create-wizard Name row description",
  },
  "services.createFieldRuntimeDescription": {
    message: "Escolha o ambiente de runtime deste serviço.",
    description: "Create-wizard Language/runtime row description",
  },
  "services.createFieldBranchDescription": {
    message: "O branch do Git a ser compilado e implantado.",
    description: "Create-wizard Branch row description",
  },
  "services.createFieldBuildCommandDescription": {
    message: "Este comando compila seu app antes de cada deploy.",
    description:
      "Create-wizard Build Command row description (native runtimes)",
  },
  "services.createFieldStartCommandDescription": {
    message: "Este comando inicia seu app a cada deploy.",
    description: "Create-wizard Start Command row description",
  },
  "services.createFieldDockerCommandDescription": {
    message:
      "Opcional: substitua o CMD e o ENTRYPOINT da imagem por outro comando para iniciar o serviço.",
    description: "Create-wizard Docker Command row description (Advanced)",
  },
  "services.createFieldPlanDescription": {
    message:
      "A instância em que seu serviço roda. Você pode alterá-la a qualquer momento.",
    description: "Create-wizard compute plan row description",
  },
  "services.createEnvVarsDescription": {
    message:
      "Defina configurações e segredos específicos do ambiente (como chaves de API) e leia esses valores no seu código.",
    description: "Create-wizard Environment Variables card description",
  },
  "services.createBlockedPublishPath": {
    message: "Insira um diretório de publicação para continuar.",
    description:
      "Create-wizard action bar: Deploy is held until a static site has a publish directory",
  },
  "services.createBlockedSchedule": {
    message: "Insira um agendamento válido para continuar.",
    description:
      "Create-wizard action bar: Deploy is held until a cron job has a valid schedule",
  },
  "services.createBlockedCommands": {
    message: "Insira os comandos de build e de início para continuar.",
    description:
      "Create-wizard action bar: Deploy is held until the native runtime's build and start commands are filled",
  },
  "services.createBlockedPort": {
    message: "Insira uma porta válida para continuar.",
    description:
      "Create-wizard action bar: Deploy is held while the port is out of range",
  },
};

export default ptServices;
