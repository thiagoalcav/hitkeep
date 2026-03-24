import { formatBytes, formatResponseMs, mapAIFetchSeries } from "@pages/ai-visibility/ai-visibility.utils";

describe("AIVisibility utils", () => {
    it("maps fetch series for charts", () => {
        expect(mapAIFetchSeries([{ time: "2026-03-01T00:00:00Z", count: 12 }])).toEqual([{ time: "2026-03-01T00:00:00Z", count: 12 }]);
    });

    it("formats bytes for display", () => {
        expect(formatBytes(1024, "en-US")).toBe("1 KB");
    });

    it("formats response time for display", () => {
        expect(formatResponseMs(842, "en-US")).toBe("842 ms");
    });
});
