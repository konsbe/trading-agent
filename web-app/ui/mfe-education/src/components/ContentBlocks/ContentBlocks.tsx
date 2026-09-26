import CaveatCallout from '@/components/CaveatCallout';
import InlineText from '@/components/InlineText';
import { ContentBlock } from '@/api';
import { ContentBlocksProps } from './types';
import './ContentBlocks-styles.css';

const renderBlock = (block: ContentBlock, index: number, Heading: 'h3' | 'h4' | 'h5') => {
    switch (block.type) {
        case 'paragraph':
            return (
                <p key={index} className="education-block education-block--paragraph">
                    <InlineText text={block.text} />
                </p>
            );
        case 'heading':
            return (
                <Heading key={index} className="education-block education-block--heading">
                    <InlineText text={block.text} />
                </Heading>
            );
        case 'list':
            return (
                <ul key={index} className="education-block education-block--list">
                    {block.items.map((item, itemIndex) => (
                        <li key={itemIndex}>
                            <InlineText text={item} />
                        </li>
                    ))}
                </ul>
            );
        case 'caveat':
            return <CaveatCallout key={index} caveatKey={block.key} text={block.text} />;
    }
};

/** An entry's `blocks` in order: paragraph, heading, list, and (Handbook) caveat. */
const ContentBlocks = ({ blocks, headingLevel = 4 }: ContentBlocksProps) => {
    const Heading = `h${headingLevel}` as 'h3' | 'h4' | 'h5';
    return <div className="education-blocks">{blocks.map((block, index) => renderBlock(block, index, Heading))}</div>;
};

export default ContentBlocks;
