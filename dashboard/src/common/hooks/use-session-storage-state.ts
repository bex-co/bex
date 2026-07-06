import { useState, useCallback, useRef, useEffect } from "react";

/**
 * Options for useSessionStorageState hook
 */
export interface UseSessionStorageStateOptions<T> {
  /**
   * Serializer function to convert value to string
   * @default JSON.stringify
   */
  serializer?: (value: T) => string;

  /**
   * Deserializer function to convert string to value
   * @default JSON.parse
   */
  deserializer?: (value: string) => T;
}

/**
 * A React hook that syncs state with sessionStorage
 *
 * Features:
 * - Persists state to sessionStorage automatically (cleared on tab/window close)
 * - Lazy initialization from sessionStorage
 * - Graceful error handling for sessionStorage failures
 * - Type-safe with TypeScript generics
 * - Customizable serialization/deserialization
 *
 * @param key - sessionStorage key
 * @param defaultValue - default value if key doesn't exist
 * @param options - configuration options
 * @returns [value, setValue, removeValue] - stateful value and setter
 *
 * @example
 * ```tsx
 * // Simple boolean state
 * const [isDismissed, setIsDismissed] = useSessionStorageState('banner-dismissed', false);
 *
 * // Complex object state
 * const [draft, setDraft] = useSessionStorageState('form-draft', { name: '', email: '' });
 *
 * // With custom serialization
 * const [date, setDate] = useSessionStorageState('selected-date', new Date(), {
 *   serializer: (d) => d.toISOString(),
 *   deserializer: (s) => new Date(s),
 * });
 * ```
 */
export function useSessionStorageState<T>(
  key: string,
  defaultValue: T,
  options: UseSessionStorageStateOptions<T> = {},
): [T, (value: T | ((prev: T) => T)) => void, () => void] {
  const { serializer = JSON.stringify, deserializer = JSON.parse } = options;

  /**
   * Read value from sessionStorage with error handling
   */
  const readValue = (): T => {
    try {
      const item = sessionStorage.getItem(key);
      if (item === null) {
        return defaultValue;
      }
      return deserializer(item) as T;
    } catch (error) {
      console.warn(`Failed to read sessionStorage key "${key}":`, error);
      return defaultValue;
    }
  };

  // Initialize state with lazy initialization
  const [storedValue, setStoredValue] = useState<T>(readValue);

  // Keep a ref to always access the latest storedValue without stale closure.
  // Updated via useEffect (after render) to satisfy react-hooks/refs rules.
  const storedValueRef = useRef(storedValue);
  useEffect(() => {
    storedValueRef.current = storedValue;
  }, [storedValue]);

  /**
   * Write value to sessionStorage with error handling
   */
  const setValue = useCallback(
    (value: T | ((prev: T) => T)) => {
      try {
        // Read latest value from ref to avoid stale closure
        const valueToStore =
          value instanceof Function ? value(storedValueRef.current) : value;

        // Update state
        setStoredValue(valueToStore);

        // Persist to sessionStorage
        sessionStorage.setItem(key, serializer(valueToStore));
      } catch (error) {
        console.warn(`Failed to write sessionStorage key "${key}":`, error);
      }
    },
    [key, serializer],
  );

  /**
   * Remove value from sessionStorage
   */
  const removeValue = useCallback(() => {
    try {
      sessionStorage.removeItem(key);
      setStoredValue(defaultValue);
    } catch (error) {
      console.warn(`Failed to remove sessionStorage key "${key}":`, error);
    }
  }, [key, defaultValue]);

  return [storedValue, setValue, removeValue];
}
