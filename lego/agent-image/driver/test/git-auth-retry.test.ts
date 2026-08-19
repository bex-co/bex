/*
 * Copyright 2026.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import assert from "node:assert/strict";
import test from "node:test";
import { withGitAuthRetry } from "../src/git-auth-retry.js";

test("withGitAuthRetry returns on the first success", async () => {
  let calls = 0;
  const value = await withGitAuthRetry(async () => {
    calls += 1;
    return "ok";
  });
  assert.equal(value, "ok");
  assert.equal(calls, 1);
});

test("withGitAuthRetry retries a transient git-proxy 403 then succeeds", async () => {
  // w2/m77: restored workspaces skip cloneWithRetry and hit ls-remote while
  // sandbox_id is still empty; one retry after the mint broker sees the id is
  // enough. delayMs=0 keeps the unit test off the production 3s backoff.
  let calls = 0;
  const value = await withGitAuthRetry(
    async () => {
      calls += 1;
      if (calls < 3) {
        const err = new Error("remote: forbidden");
        throw err;
      }
      return "cloned";
    },
    { delayMs: 0 },
  );
  assert.equal(value, "cloned");
  assert.equal(calls, 3);
});

test("withGitAuthRetry exhausts attempts and rethrows the last error", async () => {
  let calls = 0;
  await assert.rejects(
    () =>
      withGitAuthRetry(
        async () => {
          calls += 1;
          throw new Error(`forbidden attempt ${calls}`);
        },
        { attempts: 4, delayMs: 0 },
      ),
    /forbidden attempt 4/,
  );
  assert.equal(calls, 4);
});

test("withGitAuthRetry runs onRetry between attempts", async () => {
  const cleaned: number[] = [];
  let calls = 0;
  await withGitAuthRetry(
    async () => {
      calls += 1;
      if (calls === 1) throw new Error("partial clone");
      return "ok";
    },
    {
      delayMs: 0,
      onRetry: async () => {
        cleaned.push(calls);
      },
    },
  );
  assert.deepEqual(cleaned, [1]);
});
