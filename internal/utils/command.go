package utils

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type DetectionCommandString string

type DetectionCommandAction string

const (
	ActionStart DetectionCommandAction = "start"
	ActionStop  DetectionCommandAction = "stop"
)

// start -freq 5840 -set_ant 255,-turn_on_gpio 3
type DetectionCommand struct {
	Action     DetectionCommandAction
	Freq       int    // 频率
	SetAnt     string // 天线设置
	TurnOnGPIO int    // 打开GPIO
	Switch     int    // 高低频侦测模块开关参数
	FPVOnly    int    // 低频侦测模块FPV过滤参数
	Rx2        int    // 高低频侦测模块接收通道参数
	Gain       int    // 高低频侦测模块增益参数
	FFTSize    int    // 普通侦测模块频谱分析FFT点数
	BandStart  int    // 频谱分析起始频率 MHz
	BandStop   int    // 频谱分析截止频率 MHz
}

func (c *DetectionCommand) UsesDirectionData() bool {
	return c != nil && c.Action == ActionStart && c.Freq != 0 && !c.IsBandScanFrequency()
}

func (c *DetectionCommand) IsBandScanFrequency() bool {
	return c != nil && (c.Freq == 3 || c.Freq == 4)
}

func (c *DetectionCommand) IsSpectrumAnalysis() bool {
	return c != nil && c.Action == ActionStart && c.BandStart > 0 && c.BandStop > c.BandStart && c.FFTSize > 0
}

func (c *DetectionCommand) TaskKey() string {
	if c == nil {
		return ""
	}

	return fmt.Sprintf(
		"%s|freq=%d|set_ant=%s|gpio=%d|switch=%d|fpv_only=%d|rx2=%d|gain=%d|fft=%d|band=%d-%d",
		c.Action,
		c.Freq,
		c.SetAnt,
		c.TurnOnGPIO,
		c.Switch,
		c.FPVOnly,
		c.Rx2,
		c.Gain,
		c.FFTSize,
		c.BandStart,
		c.BandStop,
	)
}

func (c *DetectionCommandString) ParseCommand() (*DetectionCommand, error) {
	return c.ParseCommandWithConfig()
}

func parseCommandInt(value string) (int, error) {
	trimmed := strings.TrimSpace(strings.Trim(value, ","))
	if trimmed == "" {
		return 0, fmt.Errorf("参数为空")
	}

	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, err
	}
	return int(math.Round(parsed)), nil
}

func (c *DetectionCommandString) ParseCommandWithConfig() (*DetectionCommand, error) {
	if len(*c) == 0 {
		return nil, fmt.Errorf("命令不能为空")
	}

	// 1、去掉前后空格并验证命令前缀
	commandStr := strings.TrimSpace(string(*c))
	commandStr = strings.NewReplacer(",", " ", "\r", " ", "\n", " ").Replace(commandStr)
	parts := strings.Fields(commandStr)

	if len(parts) == 0 {
		return nil, fmt.Errorf("命令不能为空")
	}

	command := &DetectionCommand{}
	verb := strings.ToLower(parts[0])

	if strings.HasPrefix(verb, "stop") {
		command.Action = ActionStop
		return command, nil
	}

	if strings.HasPrefix(verb, "start") || verb == "st" || strings.HasPrefix(verb, "hello") {
		command.Action = ActionStart
		hasFFT := false

		// 解析start命令的参数
		for i := 1; i < len(parts); i++ {
			if parts[i] == "-freq" && i+1 < len(parts) {
				freq, err := parseCommandInt(parts[i+1])
				if err != nil {
					return nil, fmt.Errorf("频率参数解析失败: %v", err)
				}
				command.Freq = freq
				i++ // 跳过参数值
			} else if parts[i] == "-set_ant" && i+1 < len(parts) {
				command.SetAnt = parts[i+1]
				i++ // 跳过参数值
			} else if parts[i] == "-turn_on_gpio" && i+1 < len(parts) {
				gpio, err := parseCommandInt(parts[i+1])
				if err != nil {
					return nil, fmt.Errorf("GPIO参数解析失败: %v", err)
				}
				command.TurnOnGPIO = gpio
				i++ // 跳过参数值
			} else if parts[i] == "-switch" && i+1 < len(parts) {
				value, err := parseCommandInt(parts[i+1])
				if err != nil {
					return nil, fmt.Errorf("switch参数解析失败: %v", err)
				}
				command.Switch = value
				i++ // 跳过参数值
			} else if parts[i] == "-fpv_only" && i+1 < len(parts) {
				value, err := parseCommandInt(parts[i+1])
				if err != nil {
					return nil, fmt.Errorf("fpv_only参数解析失败: %v", err)
				}
				command.FPVOnly = value
				i++ // 跳过参数值
			} else if parts[i] == "-rx2" && i+1 < len(parts) {
				value, err := parseCommandInt(parts[i+1])
				if err != nil {
					return nil, fmt.Errorf("rx2参数解析失败: %v", err)
				}
				command.Rx2 = value
				i++ // 跳过参数值
			} else if parts[i] == "-gain" && i+1 < len(parts) {
				value, err := parseCommandInt(parts[i+1])
				if err != nil {
					return nil, fmt.Errorf("gain参数解析失败: %v", err)
				}
				command.Gain = value
				i++ // 跳过参数值
			} else if parts[i] == "-fft" && i+1 < len(parts) {
				value, err := parseCommandInt(parts[i+1])
				if err != nil {
					return nil, fmt.Errorf("fft参数解析失败: %v", err)
				}
				command.FFTSize = value
				hasFFT = true
				i++ // 跳过参数值
			} else if parts[i] == "-band" && i+2 < len(parts) {
				start, err := parseCommandInt(parts[i+1])
				if err != nil {
					return nil, fmt.Errorf("band起始频率解析失败: %v", err)
				}
				stop, err := parseCommandInt(parts[i+2])
				if err != nil {
					return nil, fmt.Errorf("band截止频率解析失败: %v", err)
				}
				command.BandStart = start
				command.BandStop = stop
				i += 2 // 跳过两个参数值
			}
		}

		if hasFFT && command.FFTSize == 0 {
			command.Action = ActionStop
		}

		return command, nil
	}

	return nil, fmt.Errorf("未知命令")
}
