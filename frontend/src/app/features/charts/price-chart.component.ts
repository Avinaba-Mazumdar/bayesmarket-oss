import {
    ChangeDetectionStrategy,
    Component,
    computed,
    DestroyRef,
    effect,
    ElementRef,
    inject,
    input,
    NgZone,
    OnDestroy,
    OnInit,
    signal,
    untracked,
    viewChild
} from '@angular/core';
import { AreaSeries, ColorType, createChart, IChartApi, ISeriesApi, Time, UTCTimestamp } from 'lightweight-charts';
import { LucideTrendingUp, LucideTrendingDown } from '@lucide/angular';
import { WebSocketService } from '../../core/services/websocket.service';
import { ThemeService } from '../../core/services/theme.service';
import { ButtonComponent } from '../../shared/components/button/button.component';

export type ChartTimeframe = '1H' | '1D' | '1W' | 'ALL';

interface ChartPoint {
    time: UTCTimestamp;
    value: number;
}

@Component({
    selector: 'app-price-chart',
    standalone: true,
    changeDetection: ChangeDetectionStrategy.OnPush,
    imports: [ButtonComponent, LucideTrendingUp, LucideTrendingDown],
    template: `
        <div class="price-chart-container" role="region" [attr.aria-label]="chartAriaLabel()">
            <!-- Header Bar: Current Probability & Timeframe Chips -->
            <div class="chart-header">
                <div class="probability-display">
                    <span class="prob-label">Implied Probability (YES)</span>
                    <div class="prob-stat-row">
                        <span class="prob-value tabular-nums">{{ formattedProbability() }}</span>
                        <span class="prob-change" [class.positive]="priceChange() >= 0" [class.negative]="priceChange() < 0">
                            @if (priceChange() >= 0) {
                                <svg lucideTrendingUp class="prob-change-icon" [size]="14" aria-hidden="true"></svg>
                                +{{ formattedChange() }}%
                            } @else {
                                <svg lucideTrendingDown class="prob-change-icon" [size]="14" aria-hidden="true"></svg>
                                -{{ formattedChange() }}%
                            }
                        </span>
                        <span class="timeframe-label">{{ selectedTimeframe() }}</span>
                    </div>
                </div>

                <!-- Timeframe Selector Chips (WCAG SC 2.5.5 Level AAA min 40x40px) -->
                <div class="timeframe-selector" role="group" aria-label="Chart timeframe selector">
                    @for (tf of timeframes; track tf) {
                        <app-button
                            variant="chip"
                            size="sm"
                            [selected]="selectedTimeframe() === tf"
                            [attr.aria-pressed]="selectedTimeframe() === tf"
                            [ariaLabel]="'Select ' + tf + ' timeframe'"
                            (btnClick)="selectTimeframe(tf)"
                        >
                            {{ tf }}
                        </app-button>
                    }
                </div>
            </div>

            <!-- Canvas Container -->
            <div #chartContainer class="chart-canvas-wrapper" aria-hidden="true"></div>

            <!-- Accessibility Alternative / Data Summary Table for Screen Readers -->
            <div class="sr-only">
                <table>
                    <caption>
                        Historical Implied Probabilities for
                        {{
                            selectedTimeframe()
                        }}
                    </caption>
                    <thead>
                        <tr>
                            <th scope="col">Time</th>
                            <th scope="col">Implied Probability</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr>
                            <td>Current</td>
                            <td>{{ formattedProbability() }}</td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>
    `,
    styles: [
        `
            :host {
                display: block;
                width: 100%;
            }

            .price-chart-container {
                background-color: var(--surface-card, #131126);
                border: 1px solid var(--hairline, #252140);
                border-radius: var(--radius-xl, 20px);
                padding: var(--space-lg, 20px);
                display: flex;
                flex-direction: column;
                gap: 16px;
                box-shadow: var(--shadow-sm);
            }

            .chart-header {
                display: flex;
                align-items: flex-start;
                justify-content: space-between;
                flex-wrap: wrap;
                gap: 16px;
            }

            .probability-display {
                display: flex;
                flex-direction: column;
                gap: 4px;
            }

            .prob-label {
                font-family: var(--font-ui);
                font-size: 13px;
                font-weight: 600;
                color: var(--muted, #9d97b8);
                letter-spacing: 0.2px;
            }

            .prob-stat-row {
                display: flex;
                align-items: baseline;
                gap: 12px;
            }

            .prob-value {
                font-family: var(--font-mono);
                font-size: 32px;
                font-weight: 700;
                color: var(--outcome-yes-text, #065f46);
                line-height: 1.1;
                font-feature-settings: 'tnum' 1;
            }

            .prob-change {
                display: inline-flex;
                align-items: center;
                gap: 4px;
                font-family: var(--font-mono);
                font-size: 14px;
                font-weight: 600;
                font-feature-settings: 'tnum' 1;
            }

            .prob-change-icon {
                flex-shrink: 0;
            }

            .prob-change.positive {
                color: var(--status-profit, #34d399);
            }

            .prob-change.negative {
                color: var(--status-loss, #fda4af);
            }

            .timeframe-label {
                font-family: var(--font-ui);
                font-size: 12px;
                color: var(--muted, #a2b4c9);
                background-color: var(--canvas-subtle, #0c1017);
                padding: 2px 8px;
                border-radius: var(--radius-sm, 6px);
                border: 1px solid var(--hairline, #1e2638);
            }

            .timeframe-selector {
                display: flex;
                align-items: center;
                gap: 6px;
                background-color: var(--canvas-subtle, #0c1017);
                padding: 4px;
                border-radius: var(--radius-lg, 14px);
                border: 1px solid var(--hairline, #1e2638);
            }

            .chart-canvas-wrapper {
                width: 100%;
                height: 360px;
                position: relative;
                overflow: hidden;
                border-radius: var(--radius-lg, 14px);
                background-color: var(--canvas, #07090e);
                border: 1px solid var(--hairline, #1e2638);
                contain: layout paint size;
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

            @media (max-width: 640px) {
                .chart-canvas-wrapper {
                    height: 280px;
                }
                .prob-value {
                    font-size: 26px;
                }
                .chart-header {
                    flex-direction: column;
                }
            }
        `
    ]
})
export class PriceChartComponent implements OnInit, OnDestroy {
    readonly marketId = input<string>('');
    readonly initialProbabilityYes = input<number>(0.5);

