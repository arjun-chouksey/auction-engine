import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { Link, useLocation, useParams } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { EmptyState, ErrorState, Loading, Pagination, StatusBadge } from "../components/States";
import { api, apiBase, ApiError } from "../lib/api";
import { countdown, formatDateTime } from "../lib/date";
import { decimalToMinor, formatMinor, sumMinor } from "../lib/money";
import type { AuctionEvent, Bid, Page, RankedBid, RFQ } from "../lib/types";

interface DetailData { rfq: RFQ; ranking: RankedBid[]; bids: Page<Bid>; events: Page<AuctionEvent>; receivedAt: number }
const emptyQuote = { carrier_name: "", freight: "", origin: "", destination: "", transit_time_hours: "", quote_valid_until: "" };

function eventDescription(event: AuctionEvent): string {
  if (event.event_type === "RFQ_CREATED") return "RFQ created and auction schedule recorded.";
  if (event.event_type === "BID_ACCEPTED") return "Supplier quote accepted into the immutable bid history.";
  const oldClose = typeof event.event_data.old_close_at === "string" ? formatDateTime(event.event_data.old_close_at) : "the prior close";
  const newClose = typeof event.event_data.new_close_at === "string" ? formatDateTime(event.event_data.new_close_at) : "the new close";
  return `Auction extended from ${oldClose} to ${newClose}.`;
}

