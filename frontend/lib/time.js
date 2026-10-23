export function parseTime(s) {
  return new Date(s.replace(" ", "T") + "Z");
}

export function timeAgo(s) {
  const seconds = Math.max(0, (Date.now() - parseTime(s).getTime()) / 1000);
  if (seconds < 60) return "just now";
  if (seconds < 3600) return `${Math.floor(seconds / 60)} min ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} h ago`;
  if (seconds < 7 * 86400) return `${Math.floor(seconds / 86400)} d ago`;
  return parseTime(s).toLocaleDateString();
}

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
