import { Chart, KLineData, registerIndicator, YAxis } from 'klinecharts'
import { isGapBar, SuperCandle } from '../data/index'

// Daily Volume Profile — распределение объёма по ценовым уровням внутри каждого
// торгового дня, накладываемое прямо на основной график (панель свечей).
//
// Важное допущение: в БД хранятся агрегированные суперсвечи (OHLCV + разбивка
// vol_b/vol_s за интервал), а не «сырые» сделки по ценам. Поэтому объём каждого
// бара раскладывается равномерно по его ценовому диапазону [low..high] с шагом
// Row Size. Чем мельче интервал (1m/5m), тем точнее приближение профиля.
//
// Следствие для PbD: у длинного импульсного (пробойного) бара равномерная
// раскладка «размазывает» объём по всей длине свечи и может смещать POC/VA.
// Это осознанный компромисс при отсутствии тиковых данных — при появлении
// таблицы сделок по ценам раскладку стоит заменить точной. Альтернатива без
// новых данных (вес к close/телу) — эвристика, которую нельзя вводить без
// проверки на реальных POC.

export type VpRowSize = 'auto' | 1 | 2 | 5 | 10
export type VpAlign = 'left' | 'right'

export interface VolumeProfileConfig {
  // Шаг цены: 'auto' подбирается под масштаб по Y, число — множитель тика.
  rowSize: VpRowSize
  // Доля дневного объёма, попадающая в зону стоимости (Value Area), %.
  valueAreaPct: number
  // Показывать только POC (без баров и границ VA).
  showOnlyPoc: boolean
  // Рисовать границы зоны стоимости VAH/VAL.
  showValueAreaLines: boolean
  // Разделять объём внутри бара на покупки (зелёный) и продажи (красный).
  splitBuySell: boolean
  // К какой границе дня прижимать профиль.
  align: VpAlign
}

export const volumeProfileConfig: VolumeProfileConfig = {
  rowSize: 'auto',
  valueAreaPct: 70,
  showOnlyPoc: false,
  showValueAreaLines: true,
  splitBuySell: false,
  align: 'right',
}

// Индикатор доступен только на внутридневных интервалах (там есть структура
// дня); на дневках/неделях/месяцах профиль строится не из чего.
export function isVolumeProfileInterval(interval: string): boolean {
  return interval === '1m' || interval === '5m' || interval === 'hour'
}

export const VOLUME_PROFILE_PANE_ID = 'candle_pane'
export const VOLUME_PROFILE_INDICATOR_NAME = 'volume_profile'

// --- Оформление ------------------------------------------------------------
// Полупрозрачный серо-синий, чтобы не перекрывать японские свечи. Бары внутри
// зоны стоимости ярче, вне — тусклее.
const COLOR_BAR_OUT = 'rgba(58, 75, 92, 0.25)'
const COLOR_BAR_IN = 'rgba(58, 75, 92, 0.62)'
// Buy/Sell в режиме разделения; тусклые/яркие — вне/внутри зоны стоимости.
const COLOR_BUY_IN = 'rgba(38, 166, 154, 0.72)'
const COLOR_BUY_OUT = 'rgba(38, 166, 154, 0.30)'
const COLOR_SELL_IN = 'rgba(239, 83, 80, 0.72)'
const COLOR_SELL_OUT = 'rgba(239, 83, 80, 0.30)'
const COLOR_POC = '#FFD700'
const COLOR_VA = 'rgba(176, 190, 197, 0.95)'

// Максимальная длина самого длинного бара (POC) — доля ширины дневной сессии.
const MAX_BAR_WIDTH_FRACTION = 0.30
// Профили для слишком узких дней не рисуем — иначе каша и лаги.
const MIN_DAY_WIDTH_PX = 24
// Не больше стольких дневных профилей одновременно на экране.
const MAX_VISIBLE_DAYS = 40
// Авто-шаг цены: стремимся к ~3px на строку, в пределах MIN..MAX строк.
const ROW_PX_TARGET = 3
const MIN_ROWS = 12
const MAX_ROWS = 80
// Бар короче половины пикселя не имеет смысла.
const MIN_BAR_WIDTH_PX = 0.5
// Строка профиля тоньше пикселя при фиксированном мелком шаге — не рисуем.
const MIN_ROW_HEIGHT_PX = 1
// Абсолютный предохранитель на число баров одного дня (при нормальном масштабе
// строки не тоньше пикселя, поэтому до него дело не доходит).
const MAX_DRAWN_ROWS = 1000

