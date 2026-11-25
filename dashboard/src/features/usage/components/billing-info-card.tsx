// Copyright 2026 Tian Pan
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import { memo, useCallback, useId, useMemo, useState } from "react";
import { Button } from "@/common/components/ui/button";
import { Card } from "@/common/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { Label } from "@/common/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { TextField } from "@/common/components/text-field";
import { useTranslations } from "@/common/hooks/use-translations";
import { dashboardLanguage } from "@/common/lib/format/language";
import { LANGUAGE_TAGS } from "@/i18n/config";
import { useCapabilityGate } from "@/features/capabilities/hooks/use-capability-gate";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  useBillingInfo,
  type BillingInfo,
  type BillingInfoField,
  type BillingInfoInput,
  type SaveBillingInfoResult,
} from "../hooks/use-billing-info";
import { countryOptions, type CountryOption } from "../lib/countries";
import {
  addressLines,
  billingInfoPayload,
  type BillingInfoDraft,
} from "../lib/billing-info-form";
import {
  taxIdExample,
  taxIdTypeLabel,
  taxIdTypesFor,
} from "../lib/tax-id-types";
import {
  BillingInfoCardBody,
  BillingInfoCardHeader,
  BillingInfoCardSkeleton,
  BillingInfoDetails,
} from "./billing-info-card-skeleton";

/**
 * The workspace's billing info (w6/m628): the company name, address and VAT
 * or tax ID every invoice carries, as Render's Billing page lets a workspace
 * "add additional billing info to be displayed on all invoices". Every member
 * who may view billing details reads it (Admin, Developer, Billing; w9/m620);
 * only Admin and Billing change it, so a Developer's action is held with the
 * role's reason. A workspace that never added a payment method has no billing
 * account to keep it on yet, and says so.
 */
export function BillingInfoCard() {
  const { currentWorkspaceId } = useWorkspace();
  return <BillingInfoCardView workspaceId={currentWorkspaceId ?? ""} />;
}

export function BillingInfoCardView({ workspaceId }: { workspaceId: string }) {
  const { t, i18n } = useTranslations();
  const locale = LANGUAGE_TAGS[dashboardLanguage(i18n.language)];
  const { billingInfo, loading, saving, save } = useBillingInfo(workspaceId);
  const { denied: manageDenied, reason: manageReason } =
    useCapabilityGate("canManageBilling");
  const [open, setOpen] = useState(false);

  if (!billingInfo && (loading || !workspaceId)) {
    return <BillingInfoCardSkeleton />;
  }

  if (!billingInfo) {
    // A failed read must not pass for billing info, nor for its absence.
    return (
      <Card>
        <BillingInfoCardHeader />
        <BillingInfoCardBody
          details={
            <p className="text-sm text-destructive">
              {t("usage.billingInfoUnreadable")}
            </p>
          }
          note={null}
          action={null}
        />
      </Card>
    );
  }

  const notSet = (
    <span className="text-muted-foreground">
      {t("usage.billingInfoNotSet")}
    </span>
  );
  const { address, taxId } = billingInfo;
  const empty = !billingInfo.companyName && !address && !taxId;
  const notReady = !billingInfo.customerReady;
  const notReadyNote = t("usage.billingInfoCustomerNotReady");

  return (
    <Card>
      <BillingInfoCardHeader />
      <BillingInfoCardBody
        details={
          <BillingInfoDetails
            billTo={
              billingInfo.billTo ? (
                <span className="font-medium">{billingInfo.billTo}</span>
              ) : (
                notSet
              )
            }
            companyName={billingInfo.companyName || notSet}
            address={
              address
                ? addressLines(address, locale).map((line, i) => (
                    <span key={i}>{line}</span>
                  ))
                : notSet
            }
            taxId={
              taxId ? (
                <span>
                  <span className="text-muted-foreground">
                    {taxIdTypeLabel(taxId.type)}
                  </span>{" "}
                  {taxId.value}
                </span>
              ) : (
                notSet
              )
            }
          />
        }
        note={notReady ? notReadyNote : null}
        action={
          <PermissionTooltip
            reason={manageReason ?? (notReady ? notReadyNote : undefined)}
          >
            <Button
              variant="outline"
              size="sm"
              disabled={saving || manageDenied || notReady}
              onClick={() => setOpen(true)}
            >
              {empty ? t("usage.billingInfoAdd") : t("usage.billingInfoEdit")}
            </Button>
          </PermissionTooltip>
        }
      />
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("usage.billingInfoDialogTitle")}</DialogTitle>
            <DialogDescription>
              {t("usage.billingInfoDialogDescription")}
            </DialogDescription>
          </DialogHeader>
          {/* Mounted with the dialog, so each opening starts from what is
              stored. */}
          <BillingInfoForm
            info={billingInfo}
            locale={locale}
            saving={saving}
            save={save}
            onClose={() => setOpen(false)}
          />
        </DialogContent>
      </Dialog>
    </Card>
  );
}

