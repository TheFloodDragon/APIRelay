package relay

import "github.com/apirelay/apirelay/relay/relaycommon"

// RelayInfo 是 relaycommon.RelayInfo 的别名，方便 relay 主包直接引用。
// 真正的定义在 relay/relaycommon 子包，以便各 adaptor 子包共享而不产生循环依赖。
type RelayInfo = relaycommon.RelayInfo
