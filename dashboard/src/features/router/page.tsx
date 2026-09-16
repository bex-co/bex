import { useEffect, useState } from "react";
import { Copy, Plus, Settings } from "lucide-react";
import { toast } from "sonner";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/common/components/ui/dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useRouterOverview,
  type RouterKey,
  type RouterKeyOptions,
  type RouterOverview,
} from "./data";

const WINDOWS = [
  ["FIVE_HOUR", "router.fiveHour"],
  ["WEEKLY", "router.weekly"],
  ["MONTHLY", "router.monthly"],
] as const;

export function RouterPage() {
  const state = useRouterOverview();
  return <RouterContent key={state.ownerId} {...state} />;
}

export function RouterPageSkeleton() {
  return <RouterContent loading />;
}

type RouterContentProps = {
  overview?: RouterOverview;
  loading?: boolean;
  error?: unknown;
  refetch?: () => unknown;
  create?: (name: string) => Promise<void>;
  update?: (
    id: string,
    name: string,
    options: RouterKeyOptions,
  ) => Promise<void>;
  remove?: (id: string) => Promise<void>;
};

export function RouterContent({
  overview,
  loading,
  error,
  refetch,
  create,
  update,
  remove,
}: RouterContentProps) {
  const { t, i18n } = useTranslations();
  const [editing, setEditing] = useState<RouterKey | "new" | null>(null);
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 30_000);
    return () => clearInterval(timer);
  }, []);
  const pending = Boolean(loading && !overview);
  const date = (value: string) => new Date(value).toLocaleString(i18n.language);
  const countdown = (resetsAt: string) => {
    const minutes = Math.max(
      0,
      Math.ceil(
        (Date.parse(resetsAt) -
          Math.max(now, Date.parse(overview?.quota?.observedAt ?? "") || now)) /
          60_000,
      ),
    );
    if (!minutes) return t("router.refreshing");
    const days = Math.floor(minutes / 1440);
    const hours = Math.floor((minutes % 1440) / 60);
    const parts = [
      days ? t("router.day", { count: days }) : "",
      hours ? t("router.hour", { count: hours }) : "",
      !days && minutes % 60 ? t("router.minute", { count: minutes % 60 }) : "",
    ].filter(Boolean);
    return t("router.resets", { time: parts.join(" ") });
  };
  return (
    <DashboardLayout>
      <div className="min-h-0 min-w-0 flex-1 overflow-auto">
        <div
          className="mx-auto w-full max-w-7xl space-y-8 p-4 sm:p-6"
          aria-busy={pending}
        >
          <section className="space-y-4" aria-label={t("router.usage")}>
            <div className="space-y-2">
              <h1 className="text-xl font-semibold">{t("router.usage")}</h1>
              <div className="text-muted-foreground h-5 text-sm">
                {pending ? (
                  <Skeleton className="h-5 w-64 max-w-full" />
                ) : overview?.quota ? (
                  t("router.snapshot", {
                    time: date(overview.quota.observedAt),
                  })
                ) : null}
              </div>
            </div>
            <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
              {WINDOWS.map(([kind, label]) => {
                const window = overview?.quota?.windows.find(
                  (w) => w.kind === kind,
                );
                const active = window && Date.parse(window.resetsAt) > now;
                const percentage = window ? window.utilizationBps / 100 : 0;
                return (
                  <Card key={kind} className="h-40 rounded-2xl shadow-sm">
                    <CardHeader className="flex flex-row items-center justify-between gap-2">
                      <CardTitle className="text-base">{t(label)}</CardTitle>
                      {pending ? (
                        <Skeleton className="h-5 w-14" />
                      ) : (
                        <span className="text-muted-foreground text-sm tabular-nums">
                          {active
                            ? `${new Intl.NumberFormat(i18n.language, { maximumFractionDigits: 2 }).format(percentage)}%`
                            : "—"}
                        </span>
                      )}
                    </CardHeader>
                    <CardContent className="space-y-4">
                      {pending ? (
                        <Skeleton className="h-2 w-full" />
                      ) : (
                        <div
                          role="progressbar"
                          aria-label={t(label)}
                          aria-valuemin={0}
                          aria-valuemax={100}
                          aria-valuenow={
                            active
                              ? Math.min(100, Math.max(0, percentage))
                              : undefined
                          }
                          className="h-2 overflow-hidden rounded-full bg-green-700/20"
                        >
                          <div
                            className="h-full rounded-full bg-green-700"
                            style={{
                              width: `${active ? Math.min(100, Math.max(0, percentage)) : 0}%`,
                            }}
                          />
                        </div>
                      )}
                      {pending ? (
                        <Skeleton className="h-5 w-4/5" />
                      ) : (
                        <p className="text-muted-foreground text-sm">
                          {window
                            ? countdown(window.resetsAt)
                            : t("router.unavailable")}
                        </p>
                      )}
                    </CardContent>
                  </Card>
                );
              })}
            </div>
          </section>
          <section className="space-y-6" aria-label={t("router.keys")}>
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div className="space-y-2">
                <h2 className="text-2xl font-semibold">{t("router.keys")}</h2>
                <p className="text-muted-foreground text-sm">
                  {t("router.description")}
                </p>
              </div>
              <Button
                className="bg-green-700 text-white hover:bg-green-800"
                disabled={pending || !overview || !!error || !create}
                onClick={() => setEditing("new")}
              >
                <Plus />
                {t("router.create")}
              </Button>
            </div>
            {error ? (
              <div
                role="alert"
                className="flex items-center justify-between gap-4 rounded-lg border p-4"
              >
                <span>{t("router.unavailable")}</span>
                <Button variant="outline" onClick={() => refetch?.()}>
                  {t("router.retry")}
                </Button>
              </div>
            ) : null}
            <div className="overflow-x-auto rounded-lg border">
              <table className="w-full min-w-[640px] text-left text-sm">
                <thead className="bg-muted/30">
                  <tr>
                    {["name", "key", "created", "action"].map((column) => (
                      <th
                        key={column}
                        className="px-4 py-3 font-medium last:text-right"
                      >
                        {t(`router.${column}`)}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {pending
                    ? Array.from({ length: 4 }, (_, i) => (
                        <tr key={i} className="h-[72px] border-t">
                          {[0, 1, 2, 3].map((j) => (
                            <td key={j} className="px-4 py-3">
                              <Skeleton className="h-5 w-4/5" />
                            </td>
                          ))}
                        </tr>
                      ))
                    : overview?.keys.map((key) => (
                        <tr key={key.id} className="h-[72px] border-t">
                          <td className="max-w-64 break-words px-4 py-3">
                            {key.name}
                          </td>
                          <td className="px-4 py-3">
                            <div className="flex items-center gap-2">
                              <code className="rounded bg-muted px-2 py-1">
                                {key.accessKey}
                              </code>
                              <Button
                                variant="ghost"
                                size="icon"
                                aria-label={t("router.copy")}
                                onClick={() => {
                                  void navigator.clipboard
                                    .writeText(key.accessKey)
                                    .then(
                                      () => toast.success(t("router.copied")),
                                      () => toast.error(t("router.copyFailed")),
                                    );
                                }}
                              >
                                <Copy className="size-4" />
                              </Button>
                            </div>
                          </td>
                          <td className="whitespace-nowrap px-4 py-3">
                            <time dateTime={key.createdAt}>
                              {date(key.createdAt)}
                            </time>
                          </td>
                          <td className="px-4 py-3 text-right">
                            <Button
                              variant="ghost"
                              size="icon"
                              aria-label={`${t("router.settings")}: ${key.name}`}
                              onClick={() => setEditing(key)}
                            >
                              <Settings className="size-4" />
                            </Button>
                          </td>
                        </tr>
                      ))}
                  {!pending && !error && overview?.keys.length === 0 ? (
                    <tr>
                      <td
                        colSpan={4}
                        className="h-32 p-6 text-center text-muted-foreground"
                      >
                        {t("router.empty")}
                      </td>
                    </tr>
                  ) : null}
                </tbody>
              </table>
            </div>
          </section>
          {editing && create && update && remove ? (
            <KeyDialog
              key={editing === "new" ? "new" : editing.id}
              editing={editing}
              close={() => setEditing(null)}
              create={create}
              update={update}
              remove={remove}
            />
          ) : null}
        </div>
      </div>
    </DashboardLayout>
  );
}

function KeyDialog({
  editing,
  close,
  create,
  update,
  remove,
}: {
  editing: RouterKey | "new";
  close: () => void;
  create: NonNullable<RouterContentProps["create"]>;
  update: NonNullable<RouterContentProps["update"]>;
  remove: NonNullable<RouterContentProps["remove"]>;
}) {
  const { t } = useTranslations();
  const existing = editing === "new" ? null : editing;
  const [name, setName] = useState(existing?.name ?? "");
  const [options, setOptions] = useState<RouterKeyOptions>(
    existing?.options ?? {
      allowOrigin: "",
      allowMethods: "",
      allowHeaders: "",
      allowCredentials: false,
    },
  );
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  async function submit(deleting = false) {
    setBusy(true);
    setFailed(false);
    try {
      if (existing) {
        if (deleting) await remove(existing.id);
        else await update(existing.id, name.trim(), options);
      } else await create(name.trim());
      close();
    } catch {
      setFailed(true);
    } finally {
      setBusy(false);
    }
  }
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !busy) close();
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {t(existing ? "router.settings" : "router.create")}
          </DialogTitle>
          <DialogDescription>{t("router.description")}</DialogDescription>
        </DialogHeader>
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            void submit();
          }}
        >
          <div className="space-y-2">
            <Label htmlFor="router-key-name">{t("router.name")}</Label>
            <Input
              id="router-key-name"
              value={name}
              maxLength={200}
              required
              disabled={busy}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          {existing ? (
            <>
              {(["allowOrigin", "allowMethods", "allowHeaders"] as const).map(
                (field) => (
                  <div key={field} className="space-y-2">
                    <Label htmlFor={`router-${field}`}>
                      {t(`router.${field}`)}
                    </Label>
                    <Input
                      id={`router-${field}`}
                      value={options[field]}
                      disabled={busy}
                      onChange={(e) =>
                        setOptions({ ...options, [field]: e.target.value })
                      }
                    />
                  </div>
                ),
              )}
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={options.allowCredentials}
                  disabled={busy}
                  onChange={(e) =>
                    setOptions({
                      ...options,
                      allowCredentials: e.target.checked,
                    })
                  }
                />
                {t("router.allowCredentials")}
              </label>
            </>
          ) : null}
          {failed ? (
            <p role="alert" className="text-destructive text-sm">
              {t("router.mutationFailed")}
            </p>
          ) : null}
          {confirmDelete ? (
            <p role="alert" className="text-sm">
              {t("router.deleteConfirm")}
            </p>
          ) : null}
          <div className="flex flex-wrap justify-end gap-2">
            {existing ? (
              <Button
                type="button"
                variant="destructive"
                disabled={busy}
                onClick={() => {
                  if (confirmDelete) void submit(true);
                  else setConfirmDelete(true);
                }}
              >
                {t("router.delete")}
              </Button>
            ) : null}
            <Button
              type="button"
              variant="outline"
              disabled={busy}
              onClick={close}
            >
              {t("router.cancel")}
            </Button>
            <Button disabled={busy || !name.trim()} type="submit">
              {t(
                busy
                  ? "router.saving"
                  : existing
                    ? "router.save"
                    : "router.create",
              )}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
