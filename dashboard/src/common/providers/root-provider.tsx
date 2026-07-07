import { I18nextProvider } from "react-i18next";
import { ThemeProvider } from "@/common/providers/theme-provider";
import { Toaster } from "@/common/components/ui/sonner";
import { VisualViewportHeight } from "@/common/providers/visual-viewport-height";
import i18n from "@/i18n/init";

export const RootProvider = ({ children }: { children: React.ReactNode }) => {
  return (
    <I18nextProvider i18n={i18n}>
      <ThemeProvider>
        {children}
        <VisualViewportHeight />
        <Toaster />
      </ThemeProvider>
    </I18nextProvider>
  );
};
