import { CandlestickIcon, DotIcon, GridIcon } from '@trading-agent/shared-components';
import { FALLBACK_NAV_ICON, NAV_ICONS, getNavIcon } from './navIcons';
import { PLACEHOLDER_ROUTES } from '@common/navigation';
import appConfig from '../../../public/config.json';

describe('navIcons', () => {
    it('resolves known MDI names', () => {
        expect(getNavIcon('mdi-view-grid-outline')).toBe(GridIcon);
        expect(getNavIcon('mdi-finance')).toBe(CandlestickIcon);
    });

    it.each([undefined, '', 'mdi-unknown', 'toString', '__proto__'])(
        'falls back for %p',
        name => {
            expect(getNavIcon(name)).toBe(FALLBACK_NAV_ICON);
            expect(FALLBACK_NAV_ICON).toBe(DotIcon);
        }
    );

    it('registers every icon used by config.json and the placeholders', () => {
        const names = [
            ...Object.values((appConfig as AppConfig).mfes).map(entry => entry.nav_icon),
            ...PLACEHOLDER_ROUTES.map(route => route.icon),
        ];

        names.forEach(name => expect(Object.keys(NAV_ICONS)).toContain(name));
    });
});
