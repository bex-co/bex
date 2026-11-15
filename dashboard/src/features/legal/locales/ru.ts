import type { TranslationEntry } from "@/i18n";

const ruLegal: Record<string, TranslationEntry> = {
  "legal.termsLink": {
    message: "Условия",
    description:
      "Link to the agent model usage terms, after a pricing statement",
  },
  "legal.agentModels.title": {
    message: "Условия использования моделей агентов",
    description: "Agent model terms page heading and browser title",
  },
  "legal.agentModels.effective": {
    message: "Действует с {date}",
    description: "Effective date of the current terms version",
  },
  "legal.agentModels.scopeTitle": {
    message: "Область применения",
    description: "Terms section heading",
  },
  "legal.agentModels.scopeBody": {
    message:
      "Эти условия применяются, когда сессии агентов рабочего пространства используют модель, предоставляемую bex, а не собственный ключ провайдера моделей рабочего пространства.",
    description: "Terms: scope",
  },
  "legal.agentModels.dataTitle": {
    message: "Ваши данные",
    description: "Terms section heading",
  },
  "legal.agentModels.dataSend": {
    message:
      "Чтобы генерировать ответы, bex отправляет промпты, код и вывод инструментов сессии сторонним провайдерам моделей через OpenRouter.",
    description: "Terms: what is sent and to whom",
  },
  "legal.agentModels.dataRouting": {
    message:
      "Запросы направляются только провайдерам, которые не хранят промпты дольше, чем нужно для обработки, и не обучаются на них: bex использует маршрутизацию провайдеров OpenRouter с запретом сбора данных.",
    description: "Terms: the routing policy",
  },
  "legal.agentModels.dataMuse": {
    message:
      "Сессии Muse Code — исключение. bex отправляет их напрямую в API моделей Meta на уровне для участников, при котором Meta может использовать отправленные промпты, код, файлы и контекст, а также возвращённые ответы для обучения своих моделей. Muse Code остаётся отключённым в рабочем пространстве, пока администратор не включит его и не подтвердит это.",
    description:
      "Terms: the Muse Code exception to the routing policy (ADR108)",
  },
  "legal.agentModels.dataOwnKey": {
    message:
      "Если рабочее пространство вместо этого использует собственный ключ провайдера, запросы поступают вашему провайдеру на условиях вашего соглашения с ним, а bex не выставляет счета за токены моделей.",
    description: "Terms: the bring-your-own-key alternative",
  },
  "legal.agentModels.priceTitle": {
    message: "Цена",
    description: "Terms section heading",
  },
  "legal.agentModels.priceBody": {
    message:
      "Каждый запрос оплачивается по фактической стоимости у провайдера модели плюс 30%. Использование учитывается по каждому запросу и отображается в разделе «Оплата → Модель агента».",
    description: "Terms: price",
  },
  "legal.agentModels.capsTitle": {
    message: "Лимиты расходов",
    description: "Terms section heading",
  },
  "legal.agentModels.capsPlan": {
    message:
      "У каждого рабочего пространства есть ежемесячный лимит расходов, определяемый тарифом: Hobby $20, Pro $200, Scale $2000. Администраторы рабочего пространства могут увеличить его до 10-кратного значения лимита тарифа.",
    description: "Terms: plan caps",
  },
  "legal.agentModels.capsNew": {
    message:
      "Новое рабочее пространство может запускать две параллельные сессии на модели, предоставляемой bex, до первого успешного платежа.",
    description: "Terms: new-workspace limits",
  },
  "legal.agentModels.capsReached": {
    message:
      "Когда лимит достигнут, новые запросы к моделям отклоняются до начала следующего месяца или до повышения лимита администратором.",
    description: "Terms: what happens at the cap",
  },
  "legal.agentModels.chargesTitle": {
    message: "Списания в течение месяца",
    description: "Terms section heading",
  },
  "legal.agentModels.chargesThreshold": {
    message:
      "Вы разрешаете bex списывать средства со способа оплаты рабочего пространства, когда его неоплаченное использование моделей агентов достигает порога списания рабочего пространства, в дополнение к ежемесячному счёту. Во время списания рабочее пространство продолжает работать. Порог начинается с $10 и растёт с историей платежей рабочего пространства.",
    description: "Terms: threshold charge authorization",
  },
  "legal.agentModels.chargesFailed": {
    message:
      "Если списание не удалось или требует вашей аутентификации, использование моделей агентов приостанавливается до оплаты задолженности.",
    description: "Terms: failed charge",
  },
  "legal.agentModels.deletionTitle": {
    message: "Удаление рабочего пространства или аккаунта",
    description: "Terms section heading",
  },
  "legal.agentModels.deletionSettle": {
    message:
      "Перед удалением рабочего пространства или аккаунта bex списывает плату за его неоплаченное использование. Если списание не удалось, удаление откладывается до оплаты задолженности.",
    description: "Terms: settle before delete",
  },
  "legal.agentModels.deletionAuthorize": {
    message:
      "Вы разрешаете bex списывать плату за использование, возникшее до удаления, в том числе после удаления.",
    description: "Terms: charges after deletion",
  },
  "legal.agentModels.paymentTitle": {
    message: "Способы оплаты",
    description: "Terms section heading",
  },
  "legal.agentModels.paymentBody": {
    message:
      "Предоплаченные карты не принимаются. Карта, привязанная к неоплаченной задолженности в bex, может быть отклонена.",
    description: "Terms: card restrictions",
  },
  "legal.agentModels.recordsTitle": {
    message: "Записи об использовании",
    description: "Terms section heading",
  },
  "legal.agentModels.recordsBody": {
    message:
      "bex хранит запись о каждом запросе (время, модель, токены и цена) в течение 13 месяцев. Эти записи могут использоваться как доказательство при оспаривании списания.",
    description: "Terms: usage record retention",
  },
  "legal.agentModels.changesTitle": {
    message: "Изменения этих условий",
    description: "Terms section heading",
  },
  "legal.agentModels.changesBody": {
    message:
      "bex может обновлять эти условия. Дата вступления в силу в верхней части страницы указывает на текущую версию.",
    description: "Terms: changes",
  },
};

export default ruLegal;
