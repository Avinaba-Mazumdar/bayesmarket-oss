import { ChangeDetectionStrategy, Component, computed, effect, input, signal } from '@angular/core';
import { LucideUser } from '@lucide/angular';

export type AvatarSize = 'sm' | 'default' | 'lg' | 'xl';
export type AvatarShape = 'circle' | 'square';
export type AvatarStatus = 'online' | 'offline' | 'busy' | 'away';

@Component({
    selector: 'app-avatar',
    standalone: true,
    changeDetection: ChangeDetectionStrategy.OnPush,
    imports: [LucideUser],
    template: `
        <div [class]="containerClass()" [attr.aria-label]="ariaLabelText()">
            @if (showImage()) {
                <img [src]="src()" [alt]="alt()" class="avatar-image" (error)="onImageError()" (load)="onImageLoad()" />
            } @else {
                <span class="avatar-fallback" aria-hidden="true">
                    @if (initials()) {
                        {{ initials() }}
                    } @else {
                        <ng-content>
                            <svg lucideUser class="avatar-default-icon" aria-hidden="true"></svg>
                        </ng-content>
                    }
                </span>
            }

            @if (status()) {
                <span [class]="statusClass()" [attr.title]="'Status: ' + status()" aria-hidden="true"></span>
            }
        </div>
    `,
    styles: [
        `
            :host {
                display: inline-flex;
                vertical-align: middle;
            }

            .avatar {
                position: relative;
                display: inline-flex;
                align-items: center;
                justify-content: center;
                overflow: hidden;
                user-select: none;
                flex-shrink: 0;
                background-color: var(--surface-card-elevated, #171f30);
                color: var(--ink, #f8fafc);
                border: 1px solid var(--hairline, #1e2638);
                font-family: var(--font-ui, system-ui, sans-serif);
                font-weight: 600;
                text-transform: uppercase;
                transition:
                    border-color 0.15s ease,
                    box-shadow 0.15s ease;
            }

            /* --- Shapes --- */
            .avatar-shape-circle {
                border-radius: 9999px;
            }

            .avatar-shape-square {
                border-radius: var(--radius-md, 10px);
            }

            /* --- Sizes --- */
            .avatar-size-sm {
                width: 32px;
                height: 32px;
                font-size: 11px;
            }

            .avatar-size-default {
                width: var(--touch-target-min, 40px);
                height: var(--touch-target-min, 40px);
                font-size: 14px;
            }

            .avatar-size-lg {
                width: 52px;
                height: 52px;
                font-size: 16px;
            }

            .avatar-size-xl {
                width: 64px;
                height: 64px;
                font-size: 20px;
            }

            /* --- Image & Fallback --- */
            .avatar-image {
                width: 100%;
                height: 100%;
                object-fit: cover;
                aspect-ratio: 1 / 1;
            }

            .avatar-fallback {
                display: flex;
                align-items: center;
                justify-content: center;
                width: 100%;
                height: 100%;
                line-height: 1;
            }

            .avatar-default-icon {
                width: 55%;
                height: 55%;
                opacity: 0.65;
            }

            /* --- Telemetry Status Indicator --- */
            .avatar-status-dot {
                position: absolute;
                bottom: 1px;
                right: 1px;
                border-radius: 50%;
                border: 2px solid var(--surface-card, #111622);
            }

            .avatar-size-sm .avatar-status-dot {
                width: 8px;
                height: 8px;
            }
            .avatar-size-default .avatar-status-dot {
                width: 10px;
                height: 10px;
            }
            .avatar-size-lg .avatar-status-dot {
                width: 12px;
                height: 12px;
            }
            .avatar-size-xl .avatar-status-dot {
                width: 14px;
                height: 14px;
            }

            .avatar-status-online {
                background-color: var(--outcome-yes, #10b981);
                box-shadow: 0 0 6px rgba(16, 185, 129, 0.6);
            }

            .avatar-status-offline {
                background-color: var(--border-strong, #606e85);
            }

            .avatar-status-busy {
                background-color: var(--outcome-no, #fb7185);
                box-shadow: 0 0 6px rgba(251, 113, 133, 0.6);
            }

            .avatar-status-away {
                background-color: var(--status-warning, #fcd34d);
            }
        `
    ]
})
export class AvatarComponent {
    readonly src = input<string>('');
    readonly alt = input<string>('');
    readonly initials = input<string>('');
    readonly size = input<AvatarSize>('default');
    readonly shape = input<AvatarShape>('circle');
    readonly status = input<AvatarStatus | null>(null);

    protected readonly imageFailed = signal(false);

    constructor() {
        effect(() => {
            // When src changes, reset image failure state
            this.src();
            this.imageFailed.set(false);
        });
    }

    protected readonly showImage = computed(() => {
        return !!this.src() && !this.imageFailed();
    });

    protected readonly containerClass = computed(() => {
        return ['avatar', `avatar-size-${this.size()}`, `avatar-shape-${this.shape()}`].join(' ');
    });

    protected readonly statusClass = computed(() => {
        const s = this.status();
        return s ? `avatar-status-dot avatar-status-${s}` : '';
    });

    protected readonly ariaLabelText = computed(() => {
        return this.alt() || (this.initials() ? `${this.initials()} avatar` : 'User avatar');
    });

    protected onImageError(): void {
        this.imageFailed.set(true);
    }

    protected onImageLoad(): void {
        this.imageFailed.set(false);
    }
}
