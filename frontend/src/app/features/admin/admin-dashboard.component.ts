import { ChangeDetectionStrategy, Component, computed, inject, OnInit, signal, WritableSignal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { LucideCheck, LucideShieldCheck, LucidePlus, LucideArrowRight, LucideInfo, LucideArrowUp, LucideArrowDown, LucideCheckCircle2 } from '@lucide/angular';
import { ApiService } from '../../core/services/api.service';
import { ToastService } from '../../shared/components/toast/toast.service';
import { Market, CreateMarketRequest, CreateMarketResponse } from '../../core/models/market.model';
import { ButtonComponent } from '../../shared/components/button/button.component';
import { BadgeComponent } from '../../shared/components/badge/badge.component';
import { InputComponent } from '../../shared/components/input/input.component';
import { LabelComponent } from '../../shared/components/label/label.component';
import { SelectComponent, SelectOption } from '../../shared/components/select/select.component';
import { TextareaComponent } from '../../shared/components/textarea/textarea.component';
import { BrandIconComponent } from '../../shared/components/brand-icon/brand-icon.component';

const ADMIN_TOKEN_KEY = 'bayesmarket_admin_token';

@Component({
    selector: 'app-admin-dashboard',
    standalone: true,
    changeDetection: ChangeDetectionStrategy.OnPush,
    imports: [
        CommonModule,
        FormsModule,
        RouterLink,
        ButtonComponent,
        BadgeComponent,
        BrandIconComponent,
        InputComponent,
        LabelComponent,
        SelectComponent,
        TextareaComponent,
        LucideCheck,
        LucideShieldCheck,
        LucidePlus,
        LucideArrowRight,
        LucideInfo,
        LucideArrowUp,
        LucideArrowDown,
        LucideCheckCircle2
    ],
    template: `
        <div class="admin-container" role="main">
            <!-- Header Banner -->
            <div class="admin-header">
                <div class="header-left">
                    <div class="admin-badge-row">
                        <app-badge variant="outline" size="sm">
                            <svg lucideShieldCheck class="badge-icon" [size]="14" aria-hidden="true"></svg>
                            SUPERADMIN CONSOLE
                        </app-badge>
                        <span class="env-tag">RESTRICTED ENVIRONMENT</span>
                    </div>
                    <div class="admin-title-row">
                        <app-brand-icon [size]="32" [glow]="true"></app-brand-icon>
                        <h1 class="admin-title">Prediction Market Operations</h1>
                    </div>
                    <p class="admin-subtitle">
                        Provision algorithmic binary prediction markets with complete-set CPMM liquidity, or settle mature contracts via authoritative oracle
                        proof.
                    </p>
                </div>

                @if (isUnlocked()) {
                    <div class="header-right">
                        <app-button variant="secondary" size="default" (btnClick)="lockDashboard()" ariaLabel="Lock admin console session">
                            Lock Console
                        </app-button>
                    </div>
                }
            </div>

            <!-- Passkey Gatekeeper (Shown when not authenticated) -->
            @if (!isUnlocked()) {
                <div class="gatekeeper-card" role="region" aria-label="Admin Authentication Gatekeeper">
                    <div class="gatekeeper-body">
                        <div class="gate-icon-circle">
                            <svg lucideShieldCheck class="gate-icon" [size]="32" aria-hidden="true"></svg>
                        </div>
                        <h2 class="gate-title">Enter Admin Authorization Key</h2>
                        <p class="gate-desc">Administrative operations require a valid server <code class="code-pill">ADMIN_TOKEN</code>.</p>

                        <div class="gate-form">
                            <app-label htmlFor="admin-token-input">ADMIN_TOKEN</app-label>
                            <app-input
                                id="admin-token-input"
                                type="password"
                                variant="mono"
                                size="lg"
                                [value]="tokenInput()"
                                (valueChange)="onValueChange(tokenInput, $event)"
                                placeholder="Enter admin passkey..."
                                ariaLabel="Admin authorization token"
                            />

                            @if (isDev()) {
                                <div class="dev-hint-row">
                                    <span class="hint-text">Local Dev: Enter ADMIN_TOKEN configured in your server .env</span>
                                </div>
                            }

                            <app-button
                                variant="primary"
                                size="lg"
                                [fullWidth]="true"
                                [loading]="isVerifying()"
                                (btnClick)="verifyAndUnlock()"
                                ariaLabel="Authenticate and unlock admin console"
                            >
                                Unlock Admin Console
                            </app-button>
                        </div>
                    </div>
                </div>
            } @else {
                <!-- Unlocked Workspace Tabs -->
                <div class="workspace-tabs" role="tablist" aria-label="Admin Operations">
                    <button
                        type="button"
                        role="tab"
                        class="tab-btn"
                        [class.active]="activeTab() === 'create'"
                        [attr.aria-selected]="activeTab() === 'create'"
                        (click)="activeTab.set('create')"
                    >
                        <svg lucidePlus [size]="16" aria-hidden="true"></svg>
                        <span>Create Prediction Market</span>
                    </button>

                    <button
                        type="button"
                        role="tab"
                        class="tab-btn"
                        [class.active]="activeTab() === 'resolve'"
                        [attr.aria-selected]="activeTab() === 'resolve'"
                        (click)="activeTab.set('resolve')"
                    >
                        <svg lucideCheckCircle2 [size]="16" aria-hidden="true"></svg>
                        <span>Resolve Active Market</span>
                    </button>
                </div>

                <!-- Tab 1: Create Market View -->
                @if (activeTab() === 'create') {
                    <div class="tab-pane create-market-grid">
                        <!-- Left Form Column -->
                        <div class="form-card">
                            <h2 class="card-title">Market Parameters</h2>
                            <p class="card-desc">Define the event question, resolution criteria, and initial liquidity parameters.</p>

                            <form class="market-form" (submit)="onSubmitCreateMarket($event)">
                                <!-- Title -->
                                <div class="form-group">
                                    <app-label htmlFor="m-title" size="default">Market Question / Title *</app-label>
                                    <app-input
                                        id="m-title"
                                        size="default"
                                        [value]="title()"
                                        (valueChange)="onValueChange(title, $event)"
                                        placeholder="e.g. Will Ethereum break all-time high before Dec 31, 2026?"
                                        ariaLabel="Market Title"
                                    />
                                </div>

                                <!-- Category & Resolution Date -->
                                <div class="form-row-2">
                                    <div class="form-group">
                                        <app-label htmlFor="m-category" size="default">Category *</app-label>
                                        <app-select
                                            id="m-category"
                                            [options]="categoryOptions"
                                            [value]="category()"
                                            (valueChange)="category.set($event)"
                                            placeholder="Select Category"
                                            ariaLabel="Market Category"
                                        />
                                    </div>

                                    <div class="form-group">
                                        <app-label htmlFor="m-date" size="default">Resolution Date (UTC) *</app-label>
                                        <app-input
                                            id="m-date"
                                            type="datetime-local"
                                            size="default"
                                            variant="mono"
                                            [value]="resolutionDateInput()"
                                            (valueChange)="onValueChange(resolutionDateInput, $event)"
                                            ariaLabel="Resolution Date"
                                        />
                                    </div>
                                </div>

                                <!-- Description -->
                                <div class="form-group">
                                    <app-label htmlFor="m-desc" size="default">Detailed Resolution Criteria & Rules *</app-label>
                                    <app-textarea
                                        id="m-desc"
                                        [rows]="4"
                                        [value]="description()"
                                        (valueChange)="description.set($event)"
                                        placeholder="State the objective condition for YES versus NO outcome..."
                                        ariaLabel="Market Description"
                                    />
                                </div>

                                <!-- Resolution Source -->
                                <div class="form-group">
                                    <app-label htmlFor="m-source" size="default">Authoritative Resolution Source *</app-label>
                                    <app-input
                                        id="m-source"
                                        size="default"
                                        [value]="resolutionSource()"
                                        (valueChange)="onValueChange(resolutionSource, $event)"
                                        placeholder="e.g. Official NASA telemetry, SEC filing, or Coinbase index..."
                                        ariaLabel="Resolution Source"
                                    />
                                </div>

                                <!-- Image URL -->
                                <div class="form-group">
                                    <app-label htmlFor="m-image" size="default">Image URL (Optional)</app-label>
                                    <app-input
                                        id="m-image"
                                        size="default"
                                        [value]="imageUrl()"
                                        (valueChange)="onValueChange(imageUrl, $event)"
                                        placeholder="Leave empty for category default webp asset"
                                        ariaLabel="Market Image URL"
                                    />
                                </div>

                                <!-- Collateral & Probability -->
                                <div class="form-row-2">
                                    <div class="form-group">
                                        <app-label htmlFor="m-collateral" size="default">Initial Pool Collateral (USDC)</app-label>
                                        <app-input
                                            id="m-collateral"
                                            type="number"
                                            variant="mono"
                                            size="default"
                                            [value]="collateralInput()"
                                            (valueChange)="onValueChange(collateralInput, $event)"
                                            placeholder="10000"
                                            ariaLabel="Initial Collateral"
                                        />
                                    </div>

                                    <div class="form-group">
                                        <app-label htmlFor="m-prob" size="default">Starting Implied Probability (YES %)</app-label>
                                        <app-input
                                            id="m-prob"
                                            type="number"
                                            variant="mono"
                                            size="default"
                                            [value]="probabilityInput()"
                                            (valueChange)="onValueChange(probabilityInput, $event)"
                                            placeholder="50"
                                            ariaLabel="Starting Probability"
                                        />
                                    </div>
                                </div>

                                <div class="form-actions">
                                    <app-button
                                        variant="primary"
                                        size="lg"
                                        [fullWidth]="true"
                                        [loading]="isSubmitting()"
                                        ariaLabel="Submit new market creation"
                                    >
                                        Create Market & Initialize CPMM Pool
                                    </app-button>
                                </div>
                            </form>
                        </div>

                        <!-- Right Column: Live CPMM Invariant Preview -->
                        <div class="preview-column">
                            <div class="preview-card">
                                <h3 class="preview-title">CPMM Bonding Preview</h3>
                                <p class="preview-subtitle">Derived fixed-point pool reserves ($k = R_&#123;YES&#125; imes R_&#123;NO&#125;$)</p>

                                <div class="prob-visual-track" role="progressbar" aria-label="Implied initial probability split">
                                    <div class="track-yes" [style.width.%]="numericProbability()"></div>
                                    <div class="track-no" [style.width.%]="100 - numericProbability()"></div>
                                </div>

                                <div class="prob-split-labels">
                                    <span class="label-yes tabular-nums">▲ YES: {{ numericProbability() }}¢ ({{ numericProbability() }}%)</span>
                                    <span class="label-no tabular-nums">▼ NO: {{ 100 - numericProbability() }}¢ ({{ 100 - numericProbability() }}%)</span>
                                </div>

                                <div class="preview-stats-grid">
                                    <div class="stat-box">
                                        <span class="stat-k">Collateral Reserve</span>
                                        <span class="stat-v tabular-nums">&#36;{{ formattedCollateral() }} USDC</span>
                                    </div>
                                    <div class="stat-box">
                                        <span class="stat-k">Virtual YES Reserve (R_yes)</span>
                                        <span class="stat-v tabular-nums">{{ formattedReserveYes() }}</span>
                                    </div>
                                    <div class="stat-box">
                                        <span class="stat-k">Virtual NO Reserve (R_no)</span>
                                        <span class="stat-v tabular-nums">{{ formattedReserveNo() }}</span>
                                    </div>
                                    <div class="stat-box">
                                        <span class="stat-k">Constant Product (k)</span>
                                        <span class="stat-v tabular-nums">{{ formattedInvariantK() }}</span>
                                    </div>
                                </div>

                                <div class="info-alert">
                                    <svg lucideInfo class="info-icon" [size]="16" aria-hidden="true"></svg>
                                    <span>
                                        Every issued share remains 100% redeemable for 1.00 USDC collateral. Spot prices clear exactly to the configured
                                        probability.
                                    </span>
                                </div>
                            </div>

                            @if (createdMarket(); as cm) {
                                <div class="success-banner" role="status">
                                    <div class="success-header">
                                        <svg lucideCheckCircle2 class="success-icon" [size]="20" aria-hidden="true"></svg>
                                        <span class="success-title">Market Successfully Created!</span>
                                    </div>
                                    <p class="success-text">"{{ cm.title }}" is now live on the trading engine.</p>
                                    <div class="success-actions">
                                        <a [routerLink]="'/markets/' + cm.slug" class="view-market-link">
                                            <span>View Market Page</span>
                                            <svg lucideArrowRight [size]="14" aria-hidden="true"></svg>
                                        </a>
                                    </div>
                                </div>
                            }
                        </div>
                    </div>
                }

                <!-- Tab 2: Resolve Active Market View -->
                @if (activeTab() === 'resolve') {
                    <div class="tab-pane resolve-market-pane">
                        <div class="form-card resolve-card">
                            <h2 class="card-title">Settle & Distribute Collateral</h2>
                            <p class="card-desc">
                                Select a mature prediction market, designate the authoritative verified outcome, and trigger atomic payout settlement.
                            </p>

                            <div class="market-form">
                                <div class="form-group">
                                    <app-label htmlFor="resolve-market-select">Select Active Market *</app-label>
                                    <app-select
                                        id="resolve-market-select"
                                        [options]="marketOptions()"
                                        [value]="selectedMarketId()"
                                        (valueChange)="selectedMarketId.set($event)"
                                        placeholder="Choose market to resolve..."
                                        ariaLabel="Active market selector"
                                    />
                                </div>

                                <!-- Winning Outcome Toggle -->
                                <div class="form-group">
                                    <app-label>Authoritative Winning Outcome *</app-label>
                                    <div class="outcome-select-row" role="radiogroup" aria-label="Winning outcome">
                                        <app-button
                                            variant="yes"
                                            size="lg"
                                            [selected]="winningOutcome() === 'YES'"
                                            role="radio"
                                            ariaLabel="Resolve to outcome YES"
                                            (btnClick)="winningOutcome.set('YES')"
                                        >
                                            <svg lucideArrowUp class="outcome-glyph" [size]="16" aria-hidden="true"></svg>
                                            <span>OUTCOME YES (1.00 USDC)</span>
                                        </app-button>

                                        <app-button
                                            variant="no"
                                            size="lg"
                                            [selected]="winningOutcome() === 'NO'"
                                            role="radio"
                                            ariaLabel="Resolve to outcome NO"
                                            (btnClick)="winningOutcome.set('NO')"
                                        >
                                            <svg lucideArrowDown class="outcome-glyph" [size]="16" aria-hidden="true"></svg>
                                            <span>OUTCOME NO (1.00 USDC)</span>
                                        </app-button>
                                    </div>
                                </div>

                                <!-- Oracle Proof -->
                                <div class="form-group">
                                    <app-label htmlFor="oracle-proof-input">Oracle Proof Verification & Evidence URL *</app-label>
                                    <app-textarea
                                        id="oracle-proof-input"
                                        [rows]="3"
                                        [value]="oracleProof()"
                                        (valueChange)="oracleProof.set($event)"
                                        placeholder="Record the official proof reference or verification URL (e.g. FOMC meeting announcement link)..."
                                        ariaLabel="Oracle proof input"
                                    />
                                </div>

                                <div class="form-actions">
                                    <app-button
                                        variant="primary"
                                        size="lg"
                                        [fullWidth]="true"
                                        [loading]="isResolving()"
                                        [disabled]="!selectedMarketId() || !oracleProof()"
                                        (btnClick)="onSubmitResolveMarket()"
                                        ariaLabel="Execute market resolution and payouts"
                                    >
                                        Execute Resolution & Credit Winners
                                    </app-button>
                                </div>
                            </div>

                            @if (resolutionSummary(); as res) {
                                <div class="resolution-summary-box" role="status">
                                    <div class="res-summary-title">
                                        <svg lucideCheck class="res-check-icon" [size]="18" aria-hidden="true"></svg>
                                        <span>Settlement Completed</span>
                                    </div>
                                    <div class="res-stats-grid">
                                        <div class="res-stat">
                                            <span class="stat-k">Winning Outcome:</span>
                                            <span class="stat-v tabular-nums">{{ res.winning_outcome }}</span>
                                        </div>
                                        <div class="res-stat">
                                            <span class="stat-k">Traders Credited:</span>
                                            <span class="stat-v tabular-nums">{{ res.winners_credited }}</span>
                                        </div>
                                        <div class="res-stat">
                                            <span class="stat-k">Total Payout:</span>
                                            <span class="stat-v tabular-nums">&#36;{{ res.total_payout_usdc }} USDC</span>
                                        </div>
                                    </div>
                                </div>
                            }
                        </div>
                    </div>
                }
            }
        </div>
    `,
    styles: [
        `
            :host {
                display: block;
                min-height: calc(100vh - 60px);
                background-color: var(--canvas, #07090e);
                color: var(--ink, #f8f7ff);
                padding: var(--space-xl, 32px) var(--space-lg, 20px) 80px;
            }

            .admin-container {
                max-width: 1200px;
                margin: 0 auto;
                display: flex;
                flex-direction: column;
                gap: 28px;
            }

            .admin-header {
                display: flex;
                align-items: flex-start;
                justify-content: space-between;
                gap: 16px;
                border-bottom: 1px solid var(--hairline, #1e2638);
                padding-bottom: 24px;
            }

            .admin-badge-row {
                display: flex;
                align-items: center;
                gap: 10px;
                margin-bottom: 10px;
            }

            .badge-icon {
                margin-right: 4px;
                color: var(--primary-border, #a855f7);
            }

            .env-tag {
                font-family: var(--font-mono);
                font-size: 11px;
                font-weight: 700;
                color: var(--status-warning, #f59e0b);
                background-color: rgba(245, 158, 11, 0.1);
                border: 1px solid rgba(245, 158, 11, 0.25);
                padding: 2px 8px;
                border-radius: var(--radius-sm, 4px);
                letter-spacing: 0.5px;
            }

            .admin-title-row {
                display: flex;
                align-items: center;
                gap: 12px;
                margin-bottom: 8px;
            }

            .admin-title {
                font-family: var(--font-ui);
                font-size: 28px;
                font-weight: 800;
                color: var(--ink, #f8f7ff);
                margin: 0;
                letter-spacing: -0.5px;
            }

            .admin-subtitle {
                font-family: var(--font-ui);
                font-size: 14px;
                color: var(--muted, #9d97b8);
                max-width: 720px;
                line-height: 1.5;
                margin: 0;
            }

            /* Gatekeeper Card */
            .gatekeeper-card {
                max-width: 480px;
                margin: 40px auto;
                background-color: var(--surface-card, #111622);
                border: 1px solid var(--hairline, #1e2638);
                border-radius: var(--radius-lg, 14px);
                box-shadow: 0 12px 40px rgba(0, 0, 0, 0.5);
                overflow: hidden;
            }

            .gatekeeper-body {
                padding: 32px 28px;
                display: flex;
                flex-direction: column;
                align-items: center;
                text-align: center;
                gap: 16px;
            }

            .gate-icon-circle {
                width: 64px;
                height: 64px;
                border-radius: 50%;
                background-color: rgba(168, 85, 247, 0.12);
                border: 1px solid rgba(168, 85, 247, 0.3);
                display: flex;
                align-items: center;
                justify-content: center;
            }

            .gate-icon {
                color: var(--primary-border, #a855f7);
            }

            .gate-title {
                font-family: var(--font-ui);
                font-size: 20px;
                font-weight: 700;
                color: var(--ink, #f8f7ff);
                margin: 0;
            }

            .gate-desc {
                font-family: var(--font-ui);
                font-size: 13px;
                color: var(--muted, #9d97b8);
                margin: 0;
                line-height: 1.4;
            }

            .code-pill {
                font-family: var(--font-mono);
                font-size: 12px;
                color: #a855f7;
                background-color: rgba(168, 85, 247, 0.15);
                padding: 2px 6px;
                border-radius: 4px;
            }

            .gate-form {
                width: 100%;
                display: flex;
                flex-direction: column;
                gap: 14px;
                margin-top: 8px;
                text-align: left;
            }

            .dev-hint-row {
                display: flex;
                align-items: center;
                justify-content: space-between;
                font-size: 12px;
            }

            .hint-text {
                color: var(--muted, #9d97b8);
            }

            .hint-fill-btn {
                background: none;
                border: none;
                color: var(--primary-border, #a855f7);
                cursor: pointer;
                font-family: var(--font-ui);
                font-weight: 600;
                padding: 0;
                text-decoration: underline;
            }

            /* Workspace Tabs */
            .workspace-tabs {
                display: flex;
                align-items: center;
                gap: 8px;
                border-bottom: 1px solid var(--hairline, #1e2638);
                padding-bottom: 8px;
            }

            .tab-btn {
                display: inline-flex;
                align-items: center;
                gap: 8px;
                min-height: 44px;
                padding: 8px 18px;
                border-radius: var(--radius-md, 10px);
                background-color: transparent;
                border: 1px solid transparent;
                color: var(--muted, #9d97b8);
                font-family: var(--font-ui);
                font-size: 14px;
                font-weight: 600;
                cursor: pointer;
                transition: all 0.15s ease;
            }

            .tab-btn:hover {
                background-color: var(--surface-card, #111622);
                color: var(--ink, #f8f7ff);
            }

            .tab-btn.active {
                background-color: var(--surface-card-elevated, #171f30);
                color: var(--ink, #f8f7ff);
                border-color: var(--hairline, #1e2638);
                font-weight: 700;
            }

            /* Create Market Layout */
            .create-market-grid {
                display: grid;
                grid-template-columns: 3fr 2fr;
                gap: 24px;
            }

            @media (max-width: 900px) {
                .create-market-grid {
                    grid-template-columns: 1fr;
                }
            }

            .form-card {
                background-color: var(--surface-card, #111622);
                border: 1px solid var(--hairline, #1e2638);
                border-radius: var(--radius-lg, 14px);
                padding: 24px;
                display: flex;
                flex-direction: column;
                gap: 16px;
            }

            .card-title {
                font-family: var(--font-ui);
                font-size: 18px;
                font-weight: 700;
                color: var(--ink, #f8f7ff);
                margin: 0;
            }

            .card-desc {
                font-family: var(--font-ui);
                font-size: 13px;
                color: var(--muted, #9d97b8);
                margin: 0;
            }

            .market-form {
                display: flex;
                flex-direction: column;
                gap: 16px;
            }

            .form-group {
                display: flex;
                flex-direction: column;
                gap: 6px;
            }

            .form-row-2 {
                display: grid;
                grid-template-columns: 1fr 1fr;
                gap: 16px;
            }

            @media (max-width: 600px) {
                .form-row-2 {
                    grid-template-columns: 1fr;
                }
            }

            .form-actions {
                margin-top: 10px;
            }

            /* Preview Column */
            .preview-column {
                display: flex;
                flex-direction: column;
                gap: 20px;
            }

            .preview-card {
                background-color: var(--surface-card, #111622);
                border: 1px solid var(--hairline, #1e2638);
                border-radius: var(--radius-lg, 14px);
                padding: 20px;
                display: flex;
                flex-direction: column;
                gap: 16px;
            }

            .preview-title {
                font-family: var(--font-ui);
                font-size: 16px;
                font-weight: 700;
                color: var(--ink, #f8f7ff);
                margin: 0;
            }

            .preview-subtitle {
                font-family: var(--font-ui);
                font-size: 12px;
                color: var(--muted, #9d97b8);
                margin: 0;
            }

            .prob-visual-track {
                width: 100%;
                height: 10px;
                border-radius: var(--radius-pill, 9999px);
                display: flex;
                overflow: hidden;
                background-color: #0c1017;
            }

            .track-yes {
                background-color: var(--outcome-yes, #10b981);
                transition: width 0.2s ease;
            }

            .track-no {
                background-color: var(--outcome-no, #fb7185);
                transition: width 0.2s ease;
            }

            .prob-split-labels {
                display: flex;
                align-items: center;
                justify-content: space-between;
                font-family: var(--font-mono);
                font-size: 12px;
                font-weight: 700;
            }

            .label-yes {
                color: var(--outcome-yes, #10b981);
            }

            .label-no {
                color: var(--outcome-no, #fb7185);
            }

            .preview-stats-grid {
                display: grid;
                grid-template-columns: 1fr 1fr;
                gap: 10px;
                border-top: 1px solid var(--hairline, #1e2638);
                padding-top: 14px;
            }

            .stat-box {
                display: flex;
                flex-direction: column;
                gap: 2px;
                background-color: var(--canvas-subtle, #0c1017);
                border: 1px solid var(--hairline, #1e2638);
                border-radius: var(--radius-md, 10px);
                padding: 10px 12px;
            }

            .stat-k {
                font-family: var(--font-ui);
                font-size: 11px;
                color: var(--muted, #9d97b8);
            }

            .stat-v {
                font-family: var(--font-mono);
                font-size: 13px;
                font-weight: 700;
                color: var(--ink, #f8f7ff);
                font-feature-settings: 'tnum' 1;
            }

            .info-alert {
                display: flex;
                align-items: flex-start;
                gap: 8px;
                padding: 10px 12px;
                background-color: rgba(124, 77, 255, 0.08);
                border: 1px solid rgba(124, 77, 255, 0.25);
                border-radius: var(--radius-md, 10px);
                font-family: var(--font-ui);
                font-size: 12px;
                color: var(--muted, #9d97b8);
                line-height: 1.4;
            }

            .info-icon {
                color: #a855f7;
                flex-shrink: 0;
                margin-top: 1px;
            }

            .success-banner {
                background-color: rgba(16, 185, 129, 0.1);
                border: 1px solid rgba(16, 185, 129, 0.35);
                border-radius: var(--radius-lg, 14px);
                padding: 16px 20px;
                display: flex;
                flex-direction: column;
                gap: 10px;
            }

            .success-header {
                display: flex;
                align-items: center;
                gap: 8px;
            }

            .success-icon {
                color: var(--outcome-yes, #10b981);
            }

            .success-title {
                font-family: var(--font-ui);
                font-size: 14px;
                font-weight: 700;
                color: var(--outcome-yes, #10b981);
            }

            .success-text {
                font-family: var(--font-ui);
                font-size: 13px;
                color: var(--ink, #f8f7ff);
                margin: 0;
            }

            .view-market-link {
                display: inline-flex;
                align-items: center;
                gap: 6px;
                color: var(--accent, #00d4ff);
                font-family: var(--font-ui);
                font-size: 13px;
                font-weight: 600;
                text-decoration: none;
            }

            .view-market-link:hover {
                text-decoration: underline;
            }

            /* Resolve Market Pane */
            .resolve-card {
                max-width: 640px;
            }

            .outcome-select-row {
                display: grid;
                grid-template-columns: 1fr 1fr;
                gap: 12px;
            }

            .outcome-glyph {
                margin-right: 4px;
            }

            .resolution-summary-box {
                margin-top: 16px;
                padding: 16px;
                background-color: rgba(16, 185, 129, 0.08);
                border: 1px solid rgba(16, 185, 129, 0.3);
                border-radius: var(--radius-md, 10px);
                display: flex;
                flex-direction: column;
                gap: 12px;
            }

            .res-summary-title {
                display: flex;
                align-items: center;
                gap: 8px;
                font-family: var(--font-ui);
                font-size: 14px;
                font-weight: 700;
                color: var(--outcome-yes, #10b981);
            }

            .res-stats-grid {
                display: grid;
                grid-template-columns: 1fr 1fr 1fr;
                gap: 10px;
            }

            .res-stat {
                display: flex;
                flex-direction: column;
                gap: 2px;
            }
        `
    ]
})
export class AdminDashboardComponent implements OnInit {
    private readonly apiService = inject(ApiService);
    private readonly toastService = inject(ToastService);

    readonly isUnlocked = signal<boolean>(false);
    readonly isVerifying = signal<boolean>(false);
    readonly isSubmitting = signal<boolean>(false);
    readonly isResolving = signal<boolean>(false);
    readonly isDev = signal<boolean>(typeof window !== 'undefined' && (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'));

    readonly tokenInput = signal<string>('');
    readonly adminToken = signal<string>('');
    readonly activeTab = signal<'create' | 'resolve'>('create');

    // Create Market Form fields
    readonly title = signal<string>('');
    readonly category = signal<string>('ai');
    readonly description = signal<string>('');
    readonly resolutionSource = signal<string>('');
    readonly resolutionDateInput = signal<string>('');
    readonly imageUrl = signal<string>('');
    readonly collateralInput = signal<string>('10000');
    readonly probabilityInput = signal<string>('50');

    readonly createdMarket = signal<CreateMarketResponse | null>(null);

    // Resolve Market Form fields
    readonly activeMarkets = signal<Market[]>([]);
    readonly selectedMarketId = signal<string>('');
    readonly winningOutcome = signal<'YES' | 'NO'>('YES');
    readonly oracleProof = signal<string>('');
    readonly resolutionSummary = signal<{
        status: string;
        winning_outcome: string;
        winners_credited: number;
        total_payout_usdc: string;
    } | null>(null);

    onValueChange(targetSignal: WritableSignal<string>, val: string | number): void {
        targetSignal.set(String(val ?? ''));
    }

    readonly categoryOptions: SelectOption[] = [
        { value: 'ai', label: 'AI & Tech' },
        { value: 'crypto', label: 'Crypto' },
        { value: 'macro', label: 'Macro Economy' },
        { value: 'science', label: 'Science & Space' }
    ];

    readonly marketOptions = computed<SelectOption[]>(() => {
        return this.activeMarkets()
            .filter((m) => m.status === 'active')
            .map((m) => ({
                value: m.id,
                label: m.title
            }));
    });

    readonly numericProbability = computed(() => {
        const parsed = parseFloat(this.probabilityInput());
        if (isNaN(parsed) || parsed < 1) return 1;
        if (parsed > 99) return 99;
        return parsed;
    });

    readonly numericCollateral = computed(() => {
        const parsed = parseFloat(this.collateralInput());
        return isNaN(parsed) || parsed <= 0 ? 10000 : parsed;
    });

    readonly formattedCollateral = computed(() => {
        return this.numericCollateral().toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
    });

    readonly formattedReserveYes = computed(() => {
        const c = this.numericCollateral();
        const pNo = (100 - this.numericProbability()) / 100;
        return (c * pNo).toFixed(2);
    });

    readonly formattedReserveNo = computed(() => {
        const c = this.numericCollateral();
        const pYes = this.numericProbability() / 100;
        return (c * pYes).toFixed(2);
    });

    readonly formattedInvariantK = computed(() => {
        const rYes = parseFloat(this.formattedReserveYes());
        const rNo = parseFloat(this.formattedReserveNo());
        return (rYes * rNo).toLocaleString('en-US', { maximumFractionDigits: 0 });
    });

    ngOnInit(): void {
        this.initDefaultDate();
        this.apiService.getConfig().subscribe({
            next: (cfg) => {
                if (cfg) {
                    this.isDev.set(cfg.is_dev);
                }
            }
        });
        if (typeof window !== 'undefined' && window.sessionStorage) {
            const saved = sessionStorage.getItem(ADMIN_TOKEN_KEY);
            if (saved) {
                this.adminToken.set(saved);
                this.tokenInput.set(saved);
                this.verifyAndUnlock(saved);
            }
        }
    }

    private initDefaultDate(): void {
        const future = new Date();
        future.setMonth(future.getMonth() + 3);
        const y = future.getFullYear();
        const m = String(future.getMonth() + 1).padStart(2, '0');
        const d = String(future.getDate()).padStart(2, '0');
        this.resolutionDateInput.set(`${y}-${m}-${d}T23:59`);
    }

    verifyAndUnlock(tokenOverride?: string): void {
        const token = (tokenOverride || this.tokenInput()).trim();
        if (!token) {
            this.toastService.warning('Required', 'Please enter the ADMIN_TOKEN');
            return;
        }

        this.isVerifying.set(true);
        this.apiService.verifyAdmin(token).subscribe({
            next: () => {
                this.adminToken.set(token);
                this.isUnlocked.set(true);
                this.isVerifying.set(false);
                if (typeof window !== 'undefined' && window.sessionStorage) {
                    sessionStorage.setItem(ADMIN_TOKEN_KEY, token);
                }
                this.toastService.success('Admin Console Unlocked', 'Operational management active');
                this.loadActiveMarkets();
            },
            error: () => {
                this.isVerifying.set(false);
                this.toastService.error('Authentication Failed', 'Invalid ADMIN_TOKEN credential');
            }
        });
    }

    lockDashboard(): void {
        this.isUnlocked.set(false);
        this.adminToken.set('');
        if (typeof window !== 'undefined' && window.sessionStorage) {
            sessionStorage.removeItem(ADMIN_TOKEN_KEY);
        }
        this.toastService.info('Console Locked', 'Admin session cleared');
    }

    loadActiveMarkets(): void {
        this.apiService.getMarkets().subscribe({
            next: (markets) => {
                this.activeMarkets.set(markets);
            },
            error: (err) => console.warn('Failed to load active markets:', err)
        });
    }

    onSubmitCreateMarket(e: Event): void {
        e.preventDefault();

        const title = this.title().trim();
        const desc = this.description().trim();
        const source = this.resolutionSource().trim();
        const rawDate = this.resolutionDateInput().trim();

        if (!title || !desc || !source || !rawDate) {
            this.toastService.warning('Validation Error', 'Please complete all required fields');
            return;
        }

        const payload: CreateMarketRequest = {
            title,
            description: desc,
            category: this.category(),
            resolution_source: source,
            resolution_date: new Date(rawDate).toISOString(),
            image_url: this.imageUrl().trim(),
            initial_collateral_usdc: this.numericCollateral().toFixed(8),
            initial_probability_yes: (this.numericProbability() / 100).toFixed(8)
        };

        this.isSubmitting.set(true);
        this.apiService.createMarket(payload, this.adminToken()).subscribe({
            next: (res) => {
                this.isSubmitting.set(false);
                this.createdMarket.set(res);
                this.toastService.success('Market Created', `"${res.title}" successfully provisioned!`);
                // Clear form for next entry
                this.title.set('');
                this.description.set('');
                this.resolutionSource.set('');
                this.loadActiveMarkets();
            },
            error: (err) => {
                this.isSubmitting.set(false);
                const msg = err?.error?.message || 'Failed to create market';
                this.toastService.error('Creation Error', msg);
            }
        });
    }

    onSubmitResolveMarket(): void {
        const marketId = this.selectedMarketId();
        const proof = this.oracleProof().trim();
        const outcome = this.winningOutcome();

        if (!marketId || !proof) {
            this.toastService.warning('Validation Error', 'Select a market and provide oracle proof');
            return;
        }

        this.isResolving.set(true);
        const idempotencyKey = `resolve-${marketId}-${Date.now()}`;
        this.apiService.resolveMarket(marketId, { winning_outcome: outcome, oracle_proof: proof }, this.adminToken(), idempotencyKey).subscribe({
            next: (res) => {
                this.isResolving.set(false);
                this.resolutionSummary.set(res);
                this.toastService.success('Market Resolved', `Outcome ${res.winning_outcome} settled. Credited ${res.winners_credited} traders.`);
                this.loadActiveMarkets();
            },
            error: (err) => {
                this.isResolving.set(false);
                const msg = err?.error?.message || 'Failed to resolve market';
                this.toastService.error('Resolution Error', msg);
            }
        });
    }
}
