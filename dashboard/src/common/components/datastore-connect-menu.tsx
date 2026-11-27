import { useEffect, useState, type ReactNode } from "react";
import type { ApolloClient } from "@apollo/client";
import { ChevronDown, ExternalLink, Loader2, RotateCw } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/common/components/ui/popover";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/common/components/ui/tabs";
import { CopyButton } from "@/common/components/copy-button";
import { ReservedButton } from "@/common/components/reserved-skeleton";
import { SkeletonSpan } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";
import {
  PLACEHOLDER_SENTINEL,
  aroundPlaceholder,
} from "@/common/lib/i18n-placeholder";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useConnectionReveal } from "@/features/capabilities/hooks/use-connection-reveal";

/** bex's CLI docs, where Render's Connect links its own CLI. */
const CLI_DOCS_HREF = "https://bex.co/docs/cli";

/**
 * The two strings a datastore's Connect menu reads when it opens, named as
 * both Info cards' connection-info views name them.
 */
export interface DatastoreConnectValues {
  internalConnectionString: string;
  externalConnectionString: string;
}

/** One datastore's words and links in its Connect menu. */
export interface DatastoreConnectCopy {
  /** Render's row titles: "Internal Database URL", "Internal Redis URL"… */
  internalTitle: string;
  externalTitle: string;
  cliTitle: string;
  /** The docs sections Render's two descriptions link. */
  internalDocsHref: string;
  externalDocsHref: string;
  /** The feature's own copy toasts, as ConnectionField takes them. */
  copied: string;
  copyError: string;
}

export interface DatastoreConnectMenuProps {
  id: string;
  /**
   * Reads the values (the Info card's own query, never another), a stable
   * module-level function as useConnectionReveal requires.
   */
  readValues: (
    client: ApolloClient,
    id: string,
  ) => Promise<DatastoreConnectValues>;
  copy: DatastoreConnectCopy;
  /**
   * Whether the external endpoint admits anyone: Render shows the external
   * URL and the CLI command only once an IP rule does (`public` in bex).
   */
  externalOpen: boolean;
  /** `bex psql <id>` or `bex kv-cli <id>`, Render's `render psql <id>`. */
  cliCommand: string;
  /**
   * A link to the Info card holding the IP rules, named as that card is
   * (Render's "Networking section").
   */
  networkingLink: ReactNode;
  /** The member may not read connection info: the button is disabled. */
  disabled: boolean;
  /** Why, in the tooltip of the disabled button. */
  disabledReason?: string;
}

/**
 * Render's View docs is a text link, not a button: with no padding of its own
 * it starts at the header's edge when the actions wrap under the name, and it
 * is as tall as Connect beside it.
 */
const VIEW_DOCS_CLASS =
  "inline-flex h-8 items-center gap-1.5 rounded-md text-sm font-medium underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring [&_svg]:size-4 [&_svg]:shrink-0";

/**
 * A datastore header's actions, Render's View docs then Connect, 24 px apart
 * (docs/render-artifacts/postgres-dashboard.md § Header).
 */
export function DatastoreHeaderActions({
  docsHref,
  children,
}: {
  docsHref: string;
  /** The datastore's Connect menu. */
  children: ReactNode;
}) {
  const { t } = useTranslations();
  return (
    <div className="flex shrink-0 items-center gap-6">
      <a
        href={docsHref}
        target="_blank"
        rel="noreferrer"
        className={VIEW_DOCS_CLASS}
      >
        {t("common.datastoreViewDocs")}
        <ExternalLink aria-hidden="true" />
      </a>
      {children}
    </div>
  );
}

/**
 * The actions' pending frame: each control's own box with its own label, so
 * the header neither grows nor shifts when the resource resolves.
 */
export function DatastoreHeaderActionsSkeleton() {
  const { t } = useTranslations();
  return (
    <div
      data-skeleton-region="resource-actions"
      className="flex shrink-0 items-center gap-6"
    >
      <span className="relative inline-flex shrink-0">
        <span className={cn(VIEW_DOCS_CLASS, "invisible")}>
          {t("common.datastoreViewDocs")}
          <ExternalLink aria-hidden="true" />
        </span>
        <SkeletonSpan />
      </span>
      <ReservedButton variant="outline" size="sm">
        {t("common.datastoreConnect")}
        <ChevronDown className="size-3.5" aria-hidden="true" />
      </ReservedButton>
    </div>
  );
}

/**
 * Render's Connect menu for a Postgres database or a Key Value instance
 * (w3/m675): an Internal tab with the internal URL, and an External tab with
 * the external URL and the CLI command, or Render's note that no external
 * traffic is allowed yet. The values are read each time the menu opens, with
 * the Info card's query, and leave memory when it closes.
 */
export function DatastoreConnectMenu({
  disabled,
  disabledReason,
  ...props
}: DatastoreConnectMenuProps) {
  const { t } = useTranslations();
  // A link inside the panel (Networking) navigates without unmounting the
  // header, so the panel would stay open over the destination (as the
  // service's Connect, w4/m183).
  const [open, setOpen] = useState(false);
  const trigger = (
    <Button variant="outline" size="sm" disabled={disabled}>
      {t("common.datastoreConnect")}
      <ChevronDown className="size-3.5" aria-hidden="true" />
    </Button>
  );
  if (disabled) {
    return (
      <PermissionTooltip reason={disabledReason}>{trigger}</PermissionTooltip>
    );
  }
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>{trigger}</PopoverTrigger>
      <PopoverContent
        align="end"
        aria-label={t("common.datastoreConnect")}
        className="w-[22rem] max-w-[calc(100vw-2rem)] p-3 sm:w-[30rem]"
        onClickCapture={(e) => {
          if ((e.target as Element).closest("a[href]")) setOpen(false);
        }}
      >
        <ConnectPanel {...props} />
      </PopoverContent>
    </Popover>
  );
}

