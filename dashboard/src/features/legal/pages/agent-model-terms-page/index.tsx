import type { ReactNode } from "react";
import { useTranslations } from "@/common/hooks/use-translations";
import { LanguageSwitcher } from "@/features/i18n/language-switcher";

/** The version in force; bump it (and ADR091 D8) with any wording change. */
const EFFECTIVE_DATE = Date.UTC(2026, 10, 12);

const SECTIONS = [
  ["legal.agentModels.scopeTitle", ["legal.agentModels.scopeBody"]],
  [
    "legal.agentModels.dataTitle",
    [
      "legal.agentModels.dataSend",
      "legal.agentModels.dataRouting",
      "legal.agentModels.dataOwnKey",
    ],
  ],
  ["legal.agentModels.priceTitle", ["legal.agentModels.priceBody"]],
  [
    "legal.agentModels.capsTitle",
    [
      "legal.agentModels.capsPlan",
      "legal.agentModels.capsNew",
      "legal.agentModels.capsReached",
    ],
  ],
  [
    "legal.agentModels.chargesTitle",
    ["legal.agentModels.chargesThreshold", "legal.agentModels.chargesFailed"],
  ],
  [
    "legal.agentModels.deletionTitle",
    ["legal.agentModels.deletionSettle", "legal.agentModels.deletionAuthorize"],
  ],
  ["legal.agentModels.paymentTitle", ["legal.agentModels.paymentBody"]],
  ["legal.agentModels.recordsTitle", ["legal.agentModels.recordsBody"]],
  ["legal.agentModels.changesTitle", ["legal.agentModels.changesBody"]],
] as const;

/**
 * The page frame, shared with its route skeleton so pending and ready keep one
 * geometry: the language action top right, then one centered reading column.
 */
export function LegalPageFrame({
  languageAction,
  children,
}: {
  languageAction: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="relative min-h-screen bg-background">
      <div className="absolute right-4 top-4 z-10">{languageAction}</div>
      <div className="mx-auto w-full max-w-3xl space-y-10 px-6 py-16 lg:px-8">
        {children}
      </div>
    </div>
  );
}

export function AgentModelTermsHeader() {
  const { t, i18n } = useTranslations();
  const date = new Intl.DateTimeFormat(i18n.language, {
    dateStyle: "long",
    timeZone: "UTC",
  }).format(EFFECTIVE_DATE);
  return (
    <header className="space-y-2">
      <h1 className="text-3xl font-bold text-foreground">
        {t("legal.agentModels.title")}
      </h1>
      <p className="text-muted-foreground">
        {t("legal.agentModels.effective", { date })}
      </p>
    </header>
  );
}

export function AgentModelTermsSections() {
  const { t } = useTranslations();
  return (
    <div className="space-y-8">
      {SECTIONS.map(([title, paragraphs]) => (
        <section key={title} className="space-y-3">
          <h2 className="text-lg font-semibold text-foreground">{t(title)}</h2>
          {paragraphs.map((key) => (
            <p key={key} className="leading-7 text-muted-foreground">
              {t(key)}
            </p>
          ))}
        </section>
      ))}
    </div>
  );
}

/**
 * `/legal/agent-models` (ADR091 D8 + D10.7): the public terms for the
 * bex-provided agent model, readable without signing in. bex-native — Render
 * has no agent model, so there is no render.com page to mirror.
 */
export default function AgentModelTermsPage() {
  return (
    <LegalPageFrame languageAction={<LanguageSwitcher />}>
      <AgentModelTermsHeader />
      <AgentModelTermsSections />
    </LegalPageFrame>
  );
}
