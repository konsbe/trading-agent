/** An ASCII hyphen used as a minus: at the start or after a space or "(" and right before a digit. */
export const ASCII_MINUS = /(^|[\s(])-\d/;

/** Every rendered text node (and aria-label) under `root` that shows a number with an ASCII hyphen-minus. */
export const findAsciiMinus = (root: HTMLElement): string[] => {
    const hits: string[] = [];
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    while (walker.nextNode()) {
        const text = walker.currentNode.textContent ?? '';
        if (ASCII_MINUS.test(text)) hits.push(text);
    }
    root.querySelectorAll('[aria-label]').forEach(el => {
        const label = el.getAttribute('aria-label') ?? '';
        if (ASCII_MINUS.test(label)) hits.push(label);
    });
    if (ASCII_MINUS.test(root.textContent ?? '')) hits.push(root.textContent ?? '');
    return hits;
};
