import { onBeforeUnmount, ref } from "vue";
import { readEvents, token } from "@/components/node-tests/useNodeTestSession";

/** Each run owns its request and reader; cancelled runs cannot update the next run. */
export function useTestStream() {
  const loading = ref(false);
  let controller: AbortController | undefined;
  let reader: ReadableStreamDefaultReader<Uint8Array> | undefined;
  let version = 0;
  let disposed = false;
  function cancel() {
    version++;
    controller?.abort();
    void reader?.cancel().catch(() => undefined);
    controller = undefined;
    reader = undefined;
    loading.value = false;
  }
  async function run(
    path: string,
    body: URLSearchParams,
    receive: (type: string, data: any) => void
  ) {
    if (loading.value || disposed) return;
    const own = new AbortController();
    const id = ++version;
    controller = own;
    loading.value = true;
    const current = () => !disposed && id === version && !own.signal.aborted;
    try {
      const response = await fetch(import.meta.env.VITE_APP_BASE_API + path, {
        method: "POST",
        headers: {
          "Content-Type": "application/x-www-form-urlencoded",
          Authorization: token(),
        },
        body: body.toString(),
        signal: own.signal,
      });
      if (!current()) {
        await response.body?.cancel().catch(() => undefined);
        return;
      }
      if (!response.ok) {
        const text = await response.text();
        let message = `请求失败: ${response.status}`;
        try {
          message = JSON.parse(text).msg || message;
        } catch {
          /* HTTP text may not be JSON. */
        }
        throw new Error(message);
      }
      await readEvents(
        response,
        (type, data) => {
          if (type === "error") throw new Error(data?.msg || "检测失败");
          receive(type, data);
        },
        current,
        (value) => {
          if (current()) reader = value;
          else void value?.cancel().catch(() => undefined);
        }
      );
    } catch (cause) {
      if (current()) throw cause;
    } finally {
      own.abort();
      if (id === version) {
        controller = undefined;
        reader = undefined;
        loading.value = false;
      }
    }
  }
  onBeforeUnmount(() => {
    disposed = true;
    cancel();
  });
  return { loading, run, cancel };
}
