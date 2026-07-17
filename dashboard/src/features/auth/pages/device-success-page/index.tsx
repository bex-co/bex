import { ArrowRight, CheckCircle2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { AuthPageShell } from "@/features/auth/components/auth-page-shell";

/**
 * Device-flow landing page: the user just approved `render login` in this
 * browser tab and the CLI's poll loop is picking up tokens on its own. The
 * page replays the CLI's transcript in a terminal replica to point the user
 * back where the session actually continues — their terminal.
 *
 * The terminal keeps a fixed dark palette in both themes (terminals don't
 * follow the page theme) and is aria-hidden: every line it animates in is
 * also stated in the shell title/subtitle or the hint text below it.
 */
export default function DeviceSuccessPage() {
  const { t } = useTranslations();
  return (
    <AuthPageShell
      title={t("auth.deviceSuccessTitle")}
      subtitle={t("auth.deviceSuccessSubtitle")}
    >
      <div className="space-y-6">
        <div
          aria-hidden="true"
          className="overflow-hidden rounded-xl border border-zinc-800 bg-zinc-950 shadow-lg animate-in fade-in zoom-in-95 duration-500 motion-reduce:animate-none"
        >
          <div className="flex items-center gap-1.5 border-b border-zinc-800 bg-zinc-900 px-4 py-2.5">
            <span className="size-2.5 rounded-full bg-zinc-700" />
            <span className="size-2.5 rounded-full bg-zinc-700" />
            <span className="size-2.5 rounded-full bg-zinc-700" />
            <span className="ml-2 font-mono text-xs text-zinc-500">
              render login
            </span>
          </div>
          <div className="space-y-1.5 px-4 py-4 font-mono text-[13px] leading-relaxed">
            <p>
              <span className="text-zinc-500">$ </span>
              <span className="text-zinc-100">render login</span>
            </p>
            <p className="text-zinc-500 animate-in fade-in fill-mode-backwards delay-300 duration-500 motion-reduce:animate-none">
              {t("auth.deviceSuccessWaiting")}
            </p>
            <p className="text-[oklch(0.7344_0.2016_138.3)] animate-in fade-in slide-in-from-bottom-1 fill-mode-backwards delay-700 duration-500 motion-reduce:animate-none">
              ✓ {t("auth.deviceSuccessDone")}
            </p>
            <p className="animate-in fade-in fill-mode-backwards delay-1000 duration-300 motion-reduce:animate-none">
              <span className="text-zinc-500">$ </span>
              <span className="inline-block h-3.5 w-2 translate-y-0.5 bg-zinc-400 animate-caret-blink motion-reduce:animate-none" />
            </p>
          </div>
        </div>

        <div className="flex items-start gap-2.5 text-sm animate-in fade-in fill-mode-backwards delay-1000 duration-500 motion-reduce:animate-none">
          <CheckCircle2 className="mt-0.5 size-5 shrink-0 text-primary" />
          <p className="text-muted-foreground">
            <span className="font-medium text-foreground">
              {t("auth.deviceSuccessHint")}
            </span>{" "}
            {t("auth.deviceSuccessClose")}
          </p>
        </div>

        <Button
          asChild
          variant="link"
          size="sm"
          className="h-auto px-0 text-muted-foreground hover:text-foreground"
        >
          <a href="/">
            {t("auth.deviceSuccessDashboard")}
            <ArrowRight className="size-4" />
          </a>
        </Button>
      </div>
    </AuthPageShell>
  );
}
