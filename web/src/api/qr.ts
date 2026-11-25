import type { ApiResp } from "./types";

export type QrPurpose = "cookie" | "open";
export interface QrOptions { purpose?: QrPurpose; app?: string; client_id?: string }

export interface QrStartResult {
  purpose?: QrPurpose;
  token: string;
  qr_image_base64: string;
  qr_url: string;
  expires_in?: number;
  title?: string;
  hint?: string;
}

export type QrStatus = "waiting" | "scanned" | "success" | "failed" | "expired" | "cancelled";

export interface QrPollResult {
  status: QrStatus;
  purpose?: QrPurpose;
  token_expires?: string;
  cookie_expires?: string;
  cookie?: string;
  access_token?: string;
  refresh_token?: string;
  fields?: Record<string, string>;
  message?: string;
}

async function qrFetch<T>(path: string, body: unknown): Promise<ApiResp<T>> {
  const resp = await fetch(`/api${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  return (await resp.json()) as ApiResp<T>;
}

export async function qrStart(driverType: string, config?: string, options: QrOptions = {}) {
  return qrFetch<QrStartResult>("/qr/start", {
    ...options,
    driver_type: driverType,
    config: config || "",
  });
}

export async function qrPoll(driverType: string, token: string, purpose?: QrPurpose) {
  return qrFetch<QrPollResult>("/qr/poll", { driver_type: driverType, token, purpose });
}

export function qrCancel(driverType: string, token: string, purpose?: QrPurpose) {
  return qrFetch("/qr/cancel", { driver_type: driverType, token, purpose });
}
