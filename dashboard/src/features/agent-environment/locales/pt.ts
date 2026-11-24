import type { TranslationEntry } from "@/i18n";

const ptAgentEnvironment: Record<string, TranslationEntry> = {
  "agentEnvironment.title": {
    message: "Agentes",
    description: "Settings → Agents card title and section-nav label",
  },
  "agentEnvironment.description": {
    message:
      "Como as sessões de agente neste workspace acessam a rede e qual modelo usam. Vale para toda nova sessão.",
    description: "Agents settings card description",
  },
  "agentEnvironment.loadFailed": {
    message: "Não foi possível carregar as configurações do agente.",
    description: "Agents card error when the environment query fails",
  },
  "agentEnvironment.retry": {
    message: "Tentar novamente",
    description: "Retry button on the Agents card load error",
  },
  "agentEnvironment.readOnlyNote": {
    message:
      "Somente administradores do workspace podem alterar as configurações do agente.",
    description: "Read-only notice shown to non-admin members",
  },
  "agentEnvironment.networkTitle": {
    message: "Acesso à rede",
    description: "Heading (and radiogroup label) of the network tier picker",
  },
  "agentEnvironment.networkDescription": {
    message:
      "O GitHub e o provedor do modelo sempre são acessíveis. Escolha o que mais o sandbox do agente pode acessar.",
    description: "Network tier picker description",
  },
  "agentEnvironment.tierRegistriesTitle": {
    message: "Registros de pacotes (recomendado)",
    description: "Network tier option: built-in package registries",
  },
  "agentEnvironment.tierRegistriesDescription": {
    message:
      "O agente pode instalar dependências do npm, PyPI, Go, Maven, crates, RubyGems e Docker Hub, para conseguir fazer build e testar seu código.",
    description: "Description of the registries network tier",
  },
  "agentEnvironment.tierCustomTitle": {
    message: "Personalizado",
    description: "Network tier option: registries plus listed hosts",
  },
  "agentEnvironment.tierCustomDescription": {
    message: "Registros de pacotes mais os hostnames que você listar abaixo.",
    description: "Description of the custom network tier",
  },
  "agentEnvironment.tierFullTitle": {
    message: "Internet completa",
    description: "Network tier option: any public host over HTTP(S)",
  },
  "agentEnvironment.tierFullDescription": {
    message:
      "Qualquer site público via HTTP(S) pelo proxy de saída do bex. As chamadas diretas às APIs de provedores de modelo continuam bloqueadas (o modelo do próprio agente continua funcionando), e SSH e outros protocolos não HTTP não estão disponíveis.",
    description:
      "Description of the full-internet tier: HTTP(S) via the egress proxy, direct model-provider API calls blocked, no SSH/non-HTTP",
  },
  "agentEnvironment.registryHostsSummary_one": {
    message: "Registros de pacotes incluídos ({count} host)",
    description: "Disclosure summary listing the built-in registry hosts (one)",
  },
  "agentEnvironment.registryHostsSummary_many": {
    message: "Registros de pacotes incluídos ({count} de hosts)",
    description:
      "Disclosure summary listing the built-in registry hosts (many)",
  },
  "agentEnvironment.registryHostsSummary_other": {
    message: "Registros de pacotes incluídos ({count} hosts)",
    description:
      "Disclosure summary listing the built-in registry hosts (many)",
  },
  "agentEnvironment.hostsLabel": {
    message: "Hostnames adicionais",
    description: "Label of the custom-tier hostname textarea",
  },
  "agentEnvironment.hostsHelp": {
    message:
      "Um hostname por linha, até {max}. Somente nomes exatos: sem esquema, porta, caminho nem curinga.",
    description: "Help under the custom hostname textarea",
  },
  "agentEnvironment.hostsError.too_many_entries": {
    message: "Liste no máximo {max} hostnames.",
    description: "Custom hosts error: too many entries",
  },
  "agentEnvironment.hostsError.empty": {
    message: "Os hostnames não podem ficar vazios.",
    description: "Custom hosts error: empty entry",
  },
  "agentEnvironment.hostsError.whitespace": {
    message: '"{entry}" tem espaços ao redor.',
    description: "Custom hosts error: surrounding whitespace",
  },
  "agentEnvironment.hostsError.not_a_hostname": {
    message:
      '"{entry}" não é um hostname exato. Use um nome como api.example.com, sem esquema, porta, caminho nem curinga.',
    description: "Custom hosts error: not a hostname",
  },
  "agentEnvironment.hostsError.ip_literal": {
    message: '"{entry}" é um endereço IP. Use um hostname.',
    description: "Custom hosts error: IP literal",
  },
  "agentEnvironment.hostsError.private_name": {
    message:
      '"{entry}" é um nome privado ou local do cluster e não pode ser permitido.',
    description: "Custom hosts error: private name",
  },
  "agentEnvironment.hostsError.already_baseline": {
    message:
      '"{entry}" já é acessível (ou acessível somente pelo proxy do modelo). Remova-o da lista.',
    description: "Custom hosts error: already in the baseline",
  },
  "agentEnvironment.hostsError.duplicate": {
    message: '"{entry}" aparece mais de uma vez na lista.',
    description: "Custom hosts error: duplicate",
  },
  "agentEnvironment.hostsError.invalid": {
    message: "A lista de hostnames é inválida.",
    description: "Custom hosts error: unknown reason",
  },
  "agentEnvironment.modelLabel": {
    message: "Modelo padrão",
    description: "Label of the default model field",
  },
  "agentEnvironment.modelPlaceholder": {
    message: "Padrão do provedor",
    description: "Placeholder of the empty default model field",
  },
  "agentEnvironment.modelHelp": {
    message:
      "ID do modelo que as novas sessões usam, como claude-sonnet-4-5. Deixe vazio para usar o padrão do provedor.",
    description: "Help under the default model field",
  },
  "agentEnvironment.modelPlatformDefault": {
    message: "Padrão da plataforma ({model})",
    description:
      "Default model picker — no default: the platform catalog's first model (w10/m516)",
  },
  "agentEnvironment.modelCatalogHelp": {
    message:
      "Novas sessões e automações que não indicam um modelo usam este. Os membros ainda podem escolher outro modelo do catálogo da plataforma para uma sessão.",
    description: "Help under the platform model catalog picker (w10/m516)",
  },
  "agentEnvironment.modelInvalid": {
    message: "Use um ID de modelo sem espaços (no máximo 255 caracteres).",
    description: "Default model validation error",
  },
  "agentEnvironment.modelNotOfferedError": {
    message: "O modelo da plataforma não oferece {model}. Escolha um da lista.",
    description:
      "Server refusal: the default model is not in the platform catalog (w10/m516)",
  },
  "agentEnvironment.templateLabel": {
    message: "Sandbox padrão",
    description: "Label of the default sandbox template picker (w10/m519)",
  },
  "agentEnvironment.templateHelp": {
    message:
      "Novas sessões e automações que não indicam um sandbox rodam neste. Vale para as sessões iniciadas depois; os membros ainda podem escolher outro para uma sessão.",
    description: "Help under the default sandbox template picker (w10/m519)",
  },
  "agentEnvironment.templateInvalidError": {
    message:
      "{template} não é um template de sandbox. Escolha um destes: {templates}.",
    description:
      "Server refusal: the default sandbox template is not a registered one; {templates} lists those (w10/m519)",
  },
  "agentEnvironment.tierInvalid": {
    message: "Essa opção de acesso à rede não é compatível.",
    description: "Server refusal: invalid tier",
  },
  "agentEnvironment.forbidden": {
    message:
      "Somente administradores do workspace podem alterar as configurações do agente.",
    description: "Server refusal: caller is not an admin",
  },
  "agentEnvironment.saveFailed": {
    message:
      "Não foi possível salvar as configurações do agente. Tente novamente.",
    description: "Generic save failure",
  },
  "agentEnvironment.saveFailedReason": {
    message: "Não foi possível salvar as configurações do agente: {reason}",
    description: "Save failure with the server's reason",
  },
  "agentEnvironment.save": {
    message: "Salvar",
    description: "Agents card save button",
  },
  "agentEnvironment.saved": {
    message: "Configurações do agente salvas",
    description: "Success toast after saving",
  },
  "agentEnvironment.modelKeyTitle": {
    message: "Provedor do modelo",
    description: "Heading of the model source / provider key section",
  },
  "agentEnvironment.modelKeyDescription": {
    message:
      "As sessões de agente usam o modelo fornecido pelo bex, a menos que o workspace adicione a própria chave de provedor. Com a sua chave, as sessões chamam diretamente o seu provedor e o bex cobra apenas a computação do sandbox.",
    description: "Explains the model source choice (ADR091 D7)",
  },
  "agentEnvironment.modelSourceLabel": {
    message: "Origem do modelo:",
    description: "Label before the current model source",
  },
  "agentEnvironment.modelSourcePlatform": {
    message:
      "Fornecido pelo bex (cobrado ao custo do provedor do modelo + 30%)",
    description: "Model source: the platform model",
  },
  "agentEnvironment.modelSourceByo": {
    message: "Sua própria chave (sem cobrança de tokens pelo bex)",
    description: "Model source: the workspace's own key",
  },
  "agentEnvironment.modelSourceNone": {
    message:
      "Nenhuma: adicione uma chave de provedor para iniciar sessões de agente",
    description: "Model source: no key available",
  },
  "agentEnvironment.modelKeyLabel": {
    message: "Chave de API do provedor",
    description: "Label of the write-only key field when none is set",
  },
  "agentEnvironment.modelKeyReplaceLabel": {
    message: "Substituir chave de API do provedor",
    description: "Label of the write-only key field when a key is set",
  },
  "agentEnvironment.modelKeyPlaceholder": {
    message: "sk-ant-…",
    description: "Placeholder of the key field when none is set",
  },
  "agentEnvironment.modelKeyPlaceholderSet": {
    message: "Há uma chave salva; insira uma nova para substituí-la",
    description:
      "Placeholder of the key field when a key is set (the key is never shown)",
  },
  "agentEnvironment.modelKeyHelp": {
    message:
      "Somente gravação: a chave é armazenada com segurança e nunca é exibida novamente.",
    description: "Help under the key field",
  },
  "agentEnvironment.modelKeySave": {
    message: "Salvar chave",
    description: "Button that saves the model key",
  },
  "agentEnvironment.modelKeyClear": {
    message: "Remover chave",
    description: "Button that opens the remove-key confirmation",
  },
  "agentEnvironment.modelKeyClearTitle": {
    message: "Remover a chave do provedor do modelo?",
    description: "Remove-key confirmation title",
  },
  "agentEnvironment.modelKeyClearBody": {
    message:
      "As sessões de agente deixam de usar esta chave a partir da próxima requisição ao modelo. A chave não pode ser exibida novamente, então será preciso colá-la para restaurá-la.",
    description: "Remove-key confirmation body",
  },
  "agentEnvironment.modelKeyClearConfirm": {
    message: "Remover chave",
    description: "Remove-key confirmation button",
  },
  "agentEnvironment.modelKeySaved": {
    message: "Chave do modelo salva",
    description: "Toast after saving the key",
  },
  "agentEnvironment.modelKeyCleared": {
    message: "Chave do modelo removida",
    description: "Toast after removing the key",
  },
  "agentEnvironment.modelKeyInvalid": {
    message: "Insira a chave sem espaços (no máximo 4.096 caracteres).",
    description: "Client/server refusal of a malformed key",
  },
  "agentEnvironment.modelKeySaveFailed": {
    message: "Não foi possível salvar a chave do modelo. Tente novamente.",
    description: "Generic model key write failure",
  },
  "agentEnvironment.spendTitle": {
    message: "Gastos com o modelo do agente",
    description: "Heading of the agent-model spend section (ADR091 D6)",
  },
  "agentEnvironment.spendPricing": {
    message:
      "Cobrado ao custo do provedor do modelo + 30%. O limite interrompe novas requisições ao modelo pelo restante do mês.",
    description: "Pricing statement under the spend heading",
  },
  "agentEnvironment.spendSummary": {
    message: "{used} de {cap} neste mês",
    description: "Month-to-date spend against the effective monthly cap",
  },
  "agentEnvironment.spendSummaryCeiling": {
    message:
      "{used} de {cap} além dos créditos de faturamento, {total} neste mês",
    description:
      "Spend summary and meter value when the trust tier's ceiling binds on the spend billing credits leave ({used}), which differs from month-to-date ({total})",
  },
  "agentEnvironment.spendMeterValue": {
    message: "{used} de {cap}",
    description: "Screen-reader value of the spend progress bar",
  },
  "agentEnvironment.spendUnavailable": {
    message: "Os gastos não estão disponíveis no momento.",
    description: "Shown when the spend fields could not be read",
  },
  "agentEnvironment.spendTierNextCharge": {
    message:
      "Nível de confiança {tier}: os créditos de faturamento pagam primeiro os gastos com o modelo do agente; seu cartão é cobrado quando o gasto restante chegar a {amount}.",
    description: "Trust tier with its mid-month charge point (ADR091 D10.1)",
  },
  "agentEnvironment.spendTierNextChargeCredit": {
    message:
      "Nível de confiança {tier}: os créditos de faturamento cobrem os primeiros {credit} de gastos com o modelo do agente neste período de faturamento; seu cartão é cobrado quando o gasto além deles chegar a {amount}.",
    description:
      "Trust tier with its charge point when billing credits cover agent model spend first (ADR091 D10.2a). {credit} is the credit counted in this billing period, {amount} the charge point",
  },
  "agentEnvironment.spendTierMonthEnd": {
    message:
      "Nível de confiança {tier}: os gastos com o modelo do agente são cobrados no fim do mês.",
    description: "Top trust tier, charged only at month end",
  },
  "agentEnvironment.spendTierNoCharge": {
    message:
      "Nível de confiança {tier}: os gastos com o modelo do agente não são cobrados em um cartão no meio do ciclo.",
    description:
      "Trust tier with no mid-cycle card charge: a payment hold, an unpaid invoice, the workspace's own key, or an unchargeable workspace (ADR091 D10.2a)",
  },
  "agentEnvironment.spendTierNew": {
    message:
      "Workspaces novos são cobrados quando os gastos com o modelo do agente, além de eventuais créditos de faturamento, chegam a {amount}: o cartão é cobrado nesse momento, as sessões continuam em execução assim que a cobrança é concluída, e o limite sobe para o teto do seu plano.",
    description:
      "Explains T0: charged at its threshold, working while the charge settles (ADR091 D10.2a)",
  },
  "agentEnvironment.spendTierLocked": {
    message:
      "Um pagamento falho ou contestado limita este workspace; regularize-o em Faturamento para remover o limite.",
    description: "Explains a payment-locked trust tier",
  },
  "agentEnvironment.spendCapLabel": {
    message: "Limite de gastos mensal (USD)",
    description: "Label of the admin-editable cap field",
  },
  "agentEnvironment.spendCapSave": {
    message: "Salvar limite",
    description: "Button that saves the cap",
  },
  "agentEnvironment.spendCapReset": {
    message: "Restaurar padrão do plano",
    description: "Button that returns the cap to the plan default",
  },
  "agentEnvironment.spendCapHelp": {
    message:
      "Padrão do plano: {plan}. Administradores podem definir até {max}.",
    description: "Help under the cap field",
  },
  "agentEnvironment.spendCapInvalid": {
    message: "Insira um valor maior que $0 e de no máximo {max}.",
    description: "Client/server refusal of a cap value",
  },
  "agentEnvironment.spendCapSaved": {
    message: "Limite de gastos salvo",
    description: "Toast after saving the cap",
  },
  "agentEnvironment.spendCapResetDone": {
    message: "Limite de gastos restaurado para o padrão do plano",
    description: "Toast after resetting the cap",
  },
  "agentEnvironment.spendCapSaveFailed": {
    message: "Não foi possível salvar o limite de gastos. Tente novamente.",
    description: "Generic cap write failure",
  },
  "agentEnvironment.secrets.title": {
    message: "Segredos do agente",
    description: "Settings → Agent secrets card title (ADR098)",
  },
  "agentEnvironment.secrets.description": {
    message:
      "Chaves de API e tokens que uma sessão pode receber como variáveis de ambiente. Os valores são somente de gravação: ninguém consegue lê-los de volta e eles são ocultados nas transcrições.",
    description: "Agent secrets card description",
  },
  "agentEnvironment.secrets.loadFailed": {
    message: "Não foi possível carregar os segredos do agente.",
    description: "Agent secrets card error when the list query fails",
  },
  "agentEnvironment.secrets.empty": {
    message: "Nenhum segredo do agente ainda.",
    description: "Agent secrets card empty state",
  },
  "agentEnvironment.secrets.deleteLabel": {
    message: "Excluir {name}",
    description: "Accessible label of a secret's delete button",
  },
  "agentEnvironment.secrets.deleteTitle": {
    message: "Excluir {name}?",
    description: "Delete-secret confirmation title",
  },
  "agentEnvironment.secrets.deleteBody": {
    message:
      "As sessões que usam este segredo falham no próximo turno até que ele seja definido novamente.",
    description: "Delete-secret confirmation body",
  },
  "agentEnvironment.secrets.deleteConfirm": {
    message: "Excluir segredo",
    description: "Delete-secret confirmation button",
  },
  "agentEnvironment.secrets.nameLabel": {
    message: "Nome do segredo",
    description: "Agent secrets card: name field label",
  },
  "agentEnvironment.secrets.namePlaceholder": {
    message: "STRIPE_API_KEY",
    description: "Agent secrets card: name field placeholder (an env var name)",
  },
  "agentEnvironment.secrets.valueLabel": {
    message: "Valor do segredo",
    description: "Agent secrets card: value field label",
  },
  "agentEnvironment.secrets.valuePlaceholder": {
    message: "Valor",
    description: "Agent secrets card: value field placeholder",
  },
  "agentEnvironment.secrets.save": {
    message: "Salvar",
    description: "Agent secrets card: save (create or replace) button",
  },
  "agentEnvironment.secrets.help": {
    message:
      "Salvar um nome existente substitui o valor dele. As sessões recebem as alterações a partir do próximo turno.",
    description: "Agent secrets card help line for admins",
  },
  "agentEnvironment.secrets.readOnly": {
    message:
      "Somente administradores do workspace podem adicionar, alterar ou excluir segredos do agente.",
    description: "Agent secrets card help line for non-admins",
  },
  "agentEnvironment.secrets.saved": {
    message: "{name} salvo",
    description: "Toast after a secret is saved",
  },
  "agentEnvironment.secrets.deleted": {
    message: "{name} excluído",
    description: "Toast after a secret is deleted",
  },
  "agentEnvironment.secrets.nameInvalid": {
    message:
      "Use letras, dígitos e _, começando com uma letra ou _ (no máximo 128).",
    description: "Secret name validation error",
  },
  "agentEnvironment.secrets.nameReserved": {
    message:
      "O bex define essa variável de ambiente por conta própria, então um segredo não pode usar esse nome.",
    description: "Secret name reserved error",
  },
  "agentEnvironment.secrets.valueRequired": {
    message: "Insira um valor.",
    description: "Secret value required error",
  },
  "agentEnvironment.secrets.valueInvalid": {
    message: "Um valor pode ter no máximo 32 KiB.",
    description: "Secret value too long (or contains a NUL byte)",
  },
  "agentEnvironment.secrets.limitExceeded": {
    message: "Isso atingiu o limite de segredos do agente.",
    description: "Secret count or size limit error",
  },
  "agentEnvironment.secrets.notFound": {
    message:
      "Este segredo não existe mais; alguém pode tê-lo excluído agora há pouco.",
    description: "AGENT_SECRET_NOT_FOUND error (deleted concurrently)",
  },
  "agentEnvironment.secrets.unavailable": {
    message: "Os segredos do agente não estão disponíveis nesta instalação.",
    description: "AGENT_SECRETS_UNAVAILABLE error",
  },
  "agentEnvironment.secrets.forbidden": {
    message:
      "Somente administradores do workspace podem alterar os segredos do agente.",
    description: "Secret write refused for a non-admin",
  },
  "agentEnvironment.secrets.saveFailed": {
    message: "Não foi possível salvar a alteração. Tente novamente.",
    description: "Generic secret write failure",
  },
  "agentEnvironment.secrets.valueKeepPlaceholder": {
    message: "Manter o valor atual",
    description:
      "Agent secrets card: value placeholder when the name is an existing secret (empty keeps its value)",
  },
  "agentEnvironment.secrets.editLabel": {
    message: "Editar {name}",
    description:
      "Accessible label of a secret's edit button (loads it into the form to rotate or rescope)",
  },
  "agentEnvironment.secrets.scopeLabel": {
    message: "Escopo",
    description:
      "Agent secrets card: accessible label of the scope picker (ADR098 D9)",
  },
  "agentEnvironment.secrets.scopeWorkspace": {
    message: "Todas as sessões",
    description:
      "Scope picker option: every session in the workspace receives the secret",
  },
  "agentEnvironment.secrets.scopeRepo": {
    message: "Um repositório",
    description:
      "Scope picker option: sessions on one repository receive the secret",
  },
  "agentEnvironment.secrets.scopeExplicit": {
    message: "Somente quando selecionado",
    description:
      "Scope picker option: only sessions that pick the secret receive it",
  },
  "agentEnvironment.secrets.scopeBadgeRepo": {
    message: "Repositório: {repo}",
    description:
      "Secret row badge: repo scope, naming the repository (owner/name)",
  },
  "agentEnvironment.secrets.repoLabel": {
    message: "Repositório",
    description:
      "Agent secrets card: accessible label of the repo scope's repository field",
  },
  "agentEnvironment.secrets.repoPlaceholder": {
    message: "owner/name",
    description: "Agent secrets card: repository field placeholder",
  },
  "agentEnvironment.secrets.repoEmpty": {
    message: "Nenhum repositório conectado corresponde.",
    description:
      "Agent secrets card: repository picker with no matching connected repository",
  },
  "agentEnvironment.secrets.repoInvalid": {
    message: "Insira o repositório como owner/name.",
    description:
      "Repo scope validation error (also AGENT_SECRET_SCOPE_INVALID)",
  },
  "agentEnvironment.repoEnvs.title": {
    message: "Ambientes de repositório",
    description: "Settings → Repository environments card title (ADR109)",
  },
  "agentEnvironment.repoEnvs.description": {
    message:
      "Como as sessões de agente preparam cada repositório. As etapas de configuração são executadas em ordem a partir da raiz do repositório antes de o agente de cada sessão iniciar, com os segredos da sessão. As verificações são os comandos que o agente executa antes de entregar, como lint e testes.",
    description:
      "Repository environments card description: what setup steps and checks are",
  },
  "agentEnvironment.repoEnvs.loadFailed": {
    message: "Não foi possível carregar os ambientes de repositório.",
    description: "Repository environments card error when the list query fails",
  },
  "agentEnvironment.repoEnvs.emptyList": {
    message: "Nenhum ambiente de repositório ainda.",
    description: "Repository environments card empty state",
  },
  "agentEnvironment.repoEnvs.setupSummary": {
    message: "Configuração: {names}",
    description:
      "Repository row summary: the setup step names, comma-separated",
  },
  "agentEnvironment.repoEnvs.checksSummary": {
    message: "Verificações: {names}",
    description: "Repository row summary: the check names, comma-separated",
  },
  "agentEnvironment.repoEnvs.noSetup": {
    message: "Sem etapas de configuração",
    description:
      "Repository row summary (and the read-only dialog) when the environment has no setup steps",
  },
  "agentEnvironment.repoEnvs.noChecks": {
    message: "Sem verificações",
    description:
      "Repository row summary (and the read-only dialog) when the environment has no checks",
  },
  "agentEnvironment.repoEnvs.editLabel": {
    message: "Editar {repo}",
    description:
      "Accessible label of a repository environment's edit button (repo is owner/name)",
  },
  "agentEnvironment.repoEnvs.viewLabel": {
    message: "Ver {repo}",
    description:
      "Accessible label of a repository environment's view button, for non-admins",
  },
  "agentEnvironment.repoEnvs.deleteLabel": {
    message: "Excluir o ambiente de {repo}",
    description: "Accessible label of a repository environment's delete button",
  },
  "agentEnvironment.repoEnvs.deleteTitle": {
    message: "Excluir o ambiente de {repo}?",
    description: "Delete-environment confirmation title",
  },
  "agentEnvironment.repoEnvs.deleteBody": {
    message:
      "A partir do próximo turno, as sessões deste repositório começam sem as etapas de configuração e as verificações dele.",
    description: "Delete-environment confirmation body",
  },
  "agentEnvironment.repoEnvs.deleteConfirm": {
    message: "Excluir ambiente",
    description: "Delete-environment confirmation button",
  },
  "agentEnvironment.repoEnvs.add": {
    message: "Adicionar ambiente",
    description:
      "Repository environments card: button that opens the add-environment dialog",
  },
  "agentEnvironment.repoEnvs.help": {
    message:
      "Salvar substitui o ambiente inteiro do repositório. As sessões recebem as alterações a partir do próximo turno.",
    description: "Repository environments card help line for admins",
  },
  "agentEnvironment.repoEnvs.readOnly": {
    message:
      "Somente administradores do workspace podem adicionar, alterar ou excluir ambientes de repositório.",
    description: "Repository environments card help line for non-admins",
  },
  "agentEnvironment.repoEnvs.saved": {
    message: "Ambiente de {repo} salvo",
    description: "Toast after an environment is saved",
  },
  "agentEnvironment.repoEnvs.deleted": {
    message: "Ambiente de {repo} excluído",
    description: "Toast after an environment is deleted",
  },
  "agentEnvironment.repoEnvs.createTitle": {
    message: "Adicionar ambiente de repositório",
    description: "Add-environment dialog title",
  },
  "agentEnvironment.repoEnvs.editTitle": {
    message: "Editar {repo}",
    description: "Edit-environment dialog title (repo is owner/name)",
  },
  "agentEnvironment.repoEnvs.dialogDescription": {
    message:
      "As etapas de configuração são executadas em ordem a partir da raiz do repositório antes de o agente de cada sessão iniciar, com os segredos da sessão. Uma etapa com falha encerra o turno antes de o agente iniciar. As verificações são o que o agente executa antes de entregar.",
    description: "Environment dialog description",
  },
  "agentEnvironment.repoEnvs.repoLabel": {
    message: "Repositório",
    description: "Environment dialog: repository field label",
  },
  "agentEnvironment.repoEnvs.repoPlaceholder": {
    message: "owner/name",
    description: "Environment dialog: repository field placeholder",
  },
  "agentEnvironment.repoEnvs.repoEmpty": {
    message: "Nenhum repositório conectado corresponde.",
    description:
      "Environment dialog: repository picker with no matching connected repository",
  },
  "agentEnvironment.repoEnvs.setupTitle": {
    message: "Etapas de configuração",
    description: "Environment dialog: setup steps section heading",
  },
  "agentEnvironment.repoEnvs.setupHint": {
    message:
      "Instale ferramentas e dependências. Cada etapa é executada com bash, em ordem; no máximo 10 etapas, de 10 minutos cada.",
    description: "Environment dialog: setup steps section hint",
  },
  "agentEnvironment.repoEnvs.checksTitle": {
    message: "Verificações",
    description: "Environment dialog: checks section heading",
  },
  "agentEnvironment.repoEnvs.checksHint": {
    message:
      "Comandos como lint, teste e build. O agente executa cada um a partir da raiz do repositório antes de entregar e informa os resultados; o bex não os executa por conta própria. No máximo 10.",
    description: "Environment dialog: checks section hint",
  },
  "agentEnvironment.repoEnvs.addSetup": {
    message: "Adicionar etapa de configuração",
    description: "Environment dialog: add a setup step row",
  },
  "agentEnvironment.repoEnvs.addCheck": {
    message: "Adicionar verificação",
    description: "Environment dialog: add a check row",
  },
  "agentEnvironment.repoEnvs.setupNameLabel": {
    message: "Nome da etapa de configuração {position}",
    description:
      "Accessible label of a setup step's name field (position is its 1-based place in the list)",
  },
  "agentEnvironment.repoEnvs.setupRunLabel": {
    message: "Comando da etapa de configuração {position}",
    description: "Accessible label of a setup step's command field",
  },
  "agentEnvironment.repoEnvs.setupMoveUp": {
    message: "Mover a etapa de configuração {position} para cima",
    description: "Accessible label: move a setup step one place earlier",
  },
  "agentEnvironment.repoEnvs.setupMoveDown": {
    message: "Mover a etapa de configuração {position} para baixo",
    description: "Accessible label: move a setup step one place later",
  },
  "agentEnvironment.repoEnvs.setupRemove": {
    message: "Remover a etapa de configuração {position}",
    description: "Accessible label: remove a setup step row",
  },
  "agentEnvironment.repoEnvs.checkNameLabel": {
    message: "Nome da verificação {position}",
    description:
      "Accessible label of a check's name field (position is its 1-based place in the list)",
  },
  "agentEnvironment.repoEnvs.checkRunLabel": {
    message: "Comando da verificação {position}",
    description: "Accessible label of a check's command field",
  },
  "agentEnvironment.repoEnvs.checkMoveUp": {
    message: "Mover a verificação {position} para cima",
    description: "Accessible label: move a check one place earlier",
  },
  "agentEnvironment.repoEnvs.checkMoveDown": {
    message: "Mover a verificação {position} para baixo",
    description: "Accessible label: move a check one place later",
  },
  "agentEnvironment.repoEnvs.checkRemove": {
    message: "Remover a verificação {position}",
    description: "Accessible label: remove a check row",
  },
  "agentEnvironment.repoEnvs.setupNamePlaceholder": {
    message: "deps",
    description: "Setup step name placeholder (an example step name)",
  },
  "agentEnvironment.repoEnvs.setupRunPlaceholder": {
    message: "npm ci",
    description: "Setup step command placeholder (an example shell command)",
  },
  "agentEnvironment.repoEnvs.checkNamePlaceholder": {
    message: "test",
    description: "Check name placeholder (an example check name)",
  },
  "agentEnvironment.repoEnvs.checkRunPlaceholder": {
    message: "npm test",
    description: "Check command placeholder (an example shell command)",
  },
  "agentEnvironment.repoEnvs.close": {
    message: "Fechar",
    description: "Read-only environment dialog: close button",
  },
  "agentEnvironment.repoEnvs.cancel": {
    message: "Cancelar",
    description: "Environment dialog: cancel button",
  },
  "agentEnvironment.repoEnvs.save": {
    message: "Salvar",
    description: "Environment dialog: save (create or replace) button",
  },
  "agentEnvironment.repoEnvs.nameRequired": {
    message: "Insira um nome.",
    description: "Step or check name required error",
  },
  "agentEnvironment.repoEnvs.nameInvalid": {
    message:
      "Um nome começa com uma letra ou dígito e usa letras, dígitos, espaços, '.', '_' ou '-' (no máximo 64).",
    description: "Step or check name format error",
  },
  "agentEnvironment.repoEnvs.nameDuplicate": {
    message: "Outra linha desta lista já usa este nome.",
    description: "Step or check name repeated within its list",
  },
  "agentEnvironment.repoEnvs.runRequired": {
    message: "Insira um comando.",
    description: "Step or check command required error",
  },
  "agentEnvironment.repoEnvs.setupRunTooLong": {
    message: "Um comando de configuração pode ter no máximo 8 KiB.",
    description: "Setup step command too long (or contains a NUL byte)",
  },
  "agentEnvironment.repoEnvs.checkRunTooLong": {
    message: "Um comando de verificação pode ter no máximo 2 KiB.",
    description: "Check command too long (or contains a NUL byte)",
  },
  "agentEnvironment.repoEnvs.setupTooMany": {
    message: "Um ambiente pode ter no máximo 10 etapas de configuração.",
    description: "Too many setup steps",
  },
  "agentEnvironment.repoEnvs.checksTooMany": {
    message: "Um ambiente pode ter no máximo 10 verificações.",
    description: "Too many checks",
  },
  "agentEnvironment.repoEnvs.environmentEmpty": {
    message:
      "Adicione pelo menos uma etapa de configuração ou verificação. Para remover todas, exclua o ambiente.",
    description: "Environment with neither setup steps nor checks",
  },
  "agentEnvironment.repoEnvs.repoInvalid": {
    message: "Insira o repositório como owner/name.",
    description:
      "Repository validation error (also AGENT_REPO_ENVIRONMENT_INVALID on the repository)",
  },
  "agentEnvironment.repoEnvs.repoExists": {
    message: "Este repositório já tem um ambiente. Edite-o pela lista.",
    description: "Adding an environment for a repository that already has one",
  },
  "agentEnvironment.repoEnvs.setupRefused": {
    message:
      "Uma etapa de configuração foi recusada. Verifique se cada nome é válido e único e se cada comando tem no máximo 8 KiB.",
    description: "AGENT_REPO_ENVIRONMENT_INVALID on the setup steps",
  },
  "agentEnvironment.repoEnvs.checksRefused": {
    message:
      "Uma verificação foi recusada. Verifique se cada nome é válido e único e se cada comando tem no máximo 2 KiB.",
    description: "AGENT_REPO_ENVIRONMENT_INVALID on the checks",
  },
  "agentEnvironment.repoEnvs.invalid": {
    message:
      "Este ambiente foi recusado. Verifique o repositório, as etapas de configuração e as verificações.",
    description: "AGENT_REPO_ENVIRONMENT_INVALID without a recognized field",
  },
  "agentEnvironment.repoEnvs.limitExceeded": {
    message:
      "Este workspace atingiu o limite de 50 ambientes de repositório. Exclua um para adicionar outro.",
    description:
      "AGENT_REPO_ENVIRONMENT_LIMIT_EXCEEDED (also checked before saving)",
  },
  "agentEnvironment.repoEnvs.notFound": {
    message:
      "Este repositório não tem mais um ambiente; alguém pode tê-lo excluído agora há pouco.",
    description:
      "AGENT_REPO_ENVIRONMENT_NOT_FOUND error (deleted concurrently)",
  },
  "agentEnvironment.repoEnvs.unavailable": {
    message:
      "Os ambientes de repositório não estão disponíveis nesta instalação.",
    description: "AGENT_REPO_ENVIRONMENTS_UNAVAILABLE error",
  },
  "agentEnvironment.repoEnvs.forbidden": {
    message:
      "Somente administradores do workspace podem alterar os ambientes de repositório.",
    description: "Environment write refused for a non-admin",
  },
  "agentEnvironment.repoEnvs.saveFailed": {
    message: "Não foi possível salvar a alteração. Tente novamente.",
    description: "Generic environment write failure",
  },
  "agentEnvironment.repoEnvs.builds.status.queued": {
    message: "Na fila",
    description: "Build status badge: waiting to start (ADR111 D2)",
  },
  "agentEnvironment.repoEnvs.builds.status.building": {
    message: "Em build",
    description: "Build status badge: the setup steps are running",
  },
  "agentEnvironment.repoEnvs.builds.status.success": {
    message: "Pronto",
    description:
      "Build status badge: success — sessions can start from this snapshot",
  },
  "agentEnvironment.repoEnvs.builds.status.failed": {
    message: "Falhou",
    description: "Build status badge: the build failed",
  },
  "agentEnvironment.repoEnvs.builds.status.cancelled": {
    message: "Cancelado",
    description:
      "Build status badge: a newer build replaced it, or its environment was deleted",
  },
  "agentEnvironment.repoEnvs.builds.trigger.environmentSaved": {
    message: "Etapas de configuração alteradas",
    description:
      "What queued a build: the environment was saved with changed setup steps",
  },
  "agentEnvironment.repoEnvs.builds.trigger.secretChanged": {
    message: "Segredo do repositório alterado",
    description: "What queued a build: a repo-scoped secret was set or changed",
  },
  "agentEnvironment.repoEnvs.builds.trigger.manual": {
    message: "Build manual",
    description:
      "What queued a build: an admin clicked Build now (or used the API)",
  },
  "agentEnvironment.repoEnvs.builds.trigger.refresh": {
    message: "Atualização diária",
    description: "What queued a build: the daily rebuild",
  },
  "agentEnvironment.repoEnvs.builds.failure.setupFailed": {
    message: "Uma etapa de configuração falhou.",
    description:
      "Failed build, failureCode setup_failed (the reason below names the step and exit code)",
  },
  "agentEnvironment.repoEnvs.builds.failure.stepTimeout": {
    message: "Uma etapa de configuração ultrapassou o limite de 3 horas.",
    description: "Failed build, failureCode step_timeout",
  },
  "agentEnvironment.repoEnvs.builds.failure.buildTimeout": {
    message: "O build ultrapassou o limite de 3 horas.",
    description: "Failed build, failureCode build_timeout",
  },
  "agentEnvironment.repoEnvs.builds.failure.checkoutFailed": {
    message:
      "Não foi possível fazer o checkout da branch padrão do repositório.",
    description: "Failed build, failureCode checkout_failed",
  },
  "agentEnvironment.repoEnvs.builds.failure.storageLimitReached": {
    message:
      "O snapshot ultrapassaria o limite de armazenamento de sandbox deste workspace.",
    description: "Failed build, failureCode storage_limit_reached",
  },
  "agentEnvironment.repoEnvs.builds.failure.internalError": {
    message:
      "O bex não conseguiu concluir o build do lado dele. Tente novamente.",
    description: "Failed build, failureCode internal_error",
  },
  "agentEnvironment.repoEnvs.builds.failure.unknown": {
    message: "O build falhou.",
    description: "Failed build with no recognized failureCode",
  },
  "agentEnvironment.repoEnvs.builds.title": {
    message: "Builds de {repo}",
    description: "Build log dialog title (repo is owner/name)",
  },
  "agentEnvironment.repoEnvs.builds.openLabel": {
    message: "Builds de {repo}",
    description:
      "Accessible label of a repository row's button that opens its builds and logs",
  },
  "agentEnvironment.repoEnvs.builds.description": {
    message:
      "As sessões começam a partir do build bem-sucedido mais recente cujas etapas de configuração correspondem às atuais. Quando nenhum corresponde, elas executam as etapas de configuração no início de cada turno.",
    description:
      "Build log dialog description: how sessions use snapshots and the fallback (ADR111 D4)",
  },
  "agentEnvironment.repoEnvs.builds.buildNow": {
    message: "Fazer build agora",
    description:
      "Button that queues a build of the repository's setup steps (admins only)",
  },
  "agentEnvironment.repoEnvs.builds.snapshotNote": {
    message:
      "Os arquivos que as etapas de configuração gravam ficam no snapshot, então não grave credenciais em arquivos. Os segredos chegam às etapas como variáveis de ambiente, que não são mantidas.",
    description: "Note beside Build now (Devin's warning, ADR111 D6)",
  },
  "agentEnvironment.repoEnvs.builds.readOnly": {
    message: "Somente administradores do workspace podem iniciar um build.",
    description: "Shown under a disabled Build now for non-admins",
  },
  "agentEnvironment.repoEnvs.builds.forbidden": {
    message: "Somente administradores do workspace podem iniciar um build.",
    description: "Build now refused with FORBIDDEN",
  },
  "agentEnvironment.repoEnvs.builds.nothingToBuild": {
    message:
      "Este ambiente não tem etapas de configuração, então não há nada para buildar. As verificações não precisam de build.",
    description:
      "Build now on a checks-only environment (also AGENT_REPO_ENVIRONMENT_NOTHING_TO_BUILD)",
  },
  "agentEnvironment.repoEnvs.builds.queued": {
    message: "Build de {repo} enfileirado",
    description: "Toast after Build now succeeds",
  },
  "agentEnvironment.repoEnvs.builds.rateLimited": {
    message:
      "Este repositório já iniciou 12 builds nas últimas 24 horas. Tente novamente mais tarde.",
    description:
      "AGENT_REPO_ENVIRONMENT_BUILD_RATE_LIMITED without a usable nextBuildAt",
  },
  "agentEnvironment.repoEnvs.builds.rateLimitedUntil": {
    message:
      "Este repositório já iniciou 12 builds nas últimas 24 horas. O próximo build pode começar em {time}.",
    description:
      "AGENT_REPO_ENVIRONMENT_BUILD_RATE_LIMITED; time is nextBuildAt in the viewer's local date and time",
  },
  "agentEnvironment.repoEnvs.builds.unavailable": {
    message: "Os builds de ambiente não estão disponíveis neste deployment.",
    description: "AGENT_REPO_ENVIRONMENT_BUILDS_UNAVAILABLE",
  },
  "agentEnvironment.repoEnvs.builds.buildFailed": {
    message: "Não foi possível iniciar o build. Tente novamente.",
    description: "Generic Build now failure",
  },
  "agentEnvironment.repoEnvs.builds.loadFailed": {
    message: "Não foi possível carregar os builds.",
    description: "Build history query failed",
  },
  "agentEnvironment.repoEnvs.builds.none": {
    message: "Nenhum build ainda.",
    description: "Build history list with no builds",
  },
  "agentEnvironment.repoEnvs.builds.noneLog": {
    message:
      "Este repositório ainda não passou por build. Fazer build agora executa as etapas de configuração uma vez e mantém o resultado como um snapshot.",
    description: "Build log panel when the repository has no builds",
  },
  "agentEnvironment.repoEnvs.builds.historyLabel": {
    message: "Histórico de builds",
    description: "Accessible label of the build history list",
  },
  "agentEnvironment.repoEnvs.builds.logTitle": {
    message: "Log do build",
    description: "Build log panel header when no build is selected",
  },
  "agentEnvironment.repoEnvs.builds.logFailed": {
    message: "Não foi possível carregar o log deste build.",
    description: "Single build (log) query failed",
  },
  "agentEnvironment.repoEnvs.builds.took": {
    message: "levou {duration}",
    description:
      "A finished build's wall time, e.g. 'took 3m 12s' (joined with ' · ')",
  },
  "agentEnvironment.repoEnvs.builds.imageRemoved": {
    message: "snapshot removido",
    description:
      "A successful build whose snapshot image was since deleted (older builds keep no image)",
  },
  "agentEnvironment.repoEnvs.builds.notBuilt": {
    message: "Ainda sem build",
    description: "Repository row: has setup steps but no build yet",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.queued": {
    message:
      "Este build está aguardando para começar. Os builds compartilham com as sessões o limite de sandbox de agentes do workspace.",
    description: "Build log of a queued build",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.building": {
    message:
      "As etapas de configuração estão em execução. O log delas aparece aqui quando o build terminar.",
    description: "Build log of a running build",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.cancelled": {
    message:
      "Este build foi cancelado: um build mais novo o substituiu ou o ambiente dele foi excluído.",
    description: "Build log of a cancelled build",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.failed": {
    message: "Nenhuma etapa de configuração foi executada.",
    description:
      "Build log of a build that failed before any step ran (e.g. the checkout failed)",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.other": {
    message: "Este build não registrou nenhuma etapa.",
    description: "Build log with no steps, any other status",
  },
  "agentEnvironment.repoEnvs.builds.stepExit": {
    message: "saída {code}",
    description: "A setup step's exit code badge",
  },
  "agentEnvironment.repoEnvs.builds.stepKilled": {
    message: "interrompida",
    description: "A setup step with no exit code: killed or never started",
  },
  "agentEnvironment.repoEnvs.builds.noOutput": {
    message: "Sem saída.",
    description: "A setup step that printed nothing",
  },
  "agentEnvironment.skills.title": {
    message: "Skills do agente",
    description: "Settings → Agent skills card title (ADR099)",
  },
  "agentEnvironment.skills.description": {
    message:
      "Instruções reutilizáveis que toda sessão de agente neste workspace recebe, como arquivos SKILL.md. O agente aplica uma skill quando ela se encaixa na tarefa, ou somente quando solicitado.",
    description: "Agent skills card description",
  },
  "agentEnvironment.skills.loadFailed": {
    message: "Não foi possível carregar as skills do agente.",
    description: "Agent skills card error when the list query fails",
  },
  "agentEnvironment.skills.empty": {
    message: "Nenhuma skill do agente ainda.",
    description: "Agent skills card empty state",
  },
  "agentEnvironment.skills.onlyWhenAsked": {
    message: "Somente quando solicitado",
    description:
      "Badge on a skill the agent applies only when a prompt invokes it",
  },
  "agentEnvironment.skills.enabledLabel": {
    message: "Ativar {name}",
    description: "Accessible label of a skill's enabled switch",
  },
  "agentEnvironment.skills.editLabel": {
    message: "Editar {name}",
    description: "Accessible label of a skill's edit button",
  },
  "agentEnvironment.skills.viewLabel": {
    message: "Ver {name}",
    description: "Accessible label of a skill's view button (non-admins)",
  },
  "agentEnvironment.skills.exportLabel": {
    message: "Baixar {name} como SKILL.md",
    description: "Accessible label of a skill's export button",
  },
  "agentEnvironment.skills.deleteLabel": {
    message: "Excluir {name}",
    description: "Accessible label of a skill's delete button",
  },
  "agentEnvironment.skills.deleteTitle": {
    message: "Excluir a skill {name}?",
    description: "Delete skill confirm title",
  },
  "agentEnvironment.skills.deleteBody": {
    message:
      "Os novos turnos do agente deixam de recebê-la. As sessões que já foram executadas mantêm a transcrição.",
    description: "Delete skill confirm body",
  },
  "agentEnvironment.skills.deleteConfirm": {
    message: "Excluir skill",
    description: "Delete skill confirm button",
  },
  "agentEnvironment.skills.new": {
    message: "Nova skill",
    description: "Button that opens the create-skill dialog",
  },
  "agentEnvironment.skills.import": {
    message: "Importar SKILL.md",
    description: "Button that imports a SKILL.md file",
  },
  "agentEnvironment.skills.help": {
    message:
      "Membros podem usar skills; somente administradores podem adicioná-las ou alterá-las.",
    description: "Agent skills help line for admins",
  },
  "agentEnvironment.skills.readOnly": {
    message:
      "Somente administradores do workspace podem adicionar ou alterar skills do agente.",
    description: "Agent skills help line for non-admins",
  },
  "agentEnvironment.skills.createTitle": {
    message: "Nova skill",
    description: "Create-skill dialog title",
  },
  "agentEnvironment.skills.editTitle": {
    message: "Editar {name}",
    description: "Edit-skill dialog title",
  },
  "agentEnvironment.skills.viewTitle": {
    message: "{name}",
    description: "View-skill dialog title (read-only)",
  },
  "agentEnvironment.skills.dialogDescription": {
    message:
      "O agente lê a descrição para decidir quando a skill se aplica e segue as instruções quando ela se aplica.",
    description: "Skill dialog description",
  },
  "agentEnvironment.skills.nameLabel": {
    message: "Nome",
    description: "Skill name field label",
  },
  "agentEnvironment.skills.namePlaceholder": {
    message: "run-integration-tests",
    description: "Skill name field placeholder",
  },
  "agentEnvironment.skills.descriptionLabel": {
    message: "Descrição",
    description: "Skill description field label",
  },
  "agentEnvironment.skills.descriptionPlaceholder": {
    message: "Quando usar esta skill, em uma linha",
    description: "Skill description field placeholder",
  },
  "agentEnvironment.skills.onlyWhenAskedLabel": {
    message: "Somente quando eu pedir",
    description: "Toggle: the agent applies the skill only when invoked",
  },
  "agentEnvironment.skills.onlyWhenAskedHint": {
    message: "Invoque-a pelo composer ou com @skills:name em um prompt.",
    description: "Hint under the only-when-asked toggle",
  },
  "agentEnvironment.skills.bodyLabel": {
    message: "Instruções",
    description: "Skill body field label",
  },
  "agentEnvironment.skills.bodyPlaceholder": {
    message: "Etapas que o agente segue, em Markdown.",
    description: "Skill body field placeholder",
  },
  "agentEnvironment.skills.contentLabel": {
    message: "SKILL.md",
    description: "Whole-file editor label",
  },
  "agentEnvironment.skills.editAsFile": {
    message: "Editar como SKILL.md",
    description: "Switch the dialog to the whole-file editor",
  },
  "agentEnvironment.skills.editAsFields": {
    message: "Editar campos",
    description: "Switch the dialog back to separate fields",
  },
  "agentEnvironment.skills.history.tab": {
    message: "Histórico de versões",
    description: "Skill dialog tab listing the saved versions",
  },
  "agentEnvironment.skills.history.editTab": {
    message: "Editar",
    description: "Skill dialog tab with the editing form",
  },
  "agentEnvironment.skills.history.current": {
    message: "Atual",
    description: "Badge on the newest saved version",
  },
  "agentEnvironment.skills.history.changes": {
    message: "Alterações",
    description: "Version view: line diff against the previous version",
  },
  "agentEnvironment.skills.history.content": {
    message: "Conteúdo",
    description: "Version view: the full SKILL.md",
  },
  "agentEnvironment.skills.history.load": {
    message: "Carregar no editor",
    description: "Button that fills the editor with the selected version",
  },
  "agentEnvironment.skills.history.loadMore": {
    message: "Carregar mais",
    description: "Button that loads the next page of versions",
  },
  "agentEnvironment.skills.history.noChanges": {
    message: "Esta versão não alterou nada.",
    description: "Changes view of a version identical to the previous one",
  },
  "agentEnvironment.skills.history.loadFailed": {
    message: "Não foi possível carregar o histórico de versões.",
    description: "Version history list query failed",
  },
  "agentEnvironment.skills.history.versionFailed": {
    message: "Não foi possível carregar esta versão.",
    description: "Single version query failed",
  },
  "agentEnvironment.skills.history.loadedNotice": {
    message:
      "Versão {version} carregada. Salve para mantê-la como uma nova versão.",
    description: "Notice in the editor after Load into editor",
  },
  "agentEnvironment.skills.viewTab": {
    message: "Pré-visualização",
    description: "Read-only skill dialog tab showing the skill",
  },
  "agentEnvironment.skills.save": {
    message: "Salvar skill",
    description: "Skill dialog save button",
  },
  "agentEnvironment.skills.cancel": {
    message: "Cancelar",
    description: "Skill dialog cancel button",
  },
  "agentEnvironment.skills.close": {
    message: "Fechar",
    description: "Skill dialog close button (read-only)",
  },
  "agentEnvironment.skills.saved": {
    message: "{name} salva",
    description: "Toast after a skill is saved",
  },
  "agentEnvironment.skills.deleted": {
    message: "{name} excluída",
    description: "Toast after a skill is deleted",
  },
  "agentEnvironment.skills.readFailed": {
    message: "Não foi possível abrir a skill. Tente novamente.",
    description: "Error when a single skill read fails",
  },
  "agentEnvironment.skills.importFailed": {
    message: "Não foi possível ler esse arquivo.",
    description: "Error when an imported file can't be read",
  },
  "agentEnvironment.skills.nameRequired": {
    message: "Insira um nome.",
    description: "Skill name empty",
  },
  "agentEnvironment.skills.nameInvalid": {
    message:
      "Use letras minúsculas, dígitos e hifens simples, até 64 caracteres.",
    description: "Skill name fails the Agent Skills format",
  },
  "agentEnvironment.skills.descriptionRequired": {
    message: "Insira uma descrição de uma linha.",
    description: "Skill description empty",
  },
  "agentEnvironment.skills.descriptionInvalid": {
    message: "Mantenha a descrição em uma linha de no máximo 1.024 caracteres.",
    description: "Skill description too long or multi-line",
  },
  "agentEnvironment.skills.bodyInvalid": {
    message: "As instruções são limitadas a 64 KiB.",
    description: "Skill body too large",
  },
  "agentEnvironment.skills.invalid": {
    message:
      "A skill não é válida. Verifique o nome, a descrição e as instruções.",
    description: "AGENT_SKILL_INVALID error",
  },
  "agentEnvironment.skills.limitExceeded": {
    message:
      "Isso atingiu um limite de skills do agente (100 por workspace, 1 MiB ativadas no total).",
    description: "AGENT_SKILL_LIMIT_EXCEEDED error",
  },
  "agentEnvironment.skills.conflict": {
    message:
      "Esta skill mudou desde que você a abriu, ou o nome já está em uso. Feche e abra novamente para ver a versão mais recente.",
    description: "AGENT_SKILL_CONFLICT error",
  },
  "agentEnvironment.skills.notFound": {
    message: "Essa skill não existe mais.",
    description: "AGENT_SKILL_NOT_FOUND error",
  },
  "agentEnvironment.skills.unavailable": {
    message: "As skills do agente não estão disponíveis nesta instalação.",
    description: "AGENT_SKILLS_UNAVAILABLE error",
  },
  "agentEnvironment.skills.forbidden": {
    message:
      "Somente administradores do workspace podem alterar as skills do agente.",
    description: "Skill write refused for a non-admin",
  },
  "agentEnvironment.skills.saveFailed": {
    message: "Não foi possível salvar a alteração. Tente novamente.",
    description: "Generic skill write failure",
  },
  "agentEnvironment.agentsTitle": {
    message: "Agentes disponíveis",
    description: "Settings → Agents: availability section heading",
  },
  "agentEnvironment.agentsDescription": {
    message:
      "Escolha com quais agentes os membros podem iniciar sessões. As sessões em execução continuam rodando quando você desativa um agente.",
    description: "Settings → Agents: availability section description",
  },
  "agentEnvironment.agentsDataUseNote": {
    message: "O provedor do modelo pode treinar com o que as sessões enviam.",
    description:
      "Settings → Agents: note under an agent whose provider trains on session data (Muse Code)",
  },
  "agentEnvironment.agentsDataUseTitle": {
    message: "Ativar {agent}?",
    description:
      "Confirm title before turning on an agent whose provider trains on session data",
  },
  "agentEnvironment.agentsDataUseBody.muse": {
    message:
      "Este agente é executado no nível de contribuidor da Meta. A Meta pode usar os prompts, o código, os arquivos e o contexto que as sessões enviam, e as respostas que recebem, para treinar seus modelos. Ao ativá-lo, fica registrado que você, como administrador, reconhece isso em nome do workspace.",
    description: "Confirm body: Muse Code's data-use acknowledgement (ADR108)",
  },
  "agentEnvironment.agentsDataUseConfirm": {
    message: "Reconhecer e ativar",
    description: "Confirm button: acknowledge data use and enable the agent",
  },
  "agentEnvironment.agentsSaved": {
    message: "Agentes disponíveis atualizados",
    description: "Toast after an availability change",
  },
  "agentEnvironment.agentsSaveFailed": {
    message:
      "Não foi possível atualizar os agentes disponíveis. Tente novamente.",
    description: "Error after a failed availability change",
  },
  "agentEnvironment.githubCliLabel": {
    message: "GitHub CLI (gh)",
    description:
      "Label of the switch that gives the workspace's agent sessions gh, the GitHub CLI (w10/m524)",
  },
  "agentEnvironment.githubCliHelp": {
    message:
      "Desativado, o gh deixa de estar disponível em todas as sessões de agente deste workspace, inclusive nas que estão em execução; os pushes do Git para o branch da sessão e as ferramentas de pull request do bex continuam funcionando.",
    description:
      "Help under the GitHub CLI switch: what turning it off does (w10/m524)",
  },
  "agentEnvironment.githubCliTurnedOn": {
    message: "GitHub CLI ativado para as sessões de agente",
    description:
      "Toast after an admin turns the sessions' GitHub CLI on (w10/m524)",
  },
  "agentEnvironment.githubCliTurnedOff": {
    message: "GitHub CLI desativado para as sessões de agente",
    description:
      "Toast after an admin turns the sessions' GitHub CLI off (w10/m524)",
  },
  "agentEnvironment.githubCliSaveFailed": {
    message:
      "Não foi possível alterar a configuração do GitHub CLI. Tente novamente.",
    description: "Error after a failed GitHub CLI switch (w10/m524)",
  },
  "agentEnvironment.githubCliMandatoryLabel": {
    message: "Tornar a desativação obrigatória",
    description:
      "Label of the switch that makes the workspace's GitHub CLI off a floor no automation or session can turn back on (w10/m561)",
  },
  "agentEnvironment.githubCliMandatoryHelp": {
    message:
      "Vale enquanto o GitHub CLI está desativado: nenhuma automação ou sessão pode então reativar o gh.",
    description:
      "Help under the mandatory switch: it applies only while GitHub CLI is off (w10/m561)",
  },
  "agentEnvironment.githubCliMandatoryTurnedOn": {
    message: "A desativação do GitHub CLI agora é obrigatória",
    description:
      "Toast after an admin makes the workspace's GitHub CLI off mandatory (w10/m561)",
  },
  "agentEnvironment.githubCliMandatoryTurnedOff": {
    message: "A desativação do GitHub CLI não é mais obrigatória",
    description:
      "Toast after an admin makes the workspace's GitHub CLI off no longer mandatory (w10/m561)",
  },
  "agentEnvironment.githubCliMandatorySaveFailed": {
    message:
      "Não foi possível alterar se a desativação do GitHub CLI é obrigatória. Tente novamente.",
    description: "Error after a failed mandatory switch (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsLabel": {
    message: "GitHub CLI para automações",
    description:
      "Label of the select that binds the gh of the sessions automations start: inherit, on or off (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsHelp": {
    message:
      "O gh das sessões que as automações iniciam, a menos que uma automação defina o próprio. “Herdar” segue a chave acima.",
    description:
      "Help under the automations GitHub CLI select: an automation's own binding wins, and inherit follows the workspace switch (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsInherited": {
    message: "As automações agora herdam a configuração do GitHub CLI",
    description:
      "Toast after an admin returns automations to inheriting the workspace's GitHub CLI switch (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsTurnedOn": {
    message: "GitHub CLI ativado para as automações",
    description:
      "Toast after an admin turns GitHub CLI on for the sessions automations start (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsTurnedOff": {
    message: "GitHub CLI desativado para as automações",
    description:
      "Toast after an admin turns GitHub CLI off for the sessions automations start (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsSaveFailed": {
    message:
      "Não foi possível alterar o GitHub CLI das automações. Tente novamente.",
    description:
      "Error after a failed automations GitHub CLI choice (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsLocked": {
    message:
      "A desativação do GitHub CLI é obrigatória, então as automações não podem ativar o gh.",
    description:
      "Mapped message for AGENT_ENVIRONMENT_GITHUB_CLI_LOCKED: automations cannot be bound on under a mandatory off (w10/m561)",
  },
  "agentEnvironment.modelsTitle": {
    message: "Modelos que os membros podem escolher",
    description:
      "Settings → Agents: heading of the list of platform models members can choose (w10/m527)",
  },
  "agentEnvironment.modelsDescription": {
    message:
      "Escolha quais modelos da plataforma os membros podem usar em sessões, mensagens e automações: todos, ou só alguns, por modelo ou por provedor. Uma sessão ou automação que já usa um modelo que você desativar passa para o modelo padrão a partir do próximo turno.",
    description:
      "Settings → Agents: description of the model list — what turning a model off does (w10/m527), and that it allows all models (w10/m592) or filters them by model or by provider (w10/m573)",
  },
  "agentEnvironment.modelsDefaultLocked": {
    message:
      "O modelo padrão da plataforma fica sempre ativado, porque as sessões recorrem a ele.",
    description:
      "Note under the platform catalog's default model, whose switch is locked on (w10/m527)",
  },
  "agentEnvironment.modelsNew": {
    message: "Novo no catálogo",
    description:
      "Note under a platform model that is off because it joined the catalog after an admin narrowed the list; an admin turns it on with its switch (w10/m568)",
  },
  "agentEnvironment.modelsTurnedOn": {
    message: "{model} ativado para os membros",
    description:
      "Toast after an admin turns a model on for members; {model} is its id (w10/m527)",
  },
  "agentEnvironment.modelsTurnedOff": {
    message: "{model} desativado para os membros",
    description:
      "Toast after an admin turns a model off for members; {model} is its id (w10/m527)",
  },
  "agentEnvironment.modelsInvalid": {
    message:
      "Não foi possível atualizar os modelos: o modelo padrão da plataforma precisa continuar ativado, e a lista só pode incluir modelos do catálogo da plataforma. Recarregue a página para ver o catálogo atual.",
    description:
      "Server refusal AGENT_ENVIRONMENT_MODELS_INVALID of a model list change (w10/m527)",
  },
  "agentEnvironment.modelsSaveFailed": {
    message:
      "Não foi possível atualizar os modelos que os membros podem escolher. Tente novamente.",
    description: "Error after a failed model switch (w10/m527)",
  },
  "agentEnvironment.modelsFilterLabel": {
    message: "Filtrar os modelos que os membros podem escolher",
    description:
      "Settings → Agents: accessible name of the choice between every platform model (w10/m592) and filtering them by model or by provider (w10/m573)",
  },
  "agentEnvironment.modelsAll": {
    message: "Todos",
    description:
      "Settings → Agents: option that lets members choose every platform model, one added to the catalog later included — Devin's “Allow all models”, every workspace's default (w10/m592). Keep it short: the three options share one line on a 390 px phone",
  },
  "agentEnvironment.modelsByModel": {
    message: "Por modelo",
    description:
      "Settings → Agents: option that filters the platform models members can choose one model at a time (w10/m573)",
  },
  "agentEnvironment.modelsByProvider": {
    message: "Por provedor",
    description:
      "Settings → Agents: option that filters the platform models members can choose by provider, the prefix of a model id before / (w10/m573)",
  },
  "agentEnvironment.modelsAllHelp": {
    message:
      "Os membros podem escolher qualquer modelo da plataforma, inclusive os que a plataforma adicionar mais tarde. Para desativar modelos, filtre por modelo ou por provedor.",
    description:
      "Settings → Agents: help under the choice while it is All models — every platform model, a later one included; filtering by model or by provider turns models off (w10/m592)",
  },
  "agentEnvironment.modelsByModelHelp": {
    message:
      "Os membros só podem escolher os modelos que você ativar aqui. Um modelo que a plataforma adicionar mais tarde fica desativado até você ativá-lo, mesmo que todos os modelos daqui estejam ativados.",
    description:
      "Settings → Agents: help under the choice while it is by model — an allowlist: members pick only the models on, and a model added to the catalog later stays off even while every listed model is on (w10/m568, w10/m573, w10/m592)",
  },
  "agentEnvironment.modelsByProviderHelp": {
    message:
      "Os membros podem escolher qualquer modelo dos provedores que você permitir, inclusive os que a plataforma adicionar mais tarde. Os modelos de outros provedores ficam desativados.",
    description:
      "Settings → Agents: help under the filter choice while it is by provider — every model of an allowed provider, a later one included, and other providers' models off (w10/m573)",
  },
  "agentEnvironment.modelsProviderDefault": {
    message: "{model} (o padrão, sempre ativado)",
    description:
      "Settings → Agents, by provider: the catalog's default model in the list under its provider's switch; {model} is its id (w10/m573)",
  },
  "agentEnvironment.modelsProviderLastLocked": {
    message:
      "Pelo menos um provedor continua permitido. Para oferecer só o modelo padrão, filtre por modelo.",
    description:
      "Settings → Agents, by provider: note under the one provider still allowed, whose switch is locked on (w10/m573)",
  },
  "agentEnvironment.modelsProviderTurnedOn": {
    message: "Modelos de {provider} ativados para os membros",
    description:
      "Toast after an admin allows a provider's models for members; {provider} is the provider id, e.g. anthropic (w10/m573)",
  },
  "agentEnvironment.modelsProviderTurnedOff": {
    message: "Modelos de {provider} desativados para os membros",
    description:
      "Toast after an admin stops allowing a provider's models for members; {provider} is the provider id (w10/m573)",
  },
  "agentEnvironment.modelsAllowedAll": {
    message:
      "Agora os membros escolhem entre todos os modelos, inclusive os que a plataforma adicionar mais tarde",
    description:
      "Toast after an admin switches the model list to All models (w10/m592)",
  },
  "agentEnvironment.modelsFilteredByProvider": {
    message:
      "Agora os membros escolhem entre os modelos dos provedores que você permite",
    description:
      "Toast after an admin switches the model list to filtering by provider (w10/m573)",
  },
  "agentEnvironment.modelsFilteredByModel": {
    message: "Agora os membros escolhem entre os modelos que você permite",
    description:
      "Toast after an admin switches the model list to filtering by model, from All models or from by provider (w10/m573, w10/m592)",
  },
  "agentEnvironment.modelsProvidersInvalid": {
    message:
      "Não foi possível atualizar os provedores: a lista precisa incluir pelo menos um provedor do catálogo da plataforma. Recarregue a página para ver o catálogo atual.",
    description:
      "Server refusal AGENT_ENVIRONMENT_MODELS_INVALID of a provider filter change (field enabledProviders, w10/m573)",
  },
  "agentEnvironment.modelsAllowAllInvalid": {
    message:
      "Não foi possível alterar os modelos que os membros podem escolher: este workspace não usa mais o catálogo de modelos da plataforma. Recarregue a página para ver as configurações atuais.",
    description:
      "Server refusal AGENT_ENVIRONMENT_MODELS_INVALID of a switch to or from All models (field allowAllModels): the workspace no longer uses the platform model catalog (w10/m592)",
  },
  "agentEnvironment.modelOffOption": {
    message: "{model} (desativado)",
    description:
      "Default model picker: the saved default, which an admin turned off (w10/m527)",
  },
  "agentEnvironment.modelDefaultOffNote": {
    message:
      "{model} está desativado, então sessões e automações que não indicam um modelo usam o padrão da plataforma, {fallback}, até que um administrador escolha um modelo ativado ou ative {model} de novo.",
    description:
      "Note under the default model picker when the saved default is turned off; {fallback} is the platform default that runs instead (w10/m527)",
  },
  "agentEnvironment.modelDefaultRetiredNote": {
    message:
      "{model} não está mais no catálogo da plataforma, então sessões e automações que não indicam um modelo usam o padrão da plataforma, {fallback}, até que um administrador escolha outro modelo.",
    description:
      "Note under the default model picker when the saved default left the platform catalog; {fallback} is the platform default that runs instead (w10/m527)",
  },
  "agentEnvironment.idleTimeoutTitle": {
    message: "Tempo limite de inatividade",
    description: "Heading of the Enterprise idle timeout section (w6/m563)",
  },
  "agentEnvironment.idleTimeoutDescription": {
    message:
      "Por quanto tempo o sandbox de uma sessão continua ativo depois que o turno termina ou o último editor se desconecta. Uma mensagem para uma sessão encerrada inicia um sandbox novo que continua a conversa. Os sandboxes de automações e de sessões iniciadas por outra sessão são encerrados assim que o turno termina.",
    description:
      "Description of the idle timeout section: what it bounds, and that automations' and child sessions' sandboxes close at once",
  },
  "agentEnvironment.idleTimeoutLabel": {
    message: "Minutos",
    description: "Label of the idle timeout's minutes field",
  },
  "agentEnvironment.idleTimeoutSave": {
    message: "Salvar tempo limite",
    description: "Button that saves the idle timeout",
  },
  "agentEnvironment.idleTimeoutHelp": {
    message:
      "De {min} a {max} minutos. Workspaces sem valor próprio usam {min}.",
    description: "Help under the idle timeout field; {min} is also the default",
  },
  "agentEnvironment.idleTimeoutInvalid": {
    message: "Insira um número inteiro de minutos entre {min} e {max}.",
    description:
      "Client/server refusal of an idle timeout outside {min}-{max} minutes",
  },
  "agentEnvironment.idleTimeoutEnterpriseOnly": {
    message:
      "Somente workspaces Enterprise podem alterar o tempo limite de inatividade.",
    description:
      "Server refusal of an idle timeout on a workspace no longer on Enterprise",
  },
  "agentEnvironment.idleTimeoutSaved": {
    message: "Tempo limite de inatividade definido para {minutes} minutos",
    description: "Toast after the idle timeout saved",
  },
  "agentEnvironment.idleTimeoutSaveFailed": {
    message:
      "Não foi possível salvar o tempo limite de inatividade. Tente novamente.",
    description: "Generic idle timeout write failure",
  },
};

export default ptAgentEnvironment;
