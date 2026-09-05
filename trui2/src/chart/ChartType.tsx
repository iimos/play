import React, { useEffect, useState, useRef } from 'react'
import { Chart, init, dispose, registerIndicator, IndicatorSeries } from 'klinecharts'
import Layout from '../Layout'
import TickerSelector from './TickerSelector'
import { fetchCandles, fetchLatestTime, IntervalType, SuperCandle, fetchStockFuturesOI } from '../data/index'

const klineStyle = {}

// Суммарный OI фьючерсов акции, заполняется перед созданием индикатора stock_oi
let stockOIMap = new Map<number, number>()

interface StockOIResult {
  oi_value: number | null
}

registerIndicator<SuperCandle>({
  name: 'volume_bs',
  series: IndicatorSeries.Volume,
  precision: 0,
  shouldFormatBigNumber: true,
  figures: [
    { key: 'volume_b', title: 'buy: ', type: 'line', styles: () => ({ color: "green" })},
    { key: 'volume_s', title: 'sell: ', type: 'line', styles: () => ({ color: "red" })}
  ],
  calc: dataList => dataList as SuperCandle[]
})

// Собственный OI фьючерса (данные уже в свечах: oi_close / oi_high / oi_low)
registerIndicator<SuperCandle>({
  name: 'oi',
  series: IndicatorSeries.Volume,
  precision: 0,
  shouldFormatBigNumber: true,
  figures: [
    { key: 'oi_close', title: 'OI: ', type: 'line', styles: () => ({ color: "#FFA500" })},
    { key: 'oi_high', title: 'OI high: ', type: 'dash', styles: () => ({ color: "#FFD700", size: 1 })},
    { key: 'oi_low', title: 'OI low: ', type: 'dash', styles: () => ({ color: "#D2691E", size: 1 })},
  ],
  calc: dataList => dataList as SuperCandle[]
})

// Суммарный OI фьючерсов акции (данные в stockOIMap)
registerIndicator<StockOIResult>({
  name: 'stock_oi',
  series: IndicatorSeries.Volume,
  precision: 0,
  shouldFormatBigNumber: true,
  figures: [
    { key: 'oi_value', title: 'OI: ', type: 'line', styles: () => ({ color: "#FFA500" })}
  ],
  calc: dataList => dataList.map(c => {
    const v = stockOIMap.get(c.timestamp)
    return { oi_value: v === undefined ? null : v }
  })
})

export default function ChartType () {
  const [ticker, setTicker] = useState("ROSN")
  const [interval, setInterval] = useState(IntervalType.Hour)
  const [activeVolumeIndicator, setActiveVolumeIndicator] = useState<'VOL' | 'volume_bs' | 'oi'>('volume_bs')
  const [oiAvailable, setOIAvailable] = useState(false)
  const [isFutures, setIsFutures] = useState(false)
  const chart = useRef<Chart | null>()
  const volumePaneId = useRef<string>("")
  const isFuturesRef = useRef(false)

  const switchIndicator = (name: 'VOL' | 'volume_bs' | 'oi') => {
    const c = chart.current
    if (!c || !volumePaneId.current) return
    if (name === 'oi' && !oiAvailable) return

    // createIndicator с isStack=false очищает панель и добавляет новый индикатор
    if (name === 'oi') {
      c.createIndicator(isFuturesRef.current ? 'oi' : 'stock_oi', false, { id: volumePaneId.current })
    } else {
      c.createIndicator(name, false, { id: volumePaneId.current })
    }
    setActiveVolumeIndicator(name)
  }

  // Основной эффект: инициализация чарта и загрузка данных
  useEffect(() => {
    let cancelled = false

    setActiveVolumeIndicator('volume_bs')
    setOIAvailable(false)
    stockOIMap = new Map()

    chart.current = init("real-time-k-line", { styles: klineStyle })
    volumePaneId.current = chart.current?.createIndicator('volume_bs', false) as string

    async function load() {
      try {
        const latestTime = await fetchLatestTime()
        const candles = await fetchCandles(ticker, latestTime, interval)
        if (cancelled) return
        chart.current?.applyNewData(candles)

        // Фьючерс определяется по наличию собственного oi_close в свечах
        const futures = candles.some(c => c.oi_close !== undefined)
        isFuturesRef.current = futures
        setIsFutures(futures)

        if (futures) {
          setOIAvailable(true)
        } else {
          const oiData = await fetchStockFuturesOI(ticker, latestTime, interval)
          if (cancelled) return
          const available = oiData.length > 0
          setOIAvailable(available)
          if (available) {
            stockOIMap = new Map(oiData.map(d => [d.timestamp, d.oi_close] as [number, number]))
          }
        }
      } catch (error) {
        console.error('Ошибка загрузки данных:', error)
      }
    }
    load()

    chart.current?.setLoadDataCallback(({ type, data, callback }) => {
      if (!data) {
        callback([], true)
        return
      }
      if (type === "forward") {
        fetchCandles(ticker, new Date(data.timestamp), interval)
          .then(candles => {
            if (cancelled) return
            const more = candles.length !== 0
            callback(candles, more)
          })
          .catch(console.error)
      } else {
        callback([], false)
      }
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
          Покупки/Продажи
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
              : isFutures
                ? "Открытый интерес фьючерса"
                : "Суммарный открытый интерес фьючерсов акции"
          }
        >
          Открытый интерес
        </button>
      </div>
    </Layout>
  )
}
