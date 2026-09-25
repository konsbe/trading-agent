import { CollapsibleCard } from '@trading-agent/shared-components';
import { EarningsCoverage, RawObject } from '@/api';
import { formatCompactNumber, formatDate, formatTime } from '../../utils/format';
import { num, str } from '../../utils/payload';
import { CalendarNewsSectionProps } from './types';
import '@/styles/market-report-global.css';
import './CalendarNewsSection-styles.css';

const COVERAGE_FALLBACK: Record<EarningsCoverage['status'], string> = {
    upcoming: 'Upcoming earnings',
    none_in_window: 'No earnings date in the next 14 days',
    not_ingested: 'Earnings data not available for this symbol',
};

const groupByDate = (events: RawObject[]): [string, RawObject[]][] => {
    const groups = new Map<string, RawObject[]>();
    events.forEach(event => {
        const date = (str(event, 'event_ts') ?? '').slice(0, 10) || 'undated';
        groups.set(date, [...(groups.get(date) ?? []), event]);
    });
    return [...groups.entries()];
};

const figures = (event: RawObject): string => {
    const unit = str(event, 'unit') ?? '';
    return (
        [
            ['actual', 'actual'],
            ['estimate', 'est.'],
            ['previous', 'prev.'],
        ] as const
    )
        .filter(([key]) => num(event, key) !== null)
        .map(([key, label]) => `${label} ${formatCompactNumber(num(event, key))}${unit}`)
        .join(' · ');
};

const EconomicCalendar = ({ events, gapNote }: { events: RawObject[]; gapNote: string | null }) =>
    events.length === 0 ? (
        <div className="market-report-calendar__empty" data-testid="economic-empty">
            <p className="market-report-calendar__empty-title">Economic calendar unavailable</p>
            {gapNote && <p className="market-report-muted market-report-small">{gapNote}</p>}
        </div>
    ) : (
        <div className="market-report-calendar" data-testid="economic-calendar">
            {groupByDate(events).map(([date, dayEvents]) => (
                <section key={date} className="market-report-calendar__day" aria-label={formatDate(date)}>
                    <h4 className="market-report-calendar__date">{formatDate(date)}</h4>
                    <ul className="market-report-calendar__events">
                        {dayEvents.map((event, i) => {
                            const ts = str(event, 'event_ts');
                            const detail = figures(event);
                            return (
                                <li key={`${ts}-${i}`} data-testid="economic-event">
                                    {ts && <span className="market-report-mono market-report-muted">{formatTime(ts)}</span>}{' '}
                                    {str(event, 'country') && <span className="market-report-mono">{str(event, 'country')}</span>}{' '}
                                    <span className="market-report-calendar__name">{str(event, 'event_name') ?? 'Unnamed event'}</span>
                                    {str(event, 'impact') && <span className="market-report-muted"> · {str(event, 'impact')} impact</span>}
                                    {detail && <span className="market-report-muted market-report-small"> · {detail}</span>}
                                </li>
                            );
                        })}
                    </ul>
                </section>
            ))}
        </div>
    );

const earningsDateText = (row: RawObject): string => {
    const period = num(row, 'period');
    const year = num(row, 'year');
    const hour = str(row, 'hour');
    return [
        formatDate(str(row, 'date')),
        period !== null && year !== null ? `Q${period} ${year}` : null,
        hour ? hour.toUpperCase() : null,
    ]
        .filter(Boolean)
        .join(' · ');
};

const Earnings = ({ coverage, calendar, equitySymbols }: { coverage: EarningsCoverage[]; calendar: RawObject[]; equitySymbols: string[] }) => {
    const covered = new Set(coverage.map(c => c.symbol));
    const unreported = equitySymbols.filter(s => !covered.has(s));
    return (
        <ul className="market-report-earnings" data-testid="earnings">
            {coverage.map(entry => {
                const dates = entry.status === 'upcoming' ? calendar.filter(row => str(row, 'symbol') === entry.symbol) : [];
                return (
                    <li key={entry.symbol} data-testid={`earnings-${entry.symbol}`} data-status={entry.status}>
                        <span className="market-report-mono market-report-earnings__symbol">{entry.symbol}</span>
                        {entry.status === 'upcoming' && dates.length > 0 ? (
                            <span>{dates.map(earningsDateText).join('; ')}</span>
                        ) : (
                            <span className={entry.status === 'upcoming' ? '' : 'market-report-muted'}>
                                {entry.note ?? COVERAGE_FALLBACK[entry.status]}
                            </span>
                        )}
                    </li>
                );
            })}
            {unreported.map(symbol => (
                <li key={symbol} data-testid={`earnings-${symbol}`} data-status="unreported">
                    <span className="market-report-mono market-report-earnings__symbol">{symbol}</span>
                    <span className="market-report-muted">Earnings coverage not reported for this symbol</span>
                </li>
            ))}
        </ul>
    );
};

/** Section 4 — economic calendar by date, earnings per equity symbol, macro headlines (links only). */
const CalendarNewsSection = ({ report }: CalendarNewsSectionProps) => {
    const { global } = report;
    const gapNote = report.data_gaps.find(gap => gap.key === 'economic_calendar')?.note ?? null;
    const equitySymbols = report.instruments.filter(i => i.type === 'equity').map(i => i.symbol);

    return (
        <CollapsibleCard id="report-calendar" persistKey="report.calendar" title="Calendar & news" data-testid="calendar-section">
            <div className="market-report-calendar-news">
                <div className="market-report-calendar-news__block">
                    <h3 className="market-report-eyebrow">Economic calendar</h3>
                    <EconomicCalendar events={global.calendars.economic} gapNote={gapNote} />
                </div>

                <div className="market-report-calendar-news__block">
                    <h3 className="market-report-eyebrow">Earnings (next 14 days)</h3>
                    {report.earnings_coverage.length === 0 && equitySymbols.length === 0 ? (
                        <p className="market-report-muted" data-testid="earnings-none">
                            No equity instruments in this report.
                        </p>
                    ) : (
                        <Earnings coverage={report.earnings_coverage} calendar={report.earnings_calendar} equitySymbols={equitySymbols} />
                    )}
                </div>

                <div className="market-report-calendar-news__block">
                    <h3 className="market-report-eyebrow">News</h3>
                    {global.news.length === 0 ? (
                        <p className="market-report-muted" data-testid="news-empty">
                            No macro headlines in this report.
                        </p>
                    ) : (
                        <ul className="market-report-news" data-testid="news">
                            {global.news.map(item => (
                                <li key={item.url}>
                                    <a
                                        className="market-report-news__link"
                                        href={item.url}
                                        target="_blank"
                                        rel="noopener noreferrer"
                                        data-testid="news-link"
                                    >
                                        {item.headline}
                                    </a>
                                    <span className="market-report-muted market-report-small"> · {item.source}</span>
                                </li>
                            ))}
                        </ul>
                    )}
                </div>
            </div>
        </CollapsibleCard>
    );
};

export default CalendarNewsSection;
