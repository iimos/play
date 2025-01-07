import React, { useEffect, useState } from 'react'
import Select from 'react-select'
import { fetchTickers } from '../data/index'

interface Props {
  onSelect: (ticker: string) => void
}

const selectStyles = { menu: (styles: object) => ({ ...styles, zIndex: 999 }) };

export default function TickerSelector({ onSelect }: Props) {
  const [options, setOptions] = useState([] as any[])

  useEffect(() => {
    fetchTickers()
      .then(tickers => {
        setOptions(tickers.map(x => ({ value: x, label: x })))
      })
      .catch(console.error)
  }, [])

  return (
    <Select
      options={options}
      onChange={x => onSelect(x.value)}
      styles={selectStyles}
      isLoading={options.length === 0}
    />
  )
}
