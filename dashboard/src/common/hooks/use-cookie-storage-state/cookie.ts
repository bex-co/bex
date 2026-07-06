import Cookies from "js-cookie";
import { createIsomorphicFn } from "@tanstack/react-start";
import {
  getCookie as serverGetCookie,
  setCookie as serverSetCookie,
  deleteCookie as serverDeleteCookie,
} from "@tanstack/react-start/server";

/**
 * Isomorphic cookie getter.
 * - Server: reads from the incoming request via @tanstack/react-start/server
 * - Client: delegates to js-cookie
 */
export const getCookie = createIsomorphicFn()
  .server((key: string) => serverGetCookie(key))
  .client((key: string) => Cookies.get(key));

/**
 * Isomorphic cookie setter.
 * - Server: no-op (Set-Cookie must be set on the response, not here)
 * - Client: delegates to js-cookie
 */
export const setCookie = createIsomorphicFn()
  .server(
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    (_key: string, _value: string, _options?: Cookies.CookieAttributes) => {
      serverSetCookie(_key, _value);
    },
  )
  .client((key: string, value: string, options?: Cookies.CookieAttributes) => {
    Cookies.set(key, value, options);
  });

/**
 * Isomorphic cookie remover.
 * - Server: no-op
 * - Client: delegates to js-cookie
 */
export const removeCookie = createIsomorphicFn()
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  .server((_key: string, _options?: Cookies.CookieAttributes) => {
    serverDeleteCookie(_key);
  })
  .client((key: string, options?: Cookies.CookieAttributes) => {
    Cookies.remove(key, options);
  });
