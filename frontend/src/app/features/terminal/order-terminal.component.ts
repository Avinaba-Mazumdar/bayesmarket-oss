import { ChangeDetectionStrategy, Component, computed, DestroyRef, effect, inject, input, output, signal, untracked } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormsModule } from '@angular/forms';
import { Subscription } from 'rxjs';
import { LucideArrowUp, LucideArrowDown } from '@lucide/angular';
import { ApiService } from '../../core/services/api.service';
import { BuyQuoteResponse, Market } from '../../core/models/market.model';
import { AuthStore } from '../../state/auth.store';
import { ButtonComponent } from '../../shared/components/button/button.component';
import { BadgeComponent } from '../../shared/components/badge/badge.component';
import { InputComponent } from '../../shared/components/input/input.component';
import { LabelComponent } from '../../shared/components/label/label.component';

export interface OrderIntent {
    marketId: string;
    marketTitle: string;
    outcome: 'YES' | 'NO';
    amountUSDC: string;
    quote: BuyQuoteResponse;
    maxSlippagePct: string;
    currentBalance: string;
    postTradeBalance: string;
}

@Component({
    selector: 'app-order-terminal',
    standalone: true,
    changeDetection: ChangeDetectionStrategy.OnPush,
    imports: [FormsModule, ButtonComponent, BadgeComponent, InputComponent, LabelComponent, LucideArrowUp, LucideArrowDown],
    template: `
        <div class="order-terminal-card" role="region" aria-label="Order Execution Terminal">
            <!-- Header: Title and Live Available Balance -->
            <div class="terminal-header">
                <span class="terminal-title">Trade Outcome</span>
                <div class="balance-display" aria-label="Available USDC Cash Balance">
                    @if (authStore.isAuthenticated()) {
                        <span class="balance-title">Balance:</span>
                        <span class="balance-num tabular-nums">{{ authStore.cashBalance() }}</span>
                    } @else {
                        <span class="balance-title">Mode:</span>
                        <span class="balance-num simulation-badge">Paper Trading</span>
                    }
                </div>
            </div>

            <!-- Segmented Dual-Coded Outcome Selector (WCAG AAA Touch Target >= 44x44px) -->
            <div class="outcome-toggle-group" role="radiogroup" aria-label="Outcome selection">
                <app-button
                    variant="yes"
                    size="lg"
                    [fullWidth]="true"
                    [selected]="selectedOutcome() === 'YES'"
                    [attr.aria-checked]="selectedOutcome() === 'YES'"
                    role="radio"
                    ariaLabel="Select outcome YES at implied probability"
                    (btnClick)="setOutcome('YES')"
                >
                    <svg lucideArrowUp class="outcome-glyph" [size]="16" aria-hidden="true"></svg>
                    <span class="outcome-text">BUY YES</span>
                    <span class="outcome-prob tabular-nums">{{ yesPriceCents() }}¢</span>
                </app-button>

                <app-button
                    variant="no"
                    size="lg"
                    [fullWidth]="true"
                    [selected]="selectedOutcome() === 'NO'"
                    [attr.aria-checked]="selectedOutcome() === 'NO'"
                    role="radio"
                    ariaLabel="Select outcome NO at implied probability"
                    (btnClick)="setOutcome('NO')"
                >
                    <svg lucideArrowDown class="outcome-glyph" [size]="16" aria-hidden="true"></svg>
                    <span class="outcome-text">BUY NO</span>
                    <span class="outcome-prob tabular-nums">{{ noPriceCents() }}¢</span>
                </app-button>
            </div>

            <!-- Amount Input Section with Embedded Max -->
            <div class="input-section">
                <app-label htmlFor="terminal-amount-input" size="default">Amount (USDC)</app-label>
                <app-input
                    id="terminal-amount-input"
                    type="number"
                    variant="mono"
                    size="default"
                    [value]="amountInput()"
                    (valueChange)="onAmountChange($event)"
                    placeholder="0.00"
                    ariaLabel="Amount in USDC"
                >
                    <span prefix class="currency-symbol" aria-hidden="true">$</span>
                    <app-button suffix variant="chip" size="sm" ariaLabel="Set amount to maximum available balance" (btnClick)="onSetMax()"> MAX </app-button>
                </app-input>
                <span id="amount-helper-text" class="sr-only"> Enter the amount of USDC you wish to spend on outcome {{ selectedOutcome() }} </span>
            </div>

            <!-- Quick Amount Presets (+10, +50, +100, Max, Clear) -->
            <div class="quick-chips-row" role="group" aria-label="Quick amount presets">
                <app-button variant="chip" size="sm" ariaLabel="Add 10 USDC" (btnClick)="addAmount(10)"> +$10 </app-button>
                <app-button variant="chip" size="sm" ariaLabel="Add 50 USDC" (btnClick)="addAmount(50)"> +$50 </app-button>
                <app-button variant="chip" size="sm" ariaLabel="Add 100 USDC" (btnClick)="addAmount(100)"> +$100 </app-button>
                <app-button variant="chip" size="sm" ariaLabel="Set amount to maximum available balance" (btnClick)="onSetMax()"> Max </app-button>
                <app-button variant="chip" size="sm" ariaLabel="Clear amount" (btnClick)="clearAmount()"> Clear </app-button>
            </div>

            <!-- User-Editable Slippage Tolerance Setting (Issue 10) -->
            <div class="slippage-setting-section">
                <div class="slippage-setting-header">
                    <span class="slippage-setting-title">Max Slippage Tolerance</span>
                    <span class="slippage-setting-current tabular-nums">{{ maxSlippageTolerancePct() }}%</span>
                </div>
                <div class="slippage-presets-row" role="group" aria-label="Slippage tolerance presets">
                    @for (preset of slippagePresets; track preset) {
                        <app-button
                            variant="chip"
                            size="sm"
                            [selected]="maxSlippageTolerancePct() === preset"
                            ariaLabel="Set max slippage to {{ preset }}%"
                            (btnClick)="setSlippage(preset)"
                        >
                            {{ preset }}%
                        </app-button>
                    }
                    <div class="custom-slippage-wrapper">
                        <input
                            type="number"
                            step="0.1"
                            min="0.1"
                            max="50"
                            class="custom-slippage-input"
                            [value]="maxSlippageTolerancePct()"
                            (input)="onSlippageInput($event)"
                            placeholder="Custom"
                            aria-label="Custom slippage percentage"
                        />
                        <span class="percent-suffix">%</span>
                    </div>
                </div>
            </div>

            <!-- Execution Estimate Breakdown (Authoritative CPMM Quote) -->
            <div class="quote-drawer" [class.loading]="isLoadingQuote()">
                <div class="quote-row">
                    <span class="quote-label">Est. Shares Received:</span>
                    <span class="quote-val tabular-nums shares-highlight">
                        {{ formattedSharesReceived() }}
                    </span>
                </div>
                <div class="quote-row">
                    <span class="quote-label">Avg. Execution Price:</span>
                    <span class="quote-val tabular-nums">
                        {{ formattedAvgPrice() }}
                    </span>
                </div>
                <div class="quote-row">
                    <span class="quote-label">Price Impact / Slippage:</span>
                    <div class="slippage-val-group">
                        <span class="quote-val tabular-nums">{{ formattedPriceImpact() }}%</span>
                        <app-badge [variant]="slippageBadgeVariant()" size="sm">
                            {{ slippageBadgeLabel() }}
                        </app-badge>
                    </div>
                </div>
                <div class="quote-row total-return-row">
                    <span class="quote-label">Potential Payout ($1/share):</span>
                    <div class="return-group">
                        <span class="return-payout tabular-nums">{{ potentialPayout() }}</span>
                        <span class="return-pct tabular-nums" [class.positive]="potentialReturnPct() > 0"> (+{{ potentialReturnPct() }}%) </span>
                    </div>
                </div>
            </div>

            <!-- Paper Trading Disclaimer Notice -->
            <div class="terminal-paper-notice" role="note">
                <span class="paper-dot"></span>
                <span class="paper-notice-text">Simulated paper trade — no real funds at risk</span>
            </div>

            <!-- Primary Trade Action Button (Triggers Two-Step Confirmation Review) -->
            <div class="action-footer">
                <app-button
                    [variant]="selectedOutcome() === 'YES' ? 'yes' : 'no'"
                    [selected]="true"
                    size="lg"
                    [fullWidth]="true"
                    [disabled]="isTradeDisabled()"
                    [ariaLabel]="tradeButtonLabel()"
                    (btnClick)="onRequestOrderReview()"
                >
                    {{ tradeButtonText() }}
                </app-button>

                @if (validationError()) {
                    <div class="validation-warning" role="alert">
                        {{ validationError() }}
                    </div>
                }
            </div>
        </div>
    `,
    styles: [
        `
            :host {
                display: block;
                width: 100%;
            }

            .order-terminal-card {
                background-color: var(--surface-card, #131126);
                border: 1px solid var(--hairline, #252140);
                border-radius: var(--radius-lg, 14px);
                padding: var(--space-lg, 20px);
                box-shadow: 0 8px 30px rgba(0, 0, 0, 0.4);
                display: flex;
                flex-direction: column;
                gap: 16px;
            }

            .terminal-header {
                display: flex;
                align-items: center;
                justify-content: space-between;
                border-bottom: 1px solid var(--hairline, #252140);
                padding-bottom: 12px;
            }

            .terminal-title {
                font-family: var(--font-ui);
                font-size: 16px;
                font-weight: 700;
                color: var(--ink, #f8f7ff);
                letter-spacing: -0.2px;
            }

            .balance-display {
                display: flex;
                align-items: center;
                gap: 6px;
                background-color: var(--canvas-subtle, #0e0c1c);
                padding: 4px 12px;
                border-radius: var(--radius-pill, 9999px);
                border: 1px solid var(--hairline, #252140);
            }

            .balance-title {
                font-family: var(--font-ui);
                font-size: 11px;
                font-weight: 600;
                color: var(--muted, #9d97b8);
            }

            .balance-num {
                font-family: var(--font-mono);
                font-size: 13px;
                font-weight: 700;
                color: var(--ink, #f8f7ff);
                font-feature-settings: 'tnum' 1;
            }

            .simulation-badge {
                font-size: 11px;
                color: var(--primary-border, #a855f7);
                background-color: rgba(168, 85, 247, 0.12);
                padding: 1px 6px;
                border-radius: var(--radius-pill, 9999px);
                border: 1px solid rgba(168, 85, 247, 0.3);
            }

            .terminal-paper-notice {
                display: flex;
                align-items: center;
                justify-content: center;
                gap: 6px;
                padding: 6px 10px;
                background-color: rgba(124, 77, 255, 0.06);
                border: 1px solid rgba(124, 77, 255, 0.2);
                border-radius: var(--radius-sm, 6px);
            }

            .paper-dot {
                width: 5px;
                height: 5px;
                border-radius: 50%;
                background-color: #a855f7;
            }

            .paper-notice-text {
                font-family: var(--font-ui);
                font-size: 11.5px;
                color: var(--muted, #9d97b8);
                letter-spacing: 0.1px;
            }

            .outcome-toggle-group {
                display: grid;
                grid-template-columns: 1fr 1fr;
                gap: 8px;
            }

            :host ::ng-deep .outcome-toggle-group app-button {
                display: block;
                width: 100%;
                min-width: 0;
            }

            :host ::ng-deep .outcome-toggle-group button {
                width: 100%;
                min-width: 0;
                white-space: nowrap !important;
                flex-wrap: nowrap !important;
                padding-left: 8px !important;
                padding-right: 8px !important;
                gap: 5px !important;
            }

            .outcome-glyph {
                display: inline-flex;
                align-items: center;
                justify-content: center;
                flex-shrink: 0;
                margin-right: 2px;
            }

            .outcome-text {
                font-weight: 700;
                letter-spacing: 0.2px;
                white-space: nowrap !important;
                flex-shrink: 0;
            }

            .outcome-prob {
                font-family: var(--font-mono);
                font-size: 12px;
                opacity: 0.9;
                margin-left: auto;
                white-space: nowrap !important;
                flex-shrink: 0;
            }

            .input-section {
                display: flex;
                flex-direction: column;
                gap: 8px;
            }

            .input-label {
                font-family: var(--font-ui);
                font-size: 13px;
                font-weight: 600;
                color: var(--muted, #9d97b8);
            }

            .input-wrapper {
                display: flex;
                align-items: center;
                background-color: var(--canvas-subtle, #0e0c1c);
                border: 1px solid var(--hairline, #252140);
                border-radius: var(--radius-md, 10px);
                padding: 4px 8px 4px 14px;
                gap: 8px;
                min-height: 44px;
                transition:
                    border-color 0.15s ease,
                    box-shadow 0.15s ease;
            }

            .input-wrapper:focus-within {
                border-color: var(--primary-border, #7c4dff);
                box-shadow: 0 0 0 3px rgba(124, 77, 255, 0.25);
            }

            .currency-symbol {
                font-family: var(--font-mono);
                font-size: 18px;
                font-weight: 700;
                color: var(--muted, #9d97b8);
            }

            .amount-field {
                flex: 1;
                background: transparent;
                border: none;
                color: var(--ink, #f8f7ff);
                font-family: var(--font-mono);
                font-size: 20px;
                font-weight: 700;
                outline: none;
                box-shadow: none;
                font-feature-settings: 'tnum' 1;
                min-width: 0;
            }

            .amount-field:focus,
            .amount-field:focus-visible {
                outline: none;
                box-shadow: none;
            }

            .quick-chips-row {
                display: flex;
                align-items: center;
                gap: 8px;
                flex-wrap: wrap;
            }

            .slippage-setting-section {
                display: flex;
                flex-direction: column;
                gap: 6px;
                padding: 10px 12px;
                background-color: var(--canvas-subtle, #0e0c1c);
                border: 1px solid var(--hairline, #252140);
                border-radius: var(--radius-md, 10px);
            }

            .slippage-setting-header {
                display: flex;
                align-items: center;
                justify-content: space-between;
            }

            .slippage-setting-title {
                font-family: var(--font-ui);
                font-size: 11.5px;
                font-weight: 600;
                color: var(--muted, #9d97b8);
            }

            .slippage-setting-current {
                font-family: var(--font-mono);
                font-size: 11.5px;
                font-weight: 700;
                color: var(--ink, #f8f7ff);
            }

            .slippage-presets-row {
                display: flex;
                align-items: center;
                gap: 6px;
                flex-wrap: wrap;
            }

            .custom-slippage-wrapper {
                display: flex;
                align-items: center;
                background-color: var(--canvas, #080613);
                border: 1px solid var(--hairline, #252140);
                border-radius: var(--radius-sm, 6px);
                padding: 2px 6px;
                gap: 2px;
                height: 28px;
                box-sizing: border-box;
                transition: border-color 0.15s ease;
            }

            .custom-slippage-wrapper:focus-within {
                border-color: var(--primary-border, #7c4dff);
            }

            .custom-slippage-input {
                width: 38px;
                background: transparent;
                border: none;
                color: var(--ink, #f8f7ff);
                font-family: var(--font-mono);
                font-size: 11.5px;
                font-weight: 600;
                outline: none;
                text-align: right;
                -moz-appearance: textfield;
            }

            .custom-slippage-input::-webkit-outer-spin-button,
            .custom-slippage-input::-webkit-inner-spin-button {
                -webkit-appearance: none;
                margin: 0;
            }

            .percent-suffix {
                font-family: var(--font-mono);
                font-size: 11px;
                color: var(--muted, #9d97b8);
            }

            .quote-drawer {
                background-color: var(--canvas-subtle, #0e0c1c);
                border: 1px solid var(--hairline, #252140);
                border-radius: var(--radius-md, 10px);
                padding: 14px;
                display: flex;
                flex-direction: column;
                gap: 10px;
                transition: opacity 0.15s ease;
            }

            .quote-drawer.loading {
                opacity: 0.6;
            }

            .quote-row {
                display: flex;
                align-items: center;
                justify-content: space-between;
                font-size: 13px;
            }

            .quote-label {
                font-family: var(--font-ui);
                color: var(--muted, #9d97b8);
            }

            .quote-val {
                font-family: var(--font-mono);
                font-weight: 600;
                color: var(--ink, #f8f7ff);
                font-feature-settings: 'tnum' 1;
            }

            .shares-highlight {
                font-size: 15px;
                font-weight: 700;
                color: var(--ink, #f8f7ff);
            }

            .slippage-val-group {
                display: flex;
                align-items: center;
                gap: 6px;
            }

            .total-return-row {
                border-top: 1px solid var(--hairline, #252140);
                padding-top: 10px;
                margin-top: 2px;
            }

            .return-group {
                display: flex;
                align-items: baseline;
                gap: 6px;
            }

            .return-payout {
                font-family: var(--font-mono);
                font-size: 15px;
                font-weight: 700;
                color: var(--outcome-yes-text, #065f46);
                font-feature-settings: 'tnum' 1;
            }

            .return-pct {
                font-family: var(--font-mono);
                font-size: 12px;
                font-weight: 600;
                color: var(--outcome-yes-text, #065f46);
            }

            .action-footer {
                display: flex;
                flex-direction: column;
                gap: 8px;
                margin-top: 4px;
            }

            .validation-warning {
                font-family: var(--font-ui);
                font-size: 12px;
                color: var(--status-warning, #fbbf24);
                text-align: center;
            }

            .sr-only {
                position: absolute;
                width: 1px;
                height: 1px;
                padding: 0;
                margin: -1px;
                overflow: hidden;
                clip: rect(0, 0, 0, 0);
                white-space: nowrap;
                border: 0;
            }
        `
    ]
})
export class OrderTerminalComponent {
    readonly market = input.required<Market>();
    readonly orderReviewRequested = output<OrderIntent>();

