import React, { useEffect, useRef } from 'react'
import { init, dispose, registerIndicator, Chart } from 'klinecharts'
import generatedDataList from '../generatedDataList'
import Layout from '../Layout'

const fruits = [
  '🍏', '🍎', '🍐', '🍊', '🍋', '🍌',
  '🍉', '🍇', '🍓', '🍈', '🍒', '🍑',
  '🍍', '🥥', '🥝', '🥭', '🥑', '🍏'
]

interface EmojiEntity {
  emoji: number
  text: string
}

// 自定义指标
registerIndicator<EmojiEntity>({
  name: 'EMOJI',
  figures: [
    { key: 'emoji' }
  ],
  calc: (kLineDataList) => {
    return kLineDataList.map(kLineData => ({ emoji: kLineData.close, text: fruits[Math.floor(Math.random() * 17)] }))
  },
  draw: ({
    ctx,
    chart,
    indicator,
    xAxis,
    yAxis
  }) => {
    const { from, to } = chart.getVisibleRange()
    const barSpace = chart.getBarSpace()

    ctx.font = `${barSpace.gapBar}px Helvetica Neue`
    ctx.textAlign = 'center'
    const result = indicator.result
    for (let i = from; i < to; i++) {
      const data = result[i]
      if (!data) continue
      const x = xAxis.convertToPixel(i)
      const y = yAxis.convertToPixel(data.emoji)
      ctx.fillText(data.text, x, y)
    }
    return true
  }
})

const mainIndicators = ['MA', 'EMA', 'SAR']
const subIndicators = ['VOL', 'MACD', 'KDJ']
const volumePaneId = 'indicator_vol_pane'

export default function Indicator () {
  const chart = useRef<Chart | null>(null)
  useEffect(() => {
    chart.current = init('indicator-k-line')
    chart.current?.createIndicator({ name: 'VOL', paneId: volumePaneId })
    chart.current?.setSymbol({ ticker: 'TestSymbol' })
    chart.current?.setPeriod({ span: 1, type: 'day' })
    chart.current?.setDataLoader({
      getBars: ({ callback }) => {
        callback(generatedDataList())
      }
    })
    return () => {
      dispose('indicator-k-line')
    }
  }, [])
  return (
    <Layout
      title="indicator">
      <div id="indicator-k-line" className="k-line-chart"/>
      <div
        className="k-line-chart-menu-container">
        <span style={{ paddingRight: 10 }}>主图指标</span>
        {
          mainIndicators.map(type => {
            return (
              <button
                key={type}
                onClick={_ => {
                  chart.current?.createIndicator({ name: type, paneId: 'candle_pane' })
                }}>
                {type}
              </button>
            )
          })
        }
        <button
          onClick={_ => {
            chart.current?.createIndicator({ name: 'EMOJI', paneId: 'candle_pane' })
          }}>
          自定义
        </button>
        <span style={{ paddingRight: 10, paddingLeft: 12 }}>副图指标</span>
        {
          subIndicators.map(type => {
            return (
              <button
                key={type}
                onClick={_ => {
                  chart.current?.createIndicator({ name: type, paneId: volumePaneId })
                }}>
                {type}
              </button>
            )
          })
        }
        <button
          onClick={_ => {
            chart.current?.createIndicator({ name: 'EMOJI', paneId: volumePaneId })
          }}>
          自定义
        </button>
      </div>
    </Layout>
  )
}
