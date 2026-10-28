export function HomePage() {
  return (
    <main className="shell">
      <p className="eyebrow">Local development harness</p>
      <h1>British Auction RFQ</h1>
      <p className="lede">
        The project foundation is ready. Authentication, auctions, bidding, and
        live updates are the next implementation milestones.
      </p>
      <section className="status" aria-labelledby="foundation-status">
        <h2 id="foundation-status">Foundation status</h2>
        <ul>
          <li>Go API health endpoint</li>
          <li>React router shell</li>
          <li>PostgreSQL development service</li>
          <li>Migration runner ready for schema migrations</li>
        </ul>
      </section>
    </main>
  );
}
