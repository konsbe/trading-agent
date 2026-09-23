// Import global styles
//  CSS imports don't create JS module instances, 
// so they have no effect on the Module Federation sharing mechanism. 
// They can safely import them first before the main app code.
import './styles/global-flex.css';
import './styles/global-styles.css';

// Initialize application with dynamic MFE loading
// Async boundary — lets MF shared scope settle before React runs
// it protects against the eager - loading problem — 
// it ensures webpack's shared scope negotiation completes before any React/JS code runs.
import ('./app');
