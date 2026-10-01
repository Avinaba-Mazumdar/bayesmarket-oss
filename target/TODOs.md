# BayesMarket — Active Implementation Task Board

This document tracks the phased execution plan for **BayesMarket**. It divides the entire project lifecycle into **9 sequential implementation phases**. Each phase contains 3 to 5 concrete engineering tasks and concludes with an explicit **User Verification Task** executed on the local development server (`http://localhost:4200` / `http://localhost:8080`) or production environment.

**Anti-Shortcut Protocol**: Do not mark tasks `[x]` until files exist on disk, unit/integration tests pass with race detection, the application builds cleanly, and the user verification gate is satisfied.

---

## Phase Overview & Progress Tracker

- [x] **Phase 1: Project Scaffolding, Development Tooling & Monorepo Foundation**
- [x] **Phase 2: Database Schema, Migration Pipeline & Seed Datasets**
- [x] **Phase 3: Fixed-Point Mathematical AMM Engine & Invariant Test Suite**
- [x] **Phase 4: Backend REST API, Token-Bucket Rate Limiting & Guest Session Management**
- [x] **Phase 5: Atomic Order Execution Engine & Pessimistic Concurrency Controls**
- [x] **Phase 6: Real-Time WebSocket Multiplexer & Live Telemetry Broadcasting**
- [x] **Phase 7: Frontend Design System, Typography & WCAG 2.2 Level AAA Core Components**
- [x] **Phase 8: Interactive Trading Cockpit, Lightweight Charts & Two-Step Order Flow**
- [x] **Phase 9: Portfolio Ledger, AMM Liquidation, Oracle Resolution & End-to-End Hardening**

---

## Phase 1: Project Scaffolding, Development Tooling & Monorepo Foundation

Establish the monorepo directory layout, initialize Go and Angular workspaces, configure Neon Serverless PostgreSQL 18, and set up unified development scripts.

- [x] **Task 1.1: Monorepo & Backend Workspace Initialization**
    - Initialize Go 1.24+ module in `backend/` (`go mod init github.com/bayesmarket/bayesmarket`).
    - Install core backend dependencies: `github.com/jackc/pgx/v5`, `github.com/shopspring/decimal`, `github.com/gin-gonic/gin`, `golang.org/x/time/rate` (`github.com/gorilla/websocket` installed in Phase 6).
    - Establish backend package structure: `cmd/api/`, `internal/amm/`, `internal/database/`, `internal/middleware/`, `internal/transport/`, `internal/models/`.

- [x] **Task 1.2: Frontend Workspace Initialization (Angular 22 Zoneless)**
    - Initialize Angular 22 standalone client in `frontend/` configured for zoneless change detection (`provideExperimentalZonelessChangeDetection()`).
    - Install frontend dependencies: `lightweight-charts`.
    - Set up directory structure: `src/app/core/`, `src/app/features/`, `src/app/state/`, `src/styles/`.

- [x] **Task 1.3: Neon Database & Environment Configuration**
    - Configure root `.env.example` with Neon PostgreSQL connection parameters (`DATABASE_URL=postgres://...@...neon.tech/bayesmarket?sslmode=require`, `SERVER_PORT=8080`, `JWT_SECRET`, `CORS_ORIGIN=http://localhost:4200`).
    - Implement environment loader in backend validating required configuration on startup.
    - Provide `.env` loading and verification scripts ensuring pooled/direct connection to Neon.

- [x] **Task 1.4: Code Formatting & Tooling Validation**
    - Configure root `.prettierrc` for Markdown, JSON, and CSS formatting.
    - Verify Go formatting (`gofmt`) and linting configurations.
    - Create basic root developer task scripts in `Makefile` or `package.json` (`dev:backend`, `dev:frontend`).

### 🔍 User Verification Task (Phase 1)

