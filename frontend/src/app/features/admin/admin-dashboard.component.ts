import { ChangeDetectionStrategy, Component, computed, effect, inject, OnInit, signal, WritableSignal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import {
    LucideCheck,
    LucideShieldCheck,
    LucidePlus,
    LucideArrowRight,
    LucideInfo,
    LucideArrowUp,
    LucideArrowDown,
    LucideCheckCircle2,
    LucidePencil,
    LucideTrash2,
    LucideSliders,
    LucideX
} from '@lucide/angular';
import { ApiService } from '../../core/services/api.service';
import { AuthStore } from '../../state/auth.store';
import { ToastService } from '../../shared/components/toast/toast.service';
import { Market, CreateMarketRequest, CreateMarketResponse, EditMarketRequest, ResolveMarketResponse } from '../../core/models/market.model';
import { DialogComponent } from '../../shared/components/dialog/dialog.component';
import { ButtonComponent } from '../../shared/components/button/button.component';
import { BadgeComponent, BadgeVariant } from '../../shared/components/badge/badge.component';
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
        DialogComponent,
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
        LucideCheckCircle2,
        LucidePencil,
        LucideTrash2,
        LucideSliders
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
                        Administrative command center for prediction market lifecycle: create algorithmic binary contracts, edit live metadata, execute
                        permanent deletions, or settle mature markets.
                    </p>
                </div>

                @if (isUnlocked()) {
                    <div class="header-right">
                        @if (authStore.isAdmin()) {
                            <div class="admin-identity-pill">
                                <span class="identity-dot"></span>
                                <span class="identity-role">Neon DB Admin</span>
                                <span class="identity-email">{{ authStore.user()?.email }}</span>
                            </div>
                        }
                        <app-button variant="secondary" size="default" (btnClick)="lockDashboard()" ariaLabel="Lock admin console session">
                            Lock Console
                        </app-button>
                    </div>
                }
            </div>

            <!-- Passkey Gatekeeper (Shown when not authenticated as admin) -->
            @if (!isUnlocked()) {
                <div class="gatekeeper-card" role="region" aria-label="Admin Authentication Gatekeeper">
                    <div class="gatekeeper-body">
                        <div class="gate-icon-circle">
                            <svg lucideShieldCheck class="gate-icon" [size]="32" aria-hidden="true"></svg>
                        </div>
                        <h2 class="gate-title">Admin Authorization Required</h2>

                        <!-- Neon DB Admin Invariant Notice -->
                        @if (authStore.isAuthenticated() && !authStore.isAdmin()) {
                            <div class="db-admin-notice warning-box">
                                <p class="notice-title">Admin Rights Invariant</p>
                                <p class="notice-desc">
                                    Logged in as <strong class="user-highlight">{{ authStore.user()?.email }}</strong
                                    >, but this account is not an admin.
                                </p>
                                <p class="notice-sql-label">Setting an admin can only be done directly from Neon DB:</p>
                                <pre class="sql-code-block"><code>UPDATE users SET is_admin = true WHERE email = '{{ authStore.user()?.email }}';</code></pre>
                                <p class="notice-hint">Execute this query directly in the Neon SQL console, then refresh or re-login.</p>
                            </div>
                        } @else if (!authStore.isAuthenticated()) {
                            <div class="db-admin-notice info-box">
                                <p class="notice-title">Strict Database-Level Admin Assignment</p>
                                <p class="notice-desc">
                                    Markets can only be created, edited, and deleted by an admin. Admin permissions cannot be assigned through the web UI and
                                    must be granted directly in Neon DB:
                                </p>
                                <pre class="sql-code-block"><code>UPDATE users SET is_admin = true WHERE email = 'YOUR_EMAIL';</code></pre>
                            </div>
                        }

                        <div class="gate-form">
                            <app-label htmlFor="admin-token-input">Or Enter ADMIN_TOKEN Passkey</app-label>
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
                                    <span class="hint-text">Local Dev: Enter ADMIN_TOKEN configured in backend .env</span>
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
                        [class.active]="activeTab() === 'manage'"
                        [attr.aria-selected]="activeTab() === 'manage'"
                        (click)="activeTab.set('manage')"
                    >
                        <svg lucideSliders [size]="16" aria-hidden="true"></svg>
                        <span>Manage Markets ({{ activeMarkets().length }})</span>
                    </button>

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

                <!-- Tab 0: Manage Existing Markets (Edit / Delete) -->
                @if (activeTab() === 'manage') {
                    <div class="tab-pane manage-markets-pane">
                        <div class="manage-header-row">
                            <div>
                                <h2 class="section-title">All Live Prediction Markets</h2>
                                <p class="section-desc">Full administrative control: inspect status, modify parameters, or permanently purge markets.</p>
                            </div>
                            <div class="manage-header-actions">
                                <app-button variant="outline" size="sm" (btnClick)="loadActiveMarkets()" ariaLabel="Refresh markets list">
                                    Refresh List
                                </app-button>
                                <app-button variant="primary" size="sm" (btnClick)="activeTab.set('create')" ariaLabel="Create new market">
                                    <svg lucidePlus [size]="14" aria-hidden="true"></svg>
                                    <span>Create Market</span>
                                </app-button>
                            </div>
                        </div>

                        @if (activeMarkets().length === 0) {
                            <div class="empty-markets-card">
                                <div class="empty-icon-circle">
                                    <svg lucideSliders [size]="32" aria-hidden="true"></svg>
                                </div>
                                <h3 class="empty-title">Zero Markets in Neon Database</h3>
                                <p class="empty-desc">
                                    All pre-fed markets have been purged. Use the creation form to deploy your first live prediction market.
                                </p>
                                <app-button variant="primary" size="default" (btnClick)="activeTab.set('create')" ariaLabel="Create first market">
                                    <svg lucidePlus [size]="16" aria-hidden="true"></svg>
                                    <span>Create Prediction Market</span>
                                </app-button>
                            </div>
                        } @else {
                            <div class="markets-table-container">
                                <table class="markets-table" role="table">
                                    <thead>
                                        <tr>
                                            <th>Market Question</th>
                                            <th>Category</th>
                                            <th>Status</th>
                                            <th>Resolution Date</th>
                                            <th>Volume</th>
                                            <th>Probability</th>
                                            <th class="th-actions">Actions</th>
                                        </tr>
                                    </thead>
                                    <tbody>
                                        @for (m of activeMarkets(); track m.id) {
                                            <tr>
                                                <td class="td-market">
                                                    <div class="market-cell-title">
                                                        <a [routerLink]="'/markets/' + m.slug" class="table-market-link" target="_blank" rel="noopener">
                                                            {{ m.title }}
                                                        </a>
                                                        <span class="market-cell-id mono-sub">ID: {{ m.id.slice(0, 8) }}...</span>
                                                    </div>
                                                </td>
                                                <td>
                                                    <app-badge variant="outline" size="sm">{{ m.category | uppercase }}</app-badge>
                                                </td>
                                                <td>
                                                    <app-badge [variant]="getStatusBadgeVariant(m.status)" size="sm">
                                                        {{ m.status | uppercase }}
                                                    </app-badge>
                                                </td>
                                                <td class="tabular-nums mono-sub">
                                                    {{ m.resolution_date | date: 'mediumDate' }}
                                                </td>
                                                <td class="tabular-nums mono-sub">&#36;{{ m.reserves?.total_volume_usdc || '0' }}</td>
                                                <td class="tabular-nums prob-cell">
                                                    <span class="prob-yes">▲ {{ m.probability_yes_pct }}</span>
                                                    <span class="prob-sep">/</span>
                                                    <span class="prob-no">▼ {{ m.probability_no_pct }}</span>
                                                </td>
                                                <td class="td-actions">
                                                    <div class="action-buttons-group">
                                                        <app-button
                                                            variant="secondary"
                                                            size="sm"
                                                            (btnClick)="openEditModal(m)"
                                                            ariaLabel="Edit market {{ m.title }}"
                                                        >
                                                            <svg lucidePencil [size]="14" aria-hidden="true"></svg>
                                                            <span>Edit</span>
                                                        </app-button>
                                                        <app-button
                                                            variant="destructive"
                                                            size="sm"
                                                            (btnClick)="openDeleteModal(m)"
                                                            ariaLabel="Delete market {{ m.title }}"
                                                        >
                                                            <svg lucideTrash2 [size]="14" aria-hidden="true"></svg>
                                                            <span>Delete</span>
                                                        </app-button>
                                                    </div>
                                                </td>
                                            </tr>
                                        }
                                    </tbody>
                                </table>
                            </div>
                        }
                    </div>
                }

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
                                    @if (res.settlement_digest || res.proof_hash) {
                                        <div class="res-audit-section">
                                            @if (res.settlement_digest) {
                                                <div class="res-audit-row">
                                                    <span class="stat-k">Settlement Digest:</span>
                                                    <code class="audit-digest-val">{{ res.settlement_digest }}</code>
                                                </div>
                                            }
                                            @if (res.proof_hash) {
                                                <div class="res-audit-row">
                                                    <span class="stat-k">Proof Hash:</span>
                                                    <code class="audit-digest-val">{{ res.proof_hash }}</code>
                                                </div>
                                            }
                                            @if (res.oracle_proof) {
                                                <div class="res-audit-row">
                                                    <span class="stat-k">Oracle Proof:</span>
                                                    <code class="audit-digest-val">{{ res.oracle_proof }}</code>
                                                </div>
                                            }
                                        </div>
                                    }
                                </div>
                            }
                        </div>
                    </div>
                }

                <!-- Edit Market Modal Dialog -->
                <app-dialog
                    [open]="!!editingMarket()"
                    (closed)="closeEditModal()"
                    title="Edit Prediction Market"
                    description="Modify live market question, status, or resolution criteria."
                    size="lg"
                    role="dialog"
                    ariaLabel="Edit Prediction Market Dialog"
                >
                    @if (editingMarket(); as em) {
                        <form class="market-form" (submit)="onSubmitEditMarket($event)">
                            <div class="form-group">
                                <app-label htmlFor="edit-title">Market Question / Title *</app-label>
                                <app-input
                                    id="edit-title"
                                    [value]="editTitle()"
                                    (valueChange)="onValueChange(editTitle, $event)"
                                    ariaLabel="Edit Market Title"
                                />
                            </div>

                            <div class="form-row-2">
                                <div class="form-group">
                                    <app-label htmlFor="edit-category">Category *</app-label>
                                    <app-select
                                        id="edit-category"
                                        [options]="categoryOptions"
                                        [value]="editCategory()"
                                        (valueChange)="editCategory.set($event)"
                                        ariaLabel="Edit Market Category"
                                    />
                                </div>
                                <div class="form-group">
                                    <app-label htmlFor="edit-status">Status *</app-label>
                                    <app-select
                                        id="edit-status"
                                        [options]="statusOptions"
                                        [value]="editStatus()"
                                        (valueChange)="onStatusChange($event)"
                                        ariaLabel="Edit Market Status"
                                    />
                                </div>
                            </div>

                            <div class="form-group">
                                <app-label htmlFor="edit-date">Resolution Date (UTC) *</app-label>
                                <app-input
                                    id="edit-date"
                                    type="datetime-local"
                                    variant="mono"
                                    [value]="editResolutionDateInput()"
                                    (valueChange)="onValueChange(editResolutionDateInput, $event)"
                                    ariaLabel="Edit Resolution Date"
                                />
                            </div>

                            <div class="form-group">
                                <app-label htmlFor="edit-desc">Detailed Resolution Criteria *</app-label>
                                <app-textarea
                                    id="edit-desc"
                                    [rows]="3"
                                    [value]="editDescription()"
                                    (valueChange)="editDescription.set($event)"
                                    ariaLabel="Edit Description"
                                />
                            </div>

                            <div class="form-group">
                                <app-label htmlFor="edit-source">Authoritative Resolution Source *</app-label>
                                <app-input
                                    id="edit-source"
                                    [value]="editResolutionSource()"
                                    (valueChange)="onValueChange(editResolutionSource, $event)"
                                    ariaLabel="Edit Resolution Source"
                                />
                            </div>

                            <div class="form-group">
                                <app-label htmlFor="edit-image">Image URL</app-label>
                                <app-input
                                    id="edit-image"
                                    [value]="editImageUrl()"
                                    (valueChange)="onValueChange(editImageUrl, $event)"
                                    ariaLabel="Edit Image URL"
                                />
                            </div>

                            <div class="dialog-actions">
                                <app-button variant="outline" size="default" type="button" (btnClick)="closeEditModal()" ariaLabel="Cancel editing">
                                    Cancel
                                </app-button>
                                <app-button variant="primary" size="default" type="submit" [loading]="isEditingSubmitting()" ariaLabel="Save market changes">
                                    Save Changes
                                </app-button>
                            </div>
                        </form>
                    }
                </app-dialog>

                <!-- Delete Confirmation Modal -->
                <app-dialog
                    [open]="!!deletingMarket()"
                    (closed)="closeDeleteModal()"
                    title="Delete Prediction Market"
                    description="Permanently delete prediction market and purge double-entry records."
                    size="sm"
                    role="alertdialog"
                    ariaLabel="Delete Prediction Market Alert"
                >
                    @if (deletingMarket(); as dm) {
                        <div class="delete-body">
                            <p class="delete-warning-text">Are you sure you want to permanently delete this market?</p>
                            <div class="delete-market-preview">
                                <span class="preview-label">Market Question:</span>
                                <strong class="preview-title">{{ dm.title }}</strong>
                                <span class="mono-sub">ID: {{ dm.id }}</span>
                            </div>
                            <div class="danger-box">
                                <p class="danger-text">
                                    Warning: This permanently removes the market record, all liquidity pool balances, order-flow trades, and double-entry ledger
                                    entries from Neon DB. This action cannot be reversed.
                                </p>
                            </div>

                            <div class="dialog-actions">
                                <app-button variant="outline" size="default" type="button" (btnClick)="closeDeleteModal()" ariaLabel="Cancel deletion">
                                    Cancel
                                </app-button>
                                <app-button
                                    variant="destructive"
                                    size="default"
                                    type="button"
                                    [loading]="isDeletingSubmitting()"
                                    (btnClick)="confirmDeleteMarket()"
                                    ariaLabel="Confirm permanent deletion"
                                >
                                    Permanently Delete
                                </app-button>
                            </div>
                        </div>
                    }
                </app-dialog>
            }
        </div>
    `,
    styles: [
        `
            :host {
                display: block;
                min-height: calc(100vh - 60px);
                background-color: var(--canvas);
                color: var(--ink);
                padding: var(--space-xl) var(--space-lg) 80px;
            }

            .admin-container {
                max-width: 1200px;
                margin: 0 auto;
                display: flex;
                flex-direction: column;
                gap: var(--space-xl);
            }

            .admin-header {
                display: flex;
                align-items: flex-start;
                justify-content: space-between;
                gap: var(--space-md);
                border-bottom: 1px solid var(--hairline);
                padding-bottom: var(--space-xl);
            }

            .admin-badge-row,
            .admin-title-row,
            .header-right,
            .admin-identity-pill,
            .manage-header-actions,
            .prob-cell,
            .action-buttons-group,
            .dialog-actions,
            .success-header,
            .res-summary-title,
            .workspace-tabs,
            .prob-split-labels,
            .view-market-link {
                display: flex;
                align-items: center;
            }

            .admin-badge-row {
                gap: var(--space-xs);
                margin-bottom: var(--space-xs);
            }

            .badge-icon {
                margin-right: var(--space-xxs);
                color: var(--primary-border);
            }

            .env-tag {
                font-family: var(--font-mono);
                font-size: 11px;
                font-weight: 700;
                color: var(--status-warning);
                background-color: var(--status-warning-bg);
                border: 1px solid var(--status-warning-border);
                padding: var(--space-xxs) var(--space-xs);
                border-radius: var(--radius-xs);
                letter-spacing: 0.5px;
            }

            .admin-title-row {
                gap: var(--space-sm);
                margin-bottom: var(--space-xs);
            }

            .admin-title {
                font-size: 28px;
                font-weight: 800;
                color: var(--ink);
                margin: 0;
                letter-spacing: var(--letter-spacing-heading);
            }

            .admin-subtitle {
                font-size: 14px;
                color: var(--muted);
                max-width: 720px;
                line-height: var(--line-height-prose);
                margin: 0;
            }

            .header-right {
                gap: var(--space-sm);
            }

            .admin-identity-pill {
                gap: var(--space-xs);
                background-color: var(--primary-subtle);
                border: 1px solid var(--primary-border);
                padding: 6px var(--space-sm);
                border-radius: var(--radius-pill);
                font-size: 12px;
            }

            .identity-dot {
                width: 7px;
                height: 7px;
                border-radius: 50%;
                background-color: var(--outcome-yes);
                box-shadow: 0 0 6px var(--outcome-yes);
            }

            .identity-role {
                font-weight: 700;
                color: var(--primary-text);
            }

            .identity-email {
                color: var(--ink-secondary);
                font-family: var(--font-mono);
                font-size: 11px;
            }

            /* Common Card Containers */
            .gatekeeper-card,
            .form-card,
            .preview-card,
            .markets-table-container,
            .empty-markets-card {
                background-color: var(--surface-card);
                border: 1px solid var(--hairline);
                border-radius: var(--radius-lg);
            }

            /* Gatekeeper Card */
            .gatekeeper-card {
                max-width: 520px;
                margin: var(--space-section) auto;
                overflow: hidden;
            }

            .gatekeeper-body {
                padding: var(--space-xxl) var(--space-xl);
                display: flex;
                flex-direction: column;
                align-items: center;
                text-align: center;
                gap: var(--space-md);
            }

            .gate-icon-circle {
                width: 64px;
                height: 64px;
                border-radius: 50%;
                background-color: var(--primary-subtle);
                border: 1px solid var(--primary-border);
                display: flex;
                align-items: center;
                justify-content: center;
            }

            .gate-icon {
                color: var(--primary-border);
            }

            .gate-title,
            .card-title,
            .section-title,
            .preview-title,
            .empty-title {
                font-size: 18px;
                font-weight: 700;
                color: var(--ink);
                margin: 0;
            }

            .section-title {
                font-size: 20px;
                margin-bottom: var(--space-xxs);
            }

            .section-desc,
            .card-desc,
            .preview-subtitle,
            .empty-desc {
                font-size: 13px;
                color: var(--muted);
                margin: 0;
            }

            .db-admin-notice {
                width: 100%;
                padding: var(--space-md);
                border-radius: var(--radius-md);
                text-align: left;
                display: flex;
                flex-direction: column;
                gap: var(--space-xs);
                box-sizing: border-box;
            }

            .db-admin-notice.warning-box {
                background-color: var(--status-warning-bg);
                border: 1px solid var(--status-warning-border);
                color: var(--status-warning);
            }

            .db-admin-notice.info-box {
                background-color: var(--status-info-bg);
                border: 1px solid var(--status-info-border);
                color: var(--status-info);
            }

            .notice-title {
                font-size: 13px;
                font-weight: 700;
                margin: 0;
            }

            .notice-desc {
                font-size: 12px;
                color: var(--body);
                margin: 0;
                line-height: var(--line-height-prose);
            }

            .user-highlight {
                color: var(--ink);
            }

            .notice-sql-label {
                font-size: 11px;
                font-weight: 600;
                color: var(--ink-secondary);
                margin: var(--space-xxs) 0 0;
            }

            .sql-code-block {
                margin: 0;
                padding: var(--space-xs) var(--space-sm);
                background-color: var(--surface-terminal);
                border: 1px solid var(--hairline);
                border-radius: var(--radius-xs);
                font-family: var(--font-mono);
                font-size: 11px;
                color: var(--primary-text);
                overflow-x: auto;
                white-space: pre-wrap;
                word-break: break-all;
            }

            .notice-hint,
            .hint-text {
                font-size: 11px;
                color: var(--muted);
                margin: 0;
            }

            .gate-form {
                width: 100%;
                display: flex;
                flex-direction: column;
                gap: var(--space-sm);
                margin-top: var(--space-xs);
                text-align: left;
                box-sizing: border-box;
            }

            .dev-hint-row {
                display: flex;
                align-items: center;
                justify-content: space-between;
                font-size: 12px;
            }

            /* Workspace Tabs */
            .workspace-tabs {
                gap: var(--space-xs);
                border-bottom: 1px solid var(--hairline);
                padding-bottom: var(--space-xs);
                flex-wrap: wrap;
            }

            .tab-btn {
                display: inline-flex;
                align-items: center;
                gap: var(--space-xs);
                min-height: var(--touch-target-min, 40px);
                padding: var(--space-xs) var(--space-md);
                border-radius: var(--radius-md);
                background-color: transparent;
                border: 1px solid transparent;
                color: var(--muted);
                font-size: 14px;
                font-weight: 600;
                cursor: pointer;
                transition:
                    color 0.15s ease,
                    background-color 0.15s ease,
                    border-color 0.15s ease;
            }

            .tab-btn:hover {
                background-color: var(--surface-card);
                color: var(--ink);
            }

            .tab-btn.active {
                background-color: var(--surface-card-elevated);
                color: var(--ink);
                border-color: var(--hairline);
                font-weight: 700;
            }

            /* Manage Markets Tab */
            .manage-markets-pane {
                display: flex;
                flex-direction: column;
                gap: var(--space-lg);
            }

            .manage-header-row {
                display: flex;
                align-items: center;
                justify-content: space-between;
                gap: var(--space-md);
                flex-wrap: wrap;
            }

            .manage-header-actions {
                gap: var(--space-xs);
            }

            .empty-markets-card {
                border-style: dashed;
                padding: var(--space-section) var(--space-xl);
                text-align: center;
                display: flex;
                flex-direction: column;
                align-items: center;
                gap: var(--space-sm);
            }

            .empty-icon-circle {
                width: 56px;
                height: 56px;
                border-radius: 50%;
                background-color: var(--primary-subtle);
                color: var(--primary);
                display: flex;
                align-items: center;
                justify-content: center;
            }

            .empty-desc {
                max-width: 440px;
                margin-bottom: var(--space-xs);
            }

            .markets-table-container {
                overflow-x: auto;
            }

            .markets-table {
                width: 100%;
                border-collapse: collapse;
                text-align: left;
                font-size: 13px;
            }

            .markets-table th {
                padding: 14px var(--space-md);
                font-size: 11px;
                font-weight: 700;
                text-transform: uppercase;
                letter-spacing: 0.5px;
                color: var(--muted);
                border-bottom: 1px solid var(--hairline);
                background-color: var(--canvas-subtle);
            }

            .markets-table td {
                padding: 14px var(--space-md);
                border-bottom: 1px solid var(--hairline);
                color: var(--ink);
                vertical-align: middle;
            }

            .markets-table tbody tr:hover {
                background-color: var(--canvas-subtle);
            }

            .td-market {
                max-width: 320px;
            }

            .market-cell-title {
                display: flex;
                flex-direction: column;
                gap: var(--space-xxs);
            }

            .table-market-link {
                color: var(--ink);
                font-weight: 600;
                text-decoration: none;
                transition: color 0.15s ease;
            }

            .table-market-link:hover {
                color: var(--link);
                text-decoration: underline;
            }

            .mono-sub {
                font-family: var(--font-mono);
                font-size: 11px;
                color: var(--muted);
            }

            .prob-cell {
                font-family: var(--font-mono);
                font-weight: 700;
                gap: var(--space-xxs);
            }

            .prob-yes {
                color: var(--outcome-yes);
            }

            .prob-sep {
                color: var(--hairline);
            }

            .prob-no {
                color: var(--outcome-no);
            }

            .th-actions,
            .td-actions {
                text-align: right;
            }

            .action-buttons-group {
                gap: var(--space-xs);
                justify-content: flex-end;
            }

            /* Create Market Layout */
            .create-market-grid {
                display: grid;
                grid-template-columns: 3fr 2fr;
                gap: var(--space-xl);
            }

            @media (max-width: 900px) {
                .create-market-grid {
                    grid-template-columns: 1fr;
                }
            }

            .form-card {
                padding: var(--space-xl);
                display: flex;
                flex-direction: column;
                gap: var(--space-md);
            }

            .market-form {
                display: flex;
                flex-direction: column;
                gap: var(--space-md);
            }

            .form-group {
                display: flex;
                flex-direction: column;
                gap: var(--space-xxs);
            }

            .form-row-2 {
                display: grid;
                grid-template-columns: 1fr 1fr;
                gap: var(--space-md);
            }

            @media (max-width: 600px) {
                .form-row-2 {
                    grid-template-columns: 1fr;
                }
            }

            .form-actions {
                margin-top: var(--space-xs);
            }

            /* Preview Column */
            .preview-column {
                display: flex;
                flex-direction: column;
                gap: var(--space-lg);
            }

            .preview-card {
                padding: var(--space-lg);
                display: flex;
                flex-direction: column;
                gap: var(--space-md);
            }

            .prob-visual-track {
                width: 100%;
                height: 10px;
                border-radius: var(--radius-pill);
                display: flex;
                overflow: hidden;
                background-color: var(--canvas-subtle);
                border: 1px solid var(--hairline);
            }

            .track-yes {
                background-color: var(--outcome-yes);
                transition: width 0.2s ease;
            }

            .track-no {
                background-color: var(--outcome-no);
                transition: width 0.2s ease;
            }

            .prob-split-labels {
                justify-content: space-between;
                font-family: var(--font-mono);
                font-size: 12px;
                font-weight: 700;
            }

            .label-yes {
                color: var(--outcome-yes);
            }

            .label-no {
                color: var(--outcome-no);
            }

            .preview-stats-grid {
                display: grid;
                grid-template-columns: 1fr 1fr;
                gap: var(--space-xs);
                border-top: 1px solid var(--hairline);
                padding-top: var(--space-sm);
            }

            .stat-box {
                display: flex;
                flex-direction: column;
                gap: var(--space-xxs);
                background-color: var(--canvas-subtle);
                border: 1px solid var(--hairline);
                border-radius: var(--radius-md);
                padding: var(--space-xs) var(--space-sm);
            }

            .stat-k {
                font-size: 11px;
                color: var(--muted);
            }

            .stat-v {
                font-family: var(--font-mono);
                font-size: 13px;
                font-weight: 700;
                color: var(--ink);
                font-feature-settings: 'tnum' 1;
            }

            .info-alert {
                display: flex;
                align-items: flex-start;
                gap: var(--space-xs);
                padding: var(--space-xs) var(--space-sm);
                background-color: var(--status-info-bg);
                border: 1px solid var(--status-info-border);
                border-radius: var(--radius-md);
                font-size: 12px;
                color: var(--status-info);
                line-height: var(--line-height-prose);
            }

            .info-icon {
                color: var(--status-info);
                flex-shrink: 0;
                margin-top: 2px;
            }

            .success-banner {
                background-color: var(--status-profit-bg);
                border: 1px solid var(--status-profit-border);
                border-radius: var(--radius-lg);
                padding: var(--space-md) var(--space-lg);
                display: flex;
                flex-direction: column;
                gap: var(--space-xs);
            }

            .success-header {
                gap: var(--space-xs);
            }

            .success-icon {
                color: var(--status-profit);
            }

            .success-title {
                font-size: 14px;
                font-weight: 700;
                color: var(--status-profit);
            }

            .success-text {
                font-size: 13px;
                color: var(--ink);
                margin: 0;
            }

            .view-market-link {
                gap: var(--space-xxs);
                color: var(--link);
                font-size: 13px;
                font-weight: 600;
                text-decoration: none;
            }

            .view-market-link:hover {
                color: var(--link-active);
                text-decoration: underline;
            }

            /* Resolve Market Pane */
            .resolve-card {
                max-width: 640px;
            }

            .outcome-select-row {
                display: grid;
                grid-template-columns: 1fr 1fr;
                gap: var(--space-sm);
            }

            .outcome-glyph {
                margin-right: var(--space-xxs);
            }

            .resolution-summary-box {
                margin-top: var(--space-md);
                padding: var(--space-md);
                background-color: var(--status-profit-bg);
                border: 1px solid var(--status-profit-border);
                border-radius: var(--radius-md);
                display: flex;
                flex-direction: column;
                gap: var(--space-sm);
            }

            .res-summary-title {
                gap: var(--space-xs);
                font-size: 14px;
                font-weight: 700;
                color: var(--status-profit);
            }

            .res-stats-grid {
                display: grid;
                grid-template-columns: 1fr 1fr 1fr;
                gap: var(--space-xs);
            }

            .res-stat {
                display: flex;
                flex-direction: column;
                gap: var(--space-xxs);
            }

            .res-audit-section {
                display: flex;
                flex-direction: column;
                gap: var(--space-xs);
                padding-top: var(--space-sm);
                border-top: 1px solid var(--hairline);
            }

            .res-audit-row {
                display: flex;
                flex-direction: column;
                gap: var(--space-xxs);
            }

            .audit-digest-val {
                font-family: var(--font-mono);
                font-size: 11px;
                color: var(--ink-secondary);
                word-break: break-all;
                background-color: var(--surface-terminal);
                padding: var(--space-xxs) var(--space-xs);
                border-radius: var(--radius-xs);
                border: 1px solid var(--hairline);
            }

            /* Dialog actions and Delete Body */
            .dialog-actions {
                justify-content: flex-end;
                gap: var(--space-xs);
                margin-top: var(--space-md);
            }

            .delete-body {
                display: flex;
                flex-direction: column;
                gap: var(--space-md);
            }

            .delete-warning-text {
                font-size: 14px;
                color: var(--ink);
                margin: 0;
            }

            .delete-market-preview {
                display: flex;
                flex-direction: column;
                gap: var(--space-xxs);
                padding: var(--space-xs) var(--space-sm);
                background-color: var(--canvas-subtle);
                border: 1px solid var(--hairline);
                border-radius: var(--radius-md);
            }

            .preview-label {
                font-size: 11px;
                text-transform: uppercase;
                letter-spacing: 0.5px;
                color: var(--muted);
            }

            .danger-box {
                padding: var(--space-xs) var(--space-sm);
                background-color: var(--status-loss-bg);
                border: 1px solid var(--status-loss-border);
                border-radius: var(--radius-md);
            }

            .danger-text {
                font-size: 12px;
                color: var(--status-loss);
                margin: 0;
                line-height: var(--line-height-prose);
            }
        `
    ]
})
export class AdminDashboardComponent implements OnInit {
    private readonly apiService = inject(ApiService);
    private readonly toastService = inject(ToastService);
    readonly authStore = inject(AuthStore);

    readonly isUnlocked = signal<boolean>(false);
    readonly isVerifying = signal<boolean>(false);
    readonly isSubmitting = signal<boolean>(false);
    readonly isResolving = signal<boolean>(false);
    readonly isEditingSubmitting = signal<boolean>(false);
    readonly isDeletingSubmitting = signal<boolean>(false);
    readonly isDev = signal<boolean>(typeof window !== 'undefined' && (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'));

    readonly tokenInput = signal<string>('');
    readonly adminToken = signal<string>('');
    readonly activeTab = signal<'manage' | 'create' | 'resolve'>('manage');

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
    readonly resolutionSummary = signal<ResolveMarketResponse | null>(null);

    // Edit Market Modal fields
    readonly editingMarket = signal<Market | null>(null);
    readonly editTitle = signal<string>('');
    readonly editCategory = signal<string>('ai');
    readonly editStatus = signal<'active' | 'suspended' | 'closed' | 'resolved'>('active');
    readonly editResolutionDateInput = signal<string>('');
    readonly editDescription = signal<string>('');
    readonly editResolutionSource = signal<string>('');
    readonly editImageUrl = signal<string>('');

    // Delete Market Modal fields
    readonly deletingMarket = signal<Market | null>(null);

    constructor() {
        // Auto-unlock if the logged-in user has is_admin = true in Neon DB
        effect(() => {
            if (this.authStore.isAdmin()) {
                this.isUnlocked.set(true);
                this.loadActiveMarkets();
            }
        });
    }

    onValueChange(targetSignal: WritableSignal<string>, val: string | number): void {
        targetSignal.set(String(val ?? ''));
    }

    onStatusChange(val: string): void {
        if (val === 'active' || val === 'suspended' || val === 'closed' || val === 'resolved') {
            this.editStatus.set(val);
        }
    }

    readonly categoryOptions: SelectOption[] = [
        { value: 'ai', label: 'AI & Tech' },
        { value: 'crypto', label: 'Crypto' },
        { value: 'macro', label: 'Macro Economy' },
        { value: 'science', label: 'Science & Space' }
    ];

    readonly statusOptions: SelectOption[] = [
        { value: 'active', label: 'Active (Trading Open)' },
        { value: 'suspended', label: 'Suspended (Trading Halted)' },
        { value: 'closed', label: 'Closed (Awaiting Settlement)' },
        { value: 'resolved', label: 'Resolved (Finalized)' }
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

    getStatusBadgeVariant(status: string): BadgeVariant {
        switch (status) {
            case 'active':
                return 'profit';
            case 'suspended':
                return 'warning';
            case 'closed':
                return 'secondary';
            case 'resolved':
                return 'resolved';
            default:
                return 'outline';
        }
    }

    private getEffectiveToken(): string {
        return this.authStore.token() || this.adminToken();
    }

    ngOnInit(): void {
        this.initDefaultDate();
        this.apiService.getConfig().subscribe({
            next: (cfg) => {
                if (cfg) {
                    this.isDev.set(cfg.is_dev);
                }
            }
        });

        if (this.authStore.isAdmin()) {
            this.isUnlocked.set(true);
            this.loadActiveMarkets();
            return;
        }

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

    private formatForDateTimeLocal(isoDateStr?: string): string {
        if (!isoDateStr) return '';
        const d = new Date(isoDateStr);
        if (isNaN(d.getTime())) return '';
        const y = d.getFullYear();
        const m = String(d.getMonth() + 1).padStart(2, '0');
        const day = String(d.getDate()).padStart(2, '0');
        const hours = String(d.getHours()).padStart(2, '0');
        const minutes = String(d.getMinutes()).padStart(2, '0');
        return `${y}-${m}-${day}T${hours}:${minutes}`;
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

    // --- Market Creation ---
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
        this.apiService.createMarket(payload, this.getEffectiveToken()).subscribe({
            next: (res) => {
                this.isSubmitting.set(false);
                this.createdMarket.set(res);
                this.toastService.success('Market Created', `"${res.title}" successfully provisioned!`);
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

    // --- Market Editing ---
    openEditModal(m: Market): void {
        this.editingMarket.set(m);
        this.editTitle.set(m.title);
        this.editCategory.set(m.category);
        this.editStatus.set(m.status);
        this.editResolutionDateInput.set(this.formatForDateTimeLocal(m.resolution_date));
        this.editDescription.set(m.description || '');
        this.editResolutionSource.set(m.resolution_source || '');
        this.editImageUrl.set(m.image_url || '');
    }

    closeEditModal(): void {
        this.editingMarket.set(null);
    }

    onSubmitEditMarket(e: Event): void {
        e.preventDefault();
        const m = this.editingMarket();
        if (!m) return;

        const title = this.editTitle().trim();
        const desc = this.editDescription().trim();
        const source = this.editResolutionSource().trim();
        const rawDate = this.editResolutionDateInput().trim();

        if (!title || !desc || !source || !rawDate) {
            this.toastService.warning('Validation Error', 'Please complete all required fields');
            return;
        }

        const payload: EditMarketRequest = {
            title,
            description: desc,
            category: this.editCategory(),
            resolution_source: source,
            resolution_date: new Date(rawDate).toISOString(),
            image_url: this.editImageUrl().trim(),
            status: this.editStatus()
        };

        this.isEditingSubmitting.set(true);
        this.apiService.editMarket(m.id, payload, this.getEffectiveToken()).subscribe({
            next: (updated) => {
                this.isEditingSubmitting.set(false);
                this.closeEditModal();
                this.toastService.success('Market Updated', `"${updated.title}" successfully updated!`);
                this.loadActiveMarkets();
            },
            error: (err) => {
                this.isEditingSubmitting.set(false);
                const msg = err?.error?.message || 'Failed to update market';
                this.toastService.error('Update Error', msg);
            }
        });
    }

    // --- Market Deletion ---
    openDeleteModal(m: Market): void {
        this.deletingMarket.set(m);
    }

    closeDeleteModal(): void {
        this.deletingMarket.set(null);
    }

    confirmDeleteMarket(): void {
        const m = this.deletingMarket();
        if (!m) return;

        this.isDeletingSubmitting.set(true);
        this.apiService.deleteMarket(m.id, this.getEffectiveToken()).subscribe({
            next: (res) => {
                this.isDeletingSubmitting.set(false);
                this.closeDeleteModal();
                this.toastService.success('Market Deleted', `"${m.title}" was permanently removed.`);
                this.loadActiveMarkets();
            },
            error: (err) => {
                this.isDeletingSubmitting.set(false);
                const msg = err?.error?.message || 'Failed to delete market';
                this.toastService.error('Delete Error', msg);
            }
        });
    }

    // --- Market Resolution ---
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
        this.apiService.resolveMarket(marketId, { winning_outcome: outcome, oracle_proof: proof }, this.getEffectiveToken(), idempotencyKey).subscribe({
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
