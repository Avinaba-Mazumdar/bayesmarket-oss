# BayesMarket — System Architecture & Technical Specification

> **System Name**: BayesMarket (Prediction Exchange Platform)  
> **Platform Version**: `v0.1.0` (MVP0)  
> **Target Audience**: Systems Engineers, Quantitative Developers, and Open-Source Contributors  
> **Engineering Focus**: Financial correctness, atomic execution, high-concurrency Go engine, and reactive Angular 22 Signals UI.

---

## 1. Executive Summary & Core Value Proposition

**BayesMarket** is a high-performance binary prediction market platform inspired by Polymarket. Participants trade outcome shares (**YES** and **NO**) on verifiable future events.

Unlike superficial betting applications that record arbitrary wagers, BayesMarket runs a **collateralized complete-set Constant Product Automated Market Maker (CPMM)** backed by an **atomic PostgreSQL ledger**, real-time **WebSocket broadcast pipelines**, and a **zoneless Angular 22 frontend**.

Traders and developers can interact with the platform immediately via an **instant 1-click Guest Mode** pre-loaded with **$1,000 virtual USDC**, protected by an in-memory **Token-Bucket rate limiter** to guarantee zero external API abuse and operational resilience.

### 1.1 Primary Engineering Objectives

1. **Financial Integrity**: Pure mathematical AMM bonding curve preventing arbitrage and pool depletion. Every share resolves to exactly $1.00 or $0.00 upon market resolution.
2. **Zero-Friction Sandbox Onboarding**: 1-click **Guest Mode** pre-seeded with $1,000 virtual USDC. Users can test trades, inspect slippage, and review PnL immediately without mandatory wallet connection or email signup.
3. **High Concurrency & Low Latency**: Go backend handling order execution, concurrency control via mutexes/channels, and live price updates over WebSockets.
4. **Abuse Resistance**: In-memory token-bucket rate limiting per Guest Session / IP to prevent denial-of-service and abuse of external endpoints.
5. **Modern Reactive UI**: Angular 22 standalone components, zoneless change detection, and Signals for live price updates and interactive TradingView canvas charts.

---

## 2. Tech Stack & Dependency Matrix

### 2.1 Package Management & Monorepo Toolchain

| Tool                         | Version                    | Role                           | Rationale                                                                                                         |
| :--------------------------- | :------------------------- | :----------------------------- | :---------------------------------------------------------------------------------------------------------------- |
| **Package Manager (Client)** | **pnpm** `^10.5.0`         | Frontend Dependency Management | Content-addressable storage, strict dependency isolation (no phantom dependencies), 3x faster installs than npm.  |
| **Runtime (Host)**           | **Node.js** `v24.15.0 LTS` | Build & SSR Engine             | Modern V8 runtime with native ESM, top-level await, and integrated test runner. Matches local host environment.   |
| **Monorepo Engine**          | **Turborepo** `^2.4.0`     | Task Pipeline Orchestration    | Zero-configuration cached monorepo task runner. A single `pnpm dev` concurrently launches Go API and Angular SPA. |
| **Go Dependency Manager**    | **Go Modules** (`go mod`)  | Backend Dependency Management  | Native Go 1.27 toolchain with cryptographic checksum verification (`go.sum`) and vendor directory caching.        |

### 2.2 Frontend Stack (Client SPA)

| Category               | Technology / Library          | Version   | Engineering Rationale                                                                                                                 |
| :--------------------- | :---------------------------- | :-------- | :------------------------------------------------------------------------------------------------------------------------------------ |
| **Framework**          | **Angular**                   | `^22.1.5` | Current stable major. Zoneless by default, native Signal Forms, Signal inputs/outputs, and streaming SSR.                             |
| **Language**           | **TypeScript**                | `^7.0.2`  | Major release powered by the Go-based compiler ("Project Corsa"), delivering 8–12x faster compile times and strict type verification. |
| **Financial Charting** | `lightweight-charts`          | `^5.2.1`  | High-DPI canvas charting engine (TradingView v5, 60fps hardware accelerated). Zero React/Vue bloat.                                   |
| **Icons & Micro-UI**   | `@lucide/angular`             | `^1.41.0` | Modern tree-shakeable SVG icon set for Angular.                                                                                       |
| **Design System**      | **Modern Vanilla CSS Tokens** | Native    | CSS Custom Properties, CSS Grid, Glassmorphic panels, obsidian dark mode (`#0a0d14`). Zero Tailwind runtime overhead.                 |
| **State Management**   | **Angular Signal Stores**     | Native    | Fine-grained reactivity. WebSocket price ticks mutate Signals directly without RxJS stream overhead.                                  |

### 2.3 Backend Stack (Trading Engine & API)

| Category                     | Technology / Library                 | Version   | Engineering Rationale                                                                                                        |
| :--------------------------- | :----------------------------------- | :-------- | :--------------------------------------------------------------------------------------------------------------------------- |
| **Language & Runtime**       | **Go (Golang)**                      | `1.24+`   | Latest release. Generic methods, high-performance green-thread scheduler, lightweight static binary.                         |
| **HTTP Routing**             | **Gin (`github.com/gin-gonic/gin`)** | `v1.10.0` | High-performance, battle-tested HTTP framework with built-in JSON binding/validation and native `gorilla/websocket` support. |
| **WebSocket Engine**         | `github.com/gorilla/websocket`       | `v1.5.3`  | RFC 6455 implementation with dedicated read/write pumps, ping/pong support, and stable Go ecosystem adoption.                |
| **Database Driver**          | `github.com/jackc/pgx/v5`            | `v5.7.1`  | High-performance PostgreSQL connection pool (`pgxpool`) with native binary encoding.                                         |
| **Arbitrary Precision Math** | `github.com/shopspring/decimal`      | `v1.4.0`  | Exact fixed-point arithmetic. Eliminates IEEE-754 floating-point inaccuracies in financial calculations.                     |
| **Rate Limiter**             | `golang.org/x/time/rate`             | Latest    | In-memory token-bucket limiter with sub-microsecond latency per IP and Guest UUID.                                           |
| **Authentication**           | `github.com/golang-jwt/jwt/v5`       | `v5.2.1`  | HMAC-SHA256 signed stateless tokens for ephemeral guest sessions.                                                            |
| **UUIDs**                    | `github.com/google/uuid`             | `v1.6.0`  | RFC 4122 compliant UUIDv4 identifiers for orders, trades, and markets.                                                       |

### 2.4 Persistence & Infrastructure

