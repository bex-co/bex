import type { TranslationEntry } from "@/i18n";

const enAgentEnvironment: Record<string, TranslationEntry> = {
  "agentEnvironment.title": {
    message: "Agents",
    description: "Settings → Agents card title and section-nav label",
  },
  "agentEnvironment.description": {
    message:
      "How agent sessions in this workspace reach the network and which model they use. Applies to every new session.",
    description: "Agents settings card description",
  },
  "agentEnvironment.loadFailed": {
    message: "Couldn't load the agent settings.",
    description: "Agents card error when the environment query fails",
  },
  "agentEnvironment.retry": {
    message: "Retry",
    description: "Retry button on the Agents card load error",
  },
  "agentEnvironment.readOnlyNote": {
    message: "Only workspace admins can change agent settings.",
    description: "Read-only notice shown to non-admin members",
  },
  "agentEnvironment.networkTitle": {
    message: "Network access",
    description: "Heading (and radiogroup label) of the network tier picker",
  },
  "agentEnvironment.networkDescription": {
    message:
      "GitHub and the model provider are always reachable. Choose what else the agent's sandbox can reach.",
    description: "Network tier picker description",
  },
  "agentEnvironment.tierRegistriesTitle": {
    message: "Package registries (recommended)",
    description: "Network tier option: built-in package registries",
  },
  "agentEnvironment.tierRegistriesDescription": {
    message:
      "The agent can install dependencies from npm, PyPI, Go, Maven, crates, RubyGems and Docker Hub, so it can build and test your code.",
    description: "Description of the registries network tier",
  },
  "agentEnvironment.tierCustomTitle": {
    message: "Custom",
    description: "Network tier option: registries plus listed hosts",
  },
  "agentEnvironment.tierCustomDescription": {
    message: "Package registries plus the hostnames you list below.",
    description: "Description of the custom network tier",
  },
  "agentEnvironment.tierFullTitle": {
    message: "Full internet",
    description: "Network tier option: any public host over HTTP(S)",
  },
  "agentEnvironment.tierFullDescription": {
    message:
      "Any public website over HTTP(S) through the bex egress proxy. Direct calls to model-provider APIs stay blocked (the agent's own model still works), and SSH and other non-HTTP protocols aren't available.",
    description:
      "Description of the full-internet tier: HTTP(S) via the egress proxy, direct model-provider API calls blocked, no SSH/non-HTTP",
  },
  "agentEnvironment.registryHostsSummary_one": {
    message: "Included package registries ({count} host)",
    description: "Disclosure summary listing the built-in registry hosts (one)",
  },
  "agentEnvironment.registryHostsSummary_other": {
    message: "Included package registries ({count} hosts)",
    description:
      "Disclosure summary listing the built-in registry hosts (many)",
  },
  "agentEnvironment.hostsLabel": {
    message: "Additional hostnames",
    description: "Label of the custom-tier hostname textarea",
  },
  "agentEnvironment.hostsHelp": {
    message:
      "One hostname per line, up to {max}. Exact names only: no scheme, port, path or wildcard.",
    description: "Help under the custom hostname textarea",
  },
  "agentEnvironment.hostsError.too_many_entries": {
    message: "List at most {max} hostnames.",
    description: "Custom hosts error: too many entries",
  },
  "agentEnvironment.hostsError.empty": {
    message: "Hostnames can't be empty.",
    description: "Custom hosts error: empty entry",
  },
  "agentEnvironment.hostsError.whitespace": {
    message: '"{entry}" has surrounding spaces.',
    description: "Custom hosts error: surrounding whitespace",
  },
  "agentEnvironment.hostsError.not_a_hostname": {
    message:
      '"{entry}" isn\'t an exact hostname. Use a name like api.example.com, without a scheme, port, path or wildcard.',
    description: "Custom hosts error: not a hostname",
  },
  "agentEnvironment.hostsError.ip_literal": {
    message: '"{entry}" is an IP address. Use a hostname instead.',
    description: "Custom hosts error: IP literal",
  },
  "agentEnvironment.hostsError.private_name": {
    message:
      '"{entry}" is a private or cluster-local name and can\'t be allowed.',
    description: "Custom hosts error: private name",
  },
  "agentEnvironment.hostsError.already_baseline": {
    message:
      '"{entry}" is already reachable (or reachable only through the model proxy). Remove it from the list.',
    description: "Custom hosts error: already in the baseline",
  },
  "agentEnvironment.hostsError.duplicate": {
    message: '"{entry}" is listed more than once.',
    description: "Custom hosts error: duplicate",
  },
  "agentEnvironment.hostsError.invalid": {
    message: "The hostname list is invalid.",
    description: "Custom hosts error: unknown reason",
  },
  "agentEnvironment.modelLabel": {
    message: "Default model",
    description: "Label of the default model field",
  },
  "agentEnvironment.modelPlaceholder": {
    message: "Provider default",
    description: "Placeholder of the empty default model field",
  },
  "agentEnvironment.modelHelp": {
    message:
      "Model id new sessions use, such as claude-sonnet-4-5. Leave empty for the provider default.",
    description: "Help under the default model field",
  },
  "agentEnvironment.modelPinnedHelp": {
    message:
      "This workspace runs agents on the platform model, so every session uses {model}. It can't be changed here.",
    description: "Help under the read-only pinned model",
  },
  "agentEnvironment.modelInvalid": {
    message: "Use a model id without spaces (at most 255 characters).",
    description: "Default model validation error",
  },
  "agentEnvironment.modelPinnedError": {
    message:
      "This workspace runs agents on the platform model ({model}); its default model can't be changed.",
    description: "Server refusal: model is pinned",
  },
  "agentEnvironment.tierInvalid": {
    message: "That network access option isn't supported.",
    description: "Server refusal: invalid tier",
  },
  "agentEnvironment.forbidden": {
    message: "Only workspace admins can change agent settings.",
    description: "Server refusal: caller is not an admin",
  },
  "agentEnvironment.saveFailed": {
    message: "Couldn't save the agent settings. Try again.",
    description: "Generic save failure",
  },
  "agentEnvironment.saveFailedReason": {
    message: "Couldn't save the agent settings: {reason}",
    description: "Save failure with the server's reason",
  },
  "agentEnvironment.save": {
    message: "Save",
    description: "Agents card save button",
  },
  "agentEnvironment.saved": {
    message: "Agent settings saved",
    description: "Success toast after saving",
  },
};

export default enAgentEnvironment;
