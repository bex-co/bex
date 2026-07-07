// Package build is the bex build plane: clone a repo @ ref and turn it into an OCI
// image (Dockerfile fast-path via BuildKit, else Cloud Native Buildpacks), pushed to
// the local registry. Shells out to git/docker/pack (host tools) — the same path the
// MVP proved; an in-cluster BuildKit/kpack Job is the productionization.
package build

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Options configures a single build.
type Options struct {
	Repo       string    // git URL or local path
	Ref        string    // branch or commit; defaults to repo HEAD
	Name       string    // image repo name (the service name)
	Registry   string    // e.g. 127.0.0.1:5050
	CNBBuilder string    // e.g. paketobuildpacks/builder-jammy-base
	Log        io.Writer // build log sink (may be nil)
}

// Result is a successful build.
type Result struct {
	Image  string // <registry>/<name>:<commit>
	Commit string // short commit
}

func (o Options) log(format string, a ...any) {
	if o.Log != nil {
		fmt.Fprintf(o.Log, format+"\n", a...)
	}
}

// run executes a command, streaming combined output to the log, returning an error
// (with captured output) on failure.
func run(ctx context.Context, w io.Writer, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var buf bytes.Buffer
	mw := io.Writer(&buf)
	if w != nil {
		mw = io.MultiWriter(&buf, w)
	}
	cmd.Stdout, cmd.Stderr = mw, mw
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, buf.String())
	}
	return nil
}

func output(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// Build clones, builds and pushes. Returns the image ref + resolved commit.
func Build(ctx context.Context, o Options) (Result, error) {
	work, err := os.MkdirTemp("", "bex-build-")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(work)

	o.log("=== git clone %s (ref %s) ===", o.Repo, o.Ref)
	if err := run(ctx, o.Log, "", "git", "clone", "--quiet", o.Repo, work); err != nil {
		return Result{}, err
	}
	if o.Ref != "" {
		if err := run(ctx, o.Log, work, "git", "checkout", "--quiet", o.Ref); err != nil {
			// ref may be a remote-only branch
			if err2 := run(ctx, o.Log, work, "git", "checkout", "--quiet", "origin/"+o.Ref); err2 != nil {
				return Result{}, err
			}
		}
	}
	commit, err := output(ctx, work, "git", "rev-parse", "--short", "HEAD")
	if err != nil || commit == "" {
		commit = "unknown"
	}

	image := fmt.Sprintf("%s/%s:%s", o.Registry, o.Name, commit)
	if _, statErr := os.Stat(filepath.Join(work, "Dockerfile")); statErr == nil {
		o.log("=== docker build -> %s (Dockerfile/BuildKit) ===", image)
		cmd := exec.CommandContext(ctx, "docker", "build", "--progress", "plain", "-t", image, ".")
		cmd.Dir, cmd.Env = work, append(os.Environ(), "DOCKER_BUILDKIT=1")
		if o.Log != nil {
			cmd.Stdout, cmd.Stderr = o.Log, o.Log
		}
		if err := cmd.Run(); err != nil {
			return Result{}, fmt.Errorf("docker build failed: %w", err)
		}
	} else {
		o.log("=== pack build -> %s (CNB %s) ===", image, o.CNBBuilder)
		if err := run(ctx, o.Log, "", "pack", "build", image, "--path", work,
			"--builder", o.CNBBuilder, "--pull-policy", "if-not-present"); err != nil {
			return Result{}, fmt.Errorf("pack build failed: %w", err)
		}
	}

	o.log("=== docker push %s ===", image)
	if err := run(ctx, o.Log, "", "docker", "push", image); err != nil {
		o.log("registry push failed (continuing with local image): %v", err)
	}
	return Result{Image: image, Commit: commit}, nil
}
