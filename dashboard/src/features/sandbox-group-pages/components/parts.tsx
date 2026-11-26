import { useState, type ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import { AlertTriangle, ChevronLeft, ChevronRight, Search } from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/common/components/ui/tabs";
import { CodeBlock } from "@/common/components/code-block";
import { CopyButton } from "@/common/components/copy-button";
import { useNow } from "@/common/hooks/use-now";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatDuration } from "@/common/lib/format/format-duration";
import {
  BUILD_STATUS_LABEL,
  isEndedSandbox,
  isSandboxStatus,
  sandboxDurationMs,
  SANDBOX_STATUS_LABEL,
  SNAPSHOT_STATUS_LABEL,
  TIME_RANGES,
  TIME_RANGE_LABEL,
  type TimeRange,
} from "@/features/sandbox-group-pages/lib/sandbox-view";
import {
  API_BASE_URL,
  CLI_INSTALL,
  cliLogin,
  SDK_INSTALL,
  SDK_LANGUAGES,
  SDK_RUN,
  sdkApiKey,
  type SdkLanguage,
} from "@/features/sandbox-group-pages/lib/starter-code";

type BadgeVariant =
  | "default"
  | "secondary"
  | "destructive"
  | "outline"
  | "success";

const SANDBOX_VARIANT: Record<string, BadgeVariant> = {
  running: "success",
  creating: "secondary",
  resuming: "secondary",
  suspending: "secondary",
  suspended: "outline",
  errored: "destructive",
  terminated: "outline",
};

export function SandboxStatusBadge({ status }: { status: string }) {
  const { t } = useTranslations();
  return (
    <Badge variant={SANDBOX_VARIANT[status] ?? "outline"}>
      {isSandboxStatus(status) ? t(SANDBOX_STATUS_LABEL[status]) : status}
    </Badge>
  );
}

const SNAPSHOT_VARIANT: Record<string, BadgeVariant> = {
  creating: "secondary",
  available: "success",
  failed: "destructive",
};

export function SnapshotStatusBadge({ status }: { status: string }) {
  const { t } = useTranslations();
  const key = SNAPSHOT_STATUS_LABEL[status];
  return (
    <Badge variant={SNAPSHOT_VARIANT[status] ?? "outline"}>
      {key ? t(key) : status}
    </Badge>
  );
}

const BUILD_VARIANT: Record<string, BadgeVariant> = {
  pending: "secondary",
  running: "secondary",
  succeeded: "success",
  failed: "destructive",
};

export function BuildStatusBadge({
  status,
  latest,
}: {
  status: string;
  latest?: boolean;
}) {
  const { t } = useTranslations();
  if (status === "succeeded" && latest) {
    return (
      <Badge variant="success">{t("sandboxGroupPages.imageLatest")}</Badge>
    );
  }
  const key = BUILD_STATUS_LABEL[status];
  return (
    <Badge variant={BUILD_VARIANT[status] ?? "outline"}>
      {key ? t(key) : status}
    </Badge>
  );
}

/** An id in monospace with its copy button. */
export function IdWithCopy({ id, label }: { id: string; label: string }) {
  const { t } = useTranslations();
  return (
    <span className="inline-flex min-w-0 items-center gap-1 font-mono text-xs">
      <span className="truncate">{id}</span>
      <CopyButton
        value={id}
        label={label}
        successText={t("sandboxGroupPages.copied")}
        errorText={t("sandboxGroupPages.copyError")}
      />
    </span>
  );
}

export function SearchBox({
  value,
  onChange,
  placeholder,
}: {
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
}) {
  return (
    <div className="relative w-full sm:max-w-xs">
      <Search
        className="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2"
        aria-hidden="true"
      />
      <Input
        type="search"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        aria-label={placeholder}
        className="h-8 pl-8"
      />
    </div>
  );
}

