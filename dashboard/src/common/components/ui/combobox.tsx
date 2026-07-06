import * as React from "react";
import { X } from "lucide-react";
import { cn } from "@/common/lib/utils/utils.ts";
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
  options: ComboboxOption[];
  value: string;
  onValueChange: (value: string) => void;
  placeholder?: string;
  emptyText?: string;
  allowCustom?: boolean;
  className?: string;
  disabled?: boolean;
  triggerOn?: "change" | "blur"; // When to trigger onValueChange for custom values
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
}: ComboboxProps) {
  const defaultPlaceholder = placeholder || "Select an option";
  const defaultEmptyText = emptyText || "No results found";

  const [open, setOpen] = React.useState(false);
  const [inputValue, setInputValue] = React.useState(value);
  const [popoverWidth, setPopoverWidth] = React.useState<number | undefined>(
    undefined,
  );
  const [highlightedIndex, setHighlightedIndex] = React.useState(-1);
  const inputRef = React.useRef<HTMLInputElement>(null);
  const containerRef = React.useRef<HTMLDivElement>(null);
  // Unique ID per instance to scope querySelector and avoid cross-instance conflicts
  const instanceId = React.useId().replace(/:/g, "");

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

  const handleInputFocus = () => {
    setOpen(true);
  };

  const handleInputBlur = () => {
    // If triggerOn is "blur" and allowCustom is true, update the value on blur
    if (allowCustom && triggerOn === "blur") {
      onValueChange(inputValue);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Escape") {
      setOpen(false);
      setHighlightedIndex(-1);
      inputRef.current?.blur();
    } else if (e.key === "Enter") {
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
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      if (!open) {
        setOpen(true);
        setHighlightedIndex(0);
      } else {
        setHighlightedIndex((prev) =>
          prev < filteredOptions.length - 1 ? prev + 1 : prev,
        );
      }
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      if (!open) {
        setOpen(true);
        setHighlightedIndex(filteredOptions.length - 1);
      } else {
        setHighlightedIndex((prev) => (prev > 0 ? prev - 1 : 0));
      }
    }
  };

  // Sync input value with prop value when it changes externally
  React.useEffect(() => {
    setInputValue(value);
  }, [value]);

  // Reset highlighted index when filtered options change
  React.useEffect(() => {
    setHighlightedIndex(-1);
  }, [filteredOptions]);

  // Reset highlighted index when dropdown closes
  React.useEffect(() => {
    if (!open) {
      setHighlightedIndex(-1);
    }
  }, [open]);

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
      <Popover open={open} onOpenChange={setOpen} modal={false}>
        <PopoverAnchor asChild>
          <div className="relative">
            <Input
              ref={inputRef}
              value={inputValue}
              onChange={handleInputChange}
              onFocus={handleInputFocus}
              onBlur={handleInputBlur}
              onKeyDown={handleKeyDown}
              placeholder={defaultPlaceholder}
              disabled={disabled}
              className={cn("pr-8", className)}
              autoComplete="off"
              role="combobox"
              aria-expanded={open}
              aria-autocomplete="list"
            />

            {inputValue && !disabled && (
              <button
                type="button"
                onClick={handleClear}
                onMouseDown={(e) => e.preventDefault()}
                className="absolute right-2 top-1/2 -translate-y-1/2 rounded-sm hover:bg-muted p-1 z-10 cursor-pointer"
                aria-label="Clear input"
                tabIndex={-1}
              >
                <X className="h-3 w-3 text-muted-foreground" />
              </button>
            )}
          </div>
        </PopoverAnchor>

        <PopoverContent
          className="p-0"
          style={{ width: popoverWidth ? `${popoverWidth}px` : undefined }}
          align="start"
          onOpenAutoFocus={(e) => e.preventDefault()}
          onInteractOutside={(e) => {
            // Prevent closing when interacting with the input or clear button
            if (containerRef.current?.contains(e.target as Node)) {
              e.preventDefault();
            }
          }}
        >
          <Command shouldFilter={false}>
            <CommandList>
              <CommandEmpty>
                {defaultEmptyText}
                {allowCustom && inputValue && (
                  <div className="mt-2 text-xs">
                    Press Enter to use "{inputValue}"
                  </div>
                )}
              </CommandEmpty>
              <CommandGroup>
                {filteredOptions.map((option, index) => (
                  <CommandItem
                    key={option.value}
                    value={option.value}
                    onSelect={handleSelect}
                    onMouseEnter={() => setHighlightedIndex(index)}
                    data-combobox-item-index={`${instanceId}-${index}`}
                    className={cn(
                      // Cursor pointer for clickable items
                      "cursor-pointer",
                      // Override cmdk's default selected styling for non-highlighted items
                      highlightedIndex >= 0 &&
                        highlightedIndex !== index &&
                        "data-[selected=true]:bg-transparent data-[selected=true]:text-current",
                      // Apply our keyboard highlight styling
                      highlightedIndex === index &&
                        "bg-accent text-accent-foreground",
                    )}
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
