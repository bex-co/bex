import { Fragment, useEffect, type ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import { Check, Copy } from "lucide-react";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { PageScroll } from "@/common/components/page-frame";
import { ResourceLoadError } from "@/common/components/resource-load-error";
import { Button } from "@/common/components/ui/button";
import { useCopyToClipboard } from "@/common/hooks/use-copy-to-clipboard";
import {
  useReturnHomeLink,
  type ResourceStateLink,
} from "@/common/hooks/use-resource-state-links";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatDashboardTitle } from "@/common/lib/document-head";
import { PLACEHOLDER_SENTINEL } from "@/common/lib/i18n-placeholder";
import type { ResourceReadState } from "@/common/lib/resource-read-state";

/**
 * The states a resource page draws in place of itself when its id leads
 * nowhere, as Render's dashboard does (w3/m706,
 * docs/render-artifacts/resource-not-found.md): the URL stays, nothing toasts,
 * and the page says what was missing and where to go.
 *
 * - `ResourceNotFound` — Render's `ResourceNotFound`: "Service not found" or,
 *   when the cache still names it, "Service deleted".
 * - `PageNotFound` — Render's `Error404`: "Page not found", "<id> doesn’t
 *   exist.", for the pages that draw it (Postgres, Key Value, …).
 * - `AccessDenied` — Render's `Error403`, for a read bex-api refused.
 * - `ResourceLoadFailed` — Render's "An error occurred", for any other failure.
 */

/**
 * The tab's title while the state is drawn, the drawn title itself. The route
 * head's title is the loader's, which a resource deleted while its page was
 * open never re-runs, so the page sets it and gives back the one it replaced.
 *
 * The router owns the `<title>` element and may rewrite it after this state
 * mounts: a return to a resource deleted elsewhere draws "Sandbox group
 * deleted" from the cache, then the loader's background re-read commits its
 * "Page not found" head over it. While the URL is still this page's, the
 * state takes the title back. Once the URL moves on, the next page's head
 * owns the tab: giving back the old title then would leave "Page not found"
 * on the page Return home opened (w3/m706, production walk 2026-11-28).
 * Only the state mounted last claims, so two can never trade the title.
 */
let titleOwner: symbol | null = null;

function useStateDocumentTitle(title: string) {
  useEffect(() => {
    const owner = Symbol(title);
    titleOwner = owner;
    const wanted = formatDashboardTitle(title);
    const previous = document.title;
    const pathname = window.location.pathname;
    const onThisPage = () => window.location.pathname === pathname;
    const claim = () => {
      if (titleOwner === owner && onThisPage() && document.title !== wanted) {
        document.title = wanted;
      }
    };
    claim();
    const observer = new MutationObserver(claim);
    observer.observe(document.head, {
      childList: true,
      subtree: true,
      characterData: true,
    });
    return () => {
      observer.disconnect();
      if (titleOwner === owner) titleOwner = null;
      if (onThisPage() && document.title === wanted) document.title = previous;
    };
  }, [title]);
}

/**
 * One translation with several nodes in it: each node's placeholder gets a
 * numbered sentinel, so the translated words fall around the nodes in the
 * language's own order.
 */
function TranslatedWithNodes({
  translate,
  nodes,
}: {
  translate: (params: Record<string, string>) => string;
  nodes: Record<string, ReactNode>;
}) {
  const names = Object.keys(nodes);
  const params = Object.fromEntries(
    names.map((name, index) => [
      name,
      `${PLACEHOLDER_SENTINEL}${index}${PLACEHOLDER_SENTINEL}`,
    ]),
  );
  const parts = translate(params).split(
    new RegExp(`${PLACEHOLDER_SENTINEL}(\\d+)${PLACEHOLDER_SENTINEL}`),
  );
  return (
    <>
      {parts.map((part, index) => (
        <Fragment key={index}>
          {index % 2 === 0 ? part : nodes[names[Number(part)] ?? ""]}
        </Fragment>
      ))}
    </>
  );
}

