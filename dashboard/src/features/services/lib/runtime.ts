/** Render's native runtimes — everything the platform can build without a Dockerfile. */
export type NativeRuntime =
  | "elixir"
  | "go"
  | "node"
  | "python"
  | "ruby"
  | "rust";

/** A repo-backed service's build strategy: a native runtime, or a Dockerfile. */
export type GitRuntime = NativeRuntime | "docker";

/** The default build/start commands each native runtime is seeded with. */
export const RUNTIME_COMMANDS: Record<
  NativeRuntime,
  { build: string; start: string }
> = {
  elixir: {
    build: "mix deps.get --only prod && mix compile",
    start: "mix phx.server",
  },
  go: { build: "go build -o app .", start: "./app" },
  node: { build: "npm install", start: "npm start" },
  python: {
    build: "pip install -r requirements.txt",
    start: "gunicorn app:app",
  },
  ruby: { build: "bundle install", start: "bundle exec puma" },
  rust: { build: "cargo build --release", start: "cargo run --release" },
};

// Ladder order for the runtime picker — most-used first, `docker` last because
// it swaps the whole build form. Deliberately not RUNTIME_COMMANDS' key order.
export const RUNTIME_DEFS: { id: GitRuntime; labelKey: string }[] = [
  { id: "node", labelKey: "services.createRuntimeNode" },
  { id: "python", labelKey: "services.createRuntimePython" },
  { id: "go", labelKey: "services.createRuntimeGo" },
  { id: "ruby", labelKey: "services.createRuntimeRuby" },
  { id: "rust", labelKey: "services.createRuntimeRust" },
  { id: "elixir", labelKey: "services.createRuntimeElixir" },
  { id: "docker", labelKey: "services.createRuntimeDocker" },
];
