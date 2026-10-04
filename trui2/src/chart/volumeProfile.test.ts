import { computeVolumeProfile, computeDayProfilesForTest, resetVolumeProfileCache, volumeProfileConfig } from './volumeProfile'
import { SuperCandle } from '../data/index'

function bar(partial: Partial<SuperCandle> & { timestamp: number; low: number; high: number }): SuperCandle {
  return {
    open: partial.low,
    close: partial.high,
    volume: 0,
    ...partial,
  } as SuperCandle
}

// Таймстамп в 10:00 МСК (UTC+3) для дня с месяце-днём d (2026-01).
function mskTs(day: number, hourMsk = 10): number {
  return Date.UTC(2026, 0, day, hourMsk - 3, 0, 0)
}

describe('computeVolumeProfile', () => {
  it('раскладывает объём бара по ценовым строкам и считает POC/VA', () => {
    const bars = [bar({ timestamp: 1, low: 10, high: 10.5, volume: 100 })]
    const profile = computeVolumeProfile(bars, '2026-01-05', 0.5, 70)

    expect(profile.buckets).toHaveLength(2)
    expect(profile.buckets[0].total).toBeCloseTo(50)
    expect(profile.buckets[1].total).toBeCloseTo(50)
    expect(profile.totalVolume).toBeCloseTo(100)

    // POC — первая из равных по объёму строк (нижняя).
    expect(profile.pocIndex).toBe(Math.floor(10 / 0.5))
    expect(profile.pocPrice).toBeCloseTo(10.25)

    // 70% при двух строках по 50% покрываются обеими → VAL/VAH по краям дня.
    expect(profile.valIndex).toBe(Math.floor(10 / 0.5))
    expect(profile.vahIndex).toBe(Math.floor(10.5 / 0.5))
    expect(profile.val).toBeCloseTo(10)
    expect(profile.vah).toBeCloseTo(11)
    expect(profile.hasBuySell).toBe(false)
  })

  it('сохраняет разбивку покупки/продажи', () => {
    const bars = [bar({ timestamp: 1, low: 10, high: 10, volume: 100, volume_b: 60, volume_s: 40 })]
    const profile = computeVolumeProfile(bars, '2026-01-05', 0.5, 70)

    expect(profile.buckets).toHaveLength(1)
    expect(profile.buckets[0].buy).toBeCloseTo(60)
    expect(profile.buckets[0].sell).toBeCloseTo(40)
    expect(profile.hasBuySell).toBe(true)
  })

  it('расширяет зону стоимости от POC к более объёмному соседу', () => {
    const bars = [
      bar({ timestamp: 1, low: 10, high: 10, volume: 10 }),
      bar({ timestamp: 2, low: 10.5, high: 10.5, volume: 60 }),
      bar({ timestamp: 3, low: 11, high: 11, volume: 30 }),
    ]
    const profile = computeVolumeProfile(bars, '2026-01-05', 0.5, 70)

    expect(profile.pocIndex).toBe(Math.floor(10.5 / 0.5))
    // POC=60%, нужно ещё 10% до 70% — берём более объёмного соседа сверху (30%).
    expect(profile.valIndex).toBe(Math.floor(10.5 / 0.5))
    expect(profile.vahIndex).toBe(Math.floor(11 / 0.5))
    expect(profile.val).toBeCloseTo(10.5)
    expect(profile.vah).toBeCloseTo(11.5)
  })

  it('при 100% включает все строки', () => {
    const bars = [
      bar({ timestamp: 1, low: 10, high: 10, volume: 10 }),
      bar({ timestamp: 2, low: 10.5, high: 10.5, volume: 60 }),
      bar({ timestamp: 3, low: 11, high: 11, volume: 30 }),
    ]
    const profile = computeVolumeProfile(bars, '2026-01-05', 0.5, 100)

    expect(profile.val).toBeCloseTo(10)
    expect(profile.vah).toBeCloseTo(11.5)
  })

  it('на пустом списке не падает', () => {
    const profile = computeVolumeProfile([], '2026-01-05', 0.5, 70)
    expect(profile.buckets).toHaveLength(0)
    expect(profile.totalVolume).toBe(0)
    expect(profile.pocPrice).toBe(0)
  })

  it('при неположительном rowSize возвращает пустой профиль без деления на ноль', () => {
    const bars = [bar({ timestamp: 1, low: 10, high: 11, volume: 100 })]
    for (const bad of [0, -1, NaN]) {
      const profile = computeVolumeProfile(bars, '2026-01-05', bad, 70)
      expect(profile.buckets).toHaveLength(0)
      expect(profile.totalVolume).toBe(0)
    }
  })

  it('цена ровно на границе строки попадает в старшую строку (устойчивость к округлению)', () => {
    // 0.3 / 0.1 = 2.9999... — эпсилон должен отнести её в строку 3.
    const bars = [bar({ timestamp: 1, low: 0.3, high: 0.3, volume: 10 })]
    const profile = computeVolumeProfile(bars, '2026-01-05', 0.1, 70)
    expect(profile.buckets).toHaveLength(1)
    expect(profile.buckets[0].index).toBe(3)
    expect(profile.pocPrice).toBeCloseTo(0.35)
  })

  it('цена чуть ниже границы остаётся в младшей строке', () => {
    const bars = [bar({ timestamp: 1, low: 0.29, high: 0.29, volume: 10 })]
    const profile = computeVolumeProfile(bars, '2026-01-05', 0.1, 70)
    expect(profile.buckets[0].index).toBe(2)
  })

  it('цена ровно на границе крупного уровня уходит в старшую строку (относительный эпсилон)', () => {
    // 100/10 = 10 ровно; float-шум не должен сдвигать в строку 9.
    const bars = [bar({ timestamp: 1, low: 100, high: 100, volume: 10 })]
    const profile = computeVolumeProfile(bars, '2026-01-05', 10, 70)
    expect(profile.buckets[0].index).toBe(10)
  })

  it('цена заметно ниже границы крупного уровня остаётся в младшей строке', () => {
    // Чуть ниже 100 (не float-шум): 99.999/10 = 9.9999 → строка 9.
    const bars = [bar({ timestamp: 1, low: 99.999, high: 99.999, volume: 10 })]
    const profile = computeVolumeProfile(bars, '2026-01-05', 10, 70)
    expect(profile.buckets[0].index).toBe(9)
  })
})

