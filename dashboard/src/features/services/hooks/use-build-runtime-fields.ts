import { useCallback, useRef, useState } from "react";
import {
  RUNTIME_COMMANDS,
  type GitRuntime,
} from "@/features/services/lib/runtime";

/**
 * The four coupled build fields on the create-service form, and the one rule
 * that binds them.
 *
 * Changing the runtime rewrites its siblings in the same commit: a Dockerfile
 * build has no build/start command, and a native runtime has no Dockerfile
 * path. The reset is the whole point — a value left behind from the previous
 * runtime is still submitted, which is how a stale dockerfilePath used to reach
 * the API after switching back to a native runtime. A detected runtime can
 * initialize these fields only until the user edits any of them; later probe
 * results must preserve that explicit configuration.
 */
export function useBuildRuntimeFields() {
  const [runtime, setRuntimeValue] = useState<GitRuntime>("node");
  const [buildCommand, setBuildCommandValue] = useState(
    RUNTIME_COMMANDS.node.build,
  );
  const [startCommand, setStartCommandValue] = useState(
    RUNTIME_COMMANDS.node.start,
  );
  const [dockerfilePath, setDockerfilePathValue] = useState("");
  const buildFieldsWereEdited = useRef(false);

  const applyRuntime = useCallback((next: GitRuntime) => {
    setRuntimeValue(next);
    if (next === "docker") {
      setBuildCommandValue("");
      setStartCommandValue("");
      return;
    }
    setDockerfilePathValue("");
    setBuildCommandValue(RUNTIME_COMMANDS[next].build);
    setStartCommandValue(RUNTIME_COMMANDS[next].start);
  }, []);

  const setRuntime = useCallback(
    (next: GitRuntime) => {
      buildFieldsWereEdited.current = true;
      applyRuntime(next);
    },
    [applyRuntime],
  );

  const setBuildCommand = useCallback((next: string) => {
    buildFieldsWereEdited.current = true;
    setBuildCommandValue(next);
  }, []);

  const setStartCommand = useCallback((next: string) => {
    buildFieldsWereEdited.current = true;
    setStartCommandValue(next);
  }, []);

  const setDockerfilePath = useCallback((next: string) => {
    buildFieldsWereEdited.current = true;
    setDockerfilePathValue(next);
  }, []);

  const setDetectedRuntime = useCallback(
    (next: GitRuntime) => {
      if (buildFieldsWereEdited.current) return;
      applyRuntime(next);
    },
    [applyRuntime],
  );

  return {
    runtime,
    setRuntime,
    setDetectedRuntime,
    buildCommand,
    setBuildCommand,
    startCommand,
    setStartCommand,
    dockerfilePath,
    setDockerfilePath,
  };
}
