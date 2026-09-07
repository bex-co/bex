// App Store Connect listing for co.bex.mobile, synced with `eas metadata:push`.
// Reviewer contact details and the demo account are read from the environment so
// they are never committed. Copy `.env.template` to `.env` and fill them in.
require("dotenv").config();

const env = (name) => process.env[name]?.trim() || undefined;

// App Review contact details and the demo account live only in the environment.
// The block is omitted entirely until all of them are present, so the listing
// copy can be pushed before the demo account exists. App Store *submission*
// still requires them; `eas metadata:push` will report the block as missing.
const REVIEW_VARS = [
  "ASC_REVIEW_FIRST_NAME",
  "ASC_REVIEW_LAST_NAME",
  "ASC_REVIEW_EMAIL",
  "ASC_REVIEW_PHONE",
  "ASC_DEMO_USERNAME",
  "ASC_DEMO_PASSWORD",
];
const hasReview = REVIEW_VARS.every((name) => env(name));

const SUPPORT_URL = "https://github.com/bex-co/bex/issues";
const MARKETING_URL = "https://bex.co";
const PRIVACY_URL = "https://bex.co/privacy-policy";

const description = `bex mobile is the supervision companion for the apps you run on bex. It is built for the moments you are away from your desk: a deploy fails, a service crashes, a cron run breaks, or a coding agent needs a decision.

ALERTS THAT RESPECT YOUR TIME
Push notifications for failed and successful deploys, crashed services, failed cron runs, and agent sessions that need your input. Urgency tiers, working hours, and quiet hours are configurable per service, so you are paged for what matters and left alone for what is not.

STATUS AT A GLANCE
Every service, Postgres database, and Key Value store in your workspace, grouped and glanceable, with current status, latest deploy, and month-to-date usage that is honest about how complete its evidence is.

EVIDENCE BEFORE ACTION
Follow live log tails filtered by app, build, pre-deploy, and request output. Read deploy and event timelines, CPU, memory, and network sparklines, Postgres connection, disk, and table telemetry, and Key Value availability and memory observations.

ONE-TAP SAFE ACTIONS
Deploy latest, cancel an active deploy, roll back, restart, suspend, and resume services and datastores. Every action sits behind a confirmation naming the exact target, and nothing is retried silently.

AGENT SESSIONS
Assign a coding-agent task to a repository from your phone, follow the transcript and its evidence, and get notified when the session needs a decision or opens a pull request.

WHAT IT DELIBERATELY DOES NOT DO
bex mobile is a supervision surface, not a second dashboard. It has no service creation, no blueprint or topology editing, no bulk secret management, no destructive datastore operations such as delete, point-in-time recovery, or failover, and no dangerous permission modes. Those stay on the desktop dashboard, where they belong.

SECURITY
Sign-in runs in your system browser over OAuth, so the app never sees your password or your MFA response. There is no in-app web login. Tokens live in the device keychain, never in app storage, logs, analytics, or crash reports. Protected local state is cleared when you sign out or cross a workspace boundary.

REQUIREMENTS
bex mobile requires a bex account and an existing workspace. Services, databases, and workspaces are created on the web dashboard at dashboard.bex.co.

bex is open source. Read the code or file an issue at github.com/bex-co/bex.`;

const descriptionZh = `bex mobile 是 bex 应用的运维监督伴侣。它专为你不在工位的时刻而设计：部署失败、服务崩溃、定时任务出错，或编码智能体需要你做决定。

尊重你时间的提醒
部署失败与成功、服务崩溃、定时任务失败，以及智能体会话需要你介入时，都会推送通知。紧急级别、工作时间与免打扰时段均可按服务配置，让该找你的事找到你，不该打扰的事保持安静。

一目了然的状态
工作区中的每个服务、Postgres 数据库与 Key Value 存储都分组呈现，包含当前状态、最近一次部署，以及对自身证据完整度保持诚实的当月用量。

先看证据，再动手
按应用、构建、预部署与请求类型筛选并跟随实时日志。查看部署与事件时间线、CPU、内存与网络走势图、Postgres 连接、磁盘与表级遥测，以及 Key Value 的可用性与内存观测。

一键安全操作
部署最新版本、取消进行中的部署、回滚、重启、暂停与恢复服务及数据存储。每个操作都需经过写明确切目标的二次确认，且绝不会静默重试。

智能体会话
在手机上把编码任务指派给某个代码仓库，跟进会话记录与证据，并在会话需要决策或提交拉取请求时收到通知。

刻意不做的事
bex mobile 是监督界面，而非第二个控制台。它不提供服务创建、蓝图或拓扑编辑、批量密钥管理，不提供删除、时间点恢复、故障转移等破坏性数据存储操作，也不提供危险权限模式。这些仍留在桌面端控制台。

安全性
登录在系统浏览器中通过 OAuth 完成，应用永远不会看到你的密码或多因素验证响应，也不使用应用内网页登录。令牌保存在设备钥匙串中，不写入应用存储、日志、分析或崩溃报告。退出登录或切换工作区时，受保护的本地状态会被清除。

使用要求
bex mobile 需要 bex 账号与已有的工作区。服务、数据库与工作区均在网页控制台 dashboard.bex.co 中创建。

bex 是开源项目，欢迎在 github.com/bex-co/bex 查看源码或提交问题。`;

