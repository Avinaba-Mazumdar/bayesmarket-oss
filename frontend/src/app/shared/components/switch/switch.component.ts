import { ChangeDetectionStrategy, Component, computed, input, model } from '@angular/core';

export type SwitchSize = 'sm' | 'default' | 'lg';
export type SwitchColor = 'primary' | 'yes';

@Component({
    selector: 'app-switch',
    standalone: true,
    changeDetection: ChangeDetectionStrategy.OnPush,
    template: `
        <button
            type="button"
            role="switch"
            [id]="id() || null"
            [disabled]="disabled()"
            [attr.aria-checked]="checked()"
            [attr.aria-label]="ariaLabel() || null"
            [class]="switchClass()"
            (click)="toggle()"
            (keydown.space)="$event.preventDefault(); toggle()"
            (keydown.enter)="$event.preventDefault(); toggle()"
        >
            <span class="switch-track">
                <span class="switch-thumb"></span>
            </span>
        </button>
    `,
    styles: [
        `
            :host {
                display: inline-flex;
                vertical-align: middle;
            }

            /* The outer button satisfies WCAG min 40x40px touch target */
            button {
                display: inline-flex;
                align-items: center;
                justify-content: center;
                min-width: var(--touch-target-min, 40px);
                min-height: var(--touch-target-min, 40px);
                padding: 4px;
                background: transparent;
                border: none;
                outline: none;
                cursor: pointer;
                user-select: none;
                box-sizing: border-box;
            }

            button:focus-visible .switch-track {
                outline: 2px solid var(--focus-outline, #e84089);
                outline-offset: 2px;
            }

            button:disabled {
                opacity: 0.45;
                cursor: not-allowed;
            }

            /* --- Switch Track --- */
            .switch-track {
                position: relative;
                display: inline-flex;
                align-items: center;
                border-radius: var(--radius-pill, 9999px);
                border: 1px solid var(--border-strong, #cbd5e1);
                background-color: var(--canvas-subtle, #f1f5f9);
                transition:
                    background-color 0.2s cubic-bezier(0.16, 1, 0.3, 1),
                    border-color 0.2s cubic-bezier(0.16, 1, 0.3, 1),
                    transform 0.1s ease;
                box-sizing: border-box;
            }

            button:active:not(:disabled) .switch-track {
                transform: scale(0.96);
            }

            /* --- Switch Thumb --- */
            .switch-thumb {
                display: block;
                border-radius: 50%;
                background-color: #ffffff;
                box-shadow: 0 1px 3px rgba(0, 0, 0, 0.5);
                transition: transform 0.2s cubic-bezier(0.16, 1, 0.3, 1);
            }

            /* --- Sizes --- */
            /* Default */
            .switch-size-default .switch-track {
                width: 44px;
                height: 24px;
                padding: 2px;
            }
            .switch-size-default .switch-thumb {
                width: 18px;
                height: 18px;
                transform: translateX(0);
            }
            .switch-checked.switch-size-default .switch-thumb {
                transform: translateX(20px);
            }

            /* Small */
            .switch-size-sm .switch-track {
                width: 36px;
                height: 20px;
                padding: 2px;
            }
            .switch-size-sm .switch-thumb {
                width: 14px;
                height: 14px;
                transform: translateX(0);
            }
            .switch-checked.switch-size-sm .switch-thumb {
                transform: translateX(16px);
            }

            /* Large */
            .switch-size-lg .switch-track {
                width: 52px;
                height: 28px;
                padding: 3px;
            }
            .switch-size-lg .switch-thumb {
                width: 20px;
                height: 20px;
                transform: translateX(0);
            }
            .switch-checked.switch-size-lg .switch-thumb {
                transform: translateX(24px);
            }

            /* --- Checked Colors --- */
            .switch-checked.switch-color-primary .switch-track {
                background-color: var(--primary, #a6034c);
                border-color: var(--primary-border, #e84089);
                box-shadow: var(--shadow-glow-primary, 0 0 12px rgba(166, 3, 76, 0.4));
            }

            .switch-checked.switch-color-yes .switch-track {
                background-color: var(--outcome-yes, #10b981);
                border-color: var(--outcome-yes, #10b981);
                box-shadow: 0 0 12px rgba(16, 185, 129, 0.45);
            }
        `
    ]
})
export class SwitchComponent {
    readonly checked = model<boolean>(false);
    readonly disabled = input<boolean>(false);
    readonly id = input<string>('');
    readonly size = input<SwitchSize>('default');
    readonly color = input<SwitchColor>('primary');
    readonly ariaLabel = input<string>('');

    protected readonly switchClass = computed(() => {
        const classes = ['switch-btn', `switch-size-${this.size()}`, `switch-color-${this.color()}`];
        if (this.checked()) {
            classes.push('switch-checked');
        }
        if (this.disabled()) {
            classes.push('switch-disabled');
        }
        return classes.join(' ');
    });

    toggle(): void {
        if (!this.disabled()) {
            this.checked.set(!this.checked());
        }
    }
}
