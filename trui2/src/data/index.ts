import { KLineData } from "klinecharts"
import { createClient } from "@clickhouse/client-web";

export interface SuperCandle extends KLineData {
  volume_b?: number;
  volume_s?: number;
  oi_open?: number;
  oi_high?: number;
  oi_low?: number;
  oi_close?: number;
}

interface SuperCandleRow {
	timeslot: string;
	open: number;
	high: number;
	low: number;
	close: number;
	volume: number;
	volume_s?: number;
	volume_b?: number;
	oi_open?: number;
	oi_high?: number;
	oi_low?: number;
	oi_close?: number;
}

export interface FutOIData {
  timestamp: number;
  fiz_long: number;
  fiz_short: number;
  total_long: number;
  total_short: number;
  price: number;
}

export interface FizOIResult {
  data: FutOIData[];
  // true, если данные пришли из iss_openpositions (только дневные срезы).
  isDaily: boolean;
}

export const IntervalType = {
	Minute: "1m",
	FiveMinutes: "5m",
	Hour: "hour",
	Day: "day",
	Week: "week",
	Month: "month"
}

const clickhouse = createClient({
  url: "http://localhost:3000",
  application: "trui",
  database: "tr",
  request_timeout: 3000,
})

export async function fetchSuperCandles(ticker: string, till: Date, interval: string): Promise<SuperCandle[]> {
  const range = Math.max(10*intervalMs(interval), 3*intervalMs(IntervalType.Day)) // грузим минимум 3 дня, чтобы не застрять в выходных
  const from = new Date(till.getTime() - 6000*intervalMs(interval))

  let rows: SuperCandleRow[] = []

  for (let table of ['tr.super_eq', 'tr.super_fo', 'tr.super_fx']) {
    let query = `
    select toStartOfInterval(time, interval '${interval2sql(interval)}') timeslot,
           argMin(pr_open, time) open,
           max(pr_high) high,
           min(pr_low) low,
           argMax(pr_close, time) close,
           sum(vol) volume,
           sum(vol_b) volume_b,
           sum(vol_s) volume_s`

    // Добавляем поля открытого интереса только для фьючерсов
    if (table === 'tr.super_fo') {
      query += `,
           argMin(oi_open, time) oi_open,
           max(oi_high) oi_high,
           min(oi_low) oi_low,
           argMax(oi_close, time) oi_close`
    }

    query += `
    from ${table}
    where secid='${ticker}' 
      and time >= parseDateTimeBestEffort('${from.toISOString()}')
      and time < parseDateTimeBestEffort('${till.toISOString()}')
      and pr_open > 0 -- filter out empty rows
    group by timeslot
    order by timeslot asc`

    const res = await clickhouse.query({
      query,
      format: "JSONEachRow",
    })  
    rows = await res.json()
    if (rows.length > 0) {
      break
    }
  }
  
  return rows.map(x => {
    const candle: SuperCandle = {
      timestamp: new Date(x.timeslot).getTime(),
      open: Number(x.open),
      high: Number(x.high),
      low: Number(x.low),
      close: Number(x.close),
      volume: Number(x.volume),
      volume_b: Number(x.volume_b),
      volume_s: Number(x.volume_s),
    }
    
    // Добавляем данные открытого интереса, если они есть
    if (x.oi_open !== undefined) {
      candle.oi_open = Number(x.oi_open)
    }
    if (x.oi_high !== undefined) {
      candle.oi_high = Number(x.oi_high)
    }
    if (x.oi_low !== undefined) {
      candle.oi_low = Number(x.oi_low)
    }
    if (x.oi_close !== undefined) {
      candle.oi_close = Number(x.oi_close)
    }
    
    return candle
  })
}

