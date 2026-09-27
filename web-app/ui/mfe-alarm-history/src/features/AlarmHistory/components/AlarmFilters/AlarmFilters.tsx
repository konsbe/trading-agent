import { FormEvent, useCallback, useMemo } from 'react';
import { Button, SEVERITY_LEVELS } from '@trading-agent/shared-components';
import { DEFAULT_RANGE_DAYS } from '@/common/dates/localDays';
import { useTypeLabel } from '../../providers/TypeLabelsContext';
import FilterChip from '../FilterChip';
import { AlarmFiltersProps } from './types';
import './AlarmFilters-styles.css';

/**
 * Symbol, alert type (options: every type that has fired), severity and a
 * local-day date range. Type and severity are multi-select chips; none
 * selected means all.
 */
const AlarmFilters = ({ filters, typeOptions }: AlarmFiltersProps) => {
    const {
        symbolInput,
        setSymbolInput,
        applySymbol,
        alertTypes,
        toggleAlertType,
        severities,
        toggleSeverity,
        range,
        setFrom,
        setTo,
        resetRange,
        isDefaultRange,
    } = filters;

    const typeLabel = useTypeLabel();
    const types = useMemo(
        () => [...new Set([...typeOptions, ...alertTypes])].sort((a, b) => typeLabel(a).localeCompare(typeLabel(b))),
        [typeOptions, alertTypes, typeLabel]
    );

    const handleSubmit = useCallback(
        (event: FormEvent<HTMLFormElement>) => {
            event.preventDefault();
            applySymbol();
        },
        [applySymbol]
    );

    return (
        <form className="alarm-filters" role="search" aria-label="Filter alerts" onSubmit={handleSubmit} data-testid="alarm-filters">
            <div className="alarm-filters__field">
                <label className="alarm-filters__label" htmlFor="alarm-filter-symbol">
                    Symbol
                </label>
                <input
                    id="alarm-filter-symbol"
                    className="alarm-filters__input alarm-filters__input--symbol"
                    type="search"
                    value={symbolInput}
                    placeholder="Any symbol"
                    autoComplete="off"
                    spellCheck={false}
                    onChange={event => setSymbolInput(event.target.value)}
                    data-testid="filter-symbol"
                />
            </div>

            <fieldset className="alarm-filters__group" data-testid="filter-types">
                <legend className="alarm-filters__label">Type</legend>
                <div className="alarm-filters__chips">
                    {types.length === 0 ? (
                        <span className="alarm-filters__hint">No alert types recorded yet</span>
                    ) : (
                        types.map(type => (
                            <FilterChip
                                key={type}
                                label={typeLabel(type)}
                                pressed={alertTypes.includes(type)}
                                onToggle={() => toggleAlertType(type)}
                                data-testid={`filter-type-${type}`}
                            />
                        ))
                    )}
                </div>
            </fieldset>

            <fieldset className="alarm-filters__group" data-testid="filter-severities">
                <legend className="alarm-filters__label">Severity</legend>
                <div className="alarm-filters__chips">
                    {SEVERITY_LEVELS.map(severity => (
                        <FilterChip
                            key={severity}
                            label={severity}
                            pressed={severities.includes(severity)}
                            onToggle={() => toggleSeverity(severity)}
                            data-testid={`filter-severity-${severity}`}
                        />
                    ))}
                </div>
            </fieldset>

            <fieldset className="alarm-filters__group" data-testid="filter-range">
                <legend className="alarm-filters__label">Date range (local days)</legend>
                <div className="alarm-filters__dates">
                    <label className="alarm-filters__date">
                        <span className="alarm-filters__date-label">From</span>
                        <input
                            className="alarm-filters__input"
                            type="date"
                            value={range.from}
                            max={range.to || undefined}
                            onChange={event => setFrom(event.target.value)}
                            data-testid="filter-from"
                        />
                    </label>
                    <label className="alarm-filters__date">
                        <span className="alarm-filters__date-label">To</span>
                        <input
                            className="alarm-filters__input"
                            type="date"
                            value={range.to}
                            min={range.from || undefined}
                            onChange={event => setTo(event.target.value)}
                            data-testid="filter-to"
                        />
                    </label>
                    {isDefaultRange ? (
                        <span className="alarm-filters__hint" data-testid="range-default-note">
                            The last {DEFAULT_RANGE_DAYS} days, today included (default)
                        </span>
                    ) : (
                        <Button variant="ghost" size="sm" onClick={resetRange} data-testid="range-reset">
                            Last {DEFAULT_RANGE_DAYS} days
                        </Button>
                    )}
                </div>
            </fieldset>
        </form>
    );
};

export default AlarmFilters;
