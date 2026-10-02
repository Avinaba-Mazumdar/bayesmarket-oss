import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { RouterLink, RouterLinkActive } from '@angular/router';
import { LucideLogIn, LucideSun, LucideMoon, LucideShieldCheck } from '@lucide/angular';
import { ThemeService } from '../../services/theme.service';
import { ButtonComponent } from '../../../shared/components/button/button.component';
import { AvatarComponent } from '../../../shared/components/avatar/avatar.component';
import { BrandIconComponent } from '../../../shared/components/brand-icon/brand-icon.component';
import { AuthStore } from '../../../state/auth.store';

@Component({
    selector: 'app-top-header-dock',
    standalone: true,
    changeDetection: ChangeDetectionStrategy.OnPush,
    imports: [RouterLink, RouterLinkActive, ButtonComponent, AvatarComponent, BrandIconComponent, LucideLogIn, LucideSun, LucideMoon, LucideShieldCheck],
    template: `
        <header class="top-header-dock" role="banner">
            <div class="dock-container">
                <!-- Left: Brand Emblem & Navigation -->
                <div class="dock-left">
                    <a routerLink="/" class="brand-link" aria-label="BayesMarket Home">
                        <app-brand-icon [size]="30"></app-brand-icon>
                        <span class="brand-text">Bayes<span class="brand-highlight">Market</span></span>
                    </a>

                    <nav class="nav-links" role="navigation" aria-label="Primary Navigation">
                        <a routerLink="/" routerLinkActive="active" [routerLinkActiveOptions]="{ exact: true }" class="nav-tab"> Markets </a>
                        <a routerLink="/portfolio" routerLinkActive="active" class="nav-tab"> Portfolio </a>
                        @if (authStore.isAdmin()) {
                            <a routerLink="/admin" routerLinkActive="active" class="nav-tab admin-nav-tab" aria-label="Admin operations console">
                                <svg lucideShieldCheck class="admin-tab-icon" [size]="14" aria-hidden="true"></svg>
                                <span>Admin</span>
                            </a>
                        }
                    </nav>
                </div>

                <!-- Right: Faucet, Theme Toggle, Auth -->
                <div class="dock-right">
                    @if (authStore.isAuthenticated()) {
                        <!-- Faucet Button -->
                        <app-button
                            variant="secondary"
                            size="default"
                            [loading]="isClaimingFaucet()"
                            ariaLabel="Claim testnet faucet USDC"
                            (btnClick)="onClaimFaucet()"
                        >
                            @if (isClaimingFaucet()) {
                                <span>Claiming...</span>
                            } @else {
                                <span>+100 Faucet</span>
                            }
                        </app-button>
                    }

                    <!-- Theme Switcher Toggle -->
                    <button
                        type="button"
                        class="theme-toggle-btn"
                        (click)="themeService.toggleTheme()"
                        [attr.aria-label]="themeService.isDark() ? 'Switch to light theme' : 'Switch to dark theme'"
                        [title]="themeService.isDark() ? 'Switch to light theme' : 'Switch to dark theme'"
                    >
                        @if (themeService.isDark()) {
                            <svg lucideSun [size]="16" class="theme-icon sun-icon" aria-hidden="true"></svg>
                        } @else {
                            <svg lucideMoon [size]="16" class="theme-icon moon-icon" aria-hidden="true"></svg>
                        }
                    </button>

                    <!-- Auth State: Unauthenticated vs Guest vs Google Profile -->
                    @if (!authStore.isAuthenticated()) {
                        <app-button
                            variant="primary"
                            size="default"
                            class="signin-main-btn"
                            ariaLabel="Click to sign in or start trading"
                            title="Sign In to Start Paper Trading"
                            (btnClick)="authStore.openAuthModal()"
                        >
                            <span class="auth-btn-text">Sign In</span>
                            <svg lucideLogIn class="auth-icon" [size]="14" aria-hidden="true"></svg>
                        </app-button>
                    } @else if (authStore.isGuest()) {
                        <div class="user-profile-dock">
                            <button
                                type="button"
                                class="auth-action-btn guest-logged-in-btn"
                                (click)="authStore.openAuthModal()"
                                [attr.aria-label]="'Trading as Guest. Click to connect Google account.'"
                                title="Guest Trader (Click to connect Google)"
                            >
                                <span class="guest-badge-pill">Guest</span>
                                <span class="auth-user-name">Guest Trader</span>
                            </button>
                        </div>
                    } @else {
                        <div class="user-profile-dock">
                            <button
                                type="button"
                                class="auth-action-btn user-logged-in-btn"
                                (click)="authStore.openAuthModal()"
                                [attr.aria-label]="'Trading as ' + authStore.userName() + '. Click for account details.'"
                                [title]="'Logged in as ' + authStore.userName()"
                            >
                                <app-avatar [src]="authStore.userAvatar() || ''" [alt]="authStore.userName()" size="sm" />
                                <span class="auth-user-name">{{ authStore.userName() }}</span>
                            </button>
                        </div>
                    }
                </div>
            </div>
        </header>
    `,
    styles: [
        `
            :host {
                display: block;
                position: sticky;
                top: 0;
                z-index: 1000;
            }
            .top-header-dock {
                height: 60px;
                background-color: var(--surface-glass, rgba(19, 17, 38, 0.97));
                border-bottom: 1px solid var(--hairline, #252140);
                display: flex;
                align-items: center;
                padding: 0 var(--space-lg, 20px);
            }
            .dock-container {
                width: 100%;
                max-width: 1560px;
                margin: 0 auto;
                display: flex;
                align-items: center;
                justify-content: space-between;
                gap: var(--space-md, 16px);
            }
            .dock-left,
            .dock-right {
                display: flex;
                align-items: center;
                gap: var(--space-md, 16px);
            }
            .brand-link {
                display: flex;
                align-items: center;
                gap: 10px;
                text-decoration: none;
                min-height: var(--touch-target-min, 40px);
                min-width: var(--touch-target-min, 40px);
            }
            .brand-emblem {
                display: flex;
                align-items: center;
                justify-content: center;
                width: 32px;
                height: 32px;
                background-color: rgba(54, 0, 179, 0.16);
                border: 1px solid rgba(124, 77, 255, 0.35);
                border-radius: var(--radius-sm, 6px);
            }
            .brand-text {
                font-family: var(--font-ui);
                font-size: 18px;
                font-weight: 700;
                color: var(--ink, #f8f7ff);
                letter-spacing: -0.3px;
            }
            .brand-highlight {
                color: var(--primary-border, #7c4dff);
            }
            .nav-links {
                display: flex;
                align-items: center;
                gap: 6px;
                margin-left: 12px;
            }
            .nav-tab {
                display: inline-flex;
                align-items: center;
                justify-content: center;
                min-height: 38px;
                padding: 6px 14px;
                color: var(--ink-secondary, #9d97b8);
                font-family: var(--font-ui);
                font-size: 13.5px;
                font-weight: 600;
                text-decoration: none;
                border-radius: var(--radius-pill, 9999px);
                border: 1px solid transparent;
                transition:
                    background-color 0.15s ease,
                    border-color 0.15s ease,
                    color 0.15s ease;
            }
            .nav-tab:hover {
                background-color: var(--surface-card, #131126);
                color: var(--ink, #f8f7ff);
            }
            .nav-tab.active {
                background-color: var(--surface-card-elevated, #1a1733);
                color: var(--ink, #f8f7ff);
                border: 1px solid var(--hairline, #252140);
                font-weight: 700;
            }
            .admin-nav-tab {
                gap: 6px;
                color: #e84089;
                border: 1px solid rgba(232, 64, 137, 0.25);
                background-color: rgba(232, 64, 137, 0.08);
            }
            .admin-nav-tab:hover {
                background-color: rgba(232, 64, 137, 0.16);
                border-color: rgba(232, 64, 137, 0.5);
                color: #ff60a8;
            }
            .admin-nav-tab.active {
                background-color: rgba(232, 64, 137, 0.22);
                border-color: #e84089;
                color: #ffffff;
            }
            .admin-tab-icon {
                flex-shrink: 0;
            }
            @media (max-width: 768px) {
                .dock-container {
                    gap: 8px;
                }
                .dock-left,
                .dock-right {
                    gap: 8px;
                }
                .brand-text {
                    font-size: 16px;
                }
            }

            /* Theme Switcher Toggle */
            .theme-toggle-btn {
                display: inline-flex;
                align-items: center;
                justify-content: center;
                width: 38px;
                height: 38px;
                min-width: 38px;
                min-height: 38px;
                padding: 0;
                background-color: var(--surface-card, #131126);
                border: 1px solid var(--hairline, #252140);
                border-radius: var(--radius-pill, 9999px);
                color: var(--ink-secondary, #9d97b8);
                cursor: pointer;
                transition:
                    background-color 0.15s ease,
                    border-color 0.15s ease,
                    color 0.15s ease,
                    transform 0.2s cubic-bezier(0.34, 1.56, 0.64, 1);
            }

            .theme-toggle-btn:hover {
                background-color: var(--surface-card-elevated, #1a1733);
                border-color: var(--primary-border, #7c4dff);
                color: var(--ink, #ffffff);
                transform: rotate(15deg);
            }

            .theme-toggle-btn:active {
                transform: scale(0.92) rotate(15deg);
            }

            .theme-icon {
                color: currentColor;
            }

            /* Auth Styles */
            .auth-action-btn {
                display: inline-flex;
                align-items: center;
                gap: 8px;
                min-height: 38px;
                padding: 6px 14px;
                border-radius: var(--radius-pill, 9999px);
                font-family: var(--font-ui);
                font-size: 13px;
                font-weight: 600;
                cursor: pointer;
                transition:
                    background-color 0.15s ease,
                    border-color 0.15s ease,
                    color 0.15s ease;
            }

            .user-profile-dock {
                display: flex;
                align-items: center;
                gap: 6px;
            }

            .user-logged-in-btn {
                background-color: var(--surface-card, #131126);
                border: 1px solid var(--hairline, #252140);
                color: var(--ink, #f8f7ff);
                padding: 6px;
            }

            .user-logged-in-btn:hover {
                background-color: var(--surface-card-elevated, #1a1733);
                border-color: var(--primary-border, #7c4dff);
            }

            .guest-logged-in-btn {
                background-color: var(--surface-card, #131126);
                border: 1px solid var(--hairline, #252140);
                color: var(--ink, #f8f7ff);
                padding: 6px 12px;
            }

            .guest-logged-in-btn:hover {
                background-color: var(--surface-card-elevated, #1a1733);
                border-color: var(--primary-border, #7c4dff);
            }

            .guest-badge-pill {
                font-family: var(--font-mono);
                font-size: 11px;
                font-weight: 700;
                color: var(--accent, #00d4ff);
                background-color: rgba(0, 212, 255, 0.12);
                border: 1px solid rgba(0, 212, 255, 0.3);
                padding: 2px 6px;
                border-radius: var(--radius-pill, 9999px);
            }

            .auth-user-name {
                max-width: 110px;
                white-space: nowrap;
                overflow: hidden;
                text-overflow: ellipsis;
            }
        `
    ]
})
export class TopHeaderDockComponent {
    readonly authStore = inject(AuthStore);
    readonly themeService = inject(ThemeService);

    get isClaimingFaucet() {
        return this.authStore.isClaimingFaucet;
    }

    onClaimFaucet(): void {
        this.authStore.claimFaucet();
    }
}
