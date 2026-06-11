package utils

import (
	"fmt"
	"net"
	"time"
)

// IsPortInUse 检查指定端口是否被占用（默认检查TCP）
// 参数 port: 要检查的端口号 (1-65535)
// 返回值: true表示端口被占用，false表示端口可用
func IsPortInUse(port int) bool {
	return IsPortInUseWithAddress("", port)
}

// IsPortInUseWithAddress 检查指定地址和端口是否被占用（默认检查TCP）
// 参数 address: IP地址，空字符串表示所有地址
// 参数 port: 要检查的端口号 (1-65535)
// 返回值: true表示端口被占用，false表示端口可用
func IsPortInUseWithAddress(address string, port int) bool {
	return IsPortInUseWithProtocol("tcp", address, port)
}

// IsUDPPortInUse 检查指定UDP端口是否被占用
// 参数 port: 要检查的端口号 (1-65535)
// 返回值: true表示端口被占用，false表示端口可用
func IsUDPPortInUse(port int) bool {
	return IsUDPPortInUseWithAddress("", port)
}

// IsUDPPortInUseWithAddress 检查指定地址和UDP端口是否被占用
// 参数 address: IP地址，空字符串表示所有地址
// 参数 port: 要检查的端口号 (1-65535)
// 返回值: true表示端口被占用，false表示端口可用
func IsUDPPortInUseWithAddress(address string, port int) bool {
	return IsPortInUseWithProtocol("udp", address, port)
}

// IsPortInUseWithProtocol 检查指定协议、地址和端口是否被占用
// 参数 protocol: 协议类型 ("tcp" 或 "udp")
// 参数 address: IP地址，空字符串表示所有地址
// 参数 port: 要检查的端口号 (1-65535)
// 返回值: true表示端口被占用，false表示端口可用
func IsPortInUseWithProtocol(protocol, address string, port int) bool {
	// 验证端口范围
	if port < 1 || port > 65535 {
		return true // 无效端口认为被占用
	}

	// 构建监听地址，使用net.JoinHostPort处理IPv6地址
	var addr string
	if address == "" {
		addr = fmt.Sprintf(":%d", port)
	} else {
		addr = net.JoinHostPort(address, fmt.Sprintf("%d", port))
	}

	if protocol == "udp" {
		// UDP端口检测：尝试监听该端口
		udpAddr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			return true // 解析失败认为被占用
		}

		listener, err := net.ListenUDP("udp", udpAddr)
		if err != nil {
			// 监听失败，端口被占用
			return true
		}

		// 能够监听，说明端口可用
		listener.Close()
		return false
	} else {
		// TCP端口检测：尝试连接该端口
		timeout := 2 * time.Second
		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err != nil {
			// 连接失败，端口可能未被占用
			return false
		}

		// 能够连接，说明端口被占用
		conn.Close()
		return true
	}
}

// IsPortAvailable 检查端口是否可用（IsPortInUse的反向操作）
// 返回值: true表示端口可用，false表示端口被占用
func IsPortAvailable(port int) bool {
	return !IsPortInUse(port)
}

// IsUDPPortAvailable 检查UDP端口是否可用
// 返回值: true表示端口可用，false表示端口被占用
func IsUDPPortAvailable(port int) bool {
	return !IsUDPPortInUse(port)
}
