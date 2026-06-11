package mocks

// DroneModel 无人机型号定义
type DroneModel struct {
	Model    string `json:"model"`
	RemoteID bool   `json:"remote_id"`
}

// DroneModels 所有无人机型号数据
var DroneModels = map[string][]DroneModel{
	"dji": {
		{Model: "Mini 3 Pro", RemoteID: false},
		{Model: "Mini 3", RemoteID: false},
		{Model: "Mini 2", RemoteID: false},
		{Model: "Mini 2 SE", RemoteID: false},
		{Model: "Mini SE", RemoteID: false},
		{Model: "Mini 4K", RemoteID: false},
		{Model: "Air 3", RemoteID: false},
		{Model: "Air 2S", RemoteID: false},
		{Model: "Air 2", RemoteID: false},
		{Model: "Air", RemoteID: false},
		{Model: "Mavic 3", RemoteID: false},
		{Model: "Mavic 2", RemoteID: false},
		{Model: "Mavic Pro", RemoteID: false},
		{Model: "FPV", RemoteID: false},
		{Model: "FPV Mini", RemoteID: false},
		{Model: "FPV Pro", RemoteID: false},
		{Model: "Matrice 4", RemoteID: false},
		{Model: "Matrice 300 RTK", RemoteID: false},
		{Model: "Matrice 600 Pro", RemoteID: false},
		{Model: "Matrice 200", RemoteID: false},
		{Model: "Matrice 210", RemoteID: false},
	},
	"autel": {
		{Model: "EVO II Pro", RemoteID: true},
		{Model: "EVO II", RemoteID: true},
		{Model: "EVO Nano", RemoteID: true},
		{Model: "EVO Nano+", RemoteID: true},
		{Model: "EVO Lite", RemoteID: true},
		{Model: "EVO Lite+", RemoteID: true},
		{Model: "EVO Max 4T", RemoteID: true},
		{Model: "EVO Max 4N", RemoteID: true},
	},
}

// GetAllDJIModels 获取所有大疆无人机型号
func GetAllDJIModels() []string {
	models := make([]string, 0, len(DroneModels["dji"]))
	for _, m := range DroneModels["dji"] {
		models = append(models, m.Model)
	}
	return models
}

// GetAllAutelModels 获取所有道通无人机型号
func GetAllAutelModels() []string {
	models := make([]string, 0, len(DroneModels["autel"]))
	for _, m := range DroneModels["autel"] {
		models = append(models, m.Model)
	}
	return models
}

// GetAllModels 获取所有无人机型号
func GetAllModels() []string {
	var models []string
	for _, brand := range DroneModels {
		for _, m := range brand {
			models = append(models, m.Model)
		}
	}
	return models
}

// IsRemoteIDModel 判断型号是否支持RemoteID
func IsRemoteIDModel(model string) bool {
	for _, brand := range DroneModels {
		for _, m := range brand {
			if m.Model == model {
				return m.RemoteID
			}
		}
	}
	return false
}

// IsDJIModel 判断是否为大疆型号
func IsDJIModel(model string) bool {
	for _, m := range DroneModels["dji"] {
		if m.Model == model {
			return true
		}
	}
	return false
}

// IsAutelModel 判断是否为道通型号
func IsAutelModel(model string) bool {
	for _, m := range DroneModels["autel"] {
		if m.Model == model {
			return true
		}
	}
	return false
}
