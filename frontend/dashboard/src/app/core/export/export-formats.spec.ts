import { TranslocoService } from "@jsverse/transloco";

import { buildTakeoutExportMenuItems, DEFAULT_HITS_EXPORT_FORMAT, DEFAULT_TAKEOUT_EXPORT_FORMAT, TAKEOUT_EXPORT_FORMATS } from "./export-formats";

describe("export-formats", () => {
    it("should expose all supported export formats in one place", () => {
        expect(TAKEOUT_EXPORT_FORMATS).toEqual(["csv", "xlsx", "parquet", "json", "ndjson"]);
        expect(DEFAULT_TAKEOUT_EXPORT_FORMAT).toBe("xlsx");
        expect(DEFAULT_HITS_EXPORT_FORMAT).toBe("csv");
    });

    it("should build translated menu items that call onSelect with matching format", () => {
        const selected: string[] = [];
        const transloco = {
            translate: (key: string) => `tr:${key}`
        } as unknown as TranslocoService;

        const menuItems = buildTakeoutExportMenuItems(transloco, (format) => selected.push(format));

        expect(menuItems.length).toBe(TAKEOUT_EXPORT_FORMATS.length);
        expect(menuItems.map((item) => item.label)).toEqual(["tr:common.exportFormats.csv", "tr:common.exportFormats.xlsx", "tr:common.exportFormats.parquet", "tr:common.exportFormats.json", "tr:common.exportFormats.ndjson"]);

        for (const item of menuItems) {
            item.command?.({} as never);
        }
        expect(selected).toEqual([...TAKEOUT_EXPORT_FORMATS]);
    });
});
