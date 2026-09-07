# OpenCode Agent Instructions

Project Overview:
- React 19 TypeScript app with Create React App (CRA) configuration
- Financial charting application using klinecharts library
- Backend proxy configured to `http://localhost:8123/` (see package.json proxy field)

Development Commands:
- `npm start` - Start development server with hot reload (port 3000)
- `npm test` - Run tests in interactive watch mode
- `npm run build` - Production build to `build/` directory
- `npm run eject` - One-way eject from CRA (not recommended unless necessary)

Key Dependencies:
- klinecharts@10 - Primary charting library
- @clickhouse/client-web - Database client for financial data
- react-select - Enhanced select components
- TypeScript 4.9.5 with strict mode enabled

Deps Docs:
klinecharts https://raw.githubusercontent.com/klinecharts/KLineChart/refs/heads/main/llms/llms_en-US.txt

Business Logic:
- **Data Sources**: MOEX Algopack https://moexalgo.github.io/api/rest/ (supercandles, orderbook stats)
- **Super Candles**: Advanced market data including buy/sell volumes, order book statistics, and trade analytics
- **Database Schema**: 
`tr.candles` - Basic OHLCV data for stocks and indexes
`tr.super_eq` - Enhanced equities data (stocks) with trader statistics
`tr.super_fo` - Futures market data  
`tr.super_fx` - Currency market data
`tr.futoi` - Futures open interest (FUTOI): открытые позиции по фьючерсам в разрезе физ/юр лиц
`tr.security_info` - Securities metadata (ReplacingMergeTree, в запросах нужен FINAL)

Database Access: `clickhouse client -f CSVWithNames -q "select 1"` (default on localhost:9000)
У всех таблиц и полей есть комментарии.
Более детальное описание данных есть в `../tr/docs`.

Описание данных:
https://moexalgo.github.io/docs/method/supercandles/
https://moexalgo.github.io/docs/method/futoi/
https://moexalgo.github.io/docs/method/hi2/
https://moexalgo.github.io/docs/method/megaalerts/
https://iss.moex.com/iss/reference/

Project Structure:
- `src/chart/` - Chart components (ChartType, Indicator, Theme, YAxis, etc.)
- `src/data/` - Data fetching and API integration
- Uses proxy configuration for API calls to avoid CORS issues

TypeScript Configuration:
- Strict mode enabled (`strict: true`)
- JSX transform: `react-jsx` (React 17+ style)
- `isolatedModules: false` - enables cross-file type checking
- Target: ES5 for browser compatibility

Notes:
- App uses financial data APIs with ticker symbols (e.g., "ROSN")
- Chart components manage real-time data updates and indicator calculations
- Proxy configuration means API calls from frontend are routed through dev server to avoid CORS
- Supported intervals: 1m, 5m, hour, day, week, month
- Frontend fetches data via ClickHouse HTTP interface through proxy (localhost:3000 -> localhost:8123)
