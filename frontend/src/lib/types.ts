export type Role = "BUYER" | "SUPPLIER";
export type RFQStatus = "UPCOMING" | "ACTIVE" | "CLOSED" | "FORCE_CLOSED";
export type TriggerType = "ANY_BID" | "ANY_RANK_CHANGE" | "L1_CHANGE";

export interface User { id: string; full_name: string; email: string; role: Role }
export interface RFQ {
  id: string; reference_code: string; name: string; buyer_id: string; currency_code: string;
  service_date: string; bid_start_at: string; initial_close_at: string; current_close_at: string;
  forced_close_at: string; trigger_window_minutes: number; extension_minutes: number;
  trigger_type: TriggerType; status: RFQStatus; lowest_bid_minor: string | null;
  total_bid_count: number; unique_supplier_count: number; server_time: string;
}
export interface Bid {
  id: string; rfq_id: string; supplier_id: string; carrier_name: string; freight_minor: string;
  origin_minor: string; destination_minor: string; total_minor: string;
  transit_time_hours: number; quote_valid_until: string; submitted_at: string;
}
export interface RankedBid { rank: number; supplier_id: string; bid: Bid }
export interface AuctionEvent {
  id: string; rfq_id: string; bid_id: string | null; actor_id: string | null;
  event_type: "RFQ_CREATED" | "BID_ACCEPTED" | "AUCTION_EXTENDED";
  event_data: Record<string, unknown>; created_at: string;
}
export interface Page<T> { items: T[]; page: number; page_size: number; total_items: number; server_time: string }
export interface ApiErrorBody { error?: { code?: string; message?: string; details?: Record<string, unknown> } }
