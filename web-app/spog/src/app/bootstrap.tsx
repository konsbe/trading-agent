import App from '../app/app-root'
import AppWrappers from './wrapper'
import { createRoot } from "react-dom/client";
import "@trading-agent/shared-components/theme.css";


const AppWrapper = () => {
  return (
    <AppWrappers>
      <App />
    </AppWrappers>
  )
}

// Export bootstrap function for dynamic loading
export function bootstrapApp() {
  const container = document.getElementById("root");
  if (!container) {
    throw new Error("Root container missing in index.html");
  }
  const root = createRoot(container);
  
  root.render(<AppWrapper />);

}
