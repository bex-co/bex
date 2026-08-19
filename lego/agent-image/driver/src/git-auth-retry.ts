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

// bex-api records a session's sandbox_id AFTER CreateAgentSessionSandbox
// returns (pod already running). The driver's first git-proxy call can therefore
// 403 ("remote: forbidden") until the mint broker sees the live sandbox id.
// Retrying with backoff is the established clone-path answer; restore/ensureRepo
// ls-remote/fetch hits the same race and must share it (w2/m77 live rehydrate).

export const GIT_AUTH_RETRY_ATTEMPTS = 10;
export const GIT_AUTH_RETRY_MS = 3000;

export async function withGitAuthRetry<T>(
  op: () => Promise<T>,
  opts?: { attempts?: number; delayMs?: number; onRetry?: () => Promise<void> },
): Promise<T> {
  const attempts = opts?.attempts ?? GIT_AUTH_RETRY_ATTEMPTS;
  const delayMs = opts?.delayMs ?? GIT_AUTH_RETRY_MS;
  let lastError: unknown;
  for (let attempt = 1; attempt <= attempts; attempt += 1) {
    try {
      return await op();
    } catch (error) {
      lastError = error;
      if (attempt === attempts) break;
      if (opts?.onRetry) await opts.onRetry();
      if (delayMs > 0) {
        await new Promise((resolve) => setTimeout(resolve, delayMs));
      }
    }
  }
  throw lastError;
}
