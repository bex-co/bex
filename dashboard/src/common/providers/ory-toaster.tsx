import { useEffect } from "react";
import { IntlProvider } from "react-intl";
import { OryLocales } from "@ory/elements-react";
import { toast } from "sonner";
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
      <ReplayEarlyToasts />
    </IntlProvider>
  );
}

/**
 * Shows the toasts raised before this lazily loaded Toaster subscribed.
 *
 * sonner delivers a toast only to the Toasters subscribed when it is raised
 * and never replays its store into one that subscribes later. Since the
 * Toaster became a lazy chunk (w9/m60 t004), a toast raised during hydration
 * (the "That resource doesn't exist" toast `useNotFoundRedirect` fires when a
 * directly opened dead-id URL resolves) landed before the chunk did and was
 * silently lost on six of eight route families (w4/158).
 *
 * This renders after <Toaster/>, so its mount effect runs after the Toaster's
 * own subscribe effect. It re-publishes each still-active toast through
 * `toast.message`, whose create path updates an existing id in place and
 * notifies subscribers, so the now-subscribed Toaster adds it once. It runs
 * once per page load: every active toast at that moment was raised before any
 * Toaster could show it. The sonner behavior it relies on is pinned by
 * ory-toaster.test.tsx.
 */
function ReplayEarlyToasts() {
  useEffect(() => {
    for (const early of toast.getToasts()) {
      if ("dismiss" in early && early.dismiss) continue;
      const { title, ...rest } = early as Parameters<
        typeof toast.message
      >[1] & {
        title?: Parameters<typeof toast.message>[0];
      };
      toast.message(title ?? "", rest);
    }
  }, []);
  return null;
}
