import { customVideoConfigForModel, videoRouteForModel, type AiConfig } from "@/stores/use-config-store";
import type { CustomVideoConfig } from "@/lib/custom-video-config";
import { CanvasNodeType, type CanvasConnection, type CanvasImageRole, type CanvasNodeData } from "../types";
import { canvasImageRoles, sameCanvasConnectionIdentity } from "./canvas-connections";

export type CanvasConnectionTarget = {
    readonly targetImageRole?: CanvasImageRole;
    readonly label: string;
    readonly yRatio: number;
    readonly enabled: boolean;
    readonly available: boolean;
    readonly isImageTarget: boolean;
    readonly connectedCount: number;
    readonly maxCount?: number;
    readonly unavailableReason?: string;
    readonly effectiveImageRole?: CanvasImageRole;
    readonly acceptsLegacyConnection?: boolean;
};

export function isCanvasVideoInputNode(node: CanvasNodeData | undefined): node is CanvasNodeData {
    return Boolean(node && (node.type === CanvasNodeType.Video || (node.type === CanvasNodeType.Config && node.metadata?.generationMode === "video")));
}

const imageRoleLabels: Readonly<Record<CanvasImageRole, string>> = {
    images: "图片参考",
    input_reference: "首帧参考图",
    style_references: "风格参考图",
    element_references: "元素参考图",
    reference_images: "兼容参考图",
};

export function canvasConnectionTargetsForNode(config: AiConfig, node: CanvasNodeData, connections: readonly CanvasConnection[]): readonly CanvasConnectionTarget[] {
    if (!isCanvasVideoInputNode(node)) return [nonImageTarget()];

    const model = node.metadata?.model || config.videoModel || config.model;
    const existingRoles = existingImageRoles(node.id, connections);
    if (videoRouteForModel(config, model) !== "custom") {
        return [genericTarget("图片参考", true, legacyConnectionCount(node.id, connections), "images"), ...existingRoles.map((role) => unavailableRoleTarget(role, node.id, connections, "当前模型不支持该图片角色"))];
    }

    const customConfig = customVideoConfigForModel(config, model);
    if (!customConfig) return [genericTarget("图片参考入口不可用，请检查模型配置", false)];

    const enabledRoles = canvasImageRoles.filter((role) => customConfig[role].enabled);
    const roles = canvasImageRoles.filter((role) => enabledRoles.includes(role) || existingRoles.includes(role));
    const legacyRole = canvasLegacyImageRoleForConfig(customConfig);
    const legacyCount = legacyConnectionCount(node.id, connections);
    const targets: CanvasConnectionTarget[] = [];

    if (legacyCount && !legacyRole) targets.push(genericTarget("图片参考（请重新连接到角色入口）", false, legacyCount));
    if (!enabledRoles.length) return [targets[0] || genericTarget("暂无可用图片参考入口", false), ...existingRoles.map((role) => unavailableRoleTarget(role, node.id, connections, "该图片角色已被当前模型禁用"))];

    roles.forEach((role) => {
        const roleConfig = customConfig[role];
        const connectedCount = connections.filter((connection) => connection.toNodeId === node.id && (connection.targetImageRole === role || (legacyRole === role && isLegacyConnection(connection)))).length;
        const available = roleConfig.enabled && connectedCount < roleConfig.max_count;
        targets.push({
            ...(role === "images" ? {} : { targetImageRole: role }),
            label: `${imageRoleLabels[role]}${available ? "" : roleConfig.enabled ? "（已达上限）" : "（当前不可用）"}`,
            yRatio: roleAnchorRatio(role),
            enabled: roleConfig.enabled,
            available,
            isImageTarget: true,
            connectedCount,
            maxCount: roleConfig.max_count,
            unavailableReason: roleConfig.enabled ? "该图片角色已达到数量上限" : "该图片角色已被当前模型禁用",
            effectiveImageRole: role,
            acceptsLegacyConnection: legacyRole === role,
        });
    });

    return targets;
}

export function canvasConnectionTargetForRole(targets: readonly CanvasConnectionTarget[], targetImageRole?: CanvasImageRole) {
    const normalizedRole = targetImageRole === "images" ? undefined : targetImageRole;
    return targets.find((target) => target.targetImageRole === normalizedRole) || (normalizedRole === undefined ? targets.find((target) => target.acceptsLegacyConnection) : undefined);
}