1. Configure `.env` in root with your Neon PostgreSQL connection string (`https://neon.tech`): `DATABASE_URL=postgres://...@...neon.tech/bayesmarket?sslmode=require`.
2. Start the backend entrypoint: `cd backend && go run cmd/api/main.go` and verify the server connects to Neon and starts listening on `:8080`.
3. Start the frontend client: `cd frontend && pnpm dev` (or `npm start`) and navigate to `http://localhost:4200` in the browser to confirm a clean, error-free initial render.

---

## Phase 2: Database Schema, Migration Pipeline & Seed Datasets

Implement the production PostgreSQL database schema, automated migration runner, connection pool lifecycle, and real-world seed markets.

- [x] **Task 2.1: PostgreSQL DDL Schema & Migration Pipeline**
    - Create migration files in `backend/internal/database/migrations/`:
        - `000001_init_schema.up.sql`: Tables for `users`, `markets`, `liquidity_pools`, `trades`, `user_positions`, `ledger_entries`, `idempotency_keys`, and `faucet_claims`.
        - Use `NUMERIC(28, 8)` for all balances, shares, prices, and collateral; enforce `cash_balance >= 0`, `reserve_yes > 0`, `reserve_no > 0`, `collateral_reserve >= 0`, and `shares_owned >= 0`.
        - Add `UNIQUE (user_id, idempotency_key)` to `trades` and `PRIMARY KEY (actor_id, operation, idempotency_key)` to `idempotency_keys`; add indexes `idx_trades_market_created`, `idx_positions_user`, and `idx_ledger_entries_transaction`.

- [x] **Task 2.2: Connection Pool Management (`pgx/v5`)**
    - Implement connection pool factory in `backend/internal/database/db.go` with configurable min/max connections, idle timeouts, and health ping telemetry.
    - Implement a migration runner utility executing pending SQL migration scripts on startup.

- [x] **Task 2.3: Admin-Driven Prediction Market Lifecycle (Zero Pre-Fed Markets)**
    - Removed all static pre-fed demo markets from database, code, and tests.
    - Provisioned dynamic admin market creation (`POST /api/v1/admin/markets`), editing (`PUT /api/v1/admin/markets/:id`), deletion (`DELETE /api/v1/admin/markets/:id`), and settlement (`POST /api/v1/admin/markets/:id/resolve`).
    - Enforced strict Neon DB-only admin designation (`UPDATE users SET is_admin = true WHERE email = '...';`).
    - Protected AMM constant-product invariants ($k = R_{\text{YES}} \times R_{\text{NO}}$) and complete-set collateral backing during dynamic provisioning.

- [x] **Task 2.4: Schema Constraint & Migration Unit Tests**
    - Write unit tests in `backend/internal/database/db_test.go` verifying that negative balances and zero reserves trigger database check constraint errors.

### 🔍 User Verification Task (Phase 2)

1. Run the database migration: `cd backend && go run cmd/api/main.go -migrate`.
2. Open the **Neon SQL Editor** at `console.neon.tech` (or desktop client TablePlus/DBeaver).
3. To grant admin access to a trader account, run `UPDATE users SET is_admin = true WHERE email = 'YOUR_EMAIL';`.
4. Log into the web UI as the admin user and provision, edit, or delete prediction markets dynamically via the Admin Console.

---

## Phase 3: Fixed-Point Mathematical AMM Engine & Invariant Test Suite

Construct the pure, high-precision Constant Product Market Maker mathematical engine enforcing arbitrary-precision arithmetic and zero floating-point drift.

- [x] **Task 3.1: Core CPMM Bonding Invariant Engine**
    - Create `backend/internal/amm/cpmm.go` utilizing `shopspring/decimal`.
    - Implement the virtual-reserve invariant $k = R_{\text{YES}} \times R_{\text{NO}}$ and the complete-set accounting invariant: each issued YES/NO pair is backed by exactly one USDC in `collateral_reserve`.
    - Implement spot price calculation functions:
      $$P_{\text{YES}} = \frac{R_{\text{NO}}}{R_{\text{YES}} + R_{\text{NO}}}, \quad P_{\text{NO}} = \frac{R_{\text{YES}}}{R_{\text{YES}} + R_{\text{NO}}}$$
    - Enforce constraint: $P_{\text{YES}} + P_{\text{NO}} = 1.000000$ to 6 decimal places.

