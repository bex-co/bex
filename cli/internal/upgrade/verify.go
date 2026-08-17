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

package upgrade

import (
	"bytes"
	"fmt"

	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

const (
	// certOIDCIssuer is the OIDC issuer of the keyless Fulcio certificate the
	// GitHub Actions release job signs with.
	certOIDCIssuer = "https://token.actions.githubusercontent.com"
	// certSANRegexp pins the Fulcio SAN to the exact bex-cli release workflow
	// (the cert's SAN is the job_workflow_ref). Only that workflow, on a
	// bex-cli/v* tag, can produce a signature this accepts — a signature from
	// any other repo, workflow, or ref is rejected.
	certSANRegexp = `^https://github\.com/bex-co/bex/\.github/workflows/cli-release\.yml@refs/tags/bex-cli/v`
)

// verifySignature confirms sigBundle is a cosign keyless signature over the
// exact checksums bytes, produced by the bex-cli release workflow. It fetches
// the sigstore public-good trusted root over TUF (fail-closed: no root ⇒ no
// upgrade) and enforces the certificate identity, transparency-log inclusion,
// SCT, and observer-timestamp requirements cosign's own verify-blob applies.
func verifySignature(checksums, sigBundle []byte) error {
	trustedRoot, err := root.FetchTrustedRoot()
	if err != nil {
		return fmt.Errorf("fetch sigstore trusted root: %w", err)
	}

	b := &bundle.Bundle{Bundle: new(protobundle.Bundle)}
	if err := b.UnmarshalJSON(sigBundle); err != nil {
		return fmt.Errorf("parse signature bundle: %w", err)
	}

	verifier, err := verify.NewSignedEntityVerifier(trustedRoot,
		verify.WithSignedCertificateTimestamps(1),
		verify.WithTransparencyLog(1),
		verify.WithObserverTimestamps(1),
	)
	if err != nil {
		return fmt.Errorf("build sigstore verifier: %w", err)
	}

	identity, err := verify.NewShortCertificateIdentity(certOIDCIssuer, "", "", certSANRegexp)
	if err != nil {
		return fmt.Errorf("build certificate identity policy: %w", err)
	}

	if _, err := verifier.Verify(b, verify.NewPolicy(
		verify.WithArtifact(bytes.NewReader(checksums)),
		verify.WithCertificateIdentity(identity),
	)); err != nil {
		return fmt.Errorf("checksums signature is not a valid bex release signature: %w", err)
	}
	return nil
}
