import { Link, NavLink, Outlet } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";

export function AppShell() {
  const { user, logout } = useAuth();
  return <div className="app-frame">
    <header className="topbar">
      <Link className="brand" to="/auctions"><span className="brand-mark">BA</span><span>British Auction</span></Link>
      <nav aria-label="Primary navigation">
        <NavLink to="/auctions">Auctions</NavLink>
        {user?.role === "BUYER" && <NavLink to="/auctions/new">Create RFQ</NavLink>}
      </nav>
      <div className="account"><span><strong>{user?.full_name}</strong><small>{user?.role}</small></span><button className="button ghost compact" onClick={logout}>Sign out</button></div>
    </header>
    <main className="page"><Outlet /></main>
  </div>;
}
