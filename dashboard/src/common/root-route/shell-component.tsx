import { HeadContent, Scripts } from "@tanstack/react-router";
import { ThemeScript } from "@/common/components/document/theme-script";
import { useRootContext } from "@/common/hooks/use-root-context";

export function ShellComponent({
  children,
}: {
  children: React.ReactNode;
}): React.ReactNode {
  const { language } = useRootContext();

  return (
    <html lang={language} suppressHydrationWarning>
      <head>
        <ThemeScript />
        <HeadContent />
      </head>
      <body className="font-sans antialiased [overflow-wrap:anywhere] selection:bg-[rgba(79,184,178,0.24)]">
        {children}
        <Scripts />
      </body>
    </html>
  );
}
