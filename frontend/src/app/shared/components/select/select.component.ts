import { ChangeDetectionStrategy, Component, ElementRef, HostListener, computed, inject, input, model, signal } from '@angular/core';
import { LucideChevronDown, LucideCheck } from '@lucide/angular';

export interface SelectOption {
    value: string;
    label: string;
    disabled?: boolean;
    description?: string;
}

export type SelectSize = 'sm' | 'default' | 'lg';
export type SelectVariant = 'default' | 'mono';

@Component({
    selector: 'app-select',
    standalone: true,
    changeDetection: ChangeDetectionStrategy.OnPush,
    imports: [LucideChevronDown, LucideCheck],
    template: `
        <div class="select-container" [class.select-disabled]="disabled()">
            <button
                type="button"
                role="combobox"
                [id]="id() || null"
                [disabled]="disabled()"
                [class]="triggerClass()"
                [attr.aria-expanded]="isOpen()"
                [attr.aria-haspopup]="'listbox'"
                [attr.aria-label]="ariaLabel() || placeholder()"
                (click)="toggleOpen()"
                (keydown)="onKeyDown($event)"
            >
                <span class="trigger-text" [class.is-placeholder]="!selectedOption()">
                    {{ selectedOption() ? selectedOption()!.label : placeholder() }}
                </span>

                <svg lucideChevronDown class="chevron-icon" [class.chevron-open]="isOpen()" [size]="16" aria-hidden="true"></svg>
            </button>

            @if (isOpen()) {
                <div class="select-dropdown" role="listbox" [attr.aria-label]="ariaLabel() || placeholder()" tabindex="-1">
                    @for (opt of options(); track opt.value; let idx = $index) {
                        <div
                            role="option"
                            [attr.aria-selected]="opt.value === value()"
                            [class.selected]="opt.value === value()"
                            [class.focused]="focusedIndex() === idx"
                            [class.disabled]="opt.disabled"
                            class="select-option"
                            (click)="selectOption(opt)"
                            (mouseenter)="focusedIndex.set(idx)"
                        >
                            <div class="option-content">
                                <span class="option-label">{{ opt.label }}</span>
                                @if (opt.description) {
                                    <span class="option-desc">{{ opt.description }}</span>
                                }
                            </div>

                            @if (opt.value === value()) {
                                <svg lucideCheck class="check-icon" [size]="16" [strokeWidth]="2.5" aria-hidden="true"></svg>
                            }
                        </div>
                    }
                </div>
            }
        </div>

        @if (errorMessage()) {
            <span class="select-error-msg" role="alert">{{ errorMessage() }}</span>
        }
    `,
    styles: [
        `
            :host {
                display: block;
                width: 100%;
            }

            .select-container {
                position: relative;
                width: 100%;
            }

            .select-trigger {
                display: flex;
                align-items: center;
                justify-content: space-between;
                gap: 8px;
                width: 100%;
                background-color: var(--surface-terminal, #0e131d);
                color: var(--ink, #f8fafc);
                border: 1px solid var(--border-strong, #606e85);
                border-radius: var(--radius-lg, 14px);
                padding: 0 14px;
                box-sizing: border-box;
                font-family: var(--font-ui, system-ui, sans-serif);
                font-size: 14px;
                line-height: 1.4;
                cursor: pointer;
                user-select: none;
                transition:
                    border-color 0.15s ease,
                    box-shadow 0.15s ease;
                outline: none;
            }

            .select-trigger:focus-visible {
                border-color: var(--focus-outline, #e84089);
                box-shadow: 0 0 0 2px var(--focus-ring, rgba(232, 64, 137, 0.35));
            }

            /* --- Sizes (WCAG Touch Target: min 40px) --- */
            .select-size-default {
                min-height: var(--touch-target-min, 40px);
            }
            .select-size-sm {
                min-height: var(--touch-target-min, 40px);
                padding: 0 10px;
                font-size: 13px;
            }
            .select-size-lg {
                min-height: 52px;
                padding: 0 16px;
                font-size: 16px;
            }

            /* --- Variants --- */
            .select-variant-mono {
                font-family: var(--font-mono, monospace);
                font-feature-settings: 'tnum' 1;
            }

            /* --- Error State --- */
            .select-error {
                border-color: var(--status-loss-border, #fb7185);
            }
            .select-error:focus-visible {
                box-shadow: 0 0 0 2px rgba(251, 113, 133, 0.35);
            }

            /* --- Disabled State --- */
            .select-disabled {
                opacity: 0.45;
                cursor: not-allowed;
            }
            .select-disabled .select-trigger {
                cursor: not-allowed;
            }

            .trigger-text {
                overflow: hidden;
                text-overflow: ellipsis;
                white-space: nowrap;
            }

            .trigger-text.is-placeholder {
                color: var(--muted, #a2b4c9);
            }

            .chevron-icon {
                width: 16px;
                height: 16px;
                flex-shrink: 0;
                color: var(--muted, #a2b4c9);
                transition: transform 0.2s ease;
            }

            .chevron-open {
                transform: rotate(180deg);
            }

            /* --- Dropdown Overlay --- */
            .select-dropdown {
                position: absolute;
                top: calc(100% + 4px);
                left: 0;
                right: 0;
                z-index: 50;
                max-height: 260px;
                overflow-y: auto;
                background-color: var(--surface-card-elevated, #171f30);
                border: 1px solid var(--hairline, #1e2638);
                border-radius: var(--radius-lg, 14px);
                box-shadow: var(--shadow-lg, 0 12px 28px -4px rgba(0, 0, 0, 0.65));
                padding: 4px;
                outline: none;
            }

            .select-option {
                display: flex;
                align-items: center;
                justify-content: space-between;
                gap: 8px;
                min-height: var(--touch-target-min, 40px);
                padding: 8px 12px;
                border-radius: var(--radius-md, 10px);
                color: var(--ink, #f8fafc);
                font-family: var(--font-ui, system-ui, sans-serif);
                font-size: 14px;
                cursor: pointer;
                user-select: none;
                transition: background-color 0.1s ease;
            }

            .select-option.focused {
                background-color: var(--primary-subtle, rgba(67, 56, 202, 0.08));
            }

            .select-option.selected {
                background-color: var(--primary-subtle, rgba(67, 56, 202, 0.12));
                color: var(--primary-text, #4338ca);
                font-weight: 600;
            }

            .select-option.disabled {
                opacity: 0.4;
                cursor: not-allowed;
            }

            .option-content {
                display: flex;
                flex-direction: column;
                gap: 2px;
                overflow: hidden;
            }

            .option-label {
                overflow: hidden;
                text-overflow: ellipsis;
                white-space: nowrap;
            }

            .option-desc {
                font-size: 11px;
                color: var(--muted, #3b4861);
            }

            .check-icon {
                width: 16px;
                height: 16px;
                flex-shrink: 0;
                color: var(--primary-text, #4338ca);
            }

            /* --- Error Message --- */
            .select-error-msg {
                display: block;
                font-family: var(--font-ui, system-ui, sans-serif);
                font-size: 12px;
                font-weight: 500;
                color: var(--status-loss, #fda4af);
                margin-top: 5px;
                padding-left: 2px;
            }
        `
    ]
})
export class SelectComponent {
    private readonly elRef = inject(ElementRef);

