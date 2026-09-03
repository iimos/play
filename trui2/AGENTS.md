# OpenCode Agent Instructions

## Project Overview
- React 18 TypeScript app with Create React App (CRA) configuration
- Financial charting application using `klinecharts` library
- Backend proxy configured to `http://localhost:8123/` (see `package.json` `proxy` field)

## Development Commands
- `npm start` - Start development server with hot reload (port 3000)
- `npm test` - Run tests in interactive watch mode
- `npm run build` - Production build to `build/` directory
- `npm run eject` - One-way eject from CRA (not recommended unless necessary)

## Key Dependencies
- `klinecharts` (v9.8.10) - Primary charting library
- `@clickhouse/client-web` - Database client for financial data
- `react-select` - Enhanced select components
- TypeScript 4.9.5 with strict mode enabled

## Business Logic & Data Flow
- **Data Sources**: Moscow Exchange (MOEX) via MOEX ISS API and MOEX Algopack API
- **Super Candles**: Advanced market data including buy/sell volumes, order book statistics, and trade analytics
- **Database Schema**: 
  - `tr.candles` - Basic OHLCV data for stocks and indexes
  - `tr.super_eq` - Enhanced equities data (stocks) with trader statistics
  - `tr.super_fo` - Futures market data  
  - `tr.super_fx` - Currency market data
- **Database Access**: `clickhouse client -f CSVWithNames -q "select 1"` (default on localhost:9000)

## Project Structure
- `src/chart/` - Chart components (ChartType, Indicator, Theme, YAxis, etc.)
- `src/data/` - Data fetching and API integration
- Uses proxy configuration for API calls to avoid CORS issues

## Testing
- Uses `@testing-library/react` v13.4.0
- Test files should follow CRA naming convention (`*.test.{js,jsx,ts,tsx}` or `*.spec.{js,jsx,ts,tsx}`)
- No custom test configuration found - uses default CRA/Jest setup

## TypeScript Configuration
- Strict mode enabled (`strict: true`)
- JSX transform: `react-jsx` (React 17+ style)
- `isolatedModules: false` - enables cross-file type checking
- Target: ES5 for browser compatibility

## Notes
- App uses financial data APIs with ticker symbols (e.g., "ROSN")
- Chart components manage real-time data updates and indicator calculations
- Proxy configuration means API calls from frontend are routed through dev server to avoid CORS
- Supported intervals: 1m, 5m, hour, day, week, month
- Frontend fetches data via ClickHouse HTTP interface through proxy (localhost:3000 -> localhost:8123)