# BayesMarket

[![CI](https://github.com/Avinaba-Mazumdar/bayesmarket-oss/actions/workflows/ci.yml/badge.svg)](https://github.com/Avinaba-Mazumdar/bayesmarket-oss/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Angular](<https://img.shields.io/badge/Angular-22%20(Zoneless)-DD0031?logo=angular&logoColor=white>)](https://angular.dev)
[![WCAG AAA](https://img.shields.io/badge/WCAG%202.2-Level%20AAA-brightgreen)](target/DESIGN.md)

> **High-Performance Binary Prediction Market Exchange Engine & Reactive Trading Platform**

<p align="center">
  <a href="https://bayesmarket-oss.vercel.app"><b>🌐 Launch Live Application (Vercel)</b></a> &nbsp;•&nbsp;
  <a href="https://bayesmarket-oss.onrender.com/healthz"><b>⚡ Live Backend Health (Render)</b></a> &nbsp;•&nbsp;
  <a href="#quick-start-with-docker-1-command"><b>🐳 Docker Quickstart</b></a>
</p>

BayesMarket is a modern, open-source prediction market platform inspired by Polymarket. Users trade binary outcome shares (**YES** and **NO**) on real-world events.

Unlike traditional wagering apps, BayesMarket implements a **collateralized complete-set Constant Product Automated Market Maker (CPMM)**: every USDC deposited mints matched YES/NO shares, while an atomic PostgreSQL ledger tracks collateral, positions, and settlement. The platform also includes real-time **WebSocket broadcast pipelines** and a **zoneless Angular (Signals) frontend** featuring hardware-accelerated TradingView charts.

---

## Preview

|              Institutional Trading Terminal & Live Chart               |            Real-Time Market Catalog & Probability Split            |
| :--------------------------------------------------------------------: | :----------------------------------------------------------------: |
| ![BayesMarket Trading Terminal](docs/screenshots/trading_terminal.png) | ![BayesMarket Discovery Page](docs/screenshots/discovery_page.png) |

---

## Key Features

- **Collateralized Complete-Set CPMM**: Mathematically governed bonding curve where outcome probabilities sum to $1.00 ($P*{\text{YES}} + P*{\text{NO}} = 1.00$) and every winning share is backed by market collateral.
- **Fixed-Point Financial Math**: Arbitrary precision arithmetic via `shopspring/decimal` to eliminate IEEE-754 floating-point drift and rounding errors.
- **High-Concurrency Go Engine**: Low-latency order execution, row-level locking in PostgreSQL for atomic balances and pool invariants, and non-blocking channel fan-out.
- **Real-Time Streaming**: Push-based price updates and live trade activity via resilient, auto-reconnecting WebSockets.
- **Reactive Zoneless UI**: Built with Angular Signals, computed derivations, and `lightweight-charts` for 60fps canvas rendering without change-detection overhead.
- **Instant Sandbox Mode**: 1-click ephemeral Guest Session pre-loaded with 1,000 virtual USDC and faucet replenishment to test trading workflows without wallet connection or registration.
- **WCAG 2.2 Level AAA Compliant**: Rigorously engineered accessibility architecture featuring 7:1 enhanced contrast, 44×44px hit targets, full keyboard navigation, and two-step financial order confirmation review.
- **Abuse Mitigation**: In-memory token-bucket rate limiting per IP and session (`golang.org/x/time/rate`).

---

## System Architecture

```mermaid
flowchart TB
    subgraph Client ["Client Presentation Layer (Angular 22 SPA)"]
        direction TB
        UI["Angular Client<br/><i>(Zoneless + Signals State Store)</i>"]
        TV["TradingView Lightweight Charts<br/><i>(Hardware-Accelerated Canvas)</i>"]
        Terminal["Order Terminal & Slippage Estimator"]
    end

    subgraph Transport ["Network & Transport Protocols"]
        REST["REST API<br/><code>HTTPS / JSON</code>"]
        WS["WebSocket Channel<br/><code>WSS / Real-Time Streams</code>"]
    end

    subgraph Backend ["Go Backend Trading Engine (:8080)"]
        direction TB
        RL["Token-Bucket Rate Limiter<br/><i>(golang.org/x/time/rate)</i>"]
        AUTH["Session & Auth Context<br/><i>(1-Click Guest JWT)</i>"]
        AMM["Constant Product AMM Engine<br/><code>x * y = k Invariant</code>"]
        WSHub["WebSocket Broker Hub<br/><i>(Channel Multiplexer)</i>"]
    end

    subgraph Storage ["Persistence Layer"]
        DB[("PostgreSQL 18 Database<br/><i>Atomic Row-Level Locks & Ledger</i>")]
    end

    %% Client Interactions
    UI -->|Orders, Quotes, Faucet| REST
    REST --> RL
    RL --> AUTH
    AUTH --> AMM

    %% Engine Execution & Persistence
    AMM -->|Atomic TX / SELECT FOR UPDATE| DB
    AMM -->|Broadcast Price & Trade Events| WSHub

    %% Real-Time WebSocket Streaming
    WSHub --> WS
    WS -->|Live Price Ticks & Trades| TV
    WS -->|Order & Trade Events| UI
```

---

## Mathematical Specification (CPMM)

### Constant Product Invariant

For a market with reserves $R_{\text{YES}}$ and $R_{\text{NO}}$:
$$k = R_{\text{YES}} \times R_{\text{NO}}$$

### Implied Probability & Spot Pricing

$$P_{\text{YES}} = \frac{R_{\text{NO}}}{R_{\text{YES}} + R_{\text{NO}}}, \quad P_{\text{NO}} = \frac{R_{\text{YES}}}{R_{\text{YES}} + R_{\text{NO}}}$$
$$\text{Constraint: } P_{\text{YES}} + P_{\text{NO}} = 1.00$$

### Share Purchases

When depositing $\Delta USDC$ to purchase `YES` shares:

1. The deposit mints matched YES/NO complete sets and increases the collateral reserve.
2. Virtual liquidity becomes $R_{\text{NO}}' = R_{\text{NO}} + \Delta USDC$ and $R_{\text{YES}}' = \frac{k}{R_{\text{NO}}'}$.
3. Filled shares are $\Delta \text{YES} = R_{\text{YES}} + \Delta USDC - R_{\text{YES}}'$.
4. Effective price is $\bar{P} = \frac{\Delta USDC}{\Delta \text{YES}}$.

---

## Repository Structure

```
.
├── .env.example                  # Environment configuration template (Neon PostgreSQL)
├── .gitignore
├── .prettierrc
├── README.md
│
├── backend/                      # Go Backend Trading Engine
│   ├── cmd/
│   │   └── api/                  # Server entrypoint & graceful shutdown
│   ├── internal/
│   │   ├── amm/                  # Pure financial AMM engine & invariant math
│   │   ├── config/               # Environment & configuration loader
│   │   ├── database/             # PostgreSQL connection pool, migrations & seed data
│   │   ├── middleware/           # Rate limiting, auth, CORS & request logging
│   │   └── transport/            # REST endpoints & WebSocket multiplexer hub
│   ├── go.mod
│   └── go.sum
│
├── frontend/                     # Angular 22 Zoneless Trading Client
│   ├── src/
│   │   ├── app/
│   │   │   ├── core/             # REST/WebSocket services, guards & models
│   │   │   ├── features/         # Market detail, trading terminal, charts & portfolio
│   │   │   └── state/            # Angular Signal Stores (Guest, Market, Portfolio)
│   │   └── styles/               # Design tokens & WCAG 2.2 AAA styles
│   └── package.json
│
└── target/                       # Product Specifications & System Playbooks
    ├── AGENTs.md                 # System master context & coding invariants
    ├── ARCHITECTURE.md           # Technical architecture, database DDL & engine design
    ├── DESIGN.md                 # UI/UX design tokens & WCAG 2.2 AAA specification
    └── TODOs.md                  # 9-phase implementation task board & verification gates
```

---

## Quick Start with Docker (1-Command)

Run the entire platform locally — including PostgreSQL 18, automated database migrations, pre-calibrated prediction markets, Go trading engine, and Angular 22 frontend — with a single command:

```bash
# Clone the repository
git clone https://github.com/Avinaba-Mazumdar/bayesmarket-oss.git
cd bayesmarket-oss

# Boot all services via Docker Compose
docker compose up --build -d
```

Once initialized, open your browser:

- **Trading Application**: `http://localhost:4200`
- **Backend API & Health**: `http://localhost:8080/healthz`
- **Database**: `localhost:5432` (`bayesuser` / `bayespassword`)

---

## Manual Development Setup

If developing locally without Docker:

### Prerequisites

- **Go**: `v1.24+`
- **Node.js**: `v22+` (or `v24 LTS`)
- **pnpm**: `v10+` (or `npm`)
- **PostgreSQL**: Local PostgreSQL 18 or **[Neon.tech](https://neon.tech)**

### 1. Environment Configuration

```bash
cp .env.example .env
```

Set `DATABASE_URL` in `.env`:

```env
DATABASE_URL=postgres://user:password@ep-cool-pool-123456.us-east-2.aws.neon.tech/bayesmarket?sslmode=require
SERVER_PORT=8080
CORS_ORIGIN=http://localhost:4200
JWT_SECRET=your-secure-random-jwt-secret-key-at-least-32-chars
```

### 2. Backend Setup

```bash
cd backend
go mod download
go run cmd/api/main.go
```

The API server boots on `http://localhost:8080`. Database migrations and default seed markets initialize automatically.

### 3. Frontend Setup

```bash
cd frontend
pnpm install
pnpm dev
```

The web application will be accessible at `http://localhost:4200`.

---

## Engineering Decisions & Invariants

| Decision                               | Implementation                                           | Engineering Rationale                                                                                                                 |
| :------------------------------------- | :------------------------------------------------------- | :------------------------------------------------------------------------------------------------------------------------------------ |
| **Zero-Float Financial Math**          | `shopspring/decimal` fixed-point arithmetic              | Eliminates IEEE-754 floating-point drift across iterative buy/sell bonding curves.                                                    |
| **Balance Solvency & Race Prevention** | PostgreSQL `SELECT ... FOR UPDATE` row locks             | Enforces serializable isolation; 20 parallel threads attempting to drain the same balance yields exactly 1 success and 19 rejections. |
| **High-Performance Canvas Charts**     | TradingView Lightweight Charts + Angular Signals         | 60fps canvas rendering isolated from framework change-detection cycles.                                                               |
| **Non-Blocking WS Telemetry**          | Go channels with buffered fan-out and select-drop        | Slow network consumers are gracefully disconnected without stalling the central matching pipeline.                                    |
| **Institutional Accessibility**        | WCAG 2.2 Level AAA (7:1 contrast, 44×44px touch targets) | Dual-coded outcome indicators, 2-step financial confirmation modal, full keyboard navigability.                                       |

---

## Verification & Testing

### Running Backend Unit Tests

Execute the financial engine test suite to verify the CPMM invariant preservation and slippage bounds:

```bash
cd backend
go test -v -race ./internal/amm/...
```

---

## Documentation

- [System Architecture (target/ARCHITECTURE.md)](target/ARCHITECTURE.md): Deep-dive into database schema, concurrency controls, and state management.
- [Design System & WCAG 2.2 AAA Specification (target/DESIGN.md)](target/DESIGN.md): Institutional-grade UI tokens, typography, component specifications, and comprehensive accessibility verification.
- [AI Agent System Master & Coding Invariants (target/AGENTs.md)](target/AGENTs.md): System master persona, mathematical invariants, and Definition of Done.
- [Active Implementation Task Board (target/TODOs.md)](target/TODOs.md): 9-phase execution roadmap and user verification gates.

---

## License

This project is licensed under the MIT License - see the LICENSE file for details.