interface VpBucket {
  index: number
  priceStart: number
  priceEnd: number
  total: number
  buy: number
  sell: number
}

interface DayProfile {
  dayKey: string
  rowSize: number
  buckets: VpBucket[]
  byIndex: Map<number, VpBucket>
  totalVolume: number
  maxTotal: number
  pocIndex: number
  pocPrice: number
  valIndex: number
  vahIndex: number
  vah: number
  val: number
  hasBuySell: boolean
}

interface DayEntry {
  dayKey: string
  startIndex: number
  endIndex: number
  high: number
  low: number
}

// --- Кэш вычислений --------------------------------------------------------
// Модульное состояние (как fizOIOwn/marketShares в ChartType) рассчитано на
// единственный экземпляр чарта; при смене инструмента/интервала сбрасывается
// через resetVolumeProfileCache().
//
// Индекс дней пересобирается при любом изменении данных (быстро, O(N)).
// Профили кэшируются ПО ДНЮ и по шагу: завершённые дни не пересчитываются при
// обновлении текущего бара (в реальном времени меняется только последний день),
// поэтому исторические POC/VA для отката (PbD) остаются стабильными.
let indexSignature = ''
let dayEntries: DayEntry[] = []
// ключ "dayKey|rowSize|valueAreaPct" -> подпись данных дня + посчитанный профиль.
let profileCache = new Map<string, { dayKey: string; sig: string; profile: DayProfile }>()
let tickCache: { signature: string; tick: number } | null = null

// Жёсткий предел числа профилей в кэше (защита от роста при переборе шагов
// Row Size). Сверху вытесняется самый старый (Map хранит порядок вставки).
const MAX_PROFILE_CACHE = 4000

// Полный сброс кэша. Нужен при смене инструмента/интервала, чтобы не осталось
// профилей предыдущего инструмента.
export function resetVolumeProfileCache() {
  indexSignature = ''
  dayEntries = []
  profileCache = new Map()
  tickCache = null
}

// Дата в таймзоне МСК (Europe/Moscow, UTC+3 без DST) в формате YYYY-MM-DD.
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

function clamp(v: number, lo: number, hi: number): number {
  return Math.min(hi, Math.max(lo, v))
}

// Индекс ценовой строки для цены. Эпсилон защищает от ошибок округления на
// границах строк (напр. 0.3 / 0.1 = 2.9999999999999996 вместо 3). Эпсилон
// относительный: масштабируется вместе с price/rowSize, поэтому корректно
// работает и для больших цен при мелком шаге (абсолютный 1e-9 там был бы
// ничтожен, а при price/rowSize ~10 мог бы «перебросить» реальную цену чуть
// ниже границы в верхнюю строку).
function bucketIndex(price: number, rowSize: number): number {
  const q = price / rowSize
  const eps = Math.max(1e-12, Math.abs(q) * 1e-12)
  return Math.floor(q + eps)
}

// Подпись набора данных для пересборки индекса дней. Должна меняться при любом
// изменении (загрузка истории, realtime-бар). Включает хвост, чтобы обновлялись
// high/low текущего дня (нужны для авто-шага). Профили при этом НЕ сбрасываются —
// их инвалидация идёт по подписи конкретного дня (см. getProfile).
function dataSignature(dataList: KLineData[]): string {
  if (dataList.length === 0) return '0'
  const last = dataList[dataList.length - 1]
  return `${dataList.length}|${dataList[0].timestamp}|${last.timestamp}|${last.volume ?? ''}|${last.close ?? ''}`
}

// Подпись данных ОДНОГО дня. По ней решаем, пересчитывать ли профиль этого дня.
// Завершённые дни при обновлении текущего бара не меняются → их подпись та же.
function daySignature(dataList: KLineData[], day: DayEntry): string {
  const first = dataList[day.startIndex]
  const last = dataList[day.endIndex]
  return `${day.endIndex - day.startIndex}|${first.timestamp}|${last.timestamp}|${last.volume ?? ''}|${last.close ?? ''}`
}

