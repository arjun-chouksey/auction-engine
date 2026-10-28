const minorPattern = /^-?\d+$/;

export function formatMinor(minor: string | null, currency = "INR"): string {
  if (minor === null) return "—";
  if (!minorPattern.test(minor)) return "Invalid amount";
  const negative = minor.startsWith("-");
  const digits = negative ? minor.slice(1) : minor;
  const padded = digits.padStart(3, "0");
  const major = padded.slice(0, -2).replace(/\B(?=(\d{3})+(?!\d))/g, ",");
  const fraction = padded.slice(-2);
  return `${negative ? "−" : ""}${currency} ${major}.${fraction}`;
}

export function decimalToMinor(value: string, exponent = 2): string {
  const trimmed = value.trim();
  const match = trimmed.match(/^(\d+)(?:\.(\d*))?$/);
  if (!match) throw new Error("Enter a non-negative amount using digits and an optional decimal point.");
  const fraction = match[2] ?? "";
  if (fraction.length > exponent) throw new Error(`Use no more than ${exponent} decimal places.`);
  const combined = `${match[1]}${fraction.padEnd(exponent, "0")}`.replace(/^0+(?=\d)/, "");
  return combined || "0";
}

export function sumMinor(...values: string[]): string {
  return values.reduce((total, value) => total + BigInt(value || "0"), 0n).toString();
}
