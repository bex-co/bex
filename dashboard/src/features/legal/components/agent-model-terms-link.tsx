import { useTranslations } from "@/common/hooks/use-translations";

/** The public agent model usage terms (ADR091 D8, published 2026-11-12). */
export const AGENT_MODEL_TERMS_PATH = "/legal/agent-models";

const LINK_CLASS = "underline underline-offset-4";

/**
 * "Terms", after an agent-model pricing statement. It opens in a new tab so a
 * half-edited settings form is not lost. A skeleton passes `placeholder` for
 * the same text in the same typography with nothing to focus or follow.
 */
function AgentModelTermsLink({
  placeholder = false,
}: {
  placeholder?: boolean;
}) {
  const { t } = useTranslations();
  if (placeholder) {
    return <span className={LINK_CLASS}>{t("legal.termsLink")}</span>;
  }
  return (
    <a
      href={AGENT_MODEL_TERMS_PATH}
      target="_blank"
      rel="noreferrer"
      className={`${LINK_CLASS} hover:text-foreground`}
    >
      {t("legal.termsLink")}
    </a>
  );
}

/**
 * An agent-model pricing statement followed by its "Terms" link (ADR091 D8),
 * shared by every ready view and its skeleton so both carry the same text and
 * wrap to the same lines; a skeleton passes `placeholder`.
 */
export function PricingWithTerms({
  pricing,
  placeholder = false,
}: {
  pricing: string;
  placeholder?: boolean;
}) {
  return (
    <>
      {pricing} <AgentModelTermsLink placeholder={placeholder} />
    </>
  );
}