- [x] **Task 3.2: Share Purchase & Quote Calculation Algorithm**
    - Implement `CalculateCompleteSetBuy(depositUSDC, outcome, currentYes, currentNo, collateral)`:
        - Mint $d$ complete YES/NO sets and add $d$ to collateral. For YES: $R_{\text{NO}}' = R_{\text{NO}} + d$, $R_{\text{YES}}' = k / R_{\text{NO}}'$, $\Delta \text{YES} = R_{\text{YES}} + d - R_{\text{YES}}'$.
        - Average execution price $\bar{P} = d / \Delta \text{YES}$; use the symmetric formula for NO.
        - Marginal price impact / slippage: $\text{Slippage} = \frac{\bar{P} - P_{\text{initial}}}{P_{\text{initial}}} \times 100\%$.

- [x] **Task 3.3: Pool Liquidation & "Cash Out" Share Sale Algorithm**
    - Implement `CalculateCompleteSetSell(sharesToSell, outcome, currentYes, currentNo, collateral)`:
        - For YES shares $s$, solve $d = (A - \sqrt{A^2 - 4sR_{\text{NO}}}) / 2$, where $A = R_{\text{YES}} + R_{\text{NO}} + s$; burn $d$ complete sets and return $d$ USDC.
        - Reject any quote that would make collateral, virtual inventory, or a user position negative.
        - Returns net USDC payout and post-sale spot prices.

- [x] **Task 3.4: Rigorous Invariant & Concurrency Unit Test Suite**
    - Write extensive unit tests in `backend/internal/amm/cpmm_test.go`:
        - Invariant preservation: $k_{\text{after}} = k_{\text{before}}$ within the documented decimal rounding bound across 10,000 random transaction iterations.
        - Solvency preservation: issued YES supply equals issued NO supply and is fully backed by the collateral reserve after every operation.
        - Extreme balance checks: Sub-cent trades ($0.000001 USDC$) and large whale trades ($1,000,000 USDC$).
        - Reversibility test: Buying $N$ shares and immediately selling them back returns the exact initial funds minus calculated pool spread/fee.

### 🔍 User Verification Task (Phase 3)

1. Run the pure AMM test suite with race detector enabled:
    ```bash
    cd backend && go test -v -race ./internal/amm/...
    ```
2. Verify all test cases pass with zero race conditions, confirming $P_{\text{YES}} + P_{\text{NO}} = 1.00$ and $k$-invariant conservation under heavy test loops.

---

## Phase 4: Backend REST API, Token-Bucket Rate Limiting & Guest Session Management

Implement the RESTful HTTP API layer, guest session issuance, in-memory token-bucket rate limiting, and market discovery endpoints.

- [x] **Task 4.1: Ephemeral 1-Click Guest Session Authentication**
    - Create `backend/internal/transport/rest/auth_handler.go` and `POST /api/v1/auth/guest`.
    - Provision guest record in `users` with `is_guest = true` and `cash_balance = 1000.00000000` (virtual USDC). All financial API values are decimal strings, never JSON numbers.
    - Issue cryptographically signed HMAC-SHA256 JWT containing `user_id` and guest claims.
    - Implement JWT authentication middleware extracting user context into HTTP request context.

- [x] **Task 4.2: In-Memory Token-Bucket Rate Limiting**
    - Create `backend/internal/middleware/rate_limiter.go` using `golang.org/x/time/rate`.
    - Implement 60 public reads/minute per IP (burst 10) and 12 quote/order requests per guest ID and IP per minute (burst 3).
    - Return standardized `429 Too Many Requests` with `Retry-After` header when limit is exceeded.

