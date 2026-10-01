import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { LucideCheck, LucideCircleAlert, LucideTriangleAlert, LucideInfo, LucideBell, LucideX } from '@lucide/angular';
import { Toast, ToastService } from './toast.service';

@Component({
    selector: 'app-toast-container',
    standalone: true,
    changeDetection: ChangeDetectionStrategy.OnPush,
    imports: [LucideCheck, LucideCircleAlert, LucideTriangleAlert, LucideInfo, LucideBell, LucideX],
    template: `
        <div class="toast-viewport" role="region" aria-label="Notifications" aria-live="polite">
            @for (t of toastService.toasts(); track t.id) {
                <div [class]="'toast-item toast-' + (t.variant || 'default')" [attr.role]="t.variant === 'destructive' ? 'alert' : 'status'">
                    <div class="toast-indicator" aria-hidden="true"></div>

                    <div class="toast-icon-box" aria-hidden="true">
                        @switch (t.variant) {
                            @case ('success') {
                                <svg lucideCheck class="toast-icon icon-success" [size]="18" [strokeWidth]="2.5" aria-hidden="true"></svg>
                            }
                            @case ('destructive') {
                                <svg lucideCircleAlert class="toast-icon icon-destructive" [size]="18" [strokeWidth]="2.5" aria-hidden="true"></svg>
                            }
                            @case ('warning') {
                                <svg lucideTriangleAlert class="toast-icon icon-warning" [size]="18" [strokeWidth]="2.5" aria-hidden="true"></svg>
                            }
                            @case ('info') {
                                <svg lucideInfo class="toast-icon icon-info" [size]="18" [strokeWidth]="2.5" aria-hidden="true"></svg>
                            }
                            @default {
                                <svg lucideBell class="toast-icon icon-default" [size]="18" [strokeWidth]="2.5" aria-hidden="true"></svg>
                            }
                        }
                    </div>

                    <div class="toast-body">
                        <div class="toast-title">{{ t.title }}</div>
                        @if (t.description) {
                            <div class="toast-description">{{ t.description }}</div>
                        }
                    </div>

                    @if (t.action) {
                        <button type="button" class="toast-action-btn" (click)="onAction(t)">
                            {{ t.action.label }}
                        </button>
                    }

                    <button type="button" class="toast-close-btn" aria-label="Close notification" (click)="toastService.dismiss(t.id)">
                        <svg lucideX class="close-icon" [size]="16" aria-hidden="true"></svg>
                    </button>
                </div>
            }
        </div>
    `,
    styles: [
        `
            :host {
                display: contents;
            }

            .toast-viewport {
                position: fixed;
                top: 76px;
                right: 20px;
                z-index: 9999;
                display: flex;
                flex-direction: column;
                gap: 10px;
                max-width: 420px;
                width: calc(100vw - 40px);
                pointer-events: none;
            }

            .toast-item {
                position: relative;
                pointer-events: auto;
                display: flex;
                align-items: flex-start;
                gap: 12px;
                padding: 14px 16px;
                background-color: var(--surface-glass, rgba(17, 22, 34, 0.95));
                backdrop-filter: blur(16px);
                -webkit-backdrop-filter: blur(16px);
                border: 1px solid var(--hairline, #1e2638);
                border-radius: var(--radius-lg, 14px);
                box-shadow: var(--shadow-lg, 0 12px 28px -4px rgba(0, 0, 0, 0.65));
                overflow: hidden;
                animation: toastIn 0.2s cubic-bezier(0.16, 1, 0.3, 1);
            }

            @keyframes toastIn {
                from {
                    transform: translateY(-8px) scale(0.96);
                    opacity: 0;
                }
                to {
                    transform: translateY(0) scale(1);
                    opacity: 1;
                }
            }

            /* --- Left Indicator Bar --- */
            .toast-indicator {
                position: absolute;
                top: 0;
                left: 0;
                bottom: 0;
                width: 4px;
            }

            .toast-default .toast-indicator {
                background-color: var(--primary, #a6034c);
            }
            .toast-success .toast-indicator {
                background-color: var(--outcome-yes, #10b981);
            }
            .toast-destructive .toast-indicator {
                background-color: var(--status-loss-border, #fb7185);
            }
            .toast-warning .toast-indicator {
                background-color: var(--status-warning, #fcd34d);
            }
            .toast-info .toast-indicator {
                background-color: var(--status-info, #7dd3fc);
            }

            /* --- Icons --- */
            .toast-icon-box {
                flex-shrink: 0;
                margin-top: 1px;
            }

            .toast-icon {
                width: 18px;
                height: 18px;
            }

            .icon-default {
                color: var(--primary-border, #e84089);
            }
            .icon-success {
                color: var(--outcome-yes-text, #34d399);
            }
            .icon-destructive {
                color: var(--status-loss, #fda4af);
            }
            .icon-warning {
                color: var(--status-warning, #fcd34d);
            }
            .icon-info {
                color: var(--status-info, #7dd3fc);
            }

            /* --- Content --- */
            .toast-body {
                flex: 1;
                min-width: 0;
                display: flex;
                flex-direction: column;
                gap: 3px;
            }

            .toast-title {
                font-family: var(--font-ui, system-ui, sans-serif);
                font-size: 14px;
                font-weight: 600;
                line-height: 1.3;
                color: var(--ink, #f8fafc);
            }

            .toast-description {
                font-family: var(--font-ui, system-ui, sans-serif);
                font-size: 13px;
                line-height: 1.4;
                color: var(--muted, #a2b4c9);
            }

            /* --- Action Button --- */
            .toast-action-btn {
                flex-shrink: 0;
                align-self: center;
                min-height: 32px;
                padding: 6px 12px;
                background-color: var(--surface-card-elevated, #171f30);
                color: var(--ink, #f8fafc);
                border: 1px solid var(--border-strong, #606e85);
                border-radius: var(--radius-md, 10px);
                font-family: var(--font-ui, system-ui, sans-serif);
                font-size: 12px;
                font-weight: 600;
                cursor: pointer;
                transition: background-color 0.15s ease;
            }
            .toast-action-btn:hover {
                background-color: var(--surface-card, #111622);
            }

            /* --- Close Button (WCAG 40x40px touch target) --- */
            .toast-close-btn {
                flex-shrink: 0;
                display: inline-flex;
                align-items: center;
                justify-content: center;
                min-width: var(--touch-target-min, 40px);
                min-height: var(--touch-target-min, 40px);
                margin: -10px -10px -10px 0;
                background: transparent;
                border: none;
                color: var(--muted, #a2b4c9);
                cursor: pointer;
                border-radius: var(--radius-sm, 6px);
                transition: color 0.15s ease;
            }

            .toast-close-btn:hover {
                color: var(--ink, #f8fafc);
            }

            .close-icon {
                width: 16px;
                height: 16px;
            }
        `
    ]
})
export class ToastContainerComponent {
    readonly toastService = inject(ToastService);

    onAction(toast: Toast): void {
        if (toast.action) {
            toast.action.onClick();
            this.toastService.dismiss(toast.id);
        }
    }
}
