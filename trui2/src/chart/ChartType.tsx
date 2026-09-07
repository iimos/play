import React, { useEffect, useState, useRef } from 'react'
import { Chart, init, dispose, registerIndicator, Period, TooltipLegend } from 'klinecharts'
import Layout from '../Layout'
import TickerSelector from './TickerSelector'
import { fetchCandles, fetchLatestTime, IntervalType, SuperCandle, fetchFizOI, resolveFutoiTicker, FutOIData, FizOIResult } from '../data/index'

const klineStyle = {}

// Идентификатор панели, на которой живёт переключаемый индикатор объёма.
const VOLUME_PANE_ID = 'volume_pane'

// OI физлиц (FUTOI) по инструменту, заполняется перед созданием индикатора fiz_oi
let fizOIMap = new Map<number, FizOIPoint>()
// Дневные срезы (iss_openpositions) для forward-fill на внутридневных интервалах.
let fizOIDaily: FizOIDailyPoint[] = []
let fizOIDailyMode = false

interface FizOIPoint {
  fiz_long: number | null
  fiz_short: number | null
  ruble_long: number | null
  ruble_short: number | null
  share: number | null
}

interface FizOIDailyPoint {
  timestamp: number
  point: FizOIPoint
}

function fmtRubles(n: number): string {
  const sign = n < 0 ? '-' : ''
  const a = Math.abs(n)
  if (a >= 1e9) return sign + (a / 1e9).toFixed(1) + ' млрд ₽'
  if (a >= 1e6) return sign + (a / 1e6).toFixed(1) + ' млн ₽'
  if (a >= 1e3) return sign + (a / 1e3).toFixed(1) + ' тыс ₽'
  return sign + Math.round(a) + ' ₽'
}

function toFizOIPoint(d: FutOIData): FizOIPoint {
  const total = d.total_long + d.total_short
  const share = total > 0 ? Math.min(100, (d.fiz_long + d.fiz_short) / total * 100) : null
  const hasPrice = d.price > 0
  return {
    fiz_long: d.fiz_long,
    fiz_short: d.fiz_short,
    ruble_long: hasPrice ? d.fiz_long * d.price : null,
    ruble_short: hasPrice ? d.fiz_short * d.price : null,
    share,
  }
}

function emptyFizOIPoint(): FizOIPoint {
  return { fiz_long: null, fiz_short: null, ruble_long: null, ruble_short: null, share: null }
}

// Мержит результат fetchFizOI в модульное состояние. replace=true — новый
// инструмент/интервал (сброс), иначе — докрутка вперёд (добавление точек).
function applyFizOIResult(result: FizOIResult, replace: boolean) {
  if (replace) {
    fizOIMap = new Map()
    fizOIDaily = []
  }
  fizOIDailyMode = result.isDaily
  if (result.isDaily) {
    // Докрутка вперёд перезапрашивает перекрывающийся диапазон — дедуплицируем
    // по timestamp, чтобы массив не разрастался дублями.
    const seen = new Set(fizOIDaily.map(p => p.timestamp))
    for (const d of result.data) {
      if (seen.has(d.timestamp)) continue
      seen.add(d.timestamp)
      fizOIDaily.push({ timestamp: d.timestamp, point: toFizOIPoint(d) })
    }
    fizOIDaily.sort((a, b) => a.timestamp - b.timestamp)
  } else {
    for (const d of result.data) {
      fizOIMap.set(d.timestamp, toFizOIPoint(d))
    }
  }
}

// Для дневных срезов тянем последнее известное значение на весь день
// (сплошная горизонтальная линия), т.к. внутри дня данных нет.
function forwardFillDailyPoint(ts: number): FizOIPoint {
  let lo = 0, hi = fizOIDaily.length - 1, ans = -1
  while (lo <= hi) {
    const mid = (lo + hi) >> 1
    if (fizOIDaily[mid].timestamp <= ts) {
      ans = mid
      lo = mid + 1
    } else {
      hi = mid - 1
    }
  }
  return ans >= 0 ? fizOIDaily[ans].point : emptyFizOIPoint()
}

type ActiveTradesPoint = SuperCandle & { val_net?: number }

function fmtLots(n: number | undefined): string {
  return (n ?? 0).toLocaleString('ru-RU')
}

