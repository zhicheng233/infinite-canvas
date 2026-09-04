import type { ReactNode } from "react";
import { Tooltip } from "antd";

type Props = {
    name: ReactNode;
    remark?: string | null;
    className?: string;
};

export function ChannelNameWithRemark({ name, remark, className }: Props) {
    const content = <span className={className}>{name}</span>;
    const normalizedRemark = remark?.trim();
    return normalizedRemark ? <Tooltip title={normalizedRemark}>{content}</Tooltip> : content;
}
