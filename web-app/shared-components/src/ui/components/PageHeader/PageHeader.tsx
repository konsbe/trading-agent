import React from 'react';
import { Link } from 'react-router-dom';
import { PageHeaderBack, PageHeaderProps } from './types';
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

/**
 * Page title block: optional back action above an h1, badges beside the title,
 * a subtitle line below, and actions on the right. Presentational only.
 */
const PageHeader = ({ title, subtitle, back, badges = [], actions, className = '', 'data-testid': testId = 'page-header' }: PageHeaderProps) => (
    <header className={`ta-page-header ${className}`.trim()} data-testid={testId}>
        {back && (
            <nav className="ta-page-header__back" aria-label="Breadcrumb">
                <BackAction {...back} />
            </nav>
        )}
        <div className="ta-page-header__row">
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
