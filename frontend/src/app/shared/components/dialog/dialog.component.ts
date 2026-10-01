import { ChangeDetectionStrategy, Component, ElementRef, HostListener, computed, effect, inject, input, model, output, viewChild } from '@angular/core';
import { LucideX } from '@lucide/angular';
import { ButtonComponent } from '../button/button.component';

export type DialogSize = 'sm' | 'default' | 'lg' | 'xl';
export type DialogRole = 'dialog' | 'alertdialog';

@Component({
    selector: 'app-dialog',
    standalone: true,
    changeDetection: ChangeDetectionStrategy.OnPush,
    imports: [LucideX, ButtonComponent],
    template: `
        <!-- Optional Projected Trigger Button -->
        <span class="dialog-trigger-wrapper" (click)="openDialog()">
            <ng-content select="[trigger]" />
        </span>

        @if (open()) {
            <div class="dialog-portal" role="presentation">
                <!-- Overlay Backdrop -->
                <div class="dialog-overlay" (click)="onOverlayClick()" aria-hidden="true"></div>

                <!-- Modal Window Container -->
                <div class="dialog-positioner">
                    <div
                        #dialogContent
                        [class]="contentClass()"
                        [attr.role]="role()"
                        [attr.aria-modal]="true"
                        [attr.aria-labelledby]="title() ? dialogId + '-title' : null"
                        [attr.aria-describedby]="description() ? dialogId + '-desc' : null"
                        [attr.aria-label]="!title() ? ariaLabel() || null : null"
                        tabindex="-1"
                    >
                        <!-- Close Button (WCAG AAA min 40x40px touch target) -->
                        @if (showCloseButton()) {
                            <app-button class="dialog-close-btn" variant="ghost" size="icon" ariaLabel="Close dialog" (btnClick)="close()">
                                <svg lucideX class="close-icon" [size]="18" aria-hidden="true"></svg>
                            </app-button>
                        }

                        <!-- Header -->
                        <div class="dialog-header">
                            <ng-content select="[header]" />
                            @if (title()) {
                                <h2 [id]="dialogId + '-title'" class="dialog-title">
                                    {{ title() }}
                                </h2>
                            }
                            @if (description()) {
                                <p [id]="dialogId + '-desc'" class="dialog-description">
                                    {{ description() }}
                                </p>
                            }
                        </div>

                        <!-- Body Content -->
                        <div class="dialog-body">
                            <ng-content />
                        </div>

                        <!-- Footer Actions -->
                        <div class="dialog-footer">
                            <ng-content select="[footer]" />
                        </div>
                    </div>
                </div>
            </div>
        }
    `,
    styles: [
        `
            :host {
                display: contents;
            }

            .dialog-trigger-wrapper {
                display: contents;
            }

            /* --- Fixed Portal Container --- */
            .dialog-portal {
                position: fixed;
                inset: 0;
                z-index: 2000;
                display: flex;
                align-items: center;
                justify-content: center;
                padding: 16px;
                box-sizing: border-box;
                overflow-x: hidden;
                overflow-y: auto;
                contain: paint layout;
            }

            /* --- Backdrop Overlay --- */
            .dialog-overlay {
                position: fixed;
                inset: 0;
                background-color: rgba(6, 5, 14, 0.82);
                backdrop-filter: blur(4px);
                -webkit-backdrop-filter: blur(4px);
                will-change: opacity;
                transform: translateZ(0);
                animation: overlayFadeIn 0.14s cubic-bezier(0.16, 1, 0.3, 1) forwards;
            }

            @keyframes overlayFadeIn {
                from {
                    opacity: 0;
                }
                to {
                    opacity: 1;
                }
            }

            /* --- Positioner --- */
            .dialog-positioner {
                position: relative;
                z-index: 2001;
                width: 100%;
                display: flex;
                justify-content: center;
                pointer-events: none;
                margin: auto;
                will-change: transform, opacity;
                transform: translateZ(0);
            }

            /* --- Content Modal Box --- */
            .dialog-content {
                pointer-events: auto;
                position: relative;
                width: 100%;
                max-height: calc(100vh - 48px);
                overflow-y: auto;
                background-color: var(--surface-card, #131126);
                border: 1px solid var(--border-strong, #3d3766);
                border-radius: var(--radius-xl, 20px);
                padding: 24px;
                box-shadow:
                    var(--shadow-terminal, 0 8px 32px rgba(0, 0, 0, 0.7)),
                    0 0 0 1px rgba(255, 255, 255, 0.08);
                box-sizing: border-box;
                outline: none;
                will-change: transform, opacity;
                transform: translateZ(0);
                contain: layout;
                animation: dialogZoomIn 0.15s cubic-bezier(0.16, 1, 0.3, 1) forwards;
            }

            @keyframes dialogZoomIn {
                from {
                    transform: translate3d(0, 6px, 0) scale(0.98);
                    opacity: 0;
                }
                to {
                    transform: translate3d(0, 0, 0) scale(1);
                    opacity: 1;
                }
            }

            @media (prefers-reduced-motion: reduce) {
                .dialog-overlay,
                .dialog-content {
                    animation: none !important;
                }
            }

            /* --- Sizes --- */
            .dialog-size-sm {
                max-width: 400px;
            }

            .dialog-size-default {
                max-width: 520px;
            }

            .dialog-size-lg {
                max-width: 680px;
            }

            .dialog-size-xl {
                max-width: 860px;
            }

            /* --- Close Button --- */
            .dialog-close-btn {
                position: absolute;
                top: 14px;
                right: 14px;
                z-index: 10;
            }

            .close-icon {
                width: 18px;
                height: 18px;
            }

            /* --- Header --- */
            .dialog-header {
                display: flex;
                flex-direction: column;
                gap: 6px;
                margin-bottom: 20px;
                padding-right: 36px;
            }

            .dialog-title {
                margin: 0;
                font-family: var(--font-ui, system-ui, sans-serif);
                font-size: 19px;
                font-weight: 700;
                line-height: 1.35;
                color: var(--ink, #f8fafc);
                letter-spacing: -0.2px;
            }

            .dialog-description {
                margin: 0;
                font-family: var(--font-ui, system-ui, sans-serif);
                font-size: 14px;
                line-height: 1.5;
                color: var(--muted, #a2b4c9);
            }

            /* --- Body --- */
            .dialog-body {
                margin-bottom: 24px;
            }

            /* --- Footer --- */
            .dialog-footer {
                display: flex;
                align-items: center;
                justify-content: flex-end;
                gap: 12px;
            }

            .dialog-footer:empty {
                display: none;
            }

            @media (max-width: 640px) {
                .dialog-portal {
                    padding: 0;
                    align-items: flex-end;
                }

                .dialog-content {
                    border-bottom-left-radius: 0;
                    border-bottom-right-radius: 0;
                    max-height: 85vh;
                }

                .dialog-footer {
                    flex-direction: column-reverse;
                    width: 100%;
                }
            }
        `
    ]
})
export class DialogComponent {
    private static idSeq = 0;
    protected readonly dialogId = `bayes-dialog-${++DialogComponent.idSeq}`;

