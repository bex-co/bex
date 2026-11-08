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
  "agentEnvironment.modelPinnedHelp": {
    message:
      "此工作区的智能体运行在平台模型上，所有会话都使用 {model}，无法在此更改。",
    description: "Help under the read-only pinned model",
  },
  "agentEnvironment.modelInvalid": {
    message: "请使用不含空格的模型 ID（最多 255 个字符）。",
    description: "Default model validation error",
  },
  "agentEnvironment.modelPinnedError": {
    message: "此工作区的智能体运行在平台模型（{model}）上，无法更改默认模型。",
    description: "Server refusal: model is pinned",
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
  "agentEnvironment.spendMeterValue": {
    message: "已用 {used}，上限 {cap}",
    description: "Screen-reader value of the spend progress bar",
  },
  "agentEnvironment.spendUnavailable": {
    message: "暂时无法获取支出数据。",
    description: "Shown when the spend fields could not be read",
  },
  "agentEnvironment.spendTierNextCharge": {
    message: "信任等级 {tier}：智能体模型支出达到 {amount} 时从你的卡中扣款。",
    description: "Trust tier with its mid-month charge point (ADR091 D10.1)",
  },
  "agentEnvironment.spendTierMonthEnd": {
    message: "信任等级 {tier}：智能体模型支出在月底结算。",
    description: "Top trust tier, charged only at month end",
  },
  "agentEnvironment.spendTierNew": {
    message: "新工作区的上限为 $10，首次成功付款后会提高。",
    description: "Explains the T0 new-workspace limit",
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
    message: "保存已有名称会替换其值。会话从下一轮开始读取新值。",
    description: "Agent secrets card help line for admins",
  },
  "agentEnvironment.secrets.readOnly": {
    message: "只有工作区管理员可以添加或删除智能体密钥。",
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
};

export default zhAgentEnvironment;
