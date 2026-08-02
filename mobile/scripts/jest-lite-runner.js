#!/usr/bin/env node
const fs = require("fs");
const path = require("path");
const ts = require("typescript");
const util = require("util");
const os = require("os");
const { fork } = require("child_process");

const defaultRoots = ["src", "app"];
const testFilePatterns = [".test.ts", ".test.tsx", ".spec.ts", ".spec.tsx"];

if (process.env.JEST_LITE_WORKER === "1") {
  runWorker().catch((error) => {
    console.error(
      "Unexpected error while running tests:\n",
      error && (error.stack || error.message || error),
    );
    process.exit(1);
  });
} else {
  orchestrate().catch((error) => {
    console.error(
      "Unexpected error while orchestrating tests:\n",
      error && (error.stack || error.message || error),
    );
    process.exit(1);
  });
}

async function orchestrate() {
  const projectRoot = process.cwd();
  const cliRoots = process.argv.slice(2);
  const searchRoots = cliRoots.length > 0 ? cliRoots : defaultRoots;
  const collectedFiles = collectAllTestFiles(searchRoots, projectRoot);

  if (collectedFiles.length === 0) {
    console.log("No test files found.");
    return;
  }

  const maxWorkers = determineMaxWorkers(collectedFiles.length);
  const aggregated = { total: 0, passed: 0, failed: 0, skipped: 0 };
  let hasFailure = false;

  await new Promise((resolve) => {
    let nextIndex = 0;
    let running = 0;

    const startNext = () => {
      if (nextIndex >= collectedFiles.length) {
        if (running === 0) {
          resolve();
        }
        return;
      }

      const file = collectedFiles[nextIndex++];
      running += 1;

      const child = fork(__filename, [], {
        env: {
          ...process.env,
          JEST_LITE_WORKER: "1",
          JEST_LITE_TARGET: file,
        },
        stdio: ["inherit", "inherit", "inherit", "ipc"],
      });

      child.on("message", (message) => {
        if (!message || message.type !== "summary") {
          return;
        }
        const summary = message.summary || {};
        aggregated.total += summary.total || 0;
        aggregated.passed += summary.passed || 0;
        aggregated.failed += summary.failed || 0;
        aggregated.skipped += summary.skipped || 0;
      });

      child.on("exit", (code) => {
        if (code !== 0) {
          hasFailure = true;
        }
        running -= 1;
        if (nextIndex < collectedFiles.length) {
          startNext();
        } else if (running === 0) {
          resolve();
        }
      });

      child.on("error", (error) => {
        console.error(
          `Failed to run tests in ${path.relative(projectRoot, file)}:`,
        );
        console.error(error && (error.stack || error.message || error));
        hasFailure = true;
        running -= 1;
        if (nextIndex < collectedFiles.length) {
          startNext();
        } else if (running === 0) {
          resolve();
        }
      });
    };

    const initialWorkers = Math.min(maxWorkers, collectedFiles.length);
    for (let i = 0; i < initialWorkers; i += 1) {
      startNext();
    }
  });

  const summaryParts = [`${aggregated.passed} passed`];
  if (aggregated.failed > 0) {
    summaryParts.push(`${aggregated.failed} failed`);
  }
  const computedSkipped =
    aggregated.skipped > 0
      ? aggregated.skipped
      : Math.max(aggregated.total - aggregated.passed - aggregated.failed, 0);
  if (computedSkipped > 0) {
    summaryParts.push(`${computedSkipped} skipped`);
  }
  console.log(`\nTest Summary: ${summaryParts.join(", ")}`);

  process.exit(hasFailure ? 1 : 0);
}

