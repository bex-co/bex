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
  "agentEnvironment.modelKeyTitle": {
    message: "Model provider",
    description: "Heading of the model source / provider key section",
  },
  "agentEnvironment.modelKeyDescription": {
    message:
      "Agent sessions use the bex-provided model unless the workspace adds its own provider key. With your own key, sessions call your provider directly and bex bills only sandbox compute.",
    description: "Explains the model source choice (ADR091 D7)",
  },
  "agentEnvironment.modelSourceLabel": {
    message: "Model source:",
    description: "Label before the current model source",
  },
  "agentEnvironment.modelSourcePlatform": {
    message: "bex-provided (billed at the model provider's cost + 30%)",
    description: "Model source: the platform model",
  },
  "agentEnvironment.modelSourceByo": {
    message: "Your own key (no token charges from bex)",
    description: "Model source: the workspace's own key",
  },
  "agentEnvironment.modelSourceNone": {
    message: "None: add a provider key to start agent sessions",
    description: "Model source: no key available",
  },
  "agentEnvironment.modelKeyLabel": {
    message: "Provider API key",
    description: "Label of the write-only key field when none is set",
  },
  "agentEnvironment.modelKeyReplaceLabel": {
    message: "Replace provider API key",
    description: "Label of the write-only key field when a key is set",
  },
  "agentEnvironment.modelKeyPlaceholder": {
    message: "sk-ant-…",
    description: "Placeholder of the key field when none is set",
  },
  "agentEnvironment.modelKeyPlaceholderSet": {
    message: "A key is saved; enter a new one to replace it",
    description:
      "Placeholder of the key field when a key is set (the key is never shown)",
  },
  "agentEnvironment.modelKeyHelp": {
    message: "Write-only: the key is stored securely and never shown again.",
    description: "Help under the key field",
  },
  "agentEnvironment.modelKeySave": {
    message: "Save key",
    description: "Button that saves the model key",
  },
  "agentEnvironment.modelKeyClear": {
    message: "Remove key",
    description: "Button that opens the remove-key confirmation",
  },
  "agentEnvironment.modelKeyClearTitle": {
    message: "Remove the model provider key?",
    description: "Remove-key confirmation title",
  },
  "agentEnvironment.modelKeyClearBody": {
    message:
      "Agent sessions stop using this key from their next model request. The key can't be shown again, so you would need to paste it to restore it.",
    description: "Remove-key confirmation body",
  },
  "agentEnvironment.modelKeyClearConfirm": {
    message: "Remove key",
    description: "Remove-key confirmation button",
  },
  "agentEnvironment.modelKeySaved": {
    message: "Model key saved",
    description: "Toast after saving the key",
  },
  "agentEnvironment.modelKeyCleared": {
    message: "Model key removed",
    description: "Toast after removing the key",
  },
  "agentEnvironment.modelKeyInvalid": {
    message: "Enter the key without spaces (at most 4,096 characters).",
    description: "Client/server refusal of a malformed key",
  },
  "agentEnvironment.modelKeySaveFailed": {
    message: "Couldn't save the model key. Try again.",
    description: "Generic model key write failure",
  },
  "agentEnvironment.spendTitle": {
    message: "Agent model spend",
    description: "Heading of the agent-model spend section (ADR091 D6)",
  },
  "agentEnvironment.spendPricing": {
    message:
      "Billed at the model provider's cost + 30%. The cap stops new model requests for the rest of the month.",
    description: "Pricing statement under the spend heading",
  },
  "agentEnvironment.spendSummary": {
    message: "{used} of {cap} this month",
    description: "Month-to-date spend against the effective monthly cap",
  },
  "agentEnvironment.spendMeterValue": {
    message: "{used} of {cap}",
    description: "Screen-reader value of the spend progress bar",
  },
  "agentEnvironment.spendUnavailable": {
    message: "Spend isn't available right now.",
    description: "Shown when the spend fields could not be read",
  },
  "agentEnvironment.spendTierNextCharge": {
    message:
      "Trust tier {tier}: your card is charged when agent model spend reaches {amount}.",
    description: "Trust tier with its mid-month charge point (ADR091 D10.1)",
  },
  "agentEnvironment.spendTierMonthEnd": {
    message: "Trust tier {tier}: agent model spend is charged at month end.",
    description: "Top trust tier, charged only at month end",
  },
  "agentEnvironment.spendTierNew": {
    message:
      "New workspaces start with a $10 limit that rises after the first successful payment.",
    description: "Explains the T0 new-workspace limit",
  },
  "agentEnvironment.spendTierLocked": {
    message:
      "A failed or disputed payment limits this workspace; settle it in Billing to lift the limit.",
    description: "Explains a payment-locked trust tier",
  },
  "agentEnvironment.spendCapLabel": {
    message: "Monthly spend cap (USD)",
    description: "Label of the admin-editable cap field",
  },
  "agentEnvironment.spendCapSave": {
    message: "Save cap",
    description: "Button that saves the cap",
  },
  "agentEnvironment.spendCapReset": {
    message: "Reset to plan default",
    description: "Button that returns the cap to the plan default",
  },
  "agentEnvironment.spendCapHelp": {
    message: "Plan default {plan}. Admins can set up to {max}.",
    description: "Help under the cap field",
  },
  "agentEnvironment.spendCapInvalid": {
    message: "Enter an amount more than $0 and at most {max}.",
    description: "Client/server refusal of a cap value",
  },
  "agentEnvironment.spendCapSaved": {
    message: "Spend cap saved",
    description: "Toast after saving the cap",
  },
  "agentEnvironment.spendCapResetDone": {
    message: "Spend cap reset to the plan default",
    description: "Toast after resetting the cap",
  },
  "agentEnvironment.spendCapSaveFailed": {
    message: "Couldn't save the spend cap. Try again.",
    description: "Generic cap write failure",
  },
};

export default enAgentEnvironment;
