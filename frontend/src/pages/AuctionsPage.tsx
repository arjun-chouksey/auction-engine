import { useCallback, useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { EmptyState, ErrorState, Loading, Pagination, StatusBadge } from "../components/States";
import { api } from "../lib/api";
import { formatDateTime } from "../lib/date";
import { formatMinor } from "../lib/money";
import type { Page, RFQ, RFQStatus } from "../lib/types";

const statuses: Array<{ label: string; value: "" | RFQStatus }> = [{ label: "All statuses", value: "" }, { label: "Upcoming", value: "UPCOMING" }, { label: "Active", value: "ACTIVE" }, { label: "Closed", value: "CLOSED" }, { label: "Force closed", value: "FORCE_CLOSED" }];

export function AuctionsPage() {
  const { token, user } = useAuth(); const [params, setParams] = useSearchParams();
  const page = Math.max(1, Number(params.get("page")) || 1); const status = params.get("status") ?? "";
  const [data, setData] = useState<Page<RFQ> | null>(null); const [error, setError] = useState(""); const [refreshing, setRefreshing] = useState(false);
  const load = useCallback(async (background = false) => {
    if (background) setRefreshing(true); else setData(null); setError("");
    try { setData(await api<Page<RFQ>>(`/rfqs?page=${page}&page_size=20${status ? `&status=${status}` : ""}`, token ?? undefined)); }
    catch (reason) { setError(reason instanceof Error ? reason.message : "Auctions could not be loaded."); }
    finally { setRefreshing(false); }
  }, [page, status, token]);
  useEffect(() => { void load(); }, [load]);
  useEffect(() => { const interval = window.setInterval(() => void load(true), 15000); const focus = () => void load(true); window.addEventListener("focus", focus); return () => { clearInterval(interval); window.removeEventListener("focus", focus); }; }, [load]);
  function update(key: string, value: string) { const next = new URLSearchParams(params); value ? next.set(key, value) : next.delete(key); if (key === "status") next.delete("page"); setParams(next); }
  return <>
    <header className="page-heading"><div><p className="eyebrow">Procurement desk</p><h1>British Auctions</h1><p>Live and scheduled RFQs, with every deadline shown in your local timezone.</p></div>{user?.role === "BUYER" && <Link className="button primary" to="/auctions/new">Create RFQ</Link>}</header>
    <section className="toolbar"><label>Status<select value={status} onChange={(event) => update("status", event.target.value)}>{statuses.map((item) => <option value={item.value} key={item.value}>{item.label}</option>)}</select></label><span className="refresh-status" role="status">{refreshing ? "Refreshing…" : "Auto-refreshes every 15 seconds"}</span></section>
    {error ? <ErrorState message={error} retry={() => void load()} /> : !data ? <Loading label="Loading auctions" /> : data.items.length === 0 ? <EmptyState title="No auctions match this view">Change the status filter{user?.role === "BUYER" ? " or create the first RFQ" : " to find another auction"}.</EmptyState> : <section className="table-panel"><div className="table-scroll"><table><thead><tr><th>RFQ</th><th>Status</th><th>Lowest total</th><th>Current close</th><th>Forced close</th><th>Bids</th><th><span className="sr-only">Action</span></th></tr></thead><tbody>{data.items.map((rfq) => <tr key={rfq.id}><td><strong>{rfq.name}</strong><small>{rfq.reference_code}</small></td><td><StatusBadge status={rfq.status} /></td><td className="numeric">{formatMinor(rfq.lowest_bid_minor, rfq.currency_code)}</td><td>{formatDateTime(rfq.current_close_at)}</td><td>{formatDateTime(rfq.forced_close_at)}</td><td>{rfq.total_bid_count}<small>{rfq.unique_supplier_count} suppliers</small></td><td><Link className="button outline compact" to={`/auctions/${rfq.id}`}>Open</Link></td></tr>)}</tbody></table></div><Pagination page={data.page} pageSize={data.page_size} total={data.total_items} onPage={(value) => update("page", String(value))} /></section>}
  </>;
}
