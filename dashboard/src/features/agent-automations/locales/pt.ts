import type { TranslationEntry } from "@/i18n";

const ptAgentAutomations: Record<string, TranslationEntry> = {
  "agentAutomations.pageTitle": {
    message: "Automações",
    description:
      "Page title and sidebar label of the scheduled agent automations page",
  },
  "agentAutomations.detailTitle": {
    message: "Automação",
    description: "Browser tab title of one automation's page",
  },
  "agentAutomations.pageDescription": {
    message:
      "Inicie uma sessão de agente em um agendamento. Cada execução usa a tarefa e as configurações que você salvar aqui.",
    description: "Automations list page description",
  },
  "agentAutomations.create": {
    message: "Criar automação",
    description:
      "Create automation button on the list page and in the empty state",
  },
  "agentAutomations.createTitle": {
    message: "Criar automação",
    description: "Heading of the create form",
  },
  "agentAutomations.editTitle": {
    message: "Editar automação",
    description: "Heading of the edit form",
  },
  "agentAutomations.createSubmit": {
    message: "Criar automação",
    description: "Create form submit button",
  },
  "agentAutomations.editSubmit": {
    message: "Salvar alterações",
    description: "Edit form submit button",
  },
  "agentAutomations.createDenied": {
    message:
      "Somente membros com a função de desenvolvedor ou superior podem criar automações.",
    description: "Create page notice for a member who cannot create",
  },
  "agentAutomations.backToList": {
    message: "Voltar para automações",
    description: "Accessible label of the back link to the automations list",
  },
  "agentAutomations.backToAutomation": {
    message: "Voltar para a automação",
    description: "Button returning from the edit page to the automation's page",
  },
  "agentAutomations.tabMine": {
    message: "Minhas",
    description: "List tab: automations the viewer created",
  },
  "agentAutomations.tabAll": {
    message: "Todas",
    description: "List tab: every automation in the workspace",
  },
  "agentAutomations.emptyMineTitle": {
    message: "Você ainda não tem automações",
    description: "Empty state title on the Mine tab",
  },
  "agentAutomations.emptyAllTitle": {
    message: "Este workspace ainda não tem automações",
    description: "Empty state title on the All tab",
  },
  "agentAutomations.emptyBody": {
    message:
      "Uma automação inicia uma sessão de agente em um agendamento, como todo dia útil às 09:00, com a tarefa, o repositório e os limites que você escolher.",
    description: "Empty state explanation",
  },
  "agentAutomations.loadMore": {
    message: "Carregar mais",
    description: "Load the next page of automations or runs",
  },
  "agentAutomations.loadFailed": {
    message: "Não foi possível carregar as automações.",
    description: "Load error",
  },
  "agentAutomations.retry": {
    message: "Tentar novamente",
    description: "Retry button on a load error",
  },
  "agentAutomations.forbidden": {
    message:
      "Somente membros com a função de desenvolvedor ou superior podem usar automações.",
    description: "Load error for a member without access",
  },
  "agentAutomations.notFound": {
    message: "Esta automação não existe ou foi excluída.",
    description: "Load error for a missing automation",
  },
  "agentAutomations.noTrigger": {
    message: "Sem agendamento",
    description: "Row text when an automation has no schedule",
  },
  "agentAutomations.nextRun": {
    message: "Próxima execução",
    description: "Label before the next run time",
  },
  "agentAutomations.lastRun": {
    message: "Última execução",
    description: "Label before the last run time",
  },
  "agentAutomations.neverRun": {
    message: "nunca",
    description: "Last run value when it has not run",
  },
  "agentAutomations.lastSession": {
    message: "Ver sessão",
    description: "Link to the session the last run started",
  },
  "agentAutomations.pauseLabel": {
    message: "Pausar {name}",
    description: "Accessible name of the switch that pauses an automation",
  },
  "agentAutomations.resumeLabel": {
    message: "Retomar {name}",
    description: "Accessible name of the switch that resumes an automation",
  },
  "agentAutomations.pausedBadge": {
    message: "Pausada",
    description: "Badge on an automation a person paused",
  },
  "agentAutomations.ended": {
    message: "Encerrada",
    description: "Badge on an automation with no future run",
  },
  "agentAutomations.active": {
    message: "Ativa",
    description: "Status of a running automation",
  },
  "agentAutomations.paused.repeated_failures": {
    message: "Pausada: falhas repetidas",
    description: "Badge: paused by the platform after repeated failed runs",
  },
  "agentAutomations.paused.creator_lost_access": {
    message: "Pausada: o criador perdeu o acesso",
    description:
      "Badge: paused because the creator can no longer start sessions",
  },
  "agentAutomations.paused.template_invalid": {
    message: "Pausada: modelo inválido",
    description:
      "Badge: paused because the session template no longer validates",
  },
  "agentAutomations.paused.unknown": {
    message: "Pausada pela plataforma",
    description: "Badge for a pause reason this version does not know",
  },
  "agentAutomations.pausedExplain.repeated_failures": {
    message:
      "Esta automação foi pausada após {failures} execuções com falha seguidas. Confira o histórico de execuções, corrija a causa e depois retome-a.",
    description: "Explanation on the detail page for a repeated-failures pause",
  },
  "agentAutomations.pausedExplain.creator_lost_access": {
    message:
      "O membro que criou esta automação não tem mais acesso para iniciar sessões. Um administrador do workspace pode assumi-la, o que a retoma.",
    description:
      "Explanation on the detail page for a creator-lost-access pause",
  },
  "agentAutomations.pausedExplain.template_invalid": {
    message:
      "O modelo de sessão não passa mais na validação, por exemplo porque um segredo ou uma configuração de modelo mudou. Quem a criou pode editá-la e depois retomá-la.",
    description: "Explanation on the detail page for a template-invalid pause",
  },
  "agentAutomations.pausedExplain.unknown": {
    message:
      "A plataforma pausou esta automação. Retome-a quando a causa for corrigida.",
    description: "Explanation for a pause reason this version does not know",
  },
  "agentAutomations.pausedToast": {
    message: "{name} pausada.",
    description: "Toast after pausing an automation",
  },
  "agentAutomations.resumedToast": {
    message: "{name} retomada.",
    description: "Toast after resuming an automation",
  },
  "agentAutomations.runStartedToast": {
    message: "{name} iniciou uma sessão.",
    description: "Toast after Run now started a session",
  },
  "agentAutomations.runSettledToast": {
    message: "{name} executada: {outcome}.",
    description: "Toast after Run now ended with an outcome other than started",
  },
  "agentAutomations.tookOverToast": {
    message: "Agora você é o proprietário de {name}.",
    description: "Toast after an admin takes an automation over",
  },
  "agentAutomations.deletedToast": {
    message: "{name} excluída.",
    description: "Toast after deleting an automation",
  },
  "agentAutomations.createdToast": {
    message: "{name} criada.",
    description: "Toast after creating an automation",
  },
  "agentAutomations.updatedToast": {
    message: "{name} salva.",
    description: "Toast after editing an automation",
  },
  "agentAutomations.actionFailed": {
    message: "Isso não funcionou. Tente novamente.",
    description: "Generic failure toast for an automation action",
  },
  "agentAutomations.saveFailed": {
    message: "Não foi possível salvar a automação. Tente novamente.",
    description: "Generic failure message when saving the form",
  },
  "agentAutomations.actionsLabel": {
    message: "Ações da automação",
    description: "Accessible label of the action button group",
  },
  "agentAutomations.runNow": {
    message: "Executar agora",
    description: "Button: fire the automation now",
  },
  "agentAutomations.runNowPausedHint": {
    message: "Retome esta automação antes de executá-la.",
    description:
      "Tooltip on Run now while the automation is paused by the platform",
  },
  "agentAutomations.pause": {
    message: "Pausar",
    description: "Button: pause the automation",
  },
  "agentAutomations.resume": {
    message: "Retomar",
    description: "Button: resume the automation",
  },
  "agentAutomations.edit": {
    message: "Editar",
    description: "Button: edit the automation",
  },
  "agentAutomations.editCreatorOnly": {
    message: "Somente o membro que criou esta automação pode editá-la.",
    description: "Tooltip and notice: editing is for the creator only",
  },
  "agentAutomations.takeOver": {
    message: "Assumir",
    description: "Button: an admin becomes the automation's creator",
  },
  "agentAutomations.takeOverAdminOnly": {
    message:
      "Somente um administrador do workspace que não criou esta automação pode assumi-la.",
    description: "Tooltip on a disabled Take over button",
  },
  "agentAutomations.takeOverTitle": {
    message: "Assumir {name}?",
    description: "Take over confirmation title",
  },
  "agentAutomations.takeOverBody": {
    message:
      "As próximas execuções iniciarão sessões como você, com o seu acesso, e o modelo será verificado novamente com o seu acesso. Uma automação pausada porque o criador perdeu o acesso será retomada.",
    description: "Take over confirmation body",
  },
  "agentAutomations.delete": {
    message: "Excluir",
    description: "Button: delete the automation",
  },
  "agentAutomations.deleteTitle": {
    message: "Excluir {name}?",
    description: "Delete confirmation title",
  },
  "agentAutomations.deleteBody": {
    message:
      "A automação e o histórico de execuções serão excluídos. As sessões que ela já iniciou são mantidas.",
    description: "Delete confirmation body",
  },
  "agentAutomations.settingsTitle": {
    message: "Configurações",
    description: "Settings card title on the detail page",
  },
  "agentAutomations.row.status": {
    message: "Status",
    description: "Settings row label",
  },
  "agentAutomations.row.schedule": {
    message: "Agendamento",
    description: "Settings row label",
  },
  "agentAutomations.row.nextRun": {
    message: "Próxima execução",
    description: "Settings row label",
  },
  "agentAutomations.row.lastRun": {
    message: "Última execução",
    description: "Settings row label",
  },
  "agentAutomations.row.createdBy": {
    message: "Criada por",
    description: "Settings row label",
  },
  "agentAutomations.row.repo": {
    message: "Repositório",
    description: "Settings row label",
  },
  "agentAutomations.row.agent": {
    message: "Agente",
    description: "Settings row label",
  },
  "agentAutomations.row.spendLimit": {
    message: "Limite de gastos por execução",
    description: "Settings row label",
  },
  "agentAutomations.row.emailNotifications": {
    message: "Notificações por e-mail",
    description: "Settings row label",
  },
  "agentAutomations.row.secrets": {
    message: "Segredos",
    description: "Settings row label",
  },
  "agentAutomations.row.skills": {
    message: "Skills",
    description: "Settings row label",
  },
  "agentAutomations.taskTitle": {
    message: "Tarefa",
    description: "Task card title on the detail page",
  },
  "agentAutomations.noRepo": {
    message: "Nenhum (sessão somente de chat)",
    description: "Repository value when the template has none",
  },
  "agentAutomations.noSpendLimit": {
    message: "Sem limite",
    description: "Spend limit value when the template has none",
  },
  "agentAutomations.none": {
    message: "Nenhum",
    description: "Empty value for secrets or skills",
  },
  "agentAutomations.runsTitle": {
    message: "Histórico de execuções",
    description: "Run history card title",
  },
  "agentAutomations.runsDescription": {
    message:
      "Todas as execuções agendadas e manuais, da mais recente para a mais antiga.",
    description: "Run history card description",
  },
  "agentAutomations.runsEmpty": {
    message: "Nenhuma execução ainda.",
    description: "Run history empty state",
  },
  "agentAutomations.runs.fireAt": {
    message: "Horário de disparo",
    description: "Run table column",
  },
  "agentAutomations.runs.outcome": {
    message: "Resultado",
    description: "Run table column",
  },
  "agentAutomations.runs.session": {
    message: "Sessão",
    description: "Run table column",
  },
  "agentAutomations.runs.missed": {
    message: "Perdidas",
    description: "Run table column: ticks skipped while the scheduler was down",
  },
  "agentAutomations.runs.error": {
    message: "Erro",
    description: "Run table column",
  },
  "agentAutomations.runs.openSession": {
    message: "Abrir sessão",
    description: "Link to the session a run started",
  },
  "agentAutomations.outcome.started": {
    message: "Iniciada",
    description: "Run outcome: a session was started",
  },
  "agentAutomations.outcome.claimed": {
    message: "Em andamento",
    description: "Run outcome: claimed, not yet settled",
  },
  "agentAutomations.outcome.skipped_overlap": {
    message: "Ignorada",
    description:
      "Run outcome: skipped because the previous session was still running",
  },
  "agentAutomations.outcome.failed_create": {
    message: "Falhou",
    description: "Run outcome: the session could not be created",
  },
  "agentAutomations.outcome.denied": {
    message: "Negada",
    description:
      "Run outcome: the creator was no longer allowed to start the session",
  },
  "agentAutomations.outcome.didNotStart": {
    message: "Não iniciou",
    description:
      "Run outcome: the run started a session whose sandbox did not start for a cause on bex's side (w10/m497); the cause shows in the Error column. Not a failed create",
  },
  "agentAutomations.outcome.unknown": {
    message: "Desconhecido",
    description: "Run outcome this version does not know",
  },
  "agentAutomations.section.details": {
    message: "Detalhes",
    description: "Form section title",
  },
  "agentAutomations.section.schedule": {
    message: "Agendamento",
    description: "Form section title",
  },
  "agentAutomations.section.scheduleDescription": {
    message:
      "Quando a automação é executada. O fuso horário define o que significa 09:00.",
    description: "Form section description",
  },
  "agentAutomations.section.session": {
    message: "Sessão",
    description: "Form section title",
  },
  "agentAutomations.section.sessionDescription": {
    message:
      "A sessão de agente que cada execução inicia, com as mesmas opções de uma nova sessão na página Agentes.",
    description: "Form section description",
  },
  "agentAutomations.field.name": {
    message: "Nome",
    description: "Form field label",
  },
  "agentAutomations.field.frequency": {
    message: "Repetir",
    description: "Form field label",
  },
  "agentAutomations.field.minute": {
    message: "Minuto da hora",
    description: "Form field label",
  },
  "agentAutomations.field.at": {
    message: "Horário",
    description: "Form field label",
  },
  "agentAutomations.field.weekdays": {
    message: "Dias",
    description: "Form field label",
  },
  "agentAutomations.field.runAt": {
    message: "Data e hora",
    description: "Form field label",
  },
  "agentAutomations.field.cron": {
    message: "Expressão cron",
    description: "Form field label",
  },
  "agentAutomations.field.timezone": {
    message: "Fuso horário",
    description: "Form field label",
  },
  "agentAutomations.field.endsOn": {
    message: "Termina em (opcional)",
    description: "Form field label",
  },
  "agentAutomations.field.task": {
    message: "Tarefa",
    description: "Form field label",
  },
  "agentAutomations.field.repo": {
    message: "Repositório",
    description: "Form field label",
  },
  "agentAutomations.field.agent": {
    message: "Agente",
    description: "Form field label",
  },
  "agentAutomations.field.model": {
    message: "Modelo",
    description: "Automation form — model field label (w10/m516)",
  },
  "agentAutomations.field.template": {
    message: "Sandbox",
    description: "Automation form — sandbox template field label (w10/m519)",
  },
  "agentAutomations.field.spendLimit": {
    message: "Limite de gastos por execução (USD)",
    description: "Form field label",
  },
  "agentAutomations.field.emailNotifications": {
    message: "Notificações por e-mail",
    description: "Form field label",
  },
  "agentAutomations.field.secrets": {
    message: "Segredos",
    description: "Form field label",
  },
  "agentAutomations.field.skills": {
    message: "Skills",
    description: "Form field label",
  },
  "agentAutomations.frequency.hour": {
    message: "A cada hora",
    description: "Frequency option",
  },
  "agentAutomations.frequency.day": {
    message: "Todos os dias",
    description: "Frequency option",
  },
  "agentAutomations.frequency.week": {
    message: "Toda semana",
    description: "Frequency option",
  },
  "agentAutomations.frequency.once": {
    message: "Uma vez",
    description: "Frequency option",
  },
  "agentAutomations.frequency.custom": {
    message: "Personalizado (cron)",
    description: "Frequency option",
  },
  "agentAutomations.emailNotifications.always": {
    message: "Sempre",
    description: "Email notification option: every run emails the creator",
  },
  "agentAutomations.emailNotifications.on_failure": {
    message: "Em caso de falha",
    description:
      "Email notification option: only failed runs email the creator",
  },
  "agentAutomations.emailNotifications.on_success": {
    message: "Em caso de sucesso",
    description:
      "Email notification option: only succeeded runs email the creator",
  },
  "agentAutomations.emailNotifications.never": {
    message: "Nunca",
    description: "Email notification option: no run emails the creator",
  },
  "agentAutomations.weekday.mon": {
    message: "Seg",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.tue": {
    message: "Ter",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.wed": {
    message: "Qua",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.thu": {
    message: "Qui",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.fri": {
    message: "Sex",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.sat": {
    message: "Sáb",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.sun": {
    message: "Dom",
    description: "Weekday toggle",
  },
  "agentAutomations.help.name": {
    message: "Letras minúsculas, dígitos e hifens simples, até 64 caracteres.",
    description: "Name field help",
  },
  "agentAutomations.help.minute": {
    message: "De 0 a 59.",
    description: "Minute field help",
  },
  "agentAutomations.help.runAt": {
    message: "Interpretado no fuso horário escolhido abaixo ({zone}).",
    description: "Run-at field help",
  },
  "agentAutomations.help.cron": {
    message:
      "Cinco campos: minuto hora dia-do-mês mês dia-da-semana. As execuções não podem ter intervalo menor que 5 minutos.",
    description: "Cron field help",
  },
  "agentAutomations.help.endsOn": {
    message:
      "O último dia em que pode ser executada. Deixe vazio para executar até você pausá-la.",
    description: "Ends-on field help",
  },
  "agentAutomations.help.task": {
    message:
      "O que o agente deve fazer a cada execução. Mencione o resultado esperado.",
    description: "Task field help",
  },
  "agentAutomations.help.repo": {
    message:
      "Opcional. Sem repositório, cada execução é uma sessão somente de chat. Cada execução trabalha em uma branch bex-agent nova.",
    description: "Repository field help",
  },
  "agentAutomations.help.spendLimit": {
    message:
      "Um teto de gasto com modelo para cada execução. Obrigatório quando as execuções usam o modelo da plataforma.",
    description: "Spend limit field help",
  },
  "agentAutomations.help.model": {
    message:
      "O modelo que cada execução usa. Deixe no padrão para seguir o do workspace.",
    description: "Automation form — model field help",
  },
  "agentAutomations.help.modelUnoffered": {
    message:
      "{model} não é mais oferecido neste workspace: um administrador o desativou ou ele saiu do catálogo da plataforma. As execuções usam o padrão do workspace, {fallback}, no lugar dele. Escolha um modelo oferecido para salvar as alterações.",
    description:
      "Automation form — model field note when the saved model is no longer offered (turned off by an admin or gone from the catalog); {fallback} is the workspace default its runs use (w10/m527)",
  },
  "agentAutomations.help.template": {
    message:
      "A máquina que o sandbox de cada execução recebe. O padrão do workspace segue Configurações → Agentes a cada execução.",
    description: "Automation form — sandbox template field help (w10/m519)",
  },
  "agentAutomations.help.githubCli": {
    message:
      "Se as sessões dela recebem o gh. “Herdar” segue “GitHub CLI para automações” em Configurações → Agentes.",
    description: "Automation form — GitHub CLI field help (w10/m561)",
  },
  "agentAutomations.help.emailNotifications": {
    message:
      "Quais execuções enviam e-mail para você. Uma pausa da plataforma sempre envia.",
    description: "Email notifications field help",
  },
  "agentAutomations.help.secrets": {
    message:
      "Segredos do workspace que o agente recebe como variáveis de ambiente. Adicione-os nas configurações do workspace.",
    description: "Secrets field help",
  },
  "agentAutomations.noSecrets": {
    message: "Nenhum segredo",
    description: "Secrets row when none are chosen",
  },
  "agentAutomations.noSkills": {
    message: "Nenhuma skill",
    description: "Skills row when none are chosen",
  },
  "agentAutomations.placeholder.name": {
    message: "nightly-dependency-check",
    description: "Name field placeholder (an example slug)",
  },
  "agentAutomations.placeholder.repo": {
    message: "owner/repository",
    description: "Repository field placeholder",
  },
  "agentAutomations.placeholder.model": {
    message: "Padrão do workspace",
    description: "Automation form — empty free-form model field",
  },
  "agentAutomations.placeholder.template": {
    message: "Padrão do workspace",
    description:
      "Automation form — the sandbox field while no template list is available: the workspace default",
  },
  "agentAutomations.summary": {
    message: "Executa: {schedule}",
    description: "Live summary under the schedule picker",
  },
  "agentAutomations.schedule.hour": {
    message: "A cada hora aos :{minute}",
    description: "Schedule in words, hourly",
  },
  "agentAutomations.schedule.day": {
    message: "Todos os dias às {at}",
    description: "Schedule in words, daily",
  },
  "agentAutomations.schedule.week": {
    message: "Toda {days} às {at}",
    description: "Schedule in words, weekly",
  },
  "agentAutomations.schedule.once": {
    message: "Uma vez em {when}",
    description: "Schedule in words, one-off",
  },
  "agentAutomations.schedule.custom": {
    message: "Cron {cron}",
    description: "Schedule in words, cron",
  },
  "agentAutomations.schedule.until": {
    message: "até {date}",
    description: "Schedule end date suffix",
  },
  "agentAutomations.problem.nameRequired": {
    message: "Insira um nome.",
    description: "Form validation",
  },
  "agentAutomations.problem.nameInvalid": {
    message:
      "Use letras minúsculas, dígitos e hifens simples, até 64 caracteres.",
    description: "Form validation",
  },
  "agentAutomations.problem.taskRequired": {
    message: "Descreva a tarefa.",
    description: "Form validation",
  },
  "agentAutomations.problem.spendLimit": {
    message: "Insira um valor acima de 0 e até 100000, ou deixe vazio.",
    description: "Form validation",
  },
  "agentAutomations.problem.skills": {
    message:
      "Os argumentos de uma skill devem ter uma linha de no máximo 4 KB.",
    description: "Form validation",
  },
  "agentAutomations.problem.githubCliMandatory": {
    message:
      "Neste workspace a desativação do GitHub CLI é obrigatória, então uma automação não pode ativar o gh. Escolha “Desativado” ou “Herdar”.",
    description:
      "Automation form — AGENT_AUTOMATION_GITHUB_CLI_LOCKED with reason mandatory (w10/m561)",
  },
  "agentAutomations.problem.githubCliAdminRequired": {
    message:
      "Somente um administrador do workspace pode escolher ou alterar o GitHub CLI de uma automação. Deixe como estava.",
    description:
      "Automation form — AGENT_AUTOMATION_GITHUB_CLI_LOCKED with reason admin_required: only a workspace admin sets or changes an automation's gh binding (w10/044)",
  },
  "agentAutomations.problem.timezone": {
    message: "Escolha um fuso horário válido.",
    description: "Form validation",
  },
  "agentAutomations.problem.minute": {
    message: "Insira um minuto de 0 a 59.",
    description: "Form validation",
  },
  "agentAutomations.problem.at": {
    message: "Insira um horário.",
    description: "Form validation",
  },
  "agentAutomations.problem.weekdays": {
    message: "Escolha pelo menos um dia.",
    description: "Form validation",
  },
  "agentAutomations.problem.runAtRequired": {
    message: "Escolha uma data e hora.",
    description: "Form validation",
  },
  "agentAutomations.problem.runAtPast": {
    message: "Escolha um horário no futuro.",
    description: "Form validation",
  },
  "agentAutomations.problem.cron": {
    message: "Insira cinco campos: minuto hora dia-do-mês mês dia-da-semana.",
    description: "Form validation",
  },
  "agentAutomations.problem.endsOn": {
    message: "Insira uma data válida.",
    description: "Form validation",
  },
  "agentAutomations.problem.endsOnPast": {
    message: "Escolha uma data no futuro.",
    description: "Form validation",
  },
  "agentAutomations.startedBy": {
    message: "Iniciada por automação",
    description: "Session page header note linking back to the automation",
  },
};

export default ptAgentAutomations;
