# `super_index` — суперсвечи индексов (синтез)

MOEX не публикует для индексов суперсвечи (5-минутные метрики заявок, стакана,
дисбаланса покупок/продаж). Таблица `tr.super_index` **синтезирует** 5-минутные
суперсвечи индекса из суперсвечей акций (`tr.super_eq`) и весов бумаг
(`tr.index_weights`).

- Источник: расчёт по данным `tr.super_eq` + `tr.index_weights`
- Индексы по умолчанию: все доступные (собираются те, чьи бумаги есть в `super_eq`)
- Периодичность: 5 минут
- Глубина: ограничена доступностью `super_eq` (с 2022-01) и весов

## Модель расчёта

Индекс — взвешенный по free-float капитализации. Поэтому его относительное
изменение восстанавливается по доходностям бумаг и их весам:

```
level(t) = A · (1 + Σ w_i · (P_i(t) / P_i(ref) − 1))

A         — уровень индекса-якоря (закрытие предыдущего торгового дня)
w_i       — вес бумаги (доля, из index_weights предыдущего дня)
P_i(t)    — цена бумаги на момент t (pr_close из super_eq)
P_i(ref)  — цена закрытия бумаги в предыдущий торговый день
```

- Уровни `pr_open/pr_high/pr_low/pr_close/pr_vwap*` считаются той же формулой
  по соответствующим полям бумаг (`pr_open`, `pr_high`, `pr_low`, `pr_close`,
  `pr_vwap`).
- Для бумаг без сделок в интервале цена **переносится** (forward fill),
  поэтому их вклад равен 0.
- Окно торгов индекса берётся из его 10-минутных свечей; если интрадей не
  загружен (история > 60 дней) — используется основная сессия 09:00–19:00 MSK.
- Дни, в которые индекс не торговался (нет дневной свечи в `index_candles`),
  пропускаются.

### Валютные индексы

Валюту номинала индекс берём из ISS (`CURRENCYID` рынка индексов), а не из кода.
Не-рублёвых индексов в списке немного: **USD** — `RTSI` и прочие `RTS*`, а также
`RUBMI`, `RUCEU`, `RUEU*`, `RURPL*`; **CNY** — `IMOEXCNY`, `RUCNYCP`, `RUCNYTR`.
Для них дополнительно учитывается курс валюты (`tr.super_fx`), иначе уровень
индекса «уезжал» бы на величину валютной переоценки:

```
ratio_i(t) = (P_i(t) / P_i(ref)) · (FX(ref) / FX(t))
```

Курс валюты на MOEX публикуется разреженно (нет цены в интервалах без сделок),
поэтому для валютных индексов берётся последняя ненулевая цена. Расхождение с
фактическим значением таких индексов чуть выше, чем у рублёвых (~0.5%).
Если валюта из ISS недоступна, используется эвристика по коду.

## Агрегаты по бумагам

Цена/уровень — синтез по формуле выше. Остальные метрики агрегируются по
бумагам индекса:

| Группа | Поля | Агрегация |
|---|---|---|
| Оборот и объём | `val`, `vol`, `trades` | сумма |
| Покупки/продажи | `val_b`, `val_s`, `vol_b`, `vol_s`, `trades_b`, `trades_s` | сумма, `disb = (val_b − val_s)/(val_b + val_s)` |
| Волатильность | `pr_std` | взвешенная по весам |
| Заявки (orderstats) | `put_*`, `cancel_*` | сумма |
| Стакан (obstats) | `spread_*`, `imbalance_*`, `vwap_*_l*`, `levels_*` | взвешенные по обороту бумаги в интервале (при нулевом обороте — по весу) |

`ref_close` хранит уровень якоря `A`; `constituents` — число учтённых бумаг.

## Таблица

