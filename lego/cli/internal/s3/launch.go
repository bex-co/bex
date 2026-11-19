/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package s3

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/bex-co/bex/lego/cli/internal/telemetry"
)

// installHint says how to get the AWS CLI, and what to use without it.
const installHint = "bex s3 runs the AWS CLI, which was not found on your PATH.\n" +
	"Install AWS CLI v2 (https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html),\n" +
	"or use another S3 tool: `bex s3 env` prints the endpoint and a key for rclone, boto3, s5cmd and Terraform"

// launch runs `aws <service> <args>` against bex: it resolves the AWS CLI and
// a key first, so a missing `aws` never costs a key, then replaces this
// process with it.
func (d deps) launch(c *cobra.Command, service string, args []string) error {
	if helpRequested(args) {
		return c.Help()
	}
	awsPath, err := d.lookPath("aws")
	if err != nil {
		return errors.New(installHint)
	}
	startedAt := time.Now()
	s, err := d.session(commandContext(c))
	if err != nil {
		return err
	}
	return d.exec(awsPath, awsArgv(service, args, s), childEnv(d.environ(), s), func() {
		telemetry.EmitLaunch(c.CommandPath(), startedAt, c.ErrOrStderr())
	})
}

// awsArgv is the AWS CLI's command line. The endpoint goes last, after every
// argument the person typed, so an `--endpoint-url` of theirs can never send
// bex's key to another host: the CLI takes the last one. It is on the command
// line and not in AWS_ENDPOINT_URL_S3 because an older CLI ignores that
// variable and would send the request to AWS (ADR112 D3).
func awsArgv(service string, args []string, s session) []string {
	argv := append([]string{"aws", service}, args...)
	return append(argv, "--endpoint-url", s.Endpoint)
}

// keptAWS are the AWS_ variables a person may keep: they say how the CLI
// behaves, never who it is or where it goes. Every other AWS_ variable is
// dropped, so a profile, a session token, a role, a container or web-identity
// credential source, an endpoint or a region of theirs cannot reach the
// request (ADR112 D3: the user's own AWS credentials are never sent to bex).
var keptAWS = map[string]bool{
	"AWS_CA_BUNDLE":                     true,
	"AWS_PAGER":                         true,
	"AWS_MAX_ATTEMPTS":                  true,
	"AWS_RETRY_MODE":                    true,
	"AWS_CLI_FILE_ENCODING":             true,
	"AWS_CLI_AUTO_PROMPT":               true,
	"AWS_DATA_PATH":                     true,
	"AWS_REQUEST_CHECKSUM_CALCULATION":  true,
	"AWS_RESPONSE_CHECKSUM_VALIDATION":  true,
	"AWS_EC2_METADATA_DISABLED":         true,
	"AWS_METADATA_SERVICE_TIMEOUT":      true,
	"AWS_METADATA_SERVICE_NUM_ATTEMPTS": true,
}

// childEnv is the AWS CLI's environment: the person's own, minus every AWS_
// variable that is not in keptAWS, plus bex's key, region and the two AWS
// files bex owns. The key goes in the environment and never in a file of the
// person's.
func childEnv(environ []string, s session) []string {
	out := make([]string, 0, len(environ)+6)
	for _, item := range environ {
		name, _, _ := strings.Cut(item, "=")
		if strings.HasPrefix(name, "AWS_") && !keptAWS[name] {
			continue
		}
		out = append(out, item)
	}
	return append(out,
		"AWS_ACCESS_KEY_ID="+s.AccessKeyID,
		"AWS_SECRET_ACCESS_KEY="+s.Secret,
		// Both names: aws-cli v2 reads AWS_REGION first, and an older botocore
		// reads only AWS_DEFAULT_REGION.
		"AWS_REGION="+s.Region,
		"AWS_DEFAULT_REGION="+s.Region,
		"AWS_CONFIG_FILE="+s.ConfigPath,
		"AWS_SHARED_CREDENTIALS_FILE="+s.CredentialsPath,
	)
}

// execAWS replaces this process with the AWS CLI, so it owns the terminal and
// its exit status is the command's. beforeExec is the launch telemetry: the
// last bex code that runs for this invocation (ADR113 D7).
func execAWS(path string, argv, env []string, beforeExec func()) error {
	beforeExec()
	if err := syscall.Exec(path, argv, env); err != nil {
		return fmt.Errorf("exec %s: %w", path, err)
	}
	return nil
}
