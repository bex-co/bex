import type { TranslationEntry } from "@/i18n";

// The agent model usage terms (ADR091 D8 + D10.7), published 2026-11-12 at
// /legal/agent-models. The facts here (prices, caps, thresholds, retention)
// mirror ADR091; change both together. The figures are checked against
// lego/backend/internal/pricing/pricing.yaml by __tests__/pricing-figures.test.ts.
const enLegal: Record<string, TranslationEntry> = {
  "legal.termsLink": {
    message: "Terms",
    description:
      "Link to the agent model usage terms, after a pricing statement",
  },
  "legal.agentModels.title": {
    message: "Agent model usage terms",
    description: "Agent model terms page heading and browser title",
  },
  "legal.agentModels.effective": {
    message: "Effective {date}",
    description: "Effective date of the current terms version",
  },
  "legal.agentModels.scopeTitle": {
    message: "Scope",
    description: "Terms section heading",
  },
  "legal.agentModels.scopeBody": {
    message:
      "These terms apply when a workspace's agent sessions use the bex-provided model rather than the workspace's own model provider key.",
    description: "Terms: scope",
  },
  "legal.agentModels.dataTitle": {
    message: "Your data",
    description: "Terms section heading",
  },
  "legal.agentModels.dataSend": {
    message:
      "To generate responses, bex sends the session's prompts, code and tool output to third-party model providers through OpenRouter.",
    description: "Terms: what is sent and to whom",
  },
  "legal.agentModels.dataRouting": {
    message:
      "Requests are routed only to providers that do not store prompts beyond processing them and do not train on them: bex uses OpenRouter provider routing with data collection denied.",
    description: "Terms: the routing policy",
  },
  "legal.agentModels.dataMuse": {
    message:
      "Muse Code sessions are the exception. bex sends them directly to Meta's model API on its contributor tier, under which Meta may use the prompts, code, files and context sent, and the responses returned, to train its models. Muse Code stays off in a workspace until an admin turns it on and acknowledges this.",
    description:
      "Terms: the Muse Code exception to the routing policy (ADR108)",
  },
  "legal.agentModels.dataOwnKey": {
    message:
      "If the workspace uses its own provider key instead, requests go to your provider under your agreement with them, and bex bills no model tokens.",
    description: "Terms: the bring-your-own-key alternative",
  },
  "legal.agentModels.priceTitle": {
    message: "Price",
    description: "Terms section heading",
  },
  "legal.agentModels.priceBody": {
    message:
      "Each request is billed at the model provider's actual cost plus 30%. Usage is metered per request and shown under Billing → Agent model.",
    description: "Terms: price",
  },
  "legal.agentModels.capsTitle": {
    message: "Spend caps",
    description: "Terms section heading",
  },
  "legal.agentModels.capsPlan": {
    message:
      "Each workspace has a monthly spend cap set by its plan: Hobby $20, Pro $200, Scale $2,000. Workspace admins can adjust it up to 10 times the plan's cap.",
    description: "Terms: plan caps",
  },
  "legal.agentModels.capsNew": {
    message:
      "A new workspace can run two concurrent sessions on the bex-provided model until its first successful payment.",
    description: "Terms: new-workspace limits",
  },
  "legal.agentModels.capsReached": {
    message:
      "When the cap is reached, new model requests are refused until the next month begins or an admin raises the cap.",
    description: "Terms: what happens at the cap",
  },
  "legal.agentModels.chargesTitle": {
    message: "Charges during the month",
    description: "Terms section heading",
  },
  "legal.agentModels.chargesThreshold": {
    message:
      "You authorize bex to charge the workspace's payment method whenever its unbilled agent model usage reaches the workspace's charge threshold, in addition to the monthly invoice. The workspace keeps working while the charge is made. The threshold starts at $10 and rises with the workspace's payment history.",
    description: "Terms: threshold charge authorization",
  },
  "legal.agentModels.chargesFailed": {
    message:
      "If a charge fails or needs your authentication, agent model usage is paused until the balance is paid.",
    description: "Terms: failed charge",
  },
  "legal.agentModels.deletionTitle": {
    message: "Deleting a workspace or account",
    description: "Terms section heading",
  },
  "legal.agentModels.deletionSettle": {
    message:
      "Before a workspace or account is deleted, bex charges its unbilled usage. If that charge fails, deletion waits until the balance is paid.",
    description: "Terms: settle before delete",
  },
  "legal.agentModels.deletionAuthorize": {
    message:
      "You authorize bex to charge for usage incurred before deletion, including after the deletion.",
    description: "Terms: charges after deletion",
  },
  "legal.agentModels.paymentTitle": {
    message: "Payment methods",
    description: "Terms section heading",
  },
  "legal.agentModels.paymentBody": {
    message:
      "Prepaid cards aren't accepted. A card linked to an unpaid bex balance may be refused.",
    description: "Terms: card restrictions",
  },
  "legal.agentModels.recordsTitle": {
    message: "Usage records",
    description: "Terms section heading",
  },
  "legal.agentModels.recordsBody": {
    message:
      "bex keeps a record of every request (time, model, tokens and price) for 13 months. These records may be used as evidence if a charge is disputed.",
    description: "Terms: usage record retention",
  },
  "legal.agentModels.changesTitle": {
    message: "Changes to these terms",
    description: "Terms section heading",
  },
  "legal.agentModels.changesBody": {
    message:
      "bex may update these terms. The effective date at the top of this page shows the current version.",
    description: "Terms: changes",
  },
};

export default enLegal;
