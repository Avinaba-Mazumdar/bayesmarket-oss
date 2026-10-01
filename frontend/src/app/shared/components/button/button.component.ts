import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { LucideLoaderCircle } from '@lucide/angular';

export type ButtonVariant = 'primary' | 'secondary' | 'destructive' | 'outline' | 'ghost' | 'link' | 'yes' | 'no' | 'chip';
export type ButtonSize = 'default' | 'sm' | 'lg' | 'icon' | 'pill';

@Component({
    selector: 'app-button',
    standalone: true,
    changeDetection: ChangeDetectionStrategy.OnPush,
    imports: [LucideLoaderCircle],
    host: {
        '[class.full-width]': 'fullWidth()'
    },
    template: `
        <button
            [type]="type()"
            [disabled]="disabled() || loading()"
            [class]="buttonClass()"
            [attr.aria-busy]="loading()"
            [attr.aria-pressed]="isToggleable() ? selected() : null"
            [attr.aria-label]="ariaLabel() || null"
            (click)="onClick($event)"
        >
            @if (loading()) {
                <svg lucideLoaderCircle class="btn-spinner" [size]="16" [strokeWidth]="2.5" aria-hidden="true"></svg>
            }
            <ng-content />
        </button>
    `,
    styles: [
        `
            :host {
                display: inline-block;
            }
            :host(.full-width) {
                display: block;
                width: 100%;
            }

            button {
                display: inline-flex;
                align-items: center;
                justify-content: center;
                gap: 8px;
                white-space: nowrap;
                flex-wrap: nowrap;
                font-family: var(--font-ui, system-ui, sans-serif);
                font-weight: 600;
                line-height: 1.3;
                letter-spacing: 0.1px;
                border-radius: var(--radius-lg, 14px);
                cursor: pointer;
                text-decoration: none;
                user-select: none;
                transition:
                    background-color 0.15s ease,
                    border-color 0.15s ease,
                    color 0.15s ease,
                    transform 0.1s ease,
                    box-shadow 0.15s ease;
                outline: none;
            }

            button:focus-visible {
                outline: 2px solid var(--focus-outline, #7c4dff);
                outline-offset: 2px;
            }

            button:active:not(:disabled) {
                transform: scale(0.98);
            }

            button:disabled {
                opacity: 0.45;
                cursor: not-allowed;
                filter: grayscale(30%);
            }

            /* --- Sizes (WCAG 2.2 AAA Touch Target: min 40x40px) --- */
            .btn-size-default {
                min-height: 40px;
                min-width: var(--touch-target-min, 40px);
                padding: 8px 16px;
                font-size: 14px;
            }

            .btn-size-sm {
                min-height: 36px;
                min-width: var(--touch-target-min, 40px);
                padding: 6px 12px;
                font-size: 13px;
            }

            .btn-size-lg {
                min-height: 48px;
                min-width: var(--touch-target-min, 40px);
                padding: 12px 24px;
                font-size: 15px;
            }

            .btn-size-icon {
                min-height: var(--touch-target-min, 40px);
                min-width: var(--touch-target-min, 40px);
                width: 40px;
                height: 40px;
                padding: 0;
            }

            .btn-size-pill {
                min-height: 40px;
                min-width: var(--touch-target-min, 40px);
                padding: 8px 16px;
                font-size: 13px;
                border-radius: var(--radius-pill, 9999px);
            }

            /* --- Variants --- */
            /* Primary (Electric Royal Indigo CTA) */
            .btn-variant-primary {
                background-color: var(--primary, #3600b3);
                color: var(--on-primary, #ffffff);
                border: 1px solid var(--primary-border, #7c4dff);
                box-shadow: 0 2px 10px rgba(54, 0, 179, 0.4);
            }
            .btn-variant-primary:hover:not(:disabled) {
                background-color: var(--primary-hover, #4e10d8);
                box-shadow: var(--shadow-glow-primary);
            }
            .btn-variant-primary:active:not(:disabled) {
                background-color: var(--primary-active, #29008a);
            }

            /* Secondary (Obsidian Card Surface) */
            .btn-variant-secondary {
                background-color: var(--surface-card, #131126);
                color: var(--ink, #f8f7ff);
                border: 1px solid var(--hairline, #252140);
                box-shadow: var(--shadow-sm);
            }
            .btn-variant-secondary:hover:not(:disabled) {
                background-color: var(--surface-card-elevated, #1a1733);
                border-color: var(--primary-border, #7c4dff);
            }

            /* Destructive (Loss / Sell / Cancel) */
            .btn-variant-destructive {
                background-color: #be123c;
                color: #ffffff;
                border: 1px solid var(--status-loss-border, #fb7185);
                box-shadow: var(--shadow-sm);
            }
            .btn-variant-destructive:hover:not(:disabled) {
                background-color: #9f1239;
                box-shadow: var(--shadow-glow-no);
            }

            /* Outline */
            .btn-variant-outline {
                background-color: transparent;
                color: var(--ink-secondary, #9d97b8);
                border: 1px solid var(--hairline, #252140);
            }
            .btn-variant-outline:hover:not(:disabled) {
                background-color: var(--surface-card, #131126);
                color: var(--ink, #f8f7ff);
                border-color: var(--border-strong, #3d3766);
            }

            /* Ghost */
            .btn-variant-ghost {
                background-color: transparent;
                color: var(--muted, #9d97b8);
                border: 1px solid transparent;
            }
            .btn-variant-ghost:hover:not(:disabled) {
                background-color: var(--primary-subtle, rgba(79, 70, 229, 0.08));
                color: var(--ink, #f8f7ff);
            }

            /* Link */
            .btn-variant-link {
                background-color: transparent;
                color: var(--link, #7c4dff);
                border: none;
                padding: 0;
                min-height: auto;
                min-width: auto;
            }
            .btn-variant-link:hover:not(:disabled) {
                text-decoration: underline;
                color: var(--link-active, #ffffff);
            }

            /* Outcome YES (Mint) & NO (Rose) */
            .btn-variant-yes,
            .btn-variant-no {
                border-radius: var(--radius-md, 10px);
                white-space: nowrap;
                flex-wrap: nowrap;
            }

            .btn-variant-yes.btn-size-lg,
            .btn-variant-no.btn-size-lg {
                padding: 10px 12px;
                gap: 6px;
                font-size: 14px;
            }

            /* --- Unselected Outcome Buttons --- */
            .btn-variant-yes:not(.selected) {
                background-color: var(--outcome-yes-subtle, rgba(6, 95, 70, 0.09));
                color: var(--outcome-yes-text, #065f46);
                border: 1px solid var(--outcome-yes-border, rgba(6, 95, 70, 0.35));
            }
            .btn-variant-yes:not(.selected):hover:not(:disabled) {
                background-color: var(--outcome-yes-subtle, rgba(6, 95, 70, 0.16));
                color: var(--outcome-yes-text, #065f46);
                border-color: var(--outcome-yes-border, rgba(6, 95, 70, 0.5));
                box-shadow: var(--shadow-glow-yes);
            }

            .btn-variant-no:not(.selected) {
                background-color: var(--outcome-no-subtle, rgba(159, 18, 57, 0.09));
                color: var(--outcome-no-text, #9f1239);
                border: 1px solid var(--outcome-no-border, rgba(159, 18, 57, 0.35));
            }
            .btn-variant-no:not(.selected):hover:not(:disabled) {
                background-color: var(--outcome-no-subtle, rgba(159, 18, 57, 0.16));
                color: var(--outcome-no-text, #9f1239);
                border-color: var(--outcome-no-border, rgba(159, 18, 57, 0.5));
                box-shadow: var(--shadow-glow-no);
            }

            /* --- Selected Outcome Buttons (Solid High-Contrast CTA) --- */
            .btn-variant-yes.selected {
                background-color: var(--outcome-yes-solid, #065f46);
                color: var(--outcome-yes-solid-text, #ffffff);
                border: 1px solid var(--outcome-yes-solid, #065f46);
                font-weight: 700;
                box-shadow: 0 0 16px var(--outcome-yes-glow, rgba(6, 95, 70, 0.45));
            }
            .btn-variant-yes.selected:hover:not(:disabled) {
                background-color: var(--outcome-yes-solid-hover, #044e39);
                border-color: var(--outcome-yes-solid-hover, #044e39);
                color: var(--outcome-yes-solid-text, #ffffff);
                box-shadow: 0 0 20px var(--outcome-yes-glow, rgba(6, 95, 70, 0.55));
            }
            .btn-variant-yes.selected:active:not(:disabled) {
                background-color: var(--outcome-yes-solid-hover, #044e39);
                border-color: var(--outcome-yes-solid-hover, #044e39);
                color: var(--outcome-yes-solid-text, #ffffff);
            }

            .btn-variant-no.selected {
                background-color: var(--outcome-no-solid, #9f1239);
                color: var(--outcome-no-solid-text, #ffffff);
                border: 1px solid var(--outcome-no-solid, #9f1239);
                font-weight: 700;
                box-shadow: 0 0 16px var(--outcome-no-glow, rgba(159, 18, 57, 0.45));
            }
            .btn-variant-no.selected:hover:not(:disabled) {
                background-color: var(--outcome-no-solid-hover, #881337);
                border-color: var(--outcome-no-solid-hover, #881337);
                color: var(--outcome-no-solid-text, #ffffff);
                box-shadow: 0 0 20px var(--outcome-no-glow, rgba(159, 18, 57, 0.55));
            }
            .btn-variant-no.selected:active:not(:disabled) {
                background-color: var(--outcome-no-solid-hover, #881337);
                border-color: var(--outcome-no-solid-hover, #881337);
                color: var(--outcome-no-solid-text, #ffffff);
            }

            /* Chip (Amount presets & timeframe intervals) */
            .btn-variant-chip {
                background-color: var(--surface-card, #131126);
                color: var(--ink-secondary, #9d97b8);
                font-family: var(--font-mono, monospace);
                font-size: 13px;
                letter-spacing: 0.1px;
                border: 1px solid var(--hairline, #252140);
                border-radius: var(--radius-md, 10px);
                padding: 6px 12px;
                min-height: 40px;
                font-feature-settings: 'tnum' 1;
            }
            .btn-variant-chip:hover:not(:disabled) {
                background-color: var(--surface-card-elevated, #1a1733);
                border-color: var(--primary-border, #7c4dff);
                color: var(--ink, #f8f7ff);
            }
            .btn-variant-chip.selected {
                background-color: var(--primary-subtle, rgba(54, 0, 179, 0.25));
                border-color: var(--primary-border, #7c4dff);
                color: var(--primary-text, #4338ca);
                font-weight: 700;
                box-shadow: 0 0 10px var(--primary-glow, rgba(124, 77, 255, 0.35));
            }
            .btn-variant-chip.selected:hover:not(:disabled) {
                background-color: var(--primary-subtle, rgba(54, 0, 179, 0.35));
                border-color: var(--primary-border, #7c4dff);
                color: var(--primary-text, #4338ca);
            }

            /* Full Width */
            .btn-full-width {
                width: 100%;
            }

            /* Spinner Animation */
            .btn-spinner {
                width: 16px;
                height: 16px;
                animation: spin 0.8s linear infinite;
                flex-shrink: 0;
            }

            @keyframes spin {
                from {
                    transform: rotate(0deg);
                }
                to {
                    transform: rotate(360deg);
                }
            }
        `
    ]
})
export class ButtonComponent {
    readonly variant = input<ButtonVariant>('primary');
    readonly size = input<ButtonSize>('default');
    readonly type = input<'button' | 'submit' | 'reset'>('button');
    readonly disabled = input<boolean>(false);
    readonly loading = input<boolean>(false);
    readonly fullWidth = input<boolean>(false);
    readonly selected = input<boolean>(false);
    readonly ariaLabel = input<string>('');

    readonly btnClick = output<MouseEvent>();

    protected readonly isToggleable = computed(() => {
        return this.variant() === 'yes' || this.variant() === 'no' || this.variant() === 'chip';
    });

    protected readonly buttonClass = computed(() => {
        const v = this.variant();
        const classes = ['btn', `btn-${v}`, `btn-variant-${v}`, `btn-size-${this.size()}`];
        if (this.fullWidth()) {
            classes.push('btn-full-width');
        }
        if (this.selected()) {
            classes.push('selected');
        }
        return classes.join(' ');
    });

    protected onClick(event: MouseEvent): void {
        if (!this.disabled() && !this.loading()) {
            this.btnClick.emit(event);
        }
    }
}
