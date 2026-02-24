export type SSEMessage = {
  event: string;
  data: string;
};

export async function* parseSSEStream(resp: Response, signal?: AbortSignal): AsyncGenerator<SSEMessage> {
  if (!resp.body) {
    return;
  }
  const reader = resp.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let eventName = "message";
  let dataLines: string[] = [];

  const flush = async () => {
    if (dataLines.length === 0) return;
    const data = dataLines.join("\n");
    const evt = eventName || "message";
    dataLines = [];
    eventName = "message";
    return { event: evt, data } satisfies SSEMessage;
  };

  while (true) {
    if (signal?.aborted) {
      try {
        await reader.cancel();
      } catch (e) {
        void e;
      }
      return;
    }
    const { value, done } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });

    let idx: number;
    while ((idx = buffer.indexOf("\n")) !== -1) {
      const rawLine = buffer.slice(0, idx);
      buffer = buffer.slice(idx + 1);
      const line = rawLine.endsWith("\r") ? rawLine.slice(0, -1) : rawLine;

      if (line === "") {
        const msg = await flush();
        if (msg) yield msg;
        continue;
      }
      if (line.startsWith(":")) {
        continue;
      }
      if (line.startsWith("event:")) {
        eventName = line.slice(6).trim() || "message";
        continue;
      }
      if (line.startsWith("data:")) {
        dataLines.push(line.slice(5).trimStart());
        continue;
      }
    }
  }

  if (buffer.trim() !== "") {
    const parts = buffer.split(/\r?\n/);
    for (const line of parts) {
      if (line.startsWith("event:")) {
        eventName = line.slice(6).trim() || "message";
        continue;
      }
      if (line.startsWith("data:")) {
        dataLines.push(line.slice(5).trimStart());
        continue;
      }
    }
  }
  const msg = await flush();
  if (msg) yield msg;
}
