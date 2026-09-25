import React from 'react';
import { render } from '@testing-library/react';
import '@testing-library/jest-dom';
import {
    AlertTriangleIcon,
    ArrowLeftIcon,
    CheckCircleIcon,
    ChevronDownIcon,
    CircleDashedIcon,
    CloseIcon,
    LockIcon,
    LogoutIcon,
    MenuIcon,
    MinusCircleIcon,
    MonitorIcon,
    MoonIcon,
    SunIcon,
    GridIcon,
    CandlestickIcon,
    FlaskIcon,
    BellIcon,
    BookmarkIcon,
    ListChecksIcon,
    DatabaseIcon,
    ChartBoxIcon,
    EyeIcon,
    SettingsIcon,
    DotIcon,
} from './Icons';

describe('Icons', () => {
    it.each([
        ['MenuIcon', MenuIcon],
        ['ArrowLeftIcon', ArrowLeftIcon],
        ['CloseIcon', CloseIcon],
        ['ChevronDownIcon', ChevronDownIcon],
        ['LogoutIcon', LogoutIcon],
        ['SunIcon', SunIcon],
        ['MoonIcon', MoonIcon],
        ['MonitorIcon', MonitorIcon],
        ['LockIcon', LockIcon],
        ['CheckCircleIcon', CheckCircleIcon],
        ['AlertTriangleIcon', AlertTriangleIcon],
        ['MinusCircleIcon', MinusCircleIcon],
        ['CircleDashedIcon', CircleDashedIcon],
        ['GridIcon', GridIcon],
        ['CandlestickIcon', CandlestickIcon],
        ['FlaskIcon', FlaskIcon],
        ['BellIcon', BellIcon],
        ['BookmarkIcon', BookmarkIcon],
        ['ListChecksIcon', ListChecksIcon],
        ['DatabaseIcon', DatabaseIcon],
        ['ChartBoxIcon', ChartBoxIcon],
        ['EyeIcon', EyeIcon],
        ['SettingsIcon', SettingsIcon],
        ['DotIcon', DotIcon],
    ])('%s renders a decorative 20px svg by default', (displayName, Icon) => {
        const { container } = render(<Icon />);

        const svg = container.querySelector('svg');
        expect(svg).toHaveAttribute('width', '20');
        expect(svg).toHaveAttribute('aria-hidden', 'true');
        expect(Icon.displayName).toBe(displayName);
    });

    it('accepts a size and exposes itself when labelled', () => {
        const { container } = render(<MenuIcon size={32} aria-label="Menu" className="icon" />);

        const svg = container.querySelector('svg');
        expect(svg).toHaveAttribute('width', '32');
        expect(svg).not.toHaveAttribute('aria-hidden');
        expect(svg).toHaveClass('icon');
    });
});