- [x] **Task 4.3: Market Discovery & Quote Endpoints**
    - Implement `GET /api/v1/markets`: Returns active markets with category, volume, implied probabilities, and resolution date.
    - Implement `GET /api/v1/markets/:id`: Returns full market metadata, pool reserves, 24h stats, and resolution rules.
    - Implement `POST /api/v1/markets/:id/quote`: Accepts `{ outcome: "YES", amount_usdc: "100.00000000" }` and returns authoritative decimal-string shares, price, and slippage without mutating state.

- [x] **Task 4.4: Sandbox Faucet & Portfolio Read Endpoints**
    - Implement `POST /api/v1/faucet`: Claims 500 virtual USDC for guest accounts with an enforced 5-minute cooldown period and an `Idempotency-Key`.
    - Implement `GET /api/v1/portfolio`: Returns user cash balance, open positions, current market value, and unrealized profit/loss.

### 🔍 User Verification Task (Phase 4)

1. Start the backend: `cd backend && go run cmd/api/main.go`.
2. Open terminal and issue curl commands:

    ```bash
    # 1. Obtain Guest Token
    TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/guest | grep -o '"token":"[^"]*' | cut -d'"' -f4)

    # 2. Get the UUID from GET /api/v1/markets, then quote 50 USDC
    MARKET_ID="<market-uuid>"
    curl -s -X POST "http://localhost:8080/api/v1/markets/$MARKET_ID/quote" \
      -H "Content-Type: application/json" \
      -d '{"outcome":"YES","amount_usdc":"50.00000000"}'
    ```

3. Test rate limiting: Run a bash loop firing 35 requests rapidly to verify HTTP 429 response is returned with proper headers.

---

## Phase 5: Atomic Order Execution Engine & Pessimistic Concurrency Controls

Implement atomic trade placement, database row-level locking (`SELECT ... FOR UPDATE`), position updates, and AMM liquidation handling.

- [x] **Task 5.1: Atomic Order Execution Pipeline**
    - Create `backend/internal/transport/rest/trade_handler.go` handling `POST /api/v1/markets/:id/orders`.
    - Payload validation: `{ outcome: "YES"|"NO", amount_usdc: "100.00000000", max_slippage_pct: "2.00000000" }`.
    - Enforce atomic transaction boundary:
        1. Begin a `SERIALIZABLE` transaction and lock user row first (`SELECT cash_balance FROM users WHERE id = $1 FOR UPDATE`).
        2. Check balance sufficiency (`cash_balance >= amount`).
        3. Lock market metadata, then its liquidity-pool row (`SELECT status FROM markets WHERE id = $1 FOR UPDATE`; then `SELECT reserve_yes, reserve_no, collateral_reserve FROM liquidity_pools WHERE market_id = $1 FOR UPDATE`).
        4. Compute the collateralized complete-set swap via `internal/amm` and verify price slippage does not exceed `max_slippage_pct`.
        5. Deduct user cash balance; update virtual reserves and collateral reserve.
        6. Insert a `trades` record with the required `Idempotency-Key`, returning the original receipt on a retry.
        7. Upsert user position in `user_positions` and write balanced immutable `ledger_entries` for cash, collateral, and shares.

- [x] **Task 5.2: Atomic "Cash Out" Share Liquidation Handler**
    - Implement `POST /api/v1/portfolio/cashout`:
        - Requires bearer authentication and `Idempotency-Key`, and validates user owns sufficient shares of the specified outcome.
        - Inside one serializable transaction, locks user cash balance, user position, market, then liquidity pool in the global lock order.
        - Sells shares through the collateralized complete-set AMM, credits user cash balance, decrements user position, and records trade plus immutable ledger entries.