// Radix Select takes no "" item, so the empty choice has a value of its own
// (country codes have two letters).
const NO_COUNTRY = "none";

type FieldErrors = Partial<Record<BillingInfoField, string>>;

type TextKey = Exclude<keyof BillingInfoDraft, "country" | "taxType">;

// The address lines, in the order the form shows them.
const ADDRESS_FIELDS: {
  key: TextKey;
  field: BillingInfoField;
  label: string;
  autoComplete: string;
}[] = [
  {
    key: "line1",
    field: "address.line1",
    label: "usage.billingInfoLine1",
    autoComplete: "address-line1",
  },
  {
    key: "line2",
    field: "address.line2",
    label: "usage.billingInfoLine2",
    autoComplete: "address-line2",
  },
  {
    key: "city",
    field: "address.city",
    label: "usage.billingInfoCity",
    autoComplete: "address-level2",
  },
  {
    key: "state",
    field: "address.state",
    label: "usage.billingInfoState",
    autoComplete: "address-level1",
  },
  {
    key: "postalCode",
    field: "address.postalCode",
    label: "usage.billingInfoPostalCode",
    autoComplete: "postal-code",
  },
];

function draftOf(info: BillingInfo): BillingInfoDraft {
  const address = info.address;
  return {
    companyName: info.companyName,
    line1: address?.line1 ?? "",
    line2: address?.line2 ?? "",
    city: address?.city ?? "",
    state: address?.state ?? "",
    postalCode: address?.postalCode ?? "",
    country: address?.country ?? "",
    taxType:
      info.taxId?.type ||
      (taxIdTypesFor(address?.country ?? "")[0]?.type ?? ""),
    taxValue: info.taxId?.value ?? "",
  };
}

