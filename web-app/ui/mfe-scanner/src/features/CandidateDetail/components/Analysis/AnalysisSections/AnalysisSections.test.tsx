import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ApiError } from '@/api';
import { StockAnalysisResource } from '@/hooks/scanner/useStockAnalysis';
import { COMPUTING_MESSAGE, FAILED_MESSAGE, makeAnalysis, makePending } from '@/test-utils/analysisFixtures';
import AnalysisSections from './AnalysisSections';

const retry = jest.fn();

beforeEach(() => window.sessionStorage.clear());

const renderWith = (state: Omit<StockAnalysisResource, 'retry'>) =>
    render(<AnalysisSections analysis={{ ...state, retry } as StockAnalysisResource} />);

describe('AnalysisSections', () => {
    it('renders the six sections in order when ready', () => {
        renderWith({ status: 'ready', data: makeAnalysis() });

        const ids = within(screen.getByTestId('analysis-sections'))
            .getAllByRole('region')
            .map(region => region.closest('section')?.getAttribute('data-testid'));
        expect(ids).toEqual([
            'analysis-technical',
            'analysis-fundamentals',
            'analysis-balance-sheet',
            'analysis-correlations',
            'analysis-news',
            'analysis-heuristic',
        ]);
    });

    it('puts severity badges only in Classical technical signals', () => {
        renderWith({ status: 'ready', data: makeAnalysis() });

        ['analysis-technical', 'analysis-fundamentals', 'analysis-balance-sheet', 'analysis-correlations', 'analysis-news'].forEach(id =>
            expect(within(screen.getByTestId(id)).queryAllByTestId(/severity/)).toHaveLength(0)
        );
        expect(within(screen.getByTestId('analysis-heuristic')).getAllByTestId(/severity/).length).toBeGreaterThan(0);
    });

    it('shows a loading placeholder', () => {
        renderWith({ status: 'loading' });
        expect(screen.getByRole('status', { name: 'Loading analysis' })).toBeInTheDocument();
    });

    it('shows the API computing message verbatim, not a generic spinner', () => {
        renderWith({ status: 'computing', pending: makePending({ retry_after_ms: 3000 }) });

        const notice = screen.getByTestId('analysis-computing');
        expect(notice).toHaveAttribute('role', 'status');
        expect(notice).toHaveTextContent(COMPUTING_MESSAGE);
        expect(notice).toHaveTextContent('Checking again in 3 s.');
        expect(screen.queryByTestId('analysis-sections')).not.toBeInTheDocument();
    });

    it('shows the failed message with a Retry', async () => {
        renderWith({ status: 'failed', pending: makePending({ status: 'failed', message: FAILED_MESSAGE }) });

        expect(screen.getByTestId('analysis-failed')).toHaveAttribute('role', 'alert');
        expect(screen.getByTestId('analysis-failed-message')).toHaveTextContent(FAILED_MESSAGE);
        await userEvent.click(screen.getByRole('button', { name: 'Retry' }));
        expect(retry).toHaveBeenCalledTimes(1);
    });

    it('shows a plain note for 404 (no daily bars) and an error state with Retry otherwise', async () => {
        const { unmount } = renderWith({ status: 'error', error: new ApiError(404, 'no_data_for_symbol') });
        expect(screen.getByTestId('analysis-no-data')).toHaveTextContent('No analysis for this symbol');
        unmount();

        renderWith({ status: 'error', error: new ApiError(503, 'database_unavailable') });
        expect(screen.getByTestId('api-error-state')).toHaveTextContent("Couldn't load the analysis for this symbol.");
        await userEvent.click(screen.getByRole('button', { name: 'Retry' }));
        expect(retry).toHaveBeenCalled();
    });

    it('renders nothing when idle', () => {
        const { container } = renderWith({ status: 'idle' });
        expect(container).toBeEmptyDOMElement();
    });
});
