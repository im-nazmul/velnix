import { ref } from "vue";
export const csrf = ref("");
export class APIError extends Error {
    constructor(
        message: string,
        public status: number,
        public uncertain = false,
    ) {
        super(message);
    }
}
export async function request<T>(
    path: string,
    body?: unknown,
    signal?: AbortSignal,
    loginCSRF?: string,
): Promise<T> {
    let response: Response;
    try {
        response = await fetch(path, {
            method: body === undefined ? "GET" : "POST",
            credentials: "same-origin",
            cache: "no-store",
            redirect: "error",
            signal,
            headers:
                body === undefined
                    ? {}
                    : {
                          "Content-Type": "application/json",
                          "X-CSRF-Token": loginCSRF ?? csrf.value,
                      },
            body: body === undefined ? undefined : JSON.stringify(body),
        });
    } catch (e) {
        if (e instanceof DOMException && e.name === "AbortError") throw e;
        throw new APIError(
            body !== undefined
                ? "The result is uncertain. Inspect current state before reviewing another plan."
                : "Cannot reach the local dashboard. Check its foreground terminal.",
            0,
            body !== undefined,
        );
    }
    const value = await response.json().catch(() => ({}));
    if (!response.ok) {
        if (
            response.status === 401 &&
            !["/api/login", "/api/challenge"].includes(path)
        )
            window.dispatchEvent(new Event("velnix-session-expired"));
        throw new APIError(
            value.error?.message ??
                `Request unavailable (HTTP ${response.status})`,
            response.status,
        );
    }
    return value as T;
}
