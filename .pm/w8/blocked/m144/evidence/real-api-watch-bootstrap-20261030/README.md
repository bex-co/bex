# Real API watch bootstrap — 2026-10-30

A real isolated Kubernetes v1.34.1 API exposed an empty-list bootstrap bug that the earlier localhost mock did not: kubectl aggregation discarded collection metadata and the observer refused to start. The observer now reads only its two fixed, name-filtered API collections, retains their actual resource versions, bounds response size/cardinality and projects private fields before handing off to the existing streams. Obsolete JSONPath schema is removed; the test fake independently declares its selected Job defaults.

The final 35 tests pass. Both real App and empty Job watch handshakes receive initial-state bookmarks, and child processes/threads are reaped. [The review](review.json) retains the original real-server failure and transitional fake-schema test failures separately from final success. The private kubeconfig was removed and is excluded from this archive. No production API or paid fixture was used.

[The Kubernetes API contract](https://v1-34.docs.kubernetes.io/docs/reference/using-api/api-concepts/#streaming-lists) defines the initial-state bookmark and resource-version synchronization used here. Production rollout and the original owned lifecycle walkthrough remain required.
