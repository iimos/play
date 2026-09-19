import { registerIndicator, TooltipLegend } from 'klinecharts'
import { MetricPoint, mapRealBars } from '../data/index'

function formatValue (v: number): string {
  if (!Number.isFinite(v)) return String(v)
  const a = Math.abs(v)
  if (a !== 0 && (a >= 1e6 || a < 1e-3)) return v.toExponential(3)
  return v.toFixed(4)
}

export interface MetricIndicatorSpec {
  id: number
  expression: string
  key: string
  indicatorName: string
  paneId: string
}

let metricSeq = 0

// Значения метрик по ключу -> timestamp -> value. Модульный стейт рассчитан на
// единственный экземпляр чарта (как fizOIMap в ChartType.tsx). Индикатор читает
// отсюда по timestamp свечи, а заполняется через set/appendMetricValues из
// ChartType при загрузке данных.
const metricValues = new Map<string, Map<number, number>>()

export function setMetricValues (key: string, points: MetricPoint[]) {
  metricValues.set(key, new Map(points.map(p => [p.timestamp, p.value])))
}

export function appendMetricValues (key: string, points: MetricPoint[]) {
  const m = metricValues.get(key) ?? new Map<number, number>()
  for (const p of points) {
    m.set(p.timestamp, p.value)
  }
  metricValues.set(key, m)
}

export function clearMetricValues () {
  metricValues.clear()
}

export function removeMetricValues (key: string) {
  metricValues.delete(key)
}

// Регистрирует индикатор, который рисует значение SQL-метрики по её ключу.
// Само выражение вычисляется в ClickHouse (см. fetchMetric в src/data/index.ts),
// здесь только отрисовка по уже загруженным точкам.
export function makeMetricIndicator (expression: string): MetricIndicatorSpec {
  const id = ++metricSeq
  const key = `metric_${id}`
  const indicatorName = `sql_metric_${id}`
  const paneId = `pane_metric_${id}`

  registerIndicator<{ value: number | null } | null>({
    name: indicatorName,
    shortName: expression,
    series: 'normal',
    precision: 4,
    figures: [
      { key: 'value', title: `${expression}: `, type: 'line' },
    ],
    calc: dataList => {
      const values = metricValues.get(key)
      return mapRealBars<{ value: number | null }>(dataList, c => ({ value: values?.get(c.timestamp) ?? null }))
    },
    createTooltipDataSource: ({ indicator, crosshair }) => {
      const legends: TooltipLegend[] = []
      const data = crosshair.dataIndex != null
        ? indicator.result[crosshair.dataIndex]
        : undefined
      if (data?.value != null) {
        legends.push({ title: expression + ': ', value: formatValue(data.value) })
      }
      return { name: expression, calcParamsText: '', features: [], legends }
    },
  })

  return { id, expression, key, indicatorName, paneId }
}
