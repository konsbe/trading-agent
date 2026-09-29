import React from 'react';
import { Link } from 'react-router-dom';
import { ArrowLeftIcon } from '../../icons';
import { PageHeaderBack, PageHeaderIconBack, PageHeaderProps } from './types';
import '../Button/Button-styles.css';
import './PageHeader-styles.css';

const BackAction = ({ label, to, onClick }: PageHeaderBack) =>
    to !== undefined ? (
        <Link className="ta-page-header__back-link" to={to} onClick={onClick} data-testid="page-header-back">
            {label}
        </Link>
    ) : (
        <button type="button" className="ta-page-header__back-link" onClick={onClick} data-testid="page-header-back">
            {label}
        </button>
    );

const ICON_BACK_CLASS = 'ta-button ta-button--ghost ta-button--sm ta-button--icon-only ta-page-header__back-icon';

const BackIcon = ({ label, to, onClick }: PageHeaderIconBack) =>
    to !== undefined ? (
        <Link className={ICON_BACK_CLASS} to={to} onClick={onClick} aria-label={label} title={label} data-testid="page-header-back">
            <ArrowLeftIcon size={18} />
        </Link>
    ) : (
        <button type="button" className={ICON_BACK_CLASS} onClick={onClick} aria-label={label} title={label} data-testid="page-header-back">
            <ArrowLeftIcon size={18} />
        </button>
    );

/**
 * Page title block: a back action (a text link above the title, or an arrow
 * left of it), an h1 with badges beside it, a subtitle line below, and
 * actions on the right. `variant="bar"` is the page's top header bar.
 * Presentational only.
 */
const PageHeader = ({
    title,
    subtitle,
    back,
    badges = [],
    actions,
    variant = 'page',
    className = '',
    'data-testid': testId = 'page-header',
}: PageHeaderProps) => (
    <header className={`ta-page-header ta-page-header--${variant} ${className}`.trim()} data-testid={testId}>
        {back && !back.iconOnly && (
            <nav className="ta-page-header__back" aria-label="Breadcrumb">
                <BackAction {...back} />
            </nav>
        )}
        <div className="ta-page-header__row">
            {back?.iconOnly && (
                <nav className="ta-page-header__back-nav" aria-label="Breadcrumb">
                    <BackIcon {...back} />
                </nav>
            )}
            <div className="ta-page-header__heading">
                <div className="ta-page-header__title-row">
                    <h1 className="ta-page-header__title">{title}</h1>
                    {badges.length > 0 && (
                        <ul className="ta-page-header__badges" aria-label="Details">
                            {badges.map((badge, index) => (
                                <li key={index} className="ta-page-header__badge">
                                    {badge}
                                </li>
                            ))}
                        </ul>
                    )}
                </div>
                {subtitle && <div className="ta-page-header__subtitle">{subtitle}</div>}
            </div>
            {actions && <div className="ta-page-header__actions">{actions}</div>}
        </div>
    </header>
);

export default PageHeader;