function ensureDayIndex(dataList: KLineData[]) {
  const signature = dataSignature(dataList)
  if (signature === indexSignature) return
  indexSignature = signature
  dayEntries = []
  let cur: DayEntry | null = null
  for (let i = 0; i < dataList.length; i++) {
    const c = dataList[i]
    if (isGapBar(c)) continue
    const key = mskDateKey(c.timestamp)
    if (cur == null || key !== cur.dayKey) {
      cur = { dayKey: key, startIndex: i, endIndex: i, high: c.high, low: c.low }
      dayEntries.push(cur)
    } else {
      cur.endIndex = i
      if (c.high > cur.high) cur.high = c.high
      if (c.low < cur.low) cur.low = c.low
    }
  }
  // Профили дней, выпавших из загруженного окна, больше не нужны — чистим.
  const liveDays = new Set(dayEntries.map(d => d.dayKey))
  profileCache.forEach((entry, key) => {
    if (!liveDays.has(entry.dayKey)) {
      profileCache.delete(key)
    }
  })
}

// День по индексу бара. dayEntries отсортирован по startIndex и не пересекается,
// поэтому годится бинарный поиск: O(log дней) на движение мыши вместо O(дней).
function findDay(dataIndex: number): DayEntry | null {
  let lo = 0
  let hi = dayEntries.length - 1
  while (lo <= hi) {
    const mid = (lo + hi) >> 1
    const day = dayEntries[mid]
    if (dataIndex < day.startIndex) {
      hi = mid - 1
    } else if (dataIndex > day.endIndex) {
      lo = mid + 1
    } else {
      return day
    }
  }
  return null
}

// Оценка «одного тика» — минимальный положительный шаг цены в данных, округлённый
// до точности инструмента. Для биржевых бумаг это обычно 0.01 ₽.
function inferTick(dataList: KLineData[], precision: number): number {
  const signature = dataSignature(dataList) + '|' + precision
  if (tickCache != null && tickCache.signature === signature) {
    return tickCache.tick
  }
  const factor = Math.pow(10, precision)
  const prices: number[] = []
  for (const c of dataList) {
    if (isGapBar(c)) continue
    prices.push(Math.round(c.low * factor) / factor)
    prices.push(Math.round(c.high * factor) / factor)
  }
  prices.sort((a, b) => a - b)
  let minDiff = Infinity
  for (let i = 1; i < prices.length; i++) {
    const d = prices[i] - prices[i - 1]
    if (d > 1e-9 && d < minDiff) minDiff = d
  }
  const fallback = 1 / factor
  const tick = Number.isFinite(minDiff) ? Number(minDiff.toFixed(precision + 2)) : fallback
  tickCache = { signature, tick: tick > 0 ? tick : fallback }
  return tickCache.tick
}

// Снап числа к ближайшему большему «красивому» множителю (1/2/5 × 10^k).
// Последовательность 1,2,5,10,20,50,... покрывается парой (m ∈ {1,2,5}, p = 10^k).
function niceMultiple(value: number): number {
  if (!(value > 0) || !Number.isFinite(value)) return 1
  let p = 1
  for (let guard = 0; guard < 12; guard++) {
    for (const m of [1, 2, 5]) {
      if (m * p >= value) return m * p
    }
    p *= 10
  }
  return value
}

function rowsFromPixelHeight(range: number, pixelHeight: number, tick: number): number {
  const rows = clamp(Math.round(pixelHeight / ROW_PX_TARGET), MIN_ROWS, MAX_ROWS)
  const raw = range / rows
  return niceMultiple(raw / tick) * tick
}

function resolveRowSize(day: DayEntry, tick: number, pixelHeight: number): number {
  const cfg = volumeProfileConfig
  if (cfg.rowSize !== 'auto') {
    return cfg.rowSize * tick
  }
  return rowsFromPixelHeight(day.high - day.low, pixelHeight, tick)
}

