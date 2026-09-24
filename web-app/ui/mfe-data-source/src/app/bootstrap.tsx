import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import '@trading-agent/shared-components/theme.css';
import AppRouter from '@/router/AppRouter';
import { AppWrapper } from './wrapper';

/** Standalone dev mode (http://localhost:3004): own router, OS theme, no shell. */
export const StandaloneApp = () => (
    <BrowserRouter>
        <AppWrapper>
            <AppRouter />
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
