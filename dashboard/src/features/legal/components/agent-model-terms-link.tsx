import { useTranslations } from "@/common/hooks/use-translations";
import { agentModelTermsUrl } from "@/features/legal/lib/agent-model-terms-url";

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
  const { t, i18n } = useTranslations();
  if (placeholder) {
    return <span className={LINK_CLASS}>{t("legal.termsLink")}</span>;
  }
  return (
    <a
      href={agentModelTermsUrl(i18n.language)}
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