// --- Расчёт профиля дня ----------------------------------------------------
// Чистая функция: раскладывает объём баров по ценовым строкам и считает POC и
// зону стоимости. Вынесена отдельно, чтобы её можно было покрыть тестами.
export function computeVolumeProfile(
  bars: SuperCandle[],
  dayKey: string,
  rowSize: number,
  valueAreaPct: number,
): DayProfile {
  const emptyProfile = (): DayProfile => ({
    dayKey, rowSize, buckets: [], byIndex: new Map(), totalVolume: 0, maxTotal: 0,
    pocIndex: 0, pocPrice: 0, valIndex: 0, vahIndex: 0, vah: 0, val: 0, hasBuySell: false,
  })
  // Шаг должен быть положительным, иначе деление на него даст мусорные бакеты.
  if (!(rowSize > 0)) return emptyProfile()

  const buckets = new Map<number, VpBucket>()
  for (const c of bars) {
    const volume = c.volume ?? 0
    const buy = c.volume_b ?? 0
    const sell = c.volume_s ?? 0
    let b0 = bucketIndex(c.low, rowSize)
    let b1 = bucketIndex(c.high, rowSize)
    if (b1 < b0) b1 = b0
    const count = b1 - b0 + 1
    const perTotal = volume / count
    const perBuy = buy / count
    const perSell = sell / count
    for (let b = b0; b <= b1; b++) {
      let bucket = buckets.get(b)
      if (bucket == null) {
        bucket = { index: b, priceStart: b * rowSize, priceEnd: (b + 1) * rowSize, total: 0, buy: 0, sell: 0 }
        buckets.set(b, bucket)
      }
      bucket.total += perTotal
      bucket.buy += perBuy
      bucket.sell += perSell
    }
  }

  const ordered = Array.from(buckets.values()).sort((a, b) => a.index - b.index)
  const byIndex = new Map<number, VpBucket>()
  let totalVolume = 0
  let maxTotal = 0
  let pocBucket: VpBucket | null = null
  let hasBuySell = false
  for (const bucket of ordered) {
    byIndex.set(bucket.index, bucket)
    totalVolume += bucket.total
    if (bucket.total > maxTotal) {
      maxTotal = bucket.total
      pocBucket = bucket
    }
    if (bucket.buy > 0 || bucket.sell > 0) hasBuySell = true
  }

  if (pocBucket == null) {
    return emptyProfile()
  }

  // Зона стоимости: расширяемся от POC вверх/вниз, каждый раз добавляя более
  // объёмную соседнюю строку, пока не наберём valueAreaPct дневного объёма.
  const target = totalVolume * clamp(valueAreaPct, 1, 100) / 100
  let sum = pocBucket.total
  let lo = pocBucket.index
  let hi = pocBucket.index
  const maxIter = ordered.length + 2
  for (let iter = 0; iter < maxIter && sum < target; iter++) {
    const below = byIndex.get(lo - 1)
    const above = byIndex.get(hi + 1)
    const belowTotal = below != null ? below.total : -1
    const aboveTotal = above != null ? above.total : -1
    if (aboveTotal < 0 && belowTotal < 0) break
    if (aboveTotal >= belowTotal) {
      hi += 1
      sum += aboveTotal
    } else {
      lo -= 1
      sum += belowTotal
    }
  }
  const loBucket = byIndex.get(lo) ?? pocBucket
  const hiBucket = byIndex.get(hi) ?? pocBucket

  return {
    dayKey,
    rowSize,
    buckets: ordered,
    byIndex,
    totalVolume,
    maxTotal,
    pocIndex: pocBucket.index,
    pocPrice: (pocBucket.priceStart + pocBucket.priceEnd) / 2,
    valIndex: loBucket.index,
    vahIndex: hiBucket.index,
    val: loBucket.priceStart,
    vah: hiBucket.priceEnd,
    hasBuySell,
  }
}

function computeDayProfile(dataList: KLineData[], day: DayEntry, rowSize: number): DayProfile {
  const bars: SuperCandle[] = []
  for (let i = day.startIndex; i <= day.endIndex; i++) {
    const c = dataList[i]
    if (!isGapBar(c)) bars.push(c as SuperCandle)
  }
  return computeVolumeProfile(bars, day.dayKey, rowSize, volumeProfileConfig.valueAreaPct)
}

