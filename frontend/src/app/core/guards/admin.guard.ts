import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { AuthStore } from '../../state/auth.store';
import { toObservable } from '@angular/core/rxjs-interop';
import { filter, map, take } from 'rxjs';

/**
 * Functional route guard preventing non-admins from accessing /admin.
 * Redirects unauthorized visitors directly to '/' without showing any gatekeeper card.
 */
export const adminGuard: CanActivateFn = () => {
    const authStore = inject(AuthStore);
    const router = inject(Router);

    // If session is still initializing from backend token check, wait until initialization finishes
    if (authStore.isInitializing()) {
        return toObservable(authStore.isInitializing).pipe(
            filter((initializing) => !initializing),
            take(1),
            map(() => {
                if (authStore.isAdmin()) {
                    return true;
                }
                return router.createUrlTree(['/']);
            })
        );
    }

    if (authStore.isAdmin()) {
        return true;
    }

    return router.createUrlTree(['/']);
};
