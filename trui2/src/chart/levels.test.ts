import {
  computeLevelStatuses,
  loadFixedLevels,
  loadLevelsConfig,
  saveFixedLevels,
  saveLevelsConfig,
  syncFixedLevels,
  levelLabel,
  defaultLevelColor,
  DEFAULT_LEVELS_CONFIG,
  FixedLevel,
  LevelsConfig,
  LevelExtendData,
} from './levels'
import { nextPaletteColor } from './LevelsManager'
import { Chart, KLineData } from 'klinecharts'

function bar(timestamp: number, close: number, high: number, low: number): KLineData {
  return { timestamp, open: close, high, low, close, volume: 1 }
}

function level(partial: Partial<FixedLevel> = {}): FixedLevel {
  return {
    id: 'l1',
    ticker: 'ROSN',
    type: 'POC',
    price: 100,
    startTimestamp: 0,
    endTimestamp: 0,
    visible: true,
    createdAt: 0,
    ...partial,
  }
}

const HOUR = 3600000

describe('computeLevelStatuses', () => {
  it('active — цена не подходила к уровню после дня-источника', () => {
    const data = [bar(HOUR, 100.2, 100.25, 100.15)]
    const statuses = computeLevelStatuses([level()], data, { autoExtend: true, hideOnBreakout: false, breakoutPct: 0.3 })
    expect(statuses.get('l1')).toBe('active')
  })

  it('tested — цена коснулась уровня, но пробоя не было', () => {
    const data = [bar(HOUR, 100.1, 100.1, 99.9)]
    const statuses = computeLevelStatuses([level()], data, { autoExtend: true, hideOnBreakout: false, breakoutPct: 0.3 })
    expect(statuses.get('l1')).toBe('tested')
  })

  it('broken — часовая свеча закрылась выше уровня на порог', () => {
    const data = [bar(HOUR, 101, 101, 100.5)]
    const statuses = computeLevelStatuses([level()], data, { autoExtend: true, hideOnBreakout: false, breakoutPct: 0.3 })
    expect(statuses.get('l1')).toBe('broken')
  })

  it('broken — закрытие ниже уровня на порог', () => {
    const data = [bar(HOUR, 99, 99.5, 99)]
    const statuses = computeLevelStatuses([level()], data, { autoExtend: true, hideOnBreakout: false, breakoutPct: 0.3 })
    expect(statuses.get('l1')).toBe('broken')
  })

  it('свечи дня-источника не участвуют в оценке статуса', () => {
    // Кросс уровня внутри дня-источника (timestamp 0), торговля после идёт выше.
    const data = [
      bar(0, 100, 100.5, 99.5),
      bar(HOUR, 102, 102, 101.5),
    ]
    const statuses = computeLevelStatuses([level()], data, { autoExtend: true, hideOnBreakout: false, breakoutPct: 5 })
    // 102 находится в пределах 5% (до 105), а касания не было → active.
    expect(statuses.get('l1')).toBe('active')
  })

  it('несколько часовых свечей сливаются: важна последняя цена часа', () => {
    // Два бара в один час: первый задевает уровень, второй закрывается далеко.
    const data = [bar(HOUR, 100, 100.1, 99.95), bar(HOUR + 60000, 120, 120, 119)]
    const statuses = computeLevelStatuses([level()], data, { autoExtend: true, hideOnBreakout: false, breakoutPct: 0.3 })
    // Закрытие часа 120 > 100.3 → broken (касание первого бара не спасает).
    expect(statuses.get('l1')).toBe('broken')
  })
})

