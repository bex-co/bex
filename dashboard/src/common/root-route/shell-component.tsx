import { HeadContent, Scripts } from "@tanstack/react-router";
import { ThemeScript } from "@/common/components/document/theme-script";
import { useRootContext } from "@/common/hooks/use-root-context";
import {
  i18nBootstrapScript,
  resolveShellLanguage,
} from "@/i18n/client-bootstrap";

export function ShellComponent({
  children,
}: {
  children: React.ReactNode;
}): React.ReactNode {
  // `language` is typed always-present but is transiently `undefined` while a
  // navigation's root `beforeLoad` is in flight — `resolveShellLanguage`
  // covers that window (see its doc).
  const { language } = useRootContext();
  const shellLanguage = resolveShellLanguage(language);
  // Stamp a non-default session's language + catalog into the HTML so the client
  // bundle can seed i18next synchronously and hydrate in that language, instead
  // of flashing the English fallback over a translated document (w6/m103). Runs
  // before <Scripts/>; `null` (no tag) for the default language.
  const bootstrap = i18nBootstrapScript(shellLanguage);

  return (
    <html lang={shellLanguage} suppressHydrationWarning>
      <head>
        <ThemeScript />
        {bootstrap ? (
          <script
            dangerouslySetInnerHTML={{ __html: bootstrap }}
            suppressHydrationWarning
          />
        ) : null}
        <HeadContent />
      </head>
      <body className="font-sans antialiased [overflow-wrap:anywhere] selection:bg-[rgba(79,184,178,0.24)]">
        {children}
        <Scripts />
      </body>
    </html>
  );
}
