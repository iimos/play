import React, { useRef, useState, useEffect } from 'react'
import { init, dispose, Chart, TooltipShowRule, TooltipShowType, KLineData, NeighborData, Nullable } from 'klinecharts'
import generatedDataList from '../generatedDataList'
import Layout from '../Layout'

function getTooltipOptions (candleShowType: TooltipShowType, candleShowRule: TooltipShowRule, indicatorShowRule: TooltipShowRule) {
  return {
    candle: {
      tooltip: {
        showType: candleShowType,
        showRule: candleShowRule,
        legend: {
          template: (data: NeighborData<Nullable<KLineData>>) => {
            const { prev, current } = data
            if (!current) return []
            const prevClose = (prev?.close ?? current.open)
            const change = (current.close - prevClose) / prevClose * 100
            return [
              { title: 'open', value: current.open.toFixed(2) },
              { title: 'close', value: current.close.toFixed(2) },
              {
                title: 'Change: ',
                value: {
                  text: `${change.toFixed(2)}%`,
                  color: change < 0 ? '#EF5350' : '#26A69A'
                }
              }
            ]
          }
        }
      }
    },
    indicator: {
      tooltip: {
        showRule: indicatorShowRule
      }
    }
  }
}

const rules = [
  { key: 'always', text: '总是显示' },
  { key: 'follow_cross', text: '跟随十字光标' },
  { key: 'none', text: '不显示' }
]

export default function TooltipKLineChart () {
  const chart = useRef<Chart | null>(null)
  const [candleShowType, setCandleShowType] = useState('standard')
  const [candleShowRule, setCandleShowRule] = useState('always')
  const [indicatorShowRule, setIndicatorShowRule] = useState('always')

  useEffect(() => {
    chart.current = init('tooltip-k-line')
    chart.current?.createIndicator({ name: 'MA', paneId: 'candle_pane' })
    chart.current?.createIndicator({ name: 'KDJ', paneId: 'kdj_pane' })
    chart.current?.setPaneOptions({ id: 'kdj_pane', height: 80 })
    chart.current?.setSymbol({ ticker: 'TestSymbol' })
    chart.current?.setPeriod({ span: 1, type: 'day' })
    chart.current?.setDataLoader({
      getBars: ({ callback }) => {
        callback(generatedDataList())
      }
    })
    return () => { dispose('tooltip-k-line') }
  }, [])

  useEffect(() => {
    chart.current?.setStyles(getTooltipOptions(
      candleShowType as TooltipShowType, candleShowRule as TooltipShowRule, indicatorShowRule as TooltipShowRule
    ))
  }, [candleShowType, candleShowRule, indicatorShowRule])

  return (
    <Layout title="tooltip">
      <div id="tooltip-k-line" className="k-line-chart"/>
      <div
        className="k-line-chart-menu-container">
        <span style={{ paddingRight: 10 }}>主图显示类型</span>
        <button onClick={_ => { setCandleShowType('standard') }}>
          standard
        </button>
        <button onClick={_ => { setCandleShowType('rect') }}>
          rect
        </button>
      </div>
      <div
        className="k-line-chart-menu-container">
        <span style={{ paddingRight: 10 }}>k线提示显示规则</span>
        {
          rules.map(({ key, text }) => {
            return (
              <button
                key={key}
                onClick={_ => { setCandleShowRule(key as TooltipShowRule) }}>
                {text}
              </button>
            )
          })
        }
      </div>
      <div
        className="k-line-chart-menu-container">
        <span style={{ paddingRight: 10 }}>指标提示显示规则</span>
        {
          rules.map(({ key, text }) => {
            return (
              <button
                key={key}
                onClick={_ => { setIndicatorShowRule(key as TooltipShowRule) }}>
                {text}
              </button>
            )
          })
        }
      </div>
    </Layout>
  )
}