describe('хранение уровней', () => {
  afterEach(() => {
    localStorage.clear()
  })

  it('сохраняет и читает уровни (JSON round-trip)', () => {
    const levels = [level({ id: 'a', color: '#FF0000' }), level({ id: 'b', type: 'VAL', price: 99.5, endTimestamp: null })]
    saveFixedLevels('ROSN', levels)
    expect(loadFixedLevels('ROSN')).toEqual(levels)
  })

  it('пустое хранилище даёт пустой список', () => {
    expect(loadFixedLevels('NOPE')).toEqual([])
  })

  it('конфиг читается с дефолтами и сохраняется', () => {
    expect(loadLevelsConfig()).toEqual(DEFAULT_LEVELS_CONFIG)
    saveLevelsConfig({ autoExtend: false, hideOnBreakout: true, breakoutPct: 1.5 })
    expect(loadLevelsConfig()).toEqual({ autoExtend: false, hideOnBreakout: true, breakoutPct: 1.5 })
  })

  it('отсеивает битые записи и невозможный endTimestamp', () => {
    localStorage.setItem('trui.levels.v1.ROSN', JSON.stringify([
      level({ id: 'ok' }),
      { id: 'bad-price', ticker: 'ROSN', type: 'POC', price: 'x', startTimestamp: 0, endTimestamp: null },
      { id: 'bad-type', ticker: 'ROSN', type: 'NOPE', price: 1, startTimestamp: 0, endTimestamp: null },
      { id: 'bad-end', ticker: 'ROSN', type: 'POC', price: 1, startTimestamp: 0, endTimestamp: 'x' },
    ]))
    expect(loadFixedLevels('ROSN').map(l => l.id)).toEqual(['ok'])
  })

  it('сохранение клампит порог пробоя в диапазон 0..50', () => {
    saveLevelsConfig({ autoExtend: true, hideOnBreakout: true, breakoutPct: 999 })
    expect(loadLevelsConfig().breakoutPct).toBe(50)
    saveLevelsConfig({ autoExtend: true, hideOnBreakout: true, breakoutPct: -5 })
    expect(loadLevelsConfig().breakoutPct).toBe(0)
  })
})

describe('levelLabel и цвета', () => {
  const chart = { getSymbol: () => ({ pricePrecision: 2 }) } as unknown as Chart

  it('формирует подпись dPOC с ценой и датой старта', () => {
    // 2026-10-05 12:00 МСК.
    const ts = Date.UTC(2026, 9, 5, 9, 0, 0)
    expect(levelLabel(level({ type: 'POC', price: 124.5, startTimestamp: ts }), chart))
      .toBe('dPOC 124.50 ₽ (05.10.2026)')
    expect(levelLabel(level({ type: 'MANUAL', price: 100, startTimestamp: ts }), chart))
      .toBe('Level 100.00 ₽ (05.10.2026)')
  })

  it('цвета по типу: POC золотой, VAH/VAL серо-синие', () => {
    expect(defaultLevelColor('POC')).toBe('#FFD700')
    expect(defaultLevelColor('VAH')).toBe('#90A4AE')
    expect(defaultLevelColor('VAL')).toBe('#90A4AE')
  })

  it('nextPaletteColor идёт по кругу палитры', () => {
    // gold (index 0) -> серо-синий (index 1).
    expect(nextPaletteColor(level({ type: 'POC' }))).toBe('#90A4AE')
    expect(nextPaletteColor(level({ type: 'MANUAL', color: '#EF5350' }))).toBe('#FFD700')
  })
})

