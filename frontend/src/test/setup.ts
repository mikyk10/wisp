// ResizeObserver is not implemented in jsdom; provide a no-op mock.
class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

// @ts-expect-error: jsdom does not have ResizeObserver; we provide a mock
global.ResizeObserver = ResizeObserver

// matchMedia is not implemented in jsdom either; report "not matching"
// (desktop layout) for any query.
if (!window.matchMedia) {
  window.matchMedia = (query: string): MediaQueryList =>
    ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }) as MediaQueryList
}