- [x] **Task 5.3: Concurrency & Balance Collision Test Suite**
    - Create integration test `backend/internal/transport/rest/order_concurrency_test.go`:
        - Spawns 20 concurrent goroutines attempting to spend the same $100 USDC balance simultaneously.
        - Verifies that exactly one trade succeeds and 19 fail with insufficient balance (no double spending).
        - Spawns 50 concurrent buyers on the same market and verifies final pool reserves match exact sequential math.

### 🔍 User Verification Task (Phase 5)

1. Run the concurrency integration test suite:
    ```bash
    cd backend && go test -v ./internal/transport/rest/order_concurrency_test.go
    ```
    _(Note: `-race` flag requires a C toolchain like GCC/MinGW or Linux container)_
2. Check database consistency: Confirm user balance and market reserves in PostgreSQL reflect exact mathematical outcomes with zero negative balances or drift.

---

## Phase 6: Real-Time WebSocket Multiplexer & Live Telemetry Broadcasting

Implement the low-latency WebSocket connection broker, non-blocking channel fan-out multiplexer, and real-time streaming subscriptions.

- [x] **Task 6.1: WebSocket Broker Hub & Connection Lifecycle**
    - Create `backend/internal/transport/ws/hub.go` using Gorilla WebSocket.
    - Implement dedicated per-client read and write pumps with ping/pong keep-alive (30s intervals) and write deadline timeouts.
    - Implement thread-safe client registration and unregistration via Go channels.

- [x] **Task 6.2: Non-Blocking Fan-Out Broadcast Multiplexer**
    - Implement channel broadcast multiplexer:
        - Use buffered outbound channels per client (`chan []byte`, buffer size 256).
        - Use non-blocking send with `select` + `default` to immediately disconnect slow or stalled network consumers without blocking the central trade execution engine.

- [x] **Task 6.3: Streaming Telemetry Topics**
    - Define JSON WebSocket broadcast payloads for:
        - `PRICE_UPDATE`: `{ type: "PRICE_UPDATE", market_id: "<uuid>", yes_price: "0.72000000", no_price: "0.28000000", timestamp: "2026-09-07T00:00:00Z" }`
        - `TRADE_EVENT`: `{ type: "TRADE_EVENT", market_id: "<uuid>", outcome: "YES", shares: "138.67403315", price: "0.72111554", trade_id: "<uuid>" }`
        - `MARKET_RESOLVED`: `{ type: "MARKET_RESOLVED", market_id: "<uuid>", winning_outcome: "YES" }`
    - Hook trade execution handler (Phase 5) into `wsHub.Broadcast()` to push live events on every completed trade.

- [x] **Task 6.4: Frontend Reconnecting WebSocket Service**
    - Create `frontend/src/app/core/services/websocket.service.ts`:
        - Auto-reconnect with exponential backoff (`1s`, `2s`, `5s`, `10s`).
        - Expose Angular Signals: `isConnected()`, `lastPriceTick()`, `recentTrades()`.
        - Provide user-controlled stream pause/mute toggle (satisfies WCAG 2.2 SC 2.2.4 Interruptions).

### 🔍 User Verification Task (Phase 6)

1. Start the backend: `cd backend && go run cmd/api/main.go`.
2. Open Chrome/Firefox DevTools on any WebSocket test client (or browser console on `http://localhost:4200`):
    ```javascript
    const ws = new WebSocket('ws://localhost:8080/ws/markets/<market-uuid>');
    ws.onmessage = (e) => console.log('LIVE TICK:', JSON.parse(e.data));
    ```
3. In a separate terminal, trigger a trade via curl.
4. Verify the `PRICE_UPDATE` and `TRADE_EVENT` JSON payloads arrive immediately in the browser console.

---

## Phase 7: Frontend Design System, Typography & WCAG 2.2 Level AAA Core Components

Implement the institutional-grade design system stylesheet, Scoutie Sans + JetBrains Mono typography pairing, universal 44×44px interactive components, and WCAG 2.2 Level AAA layout shell.

