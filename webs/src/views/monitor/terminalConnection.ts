export interface TerminalEvent {
  type: string;
  data?: Uint8Array;
  error?: string;
  sessionId?: string;
  persistent?: boolean;
}
export function createTerminalConnection(options: {
  ticket: (monitorId: number, sessionId: string) => Promise<string>;
  event: (event: TerminalEvent) => void;
}) {
  let socket: WebSocket | undefined;
  let generation = 0,
    monitorId = 0,
    sessionId = "",
    attempts = 0;
  let pending = false,
    ready = false,
    stopped = true;
  let retryTimer: ReturnType<typeof setTimeout> | undefined;
  let handshakeTimer: ReturnType<typeof setTimeout> | undefined;
  const emit = options.event;
  function closeSocket() {
    clearTimeout(handshakeTimer);
    if (socket) {
      socket.onopen = socket.onmessage = socket.onerror = socket.onclose = null;
      socket.close();
      socket = undefined;
    }
    ready = false;
  }
  function leave() {
    stopped = true;
    generation++;
    pending = false;
    clearTimeout(retryTimer);
    closeSocket();
    emit({ type: "detached" });
  }
  function retry() {
    if (stopped) return;
    closeSocket();
    if (attempts >= 8) {
      emit({
        type: "error",
        error: "暂时无法恢复连接，任务仍由服务器会话管理，可稍后手动重连",
      });
      return;
    }
    emit({ type: "reconnecting" });
    const token = generation;
    clearTimeout(retryTimer);
    retryTimer = setTimeout(
      async () => {
        if (!stopped && token === generation) await attach();
      },
      Math.min(1000 * 2 ** attempts++, 30000)
    );
  }
  async function attach() {
    if (pending || stopped) return;
    pending = true;
    const token = generation;
    emit({ type: attempts ? "reconnecting" : "connecting" });
    try {
      const ticket = await options.ticket(monitorId, sessionId);
      if (stopped || token !== generation) return;
      const protocol = location.protocol === "https:" ? "wss" : "ws";
      socket = new WebSocket(
        `${protocol}://${location.host}/api/v1/monitors/ssh/ws?ticket=${encodeURIComponent(ticket)}`
      );
      const current = socket;
      handshakeTimer = setTimeout(() => {
        if (token === generation && !ready) retry();
      }, 15000);
      current.onopen = () => {};
      current.onmessage = (message) => {
        if (stopped || token !== generation || socket !== current) return;
        try {
          const event = JSON.parse(message.data);
          if (event.type === "output") {
            const raw = atob(event.data || "");
            emit({
              type: "output",
              data: Uint8Array.from(raw, (char: string) => char.charCodeAt(0)),
            });
          } else if (
            event.type === "ready" &&
            event.persistent &&
            event.sessionId === sessionId
          ) {
            ready = true;
            attempts = 0;
            clearTimeout(handshakeTimer);
            emit(event);
          } else if (
            ["exit", "detached", "unauthorized", "error"].includes(event.type)
          ) {
            leave();
            emit(event);
          }
        } catch {
          leave();
          emit({ type: "error", error: "终端数据无效，请重新连接" });
        }
      };
      current.onclose = () => {
        if (token === generation && socket === current) retry();
      };
      current.onerror = () => {
        if (token === generation && socket === current) retry();
      };
    } catch (error: any) {
      if (token !== generation || stopped) return;
      if ([401, 403, 410].includes(error.response?.status)) {
        leave();
        emit({
          type: error.response.status === 410 ? "exit" : "unauthorized",
          error: error.message,
        });
      } else retry();
    } finally {
      if (token === generation) pending = false;
    }
  }
  async function connect(id: number, selected: string) {
    if (
      !stopped &&
      id === monitorId &&
      selected === sessionId &&
      (pending || socket)
    )
      return;
    leave();
    monitorId = id;
    sessionId = selected;
    attempts = 0;
    stopped = false;
    await attach();
  }
  function send(value: unknown) {
    if (!ready || socket?.readyState !== WebSocket.OPEN) return false;
    socket.send(JSON.stringify(value));
    return true;
  }
  return {
    connect,
    leave,
    input: (data: string) => send({ type: "input", data }),
    resize: (cols: number, rows: number) =>
      send({ type: "resize", cols, rows }),
  };
}
