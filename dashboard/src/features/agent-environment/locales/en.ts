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
  "agentEnvironment.secrets.title": {
    message: "Agent secrets",
    description: "Settings \u2192 Agent secrets card title (ADR098)",
  },
  "agentEnvironment.secrets.description": {
    message:
      "API keys and tokens a session can be given as environment variables. Values are write-only: no one can read them back, and they are redacted from transcripts.",
    description: "Agent secrets card description",
  },
  "agentEnvironment.secrets.loadFailed": {
    message: "Couldn't load the agent secrets.",
    description: "Agent secrets card error when the list query fails",
  },
  "agentEnvironment.secrets.empty": {
    message: "No agent secrets yet.",
    description: "Agent secrets card empty state",
  },
  "agentEnvironment.secrets.deleteLabel": {
    message: "Delete {name}",
    description: "Accessible label of a secret's delete button",
  },
  "agentEnvironment.secrets.deleteTitle": {
    message: "Delete {name}?",
    description: "Delete-secret confirmation title",
  },
  "agentEnvironment.secrets.deleteBody": {
    message:
      "Sessions that use this secret fail their next turn until it is set again.",
    description: "Delete-secret confirmation body",
  },
  "agentEnvironment.secrets.deleteConfirm": {
    message: "Delete secret",
    description: "Delete-secret confirmation button",
  },
  "agentEnvironment.secrets.nameLabel": {
    message: "Secret name",
    description: "Agent secrets card: name field label",
  },
  "agentEnvironment.secrets.namePlaceholder": {
    message: "STRIPE_API_KEY",
    description: "Agent secrets card: name field placeholder (an env var name)",
  },
  "agentEnvironment.secrets.valueLabel": {
    message: "Secret value",
    description: "Agent secrets card: value field label",
  },
  "agentEnvironment.secrets.valuePlaceholder": {
    message: "Value",
    description: "Agent secrets card: value field placeholder",
  },
  "agentEnvironment.secrets.save": {
    message: "Save",
    description: "Agent secrets card: save (create or replace) button",
  },
  "agentEnvironment.secrets.help": {
    message:
      "Saving an existing name replaces its value. Sessions read the new value from their next turn.",
    description: "Agent secrets card help line for admins",
  },
  "agentEnvironment.secrets.readOnly": {
    message: "Only workspace admins can add or delete agent secrets.",
    description: "Agent secrets card help line for non-admins",
  },
  "agentEnvironment.secrets.saved": {
    message: "Saved {name}",
    description: "Toast after a secret is saved",
  },
  "agentEnvironment.secrets.deleted": {
    message: "Deleted {name}",
    description: "Toast after a secret is deleted",
  },
  "agentEnvironment.secrets.nameInvalid": {
    message:
      "Use letters, digits and _, starting with a letter or _ (at most 128).",
    description: "Secret name validation error",
  },
  "agentEnvironment.secrets.nameReserved": {
    message:
      "bex sets this environment variable itself, so a secret can't use the name.",
    description: "Secret name reserved error",
  },
  "agentEnvironment.secrets.valueRequired": {
    message: "Enter a value.",
    description: "Secret value required error",
  },
  "agentEnvironment.secrets.valueInvalid": {
    message: "A value can be at most 32 KiB.",
    description: "Secret value too long (or contains a NUL byte)",
  },
  "agentEnvironment.secrets.limitExceeded": {
    message: "This reached the limit on agent secrets.",
    description: "Secret count or size limit error",
  },
  "agentEnvironment.secrets.notFound": {
    message: "This secret no longer exists; someone may have just deleted it.",
    description: "AGENT_SECRET_NOT_FOUND error (deleted concurrently)",
  },
  "agentEnvironment.secrets.unavailable": {
    message: "Agent secrets aren't available on this deployment.",
    description: "AGENT_SECRETS_UNAVAILABLE error",
  },
  "agentEnvironment.secrets.forbidden": {
    message: "Only workspace admins can change agent secrets.",
    description: "Secret write refused for a non-admin",
  },
  "agentEnvironment.secrets.saveFailed": {
    message: "Couldn't save the change. Try again.",
    description: "Generic secret write failure",
  },
  "agentEnvironment.skills.title": {
    message: "Agent skills",
    description: "Settings → Agent skills card title (ADR099)",
  },
  "agentEnvironment.skills.description": {
    message:
      "Reusable instructions every agent session in this workspace receives, as SKILL.md files. The agent applies a skill when it fits the task, or only when asked.",
    description: "Agent skills card description",
  },
  "agentEnvironment.skills.loadFailed": {
    message: "Couldn't load the agent skills.",
    description: "Agent skills card error when the list query fails",
  },
  "agentEnvironment.skills.empty": {
    message: "No agent skills yet.",
    description: "Agent skills card empty state",
  },
  "agentEnvironment.skills.onlyWhenAsked": {
    message: "Only when asked",
    description:
      "Badge on a skill the agent applies only when a prompt invokes it",
  },
  "agentEnvironment.skills.enabledLabel": {
    message: "Enable {name}",
    description: "Accessible label of a skill's enabled switch",
  },
  "agentEnvironment.skills.editLabel": {
    message: "Edit {name}",
    description: "Accessible label of a skill's edit button",
  },
  "agentEnvironment.skills.viewLabel": {
    message: "View {name}",
    description: "Accessible label of a skill's view button (non-admins)",
  },
  "agentEnvironment.skills.exportLabel": {
    message: "Download {name} as SKILL.md",
    description: "Accessible label of a skill's export button",
  },
  "agentEnvironment.skills.deleteLabel": {
    message: "Delete {name}",
    description: "Accessible label of a skill's delete button",
  },
  "agentEnvironment.skills.deleteTitle": {
    message: "Delete the skill {name}?",
    description: "Delete skill confirm title",
  },
  "agentEnvironment.skills.deleteBody": {
    message:
      "New agent turns stop receiving it. Sessions that already ran keep their transcript.",
    description: "Delete skill confirm body",
  },
  "agentEnvironment.skills.deleteConfirm": {
    message: "Delete skill",
    description: "Delete skill confirm button",
  },
  "agentEnvironment.skills.new": {
    message: "New skill",
    description: "Button that opens the create-skill dialog",
  },
  "agentEnvironment.skills.import": {
    message: "Import SKILL.md",
    description: "Button that imports a SKILL.md file",
  },
  "agentEnvironment.skills.help": {
    message: "Members can use skills; only admins can add or change them.",
    description: "Agent skills help line for admins",
  },
  "agentEnvironment.skills.readOnly": {
    message: "Only workspace admins can add or change agent skills.",
    description: "Agent skills help line for non-admins",
  },
  "agentEnvironment.skills.createTitle": {
    message: "New skill",
    description: "Create-skill dialog title",
  },
  "agentEnvironment.skills.editTitle": {
    message: "Edit {name}",
    description: "Edit-skill dialog title",
  },
  "agentEnvironment.skills.viewTitle": {
    message: "{name}",
    description: "View-skill dialog title (read-only)",
  },
  "agentEnvironment.skills.dialogDescription": {
    message:
      "The agent reads the description to decide when the skill applies, and follows the instructions when it does.",
    description: "Skill dialog description",
  },
  "agentEnvironment.skills.nameLabel": {
    message: "Name",
    description: "Skill name field label",
  },
  "agentEnvironment.skills.namePlaceholder": {
    message: "run-integration-tests",
    description: "Skill name field placeholder",
  },
  "agentEnvironment.skills.descriptionLabel": {
    message: "Description",
    description: "Skill description field label",
  },
  "agentEnvironment.skills.descriptionPlaceholder": {
    message: "When to use this skill, in one line",
    description: "Skill description field placeholder",
  },
  "agentEnvironment.skills.onlyWhenAskedLabel": {
    message: "Only when I ask",
    description: "Toggle: the agent applies the skill only when invoked",
  },
  "agentEnvironment.skills.onlyWhenAskedHint": {
    message: "Invoke it from the composer or with @skills:name in a prompt.",
    description: "Hint under the only-when-asked toggle",
  },
  "agentEnvironment.skills.bodyLabel": {
    message: "Instructions",
    description: "Skill body field label",
  },
  "agentEnvironment.skills.bodyPlaceholder": {
    message: "Steps the agent follows, in Markdown.",
    description: "Skill body field placeholder",
  },
  "agentEnvironment.skills.contentLabel": {
    message: "SKILL.md",
    description: "Whole-file editor label",
  },
  "agentEnvironment.skills.editAsFile": {
    message: "Edit as SKILL.md",
    description: "Switch the dialog to the whole-file editor",
  },
  "agentEnvironment.skills.editAsFields": {
    message: "Edit fields",
    description: "Switch the dialog back to separate fields",
  },
  "agentEnvironment.skills.save": {
    message: "Save skill",
    description: "Skill dialog save button",
  },
  "agentEnvironment.skills.cancel": {
    message: "Cancel",
    description: "Skill dialog cancel button",
  },
  "agentEnvironment.skills.close": {
    message: "Close",
    description: "Skill dialog close button (read-only)",
  },
  "agentEnvironment.skills.saved": {
    message: "Saved {name}",
    description: "Toast after a skill is saved",
  },
  "agentEnvironment.skills.deleted": {
    message: "Deleted {name}",
    description: "Toast after a skill is deleted",
  },
  "agentEnvironment.skills.readFailed": {
    message: "Couldn't open the skill. Try again.",
    description: "Error when a single skill read fails",
  },
  "agentEnvironment.skills.importFailed": {
    message: "Couldn't read that file.",
    description: "Error when an imported file can't be read",
  },
  "agentEnvironment.skills.nameRequired": {
    message: "Enter a name.",
    description: "Skill name empty",
  },
  "agentEnvironment.skills.nameInvalid": {
    message:
      "Use lowercase letters, digits and single hyphens, up to 64 characters.",
    description: "Skill name fails the Agent Skills format",
  },
  "agentEnvironment.skills.descriptionRequired": {
    message: "Enter a one-line description.",
    description: "Skill description empty",
  },
  "agentEnvironment.skills.descriptionInvalid": {
    message: "Keep the description to one line of at most 1,024 characters.",
    description: "Skill description too long or multi-line",
  },
  "agentEnvironment.skills.bodyInvalid": {
    message: "Instructions are limited to 64 KiB.",
    description: "Skill body too large",
  },
  "agentEnvironment.skills.invalid": {
    message:
      "The skill isn't valid. Check its name, description and instructions.",
    description: "AGENT_SKILL_INVALID error",
  },
  "agentEnvironment.skills.limitExceeded": {
    message:
      "This reached a limit on agent skills (100 per workspace, 1 MiB enabled in total).",
    description: "AGENT_SKILL_LIMIT_EXCEEDED error",
  },
  "agentEnvironment.skills.conflict": {
    message:
      "This skill changed since you opened it, or the name is taken. Close and reopen it to see the latest.",
    description: "AGENT_SKILL_CONFLICT error",
  },
  "agentEnvironment.skills.notFound": {
    message: "That skill no longer exists.",
    description: "AGENT_SKILL_NOT_FOUND error",
  },
  "agentEnvironment.skills.unavailable": {
    message: "Agent skills aren't available on this deployment.",
    description: "AGENT_SKILLS_UNAVAILABLE error",
  },
  "agentEnvironment.skills.forbidden": {
    message: "Only workspace admins can change agent skills.",
    description: "Skill write refused for a non-admin",
  },
  "agentEnvironment.skills.saveFailed": {
    message: "Couldn't save the change. Try again.",
    description: "Generic skill write failure",
  },
};

export default enAgentEnvironment;