    protected readonly chartContainer = viewChild<ElementRef<HTMLDivElement>>('chartContainer');

    private readonly wsService = inject(WebSocketService);
    private readonly destroyRef = inject(DestroyRef);
    private readonly ngZone = inject(NgZone);
    private readonly themeService = inject(ThemeService);

    protected readonly timeframes: ChartTimeframe[] = ['1H', '1D', '1W', 'ALL'];
    readonly selectedTimeframe = signal<ChartTimeframe>('1D');
    readonly currentProbability = signal<number>(0.5);
    readonly priceChange = signal<number>(0.0);

    private chart: IChartApi | null = null;
    private areaSeries: ISeriesApi<'Area'> | null = null;
    private tickRafId: number | null = null;
    private pendingTick: { prob: number; date: Date } | null = null;
    private currentData: ChartPoint[] = [];

    readonly formattedProbability = computed(() => {
        const p = this.currentProbability();
        return `${(p * 100).toFixed(1)}%`;
    });

    protected readonly formattedChange = computed(() => {
        return Math.abs(this.priceChange()).toFixed(2);
    });

    protected readonly chartAriaLabel = computed(() => {
        return `Probability chart for market, current probability is ${this.formattedProbability()}`;
    });

    constructor() {
        // Stream live ticks into the chart series directly without triggering Angular CD cycles
        effect(() => {
            const tick = this.wsService.lastPriceTick();
            const activeId = this.marketId();

            if (tick && (!tick.market_id || tick.market_id === activeId)) {
                const yesPrice = parseFloat(tick.yes_price);
                if (!isNaN(yesPrice)) {
                    untracked(() => {
                        this.onNewPriceTick(yesPrice, tick.timestamp ? new Date(tick.timestamp) : new Date());
                    });
                }
            }
        });

        // Reactively update chart colors and series styling when active theme changes
        effect(() => {
            const isDark = this.themeService.isDark();
            this.updateChartTheme(isDark);
        });
    }

