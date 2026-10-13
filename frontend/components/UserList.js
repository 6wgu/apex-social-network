import Link from "next/link";
import Avatar from "@/components/Avatar";

// A list of UserSummary rows linking to each profile.
export default function UserList({ users, empty, meId }) {
  if (users.length === 0) return <p className="muted tab-placeholder">{empty}</p>;
  return (
    <ul className="people">
      {users.map((p) => (
        <li key={p.id}>
          <Link href={`/profile/${p.id}`}>
            <Avatar user={p} size={40} />
            <span>
              <strong>
                {p.first_name} {p.last_name}
              </strong>
              {p.nickname && <span className="muted"> @{p.nickname}</span>}
              {p.id === meId && <span className="muted"> (you)</span>}
            </span>
          </Link>
        </li>
      ))}
    </ul>
  );
}
