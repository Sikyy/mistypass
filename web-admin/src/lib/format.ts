import i18n from "@/lib/i18n"

function locale(): string {
  return i18n.language || "id-ID"
}

export function formatDateTime(value?: string | null): string {
  if (!value) return "-"
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return String(value)
  return d.toLocaleString(locale(), {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  })
}

export function formatDate(value?: string | null): string {
  if (!value) return "-"
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return String(value)
  return d.toLocaleDateString(locale(), {
    year: "numeric",
    month: "short",
    day: "numeric",
  })
}

export function formatDateLong(value?: string | null): string {
  if (!value) return "-"
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return String(value)
  return d.toLocaleDateString(locale(), {
    weekday: "long",
    year: "numeric",
    month: "short",
    day: "numeric",
  })
}

export function formatTime(value?: string | null): string {
  if (!value) return "-"
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return String(value)
  return d.toLocaleTimeString(locale(), { hour: "2-digit", minute: "2-digit" })
}

export function formatNumber(value?: number | null): string {
  if (value == null) return "-"
  return value.toLocaleString(locale())
}

export function formatCurrency(value?: number | null, currency = "IDR"): string {
  if (value == null) return "-"
  return new Intl.NumberFormat(locale(), {
    style: "currency",
    currency,
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(value)
}
