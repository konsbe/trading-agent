import { render } from '@testing-library/react';
import InlineText from './InlineText';
import { parseInline } from './parseInline';

const renderInline = (text: string) => render(<p data-testid="p"><InlineText text={text} /></p>).getByTestId('p');

describe('parseInline', () => {
    it.each([
        ['plain text', [{ kind: 'text', text: 'plain text' }]],
        ['a **bold** word', [{ kind: 'text', text: 'a ' }, { kind: 'bold', children: [{ kind: 'text', text: 'bold' }] }, { kind: 'text', text: ' word' }]],
        ['an *italic* word', [{ kind: 'text', text: 'an ' }, { kind: 'italic', text: 'italic' }, { kind: 'text', text: ' word' }]],
        ['**a *b* c**', [{ kind: 'bold', children: [{ kind: 'text', text: 'a ' }, { kind: 'italic', text: 'b' }, { kind: 'text', text: ' c' }] }]],
        ['*x:* y', [{ kind: 'italic', text: 'x:' }, { kind: 'text', text: ' y' }]],
    ])('parses %j', (text, nodes) => {
        expect(parseInline(text)).toEqual(nodes);
    });

    it.each(['3 * 4 * 5', 'a ** b ** c', 'unclosed *italic', 'unclosed **bold', '*', '**', 'BUY_WATCH / TRIM_WATCH', ''])(
        'leaves %j literal',
        text => {
            expect(parseInline(text).every(node => node.kind === 'text')).toBe(true);
            expect(parseInline(text).map(node => (node.kind === 'text' ? node.text : '')).join('')).toBe(text);
        }
    );
});

describe('InlineText', () => {
    it('renders bold as <strong> and italic as <em>', () => {
        const p = renderInline('Keep **this** and *that*.');

        expect(p).toHaveTextContent('Keep this and that.');
        expect(p.querySelector('strong')).toHaveTextContent('this');
        expect(p.querySelector('em')).toHaveTextContent('that');
    });

    it('renders italic nested in bold', () => {
        const p = renderInline('**bold with *italic* inside**');

        expect(p.querySelector('strong em')).toHaveTextContent('italic');
        expect(p.querySelector('strong')).toHaveTextContent('bold with italic inside');
    });

    it('renders HTML-looking text literally, never as markup', () => {
        const text = '<b>not bold</b> <script>alert(1)</script> <a href="x">link</a> & [md](link) `code` # h';
        const p = renderInline(text);

        expect(p.textContent).toBe(text);
        expect(p.querySelector('b, script, a, code, h1')).toBeNull();
        expect(p.children).toHaveLength(0);
    });

    it('keeps markers literal when they do not hug text', () => {
        const p = renderInline('RS = gain * 2 * loss');

        expect(p.textContent).toBe('RS = gain * 2 * loss');
        expect(p.querySelector('em')).toBeNull();
    });
});