function BillingInfoForm({
  info,
  locale,
  saving,
  save,
  onClose,
}: {
  info: BillingInfo;
  locale: string;
  saving: boolean;
  save: (input: BillingInfoInput) => Promise<SaveBillingInfoResult>;
  onClose: () => void;
}) {
  const { t } = useTranslations();
  const id = useId();
  const storedCountry = info.address?.country ?? "";
  const storedType = info.taxId?.type ?? "";
  const [draft, setDraft] = useState(() => draftOf(info));
  const [errors, setErrors] = useState<FieldErrors>({});

  const countries = useMemo(
    () => countryOptions(locale, storedCountry),
    [locale, storedCountry],
  );
  // The chosen country's types, plus a stored type it lacks (one set in the
  // Stripe portal, or under another country), which stays choosable rather
  // than silently dropped. None: the form shows no tax ID field.
  const taxTypes = useMemo(() => {
    const own = taxIdTypesFor(draft.country).map((option) => option.type);
    return storedType && !own.includes(storedType) ? [...own, storedType] : own;
  }, [draft.country, storedType]);
  const effectiveTaxType = taxTypes.includes(draft.taxType)
    ? draft.taxType
    : (taxTypes[0] ?? "");
  const typePicker = taxTypes.length > 1;

  // Editing a field drops the refusal shown under it.
  const clearError = useCallback((field: BillingInfoField) => {
    setErrors((current) => {
      if (!(field in current)) return current;
      const next = { ...current };
      delete next[field];
      return next;
    });
  }, []);

  function edit(key: TextKey | "taxType", field: BillingInfoField) {
    return (value: string) => {
      setDraft((current) => ({ ...current, [key]: value }));
      clearError(field);
    };
  }

  // Stable, so the memoized country list does not re-render on every
  // keystroke elsewhere in the form.
  const chooseCountry = useCallback(
    (code: string) => {
      setDraft((current) => {
        // A new country preselects its own default type, unless the type in
        // hand is one of its own.
        const own = taxIdTypesFor(code);
        const keepType =
          own.length === 0 ||
          own.some((option) => option.type === current.taxType);
        return {
          ...current,
          country: code,
          taxType: keepType ? current.taxType : own[0].type,
        };
      });
      clearError("address.country");
    },
    [clearError],
  );

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    if (saving) return;
    const payload = billingInfoPayload({ ...draft, taxType: effectiveTaxType });
    if ("missing" in payload) {
      setErrors({ [payload.missing]: t("usage.billingInfoCountryRequired") });
      return;
    }
    const result = await save(payload.input);
    if (result.ok) {
      onClose();
    } else if (result.fieldError) {
      // A single-type tax ID has no type picker: its value field shows a
      // refused type.
      const field =
        result.fieldError.field === "taxId.type" && !typePicker
          ? "taxId.value"
          : result.fieldError.field;
      setErrors({ [field]: result.fieldError.message });
    }
  }

  const controlId = (field: BillingInfoField) =>
    `${id}-${field.replace(".", "-")}`;
  const textField = (
    key: TextKey,
    field: BillingInfoField,
    label: string,
    extra: { autoComplete?: string; hint?: string; placeholder?: string },
  ) => (
    <TextField
      id={controlId(field)}
      label={label}
      value={draft[key]}
      onChange={edit(key, field)}
      error={errors[field]}
      {...extra}
    />
  );

  return (
    <form className="space-y-4" onSubmit={(e) => void submit(e)} noValidate>
      {textField(
        "companyName",
        "companyName",
        t("usage.billingInfoCompanyName"),
        {
          autoComplete: "organization",
          hint: t("usage.billingInfoCompanyNameHint"),
        },
      )}
      <SelectRow
        controlId={controlId("address.country")}
        label={t("usage.billingInfoCountry")}
        error={errors["address.country"]}
      >
        {(control) => (
          <CountrySelect
            control={control}
            value={draft.country}
            countries={countries}
            onChange={chooseCountry}
            placeholder={t("usage.billingInfoCountryPlaceholder")}
            noCountry={t("usage.billingInfoNoCountry")}
          />
        )}
      </SelectRow>
      {ADDRESS_FIELDS.slice(0, 2).map((f) =>
        textField(f.key, f.field, t(f.label), { autoComplete: f.autoComplete }),
      )}
      <div className="grid gap-4 sm:grid-cols-2">
        {ADDRESS_FIELDS.slice(2).map((f) =>
          textField(f.key, f.field, t(f.label), {
            autoComplete: f.autoComplete,
          }),
        )}
      </div>
      {taxTypes.length > 0 ? (
        <div
          className={
            typePicker
              ? "grid gap-4 sm:grid-cols-[minmax(0,11rem)_minmax(0,1fr)]"
              : undefined
          }
        >
          {typePicker ? (
            <SelectRow
              controlId={controlId("taxId.type")}
              label={t("usage.billingInfoTaxIdType")}
              error={errors["taxId.type"]}
            >
              {(control) => (
                <Select
                  value={effectiveTaxType}
                  onValueChange={edit("taxType", "taxId.type")}
                >
                  <SelectTrigger {...control} className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {taxTypes.map((type) => (
                      <SelectItem key={type} value={type}>
                        {taxIdTypeLabel(type)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            </SelectRow>
          ) : null}
          {textField(
            "taxValue",
            "taxId.value",
            typePicker
              ? t("usage.billingInfoTaxId")
              : taxIdTypeLabel(effectiveTaxType),
            { placeholder: taxIdExample(effectiveTaxType, draft.country) },
          )}
        </div>
      ) : null}
      <DialogFooter>
        <Button type="button" variant="outline" onClick={onClose}>
          {t("common.cancel")}
        </Button>
        <Button type="submit" disabled={saving} loading={saving}>
          {t("usage.billingInfoSave")}
        </Button>
      </DialogFooter>
    </form>
  );
}

interface SelectControl {
  id: string;
  "aria-invalid": true | undefined;
  "aria-describedby": string | undefined;
}

/** A labelled select with its refusal under it, as TextField lays one out. */
function SelectRow({
  controlId,
  label,
  error,
  children,
}: {
  controlId: string;
  label: string;
  error: string | undefined;
  children: (control: SelectControl) => React.ReactNode;
}) {
  const errorId = `${controlId}-error`;
  return (
    <div className="space-y-2">
      <Label htmlFor={controlId}>{label}</Label>
      {children({
        id: controlId,
        "aria-invalid": error ? true : undefined,
        "aria-describedby": error ? errorId : undefined,
      })}
      {error ? (
        <p id={errorId} className="text-destructive text-sm" role="alert">
          {error}
        </p>
      ) : null}
    </div>
  );
}

/**
 * Every country, memoized: Radix renders a closed Select's items too, and
 * there are about 250 of them.
 */
const CountrySelect = memo(function CountrySelect({
  control,
  value,
  countries,
  onChange,
  placeholder,
  noCountry,
}: {
  control: SelectControl;
  value: string;
  countries: CountryOption[];
  onChange: (code: string) => void;
  placeholder: string;
  noCountry: string;
}) {
  return (
    <Select
      value={value}
      onValueChange={(next) => onChange(next === NO_COUNTRY ? "" : next)}
    >
      <SelectTrigger {...control} className="w-full">
        <SelectValue placeholder={placeholder} />
      </SelectTrigger>
      <SelectContent className="max-h-[min(18rem,var(--radix-select-content-available-height))]">
        <SelectItem value={NO_COUNTRY}>{noCountry}</SelectItem>
        {countries.map((option) => (
          <SelectItem key={option.code} value={option.code}>
            {option.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
});
