export class ApiError extends Error {
  constructor(status, message, fields = null, body = null) {
    super(message);
    this.status = status;
    this.fields = fields;
    this.body = body;
  }
}

export const PUBLIC_PAGES = ["/", "/login", "/register", "/forgot-password"];

export async function api(path, { method = "GET", body, redirectOn401 = true } = {}) {
  const opts = { method, credentials: "include", headers: {} };
  if (body instanceof FormData) {
    opts.body = body;
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
      window.location.replace("/login");
    }
    throw new ApiError(res.status, data?.error || `Request failed (${res.status})`, data?.fields || null, data);
  }
  return data;
}
