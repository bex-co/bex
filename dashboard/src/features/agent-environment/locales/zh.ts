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
};

export default zhAgentEnvironment;
