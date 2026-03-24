import { EventSeriesPoint } from "@models/analytics.types";

export type ChatbotMetricKey = "started" | "sent" | "rendered" | "clicked" | "handoff" | "assisted";

export interface ChatbotSeriesState {
    started: EventSeriesPoint[];
    sent: EventSeriesPoint[];
    rendered: EventSeriesPoint[];
    clicked: EventSeriesPoint[];
    handoff: EventSeriesPoint[];
    assisted: EventSeriesPoint[];
}

export function createEmptySeries(): ChatbotSeriesState {
    return {
        started: [],
        sent: [],
        rendered: [],
        clicked: [],
        handoff: [],
        assisted: []
    };
}

export function computeComparisonPeriod(from: string, to: string): { from: string; to: string } {
    const start = new Date(from);
    const end = new Date(to);
    const duration = end.getTime() - start.getTime();
    const comparisonEnd = new Date(start.getTime() - 1);
    return {
        from: new Date(comparisonEnd.getTime() - duration).toISOString(),
        to: comparisonEnd.toISOString()
    };
}

export function totalFor(key: ChatbotMetricKey, state: ChatbotSeriesState): number {
    return state[key].reduce((sum, point) => sum + point.count, 0);
}

export function safeRate(numerator: number, denominator: number): number {
    if (denominator === 0) return 0;
    return (numerator / denominator) * 100;
}

export function calcDelta(current: number, previous: number): number | null {
    if (previous === 0) return null;
    return ((current - previous) / previous) * 100;
}