type ConnectPanelProps = Omit<
  DatastoreConnectMenuProps,
  "disabled" | "disabledReason"
>;

/** What one value row shows: the value is read, failed, or there. */
type ValueStatus = "loading" | "error" | "ready";

/** Mounted only while the menu is open, so opening it is what reads. */
function ConnectPanel({
  id,
  readValues,
  copy,
  externalOpen,
  cliCommand,
  networkingLink,
}: ConnectPanelProps) {
  const { t } = useTranslations();
  const { info, loading, error, reveal } = useConnectionReveal(id, readValues);
  useEffect(() => {
    void reveal();
  }, [reveal]);
  const status: ValueStatus = info
    ? "ready"
    : error && !loading
      ? "error"
      : "loading";
  const valueRow = (title: string, value: string) => (
    <ValueRow
      title={title}
      value={value}
      status={status}
      onRetry={() => void reveal()}
      copy={copy}
    />
  );

  return (
    <Tabs defaultValue="internal" className="gap-3">
      <TabsList className="w-full">
        <TabsTrigger value="internal">
          {t("common.datastoreConnectInternal")}
        </TabsTrigger>
        <TabsTrigger value="external">
          {t("common.datastoreConnectExternal")}
        </TabsTrigger>
      </TabsList>
      <TabsContent value="internal" className="py-1">
        <Section
          title={copy.internalTitle}
          description={withLink(
            t("common.datastoreConnectInternalBody", {
              link: PLACEHOLDER_SENTINEL,
            }),
            <DocsLink href={copy.internalDocsHref}>
              {t("common.datastoreConnectInternalLink")}
            </DocsLink>,
          )}
        >
          {valueRow(copy.internalTitle, info?.internalConnectionString ?? "")}
        </Section>
      </TabsContent>
      <TabsContent value="external" className="space-y-6 py-1">
        {externalOpen ? (
          <>
            <Section
              title={copy.externalTitle}
              description={withLink(
                t("common.datastoreConnectExternalBody", {
                  link: PLACEHOLDER_SENTINEL,
                }),
                <DocsLink href={copy.externalDocsHref}>
                  {t("common.datastoreConnectExternalLink")}
                </DocsLink>,
              )}
            >
              {valueRow(
                copy.externalTitle,
                info?.externalConnectionString ?? "",
              )}
            </Section>
            <Section
              title={copy.cliTitle}
              description={withLink(
                t("common.datastoreConnectCliBody", {
                  link: PLACEHOLDER_SENTINEL,
                }),
                <DocsLink href={CLI_DOCS_HREF}>
                  {t("common.datastoreConnectCliLink")}
                </DocsLink>,
              )}
            >
              {/* The command names the resource, not a secret: nothing to
                  read before it can be copied. */}
              <ValueRow
                title={copy.cliTitle}
                value={cliCommand}
                status="ready"
                copy={copy}
              />
            </Section>
          </>
        ) : (
          <Section
            title={copy.externalTitle}
            description={withLink(
              t("common.datastoreConnectExternalClosed", {
                link: PLACEHOLDER_SENTINEL,
              }),
              networkingLink,
            )}
          />
        )}
      </TabsContent>
    </Tabs>
  );
}

/** A translation made with the sentinel as `{link}`, the link in its place. */
function withLink(translated: string, link: ReactNode): ReactNode {
  const [before, after] = aroundPlaceholder(translated);
  return (
    <>
      {before}
      {link}
      {after}
    </>
  );
}

function DocsLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noreferrer"
      className="text-foreground underline underline-offset-2"
    >
      {children}
    </a>
  );
}

/** One of Render's sections: its title, its line, and its value. */
function Section({
  title,
  description,
  children,
}: {
  title: string;
  description: ReactNode;
  children?: ReactNode;
}) {
  return (
    <div className="space-y-2">
      <div>
        <p className="text-sm font-medium">{title}</p>
        <p className="text-muted-foreground text-sm">{description}</p>
      </div>
      {children}
    </div>
  );
}

/**
 * A value on one line, with Render's one button at its end: a spinner while
 * it is read, a retry after an error, Copy once it is there.
 */
function ValueRow({
  title,
  value,
  status,
  onRetry,
  copy,
}: {
  title: string;
  value: string;
  status: ValueStatus;
  onRetry?: () => void;
  copy: DatastoreConnectCopy;
}) {
  const { t } = useTranslations();
  const shown = status === "ready" ? value : "";
  return (
    <div
      role="group"
      aria-label={title}
      className="bg-muted flex h-10 min-w-0 items-center gap-1 rounded-md pr-1 pl-3"
    >
      <code className="min-w-0 flex-1 truncate font-mono text-xs" title={shown}>
        {shown}
      </code>
      {status === "loading" ? (
        <span
          role="status"
          aria-label={t("common.loading")}
          className="flex size-8 shrink-0 items-center justify-center"
        >
          <Loader2 className="size-4 animate-spin" aria-hidden="true" />
        </span>
      ) : status === "error" ? (
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={onRetry}
          aria-label={t("common.datastoreConnectRetry")}
          title={t("common.datastoreConnectRetry")}
        >
          <RotateCw className="text-destructive" aria-hidden="true" />
        </Button>
      ) : (
        <CopyButton
          value={value}
          label={t("common.copyField", { field: title })}
          successText={copy.copied}
          errorText={copy.copyError}
        />
      )}
    </div>
  );
}