    protected readonly apiService = inject(ApiService);
    readonly authStore = inject(AuthStore);

    readonly selectedOutcome = signal<'YES' | 'NO'>('YES');
    readonly amountInput = signal<string>('50');
    readonly maxSlippageTolerancePct = signal<string>('1.00');
    readonly slippagePresets: readonly string[] = ['0.5', '1.0', '2.0', '5.0'];

    setSlippage(preset: string): void {
        this.maxSlippageTolerancePct.set(preset);
    }

    onSlippageInput(event: Event): void {
        const input = event.target as HTMLInputElement;
        if (!input) return;
        const val = parseFloat(input.value);
        if (!isNaN(val) && val >= 0.1 && val <= 50) {
            this.maxSlippageTolerancePct.set(val.toFixed(1));
        }
    }

    readonly latestQuote = signal<BuyQuoteResponse | null>(null);
    readonly isLoadingQuote = signal<boolean>(false);
    private quoteDebounceTimer: ReturnType<typeof setTimeout> | null = null;
    private inFlightQuoteSub: Subscription | null = null;
    private quoteCache = new Map<string, { quote: BuyQuoteResponse; timestamp: number }>();
    private readonly destroyRef = inject(DestroyRef);

    protected readonly yesPriceCents = computed(() => {
        const m = this.market();
        const p = parseFloat(m?.probability_yes_pct || '50.00');
        return isNaN(p) ? '50' : Math.round(p).toString();
    });

