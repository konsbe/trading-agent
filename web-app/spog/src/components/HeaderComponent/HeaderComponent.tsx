import { useNavigate, useSearchParams } from "react-router-dom";
import { ComponentType, SVGProps, useEffect, useRef } from "react";
import { ArrowLeftIcon, Button } from "@trading-agent/shared-components";
import { useBrowserHistory } from "../../providers/BrowserHistory";
import "./HeaderComponent-styles.css";

// Validate that a path is a safe internal route
const isSafeInternalPath = (path: string) => {
    return typeof path === "string" && path.startsWith("/") && !path.startsWith("//");
}
interface PageHeaderProps {
    title: string;
    icon?: ComponentType<SVGProps<SVGSVGElement>>;
    enableNavigation?: boolean;
    navigationPath: string; // Optional prop for navigation path, if needed in the future
    showNavigator?: boolean; // Optional prop to control the visibility of the navigator
}
/**
 * 
 * @params enableNavigation with navigationPath are used together to determine if the back navigation
 *  should be shown and where it should navigate to. If enableNavigation is true and there are search parameters in the URL, 
 * the component will show a back arrow and use the navigationPath for navigation when the arrow is clicked. 
 * @params showNavigator is used to control the visibility of the navigator independently of the enableNavigation prop. 
 * This allows for scenarios where you want to show the navigator without enabling the back navigation functionality, 
 * and to handle the title via props.
 *
 */
const HeaderComponent = ({ title, icon: Icon, enableNavigation, navigationPath, showNavigator }: PageHeaderProps) => {
    const [searchParams] = useSearchParams();
    const navigate = useNavigate();
    const { historyStack, popPath } = useBrowserHistory();

    const hasNavigatedRef = useRef(false);

    // Extract the first search parameter value and replace underscores with spaces for the header title
    const searchParamsObject = Object.fromEntries(searchParams) ?? {};
    const navigationPathValue = Object.values(searchParamsObject)[0];
    // Show navigation if enableNavigation is true and there are search parameters in the URL
    // Note: URLSearchParams.size is not available in all environments (e.g. JSDOM), so we check
    // the derived navigationPathValue instead, which already reads from the params object.
    const showNavigation = !!(enableNavigation && navigationPathValue);

    // Decode the navigationPathValue to handle any URL-encoded characters 
    const decodedValue = decodeURIComponent(navigationPathValue || "");
    // Sanitize the decoded value to prevent XSS
    // Utility to escape HTML special characters
    const escapeHtml = (str: string) => {
        return str.replace(/[&<>'"]/g, (tag) => {
            const chars: { [key: string]: string } = {
                '&': '&amp;',
                '<': '&lt;',
                '>': '&gt;',
                "'": '&#39;',
                '"': '&quot;'
            };
            return chars[tag] || tag;
        });
    }

    const safeDecodedValue = escapeHtml(decodedValue);
    // Check for special characters in case of vulnerability
    const hasSpecialChars = /[^a-zA-Z0-9 _-]/.test(decodedValue);

    const headerTitle = showNavigation ? safeDecodedValue : title;

    const handleBackHistory = () => {
        // This will pop the current path 
        // and navigate to the previous one in the custom stack.
        if (historyStack.length > 1) {
            popPath();
            const prevPath = historyStack[historyStack.length - 2];
            if (isSafeInternalPath(prevPath)) {
                navigate(prevPath);
            } else {
                navigate(navigationPath); // fallback to a safe default
            }
        } else {
            navigate(navigationPath);
        }
    };

    useEffect(() => {
        // ref to track if navigation already occurred to prevent the risk of infinite loop
        if (hasSpecialChars && !hasNavigatedRef.current) {
            hasNavigatedRef.current = true;
            navigate(navigationPath);
        }
    }, [hasSpecialChars, navigationPath, navigate]);



    return (
        <div className="d-flex-row-start page-header" data-testid="page-header">
            {(showNavigation || showNavigator)
                ? <Button
                    variant="ghost"
                    size="sm"
                    iconOnly
                    aria-label="Go back"
                    onClick={handleBackHistory}
                    data-testid="page-header-back"
                >
                    <ArrowLeftIcon size={16} />
                </Button>
                : Icon && <Icon className="page-header__icon" />}
            <div className="page-header__title">{headerTitle}</div>
        </div>
    );
};

export default HeaderComponent;
