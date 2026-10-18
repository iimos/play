import {
  Chart,
  Coordinate,
  KLineData,
  OverlayEvent,
  OverlayFigure,
  registerHotkey,
  registerOverlay,
} from 'klinecharts'
import { realBars, SuperCandle } from '../data/index'

// Модуль фиксации ключевых уровней дневного профиля объёма (POC / VAH / VAL)
// для стратегии PbD (Pullback after Breakout).
//
// Уровень — это горизонтальная линия, привязанная к конкретной ЦЕНЕ. Время
// начала (startTimestamp) задаёт, откуда линия начинается (обычно начало
// торгового дня, зафиксировавшего уровень), а конец — либо край графика
// (Auto-Extend), либо конец исходного дня. Поэтому при смене таймфрейма линия
// остаётся на той же цене (пересчитывается только её пиксельная координата Y).
//
// Состояние уровня (active / tested / broken) НЕ хранится, а вычисляется на
// лету по загруженным свечам: так оно само обновляется в реальном времени и не
// устаревает при переоткрытии графика.

export type LevelType = 'POC' | 'VAH' | 'VAL' | 'MANUAL'
export type LevelStatus = 'active' | 'tested' | 'broken'

// Сериализуемое (JSON.stringify) описание зафиксированного уровня.
export interface FixedLevel {
  id: string
  ticker: string
  type: LevelType
  // Цена уровня (та самая, что не «плывёт» при смене таймфрейма).
  price: number
  // Момент, с которого рисуется линия (обычно начало исходного дня).
  startTimestamp: number
  // Конец исходного дня для исторических уровней. null — линия тянется вправо
  // всегда (ручные уровни и уровни, у которых день неизвестен).
  endTimestamp: number | null
  // День-источник уровня (YYYY-MM-DD, МСК) — для POC/VAH/VAL из профиля.
  sourceDay?: string
  // Пользовательский цвет (задаётся в быстром меню); иначе берётся по типу.
  color?: string
  visible: boolean
  createdAt: number
}

export interface LevelsConfig {
  // Продлевать исторические уровни до текущего дня (правый край графика).
  autoExtend: boolean
  // Автоматически скрывать уровень, если часовая свеча закрылась выше/ниже
  // него на breakoutPct%.
  hideOnBreakout: boolean
  // Порог пробоя в процентах.
  breakoutPct: number
}

export const DEFAULT_LEVELS_CONFIG: LevelsConfig = {
  autoExtend: true,
  hideOnBreakout: false,
  breakoutPct: 0.3,
}

// Приводит произвольный (в т.ч. частичный/битый) конфиг к валидному виду.
export function normalizeLevelsConfig(value: Partial<LevelsConfig> | null | undefined): LevelsConfig {
  const v = value ?? {}
  return {
    autoExtend: v.autoExtend !== false,
    hideOnBreakout: v.hideOnBreakout === true,
    breakoutPct: typeof v.breakoutPct === 'number' && Number.isFinite(v.breakoutPct)
      ? Math.min(50, Math.max(0, v.breakoutPct))
      : DEFAULT_LEVELS_CONFIG.breakoutPct,
  }
}

// Палитра для быстрой смены цвета уровня (кружок в мини-меню и в менеджере).
export const LEVEL_PALETTE = [
  '#FFD700',
  '#90A4AE',
  '#42A5F5',
  '#26A69A',
  '#AB47BC',
  '#FF7043',
  '#EF5350',
]

// Данные, которые оверлей хранит в extendData. Здесь же передаётся вычисленный
// статус и актуальная конфигурация (draw-колбэк не имеет доступа к React-состоянию).
export interface LevelExtendData {
  level: FixedLevel
  status: LevelStatus
  config: LevelsConfig
  // Дешёвая подпись входных данных. klinecharts при override ГЛУБОКО клонирует
  // extendData, поэтому сравнивать level/config по ссылке нельзя — только sig.
  sig: string
}

