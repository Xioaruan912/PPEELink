export async function runDiagnosticQueue<T, R>(
  items: T[],
  run: (item: T) => Promise<R>,
  events: {
    signal: AbortSignal;
    onStart: (item: T) => void;
    onResult: (item: T, result: R) => void;
    onError: (item: T, error: unknown) => void;
    onCancel: (item: T) => void;
  }
) {
  let next = 0;
  async function worker() {
    while (!events.signal.aborted && next < items.length) {
      const item = items[next++];
      events.onStart(item);
      try {
        const result = await run(item);
        if (events.signal.aborted) events.onCancel(item);
        else events.onResult(item, result);
      } catch (error) {
        if (events.signal.aborted) events.onCancel(item);
        else events.onError(item, error);
      }
    }
  }
  await Promise.all([worker(), worker()]);
  while (next < items.length) events.onCancel(items[next++]);
}
export function diagnosticVerdict(result: {
  steps: { key: string; status: string }[];
}): "passed" | "failed" | "unknown" {
  if (
    result.steps.some(
      (step) => step.key === "https" && step.status === "passed"
    )
  )
    return "passed";
  if (result.steps.some((step) => step.status === "failed")) return "failed";
  return "unknown";
}
