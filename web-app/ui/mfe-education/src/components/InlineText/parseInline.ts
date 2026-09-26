/**
 * The only markup Education content may use: `**bold**` and `*italic*`
 * (`shared/content/README.md`). Markers must hug their text (`**x**`, not
 * `** x **`), so a stray or arithmetic `*` stays literal. Everything else —
 * HTML-looking text included — is literal text.
 */
export type InlineNode =
    | { kind: 'text'; text: string }
    | { kind: 'italic'; text: string }
    | { kind: 'bold'; children: InlineNode[] };

const BOLD_OR_ITALIC = /\*\*(?!\s)(.+?)(?<!\s)\*\*|\*(?!\s)([^*]+?)(?<!\s)\*/g;
const ITALIC = /\*(?!\s)([^*]+?)(?<!\s)\*/g;

const tokenize = (text: string, pattern: RegExp, allowBold: boolean): InlineNode[] => {
    const nodes: InlineNode[] = [];
    let last = 0;
    for (const match of text.matchAll(pattern)) {
        const index = match.index ?? 0;
        if (index > last) nodes.push({ kind: 'text', text: text.slice(last, index) });
        const [, bold, italic] = allowBold ? match : [match[0], undefined, match[1]];
        if (bold !== undefined) {
            nodes.push({ kind: 'bold', children: tokenize(bold, ITALIC, false) });
        } else {
            nodes.push({ kind: 'italic', text: italic as string });
        }
        last = index + match[0].length;
    }
    if (last < text.length) nodes.push({ kind: 'text', text: text.slice(last) });
    return nodes;
};

export const parseInline = (text: string): InlineNode[] => tokenize(text, BOLD_OR_ITALIC, true);
