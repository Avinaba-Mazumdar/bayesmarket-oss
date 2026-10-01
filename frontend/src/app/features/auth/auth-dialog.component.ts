import { ChangeDetectionStrategy, Component, DestroyRef, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { CommonModule } from '@angular/common';
import { LucideCheck, LucideZap } from '@lucide/angular';
import { DialogComponent } from '../../shared/components/dialog/dialog.component';
import { ButtonComponent } from '../../shared/components/button/button.component';
import { AvatarComponent } from '../../shared/components/avatar/avatar.component';
import { BadgeComponent } from '../../shared/components/badge/badge.component';
import { AuthStore } from '../../state/auth.store';
import { ApiService } from '../../core/services/api.service';
import { ToastService } from '../../shared/components/toast/toast.service';

@Component({
    selector: 'app-auth-dialog',
    standalone: true,
    changeDetection: ChangeDetectionStrategy.OnPush,
    imports: [CommonModule, DialogComponent, ButtonComponent, AvatarComponent, BadgeComponent, LucideCheck, LucideZap],
    template: `
        <app-dialog
            [open]="authStore.isAuthModalOpen()"
            (closed)="onClose()"
            [title]="authStore.isGuest() ? 'Guest Account' : authStore.isAuthenticated() ? 'Your Account' : 'Sign In to BayesMarket'"
            [description]="
                authStore.isGuest()
                    ? 'You are currently trading in an anonymous guest session with virtual testnet USDC.'
                    : authStore.isAuthenticated()
                      ? 'Manage your verified trading profile and preferences.'
                      : 'Choose an option to begin trading with your complimentary $1,000.00 USDC paper balance.'
            "
            size="default"
            role="dialog"
            ariaLabel="Authentication Dialog"
        >
            <div class="auth-dialog-content">
                @if (authStore.isGuest()) {
                    <!-- Guest Account & Session View -->
                    <div class="profile-card">
                        <div class="profile-header">
                            <app-avatar [src]="authStore.userAvatar() || ''" [alt]="authStore.userName()" size="lg" />
                            <div class="profile-meta">
                                <h3 class="profile-name">{{ authStore.userName() }}</h3>
                                <p class="profile-email">Anonymous Guest Session</p>
                                <app-badge class="provider-badge" variant="warning" size="sm"> Guest Trader </app-badge>
                            </div>
                        </div>

                        <div class="profile-stats">
                            <div class="stat-item">
                                <span class="stat-label">Available Balance</span>
                                <span class="stat-val tabular-nums">{{ authStore.cashBalance() }}</span>
                            </div>
                            <div class="stat-item">
                                <span class="stat-label">Account Mode</span>
                                <span class="stat-val warning-text">Guest (Ephemeral)</span>
                            </div>
                        </div>

                        <!-- Upgrade / Connect Google Section -->
                        <div class="guest-upgrade-box">
                            <div class="upgrade-info">
                                <span class="upgrade-title">Save Your Balance & Progress</span>
                                <p class="upgrade-desc">
                                    Connect your Google account to preserve your active positions, predictions, and {{ authStore.cashBalance() }} balance.
                                </p>
                            </div>

                            <button
                                type="button"
                                class="google-signin-btn"
                                (click)="initiateGoogleAuth()"
                                [disabled]="authStore.isAuthenticating() || isRedirecting()"
                                aria-label="Connect Google account"
                            >
                                @if (authStore.isAuthenticating() || isRedirecting()) {
                                    <div class="spinner" aria-hidden="true"></div>
                                    <span>Connecting to Google...</span>
                                } @else {
                                    <svg class="google-icon" width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
                                        <path
                                            d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
                                            fill="#4285F4"
                                        />
                                        <path
                                            d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
                                            fill="#34A853"
                                        />
                                        <path
                                            d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22.81-.63z"
                                            fill="#FBBC05"
                                        />
                                        <path
                                            d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52z"
                                            fill="#EA4335"
                                        />
                                    </svg>
                                    <span class="btn-text">Connect Google Account</span>
                                }
                            </button>
                        </div>

                        <div class="profile-actions">
                            <app-button variant="destructive" size="default" (btnClick)="onSignOut()" ariaLabel="Sign out of guest session">
                                Sign Out
                            </app-button>
                        </div>
                    </div>
                } @else if (authStore.isAuthenticated()) {
                    <!-- Authenticated Profile View -->
                    <div class="profile-card">
                        <div class="profile-header">
                            <app-avatar [src]="authStore.userAvatar() || ''" [alt]="authStore.userName()" size="lg" />
                            <div class="profile-meta">
                                <h3 class="profile-name">{{ authStore.userName() }}</h3>
                                <p class="profile-email">{{ authStore.userEmail() || 'Google Authenticated User' }}</p>
                                <app-badge class="provider-badge" variant="profit" size="sm">
                                    <svg lucideCheck class="badge-icon" [size]="12" aria-hidden="true"></svg>
                                    Google Verified
                                </app-badge>
                            </div>
                        </div>

                        <div class="profile-stats">
                            <div class="stat-item">
                                <span class="stat-label">Available Balance</span>
                                <span class="stat-val tabular-nums">{{ authStore.cashBalance() }}</span>
                            </div>
                            <div class="stat-item">
                                <span class="stat-label">Account Mode</span>
                                <span class="stat-val highlight">Persistent</span>
                            </div>
                        </div>

                        <div class="profile-actions">
                            <app-button variant="destructive" size="default" (btnClick)="onSignOut()" ariaLabel="Sign out of BayesMarket">
                                Sign Out
                            </app-button>
                        </div>
                    </div>
                } @else {
                    <!-- Simulation / Paper Trading Notice -->
                    <div class="simulation-banner" role="status">
                        <span class="simulation-tag">PAPER TRADING</span>
                        <span class="simulation-text"
                            >All trading on BayesMarket is simulated with virtual USDC. No real money or cryptocurrency is involved.</span
                        >
                    </div>

                    <!-- Main Google Sign-In Action -->
                    <div class="auth-card primary-auth">
                        <div class="auth-card-body">
                            <h3 class="auth-method-title">Sign in with Google</h3>
                            <p class="auth-method-desc">Seamless one-click authentication. Never lose your predictions or liquidity pool allocations.</p>

                            <button
                                type="button"
                                class="google-signin-btn"
                                (click)="initiateGoogleAuth()"
                                [disabled]="authStore.isAuthenticating() || isRedirecting()"
                                aria-label="Sign in with your Google account"
                            >
                                @if (authStore.isAuthenticating() || isRedirecting()) {
                                    <div class="spinner" aria-hidden="true"></div>
                                    <span>Connecting to Google...</span>
                                } @else {
                                    <svg class="google-icon" width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
                                        <path
                                            d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
                                            fill="#4285F4"
                                        />
                                        <path
                                            d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
                                            fill="#34A853"
                                        />
                                        <path
                                            d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22.81-.63z"
                                            fill="#FBBC05"
                                        />
                                        <path
                                            d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52z"
                                            fill="#EA4335"
                                        />
                                    </svg>
                                    <span class="btn-text">Continue with Google</span>
                                }
                            </button>
                        </div>
                    </div>

                    <!-- Development Quick Login (Available only in local / dev environments) -->
                    @if (authStore.isDev()) {
                        <div class="dev-quick-option">
                            <div class="divider">
                                <span class="divider-label">DEVELOPMENT & TESTING</span>
                            </div>
                            <div class="dev-box">
                                <div class="dev-info">
                                    <svg lucideZap class="dev-icon" [size]="16" aria-hidden="true"></svg>
                                    <span>Simulate Google Sign-In with mock developer credentials</span>
                                </div>
                                <app-button
                                    variant="secondary"
                                    size="sm"
                                    [loading]="authStore.isAuthenticating()"
                                    (btnClick)="onDevQuickLogin()"
                                    ariaLabel="One-click simulate Google sign-in"
                                >
                                    Quick Dev Sign-In
                                </app-button>
                            </div>
                        </div>
                    }

                    <div class="divider">
                        <span class="divider-label">OR</span>
                    </div>

                    <!-- Guest Session Option -->
                    <div class="auth-card guest-auth">
                        <div class="auth-card-body">
                            <div class="guest-info-row">
                                <div class="guest-details">
                                    <h4 class="guest-title">Continue as Guest</h4>
                                    <p class="guest-desc">
                                        Immediate sandbox trading. You will receive a $1,000.00 virtual USDC gift with zero registration required.
                                    </p>
                                </div>
                                <app-button
                                    variant="secondary"
                                    size="default"
                                    [loading]="authStore.isInitializing()"
                                    (btnClick)="continueAsGuest()"
                                    ariaLabel="Continue as guest trader"
                                >
                                    Start as Guest
                                </app-button>
                            </div>
                        </div>
                    </div>
                }
            </div>
        </app-dialog>
    `,
    styles: [
        `
            .auth-dialog-content {
                display: flex;
                flex-direction: column;
                gap: 16px;
            }

            .simulation-banner {
                display: flex;
                align-items: center;
                gap: 10px;
                padding: 10px 14px;
                background-color: rgba(124, 77, 255, 0.08);
                border: 1px solid rgba(124, 77, 255, 0.25);
                border-radius: var(--radius-md, 10px);
            }

            .simulation-tag {
                font-family: var(--font-mono);
                font-size: 10px;
                font-weight: 800;
                color: var(--primary-border, #7c4dff);
                background-color: rgba(124, 77, 255, 0.16);
                padding: 2px 6px;
                border-radius: var(--radius-sm, 4px);
                letter-spacing: 0.5px;
                flex-shrink: 0;
            }

            .simulation-text {
                font-family: var(--font-ui);
                font-size: 12px;
                color: var(--muted, #9d97b8);
                line-height: 1.35;
            }

            .upgrade-banner {
                display: flex;
                align-items: flex-start;
                gap: 12px;
                padding: 12px 16px;
                background-color: var(--status-info-bg, #f0f9ff);
                border: 1px solid var(--status-info-border, #0284c7);
                border-radius: var(--radius-md, 10px);
            }

            .banner-icon {
                color: var(--status-info, #075985);
                flex-shrink: 0;
                margin-top: 2px;
            }

            .banner-text {
                display: flex;
                flex-direction: column;
                gap: 2px;
            }

            .banner-title {
                font-family: var(--font-ui);
                font-size: 13px;
                font-weight: 700;
                color: var(--status-info, #075985);
            }

            .banner-desc {
                font-family: var(--font-ui);
                font-size: 12px;
                color: var(--body, #334155);
                line-height: 1.4;
            }

            .auth-card {
                background-color: var(--canvas-subtle, #f1f5f9);
                border: 1px solid var(--hairline, #cbd5e1);
                border-radius: var(--radius-lg, 14px);
                overflow: hidden;
            }

            .auth-card.primary-auth {
                border-color: var(--primary-border, #4338ca);
                background-color: var(--surface-card, #ffffff);
                box-shadow: var(--shadow-sm);
            }

            .auth-card-body {
                padding: 18px 20px;
                display: flex;
                flex-direction: column;
                gap: 14px;
            }

            .auth-method-title {
                margin: 0;
                font-family: var(--font-ui);
                font-size: 16px;
                font-weight: 700;
                color: var(--ink, #f8f7ff);
            }

            .auth-method-desc {
                margin: 0;
                font-family: var(--font-ui);
                font-size: 13px;
                line-height: 1.45;
                color: var(--body, #ccc7e6);
            }

            .google-signin-btn {
                display: flex;
                align-items: center;
                justify-content: center;
                gap: 12px;
                width: 100%;
                min-height: var(--touch-target-min, 44px);
                padding: 10px 16px;
                background-color: #ffffff;
                color: #1f2937;
                border: 1px solid #d1d5db;
                border-radius: var(--radius-md, 10px);
                font-family: var(--font-ui);
                font-size: 14px;
                font-weight: 600;
                cursor: pointer;
                box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
                transition:
                    background-color 0.15s ease,
                    box-shadow 0.15s ease,
                    transform 0.05s ease;
            }

            .google-signin-btn:hover:not(:disabled) {
                background-color: #f3f4f6;
                box-shadow: 0 2px 6px rgba(0, 0, 0, 0.3);
            }

            .google-signin-btn:active:not(:disabled) {
                transform: translateY(1px);
            }

            .google-signin-btn:focus-visible {
                outline: 2px solid var(--primary-border, #7c4dff);
                outline-offset: 2px;
            }

            .google-signin-btn:disabled {
                opacity: 0.6;
                cursor: not-allowed;
            }

            .google-icon {
                flex-shrink: 0;
            }

            .divider {
                display: flex;
                align-items: center;
                text-align: center;
                margin: 4px 0;
            }

            .divider::before,
            .divider::after {
                content: '';
                flex: 1;
                border-bottom: 1px solid var(--hairline, #252140);
            }

            .divider-label {
                padding: 0 12px;
                font-family: var(--font-mono);
                font-size: 10px;
                font-weight: 700;
                color: var(--muted, #9d97b8);
                letter-spacing: 0.8px;
            }

            .dev-quick-option {
                display: flex;
                flex-direction: column;
                gap: 8px;
            }

            .dev-box {
                display: flex;
                align-items: center;
                justify-content: space-between;
                gap: 12px;
                padding: 10px 14px;
                background-color: var(--status-warning-bg, rgba(245, 158, 11, 0.06));
                border: 1px dashed var(--status-warning-border, rgba(245, 158, 11, 0.3));
                border-radius: var(--radius-md, 10px);
            }

            .dev-info {
                display: flex;
                align-items: center;
                gap: 8px;
                font-family: var(--font-ui);
                font-size: 12px;
                color: var(--status-warning, #b45309);
            }

            .dev-icon {
                flex-shrink: 0;
                color: var(--status-warning, #b45309);
            }

            .guest-info-row {
                display: flex;
                align-items: center;
                justify-content: space-between;
                gap: 16px;
                flex-wrap: wrap;
            }

            .guest-details {
                display: flex;
                flex-direction: column;
                gap: 4px;
                flex: 1 1 200px;
                min-width: 0;
            }

            .guest-actions {
                display: flex;
                align-items: center;
                gap: 8px;
                flex-shrink: 0;
                flex-wrap: wrap;
            }

            .guest-title {
                margin: 0;
                font-family: var(--font-ui);
                font-size: 14px;
                font-weight: 600;
                color: var(--ink, #f8f7ff);
            }

            .guest-desc {
                margin: 0;
                font-family: var(--font-ui);
                font-size: 12px;
                color: var(--muted, #9d97b8);
                line-height: 1.4;
            }

            /* Profile Card Styles */
            .profile-card {
                display: flex;
                flex-direction: column;
                gap: 20px;
                padding: 16px;
                background-color: var(--canvas-subtle, #0e0c1c);
                border: 1px solid var(--border-subtle, #252140);
                border-radius: var(--radius-lg, 14px);
            }

            .profile-header {
                display: flex;
                align-items: center;
                gap: 16px;
            }

            .user-avatar {
                width: 52px;
                height: 52px;
                border-radius: 50%;
                border: 2px solid var(--primary-border, #e84089);
                object-fit: cover;
            }

            .avatar-placeholder {
                width: 52px;
                height: 52px;
                border-radius: 50%;
                background-color: rgba(166, 3, 76, 0.2);
                border: 2px solid var(--primary-border, #e84089);
                display: flex;
                align-items: center;
                justify-content: center;
                color: #ffffff;
            }

            .profile-meta {
                display: flex;
                flex-direction: column;
                gap: 4px;
            }

            .profile-name {
                margin: 0;
                font-family: var(--font-ui);
                font-size: 16px;
                font-weight: 700;
                color: var(--ink, #f8fafc);
            }

            .profile-email {
                margin: 0;
                font-family: var(--font-mono);
                font-size: 12px;
                color: var(--ink-secondary, #cbd5e1);
            }

            .provider-badge {
                width: fit-content;
                margin-top: 2px;
            }

            .badge-icon {
                flex-shrink: 0;
            }

            .profile-stats {
                display: grid;
                grid-template-columns: 1fr 1fr;
                gap: 12px;
                padding: 12px;
                background-color: var(--surface-card, #111622);
                border: 1px solid var(--hairline, #1e2638);
                border-radius: var(--radius-md, 10px);
            }

            .stat-item {
                display: flex;
                flex-direction: column;
                gap: 4px;
            }

            .stat-label {
                font-family: var(--font-ui);
                font-size: 11px;
                color: var(--muted, #64748b);
                font-weight: 600;
                text-transform: uppercase;
                letter-spacing: 0.5px;
            }

            .stat-val {
                font-family: var(--font-mono);
                font-size: 15px;
                font-weight: 700;
                color: var(--ink, #f8fafc);
            }

            .stat-val.highlight {
                color: var(--accent, #075985);
            }

            .stat-val.warning-text {
                color: var(--status-warning, #f59e0b);
            }

            .guest-upgrade-box {
                display: flex;
                flex-direction: column;
                gap: 12px;
                padding: 14px 16px;
                background-color: var(--surface-card, #111622);
                border: 1px solid var(--primary-border, #4338ca);
                border-radius: var(--radius-md, 10px);
            }

            .upgrade-info {
                display: flex;
                flex-direction: column;
                gap: 4px;
            }

            .upgrade-title {
                font-family: var(--font-ui);
                font-size: 13px;
                font-weight: 700;
                color: var(--ink, #f8fafc);
            }

            .upgrade-desc {
                margin: 0;
                font-family: var(--font-ui);
                font-size: 12px;
                color: var(--muted, #9d97b8);
                line-height: 1.4;
            }

            .profile-actions {
                display: flex;
                justify-content: flex-end;
            }

            .spinner {
                width: 16px;
                height: 16px;
                border: 2px solid rgba(31, 41, 55, 0.2);
                border-top-color: #1f2937;
                border-radius: 50%;
                animation: spin 0.8s linear infinite;
            }

            @keyframes spin {
                to {
                    transform: rotate(360deg);
                }
            }

            @media (max-width: 640px) {
                .guest-info-row {
                    flex-direction: column;
                    align-items: flex-start;
                    gap: 12px;
                }
                .guest-actions {
                    width: 100%;
                    justify-content: flex-end;
                }
                .dev-box {
                    flex-direction: column;
                    align-items: flex-start;
                }
            }
        `
    ]
})
export class AuthDialogComponent {
    readonly authStore = inject(AuthStore);
    private readonly apiService = inject(ApiService);
    private readonly toastService = inject(ToastService);
    private readonly destroyRef = inject(DestroyRef);

    readonly isRedirecting = signal<boolean>(false);

    onClose(): void {
        this.authStore.closeAuthModal();
    }

    continueAsGuest(): void {
        if (this.authStore.isGuest()) {
            this.authStore.closeAuthModal();
        } else {
            this.authStore.continueAsGuest();
        }
    }

    onDevQuickLogin(): void {
        if (!this.authStore.isDev()) {
            this.toastService.warning('Disabled', 'Dev Quick Login is only available when APP_ENV=local or dev');
            return;
        }
        this.authStore.loginWithGoogle('dev-mock-id-token-12345', 'trader@bayesmarket.com', 'Alex Mercer (Trader)');
    }

    initiateGoogleAuth(): void {
        this.isRedirecting.set(true);

        // Fetch the Google OAuth authorization URL from backend
        this.apiService
            .getGoogleAuthUrl()
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: (res) => {
                    this.isRedirecting.set(false);
                    if (res.simulated || !res.url) {
                        if (this.authStore.isDev()) {
                            // Backend is running in simulated / dev mode without client ID
                            this.onDevQuickLogin();
                        } else {
                            this.toastService.error('OAuth Error', 'Google OAuth is not configured in this environment');
                        }
                    } else {
                        // Store OAuth state in sessionStorage for CSRF validation upon callback
                        if (res.state && typeof window !== 'undefined' && window.sessionStorage) {
                            sessionStorage.setItem('bayesmarket_oauth_state', res.state);
                        }
                        // Real Google OAuth redirect flow
                        window.location.href = res.url;
                    }
                },
                error: (err) => {
                    this.isRedirecting.set(false);
                    if (this.authStore.isDev()) {
                        // Fallback to dev quick login if error or offline in dev
                        this.onDevQuickLogin();
                    } else {
                        const msg = err?.error?.message || 'Failed to initiate Google authentication';
                        this.toastService.error('Authentication Error', msg);
                    }
                }
            });
    }

    onSignOut(): void {
        this.authStore.logout();
        this.authStore.closeAuthModal();
    }
}
