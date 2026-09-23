import React from "react";
import { render, screen } from "@testing-library/react";
import { MessageServiceProvider, useMessageService } from "./MessageServiceProvider";
import { type MessageServiceContextType } from "./types";

describe("MessageServiceProvider", () => {
    describe("Provider Rendering", () => {
        it("should render children correctly", () => {
            render(
                <MessageServiceProvider>
                    <div data-testid="child-element">Test Child</div>
                </MessageServiceProvider>
            );

            expect(screen.getByTestId("child-element")).toBeInTheDocument();
            expect(screen.getByText("Test Child")).toBeInTheDocument();
        });

        it("should render multiple children", () => {
            render(
                <MessageServiceProvider>
                    <div data-testid="child-1">Child 1</div>
                    <div data-testid="child-2">Child 2</div>
                </MessageServiceProvider>
            );

            expect(screen.getByTestId("child-1")).toBeInTheDocument();
            expect(screen.getByTestId("child-2")).toBeInTheDocument();
        });

        it("should provide context value with services", () => {
            const TestComponent = () => {
                const context = useMessageService();
                return <div data-testid="has-services">{context.userDataService && context.filterDataService ? "true" : "false"}</div>;
            };

            render(
                <MessageServiceProvider>
                    <TestComponent />
                </MessageServiceProvider>
            );

            expect(screen.getByTestId("has-services")).toHaveTextContent("true");
        });
    });

    describe("useMessageService Hook", () => {
        it("should return context when used within provider", () => {
            const TestComponent = () => {
                const context = useMessageService();
                return (
                    <div data-testid="context-value">
                        {context.userDataService && context.filterDataService ? "initialized" : "not-initialized"}
                    </div>
                );
            };

            render(
                <MessageServiceProvider>
                    <TestComponent />
                </MessageServiceProvider>
            );

            const contextValue = screen.getByTestId("context-value");
            expect(contextValue).toBeInTheDocument();
            expect(contextValue).toHaveTextContent("initialized");
        });

        it("should throw error when used outside provider", () => {
            const TestComponent = () => {
                useMessageService();
                return <div>Test</div>;
            };

            // Suppress console.error for this test
            const consoleSpy = jest.spyOn(console, "error").mockImplementation(() => { });

            expect(() => {
                render(<TestComponent />);
            }).toThrow("useMessageService must be used within a MessageServiceProvider");

            consoleSpy.mockRestore();
        });

        it("should throw correct error message when context is undefined", () => {
            const TestComponent = () => {
                useMessageService();
                return null;
            };

            const consoleSpy = jest.spyOn(console, "error").mockImplementation(() => { });

            expect(() => {
                render(<TestComponent />);
            }).toThrow("useMessageService must be used within a MessageServiceProvider");

            consoleSpy.mockRestore();
        });
    });

    describe("Context Value", () => {
        it("should provide consistent context value across multiple hook calls", () => {
            const values: MessageServiceContextType[] = [];

            const TestComponent = () => {
                const context = useMessageService();
                values.push(context);
                return null;
            };

            render(
                <MessageServiceProvider>
                    <TestComponent />
                    <TestComponent />
                </MessageServiceProvider>
            );

            expect(values).toHaveLength(2);
            expect(values[0]).toEqual(values[1]);
        });

        it("should have services in context value", () => {
            const TestComponent = () => {
                const context = useMessageService();
                return (
                    <div data-testid="has-services">
                        {context.userDataService && context.filterDataService && context.mfeOrchestrationService ? "true" : "false"}
                    </div>
                );
            };

            render(
                <MessageServiceProvider>
                    <TestComponent />
                </MessageServiceProvider>
            );

            expect(screen.getByTestId("has-services")).toHaveTextContent("true");
        });

        it("should have initialized services", () => {
            const TestComponent = () => {
                const { userDataService, filterDataService } = useMessageService();
                return (
                    <div data-testid="services-initialized">{userDataService && filterDataService ? "initialized" : "not-initialized"}</div>
                );
            };

            render(
                <MessageServiceProvider>
                    <TestComponent />
                </MessageServiceProvider>
            );

            expect(screen.getByTestId("services-initialized")).toHaveTextContent("initialized");
        });
    });

    describe("useEffect Initialization", () => {
        it("should execute useEffect on mount", () => {
            const effectSpy = jest.fn();

            jest.spyOn(React, "useEffect").mockImplementation((callback) => {
                effectSpy(callback);
            });

            render(
                <MessageServiceProvider>
                    <div>Test</div>
                </MessageServiceProvider>
            );

            expect(effectSpy).toHaveBeenCalled();
        });
    });

    describe("Integration Tests", () => {
        it("should work with nested components accessing the service", () => {
            const InnerComponent = () => {
                const { userDataService } = useMessageService();
                return <div data-testid="inner-service">{userDataService ? "available" : "unavailable"}</div>;
            };

            const MiddleComponent = () => {
                return <InnerComponent />;
            };

            render(
                <MessageServiceProvider>
                    <MiddleComponent />
                </MessageServiceProvider>
            );

            expect(screen.getByTestId("inner-service")).toHaveTextContent("available");
        });

        it("should handle rapid sequential hook calls", () => {
            const values: MessageServiceContextType[] = [];

            const TestComponent = () => {
                values.push(useMessageService());
                values.push(useMessageService());
                values.push(useMessageService());
                return null;
            };

            render(
                <MessageServiceProvider>
                    <TestComponent />
                </MessageServiceProvider>
            );

            expect(values).toHaveLength(3);
            expect(values[0]).toBe(values[1]);
            expect(values[1]).toBe(values[2]);
        });
    });
});