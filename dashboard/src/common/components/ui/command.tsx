import * as React from "react";
import { type DialogProps } from "@radix-ui/react-dialog";
// import { Search } from "lucide-react";
import { Command as CommandPrimitive } from "cmdk";

import { cn } from "@/common/lib/utils/utils.ts";
import { useTranslations } from "@/common/hooks/use-translations";
import { Dialog, DialogContent } from "@/common/components/ui/dialog.tsx";

// Focus stays in the search field while the arrows move cmdk's selection
// (aria-activedescendant), so the field's ring does not move with them: the
// active option draws its own indicator, a 2px outline inset in the focus
// colour as Render's list rows do (w3/114). The tint alone measured 1.2:1 on
// the list. cmdk also selects the option under a moving pointer, and a ring
// there would follow the mouse, so an option the pointer chose keeps the tint
// alone until a key is pressed.
const SelectedByPointer = React.createContext(false);

function Command({
  className,
  onKeyDown,
  onPointerMove,
  ...props
}: React.ComponentPropsWithoutRef<typeof CommandPrimitive>) {
  const [selectedByPointer, setSelectedByPointer] = React.useState(false);
  return (
    <SelectedByPointer.Provider value={selectedByPointer}>
      <CommandPrimitive
        className={cn(
          "flex h-full w-full flex-col overflow-hidden rounded-md bg-popover text-popover-foreground",
          className,
        )}
        onKeyDown={(event) => {
          setSelectedByPointer(false);
          onKeyDown?.(event);
        }}
        onPointerMove={(event) => {
          // Where cmdk's own item handler moves the selection to the pointer.
          if (
            !selectedByPointer &&
            !props.disablePointerSelection &&
            event.target instanceof Element &&
            event.target.closest('[cmdk-item]:not([aria-disabled="true"])')
          )
            setSelectedByPointer(true);
          onPointerMove?.(event);
        }}
        {...props}
      />
    </SelectedByPointer.Provider>
  );
}

function CommandDialog({
  children,
  shouldFilter,
  // cmdk's Input always sets aria-labelledby to Command's labelId. Without a
  // non-empty `label`, that target stays blank and suppresses the placeholder
  // name (m101/t011). aria-label on CommandInput does not win over labelledby.
  label,
  onCloseAutoFocus,
  ...props
}: DialogProps & {
  shouldFilter?: boolean;
  label?: string;
  /**
   * Where focus goes when the dialog closes. A dialog opened from outside its
   * own Radix trigger has none registered, so Radix's default restores
   * nothing; such a caller supplies the destination here.
   */
  onCloseAutoFocus?: (event: Event) => void;
}) {
  return (
    <Dialog {...props}>
      <DialogContent
        className="overflow-hidden p-0"
        onCloseAutoFocus={onCloseAutoFocus}
      >
        <Command
          shouldFilter={shouldFilter}
          label={label}
          className="[&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:font-medium [&_[cmdk-group-heading]]:text-muted-foreground [&_[cmdk-group]:not([hidden])_~[cmdk-group]]:pt-0 [&_[cmdk-group]]:px-2 [&_[cmdk-input-wrapper]_svg]:h-5 [&_[cmdk-input-wrapper]_svg]:w-5 [&_[cmdk-input]]:h-12 [&_[cmdk-item]]:px-2 [&_[cmdk-item]]:py-3 [&_[cmdk-item]_svg]:h-5 [&_[cmdk-item]_svg]:w-5"
        >
          {children}
        </Command>
      </DialogContent>
    </Dialog>
  );
}

function CommandInput({
  className,
  ...props
}: React.ComponentPropsWithoutRef<typeof CommandPrimitive.Input>) {
  return (
    <div className="flex items-center border-b px-3" cmdk-input-wrapper="">
      {/* <Search className="mr-2 h-4 w-4 shrink-0 opacity-50" /> */}
      <CommandPrimitive.Input
        className={cn(
          "flex h-10 w-full rounded-md bg-transparent py-3 text-sm outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-50",
          className,
        )}
        {...props}
      />
    </div>
  );
}

// cmdk names the listbox through its `label` prop (default "Suggestions"),
// applied after spread props, so an aria-label cannot override it (w4/312).
function CommandList({
  className,
  label,
  ...props
}: React.ComponentPropsWithoutRef<typeof CommandPrimitive.List>) {
  const { t } = useTranslations();
  return (
    <CommandPrimitive.List
      className={cn(
        "max-h-[300px] overflow-y-auto overflow-x-hidden",
        className,
      )}
      label={label ?? t("common.commandSuggestions")}
      {...props}
    />
  );
}

function CommandEmpty(
  props: React.ComponentPropsWithoutRef<typeof CommandPrimitive.Empty>,
) {
  return (
    <CommandPrimitive.Empty className="py-6 text-center text-sm" {...props} />
  );
}

function CommandGroup({
  className,
  ...props
}: React.ComponentPropsWithoutRef<typeof CommandPrimitive.Group>) {
  return (
    <CommandPrimitive.Group
      className={cn(
        "overflow-hidden p-1 text-foreground [&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:py-1.5 [&_[cmdk-group-heading]]:text-xs [&_[cmdk-group-heading]]:font-medium [&_[cmdk-group-heading]]:text-muted-foreground",
        className,
      )}
      {...props}
    />
  );
}

function CommandSeparator({
  className,
  ...props
}: React.ComponentPropsWithoutRef<typeof CommandPrimitive.Separator>) {
  return (
    <CommandPrimitive.Separator
      className={cn("-mx-1 h-px bg-border", className)}
      {...props}
    />
  );
}

function CommandItem({
  className,
  active,
  ...props
}: React.ComponentPropsWithoutRef<typeof CommandPrimitive.Item> & {
  /**
   * For a list that tracks its active option itself, as the combobox does
   * (its field sits outside cmdk): how this option became the active one, or
   * false. Set, it replaces cmdk's selection as what the option marks.
   */
  active?: "keyboard" | "pointer" | false;
}) {
  const selectedByPointer = React.useContext(SelectedByPointer);
  return (
    <CommandPrimitive.Item
      className={cn(
        // No `outline-none`: the option never takes focus, and it would void
        // the indicator's width (Tailwind 4 draws `var(--tw-outline-style)`).
        "relative flex cursor-pointer gap-2 select-none items-center rounded-sm px-2 py-1.5 text-sm data-[disabled=true]:pointer-events-none data-[disabled=true]:opacity-50 [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0",
        active === undefined
          ? [
              "data-[selected=true]:bg-accent data-[selected=true]:text-accent-foreground",
              !selectedByPointer &&
                "data-[selected=true]:outline-2 data-[selected=true]:-outline-offset-2 data-[selected=true]:outline-ring",
            ]
          : [
              active && "bg-accent text-accent-foreground",
              active === "keyboard" &&
                "outline-2 -outline-offset-2 outline-ring",
            ],
        className,
      )}
      {...props}
    />
  );
}

function CommandShortcut({
  className,
  ...props
}: React.HTMLAttributes<HTMLSpanElement>) {
  return (
    <span
      className={cn(
        "ml-auto text-xs tracking-widest text-muted-foreground",
        className,
      )}
      {...props}
    />
  );
}

export {
  Command,
  CommandDialog,
  CommandInput,
  CommandList,
  CommandEmpty,
  CommandGroup,
  CommandItem,
  CommandSeparator,
  CommandShortcut,
};
