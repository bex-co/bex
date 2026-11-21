import { useState, type FormEvent } from "react";
import { AlertCircle } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { Switch } from "@/common/components/ui/switch";
import { Textarea } from "@/common/components/ui/textarea";
import { useTranslations } from "@/common/hooks/use-translations";
import type {
  CreateSandboxGroupCredentialInput,
  UpdateSandboxGroupCredentialInput,
} from "@/graphql/definitions";
import type { SandboxGroupCredential } from "@/features/sandbox-groups/hooks/use-sandbox-group-credentials";
import {
  CREDENTIAL_HTTP_METHODS,
  CREDENTIAL_INJECTION_TYPES,
  CREDENTIAL_TEMPLATE_PLACEHOLDER,
  MAX_CREDENTIAL_TEMPLATE_PLACEHOLDERS,
  MAX_CREDENTIAL_VALUE_BYTES,
  parsePathsText,
  validateCredentialForm,
  type CredentialFormValues,
  type CredentialInjectionType,
  type CredentialValidationError,
} from "@/features/sandbox-groups/lib/credential-validation";

export interface CredentialFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** When non-null, edits this credential; when null, creates a new one. */
  editing: SandboxGroupCredential | null;
  /** True for workspace Admins (`canManage`), false for Developers (`canCreate`). */
  canManage: boolean;
  submitting: boolean;
  onCreate: (
    input: CreateSandboxGroupCredentialInput,
  ) => Promise<{ ok: boolean; errorMessage: string | null }>;
  onUpdate: (
    id: string,
    input: UpdateSandboxGroupCredentialInput,
  ) => Promise<{ ok: boolean; errorMessage: string | null }>;
}

function toInjectionType(raw: string): CredentialInjectionType {
  return (CREDENTIAL_INJECTION_TYPES as readonly string[]).includes(raw)
    ? (raw as CredentialInjectionType)
    : "header";
}

function initialValues(
  editing: SandboxGroupCredential | null,
): CredentialFormValues {
  if (!editing) {
    return {
      id: "",
      domain: "",
      value: "",
      enabled: true,
      methods: [],
      pathsText: "",
      injectionType: "header",
      injectionName: "Authorization",
      injectionTemplate: "Bearer ${SECRET}",
    };
  }
  return {
    id: editing.id,
    domain: editing.domain,
    value: "",
    enabled: editing.enabled,
    methods: [...editing.methods],
    pathsText: editing.paths.join("\n"),
    injectionType: toInjectionType(editing.injection.type),
    injectionName: editing.injection.name ?? "",
    injectionTemplate: editing.injection.template ?? "",
  };
}

