import { IntlProvider } from "react-intl";
import { OryLocales } from "@ory/elements-react";
import { Toaster } from "@/common/components/ui/sonner";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * The `<Toaster/>`, wrapped in the intl context Ory Elements' toasts need.
 *
 * Ory's flow components raise their messages as toasts via sonner's `toast()`,
 * which PORTALS the toast into `<Toaster/>` — so the toast renders in the
 * Toaster's tree, not under the `<Settings>`/`<Login>` that fired it, and
 * therefore *outside* the IntlProvider those flow components mount internally.
 * Ory's `DefaultToast` calls `useIntl()`, so with no intl context above the
 * Toaster it throws "Could not find required `intl` object" and the root error
 * boundary takes down the entire page. That made /settings unusable the moment
 * a flow produced any message — e.g. landing there from a completed recovery,
 * or a rejected password — which is exactly how it shipped broken.
 *
 * Ory's own catalog (`OryLocales`) is passed so the toast stays translated
 * rather than falling back to the hardcoded English defaults; the locale
 * follows i18next, matching `useOryConfig()`'s `intl.locale`.
 *
 * This lives in its own module so `react-intl`/formatjs (~15–30 KB gzip, a
 * second i18n stack used only here) is lazy-loaded via `React.lazy` from
 * `root-provider.tsx` instead of pinned into the always-mounted entry chunk
 * (w9/m60 t004).
 */
export default function OryToaster() {
  const { i18n: i18next } = useTranslations();
  const locale = i18next.language;
  const messages = (OryLocales as Record<string, Record<string, string>>)[
    locale
  ];

  return (
    <IntlProvider
      locale={locale}
      defaultLocale="en"
      messages={messages ?? OryLocales.en}
    >
      <Toaster />
    </IntlProvider>
  );
}
