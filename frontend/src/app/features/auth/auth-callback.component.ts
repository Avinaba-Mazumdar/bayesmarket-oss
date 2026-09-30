import { ChangeDetectionStrategy, Component, inject, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router } from '@angular/router';
import { LucideAlertCircle, LucideLoader2 } from '@lucide/angular';
import { timeout } from 'rxjs';
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
                    <div class="callback-actions">
                        <app-button variant="ghost" size="sm" (btnClick)="navigateHome()" ariaLabel="Cancel and return to Markets">
                            Cancel & Return
                        </app-button>
                    </div>
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
                background-color: var(--canvas, #f8f9fc);
            }

            .callback-card {
                display: flex;
                flex-direction: column;
                align-items: center;
                text-align: center;
                max-width: 440px;
                width: 100%;
                padding: 40px 32px;
                background-color: var(--surface-card, #ffffff);
                border: 1px solid var(--hairline, #cbd5e1);
                border-radius: var(--radius-xl, 18px);
                box-shadow: var(--shadow-terminal, 0 10px 25px -5px rgba(15, 23, 42, 0.1));
            }

            .status-icon {
                margin-bottom: 20px;
                display: flex;
                align-items: center;
                justify-content: center;
            }

            .status-icon.loading {
                color: var(--primary, #4338ca);
            }

            .status-icon.error {
                color: var(--status-loss, #9f1239);
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
                font-family: var(--font-ui, system-ui, sans-serif);
                font-size: 20px;
                font-weight: 700;
                color: var(--ink, #0f172a);
                margin: 0 0 10px 0;
                letter-spacing: -0.2px;
            }

            .callback-desc {
                font-family: var(--font-ui, system-ui, sans-serif);
                font-size: 14px;
                color: var(--muted, #3b4861);
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

        // Validate state against sessionStorage to guard against CSRF and guest account takeover
        const savedState = typeof window !== 'undefined' && window.sessionStorage
            ? sessionStorage.getItem('bayesmarket_oauth_state')
            : null;
        if (typeof window !== 'undefined' && window.sessionStorage) {
            sessionStorage.removeItem('bayesmarket_oauth_state');
        }

        if (!savedState || !state || savedState !== state) {
            this.errorMessage.set('Security verification failed: missing or invalid OAuth state parameter. Please initiate sign-in again.');
            this.toastService.error('Security Verification Failed', 'OAuth state verification failed. Request may have been forged or session expired.');
            return;
        }

        // Exchange code and state for session with 12s timeout
        this.authStore
            .loginWithGoogleCallback(code, state || undefined)
            .pipe(timeout(12000))
            .subscribe({
                next: () => {
                    this.router.navigate(['/']);
                },
                error: (err) => {
                    const msg =
                        err?.name === 'TimeoutError'
                            ? 'Authentication timed out connecting to the server. Please check your network and try again.'
                            : err?.error?.message || 'Failed to complete Google authentication';
                    this.errorMessage.set(msg);
                }
            });
    }

    navigateHome(): void {
        this.router.navigate(['/']);
    }
}
