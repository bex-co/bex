// Copyright 2026 Tian Pan
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

export class PaymentOnboardingCancelledError extends Error {
  constructor() {
    super("Payment onboarding was cancelled");
    this.name = "PaymentOnboardingCancelledError";
  }
}

export function isPaymentOnboardingCancelled(
  error: unknown,
): error is PaymentOnboardingCancelledError {
  return error instanceof PaymentOnboardingCancelledError;
}
