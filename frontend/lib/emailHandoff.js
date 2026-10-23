const KEY = "apex.authEmail";

export function handOffEmail(email) {
  try {
    sessionStorage.setItem(KEY, email.trim());
  } catch {
  }
}

export function takeEmail() {
  try {
    const email = sessionStorage.getItem(KEY) || "";
    sessionStorage.removeItem(KEY);
    return email;
  } catch {
    return "";
  }
}
