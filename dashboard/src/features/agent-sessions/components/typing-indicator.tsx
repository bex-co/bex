// Three bouncing dots shown in a small bubble while the agent is thinking and
// hasn't yet streamed any assistant content (the last turn is a user prompt, or
// the assistant message is still empty). Purely presentational.
export function TypingIndicator() {
  return (
    <div className="flex items-center gap-1 px-3.5 py-3">
      <div className="bg-current h-1.5 w-1.5 animate-bounce rounded-full opacity-40 [animation-delay:-0.3s]" />
      <div className="bg-current h-1.5 w-1.5 animate-bounce rounded-full opacity-40 [animation-delay:-0.15s]" />
      <div className="bg-current h-1.5 w-1.5 animate-bounce rounded-full opacity-40" />
    </div>
  );
}
