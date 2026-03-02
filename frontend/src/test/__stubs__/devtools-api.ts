// Stub for @vue/devtools-api used in tests.
// Pinia imports setupDevtoolsPlugin from this package in dev mode,
// which transitively imports @vue/devtools-kit and calls localStorage
// at module initialisation — before jsdom is ready.
// Replacing the whole package with no-ops prevents the crash.
export const setupDevtoolsPlugin = () => {}
export const addCustomTab = () => {}
export const addCustomCommand = () => {}
export const onDevToolsClientConnected = () => {}
