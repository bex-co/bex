import type { TranslationEntry } from "@/i18n";

const ptLegal: Record<string, TranslationEntry> = {
  "legal.termsLink": {
    message: "Termos",
    description:
      "Link to the agent model usage terms, after a pricing statement",
  },
  "legal.agentModels.title": {
    message: "Termos de uso do modelo do agente",
    description: "Agent model terms page heading and browser title",
  },
  "legal.agentModels.effective": {
    message: "Em vigor desde {date}",
    description: "Effective date of the current terms version",
  },
  "legal.agentModels.scopeTitle": {
    message: "Escopo",
    description: "Terms section heading",
  },
  "legal.agentModels.scopeBody": {
    message:
      "Estes termos se aplicam quando as sessões de agente de um workspace usam o modelo fornecido pelo bex em vez da chave de provedor de modelo do próprio workspace.",
    description: "Terms: scope",
  },
  "legal.agentModels.dataTitle": {
    message: "Seus dados",
    description: "Terms section heading",
  },
  "legal.agentModels.dataSend": {
    message:
      "Para gerar respostas, o bex envia os prompts, o código e a saída de ferramentas da sessão a provedores de modelos de terceiros por meio do OpenRouter.",
    description: "Terms: what is sent and to whom",
  },
  "legal.agentModels.dataRouting": {
    message:
      "As requisições são roteadas apenas para provedores que não armazenam prompts além do processamento e não treinam com eles: o bex usa o roteamento de provedores do OpenRouter com a coleta de dados negada.",
    description: "Terms: the routing policy",
  },
  "legal.agentModels.dataMuse": {
    message:
      "As sessões do Muse Code são a exceção. O bex as envia diretamente à API de modelos da Meta no nível de colaborador, em que a Meta pode usar os prompts, o código, os arquivos e o contexto enviados, e as respostas retornadas, para treinar seus modelos. O Muse Code permanece desativado em um workspace até que um administrador o ative e reconheça isso.",
    description:
      "Terms: the Muse Code exception to the routing policy (ADR108)",
  },
  "legal.agentModels.dataOwnKey": {
    message:
      "Se o workspace usar a própria chave de provedor, as requisições vão ao seu provedor conforme o seu contrato com ele, e o bex não cobra tokens de modelo.",
    description: "Terms: the bring-your-own-key alternative",
  },
  "legal.agentModels.priceTitle": {
    message: "Preço",
    description: "Terms section heading",
  },
  "legal.agentModels.priceBody": {
    message:
      "Cada requisição é cobrada pelo custo real do provedor do modelo mais 30%. O uso é medido por requisição e exibido em Faturamento → Modelo do agente.",
    description: "Terms: price",
  },
  "legal.agentModels.capsTitle": {
    message: "Limites de gastos",
    description: "Terms section heading",
  },
  "legal.agentModels.capsPlan": {
    message:
      "Cada workspace tem um limite de gastos mensal definido pelo plano: Hobby US$ 20, Pro US$ 200, Scale US$ 2.000. Os administradores do workspace podem ajustá-lo até 10 vezes o limite do plano.",
    description: "Terms: plan caps",
  },
  "legal.agentModels.capsNew": {
    message:
      "Um workspace novo pode executar duas sessões simultâneas no modelo fornecido pelo bex até o primeiro pagamento bem-sucedido.",
    description: "Terms: new-workspace limits",
  },
  "legal.agentModels.capsReached": {
    message:
      "Quando o limite é atingido, novas requisições ao modelo são recusadas até o início do mês seguinte ou até um administrador aumentar o limite.",
    description: "Terms: what happens at the cap",
  },
  "legal.agentModels.chargesTitle": {
    message: "Cobranças durante o mês",
    description: "Terms section heading",
  },
  "legal.agentModels.chargesThreshold": {
    message:
      "Você autoriza o bex a cobrar o método de pagamento do workspace sempre que o uso não faturado do modelo do agente atingir o limite de cobrança do workspace, além da fatura mensal. O workspace continua funcionando enquanto a cobrança é feita. O limite começa em US$ 10 e aumenta com o histórico de pagamentos do workspace.",
    description: "Terms: threshold charge authorization",
  },
  "legal.agentModels.chargesFailed": {
    message:
      "Se uma cobrança falhar ou exigir sua autenticação, o uso do modelo do agente é pausado até o saldo ser pago.",
    description: "Terms: failed charge",
  },
  "legal.agentModels.deletionTitle": {
    message: "Exclusão de um workspace ou conta",
    description: "Terms section heading",
  },
  "legal.agentModels.deletionSettle": {
    message:
      "Antes de um workspace ou conta ser excluído, o bex cobra o uso não faturado. Se essa cobrança falhar, a exclusão aguarda até o saldo ser pago.",
    description: "Terms: settle before delete",
  },
  "legal.agentModels.deletionAuthorize": {
    message:
      "Você autoriza o bex a cobrar pelo uso incorrido antes da exclusão, inclusive depois dela.",
    description: "Terms: charges after deletion",
  },
  "legal.agentModels.paymentTitle": {
    message: "Métodos de pagamento",
    description: "Terms section heading",
  },
  "legal.agentModels.paymentBody": {
    message:
      "Cartões pré-pagos não são aceitos. Um cartão vinculado a um saldo não pago do bex pode ser recusado.",
    description: "Terms: card restrictions",
  },
  "legal.agentModels.recordsTitle": {
    message: "Registros de uso",
    description: "Terms section heading",
  },
  "legal.agentModels.recordsBody": {
    message:
      "O bex mantém um registro de cada requisição (horário, modelo, tokens e preço) por 13 meses. Esses registros podem ser usados como prova se uma cobrança for contestada.",
    description: "Terms: usage record retention",
  },
  "legal.agentModels.changesTitle": {
    message: "Alterações nestes termos",
    description: "Terms section heading",
  },
  "legal.agentModels.changesBody": {
    message:
      "O bex pode atualizar estes termos. A data de vigência no topo desta página indica a versão atual.",
    description: "Terms: changes",
  },
};

export default ptLegal;
