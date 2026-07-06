import { HeadContent, Scripts } from "@tanstack/react-router";
import { ThemeScript } from "@/common/components/document/theme-script";

export function ShellComponent({
  children,
}: {
  children: React.ReactNode;
}): React.ReactNode {
  return (
    <html lang="en" suppressHydrationWarning>
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
