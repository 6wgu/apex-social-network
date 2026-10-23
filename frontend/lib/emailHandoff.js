// Carries the email typed on one sign-in page to the next (login, forgot
// password, back to login), so it need not be typed again. It goes through
// sessionStorage, not the URL, to keep the address out of the history and
// the server logs.
const KEY = "apex.authEmail";

export function handOffEmail(email) {
  try {
    sessionStorage.setItem(KEY, email.trim());
  } catch {
    // Storage is blocked: the next page simply starts empty.
  }
}

// The email the page before handed over, once; "" when there is none.
export function takeEmail() {
  try {
    const email = sessionStorage.getItem(KEY) || "";
    sessionStorage.removeItem(KEY);
    return email;
  } catch {
    return "";
  }
}
