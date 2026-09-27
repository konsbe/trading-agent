import PageLayout from '@/components/PageLayout';
import AlarmHistoryScreen from '@/features/AlarmHistory';

const AlarmHistoryPage = () => (
    <PageLayout
        title="Alarm History"
        subtitle="Alerts the analyst bot posted to Discord, newest first, in your local time."
    >
        <AlarmHistoryScreen />
    </PageLayout>
);

export default AlarmHistoryPage;
