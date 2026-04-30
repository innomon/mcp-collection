# Hedge MCP Server

A Model Context Protocol (MCP) server providing financial market data, technical indicators, fundamental analysis, and news sentiment tools.


[TradingAgents: Multi-Agents LLM Financial Trading Framework](https://arxiv.org/pdf/2412.20138)

## Features

- **Market Data**: Fetch historical OHLCV price data.
- **Technical Analysis**: Calculate RSI, MACD, Bollinger Bands, EMA, and ATR.
- **Fundamental Analysis**: Retrieve key financial ratios and company metrics.
- **News & Sentiment**: Get recent headlines for specific symbols.
- **Simulation Mode**: Run with offline synthetic data for testing and development.

## Tools

### `get_prices`
Fetches historical price data for a given symbol and resolution.
- **Arguments**: `symbol` (string), `resolution` (string, e.g., "D", "60").

### `calculate_indicators`
Calculates technical indicators from price data.
- **Arguments**: `symbol` (string), `resolution` (string).

### `get_financials`
Fetches fundamental data and financial ratios.
- **Arguments**: `symbol` (string).

### `get_news`
Fetches recent news headlines.
- **Arguments**: `symbol` (string).

## Installation

```bash
cd /home/innomon/orez/mcp/mcp-collection/hedge-mcp
go build .
```

## Running the Server

### Stdio Mode (Default)
Used for local integration where the client launches the server as a subprocess.
```bash
./hedge-mcp
```

### SSE Mode
Used for remote connections or when using `agentic` with URL-based toolsets.
```bash
export HEDGE_MCP_TRANSPORT=sse
./hedge-mcp
```
Default SSE endpoint: `http://localhost:8082/mcp`

### Simulation Mode
Run without live API keys using built-in synthetic data (AAPL and BTC/USD).
```bash
./hedge-mcp -simulate
```

## Configuration

| Environment Variable | Description | Default |
|----------------------|-------------|---------|
| `FINNHUB_API_KEY` | API key for FinnHub.io | - |
| `ALPHAVANTAGE_API_KEY`| API key for AlphaVantage.co | - |
| `HEDGE_MCP_TRANSPORT` | Transport type (`stdio` or `sse`) | `stdio` |
| `HEDGE_MCP_SSE_HOST` | Host for SSE server | `0.0.0.0` |
| `HEDGE_MCP_SSE_PORT` | Port for SSE server | `8082` |
| `HEDGE_MCP_SSE_PATH` | Path for SSE endpoint | `/mcp` |