async function runWorker() {
  const projectRoot = process.cwd();
  const targetEnv = process.env.JEST_LITE_TARGET;

  const collectedFiles = [];
  if (targetEnv) {
    for (const file of targetEnv.split(path.delimiter)) {
      if (!file) {
        continue;
      }
      const absoluteFile = path.isAbsolute(file)
        ? file
        : path.resolve(projectRoot, file);
      collectedFiles.push(absoluteFile);
    }
  } else {
    const cliRoots = process.argv.slice(2);
    const searchRoots = cliRoots.length > 0 ? cliRoots : defaultRoots;
    collectedFiles.push(...collectAllTestFiles(searchRoots, projectRoot));
  }

  if (collectedFiles.length === 0) {
    console.log("No test files found.");
    await reportAndExit(createRunnerContext(), null);
    return;
  }

  const tsCompilerOptions = {
    module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2019,
    jsx: ts.JsxEmit.React,
    esModuleInterop: true,
    allowSyntheticDefaultImports: true,
    resolveJsonModule: true,
    isolatedModules: false,
    sourceMap: false,
  };

  const compiler = createTsCompiler(tsCompilerOptions);
  const originalTsExtension = require.extensions[".ts"];
  const originalTsxExtension = require.extensions[".tsx"];

  require.extensions[".ts"] = compiler;
  require.extensions[".tsx"] = compiler;

  const context = createRunnerContext();

  setupGlobals(context);
  loadTestFiles(context, collectedFiles, projectRoot);

  let unexpectedError = null;
  try {
    await runTests(context);
  } catch (error) {
    unexpectedError = error;
  } finally {
    restoreExtensions(originalTsExtension, originalTsxExtension);
  }

  await reportAndExit(context, unexpectedError);
}

function collectAllTestFiles(searchRoots, projectRoot) {
  const files = [];
  for (const root of searchRoots) {
    const absoluteRoot = path.resolve(projectRoot, root);
    if (fs.existsSync(absoluteRoot)) {
      collectTestFilesRecursively(absoluteRoot, files);
    }
  }
  return files;
}

function collectTestFilesRecursively(directory, accumulator) {
  const entries = fs.readdirSync(directory, { withFileTypes: true });

  for (const entry of entries) {
    if (entry.name === "node_modules" || entry.name.startsWith(".")) {
      continue;
    }

    const absolutePath = path.join(directory, entry.name);

    if (entry.isDirectory()) {
      collectTestFilesRecursively(absolutePath, accumulator);
      continue;
    }

    if (!entry.isFile()) {
      continue;
    }

    if (testFilePatterns.some((pattern) => entry.name.endsWith(pattern))) {
      accumulator.push(absolutePath);
    }
  }
}

function determineMaxWorkers(totalFiles) {
  const envValue = process.env.JEST_LITE_MAX_WORKERS;
  if (envValue) {
    const parsed = Number(envValue);
    if (Number.isFinite(parsed) && parsed > 0) {
      return Math.min(Math.floor(parsed), totalFiles);
    }
  }
  const cpuCount = os.cpus ? os.cpus().length : 1;
  return Math.max(1, Math.min(cpuCount, totalFiles));
}

function createRunnerContext() {
  const rootSuite = createSuite("");
  return {
    rootSuite,
    suiteStack: [rootSuite],
    totalTests: 0,
    passedTests: 0,
    failedTests: 0,
  };
}

function setupGlobals(context) {
  const nowSuite = () => context.suiteStack[context.suiteStack.length - 1];

  const addTest = (name, fn, mode = "run") => {
    const suite = nowSuite();
    const testNode = {
      type: "test",
      name,
      fn,
      mode,
    };
    suite.children.push(testNode);
    context.totalTests += 1;
  };

  global.describe = (name, fn) => {
    const suite = createSuite(name);
    nowSuite().children.push(suite);
    context.suiteStack.push(suite);
    try {
      fn();
    } finally {
      context.suiteStack.pop();
    }
  };

  global.it = global.test = (name, fn) => addTest(name, fn);
  global.it.skip = global.test.skip = (name, fn) => addTest(name, fn, "skip");
  global.it.only = global.test.only = (name, fn) => addTest(name, fn, "only");

  global.describe.skip = (name, fn) => {
    const suite = createSuite(name);
    suite.mode = "skip";
    nowSuite().children.push(suite);
    context.suiteStack.push(suite);
    try {
      fn();
    } finally {
      context.suiteStack.pop();
    }
  };

  global.describe.only = (name, fn) => {
    const suite = createSuite(name);
    suite.mode = "only";
    nowSuite().children.push(suite);
    context.suiteStack.push(suite);
    try {
      fn();
    } finally {
      context.suiteStack.pop();
    }
  };

  const hook = (name) => (fn) => {
    if (typeof fn !== "function") {
      throw new TypeError(`${name} expects a function`);
    }
    nowSuite()[name].push(fn);
  };

  global.beforeAll = hook("beforeAll");
  global.afterAll = hook("afterAll");
  global.beforeEach = hook("beforeEach");
  global.afterEach = hook("afterEach");

  global.expect = createExpect();
}

