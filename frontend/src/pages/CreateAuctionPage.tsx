import { useRef, useState, type FormEvent } from "react";
import { Navigate, useNavigate } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { api } from "../lib/api";
import { localDateTimeToRFC3339, localTimeZone } from "../lib/date";
import type { RFQ, TriggerType } from "../lib/types";

interface FormState { name: string; reference_code: string; service_date: string; bid_start_at: string; initial_close_at: string; forced_close_at: string; trigger_window_minutes: string; extension_minutes: string; trigger_type: TriggerType }
const initial: FormState = { name: "", reference_code: "", service_date: "", bid_start_at: "", initial_close_at: "", forced_close_at: "", trigger_window_minutes: "10", extension_minutes: "5", trigger_type: "L1_CHANGE" };
const triggerHelp: Record<TriggerType, string> = { ANY_BID: "Extend after any accepted bid in the final trigger window.", ANY_RANK_CHANGE: "Extend when the order of ranked suppliers changes in the final trigger window.", L1_CHANGE: "Extend only when another supplier becomes the lowest bidder in the final trigger window." };

export function CreateAuctionPage() {
  const { token, user } = useAuth(); const navigate = useNavigate(); const [form, setForm] = useState(initial); const [errors, setErrors] = useState<Record<string, string>>({}); const [serverError, setServerError] = useState(""); const [busy, setBusy] = useState(false); const firstError = useRef<HTMLInputElement>(null);
  if (user?.role !== "BUYER") return <Navigate to="/forbidden" replace />;
  function update(name: keyof FormState, value: string) { setForm((current) => ({ ...current, [name]: value })); setErrors((current) => ({ ...current, [name]: "" })); }
  async function submit(event: FormEvent) {
    event.preventDefault(); const next: Record<string, string> = {};
    if (!form.name.trim()) next.name = "Enter an RFQ name."; if (!form.reference_code.trim()) next.reference_code = "Enter a reference code."; if (!form.service_date) next.service_date = "Choose a service date.";
    const start = new Date(form.bid_start_at).getTime(), initialClose = new Date(form.initial_close_at).getTime(), forced = new Date(form.forced_close_at).getTime();
    if (!form.bid_start_at) next.bid_start_at = "Choose a bid start."; if (!form.initial_close_at) next.initial_close_at = "Choose an initial close."; if (!form.forced_close_at) next.forced_close_at = "Choose a forced close.";
    if (form.bid_start_at && form.initial_close_at && !(start < initialClose)) next.initial_close_at = "Initial close must be after bid start.";
    if (form.initial_close_at && form.forced_close_at && !(initialClose < forced)) next.forced_close_at = "Forced close must be after initial close.";
    if (Number(form.trigger_window_minutes) <= 0) next.trigger_window_minutes = "Enter a positive number of minutes."; if (Number(form.extension_minutes) <= 0) next.extension_minutes = "Enter a positive number of minutes.";
    setErrors(next); if (Object.keys(next).length) { requestAnimationFrame(() => firstError.current?.focus()); return; }
    setBusy(true); setServerError("");
    try {
      const created = await api<RFQ>("/rfqs", token ?? undefined, { method: "POST", body: JSON.stringify({ ...form, currency_code: "INR", bid_start_at: localDateTimeToRFC3339(form.bid_start_at), initial_close_at: localDateTimeToRFC3339(form.initial_close_at), forced_close_at: localDateTimeToRFC3339(form.forced_close_at), trigger_window_minutes: Number(form.trigger_window_minutes), extension_minutes: Number(form.extension_minutes) }) });
      navigate(`/auctions/${created.id}`, { state: { notice: "RFQ created and ready for suppliers." } });
    } catch (reason) { setServerError(reason instanceof Error ? reason.message : "The RFQ could not be created."); }
    finally { setBusy(false); }
  }
  const field = (name: keyof FormState, label: string, type = "text") => <label>{label}<input ref={errors[name] && !Object.values(errors).slice(0, Object.keys(errors).indexOf(name)).some(Boolean) ? firstError : undefined} type={type} value={form[name]} onChange={(event) => update(name, event.target.value)} aria-invalid={Boolean(errors[name])} aria-describedby={errors[name] ? `${name}-error` : undefined} />{errors[name] && <small className="field-error" id={`${name}-error`}>{errors[name]}</small>}</label>;
  return <><header className="page-heading"><div><p className="eyebrow">Buyer workspace</p><h1>Create an RFQ</h1><p>Set the auction window and the exact rule that can extend it.</p></div></header>
    {serverError && <div className="banner error" role="alert">{serverError}</div>}
    <form className="form-card" noValidate onSubmit={submit}><fieldset><legend>RFQ details</legend><div className="form-grid">{field("name", "RFQ name")}{field("reference_code", "Reference code")}{field("service_date", "Service date", "date")}<label>Currency<input value="INR" disabled /><small>Amounts use two decimal places.</small></label></div></fieldset>
      <fieldset><legend>Auction timing</legend><p className="fieldset-note">Times are entered in {localTimeZone} and sent with an unambiguous UTC offset.</p><div className="form-grid three">{field("bid_start_at", "Bid start", "datetime-local")}{field("initial_close_at", "Initial close", "datetime-local")}{field("forced_close_at", "Absolute forced close", "datetime-local")}</div></fieldset>
      <fieldset><legend>Extension rule</legend><div className="form-grid three"><label>Trigger policy<select value={form.trigger_type} onChange={(event) => update("trigger_type", event.target.value)}><option value="ANY_BID">Any accepted bid</option><option value="ANY_RANK_CHANGE">Any rank-order change</option><option value="L1_CHANGE">Another supplier becomes L1</option></select><small>{triggerHelp[form.trigger_type]}</small></label>{field("trigger_window_minutes", "Trigger window (minutes)", "number")}{field("extension_minutes", "Extension duration (minutes)", "number")}</div></fieldset>
      <div className="form-actions"><button type="button" className="button ghost" onClick={() => navigate("/auctions")}>Cancel</button><button className="button primary" disabled={busy}>{busy ? "Creating RFQ…" : "Create RFQ"}</button></div></form></>;
}