describe('syncFixedLevels', () => {
  interface FakeOverlay {
    id: string
    name: string
    extendData: LevelExtendData
    points: Array<{ timestamp: number; value: number }>
    visible: boolean
  }

  function fakeChart() {
    const overlays: FakeOverlay[] = []
    const calls = { create: 0, override: 0, remove: 0 }
    // Реальный klinecharts глубоко клонирует extendData при override — эмулируем,
    // иначе тест на «не шлёт override» был бы ложноположительным.
    const deepClone = <T,>(value: T): T => JSON.parse(JSON.stringify(value)) as T
    const chart = {
      getOverlays: () => overlays,
      createOverlay: (c: { id: string; name: string; extendData: LevelExtendData; points: FakeOverlay['points'] }) => {
        calls.create++
        if (overlays.some(o => o.id === c.id)) return c.id
        overlays.push({ id: c.id, name: c.name, extendData: deepClone(c.extendData), points: c.points, visible: true })
        return c.id
      },
      overrideOverlay: (c: { id: string; extendData: LevelExtendData; points: FakeOverlay['points'] }) => {
        calls.override++
        const o = overlays.find(o => o.id === c.id)
        if (o != null) {
          o.extendData = deepClone(c.extendData)
          o.points = c.points
        }
        return true
      },
      removeOverlay: (f: { id: string }) => {
        calls.remove++
        const i = overlays.findIndex(o => o.id === f.id)
        if (i >= 0) overlays.splice(i, 1)
        return true
      },
    }
    return { chart: chart as unknown as Chart, overlays, calls }
  }

  const cfg: LevelsConfig = { autoExtend: true, hideOnBreakout: false, breakoutPct: 0.3 }

  it('создаёт оверлей для видимого уровня и пропускает скрытый', () => {
    const { chart, overlays, calls } = fakeChart()
    syncFixedLevels(chart, [level({ id: 'a' }), level({ id: 'b', visible: false })], cfg, [])
    expect(overlays.map(o => o.id)).toEqual(['a'])
    expect(calls.create).toBe(1)
  })

  it('не шлёт override, если уровень и статус не менялись', () => {
    const { chart, calls } = fakeChart()
    const levels = [level({ id: 'a' })]
    syncFixedLevels(chart, levels, cfg, [])
    syncFixedLevels(chart, levels, cfg, [])
    expect(calls.create).toBe(1)
    expect(calls.override).toBe(0)
  })

  it('не шлёт override при новом объекте уровня с теми же значениями (sig)', () => {
    const { chart, calls } = fakeChart()
    syncFixedLevels(chart, [level({ id: 'a', price: 100, color: '#FF0000' })], cfg, [])
    // Новый объект (как после любого setState), но значения те же.
    syncFixedLevels(chart, [level({ id: 'a', price: 100, color: '#FF0000' })], cfg, [])
    expect(calls.override).toBe(0)
  })

  it('шлёт override при смене цвета уровня', () => {
    const { chart, calls } = fakeChart()
    syncFixedLevels(chart, [level({ id: 'a', color: '#FF0000' })], cfg, [])
    syncFixedLevels(chart, [level({ id: 'a', color: '#00FF00' })], cfg, [])
    expect(calls.override).toBe(1)
  })

  it('шлёт override при смене статуса (данные изменились)', () => {
    const { chart, calls } = fakeChart()
    syncFixedLevels(chart, [level({ id: 'a', price: 100 })], cfg, [])
    // Часовая свеча закрылась выше 100.3 → статус broken.
    syncFixedLevels(chart, [level({ id: 'a', price: 100 })], cfg, [bar(HOUR, 101, 101, 100.5)])
    expect(calls.override).toBe(1)
  })

  it('шлёт override при смене только конфига (breakoutPct)', () => {
    const { chart, calls } = fakeChart()
    syncFixedLevels(chart, [level({ id: 'a' })], cfg, [])
    syncFixedLevels(chart, [level({ id: 'a' })], { ...cfg, breakoutPct: 1 }, [])
    expect(calls.override).toBe(1)
  })

  it('шлёт override при смене цены уровня', () => {
    const { chart, calls } = fakeChart()
    syncFixedLevels(chart, [level({ id: 'a', price: 100 })], cfg, [])
    syncFixedLevels(chart, [level({ id: 'a', price: 101 })], cfg, [])
    expect(calls.override).toBe(1)
  })

  it('удаляет оверлеи пропавших уровней', () => {
    const { chart, overlays } = fakeChart()
    syncFixedLevels(chart, [level({ id: 'a' })], cfg, [])
    syncFixedLevels(chart, [level({ id: 'b' })], cfg, [])
    expect(overlays.map(o => o.id)).toEqual(['b'])
  })

  it('hideOnBreakout скрывает пробитый уровень', () => {
    const { chart, overlays } = fakeChart()
    const data = [{ timestamp: HOUR, open: 100, high: 105, low: 99, close: 105, volume: 1 }]
    syncFixedLevels(chart, [level({ id: 'a', price: 100 })], { ...cfg, hideOnBreakout: true }, data)
    expect(overlays).toHaveLength(0)
    // Без опции — уровень остаётся (и помечается пробитым).
    const { chart: c2, overlays: o2 } = fakeChart()
    syncFixedLevels(c2, [level({ id: 'a', price: 100 })], cfg, data)
    expect(o2).toHaveLength(1)
    expect((o2[0].extendData as LevelExtendData).status).toBe('broken')
  })
})
