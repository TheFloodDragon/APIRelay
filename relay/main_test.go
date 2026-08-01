package relay

import (
	"os"
	"testing"

	"github.com/apirelay/apirelay/relay/adaptor"
)

// TestMain 为整个 relay 包关闭 SSRF 守卫。
//
// 探测与连通性测试的用例都用 httptest 起在 127.0.0.1 上，而守卫按设计会拦截环回地址。
// 在这里统一关闭，新增用例无需各自处理；守卫本身的行为由
// relay/adaptor/ssrf_test.go 与 relay/probe_ssrf_test.go 独立覆盖。
func TestMain(m *testing.M) {
	restore := adaptor.SetProbeGuardDisabledForTest(true)
	code := m.Run()
	restore()
	os.Exit(code)
}
