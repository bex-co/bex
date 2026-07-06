import { useState, useCallback, useRef, useEffect } from "react";
import { getCookie, setCookie, removeCookie } from "./cookie";

export interface UseCookieStorageStateOptions<T> {
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
   * Cookie options passed to js-cookie (expires, path, domain, secure, sameSite)
   */
  cookieOptions?: Cookies.CookieAttributes;
}

/**
 * A React hook that syncs state with a cookie.
 *
 * Uses the isomorphic getCookie/setCookie/removeCookie helpers so the hook is
 * safe to call in SSR environments — reads work on both server and client,
 * while writes are no-ops on the server.
 *
 * @param key - cookie name
 * @param defaultValue - default value if cookie doesn't exist
 * @param options - configuration options
 * @returns [value, setValue, removeValue]
 *
 * @example
 * ```tsx
 * const [theme, setTheme, clearTheme] = useCookieStorageState('theme', 'system');
 *
 * const [dismissed, setDismissed] = useCookieStorageState('onboarding-dismissed', false, {
 *   cookieOptions: { expires: 7, secure: true },
 * });
 * ```
 */
export function useCookieStorageState<T>(
  key: string,
  defaultValue: T,
  options: UseCookieStorageStateOptions<T> = {},
): [T, (value: T | ((prev: T) => T)) => void, () => void] {
  const {
    serializer = JSON.stringify,
    deserializer = JSON.parse,
    cookieOptions,
  } = options;

  const readValue = (): T => {
    try {
      const raw = getCookie(key);
      if (raw === undefined) return defaultValue;
      return deserializer(raw) as T;
    } catch {
      return defaultValue;
    }
  };

  const [storedValue, setStoredValue] = useState<T>(readValue);

  const storedValueRef = useRef(storedValue);
  useEffect(() => {
    storedValueRef.current = storedValue;
  }, [storedValue]);

  const setValue = useCallback(
    (value: T | ((prev: T) => T)) => {
      try {
        const valueToStore =
          value instanceof Function ? value(storedValueRef.current) : value;
        setStoredValue(valueToStore);
        setCookie(key, serializer(valueToStore), cookieOptions);
      } catch (error) {
        console.warn(`Failed to write cookie "${key}":`, error);
      }
    },
    [key, serializer, cookieOptions],
  );

  const removeValue = useCallback(() => {
    removeCookie(key, cookieOptions);
    setStoredValue(defaultValue);
  }, [key, defaultValue, cookieOptions]);

  return [storedValue, setValue, removeValue];
}
