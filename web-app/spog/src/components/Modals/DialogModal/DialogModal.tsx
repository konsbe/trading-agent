import { Button, Dialog } from '@trading-agent/shared-components';
import { DialogModalProps } from './types';
import './DialogModal-styles.css';

const DialogModal = ({ isOpen, dialogTitle, dialogBody, submitButton, cancelButton }: DialogModalProps) => (
    <Dialog
        isOpen={isOpen}
        title={dialogTitle}
        closeOnOverlayClick={false}
        data-testid="dialog-modal"
        footer={
            <>
                <Button variant={cancelButton.variant ?? 'secondary'} onClick={cancelButton.onSubmit}>
                    {cancelButton.content}
                </Button>
                <Button variant={submitButton.variant ?? 'primary'} onClick={submitButton.onSubmit} data-autofocus>
                    {submitButton.content}
                </Button>
            </>
        }
    >
        <p className="dialog-modal__body">{dialogBody}</p>
    </Dialog>
);

export default DialogModal;