    protected readonly noPriceCents = computed(() => {
        const m = this.market();
        const p = parseFloat(m?.probability_no_pct || '50.00');
        return isNaN(p) ? '50' : Math.round(p).toString();
    });

    protected readonly currentNumericAmount = computed(() => {
        const parsed = parseFloat(this.amountInput());
        return isNaN(parsed) ? 0 : parsed;
    });

    protected readonly validationError = computed(() => {
        const amt = this.currentNumericAmount();
        if (amt <= 0) {
            return 'Enter a valid positive USDC amount';
        }
        if (!this.authStore.isAuthenticated()) {
            return null;
        }
        const balStr = this.authStore.cashBalance().replace(/[$,]/g, '');
        const balNum = parseFloat(balStr);
        if (!isNaN(balNum) && amt > balNum) {
            return `Amount exceeds available cash balance ($${balNum.toFixed(2)})`;
        }
        return null;
    });

    protected readonly isTradeDisabled = computed(() => {
        if (!this.authStore.isAuthenticated()) {
            return false;
        }
        return !!this.validationError() || this.isLoadingQuote() || !this.latestQuote();
    });

    protected readonly formattedSharesReceived = computed(() => {
        const q = this.latestQuote();
        if (!q) return '0.00';
        const num = parseFloat(q.shares_received);
        return isNaN(num) ? '0.00' : num.toFixed(2);
    });

