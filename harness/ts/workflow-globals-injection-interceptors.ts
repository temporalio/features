/**
 * Unsafely inject some packages into the isolated context
 *
 * @module
 */
const { mainModule } = globalThis.constructor.constructor('return process')();

Object.assign(globalThis, {
  temporalioHarness: mainModule.require('@temporalio/harness'),
  temporalioClient: mainModule.require('@temporalio/client'),
  temporalioActivity: mainModule.require('@temporalio/activity'),
});

// Doesn't actually do anything, just inject global variables before main workflow code is evaluated
export const interceptors = (): Record<string, never> => ({});
