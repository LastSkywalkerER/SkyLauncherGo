import { useEffect, useState } from "react";

import { Auth, type UserProfile } from "../wails/bindings";

export function TopBar() {
  const [user, setUser] = useState<UserProfile | null>(null);

  useEffect(() => {
    Auth.current()
      .then((u) => (u && u.name ? setUser(u) : setUser(null)))
      .catch(() => setUser(null));
  }, []);

  return (
    <header className="flex items-center justify-between border-b border-white/10 px-6 py-3">
      <div className="text-sm text-white/60">SkyLauncher</div>
      <div className="text-sm">
        {user ? (
          <span>
            <span className="text-white/60">Signed in as</span>{" "}
            <span className="font-medium">{user.name}</span>{" "}
            <span className="text-white/40">({user.type})</span>
          </span>
        ) : (
          <span className="text-white/60">Not signed in</span>
        )}
      </div>
    </header>
  );
}
