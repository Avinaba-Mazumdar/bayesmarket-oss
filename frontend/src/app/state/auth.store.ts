import { computed, inject, Injectable, signal } from '@angular/core';
import { Observable, tap } from 'rxjs';
import { ApiService } from '../core/services/api.service';
import { ToastService } from '../shared/components/toast/toast.service';
import { AuthResponse, UserProfile } from '../core/models/market.model';

const TOKEN_STORAGE_KEY = 'bayesmarket_auth_token';
const LEGACY_TOKEN_STORAGE_KEY = 'bayesmarket_guest_token';
const USER_PROFILE_STORAGE_KEY = 'bayesmarket_auth_profile';
const BALANCE_STORAGE_KEY = 'bayesmarket_user_balance';

@Injectable({
    providedIn: 'root'
})
export class AuthStore {
    private readonly apiService = inject(ApiService);
    private readonly toastService = inject(ToastService);

    readonly token = signal<string | null>(this.getInitialToken());
    readonly user = signal<UserProfile | null>(this.getInitialProfile());
    readonly userId = signal<string | null>(this.getInitialProfile()?.id || null);
    readonly cashBalance = signal<string>(this.getInitialBalance());

    readonly isGuest = computed(() => this.user()?.is_guest ?? false);
    readonly isAuthenticated = computed(() => !!this.token() && !!this.user());
    readonly authProvider = computed(() => this.user()?.auth_provider || 'none');
    readonly userName = computed(() => this.user()?.name || (this.isGuest() ? 'Guest Trader' : this.isAuthenticated() ? 'Verified Trader' : 'Visitor'));
    readonly userEmail = computed(() => this.user()?.email || null);
    readonly userAvatar = computed(() => this.user()?.avatar_url || null);

    readonly isInitializing = signal<boolean>(false);
    readonly isAuthenticating = signal<boolean>(false);
    readonly isClaimingFaucet = signal<boolean>(false);
    readonly isAuthModalOpen = signal<boolean>(false);

