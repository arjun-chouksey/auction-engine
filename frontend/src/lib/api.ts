import type { ApiErrorBody } from "./types";

// Keep browser requests same-origin. Vite and the packaged Nginx image both
// proxy /api to the Go service, which also lets authenticated SSE work without
// weakening the API with cross-origin credentials.
const API_BASE = "";

export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string) { super(message); }
}

export async function api<T>(path: string, token?: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body) headers.set("Content-Type", "application/json");
  if (token) headers.set("Authorization", `Bearer ${token}`);
  let response: Response;
  try { response = await fetch(`${API_BASE}/api/v1${path}`, { ...init, headers }); }
  catch { throw new ApiError(0, "NETWORK_ERROR", "The server could not be reached. Check your connection and try again."); }
  if (!response.ok) {
    const body = await response.json().catch(() => ({})) as ApiErrorBody;
    throw new ApiError(response.status, body.error?.code ?? "REQUEST_FAILED", body.error?.message ?? "The request could not be completed.");
  }
  return response.json() as Promise<T>;
}

export function apiBase(): string { return API_BASE; }
