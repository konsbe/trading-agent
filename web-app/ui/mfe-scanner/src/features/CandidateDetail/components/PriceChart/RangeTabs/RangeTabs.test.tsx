import { useState } from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BarsRange } from '@/api';
import RangeTabs from './RangeTabs';

const Controlled = ({ onChange }: { onChange?: (r: BarsRange) => void }) => {
    const [value, setValue] = useState<BarsRange>('1M');
    return (
        <RangeTabs
            value={value}
            label="VGZ chart range"
            onChange={r => {
                setValue(r);
                onChange?.(r);
            }}
        />
    );
};

const radio = (name: string) => screen.getByRole('radio', { name });

describe('RangeTabs', () => {
    it('renders the six ranges as a labelled radio group with one tab stop', () => {
        render(<Controlled />);

        expect(screen.getByRole('radiogroup', { name: 'VGZ chart range' })).toBeInTheDocument();
        expect(screen.getAllByRole('radio').map(r => r.textContent)).toEqual(['1D', '5D', '1M', '6M', '1Y', 'ALL']);
        expect(radio('1M')).toHaveAttribute('aria-checked', 'true');
        expect(radio('1M')).toHaveAttribute('tabindex', '0');
        expect(radio('1D')).toHaveAttribute('aria-checked', 'false');
        expect(radio('1D')).toHaveAttribute('tabindex', '-1');
    });

    it('selects on click', async () => {
        const onChange = jest.fn();
        render(<Controlled onChange={onChange} />);

        await userEvent.click(radio('1Y'));

        expect(onChange).toHaveBeenCalledWith('1Y');
        expect(radio('1Y')).toHaveAttribute('aria-checked', 'true');
    });

    it('moves the selection and focus with arrows, Home and End (wrapping)', async () => {
        const onChange = jest.fn();
        render(<Controlled onChange={onChange} />);

        await userEvent.tab();
        expect(radio('1M')).toHaveFocus();

        await userEvent.keyboard('{ArrowRight}');
        expect(radio('6M')).toHaveAttribute('aria-checked', 'true');
        expect(radio('6M')).toHaveFocus();

        await userEvent.keyboard('{ArrowLeft}{ArrowLeft}');
        expect(radio('5D')).toHaveFocus();

        await userEvent.keyboard('{End}');
        expect(radio('ALL')).toHaveAttribute('aria-checked', 'true');

        await userEvent.keyboard('{ArrowRight}');
        expect(radio('1D')).toHaveAttribute('aria-checked', 'true');

        await userEvent.keyboard('{Home}{ArrowUp}');
        expect(radio('ALL')).toHaveFocus();

        await userEvent.keyboard('a');
        expect(onChange).toHaveBeenLastCalledWith('ALL');
    });
});
