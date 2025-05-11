package XRay

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	_ "github.com/YangCheng0121/xray-core-mobile-wrapper/all_core_packages"

	xrayNet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf/serial"
)

type CompletionHandler func(int64, error)

type Logger interface {
	LogInput(s string)
}

var coreInstance *core.Instance

// Sets the limit on memory consumption by a process.
// Also set garbage collection target percentage
func SetMemoryLimit(byteLimit int64, garbageCollectionTargetPercentage int) {
	debug.SetGCPercent(garbageCollectionTargetPercentage)
	debug.SetMemoryLimit(byteLimit)
}

// Removes the memory usage limit
// and returns the garbage collector frequency to the default
func RemoveMemoryLimit() {
	debug.SetGCPercent(100)
	debug.SetMemoryLimit(math.MaxInt64)
}

// Ser AssetsDirectory in Xray env
func SetAssetsDirectory(path string) {
	os.Setenv("xray.location.asset", path)
}

// [key] can be:
// PluginLocation  = "xray.location.plugin"  // 插件二进制文件目录路径
// ConfigLocation  = "xray.location.config"  // 主配置文件(config.json)路径
// ConfdirLocation = "xray.location.confdir" // 多配置文件目录路径
// ToolLocation    = "xray.location.tool"    // 工具程序目录路径
// AssetLocation   = "xray.location.asset"   // 资源文件(geoip/geosite.dat)目录
// CertLocation    = "xray.location.cert"    // 证书文件目录路径
//
// UseReadV         = "xray.buf.readv"         // 是否启用readv系统调用(性能优化)
// UseFreedomSplice = "xray.buf.splice"        // 是否启用splice零拷贝传输(Linux特有)
// UseVmessPadding  = "xray.vmess.padding"     // 是否启用VMess协议流量填充(混淆)
// UseCone          = "xray.cone.disabled"     // 是否禁用Cone NAT检测
//
// BufferSize           = "xray.ray.buffer.size"    // 每个连接的缓冲区大小(单位KB)
// BrowserDialerAddress = "xray.browser.dialer"    // 浏览器API代理地址
// XUDPLog              = "xray.xudp.show"         // 是否显示XUDP协议调试日志
// XUDPBaseKey          = "xray.xudp.basekey"      // XUDP协议基础密钥
func SetXrayEnv(key string, path string) {
	os.Setenv(key, path)
}

func StartXray(config []byte, logger Logger) error {
	conf, err := serial.DecodeJSONConfig(bytes.NewReader(config))
	if err != nil {
		logger.LogInput("Config load error: " + err.Error())
		return err
	}

	pbConfig, err := conf.Build()
	if err != nil {
		return err
	}
	instance, err := core.New(pbConfig)
	if err != nil {
		logger.LogInput("Create XRay error: " + err.Error())
		return err
	}

	err = instance.Start()
	if err != nil {
		logger.LogInput("Start XRay error: " + err.Error())
	}

	coreInstance = instance
	return nil
}

func StopXray() {
	coreInstance.Close()
}

// / Real ping
func MeasureOutboundDelay(config []byte, url string) (int64, error) {
	conf, err := serial.DecodeJSONConfig(bytes.NewReader(config))
	if err != nil {
		return -1, err
	}
	pbConfig, err := conf.Build()
	if err != nil {
		return -1, err
	}

	// dont listen to anything for test purpose
	pbConfig.Inbound = nil
	// config.App: (fakedns), log, dispatcher, InboundConfig, OutboundConfig, (stats), router, dns, (policy)
	// keep only basic features
	pbConfig.App = pbConfig.App[:5]

	inst, err := core.New(pbConfig)
	if err != nil {
		return -1, err
	}

	inst.Start()
	return measureInstDelay(context.Background(), inst, url)
}

func measureInstDelay(ctx context.Context, inst *core.Instance, url string) (int64, error) {
	if inst == nil {
		return -1, errors.New("core instance nil")
	}

	tr := &http.Transport{
		TLSHandshakeTimeout: 6 * time.Second,
		DisableKeepAlives:   true,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dest, err := xrayNet.ParseDestination(fmt.Sprintf("%s:%s", network, addr))
			if err != nil {
				return nil, err
			}
			return core.Dial(ctx, inst, dest)
		},
	}

	c := &http.Client{
		Transport: tr,
		Timeout:   12 * time.Second,
	}

	if len(url) <= 0 {
		url = "https://www.google.com/generate_204"
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	start := time.Now()

	resp, err := c.Do(req)
	if err != nil {
		return -1, err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return -1, fmt.Errorf("status != 20x: %s", resp.Status)
	}
	resp.Body.Close()
	return time.Since(start).Milliseconds(), nil

}

// GetCoreVersion
func GetCoreVersion() string {
	var version = 99
	return fmt.Sprintf("Lib v%d, Xray-core v%s", version, core.Version())
}
