import { ChangeDetectionStrategy, Component, computed, DestroyRef, inject, input, model, output, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { LucideArrowUp, LucideArrowDown, LucideAlertCircle } from '@lucide/angular';
import { ApiService } from '../../core/services/api.service';
import { AuthStore } from '../../state/auth.store';
import { ToastService } from '../../shared/components/toast/toast.service';
import { CashOutResponse, UserPosition } from '../../core/models/market.model';
import { DialogComponent } from '../../shared/components/dialog/dialog.component';
import { ButtonComponent } from '../../shared/components/button/button.component';
import { BadgeComponent } from '../../shared/components/badge/badge.component';

@Component({
    selector: 'app-cash-out-dialog',
    standalone: true,
    changeDetection: ChangeDetectionStrategy.OnPush,
    imports: [DialogComponent, ButtonComponent, BadgeComponent, LucideArrowUp, LucideArrowDown, LucideAlertCircle],
    template: `
        <app-dialog
            [open]="isOpen()"
            [title]="'Confirm AMM Liquidation'"
            [description]="'Liquidate your outcome shares back into the automated liquidity pool for USDC cash.'"
            size="default"
            role="alertdialog"
            [closeOnOverlayClick]="!isExecuting()"
            [closeOnEscape]="!isExecuting()"
            [showCloseButton]="!isExecuting()"
            (closed)="onDismiss()"
        >
            @if (position(); as pos) {
                <div class="liquidation-breakdown">
                    <!-- Market Summary Card -->
                    <div class="market-summary-box">
                        <span class="summary-label">Market</span>
                        <p class="market-question">{{ pos.market_title }}</p>
                    </div>

                    <!-- Outcome & Shares Badge Row -->
                    <div class="meta-badges-row">
                        <div class="meta-item">
                            <span class="meta-label">Liquidating Outcome</span>
                            <app-badge [variant]="pos.outcome === 'YES' ? 'outline' : 'destructive'" size="lg">
                                @if (pos.outcome === 'YES') {
                                    <svg lucideArrowUp class="badge-glyph" [size]="14" aria-hidden="true"></svg>
                                } @else {
                                    <svg lucideArrowDown class="badge-glyph" [size]="14" aria-hidden="true"></svg>
                                }
                                {{ pos.outcome }}
                            </app-badge>
                        </div>

                        <div class="meta-item">
                            <span class="meta-label">Shares to Sell</span>
                            <span class="shares-amount tabular-nums">{{ formattedShares() }} Shares</span>
                        </div>
                    </div>

                    <!-- Liquidation Financials Table -->
                    <div class="financial-table" role="table" aria-label="Liquidation calculation breakdown">
                        <div class="table-row" role="row">
                            <span class="cell-label" role="rowheader">Current Spot Price</span>
                            <span class="cell-val tabular-nums" role="cell">{{ '$' + formattedPrice() }}</span>
                        </div>

                        <div class="table-row" role="row">
                            <span class="cell-label" role="rowheader">Estimated Gross Proceeds</span>
                            <span class="cell-val tabular-nums highlight-val" role="cell">
                                {{ '$' + formattedProceeds() + ' USDC' }}
                            </span>
                        </div>

                        <div class="table-row" role="row">
                            <span class="cell-label" role="rowheader">Min Payout (2% Slippage Guard)</span>
                            <span class="cell-val tabular-nums" role="cell">
                                {{ '$' + formattedMinPayout() + ' USDC' }}
                            </span>
                        </div>

                        <div class="table-row" role="row">
                            <span class="cell-label" role="rowheader">Unrealized P&L Impact</span>
                            <span class="cell-val tabular-nums" [class.profit-val]="isPnLPositive()" [class.loss-val]="!isPnLPositive()" role="cell">
                                {{ pos.unrealized_pnl_usdc || pos.unrealized_pnl || '$0.00' }}
                                @if (pos.unrealized_pnl_pct) {
                                    ({{ pos.unrealized_pnl_pct }}%)
                                }
                            </span>
                        </div>

                        <div class="table-row balance-preview-row" role="row">
                            <span class="cell-label" role="rowheader">Est. Post-Liquidation Cash</span>
                            <span class="cell-val tabular-nums balance-preview" role="cell">
                                {{ '$' + estimatedPostBalance() }}
                            </span>
                        </div>
                    </div>

                    <div class="amm-notice" role="note">
                        <svg lucideAlertCircle class="notice-glyph" [size]="16" aria-hidden="true"></svg>
                        <span> Shares sell atomically into the AMM bonding curve. Proceeds are credited immediately to your USDC balance. </span>
                    </div>
                </div>
            }

            @if (position(); as pos) {
                <div footer class="dialog-actions-row">
                    <app-button variant="secondary" size="default" [disabled]="isExecuting()" ariaLabel="Cancel cash out liquidation" (btnClick)="onDismiss()">
                        Cancel
                    </app-button>

                    <app-button
                        variant="primary"
                        size="default"
                        [loading]="isExecuting()"
                        [disabled]="isExecuting()"
                        ariaLabel="Confirm liquidation and sell shares for USDC"
                        (btnClick)="onConfirmCashOut(pos)"
                    >
                        @if (isExecuting()) {
                            <span>Liquidating Shares...</span>
                        } @else {
                            <span>Confirm Cash Out</span>
                        }
                    </app-button>
                </div>
            }
        </app-dialog>
    `,
    styles: [
        `
            .liquidation-breakdown {
                display: flex;
                flex-direction: column;
                gap: 16px;
            }

            .market-summary-box {
                background-color: var(--canvas-subtle, #0e0c1c);
                border: 1px solid var(--hairline, #252140);
                border-radius: var(--radius-md, 10px);
                padding: 12px 14px;
                display: flex;
                flex-direction: column;
                gap: 4px;
            }

            .summary-label {
                font-family: var(--font-ui);
                font-size: 11px;
                font-weight: 700;
                text-transform: uppercase;
                letter-spacing: 0.5px;
                color: var(--muted, #9d97b8);
            }

            .market-question {
                font-family: var(--font-ui);
                font-size: 14.5px;
                font-weight: 600;
                color: var(--ink, #f8f7ff);
                margin: 0;
                line-height: 1.35;
            }

            .meta-badges-row {
                display: flex;
                align-items: center;
                justify-content: space-between;
                gap: 12px;
                background-color: var(--surface-card, #131126);
                border: 1px solid var(--hairline, #252140);
                border-radius: var(--radius-md, 10px);
                padding: 12px 14px;
            }

            .meta-item {
                display: flex;
                flex-direction: column;
                gap: 4px;
            }

            .meta-label {
                font-family: var(--font-ui);
                font-size: 11px;
                color: var(--muted, #9d97b8);
                text-transform: uppercase;
                letter-spacing: 0.3px;
            }

            .shares-amount {
                font-family: var(--font-mono);
                font-size: 15px;
                font-weight: 700;
                color: var(--ink, #f8f7ff);
            }

            .badge-glyph {
                margin-right: 4px;
                flex-shrink: 0;
            }

            .financial-table {
                display: flex;
                flex-direction: column;
                border: 1px solid var(--hairline, #252140);
                border-radius: var(--radius-md, 10px);
                background-color: var(--canvas, #080711);
                overflow: hidden;
            }

            .table-row {
                display: flex;
                align-items: center;
                justify-content: space-between;
                padding: 10px 14px;
                border-bottom: 1px solid var(--hairline, #252140);
                font-size: 13px;
            }

            .table-row:last-child {
                border-bottom: none;
            }

            .cell-label {
                font-family: var(--font-ui);
                color: var(--muted, #9d97b8);
            }

            .cell-val {
                font-family: var(--font-mono);
                font-weight: 600;
                color: var(--ink, #f8f7ff);
                font-feature-settings: 'tnum' 1;
            }

            .highlight-val {
                font-size: 15px;
                color: var(--status-profit, #065f46);
            }

            .profit-val {
                color: var(--status-profit, #065f46);
            }

            .loss-val {
                color: var(--status-loss, #9f1239);
            }

            .balance-preview-row {
                background-color: var(--status-info-bg, #f0f9ff);
            }

            .balance-preview {
                font-size: 14.5px;
                color: var(--status-info, #075985);
            }

            .amm-notice {
                display: flex;
                align-items: flex-start;
                gap: 8px;
                background-color: var(--primary-subtle, rgba(67, 56, 202, 0.08));
                border: 1px solid var(--primary-border, #4338ca);
                border-radius: var(--radius-md, 10px);
                padding: 10px 12px;
                font-family: var(--font-ui);
                font-size: 12px;
                color: var(--primary-text, #4338ca);
                line-height: 1.45;
            }

            .notice-glyph {
                flex-shrink: 0;
                margin-top: 1px;
            }

            .dialog-actions-row {
                display: flex;
                align-items: center;
                justify-content: flex-end;
                gap: 12px;
                width: 100%;
            }
        `
    ]
})
export class CashOutDialogComponent {
    readonly isOpen = model<boolean>(false);
    readonly position = input<UserPosition | null>(null);

    readonly cashedOut = output<CashOutResponse>();
    readonly dialogDismissed = output<void>();

    private readonly apiService = inject(ApiService);
    private readonly authStore = inject(AuthStore);
    private readonly toastService = inject(ToastService);
    private readonly destroyRef = inject(DestroyRef);

    readonly isExecuting = signal<boolean>(false);

    protected readonly formattedShares = computed(() => {
        const pos = this.position();
        if (!pos) return '0.00';
        const n = parseFloat(pos.shares_owned);
        return isNaN(n) ? '0.00' : n.toFixed(2);
    });

    protected readonly formattedPrice = computed(() => {
        const pos = this.position();
        if (!pos) return '0.0000';
        const n = parseFloat(pos.current_price);
        return isNaN(n) ? '0.0000' : n.toFixed(4);
    });

    protected readonly formattedProceeds = computed(() => {
        const pos = this.position();
        if (!pos) return '0.00';
        const val = parseFloat(pos.current_value_usdc || pos.market_value || '0');
        if (!isNaN(val) && val > 0) return val.toFixed(2);
        const shares = parseFloat(pos.shares_owned || '0');
        const price = parseFloat(pos.current_price || '0');
        return isNaN(shares) || isNaN(price) ? '0.00' : (shares * price).toFixed(2);
    });

    protected readonly formattedMinPayout = computed(() => {
        const proceeds = parseFloat(this.formattedProceeds()) || 0;
        const minPayout = Math.max(0, proceeds * 0.98);
        return minPayout.toFixed(2);
    });

    protected readonly isPnLPositive = computed(() => {
        const pos = this.position();
        if (!pos) return true;
        const pnl = parseFloat(pos.unrealized_pnl_usdc || pos.unrealized_pnl || '0');
        return pnl >= 0;
    });

    protected readonly estimatedPostBalance = computed(() => {
        const balStr = this.authStore.cashBalance().replace(/[$,]/g, '');
        const currentBal = parseFloat(balStr) || 0;
        const proceeds = parseFloat(this.formattedProceeds()) || 0;
        return (currentBal + proceeds).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
    });

    onDismiss(): void {
        if (this.isExecuting()) return;
        this.isOpen.set(false);
        this.dialogDismissed.emit();
    }

    onConfirmCashOut(pos: UserPosition): void {
        const token = this.authStore.token();
        if (!token) {
            this.toastService.error('Authentication Required', 'Please wait for your guest session to initialize.');
            return;
        }

        this.isExecuting.set(true);
        const idempotencyKey = `cashout-${Date.now()}-${Math.random().toString(36).substring(2, 9)}`;

        const proceeds = parseFloat(this.formattedProceeds()) || 0;
        // Apply 2% maximum slippage protection against front-running and AMM reserve drift
        const minPayout = Math.max(0, proceeds * 0.98);

        this.apiService
            .cashOut(
                {
                    market_id: pos.market_id,
                    outcome: pos.outcome,
                    shares: parseFloat(pos.shares_owned).toFixed(8),
                    min_payout_usdc: minPayout > 0 ? minPayout.toFixed(8) : undefined
                },
                token,
                idempotencyKey
            )
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: (receipt) => {
                    this.isExecuting.set(false);
                    this.isOpen.set(false);

                    // Update cash balance in AuthStore
                    const updatedBal = this.authStore.formatBalance(receipt.new_cash_balance);
                    this.authStore.updateBalance(updatedBal);

                    this.toastService.success(
                        'Shares Liquidated',
                        `Successfully sold ${parseFloat(receipt.shares_sold).toFixed(2)} ${receipt.outcome} shares for $${parseFloat(receipt.payout_usdc).toFixed(2)} USDC.`
                    );

                    this.cashedOut.emit(receipt);
                },
                error: (err) => {
                    this.isExecuting.set(false);
                    const msg = err?.error?.message || 'Failed to liquidate outcome shares back into the pool.';
                    this.toastService.error('Liquidation Failed', msg);
                }
            });
    }
}