// Подпись уровня + статуса + настроек: по ней решаем, нужен ли overrideOverlay.
export function levelSignature(level: FixedLevel, status: LevelStatus, config: LevelsConfig): string {
  return [
    level.price,
    level.color ?? '',
    level.visible ? 1 : 0,
    level.startTimestamp,
    level.endTimestamp ?? '',
    level.type,
    status,
    config.autoExtend ? 1 : 0,
    config.hideOnBreakout ? 1 : 0,
    config.breakoutPct,
  ].join('|')
}

export const LEVEL_OVERLAY_NAME = 'fixedLevel'

// --- Цвета и статусы --------------------------------------------------------

const TYPE_COLORS: Record<LevelType, string> = {
  POC: '#FFD700', // золотой — POC
  VAH: '#90A4AE', // серо-синий — границы зоны стоимости
  VAL: '#90A4AE',
  MANUAL: '#42A5F5',
}

// Пробитый уровень — тускло-красный (сплошной): виден, но не спорит с активным.
const COLOR_BROKEN = 'rgba(183, 28, 28, 0.55)'

export function defaultLevelColor(type: LevelType): string {
  return TYPE_COLORS[type]
}

// Дописывает альфу к hex-цвету (#RRGGBB -> #RRGGBBAA).
function withAlpha(color: string, alphaHex: string): string {
  return /^#[0-9a-fA-F]{6}$/.test(color) ? color + alphaHex : color
}

// Итоговый цвет линии: пробитый — тускло-красный, отработанный — полупрозрачный,
// активный — яркий пользовательский/типовой.
function statusColor(status: LevelStatus, base: string): string {
  if (status === 'broken') return COLOR_BROKEN
  if (status === 'tested') return withAlpha(base, '8C')
  return base
}

// --- Хранение (localStorage) ------------------------------------------------

const LEVELS_KEY_PREFIX = 'trui.levels.v1.'
const CONFIG_KEY = 'trui.levels.config.v1'

function storage(): Storage | null {
  try {
    if (typeof window === 'undefined') return null
    return window.localStorage ?? null
  } catch {
    return null
  }
}

function isValidLevel(value: unknown): value is FixedLevel {
  if (value == null || typeof value !== 'object') return false
  const l = value as Record<string, unknown>
  return typeof l.id === 'string'
    && typeof l.ticker === 'string'
    && (l.type === 'POC' || l.type === 'VAH' || l.type === 'VAL' || l.type === 'MANUAL')
    && typeof l.price === 'number' && Number.isFinite(l.price)
    && typeof l.startTimestamp === 'number' && Number.isFinite(l.startTimestamp)
    && (l.endTimestamp === null || (typeof l.endTimestamp === 'number' && Number.isFinite(l.endTimestamp)))
}

export function loadFixedLevels(ticker: string): FixedLevel[] {
  const store = storage()
  if (store == null) return []
  try {
    const raw = store.getItem(LEVELS_KEY_PREFIX + ticker)
    if (raw == null) return []
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.filter(isValidLevel).map(l => ({ ...l, visible: l.visible !== false }))
  } catch {
    return []
  }
}

export function saveFixedLevels(ticker: string, levels: FixedLevel[]): void {
  const store = storage()
  if (store == null) return
  try {
    store.setItem(LEVELS_KEY_PREFIX + ticker, JSON.stringify(levels))
  } catch {
    // Переполнение хранилища не должно ломать UI.
  }
}

export function loadLevelsConfig(): LevelsConfig {
  const store = storage()
  if (store == null) return { ...DEFAULT_LEVELS_CONFIG }
  try {
    const raw = store.getItem(CONFIG_KEY)
    if (raw == null) return { ...DEFAULT_LEVELS_CONFIG }
    return normalizeLevelsConfig(JSON.parse(raw) as Partial<LevelsConfig>)
  } catch {
    return { ...DEFAULT_LEVELS_CONFIG }
  }
}

