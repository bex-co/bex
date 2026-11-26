import * as React from "react";
import { X } from "lucide-react";
import { cn } from "@/common/lib/utils/utils.ts";
import { useTranslations } from "@/common/hooks/use-translations";
import { Input } from "@/common/components/ui/input.tsx";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandItem,
  CommandList,
} from "@/common/components/ui/command.tsx";
import {
  Popover,
  PopoverContent,
  PopoverAnchor,
} from "@/common/components/ui/popover.tsx";
export interface ComboboxOption {
  value: string;
  label: string;
  indent?: number; // For hierarchical display
}

interface ComboboxProps {
  /** Pass a stable reference: a new array resets the keyboard highlight. */
  options: readonly ComboboxOption[];
  value: string;
  onValueChange: (value: string) => void;
  placeholder?: string;
  emptyText?: string;
  allowCustom?: boolean;
  className?: string;
  disabled?: boolean;
  triggerOn?: "change" | "blur"; // When to trigger onValueChange for custom values
  ariaLabel?: string; // Accessible name for the combobox input (a11y + tests)
}

export function Combobox({
  options,
  value,
  onValueChange,
  placeholder,
  emptyText,
  allowCustom = true,
  className,
  disabled = false,
  triggerOn = "change",
  ariaLabel,
}: ComboboxProps) {
  const { t } = useTranslations();
  const defaultPlaceholder = placeholder || t("common.comboboxPlaceholder");
  const defaultEmptyText = emptyText || t("common.comboboxEmpty");

  const [open, setOpen] = React.useState(false);
  const [inputValue, setInputValue] = React.useState(value);
  const [popoverWidth, setPopoverWidth] = React.useState<number | undefined>(
    undefined,
  );
  const [highlightedIndex, setHighlightedIndex] = React.useState(-1);
  // The arrows' highlight draws the focus ring, the pointer's the tint alone.
  const [highlightByPointer, setHighlightByPointer] = React.useState(false);
  const inputRef = React.useRef<HTMLInputElement>(null);
  const containerRef = React.useRef<HTMLDivElement>(null);
  // Unique ID per instance to scope querySelector and avoid cross-instance conflicts
  const instanceId = React.useId().replace(/:/g, "");

  // Sync input value with prop value when it changes externally. Adjusted
  // during render (React's "storing information from previous renders") so the
  // stale text never commits.
  const [syncedValue, setSyncedValue] = React.useState(value);
  if (value !== syncedValue) {
    setSyncedValue(value);
    setInputValue(value);
  }

  // Filter options based on input value
  const filteredOptions = React.useMemo(() => {
    if (!inputValue) return options;

    const search = inputValue.toLowerCase();
    return options.filter(
      (option) =>
        option.label.toLowerCase().includes(search) ||
        option.value.toLowerCase().includes(search),
    );
  }, [options, inputValue]);

  // The highlight is a position in `filteredOptions`, so it is dropped when
  // that list changes or the dropdown closes (reopening starts unhighlighted).
  const [highlightScope, setHighlightScope] = React.useState({
    filteredOptions,
    open,
  });
  if (
    highlightScope.filteredOptions !== filteredOptions ||
    highlightScope.open !== open
  ) {
    setHighlightScope({ filteredOptions, open });
    if (highlightScope.filteredOptions !== filteredOptions || !open) {
      setHighlightedIndex(-1);
    }
  }

  // A free-text field with no options has nothing to list: a panel there only
  // said "no matches" about a value the field already accepts.
  const panelOpen = open && !(allowCustom && options.length === 0);

  const handleSelect = (selectedValue: string) => {
    setInputValue(selectedValue);
    onValueChange(selectedValue);
    setOpen(false);
    inputRef.current?.blur();
  };

  const handleClear = (e: React.MouseEvent) => {
    e.stopPropagation();
    setInputValue("");
    onValueChange("");
    inputRef.current?.focus();
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newValue = e.target.value;
    setInputValue(newValue);

    // If allowCustom is true and triggerOn is "change", immediately update the value
    if (allowCustom && triggerOn === "change") {
      onValueChange(newValue);
    }

    // Open dropdown when typing
    if (!open) {
      setOpen(true);
    }
  };

  // The list opens when it is asked for: a click, typing, or an arrow key.
  // Focus alone does not open it, so tabbing through a form does not drop a
  // panel over the next field (the ARIA combobox pattern).
  const handleInputClick = () => {
    setOpen(true);
  };

  const handleInputBlur = () => {
    // If triggerOn is "blur" and allowCustom is true, update the value on blur
    if (allowCustom && triggerOn === "blur") {
      onValueChange(inputValue);
    }
    // The list belongs to the focused field: focus leaving closes it. Tabbing
    // on used to leave it open over the next field, and Escape then closed
    // the list instead of the dialog around it. A click in the list never
    // gets here: the list keeps focus in the input (onMouseDown below).
    setOpen(false);
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Escape") {
      setOpen(false);
      setHighlightedIndex(-1);
      inputRef.current?.blur();
      return;
    }
    if (e.key === "Enter") {
      e.preventDefault();
      if (highlightedIndex >= 0 && highlightedIndex < filteredOptions.length) {
        // Select the highlighted option
        handleSelect(filteredOptions[highlightedIndex].value);
      } else if (allowCustom) {
        // Use custom value
        onValueChange(inputValue);
        setOpen(false);
        inputRef.current?.blur();
      }
      return;
    }
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;

    e.preventDefault();
    setHighlightByPointer(false);
    const movingDown = e.key === "ArrowDown";
    if (!open) {
      setOpen(true);
      setHighlightedIndex(movingDown ? 0 : filteredOptions.length - 1);
      return;
    }
    setHighlightedIndex((prev) => {
      if (movingDown) {
        return prev < filteredOptions.length - 1 ? prev + 1 : prev;
      }
      return prev > 0 ? prev - 1 : 0;
    });
  };

  // Scroll highlighted item into view
  React.useEffect(() => {
    if (highlightedIndex >= 0 && open) {
      const item = document.querySelector(
        `[data-combobox-item-index="${instanceId}-${highlightedIndex}"]`,
      );
      if (item && typeof item.scrollIntoView === "function") {
        item.scrollIntoView({ block: "nearest", behavior: "smooth" });
      }
    }
  }, [highlightedIndex, open, instanceId]);

  // Measure container width for popover
  React.useLayoutEffect(() => {
    const updateWidth = () => {
      if (containerRef.current) {
        setPopoverWidth(containerRef.current.offsetWidth);
      }
    };

    updateWidth();

    // Update width on window resize
    window.addEventListener("resize", updateWidth);
    return () => window.removeEventListener("resize", updateWidth);
  }, []);

  return (
    <div ref={containerRef} className="relative">
      <Popover open={panelOpen} onOpenChange={setOpen} modal={false}>
        <PopoverAnchor asChild>
          <div className="relative">
            <Input
              ref={inputRef}
              value={inputValue}
              onChange={handleInputChange}
              onClick={handleInputClick}
              onBlur={handleInputBlur}
              onKeyDown={handleKeyDown}
              placeholder={defaultPlaceholder}
              disabled={disabled}
              className={cn("pr-8", className)}
              autoComplete="off"
              role="combobox"
              aria-label={ariaLabel}
              aria-expanded={panelOpen}
              aria-autocomplete="list"
            />

            {inputValue && !disabled && (
              <button
                type="button"
                onClick={handleClear}
                onMouseDown={(e) => e.preventDefault()}
                className="absolute right-2 top-1/2 -translate-y-1/2 rounded-sm hover:bg-muted p-1 z-10 cursor-pointer after:absolute after:-inset-3 after:content-['']"
                aria-label={t("common.comboboxClear")}
                tabIndex={-1}
              >
                <X className="h-3 w-3 text-muted-foreground" />
              </button>
            )}
          </div>
        </PopoverAnchor>

        <PopoverContent
          aria-label={ariaLabel ?? defaultPlaceholder}
          className="overflow-hidden p-0"
          style={{ width: popoverWidth ? `${popoverWidth}px` : undefined }}
          align="start"
          onOpenAutoFocus={(e) => e.preventDefault()}
          // Choosing an option must not blur the input first: the blur would
          // close the list before the click lands.
          onMouseDown={(e) => e.preventDefault()}
          onInteractOutside={(e) => {
            // Prevent closing when interacting with the input or clear button
            if (containerRef.current?.contains(e.target as Node)) {
              e.preventDefault();
            }
          }}
        >
          <Command shouldFilter={false}>
            {/* The list stays the only scroller: it caps at the space Radix
                reports (less the 2px border) when that is under 300px. */}
            <CommandList className="max-h-[min(300px,calc(var(--radix-popover-content-available-height)-2px))]">
              <CommandEmpty>
                {defaultEmptyText}
                {allowCustom && inputValue && (
                  <div className="mt-2 text-xs">
                    {t("common.comboboxUseCustom", { value: inputValue })}
                  </div>
                )}
              </CommandEmpty>
              <CommandGroup>
                {filteredOptions.map((option, index) => (
                  <CommandItem
                    key={option.value}
                    value={option.value}
                    onSelect={handleSelect}
                    // A move, not an enter: the list scrolling under a still
                    // pointer must not take the arrows' highlight.
                    onMouseMove={() => {
                      setHighlightedIndex(index);
                      setHighlightByPointer(true);
                    }}
                    data-combobox-item-index={`${instanceId}-${index}`}
                    // The field, not cmdk, owns the highlight: cmdk's own
                    // selection (its first option, until the pointer moves)
                    // is not what Enter picks.
                    active={
                      highlightedIndex === index &&
                      (highlightByPointer ? "pointer" : "keyboard")
                    }
                  >
                    {option.label}
                  </CommandItem>
                ))}
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      </Popover>
    </div>
  );
}
