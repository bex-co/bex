import { Eye, Plus } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { EditableFieldRowPreview } from "@/features/services/components/editable-field-row";
import type { en } from "@/i18n";

type Key = keyof typeof en;

/**
 * Route-pending previews of the datastore Overview cards. Each renders the
 * ready card's anatomy with its static translated copy — so titles, help and
 * labels wrap exactly as they will — and inert placeholders wherever the
 * resource decides the content. Nothing here reads a datastore, binds a
 * mutation or reveals a value (w4/298).
 */

const METADATA_COPY = {
  postgres: {
    title: "databases.metaTitle",
    name: "databases.fieldName",
    nameHint: "databases.nameDescription",
    // Representative facts: the always-present rows plus Region. External
    // host appears only for a public instance and is not assumed.
    facts: [
      "databases.metaStatus",
      "databases.metaPlan",
      "databases.metaVersion",
      "databases.metaDatabaseName",
      "databases.metaDatabaseUser",
      "databases.metaStorage",
      "databases.metaHighAvailability",
      "databases.metaPublic",
      "databases.metaRegion",
      "databases.metaCreated",
    ],
  },
  keyvalue: {
    title: "keyvalue.metaTitle",
    name: "keyvalue.fieldName",
    nameHint: "keyvalue.nameDescription",
    facts: [
      "keyvalue.metaId",
      "keyvalue.metaStatus",
      "keyvalue.metaPlan",
      "keyvalue.metaVersion",
      "keyvalue.metaPublic",
      "keyvalue.metaRegion",
      "keyvalue.metaCreated",
    ],
  },
} as const satisfies Record<
  string,
  { title: Key; name: Key; nameHint: Key; facts: readonly Key[] }
>;

type DatastoreKind = keyof typeof METADATA_COPY;

/**
 * The Details card (`MetadataList` with a name-row lead): the Name field's
 * label, hint, disabled input and pencil above a divider, then the facts grid
 * with real labels and 20px value placeholders in the ready `dl` spacing.
 */
export function DatastoreMetadataPreview({ kind }: { kind: DatastoreKind }) {
  const { t } = useTranslations();
  const copy = METADATA_COPY[kind];
  return (
    <Card inert aria-busy="true">
      <CardHeader>
        <CardTitle>{t(copy.title)}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="mb-4 border-b pb-4">
          <EditableFieldRowPreview
            label={t(copy.name)}
            hint={t(copy.nameHint)}
          />
        </div>
        <dl className="grid grid-cols-1 gap-x-6 gap-y-3 sm:grid-cols-2">
          {copy.facts.map((label) => (
            <div
              key={label}
              className="flex justify-between gap-4 border-b pb-2 last:border-0 sm:last:border-b"
            >
              <dt className="text-sm text-muted-foreground">{t(label)}</dt>
              <dd className="flex h-5 min-w-0 items-center text-sm font-medium">
                <Skeleton className="h-3.5 w-20" />
              </dd>
            </div>
          ))}
        </dl>
      </CardContent>
    </Card>
  );
}

const CONNECTION_COPY = {
  postgres: {
    title: "databases.connTitle",
    description: "databases.connDescription",
    reveal: "databases.connReveal",
  },
  keyvalue: {
    title: "keyvalue.connTitle",
    description: "keyvalue.connDescription",
    reveal: "keyvalue.connReveal",
  },
} as const satisfies Record<
  DatastoreKind,
  { title: Key; description: Key; reveal: Key }
>;

/**
 * The masked Connections card: title, the static description and the
 * disabled 36px Reveal button. Connection values are never part of a preview.
 */
export function DatastoreConnectionsPreview({ kind }: { kind: DatastoreKind }) {
  const { t } = useTranslations();
  const copy = CONNECTION_COPY[kind];
  return (
    <Card inert aria-busy="true">
      <CardHeader>
        <CardTitle>{t(copy.title)}</CardTitle>
        <CardDescription>{t(copy.description)}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <Button disabled>
          <Eye />
          {t(copy.reveal)}
        </Button>
      </CardContent>
    </Card>
  );
}

