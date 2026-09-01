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

// Command backup-encrypt is the ADR050 Tier A encrypt stage of the managed Key
// Value backup pipeline, as a first-party entrypoint of the bex image.
//
// It replaces an `alpine` container that fetched the upstream `age` release
// tarball over the network and ran it (ADR067 #8 → ADR068 #9: the stage reads
// the PLAINTEXT RDB, so anything it resolves at run time executes next to
// unencrypted tenant data). The same encryption is now performed by
// filippo.io/age compiled into the image bex already builds, signs, and
// digest-pins — no package manager, no download, nothing resolved at run time.
//
// Wire-compatible with the `age` CLI it replaces: the output is a standard age
// file addressed to one X25519 recipient, so an object produced here still
// decrypts with `age -d -i <identity>` and therefore with
// scripts/restore-keyvalue.sh unchanged.
//
// Usage: backup-encrypt <plaintext-in> <ciphertext-out>, recipient in
// AGE_PUBLIC_KEY. The plaintext input is removed on success — it shares an
// EmptyDir with the upload stage, which must never see it.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"filippo.io/age"
)

func main() {
	if err := run(os.Args[1:], os.Getenv("AGE_PUBLIC_KEY")); err != nil {
		fmt.Fprintf(os.Stderr, "backup-encrypt: %v\n", err)
		os.Exit(1)
	}
}

// run encrypts inPath to outPath for recipient. Every failure leaves no output
// file: the upload stage globs for the encrypted object, so a truncated or
// unencrypted file left behind would be uploaded as if it were a backup.
func run(args []string, recipient string) error {
	if len(args) != 2 {
		return errors.New("usage: backup-encrypt <plaintext-in> <ciphertext-out>")
	}
	inPath, outPath := args[0], args[1]
	if recipient == "" {
		return errors.New("AGE_PUBLIC_KEY is empty")
	}
	to, err := age.ParseX25519Recipient(recipient)
	if err != nil {
		return fmt.Errorf("parse recipient: %w", err)
	}

	in, err := os.Open(inPath)
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck // read side; the write side below is what must be checked

	if err := encrypt(in, outPath, to); err != nil {
		os.Remove(outPath) //nolint:errcheck // best-effort cleanup of a partial object
		return err
	}
	// The plaintext is only removed once the ciphertext is durable, so a crash
	// mid-run leaves a re-runnable Job rather than a lost snapshot.
	return os.Remove(inPath)
}

func encrypt(in io.Reader, outPath string, to age.Recipient) (retErr error) {
	// 0644, not 0600: the upload stage reads this from the shared EmptyDir as a
	// DIFFERENT uid with every capability (including DAC_OVERRIDE) dropped, so
	// an owner-only file is unreadable there and the Job fails with nothing
	// uploaded. The bytes are age ciphertext; pod-internal readability leaks
	// nothing.
	out, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	// A writable file can report delayed write failures only from Close. Join
	// that result into every return path so even an earlier encryption failure
	// cannot hide a failed final flush.
	defer func() {
		retErr = errors.Join(retErr, out.Close())
	}()

	w, err := age.Encrypt(out, to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, in); err != nil {
		return err
	}
	// age buffers: Close writes the final chunk and its authentication tag, so
	// skipping it produces a file that fails to decrypt.
	if err := w.Close(); err != nil {
		return err
	}
	// Sync before the caller removes the plaintext.
	if err := out.Sync(); err != nil {
		return err
	}
	return nil
}
