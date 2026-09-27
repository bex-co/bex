import {
  useCallback,
  useState,
  type Dispatch,
  type SetStateAction,
} from "react";
import { Link } from "@tanstack/react-router";
import { Plus, Trash2, ArrowRightLeft, Tags } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { useTranslations } from "@/common/hooks/use-translations";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";
import { useServiceBase } from "@/features/services/lib/service-base";
import { useStaticSiteMutations } from "@/features/services/hooks/use-static-site";
import type { StaticRuleSaveResult } from "@/features/services/hooks/use-static-site";
import { rootDirPrefix } from "@/features/services/lib/format";
import {
  headerErrors,
  routeErrors,
  type RuleFieldErrors,
} from "@/features/services/lib/static-rule-validation";
import type { en } from "@/i18n";
import type {
  ServiceView,
  StaticRouteView,
  StaticHeaderView,
} from "@/features/services/types";

/**
 * Saves the current draft and adopts the rows the server actually accepted.
 *
 * The backend normalizes what it stores — a padded `" /qa/* "` path saves
 * successfully as `/qa/*` — so an editor that keeps displaying its submitted
 * draft stays permanently dirty against the refetched props: Save enabled and
 * Cancel present, for a change that already persisted (w4/136).
 *
 * The later-draft guard is the reason this is not an unconditional reset. Rows
 * stay editable while the save is in flight, so a user can type during the
 * request; reference equality answers "is the current draft still the one I
 * submitted" exactly, because every edit produces a new array.
 *
 * An `ok` result with no `saved` rows means the post-save read did not produce
 * this service. The draft is left alone rather than emptied.
 */
function useAdoptAcceptedRows<T>(
  draft: T[],
  setDraft: Dispatch<SetStateAction<T[]>>,
  onSave: (rows: T[]) => Promise<StaticRuleSaveResult<T>>,
) {
  return useCallback(async () => {
    const submitted = draft;
    const result = await onSave(submitted);
    if (!result.ok || !result.saved) return;
    const accepted = result.saved;
    setDraft((current) => (current === submitted ? accepted : current));
  }, [draft, onSave, setDraft]);
}

/**
 * The Static Site section of the service Settings tab (w1/m21): the published
 * output directory (publishPath), over bex-api's static-site GraphQL
 * (docs/ADR029-static-sites.md). The two edge-rule editors — redirects/rewrites
 * (routes) and custom response headers — moved to their own Manage pages
 * (w5/m48/t002, Render's sidebar placement); this card links to them. A
 * publishPath change republishes. Rendered only when the service is a static_site.
 */