const POLICY_COPY = {
  // The recommended policy's label ("allkeys-lru (recommended for caches)")
  // wraps to two lines on a narrow select; "Journal + Snapshot" fits one.
  maxmemory: {
    title: "keyvalue.maxmemoryTitle",
    description: "keyvalue.maxmemoryDescription",
    label: "keyvalue.maxmemoryLabel",
    hint: "keyvalue.fieldMaxmemoryPolicyHint",
    narrowLines: 2,
  },
  persistence: {
    title: "keyvalue.persistenceTitle",
    description: "keyvalue.persistenceDescription",
    label: "keyvalue.persistenceLabel",
    hint: "keyvalue.fieldPersistenceModeHint",
    narrowLines: 1,
  },
} as const satisfies Record<
  string,
  { title: Key; description: Key; label: Key; hint: Key; narrowLines: 1 | 2 }
>;

/**
 * A Key Value policy card (Maxmemory Policy / Persistence Mode): its title and
 * description, then the select field's label, hint, value slot and pencil
 * slot — the ready section's anatomy with no policy value and no editor
 * (w4/299).
 */
export function KeyValuePolicyPreview({
  policy,
}: {
  policy: keyof typeof POLICY_COPY;
}) {
  const { t } = useTranslations();
  const copy = POLICY_COPY[policy];
  return (
    <Card inert aria-busy="true">
      <CardHeader>
        <CardTitle>{t(copy.title)}</CardTitle>
        <CardDescription>{t(copy.description)}</CardDescription>
      </CardHeader>
      <CardContent>
        <EditableFieldRowPreview
          label={t(copy.label)}
          hint={t(copy.hint)}
          select={{ narrowLines: copy.narrowLines }}
        />
      </CardContent>
    </Card>
  );
}

/**
 * The Key Value Networking card (`KeyValueNetworkingPanel` over
 * `IPAllowListEditor`) before its resource is known: real title, description
 * and allowlist help, the ready add row (two empty disabled inputs, disabled
 * Add and Save) in its flex-wrap, and placeholder lines for the two regions
 * the data decides — the external-access status line and the rule list.
 * Those reserve the private, empty state (the default): one status line and
 * the one-line empty text, each wrapping to two lines below `sm` as their copy
 * does. A public store drops the status line and a store with rules lists
 * them; neither is assumed or rendered here (w4/303).
 */
export function KeyValueNetworkingPreview() {
  const { t } = useTranslations();
  return (
    <Card inert aria-busy="true">
      <CardHeader>
        <CardTitle>{t("keyvalue.networkingTitle")}</CardTitle>
        <CardDescription>{t("keyvalue.networkingDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-2">
        <div data-preview="networking-status" className="text-xs">
          <span className="flex h-4 items-center">
            <Skeleton className="h-3 w-full max-w-md" />
          </span>
          <span className="flex h-4 items-center sm:hidden">
            <Skeleton className="h-3 w-1/2" />
          </span>
        </div>
        <section className="space-y-2">
          <p className="text-xs text-muted-foreground">
            {t("keyvalue.networkingHint")}
          </p>
          {/* Inline like the ready empty text, so the add row follows it
              with no block gap. */}
          <span
            data-preview="networking-rules"
            className="text-sm text-muted-foreground"
          >
            <Skeleton className="inline-block h-3.5 w-full align-middle sm:w-80" />
            <Skeleton className="inline-block h-3.5 w-40 align-middle sm:hidden" />
          </span>
          <div className="flex flex-wrap gap-2">
            <Input
              disabled
              placeholder="203.0.113.0/24"
              aria-label={t("keyvalue.networkingCIDR")}
              className="max-w-xs"
            />
            <Input
              disabled
              aria-label={t("keyvalue.networkingDescriptionLabel")}
              placeholder={t("keyvalue.networkingEntryDescription")}
              className="max-w-xs"
            />
            <Button type="button" variant="outline" size="sm" disabled>
              <Plus />
              {t("keyvalue.networkingAdd")}
            </Button>
            <Button type="button" size="sm" disabled>
              {t("keyvalue.networkingSave")}
            </Button>
          </div>
        </section>
      </CardContent>
    </Card>
  );
}
