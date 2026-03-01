export function LogDebug(message: string): void;
export function LogInfo(message: string): void;
export function LogWarning(message: string): void;
export function LogError(message: string): void;
export function EventsOn(eventName: string, callback: (...data: any) => void): () => void;
export function EventsEmit(eventName: string, ...data: any): void;
export function Quit(): void;
