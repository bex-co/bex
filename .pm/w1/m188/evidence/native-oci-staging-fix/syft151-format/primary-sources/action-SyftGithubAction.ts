import * as core from "@actions/core";
import * as github from "@actions/github";
import * as cache from "@actions/tool-cache";
import {
  PullRequestEvent,
  Release,
  ReleaseEvent,
} from "@octokit/webhooks-types";
import * as fs from "fs";
import os from "os";
import path from "path";
import stream from "stream";
import { SyftOptions } from "../Syft";
import { VERSION } from "../SyftVersion";
import { execute } from "./Executor";
import {
  DependencySnapshot,
  dashWrap,
  debugLog,
  getClient,
} from "./GithubClient";
import { downloadSyftFromZip } from "./SyftDownloader";
import { isReleaseTag, stringify, stripEmojis } from "./Util";

export const SYFT_BINARY_NAME = "syft";

/**
 * Gets the version of Syft to install, which is the pinned default unless the
 * workflow requested another one.
 */
export function getSyftVersion(): string {
  return core.getInput("syft-version") || VERSION;
}

const PRIOR_ARTIFACT_ENV_VAR = "ANCHORE_SBOM_ACTION_PRIOR_ARTIFACT";

const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "sbom-action-"));
const githubDependencySnapshotFile = `${tempDir}/github.sbom.json`;

const exeSuffix = process.platform == "win32" ? ".exe" : "";

/**
 * Tries to get a unique artifact name or otherwise as appropriate as possible
 */
export function getArtifactName(): string {
  const fileName = getArtifactNameInput();

  // if there is an explicit filename just return it, this could cause issues
  // where earlier sboms are overwritten by later ones
  if (fileName) {
    return fileName;
  }

  const format = getSbomFormat();
  let extension: string = format;
  switch (format) {
    case "spdx":
    case "spdx-tag-value":
      extension = "spdx";
      break;
    case "spdx-json":
      extension = "spdx.json";
      break;
    case "cyclonedx":
    case "cyclonedx-xml":
      extension = "cyclonedx.xml";
      break;
    case "cyclonedx-json":
      extension = "cyclonedx.json";
      break;
    case "json":
      extension = "syft.json";
      break;
  }

  const imageName = core.getInput("image");
  if (imageName) {
    const parts = imageName.split("/");
    // remove the hostname
    if (parts.length > 2) {
      parts.splice(0, 1);
    }
    const prefix = parts.join("-").replace(/[^-a-zA-Z0-9]/g, "_");
    return `${prefix}.${extension}`;
  }

  const {
    repo: { repo },
    job,
    action,
  } = github.context;
  // when run without an id, we get various auto-generated names, like:
  // __self __self_2 __anchore_sbom-action  __anchore_sbom-action_2 etc.
  // so just keep the number at the end if there is one, otherwise
  // this will not match an id unless for some reason it starts with __
  let stepName = action.replace(/__[-_a-z]+/, "");
  if (stepName) {
    stepName = `-${stepName}`;
  }
  return `${repo}-${job}${stepName}.${extension}`;
}

/**
 * Returns the artifact-name input value
 */
function getArtifactNameInput() {
  return core.getInput("artifact-name");
}

/**
 * Gets a reference to the syft command and executes the syft action
 * @param input syft input parameters
 * @param format syft output format
 * @param opts additional options
 */
