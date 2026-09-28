import { createRoot } from 'react-dom/client';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import { STOCK_DETAIL_BASE_PATH } from '@trading-agent/shared-components';
import '@trading-agent/shared-components/theme.css';
import AppRouter from '@/router/AppRouter';
import { AppWrapper } from './wrapper';

/**
 * Standalone dev mode (http://localhost:3001): own router, OS theme, no shell.
 * Mounted at spog's `/candidates/*` too, so absolute Stock Detail links and
 * "Back to …" URLs behave the same as hosted.
 */
export const StandaloneApp = () => (
    <BrowserRouter>
        <AppWrapper>
            <Routes>
                <Route path={`${STOCK_DETAIL_BASE_PATH}/*`} element={<AppRouter />} />
                <Route path="*" element={<Navigate to={STOCK_DETAIL_BASE_PATH} replace />} />
            </Routes>
        </AppWrapper>
    </BrowserRouter>
);

export const mount = (container: HTMLElement | null = document.getElementById('root')) => {
    if (!container) {
        throw new Error('Root container missing in index.html');
    }
    const root = createRoot(container);
    root.render(<StandaloneApp />);
    return root;
};

if (process.env.NODE_ENV !== 'test') {
    mount();
}
