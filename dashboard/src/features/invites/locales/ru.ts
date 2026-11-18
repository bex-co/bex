import type { TranslationEntry } from "@/i18n";

const ruInvites: Record<string, TranslationEntry> = {
  "invites.title": {
    message: "Приглашение в рабочее пространство",
    description: "Invitation review: title",
  },
  "invites.authenticate": {
    message:
      "Создайте аккаунт или войдите, чтобы просмотреть приглашение в рабочее пространство.",
    description: "Invitation review: authenticate",
  },
  "invites.signUp": {
    message: "Зарегистрироваться",
    description: "Invitation review: signUp",
  },
  "invites.signIn": {
    message: "Войти",
    description: "Invitation review: signIn",
  },
  "invites.joinTitle": {
    message: "Присоединиться к {workspace}",
    description: "Invitation review: joinTitle",
  },
  "invites.memberTitle": {
    message: "Вы участник {workspace}",
    description: "Invitation review: memberTitle",
  },
  "invites.role": {
    message: "Вы присоединитесь в роли: {role}.",
    description: "Invitation review: role",
  },
  "invites.memberRole": {
    message: "Ваша роль: {role}.",
    description: "Invitation review: memberRole",
  },
  "invites.inviter": {
    message: "Пригласил(а): {email}",
    description: "Invitation review: inviter",
  },
  "invites.account": {
    message: "Вы вошли как {email}",
    description: "Invitation review: account",
  },
  "invites.join": {
    message: "Присоединиться к рабочему пространству",
    description: "Invitation review: join",
  },
  "invites.joining": {
    message: "Присоединение…",
    description: "Invitation review: joining",
  },
  "invites.open": {
    message: "Открыть рабочее пространство",
    description: "Invitation review: open",
  },
  "invites.opening": {
    message: "Открытие…",
    description: "Invitation review: opening",
  },
  "invites.notNow": {
    message: "Не сейчас",
    description: "Invitation review: notNow",
  },
  "invites.continue": {
    message: "Перейти в bex",
    description: "Invitation review: continue",
  },
  "invites.unavailableTitle": {
    message: "Приглашение недоступно",
    description: "Invitation review: unavailableTitle",
  },
  "invites.invalid": {
    message:
      "Эта ссылка-приглашение недействительна или была отозвана. Попросите администратора рабочего пространства прислать новое приглашение.",
    description: "Invitation review: invalid",
  },
  "invites.expired": {
    message:
      "Срок действия этого приглашения истёк. Попросите администратора рабочего пространства отправить его повторно.",
    description: "Invitation review: expired",
  },
  "invites.used": {
    message:
      "Это приглашение уже использовано. Войдите в аккаунт, который присоединился, или попросите новое приглашение.",
    description: "Invitation review: used",
  },
  "invites.planLimit": {
    message:
      "На текущем тарифе это рабочее пространство не может принять ещё одного участника. Попросите администратора рабочего пространства проверить тариф и количество мест, затем повторите попытку.",
    description: "Invitation review: planLimit",
  },
  "invites.retryError": {
    message:
      "Не удалось выполнить этот запрос. Ваше приглашение сохранено в этой вкладке. Попробуйте ещё раз.",
    description: "Invitation review: retryError",
  },
  "invites.accessPending": {
    message:
      "Ваше участие подтверждено, но доступ к рабочему пространству ещё не готов. Попробуйте открыть рабочее пространство снова.",
    description: "Invitation review: accessPending",
  },
  "invites.storageUnavailable": {
    message:
      "Не удалось сохранить ваше приглашение в этом браузере. Разрешите хранилище сайта, затем снова откройте ссылку-приглашение.",
    description: "Invitation review: storageUnavailable",
  },
  "invites.retry": {
    message: "Повторить",
    description: "Invitation review: retry",
  },
  "invites.orgTitle": {
    message: "Приглашение в организацию",
    description: "Invitation review: orgTitle",
  },
  "invites.orgAuthenticate": {
    message:
      "Создайте аккаунт или войдите, чтобы посмотреть приглашение в организацию.",
    description: "Invitation review: orgAuthenticate",
  },
  "invites.orgJoinTitle": {
    message: "Присоединиться к {organization}",
    description: "Invitation review: orgJoinTitle",
  },
  "invites.orgMemberTitle": {
    message: "Вы состоите в {organization}",
    description: "Invitation review: orgMemberTitle",
  },
  "invites.orgRole": {
    message: "Ваша роль в организации: {role}.",
    description: "Invitation review: orgRole",
  },
  "invites.orgJoin": {
    message: "Присоединиться к организации",
    description: "Invitation review: orgJoin",
  },
  "invites.orgOpen": {
    message: "Открыть организацию",
    description: "Invitation review: orgOpen",
  },
  "invites.orgInvalid": {
    message:
      "Ссылка-приглашение недействительна или отозвана. Попросите владельца организации прислать новое приглашение.",
    description: "Invitation review: orgInvalid",
  },
  "invites.orgExpired": {
    message:
      "Срок действия приглашения истёк. Попросите владельца организации пригласить вас снова.",
    description: "Invitation review: orgExpired",
  },
  "invites.orgAccessPending": {
    message:
      "Членство подтверждено, но доступ к организации ещё не готов. Попробуйте открыть организацию ещё раз.",
    description: "Invitation review: orgAccessPending",
  },
  "invites.orgAccepted": {
    message: "Вы присоединились к {organization}.",
    description: "Invitation review: orgAccepted",
  },
};

export default ruInvites;
