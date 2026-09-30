import { inject, Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { catchError, Observable, of } from 'rxjs';
import {
    AppConfig,
    AuthResponse,
    BuyQuoteResponse,
    CashOutRequest,
    CashOutResponse,
    CreateMarketRequest,
    CreateMarketResponse,
    FaucetResponse,
    GoogleCallbackRequest,
    GoogleVerifyRequest,
    Market,
    OrderResponse,
    PlaceOrderRequest,
    PortfolioResponse,
    QuoteRequest,
    ResolveMarketRequest,
    ResolveMarketResponse,
    UserProfile
} from '../models/market.model';

@Injectable({
    providedIn: 'root'
})
export class ApiService {
    private readonly http = inject(HttpClient);
    private readonly baseUrl = this.resolveBaseUrl();

    private resolveBaseUrl(): string {
        if (typeof window !== 'undefined' && window.location) {
            if (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1') {
                return `${window.location.protocol}//${window.location.hostname}:8080/api/v1`;
            }
            return `${window.location.origin}/api/v1`;
        }
        return 'http://localhost:8080/api/v1';
    }

    /**
     * Helper to construct HttpHeaders with optional Bearer auth and Idempotency-Key.
     */
    private buildHeaders(token?: string | null, idempotencyKey?: string): HttpHeaders {
        let headers = new HttpHeaders({ 'Content-Type': 'application/json' });
        if (token) {
            headers = headers.set('Authorization', `Bearer ${token}`);
        }
        if (idempotencyKey) {
            headers = headers.set('Idempotency-Key', idempotencyKey);
        }
        return headers;
    }

    /**
     * Create or retrieve an ephemeral guest trading session.
     */
    createGuestSession(): Observable<AuthResponse> {
        return this.http.post<AuthResponse>(`${this.baseUrl}/auth/guest`, {});
    }

    /**
     * Authenticate via Google OAuth ID token (GIS / One Tap or simulation).
     * Optionally passes existing guest token to upgrade session.
     */
    verifyGoogleToken(request: GoogleVerifyRequest, guestToken?: string | null): Observable<AuthResponse> {
        const headers = this.buildHeaders(guestToken);
        return this.http.post<AuthResponse>(`${this.baseUrl}/auth/google/verify`, request, { headers });
    }

    /**
     * Retrieve the Google OAuth 2.0 authorization URL.
     */
    getGoogleAuthUrl(): Observable<{ url: string; state?: string; simulated: boolean; message?: string }> {
        return this.http.get<{ url: string; state?: string; simulated: boolean; message?: string }>(`${this.baseUrl}/auth/google/url`);
    }

    /**
     * Exchange OAuth 2.0 authorization code and state for a user session.
     */
    callbackGoogleAuth(request: GoogleCallbackRequest, guestToken?: string | null): Observable<AuthResponse> {
        const headers = this.buildHeaders(guestToken);
        return this.http.post<AuthResponse>(`${this.baseUrl}/auth/google/callback`, request, { headers });
    }

    /**
     * Get the authenticated user's profile and current balances.
     */
    getCurrentUser(token: string): Observable<UserProfile> {
        const headers = this.buildHeaders(token);
        return this.http.get<UserProfile>(`${this.baseUrl}/auth/me`, { headers });
    }

    /**
     * Fetch active prediction markets, optionally filtered by category.
     */
    getMarkets(category?: string): Observable<Market[]> {
        const url = category && category !== 'all' ? `${this.baseUrl}/markets?category=${encodeURIComponent(category)}` : `${this.baseUrl}/markets`;
        return this.http.get<Market[]>(url);
    }

    /**
     * Fetch a single market by UUID or slug.
     */
    getMarketById(idOrSlug: string): Observable<Market> {
        return this.http.get<Market>(`${this.baseUrl}/markets/${encodeURIComponent(idOrSlug)}`);
    }

    /**
     * Get an authoritative execution quote for BUY/SELL.
     */
    getQuote(marketId: string, request: QuoteRequest, fresh = false): Observable<BuyQuoteResponse> {
        const url = fresh
            ? `${this.baseUrl}/markets/${encodeURIComponent(marketId)}/quote?fresh=true`
            : `${this.baseUrl}/markets/${encodeURIComponent(marketId)}/quote`;
        const headers = fresh ? new HttpHeaders({ 'Cache-Control': 'no-cache' }) : undefined;
        return this.http.post<BuyQuoteResponse>(url, request, { headers });
    }

    /**
     * Place an order with mandatory Bearer authentication and unique Idempotency-Key.
     */
    placeOrder(marketId: string, request: PlaceOrderRequest, token: string, idempotencyKey: string): Observable<OrderResponse> {
        const headers = this.buildHeaders(token, idempotencyKey);
        return this.http.post<OrderResponse>(`${this.baseUrl}/markets/${encodeURIComponent(marketId)}/orders`, request, { headers });
    }

    /**
     * Claim testnet faucet USDC.
     */
    claimFaucet(token: string): Observable<FaucetResponse> {
        const headers = this.buildHeaders(token);
        return this.http.post<FaucetResponse>(`${this.baseUrl}/faucet`, {}, { headers });
    }

    /**
     * Get portfolio positions and cash balance.
     */
    getPortfolio(token: string): Observable<PortfolioResponse> {
        const headers = this.buildHeaders(token);
        return this.http.get<PortfolioResponse>(`${this.baseUrl}/portfolio`, { headers });
    }

    /**
     * Liquidate outcome shares back to USDC via AMM pool.
     */
    cashOut(request: CashOutRequest, token: string, idempotencyKey: string): Observable<CashOutResponse> {
        const headers = this.buildHeaders(token, idempotencyKey);
        return this.http.post<CashOutResponse>(`${this.baseUrl}/portfolio/cashout`, request, { headers });
    }

    /**
     * Administratively resolve a prediction market and trigger complete-set payout distribution.
     */
    resolveMarket(marketId: string, request: ResolveMarketRequest, adminToken: string, idempotencyKey: string): Observable<ResolveMarketResponse> {
        const headers = this.buildHeaders(adminToken, idempotencyKey);
        return this.http.post<ResolveMarketResponse>(`${this.baseUrl}/admin/markets/${encodeURIComponent(marketId)}/resolve`, request, { headers });
    }

    /**
     * Verify whether an admin authorization token is valid.
     */
    verifyAdmin(adminToken: string): Observable<{ status: string; message: string }> {
        const headers = this.buildHeaders(adminToken);
        return this.http.get<{ status: string; message: string }>(`${this.baseUrl}/admin/verify`, { headers });
    }

    /**
     * Create a new prediction market with calibrated CPMM liquidity reserves.
     */
    createMarket(request: CreateMarketRequest, adminToken: string): Observable<CreateMarketResponse> {
        const headers = this.buildHeaders(adminToken);
        return this.http.post<CreateMarketResponse>(`${this.baseUrl}/admin/markets`, request, { headers });
    }

    /**
     * Fetch runtime system configuration including APP_ENV and is_dev mode.
     */
    getConfig(): Observable<AppConfig> {
        return this.http.get<AppConfig>(`${this.baseUrl}/config`).pipe(
            catchError(() =>
                of({
                    app_env:
                        typeof window !== 'undefined' && (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1')
                            ? 'local'
                            : 'production',
                    is_dev: typeof window !== 'undefined' && (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'),
                    google_auth_enabled: false
                })
            )
        );
    }
}
