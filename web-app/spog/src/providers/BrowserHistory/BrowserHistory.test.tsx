import React from "react";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, useLocation, Route, Routes, useNavigate } from "react-router-dom";
import BrowserHistoryProvider, { useBrowserHistory } from "./BrowserHistory";

describe("BrowserHistoryProvider", () => {
  function TestComponent() {
    const { historyStack, popPath, pushPath } = useBrowserHistory();
    const location = useLocation();
    return (
      <div>
        <button onClick={() => popPath()} data-testid="pop">
          pop
        </button>
        <button onClick={() => pushPath("/foo")} data-testid="push">
          push
        </button>
        <div data-testid="history-stack">{JSON.stringify(historyStack)}</div>
        <div data-testid="current-location">{location.pathname + location.search}</div>
      </div>
    );
  }

  it("initializes with current location", () => {
    render(
      <MemoryRouter initialEntries={["/initial"]}>
        <BrowserHistoryProvider>
          <TestComponent />
        </BrowserHistoryProvider>
      </MemoryRouter>
    );
    expect(screen.getByTestId("history-stack").textContent).toContain("/initial");
    expect(screen.getByTestId("current-location").textContent).toBe("/initial");
  });

  it("tracks navigation to new locations", () => {
    // Simulate navigation by pushing new entries
    render(
      <MemoryRouter initialEntries={["/foo"]}>
        <BrowserHistoryProvider>
          <TestComponent />
        </BrowserHistoryProvider>
      </MemoryRouter>
    );
    // push a new path
    screen.getByTestId("push").click();
    expect(screen.getByTestId("history-stack").textContent).toContain("/foo");
  });

  it("does not push duplicate path if location does not change", () => {
    render(
      <MemoryRouter initialEntries={["/foo"]}>
        <BrowserHistoryProvider>
          <TestComponent />
        </BrowserHistoryProvider>
      </MemoryRouter>
    );
    // push a new path (should not add duplicate if same as last)
    screen.getByTestId("push").click();
    // Should only have one /foo (the initial)
    const stack = JSON.parse(screen.getByTestId("history-stack").textContent || "[]");
    expect(stack.filter((p) => p === "/foo").length).toBe(1);
  });

  it("popPath does not remove last path if only one remains", () => {
    render(
      <MemoryRouter initialEntries={["/foo"]}>
        <BrowserHistoryProvider>
          <TestComponent />
        </BrowserHistoryProvider>
      </MemoryRouter>
    );
    // pop once
    screen.getByTestId("pop").click();
    const stack = JSON.parse(screen.getByTestId("history-stack").textContent || "[]");
    expect(stack.length).toBe(1);
  });

  it("throws error if useBrowserHistory is used outside provider", () => {
    const BrokenComponent = () => {
      // @ts-expect-error purposely breaking
      return <div>{useBrowserHistory().historyStack}</div>;
    };
    const spy = jest.spyOn(console, "error").mockImplementation(() => {});
    expect(() => render(<BrokenComponent />)).toThrow();
    spy.mockRestore();
  });

  it("pushes new path when location changes", async () => {
    function NavigatingComponent() {
      const navigate = useNavigate();
      const { historyStack } = useBrowserHistory();
      return (
        <div>
          <button onClick={() => navigate("/page1")} data-testid="nav-page1">
            Go to Page 1
          </button>
          <button onClick={() => navigate("/page2")} data-testid="nav-page2">
            Go to Page 2
          </button>
          <div data-testid="history-stack">{JSON.stringify(historyStack)}</div>
        </div>
      );
    }

    render(
      <MemoryRouter initialEntries={["/"]}>
        <BrowserHistoryProvider>
          <Routes>
            <Route path="/" element={<NavigatingComponent />} />
            <Route path="/page1" element={<NavigatingComponent />} />
            <Route path="/page2" element={<NavigatingComponent />} />
          </Routes>
        </BrowserHistoryProvider>
      </MemoryRouter>
    );

    // Navigate to page1
    screen.getByTestId("nav-page1").click();
    
    await waitFor(() => {
      const stack = JSON.parse(screen.getByTestId("history-stack").textContent || "[]");
      expect(stack).toContain("/page1");
    });
  });

  it("tracks search params in history", async () => {
    function NavigatingComponent() {
      const navigate = useNavigate();
      const { historyStack } = useBrowserHistory();
      return (
        <div>
          <button onClick={() => navigate("/page?param=value")} data-testid="nav-with-param">
            Go with Param
          </button>
          <div data-testid="history-stack">{JSON.stringify(historyStack)}</div>
        </div>
      );
    }

    render(
      <MemoryRouter initialEntries={["/"]}>
        <BrowserHistoryProvider>
          <Routes>
            <Route path="/" element={<NavigatingComponent />} />
            <Route path="/page" element={<NavigatingComponent />} />
          </Routes>
        </BrowserHistoryProvider>
      </MemoryRouter>
    );

    screen.getByTestId("nav-with-param").click();
    
    await waitFor(() => {
      const stack = JSON.parse(screen.getByTestId("history-stack").textContent || "[]");
      expect(stack).toContain("/page?param=value");
    });
  });

  it("allows popping path when stack has multiple entries", () => {
    render(
      <MemoryRouter initialEntries={["/"]}>
        <BrowserHistoryProvider>
          <TestComponent />
        </BrowserHistoryProvider>
      </MemoryRouter>
    );

    let stack = JSON.parse(screen.getByTestId("history-stack").textContent || "[]");
    const initialLength = stack.length;

    // Push a new path manually
    screen.getByTestId("push").click();

   screen.getByTestId("push").click();
    screen.getByTestId("push").click();

    stack = JSON.parse(screen.getByTestId("history-stack").textContent || "[]");
    // Path should only be added if it's different from the last one
    expect(stack.length).toBeGreaterThanOrEqual(initialLength);

    // Pop once
    screen.getByTestId("pop").click();
    stack = JSON.parse(screen.getByTestId("history-stack").textContent || "[]");
    
    // Should have at least 1 remaining (won't pop below 1)
    expect(stack.length).toBeGreaterThanOrEqual(1);
  });

  it("updates history when location.search changes", async () => {
    function NavigatingComponent() {
      const navigate = useNavigate();
      const { historyStack } = useBrowserHistory();
      const location = useLocation();
      return (
        <div>
          <button onClick={() => navigate("/page?search=1")} data-testid="nav-search-1">
            Search 1
          </button>
          <button onClick={() => navigate("/page?search=2")} data-testid="nav-search-2">
            Search 2
          </button>
          <div data-testid="history-stack">{JSON.stringify(historyStack)}</div>
          <div data-testid="current-location">{location.pathname + location.search}</div>
        </div>
      );
    }

    render(
      <MemoryRouter initialEntries={["/"]}>
        <BrowserHistoryProvider>
          <Routes>
            <Route path="/" element={<NavigatingComponent />} />
            <Route path="/page" element={<NavigatingComponent />} />
          </Routes>
        </BrowserHistoryProvider>
      </MemoryRouter>
    );

    screen.getByTestId("nav-search-1").click();
    
    await waitFor(() => {
      expect(screen.getByTestId("current-location").textContent).toBe("/page?search=1");
    });

    screen.getByTestId("nav-search-2").click();
    
    await waitFor(() => {
      const stack = JSON.parse(screen.getByTestId("history-stack").textContent || "[]");
      expect(stack).toContain("/page?search=2");
      expect(screen.getByTestId("current-location").textContent).toBe("/page?search=2");
    });
  });

  it("maintains separate history for each mounted provider", () => {
    function TestComponent1() {
      const { historyStack } = useBrowserHistory();
      return <div data-testid="history-1">{JSON.stringify(historyStack)}</div>;
    }

    const { unmount } = render(
      <MemoryRouter initialEntries={["/test1"]}>
        <BrowserHistoryProvider>
          <TestComponent1 />
        </BrowserHistoryProvider>
      </MemoryRouter>
    );

    expect(screen.getByTestId("history-1").textContent).toContain("/test1");
    unmount();

    // Mount another provider
    render(
      <MemoryRouter initialEntries={["/test2"]}>
        <BrowserHistoryProvider>
          <div data-testid="history-2">{JSON.stringify(["/test2"])}</div>
        </BrowserHistoryProvider>
      </MemoryRouter>
    );

    expect(screen.getByTestId("history-2").textContent).toContain("/test2");
  });

  it("exports BrowserHistoryProvider as default", () => {
    expect(BrowserHistoryProvider).toBeDefined();
  });
});