| Layer                   | Provider / Tool             | Configuration         | Rationale                                                                                                            |
| :---------------------- | :-------------------------- | :-------------------- | :------------------------------------------------------------------------------------------------------------------- |
| **Database**            | **Neon PostgreSQL**         | Version `18.x`        | Latest major release with enhanced concurrent B-tree index scans and `FOR UPDATE` locking. Scales to zero when idle. |
| **Database Migrations** | `golang-migrate/migrate/v4` | `v4.18.0`             | Version-controlled idempotent SQL migrations (`.up.sql` / `.down.sql`).                                              |
| **Packaging & Runtime** | **Native Static Binary**    | `CGO_ENABLED=0` build | Minimal memory footprint with sub-millisecond invocation and zero external container dependencies.                   |
| **Backend Hosting**     | **Fly.io** / **Render**     | Shared CPU, 256MB RAM | Sub-15ms cold start for Go binaries, generous free tier.                                                             |
| **Frontend Hosting**    | **Cloudflare Pages**        | Global Edge CDN       | Sub-50ms TTFB globally, automated Git CI/CD, free SSL and DDoS mitigation.                                           |

---

## 3. System Design & Topology (HLD)

### 3.1 End-to-End System Topology

```mermaid
graph TB
    subgraph ClientLayer ["Client Presentation Layer (Browser)"]
        UI["Angular 22 SPA (Zoneless + Signals)"]
        TVChart["TradingView Lightweight Canvas Engine"]
        WSSub["WebSocket Client (Auto-reconnect)"]
    end

    subgraph EdgeLayer ["Global Edge Network (Cloudflare)"]
        CDN["Cloudflare Edge CDN"]
        SSL["Edge SSL / DDoS Shield"]
    end

    subgraph GatewayLayer ["Go Backend Gateway (:8080)"]
        CORS["CORS & Security Headers"]
        RateLimit["Token-Bucket Limiter (IP + Session)"]
        AuthCtx["JWT / Guest Session Context Middleware"]
        Router["Gin HTTP Engine & WebSocket Upgrader"]
    end

    subgraph EngineLayer ["Core Trading & Market Domain"]
        AMM["AMM Engine (Pure Constant Product Math)"]
        WSHub["WebSocket Broker (Channel Fan-out)"]
        TickerBot["Simulated Market Activity Worker"]
    end

    subgraph PersistenceLayer ["ACID Storage (PostgreSQL 18)"]
        DB[(Neon Serverless Postgres)]
        PoolLock["Row-Level Lock (SELECT FOR UPDATE)"]
    end

    UI --> CDN
    UI -->|REST HTTPS| SSL
    WSSub -->|WSS Protocol| SSL
    SSL --> CORS
    CORS --> RateLimit
    RateLimit --> AuthCtx
    AuthCtx --> Router

    Router -->|Trade Execution| AMM
    AMM -->|Atomic Transaction| PoolLock
    PoolLock --> DB
    AMM -->|Broadcast Event| WSHub
    TickerBot -->|Simulated Micro-trades| AMM
    WSHub -->|Price & Trade Frames| WSSub
```

---

### 3.2 Market State Machine Lifecycle

Prediction markets undergo a strict finite state machine to guarantee that trading halts before an event occurs and that funds are only distributed upon validated resolution:

```mermaid
stateDiagram-v2
    [*] --> DRAFT: Admin Creates Market
    DRAFT --> ACTIVE: Admin Publishes & Funds Initial Reserves

    ACTIVE --> SUSPENDED: Admin Emergency Freeze / Circuit Breaker
    SUSPENDED --> ACTIVE: Resume Trading

    ACTIVE --> LOCK_TRADING: Resolution Date Reached (Trading Disabled)
    SUSPENDED --> LOCK_TRADING: Direct Resolution Triggered

    LOCK_TRADING --> RESOLVED: Winning Outcome Declared (YES or NO)
    RESOLVED --> SETTLED: All User Positions Redeemed ($1.00 per share)

    SETTLED --> [*]
    ACTIVE --> CANCELLED: Event Invalidation / Refund All Collateral
    CANCELLED --> [*]
```

---

### 3.3 Atomic Order Execution Lifecycle

```mermaid
sequenceDiagram
    autonumber
    actor User as Guest / Trader
    participant Client as Angular Client (Signal Store)
    participant API as Go Gateway (:8080)
    participant Limiter as Token-Bucket Limiter
    participant AMM as AMM Math Engine
    participant DB as PostgreSQL 18 (Neon)
    participant Hub as WebSocket Hub

    User->>Client: Clicks "Buy $50 YES"
    Client->>Client: Set Optimistic Pending State
    Client->>API: POST /api/v1/markets/:id/orders {outcome, amount_usdc, max_slippage_pct} + Idempotency-Key
    API->>Limiter: Allow(IP, GuestUUID)?
    alt Rate Limit Exceeded
        Limiter-->>API: 429 Too Many Requests
        API-->>Client: HTTP 429 (ERR_RATE_LIMIT_EXCEEDED)
    else Limit OK
        Limiter-->>API: Allow
        API->>DB: BEGIN TRANSACTION (ISOLATION SERIALIZABLE)
        API->>DB: SELECT cash_balance FROM users WHERE id = $1 FOR UPDATE
        DB-->>API: User Balance ($1,000.00)
        API->>DB: SELECT status FROM markets WHERE id = $2 FOR UPDATE
        API->>DB: SELECT * FROM liquidity_pools WHERE market_id = $2 FOR UPDATE
        DB-->>API: Current inventories and collateral (R_yes, R_no, C, k)

        API->>AMM: CalculateCompleteSetBuy(R_yes, R_no, C, k, amount, max_slippage)
        alt Slippage Exceeded or Insufficient Balance
            AMM-->>API: ErrSlippageExceeded / ErrInsufficientFunds
            API->>DB: ROLLBACK
            API-->>Client: HTTP 400 Bad Request (Standard Error Envelope)
        else Calculation Valid
            AMM-->>API: SharesFilled, AvgPrice, NewReserves
            API->>DB: UPDATE users SET cash_balance = cash_balance - amount WHERE id = $1
            API->>DB: UPDATE liquidity_pools SET reserve_yes = $1, reserve_no = $2, collateral_reserve = collateral_reserve + $3 WHERE market_id = $4
            API->>DB: INSERT INTO user_positions (shares, avg_price) ON CONFLICT DO UPDATE
            API->>DB: INSERT INTO trades (idempotency key, execution receipt)
            API->>DB: INSERT INTO ledger_entries (cash, collateral, position deltas)
            API->>DB: COMMIT TRANSACTION
            API-->>Client: HTTP 200 OK (Trade Receipt)
            API->>Hub: Broadcast(MarketID, NewPrice, TradeEvent)
            Hub-->>Client: WS Frame: PRICE_UPDATE & TRADE_EVENT
            Client->>Client: Signals Update (Balance, Positions, Chart Tick)
        end
    end
```

---

### 3.4 Mathematical Market Engine (Constant Product Binary AMM)

The exchange uses a **collateralized complete-set Constant Product Market Maker (CPMM)**. A complete set contains one YES share and one NO share and is minted only against exactly $1.00 USDC in the market collateral reserve. Virtual YES/NO inventories determine price; the collateral reserve guarantees settlement.

#### 1. The Invariant

$$k = R_{\text{YES}} \times R_{\text{NO}}$$
$R_{\text{YES}}$ and $R_{\text{NO}}$ are virtual share inventories. The pool also holds collateral reserve $C$. Issued YES shares and issued NO shares always have equal supply because they are minted only as complete sets; $C$ therefore covers the maximum settlement liability.

