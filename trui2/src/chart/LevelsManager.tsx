import React, { useState } from 'react'
import {
  FixedLevel,
  LevelStatus,
  LevelsConfig,
  LEVEL_PALETTE,
  defaultLevelColor,
  formatLevelDate,
} from './levels'

export interface LevelsManagerProps {
  levels: FixedLevel[]
  statuses: Record<string, LevelStatus>
  config: LevelsConfig
  precision: number
  // Показывать/скрывать панель.
  open: boolean
  onToggleOpen: () => void
  onToggleVisible: (id: string) => void
  onDelete: (id: string) => void
  onCycleColor: (id: string) => void
  onConfigChange: (patch: Partial<LevelsConfig>) => void
  onClear: () => void
}

const STATUS_LABEL: Record<LevelStatus, string> = {
  active: 'ожидание',
  tested: 'отработан',
  broken: 'пробит',
}

const STATUS_COLOR: Record<LevelStatus, string> = {
  active: '#26A69A',
  tested: '#FFB300',
  broken: '#EF5350',
}

// Смена цвета по кругу по палитре (клик по цветному кружку).
function nextPaletteColor(level: FixedLevel): string {
  const current = level.color ?? defaultLevelColor(level.type)
  const idx = LEVEL_PALETTE.indexOf(current)
  return LEVEL_PALETTE[(idx + 1) % LEVEL_PALETTE.length]
}

export default function LevelsManager({
  levels,
  statuses,
  config,
  precision,
  open,
  onToggleOpen,
  onToggleVisible,
  onDelete,
  onCycleColor,
  onConfigChange,
  onClear,
}: LevelsManagerProps) {
  // Черновик строки порога пробоя: позволяет стереть поле и набрать новое,
  // не сваливаясь в 0 (Number('') === 0).
  const [breakoutText, setBreakoutText] = useState(String(config.breakoutPct))

  return (
    <div className="levels-manager">
      <div className="levels-manager-header">
        <button onClick={onToggleOpen}>
          {open ? '▾' : '▸'} Levels Manager ({levels.length})
        </button>
        {levels.length > 0 && open && (
          <button className="levels-clear" onClick={onClear} title="Удалить все уровни по текущему тикеру">
            Очистить
          </button>
        )}
      </div>

      {open && (
        <>
          <div className="levels-options">
            <label title="Продлевать исторические уровни до текущего дня">
              <input
                type="checkbox"
                checked={config.autoExtend}
                onChange={e => onConfigChange({ autoExtend: e.target.checked })}
              />
              <span>Auto-Extend</span>
            </label>
            <label title="Скрывать уровень, если часовая свеча закрылась выше/ниже него на Y%">
              <input
                type="checkbox"
                checked={config.hideOnBreakout}
                onChange={e => onConfigChange({ hideOnBreakout: e.target.checked })}
              />
              <span>Hide on Breakout ≥</span>
            </label>
            <input
              type="number"
              min={0}
              max={50}
              step={0.1}
              value={breakoutText}
              disabled={!config.hideOnBreakout}
              onChange={e => {
                const text = e.target.value
                setBreakoutText(text)
                if (text.trim() === '') return
                const n = Number(text)
                if (Number.isFinite(n)) onConfigChange({ breakoutPct: Math.min(50, Math.max(0, n)) })
              }}
              onBlur={() => setBreakoutText(String(config.breakoutPct))}
              style={{ width: 56, height: 22 }}
            />
            <span>%</span>
          </div>

          {levels.length === 0
            ? (
              <div className="levels-empty">
                Нет уровней. ПКМ по бару профиля объёма — зафиксировать POC/VAH/VAL,
                кнопка «＋ Уровень» — нарисовать вручную.
              </div>
            )
            : (
              <div className="levels-list">
                {levels.map(level => {
                  const status = statuses[level.id] ?? 'active'
                  const color = level.color ?? defaultLevelColor(level.type)
                  return (
                    <div className="levels-row" key={level.id}>
                      <button
                        className="levels-color"
                        style={{ backgroundColor: color, opacity: level.visible ? 1 : 0.3 }}
                        title="Сменить цвет"
                        onClick={() => onCycleColor(level.id)}
                      />
                      <span className="levels-badge" style={{ color }}>{level.type}</span>
                      <span className="levels-price">{level.price.toFixed(precision)}</span>
                      <span className="levels-date">{formatLevelDate(level.startTimestamp)}</span>
                      <span className="levels-status" style={{ color: STATUS_COLOR[status] }}>
                        {STATUS_LABEL[status]}
                      </span>
                      <button
                        className="levels-eye"
                        title={level.visible ? 'Скрыть уровень' : 'Показать уровень'}
                        onClick={() => onToggleVisible(level.id)}
                      >
                        {level.visible ? '👁' : '🚫'}
                      </button>
                      <button
                        className="levels-delete"
                        title="Удалить уровень"
                        onClick={() => onDelete(level.id)}
                      >
                        🗑
                      </button>
                    </div>
                  )
                })}
              </div>
            )}
        </>
      )}
    </div>
  )
}

export { nextPaletteColor }
