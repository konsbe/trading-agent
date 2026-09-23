import { type ReactNode } from "react";
import styles from "./Alert.module.css";

type SeverityType = 'info' | 'warning' | 'error';

type AlertProps = {
    severity?: SeverityType;
    children: ReactNode;
};

const isSeverity = (value: any): value is SeverityType => {
    return ['info', 'warning', 'error'].includes(value);
}

export default function Alert({ severity = "info", children }: AlertProps) {
    const safeSeverity = isSeverity(severity) ? severity : 'info';
    const severityClass = styles[safeSeverity];

    return (
        <div className={`${styles.alert} ${severityClass}`} role="alert">
            <span>{children}</span>
        </div>
    );
}