async function executeSyft({
  input,
  format,
  ...opts
}: SyftOptions): Promise<string> {
  let stdout = "";

  const cmd = await getSyftCommand();

  const env: { [key: string]: string } = {
    ...process.env,
    SYFT_CHECK_FOR_APP_UPDATE: "false",
  };

  const registryUser = core.getInput("registry-username");
  const registryPass = core.getInput("registry-password");

  if (registryUser) {
    env.SYFT_REGISTRY_AUTH_USERNAME = registryUser;
    if (registryPass) {
      env.SYFT_REGISTRY_AUTH_PASSWORD = registryPass;
    } else {
      core.warning(
        "WARNING: registry-username specified without registry-password",
      );
    }
  }

  // https://github.com/anchore/syft#configuration
  let args = ["scan"];

  if (core.isDebug()) {
    args = [...args, "-vv"];
  }

  if ("image" in input && input.image) {
    if (registryUser) {
      args = [...args, `registry:${input.image}`];
    } else {
      args = [...args, `${input.image}`];
    }
  } else if ("file" in input && input.file) {
    args = [...args, `file:${input.file}`];
  } else if ("path" in input && input.path) {
    args = [...args, `dir:${input.path}`];
  } else {
    throw new Error("Invalid input, no image or path specified");
  }

  args = [...args, "-o", format];

  if (opts.uploadToDependencySnapshotAPI) {
    // generate github dependency format
    args = [...args, "-o", `github=${githubDependencySnapshotFile}`];
  }

  if (opts.configFile) {
    args = [...args, "-c", opts.configFile];
  }

  // Execute in a group so the syft output is collapsed in the GitHub log
  core.info(`[command]${cmd} ${args.join(" ")}`);

  // This /dev/null writable stream is required so the entire contents
  // of the SBOM is not written to the GitHub action log. the listener below
  // will actually capture the output
  const outStream = new stream.Writable({
    write(buffer, encoding, next) {
      next();
    },
  });

  const exitCode = await core.group("Executing Syft...", async () =>
    execute(cmd, args, {
      env,
      outStream,
      listeners: {
        stdout(buffer) {
          stdout += buffer.toString();
        },
        stderr(buffer) {
          core.info(buffer.toString());
        },
        debug(message) {
          core.debug(message);
        },
      },
    }),
  );

  if (exitCode > 0) {
    debugLog("Syft stdout:", stdout);
    throw new Error("An error occurred running Syft");
  } else {
    return stdout;
  }
}

/**
 * Renders a caught value for a message, as only the message of a thrown error
 * is reported to the build; a cause is not.
 */
function describeError(e: unknown): string {
  return e instanceof Error ? e.message : stringify(e);
}

function isWindows(): boolean {
  return process.platform == "win32";
}

async function downloadSyftWindowsWorkaround(version: string): Promise<string> {
  const versionNoV = version.replace(/^v/, "");
  const url = `https://github.com/anchore/syft/releases/download/${version}/syft_${versionNoV}_windows_amd64.zip`;
  core.info(`Downloading syft from ${url}`);
  let zipPath;
  try {
    zipPath = await cache.downloadTool(url);
  } catch (e) {
    throw new Error(
      `Unable to download Syft from ${url}: ${describeError(e)}. If this is not a transient network failure, check that '${version}' is a released version of Syft: https://github.com/anchore/${SYFT_BINARY_NAME}/releases`,
      { cause: e },
    );
  }
  const toolDir = await cache.extractZip(zipPath);
  return path.join(toolDir, `${SYFT_BINARY_NAME}${exeSuffix}`);
}

/**
 * Downloads the appropriate Syft binary for the platform
 */
