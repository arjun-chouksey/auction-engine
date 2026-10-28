import { useState, type FormEvent } from "react";
import { Navigate, useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";

const accounts = ["buyer@example.test", "supplier.a@example.test", "supplier.b@example.test", "supplier.c@example.test"];

export function LoginPage() {
  const auth = useAuth(); const navigate = useNavigate(); const location = useLocation();
  const [email, setEmail] = useState(accounts[0]); const [password, setPassword] = useState("AuctionDemo2026!");
  const [showPassword, setShowPassword] = useState(false); const [error, setError] = useState(""); const [busy, setBusy] = useState(false);
  if (auth.user) return <Navigate to="/auctions" replace />;
  async function submit(event: FormEvent) {
    event.preventDefault(); setError("");
    if (!email.trim() || !password) { setError("Enter both an email address and password."); return; }
    setBusy(true);
    try { await auth.login(email, password); navigate((location.state as { from?: string } | null)?.from ?? "/auctions", { replace: true }); }
    catch (reason) { setError(reason instanceof Error ? reason.message : "Sign in failed."); setPassword(""); }
    finally { setBusy(false); }
  }
  return <main className="login-page">
    <section className="login-intro"><div className="brand-mark large">BA</div><p className="eyebrow">Reverse procurement, precisely timed</p><h1>Every quote.<br />One clear rank.</h1><p>Run auditable British auctions with deterministic extensions and a complete bid history.</p></section>
    <section className="login-panel" aria-labelledby="login-heading"><div><p className="eyebrow">Local demonstration</p><h2 id="login-heading">Sign in to the auction room</h2></div>
      {error && <div className="banner error" role="alert">{error}</div>}
      <form noValidate onSubmit={submit}>
        <label>Email address<select value={email} onChange={(event) => setEmail(event.target.value)} autoComplete="username">{accounts.map((account) => <option key={account}>{account}</option>)}</select></label>
        <label>Password<span className="password-field"><input type={showPassword ? "text" : "password"} value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="current-password" /><button type="button" className="button ghost compact" aria-label={showPassword ? "Hide password" : "Show password"} aria-pressed={showPassword} onClick={() => setShowPassword((value) => !value)}>{showPassword ? "Hide" : "Show"}</button></span></label>
        <button className="button primary full" disabled={busy}>{busy ? "Signing in…" : "Sign in"}</button>
      </form>
      <p className="demo-note"><strong>Demo only.</strong> These local accounts share the password shown above. Never reuse it outside this project.</p>
    </section>
  </main>;
}