export function saveLevelsConfig(config: LevelsConfig): void {
  const store = storage()
  if (store == null) return
  try {
    store.setItem(CONFIG_KEY, JSON.stringify(normalizeLevelsConfig(config)))
  } catch {
    // ignore
  }
}

export function newLevelId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return 'lvl_' + crypto.randomUUID()
  }
  return 'lvl_' + Date.now().toString(36) + '_' + Math.random().toString(36).slice(2, 10)
}

// --- Форматирование ---------------------------------------------------------

const mskDayFmt = new Intl.DateTimeFormat('ru-RU', {
  timeZone: 'Europe/Moscow',
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
})

const mskShortDayFmt = new Intl.DateTimeFormat('ru-RU', {
  timeZone: 'Europe/Moscow',
  day: '2-digit',
  month: '2-digit',
})

function formatMskDate(ts: number): string {
  return mskDayFmt.format(new Date(ts))
}

// Экспорт для панели уровней (дата старта уровня).
export function formatLevelDate(ts: number): string {
  return formatMskDate(ts)
}

function formatMskShortDate(ts: number): string {
  return mskShortDayFmt.format(new Date(ts))
}

function pricePrecision(chart: Chart): number {
  return chart.getSymbol()?.pricePrecision ?? 2
}

// Подпись линии у правого края: "dPOC 124.50 ₽ (05.10.2026)".
export function levelLabel(level: FixedLevel, chart: Chart): string {
  const price = level.price.toFixed(pricePrecision(chart))
  const prefix = level.type === 'MANUAL' ? 'Level' : 'd' + level.type
  return `${prefix} ${price} ₽ (${formatMskDate(level.startTimestamp)})`
}

// --- Вычисление статуса -----------------------------------------------------

interface HourBar {
  close: number
  high: number
  low: number
}

// Приводим свечи к часовым: статус уровня по ТЗ оценивается по закрытию часовой
// свечи. На интервалах мельче часа несколько баров сливаются в один (close —
// последний), на дневках/неделях каждый бар и так один. Бары отсортированы.
// Оговорка: на day/week/month «часовых» баров в данных нет, поэтому статус
// считается по закрытию самого бара периода — приближение, приемлемое для PbD.
function toHourlyBars(bars: SuperCandle[]): HourBar[] {
  const out: HourBar[] = []
  let currentKey: number | null = null
  for (const b of bars) {
    if (!Number.isFinite(b.close)) continue
    const key = Math.floor(b.timestamp / 3600000)
    if (key !== currentKey) {
      out.push({ close: b.close, high: b.high, low: b.low })
      currentKey = key
    } else {
      const cur = out[out.length - 1]
      cur.close = b.close
      cur.high = Math.max(cur.high, b.high)
      cur.low = Math.min(cur.low, b.low)
    }
  }
  return out
}

// Статус одного уровня по свечам ПОСЛЕ момента фиксации (для исторического дня
// это фактически следующий день; для сегодняшнего — бары после того, как уровень
// поставили). Так POC дня не считается «отработанным» в момент своего рождения.
//   broken — часовая свеча закрылась выше/ниже цены более чем на breakoutPct%;
//   tested — цена подходила к уровню в пределах допуска, но пробоя не было;
//   active — цены у уровня ещё не было.
function evaluateLevelStatus(level: FixedLevel, hourly: HourBar[], breakoutPct: number): LevelStatus {
  const price = level.price
  const up = price * (1 + breakoutPct / 100)
  const down = price * (1 - breakoutPct / 100)
  const tol = Math.max(price * 0.0005, 1e-6)
  let tested = false
  for (const h of hourly) {
    if (h.close > up || h.close < down) return 'broken'
    if (h.low <= price + tol && h.high >= price - tol) tested = true
  }
  return tested ? 'tested' : 'active'
}

