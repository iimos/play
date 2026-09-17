import React, { useEffect, useLayoutEffect, useState, useRef } from 'react'
import { Chart, init, dispose, registerIndicator, registerOverlay, Period, TooltipLegend, Point } from 'klinecharts'
import Layout from '../Layout'
import TickerSelector from './TickerSelector'
import { fetchCandles, fetchLatestTime, fetchMetric, fetchMarketShares, IntervalType, SuperCandle, fetchFizOI, resolveFutoiTicker, resolveSecGroup, SEC_GROUP_TO_TABLE, FutOIData, FizOIResult, MetricPoint } from '../data/index'
import { makeMetricIndicator, MetricIndicatorSpec, setMetricValues, appendMetricValues, clearMetricValues, removeMetricValues } from './metricIndicator'

const klineStyle = {}

type VolumeIndicatorId = 'VOL' | 'volume_bs' | 'volume_bs_cum' | 'oi' | 'fiz_imbalance' | 'pr_vwap_spread'

interface VolumeIndicatorDef {
  id: VolumeIndicatorId
  label: string
  indicatorName: string
  paneId: string
  requiresOI: boolean
  // Интервалы, на которых индикатор скрыт (недоступен).
  hiddenOn?: string[]
}

// Все индикаторы объёма показываются друг под другом на отдельных панелях,
// каждый можно независимо включать/выключать чекбоксом.
const VOLUME_INDICATORS: VolumeIndicatorDef[] = [
  { id: 'VOL', label: 'Объем', indicatorName: 'VOL', paneId: 'pane_vol', requiresOI: false },
  { id: 'volume_bs', label: 'Активные сделки', indicatorName: 'volume_bs', paneId: 'pane_volume_bs', requiresOI: false },
  { id: 'volume_bs_cum', label: 'Накопл. объём активных сделок', indicatorName: 'volume_bs_cum', paneId: 'pane_volume_bs_cum', requiresOI: false, hiddenOn: [IntervalType.Week, IntervalType.Month] },
  { id: 'oi', label: 'Открытый интерес', indicatorName: 'fiz_oi', paneId: 'pane_oi', requiresOI: true },
  { id: 'fiz_imbalance', label: 'Дисбаланс ОИ', indicatorName: 'fiz_imbalance', paneId: 'pane_fiz_imbalance', requiresOI: true },
  { id: 'pr_vwap_spread', label: 'Спред VWAP', indicatorName: 'pr_vwap_spread', paneId: 'pane_pr_vwap_spread', requiresOI: false },
]

const DEFAULT_ENABLED: Record<VolumeIndicatorId, boolean> = {
  VOL: true,
  volume_bs: true,
  volume_bs_cum: true,
  oi: true,
  fiz_imbalance: true,
  pr_vwap_spread: true,
}

// OI физлиц (FUTOI) по инструменту, заполняется перед созданием индикатора fiz_oi
let fizOIMap = new Map<number, FizOIPoint>()
// Дневные срезы (iss_openpositions) для forward-fill на внутридневных интервалах.
let fizOIDaily: FizOIDailyPoint[] = []
let fizOIDailyMode = false

// Текущий интервал графика — нужен calc-функции volume_bs_cum, чтобы выбрать
// период накопления (торговый день МСК на внутридневных, неделя на дневках).
// Модульная переменная рассчитана на единственный экземпляр чарта (как и
// fizOIMap/fizOIDailyMode выше).
let currentInterval = IntervalType.Hour

// Дата в таймзоне МСК (Europe/Moscow, UTC+3 без DST) в формате YYYY-MM-DD.
// Собираем через formatToParts, чтобы не зависеть от локали/движка ('en-CA'
// выдаёт YYYY-MM-DD только де-факто, а не по спецификации).
const mskDayParts = new Intl.DateTimeFormat('en-US', {
  timeZone: 'Europe/Moscow',
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
})

function mskDateKey(ts: number): string {
  const parts = mskDayParts.formatToParts(new Date(ts))
  const get = (type: string) => parts.find(p => p.type === type)?.value ?? ''
  return `${get('year')}-${get('month')}-${get('day')}`
}

// Время суток в МСК (HH:MM) — ключ «того же времени суток» для сравнения объёма.
const mskTimeParts = new Intl.DateTimeFormat('en-US', {
  timeZone: 'Europe/Moscow',
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
})

function mskTimeKey(ts: number): string {
  const parts = mskTimeParts.formatToParts(new Date(ts))
  const get = (type: string) => parts.find(p => p.type === type)?.value ?? ''
  // Некоторые движки отдают '24' вместо '00' при hour12:false.
  const hour = get('hour') === '24' ? '00' : get('hour')
  return `${hour}:${get('minute')}`
}

// Понедельник недели (в МСК) для заданного timestamp, в формате YYYY-MM-DD.
function mskMonday(ts: number): string {
  const dateKey = mskDateKey(ts)
  const d = new Date(dateKey + 'T00:00:00Z')
  const diff = (d.getUTCDay() + 6) % 7 // 0 = понедельник
  d.setUTCDate(d.getUTCDate() - diff)
  return d.toISOString().slice(0, 10)
}

// Ключ периода накопления для volume_bs_cum. На внутридневных интервалах это
// календарный день МСК (вечерняя сессия 19:00–23:50 остаётся в своей дате),
// на дневках — неделя (понедельник).
function accumulationPeriodKey(ts: number): string {
  if (currentInterval === IntervalType.Day) {
    return mskMonday(ts)
  }
  return mskDateKey(ts)
}

interface FizOIPoint {
  fiz_long: number | null
  fiz_short: number | null
  ruble_long: number | null
  ruble_short: number | null
  share: number | null
  oi_imbalance: number | null
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
  const sum = d.fiz_long + d.fiz_short
  const oi_imbalance = sum > 0 ? (d.fiz_long - d.fiz_short) / sum : null
  const hasPrice = d.price > 0
  return {
    fiz_long: d.fiz_long,
    fiz_short: d.fiz_short,
    ruble_long: hasPrice ? d.fiz_long * d.price : null,
    ruble_short: hasPrice ? d.fiz_short * d.price : null,
    share,
    oi_imbalance,
  }
}

