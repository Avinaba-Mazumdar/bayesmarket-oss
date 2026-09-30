import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute, Router } from '@angular/router';
import { of, throwError } from 'rxjs';
import { AuthCallbackComponent } from './auth-callback.component';
import { AuthStore } from '../../state/auth.store';
import { ApiService } from '../../core/services/api.service';
import { ToastService } from '../../shared/components/toast/toast.service';

describe('AuthCallbackComponent', () => {
    let fixture: ComponentFixture<AuthCallbackComponent>;
    let component: AuthCallbackComponent;
    let routerSpy: { navigate: any };
    let toastServiceSpy: { success: any; error: any; warning: any; info: any };
    let apiServiceSpy: { callbackGoogleAuth: any };

    const mockStorage: Record<string, string> = {};
    const sessionStorageMock = {
        getItem: (key: string) => mockStorage[key] ?? null,
        setItem: (key: string, val: string) => {
            mockStorage[key] = val;
        },
        removeItem: (key: string) => {
            delete mockStorage[key];
        },
        clear: () => {
            Object.keys(mockStorage).forEach((k) => delete mockStorage[k]);
        }
    };

    beforeEach(() => {
        (globalThis as any).sessionStorage = sessionStorageMock;
        sessionStorageMock.clear();

        routerSpy = { navigate: vi.fn() };
        toastServiceSpy = {
            success: vi.fn(),
            error: vi.fn(),
            warning: vi.fn(),
            info: vi.fn()
        };
        apiServiceSpy = {
            callbackGoogleAuth: vi.fn()
        };
    });

    afterEach(() => {
        sessionStorageMock.clear();
    });

    async function setupComponent(queryParams: Record<string, string>) {
        await TestBed.configureTestingModule({
            imports: [AuthCallbackComponent],
            providers: [
                AuthStore,
                { provide: Router, useValue: routerSpy },
                { provide: ToastService, useValue: toastServiceSpy },
                { provide: ApiService, useValue: apiServiceSpy },
                {
                    provide: ActivatedRoute,
                    useValue: {
                        snapshot: {
                            queryParamMap: {
                                get: (key: string) => queryParams[key] || null
                            }
                        }
                    }
                }
            ]
        }).compileComponents();

        fixture = TestBed.createComponent(AuthCallbackComponent);
        component = fixture.componentInstance;
    }

    it('should handle OAuth redirect error and show message with return action', async () => {
        await setupComponent({ error: 'access_denied', error_description: 'User denied authorization' });
        fixture.detectChanges();

        expect(toastServiceSpy.error).toHaveBeenCalledWith('Sign-in Cancelled', 'User denied authorization');
        expect(component.errorMessage()).toContain('User denied authorization');
        component.navigateHome();
        expect(routerSpy.navigate).toHaveBeenCalledWith(['/']);
    });

    it('should handle missing authorization code and show error with return action', async () => {
        await setupComponent({});
        fixture.detectChanges();

        expect(toastServiceSpy.error).toHaveBeenCalledWith('Authentication Error', 'No authorization code received');
        expect(component.errorMessage()).toContain('Missing authorization code');
        component.navigateHome();
        expect(routerSpy.navigate).toHaveBeenCalledWith(['/']);
    });

    it('should reject when state does not match sessionStorage (CSRF protection)', async () => {
        sessionStorageMock.setItem('bayesmarket_oauth_state', 'expected-state-123');
        await setupComponent({ code: 'auth-code-xyz', state: 'tampered-state-999' });
        fixture.detectChanges();

        expect(toastServiceSpy.error).toHaveBeenCalledWith('Security Verification Failed', expect.stringContaining('OAuth state verification failed'));
        expect(apiServiceSpy.callbackGoogleAuth).not.toHaveBeenCalled();
        expect(sessionStorageMock.getItem('bayesmarket_oauth_state')).toBeNull();
    });

    it('should reject when saved state is missing in browser session (prevent account takeover)', async () => {
        // No saved state in sessionStorage (user did not initiate OAuth in this session)
        await setupComponent({ code: 'attacker-auth-code', state: 'attacker-state' });
        fixture.detectChanges();

        expect(toastServiceSpy.error).toHaveBeenCalledWith('Security Verification Failed', expect.stringContaining('OAuth state verification failed'));
        expect(apiServiceSpy.callbackGoogleAuth).not.toHaveBeenCalled();
        expect(component.errorMessage()).toContain('missing or invalid OAuth state parameter');
    });

    it('should exchange authorization code and state and navigate home on success', async () => {
        sessionStorageMock.setItem('bayesmarket_oauth_state', 'valid-state-abc');
        apiServiceSpy.callbackGoogleAuth.mockReturnValue(
            of({
                token: 'jwt-token-123',
                user: { id: 'u-1', name: 'Trader', auth_provider: 'google', cash_balance: '1000.00' }
            })
        );

        await setupComponent({ code: 'valid-code-456', state: 'valid-state-abc' });
        fixture.detectChanges();

        expect(apiServiceSpy.callbackGoogleAuth).toHaveBeenCalledWith({ code: 'valid-code-456', state: 'valid-state-abc' }, null);
        expect(routerSpy.navigate).toHaveBeenCalledWith(['/']);
    });

    it('should display error message when login exchange fails', async () => {
        sessionStorageMock.setItem('bayesmarket_oauth_state', 'valid-state-abc');
        apiServiceSpy.callbackGoogleAuth.mockReturnValue(throwError(() => ({ error: { message: 'Invalid authorization code' } })));

        await setupComponent({ code: 'invalid-code', state: 'valid-state-abc' });
        fixture.detectChanges();

        expect(component.errorMessage()).toBe('Invalid authorization code');
    });
});
