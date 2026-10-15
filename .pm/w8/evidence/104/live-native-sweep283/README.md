# w8/104 — live behavior remains unaccepted, sweep283

Published Bex0.3.2 and unchanged Render2.27.0/a764810a7682 both still reject the original root-service unquoted `9223372036854775808`: native exit1, valid:false, `Blueprint contains an invalid integer`, at services[0].envVars[0].value, line7/column20. [All six complete sanitized native runs and manifests](native-results.json).

The signed64 maximum and quoted unsigned64 maximum controls both validate with exit0 through each client, four successes. With the first unsigned case still failing, the remaining unsigned shape matrix was deliberately not repeated. This is a six-call validation-only check against Bex production under the existing isolated human QA session/workspace; no resource or deploy is created. The copied Render config is removed after every command. Native HTTP wire was not intercepted.

The source fix independently passes [sweep282](../../current-source-sweep282/README.md), but this live result keeps [104](../../../done/104.md) blocked. Actual deployed revision is unknown; no claim is made that the fixed artifact has rolled out, regressed, or been tested in Render production. No duplicate finding, deployment, product edit, commit or push was made by QA.