    readonly options = input<SelectOption[]>([]);
    readonly value = model<string>('');
    readonly placeholder = input<string>('Select an option...');
    readonly disabled = input<boolean>(false);
    readonly id = input<string>('');
    readonly size = input<SelectSize>('default');
    readonly variant = input<SelectVariant>('default');
    readonly error = input<string | boolean>('');
    readonly ariaLabel = input<string>('');

    readonly isOpen = signal(false);
    readonly focusedIndex = signal<number>(-1);

    protected readonly selectedOption = computed(() => {
        const v = this.value();
        return this.options().find((opt) => opt.value === v) || null;
    });

    protected readonly errorMessage = computed(() => {
        const err = this.error();
        return typeof err === 'string' ? err : '';
    });

    protected readonly triggerClass = computed(() => {
        const classes = ['select-trigger', `select-size-${this.size()}`, `select-variant-${this.variant()}`];
        if (this.error()) {
            classes.push('select-error');
        }
        return classes.join(' ');
    });

    @HostListener('document:click', ['$event'])
    onDocumentClick(event: MouseEvent): void {
        if (this.isOpen() && !this.elRef.nativeElement.contains(event.target)) {
            this.isOpen.set(false);
        }
    }

    toggleOpen(): void {
        if (!this.disabled()) {
            const next = !this.isOpen();
            this.isOpen.set(next);
            if (next) {
                const curIdx = this.options().findIndex((o) => o.value === this.value());
                this.focusedIndex.set(curIdx >= 0 ? curIdx : 0);
            }
        }
    }

    selectOption(option: SelectOption): void {
        if (!option.disabled) {
            this.value.set(option.value);
            this.isOpen.set(false);
        }
    }

    onKeyDown(event: KeyboardEvent): void {
        if (this.disabled()) return;

        const opts = this.options();
        if (!opts.length) return;

        switch (event.key) {
            case 'ArrowDown':
                event.preventDefault();
                if (!this.isOpen()) {
                    this.isOpen.set(true);
                    this.focusedIndex.set(0);
                } else {
                    const next = (this.focusedIndex() + 1) % opts.length;
                    this.focusedIndex.set(next);
                }
                break;

            case 'ArrowUp':
                event.preventDefault();
                if (!this.isOpen()) {
                    this.isOpen.set(true);
                    this.focusedIndex.set(opts.length - 1);
                } else {
                    const prev = (this.focusedIndex() - 1 + opts.length) % opts.length;
                    this.focusedIndex.set(prev);
                }
                break;

            case 'Enter':
            case ' ':
                event.preventDefault();
                if (this.isOpen()) {
                    const focused = opts[this.focusedIndex()];
                    if (focused && !focused.disabled) {
                        this.selectOption(focused);
                    }
                } else {
                    this.toggleOpen();
                }
                break;

            case 'Escape':
                if (this.isOpen()) {
                    event.preventDefault();
                    this.isOpen.set(false);
                }
                break;
        }
    }
}
