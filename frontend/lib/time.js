// The API sends UTC times as "2026-10-13 21:35:45" (SQLite's format).
export function parseTime(s) {
  return new Date(s.replace(" ", "T") + "Z");
}

// "just now", "5 min ago", "3 h ago", "2 d ago", then the date.
export function timeAgo(s) {
  const seconds = Math.max(0, (Date.now() - parseTime(s).getTime()) / 1000);
  if (seconds < 60) return "just now";
  if (seconds < 3600) return `${Math.floor(seconds / 60)} min ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} h ago`;
  if (seconds < 7 * 86400) return `${Math.floor(seconds / 86400)} d ago`;
  return parseTime(s).toLocaleDateString();
}

// "Sat, 1 Nov 2026, 18:00" in the viewer's own time zone (event times).
export function formatEventTime(s) {
  return parseTime(s).toLocaleString(undefined, {
    weekday: "short",
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