export function TimeRangeSelect({
  value,
  onChange,
}: {
  value: TimeRange;
  onChange: (value: TimeRange) => void;
}) {
  const { t } = useTranslations();
  return (
    <Select value={value} onValueChange={(v) => onChange(v as TimeRange)}>
      <SelectTrigger
        size="sm"
        className="w-full sm:w-40"
        aria-label={t("sandboxGroupPages.rangeLabel")}
      >
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {TIME_RANGES.map((range) => (
          <SelectItem key={range} value={range}>
            {t(TIME_RANGE_LABEL[range])}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

/** Previous and Next over cursor pages. */
export function PagingControls({
  page,
  hasMore,
  onPrevious,
  onNext,
}: {
  page: number;
  hasMore: boolean;
  onPrevious: () => void;
  onNext: () => void;
}) {
  const { t } = useTranslations();
  if (page === 0 && !hasMore) return null;
  return (
    <div className="flex items-center justify-end gap-2">
      <Button
        size="sm"
        variant="outline"
        onClick={onPrevious}
        disabled={page === 0}
      >
        <ChevronLeft className="size-4" aria-hidden="true" />
        {t("sandboxGroupPages.previousPage")}
      </Button>
      <Button size="sm" variant="outline" onClick={onNext} disabled={!hasMore}>
        {t("sandboxGroupPages.nextPage")}
        <ChevronRight className="size-4" aria-hidden="true" />
      </Button>
    </div>
  );
}

/** One labelled row of a drawer's details. */
export function DetailRow({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-0.5 py-2 sm:flex-row sm:items-start sm:justify-between sm:gap-4">
      <dt className="text-muted-foreground text-sm">{label}</dt>
      <dd className="min-w-0 text-sm break-words sm:text-right">{children}</dd>
    </div>
  );
}

function Step({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="space-y-1">
      <h4 className="text-sm font-medium">{title}</h4>
      {children}
    </section>
  );
}

/**
 * Render's quick-start empty state: a title, a line, and CLI and SDK tabs
 * with the commands and a starter file.
 */
export function StarterPanel({
  title,
  description,
  workspaceId,
  cli,
  sdk,
}: {
  title: string;
  description: string;
  workspaceId: string;
  /** The steps after installing the CLI and logging in. */
  cli: { title: string; command: string }[];
  sdk: {
    title: string;
    description?: string;
    file: Record<SdkLanguage, string>;
    code: (language: SdkLanguage, apiBaseUrl: string) => string;
    run?: boolean;
  };
}) {
  const { t } = useTranslations();
  const [language, setLanguage] = useState<SdkLanguage>("typescript");
  const [os, setOs] = useState<keyof typeof CLI_INSTALL>("homebrew");
  return (
    <div
      data-skeleton-region="starter-panel"
      className="space-y-4 rounded-lg border p-4 sm:p-6"
    >
      <div className="space-y-1">
        <h3 className="text-base font-semibold">{title}</h3>
        <p className="text-muted-foreground text-sm">{description}</p>
      </div>
      <Tabs defaultValue="cli">
        <TabsList variant="underline" className="justify-start">
          <TabsTrigger value="cli">
            {t("sandboxGroupPages.starterCli")}
          </TabsTrigger>
          <TabsTrigger value="sdk">
            {t("sandboxGroupPages.starterSdk")}
          </TabsTrigger>
        </TabsList>
        <TabsContent value="cli" className="min-w-0 space-y-4">
          <Step title={t("sandboxGroupPages.starterInstallCli")}>
            <div className="flex gap-2">
              {(Object.keys(CLI_INSTALL) as (keyof typeof CLI_INSTALL)[]).map(
                (key) => (
                  <Button
                    key={key}
                    size="sm"
                    variant={os === key ? "secondary" : "ghost"}
                    onClick={() => setOs(key)}
                  >
                    {key === "homebrew"
                      ? t("sandboxGroupPages.starterHomebrew")
                      : t("sandboxGroupPages.starterDirect")}
                  </Button>
                ),
              )}
            </div>
            <CodeBlock code={CLI_INSTALL[os]} language="shell" />
          </Step>
          <Step title={t("sandboxGroupPages.starterLogin")}>
            <CodeBlock code={cliLogin(workspaceId)} language="shell" />
          </Step>
          {cli.map((step) => (
            <Step key={step.title} title={step.title}>
              <CodeBlock code={step.command} language="shell" />
            </Step>
          ))}
        </TabsContent>
        <TabsContent value="sdk" className="min-w-0 space-y-4">
          <SdkLanguageToggle value={language} onChange={setLanguage} />
          {sdk.run ? (
            <>
              <Step title={t("sandboxGroupPages.starterApiKey")}>
                <p className="text-muted-foreground text-sm">
                  {t("sandboxGroupPages.starterApiKeyHelp")}
                </p>
                <CodeBlock code={sdkApiKey()} language="shell" />
              </Step>
              <Step title={t("sandboxGroupPages.starterInstallSdk")}>
                <p className="text-muted-foreground text-sm">
                  {language === "python"
                    ? t("sandboxGroupPages.starterPythonNeeds")
                    : t("sandboxGroupPages.starterNodeNeeds")}
                </p>
                <CodeBlock code={SDK_INSTALL[language]} language="shell" />
              </Step>
            </>
          ) : null}
          <Step title={sdk.title}>
            {sdk.description ? (
              <p className="text-muted-foreground text-sm">{sdk.description}</p>
            ) : null}
            <CodeBlock
              code={sdk.code(language, API_BASE_URL)}
              language={sdk.file[language]}
            />
          </Step>
          {sdk.run ? (
            <Step title={t("sandboxGroupPages.starterRun")}>
              <CodeBlock code={SDK_RUN[language]} language="shell" />
            </Step>
          ) : null}
        </TabsContent>
      </Tabs>
    </div>
  );
}

/** TypeScript or Python, for a block of SDK code. */
export function SdkLanguageToggle({
  value,
  onChange,
}: {
  value: SdkLanguage;
  onChange: (language: SdkLanguage) => void;
}) {
  const { t } = useTranslations();
  return (
    <div className="flex gap-2">
      {SDK_LANGUAGES.map((lang) => (
        <Button
          key={lang}
          size="sm"
          variant={value === lang ? "secondary" : "ghost"}
          onClick={() => onChange(lang)}
        >
          {lang === "typescript"
            ? t("sandboxGroupPages.starterTypeScript")
            : t("sandboxGroupPages.starterPython")}
        </Button>
      ))}
    </div>
  );
}

/** A pane that could not load, announced, with Try again. */
export function LoadError({
  title,
  onRetry,
}: {
  title: string;
  onRetry: () => void;
}) {
  const { t } = useTranslations();
  return (
    <div
      role="alert"
      className="flex flex-col items-center gap-2 rounded-lg border py-10 text-center"
    >
      <AlertTriangle
        className="text-muted-foreground size-6"
        aria-hidden="true"
      />
      <p className="text-sm font-medium">{title}</p>
      <Button size="sm" variant="outline" onClick={onRetry}>
        {t("common.tryAgain")}
      </Button>
    </div>
  );
}

/**
 * How long a sandbox has run: ticking while it lives, fixed once it ended,
 * so only a live row re-renders every second.
 */
export function SandboxDuration({
  sandbox,
}: {
  sandbox: {
    status: string;
    createdAt: string | null;
    startedAt: string | null;
    endedAt: string | null;
  };
}) {
  return isEndedSandbox(sandbox.status) ? (
    <DurationText sandbox={sandbox} now={0} />
  ) : (
    <LiveDurationText sandbox={sandbox} />
  );
}

function LiveDurationText({
  sandbox,
}: {
  sandbox: Parameters<typeof SandboxDuration>[0]["sandbox"];
}) {
  const now = useNow(1_000);
  return <DurationText sandbox={sandbox} now={now} />;
}

function DurationText({
  sandbox,
  now,
}: {
  sandbox: Parameters<typeof SandboxDuration>[0]["sandbox"];
  now: number;
}) {
  const { i18n } = useTranslations();
  const ms = sandboxDurationMs(sandbox, now);
  return <>{ms == null ? "—" : formatDuration(ms, i18n.language)}</>;
}

/** The sandboxes that started from a snapshot or an image, each a link to its drawer. */
export function StartedSandboxesList({
  groupId,
  sandboxes,
  loading,
  empty,
}: {
  groupId: string;
  sandboxes: { id: string; name: string | null; status: string }[];
  loading: boolean;
  empty: string;
}) {
  const { t } = useTranslations();
  if (loading) {
    return (
      <p className="text-muted-foreground text-sm">
        {t("sandboxGroupPages.loading")}
      </p>
    );
  }
  if (sandboxes.length === 0) {
    return <p className="text-muted-foreground text-sm">{empty}</p>;
  }
  return (
    <ul className="divide-y rounded-lg border">
      {sandboxes.map((sb) => (
        <li key={sb.id} className="flex items-center justify-between gap-2 p-2">
          <Link
            to="/sandbox-groups/$sandboxGroupId"
            params={{ sandboxGroupId: groupId }}
            search={{ sandbox: sb.id }}
            className="truncate text-sm hover:underline"
          >
            {sb.name ?? sb.id}
          </Link>
          <SandboxStatusBadge status={sb.status} />
        </li>
      ))}
    </ul>
  );
}
