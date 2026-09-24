# Индексы MOEX (IMOEX, RTSI, MOEXBMI)

Данные по **самим индексам** Московской биржи (не фьючерсам на индекс):
значение индекса (OHLC), оборот по бумагам индекса и веса бумаг в индексе.

- Источник: MOEX ISS (публичный, без токена)
  - свечи: `https://iss.moex.com/iss/engines/stock/markets/index/securities/{ID}/candles.json`
  - веса: `https://iss.moex.com/iss/statistics/engines/stock/markets/index/analytics/{ID}.json`
- Индексы по умолчанию: все доступные (~288 из ISS analytics; основные — `IMOEX`, `RTSI`, `MOEXBMI`)
- Глубина истории весов: с 2001-01-03
- Свечи: дневные (`interval=24`) за весь диапазон, 10-минутные (`interval=10`) — за последние 60 дней

> У свечей индекса `volume` всегда 0, а `value` — это суммарный **оборот в рублях**
> по бумагам индекса (то, что обычно имеют в виду под «объёмом» индекса).

## Таблицы

### `tr.index_candles`

```sql
CREATE TABLE tr.index_candles (
    indexid  LowCardinality(String), -- IMOEX / RTSI / MOEXBMI / ...
    interval UInt16,                 -- интервал в минутах: 1, 10, 60, 24
    time     DateTime,               -- начало интервала
    end      DateTime,               -- конец интервала
    open     Float64,
    close    Float64,
    high     Float64,
    low      Float64,
    value    Float64,                -- оборот по бумагам индекса, ₽
    volume   Float64                 -- объём в лотах (у индексов 0)
) ENGINE = MergeTree()
PARTITION BY Date(time)
ORDER BY (indexid, interval, time);
```

### `tr.index_weights`

```sql
CREATE TABLE tr.index_weights (
    indexid            LowCardinality(String),
    tradedate          Date,
    ticker             LowCardinality(String),
    shortname          String,
    secid              LowCardinality(String),
    weight             Float64,       -- вес бумаги в индексе, %
    tradingsession     Int32,         -- код сессии ISS
    trade_session_date Date
) ENGINE = MergeTree()
PARTITION BY tradedate
ORDER BY (indexid, tradedate, ticker);
```

Веса за торговый день дают состав индекса и вклад бумаг (см. `super_index` в
[docs/index_super.md](index_super.md)). Сумма весов ≈ 100 %.

Идентификаторы индексов (`IMOEX`, `RTSI`, `MOEXBMI`, ...) входят в справочник
`tr.security_info` (`load-securities`), где имеют `sec_group = 'stock_index'` и
торговые параметры с борда `SNDX`/`RTSI`.

## Загрузка

```bash
# за последние 10 дней, все доступные индексы (~288, включая истёкшие)
go run main.go load-index

# конкретный диапазон
go run main.go load-index --start 2024-01-01 --end 2024-12-31

# полная история весов и дневных свечей
go run main.go load-index --start 2001-01-03

# постоянное обновление (раз в час перезаливает текущий день)
go run main.go load-index --watch
```

Флаги: `--force` (перезалить), `--start`, `--end`, `--watch`.
Последняя дата в таблице перезаливается автоматически.

Перезагрузка идёт **днём целиком**: для дня параллельно (8 воркеров)
скачиваются все действующие в этот день индексы в память, затем партиция дня
удаляется (`ALTER TABLE ... DROP PARTITION`) и данные вставляются одним батчем.
Точечных мутаций `DELETE` нет. День перезагружается, если включён `--force`,
или в нём нет свечей/весов, или это последняя дата какого-либо индекса. Если
индекс действовал в этот день (`from` из ISS analytics, `till` + 10 дней — так
учитываются свечи дней после последней даты весов), он скачивается;
пропуск отдельного индекса внутри уже загруженного дня автоматически не
латается — нужен `--force`.

Список всех индексов берётся из ISS analytics
(`/iss/statistics/engines/stock/markets/index/analytics.json`, блок `indices`) —
это индексы, по которым публикуются веса; там же указаны `from`/`till` периода
действия. Флаг `--index` для `load-index` игнорируется (используется только
`build-index-super`). Если список получить не удалось, загрузка завершается
ошибкой.

## Примеры запросов

Оборот и диапазон индекса по дням:

```sql
SELECT toDate(time) AS d, open, high, low, close,
       round(value / 1e9, 2) AS turnover_bln
FROM tr.index_candles
WHERE indexid = 'IMOEX' AND interval = 24
ORDER BY d DESC
LIMIT 20;
```

Состав индекса на дату (топ по весу):

```sql
SELECT ticker, shortname, weight
FROM tr.index_weights
WHERE indexid = 'IMOEX' AND tradedate = '2026-09-21'
ORDER BY weight DESC
LIMIT 15;
```

Суммарный оборот бумаг индекса из 5-минутных данных акций (сверка с `value`):

```sql
SELECT sum(val) AS turnover
FROM tr.super_eq
WHERE secid IN (SELECT secid FROM tr.index_weights WHERE indexid = 'IMOEX' AND tradedate = '2026-09-18')
AND toDate(time) = '2026-09-18' AND time < '2026-09-18 19:00:00';
```
