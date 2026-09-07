import React, { useEffect } from 'react'
import { init, dispose, Chart } from 'klinecharts'
import generatedDataList from '../generatedDataList'
import Layout from '../Layout'

let timer: ReturnType<typeof setTimeout> | null = null

function clearTimer () {
  if (timer !== null) {
    clearTimeout(timer)
    timer = null
  }
}

export default function Update () {
  useEffect(() => {
    const chart = init('update-k-line')
    chart?.setSymbol({ ticker: 'TestSymbol' })
    chart?.setPeriod({ span: 1, type: 'minute' })
    chart?.setDataLoader({
      getBars: ({ callback }) => {
        callback(generatedDataList())
      },
      subscribeBar: ({ callback }) => {
        clearTimer()
        const update = () => {
          const dataList = chart?.getDataList() ?? []
          const lastData = dataList[dataList.length - 1]
          if (lastData) {
            const newData = generatedDataList(lastData.timestamp, lastData.close, 1)[0]
            newData.timestamp += 1000 * 60
            callback(newData)
          }
          timer = setTimeout(update, 1000)
        }
        update()
      },
      unsubscribeBar: () => {
        clearTimer()
      }
    })
    return () => {
      clearTimer()
      dispose('update-k-line')
    }
  }, [])
  return (
    <Layout
      title="update">
      <div id="update-k-line" className="k-line-chart"/>
    </Layout>
  )
}