export async function fetchCandles(ticker: string, till: Date, interval: string): Promise<SuperCandle[]> {
  if (notSuperCandles.get(ticker) !== true && interval !== IntervalType.Minute) {
    return fetchSuperCandles(ticker, till, interval)
  }

  const range = Math.max(10*intervalMs(interval), 3*intervalMs(IntervalType.Day)) // грузим минимум 3 дня, чтобы не застрять в выходных
  const from = new Date(till.getTime() - 6000*intervalMs(interval))

  const res = await clickhouse.query({
    query: `
    select toStartOfInterval(time, interval '${interval2sql(interval)}') timeslot,
           argMin(open, time) open,
           max(high) high,
           min(low) low,
           argMax(close, time) close,
           sum(volume) volume
    from tr.candles
    where ticker='${ticker}' 
      and time >= parseDateTimeBestEffort('${from.toISOString()}')
      and time < parseDateTimeBestEffort('${till.toISOString()}')
    group by timeslot
    order by timeslot asc`,
    format: "JSONEachRow",
  })
  const rows: SuperCandleRow[] = await res.json()
  return rows.map(x => {
    return {
      timestamp: new Date(x.timeslot).getTime(),
      open: Number(x.open),
      high: Number(x.high),
      low: Number(x.low),
      close: Number(x.close),
      volume: Number(x.volume),
    }
  })
}

export async function fetchLatestTime(): Promise<Date> {
  const res = await clickhouse.query({
    query: `select max(time) latest from (
              select max(time) time from tr.super_eq
              union all
              select max(time) time from tr.super_fo
              union all
              select max(time) time from tr.super_fx
            )`,
    format: "JSONEachRow",
  })  
  const row: any = await res.json()
  if (!row || row.length === 0 || row[0].latest === null) {
    return new Date()
  }
  return new Date(row[0].latest)
}

const notSuperCandles = new Map([])

// Код акции -> код базового актива фьючерсов в super_fo, когда они не совпадают
// (FUTOI хранит данные по базовому активу фьючерсов, а не по акции)
const STOCK_FUTURES_ASSET_ALIAS: Record<string, string> = {
  ABIO: 'ISKJ',
  BANEP: 'BANE',
  BELU: 'BELUGA',
  BSPBP: 'BSPB',
  GAZP: 'GAZR',
  MTLRP: 'MTLR',
  MTSS: 'MTSI',
  NVTK: 'NOTKM',
  PLZL: 'PLZLM',
  SBER: 'SBRF',
  SBERP: 'SBPR',
  SNGS: 'SNGR',
  SNGSP: 'SNGP',
  TATNP: 'TATP',
  TRNFP: 'TRNF',
}

// Отсекает код месяца + год от secid контракта: "SiZ6" -> "Si", "RNH6" -> "RN".
// Бессрочные фьючерсы (USDRUBF, CNYRUBF и т.п.) не заканчиваются на [месяц][цифра],
// поэтому возвращаются как есть.
function stripFutoiTicker(secid: string): string {
  const m = secid.match(/^(.*)[FGHJKMNQUVXZ]\d$/)
  return m ? m[1] : secid
}

export interface FutOIResolution {
  // Тикер FUTOI (старая нотация, напр. "Si", "IS") для запроса tr.futoi.
  futoiTicker: string | null
  // Полный ASSETCODE (ISS, напр. "USDRUBTOM", "ISKJ") для запроса tr.iss_openpositions.
  asset: string | null
}

// Возвращает тикер FUTOI и ASSETCODE для выбранного инструмента, либо null,
// если по нему нет данных об открытом интересе вовсе.
export async function resolveFutoiTicker(secid: string, isFutures: boolean): Promise<FutOIResolution> {
  if (isFutures) {
    // secid уже является контрактом фьючерса. ASSETCODE достаём из короткого
    // имени ('ISKJ-3.26' -> 'ISKJ', 'Si-12.26' -> 'Si').
    const res = await clickhouse.query({
      query: `select shortname from tr.security_info FINAL where secid='${secid}'`,
      format: "JSONEachRow",
    })
    const rows: any[] = await res.json()
    const shortname: string | undefined = rows[0]?.shortname
    const asset = shortname ? shortname.split('-')[0] : null
    return { futoiTicker: stripFutoiTicker(secid), asset }
  }

  // Код базового актива фьючерса обычно совпадает с secid, но у части
  // переименованных тикеров отличается (SBER -> SBRF, GAZP -> GAZR и т.п.).
  const asset = STOCK_FUTURES_ASSET_ALIAS[secid] ?? secid

  // Контракт ищем в справочнике по короткому имени 'ASSET-M.YY' (например
  // 'DOMRF-6.26', 'SBRF-3.26'). Из secid контракта ('DRM6') вырезаем тикер
  // FUTOI в старой нотации ('DR').
  const res = await clickhouse.query({
    query: `
      select secid
      from tr.security_info FINAL
      where sec_group = 'futures_forts'
        and shortname like '${asset}-%'
      order by is_traded desc
      limit 1
    `,
    format: "JSONEachRow",
  })
  const rows: any[] = await res.json()
  if (rows.length === 0) {
    return { futoiTicker: null, asset: null }
  }
  return { futoiTicker: stripFutoiTicker(rows[0].secid), asset }
}