export function CredentialFormDialog({
  open,
  onOpenChange,
  editing,
  canManage,
  submitting,
  onCreate,
  onUpdate,
}: CredentialFormDialogProps) {
  const mode = editing ? "edit" : "create";
  // Key inner form by open state + target credential so secret state is wiped
  // clean every time the dialog opens or closes.
  const formKey = `${open ? "open" : "closed"}:${editing?.id ?? "new"}:${editing?.version.id ?? 0}`;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
        {open ? (
          <CredentialFormBody
            key={formKey}
            mode={mode}
            editing={editing}
            canManage={canManage}
            submitting={submitting}
            onClose={() => onOpenChange(false)}
            onCreate={onCreate}
            onUpdate={onUpdate}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  );
}

function CredentialFormBody({
  mode,
  editing,
  canManage,
  submitting,
  onClose,
  onCreate,
  onUpdate,
}: {
  mode: "create" | "edit";
  editing: SandboxGroupCredential | null;
  canManage: boolean;
  submitting: boolean;
  onClose: () => void;
  onCreate: CredentialFormDialogProps["onCreate"];
  onUpdate: CredentialFormDialogProps["onUpdate"];
}) {
  const { t } = useTranslations();
  const [values, setValues] = useState<CredentialFormValues>(() =>
    initialValues(editing),
  );
  const [validationError, setValidationError] =
    useState<CredentialValidationError | null>(null);
  const [serverError, setServerError] = useState<string | null>(null);

  const canEditSecretAndDomain = mode === "create" || canManage;

  const updateField = <K extends keyof CredentialFormValues>(
    field: K,
    value: CredentialFormValues[K],
  ) => {
    setValues((prev) => ({ ...prev, [field]: value }));
    setValidationError(null);
    setServerError(null);
  };

  const toggleMethod = (method: string) => {
    setValues((prev) => {
      const exists = prev.methods.includes(method);
      return {
        ...prev,
        methods: exists
          ? prev.methods.filter((m) => m !== method)
          : [...prev.methods, method],
      };
    });
    setValidationError(null);
    setServerError(null);
  };

  const handleInjectionTypeChange = (next: string) => {
    const nextType = toInjectionType(next);
    setValues((prev) => ({
      ...prev,
      injectionType: nextType,
      injectionName: nextType === "basic" ? "" : prev.injectionName,
    }));
    setValidationError(null);
    setServerError(null);
  };

  const validationMessage = (err: CredentialValidationError | null): string | null => {
    if (!err) return null;
    switch (err.code) {
      case "idRequired":
        return t("sandboxGroups.credValIdRequired");
      case "idInvalid":
        return t("sandboxGroups.credValIdInvalid");
      case "idReserved":
        return t("sandboxGroups.credValIdReserved");
      case "domainRequired":
        return t("sandboxGroups.credValDomainRequired");
      case "domainInvalid":
        return t("sandboxGroups.credValDomainInvalid");
      case "domainPrivate":
        return t("sandboxGroups.credValDomainPrivate");
      case "valueRequired":
        return t("sandboxGroups.credValValueRequired");
      case "valueTooLong":
        return t("sandboxGroups.credValValueTooLong", {
          maxBytes: MAX_CREDENTIAL_VALUE_BYTES,
        });
      case "valueNul":
        return t("sandboxGroups.credValValueNul");
      case "injectionNameRequired":
        return t("sandboxGroups.credValInjectionNameRequired");
      case "injectionNameForbidden":
        return t("sandboxGroups.credValInjectionNameForbidden");
      case "injectionHeaderForbidden":
        return t("sandboxGroups.credValInjectionHeaderForbidden");
      case "injectionNameInvalid":
        return t("sandboxGroups.credValInjectionNameInvalid");
      case "injectionTemplateCount":
        return t("sandboxGroups.credValInjectionTemplateCount", {
          max: MAX_CREDENTIAL_TEMPLATE_PLACEHOLDERS,
          token: CREDENTIAL_TEMPLATE_PLACEHOLDER,
        });
      case "injectionBasicFormat":
        return t("sandboxGroups.credValInjectionBasicFormat");
      case "injectionHeaderControl":
        return t("sandboxGroups.credValInjectionHeaderControl");
      case "injectionCookieInvalid":
        return t("sandboxGroups.credValInjectionCookieInvalid");
      case "pathsInvalid":
        return t("sandboxGroups.credValPathsInvalid");
    }
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    const err = validateCredentialForm(values, { mode, canManage });
    if (err) {
      setValidationError(err);
      return;
    }

    const methods = [...values.methods];
    const paths = parsePathsText(values.pathsText);
    const injection = {
      type: values.injectionType,
      ...(values.injectionType !== "basic"
        ? { name: values.injectionName.trim() }
        : {}),
      ...(values.injectionTemplate.length > 0
        ? { template: values.injectionTemplate }
        : {}),
    };

    if (mode === "create") {
      const res = await onCreate({
        id: values.id.trim(),
        domain: values.domain.trim(),
        value: values.value,
        enabled: values.enabled,
        methods,
        paths,
        injection,
      });
      if (res.ok) {
        setValues((prev) => ({ ...prev, value: "" }));
        onClose();
      } else {
        setServerError(res.errorMessage);
      }
      return;
    }

    if (!editing) return;
    // Developers (`!canManage`) must never send `domain` or `value` so the
    // server's split RBAC (`can_create`) accepts their routing/injection edit.
    const updateInput: UpdateSandboxGroupCredentialInput = {
      enabled: values.enabled,
      methods,
      paths,
      injection,
      ...(canManage
        ? {
            domain: values.domain.trim(),
            ...(values.value.length > 0 ? { value: values.value } : {}),
          }
        : {}),
    };
    const res = await onUpdate(editing.id, updateInput);
    if (res.ok) {
      setValues((prev) => ({ ...prev, value: "" }));
      onClose();
    } else {
      setServerError(res.errorMessage);
    }
  };

  const fieldError = (field: CredentialValidationError["field"]) =>
    validationError?.field === field ? validationMessage(validationError) : null;

  return (
    <form onSubmit={(e) => void handleSubmit(e)} className="space-y-4">
      <DialogHeader>
        <DialogTitle>
          {mode === "create"
            ? t("sandboxGroups.credCreateTitle")
            : t("sandboxGroups.credEditTitle", { id: editing?.id ?? "" })}
        </DialogTitle>
        <DialogDescription>
          {mode === "create"
            ? t("sandboxGroups.credCreateDescription")
            : canManage
              ? t("sandboxGroups.credEditDescriptionAdmin")
              : t("sandboxGroups.credEditDescriptionDeveloper")}
        </DialogDescription>
      </DialogHeader>

      {serverError ? (
        <Alert variant="destructive">
          <AlertCircle className="size-4" />
          <AlertDescription>{serverError}</AlertDescription>
        </Alert>
      ) : null}

      {/* Credential ID */}
      <div className="space-y-1.5">
        <Label htmlFor="cred-id">{t("sandboxGroups.credFieldId")}</Label>
        <Input
          id="cred-id"
          value={values.id}
          onChange={(e) => updateField("id", e.target.value)}
          disabled={mode === "edit" || submitting}
          placeholder="openai-key"
          aria-invalid={fieldError("id") != null}
        />
        {fieldError("id") ? (
          <p className="text-destructive text-xs" role="alert">
            {fieldError("id")}
          </p>
        ) : (
          <p className="text-muted-foreground text-xs">
            {t("sandboxGroups.credFieldIdHelp")}
          </p>
        )}
      </div>

      {/* Destination domain */}
      <div className="space-y-1.5">
        <Label htmlFor="cred-domain">
          {t("sandboxGroups.credFieldDomain")}
        </Label>
        <Input
          id="cred-domain"
          value={values.domain}
          onChange={(e) => updateField("domain", e.target.value)}
          disabled={!canEditSecretAndDomain || submitting}
          placeholder="api.openai.com"
          aria-invalid={fieldError("domain") != null}
        />
        {fieldError("domain") ? (
          <p className="text-destructive text-xs" role="alert">
            {fieldError("domain")}
          </p>
        ) : !canEditSecretAndDomain ? (
          <p className="text-muted-foreground text-xs">
            {t("sandboxGroups.credAdminOnlyDomainSecret")}
          </p>
        ) : null}
      </div>

      {/* Write-only secret value (hidden for Developers in edit mode) */}
      {canEditSecretAndDomain ? (
        <div className="space-y-1.5">
          <Label htmlFor="cred-value">
            {mode === "create"
              ? t("sandboxGroups.credFieldValue")
              : t("sandboxGroups.credFieldReplacementValue")}
          </Label>
          <Input
            id="cred-value"
            type="password"
            autoComplete="new-password"
            value={values.value}
            onChange={(e) => updateField("value", e.target.value)}
            disabled={submitting}
            placeholder={
              mode === "create"
                ? t("sandboxGroups.credFieldValuePlaceholder")
                : t("sandboxGroups.credFieldReplacementValuePlaceholder")
            }
            aria-invalid={fieldError("value") != null}
          />
          {fieldError("value") ? (
            <p className="text-destructive text-xs" role="alert">
              {fieldError("value")}
            </p>
          ) : (
            <p className="text-muted-foreground text-xs">
              {t("sandboxGroups.credFieldValueWriteOnlyHelp")}
            </p>
          )}
        </div>
      ) : null}

      {/* Injection settings */}
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <div className="space-y-1.5">
          <Label htmlFor="cred-injection-type">
            {t("sandboxGroups.credFieldInjectionType")}
          </Label>
          <Select
            value={values.injectionType}
            onValueChange={handleInjectionTypeChange}
            disabled={submitting}
          >
            <SelectTrigger id="cred-injection-type">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="header">
                {t("sandboxGroups.credInjectionHeader")}
              </SelectItem>
              <SelectItem value="basic">
                {t("sandboxGroups.credInjectionBasic")}
              </SelectItem>
              <SelectItem value="cookie">
                {t("sandboxGroups.credInjectionCookie")}
              </SelectItem>
              <SelectItem value="query">
                {t("sandboxGroups.credInjectionQuery")}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>

        {values.injectionType !== "basic" ? (
          <div className="space-y-1.5">
            <Label htmlFor="cred-injection-name">
              {t("sandboxGroups.credFieldInjectionName")}
            </Label>
            <Input
              id="cred-injection-name"
              value={values.injectionName}
              onChange={(e) => updateField("injectionName", e.target.value)}
              disabled={submitting}
              placeholder={
                values.injectionType === "header"
                  ? "Authorization"
                  : values.injectionType === "cookie"
                    ? "session"
                    : "api_key"
              }
              aria-invalid={fieldError("injectionName") != null}
            />
            {fieldError("injectionName") ? (
              <p className="text-destructive text-xs" role="alert">
                {fieldError("injectionName")}
              </p>
            ) : null}
          </div>
        ) : (
          <div className="flex items-end pb-2">
            <p className="text-muted-foreground text-xs">
              {t("sandboxGroups.credBasicHelp")}
            </p>
          </div>
        )}
      </div>

      {/* Optional template */}
      <div className="space-y-1.5">
        <Label htmlFor="cred-injection-template">
          {t("sandboxGroups.credFieldTemplate")}
        </Label>
        <Input
          id="cred-injection-template"
          value={values.injectionTemplate}
          onChange={(e) => updateField("injectionTemplate", e.target.value)}
          disabled={submitting}
          placeholder="${SECRET}"
          aria-invalid={fieldError("injectionTemplate") != null}
        />
        {fieldError("injectionTemplate") ? (
          <p className="text-destructive text-xs" role="alert">
            {fieldError("injectionTemplate")}
          </p>
        ) : (
          <p className="text-muted-foreground text-xs">
            {t("sandboxGroups.credFieldTemplateHelp", {
              token: CREDENTIAL_TEMPLATE_PLACEHOLDER,
            })}
          </p>
        )}
      </div>

      {/* Methods multi-select */}
      <div className="space-y-1.5">
        <span className="text-sm font-medium">
          {t("sandboxGroups.credFieldMethods")}
        </span>
        <div
          className="flex flex-wrap gap-1.5"
          role="group"
          aria-label={t("sandboxGroups.credFieldMethods")}
        >
          {CREDENTIAL_HTTP_METHODS.map((method) => {
            const selected = values.methods.includes(method);
            return (
              <Button
                key={method}
                type="button"
                size="sm"
                variant={selected ? "default" : "outline"}
                aria-pressed={selected}
                disabled={submitting}
                onClick={() => toggleMethod(method)}
              >
                {method}
              </Button>
            );
          })}
        </div>
        <p className="text-muted-foreground text-xs">
          {t("sandboxGroups.credFieldMethodsHelp")}
        </p>
      </div>

      {/* Path patterns */}
      <div className="space-y-1.5">
        <Label htmlFor="cred-paths">{t("sandboxGroups.credFieldPaths")}</Label>
        <Textarea
          id="cred-paths"
          rows={3}
          value={values.pathsText}
          onChange={(e) => updateField("pathsText", e.target.value)}
          disabled={submitting}
          placeholder={"/v1/**\n/v2/models/*"}
          aria-invalid={fieldError("paths") != null}
        />
        {fieldError("paths") ? (
          <p className="text-destructive text-xs" role="alert">
            {fieldError("paths")}
          </p>
        ) : (
          <p className="text-muted-foreground text-xs">
            {t("sandboxGroups.credFieldPathsHelp")}
          </p>
        )}
      </div>

      {/* Enabled toggle */}
      <div className="flex items-center justify-between gap-3 rounded-lg border px-3 py-2.5">
        <div className="space-y-0.5">
          <Label htmlFor="cred-enabled">
            {t("sandboxGroups.credFieldEnabled")}
          </Label>
          <p className="text-muted-foreground text-xs">
            {t("sandboxGroups.credFieldEnabledHelp")}
          </p>
        </div>
        <Switch
          id="cred-enabled"
          checked={values.enabled}
          onCheckedChange={(checked) => updateField("enabled", checked)}
          disabled={submitting}
        />
      </div>

      <DialogFooter>
        <Button
          type="button"
          variant="outline"
          disabled={submitting}
          onClick={onClose}
        >
          {t("sandboxGroups.createCancel")}
        </Button>
        <Button type="submit" disabled={submitting}>
          {mode === "create"
            ? t("sandboxGroups.credCreateSubmit")
            : t("sandboxGroups.credEditSubmit")}
        </Button>
      </DialogFooter>
    </form>
  );
}
