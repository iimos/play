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

# Available flags:
# --force        Force reload all dates (delete and reload)
# --start {date} Start date (format: YYYY-MM-DD)
# --end {date}   End date (format: YYYY-MM-DD, defaults to today)
# Note: Last date in table is always reloaded automatically
```

## Architecture Notes

- Uses ClickHouse (`clickhouse-go/v2`) for data storage
- MOEX data sources: gomoex library + moexalgo package for Algopack API
- Separate `store/` package handles database operations
- `cmd/` directory contains command implementations
- Main financial data types: candles, supercandles, orderbook stats, trade stats

## Dependencies

- `github.com/WLM1ke/gomoex` - MOEX ISS API client
- `github.com/ClickHouse/clickhouse-go/v2` - ClickHouse database driver
- `github.com/apache/arrow/go/v16` - Arrow columnar format
- `github.com/polarsignals/frostdb` - Columnar database (likely for analytics)
- `github.com/Ruvad39/go-alor` - Alor trading platform client (in alor/ submodule)

## Data Sources

- MOEX Algopack: https://moexalgo.github.io/api/rest/ (supercandles, orderbook stats)
- MOEX ISS API: https://iss.moex.com/iss/reference/

## Database

`sql` directory may contain ClickHouse schema definitions

Main tables:
`tr.super_eq` - Enhanced equities data (stocks) with trader statistics
`tr.super_fo` - Futures market data  
`tr.super_fx` - Currency market data

Все таблицы используют партиционирование по дням: `PARTITION BY Date(time)`.

Access: `clickhouse client -f CSVWithNames -q "select 1"` (default on localhost:9000)

## Автоматическая перезагрузка последней даты

При загрузке данных выполняется автоматическое определение последней даты в таблице. Если загружаемая дата совпадает с последней датой в таблице, она перезагружается полностью. Это решает проблему с неполными данными, когда загрузка была прервана в середине дня.

