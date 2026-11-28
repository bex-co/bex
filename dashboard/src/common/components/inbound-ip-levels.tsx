import { useId, useState } from "react";
import {
  AlertTriangle,
  Ban,
  Building2,
  ChevronRight,
  CircleCheck,
  ExternalLink,
  Info,
  Layers,
} from "lucide-react";
import { ReservedText } from "@/common/components/reserved-skeleton";
import { Badge } from "@/common/components/ui/badge";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Skeleton, SkeletonSpan } from "@/common/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { useInboundIPLevels } from "@/common/hooks/use-inbound-ip-levels";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  checkInboundAddress,
  inboundRangesAllowedByEveryLevel,
  type InboundAddressCheck,
  type InboundIPLevel,
  type IPAllowListEntryDraft,
} from "@/common/lib/ip-allow-list";
import { cn } from "@/common/lib/utils/utils";

/** Render's Network Access page, where the workspace's own rules are edited. */
const WORKSPACE_IP_RULES_HREF = "/network-access";

// The ready elements' classes, shared with the skeleton so the two keep one
// geometry (dashboard/AGENTS.md, a skeleton matches its ready state).
const HEADER_CLASS = "flex flex-wrap items-center gap-x-3 gap-y-1 text-sm";
const LABEL_CLASS = "text-xs font-medium text-muted-foreground";
const LINK_CLASS =
  "inline-flex items-center gap-1 rounded-sm font-medium underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring [&_svg]:size-3.5 [&_svg]:shrink-0";
const STATUS_CLASS = "flex items-center gap-2 text-sm text-muted-foreground";
const FIELD_CLASS = "w-80 max-w-full space-y-1.5";
const SOURCE_HEAD_CLASS = "w-1/3 whitespace-nowrap";
const ROW_CLASS = "hover:bg-transparent";

/**
 * The header of an "Affected by:" section: the label, Render's breadcrumb
 * (Workspace, or Workspace › Environment, the edited level a link) and a View
 * rules link that opens where the level is edited in a new tab. With no
 * `href` it is the skeleton's: the same boxes, their ink reserved.
 */
function AffectedByHeader({
  level,
  href,
  headingId,
}: {
  level: InboundIPLevel;
  href?: string;
  headingId?: string;
}) {
  const { t } = useTranslations();
  const workspace = (
    <>
      <Building2 aria-hidden="true" />
      {t("common.inboundIpLevelWorkspace")}
    </>
  );
  const environment = (
    <>
      <Layers aria-hidden="true" />
      {t("common.inboundIpLevelEnvironment")}
    </>
  );
  const viewRules = (
    <>
      {t("common.inboundIpViewRules")}
      <ExternalLink aria-hidden="true" />
    </>
  );
  if (!href) {
    return (
      <div className={HEADER_CLASS}>
        <span className={cn(LABEL_CLASS, "relative")}>
          <ReservedText>{t("common.inboundIpAffectedBy")}</ReservedText>
        </span>
        {/* Flex boxes as in the ready header, so the icons sit in line. */}
        <span className={cn(LINK_CLASS, "relative")}>
          <span className="invisible inline-flex items-center gap-1">
            {workspace}
            {level === "environment" ? (
              <>
                <ChevronRight aria-hidden="true" />
                {environment}
              </>
            ) : null}
          </span>
          <SkeletonSpan />
        </span>
        <span className={cn(LINK_CLASS, "relative ml-auto")}>
          <span className="invisible inline-flex items-center gap-1">
            {viewRules}
          </span>
          <SkeletonSpan />
        </span>
      </div>
    );
  }
  return (
    <header className={HEADER_CLASS}>
      <h5 id={headingId} className={LABEL_CLASS}>
        {t("common.inboundIpAffectedBy")}
      </h5>
      <ol className="flex min-w-0 items-center gap-1 [&_svg]:size-3.5 [&_svg]:shrink-0">
        <li className="flex items-center gap-1">
          {level === "workspace" ? (
            <a href={href} className={LINK_CLASS}>
              {workspace}
            </a>
          ) : (
            <span className="inline-flex items-center gap-1 text-muted-foreground">
              {workspace}
            </span>
          )}
        </li>
        {level === "environment" ? (
          <li className="flex items-center gap-1">
            <ChevronRight
              aria-hidden="true"
              className="text-muted-foreground"
            />
            <a href={href} className={LINK_CLASS}>
              {environment}
            </a>
          </li>
        ) : null}
      </ol>
      <a
        href={href}
        target="_blank"
        rel="noreferrer"
        className={cn(LINK_CLASS, "ml-auto")}
      >
        {viewRules}
      </a>
    </header>
  );
}

/**
 * Render's "Affected by:" section (w4/m710): one higher level of inbound IP
 * rules that also applies to the resource on this panel, read-only: its
 * Source/Description table, or Render's warning that the level blocks all
 * internet traffic when it has no rule.
 */
