import { Fragment, useId, type ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import { Info } from "lucide-react";
import { useQuery } from "@apollo/client/react";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { useTranslations } from "@/common/hooks/use-translations";
import { PLACEHOLDER_SENTINEL } from "@/common/lib/i18n-placeholder";
import { cn } from "@/common/lib/utils/utils";
import {
  DeploysDocument,
  WorkspaceOverlappingDeployPolicyDocument,
} from "@/graphql/definitions";
import { useServiceBase } from "@/features/services/lib/service-base";
import type { ServiceView } from "@/features/services/types";
import {
  IN_PROGRESS_DEPLOY_STATUSES,
  RESERVED_PORTS,
  deployNotices,
  isSupersededDeploy,
  reservedPortWarning,
  type DeployNotice,
  type OverlapPolicy,
} from "../lib/deploy-notices";
import type { DeployView } from "../hooks/use-deploy";
import { DeployFailureReason } from "./deploy-failure-reason";

/** bex's pages for the docs Render's Notices link (w1/m659). */
export const TROUBLESHOOT_DOCS_URL =
  "https://bex.co/docs/platform/how-deploys-work#troubleshoot-in-order";
export const HEALTH_CHECK_DOCS_URL =
  "https://bex.co/docs/platform/health-checks";
/** No bex docs page lists the reserved ports; the free plans are priced here. */
export const FREE_PLANS_URL = "https://bex.co/pricing";

const LINK_CLASS =
  "font-medium text-foreground underline underline-offset-4 hover:text-primary";

/** A named slot of a translated sentence, for fill to put a node in. */
const slot = (name: string) =>
  `${PLACEHOLDER_SENTINEL}${name}${PLACEHOLDER_SENTINEL}`;
const SLOT_PATTERN = new RegExp(
  `${PLACEHOLDER_SENTINEL}(\\w+)${PLACEHOLDER_SENTINEL}`,
);

/**
 * translated, a sentence made with slot(name) values, with nodes[name] in
 * each slot wherever the language puts it.
 */
function fill(translated: string, nodes: Record<string, ReactNode>) {
  return translated
    .split(SLOT_PATTERN)
    .map((part, i) => (
      <Fragment key={i}>{i % 2 === 1 ? nodes[part] : part}</Fragment>
    ));
}

function ExternalLink({
  href,
  children,
}: {
  href: string;
  children: ReactNode;
}) {
  return (
    <a href={href} target="_blank" rel="noreferrer" className={LINK_CLASS}>
      {children}
    </a>
  );
}

/**
 * The facts the Notices need beyond the deploy itself: a queued deploy's
 * blocker, found in the service's deploys as Render's page finds it, and the
 * workspace's Overlapping Deploy Policy, read only by a queued deploy or one a
 * newer deploy canceled.
 */
function useNoticeFacts(
  serviceId: string,
  deploy: DeployView | null | undefined,
  ownerId: string,
) {
  const queued = deploy?.status === "queued";
  const superseded = !!deploy && isSupersededDeploy(deploy);
  const { data: deploys } = useQuery(DeploysDocument, {
    variables: { serviceId, status: IN_PROGRESS_DEPLOY_STATUSES, limit: 5 },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    skip: !queued,
  });
  const { data: workspace } = useQuery(
    WorkspaceOverlappingDeployPolicyDocument,
    {
      variables: { id: ownerId },
      errorPolicy: "all",
      skip: !ownerId || !(queued || superseded),
    },
  );
  // The read asks for in-progress deploys only, so the first other one is
  // the deploy this one waits behind.
  const inProgress = deploys?.deploys?.find(
    (d) => !!d?.id && d.id !== deploy?.id,
  );
  const saved = workspace?.workspace?.overlappingDeployPolicy;
  const policy: OverlapPolicy =
    saved === "wait" || saved === "override" ? saved : null;
  return { inProgressDeployId: inProgress?.id ?? null, policy };
}

export interface DeployNoticesProps {
  serviceId: string;
  deploy: DeployView | null | undefined;
  service: ServiceView | null | undefined;
  /** The service's workspace, whose Overlapping Deploy Policy is read. */
  ownerId?: string | null;
}

/**
 * The deploy page's Notices (w1/m659): Render's list under the deploy header
 * that says why the deploy looks the way it does, in Render's order and words,
 * then Render's free-plan reserved-port warning. Nothing renders while the
 * deploy loads or when there is nothing to say, as Render hides an empty
 * list's heading.
 */
export function DeployNotices({
  serviceId,
  deploy,
  service,
  ownerId,
}: DeployNoticesProps) {
  const facts = useNoticeFacts(serviceId, deploy, ownerId ?? "");
  if (!deploy) return null;
  const notices = deployNotices(deploy, {
    healthCheckPath: service?.healthCheckPath,
    port: service?.port,
    ...facts,
  });
  const reserved = reservedPortWarning(deploy, service?.plan);
  if (notices.length === 0 && reserved === null) return null;
  return (
    <div className="space-y-6">
      {notices.length > 0 ? (
        <NoticesList serviceId={serviceId} deploy={deploy} notices={notices} />
      ) : null}
      {reserved !== null ? <ReservedPortWarning port={reserved} /> : null}
    </div>
  );
}

function NoticesList({
  serviceId,
  deploy,
  notices,
}: {
  serviceId: string;
  deploy: DeployView;
  notices: DeployNotice[];
}) {
  const { t } = useTranslations();
  const titleId = useId();
  return (
    <section
      aria-labelledby={titleId}
      className="overflow-hidden rounded-md border bg-card"
    >
      {/* Render's small-caps heading band; CJK has no capitals to carry. */}
      <h2
        id={titleId}
        className="border-b bg-muted/50 px-4 py-2.5 text-[11px] leading-4 font-medium tracking-wider text-muted-foreground uppercase [&:is(:lang(zh),:lang(ja),:lang(ko))]:text-xs [&:is(:lang(zh),:lang(ja),:lang(ko))]:tracking-normal"
      >
        {t("deploys.noticesTitle")}
      </h2>
      <ul className="divide-y">
        {notices.map((notice) => (
          <li
            key={notice.kind}
            className="space-y-1 px-4 py-2.5 text-sm leading-5 break-words text-muted-foreground"
          >
            <NoticeText serviceId={serviceId} deploy={deploy} notice={notice} />
          </li>
        ))}
      </ul>
    </section>
  );
}

function NoticeText({
  serviceId,
  deploy,
  notice,
}: {
  serviceId: string;
  deploy: DeployView;
  notice: DeployNotice;
}) {
  const { t } = useTranslations();
  const base = useServiceBase();
  const deployLink = (deployId: string, label: ReactNode) => (
    <Link
      to={`${base}/$serviceId/deploys/$deployId` as const}
      params={{ serviceId, deployId }}
      className={LINK_CLASS}
    >
      {label}
    </Link>
  );
  // Render's policy sentence follows its notice; here it is a line of its
  // own, so no language has to join two sentences with a period.
  const policy = (
    key: "deploys.noticeCancelPolicy" | "deploys.noticeQueuePolicy",
  ) => (
    <p>
      {fill(t(key, { policy: slot("policy") }), {
        policy: (
          <Link
            to="/workspace/settings"
            hash="overlapping-deploy-policy"
            className={LINK_CLASS}
          >
            {t("deploys.noticePolicyLink")}
          </Link>
        ),
      })}
    </p>
  );

  switch (notice.kind) {
    case "failure":
      return (
        <>
          <DeployFailureReason
            reason={deploy.failureReason}
            template={deploy.failureReasonTemplate}
            className="text-sm leading-5"
          />
          {notice.troubleshoot ? (
            <p>
              {fill(t("deploys.noticeTroubleshoot", { link: slot("link") }), {
                link: (
                  <ExternalLink href={TROUBLESHOOT_DOCS_URL}>
                    {t("deploys.noticeTroubleshootLink")}
                  </ExternalLink>
                ),
              })}
            </p>
          ) : null}
        </>
      );
    case "buildCacheCleared":
      return <p>{t("deploys.noticeBuildCacheCleared")}</p>;
    case "freePlan":
      return <p>{t("deploys.noticeFreePlan")}</p>;
    case "maintenance":
      return <p>{t("deploys.noticeMaintenance")}</p>;
    case "rolledBackTo":
      return (
        <p>
          {fill(t("deploys.noticeRolledBackTo", { deploy: slot("deploy") }), {
            deploy: (
              <span className="font-mono">
                {deployLink(notice.deployId, notice.deployId)}
              </span>
            ),
          })}
        </p>
      );
    case "buildGone":
      return <p>{t("deploys.noticeBuildGone")}</p>;
    case "syncedFrom":
      return (
        <p>
          {fill(
            t("deploys.noticeSyncedFrom", { blueprint: slot("blueprint") }),
            {
              blueprint: (
                <Link
                  to="/blueprints/$blueprintId"
                  params={{ blueprintId: notice.blueprintId }}
                  className={cn(LINK_CLASS, "font-mono")}
                >
                  {notice.blueprintId}
                </Link>
              ),
            },
          )}
        </p>
      );
    case "cancel":
      return (
        <>
          <DeployFailureReason
            reason={deploy.cancelReason}
            template={deploy.cancelReasonTemplate}
            tone="neutral"
            className="text-sm leading-5"
          />
          {notice.policyLink ? policy("deploys.noticeCancelPolicy") : null}
        </>
      );
    case "queued":
      return (
        <>
          <p>
            {fill(t("deploys.noticeInProgress", { deploy: slot("deploy") }), {
              deploy: deployLink(
                notice.deployId,
                t("deploys.noticeAnotherDeploy"),
              ),
            })}
          </p>
          {notice.policyLink ? policy("deploys.noticeQueuePolicy") : null}
        </>
      );
    case "healthCheck": {
      const key = notice.timedOut
        ? notice.address
          ? "deploys.noticeHealthTimedOutAt"
          : "deploys.noticeHealthTimedOut"
        : notice.address
          ? "deploys.noticeHealthWaitingAt"
          : "deploys.noticeHealthWaiting";
      return (
        <p>
          {fill(t(key, { link: slot("link"), address: slot("address") }), {
            link: (
              <ExternalLink href={HEALTH_CHECK_DOCS_URL}>
                {t("deploys.noticeHealthLink")}
              </ExternalLink>
            ),
            address: (
              <code className="rounded bg-muted px-1 py-0.5 font-mono text-xs break-all text-foreground">
                {notice.address}
              </code>
            ),
          })}
        </p>
      );
    }
    case "stall":
      return (
        <DeployFailureReason
          reason={deploy.stallReason}
          template={deploy.stallReasonTemplate}
          tone="neutral"
          className="text-sm leading-5"
        />
      );
  }
}

/**
 * Render's free-plan banner for an instance serving HTTP on a reserved port
 * (w1/m659), between the Notices and the log.
 */
function ReservedPortWarning({ port }: { port: number }) {
  const { t } = useTranslations();
  return (
    <Alert role="note" data-testid="deploy-reserved-port-warning">
      <Info aria-hidden />
      <AlertDescription className="text-foreground">
        <p>
          {fill(
            t("deploys.reservedPortWarning", {
              port: String(port),
              link: slot("link"),
              ports: RESERVED_PORTS.join(", "),
            }),
            {
              link: (
                <ExternalLink href={FREE_PLANS_URL}>
                  {t("deploys.reservedPortWarningLink")}
                </ExternalLink>
              ),
            },
          )}
        </p>
      </AlertDescription>
    </Alert>
  );
}
