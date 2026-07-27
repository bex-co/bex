import { useState } from "react";
import {
  Plus,
  Globe,
  MoreHorizontal,
  Trash2,
  CheckCircle2,
  Clock,
  AlertTriangle,
  ExternalLink,
  ChevronDown,
  RefreshCw,
} from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardAction,
  CardContent,
} from "@/common/components/ui/card";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "@/common/components/ui/table";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Badge } from "@/common/components/ui/badge";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/common/components/ui/alert-dialog";
import { Separator } from "@/common/components/ui/separator";
import { CopyButton } from "@/common/components/copy-button";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useCustomDomains,
  useCustomDomainMutations,
  type AddedDomain,
} from "@/features/services/hooks/use-custom-domains";
import { CenteredState } from "@/features/services/components/centered-state";
import { PlatformSubdomainRow } from "@/features/services/components/platform-subdomain-section";
import {
  pairedSibling,
  type CustomDomainView,
} from "@/features/services/types";

// A hostname bex-api (and the operator's Ingress) will accept: optional wildcard
// label, then dot-separated labels, ending in a 2+ char TLD. Reject bad input
// client-side rather than round-tripping to a 400; the backend stays authoritative.
const VALID_FQDN = /^(\*\.)?([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,}$/i;

// The custom-domains table has four columns (Name, Verified, Certificate, Actions);
// the DNS-instructions detail row spans all of them.
const COLUMN_COUNT = 4;

/**
 * The Custom Domains section of the service Settings tab (Render dashboard shape,
 * captured live 2026-07-09 + DNS instructions w5/m10): a table of domains with
 * Verified/Certificate status, an "Add Custom Domain" dialog, a per-row delete, and
 * — for each domain — the DNS record to create (CNAME/ALIAS → the app's platform
 * host) with copy buttons and a "Re-check" action. All over bex-api's custom-domains
 * GraphQL (docs/ADR006-bex-api.md), a veneer over App.spec.hosts[].
 */
export function CustomDomainsSection({
  serviceId,
  subdomain,
}: {
  serviceId: string;
  /** When set, the platform-subdomain toggle nests at the bottom of this card
   *  (Render folds "Render Subdomain" into Custom Domains, w5/m52). Omitted for
   *  service types without a public platform subdomain. */
  subdomain?: {
    url: string | null;
    renderSubdomainPolicy: string | null | undefined;
  };
}) {
  const { t } = useTranslations();
  const { domains, loading, error, refetch } = useCustomDomains(serviceId);
  const { addDomain, deleteDomain, verifyDomain, busy } =
    useCustomDomainMutations(serviceId, refetch);

  const initialLoading = loading && domains.length === 0 && !error;

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.domainsTitle")}</CardTitle>
        <CardDescription>{t("services.domainsDescription")}</CardDescription>
        <CardAction>
          <AddDomainButton addDomain={addDomain} disabled={busy} />
        </CardAction>
      </CardHeader>
      <CardContent className="space-y-6">
        {error ? (
          <CenteredState
            icon={<AlertTriangle />}
            title={t("services.domainsErrorTitle")}
            body={t("services.domainsErrorBody")}
          />
        ) : initialLoading ? (
          <TableSkeleton />
        ) : domains.length === 0 ? (
          <CenteredState
            icon={<Globe />}
            title={t("services.domainsEmptyTitle")}
            body={t("services.domainsEmptyBody")}
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("services.domainColName")}</TableHead>
                <TableHead>{t("services.domainColVerified")}</TableHead>
                <TableHead>{t("services.domainColCertificate")}</TableHead>
                <TableHead className="sr-only text-right">
                  {t("services.domainColActions")}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {domains.map((domain) => (
                <CustomDomainRow
                  key={domain.name}
                  domain={domain}
                  sibling={pairedSibling(domain, domains)}
                  onDelete={deleteDomain}
                  onVerify={verifyDomain}
                  busy={busy}
                />
              ))}
            </TableBody>
          </Table>
        )}
        {subdomain && (
          <>
            <Separator />
            <PlatformSubdomainRow
              serviceId={serviceId}
              url={subdomain.url}
              renderSubdomainPolicy={subdomain.renderSubdomainPolicy}
            />
          </>
        )}
      </CardContent>
    </Card>
  );
}

/** One domain row: the FQDN as an external link, the two status badges, a DNS-setup
 *  disclosure toggle + kebab menu, and (when expanded) the DNS-instructions panel.
 *  `sibling` is the www<->apex domain bex auto-paired alongside this one (w6/m23,
 *  null if this domain wasn't part of an auto-pair) — surfaced as a small note so a
 *  tenant who only typed one hostname understands where the second row came from. */
