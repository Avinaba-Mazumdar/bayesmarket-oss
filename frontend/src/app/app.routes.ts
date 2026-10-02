import { Routes } from '@angular/router';
import { adminGuard } from './core/guards/admin.guard';

export const routes: Routes = [
    {
        path: '',
        loadComponent: () => import('./features/market/market-list.component').then((m) => m.MarketListComponent),
        title: 'BayesMarket — Institutional Binary Prediction Market Exchange'
    },
    {
        path: 'markets/:id',
        loadComponent: () => import('./features/market/market-detail.component').then((m) => m.MarketDetailComponent),
        title: 'Trading Cockpit — BayesMarket'
    },
    {
        path: 'portfolio',
        loadComponent: () => import('./features/portfolio/portfolio-view.component').then((m) => m.PortfolioViewComponent),
        title: 'Portfolio & PnL Ledger — BayesMarket'
    },
    {
        path: 'admin',
        loadComponent: () => import('./features/admin/admin-dashboard.component').then((m) => m.AdminDashboardComponent),
        canActivate: [adminGuard],
        title: 'Admin Liquidity & Market Creator — BayesMarket'
    },
    {
        path: 'auth/callback',
        loadComponent: () => import('./features/auth/auth-callback.component').then((m) => m.AuthCallbackComponent),
        title: 'Authenticating — BayesMarket'
    },
    {
        path: '**',
        redirectTo: ''
    }
];