function loadTestFiles(context, files, projectRoot) {
  for (const file of files) {
    const relativePath = path.relative(projectRoot, file);
    try {
      require(file);
    } catch (error) {
      const suite = context.suiteStack[context.suiteStack.length - 1];
      suite.children.push({
        type: "test",
        name: `Failed to load ${relativePath}`,
        fn: () => {
          throw error;
        },
      });
      context.totalTests += 1;
    }
  }
}

async function runTests(context) {
  const onlyNodes = findOnlyNodes(context.rootSuite);

  if (onlyNodes.length > 0) {
    applyOnlyMode(context.rootSuite, new Set(onlyNodes));
  }

  await runSuite(context, context.rootSuite, []);
}

async function runSuite(
  context,
  suite,
  ancestors,
  parentHooks = { beforeEach: [], afterEach: [] },
) {
  const namePath = suite.name ? [...ancestors, suite.name] : [...ancestors];

  const combinedBeforeEach = [...parentHooks.beforeEach, ...suite.beforeEach];
  const combinedAfterEach = [...suite.afterEach, ...parentHooks.afterEach];

  if (suite.mode === "skip") {
    skipSuite(suite, namePath);
    return;
  }

  await runHooksSequentially(context, suite.beforeAll, namePath, "beforeAll");

  for (const child of suite.children) {
    if (child.type === "suite") {
      await runSuite(context, child, namePath, {
        beforeEach: combinedBeforeEach,
        afterEach: combinedAfterEach,
      });
    } else {
      await runTest(
        context,
        child,
        namePath,
        combinedBeforeEach,
        combinedAfterEach,
      );
    }
  }

  await runHooksSequentially(context, suite.afterAll, namePath, "afterAll");
}

function skipSuite(suite, ancestors) {
  for (const child of suite.children) {
    if (child.type === "suite") {
      skipSuite(child, [...ancestors, suite.name]);
    } else {
      console.log(
        `○ ${formatTestName([...ancestors, suite.name, child.name])}`,
      );
    }
  }
}

function findOnlyNodes(node) {
  const results = [];

  if (node.type === "test" && node.mode === "only") {
    results.push(node);
  }

  if (node.type === "suite" && node.mode === "only") {
    results.push(node);
  }

  if (node.type === "suite") {
    for (const child of node.children) {
      results.push(...findOnlyNodes(child));
    }
  }

  return results;
}

function applyOnlyMode(node, onlySet, forceRun = false) {
  if (node.type === "test") {
    const shouldRun = forceRun || onlySet.has(node);
    if (!shouldRun) {
      node.mode = "skip";
    }
    return shouldRun;
  }

  if (node.type === "suite") {
    const isOnlySuite = forceRun || onlySet.has(node);
    let hasRunnableChild = false;
    for (const child of node.children) {
      const childRunnable = applyOnlyMode(child, onlySet, isOnlySuite);
      hasRunnableChild = hasRunnableChild || childRunnable;
    }

    if (!isOnlySuite && !hasRunnableChild) {
      node.mode = "skip";
      return false;
    }

    return true;
  }

  return forceRun;
}

