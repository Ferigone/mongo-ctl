export namespace app {
	
	export class CreateInstanceRequest {
	    name: string;
	    port: number;
	    dataDir: string;
	
	    static createFrom(source: any = {}) {
	        return new CreateInstanceRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.port = source["port"];
	        this.dataDir = source["dataDir"];
	    }
	}
	export class GroupRequest {
	    name: string;
	    instanceIds: string[];
	    clearNonSeedData: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GroupRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.instanceIds = source["instanceIds"];
	        this.clearNonSeedData = source["clearNonSeedData"];
	    }
	}
	export class InstanceView {
	    id: string;
	    name: string;
	    port: number;
	    dataDir: string;
	    configPath: string;
	    replicaSet: string;
	    // Go type: time
	    createdAt: any;
	    state: string;
	    pid: number;
	    // Go type: time
	    startedAt: any;
	    error: string;
	    uncleanStop: boolean;
	
	    static createFrom(source: any = {}) {
	        return new InstanceView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.port = source["port"];
	        this.dataDir = source["dataDir"];
	        this.configPath = source["configPath"];
	        this.replicaSet = source["replicaSet"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.state = source["state"];
	        this.pid = source["pid"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.error = source["error"];
	        this.uncleanStop = source["uncleanStop"];
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
	export class PreflightReport {
	    seedName: string;
	    requiresRestart: string[];
	    populatedNonSeed: string[];
	    evenMemberCount: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PreflightReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.seedName = source["seedName"];
	        this.requiresRestart = source["requiresRestart"];
	        this.populatedNonSeed = source["populatedNonSeed"];
	        this.evenMemberCount = source["evenMemberCount"];
	    }
	}
	export class ReplicaSetView {
	    name: string;
	    memberIds: string[];
	    // Go type: time
	    createdAt: any;
	    status?: replicaset.Status;
	    statusError: string;
	
	    static createFrom(source: any = {}) {
	        return new ReplicaSetView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.memberIds = source["memberIds"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.status = this.convertValues(source["status"], replicaset.Status);
	        this.statusError = source["statusError"];
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
	export class SystemInfo {
	    mongod: mongobin.Binary;
	    settings: store.Settings;
	    elevated: boolean;
	    service?: winsys.ServiceInfo;
	    serviceError: string;
	
	    static createFrom(source: any = {}) {
	        return new SystemInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mongod = this.convertValues(source["mongod"], mongobin.Binary);
	        this.settings = this.convertValues(source["settings"], store.Settings);
	        this.elevated = source["elevated"];
	        this.service = this.convertValues(source["service"], winsys.ServiceInfo);
	        this.serviceError = source["serviceError"];
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

export namespace logstream {
	
	export class Line {
	    // Go type: time
	    timestamp: any;
	    severity: string;
	    component: string;
	    message: string;
	    raw: string;
	
	    static createFrom(source: any = {}) {
	        return new Line(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.severity = source["severity"];
	        this.component = source["component"];
	        this.message = source["message"];
	        this.raw = source["raw"];
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

export namespace metrics {
	
	export class DatabaseInfo {
	    name: string;
	    sizeOnDisk: number;
	    empty: boolean;
	    collections: number;
	    objects: number;
	    dataSize: number;
	    storageSize: number;
	    indexSize: number;
	    avgObjSize: number;
	
	    static createFrom(source: any = {}) {
	        return new DatabaseInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.sizeOnDisk = source["sizeOnDisk"];
	        this.empty = source["empty"];
	        this.collections = source["collections"];
	        this.objects = source["objects"];
	        this.dataSize = source["dataSize"];
	        this.storageSize = source["storageSize"];
	        this.indexSize = source["indexSize"];
	        this.avgObjSize = source["avgObjSize"];
	    }
	}
	export class Sample {
	    // Go type: time
	    timestamp: any;
	    connectionsCurrent: number;
	    connectionsActive: number;
	    connectionsAvailable: number;
	    memResidentMb: number;
	    memVirtualMb: number;
	    cacheUsedBytes: number;
	    cacheMaxBytes: number;
	    cacheFillPercent: number;
	    queueReaders: number;
	    queueWriters: number;
	    insertsPerSec: number;
	    queriesPerSec: number;
	    updatesPerSec: number;
	    deletesPerSec: number;
	    commandsPerSec: number;
	    getMoresPerSec: number;
	    bytesInPerSec: number;
	    bytesOutPerSec: number;
	    readLatencyMs: number;
	    writeLatencyMs: number;
	    commandLatencyMs: number;
	    gap: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Sample(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.connectionsCurrent = source["connectionsCurrent"];
	        this.connectionsActive = source["connectionsActive"];
	        this.connectionsAvailable = source["connectionsAvailable"];
	        this.memResidentMb = source["memResidentMb"];
	        this.memVirtualMb = source["memVirtualMb"];
	        this.cacheUsedBytes = source["cacheUsedBytes"];
	        this.cacheMaxBytes = source["cacheMaxBytes"];
	        this.cacheFillPercent = source["cacheFillPercent"];
	        this.queueReaders = source["queueReaders"];
	        this.queueWriters = source["queueWriters"];
	        this.insertsPerSec = source["insertsPerSec"];
	        this.queriesPerSec = source["queriesPerSec"];
	        this.updatesPerSec = source["updatesPerSec"];
	        this.deletesPerSec = source["deletesPerSec"];
	        this.commandsPerSec = source["commandsPerSec"];
	        this.getMoresPerSec = source["getMoresPerSec"];
	        this.bytesInPerSec = source["bytesInPerSec"];
	        this.bytesOutPerSec = source["bytesOutPerSec"];
	        this.readLatencyMs = source["readLatencyMs"];
	        this.writeLatencyMs = source["writeLatencyMs"];
	        this.commandLatencyMs = source["commandLatencyMs"];
	        this.gap = source["gap"];
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

export namespace mongobin {
	
	export class Binary {
	    path: string;
	    version: string;
	    major: number;
	    minor: number;
	    patch: number;
	
	    static createFrom(source: any = {}) {
	        return new Binary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.version = source["version"];
	        this.major = source["major"];
	        this.minor = source["minor"];
	        this.patch = source["patch"];
	    }
	}

}

export namespace replicaset {
	
	export class Member {
	    id: number;
	    host: string;
	    stateStr: string;
	    health: number;
	    uptimeSecs: number;
	    // Go type: time
	    optimeDate: any;
	    lagSeconds: number;
	    isPrimary: boolean;
	    self: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Member(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.host = source["host"];
	        this.stateStr = source["stateStr"];
	        this.health = source["health"];
	        this.uptimeSecs = source["uptimeSecs"];
	        this.optimeDate = this.convertValues(source["optimeDate"], null);
	        this.lagSeconds = source["lagSeconds"];
	        this.isPrimary = source["isPrimary"];
	        this.self = source["self"];
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
	export class Status {
	    name: string;
	    members: Member[];
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.members = this.convertValues(source["members"], Member);
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

export namespace store {
	
	export class Settings {
	    dataRoot: string;
	    mongodPath: string;
	    basePort: number;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dataRoot = source["dataRoot"];
	        this.mongodPath = source["mongodPath"];
	        this.basePort = source["basePort"];
	    }
	}

}

export namespace winsys {
	
	export class ServiceInfo {
	    name: string;
	    displayName: string;
	    state: string;
	    startType: string;
	    binaryPath: string;
	
	    static createFrom(source: any = {}) {
	        return new ServiceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.displayName = source["displayName"];
	        this.state = source["state"];
	        this.startType = source["startType"];
	        this.binaryPath = source["binaryPath"];
	    }
	}

}

