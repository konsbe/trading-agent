import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { DataSourceStatus } from '@/api';
import useDataSourceStatus from '@/hooks/dataSources/useDataSourceStatus';
import { HostModeProvider } from '@/providers/HostModeContext';
import { makeEveryStatusBody } from '@/test-utils/fixtures';
import DataSourcePage from './DataSourcePage';

jest.mock('@/hooks/dataSources/useDataSourceStatus', () => ({ __esModule: true, default: jest.fn() }));

const hookMock = useDataSourceStatus as jest.MockedFunction<typeof useDataSourceStatus>;

const Location = () => {
    const { pathname, search } = useLocation();
    return <span data-testid="location">{`${pathname}${search}`}</span>;
};

const renderPage = (url = '/data-source') => {
    hookMock.mockReturnValue({
        status: makeEveryStatusBody() as DataSourceStatus,
        error: null,
        isLoading: false,
        isRefreshing: false,
        refresh: jest.fn(),
    });
    render(
        <MemoryRouter initialEntries={[url]}>
            <HostModeProvider hosted>
                <Location />
                <DataSourcePage />
            </HostModeProvider>
        </MemoryRouter>
    );
};

const sessions = () =>
    within(screen.getByTestId('chain-table'))
        .getAllByTestId(/^chain-row-/)
        .map(r => r.getAttribute('data-testid')!.replace('chain-row-2026-09-', ''));
const header = (name: string) => within(screen.getByTestId('chain-table')).getByRole('button', { name });

beforeEach(() => window.sessionStorage.clear());

describe('Daily chain table view', () => {
    it('defaults to the newest session first', () => {
        renderPage();
        expect(sessions()).toEqual(['24', '23', '22', '21', '20', '19']);
        expect(screen.getByRole('columnheader', { name: 'Session' })).toHaveAttribute('aria-sort', 'descending');
    });

    it('sorts Status most concerning first, then reversed', async () => {
        renderPage();

        await userEvent.click(header('Status'));
        expect(sessions()).toEqual(['20', '21', '22', '23', '24', '19']);
        expect(screen.getByTestId('location')).toHaveTextContent('/data-source?chain_sort=status%3Aasc');
        await userEvent.click(header('Status'));
        expect(sessions()).toEqual(['19', '24', '23', '22', '21', '20']);
    });

    it('keeps "not computed" coverage last both ways; equal values stay newest first', async () => {
        renderPage();

        await userEvent.click(header('Coverage now'));
        expect(sessions()).toEqual(['24', '23', '21', '19', '22', '20']);
        await userEvent.click(header('Coverage now'));
        expect(sessions()).toEqual(['22', '24', '23', '21', '19', '20']);
    });

    it('searches the text shown, including a row’s detail line', async () => {
        renderPage();

        await userEvent.type(screen.getByTestId('chain-search-input'), 'gave up');
        await waitFor(() => expect(sessions()).toEqual(['21']));
        expect(screen.getByTestId('chain-search-count')).toHaveTextContent('1 of 6 rows');
    });

    it('restores sort and search from the URL', () => {
        renderPage('/data-source?chain_sort=attempts%3Adesc&chain_q=not%20done');
        expect(sessions()).toEqual(['21', '22', '20', '19']);
        expect(screen.getByTestId('chain-search-input')).toHaveValue('not done');
    });
});