export function computeLevelStatuses(
  levels: FixedLevel[],
  dataList: KLineData[],
  config: LevelsConfig,
): Map<string, LevelStatus> {
  const bars = realBars(dataList)
  const result = new Map<string, LevelStatus>()
  for (const level of levels) {
    const evaluationStart = level.endTimestamp != null ? level.endTimestamp + 1 : level.startTimestamp
    const hourly = toHourlyBars(bars.filter(b => b.timestamp >= evaluationStart))
    result.set(level.id, evaluateLevelStatus(level, hourly, config.breakoutPct))
  }
  return result
}

// --- Синхронизация оверлеев с чартом ---------------------------------------

function xFromTimestamp(chart: Chart, paneId: string, timestamp: number): number {
  const coords = chart.convertToPixel([{ timestamp }], { paneId }) as Array<Partial<Coordinate>>
  const x = coords[0]?.x
  return typeof x === 'number' ? x : NaN
}

// id выделенного сейчас оверлея — нужен горячей клавише Delete.
let selectedLevelId: string | null = null

// Сброс выделения (смена тикера, удаление уровня вне клика по нему).
export function clearSelectedLevel(): void {
  selectedLevelId = null
}

export interface LevelEventHandlers {
  onClick?: (event: OverlayEvent<LevelExtendData>) => void
  onSelected?: (event: OverlayEvent<LevelExtendData>) => void
  onDeselected?: (event: OverlayEvent<LevelExtendData>) => void
  onPressedMoveEnd?: (event: OverlayEvent<LevelExtendData>) => void
  onDelete?: (id: string) => void
}

let handlers: LevelEventHandlers = {}

// ChartType подписывается один раз; колбэки читают refs, поэтому всегда актуальны.
export function setLevelEventHandlers(next: LevelEventHandlers): void {
  handlers = next
}

// Горизонтальный луч уровня: линия от startTimestamp до правого края (или до
// конца исходного дня), подпись у правого края и метка на ценовой шкале.
registerOverlay<LevelExtendData>({
  name: LEVEL_OVERLAY_NAME,
  totalStep: 1,
  needDefaultPointFigure: true,
  needDefaultXAxisFigure: false,
  needDefaultYAxisFigure: false,
  createPointFigures: ({ chart, overlay, coordinates, bounding }) => {
    if (coordinates.length === 0) return []
    const ext = overlay.extendData
    if (ext == null) return []
    const { level, status, config } = ext

    const y = coordinates[0].y
    const startX = coordinates[0].x
    let endX = bounding.width
    if (!config.autoExtend && level.endTimestamp != null) {
      const ex = xFromTimestamp(chart, overlay.paneId, level.endTimestamp)
      if (Number.isFinite(ex)) {
        endX = Math.max(startX, Math.min(bounding.width, ex))
      }
    }
    if (endX < startX) endX = startX

    const base = level.color ?? defaultLevelColor(level.type)
    const color = statusColor(status, base)
    // Active — сплошная яркая; Tested — пунктирная; Broken — тускло-красная сплошная.
    const tested = status === 'tested'

    const figures: OverlayFigure[] = [
      {
        type: 'line',
        attrs: { coordinates: [{ x: startX, y }, { x: endX, y }] },
        styles: {
          style: tested ? 'dashed' : 'solid',
          dashedValue: [4, 4],
          color,
          size: status === 'active' ? 1.5 : 1,
        },
      },
      {
        type: 'text',
        attrs: {
          x: endX - 4,
          y: y - 6,
          text: levelLabel(level, chart),
          align: 'right',
          baseline: 'bottom',
        },
        styles: {
          color: '#FFFFFF',
          backgroundColor: color,
          size: 10,
          paddingLeft: 4,
          paddingRight: 4,
          paddingTop: 2,
          paddingBottom: 2,
          borderRadius: 3,
        },
      },
    ]
    return figures
  },
  createYAxisFigures: ({ chart, overlay, coordinates, bounding }) => {
    if (coordinates.length === 0) return []
    const ext = overlay.extendData
    if (ext == null) return []
    const { level, status } = ext
    const base = level.color ?? defaultLevelColor(level.type)
    const color = statusColor(status, base)
    const text = `${level.type} ${level.price.toFixed(pricePrecision(chart))} ${formatMskShortDate(level.startTimestamp)}`
    return {
      type: 'text',
      attrs: {
        x: bounding.width,
        y: coordinates[0].y,
        text,
        align: 'right',
        baseline: 'middle',
      },
      styles: {
        color: '#FFFFFF',
        backgroundColor: color,
        size: 10,
        paddingLeft: 3,
        paddingRight: 3,
        paddingTop: 1,
        paddingBottom: 1,
      },
    }
  },
  onSelected: event => {
    selectedLevelId = event.overlay.id
    handlers.onSelected?.(event)
  },
  onDeselected: event => {
    if (selectedLevelId === event.overlay.id) selectedLevelId = null
    handlers.onDeselected?.(event)
  },
  onClick: event => {
    handlers.onClick?.(event)
  },
  onRightClick: event => {
    // Только гасим штатное удаление оверлея по ПКМ: удаление уровня делает
    // wrapper-обработчик ChartType (по попаданию в линию), чтобы обновился
    // localStorage и панель уровней, а не «пересоздание» уровня.
    event.preventDefault?.()
  },
  onPressedMoveEnd: event => {
    handlers.onPressedMoveEnd?.(event)
  },
})

