# Индексы MOEX (IMOEX, RTSI, MOEXBMI)

Данные по **самим индексам** Московской биржи (не фьючерсам на индекс):
значение индекса (OHLC), оборот по бумагам индекса и веса бумаг в индексе.

- Источник: MOEX ISS (публичный, без токена)
  - свечи: `https://iss.moex.com/iss/engines/stock/markets/index/securities/{ID}/candles.json`
  - веса: `https://iss.moex.com/iss/statistics/engines/stock/markets/index/analytics/{ID}.json`
- Индексы по умолчанию: все доступные (~288 из ISS analytics; основные — `IMOEX`, `RTSI`, `MOEXBMI`)
- Глубина истории весов: с 2001-01-03
- Свечи: дневные (`interval=24`) за весь диапазон, 10-минутные (`interval=10`) —
  запрашиваются за последние 60 дней; глубже ISS отдаёт их неполно (примерно
  основная сессия), уже загруженные 10-минутные строки при перезагрузке дня не
  теряются

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
# с самой ранней последней даты среди индексов (fallback — 10 дней),
# все доступные индексы (~288, включая истёкшие)
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

Дни обходятся **от старых к новым**, когда старт не задан через `--start`
(он выводится из последних дат в таблице). Так обрыв загрузки оставляет
последнюю дату перед дырой, и повторный запуск без флагов продолжает с неё,
а не считает диапазон загруженным. Явный `--start` идёт как раньше — от новых
к старым. То же правило действует и в остальных лоадерах
(`load-supereq/fo/fx`, `load-futoi`, `load-iss-openpositions`, `load-bond-daily`,
`build-index-super`). Границы диапазона нормализуются к календарным дням MSK,
поэтому `--start` (парсится в UTC) не съедает первый или последний день.

Про восстановление: автостарт не закрывает дыры, оставшиеся **до** его точки
возобновления (`minLastDate` в `load-index`, `max(date)` в остальных лоадерах) —
для них нужен явный `--start`. Дыры внутри окна `[точка, сегодня]`
перепроверяются на каждом запуске и догружаются, если партиция пуста.

Чтобы такие дыры не оставались незамеченными, в конце прогона лоадер сверяет
будние дни последних 30 (`watch.CoverageDays`) с содержимым таблицы и печатает
`warning: <таблица>: нет данных за ...` с подсказкой `--start`. Праздники в
отчёт тоже попадают: без торгового календаря их не отличить.

Пустые торговые дни, встреченные по ходу прогона, печатаются предупреждением
(после записи следующих дней автостарт их уже не увидит); пустые выходные —
отдельной `info`-строкой, чтобы объявленная торговая суббота не потерялась.

Если ISS вернул по индексу пустой массив, его прежние строки дня не удаляются:
они переносятся в пересобранную партицию (с предупреждением в логе), иначе
`DROP PARTITION` молча терял бы данные. Пустота считается по паре
«индекс + интервал», потому что ISS часто отдаёт дневную свечу и не отдаёт
10-минутные. 10-минутные свечи дней старше `intradayDays` в этом прогоне не
запрашиваются, но их строки из партиции тоже переносятся — они не теряются при
`--force`. Если не скачано ничего (ни свечей, ни весов), партиция не трогается
вовсе. Счётчики в логе (`candles N`) считают только скачанное, перенесённые
строки в них не входят.

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
