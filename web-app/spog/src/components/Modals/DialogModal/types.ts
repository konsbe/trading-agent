import { ButtonVariant } from '@trading-agent/shared-components';

export interface DialogModalAction {
    content: string;
    onSubmit: () => void | Promise<void>;
    variant?: ButtonVariant;
}

export interface DialogModalProps {
    isOpen: boolean;
    dialogTitle: string;
    dialogBody: string;
    submitButton: DialogModalAction;
    cancelButton: DialogModalAction;
}
