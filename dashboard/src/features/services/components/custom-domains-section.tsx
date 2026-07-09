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
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useCustomDomains,
  useCustomDomainMutations,
} from "@/features/services/hooks/use-custom-domains";
import { CenteredState } from "@/features/services/components/centered-state";
import type { CustomDomainView } from "@/features/services/types";

// A hostname bex-api (and the operator's Ingress) will accept: optional wildcard
// label, then dot-separated labels, ending in a 2+ char TLD. Reject bad input
// client-side rather than round-tripping to a 400; the backend stays authoritative.
const VALID_FQDN = /^(\*\.)?([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,}$/i;

/**
 * The Custom Domains section of the service Settings tab (Render dashboard shape,
 * captured live 2026-07-09): a table of domains with Verified/Certificate status,
 * an "Add Custom Domain" dialog, and a per-row delete — all over bex-api's
 * custom-domains GraphQL (docs/bex-api.md), which is a veneer over App.spec.hosts[].
 */
export function CustomDomainsSection({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();
  const { domains, loading, error, refetch } = useCustomDomains(serviceId);
  const { addDomain, deleteDomain, busy } = useCustomDomainMutations(
    serviceId,
    refetch,
  );

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
      <CardContent>
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
                  onDelete={deleteDomain}
                  busy={busy}
                />
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

/** One domain row: the FQDN as an external link, the two status badges, and a
 *  kebab menu whose Delete opens a confirmation. */
function CustomDomainRow({
  domain,
  onDelete,
  busy,
}: {
  domain: CustomDomainView;
  onDelete: (name: string) => Promise<boolean>;
  busy: boolean;
}) {
  const { t } = useTranslations();
  const [confirming, setConfirming] = useState(false);

  return (
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
          onOpenChange={(open) => !open && setConfirming(false)}
        >
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>
                {t("services.domainDeleteConfirmTitle", { name: domain.name })}
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

/** The "Add Custom Domain" affordance: a button that opens an FQDN-input dialog. */
function AddDomainButton({
  addDomain,
  disabled,
}: {
  addDomain: (name: string) => Promise<boolean>;
  disabled: boolean;
}) {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [invalid, setInvalid] = useState(false);

  function reset() {
    setName("");
    setInvalid(false);
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
    const ok = await addDomain(trimmed);
    if (ok) reset();
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
