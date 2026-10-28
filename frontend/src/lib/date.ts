export const localTimeZone = Intl.DateTimeFormat().resolvedOptions().timeZone || "local time";

export function formatDateTime(value: string): string {
  return `${new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(new Date(value))} ${localTimeZone}`;
}

export function localDateTimeToRFC3339(value: string): string {
  const date = new Date(value);
  if (!value || Number.isNaN(date.getTime())) throw new Error("Enter a valid date and time.");
  return date.toISOString();
}

export function countdown(closeAt: string, serverTime: string, receivedAt: number, now = Date.now()): string {
  const adjustedNow = new Date(serverTime).getTime() + (now - receivedAt);
  const remaining = Math.max(0, new Date(closeAt).getTime() - adjustedNow);
  const seconds = Math.floor(remaining / 1000);
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const secs = seconds % 60;
  return days > 0 ? `${days}d ${hours}h ${minutes}m` : `${String(hours).padStart(2, "0")}:${String(minutes).padStart(2, "0")}:${String(secs).padStart(2, "0")}`;
}