// Удаление выбранного уровня клавишей Delete (на macOS основная клавиша —
// Backspace, поэтому слушаем обе).
registerHotkey({
  name: 'deleteFixedLevel',
  keys: ['Delete', 'Backspace'],
  check: () => selectedLevelId != null,
  action: ({ event }) => {
    if (selectedLevelId == null) return
    event.preventDefault()
    const id = selectedLevelId
    selectedLevelId = null
    handlers.onDelete?.(id)
  },
})

// Полная синхронизация оверлеев с состоянием React: создать новые, обновить
// изменившиеся, удалить пропавшие. Возвращает вычисленные статусы — ChartType
// использует их для панели уровней.
export function syncFixedLevels(
  chart: Chart,
  levels: FixedLevel[],
  config: LevelsConfig,
  dataList: KLineData[],
): Map<string, LevelStatus> {
  const statuses = computeLevelStatuses(levels, dataList, config)
  const existing = chart.getOverlays({ name: LEVEL_OVERLAY_NAME })
  const existingById = new Map(existing.map(o => [o.id, o]))
  const keep = new Set<string>()

  for (const level of levels) {
    if (!level.visible) continue
    const status = statuses.get(level.id) ?? 'active'
    if (config.hideOnBreakout && status === 'broken') continue
    keep.add(level.id)

    const points = [{ timestamp: level.startTimestamp, value: level.price }]
    const sig = levelSignature(level, status, config)
    const extendData: LevelExtendData = { level, status, config, sig }
    const current = existingById.get(level.id)
    if (current == null) {
      chart.createOverlay({ name: LEVEL_OVERLAY_NAME, id: level.id, points, extendData })
    } else {
      // Не перерисовываем неизменившийся уровень. Сравниваем по sig, а не по
      // ссылке: klinecharts глубоко клонирует extendData при override.
      const prev = current.extendData as LevelExtendData | undefined
      if (prev?.sig !== sig) {
        chart.overrideOverlay({ id: level.id, points, extendData })
      }
    }
  }

  for (const overlay of existing) {
    if (!keep.has(overlay.id)) {
      chart.removeOverlay({ id: overlay.id })
    }
  }
  // Уровень, которого больше нет среди нарисованных, не должен оставаться
  // выделенным (иначе Delete «выстрелит» в мёртвый/чужой id).
  if (selectedLevelId != null && !keep.has(selectedLevelId)) {
    selectedLevelId = null
  }
  return statuses
}