function InboundIPAffectedBy({
  level,
  entries,
  href,
}: {
  level: InboundIPLevel;
  entries: ReadonlyArray<IPAllowListEntryDraft>;
  href: string;
}) {
  const { t } = useTranslations();
  const headingId = useId();
  return (
    <section
      aria-labelledby={headingId}
      className="space-y-2"
      data-testid={`inbound-ip-affected-by-${level}`}
    >
      <AffectedByHeader level={level} href={href} headingId={headingId} />
      {entries.length > 0 ? (
        <Table>
          <TableHeader>
            <TableRow className={ROW_CLASS}>
              <TableHead className={SOURCE_HEAD_CLASS}>
                {t("common.inboundIpSource")}
              </TableHead>
              <TableHead>{t("common.inboundIpDescription")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {entries.map((entry) => (
              <TableRow key={entry.cidrBlock} className={ROW_CLASS}>
                <TableCell className="font-mono whitespace-nowrap">
                  {entry.cidrBlock}
                </TableCell>
                <TableCell className="text-muted-foreground">
                  {entry.description}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      ) : (
        <p className="flex items-start gap-2 text-sm text-amber-700 dark:text-amber-400">
          <AlertTriangle
            aria-hidden="true"
            className="mt-0.5 size-4 shrink-0"
          />
          {t(
            level === "workspace"
              ? "common.inboundIpBlockedWorkspace"
              : "common.inboundIpBlockedEnvironment",
          )}
        </p>
      )}
    </section>
  );
}

const CHECK_ICON: Record<
  InboundAddressCheck["state"],
  { icon: typeof Info; tone: string }
> = {
  empty: { icon: Info, tone: "text-sky-600 dark:text-sky-400" },
  invalid: { icon: Ban, tone: "text-destructive" },
  refused: { icon: AlertTriangle, tone: "text-amber-700 dark:text-amber-400" },
  allowed: {
    icon: CircleCheck,
    tone: "text-emerald-700 dark:text-emerald-400",
  },
};

/**
 * Render's Validate IP address field: type an IPv4 address to learn whether
 * every level admits it and, when not, which level refuses it first. Render
 * puts the answer in the icon's tooltip and a screen-reader alert; here it is
 * the line under the field, so a touch screen reads it too.
 */
function ValidateIPAddress({
  check,
  ownType,
  value,
  onChange,
}: {
  check: InboundAddressCheck;
  ownType: string;
  value: string;
  onChange: (value: string) => void;
}) {
  const { t } = useTranslations();
  const inputId = useId();
  const messageId = useId();
  const { icon: Icon, tone } = CHECK_ICON[check.state];
  const message =
    check.state === "empty"
      ? t("common.inboundIpCheckEmpty")
      : check.state === "invalid"
        ? t("common.inboundIpCheckInvalid")
        : check.state === "allowed"
          ? t("common.inboundIpCheckAllowed")
          : check.level === "workspace"
            ? t("common.inboundIpRefusedWorkspace")
            : check.level === "environment"
              ? t("common.inboundIpRefusedEnvironment")
              : t("common.inboundIpRefusedOwn", { type: ownType });
  return (
    <div className={FIELD_CLASS}>
      <Label htmlFor={inputId}>{t("common.inboundIpValidateLabel")}</Label>
      <div className="relative">
        <Input
          id={inputId}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          placeholder="0.0.0.0"
          inputMode="decimal"
          autoComplete="off"
          spellCheck={false}
          aria-describedby={messageId}
          aria-invalid={check.state === "invalid"}
          className="pr-9 font-mono"
        />
        <Icon
          aria-hidden="true"
          data-check={check.state}
          className={cn(
            "pointer-events-none absolute top-1/2 right-2.5 size-4 -translate-y-1/2",
            tone,
          )}
        />
      </div>
      <p
        id={messageId}
        aria-live="polite"
        aria-atomic="true"
        className={cn(
          "text-xs",
          check.state === "empty" ? "text-muted-foreground" : tone,
        )}
      >
        {message}
      </p>
    </div>
  );
}

/** Render's heading over the resource's own rules ("Web Service Inbound IP Rules"). */
function OwnRulesHeading({
  ownType,
  reserved = false,
}: {
  ownType: string;
  reserved?: boolean;
}) {
  const { t } = useTranslations();
  const text = t("common.inboundIpOwnRules", { type: ownType });
  return reserved ? (
    <p className={cn(LABEL_CLASS, "relative w-fit")}>
      <ReservedText>{text}</ReservedText>
    </p>
  ) : (
    <h5 className={LABEL_CLASS}>{text}</h5>
  );
}

/**
 * Render's Networking view above a resource's own inbound IP rules (w4/m710):
 * how many IP ranges get through every level together, the Validate IP
 * address field, and one "Affected by:" section per higher level that also
 * applies. The workspace level applies everywhere this is mounted; the
 * environment level when the resource belongs to one (`environmentId`), and
 * never on an environment's own card, which passes none.
 *
 * The count reads the saved lists, as Render's does; the check reads the
 * editor's unsaved `draft` of the resource's own rules, so an edit changes
 * its answer at once. While a level is still being read nothing is claimed
 * about it: the view keeps its ready geometry as a skeleton.
 */
export function InboundIPLevels({
  environmentId,
  saved,
  draft,
  ownType,
}: {
  environmentId?: string | null;
  saved: ReadonlyArray<IPAllowListEntryDraft>;
  draft: ReadonlyArray<IPAllowListEntryDraft>;
  /** The resource's type as the refusal names it ("Web Service", "PostgreSQL"). */
  ownType: string;
}) {
  const { t } = useTranslations();
  const levels = useInboundIPLevels(environmentId);
  const [address, setAddress] = useState("");

  if (levels.state === "loading") {
    return (
      <InboundIPLevelsSkeleton
        environment={Boolean(environmentId)}
        ownType={ownType}
      />
    );
  }
  if (levels.state === "error") {
    return (
      <p role="alert" className="text-sm text-destructive">
        {t("common.inboundIpLoadError")}
      </p>
    );
  }

  const { workspace, environment } = levels;
  const count = inboundRangesAllowedByEveryLevel([
    workspace,
    environment?.entries,
    saved,
  ]).length;
  const check = checkInboundAddress(address, {
    workspace,
    environment: environment?.entries,
    own: draft,
  });

  return (
    <div className="space-y-6" data-testid="inbound-ip-levels">
      <div className="space-y-4">
        <p role="status" aria-atomic="true" className={STATUS_CLASS}>
          <Badge variant="secondary" className="tabular-nums">
            {count.toLocaleString()}
          </Badge>{" "}
          <span>{t("common.inboundIpAllowedRanges", { count })}</span>
        </p>
        <ValidateIPAddress
          check={check}
          ownType={ownType}
          value={address}
          onChange={setAddress}
        />
      </div>
      <InboundIPAffectedBy
        level="workspace"
        entries={workspace}
        href={WORKSPACE_IP_RULES_HREF}
      />
      {environment ? (
        <InboundIPAffectedBy
          level="environment"
          entries={environment.entries}
          href={environment.href}
        />
      ) : null}
      <OwnRulesHeading ownType={ownType} />
    </div>
  );
}

/**
 * A level's section at its ready geometry for Render's seeded allow-all pair
 * (two rows), the list most workspaces and environments read.
 */
function AffectedBySkeleton({ level }: { level: InboundIPLevel }) {
  const { t } = useTranslations();
  return (
    <div className="space-y-2" data-preview="inbound-ip-affected-by">
      <AffectedByHeader level={level} />
      <Table>
        <TableHeader>
          <TableRow className={ROW_CLASS}>
            <TableHead className={SOURCE_HEAD_CLASS}>
              <span className="relative">
                <ReservedText>{t("common.inboundIpSource")}</ReservedText>
              </span>
            </TableHead>
            <TableHead>
              <span className="relative">
                <ReservedText>{t("common.inboundIpDescription")}</ReservedText>
              </span>
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {[0, 1].map((row) => (
            <TableRow key={row} className={ROW_CLASS}>
              <TableCell>
                <Skeleton className="h-5 w-24" />
              </TableCell>
              <TableCell>
                <Skeleton className="h-5 w-32 max-w-full" />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

/**
 * `InboundIPLevels` before its levels are read, at the ready view's geometry:
 * the count line, the field (a real disabled input, so it takes the touch
 * height the ready one does) with its message line, the workspace section
 * and, in an environment, the environment's, then the own-rules heading.
 * Static copy is reserved with its own translated text, so it wraps as the
 * ready copy does in every language.
 */
export function InboundIPLevelsSkeleton({
  environment,
  ownType,
}: {
  environment: boolean;
  ownType: string;
}) {
  const { t } = useTranslations();
  return (
    <div
      className="space-y-6"
      inert
      aria-hidden="true"
      data-preview="inbound-ip-levels"
    >
      <div className="space-y-4">
        <div className={STATUS_CLASS}>
          <Skeleton className="h-[22px] w-6 shrink-0 rounded-md" />
          <span className="relative">
            <ReservedText>
              {t("common.inboundIpAllowedRanges", { count: 1 })}
            </ReservedText>
          </span>
        </div>
        <div className={FIELD_CLASS}>
          <Label className="relative w-fit">
            <ReservedText>{t("common.inboundIpValidateLabel")}</ReservedText>
          </Label>
          <Input
            disabled
            aria-label={t("common.inboundIpValidateLabel")}
            placeholder="0.0.0.0"
            className="pr-9 font-mono"
          />
          <p className="relative w-fit text-xs">
            <ReservedText>{t("common.inboundIpCheckEmpty")}</ReservedText>
          </p>
        </div>
      </div>
      <AffectedBySkeleton level="workspace" />
      {environment ? <AffectedBySkeleton level="environment" /> : null}
      <OwnRulesHeading ownType={ownType} reserved />
    </div>
  );
}
