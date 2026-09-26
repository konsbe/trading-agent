import { renderHook, waitFor } from '@testing-library/react';
import { fetchGlossary, fetchHandbook, fetchMasterClass } from '@/api';
import { glossaryFixture, handbookFixture, masterClassFixture } from '@/test-utils/fixtures';
import { useGlossary, useHandbook, useMasterClass } from './useEducationContent';

jest.mock('@/api', () => ({
    ...jest.requireActual('@/api'),
    fetchHandbook: jest.fn(),
    fetchMasterClass: jest.fn(),
    fetchGlossary: jest.fn(),
}));

describe.each([
    ['useHandbook', useHandbook, fetchHandbook as jest.Mock, handbookFixture()],
    ['useMasterClass', useMasterClass, fetchMasterClass as jest.Mock, masterClassFixture()],
    ['useGlossary', useGlossary, fetchGlossary as jest.Mock, glossaryFixture()],
])('%s', (_name, useHook, fetchMock, fixture) => {
    it('loads its content with an abort signal', async () => {
        fetchMock.mockResolvedValue(fixture);
        const { result } = renderHook(() => useHook());

        await waitFor(() => expect(result.current.data).toEqual(fixture));
        expect(fetchMock).toHaveBeenCalledWith({ signal: expect.any(AbortSignal) });
    });
});
