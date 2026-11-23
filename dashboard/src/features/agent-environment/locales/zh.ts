import type { TranslationEntry } from "@/i18n";

const zhAgentEnvironment: Record<string, TranslationEntry> = {
  "agentEnvironment.title": {
    message: "智能体",
    description: "Settings → Agents card title and section-nav label",
  },
  "agentEnvironment.description": {
    message:
      "此工作区中的智能体会话如何访问网络以及使用哪个模型。适用于所有新会话。",
    description: "Agents settings card description",
  },
  "agentEnvironment.loadFailed": {
    message: "无法加载智能体设置。",
    description: "Agents card error when the environment query fails",
  },
  "agentEnvironment.retry": {
    message: "重试",
    description: "Retry button on the Agents card load error",
  },
  "agentEnvironment.readOnlyNote": {
    message: "只有工作区管理员可以更改智能体设置。",
    description: "Read-only notice shown to non-admin members",
  },
  "agentEnvironment.networkTitle": {
    message: "网络访问",
    description: "Heading (and radiogroup label) of the network tier picker",
  },
  "agentEnvironment.networkDescription": {
    message: "GitHub 和模型提供方始终可访问。选择智能体沙箱还能访问哪些地址。",
    description: "Network tier picker description",
  },
  "agentEnvironment.tierRegistriesTitle": {
    message: "软件包仓库（推荐）",
    description: "Network tier option: built-in package registries",
  },
  "agentEnvironment.tierRegistriesDescription": {
    message:
      "智能体可以从 npm、PyPI、Go、Maven、crates、RubyGems 和 Docker Hub 安装依赖，从而构建和测试你的代码。",
    description: "Description of the registries network tier",
  },
  "agentEnvironment.tierCustomTitle": {
    message: "自定义",
    description: "Network tier option: registries plus listed hosts",
  },
  "agentEnvironment.tierCustomDescription": {
    message: "软件包仓库，加上你在下方列出的主机名。",
    description: "Description of the custom network tier",
  },
  "agentEnvironment.tierFullTitle": {
    message: "完整互联网",
    description: "Network tier option: any public host over HTTP(S)",
  },
  "agentEnvironment.tierFullDescription": {
    message:
      "可通过 bex 出站代理以 HTTP(S) 访问任何公共网站。直接调用模型提供商 API 仍被屏蔽（智能体自身的模型不受影响），且不支持 SSH 及其他非 HTTP 协议。",
    description:
      "Description of the full-internet tier: HTTP(S) via the egress proxy, direct model-provider API calls blocked, no SSH/non-HTTP",
  },
  "agentEnvironment.registryHostsSummary_other": {
    message: "包含的软件包仓库（{count} 个主机）",
    description:
      "Disclosure summary listing the built-in registry hosts (many)",
  },
  "agentEnvironment.hostsLabel": {
    message: "额外主机名",
    description: "Label of the custom-tier hostname textarea",
  },
  "agentEnvironment.hostsHelp": {
    message:
      "每行一个主机名，最多 {max} 个。仅限精确名称：不含协议、端口、路径或通配符。",
    description: "Help under the custom hostname textarea",
  },
  "agentEnvironment.hostsError.too_many_entries": {
    message: "最多只能列出 {max} 个主机名。",
    description: "Custom hosts error: too many entries",
  },
  "agentEnvironment.hostsError.empty": {
    message: "主机名不能为空。",
    description: "Custom hosts error: empty entry",
  },
  "agentEnvironment.hostsError.whitespace": {
    message: "“{entry}”前后含有空格。",
    description: "Custom hosts error: surrounding whitespace",
  },
  "agentEnvironment.hostsError.not_a_hostname": {
    message:
      "“{entry}”不是精确的主机名。请使用 api.example.com 这样的名称，不要包含协议、端口、路径或通配符。",
    description: "Custom hosts error: not a hostname",
  },
  "agentEnvironment.hostsError.ip_literal": {
    message: "“{entry}”是 IP 地址。请改用主机名。",
    description: "Custom hosts error: IP literal",
  },
  "agentEnvironment.hostsError.private_name": {
    message: "“{entry}”是私有或集群内部名称，无法放行。",
    description: "Custom hosts error: private name",
  },
  "agentEnvironment.hostsError.already_baseline": {
    message:
      "“{entry}”已可访问（或只能通过模型代理访问）。请将其从列表中移除。",
    description: "Custom hosts error: already in the baseline",
  },
  "agentEnvironment.hostsError.duplicate": {
    message: "“{entry}”重复列出。",
    description: "Custom hosts error: duplicate",
  },
  "agentEnvironment.hostsError.invalid": {
    message: "主机名列表无效。",
    description: "Custom hosts error: unknown reason",
  },
  "agentEnvironment.modelLabel": {
    message: "默认模型",
    description: "Label of the default model field",
  },
  "agentEnvironment.modelPlaceholder": {
    message: "提供方默认",
    description: "Placeholder of the empty default model field",
  },
  "agentEnvironment.modelHelp": {
    message:
      "新会话使用的模型 ID，例如 claude-sonnet-4-5。留空则使用提供方默认模型。",
    description: "Help under the default model field",
  },
  "agentEnvironment.modelPlatformDefault": {
    message: "平台默认（{model}）",
    description:
      "Default model picker — no default: the platform catalog's first model (w10/m516)",
  },
  "agentEnvironment.modelCatalogHelp": {
    message:
      "未指定模型的新会话和自动化将使用此模型。成员仍可为单个会话选择平台目录中的其他模型。",
    description: "Help under the platform model catalog picker (w10/m516)",
  },
  "agentEnvironment.modelInvalid": {
    message: "请使用不含空格的模型 ID（最多 255 个字符）。",
    description: "Default model validation error",
  },
  "agentEnvironment.modelNotOfferedError": {
    message: "平台模型不提供 {model}。请从列表中选择。",
    description:
      "Server refusal: the default model is not in the platform catalog (w10/m516)",
  },
  "agentEnvironment.templateLabel": {
    message: "默认沙箱",
    description: "Label of the default sandbox template picker (w10/m519)",
  },
  "agentEnvironment.templateHelp": {
    message:
      "未指定沙箱的新会话和自动化将在此沙箱上运行。仅对之后启动的会话生效；成员仍可为单个会话选择其他沙箱。",
    description: "Help under the default sandbox template picker (w10/m519)",
  },
  "agentEnvironment.templateInvalidError": {
    message: "{template} 不是沙箱模板。请从以下模板中选择：{templates}。",
    description:
      "Server refusal: the default sandbox template is not a registered one; {templates} lists those (w10/m519)",
  },
  "agentEnvironment.tierInvalid": {
    message: "不支持该网络访问选项。",
    description: "Server refusal: invalid tier",
  },
  "agentEnvironment.forbidden": {
    message: "只有工作区管理员可以更改智能体设置。",
    description: "Server refusal: caller is not an admin",
  },
  "agentEnvironment.saveFailed": {
    message: "无法保存智能体设置，请重试。",
    description: "Generic save failure",
  },
  "agentEnvironment.saveFailedReason": {
    message: "无法保存智能体设置：{reason}",
    description: "Save failure with the server's reason",
  },
  "agentEnvironment.save": {
    message: "保存",
    description: "Agents card save button",
  },
  "agentEnvironment.saved": {
    message: "智能体设置已保存",
    description: "Success toast after saving",
  },
  "agentEnvironment.modelKeyTitle": {
    message: "模型提供方",
    description: "Heading of the model source / provider key section",
  },
  "agentEnvironment.modelKeyDescription": {
    message:
      "除非工作区添加了自己的提供方密钥，否则智能体会话使用 bex 提供的模型。使用你自己的密钥时，会话直接调用你的提供方，bex 只收取沙箱计算费用。",
    description: "Explains the model source choice (ADR091 D7)",
  },
  "agentEnvironment.modelSourceLabel": {
    message: "模型来源：",
    description: "Label before the current model source",
  },
  "agentEnvironment.modelSourcePlatform": {
    message: "bex 提供（按模型提供方成价 + 30% 计费）",
    description: "Model source: the platform model",
  },
  "agentEnvironment.modelSourceByo": {
    message: "你自己的密钥（bex 不收取 token 费用）",
    description: "Model source: the workspace's own key",
  },
  "agentEnvironment.modelSourceNone": {
    message: "无：添加提供方密钥后才能启动智能体会话",
    description: "Model source: no key available",
  },
  "agentEnvironment.modelKeyLabel": {
    message: "提供方 API 密钥",
    description: "Label of the write-only key field when none is set",
  },
  "agentEnvironment.modelKeyReplaceLabel": {
    message: "替换提供方 API 密钥",
    description: "Label of the write-only key field when a key is set",
  },
  "agentEnvironment.modelKeyPlaceholder": {
    message: "sk-ant-…",
    description: "Placeholder of the key field when none is set",
  },
  "agentEnvironment.modelKeyPlaceholderSet": {
    message: "已保存密钥；输入新密钥以替换",
    description:
      "Placeholder of the key field when a key is set (the key is never shown)",
  },
  "agentEnvironment.modelKeyHelp": {
    message: "仅可写入：密钥会被安全存储，之后不会再显示。",
    description: "Help under the key field",
  },
  "agentEnvironment.modelKeySave": {
    message: "保存密钥",
    description: "Button that saves the model key",
  },
  "agentEnvironment.modelKeyClear": {
    message: "移除密钥",
    description: "Button that opens the remove-key confirmation",
  },
  "agentEnvironment.modelKeyClearTitle": {
    message: "移除模型提供方密钥？",
    description: "Remove-key confirmation title",
  },
  "agentEnvironment.modelKeyClearBody": {
    message:
      "智能体会话将从下一次模型请求起停止使用此密钥。密钥无法再次显示，如需恢复，需要重新粘贴。",
    description: "Remove-key confirmation body",
  },
  "agentEnvironment.modelKeyClearConfirm": {
    message: "移除密钥",
    description: "Remove-key confirmation button",
  },
  "agentEnvironment.modelKeySaved": {
    message: "模型密钥已保存",
    description: "Toast after saving the key",
  },
  "agentEnvironment.modelKeyCleared": {
    message: "模型密钥已移除",
    description: "Toast after removing the key",
  },
  "agentEnvironment.modelKeyInvalid": {
    message: "请输入不含空格的密钥（最多 4,096 个字符）。",
    description: "Client/server refusal of a malformed key",
  },
  "agentEnvironment.modelKeySaveFailed": {
    message: "无法保存模型密钥，请重试。",
    description: "Generic model key write failure",
  },
  "agentEnvironment.spendTitle": {
    message: "智能体模型支出",
    description: "Heading of the agent-model spend section (ADR091 D6)",
  },
  "agentEnvironment.spendPricing": {
    message:
      "按模型提供方成本 + 30% 计费。达到上限后，本月剩余时间内不再发起新的模型请求。",
    description: "Pricing statement under the spend heading",
  },
  "agentEnvironment.spendSummary": {
    message: "本月 {used} / {cap}",
    description: "Month-to-date spend against the effective monthly cap",
  },
  "agentEnvironment.spendSummaryCeiling": {
    message: "超出 billing credits 的部分 {used} / {cap}，本月共 {total}",
    description:
      "Spend summary and meter value when the trust tier's ceiling binds on the spend billing credits leave ({used}), which differs from month-to-date ({total})",
  },
  "agentEnvironment.spendMeterValue": {
    message: "已用 {used}，上限 {cap}",
    description: "Screen-reader value of the spend progress bar",
  },
  "agentEnvironment.spendUnavailable": {
    message: "暂时无法获取支出数据。",
    description: "Shown when the spend fields could not be read",
  },
  "agentEnvironment.spendTierNextCharge": {
    message:
      "信任等级 {tier}：智能体模型支出先用 billing credits 抵扣；credits 抵扣不了的部分达到 {amount} 时从你的卡中扣款。",
    description: "Trust tier with its mid-month charge point (ADR091 D10.1)",
  },
  "agentEnvironment.spendTierNextChargeCredit": {
    message:
      "信任等级 {tier}：本账单周期内，billing credits 先抵扣前 {credit} 的智能体模型支出；超出部分达到 {amount} 时从你的卡中扣款。",
    description:
      "Trust tier with its charge point when billing credits cover agent model spend first (ADR091 D10.2a). {credit} is the credit counted in this billing period, {amount} the charge point",
  },
  "agentEnvironment.spendTierMonthEnd": {
    message: "信任等级 {tier}：智能体模型支出在月底结算。",
    description: "Top trust tier, charged only at month end",
  },
  "agentEnvironment.spendTierNew": {
    message:
      "新工作区的智能体模型支出先用 billing credits 抵扣，超出部分达到 {amount} 时会当场扣款；扣款成功后会话继续运行，上限提高到套餐上限。",
    description:
      "Explains T0: charged at its threshold, working while the charge settles (ADR091 D10.2a)",
  },
  "agentEnvironment.spendTierLocked": {
    message:
      "由于付款失败或存在争议，此工作区受到限制；请在账单页面结清后解除限制。",
    description: "Explains a payment-locked trust tier",
  },
  "agentEnvironment.spendCapLabel": {
    message: "每月支出上限（美元）",
    description: "Label of the admin-editable cap field",
  },
  "agentEnvironment.spendCapSave": {
    message: "保存上限",
    description: "Button that saves the cap",
  },
  "agentEnvironment.spendCapReset": {
    message: "恢复为套餐默认值",
    description: "Button that returns the cap to the plan default",
  },
  "agentEnvironment.spendCapHelp": {
    message: "套餐默认值 {plan}。管理员最高可设置为 {max}。",
    description: "Help under the cap field",
  },
  "agentEnvironment.spendCapInvalid": {
    message: "请输入大于 $0 且不超过 {max} 的金额。",
    description: "Client/server refusal of a cap value",
  },
  "agentEnvironment.spendCapSaved": {
    message: "支出上限已保存",
    description: "Toast after saving the cap",
  },
  "agentEnvironment.spendCapResetDone": {
    message: "支出上限已恢复为套餐默认值",
    description: "Toast after resetting the cap",
  },
  "agentEnvironment.spendCapSaveFailed": {
    message: "无法保存支出上限，请重试。",
    description: "Generic cap write failure",
  },
  "agentEnvironment.secrets.title": {
    message: "智能体密钥",
    description: "Settings \u2192 Agent secrets card title (ADR098)",
  },
  "agentEnvironment.secrets.description": {
    message:
      "可以作为环境变量提供给会话的 API 密钥和令牌。值只能写入：任何人都无法读回，并会从对话记录中隐去。",
    description: "Agent secrets card description",
  },
  "agentEnvironment.secrets.loadFailed": {
    message: "无法加载智能体密钥。",
    description: "Agent secrets card error when the list query fails",
  },
  "agentEnvironment.secrets.empty": {
    message: "还没有智能体密钥。",
    description: "Agent secrets card empty state",
  },
  "agentEnvironment.secrets.deleteLabel": {
    message: "删除 {name}",
    description: "Accessible label of a secret's delete button",
  },
  "agentEnvironment.secrets.deleteTitle": {
    message: "删除 {name}？",
    description: "Delete-secret confirmation title",
  },
  "agentEnvironment.secrets.deleteBody": {
    message: "使用此密钥的会话在重新设置之前，下一轮将会失败。",
    description: "Delete-secret confirmation body",
  },
  "agentEnvironment.secrets.deleteConfirm": {
    message: "删除密钥",
    description: "Delete-secret confirmation button",
  },
  "agentEnvironment.secrets.nameLabel": {
    message: "密钥名称",
    description: "Agent secrets card: name field label",
  },
  "agentEnvironment.secrets.namePlaceholder": {
    message: "STRIPE_API_KEY",
    description: "Agent secrets card: name field placeholder (an env var name)",
  },
  "agentEnvironment.secrets.valueLabel": {
    message: "密钥值",
    description: "Agent secrets card: value field label",
  },
  "agentEnvironment.secrets.valuePlaceholder": {
    message: "值",
    description: "Agent secrets card: value field placeholder",
  },
  "agentEnvironment.secrets.save": {
    message: "保存",
    description: "Agent secrets card: save (create or replace) button",
  },
  "agentEnvironment.secrets.help": {
    message: "保存已有名称会替换其值。会话从下一轮开始使用更改。",
    description: "Agent secrets card help line for admins",
  },
  "agentEnvironment.secrets.readOnly": {
    message: "只有工作区管理员可以添加、更改或删除智能体密钥。",
    description: "Agent secrets card help line for non-admins",
  },
  "agentEnvironment.secrets.saved": {
    message: "已保存 {name}",
    description: "Toast after a secret is saved",
  },
  "agentEnvironment.secrets.deleted": {
    message: "已删除 {name}",
    description: "Toast after a secret is deleted",
  },
  "agentEnvironment.secrets.nameInvalid": {
    message: "请使用字母、数字和 _，并以字母或 _ 开头（最多 128 个字符）。",
    description: "Secret name validation error",
  },
  "agentEnvironment.secrets.nameReserved": {
    message: "bex 会自行设置此环境变量，因此密钥不能使用该名称。",
    description: "Secret name reserved error",
  },
  "agentEnvironment.secrets.valueRequired": {
    message: "请输入值。",
    description: "Secret value required error",
  },
  "agentEnvironment.secrets.valueInvalid": {
    message: "值最多为 32 KiB。",
    description: "Secret value too long (or contains a NUL byte)",
  },
  "agentEnvironment.secrets.limitExceeded": {
    message: "已达到智能体密钥数量上限。",
    description: "Secret count or size limit error",
  },
  "agentEnvironment.secrets.notFound": {
    message: "该密钥已不存在，可能刚被其他人删除。",
    description: "AGENT_SECRET_NOT_FOUND error (deleted concurrently)",
  },
  "agentEnvironment.secrets.unavailable": {
    message: "此部署不支持智能体密钥。",
    description: "AGENT_SECRETS_UNAVAILABLE error",
  },
  "agentEnvironment.secrets.forbidden": {
    message: "只有工作区管理员可以更改智能体密钥。",
    description: "Secret write refused for a non-admin",
  },
  "agentEnvironment.secrets.saveFailed": {
    message: "无法保存更改，请重试。",
    description: "Generic secret write failure",
  },
  "agentEnvironment.secrets.valueKeepPlaceholder": {
    message: "保留当前值",
    description:
      "Agent secrets card: value placeholder when the name is an existing secret (empty keeps its value)",
  },
  "agentEnvironment.secrets.editLabel": {
    message: "编辑 {name}",
    description:
      "Accessible label of a secret's edit button (loads it into the form to rotate or rescope)",
  },
  "agentEnvironment.secrets.scopeLabel": {
    message: "范围",
    description:
      "Agent secrets card: accessible label of the scope picker (ADR098 D9)",
  },
  "agentEnvironment.secrets.scopeWorkspace": {
    message: "所有会话",
    description:
      "Scope picker option: every session in the workspace receives the secret",
  },
  "agentEnvironment.secrets.scopeRepo": {
    message: "单个仓库",
    description:
      "Scope picker option: sessions on one repository receive the secret",
  },
  "agentEnvironment.secrets.scopeExplicit": {
    message: "仅在选中时",
    description:
      "Scope picker option: only sessions that pick the secret receive it",
  },
  "agentEnvironment.secrets.scopeBadgeRepo": {
    message: "仓库：{repo}",
    description:
      "Secret row badge: repo scope, naming the repository (owner/name)",
  },
  "agentEnvironment.secrets.repoLabel": {
    message: "仓库",
    description:
      "Agent secrets card: accessible label of the repo scope's repository field",
  },
  "agentEnvironment.secrets.repoPlaceholder": {
    message: "owner/name",
    description: "Agent secrets card: repository field placeholder",
  },
  "agentEnvironment.secrets.repoEmpty": {
    message: "没有匹配的已连接仓库。",
    description:
      "Agent secrets card: repository picker with no matching connected repository",
  },
  "agentEnvironment.secrets.repoInvalid": {
    message: "请以 owner/name 的形式输入仓库。",
    description:
      "Repo scope validation error (also AGENT_SECRET_SCOPE_INVALID)",
  },
  "agentEnvironment.repoEnvs.title": {
    message: "仓库环境",
    description: "Settings → Repository environments card title (ADR109)",
  },
  "agentEnvironment.repoEnvs.description": {
    message:
      "智能体会话如何准备每个仓库。设置步骤会在每个会话的智能体启动前，从仓库根目录按顺序运行，并带有该会话的密钥。检查是智能体在交付前运行的命令，例如 lint 和 test。",
    description:
      "Repository environments card description: what setup steps and checks are",
  },
  "agentEnvironment.repoEnvs.loadFailed": {
    message: "无法加载仓库环境。",
    description: "Repository environments card error when the list query fails",
  },
  "agentEnvironment.repoEnvs.emptyList": {
    message: "还没有仓库环境。",
    description: "Repository environments card empty state",
  },
  "agentEnvironment.repoEnvs.setupSummary": {
    message: "设置：{names}",
    description:
      "Repository row summary: the setup step names, comma-separated",
  },
  "agentEnvironment.repoEnvs.checksSummary": {
    message: "检查：{names}",
    description: "Repository row summary: the check names, comma-separated",
  },
  "agentEnvironment.repoEnvs.noSetup": {
    message: "没有设置步骤",
    description:
      "Repository row summary (and the read-only dialog) when the environment has no setup steps",
  },
  "agentEnvironment.repoEnvs.noChecks": {
    message: "没有检查",
    description:
      "Repository row summary (and the read-only dialog) when the environment has no checks",
  },
  "agentEnvironment.repoEnvs.editLabel": {
    message: "编辑 {repo}",
    description:
      "Accessible label of a repository environment's edit button (repo is owner/name)",
  },
  "agentEnvironment.repoEnvs.viewLabel": {
    message: "查看 {repo}",
    description:
      "Accessible label of a repository environment's view button, for non-admins",
  },
  "agentEnvironment.repoEnvs.deleteLabel": {
    message: "删除 {repo} 的环境",
    description: "Accessible label of a repository environment's delete button",
  },
  "agentEnvironment.repoEnvs.deleteTitle": {
    message: "删除 {repo} 的环境？",
    description: "Delete-environment confirmation title",
  },
  "agentEnvironment.repoEnvs.deleteBody": {
    message:
      "从下一轮开始，此仓库上的会话将不再运行其设置步骤，也不再知道其检查。",
    description: "Delete-environment confirmation body",
  },
  "agentEnvironment.repoEnvs.deleteConfirm": {
    message: "删除环境",
    description: "Delete-environment confirmation button",
  },
  "agentEnvironment.repoEnvs.add": {
    message: "添加环境",
    description:
      "Repository environments card: button that opens the add-environment dialog",
  },
  "agentEnvironment.repoEnvs.help": {
    message: "保存会替换该仓库的整个环境。会话从下一轮开始使用更改。",
    description: "Repository environments card help line for admins",
  },
  "agentEnvironment.repoEnvs.readOnly": {
    message: "只有工作区管理员可以添加、更改或删除仓库环境。",
    description: "Repository environments card help line for non-admins",
  },
  "agentEnvironment.repoEnvs.saved": {
    message: "已保存 {repo} 的环境",
    description: "Toast after an environment is saved",
  },
  "agentEnvironment.repoEnvs.deleted": {
    message: "已删除 {repo} 的环境",
    description: "Toast after an environment is deleted",
  },
  "agentEnvironment.repoEnvs.createTitle": {
    message: "添加仓库环境",
    description: "Add-environment dialog title",
  },
  "agentEnvironment.repoEnvs.editTitle": {
    message: "编辑 {repo}",
    description: "Edit-environment dialog title (repo is owner/name)",
  },
  "agentEnvironment.repoEnvs.dialogDescription": {
    message:
      "设置步骤会在每个会话的智能体启动前，从仓库根目录按顺序运行，并带有该会话的密钥。任何一步失败都会在智能体启动前结束本轮。检查是智能体在交付前运行的命令。",
    description: "Environment dialog description",
  },
  "agentEnvironment.repoEnvs.repoLabel": {
    message: "仓库",
    description: "Environment dialog: repository field label",
  },
  "agentEnvironment.repoEnvs.repoPlaceholder": {
    message: "owner/name",
    description: "Environment dialog: repository field placeholder",
  },
  "agentEnvironment.repoEnvs.repoEmpty": {
    message: "没有匹配的已连接仓库。",
    description:
      "Environment dialog: repository picker with no matching connected repository",
  },
  "agentEnvironment.repoEnvs.setupTitle": {
    message: "设置步骤",
    description: "Environment dialog: setup steps section heading",
  },
  "agentEnvironment.repoEnvs.setupHint": {
    message:
      "安装工具和依赖。每一步用 bash 按顺序运行；最多 10 步，每步最多 10 分钟。",
    description: "Environment dialog: setup steps section hint",
  },
  "agentEnvironment.repoEnvs.checksTitle": {
    message: "检查",
    description: "Environment dialog: checks section heading",
  },
  "agentEnvironment.repoEnvs.checksHint": {
    message:
      "例如 lint、test 和 build 等命令。智能体会在交付前从仓库根目录运行每一项并报告结果；bex 本身不会运行它们。最多 10 项。",
    description: "Environment dialog: checks section hint",
  },
  "agentEnvironment.repoEnvs.addSetup": {
    message: "添加设置步骤",
    description: "Environment dialog: add a setup step row",
  },
  "agentEnvironment.repoEnvs.addCheck": {
    message: "添加检查",
    description: "Environment dialog: add a check row",
  },
  "agentEnvironment.repoEnvs.setupNameLabel": {
    message: "设置步骤 {position} 的名称",
    description:
      "Accessible label of a setup step's name field (position is its 1-based place in the list)",
  },
  "agentEnvironment.repoEnvs.setupRunLabel": {
    message: "设置步骤 {position} 的命令",
    description: "Accessible label of a setup step's command field",
  },
  "agentEnvironment.repoEnvs.setupMoveUp": {
    message: "上移设置步骤 {position}",
    description: "Accessible label: move a setup step one place earlier",
  },
  "agentEnvironment.repoEnvs.setupMoveDown": {
    message: "下移设置步骤 {position}",
    description: "Accessible label: move a setup step one place later",
  },
  "agentEnvironment.repoEnvs.setupRemove": {
    message: "移除设置步骤 {position}",
    description: "Accessible label: remove a setup step row",
  },
  "agentEnvironment.repoEnvs.checkNameLabel": {
    message: "检查 {position} 的名称",
    description:
      "Accessible label of a check's name field (position is its 1-based place in the list)",
  },
  "agentEnvironment.repoEnvs.checkRunLabel": {
    message: "检查 {position} 的命令",
    description: "Accessible label of a check's command field",
  },
  "agentEnvironment.repoEnvs.checkMoveUp": {
    message: "上移检查 {position}",
    description: "Accessible label: move a check one place earlier",
  },
  "agentEnvironment.repoEnvs.checkMoveDown": {
    message: "下移检查 {position}",
    description: "Accessible label: move a check one place later",
  },
  "agentEnvironment.repoEnvs.checkRemove": {
    message: "移除检查 {position}",
    description: "Accessible label: remove a check row",
  },
  "agentEnvironment.repoEnvs.setupNamePlaceholder": {
    message: "deps",
    description: "Setup step name placeholder (an example step name)",
  },
  "agentEnvironment.repoEnvs.setupRunPlaceholder": {
    message: "例如 npm ci",
    description: "Setup step command placeholder (an example shell command)",
  },
  "agentEnvironment.repoEnvs.checkNamePlaceholder": {
    message: "test",
    description: "Check name placeholder (an example check name)",
  },
  "agentEnvironment.repoEnvs.checkRunPlaceholder": {
    message: "例如 npm test",
    description: "Check command placeholder (an example shell command)",
  },
  "agentEnvironment.repoEnvs.close": {
    message: "关闭",
    description: "Read-only environment dialog: close button",
  },
  "agentEnvironment.repoEnvs.cancel": {
    message: "取消",
    description: "Environment dialog: cancel button",
  },
  "agentEnvironment.repoEnvs.save": {
    message: "保存",
    description: "Environment dialog: save (create or replace) button",
  },
  "agentEnvironment.repoEnvs.nameRequired": {
    message: "请输入名称。",
    description: "Step or check name required error",
  },
  "agentEnvironment.repoEnvs.nameInvalid": {
    message:
      "名称须以字母或数字开头，只能包含字母、数字、空格、“.”、“_” 或 “-”（最多 64 个字符）。",
    description: "Step or check name format error",
  },
  "agentEnvironment.repoEnvs.nameDuplicate": {
    message: "此列表中已有其他行使用这个名称。",
    description: "Step or check name repeated within its list",
  },
  "agentEnvironment.repoEnvs.runRequired": {
    message: "请输入命令。",
    description: "Step or check command required error",
  },
  "agentEnvironment.repoEnvs.setupRunTooLong": {
    message: "设置命令最多 8 KiB。",
    description: "Setup step command too long (or contains a NUL byte)",
  },
  "agentEnvironment.repoEnvs.checkRunTooLong": {
    message: "检查命令最多 2 KiB。",
    description: "Check command too long (or contains a NUL byte)",
  },
  "agentEnvironment.repoEnvs.setupTooMany": {
    message: "一个环境最多有 10 个设置步骤。",
    description: "Too many setup steps",
  },
  "agentEnvironment.repoEnvs.checksTooMany": {
    message: "一个环境最多有 10 项检查。",
    description: "Too many checks",
  },
  "agentEnvironment.repoEnvs.environmentEmpty": {
    message: "请至少添加一个设置步骤或检查。如需全部移除，请改为删除该环境。",
    description: "Environment with neither setup steps nor checks",
  },
  "agentEnvironment.repoEnvs.repoInvalid": {
    message: "请以 owner/name 的形式输入仓库。",
    description:
      "Repository validation error (also AGENT_REPO_ENVIRONMENT_INVALID on the repository)",
  },
  "agentEnvironment.repoEnvs.repoExists": {
    message: "此仓库已有环境。请改为在列表中编辑它。",
    description: "Adding an environment for a repository that already has one",
  },
  "agentEnvironment.repoEnvs.setupRefused": {
    message:
      "有设置步骤被拒绝。请检查每个名称是否有效且不重复，每条命令是否不超过 8 KiB。",
    description: "AGENT_REPO_ENVIRONMENT_INVALID on the setup steps",
  },
  "agentEnvironment.repoEnvs.checksRefused": {
    message:
      "有检查被拒绝。请检查每个名称是否有效且不重复，每条命令是否不超过 2 KiB。",
    description: "AGENT_REPO_ENVIRONMENT_INVALID on the checks",
  },
  "agentEnvironment.repoEnvs.invalid": {
    message: "此环境被拒绝。请检查其仓库、设置步骤和检查。",
    description: "AGENT_REPO_ENVIRONMENT_INVALID without a recognized field",
  },
  "agentEnvironment.repoEnvs.limitExceeded": {
    message: "此工作区已达到 50 个仓库环境的上限。请先删除一个再添加。",
    description:
      "AGENT_REPO_ENVIRONMENT_LIMIT_EXCEEDED (also checked before saving)",
  },
  "agentEnvironment.repoEnvs.notFound": {
    message: "此仓库已没有环境；可能有人刚刚删除了它。",
    description:
      "AGENT_REPO_ENVIRONMENT_NOT_FOUND error (deleted concurrently)",
  },
  "agentEnvironment.repoEnvs.unavailable": {
    message: "此部署不支持仓库环境。",
    description: "AGENT_REPO_ENVIRONMENTS_UNAVAILABLE error",
  },
  "agentEnvironment.repoEnvs.forbidden": {
    message: "只有工作区管理员可以更改仓库环境。",
    description: "Environment write refused for a non-admin",
  },
  "agentEnvironment.repoEnvs.saveFailed": {
    message: "无法保存更改，请重试。",
    description: "Generic environment write failure",
  },
  "agentEnvironment.repoEnvs.builds.status.queued": {
    message: "排队中",
    description: "Build status badge: waiting to start (ADR111 D2)",
  },
  "agentEnvironment.repoEnvs.builds.status.building": {
    message: "构建中",
    description: "Build status badge: the setup steps are running",
  },
  "agentEnvironment.repoEnvs.builds.status.success": {
    message: "就绪",
    description:
      "Build status badge: success — sessions can start from this snapshot",
  },
  "agentEnvironment.repoEnvs.builds.status.failed": {
    message: "失败",
    description: "Build status badge: the build failed",
  },
  "agentEnvironment.repoEnvs.builds.status.cancelled": {
    message: "已取消",
    description:
      "Build status badge: a newer build replaced it, or its environment was deleted",
  },
  "agentEnvironment.repoEnvs.builds.trigger.environmentSaved": {
    message: "设置步骤已更改",
    description:
      "What queued a build: the environment was saved with changed setup steps",
  },
  "agentEnvironment.repoEnvs.builds.trigger.secretChanged": {
    message: "仓库密钥已更改",
    description: "What queued a build: a repo-scoped secret was set or changed",
  },
  "agentEnvironment.repoEnvs.builds.trigger.manual": {
    message: "手动构建",
    description:
      "What queued a build: an admin clicked Build now (or used the API)",
  },
  "agentEnvironment.repoEnvs.builds.trigger.refresh": {
    message: "每日刷新",
    description: "What queued a build: the daily rebuild",
  },
  "agentEnvironment.repoEnvs.builds.failure.setupFailed": {
    message: "某个设置步骤失败了。",
    description:
      "Failed build, failureCode setup_failed (the reason below names the step and exit code)",
  },
  "agentEnvironment.repoEnvs.builds.failure.stepTimeout": {
    message: "某个设置步骤超过了 3 小时的时限。",
    description: "Failed build, failureCode step_timeout",
  },
  "agentEnvironment.repoEnvs.builds.failure.buildTimeout": {
    message: "构建超过了 3 小时的时限。",
    description: "Failed build, failureCode build_timeout",
  },
  "agentEnvironment.repoEnvs.builds.failure.checkoutFailed": {
    message: "无法检出该仓库的默认分支。",
    description: "Failed build, failureCode checkout_failed",
  },
  "agentEnvironment.repoEnvs.builds.failure.storageLimitReached": {
    message: "快照会超出此工作区的沙箱存储上限。",
    description: "Failed build, failureCode storage_limit_reached",
  },
  "agentEnvironment.repoEnvs.builds.failure.internalError": {
    message: "bex 这边未能完成构建，请重试。",
    description: "Failed build, failureCode internal_error",
  },
  "agentEnvironment.repoEnvs.builds.failure.unknown": {
    message: "构建失败。",
    description: "Failed build with no recognized failureCode",
  },
  "agentEnvironment.repoEnvs.builds.title": {
    message: "{repo} 的构建",
    description: "Build log dialog title (repo is owner/name)",
  },
  "agentEnvironment.repoEnvs.builds.openLabel": {
    message: "{repo} 的构建",
    description:
      "Accessible label of a repository row's button that opens its builds and logs",
  },
  "agentEnvironment.repoEnvs.builds.description": {
    message:
      "会话从设置步骤与当前一致的最新成功构建启动；若没有匹配的构建，则改为在每轮开始时运行设置步骤。",
    description:
      "Build log dialog description: how sessions use snapshots and the fallback (ADR111 D4)",
  },
  "agentEnvironment.repoEnvs.builds.buildNow": {
    message: "立即构建",
    description:
      "Button that queues a build of the repository's setup steps (admins only)",
  },
  "agentEnvironment.repoEnvs.builds.snapshotNote": {
    message:
      "设置步骤写入的文件会保留在快照中，因此不要把凭据写入文件。密钥以环境变量的形式提供给各步骤，不会保留在快照中。",
    description: "Note beside Build now (Devin's warning, ADR111 D6)",
  },
  "agentEnvironment.repoEnvs.builds.readOnly": {
    message: "只有工作区管理员可以启动构建。",
    description: "Shown under a disabled Build now for non-admins",
  },
  "agentEnvironment.repoEnvs.builds.forbidden": {
    message: "只有工作区管理员可以启动构建。",
    description: "Build now refused with FORBIDDEN",
  },
  "agentEnvironment.repoEnvs.builds.nothingToBuild": {
    message: "此环境没有设置步骤，因此无需构建；检查项不需要构建。",
    description:
      "Build now on a checks-only environment (also AGENT_REPO_ENVIRONMENT_NOTHING_TO_BUILD)",
  },
  "agentEnvironment.repoEnvs.builds.queued": {
    message: "已将 {repo} 的构建加入队列",
    description: "Toast after Build now succeeds",
  },
  "agentEnvironment.repoEnvs.builds.rateLimited": {
    message: "此仓库在过去 24 小时内已启动 12 次构建，请稍后再试。",
    description:
      "AGENT_REPO_ENVIRONMENT_BUILD_RATE_LIMITED without a usable nextBuildAt",
  },
  "agentEnvironment.repoEnvs.builds.rateLimitedUntil": {
    message:
      "此仓库在过去 24 小时内已启动 12 次构建，下一次构建可在 {time} 开始。",
    description:
      "AGENT_REPO_ENVIRONMENT_BUILD_RATE_LIMITED; time is nextBuildAt in the viewer's local date and time",
  },
  "agentEnvironment.repoEnvs.builds.unavailable": {
    message: "此部署不支持环境构建。",
    description: "AGENT_REPO_ENVIRONMENT_BUILDS_UNAVAILABLE",
  },
  "agentEnvironment.repoEnvs.builds.buildFailed": {
    message: "无法启动构建，请重试。",
    description: "Generic Build now failure",
  },
  "agentEnvironment.repoEnvs.builds.loadFailed": {
    message: "无法加载构建记录。",
    description: "Build history query failed",
  },
  "agentEnvironment.repoEnvs.builds.none": {
    message: "暂无构建。",
    description: "Build history list with no builds",
  },
  "agentEnvironment.repoEnvs.builds.noneLog": {
    message:
      "此仓库尚未构建。“立即构建”会运行一次设置步骤，并将结果保存为快照。",
    description: "Build log panel when the repository has no builds",
  },
  "agentEnvironment.repoEnvs.builds.historyLabel": {
    message: "构建历史",
    description: "Accessible label of the build history list",
  },
  "agentEnvironment.repoEnvs.builds.logTitle": {
    message: "构建日志",
    description: "Build log panel header when no build is selected",
  },
  "agentEnvironment.repoEnvs.builds.logFailed": {
    message: "无法加载此构建的日志。",
    description: "Single build (log) query failed",
  },
  "agentEnvironment.repoEnvs.builds.took": {
    message: "耗时 {duration}",
    description:
      "A finished build's wall time, e.g. 'took 3m 12s' (joined with ' · ')",
  },
  "agentEnvironment.repoEnvs.builds.imageRemoved": {
    message: "快照已删除",
    description:
      "A successful build whose snapshot image was since deleted (older builds keep no image)",
  },
  "agentEnvironment.repoEnvs.builds.notBuilt": {
    message: "尚未构建",
    description: "Repository row: has setup steps but no build yet",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.queued": {
    message: "此构建正在等待启动。构建与会话共享工作区的智能体沙箱上限。",
    description: "Build log of a queued build",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.building": {
    message: "设置步骤正在运行，构建完成后日志会显示在这里。",
    description: "Build log of a running build",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.cancelled": {
    message: "此构建已取消：被更新的构建取代，或其环境已被删除。",
    description: "Build log of a cancelled build",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.failed": {
    message: "没有运行任何设置步骤。",
    description:
      "Build log of a build that failed before any step ran (e.g. the checkout failed)",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.other": {
    message: "此构建没有记录任何步骤。",
    description: "Build log with no steps, any other status",
  },
  "agentEnvironment.repoEnvs.builds.stepExit": {
    message: "退出码 {code}",
    description: "A setup step's exit code badge",
  },
  "agentEnvironment.repoEnvs.builds.stepKilled": {
    message: "已中止",
    description: "A setup step with no exit code: killed or never started",
  },
  "agentEnvironment.repoEnvs.builds.noOutput": {
    message: "无输出。",
    description: "A setup step that printed nothing",
  },
  "agentEnvironment.skills.title": {
    message: "智能体技能",
    description: "Settings → Agent skills card title (ADR099)",
  },
  "agentEnvironment.skills.description": {
    message:
      "以 SKILL.md 文件形式保存的可复用指令，本工作区的每个智能体会话都会收到。智能体会在任务合适时自动使用技能，或只在被要求时使用。",
    description: "Agent skills card description",
  },
  "agentEnvironment.skills.loadFailed": {
    message: "无法加载智能体技能。",
    description: "Agent skills card error when the list query fails",
  },
  "agentEnvironment.skills.empty": {
    message: "还没有智能体技能。",
    description: "Agent skills card empty state",
  },
  "agentEnvironment.skills.onlyWhenAsked": {
    message: "仅在要求时",
    description:
      "Badge on a skill the agent applies only when a prompt invokes it",
  },
  "agentEnvironment.skills.enabledLabel": {
    message: "启用 {name}",
    description: "Accessible label of a skill's enabled switch",
  },
  "agentEnvironment.skills.editLabel": {
    message: "编辑 {name}",
    description: "Accessible label of a skill's edit button",
  },
  "agentEnvironment.skills.viewLabel": {
    message: "查看 {name}",
    description: "Accessible label of a skill's view button (non-admins)",
  },
  "agentEnvironment.skills.exportLabel": {
    message: "将 {name} 下载为 SKILL.md",
    description: "Accessible label of a skill's export button",
  },
  "agentEnvironment.skills.deleteLabel": {
    message: "删除 {name}",
    description: "Accessible label of a skill's delete button",
  },
  "agentEnvironment.skills.deleteTitle": {
    message: "删除技能 {name}？",
    description: "Delete skill confirm title",
  },
  "agentEnvironment.skills.deleteBody": {
    message: "新的智能体轮次将不再收到它。已运行的会话保留其记录。",
    description: "Delete skill confirm body",
  },
  "agentEnvironment.skills.deleteConfirm": {
    message: "删除技能",
    description: "Delete skill confirm button",
  },
  "agentEnvironment.skills.new": {
    message: "新建技能",
    description: "Button that opens the create-skill dialog",
  },
  "agentEnvironment.skills.import": {
    message: "导入 SKILL.md",
    description: "Button that imports a SKILL.md file",
  },
  "agentEnvironment.skills.help": {
    message: "成员可以使用技能；只有管理员可以添加或修改。",
    description: "Agent skills help line for admins",
  },
  "agentEnvironment.skills.readOnly": {
    message: "只有工作区管理员可以添加或修改智能体技能。",
    description: "Agent skills help line for non-admins",
  },
  "agentEnvironment.skills.createTitle": {
    message: "新建技能",
    description: "Create-skill dialog title",
  },
  "agentEnvironment.skills.editTitle": {
    message: "编辑 {name}",
    description: "Edit-skill dialog title",
  },
  "agentEnvironment.skills.viewTitle": {
    message: "{name}",
    description: "View-skill dialog title (read-only)",
  },
  "agentEnvironment.skills.dialogDescription": {
    message: "智能体根据描述判断何时适用该技能，并在适用时按指令执行。",
    description: "Skill dialog description",
  },
  "agentEnvironment.skills.nameLabel": {
    message: "名称",
    description: "Skill name field label",
  },
  "agentEnvironment.skills.namePlaceholder": {
    message: "run-integration-tests",
    description: "Skill name field placeholder",
  },
  "agentEnvironment.skills.descriptionLabel": {
    message: "描述",
    description: "Skill description field label",
  },
  "agentEnvironment.skills.descriptionPlaceholder": {
    message: "一句话说明何时使用该技能",
    description: "Skill description field placeholder",
  },
  "agentEnvironment.skills.onlyWhenAskedLabel": {
    message: "仅在我要求时使用",
    description: "Toggle: the agent applies the skill only when invoked",
  },
  "agentEnvironment.skills.onlyWhenAskedHint": {
    message: "可在输入框中选择，或在提示中写 @skills:名称 来调用。",
    description: "Hint under the only-when-asked toggle",
  },
  "agentEnvironment.skills.bodyLabel": {
    message: "指令",
    description: "Skill body field label",
  },
  "agentEnvironment.skills.bodyPlaceholder": {
    message: "智能体要执行的步骤（Markdown）。",
    description: "Skill body field placeholder",
  },
  "agentEnvironment.skills.contentLabel": {
    message: "SKILL.md",
    description: "Whole-file editor label",
  },
  "agentEnvironment.skills.editAsFile": {
    message: "以 SKILL.md 编辑",
    description: "Switch the dialog to the whole-file editor",
  },
  "agentEnvironment.skills.editAsFields": {
    message: "按字段编辑",
    description: "Switch the dialog back to separate fields",
  },
  "agentEnvironment.skills.history.tab": {
    message: "版本历史",
    description: "Skill dialog tab listing the saved versions",
  },
  "agentEnvironment.skills.history.editTab": {
    message: "编辑",
    description: "Skill dialog tab with the editing form",
  },
  "agentEnvironment.skills.history.current": {
    message: "当前",
    description: "Badge on the newest saved version",
  },
  "agentEnvironment.skills.history.changes": {
    message: "变更",
    description: "Version view: line diff against the previous version",
  },
  "agentEnvironment.skills.history.content": {
    message: "内容",
    description: "Version view: the full SKILL.md",
  },
  "agentEnvironment.skills.history.load": {
    message: "载入编辑器",
    description: "Button that fills the editor with the selected version",
  },
  "agentEnvironment.skills.history.loadMore": {
    message: "加载更多",
    description: "Button that loads the next page of versions",
  },
  "agentEnvironment.skills.history.noChanges": {
    message: "此版本没有任何变更。",
    description: "Changes view of a version identical to the previous one",
  },
  "agentEnvironment.skills.history.loadFailed": {
    message: "无法加载版本历史。",
    description: "Version history list query failed",
  },
  "agentEnvironment.skills.history.versionFailed": {
    message: "无法加载此版本。",
    description: "Single version query failed",
  },
  "agentEnvironment.skills.history.loadedNotice": {
    message: "已载入版本 {version}。保存后将作为新版本保留。",
    description: "Notice in the editor after Load into editor",
  },
  "agentEnvironment.skills.viewTab": {
    message: "预览",
    description: "Read-only skill dialog tab showing the skill",
  },
  "agentEnvironment.skills.save": {
    message: "保存技能",
    description: "Skill dialog save button",
  },
  "agentEnvironment.skills.cancel": {
    message: "取消",
    description: "Skill dialog cancel button",
  },
  "agentEnvironment.skills.close": {
    message: "关闭",
    description: "Skill dialog close button (read-only)",
  },
  "agentEnvironment.skills.saved": {
    message: "已保存 {name}",
    description: "Toast after a skill is saved",
  },
  "agentEnvironment.skills.deleted": {
    message: "已删除 {name}",
    description: "Toast after a skill is deleted",
  },
  "agentEnvironment.skills.readFailed": {
    message: "无法打开该技能，请重试。",
    description: "Error when a single skill read fails",
  },
  "agentEnvironment.skills.importFailed": {
    message: "无法读取该文件。",
    description: "Error when an imported file can't be read",
  },
  "agentEnvironment.skills.nameRequired": {
    message: "请输入名称。",
    description: "Skill name empty",
  },
  "agentEnvironment.skills.nameInvalid": {
    message: "请使用小写字母、数字和单个连字符，最多 64 个字符。",
    description: "Skill name fails the Agent Skills format",
  },
  "agentEnvironment.skills.descriptionRequired": {
    message: "请输入一行描述。",
    description: "Skill description empty",
  },
  "agentEnvironment.skills.descriptionInvalid": {
    message: "描述须为一行，最多 1,024 个字符。",
    description: "Skill description too long or multi-line",
  },
  "agentEnvironment.skills.bodyInvalid": {
    message: "指令最多 64 KiB。",
    description: "Skill body too large",
  },
  "agentEnvironment.skills.invalid": {
    message: "技能无效，请检查名称、描述和指令。",
    description: "AGENT_SKILL_INVALID error",
  },
  "agentEnvironment.skills.limitExceeded": {
    message: "已达到智能体技能上限（每个工作区 100 个，启用总量 1 MiB）。",
    description: "AGENT_SKILL_LIMIT_EXCEEDED error",
  },
  "agentEnvironment.skills.conflict": {
    message:
      "该技能在你打开后已被修改，或名称已被占用。请关闭后重新打开查看最新内容。",
    description: "AGENT_SKILL_CONFLICT error",
  },
  "agentEnvironment.skills.notFound": {
    message: "该技能已不存在。",
    description: "AGENT_SKILL_NOT_FOUND error",
  },
  "agentEnvironment.skills.unavailable": {
    message: "此部署不支持智能体技能。",
    description: "AGENT_SKILLS_UNAVAILABLE error",
  },
  "agentEnvironment.skills.forbidden": {
    message: "只有工作区管理员可以修改智能体技能。",
    description: "Skill write refused for a non-admin",
  },
  "agentEnvironment.skills.saveFailed": {
    message: "无法保存更改，请重试。",
    description: "Generic skill write failure",
  },
  "agentEnvironment.agentsTitle": {
    message: "可用的智能体",
    description: "Settings \u2192 Agents: availability section heading",
  },
  "agentEnvironment.agentsDescription": {
    message:
      "选择成员可以用来启动会话的智能体。关闭某个智能体后，正在运行的会话会继续运行。",
    description: "Settings \u2192 Agents: availability section description",
  },
  "agentEnvironment.agentsDataUseNote": {
    message: "其模型提供方可能使用会话发送的内容训练模型。",
    description:
      "Settings \u2192 Agents: note under an agent whose provider trains on session data (Muse Code)",
  },
  "agentEnvironment.agentsDataUseTitle": {
    message: "开启 {agent}？",
    description:
      "Confirm title before turning on an agent whose provider trains on session data",
  },
  "agentEnvironment.agentsDataUseBody.muse": {
    message:
      "该智能体运行在 Meta 的贡献者层级上。Meta 可能使用其会话发送的提示词、代码、文件和上下文以及返回的回复来训练其模型。开启它即表示你作为管理员代表工作区确认这一点。",
    description: "Confirm body: Muse Code's data-use acknowledgement (ADR108)",
  },
  "agentEnvironment.agentsDataUseConfirm": {
    message: "确认并开启",
    description: "Confirm button: acknowledge data use and enable the agent",
  },
  "agentEnvironment.agentsSaved": {
    message: "可用的智能体已更新",
    description: "Toast after an availability change",
  },
  "agentEnvironment.agentsSaveFailed": {
    message: "无法更新可用的智能体，请重试。",
    description: "Error after a failed availability change",
  },
  "agentEnvironment.githubCliLabel": {
    message: "GitHub CLI（gh）",
    description:
      "Label of the switch that gives the workspace's agent sessions gh, the GitHub CLI (w10/m524)",
  },
  "agentEnvironment.githubCliHelp": {
    message:
      "关闭后，此工作区的所有智能体会话（包括正在运行的会话）都将无法使用 gh；推送到会话分支的 Git 操作和 bex 的拉取请求工具仍可正常使用。",
    description:
      "Help under the GitHub CLI switch: what turning it off does (w10/m524)",
  },
  "agentEnvironment.githubCliTurnedOn": {
    message: "已为智能体会话开启 GitHub CLI",
    description:
      "Toast after an admin turns the sessions' GitHub CLI on (w10/m524)",
  },
  "agentEnvironment.githubCliTurnedOff": {
    message: "已为智能体会话关闭 GitHub CLI",
    description:
      "Toast after an admin turns the sessions' GitHub CLI off (w10/m524)",
  },
  "agentEnvironment.githubCliSaveFailed": {
    message: "无法更改 GitHub CLI 设置，请重试。",
    description: "Error after a failed GitHub CLI switch (w10/m524)",
  },
  "agentEnvironment.githubCliMandatoryLabel": {
    message: "强制保持关闭",
    description:
      "Label of the switch that makes the workspace's GitHub CLI off a floor no automation or session can turn back on (w10/m561)",
  },
  "agentEnvironment.githubCliMandatoryHelp": {
    message:
      "在 GitHub CLI 关闭期间生效：此时任何自动化或会话都无法重新开启 gh。",
    description:
      "Help under the mandatory switch: it applies only while GitHub CLI is off (w10/m561)",
  },
  "agentEnvironment.githubCliMandatoryTurnedOn": {
    message: "已强制保持 GitHub CLI 关闭",
    description:
      "Toast after an admin makes the workspace's GitHub CLI off mandatory (w10/m561)",
  },
  "agentEnvironment.githubCliMandatoryTurnedOff": {
    message: "已取消强制保持 GitHub CLI 关闭",
    description:
      "Toast after an admin makes the workspace's GitHub CLI off no longer mandatory (w10/m561)",
  },
  "agentEnvironment.githubCliMandatorySaveFailed": {
    message: "无法更改是否强制保持 GitHub CLI 关闭，请重试。",
    description: "Error after a failed mandatory switch (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsLabel": {
    message: "自动化的 GitHub CLI",
    description:
      "Label of the select that binds the gh of the sessions automations start: inherit, on or off (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsHelp": {
    message:
      "自动化启动的会话所用的 gh，除非某个自动化自行设置。“继承”跟随上方的开关。",
    description:
      "Help under the automations GitHub CLI select: an automation's own binding wins, and inherit follows the workspace switch (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsInherited": {
    message: "自动化现在继承 GitHub CLI 设置",
    description:
      "Toast after an admin returns automations to inheriting the workspace's GitHub CLI switch (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsTurnedOn": {
    message: "已为自动化开启 GitHub CLI",
    description:
      "Toast after an admin turns GitHub CLI on for the sessions automations start (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsTurnedOff": {
    message: "已为自动化关闭 GitHub CLI",
    description:
      "Toast after an admin turns GitHub CLI off for the sessions automations start (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsSaveFailed": {
    message: "无法更改自动化的 GitHub CLI，请重试。",
    description:
      "Error after a failed automations GitHub CLI choice (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsLocked": {
    message: "已强制保持 GitHub CLI 关闭，因此自动化无法开启 gh。",
    description:
      "Mapped message for AGENT_ENVIRONMENT_GITHUB_CLI_LOCKED: automations cannot be bound on under a mandatory off (w10/m561)",
  },
  "agentEnvironment.modelsTitle": {
    message: "成员可选的模型",
    description:
      "Settings → Agents: heading of the list of platform models members can choose (w10/m527)",
  },
  "agentEnvironment.modelsDescription": {
    message:
      "选择成员在会话、消息和自动化中可以选用的平台模型：全部模型，或按模型、按提供商只允许其中一部分。已在使用被你关闭的模型的会话或自动化，会从下一轮起改用默认模型。",
    description:
      "Settings → Agents: description of the model list — what turning a model off does (w10/m527), and that it allows all models (w10/m592) or filters them by model or by provider (w10/m573)",
  },
  "agentEnvironment.modelsDefaultLocked": {
    message: "平台的默认模型始终保持开启，因为会话会回退到它。",
    description:
      "Note under the platform catalog's default model, whose switch is locked on (w10/m527)",
  },
  "agentEnvironment.modelsNew": {
    message: "目录中新增",
    description:
      "Note under a platform model that is off because it joined the catalog after an admin narrowed the list; an admin turns it on with its switch (w10/m568)",
  },
  "agentEnvironment.modelsTurnedOn": {
    message: "已为成员开启 {model}",
    description:
      "Toast after an admin turns a model on for members; {model} is its id (w10/m527)",
  },
  "agentEnvironment.modelsTurnedOff": {
    message: "已为成员关闭 {model}",
    description:
      "Toast after an admin turns a model off for members; {model} is its id (w10/m527)",
  },
  "agentEnvironment.modelsInvalid": {
    message:
      "无法更新模型：平台的默认模型必须保持开启，且列表只能包含平台目录中的模型。请刷新页面查看当前目录。",
    description:
      "Server refusal AGENT_ENVIRONMENT_MODELS_INVALID of a model list change (w10/m527)",
  },
  "agentEnvironment.modelsSaveFailed": {
    message: "无法更新成员可选的模型，请重试。",
    description: "Error after a failed model switch (w10/m527)",
  },
  "agentEnvironment.modelsFilterLabel": {
    message: "筛选成员可选用的模型的方式",
    description:
      "Settings → Agents: accessible name of the choice between every platform model (w10/m592) and filtering them by model or by provider (w10/m573)",
  },
  "agentEnvironment.modelsAll": {
    message: "全部模型",
    description:
      "Settings → Agents: option that lets members choose every platform model, one added to the catalog later included — Devin's “Allow all models”, every workspace's default (w10/m592). Keep it short: the three options share one line on a 390 px phone",
  },
  "agentEnvironment.modelsByModel": {
    message: "按模型",
    description:
      "Settings → Agents: option that filters the platform models members can choose one model at a time (w10/m573)",
  },
  "agentEnvironment.modelsByProvider": {
    message: "按提供商",
    description:
      "Settings → Agents: option that filters the platform models members can choose by provider, the prefix of a model id before / (w10/m573)",
  },
  "agentEnvironment.modelsAllHelp": {
    message:
      "成员可以选用所有平台模型，包括之后加入平台的模型。若要关闭某些模型，请按模型或按提供商筛选。",
    description:
      "Settings → Agents: help under the choice while it is All models — every platform model, a later one included; filtering by model or by provider turns models off (w10/m592)",
  },
  "agentEnvironment.modelsByModelHelp": {
    message:
      "成员只能选用此处开启的模型。之后加入平台的模型会保持关闭，直到你将其开启，即使此处的模型已全部开启。",
    description:
      "Settings → Agents: help under the choice while it is by model — an allowlist: members pick only the models on, and a model added to the catalog later stays off even while every listed model is on (w10/m568, w10/m573, w10/m592)",
  },
  "agentEnvironment.modelsByProviderHelp": {
    message:
      "成员可以选用你允许的提供商的所有模型，包括之后加入平台的模型。其他提供商的模型保持关闭。",
    description:
      "Settings → Agents: help under the filter choice while it is by provider — every model of an allowed provider, a later one included, and other providers' models off (w10/m573)",
  },
  "agentEnvironment.modelsProviderDefault": {
    message: "{model}（默认模型，始终开启）",
    description:
      "Settings → Agents, by provider: the catalog's default model in the list under its provider's switch; {model} is its id (w10/m573)",
  },
  "agentEnvironment.modelsProviderLastLocked": {
    message: "至少保留一个允许的提供商。若只提供默认模型，请改为按模型筛选。",
    description:
      "Settings → Agents, by provider: note under the one provider still allowed, whose switch is locked on (w10/m573)",
  },
  "agentEnvironment.modelsProviderTurnedOn": {
    message: "已为成员开启 {provider} 的模型",
    description:
      "Toast after an admin allows a provider's models for members; {provider} is the provider id, e.g. anthropic (w10/m573)",
  },
  "agentEnvironment.modelsProviderTurnedOff": {
    message: "已为成员关闭 {provider} 的模型",
    description:
      "Toast after an admin stops allowing a provider's models for members; {provider} is the provider id (w10/m573)",
  },
  "agentEnvironment.modelsAllowedAll": {
    message: "成员现在可以从所有模型中选用，包括之后加入平台的模型",
    description:
      "Toast after an admin switches the model list to All models (w10/m592)",
  },
  "agentEnvironment.modelsFilteredByProvider": {
    message: "成员现在从你允许的提供商的模型中选用",
    description:
      "Toast after an admin switches the model list to filtering by provider (w10/m573)",
  },
  "agentEnvironment.modelsFilteredByModel": {
    message: "成员现在从你允许的模型中选用",
    description:
      "Toast after an admin switches the model list to filtering by model, from All models or from by provider (w10/m573, w10/m592)",
  },
  "agentEnvironment.modelsProvidersInvalid": {
    message:
      "无法更新提供商：列表必须至少包含一个平台目录中的提供商。请刷新页面查看当前目录。",
    description:
      "Server refusal AGENT_ENVIRONMENT_MODELS_INVALID of a provider filter change (field enabledProviders, w10/m573)",
  },
  "agentEnvironment.modelsAllowAllInvalid": {
    message:
      "无法更改成员可选的模型：此工作区已不再使用平台模型目录。请刷新页面查看当前设置。",
    description:
      "Server refusal AGENT_ENVIRONMENT_MODELS_INVALID of a switch to or from All models (field allowAllModels): the workspace no longer uses the platform model catalog (w10/m592)",
  },
  "agentEnvironment.modelOffOption": {
    message: "{model}（已关闭）",
    description:
      "Default model picker: the saved default, which an admin turned off (w10/m527)",
  },
  "agentEnvironment.modelDefaultOffNote": {
    message:
      "{model} 已关闭，因此未指定模型的会话和自动化将使用平台默认模型 {fallback}，直到管理员选择一个已开启的模型，或重新开启 {model}。",
    description:
      "Note under the default model picker when the saved default is turned off; {fallback} is the platform default that runs instead (w10/m527)",
  },
  "agentEnvironment.modelDefaultRetiredNote": {
    message:
      "{model} 已不在平台目录中，因此未指定模型的会话和自动化将使用平台默认模型 {fallback}，直到管理员选择其他模型。",
    description:
      "Note under the default model picker when the saved default left the platform catalog; {fallback} is the platform default that runs instead (w10/m527)",
  },
  "agentEnvironment.idleTimeoutTitle": {
    message: "空闲超时",
    description: "Heading of the Enterprise idle timeout section (w6/m563)",
  },
  "agentEnvironment.idleTimeoutDescription": {
    message:
      "会话的沙箱在其轮次结束或最后一个编辑器断开后保持运行的时长。向已关闭的会话发送消息会启动一个新的沙箱并继续对话。自动化的会话以及由其他会话启动的会话，其沙箱会在轮次结束后立即关闭。",
    description:
      "Description of the idle timeout section: what it bounds, and that automations' and child sessions' sandboxes close at once",
  },
  "agentEnvironment.idleTimeoutLabel": {
    message: "分钟",
    description: "Label of the idle timeout's minutes field",
  },
  "agentEnvironment.idleTimeoutSave": {
    message: "保存超时",
    description: "Button that saves the idle timeout",
  },
  "agentEnvironment.idleTimeoutHelp": {
    message: "{min} 到 {max} 分钟。未设置的工作区使用 {min} 分钟。",
    description: "Help under the idle timeout field; {min} is also the default",
  },
  "agentEnvironment.idleTimeoutInvalid": {
    message: "请输入 {min} 到 {max} 之间的整数分钟数。",
    description:
      "Client/server refusal of an idle timeout outside {min}-{max} minutes",
  },
  "agentEnvironment.idleTimeoutEnterpriseOnly": {
    message: "只有 Enterprise 工作区可以更改空闲超时。",
    description:
      "Server refusal of an idle timeout on a workspace no longer on Enterprise",
  },
  "agentEnvironment.idleTimeoutSaved": {
    message: "空闲超时已设为 {minutes} 分钟",
    description: "Toast after the idle timeout saved",
  },
  "agentEnvironment.idleTimeoutSaveFailed": {
    message: "无法保存空闲超时，请重试。",
    description: "Generic idle timeout write failure",
  },
};

export default zhAgentEnvironment;
