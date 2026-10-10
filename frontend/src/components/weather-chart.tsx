import {
  Area,
  AreaChart,
  CartesianGrid,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'

import type { Report } from '@/lib/api/schemas'
import { formatHourLabel, formatTemperature } from '@/lib/weather-conditions'

interface ChartPoint {
  time: string
  temperature: number
  index: number
}

interface TooltipProps {
  active?: boolean
  payload?: ReadonlyArray<{ payload?: ChartPoint }>
}

/**
 * Tooltip rendered by hand so the reading is legible, rather than the default
 * which prints the raw data point.
 */
function TemperatureTooltip({ active, payload }: TooltipProps) {
  if (!active || !payload || payload.length === 0) return null

  const point = payload[0]?.payload
  if (point === undefined) return null

  return (
    <div className="rounded-lg border border-border bg-card px-2.5 py-1.5 shadow-lg">
      <p className="text-xs text-muted-foreground">{point.time}</p>
      <p className="tabular text-lg font-medium leading-tight">
        {formatTemperature(point.temperature)}
        <span className="text-muted-foreground">&deg;C</span>
      </p>
    </div>
  )
}

interface WeatherChartProps {
  report: Report
}

/**
 * The 24-hour temperature trend for one city.
 *
 * The fill is a gradient from the line down to the axis, which gives the shape
 * of the day weight without needing a heavy visual. Min and max are labelled
 * directly because those are the two numbers a reader actually wants from a
 * daily trend.
 */
export default function WeatherChart({ report }: WeatherChartProps) {
  const data: ChartPoint[] = report.hourly.map((point, index) => ({
    time: formatHourLabel(point.time),
    temperature: point.temperature_c,
    index,
  }))

  if (data.length === 0) return null

  const temperatures = data.map((d) => d.temperature)
  const minimum = Math.min(...temperatures)
  const maximum = Math.max(...temperatures)
  const average = temperatures.reduce((sum, t) => sum + t, 0) / temperatures.length

  // Padding the domain stops the line from touching the frame at the extremes.
  const pad = Math.max(1, Math.round((maximum - minimum) * 0.25))

  return (
    <section
      aria-label={`24 hour temperature trend for ${report.city}`}
      className="rounded-2xl border border-border bg-card p-5"
    >
      <header className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
        <h2 className="text-lg font-medium">Next 24 hours</h2>
        <p className="text-sm text-muted-foreground">{report.city}</p>

        <dl className="tabular ml-auto flex gap-4 text-sm">
          <div className="flex gap-1.5">
            <dt className="text-muted-foreground">Low</dt>
            <dd className="font-medium">{formatTemperature(minimum)}&deg;</dd>
          </div>
          <div className="flex gap-1.5">
            <dt className="text-muted-foreground">Avg</dt>
            <dd className="font-medium">{formatTemperature(average)}&deg;</dd>
          </div>
          <div className="flex gap-1.5">
            <dt className="text-muted-foreground">High</dt>
            <dd className="font-medium">{formatTemperature(maximum)}&deg;</dd>
          </div>
        </dl>
      </header>

      <div className="mt-4 h-64 w-full">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart
            data={data}
            margin={{ top: 8, right: 8, bottom: 0, left: -24 }}
          >
            <defs>
              {/* Vertical fade so the area reads as weight under the line
                  rather than a flat block of colour. */}
              <linearGradient id="temperature-fill" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="var(--primary)" stopOpacity={0.28} />
                <stop offset="100%" stopColor="var(--primary)" stopOpacity={0.02} />
              </linearGradient>
            </defs>

            <CartesianGrid
              stroke="var(--border)"
              strokeDasharray="2 4"
              vertical={false}
            />
            <XAxis
              dataKey="time"
              tick={{ fontSize: 11, fill: 'var(--muted-foreground)' }}
              tickLine={false}
              axisLine={{ stroke: 'var(--border)' }}
              interval="preserveStartEnd"
              // Wide enough that the axis shows roughly every third hour on
              // desktop and fewer on mobile, instead of a wall of labels.
              minTickGap={72}
            />
            <YAxis
              tick={{ fontSize: 11, fill: 'var(--muted-foreground)' }}
              tickLine={false}
              axisLine={false}
              width={48}
              domain={[minimum - pad, maximum + pad]}
              tickFormatter={(value: number) => `${Math.round(value)}°`}
            />
            <Tooltip
              content={<TemperatureTooltip />}
              cursor={{ stroke: 'var(--border)', strokeWidth: 1 }}
            />
            {/* Zero-degree reference makes the shape readable at a glance. */}
            <ReferenceLine
              y={0}
              stroke="var(--muted-foreground)"
              strokeDasharray="3 3"
              opacity={0.5}
            />
            <Area
              type="monotone"
              dataKey="temperature"
              stroke="var(--primary)"
              strokeWidth={2}
              fill="url(#temperature-fill)"
              dot={false}
              activeDot={{ r: 4, strokeWidth: 2, stroke: 'var(--card)' }}
              isAnimationActive={false}
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </section>
  )
}