/** The id as Render's inline code snippet: a code chip with a copy button. */
function ResourceIdSnippet({ id }: { id: string }) {
  const { t } = useTranslations();
  const { copied, copy } = useCopyToClipboard({
    errorText: t("common.copyIdError"),
  });
  return (
    <span className="bg-muted text-foreground inline-flex max-w-full items-center gap-0.5 rounded-md py-0.5 pr-0.5 pl-1.5 align-middle font-mono text-xs">
      <code className="truncate">{id}</code>
      <Button
        type="button"
        variant="ghost"
        size="icon-sm"
        className="size-6"
        onClick={() => void copy(id)}
        aria-label={t("common.copyId")}
      >
        {copied ? <Check className="text-green-600" /> : <Copy />}
      </Button>
    </span>
  );
}

/**
 * Render's ghost empty state inside its `Wrapper` (`mt-12 mb-20`): a 500 px
 * column, the title, the body, then the buttons — the primary first.
 */
function ResourceStateFrame({
  title,
  body,
  links,
  testId,
}: {
  title: string;
  body: ReactNode;
  links: ResourceStateLink[];
  testId: string;
}) {
  useStateDocumentTitle(title);
  return (
    <div className="mt-12 mb-20 w-full" data-resource-state={testId}>
      <div className="mx-auto w-full max-w-[500px] space-y-8 py-20">
        <div className="space-y-2">
          <h1 className="text-xl font-semibold tracking-tight">{title}</h1>
          {body ? (
            <p className="text-muted-foreground text-sm leading-6">{body}</p>
          ) : null}
        </div>
        <div className="flex flex-wrap gap-4">
          {links.map((link, index) => (
            <Button
              key={`${link.to}:${link.label}`}
              asChild
              variant={index === 0 ? "default" : "outline"}
            >
              <Link to={link.to} params={link.params}>
                {link.label}
              </Link>
            </Button>
          ))}
        </div>
      </div>
    </div>
  );
}

/** The resources whose pages draw Render's `ResourceNotFound`. */
export type MissingResourceKind =
  "service" | "cronJob" | "sandboxGroup" | "deploy" | "build";

/** Each kind's title, whole per language ("Service deleted", "Build not
 *  found"), so a gendered language agrees with its noun. A deploy or build
 *  has no deleted variant: Render always draws it as not found. */
const MISSING_TITLE_KEYS = {
  service: ["common.serviceNotFoundTitle", "common.serviceDeletedTitle"],
  cronJob: ["common.cronJobNotFoundTitle", "common.cronJobDeletedTitle"],
  sandboxGroup: [
    "common.sandboxGroupNotFoundTitle",
    "common.sandboxGroupDeletedTitle",
  ],
  deploy: ["common.deployNotFoundTitle", "common.deployNotFoundTitle"],
  build: ["common.buildNotFoundTitle", "common.buildNotFoundTitle"],
} as const;

export interface ResourceNotFoundProps {
  kind: MissingResourceKind;
  id: string;
  /** The cached name of a resource the dashboard had read: "<Label> deleted". */
  name?: string | null;
  /** From `useResourceNotFoundLinks`; Render draws the first as the primary
   *  button and Return home beside it. */
  links: ResourceStateLink[];
  /** The body in place of the default, given the id snippet. */
  body?: (id: ReactNode) => ReactNode;
}

/**
 * Render's `ResourceNotFound`. Without a cached name: "<Label> not found",
 * "We couldn’t find <id>. It may have been deleted, or the link may be
 * incorrect." With one: "<Label> deleted", "<name> (<id>) was deleted." At
 * most two buttons: the first link, then Return home unless it is the first.
 */
export function ResourceNotFound({
  kind,
  id,
  name,
  links,
  body,
}: ResourceNotFoundProps) {
  const { t } = useTranslations();
  const home = useReturnHomeLink();
  const primary = links[0] ?? home;
  const buttons = primary.to === home.to ? [primary] : [primary, home];
  const snippet = <ResourceIdSnippet id={id} />;
  const deleted = Boolean(name);
  const [notFoundKey, deletedKey] = MISSING_TITLE_KEYS[kind];
  return (
    <ResourceStateFrame
      testId={deleted ? "resource-deleted" : "resource-not-found"}
      title={t(deleted ? deletedKey : notFoundKey)}
      body={
        body ? (
          body(snippet)
        ) : deleted ? (
          <TranslatedWithNodes
            translate={(params) => t("common.resourceDeletedBody", params)}
            nodes={{
              name: <span className="text-foreground font-medium">{name}</span>,
              id: snippet,
            }}
          />
        ) : (
          <TranslatedWithNodes
            translate={(params) => t("common.resourceNotFoundBody", params)}
            nodes={{ id: snippet }}
          />
        )
      }
      links={buttons}
    />
  );
}

