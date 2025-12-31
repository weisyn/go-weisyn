package interfaces

import pkgispc "github.com/weisyn/v1/pkg/interfaces/ispc"

// 使用公共接口，保持 engines ↔ ISPC 解耦
// 内部实现（runtime.HostRuntimePorts）需满足该接口

type HostABI = pkgispc.HostABI
