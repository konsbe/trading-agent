import { IconComponent, ThemeMode } from '@trading-agent/shared-components';

export type ThemeOptionId = ThemeMode | 'system';

export interface ThemeOption {
    id: ThemeOptionId;
    label: string;
    Icon: IconComponent;
}
