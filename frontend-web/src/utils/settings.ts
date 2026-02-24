export type ApiKeyMode = "authorization_bearer" | "x-api-key" | "x-goog-api-key";

const KEY_KEY = "genfu_api_key";
const KEY_MODE = "genfu_api_key_mode";

export function getApiKeyConfig(): { key: string; mode: ApiKeyMode } {
  if (typeof window === "undefined") {
    return { key: "", mode: "authorization_bearer" };
  }
  const key = window.localStorage.getItem(KEY_KEY) ?? "";
  const mode = (window.localStorage.getItem(KEY_MODE) as ApiKeyMode | null) ?? "authorization_bearer";
  if (mode !== "authorization_bearer" && mode !== "x-api-key" && mode !== "x-goog-api-key") {
    return { key, mode: "authorization_bearer" };
  }
  return { key, mode };
}

export function setApiKeyConfig(next: { key: string; mode: ApiKeyMode }) {
  window.localStorage.setItem(KEY_KEY, next.key);
  window.localStorage.setItem(KEY_MODE, next.mode);
}