export function StaticSiteSection({
  serviceId,
  service,
  refetch,
}: {
  serviceId: string;
  service: ServiceView;
  refetch: () => Promise<ServiceView[]>;
}) {
  const { t } = useTranslations();
  const base = useServiceBase();
  const { setPublishPath, busy } = useStaticSiteMutations(serviceId, refetch);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.staticTitle")}</CardTitle>
        <CardDescription>{t("services.staticDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-8">
        <PublishPathRow
          publishPath={service.publishPath}
          rootDir={service.rootDir}
          onSave={setPublishPath}
          busy={busy}
        />
        <div className="text-muted-foreground flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
          <span>{t("services.staticEdgeRulesHint")}</span>
          <Link
            to={`${base}/$serviceId/redirects`}
            params={{ serviceId }}
            className="text-foreground inline-flex items-center gap-1 hover:underline"
          >
            <ArrowRightLeft className="size-3.5" />
            {t("services.navRedirects")}
          </Link>
          <Link
            to={`${base}/$serviceId/headers`}
            params={{ serviceId }}
            className="text-foreground inline-flex items-center gap-1 hover:underline"
          >
            <Tags className="size-3.5" />
            {t("services.navHeaders")}
          </Link>
        </div>
      </CardContent>
    </Card>
  );
}

/** The publish-directory row: shows the current value, with an inline edit that
 *  republishes on save (mirrors BuildDeploySection's Root Directory row). When a
 *  Root Directory is set, the value carries Render's `<rootDir>/` prefix
 *  affordance (w5/m48/t004) — the path is resolved from there, not the repo root. */
function PublishPathRow({
  publishPath,
  rootDir,
  onSave,
  busy,
}: {
  publishPath: string | null;
  rootDir?: string | null;
  onSave: (v: string) => Promise<boolean>;
  busy: boolean;
}) {
  const { t } = useTranslations();
  const current = publishPath ?? "";
  const prefix = rootDirPrefix(rootDir);

  return (
    <EditableFieldRow
      label={t("services.publishPathLabel")}
      hint={t("services.publishPathHint")}
      value={current}
      placeholder={t("services.publishPathPlaceholder")}
      editLabel={t("services.publishPathEdit")}
      valuePrefix={prefix || undefined}
      mono
      busy={busy}
      // A publish directory is required, so an empty value is never savable.
      dirty={(value) => value !== "" && value !== current}
      onSave={onSave}
    />
  );
}

/** The redirects/rewrites editor: an editable list of {type, source, destination}
 *  rows with add/remove, saved as a bulk replace (Render's /routes). Rendered by
 *  the dedicated Redirects/Rewrites page (w5/m48/t002). */
export function RoutesEditor({
  routes,
  onSave,
  busy,
}: {
  routes: StaticRouteView[];
  onSave: (
    r: StaticRouteView[],
  ) => Promise<StaticRuleSaveResult<StaticRouteView>>;
  busy: boolean;
}) {
  const { t } = useTranslations();
  const [draft, setDraft] = useState<StaticRouteView[]>(routes);
  const dirty = JSON.stringify(draft) !== JSON.stringify(routes);
  const save = useAdoptAcceptedRows(draft, setDraft, onSave);
  const errors = draft.map(routeErrors);
  const blocked = rulesBlocked(
    errors,
    draft,
    (r) => !!(r.source || r.destination),
  );

  function update(i: number, patch: Partial<StaticRouteView>) {
    setDraft((d) => d.map((r, j) => (j === i ? { ...r, ...patch } : r)));
  }
  function addRow() {
    setDraft((d) => [...d, { type: "rewrite", source: "", destination: "" }]);
  }
  function removeRow(i: number) {
    setDraft((d) => d.filter((_, j) => j !== i));
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between gap-2">
        <div>
          <p className="text-sm font-medium">{t("services.routesTitle")}</p>
          <p className="text-muted-foreground text-xs">
            {t("services.routesHint")}
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={addRow}>
          <Plus /> {t("services.routeAdd")}
        </Button>
      </div>
      {draft.length > 0 && (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-32">{t("services.routeType")}</TableHead>
              <TableHead>{t("services.routeSource")}</TableHead>
              <TableHead>{t("services.routeDestination")}</TableHead>
              <TableHead className="w-10" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {draft.map((r, i) => (
              <TableRow key={i}>
                <TableCell>
                  <Select
                    value={r.type}
                    onValueChange={(v) => update(i, { type: v })}
                  >
                    {/* The <th>s label cells, not the controls inside them, so
                        each control carries the column heading as its own
                        accessible name; placeholders stay literal examples
                        (path syntax / header literals, untranslated). */}
                    <SelectTrigger
                      size="sm"
                      aria-label={t("services.routeType")}
                    >
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="rewrite">
                        {t("services.routeRewrite")}
                      </SelectItem>
                      <SelectItem value="redirect">
                        {t("services.routeRedirect")}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </TableCell>
                <TableCell className="align-top">
                  <RuleInput
                    id={`route-${i}-source`}
                    value={r.source}
                    onChange={(source) => update(i, { source })}
                    label={t("services.routeSource")}
                    placeholder="/old/*"
                    error={
                      r.source || r.destination ? errors[i].source : undefined
                    }
                  />
                </TableCell>
                <TableCell className="align-top">
                  <RuleInput
                    id={`route-${i}-destination`}
                    value={r.destination}
                    onChange={(destination) => update(i, { destination })}
                    label={t("services.routeDestination")}
                    placeholder="/index.html"
                    error={
                      r.source || r.destination
                        ? errors[i].destination
                        : undefined
                    }
                  />
                </TableCell>
                <TableCell>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={t("services.routeRemove")}
                    onClick={() => removeRow(i)}
                  >
                    <Trash2 />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
      <div className="flex justify-end gap-2">
        {dirty && (
          <Button variant="ghost" onClick={() => setDraft(routes)}>
            {t("services.staticCancel")}
          </Button>
        )}
        <SaveBlockedNote blocked={dirty ? blocked : null} />
        <Button
          disabled={busy || !dirty || blocked !== null}
          onClick={() => void save()}
        >
          {t("services.routesSave")}
        </Button>
      </div>
    </div>
  );
}

/** The custom-headers editor: an editable list of {path, name, value} rows with
 *  add/remove, saved as a bulk replace (Render's /headers). Rendered by the
 *  dedicated Headers page (w5/m48/t002). */
export function HeadersEditor({
  headers,
  onSave,
  busy,
}: {
  headers: StaticHeaderView[];
  onSave: (
    h: StaticHeaderView[],
  ) => Promise<StaticRuleSaveResult<StaticHeaderView>>;
  busy: boolean;
}) {
  const { t } = useTranslations();
  const [draft, setDraft] = useState<StaticHeaderView[]>(headers);
  const dirty = JSON.stringify(draft) !== JSON.stringify(headers);
  const save = useAdoptAcceptedRows(draft, setDraft, onSave);
  const errors = draft.map(headerErrors);
  // A new header row starts with the "/*" path filled in, so it counts as
  // begun once a name or value is typed.
  const started = (h: StaticHeaderView) => !!(h.name || h.value);
  const blocked = rulesBlocked(errors, draft, started);

  function update(i: number, patch: Partial<StaticHeaderView>) {
    setDraft((d) => d.map((h, j) => (j === i ? { ...h, ...patch } : h)));
  }
  function addRow() {
    setDraft((d) => [...d, { path: "/*", name: "", value: "" }]);
  }
  function removeRow(i: number) {
    setDraft((d) => d.filter((_, j) => j !== i));
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between gap-2">
        <div>
          <p className="text-sm font-medium">{t("services.headersTitle")}</p>
          <p className="text-muted-foreground text-xs">
            {t("services.headersHint")}
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={addRow}>
          <Plus /> {t("services.headerAdd")}
        </Button>
      </div>
      {draft.length > 0 && (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-40">{t("services.headerPath")}</TableHead>
              <TableHead>{t("services.headerName")}</TableHead>
              <TableHead>{t("services.headerValue")}</TableHead>
              <TableHead className="w-10" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {draft.map((h, i) => (
              <TableRow key={i}>
                <TableCell className="align-top">
                  {/* Same pattern as RoutesEditor: the column heading doubles
                      as each control's accessible name. */}
                  <RuleInput
                    id={`header-${i}-path`}
                    value={h.path}
                    onChange={(path) => update(i, { path })}
                    label={t("services.headerPath")}
                    placeholder="/*"
                    error={errors[i].path}
                  />
                </TableCell>
                <TableCell className="align-top">
                  <RuleInput
                    id={`header-${i}-name`}
                    value={h.name}
                    onChange={(name) => update(i, { name })}
                    label={t("services.headerName")}
                    placeholder="X-Frame-Options"
                    error={started(h) ? errors[i].name : undefined}
                  />
                </TableCell>
                <TableCell className="align-top">
                  <Input
                    value={h.value}
                    onChange={(e) => update(i, { value: e.target.value })}
                    aria-label={t("services.headerValue")}
                    placeholder="DENY"
                    className="font-mono text-xs"
                  />
                </TableCell>
                <TableCell>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={t("services.headerRemove")}
                    onClick={() => removeRow(i)}
                  >
                    <Trash2 />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
      <div className="flex justify-end gap-2">
        {dirty && (
          <Button variant="ghost" onClick={() => setDraft(headers)}>
            {t("services.staticCancel")}
          </Button>
        )}
        <SaveBlockedNote blocked={dirty ? blocked : null} />
        <Button
          disabled={busy || !dirty || blocked !== null}
          onClick={() => void save()}
        >
          {t("services.headersSave")}
        </Button>
      </div>
    </div>
  );
}

/**
 * Why a rules list can't be saved yet: a started row with a marked field, or a
 * row left completely empty. Null when every row is valid (w4/145).
 */
function rulesBlocked<T>(
  errors: RuleFieldErrors<T>[],
  rows: T[],
  started: (row: T) => boolean,
): "invalid" | "empty" | null {
  const bad = errors.flatMap((e, i) =>
    Object.keys(e).length > 0 ? [rows[i]] : [],
  );
  if (bad.length === 0) return null;
  return bad.some(started) ? "invalid" : "empty";
}

function SaveBlockedNote({ blocked }: { blocked: "invalid" | "empty" | null }) {
  const { t } = useTranslations();
  if (!blocked) return null;
  return (
    <p className="text-muted-foreground self-center text-xs">
      {blocked === "invalid"
        ? t("services.staticRuleFixRows")
        : t("services.staticRuleEmptyRow")}
    </p>
  );
}

/** A rule field: the column heading as its accessible name, and an inline,
 *  announced error under it when the value breaks the server's rule. */
function RuleInput({
  id,
  value,
  onChange,
  label,
  placeholder,
  error,
}: {
  id: string;
  value: string;
  onChange: (value: string) => void;
  label: string;
  placeholder: string;
  error: keyof typeof en | undefined;
}) {
  const { t } = useTranslations();
  return (
    <div className="space-y-1">
      <Input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        aria-label={label}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? `${id}-error` : undefined}
        placeholder={placeholder}
        className="font-mono text-xs"
      />
      {error ? (
        <p id={`${id}-error`} className="text-destructive text-xs">
          {t(error)}
        </p>
      ) : null}
    </div>
  );
}
