
import { KLineData } from "klinecharts"
import { createClient } from "@clickhouse/client-web";

export interface SuperCandle extends KLineData {
  volume_b?: number;
  volume_s?: number;
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
    const res = await clickhouse.query({
      query: `
      select toStartOfInterval(time, interval '${interval2sql(interval)}') timeslot,
             argMin(pr_open, time) open,
             max(pr_high) high,
             min(pr_low) low,
             argMax(pr_close, time) close,
             sum(vol) volume,
             sum(vol_b) volume_b,
             sum(vol_s) volume_s
      from ${table}
      where secid='${ticker}' 
        and time >= parseDateTimeBestEffort('${from.toISOString()}')
        and time < parseDateTimeBestEffort('${till.toISOString()}')
        and pr_open > 0 -- filter out empty rows
      group by timeslot
      order by timeslot asc`,
      format: "JSONEachRow",
    })  
    rows = await res.json()
    if (rows.length > 0) {
      break
    }
  }
  
  return rows.map(x => {
    return {
      timestamp: new Date(x.timeslot).getTime(),
      open: Number(x.open),
      high: Number(x.high),
      low: Number(x.low),
      close: Number(x.close),
      volume: Number(x.volume),
      volume_b: Number(x.volume_b),
      volume_s: Number(x.volume_s),
    }
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
    query: "select max(time) latest from tr.super_eq",
    format: "JSONEachRow",
  })  
  const row: any = await res.json()
  if (!row || row.length === 0) {
    return new Date()
  }
  return new Date(row[0].latest)
}

const notSuperCandles = new Map([
  ["IMOEX2", true],
  ["LQDT", true],
])

export async function fetchTickers(): Promise<string[]> {
  const res = await clickhouse.query({
    query: `select distinct ticker from (
                select distinct secid as ticker from tr.super_eq
                union all
                select distinct secid as ticker from tr.super_fo
                union all
                select distinct secid as ticker from tr.super_fx
                union all
                select distinct ticker from tr.candles
            )
            order by ticker`,
    format: "JSONEachRow",
  })  
  const rows: any[] = await res.json()
  return rows.map(x => x.ticker)
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
