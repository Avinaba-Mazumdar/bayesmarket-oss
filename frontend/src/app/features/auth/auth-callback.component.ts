import { ChangeDetectionStrategy, Component, inject, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router } from '@angular/router';
import { LucideAlertCircle, LucideLoader2 } from '@lucide/angular';
import { AuthStore } from '../../state/auth.store';
import { ToastService } from '../../shared/components/toast/toast.service';
import { ButtonComponent } from '../../shared/components/button/button.component';

@Component({
    selector: 'app-auth-callback',
    standalone: true,
    changeDetection: ChangeDetectionStrategy.OnPush,
    imports: [CommonModule, ButtonComponent, LucideLoader2, LucideAlertCircle],
    template: `
        <div class="callback-container" role="main">
            <div class="callback-card">
                @if (errorMessage()) {
                    <div class="status-icon error" role="alert">
                        <svg lucideAlertCircle [size]="48" aria-hidden="true"></svg>
                    </div>
                    <h1 class="callback-title">Authentication Failed</h1>
                    <p class="callback-desc">{{ errorMessage() }}</p>
                    <div class="callback-actions">
                        <app-button variant="primary" size="default" (btnClick)="navigateHome()" ariaLabel="Return to Markets"> Return to Markets </app-button>
                    </div>
                } @else {
                    <div class="status-icon loading" role="status" aria-live="polite">
                        <svg lucideLoader2 [size]="48" class="spin" aria-hidden="true"></svg>
                    </div>
                    <h1 class="callback-title">Completing Sign-In</h1>
                    <p class="callback-desc">Verifying your Google credentials and securing session...</p>
                }
            </div>
        </div>
    `,
    styles: [
        `
            .callback-container {
                display: flex;
                align-items: center;
                justify-content: center;
                min-height: calc(100vh - 120px);
                padding: 24px;
            }

            .callback-card {
                display: flex;
                flex-direction: column;
                align-items: center;
                text-align: center;
                max-width: 440px;
                width: 100%;
                padding: 40px 32px;
                background-color: var(--surface, #13111c);
                border: 1px solid var(--border-default, rgba(255, 255, 255, 0.08));
                border-radius: var(--radius-lg, 16px);
                box-shadow: 0 16px 40px rgba(0, 0, 0, 0.4);
            }

            .status-icon {
                margin-bottom: 20px;
                display: flex;
                align-items: center;
                justify-content: center;
            }

            .status-icon.loading {
                color: var(--primary, #7c4dff);
            }

            .status-icon.error {
                color: var(--status-error, #f43f5e);
            }

            .spin {
                animation: spin 1s linear infinite;
            }

            @keyframes spin {
                from {
                    transform: rotate(0deg);
                }
                to {
                    transform: rotate(360deg);
                }
            }

            .callback-title {
                font-family: var(--font-display, sans-serif);
                font-size: 22px;
                font-weight: 700;
                color: var(--foreground, #ffffff);
                margin: 0 0 10px 0;
            }

            .callback-desc {
                font-family: var(--font-ui, sans-serif);
                font-size: 14px;
                color: var(--muted, #9d97b8);
                line-height: 1.5;
                margin: 0 0 24px 0;
            }

            .callback-actions {
                width: 100%;
                display: flex;
                justify-content: center;
            }
        `
    ]
})
export class AuthCallbackComponent implements OnInit {
    private readonly route = inject(ActivatedRoute);
    private readonly router = inject(Router);
    private readonly authStore = inject(AuthStore);
    private readonly toastService = inject(ToastService);

    readonly errorMessage = signal<string | null>(null);

    ngOnInit(): void {
        const queryParams = this.route.snapshot.queryParamMap;
        const error = queryParams.get('error');
        const code = queryParams.get('code');
        const state = queryParams.get('state');

        if (error) {
            const desc = queryParams.get('error_description') || error;
            this.errorMessage.set(`Google authentication was cancelled or rejected: ${desc}`);
            this.toastService.error('Sign-in Cancelled', desc);
            return;
        }

        if (!code) {
            this.errorMessage.set('Missing authorization code from Google OAuth redirect.');
            this.toastService.error('Authentication Error', 'No authorization code received');
            return;
        }

        // Validate state against sessionStorage to guard against CSRF
        if (typeof window !== 'undefined' && window.sessionStorage) {
            const savedState = sessionStorage.getItem('bayesmarket_oauth_state');
            sessionStorage.removeItem('bayesmarket_oauth_state');

            if (savedState && state && savedState !== state) {
                this.errorMessage.set('Security verification failed (state parameter mismatch). Please try logging in again.');
                this.toastService.error('Security Verification Failed', 'OAuth state verification failed. Request may have been forged.');
                return;
            }
        }

        // Exchange code and state for session
        this.authStore.loginWithGoogleCallback(code, state || undefined).subscribe({
            next: () => {
                this.router.navigate(['/']);
            },
            error: (err) => {
                const msg = err?.error?.message || 'Failed to complete Google authentication';
                this.errorMessage.set(msg);
            }
        });
    }

    navigateHome(): void {
        this.router.navigate(['/']);
    }
}
