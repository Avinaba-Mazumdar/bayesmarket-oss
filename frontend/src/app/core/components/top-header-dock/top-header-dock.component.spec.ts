import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { of } from 'rxjs';
import { TopHeaderDockComponent } from './top-header-dock.component';
import { ThemeService } from '../../services/theme.service';
import { AuthStore } from '../../../state/auth.store';
import { ApiService } from '../../services/api.service';
import { ToastService } from '../../../shared/components/toast/toast.service';

describe('TopHeaderDockComponent', () => {
    let fixture: ComponentFixture<TopHeaderDockComponent>;
    let component: TopHeaderDockComponent;
    let authStore: AuthStore;
    let apiServiceSpy: any;

    beforeEach(async () => {
        apiServiceSpy = {
            createGuestSession: vi.fn().mockReturnValue(
                of({
                    token: 'guest-tok',
                    user: {
                        id: 'u-1',
                        cash_balance: '1000.00',
                        is_guest: true,
                        created_at: '2026-01-01T00:00:00Z'
                    }
                })
            ),
            getCurrentUser: vi.fn().mockReturnValue(
                of({
                    id: 'u-1',
                    cash_balance: '1000.00',
                    is_guest: true,
                    created_at: '2026-01-01T00:00:00Z'
                })
            ),
            getGoogleAuthUrl: vi.fn().mockReturnValue(of({ url: '', simulated: true })),
            claimFaucet: vi.fn().mockReturnValue(
                of({
                    success: true,
                    amount: '100',
                    user: { id: 'u-1', cash_balance: '1100.00' }
                })
            )
        };

        await TestBed.configureTestingModule({
            imports: [TopHeaderDockComponent],
            providers: [provideRouter([]), AuthStore, { provide: ApiService, useValue: apiServiceSpy }, ToastService]
        }).compileComponents();

        fixture = TestBed.createComponent(TopHeaderDockComponent);
        component = fixture.componentInstance;
        authStore = TestBed.inject(AuthStore);
        fixture.detectChanges();
    });

    it('should render brand logo and navigation links when authenticated, without balance badge', () => {
        authStore.token.set('tok-1');
        authStore.user.set({
            id: 'u-1',
            cash_balance: '1000.00',
            is_guest: true,
            auth_provider: 'guest',
            created_at: '2026-01-01T00:00:00Z'
        });
        authStore.cashBalance.set('$1,000.00');
        fixture.detectChanges();

        const el = fixture.nativeElement as HTMLElement;
        expect(el.querySelector('.brand-text')?.textContent).toContain('BayesMarket');
        expect(el.querySelector('.balance-badge')).toBeNull();

        const navTabs = el.querySelectorAll('.nav-tab');
        expect(navTabs.length).toBe(2);
        expect(navTabs[0].textContent?.trim()).toBe('Markets');
        expect(navTabs[1].textContent?.trim()).toBe('Portfolio');
    });

    it('should render faucet button and handle faucet claims when authenticated', () => {
        authStore.token.set('tok-1');
        authStore.user.set({
            id: 'u-1',
            cash_balance: '1000.00',
            is_guest: true,
            auth_provider: 'guest',
            created_at: '2026-01-01T00:00:00Z'
        });
        fixture.detectChanges();

        const el = fixture.nativeElement as HTMLElement;
        const faucetBtn = el.querySelector('app-button[variant="secondary"]') as HTMLElement;
        expect(faucetBtn).toBeTruthy();
        expect(faucetBtn.textContent).toContain('+100 Faucet');

        const claimSpy = vi.spyOn(authStore, 'claimFaucet');
        component.onClaimFaucet();
        expect(claimSpy).toHaveBeenCalled();
    });

    it('should show Sign In button for unauthenticated visitor and open auth modal on click', () => {
        authStore.token.set(null);
        authStore.user.set(null);
        fixture.detectChanges();

        const el = fixture.nativeElement as HTMLElement;
        const signInBtn = el.querySelector('.signin-main-btn') as HTMLElement;
        expect(signInBtn).toBeTruthy();
        expect(signInBtn.textContent).toContain('Sign In');

        const openSpy = vi.spyOn(authStore, 'openAuthModal');
        const buttonNative = signInBtn.querySelector('button') || signInBtn;
        (buttonNative as HTMLElement).click();
        expect(openSpy).toHaveBeenCalled();
    });

    it('should show user profile and sign out button when logged in with Google', () => {
        authStore.token.set('tok-google');
        authStore.user.set({
            id: 'google-1',
            email: 'trader@bayesmarket.com',
            name: 'Alex Mercer',
            avatar_url: null,
            auth_provider: 'google',
            cash_balance: '2500.00',
            is_guest: false,
            created_at: '2026-01-01T00:00:00Z'
        });
        fixture.detectChanges();

        const el = fixture.nativeElement as HTMLElement;
        expect(el.querySelector('.signin-main-btn')).toBeNull();
        expect(el.querySelector('.user-profile-dock')).toBeTruthy();
        expect(el.querySelector('.auth-user-name')?.textContent).toContain('Alex Mercer');

        const profileBtn = el.querySelector('.user-logged-in-btn') as HTMLButtonElement;
        expect(profileBtn).toBeTruthy();
        const openAuthSpy = vi.spyOn(authStore, 'openAuthModal');
        profileBtn.click();
        expect(openAuthSpy).toHaveBeenCalled();
    });

    it('should render theme toggle button and toggle theme when clicked', () => {
        const el = fixture.nativeElement as HTMLElement;
        const themeToggleBtn = el.querySelector('.theme-toggle-btn') as HTMLButtonElement;
        expect(themeToggleBtn).toBeTruthy();

        const themeService = TestBed.inject(ThemeService);
        const initialTheme = themeService.theme();
        expect(initialTheme).toBe('light');

        themeToggleBtn.click();
        expect(themeService.theme()).toBe('dark');

        themeToggleBtn.click();
        expect(themeService.theme()).toBe('light');
    });
});