export function AuctionDetailPage() {
  const { id = "" } = useParams(); const { token, user } = useAuth(); const location = useLocation();
  const [data, setData] = useState<DetailData | null>(null); const [error, setError] = useState(""); const [refreshing, setRefreshing] = useState(false); const [notice, setNotice] = useState((location.state as { notice?: string } | null)?.notice ?? "");
  const [bidPage, setBidPage] = useState(1); const [eventPage, setEventPage] = useState(1); const [bidSort, setBidSort] = useState("price_asc"); const [tick, setTick] = useState(Date.now());
  const load = useCallback(async (background = false) => {
    if (background) setRefreshing(true); else setData(null); setError("");
    try {
      const [rfq, ranking, bids, events] = await Promise.all([
        api<RFQ>(`/rfqs/${id}`, token ?? undefined), api<{ items: RankedBid[] }>(`/rfqs/${id}/ranking`, token ?? undefined),
        api<Page<Bid>>(`/rfqs/${id}/bids?page=${bidPage}&page_size=20&sort=${bidSort}`, token ?? undefined), api<Page<AuctionEvent>>(`/rfqs/${id}/events?page=${eventPage}&page_size=30`, token ?? undefined),
      ]);
      setData({ rfq, ranking: ranking.items, bids, events, receivedAt: Date.now() });
    } catch (reason) { setError(reason instanceof Error ? reason.message : "Auction details could not be loaded."); }
    finally { setRefreshing(false); }
  }, [bidPage, bidSort, eventPage, id, token]);
  useEffect(() => { void load(); }, [load]);
  useEffect(() => { const timer = window.setInterval(() => setTick(Date.now()), 1000); return () => clearInterval(timer); }, []);
  useEffect(() => { const interval = window.setInterval(() => void load(true), 15000); const focus = () => void load(true); window.addEventListener("focus", focus); return () => { clearInterval(interval); window.removeEventListener("focus", focus); }; }, [load]);
  useEffect(() => {
    if (!token || !id) return; const controller = new AbortController(); let retry = 1000; let timeout = 0;
    const connect = async () => {
      try {
        const response = await fetch(`${apiBase()}/api/v1/rfqs/${id}/stream`, { headers: { Authorization: `Bearer ${token}` }, signal: controller.signal });
        if (!response.ok || !response.body) throw new Error("stream unavailable"); retry = 1000; void load(true);
        const reader = response.body.getReader(); const decoder = new TextDecoder(); let buffer = "";
        while (!controller.signal.aborted) { const result = await reader.read(); if (result.done) break; buffer += decoder.decode(result.value, { stream: true }); let boundary: number; while ((boundary = buffer.indexOf("\n\n")) >= 0) { const message = buffer.slice(0, boundary); buffer = buffer.slice(boundary + 2); if (message.includes("event: rfq_changed")) void load(true); } }
      } catch { if (!controller.signal.aborted) { timeout = window.setTimeout(connect, retry); retry = Math.min(retry * 2, 15000); } }
    }; void connect(); return () => { controller.abort(); clearTimeout(timeout); };
  }, [id, load, token]);
  if (error) return <ErrorState message={error} retry={() => void load()} />; if (!data) return <Loading label="Loading auction room" />;
  const { rfq } = data; const leader = data.ranking[0]; const latestExtension = data.events.items.find((event) => event.event_type === "AUCTION_EXTENDED");
  return <>
    <div className="breadcrumbs"><Link to="/auctions">Auctions</Link><span>/</span><span>{rfq.reference_code}</span></div>
    {notice && <div className="banner success" role="status">{notice}<button aria-label="Dismiss notification" onClick={() => setNotice("")}>×</button></div>}
    <header className="auction-hero"><div><div className="hero-meta"><StatusBadge status={rfq.status} /><span>{rfq.reference_code}</span><span>Service {rfq.service_date}</span></div><h1>{rfq.name}</h1><p>{leader ? `${leader.bid.carrier_name} currently leads at ${formatMinor(leader.bid.total_minor, rfq.currency_code)}.` : "No supplier quotes have been accepted yet."}</p></div><div className="clock-rail"><span>Bidding closes in</span><strong>{rfq.status === "ACTIVE" ? countdown(rfq.current_close_at, rfq.server_time, data.receivedAt, tick) : rfq.status.replace("_", " ")}</strong><small>Current close · {formatDateTime(rfq.current_close_at)}</small><small>Absolute cap · {formatDateTime(rfq.forced_close_at)}</small></div></header>
    {latestExtension && <div className="banner extension"><strong>Closing time extended</strong><span>{eventDescription(latestExtension)}</span></div>}
    <div className="detail-grid"><div className="detail-main">
      <section className="panel"><div className="section-heading"><div><p className="eyebrow">Latest quote per supplier</p><h2>Live ranking</h2></div><span className="refresh-status" role="status">{refreshing ? "Refreshing…" : `${data.ranking.length} ranked`}</span></div>{data.ranking.length === 0 ? <EmptyState title="Ranking is open">Accepted quotes will appear here.</EmptyState> : <div className="table-scroll"><table><thead><tr><th>Rank</th><th>Supplier / carrier</th><th>Total</th><th>Breakdown</th><th>Transit</th><th>Valid until</th></tr></thead><tbody>{data.ranking.map((item) => <tr key={item.supplier_id}><td><span className="rank">L{item.rank}</span></td><td><strong>{item.bid.carrier_name}</strong><small>{item.supplier_id.slice(0, 8)}…</small></td><td className="numeric"><strong>{formatMinor(item.bid.total_minor, rfq.currency_code)}</strong></td><td><small>Freight {formatMinor(item.bid.freight_minor, rfq.currency_code)}<br />Origin {formatMinor(item.bid.origin_minor, rfq.currency_code)} · Destination {formatMinor(item.bid.destination_minor, rfq.currency_code)}</small></td><td>{item.bid.transit_time_hours}h</td><td>{item.bid.quote_valid_until}</td></tr>)}</tbody></table></div>}</section>
      {user?.role === "SUPPLIER" && <QuoteForm rfq={rfq} token={token!} onAccepted={(message) => { setNotice(message); void load(true); }} />}
      <section className="panel"><div className="section-heading"><div><p className="eyebrow">Immutable submissions</p><h2>All supplier bids</h2></div><label className="inline-control">Sort<select value={bidSort} onChange={(event) => { setBidSort(event.target.value); setBidPage(1); }}><option value="price_asc">Price: low to high</option><option value="newest">Newest first</option></select></label></div>{data.bids.items.length === 0 ? <EmptyState title="No bid history">Every accepted quote will remain visible here.</EmptyState> : <><div className="table-scroll"><table><thead><tr><th>Supplier / carrier</th><th>Total</th><th>Submitted</th><th>Transit</th><th>Valid until</th></tr></thead><tbody>{data.bids.items.map((bid) => <tr key={bid.id}><td><strong>{bid.carrier_name}</strong><small>{bid.supplier_id.slice(0, 8)}…</small></td><td className="numeric">{formatMinor(bid.total_minor, rfq.currency_code)}</td><td>{formatDateTime(bid.submitted_at)}</td><td>{bid.transit_time_hours}h</td><td>{bid.quote_valid_until}</td></tr>)}</tbody></table></div><Pagination page={data.bids.page} pageSize={data.bids.page_size} total={data.bids.total_items} onPage={setBidPage} /></>}</section>
    </div><aside className="detail-side"><section className="panel"><p className="eyebrow">Rulebook</p><h2>Auction configuration</h2><dl className="config-list"><div><dt>Trigger</dt><dd>{rfq.trigger_type.replaceAll("_", " ")}</dd></div><div><dt>Trigger window</dt><dd>{rfq.trigger_window_minutes} minutes</dd></div><div><dt>Extension</dt><dd>{rfq.extension_minutes} minutes</dd></div><div><dt>Bid start</dt><dd>{formatDateTime(rfq.bid_start_at)}</dd></div><div><dt>Original close</dt><dd>{formatDateTime(rfq.initial_close_at)}</dd></div><div><dt>Current close</dt><dd>{formatDateTime(rfq.current_close_at)}</dd></div><div><dt>Forced close</dt><dd>{formatDateTime(rfq.forced_close_at)}</dd></div></dl></section>
      <section className="panel"><div className="section-heading"><div><p className="eyebrow">Audit trail</p><h2>Activity</h2></div></div>{data.events.items.length === 0 ? <EmptyState title="No activity recorded">Auction events will appear here.</EmptyState> : <ol className="timeline">{data.events.items.map((event) => <li key={event.id}><span className="timeline-dot" /><strong>{event.event_type.replaceAll("_", " ")}</strong><p>{eventDescription(event)}</p><time>{formatDateTime(event.created_at)}</time></li>)}</ol>}<Pagination page={data.events.page} pageSize={data.events.page_size} total={data.events.total_items} onPage={setEventPage} /></section>
    </aside></div>
  </>;
}

