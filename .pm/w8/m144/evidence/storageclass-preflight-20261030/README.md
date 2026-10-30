# Fixed StorageClass preflight

Inventory now requests only `hcloud-volumes-luks` and `hcloud-volumes-luks-bounded`. It reports their names/UIDs/deletion state, provisioner, expansion/binding/reclaim policy, filesystem/mode and four exact publish/expand Secret-reference templates. Missing classes are explicit; unexpected/duplicate objects and invalid expansion values fail. No Secret data, arbitrary parameters or cluster mutation is added.

Thirty unit tests and both shell protocols pass. Real kubectl v1.36.1 against an isolated localhost discovery/StorageClass server verifies both-present, bounded-missing and both-missing behavior; private annotation/parameter canaries never reach the projected TSV/JSON. The proof records only those fixed mock GETs. Root reviewed the selected fields and missing-object path. This is preparation evidence, not a production class observation.

Before the all-workspace class switch, inspect the next production inventory for the actual bounded class UID and exact encryption/filesystem/Secret-template contract, as well as the newer Ready operator. GitOps source alone does not establish that the class has converged. Original production full/grow/remount/restore and cleanup acceptance remain open.
