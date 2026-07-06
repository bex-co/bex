import { ThemeProvider } from "@/common/providers/theme-provider";
import { Toaster } from "@/common/components/ui/sonner";
import { VisualViewportHeight } from "@/common/providers/visual-viewport-height";

export const RootProvider = ({ children }: { children: React.ReactNode }) => {
  return (
    <ThemeProvider>
      {children}
      <VisualViewportHeight />
      <Toaster />
    </ThemeProvider>
  );
};
