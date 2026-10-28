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
    description: "Network tier option: any host (not yet available)",
  },
  "agentEnvironment.tierFullDescription": {
    message:
      "暂不可用：沙箱网络策略目前还无法放行任意主机。请使用“自定义”添加所需的主机。",
    description:
      "Description of the disabled full-internet tier; explains it is not enforced yet",
  },
  "agentEnvironment.comingSoon": {
    message: "即将推出",
    description: "Badge on the disabled full-internet tier",
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
};

export default zhAgentEnvironment;