async function runHooksSequentially(context, hooks, namePath, hookName) {
  for (const hook of hooks) {
    try {
      await hook();
    } catch (error) {
      context.failedTests += 1;
      console.error(`✗ ${formatTestName([...namePath, `${hookName} hook`])}`);
      printError(error);
    }
  }
}

async function runTest(
  context,
  testNode,
  ancestors,
  beforeEachHooks,
  afterEachHooks,
) {
  if (testNode.mode === "skip") {
    console.log(`○ ${formatTestName([...ancestors, testNode.name])}`);
    return;
  }

  for (const hook of beforeEachHooks) {
    try {
      await hook();
    } catch (error) {
      context.failedTests += 1;
      console.error(`✗ ${formatTestName([...ancestors, testNode.name])}`);
      console.error("Error in beforeEach hook:");
      printError(error);
      return;
    }
  }

  try {
    await testNode.fn();
    context.passedTests += 1;
    console.log(`✓ ${formatTestName([...ancestors, testNode.name])}`);
  } catch (error) {
    context.failedTests += 1;
    console.error(`✗ ${formatTestName([...ancestors, testNode.name])}`);
    printError(error);
  }

  for (const hook of afterEachHooks) {
    try {
      await hook();
    } catch (error) {
      context.failedTests += 1;
      console.error(`✗ ${formatTestName([...ancestors, testNode.name])}`);
      console.error("Error in afterEach hook:");
      printError(error);
      return;
    }
  }
}

function createSuite(name) {
  return {
    type: "suite",
    name,
    children: [],
    beforeAll: [],
    afterAll: [],
    beforeEach: [],
    afterEach: [],
  };
}

function createTsCompiler(compilerOptions) {
  return (module, filename) => {
    if (filename.endsWith(".d.ts")) {
      module._compile("", filename);
      return;
    }

    const source = fs.readFileSync(filename, "utf8");
    const output = ts.transpileModule(source, {
      fileName: filename,
      compilerOptions,
      reportDiagnostics: true,
    });

    if (output.diagnostics && output.diagnostics.length > 0) {
      for (const diagnostic of output.diagnostics) {
        const message = ts.flattenDiagnosticMessageText(
          diagnostic.messageText,
          "\n",
        );
        if (diagnostic.file) {
          const { line, character } =
            diagnostic.file.getLineAndCharacterOfPosition(
              diagnostic.start || 0,
            );
          console.warn(
            `${diagnostic.file.fileName} (${line + 1},${character + 1}): ${message}`,
          );
        } else {
          console.warn(message);
        }
      }
    }

    module._compile(output.outputText, filename);
  };
}

function restoreExtensions(originalTsExtension, originalTsxExtension) {
  if (originalTsExtension) {
    require.extensions[".ts"] = originalTsExtension;
  } else {
    delete require.extensions[".ts"];
  }

  if (originalTsxExtension) {
    require.extensions[".tsx"] = originalTsxExtension;
  } else {
    delete require.extensions[".tsx"];
  }
}

