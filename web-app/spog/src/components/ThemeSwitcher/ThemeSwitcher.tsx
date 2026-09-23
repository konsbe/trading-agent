import { MonitorIcon, MoonIcon, SunIcon } from '@trading-agent/shared-components';
import { useThemeProvider } from '../../providers/ThemeProvider';
import { ThemeOption, ThemeOptionId } from './types';
import './ThemeSwitcher-styles.css';

const THEME_OPTIONS: ThemeOption[] = [
    { id: 'light', label: 'Light', Icon: SunIcon },
    { id: 'dark', label: 'Dark', Icon: MoonIcon },
    { id: 'system', label: 'System', Icon: MonitorIcon },
];

const ThemeSwitcher = () => {
    const { theme, isSystemMode, switchThemeMode, switchToSystemTheme } = useThemeProvider();
    const selected: ThemeOptionId = isSystemMode ? 'system' : theme;

    const select = (id: ThemeOptionId) => {
        if (id === 'system') switchToSystemTheme();
        else switchThemeMode(id);
    };

    return (
        <div role="radiogroup" aria-label="Theme" className="theme-switcher" data-testid="theme-switcher">
            {THEME_OPTIONS.map(({ id, label, Icon }) => {
                const isSelected = selected === id;
                return (
                    <button
                        key={id}
                        type="button"
                        role="radio"
                        aria-checked={isSelected}
                        className={`theme-switcher__option ${isSelected ? 'theme-switcher__option--selected' : ''}`.trim()}
                        onClick={() => select(id)}
                    >
                        <Icon size={14} />
                        {label}
                    </button>
                );
            })}
        </div>
    );
};

export default ThemeSwitcher;
