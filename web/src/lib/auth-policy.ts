const usernamePattern = /^[A-Za-z0-9_-]{1,64}$/;
const passwordLetterPattern = /[A-Za-z]/;
const passwordDigitPattern = /[0-9]/;

export function getUsernamePolicyError(username: string): string | null {
    const normalized = username.trim();
    if (normalized.length === 0) {
        return "请输入用户名";
    }
    if (!usernamePattern.test(normalized)) {
        return "用户名仅支持1-64位字母、数字、下划线或短横线";
    }
    return null;
}

export function getPasswordPolicyError(password: string): string | null {
    if (Array.from(password).length < 8) {
        return "密码至少需要8个字符";
    }
    if (!passwordLetterPattern.test(password) || !passwordDigitPattern.test(password)) {
        return "密码需包含字母和数字";
    }
    return null;
}
