// Fetch wrapper used by every page. Paths are relative ("/api/..."), so they go
// through the Next.js rewrite to the backend and cookies are sent automatically.

// fields is the response's { "<field>": "<problem>" } map when it has one
// (register reports every bad field at once), otherwise null.
export class ApiError extends Error {
  constructor(status, message, fields = null) {
    super(message);
    this.status = status;
    this.fields = fields;
  }
}

// Pages a logged-out visitor may see. Everything else needs a session.
export const PUBLIC_PAGES = ["/", "/login", "/register"];

/**
 * api("/api/me")
 * api("/api/login", { method: "POST", body: { email, password } })
 * api("/api/posts", { method: "POST", body: formData })   // multipart
 * Pass { redirectOn401: false } to handle 401 yourself.
 */
export async function api(path, { method = "GET", body, redirectOn401 = true } = {}) {
  const opts = { method, credentials: "include", headers: {} };
  if (body instanceof FormData) {
    opts.body = body; // browser sets the multipart boundary
  } else if (body !== undefined) {
    opts.headers["Content-Type"] = "application/json";
    opts.body = JSON.stringify(body);
  }

  const res = await fetch(path, opts);
  const data = res.status === 204 ? null : await res.json().catch(() => null);

  if (!res.ok) {
    if (
      res.status === 401 &&
      redirectOn401 &&
      typeof window !== "undefined" &&
      !PUBLIC_PAGES.includes(window.location.pathname)
    ) {
      // Replace, not push: Back from /login would land on this page, find
      // no session and be sent to /login again.
      window.location.replace("/login");
    }
    throw new ApiError(res.status, data?.error || `Request failed (${res.status})`, data?.fields || null);
  }
  return data;
}
