// Jest stand-in for the spog remote module `shellSpog/userDataMessageService`.
export const mfeUserDataMessageService = {
    subscribe: jest.fn((_componentName: string, _handler: (event: Event) => void) => jest.fn()),
    publish: jest.fn(),
};