// Ключ кэша профиля. Должен включать ВСЕ входы, влияющие на результат:
// день, шаг цены и процент зоны стоимости (от него зависят val/vah/valIndex/vahIndex).
function profileCacheKey(dayKey: string, rowSize: number, valueAreaPct: number): string {
  return `${dayKey}|${rowSize.toPrecision(12)}|${valueAreaPct}`
}

function getProfile(dataList: KLineData[], day: DayEntry, rowSize: number): DayProfile {
  const key = profileCacheKey(day.dayKey, rowSize, volumeProfileConfig.valueAreaPct)
  const sig = daySignature(dataList, day)
  const cached = profileCache.get(key)
  if (cached != null && cached.sig === sig) {
    return cached.profile
  }
  const profile = computeDayProfile(dataList, day, rowSize)
  profileCache.set(key, { dayKey: day.dayKey, sig, profile })
  // Вытесняем самые старые записи при переполнении (перебор шагов Row Size / VA%).
  while (profileCache.size > MAX_PROFILE_CACHE) {
    const oldest = profileCache.keys().next().value
    if (oldest == null) break
    profileCache.delete(oldest)
  }
  return profile
}

// Только для тестов: профили всех загруженных дней при текущих настройках,
// через общий per-day кэш. Позволяет проверить инвалидацию кэша (в т.ч. при
// смене valueAreaPct) без доступа к внутренним структурам.
export interface DayProfileSummary {
  dayKey: string
  pocPrice: number
  vah: number
  val: number
}

export function computeDayProfilesForTest(dataList: KLineData[], rowSize: number): DayProfileSummary[] {
  ensureDayIndex(dataList)
  return dayEntries.map(day => {
    const p = getProfile(dataList, day, rowSize)
    return { dayKey: p.dayKey, pocPrice: p.pocPrice, vah: p.vah, val: p.val }
  })
}

// --- Координатные помощники ------------------------------------------------
// chart.convertToPixel при передаче массива возвращает массив координат,
// поэтому разыменовываем первый элемент (как и остальной код проекта).
function xOfDataIndex(chart: Chart, dataIndex: number): number {
  const coord = (chart.convertToPixel([{ dataIndex }], { paneId: VOLUME_PROFILE_PANE_ID }) as Array<{ x?: number }>)[0]
  return coord?.x ?? 0
}

function yOfValue(chart: Chart, value: number): number {
  const coord = (chart.convertToPixel([{ value }], { paneId: VOLUME_PROFILE_PANE_ID }) as Array<{ y?: number }>)[0]
  return coord?.y ?? 0
}

// Дневная сессия на экране: от левого края первого бара до правого края
// последнего (учитываем полширины бара).
function dayEdges(chart: Chart, day: DayEntry): { xLeft: number; xRight: number } {
  const halfBar = chart.getBarSpace().halfBar
  return {
    xLeft: xOfDataIndex(chart, day.startIndex) - halfBar,
    xRight: xOfDataIndex(chart, day.endIndex) + halfBar,
  }
}

// --- Отрисовка -------------------------------------------------------------
function drawBar(
  ctx: CanvasRenderingContext2D,
  anchorX: number,
  width: number,
  top: number,
  height: number,
  align: VpAlign,
  color: string,
  offset = 0,
) {
  const x = align === 'left' ? anchorX + offset : anchorX - offset - width
  ctx.fillStyle = color
  ctx.fillRect(x, top, width, height)
}

