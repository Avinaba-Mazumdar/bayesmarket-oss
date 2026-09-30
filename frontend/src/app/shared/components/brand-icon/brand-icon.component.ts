import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { CommonModule } from '@angular/common';
import { LucideTrendingUp } from '@lucide/angular';

export type BrandIconSize = 'xs' | 'sm' | 'md' | 'lg' | 'xl' | number;

@Component({
    selector: 'app-brand-icon',
    standalone: true,
    imports: [CommonModule, LucideTrendingUp],
    changeDetection: ChangeDetectionStrategy.OnPush,
    template: `
        <div
            class="brand-icon-box"
            [class.has-glow]="glow()"
            [style.width.px]="pixelSize()"
            [style.height.px]="pixelSize()"
            role="img"
            [attr.aria-label]="ariaLabel() || 'BayesMarket logo'"
        >
            <svg lucideTrendingUp class="brand-lucide-icon" [size]="iconSize()" [strokeWidth]="2.5" aria-hidden="true"></svg>
        </div>
    `,
    styles: [
        `
            :host {
                display: inline-flex;
                align-items: center;
                justify-content: center;
                vertical-align: middle;
                user-select: none;
                flex-shrink: 0;
            }

            .brand-icon-box {
                display: inline-flex;
                align-items: center;
                justify-content: center;
                background-color: var(--primary, #4338ca);
                color: #ffffff;
                border-radius: var(--radius-md, 8px);
                transition: opacity 150ms ease;
            }

            .brand-icon-box:hover {
                opacity: 0.92;
            }

            .brand-lucide-icon {
                display: block;
                color: #ffffff;
            }
        `
    ]
})
export class BrandIconComponent {
    public readonly size = input<BrandIconSize>('md');
    public readonly glow = input<boolean>(false);
    public readonly ariaLabel = input<string>('BayesMarket logo');

    protected readonly pixelSize = computed(() => {
        const s = this.size();
        if (typeof s === 'number') {
            return s;
        }
        switch (s) {
            case 'xs':
                return 18;
            case 'sm':
                return 24;
            case 'md':
                return 32;
            case 'lg':
                return 44;
            case 'xl':
                return 56;
            default:
                return 32;
        }
    });

    protected readonly iconSize = computed(() => {
        const px = this.pixelSize();
        return Math.round(px * 0.62);
    });
}
