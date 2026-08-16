import { useCallback, useState } from "react";
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
 * the API after switching back to a native runtime.
 */
export function useBuildRuntimeFields() {
  const [runtime, setRuntimeValue] = useState<GitRuntime>("node");
  const [buildCommand, setBuildCommand] = useState(RUNTIME_COMMANDS.node.build);
  const [startCommand, setStartCommand] = useState(RUNTIME_COMMANDS.node.start);
  const [dockerfilePath, setDockerfilePath] = useState("");

  const setRuntime = useCallback((next: GitRuntime) => {
    setRuntimeValue(next);
    if (next === "docker") {
      setBuildCommand("");
      setStartCommand("");
      return;
    }
    setDockerfilePath("");
    setBuildCommand(RUNTIME_COMMANDS[next].build);
    setStartCommand(RUNTIME_COMMANDS[next].start);
  }, []);

  return {
    runtime,
    setRuntime,
    buildCommand,
    setBuildCommand,
    startCommand,
    setStartCommand,
    dockerfilePath,
    setDockerfilePath,
  };
}
