import { customVideoConfigForModel, videoRouteForModel, type AiConfig } from "@/stores/use-config-store";
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
};

export function isCanvasVideoInputNode(node: CanvasNodeData | undefined): node is CanvasNodeData {
    return Boolean(node && (node.type === CanvasNodeType.Video || (node.type === CanvasNodeType.Config && node.metadata?.generationMode === "video")));
}

const imageRoleLabels: Readonly<Record<CanvasImageRole, string>> = {
    images: "普通参考图",
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
        return [genericTarget("图片参考", true), ...existingRoles.map((role) => unavailableRoleTarget(role, canvasImageRoles.indexOf(role), canvasImageRoles.length, "当前模型不支持该图片角色"))];
    }

    const customConfig = customVideoConfigForModel(config, model);
    if (!customConfig) return [genericTarget("图片参考入口不可用，请检查模型配置", false)];

    const enabledRoles = canvasImageRoles.filter((role) => customConfig[role].enabled);
    const roles = canvasImageRoles.filter((role) => enabledRoles.includes(role) || existingRoles.includes(role));
    const genericConnection = connections.some((connection) => connection.toNodeId === node.id && connection.targetImageRole === undefined);
    const targets: CanvasConnectionTarget[] = [];

    if (genericConnection) targets.push(genericTarget("通用图片（请重新连接到角色入口）", false));
    if (!enabledRoles.length) return [genericTarget("暂无可用图片参考入口", false), ...existingRoles.map((role) => unavailableRoleTarget(role, canvasImageRoles.indexOf(role), canvasImageRoles.length, "该图片角色已被当前模型禁用"))];

    roles.forEach((role) => {
        const roleConfig = customConfig[role];
        const connectedCount = connections.filter((connection) => connection.toNodeId === node.id && connection.targetImageRole === role).length;
        const available = roleConfig.enabled && connectedCount < roleConfig.max_count;
        targets.push({
            targetImageRole: role,
            label: `${imageRoleLabels[role]}${available ? "" : roleConfig.enabled ? "（已达上限）" : "（当前不可用）"}`,
            yRatio: roleAnchorRatio(role),
            enabled: roleConfig.enabled,
            available,
            isImageTarget: true,
            connectedCount,
            maxCount: roleConfig.max_count,
            unavailableReason: roleConfig.enabled ? "该图片角色已达到数量上限" : "该图片角色已被当前模型禁用",
        });
    });

    return targets;
}

export function canvasConnectionTargetForRole(targets: readonly CanvasConnectionTarget[], targetImageRole?: CanvasImageRole) {
    return targets.find((target) => target.targetImageRole === targetImageRole);
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

function genericTarget(label: string, available: boolean): CanvasConnectionTarget {
    return { label, yRatio: 0.5, enabled: available, available, isImageTarget: true, connectedCount: 0 };
}

function nonImageTarget(): CanvasConnectionTarget {
    return { label: "", yRatio: 0.5, enabled: true, available: true, isImageTarget: false, connectedCount: 0 };
}

function unavailableRoleTarget(role: CanvasImageRole, index: number, count: number, reason: string): CanvasConnectionTarget {
    return {
        targetImageRole: role,
        label: `${imageRoleLabels[role]}（当前不可用）`,
        yRatio: roleAnchorRatio(role, index, count),
        enabled: false,
        available: false,
        isImageTarget: true,
        connectedCount: 0,
        unavailableReason: reason,
    };
}

function existingImageRoles(nodeId: string, connections: readonly CanvasConnection[]) {
    return canvasImageRoles.filter((role) => connections.some((connection) => connection.toNodeId === nodeId && connection.targetImageRole === role));
}

function roleAnchorRatio(role: CanvasImageRole, index = canvasImageRoles.indexOf(role), count = canvasImageRoles.length) {
    if (count <= 1) return 0.5;
    return 0.18 + (0.64 * index) / (count - 1);
}
