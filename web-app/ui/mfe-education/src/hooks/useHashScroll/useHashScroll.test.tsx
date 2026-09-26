import { act, render } from '@testing-library/react';
import { MemoryRouter, useNavigate } from 'react-router-dom';
import useHashScroll from './useHashScroll';

let navigate: ReturnType<typeof useNavigate>;

const Harness = ({ ready }: { ready: boolean }) => {
    useHashScroll(ready);
    navigate = useNavigate();
    return (
        <>
            <article id="rsi" tabIndex={-1}>RSI</article>
            <article id="p/e ratio" tabIndex={-1}>P/E</article>
        </>
    );
};

const renderAt = (path: string, ready = true) =>
    render(
        <MemoryRouter initialEntries={[path]}>
            <Harness ready={ready} />
        </MemoryRouter>
    );

describe('useHashScroll', () => {
    const scrollIntoView = jest.fn();

    beforeEach(() => {
        scrollIntoView.mockReset();
        Element.prototype.scrollIntoView = scrollIntoView;
    });

    it('scrolls to and focuses the hash target on arrival', () => {
        const { container } = renderAt('/masterclass#rsi');

        expect(scrollIntoView).toHaveBeenCalledTimes(1);
        expect(scrollIntoView.mock.contexts[0]).toBe(container.querySelector('#rsi'));
        expect(document.activeElement).toBe(container.querySelector('#rsi'));
    });

    it('decodes the hash', () => {
        const { container } = renderAt('/masterclass#p%2Fe%20ratio');
        expect(document.activeElement).toBe(container.querySelector('[id="p/e ratio"]'));
    });

    it('waits until the content is ready', () => {
        const { rerender } = renderAt('/masterclass#rsi', false);
        expect(scrollIntoView).not.toHaveBeenCalled();

        rerender(
            <MemoryRouter initialEntries={['/masterclass#rsi']}>
                <Harness ready />
            </MemoryRouter>
        );
        expect(scrollIntoView).toHaveBeenCalledTimes(1);
    });

    it('does nothing without a hash or a matching element', () => {
        renderAt('/masterclass');
        renderAt('/masterclass#missing');
        expect(scrollIntoView).not.toHaveBeenCalled();
    });

    it('scrolls again when the same hash is navigated to twice', () => {
        renderAt('/masterclass');

        act(() => navigate({ hash: '#rsi' }));
        act(() => navigate({ hash: '#rsi' }));

        expect(scrollIntoView).toHaveBeenCalledTimes(2);
    });
});
