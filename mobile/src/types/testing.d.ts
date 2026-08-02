declare type TestFunction = () => void | Promise<void>;

declare function describe(name: string, fn: () => void): void;
declare namespace describe {
  function only(name: string, fn: () => void): void;
  function skip(name: string, fn: () => void): void;
}

declare function it(name: string, fn: TestFunction): void;
declare namespace it {
  function only(name: string, fn: TestFunction): void;
  function skip(name: string, fn: TestFunction): void;
}

declare function test(name: string, fn: TestFunction): void;
declare namespace test {
  function only(name: string, fn: TestFunction): void;
  function skip(name: string, fn: TestFunction): void;
}

declare function beforeAll(fn: TestFunction): void;
declare function afterAll(fn: TestFunction): void;
declare function beforeEach(fn: TestFunction): void;
declare function afterEach(fn: TestFunction): void;

declare function expect<T>(value: T): Expectation<T>;

declare interface Expectation<T> {
  toBe(expected: T): void;
  toEqual(expected: unknown): void;
  toContain(expected: unknown): void;
  toBeCloseTo(expected: number, precision?: number): void;
  toBeTruthy(): void;
  toBeFalsy(): void;
  toThrow(expected?: string | RegExp | ErrorConstructor): void;
  readonly not: Expectation<T>;
}
