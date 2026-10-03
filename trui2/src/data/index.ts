import { KLineData } from "klinecharts"
import { createClient } from "@clickhouse/client-web";

export interface SuperCandle extends KLineData {
  volume_b?: number;
  volume_s?: number;
  val_b?: number;
  val_s?: number;
  pr_vwap_b?: number;
  pr_vwap_s?: number;
  oi_open?: number;
  oi_high?: number;
  oi_low?: number;
  oi_close?: number;
}

// Бар-разрыв (неторговый день): валидный timestamp, но open/close = NaN, поэтому
// свеча не рисуется (canvas игнорирует нечисловые координаты). high/low при этом
// конечны (последняя известная цена), чтобы штатный диапазон y-оси не ломался.
// Такие бары вставляются в серию, чтобы на графике были разрывы; собственные
// расчёты должны пропускать их через эту проверку.
export function isGapBar(c: KLineData | null | undefined): boolean {
  return c != null && !Number.isFinite(c.open)
}

// Прогоняет функцию только по реальным свечам, оставляя на местах баров-разрывов
// null: ядро klinecharts корректно пропускает null в результатах индикаторов
// (`result[i] ?? {}`, отрисовка по isNumber), поэтому это безопасный способ
// считать индикаторы «без разрывов».
export function mapRealBars<D>(dataList: KLineData[], fn: (c: SuperCandle) => D): Array<D | null> {
  return dataList.map(c => (isGapBar(c) ? null : fn(c as SuperCandle)))
}

// Только реальные свечи (без баров-разрывов).
export function realBars(dataList: KLineData[]): SuperCandle[] {
  return dataList.filter(c => !isGapBar(c)) as SuperCandle[]
}

// Одна точка пользовательской SQL-метрики: значение на конкретном таймслоте.
export interface MetricPoint {
  timestamp: number;
  value: number;
}