    protected readonly formattedAvgPrice = computed(() => {
        const q = this.latestQuote();
        if (!q) return '$0.00';
        const num = parseFloat(q.avg_price);
        return isNaN(num) ? '$0.00' : `$${num.toFixed(4)}`;
    });

    protected readonly formattedPriceImpact = computed(() => {
        const q = this.latestQuote();
        if (!q) return '0.00';
        const num = parseFloat(q.price_impact_pct);
        return isNaN(num) ? '0.00' : Math.abs(num).toFixed(2);
    });

    protected readonly slippageBadgeVariant = computed(() => {
        const q = this.latestQuote();
        if (!q) return 'secondary';
        const num = Math.abs(parseFloat(q.price_impact_pct));
        if (num < 1.0) return 'profit';
        if (num <= 3.0) return 'warning';
        return 'destructive';
    });

    protected readonly slippageBadgeLabel = computed(() => {
        const q = this.latestQuote();
        if (!q) return 'Normal';
        const num = Math.abs(parseFloat(q.price_impact_pct));
        if (num < 1.0) return 'Minimal';
        if (num <= 3.0) return 'Moderate';
        return 'High Impact';
    });

    protected readonly potentialPayout = computed(() => {
        const q = this.latestQuote();
        if (!q) return '$0.00';
        const shares = parseFloat(q.shares_received);
        return isNaN(shares) ? '$0.00' : `$${shares.toFixed(2)}`;
    });

