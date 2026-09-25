// Jest stand-in for the spog remote module `shellSpog/filterDataMessageService`.
export const mfeFilterDataMessageService = {
    subscribe: jest.fn((_componentName: string, _handler: (event: Event) => void) => jest.fn()),
    publish: jest.fn(),
};
