import { Navigate, Route, Routes } from 'react-router-dom';
import GlossaryPage from '@/pages/GlossaryPage';
import HandbookPage from '@/pages/HandbookPage';
import MasterClassPage from '@/pages/MasterClassPage';

/**
 * Standalone only (http://localhost:3008). Hosted, spog mounts each exposed root
 * directly under its own `/handbook`, `/masterclass` or `/glossary` route.
 */
const AppRouter = () => (
    <Routes>
        <Route index element={<Navigate to="handbook" replace />} />
        <Route path="handbook/*" element={<HandbookPage />} />
        <Route path="masterclass/*" element={<MasterClassPage />} />
        <Route path="glossary/*" element={<GlossaryPage />} />
    </Routes>
);

export default AppRouter;
