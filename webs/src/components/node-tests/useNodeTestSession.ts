import { onBeforeUnmount, ref } from "vue";
import { CancelTest, GetTestStatus } from "@/api/subcription/node";
import { useMotion } from "@/motion";

/** A run owns its node snapshot, request, stream reader and lock-release wait. */
export function useNodeTestSession() {
  const loading = ref(false);
  let version = 0;
  let controller: AbortController | undefined;
  let reader: ReadableStreamDefaultReader<Uint8Array> | undefined;
  let stopping: Promise<unknown> | undefined;
  let disposed = false;

  function abort() {
    version++;
    controller?.abort();
    void reader?.cancel().catch(() => undefined);
    reader = undefined;
    controller = undefined;
    loading.value = false;
  }
  function wait(signal: AbortSignal) {
    return new Promise<void>((resolve, reject) => {
      const finish = () => {
        clearTimeout(timer);
        signal.removeEventListener("abort", cancel);
        resolve();
      };
      const cancel = () => {
        clearTimeout(timer);
        signal.removeEventListener("abort", cancel);
        reject(new DOMException("已停止", "AbortError"));
      };
      const timer = setTimeout(finish, 500);
      signal.addEventListener("abort", cancel, { once: true });
      if (signal.aborted) cancel();
    });
  }
  async function run(
    task: (
      signal: AbortSignal,
      current: () => boolean,
      setReader: (
        value: ReadableStreamDefaultReader<Uint8Array> | undefined
      ) => void
    ) => Promise<void>
  ) {
    if (loading.value || disposed) return;
    abort();
    const id = version;
    const own = new AbortController();
    controller = own;
    const current = () => !disposed && id === version && !own.signal.aborted;
    loading.value = true;
    try {
      if (stopping) await stopping;
      if (!current()) return;
      let status = (await GetTestStatus())?.data;
      if (!current()) return;
      if (status) {
        await CancelTest();
        for (let attempt = 0; attempt < 6; attempt++) {
          if (!current()) return;
          await wait(own.signal);
          status = (await GetTestStatus())?.data;
          if (!status) break;
        }
        if (status) throw new Error("上次测试尚未停止，请稍后重试");
      }
      if (current())
        await task(own.signal, current, (value) => {
          if (id === version) reader = value;
          else void value?.cancel().catch(() => undefined);
        });
    } catch (cause) {
      if (current()) throw cause;
    } finally {
      own.abort();
      if (id === version) {
        loading.value = false;
        controller = undefined;
        reader = undefined;
      }
    }
  }
  async function stop() {
    const running = loading.value;
    abort();
    if (!running) return;
    const id = version;
    loading.value = true;
    const pending = CancelTest();
    stopping = pending;
    try {
      await pending;
    } finally {
      if (stopping === pending) stopping = undefined;
      if (id === version) loading.value = false;
    }
  }
  onBeforeUnmount(() => {
    disposed = true;
    abort();
  });
  return { loading, run, abort, stop };
}

export function token() {
  try {
    return localStorage.getItem("accessToken") || "";
  } catch {
    return "";
  }
}

export async function readEvents(
  response: Response,
  receive: (type: string, data: any) => void,
  current: () => boolean,
  setReader: (
    reader: ReadableStreamDefaultReader<Uint8Array> | undefined
  ) => void
) {
  const reader = response.body?.getReader();
  if (!reader) throw new Error("无法读取测试结果");
  setReader(reader);
  const decoder = new TextDecoder();
  let buffer = "";
  const dispatch = (event: string) => {
    if (!current()) return;
    const type = event.match(/^event:\s*(.+)$/m)?.[1]?.trim() || "message";
    const data = event
      .split("\n")
      .filter((line) => line.startsWith("data:"))
      .map((line) => line.slice(5).trimStart())
      .join("\n");
    if (!data) return;
    let parsed: any;
    try {
      parsed = JSON.parse(data);
    } catch {
      return;
    }
    receive(type, parsed);
  };
  try {
    while (current()) {
      const { value, done } = await reader.read();
      buffer = (buffer + decoder.decode(value, { stream: !done })).replace(
        /\r\n/g,
        "\n"
      );
      const events = buffer.split("\n\n");
      buffer = events.pop() || "";
      events.forEach(dispatch);
      if (done) {
        if (buffer.trim()) dispatch(buffer);
        break;
      }
    }
  } finally {
    await reader.cancel().catch(() => undefined);
    reader.releaseLock();
    setReader(undefined);
  }
}

export function useResultMotion() {
  const { animate, reduced } = useMotion();
  function enter(element: Element, done: () => void) {
    animate(
      element,
      { opacity: 0, y: 16 },
      {
        opacity: 1,
        y: 0,
        duration: 0.32,
        delay: Math.min(
          Number((element as HTMLElement).dataset.index || 0) * 0.04,
          0.24
        ),
        clearProps: "opacity,transform",
        onComplete: done,
        onInterrupt: done,
      }
    );
    if (reduced.value) done();
  }
  function leave(element: Element, done: () => void) {
    animate(
      element,
      { opacity: 1, y: 0 },
      {
        opacity: 0,
        y: -8,
        duration: 0.18,
        onComplete: done,
        onInterrupt: done,
      }
    );
    if (reduced.value) done();
  }
  return { enter, leave };
}
