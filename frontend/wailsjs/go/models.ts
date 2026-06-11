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
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.modules = this.convertValues(source["modules"], ModuleStatus);
	        this.droneCount = source["droneCount"];
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