function drawDayProfile(
  ctx: CanvasRenderingContext2D,
  chart: Chart,
  yAxis: YAxis,
  dataList: KLineData[],
  day: DayEntry,
  rowSize: number,
) {
  const cfg = volumeProfileConfig
  const profile = getProfile(dataList, day, rowSize)
  if (profile.buckets.length === 0 || profile.maxTotal <= 0) return

  const { xLeft, xRight } = dayEdges(chart, day)
  const dayWidth = xRight - xLeft
  if (dayWidth < MIN_DAY_WIDTH_PX) return

  const maxBarWidth = dayWidth * MAX_BAR_WIDTH_FRACTION
  const anchorX = cfg.align === 'left' ? xLeft : xRight

  if (!cfg.showOnlyPoc) {
    let drawn = 0
    for (const bucket of profile.buckets) {
      if (drawn >= MAX_DRAWN_ROWS) break
      const width = (bucket.total / profile.maxTotal) * maxBarWidth
      if (width < MIN_BAR_WIDTH_PX) continue
      const yStart = yAxis.convertToPixel(bucket.priceStart)
      const yEnd = yAxis.convertToPixel(bucket.priceEnd)
      const rowPx = Math.abs(yEnd - yStart)
      if (rowPx < MIN_ROW_HEIGHT_PX) continue
      const top = Math.min(yStart, yEnd)
      const height = Math.max(1, rowPx - 1)
      const inVA = bucket.index >= profile.valIndex && bucket.index <= profile.vahIndex
      drawn++

      if (cfg.splitBuySell && bucket.buy + bucket.sell > 0) {
        const bs = bucket.buy + bucket.sell
        const buyW = width * (bucket.buy / bs)
        const sellW = width * (bucket.sell / bs)
        const buyColor = inVA ? COLOR_BUY_IN : COLOR_BUY_OUT
        const sellColor = inVA ? COLOR_SELL_IN : COLOR_SELL_OUT
        drawBar(ctx, anchorX, buyW, top, height, cfg.align, buyColor)
        drawBar(ctx, anchorX, sellW, top, height, cfg.align, sellColor, buyW)
      } else {
        drawBar(ctx, anchorX, width, top, height, cfg.align, inVA ? COLOR_BAR_IN : COLOR_BAR_OUT)
      }
    }
  }

  // Границы зоны стоимости.
  if (cfg.showValueAreaLines && !cfg.showOnlyPoc) {
    ctx.save()
    ctx.strokeStyle = COLOR_VA
    ctx.lineWidth = 1
    ctx.setLineDash([3, 3])
    for (const value of [profile.vah, profile.val]) {
      const y = Math.round(yAxis.convertToPixel(value)) + 0.5
      ctx.beginPath()
      ctx.moveTo(xLeft, y)
      ctx.lineTo(xRight, y)
      ctx.stroke()
    }
    ctx.restore()
  }

  // POC — самая объёмная цена дня.
  const pocY = Math.round(yAxis.convertToPixel(profile.pocPrice)) + 0.5
  ctx.save()
  ctx.strokeStyle = COLOR_POC
  ctx.lineWidth = cfg.showOnlyPoc ? 2 : 1.5
  ctx.beginPath()
  ctx.moveTo(xLeft, pocY)
  ctx.lineTo(xRight, pocY)
  ctx.stroke()
  ctx.restore()
}

// --- Индикатор -------------------------------------------------------------
// Профиль рисуется собственным draw-колбэком поверх свечей; фигур у индикатора
// нет (calc отдаёт null-значения и не влияет на диапазон ценовой оси).
registerIndicator<null>({
  name: VOLUME_PROFILE_INDICATOR_NAME,
  shortName: 'Профиль объёма',
  series: 'normal',
  zLevel: 1,
  // Высокая точность, чтобы через Math.min(indicatorPrecision, pricePrecision)
  // не понизить точность тиков ценовой оси свечного графика. Фигур у индикатора
  // нет, на форматирование своих значений это не влияет.
  precision: 8,
  figures: [],
  calc: dataList => dataList.map(() => null),
  createTooltipDataSource: () => ({ name: '', calcParamsText: '', features: [], legends: [] }),
  draw: ({ ctx, chart, yAxis }) => {
    const dataList = chart.getDataList()
    if (dataList.length === 0) return true
    ensureDayIndex(dataList)
    if (dayEntries.length === 0) return true

    const precision = chart.getSymbol()?.pricePrecision ?? 2
    const tick = inferTick(dataList, precision)
    const range = chart.getVisibleRange()
    const from = Math.max(0, range.realFrom)
    const to = Math.min(dataList.length - 1, range.realTo - 1)

    // Отбираем видимые дни и рисуем не больше MAX_VISIBLE_DAYS (правые, т.е.
    // самые свежие — ближе к текущему времени).
    const visible: DayEntry[] = []
    for (const day of dayEntries) {
      if (day.endIndex < from || day.startIndex > to) continue
      visible.push(day)
    }
    const slice = visible.length > MAX_VISIBLE_DAYS ? visible.slice(-MAX_VISIBLE_DAYS) : visible

    for (const day of slice) {
      const pixelHeight = Math.abs(yAxis.convertToPixel(day.high) - yAxis.convertToPixel(day.low))
      const rowSize = resolveRowSize(day, tick, pixelHeight)
      drawDayProfile(ctx, chart, yAxis, dataList, day, rowSize)
    }
    return true
  },
})

