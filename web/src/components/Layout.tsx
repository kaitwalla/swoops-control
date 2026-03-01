import { NavLink, Outlet } from 'react-router-dom';
import { LayoutDashboard, Server, Terminal, Puzzle, FileSliders } from 'lucide-react';
import { UpdateBanner } from './UpdateBanner';

const navItems = [
  { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/hosts', label: 'Hosts', icon: Server },
  { to: '/sessions', label: 'Sessions', icon: Terminal },
  { to: '/plugins', label: 'Plugins', icon: Puzzle },
  { to: '/templates', label: 'Templates', icon: FileSliders },
];

export function Layout() {
  return (
    <div className="flex h-screen bg-gray-950 text-gray-100">
      <aside className="w-56 border-r border-gray-800 flex flex-col">
        <div className="p-4 border-b border-gray-800">
          <h1 className="text-lg font-bold tracking-tight">Swoops</h1>
          <p className="text-xs text-gray-500">Agent Orchestrator</p>
        </div>
        <nav className="flex-1 p-2 space-y-1">
          {navItems.map(({ to, label, icon: Icon }) => (
            <NavLink
              key={to}
              to={to}
              className={({ isActive }) =>
                `flex items-center gap-2 px-3 py-2 rounded-md text-sm transition-colors ${
                  isActive
                    ? 'bg-gray-800 text-white'
                    : 'text-gray-400 hover:bg-gray-800/50 hover:text-gray-200'
                }`
              }
            >
              <Icon size={16} />
              {label}
            </NavLink>
          ))}
        </nav>
      </aside>
      <main className="flex-1 overflow-auto flex flex-col">
        <UpdateBanner />
        <div className="flex-1 overflow-auto">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
