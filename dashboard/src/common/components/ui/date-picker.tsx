"use client";

import * as React from "react";
import { CalendarIcon } from "lucide-react";

import { Button } from "@/common/components/ui/button";
import { Calendar } from "@/common/components/ui/calendar";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/common/components/ui/popover";
function formatDate(date: Date | undefined, short = false) {
  if (!date) {
    return "";
  }

  if (short) {
    // Compact format: MM/DD/YYYY
    return date.toLocaleDateString("en-US", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
    });
  }

  return date.toLocaleDateString("en-US", {
    day: "2-digit",
    month: "long",
    year: "numeric",
  });
}

function isValidDate(date: Date | undefined) {
  if (!date) {
    return false;
  }
  return !isNaN(date.getTime());
}

interface DatePickerProps {
  id?: string;
  label?: string;
  value: Date | undefined;
  onChange: (date: Date | undefined) => void;
  placeholder?: string;
  required?: boolean;
  className?: string;
}

/**
 * Reusable date picker component using shadcn/ui Calendar and Popover
 * Provides both input field and calendar popup for date selection
 */
export function DatePicker({
  id,
  label,
  value,
  onChange,
  placeholder,
  required = false,
  className = "",
}: DatePickerProps) {
  const defaultPlaceholder = placeholder || "Select date";
  const [open, setOpen] = React.useState(false);
  const [month, setMonth] = React.useState<Date | undefined>(value);
  const [inputValue, setInputValue] = React.useState(formatDate(value, true));

  // Update input value when value prop changes
  React.useEffect(() => {
    setInputValue(formatDate(value, true));
    if (value) {
      setMonth(value);
    }
  }, [value]);

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const inputVal = e.target.value;
    setInputValue(inputVal);

    // Try to parse the input as a date
    const date = new Date(inputVal);
    if (isValidDate(date)) {
      onChange(date);
      setMonth(date);
    }
  };

  const handleCalendarSelect = (date: Date | undefined) => {
    onChange(date);
    setInputValue(formatDate(date, true));
    setOpen(false);
  };

  const handleTodayClick = () => {
    const today = new Date();
    onChange(today);
    setInputValue(formatDate(today, true));
    setMonth(today);
    setOpen(false);
  };

  return (
    <div className={`space-y-2 ${className}`}>
      {label && (
        <Label htmlFor={id} className="px-1">
          {label}
        </Label>
      )}
      <div className="relative flex gap-2 w-[128px]">
        <Input
          id={id}
          value={inputValue}
          placeholder={defaultPlaceholder}
          className="bg-background pr-10"
          onChange={handleInputChange}
          onKeyDown={(e) => {
            if (e.key === "ArrowDown") {
              e.preventDefault();
              setOpen(true);
            }
          }}
          required={required}
        />
        <Popover open={open} onOpenChange={setOpen}>
          <PopoverTrigger asChild>
            <Button
              type="button"
              variant="ghost"
              className="absolute top-1/2 right-2 size-6 -translate-y-1/2"
            >
              <CalendarIcon className="size-3.5" />
              <span className="sr-only">Select date</span>
            </Button>
          </PopoverTrigger>
          <PopoverContent
            className="w-auto overflow-hidden p-0"
            align="end"
            alignOffset={-8}
            sideOffset={10}
          >
            <div className="flex items-center justify-between border-b px-3 py-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={handleTodayClick}
                className="h-7 text-xs"
              >
                Today
              </Button>
            </div>
            <Calendar
              mode="single"
              selected={value}
              captionLayout="dropdown"
              month={month}
              onMonthChange={setMonth}
              onSelect={handleCalendarSelect}
            />
          </PopoverContent>
        </Popover>
      </div>
    </div>
  );
}
