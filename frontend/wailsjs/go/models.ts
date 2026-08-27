export namespace config {

	export class AnalysisConfig {
	    enabled: boolean;
	    deviceID: number;
	    drone_count: number;
	    hosts: string[];
	    port: number;
	    empty_packet_probability: number;
	    o3_plus_o4_data_file?: string;

	    static createFrom(source: any = {}) {
	        return new AnalysisConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.deviceID = source["deviceID"];
	        this.drone_count = source["drone_count"];
	        this.hosts = source["hosts"];
	        this.port = source["port"];
	        this.empty_packet_probability = source["empty_packet_probability"];
	        this.o3_plus_o4_data_file = source["o3_plus_o4_data_file"];
	    }
	}
	export class BaseConfig {
	    enabled: boolean;
	    deviceID: number;
	    hosts: string[];
	    port: number;

	    static createFrom(source: any = {}) {
	        return new BaseConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.deviceID = source["deviceID"];
	        this.hosts = source["hosts"];
	        this.port = source["port"];
	    }
	}
	export class DirectedStrikeConfig {
	    enabled: boolean;
	    host: string;
	    port: number;
	    response_delay_ms: number;

	    static createFrom(source: any = {}) {
	        return new DirectedStrikeConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.response_delay_ms = source["response_delay_ms"];
	    }
	}
	export class DetectionConfig {
	    enabled: boolean;
	    deviceID: number;
	    drone_count: number;
	    host: string;
	    port: number;
	    heartbeat_interval: number;

	    static createFrom(source: any = {}) {
	        return new DetectionConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.deviceID = source["deviceID"];
	        this.drone_count = source["drone_count"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.heartbeat_interval = source["heartbeat_interval"];
	    }
	}
	export class GPS {
	    lat: number;
	    lng: number;

	    static createFrom(source: any = {}) {
	        return new GPS(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lat = source["lat"];
	        this.lng = source["lng"];
	    }
	}
	export class PredefinedDrone {
	    serial: string;
	    model: string;
	    freq: number;
	    rssi: number;
	    drone_gps: GPS;
	    pilot_gps: GPS;
	    type: string;

	    static createFrom(source: any = {}) {
	        return new PredefinedDrone(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.serial = source["serial"];
	        this.model = source["model"];
	        this.freq = source["freq"];
	        this.rssi = source["rssi"];
	        this.drone_gps = this.convertValues(source["drone_gps"], GPS);
	        this.pilot_gps = this.convertValues(source["pilot_gps"], GPS);
	        this.type = source["type"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Config {
	    min_push_speed: number;
	    max_push_speed: number;
	    predefined_drones: PredefinedDrone[];
	    random_drone_refresh_interval: number;
	    max_direction_change: number;
	    max_distance_from_center_point: number;
	    center_point: GPS;
	    analysis: AnalysisConfig;
	    detection: DetectionConfig;
	    detections: DetectionConfig[];
	    fpv: BaseConfig;
	    jamming: BaseConfig;
	    directed_strike: DirectedStrikeConfig;

	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.min_push_speed = source["min_push_speed"];
	        this.max_push_speed = source["max_push_speed"];
	        this.predefined_drones = this.convertValues(source["predefined_drones"], PredefinedDrone);
	        this.random_drone_refresh_interval = source["random_drone_refresh_interval"];
	        this.max_direction_change = source["max_direction_change"];
	        this.max_distance_from_center_point = source["max_distance_from_center_point"];
	        this.center_point = this.convertValues(source["center_point"], GPS);
	        this.analysis = this.convertValues(source["analysis"], AnalysisConfig);
	        this.detection = this.convertValues(source["detection"], DetectionConfig);
	        this.detections = this.convertValues(source["detections"], DetectionConfig);
	        this.fpv = this.convertValues(source["fpv"], BaseConfig);
	        this.jamming = this.convertValues(source["jamming"], BaseConfig);
	        this.directed_strike = this.convertValues(source["directed_strike"], DirectedStrikeConfig);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}




}

export namespace engine {

	export class ModuleStatus {
	    name: string;
	    connected: boolean;
	    connectionCount: number;
	    sentCount: number;
	    lastActivityAt?: string;
	    clientAddresses?: string[];

	    static createFrom(source: any = {}) {
	        return new ModuleStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.connected = source["connected"];
	        this.connectionCount = source["connectionCount"];
	        this.sentCount = source["sentCount"];
	        this.lastActivityAt = source["lastActivityAt"];
	        this.clientAddresses = source["clientAddresses"];
	    }
	}
	export class Status {
	    running: boolean;
	    modules: ModuleStatus[];
	    droneCount: number;
	    directedStrike?: modules.DirectedStrikeSnapshot;

	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.modules = this.convertValues(source["modules"], ModuleStatus);
	        this.droneCount = source["droneCount"];
	        this.directedStrike = this.convertValues(source["directedStrike"], modules.DirectedStrikeSnapshot);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace main {

	export class LocationSearchResult {
	    displayName: string;
	    lat: number;
	    lng: number;

	    static createFrom(source: any = {}) {
	        return new LocationSearchResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.displayName = source["displayName"];
	        this.lat = source["lat"];
	        this.lng = source["lng"];
	    }
	}

}

export namespace modules {

	export class DirectedStrikeAmpStatus {
	    address: string;
	    enabled: boolean;

	    static createFrom(source: any = {}) {
	        return new DirectedStrikeAmpStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.address = source["address"];
	        this.enabled = source["enabled"];
	    }
	}
	export class DirectedStrikeFrequencyStatus {
	    address: string;
	    startFreqMHz: number;
	    endFreqMHz: number;
	    lastUpdatedAt?: string;

	    static createFrom(source: any = {}) {
	        return new DirectedStrikeFrequencyStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.address = source["address"];
	        this.startFreqMHz = source["startFreqMHz"];
	        this.endFreqMHz = source["endFreqMHz"];
	        this.lastUpdatedAt = source["lastUpdatedAt"];
	    }
	}
	export class DirectedStrikePTZStatus {
	    horizontalAngle: number;
	    pitchAngle: number;
	    action?: string;
	    horizontalSpeed: number;
	    verticalSpeed: number;
	    locateHorizontalSpeed: number;
	    locateVerticalSpeed: number;
	    presetSaved: boolean;

	    static createFrom(source: any = {}) {
	        return new DirectedStrikePTZStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.horizontalAngle = source["horizontalAngle"];
	        this.pitchAngle = source["pitchAngle"];
	        this.action = source["action"];
	        this.horizontalSpeed = source["horizontalSpeed"];
	        this.verticalSpeed = source["verticalSpeed"];
	        this.locateHorizontalSpeed = source["locateHorizontalSpeed"];
	        this.locateVerticalSpeed = source["locateVerticalSpeed"];
	        this.presetSaved = source["presetSaved"];
	    }
	}
	export class DirectedStrikeSourceStatus {
	    address: string;
	    enabled: boolean;

	    static createFrom(source: any = {}) {
	        return new DirectedStrikeSourceStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.address = source["address"];
	        this.enabled = source["enabled"];
	    }
	}
	export class DirectedStrikeSnapshot {
	    listening: boolean;
	    listenAddress: string;
	    activeConnections: number;
	    clientAddresses: string[];
	    heartbeatCount: number;
	    receivedFrames: number;
	    sentFrames: number;
	    lastCommand?: string;
	    lastCommandDetail?: string;
	    lastActivityAt?: string;
	    lastError?: string;
	    signalSources: DirectedStrikeSourceStatus[];
	    widebandConfigs: DirectedStrikeFrequencyStatus[];
	    narrowbandAmps: DirectedStrikeAmpStatus[];
	    ptz: DirectedStrikePTZStatus;

	    static createFrom(source: any = {}) {
	        return new DirectedStrikeSnapshot(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.listening = source["listening"];
	        this.listenAddress = source["listenAddress"];
	        this.activeConnections = source["activeConnections"];
	        this.clientAddresses = source["clientAddresses"];
	        this.heartbeatCount = source["heartbeatCount"];
	        this.receivedFrames = source["receivedFrames"];
	        this.sentFrames = source["sentFrames"];
	        this.lastCommand = source["lastCommand"];
	        this.lastCommandDetail = source["lastCommandDetail"];
	        this.lastActivityAt = source["lastActivityAt"];
	        this.lastError = source["lastError"];
	        this.signalSources = this.convertValues(source["signalSources"], DirectedStrikeSourceStatus);
	        this.widebandConfigs = this.convertValues(source["widebandConfigs"], DirectedStrikeFrequencyStatus);
	        this.narrowbandAmps = this.convertValues(source["narrowbandAmps"], DirectedStrikeAmpStatus);
	        this.ptz = this.convertValues(source["ptz"], DirectedStrikePTZStatus);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