export async function downloadSyft(): Promise<string> {
  const name = SYFT_BINARY_NAME;
  const version = getSyftVersion();
  const isTag = isReleaseTag(version);

  if (isWindows()) {
    // Only release assets are downloaded here, so a version that is not a
    // release tag has nothing to download.
    if (!isTag) {
      throw new Error(
        `Syft version '${version}' is not a release tag. Specify a tag such as '${VERSION}': https://github.com/anchore/${name}/releases`,
      );
    }
    return downloadSyftWindowsWorkaround(version);
  }

  // Pin the installer to the tag of the release being installed
  if (!isTag) {
    core.warning(
      `Syft version '${version}' is not a release tag, so the installer cannot be pinned to it. Specify a tag such as '${VERSION}' to install a pinned version of Syft.`,
    );
  }
  const ref = isTag ? version : "main";
  const url = `https://raw.githubusercontent.com/anchore/${name}/${ref}/install.sh`;

  core.debug(`Installing ${name} ${version}`);

  // Download the installer, and run
  let installPath;
  try {
    installPath = await cache.downloadTool(url);
  } catch (e) {
    const hint = isTag
      ? ` If this is not a transient network failure, check that '${version}' is a released version of Syft: https://github.com/anchore/${name}/releases`
      : "";
    throw new Error(
      `Unable to download the Syft installer from ${url}: ${describeError(e)}.${hint}`,
      { cause: e },
    );
  }

  const syftBinaryPath = `${installPath}_${name}`;

  const exitCode = await execute(
    "sh",
    [installPath, "-d", "-b", syftBinaryPath, version],
    {
      env: {
        ...process.env,
        // The installer fetches and re-executes the copy of itself belonging to
        // the tag it is installing, which would undo the pin above; tell the
        // pinned script to install directly instead. A version that is not a
        // tag is resolved by the installer, so it has to do that fetch itself.
        DOWNLOAD_TAG_INSTALL_SCRIPT: isTag ? "false" : "true",
      } as { [key: string]: string },
      ignoreReturnCode: true,
    },
  );
  if (exitCode > 0) {
    throw new Error(
      `The Syft installer failed to install ${version}; see the log above for details`,
    );
  }

  return path.join(syftBinaryPath, name) + exeSuffix;
}

/**
 * Gets the Syft command to run via exec
 */
export async function getSyftCommand(): Promise<string> {
  const name = SYFT_BINARY_NAME + exeSuffix;
  const version = getSyftVersion();

  const sourceSyft = await downloadSyftFromZip(version);
  if (sourceSyft) {
    core.info(`Using sourceSyft: '${sourceSyft}'`);
    return sourceSyft;
  }

  let syftPath = cache.find(name, version);
  if (!syftPath) {
    // Not found; download and install it; returns a path to the binary
    syftPath = await downloadSyft();

    // Cache the downloaded file
    syftPath = await cache.cacheFile(syftPath, name, name, version);
  }

  core.debug(`Got Syft path: ${syftPath} binary at: ${syftPath}/${name}`);

  // Add tool to path for this and future actions to use
  core.addPath(syftPath);
  return `${syftPath}/${name}`;
}

/**
 * Returns the SBOM format as specified by the user, defaults to SPDX
 */
export function getSbomFormat(): SyftOptions["format"] {
  return (core.getInput("format") as SyftOptions["format"]) || "spdx-json";
}

/**
 * Returns the SHA of the current commit, which will either be the head
 * of the pull request branch or the value of github.context.sha, depending
 * on the event type.
 */
export function getSha(): string {
  const pull_request_events = [
    "pull_request",
    "pull_request_comment",
    "pull_request_review",
    "pull_request_review_comment",
    // Note that pull_request_target is omitted here.
    // That event runs in the context of the base commit of the PR,
    // so the snapshot should not be associated with the head commit.
  ];
  if (pull_request_events.includes(github.context.eventName)) {
    const pr = (github.context.payload as PullRequestEvent).pull_request;
    return pr.head.sha;
  } else {
    return github.context.sha;
  }
}

/**
 * Uploads a SBOM as a workflow artifact
 * @param contents SBOM file contents
 */
export async function uploadSbomArtifact(contents: string): Promise<void> {
  const { repo } = github.context;
  const client = getClient(repo, core.getInput("github-token"));

  const fileName = getArtifactName();

  const filePath = `${tempDir}/${fileName}`;
  fs.writeFileSync(filePath, contents);

  const retentionDays = parseInt(core.getInput("upload-artifact-retention"));

  core.info(dashWrap("Uploading workflow artifacts"));
  core.info(filePath);

  await client.uploadWorkflowArtifact({
    file: filePath,
    name: fileName,
    retention: retentionDays,
  });
}

/**
 * Gets a boolean input value if supplied, otherwise returns the default
 * @param name name of the input
 * @param defaultValue default value to return if not set
 */
function getBooleanInput(name: string, defaultValue: boolean): boolean {
  const val = core.getInput(name);
  if (val === undefined || val === "") {
    return defaultValue;
  }
  return val.toLowerCase() === "true";
}

