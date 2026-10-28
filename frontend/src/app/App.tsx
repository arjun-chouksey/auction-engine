import { useEffect, type ReactNode } from "react";
import { Navigate, Route, Routes, useLocation } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { AppShell } from "../components/AppShell";
import { AuctionDetailPage } from "../pages/AuctionDetailPage";
import { AuctionsPage } from "../pages/AuctionsPage";
import { CreateAuctionPage } from "../pages/CreateAuctionPage";
import { LoginPage } from "../pages/LoginPage";
import { NotFoundPage } from "./NotFoundPage";

function Protected({ children }: { children: ReactNode }) {
  const { user } = useAuth(); const location = useLocation();
  return user ? children : <Navigate to="/login" state={{ from: location.pathname + location.search }} replace />;
}

function ForbiddenPage() {
  return <section className="state-card error"><p className="eyebrow">403</p><h1>Access denied</h1><p>Your account does not have permission to open this page.</p><a className="button outline" href="/auctions">Return to auctions</a></section>;
}

export function App() {
  const location = useLocation();
  useEffect(() => {
    const title = location.pathname === "/login" ? "Sign in" : location.pathname === "/auctions" ? "British Auctions" : location.pathname === "/auctions/new" ? "Create RFQ" : location.pathname.startsWith("/auctions/") ? "Auction details" : location.pathname === "/forbidden" ? "Access denied" : "Page not found";
    document.title = `${title} | British Auction RFQ`;
  }, [location.pathname]);
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/" element={<Protected><AppShell /></Protected>}>
        <Route index element={<Navigate to="/auctions" replace />} />
        <Route path="auctions" element={<AuctionsPage />} />
        <Route path="auctions/new" element={<CreateAuctionPage />} />
        <Route path="auctions/:id" element={<AuctionDetailPage />} />
        <Route path="forbidden" element={<ForbiddenPage />} />
      </Route>
      <Route path="/404" element={<NotFoundPage />} />
      <Route path="*" element={<Navigate to="/404" replace />} />
    </Routes>
  );
}