registerIndicator<ActiveTradesPoint>({
  name: 'volume_bs',
  shortName: 'Активные сделки',
  series: 'volume',
  precision: 0,
  shouldFormatBigNumber: true,
  figures: [
    {
      key: 'val_net',
      title: 'нетто (₽): ',
      type: 'bar',
      baseValue: 0,
      styles: (params) => {
        const net = params.data.current?.val_net ?? 0
        return { color: net >= 0 ? 'green' : 'red' }
      },
    }
  ],
  calc: dataList => dataList.map(c => {
    const k = c as unknown as SuperCandle
    return {
      ...k,
      val_net: (k.val_b ?? 0) - (k.val_s ?? 0),
    } as ActiveTradesPoint
  }),
  createTooltipDataSource: ({ indicator, crosshair }) => {
    const legends: TooltipLegend[] = []
    const data = crosshair.dataIndex != null
      ? indicator.result[crosshair.dataIndex]
      : undefined
    if (data?.val_net != null) {
      legends.push({ title: 'нетто: ', value: fmtRubles(data.val_net) })
    }
    legends.push({ title: 'vol_b (лоты): ', value: fmtLots(data?.volume_b) })
    legends.push({ title: 'vol_s (лоты): ', value: fmtLots(data?.volume_s) })
    if (data?.val_b != null) {
      legends.push({ title: 'val_b: ', value: fmtRubles(data.val_b) })
    }
    if (data?.val_s != null) {
      legends.push({ title: 'val_s: ', value: fmtRubles(data.val_s) })
    }
    return { name: 'Активные сделки', calcParamsText: '', features: [], legends }
  },
})

// Открытый интерес физлиц в рублях (данные FUTOI из tr.futoi)
registerIndicator<FizOIPoint>({
  name: 'fiz_oi',
  shortName: 'Открытый интерес физлиц',
  series: 'volume',
  precision: 0,
  shouldFormatBigNumber: true,
  figures: [
    { key: 'ruble_long', title: 'ОИ физлиц (лонг, ₽): ', type: 'line', styles: () => ({ color: "#26A69A" })},
    { key: 'ruble_short', title: 'ОИ физлиц (шорт, ₽): ', type: 'line', styles: () => ({ color: "#EF5350" })},
  ],
  calc: dataList => dataList.map(c => {
    if (fizOIDailyMode) {
      return forwardFillDailyPoint(c.timestamp)
    }
    return fizOIMap.get(c.timestamp) ?? emptyFizOIPoint()
  }),
  createTooltipDataSource: ({ indicator, crosshair }) => {
    const legends: TooltipLegend[] = []
    const data = crosshair.dataIndex != null
      ? indicator.result[crosshair.dataIndex]
      : undefined
    if (data?.ruble_long != null) {
      legends.push({ title: 'Лонг=', value: fmtRubles(data.ruble_long) })
    }
    if (data?.ruble_short != null) {
      legends.push({ title: 'Шорт=', value: fmtRubles(data.ruble_short) })
    }
    if (data?.share != null) {
      legends.push({ title: 'Доля физлиц=', value: `${data.share.toFixed(1)}%` })
    }
    return { name: 'ОИ физлиц', calcParamsText: '', features: [], legends }
  },
})

function intervalToPeriod(interval: string): Period {
  switch (interval) {
    case IntervalType.Minute:
      return { span: 1, type: 'minute' }
    case IntervalType.FiveMinutes:
      return { span: 5, type: 'minute' }
    case IntervalType.Hour:
      return { span: 1, type: 'hour' }
    case IntervalType.Day:
      return { span: 1, type: 'day' }
    case IntervalType.Week:
      return { span: 1, type: 'week' }
    case IntervalType.Month:
      return { span: 1, type: 'month' }
    default:
      throw new Error("Unsupported interval type")
  }
}

