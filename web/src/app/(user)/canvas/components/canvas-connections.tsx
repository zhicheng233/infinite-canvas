import type { MouseEvent as ReactMouseEvent } from "react";

import { canvasThemes } from "@/lib/canvas-theme";
import { useThemeStore } from "@/stores/use-theme-store";
import type { CanvasConnection, CanvasNodeData, ConnectionHandle, Position } from "../types";
import { canvasConnectionSourceAnchor, canvasConnectionTargetAnchor, type CanvasConnectionTarget } from "../utils/canvas-connection-targets";

export function ConnectionPath({
    connection,
    from,
    to,
    targetTargets,
    active,
    onSelect,
    onContextMenu,
}: {
    connection: CanvasConnection;
    from: CanvasNodeData;
    to: CanvasNodeData;
    targetTargets?: readonly CanvasConnectionTarget[];
    active: boolean;
    onSelect: () => void;
    onContextMenu?: (event: ReactMouseEvent<SVGPathElement>) => void;
}) {
    const theme = canvasThemes[useThemeStore((state) => state.theme)];
    const start = canvasConnectionSourceAnchor(from);
    const end = canvasConnectionTargetAnchor(to, targetTargets || [], connection.targetImageRole);
    const startX = start.x;
    const startY = start.y;
    const endX = end.x;
    const endY = end.y;
    const dx = Math.abs(endX - startX);
    const curvature = Math.max(dx * 0.5, 50);
    const pathD = `M ${startX} ${startY} C ${startX + curvature} ${startY}, ${endX - curvature} ${endY}, ${endX} ${endY}`;

    return (
        <g>
            <path
                data-connection-id={connection.id}
                d={pathD}
                stroke="transparent"
                strokeWidth="16"
                fill="none"
                style={{ cursor: "pointer", pointerEvents: "stroke" }}
                onClick={(event) => {
                    event.stopPropagation();
                    onSelect();
                }}
                onContextMenu={(event) => {
                    event.preventDefault();
                    event.stopPropagation();
                    onContextMenu?.(event);
                }}
            />
            <path
                d={pathD}
                stroke={active ? theme.node.activeStroke : theme.node.muted}
                strokeWidth={active ? 3 : 2}
                strokeOpacity={active ? 1 : 0.82}
                fill="none"
                style={{ filter: active ? `drop-shadow(0 0 8px ${theme.node.activeStroke}66)` : undefined, pointerEvents: "none" }}
            />
        </g>
    );
}

export function ActiveConnectionPath({
    node,
    handle,
    mouseWorld,
    target,
    targetImageRole,
    nodeTargets,
    targetTargets,
}: {
    node?: CanvasNodeData;
    handle: ConnectionHandle;
    mouseWorld: Position;
    target?: CanvasNodeData;
    targetImageRole?: ConnectionHandle["targetImageRole"];
    nodeTargets?: readonly CanvasConnectionTarget[];
    targetTargets?: readonly CanvasConnectionTarget[];
}) {
    const theme = canvasThemes[useThemeStore((state) => state.theme)];
    if (!node) return null;

    const effectiveTargetImageRole = targetImageRole ?? handle.targetImageRole;
    const source = canvasConnectionSourceAnchor(node);
    const targetSource = target ? canvasConnectionSourceAnchor(target) : undefined;
    const targetImage = target ? canvasConnectionTargetAnchor(target, targetTargets || [], effectiveTargetImageRole) : undefined;
    const nodeTarget = canvasConnectionTargetAnchor(node, nodeTargets || [], effectiveTargetImageRole);
    const startX = handle.handleType === "source" ? source.x : mouseWorld.x;
    const startY = handle.handleType === "source" ? source.y : mouseWorld.y;
    const endX = handle.handleType === "source" ? mouseWorld.x : nodeTarget.x;
    const endY = handle.handleType === "source" ? mouseWorld.y : nodeTarget.y;
    const snappedStartX = handle.handleType === "target" && targetSource ? targetSource.x : startX;
    const snappedStartY = handle.handleType === "target" && targetSource ? targetSource.y : startY;
    const snappedEndX = handle.handleType === "source" && targetImage ? targetImage.x : endX;
    const snappedEndY = handle.handleType === "source" && targetImage ? targetImage.y : endY;
    const distance = Math.abs(snappedEndX - snappedStartX);
    const pathD = `M ${snappedStartX} ${snappedStartY} C ${snappedStartX + distance * 0.5} ${snappedStartY}, ${snappedEndX - distance * 0.5} ${snappedEndY}, ${snappedEndX} ${snappedEndY}`;

    return <path d={pathD} stroke={theme.node.activeStroke} strokeWidth="2" fill="none" strokeDasharray="5,5" />;
}