    ngOnInit(): void {
        const initProb = this.initialProbabilityYes();
        if (initProb && !isNaN(initProb)) {
            this.currentProbability.set(initProb);
        }
    }

    ngAfterViewInit(): void {
        this.initChart();
    }

    ngOnDestroy(): void {
        this.teardownChart();
        this.currentData = [];
    }

    private initChart(): void {
        const el = this.chartContainer()?.nativeElement;
        if (!el) return;

        const isDark = this.themeService.isDark();

        this.ngZone.runOutsideAngular(() => {
            this.chart = createChart(el, {
                autoSize: true,
                layout: {
                    background: { type: ColorType.Solid, color: isDark ? '#080711' : '#ffffff' },
                    textColor: isDark ? '#b8b3d4' : '#334155',
                    fontSize: 12,
                    fontFamily: "'JetBrains Mono', monospace"
                },
                grid: {
                    vertLines: { color: isDark ? 'rgba(37, 33, 64, 0.55)' : 'rgba(203, 213, 225, 0.65)', style: 1 },
                    horzLines: { color: isDark ? 'rgba(37, 33, 64, 0.55)' : 'rgba(203, 213, 225, 0.65)', style: 1 }
                },
                crosshair: {
                    vertLine: {
                        color: isDark ? '#c4b5fd' : '#4338ca',
                        width: 1,
                        style: 2,
                        labelBackgroundColor: isDark ? '#3600b3' : '#4338ca'
                    },
                    horzLine: {
                        color: isDark ? '#c4b5fd' : '#4338ca',
                        width: 1,
                        style: 2,
                        labelBackgroundColor: isDark ? '#3600b3' : '#4338ca'
                    }
                },
                rightPriceScale: {
                    borderColor: isDark ? '#352f5e' : '#cbd5e1',
                    scaleMargins: {
                        top: 0.1,
                        bottom: 0.1
                    }
                },
                timeScale: {
                    borderColor: isDark ? '#352f5e' : '#cbd5e1',
                    timeVisible: true,
                    secondsVisible: false
                }
            });

            this.areaSeries = this.chart.addSeries(AreaSeries, {
                topColor: isDark ? 'rgba(0, 220, 130, 0.35)' : 'rgba(6, 95, 70, 0.22)',
                bottomColor: isDark ? 'rgba(0, 220, 130, 0.01)' : 'rgba(6, 95, 70, 0.01)',
                lineColor: isDark ? '#00dc82' : '#065f46',
                lineWidth: 2,
                priceFormat: {
                    type: 'custom',
                    formatter: (price: number) => `${(price * 100).toFixed(1)}%`
                }
            });

            this.generateDataForTimeframe(this.selectedTimeframe());
        });
    }

    private updateChartTheme(isDark: boolean): void {
        if (!this.chart || !this.areaSeries) return;

        this.ngZone.runOutsideAngular(() => {
            this.chart?.applyOptions({
                layout: {
                    background: { type: ColorType.Solid, color: isDark ? '#080711' : '#ffffff' },
                    textColor: isDark ? '#b8b3d4' : '#334155'
                },
                grid: {
                    vertLines: { color: isDark ? 'rgba(37, 33, 64, 0.55)' : 'rgba(203, 213, 225, 0.65)', style: 1 },
                    horzLines: { color: isDark ? 'rgba(37, 33, 64, 0.55)' : 'rgba(203, 213, 225, 0.65)', style: 1 }
                },
                crosshair: {
                    vertLine: {
                        color: isDark ? '#c4b5fd' : '#4338ca',
                        labelBackgroundColor: isDark ? '#3600b3' : '#4338ca'
                    },
                    horzLine: {
                        color: isDark ? '#c4b5fd' : '#4338ca',
                        labelBackgroundColor: isDark ? '#3600b3' : '#4338ca'
                    }
                },
                rightPriceScale: {
                    borderColor: isDark ? '#352f5e' : '#cbd5e1'
                },
                timeScale: {
                    borderColor: isDark ? '#352f5e' : '#cbd5e1'
                }
            });

            this.areaSeries?.applyOptions({
                topColor: isDark ? 'rgba(0, 220, 130, 0.35)' : 'rgba(6, 95, 70, 0.22)',
                bottomColor: isDark ? 'rgba(0, 220, 130, 0.01)' : 'rgba(6, 95, 70, 0.01)',
                lineColor: isDark ? '#00dc82' : '#065f46'
            });
        });
    }

