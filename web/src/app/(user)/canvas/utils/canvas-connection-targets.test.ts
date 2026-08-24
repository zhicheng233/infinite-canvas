import { describe, expect, test } from "bun:test";

import { createDefaultCustomVideoConfig, type CustomVideoConfig } from "@/lib/custom-video-config";
import { defaultConfig, type AiConfig } from "@/stores/use-config-store";
import { CanvasNodeType, type CanvasConnection, type CanvasImageRole, type CanvasNodeData } from "../types";
import { canvasConnectionImageRole, canvasConnectionTargetForRole, canvasConnectionTargetsForNode } from "./canvas-connection-targets";
import { normalizeCanvasConnection, normalizeCanvasConnections } from "./canvas-connections";

const model = "canvas-reference-compatibility";
const videoNode: CanvasNodeData = {
    id: "video-node",
    type: CanvasNodeType.Config,
    title: "视频配置",
    position: { x: 100, y: 100 },
    width: 360,
    height: 240,
    metadata: { generationMode: "video", model },
};
const legacyConnection: CanvasConnection = { id: "legacy", fromNodeId: "image-node", toNodeId: videoNode.id };

describe("canvas video connection targets", () => {
    test("uses the legacy connection shape and label for normal reference images", () => {
        const config = customAiConfig(customConfig("images"));
        const targets = canvasConnectionTargetsForNode(config, videoNode, [legacyConnection]);

        expect(targets).toHaveLength(1);
        expect(targets[0]).toMatchObject({ label: "图片参考", effectiveImageRole: "images", acceptsLegacyConnection: true, connectedCount: 1 });
        expect(targets[0]?.targetImageRole).toBeUndefined();
        expect(canvasConnectionImageRole(config, videoNode)).toBe("images");
    });

    test("derives the legacy slot again after switching away from and back to a custom model", () => {
        const custom = customAiConfig(customConfig("input_reference"));
        const standard = standardAiConfig();

        expect(canvasConnectionTargetsForNode(custom, videoNode, [legacyConnection])[0]?.label).toBe("首帧参考图");
        expect(canvasConnectionTargetsForNode(standard, videoNode, [legacyConnection])[0]?.label).toBe("图片参考");
        const restoredTargets = canvasConnectionTargetsForNode(custom, videoNode, [legacyConnection]);
        expect(restoredTargets[0]?.label).toBe("首帧参考图");
        expect(canvasConnectionTargetForRole(restoredTargets)?.effectiveImageRole).toBe("input_reference");
        expect(canvasConnectionImageRole(custom, videoNode)).toBe("input_reference");
        expect(canvasConnectionImageRole(custom, videoNode, "images")).toBe("input_reference");
        expect(legacyConnection).toEqual({ id: "legacy", fromNodeId: "image-node", toNodeId: videoNode.id });
    });

    test("keeps an ambiguous legacy slot unavailable when multiple non-default roles are enabled", () => {
        const config = customAiConfig(customConfig("input_reference", "style_references"));
        const targets = canvasConnectionTargetsForNode(config, videoNode, [legacyConnection]);

        expect(targets.map((target) => target.label)).toEqual(["图片参考（请重新连接到角色入口）", "首帧参考图", "风格参考图"]);
        expect(targets[0]).toMatchObject({ available: false, connectedCount: 1 });
        expect(canvasConnectionImageRole(config, videoNode)).toBeUndefined();
    });

    test("normalizes explicit images connections to the legacy shape and removes aliases", () => {
        const explicit = { ...legacyConnection, id: "explicit", targetImageRole: "images" as const };

        expect(normalizeCanvasConnection(explicit)).toEqual({ id: "explicit", fromNodeId: "image-node", toNodeId: videoNode.id });
        expect(normalizeCanvasConnections([legacyConnection, explicit])).toEqual([legacyConnection]);
    });

    test("retains an explicit role while it is unsupported and restores it with the custom model", () => {
        const roleConnection: CanvasConnection = { id: "style", fromNodeId: "image-node", toNodeId: videoNode.id, targetImageRole: "style_references" };
        const standardTargets = canvasConnectionTargetsForNode(standardAiConfig(), videoNode, [roleConnection]);
        const customTargets = canvasConnectionTargetsForNode(customAiConfig(customConfig("style_references")), videoNode, [roleConnection]);

        expect(canvasConnectionTargetForRole(standardTargets, "style_references")).toMatchObject({ available: false, connectedCount: 1 });
        expect(canvasConnectionTargetForRole(customTargets, "style_references")).toMatchObject({ enabled: true, connectedCount: 1 });
        expect(roleConnection.targetImageRole).toBe("style_references");
    });
});

function customConfig(...roles: CanvasImageRole[]): CustomVideoConfig {
    const config = createDefaultCustomVideoConfig();
    return {
        ...config,
        images: { ...config.images, enabled: roles.includes("images"), max_count: 2 },
        input_reference: { ...config.input_reference, enabled: roles.includes("input_reference"), max_count: 2 },
        style_references: { ...config.style_references, enabled: roles.includes("style_references"), max_count: 2 },
        element_references: { ...config.element_references, enabled: roles.includes("element_references"), max_count: 2 },
        reference_images: { ...config.reference_images, enabled: roles.includes("reference_images"), max_count: 2 },
    };
}

function customAiConfig(videoConfig: CustomVideoConfig): AiConfig {
    return {
        ...defaultConfig,
        model,
        videoModel: model,
        modelRoutes: { [`video:${model}`]: "custom" },
        modelCustomVideoConfigs: { [model]: videoConfig },
    };
}

function standardAiConfig(): AiConfig {
    return { ...defaultConfig, model, videoModel: model, modelRoutes: { [`video:${model}`]: "openai" }, modelCustomVideoConfigs: {} };
}
