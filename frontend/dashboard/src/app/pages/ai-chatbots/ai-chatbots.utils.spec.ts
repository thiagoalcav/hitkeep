import { calcDelta, computeComparisonPeriod, createEmptySeries, safeRate, totalFor } from "@pages/ai-chatbots/ai-chatbots.utils";

describe("AI chatbot utils", () => {
    it("creates an empty series state", () => {
        expect(createEmptySeries()).toEqual({
            started: [],
            sent: [],
            rendered: [],
            clicked: [],
            handoff: [],
            assisted: []
        });
    });

    it("computes the previous comparison window", () => {
        expect(computeComparisonPeriod("2026-03-10T00:00:00.000Z", "2026-03-20T00:00:00.000Z")).toEqual({
            from: "2026-02-27T23:59:59.999Z",
            to: "2026-03-09T23:59:59.999Z"
        });
    });

    it("totals a metric series", () => {
        const state = createEmptySeries();
        state.started = [
            { time: "2026-03-18T00:00:00Z", count: 3 },
            { time: "2026-03-19T00:00:00Z", count: 4 }
        ];

        expect(totalFor("started", state)).toBe(7);
    });

    it("guards divide-by-zero in rates", () => {
        expect(safeRate(2, 0)).toBe(0);
        expect(safeRate(2, 5)).toBe(40);
    });

    it("returns null delta when there is no previous baseline", () => {
        expect(calcDelta(12, 0)).toBeNull();
        expect(calcDelta(15, 10)).toBe(50);
    });
});
