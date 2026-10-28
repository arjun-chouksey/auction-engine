import type { ReactNode } from "react";

export function Loading({ label = "Loading" }: { label?: string }) {
  return <div className="state-card" role="status"><span className="spinner" aria-hidden="true" />{label}…</div>;
}
export function ErrorState({ message, retry }: { message: string; retry?: () => void }) {
  return <div className="state-card error" role="alert"><strong>Could not load this view</strong><p>{message}</p>{retry && <button className="button outline" onClick={retry}>Try again</button>}</div>;
}
export function EmptyState({ title, children }: { title: string; children: ReactNode }) {
  return <div className="state-card"><strong>{title}</strong><p>{children}</p></div>;
}
export function StatusBadge({ status }: { status: string }) {
  return <span className={`status-badge status-${status.toLowerCase().replace("_", "-")}`}>{status.replace("_", " ")}</span>;
}
export function Pagination({ page, pageSize, total, onPage }: { page: number; pageSize: number; total: number; onPage: (page: number) => void }) {
  const last = Math.max(1, Math.ceil(total / pageSize));
  return <div className="pagination" aria-label="Pagination"><span>{total === 0 ? "0" : `${(page - 1) * pageSize + 1}–${Math.min(page * pageSize, total)}`} of {total}</span><div><button className="button ghost compact" disabled={page <= 1} onClick={() => onPage(page - 1)}>Previous</button><span>Page {page} of {last}</span><button className="button ghost compact" disabled={page >= last} onClick={() => onPage(page + 1)}>Next</button></div></div>;
}
