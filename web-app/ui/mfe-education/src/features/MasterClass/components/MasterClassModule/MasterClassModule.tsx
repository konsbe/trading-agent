import MasterClassEntry from '../MasterClassEntry';
import { MasterClassModuleProps } from './types';
import '@/styles/education-document.css';

export const moduleLabel = (number?: number): string | undefined => (number === undefined ? undefined : `Module ${number}`);

/** One MasterClass module: its title (with "Module N" above it when numbered) and its entries. */
const MasterClassModule = ({ module }: MasterClassModuleProps) => {
    const label = moduleLabel(module.number);
    const labelId = `${module.id}-label`;
    const titleId = `${module.id}-title`;
    return (
        <section
            id={module.id}
            tabIndex={-1}
            className="education-group"
            aria-labelledby={label ? `${labelId} ${titleId}` : titleId}
            data-testid="masterclass-module"
        >
            <div>
                {label && (
                    <p id={labelId} className="education-group__eyebrow">
                        {label}
                    </p>
                )}
                <h2 id={titleId} className="education-group__title">
                    {module.title}
                </h2>
            </div>
            {module.entries.map(entry => (
                <MasterClassEntry key={entry.id} entry={entry} />
            ))}
        </section>
    );
};

export default MasterClassModule;
