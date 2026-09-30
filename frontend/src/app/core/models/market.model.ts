export interface MarketReserves {
    reserve_yes: string;
    reserve_no: string;
    collateral_reserve: string;
    total_volume_usdc: string;
}

export interface Market {
    id: string;
    slug: string;
    title: string;
    description: string;
    category: string;
    image_url?: string;
    resolution_source: string;
    resolution_date: string;
    status: 'active' | 'closed' | 'resolved';
    winning_outcome?: 'YES' | 'NO' | string;
    probability_yes: string;
    probability_no: string;
    probability_yes_pct: string;
    probability_no_pct: string;
    reserves: MarketReserves;
    created_at: string;
}

export interface QuoteRequest {
    action: 'BUY' | 'SELL';
    outcome: 'YES' | 'NO';
    amount_usdc?: string;
    shares?: string;
}

export interface BuyQuoteResponse {
    market_id: string;
    action: 'BUY';
    outcome: 'YES' | 'NO';
    deposit_usdc: string;
    shares_received: string;
    avg_price: string;
    initial_price: string;
    new_price: string;
    price_impact_pct: string;
    new_reserve_yes: string;
    new_reserve_no: string;
    new_collateral: string;
}

export interface PlaceOrderRequest {
    outcome: 'YES' | 'NO';
    amount_usdc: string;
    max_slippage_pct?: string;
}

export interface OrderResponse {
    trade_id: string;
    market_id: string;
    user_id: string;
    trade_type: string;
    outcome: 'YES' | 'NO';
    amount_usdc: string;
    shares_filled: string;
    execution_price: string;
    price_impact_pct: string;
    new_cash_balance: string;
    new_shares_owned: string;
    avg_buy_price: string;
    created_at: string;
}

export interface UserPosition {
    id?: string;
    market_id: string;
    market_slug?: string;
    market_title: string;
    category?: string;
    outcome: 'YES' | 'NO';
    shares_owned: string;
    avg_buy_price: string;
    total_invested_usdc?: string;
    current_price: string;
    current_value_usdc?: string;
    market_value?: string;
    unrealized_pnl_usdc?: string;
    unrealized_pnl?: string;
    unrealized_pnl_pct?: string;
}

export interface PortfolioResponse {
    user_id?: string;
    cash_balance_usdc?: string;
    cash_balance?: string;
    positions_value_usdc?: string;
    total_portfolio_value_usdc?: string;
    total_portfolio_value?: string;
    total_invested_usdc?: string;
    total_unrealized_pnl_usdc?: string;
    total_unrealized_pnl_pct?: string;
    positions: UserPosition[];
}

export interface CashOutRequest {
    market_id: string;
    outcome: 'YES' | 'NO';
    shares: string;
    min_payout_usdc?: string;
}

export interface CashOutResponse {
    trade_id: string;
    market_id: string;
    user_id: string;
    trade_type: string;
    outcome: 'YES' | 'NO';
    shares_sold: string;
    payout_usdc: string;
    execution_price: string;
    price_impact_pct: string;
    new_cash_balance: string;
    remaining_shares: string;
    created_at: string;
}

export interface ResolveMarketRequest {
    winning_outcome: 'YES' | 'NO';
    oracle_proof: string;
}

export interface ResolveMarketResponse {
    status: string;
    market_id: string;
    winning_outcome: 'YES' | 'NO';
    total_payout_usdc: string;
    winners_credited: number;
    oracle_proof: string;
    resolved_at: string;
}

export interface UserProfile {
    id: string;
    email?: string | null;
    name?: string | null;
    avatar_url?: string | null;
    is_guest: boolean;
    auth_provider: 'guest' | 'google';
    cash_balance: string;
    created_at: string;
}

export interface GoogleVerifyRequest {
    id_token: string;
    email?: string;
    name?: string;
}

export interface GoogleCallbackRequest {
    code: string;
    state?: string;
}

export interface AuthResponse {
    token: string;
    user: UserProfile;
}

export interface FaucetResponse {
    success?: boolean;
    amount_claimed?: string;
    new_balance?: string;
    cooldown_seconds?: number;
    amount?: string;
    user?: {
        id?: string;
        cash_balance?: string;
    };
    message?: string;
}

export interface CreateMarketRequest {
    title: string;
    description: string;
    category: string;
    resolution_source: string;
    resolution_date: string;
    image_url?: string;
    initial_collateral_usdc?: string;
    initial_probability_yes?: string;
}

export interface CreateMarketResponse {
    id: string;
    slug: string;
    title: string;
    description: string;
    category: string;
    image_url?: string;
    resolution_source: string;
    resolution_date: string;
    status: string;
    reserve_yes: string;
    reserve_no: string;
    collateral_reserve: string;
    k_invariant: string;
    probability_yes: string;
    probability_no: string;
    probability_yes_pct: string;
    probability_no_pct: string;
    created_at: string;
}

export interface AppConfig {
    app_env: string;
    is_dev: boolean;
    google_auth_enabled: boolean;
}