function emptyFizOIPoint(): FizOIPoint {
  return { fiz_long: null, fiz_short: null, ruble_long: null, ruble_short: null, share: null, oi_imbalance: null }
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

// Кумулятивная сумма разностей (vol_b - vol_s) с начала периода:
// на внутридневных интервалах — с начала торгового дня (МСК), на дневках —
// с начала недели. Рисуется красными/зелёными столбиками.
type VolBsCumPoint = SuperCandle & { net?: number }

registerIndicator<VolBsCumPoint>({
  name: 'volume_bs_cum',
  shortName: 'Накопл. объём активных сделок',
  series: 'volume',
  precision: 0,
  shouldFormatBigNumber: true,
  figures: [
    {
      key: 'net',
      title: 'vol_b - vol_s: ',
      type: 'bar',
      baseValue: 0,
      styles: (params) => {
        const v = params.data.current?.net ?? 0
        return { color: v >= 0 ? 'green' : 'red' }
      },
    },
  ],
  // ВАЖНО: klinecharts вызывает calc на полном отсортированном списке данных при
  // каждом изменении (init/forward). Кумулятивная сумма пересчитывается с нуля,
  // поэтому корректность зависит от идемпотентного полного прохода по списку.
  calc: dataList => {
    let cum = 0
    let prevKey = ''
    return dataList.map(c => {
      const k = c as unknown as SuperCandle
      const key = accumulationPeriodKey(k.timestamp)
      if (key !== prevKey) {
        cum = 0
        prevKey = key
      }
      // Нет данных по покупкам/продажам (напр. интервал 1m или тикер из
      // tr.candles) — не рисуем фантомный нулевой бар.
      if (k.volume_b == null && k.volume_s == null) {
        return { ...k } as VolBsCumPoint
      }
      cum += (k.volume_b ?? 0) - (k.volume_s ?? 0)
      return { ...k, net: cum } as VolBsCumPoint
    })
  },
  createTooltipDataSource: ({ indicator, crosshair }) => {
    const legends: TooltipLegend[] = []
    const data = crosshair.dataIndex != null
      ? indicator.result[crosshair.dataIndex]
      : undefined
    if (data?.net != null) {
      legends.push({ title: 'vol_b - vol_s: ', value: fmtLots(data.net) })
    }
    return { name: 'Накопл. объём активных сделок', calcParamsText: '', features: [], legends }
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

// Дисбаланс позиций физлиц: (fiz_long - fiz_short) / (fiz_long + fiz_short)
registerIndicator<FizOIPoint>({
  name: 'fiz_imbalance',
  shortName: 'Дисбаланс физлиц',
  series: 'volume',
  precision: 2,
  minValue: -1,
  maxValue: 1,
  figures: [
    {
      key: 'oi_imbalance',
      title: 'imbalance: ',
      type: 'bar',
      baseValue: 0,
      styles: (params) => {
        const v = params.data.current?.oi_imbalance ?? 0
        return { color: v >= 0 ? 'green' : 'red' }
      },
    }
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
    if (data?.oi_imbalance != null) {
      legends.push({ title: 'imbalance: ', value: data.oi_imbalance.toFixed(3) })
    }
    if (data?.ruble_long != null) {
      legends.push({ title: 'Лонг=', value: fmtRubles(data.ruble_long) })
    }
    if (data?.ruble_short != null) {
      legends.push({ title: 'Шорт=', value: fmtRubles(data.ruble_short) })
    }
    if (data?.share != null) {
      legends.push({ title: 'Доля физлиц=', value: `${data.share.toFixed(1)}%` })
    }
    return { name: 'Дисбаланс физлиц', calcParamsText: '', features: [], legends }
  },
})

// Спред между средней ценой покупателей и продавцов: pr_vwap_b - pr_vwap_s
type SpreadPoint = SuperCandle & { spread?: number }

registerIndicator<SpreadPoint>({
  name: 'pr_vwap_spread',
  shortName: 'Спред VWAP',
  series: 'normal',
  precision: 2,
  figures: [
    {
      key: 'spread',
      title: 'спред (₽): ',
      type: 'bar',
      baseValue: 0,
      styles: (params) => {
        const v = params.data.current?.spread ?? 0
        return { color: v >= 0 ? 'green' : 'red' }
      },
    },
  ],
  calc: dataList => dataList.map(c => {
    const k = c as unknown as SuperCandle
    const b = k.pr_vwap_b
    const s = k.pr_vwap_s
    return {
      ...k,
      spread: (b != null && s != null) ? b - s : undefined,
    } as SpreadPoint
  }),
  createTooltipDataSource: ({ indicator, crosshair }) => {
    const legends: TooltipLegend[] = []
    const data = crosshair.dataIndex != null
      ? indicator.result[crosshair.dataIndex]
      : undefined
    if (data?.spread != null) {
      legends.push({ title: 'спред: ', value: `${data.spread.toFixed(2)} ₽` })
    }
    if (data?.pr_vwap_b != null) {
      legends.push({ title: 'VWAP покуп.: ', value: data.pr_vwap_b.toFixed(2) })
    }
    if (data?.pr_vwap_s != null) {
      legends.push({ title: 'VWAP продаж: ', value: data.pr_vwap_s.toFixed(2) })
    }
    return { name: 'Спред VWAP', calcParamsText: '', features: [], legends }
  },
})

// Вертикальная пунктирная линия-разделитель на всю высоту панели свечей.
// Используется для визуального отделения торговых дней (внутридневные интервалы)
// и недель (дневной интервал).
registerOverlay({
  name: 'periodSeparator',
  totalStep: 1,
  needDefaultPointFigure: false,
  needDefaultXAxisFigure: false,
  needDefaultYAxisFigure: false,
  createPointFigures: ({ chart, coordinates, bounding }) => {
    if (coordinates.length === 0) {
      return []
    }
    // dataIndexToCoordinate возвращает центр бара, а разделитель должен лежать
    // в середине зазора между свечами — сдвигаем на половину шага влево.
    const x = coordinates[0].x - chart.getBarSpace().halfBar
    return [{
      type: 'line',
      attrs: {
        coordinates: [
          { x, y: 0 },
          { x, y: bounding.height },
        ],
      },
      styles: { style: 'dashed', color: 'rgba(148, 148, 148, 0.5)', size: 1, dashedValue: [4, 4] },
      ignoreEvent: true,
    }]
  },
})

// Полупрозрачная заливка выходного дня (субботы/воскресенья). Две точки: первая
// и последняя свеча этого дня.
registerOverlay({
  name: 'weekendHighlight',
  totalStep: 2,
  needDefaultPointFigure: false,
  needDefaultXAxisFigure: false,
  needDefaultYAxisFigure: false,
  createPointFigures: ({ chart, coordinates, bounding }) => {
    if (coordinates.length < 2) {
      return []
    }
    // Заливаем от левого края первой свечи до правого края последней.
    const halfBar = chart.getBarSpace().halfBar
    const x0 = coordinates[0].x - halfBar
    const x1 = coordinates[1].x + halfBar
    return [{
      type: 'polygon',
      attrs: {
        coordinates: [
          { x: x0, y: 0 },
          { x: x1, y: 0 },
          { x: x1, y: bounding.height },
          { x: x0, y: bounding.height },
        ],
      },
      styles: { style: 'fill', color: 'rgba(255, 183, 77, 0.12)' },
      ignoreEvent: true,
    }]
  },
})

// Полупрозрачные рамки вокруг свечей с заметным объёмом. Один оверлей рисует
// сразу все подсвеченные свечи: точки идут парами (high, low) на каждую свечу.
registerOverlay({
  name: 'volumeHighlights',
  totalStep: 2,
  needDefaultPointFigure: false,
  needDefaultXAxisFigure: false,
  needDefaultYAxisFigure: false,
  createPointFigures: ({ chart, coordinates }) => {
    const halfBar = chart.getBarSpace().halfBar
    const figures = []
    for (let i = 0; i + 1 < coordinates.length; i += 2) {
      const x = coordinates[i].x
      const yTop = Math.min(coordinates[i].y, coordinates[i + 1].y)
      const yBottom = Math.max(coordinates[i].y, coordinates[i + 1].y)
      figures.push({
        type: 'polygon',
        attrs: {
          coordinates: [
            { x: x - halfBar, y: yTop },
            { x: x + halfBar, y: yTop },
            { x: x + halfBar, y: yBottom },
            { x: x - halfBar, y: yBottom },
          ],
        },
        styles: {
          style: 'stroke_fill',
          color: 'rgba(255, 179, 0, 0.15)',
          borderColor: 'rgba(255, 179, 0, 0.9)',
          borderSize: 1,
        },
        ignoreEvent: true,
      })
    }
    return figures
  },
})

// Доля рынка (в рублях) по умолчанию, при которой свеча считается заметной.
const DEFAULT_SHARE_THRESHOLD = 0.20
// Порог аномалии собственного объёма: объём должен быть строго выше
// mean + ANOMALY_SIGMA * std (см. base.cut).
const ANOMALY_SIGMA = 2
// Окно «среднеисторического» объёма: 60 дней.
const VOLUME_WINDOW_MS = 60 * 24 * 60 * 60 * 1000
// Минимум наблюдений в окне, чтобы считать аномалию осмысленной.
const MIN_BASELINE_SAMPLES = 5
// Страховка от лавины подсветок при слишком низком пороге доли рынка.
const MAX_HIGHLIGHTS = 300

// Длительность одного бара интервала в мс.
function intervalMs(interval: string): number {
  switch (interval) {
    case IntervalType.Minute: return 60 * 1000
    case IntervalType.FiveMinutes: return 5 * 60 * 1000
    case IntervalType.Hour: return 60 * 60 * 1000
    case IntervalType.Day: return 24 * 60 * 60 * 1000
    case IntervalType.Week: return 7 * 24 * 60 * 60 * 1000
    case IntervalType.Month: return 31 * 24 * 60 * 60 * 1000
    default: return 24 * 60 * 60 * 1000
  }
}

// Окно базы объёма. Не меньше 60 дней, но для недель/месяцев расширяется так,
// чтобы в него помещалось MIN_BASELINE_SAMPLES баров (иначе аномалия мертва).
function baselineWindowMs(): number {
  return Math.max(VOLUME_WINDOW_MS, MIN_BASELINE_SAMPLES * intervalMs(currentInterval))
}

// Доля инструмента в суммарном обороте всех акций по каждому таймслоту (0..1).
// Заполняется из БД вместе со свечами; пусто для не-акций и интервала 1m.
let marketShares = new Map<number, number>()

// Таймслоты, которые надо подсветить: доля рынка >= порога ИЛИ аномалия объёма.
let highlightTimes = new Set<number>()

// База для сравнения объёма на каждый таймслот: средний объём в то же время
// суток за последние 60 дней и порог аномалии mean + ANOMALY_SIGMA*std.
// Для дневок/недель/месяцев «время суток» не различается — одна группа.
let volumeBaseline = new Map<number, { mean: number; cut: number }>()

// Внутридневные интервалы — сравниваем в пределах одного времени суток.
function isIntraday(): boolean {
  return currentInterval !== IntervalType.Day
    && currentInterval !== IntervalType.Week
    && currentInterval !== IntervalType.Month
}

// Оборот свечи в рублях (val_b + val_s). Объём сравниваем в деньгах, а не в
// лотах: лоты между разными бумагами несопоставимы.
function candleTurnover(c: SuperCandle): number {
  return (c.val_b ?? 0) + (c.val_s ?? 0)
}

// Для каждой свечи считает среднее (и порог аномалии) по свечам того же времени
// суток за предшествующие 60 дней. Скользящее окно по каждой группе за O(N).
function computeVolumeBaselines(dataList: SuperCandle[]): Map<number, { mean: number; cut: number }> {
  const intraday = isIntraday()
  const windowMs = baselineWindowMs()
  const result = new Map<number, { mean: number; cut: number }>()
  const groups = new Map<string, { ts: number[]; val: number[]; start: number; sum: number; sumsq: number }>()
  for (const c of dataList) {
    const key = intraday ? mskTimeKey(c.timestamp) : ''
    let group = groups.get(key)
    if (group == null) {
      group = { ts: [], val: [], start: 0, sum: 0, sumsq: 0 }
      groups.set(key, group)
    }
    const turnover = candleTurnover(c)
    const cutoff = c.timestamp - windowMs
    while (group.start < group.ts.length && group.ts[group.start] < cutoff) {
      const old = group.val[group.start]
      group.sum -= old
      group.sumsq -= old * old
      group.start++
    }
    const count = group.ts.length - group.start
    if (count >= MIN_BASELINE_SAMPLES) {
      const mean = group.sum / count
      const variance = Math.max(0, group.sumsq / count - mean * mean)
      result.set(c.timestamp, { mean, cut: mean + ANOMALY_SIGMA * Math.sqrt(variance) })
    }
    group.ts.push(c.timestamp)
    group.val.push(turnover)
    group.sum += turnover
    group.sumsq += turnover * turnover
  }
  return result
}

// Текстовое объяснение, почему свеча подсвечена (для всплывашки при наведении).
// Галочкой помечен критерий, который сработал.
function describeVolumeHighlight(timestamp: number, turnover: number, shareThreshold: number): string[] {
  const lines: string[] = []
  const share = marketShares.get(timestamp)
  if (share != null) {
    const hit = share >= shareThreshold ? ' ✓' : ''
    lines.push(`Доля рынка: ${(share * 100).toFixed(1)}%${hit}`)
  }
  const base = volumeBaseline.get(timestamp)
  if (base != null && base.mean > 0) {
    const ratio = turnover / base.mean
    const pct = (ratio - 1) * 100
    const sign = pct >= 0 ? '+' : ''
    const hit = turnover > base.cut ? ' ✓' : ''
    const windowDays = Math.round(baselineWindowMs() / (24 * 60 * 60 * 1000))
    const suffix = isIntraday() ? `за ${windowDays} дн. в это время` : `за ${windowDays} дн.`
    lines.push(`Оборот: ${fmtRubles(turnover)} (×${ratio.toFixed(2)}, ${sign}${pct.toFixed(0)}% к среднему ${suffix})${hit}`)
  }
  return lines
}

// Мержит доли рынка (Map.set дедуплицирует по таймслоту). Полный сброс делается
// синхронно при пересоздании чарта, поэтому здесь всегда только добавление —
// так поздний ответ init не затирает доли, догруженные вперёд.
function applyMarketShares(points: MetricPoint[]) {
  for (const p of points) {
    marketShares.set(p.timestamp, p.value)
  }
}

// Пересчитывает множество подсвечиваемых таймслотов по загруженным свечам.
function recomputeHighlightTimes(chart: Chart, shareThreshold: number) {
  const dataList = chart.getDataList() as SuperCandle[]
  const next = new Set<number>()
  volumeBaseline = computeVolumeBaselines(dataList)
  for (const c of dataList) {
    const share = marketShares.get(c.timestamp)
    const byShare = share != null && share >= shareThreshold
    const base = volumeBaseline.get(c.timestamp)
    const byAnomaly = base != null && candleTurnover(c) > base.cut
    if (byShare || byAnomaly) {
      next.add(c.timestamp)
    }
  }
  highlightTimes = next
}

// id единственного оверлея подсветки (создаётся лениво, живёт между синками).
interface VolumeHighlightState {
  overlayId: string | null
}

// Перерисовывает рамки видимых подсвеченных свечей в одном оверлее. Вне видимого
// диапазона рамки не рисуются (иначе при мелком зуме их могут быть тысячи).
function syncVolumeHighlights(chart: Chart, state: VolumeHighlightState) {
  const dataList = chart.getDataList() as SuperCandle[]
  const points: Array<{ timestamp: number; value: number }> = []
  if (dataList.length > 0 && highlightTimes.size > 0) {
    const range = chart.getVisibleRange()
    const from = Math.max(0, range.realFrom)
    const to = Math.min(dataList.length - 1, range.realTo - 1)
    for (let i = from; i <= to && points.length < MAX_HIGHLIGHTS * 2; i++) {
      const c = dataList[i]
      if (highlightTimes.has(c.timestamp)) {
        points.push({ timestamp: c.timestamp, value: c.high })
        points.push({ timestamp: c.timestamp, value: c.low })
      }
    }
  }
  if (points.length === 0) {
    // override не умеет очищать точки (пустой массив игнорируется), поэтому
    // когда подсветок нет — оверлей удаляем.
    removeVolumeHighlights(chart, state)
    return
  }
  if (state.overlayId == null) {
    const overlayId = chart.createOverlay({ name: 'volumeHighlights', points }) as string | null
    if (overlayId != null) {
      state.overlayId = overlayId
    }
    return
  }
  chart.overrideOverlay({ id: state.overlayId, points })
}

function removeVolumeHighlights(chart: Chart, state: VolumeHighlightState) {
  if (state.overlayId != null) {
    chart.removeOverlay({ id: state.overlayId })
    state.overlayId = null
  }
}

// true, если таймстамп попадает на субботу или воскресенье (по МСК).
function isWeekend(ts: number): boolean {
  const day = new Date(mskDateKey(ts) + 'T00:00:00Z').getUTCDay()
  return day === 0 || day === 6
}

interface WeekendHighlight {
  overlayId: string
  startTs: number
  endTs: number
}

// Создаёт подсветку выходного дня или расширяет уже созданную, если подгрузка
// истории вперёд (влево) добавила более ранние свечи этого же дня.
function upsertWeekendHighlight(chart: Chart, weekends: Map<string, WeekendHighlight>, dateKey: string, startTs: number, endTs: number) {
  const existing = weekends.get(dateKey)
  if (!existing) {
    const overlayId = chart.createOverlay({ name: 'weekendHighlight', points: [{ timestamp: startTs }, { timestamp: endTs }] }) as string | null
    if (overlayId != null) {
      weekends.set(dateKey, { overlayId, startTs, endTs })
    }
    return
  }
  if (existing.startTs !== startTs || existing.endTs !== endTs) {
    chart.overrideOverlay({ id: existing.overlayId, points: [{ timestamp: startTs }, { timestamp: endTs }] })
    existing.startTs = startTs
    existing.endTs = endTs
  }
}

// Ключ периода, между которым рисуется разделитель: на внутридневных интервалах
// это календарный день (МСК), на дневках — неделя (понедельник). Вызывается
// только для внутридневных и дневного интервалов (неделя/месяц отсекаются в
// syncPeriodSeparators).
function separatorPeriodKey(ts: number): string {
  if (currentInterval === IntervalType.Day) {
    return mskMonday(ts)
  }
  return mskDateKey(ts)
}

// Рисует разделители периодов и подсветку выходных для уже загруженных свечей.
// Отслеживает уже созданные timestamps/даты, чтобы не плодить дубли при подгрузке
// истории вперёд.
function syncPeriodSeparators(chart: Chart, createdSeparators: Set<number>, createdWeekends: Map<string, WeekendHighlight>) {
  const dataList = chart.getDataList() as SuperCandle[]
  if (currentInterval === IntervalType.Week || currentInterval === IntervalType.Month) {
    return
  }
  let prevKey = ''
  let prevDateKey: string | null = null
  let dayStartTs = 0
  let prevTs = 0
  for (const c of dataList) {
    const key = separatorPeriodKey(c.timestamp)
    const dateKey = mskDateKey(c.timestamp)

    // Разделитель на границе периода (день/неделя).
    if (prevKey !== '' && key !== prevKey && !createdSeparators.has(c.timestamp)) {
      createdSeparators.add(c.timestamp)
      chart.createOverlay({ name: 'periodSeparator', points: [{ timestamp: c.timestamp }] })
    }

    // Подсветка выходного дня целиком — по переходу на новый календарный день
    // финализируем предыдущий, если он суббота/воскресенье.
    if (prevDateKey !== null && dateKey !== prevDateKey) {
      if (isWeekend(prevTs)) {
        upsertWeekendHighlight(chart, createdWeekends, prevDateKey, dayStartTs, prevTs)
      }
      dayStartTs = c.timestamp
    } else if (prevDateKey === null) {
      dayStartTs = c.timestamp
    }

    prevKey = key
    prevDateKey = dateKey
    prevTs = c.timestamp
  }
  // Последний (ещё не финализированный) день.
  if (prevDateKey !== null && isWeekend(prevTs)) {
    upsertWeekendHighlight(chart, createdWeekends, prevDateKey, dayStartTs, prevTs)
  }
}

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

function readQuery() {
  const params = new URLSearchParams(window.location.search)
  const validIntervals = Object.values(IntervalType)
  const interval = params.get('interval') ?? ''
  return {
    ticker: params.get('ticker') ?? 'ROSN',
    interval: validIntervals.includes(interval) ? interval : IntervalType.Hour,
  }
}

export default function ChartType () {
  const [initial] = useState(readQuery)
  const [ticker, setTicker] = useState(initial.ticker)
  const [interval, setInterval] = useState(initial.interval)
  const [enabled, setEnabled] = useState<Record<VolumeIndicatorId, boolean>>(DEFAULT_ENABLED)
  const [oiAvailable, setOIAvailable] = useState(false)
  const [metrics, setMetrics] = useState<MetricIndicatorSpec[]>([])
  const [metricInput, setMetricInput] = useState('')
  const [superTable, setSuperTable] = useState<string | null>(null)
  const [dataError, setDataError] = useState<string | null>(null)
  // Доля рынка считается только для акций (sec_group='stock_shares').
  const [stockSharesAvailable, setStockSharesAvailable] = useState(false)
  const [highlightVolume, setHighlightVolume] = useState(true)
  // Порог доли рынка в процентах (число + черновик строки для поля ввода,
  // чтобы можно было стереть значение и набрать новое).
  const [shareThresholdPct, setShareThresholdPct] = useState(DEFAULT_SHARE_THRESHOLD * 100)
  const [shareThresholdText, setShareThresholdText] = useState(String(DEFAULT_SHARE_THRESHOLD * 100))
  const chart = useRef<Chart | null>(null)
  const futoiTickerRef = useRef<string | null>(null)
  const futoiAssetRef = useRef<string | null>(null)
  // Значения, читаемые из колбэков чарта, которые не пересоздаются при смене
  // состояния (включена ли подсветка, порог доли рынка 0..1).
  const highlightVolumeRef = useRef(highlightVolume)
  const shareThresholdRef = useRef(shareThresholdPct / 100)
  // Единственный оверлей подсветки; пересоздаётся вместе с чартом.
  const volumeHighlightStateRef = useRef<VolumeHighlightState>({ overlayId: null })
  // Пересчёт множества подсветок + отрисовка (назначается в основном эффекте).
  const refreshVolumeHighlightsRef = useRef<() => void>(() => {})
  // Всплывашка с причиной подсветки. Содержимое (без координат) в state — чтобы
  // не перерисовывать список контролов на каждое движение мыши; позиция —
  // напрямую через DOM-реф.
  const [volumeTip, setVolumeTip] = useState<string[] | null>(null)
  const volumeTipKeyRef = useRef<number | null>(null)
  const volumeTipElRef = useRef<HTMLDivElement | null>(null)
  const volumeTipCoordRef = useRef<{ x: number; y: number }>({ x: 0, y: 0 })

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    params.set('ticker', ticker)
    params.set('interval', interval)
    window.history.replaceState(null, '', `${window.location.pathname}?${params.toString()}`)
  }, [ticker, interval])

  // Позиционирование всплывашки рядом с курсором (в координатах контейнера
  // чарта). Храним в рефе, чтобы колбэк чарта не зависел от пересоздания.
  // Ширины кэшируются при смене содержимого, чтобы не читать layout на каждое
  // движение мыши (в hot-path только запись transform).
  const volumeTipSizeRef = useRef({ parentWidth: 0, tipWidth: 0 })
  const placeVolumeTipRef = useRef<() => void>(() => {})
  placeVolumeTipRef.current = () => {
    const el = volumeTipElRef.current
    if (el == null) return
    const { x, y } = volumeTipCoordRef.current
    const { parentWidth, tipWidth } = volumeTipSizeRef.current
    const offset = 12
    const maxLeft = parentWidth - tipWidth - 4
    const left = Math.min(Math.max(4, x + offset), Math.max(4, maxLeft))
    el.style.transform = `translate(${Math.round(left)}px, ${Math.round(y + offset)}px)`
  }

  const hideVolumeTipRef = useRef<() => void>(() => {})
  hideVolumeTipRef.current = () => {
    if (volumeTipKeyRef.current !== null) {
      volumeTipKeyRef.current = null
      setVolumeTip(null)
    }
  }

  // useLayoutEffect — чтобы первый кадр с контентом сразу был на месте (без
  // «вспышки» у левого верхнего угла). Меряем ширины один раз на контент.
  useLayoutEffect(() => {
    if (volumeTip == null) return
    const el = volumeTipElRef.current
    if (el != null) {
      volumeTipSizeRef.current = {
        tipWidth: el.offsetWidth,
        parentWidth: el.parentElement?.clientWidth ?? 0,
      }
      placeVolumeTipRef.current()
    }
  }, [volumeTip])

  const toggleIndicator = (id: VolumeIndicatorId) => {
    setEnabled(prev => ({ ...prev, [id]: !prev[id] }))
  }

  const addMetric = () => {
    const expression = metricInput.trim()
    if (!expression) return
    const spec = makeMetricIndicator(expression)
    setMetrics(prev => [...prev, spec])
    setMetricInput('')
  }

  const removeMetric = (id: number) => {
    const spec = metrics.find(f => f.id === id)
    if (spec) {
      chart.current?.removeIndicator({ paneId: spec.paneId })
      removeMetricValues(spec.key)
    }
    setMetrics(prev => prev.filter(f => f.id !== id))
  }

  // Основной эффект: инициализация чарта и загрузка данных
  useEffect(() => {
    let cancelled = false

    setOIAvailable(false)
    setDataError(null)
    setSuperTable(null)
    setStockSharesAvailable(false)
    clearMetricValues()
    fizOIMap = new Map()
    fizOIDaily = []
    fizOIDailyMode = false
    futoiTickerRef.current = null
    futoiAssetRef.current = null
    marketShares = new Map()
    highlightTimes = new Set()
    volumeBaseline = new Map()
    currentInterval = interval

    // Резолвим группу инструмента: по ней определяется таблица суперсвечей (для
    // UI и SQL-метрик) и участие в рыночном ранжировании по доле оборота.
    const secGroupPromise = resolveSecGroup(ticker)
    secGroupPromise.then(group => {
      if (!cancelled) setSuperTable(group ? SEC_GROUP_TO_TABLE[group] ?? null : null)
    }).catch(() => {
      if (!cancelled) setSuperTable(null)
    })

    chart.current = init("real-time-k-line", { styles: klineStyle })
    // Разделители периодов уже созданные для текущего чарта (сбрасываются вместе
    // с пересозданием чарта при смене тикера/интервала).
    const createdSeparators = new Set<number>()
    const createdWeekends = new Map<string, WeekendHighlight>()
    // Старый оверлей уничтожен вместе с чартом — начинаем с чистого состояния.
    volumeHighlightStateRef.current = { overlayId: null }

    // Пересчитываем множество подсветок и рисуем его для видимого диапазона.
    const refreshVolumeHighlights = () => {
      if (cancelled || !chart.current) return
      if (!highlightVolumeRef.current) {
        hideVolumeTipRef.current()
        return
      }
      recomputeHighlightTimes(chart.current, shareThresholdRef.current)
      syncVolumeHighlights(chart.current, volumeHighlightStateRef.current)
      // Тултип не закрываем, если наведённая свеча всё ещё подсвечена (иначе он
      // пропадал бы при подгрузке истории), но обновляем текст; если критерий
      // пропал — закрываем.
      const openKey = volumeTipKeyRef.current
      if (openKey != null) {
        if (!highlightTimes.has(openKey)) {
          hideVolumeTipRef.current()
        } else {
          const candle = (chart.current.getDataList() as SuperCandle[]).find(c => c.timestamp === openKey)
          if (candle != null) {
            setVolumeTip(describeVolumeHighlight(openKey, candleTurnover(candle), shareThresholdRef.current))
          }
        }
      }
    }
    refreshVolumeHighlightsRef.current = refreshVolumeHighlights

    // При скролле/зуме множество подсветок не меняется — только перерисовываем
    // видимую часть одним оверлеем.
    const resyncVisibleHighlights = () => {
      if (cancelled || !chart.current) return
      if (highlightVolumeRef.current) {
        syncVolumeHighlights(chart.current, volumeHighlightStateRef.current)
      }
    }
    chart.current?.subscribeAction('onVisibleRangeChange', resyncVisibleHighlights)

    // Всплывашка при наведении: если под курсором подсвеченная свеча — показываем
    // долю рынка и превышение среднего объёма.
    volumeTipKeyRef.current = null
    setVolumeTip(null)
    const onCrosshairChange = (data?: unknown) => {
      if (cancelled) return
      const c = chart.current
      const cr = data as { x?: number; y?: number; paneId?: string } | undefined
      // Показываем только для панели свечей; на индикаторных панелях те же бары,
      // но другой Y (подсказка уезжает вверх).
      if (!c || !highlightVolumeRef.current || cr?.paneId !== 'candle_pane' || typeof cr.x !== 'number' || typeof cr.y !== 'number') {
        hideVolumeTipRef.current()
        return
      }
      const point = (c.convertFromPixel([{ x: cr.x }], { paneId: 'candle_pane' }) as Array<Partial<Point>>)[0]
      const dataIndex = point?.dataIndex
      if (dataIndex == null || dataIndex < 0) {
        hideVolumeTipRef.current()
        return
      }
      const candle = (c.getDataList() as SuperCandle[])[Math.round(dataIndex)]
      if (candle == null || !highlightTimes.has(candle.timestamp)) {
        hideVolumeTipRef.current()
        return
      }
      volumeTipCoordRef.current = { x: cr.x, y: cr.y }
      if (volumeTipKeyRef.current !== candle.timestamp) {
        volumeTipKeyRef.current = candle.timestamp
        setVolumeTip(describeVolumeHighlight(candle.timestamp, candleTurnover(candle), shareThresholdRef.current))
      } else {
        placeVolumeTipRef.current()
      }
    }
    chart.current?.subscribeAction('onCrosshairChange', onCrosshairChange)

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

    // Загружаем SQL-метрики для диапазона (отдельные запросы, см. fetchMetric).
    async function loadMetrics(till: Date) {
      for (const m of metrics) {
        try {
          const points = await fetchMetric(ticker, till, interval, m.expression)
          if (cancelled) return
          setMetricValues(m.key, points)
        } catch (error) {
          console.error('Ошибка загрузки метрики:', error)
          if (!cancelled) setDataError(error instanceof Error ? error.message : String(error))
        }
      }
    }

    // Докрутка SQL-метрик вперёд при подгрузке истории.
    async function loadMetricsForward(till: Date) {
      for (const m of metrics) {
        try {
          const points = await fetchMetric(ticker, till, interval, m.expression)
          if (cancelled) return
          appendMetricValues(m.key, points)
        } catch (error) {
          console.error('Ошибка загрузки метрики:', error)
        }
      }
    }

    // Доля инструмента в обороте всех акций. Считается только для акций
    // (sec_group='stock_shares') и не на 1m: исходные суперсвечи 5-минутные,
    // поэтому минутные таймслоты не совпадут со свечами графика.
    async function loadMarketShares(till: Date) {
      try {
        if (interval === IntervalType.Minute) {
          setStockSharesAvailable(false)
          return
        }
        const group = await secGroupPromise
        if (cancelled) return
        const available = group === 'stock_shares'
        setStockSharesAvailable(available)
        if (!available) return
        const points = await fetchMarketShares(ticker, till, interval)
        if (cancelled) return
        applyMarketShares(points)
      } catch (error) {
        console.error('Ошибка загрузки долей рынка:', error)
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
              await loadMetrics(latestTime)
              loadMarketShares(latestTime).then(refreshVolumeHighlights)
              if (cancelled) return
              callback(candles, { forward: true, backward: false })
              if (chart.current) syncPeriodSeparators(chart.current, createdSeparators, createdWeekends)
              refreshVolumeHighlights()
            })
            .catch(error => {
              console.error('Ошибка загрузки данных:', error)
              if (!cancelled) {
                setDataError(error instanceof Error ? error.message : String(error))
                callback([], { forward: false, backward: false })
              }
            })
        } else if (type === 'forward' && timestamp != null) {
          fetchCandles(ticker, new Date(timestamp), interval)
            .then(async candles => {
              if (cancelled) return
              if (candles.length > 0) {
                await loadOIForward(new Date(timestamp))
                await loadMetricsForward(new Date(timestamp))
                loadMarketShares(new Date(timestamp)).then(refreshVolumeHighlights)
              }
              if (cancelled) return
              callback(candles, { forward: candles.length !== 0, backward: false })
              if (chart.current) syncPeriodSeparators(chart.current, createdSeparators, createdWeekends)
              refreshVolumeHighlights()
            })
            .catch(error => {
              console.error('Ошибка загрузки данных:', error)
              if (!cancelled) {
                setDataError(error instanceof Error ? error.message : String(error))
                callback([], { forward: false, backward: false })
              }
            })
        } else {
          callback([], { forward: false, backward: false })
        }
      },
    })

    return () => {
      cancelled = true
      if (chart.current) {
        chart.current.unsubscribeAction('onVisibleRangeChange', resyncVisibleHighlights)
        chart.current.unsubscribeAction('onCrosshairChange', onCrosshairChange)
        dispose(chart.current)
        chart.current = null
      }
    }
  }, [ticker, interval, metrics])

  // Включение/выключение подсветки и смена порога доли рынка.
  useEffect(() => {
    highlightVolumeRef.current = highlightVolume
    shareThresholdRef.current = shareThresholdPct / 100
    const c = chart.current
    if (!c) return
    if (highlightVolume) {
      refreshVolumeHighlightsRef.current()
    } else {
      hideVolumeTipRef.current()
      removeVolumeHighlights(c, volumeHighlightStateRef.current)
    }
  }, [highlightVolume, shareThresholdPct, ticker, interval])

  // Синхронизация индикаторов с чартом: для каждого включённого индикатора
  // держим свою панель, для выключенного — убираем панель.
  useEffect(() => {
    const c = chart.current
    if (!c) return
    for (const ind of VOLUME_INDICATORS) {
      const hidden = ind.hiddenOn?.includes(interval) ?? false
      const shouldShow = enabled[ind.id] && (!ind.requiresOI || oiAvailable) && !hidden
      const isShown = c.getIndicators({ paneId: ind.paneId }).length > 0
      if (shouldShow && !isShown) {
        c.createIndicator({ name: ind.indicatorName, paneId: ind.paneId })
      } else if (!shouldShow && isShown) {
        c.removeIndicator({ paneId: ind.paneId })
      }
    }
    // Индикаторы по SQL-метрике — одна панель на метрику.
    for (const f of metrics) {
      const isShown = c.getIndicators({ paneId: f.paneId }).length > 0
      if (!isShown) {
        c.createIndicator({ name: f.indicatorName, paneId: f.paneId })
      }
    }
  }, [enabled, oiAvailable, ticker, interval, metrics])

  // Высота контейнера растёт вместе с числом включённых индикаторов,
  // чтобы основной график не сжимался.
  const visibleIndicatorCount = VOLUME_INDICATORS.filter(
    ind => enabled[ind.id] && (!ind.requiresOI || oiAvailable) && !(ind.hiddenOn?.includes(interval) ?? false)
  ).length
  const containerHeight = 480 + visibleIndicatorCount * 100 + metrics.length * 100

  return (
    <Layout title={`${ticker} ${interval}`} style={{ height: containerHeight }}>
      <TickerSelector onSelect={setTicker} />
      <div className="k-line-chart-menu-container">
        <button onClick={_ => setInterval(IntervalType.FiveMinutes)} style={{ backgroundColor: interval === IntervalType.FiveMinutes ? '#4CAF50' : '' }}>5m</button>
        <button onClick={_ => setInterval(IntervalType.Hour)} style={{ backgroundColor: interval === IntervalType.Hour ? '#4CAF50' : '' }}>hour</button>
        <button onClick={_ => setInterval(IntervalType.Day)} style={{ backgroundColor: interval === IntervalType.Day ? '#4CAF50' : '' }}>day</button>
        <button onClick={_ => setInterval(IntervalType.Week)} style={{ backgroundColor: interval === IntervalType.Week ? '#4CAF50' : '' }}>week</button>
        <button onClick={_ => setInterval(IntervalType.Month)} style={{ backgroundColor: interval === IntervalType.Month ? '#4CAF50' : '' }}>month</button>

        <label
          style={{ marginLeft: 12, display: 'inline-flex', alignItems: 'center', cursor: 'pointer' }}
          title="Подсвечивать свечи с долей ≥ порога от оборота всех акций ИЛИ с оборотом (₽) выше mean+2σ своей истории за то же время суток"
        >
          <input
            type="checkbox"
            checked={highlightVolume}
            onChange={_ => setHighlightVolume(v => !v)}
          />
          <span style={{ paddingLeft: 4 }}>Подсветка объёма</span>
        </label>
        <label
          style={{
            marginLeft: 8,
            display: 'inline-flex',
            alignItems: 'center',
            cursor: stockSharesAvailable ? 'pointer' : 'not-allowed',
            opacity: stockSharesAvailable ? 1 : 0.5,
          }}
          title={
            stockSharesAvailable
              ? "Порог доли рынка: свеча подсвечивается, если оборот инструмента ≥ X% оборота всех акций"
              : "Доля рынка считается только для акций"
          }
        >
          <span style={{ paddingRight: 4 }}>доля рынка ≥</span>
          <input
            type="number"
            min={1}
            max={100}
            step={1}
            value={shareThresholdText}
            disabled={!stockSharesAvailable}
            onChange={e => {
              const text = e.target.value
              setShareThresholdText(text)
              if (text.trim() === '') return
              const n = Number(text)
              if (!Number.isFinite(n)) return
              setShareThresholdPct(Math.min(100, Math.max(1, Math.round(n))))
            }}
            onBlur={() => setShareThresholdText(String(shareThresholdPct))}
            style={{ width: 56, height: 24 }}
          />
          <span style={{ paddingLeft: 4 }}>%</span>
        </label>

        <span style={{ paddingLeft: 12, paddingRight: 6 }}>Индикаторы:</span>
        {VOLUME_INDICATORS.map(ind => {
          const oiDisabled = ind.requiresOI && !oiAvailable
          const hidden = ind.hiddenOn?.includes(interval) ?? false
          const disabled = oiDisabled || hidden
          return (
            <label
              key={ind.id}
              style={{
                marginRight: 12,
                display: 'inline-flex',
                alignItems: 'center',
                cursor: disabled ? 'not-allowed' : 'pointer',
                opacity: disabled ? 0.5 : 1,
              }}
              title={
                oiDisabled
                  ? "Нет данных по открытому интересу"
                  : hidden
                    ? "Индикатор недоступен на этом интервале"
                    : ind.label
              }
            >
              <input
                type="checkbox"
                checked={enabled[ind.id]}
                disabled={disabled}
                onChange={_ => toggleIndicator(ind.id)}
              />
              <span style={{ paddingLeft: 4 }}>{ind.label}</span>
            </label>
          )
        })}
      </div>
      <div className="k-line-chart-menu-container">
        <span style={{ paddingRight: 6 }}>SQL-метрика:</span>
        <input
          type="text"
          value={metricInput}
          placeholder="sum(val_b) - sum(val_s)"
          onChange={e => setMetricInput(e.target.value)}
          onKeyDown={e => { if (e.key === 'Enter') addMetric() }}
          title={`Агрегатное выражение над сырыми колонками таблицы ${superTable ?? 'super_*'}. Группировка по timeslot, поэтому нужен агрегат (sum/argMax/...). Примеры: sum(vol_b)-sum(vol_s), argMax(pr_close,time)-argMax(pr_open,time)`}
          style={{ height: 24, padding: '0 6px', marginRight: 8, width: 260 }}
        />
        <button onClick={addMetric}>Добавить</button>
        {superTable != null && <span style={{ paddingLeft: 8 }}>{superTable}</span>}
      </div>
      {dataError != null && (
        <div className="k-line-chart-menu-container" style={{ color: '#EF5350' }}>
          {dataError}
        </div>
      )}
      {metrics.length > 0 && (
        <div className="k-line-chart-menu-container">
          <span style={{ paddingRight: 6 }}>Свои метрики:</span>
          {metrics.map(f => (
            <span key={f.id} style={{ marginRight: 8, display: 'inline-flex', alignItems: 'center' }}>
              <span>{f.expression}</span>
              <button
                onClick={_ => removeMetric(f.id)}
                style={{ backgroundColor: '#EF5350', marginLeft: 4 }}>
                ✕
              </button>
            </span>
          ))}
        </div>
      )}
      <div
        className="k-line-chart-wrapper"
        onMouseLeave={() => hideVolumeTipRef.current()}
        onMouseMove={e => {
          // На осях/разделителях klinecharts сбрасывает кроссхейр без колбэка,
          // поэтому прячем подсказку, как только курсор ушёл с области свечей.
          if (volumeTipKeyRef.current === null) return
          const mainDom = chart.current?.getDom('candle_pane', 'main')
          if (mainDom == null || !mainDom.contains(e.target as Node)) {
            hideVolumeTipRef.current()
          }
        }}
      >
        <div id="real-time-k-line" className="k-line-chart" />
        {volumeTip != null && (
          <div ref={volumeTipElRef} className="volume-highlight-tip">
            <div className="volume-highlight-tip-title">Подсвечено</div>
            {volumeTip.length === 0
              ? <div>—</div>
              : volumeTip.map((line, i) => <div key={i}>{line}</div>)}
          </div>
        )}
      </div>
    </Layout>
  )
}
