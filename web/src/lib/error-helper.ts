import { ApiError } from "@/lib/api-error";

// Balance error keywords matching backend's IsUpstreamBalanceError
const BALANCE_KEYWORDS = [
    "余额不足",
    "额度不足",
    "用户额度不足",
    "积分不足",
    "insufficient balance",
    "insufficient_user_quota",
    "insufficient_quota",
    "quota exceeded",
    "billing failed",
    "扣费额度失败",
];

/**
 * Check if an error message indicates a balance/credit issue.
 */
export function isBalanceError(message: string): boolean {
    if (!message) return false;
    return BALANCE_KEYWORDS.some((kw) =>
        message.toLowerCase().includes(kw.toLowerCase())
    );
}

/**
 * Extract raw upstream error detail from an error object.
 * Returns undefined if no error_detail is available.
 */
export function extractErrorDetail(error: unknown): string | undefined {
    if (error instanceof ApiError && error.errorDetail) {
        return error.errorDetail;
    }
    return undefined;
}

export type VideoErrorDisplay = {
    message: string;
    rawDetail?: string;
};

export function formatVideoGenerationError(error: unknown, fallback = "生成失败"): VideoErrorDisplay {
    const rawMessage = error instanceof Error ? error.message : fallback;
    const rawDetail = extractErrorDetail(error);
    const upstreamMessage = firstMeaningfulError(rawDetail, rawMessage);
    const balanceSource = [rawMessage, rawDetail, upstreamMessage].filter(Boolean).join("\n");
    if (isBalanceError(balanceSource)) {
        return { message: safeBalanceVideoMessage(rawMessage, rawDetail) };
    }

    const primary = upstreamMessage || beautifyVideoError(rawMessage || fallback);
    const message = withVideoErrorGuidance(primary);
    const detailText = normalizeErrorText(rawDetail || "");
    const detail = detailText && !detailText.includes(normalizeErrorText(primary)) ? rawDetail : undefined;
    return { message, rawDetail: detail };
}

/**
 * 美化视频生成错误消息，将技术错误转换为用户友好的中文提示
 */
export function beautifyVideoError(rawError: string): string {
    if (!rawError) return "生成失败，请稍后重试";

    const error = rawError.toLowerCase();
    const meaningfulPart = extractMeaningfulError(rawError);

    if (meaningfulPart && !isBalanceError(meaningfulPart) && /[\u4e00-\u9fa5]/.test(meaningfulPart)) {
        return meaningfulPart;
    }

    // 名人检测失败
    if (error.includes("prominent_people") || error.includes("prominent_person")) {
        return "检测到名人或公众人物内容。VEO 模型不支持生成真实名人视频，请修改提示词或更换参考图片后重试。";
    }

    // 提示词超长
    if (error.includes("prompt length exceeds") || error.includes("maximum allowed length")) {
        const match = rawError.match(/(\d+)/);
        const limit = match ? match[1] : "4096";
        return `提示词过长，最多支持 ${limit} 字符。请精简描述后重试。`;
    }

    // 内容安全过滤
    if (error.includes("safety") || error.includes("policy") || error.includes("moderation")) {
        return "内容可能违反安全策略。请修改提示词，避免暴力、色情或不当内容。";
    }

    // 图片相关错误
    if (error.includes("image") && (error.includes("invalid") || error.includes("format"))) {
        return "参考图片格式不支持或已损坏，请更换图片后重试。";
    }
    if (error.includes("image") && error.includes("size")) {
        return "参考图片尺寸超限，建议使用小于 5MB 的图片。";
    }

    // 超时错误
    if (error.includes("timeout") || error.includes("timed out")) {
        return "生成超时，可能因服务繁忙。请稍后重试。";
    }

    // 配额/限流
    if (error.includes("quota") || error.includes("rate limit")) {
        return "上游服务触发限流，请稍后再试。";
    }

    // 积分不足
    if (error.includes("积分不足") || error.includes("insufficient")) {
        return rawError; // 保持原样，已经是中文
    }

    // 模型不可用
    if (error.includes("model") && (error.includes("not available") || error.includes("unavailable"))) {
        return "当前模型暂时不可用，请稍后重试或更换其他模型。";
    }

    // 通用网络错误
    if (error.includes("network") || error.includes("connection")) {
        return "网络连接失败，请检查网络后重试。";
    }

    // 上游 API 错误
    if (error.includes("upstream") || error.includes("bad_response")) {
        return "上游服务异常，请稍后重试。如持续失败，请联系管理员。";
    }

    // 其他情况：尝试提取有意义的部分
    return meaningfulPart || "生成失败，请稍后重试";
}

