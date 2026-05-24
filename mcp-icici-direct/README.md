# ICICI Direct Breeze API MCP Server

A high-performance Model Context Protocol (MCP) server implemented in Go 1.25+ designed to interface with the **ICICI Direct Breeze API**. It enables AI agents to query portfolio holdings, analyze market quotes, pull historical chart candlesticks, calculate margins, allocate segment funds, and securely execute trading orders.

---

## ⚠️ Regulatory & Compliance Guardrails

Before running the server in live environments, carefully read the following compliance guidelines from ICICI Securities:
1. **Static IP Whitelisting**: The Breeze API strictly permits order placement, cancellation, and modifications **only** from the static IP registered with your ICICI developer account.
2. **Limit Orders Only**: Market orders are prohibited by regulation for unregistered algorithms. Always utilize limit pricing.
3. **Margin Constraints**: Modifying or placing Margin or Option Plus orders via the API is disallowed.
4. **Rate Limits**: The API is restricted to **100 calls per minute**, **5,000 calls per day**, and a maximum of **10 order modifications/placements per second**.

---

## 🛠️ Onboarding & Authentication Flow

Breeze API uses an OAuth 2.0 flow to secure transactions. Follow these steps to obtain a session:

1. **Get Keys**: Register your app on the [ICICI Direct Breeze Portal](https://api.icicidirect.com/breezeapi) to retrieve your `AppKey` and `SecretKey`.
2. **Onboard Session**:
   Navigate to the URL login portal with your `AppKey` URL-encoded:
   ```
   https://api.icicidirect.com/apiuser/login?api_key=<YOUR_APP_KEY>
   ```
3. **Retrieve Session Token**:
   Log in. Upon success, you will be redirected to your registered Redirect URL. Copy the value of the `apisession` parameter in your browser address bar:
   ```
   https://127.0.0.1/?apisession=12345abcde...
   ```
   Here, `12345abcde...` is your **`BREEZE_SESSION_TOKEN`**.
4. **Environment Setups**:
   Export these values on your terminal session before launching the server:
   ```bash
   export BREEZE_APP_KEY="your_app_key"
   export BREEZE_SECRET_KEY="your_secret_key"
   export BREEZE_SESSION_TOKEN="the_apisession_value"
   ```

---

## 🚀 Modes of Operation

### 1. Mock / Simulation Mode (Recommended for testing)
To fully test capabilities and agent workflows safely without risking live capital, run the server in simulation mode:
```bash
./mcp-icici-direct -simulate -data synthetic_data.json
```
In simulation mode:
- Credentials (`BREEZE_APP_KEY`, etc.) are **not required**.
- The server intercepts API requests and serves mock data loaded from `synthetic_data.json`.
- Mock orders, funds updates, and GTT modifications are dynamically appended and persisted to `synthetic_data.json`.

### 2. Live Trading Mode
Set environment variables and execute:
```bash
./mcp-icici-direct
```

---

## ⚡ Transport Options

By default, the server runs over standard I/O (`stdio`). You can switch to HTTP Server-Sent Events (`sse`) by configuring the transport environments:

```bash
export ICICI_MCP_TRANSPORT="sse"
export ICICI_MCP_SSE_PORT="8086"
./mcp-icici-direct
```

---

## 📦 Exposed MCP Tools

### Account & Margins
- **`get_demat_holdings`**: Retrieve all shares, quantities, and pledgings from the user's Demat holdings.
- **`get_funds`**: Retrieve limits and allocation balances for Equity and F&O segments.
- **`set_funds`**: Allocate or reduce fund limits for a segment (`action: "add"|"reduce"`).
- **`get_margins`**: Retrieve active limit balances and total utilized margins.

### Market Data
- **`get_quotes`**: Fetch live L1 quotes (LTP, bid, ask, day volume) on NSE.
- **`get_historical_charts`**: Retrieve high-resolution historical OHLCV candlesticks for Equity or F&O.

### Trade Execution
- **`place_order`**: Place a new Limit or Stop-Loss order in NSE cash or F&O derivative segments.
- **`get_order_details`**: Query execution state and status logs for a specific order.
- **`get_order_list`**: Fetch the day's full active and filled order books.
- **`modify_order`**: Update price or quantity limits of a pending order.
- **`cancel_order`**: Cancel a pending order before it gets filled.
- **`square_off`**: Instantly cover and liquidate open intraday/derivative positions.

### GTT (Good Till Triggered)
- **`place_gtt_order`**: Create a long-duration order valid for up to 365 days.
- **`get_gtt_order_book`**: Fetch active/triggered GTT orders.
- **`cancel_gtt_order`**: Cancel a pending GTT order.

---

## 🏗️ Technical Development

Build the project locally:
```bash
go build .
```

Verify standard code format:
```bash
go vet ./...
```
