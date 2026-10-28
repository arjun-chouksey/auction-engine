import { Link } from "react-router-dom";

export function NotFoundPage() {
  return (
    <main className="shell">
      <p className="eyebrow">404</p>
      <h1>Page not found</h1>
      <p className="lede">
        This route is not part of the current project shell.
      </p>
      <Link className="link" to="/">
        Return home
      </Link>
    </main>
  );
}
