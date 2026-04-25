import { fetchWithTimeout, jsonHeaders } from "./shared";

const RUM_ENDPOINT = "/v1/rum";

export function sendRUMPayload(
  payload: string,
  options?: { keepalive?: boolean },
): Promise<Response> {
  return fetchWithTimeout(RUM_ENDPOINT, {
    method: "POST",
    headers: jsonHeaders,
    body: payload,
    keepalive: options?.keepalive,
  });
}
