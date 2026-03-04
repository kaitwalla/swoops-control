import { NavLink, Outlet, useNavigate } from 'react-router-dom';
import { LayoutDashboard, Server, Terminal, Puzzle, FileSliders, LogOut, User } from 'lucide-react';
import { UpdateBanner } from './UpdateBanner';
import { useAuthStore } from '../stores/authStore';

const navItems = [
  { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/hosts', label: 'Hosts', icon: Server },
  { to: '/sessions', label: 'Sessions', icon: Terminal },
  { to: '/plugins', label: 'Plugins', icon: Puzzle },
  { to: '/templates', label: 'Templates', icon: FileSliders },
];

export function Layout() {
  const navigate = useNavigate();
  const { user, logout } = useAuthStore();

  const handleLogout = async () => {
    await logout();
    navigate('/login');
  };

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

        {/* User menu at bottom */}
        <div className="p-2 border-t border-gray-800">
          <div className="px-3 py-2 text-xs text-gray-500 flex items-center gap-2">
            <User size={14} />
            <span className="truncate">{user?.username || 'User'}</span>
          </div>
          <button
            onClick={handleLogout}
            className="w-full flex items-center gap-2 px-3 py-2 rounded-md text-sm text-gray-400 hover:bg-gray-800/50 hover:text-gray-200 transition-colors"
          >
            <LogOut size={16} />
            Logout
          </button>
        </div>
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
