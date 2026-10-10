import { z } from 'zod'

/**
 * Runtime schemas for the Weather Watcher API.
 *
 * Types are derived from these schemas rather than declared separately, so the
 * static type and the runtime contract cannot drift apart. Validating the
 * response means a malformed or unexpected payload surfaces as one clear error
 * here, instead of an undefined property blowing up somewhere inside a
 * component several layers down.
 */

export const hourlyPointSchema = z.object({
  time: z.string(),
  temperature_c: z.number(),
})

export const currentConditionsSchema = z.object({
  temperature_c: z.number(),
  feels_like_c: z.number(),
  humidity_pct: z.number(),
  wind_speed_kph: z.number(),
  code: z.number(),
  description: z.string(),
  observed_at: z.string(),
})

export const reportSchema = z.object({
  city: z.string(),
  // Omitted by the backend when the geocoder gives no country.
  country: z.string().optional(),
  latitude: z.number(),
  longitude: z.number(),
  current: currentConditionsSchema,
  hourly: z.array(hourlyPointSchema),
  fetched_at: z.string(),
  from_cache: z.boolean(),
})

export const cityErrorSchema = z.object({
  city: z.string(),
  message: z.string(),
})

export const weatherResponseSchema = z.object({
  results: z.array(reportSchema),
  errors: z.array(cityErrorSchema),
  requested_at: z.string(),
  cached_count: z.number(),
  total: z.number(),
})

export const healthResponseSchema = z.object({
  status: z.string(),
  uptime_seconds: z.number(),
  cached_cities: z.number(),
  time: z.string(),
})

export type HourlyPoint = z.infer<typeof hourlyPointSchema>
export type CurrentConditions = z.infer<typeof currentConditionsSchema>
export type Report = z.infer<typeof reportSchema>
export type CityError = z.infer<typeof cityErrorSchema>
export type WeatherResponse = z.infer<typeof weatherResponseSchema>
export type HealthResponse = z.infer<typeof healthResponseSchema>