/**
 * Optionally fetches the target SBOM in order to provide some information
 * on changes
 */
async function comparePullRequestTargetArtifact(): Promise<void> {
  const doCompare = getBooleanInput("compare-pulls", false);
  const { eventName, payload, repo } = github.context;
  if (doCompare && eventName === "pull_request") {
    const client = getClient(repo, core.getInput("github-token"));

    const pr = (payload as PullRequestEvent).pull_request;
    const branchWorkflow = await client.findLatestWorkflowRunForBranch({
      branch: pr.base.ref,
    });

    debugLog("Got branchWorkflow:", branchWorkflow);

    if (branchWorkflow) {
      const baseBranchArtifacts = await client.listWorkflowRunArtifacts({
        runId: branchWorkflow.id,
      });

      debugLog("Got baseBranchArtifacts:", baseBranchArtifacts);

      for (const artifact of baseBranchArtifacts) {
        if (artifact.name === getArtifactName()) {
          const baseArtifact = await client.downloadWorkflowRunArtifact({
            artifactId: artifact.id,
          });

          core.info(
            `Downloaded SBOM from ref '${pr.base.ref}' to ${baseArtifact}`,
          );
        }
      }
    }
  }
}

function uploadToSnapshotAPI() {
  return getBooleanInput("dependency-snapshot", false);
}

export async function runSyftAction(): Promise<void> {
  core.info(dashWrap("Running SBOM Action"));

  debugLog(`Got github context:`, github.context);

  const start = Date.now();

  const doUpload = getBooleanInput("upload-artifact", true);

  const output = await executeSyft({
    input: {
      path: core.getInput("path"),
      file: core.getInput("file"),
      image: core.getInput("image"),
    },
    format: getSbomFormat(),
    uploadToDependencySnapshotAPI: uploadToSnapshotAPI(),
    configFile: core.getInput("config"),
  });

  core.info(`SBOM scan completed in: ${(Date.now() - start) / 1000}s`);

  if (output) {
    await comparePullRequestTargetArtifact();

    // We may want to develop a supply chain during the build, this is one
    // potential way to do so:
    const priorArtifact = process.env[PRIOR_ARTIFACT_ENV_VAR];
    if (priorArtifact) {
      core.debug(`Prior artifact: ${priorArtifact}`);
    }

    const outputFile = core.getInput("output-file");
    if (outputFile) {
      fs.writeFileSync(outputFile, output);
    }

    if (doUpload) {
      await uploadSbomArtifact(output);

      core.exportVariable(PRIOR_ARTIFACT_ENV_VAR, getArtifactName());
    }
  } else {
    throw new Error(`No Syft output`);
  }
}

/**
 * Attaches the SBOM assets to a release if run in release mode
 */
export async function uploadDependencySnapshot(): Promise<void> {
  if (!uploadToSnapshotAPI()) {
    return;
  }

  if (!fs.existsSync(githubDependencySnapshotFile)) {
    core.warning(
      `No dependency snapshot found at '${githubDependencySnapshotFile}'`,
    );
    return;
  }
  const { workflow, job, runId, repo, ref } = github.context;
  const sha = getSha();
  const client = getClient(repo, core.getInput("github-token"));

  const snapshot = JSON.parse(
    fs.readFileSync(githubDependencySnapshotFile).toString("utf8"),
  ) as DependencySnapshot;

  let correlator = `${workflow}_${job}`;
  // if running in a matrix build, it is not possible to determine a unique value,
  // so a user must explicitly specify the artifact-name input, there isn't any
  // other indicator of being run within a matrix build, so we must use that
  // here in order to properly correlate dependency snapshots
  const artifactInput = getArtifactNameInput();
  if (artifactInput) {
    correlator += `_${artifactInput}`;
  }

  // Need to add the job and repo details
  snapshot.job = {
    correlator: stripEmojis(
      core.getInput("dependency-snapshot-correlator") || correlator,
    ),
    id: `${runId}`,
  };
  snapshot.sha = sha;
  snapshot.ref = ref;

  core.info(
    `Uploading GitHub dependency snapshot from ${githubDependencySnapshotFile}`,
  );
  debugLog("Snapshot:", snapshot);

  await client.postDependencySnapshot(snapshot);
}

