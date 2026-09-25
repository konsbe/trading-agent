import { RouterProvider } from "react-router-dom";
import { getAppRouter } from "../router/AppRouter";


function App() {

    return (
        <RouterProvider router={getAppRouter()} />
    );
}


export default App