    protected readonly potentialReturnPct = computed(() => {
        const q = this.latestQuote();
        const amt = this.currentNumericAmount();
        if (!q || amt <= 0) return 0;
        const shares = parseFloat(q.shares_received);
        if (isNaN(shares)) return 0;
        const profit = shares - amt;
        return Math.round((profit / amt) * 100);
    });

    protected readonly tradeButtonText = computed(() => {
        if (!this.authStore.isAuthenticated()) {
            return 'Sign in to Trade';
        }
        const outcome = this.selectedOutcome();
        const amt = this.currentNumericAmount();
        return `Trade $${amt.toFixed(2)} on ${outcome}`;
    });

    protected readonly tradeButtonLabel = computed(() => {
        if (!this.authStore.isAuthenticated()) {
            return 'Sign in or continue as guest to start trading';
        }
        return `Review and place trade for $${this.currentNumericAmount().toFixed(2)} USDC on outcome ${this.selectedOutcome()}`;
    });

    constructor() {
        this.destroyRef.onDestroy(() => {
            if (this.quoteDebounceTimer) {
                clearTimeout(this.quoteDebounceTimer);
            }
            if (this.inFlightQuoteSub) {
                this.inFlightQuoteSub.unsubscribe();
            }
        });

        // Automatically fetch quote whenever market, outcome, or amount changes
        effect(() => {
            const m = this.market();
            const outcome = this.selectedOutcome();
            const amt = this.amountInput();

            if (m && m.id && parseFloat(amt) > 0) {
                untracked(() => {
                    this.scheduleQuoteFetch(m.id, outcome, amt);
                });
            }
        });
    }

