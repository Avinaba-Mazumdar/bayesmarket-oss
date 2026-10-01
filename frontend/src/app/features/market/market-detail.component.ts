import {
    ChangeDetectionStrategy,
    Component,
    computed,
    DestroyRef,
    effect,
    inject,
    input,
    OnDestroy,
    OnInit,
    signal,
    untracked,
    viewChild
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { RouterLink } from '@angular/router';
import { LucideArrowLeft, LucideArrowUp, LucideArrowDown, LucideCheckCircle2 } from '@lucide/angular';
import { ApiService } from '../../core/services/api.service';
import { WebSocketService } from '../../core/services/websocket.service';
import { Market, OrderResponse } from '../../core/models/market.model';
import { formatUSDC, formatShares } from '../../core/utils/formatters';
import { PriceChartComponent } from '../charts/price-chart.component';
import { OrderIntent, OrderTerminalComponent } from '../terminal/order-terminal.component';
import { OrderConfirmDialogComponent } from '../terminal/order-confirm-dialog.component';
import { ActivityFeedComponent } from '../trade/activity-feed.component';
import { GlossaryPopoverComponent } from './glossary-popover.component';
import { BadgeComponent } from '../../shared/components/badge/badge.component';
import { ButtonComponent } from '../../shared/components/button/button.component';

@Component({
    selector: 'app-market-detail',
    standalone: true,
    changeDetection: ChangeDetectionStrategy.OnPush,
    imports: [
        RouterLink,
        PriceChartComponent,
        OrderTerminalComponent,
        OrderConfirmDialogComponent,
        ActivityFeedComponent,
        GlossaryPopoverComponent,
        BadgeComponent,
        ButtonComponent,
        LucideArrowLeft,
        LucideArrowUp,
        LucideArrowDown,
        LucideCheckCircle2
    ],
    template: `
        <div class="cockpit-container">
            @if (isLoading()) {
                <div class="loading-container" role="status">
                    <div class="spinner" aria-hidden="true"></div>
                    <p>Loading trading cockpit...</p>
                </div>
            } @else if (!market()) {
                <div class="error-container" role="alert">
                    <h2>Market Not Found</h2>
                    <p>The requested prediction market could not be retrieved.</p>
                    <app-button variant="primary" size="default" routerLink="/"> Back to Markets </app-button>
                </div>
            } @else {
                <!-- Breadcrumb & Top Metadata Bar -->
                <nav class="breadcrumb-bar" aria-label="Market navigation breadcrumbs">
                    <a routerLink="/" class="breadcrumb-link">
                        <svg lucideArrowLeft [size]="14" aria-hidden="true"></svg>
                        <span>All Markets</span>
                    </a>
                    <span class="breadcrumb-separator" aria-hidden="true">/</span>
                    <app-badge variant="outline" size="sm">{{ market()!.category.toUpperCase() }}</app-badge>
                    <span class="breadcrumb-separator" aria-hidden="true">/</span>
                    <span class="breadcrumb-current">{{ market()!.title }}</span>
                </nav>

                <!-- Market Title & Primary Outcome Metrics -->
                <header class="market-hero-header">
                    <div class="title-meta-block">
                        <div class="status-tags-row">
                            <app-badge [variant]="statusBadgeVariant()" size="sm">
                                {{ market()!.status.toUpperCase() }}
                            </app-badge>
                            <span class="res-date"> Resolves {{ formattedResolutionDate() }} </span>
                        </div>
                        <h1 class="market-hero-title">
                            {{ market()!.title }}
                        </h1>
                    </div>

                    <div class="probability-pills-row" role="region" aria-label="Current spot probability">
                        <div class="spot-pill spot-yes">
                            <svg lucideArrowUp class="spot-glyph" [size]="14" aria-hidden="true"></svg>
                            <span class="spot-prob tabular-nums">{{ yesPct() }}%</span>
                            <span class="spot-tag">YES</span>
                        </div>
                        <div class="spot-pill spot-no">
                            <svg lucideArrowDown class="spot-glyph" [size]="14" aria-hidden="true"></svg>
                            <span class="spot-prob tabular-nums">{{ noPct() }}%</span>
                            <span class="spot-tag">NO</span>
                        </div>
                    </div>
                </header>

                <!-- Resolution Celebration Banner (WCAG live polite alert) -->
                @if (isResolved()) {
                    <section class="resolution-celebration-banner" role="alert" aria-live="polite">
                        <div class="banner-badge-icon">
                            <svg lucideCheckCircle2 [size]="28" aria-hidden="true"></svg>
                        </div>
                        <div class="banner-body">
                            <div class="banner-title-row">
                                <h2 class="banner-title">Market Resolved: {{ winningOutcomeDisplay() }} Won</h2>
                                <app-badge variant="secondary" size="sm">OFFICIAL SETTLEMENT</app-badge>
                            </div>
                            <p class="banner-description">
                                This binary prediction market has officially settled. All winning shares (<strong>{{ winningOutcomeDisplay() }}</strong
                                >) have been redeemed at the fixed oracle price of <strong>$1.00 USDC</strong> per share. Losing shares have expired at $0.00.
                            </p>
                        </div>
                    </section>
                }

                <!-- 2-Column Cockpit Grid (Reflows to Single Column on Mobile/Zoom) -->
                <div class="cockpit-grid">
                    <!-- Left Column: Canvas Chart, Resolution Criteria, Live Trades Tape -->
                    <main class="cockpit-main">
                        <!-- TradingView Canvas Probability Chart -->
                        <app-price-chart [marketId]="market()!.id" [initialProbabilityYes]="initialProbabilityNum()" />

                        <!-- Market Rules & Resolution Criteria (WCAG Level AAA Clarity) -->
                        <section class="resolution-card" aria-labelledby="resolution-heading">
                            <div class="card-header">
                                <h2 id="resolution-heading" class="section-title">Resolution Criteria & Rules</h2>
                                <app-glossary-popover />
                            </div>

                            <p class="market-description">
                                {{ market()!.description }}
                            </p>

                            <div class="oracle-source-box">
                                <span class="oracle-label">Official Resolution Source</span>
                                <p class="oracle-text">{{ market()!.resolution_source }}</p>
                            </div>

                            <div class="liquidity-specs-grid">
                                <div class="spec-item">
                                    <span class="spec-label">Collateral Locked</span>
                                    <span class="spec-val tabular-nums">{{ '$' + formattedCollateral() }} USDC</span>
                                </div>
                                <div class="spec-item">
                                    <span class="spec-label">Total Volume</span>
                                    <span class="spec-val tabular-nums">{{ '$' + formattedVolume() }} USDC</span>
                                </div>
                                <div class="spec-item">
                                    <span class="spec-label">YES Pool Reserve</span>
                                    <span class="spec-val tabular-nums">{{ formattedReserveYes() }}</span>
                                </div>
                                <div class="spec-item">
                                    <span class="spec-label">NO Pool Reserve</span>
                                    <span class="spec-val tabular-nums">{{ formattedReserveNo() }}</span>
                                </div>
                            </div>
                        </section>

                        <!-- Live Trade Stream Tape -->
                        <app-activity-feed [marketId]="market()!.id" />
                    </main>

                    <!-- Right Column: Order Execution Terminal & Two-Step Confirmation Dialog -->
                    <aside class="cockpit-sidebar" aria-label="Trade Execution Sidebar">
                        <div class="sticky-sidebar-wrapper">
                            <app-order-terminal #orderTerminal [market]="market()!" (orderReviewRequested)="onOrderReviewRequested($event)" />
                        </div>
                    </aside>
                </div>

                <!-- Two-Step Financial Confirmation Review Dialog (SC 3.3.6) -->
                <app-order-confirm-dialog
                    [isOpen]="isConfirmDialogOpen()"
                    [intent]="pendingOrderIntent()"
                    (orderPlaced)="onOrderSuccessfullyPlaced($event)"
                    (dialogDismissed)="onOrderDialogDismissed()"
                />
            }
        </div>
    `,
    styles: [
        `
            :host {
                display: block;
            }

            .cockpit-container {
                max-width: 1440px;
                margin: 0 auto;
                padding: var(--space-lg, 20px) var(--space-lg, 20px) 64px;
                display: flex;
                flex-direction: column;
                gap: 24px;
            }

            .breadcrumb-bar {
                display: flex;
                align-items: center;
                gap: 8px;
                font-family: var(--font-ui);
                font-size: 13px;
            }

            .breadcrumb-link {
                color: var(--muted, #a2b4c9);
                text-decoration: none;
                font-weight: 600;
                min-height: var(--touch-target-min, 40px);
                display: inline-flex;
                align-items: center;
                gap: 6px;
                padding: 4px 8px;
                border-radius: var(--radius-sm, 6px);
            }

            .breadcrumb-link:hover {
                color: var(--ink, #f8fafc);
                background-color: var(--surface-card, #111622);
            }

            .breadcrumb-separator {
                color: var(--hairline, #1e2638);
            }

            .breadcrumb-current {
                color: var(--ink-secondary, #cbd5e1);
                white-space: nowrap;
                overflow: hidden;
                text-overflow: ellipsis;
                max-width: 400px;
            }

            .market-hero-header {
                display: flex;
                align-items: flex-start;
                justify-content: space-between;
                gap: 24px;
                flex-wrap: wrap;
                border-bottom: 1px solid var(--hairline, #1e293b);
                padding-bottom: 20px;
            }

            .resolution-celebration-banner {
                display: flex;
                align-items: flex-start;
                gap: 16px;
                padding: 18px 24px;
                background-color: var(--status-profit-bg, #ecfdf5);
                border: 1px solid var(--status-profit-border, #059669);
                border-radius: var(--radius-lg, 14px);
                box-shadow: var(--shadow-sm);
            }

            .banner-badge-icon {
                color: var(--status-profit, #065f46);
                flex-shrink: 0;
                display: flex;
                align-items: center;
                margin-top: 2px;
            }

            .banner-body {
                display: flex;
                flex-direction: column;
                gap: 6px;
            }

            .banner-title-row {
                display: flex;
                align-items: center;
                gap: 12px;
                flex-wrap: wrap;
            }

            .banner-title {
                margin: 0;
                font-family: var(--font-ui);
                font-size: 18px;
                font-weight: 700;
                color: var(--ink, #f8fafc);
            }

            .banner-description {
                margin: 0;
                font-family: var(--font-ui);
                font-size: 13.5px;
                color: var(--ink-secondary, #94a3b8);
                line-height: 1.5;
            }

            .title-meta-block {
                flex: 1;
                min-width: 320px;
                display: flex;
                flex-direction: column;
                gap: 8px;
            }

            .status-tags-row {
                display: flex;
                align-items: center;
                gap: 12px;
            }

            .res-date {
                font-family: var(--font-ui);
                font-size: 12px;
                color: var(--muted, #94a3b8);
            }

            .market-hero-title {
                margin: 0;
                font-family: var(--font-ui);
                font-size: 26px;
                font-weight: 700;
                color: var(--ink, #f8fafc);
                line-height: 1.3;
                letter-spacing: -0.3px;
            }

            .probability-pills-row {
                display: flex;
                align-items: center;
                gap: 12px;
            }

            .spot-pill {
                display: flex;
                align-items: center;
                gap: 8px;
                padding: 8px 16px;
                border-radius: var(--radius-pill, 9999px);
                user-select: none;
                min-height: var(--touch-target-min, 40px);
            }

            .spot-yes {
                background-color: var(--outcome-yes-subtle, rgba(6, 95, 70, 0.09));
                border: 1.5px solid var(--outcome-yes-border, rgba(6, 95, 70, 0.35));
                color: var(--outcome-yes-text, #065f46);
            }

            .spot-no {
                background-color: var(--outcome-no-subtle, rgba(159, 18, 57, 0.09));
                border: 1.5px solid var(--outcome-no-border, rgba(159, 18, 57, 0.35));
                color: var(--outcome-no-text, #9f1239);
            }

            .spot-glyph {
                display: inline-flex;
                align-items: center;
                justify-content: center;
                flex-shrink: 0;
            }

            .spot-prob {
                font-family: var(--font-mono);
                font-size: 20px;
                font-weight: 800;
                font-feature-settings: 'tnum' 1;
            }

            .spot-tag {
                font-family: var(--font-ui);
                font-size: 12px;
                font-weight: 700;
                opacity: 0.9;
            }

            /* --- 2-Column Grid --- */
            .cockpit-grid {
                display: grid;
                grid-template-columns: 1fr 380px;
                gap: 24px;
                align-items: start;
            }

            .cockpit-main {
                display: flex;
                flex-direction: column;
                gap: 24px;
                min-width: 0;
                contain: layout style;
            }

            .cockpit-sidebar {
                min-width: 0;
                contain: layout style;
            }

            .sticky-sidebar-wrapper {
                position: sticky;
                top: 80px;
            }

            /* --- Resolution Card --- */
            .resolution-card {
                background-color: var(--surface-card, #131126);
                border: 1px solid var(--hairline, #252140);
                border-radius: var(--radius-lg, 14px);
                padding: 24px;
                display: flex;
                flex-direction: column;
                gap: 16px;
            }

            .card-header {
                display: flex;
                align-items: center;
                justify-content: space-between;
                gap: 12px;
            }

            .section-title {
                margin: 0;
                font-family: var(--font-ui);
                font-size: 17px;
                font-weight: 700;
                color: var(--ink, #f8f7ff);
            }

            .market-description {
                font-family: var(--font-ui);
                font-size: 14.5px;
                line-height: 1.6;
                color: var(--muted, #9d97b8);
                margin: 0;
            }

            .oracle-source-box {
                background-color: var(--canvas-subtle, #0e0c1c);
                border: 1px solid var(--hairline, #252140);
                border-radius: var(--radius-md, 10px);
                padding: 12px 16px;
                display: flex;
                flex-direction: column;
                gap: 4px;
            }

            .oracle-label {
                font-family: var(--font-ui);
                font-size: 11px;
                font-weight: 700;
                text-transform: uppercase;
                letter-spacing: 0.5px;
                color: var(--muted, #9d97b8);
            }

            .oracle-text {
                font-family: var(--font-ui);
                font-size: 13.5px;
                color: var(--accent, #00d4ff);
                margin: 0;
            }

            .liquidity-specs-grid {
                display: grid;
                grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
                gap: 12px;
                border-top: 1px solid var(--hairline, #252140);
                padding-top: 16px;
            }

            .spec-item {
                display: flex;
                flex-direction: column;
                gap: 4px;
            }

            .spec-label {
                font-family: var(--font-ui);
                font-size: 11px;
                color: var(--muted, #9d97b8);
                text-transform: uppercase;
            }

            .spec-val {
                font-family: var(--font-mono);
                font-size: 14px;
                font-weight: 600;
                color: var(--ink, #f8f7ff);
                font-feature-settings: 'tnum' 1;
            }

            .loading-container,
            .error-container {
                display: flex;
                flex-direction: column;
                align-items: center;
                justify-content: center;
                padding: 100px 20px;
                text-align: center;
                gap: 16px;
            }

            .spinner {
                width: 40px;
                height: 40px;
                border: 3px solid rgba(124, 77, 255, 0.2);
                border-top-color: var(--primary-border, #7c4dff);
                border-radius: 50%;
                animation: spin 0.8s linear infinite;
            }

            @keyframes spin {
                to {
                    transform: rotate(360deg);
                }
            }

            @media (max-width: 1024px) {
                .cockpit-grid {
                    grid-template-columns: 1fr;
                }
                .sticky-sidebar-wrapper {
                    position: static;
                }
                .market-hero-title {
                    font-size: 22px;
                }
            }
        `
    ]
})
export class MarketDetailComponent implements OnInit, OnDestroy {
    readonly id = input.required<string>();

    private readonly apiService = inject(ApiService);
    private readonly wsService = inject(WebSocketService);
    private readonly destroyRef = inject(DestroyRef);

    readonly market = signal<Market | null>(null);
    readonly isLoading = signal<boolean>(true);

    protected readonly orderTerminal = viewChild<OrderTerminalComponent>('orderTerminal');

    // Two-step confirmation state
    readonly isConfirmDialogOpen = signal<boolean>(false);
    readonly pendingOrderIntent = signal<OrderIntent | null>(null);

    // Oracle Market Resolution state
    readonly isResolvedBannerVisible = signal<boolean>(false);
    readonly winningOutcome = signal<string | null>(null);

    protected readonly isResolved = computed(() => {
        return this.market()?.status === 'resolved' || this.isResolvedBannerVisible();
    });

    protected readonly winningOutcomeDisplay = computed(() => {
        return this.winningOutcome() || this.market()?.winning_outcome || 'YES';
    });

    constructor() {
        effect(() => {
            const res = this.wsService.lastMarketResolved();
            const targetId = this.id();
            if (res && res.market_id === targetId) {
                untracked(() => {
                    this.isResolvedBannerVisible.set(true);
                    this.winningOutcome.set(res.winning_outcome);
                    const current = this.market();
                    if (current) {
                        this.market.set({
                            ...current,
                            status: 'resolved',
                            winning_outcome: res.winning_outcome
                        });
                    }
                });
            }
        });
    }

    protected readonly yesPct = computed(() => {
        const m = this.market();
        if (!m) return 50;
        const p = parseFloat(m.probability_yes_pct);
        return isNaN(p) ? 50 : Math.round(p);
    });

    protected readonly noPct = computed(() => {
        return Math.max(0, 100 - this.yesPct());
    });

    protected readonly initialProbabilityNum = computed(() => {
        const m = this.market();
        if (!m) return 0.5;
        const p = parseFloat(m.probability_yes);
        return isNaN(p) ? 0.5 : p;
    });

    protected readonly statusBadgeVariant = computed(() => {
        const s = this.market()?.status;
        if (s === 'active') return 'outline';
        if (s === 'resolved') return 'secondary';
        return 'destructive';
    });

    protected readonly formattedResolutionDate = computed(() => {
        const raw = this.market()?.resolution_date;
        if (!raw) return 'Soon';
        const d = new Date(raw);
        return d.toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' });
    });

    protected readonly formattedCollateral = computed(() => {
        return formatUSDC(this.market()?.reserves?.collateral_reserve).replace('$', '');
    });

    protected readonly formattedVolume = computed(() => {
        return formatUSDC(this.market()?.reserves?.total_volume_usdc).replace('$', '');
    });

    protected readonly formattedReserveYes = computed(() => {
        return formatShares(this.market()?.reserves?.reserve_yes);
    });

    protected readonly formattedReserveNo = computed(() => {
        return formatShares(this.market()?.reserves?.reserve_no);
    });

    ngOnInit(): void {
        this.fetchMarket();
    }

    ngOnDestroy(): void {
        this.wsService.disconnect();
        this.wsService.clearTrades();
        this.wsService.lastPriceTick.set(null);
        this.market.set(null);
        this.pendingOrderIntent.set(null);
    }

    private fetchMarket(): void {
        const marketId = this.id();
        this.isLoading.set(true);

        this.apiService
            .getMarketById(marketId)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: (data) => {
                    this.market.set(data);
                    this.isLoading.set(false);
                    // Connect WebSocket to market topic
                    this.wsService.connect(data.id);
                },
                error: (err) => {
                    console.error('Failed to load market:', err);
                    this.isLoading.set(false);
                }
            });
    }

    onOrderReviewRequested(intent: OrderIntent): void {
        this.pendingOrderIntent.set(intent);
        this.isConfirmDialogOpen.set(true);
    }

    onOrderSuccessfullyPlaced(receipt: OrderResponse): void {
        const execPrice = parseFloat(receipt.execution_price);
        if (!isNaN(execPrice) && execPrice > 0) {
            const isYes = receipt.outcome === 'YES';
            const priceYes = isYes ? execPrice : Math.max(0, 1 - execPrice);
            const priceNo = isYes ? Math.max(0, 1 - execPrice) : execPrice;
            this.wsService.lastPriceTick.set({
                type: 'PRICE_UPDATE',
                market_id: this.id(),
                yes_price: priceYes.toFixed(4),
                no_price: priceNo.toFixed(4),
                timestamp: receipt.created_at || new Date().toISOString()
            });
        }
        // Re-fetch market to update reserves, spot price, and volume
        this.fetchMarket();
    }

    onOrderDialogDismissed(): void {
        this.isConfirmDialogOpen.set(false);
        this.pendingOrderIntent.set(null);
        setTimeout(() => this.orderTerminal()?.focusAmountInput(), 16);
    }
}