export function canvasConnectionImageRole(config: AiConfig, node: CanvasNodeData | undefined, targetImageRole?: CanvasImageRole) {
    if (targetImageRole && targetImageRole !== "images") return targetImageRole;
    if (!isCanvasVideoInputNode(node)) return undefined;
    const model = node.metadata?.model || config.videoModel || config.model;
    if (videoRouteForModel(config, model) !== "custom") return "images" as const;
    const customConfig = customVideoConfigForModel(config, model);
    return customConfig ? canvasLegacyImageRoleForConfig(customConfig) : undefined;
}

export function canvasLegacyImageRoleForConfig(config: CustomVideoConfig): CanvasImageRole | undefined {
    if (config.images.enabled) return "images";
    const enabledRoles = canvasImageRoles.filter((role) => role !== "images" && config[role].enabled);
    return enabledRoles.length === 1 ? enabledRoles[0] : undefined;
}

export function canvasConnectionValidationError(connection: Pick<CanvasConnection, "fromNodeId" | "toNodeId" | "targetImageRole">, config: AiConfig, nodes: CanvasNodeData[], connections: CanvasConnection[]) {
    const destinationNode = nodes.find((node) => node.id === connection.toNodeId);
    const destinationTargets = destinationNode ? canvasConnectionTargetsForNode(config, destinationNode, connections) : [];
    const target = canvasConnectionTargetForRole(destinationTargets, connection.targetImageRole);
    if (connection.targetImageRole && (!target?.isImageTarget || !isCanvasVideoInputNode(destinationNode))) return "该图片角色入口当前不可用";
    if (target?.isImageTarget && !target.available) return target.unavailableReason || "该图片入口当前不可用";
    if (target?.isImageTarget && nodes.find((node) => node.id === connection.fromNodeId)?.type !== CanvasNodeType.Image) return "图片参考入口只接受图片节点";
    if (connections.some((existing) => sameCanvasConnectionIdentity(existing, connection))) return "同一图片角色不能重复连接";
    return undefined;
}

export function canvasConnectionTargetAnchor(node: CanvasNodeData, targets: readonly CanvasConnectionTarget[], targetImageRole?: CanvasImageRole) {
    const target = canvasConnectionTargetForRole(targets, targetImageRole);
    return { x: node.position.x, y: node.position.y + node.height * (target?.yRatio ?? 0.5) };
}

export function canvasConnectionSourceAnchor(node: CanvasNodeData) {
    return { x: node.position.x + node.width, y: node.position.y + node.height / 2 };
}

export function canvasImageRoleLabel(role: CanvasImageRole) {
    return imageRoleLabels[role];
}

function genericTarget(label: string, available: boolean, connectedCount = 0, effectiveImageRole?: CanvasImageRole): CanvasConnectionTarget {
    return { label, yRatio: 0.5, enabled: available, available, isImageTarget: true, connectedCount, ...(effectiveImageRole ? { effectiveImageRole, acceptsLegacyConnection: true } : {}) };
}

function nonImageTarget(): CanvasConnectionTarget {
    return { label: "", yRatio: 0.5, enabled: true, available: true, isImageTarget: false, connectedCount: 0 };
}

function unavailableRoleTarget(role: CanvasImageRole, nodeId: string, connections: readonly CanvasConnection[], reason: string): CanvasConnectionTarget {
    return {
        targetImageRole: role,
        label: `${imageRoleLabels[role]}（当前不可用）`,
        yRatio: roleAnchorRatio(role),
        enabled: false,
        available: false,
        isImageTarget: true,
        connectedCount: connections.filter((connection) => connection.toNodeId === nodeId && connection.targetImageRole === role).length,
        unavailableReason: reason,
        effectiveImageRole: role,
    };
}

function existingImageRoles(nodeId: string, connections: readonly CanvasConnection[]) {
    return canvasImageRoles.filter((role) => role !== "images" && connections.some((connection) => connection.toNodeId === nodeId && connection.targetImageRole === role));
}

function legacyConnectionCount(nodeId: string, connections: readonly CanvasConnection[]) {
    return connections.filter((connection) => connection.toNodeId === nodeId && isLegacyConnection(connection)).length;
}

function isLegacyConnection(connection: Pick<CanvasConnection, "targetImageRole">) {
    return connection.targetImageRole === undefined || connection.targetImageRole === "images";
}

function roleAnchorRatio(role: CanvasImageRole, index = canvasImageRoles.indexOf(role), count = canvasImageRoles.length) {
    if (count <= 1) return 0.5;
    return 0.18 + (0.64 * index) / (count - 1);
}