module.exports = {
  configVersion: 0,
  apple: {
    version: "1.0",
    copyright: "2026 Stargately, Inc.",
    categories: ["DEVELOPER_TOOLS", "PRODUCTIVITY"],
    release: {
      automaticRelease: true,
    },
    info: {
      "en-US": {
        title: "bex.co",
        subtitle: "Deploy supervision on the go",
        description,
        keywords: [
          "paas",
          "deploy",
          "devops",
          "server",
          "logs",
          "monitoring",
          "hosting",
          "cloud",
          "ops",
          "oncall",
          "kubernetes",
          "alerts",
        ],
        promoText:
          "Deploy failures, crashed services, and agent decisions reach you wherever you are, with the logs, metrics, and one-tap safe actions to respond right away.",
        releaseNotes:
          "First release of bex mobile.\n\n- Push alerts for deploys, crashes, cron failures, and agent sessions, with urgency tiers and working hours\n- Service, Postgres, and Key Value status with month-to-date usage\n- Live log tails, deploy and event timelines, and metric sparklines\n- One-tap deploy, cancel, rollback, restart, suspend, and resume behind explicit confirmation\n- Agent sessions: assign a task to a repository and follow it to a pull request\n- English and Simplified Chinese",
        marketingUrl: MARKETING_URL,
        supportUrl: SUPPORT_URL,
        privacyPolicyUrl: PRIVACY_URL,
      },
      "zh-Hans": {
        title: "bex.co",
        subtitle: "随时随地监督你的部署",
        description: descriptionZh,
        keywords: [
          "paas",
          "部署",
          "运维",
          "服务器",
          "日志",
          "监控",
          "云",
          "托管",
          "告警",
          "值班",
        ],
        promoText:
          "部署失败、服务崩溃与智能体决策随时找到你，并附上日志、指标与一键安全操作，让你立即响应。",
        releaseNotes:
          "bex mobile 首个版本。\n\n- 部署、崩溃、定时任务失败与智能体会话的推送提醒，支持紧急级别与工作时间\n- 服务、Postgres 与 Key Value 状态，以及当月用量\n- 实时日志、部署与事件时间线、指标走势图\n- 一键部署、取消、回滚、重启、暂停与恢复，均需明确确认\n- 智能体会话：指派任务到代码仓库并跟进至拉取请求\n- 支持英文与简体中文",
        marketingUrl: MARKETING_URL,
        supportUrl: SUPPORT_URL,
        privacyPolicyUrl: PRIVACY_URL,
      },
    },
    ...(hasReview && {
      review: {
        firstName: env("ASC_REVIEW_FIRST_NAME"),
        lastName: env("ASC_REVIEW_LAST_NAME"),
        email: env("ASC_REVIEW_EMAIL"),
        phone: env("ASC_REVIEW_PHONE"),
        demoRequired: true,
        demoUsername: env("ASC_DEMO_USERNAME"),
        demoPassword: env("ASC_DEMO_PASSWORD"),
        notes: `bex mobile is a supervision client for the bex platform (https://bex.co). It requires an account and a workspace that already contains at least one deployed service.

SIGNING IN
Tap "Continue securely" on the launch screen. Sign-in opens the system browser for an OAuth flow at oauth.bex.co; the app never receives the password directly. Use the demo credentials above. After approving, the browser returns to the app automatically.

The demo workspace is pre-populated with a running web service, a Postgres database, and a Key Value store, so the Status, Activity, and Alerts tabs all have content on first launch.

PUSH NOTIFICATIONS
The app requests notification permission only after you tap "Enable" on the Alerts tab. Permission is not requested at launch. Alerts are operational events (deploy finished, service crashed, cron run failed) for the signed-in workspace.

AGENT SESSIONS
The Sessions tab requires the agent feature to be enabled for the workspace; it is enabled for the demo account. If a session shows as unavailable, it means the workspace-level configuration is off, which is expected behavior for accounts without it.

SAFE ACTIONS
Deploy, restart, suspend, and resume are real operations against the demo workspace and are safe to exercise. They are reversible from the same screen.`,
      },
    }),
    advisory: {
      ageRatingOverride: "NONE",
      alcoholTobaccoOrDrugUseOrReferences: "NONE",
      contests: "NONE",
      gambling: false,
      gamblingSimulated: "NONE",
      horrorOrFearThemes: "NONE",
      kidsAgeBand: null,
      koreaAgeRatingOverride: "NONE",
      lootBox: false,
      matureOrSuggestiveThemes: "NONE",
      medicalOrTreatmentInformation: "NONE",
      profanityOrCrudeHumor: "NONE",
      sexualContentGraphicAndNudity: "NONE",
      sexualContentOrNudity: "NONE",
      unrestrictedWebAccess: false,
      violenceCartoonOrFantasy: "NONE",
      violenceRealistic: "NONE",
      violenceRealisticProlongedGraphicOrSadistic: "NONE",
      advertising: false,
      ageAssurance: false,
      ageRatingOverrideV2: "NONE",
      developerAgeRatingInfoUrl: null,
      gunsOrOtherWeapons: "NONE",
      healthOrWellnessTopics: false,
      messagingAndChat: false,
      parentalControls: false,
      userGeneratedContent: false,
    },
  },
};
