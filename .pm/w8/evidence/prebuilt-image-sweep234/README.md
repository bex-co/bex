# Prebuilt-image deploy and instance formats — sweep234

Published Bex 0.3.2 and unchanged Render 2.27.0/a764810a7682, production API, isolated human QA session in bex-canary/tea-daif693dqjvc73e7as3g. Source 195e2ebb1f1a086180bc674928d9c05a45a86153; deployed API/operator revision unknown. No product implementation, QA commit/push or external filing.

Both native clients create a fresh owned Free web service from the public immutable mendhak/http-https-echo reference recorded in the [ledger](owned-native-runtime-cleanup.json), with HTTP_PORT/WHOAMI_PORT_NUMBER=3000 and auto-deploy disabled. Both become Live and their own nonce path plus X-QA-Marker echo correctly over HTTP200.

Each client performs one default deploys create and one deploys create --image <same-pinned-reference>, both with --wait. All four native processes exit0, take about 16–32 seconds and end Live; [commands, complete output and bounded waits](native-wait-deploys.json). Final metadata still reports the configured imagePath, no Git repo and runtime/env=image. The final runtime retains each request marker. Native POSTs were not intercepted; complete native results and readback are preserved, so exact mutation-wire capture is not claimed.

Each client runs services instances in JSON, YAML and text. Both report one owned instance; parsed YAML equals JSON exactly, text retains every ID and each ID belongs to its own service. [Comparisons](instance-format-comparison.json). Native output copies are in the ledger. This does not exercise SSH or certify its authentication.

Maintenance mode was excluded after reading the pinned compatibility ledger's paid-service requirement; no paid plan or maintenance mutation is attempted. No Git/image transition, w8/088 acceptance, browser/shared-surface parity, deployed OCI digest verification or full suite is claimed. No new defect is confirmed. [Metadata](metadata.json), [finite driver](native-image-driver.py.txt).

Both exact-owned services and 6 recorded deploy rows are removed after enumeration. Native delete acknowledgments are followed by detail/list/runtime absence, baseline service presence and quota termination zero. No other hosted resource or persistent Blueprint is created. Actual credential/email and incidental client-address values are redacted; harmless request markers remain. The ongoing hunt retains its isolated session.
