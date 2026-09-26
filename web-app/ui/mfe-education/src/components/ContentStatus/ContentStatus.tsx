import { ContentStatusProps } from './types';
import './ContentStatus-styles.css';

const capitalise = (value: string): string => value.charAt(0).toUpperCase() + value.slice(1);

/** Muted "Draft · version 0.1.0" line above authored content. */
const ContentStatus = ({ status, version }: ContentStatusProps) => (
    <p className="education-status" data-testid="content-status">
        {capitalise(status)} · version <span className="education-status__version">{version}</span>
    </p>
);

export default ContentStatus;
