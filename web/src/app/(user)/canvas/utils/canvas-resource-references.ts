import { imageReferenceLabel } from "@/lib/image-reference-prompt";
import { seedanceReferenceLabel } from "@/lib/seedance-video";
import type { AiConfig } from "@/stores/use-config-store";
import { canvasConnectionTargetForRole, canvasConnectionTargetsForNode } from "./canvas-connection-targets";
import { CanvasNodeType, type CanvasConnection, type CanvasImageRole, type CanvasNodeData } from "../types";

export type CanvasResourceKind = "image" | "video" | "audio" | "text";

export type CanvasResourceReference = {
    id: string;
    nodeId: string;
    kind: CanvasResourceKind;
    label: string;
    title: string;
    previewUrl?: string;
    text?: string;
    active: boolean;
};

export type CanvasGenerationResourceNode = {
    readonly node: CanvasNodeData;
    readonly targetImageRole?: CanvasImageRole;
};

export function buildCanvasResourceReferences(nodes: CanvasNodeData[], connections: CanvasConnection[], contextNodeId?: string | null) {
    const contextNodes = contextNodeId ? getMentionResourceNodes(contextNodeId, nodes, connections) : [];
    const globalReferences = labelResourceNodes(nodes.filter(isResourceNode), false);
    const activeByNodeId = new Map(labelResourceNodes(contextNodes, true).map((reference) => [reference.nodeId, reference]));
    return globalReferences.map((reference) => activeByNodeId.get(reference.nodeId) || reference);
}

export function buildNodeMentionReferences(node: CanvasNodeData, nodes: CanvasNodeData[], connections: CanvasConnection[]) {
    return labelResourceNodes(getMentionResourceNodes(node.id, nodes, connections), true);
}

export function getMentionResourceNodes(nodeId: string, nodes: CanvasNodeData[], connections: CanvasConnection[]) {
    const configInputs = getConnectedConfigResourceNodes(nodeId, nodes, connections);
    if (configInputs.length) return configInputs;
    const ownInputs = getContextResourceNodes(nodeId, nodes, connections);
    if (ownInputs.length) return ownInputs;
    const node = nodes.find((item) => item.id === nodeId);
    return node && isResourceNode(node) ? [node] : [];
}

export function getGenerationResourceNodes(nodeId: string, nodes: CanvasNodeData[], connections: CanvasConnection[], config?: AiConfig) {
    const configInputs = getConnectedConfigGenerationResources(nodeId, nodes, connections, config);
    if (configInputs.length) return configInputs;
    const ownInputs = getContextGenerationResources(nodeId, nodes, connections, config);
    if (ownInputs.length) return ownInputs;
    return [];
}

function getContextResourceNodes(nodeId: string, nodes: CanvasNodeData[], connections: CanvasConnection[]) {
    return getContextGenerationResources(nodeId, nodes, connections).map((resource) => resource.node);
}

function getContextGenerationResources(nodeId: string, nodes: CanvasNodeData[], connections: CanvasConnection[], config?: AiConfig) {
    return connections
        .filter((connection) => connection.toNodeId === nodeId)
        .flatMap((connection): CanvasGenerationResourceNode[] => {
            const node = nodes.find((item) => item.id === connection.fromNodeId);
            if (!node || !isResourceNode(node)) return [];
            if (connection.targetImageRole && config) {
                const targetNode = nodes.find((item) => item.id === connection.toNodeId);
                if (!targetNode) return [];
                const target = canvasConnectionTargetForRole(canvasConnectionTargetsForNode(config, targetNode, connections), connection.targetImageRole);
                if (!target?.enabled) return [];
            }
            return [{ node, ...(connection.targetImageRole ? { targetImageRole: connection.targetImageRole } : {}) }];
        });
}

function getConnectedConfigResourceNodes(nodeId: string, nodes: CanvasNodeData[], connections: CanvasConnection[]) {
    return getConnectedConfigGenerationResources(nodeId, nodes, connections).map((resource) => resource.node);
}

function getConnectedConfigGenerationResources(nodeId: string, nodes: CanvasNodeData[], connections: CanvasConnection[], config?: AiConfig) {
    const configConnection = connections.find((connection) => connection.fromNodeId === nodeId && nodes.find((node) => node.id === connection.toNodeId)?.type === CanvasNodeType.Config);
    if (!configConnection) return [];
    return getContextGenerationResources(configConnection.toNodeId, nodes, connections, config).filter((resource) => resource.node.id !== nodeId);
}

function labelResourceNodes(nodes: CanvasNodeData[], active: boolean) {
    const counts: Record<CanvasResourceKind, number> = { image: 0, video: 0, audio: 0, text: 0 };
    return nodes.flatMap((node): CanvasResourceReference[] => {
        const kind = resourceKind(node);
        if (!kind) return [];
        const index = counts[kind]++;
        const label = labelForKind(kind, index);
        return [
            {
                id: node.id,
                nodeId: node.id,
                kind,
                label,
                title: node.title || label,
                previewUrl: node.metadata?.content,
                text: node.type === CanvasNodeType.Text ? node.metadata?.content || node.metadata?.prompt : undefined,
                active,
            },
        ];
    });
}

function labelForKind(kind: CanvasResourceKind, index: number) {
    if (kind === "image") return imageReferenceLabel(index);
    if (kind === "video") return seedanceReferenceLabel("video", index);
    if (kind === "audio") return seedanceReferenceLabel("audio", index);
    return `文本${index + 1}`;
}

function isResourceNode(node: CanvasNodeData) {
    return Boolean(resourceKind(node));
}

function resourceKind(node: CanvasNodeData): CanvasResourceKind | null {
    if (node.type === CanvasNodeType.Image && node.metadata?.content) return "image";
    if (node.type === CanvasNodeType.Video && node.metadata?.content) return "video";
    if (node.type === CanvasNodeType.Audio && node.metadata?.content) return "audio";
    if (node.type === CanvasNodeType.Text && (node.metadata?.content || node.metadata?.prompt)) return "text";
    return null;
}
