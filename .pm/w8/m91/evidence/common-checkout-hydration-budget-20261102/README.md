# Complete candidate checkout budget

[Run89](https://github.com/bex-co/bex/actions/runs/41449940993), source `b067de3c`, failed during evidence materialization. The initial source checkout completed in8seconds; the subsequent bounded Git sparse checkout hit its180second deadline with exit124. Strict selection and all utility tests never ran, and no artifact was produced. The full original log and terminal metadata remain unchanged. The underlying I/O or transport cause is unknown.

The workflow changes only the three matching evidence-checkout budgets: Git600seconds with the existing10second kill grace, inside a12minute step. Planning retains its15minute limit. This fits the previously observed493/523second complete checkouts while retaining every evidence packet, validation, metadata-client bound, runtime deadline, authentication and cleanup command. A later real run must establish completion.

All six existing Git controls, the workflow guard, configured actionlint and the Actions guard pass in3.012seconds total. Source301, the640-file check view and the foreign sum remain unchanged during checks; owned groups and TMP are gone. No Go/full-Python suite or runtime was repeated for this literal workflow-budget change.

[SUMMARY.json](SUMMARY.json) and [ROOT-CUSTODY.json](ROOT-CUSTODY.json) distinguish actual failure, applied source and host checks. The same0941 current-source utility baseline, remaining families, production adoption and original28 hosted acceptance stay open.