    private teardownChart(): void {
        this.ngZone.runOutsideAngular(() => {
            if (this.tickRafId !== null) {
                cancelAnimationFrame(this.tickRafId);
                this.tickRafId = null;
            }
            this.pendingTick = null;
            if (this.areaSeries) {
                this.areaSeries.setData([]);
                this.areaSeries = null;
            }
            if (this.chart) {
                this.chart.remove();
                this.chart = null;
            }
        });
    }

    selectTimeframe(tf: ChartTimeframe): void {
        this.selectedTimeframe.set(tf);
        this.ngZone.runOutsideAngular(() => {
            this.generateDataForTimeframe(tf);
        });
    }

    private generateDataForTimeframe(tf: ChartTimeframe): void {
        const nowSec = Math.floor(Date.now() / 1000);
        let count = 60;
        let stepSec = 60; // 1 min steps for 1H

        switch (tf) {
            case '1H':
                count = 60;
                stepSec = 60;
                break;
            case '1D':
                count = 96;
                stepSec = 900; // 15 min steps
                break;
            case '1W':
                count = 84;
                stepSec = 7200; // 2 hour steps
                break;
            case 'ALL':
                count = 120;
                stepSec = 21600; // 6 hour steps
                break;
        }

        const startSec = nowSec - count * stepSec;
        const targetProb = this.currentProbability();
        const data: ChartPoint[] = [];

        // Generate smooth historical curve converging towards target probability
        let p = Math.max(0.05, Math.min(0.95, targetProb - 0.08 + (Math.random() * 0.06 - 0.03)));
        for (let i = 0; i < count; i++) {
            const t = (startSec + i * stepSec) as UTCTimestamp;
            const progress = i / count;
            // Drift towards target
            const drift = (targetProb - p) * 0.15;
            const noise = (Math.random() - 0.5) * 0.02 * (1 - progress);
            p = Math.max(0.02, Math.min(0.98, p + drift + noise));
            data.push({ time: t, value: p });
        }

        // Final point exact current probability
        data[data.length - 1] = { time: nowSec as UTCTimestamp, value: targetProb };

        this.currentData = data;
        if (this.areaSeries) {
            this.areaSeries.setData(data as any);
            this.chart?.timeScale().fitContent();
        }

        const first = data[0].value;
        const last = data[data.length - 1].value;
        this.priceChange.set((last - first) * 100);
    }

    private onNewPriceTick(newProb: number, date: Date): void {
        if (this.currentProbability() !== newProb) {
            this.currentProbability.set(newProb);
        }
        this.pendingTick = { prob: newProb, date };
        if (this.tickRafId === null) {
            this.ngZone.runOutsideAngular(() => {
                this.tickRafId = requestAnimationFrame(() => {
                    this.tickRafId = null;
                    if (!this.pendingTick) return;
                    const { prob, date: tickDate } = this.pendingTick;
                    this.pendingTick = null;

                    const nowSec = Math.floor(tickDate.getTime() / 1000);
                    const lastTime = this.currentData.length > 0 ? (this.currentData[this.currentData.length - 1].time as number) : 0;
                    const time = (nowSec >= lastTime ? nowSec : lastTime) as UTCTimestamp;

                    if (this.areaSeries) {
                        try {
                            const point: ChartPoint = { time, value: prob };
                            this.areaSeries.update(point as any);
                            if (this.currentData.length > 0) {
                                const first = this.currentData[0].value;
                                const newChange = (prob - first) * 100;
                                if (Math.abs(this.priceChange() - newChange) > 0.001) {
                                    this.priceChange.set(newChange);
                                }
                            }
                        } catch {
                            // Ignore non-critical timestamp collision
                        }
                    }
                });
            });
        }
    }
}