describe('кэш профилей дней', () => {
  // Два торговых дня; в первом распределение такое, что при 70% и 90% границы
  // зоны стоимости заведомо разные (POC 50%, затем 30%, затем 20%).
  const day1 = [
    bar({ timestamp: mskTs(5, 10), low: 10, high: 10, volume: 50 }),
    bar({ timestamp: mskTs(5, 11), low: 10.5, high: 10.5, volume: 30 }),
    bar({ timestamp: mskTs(5, 12), low: 11, high: 11, volume: 20 }),
  ]
  const day2 = [
    bar({ timestamp: mskTs(6, 10), low: 20, high: 20, volume: 100 }),
  ]
  const dataList = [...day1, ...day2] as SuperCandle[]

  beforeEach(() => {
    resetVolumeProfileCache()
  })

  afterEach(() => {
    volumeProfileConfig.valueAreaPct = 70
    resetVolumeProfileCache()
  })

  it('строит по профилю на каждый день', () => {
    const profiles = computeDayProfilesForTest(dataList, 0.5)
    expect(profiles).toHaveLength(2)
    expect(profiles.map(p => p.dayKey)).toEqual(['2026-01-05', '2026-01-06'])
  })

  it('смена valueAreaPct пересчитывает профиль (инвалидация кэша)', () => {
    volumeProfileConfig.valueAreaPct = 70
    const va70 = computeDayProfilesForTest(dataList, 0.5)[0]
    // 70%: POC(50) + 30 = 80 ≥ 70 → верхняя граница на строке 10.5..11.
    expect(va70.vah).toBeCloseTo(11)
    expect(va70.val).toBeCloseTo(10)

    volumeProfileConfig.valueAreaPct = 90
    const va90 = computeDayProfilesForTest(dataList, 0.5)[0]
    // 90%: 50+30=80 < 90 → добавляем строку 11 (20) → VAH=11.5.
    expect(va90.vah).toBeCloseTo(11.5)

    // Ключевое: границы действительно изменились, а не взялись из кэша.
    expect(va90.vah).not.toBeCloseTo(va70.vah)
  })

  it('возврат к прежнему valueAreaPct берёт корректный профиль из кэша', () => {
    volumeProfileConfig.valueAreaPct = 90
    const va90 = computeDayProfilesForTest(dataList, 0.5)[0].vah
    volumeProfileConfig.valueAreaPct = 70
    const va70 = computeDayProfilesForTest(dataList, 0.5)[0].vah
    volumeProfileConfig.valueAreaPct = 90
    const va90again = computeDayProfilesForTest(dataList, 0.5)[0].vah

    expect(va70).toBeCloseTo(11)
    expect(va90).toBeCloseTo(11.5)
    expect(va90again).toBeCloseTo(va90)
  })
})