#### 2. Spot Price Calculation

$$\text{Price}(\text{YES}) = \frac{R_{\text{NO}}}{R_{\text{YES}} + R_{\text{NO}}}, \quad \text{Price}(\text{NO}) = \frac{R_{\text{YES}}}{R_{\text{YES}} + R_{\text{NO}}}$$
$$\text{Constraint: } \text{Price}(\text{YES}) + \text{Price}(\text{NO}) = 1.0000$$

#### 3. Buying Outcome Shares (Depositing $d$ USDC for YES)

1. Add $d$ USDC to collateral $C$ and mint $d$ complete sets. Add their YES and NO shares to the virtual inventories.
2. Preserve the invariant after removing the purchased YES from inventory:
   $$R_{\text{NO}}' = R_{\text{NO}} + d, \quad R_{\text{YES}}' = \frac{k}{R_{\text{NO}}'}$$
3. Transfer the bought shares to the user:
   $$\Delta \text{YES} = R_{\text{YES}} + d - R_{\text{YES}}'$$
4. Average execution price:
   $$\bar{P}_{\text{exec}} = \frac{d}{\Delta \text{YES}}$$
5. Price Impact / Slippage:
   $$\text{Slippage} = \left| \frac{\text{Price}_{\text{new}} - \text{Price}_{\text{initial}}}{\text{Price}_{\text{initial}}} \right| \times 100\%$$

A NO purchase is symmetric: add $d$ to both inventories, set $R_{\text{YES}}' = R_{\text{YES}} + d$, calculate $R_{\text{NO}}' = k / R_{\text{YES}}'$, and transfer $R_{\text{NO}} + d - R_{\text{NO}}'$ NO shares.

#### 4. Selling Outcome Shares (Returning $s$ YES Shares for USDC)

1. Add $s$ YES shares returned by the user to the YES inventory.
2. Withdraw and burn $d$ complete sets from the pool. The post-trade inventories are $R_{\text{YES}}' = R_{\text{YES}} + s - d$ and $R_{\text{NO}}' = R_{\text{NO}} - d$.
3. Solve the invariant for the smaller valid root, then transfer that collateral to the user:
   $$d = \frac{A - \sqrt{A^2 - 4sR_{\text{NO}}}}{2}, \quad A = R_{\text{YES}} + R_{\text{NO}} + s$$
4. The trade is valid only if $0 < d < \min(C, R_{\text{NO}})$; the symmetric formula applies to NO sales.
5. Effective selling price:
   $$\bar{P}_{\text{sell}} = \frac{d}{s}$$

#### 5. Settlement & Resolution Payout & Trust Model

- **Paper Trading Trust Model**: BayesMarket is an open-source paper-trading and educational simulation platform. Market resolutions are admin-mediated via authorized credentials (`ADMIN_TOKEN`). Unlike decentralized oracles (e.g., UMA or Chainlink), the administrator or automated ingest pipeline is trusted to authoritatively declare market outcomes based on real-world evidence.
- **Cryptographic Proof Hashing & Audit Digest**:
    - Every resolution requires verifiable `oracle_proof` documentation (e.g., official bureau reports, market index settlement prices, regulatory filings).
    - The server computes a SHA-256 hash of the proof (`proof_hash = SHA256(oracle_proof)`).
    - An atomic settlement payload digest is generated: `settlement_digest = SHA256(market_id : winning_outcome : total_payout : proof_hash : resolved_at)`.
    - Resolutions emit structured audit logs (`[AUTHORITATIVE RESOLUTION AUDIT]`) and attribute actions deterministically to the System Administrator identity (`00000000-0000-0000-0000-000000000001`), ensuring all settlements are auditable in immutable double-entry ledger entries.
    - An in-memory resolution cache enforces proof integrity, rejecting any subsequent calls with conflicting outcomes or divergent proof hashes.
- **Collateral & Share Redemption**:
    - Each winning share is unconditionally redeemable for **$1.00 USDC** from the market collateral reserve.
    - Losing shares expire at **$0.00 USDC**.
    - Payout = $\text{Shares}_{\text{winning}} \times 1.00$; settlement executes in a `SERIALIZABLE` database transaction, crediting winning cash balances, zeroing winning/losing positions, and debiting the pool collateral reserve.

---

## 4. Monorepo Structure (`bayesmarket/`)

```
bayesmarket/
├── target/
│   ├── AGENTs.md                 # System Master Context & Operational Playbook
│   ├── ARCHITECTURE.md           # Primary System Architecture Specification
│   ├── DESIGN.md                 # UI/UX Design System & WCAG 2.2 AAA Specification
│   └── TODOs.md                  # 9-Phase Implementation Task Board & Verification Gates
├── .env.example                  # Environment Configuration Template (Neon Database)
├── .prettierrc                  # Workspace Formatting Configuration
├── Makefile                     # Developer Commands (make test, make dev, make seed)
├── turbo.json                   # Turborepo Pipeline Configuration
├── pnpm-workspace.yaml          # Monorepo Workspace Definitions
│
├── backend/                     # GO BACKEND MICROSERVICE
│   ├── cmd/
│   │   └── api/
│   │       └── main.go          # Application Entry Point & Dependency Injection
│   ├── internal/
│   │   ├── amm/                 # PURE FINANCIAL AMM DOMAIN
│   │   │   ├── engine.go        # Pure Math: QuoteBuy, QuoteSell, ApplyTrade
│   │   │   ├── engine_test.go   # 100% Coverage Financial Unit Tests
│   │   │   └── types.go         # Domain Structs & Decimal Types
│   │   ├── config/              # Environment & Configuration Loader
│   │   │   └── config.go
│   │   ├── database/            # PERSISTENCE LAYER
│   │   │   ├── db.go            # Pgx Connection Pool & Health Check
│   │   │   ├── migrations/      # Sequential SQL Migrations
│   │   │   │   ├── 000001_init_schema.up.sql
│   │   │   │   └── 000001_init_schema.down.sql
│   │   │   ├── queries.go       # SQL Query Execution & Row Scanners
│   │   │   └── seed.go          # Realistic Pre-seeded Markets & Liquidity
│   │   ├── middleware/          # CROSS-CUTTING CONCERNS
│   │   │   ├── auth.go          # JWT & Guest Session Extractor
│   │   │   ├── ratelimit.go     # In-Memory Token-Bucket Rate Limiter
│   │   │   ├── cors.go          # Production CORS Origin Validator
│   │   │   └── logger.go        # Structured JSON Request Logger & Trace IDs
│   │   └── transport/           # TRANSPORT ADAPTERS
│   │       ├── rest/
│   │       │   ├── auth_handler.go      # Guest Onboarding & Faucet
│   │       │   ├── market_handler.go    # Market Catalog & Detail
│   │       │   ├── trade_handler.go     # Quotes & Trade Execution
│   │       │   ├── portfolio_handler.go # User Positions & PnL
│   │       │   └── admin_handler.go     # Market Resolution & Settlement
│   │       └── ws/
│   │           ├── hub.go               # WebSocket Hub & Topic Multiplexer
│   │           ├── client.go            # Goroutine Read/Write Pumps
│   │           └── simulator.go         # Background Ticker (Simulated Market Activity)
│   ├── go.mod
│   └── go.sum
│
└── frontend/                    # ANGULAR 22+ FRONTEND CLIENT
    ├── src/
    │   ├── app/
    │   │   ├── app.config.ts    # Application Config (Zoneless, Router, HTTP)
    │   │   ├── app.routes.ts    # Route Definitions
    │   │   ├── app.component.ts # Root Layout Shell
    │   │   ├── core/
    │   │   │   ├── guards/
    │   │   │   │   └── guest.guard.ts        # Auto-provision Guest Session
    │   │   │   ├── interceptors/
    │   │   │   │   └── auth.interceptor.ts   # Attach Bearer Token to Requests
    │   │   │   ├── models/
    │   │   │   │   ├── market.model.ts       # Shared TypeScript Models
    │   │   │   │   ├── trade.model.ts
    │   │   │   │   └── user.model.ts
    │   │   │   └── services/
    │   │   │       ├── api.service.ts        # Typed REST Client
    │   │   │       ├── ws.service.ts         # Reconnecting WebSocket Engine
    │   │   │       └── amm-math.service.ts   # Instant Client-side Slippage Quotes
    │   │   ├── state/
    │   │   │   ├── guest.store.ts            # Signal Store: Balance & Session
    │   │   │   ├── market.store.ts           # Signal Store: Active Market & Tickers
    │   │   │   └── portfolio.store.ts        # Signal Store: Positions & Unrealized PnL
    │   │   ├── features/
    │   │   │   ├── navbar/                   # Logo, Faucet, Balance Pill, Guest Badge
    │   │   │   ├── market-list/              # Catalog Grid, Category Filter, Search
    │   │   │   ├── market-detail/
    │   │   │   │   ├── market-header/        # Probability Badge & Resolution Rules
    │   │   │   │   ├── price-chart/          # Lightweight Charts Canvas Component
    │   │   │   │   ├── order-terminal/       # Buy/Sell Toggle, Slippage Warning, Buy Button
    │   │   │   │   └── activity-feed/        # Live Streaming Micro-trade Log
    │   │   │   ├── portfolio/                # Open Positions Table, Cash Out Modal
    │   │   │   └── admin-resolve/            # Admin Market Resolution Modal
    │   │   └── shared/
    │   │       ├── components/               # Modal, Badge, Button, Input, Skeleton
    │   │       └── styles/
    │   │           ├── tokens.css            # Dark Mode Color Palette & Variables
    │   │           └── base.css              # Global Typography & Reset
    ├── angular.json
    ├── package.json
    └── tsconfig.json
```

---

## 5. Client Routes & Navigation Mapping

| Route Path       | Component View          | Guard / Pre-condition    | Purpose                                                                                                                |
| :--------------- | :---------------------- | :----------------------- | :--------------------------------------------------------------------------------------------------------------------- |
| `/`              | `MarketListComponent`   | `GuestGuard`             | Landing page, category filters (AI, Crypto, Tech, Macro), trending markets grid, real-time volume counters.            |
| `/markets/:id`   | `MarketDetailComponent` | `GuestGuard`             | Deep-dive market view: Interactive probability canvas, order terminal, resolution criteria, recent trade stream.       |
| `/portfolio`     | `PortfolioComponent`    | `GuestGuard`             | User position ledger: Current shares, average buy price, current market value, unrealized PnL, "Sell/Cash Out" action. |
| `/admin/markets` | `AdminResolveComponent` | Protected (Admin Secret) | Administrative interface to manually resolve markets to YES or NO to finalize payouts and trigger winnings claims.     |
| `**`             | `NotFoundComponent`     | None                     | Fallback 404 page with navigation back to market catalog.                                                              |

### 5.1 Reactive State Management via Angular Signals

BayesMarket avoids heavy state libraries (like NgRx or Redux) in favor of Angular 22 native Signal Stores and computed derivations for low-latency reactive rendering:

```typescript
// Financial API fields are canonical decimal strings. The client never performs
// financial arithmetic; it requests authoritative server quotes and formats strings.
type DecimalString = string;

// 1. GuestStore (state/guest.store.ts)
export const GuestStore = signalStore(
    { providedIn: 'root' },
    withState({
        user: signal<User | null>(null),
        balance: signal<DecimalString>('1000.00000000'),
        faucetEligible: signal<boolean>(false),
        isLoading: signal<boolean>(false)
    }),
    withComputed(({ user, balance, faucetEligible }) => ({
        isGuest: computed(() => user()?.isGuest ?? true),
        formattedBalance: computed(() => formatUsdc(balance())),
        canClaimFaucet: computed(() => faucetEligible())
    }))
);

// 2. MarketStore (core/state/market.store.ts)
export const MarketStore = signalStore(
    { providedIn: 'root' },
    withState({
        activeMarket: signal<MarketDetail | null>(null),
        priceHistory: signal<PricePoint[]>([])
    }),
    withComputed(({ activeMarket }) => ({
        yesProbability: computed(() => activeMarket()?.yesProbability ?? '0.50000000'),
        noProbability: computed(() => activeMarket()?.noProbability ?? '0.50000000'),
        formattedYesProbability: computed(() => formatProbability(activeMarket()?.yesProbability ?? '0.50000000'))
    }))
);

// 3. TradingStore (core/state/trading.store.ts)
export const TradingStore = signalStore(
    { providedIn: 'root' },
    withState({
        selectedOutcome: signal<'YES' | 'NO'>('YES'),
        tradeAmount: signal<DecimalString>('50.00000000'),
        maxSlippagePercent: signal<DecimalString>('2.00000000'),
        isSubmitting: signal<boolean>(false)
    }),
    withComputed(({ selectedOutcome, tradeAmount }) => ({
        quoteRequest: computed(() => ({ amountUsdc: tradeAmount(), outcome: selectedOutcome() }))
    }))
);
```

---

## 6. Entity Relationship Diagram & Database Schema (PostgreSQL 18)

### 6.1 Mermaid Entity-Relationship Diagram (ERD)

```mermaid
erDiagram
    USERS ||--o{ USER_POSITIONS : "holds"
    USERS ||--o{ TRADES : "executes"
    USERS ||--o{ FAUCET_CLAIMS : "requests"
    USERS ||--o{ LEDGER_ENTRIES : "accounts"
    USERS ||--o{ IDEMPOTENCY_KEYS : "deduplicates"

    MARKETS ||--|| LIQUIDITY_POOLS : "governed by"
    MARKETS ||--o{ USER_POSITIONS : "denominates"
    MARKETS ||--o{ TRADES : "records"
    MARKETS ||--o{ LEDGER_ENTRIES : "reconciles"

    USERS {
        uuid id PK
        boolean is_guest
        numeric cash_balance
        varchar ip_address
        timestamp created_at
        timestamp last_active
    }

    MARKETS {
        uuid id PK
        varchar slug UK
        varchar title
        text description
        varchar category
        varchar status
        varchar winning_outcome
        timestamp resolution_date
    }

    LIQUIDITY_POOLS {
        uuid market_id PK, FK
        numeric reserve_yes
        numeric reserve_no
        numeric collateral_reserve
        numeric k_invariant
        numeric total_volume_usdc
        int lock_version
        timestamp updated_at
    }

    USER_POSITIONS {
        uuid id PK
        uuid user_id FK
        uuid market_id FK
        varchar outcome
        numeric shares_owned
        numeric avg_buy_price
        numeric total_invested_usdc
    }

    TRADES {
        uuid id PK
        uuid market_id FK
        uuid user_id FK
        varchar trade_type
        varchar outcome
        numeric amount_usdc
        numeric shares_filled
        numeric execution_price
        numeric price_impact_pct
        timestamp created_at
    }

    LEDGER_ENTRIES {
        uuid id PK
        uuid transaction_id
        uuid user_id FK
        uuid market_id FK
        varchar account
        varchar asset
        numeric delta
        varchar entry_type
        timestamp created_at
    }

    IDEMPOTENCY_KEYS {
        uuid actor_id FK
        varchar operation
        varchar idempotency_key
        jsonb response
        timestamp created_at
    }

    FAUCET_CLAIMS {
        uuid id PK
        uuid user_id FK
        varchar ip_address
        numeric amount
        timestamp claimed_at
    }
```

---

### 6.2 PostgreSQL 18 DDL Script

```sql
-- Enable cryptographic extension for UUID generation
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- 1. USERS, GUEST SESSIONS & GOOGLE OAUTH
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    is_guest BOOLEAN NOT NULL DEFAULT true,
    cash_balance NUMERIC(28, 8) NOT NULL DEFAULT 1000.00000000, -- Seeded with $1,000.00 USDC
    email VARCHAR(255) NULL,
    name VARCHAR(255) NULL,
    avatar_url TEXT NULL,
    google_id VARCHAR(255) NULL,
    auth_provider VARCHAR(50) NOT NULL DEFAULT 'guest',          -- 'guest' | 'google'
    ip_address VARCHAR(45),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_active TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_positive_balance CHECK (cash_balance >= 0)
);

CREATE UNIQUE INDEX idx_users_email_unique ON users(email) WHERE email IS NOT NULL;
CREATE UNIQUE INDEX idx_users_google_id_unique ON users(google_id) WHERE google_id IS NOT NULL;

-- 2. PREDICTION MARKETS
CREATE TABLE markets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(128) NOT NULL UNIQUE,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    category VARCHAR(64) NOT NULL,            -- 'ai', 'crypto', 'macro', 'space'
    image_url VARCHAR(512),
    resolution_source TEXT NOT NULL,           -- Criteria / URL defining resolution
    resolution_date TIMESTAMP WITH TIME ZONE NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active', -- 'draft', 'active', 'suspended', 'locked', 'resolved', 'settled'
    winning_outcome VARCHAR(8),                -- 'YES', 'NO', NULL
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 3. LIQUIDITY POOLS (CPMM Invariant State)
CREATE TABLE liquidity_pools (
    market_id UUID PRIMARY KEY REFERENCES markets(id) ON DELETE CASCADE,
    reserve_yes NUMERIC(28, 8) NOT NULL,       -- R_yes virtual inventory
    reserve_no NUMERIC(28, 8) NOT NULL,        -- R_no virtual inventory
    collateral_reserve NUMERIC(28, 8) NOT NULL, -- USDC backing issued complete sets
    k_invariant NUMERIC(56, 16) NOT NULL,      -- Invariant k = R_yes * R_no
    total_volume_usdc NUMERIC(28, 8) NOT NULL DEFAULT 0.00000000,
    lock_version INT NOT NULL DEFAULT 0,       -- Optimistic concurrency versioning
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_positive_reserves CHECK (reserve_yes > 0 AND reserve_no > 0),
    CONSTRAINT chk_nonnegative_collateral CHECK (collateral_reserve >= 0)
);

-- 4. USER OUTCOME POSITIONS
CREATE TABLE user_positions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    market_id UUID NOT NULL REFERENCES markets(id) ON DELETE CASCADE,
    outcome VARCHAR(8) NOT NULL,               -- 'YES' or 'NO'
    shares_owned NUMERIC(28, 8) NOT NULL DEFAULT 0.00000000,
    avg_buy_price NUMERIC(28, 8) NOT NULL DEFAULT 0.00000000,
    total_invested_usdc NUMERIC(28, 8) NOT NULL DEFAULT 0.00000000,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_user_market_outcome UNIQUE (user_id, market_id, outcome),
    CONSTRAINT chk_positive_shares CHECK (shares_owned >= 0)
);

-- 5. ORDER & TRADE EXECUTION AUDIT LOG
CREATE TABLE trades (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    market_id UUID NOT NULL REFERENCES markets(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    idempotency_key VARCHAR(128) NOT NULL,
    trade_type VARCHAR(8) NOT NULL,            -- 'BUY' or 'SELL'
    outcome VARCHAR(8) NOT NULL,               -- 'YES' or 'NO'
    amount_usdc NUMERIC(28, 8) NOT NULL,
    shares_filled NUMERIC(28, 8) NOT NULL,
    execution_price NUMERIC(28, 8) NOT NULL,   -- Effective price per share
    price_impact_pct NUMERIC(16, 8) NOT NULL,  -- Calculated slippage
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_trades_user_idempotency UNIQUE (user_id, idempotency_key)
);

-- 6. IMMUTABLE FINANCIAL LEDGER
CREATE TABLE ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL,
    user_id UUID REFERENCES users(id) ON DELETE RESTRICT,
    market_id UUID REFERENCES markets(id) ON DELETE RESTRICT,
    account VARCHAR(32) NOT NULL,              -- 'user_cash', 'pool_collateral', 'position_yes', 'position_no'
    asset VARCHAR(8) NOT NULL,                 -- 'USDC', 'YES', 'NO'
    delta NUMERIC(28, 8) NOT NULL,
    entry_type VARCHAR(32) NOT NULL,           -- 'trade', 'faucet', 'settlement', 'refund'
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 7. MUTATION IDEMPOTENCY RECEIPTS
CREATE TABLE idempotency_keys (
    actor_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    operation VARCHAR(64) NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL,
    response JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (actor_id, operation, idempotency_key)
);

-- 8. FAUCET CLAIMS (Abuse Prevention Ledger)
CREATE TABLE faucet_claims (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ip_address VARCHAR(45) NOT NULL,
    amount NUMERIC(28, 8) NOT NULL DEFAULT 500.00000000,
    claimed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- HIGH-PERFORMANCE INDEXES
CREATE INDEX idx_markets_status_category ON markets(status, category);
CREATE INDEX idx_trades_market_created ON trades(market_id, created_at DESC);
CREATE INDEX idx_trades_user ON trades(user_id, created_at DESC);
CREATE INDEX idx_positions_user ON user_positions(user_id);
CREATE INDEX idx_ledger_entries_transaction ON ledger_entries(transaction_id);
CREATE INDEX idx_faucet_ip_time ON faucet_claims(ip_address, claimed_at DESC);
```

---

## 7. API Specifications & Error Taxonomy

### 7.1 Standardized Financial Error Envelope

All API endpoints strictly return a structured error response with machine-readable error codes and correlation trace IDs:

All monetary values, share quantities, prices, percentages, reserves, and collateral values are encoded as fixed-scale decimal **strings** in JSON. Timestamps and non-financial counters may remain numbers.

```json
{
    "success": false,
    "error": {
        "code": "ERR_SLIPPAGE_EXCEEDED",
        "message": "Execution slippage 3.2% exceeded maximum tolerance of 2.0%",
        "details": {
            "quoted_price": "0.72000000",
            "actual_price": "0.74300000",
            "max_slippage_pct": "2.00000000"
        },
        "trace_id": "tr_8f8b1c4e28b9"
    }
}
```

#### Financial Error Taxonomy

| Error Code                   | HTTP Status             | Description                                                          |
| :--------------------------- | :---------------------- | :------------------------------------------------------------------- |
| `ERR_INSUFFICIENT_FUNDS`     | `400 Bad Request`       | User balance is less than required trade amount.                     |
| `ERR_SLIPPAGE_EXCEEDED`      | `400 Bad Request`       | Price shifted beyond `max_slippage_pct` between quote and execution. |
| `ERR_MINIMUM_TRADE_SIZE`     | `400 Bad Request`       | Trade amount is below the minimum threshold ($0.000001 USDC).        |
| `ERR_POOL_RESERVE_EXHAUSTED` | `400 Bad Request`       | Trade would drain reserves below safety threshold ($100.00 USDC).    |
| `ERR_MARKET_NOT_TRADABLE`    | `409 Conflict`          | Market is locked, suspended, or already resolved.                    |
| `ERR_RATE_LIMIT_EXCEEDED`    | `429 Too Many Requests` | Token-bucket limit exceeded. Includes `Retry-After` header.          |
| `ERR_FAUCET_COOLDOWN`        | `429 Too Many Requests` | Claim submitted before 5-minute cooldown expired.                    |

---

### 7.2 REST Endpoints

#### 1. Ephemeral Guest Session Initialization

- **Endpoint**: `POST /api/v1/auth/guest`
- **Response `200 OK`**:

```json
{
    "success": true,
    "data": {
        "user_id": "8f8b1c4e-28b9-4673-9a3b-74b83fa61021",
        "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
        "balance": "1000.00000000",
        "is_guest": true,
        "expires_at": 1726264800
    }
}
```

#### 2. Claim Faucet Funds

- **Endpoint**: `POST /api/v1/faucet`
- **Headers**: `Authorization: Bearer <token>`
- **Rules**: Allowed if `cash_balance < 200.00000000` and last claim was `> 5 minutes` ago.
- **Response `200 OK`**:

```json
{
    "success": true,
    "data": {
        "claimed_amount": "500.00000000",
        "new_balance": "1500.00000000"
    }
}
```

#### 3. List All Markets (Explore / Catalog)

- **Endpoint**: `GET /api/v1/markets`
- **Query Params**: `category=ai|crypto|all`, `status=active|resolved`, `limit=20`
- **Response `200 OK`**:

```json
{
    "success": true,
    "data": [
        {
            "id": "2d193798-75c1-4b19-bf93-9c7625807914",
            "slug": "will-openai-release-gpt-5-in-2026",
            "title": "Will OpenAI release GPT-5 before December 2026?",
            "category": "ai",
            "image_url": "/assets/markets/gpt5.webp",
            "yes_price": "0.72000000",
            "no_price": "0.28000000",
            "yes_probability": "0.72000000",
            "no_probability": "0.28000000",
            "total_volume_usdc": "14250.00000000",
            "resolution_date": "2026-12-31T23:59:59Z",
            "status": "active"
        }
    ]
}
```

#### 4. Market Detail & Liquidity Pool Inspection

- **Endpoint**: `GET /api/v1/markets/:id`
- **Response `200 OK`**:

```json
{
    "success": true,
    "data": {
        "id": "2d193798-75c1-4b19-bf93-9c7625807914",
        "slug": "will-openai-release-gpt-5-in-2026",
        "title": "Will OpenAI release GPT-5 before December 2026?",
        "description": "Resolves to YES if OpenAI officially releases or announces general commercial availability of GPT-5 on or before December 31, 2026, 23:59:59 UTC.",
        "category": "ai",
        "image_url": "/assets/markets/gpt5.webp",
        "resolution_source": "https://openai.com/blog / official press release",
        "resolution_date": "2026-12-31T23:59:59Z",
        "status": "active",
        "yes_price": "0.72000000",
        "no_price": "0.28000000",
        "total_volume_usdc": "14250.00000000",
        "pool": {
            "reserve_yes": "7000.00000000",
            "reserve_no": "18000.00000000",
            "collateral_reserve": "25000.00000000",
            "k_invariant": "126000000.0000000000000000"
        }
    }
}
```

#### 5. Pre-Trade Quote (Slippage & Return Calculation)

- **Endpoint**: `POST /api/v1/markets/:id/quote`
- **Payload**:

```json
{
    "outcome": "YES",
    "amount_usdc": "100.00000000"
}
```

- **Response `200 OK`**:

```json
{
    "success": true,
    "data": {
        "expected_shares": "138.67403315",
        "average_price": "0.72111554",
        "initial_price": "0.72000000",
        "price_impact_pct": "0.30949249",
        "potential_payout_usdc": "138.67403315",
        "potential_return_pct": "38.67403315"
    }
}
```

#### 6. Execute Trade (Atomic Order Placement)

- **Endpoint**: `POST /api/v1/markets/:id/orders`
- **Headers**: `Authorization: Bearer <token>` and `Idempotency-Key: <opaque-client-key>`
- **Payload**:

```json
{
    "outcome": "YES",
    "amount_usdc": "100.00000000",
    "max_slippage_pct": "2.50000000"
}
```

- **Response `200 OK`**:

```json
{
    "success": true,
    "data": {
        "trade_id": "6c459cfa-dfc5-4bc2-8eb1-5ef621f38012",
        "shares_filled": "138.67403315",
        "execution_price": "0.72111554",
        "remaining_balance": "900.00000000",
        "timestamp": "2026-09-06T23:50:12Z"
    }
}
```

#### 7. User Portfolio & Positions

- **Endpoint**: `GET /api/v1/portfolio`
- **Headers**: `Authorization: Bearer <token>`
- **Response `200 OK`**:

```json
{
    "success": true,
    "data": {
        "balance_usdc": "900.00000000",
        "portfolio_value_usdc": "98.62745098",
        "total_unrealized_pnl": "-1.37254902",
        "positions": [
            {
                "market_id": "2d193798-75c1-4b19-bf93-9c7625807914",
                "market_title": "Will OpenAI release GPT-5 before December 2026?",
                "outcome": "YES",
                "shares_owned": "138.67403315",
                "avg_buy_price": "0.72111554",
                "current_price": "0.72000000",
                "current_value": "99.84530387",
                "unrealized_pnl": "-0.15469613",
                "unrealized_pnl_pct": "-0.15469613"
            }
        ]
    }
}
```

#### 8. Cash Out and Admin Market Resolution

- **Cash-out endpoint**: `POST /api/v1/portfolio/cashout`, with bearer authentication and an `Idempotency-Key`; it accepts decimal-string shares and returns an authoritative sell quote/receipt.
- **Resolution endpoint**: `POST /api/v1/admin/markets/:id/resolve`.
- **Headers**: `Authorization: Bearer <admin-token>` and `Idempotency-Key: <opaque-client-key>`.
- **Payload**: `{ "winning_outcome": "YES", "oracle_proof": "immutable source reference" }`.
- **Response `200 OK`**: Locks the market, records the resolution proof, and atomically distributes $1.00 per winning share with immutable ledger entries.

---

### 7.3 WebSocket Real-Time Protocol

- **URL**: `wss://<host>/ws/markets/:id`
- **Handshake**: Upgrades the standard HTTP request using `github.com/gorilla/websocket`.
- **Heartbeat**: Ping/pong heartbeat every 30 seconds to maintain edge gateway tunnels and detect disconnected clients.
- **Broadcast Frames**:

#### 1. `PRICE_UPDATE` Frame

Broadcast whenever an order fills and shifts the CPMM invariant:

```json
{
    "type": "PRICE_UPDATE",
    "market_id": "2d193798-75c1-4b19-bf93-9c7625807914",
    "yes_price": "0.72000000",
    "no_price": "0.28000000",
    "reserves": {
        "yes": "7000.00000000",
        "no": "18000.00000000"
    },
    "total_volume_usdc": "14250.00000000",
    "timestamp": 1725660000
}
```

#### 2. `TRADE_EVENT` / `RECENT_TRADE` Frame

Broadcast on every confirmed trade to populate real-time ticker feeds and TradingView chart bars:

```json
{
    "type": "TRADE_EVENT",
    "market_id": "2d193798-75c1-4b19-bf93-9c7625807914",
    "trade_type": "BUY",
    "outcome": "YES",
    "shares": "138.67403315",
    "price": "0.72111554",
    "amount_usdc": "100.00000000",
    "timestamp": "2026-09-06T23:50:12Z"
}
```

#### 3. Simulated Market Ticker Worker (`internal/transport/ws/simulator.go`)

- In development/sandbox environments, a background worker simulates market activity by generating micro-trades every **5–15 seconds** from simulated liquidity accounts.
- Guarantees live chart movements, real-time candlestick rendering, and active WebSocket testing without manual trade submission.

---

## 8. Concurrency, Race Conditions & CAP Theorem Analysis

### 8.1 The Two-Tier Concurrency Architecture

```mermaid
flowchart TD
    ReqA[Concurrent Trade Request A] --> Gateway[Go API Gateway]
    ReqB[Concurrent Trade Request B] --> Gateway

    Gateway --> Tier1["Tier 1: In-Memory Mutex per Market<br/>sync.Mutex in Go Hub"]
    Tier1 -->|Serializes within Node| Tier2["Tier 2: Row-Level Locking in PostgreSQL 18<br/>SELECT ... FOR UPDATE on liquidity_pools"]

    Tier2 --> Step1[Read Fresh Reserves]
    Step1 --> Step2[Compute AMM Math]
    Step2 --> Step3[Update Reserves & Deduct Balance]
    Step3 --> Step4[Commit DB Transaction]
    Step4 --> ReleaseLock[Release Lock & Broadcast WS Update]
```

1. **Tier 1 (In-Memory Go Mutex per Market)**:
   A thread-safe registry of `sync.Mutex` instances keyed by `market_id`. Requests for the same market within the Go service are serialized before hitting the database, eliminating unnecessary PostgreSQL lock contention.
2. **Tier 2 (PostgreSQL `SELECT ... FOR UPDATE`)**:
   Enforces ACID transaction guarantees at the database engine level:
    ```sql
    BEGIN;
    SELECT reserve_yes, reserve_no, k_invariant
    FROM liquidity_pools
    WHERE market_id = $1
    FOR UPDATE;
    -- Reserves locked exclusively until COMMIT or ROLLBACK
    UPDATE liquidity_pools SET reserve_yes = $2, reserve_no = $3 WHERE market_id = $1;
    COMMIT;
    ```

### 8.2 CAP Theorem Positioning

- **BayesMarket is a CP (Consistency + Partition Tolerance) System**.
- In financial market trading, executing a trade at an incorrect price due to network latency or stale state is unacceptable. If network partitioning occurs between the Go gateway and PostgreSQL, the system **rejects** the order with an explicit `ERR_MARKET_UNAVAILABLE` rather than accepting an inconsistent trade.

---

## 9. Performance Benchmarks, Latency Budgets & Observability

### 9.1 Latency Budgets & SLA Targets

| Operation                         | Metric Target (SLA)                   | Optimization Strategy                                             |
| :-------------------------------- | :------------------------------------ | :---------------------------------------------------------------- |
| **In-Memory AMM Math**            | $< 50\mu\text{s}$                     | Zero allocation decimal arithmetic in Go.                         |
| **Order Execution (End-to-End)**  | $p50 < 3\text{ms}, p99 < 15\text{ms}$ | Prepared SQL statements, pgx connection pooling, row-level locks. |
| **Throughput Target**             | $> 1,500\text{ req/sec}$              | Single Go process on 1 vCPU with Goroutine worker pools.          |
| **WebSocket Broadcast Latency**   | $< 10\text{ms}$                       | Non-blocking channel fan-out to connected clients.                |
| **Client First Contentful Paint** | $< 400\text{ms}$                      | Zoneless Angular 22 with streaming SSR on Cloudflare Edge.        |

### 9.2 Observability & Telemetry (Prometheus & OpenTelemetry)

The Go service exposes a `/metrics` endpoint scraped by Prometheus:

- `bayes_trades_total{outcome="YES|NO", status="success|failed"}`: Trade throughput counter.
- `bayes_trade_latency_seconds`: Histogram measuring execution duration.
- `bayes_pool_reserves{market_id="...", token="YES|NO"}`: Real-time gauge of reserve balances.
- `bayes_ws_connected_clients`: Gauge of active WebSocket connections.

Every inbound request is assigned a unique `X-Trace-ID` propagated through Go `context.Context` to all SQL queries and structured log outputs.

---

## 10. Security, Privacy & Vulnerability Mitigation

| Threat Vector                         | Severity | Mitigation Architecture                                                                                                                                                                      |
| :------------------------------------ | :------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **API Drain / DDoS Attack**           | High     | **Token-Bucket Rate Limiter**: 60 req/min for reads; 12 req/min for trades per Guest ID and IP. Excess requests return HTTP 429 immediately before hitting database queries.                 |
| **Floating-Point Rounding Exploits**  | High     | **Decimal-Only Computation**: Zero `float64` operations in the trading path. All reserves and balances use `shopspring/decimal` fixed-point representations.                                 |
| **Infinite Division / Zero Reserves** | High     | **Boundary Clamping**: Market prices are restricted to the interval `[0.0100, 0.9900]`. If a trade would drive a reserve below minimum liquidity threshold ($100.00 USDC), execution aborts. |
| **Sybil Faucet Depletion**            | Medium   | **Dual-Key Faucet Lock**: Claims require both a minimum time cooldown (5 min) per IP address and an active user cash-balance check (`cash_balance < $200.00`).                               |
| **SQL Injection**                     | Critical | **Prepared Statements**: All database operations use parameter placeholders (`$1, $2`) via `pgx`. No dynamic string concatenation.                                                           |
| **Cross-Site Scripting (XSS)**        | High     | **Strict CSP & Sanitization**: Angular's default DOM sanitization prevents script injection; HTTP-Only cookie flags for JWT tokens prevent client-side credential theft.                     |

### 10.1 Token-Bucket Rate Limiter & Oracle Cache Policy

Implemented in Go middleware using `golang.org/x/time/rate`:

- **Public Read Endpoints** (`GET /api/v1/markets`, `GET /api/v1/markets/:id`):
    - Rate: **60 requests/minute** per client IP.
    - Burst: **10 requests**.
- **Trade Execution Endpoints** (`POST /api/v1/markets/:id/quote`, `POST /api/v1/markets/:id/orders`):
    - Rate: **12 requests/minute** per Guest ID / IP address.
    - Burst: **3 requests**.
    - Enforces backpressure against spamming the AMM engine and database row locks.
- **Faucet Claim Throttling** (`POST /api/v1/faucet`):
    - Max **1 claim per 5 minutes** per Guest ID / IP address.
    - Eligibility gate: Balance must be below `$200.00 USDC`.
- **External Oracle / 3rd-Party Data Cache**:
    - Any external market data (e.g., live crypto spot feeds or news APIs) is cached in-memory with a **60-second TTL**.
    - Client queries resolve strictly against the in-memory cache and never directly trigger outbound third-party API calls.

---

## 11. Ephemeral Guest Session Cleanup (TTL Pruning)

To keep database storage well within free-tier limits (Neon 0.5 GiB limit), a background Go ticker runs a daily cleanup routine:

```sql
-- Purge guest accounts inactive for more than 7 days
DELETE FROM users
WHERE is_guest = true
  AND last_active < NOW() - INTERVAL '7 days';
```

_(Cascading foreign keys automatically purge associated positions and faucet claim logs)._

---

## 12. 100% Free-Tier Deployment Strategy

```
[User Browser]
      │
      ▼
[Cloudflare Pages] (Angular 22 SPA)
      │
      ▼ (HTTPS REST / WSS)
[Render / Fly.io Container] (Go Static Binary, Port 8080)
      │
      ▼ (SSL pgx connection)
[Neon Serverless Postgres] (PostgreSQL 18)
```

- **Frontend**: Cloudflare Pages (Free, global CDN edge, zero cold starts).
- **Backend**: Fly.io / Render (Free container tier running the <18MB multi-stage Go scratch binary).
- **Database**: Neon Serverless PostgreSQL 18 (Free tier, auto-sleeps when idle, instant resume).

### 12.1 Environment Variables Configuration

| Variable       | Service | Description                                                   | Example Value                                                     |
| :------------- | :------ | :------------------------------------------------------------ | :---------------------------------------------------------------- |
| `DATABASE_URL` | Backend | PostgreSQL connection string with SSL mode enabled            | `postgres://user:pass@ep-cool-db.neon.tech/bayes?sslmode=require` |
| `SERVER_PORT`  | Backend | HTTP / WebSocket server listen port                           | `8080`                                                            |
| `CORS_ORIGIN`  | Backend | Permitted frontend origin                                     | `http://localhost:4200`                                           |
| `JWT_SECRET`   | Backend | Cryptographic HMAC-SHA256 secret salt for guest token signing | Generated secret; never committed                                 |

---

## 13. Pre-Seeded Default Markets

On database boot, `seed.go` executes idempotently to populate four realistic prediction markets:

1. **"Will OpenAI release GPT-5 before December 2026?"**  
   _Category_: AI & Tech | _Initial Odds_: 72% YES / 28% NO | _Virtual reserves_: YES 7,000 / NO 18,000 | _Collateral_: $25,000 USDC
2. **"Will Bitcoin exceed $125,000 in Q4 2026?"**  
   _Category_: Crypto | _Initial Odds_: 45% YES / 55% NO | _Virtual reserves_: YES 11,000 / NO 9,000 | _Collateral_: $20,000 USDC
3. **"Will the US Federal Reserve cut interest rates at the next FOMC?"**  
   _Category_: Macro / Finance | _Initial Odds_: 84% YES / 16% NO | _Virtual reserves_: YES 8,000 / NO 42,000 | _Collateral_: $50,000 USDC
4. **"Will SpaceX successfully land a Starship on Mars by 2027?"**  
   _Category_: Space & Science | _Initial Odds_: 31% YES / 69% NO | _Virtual reserves_: YES 6,900 / NO 3,100 | _Collateral_: $10,000 USDC

For every seed, the loader mints exactly the stated collateral amount of complete sets. Any initial inventory not held by the pool is assigned to a system liquidity-provider account, with matching ledger entries, so issued YES supply, issued NO supply, and collateral reconcile from the first transaction.

---

## 14. End-to-End Operational & Testing Workflow

1. **Backend & Database Initialization**:
    - Provision PostgreSQL schema via migrations: `make migrate-up` or run the database migration runner.
    - Start the Go API engine: `go run cmd/api/main.go`. Seed runner automatically initializes default liquidity pools.
2. **Frontend Client Startup**:
    - Launch the Angular client: `pnpm dev` or `npm start`.
    - Access the web interface at `http://localhost:4200`.
3. **Session Provisioning**:
    - Initial visit automatically issues an ephemeral guest JWT with 1,000 virtual USDC via `POST /api/v1/auth/guest`.
4. **Order Execution & Mathematical Integrity**:
    - Select an active prediction market and configure order sizing (e.g., 100 USDC on YES).
    - Real-time AMM quote computes spot price, slippage impact, and expected shares.
    - On submission, backend executes atomic row-level locked transaction in PostgreSQL and decrements user balance.
5. **Real-Time WebSocket Propagation**:
    - Broadcast hub dispatches price tick and recent trade record to all active subscribers.
    - TradingView canvas chart dynamically redraws candles/lines at 60fps without page reload.
6. **Portfolio Position Management**:
    - Navigate to `/portfolio` to inspect active share balances, weighted average cost basis, and real-time unrealized PnL.
    - Execute liquidation ("Cash Out") to redeem shares back into the pool.