- [x] **Task 7.1: Design Tokens & Base CSS Architecture**
    - Create `frontend/src/styles/tokens.css` declaring all CSS custom properties from `target/DESIGN.md`:
        - Canvas: `--canvas: #07090e`, `--canvas-subtle: #0c1017`.
        - Cards: `--surface-card: #111622`, `--surface-card-elevated: #171f30`, `--surface-terminal: #0e131d`.
        - Brand: `--primary: #a6034c`, `--primary-hover: #ab034e`, `--primary-active: #85023d`, `--primary-border: #e84089`.
        - Outcomes: `--outcome-yes: #10b981`, `--outcome-yes-text: #34d399`, `--outcome-no: #fb7185`, `--outcome-no-text: #fda4af`.
        - Text: `--ink: #f8fafc`, `--ink-secondary: #cbd5e1`, `--body: #a2b4c9`.
        - Borders: `--border-strong: #606e85`, `--hairline: #1e2638`.

- [x] **Task 7.2: Typography System & SC 1.4.8 Visual Rules**
    - Import web fonts: **Scoutie Sans** (interface) and **JetBrains Mono** (quantitative financial figures).
    - Enforce tabular figures across JetBrains Mono (`font-feature-settings: "tnum" 1`).
    - Implement SC 1.4.8 visual presentation constraints: `max-width: 68ch` on prose, `line-height: 1.55`, `margin-bottom: 2.25em` on paragraphs, left-aligned only (`text-align: left`).

- [x] **Task 7.3: Universal 44×44px Interactive Primitives (SC 2.5.5 Level AAA)**
    - Implement unified `ButtonComponent` (`frontend/src/app/shared/components/button/button.component.ts`) with variant-driven styling, guaranteed `min-height: 44px; min-width: 44px;`, and $\ge 8\text{px}$ touch envelopes:
        - Variant `primary`: Deep Amaranth button with `#ffffff` text (**7.67:1** contrast).
        - Variant `yes` / `no`: Dual-coded outcome selector buttons.
        - Variant `faucet`: Pill button with `#fbcfe8` text on amaranth subtle base (**12.35:1** contrast).
        - Variant `chip`: Compact selection chip for quick-fill and timeframe selectors.
    - Additional shared primitives: `DialogComponent`, `InputComponent`, `SelectComponent`, `SwitchComponent`, `BadgeComponent`, `LabelComponent`, `TextareaComponent`, `ToastComponent`, `AvatarComponent`.
    - _Note: Feature-specific chip compositions (`QuickAmountChipComponent`, `ChartIntervalChipComponent`) are delivered in Phase 8 alongside their consuming views._

- [x] **Task 7.4: Navigation Shell & Focus Visibility (SC 2.4.12 & SC 2.4.13 Level AAA)**
    - Implement `TopHeaderDockComponent`:
        - Left: BayesMarket brand emblem and navigation links.
        - Right: Guest balance pill (`"$1,000.00 USDC"` in JetBrains Mono), `[+ Faucet]` button, live WebSocket pulse indicator.
    - Implement global focus indicators:
        ```css
        :focus-visible {
            outline: 2px solid #e84089;
            outline-offset: 2px;
            box-shadow: 0 0 0 4px rgba(232, 64, 137, 0.35);
        }
        ```
    - Enforce `scroll-padding-top: 76px; scroll-padding-bottom: 96px;` so sticky docks never obscure focused elements.
    - Add sensory overrides (`prefers-reduced-motion` and `@media (forced-colors: active)`).

### 🔍 User Verification Task (Phase 7)

1. Start frontend: `cd frontend && pnpm dev`.
2. Open `http://localhost:4200` in browser.
3. Open Chrome DevTools > Elements:
    - Verify all buttons and chips measure at least 44×44 CSS pixels.
    - Test keyboard navigation (`Tab` / `Shift+Tab`): verify 2px solid `#e84089` focus ring is crisp and visible.
    - Run Chrome Lighthouse Accessibility audit: verify contrast scores pass WCAG AAA (zero violations on text contrast).

