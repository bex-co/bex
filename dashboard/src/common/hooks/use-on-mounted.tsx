import { useEffect, useRef } from "react";

/**
 * Hook that runs a callback function only once when the component mounts.
 * Supports both synchronous and asynchronous callbacks.
 *
 * @param callback - Function to execute on mount. Can optionally return a cleanup function (sync only).
 *
 * @example
 * ```tsx
 * useOnMounted(() => {
 *   console.log('Component mounted');
 * });
 * ```
 *
 * @example
 * ```tsx
 * useOnMounted(async () => {
 *   const data = await fetchData();
 *   console.log('Data loaded:', data);
 * });
 * ```
 *
 * @example
 * ```tsx
 * useOnMounted(() => {
 *   const timer = setInterval(() => {
 *     console.log('Timer tick');
 *   }, 1000);
 *
 *   return () => {
 *     clearInterval(timer);
 *   };
 * });
 * ```
 */
export function useOnMounted(
  callback: () => void | (() => void) | Promise<void>,
) {
  const mounted = useRef(false);
  useEffect(() => {
    if (mounted.current) return;
    mounted.current = true;
    const result = callback();
    // If callback returns a Promise, handle errors
    if (result instanceof Promise) {
      result.catch((error) => {
        console.error("Error in useOnMounted async callback:", error);
      });
      return;
    }
    // For sync callbacks, return cleanup function if provided
    return result;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []); // Intentionally empty - callback runs only once on mount, not when it changes
}
