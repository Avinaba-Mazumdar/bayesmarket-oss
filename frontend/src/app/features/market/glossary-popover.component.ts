import { ChangeDetectionStrategy, Component, model, signal } from '@angular/core';
import { LucideBookOpen } from '@lucide/angular';
import { DialogComponent } from '../../shared/components/dialog/dialog.component';
import { ButtonComponent } from '../../shared/components/button/button.component';

interface GlossaryTerm {
    term: string;
    definition: string;
    plainEnglish: string;
}

@Component({
    selector: 'app-glossary-popover',
    standalone: true,
    changeDetection: ChangeDetectionStrategy.OnPush,
    imports: [DialogComponent, ButtonComponent, LucideBookOpen],
    template: `
        <app-button variant="ghost" size="sm" ariaLabel="Open Prediction Market Trading Glossary" (btnClick)="open()">
            <svg lucideBookOpen class="glossary-icon" [size]="16" aria-hidden="true"></svg>
            <span>Trading Guide & Glossary</span>
        </app-button>

        <app-dialog
            [open]="isOpen()"
            title="Trading Glossary & Concepts"
            description="Clear explanations of prediction market mechanics."
            size="lg"
            (closed)="isOpen.set(false)"
        >
            <div class="glossary-content">
                <!-- Plain-Language Summary Box (Flesch-Kincaid Grade 7-8) -->
                <div class="plain-english-summary">
                    <h3 class="summary-heading">How BayesMarket Works in Simple Words</h3>
                    <p class="summary-text">
                        You are predicting what will happen in the real world. Every event has two answers: <strong>YES</strong> or <strong>NO</strong>. Prices
                        always add up to $1.00. For example, if YES costs 70¢, the crowd thinks there is a 70% chance it happens. If you are right, each winning
                        share pays $1.00 cash. If you are wrong, it pays $0.00. You can cash out your shares at the current market price whenever you want
                        before the event concludes.
                    </p>
                </div>

                <!-- Terms Accordion / Definition List -->
                <dl class="terms-list">
                    @for (item of terms; track item.term) {
                        <div class="term-card">
                            <dt class="term-title">{{ item.term }}</dt>
                            <dd class="term-def">{{ item.definition }}</dd>
                            <dd class="term-plain">
                                <span class="plain-label">Plain English:</span>
                                {{ item.plainEnglish }}
                            </dd>
                        </div>
                    }
                </dl>
            </div>

            <div footer>
                <app-button variant="primary" size="default" (btnClick)="close()"> Got It </app-button>
            </div>
        </app-dialog>
    `,
    styles: [
        `
            .glossary-icon {
                display: inline-flex;
                align-items: center;
                justify-content: center;
                flex-shrink: 0;
                margin-right: 6px;
            }

            .glossary-content {
                display: flex;
                flex-direction: column;
                gap: 20px;
                max-height: 60vh;
                overflow-y: auto;
                padding-right: 4px;
            }

            .plain-english-summary {
                background-color: var(--primary-subtle, rgba(79, 70, 229, 0.08));
                border: 1px solid var(--primary-border, #6366f1);
                border-radius: var(--radius-lg, 14px);
                padding: 16px;
            }

            .summary-heading {
                font-family: var(--font-ui);
                font-size: 15px;
                font-weight: 700;
                color: var(--primary-text, #4338ca);
                margin-bottom: 8px;
            }

            .summary-text {
                font-family: var(--font-ui);
                font-size: 13.5px;
                line-height: 1.6;
                color: var(--ink, #f8fafc);
                margin: 0;
            }

            .terms-list {
                display: flex;
                flex-direction: column;
                gap: 12px;
                margin: 0;
            }

            .term-card {
                background-color: var(--canvas-subtle, #0c1017);
                border: 1px solid var(--hairline, #1e2638);
                border-radius: var(--radius-md, 10px);
                padding: 14px;
                display: flex;
                flex-direction: column;
                gap: 6px;
            }

            .term-title {
                font-family: var(--font-ui);
                font-size: 15px;
                font-weight: 700;
                color: var(--primary-border, #6366f1);
            }

            .term-def {
                font-family: var(--font-ui);
                font-size: 13px;
                line-height: 1.5;
                color: var(--ink-secondary, #cbd5e1);
                margin: 0;
            }

            .term-plain {
                font-family: var(--font-ui);
                font-size: 12.5px;
                color: var(--status-info, #075985);
                margin: 0;
                background-color: var(--status-info-bg, #f0f9ff);
                border: 1px solid var(--status-info-border, #0284c7);
                padding: 6px 10px;
                border-radius: var(--radius-sm, 6px);
                line-height: 1.45;
            }

            .plain-label {
                font-weight: 700;
                color: var(--ink, #0f172a);
            }
        `
    ]
})
export class GlossaryPopoverComponent {
    readonly isOpen = model<boolean>(false);

    protected readonly terms: GlossaryTerm[] = [
        {
            term: 'Automated Market Maker (AMM)',
            definition:
                'A decentralized system of algorithmic liquidity pools that enables assets to be traded automatically without traditional order books or centralized intermediaries.',
            plainEnglish:
                'A computer program that automatically sets prices and holds money so anyone can buy or sell shares instantly without needing a matching buyer.'
        },
        {
            term: 'Constant Product Market Maker (CPMM)',
            definition:
                'An automated liquidity pool governed by the invariant formula x * y = k. Unlike traditional order books with buy/sell queues, a mathematical algorithm quotes instant buy and sell prices continuously.',
            plainEnglish: 'A robot that is always ready to trade with you automatically, without waiting for another person to accept your offer.'
        },
        {
            term: 'Implied Probability',
            definition:
                'The spot price of an outcome share between $0.00 and $1.00, reflecting the aggregate consensus probability of the event occurring (P_YES + P_NO = 1.00).',
            plainEnglish: 'If YES costs 65¢, the market believes there is a 65% probability the event will happen.'
        },
        {
            term: 'Slippage & Price Impact',
            definition:
                'The difference between the quoted spot price before an order and the actual average execution price resulting from shifting liquidity pool reserves.',
            plainEnglish: 'When you buy a large amount, the price moves slightly up as you purchase. Smaller orders experience little to no price shift.'
        },
        {
            term: 'Complete-Set Collateralization',
            definition:
                'Every 1.00 USDC deposited simultaneously mints 1 YES share and 1 NO share. Because one outcome will occur, the winning share is guaranteed to be 100% redeemable for $1.00 from collateral reserves.',
            plainEnglish: 'All winning payouts are fully locked in the exchange beforehand. There is no debt or default risk.'
        },
        {
            term: 'Oracle Resolution',
            definition:
                'The deterministic external data source (such as official government reports or cryptographic feeds) used to certify the final winning outcome of a market.',
            plainEnglish: 'The official referee or source of truth that announces whether YES or NO won when the market ends.'
        }
    ];

    open(): void {
        this.isOpen.set(true);
    }

    close(): void {
        this.isOpen.set(false);
    }
}
