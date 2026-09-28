import { mockResponse } from '@/test-utils/fixtures';
import { makeTracked } from '@/test-utils/tracked';
import { ApiError } from '../fetch-client';
import { fetchTracked } from './trackedApi';

const fetchMock = jest.fn();

beforeEach(() => {
    global.fetch = fetchMock as unknown as typeof fetch;
});

describe('fetchTracked', () => {
    it('requests the given status and parses the body', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, makeTracked()));

        const result = await fetchTracked('all');

        expect(fetchMock).toHaveBeenCalledWith(
            'http://localhost:8090/api/v1/scanner/tracked?status=all',
            expect.objectContaining({ method: 'GET' })
        );
        expect(result.summary).toEqual({ active_count: 2, closed_count: 1 });
    });

    it.each([
        [400, 'invalid_status_param'],
        [500, 'session_calendar_unavailable'],
        [503, 'database_unavailable'],
    ])('surfaces HTTP %i as the API error code %s', async (status, code) => {
        fetchMock.mockResolvedValue(mockResponse(status, { error: code }));

        await expect(fetchTracked('active')).rejects.toEqual(new ApiError(status, code));
    });

    it('reports a body of the wrong shape as invalid_response', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, { tracked: [] }));

        await expect(fetchTracked('all')).rejects.toMatchObject({ code: 'invalid_response', status: 200 });
    });
});