    readonly open = model<boolean>(false);
    readonly title = input<string>('');
    readonly description = input<string>('');
    readonly closeOnOverlayClick = input<boolean>(true);
    readonly closeOnEscape = input<boolean>(true);
    readonly showCloseButton = input<boolean>(true);
    readonly size = input<DialogSize>('default');
    readonly role = input<DialogRole>('dialog');
    readonly ariaLabel = input<string>('');

    readonly closed = output<void>();

    private readonly hostEl = inject(ElementRef<HTMLElement>);
    protected readonly dialogContentEl = viewChild<ElementRef<HTMLDivElement>>('dialogContent');
    private previouslyFocusedElement: HTMLElement | null = null;
    private inertElements: HTMLElement[] = [];

    protected readonly contentClass = computed(() => {
        return ['dialog-content', `dialog-size-${this.size()}`].join(' ');
    });

    constructor() {
        // Effect to manage inert background and focus lifecycle
        effect(() => {
            const isOpen = this.open();
            if (isOpen) {
                this.onDialogOpen();
            } else {
                this.onDialogClose();
            }
        });
    }

    private onDialogOpen(): void {
        if (typeof document === 'undefined') return;

        this.previouslyFocusedElement = document.activeElement as HTMLElement | null;

        // Apply inert to #app-main-content ONLY if the dialog is outside of it (e.g. root modals)
        // If the dialog is rendered inside #app-main-content, setting inert on mainContent would
        // render the dialog itself completely inert and unclickable!
        const mainContent = document.getElementById('app-main-content');
        if (mainContent && !mainContent.contains(this.hostEl.nativeElement) && !mainContent.hasAttribute('inert')) {
            mainContent.setAttribute('inert', '');
            this.inertElements.push(mainContent);
        }

        // Apply inert to header dock when dialog is open
        const topHeader = document.querySelector('app-top-header-dock');
        if (topHeader && !topHeader.contains(this.hostEl.nativeElement) && !topHeader.hasAttribute('inert')) {
            topHeader.setAttribute('inert', '');
            this.inertElements.push(topHeader as HTMLElement);
        }

        // Delay slightly for DOM render, then focus dialog or first focusable
        setTimeout(() => {
            const content = this.dialogContentEl()?.nativeElement;
            if (content) {
                const focusables = this.getFocusableElements(content);
                if (focusables.length > 0) {
                    focusables[0].focus({ preventScroll: true });
                } else {
                    content.focus({ preventScroll: true });
                }
            }
        }, 16);
    }

