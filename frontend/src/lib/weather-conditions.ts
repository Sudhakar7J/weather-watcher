import {
  Cloud,
  CloudDrizzle,
  CloudFog,
  CloudLightning,
  CloudRain,
  CloudSnow,
  CloudSun,
  Sun,
  type LucideIcon,
} from 'lucide-react'

/**
 * Broad condition categories derived from the WMO weather codes.
 *
 * The card's colour comes from these rather than from the raw code, so the
 * palette stays small and intentional. Colour here carries information — it
 * tells you the conditions at a glance without reading the label.
 */
export type ConditionKind =
  | 'clear'
  | 'cloudy'
  | 'overcast'
  | 'fog'
  | 'drizzle'
  | 'rain'
  | 'snow'
  | 'thunder'

interface ConditionStyle {
  icon: LucideIcon
  /** Sentence-case label for the card, shorter than the API description. */
  label: string
}

const STYLES: Record<ConditionKind, ConditionStyle> = {
  clear: { icon: Sun, label: 'Clear' },
  cloudy: { icon: CloudSun, label: 'Partly cloudy' },
  overcast: { icon: Cloud, label: 'Overcast' },
  fog: { icon: CloudFog, label: 'Fog' },
  drizzle: { icon: CloudDrizzle, label: 'Drizzle' },
  rain: { icon: CloudRain, label: 'Rain' },
  snow: { icon: CloudSnow, label: 'Snow' },
  thunder: { icon: CloudLightning, label: 'Thunderstorm' },
}

/** Maps a WMO weather code to its broad condition category. */
export function conditionKind(code: number): ConditionKind {
  switch (code) {
    case 0:
    case 1:
      return 'clear'
    case 2:
      return 'cloudy'
    case 3:
      return 'overcast'
    case 45:
    case 48:
      return 'fog'
    case 51:
    case 53:
    case 54:
    case 55:
    case 56:
    case 57:
      return 'drizzle'
    case 61:
    case 63:
    case 65:
    case 66:
    case 67:
    case 80:
    case 81:
    case 82:
      return 'rain'
    case 71:
    case 73:
    case 75:
    case 77:
    case 85:
    case 86:
      return 'snow'
    case 95:
    case 96:
    case 99:
      return 'thunder'
    default:
      // An unmapped code should still render something sensible rather than
      // falling through to a blank card.
      return 'overcast'
  }
}

/** Returns the icon and short label for a WMO weather code. */
export function conditionStyle(code: number): ConditionStyle {
  return STYLES[conditionKind(code)]
}

/**
 * Formats a timestamp as a short relative age, e.g. "just now", "12 min ago".
 *
 * This is what the user cares about — whether the reading is current. It
 * deliberately does not expose whether the response came from the server's
 * cache, which is an implementation detail with no meaning to the person
 * reading it.
 */
export function formatRelativeTime(isoString: string, now: Date = new Date()): string {
  const timestamp = new Date(isoString)
  if (Number.isNaN(timestamp.getTime())) return 'just now'

  const seconds = Math.round((now.getTime() - timestamp.getTime()) / 1000)

  if (seconds < 60) return 'just now'

  const minutes = Math.round(seconds / 60)
  if (minutes < 60) return `${minutes} min ago`

  const hours = Math.round(minutes / 60)
  if (hours < 24) return `${hours} ${hours === 1 ? 'hour' : 'hours'} ago`

  const days = Math.round(hours / 24)
  return `${days} ${days === 1 ? 'day' : 'days'} ago`
}

/** Formats a timestamp as local wall-clock time, e.g. "14:05". */
export function formatClockTime(isoString: string): string {
  const date = new Date(isoString)
  if (Number.isNaN(date.getTime())) return '--'

  return new Intl.DateTimeFormat(undefined, {
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

/** Formats a timestamp as a compact hour label for the chart axis. */
export function formatHourLabel(isoString: string): string {
  return formatClockTime(isoString)
}

/** Rounds a temperature for display. */
export function formatTemperature(value: number): string {
  return String(Math.round(value))
}
