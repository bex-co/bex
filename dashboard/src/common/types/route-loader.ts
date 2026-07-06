import type {
  ResolveParams,
  ParsedLocation,
  NavigateFn,
  BuildLocationFn,
} from "@tanstack/react-router";
import type { RouterContext } from "@/common/types/router-context";

/**
 * Utility type for typing route loader functions by route path.
 *
 * Derives the params type directly from the path string — no dependency on
 * the registered router, so there's no circular reference with routeTree.gen.
 *
 * @example
 * ```ts
 * export const myLoader: RouteLoader<"/services/$serviceId"> = async ({ params, context }) => {
 *   // params.serviceId — typed
 *   // context.client — typed from RouterContext
 * };
 * ```
 */
export type RouteLoader<TPath extends string, T = void> = (ctx: {
  params: ResolveParams<TPath>;
  context: RouterContext;
  abortController: AbortController;
  preload: boolean;
  cause: "preload" | "enter" | "stay";
}) => T | Promise<T>;

/**
 * Utility type for typing route beforeLoad functions by route path.
 *
 * Derives the params type directly from the path string — no dependency on
 * the registered router, so there's no circular reference with routeTree.gen.
 *
 * Use the optional `TSearch` type parameter to type the validated search params
 * when the route uses `validateSearch`.
 *
 * @example
 * ```ts
 * export const myBeforeLoader: RouteBeforeLoader<"/services/$serviceId"> = ({ params, context, search }) => {
 *   // params.serviceId — typed
 *   // context.client — typed from RouterContext
 *   // search — Record<string, unknown> by default
 * };
 *
 * // With typed search params:
 * export const myBeforeLoader: RouteBeforeLoader<"/login", { redirect?: string }> = ({ search }) => { ... };
 * ```
 */
export type RouteBeforeLoader<
  TPath extends string,
  TSearch extends Record<string, unknown> = Record<string, unknown>,
> = (ctx: {
  params: ResolveParams<TPath>;
  context: RouterContext;
  search: TSearch;
  location: ParsedLocation;
  /** @deprecated Use `throw redirect({ to: '/somewhere' })` instead */
  navigate: NavigateFn;
  buildLocation: BuildLocationFn;
  abortController: AbortController;
  preload: boolean;
  cause: "preload" | "enter" | "stay";
}) => void | Promise<void>;
