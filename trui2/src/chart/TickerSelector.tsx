import React, { useEffect, useMemo, useState } from 'react'
import Select from 'react-select'
import { Security, fetchSecurities } from '../data/index'

interface Props {
  onSelect: (ticker: string) => void
}

interface SecurityOption {
  value: string
  label: string
  secid: string
  shortname: string
  name: string
  emitent_title: string
}

const selectStyles = { menu: (styles: object) => ({ ...styles, zIndex: 999 }) }

const groupOrder: { key: string; label: string }[] = [
  { key: 'stock_shares', label: 'Акции' },
  { key: 'futures_forts', label: 'Фьючерсы' },
  { key: 'currency_selt', label: 'Валюты' },
]

function toOption(s: Security): SecurityOption {
  return {
    value: s.secid,
    label: s.shortname ? `${s.secid} — ${s.shortname}` : s.secid,
    secid: s.secid,
    shortname: s.shortname,
    name: s.name,
    emitent_title: s.emitent_title,
  }
}

export default function TickerSelector({ onSelect }: Props) {
  const [securities, setSecurities] = useState<Security[]>([])
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    fetchSecurities()
      .then(setSecurities)
      .catch(console.error)
      .finally(() => setIsLoading(false))
  }, [])

  const options = useMemo(() => {
    const byGroup = new Map<string, SecurityOption[]>()
    for (const s of securities) {
      const key = s.sec_group || 'other'
      if (!byGroup.has(key)) byGroup.set(key, [])
      byGroup.get(key)!.push(toOption(s))
    }

    const grouped = groupOrder
      .map(g => ({ label: g.label, options: byGroup.get(g.key) ?? [] }))
      .filter(g => g.options.length > 0)

    const other = byGroup.get('other')
    if (other) grouped.push({ label: 'Прочее', options: other })

    return grouped
  }, [securities])

  return (
    <Select<SecurityOption, false>
      options={options}
      onChange={x => x && onSelect(x.value)}
      styles={selectStyles}
      isLoading={isLoading}
      placeholder="Поиск по тикеру или названию…"
      noOptionsMessage={() => 'Ничего не найдено'}
      filterOption={(option, input) => {
        const q = input.trim().toLowerCase()
        if (!q) return true
        const d = option.data
        return (
          d.secid.toLowerCase().includes(q) ||
          d.shortname.toLowerCase().includes(q) ||
          d.name.toLowerCase().includes(q) ||
          d.emitent_title.toLowerCase().includes(q)
        )
      }}
      formatOptionLabel={option => (
        <span>
          <span style={{ fontWeight: 600 }}>{option.secid}</span>
          {option.shortname && <span style={{ color: '#666' }}> · {option.shortname}</span>}
        </span>
      )}
    />
  )
}
