import type { TranslationEntry } from "@/i18n";

// The agent model terms are a section of the bex.co Terms of Service
// (eden-cms-v2 pages/terms-of-service.md, ADR091 D8); the dashboard carries
// only the link to them.
const enLegal: Record<string, TranslationEntry> = {
  "legal.termsLink": {
    message: "Terms",
    description:
      "Link to the agent model usage terms, after a pricing statement",
  },
};

export default enLegal;
