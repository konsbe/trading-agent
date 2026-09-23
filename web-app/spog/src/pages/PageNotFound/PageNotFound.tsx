import styles from './PageNotFound.module.css';

type PageNotFoundProps = {
    statusCode: number;
    message: string;
}

const PageNotFound = ({ statusCode, message }: PageNotFoundProps) => (

    <div className={styles.container}>
        <div className={styles.mfeContainer}>
            <h1 className={styles.heading404}>
                {statusCode}
            </h1>
            <p className={styles.textNotFound}>
                {message}
            </p>
        </div>
    </div >


);

export default PageNotFound;