/**
 * Render's deploy and build not-found body: "We couldn’t find <id> for
 * <parent>. The link may be incorrect."
 */
export function NotFoundForParentBody({
  id,
  parent,
}: {
  id: ReactNode;
  parent: string;
}) {
  const { t } = useTranslations();
  return (
    <TranslatedWithNodes
      translate={(params) => t("common.resourceNotFoundForBody", params)}
      nodes={{
        id,
        parent: <span className="text-foreground font-medium">{parent}</span>,
      }}
    />
  );
}

/**
 * Render's `Error404`: "Page not found", "<id> doesn’t exist.", and one
 * button, Return home unless the page names its own (Go to Webhooks).
 */
export function PageNotFound({
  id,
  action,
}: {
  id: string;
  action?: ResourceStateLink;
}) {
  const { t } = useTranslations();
  const home = useReturnHomeLink();
  return (
    <ResourceStateFrame
      testId="page-not-found"
      title={t("common.notFoundTitle")}
      body={
        <TranslatedWithNodes
          translate={(params) => t("common.resourceDoesNotExist", params)}
          nodes={{ id: <ResourceIdSnippet id={id} /> }}
        />
      }
      links={[action ?? home]}
    />
  );
}

/**
 * Render's `Error403`: "Access denied", "You don’t have access to <id>." (or
 * "…to this resource." without an id), and one button. bex-api sends no join
 * hint, so Render's "It belongs to <workspace>…" paragraph and its Join
 * workspace button have nothing to draw from (w3/158).
 */
export function AccessDenied({
  id,
  action,
}: {
  id?: string;
  action?: ResourceStateLink;
}) {
  const { t } = useTranslations();
  const home = useReturnHomeLink();
  return (
    <ResourceStateFrame
      testId="access-denied"
      title={t("common.accessDeniedTitle")}
      body={
        id ? (
          <TranslatedWithNodes
            translate={(params) => t("common.accessDeniedBody", params)}
            nodes={{ id: <ResourceIdSnippet id={id} /> }}
          />
        ) : (
          t("common.accessDeniedBodyNoId")
        )
      }
      links={[action ?? home]}
    />
  );
}

/**
 * Render's load-error state: "An error occurred", "<id> could not be
 * loaded.", Return home.
 */
export function ResourceLoadFailed({ id }: { id: string }) {
  const { t } = useTranslations();
  const home = useReturnHomeLink();
  return (
    <ResourceStateFrame
      testId="resource-load-failed"
      title={t("common.resourceLoadFailedTitle")}
      body={
        <TranslatedWithNodes
          translate={(params) => t("common.resourceLoadFailedBody", params)}
          nodes={{ id: <ResourceIdSnippet id={id} /> }}
        />
      }
      links={[home]}
    />
  );
}

/**
 * The state a resource page draws for its read (`resourceReadState`): the
 * page's own not-found (Page not found unless the page passes Render's
 * `ResourceNotFound`), Access denied, the expired session's Sign in, or An
 * error occurred. `action` replaces Return home on Page not found and Access
 * denied, as Render's webhook page does with Go to Webhooks.
 */
export function ResourceStateView({
  state,
  id,
  notFound,
  action,
}: {
  state: ResourceReadState;
  id: string;
  notFound?: ReactNode;
  action?: ResourceStateLink;
}) {
  switch (state) {
    case "not-found":
      return notFound ?? <PageNotFound id={id} action={action} />;
    case "forbidden":
      return <AccessDenied id={id} action={action} />;
    case "unauthenticated":
      return <ResourceLoadError variant="unauthenticated" />;
    case "error":
      return <ResourceLoadFailed id={id} />;
  }
}

/**
 * A resource page whose read ended in a state: the dashboard shell and its
 * page scroller, with the state in place of the resource's header and page,
 * as Render draws it inside its shell.
 */
export function ResourceStatePage({ children }: { children: ReactNode }) {
  return (
    <DashboardLayout>
      <PageScroll>{children}</PageScroll>
    </DashboardLayout>
  );
}
