package modules

type ModuleType string

const (
	ModuleTypeAnalysis  ModuleType = "analysis"
	ModuleTypeDetection ModuleType = "detection"
	ModuleTypeFPV       ModuleType = "fpv"
	ModuleTypeJamming   ModuleType = "jamming" // 新增干扰打击类型
)