---

## Phase 8: Interactive Trading Cockpit, Lightweight Charts & Two-Step Order Flow

Assemble the TradingView candlestick/probability chart, the real-time Order Execution Terminal, and the mandatory WCAG 2.2 SC 3.3.6 Two-Step Order Confirmation Review Dialog.

- [x] **Task 8.1: TradingView Lightweight Charts Integration**
    - Create `frontend/src/app/features/charts/price-chart.component.ts`:
        - Wraps `lightweight-charts` inside an Angular Signal component.
        - Renders probability timeline curve ($0\%$ to $100\%$) with hardware-accelerated 60fps canvas.
        - Supports timeframe selector (`1H`, `1D`, `1W`, `ALL`) via 44×44px chips.
        - Updates dynamically from WebSocket `PRICE_UPDATE` events without triggering Angular change-detection cycles.

- [x] **Task 8.2: Order Execution Terminal & Real-Time Slippage Drawer**
    - Create `frontend/src/app/features/terminal/order-terminal.component.ts`:
        - Segmented outcome toggle: `[BUY YES]` vs `[BUY NO]` (dual-coded with `▲` and `▼` glyphs).
        - Numerical USDC amount input with embedded `MAX` chip.
        - Quick-fill sizing buttons: `+$10`, `+$50`, `+$100`, `Max`.
        - Real-time slippage estimator debounced against `POST /api/v1/markets/:id/quote`; server decimal-string quotes are authoritative and the client performs no financial arithmetic:
            - Estimated shares filled (`typography.mono-md`).
            - Average execution price (`typography.mono-sm`).
            - Slippage warning badge (Green $< 1.0\%$, Amber $1.0\%–3.0\%$, Crimson $> 3.0\%$).
            - Potential percentage return (`+36.9%`).

- [x] **Task 8.3: Two-Step Financial Order Confirmation Dialog (SC 3.3.6 Level AAA)**
    - Create `frontend/src/app/features/terminal/order-confirm-dialog.component.ts`:
        - Triggered when the user clicks `"Trade $100.00 on YES"`.
        - Traps keyboard focus (`inert` attribute on parent containers) and prevents accidental one-click submissions.
        - Summarizes: Market question, Outcome (`▲ YES`), USDC deposit, Estimated shares received, Avg execution price, Max slippage tolerance, and Post-trade cash balance.
        - Actions: `"Confirm & Place Trade"` (`ButtonPrimary`) and `"Edit Order"` (dismisses dialog and refocuses input).

- [x] **Task 8.4: Market Discovery Grid & Jargon Glossary Popovers**
    - Implement `MarketCardComponent` in discovery catalog:
        - Dual probability split bar (72% Green / 28% Red track).
        - High-contrast probability pills (`▲ 72¢ YES` / `▼ 28¢ NO`).
    - Implement `GlossaryPopoverComponent` (SC 3.1.3 & SC 3.1.4):
        - Accessible popover disclosures explaining "AMM", "CPMM", "Slippage", "Implied Probability", and "Oracle".
        - Plain-language market resolution criteria summary box (Flesch-Kincaid Grade 7-8).

### 🔍 User Verification Task (Phase 8)

1. Navigate to `http://localhost:4200/markets/<market-uuid>`.
2. Inspect the TradingView canvas chart; toggle between `1H`, `1D`, and `ALL` intervals.
3. In the Order Terminal, select `[BUY YES]`, click `+$50` chip, and observe dynamic slippage and potential return calculation.
4. Click `"Trade $50.00 on YES"`:
    - Verify the **Order Confirmation Review Dialog** opens and displays exact order breakdown.
    - Click `"Edit Order"`: confirm modal closes and focus returns to amount input.
    - Click `"Trade $50.00 on YES"` again and click `"Confirm & Place Trade"`: confirm trade executes, user cash balance updates to `$950.00`, and live price updates on the chart.