// Цена для рублёвой оценки берётся по самому ликвидному (max oi_close) контракту
// базового актива на каждый таймслот — FUTOI агрегирует позиции по всем контрактам,
// поэтому это аппроксимация, а не точная оценка стоимости.
function fizOIPriceSubquery(futoiTicker: string, from: Date, till: Date, interval: string): string {
  return `
      select timeslot, argMax(price, oi) as price
      from (
        select toStartOfInterval(time, interval '${interval2sql(interval)}') timeslot,
               argMax(pr_close, time) as price,
               argMax(oi_close, time) as oi
        from tr.super_fo
        where match(secid, '^${futoiTicker}([FGHJKMNQUVXZ][0-9])?$')
          and pr_close > 0
          and time >= parseDateTimeBestEffort('${from.toISOString()}')
          and time < parseDateTimeBestEffort('${till.toISOString()}')
        group by timeslot, secid
      )
      group by timeslot`
}

function mapFizOIRows(rows: any[]): FutOIData[] {
  return rows.map(x => ({
    timestamp: new Date(x.timeslot).getTime(),
    fiz_long: Number(x.fiz_long ?? 0),
    fiz_short: Number(x.fiz_short ?? 0),
    total_long: Number(x.total_long ?? 0),
    total_short: Number(x.total_short ?? 0),
    price: Number(x.price ?? 0),
  }))
}

async function fetchFizOIFromFutoi(futoiTicker: string, from: Date, till: Date, interval: string): Promise<FutOIData[]> {
  const res = await clickhouse.query({
    query: `
    select f.timeslot, f.fiz_long, f.fiz_short, f.total_long, f.total_short, p.price
    from (
      select timeslot,
             sumIf(pos_long, clgroup='FIZ') as fiz_long,
             sumIf(abs(pos_short), clgroup='FIZ') as fiz_short,
             sum(pos_long) as total_long,
             sum(abs(pos_short)) as total_short
      from (
        select toStartOfInterval(time, interval '${interval2sql(interval)}') timeslot,
               clgroup,
               argMax(pos_long, time) as pos_long,
               argMax(pos_short, time) as pos_short
        from tr.futoi
        where ticker='${futoiTicker}'
          and time >= parseDateTimeBestEffort('${from.toISOString()}')
          and time < parseDateTimeBestEffort('${till.toISOString()}')
        group by timeslot, clgroup
      )
      group by timeslot
    ) f
    left join (
      ${fizOIPriceSubquery(futoiTicker, from, till, interval)}
    ) p on p.timeslot = f.timeslot
    order by f.timeslot asc`,
    format: "JSONEachRow",
  })
  const rows: any[] = await res.json()
  return mapFizOIRows(rows)
}

// Для дневных данных iss_openpositions цена должна быть привязана к дневному
// срезу, а не к внутридневному таймслоту: на часовых и более мелких интервалах
// срез ложится в 00:00, а внутридневная цена — в торговые часы, поэтому джойн
// по таймслоту разъезжается.
function fizOIPriceInterval(interval: string): string {
  switch (interval) {
    case IntervalType.Minute:
    case IntervalType.FiveMinutes:
    case IntervalType.Hour:
      return IntervalType.Day
    default:
      return interval
  }
}

