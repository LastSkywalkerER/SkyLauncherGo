import { NavLink } from "react-router-dom";

const links = [
  { to: "/", icon: "pi-th-large", label: "Instances" },
  { to: "/browse", icon: "pi-search", label: "Browse" },
  { to: "/login", icon: "pi-user", label: "Account" },
  { to: "/settings", icon: "pi-cog", label: "Settings" },
];

export function Sidebar() {
  return (
    <aside className="flex w-56 flex-col gap-1 border-r border-white/10 bg-surface p-3">
      <div className="px-3 pb-4 text-lg font-semibold">SkyLauncher</div>
      {links.map((l) => (
        <NavLink
          key={l.to}
          to={l.to}
          end={l.to === "/"}
          className={({ isActive }) =>
            `flex items-center gap-3 rounded-md px-3 py-2 text-sm transition ${
              isActive ? "bg-accent/20 text-white" : "text-white/80 hover:bg-white/5"
            }`
          }
        >
          <i className={`pi ${l.icon}`} />
          <span>{l.label}</span>
        </NavLink>
      ))}
    </aside>
  );
}
