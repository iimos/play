# OpenCode Agent Guidelines

## Project Overview

Go application for Moscow Exchange (MOEX) data processing and analysis with ClickHouse storage.

## Commands

### Main Application
```bash
# Build and run commands
go run main.go <command> [flags]

# Available commands:
go run main.go load-supereq    # Load stock supercandles
go run main.go load-superfo    # Load futures supercandles  
go run main.go load-superfx    # Load currency supercandles
go run main.go load-supereq-latest # Snapshot of the open 5-min supercandle (stocks)
go run main.go load-superfo-latest # Snapshot of the open 5-min supercandle (futures)
go run main.go load-superfx-latest # Snapshot of the open 5-min supercandle (currency)
go run main.go load-latest     # Snapshot of the open 5-min supercandles (all markets)
go run main.go load-futoi      # Load futures open interest (FUTOI)
go run main.go load-iss-openpositions # Load daily futures open positions by phys/legal (ISS statistics)
go run main.go load-index      # Load index candles + constituent weights (all available indices; reloads whole day via DROP PARTITION)
go run main.go build-index-super # Synthesize index supercandles from stock supercandles + weights
go run main.go load            # Load all (supercandles, futoi, etc) then rebuild super_index
                               # With --watch: super_index is rebuilt live after each day reload

# Available flags:
# --force        Force reload all dates (delete and reload)
# --start {date} Start date (format: YYYY-MM-DD)
# --end {date}   End date (format: YYYY-MM-DD, defaults to today)
# --index {ids}  Comma-separated index ids (build-index-super only; ignored by load-index)
# --watch        Keep running: reload the current day every 5 minutes
#                (daily data every hour; latest supercandles every 20s).
#                Incompatible with --start/--end.
# Note: Last date in table is always reloaded automatically
```

## Architecture Notes

- Uses ClickHouse (`clickhouse-go/v2`) for data storage
- MOEX data sources: gomoex library + moexalgo package for Algopack API
- Separate `store/` package handles database operations
- `cmd/` directory contains command implementations
- Main financial data types: candles, supercandles, orderbook stats, trade stats

### Перезагрузка дня: порядок операций

Все дневные загрузчики (`super_eq/fo/fx`, `futoi`, `iss_openpositions`,
`bond_daily`, `index_candles`) перезагружают день целиком: **сначала скачивают
данные, потом удаляют партицию (`DROP PARTITION`) и вставляют**. Удалять до
скачивания нельзя — сбой сети, 401/5xx или пустой ответ стирают уже
загруженный день.

`DROP PARTITION` + `INSERT` выполняются под `context.WithoutCancel(ctx)`:
отмена (Ctrl-C, падение соседнего лоадера в `load`) действует на следующий
`fetch`, но не рвёт окно между drop и insert. Цена — процесс не завершится,
пока не закончится вставка; второй Ctrl-C убивает его принудительно и может
оставить день пустым, но тогда `max(date)` не сдвинется и день перезалиется
следующим запуском.

Пустой ответ по всем данным дня оставляет партицию нетронутой (проверки
`len(data) == 0` в лоадерах и `watch.ReloadDay` в `--watch`-циклах). Исключение —
`load-index`: пустой ответ приходит по отдельному индексу (часто только по
`interval=10`), его прежние строки не стираются, а переносятся в пересобираемую
партицию (`rescueEmpty` в `cmd/indexdata`).

Частичный ответ Algopack (`tradestats` есть, `orderstats` пуст) для прошедшего
дня — **ошибка**, а не «частичная загрузка»: иначе день перезаписался бы
неполными строками, а `count > 0` зафиксировал бы его как загруженный навсегда.
См. `checkPartial` в `cmd/supercandles`. Для текущего дня и режима `latest=1`
частичность допустима. Команда `load-candles` устарела и этих гарантий не имеет.

