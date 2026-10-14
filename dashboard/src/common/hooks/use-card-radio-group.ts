import { useRef, type KeyboardEvent } from "react";

const STEP: Record<string, number> = {
  ArrowRight: 1,
  ArrowDown: 1,
  ArrowLeft: -1,
  ArrowUp: -1,
};

/**
 * The W3C radio-group keyboard contract for card-button radios (w4/m207):
 * one Tab stop — the checked card, else the first — and arrow keys that move
 * focus and selection together, wrapping at either end; Home/End jump to the
 * ends. Space and click stay the buttons' own. Nothing is selected on render
 * or focus, so an empty or unknown value never fires `onChange` by itself.
 */
/** The props one card radio spreads onto its button. */
export interface CardRadioProps {
  ref: (el: HTMLButtonElement | null) => void;
  tabIndex: number;
  onKeyDown: (event: KeyboardEvent<HTMLButtonElement>) => void;
}

export function useCardRadioGroup<T extends string>(
  options: readonly T[],
  value: string | undefined,
  onChange: (option: T) => void,
  disabled = false,
) {
  const buttons = useRef(new Map<T, HTMLButtonElement>());
  const tabStop = options.find((option) => option === value) ?? options[0];

  return (option: T): CardRadioProps => ({
    ref: (el: HTMLButtonElement | null) => {
      if (el) buttons.current.set(option, el);
      else buttons.current.delete(option);
    },
    tabIndex: option === tabStop ? 0 : -1,
    onKeyDown: (event: KeyboardEvent<HTMLButtonElement>) => {
      if (disabled || options.length === 0) return;
      const index = options.indexOf(option);
      let next: T | undefined;
      if (event.key in STEP) {
        next =
          options[(index + STEP[event.key] + options.length) % options.length];
      } else if (event.key === "Home") {
        next = options[0];
      } else if (event.key === "End") {
        next = options[options.length - 1];
      }
      if (next === undefined) return;
      event.preventDefault();
      buttons.current.get(next)?.focus();
      if (next !== value) onChange(next);
    },
  });
}