export interface FutOIData {
  timestamp: number;
  fiz_long: number;
  fiz_short: number;
  total_long: number;
  total_short: number;
  // Оценка стоимости ОИ в ₽ (контракты × стоимость контракта) для физлиц и для
  // всех участников (total). null, если по инструменту нет цены.
  ruble_long: number | null;
  ruble_short: number | null;
  ruble_total_long: number | null;
  ruble_total_short: number | null;
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

// ClickHouse сериализует DateTime в JSONEachRow строкой в таймзоне сервера
// (Europe/Moscow) без суффикса, напр. "2026-01-05 07:00:00"; тип Date (напр.
// toStartOfInterval по месяцу) — как "2026-01-01" без времени. Парсим явно как
// МСК (UTC+3, без DST), а не как локальное время браузера, чтобы timestamp был
// корректным абсолютным моментом независимо от таймзоны клиента.
function parseMskTime(s: string): number {
  const withTime = /\d{2}:\d{2}/.test(s) ? s.replace(' ', 'T') : s + 'T00:00:00'
  return new Date(withTime + '+03:00').getTime()
}

// Группа инструмента (sec_group из security_info) -> таблица суперсвечей.
// Отсутствующие/неизвестные группы возвращают null (не супер-инструмент).
export const SEC_GROUP_TO_TABLE: Record<string, string> = {
  futures_forts: 'tr.super_fo',
  currency_selt: 'tr.super_fx',
  currency_metal: 'tr.super_fx',
  stock_shares: 'tr.super_eq',
  stock_ppif: 'tr.super_eq',
  stock_dr: 'tr.super_eq',
  // Индексы MOEX (IMOEX, RTSI, MOEXBMI, ...): суперсвечи синтезируются из
  // super_eq + index_weights, ключ — indexid (а не secid).
  stock_index: 'tr.super_index',
}

// Резолвит таблицу суперсвечей по типу инструмента. Нужно для SQL-метрик,
// чтобы инжектить выражение в правильную таблицу (колонки у них различаются).
export async function resolveSuperTable(secid: string): Promise<string | null> {
  const group = await resolveSecGroup(secid)
  return group ? SEC_GROUP_TO_TABLE[group] ?? null : null
}

// Группа инструмента (sec_group из security_info): 'stock_shares', 'futures_forts'
// и т.п. Нужна, чтобы понять, участвует ли инструмент в рыночном ранжировании.
export async function resolveSecGroup(secid: string): Promise<string | null> {
  const res = await clickhouse.query({
    query: `select sec_group from tr.security_info where secid='${secid}' limit 1`,
    format: "JSONEachRow",
  })
  const rows: any[] = await res.json()
  const group: string | undefined = rows[0]?.sec_group
  return group ?? null
}

// Кэш идентификаторов индексов (sec_group='stock_index'). Инструменты-индексы
// хранятся в отдельных таблицах (index_candles/super_index) с ключом indexid,
// поэтому загрузку свечей/метрик надо роутить иначе, чем для бумаг.
let indexIds: Set<string> | null = null
let indexIdsPromise: Promise<Set<string> | null> | null = null

export async function isIndexId(secid: string): Promise<boolean> {
  if (indexIds != null) {
    return indexIds.has(secid)
  }
  if (indexIdsPromise == null) {
    indexIdsPromise = clickhouse
      .query({
        query: `select secid from tr.security_info FINAL where sec_group = 'stock_index'`,
        format: "JSONEachRow",
      })
      .then(res => res.json<{ secid: string }>())
      .then(rows => new Set(rows.map(r => r.secid)))
      .catch(err => {
        // Транзиентный сбой справочника не должен ломать загрузку обычных бумаг:
        // считаем инструмент не-индексом и попробуем перезапросить позже.
        console.error('Не удалось получить список индексов:', err)
        return null
      })
  }
  const loaded = await indexIdsPromise
  if (loaded == null) {
    // Сбрасываем неудачный промис, чтобы следующая попытка перезапросила список
    // (сам indexIds при этом остаётся null).
    indexIdsPromise = null
    return false
  }
  indexIds = loaded
  return indexIds.has(secid)
}

// Начало окна загрузки: `bars` свечей интервала `interval` назад от `till`.
// Для начальной загрузки берём 6000 баров, для периодического обновления —
// только несколько последних.
function windowFrom(till: Date, interval: string, bars: number): Date {
  return new Date(till.getTime() - bars*intervalMs(interval))
}

// Начало окна в SQL, выровненное по границе интервала. Без выравнивания самый
// старый бакет попадает в выборку частичным и перезаписывает уже загруженное
// полное значение метрики/OI/доли рынка (Map.set по timestamp), т.к. при доборе
// вперёд результаты мержатся, а не заменяются целиком.
function sqlWindowStart(from: Date, interval: string): string {
  return `toStartOfInterval(parseDateTimeBestEffort('${from.toISOString()}'), interval '${interval2sql(interval)}')`
}

export async function fetchSuperCandles(ticker: string, till: Date, interval: string, bars = 6000): Promise<SuperCandle[]> {
  const from = windowFrom(till, interval, bars)

  let rows: any[] = []

  for (let table of ['tr.super_eq', 'tr.super_fo', 'tr.super_fx']) {
    let query = `
    select toStartOfInterval(time, interval '${interval2sql(interval)}') timeslot,
           argMin(pr_open, time) open,
           max(pr_high) high,
           min(pr_low) low,
           argMax(pr_close, time) close,
           sum(vol) volume,
           sum(vol_b) volume_b,
           sum(vol_s) volume_s,
           sum(val_b) val_b,
           sum(val_s) val_s,
           val_b/nullIf(volume_b,0) pr_vwap_b,
           val_s/nullIf(volume_s,0) pr_vwap_s`

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
      and time >= ${sqlWindowStart(from, interval)}
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
      timestamp: parseMskTime(x.timeslot),
      open: Number(x.open),
      high: Number(x.high),
      low: Number(x.low),
      close: Number(x.close),
      volume: Number(x.volume),
      volume_b: Number(x.volume_b),
      volume_s: Number(x.volume_s),
      val_b: Number(x.val_b),
      val_s: Number(x.val_s),
    }
    if (x.pr_vwap_b != null) {
      candle.pr_vwap_b = Number(x.pr_vwap_b)
    }
    if (x.pr_vwap_s != null) {
      candle.pr_vwap_s = Number(x.pr_vwap_s)
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

// Свечи индекса. Днёвки/недели/месяцы берём из tr.index_candles (interval=24;
// у индексов volume=0, а оборот по бумагам индекса лежит в value, что мы и
// кладём в volume, т.к. для индекса «объём» — это оборот в ₽). Внутридневные
// интервалы — из tr.super_index (5-минутные синтезированные суперсвечи с
// покупками/продажами и метриками заявок/стакана).
export async function fetchIndexCandles(indexid: string, till: Date, interval: string, bars = 6000): Promise<SuperCandle[]> {
  // super_index — 5-минутный, минутного интервала у индексов нет.
  if (interval === IntervalType.Minute) {
    return []
  }
  if (interval === IntervalType.Day || interval === IntervalType.Week || interval === IntervalType.Month) {
    return fetchIndexDailyCandles(indexid, till, interval, bars)
  }
  return fetchIndexSuperCandles(indexid, till, interval, bars)
}

async function fetchIndexDailyCandles(indexid: string, till: Date, interval: string, bars: number): Promise<SuperCandle[]> {
  const from = windowFrom(till, interval, bars)
  const res = await clickhouse.query({
    query: `
    select toStartOfInterval(time, interval '${interval2sql(interval)}') timeslot,
           argMin(open, time) open,
           max(high) high,
           min(low) low,
           argMax(close, time) close,
           sum(value) value
    from tr.index_candles
    where indexid='${indexid}'
      and interval = 24
      and time >= ${sqlWindowStart(from, interval)}
      and time < parseDateTimeBestEffort('${till.toISOString()}')
    group by timeslot
    order by timeslot asc`,
    format: "JSONEachRow",
  })
  const rows: any[] = await res.json()
  return rows.map(x => ({
    timestamp: parseMskTime(x.timeslot),
    open: Number(x.open),
    high: Number(x.high),
    low: Number(x.low),
    close: Number(x.close),
    volume: Number(x.value),
  }))
}

async function fetchIndexSuperCandles(indexid: string, till: Date, interval: string, bars: number): Promise<SuperCandle[]> {
  const from = windowFrom(till, interval, bars)
  const res = await clickhouse.query({
    query: `
    select toStartOfInterval(time, interval '${interval2sql(interval)}') timeslot,
           argMin(pr_open, time) open,
           max(pr_high) high,
           min(pr_low) low,
           argMax(pr_close, time) close,
           sum(val) volume,
           sum(vol_b) volume_b,
           sum(vol_s) volume_s,
           sum(val_b) val_b_sum,
           sum(val_s) val_s_sum,
           sum(pr_vwap_b * val_b) / nullIf(sum(val_b), 0) pr_vwap_b,
           sum(pr_vwap_s * val_s) / nullIf(sum(val_s), 0) pr_vwap_s
    from tr.super_index
    where indexid='${indexid}'
      and time >= ${sqlWindowStart(from, interval)}
      and time < parseDateTimeBestEffort('${till.toISOString()}')
      and pr_open > 0
    group by timeslot
    order by timeslot asc`,
    format: "JSONEachRow",
  })
  const rows: any[] = await res.json()
  return rows.map(x => {
    const candle: SuperCandle = {
      timestamp: parseMskTime(x.timeslot),
      open: Number(x.open),
      high: Number(x.high),
      low: Number(x.low),
      close: Number(x.close),
      // Оборот по бумагам индекса — это «объём индекса».
      volume: Number(x.volume),
      volume_b: Number(x.volume_b),
      volume_s: Number(x.volume_s),
      val_b: Number(x.val_b_sum),
      val_s: Number(x.val_s_sum),
    }
    if (x.pr_vwap_b != null) {
      candle.pr_vwap_b = Number(x.pr_vwap_b)
    }
    if (x.pr_vwap_s != null) {
      candle.pr_vwap_s = Number(x.pr_vwap_s)
    }
    return candle
  })
}

export async function fetchCandles(ticker: string, till: Date, interval: string, bars = 6000): Promise<SuperCandle[]> {
  if (await isIndexId(ticker)) {
    return fetchIndexCandles(ticker, till, interval, bars)
  }
  if (notSuperCandles.get(ticker) !== true && interval !== IntervalType.Minute) {
    return fetchSuperCandles(ticker, till, interval, bars)
  }

  const from = windowFrom(till, interval, bars)

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
      and time >= ${sqlWindowStart(from, interval)}
      and time < parseDateTimeBestEffort('${till.toISOString()}')
    group by timeslot
    order by timeslot asc`,
    format: "JSONEachRow",
  })
  const rows: any[] = await res.json()
  return rows.map(x => {
    return {
      timestamp: parseMskTime(x.timeslot),
      open: Number(x.open),
      high: Number(x.high),
      low: Number(x.low),
      close: Number(x.close),
      volume: Number(x.volume),
    }
  })
}

// Значение пользовательской SQL-метрики на диапазон свечей. Отдельный запрос,
// чтобы метрики не смешивались с основным запросом свечей и не конфликтовали
// с его алиасами. Выражение — агрегат над сырыми колонками (группировка по
// timeslot, как и у свечей). Таблица резолвится по типу инструмента; если тип
// неизвестен — перебираем таблицы, пропуская те, где нет нужных колонок.
export async function fetchMetric(ticker: string, till: Date, interval: string, expression: string, bars = 6000): Promise<MetricPoint[]> {
  // Индексы живут в tr.super_index с ключом indexid — отдельный запрос.
  if (await isIndexId(ticker)) {
    return fetchIndexMetric(ticker, till, interval, expression, bars)
  }
  const from = windowFrom(till, interval, bars)
  const tables = [await resolveSuperTable(ticker), 'tr.super_eq', 'tr.super_fo', 'tr.super_fx']
    .filter((v, i, a) => v != null && a.indexOf(v) === i) as string[]

  for (let idx = 0; idx < tables.length; idx++) {
    const table = tables[idx]
    const query = `
    select toStartOfInterval(time, interval '${interval2sql(interval)}') timeslot,
           (${expression}) as value
    from ${table}
    where secid='${ticker}' 
      and time >= ${sqlWindowStart(from, interval)}
      and time < parseDateTimeBestEffort('${till.toISOString()}')
      and pr_open > 0
    group by timeslot
    order by timeslot asc`

    let rows: any[]
    try {
      const res = await clickhouse.query({ query, format: "JSONEachRow" })
      rows = await res.json()
    } catch (e) {
      // Выражение может ссылаться на колонку, которой нет в этой таблице
      // (напр. oi_* есть только в super_fo) — пробуем следующую таблицу.
      // На последней даём ошибке всплыть, чтобы пользователь увидел её в UI.
      if (idx === tables.length - 1) {
        throw e
      }
      continue
    }
    if (rows.length > 0) {
      return rows
        .filter(r => r.value != null && Number.isFinite(Number(r.value)))
        .map(r => ({
          timestamp: parseMskTime(r.timeslot),
          value: Number(r.value),
        }))
    }
  }
  return []
}

// SQL-метрика по индексу: агрегат над сырыми колонками tr.super_index с
// группировкой по timeslot (та же семантика, что у fetchMetric для бумаг).
async function fetchIndexMetric(indexid: string, till: Date, interval: string, expression: string, bars: number): Promise<MetricPoint[]> {
  const from = windowFrom(till, interval, bars)
  const query = `
    select toStartOfInterval(time, interval '${interval2sql(interval)}') timeslot,
           (${expression}) as value
    from tr.super_index
    where indexid='${indexid}'
      and time >= ${sqlWindowStart(from, interval)}
      and time < parseDateTimeBestEffort('${till.toISOString()}')
      and pr_open > 0
    group by timeslot
    order by timeslot asc`
  const res = await clickhouse.query({ query, format: "JSONEachRow" })
  const rows: any[] = await res.json()
  return rows
    .filter(r => r.value != null && Number.isFinite(Number(r.value)))
    .map(r => ({
      timestamp: parseMskTime(r.timeslot),
      value: Number(r.value),
    }))
}

// Доля инструмента в суммарном обороте всех акций рынка (в рублях) по каждому
// таймслоту выбранного интервала. value ∈ [0..1]. Инструменты вне stock_shares
// не покрываются — вернётся пустой список.
export async function fetchMarketShares(ticker: string, till: Date, interval: string, bars = 6000): Promise<MetricPoint[]> {
  const from = windowFrom(till, interval, bars)
  const res = await clickhouse.query({
    query: `
    select p.timeslot timeslot, p.val / t.total_val as share
    from (
      select toStartOfInterval(time, interval '${interval2sql(interval)}') timeslot,
             secid,
             sum(val) val
      from tr.super_eq
      where secid = '${ticker}'
        and time >= ${sqlWindowStart(from, interval)}
        and time < parseDateTimeBestEffort('${till.toISOString()}')
        and pr_open > 0
      group by timeslot, secid
    ) p
    join (
      select toStartOfInterval(time, interval '${interval2sql(interval)}') timeslot,
             sum(val) total_val
      from tr.super_eq
      where secid in (select secid from tr.security_info FINAL where sec_group = 'stock_shares')
        and time >= ${sqlWindowStart(from, interval)}
        and time < parseDateTimeBestEffort('${till.toISOString()}')
        and pr_open > 0
      group by timeslot
    ) t using timeslot
    order by p.timeslot asc`,
    format: "JSONEachRow",
  })
  const rows: any[] = await res.json()
  return rows
    .filter(r => r.share != null && Number.isFinite(Number(r.share)))
    .map(r => ({
      timestamp: parseMskTime(r.timeslot),
      value: Number(r.share),
    }))
}

export async function fetchLatestTime(): Promise<Date> {
  const res = await clickhouse.query({
    query: `select max(time) latest from (
              select max(time) time from tr.super_eq
              union all
              select max(time) time from tr.super_fo
              union all
              select max(time) time from tr.super_fx
              union all
              select max(time) time from tr.super_index
            )`,
    format: "JSONEachRow",
  })  
  const row: any = await res.json()
  if (!row || row.length === 0 || row[0].latest === null) {
    return new Date()
  }
  return new Date(parseMskTime(row[0].latest))
}

const notSuperCandles = new Map([])

// Код акции -> код базового актива фьючерсов в super_fo, когда они не совпадают
// (FUTOI хранит данные по базовому активу фьючерсов, а не по акции). Устаревшие
// серии (напр. CHMF) заменяются актуальными (CHMFM): иначе резолв находил бы
// только истёкшие контракты и тянул бы мёртвый тикер FUTOI.
const STOCK_FUTURES_ASSET_ALIAS: Record<string, string> = {
  ABIO: 'ISKJ',
  BANEP: 'BANE',
  BELU: 'BELUGA',
  BSPBP: 'BSPB',
  CHMF: 'CHMFM',
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

// Источник открытого интереса: тикер FUTOI (короткий код базового актива) и
// ASSETCODE (ISS, для фолбэка в tr.iss_openpositions). weight — вес источника
// при агрегации (у бумаг индекса — вес в индексе, 0..1; у собственных фьючерсов
// — 1). Позиции и рублёвая оценка домножаются на вес.
export interface FutOISource {
  futoiTicker: string
  asset: string | null
  weight: number
}

export interface FutOIResolution {
  // Собственный ОИ инструмента. Для индекса — фьючерсы самого индекса
  // (MIX + вечный IMOEXF), без веса. Для бумаги/фьючерса — её единственный источник.
  own: FutOISource[]
  // ОИ бумаг, входящих в индекс, с весами как в индексе (доля 0..1). Для не-индекса — пусто.
  constituents: FutOISource[]
}

// Индекс -> базовые активы фьючерсов (asset_code в tr.security_info), по которым
// формируется собственный открытый интерес индекса. Для IMOEX это срочные
// фьючерсы MIX и вечный IMOEXF (его asset_code совпадает с индексом — IMOEX);
// у RTSI — фьючерсы RTS. Помимо этого у любого индекса добавляется ОИ бумаг,
// входящих в него (см. resolveIndexConstituentSources).
const INDEX_FUTURES_ASSETS: Record<string, string[]> = {
  IMOEX: ['MIX', 'IMOEX'],
  IMOEX2: ['MIX', 'IMOEX'],
  RTSI: ['RTS'],
}

// Резолвит источник ОИ по asset_code фьючерса.
async function resolveFutoiSourceByAsset(asset: string): Promise<FutOISource | null> {
  const res = await clickhouse.query({
    query: `
      select secid
      from tr.security_info FINAL
      where sec_group = 'futures_forts'
        and asset_code = '${asset}'
      order by is_traded desc
      limit 1
    `,
    format: "JSONEachRow",
  })
  const rows: any[] = await res.json()
  if (rows.length === 0) {
    return null
  }
  return {
    futoiTicker: stripFutoiTicker(rows[0].secid),
    asset,
    weight: 1,
  }
}

// Резолвит источники ОИ по бумагам, входящим в индекс: для каждой бумаги
// находится её фьючерс (в т.ч. переименованный, напр. SBER -> SBRF) и
// проставляется вес бумаги в индексе (доля 0..1).
async function resolveIndexConstituentSources(indexid: string): Promise<FutOISource[]> {
  const res = await clickhouse.query({
    query: `
      select secid, weight
      from tr.index_weights
      where indexid = '${indexid}'
        and tradedate = (select max(tradedate) from tr.index_weights where indexid = '${indexid}')
        and weight > 0
    `,
    format: "JSONEachRow",
  })
  const rows: any[] = await res.json()
  if (rows.length === 0) {
    return []
  }

  // Базовый код фьючерса бумаги (shortname 'BASE-M.YY'). Веса бумаг с общим
  // базовым кодом (напр. BSPB и BSPBP -> BSPB) суммируем, чтобы не учитывать
  // один и тот же фьючерс дважды.
  const weightByBase = new Map<string, number>()
  const bases: string[] = []
  for (const row of rows) {
    const base = STOCK_FUTURES_ASSET_ALIAS[String(row.secid)] ?? String(row.secid)
    const weight = Number(row.weight)
    if (!Number.isFinite(weight) || weight <= 0) continue
    if (!weightByBase.has(base)) {
      bases.push(base)
    }
    weightByBase.set(base, (weightByBase.get(base) ?? 0) + weight)
  }

  // Одним запросом находим фьючерсы для всех базовых кодов. Берём актуальный
  // контракт (торгуемый с самой поздней датой экспирации) и его asset_code:
  // именно он — ключ цены в super_fo и ASSETCODE для iss_openpositions, и он
  // может отличаться от базового кода shortname (напр. у мини-серии).
  const futRes = await clickhouse.query({
    query: `
      select
        splitByChar('-', shortname)[1] base,
        argMax(secid, tuple(is_traded, coalesce(last_tradedate, toDate(0)))) secid,
        argMax(asset_code, tuple(is_traded, coalesce(last_tradedate, toDate(0)))) asset_code
      from tr.security_info FINAL
      where sec_group = 'futures_forts'
        and splitByChar('-', shortname)[1] in (${bases.map(b => `'${b.replace(/'/g, "''")}'`).join(', ')})
      group by base
    `,
    format: "JSONEachRow",
  })
  const futRows: any[] = await futRes.json()
  const contractByBase = new Map<string, { ticker: string; asset: string }>()
  for (const f of futRows) {
    const assetCode = String(f.asset_code ?? '')
    contractByBase.set(String(f.base), {
      ticker: stripFutoiTicker(String(f.secid)),
      asset: assetCode !== '' ? assetCode : String(f.base),
    })
  }

  const sources: FutOISource[] = []
  for (const base of bases) {
    const contract = contractByBase.get(base)
    const weight = weightByBase.get(base)
    if (contract == null || weight == null) {
      continue
    }
    sources.push({ futoiTicker: contract.ticker, asset: contract.asset, weight: weight / 100 })
  }
  return sources
}

// Схлопывает источники с одинаковым тикером FUTOI, суммируя веса: один и тот же
// фьючерс может прийти от нескольких бумаг (напр. обыкновенная и привилегированная
// с общим базовым активом), и учитывать его в агрегате дважды нельзя.
function dedupeSourcesByTicker(sources: FutOISource[]): FutOISource[] {
  const byTicker = new Map<string, FutOISource>()
  for (const s of sources) {
    const cur = byTicker.get(s.futoiTicker)
    if (cur == null) {
      byTicker.set(s.futoiTicker, { ...s })
    } else {
      cur.weight += s.weight
      if (cur.asset == null) cur.asset = s.asset
    }
  }
  return Array.from(byTicker.values())
}

// Возвращает источники ОИ для выбранного инструмента, разделённые на две группы.
// own — собственный ОИ инструмента (для индекса: срочные MIX + вечный IMOEXF;
// для бумаги/фьючерса — её единственный источник; пусто, если ОИ нет).
// constituents — взвешенные бумаги, входящие в индекс (только для индекса).
export async function resolveFutoiTicker(secid: string, isFutures: boolean): Promise<FutOIResolution> {
  if (await isIndexId(secid)) {
    const own: FutOISource[] = []
    const ownAssets = INDEX_FUTURES_ASSETS[secid]
    if (ownAssets != null) {
      const resolved = await Promise.all(ownAssets.map(resolveFutoiSourceByAsset))
      for (const s of resolved) {
        if (s != null) own.push(s)
      }
    }
    const constituents = await resolveIndexConstituentSources(secid)
    return { own: dedupeSourcesByTicker(own), constituents: dedupeSourcesByTicker(constituents) }
  }

  if (isFutures) {
    // secid уже является контрактом фьючерса. ASSETCODE достаём из asset_code.
    const res = await clickhouse.query({
      query: `select asset_code from tr.security_info FINAL where secid='${secid}'`,
      format: "JSONEachRow",
    })
    const rows: any[] = await res.json()
    const assetCode: string | undefined = rows[0]?.asset_code
    // asset_code — это ASSETCODE из ISS (совпадает с ключом iss_openpositions):
    // у вечных контрактов он отличается от shortname ('IMOEXF' -> 'IMOEX',
    // 'USDRUBF' -> 'USDRUBTOM').
    return { own: [{ futoiTicker: stripFutoiTicker(secid), asset: assetCode ?? null, weight: 1 }], constituents: [] }
  }

  // Код базового актива фьючерса обычно совпадает с secid, но у части
  // переименованных тикеров отличается (SBER -> SBRF, GAZP -> GAZR и т.п.).
  const asset = STOCK_FUTURES_ASSET_ALIAS[secid] ?? secid

  // Контракт ищем в справочнике по короткому имени 'ASSET-M.YY' (например
  // 'DOMRF-6.26', 'SBRF-3.26'). Из secid контракта ('DRM6') вырезаем тикер
  // FUTOI в старой нотации ('DR'). Берём актуальный контракт (торгуемый с самой
  // поздней экспирацией), а ключ цены — его asset_code, который может отличаться
  // от базового кода shortname (напр. у мини-серии CHMFM).
  const res = await clickhouse.query({
    query: `
      select secid, asset_code
      from tr.security_info FINAL
      where sec_group = 'futures_forts'
        and shortname like '${asset}-%'
      order by is_traded desc, coalesce(last_tradedate, toDate(0)) desc
      limit 1
    `,
    format: "JSONEachRow",
  })
  const rows: any[] = await res.json()
  if (rows.length === 0) {
    return { own: [], constituents: [] }
  }
  const contractAsset: string | undefined = rows[0].asset_code
  return { own: [{ futoiTicker: stripFutoiTicker(rows[0].secid), asset: contractAsset || asset, weight: 1 }], constituents: [] }
}

// Стоимость одного контракта в ₽ для рублёвой оценки. Берём фактическую
// торгуемую стоимость из super_fo: val/vol (оборот / число контрактов). Она
// корректна и для акций, и для индексных фьючерсов, у которых цена котируется
// за пункт индекса, а не за контракт (напр. вечный IMOEXF: val/vol ≈ pr_close×10,
// фьючерс RTS — с учётом курса). На пустых по объёму слотах тянем последнее
// известное значение (иначе рублёвая оценка «проваливалась» бы в таких слотах).
// Контракт выбирается самый ликвидный (max oi_close): FUTOI агрегирует позиции
// по всем сериям, поэтому это аппроксимация, а не точная оценка стоимости.
function fizOIPriceSubquery(futoiTicker: string, from: Date, till: Date, interval: string): string {
  return `
      select timeslot, argMax(price, oi) as price
      from (
        select toStartOfInterval(time, interval '${interval2sql(interval)}') timeslot, secid,
               argMax(oi_close, time) as oi,
               last_value(sum(val) / nullIf(sum(vol), 0))
                 ignore nulls over (partition by secid order by timeslot) as price
        from tr.super_fo
        where match(secid, '^${futoiTicker}([FGHJKMNQUVXZ][0-9])?$')
          and pr_close > 0
          and time >= ${sqlWindowStart(from, interval)}
          and time < parseDateTimeBestEffort('${till.toISOString()}')
        group by timeslot, secid
      )
      group by timeslot`
}

function mapFizOIRows(rows: any[], weight: number): FutOIData[] {
  return rows.map(x => {
    const price = Number(x.price ?? 0)
    const fiz_long = Number(x.fiz_long ?? 0) * weight
    const fiz_short = Number(x.fiz_short ?? 0) * weight
    const total_long = Number(x.total_long ?? 0) * weight
    const total_short = Number(x.total_short ?? 0) * weight
    return {
      timestamp: parseMskTime(x.timeslot),
      fiz_long,
      fiz_short,
      total_long,
      total_short,
      ruble_long: price > 0 ? fiz_long * price : null,
      ruble_short: price > 0 ? fiz_short * price : null,
      ruble_total_long: price > 0 ? total_long * price : null,
      ruble_total_short: price > 0 ? total_short * price : null,
    }
  })
}

// Позиции FUTOI по тикеру в «сыром» виде (без цены/веса), чтобы применять их
// уже после батч-загрузки цены.
interface FutOIRawPoint {
  timestamp: number
  fiz_long: number
  fiz_short: number
  total_long: number
  total_short: number
}

// Батч-загрузка открытых позиций FUTOI сразу по всем источникам (один запрос
// вместо запроса на каждый источник). Ключ — тикер FUTOI, значение — точки по
// возрастанию времени. tr.futoi отсортирована по (ticker, clgroup, time),
// поэтому `ticker in (...)` использует первичный ключ.
async function fetchFutoiBatch(sources: FutOISource[], from: Date, till: Date, interval: string): Promise<Map<string, FutOIRawPoint[]>> {
  const tickers = Array.from(new Set(sources.map(s => s.futoiTicker)))
  if (tickers.length === 0) {
    return new Map()
  }
  const res = await clickhouse.query({
    query: `
    select ticker, timeslot, fiz_long, fiz_short, total_long, total_short
    from (
      select ticker, timeslot,
             sumIf(pos_long, clgroup='FIZ') as fiz_long,
             sumIf(abs(pos_short), clgroup='FIZ') as fiz_short,
             sum(pos_long) as total_long,
             sum(abs(pos_short)) as total_short
      from (
        select ticker,
               toStartOfInterval(time, interval '${interval2sql(interval)}') timeslot,
               clgroup,
               argMax(pos_long, time) as pos_long,
               argMax(pos_short, time) as pos_short
        from tr.futoi
        where ticker in (${tickers.map(t => `'${t.replace(/'/g, "''")}'`).join(', ')})
          and time >= ${sqlWindowStart(from, interval)}
          and time < parseDateTimeBestEffort('${till.toISOString()}')
        group by ticker, timeslot, clgroup
      )
      group by ticker, timeslot
    )
    order by ticker, timeslot asc`,
    format: "JSONEachRow",
  })
  const rows: any[] = await res.json()
  const byTicker = new Map<string, FutOIRawPoint[]>()
  for (const x of rows) {
    const ticker = String(x.ticker)
    let points = byTicker.get(ticker)
    if (points == null) {
      points = []
      byTicker.set(ticker, points)
    }
    points.push({
      timestamp: parseMskTime(x.timeslot),
      fiz_long: Number(x.fiz_long ?? 0),
      fiz_short: Number(x.fiz_short ?? 0),
      total_long: Number(x.total_long ?? 0),
      total_short: Number(x.total_short ?? 0),
    })
  }
  return byTicker
}

// Батч-загрузка стоимости одного контракта в ₽ сразу по всем базовым активам
// (один запрос вместо запроса на источник). Ключ внешней карты — asset_code
// (ASSETCODE, равен FutOISource.asset), значение — цена по таймслоту. В отличие
// от match(secid, regex) фильтр по asset_code не перечитывает всю super_fo на
// каждый источник и находит вечные контракты (SBERF, IMOEXF) по их коду актива.
async function fetchPriceBatch(assets: string[], from: Date, till: Date, interval: string): Promise<Map<string, Map<number, number>>> {
  if (assets.length === 0) {
    return new Map()
  }
  const res = await clickhouse.query({
    query: `
    select asset_code, timeslot, argMax(price, oi) as price
    from (
      select asset_code,
             toStartOfInterval(time, interval '${interval2sql(interval)}') timeslot,
             secid,
             argMax(oi_close, time) as oi,
             last_value(sum(val) / nullIf(sum(vol), 0))
               ignore nulls over (partition by secid order by timeslot) as price
      from tr.super_fo
      where asset_code in (${assets.map(a => `'${a.replace(/'/g, "''")}'`).join(', ')})
        and pr_close > 0
        and time >= ${sqlWindowStart(from, interval)}
        and time < parseDateTimeBestEffort('${till.toISOString()}')
      group by asset_code, timeslot, secid
    )
    group by asset_code, timeslot`,
    format: "JSONEachRow",
  })
  const rows: any[] = await res.json()
  const byAsset = new Map<string, Map<number, number>>()
  for (const x of rows) {
    const asset = String(x.asset_code)
    let prices = byAsset.get(asset)
    if (prices == null) {
      prices = new Map()
      byAsset.set(asset, prices)
    }
    prices.set(parseMskTime(x.timeslot), Number(x.price ?? 0))
  }
  return byAsset
}

// Собирает точки одного источника: домножает позиции на вес источника и считает
// рублёвую оценку по цене его контракта. Без цены рублёвые поля остаются null.
function buildFizOIData(points: FutOIRawPoint[], prices: Map<number, number>, weight: number): FutOIData[] {
  return points.map(p => {
    const price = prices.get(p.timestamp) ?? 0
    const fiz_long = p.fiz_long * weight
    const fiz_short = p.fiz_short * weight
    const total_long = p.total_long * weight
    const total_short = p.total_short * weight
    return {
      timestamp: p.timestamp,
      fiz_long,
      fiz_short,
      total_long,
      total_short,
      ruble_long: price > 0 ? fiz_long * price : null,
      ruble_short: price > 0 ? fiz_short * price : null,
      ruble_total_long: price > 0 ? total_long * price : null,
      ruble_total_short: price > 0 ? total_short * price : null,
    }
  })
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

async function fetchFizOIFromIssOpenPositions(source: FutOISource, from: Date, till: Date, interval: string): Promise<FutOIData[]> {
  if (source.asset == null) {
    return []
  }
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
        where asset='${source.asset}'
          and time >= ${sqlWindowStart(from, interval)}
          and time < parseDateTimeBestEffort('${till.toISOString()}')
        group by timeslot, clgroup
      )
      group by timeslot
    ) f
    left join (
      ${fizOIPriceSubquery(source.futoiTicker, from, till, priceInterval)}
    ) p on p.timeslot = f.timeslot
    order by f.timeslot asc`,
    format: "JSONEachRow",
  })
  const rows: any[] = await res.json()
  return mapFizOIRows(rows, source.weight)
}

function addNullable(a: number | null, b: number | null): number | null {
  if (a == null) return b
  if (b == null) return a
  return a + b
}

// Суммирует точки нескольких источников по timestamp (для индекса — срочные и
// вечные фьючерсы одного индекса).
function mergeFizOIData(dataSets: FutOIData[][]): FutOIData[] {
  const byTs = new Map<number, FutOIData>()
  for (const data of dataSets) {
    for (const d of data) {
      const cur = byTs.get(d.timestamp)
      if (cur == null) {
        byTs.set(d.timestamp, { ...d })
      } else {
        cur.fiz_long += d.fiz_long
        cur.fiz_short += d.fiz_short
        cur.total_long += d.total_long
        cur.total_short += d.total_short
        cur.ruble_long = addNullable(cur.ruble_long, d.ruble_long)
        cur.ruble_short = addNullable(cur.ruble_short, d.ruble_short)
        cur.ruble_total_long = addNullable(cur.ruble_total_long, d.ruble_total_long)
        cur.ruble_total_short = addNullable(cur.ruble_total_short, d.ruble_total_short)
      }
    }
  }
  const merged: FutOIData[] = []
  byTs.forEach(v => merged.push(v))
  return merged.sort((a, b) => a.timestamp - b.timestamp)
}

// Открытый интерес физлиц. Сначала пробуем внутридневные срезы tr.futoi
// (5 минут) по переданным источникам; точки суммируются по таймслоту (внутри
// одной группы источников: собственные фьючерсы индекса либо взвешенные бумаги).
// Дневной iss_openpositions используется ТОЛЬКО если FUTOI нет ни по
// одному источнику.
//
// Осознанное упрощение: смешивать внутридневные (FUTOI) и дневные
// (iss_openpositions) точки нельзя — дневные срезы имеют метку 00:00 и на
// внутридневной сетке молча выпадают. Поэтому при наличии FUTOI бумаги без него
// (напр. на 5m у индекса) в ОИ не попадают. Так задумано; не пересматривать без
// веской причины.
export async function fetchFizOI(sources: FutOISource[], till: Date, interval: string, bars = 6000): Promise<FizOIResult> {
  if (sources.length === 0) {
    return { data: [], isDaily: false }
  }
  const from = windowFrom(till, interval, bars)

  // Два батч-запроса на все источники: позиции FUTOI и цена контракта. Раньше
  // на каждый источник выполнялось по два отдельных запроса (для индекса — ~90
  // параллельных запросов), из-за чего они не укладывались в request_timeout
  // клиента (3 с) и Promise.all падал целиком — ОИ не показывался вовсе.
  const futoiByTicker = await fetchFutoiBatch(sources, from, till, interval)
  if (futoiByTicker.size > 0) {
    const assets = Array.from(new Set(sources.map(s => s.asset).filter((a): a is string => a != null)))
    const priceByAsset = await fetchPriceBatch(assets, from, till, interval)
    const dataSets = sources.map(s => {
      const points = futoiByTicker.get(s.futoiTicker) ?? []
      const prices = s.asset != null ? priceByAsset.get(s.asset) : undefined
      return buildFizOIData(points, prices ?? new Map(), s.weight)
    })
    return { data: mergeFizOIData(dataSets), isDaily: false }
  }

  // FUTOI нет ни по одному источнику — только дневные срезы (forward-fill на графике).
  const daily = await Promise.all(
    sources
      .filter(s => s.asset != null)
      .map(s => fetchFizOIFromIssOpenPositions(s, from, till, interval)),
  )
  return { data: mergeFizOIData(daily), isDaily: true }
}

export interface Security {
  secid: string
  shortname: string
  name: string
  contract_name: string
  emitent_title: string
  sec_type: string
  sec_group: string
  last_tradedate: string
}

interface SecurityRow {
  secid: string
  shortname: string
  name: string
  contract_name: string
  emitent_title: string
  sec_type: string
  sec_group: string
  last_tradedate: string | null
}

export async function fetchSecurities(): Promise<Security[]> {
  // Истёкшие фьючерсы (last_tradedate в прошлом) не отдаём. Дату сравниваем на
  // сервере (today() в МСК), чтобы граница не зависела от таймзоны клиента.
  // Бессрочные контракты имеют дату в далёком будущем и остаются.
  const res = await clickhouse.query({
    query: `select secid, shortname, name, contract_name, emitent_title, sec_type, sec_group, last_tradedate
            from tr.security_info FINAL
            where sec_group != 'futures_forts'
               or last_tradedate is null
               or last_tradedate >= today()
            order by secid`,
    format: "JSONEachRow",
  })  
  const rows = await res.json<SecurityRow>()
  return rows.map(x => ({
    secid: x.secid ?? '',
    shortname: x.shortname ?? '',
    name: x.name ?? '',
    contract_name: x.contract_name ?? '',
    emitent_title: x.emitent_title ?? '',
    sec_type: x.sec_type ?? '',
    sec_group: x.sec_group ?? '',
    last_tradedate: x.last_tradedate ?? '',
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

export function intervalMs(interval: string): number {
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