function CustomDomainRow({
  domain,
  sibling,
  onDelete,
  onVerify,
  busy,
}: {
  domain: CustomDomainView;
  sibling: CustomDomainView | null;
  onDelete: (name: string) => Promise<boolean>;
  onVerify: (name: string) => Promise<CustomDomainView | null>;
  busy: boolean;
}) {
  const { t } = useTranslations();
  const [confirming, setConfirming] = useState(false);
  // Pending domains still need their DNS record created, so open them by default;
  // a verified domain is done, so it starts collapsed. Toggling overrides this.
  const [open, setOpen] = useState(() => !domain.verified);

  return (
    <>
      <TableRow>
        <TableCell className="font-medium break-all">
          <a
            href={`https://${domain.name}`}
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-1 hover:underline"
          >
            {domain.name}
            <ExternalLink className="text-muted-foreground size-3" />
          </a>
          {domain.redirectForName && (
            <p className="text-muted-foreground text-xs font-normal">
              {t("services.domainRedirectsTo", {
                canonical: domain.redirectForName,
              })}
            </p>
          )}
          {!domain.redirectForName &&
            sibling?.redirectForName === domain.name && (
              <p className="text-muted-foreground text-xs font-normal">
                {t("services.domainRedirectsHere", { sibling: sibling.name })}
              </p>
            )}
        </TableCell>
        <TableCell>
          <StatusBadge
            ok={domain.verified}
            okLabel={t("services.domainVerified")}
            pendingLabel={t("services.domainPending")}
          />
        </TableCell>
        <TableCell>
          <StatusBadge
            ok={domain.active}
            okLabel={t("services.domainCertActive")}
            pendingLabel={t("services.domainPending")}
          />
        </TableCell>
        <TableCell className="text-right whitespace-nowrap">
          <Button
            variant="ghost"
            size="icon-sm"
            aria-expanded={open}
            aria-label={t("services.domainDnsToggle")}
            onClick={() => setOpen((o) => !o)}
          >
            <ChevronDown
              className={`transition-transform ${open ? "rotate-180" : ""}`}
            />
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon-sm"
                disabled={busy}
                aria-label={t("services.domainActionsMenu")}
              >
                <MoreHorizontal />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem
                variant="destructive"
                disabled={busy}
                onSelect={() => setConfirming(true)}
              >
                <Trash2 /> {t("services.domainDelete")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>

          <AlertDialog
            open={confirming}
            onOpenChange={(o) => !o && setConfirming(false)}
          >
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>
                  {t("services.domainDeleteConfirmTitle", {
                    name: domain.name,
                  })}
                </AlertDialogTitle>
                <AlertDialogDescription>
                  {t("services.domainDeleteConfirmBody")}
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>
                  {t("services.domainCancel")}
                </AlertDialogCancel>
                <AlertDialogAction
                  onClick={() => {
                    void onDelete(domain.name);
                    setConfirming(false);
                  }}
                >
                  {t("services.domainDelete")}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </TableCell>
      </TableRow>
      {open && (
        <TableRow className="hover:bg-transparent">
          <TableCell colSpan={COLUMN_COUNT} className="bg-muted/30">
            <DnsInstructions domain={domain} onVerify={onVerify} busy={busy} />
          </TableCell>
        </TableRow>
      )}
    </>
  );
}

/** The apex/subdomain guidance line + the DNS record to create (Type/Host/Target
 *  with copy buttons), or a fallback when the backend couldn't derive the target.
 *  Shared by the per-row panel and the post-add dialog so the two can't drift. */
function DnsRecordFields({ domain }: { domain: CustomDomainView }) {
  const { t } = useTranslations();
  const record = domain.dnsRecord;
  return (
    <div className="space-y-3">
      <p className="text-muted-foreground text-sm">
        {domain.domainType === "apex"
          ? t("services.domainDnsApexGuidance")
          : t("services.domainDnsSubdomainGuidance")}
      </p>
      {record ? (
        <div className="grid gap-2 sm:grid-cols-3">
          <RecordField
            label={t("services.domainRecordType")}
            value={record.type}
          />
          <RecordField
            label={t("services.domainRecordHost")}
            value={record.name}
            copy
          />
          <RecordField
            label={t("services.domainRecordTarget")}
            value={record.value}
            copy
          />
        </div>
      ) : (
        <p className="text-muted-foreground text-sm italic">
          {t("services.domainDnsUnavailable")}
        </p>
      )}
    </div>
  );
}

/** The DNS-instructions panel for a domain row: a titled header with a "Re-check"
 *  action, then the shared record fields. */
function DnsInstructions({
  domain,
  onVerify,
  busy,
}: {
  domain: CustomDomainView;
  onVerify: (name: string) => Promise<CustomDomainView | null>;
  busy: boolean;
}) {
  const { t } = useTranslations();
  return (
    <div className="space-y-3 py-1">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-sm font-medium">{t("services.domainDnsTitle")}</p>
        <Button
          variant="outline"
          size="sm"
          disabled={busy}
          onClick={() => void onVerify(domain.name)}
        >
          <RefreshCw /> {t("services.domainRecheck")}
        </Button>
      </div>
      <DnsRecordFields domain={domain} />
    </div>
  );
}

/** A labeled, monospace DNS-record field, optionally with a copy button. */
function RecordField({
  label,
  value,
  copy,
}: {
  label: string;
  value: string;
  copy?: boolean;
}) {
  const { t } = useTranslations();
  return (
    <div className="space-y-1">
      <div className="flex items-center justify-between gap-2">
        <span className="text-muted-foreground text-xs font-medium">
          {label}
        </span>
        {copy && value ? (
          <CopyButton
            value={value}
            label={label}
            successText={t("services.domainCopied")}
            errorText={t("services.domainCopyError")}
          />
        ) : null}
      </div>
      <div className="bg-background overflow-x-auto rounded-md border px-3 py-2">
        <code className="font-mono text-xs whitespace-pre">{value || "—"}</code>
      </div>
    </div>
  );
}

/** A status pill: a green check when the state is reached, a muted clock while
 *  it's still pending — mirroring Render's Verified / Certificate columns. */
function StatusBadge({
  ok,
  okLabel,
  pendingLabel,
}: {
  ok: boolean;
  okLabel: string;
  pendingLabel: string;
}) {
  if (ok) {
    return (
      <Badge variant="success">
        <CheckCircle2 /> {okLabel}
      </Badge>
    );
  }
  return (
    <Badge variant="outline" className="text-muted-foreground">
      <Clock /> {pendingLabel}
    </Badge>
  );
}

/** The "Add Custom Domain" affordance: a button that opens an FQDN-input dialog,
 *  then — on a successful add — swaps to the DNS record(s) the tenant must
 *  create: just the domain they typed, or both halves of a www<->apex pair
 *  bex auto-added alongside it (w6/m23). */
function AddDomainButton({
  addDomain,
  disabled,
}: {
  addDomain: (name: string) => Promise<AddedDomain | null>;
  disabled: boolean;
}) {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [invalid, setInvalid] = useState(false);
  // The just-added domain (+ auto-paired sibling, if any): on success the dialog
  // swaps the input for the DNS record(s) so the tenant sees exactly what to
  // create without hunting for the new row(s).
  const [added, setAdded] = useState<AddedDomain | null>(null);

  function reset() {
    setName("");
    setInvalid(false);
    setAdded(null);
    setOpen(false);
  }

  async function submit() {
    const trimmed = name.trim();
    if (!VALID_FQDN.test(trimmed)) {
      setInvalid(true);
      return;
    }
    // `disabled` (the hook's busy flag) gates the button while the write is in
    // flight, so no separate saving state is needed.
    const result = await addDomain(trimmed);
    if (result) setAdded(result);
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) reset();
      }}
    >
      <Button
        variant="outline"
        size="sm"
        disabled={disabled}
        onClick={() => setOpen(true)}
      >
        <Plus /> {t("services.domainAdd")}
      </Button>
      <DialogContent>
        {added ? (
          <>
            <DialogHeader>
              <DialogTitle>{t("services.domainAddedTitle")}</DialogTitle>
              <DialogDescription>
                {t("services.domainAddedDescription")}
              </DialogDescription>
            </DialogHeader>
            <DnsRecordFields domain={added.primary} />
            {added.sibling && (
              <>
                <Separator />
                <p className="text-sm font-medium">
                  {t("services.domainPairedDnsTitle", {
                    sibling: added.sibling.name,
                    canonical: added.primary.name,
                  })}
                </p>
                <DnsRecordFields domain={added.sibling} />
              </>
            )}
            <DialogFooter>
              <Button onClick={reset}>{t("services.domainDone")}</Button>
            </DialogFooter>
          </>
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>{t("services.domainAddTitle")}</DialogTitle>
              <DialogDescription>
                {t("services.domainAddDescription")}
              </DialogDescription>
            </DialogHeader>
            <div className="grid gap-2">
              <Label htmlFor="custom-domain-name">
                {t("services.domainColName")}
              </Label>
              <Input
                id="custom-domain-name"
                value={name}
                onChange={(e) => {
                  setName(e.target.value);
                  setInvalid(false);
                }}
                placeholder={t("services.domainPlaceholder")}
                aria-invalid={invalid}
                autoFocus
                onKeyDown={(e) => {
                  if (e.key === "Enter") void submit();
                }}
              />
              {invalid && (
                <p className="text-destructive text-xs">
                  {t("services.domainInvalid")}
                </p>
              )}
            </div>
            <DialogFooter>
              <Button variant="ghost" onClick={reset}>
                {t("services.domainCancel")}
              </Button>
              <Button disabled={disabled} onClick={() => void submit()}>
                {t("services.domainAddButton")}
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}

function TableSkeleton() {
  return (
    <div className="space-y-2">
      {[0, 1].map((i) => (
        <div key={i} className="flex items-center gap-4">
          <Skeleton className="h-6 flex-1" />
          <Skeleton className="h-6 w-24" />
          <Skeleton className="h-6 w-24" />
        </div>
      ))}
    </div>
  );
}
