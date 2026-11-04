import { Outlet, useNavigate, Link } from "@tanstack/react-router";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/features/auth/use-auth";
import { LogOut, MessagesSquare } from "lucide-react";

export function RootLayout() {
  const { logout } = useAuth();
  const navigate = useNavigate();
  return (
    <div className="flex h-screen flex-col">
      <header className="flex items-center justify-between border-b px-4 py-2">
        <div className="flex items-center gap-2 font-semibold text-primary">
          <MessagesSquare className="h-5 w-5" /> Pancake Inbox
        </div>
        <nav className="flex gap-4 text-sm">
          <Link to="/" className="hover:text-primary [&.active]:text-primary [&.active]:font-medium">Inbox</Link>
          <Link to="/search" className="hover:text-primary [&.active]:text-primary [&.active]:font-medium">Tìm kiếm</Link>
          <Link to="/dashboard" className="hover:text-primary [&.active]:text-primary [&.active]:font-medium">Dashboard</Link>
        </nav>
        <Button variant="ghost" size="sm" onClick={() => { logout(); navigate({ to: "/login" }); }}>
          <LogOut className="mr-1 h-4 w-4" /> Đăng xuất
        </Button>
      </header>
      <main className="flex-1 overflow-hidden">
        <Outlet />
      </main>
    </div>
  );
}