/**
 * 从复杂的错误 JSON 中提取有意义的错误信息
 */
function extractMeaningfulError(raw: string): string {
    const text = normalizeErrorText(raw);
    if (!text) return "";
    if (looksLikeJSON(text)) {
        const value = extractMeaningfulValue(text, 0);
        if (value) return value;
    }
    // 不是 JSON，尝试正则提取
    const match = text.match(/(?:error|message|msg)[":"]\s*["']([^"']+)["']/i);
    if (match) return match[1];
    return "";
}

function firstMeaningfulError(...values: Array<string | undefined>) {
    for (const value of values) {
        const direct = normalizeErrorText(value || "");
        if (!direct) continue;
        const extracted = extractMeaningfulError(direct);
        return extracted || direct;
    }
    return "";
}

function extractMeaningfulValue(value: unknown, depth: number): string {
    if (depth > 4 || value == null) return "";
    if (typeof value === "string") {
        const raw = normalizeErrorText(value);
        if (!raw) return "";
        if (!looksLikeJSON(raw)) return raw;
        try {
            return extractMeaningfulValue(JSON.parse(raw), depth + 1) || raw;
        } catch {
            return raw;
        }
    }
    if (typeof value !== "object") return "";
    const data = value as Record<string, unknown>;
    for (const key of ["error", "message", "msg", "error_detail", "detail"]) {
        const extracted = extractMeaningfulValue(data[key], depth + 1);
        if (extracted) return extracted;
    }
    return "";
}

function looksLikeJSON(value: string) {
    const first = value.trimStart()[0];
    return first === "{" || first === "[";
}

function normalizeErrorText(value: string) {
    return String(value || "").trim();
}

function withVideoErrorGuidance(message: string) {
    const text = normalizeErrorText(message) || "生成失败，请稍后重试";
    if (text.includes("因上游问题被禁用")) return text;
    const guidance = videoErrorGuidance(text);
    return guidance && !text.includes(guidance) ? `${text}\n\n建议：${guidance}` : text;
}

function safeBalanceVideoMessage(rawMessage: string, rawDetail?: string) {
    const text = normalizeErrorText(rawMessage);
    if (text.includes("因上游问题被禁用")) return text;
    if (!rawDetail && text.includes("积分不足")) return text;
    return "因上游问题被禁用";
}

function videoErrorGuidance(message: string) {
    const error = message.toLowerCase();
    if (error.includes("图生视频") || error.includes("参考图") || error.includes("reference_images") || error.includes("image")) {
        if (error.includes("必须") || error.includes("required") || error.includes("provide")) return "添加 1 张参考图后重试，或切换到支持纯文生视频的模型。";
        if (error.includes("format") || error.includes("格式") || error.includes("size") || error.includes("大小")) return "更换或压缩参考图后重试。";
    }
    if (error.includes("线路") || error.includes("暂时不可用") || error.includes("unavailable") || error.includes("not available")) {
        return "稍后重试，或切换其它模型/渠道。";
    }
    if (error.includes("真人脸") || error.includes("真实人物") || error.includes("safety") || error.includes("policy") || error.includes("moderation") || error.includes("real person")) {
        return "更换参考图或调整提示词，避免真实人物、人脸或敏感内容。";
    }
    if (error.includes("图片") || error.includes("image")) return "检查参考图是否可访问、格式是否支持，必要时更换或压缩后重试。";
    if (error.includes("prompt is too short")) return "提示词过短，请补充描述后重试。";
    return "可稍后重试，或更换模型/渠道。";

}
