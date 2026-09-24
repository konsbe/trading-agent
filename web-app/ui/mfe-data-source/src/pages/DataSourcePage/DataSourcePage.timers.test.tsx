import { act, render, screen } from '@testing-library/react';
import { fetchDataSourceStatus } from '@/api';
import { makeStatus } from '@/test-utils/fixtures';
import DataSourcePage from './DataSourcePage';

jest.mock('@/api/dataSources/dataSourcesApi', () => ({ fetchDataSourceStatus: jest.fn() }));

const fetchMock = fetchDataSourceStatus as jest.MockedFunction<typeof fetchDataSourceStatus>;

describe('DataSourcePage without timers', () => {
    beforeEach(() => jest.useFakeTimers());
    afterEach(() => jest.useRealTimers());

    it('fetches once on mount and never again on its own; Refresh fetches once more', async () => {
        fetchMock.mockResolvedValue(makeStatus());
        render(<DataSourcePage />);
        await act(async () => undefined);
        expect(screen.getByTestId('status-view')).toBeInTheDocument();

        await act(async () => {
            jest.advanceTimersByTime(10 * 60 * 1000);
        });
        expect(fetchMock).toHaveBeenCalledTimes(1);

        await act(async () => {
            screen.getByRole('button', { name: 'Refresh' }).click();
        });
        await act(async () => {
            jest.advanceTimersByTime(10 * 60 * 1000);
        });
        expect(fetchMock).toHaveBeenCalledTimes(2);
    });
});