    readonly isDev = signal<boolean>(typeof window !== 'undefined' && (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'));
    readonly appEnv = signal<string>('development');

    constructor() {
        this.initializeSession();
    }

    private getInitialToken(): string | null {
        if (typeof window !== 'undefined') {
            if (window.sessionStorage) {
                const sessionToken = sessionStorage.getItem(TOKEN_STORAGE_KEY) || sessionStorage.getItem(LEGACY_TOKEN_STORAGE_KEY);
                if (sessionToken) return sessionToken;
            }
            if (window.localStorage) {
                const localToken = localStorage.getItem(TOKEN_STORAGE_KEY) || localStorage.getItem(LEGACY_TOKEN_STORAGE_KEY);
                if (localToken) {
                    // Migrate to sessionStorage and purge from localStorage to eliminate long-term disk exposure
                    if (window.sessionStorage) {
                        sessionStorage.setItem(TOKEN_STORAGE_KEY, localToken);
                    }
                    localStorage.removeItem(TOKEN_STORAGE_KEY);
                    localStorage.removeItem(LEGACY_TOKEN_STORAGE_KEY);
                    return localToken;
                }
            }
        }
        return null;
    }

    private getInitialProfile(): UserProfile | null {
        if (typeof window !== 'undefined') {
            let raw: string | null = null;
            if (window.sessionStorage) {
                raw = sessionStorage.getItem(USER_PROFILE_STORAGE_KEY);
            }
            if (!raw && window.localStorage) {
                raw = localStorage.getItem(USER_PROFILE_STORAGE_KEY);
                if (raw) {
                    if (window.sessionStorage) {
                        sessionStorage.setItem(USER_PROFILE_STORAGE_KEY, raw);
                    }
                    localStorage.removeItem(USER_PROFILE_STORAGE_KEY);
                }
            }
            if (raw) {
                try {
                    return JSON.parse(raw);
                } catch {
                    return null;
                }
            }
        }
        return null;
    }

    private getInitialBalance(): string {
        if (typeof window !== 'undefined') {
            if (window.sessionStorage) {
                const bal = sessionStorage.getItem(BALANCE_STORAGE_KEY);
                if (bal) return bal;
            }
            if (window.localStorage) {
                const bal = localStorage.getItem(BALANCE_STORAGE_KEY);
                if (bal) {
                    if (window.sessionStorage) {
                        sessionStorage.setItem(BALANCE_STORAGE_KEY, bal);
                    }
                    localStorage.removeItem(BALANCE_STORAGE_KEY);
                    return bal;
                }
            }
            const token = this.getInitialToken();
            if (token) {
                return '$1,000.00';
            }
            return '$0.00';
        }
        return '$0.00';
    }

    /**
     * Initializes or verifies session (Google or Guest).
     * Does NOT automatically create guest sessions for first-time visitors.
     */
    initializeSession(): void {
        this.fetchAppConfig();
        const existingToken = this.token();
        if (existingToken) {
            // Verify session by fetching user profile and portfolio balance
            this.fetchCurrentUserProfile(existingToken);
        }
    }

    private fetchAppConfig(): void {
        if (!this.apiService || typeof this.apiService.getConfig !== 'function') {
            return;
        }
        this.apiService.getConfig().subscribe({
            next: (cfg) => {
                if (cfg) {
                    this.isDev.set(cfg.is_dev);
                    this.appEnv.set(cfg.app_env);
                }
            }
        });
    }

    private fetchCurrentUserProfile(token: string): void {
        if (!this.apiService || typeof this.apiService.getCurrentUser !== 'function') {
            return;
        }
        this.apiService.getCurrentUser(token).subscribe({
            next: (profile) => {
                if (profile) {
                    this.user.set(profile);
                    this.userId.set(profile.id);
                    const formatted = this.formatBalance(profile.cash_balance ?? '1000.00');
                    this.updateBalance(formatted);
                    this.persistSession(token, profile);
                }
            },
            error: (err) => {
                // If token expired or invalid, clear session
                if (err?.status === 401) {
                    this.clearSession();
                }
            }
        });
    }

    /**
     * Explicitly provisions a fresh guest session seeded with 1,000 USD on user request.
     */
    continueAsGuest(onSuccess?: () => void): void {
        if (!this.apiService || typeof this.apiService.createGuestSession !== 'function') {
            return;
        }
        this.isInitializing.set(true);
        this.apiService.createGuestSession().subscribe({
            next: (res) => {
                if (res && res.user) {
                    this.token.set(res.token);
                    this.user.set(res.user);
                    this.userId.set(res.user.id);
                    const formattedBalance = this.formatBalance(res.user.cash_balance ?? '1000.00');
                    this.updateBalance(formattedBalance);
                    this.persistSession(res.token, res.user);
                    this.closeAuthModal();
                    this.toastService.success('Guest Session Active', 'Gifted $1,000.00 virtual USDC for paper trading!');
                    if (onSuccess) onSuccess();
                }
                this.isInitializing.set(false);
            },
            error: (err) => {
                console.error('Failed to provision guest session:', err);
                this.isInitializing.set(false);
                this.toastService.error('Session Error', 'Could not start guest session. Please try again.');
            }
        });
    }

    /**
     * Provisions a fresh anonymous guest session (internal).
     */
    provisionNewGuestSession(): void {
        this.continueAsGuest();
    }

    /**
     * Authenticates with Google via ID Token.
     */
    loginWithGoogle(idToken: string, email?: string, name?: string): void {
        this.isAuthenticating.set(true);
        const guestToken = this.isGuest() ? this.token() : null;

        this.apiService.verifyGoogleToken({ id_token: idToken, email, name }, guestToken).subscribe({
            next: (res) => {
                if (res && res.user) {
                    this.token.set(res.token);
                    this.user.set(res.user);
                    this.userId.set(res.user.id);
                    const formattedBalance = this.formatBalance(res.user.cash_balance ?? '1000.00');
                    this.updateBalance(formattedBalance);
                    this.persistSession(res.token, res.user);
                }
                this.isAuthenticating.set(false);
                this.isAuthModalOpen.set(false);

                this.toastService.success('Signed in with Google', `Welcome, ${res.user.name || res.user.email || 'Trader'}! Your account is connected.`);
            },
            error: (err) => {
                this.isAuthenticating.set(false);
                const msg = err?.error?.message || 'Failed to authenticate with Google';
                this.toastService.error('Authentication Error', msg);
            }
        });
    }

    /**
     * Completes OAuth 2.0 redirect flow by exchanging code and state.
     */
    loginWithGoogleCallback(code: string, state?: string): Observable<AuthResponse> {
        this.isAuthenticating.set(true);
        const guestToken = this.isGuest() ? this.token() : null;

        return this.apiService.callbackGoogleAuth({ code, state }, guestToken).pipe(
            tap({
                next: (res) => {
                    if (res && res.user) {
                        this.token.set(res.token);
                        this.user.set(res.user);
                        this.userId.set(res.user.id);
                        const formattedBalance = this.formatBalance(res.user.cash_balance ?? '1000.00');
                        this.updateBalance(formattedBalance);
                        this.persistSession(res.token, res.user);
                    }
                    this.isAuthenticating.set(false);
                    this.isAuthModalOpen.set(false);
                    this.toastService.success('Signed in with Google', `Welcome, ${res.user.name || res.user.email || 'Trader'}! Your account is connected.`);
                },
                error: (err) => {
                    this.isAuthenticating.set(false);
                    const msg = err?.error?.message || 'Failed to authenticate with Google';
                    this.toastService.error('Authentication Error', msg);
                }
            })
        );
    }

    /**
     * Logs out of account and reverts to unauthenticated surfing state.
     */
    logout(): void {
        this.clearSession();
        this.toastService.info('Signed Out', 'You have been signed out.');
    }

    /**
     * Synchronizes balance with the backend.
     */
    refreshBalance(): void {
        const token = this.token();
        if (!token) return;

        this.apiService.getPortfolio(token).subscribe({
            next: (portfolio) => {
                const formatted = this.formatBalance(portfolio.cash_balance || portfolio.cash_balance_usdc || '0');
                this.updateBalance(formatted);
            },
            error: (err) => {
                if (err?.status === 401) {
                    this.initializeSession();
                }
            }
        });
    }

    /**
     * Updates cash balance in state and sessionStorage.
     */
    updateBalance(formattedBalance: string): void {
        this.cashBalance.set(formattedBalance);
        if (typeof window !== 'undefined') {
            if (window.sessionStorage) {
                sessionStorage.setItem(BALANCE_STORAGE_KEY, formattedBalance);
            }
            if (window.localStorage) {
                localStorage.removeItem(BALANCE_STORAGE_KEY);
            }
        }
    }

    /**
     * Claims testnet faucet funds.
     */
    claimFaucet(): void {
        const token = this.token();
        if (!token) {
            this.toastService.error('Session Error', 'Session not ready');
            return;
        }

        this.isClaimingFaucet.set(true);
        this.apiService.claimFaucet(token).subscribe({
            next: (res) => {
                const rawBalance = res.new_balance ?? res.user?.cash_balance ?? '0';
                const formatted = this.formatBalance(rawBalance);
                this.updateBalance(formatted);

                const currentUser = this.user();
                if (currentUser) {
                    const updatedProfile = { ...currentUser, cash_balance: rawBalance };
                    this.user.set(updatedProfile);
                    if (typeof window !== 'undefined') {
                        if (window.sessionStorage) {
                            sessionStorage.setItem(USER_PROFILE_STORAGE_KEY, JSON.stringify(updatedProfile));
                        }
                        if (window.localStorage) {
                            localStorage.removeItem(USER_PROFILE_STORAGE_KEY);
                        }
                    }
                }

                this.isClaimingFaucet.set(false);
                const claimedAmount = res.amount_claimed ? Math.round(parseFloat(res.amount_claimed)).toString() : (res.amount ?? '100');
                this.toastService.success('Faucet Claimed', `+$${claimedAmount} USDC credited to your balance`);
            },
            error: (err) => {
                this.isClaimingFaucet.set(false);
                const msg = err?.error?.message || 'Faucet cooldown in effect or request rate exceeded';
                this.toastService.warning('Faucet Cooldown', msg);
            }
        });
    }

    openAuthModal(): void {
        this.isAuthModalOpen.set(true);
    }

    closeAuthModal(): void {
        this.isAuthModalOpen.set(false);
    }

    private persistSession(token: string, profile: UserProfile): void {
        if (typeof window !== 'undefined') {
            if (window.sessionStorage) {
                sessionStorage.setItem(TOKEN_STORAGE_KEY, token);
                sessionStorage.setItem(USER_PROFILE_STORAGE_KEY, JSON.stringify(profile));
            }
            // Clear any lingering localStorage records to mitigate XSS exposure
            if (window.localStorage) {
                localStorage.removeItem(TOKEN_STORAGE_KEY);
                localStorage.removeItem(LEGACY_TOKEN_STORAGE_KEY);
                localStorage.removeItem(USER_PROFILE_STORAGE_KEY);
            }
        }
    }

    private clearSession(): void {
        if (typeof window !== 'undefined') {
            if (window.sessionStorage) {
                sessionStorage.removeItem(TOKEN_STORAGE_KEY);
                sessionStorage.removeItem(LEGACY_TOKEN_STORAGE_KEY);
                sessionStorage.removeItem(USER_PROFILE_STORAGE_KEY);
                sessionStorage.removeItem(BALANCE_STORAGE_KEY);
            }
            if (window.localStorage) {
                localStorage.removeItem(TOKEN_STORAGE_KEY);
                localStorage.removeItem(LEGACY_TOKEN_STORAGE_KEY);
                localStorage.removeItem(USER_PROFILE_STORAGE_KEY);
                localStorage.removeItem(BALANCE_STORAGE_KEY);
            }
        }
        this.token.set(null);
        this.user.set(null);
        this.userId.set(null);
        this.cashBalance.set('$0.00');
    }

    formatBalance(raw: string | number): string {
        const num = typeof raw === 'string' ? parseFloat(raw) : raw;
        if (isNaN(num)) return '$1,000.00';
        return `$${num.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
    }
}
