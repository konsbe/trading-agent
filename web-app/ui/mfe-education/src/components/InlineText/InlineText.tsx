import { InlineNode, parseInline } from './parseInline';
import { InlineTextProps } from './types';

const renderNodes = (nodes: InlineNode[]) =>
    nodes.map((node, index) => {
        switch (node.kind) {
            case 'bold':
                return <strong key={index}>{renderNodes(node.children)}</strong>;
            case 'italic':
                return <em key={index}>{node.text}</em>;
            default:
                return node.text;
        }
    });

/** Renders authored text with `**bold**` / `*italic*`; all other text is rendered literally (React-escaped). */
const InlineText = ({ text }: InlineTextProps) => <>{renderNodes(parseInline(text))}</>;

export default InlineText;