```sql
CREATE TABLE tr.super_index (
    indexid  LowCardinality(String),
    time     DateTime,
    ref_close Float64,
    pr_open Float32, pr_high Float32, pr_low Float32, pr_close Float32, pr_std Float32,
    vol UInt64, val Float64, trades UInt64, pr_vwap Float32, pr_change Float32,
    trades_b UInt64, trades_s UInt64, val_b Float64, val_s Float64, vol_b UInt64, vol_s UInt64,
    disb Float32, pr_vwap_b Float32, pr_vwap_s Float32,
    spread_bbo Float32, spread_lv10 Float32, spread_1mio Float32, levels_b UInt64, levels_s UInt64,
    imbalance_vol_bbo Float32, imbalance_val_bbo Float32, imbalance_vol Float32, imbalance_val Float32,
    vwap_b Float32, vwap_s Float32, vwap_b_1mio Float32, vwap_s_1mio Float32,
    put_orders_b UInt64, put_orders_s UInt64, put_val_b Float64, put_val_s Float64,
    put_vol_b UInt64, put_vol_s UInt64, put_vwap_b Float32, put_vwap_s Float32,
    put_vol UInt64, put_val Float64, put_orders UInt64,
    cancel_orders_b UInt64, cancel_orders_s UInt64, cancel_val_b Float64, cancel_val_s Float64,
    cancel_vol_b UInt64, cancel_vol_s UInt64, cancel_vwap_b Float32, cancel_vwap_s Float32,
    cancel_vol UInt64, cancel_val Float64, cancel_orders UInt64,
    constituents UInt16
) ENGINE = MergeTree()
PARTITION BY Date(time)
ORDER BY (indexid, time);
```

## Сборка

```bash
# за последние 10 дней, все доступные индексы (собираются те, чьи бумаги есть в super_eq)
go run main.go build-index-super

# конкретный диапазон
go run main.go build-index-super --start 2026-01-01 --end 2026-09-21

# только отдельные индексы, перезалив
go run main.go build-index-super --index IMOEX --force

# постоянное обновление (раз в час)
go run main.go build-index-super --watch
```

Синтез использует `super_eq` и `index_weights`, поэтому сначала должны быть
загружены суперсвечи акций (`load-supereq`) и данные индексов (`load-index`).
По умолчанию берутся все индексы, но реально собираются только те, чьи бумаги
присутствуют в `super_eq` с ценами (акции/фонды); долговые и часть зарубежных
индексов пропускаются.

## Точность

Синтезированное закрытие дня сверяется с фактической дневной свечой
`index_candles`; при расхождении > 0.5 % печатается предупреждение.
На проверенном диапазоне (сентябрь 2026):

| Индекс | Расхождение закрытия дня | Средняя ошибка 10-мин уровня |
|---|---|---|
| IMOEX | ≤ 0.24 % | 0.14 % |
| MOEXBMI | ≤ 0.22 % | 0.13 % |
| RTS* (RTSI и др.) | ≤ 0.65 % | 0.31 % |

## Ограничения

- **Валютные индексы (RTS\*, IMOEXCNY)**: курс валюты на MOEX публикуется
  разреженно (нет цены в интервалах без сделок). Первый ненулевой курс дня может
  появиться позже открытия, поэтому у синтезированного открытия ошибка выше, чем
  у закрытия.
- **Цены для неликвидных бумаг**: если бумага не торговалась в интервале,
  её цена переносится (open = high = low = close), вклад в изменение нулевой.
- **Последний интервал** (закрытие 19:00) может не иметь сделок по бумагам —
  уровень «замирает» на последней цене основной сессии.
- **Замена данных точечная**: `--index` грузит/пересобирает только указанные
  индексы и не затирает остальные. Дневной партишен целиком не пересоздаётся,
  поэтому полная пересборка больших диапазонов идёт медленнее (мутации).
- При отсутствии последней цены в референсный день (бумага не торговалась)
  берётся последняя ненулевая цена за предыдущие 30 дней.

## Примеры запросов

Значение и оборот индекса по 5-минуткам:

```sql
SELECT time, pr_open, pr_high, pr_low, pr_close,
       round(val / 1e9, 3) AS turnover_bln, disb
FROM tr.super_index
WHERE indexid = 'IMOEX' AND toDate(time) = '2026-09-21'
ORDER BY time;
```

Дневной оборот и нетто-поток (покупки минус продажи) по индексу:

```sql
SELECT toDate(time) AS d,
       round(sum(val) / 1e9, 2) AS turnover_bln,
       round(sum(val_b - val_s) / 1e9, 2) AS net_flow_bln
FROM tr.super_index
WHERE indexid = 'IMOEX'
GROUP BY d
ORDER BY d DESC
LIMIT 20;
```
