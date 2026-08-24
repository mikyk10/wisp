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

// visualViewport is not implemented in jsdom. Vuetify's overlays (menus,
// bottom sheets) read it while positioning themselves, so without it any test
// that opens one dies with "visualViewport is not defined" before it can
// assert anything. The numbers are the jsdom window's own.
if (!window.visualViewport) {
  Object.defineProperty(window, 'visualViewport', {
    configurable: true,
    value: {
      width: window.innerWidth,
      height: window.innerHeight,
      offsetLeft: 0,
      offsetTop: 0,
      pageLeft: 0,
      pageTop: 0,
      scale: 1,
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    },
  })
}