Порядок обхода дней: при автостарте (без `--start`) — от старых к новым
(`watch.DateRange`), чтобы обрыв оставлял последнюю дату таблицы перед дырой и
ретрай продолжал с неё. Явный `--start` — от новых к старым. Точка возобновления
автостарта: в `load-index`/`build-index-super` — самая ранняя последняя дата
среди индексов не старше 30 дней, в остальных лоадерах — последняя дата таблицы.

### Диагностика пропусков

Автостарт не видит дыр **до** точки возобновления, поэтому в конце прогона
каждый лоадер сверяет будние дни последних `watch.CoverageDays` (30) с тем, что
лежит в таблице, и печатает `warning: <таблица>: нет данных за ...` с подсказкой
`--start`. Праздники в этот отчёт тоже попадают — без торгового календаря их не
отличить от недогрузки, а молчание дороже лишней строки.

Пустые дни, встреченные по ходу прогона, собирает `watch.EmptyDays`: пустой
торговый день печатается предупреждением (после записи следующих дней автостарт
его уже не увидит), пустые выходные — отдельной `info`-строкой, чтобы
объявленная торговая суббота не осталась незамеченной (`SkipWeekends`).

## Dependencies

- `github.com/WLM1ke/gomoex` - MOEX ISS API client
- `github.com/ClickHouse/clickhouse-go/v2` - ClickHouse database driver
- `github.com/apache/arrow/go/v16` - Arrow columnar format
- `github.com/polarsignals/frostdb` - Columnar database (likely for analytics)
- `github.com/Ruvad39/go-alor` - Alor trading platform client (in alor/ submodule)

## Data Sources

- MOEX Algopack: https://moexalgo.github.io/api/rest/ (supercandles, orderbook stats)
- MOEX ISS API: https://iss.moex.com/iss/reference/

Примеры
```
curl -sk -H "Authorization: Bearer $MOEX_ALGOPACK_TOKEN" https://apim.moex.com/iss/calendars.json
curl -sk https://iss.moex.com/iss/securities.json
```

## Docs
https://moexalgo.github.io/docs/method/supercandles/
https://moexalgo.github.io/docs/method/futoi/
https://moexalgo.github.io/docs/method/hi2/
https://moexalgo.github.io/docs/method/megaalerts/
https://iss.moex.com/iss/reference/
https://raw.githubusercontent.com/moexalgo/moexalgo.github.io/main/static/openapi/openapi.yaml

## Database

`sql` directory may contain ClickHouse schema definitions

Main tables:
`tr.super_eq` - Enhanced equities data (stocks) with trader statistics
`tr.super_fo` - Futures market data  
`tr.super_fx` - Currency market data
`tr.futoi` - Futures open interest (FUTOI): открытые позиции по фьючерсам в разрезе физ/юр лиц
`tr.iss_openpositions` - Дневные открытые позиции по фьючерсам в разрезе физ/юр лиц (ISS statistics openpositions)
`tr.security_info` - Securities metadata (ReplacingMergeTree, в запросах нужен FINAL)
`tr.index_candles` - Свечи индексов MOEX (сам индекс, не фьючерс): OHLC + оборот ₽
`tr.index_weights` - Веса бумаг в индексах MOEX по дням (ISS index analytics)
`tr.super_index` - Синтез 5-минутных суперсвечей индексов из super_eq + index_weights
`tr.super_eq_latest` / `tr.super_fo_latest` / `tr.super_fx_latest` - Снимок открытой (текущей) 5-минутной суперсвечи (API `latest=1`), по одной живой строке на инструмент. `ReplacingMergeTree(updated_at)`, `ORDER BY (secid)`; читать через `FINAL`

Все таблицы используют партиционирование по дням: `PARTITION BY Date(time)`
(исключение: `index_weights` — `PARTITION BY tradedate`).

Access: `clickhouse client -f CSVWithNames -q "select 1"` (default on localhost:9000)

Описание данных находится в ./docs
