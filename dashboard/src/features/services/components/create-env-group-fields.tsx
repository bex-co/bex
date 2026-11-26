import { useEffect, useRef } from "react";
import { Link2Off } from "lucide-react";

import { Button } from "@/common/components/ui/button";
import {
  CREATE_INPUT_CLASS,
  CREATE_SELECT_CLASS,
  CreateField,
} from "@/common/components/create-form";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectSeparator,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { useTranslations } from "@/common/hooks/use-translations";
import type { EnvGroupView } from "@/features/env-groups/types";
import { linkableEnvGroups } from "@/features/services/lib/create-env-groups";

const ADD_ID = "svc-env-group-add";

/**
 * Render's Linked Environment Groups in the create form's Advanced card
 * (w4/731, docs/render-artifacts/new-service-wizard.md § The Advanced
 * section): one picker per linked group with Unlink beside it, then an empty
 * picker while another group can be linked. Each picker offers the groups the
 * chosen Environment can link that no other row holds, the workspace's first.
 * Absent, as on Render, while the workspace has no such group and none is
 * linked. The chosen groups are linked before the first deploy.
 */
export function CreateEnvGroupFields({
  groups,
  environmentId,
  value,
  onChange,
  onOpen,
}: {
  /** The workspace's groups, whatever their scope. */
  groups: ReadonlyArray<EnvGroupView>;
  /** The Environment the service is created in; `null` for none. */
  environmentId: string | null;
  /** The linked groups' ids, in the order chosen. */
  value: string[];
  onChange: (value: string[]) => void;
  /** A picker opened: re-read the groups, as Render's picker does. */
  onOpen: () => void;
}) {
  const { t } = useTranslations();
  // Unlink removes the button pressed, so focus moves to the picker that
  // offers the group again instead of falling back to the page; a row whose
  // group changes keeps it.
  const focusNext = useRef<string | null>(null);
  useEffect(() => {
    if (focusNext.current === null) return;
    document.getElementById(focusNext.current)?.focus();
    focusNext.current = null;
  });

  const linkable = linkableEnvGroups(groups, environmentId);
  const anyLinkable =
    linkable.workspace.length > 0 || linkable.environment.length > 0;
  if (!anyLinkable && value.length === 0) return null;
  const nameOf = (id: string) =>
    groups.find((group) => group.id === id)?.name ?? id;
  const canAdd = [...linkable.workspace, ...linkable.environment].some(
    (group) => !value.includes(group.id),
  );

  return (
    <CreateField
      label={t("services.envGroupsLinkedTitle")}
      optional
      description={t("services.createEnvGroupsDescription")}
    >
      <ul className="space-y-2">
        {value.map((groupId, index) => (
          <li key={groupId} className="flex items-center gap-2">
            <EnvGroupPicker
              id={`svc-env-group-${index}`}
              label={t("services.createEnvGroupRowLabel", {
                n: String(index + 1),
              })}
              linkable={linkable}
              selected={groupId}
              selectedName={nameOf(groupId)}
              taken={value}
              onOpen={onOpen}
              onPick={(next) => {
                // The row is keyed by its group, so it remounts.
                focusNext.current = `svc-env-group-${index}`;
                onChange(value.map((id, i) => (i === index ? next : id)));
              }}
            />
            <Button
              type="button"
              variant="outline"
              className={CREATE_INPUT_CLASS}
              aria-label={t("services.envGroupUnlinkAria", {
                name: nameOf(groupId),
              })}
              onClick={() => {
                focusNext.current = ADD_ID;
                onChange(value.filter((_, i) => i !== index));
              }}
            >
              <Link2Off aria-hidden />
              {t("services.envGroupUnlink")}
            </Button>
          </li>
        ))}
        {canAdd ? (
          <li className="flex">
            <EnvGroupPicker
              id={ADD_ID}
              label={t("services.createEnvGroupAddLabel")}
              linkable={linkable}
              selected=""
              taken={value}
              onOpen={onOpen}
              onPick={(groupId) => onChange([...value, groupId])}
            />
          </li>
        ) : null}
      </ul>
    </CreateField>
  );
}

/**
 * One Linked Environment Groups picker: the linkable groups no other row
 * holds, the workspace's, a separator, then the Environment's, as Render's
 * picker lists them.
 */
function EnvGroupPicker({
  id,
  label,
  linkable,
  selected,
  selectedName,
  taken,
  onOpen,
  onPick,
}: {
  id: string;
  label: string;
  linkable: ReturnType<typeof linkableEnvGroups>;
  /** The row's group, `""` for the empty picker. */
  selected: string;
  selectedName?: string;
  /** Every linked group, this row's included. */
  taken: string[];
  onOpen: () => void;
  onPick: (groupId: string) => void;
}) {
  const { t } = useTranslations();
  const offered = (group: EnvGroupView) =>
    group.id === selected || !taken.includes(group.id);
  const workspace = linkable.workspace.filter(offered);
  const environment = linkable.environment.filter(offered);
  return (
    // Its own box: before hydration Radix adds a hidden native <select>
    // beside the trigger, which must not take the row's gap.
    <div className="min-w-0 flex-1">
      <Select
        value={selected}
        onValueChange={onPick}
        onOpenChange={(open) => {
          if (open) onOpen();
        }}
      >
        <SelectTrigger
          id={id}
          aria-label={label}
          className={CREATE_SELECT_CLASS}
        >
          <SelectValue placeholder={t("services.createEnvGroupPlaceholder")}>
            {selected ? selectedName : undefined}
          </SelectValue>
        </SelectTrigger>
        <SelectContent>
          {workspace.map((group) => (
            <SelectItem key={group.id} value={group.id}>
              {group.name}
            </SelectItem>
          ))}
          {workspace.length > 0 && environment.length > 0 ? (
            <SelectSeparator />
          ) : null}
          {environment.map((group) => (
            <SelectItem key={group.id} value={group.id}>
              {group.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
