import React, { useEffect, useState, useRef } from 'react'
import { Chart, init, dispose, registerIndicator, IndicatorSeries, LineType, CandleType, TooltipShowRule, TooltipShowType } from 'klinecharts'
import Layout from '../Layout'
import TickerSelector from './TickerSelector'
import { fetchSuperCandles, fetchCandles, fetchLatestTime, IntervalType, SuperCandle } from '../data/index'

const klineStyle = {
  // grid: {
  //   show: true,
  //   horizontal: {
  //     show: true,
  //     size: 1,
  //     color: "#393939",
  //     style: LineType.Dashed,
  //     dashValue: [2, 2]
  //   },
  //   vertical: {
  //     show: false,
  //     size: 1,
  //     color: "#393939",
  //     style: LineType.Dashed,
  //     dashValue: [2, 2]
  //   }
  // },
  // candle: {
  //   margin: {
  //     top: 0.2,
  //     bottom: 0.1
  //   },
  //   type: CandleType.CandleSolid,
  //   bar: {
  //     upColor: "#26A69A",
  //     downColor: "#EF5350",
  //     noChangeColor: "#888888"
  //   },
  //   area: {
  //     lineSize: 2,
  //     lineColor: "#2196F3",
  //     value: "close",
  //     backgroundColor: [
  //       {
  //         offset: 0,
  //         color: "rgba(33, 150, 243, 0.01)"
  //       },
  //       {
  //         offset: 1,
  //         color: "rgba(33, 150, 243, 0.2)"
  //       }
  //     ]
  //   },
  //   priceMark: {
  //     show: true,
  //     high: {
  //       show: true,
  //       color: "#D9D9D9",
  //       textMargin: 5,
  //       textSize: 10,
  //       textFamily: "Helvetica Neue",
  //       textWeight: "normal"
  //     },
  //     low: {
  //       show: true,
  //       color: "#D9D9D9",
  //       textMargin: 5,
  //       textSize: 10,
  //       textFamily: "Helvetica Neue",
  //       textWeight: "normal"
  //     },
  //     last: {
  //       show: true,
  //       upColor: "#26A69A",
  //       downColor: "#EF5350",
  //       noChangeColor: "#888888",
  //       line: {
  //         show: true,
  //         style: LineType.Dashed,
  //         dashValue: [4, 4],
  //         size: 1
  //       },
  //       text: {
  //         show: true,
  //         size: 12,
  //         paddingLeft: 2,
  //         paddingTop: 2,
  //         paddingRight: 2,
  //         paddingBottom: 2,
  //         color: "#FFFFFF",
  //         family: "Helvetica Neue",
  //         weight: "normal",
  //         borderRadius: 2
  //       }
  //     }
  //   },
  //   tooltip: {
  //     showRule: TooltipShowRule.Always,
  //     showType: TooltipShowType.Standard,
  //     labels: ["时间", "开", "收", "高", "低", "成交量"],
  //     values: null,
  //     defaultValue: "n/a",
  //     rect: {
  //       paddingLeft: 0,
  //       paddingRight: 0,
  //       paddingTop: 0,
  //       paddingBottom: 6,
  //       offsetLeft: 8,
  //       offsetTop: 8,
  //       offsetRight: 8,
  //       borderRadius: 4,
  //       borderSize: 1,
  //       borderColor: "#3f4254",
  //       backgroundColor: "rgba(17, 17, 17, .3)"
  //     },
  //     text: {
  //       size: 12,
  //       family: "Helvetica Neue",
  //       weight: "normal",
  //       color: "#D9D9D9",
  //       marginLeft: 8,
  //       marginTop: 6,
  //       marginRight: 8,
  //       marginBottom: 0
  //     }
  //   }
  // },
}

// interface Avp {
//   avp?: number
// }

registerIndicator<SuperCandle>({
  name: 'volume_bs',
  series: IndicatorSeries.Volume,
  precision: 0,
  // minValue: 0,
  shouldFormatBigNumber: true,
  figures: [
    { key: 'volume_b', title: 'buy: ', type: 'line', styles: (data, indicator, defaultStyles) => {
      // const kLineData = data.current.kLineData
      // let color = formatValue(indicator.styles, 'bars[0].noChangeColor', (defaultStyles.bars)[0].noChangeColor)
      // if (isValid(kLineData)) {
      //   if (kLineData.close > kLineData.open) {
      //     color = formatValue(indicator.styles, 'bars[0].upColor', (defaultStyles.bars)[0].upColor)
      //   } else if (kLineData.close < kLineData.open) {
      //     color = formatValue(indicator.styles, 'bars[0].downColor', (defaultStyles.bars)[0].downColor)
      //   }
      // }
      return { color: "green" }
    }},
    { key: 'volume_s', title: 'sell: ', type: 'line', styles: (data, indicator, defaultStyles) => ({ color: "red" })}
  ],
  calc: dataList => {
    return dataList as SuperCandle[]
    // console.log(dataList)
    // return dataList.map(d => {
    //   return d
    //   // const avp: Avp = {}
    //   // avp.avp = d.volume_s ?? 0
    //   // return avp
    // })
  }
})


export default function ChartType () {
  const [ticker, setTicker] = useState("ROSN")
  const [interval, setInterval] = useState(IntervalType.FiveMinutes)
  const chart = useRef<Chart | null>()
  const paneId = useRef<string>("")

  useEffect(() => {
    chart.current = init("real-time-k-line", {
      styles: klineStyle
    })
    chart.current?.createIndicator('VOL', true)
    paneId.current = chart.current?.createIndicator('volume_bs', true) as string

    async function fetch() {
      const latestTime = await fetchLatestTime()
      const candles = await fetchCandles(ticker, latestTime, interval)
      chart.current?.applyNewData(candles)
    }
    fetch().catch(console.error)

    chart.current?.setLoadDataCallback(({ type, data, callback }) => {
      if (!data) {
        callback([], true)
        return
      }
      // console.log(type, new Date(data.timestamp + 3*3600*1000).toISOString())
      if (type === "forward") {
        // const firstData = chart.current?.getDataList()[0]
        fetchCandles(ticker, new Date(data.timestamp), interval)
          .then(candles => {
            const more = candles.length !== 0
            callback(candles, more)
          })
          .catch(console.error)
      } else {
        callback([], false)
      }
    })

    return () => {
      if (chart.current) {
        dispose(chart.current)
      }
    }
  }, [ticker, interval])

  return (
    <Layout title={`${ticker} ${interval}`}>
      <TickerSelector onSelect={setTicker} />
      <div id="real-time-k-line" className="k-line-chart" />
      <div className="k-line-chart-menu-container">
        <button onClick={_ => setInterval(IntervalType.Minute)}>1m</button>
        <button onClick={_ => setInterval(IntervalType.FiveMinutes)}>5m</button>
        <button onClick={_ => setInterval(IntervalType.Hour)}>hour</button>
        <button onClick={_ => setInterval(IntervalType.Day)}>day</button>
        <button onClick={_ => setInterval(IntervalType.Week)}>week</button>
        <button onClick={_ => setInterval(IntervalType.Month)}>month</button>
      </div>
    </Layout>
  )
}
