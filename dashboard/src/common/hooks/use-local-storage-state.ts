import { useState, useCallback, useEffect, useRef } from "react";

/**
 * Options for useLocalStorageState hook
 */
export interface UseLocalStorageStateOptions<T> {
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

  /**
   * Whether to sync state across tabs/windows
   * @default false
   */
  syncAcrossTabs?: boolean;
}

/**
 * A React hook that syncs state with localStorage
 *
 * Features:
 * - Persists state to localStorage automatically
 * - Lazy initialization from localStorage
 * - Optional cross-tab synchronization
 * - Graceful error handling for localStorage failures
 * - Type-safe with TypeScript generics
 * - Customizable serialization/deserialization
 *
 * @param key - localStorage key
 * @param defaultValue - default value if key doesn't exist
 * @param options - configuration options
 * @returns [value, setValue, removeValue] - stateful value and setter
 *
 * @example
 * ```tsx
 * // Simple boolean state
 * const [isDismissed, setIsDismissed] = useLocalStorageState('banner-dismissed', false);
 *
 * // Complex object state
 * const [user, setUser] = useLocalStorageState('user', { name: '', email: '' });
 *
 * // With custom serialization
 * const [date, setDate] = useLocalStorageState('last-visit', new Date(), {
 *   serializer: (d) => d.toISOString(),
 *   deserializer: (s) => new Date(s),
 * });
 *
 * // With cross-tab sync
 * const [theme, setTheme] = useLocalStorageState('theme', 'light', {
 *   syncAcrossTabs: true,
 * });
 * ```
 */
export function useLocalStorageState<T>(
  key: string,
  defaultValue: T,
  options: UseLocalStorageStateOptions<T> = {},
): [T, (value: T | ((prev: T) => T)) => void, () => void] {
  const {
    serializer = JSON.stringify,
    deserializer = JSON.parse,
    syncAcrossTabs = false,
  } = options;

  /**
   * Read value from localStorage with error handling
   */
  const readValue = (): T => {
    try {
      const item = localStorage.getItem(key);
      if (item === null) {
        return defaultValue;
      }
      return deserializer(item) as T;
    } catch (error) {
      console.warn(`Failed to read localStorage key "${key}":`, error);
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
   * Write value to localStorage with error handling
   */
  const setValue = useCallback(
    (value: T | ((prev: T) => T)) => {
      try {
        // Read latest value from ref to avoid stale closure
        const valueToStore =
          value instanceof Function ? value(storedValueRef.current) : value;

        // Update state
        setStoredValue(valueToStore);

        // Persist to localStorage
        localStorage.setItem(key, serializer(valueToStore));

        // Dispatch custom event for cross-tab sync
        if (syncAcrossTabs) {
          window.dispatchEvent(
            new CustomEvent("local-storage-change", {
              detail: { key, value: valueToStore },
            }),
          );
        }
      } catch (error) {
        console.warn(`Failed to write localStorage key "${key}":`, error);
      }
    },
    [key, serializer, syncAcrossTabs],
  );

  /**
   * Remove value from localStorage
   */
  const removeValue = useCallback(() => {
    try {
      localStorage.removeItem(key);
      setStoredValue(defaultValue);

      // Dispatch custom event for cross-tab sync
      if (syncAcrossTabs) {
        window.dispatchEvent(
          new CustomEvent("local-storage-change", {
            detail: { key, value: null },
          }),
        );
      }
    } catch (error) {
      console.warn(`Failed to remove localStorage key "${key}":`, error);
    }
  }, [key, defaultValue, syncAcrossTabs]);

  /**
   * Listen for storage events (cross-tab sync)
   */
  useEffect(() => {
    if (!syncAcrossTabs) return;

    const handleStorageChange = (e: StorageEvent | CustomEvent) => {
      // Handle native storage event (other tabs)
      if (e instanceof StorageEvent) {
        if (e.key === key && e.newValue !== null) {
          try {
            setStoredValue(deserializer(e.newValue));
          } catch (error) {
            console.warn(
              `Failed to deserialize storage event for key "${key}":`,
              error,
            );
          }
        } else if (e.key === key && e.newValue === null) {
          setStoredValue(defaultValue);
        }
      }
      // Handle custom event (same tab)
      else if ("detail" in e) {
        const detail = e.detail as { key: string; value: T | null };
        if (detail.key === key) {
          setStoredValue(detail.value ?? defaultValue);
        }
      }
    };

    // Listen for both native storage events and custom events
    window.addEventListener("storage", handleStorageChange as EventListener);
    window.addEventListener(
      "local-storage-change",
      handleStorageChange as EventListener,
    );

    return () => {
      window.removeEventListener(
        "storage",
        handleStorageChange as EventListener,
      );
      window.removeEventListener(
        "local-storage-change",
        handleStorageChange as EventListener,
      );
    };
  }, [key, defaultValue, deserializer, syncAcrossTabs]);

  return [storedValue, setValue, removeValue];
}