function createExpect() {
  const format = (value) => util.inspect(value, { depth: null, colors: false });

  const makeMatchers = (received, isNot = false) => {
    const ensure = (condition, message, invertedMessage) => {
      if (isNot ? condition : !condition) {
        throw new Error(isNot ? invertedMessage : message);
      }
    };

    const compareEquality = (expected, comparator, description) => {
      const result = comparator(received, expected);
      const expectedText = format(expected);
      const receivedText = format(received);
      ensure(
        result,
        `Expected ${receivedText} to ${description} ${expectedText}`,
        `Expected ${receivedText} not to ${description} ${expectedText}`,
      );
    };

    const matchers = {
      toBe(expected) {
        compareEquality(expected, Object.is, "be");
      },
      toEqual(expected) {
        compareEquality(
          expected,
          (a, b) => util.isDeepStrictEqual(a, b),
          "deep-equal",
        );
      },
      toContain(expected) {
        const result =
          typeof received === "string"
            ? received.includes(expected)
            : Array.isArray(received) && received.includes(expected);
        ensure(
          result,
          `Expected ${format(received)} to contain ${format(expected)}`,
          `Expected ${format(received)} not to contain ${format(expected)}`,
        );
      },
      toBeCloseTo(expected, precision = 2) {
        const factor = Math.pow(10, precision);
        const result =
          Math.round(Number(received) * factor) ===
          Math.round(Number(expected) * factor);
        ensure(
          result,
          `Expected ${received} to be close to ${expected} (precision ${precision})`,
          `Expected ${received} not to be close to ${expected} (precision ${precision})`,
        );
      },
      toBeTruthy() {
        ensure(
          Boolean(received),
          `Expected ${received} to be truthy`,
          `Expected ${received} not to be truthy`,
        );
      },
      toBeFalsy() {
        ensure(
          !received,
          `Expected ${received} to be falsy`,
          `Expected ${received} not to be falsy`,
        );
      },
      toThrow(expected) {
        if (typeof received !== "function") {
          throw new Error(
            "toThrow matcher requires a function as the expectation target",
          );
        }
        let thrownError;
        try {
          received();
        } catch (error) {
          thrownError = error;
        }
        const didThrow = Boolean(thrownError);
        if (!expected) {
          ensure(
            didThrow,
            "Expected function to throw an error",
            "Expected function not to throw an error",
          );
          return;
        }
        if (!didThrow) {
          throw new Error(
            `Expected function to throw ${format(expected)}, but it did not throw`,
          );
        }
        if (typeof expected === "string") {
          ensure(
            thrownError && thrownError.message === expected,
            `Expected error message to be '${expected}' but received '${thrownError.message}'`,
            `Expected error message not to be '${expected}'`,
          );
          return;
        }
        if (expected instanceof RegExp) {
          ensure(
            expected.test(thrownError.message || ""),
            `Expected error message to match ${expected}`,
            `Expected error message not to match ${expected}`,
          );
          return;
        }
        if (typeof expected === "function") {
          ensure(
            thrownError instanceof expected,
            `Expected thrown error to be instance of ${expected.name}`,
            `Expected thrown error not to be instance of ${expected.name}`,
          );
          return;
        }
        throw new TypeError("Unsupported argument for expect(...).toThrow");
      },
    };

    Object.defineProperty(matchers, "not", {
      get() {
        return makeMatchers(received, !isNot);
      },
    });

    return matchers;
  };

  return (received) => makeMatchers(received);
}

function formatTestName(parts) {
  return parts.filter(Boolean).join(" › ");
}

function printError(error) {
  if (!error) {
    console.error("Unknown error");
    return;
  }
  if (error.stack) {
    const stackLines = error.stack.split("\n");
    console.error("  " + stackLines.join("\n  "));
  } else {
    console.error("  " + (error.message || error));
  }
}

function buildSummary(context) {
  const skipped = Math.max(
    context.totalTests - context.passedTests - context.failedTests,
    0,
  );
  return {
    total: context.totalTests,
    passed: context.passedTests,
    failed: context.failedTests,
    skipped,
  };
}

function reportAndExit(context, unexpectedError) {
  return new Promise((resolve) => {
    if (unexpectedError) {
      console.error(
        "Unexpected error while running tests:\n",
        unexpectedError &&
          (unexpectedError.stack || unexpectedError.message || unexpectedError),
      );
    }

    const summary = buildSummary(context);
    const exitCode = unexpectedError || context.failedTests > 0 ? 1 : 0;

    if (typeof process.send === "function") {
      process.send({ type: "summary", summary }, () => {
        process.exit(exitCode);
        resolve();
      });
      return;
    }

    const summaryParts = [`${summary.passed} passed`];
    if (summary.failed > 0) {
      summaryParts.push(`${summary.failed} failed`);
    }
    if (summary.skipped > 0) {
      summaryParts.push(`${summary.skipped} skipped`);
    }
    console.log(`\nTest Summary: ${summaryParts.join(", ")}`);
    process.exit(exitCode);
    resolve();
  });
}
