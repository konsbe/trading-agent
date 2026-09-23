import { useCallback, useEffect, useId, useRef, useState } from 'react';
import { Button, ChevronDownIcon, LogoutIcon } from '@trading-agent/shared-components';
import ThemeSwitcher from '../ThemeSwitcher';
import { UserMenuProps } from './types';
import './UserMenu-styles.css';

const getInitials = (userName: string) =>
    userName
        .split(/\s+/)
        .filter(Boolean)
        .slice(0, 2)
        .map(part => part[0]?.toUpperCase())
        .join('') || '?';

const UserMenu = ({ userName, userRoles, onSignOut }: UserMenuProps) => {
    const [isOpen, setIsOpen] = useState(false);
    const containerRef = useRef<HTMLDivElement>(null);
    const menuId = useId();

    const close = useCallback(() => setIsOpen(false), []);

    useEffect(() => {
        if (!isOpen) return;
        const handlePointerDown = (event: MouseEvent) => {
            if (!containerRef.current?.contains(event.target as Node)) close();
        };
        const handleKeyDown = (event: KeyboardEvent) => {
            if (event.key === 'Escape') close();
        };
        document.addEventListener('mousedown', handlePointerDown);
        document.addEventListener('keydown', handleKeyDown);
        return () => {
            document.removeEventListener('mousedown', handlePointerDown);
            document.removeEventListener('keydown', handleKeyDown);
        };
    }, [isOpen, close]);

    const handleSignOut = useCallback(() => {
        close();
        onSignOut();
    }, [close, onSignOut]);

    return (
        <div className="user-menu" ref={containerRef}>
            <Button
                variant="ghost"
                className="user-menu__trigger"
                aria-haspopup="menu"
                aria-expanded={isOpen}
                aria-controls={isOpen ? menuId : undefined}
                onClick={() => setIsOpen(open => !open)}
                data-testid="user-menu-trigger"
            >
                <span className="user-menu__avatar" aria-hidden="true">{getInitials(userName)}</span>
                <span className="user-menu__name">{userName}</span>
                <ChevronDownIcon size={16} />
            </Button>
            {isOpen && (
                <div id={menuId} role="menu" className="user-menu__dropdown" data-testid="user-menu-dropdown">
                    <div className="user-menu__section">
                        <div className="user-menu__label">Roles</div>
                        <div className="user-menu__roles">{userRoles.length ? userRoles.join(', ') : 'N/A'}</div>
                    </div>
                    <div className="user-menu__section">
                        <div className="user-menu__label">Theme</div>
                        <ThemeSwitcher />
                    </div>
                    <Button role="menuitem" variant="ghost" fullWidth className="user-menu__item" onClick={handleSignOut}>
                        <LogoutIcon size={16} />
                        Sign out
                    </Button>
                </div>
            )}
        </div>
    );
};

export default UserMenu;