---

## Phase 9: Portfolio Ledger, AMM Liquidation, Oracle Resolution & End-to-End Hardening

Build the portfolio position tracker, "Cash Out" AMM liquidation flow, automated oracle market resolution, responsive mobile audits, and production packaging.

- [x] **Task 9.1: Portfolio Ledger & Real-Time Position Accounting**
    - Create `frontend/src/app/features/portfolio/portfolio-view.component.ts`:
        - Summary metrics: Total Portfolio Value, Cash Balance, Active Positions Count, Total Unrealized PnL.
        - Positions table: Market Name, Outcome (`▲ YES` / `▼ NO`), Shares Owned, Avg Entry Price, Current Spot Price, Market Value, and PnL badge.
        - Each position row includes a 44×44px `"Cash Out"` button.

- [x] **Task 9.2: 1-Tap "Cash Out" AMM Liquidation Workflow**
    - Implement Cash Out interaction modal:
        - Calculates expected USDC proceeds from selling shares back into the AMM pool.
        - Confirms liquidation with the user.
        - Calls `POST /api/v1/portfolio/cashout` and updates cash balance and position tables in real time via Angular Signals.

- [x] **Task 9.3: Oracle Market Resolution & Payout Distribution Engine**
    - Create `backend/internal/transport/rest/admin_handler.go` (`POST /api/v1/admin/markets/:id/resolve`):
        - Accepts `{ winning_outcome: "YES"|"NO", oracle_proof: "..." }`.
        - Requires an authenticated admin role and `Idempotency-Key`; records the oracle proof before any payout.
        - Atomic payout transaction: Updates market status to `RESOLVED`, credits holders of winning shares at $1.00 USDC per share from pool collateral, zeroes losing positions, and writes immutable ledger entries.
        - In-memory 60-second caching for external oracle source responses.
        - Broadcasts `MARKET_RESOLVED` event over WebSockets to trigger client celebration banners and badge state transitions.

- [x] **Task 9.4: Multi-Device Responsive Ergonomics & Zoom Reflow Audit**
    - Audit mobile view (375px–430px): Order terminal docks cleanly to bottom 45% of viewport with thumb-zone ergonomics.
    - Audit tablet view (768px–1024px): 2-column split with sticky chart and order book.
    - Test browser zoom up to 200%: Verify 3-column desktop layout gracefully reflows to single-column without horizontal scrolling (SC 1.4.8).

- [x] **Task 9.5: Automated Testing, Telemetry & Health Endpoints**
    - Implement `/healthz` (liveness) and `/metrics` (Prometheus telemetry) endpoints.
    - Run full test suite: backend tests (`go test ./...`) and frontend production build (`pnpm build`).

### 🔍 User Verification Task (Phase 9)

1. Open `http://localhost:4200` in browser.
2. Complete the full trader lifecycle:
    - Click `[+ Faucet]` to receive testnet funds.
    - Place a $100 trade on `YES` via the confirmation modal.
    - Navigate to `/portfolio` and verify open position shows positive/negative unrealized PnL based on market price.
    - Click `"Cash Out"` on the position and verify shares are liquidated back to USDC cash balance.
3. Simulate market resolution:
    ```bash
    curl -X POST http://localhost:8080/api/v1/admin/markets/<market-uuid>/resolve \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer <admin-token>" \
      -H "Idempotency-Key: <unique-key>" \
      -d '{"winning_outcome":"YES","oracle_proof":"local test resolution"}'
    ```
4. Confirm resolved badge displays on the market card and final payouts are credited.
5. Resize browser to 375px mobile viewport and test at 200% zoom to verify zero horizontal scrolling and full WCAG 2.2 AAA accessibility.
