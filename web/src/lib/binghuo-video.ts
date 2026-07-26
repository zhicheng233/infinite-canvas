export const binghuoResolutionOptions = ["480P", "540P", "720P", "1080P", "4K"] as const;
export const binghuoRatioOptions = ["16:9", "9:16", "1:1", "4:3", "3:4", "2:3", "3:2", "21:9"] as const;

export type BinghuoReferenceMode = "reference" | "first_last";

export function normalizeBinghuoRatio(value: string) {
    const ratio = dimensionsRatio(value);
    if (!ratio) return "16:9";
    return binghuoRatioOptions.reduce((best, item) => Math.abs(ratioValue(item) - ratio) < Math.abs(ratioValue(best) - ratio) ? item : best, binghuoRatioOptions[0]);
}

export function normalizeBinghuoResolution(value: string) {
    const normalized = String(value || "").trim().toUpperCase();
    if (normalized === "4K") return "4K";
    const pixels = Number(normalized.replace(/P$/, ""));
    if (!pixels) return "720P";
    const values = [480, 540, 720, 1080, 2160] as const;
    const nearest = values.reduce((best, item) => Math.abs(item - pixels) < Math.abs(best - pixels) ? item : best, values[0]);
    return nearest === 2160 ? "4K" : `${nearest}P`;
}

export function normalizeBinghuoReferenceMode(value: string | undefined): BinghuoReferenceMode {
    return value === "first_last" ? "first_last" : "reference";
}

function dimensionsRatio(value: string) {
    const normalized = String(value || "").trim();
    const match = normalized.match(/^(\d+(?:\.\d+)?)\s*[:x]\s*(\d+(?:\.\d+)?)$/i);
    if (!match) return 0;
    const width = Number(match[1]);
    const height = Number(match[2]);
    return width > 0 && height > 0 ? width / height : 0;
}

function ratioValue(value: string) {
    const [width, height] = value.split(":").map(Number);
    return width / height;
}