async function fetchFizOIFromIssOpenPositions(asset: string, futoiTicker: string, from: Date, till: Date, interval: string): Promise<FutOIData[]> {
  // Данные только дневные (срез за торговый день), поэтому на внутридневных
  // интервалах каждый день даст одну точку в 00:00.
  const priceInterval = fizOIPriceInterval(interval)
  const res = await clickhouse.query({
    query: `
    select f.timeslot, f.fiz_long, f.fiz_short, f.total_long, f.total_short, p.price
    from (
      select timeslot,
             sumIf(open_position_long, clgroup='FIZ') as fiz_long,
             sumIf(open_position_short, clgroup='FIZ') as fiz_short,
             sum(open_position_long) as total_long,
             sum(open_position_short) as total_short
      from (
        select toStartOfInterval(time, interval '${interval2sql(interval)}') timeslot,
               clgroup,
               argMax(open_position_long, time) as open_position_long,
               argMax(open_position_short, time) as open_position_short
        from tr.iss_openpositions
        where asset='${asset}'
          and time >= parseDateTimeBestEffort('${from.toISOString()}')
          and time < parseDateTimeBestEffort('${till.toISOString()}')
        group by timeslot, clgroup
      )
      group by timeslot
    ) f
    left join (
      ${fizOIPriceSubquery(futoiTicker, from, till, priceInterval)}
    ) p on p.timeslot = f.timeslot
    order by f.timeslot asc`,
    format: "JSONEachRow",
  })
  const rows: any[] = await res.json()
  return mapFizOIRows(rows)
}

// Открытый интерес физлиц по фьючерсам. Сначала пробуем tr.futoi (5-минутные
// срезы). Если по активу данных нет — падаем в tr.iss_openpositions (только
// дневные срезы).
export async function fetchFizOI(futoiTicker: string, asset: string | null, till: Date, interval: string): Promise<FizOIResult> {
  const from = new Date(till.getTime() - 6000*intervalMs(interval))

  const data = await fetchFizOIFromFutoi(futoiTicker, from, till, interval)
  if (data.length > 0 || asset == null) {
    return { data, isDaily: false }
  }
  return { data: await fetchFizOIFromIssOpenPositions(asset, futoiTicker, from, till, interval), isDaily: true }
}

export interface Security {
  secid: string
  shortname: string
  name: string
  emitent_title: string
  sec_type: string
  sec_group: string
}

export async function fetchSecurities(): Promise<Security[]> {
  const res = await clickhouse.query({
    query: `select secid, shortname, name, emitent_title, sec_type, sec_group
            from tr.security_info FINAL
            order by secid`,
    format: "JSONEachRow",
  })  
  const rows: any[] = await res.json()
  return rows.map(x => ({
    secid: x.secid ?? '',
    shortname: x.shortname ?? '',
    name: x.name ?? '',
    emitent_title: x.emitent_title ?? '',
    sec_type: x.sec_type ?? '',
    sec_group: x.sec_group ?? '',
  }))
}

function interval2sql(interval: string): string {
  switch (interval) {
    case IntervalType.Minute:
      return "1 minute"
    case IntervalType.FiveMinutes:
      return "5 minute"
    case IntervalType.Hour:
      return "1 hour"
    case IntervalType.Day:
      return "1 day"
    case IntervalType.Week:
      return "7 day"
    case IntervalType.Month:
      return "1 month"
    default:
      throw new Error("Unsupported interval type")
  }
}

function intervalMs(interval: string): number {
  switch (interval) {
    case IntervalType.Minute:
      return 60*1000
    case IntervalType.FiveMinutes:
      return 5*60*1000
    case IntervalType.Hour:
      return 60*60*1000
    case IntervalType.Day:
      return 24*60*60*1000
    case IntervalType.Week:
      return 7*24*60*60*1000
    case IntervalType.Month:
      return 31*24*60*60*1000
    default:
      throw new Error("Unsupported interval type")
  }
}