// --- Наведение (тултип) ----------------------------------------------------
export interface VpBarHit {
  key: string
  price: number
  total: number
  pct: number
  buy: number
  sell: number
  hasBuySell: boolean
  isPoc: boolean
  inValueArea: boolean
}

function buildHit(profile: DayProfile, bucket: VpBucket, key: string): VpBarHit {
  return {
    key,
    price: (bucket.priceStart + bucket.priceEnd) / 2,
    total: bucket.total,
    pct: profile.totalVolume > 0 ? bucket.total / profile.totalVolume : 0,
    buy: bucket.buy,
    sell: bucket.sell,
    hasBuySell: profile.hasBuySell,
    isPoc: bucket.index === profile.pocIndex,
    inValueArea: bucket.index >= profile.valIndex && bucket.index <= profile.vahIndex,
  }
}

// Хит-тест профиля под курсором. Возвращает данные бара, если курсор на баре
// (или рядом с линией POC).
export function hitTestVolumeProfile(chart: Chart, x: number, y: number): VpBarHit | null {
  const dataList = chart.getDataList()
  if (dataList.length === 0) return null
  ensureDayIndex(dataList)
  if (dayEntries.length === 0) return null

  const point = (chart.convertFromPixel([{ x, y }], { paneId: VOLUME_PROFILE_PANE_ID }) as Array<{ dataIndex?: number; value?: number }>)[0]
  const dataIndex = point?.dataIndex != null ? Math.round(point.dataIndex) : null
  const price = point?.value
  if (dataIndex == null || price == null || dataIndex < 0 || dataIndex >= dataList.length) return null

  const day = findDay(dataIndex)
  if (day == null) return null

  const precision = chart.getSymbol()?.pricePrecision ?? 2
  const tick = inferTick(dataList, precision)
  const pixelHeight = Math.abs(yOfValue(chart, day.high) - yOfValue(chart, day.low))
  const rowSize = resolveRowSize(day, tick, pixelHeight)
  if (!(rowSize > 0)) return null

  const profile = getProfile(dataList, day, rowSize)
  if (profile.buckets.length === 0) return null

  const bucket = profile.byIndex.get(bucketIndex(price, rowSize))
  if (bucket != null && !volumeProfileConfig.showOnlyPoc) {
    const { xLeft, xRight } = dayEdges(chart, day)
    const dayWidth = xRight - xLeft
    // Строка должна быть реально нарисована (совпадаем с условиями draw).
    const rowPx = Math.abs(yOfValue(chart, bucket.priceEnd) - yOfValue(chart, bucket.priceStart))
    if (dayWidth >= MIN_DAY_WIDTH_PX && rowPx >= MIN_ROW_HEIGHT_PX) {
      const width = (bucket.total / profile.maxTotal) * dayWidth * MAX_BAR_WIDTH_FRACTION
      const anchorX = volumeProfileConfig.align === 'left' ? xLeft : xRight
      const lo = Math.min(anchorX, anchorX + (volumeProfileConfig.align === 'left' ? width : -width))
      const hi = Math.max(anchorX, anchorX + (volumeProfileConfig.align === 'left' ? width : -width))
      if (width >= MIN_BAR_WIDTH_PX && x >= lo && x <= hi) {
        return buildHit(profile, bucket, `${day.dayKey}|${bucket.index}`)
      }
    }
  }

  // Курсор рядом с линией POC — показываем данные POC-строки.
  const pocBucket = profile.byIndex.get(profile.pocIndex)
  if (pocBucket != null) {
    const pocY = yOfValue(chart, profile.pocPrice)
    if (Math.abs(y - pocY) <= 4) {
      return buildHit(profile, pocBucket, `${day.dayKey}|poc`)
    }
  }
  return null
}