    setOutcome(outcome: 'YES' | 'NO'): void {
        this.selectedOutcome.set(outcome);
    }

    onAmountChange(val: string | number): void {
        this.amountInput.set(val !== '' && val !== null && val !== undefined ? val.toString() : '');
    }

    addAmount(addition: number): void {
        const current = this.currentNumericAmount();
        this.amountInput.set((current + addition).toString());
    }

    clearAmount(): void {
        this.amountInput.set('0');
    }

    onSetMax(): void {
        if (!this.authStore.isAuthenticated()) {
            this.authStore.openAuthModal();
            return;
        }
        const balStr = this.authStore.cashBalance().replace(/[$,]/g, '');
        const balNum = parseFloat(balStr);
        if (!isNaN(balNum) && balNum > 0) {
            this.amountInput.set(balNum.toFixed(0));
        }
    }

    private scheduleQuoteFetch(marketId: string, outcome: 'YES' | 'NO', amountStr: string): void {
        if (this.quoteDebounceTimer) {
            clearTimeout(this.quoteDebounceTimer);
        }
        if (this.inFlightQuoteSub) {
            this.inFlightQuoteSub.unsubscribe();
            this.inFlightQuoteSub = null;
        }

        const amtNum = parseFloat(amountStr);
        if (isNaN(amtNum) || amtNum <= 0) {
            this.isLoadingQuote.set(false);
            return;
        }

        // 1. Client-Side Cache Check (1s TTL to prevent confirm-time staleness)
        const cacheKey = `${marketId}:${outcome}:${amtNum.toFixed(2)}`;
        const now = Date.now();
        const cached = this.quoteCache.get(cacheKey);
        if (cached && now - cached.timestamp < 1000) {
            this.latestQuote.set(cached.quote);
            this.isLoadingQuote.set(false);
            return;
        }

        // 2. Fast Optimistic Quote for Instant Perceived Responsiveness
        const m = this.market();
        const probStr = outcome === 'YES' ? m?.probability_yes : m?.probability_no;
        const prob = probStr ? parseFloat(probStr) : 0.5;
        if (prob > 0 && (!this.latestQuote() || this.latestQuote()?.outcome !== outcome)) {
            const estShares = amtNum / prob;
            this.latestQuote.set({
                market_id: marketId,
                action: 'BUY',
                outcome: outcome,
                deposit_usdc: amtNum.toFixed(8),
                shares_received: estShares.toFixed(8),
                avg_price: prob.toFixed(8),
                initial_price: prob.toFixed(8),
                new_price: prob.toFixed(8),
                price_impact_pct: '0.00000000',
                new_reserve_yes: m?.reserves?.reserve_yes || '0',
                new_reserve_no: m?.reserves?.reserve_no || '0',
                new_collateral: m?.reserves?.collateral_reserve || '0'
            });
        }

        this.isLoadingQuote.set(true);
        this.quoteDebounceTimer = setTimeout(() => {
            this.inFlightQuoteSub = this.apiService
                .getQuote(marketId, {
                    action: 'BUY',
                    outcome: outcome,
                    amount_usdc: amtNum.toFixed(8)
                })
                .pipe(takeUntilDestroyed(this.destroyRef))
                .subscribe({
                    next: (quote) => {
                        this.quoteCache.set(cacheKey, { quote, timestamp: Date.now() });
                        this.latestQuote.set(quote);
                        this.isLoadingQuote.set(false);
                        this.inFlightQuoteSub = null;
                    },
                    error: (err) => {
                        console.warn('Quote calculation failed:', err);
                        this.isLoadingQuote.set(false);
                        this.inFlightQuoteSub = null;
                    }
                });
        }, 150);
    }

