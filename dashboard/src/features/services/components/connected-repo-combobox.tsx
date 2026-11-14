import { useMemo } from "react";
import { Combobox } from "@/common/components/ui/combobox";
import { useRepos } from "@/features/services/hooks/use-repos";

/**
 * A combobox of the workspace's connected repositories (owner/name) that also
 * accepts a typed value. The list loads only when this mounts.
 */
export function ConnectedRepoCombobox({
  value,
  onChange,
  placeholder,
  emptyText,
  ariaLabel,
  className,
  disabled,
}: {
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
  emptyText: string;
  ariaLabel: string;
  className?: string;
  disabled?: boolean;
}) {
  const { repos } = useRepos();
  const options = useMemo(
    () => repos.map((r) => ({ value: r.fullName, label: r.fullName })),
    [repos],
  );
  return (
    <Combobox
      options={options}
      value={value}
      onValueChange={onChange}
      placeholder={placeholder}
      emptyText={emptyText}
      ariaLabel={ariaLabel}
      disabled={disabled}
      className={className}
    />
  );
}