/**
 * Attaches the SBOM assets to a release if run in release mode
 */
export async function attachReleaseAssets(): Promise<void> {
  const doRelease = getBooleanInput("upload-release-assets", true);

  if (!doRelease) {
    return;
  }

  debugLog("Got github context:", github.context);

  const { eventName, ref, payload, repo } = github.context;
  const client = getClient(repo, core.getInput("github-token"));

  let release: Release | undefined = undefined;

  // Try to detect a release
  if (eventName === "release") {
    // Obviously if this is run during a release
    release = (payload as ReleaseEvent).release;
    debugLog("Got releaseEvent:", release);
  } else {
    // We may have a tag-based workflow that creates releases or even drafts
    const releaseRefPrefix =
      core.getInput("release-ref-prefix") || "refs/tags/";
    const isRefPush = eventName === "push" && ref.startsWith(releaseRefPrefix);
    if (isRefPush) {
      const tag = ref.substring(releaseRefPrefix.length);
      release = await client.findRelease({ tag });
      debugLog("Found release for ref push:", release);
    }
  }

  if (release) {
    // ^sbom.*\\.${format}$`;
    const sbomArtifactInput = core.getInput("sbom-artifact-match");
    const sbomArtifactPattern = sbomArtifactInput || `^${getArtifactName()}$`;
    const matcher = new RegExp(sbomArtifactPattern);

    const artifacts = await client.listCurrentWorkflowArtifacts();
    let matched = artifacts.filter((a) => {
      const matches = matcher.test(a.name);
      if (matches) {
        core.debug(`Found artifact: ${a.name}`);
      } else {
        core.debug(`Artifact: ${a.name} not matching ${sbomArtifactPattern}`);
      }
      return matches;
    });

    // We may have a release run based on a prior build from another workflow
    if (eventName === "release" && !matched.length) {
      core.info(
        "No artifacts found in this workflow. Searching for release artifacts from prior workflow...",
      );
      const latestRun = await client.findLatestWorkflowRunForBranch({
        branch: release.target_commitish,
      });

      debugLog("Got latest run for prior workflow", latestRun);

      if (latestRun) {
        const runArtifacts = await client.listWorkflowRunArtifacts({
          runId: latestRun.id,
        });

        matched = runArtifacts.filter((a) => {
          const matches = matcher.test(a.name);
          if (matches) {
            core.debug(`Found run artifact: ${a.name}`);
          } else {
            core.debug(
              `Run artifact: ${a.name} not matching ${sbomArtifactPattern}`,
            );
          }
          return matches;
        });
      }
    }

    if (!matched.length && sbomArtifactInput) {
      core.warning(`WARNING: no SBOMs found matching ${sbomArtifactInput}`);
      return;
    }

    core.info(dashWrap(`Attaching SBOMs to release: '${release.tag_name}'`));
    for (const artifact of matched) {
      const file = await client.downloadWorkflowArtifact(artifact);

      core.info(file);
      const contents = fs.readFileSync(file);
      const assetName = path.basename(file);

      const assets = await client.listReleaseAssets({
        release,
      });

      const asset = assets.find((a) => a.name === assetName);
      if (asset) {
        await client.deleteReleaseAsset({
          release,
          asset,
        });
      }

      await client.uploadReleaseAsset({
        release,
        assetName,
        contents: contents.toString(),
        contentType: "text/plain",
      });
    }
  }
}

/**
 * Executes the provided callback and wraps any exceptions in a build failure
 */
export async function runAndFailBuildOnException<T>(
  fn: () => Promise<T>,
): Promise<T | void> {
  try {
    return await fn();
  } catch (e) {
    if (e instanceof Error) {
      core.setFailed(e.message);
    } else if (e instanceof Object) {
      core.setFailed(`Action failed: ${stringify(e)}`);
    } else {
      core.setFailed(`An unknown error occurred: ${stringify(e)}`);
    }
  }
}