export default function ChartType () {
  const [ticker, setTicker] = useState("ROSN")
  const [interval, setInterval] = useState(IntervalType.Hour)
  const [activeVolumeIndicator, setActiveVolumeIndicator] = useState<'VOL' | 'volume_bs' | 'oi'>('volume_bs')
  const [oiAvailable, setOIAvailable] = useState(false)
  const [oiIsDaily, setOIIsDaily] = useState(false)
  const chart = useRef<Chart | null>(null)
  const futoiTickerRef = useRef<string | null>(null)
  const futoiAssetRef = useRef<string | null>(null)

  const switchIndicator = (name: 'VOL' | 'volume_bs' | 'oi') => {
    const c = chart.current
    if (!c) return
    if (name === 'oi' && !oiAvailable) return

    // Убираем предыдущий индикатор с панели объёма и создаём новый на её месте.
    c.removeIndicator({ paneId: VOLUME_PANE_ID })
    if (name === 'oi') {
      c.createIndicator({ name: 'fiz_oi', paneId: VOLUME_PANE_ID })
    } else {
      c.createIndicator({ name, paneId: VOLUME_PANE_ID })
    }
    setActiveVolumeIndicator(name)
  }

  // Основной эффект: инициализация чарта и загрузка данных
  useEffect(() => {
    let cancelled = false

    setActiveVolumeIndicator('volume_bs')
    setOIAvailable(false)
    setOIIsDaily(false)
    fizOIMap = new Map()
    fizOIDaily = []
    fizOIDailyMode = false
    futoiTickerRef.current = null
    futoiAssetRef.current = null

    chart.current = init("real-time-k-line", { styles: klineStyle })
    chart.current?.createIndicator({ name: 'volume_bs', paneId: VOLUME_PANE_ID })

    // Загружаем OI физлиц для диапазона и резолвим тикер FUTOI (для init-загрузки).
    async function loadOI(till: Date, candles: SuperCandle[]) {
      const futures = candles.some(c => c.oi_close !== undefined)
      const resolution = await resolveFutoiTicker(ticker, futures)
      if (cancelled) return
      futoiTickerRef.current = resolution.futoiTicker
      futoiAssetRef.current = resolution.asset
      if (resolution.futoiTicker) {
        const oiResult = await fetchFizOI(resolution.futoiTicker, resolution.asset, till, interval)
        if (cancelled) return
        const available = oiResult.data.length > 0
        setOIAvailable(available)
        setOIIsDaily(available && oiResult.isDaily)
        if (available) {
          applyFizOIResult(oiResult, true)
        }
      } else {
        setOIAvailable(false)
      }
    }

    // Докрутка OI вперёд при подгрузке истории.
    async function loadOIForward(till: Date) {
      if (!futoiTickerRef.current) return
      try {
        const oiResult = await fetchFizOI(futoiTickerRef.current, futoiAssetRef.current, till, interval)
        if (!cancelled) {
          applyFizOIResult(oiResult, false)
        }
      } catch (error) {
        console.error('Ошибка загрузки OI:', error)
      }
    }

    chart.current?.setSymbol({ ticker, pricePrecision: 2, volumePrecision: 0 })
    chart.current?.setPeriod(intervalToPeriod(interval))
    chart.current?.setDataLoader({
      getBars: ({ type, timestamp, callback }) => {
        if (type === 'init') {
          fetchLatestTime()
            .then(async latestTime => {
              const candles = await fetchCandles(ticker, latestTime, interval)
              if (cancelled) return
              try {
                await loadOI(latestTime, candles)
              } catch (error) {
                console.error('Ошибка загрузки данных:', error)
              }
              callback(candles, { forward: true, backward: false })
            })
            .catch(error => {
              console.error('Ошибка загрузки данных:', error)
              if (!cancelled) callback([], { forward: false, backward: false })
            })
        } else if (type === 'forward' && timestamp != null) {
          fetchCandles(ticker, new Date(timestamp), interval)
            .then(async candles => {
              if (cancelled) return
              if (candles.length > 0) {
                await loadOIForward(new Date(timestamp))
              }
              if (cancelled) return
              callback(candles, { forward: candles.length !== 0, backward: false })
            })
            .catch(error => {
              console.error('Ошибка загрузки данных:', error)
              if (!cancelled) callback([], { forward: false, backward: false })
            })
        } else {
          callback([], { forward: false, backward: false })
        }
      },
    })

    return () => {
      cancelled = true
      if (chart.current) {
        dispose(chart.current)
        chart.current = null
      }
    }
  }, [ticker, interval])

  return (
    <Layout title={`${ticker} ${interval}`}>
      <TickerSelector onSelect={setTicker} />
      <div id="real-time-k-line" className="k-line-chart" />
      <div className="k-line-chart-menu-container">
        <button onClick={_ => setInterval(IntervalType.FiveMinutes)}>5m</button>
        <button onClick={_ => setInterval(IntervalType.Hour)}>hour</button>
        <button onClick={_ => setInterval(IntervalType.Day)}>day</button>
        <button onClick={_ => setInterval(IntervalType.Week)}>week</button>
        <button onClick={_ => setInterval(IntervalType.Month)}>month</button>

        <span style={{ paddingLeft: 12, paddingRight: 6 }}>Объем:</span>
        <button
          onClick={_ => switchIndicator('VOL')}
          style={{ backgroundColor: activeVolumeIndicator === 'VOL' ? '#4CAF50' : '' }}
        >
          Объем
        </button>
        <button
          onClick={_ => switchIndicator('volume_bs')}
          style={{ backgroundColor: activeVolumeIndicator === 'volume_bs' ? '#4CAF50' : '' }}
        >
          Активные сделки
        </button>
        <button
          onClick={_ => switchIndicator('oi')}
          disabled={!oiAvailable}
          style={{
            backgroundColor: activeVolumeIndicator === 'oi' ? '#4CAF50' : '',
            opacity: oiAvailable ? 1 : 0.5,
            cursor: oiAvailable ? 'pointer' : 'not-allowed'
          }}
          title={
            !oiAvailable
              ? "Нет данных по открытому интересу"
              : oiIsDaily
                ? "Открытый интерес физлиц (данные суточные)"
                : "Открытый интерес физлиц (FUTOI)"
          }
        >
          Открытый интерес
        </button>
        {oiAvailable && oiIsDaily && (
          <span style={{ paddingLeft: 6, color: '#FFB74D', fontSize: 12 }}>
            данные суточные
          </span>
        )}
      </div>
    </Layout>
  )
}
