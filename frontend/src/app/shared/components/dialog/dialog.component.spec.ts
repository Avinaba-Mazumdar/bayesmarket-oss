import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { DialogComponent, DialogRole, DialogSize } from './dialog.component';

@Component({
    standalone: true,
    imports: [DialogComponent],
    template: `
        <app-dialog
            [(open)]="isOpen"
            [title]="title()"
            [description]="description()"
            [closeOnOverlayClick]="closeOnOverlay()"
            [closeOnEscape]="closeOnEsc()"
            [size]="size()"
            [role]="role()"
            (closed)="onClosed()"
        >
            <button trigger type="button" class="test-trigger-btn">Open Dialog</button>
            <div class="test-dialog-body">Review limit order details before signing.</div>
            <div footer class="test-dialog-footer">
                <button type="button" class="confirm-btn">Confirm Order</button>
            </div>
        </app-dialog>
    `
})
class TestHostComponent {
    readonly isOpen = signal(false);
    readonly title = signal('Confirm Order Execution');
    readonly description = signal('You are placing a binary outcome buy order.');
    readonly closeOnOverlay = signal(true);
    readonly closeOnEsc = signal(true);
    readonly size = signal<DialogSize>('default');
    readonly role = signal<DialogRole>('dialog');
    closedEmitted = false;

    onClosed(): void {
        this.closedEmitted = true;
    }
}

describe('DialogComponent', () => {
    let fixture: ComponentFixture<TestHostComponent>;
    let host: TestHostComponent;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [TestHostComponent]
        }).compileComponents();

        fixture = TestBed.createComponent(TestHostComponent);
        host = fixture.componentInstance;
        fixture.detectChanges();
    });

    it('should not render dialog portal when open is false', () => {
        const portal = fixture.nativeElement.querySelector('.dialog-portal');
        expect(portal).toBeNull();
    });

    it('should open dialog when projected trigger is clicked', () => {
        const trigger = fixture.nativeElement.querySelector('.test-trigger-btn') as HTMLButtonElement;
        trigger.click();
        fixture.detectChanges();

        expect(host.isOpen()).toBe(true);
        const portal = fixture.nativeElement.querySelector('.dialog-portal');
        expect(portal).toBeTruthy();
    });

    it('should render title, description, body, and footer when open is true', () => {
        host.isOpen.set(true);
        fixture.detectChanges();

        const titleEl = fixture.nativeElement.querySelector('.dialog-title');
        const descEl = fixture.nativeElement.querySelector('.dialog-description');
        const bodyEl = fixture.nativeElement.querySelector('.test-dialog-body');
        const footerEl = fixture.nativeElement.querySelector('.test-dialog-footer');

        expect(titleEl?.textContent?.trim()).toBe('Confirm Order Execution');
        expect(descEl?.textContent?.trim()).toBe('You are placing a binary outcome buy order.');
        expect(bodyEl?.textContent?.trim()).toContain('Review limit order details before signing.');
        expect(footerEl?.textContent?.trim()).toContain('Confirm Order');
    });

    it('should close dialog and emit closed when close button is clicked', () => {
        host.isOpen.set(true);
        fixture.detectChanges();

        const closeBtnWrapper = fixture.nativeElement.querySelector('.dialog-close-btn') as HTMLElement;
        expect(closeBtnWrapper).toBeTruthy();

        const closeBtn = (closeBtnWrapper.querySelector('button') ?? closeBtnWrapper) as HTMLButtonElement;
        closeBtn.click();
        fixture.detectChanges();

        expect(host.isOpen()).toBe(false);
        expect(host.closedEmitted).toBe(true);
        expect(fixture.nativeElement.querySelector('.dialog-portal')).toBeNull();
    });

    it('should close dialog when overlay is clicked and closeOnOverlayClick is true', () => {
        host.isOpen.set(true);
        fixture.detectChanges();

        const overlay = fixture.nativeElement.querySelector('.dialog-overlay') as HTMLDivElement;
        overlay.click();
        fixture.detectChanges();

        expect(host.isOpen()).toBe(false);
    });

    it('should NOT close dialog on overlay click when closeOnOverlayClick is false', () => {
        host.isOpen.set(true);
        host.closeOnOverlay.set(false);
        fixture.detectChanges();

        const overlay = fixture.nativeElement.querySelector('.dialog-overlay') as HTMLDivElement;
        overlay.click();
        fixture.detectChanges();

        expect(host.isOpen()).toBe(true);
    });

    it('should close on Escape key press when open and closeOnEscape is true', () => {
        host.isOpen.set(true);
        fixture.detectChanges();

        window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
        fixture.detectChanges();

        expect(host.isOpen()).toBe(false);
    });

    it('should support role="alertdialog" for critical order review dialogs', () => {
        host.isOpen.set(true);
        host.role.set('alertdialog');
        fixture.detectChanges();

        const content = fixture.nativeElement.querySelector('.dialog-content');
        expect(content.getAttribute('role')).toBe('alertdialog');
    });

    it('should dynamically apply size classes', () => {
        host.isOpen.set(true);
        const sizes: DialogSize[] = ['sm', 'lg', 'xl', 'default'];

        for (const s of sizes) {
            host.size.set(s);
            fixture.detectChanges();
            const content = fixture.nativeElement.querySelector('.dialog-content');
            expect(content.classList.contains(`dialog-size-${s}`)).toBe(true);
        }
    });

    it('should NOT set inert on #app-main-content if mainContent contains the dialog', () => {
        const mockMain = document.createElement('main');
        mockMain.id = 'app-main-content';
        document.body.appendChild(mockMain);
        mockMain.appendChild(fixture.nativeElement);

        try {
            host.isOpen.set(true);
            fixture.detectChanges();

            expect(mockMain.hasAttribute('inert')).toBe(false);
        } finally {
            mockMain.remove();
        }
    });

    it('should set inert on #app-main-content if mainContent does NOT contain the dialog', () => {
        const mockMain = document.createElement('main');
        mockMain.id = 'app-main-content';
        document.body.appendChild(mockMain);

        try {
            host.isOpen.set(true);
            fixture.detectChanges();

            expect(mockMain.hasAttribute('inert')).toBe(true);
            host.isOpen.set(false);
            fixture.detectChanges();
            expect(mockMain.hasAttribute('inert')).toBe(false);
        } finally {
            mockMain.remove();
        }
    });
});