    private onDialogClose(): void {
        // Remove inert from background elements
        for (const el of this.inertElements) {
            el.removeAttribute('inert');
        }
        this.inertElements = [];

        // Restore focus
        if (this.previouslyFocusedElement && typeof this.previouslyFocusedElement.focus === 'function') {
            try {
                this.previouslyFocusedElement.focus({ preventScroll: true });
            } catch {
                // Element might have unmounted
            }
            this.previouslyFocusedElement = null;
        }
    }

    @HostListener('window:keydown.escape', ['$event'])
    onEscapeKey(event: Event): void {
        if (this.open() && this.closeOnEscape()) {
            event.preventDefault();
            this.close();
        }
    }

    @HostListener('window:keydown', ['$event'])
    onKeydown(event: KeyboardEvent): void {
        if (!this.open() || event.key !== 'Tab') return;

        const content = this.dialogContentEl()?.nativeElement;
        if (!content) return;

        const focusables = this.getFocusableElements(content);
        if (focusables.length === 0) {
            event.preventDefault();
            return;
        }

        const first = focusables[0];
        const last = focusables[focusables.length - 1];

        if (event.shiftKey) {
            if (document.activeElement === first || !content.contains(document.activeElement)) {
                event.preventDefault();
                last.focus();
            }
        } else {
            if (document.activeElement === last || !content.contains(document.activeElement)) {
                event.preventDefault();
                first.focus();
            }
        }
    }

    private getFocusableElements(container: HTMLElement): HTMLElement[] {
        const selector =
            'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';
        return Array.from(container.querySelectorAll<HTMLElement>(selector)).filter((el) => el.offsetParent !== null || el.getClientRects().length > 0);
    }

    openDialog(): void {
        this.open.set(true);
    }

    close(): void {
        this.open.set(false);
        this.closed.emit();
    }

    onOverlayClick(): void {
        if (this.closeOnOverlayClick()) {
            this.close();
        }
    }
}
