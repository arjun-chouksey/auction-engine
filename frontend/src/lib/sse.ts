export async function consumeRFQStream(
  response: Response,
  signal: AbortSignal,
  onChange: () => void,
): Promise<void> {
  if (!response.ok || !response.body)
    throw new Error("SSE stream is unavailable");
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  while (!signal.aborted) {
    const result = await reader.read();
    if (result.done) break;
    buffer += decoder
      .decode(result.value, { stream: true })
      .replaceAll("\r\n", "\n");
    let boundary = buffer.indexOf("\n\n");
    while (boundary >= 0) {
      const message = buffer.slice(0, boundary);
      buffer = buffer.slice(boundary + 2);
      if (
        message.split("\n").some((line) => line.trim() === "event: rfq_changed")
      )
        onChange();
      boundary = buffer.indexOf("\n\n");
    }
  }
}
