export interface CaveatCalloutProps {
    /** Key into `momentum_caveats.json`; picks the title. */
    caveatKey: string;
    /** The caveat text as served, rendered verbatim. */
    text: string;
}
