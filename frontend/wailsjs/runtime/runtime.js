export function LogDebug(message) { window.runtime.LogDebug(message); }
export function LogInfo(message) { window.runtime.LogInfo(message); }
export function LogWarning(message) { window.runtime.LogWarning(message); }
export function LogError(message) { window.runtime.LogError(message); }
export function EventsOn(eventName, callback) { return window.runtime.EventsOn(eventName, callback); }
export function EventsEmit(eventName, ...data) { window.runtime.EventsEmit(eventName, ...data); }
export function Quit() { window.runtime.Quit(); }
