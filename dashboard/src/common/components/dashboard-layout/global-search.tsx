import { lazy, Suspense, useEffect, useState } from "react";
import { Search } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";

const loadGlobalSearchDialog = () =>
  import("./global-search-dialog").then((m) => ({
    default: m.GlobalSearchDialog,
  }));

const GlobalSearchDialog = lazy(loadGlobalSearchDialog);

/**
 * Workspace-wide command search, opened from any dashboard page or with
 * Cmd/Ctrl+K. The cmdk dialog (and its resource hooks) load only after the
 * first open (or hover/focus prefetch), keeping the persistent topbar free of
 * that weight on every chrome page. Once mounted, the dialog stays mounted so
 * cmdk's close animation is not cut by unmount-on-close.
 */
export function GlobalSearch() {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);
  const [mounted, setMounted] = useState(false);
  // Reading navigator.platform during render makes the first client render
  // disagree with the server ("⌘ K" vs "Ctrl K"), a hydration text mismatch
  // (React #418) on every page since this lives in the persistent header. Start
  // false so the server and the first client render agree on "Ctrl K", then
  // switch to the Mac glyph after mount.
  const [isMac, setIsMac] = useState(false);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- post-hydration platform detection, intentionally after first paint
    setIsMac(/Mac|iPhone|iPad/.test(navigator.platform));
  }, []);

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key.toLowerCase() === "k" && (event.metaKey || event.ctrlKey)) {
        event.preventDefault();
        setMounted(true);
        setOpen((value) => !value);
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, []);

  function prefetchDialog() {
    void loadGlobalSearchDialog();
  }

  function openDialog() {
    setMounted(true);
    setOpen(true);
  }

  return (
    <>
      <Button
        variant="ghost"
        size="sm"
        className="gap-2 px-2 text-muted-foreground hover:text-foreground sm:px-3"
        onClick={openDialog}
        onPointerEnter={prefetchDialog}
        onFocus={prefetchDialog}
        aria-label={t("common.topbarSearch")}
      >
        <Search />
        <span className="hidden md:inline">{t("common.topbarSearch")}</span>
        <kbd className="hidden rounded border bg-muted px-1.5 py-0.5 font-sans text-[10px] font-medium lg:inline">
          {isMac ? "⌘ K" : "Ctrl K"}
        </kbd>
      </Button>
      {mounted ? (
        <Suspense fallback={null}>
          <GlobalSearchDialog open={open} onOpenChange={setOpen} />
        </Suspense>
      ) : null}
    </>
  );
}
