// ResizeObserver is not implemented in jsdom; provide a no-op mock.
class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

// @ts-expect-error: jsdom does not have ResizeObserver; we provide a mock
global.ResizeObserver = ResizeObserver