    onRequestOrderReview(): void {
        if (!this.authStore.isAuthenticated()) {
            this.authStore.openAuthModal();
            return;
        }

        const m = this.market();
        if (!m || this.isTradeDisabled()) return;

        const tradeAmt = this.currentNumericAmount();
        this.isLoadingQuote.set(true);

        // Clear quote cache to guarantee up-to-date CPMM quote for confirmation review
        this.quoteCache.clear();

        // Fresh un-cached execution quote to prevent review price drift (Issue 11)
        this.apiService
            .getQuote(
                m.id,
                {
                    action: 'BUY',
                    outcome: this.selectedOutcome(),
                    amount_usdc: tradeAmt.toFixed(8)
                },
                true // fresh quote bypassing server and client cache
            )
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: (freshQuote) => {
                    this.isLoadingQuote.set(false);
                    this.latestQuote.set(freshQuote);

                    const currentBalStr = this.authStore.cashBalance();
                    const currentBalNum = parseFloat(currentBalStr.replace(/[$,]/g, ''));
                    const postBalNum = Math.max(0, currentBalNum - tradeAmt);
                    const postTradeBalStr = `$${postBalNum.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;

                    this.orderReviewRequested.emit({
                        marketId: m.id,
                        marketTitle: m.title,
                        outcome: this.selectedOutcome(),
                        amountUSDC: tradeAmt.toFixed(8),
                        quote: freshQuote,
                        maxSlippagePct: this.maxSlippageTolerancePct(),
                        currentBalance: currentBalStr,
                        postTradeBalance: postTradeBalStr
                    });
                },
                error: (err) => {
                    this.isLoadingQuote.set(false);
                    console.warn('Confirm-time fresh quote fetch failed, falling back to latest quote:', err);
                    const fallbackQuote = this.latestQuote();
                    if (!fallbackQuote) return;

                    const currentBalStr = this.authStore.cashBalance();
                    const currentBalNum = parseFloat(currentBalStr.replace(/[$,]/g, ''));
                    const postBalNum = Math.max(0, currentBalNum - tradeAmt);
                    const postTradeBalStr = `$${postBalNum.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;

                    this.orderReviewRequested.emit({
                        marketId: m.id,
                        marketTitle: m.title,
                        outcome: this.selectedOutcome(),
                        amountUSDC: tradeAmt.toFixed(8),
                        quote: fallbackQuote,
                        maxSlippagePct: this.maxSlippageTolerancePct(),
                        currentBalance: currentBalStr,
                        postTradeBalance: postTradeBalStr
                    });
                }
            });
    }

    focusAmountInput(): void {
        const input = document.getElementById('terminal-amount-input') as HTMLInputElement | null;
        if (input) {
            input.focus();
            if (typeof input.select === 'function') {
                input.select();
            }
        }
    }
}
