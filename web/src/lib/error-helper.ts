/**
 * 美化视频生成错误消息，将技术错误转换为用户友好的中文提示
 */
export function beautifyVideoError(rawError: string): string {
  if (!rawError) return "生成失败，请稍后重试";

  const error = rawError.toLowerCase();

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
  const meaningfulPart = extractMeaningfulError(rawError);
  return meaningfulPart || "生成失败，请稍后重试";
}

/**
 * 从复杂的错误 JSON 中提取有意义的错误信息
 */
function extractMeaningfulError(raw: string): string {
  try {
    // 尝试解析 JSON 错误
    const parsed = JSON.parse(raw);
    if (parsed.error?.message) return parsed.error.message;
    if (parsed.message) return parsed.message;
  } catch {
    // 不是 JSON，尝试正则提取
    const match = raw.match(/message[":"]\s*["']([^"']+)["']/i);
    if (match) return match[1];
  }
  return "";
}