function QuoteForm({ rfq, token, onAccepted }: { rfq: RFQ; token: string; onAccepted: (message: string) => void }) {
  const [quote, setQuote] = useState(emptyQuote); const [errors, setErrors] = useState<Record<string, string>>({}); const [serverError, setServerError] = useState(""); const [busy, setBusy] = useState(false); const key = useRef<string | null>(null); const fingerprint = useRef("");
  const parsed = useMemo(() => { try { return [decimalToMinor(quote.freight || "0"), decimalToMinor(quote.origin || "0"), decimalToMinor(quote.destination || "0")]; } catch { return null; } }, [quote.destination, quote.freight, quote.origin]);
  const total = parsed ? sumMinor(...parsed) : null; const active = rfq.status === "ACTIVE";
  function update(name: keyof typeof emptyQuote, value: string) { setQuote((current) => ({ ...current, [name]: value })); setErrors((current) => ({ ...current, [name]: "" })); setServerError(""); }
  async function submit(event: FormEvent) {
    event.preventDefault(); const next: Record<string, string> = {}; let money: string[] | null = null;
    if (!quote.carrier_name.trim()) next.carrier_name = "Enter a carrier name.";
    for (const name of ["freight", "origin", "destination"] as const) { try { decimalToMinor(quote[name]); } catch (reason) { next[name] = reason instanceof Error ? reason.message : "Enter a valid amount."; } }
    try { money = [decimalToMinor(quote.freight), decimalToMinor(quote.origin), decimalToMinor(quote.destination)]; } catch { money = null; }
    if (Number(quote.transit_time_hours) <= 0) next.transit_time_hours = "Enter a positive number of hours."; if (!quote.quote_valid_until) next.quote_valid_until = "Choose a quote-valid-until date.";
    setErrors(next); if (Object.keys(next).length || !money) return;
    const payload = { carrier_name: quote.carrier_name.trim(), freight_minor: money[0], origin_minor: money[1], destination_minor: money[2], transit_time_hours: Number(quote.transit_time_hours), quote_valid_until: quote.quote_valid_until };
    const serialized = JSON.stringify(payload); if (!key.current || fingerprint.current !== serialized) { key.current = crypto.randomUUID(); fingerprint.current = serialized; }
    setBusy(true); setServerError("");
    try {
      const result = await api<{ current_rank: number; extended: boolean; idempotent_replay: boolean }>(`/rfqs/${rfq.id}/bids`, token, { method: "POST", headers: { "Idempotency-Key": key.current }, body: serialized });
      onAccepted(result.idempotent_replay ? `Quote already accepted. Your current rank is L${result.current_rank}.` : `Quote accepted at rank L${result.current_rank}${result.extended ? "; the auction close was extended" : ""}.`);
      setQuote(emptyQuote); setErrors({}); key.current = null; fingerprint.current = "";
    } catch (reason) { setServerError(reason instanceof ApiError ? `${reason.code.replaceAll("_", " ")}: ${reason.message}` : "The quote could not be submitted. Retry keeps the same request key."); if (reason instanceof ApiError && reason.code === "AUCTION_CLOSED") onAccepted("The auction has closed. The latest state has been refreshed."); }
    finally { setBusy(false); }
  }
  return <section className="panel quote-panel"><div className="section-heading"><div><p className="eyebrow">Supplier workspace</p><h2>Submit a quote</h2></div>{total && <div className="quote-total"><span>Computed total</span><strong>{formatMinor(total, rfq.currency_code)}</strong></div>}</div>{!active && <div className="banner warning">This auction is {rfq.status.toLowerCase().replace("_", " ")}; new quotes are disabled.</div>}{serverError && <div className="banner error" role="alert">{serverError}</div>}<form noValidate onSubmit={submit}><div className="form-grid three"><QuoteField label="Carrier name" name="carrier_name" value={quote.carrier_name} error={errors.carrier_name} update={update} /><QuoteField label="Freight (INR)" name="freight" value={quote.freight} error={errors.freight} update={update} inputMode="decimal" /><QuoteField label="Origin charges (INR)" name="origin" value={quote.origin} error={errors.origin} update={update} inputMode="decimal" /><QuoteField label="Destination charges (INR)" name="destination" value={quote.destination} error={errors.destination} update={update} inputMode="decimal" /><QuoteField label="Transit time (hours)" name="transit_time_hours" value={quote.transit_time_hours} error={errors.transit_time_hours} update={update} type="number" /><QuoteField label="Quote valid until" name="quote_valid_until" value={quote.quote_valid_until} error={errors.quote_valid_until} update={update} type="date" /></div><div className="form-actions"><small>Retries of an unchanged quote reuse one idempotency key.</small><button className="button primary" disabled={!active || busy}>{busy ? "Submitting quote…" : "Submit quote"}</button></div></form></section>;
}

function QuoteField({ label, name, value, error, update, type = "text", inputMode }: { label: string; name: keyof typeof emptyQuote; value: string; error?: string; update: (name: keyof typeof emptyQuote, value: string) => void; type?: string; inputMode?: "decimal" }) {
  return <label>{label}<input type={type} inputMode={inputMode} value={value} onChange={(event) => update(name, event.target.value)} aria-invalid={Boolean(error)} aria-describedby={error ? `${name}-error` : undefined} />{error && <small className="field-error" id={`${name}-error`}>{error}</small>}</label>;
}
