import type { TranslationEntry } from "@/i18n";

const zhInvites: Record<string, TranslationEntry> = {
  "invites.title": {
    message: "工作区邀请",
    description: "Invitation review: title",
  },
  "invites.authenticate": {
    message: "创建账户或登录以查看工作区邀请。",
    description: "Invitation review: authenticate",
  },
  "invites.signUp": {
    message: "注册",
    description: "Invitation review: signUp",
  },
  "invites.signIn": {
    message: "登录",
    description: "Invitation review: signIn",
  },
  "invites.joinTitle": {
    message: "加入 {workspace}",
    description: "Invitation review: joinTitle",
  },
  "invites.memberTitle": {
    message: "你已是 {workspace} 的成员",
    description: "Invitation review: memberTitle",
  },
  "invites.role": {
    message: "你将以 {role} 的角色加入。",
    description: "Invitation review: role",
  },
  "invites.memberRole": {
    message: "你的角色是 {role}。",
    description: "Invitation review: memberRole",
  },
  "invites.inviter": {
    message: "邀请人：{email}",
    description: "Invitation review: inviter",
  },
  "invites.account": {
    message: "当前登录账户：{email}",
    description: "Invitation review: account",
  },
  "invites.join": {
    message: "加入工作区",
    description: "Invitation review: join",
  },
  "invites.joining": {
    message: "正在加入…",
    description: "Invitation review: joining",
  },
  "invites.open": {
    message: "打开工作区",
    description: "Invitation review: open",
  },
  "invites.opening": {
    message: "正在打开…",
    description: "Invitation review: opening",
  },
  "invites.notNow": {
    message: "暂不处理",
    description: "Invitation review: notNow",
  },
  "invites.continue": {
    message: "继续前往 bex",
    description: "Invitation review: continue",
  },
  "invites.unavailableTitle": {
    message: "邀请不可用",
    description: "Invitation review: unavailableTitle",
  },
  "invites.invalid": {
    message: "此邀请链接无效或已被撤销。请向工作区管理员索取新邀请。",
    description: "Invitation review: invalid",
  },
  "invites.expired": {
    message: "此邀请已过期。请让工作区管理员重新发送。",
    description: "Invitation review: expired",
  },
  "invites.used": {
    message: "此邀请已被使用。请使用已加入的账户登录，或索取新邀请。",
    description: "Invitation review: used",
  },
  "invites.planLimit": {
    message:
      "此工作区当前的套餐无法接纳更多成员。请让管理员检查套餐和席位后重试。",
    description: "Invitation review: planLimit",
  },
  "invites.retryError": {
    message: "无法完成请求。邀请已保存在此标签页中，请重试。",
    description: "Invitation review: retryError",
  },
  "invites.accessPending": {
    message: "你的成员资格已确认，但工作区暂时无法打开。请重试。",
    description: "Invitation review: accessPending",
  },
  "invites.storageUnavailable": {
    message: "无法在此浏览器中保存邀请。请允许网站存储，然后重新打开邀请链接。",
    description: "Invitation review: storageUnavailable",
  },
  "invites.retry": { message: "重试", description: "Invitation review: retry" },
};

export default zhInvites;
